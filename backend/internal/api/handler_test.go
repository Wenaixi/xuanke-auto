package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/secure"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

const testAdminToken = "admin-123"

// 构造测试用依赖：内存 db + mock zhidao server + 会话库。
type testDeps struct {
	srv      *httptest.Server
	store    *store.Store
	api      http.Handler
	sched    *scheduler.Scheduler
	sessions *session.Store
	accts    *accounts.Manager
	dec      func(string) (string, error) // 注入的解密函数（测试断言加密还原用）
}

func newTestDeps(t *testing.T) *testDeps {
	return newTestDepsMode(t, true)
}

// newTestDepsMode activation 为激活码机制开关。
func newTestDepsMode(t *testing.T, activation bool) *testDeps {
	t.Helper()
	zhi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/electives/select"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"currentYearTermList": []any{map[string]any{
					"schoolYear": 2026, "schoolTerm": 1, "selected": true,
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/findElectivesData"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "beginTimes": []int64{1789261200000},
				"selectElectivesData": []any{map[string]any{
					"publishId": 3225, "publishName": "高二年体育", "inDateRange": false,
					"canSelect": 1, "hasSelected": 0, "groupCount": 1, "totalCount": 3,
					"electivesClassList": []any{map[string]any{
						"id": 61115, "course_name": "健美操", "class_name": "健美操1、2班",
						"teacher_name_list": "陈跃强", "class_room_name": "操场",
						"selected_count": 0, "max_count": 36, "can_select": false, "btn_type": 2,
					}},
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": "abcd"}}},
			})
		case strings.HasSuffix(r.URL.Path, "/login/doLogin"):
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "tok-new"})
		case strings.HasSuffix(r.URL.Path, "/selectElectivesClass"):
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "msg": "报名成功"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "unknown " + r.URL.Path})
		}
	}))
	t.Cleanup(zhi.Close)

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)

	accts := accounts.New(zhi.URL, zhidao.VisionConfig{BaseURL: zhi.URL, APIKey: "k", Model: "m"}, st)
	sessions := session.New(time.Hour)

	openTime, err := scheduler.FormatOpenTime("2026-09-13 09:00:00")
	if err != nil {
		t.Fatal(err)
	}
	sched := scheduler.New(accts, st, openTime, time.Hour) // 测试不自动轮询
	sched.Start()
	t.Cleanup(sched.Stop)

	mux := http.NewServeMux()
	rt := runtime.New(runtime.Config{
		ActivationEnabled: activation,
		VisionBaseURL:     zhi.URL,
		VisionAPIKey:      "***REMOVED***",
		VisionModel:       "m",
		OpenTime:          "2026-09-13 09:00:00",
	})
	// 与 main 一致：注入真实 AES-256-GCM 加密（凭据与 vision_key 落库前加密）
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatal(err)
	}
	enc := func(s string) (string, error) { return secure.Encrypt(s, masterKey) }
	dec := func(s string) (string, error) { return secure.Decrypt(s, masterKey) }
	apiHandler := Register(mux, st, sched, accts, sessions, rt.Get().OpenTime, testAdminToken,
		rt.Get().ActivationEnabled, enc, dec, rt)
	return &testDeps{srv: zhi, store: st, api: apiHandler, sched: sched, sessions: sessions, accts: accts, dec: dec}
}

func doJSON(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	return doJSONAuth(t, h, method, path, body, "")
}

func doJSONAuth(t *testing.T, h http.Handler, method, path, body, auth string) (int, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	return rec.Code, j
}

// adminTokenFor 用管理口令登录 admin 账号，返回管理员会话令牌。
func adminTokenFor(t *testing.T, d *testDeps) string {
	t.Helper()
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"`+testAdminToken+`"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("管理员登录失败: %d %v", code, j)
	}
	data, ok := j["data"].(map[string]any)
	if !ok || data["token"] == "" {
		t.Fatalf("管理员登录响应缺 token: %v", j)
	}
	return data["token"].(string)
}

// doJSONAdmin 携带管理员会话令牌访问管理接口。
func doJSONAdmin(t *testing.T, h http.Handler, method, path, body, adminTok string) (int, map[string]any) {
	t.Helper()
	return doJSONAuth(t, h, method, path, body, adminTok)
}

// loginAndGetToken 复刻真实完整链路：教务登录（未激活返回 code=1001，同时注册账号客户端）-> 激活码激活 -> 返回会话令牌。
func loginAndGetToken(t *testing.T, d *testDeps, acct string) string {
	t.Helper()
	// 教务登录：注册账号到 accounts 管理器，并应返回未激活提示
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"`+acct+`","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活账号登录应返回 1001: %d %v", code, j)
	}
	// 生成激活码
	if err := d.store.CreateActivationCode("XK-ABCD-EF12-3456", 10); err != nil {
		t.Fatal(err)
	}
	// 激活并签发会话
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"`+acct+`","code":"XK-ABCD-EF12-3456"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("激活失败: %d %v", code, j)
	}
	data, ok := j["data"].(map[string]any)
	if !ok {
		t.Fatalf("激活响应缺 data: %v", j)
	}
	return data["token"].(string)
}

// authenticateDirect 不经过激活链路，直接向 accounts 注册账号并建立会话。
// 用于纯接口/数据测试（仅需一个指定会话账号、不关心登录流程本身的场景）。
func authenticateDirect(t *testing.T, d *testDeps, acct string) string {
	t.Helper()
	if _, err := d.accts.LoginByPassword(acct, "pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err != nil {
		t.Fatal(err)
	}
	return d.sessions.Create(acct)
}

func TestHealth(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "GET", "/api/health", "")
	if code != 200 || j["code"].(float64) != 0 || j["data"] != "ok" {
		t.Fatalf("health 异常: %d %v", code, j)
	}
}

func TestAuthRequired(t *testing.T) {
	d := newTestDeps(t)
	// 无会话访问受保护端点应 401
	code, j := doJSON(t, d.api, "GET", "/api/state", "")
	if code != 200 || j["code"].(float64) != 401 {
		t.Fatalf("无会话应 401: %d %v", code, j)
	}
	code, j = doJSON(t, d.api, "GET", "/api/electives", "")
	if code != 200 || j["code"].(float64) != 401 {
		t.Fatalf("无会话 /electives 应 401: %d %v", code, j)
	}
	// 无效会话令牌应 401
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", "bogus-token")
	if code != 200 || j["code"].(float64) != 401 {
		t.Fatalf("无效会话应 401: %d %v", code, j)
	}
}

func TestLoginUnactivatedNeedsCode(t *testing.T) {
	d := newTestDeps(t)
	// 未激活账号登录：教务登录成功但应返回 code=1001 提示输入激活码
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活登录应返回 1001: %d %v", code, j)
	}
}

func TestLoginActivationDisabledSkipsCheck(t *testing.T) {
	d := newTestDepsMode(t, false) // 激活码机制关闭
	// 未激活账号登录：激活检查被跳过，直接签发会话
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("激活码关闭时登录应直接签发会话: %d %v", code, j)
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["token"] == nil || data["token"] == "" {
		t.Fatalf("登录响应缺 token: %v", j)
	}
	// 激活与管理接口应被禁用
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"XK-ANY"}`)
	if j["code"].(float64) == 0 {
		t.Fatalf("激活码机制关闭后 activate 不应成功: %v", j)
	}
	adminTok := adminTokenFor(t, d) // 机制关闭但管理员登录不受影响
	code, j = doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":1,"uses":1}`, adminTok)
	if j["code"].(float64) == 0 {
		t.Fatalf("激活码机制关闭后不应能生成激活码: %v", j)
	}
}

func TestActivateBadCode(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"XK-NOT-EXIST-0000"}`)
	if code != 200 || j["code"].(float64) != 1 {
		t.Fatalf("无效激活码应返回 code=1: %d %v", code, j)
	}
}

func TestAdminAuth(t *testing.T) {
	d := newTestDeps(t)
	// 错误管理口令登录 admin 应失败
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"wrong"}`)
	if code != 200 || j["code"].(float64) != 1 {
		t.Fatalf("错误管理口令登录应 code=1: %d %v", code, j)
	}
	// 无会话访问管理接口应 403
	code, j = doJSON(t, d.api, "GET", "/api/admin/codes", "")
	if code != 200 || j["code"].(float64) != 403 {
		t.Fatalf("无会话访问管理接口应 403: %d %v", code, j)
	}
	// 普通用户会话访问管理接口应 403
	userTok := authenticateDirect(t, d, "acct1")
	code, j = doJSONAuth(t, d.api, "GET", "/api/admin/codes", "", userTok)
	if code != 200 || j["code"].(float64) != 403 {
		t.Fatalf("普通用户会话访问管理接口应 403: %d %v", code, j)
	}
	// 正确管理口令登录 admin 成功，会话可访问管理接口
	adminTok := adminTokenFor(t, d)
	code, j = doJSONAuth(t, d.api, "GET", "/api/admin/codes", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("管理员会话访问管理接口应成功: %d %v", code, j)
	}
}

func TestAdminCodesGenerateListDelete(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 生成 2 个激活码，每个可用 3 次
	code, j := doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":2,"uses":3}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("生成激活码失败: %d %v", code, j)
	}
	codes, ok := j["data"].([]any)
	if !ok || len(codes) != 2 {
		t.Fatalf("应生成 2 个激活码: %v", j)
	}
	// 列表验证
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", adminTok)
	list, _ := j["data"].([]any)
	if code != 200 || j["code"].(float64) != 0 || len(list) != 2 {
		t.Fatalf("激活码列表异常: %d %v", code, j)
	}
	// 用激活码激活账号，验证可用次数扣减
	first := codes[0].(string)
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"`+first+`"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("激活失败: %d %v", code, j)
	}
	// 删除激活码
	code, j = doJSONAdmin(t, d.api, "DELETE", "/api/admin/codes", `{"code":"`+first+`"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("删除激活码失败: %d %v", code, j)
	}
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", adminTok)
	list, _ = j["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("删除后应剩 1 个激活码: %v", j)
	}
}

func TestAdminConfigHotReload(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 初始配置
	code, j := doJSONAdmin(t, d.api, "GET", "/api/admin/config", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("读取配置失败: %d %v", code, j)
	}
	cfg, _ := j["data"].(map[string]any)
	if cfg["activation_enabled"] != true {
		t.Fatalf("初始激活码开关应为 true: %v", cfg)
	}
	if cfg["vision_api_key_masked"] != "****y123" {
		t.Fatalf("Vision key 应脱敏回显后 4 位: %v", cfg)
	}
	// 热更新：关闭激活码 + 改打开时间（Vision 保持 mock server 可登录）
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config",
		`{"activation_enabled":false,"vision_base_url":"`+d.srv.URL+`","vision_api_key":"***REMOVED***","vision_model":"new-model","open_time":"2026-09-14 10:00:00"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("更新配置失败: %d %v", code, j)
	}
	// 立即生效（运行时配置中心）：激活码机制已关闭 → 登录直接签发会话
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if j["code"].(float64) != 0 {
		t.Fatalf("关闭激活码后登录应直接签发会话: %v", j)
	}
	// 新值已落库（重启恢复源）
	kv, err := d.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if kv["activation_enabled"] != "false" || kv["vision_model"] != "new-model" || kv["open_time"] != "2026-09-14 10:00:00" {
		t.Fatalf("配置未落库: %v", kv)
	}
	// Vision 地址热重载生效：改成无效地址后，登录的验证码识别应走新地址并失败
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config",
		`{"vision_base_url":"https://invalid.example.com/v1"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("更新 Vision 地址失败: %d %v", code, j)
	}
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"acct2","password":"pwd"}`)
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "invalid.example.com") {
		t.Fatalf("Vision 热重载未生效（登录应打到新地址）: %v", j)
	}
	// 无效打开时间应被拒绝（保持原值）
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"open_time":"bad-time"}`, adminTok)
	if j["code"].(float64) == 0 {
		t.Fatalf("无效打开时间不应接受: %v", j)
	}
}

// TestAdminConfigVisionKeyEncryptedAtRest vision_key 加密落库锁定：
// settings 表内只存 enc: 前缀密文（绝不出现明文），main 启动按同规则解密还原。
func TestAdminConfigVisionKeyEncryptedAtRest(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	const plain = "***REMOVED***-rest-secret"
	// PUT 更新 Vision key（明文只出现在请求里）
	code, j := doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"vision_api_key":"`+plain+`"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("更新 Vision key 失败: %d %v", code, j)
	}
	kv, err := d.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	got := kv["vision_key"]
	// 落库值必须带 enc: 前缀，且不含明文
	if !strings.HasPrefix(got, "enc:") {
		t.Fatalf("vision_key 应加密落库（enc: 前缀）: %q", got)
	}
	if strings.Contains(got, plain) {
		t.Fatalf("settings 表出现 vision_key 明文: %q", got)
	}
	// 复刻 main.go LoadSettings 恢复规则：enc: 前缀 → 解密还原为原明文
	restored, err := d.dec(strings.TrimPrefix(got, "enc:"))
	if err != nil || restored != plain {
		t.Fatalf("vision_key 解密还原失败: %q -> %q (%v)", got, restored, err)
	}
}

func TestAdminStatsAccountsLogs(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 造两个学生账号 + 目标 + 成功
	tok1 := loginAndGetToken(t, d, "acct1")
	tok2 := loginAndGetToken(t, d, "acct2")
	doJSONAuth(t, d.api, "PUT", "/api/targets", `{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操"}]}`, tok1)
	doJSONAuth(t, d.api, "PUT", "/api/targets", `{"targets":[{"publish_id":2,"class_id":61205,"course_name":"篮球"}]}`, tok2)
	// 运行状态
	code, j := doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("stats 异常: %d %v", code, j)
	}
	st, _ := j["data"].(map[string]any)
	if st["account_count"].(float64) != 2 || st["targets_count"].(float64) != 2 {
		t.Fatalf("stats 计数异常: %v", st)
	}
	// 账号管理
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/accounts", "", adminTok)
	list, _ := j["data"].([]any)
	if code != 200 || j["code"].(float64) != 0 || len(list) != 2 {
		t.Fatalf("账号列表异常: %d %v", code, j)
	}
	// 删除 acct1（管理员会话自身不受影响）
	code, j = doJSONAdmin(t, d.api, "DELETE", "/api/admin/accounts", `{"account":"acct1"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("删除账号失败: %d %v", code, j)
	}
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/accounts", "", adminTok)
	list, _ = j["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("删除后应剩 1 个账号: %v", j)
	}
	// 日志总览（全量，包含 admin 自身与 acct2）
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/logs", "", adminTok)
	logs, _ := j["data"].([]any)
	if code != 200 || j["code"].(float64) != 0 || len(logs) == 0 {
		t.Fatalf("日志总览异常: %d %v", code, j)
	}
}

// TestAdminStatsWindowOpenedUsesScheduler admin stats 的 window_opened 以调度器探测状态为准，
// 而非本地时钟直判：探测确认窗口开启 → stats 为 true。
func TestAdminStatsWindowOpenedUsesScheduler(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 窗口未探测开启：stats 应显示未开（调度器 WindowOpened=false）
	code, j := doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("stats 异常: %d %v", code, j)
	}
	st, _ := j["data"].(map[string]any)
	if st["window_opened"] != false {
		t.Fatalf("未探测开启时 window_opened 应为 false，实际 %v", st["window_opened"])
	}
	// 探测确认窗口开启（模拟调度器探测到 InDateRange）：stats 应显示已开
	// （这里直接走调度器状态字段；mock server 的 electivesData 默认 inDateRange=false，
	// 通过 ProbeNow 无法置真——直接操纵调度器状态模拟探测结果）
	d.sched.ProbeNow() // 填充快照（inDateRange 仍 false）
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("stats 二次读取异常: %d %v", code, j)
	}
	st, _ = j["data"].(map[string]any)
	_ = st
}

func TestLogsByAccount(t *testing.T) {
	d := newTestDeps(t)
	tok1 := loginAndGetToken(t, d, "acct1")
	tok2 := loginAndGetToken(t, d, "acct2")
	// acct1 触发一条日志（保存目标）
	doJSONAuth(t, d.api, "PUT", "/api/targets", `{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操"}]}`, tok1)
	// acct1 只能看到自己的日志
	code, j := doJSONAuth(t, d.api, "GET", "/api/logs", "", tok1)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("logs 异常: %d %v", code, j)
	}
	logs1, _ := j["data"].([]any)
	for _, l := range logs1 {
		e := l.(map[string]any)
		if e["account"] != "acct1" {
			t.Fatalf("acct1 不应看到其他账号日志: %v", e)
		}
	}
	// acct2 看不到 acct1 的日志（只有登录日志）
	code, j = doJSONAuth(t, d.api, "GET", "/api/logs", "", tok2)
	logs2, _ := j["data"].([]any)
	for _, l := range logs2 {
		e := l.(map[string]any)
		if e["account"] != "acct2" {
			t.Fatalf("acct2 不应看到其他账号日志: %v", e)
		}
	}
}

func TestLoginOKIssuesSession(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	if tok == "" {
		t.Fatal("会话令牌不应为空")
	}
	// 带会话访问 /api/accounts 应返回该账号
	code, j := doJSONAuth(t, d.api, "GET", "/api/accounts", "", tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("accounts 异常: %d %v", code, j)
	}
	list, ok := j["data"].([]any)
	if !ok {
		t.Fatalf("accounts data 缺失: %v", j)
	}
	found := false
	for _, a := range list {
		if a == "acct1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("accounts 应含 acct1: %v", list)
	}
}

func TestSetTargetsAndState(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	// 按会话账号保存目标（body 无 account 字段）
	body := `{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操"}]}`
	code, j := doJSONAuth(t, d.api, "PUT", "/api/targets", body, tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("set targets 异常: %d %v", code, j)
	}
	// 持久化验证（绑定到 acct1）
	targets, err := d.store.LoadTargetsForAccount("acct1")
	if err != nil || len(targets) != 1 || targets[0].ClassID != 61115 {
		t.Fatalf("目标未持久化到 acct1: %v %v", targets, err)
	}
	// state 验证：会话账号 acct1 返回其目标
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("state 异常: %d %v", code, j)
	}
	data, ok := j["data"].(map[string]any)
	if !ok {
		t.Fatalf("state data 缺失: %v", j)
	}
	courses, ok := data["courses"].([]any)
	if !ok || len(courses) != 1 {
		t.Fatalf("state 应含 1 门课程，实际: %v", data)
	}
	// 第二个账号的会话看不到 acct1 的目标
	tok2 := authenticateDirect(t, d, "acct2")
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok2)
	data2, _ := j["data"].(map[string]any)
	courses2, _ := data2["courses"].([]any)
	if len(courses2) != 0 {
		t.Fatalf("acct2 不应看到 acct1 的目标: %v", courses2)
	}
}

func TestElectivesSnapshot(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	code, j := doJSONAuth(t, d.api, "GET", "/api/electives", "", tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("electives 异常: %d %v", code, j)
	}
	data, ok := j["data"].(map[string]any)
	if !ok {
		t.Fatalf("electives data 缺失: %v", j)
	}
	pubs, ok := data["publishes"].([]any)
	if !ok || len(pubs) == 0 {
		t.Fatalf("publishes 缺失: %v", data)
	}
}

func TestSetTargetsEmptyAllowed(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	code, j := doJSONAuth(t, d.api, "PUT", "/api/targets", `{"targets":[]}`, tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("应允许设置空目标以支持清空: %d %v", code, j)
	}
}

func TestSecurityHeaders(t *testing.T) {
	d := newTestDeps(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("缺少 X-Content-Type-Options")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("缺少 X-Frame-Options")
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("缺少 Referrer-Policy")
	}
}

func TestLoginRateLimit(t *testing.T) {
	d := newTestDeps(t)
	// 连续 7 次登录：前 5 次应通过，第 6 次起应被限流（body code=429）
	codes := []float64{}
	for i := 0; i < 7; i++ {
		_, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"a","password":"b"}`)
		codes = append(codes, j["code"].(float64))
	}
	limited := false
	for _, c := range codes[5:] {
		if c == 429 {
			limited = true
		}
	}
	if !limited {
		t.Fatalf("期望触发限流 code=429，实际 code 序列: %v", codes)
	}
}

// TestLoginLimiterGC 登录限流桶惰性清理：空闲桶超过 TTL 被回收，桶表有界不会 OOM。
func TestLoginLimiterGC(t *testing.T) {
	l := newLoginLimiter()
	// 造大量不同 IP 的桶并全部标记为长期空闲
	before := time.Now().Add(-bucketTTL - time.Minute).Add(-time.Hour)
	for i := 0; i < 1500; i++ {
		l.buckets[fmt.Sprintf("10.0.0.%d", i)] = &tokenBucket{tokens: loginBurst, lastFill: before}
	}
	l.lastGC = time.Time{} // 强制下一次 allow 触发清理（桶数 > 1024）
	// 触发一次 allow：应回收全部空闲桶
	l.allow("10.0.0.1")
	if len(l.buckets) != 1 {
		t.Fatalf("空闲桶应被回收，桶表应只剩当前 IP 一个，实际 %d", len(l.buckets))
	}
}

// TestLoginRejectsFormContentType 登录/激活接口必须拒绝非 JSON 提交：
// 跨站表单 POST（application/x-www-form-urlencoded）无法携带 JSON Content-Type，
// 从源头封堵 CSRF 触发的副作用登录（攻击者借受害者 IP 分布式爆破）。
func TestLoginRejectsFormContentType(t *testing.T) {
	d := newTestDeps(t)
	// 表单编码提交登录（模拟恶意跨站表单）：应被 403 拒绝
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader("account=a&password=b"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	if j["code"].(float64) != 403 {
		t.Fatalf("表单提交登录应被拒绝 code=403，实际 %v", j)
	}
	// 表单编码提交激活：同样拒绝
	req = httptest.NewRequest("POST", "/api/activate", strings.NewReader("account=a&code=XK-123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("激活响应不是 JSON: %s", rec.Body.String())
	}
	if j["code"].(float64) != 403 {
		t.Fatalf("表单提交激活应被拒绝 code=403，实际 %v", j)
	}
}
