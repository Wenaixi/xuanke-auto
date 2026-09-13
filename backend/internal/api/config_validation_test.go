package api

import (
	"strings"
	"testing"
)

// TestAdminConfigRejectsInvalidCaptchaEngine 管理端 PUT /api/admin/config 对非法识别引擎必须拒绝
// （D-A1：值域校验）——垃圾字符串不能进入运行时配置与落库，否则 stats 显示与实际引擎错位。
// 先 PUT 建立合法基线（production 里 main 启动恒注入 vision/1），再 PUT 非法值验证拒绝且基线不被污染。
func TestAdminConfigRejectsInvalidCaptchaEngine(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)

	// 建立合法基线：vision 引擎 + 并发 5
	_, j := doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_engine":"vision","captcha_concurrency":5}`, adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("建立基线失败: %v", j)
	}

	// 非法引擎：拒绝
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_engine":"garbage"}`, adminTok)
	if j["code"].(float64) != 1 {
		t.Fatalf("非法引擎应被拒绝 code=1，实际 %v", j)
	}
	if !strings.Contains(j["msg"].(string), "引擎") {
		t.Fatalf("拒绝文案应明示引擎值域: %v", j)
	}
	// 配置未被污染：运行时仍为 vision，并发仍为 5
	_, j = doJSONAdmin(t, d.api, "GET", "/api/admin/config", "", adminTok)
	cfg := j["data"].(map[string]any)
	if cfg["captcha_engine"] != "vision" {
		t.Fatalf("非法值不得污染运行时配置，仍应 vision，实际 %v", cfg["captcha_engine"])
	}
	if cfg["captcha_concurrency"].(float64) != 5 {
		t.Fatalf("非法引擎不应顺带改动并发，仍应 5，实际 %v", cfg["captcha_concurrency"])
	}
}

// TestAdminConfigRejectsCaptchaConcurrencyOutOfRange 管理端 PUT 对并发越界值必须拒绝——
// 上限 maxCaptchaConcurrency（20），下限 1；越界一律 code=1 且不改运行时/落库值。
func TestAdminConfigRejectsCaptchaConcurrencyOutOfRange(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)

	// 建立合法基线：并发 5
	_, j := doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_concurrency":5}`, adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("建立基线失败: %v", j)
	}

	// 并发 100000：拒绝
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_concurrency":100000}`, adminTok)
	if j["code"].(float64) != 1 {
		t.Fatalf("超限并发应被拒绝 code=1，实际 %v", j)
	}
	// 并发 0：拒绝
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_concurrency":0}`, adminTok)
	if j["code"].(float64) != 1 {
		t.Fatalf("低于下限并发应被拒绝 code=1，实际 %v", j)
	}
	// 运行时未被污染：仍为基线 5
	if got := d.rt.Get().CaptchaConcurrency; got != 5 {
		t.Fatalf("越界并发不得污染运行时配置，仍应 5，实际 %d", got)
	}
	// 边界值：上限 20 应接受且生效
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_concurrency":20}`, adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("上限 20 应被接受，实际 %v", j)
	}
	if got := d.rt.Get().CaptchaConcurrency; got != 20 {
		t.Fatalf("并发上限 20 应生效，实际 %d", got)
	}
}