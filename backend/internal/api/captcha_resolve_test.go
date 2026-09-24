package api

import (
	"reflect"
	"strings"
	"testing"

	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/zhidao"
)

// captchaProbeSpy 探测函数探针：记录是否被调用（探测成本契约断言用）。
type captchaProbeSpy struct {
	nativeCalled int
	localCalled  int
	native       bool
	local        bool
}

func (s *captchaProbeSpy) hasNative() bool { s.nativeCalled++; return s.native }
func (s *captchaProbeSpy) hasLocal() bool  { s.localCalled++; return s.local }

// TestResolveCaptchaRecognizer 引擎解析纯函数表驱动测试。
//
// 覆盖核心契约（引擎兜底开关默认关闭 → 两引擎严格互不回退）：
//   - ddddocr 本机不可用 + 开关关闭 → 无引擎（绝不静默换云端计费）
//   - ddddocr 本机不可用 + 开关开启 → 回退 Vision
//   - Vision 无密钥 + 开关关闭 → 无引擎（绝不静默换本地）
//   - Vision 无密钥 + 开关开启 → 回退本机 ddddocr
//   - 探测成本：vision 且开关关闭 / vision 且已有密钥时，绝不触发本地 Python 探测
func TestResolveCaptchaRecognizer(t *testing.T) {
	cases := []struct {
		name          string
		engine        string
		fallback      bool
		apiKey        string
		native, local bool
		wantEngine    string
		wantNil       bool
		// wantNative 断言识别器具体类型（ddddocr 双实现区分）
		wantNative bool
		// wantLocalProbe / wantNativeProbe 断言探测是否被调用（-1=不关心）
		wantNativeProbe int
		wantLocalProbe  int
	}{
		{
			name: "ddddocr_内置原生可用", engine: "ddddocr", native: true, local: false,
			wantEngine: "ddddocr", wantNative: true, wantNativeProbe: 1, wantLocalProbe: 0,
		},
		{
			name: "ddddocr_仅本机Python可用", engine: "ddddocr", native: false, local: true,
			wantEngine: "ddddocr", wantNative: false, wantNativeProbe: 1, wantLocalProbe: 1,
		},
		{
			name:   "ddddocr_本机不可用_开关关闭_有密钥_仍无引擎",
			engine: "ddddocr", fallback: false, apiKey: "sk-x", native: false, local: false,
			wantNil: true, wantEngine: "none", wantNativeProbe: 1, wantLocalProbe: 1,
		},
		{
			name:   "ddddocr_本机不可用_开关开启_回退Vision",
			engine: "ddddocr", fallback: true, apiKey: "sk-x", native: false, local: false,
			wantEngine: "vision", wantNativeProbe: 1, wantLocalProbe: 1,
		},
		{
			name:   "ddddocr_本机不可用_开关开启_无密钥_无引擎",
			engine: "ddddocr", fallback: true, apiKey: "", native: false, local: false,
			wantNil: true, wantEngine: "none", wantNativeProbe: 1, wantLocalProbe: 1,
		},
		{
			name: "vision_有密钥_开关关闭", engine: "vision", fallback: false, apiKey: "sk-x",
			wantEngine: "vision", wantNativeProbe: 0, wantLocalProbe: 0,
		},
		{
			name: "vision_无密钥_开关关闭_无引擎", engine: "vision", fallback: false, apiKey: "",
			wantNil: true, wantEngine: "none", wantNativeProbe: 0, wantLocalProbe: 0,
		},
		{
			name:   "vision_无密钥_开关开启_回退本机ddddocr",
			engine: "vision", fallback: true, apiKey: "", native: true,
			wantEngine: "ddddocr", wantNative: true, wantNativeProbe: 1, wantLocalProbe: 0,
		},
		{
			name:   "vision_无密钥_开关开启_本机也无ddddocr_无引擎",
			engine: "vision", fallback: true, apiKey: "", native: false, local: false,
			wantNil: true, wantEngine: "none", wantNativeProbe: 1, wantLocalProbe: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 内置原生 ddddocr 是 build-tag 条件能力（仅 Windows+CGO 构建具备）。
			// CGO=0 构建下该分支不可达，跳过而非失败——断言恒真比跳过更糟（假绿）。
			if tc.native && !zhidao.NativeDdddOcrAvailable() {
				t.Skip("本机构建无内置原生 ddddocr（非 Windows+CGO），跳过原生分支")
			}
			spy := &captchaProbeSpy{native: tc.native, local: tc.local}
			cfg := runtime.Config{
				CaptchaEngine:   tc.engine,
				CaptchaFallback: tc.fallback,
				VisionAPIKey:    tc.apiKey,
				VisionBaseURL:   "https://api.example.com/v1",
				VisionModel:     "m",
			}
			got := resolveCaptchaRecognizer(cfg, spy.hasNative, spy.hasLocal)

			if tc.wantNil {
				if got.recognizer != nil {
					t.Fatalf("应无可用引擎（nil），实际 %T", got.recognizer)
				}
			} else if got.recognizer == nil {
				t.Fatalf("应有可用引擎 %s，实际 nil（note=%s）", tc.wantEngine, got.note)
			}
			if got.engine != tc.wantEngine {
				t.Fatalf("实际生效引擎应为 %q，实际 %q", tc.wantEngine, got.engine)
			}
			if got.note == "" {
				t.Fatal("日志说明不得为空（管理员靠它定位引擎来源）")
			}
			// 类型断言：ddddocr 双实现必须区分，避免"配置 ddddocr 却回退 Python"静默。
			// NativeDdddOcrRecognizer 是 build-tag 条件类型（非 Windows/无 CGO 下不存在），
			// 用 reflect 按类型名断言，跨构建环境恒可编译。
			if !tc.wantNil {
				gotType := strings.TrimPrefix(reflect.TypeOf(got.recognizer).String(), "*")
				switch tc.wantEngine {
				case "ddddocr":
					wantType := "zhidao.LocalDdddOcrRecognizer"
					if tc.wantNative {
						wantType = "zhidao.NativeDdddOcrRecognizer"
					}
					if gotType != wantType {
						t.Fatalf("应为 %s，实际 %s", wantType, gotType)
					}
				case "vision":
					if _, ok := got.recognizer.(*zhidao.VisionRecognizer); !ok {
						t.Fatalf("应为 Vision 识别器，实际 %T", got.recognizer)
					}
				}
			}
			// 探测成本契约：本地探测要起 Python 子进程（最坏 15s），
			// 只有确实可能用到本地 ddddocr 时才允许触发。
			if spy.nativeCalled != tc.wantNativeProbe {
				t.Fatalf("原生能力探测调用次数应为 %d，实际 %d", tc.wantNativeProbe, spy.nativeCalled)
			}
			if spy.localCalled != tc.wantLocalProbe {
				t.Fatalf("本机 Python 探测调用次数应为 %d，实际 %d", tc.wantLocalProbe, spy.localCalled)
			}
		})
	}
}

// TestResolveCaptchaRecognizerDefaultNoFallback 默认零值 Config（CaptchaFallback=false）
// 必须表现为"不回退"——这是本特性的核心承诺：默认状态下两引擎互不兜底。
// 用真实默认值构造（不显式赋 CaptchaFallback），防止未来有人把默认值改成 true 而无测试拦截。
func TestResolveCaptchaRecognizerDefaultNoFallback(t *testing.T) {
	cfg := runtime.Config{CaptchaEngine: "ddddocr", VisionAPIKey: "sk-x"}
	if cfg.CaptchaFallback {
		t.Fatal("runtime.Config 零值的 CaptchaFallback 必须为 false（默认不回退）")
	}
	got := resolveCaptchaRecognizer(cfg, func() bool { return false }, func() bool { return false })
	if got.recognizer != nil {
		t.Fatalf("默认配置下 ddddocr 不可用必须无引擎，实际 %T", got.recognizer)
	}
	if !strings.Contains(got.note, "兜底已关闭") {
		t.Fatalf("无引擎日志必须说明是兜底关闭所致（可执行修复指引），实际 %q", got.note)
	}
}

// TestCaptchaActiveEngineTracked 实际生效引擎落地：stats 靠它区分"配置值"与"运行真相"。
func TestCaptchaActiveEngineTracked(t *testing.T) {
	prev := CaptchaActiveEngine()
	t.Cleanup(func() { setCaptchaActiveEngine(prev) })

	setCaptchaActiveEngine("none")
	if got := CaptchaActiveEngine(); got != "none" {
		t.Fatalf("实际引擎应为 none，实际 %q", got)
	}
	setCaptchaActiveEngine("ddddocr")
	if got := CaptchaActiveEngine(); got != "ddddocr" {
		t.Fatalf("实际引擎应为 ddddocr，实际 %q", got)
	}
}
