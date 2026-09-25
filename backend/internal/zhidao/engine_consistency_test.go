package zhidao

import "testing"

// TestNewDoesNotSilentlyBuildVision New 不再静默自建 Vision 引擎（C5-2 配置缺陷根治）——
// 识别引擎唯一注入通道 = SetRecognizer / SetVision（accounts.Manager 按运行时配置显式注入）。
// 此前 New 在 recognizer==nil 且 APIKey 非空时静默 NewVisionRecognizer：管理员配置
// ddddocr+兜底关（本机无引擎=识别不可用）时，新建客户端仍经此旁路全走 Vision（静默计费）。
func TestNewDoesNotSilentlyBuildVision(t *testing.T) {
	c := New("http://x", VisionConfig{BaseURL: "http://x", APIKey: "sk-x", Model: "m"})
	if c.CurrentRecognizer() != nil {
		t.Fatalf("New 不应静默自建 Vision 引擎（engine 只由 SetRecognizer 注入），got %T", c.CurrentRecognizer())
	}
	if c.loginEngine.CurrentRecognizerExportedForTest() != nil {
		t.Fatal("loginEngine 同样不应静默自建引擎")
	}
}

// TestSetRecognizerPropagatesToLoginEngine SetRecognizer 双驱动登录引擎（C5-4）——
// 否则 SetRecognizer（accounts.Manager 热切换）后登录引擎的识别器恒 nil，
// 新账号登录识别直接报"验证码识别器未初始化"。
func TestSetRecognizerPropagatesToLoginEngine(t *testing.T) {
	c := New("http://x", VisionConfig{})
	fr := fixedRecognizer("0000")
	c.SetRecognizer(fr)
	if got := c.loginEngine.recognizer; got != fr {
		t.Fatalf("SetRecognizer 应驱动登录引擎（指针同一性），got %v want %v", got, fr)
	}
	if got := c.loginEngine.Vision.Recognizer(); got != fr {
		t.Fatalf("登录引擎 VisionConfig 模板也应带识别引擎，got %v", got)
	}
}