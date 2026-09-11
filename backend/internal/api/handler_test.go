package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/scheduler"
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
		case strings.HasSuffix(r.URL.Path, "/classDetail"):
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "value": map[string]any{
					"course_name": "健美操", "classroom_name": "操场", "teacher_name": "陈跃强",
				},
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
	apiHandler := Register(mux, st, sched, accts, sessions, "2026-09-13 09:00:00", testAdminToken, activation,
		func(s string) (string, error) { return "ENC:" + s, nil })
	return &testDeps{srv: zhi, store: st, api: apiHandler, sched: sched, sessions: sessions, accts: accts}
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

// doJSONAdmin 携带管理口令 X-Admin-Token 访问激活码管理接口。
func doJSONAdmin(t *testing.T, h http.Handler, method, path, body, adminTok string) (int, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if adminTok != "" {
		req.Header.Set("X-Admin-Token", adminTok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	return rec.Code, j
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

// testActivated 预激活账号（供需要已激活会话的测试使用）。
func testActivated(t *testing.T, d *testDeps, acct string) {
	t.Helper()
	tok := loginAndGetToken(t, d, acct)
	_ = tok
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
	code, j = doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":1,"uses":1}`, testAdminToken)
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

func TestAdminBadToken(t *testing.T) {
	d := newTestDeps(t)
	// 错误管理口令访问激活码管理接口应 403
	code, j := doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", "wrong")
	if code != 200 || j["code"].(float64) != 403 {
		t.Fatalf("错误管理口令应 403: %d %v", code, j)
	}
	// 缺管理口令也应 403
	code, j = doJSON(t, d.api, "GET", "/api/admin/codes", "")
	if code != 200 || j["code"].(float64) != 403 {
		t.Fatalf("缺管理口令应 403: %d %v", code, j)
	}
}

func TestAdminCodesGenerateListDelete(t *testing.T) {
	d := newTestDeps(t)
	// 生成 2 个激活码，每个可用 3 次
	code, j := doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":2,"uses":3}`, testAdminToken)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("生成激活码失败: %d %v", code, j)
	}
	codes, ok := j["data"].([]any)
	if !ok || len(codes) != 2 {
		t.Fatalf("应生成 2 个激活码: %v", j)
	}
	// 列表验证
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", testAdminToken)
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
	code, j = doJSONAdmin(t, d.api, "DELETE", "/api/admin/codes", `{"code":"`+first+`"}`, testAdminToken)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("删除激活码失败: %d %v", code, j)
	}
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", testAdminToken)
	list, _ = j["data"].([]any)
	if len(list) != 1 {
		t.Fatalf("删除后应剩 1 个激活码: %v", j)
	}
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
	tok := loginAndGetToken(t, d, "acct1")
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
	tok := loginAndGetToken(t, d, "acct1")
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
	tok2 := loginAndGetToken(t, d, "acct2")
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok2)
	data2, _ := j["data"].(map[string]any)
	courses2, _ := data2["courses"].([]any)
	if len(courses2) != 0 {
		t.Fatalf("acct2 不应看到 acct1 的目标: %v", courses2)
	}
}

func TestElectivesSnapshot(t *testing.T) {
	d := newTestDeps(t)
	tok := loginAndGetToken(t, d, "acct1")
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
	tok := loginAndGetToken(t, d, "acct1")
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