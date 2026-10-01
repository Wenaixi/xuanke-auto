package upstream

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeRecognizer 固定返回指定文本的识别引擎（Vision ocr 引擎之外的假实现，登录链路测试用）。
type fakeRecognizer struct {
	mu    sync.Mutex
	text  string
	calls int
	failN int // 前 N 次返回错误（模拟识别失败触发刷新重试）
}

func (f *fakeRecognizer) Recognize(img []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failN {
		return "", errFakeRecognize
	}
	return f.text, nil
}

func (f *fakeRecognizer) callsCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

var errFakeRecognize = errors.New("fake 识别失败")

func fixedRecognizer(text string) *fakeRecognizer {
	return &fakeRecognizer{text: text}
}

// TestLoginEngineRoundTrip 登录链路深模块首次可测（TDD 红灯）：
// 用 httptest 起最小登录平台（/login /login/captcha /login/doLogin 三态），
// 注入固定识别引擎，断言 LoginEngine.Login 完整走通并返回平台 token。
func TestLoginEngineRoundTrip(t *testing.T) {
	var doLoginHits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.Write([]byte("<html></html>"))
		case "/login/captcha":
			w.Write([]byte("FAKEIMG"))
		case "/login/doLogin":
			mu.Lock()
			doLoginHits++
			mu.Unlock()
			_ = r.ParseForm()
			if r.Form.Get("captcha") == "" || r.Form.Get("identification") == "" || r.Form.Get("uniqueId") == "" {
				w.Write([]byte(`{"code":-1,"isOk":false,"token":"","msg":"参数缺失"}`))
				return
			}
			w.Write([]byte(`{"code":0,"isOk":true,"token":"tok-123","msg":"登录成功"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	e := NewLoginEngine(testDescriptor(srv.URL), srv.URL, VisionConfig{})
	e.UpdateCaptcha(fixedRecognizer("1234"), VisionConfig{})
	tok, err := e.Login("u", "p")
	if err != nil {
		t.Fatalf("期望登录成功，got %v", err)
	}
	if tok != "tok-123" {
		t.Fatalf("token = %q, want %q", tok, "tok-123")
	}
	if doLoginHits != 1 {
		t.Fatalf("doLogin 应只调 1 次，got %d", doLoginHits)
	}
}

// TestLoginEngineRecognizerRetry 识别失败刷新重试（识别≤3 契约）：
// 识别引擎前 2 次失败、第 3 次成功 → doLogin 最终 1 次成功（验证码重试循环收敛）。
func TestLoginEngineRecognizerRetry(t *testing.T) {
	var doLoginHits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.Write([]byte("<html></html>"))
		case "/login/captcha":
			w.Write([]byte("FAKEIMG"))
		case "/login/doLogin":
			mu.Lock()
			doLoginHits++
			mu.Unlock()
			w.Write([]byte(`{"code":0,"isOk":true,"token":"tok-1","msg":"登录成功"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	e := NewLoginEngine(testDescriptor(srv.URL), srv.URL, VisionConfig{})
	rec := fixedRecognizer("1234")
	rec.failN = 2 // 前 2 次识别失败 → 第 3 次成功（识别≤3 预算内）
	e.UpdateCaptcha(rec, VisionConfig{})
	tok, err := e.Login("u", "p")
	if err != nil {
		t.Fatalf("识别失败重试后应登录成功，got %v", err)
	}
	if tok != "tok-1" {
		t.Fatalf("token = %q, want tok-1", tok)
	}
	if rec.callsCount() != 3 {
		t.Fatalf("识别应恰好 3 次（前 2 失败 + 第 3 成功），got %d", rec.callsCount())
	}
	if doLoginHits != 1 {
		t.Fatalf("doLogin 应只在识别成功后调 1 次，got %d", doLoginHits)
	}
}

// TestSubmitLoginOmitsEmptyKeyNames 登录表单绝不出现空键名。
//
// 回归动机：url.Values.Set("", v) 会写出 `=v`（空键名），平台多半直接拒，
// 现场报的是"参数缺失"这类难定位的错误。Captcha/UniqueID/PriorityID 三处
// 早已守卫，Identification 曾是裸调——四个登录表单键里唯一的漏网。
//
// 判据用键名迭代而非 Form.Get("")：空键名恰恰取不到值，只能靠遍历发现。
// 变异验证：摘掉 submitLogin 里 Form.Identification 的判空，本测试必须红。
func TestSubmitLoginOmitsEmptyKeyNames(t *testing.T) {
	var gotKeys []string
	var gotIdent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/doLogin":
			_ = r.ParseForm()
			for k := range r.Form {
				gotKeys = append(gotKeys, k)
			}
			gotIdent = r.Form.Get("identification")
			w.Write([]byte(`{"code":0,"isOk":true,"token":"tok-1","msg":"登录成功"}`))
		default:
			w.Write([]byte("{}"))
		}
	}))
	defer srv.Close()

	d := testDescriptor(srv.URL)
	d.Captcha.Enabled = false // 无验证码：Captcha 键名留空，走守卫分支
	d.Form.Identification = "" // 该站登录表单无密文字段（契约上由 Validate 拦下，
	//                            此处直构以验证提交侧的守卫本身有效）
	d.Form.UniqueID = ""
	d.Login.DeviceID = func(string, time.Time) string { return "" }
	e := NewLoginEngine(d, srv.URL, VisionConfig{})

	// 会话客户端必须带 Cookie Jar：submitLogin 收尾要从 jar 回收平台下发的
	// 会话 Cookie（httptest.Server.Client() 的 Jar 为 nil，会当场空指针）。
	jar, _ := cookiejar.New(nil)
	sess := &http.Client{Timeout: 15 * time.Second, Jar: jar}
	if _, err := e.submitLogin(sess, "UA", "1234", "enc-uid"); err != nil {
		t.Fatalf("提交登录失败: %v", err)
	}
	for _, k := range gotKeys {
		if k == "" {
			t.Fatalf("表单含空键名（会写出 `=value`），实际键集合=%v", gotKeys)
		}
	}
	if gotIdent != "" {
		t.Fatalf("档案未声明 Identification 键名就不该提交该键，实际收到 %q", gotIdent)
	}
}
