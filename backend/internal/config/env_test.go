package config

import (
	"os"
	"path/filepath"
	"reflect"
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

// TestConfigDoesNotInjectOpenTime 开放时间唯一事实源 = 平台 beginTimes 自动识别
// （scheduler 层），配置层不再注入任何默认值——XUANKE_OPEN_TIME 环境变量与硬编码
// 2026 日期残留都是维护陷阱：运维按旧文档配置会产生"识别槽为空时把 2026-09-13
// 当开窗点"的误导。回归守护：Config 结构体不出现 OpenTime 字段，环境变量不被读取。
func TestConfigDoesNotInjectOpenTime(t *testing.T) {
	if _, ok := reflect.TypeOf(Config{}).FieldByName("OpenTime"); ok {
		t.Fatal("Config 不得再含 OpenTime 字段：开放时间由平台 beginTimes 自动识别，配置层注入是死配置+过期日期残留")
	}
	// 即便运维按旧文档设置了环境变量，也不得出现在 Load 返回值中（Load 不再消费它）
	t.Setenv("XUANKE_OPEN_TIME", "2099-01-01 00:00:00")
	cfg := Load()
	rt := reflect.TypeOf(cfg)
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Name == "OpenTime" {
			t.Fatal("Load 返回的配置不得含 OpenTime 字段")
		}
	}
}
