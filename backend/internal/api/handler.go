package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

// Deps API 层依赖。
type Deps struct {
	Store   *store.Store
	Client  *zhidao.Client
	Sched   *scheduler.Scheduler
	OpenTime string // 选课开放时间（与调度器一致）
}

// writeJSON 统一 JSON 响应：{"code":0,"data":...,"msg":""}
func writeJSON(w http.ResponseWriter, code int, data any, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"code": code, "data": data, "msg": msg})
}

// LoginRequest 登录请求体。
type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// handleLogin 账密登录：调用至道登录链路，成功后保存账密与 token。
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
	token, err := d.Client.Login(req.Account, req.Password)
	if err != nil {
		writeJSON(w, 1, nil, "登录失败: "+err.Error())
		return
	}
	if err := d.Store.SaveAccount(req.Account, req.Password, token); err != nil {
		log.Printf("[api] 保存账密失败: %v", err)
	}
	d.Client.SetCredentials(req.Account, req.Password, token)
	d.Store.AppendLog(0, "login", "登录成功", true)
	writeJSON(w, 0, map[string]string{"token": token}, "登录成功")
}

// handleElectives 课程列表（三个发布）。
func (d *Deps) handleElectives(w http.ResponseWriter, r *http.Request) {
	data, err := d.Client.FindElectives()
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
	detail, err := d.Client.ClassDetail(id)
	if err != nil {
		writeJSON(w, 1, nil, "查询详情失败: "+err.Error())
		return
	}
	writeJSON(w, 0, detail, "")
}

// TargetsRequest 设置目标请求体。
type TargetsRequest struct {
	Targets []scheduler.Target `json:"targets"`
}

// handleSetTargets 设置目标课程并持久化。
func (d *Deps) handleSetTargets(w http.ResponseWriter, r *http.Request) {
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
	if err := d.Store.SetTargets(req.Targets); err != nil {
		writeJSON(w, 1, nil, "保存目标失败: "+err.Error())
		return
	}
	d.Sched.SetTargets(req.Targets)
	d.Store.AppendLog(0, "set_targets", fmt.Sprintf("%d 门目标课程", len(req.Targets)), true)
	writeJSON(w, 0, req.Targets, "目标已保存")
}

// handleState 调度器状态。
func (d *Deps) handleState(w http.ResponseWriter, r *http.Request) {
	st := d.Sched.State()
	writeJSON(w, 0, st, "")
}

// handleLogs 报名日志。
func (d *Deps) handleLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := d.Store.LoadLogs(100)
	if err != nil {
		writeJSON(w, 1, nil, "读取日志失败: "+err.Error())
		return
	}
	writeJSON(w, 0, logs, "")
}

// handleHealth 健康检查。
func (d *Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 0, "ok", "")
}

// ---- 安全中间件 ----

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
		next.ServeHTTP(w, r)
	})
}
