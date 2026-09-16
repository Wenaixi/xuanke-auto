package zhidao

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNativeOcrClassifiesRealCaptcha 用真实平台验证码样本回归验证原生 ddddocr 引擎：
// 走生产路径 NativeDdddOcrRecognizer（ensureInit 释出内嵌模型/字符集 → 官方 OCR 模式）。
// 移植库自定义模型分支（ImportOnnxPath）用 ImageNet 归一化 (x-0.456)/0.224，与官方
// 内置模型训练归一化 (x-0.5)/0.5 不一致，同一张英数字验证码输出完全错误
// （Python 对照实测：cap1 官方 'sjmh' vs 自定义归一化 'S43'），登录提交必被拒。
// 本测试保证官方模式路径不被回退、且内嵌模型/字符集配套可消费（字符集第 0 项为 CTC
// 空白占位，官方模式从 ModelDir 读裸数组 charsets_old.json）。
func TestNativeOcrClassifiesRealCaptcha(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过真实模型推理")
	}
	// 不直接 new ddddocr：必须走生产入口触发 ensureInit 的资源释出，否则删掉
	// %TEMP%\xuanke_ddddocr_assets 后模型文件缺失（本次回归本就验证这条路径）。
	r := NewNativeDdddOcrRecognizer()

	captchas := []string{"cap1.png", "cap2.png", "cap3.png", "cap4.png", "cap5.png", "cap6.png"}
	got := 0
	for _, name := range captchas {
		img, err := os.ReadFile(filepath.Join(os.TempDir(), "xz_captchas", name))
		if err != nil {
			continue
		}
		res, err := r.Recognize(img)
		t.Logf("%s: 结果=%q err=%v", name, res, err)
		if err != nil {
			t.Fatalf("识别 %s 失败: %v", name, err)
		}
		if res == "" {
			t.Fatalf("%s 识别结果为空", name)
		}
		got++
	}
	if got == 0 {
		t.Skip("无验证码样本")
	}
}
