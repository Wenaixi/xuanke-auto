package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
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

// TestActivationCodesDefaultOff 激活码机制默认关闭：未设置 XUANKE_ACTIVATION 时
// ActivationCodesEnabled 必须为 false（本地双击 exe 开箱即用，账号登录直接进系统）；
// 仅显式 XUANKE_ACTIVATION=on 才启用（公网分发场景）。默认配置改为不激活。
func TestActivationCodesDefaultOff(t *testing.T) {
	// 未设置 → 默认关闭
	t.Setenv("XUANKE_ACTIVATION", "")
	cfg := Load()
	if cfg.ActivationCodesEnabled {
		t.Fatal("默认（未设置 XUANKE_ACTIVATION）激活码机制必须关闭，账号登录直接进系统")
	}
	// 显式 on → 启用
	t.Setenv("XUANKE_ACTIVATION", "on")
	if cfg := Load(); !cfg.ActivationCodesEnabled {
		t.Fatal("XUANKE_ACTIVATION=on 必须启用激活码机制")
	}
	// 显式 off → 关闭
	t.Setenv("XUANKE_ACTIVATION", "off")
	if cfg := Load(); cfg.ActivationCodesEnabled {
		t.Fatal("XUANKE_ACTIVATION=off 必须关闭激活码机制")
	}
}

// TestCaptchaEngineDefaultDdddocr 识别引擎默认 ddddocr：未设置 XUANKE_CAPTCHA_ENGINE
// 时 CaptchaEngineDefault 必须返回 ddddocr（本地免密钥、开箱即用）；显式
// vision/dddddocr 时才按配置返回。
func TestCaptchaEngineDefaultDdddocr(t *testing.T) {
	t.Setenv("XUANKE_CAPTCHA_ENGINE", "")
	if got := CaptchaEngineDefault(); got != "ddddocr" {
		t.Fatalf("默认识别引擎应为 ddddocr，实际 %q", got)
	}
	t.Setenv("XUANKE_CAPTCHA_ENGINE", "vision")
	if got := CaptchaEngineDefault(); got != "vision" {
		t.Fatalf("显式 vision 应返回 vision，实际 %q", got)
	}
	t.Setenv("XUANKE_CAPTCHA_ENGINE", "ddddocr")
	if got := CaptchaEngineDefault(); got != "ddddocr" {
		t.Fatalf("显式 ddddocr 应返回 ddddocr，实际 %q", got)
	}
	// 兜底开关是运行时可配置项（管理员后台热配置并落库），配置层不读 env——
	// 留注释防止后来人误加 XUANKE_CAPTCHA_FALLBACK env 读取（开关唯一事实源=落库配置）。
}

// resetProfileState 复位平台注入与数据目录的包级状态（测试隔离）。
// 同包测试直接改写私有包级变量：不为此新增导出复位 API（包内机制不进 interface 面）。
func resetProfileState(t *testing.T) {
	t.Helper()
	platform.Store(nil)
	forcedDataDir = atomic.Value{}
	t.Cleanup(func() {
		platform.Store(nil)
		forcedDataDir = atomic.Value{}
	})
}

// TestPlatformProfileWritesFixedAuthToEnvFile APK 形态：注入内置 profile 后，
// 管理员账密固定、只监听回环，且账密写进 data/.env（文件内容与生效值必须一致）。
func TestPlatformProfileWritesFixedAuthToEnvFile(t *testing.T) {
	resetProfileState(t)
	t.Setenv("XUANKE_ADMIN_TOKEN", "")
	t.Setenv("XUANKE_ADMIN_NAME", "")
	root := t.TempDir()
	SetDataDirForPlatform(root)
	SetPlatformProfileForPlatform("admin", "admin123")

	cfg := Load()
	if cfg.AdminName != "admin" || cfg.AdminToken != "admin123" || cfg.ListenHost != "127.0.0.1" {
		t.Fatalf("APK 内置配置未生效: %+v", cfg)
	}
	b, err := os.ReadFile(filepath.Join(root, "data", ".env"))
	if err != nil {
		t.Fatalf("APK 首次启动必须写出 data/.env: %v", err)
	}
	if !strings.Contains(string(b), "XUANKE_ADMIN_TOKEN=admin123") ||
		!strings.Contains(string(b), "XUANKE_ADMIN_NAME=admin") {
		t.Fatalf(".env 必须写入固定账密，实际:\n%s", b)
	}
}

// TestPlatformProfileRewritesExistingEnvFileAuth APK 升级安装后 data/.env 仍是
// 旧的随机口令：必须被就地改写为固定账密，其余配置保留。
func TestPlatformProfileRewritesExistingEnvFileAuth(t *testing.T) {
	resetProfileState(t)
	t.Setenv("XUANKE_ADMIN_TOKEN", "")
	t.Setenv("XUANKE_ADMIN_NAME", "")
	root := t.TempDir()
	SetDataDirForPlatform(root)
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := "XUANKE_ADMIN_TOKEN=deadbeefdeadbeefdeadbeef\n" +
		"XUANKE_ADMIN_NAME=someone\n" +
		"SF_API_KEY=sk-keep-me\n"
	if err := os.WriteFile(filepath.Join(root, "data", ".env"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	SetPlatformProfileForPlatform("admin", "admin123")

	cfg := Load()
	if cfg.AdminToken != "admin123" || cfg.AdminName != "admin" {
		t.Fatalf("内置账密必须覆盖 .env 历史值: %+v", cfg)
	}
	b, err := os.ReadFile(filepath.Join(root, "data", ".env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "deadbeef") || strings.Contains(s, "someone") {
		t.Fatalf(".env 不得残留历史账密: %s", s)
	}
	if !strings.Contains(s, "XUANKE_ADMIN_TOKEN=admin123") || !strings.Contains(s, "SF_API_KEY=sk-keep-me") {
		t.Fatalf(".env 应为「固定账密 + 保留其它配置」，实际: %s", s)
	}
}

// TestPlatformProfileBeatsRealEnvironment APK 上即便真实环境变量已设口令，
// 内置值仍必须胜出（「固定为 admin/admin123」是无条件语义）。
func TestPlatformProfileBeatsRealEnvironment(t *testing.T) {
	resetProfileState(t)
	t.Setenv("XUANKE_ADMIN_TOKEN", "env-token-should-lose")
	t.Setenv("XUANKE_ADMIN_NAME", "")
	SetDataDirForPlatform(t.TempDir())
	SetPlatformProfileForPlatform("admin", "admin123")

	if cfg := Load(); cfg.AdminToken != "admin123" || cfg.AdminName != "admin" {
		t.Fatalf("APK 内置账密必须无条件优于环境变量: %+v", cfg)
	}
}

// TestNoPlatformProfileKeepsDesktopBehavior 桌面/服务器未注入 profile 时：
// 环境变量口令照用、ListenHost 为空（全接口监听）、且不因本次改动写 .env。
func TestNoPlatformProfileKeepsDesktopBehavior(t *testing.T) {
	resetProfileState(t)
	t.Setenv("XUANKE_ADMIN_TOKEN", "desktop-token")
	t.Setenv("XUANKE_ADMIN_NAME", "")
	root := t.TempDir()
	SetDataDirForPlatform(root)

	cfg := Load()
	if cfg.AdminToken != "desktop-token" || cfg.AdminName != "" || cfg.ListenHost != "127.0.0.1" {
		t.Fatalf("桌面形态不得被 APK 内置配置污染（监听默认只绑回环）: %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, "data", ".env")); !os.IsNotExist(err) {
		t.Fatal("管理口令来自真实环境变量时不得写 .env（保持原行为）")
	}
}

// TestListenHostConfigurable 监听地址对桌面与 APK 是同一套 env 语义：
// 默认 127.0.0.1（只允许本机），填内网 IP 开局域网、填域名走穿透/公网、
// 填 0.0.0.0 等价旧版的全接口监听。
func TestListenHostConfigurable(t *testing.T) {
	resetProfileState(t)
	t.Setenv("XUANKE_ADMIN_TOKEN", "tok")
	t.Setenv("XUANKE_LISTEN_HOST", "")
	SetDataDirForPlatform(t.TempDir())

	if cfg := Load(); cfg.ListenHost != "127.0.0.1" {
		t.Fatalf("默认监听地址必须是 127.0.0.1（仅本机），实际 %q", cfg.ListenHost)
	}
	for _, host := range []string{"192.168.1.10", "xuanke.example.com", "0.0.0.0"} {
		t.Setenv("XUANKE_LISTEN_HOST", host)
		if cfg := Load(); cfg.ListenHost != host {
			t.Fatalf("XUANKE_LISTEN_HOST=%s 必须生效，实际 %q", host, cfg.ListenHost)
		}
	}
}
