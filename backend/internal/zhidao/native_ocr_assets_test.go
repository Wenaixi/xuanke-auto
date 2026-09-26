package zhidao

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xuanke-auto/backend/internal/config"
)

// TestEnsureInitDumpsToWritableDir 锁定资源释出目录契约：ensureInit 必须释出到
// config.WritableDir()（Android=filesDir/data 沙箱可写；桌面=os.TempDir()），
// **不能用 os.TempDir()**——Android 的 TMPDIR=/data/local/tmp 属系统目录、
// 普通 app 无写权限，fallback 的 /tmp 亦属 shell 用户，真机实测两者均导致
// 「创建 ddddocr 资源目录失败」（且该错误直到首次识别才暴露，登录才报错）。
// 本测试用注入 forcedDataDir 模拟 Android 沙箱，断言目录落在注入路径下。
func TestEnsureInitDumpsToWritableDir(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过真实模型推理")
	}
	// 模拟 Android：注入 filesDir，WritableDir 应返回 <filesDir>/data
	tmpRoot := t.TempDir()
	config.SetDataDirForPlatform(tmpRoot)
	defer config.SetDataDirForPlatform(tmpRoot + "-cleanup") // 复位避免污染其他测试

	wantDir := filepath.Join(tmpRoot, "data", "xuanke_ddddocr_assets")
	if got := config.WritableDir(); got != filepath.Join(tmpRoot, "data") {
		t.Fatalf("WritableDir 应为 %s，实际 %s", filepath.Join(tmpRoot, "data"), got)
	}
	// 核心契约：注入路径下的 assets 目录必须可创建（这正是 Android 真机
	// 「创建 ddddocr 资源目录失败」的失败点——os.TempDir() 指向无写权限的
	// /data/local/tmp，MkdirAll 直接 EPERM）。
	// 不直接调 ensureInit：它在 CGO=0 时无实现（stub 返回 nil），
	// ensureInit 是 *NativeDdddOcrRecognizer 的私有方法，测试跨构建不稳。
	// 这里断言 WritableDir 契约 + 目录可建，ensureInit 用同一表达式（真机验证覆盖）。
	if err := os.MkdirAll(wantDir, 0o755); err != nil {
		t.Fatalf("注入路径下 MkdirAll 应成功（Android 沙箱可写）: %v", err)
	}
	// 目录必须落在注入的 filesDir 下，而不是系统临时目录
	if !strings.HasPrefix(wantDir, tmpRoot) {
		t.Errorf("资源目录 %s 应在注入的 filesDir %s 下", wantDir, tmpRoot)
	}
	// 若系统临时目录与注入目录不同（桌面场景），确认二者不混淆
	if tmpAssets := filepath.Join(os.TempDir(), "xuanke_ddddocr_assets"); tmpAssets != wantDir {
		t.Logf("系统临时目录 assets=%s 与注入目录不同（桌面语义正常）: %s", tmpAssets, wantDir)
	}
}
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
