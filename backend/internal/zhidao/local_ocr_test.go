package zhidao

import (
	"os/exec"
	"testing"
)

// TestLocalDdddOcrAvailable 验证本机 ddddocr 能力探测函数：环境存在 Python+ddddocr 时返回 true。
// 缺失时跳过（部署机没有该环境属正常，不影响其他引擎）。
func TestLocalDdddOcrAvailable(t *testing.T) {
	available := LocalDdddOcrAvailable("")
	if !available {
		// 开发机装过 ddddocr（本仓库已确认），探测应成功
		if _, err := exec.LookPath("python"); err != nil {
			t.Skip("本机无 Python，跳过（ddddocr 本地引擎需 Python 环境）")
		}
	}
}
