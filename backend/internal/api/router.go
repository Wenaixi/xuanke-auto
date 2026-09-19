package api

import (
	"log"
	"net/http"
	"strings"

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
func applyCaptchaRecognizerFor(rt *runtime.Store, accts *accounts.Manager) {
	cfg := rt.Get()
	switch cfg.CaptchaEngine {
	case "ddddocr":
		if zhidao.NativeDdddOcrAvailable() {
			accts.SetRecognizer(zhidao.NewNativeDdddOcrRecognizer())
			log.Printf("[api] 验证码识别引擎：单二进制内置原生 ddddocr（免 Python / 免 API 密钥，5~10ms 极速推理）")
		} else if zhidao.LocalDdddOcrAvailable("") {
			accts.SetRecognizer(zhidao.NewLocalDdddOcrRecognizer(""))
			log.Printf("[api] 验证码识别引擎：本地 Python ddddocr（无 API 密钥）")
		} else {
			log.Printf("[api] 配置为 ddddocr 但无内置模型且本机无 Python/ddddocr，回退 Vision")
			accts.SetRecognizer(zhidao.NewVisionRecognizer(zhidao.VisionConfig{
				BaseURL: cfg.VisionBaseURL, APIKey: cfg.VisionAPIKey, Model: cfg.VisionModel,
			}))
		}
	default: // vision
		accts.SetRecognizer(zhidao.NewVisionRecognizer(zhidao.VisionConfig{
			BaseURL: cfg.VisionBaseURL, APIKey: cfg.VisionAPIKey, Model: cfg.VisionModel,
		}))
	}
	zhidao.SetCaptchaConcurrency(cfg.CaptchaConcurrency)
}

// jsonContentType 检查请求体是否为 JSON（反跨站表单 POST 的 CSRF 缓解）。
// 前端统一用 fetch+JSON，必带 application/json；跨站表单提交是
// application/x-www-form-urlencoded，无法伪造该头 → 直接 403 拒绝副作用请求。
// m8 修复：admin 的 PUT/DELETE（config 热改、codes 生成/删除、账号删除）同样复用此检查，
// 防止管理员接口被跨站表单 POST 挟持。
func jsonContentType(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

// requireJSONBody 复用 jsonContentType 拒绝非 JSON 提交的副作用请求（m8）。
func requireJSONBody(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !jsonContentType(r) {
			writeJSON(w, 403, nil, "仅接受 JSON 提交")
			return
		}
		next(w, r)
	}
}

// Register 注册所有 API 路由到 mux，并返回包装了安全中间件的根 handler。
// accts 为多账号客户端注册表；sessions 为会话库；adminToken 为管理口令；activationEnabled 为激活码机制开关。
// encrypt/decrypt 用于敏感配置（vision_key）加密入库/解密读回；Decrypt 字段仅注入备用（B10-08 起未消费）。
func Register(mux *http.ServeMux, st *store.Store, sched *scheduler.Scheduler,
	accts *accounts.Manager, sessions *session.Store, adminToken, adminName string,
	activationEnabled bool, encrypt, decrypt func(string) (string, error), rt *runtime.Store) http.Handler {

	d := &Deps{Store: st, Sched: sched, Accounts: accts, Sessions: sessions,
		Runtime: rt, AdminToken: adminToken, ActivationEnabled: activationEnabled,
		Encrypt: encrypt, Decrypt: decrypt, AdminName: adminName}
	// M-6 修复：登录与激活各自独立限流桶——激活码输入错误不消耗登录额度、
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
			// B39-02：限流是真实 429，必须写 HTTP 状态码（此前恒 200）。
			writeJSONStatus(w, http.StatusTooManyRequests, 429, nil, "登录尝试过于频繁，请稍后再试")
			return
		}
		d.handleLogin(w, r)
	})
	// 激活接口（登录后未激活才需要，未认证；机制关闭时 handler 直接拒绝）
	// M-6：激活用独立限流桶——攻击者刷激活码不会消耗他人登录额度，反之亦然
	mux.HandleFunc("POST /api/activate", func(w http.ResponseWriter, r *http.Request) {
		if !jsonContentType(r) {
			writeJSONStatus(w, http.StatusForbidden, 403, nil, "仅接受 JSON 提交")
			return
		}
		if !activateLim.allow(clientIP(r)) {
			// B39-02：激活限流同为真实 429，写 HTTP 状态码。
			writeJSONStatus(w, http.StatusTooManyRequests, 429, nil, "激活尝试过于频繁，请稍后再试")
			return
		}
		d.handleActivate(w, r)
	})
	// 激活码管理接口（会话级管理员鉴权）；m8：生成/删除是副作用请求，强制 JSON
	mux.HandleFunc("GET /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminCodes)(w, r)
	})
	// B7-M8/M9：生成/列表/删除三态同级路由。此前 DELETE 也强制 requireJSONBody，
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
	// 其余接口全部要求会话认证；m8：报名/退选/目标设置等副作用请求同样强制 JSON
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
	// M-7：登出接口——会话级鉴权，立即吊销服务端令牌（防止令牌外流残留）
	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleLogout)(w, r)
	})
	// B7-C4：未知 /api/ 路径显式 404——此前未注册的 /api/xxx 落入
	// main.go 的 mux.Handle("/", SpaHandler) 兜底，返回 index.html（HTTP 200 text/html）：
	// 前端 fetch 拿到 200 HTML 解析 JSON 报错掩盖真实 404，且被安全扫描误判"任意路径可 200"。
	// 显式注册 "/api/" 前缀后，所有已注册的精确 method+pattern 优先命中，未匹配的 /api/xxx
	// 一律 HTTP 404 + JSON body，绝不再回退 SPA。
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // 业务代码恒 200 约定不适用于"端点不存在"——真实 404 语义才对
		writeJSON(w, 404, nil, "接口不存在")
	})

	return recoverMiddleware(securityHeaders(mux))
}
