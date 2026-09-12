package accounts

import (
	"fmt"
	"log"
	"sync"
	"time"

	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/zhidao"
)

// Store 凭据持久化最小接口（store.Store 实现）。
type Store interface {
	SaveCredential(acct, passwordEnc, idToken string) error
	LoadCredentials() ([]Credential, error)
}

// Credential 账号凭据（与 store.Credential 字段一一对应，避免包循环）。
type Credential struct {
	Account     string
	PasswordEnc string
	IDToken     string
}

// Manager 多账号客户端注册表：每个账号一个独立 zhidao.Client（独立 token/cookie 会话）。
// 账号 A 的请求绝不携带账号 B 的会话——物理隔离的核心。
type Manager struct {
	baseURL string
	vision  zhidao.VisionConfig
	st      Store

	mu      sync.Mutex
	clients map[string]*zhidao.Client // 账号名 -> 独立客户端
	order   []string                  // 登录顺序

	// 全局重登频率闸门（安全审计 CRITICAL 2a）：所有账号共享同一出口 IP 打平台 doLogin，
	// 若平台风控含 IP 维度，多个账号同时失效时全速重登会把整个 IP 刷到锁号（全盘陪葬）。
	// 令牌桶：全账号合计每分钟 doLogin 最多 gateLoginPerMin 次；超出的重登等待下个窗口。
	gateMu     sync.Mutex
	gateWindow time.Time // 当前一分钟窗口起点
	gateUsed   int       // 本窗口已消耗的 doLogin 次数
	gateCond   *sync.Cond
}

// gateWait 申请一次 doLogin 预算：窗口内已用满则阻塞等待下一个窗口的广播
// （被调度的重登 goroutine 挂起而非取消，保证所有账号最终都能完成重登）。
// 广播后所有等待者重新竞争预算，每窗口严格不超过 gateLoginPerMin 次。
func (m *Manager) gateWait() {
	m.gateMu.Lock()
	defer m.gateMu.Unlock()
	for {
		now := time.Now()
		if now.Sub(m.gateWindow) >= time.Minute {
			m.gateWindow = now
			m.gateUsed = 0
		}
		if m.gateUsed < gateLoginPerMin {
			m.gateUsed++
			return
		}
		m.gateCond.Wait() // 预算耗尽：释放锁等待下个窗口广播
	}
}

// GatePump 每分钟窗口到点后广播，唤醒排队中的重登重新竞争预算。
// 由 main 的后台协程每 30 秒调用；无等待者时是空转，开销可忽略。
func (m *Manager) GatePump() {
	m.gateMu.Lock()
	defer m.gateMu.Unlock()
	if time.Since(m.gateWindow) < time.Minute {
		return
	}
	m.gateWindow = time.Now()
	m.gateUsed = 0
	m.gateCond.Broadcast()
}

// New 创建多账号客户端注册表。
func New(baseURL string, vision zhidao.VisionConfig, st Store) *Manager {
	m := &Manager{
		baseURL: baseURL,
		vision:  vision,
		st:      st,
		clients: make(map[string]*zhidao.Client),
	}
	m.gateCond = sync.NewCond(&m.gateMu)
	return m
}

// ensure 返回账号对应的独立客户端（不存在则创建空壳）。
func (m *Manager) ensure(acct string) *zhidao.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[acct]; ok {
		return c
	}
	c := zhidao.New(m.baseURL, m.vision)
	m.clients[acct] = c
	m.order = append(m.order, acct)
	return c
}

// ClientFor 返回指定账号的独立客户端。
func (m *Manager) ClientFor(acct string) (scheduler.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[acct]
	return c, ok
}

// Remove 移除账号客户端与登录顺序（管理员删除账号后调用）。
// 调度器据此不再为该账号生成提交链（submitAll 遍历时 ClientFor 返回不存在即跳过）。
// 已在跑的链不会被打断（生命周期归调度器 chains 标记管理），但下个 tick 起彻底隔离。
func (m *Manager) Remove(acct string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, acct)
	for i, a := range m.order {
		if a == acct {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

// AnyClient 返回任一已登录账号客户端（课程数据全校共享，任一账号可探测）。
func (m *Manager) AnyClient() (scheduler.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.order) == 0 {
		return nil, false
	}
	return m.clients[m.order[0]], true
}

// AnyClientWithAccount 返回任一已登录账号的客户端与账号名（失效时定位账号用）。
func (m *Manager) AnyClientWithAccount() (string, scheduler.Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.order) == 0 {
		return "", nil, false
	}
	acct := m.order[0]
	return acct, m.clients[acct], true
}

// gateLoginPerMin 全账号合计每分钟 doLogin 上限。平台登录限流实测「登录失败次数过多，
// 请 30 分钟后重试」按账号/IP 计数；2 次/分钟是保守下限，10 账号同时失效也不打爆 IP。
// 反代后跨 IP 共享配额，此闸门仍按服务出口 IP 收敛所有账号的登录流量。
const gateLoginPerMin = 2

// Relogin 对指定账号客户端执行自动重登（返回是否已重登与错误）。
// 走全局重登频率闸门：多账号并发重登时，实际触达平台 doLogin 的速率被收敛到
// gateLoginPerMin/分钟，超出预算的账号排队等待，杜绝把出口 IP 刷到平台锁号。
func (m *Manager) Relogin(acct string) (bool, error) {
	m.mu.Lock()
	c, ok := m.clients[acct]
	m.mu.Unlock()
	if !ok {
		return false, fmt.Errorf("账号 %s 未注册", acct)
	}
	m.gateWait()
	return c.ReloginIfNeeded()
}

// Registered 返回已注册账号（按登录顺序）。
func (m *Manager) Registered() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.order))
	copy(out, m.order)
	return out
}

// SetVision 热更新全部账号客户端的验证码识别配置（管理员运行时修改立即生效）。
func (m *Manager) SetVision(cfg zhidao.VisionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vision = cfg
	for _, c := range m.clients {
		c.SetVision(cfg)
	}
}

// SetRecognizer 热切换全部账号客户端的验证码识别引擎（ddddocr 本地 / Vision 二选一）。
// recognizer 为 nil 时表示"无引擎"（登录识别立即报错，直到管理员恢复配置）。
func (m *Manager) SetRecognizer(r zhidao.CaptchaRecognizer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.SetRecognizer(r)
	}
}

// LoginByPassword 用账密登录该账号独立客户端；成功后加密密码与 token 落库。
// 管理员入口（换绑定新账密）不受全局重登闸门约束，仍走平台登录接口。
func (m *Manager) LoginByPassword(acct, password string, encrypt func(string) (string, error)) (string, error) {
	c := m.ensure(acct)
	token, err := c.Login(acct, password)
	if err != nil {
		return "", err
	}
	if m.st != nil && encrypt != nil {
		if enc, err := encrypt(password); err == nil {
			if err := m.st.SaveCredential(acct, enc, token); err != nil {
				log.Printf("[accounts] 持久化凭据失败: %v", err)
			}
		}
	}
	return token, nil
}

// Restore 重启时用持久化凭据恢复各账号客户端（密码解密后在内存中，仅用于自动重登）。
func (m *Manager) Restore(creds []Credential, decrypt func(string) (string, error)) {
	for _, cd := range creds {
		c := m.ensure(cd.Account)
		pwd := ""
		if decrypt != nil {
			if p, err := decrypt(cd.PasswordEnc); err == nil {
				pwd = p
			}
		}
		c.SetCredentials(cd.Account, pwd, cd.IDToken)
		c.SetCookies(map[string]string{
			"access_limit_cookie": "***REMOVED***",
			"zd_edu_cookie":       cd.IDToken,
		})
		log.Printf("[accounts] 恢复账号 %s 的会话（token %s）", cd.Account, tokenShort(cd.IDToken))
	}
}

func tokenShort(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "..."
}
