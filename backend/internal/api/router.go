package api

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/zhidao"
)

// captchaSemInit 验证码识别并发信号量初始化标记（仅在首次 Register 时初始化一次）。
// 全局信号量生命周期与进程等同，重复 Register（测试）不重复初始化。
var captchaSemInit bool

// initCaptchaAtStartup 初始化识别并发信号量（默认并发 1），并按当前引擎预置识别器。
// 幂等只护"并发信号量"（进程级单例）；识别引擎注入 applyCaptchaRecognizerFor 必须
// **每次 Register 都跑**——每个 Register 的 accounts.Manager 是自己的实例，若被
// captchaSemInit 短路跳掉，第二个及以后的 Register 的 Manager 模板零识别器，
// ensure 新建客户端识别器恒 nil（"验证码识别器未初始化"）。此缺陷在删
// zhidao.New 静默建 Vision 旁路后被暴露（此前客户端级旁路掩盖了模板级缺失）。
func initCaptchaAtStartup(rt *runtime.Store, accts *accounts.Manager) {
	if !captchaSemInit {
		captchaSemInit = true
		cfg := rt.Get()
		zhidao.NewCaptchaSemaphore(cfg.CaptchaConcurrency)
	}
	applyCaptchaRecognizerFor(rt, accts)
}

// applyCaptchaRecognizerFor 按运行时配置切换识别引擎与并发（handleAdminConfig 热更新时同样调用）。
// 引擎解析交给纯函数 resolveCaptchaRecognizer，本函数只负责探测函数注入与副作用落地。
func applyCaptchaRecognizerFor(rt *runtime.Store, accts *accounts.Manager) {
	cfg := rt.Get()
	r := resolveCaptchaRecognizer(cfg, zhidao.NativeDdddOcrAvailable, func() bool {
		return zhidao.LocalDdddOcrAvailable("")
	})
	if r.recognizer == nil {
		// 无引擎：登录识别立即报错（配置的引擎不可用且未开启兜底），
		// 日志必须给出可执行的修复动作，不能只报"不可用"。
		log.Printf("[api] %s", r.note)
	} else {
		log.Printf("[api] 验证码识别引擎：%s", r.note)
	}
	setCaptchaActiveEngine(r.engine)
	accts.SetRecognizer(r.recognizer)
	zhidao.SetCaptchaConcurrency(cfg.CaptchaConcurrency)
}

// captchaResolution 引擎解析结果：识别器 + 实际生效引擎名 + 面向管理员的日志说明。
type captchaResolution struct {
	recognizer zhidao.CaptchaRecognizer
	// engine 实际生效引擎："ddddocr" / "vision" / "none"（无可用引擎）。
	// 与配置值 CaptchaEngine 分列：兜底关闭时配置 ddddocr 而本机不可用会出现
	// "配置=ddddocr、实际=none"，stats 只报配置值会让管理员误以为识别正常。
	engine string
	note   string
}

// resolveCaptchaRecognizer 识别引擎解析（下沉后为 zhidao.ResolveCaptchaEngine 的薄适配：
// 组装 EngineConfig + 注入探测函数 + 把 EngineResolution 转回 api 包内 captchaResolution）。
// 契约（兜底开关、双引擎互不回退、探测成本）全在 zhidao 单源，表驱动测试在 zhidao/engine_test.go。
func resolveCaptchaRecognizer(
	cfg runtime.Config,
	hasNative func() bool,
	hasLocal func() bool,
) captchaResolution {
	r := zhidao.ResolveCaptchaEngine(zhidao.EngineConfig{
		VisionBaseURL:   cfg.VisionBaseURL,
		VisionAPIKey:    cfg.VisionAPIKey,
		VisionModel:     cfg.VisionModel,
		CaptchaEngine:   cfg.CaptchaEngine,
		CaptchaFallback: cfg.CaptchaFallback,
	}, hasNative, hasLocal)
	return captchaResolution{recognizer: r.Recognizer, engine: r.Engine, note: r.Note}
}

// captchaActive 最近一次引擎解析的实际生效引擎（"ddddocr"/"vision"/"none"，空=尚未解析）。
// 管理端 stats 用它取代配置值展示运行真相——兜底关闭后"配置 ddddocr 而实际无引擎"
// 成为可达状态，只报配置值会误导管理员以为识别正常。
var (
	captchaActiveMu     sync.RWMutex
	captchaActiveEngine string
)

// CaptchaActiveEngine 返回实际生效引擎（供 /api/admin/stats 读取）。
func CaptchaActiveEngine() string {
	captchaActiveMu.RLock()
	defer captchaActiveMu.RUnlock()
	return captchaActiveEngine
}

func setCaptchaActiveEngine(e string) {
	captchaActiveMu.Lock()
	defer captchaActiveMu.Unlock()
	captchaActiveEngine = e
}

// jsonContentType 检查请求体是否为 JSON（反跨站表单 POST 的 CSRF 缓解）。
// 前端统一用 fetch+JSON，必带 application/json；跨站表单提交是
// application/x-www-form-urlencoded，无法伪造该头 → 直接 403 拒绝副作用请求。
// admin 的 PUT/DELETE（config 热改、codes 生成/删除、账号删除）同样复用此检查，
// 防止管理员接口被跨站表单 POST 挟持。
func jsonContentType(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

// requireJSONBody 复用 jsonContentType 拒绝非 JSON 提交的副作用请求。
// 拒绝分支必须写真实 HTTP 403（与登录/激活两处 CSRF 门同款）：安全扫描/反代需要在
// HTTP 层识别被 CSRF 拒的副作用请求，恒 200 会让监控与安全工具漏判。
func requireJSONBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !jsonContentType(r) {
			writeJSONStatus(w, http.StatusForbidden, 403, nil, "仅接受 JSON 提交")
			return
		}
		next(w, r)
	}
}

// Register 注册所有 API 路由到 mux，并返回包装了安全中间件的根 handler。
// 装配经 Options 配置对象（架构深化 E）——字段即语义，替代 11 位置参数。
func Register(opts Options) http.Handler {
	// 解包为局部变量——函数体既有 100+ 行直接引用 mux/rt/accts 等原名，
	// 保持内部引用零改动（架构深化 E 只换装配面，不扰动路由注册体）。
	mux, rt, accts := opts.Mux, opts.Runtime, opts.Accounts

	d := &Deps{Store: opts.Store, Sched: opts.Sched, Accounts: opts.Accounts, Sessions: opts.Sessions,
		Runtime: opts.Runtime, AdminToken: opts.AdminToken, ActivationEnabled: opts.ActivationEnabled,
		Encrypt: opts.Encrypt, AdminName: opts.AdminName}
	// 登录与激活各自独立限流桶——激活码输入错误不消耗登录额度、
	// 登录尝试不消耗激活额度；且各自按（IP 维度）独立记账，学校 NAT/反代下互不锁死。
	loginLim := newLoginLimiter()
	activateLim := newLoginLimiter()

	// 启动即按运行时配置初始化验证码识别引擎与并发信号量（幂等）
	initCaptchaAtStartup(rt, accts)

	mux.HandleFunc("GET /api/health", d.handleHealth)
	// 登录接口限流（激活码已取代部署口令 gate；未启用激活码机制时登录即发会话）
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		// CSRF 缓解：仅接受 JSON 提交（跨站表单 POST 无法伪造该头）
		if !jsonContentType(r) {
			writeJSONStatus(w, http.StatusForbidden, 403, nil, "仅接受 JSON 提交")
			return
		}
		if !loginLim.allow(clientIP(r)) {
			// 限流是真实 429，必须写 HTTP 状态码（此前恒 200）。
			writeJSONStatus(w, http.StatusTooManyRequests, 429, nil, "登录尝试过于频繁，请稍后再试")
			return
		}
		d.handleLogin(w, r)
	})
	// 激活接口（登录后未激活才需要，未认证；机制关闭时 handler 直接拒绝）
	// 激活用独立限流桶——攻击者刷激活码不会消耗他人登录额度，反之亦然
	mux.HandleFunc("POST /api/activate", func(w http.ResponseWriter, r *http.Request) {
		if !jsonContentType(r) {
			writeJSONStatus(w, http.StatusForbidden, 403, nil, "仅接受 JSON 提交")
			return
		}
		if !activateLim.allow(clientIP(r)) {
			// 激活限流同为真实 429，写 HTTP 状态码。
			writeJSONStatus(w, http.StatusTooManyRequests, 429, nil, "激活尝试过于频繁，请稍后再试")
			return
		}
		d.handleActivate(w, r)
	})
	// 激活码管理接口（会话级管理员鉴权）；生成/删除是副作用请求，强制 JSON
	mux.HandleFunc("GET /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminCodes)(w, r)
	})
	// 生成/列表/删除三态同级路由。此前 DELETE 也强制 requireJSONBody，
	// 但标准 REST 客户端 DELETE 默认无 body（Content-Type 缺失）→ 被 403 拒——管理员用
	// curl/脚本删除激活码必然踩坑，且 DELETE 分支本就接受空 body（"无副作用无需强制 JSON"）。
	// POST 仍是副作用+需要 body（强制 JSON 防跨站表单挟持），GET/DELETE 放行空 body。
	mux.HandleFunc("POST /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, requireJSONBody(d.handleAdminCodes))(w, r)
	})
	mux.HandleFunc("DELETE /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminCodes)(w, r)
	})
	// 管理员后台：配置热重载 / 运行状态 / 账号管理 / 日志总览（会话级管理员鉴权）
	mux.HandleFunc("GET /api/admin/config", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminConfig)(w, r)
	})
	mux.HandleFunc("PUT /api/admin/config", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, requireJSONBody(d.handleAdminConfig))(w, r)
	})
	mux.HandleFunc("GET /api/admin/stats", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminStats)(w, r)
	})
	mux.HandleFunc("GET /api/admin/accounts", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminAccounts)(w, r)
	})
	mux.HandleFunc("DELETE /api/admin/accounts", func(w http.ResponseWriter, r *http.Request) {
		// 与 codes 的 DELETE 对称——账号删除同为"DESTROY + 空 body 合法"的 REST 语义，
		// 标准客户端 curl/Postman/脚本 DELETE 默认无 body（Content-Type 缺失）→
		// requireJSONBody 会 403 拒。去掉 JSON 门，空 body 由 handler 解码失败返回
		// 明确业务错误；前端始终带 JSON body 不受影响。
		requireAdminSession(d, d.handleAdminDeleteAccount)(w, r)
	})
	mux.HandleFunc("GET /api/admin/logs", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminLogs)(w, r)
	})
	// 其余接口全部要求会话认证；报名/退选/目标设置等副作用请求同样强制 JSON
	mux.HandleFunc("GET /api/electives", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleElectives)(w, r)
	})
	mux.HandleFunc("POST /api/electives/select", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, requireJSONBody(d.handleElectiveSelect))(w, r)
	})
	mux.HandleFunc("POST /api/electives/select/exit", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, requireJSONBody(d.handleElectiveExit))(w, r)
	})
	mux.HandleFunc("PUT /api/targets", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, requireJSONBody(d.handleSetTargets))(w, r)
	})
	mux.HandleFunc("GET /api/accounts", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleAccounts)(w, r)
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleState)(w, r)
	})
	mux.HandleFunc("GET /api/logs", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleLogs)(w, r)
	})
	// 登出接口——会话级鉴权，立即吊销服务端令牌（防止令牌外流残留）
	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleLogout)(w, r)
	})
	// 未知 /api/ 路径显式 404——此前未注册的 /api/xxx 落入
	// main.go 的 mux.Handle("/", SpaHandler) 兜底，返回 index.html（HTTP 200 text/html）：
	// 前端 fetch 拿到 200 HTML 解析 JSON 报错掩盖真实 404，且被安全扫描误判"任意路径可 200"。
	// 显式注册 "/api/" 前缀后，所有已注册的精确 method+pattern 优先命中，未匹配的 /api/xxx
	// 一律 HTTP 404 + JSON body，绝不再回退 SPA。
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// writeJSONStatus 家族：真实 HTTP 404 + JSON body——必须先设头再
		// WriteHeader（writeJSON 在 WriteHeader 后设 CT 会被 net/http 丢弃，真实
		// Server 上 404 错标 text/plain，httptest.ResponseRecorder 测试路径假绿掩盖）；与 401/403/429/500 同族对齐。
		writeJSONStatus(w, http.StatusNotFound, 404, nil, "接口不存在")
	})

	return recoverMiddleware(securityHeaders(mux))
}
