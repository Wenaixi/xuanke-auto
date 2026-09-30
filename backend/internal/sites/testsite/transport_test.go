package testsite

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"xuanke-auto/backend/internal/upstream"
)

// TestDecodeEnvelopeStates Envelope 钩子三态：未登录码必须被识别为未登录
// （这是引擎重登的唯一触发依据，漏判即调度器静默停摆）；
// 正常 JSON 交下层解码；非 JSON（平台回纯文本错误页）返回 err 让引擎跳过信封检查。
// 回归动机：引擎曾写死解 `{"code":int}` —— 字符串码直接崩、纯文本直接崩，
// 而「状态键名不同」的未登录响应会被当成功**静默放行**。
func TestDecodeEnvelopeStates(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantUnauth bool
		wantErr    bool
	}{
		{name: "字符串未登录码", body: `{"status":"UNAUTHORIZED"}`, wantUnauth: true},
		{name: "成功状态", body: `{"status":"OK"}`},
		{name: "无状态键的正常响应", body: `{"data":{"classes":[]}}`},
		{name: "纯文本错误页", body: `UNAUTHORIZED`, wantErr: true},
		{name: "HTML 错误页", body: `<html>502</html>`, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, unauth, err := decodeEnvelope([]byte(c.body))
			if c.wantErr {
				if err == nil {
					t.Fatalf("%s 应返回 err 让引擎跳过信封检查", c.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s 不应报错: %v", c.body, err)
			}
			if unauth != c.wantUnauth {
				t.Fatalf("%s 未登录判据 = %v, want %v", c.body, unauth, c.wantUnauth)
			}
		})
	}
}

// TestTransportEndToEndOverRealHTTP 端到端穿过真实 HTTP 往返，证明本档案的
// 四个传输层维度真的表达得出——它们是「接缝已解耦」的机械证据：
//  1. 鉴权只经 Authorization 头，URL 不带任何 token 查询串；
//  2. 平台返回的未登录是字符串状态，引擎上抛 ErrUnauthorized（而非静默当成功）；
//  3. jQuery 指纹头已被档案删除；
//  4. 平台回纯文本错误页时引擎不崩，交下层解码器判定。
func TestTransportEndToEndOverRealHTTP(t *testing.T) {
	var gotQuery, gotAuth, gotXHR string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotXHR = r.Header.Get("X-Requested-With")
		w.Write([]byte(`{"status":"UNAUTHORIZED"}`))
	}))
	defer srv.Close()

	d := Descriptor()
	d.DefaultBaseURL = srv.URL
	c := upstream.New(d, srv.URL, upstream.VisionConfig{})
	c.SetCredentials("acct", "pw", "TOK-9")

	// 维度 1 + 2 + 3：请求形态与未登录判据。
	_, err := c.FindElectives()
	if !errors.Is(err, upstream.ErrUnauthorized) {
		t.Fatalf("字符串未登录状态应上抛 ErrUnauthorized，实际: %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("Header 鉴权档案不该带 token 查询串，实际: %q", gotQuery)
	}
	if gotAuth != "Bearer TOK-9" {
		t.Fatalf("Authorization 应为 Bearer TOK-9，实际: %q", gotAuth)
	}
	if gotXHR != "" {
		t.Fatalf("档案要求删除 jQuery 指纹头，实际收到: %q", gotXHR)
	}
}

// TestPureTextErrorPageDoesNotCrash 平台回纯文本错误页（网关 502、WAF 拦截页）时，
// 引擎必须跳过信封检查并把 body 交下层解码器，绝不在第一次请求就崩。
// 回归动机：引擎曾写死解 `{"code":int}`，纯文本响应体直接触发
// `响应解析失败: invalid character 'U' looking for beginning of value`。
func TestPureTextErrorPageDoesNotCrash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`UNAUTHORIZED`))
	}))
	defer srv.Close()

	d := Descriptor()
	d.DefaultBaseURL = srv.URL
	c := upstream.New(d, srv.URL, upstream.VisionConfig{})
	c.SetCredentials("acct", "pw", "TOK-9")

	_, err := c.FindElectives()
	if err == nil {
		t.Fatal("纯文本响应体最终应由下层解码器判失败")
	}
	// Envelope 已弃权（不是未登录形态），故不得报 ErrUnauthorized——
	// 否则等于把「看不懂的响应」当成「会话失效」，会触发无意义的自动重登风暴。
	if errors.Is(err, upstream.ErrUnauthorized) {
		t.Fatalf("纯文本错误页不该被判未登录（Envelope 已弃权），实际: %v", err)
	}
}
