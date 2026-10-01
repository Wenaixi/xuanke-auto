package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/api"
	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/secure"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/sites"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/upstream"
	"xuanke-auto/backend/web"
)

// Started 服务启动结果：监听地址 + 优雅关服入口 + 退出原因通道。
// 桌面 main 与安卓平台入口共用同一套引擎初始化，仅形态收尾不同。
type Started struct {
	// Addr 实际监听地址（如默认 "127.0.0.1:3091"）；供日志与服务启动后提示。
	Addr string
	// Rebind 热切换监听地址（管理员后台改 listen_host / listen_port 时调用）：
	// 新地址先绑定成功再关旧监听，失败即返回错误且旧监听继续服务。
	Rebind func(host, port string) error
	// Shutdown 优雅关服（等服务在飞请求结束），托盘「退出」与安卓销毁时调用。
	Shutdown func()
	// Cleanup 关闭 DB/会话等底层资源（Android 常驻共享库 main 永不返回，
	// 库内 defer 不触发，必须由 Java 销毁路径显式调用；桌面 main 的 defer 仍兜底）。
	Cleanup func()
	// ErrCh 服务退出原因（ErrServerClosed 表示被 Shutdown 主动关闭）。
	ErrCh <-chan error
}

// runServer 组装并启动整套服务引擎：数据库 / 凭据加密 / 多账号注册表 / 运行时配置
// / 调度器（开窗探测与按账号提交）/ 会话库 / API 路由 / 前端 SPA。
// 与桌面 main 的分工：本函数只管「引擎 + HTTP 服务」；托盘 / 开浏览器等桌面形态
// 专属物由 main 负责（安卓由 platform_android.go 的入口负责）。
func runServer(cfg config.Config) *Started {
	// 公网安全：管理口令必填（用于生成激活码），否则拒绝启动
	if cfg.AdminToken == "" {
		log.Fatal("未设置管理口令：请在 data/.env 中填写 XUANKE_ADMIN_TOKEN（或设置同名环境变量）后启动")
	}
	if cfg.SFAPIKey == "" {
		log.Println("[main] 警告：未设置 SF_API_KEY，教务登录验证码识别将不可用")
	}
	if cfg.ActivationCodesEnabled {
		log.Printf("[main] 激活码机制已启用（XUANKE_ACTIVATION=off 可完全关闭）")
	} else {
		log.Printf("[main] 激活码机制已关闭：账号登录后直接进入系统")
	}

	// d/err 生命周期归 Started.Cleanup（Android 常驻共享库：main 不返回，
	// 库内 defer 永不触发，DB/会话需随 Java 销毁显式关闭）。桌面 main 用 defer
	// 仍稳妥——Cleanup 由托盘退出路径调用，两形态共用同一收尾。
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	st := store.New(d)

	// 凭据加密主密钥（环境变量或 DB 旁 .master_key）
	masterKey, err := secure.LoadOrCreateKey(cfg.DBPath)
	if err != nil {
		log.Fatalf("初始化数据加密密钥失败: %v", err)
	}
	encrypt := func(s string) (string, error) { return secure.Encrypt(s, masterKey) }
	decrypt := func(s string) (string, error) { return secure.Decrypt(s, masterKey) }

	// 进程内配置中心（管理员可热重载：激活码开关 / Vision / 识别引擎与并发 /
	// 监听地址 / 选课平台档案与站点地址）。
	// env 侧 → 运行时配置中心的字段搬运收在 config.RuntimeSettings 一处
	// （两结构体字段名系统性错位，Go 编译器不校验跨结构体手抄，漏搬即静默丢
	// env 初值；覆盖面由 config 包的反射测试钉死）。此处只补 config 不提供的
	// 两项：识别引擎默认 ddddocr 本地识别（免密钥），并发上限恒 1 串行防熔断。
	rs := config.RuntimeSettings(cfg)
	rt := runtime.New(runtime.Config{
		ActivationEnabled:  rs.ActivationEnabled,
		VisionBaseURL:      rs.VisionBaseURL,
		VisionAPIKey:       rs.VisionAPIKey,
		VisionModel:        rs.VisionModel,
		CaptchaEngine:      config.CaptchaEngineDefault(), // 默认 ddddocr 本地识别（免密钥），vision 云识别需显式配置
		CaptchaConcurrency: 1,
		// 监听地址同样进运行时配置：管理员后台可热改（重绑 socket）并落库 settings。
		// 启动顺序 = env 初值 → 落库值覆盖（ApplySettings）→ 按最终值真正监听。
		ListenHost: rs.ListenHost,
		ListenPort: rs.ListenPort,
		// 选课平台档案：env 初值 → 落库值覆盖（非法 ID 由 runtime 配置表拒绝并保留既有值）。
		PlatformID:      rs.PlatformID,
		PlatformBaseURL: rs.PlatformBaseURL,
	})
	// 从数据库恢复管理员上次的运行时配置（优先于环境变量，覆盖持久化值）。
	// 键名与解析规则由 runtime 包的配置表统一定义——落库侧（api PUT）与本还原侧
	// 读同一张表，新增字段不可能只改一处而漏另一处。
	if kv, err := st.LoadSettings(); err != nil {
		log.Printf("[main] 读取运行时配置失败: %v", err)
	} else if len(kv) > 0 {
		rt.Update(func(c *runtime.Config) {
			runtime.ApplySettings(c, kv, decrypt)
		})
	}

	// 选课平台档案必须在建客户端之前解析：档案决定路径/键名/解码，是客户端的构造入参。
	// 未知 ID（只可能来自 .env，因为落库侧已被配置表把关）一律拒绝启动——
	// 显式报错好过静默跑在另一个站点上。
	rtCfg := rt.Get()
	site, err := resolveStartupPlatform(rtCfg)
	if err != nil {
		log.Fatalf("选课平台档案无效：%v（改 data/.env 的 XUANKE_PLATFORM 后重启）", err)
	}
	baseURL := sites.EffectiveBaseURL(site, rtCfg.PlatformBaseURL)
	log.Printf("[main] 选课平台：%s（%s，站点 %s）", site.Name, site.ID, baseURL)

	// 多账号客户端注册表（每账号独立会话）+ 重启恢复。
	// Restore 只恢复与本档案同源的会话 token（credentials.platform_id 比对），
	// 跨平台 token 一律丢弃（既防凭据外泄，也避免必然 401 白跑一轮）。
	vision := upstream.VisionConfig{BaseURL: cfg.SFBaseURL, APIKey: cfg.SFAPIKey, Model: cfg.SFModel}
	accts := accounts.New(site, baseURL, vision, st)
	if creds, err := st.LoadCredentials(); err != nil {
		log.Printf("[main] 读取凭据失败: %v", err)
	} else if len(creds) > 0 {
		accts.Restore(creds, decrypt)
	}

	// 调度器（窗口到点立即探测 + 课程快照 + 按账号并发提交）
	// 开放时间不做任何配置注入：平台 beginTimes 自动识别是唯一事实源（open_time 零值）。
	sched := scheduler.New(accts, st, time.Time{}, 300*time.Millisecond)
	// 启动期按当前档案同步开窗信号能力（New 默认 true，此处按档案纠正：
	// 无信号平台必须走退化模式，否则永远判不出开窗）。
	sched.SetHasWindowSignal(site.HasWindowSignal)

	// 平台档案热切换（管理员后台「系统配置 → 选课平台」）：闭包收进具名构造函数，
	// 让"解析档案 → 校验地址 → 读凭据 → 重建客户端"这条编排在测试里可直接断言。
	// onProfile 同步调度器的开窗信号能力与窗口状态——平台切换必须同步，否则会用
	// 旧站点的信号语义判新站点（切到无信号平台后开窗永远判不出来，调度器全线停摆），
	// 且会按旧站点的开放时间判新站点的窗口（识别槽无 TTL 兜底，永久残留）。
	rebindPlatform := newPlatformRebinder(st, accts, decrypt, func(desc upstream.SiteDescriptor) {
		sched.SetHasWindowSignal(desc.HasWindowSignal)
		sched.ResetWindowState()
	})

	if success, err := st.LoadSuccess(); err != nil {
		log.Printf("[main] 读取成功记录失败: %v", err)
	} else if len(success) > 0 {
		sched.RestoreDone(success)
	}
	for _, a := range accts.Registered() {
		ts, err := st.LoadTargetsForAccount(a)
		if err != nil {
			log.Printf("[main] 读取账号 %s 目标失败: %v", a, err)
			continue
		}
		if len(ts) > 0 {
			// 重启恢复目标用 RestoreTargets（不清 refused）——
			// 此前用 SetTargetsForAccount 会 delete refused + 删库行，重启后手动退选
			// 记录全丢、自动引擎重新抢回（恢复顺序抵消）。
			sched.RestoreTargets(a, ts)
		}
	}
	// 恢复已手动退选记录——RestoreTargets 不清库行，此处 LoadRefused
	// 仍能读到全部退选；RestoreRefused 注入内存后不被任何后续步骤覆盖。
	if refused, err := st.LoadRefused(); err != nil {
		log.Printf("[main] 读取已退选记录失败: %v", err)
	} else if len(refused) > 0 {
		sched.RestoreRefused(refused)
	}
	sched.Start()

	// 会话库（12 小时过期；后台周期清扫过期会话与票据）
	sessions := session.New(12 * time.Hour)

	// 全局重登频率闸门的分钟推进器：每 30 秒检查一次窗口翻页，翻页时放行队列中的重登。
	// 桌面与安卓共用此初始化路径，故闸门推进也统一放在这里（accts 在此创建）。
	go func() {
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		for range tk.C {
			accts.GatePump()
		}
	}()

	mux := http.NewServeMux()

	// 监听地址：runtime 配置优先（管理员后台热改与落库值），回退 env 启动值。
	// 默认 127.0.0.1（只允许本机）；内网 IP 开局域网、域名走穿透/公网、0.0.0.0 所有网卡。
	listenHost, listenPort := rtCfg.ListenHost, rtCfg.ListenPort
	if listenHost == "" {
		listenHost = "127.0.0.1"
	}
	if listenPort == "" {
		listenPort = cfg.Port
	}
	addr := net.JoinHostPort(listenHost, listenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// 监听失败即退出：起不来的服务静默存活比崩溃更难排查。
		log.Fatalf("服务启动失败（监听 %s）: %v", addr, err)
	}

	// http.Server 显式超时——公网部署时 slowloris/慢速 POST
	// 不再能占用 goroutine 与连接池饿死调度器 tick 与健康检查。
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	ch := make(chan error, 1)

	// currentLn 当前活跃监听：热重绑后旧监听的 Serve 会返回
	// "use of closed network connection"——那是正常路径，不能按启动失败处理
	// （否则管理员改一次监听地址就把进程打死）。只有"仍是当前监听的 Serve
	// 意外返回"才算服务真的挂了。
	var currentLn atomic.Value
	currentLn.Store(ln)
	var rebindMu sync.Mutex

	serve := func(l net.Listener) {
		serveErr := srv.Serve(l)
		if errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case ch <- http.ErrServerClosed:
			default:
			}
			return
		}
		if cur, ok := currentLn.Load().(net.Listener); ok && cur == l {
			log.Printf("[main] 监听服务意外退出: %v", serveErr)
			select {
			case ch <- serveErr:
			default:
			}
		}
	}

	// rebind 热切换监听：新地址先起来再关旧地址——新地址失败时旧服务照常可用，
	// 绝不出现"配置改了、服务没了"的空窗。
	rebind := func(host, port string) error {
		rebindMu.Lock()
		defer rebindMu.Unlock()
		newAddr := net.JoinHostPort(host, port)
		newLn, err := net.Listen("tcp", newAddr)
		if err != nil {
			return fmt.Errorf("监听 %s 失败: %w", newAddr, err)
		}
		old, _ := currentLn.Load().(net.Listener)
		currentLn.Store(newLn)
		go serve(newLn)
		if old != nil && old != newLn {
			_ = old.Close()
		}
		srv.Addr = newAddr
		log.Printf("[main] 监听地址已热切换: http://%s", net.JoinHostPort(displayHost(host), port))
		return nil
	}

	apiHandler := api.Register(api.Options{
		Mux: mux, Store: st, Sched: sched, Accounts: accts, Sessions: sessions,
		AdminToken: cfg.AdminToken, AdminName: cfg.AdminName,
		Encrypt: encrypt, Runtime: rt,
		PlatformEmbedded: cfg.PlatformEmbedded, Rebind: rebind, RebindPlatform: rebindPlatform,
	})
	mux.Handle("/", web.SpaHandler())
	srv.Handler = apiHandler

	log.Printf("[main] 自动选课服务启动: http://%s（激活码机制: %v）", net.JoinHostPort(displayHost(listenHost), listenPort), cfg.ActivationCodesEnabled)
	log.Printf("[main] 管理员登录：账号 %s，口令见 data/.env 的 XUANKE_ADMIN_TOKEN", adminNameOrDefault(cfg.AdminName))

	started := &Started{Addr: addr, Rebind: rebind}
	started.Cleanup = func() {
		// DB 与会话库的关闭收口：桌面死库 defer 与安卓 Java 销毁都会到达这里。
		// 幂等（多次调仅首回收）——sessions.Close 与 d.Close 内部都保证幂等。
		sessions.Close()
		d.Close()
	}
	started.Shutdown = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[main] 服务优雅关闭异常: %v", err)
		}
	}
	started.ErrCh = ch
	go serve(ln)
	return started
}

// resolveStartupPlatform 解析启动期的选课平台档案：空 ID 按默认档案
// （config.Load 恒注入；直构 config.Config 的嵌入式/测试调用给空值时不炸），
// **未知 ID 一律返回错误**——调用方 log.Fatalf 拒绝启动，绝不静默换到别的站点。
func resolveStartupPlatform(cfg runtime.Config) (upstream.SiteDescriptor, error) {
	id := cfg.PlatformID
	if id == "" {
		id = sites.DefaultID
	}
	// 档案自检钉在装配面（sites.ResolveValidated 单点）：一份缺路径或缺解码钩子的档案会让
	// 运行时在第一次请求时才炸（缺钩子是 nil 函数调用硬崩，缺路径拼出无前导斜杠 URL
	// 让平台 404），现场只是"未知的解析错误"。故此处拦下并由调用方 log.Fatalf 拒绝启动。
	return sites.ResolveValidated(id)
}

// newPlatformRebinder 组装平台热切换闭包（api PUT 的 RebindPlatform 依赖）。
// 契约：**先验证后生效**——未知档案 ID、非法站点地址、读凭据失败一律返回错误（调用方
// 整体拒绝、不落库），绝不留"档案换了、客户端没换"的半生效状态；
// 成功才把新档案与地址交给 SetProfile（重建客户端 + 清跨平台 token + 落库打新标记）。
func newPlatformRebinder(
	st *store.Store,
	accts *accounts.Manager,
	decrypt func(string) (string, error),
	// onProfile 档案切换成功后的额外编排（可为 nil）。调度器经此同步
	// hasWindowSignal——**平台切换必须同步它**，否则会用旧站点的开窗信号语义判新站点：
	// 从"下发in_date_range"的档案切到"不下发"的档案后，若仍按"必须见到 selectable=true"
	// 判定，开窗将永远判不出来，调度器全线停摆。
	onProfile func(desc upstream.SiteDescriptor),
) func(platformID, baseURL string) error {
	return func(platformID, baseURL string) error {
		desc, err := sites.ResolveValidated(platformID)
		if err != nil {
			return err
		}
		if err := upstream.ValidateBaseURL(baseURL); err != nil {
			return err
		}
		creds, err := st.LoadCredentials()
		if err != nil {
			return fmt.Errorf("读取账号凭据失败: %w", err)
		}
		accts.SetProfile(desc, sites.EffectiveBaseURL(desc, baseURL), creds, decrypt)
		if onProfile != nil {
			onProfile(desc)
		}
		return nil
	}
}


// displayHost 提示/日志用的主机名：0.0.0.0 / :: / 空（监听所有网卡）显示成
// localhost，其余照实显示（127.0.0.1 / 内网 IP / 域名）。
func displayHost(listenHost string) string {
	switch listenHost {
	case "", "0.0.0.0", "::", "[::]":
		return "localhost"
	}
	return listenHost
}

// adminNameOrDefault 管理员账号名（配置为空时默认 admin）。
func adminNameOrDefault(name string) string {
	if name == "" {
		return "admin"
	}
	return name
}
