package upstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newEnvelopeTestClient 造一个只关心传输层的最小客户端：复用测试夹具档案，
// 覆盖 Envelope 钩子后指向给定假服务器。
func newEnvelopeTestClient(baseURL string, env func([]byte) (int, bool, error)) *Client {
	d := testDescriptor(baseURL)
	d.Envelope = env
	return New(d, baseURL, VisionConfig{})
}

// TestEnvelopeUnauthorizedFromProfile 档案判定的未登录必须上抛 ErrUnauthorized。
// 回归动机：引擎曾写死解 `{"code":int}`，于是「未登录状态键名不同」的站点
// （如 {"status":401}）会被当成成功静默放行——token 失效后的自动重登永不触发，
// 调度器无声停摆且现场无任何日志线索。
func TestEnvelopeUnauthorizedFromProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"UNAUTHORIZED"}`))
	}))
	defer srv.Close()

	c := newEnvelopeTestClient(srv.URL, func(body []byte) (int, bool, error) {
		var j struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &j); err != nil {
			return 0, false, err
		}
		return 0, j.Status == "UNAUTHORIZED", nil
	})

	_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("档案判定未登录时应上抛 ErrUnauthorized，实际: %v", err)
	}
}

// TestEnvelopeNonJSONBodySkipsCheck 响应体不是档案形态（如平台回纯文本错误页）时，
// 引擎必须跳过信封检查把原始 body 交给下层解码器，而不是自己崩掉。
func TestEnvelopeNonJSONBodySkipsCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`UNAUTHORIZED`))
	}))
	defer srv.Close()

	c := newEnvelopeTestClient(srv.URL, func(body []byte) (int, bool, error) {
		return 0, false, fmt.Errorf("不是本形态")
	})

	body, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("非本形态响应体不应让引擎报错，实际: %v", err)
	}
	if string(body) != "UNAUTHORIZED" {
		t.Fatalf("body 应原样交给下层，实际: %q", body)
	}
}

// TestEnvelopeNilFallsBackToLegacyCode 零值档案（Envelope == nil）必须逐字保持旧行为：
// 字符串 code 仍报「响应解析失败」、整数未登录码仍上抛 ErrUnauthorized。
// 这是本轮「零值兼容」硬约束的守卫，改默认分支时它会红。
func TestEnvelopeNilFallsBackToLegacyCode(t *testing.T) {
	t.Run("字符串码仍报解析失败", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":"UNAUTHORIZED"}`))
		}))
		defer srv.Close()

		c := newEnvelopeTestClient(srv.URL, nil)
		_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
		if err == nil || !strings.Contains(err.Error(), "响应解析失败") {
			t.Fatalf("回落路径行为应与改动前一致（响应解析失败），实际: %v", err)
		}
	})

	t.Run("整数未登录码仍上抛", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"code":-1}`))
		}))
		defer srv.Close()

		c := newEnvelopeTestClient(srv.URL, nil)
		_, err := c.doRequest(http.MethodPost, "/x", nil, "application/x-www-form-urlencoded")
		if !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("回落路径应仍识别整数未登录码，实际: %v", err)
		}
	})
}
