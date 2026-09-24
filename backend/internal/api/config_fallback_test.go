package api

import (
	"testing"
)

// TestAdminConfigCaptchaFallbackRoundTrip 管理端识别引擎兜底开关的端到端契约：
//   - GET 默认下发 captcha_fallback=false（默认两引擎互不回退，核心承诺）；
//   - PUT true 后运行时配置立即生效、GET 回显 true、落库可恢复（重启保持）；
//   - PUT false 可关回去（开关必须双向可改，不能只能开不能关）；
//   - 未提交该字段时保持原值（指针语义：缺席=不改动，绝不静默重置为 false）。
func TestAdminConfigCaptchaFallbackRoundTrip(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)

	// 默认关闭
	_, j := doJSONAdmin(t, d.api, "GET", "/api/admin/config", "", adminTok)
	cfg := j["data"].(map[string]any)
	if cfg["captcha_fallback"] != false {
		t.Fatalf("默认 captcha_fallback 必须为 false（两引擎互不回退），实际 %v", cfg["captcha_fallback"])
	}

	// 开启
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_fallback":true}`, adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("开启兜底应成功，实际 %v", j)
	}
	if !d.rt.Get().CaptchaFallback {
		t.Fatal("运行时配置必须立即生效（热重载，无需重启）")
	}
	_, j = doJSONAdmin(t, d.api, "GET", "/api/admin/config", "", adminTok)
	if j["data"].(map[string]any)["captcha_fallback"] != true {
		t.Fatalf("GET 应回显 true，实际 %v", j["data"])
	}

	// 落库持久化（重启恢复靠 settings 表）
	kv, err := d.store.LoadSettings()
	if err != nil {
		t.Fatalf("读取设置失败: %v", err)
	}
	if kv["captcha_fallback"] != "true" {
		t.Fatalf("必须落库 captcha_fallback=true（否则重启即回退默认），实际 %q", kv["captcha_fallback"])
	}

	// 缺席该字段的 PUT 不得重置开关（只改并发的 PUT 不能顺手关掉兜底）
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_concurrency":3}`, adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("改并发应成功，实际 %v", j)
	}
	if !d.rt.Get().CaptchaFallback {
		t.Fatal("未提交 captcha_fallback 时不得重置开关（指针缺席=不改动）")
	}

	// 关回去
	_, j = doJSONAdmin(t, d.api, "PUT", "/api/admin/config", `{"captcha_fallback":false}`, adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("关闭兜底应成功，实际 %v", j)
	}
	if d.rt.Get().CaptchaFallback {
		t.Fatal("开关必须可关闭（双向可改）")
	}
}

// TestAdminStatsReportsActiveCaptchaEngine stats 必须同时给出配置引擎与实际生效引擎——
// 兜底关闭后"配置 ddddocr 而本机无引擎"成为可达状态，只报配置值会让管理员
// 以为识别正常，实际每次登录都在失败。
func TestAdminStatsReportsActiveCaptchaEngine(t *testing.T) {
	d := newTestDeps(t)
	adminTok := adminTokenFor(t, d)

	_, j := doJSONAdmin(t, d.api, "GET", "/api/admin/stats", "", adminTok)
	if j["code"].(float64) != 0 {
		t.Fatalf("stats 应成功，实际 %v", j)
	}
	data := j["data"].(map[string]any)
	if _, ok := data["captcha_engine"]; !ok {
		t.Fatal("stats 必须含 captcha_engine（配置值）")
	}
	active, ok := data["captcha_active_engine"]
	if !ok {
		t.Fatal("stats 必须含 captcha_active_engine（实际生效引擎）")
	}
	if s, _ := active.(string); s == "" {
		t.Fatalf("实际生效引擎不得为空串（前端会展示空值），实际 %v", active)
	}
	// 测试夹具走真实 applyCaptchaRecognizerFor：CGO=0 构建下配置 ddddocr 且本机
	// 无 Python 时实际引擎为 none，有 Python 时为 ddddocr——两者都是合法真值，
	// 只断言"非空且取值属于值域"（不臆断宿主环境）。
	if s := active.(string); s != "ddddocr" && s != "vision" && s != "none" {
		t.Fatalf("实际引擎取值必须在 ddddocr/vision/none 内，实际 %q", s)
	}
}
