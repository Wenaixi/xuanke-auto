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
		if zhidao.LocalDdddOcrAvailable("") {
			accts.SetRecognizer(zhidao.NewLocalDdddOcrRecognizer(""))
			log.Printf("[api] 验证码识别引擎：本地 ddddocr（无 API 密钥）")
		} else {
			log.Printf("[api] 配置为 ddddocr 但本机无 Python/ddddocr，回退 Vision")
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
func jsonContentType(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

// Register 注册所有 API 路由到 mux，并返回包装了安全中间件的根 handler。
// accts 为多账号客户端注册表；sessions 为会话库；adminToken 为管理口令；activationEnabled 为激活码机制开关。
func Register(mux *http.ServeMux, st *store.Store, sched *scheduler.Scheduler,
	accts *accounts.Manager, sessions *session.Store, openTime, adminToken, adminName string,
	activationEnabled bool, encrypt, decrypt func(string) (string, error), rt *runtime.Store) http.Handler {

	d := &Deps{Store: st, Sched: sched, Accounts: accts, Sessions: sessions,
		OpenTime: openTime, Runtime: rt, AdminToken: adminToken, ActivationEnabled: activationEnabled,
		Encrypt: encrypt, Decrypt: decrypt, AdminName: adminName}
	limiter := newLoginLimiter()

	// 启动即按运行时配置初始化验证码识别引擎与并发信号量（幂等）
	initCaptchaAtStartup(rt, accts)

	mux.HandleFunc("GET /api/health", d.handleHealth)
	// 登录接口限流（激活码已取代部署口令 gate；未启用激活码机制时登录即发会话）
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		// CSRF 缓解：仅接受 JSON 提交（跨站表单 POST 无法伪造该头）
		if !jsonContentType(r) {
			writeJSON(w, 403, nil, "仅接受 JSON 提交")
			return
		}
		if !limiter.allow(clientIP(r)) {
			writeJSON(w, 429, nil, "登录尝试过于频繁，请稍后再试")
			return
		}
		d.handleLogin(w, r)
	})
	// 激活接口（登录后未激活才需要，未认证；机制关闭时 handler 直接拒绝）
	mux.HandleFunc("POST /api/activate", func(w http.ResponseWriter, r *http.Request) {
		if !jsonContentType(r) {
			writeJSON(w, 403, nil, "仅接受 JSON 提交")
			return
		}
		if !limiter.allow(clientIP(r)) {
			writeJSON(w, 429, nil, "激活尝试过于频繁，请稍后再试")
			return
		}
		d.handleActivate(w, r)
	})
	// 激活码管理接口（会话级管理员鉴权）
	mux.HandleFunc("GET /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminCodes)(w, r)
	})
	mux.HandleFunc("POST /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminCodes)(w, r)
	})
	mux.HandleFunc("DELETE /api/admin/codes", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminCodes)(w, r)
	})
	// 管理员后台：配置热重载 / 运行状态 / 账号管理 / 日志总览（会话级管理员鉴权）
	mux.HandleFunc("GET /api/admin/config", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminConfig)(w, r)
	})
	mux.HandleFunc("PUT /api/admin/config", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminConfig)(w, r)
	})
	mux.HandleFunc("GET /api/admin/stats", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminStats)(w, r)
	})
	mux.HandleFunc("GET /api/admin/accounts", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminAccounts)(w, r)
	})
	mux.HandleFunc("DELETE /api/admin/accounts", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminDeleteAccount)(w, r)
	})
	mux.HandleFunc("GET /api/admin/logs", func(w http.ResponseWriter, r *http.Request) {
		requireAdminSession(d, d.handleAdminLogs)(w, r)
	})
	// 其余接口全部要求会话认证
	mux.HandleFunc("GET /api/electives", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleElectives)(w, r)
	})
	mux.HandleFunc("PUT /api/targets", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleSetTargets)(w, r)
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

	return recoverMiddleware(securityHeaders(mux))
}