package api

import (
	"net/http"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
)

// Register 注册所有 API 路由到 mux，并返回包装了安全中间件的根 handler。
// accts 为多账号客户端注册表；sessions 为会话库；adminToken 为部署访问口令；encrypt 为密码加密函数。
func Register(mux *http.ServeMux, st *store.Store, sched *scheduler.Scheduler,
	accts *accounts.Manager, sessions *session.Store, openTime, adminToken string,
	encrypt func(string) (string, error)) http.Handler {

	d := &Deps{Store: st, Sched: sched, Accounts: accts, Sessions: sessions,
		OpenTime: openTime, AdminToken: adminToken, Encrypt: encrypt}
	limiter := newLoginLimiter()

	mux.HandleFunc("GET /api/health", d.handleHealth)
	// 登录接口限流 + 部署口令
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			writeJSON(w, 429, nil, "登录尝试过于频繁，请稍后再试")
			return
		}
		d.handleLogin(w, r)
	})
	// 其余接口全部要求会话认证
	mux.HandleFunc("GET /api/electives", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleElectives)(w, r)
	})
	mux.HandleFunc("GET /api/electives/detail", func(w http.ResponseWriter, r *http.Request) {
		requireAuth(d, d.handleElectivesDetail)(w, r)
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