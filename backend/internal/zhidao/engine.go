package zhidao

// EngineConfig 识别引擎决策入参（从 runtime.Config 解耦，避免 zhidao → runtime
// 反向依赖成环——runtime 目前依赖 zhidao，若 zhidao 反过来引 runtime 会 import cycle）。
// 字段与 runtime.Config 的识别相关字段一一对应，api/router 组装传入。
type EngineConfig struct {
	VisionBaseURL   string
	VisionAPIKey    string
	VisionModel     string
	CaptchaEngine   string // "ddddocr" / "vision"
	CaptchaFallback bool
}

// EngineResolution 引擎决策结果：识别器 + 实际生效引擎名 + 面向管理员的日志说明。
// Engine 字段："ddddocr" / "vision" / "none"（无可用引擎）——与配置值 CaptchaEngine
// 分列：兜底关闭时配置 ddddocr 而本机不可用会出现"配置=ddddocr、实际=none"，
// 只报配置值会误导管理员以为识别正常（stats 展示用）。
type EngineResolution struct {
	Recognizer CaptchaRecognizer
	Engine     string
	Note       string
}

// ResolveCaptchaEngine 识别引擎决策单源纯函数（下沉：api/router.go 的
// resolveCaptchaRecognizer 与 cmd/logintest 的 resolveLoginTestEngine 共用，取代复制）。
// 无副作用、探测函数注入，便于表驱动测试。
//
// 契约（fallback = CaptchaFallback 开关，默认关闭）：
//   - 关闭：两引擎严格互不回退。配置 ddddocr 但本机无引擎 → Recognizer=nil（登录报
//     "未配置验证码识别引擎"），绝不静默换云端；配置 vision 但无密钥 → 同样 nil。
//   - 开启：双向兜底。ddddocr 不可用 → Vision；Vision 无密钥 → 本机 ddddocr。
//
// 探测成本契约：只有"确实可能用到本地 ddddocr"时才调用 hasLocal（本地探测要起
// Python 子进程，最坏 15s）——配置 vision 且开关关闭、或配置 vision 且已有密钥时
// 都不得探测，否则管理员每次保存配置都要白等一次 Python 冷启动。
func ResolveCaptchaEngine(cfg EngineConfig, hasNative, hasLocal func() bool) EngineResolution {
	vision := func() CaptchaRecognizer {
		if cfg.VisionAPIKey == "" {
			return nil
		}
		return NewVisionRecognizer(VisionConfig{BaseURL: cfg.VisionBaseURL, APIKey: cfg.VisionAPIKey, Model: cfg.VisionModel})
	}
	// ddddocr 可用性惰性求值：原生内置 → 本机 Python；两者皆无返回 nil。
	// 类型断言用 reflect 类型名而非具体类型：NativeDdddOcrRecognizer 是 build-tag
	// 条件类型（非 Windows / 无 CGO 下不存在），直接断言会让 CGO=0 构建编不过。
	local := func() (CaptchaRecognizer, string) {
		if hasNative() {
			if r := NewNativeDdddOcrRecognizer(); r != nil {
				return r, "单二进制内置原生 ddddocr（免 Python / 免 API 密钥，5~10ms 极速推理）"
			}
		}
		if hasLocal() {
			return NewLocalDdddOcrRecognizer(""), "本地 Python ddddocr（无 API 密钥）"
		}
		return nil, ""
	}

	if cfg.CaptchaEngine == "ddddocr" {
		if l, note := local(); l != nil {
			return EngineResolution{l, "ddddocr", note}
		}
		if !cfg.CaptchaFallback {
			return EngineResolution{nil, "none",
				"配置为 ddddocr 但本机无内置模型且无 Python/ddddocr；引擎兜底已关闭，识别不可用（请安装 ddddocr 或改选 Vision）"}
		}
		if v := vision(); v != nil {
			return EngineResolution{v, "vision", "配置为 ddddocr 但本机不可用，按兜底开关回退 Vision"}
		}
		return EngineResolution{nil, "none",
			"配置为 ddddocr 但本机不可用，且 Vision 未配置密钥；识别不可用"}
	}

	// 配置为 vision（值域已由 handleAdminConfig 校验，其余值一律按 vision 处理）
	if v := vision(); v != nil {
		return EngineResolution{v, "vision", "OpenAI 兼容视觉 API 云识别"}
	}
	if !cfg.CaptchaFallback {
		return EngineResolution{nil, "none",
			"配置为 Vision 但未填 API 密钥；引擎兜底已关闭，识别不可用（请填写密钥或改选 ddddocr）"}
	}
	if l, note := local(); l != nil {
		return EngineResolution{l, "ddddocr", "配置为 Vision 但未配置密钥，按兜底开关回退" + note}
	}
	return EngineResolution{nil, "none", "配置为 Vision 但未配置密钥，且本机无 ddddocr；识别不可用"}
}