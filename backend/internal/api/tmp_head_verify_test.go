// 一次性验证：确认 HEAD 请求能进入 GET 注册的 admin 路由并命中 default 405。
// 这决定 handler 内 method switch 的 default 分支是"死代码"还是"可达防御"。
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTmpHeadReachesGetRoute(t *testing.T) {
	d := newTestDepsModeName(t, true, "admin")
	// HEAD /api/admin/codes 未显式注册，Go ServeMux 会回退到 GET 节点
	req := httptest.NewRequest(http.MethodHead, "/api/admin/codes", nil)
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	t.Logf("HEAD /api/admin/codes -> HTTP %d, body=%q", rec.Code, rec.Body.String())

	// 对照：显式注册了 HEAD 的路由不存在，这里验证 PATCH 走不到（无 PATCH 节点，
	// 且非 HEAD 故不回退 GET，应落 catch-all /api/ 404）
	req2 := httptest.NewRequest(http.MethodPatch, "/api/admin/codes", nil)
	rec2 := httptest.NewRecorder()
	d.api.ServeHTTP(rec2, req2)
	t.Logf("PATCH /api/admin/codes -> HTTP %d, body=%q", rec2.Code, rec2.Body.String())
}
