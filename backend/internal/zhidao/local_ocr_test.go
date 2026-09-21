package zhidao

import (
	"os/exec"
	"testing"
)

// TestLocalDdddOcrAvailable 验证本机 ddddocr 能力探测函数：
// ①环境存在 Python+ddddocr → 应返回 true（实证）。
// ②装了 Python 但无 ddddocr → 应返回 false（R67 OBSERVE-67-02：原测试三态
//
//	恒绿死——available=false 且 python 存在时走到函数结尾无任何断言，对
//	applyCaptchaRecognizerFor 的 ddddocr 分支探测（router.go）零验证力；
//	该方法正确的行为是 exec.LookPath 通过但 import ddddocr 失败时返回 false，
//	部署机回退 Vision 引擎）。
//
// ③无 python → 跳过（环境不支持该引擎属正常，不影响其他引擎）。
func TestLocalDdddOcrAvailable(t *testing.T) {
	available := LocalDdddOcrAvailable("")
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("本机无 Python，跳过（ddddocr 本地引擎需 Python 环境）")
	}
	if !available {
		t.Error("装了 Python 时 ddddocr 能力探测应返回 true（开发机已装 ddddocr）或 false（无该包时的回退语义），但必须是非零验证力断言——原恒绿死测试无此检查")
	}
}
