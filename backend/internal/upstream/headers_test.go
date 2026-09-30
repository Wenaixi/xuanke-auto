package upstream

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHeadersProfileOverridesXHRFingerprint 档案声明 Headers 即逐条接管默认头集：
// 空串值显式删除 jQuery 指纹头，非空值精确覆盖。
// 回归动机：引擎曾无条件写死 `X-Requested-With: XMLHttpRequest` 与
// `Accept: application/json, text/javascript, */*; q=0.01`（jQuery 1.x 字节级指纹），
// 纯 REST 平台既不需要也可能被 WAF 拦，而档案此前连加一个自定义头的入口都没有。
func TestHeadersProfileOverridesXHRFingerprint(t *testing.T) {
	var gotXHR, gotAccept, gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXHR = r.Header.Get("X-Requested-With")
		gotAccept = r.Header.Get("Accept")
		gotAPIKey = r.Header.Get("X-API-Key")
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	d := testDescriptor(srv.URL)
	d.Headers = map[string]string{
		"X-Requested-With": "", // 空串 = 显式删除
		"Accept":           "application/json",
		"X-API-Key":        "K123",
	}
	c := New(d, srv.URL, VisionConfig{})

	if _, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded"); err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if gotXHR != "" {
		t.Fatalf("档案要求删除 X-Requested-With，实际收到: %q", gotXHR)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept 应被档案覆盖为 application/json，实际: %q", gotAccept)
	}
	if gotAPIKey != "K123" {
		t.Fatalf("档案自定义头应写入，实际: %q", gotAPIKey)
	}
}

// TestHeadersNilKeepsEngineDefaults 零值档案（Headers 未声明）必须逐字保持旧行为：
// 仍带 jQuery 指纹头。这是「零值兼容」硬约束在请求头维度的守卫。
func TestHeadersNilKeepsEngineDefaults(t *testing.T) {
	var gotXHR, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXHR = r.Header.Get("X-Requested-With")
		gotAccept = r.Header.Get("Accept")
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, VisionConfig{})
	if _, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded"); err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if gotXHR != "XMLHttpRequest" {
		t.Fatalf("零值档案应保留默认 X-Requested-With，实际: %q", gotXHR)
	}
	if gotAccept != "application/json, text/javascript, */*; q=0.01" {
		t.Fatalf("零值档案应保留默认 Accept 指纹，实际: %q", gotAccept)
	}
}

// TestFormEncodingJSONBody 档案声明 json 编码时，登录体必须是 JSON 且
// Content-Type 为 application/json（jQuery form 编码的现代替代形态）。
func TestFormEncodingJSONBody(t *testing.T) {
	var gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.Write([]byte("<html></html>"))
		case "/login/captcha":
			w.Write([]byte("FAKEIMG"))
		case "/login/doLogin":
			gotCT = r.Header.Get("Content-Type")
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.Write([]byte(`{"isOk":true,"token":"TOK"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	d := testDescriptor(srv.URL)
	d.Captcha = CaptchaSpec{Enabled: false}
	d.Form.Captcha = ""
	d.Form.UniqueID = ""
	d.FormEncoding = "json"

	e := NewLoginEngine(d, srv.URL, VisionConfig{})
	if _, err := e.Login("acct", "pw"); err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if gotCT != "application/json" {
		t.Fatalf("json 编码应发 application/json，实际: %q", gotCT)
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(gotBody), &obj); err != nil {
		t.Fatalf("登录体应为 JSON 对象，实际 %q: %v", gotBody, err)
	}
	if obj[d.Form.Identification] == "" {
		t.Fatalf("JSON 体应含档案声明的账密键 %q，实际: %v", d.Form.Identification, obj)
	}
	// 空键名契约在 JSON 路径上同样成立：绝不写出 "" 这个键。
	if _, leaked := obj[""]; leaked {
		t.Fatalf("JSON 体不得含空键名，实际: %v", obj)
	}
}

// TestFormEncodingRejectsUnknown 写错的编码名不会崩，只会让平台收到无法解析的
// 请求体（现场报"参数错误"）——必须在装配面拦下。
func TestFormEncodingRejectsUnknown(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.FormEncoding = "xml"
	err := d.Validate()
	if err == nil {
		t.Fatal("非法 FormEncoding 应被 Validate 拒绝")
	}
	if !strings.Contains(err.Error(), "FormEncoding") {
		t.Fatalf("报错应点名 FormEncoding，实际: %v", err)
	}
}
