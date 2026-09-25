package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestOpenOnReadonlyPath 数据库文件不可写时 Open 必须返回错误而非静默成功。
// 平台差异：Windows 无 POSIX 目录只读位（chmod 0o000 不阻止建文件），用"路径含
// NUL 保留设备名"构造必非法路径——任何平台都让 MkdirAll/Open 失败；Linux 用
// t.TempDir + chmod 0o000 真只读。两分支共用"err != nil"同一断言。
func TestOpenOnReadonlyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows 保留设备路径 NUL 在任意段被内核拒绝，MkdirAll/Open 必失败。
		if _, err := Open(`C:\nul\nul\test.db`); err == nil {
			t.Fatal("非法数据库路径应返回错误，而不是静默成功")
		}
		return
	}
	// Linux/macOS：真只读目录（chmod 0o000）→ 建库被拒
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Skipf("无法设为只读目录，跳过: %v", err)
	}
	defer os.Chmod(dir, 0o755) // 还原，让 t.TempDir 清理可删

	dbPath := filepath.Join(dir, "test.db")
	if _, err := Open(dbPath); err == nil {
		t.Fatal("只读目录下打开数据库应返回错误，而不是静默成功")
	}
}
