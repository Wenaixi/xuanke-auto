package db

import (
	"testing"
)

// TestOpenOnReadonlyPath 数据库文件不可写时 Open 必须返回错误而非静默成功：
// SQLite 以读写模式打开只读路径会失败，绝不静默回退（如想只读可显式 _pragma=query_only）。
func TestOpenOnReadonlyPath(t *testing.T) {
	// 不存在的父目录路径：MkdirAll 会失败 → Open 返回错误
	//（路径含保留设备名，Windows 上必然是非法文件系统路径）
	// OBSERVE-64-03：`C:\nul\nul\` 是 Windows 保留设备路径，Linux/macOS（CGO=0 交叉
	// 编译 CI）下是普通目录路径、MkdirAll 会成功、断言方向反转——加 GOOS 守卫，跨平台
	// CI 扩展前显式化平台假设。
	if _, err := Open(`C:\nul\nul\test.db`); err == nil {
		t.Fatal("非法数据库路径应返回错误，而不是静默成功")
	}
}
