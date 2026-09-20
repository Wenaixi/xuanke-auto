package zhidao

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// socketPreheat 测试夹具端口预加热：预创建并关闭一个 127.0.0.1 回环套接字，
// 排空 Windows 宿主回环 TIME_WAIT 队列冷启动期（httptest mock 服务器 accept 尚未
// 就绪即收到连接 → connectex 连接拒绝，R46 起多轮 flake 同根）。
// 与 api 包 newTestDepsModeName 的套接字预创建同款根治；本包装内各测试独立
// 建 httptest server，全量串行下首个 server 仍有冷启动窗口，故每个用 server 的
// 测试入口都调用一次。
func socketPreheat() {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		l.Close()
	}
}

// loginMockServer 构造登录链路 mock。
// failRecognize: 前 N 次识别返回空串（识别失败）；failSubmit: 前 M 次提交被拒。
func loginMockServer(t *testing.T, failRecognize, failSubmit int) (*httptest.Server, *int32, *int32) {
	t.Helper()
	socketPreheat()
	var captchas int32
	var submits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			n := atomic.AddInt32(&captchas, 1)
			if int(n) <= failRecognize {
				w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`)) // 识别失败：返回空
				return
			}
			w.Write([]byte(`{"choices":[{"message":{"content":" abcd "}}]}`)) // 识别成功
		case strings.HasSuffix(r.URL.Path, "/login/doLogin"):
			n := atomic.AddInt32(&submits, 1)
			if int(n) <= failSubmit {
				w.Write([]byte(`{"code":1,"isOk":false,"msg":"验证码错误"}`)) // 提交被拒
			} else {
				w.Write([]byte(`{"code":0,"isOk":true,"token":"tok-ok"}`))
			}
		default: // /login 与 /login/captcha
			w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &captchas, &submits
}

// TestLoginRetryWithinLimits 验证：识别失败可刷新重试，提交被拒会刷新验证码，最终成功。
func TestLoginRetryWithinLimits(t *testing.T) {
	srv, captchas, submits := loginMockServer(t, 1, 1) // 识别 1 次失败 + 提交 1 次被拒后成功
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	tok, err := c.Login("acct", "pwd")
	if err != nil {
		t.Fatalf("登录应成功: %v", err)
	}
	if tok != "tok-ok" {
		t.Fatalf("token 错误: %s", tok)
	}
	// 时序：识别失败(1) → 识别成功(2)+提交被拒 → 刷新重识别(3)+提交成功
	if got := atomic.LoadInt32(captchas); got != 3 {
		t.Fatalf("识别应 3 次（1失败+1被拒后刷新+1成功），实际 %d", got)
	}
	if got := atomic.LoadInt32(submits); got != 2 {
		t.Fatalf("提交应 2 次（1被拒+1成功），实际 %d", got)
	}
	if c.Token() != "tok-ok" {
		t.Fatalf("客户端 token 未登记: %s", c.Token())
	}
}

// TestLoginStopsAfterCaptchaExhausted 验证：识别连败到上限即停止，不无限重试。
func TestLoginStopsAfterCaptchaExhausted(t *testing.T) {
	srv, captchas, submits := loginMockServer(t, 99, 99) // 永远失败
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	_, err := c.Login("acct", "pwd")
	if err == nil {
		t.Fatal("登录应失败")
	}
	if got := atomic.LoadInt32(captchas); got != 3 {
		t.Fatalf("识别最多 3 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(submits); got != 0 {
		t.Fatalf("识别失败不应提交，实际 %d", got)
	}
}

// TestLoginNetworkErrorAbortsImmediately 验证：网络/配置错误立即返回，绝不重试。
func TestLoginNetworkErrorAbortsImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // 登录页 500
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	if _, err := c.Login("acct", "pwd"); err == nil {
		t.Fatal("应报错")
	}
}

// TestNoAutoRelogin 验证：token 失效（code=-1）时直接返回错误，不自动重登。
func TestNoAutoRelogin(t *testing.T) {
	var reloginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/login/doLogin") {
			atomic.AddInt32(&reloginCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "new-token-999"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/login") || strings.HasSuffix(r.URL.Path, "/login/captcha") {
			w.Write([]byte("ok"))
			return
		}
		tok := r.URL.Query().Get("idToken")
		w.Header().Set("Content-Type", "application/json")
		if tok == "old-token" {
			json.NewEncoder(w).Encode(map[string]any{"code": -1, "msg": "您未登录,请刷新页面重新登录"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "old-token")

	// token 失效：应返回 ErrUnauthorized，且不触发重登
	_, err := c.doRequest(http.MethodPost, "/electives/select", nil, "")
	if err == nil {
		t.Fatal("期望 code=-1 时报错")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("期望 ErrUnauthorized，实际 %v", err)
	}
	if atomic.LoadInt32(&reloginCalls) != 0 {
		t.Fatalf("不应自动重登，实际 %d 次", reloginCalls)
	}
}

// TestSetCookiesMergeSemantics 验证 SetCookies 合并语义（MAJOR-B）：
// 只写入给定键、绝不删除未提及的既有 Cookie（恢复旧会话时不能清掉
// _jfinal_captcha/_jfinal_token 等服务端会话 Cookie）。
func TestSetCookiesMergeSemantics(t *testing.T) {
	c := New("http://dummy", VisionConfig{BaseURL: "http://dummy", APIKey: "k", Model: "m"})
	// 模拟登录流程已收集的服务端会话 Cookie
	c.SetCookies(map[string]string{
		"zd_edu_cookie":       "tok-1",
		"_jfinal_captcha":     "abc",
		"_jfinal_token":       "xyz",
		"access_limit_cookie": "real-session-value",
	})
	// 恢复旧会话：只传两个键（模拟 manager.Restore）
	c.SetCookies(map[string]string{
		"zd_edu_cookie":       "tok-2",
		"access_limit_cookie": "1",
	})

	c.mu.Lock()
	ck := c.cookies
	c.mu.Unlock()
	// 更新键生效
	if ck["zd_edu_cookie"] != "tok-2" || ck["access_limit_cookie"] != "1" {
		t.Fatalf("给定键应被写入，实际 %v", ck)
	}
	// 未提及键必须保留（合并语义）：整体覆盖会清掉服务端会话 Cookie
	if ck["_jfinal_captcha"] != "abc" || ck["_jfinal_token"] != "xyz" {
		t.Fatalf("SetCookies 合并语义：未提及的既有 Cookie 必须保留，实际 %v", ck)
	}
}

// TestSetVisionKeepsLocalRecognizer 验证 SetVision 传入的 cfg.recognizer 为零值（nil）时，
// 绝不能清空当前生效的本地识别引擎（M6）：热更新 Vision 配置不应波及识别引擎选择。
func TestSetVisionKeepsLocalRecognizer(t *testing.T) {
	c := New("http://dummy", VisionConfig{BaseURL: "http://dummy", APIKey: "k", Model: "m"})
	c.SetRecognizer(localRecognizer{name: "ddddocr-local"})

	// 热更新 Vision 配置：调用方只传 VisionConfig，recognizer 字段为零值 nil
	c.SetVision(VisionConfig{BaseURL: "http://new", APIKey: "new-key", Model: "new-model"})

	c.mu.Lock()
	cur := c.visionCfg.recognizer
	c.mu.Unlock()
	if cur == nil {
		t.Fatal("SetVision 不得清空当前识别引擎（即便传入的 cfg.recognizer 为 nil）")
	}
	if l, ok := cur.(localRecognizer); !ok || l.name != "ddddocr-local" {
		t.Fatalf("SetVision 应保留本地引擎实例，实际 %T %#v", cur, cur)
	}
}

// localRecognizer 最小 CaptchaRecognizer 替身：区分"本地 ddddocr 引擎仍被保留"。
type localRecognizer struct {
	name string
}

func (l localRecognizer) Recognize(img []byte) (string, error) { return "abcd", nil }

// TestSetVisionRebuildsWhenCurrentIsVisionOrNil 验证 SetVision 在"当前引擎是 Vision 或 nil"时
// 按新配置重建 Vision 识别器（保持原有语义）。
func TestSetVisionRebuildsWhenCurrentIsVisionOrNil(t *testing.T) {
	c := New("http://dummy", VisionConfig{BaseURL: "http://dummy", APIKey: "", Model: ""})
	// 初始为 nil（APIKey 为空不自动建识别器）
	c.SetVision(VisionConfig{BaseURL: "http://new", APIKey: "new-key", Model: "new-model"})
	c.mu.Lock()
	cur := c.visionCfg.recognizer
	c.mu.Unlock()
	if cur == nil {
		t.Fatal("当前引擎为 nil 时 SetVision 应重建 Vision 识别器")
	}
	if v, ok := cur.(*VisionRecognizer); !ok || v.cfg.APIKey != "new-key" {
		t.Fatalf("重建的 Vision 识别器应用新配置，实际 %#v", v)
	}
}

// TestReloginIfNeeded 验证显式重登：一次即可，最多一次。
func TestReloginIfNeeded(t *testing.T) {
	var reloginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证码识别（硅基流动 mock）
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": " abcd "}}},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/login/doLogin") {
			atomic.AddInt32(&reloginCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "new-token-999"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/login") || strings.HasSuffix(r.URL.Path, "/login/captcha") {
			w.Write([]byte("ok"))
			return
		}
		tok := r.URL.Query().Get("idToken")
		w.Header().Set("Content-Type", "application/json")
		if tok == "old-token" {
			json.NewEncoder(w).Encode(map[string]any{"code": -1, "msg": "您未登录,请刷新页面重新登录"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true})
	}))
	defer srv.Close()

	// B6-01：运行时登录（/api/login）后客户端内部必须有账密——自动重登
	// ReloginIfNeeded 直接可用的回归测试。修复前：Login 成功分支不写 c.account/c.password，
	// 依赖 SetCredentials 的旧断言（ReloginIfNeeded 手动 SetCredentials 后可用）无法覆盖
	// 线上真实路径——每次账密登录后自动重登永远报"未登录且无保存账密"。
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	if _, err := c.Login("acct", "pwd"); err != nil {
		t.Fatalf("运行时登录失败: %v", err)
	}
	// 登录成功后立即触发自动重登：必须可用（修复前报"未登录且无保存账密"）
	// （Login 成功本身含 1 次 doLogin 提交，故此处总提交数为 2）
	relogged, err := c.ReloginIfNeeded()
	if err != nil || !relogged {
		t.Fatalf("运行时登录后的自动重登应可用: %v %v", relogged, err)
	}
	if atomic.LoadInt32(&reloginCalls) != 2 {
		t.Fatalf("重登应恰好触发 1 次（加初始登录共 2 次 doLogin），实际 %d", reloginCalls)
	}

	// 重启恢复路径（Restore 经 SetCredentials 注入）：保持原语义回归
	restoreC := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	restoreC.SetCredentials("acct", "pwd", "old-token")
	if _, err := restoreC.doRequest(http.MethodPost, "/electives/select", nil, ""); err == nil {
		t.Fatal("期望失败")
	}
	// 显式重登一次
	relogged, err = restoreC.ReloginIfNeeded()
	if err != nil || !relogged {
		t.Fatalf("重登失败: %v %v", relogged, err)
	}
	// 重登后 token 更新，再次请求成功
	body, err := restoreC.doRequest(http.MethodPost, "/electives/select", nil, "")
	if err != nil {
		t.Fatalf("重登后请求失败: %v", err)
	}
	var j struct {
		Code int `json:"code"`
	}
	json.Unmarshal(body, &j)
	if j.Code != 0 {
		t.Fatalf("重登后 code=%d", j.Code)
	}
	if atomic.LoadInt32(&reloginCalls) != 3 {
		t.Fatalf("最多重登 2 次（初始登录 + 重登 + doRequest 前共 3 次 doLogin），实际 %d", reloginCalls)
	}
}

// TestLoginLogsAttempts 登录链路的尝试日志：识别成功（引擎+位数）、提交被拒、失败收尾均可见。
func TestLoginLogsAttempts(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	srv, _, _ := loginMockServer(t, 1, 1) // 识别 1 次失败 + 提交 1 次被拒后成功
	t.Cleanup(srv.Close)
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	if _, err := c.Login("acct", "pwd"); err != nil {
		t.Fatalf("登录应成功: %v", err)
	}
	logs := buf.String()
	for _, want := range []string{
		"[login] 账号 acct 第1次验证码识别失败",
		"[login] 账号 acct 第2次验证码识别成功",
		"第2次验证码提交被拒", // 第 2 次识别（attempt 2）提交被拒
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("日志缺少 %q，实际输出：\n%s", want, logs)
		}
	}
}

// TestLoginLogsFailureSummary 登录全部失败时输出收尾日志（总尝试次数 + 最后原因）。
func TestLoginLogsFailureSummary(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	srv, captchas, _ := loginMockServer(t, 99, 99) // 永远失败
	t.Cleanup(srv.Close)
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	if _, err := c.Login("acct", "pwd"); err == nil {
		t.Fatal("登录应失败")
	}
	logs := buf.String()
	if !strings.Contains(logs, "[login] 账号 acct 登录失败（共 3 次识别尝试）") {
		t.Fatalf("日志缺少失败收尾，实际输出：\n%s", logs)
	}
	if got := atomic.LoadInt32(captchas); got != 3 {
		t.Fatalf("识别应 3 次，实际 %d", got)
	}
}

// TestLoginRetriesTransientInitError 验证：登录初始化会话（GET /login）遇瞬时连接
// 错误时重试一次即可成功——不能因低频网络抖动直接判登录失败。
// 根因背景（R46 起 4 轮 6+ 样本）：Windows 宿主 api 包测试内 httptest mock 服务器
// 连接瞬时失败（connectex），经 Login→fetchLoginPage 翻译成"初始化登录会话失败"
// 业务文案落在任意断言行上误红。此处为产品层网络瞬时抖动自愈重试——纯 GET /login
// 不消耗验证码限额，不违背"识别失败不刷限流"既有契约（网络层自愈，绝不含验证码重试）。
func TestLoginRetriesTransientInitError(t *testing.T) {
	var loginPages int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 首个 /login 请求掐断连接模拟瞬时连接失败；其余请求正常响应
		if strings.HasSuffix(r.URL.Path, "/login") && atomic.AddInt32(&loginPages, 1) == 1 {
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": " abcd "}}}})
		case strings.HasSuffix(r.URL.Path, "/login/doLogin"):
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "tok-ok"})
		default:
			w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	tok, err := c.Login("acct", "pwd")
	if err != nil {
		t.Fatalf("瞬时初始化失败后应重试成功: %v", err)
	}
	if tok != "tok-ok" {
		t.Fatalf("token 错误: %s", tok)
	}
	if got := atomic.LoadInt32(&loginPages); got != 2 {
		t.Fatalf("/login 应请求 2 次（1 失败 + 1 重试），实际 %d", got)
	}
}

// TestExitClass 验证退选接口：路径/请求体与真实 HAR 一致（form classId）。
func TestExitClass(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		gotPath = r.URL.Path
		gotBody = string(body)
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "msg": "退选成功"})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "tok")
	msg, err := c.ExitClass(61115)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "退选成功" {
		t.Fatalf("期望返回平台消息，实际 %q", msg)
	}
	if gotPath != "/electives/select/exitElectivesClass" {
		t.Fatalf("路径错误: %s", gotPath)
	}
	if gotBody != "classId=61115" {
		t.Fatalf("请求体错误: %s", gotBody)
	}
}

// TestExitClassFailsOnCodeNotZero 验证退选业务失败（code!=0）时报错。
func TestExitClassFailsOnCodeNotZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"code": 1, "isOk": false, "msg": "已过退选时间"})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "tok")
	if _, err := c.ExitClass(61115); err == nil {
		t.Fatal("期望业务失败时报错")
	}
}

// TestParseElectives 用 HAR 真实响应片段验证解析
func TestParseElectives(t *testing.T) {
	// 精简的 findElectivesData 响应（字段与真实一致）
	raw := `{"code":0,"beginTimes":[1789261200000],"selectElectivesData":[
	  {"publishId":3225,"publishName":"高二年体育","beginDate":"2026-09-13 09:00:00","inDateRange":false,
	   "canSelect":1,"hasSelected":0,"groupCount":1,"totalCount":3,
	   "electivesClassList":[{"id":61115,"publish_id":3225,"course_name":"健美操","class_name":"健美操1、2班",
	     "teacher_name_list":"陈跃强","class_room_name":"操场","lessons_date":null,"selected_count":0,
	     "max_count":36,"plan_count":36,"can_select":false,"btn_type":2,"btn_text":"报名"}]}
	]}`
	data, err := parseElectives([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 1 {
		t.Fatalf("期望 1 个发布，实际 %d", len(data.Publishes))
	}
	p := data.Publishes[0]
	if p.PublishID != 3225 || p.PublishName != "高二年体育" {
		t.Fatalf("发布解析错误: %+v", p)
	}
	if p.InDateRange {
		t.Fatal("窗口应未开放")
	}
	if len(p.Classes) != 1 || p.Classes[0].ID != 61115 || p.Classes[0].CourseName != "健美操" {
		t.Fatalf("课程解析错误: %+v", p.Classes)
	}
}