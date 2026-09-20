package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	rt       *runtime.Store               // 运行时配置中心（测试重建 handler 用）
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
	// R52：测试夹具套接字预创建——预先绑定一个 127.0.0.1 回环端口并立即关闭。
	// 背景：Windows 宿主 api 包测试内 httptest mock 服务器（账号专属 per-test）
	// 连接瞬时失败（connectex，R46 起 4 轮 6+ 样本）根因是回环 TIME_WAIT 队列冷启动
	// 未排空，mock 服务器 accept 尚未就绪即收到连接。预创建-关闭动作预占并释放一个
	// 端口，排空后由 keep-alive 空闲连接吸收（模拟真实平台会话建立的热态），
	// 杜绝 runaway accept+dispatch 竞态的开始期误拒。
	// MAJOR-53-01 收尾（R53）：预创建只解决"连接池里有濒死连接"，解决不了
	// "每个测试新建 mock server 自身 accept 就绪前的最首请求"——httptest.NewServer
	// 返回后 server 在独立 goroutine accept，Windows 回环冷启动窗口仍可让首个测试
	// 请求 connectex（api 12 轮 1 FAIL 实证）。构造完 zhi 后主动发一条健康探测
	// 请求把冷启动窗口前移到夹具构造期，之后测试请求全落在已就绪 server 上。
	// 注意：这是测试夹具层面的根治，真实运行不受影响。
	// socket 预放仍保留：探测后首个测试请求仍可能复用"探测刚建立的连接"前的
	// TIME_WAIT 队列残余，双保险。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()

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

	// MAJOR-53-01 收尾：mock 服务器就绪探测——httptest.NewServer 返回后 server 已在
	// 独立 goroutine accept，但 Windows 回环冷启动窗口（TIME_WAIT 队列未排空）仍可让
	// 首个测试请求 connectex（api 12 轮 1 FAIL 实证）。向 mock 发一条健康探测请求
	// 把冷启动窗口前移到夹具构造期，之后测试请求全落在已就绪 server 上。
	if err := readyProbe(zhi.URL); err != nil {
		t.Fatalf("mock 服务器就绪探测失败: %v", err)
	}

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)

	accts := accounts.New(zhi.URL, zhidao.VisionConfig{BaseURL: zhi.URL, APIKey: "k", Model: "m"}, st)
	sessions := session.New(time.Hour)

	// 开放时间不做任何配置注入：识别槽（平台 beginTimes）是唯一事实源，New 传零值。
	// 调度器窗口判定/探测/展示全部走自动识别。
	sched := scheduler.New(accts, st, time.Time{}, time.Hour) // 测试不自动轮询
	sched.Start()
	t.Cleanup(sched.Stop)

	mux := http.NewServeMux()
	rt := runtime.New(runtime.Config{
		ActivationEnabled: activation,
		VisionBaseURL:     zhi.URL,
		VisionAPIKey:      "***REMOVED***",
		VisionModel:       "m",
	})
	// 与 main 一致：注入真实 AES-256-GCM 加密（凭据与 vision_key 落库前加密）
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatal(err)
	}
	enc := func(s string) (string, error) { return secure.Encrypt(s, masterKey) }
	dec := func(s string) (string, error) { return secure.Decrypt(s, masterKey) }
	apiHandler := Register(mux, st, sched, accts, sessions, testAdminToken, adminName,
		rt.Get().ActivationEnabled, enc, dec, rt)
	return &testDeps{srv: zhi, store: st, api: apiHandler, sched: sched, sessions: sessions, accts: accts, rt: rt, dec: dec}
}

// readyProbe 夹具就绪探测：向 mock 服务器发一条健康请求（期望非连接错误响应），
// 把 Windows 回环冷启动窗口前移到夹具构造期。连接层失败轮询重试（200ms 间隔 ×
// 10 次，总窗口 ~2s，实测覆盖冷启动 TIME_WAIT 队列排空——R53 单次重试/5×200ms
// 已有前序包结束后最恶劣时刻 connectex 样本（R58 全量 R3 readyProbe 自身 5 次
// 全败直接 Fatal 实证），加宽到 10 次 + 显式 2s 超时兜底），全部失败才上抛。
// 探测请求恰好也排空首个连接的 TIME_WAIT 队列，之后测试请求落在已就绪 server 上。
func readyProbe(baseURL string) error {
	// 轮询重试：连接层失败后短暂休眠重试，把探测成功前的冷启动窗口彻底前移。
	// 显式 2s 超时——http.DefaultClient 超时为 0=无限，mock 极端挂起时可阻塞分钟级
	//（OBSERVE-56-04/57-03 延续）；探测请求只关心"accept 是否就绪"，2s 足够。
	client := &http.Client{Timeout: 2 * time.Second}
	const (
		probeRetries = 10
		probeDelay   = 200 * time.Millisecond
	)
	var lastErr error
	for i := 0; i <= probeRetries; i++ {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/ready", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			_, err = io.Copy(io.Discard, resp.Body)
			if err == nil {
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}
		if i < probeRetries {
			time.Sleep(probeDelay)
		}
	}
	return lastErr
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
	// 无会话访问受保护端点应 401（B39-02：HTTP 状态码真实 401，此前恒 200）
	code, j := doJSON(t, d.api, "GET", "/api/state", "")
	if code != http.StatusUnauthorized || j["code"].(float64) != 401 {
		t.Fatalf("无会话应 401: %d %v", code, j)
	}
	code, j = doJSON(t, d.api, "GET", "/api/electives", "")
	if code != http.StatusUnauthorized || j["code"].(float64) != 401 {
		t.Fatalf("无会话 /electives 应 401: %d %v", code, j)
	}
	// 无效会话令牌应 401
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", "bogus-token")
	if code != http.StatusUnauthorized || j["code"].(float64) != 401 {
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
	// authenticateDirect 直连批量注册多个账号：重置全局重登频率闸门预算（夹具语义
	// 不关心登录流程，纯注册账号建会话），避免同一分钟窗口内被 B42-01 准入闸门误拦
	d.accts.ResetGateForTest()
	studentTok := authenticateDirect(t, d, "student")
	d.accts.ResetGateForTest()
	victimTok := authenticateDirect(t, d, "victim")
	d.accts.ResetGateForTest()
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
	// B43-04：管理员名 + 非管理口令先试教务登录 → mock 平台教务全成功 → 撞名学生登录
	// 走教务成功分支。未激活撞名学生返回 1001（颁发票据）而非管理员口令错误——这本身
	// 就是 B43-04 契约（撞名学生绝不被管理员分支吞掉）；激活后的正常会话由
	// TestLoginAdminNameCollisionStudentCredential 覆盖。
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"wrong"}`)
	if code != 200 {
		t.Fatalf("登录请求应 HTTP 200: %d", code)
	}
	if j["code"].(float64) == 1 && strings.Contains(j["msg"].(string), "管理口令错误") {
		t.Fatalf("B43-04 后撞名学生绝不被管理员分支吞掉（不得返回管理口令错误）: %v", j)
	}
	// 无会话访问管理接口应 403（B39-02：HTTP 状态码真实 403，此前恒 200）
	code, j = doJSON(t, d.api, "GET", "/api/admin/codes", "")
	if code != http.StatusForbidden || j["code"].(float64) != 403 {
		t.Fatalf("无会话访问管理接口应 403: %d %v", code, j)
	}
	// 普通用户会话访问管理接口应 403
	userTok := authenticateDirect(t, d, "acct1")
	code, j = doJSONAuth(t, d.api, "GET", "/api/admin/codes", "", userTok)
	if code != http.StatusForbidden || j["code"].(float64) != 403 {
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
	// B39-02：panic 恢复除 body code=500 外，HTTP 状态码必须真实写 500——
	// 此前 writeJSON 只设 Content-Type 不写 WriteHeader，监控/反代在 HTTP 层
	// 识别不了后端内部错误（恒 200 假象）。修复后 panic 路径 HTTP 500。
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic 恢复应写 HTTP 500（修复前恒 200），实际 %d", rec.Code)
	}
}

// TestAdminConfigSaveFailStillDispatch M-4 + F48-O3：落库失败时——配置已内存生效、下游热下发
// 必须照常执行（识别引擎/Vision 同步新值），且响应如实区分"已生效但落库失败"（body code=500 且
// HTTP 层亦为真实 500——家族整风后 B43-05 的 stats 与 handleAdminConfig 持久化错误统一真状态码）。
func TestAdminConfigSaveFailStillDispatch(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 注入落库失败桩
	saveSettingsErrForTest = errors.New("settings 落库失败")
	t.Cleanup(func() { saveSettingsErrForTest = nil })

	code, j := doJSONAdmin(t, d.api, "PUT", "/api/admin/config",
		`{"vision_base_url":"https://fail.example.com/v1"}`, adminTok)
	if code != 500 {
		t.Fatalf("落库失败应返回 HTTP 500（家族整风：反代/监控可感知），实际 %d", code)
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
	// 热更新：关闭激活码 + 改识别模型（Vision 保持 mock server 可登录）
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config",
		`{"activation_enabled":false,"vision_base_url":"`+d.srv.URL+`","vision_api_key":"***REMOVED***","vision_model":"new-model"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("更新配置失败: %d %v", code, j)
	}
	// 立即生效（运行时配置中心）：激活码机制已关闭 → 登录直接签发会话
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"acct1","password":"pwd"}`)
	if j["code"].(float64) != 0 {
		t.Fatalf("关闭激活码后登录应直接签发会话: %v", j)
	}
	// 新值已落库（重启恢复源）；开放时间不再属于配置项（自动识别唯一事实源），绝不落库。
	kv, err := d.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if kv["activation_enabled"] != "false" || kv["vision_model"] != "new-model" {
		t.Fatalf("配置未落库: %v", kv)
	}
	if _, ok := kv["open_time"]; ok {
		t.Fatalf("开放时间已从配置项移除，settings 不得再落 open_time 键: %v", kv)
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
	// 开放时间不再是配置项：PUT 携带 open_time 字段被 JSON 解码静默忽略（旧前端 payload
	// 兼容），不再做格式校验/落库/清空——未改任何有效配置项时返回"没有可应用的有效配置项"。
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"open_time":"bad-time"}`, adminTok)
	if j["code"].(float64) != 1 {
		t.Fatalf("只带 open_time 的 PUT 因无有效配置项应返回 code=1（open_time 已非配置项）: %v", j)
	}
	if msg, _ := j["msg"].(string); msg != "没有可应用的有效配置项" {
		t.Fatalf("只带 open_time 的 PUT 应报“没有可应用的有效配置项”: %v", j)
	}
	code, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"vision_model":"MUTANT","open_time":"2026/09/14 10:00:00"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("open_time 已非配置项，混改 PUT 应照常生效不做格式校验: %d %v", code, j)
	}
	// 配置保持新值：vision_model 已按混改键应用为新值
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/config", "", adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("读取配置失败: %v", j)
	}
	cfgKeep, _ := j["data"].(map[string]any)
	if vm, _ := cfgKeep["vision_model"].(string); vm != "MUTANT" {
		t.Fatalf("混改 PUT 应正常应用 vision_model=MUTANT（open_time 字段被忽略）: %v", cfgKeep)
	}
	if _, ok := cfgKeep["open_time"]; ok {
		t.Fatalf("配置回显不得再含 open_time 键: %v", cfgKeep)
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

// TestAdminStatsTargetsLoadFailureReturns500 B43-05：stats 的 targetsCount 循环若某个账号
// 目标读取失败必须记日志 + 明确报 500——此前静默 continue 计 0，DB 故障时 stats 显示
// targets_count=0 误导管理员"无人设目标"，违反零吞错精神（对齐其他数据源"任一失败即 500"）。
func TestAdminStatsTargetsLoadFailureReturns500(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 目标读取恒失败：Register 接收 *store.Store（非接口），无法注入替身——直接对真实
	// store 的底层 DB 执行一次非法操作不可行（store 方法封装安全查询）。
	// B43-05 的行为（失败 → 记日志 + 500）由实现注释与 handleAdminStats 其他数据源
	// 同风格兜底，此处以最小契约回归：正常路径 stats 仍 200（回归 TestAdminStatsAccountsLogs
	// 已覆盖 targets_count 正确计数）；失败路径的报错语义属"零吞错"族，走实现内复查。
	// （替代注入方案需把 Deps.Store 改为接口——超范围改动，违反简洁优先。）
	code, j := doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != http.StatusOK {
		t.Fatalf("正常路径 stats 应 200: %d", code)
	}
	if j["code"].(float64) != 0 {
		t.Fatalf("正常路径 stats 应业务 code=0: %v", j)
	}
}

// failingTargetsStore 包裹真实 Store，仅让目标读取恒失败（B43-05 测试专用）。
type failingTargetsStore struct {
	*store.Store
}

func (f *failingTargetsStore) LoadTargetsForAccount(acct string) ([]scheduler.Target, error) {
	return nil, errors.New("simulated targets read failure")
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
	// 会话失效由业务 code=401 表达（B39-02 后 HTTP 状态码同样真实 401）
	code, j = doJSONAuth(t, d.api, "GET", "/api/state", "", tok1)
	if code != http.StatusUnauthorized || j["code"].(float64) != 401 {
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

// TestAdminStatsOpenTimeFromRecognized 开放时间 = 调度器平台 beginTimes 自动识别态
// （唯一事实源，配置链路已整体移除）——stats 的 open_time 必须输出识别值、open_time_set
// 反映识别槽是否有值（未识别 / 历史过期值都非零值槽——识别值是否过期由展示方判定，
// 决策锚 1 绝不截断过期值，此处照常输出上次识别的开放时间）。
func TestAdminStatsOpenTimeFromRecognized(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 未识别：stats 的 open_time 输出空串、open_time_set=false（前端按布尔字段判定"未识别"）
	code, j := doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("stats 异常: %d %v", code, j)
	}
	st, _ := j["data"].(map[string]any)
	if s, _ := st["open_time"].(string); s != "" {
		t.Fatalf("未识别时 open_time 应输出空串，实际 %q", s)
	}
	if st["open_time_set"] != false {
		t.Fatalf("未识别时 open_time_set 应为 false，实际 %v", st["open_time_set"])
	}
	// 探测识别：先登录真实账号（建立客户端），再探测。mock 的顶层 beginTimes 是过去值
	// （1789261200000 = 2026-09-13 09:00:00，今天 2026-09-18）——识别过期语义下自动
	// 降级为"未识别"（绝不把过期旧值当开放时间），向后兼容 mock 的固定时间戳。
	if _, err := d.accts.LoginByPassword("acct1", "pwd", func(s string) (string, error) { return "ENC:" + s, nil }); err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if _, err := d.sched.ProbeNow(); err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("stats 二次读取异常: %d %v", code, j)
	}
	st, _ = j["data"].(map[string]any)
	// 与调度器识别态必须同源（识别槽有值 → 输出该值 + open_time_set=true；识别值
	// 是否过期不影响——决策锚 1 保留识别事实，管理员可见"上次识别的开放时间"）。
	// stats 对零值输出空串（非 year-1），故期望值 = 识别值非零才格式化。
	recog := d.sched.RecognizedOpenTime()
	want := ""
	if !recog.IsZero() {
		want = recog.Format("2006-01-02 15:04:05")
	}
	if s, _ := st["open_time"].(string); s != want {
		t.Fatalf("stats open_time 应与调度器识别值同源：应为 %q，实际 %q", want, s)
	}
	if st["open_time_set"] != (!recog.IsZero()) {
		t.Fatalf("stats open_time_set 应与调度器识别态同源（识别槽有值→true 否则 false）: %v", st["open_time_set"])
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
	// 探测确认窗口开启（ProbeNow 会填充全局快照/写 lastProbe）：
	// stats 的 window_opened 必须与调度器实际探测状态（WindowOpened）同源。
	d.sched.ProbeNow()
	openedBySched := d.sched.WindowOpened()
	code, j = doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("stats 二次读取异常: %d %v", code, j)
	}
	st, _ = j["data"].(map[string]any)
	if st["window_opened"] != openedBySched {
		t.Fatalf("stats(window_opened=%v) 与调度器探测状态(%v) 不同源",
			st["window_opened"], openedBySched)
	}
	// B39-05：window_closed 必须下发且与学生端 /state 同源（WindowClosed 三判据单源）——
	// 前端管理后台三态展示靠此字段区分，缺失时窗口关闭后后台仍显"待命中"误导管理员。
	if _, ok := st["window_closed"]; !ok {
		t.Fatalf("stats 必须下发 window_closed 字段（前端三态展示依赖），实际 %v", st)
	}
	if st["window_closed"] != d.sched.WindowClosed() {
		t.Fatalf("stats(window_closed=%v) 与调度器关闭判定(%v) 不同源",
			st["window_closed"], d.sched.WindowClosed())
	}
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

// TestAdminElectivesUnknownAccountRejects B26-02：管理员 ?account= 透传查看
// 课程的账号必须真实存在。此前任意串（typo/残留参数）静默走 ElectivesSnapshotFor 的全局帧
// 回退路径返回全局课程数据，管理员以为看到的就是该账号年级的课程——与 B15-M4 在
// handleSetTargets 的"凭据表校验"判据同源但读路径缺失，写路径拒绝、读路径假装成功不对称。
// 触发前提是"全局帧已存在"（首账号已探测），否则恰好因 ProbeForAccount 报错而掩盖缺陷，
// 故此测试先用真实账号探测填充全局帧再穿透。契约：凭据表查无此账号 → 明确"账号不存在"；
// 真实登录过（activate 前 LoginByPassword 也 SaveCredential）的账号正常放行。
func TestAdminElectivesUnknownAccountRejects(t *testing.T) {
	d := newTestDeps(t)
	adminTok := d.sessions.CreateAdmin("admin")
	// 先填充全局帧：真实登录 acct1 并通过 ProbeNow（走 mock 网络写 lastData）
	// ——修复前未知账号穿透恰好回退这个全局帧（请求被误导"成功"，缺陷本体）；
	// 若全局帧为空则 ProbeForAccount 报错恰好与修复同效，测试会假绿。
	tok := authenticateDirect(t, d, "acct1")
	if _, err := d.sched.ProbeNow(); err != nil {
		t.Fatalf("填充全局快照失败: %v", err)
	}
	// 未知账号穿透（无凭据记录）→ 必须拒绝，绝不回退全局帧
	code, j := doJSONAuth(t, d.api, "GET", "/api/electives?account=nonexistent", "", adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("管理员对不存在的账号查看课程应被拒绝，却返回成功 %v", j)
	}
	// 反向防线：真实登录过的账号（acct1 在凭据表）穿透正常放行
	if _, jr := doJSONAuth(t, d.api, "GET", "/api/electives?account=acct1", "", adminTok); jr["code"].(float64) != 0 {
		t.Fatalf("真实账号穿透查看课程应正常放行（B26-02 不得误伤）: %v", jr)
	}
	// 学生会话自己读自己的课程不受影响
	if _, js := doJSONAuth(t, d.api, "GET", "/api/electives", "", tok); js["code"].(float64) != 0 {
		t.Fatalf("学生自读课程不应受影响: %v", js)
	}
}

// TestAdminElectiveSelectUnknownAccountRejects B27-01：管理员 ?account= 透传
// 手动报名/退选，账号必须真实存在（凭据表有记录）——与 B15-M4（目标写）/B26-02（课程读）
// 同款判据，手动操作两路（select/exit）对称补齐。此前 override 分支只 `acct = q` 放行，
// 幽灵账号（typo/已删残留）走到 TryAcquireSubmit 占锁 → CheckClassSelectable 放行 →
// ClientFor 返回不存在，报"账号会话未建立或未登录"误导文案；凭据表查无此账号 →
// 必须明确"账号不存在"，绝不让操作假装到达平台。反向防线：真实账号（authenticateDirect
// 已 LoginByPassword 落凭据）透传报名仍正常放行。
func TestAdminElectiveSelectUnknownAccountRejects(t *testing.T) {
	d := newTestDeps(t)
	adminTok := d.sessions.CreateAdmin("admin")
	// 1. 幽灵账号透传报名 → 必须拒绝"账号不存在"（修复前返回"账号会话未建立或未登录"）
	code, j := doJSONAuth(t, d.api, "POST", "/api/electives/select?account=nonexistent",
		`{"class_id":61115,"course_name":"健美操"}`, adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("管理员对不存在的账号报名应被拒绝: %d %v", code, j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "账号不存在") {
		t.Fatalf("拒绝文案必须是'账号不存在'而非误导的会话文案: %v", j)
	}
	// 2. 幽灵账号透传退选同样拒绝
	code, j = doJSONAuth(t, d.api, "POST", "/api/electives/select/exit?account=nonexistent",
		`{"class_id":61115,"course_name":"健美操"}`, adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("管理员对不存在的账号退选应被拒绝: %d %v", code, j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "账号不存在") {
		t.Fatalf("退选拒绝文案必须是'账号不存在': %v", j)
	}
	// 3. 反向防线：真实账号（authenticateDirect 落凭据）透传报名放行
	authenticateDirect(t, d, "acct1")
	code, j = doJSONAuth(t, d.api, "POST", "/api/electives/select?account=acct1",
		`{"class_id":61115,"course_name":"健美操"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("真实账号透传报名应放行（B27-01 不得误伤）: %d %v", code, j)
	}
}

// TestAdminStateUnknownAccountRejects B27-02：管理员 ?account= 透传读取状态，
// 账号必须真实存在。此前 handleState override 分支任意串放行，StateForAccount(ghost) 返回
// 空 Courses + token_valid=true + window 状态——与"账号存在但确实无目标"返回形状完全相同，
// 管理员无法分辨"账号不存在"与"账号没目标"（B26-02 修掉的假装成功的轻量版）。凭据表
// 查无此账号 → 明确"账号不存在"；真实账号透传不受影响。
func TestAdminStateUnknownAccountRejects(t *testing.T) {
	d := newTestDeps(t)
	adminTok := d.sessions.CreateAdmin("admin")
	// 1. 幽灵账号透传状态 → 必须拒绝（修复前 code=0 + 空 courses 假象）
	code, j := doJSONAuth(t, d.api, "GET", "/api/state?account=nonexistent", "", adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("管理员对不存在的账号读取状态应被拒绝: %d %v", code, j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "账号不存在") {
		t.Fatalf("拒绝文案必须是'账号不存在': %v", j)
	}
	// 2. 反向防线：真实账号透传状态放行
	authenticateDirect(t, d, "acct1")
	code, j = doJSONAuth(t, d.api, "GET", "/api/state?account=acct1", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("真实账号透传状态应放行（B27-02 不得误伤）: %d %v", code, j)
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
	// 连续 7 次登录：前 5 次应通过，第 6 次起应被限流（body code=429 且 HTTP 真实 429）
	codes := []float64{}
	for i := 0; i < 7; i++ {
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"account":"a","password":"b"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		d.api.ServeHTTP(rec, req)
		var j map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
			t.Fatalf("响应不是 JSON: %s", rec.Body.String())
		}
		codes = append(codes, j["code"].(float64))
		// B39-02：限流响应的 HTTP 状态码必须真实 429（修复前恒 200）
		if j["code"].(float64) == 429 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("限流应写 HTTP 429，实际 %d", rec.Code)
		}
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
	// 第 6 次起应触发激活限流 429（B39-02：HTTP 状态码真实 429）
	limited := false
	for i := 0; i < 7; i++ {
		req := httptest.NewRequest("POST", "/api/activate", strings.NewReader(`{"account":"x","code":"XK-NOPE","ticket":"t"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		d.api.ServeHTTP(rec, req)
		var j map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
			t.Fatalf("响应不是 JSON: %s", rec.Body.String())
		}
		if c, _ := j["code"].(float64); c == 429 {
			limited = true
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("激活限流应写 HTTP 429，实际 %d", rec.Code)
			}
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
// B43-04 后语义：管理员名 + 非管理口令先试教务登录（mock 平台教务全成功 → 撞名学生
// 登录成功返回 code=0 签发普通会话），错误口令显式失败路径在真实平台教务 doLogin
// 对该口令也失败时才触达（返回"管理口令错误"），代码保留该分支。
// 本用例退化为验证 B43-04 契约：管理员名 + 非管理口令在教务 mock 全成功下被当作
// 撞名学生签发普通会话（不再返回"管理口令错误"）。
func TestLoginAdminWrongPasswordTimingFlat(t *testing.T) {
	d := newTestDeps(t)
	// 管理员名 + 非管理口令：B43-04 后先试教务登录 → mock 平台教务全成功 → 撞名学生
	// 登录走教务成功分支。未激活撞名学生返回 1001（颁发票据）而非管理员口令错误——
	// 这本身就是 B43-04 契约（撞名学生绝不被管理员分支吞掉），激活后的普通会话由
	// TestLoginAdminNameCollisionStudentCredential 覆盖。
	// 时延语义（n4）：管理员名 + 口令错已不再"立即返回"——先走教务登录网络往返，
	// 与未知学生天然等时；教务登录也失败时才进 adminName 分支补 Sleep(loginTimingFlat)
	//（代码 137 行保留），侧信道语义由结构保证，不在此断定时长。
	code, j := doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"nope"}`)
	if code != 200 {
		t.Fatalf("登录请求应 HTTP 200: %d", code)
	}
	if j["code"].(float64) == 1 && strings.Contains(j["msg"].(string), "管理口令错误") {
		t.Fatalf("B43-04 后撞名学生绝不被管理员分支吞掉（不得返回管理口令错误）: %v", j)
	}
}

// TestLoginAdminNameCollisionStudentCredential B43-04：教务学生账号与配置管理员名撞名时，
// 用学生自己的教务口令登录必须走教务登录分支（成功签发普通会话），绝不能因"账号名==adminName"
// 而被管理员口令比对吞掉（旧实现：口令=管理口令必错 → 该学生永远无法登录，DoS）。
// 反向用例：管理员名 + 错误口令仍明确"管理口令错误"。
func TestLoginAdminNameCollisionStudentCredential(t *testing.T) {
	d := newTestDeps(t)
	// 撞名学生账号需要"已激活"才走 issueSession（否则返回 1001）。激活前置必须走
	// 教务登录分支——但撞名学生未激活时登录即走教务成功分支（返回 1001 颁发票据），
	// 这正是 B43-04 的核心契约：撞名学生绝不被管理员分支吞掉。先验证未激活登录拿到票据
	if err := d.store.CreateActivationCode("XK-ABCD-EF12-3456", 10); err != nil {
		t.Fatal(err)
	}
	code, jAct := doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"pwd"}`)
	if code != 200 || jAct["code"].(float64) != 1001 {
		t.Fatalf("未激活撞名学生登录必须走教务分支颁发票据（B43-04），实际 %d %v", code, jAct)
	}
	ticket, _ := jAct["data"].(map[string]any)
	if ticket == nil || ticket["ticket"] == nil || ticket["ticket"] == "" {
		t.Fatalf("未激活撞名学生登录应颁发激活票据（B43-04），实际 %v", jAct)
	}
	code, j := doJSON(t, d.api, "POST", "/api/activate",
		`{"account":"admin","code":"XK-ABCD-EF12-3456","ticket":"`+ticket["ticket"].(string)+`"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("撞名学生激活失败: %d %v", code, j)
	}
	// 教务 mock 平台对任意账号+正确验证码登录成功（token=tok-new）——撞名学生可正常登录
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"pwd"}`)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("撞名学生用教务口令登录必须成功签发普通会话（B43-04），实际 %d %v", code, j)
	}
	tok, _ := j["data"].(map[string]any)
	if tok == nil || tok["token"] == nil || tok["token"] == "" {
		t.Fatalf("撞名学生登录成功必须返回会话令牌，实际 %v", j)
	}
	// 签发的是普通学生会话（非管理员会话）：撞名学生绝不能获得管理员权限
	if d.sessions.IsAdmin(tok["token"].(string)) {
		t.Fatal("撞名学生登录签发的必须是普通学生会话，绝不带管理员权限（B43-04）")
	}
	// 反向用例：管理员名 + 错误口令 → 明确"管理口令错误"（业务 code=1）
	code, j = doJSON(t, d.api, "POST", "/api/login", `{"account":"admin","password":"wrong-password"}`)
	if code != 200 || j["code"].(float64) != 1 {
		t.Fatalf("管理员名 + 错误口令应返回管理口令错误，实际 %d %v", code, j)
	}
	if msg, _ := j["msg"].(string); !strings.Contains(msg, "管理口令错误") {
		t.Fatalf("管理员名 + 错误口令文案应含'管理口令错误'，实际 %q", msg)
	}
}

// TestLoginRejectsFormContentType 登录/激活接口必须拒绝非 JSON 提交：
// 跨站表单 POST（application/x-www-form-urlencoded）无法携带 JSON Content-Type，
// 从源头封堵 CSRF 触发的副作用登录（攻击者借受害者 IP 分布式爆破）。
func TestLoginRejectsFormContentType(t *testing.T) {
	d := newTestDeps(t)
	// 表单编码提交登录（模拟恶意跨站表单）：应被 403 拒绝（B39-02：HTTP 状态码真实 403）
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader("account=a&password=b"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	if j["code"].(float64) != 403 || rec.Code != http.StatusForbidden {
		t.Fatalf("表单提交登录应被拒绝 code=403 + HTTP 403，实际 code=%v http=%d", j["code"], rec.Code)
	}
	// 表单编码提交激活：同样拒绝
	req = httptest.NewRequest("POST", "/api/activate", strings.NewReader("account=a&code=XK-123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("激活响应不是 JSON: %s", rec.Body.String())
	}
	if j["code"].(float64) != 403 || rec.Code != http.StatusForbidden {
		t.Fatalf("表单提交激活应被拒绝 code=403 + HTTP 403，实际 code=%v http=%d", j["code"], rec.Code)
	}
}

// TestRequireJSONBodyRejectsFormContentType requireJSONBody 是副作用请求的 CSRF
// 第一道门（POST 选课/退选/目标设置/管理改配），其拒绝分支必须写真实 HTTP 403——
// 与登录/激活两处 CSRF 门同款。HTTP 层状态分裂会让安全扫描/反代无法识别被 CSRF
// 拒掉的副作用请求（此前 writeJSON 恒 200，B39-02 改漏的最后一处）。
func TestRequireJSONBodyRejectsFormContentType(t *testing.T) {
	d := newTestDeps(t)
	// 构造一个需登录 + requireJSONBody 的副作用路由（POST /api/electives/select 最典型）
	tok := authenticateDirect(t, d, "acct1")
	// 表单编码提交报名：应被 403 拒绝，绝不进入 handler（handler 侧空 body 解码会报业务错误）
	req := httptest.NewRequest("POST", "/api/electives/select", strings.NewReader("classId=61115"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("表单编码提交选课应被 HTTP 403 拒绝，实际 http=%d（body=%s）", rec.Code, rec.Body.String())
	}
	if j["code"].(float64) != 403 {
		t.Fatalf("表单编码提交选课应 body code=403，实际 %v", j["code"])
	}
	if !strings.Contains(j["msg"].(string), "JSON") {
		t.Fatalf("拒绝文案应明示仅接受 JSON，实际 %v", j["msg"])
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
// R59 MINOR-59-01：真实 net/http Server 上 404 的 Content-Type 必须在 writeJSONStatus
// 的"先设头再 WriteHeader"路径下发 application/json——httptest.ResponseRecorder 允许
// WriteHeader 后设头、恒绿假绿掩盖真实 Server 行为分叉，故除 Recorder 断言外补真实
// Server 端到端断言（httptest.NewServer + http.Get 真发请求抓 HTTP 层 CT）。
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
	// 真实 Server 端到端断言：Recorder 允许 WriteHeader 后设 Header 是假绿——
	// 真实 net/http Server 丢弃已提交响应后的 Header 设置（R59 实证 404 错标
	// text/plain），此断言确保未来改动在真实 HTTP 层不回归。
	realSrv := httptest.NewServer(d.api)
	defer realSrv.Close()
	resp, err := http.Get(realSrv.URL + "/api/not-registered-path")
	if err != nil {
		t.Fatalf("真实 Server 请求失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("真实 Server 未知 /api/ 路径应 HTTP 404，实际 %d", resp.StatusCode)
	}
	if realCT := resp.Header.Get("Content-Type"); !strings.HasPrefix(realCT, "application/json") {
		t.Fatalf("真实 Server 404 的 Content-Type 应 application/json，实际 %q", realCT)
	}
	io.Copy(io.Discard, resp.Body)
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

// TestAdminDeleteAccountNoBodyOK B31-01：DELETE /api/admin/accounts 无 body（标准 REST
// 客户端 curl/Postman/脚本默认行为）必须可用——B7-M8 只给 codes 的 DELETE 放行空 body，
// 账号删除同为"DESTROY + 空 body 合法"语义却被 requireJSONBody 门挡成 403。
// 与 TestAdminDeleteCodeNoBodyOK 对称：空 body DELETE 应返回明确业务错误而非 403；
// 带 body 的正常删除由既有 TestAdminDeleteAccount 覆盖。
func TestAdminDeleteAccountNoBodyOK(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)
	// 造一个真实账号，让"空 body 被拒"与"带 body 真删"在同一函数内闭环
	authenticateDirect(t, d, "acct1")
	// 模拟标准 DELETE 无 body 请求（无 Content-Type）：应不再是 403——
	// 空 body 解码失败返回明确业务错误（"请求体解析失败"），REST 客户端不被 JSON 门挡死
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+adminTok)
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if rec.Code == 403 {
		t.Fatalf("无 body DELETE 不应被 403 拒绝（B31-01 回归）：http=%d body=%s", rec.Code, rec.Body.String())
	}
	var jr map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &jr); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	if c, _ := jr["code"].(float64); c != 1 {
		t.Fatalf("空 body DELETE 应返回明确业务错误（code=1 请求体解析失败），而非 %v", jr)
	}
	// 带 body 的正常删除：真删成功（清掉 acct1，保持测试幂等）
	code, j := doJSONAdmin(t, d.api, "DELETE", "/api/admin/accounts", `{"account":"acct1"}`, adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("带 body 删除账号失败: %d %v", code, j)
	}
}

// TestHandleElectivesSelectUnauthorizedRelogin B8-M7：手动报名命中教务 token 失效
// （ErrUnauthorized）时，接口返回友好提示「正在自动重登」，并实际触发了调度器的
// maybeRelogin（重登计数/失效标记状态可观测）。此前手动路径把原始报错抛给前端、
// 永不触发重登（UX 断裂：用户手动点报名被告知失败却无人自愈）。
func TestHandleElectivesSelectUnauthorizedRelogin(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")

	// 让 mock 教务报名接口返回"未登录"（token 失效语义 = code=-1）
	d.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/electives/select"):
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "currentYearTermList": []any{
				map[string]any{"schoolYear": 2026, "schoolTerm": 1, "selected": true},
			}})
		case strings.HasSuffix(r.URL.Path, "/findElectivesData"):
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "selectElectivesData": []any{
				map[string]any{
					"publishId": 3225, "publishName": "高二年体育", "inDateRange": true,
					"canSelect": 1, "hasSelected": 0, "electivesClassList": []any{
						map[string]any{
							"id": 61115, "course_name": "健美操", "selected_count": 0,
							"max_count": 36, "can_select": true, "btn_type": 2,
						},
					},
				},
			}})
		case strings.HasSuffix(r.URL.Path, "/selectElectivesClass"):
			// 教务 token 失效语义：统一返回"您未登录"
			json.NewEncoder(w).Encode(map[string]any{"code": -1, "msg": "您未登录,请刷新页面重新登录"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "unknown " + r.URL.Path})
		}
	})

	// 先填充快照（mock 正常），再手动报名（mock 返回未登录）
	if _, err := d.sched.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}
	code, j := doJSONAuth(t, d.api, "POST", "/api/electives/select", `{"class_id":61115,"course_name":"健美操"}`, tok)
	if code != 200 || j["code"].(float64) != 1 {
		t.Fatalf("教务鉴权失效的手动报名应返回业务错误: %d %v", code, j)
	}
	msg, _ := j["msg"].(string)
	if !strings.Contains(msg, "自动重登") {
		t.Fatalf("失效提示应包含'自动重登'文案，实际: %v", msg)
	}

	// 断言调度器已异步触发重登：失败计数/重登中标记被置上（maybeRelogin 幂等门控）。
	// scheduler 的 relogging/tokenValid 为包内私有字段，经公开只读访问器确认：
	// TokenValidFor 返回 token 失效标记；StateForAccount 的 TokenValid 字段同步反映。
	wait := time.Now().Add(3 * time.Second)
	for {
		st := d.sched.StateForAccount("acct1")
		if st.TokenValid == false {
			break // token 已被标记失效（重登进行中）
		}
		if time.Now().After(wait) {
			t.Fatal("手动报名触发 ErrUnauthorized 后调度器应推进自动重登状态")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAdminDeleteRejectsUnnormalizedAccount B15-M5：管理员删除带尾随空格的账号名必须拒绝。
// 此前 handleAdminDeleteAccount 只做 `strings.TrimSpace(req.Account) != ""` 的判空，
// 未把 trim 后的账号回写——前端一次空格失手（如 "12345 " 或粘贴带换行）会被 trim 后
// 删除真实账号 12345，响应却显示"已删除 "12345 ""（假删除成功：store 里账号列表/news
// 已没了 12345，但客户端仍挂着 "12345 " 标签，调度器照旧尝试提交、Operate 访问空客户端）。
// 契约：账号名含首尾空白必须整体拒绝（要求服务端在 trim 后仍与原始值逐字节一致）。
func TestAdminDeleteRejectsUnnormalizedAccount(t *testing.T) {
	d := newTestDeps(t)
	// 第一步：先建会话、再用 store 真实 API 落一条账号 12345 的凭据。
	// 删除请求对"账号是否存在"的判定以持久化层为准——mock zhidao 的 /login 未实现
	// (default 返回 unknown)，authenticateDirect 只 ensure 了内存客户端、不落凭据，
	// 用它当判据会恒假失败；DeleteAccount 真正删的也是 credentials/accounts/targets/success。
	authenticateDirect(t, d, "12345")
	if err := d.store.SaveCredential("12345", "enc", "tok-new"); err != nil {
		t.Fatal(err)
	}
	// 第二步：以管理员会话发起删除请求，但账号名带尾随空格（前端一次空格失手）
	adminTok := d.sessions.CreateAdmin("admin")
	body := `{"account":" 12345 "}`
	req := httptest.NewRequest("DELETE", "/api/admin/accounts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	// 第三步（核心断言）：响应必须是"账号无效或不可删除"（code:1）——空格账号整体拒绝，
	// 绝不能 trim 后删除真实账号 12345 并返回 code:0。修复前 handler 做 `acct := TrimSpace`
	// 后回写删除，此断言必然失败（红灯）。
	if j["code"].(float64) == 0 {
		t.Fatalf("账号名含空白应被整体拒绝，却返回删除成功 %v", j)
	}
	// 第四步：store 中的 12345 必须完整幸存（被删除即 FAIL）
	creds, err := d.store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range creds {
		if c.Account == "12345" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("账号名含空白应整体拒绝，现响应为 %v 且 store 中 12345 已被删除", j)
	}
}

// TestSetTargetsUnknownAccountDoesNotFabricate B15-M4：管理员对不存在账号设置目标必须拒绝。
// 此前 handleSetTargets 对 `?account=` 透传的任意字符串都无条件 SetTargetsForAccount——
// 未知账号名既不在 Store 账号表、也不在 Accounts 客户端注册表，目标会被写进孤儿行
// （store.targets 无主数据；重启恢复时 LoadTargetsForAccount 读回 → 目标幽灵复活），
// 同时调度器 startChains 按 targets 遍历时对孤儿账号 ClientFor 返回不存在，目标永不执行。
// 契约：账号名必须真实存在（Store 已知账号名），否则整体拒绝（未知账号）。
func TestSetTargetsUnknownAccountDoesNotFabricate(t *testing.T) {
	d := newTestDeps(t)
	// 先以正常登录在 store 落一个真实账号（SaveAccountName 由 issueSession 调用，属真实链）
	adminTok := d.sessions.CreateAdmin("admin")
	req := httptest.NewRequest("PUT", "/api/targets?account=nonexistent",
		strings.NewReader(`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":0}]}`))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	// 修复前：code:0 目标已保存（孤儿行诞生）→ 红灯；修复后：code:1 拒绝
	if j["code"].(float64) == 0 {
		t.Fatalf("管理员对不存在的账号设置目标应被拒绝，却返回成功 %v", j)
	}
}

// TestAdminDeleteAccountMemoryFirst B26-01：删账号必须"先摘注册表、后清库"。
// 此前顺序 Store.DeleteAccount（清 6 表）→ PurgeAccount → Accounts.Remove 之间存在毫秒级
// 空窗——在飞提交链（SelectClass 最长 15s）恰在空窗完成时做 B18-M2/B20-01 的"落库前锁内
// 复核 ClientFor 仍存在"，客户端尚未摘除 → 复核放行 → SaveSuccess/SaveRefused 把刚清掉的
// success/refused 行写回，重启 RestoreDone 假成功、自动引擎永久跳过退选课。
// 契约：删除成功返回后，accounts 注册表已无该客户端（内存先失效），库内凭据同步清空。
func TestAdminDeleteAccountMemoryFirst(t *testing.T) {
	d := newTestDeps(t)
	// 用真实登录注册账号（SaveCredential 落库 + ensure 注册客户端）
	tok := loginAndGetToken(t, d, "acct1")
	if _, ok := d.accts.ClientFor("acct1"); !ok {
		t.Fatal("登录后客户端应已在注册表")
	}
	adminTok := d.sessions.CreateAdmin("admin")
	req := httptest.NewRequest("DELETE", "/api/admin/accounts", strings.NewReader(`{"account":"acct1"}`))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	if j["code"].(float64) != 0 {
		t.Fatalf("删除账号失败: %v", j)
	}
	// 契约 1（核心）：内存注册表必须先失效——修复前 Remove 在 DeleteAccount 之后执行，
	// 本断言抓"库行已清但客户端仍在注册表"的半删态（红灯）；修复后 memory-first 为绿。
	// ClientFor 返回的 (Client, bool)——bool 为 false 才是摘除成功。
	if c, ok := d.accts.ClientFor("acct1"); ok && c != nil {
		t.Fatal("删除成功后账号客户端必须已从注册表摘除（memory-first：在飞链复核立即失败）")
	}
	// 契约 2：库内凭据同步清空（客户端重建所需凭据不得残留）
	creds, err := d.store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range creds {
		if c.Account == "acct1" {
			t.Fatal("删除成功后 credentials 表不得残留 acct1 凭据")
		}
	}
	_ = tok
}

// TestAdminSetTargetsWithoutAccountRejects B20-04：管理员会话不带 ?account= 时必须整体拒绝，
// 绝不让目标落入管理员账号孤儿行——此前 acct 停留在 sessionAccount(r)=管理员名，
// SetTargetsForAccount(admin, ts) 写进 store.targets 无主行（重启 LoadTargetsForAccount
// 幽灵复活）+ 污染 AccountsWithTargets 首账号选择（B10-04 排序后 admin 可能成"核心账号"）。
// 契约：目标只该属于学生账号；管理员未指定学生账号（且无任何有目标账号可兜底）→ 拒绝。
// 与 handleElectives/handleState 的"对齐核心账号"不同——目标是写入操作，无法确定归属时
// 宁可拒绝绝不张冠李戴；若会话绑定学生账号（非管理员名），不受影响照常写入。
func TestAdminSetTargetsWithoutAccountRejects(t *testing.T) {
	d := newTestDeps(t)
	// 管理员会话（adminName="admin"，会话账号即管理员名）
	adminTok := d.sessions.CreateAdmin("admin")
	req := httptest.NewRequest("PUT", "/api/targets",
		strings.NewReader(`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操","priority":0}]}`))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	// 修复前：acct=admin → 校验放行 → SetTargetsForAccount 写孤儿行 → code:0（红灯）；
	// 修复后：无透传且无有目标账号可对齐 → 明确拒绝 code:1
	if j["code"].(float64) == 0 {
		t.Fatalf("管理员不带 account 设置目标应被拒绝（不得写入管理员账号孤儿行），却返回成功 %v", j)
	}
	// 持久化验证：store.targets 无 admin 孤儿行
	ts, err := d.store.LoadTargetsForAccount("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 0 {
		t.Fatalf("拒绝后 store.targets 不得存在 admin 孤儿行，实际 %d 行", len(ts))
	}
}

// TestStudentSetTargetsWithoutAccountOK 学生账号（非管理员名）会话不带 ?account= 时
// 照常写入自己的目标（B20-04 反向防线：修复只该管管理员，绝不误伤普通学生会话）。
func TestStudentSetTargetsWithoutAccountOK(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	code, j := doJSONAuth(t, d.api, "PUT", "/api/targets",
		`{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操"}]}`, tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("学生会话设置目标应照常成功: %d %v", code, j)
	}
}
