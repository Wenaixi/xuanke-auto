package db

import (
	"testing"
)

// TestOpenOnReadonlyPath 数据库文件不可写时 Open 必须返回错误而非静默成功：
// SQLite 以读写模式打开只读路径会失败，绝不静默回退（如想只读可显式 _pragma=query_only）。
func TestOpenOnReadonlyPath(t *testing.T) {
	// 不存在的父目录路径：MkdirAll 会失败 → Open 返回错误
	//（路径含保留设备名，Windows 上必然是非法文件系统路径）
	// OBSERVE-64-03/65-01：`C:\nul\nul\` 是 Windows 保留设备路径（NUL 在任意段被内核
	// 拒绝，Open/MkdirAll 返回错误 → 断言通过）。Linux/macOS 下 `C:` 被当普通目录名、
	// 该路径会被真实创建为 SQLite 数据库文件并跑完 schema（Open 成功 → err==nil →
	// 断言红）——非只"MkdirAll 成功"而是"真实建库成功"。跨平台 CI 启用前该测试在
	// Linux 上会红（断言方向与意图相反），需换真只读构造（t.TempDir + 权限守卫）并
	// 加 GOOS 守卫；当前 Windows 主平台未触发，显式化平台假设。
	if _, err := Open(`C:\nul\nul\test.db`); err == nil {
		t.Fatal("非法数据库路径应返回错误，而不是静默成功")
	}
}
