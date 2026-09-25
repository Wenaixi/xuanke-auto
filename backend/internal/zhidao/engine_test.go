package zhidao

import "testing"

// TestResolveCaptchaEngine 识别引擎决策单源纯函数（C5-1 TDD 表驱动）。
// 从 api/router.go:72 resolveCaptchaRecognizer 逐字下沉——契约（CLAUDE.md）：
//   - 兜底关闭：两引擎严格互不回退。配置 ddddocr 但本机无引擎 → Recognizer=nil
//     （登录报"未配置验证码识别引擎"，绝不静默换云端）；配置 vision 但无密钥 → 同样 nil。
//   - 兜底开启：双向兜底。ddddocr 不可用 → Vision；Vision 无密钥 → 本机 ddddocr。
// 探测成本契约：只有"确实可能用到本地 ddddocr"时才调用 hasLocal（起 Python 子进程
// 最坏 15s）——配置 vision 且开关关闭、或配置 vision 且已有密钥时都不得探测。
func TestResolveCaptchaEngine(t *testing.T) {
	// 默认探测：无内置 ddddocr、无本地 Python
	hasNative := func() bool { return false }
	hasLocal := func() bool { return false }
	vision := func(k string) EngineConfig {
		return EngineConfig{
			VisionBaseURL: "http://x", VisionAPIKey: k, VisionModel: "m",
			CaptchaEngine: "vision", CaptchaFallback: false,
		}
	}
	dddd := func(fallback bool, visionKey string) EngineConfig {
		return EngineConfig{
			VisionBaseURL: "http://x", VisionAPIKey: visionKey, VisionModel: "m",
			CaptchaEngine: "ddddocr", CaptchaFallback: fallback,
		}
	}
	// hasLocal 注入变体：模拟本机有 Python ddddocr
	hasLocalTrue := func() bool { return true }

	cases := []struct {
		name string
		cfg  EngineConfig
		hn   func() bool // hasNative
		hl   func() bool // hasLocal
		want string      // engine："ddddocr"/"vision"/"none"
	}{
		{"vision 有密钥", vision("sk-x"), hasNative, hasLocal, "vision"},
		{"vision 无密钥 兜底关", vision(""), hasNative, hasLocal, "none"},
		{"vision 无密钥 兜底开+本地无", EngineConfig{CaptchaEngine: "vision", CaptchaFallback: true, VisionAPIKey: "", VisionBaseURL: "http://x", VisionModel: "m"}, hasNative, hasLocal, "none"},
		{"vision 无密钥 兜底开+本地有", EngineConfig{CaptchaEngine: "vision", CaptchaFallback: true, VisionAPIKey: "", VisionBaseURL: "http://x", VisionModel: "m"}, hasNative, hasLocalTrue, "ddddocr"},
		{"ddddocr 无引擎 兜底关", dddd(false, ""), hasNative, hasLocal, "none"},
		{"ddddocr 无引擎 兜底开+vision 无密钥", dddd(true, ""), hasNative, hasLocal, "none"},
		{"ddddocr 无引擎 兜底开+vision 有密钥", dddd(true, "sk-x"), hasNative, hasLocal, "vision"},
		{"ddddocr 无引擎 兜底开+vision 无密钥+本地有", dddd(true, ""), hasNative, hasLocalTrue, "ddddocr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := ResolveCaptchaEngine(c.cfg, c.hn, c.hl)
			if r.Engine != c.want {
				t.Fatalf("ResolveCaptchaEngine(%+v).Engine = %q, want %q", c.cfg, r.Engine, c.want)
			}
			// 兜底关 + 引擎不可用 → Recognizer 必须 nil（绝不静默换引擎）
			if !c.cfg.CaptchaFallback && r.Engine == "none" && r.Recognizer != nil {
				t.Fatal("兜底关闭时不可用引擎必须返回 nil Recognizer")
			}
		})
	}
}
