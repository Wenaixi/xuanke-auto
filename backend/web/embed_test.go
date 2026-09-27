package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 静态图标资源可服务性回归钉。
//
// 为什么必须断言「文件头签名」而不只看状态码：SpaHandler 对嵌入式文件系统里不存在的
// 路径会回退返回 index.html 并给 HTTP 200。若前端漏 build（embed 是编译期固化），
// 请求 /favicon.ico 会拿到一坨 HTML 却是 200，浏览器表现为「标签页图标一直不出来」
// 而控制台毫无报错——无声故障只能靠字节断言抓。
func TestSpaHandlerServesIconAssets(t *testing.T) {
	handler := SpaHandler()

	cases := []struct {
		path      string
		magic     []byte
		magicName string
	}{
		{"/favicon.ico", []byte{0x00, 0x00, 0x01, 0x00}, "ICO 目录头（reserved=0 type=1）"},
		{"/logo.png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, "PNG 签名"},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("%s 状态码须 200, got %d", c.path, rec.Code)
		}
		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			t.Fatalf("%s Content-Type 须为图片, got %q（很可能是回退成了 index.html）", c.path, contentType)
		}
		body := rec.Body.Bytes()
		if !bytes.HasPrefix(body, c.magic) {
			t.Fatalf("%s 文件头不是 %s，实得前 8 字节 %v", c.path, c.magicName, body[:min(8, len(body))])
		}
	}
}
