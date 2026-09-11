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
	apiHandler := Register(mux, st, sched, accts, sessions, "2026-09-13 09:00:00", testAdminToken,
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

// loginAndGetToken 用部署口令 + 账密登录，返回会话令牌。
func loginAndGetToken(t *testing.T, d *testDeps, acct string) string {
	t.Helper()
	body := `{"account":"` + acct + `","password":"pwd","admin_token":"` + testAdminToken + `"}`
	code, j := doJSON(t, d.api, "POST", "/api/login", body)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("登录失败: %d %v", code, j)
	}
	data, ok := j["data"].(map[string]any)
	if !ok {
		t.Fatalf("登录响应缺 data: %v", j)
	}
	return data["token"].(string)
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

func TestLoginBadAdmin(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"a","password":"b","admin_token":"wrong"}`)
	if code != 200 || j["code"].(float64) != 403 {
		t.Fatalf("错误部署口令应 403: %d %v", code, j)
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
		_, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"a","password":"b","admin_token":"wrong"}`)
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