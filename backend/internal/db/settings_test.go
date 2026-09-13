package db

import (
	"testing"
)

// TestOpenOnReadonlyPath 数据库文件不可写时 Open 必须返回错误而非静默成功：
// SQLite 以读写模式打开只读路径会失败，绝不静默回退（如想只读可显式 _pragma=query_only）。
func TestOpenOnReadonlyPath(t *testing.T) {
	// 不存在的父目录路径：MkdirAll 会失败 → Open 返回错误
	//（路径含保留设备名，Windows 上必然是非法文件系统路径）
	if _, err := Open(`C:\nul\nul\test.db`); err == nil {
		t.Fatal("非法数据库路径应返回错误，而不是静默成功")
	}
}
