package accounts

import (
	"fmt"
	"log"
	"sync"
	"time"

	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/upstream"
)

// Store 凭据持久化最小接口（store.Store 实现）。
// SaveCredential 一次写齐「密文账密 + token + token 所属平台」——切换平台时正是靠
// 「空 token + 新平台 ID」的同一次写入清掉跨平台会话并打上新标记。
type Store interface {
	SaveCredential(acct, passwordEnc, idToken, platformID string) error
	LoadCredentials() ([]Credential, error)
}

// Credential 账号凭据（与 store.Credential 字段一一对应，避免包循环）。
type Credential struct {
	Account     string
	PasswordEnc string
	IDToken     string
	// PlatformID token 所属的平台档案 ID。与当前档案不一致时 token 必须丢弃：
	// 跨平台会话必废，且绝不把 A 平台的会话 token 发往 B 平台（凭据外泄）。
	PlatformID string
}

// Manager 多账号客户端注册表：每个账号一个独立 upstream.Client（独立 token/cookie 会话）。
// 账号 A 的请求绝不携带账号 B 的会话——物理隔离的核心。
type Manager struct {
	desc    upstream.SiteDescriptor // 当前平台档案（路径/键名/解码钩子）
	baseURL string                  // 实际生效的站点根地址（管理员覆盖优先）
	vision  upstream.VisionConfig
	st      Store

	mu      sync.Mutex
	clients map[string]*upstream.Client // 账号名 -> 独立客户端
	order   []string                    // 登录顺序

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

// New 创建多账号客户端注册表。baseURL 空串表示用档案默认地址。
func New(desc upstream.SiteDescriptor, baseURL string, vision upstream.VisionConfig, st Store) *Manager {
	if baseURL == "" {
		baseURL = upstream.NormalizeBaseURL(desc.DefaultBaseURL)
	}
	m := &Manager{
		desc:    desc,
		baseURL: baseURL,
		vision:  vision,
		st:      st,
		clients: make(map[string]*upstream.Client),
	}
	m.gateCond = sync.NewCond(&m.gateMu)
	return m
}

// profileID 当前档案 ID（读锁快照：SetProfile 会换档案，落库前必须取当前值）。
func (m *Manager) profileID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.desc.ID
}

// sessionCookies 当前档案声明的会话 Cookie 占位（读锁快照）。
// 站点会话 Cookie 属档案事实（某站需频次标记 Cookie，无此需求的平台声明空 map），
// accounts 层只消费不硬编码——换平台时随档案自动改变。
func (m *Manager) sessionCookies() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.desc.SessionCookies
}

// ensure 返回账号对应的独立客户端（不存在则创建空壳）。
func (m *Manager) ensure(acct string) *upstream.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[acct]; ok {
		return c
	}
	c := upstream.New(m.desc, m.baseURL, m.vision)
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

// removeLocked 从注册表摘除账号客户端与登录顺序。**调用者必须已持 m.mu**——
// 两处消费（管理员删号的 Remove、登录失败清理空壳的 LoginByPassword）都已在
// 锁内调用，拆成独立方法时不得各自再加一层锁。
func (m *Manager) removeLocked(acct string) {
	delete(m.clients, acct)
	for i, a := range m.order {
		if a == acct {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

// Remove 移除账号客户端与登录顺序（管理员删除账号后调用）。
// 调度器据此不再为该账号生成提交链（submitAll 遍历时 ClientFor 返回不存在即跳过）。
// 已在跑的链不会被打断（生命周期归调度器 chains 标记管理），但下个 tick 起彻底隔离。
func (m *Manager) Remove(acct string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(acct)
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

// UpdateCaptchaEngine 单一出口原子更新识别管线（替代原 SetVision + SetRecognizer 分步调用）。
// 关键纪律：探测不可持锁——锁外执行无锁决策解析（耗时本地探测绝不卡锁），锁内单次持锁原子同步模板与全部客户端。
func (m *Manager) UpdateCaptchaEngine(cfg upstream.EngineConfig) upstream.EngineResolution {
	res := upstream.ResolveCaptchaEngine(cfg, upstream.NativeDdddOcrAvailable, func() bool {
		return upstream.LocalDdddOcrAvailable("")
	})
	vc := upstream.VisionConfig{
		BaseURL: cfg.VisionBaseURL,
		APIKey:  cfg.VisionAPIKey,
		Model:   cfg.VisionModel,
	}.WithRecognizer(res.Recognizer)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.vision = vc
	for _, c := range m.clients {
		c.UpdateCaptcha(res.Recognizer, vc)
	}
	return res
}

// SetVision 热更新全部账号客户端的验证码识别配置（管理员运行时修改立即生效）。
// 赋值 m.vision 前先保留模板当前引擎——dispatchRuntimeConfig 先
// SetVision 再 applyCaptchaRecognizerFor(SetRecognizer)，若 SetVision 直接覆盖模板，
// 两条调用之间新 ensure 的客户端会短暂拿到 nil 引擎；保留当前引擎与
// upstream.Client.SetVision 的"绝不挥动引擎切换"语义对齐（引擎归属 SetRecognizer）。
func (m *Manager) SetVision(cfg upstream.VisionConfig) {
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
// upstream.New(m.baseURL, m.vision) 时 recognizer 拿不到引擎，默认兜底仅认 APIKey
// （SF_API_KEY 留空的 ddddocr 部署下），新账号登录识别直接报"未配置验证码识别引擎"，
// 系统从第一个新账号起无法登录任何新账号（既有客户端因已注入引擎被掩盖）。同理
// SetVision 赋值前保留当前引擎，绝不把模板的 recognizer 清成 nil。
func (m *Manager) SetRecognizer(r upstream.CaptchaRecognizer) {
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
			m.removeLocked(acct)
			m.mu.Unlock()
		}
		return "", err
	}
	if m.st != nil && encrypt != nil {
		if enc, err := encrypt(password); err == nil {
			if err := m.st.SaveCredential(acct, enc, token, m.profileID()); err != nil {
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
// **跨平台会话丢弃**：库内 token 若属于别的平台档案（换过平台、或恢复了他机备份的库），
// 一律不注入——否则会把 A 平台的会话 token 发往 B 平台（凭据外泄），且必然 401 白跑一轮。
// 账密照常注入，调度器随即自动重登建新会话。
// SetCredentials 已写入档案声明的会话 Cookie；SetCookies 为合并语义，
// 只补齐档案声明的占位 Cookie，绝不覆盖登录流程收集的服务端会话 Cookie。
func (m *Manager) Restore(creds []Credential, decrypt func(string) (string, error)) {
	cur := m.profileID()
	cookies := m.sessionCookies()
	for _, cd := range creds {
		c := m.ensure(cd.Account)
		token := cd.IDToken
		if token != "" && cd.PlatformID != cur {
			log.Printf("[accounts] 账号 %s 的会话属于平台 %s，与当前平台 %s 不一致，已丢弃（将自动重登）",
				cd.Account, cd.PlatformID, cur)
			token = ""
		}
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
		c.SetCredentials(cd.Account, pwd, token)
		// 会话 Cookie 占位由档案声明（某站需频次标记 Cookie；无需求的平台空 map）。
		// 只做占位补充，不覆盖真实会话值（合并语义由 SetCookies 保证）。
		c.SetCookies(cookies)
		log.Printf("[accounts] 恢复账号 %s 的会话（token %s）", cd.Account, tokenShort(token))
	}
}

// SetProfile 管理员切换选课平台档案后重建全部客户端。
//
// 语义（顺序即契约）：
//  1. **memory-first**：先换档案与生效地址、整表清空注册表——旧客户端立即不可达，
//     在飞链的锁内身份复核随之失败并静默放弃落库（复用「删号同名重建」那条防线），
//     在飞结果绝不污染新平台状态；
//  2. 逐账号用库内密文账密重建客户端，**token 一律置空**——跨平台会话必废，
//     且物理上不存在「A 平台 token 发往 B 平台」的可能；
//  3. 每账号落库一次 SaveCredential（空 token + 新平台 ID）：同一次写入既清掉库内旧
//     token（防重启后把旧平台 token 当自己的用），又打上新平台标记。
//
// 目标课程/成功记录/已退选记录**刻意不动**：换平台是同一套选课语义的接口换代，
// 这些记录仍然有效；只有会话与站点地址属于平台。
func (m *Manager) SetProfile(desc upstream.SiteDescriptor, baseURL string, creds []Credential, decrypt func(string) (string, error)) {
	if baseURL == "" {
		baseURL = upstream.NormalizeBaseURL(desc.DefaultBaseURL)
	}
	m.mu.Lock()
	m.desc, m.baseURL = desc, baseURL
	m.clients = make(map[string]*upstream.Client)
	m.order = nil
	m.mu.Unlock()

	for _, cd := range creds {
		c := m.ensure(cd.Account)
		pwd := ""
		if decrypt != nil {
			if p, err := decrypt(cd.PasswordEnc); err == nil {
				pwd = p
			} else {
				log.Printf("[accounts] 账号 %s 凭据解密失败，切换平台后需手动登录: %v", cd.Account, err)
			}
		}
		c.SetCredentials(cd.Account, pwd, "")
		c.SetCookies(desc.SessionCookies)
		if m.st != nil {
			if err := m.st.SaveCredential(cd.Account, cd.PasswordEnc, "", desc.ID); err != nil {
				// 落库失败绝不静默：库内残留旧平台 token，重启时会被 Restore 丢弃（防线仍在），
				// 但管理员必须能从日志看到"这次切换没完全落库"。
				log.Printf("[accounts] 切换平台后更新账号 %s 凭据失败: %v", cd.Account, err)
			}
		}
	}
	log.Printf("[accounts] 已切换选课平台档案 %s（%s）：%d 个账号客户端已重建，将自动重登", desc.ID, baseURL, len(creds))
}

func tokenShort(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:8] + "..."
}
