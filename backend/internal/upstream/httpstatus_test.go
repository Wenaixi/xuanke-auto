package upstream

// HTTP 状态码层的未登录判据。
//
// 回归动机（实测证据，非推测）：平台只回 HTTP 状态码表达会话失效时，引擎
// 走 doRequest 只读响应体，业务路径全程不看 resp.StatusCode，实测四形态：
//   401+空体   → 报「响应解析失败: unexpected end of JSON input」（误判成网络抖动，
//                调度器按可重试处理 → 无限重试且**永不触发自动重登**）
//   401+业务码 → **err=nil 完全静默放行**（Envelope 只匹配档案声明的 code 形态）
//   403       → 同上静默放行
//   500+空体   → 报解析失败
// 「静默放行」最危险：会话已死引擎以为还活着，报名请求全部失败而日志一片正常。
//
// 档案字段 UnauthorizedStatuses 声明「哪些 HTTP 状态码代表未登录」。**零值 =
// 不声明 = 行为逐字不变**（沿用只靠响应体表达未登录的站点，如知到），
// 纯 REST 平台按标准声明 401/403 即可。

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newStatusAuthTestClient 造一个声明了 HTTP 未登录状态码的客户端。
func newStatusAuthTestClient(baseURL string, statuses []int) *Client {
	d := testDescriptor(baseURL)
	d.UnauthorizedStatuses = statuses
	return New(d, baseURL, VisionConfig{})
}

// TestHTTPUnauthorizedStatusFromProfile 档案声明的 HTTP 状态码必须上抛
// ErrUnauthorized，且**在读响应体之前**判定——否则 401+空体会先崩在 JSON 解析上。
func TestHTTPUnauthorizedStatusFromProfile(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			// 空体：确保判据不依赖响应体内容（401+空体是实测中最该被识别的形态）。
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			c := newStatusAuthTestClient(srv.URL, []int{http.StatusUnauthorized, http.StatusForbidden})
			_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("HTTP %d 应识别为未登录（否则自动重登永不触发），实际 err=%v", status, err)
			}
		})
	}
}

// TestHTTPUnauthorizedBeatsBodyParse 401+业务码（信封码与档案声明不匹配）时，
// HTTP 状态码判据必须**先于**信封判据生效，否则会被静默放行。
func TestHTTPUnauthorizedBeatsBodyParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		// 业务码刻意不等于夹具的 CodeUnauthorized，且 isOk=true —— 旧实现会当成功。
		_, _ = w.Write([]byte(`{"code":401,"isOk":true,"msg":"session expired"}`))
	}))
	defer srv.Close()

	c := newStatusAuthTestClient(srv.URL, []int{http.StatusUnauthorized})
	_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("HTTP 401 必须先于信封判据生效，否则会话失效被静默放行，实际 err=%v", err)
	}
}

// TestHTTPUnauthorizedZeroValueUnchanged 零值档案（未声明状态码）行为必须逐字不变：
// 400 不被当成未登录（那是客户端错误，不是会话失效），否则既有档案行为漂移。
func TestHTTPUnauthorizedZeroValueUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":0,"isOk":true}`))
	}))
	defer srv.Close()

	c := newStatusAuthTestClient(srv.URL, nil) // 零值：未声明
	_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("未声明状态码时 400 不应被当成未登录，实际 err=%v", err)
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Fatal("未声明状态码的档案不得把 400 判为未登录")
	}
}

// TestHTTPUnauthorizedNotDeclaredForOtherStatus 声明了 401 的档案遇 500 时
// 不得误判为未登录——500 是服务端故障，重试即可，判成会话失效会引发重登风暴。
func TestHTTPUnauthorizedNotDeclaredForOtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newStatusAuthTestClient(srv.URL, []int{http.StatusUnauthorized})
	_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
	if errors.Is(err, ErrUnauthorized) {
		t.Fatal("500 是服务端故障，不得判为未登录（否则触发无意义的重登风暴）")
	}
}
