package api

import (
	"net/http"

	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

// Register 注册所有 API 路由到 mux，并返回包装了安全中间件的根 handler。
func Register(mux *http.ServeMux, st *store.Store, client *zhidao.Client, sched *scheduler.Scheduler, openTime string) http.Handler {
	d := &Deps{Store: st, Client: client, Sched: sched, OpenTime: openTime}
	limiter := newLoginLimiter()

	mux.HandleFunc("GET /api/health", d.handleHealth)
	// 登录接口限流
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			writeJSON(w, 429, nil, "登录尝试过于频繁，请稍后再试")
			return
		}
		d.handleLogin(w, r)
	})
	mux.HandleFunc("GET /api/electives", d.handleElectives)
	mux.HandleFunc("GET /api/electives/detail", d.handleElectivesDetail)
	mux.HandleFunc("PUT /api/targets", d.handleSetTargets)
	mux.HandleFunc("GET /api/accounts", d.handleAccounts)
	mux.HandleFunc("GET /api/state", d.handleState)
	mux.HandleFunc("GET /api/logs", d.handleLogs)

	return recoverMiddleware(securityHeaders(mux))
}
