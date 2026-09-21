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

	// 全局重登频率闸门（安全审计）：所有账号共享同一出口 IP 打平台 doLogin，
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

// ResetGateForTest 测试专用：清空全局重登频率闸门计数与窗口起点。
// api 层测试夹具 authenticateDirect（语义"不关心登录流程，仅注册账号建会话"）在同一分钟
// 窗口内连续注册多个账号用，避免夹具被闸门预算误拦；正式代码不调用。
func (m *Manager) ResetGateForTest() {
	m.gateMu.Lock()
	defer m.gateMu.Unlock()
	m.gateWindow = time.Time{}
	m.gateUsed = 0
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
// 赋值 m.vision 前先保留模板当前引擎——dispatchRuntimeConfig 先
// SetVision 再 applyCaptchaRecognizerFor(SetRecognizer)，若 SetVision 直接覆盖模板，
// 两条调用之间新 ensure 的客户端会短暂拿到 nil 引擎；保留当前引擎与
// zhidao.Client.SetVision 的"绝不挥动引擎切换"语义对齐（引擎归属 SetRecognizer）。
func (m *Manager) SetVision(cfg zhidao.VisionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg = cfg.WithRecognizer(m.vision.Recognizer()) // 保留模板当前引擎
	m.vision = cfg
	for _, c := range m.clients {
		c.SetVision(cfg)
	}
}

// SetRecognizer 热切换全部账号客户端的验证码识别引擎（ddddocr 本地 / Vision 二选一）。
// recognizer 为 nil 时表示"无引擎"（登录识别立即报错，直到管理员恢复配置）。
// 同时写入 m.vision.recognizer 模板——否则 SetRecognizer 只注入
// 当前已有客户端，m.vision 模板的 recognizer 恒为 nil：此后 ensure 新建客户端经
// zhidao.New(m.baseURL, m.vision) 时 recognizer 拿不到引擎，默认兜底仅认 APIKey
// （SF_API_KEY 留空的 ddddocr 部署下），新账号登录识别直接报"未配置验证码识别引擎"，
// 系统从第一个新账号起无法登录任何新账号（既有客户端因已注入引擎被掩盖）。同理
// SetVision 赋值前保留当前引擎，绝不把模板的 recognizer 清成 nil。
func (m *Manager) SetRecognizer(r zhidao.CaptchaRecognizer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vision = m.vision.WithRecognizer(r)
	for _, c := range m.clients {
		c.SetRecognizer(r)
	}
}

// gateTryAcquire 非阻塞申请一次 doLogin 预算（tokenBucket 语义借用 gateWait 同款计数）：
// 窗口内 quota 充足则消耗并返回 true，已满则返回 false——绝不阻塞等待下个窗口。
// 学生手动登录（LoginByPassword）用它收口全局 doLogin 频率闸门：窗口已满时立即拒绝
// （提示稍后再试），不挂起用户登录响应（排队重登可能数分钟）；与 gateWait 共享同一
// gateMu 与 gateUsed 计数，排队重登与手动登录严格共享全账号每分钟 doLogin 预算。
func (m *Manager) gateTryAcquire() bool {
	m.gateMu.Lock()
	defer m.gateMu.Unlock()
	if time.Since(m.gateWindow) >= time.Minute {
		m.gateWindow = time.Now()
		m.gateUsed = 0
	}
	if m.gateUsed < gateLoginPerMin {
		m.gateUsed++
		return true
	}
	return false
}

// LoginByPassword 用账密登录该账号独立客户端；成功后加密密码与 token 落库。
// doLogin 前先经全局频率闸门非阻塞准入：窗口内预算已满立即返回明确错误，
// 绝不放行直发平台 doLogin——多账号集中失效自动重登排队时，任一学生手动重登与排队
// 重登并发触达平台，N+1 并发可刷爆出口 IP（平台"登录失败次数过多"按 IP 计数）。
// 管理员入口（换绑定新账密）同样收口到此闸门：换绑是低频操作，被拦一次重试即可，
// 统一收敛更安全（管理员登录本身走 handleLogin 单独分支，不触碰教务登录不受影响）。
func (m *Manager) LoginByPassword(acct, password string, encrypt func(string) (string, error)) (string, error) {
	if !m.gateTryAcquire() {
		return "", fmt.Errorf("登录尝试过于频繁，请稍后再试")
	}
	c := m.ensure(acct)
	// 判别本次是不是"纯新建的空壳"再决定失败清理——
	// 需在 Login 前快照，因为 Login 成功分支会 SetCredentials 写 token，失败返回时
	// 无法再区分"本次新建"与"此前已持有效 token 的既有客户端"（此前清理无判别
	// 直接摘除，会误删后者：自动抢课静默停摆 + 该账号选课大厅持续报错，直到手动
	// 重新登录成功——黄金期手滑输错密码即全程失联）。
	wasShell := c.Token() == ""
	token, err := c.Login(acct, password)
	if err != nil {
		// 登录失败残留空 token 客户端抢占核心账号位——ensure 已把该
		// 账号写入注册表（clients+order 首位），但空 token（无账密）客户端对 FindElectives/
		// AnyClientWithAccount 恒返回 code=-1 ErrUnauthorized：调度器 probe() 主体每次探测
		// 都触发 maybeRelogin("order[0]") → ReloginIfNeeded 报"未登录且无保存账密"、reloginFail
		// 递增，lastData 永不刷新，未配置目标的全校浏览视图持续报错，直到该账号成功登录。
		// 失败只摘除"本次新建的空壳"（原注册表里无此账号）；若注册表里已躺着持有效 token
		// 的工作客户端（重启 Restore/此前登录成功注册），本次失败绝不误删——旧 token 是否
		// 失效交给调度器现有失效检测 + 自动重登链处理，远优于直接失联。
		if wasShell {
			m.mu.Lock()
			delete(m.clients, acct)
			for i, a := range m.order {
				if a == acct {
					m.order = append(m.order[:i], m.order[i+1:]...)
					break
				}
			}
			m.mu.Unlock()
		}
		return "", err
	}
	if m.st != nil && encrypt != nil {
		if enc, err := encrypt(password); err == nil {
			if err := m.st.SaveCredential(acct, enc, token); err != nil {
				log.Printf("[accounts] 持久化凭据失败: %v", err)
			}
		} else {
			// 加密失败：内存登录已成功但凭据不落库——重启 Restore 无账密、自动重登
			// 永久"无保存账密"，与决策锚 17 零吞错对称，必须留痕（主密钥损坏启动即拒，
			// 此处触达意味着运行期加密器异常，日志是唯一审计线索）。
			log.Printf("[accounts] 账号 %s 密码加密失败，凭据未落库（自动重登将无保存账密）: %v", acct, err)
		}
	}
	return token, nil
}

// Restore 重启时用持久化凭据恢复各账号客户端（密码解密后在内存中，仅用于自动重登）。
// SetCredentials 已写入 zd_edu_cookie（token）；SetCookies 为合并语义，
// 只补齐 access_limit_cookie，绝不覆盖登录流程收集的服务端会话 Cookie。
func (m *Manager) Restore(creds []Credential, decrypt func(string) (string, error)) {
	for _, cd := range creds {
		c := m.ensure(cd.Account)
		pwd := ""
		if decrypt != nil {
			if p, err := decrypt(cd.PasswordEnc); err == nil {
				pwd = p
			} else {
				// 解密失败（主密钥变更后旧密文不可解）：自动重登将"无保存账密"，与
				// LoginByPassword 加密失败留痕对称——运行期主密钥不可变，
				// 此处触达意味着存储被外部改写/降级，日志是唯一排查线索。
				log.Printf("[accounts] 账号 %s 凭据解密失败，自动重登将无保存账密: %v", cd.Account, err)
			}
		}
		c.SetCredentials(cd.Account, pwd, cd.IDToken)
		// access_limit_cookie 由 login/submitLogin 动态更新；这里只做占位补充，
		// 不写死覆盖真实会话值（合并语义由 SetCookies 保证）。
		c.SetCookies(map[string]string{
			"access_limit_cookie": "1",
		})
		log.Printf("[accounts] 恢复账号 %s 的会话（token %s）", cd.Account, tokenShort(cd.IDToken))
	}
}

func tokenShort(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:8] + "..."
}
