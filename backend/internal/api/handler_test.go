package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

// 构造测试用依赖：内存 db + mock zhidao client（用真实 client 但指向 httptest mock 服务器）
type testDeps struct {
	srv   *httptest.Server
	store *store.Store
	api   http.Handler
	sched *scheduler.Scheduler
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()
	// mock 至道服务器：findElectivesData / login 等
	zhi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/electives/select"):
			// 学期列表
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

	client := zhidao.New(zhi.URL, zhidao.VisionConfig{BaseURL: zhi.URL, APIKey: "k", Model: "m"})
	client.SetCredentials("acct", "pwd", "tok")

	openTime, err := scheduler.FormatOpenTime("2026-09-13 09:00:00")
	if err != nil {
		t.Fatal(err)
	}
	sched := scheduler.New(client, st, openTime, time.Hour) // 测试不自动轮询
	sched.Start()
	t.Cleanup(sched.Stop)

	mux := http.NewServeMux()
	apiHandler := Register(mux, st, client, sched, "2026-09-13 09:00:00")
	return &testDeps{srv: zhi, store: st, api: apiHandler, sched: sched}
}

func doJSON(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	return rec.Code, j
}

func TestHealth(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "GET", "/api/health", "")
	if code != 200 || j["code"].(float64) != 0 || j["data"] != "ok" {
		t.Fatalf("health 异常: %d %v", code, j)
	}
}

func TestElectives(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "GET", "/api/electives", "")
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("electives 异常: %d %v", code, j)
	}
	data, ok := j["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 缺失: %v", j)
	}
	pubs, ok := data["publishes"].([]any)
	if !ok || len(pubs) == 0 {
		t.Fatalf("publishes 缺失: %v", data)
	}
}

func TestElectivesDetail(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "GET", "/api/electives/detail?id=61115", "")
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("detail 异常: %d %v", code, j)
	}
}

func TestSetTargetsAndState(t *testing.T) {
	d := newTestDeps(t)
	body := `{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操"}]}`
	code, j := doJSON(t, d.api, "PUT", "/api/targets", body)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("set targets 异常: %d %v", code, j)
	}
	// 持久化验证
	targets, err := d.store.LoadTargets()
	if err != nil || len(targets) != 1 || targets[0].ClassID != 61115 {
		t.Fatalf("目标未持久化: %v %v", targets, err)
	}
	// state 验证
	code, j = doJSON(t, d.api, "GET", "/api/state", "")
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("state 异常: %d %v", code, j)
	}
}

func TestSetTargetsEmptyAllowed(t *testing.T) {
	d := newTestDeps(t)
	code, j := doJSON(t, d.api, "PUT", "/api/targets", `{"targets":[]}`)
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
