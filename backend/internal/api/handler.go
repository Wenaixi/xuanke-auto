package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

// Deps API 层依赖。
type Deps struct {
	Store    *store.Store
	Sched    *scheduler.Scheduler
	Accounts *accounts.Manager
	Sessions *session.Store
	OpenTime string
	// Runtime 进程内配置中心（管理员热重载生效）。
	Runtime *runtime.Store
	// AdminToken 管理口令（main 从环境变量/.env 注入，启动必填；admin 账号的密码）。
	// Encrypt 数据加密函数（main 注入：secure.Encrypt，凭据与 vision_key 落库前加密）。
	Encrypt func(string) (string, error)
	// Decrypt 数据解密函数（main 注入：secure.Decrypt，vision_key 读回时解密）。
	Decrypt func(string) (string, error)
	AdminToken string
	// ActivationEnabled 激活码机制是否启用（XUANKE_ACTIVATION=off 时完全禁用）。
	ActivationEnabled bool
}

// secureEncrypt 用注入的 Encrypt 加密敏感值，并加 enc: 前缀标记（main 读回时据此解密）。
// 严禁未加密存储：未注入 Encrypt 时报错拒绝，杜绝明文入库。
func (d *Deps) secureEncrypt(v string) (string, error) {
	if d.Encrypt == nil {
		return "", errors.New("数据加密服务未初始化，拒绝未加密存储")
	}
	enc, err := d.Encrypt(v)
	if err != nil {
		return "", err
	}
	return "enc:" + enc, nil
}

// writeJSON 统一 JSON 响应：{"code":0,"data":...,"msg":""}
func writeJSON(w http.ResponseWriter, code int, data any, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"code": code, "data": data, "msg": msg})
}

// LoginRequest 登录请求体（教务账密，无部署口令——激活码已取代登录口令 gate）。
type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// handleLogin 登录：
//   - admin 账号 + 管理口令 → 签发管理员会话（绕过教务登录，避免平台登录限流）
//   - 其他账号 → 教务登录 -> 检查激活状态 -> 已激活签发会话，未激活提示输激活码。
// 激活码机制关闭（ActivationEnabled=false）时跳过激活检查，登录即签发会话。
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
	// 管理员入口：账号 admin + 管理口令（不触碰教务登录，口令比对恒定时间防爆破）
	if req.Account == "admin" {
		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(d.AdminToken)) != 1 {
			writeJSON(w, 1, nil, "管理口令错误")
			return
		}
		sess := d.Sessions.CreateAdmin()
		d.Store.AppendLog("admin", 0, "login", "管理员登录成功", true)
		writeJSON(w, 0, map[string]string{"token": sess, "account": "admin"}, "管理员登录成功")
		return
	}
	if _, err := d.Accounts.LoginByPassword(req.Account, req.Password, d.Encrypt); err != nil {
		writeJSON(w, 1, nil, "登录失败: "+err.Error())
		return
	}
	if d.activationEnabled() {
		activated, err := d.Store.IsActivated(req.Account)
		if err != nil {
			writeJSON(w, 1, nil, "查询激活状态失败: "+err.Error())
			return
		}
		if !activated {
			// 未激活：前端据此弹出激活码输入框
			writeJSON(w, 1001, map[string]string{"account": req.Account}, "该账号尚未激活，请输入激活码")
			return
		}
	}
	d.issueSession(w, req.Account)
}

// ActivateRequest 激活请求体。
type ActivateRequest struct {
	Account string `json:"account"`
	Code    string `json:"code"`
}

// activationEnabled 读取激活码机制开关：优先运行时配置（管理员热改立即生效），
// 无配置中心时回落静态 Deps 字段（测试直构场景）。
func (d *Deps) activationEnabled() bool {
	if d.Runtime != nil {
		return d.Runtime.Get().ActivationEnabled
	}
	return d.ActivationEnabled
}

// handleActivate 激活账号：消耗激活码并签发会话（机制关闭时拒绝）。
func (d *Deps) handleActivate(w http.ResponseWriter, r *http.Request) {
	if !d.activationEnabled() {
		writeJSON(w, 1, nil, "激活码机制已关闭")
		return
	}
	var req ActivateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.Account == "" || req.Code == "" {
		writeJSON(w, 1, nil, "账号与激活码不能为空")
		return
	}
	ok, err := d.Store.ConsumeActivationCode(strings.TrimSpace(req.Code), strings.TrimSpace(req.Account))
	if err != nil {
		writeJSON(w, 1, nil, "激活失败: "+err.Error())
		return
	}
	if !ok {
		writeJSON(w, 1, nil, "激活码无效或已用尽")
		return
	}
	d.issueSession(w, strings.TrimSpace(req.Account))
}

// issueSession 记录账号名 + 签发会话 + 记日志。
func (d *Deps) issueSession(w http.ResponseWriter, acct string) {
	if err := d.Store.SaveAccountName(acct); err != nil {
		log.Printf("[api] 保存账号名失败: %v", err)
	}
	sess := d.Sessions.Create(acct)
	d.Store.AppendLog(acct, 0, "login", "账号 "+acct+" 登录成功", true)
	writeJSON(w, 0, map[string]string{"token": sess, "account": acct}, "登录成功")
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

// handleLogs 报名日志（仅返回当前会话账号自己的日志）。
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

// ---- 激活码管理接口 ----

// requireAdminSession 管理员会话校验：仅接受 admin 账号登录签发的会话令牌。
// 管理口令只在登录时比对一次，后续一律走会话（X-Admin-Token 已废弃）。
func requireAdminSession(d *Deps, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := ""
		if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
			tok = h[7:]
		} else if xt := r.Header.Get("X-Auth-Token"); xt != "" {
			tok = xt
		}
		if !d.Sessions.IsAdmin(tok) {
			writeJSON(w, 403, nil, "需要管理员权限")
			return
		}
		next(w, r)
	}
}

// handleAdminCodes 激活码管理：POST 生成 / GET 列表 / DELETE 删除。
// 激活码机制关闭时整个接口禁用（关了就根本没有）。
func (d *Deps) handleAdminCodes(w http.ResponseWriter, r *http.Request) {
	if !d.activationEnabled() {
		writeJSON(w, 1, nil, "激活码机制已关闭")
		return
	}
	switch r.Method {
	case http.MethodGet:
		codes, err := d.Store.ListActivationCodes()
		if err != nil {
			writeJSON(w, 1, nil, "读取激活码失败: "+err.Error())
			return
		}
		writeJSON(w, 0, codes, "")
	case http.MethodPost:
		var req struct {
			Count int `json:"count"`
			Uses  int `json:"uses"` // 每个激活码可用次数
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
			return
		}
		if req.Count < 1 || req.Count > 100 {
			writeJSON(w, 1, nil, "生成数量需在 1-100 之间")
			return
		}
		if req.Uses < 1 {
			writeJSON(w, 1, nil, "每个激活码使用次数至少为 1")
			return
		}
		codes := make([]string, 0, req.Count)
		for i := 0; i < req.Count; i++ {
			code := newActivationCode()
			if err := d.Store.CreateActivationCode(code, req.Uses); err != nil {
				writeJSON(w, 1, nil, "生成激活码失败: "+err.Error())
				return
			}
			codes = append(codes, code)
		}
		writeJSON(w, 0, codes, "生成成功")
	case http.MethodDelete:
		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
			return
		}
		if err := d.Store.DeleteActivationCode(strings.TrimSpace(req.Code)); err != nil {
			writeJSON(w, 1, nil, "删除失败: "+err.Error())
			return
		}
		writeJSON(w, 0, nil, "已删除")
	default:
		writeJSON(w, 405, nil, "方法不允许")
	}
}

// newActivationCode 生成 XK-XXXX-XXXX-XXXX 格式激活码（12 位十六进制）。
func newActivationCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	s := strings.ToUpper(hex.EncodeToString(b))
	return fmt.Sprintf("XK-%s-%s-%s", s[0:4], s[4:8], s[8:12])
}

// ---- 管理员后台接口（会话级鉴权，见 requireAdminSession） ----

// maskKey 敏感值脱敏：仅回显后 4 位，其余掩码（空值返回空串）。
func maskKey(v string) string {
	if len(v) <= 4 {
		return ""
	}
	return "****" + v[len(v)-4:]
}

// AdminConfigView 配置响应（Vision key 脱敏回显）。
type AdminConfigView struct {
	ActivationEnabled bool   `json:"activation_enabled"`
	VisionBaseURL     string `json:"vision_base_url"`
	VisionAPIKey      string `json:"vision_api_key_masked"`
	VisionModel       string `json:"vision_model"`
	OpenTime          string `json:"open_time"`
}

// handleAdminConfig GET 读取 / PUT 热更新系统配置。
// PUT 立即写入运行时配置中心（调度器/账号管理器/Vision 同步生效）并落库 settings。
func (d *Deps) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := d.Runtime.Get()
		writeJSON(w, 0, AdminConfigView{
			ActivationEnabled: cfg.ActivationEnabled,
			VisionBaseURL:     cfg.VisionBaseURL,
			VisionAPIKey:      maskKey(cfg.VisionAPIKey),
			VisionModel:       cfg.VisionModel,
			OpenTime:          cfg.OpenTime,
		}, "")
	case http.MethodPut:
		var req struct {
			ActivationEnabled *bool   `json:"activation_enabled"`
			VisionBaseURL     *string `json:"vision_base_url"`
			VisionAPIKey      *string `json:"vision_api_key"`
			VisionModel       *string `json:"vision_model"`
			OpenTime          *string `json:"open_time"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
			return
		}
		var changed []string
		d.Runtime.Update(func(c *runtime.Config) {
			if req.ActivationEnabled != nil {
				c.ActivationEnabled = *req.ActivationEnabled
				changed = append(changed, "activation_enabled")
			}
			if req.VisionBaseURL != nil {
				c.VisionBaseURL = *req.VisionBaseURL
				changed = append(changed, "vision_base_url")
			}
			if req.VisionAPIKey != nil {
				if strings.TrimSpace(*req.VisionAPIKey) != "" {
					c.VisionAPIKey = *req.VisionAPIKey
					changed = append(changed, "vision_api_key")
				}
			}
			if req.VisionModel != nil {
				c.VisionModel = *req.VisionModel
				changed = append(changed, "vision_model")
			}
			if req.OpenTime != nil {
				if _, err := scheduler.FormatOpenTime(*req.OpenTime); err == nil {
					c.OpenTime = *req.OpenTime
					changed = append(changed, "open_time")
				}
			}
		})
		cfg := d.Runtime.Get()
		// 配置变更落库（settings 全量替换，重启后恢复）。vision_key 加密落库：
		// 与凭据同强度（AES-256-GCM），settings 表内永不出现明文密钥。
		visionKey, vErr := d.secureEncrypt(cfg.VisionAPIKey)
		if vErr != nil {
			writeJSON(w, 1, nil, "配置加密失败: "+vErr.Error())
			return
		}
		if sErr := d.Store.SaveSettings(map[string]string{
			"activation_enabled": strconv.FormatBool(cfg.ActivationEnabled),
			"vision_base_url":    cfg.VisionBaseURL,
			"vision_key":         visionKey,
			"vision_model":       cfg.VisionModel,
			"open_time":          cfg.OpenTime,
		}); sErr != nil {
			log.Printf("[api] 配置落库失败: %v", sErr)
			// 落库失败不阻塞生效（内存已改），但必须如实记录，避免重启后配置回退无感知
		}
		// 热重载下游组件：验证码识别配置推给全部账号客户端；打开时间由调度器运行时读取
		d.Accounts.SetVision(zhidao.VisionConfig{
			BaseURL: cfg.VisionBaseURL, APIKey: cfg.VisionAPIKey, Model: cfg.VisionModel,
		})
		if len(changed) == 0 {
			writeJSON(w, 1, nil, "没有可应用的有效配置项")
			return
		}
		d.Store.AppendLog("admin", 0, "config", "更新配置: "+strings.Join(changed, ", "), true)
		writeJSON(w, 0, AdminConfigView{
			ActivationEnabled: cfg.ActivationEnabled,
			VisionBaseURL:     cfg.VisionBaseURL,
			VisionAPIKey:      maskKey(cfg.VisionAPIKey),
			VisionModel:       cfg.VisionModel,
			OpenTime:          cfg.OpenTime,
		}, "配置已更新并生效")
	default:
		writeJSON(w, 405, nil, "方法不允许")
	}
}

// handleAdminStats 系统运行状态总览。
func (d *Deps) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	cfg := d.Runtime.Get()
	accounts, _ := d.Store.ListAccounts()
	success, _ := d.Store.LoadSuccess()
	allLogs, _ := d.Store.LoadAllLogs(1000)
	open := time.Time{}
	if !cfg.OpenTimeParsed.IsZero() {
		open = cfg.OpenTimeParsed
	}
	targetsCount := 0
	for _, a := range accounts {
		ts, err := d.Store.LoadTargetsForAccount(a)
		if err == nil {
			targetsCount += len(ts)
		}
	}
	// window_opened 与调度器实际探测状态保持一致（学生端 /state 同源），
	// 不用本地时钟直判——平台开放时间与本地配置若有偏差，管理员不会误判。
	windowOpened := d.Sched.WindowOpened()
	if windowOpened == nil {
		windowOpened = boolPtr(time.Now().After(open)) // 调度器未启动时的回退
	}
	writeJSON(w, 0, map[string]any{
		"open_time":        open.Format("2006-01-02 15:04:05"),
		"activation_on":    cfg.ActivationEnabled,
		"window_opened":    *windowOpened,
		"account_count":    len(accounts),
		"targets_count":    targetsCount,
		"success_count":    len(success),
		"log_count":        len(allLogs),
		"vision_model":     cfg.VisionModel,
		"vision_base_url":  cfg.VisionBaseURL,
	}, "")
}

func boolPtr(b bool) *bool { return &b }

// handleAdminAccounts 账号管理列表（含目标与已成功课程）。
func (d *Deps) handleAdminAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := d.Store.ListAdminAccounts()
	if err != nil {
		writeJSON(w, 1, nil, "读取账号失败: "+err.Error())
		return
	}
	writeJSON(w, 0, list, "")
}

// handleAdminDeleteAccount 删除账号（清凭据/目标/成功/激活状态；日志保留审计）。
func (d *Deps) handleAdminDeleteAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account string `json:"account"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Account) == "" || req.Account == "admin" {
		writeJSON(w, 1, nil, "账号无效或不可删除")
		return
	}
	if err := d.Store.DeleteAccount(req.Account); err != nil {
		writeJSON(w, 1, nil, "删除失败: "+err.Error())
		return
	}
	// 调度器与账号注册表同步隔离：清空内存目标（下个 tick 不再提交）+ 移除客户端。
	d.Sched.SetTargetsForAccount(req.Account, nil)
	d.Accounts.Remove(req.Account)
	d.Store.AppendLog("admin", 0, "delete_account", "删除账号 "+req.Account, true)
	writeJSON(w, 0, nil, "已删除账号 "+req.Account)
}

// handleAdminLogs 全量日志总览（不按账号过滤）。
func (d *Deps) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	logs, err := d.Store.LoadAllLogs(limit)
	if err != nil {
		writeJSON(w, 1, nil, "读取日志失败: "+err.Error())
		return
	}
	writeJSON(w, 0, logs, "")
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
	lastGC  time.Time // 上次惰性清理时间
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

const (
	loginRate  = 5.0 / 60.0 // 每分钟 5 次
	loginBurst = 5
	bucketTTL  = 10 * time.Minute // 空闲桶回收阈值
)

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{buckets: make(map[string]*tokenBucket)}
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	// 惰性清理：回收超过 10 分钟未活动的 IP 桶，防止公网扫描/代理轮换把桶表撑到 OOM
	// （偶发触发，O(桶数) 线性扫描可接受——桶数本应有界）
	if len(l.buckets) > 1024 && now.Sub(l.lastGC) > time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.lastFill) > bucketTTL {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}
	b, ok := l.buckets[ip]
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