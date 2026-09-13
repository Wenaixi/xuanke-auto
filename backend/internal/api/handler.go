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
	// AdminToken 管理口令（main 从环境变量/.env 注入，启动必填；管理员账号的密码）。
	Encrypt func(string) (string, error)
	// Decrypt 数据解密函数（main 注入：secure.Decrypt，vision_key 读回时解密）。
	Decrypt func(string) (string, error)
	AdminToken string
	// AdminName 管理员登录账号名（默认 admin，可用 XUANKE_ADMIN_NAME 改名）。
	AdminName string
	// ActivationEnabled 激活码机制是否启用（XUANKE_ACTIVATION=off 时完全禁用）。
	ActivationEnabled bool
}

// AdminNameValue 返回管理员账号名（默认 admin）。
func (d *Deps) AdminNameValue() string {
	if d.AdminName == "" {
		return "admin"
	}
	return d.AdminName
}

// IsAdminAccountName 判断账号名是否为管理员账号名（删除保护等硬判据）。
func (d *Deps) IsAdminAccountName(acct string) bool {
	return acct == d.AdminNameValue()
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
// handleLogin 登录：
//   - 管理员账号（默认 admin，可在 data/.env 用 XUANKE_ADMIN_NAME 改名）+ 管理口令 → 签发管理员会话（绕过教务登录，避免平台登录限流）
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
	adminName := d.AdminNameValue()
	// 管理员入口：默认账号 admin（可配置改名）+ 管理口令（不触碰教务登录，口令比对恒定时间防爆破）
	if req.Account == adminName {
		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(d.AdminToken)) != 1 {
			// 口令错误：恒定时间比对已抹平字节级差异（时序安全）。
			// n4（第 3 轮）：固定延迟 loginTimingFlat 抹平"管理员口令错（立即回）vs
			// 教务登录（网络往返）"的时延差——管理员账号名不再能靠响应快慢被侧信道枚举。
			time.Sleep(loginTimingFlat)
			writeJSON(w, 1, nil, "管理口令错误")
			return
		}
		sess := d.Sessions.CreateAdmin(adminName)
		d.Store.AppendLog(adminName, 0, "login", "管理员登录成功", true)
		writeJSON(w, 0, map[string]string{"token": sess, "account": adminName, "adminName": adminName}, "管理员登录成功")
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
			// C-2（第 3 轮）：教务登录成功即颁发短期单次激活票据（绑定本次登录账号），
			// 未激活账号的 /api/activate 必须携带它才能消耗激活码，杜绝持码者对任意已登录账号激活。
			ticket := d.Sessions.CreateTicket(req.Account)
			writeJSON(w, 1001, map[string]string{"ticket": ticket, "account": req.Account}, "该账号尚未激活，请输入激活码")
			return
		}
	}
	d.issueSession(w, req.Account)
}

// ActivateRequest 激活请求体（C-2 修复后：激活必须携带登录签发的短期票据）。
type ActivateRequest struct {
	Account string `json:"account"`
	Code    string `json:"code"`
	Ticket  string `json:"ticket"`
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
// C-2 修复（第 3 轮）：激活必须携带登录颁发的短期单次激活票据，且票据绑定账号
// 与本次激活账号必须一致——激活码从此绑定"刚通过教务登录的账号"，
// 不再允许持码者对任意已登录过本应用的账号名激活（学号可猜测的台账外接管已封堵）。
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
	if req.Account == "" || req.Code == "" || req.Ticket == "" {
		writeJSON(w, 1, nil, "账号、激活码与激活票据不能为空")
		return
	}
	acct := strings.TrimSpace(req.Account)
	// 票据必须有效（存在、未用尽）且绑定账号与本次激活账号一致
	if err := d.Sessions.ConsumeTicket(req.Ticket, acct); err != nil {
		writeJSON(w, 1, nil, "激活票据无效或已过期，请重新登录后再激活")
		return
	}
	ok, err := d.Store.ConsumeActivationCode(strings.TrimSpace(req.Code), acct)
	if err != nil {
		writeJSON(w, 1, nil, "激活失败: "+err.Error())
		return
	}
	if !ok {
		writeJSON(w, 1, nil, "激活码无效或已用尽")
		return
	}
	d.issueSession(w, acct)
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
// 支持 ?account= 参数，允许管理员任选指定账号的专属选课大厅（仅管理员会话可穿透）。
func (d *Deps) handleElectives(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	if d.allowAccountOverride(r) {
		if q := r.URL.Query().Get("account"); q != "" {
			acct = q
		} else if targetAccts := d.Sched.AccountsWithTargets(); len(targetAccts) > 0 {
			acct = targetAccts[0]
		}
	}
	if acct != "" && !d.IsAdminAccountName(acct) {
		if data, ok := d.Sched.ElectivesSnapshotFor(acct); ok {
			writeJSON(w, 0, data, "")
			return
		}
		data, err := d.Sched.ProbeForAccount(acct)
		if err != nil {
			writeJSON(w, 1, nil, "查询课程失败: "+err.Error())
			return
		}
		writeJSON(w, 0, data, "")
		return
	}
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

// ElectiveActionRequest 手动报名或退选请求体。
type ElectiveActionRequest struct {
	ClassID    int    `json:"class_id"`
	CourseName string `json:"course_name,omitempty"`
}

// handleElectiveSelect 手动报名指定课程 (POST /api/electives/select)
func (d *Deps) handleElectiveSelect(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	if d.allowAccountOverride(r) {
		if q := r.URL.Query().Get("account"); q != "" {
			acct = q
		}
	}
	if acct == "" || d.IsAdminAccountName(acct) {
		writeJSON(w, 1, nil, "请指定有效学生账号")
		return
	}

	var req ElectiveActionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.ClassID <= 0 {
		writeJSON(w, 1, nil, "无效的课程ID")
		return
	}

	// 1. 获取单课提交排他锁，防止与后台自动抢课并发冲突
	release, ok := d.Sched.TryAcquireSubmit(acct, req.ClassID)
	if !ok {
		writeJSON(w, 1, nil, "该课程正在提交中，请勿重复操作")
		return
	}
	defer release()

	// 2. 报名前按该账号最近快照复核窗口与满员状态（M7）：
	//    窗口关闭/课程满员时返回友好错误，避免无谓打教务平台拿生硬 code=1
	if reason, ok := d.Sched.CheckClassSelectable(acct, req.ClassID); !ok {
		writeJSON(w, 1, nil, reason)
		return
	}

	// 3. 获取该账号独立客户端
	client, ok := d.Accounts.ClientFor(acct)
	if !ok {
		writeJSON(w, 1, nil, "账号会话未建立或未登录")
		return
	}

	// 4. 调用教务平台真实报名接口
	msg, err := client.SelectClass(req.ClassID)
	if err != nil {
		writeJSON(w, 1, nil, err.Error())
		return
	}

	// 5. 报名成功：同步调度器 done 状态并持久化
	_ = d.Sched.MarkDone(acct, req.ClassID, req.CourseName, msg)
	writeJSON(w, 0, map[string]any{"msg": msg, "class_id": req.ClassID}, msg)
}

// handleElectiveExit 手动退选指定课程 (POST /api/electives/select/exit)
func (d *Deps) handleElectiveExit(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	if d.allowAccountOverride(r) {
		if q := r.URL.Query().Get("account"); q != "" {
			acct = q
		}
	}
	if acct == "" || d.IsAdminAccountName(acct) {
		writeJSON(w, 1, nil, "请指定有效学生账号")
		return
	}

	var req ElectiveActionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	if req.ClassID <= 0 {
		writeJSON(w, 1, nil, "无效的课程ID")
		return
	}

	// 1. 获取单课提交排他锁，防止并发冲突
	release, ok := d.Sched.TryAcquireSubmit(acct, req.ClassID)
	if !ok {
		writeJSON(w, 1, nil, "该课程正在操作中，请勿重复操作")
		return
	}
	defer release()

	// 2. 获取该账号独立客户端
	client, ok := d.Accounts.ClientFor(acct)
	if !ok {
		writeJSON(w, 1, nil, "账号会话未建立或未登录")
		return
	}

	// 3. 调用教务平台真实退选接口
	msg, err := client.ExitClass(req.ClassID)
	if err != nil {
		writeJSON(w, 1, nil, err.Error())
		return
	}

	// 4. 退选成功：从调度器 done 移除（后台自动引擎下个 tick 可重新接管），记日志
	_ = d.Sched.RemoveDone(acct, req.ClassID)
	writeJSON(w, 0, map[string]any{"msg": msg, "class_id": req.ClassID}, msg)
}

// TargetsRequest 设置目标请求体（账号由会话决定，不接收客户端传账号）。
type TargetsRequest struct {
	Targets []scheduler.Target `json:"targets"`
}

// maxTargetsPerAccount 每个账号目标课程条数上限（n2 防异常放大；前端选择远达不到）。
const maxTargetsPerAccount = 100

// handleSetTargets 设置目标课程并持久化（账号来自会话绑定；仅管理员会话可跨账号）。
func (d *Deps) handleSetTargets(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	if d.allowAccountOverride(r) {
		if q := r.URL.Query().Get("account"); q != "" {
			acct = q
		}
	}
	var req TargetsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 1, nil, "请求体解析失败: "+err.Error())
		return
	}
	// 允许清空所有目标课程（len == 0），满足用户随时重置预选目标的需求
	if req.Targets == nil {
		req.Targets = []scheduler.Target{}
	}
	// n2 修复（第 3 轮）：目标数量与范围双重校验——
	// 条数上限防恶意放大（每个账号最多 100 门，前端单选多选也远达不到）；
	// publish_id/priority 必须有界，防止越界值干扰调度器按发布分组与优先级排序。
	if len(req.Targets) > maxTargetsPerAccount {
		writeJSON(w, 1, nil, fmt.Sprintf("目标课程数量超过上限（最多 %d 门）", maxTargetsPerAccount))
		return
	}
	for _, t := range req.Targets {
		if t.ClassID <= 0 {
			writeJSON(w, 1, nil, "class_id 无效")
			return
		}
		if t.PublishID <= 0 {
			writeJSON(w, 1, nil, "publish_id 无效")
			return
		}
		if t.Priority < 0 || t.Priority > 999 {
			writeJSON(w, 1, nil, "priority 需在 0-999 之间")
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

// handleState 调度器状态（按会话账号过滤；仅管理员会话可跨账号）。
func (d *Deps) handleState(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	if d.allowAccountOverride(r) {
		if q := r.URL.Query().Get("account"); q != "" {
			acct = q
		} else if targetAccts := d.Sched.AccountsWithTargets(); len(targetAccts) > 0 {
			acct = targetAccts[0]
		}
	}
	st := d.Sched.StateForAccount(acct)
	writeJSON(w, 0, st, "")
}

// handleAccounts 当前会话账号视角的账号列表。
// M-2 修复（第 3 轮）：普通会话只回显自身绑定账号（学号即情报，杜绝账号枚举）；
// 管理员会话回显全量（多账号维护管理需要），与"日志按账号隔离"同隐私边界。
func (d *Deps) handleAccounts(w http.ResponseWriter, r *http.Request) {
	acct := sessionAccount(r)
	if d.allowAccountOverride(r) {
		writeJSON(w, 0, d.Accounts.Registered(), "")
		return
	}
	writeJSON(w, 0, []string{acct}, "")
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
// 注意：仅负责鉴权，不负责 JSON Content-Type 检查——副作用请求的 CSRF 防线
// 由 router 的 requireJSONBody 单独叠加（m8）。
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
		if req.Uses > 1000 {
			writeJSON(w, 1, nil, "每个激活码使用次数上限为 1000")
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

// newActivationCode 生成 XK-XXXX-XXXX-XXXX 格式激活码（16 位十六进制）。
// m7 修复：原 12 位 hex（48bit 熵）对有效期长的激活码偏低，提升至 16 位 hex（64bit 熵）。
// M-1 修复（第 3 轮）：crypto/rand 失败即 panic（与 randToken 同策略）——
// 熵源故障时代码绝不静默产出全零可预测激活码，让攻击者拿到重复码无限激活。
func newActivationCode() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand 不可用，无法生成安全激活码: " + err.Error())
	}
	s := strings.ToUpper(hex.EncodeToString(b))
	return fmt.Sprintf("XK-%s-%s-%s-%s", s[0:4], s[4:8], s[8:12], s[12:16])
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
	ActivationEnabled  bool   `json:"activation_enabled"`
	VisionBaseURL      string `json:"vision_base_url"`
	VisionAPIKey       string `json:"vision_api_key_masked"`
	VisionModel        string `json:"vision_model"`
	CaptchaEngine      string `json:"captcha_engine"`
	CaptchaConcurrency int    `json:"captcha_concurrency"`
	OpenTime           string `json:"open_time"`
}

// handleAdminConfig GET 读取 / PUT 热更新系统配置。
// PUT 立即写入运行时配置中心（调度器/账号管理器/Vision 同步生效）并落库 settings。
func (d *Deps) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := d.Runtime.Get()
		writeJSON(w, 0, AdminConfigView{
			ActivationEnabled:  cfg.ActivationEnabled,
			VisionBaseURL:      cfg.VisionBaseURL,
			VisionAPIKey:       maskKey(cfg.VisionAPIKey),
			VisionModel:        cfg.VisionModel,
			CaptchaEngine:      cfg.CaptchaEngine,
			CaptchaConcurrency: cfg.CaptchaConcurrency,
			OpenTime:           cfg.OpenTime,
		}, "")
	case http.MethodPut:
		var req struct {
			ActivationEnabled  *bool   `json:"activation_enabled"`
			VisionBaseURL      *string `json:"vision_base_url"`
			VisionAPIKey       *string `json:"vision_api_key"`
			VisionModel        *string `json:"vision_model"`
			CaptchaEngine      *string `json:"captcha_engine"`
			CaptchaConcurrency *int    `json:"captcha_concurrency"`
			OpenTime           *string `json:"open_time"`
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
			if req.CaptchaEngine != nil {
				c.CaptchaEngine = *req.CaptchaEngine
				changed = append(changed, "captcha_engine")
			}
			if req.CaptchaConcurrency != nil {
				if *req.CaptchaConcurrency < 1 {
					*req.CaptchaConcurrency = 1
				}
				c.CaptchaConcurrency = *req.CaptchaConcurrency
				changed = append(changed, "captcha_concurrency")
			}
			if req.OpenTime != nil {
				if _, err := scheduler.FormatOpenTime(*req.OpenTime); err == nil {
					c.OpenTime = *req.OpenTime
					changed = append(changed, "open_time")
				}
			}
		})
		cfg := d.Runtime.Get()
		// 先尝试落库（settings 全量替换，重启后恢复）。vision_key 加密落库：
		// 与凭据同强度（AES-256-GCM），settings 表内永不出现明文密钥。
		visionKey, vErr := d.secureEncrypt(cfg.VisionAPIKey)
		if vErr != nil {
			writeJSON(w, 1, nil, "配置加密失败: "+vErr.Error())
			return
		}
		// 先落库、后内存生效与下游下发（M-4：落库失败也要完成下发，杜绝半生效误导）。
		if sErr := d.saveSettings(map[string]string{
			"activation_enabled": strconv.FormatBool(cfg.ActivationEnabled),
			"vision_base_url":    cfg.VisionBaseURL,
			"vision_key":         visionKey,
			"vision_model":       cfg.VisionModel,
			"captcha_engine":     cfg.CaptchaEngine,
			"captcha_concurrency": strconv.Itoa(cfg.CaptchaConcurrency),
			"open_time":          cfg.OpenTime,
		}); sErr != nil {
			// M-4 修复（第 3 轮）：落库失败绝不静默——配置已内存生效，但重启即回退。
			// 如实返回 500 让管理员立即知晓持久化失败；不再跳过下游热下发，
			// 识别引擎/Vision 仍按新配置同步给账号客户端，杜绝"半生效"误导。
			log.Printf("[api] 配置落库失败: %v", sErr)
			d.dispatchRuntimeConfig(cfg)
			writeJSON(w, 500, nil, "配置已生效但落库失败（重启后将回退）："+sErr.Error())
			return
		}
		// 热重载下游组件：验证码识别配置推给全部账号客户端；打开时间由调度器运行时读取
		d.dispatchRuntimeConfig(cfg)
		if len(changed) == 0 {
			writeJSON(w, 1, nil, "没有可应用的有效配置项")
			return
		}
		d.Store.AppendLog("admin", 0, "config", "更新配置: "+strings.Join(changed, ", "), true)
		writeJSON(w, 0, AdminConfigView{
			ActivationEnabled:  cfg.ActivationEnabled,
			VisionBaseURL:      cfg.VisionBaseURL,
			VisionAPIKey:       maskKey(cfg.VisionAPIKey),
			VisionModel:        cfg.VisionModel,
			CaptchaEngine:      cfg.CaptchaEngine,
			CaptchaConcurrency: cfg.CaptchaConcurrency,
			OpenTime:           cfg.OpenTime,
		}, "配置已更新并生效")
	default:
		writeJSON(w, 405, nil, "方法不允许")
	}
}

// dispatchRuntimeConfig 把运行时配置热下发到下游组件（M-4 提取）：
// 验证码识别配置推给全部账号客户端 + 识别引擎热切换（ddddocr 本地 / Vision 二选一）
// + 并发限流信号量热收敛 + 调度器开放时间由运行时逐 tick 读取（无需显式通知）。
// 放在"落库成功"与"落库失败但内存已生效"两条路径共用——半生效绝不静默。
func (d *Deps) dispatchRuntimeConfig(cfg runtime.Config) {
	// d.Store 为 nil 时跳过（测试直接直构 Deps 的场景；生产恒非 nil）
	if d.Accounts != nil {
		d.Accounts.SetVision(zhidao.VisionConfig{
			BaseURL: cfg.VisionBaseURL, APIKey: cfg.VisionAPIKey, Model: cfg.VisionModel,
		})
		applyCaptchaRecognizerFor(d.Runtime, d.Accounts)
	}
}

// saveSettingsErrForTest 测试注入钩子（仅测试包内使用）：置非 nil 时 saveSettings 直接
// 返回该错误，模拟 settings 落库失败——M-4 验证"内存生效但落库失败"时下游热下发不被跳过。
var saveSettingsErrForTest error

// saveSettings 配置落库（settings 全量替换）。经此间接方法路由，便于测试注入落库失败
// （M-4：验证"落库失败但内存生效"时下游热下发不被跳过）。
func (d *Deps) saveSettings(kv map[string]string) error {
	if saveSettingsErrForTest != nil {
		return saveSettingsErrForTest
	}
	return d.Store.SaveSettings(kv)
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
	// WindowOpened 返回布尔快照（不再返回裸指针，评审 MAJOR 已根除指针悬空竞态）。
	windowOpened := d.Sched.WindowOpened()
	// 全账号日志总数（LoadAllLogs 含全部账号）
	logsCount := len(allLogs)
	// 识别引擎与并发上限（管理员后台展示当前生效值）
	eng := cfg.CaptchaEngine
	if eng == "" {
		eng = "ddddocr"
	}
	// N5：各账号教务 token 有效性汇总（管理员后台一眼看到哪些账号 token 失效/恢复中）。
	// 用调度器对外方法逐一查询（含 relogining 半态），不直接读内部 map。
	tokValid := map[string]bool{}
	for _, a := range accounts {
		tokValid[a] = d.Sched.TokenValidFor(a)
	}
	writeJSON(w, 0, map[string]any{
		"open_time":           open.Format("2006-01-02 15:04:05"),
		"activation_on":       cfg.ActivationEnabled,
		"window_opened":       windowOpened,
		"account_count":       len(accounts),
		"targets_count":       targetsCount,
		"success_count":       len(success),
		"log_count":           logsCount,
		"vision_model":        cfg.VisionModel,
		"vision_base_url":     cfg.VisionBaseURL,
		"captcha_engine":      eng,
		"captcha_concurrency": cfg.CaptchaConcurrency,
		"token_valid":         tokValid,
		// N6：open_time_set 按开放时间是否真被配置输出，不再恒 true——
		// 管理员未设置开放时间时前端如实显示"未设置"，避免误导
		"open_time_set": !open.IsZero(),
	}, "")
}

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
	if strings.TrimSpace(req.Account) == "" || d.IsAdminAccountName(req.Account) {
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
	// 吊销该账号签发的全部会话（MAJOR-A）：被删账号既有浏览器令牌立即失效，
	// 等不到 12h TTL——"删除"对已持有 token 的客户端不再形同虚设。
	d.Sessions.RevokeAccount(req.Account)
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
const sessionTokCtxKey ctxKey = 2

// sessionAccount 从请求上下文取会话绑定的账号。
func sessionAccount(r *http.Request) string {
	if v, ok := r.Context().Value(sessionCtxKey).(string); ok {
		return v
	}
	return ""
}

// sessionToken 从请求上下文取当前会话令牌（穿透判定用）。
func sessionToken(r *http.Request) string {
	if v, ok := r.Context().Value(sessionTokCtxKey).(string); ok {
		return v
	}
	return ""
}

// allowAccountOverride 当前会话是否允许 ?account= 穿透到任意账号：
// 仅管理员会话（会话身份 Admin:true）可穿透，普通会话一律只操作自己绑定账号。
func (d *Deps) allowAccountOverride(r *http.Request) bool {
	return d.Sessions.IsAdminToken(sessionToken(r))
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
		ctx := r.Context()
		ctx = context.WithValue(ctx, sessionCtxKey, acct)
		ctx = context.WithValue(ctx, sessionTokCtxKey, tok)
		next(w, r.WithContext(ctx))
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
	// loginTimingFlat 登录响应时延拉平值（n4）：管理员口令错误分支固定延迟，
	// 抹平"管理员口令错（立即回）vs 教务登录（网络往返）"的时延差——管理员账号名
	// 不再能靠响应快慢被侧信道枚举。取值 300ms 与教务登录同量级（Vision+网络往返）。
	loginTimingFlat = 300 * time.Millisecond
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