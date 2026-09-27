package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// randomAdminToken 生成 24 位十六进制随机管理口令（首次运行自动生成，用户可在 .env 修改）。
// crypto/rand 失败（熵源故障）时拒绝启动：宁可显式报错也不接受可预测兜底口令（评审项）。
func randomAdminToken() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand 不可用，无法生成安全的管理口令，拒绝启动")
	}
	return hex.EncodeToString(b)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Config 应用配置：环境变量 + data/.env 配置文件，其余为编译期常量。
// 安全策略：SF_API_KEY / XUANKE_ADMIN_TOKEN / XUANKE_MASTER_KEY 等敏感项来自
// 真实环境变量或 data/.env 文件，代码内不含任何硬编码密钥。
type Config struct {
	Port    string // HTTP 监听端口
	DBPath  string // SQLite 数据库路径
	BaseURL string // 至道平台根地址
	// OpenAI 兼容视觉 API 验证码识别配置（登录必需；默认指向一个 OpenAI 兼容服务地址，可改任意兼容服务）
	SFBaseURL string
	SFAPIKey  string
	SFModel   string
	// AdminToken 管理口令（用于生成激活码；缺失拒绝启动）
	AdminToken string
	// AdminName 管理员账号名（默认 admin）
	AdminName string
	// ListenHost 监听主机（默认 127.0.0.1，只允许本机访问）。XUANKE_LISTEN_HOST
	// 可覆盖：局域网填本机内网 IP（如 192.168.1.10），穿透/公网填域名或 0.0.0.0
	// （所有网卡，等价旧版的全接口监听）。
	ListenHost string
	// PlatformEmbedded 平台内置形态（APK）：管理账密由 profile 固定、端口不可改
	// （Java 壳按 3091 加载页面）。桌面/服务器恒 false。
	PlatformEmbedded bool
	// ActivationCodesEnabled 激活码机制开关（XUANKE_ACTIVATION，默认 off；on 才启用激活码）
	// 默认关闭：本地双击 exe 开箱即用（账号登录直接进系统），公网分发才显式开启激活码。
	ActivationCodesEnabled bool
}

// Load 从环境变量与 data/.env 文件组装配置；首次运行自动生成全部配置。
// 数据目录固定为可执行文件同目录下的 data/（data/.env 含管理员账密/激活码开关等）。
// 首次运行没有 data/.env 时自动写入随机管理员账密与默认配置，之后每次读取。
func Load() Config {
	prof := platform.Load()
	envPath := filepath.Join(dataDir(), ".env")
	ensureEnvFile(envPath, prof)
	loadDotEnv(envPath)
	// 开放时间唯一事实源 = 平台 beginTimes 自动识别（scheduler 层），配置层不再注入，
	// 也不读取 XUANKE_OPEN_TIME 环境变量——旧文档的硬编码 2026 默认值已整体移除，
	// 避免"识别槽为空时把过期日期当开窗点"的误导。
	dbPath := envOr("XUANKE_DB", filepath.Join(dataDir(), "xuanke.db"))
	cfg := Config{
		Port:                   envOr("XUANKE_PORT", "3091"),
		DBPath:                 dbPath,
		BaseURL:                "https://www.zhidao.fj.cn",
		SFBaseURL:              envOr("SF_BASE_URL", "https://api.siliconflow.cn/v1"),
		SFAPIKey:               os.Getenv("SF_API_KEY"),
		SFModel:                envOr("SF_MODEL", "Qwen/Qwen3-VL-30B-A3B-Instruct"),
		AdminToken:             os.Getenv("XUANKE_ADMIN_TOKEN"),
		AdminName:              os.Getenv("XUANKE_ADMIN_NAME"),
		// 默认只绑回环：要开局域网就显式填本机内网 IP，要公网/穿透就填域名
		// 或 0.0.0.0（所有网卡）。
		ListenHost:             envOr("XUANKE_LISTEN_HOST", "127.0.0.1"),
		ActivationCodesEnabled: os.Getenv("XUANKE_ACTIVATION") == "on",
	}
	if prof != nil {
		// APK 内置账密最后覆盖：.env / 环境变量里的历史值一律让位
		// （监听地址不覆盖——它与桌面共用同一套 XUANKE_LISTEN_HOST 语义）。
		cfg.AdminName = prof.AdminName
		cfg.AdminToken = prof.AdminToken
		cfg.PlatformEmbedded = true
	}
	return cfg
}

// loadDotEnv 读取 data/.env 的键值对回填环境变量（真实环境变量优先，文件兜底）。
// 这样 .env 既是“配置持久化”也是“当前生效值”，双击 exe 无需任何外部环境。
func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if os.Getenv(key) == "" {
			// 不再按 # 截断值——口令/密钥中合法 # 会被截断破坏。
			// 旧版"行内注释截断"只服务于模板注释（# 开头行已被上方整行跳过）；
			// 真实值里出现 # 属于合法字符，宁可保留也不破坏凭据。
			// "仅回填空值"语义：XUANKE_MASTER_KEY 在 .env 里会被 loadDotEnv
			// 正确回填，secure.LoadOrCreateKey 照常读取——真实环境变量恒优先，
			// 且 l main 启动单线程时序调用一次，无运行时覆盖竞态。
			_ = os.Setenv(key, strings.TrimSpace(val))
		}
	}
}

// CaptchaEngineDefault 默认验证码识别引擎。
// 默认 ddddocr 本地识别（免 API 密钥、无外网依赖，双击 exe 开箱即用）；需云识别
// 时显式设 XUANKE_CAPTCHA_ENGINE=vision 并填 SF_API_KEY（默认改 ddddocr）。
// 说明：ddddocr 引擎可用性依赖本机 Python + ddddocr 包（或内嵌模型）。默认不回退：
// 配置的引擎不可用即识别不可用；需双向兜底请在管理员后台开启"引擎兜底"开关。
// 开关本身由管理员后台热配置并落库（runtime.Config.CaptchaFallback），这里不读 env。
func CaptchaEngineDefault() string {
	if v := os.Getenv("XUANKE_CAPTCHA_ENGINE"); v == "vision" || v == "ddddocr" {
		return v
	}
	return "ddddocr"
}

// dataDir 数据目录：可执行文件同目录下的 data/（保证双击 exe 即可用，不依赖 cwd）。
func dataDir() string {
	// Android 平台入口先调 SetDataDirForPlatform 注入 filesDir（os.Executable 在
	// APK 内不可靠：返回安装路径无写权限），见 platform_android.go 的 JNI 入口。
	if forced := forcedDataDir.Load(); forced != nil {
		return filepath.Join(forced.(string), "data")
	}
	exe, err := os.Executable()
	if err != nil {
		return "data"
	}
	dir := filepath.Dir(exe)
	// 开发态（go run / 仓库内启动）时 exe 位于临时目录，回退仓库根 data/ 便于联调
	if strings.Contains(dir, "TEMP") || strings.Contains(dir, "tmp") || strings.Contains(dir, "go-build") {
		// 优先 cwd 下的 data/（仓库根），与旧版行为一致
		if st, err := os.Stat(filepath.Join(".", "data")); err == nil && st.IsDir() {
			return filepath.Join(".", "data")
		}
		return "data"
	}
	return filepath.Join(dir, "data")
}

// forcedDataDir 平台强制数据目录（安卓 filesDir 注入；桌面恒 nil）。
var forcedDataDir atomic.Value

// SetDataDirForPlatform 由平台入口强制数据根目录（成为 data/ 的父目录）：
// Android 上 Java 壳先调 XuankeSetDataDir(filesDir) 注入，config.Load 的 dataDir
// 即变成 <filesDir>/data（App 沙箱内可读写）。桌面/服务器不调用，保持现有
// "可执行文件同目录 data/" 语义。注入必须在 config.Load 之前（运行时单点）。
func SetDataDirForPlatform(dir string) {
	forcedDataDir.Store(dir)
}

// platformProfile 平台内置配置（APK 专属）：管理员账密随包固定。
// 桌面/服务器不注入（nil），一切行为与既有版本完全一致。
type platformProfile struct {
	AdminName  string
	AdminToken string
}

// platform 平台内置配置（Android JNI 入口在 config.Load 之前注入；桌面恒 nil）。
var platform atomic.Pointer[platformProfile]

// SetPlatformProfileForPlatform 由平台入口强制管理员账密：APK 侧载场景下用户
// 无需任何 .env 配置即可进管理页（桌面/服务器不调用，保持「随机口令」语义；
// 与 SetDataDirForPlatform 同一套平台注入模式）。
// 监听地址不在这里固定——它与桌面同一套 XUANKE_LISTEN_HOST 语义（默认 127.0.0.1，
// 需要局域网/公网时改 .env 即可）。
func SetPlatformProfileForPlatform(name, token string) {
	platform.Store(&platformProfile{AdminName: name, AdminToken: token})
}

// WritableDir 返回应用私有、可写的资源释出目录（供内嵌资源落地磁盘）。
//
// **Android 上绝不能用 os.TempDir()**：其返回 /data/local/tmp（系统目录，
// 普通 app 无写权限，实测 TMPDIR 环境变量即指向此处），fallback 的 /tmp 属
// shell 用户同样不可写——两者都会让「创建资源目录失败」。故 Android 走
// SetDataDirForPlatform 注入的 filesDir/data（App 沙箱内可读写）；
// 未注入时（理论上不该发生）退回 "." 交由调用方报错而非静默写到系统目录。
// 桌面/服务器不注入，os.TempDir() 语义正确，保持原样。
func WritableDir() string {
	if forced := forcedDataDir.Load(); forced != nil {
		return filepath.Join(forced.(string), "data")
	}
	return os.TempDir()
}

// ensureEnvFile 确保 .env 存在：prof 为 nil（桌面/服务器）时不存在则自动生成
// 随机管理员口令与默认配置；prof 非 nil（APK）时写入随包固定的账密，并在已有
// .env 上就地改写账密两键——保证「文件里写的」与「实际生效的」永远一致
// （否则用户按 .env 里的随机口令登录必失败，排障被彻底误导）。
func ensureEnvFile(path string, prof *platformProfile) {
	if prof == nil && os.Getenv("XUANKE_ADMIN_TOKEN") != "" {
		return // 真实环境变量已提供管理口令，无需写文件
	}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if prof != nil {
			_ = os.WriteFile(path, []byte(rewriteAuthLines(string(b), prof)), 0o600)
		}
		return // 已有配置
	}
	admin := randomAdminToken()
	nameLine := "# XUANKE_ADMIN_NAME=admin"
	if prof != nil {
		admin = prof.AdminToken
		nameLine = "XUANKE_ADMIN_NAME=" + prof.AdminName
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	tpl := `# 至道选课自动化 - 环境配置文件（首次运行自动生成）
# 管理员登录账号（可选，默认 admin；改成任意名字即为管理员登录账号）
` + nameLine + `
# 管理员口令（首次运行自动生成；删除本行后重启可重新生成随机口令）
XUANKE_ADMIN_TOKEN=` + admin + `

# 教务登录验证码识别密钥（可选留空；默认识别引擎 ddddocr 免密钥，vision 云识别才需填）
SF_API_KEY=
# 识别引擎（ddddocr=默认，本地免密钥无外网；vision=OpenAI 兼容视觉 API，需填 SF_API_KEY）
XUANKE_CAPTCHA_ENGINE=ddddocr
# 识别引擎兜底开关（默认不回退：ddddocr 与 vision 严格互不兜底；如需双向兜底请在管理员后台开启）

# 激活码机制开关：on=启用激活码（分发用）；默认关闭（off），本地双击 exe 账号登录直接进入系统
XUANKE_ACTIVATION=off

# 服务端口与数据库路径（可选）
# XUANKE_PORT=3091
# XUANKE_DB=data/xuanke.db

# 监听地址（默认 127.0.0.1，只允许本机访问）
# 局域网：填本机内网 IP（如 192.168.1.10）；穿透/公网：填域名，或 0.0.0.0（所有网卡）
# XUANKE_LISTEN_HOST=127.0.0.1
`
	_ = os.WriteFile(path, []byte(tpl), 0o600)
}

// rewriteAuthLines 就地改写 .env 的管理员账密两键：先删掉两键的全部旧行
// （含注释行），再把内置值前置，其余配置原样保留。
func rewriteAuthLines(old string, prof *platformProfile) string {
	var keep []string
	for _, line := range strings.Split(old, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "XUANKE_ADMIN_TOKEN=") ||
			strings.HasPrefix(t, "XUANKE_ADMIN_NAME=") ||
			strings.HasPrefix(t, "# XUANKE_ADMIN_TOKEN=") ||
			strings.HasPrefix(t, "# XUANKE_ADMIN_NAME=") {
			continue
		}
		keep = append(keep, line)
	}
	head := "# 管理员账号与口令（APK 随包固定；桌面版不受影响）\n" +
		"XUANKE_ADMIN_NAME=" + prof.AdminName + "\n" +
		"XUANKE_ADMIN_TOKEN=" + prof.AdminToken + "\n"
	return head + strings.TrimLeft(strings.Join(keep, "\n"), "\n")
}
