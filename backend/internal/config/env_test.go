package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDotEnvKeepsHashInValue 口令值中的合法 # 字符不得被截断：
// 旧版按首个 # 截断会把口令截成前半段；修复后仅去掉尾部空白。
// 注意：loadDotEnv 只回填空环境变量（真实环境变量优先）——本测试预置的环境变量
// 必须显式清空，否则 .env 文件的值永远不会被采用。
func TestLoadDotEnvKeepsHashInValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	// 用一个绝不存在的环境变量名，并显式清空，保证 .env 文件的值会被回填
	const key = "XUANKE_TEST_HASH_KEEP"
	t.Setenv(key, "")
	_ = os.WriteFile(path, []byte(key+"=p@ss#word # 合法井号保留\n"), 0o600)
	loadDotEnv(path)
	if got := os.Getenv(key); got != "p@ss#word # 合法井号保留" {
		t.Fatalf("口令中的 # 应保留，实际 %q", got)
	}
}
