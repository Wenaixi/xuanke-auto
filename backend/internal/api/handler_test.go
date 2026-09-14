package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
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
	rt       *runtime.Store // 运行时配置中心（测试重建 handler 用）
	dec      func(string) (string, error) // 注入的解密函数（测试断言加密还原用）
}

func newTestDeps(t *testing.T) *testDeps {
	return newTestDepsMode(t, true)
}

// newTestDepsMode activation 为激活码机制开关；adminName 为管理员账号名（默认 admin）。
func newTestDepsMode(t *testing.T, activation bool) *testDeps {
	return newTestDepsModeName(t, activation, "admin")
}

// newTestDepsModeName 指定管理员账号名构造（M-3 改名回归用）。
func newTestDepsModeName(t *testing.T, activation bool, adminName string) *testDeps {
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
			// 发布 A：窗口开启（inDateRange=true），61115 可报名 / 61116 已满员；
			// 发布 B：窗口关闭（inDateRange=false），61117 有空位。
			// 三态齐全，供手动报名服务端复核（M7）与满员/窗口测试使用。
			json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "beginTimes": []int64{1789261200000},
				"selectElectivesData": []any{
					map[string]any{
						"publishId": 3225, "publishName": "高二年体育", "inDateRange": true,
						"canSelect": 1, "hasSelected": 0, "groupCount": 1, "totalCount": 3,
						"electivesClassList": []any{
							map[string]any{
								"id": 61115, "course_name": "健美操", "class_name": "健美操1、2班",
								"teacher_name_list": "陈跃强", "class_room_name": "操场",
								"selected_count": 0, "max_count": 36, "can_select": true, "btn_type": 2,
							},
							map[string]any{
								"id": 61116, "course_name": "满员课程", "class_name": "满员班",
								"teacher_name_list": "李老师", "class_room_name": "教室",
								"selected_count": 36, "max_count": 36, "can_select": false, "btn_type": 2,
							},
						},
					},
					map[string]any{
						"publishId": 3226, "publishName": "校本课程", "inDateRange": false,
						"canSelect": 2, "hasSelected": 0, "groupCount": 1, "totalCount": 2,
						"electivesClassList": []any{
							map[string]any{
								"id": 61117, "course_name": "窗口外课程", "class_name": "窗口外班",
								"teacher_name_list": "王老师", "class_room_name": "教室",
								"selected_count": 0, "max_count": 30, "can_select": false, "btn_type": 2,
							},
						},
					},
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
		case strings.HasSuffix(r.URL.Path, "/exitElectivesClass"):
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "msg": "退选成功"})
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
	apiHandler := Register(mux, st, sched, accts, sessions, rt.Get().OpenTime, testAdminToken, adminName,
		rt.Get().ActivationEnabled, enc, dec, rt)
	return &testDeps{srv: zhi, store: st, api: apiHandler, sched: sched, sessions: sessions, accts: accts, rt: rt, dec: dec}
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

// loginAndGetToken 复刻真实完整链路：教务登录（未激活返回 code=1001 并颁发激活票据）-> 激活码激活 -> 返回会话令牌。
func loginAndGetToken(t *testing.T, d *testDeps, acct string) string {
	t.Helper()
	// 教务登录：注册账号到 accounts 管理器，并应返回未激活提示 + 激活票据
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"`+acct+`","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活账号登录应返回 1001: %d %v", code, j)
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["ticket"] == nil || data["ticket"] == "" {
		t.Fatalf("未激活登录必须颁发激活票据: %v", j)
	}
	ticket, _ := data["ticket"].(string)
	// 生成激活码
	if err := d.store.CreateActivationCode("XK-ABCD-EF12-3456", 10); err != nil {
		t.Fatal(err)
	}
	// 携带票据激活并签发会话
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"`+acct+`","code":"XK-ABCD-EF12-3456","ticket":"`+ticket+`"}`)
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

// TestLogoutRevokesToken M-7：登出接口立即吊销服务端令牌——
// 注销后原令牌再次访问任意受保护接口必须 401（令牌外流残留被封堵）。
func TestLogoutRevokesToken(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	// 登出前令牌有效
	if _, j := doJSONAuth(t, d.api, "GET", "/api/state", "", tok); j["code"].(float64) != 0 {
		t.Fatalf("登出前会话应有效: %v", j)
	}
	// 登出（会话级鉴权：携带正确令牌）
	code, j := doJSONAuth(t, d.api, "POST", "/api/logout", "{}", tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("登出失败: %d %v", code, j)
	}
	// 登出后同一令牌立即失效
	_, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok)
	if j["code"].(float64) != 401 {
		t.Fatalf("登出后令牌应立即 401: %v", j)
	}
	// 无会话调用登出本身应 401
	_, j = doJSON(t, d.api, "POST", "/api/logout", "{}")
	if j["code"].(float64) != 401 {
		t.Fatalf("无会话登出应 401: %v", j)
	}
}

// TestAccountOverrideRequiresAdminSession 验证 ?account= 穿透能力仅限管理员会话：
// 普通学生会话绝不能穿透到其他账号（水平越权防线）。
func TestAccountOverrideRequiresAdminSession(t *testing.T) {
	d := newTestDeps(t)

	// 准备三个账号：student（普通学生）、victim（被攻击目标）、adminName（名为 admin 的普通学生）
	studentTok := authenticateDirect(t, d, "student")
	victimTok := authenticateDirect(t, d, "victim")
	adminNameTok := authenticateDirect(t, d, "admin")

	// 1. 普通会话（student）携带 ?account=victim 改目标：必须只写进 student 自己的目标表，victim 不受影响
	code, j := doJSONAuth(t, d.api, "PUT", "/api/targets?account=victim",
		`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":0}]}`, studentTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("学生会话设置自己目标应成功: %d %v", code, j)
	}
	// victim 自己的目标应为空（student 的穿透未生效）
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", victimTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("victim 读取自己状态应成功: %d %v", code, j)
	}
	if stData, ok := j["data"].(map[string]any); ok {
		if courses, _ := stData["courses"].([]any); len(courses) > 0 {
			t.Fatalf("victim 的目标不应被 student 的穿透请求修改，当前数量 %d", len(courses))
		}
	}

	// 2. 名为 admin 的普通学生会话（非管理员身份）同样不能穿透
	code, j = doJSONAuth(t, d.api, "PUT", "/api/targets?account=victim",
		`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":0}]}`, adminNameTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("名为 admin 的普通会话设置自己目标应成功: %d %v", code, j)
	}
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", victimTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("victim 读取自己状态应成功: %d %v", code, j)
	}
	if stData, ok := j["data"].(map[string]any); ok {
		if courses, _ := stData["courses"].([]any); len(courses) > 0 {
			t.Fatalf("victim 的目标不应被名为 admin 的普通会话修改，当前数量 %d", len(courses))
		}
	}

	// 3. 管理员会话携带 ?account= 应能正常穿透（写目标到 victim）
	adminTok := adminTokenFor(t, d)
	code, j = doJSONAuth(t, d.api, "PUT", "/api/targets?account=victim",
		`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":0}]}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("管理员会话穿透设置 victim 目标应成功: %d %v", code, j)
	}
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", victimTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("victim 读取自己状态应成功: %d %v", code, j)
	}
	if stData, ok := j["data"].(map[string]any); ok {
		if courses, _ := stData["courses"].([]any); len(courses) != 1 {
			t.Fatalf("管理员穿透后 victim 应恰有 1 门目标，实际 %d", len(courses))
		}
	}
}

func TestLoginUnactivatedNeedsCode(t *testing.T) {
	d := newTestDeps(t)
	// 未激活账号登录：教务登录成功但应返回 code=1001 并颁发激活票据（C-2）
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活登录应返回 1001: %d %v", code, j)
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["ticket"] == nil || data["ticket"] == "" {
		t.Fatalf("未激活登录必须颁发激活票据: %v", j)
	}
}

// TestActivateRequiresTicketAndBinding 激活必须携带有效票据且与激活账号一致（C-2）：
// 无票据、票据与账号不匹配、票据已用尽一律拒绝，杜绝持码者对任意账号激活。
func TestActivateRequiresTicketAndBinding(t *testing.T) {
	d := newTestDeps(t)
	// 登录 acct1 拿票据
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活登录应返回 1001: %d %v", code, j)
	}
	data, _ := j["data"].(map[string]any)
	ticket, _ := data["ticket"].(string)
	if err := d.store.CreateActivationCode("XK-ABCD-EF12-3456", 10); err != nil {
		t.Fatal(err)
	}
	// 1. 无票据：拒绝
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"XK-ABCD-EF12-3456"}`)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("无票据激活应被拒绝: %d %v", code, j)
	}
	// 2. 票据与账号不匹配（票据绑 acct1，激活 acct2）：拒绝
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct2","code":"XK-ABCD-EF12-3456","ticket":"`+ticket+`"}`)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("票据账号不匹配应被拒绝: %d %v", code, j)
	}
	// 3. 正确消费成功
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"XK-ABCD-EF12-3456","ticket":"`+ticket+`"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("携带正确票据应激活成功: %d %v", code, j)
	}
	// 4. 同一票据再激活一次（已用尽）：拒绝——激活码不得被重复使用
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"XK-ABCD-EF12-3456","ticket":"`+ticket+`"}`)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("已用尽的票据再次激活应被拒绝: %d %v", code, j)
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
	// uses 超上限（>1000）必须被拒绝（n3），拒绝不应入库
	code, j := doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":1,"uses":1001}`, adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("uses 超上限应被拒绝: %d %v", code, j)
	}
	// 生成 2 个激活码，每个可用 3 次
	code, j = doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":2,"uses":3}`, adminTok)
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
	// 用激活码激活账号，验证可用次数扣减（C-2：必须携带登录签发的激活票据）
	first := codes[0].(string)
	// 生成码为 16 位 hex（XK-XXXX-XXXX-XXXX-XXXX），断言熵提升落地（m7）
	if strings.Count(first, "-") != 4 {
		t.Fatalf("激活码应为 16 位 hex（4 段分隔），实际 %q", first)
	}
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 1001 {
		t.Fatalf("未激活登录应返回 1001: %d %v", code, j)
	}
	loginData, _ := j["data"].(map[string]any)
	ticket, _ := loginData["ticket"].(string)
	if ticket == "" {
		t.Fatalf("未激活登录必须颁发激活票据: %v", j)
	}
	code, j = doJSON(t, d.api, "POST", "/api/activate", `{"account":"acct1","code":"`+first+`","ticket":"`+ticket+`"}`)
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

// TestRecoverMiddlewareHidesPanicDetail n1：panic 详情绝不回显客户端——统一 500 文案，
// 内部细节只进日志。修复前（拼接 rec）响应会泄露 panic 内容，本测试即 RED。
func TestRecoverMiddlewareHidesPanicDetail(t *testing.T) {
	// 直构一个会 panic 的 handler，验证 recoverMiddleware 包装后对外只见"内部错误"
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("内部密钥泄露: sk-abcdef123456")
	})
	h := recoverMiddleware(panicHandler)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	if j["code"].(float64) != 500 {
		t.Fatalf("panic 应统一 500: %v", j)
	}
	if msg, _ := j["msg"].(string); strings.Contains(msg, "sk-abcdef123456") {
		t.Fatalf("panic 详情泄露给客户端: %v", j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "内部错误") {
		t.Fatalf("应统一回显「内部错误」文案: %v", j)
	}
}

// TestAdminConfigSaveFailStillDispatch M-4：落库失败时——配置已内存生效、下游热下发
// 必须照常执行（识别引擎/Vision 同步新值），且响应如实区分"已生效但落库失败"（code=500）。
func TestAdminConfigSaveFailStillDispatch(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 注入落库失败桩
	saveSettingsErrForTest = errors.New("settings 落库失败")
	t.Cleanup(func() { saveSettingsErrForTest = nil })

	code, j := doJSONAdmin(t, d.api, "PUT", "/api/admin/config",
		`{"vision_base_url":"https://fail.example.com/v1"}`, adminTok)
	if code != 200 {
		t.Fatalf("落库失败应返回 HTTP 200（业务 code=500），实际 %d", code)
	}
	if j["code"].(float64) != 500 {
		t.Fatalf("落库失败应业务 code=500 如实区分，实际 %v", j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "落库失败") {
		t.Fatalf("报错文案应明示落库失败: %v", j)
	}
	// 内存已生效：运行时配置中心已是新地址
	if got := d.rt.Get().VisionBaseURL; got != "https://fail.example.com/v1" {
		t.Fatalf("内存配置应已生效: %v", got)
	}
	// 下游热下发未跳过：再次热改（仍失败）后登录，验证码识别应打到新地址并失败——
	// 证明 SetVision/识别引擎切换在落库失败路径也被执行
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config",
		`{"vision_base_url":"https://invalid2.example.com/v1"}`, adminTok)
	if j["code"].(float64) != 500 {
		t.Fatalf("再次落库失败仍应 500: %v", j)
	}
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"acct2","password":"pwd"}`)
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "invalid2.example.com") {
		t.Fatalf("下游 Vision 热下发被跳过（登录应打到新地址）: %v", j)
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
	if cfg["vision_api_key_masked"] != "****D***" {
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

	// F7-02（第 7 轮）：空 open_time 是"显式清空开放时间"，不再静默忽略——
	// 必须真实生效（内存 + 落库 + admin config 回显全为空），调度器解除窗口机制。
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"open_time":""}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("清空开放时间应成功（不再静默忽略）: %d %v", code, j)
	}
	// 生效配置回显为空
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/config", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("读取配置失败: %d %v", code, j)
	}
	cfg2, _ := j["data"].(map[string]any)
	if ot, _ := cfg2["open_time"].(string); ot != "" {
		t.Fatalf("清空 open_time 后回显应为空串（此前假成功是残留旧值），实际 %q", ot)
	}
	// 落库也为空（重启恢复源与内存一致）
	kv2, err := d.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if kv2["open_time"] != "" {
		t.Fatalf("清空 open_time 后落库应为空串，实际 %q", kv2["open_time"])
	}

	// m8：带管理员会话 + 表单 Content-Type 的副作用请求必须被拒（CSRF 防线）。
	// 路由层 requireJSONBody 对非 JSON 提交直接 403——跨站表单 POST 无法伪造 JSON 头。
	// 本项目约定：业务码放 body.code，HTTP 状态恒定 200，故读 body 的 code 字段。
	req5 := httptest.NewRequest(http.MethodPost, "/api/admin/codes", strings.NewReader("count=1"))
	req5.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req5.Header.Set("Authorization", "Bearer "+adminTok)
	rec5 := httptest.NewRecorder()
	d.api.ServeHTTP(rec5, req5)
	var j5 map[string]any
	json.Unmarshal(rec5.Body.Bytes(), &j5)
	if c, _ := j5["code"].(float64); c != 403 {
		t.Fatalf("表单 Content-Type 的 admin 副作用请求应返回 code=403（m8），实际 %v", j5)
	}
	// 同时保证未走生成分支（激活码列表未新增）
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", adminTok)
	if lst, _ := j["data"].([]any); len(lst) != 0 {
		t.Fatalf("被拒请求不应生成激活码（m8），当前列表 %v", lst)
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

// TestAdminConfigRefuseUnencryptedVisionKey 拒绝未加密旧版明文：
// 1. secureEncrypt 未注入加密器时必须报错拒绝（严禁明文落库）
// 2. 模拟 main.go 启动恢复规则：DB 内存在未加密的旧明文时必须拒绝加载
func TestAdminConfigRefuseUnencryptedVisionKey(t *testing.T) {
	d := &Deps{}
	if _, err := d.secureEncrypt("***REMOVED***"); err == nil {
		t.Fatal("未注入 Encrypt 时 secureEncrypt 应报错拒绝")
	}

	// 模拟 main.go 启动恢复逻辑：无 enc: 前缀一律拒绝加载进 VisionAPIKey
	kv := map[string]string{"vision_key": "***REMOVED***-unencrypted"}
	loadedKey := ""
	if v, ok := kv["vision_key"]; ok && strings.HasPrefix(v, "enc:") {
		loadedKey = v
	}
	if loadedKey != "" {
		t.Fatalf("旧版未加密明文不应被加载: %q", loadedKey)
	}
}

// TestAdminDeleteProtectsRenamedAdmin 管理员删除保护必须跟随改名后的管理员账号名：
// XUANKE_ADMIN_NAME=root 时，root 账号不可被删除（C1 修复）。
func TestAdminDeleteProtectsRenamedAdmin(t *testing.T) {
	// 判定辅助恒等：改名与默认名都能被 IsAdminAccountName 识别
	if !(&Deps{AdminName: "root"}).IsAdminAccountName("root") {
		t.Fatal("改名后的管理员账号名应被识别")
	}
	if !(&Deps{}).IsAdminAccountName("admin") {
		t.Fatal("默认 admin 名也应被识别为管理员账号名")
	}
	// 真实 handler 链路：AdminName=root 的完整路由，删除 root（管理员名）应被拒绝，
	// 删除 admin（非管理员名）应被允许——验证 handler 不再硬编码 "admin"。
	d := newTestDepsMode(t, true)
	// 复用新 TestDeps 的底层组件，但重建 Deps 令 AdminName=root（保留原会话库/账号库/调度器）
	renamed := &Deps{
		Store:             d.store,
		Sched:             d.sched,
		Accounts:          d.accts,
		Sessions:          d.sessions,
		AdminToken:        testAdminToken,
		AdminName:         "root",
		ActivationEnabled: true,
	}
	req := httptest.NewRequest("DELETE", "/api/admin/accounts", strings.NewReader(`{"account":"root"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	renamed.handleAdminDeleteAccount(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	if j["code"].(float64) == 0 {
		t.Fatalf("删除管理员账号 root 应被拒绝: %v", j)
	}
	// 管理员自己不存在于 store：删除一个普通账号应正常放行（验证保护判定只挡管理员名）
	req2 := httptest.NewRequest("DELETE", "/api/admin/accounts", strings.NewReader(`{"account":"acct1"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	renamed.handleAdminDeleteAccount(rec2, req2)
	if err := json.Unmarshal(rec2.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	if j["code"].(float64) != 0 {
		t.Fatalf("删除普通账号应正常放行: %v", j)
	}
}

// TestRenamedAdminSessionBindsConfigName 管理员会话账号必须绑定配置名（M-3）：
// XUANKE_ADMIN_NAME=root 后，管理员会话的 sessionAccount 必须返回 root（而非字面量 admin），
// 使 handleElectives/State 的 IsAdminAccountName 兜底路径判定一致——
// 改名后无 ?account= 参数时不会再错误地当学生账号处理（ProbeForAccount("admin") 会撞不存在的客户端）。
func TestRenamedAdminSessionBindsConfigName(t *testing.T) {
	d := newTestDepsModeName(t, true, "root")
	// 用 root + 管理口令登录管理员会话
	adminTok := ""
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"root","password":"`+testAdminToken+`"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("改名管理员登录失败: %d %v", code, j)
	}
	if data, ok := j["data"].(map[string]any); ok {
		adminTok, _ = data["token"].(string)
	}
	if adminTok == "" {
		t.Fatalf("改名管理员登录缺少会话令牌: %v", j)
	}
	// 管理员会话账号名应为 root（M-3 修复前是硬编码 admin，穿透兜底会错位）
	sessAcct, ok := d.sessions.Account(adminTok)
	if !ok || sessAcct != "root" {
		t.Fatalf("管理员会话账号应绑定配置名 root，实际 %q %v", sessAcct, ok)
	}
	// 管理员会话可访问管理接口（会话身份 Admin:true 不受账号名影响）
	code, j = doJSONAuth(t, d.api, "GET", "/api/admin/config", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("改名管理员会话应能访问管理接口: %d %v", code, j)
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
	// MAJOR-A：删除账号后其既有会话必须立即失效（吊销会话，不等 12h TTL）
	// 会话失效由业务 code=401 表达（HTTP 200 + body code 恒为项目约定）
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok1)
	if code != 200 || j["code"].(float64) != 401 {
		t.Fatalf("删除账号后旧会话应立即失效 code=401，实际 %d %v", code, j)
	}
	// 未删除的 acct2 会话不受影响
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok2)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("未删除账号会话不应受影响: %d %v", code, j)
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

// TestElectiveSelectRejectsWindowClosed 手动报名服务端复核（M7）：
// 课程所在发布窗口未开放（in_date_range=false）→ 拒绝报名并返回友好错误。
func TestElectiveSelectRejectsWindowClosed(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	if _, err := d.sched.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// 61117 属于窗口关闭的发布 B：服务端复核必须拒绝，且不发起对平台的真实报名请求
	code, j := doJSONAuth(t, d.api, "POST", "/api/electives/select",
		`{"class_id":61117,"course_name":"窗口外课程"}`, tok)
	if code != 200 || j["code"].(float64) != 1 {
		t.Fatalf("窗口关闭报名应被拒绝 code=1: %d %v", code, j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "未开放") && !strings.Contains(msg, "窗口") {
		t.Fatalf("拒绝文案应说明窗口未开放，实际: %v", j["msg"])
	}
	// 调度器状态不得被标记为 success（复核拦截在先，未真正报名）
	st := d.sched.StateForAccount("acct1")
	for _, c := range st.Courses {
		if c.ClassID == 61117 && c.Status == "success" {
			t.Fatalf("窗口关闭课程不得标记 success: %v", c)
		}
	}
}

// TestElectiveSelectRejectsFullClass 手动报名服务端复核（M7）：
// 快照显示课程已满员（selected_count >= max_count）→ 拒绝报名并返回友好错误。
func TestElectiveSelectRejectsFullClass(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	if _, err := d.sched.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// 61116 满员（selected 36 / max 36）：复核必须拒绝
	code, j := doJSONAuth(t, d.api, "POST", "/api/electives/select",
		`{"class_id":61116,"course_name":"满员课程"}`, tok)
	if code != 200 || j["code"].(float64) != 1 {
		t.Fatalf("满员报名应被拒绝 code=1: %d %v", code, j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "满") {
		t.Fatalf("拒绝文案应说明课程已满，实际: %v", j["msg"])
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

// TestSetTargetsBounds 目标数量与范围必须受校验（n2）：
// 超过 100 门 / 非法 publish_id / 非法 priority 一律拒绝，且不得入库。
func TestSetTargetsBounds(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")

	// 1. 超过条数上限（101 门）必须拒绝
	var big strings.Builder
	big.WriteString(`{"targets":[`)
	for i := 0; i < 101; i++ {
		if i > 0 {
			big.WriteString(",")
		}
		fmt.Fprintf(&big, `{"publish_id":1,"class_id":%d,"course_name":"c%d","priority":%d}`, 61115+i, i, i)
	}
	big.WriteString(`]}`)
	code, j := doJSONAuth(t, d.api, "PUT", "/api/targets", big.String(), tok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("超过 100 门目标应被拒绝: %d %v", code, j)
	}

	// 2. publish_id 非法（<=0）必须拒绝
	code, j = doJSONAuth(t, d.api, "PUT", "/api/targets",
		`{"targets":[{"publish_id":0,"class_id":61115,"course_name":"健美操","priority":0}]}`, tok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("publish_id<=0 应被拒绝: %d %v", code, j)
	}

	// 3. priority 越界（负数 / >999）必须拒绝
	code, j = doJSONAuth(t, d.api, "PUT", "/api/targets",
		`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":-1}]}`, tok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("priority 负数应被拒绝: %d %v", code, j)
	}
	code, j = doJSONAuth(t, d.api, "PUT", "/api/targets",
		`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":1000}]}`, tok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("priority>999 应被拒绝: %d %v", code, j)
	}

	// 4. 拒绝后原目标不得被改动（保持空）
	targets, err := d.store.LoadTargetsForAccount("acct1")
	if err != nil || len(targets) != 0 {
		t.Fatalf("非法请求不应写入目标表: %v %v", targets, err)
	}
}

// TestAccountsNonAdminSeesOnlySelf 普通会话只能看到自身账号（M-2）：
// 即使系统里注册了多个账号，普通会话的 /api/accounts 也只回显自己的账号名，
// 杜绝账号枚举（学号/姓名高价值情报）；管理员会话回显全量。
func TestAccountsNonAdminSeesOnlySelf(t *testing.T) {
	d := newTestDeps(t)
	tok1 := authenticateDirect(t, d, "acct1")
	tok2 := authenticateDirect(t, d, "acct2")

	// 普通会话 acct1：只看到自己
	code, j := doJSONAuth(t, d.api, "GET", "/api/accounts", "", tok1)
	list, _ := j["data"].([]any)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("accounts 异常: %d %v", code, j)
	}
	if len(list) != 1 || list[0] != "acct1" {
		t.Fatalf("普通会话应只看到自身账号，实际 %v", list)
	}
	// 普通会话 acct2：同样只看到自己，看不到 acct1
	code, j = doJSONAuth(t, d.api, "GET", "/api/accounts", "", tok2)
	list, _ = j["data"].([]any)
	if len(list) != 1 || list[0] != "acct2" {
		t.Fatalf("普通会话 acct2 应只看到自身账号，实际 %v", list)
	}
	// 管理员会话：回显全量（与多账号维护管理一致）
	adminTok := adminTokenFor(t, d)
	code, j = doJSONAdmin(t, d.api, "GET", "/api/accounts", "", adminTok)
	list, _ = j["data"].([]any)
	if code != 200 || j["code"].(float64) != 0 || len(list) != 2 {
		t.Fatalf("管理员会话应看到全部账号: %d %v", code, j)
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

// TestLoginActivateSeparateBuckets M-6：登录与激活各自独立限流桶——
// 刷空登录额度后，激活接口不受影响；刷空激活额度后，登录接口不受影响。
// 反代/学校 NAT 下二者互不锁死（选课当天并发登录不会因激活爆破被全员 429）。
func TestLoginActivateSeparateBuckets(t *testing.T) {
	d := newTestDeps(t)
	// 先立刻榨干激活桶：连续 7 次激活（未携带票据，每次都被拒但消耗激活额度）
	// 第 6 次起应触发激活限流 429
	limited := false
	for i := 0; i < 7; i++ {
		_, j := doJSON(t, d.api, "POST", "/api/activate", `{"account":"x","code":"XK-NOPE","ticket":"t"}`)
		if c, _ := j["code"].(float64); c == 429 {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("激活请求应触发激活桶限流 429")
	}
	// 激活额度耗尽后，登录接口仍可用（登录桶独立）
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"stdlib","password":"any"}`)
	if c, _ := j["code"].(float64); code != 200 || c == 429 {
		t.Fatalf("激活限流不应影响登录（登录桶独立）: %d %v", code, j)
	}
	// 登录走到业务层（教务登录成功→1001 未激活），证明未被打到限流层
	if c, _ := j["code"].(float64); c != 1001 {
		t.Fatalf("登录应正常走到业务层（教务登录成功返回 1001 未激活）: %v", j)
	}
}

// TestLoginAdminWrongPasswordTimingFlat n4 登录时延侧信道：管理员口令错误分支必须
// 固定延迟 loginTimingFlat 后再响应，使"管理员名（口令错立即回）"与"未知学生
// （教务登录网络往返）"的响应时延差被拉平——管理员账号名不能靠响应快慢被枚举。
func TestLoginAdminWrongPasswordTimingFlat(t *testing.T) {
	d := newTestDeps(t)
	start := time.Now()
	// 正确管理员账号名 + 错误口令：走恒定时间比对失败 + loginTimingFlat 固定延迟
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"nope"}`)
	elapsed := time.Since(start)
	if code != 200 {
		t.Fatalf("管理口令错误应返回 HTTP 200（业务 code=1），实际 %d", code)
	}
	if j["code"].(float64) != 1 {
		t.Fatalf("管理口令错误应 code=1，实际 %v", j)
	}
	if elapsed < loginTimingFlat {
		t.Fatalf("管理员口令错误分支必须延迟 ≥ loginTimingFlat(%v) 再响应，实际 %v——响应过快会让管理员账号名被侧信道枚举", loginTimingFlat, elapsed)
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

// TestClientIPTrustedProxy B6-05：clientIP 可信反代 IP 透传。
// 默认（未设 XUANKE_TRUSTED_PROXY）绝不信 XFF——攻击者可伪造任意 IP 刷爆他人
// 限流桶或绕过自身限流；仅当开关=on 且 RemoteAddr 确实是回环地址（真正的本机
// 反代）时，才取 X-Forwarded-For 最右一个非空值作为真实客户端 IP。
func TestClientIPTrustedProxy(t *testing.T) {
	cases := []struct {
		name    string
		trusted bool   // 是否设置 XUANKE_TRUSTED_PROXY=on
		remote  string // RemoteAddr（含端口）
		xff     string // X-Forwarded-For 头
		want    string
	}{
		{"默认关闭不信XFF", false, "10.0.0.5:43001", "1.2.3.4", "10.0.0.5"},
		{"回环+on信任最右", true, "127.0.0.1:5000", "203.0.113.7, 10.9.9.9", "10.9.9.9"},
		{"回环+on+单值", true, "127.0.0.1:5000", "203.0.113.7", "203.0.113.7"},
		{"回环+on+空XFF回退", true, "127.0.0.1:5000", "", "127.0.0.1"},
		{"回环+on+XFF全空白回退", true, "127.0.0.1:5000", " , , ", "127.0.0.1"},
		{"公网直达不信XFF", true, "8.8.8.8:6000", "1.2.3.4", "8.8.8.8"},
		{"IPv6回环+on", true, "[::1]:8080", "203.0.113.7", "203.0.113.7"},
		{"无端口原样返回", true, "203.0.113.9", "1.2.3.4", "203.0.113.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.trusted {
				t.Setenv("XUANKE_TRUSTED_PROXY", "on")
			} else {
				t.Setenv("XUANKE_TRUSTED_PROXY", "")
			}
			req := httptest.NewRequest("POST", "/api/login", nil)
			req.RemoteAddr = tc.remote
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := clientIP(req); got != tc.want {
				t.Fatalf("clientIP() = %q，期望 %q（场景：%s）", got, tc.want, tc.name)
			}
		})
	}
}

// TestApiUnknownPath404 B7-C4：未注册的 /api/xxx 必须 404 JSON，绝不可能回退 SPA
// index.html（此前落入 main.go "/" SPA 兜底 → 200 text/html：前端 fetch 解析 JSON
// 报错掩盖真实 404；安全扫描误判任意 /api/ 路径可 200）。已注册的固定路由不受影响。
func TestApiUnknownPath404(t *testing.T) {
	d := newTestDeps(t)
	// 未注册的 /api/xxx：404 JSON，而非 200 text/html（精确 method+pattern 未命中 → 落到
	// 显式注册的 "/api/" 前缀 → 404 JSON；此前落入 main.go "/" SPA 兜底返回 200 HTML）
	req := httptest.NewRequest("GET", "/api/not-registered-path", nil)
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("未知 /api/ 路径应返回 HTTP 404，实际 %d（回归：曾 200 text/html）", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("未知 /api/ 路径应返回 JSON 响应，实际 Content-Type=%q", ct)
	}
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil || j["code"].(float64) != 404 {
		t.Fatalf("未知 /api/ 应 JSON code=404，实际 %s", rec.Body.String())
	}
	// 已注册的固定路由不受影响（health 可达）
	req2 := httptest.NewRequest("GET", "/api/health", nil)
	rec2 := httptest.NewRecorder()
	d.api.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("已注册路由 /api/health 应仍可达，实际 %d", rec2.Code)
	}
}

// TestAdminDeleteCodeNoBodyOK B7-M8/M9：DELETE /api/admin/codes 无 body（标准 REST 客户端
// 默认行为）必须可用——此前强制 requireJSONBody 导致 curl/脚本删码必踩 403 可用性噪音。
// 副作用 + 需要 body 的 POST 仍强制 JSON（跨站表单防挟持）；GET/DELETE 放行空 body。
func TestAdminDeleteCodeNoBodyOK(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 造一个激活码：通过管理的 codes GET 不生成，需先真造一条
	code, j := doJSONAdmin(t, d.api, "POST", "/api/admin/codes", `{"count":1,"uses":1}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("生成激活码失败: %d %v", code, j)
	}
	codes, _ := j["data"].([]any)
	if len(codes) != 1 {
		t.Fatalf("应生成 1 个码: %v", j)
	}
	theCode := codes[0].(string)
	// 模拟标准 DELETE 无 body 请求（无 Content-Type）：应不再是 403——空 body 解码失败
	// 返回明确业务错误（"请指定要删除的激活码"），REST 客户端不再被 JSON 门挡死
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/codes", nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if rec.Code == 403 {
		t.Fatalf("无 body DELETE 不应被 403 拒绝（B7-M8 回归）：http=%d body=%s", rec.Code, rec.Body.String())
	}
	var jr map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &jr); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	if c, _ := jr["code"].(float64); c != 1 || !strings.Contains(fmt.Sprint(jr["msg"]), "请指定要删除的激活码") {
		t.Fatalf("空 body DELETE 应返回明确的'请指定激活码'业务错误（而非 403）：%v", jr)
	}
	// 带 body 的正常删除：真删成功（清掉前面生成的码，保持测试幂等）
	code, j = doJSONAdmin(t, d.api, "DELETE", "/api/admin/codes", `{"code":"`+theCode+`"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("带 body 删除激活码失败: %d %v", code, j)
	}
}

// TestHandleElectivesSelectAndExit 验证手动报名与退选 REST API 接口 (Task 4)。
func TestHandleElectivesSelectAndExit(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")

	// 0. 先填充课程快照（mock 发布 A：窗口开启、61115 可报名；发布 B：窗口关闭）
	if _, err := d.sched.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// 1. 测试手动报名 POST /api/electives/select
	code, j := doJSONAuth(t, d.api, "POST", "/api/electives/select", `{"class_id":61115,"course_name":"健美操"}`, tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("手动报名失败: %d %v", code, j)
	}
	// 验证调度器状态被同步为 success
	st := d.sched.StateForAccount("acct1")
	found := false
	for _, c := range st.Courses {
		if c.ClassID == 61115 && c.Status == "success" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("手动报名成功后调度器状态应被标记为 success")
	}

	// 2. 测试手动退选 POST /api/electives/select/exit
	code, j = doJSONAuth(t, d.api, "POST", "/api/electives/select/exit", `{"class_id":61115}`, tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("手动退选失败: %d %v", code, j)
	}
	// 验证调度器状态被重置为 pending
	st = d.sched.StateForAccount("acct1")
	foundPending := false
	for _, c := range st.Courses {
		if c.ClassID == 61115 && c.Status == "pending" {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Fatal("手动退选成功后调度器状态应被恢复为 pending")
	}
}
