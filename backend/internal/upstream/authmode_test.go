package upstream

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthModeHeaderOnlyQuery PureHeader 平台不该在 URL 上收到 token 查询串。
// 回归动机：引擎曾无条件拼 `?TokenParam=TOK`，纯 Header 鉴权的现代 API 接不了；
// 参数名为空时更会拼出 `?=TOK` 的畸形 URL，平台直接拒而现场报"参数缺失"。
func TestAuthModeHeaderOnlyNoQueryString(t *testing.T) {
	var gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	d := testDescriptor(srv.URL)
	d.TokenParam = ""
	d.TokenCookie = ""
	d.AuthMode = AuthHeader
	d.HeaderTokenName = "Authorization"
	c := New(d, srv.URL, VisionConfig{})
	c.SetCredentials("acct", "pw", "TOK123")

	if _, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded"); err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("Header 通道不该带查询串，实际: %q", gotQuery)
	}
	if gotAuth != "Bearer TOK123" {
		t.Fatalf("Authorization 头应为 Bearer TOK123，实际: %q", gotAuth)
	}
}

// TestAuthModeDualKeepsQueryParam 零值档案（AuthDual）必须逐字保持旧行为：
// URL 仍带 token 查询串。没有这条守卫，本轮改动会静默改掉知到的实际请求形态。
func TestAuthModeDualKeepsQueryParam(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"code":0}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, VisionConfig{})
	c.SetCredentials("acct", "pw", "TOK123")

	if _, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded"); err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if gotQuery != "idToken=TOK123" {
		t.Fatalf("双通道档案应保留 idToken 查询串，实际: %q", gotQuery)
	}
}

// TestAuthModeQueryOnlyNoCookieName 仅查询通道的平台不必给 Cookie 名。
// 它同时证明 Validate 的鉴权判据确实随通道而变，而非一刀切要求两个名字。
func TestAuthModeQueryOnlyNoCookieName(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.AuthMode = AuthQuery
	d.TokenParam = "tk"
	d.TokenCookie = ""
	if err := d.Validate(); err != nil {
		t.Fatalf("仅查询通道不应要求 TokenCookie: %v", err)
	}
}

// TestAuthModeCookieOnlyNoParamName 仅 Cookie 通道的平台同样不必给查询参数名
// （与 TestAuthModeQueryOnlyNoCookieName 互为对称面：两条一起钉住判据
// 「按通道要求载体名」，任何一边漏改都会红）。
func TestAuthModeCookieOnlyNoParamName(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.AuthMode = AuthCookie
	d.TokenParam = ""
	d.TokenCookie = "sess"
	if err := d.Validate(); err != nil {
		t.Fatalf("仅 Cookie 通道不应要求 TokenParam: %v", err)
	}
}

// TestAuthModeHeaderRejectsMissingHeaderName 声明 Header 鉴权却没给头名：
// 引擎会往空头名写值而平台永远收不到鉴权——必须在装配面拦下。
func TestAuthModeHeaderRejectsMissingHeaderName(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.AuthMode = AuthHeader
	d.HeaderTokenName = ""
	err := d.Validate()
	if err == nil {
		t.Fatal("Header 通道缺头名时 Validate 应报错")
	}
	if !strings.Contains(err.Error(), "HeaderTokenName") {
		t.Fatalf("报错应点名 HeaderTokenName，实际: %v", err)
	}
}
