package api

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

// captchaSemInit 验证码识别并发信号量初始化标记（仅在首次 Register 时初始化一次）。
// 全局信号量生命周期与进程等同，重复 Register（测试）不重复初始化。
var captchaSemInit bool

// initCaptchaAtStartup 初始化识别并发信号量（默认并发 1），并按当前引擎预置识别器。
// 多次调用幂等：仅首次生效。
func initCaptchaAtStartup(rt *runtime.Store, accts *accounts.Manager) {
	if captchaSemInit {
		return
	}
	captchaSemInit = true
	cfg := rt.Get()
	zhidao.NewCaptchaSemaphore(cfg.CaptchaConcurrency)
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

// resolveCaptchaRecognizer 引擎解析纯函数（无副作用、探测函数注入，便于表驱动测试）。
//
// 契约（fallback = CaptchaFallback 开关，默认关闭）：
//   - 关闭：两引擎严格互不回退。配置 ddddocr 但本机无引擎 → 返回 nil（登录报
//     "未配置验证码识别引擎"），绝不静默换云端；配置 vision 但无密钥 → 同样 nil。
//   - 开启：双向兜底。ddddocr 不可用 → Vision；Vision 无密钥 → 本机 ddddocr。
//
// 探测成本契约：只有"确实可能用到本地 ddddocr"时才调用 hasLocal 探测
// （本地探测要起 Python 子进程，最坏 15s）——配置 vision 且开关关闭、或配置 vision
// 且已有密钥时都不得探测，否则管理员每次保存配置都要白等一次 Python 冷启动。
func resolveCaptchaRecognizer(
	cfg runtime.Config,
	hasNative func() bool,
	hasLocal func() bool,
) captchaResolution {
	vision := func() zhidao.CaptchaRecognizer {
		if cfg.VisionAPIKey == "" {
			return nil
		}
		return zhidao.NewVisionRecognizer(zhidao.VisionConfig{
			BaseURL: cfg.VisionBaseURL, APIKey: cfg.VisionAPIKey, Model: cfg.VisionModel,
		})
	}
	// ddddocr 可用性惰性求值：原生内置 → 本机 Python；两者皆无返回 nil。
	// 类型断言用 reflect 类型名而非具体类型：NativeDdddOcrRecognizer 是 build-tag
	// 条件类型（非 Windows / 无 CGO 下不存在），直接断言会让 CGO=0 构建编不过。
	local := func() (zhidao.CaptchaRecognizer, string) {
		if hasNative() {
			if r := zhidao.NewNativeDdddOcrRecognizer(); r != nil {
				return r, "单二进制内置原生 ddddocr（免 Python / 免 API 密钥，5~10ms 极速推理）"
			}
		}
		if hasLocal() {
			return zhidao.NewLocalDdddOcrRecognizer(""), "本地 Python ddddocr（无 API 密钥）"
		}
		return nil, ""
	}

	if cfg.CaptchaEngine == "ddddocr" {
		if l, note := local(); l != nil {
			return captchaResolution{l, "ddddocr", note}
		}
		if !cfg.CaptchaFallback {
			return captchaResolution{nil, "none",
				"配置为 ddddocr 但本机无内置模型且无 Python/ddddocr；引擎兜底已关闭，识别不可用（请安装 ddddocr 或改选 Vision）"}
		}
		if v := vision(); v != nil {
			return captchaResolution{v, "vision", "配置为 ddddocr 但本机不可用，按兜底开关回退 Vision"}
		}
		return captchaResolution{nil, "none",
			"配置为 ddddocr 但本机不可用，且 Vision 未配置密钥；识别不可用"}
	}

	// 配置为 vision（值域已由 handleAdminConfig 校验，其余值一律按 vision 处理）
	if v := vision(); v != nil {
		return captchaResolution{v, "vision", "硅基流动 Vision 云识别"}
	}
	if !cfg.CaptchaFallback {
		return captchaResolution{nil, "none",
			"配置为 Vision 但未填 API 密钥；引擎兜底已关闭，识别不可用（请填写密钥或改选 ddddocr）"}
	}
	if l, note := local(); l != nil {
		return captchaResolution{l, "ddddocr", "配置为 Vision 但未配置密钥，按兜底开关回退" + note}
	}
	return captchaResolution{nil, "none", "配置为 Vision 但未配置密钥，且本机无 ddddocr；识别不可用"}
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
// accts 为多账号客户端注册表；sessions 为会话库；adminToken 为管理口令；activationEnabled 为激活码机制开关。
// encrypt/decrypt 用于敏感配置（vision_key）加密入库/解密读回；Decrypt 字段仅注入备用。
func Register(mux *http.ServeMux, st *store.Store, sched *scheduler.Scheduler,
	accts *accounts.Manager, sessions *session.Store, adminToken, adminName string,
	activationEnabled bool, encrypt, decrypt func(string) (string, error), rt *runtime.Store) http.Handler {

	d := &Deps{Store: st, Sched: sched, Accounts: accts, Sessions: sessions,
		Runtime: rt, AdminToken: adminToken, ActivationEnabled: activationEnabled,
		Encrypt: encrypt, Decrypt: decrypt, AdminName: adminName}
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
