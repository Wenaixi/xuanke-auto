package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
)

// Deps API 层依赖。
type Deps struct {
	Store    *store.Store
	Sched    *scheduler.Scheduler
	Accounts *accounts.Manager
	Sessions *session.Store
	OpenTime string
	// AdminToken 部署访问口令（main 从环境变量注入，启动必填）。
	AdminToken string
	// Encrypt 密码加密（secure.Encrypt 绑定主密钥闭包）。
	Encrypt func(string) (string, error)
}

// writeJSON 统一 JSON 响应：{"code":0,"data":...,"msg":""}
func writeJSON(w http.ResponseWriter, code int, data any, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"code": code, "data": data, "msg": msg})
}

// LoginRequest 登录请求体（部署口令 + 教务账密）。
type LoginRequest struct {
	Account    string `json:"account"`
	Password   string `json:"password"`
	AdminToken string `json:"admin_token"`
}

// handleLogin 登录：校验部署口令 -> 教务登录 -> 签发会话并返回。
func (d *Deps) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.Account == "" || req.Password == "" {
		writeJSON(w, 1, nil, "账号与密码不能为空")
		return
	}
	// 部署口令校验（启动必须配置）
	if d.AdminToken == "" || subtle.ConstantTimeCompare([]byte(req.AdminToken), []byte(d.AdminToken)) != 1 {
		writeJSON(w, 403, nil, "部署访问口令错误")
		return
	}
	if _, err := d.Accounts.LoginByPassword(req.Account, req.Password, d.Encrypt); err != nil {
		writeJSON(w, 1, nil, "登录失败: "+err.Error())
		return
	}
	if err := d.Store.SaveAccountName(req.Account); err != nil {
		log.Printf("[api] 保存账号名失败: %v", err)
	}
	sess := d.Sessions.Create(req.Account)
	d.Store.AppendLog(req.Account, 0, "login", "账号 "+req.Account+" 登录成功", true)
	writeJSON(w, 0, map[string]string{"token": sess, "account": req.Account}, "登录成功")
}

// handleElectives 课程列表：直读调度器内存快照（超高性能），快照过期才触发探测。
func (d *Deps) handleElectives(w http.ResponseWriter, r *http.Request) {
	if data, ok := d.Sched.ElectivesSnapshot(); ok {
		writeJSON(w, 0, data, "")
		return
	}
	data, err := d.Sched.ProbeNow()
	if err != nil {
		writeJSON(w, 1, nil, "查询课程失败: "+err.Error())
		return
	}
	writeJSON(w, 0, data, "")
}

// handleElectivesDetail 课程详情。
func (d *Deps) handleElectivesDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeJSON(w, 1, nil, "id 参数无效")
		return
	}
	acct := sessionAccount(r)
	client, ok := d.Accounts.ClientFor(acct)
	if !ok {
		writeJSON(w, 1, nil, "账号会话未建立，请重新登录")
		return
	}
	detail, err := client.ClassDetail(id)
	if err != nil {
		writeJSON(w, 1, nil, "查询详情失败: "+err.Error())
		return
	}
	writeJSON(w, 0, detail, "")
}

// TargetsRequest 设置目标请求体（账号由会话决定，不接收客户端传账号）。
type TargetsRequest struct {
	Targets []scheduler.Target `json:"targets"`
}

// handleSetTargets 设置目标课程并持久化（账号来自会话绑定）。
func (d *Deps) handleSetTargets(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	var req TargetsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	// 允许清空所有目标课程（len == 0），满足用户随时重置预选目标的需求
	if req.Targets == nil {
		req.Targets = []scheduler.Target{}
	}
	for _, t := range req.Targets {
		if t.ClassID <= 0 {
			writeJSON(w, 1, nil, "class_id 无效")
			return
		}
	}
	if err := d.Store.SetTargetsForAccount(acct, req.Targets); err != nil {
		writeJSON(w, 1, nil, "保存目标失败: "+err.Error())
		return
	}
	d.Sched.SetTargetsForAccount(acct, req.Targets)
	d.Store.AppendLog(acct, 0, "set_targets", fmt.Sprintf("账号 %s：%d 门目标课程", acct, len(req.Targets)), true)
	writeJSON(w, 0, req.Targets, "目标已保存")
}

// handleState 调度器状态（按会话账号过滤目标）。
func (d *Deps) handleState(w http.ResponseWriter, r *http.Request) {
	st := d.Sched.StateForAccount(sessionAccount(r))
	writeJSON(w, 0, st, "")
}

// handleAccounts 当前会话账号视角的账号列表（注册表顺序）。
func (d *Deps) handleAccounts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 0, d.Accounts.Registered(), "")
}

// handleLogs 报名日志。
func (d *Deps) handleLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := d.Store.LoadLogs(sessionAccount(r), 100)
	if err != nil {
		writeJSON(w, 1, nil, "读取日志失败: "+err.Error())
		return
	}
	writeJSON(w, 0, logs, "")
}

// handleHealth 健康检查（免认证，仅探活）。
func (d *Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 0, "ok", "")
}

// ---- 会话认证中间件 ----

type ctxKey int

const sessionCtxKey ctxKey = 1

// sessionAccount 从请求上下文取会话绑定的账号。
func sessionAccount(r *http.Request) string {
	if v, ok := r.Context().Value(sessionCtxKey).(string); ok {
		return v
	}
	return ""
}

// requireAuth 会话校验中间件：无/无效令牌返回 401。
func requireAuth(d *Deps, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := ""
		if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
			tok = h[7:]
		} else if xt := r.Header.Get("X-Auth-Token"); xt != "" {
			tok = xt
		}
		acct, ok := d.Sessions.Account(tok)
		if !ok {
			writeJSON(w, 401, nil, "会话无效或已过期，请重新登录")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionCtxKey, acct)))
	}
}

// ---- 登录限流 ----

// loginLimiter 登录限流：每 IP 每分钟最多 5 次登录尝试。
type loginLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

const (
	loginRate  = 5.0 / 60.0 // 每分钟 5 次
	loginBurst = 5
)

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{buckets: make(map[string]*tokenBucket)}
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[ip]
	now := time.Now()
	if !ok {
		b = &tokenBucket{tokens: loginBurst, lastFill: now}
		l.buckets[ip] = b
	}
	// 按速率补充令牌
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens = min(loginBurst, b.tokens+elapsed*loginRate)
	b.lastFill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// recoverMiddleware panic 恢复：任何 handler panic 返回 500 不崩溃。
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[api] panic recovered: %v", rec)
				writeJSON(w, 500, nil, "内部错误: "+fmt.Sprint(rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders 安全响应头。
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}