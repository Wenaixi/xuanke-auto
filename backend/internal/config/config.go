package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// randomAdminToken 生成 24 位十六进制随机管理口令（首次运行自动生成，用户可在 .env 修改）。
// crypto/rand 失败（熵源故障）时拒绝启动：宁可显式报错也不接受可预测兜底口令（评审 MINOR 4）。
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
	// 硅基流动 Vision 验证码识别配置（登录必需）
	SFBaseURL string
	SFAPIKey  string
	SFModel   string
	// AdminToken 管理口令（用于生成激活码；缺失拒绝启动）
	AdminToken string
	// AdminName 管理员账号名（默认 admin）
	AdminName string
	// ActivationCodesEnabled 激活码机制开关（XUANKE_ACTIVATION，默认 off；on 才启用激活码）
	// 默认关闭：本地双击 exe 开箱即用（账号登录直接进系统），公网分发才显式开启激活码。
	ActivationCodesEnabled bool
}

// Load 从环境变量与 data/.env 文件组装配置；首次运行自动生成全部配置。
// 数据目录固定为可执行文件同目录下的 data/（data/.env 含管理员账密/激活码开关等）。
// 首次运行没有 data/.env 时自动写入随机管理员账密与默认配置，之后每次读取。
func Load() Config {
	envPath := filepath.Join(dataDir(), ".env")
	ensureEnvFile(envPath)
	loadDotEnv(envPath)
	// 开放时间唯一事实源 = 平台 beginTimes 自动识别（scheduler 层），配置层不再注入，
	// 也不读取 XUANKE_OPEN_TIME 环境变量——旧文档的硬编码 2026 默认值已整体移除，
	// 避免"识别槽为空时把过期日期当开窗点"的误导。
	dbPath := envOr("XUANKE_DB", filepath.Join(dataDir(), "xuanke.db"))
	return Config{
		Port:                   envOr("XUANKE_PORT", "3091"),
		DBPath:                 dbPath,
		BaseURL:                "https://www.zhidao.fj.cn",
		SFBaseURL:              envOr("SF_BASE_URL", "https://api.siliconflow.cn/v1"),
		SFAPIKey:               os.Getenv("SF_API_KEY"),
		SFModel:                envOr("SF_MODEL", "Qwen/Qwen3-VL-30B-A3B-Instruct"),
		AdminToken:             os.Getenv("XUANKE_ADMIN_TOKEN"),
		AdminName:              os.Getenv("XUANKE_ADMIN_NAME"),
		ActivationCodesEnabled: os.Getenv("XUANKE_ACTIVATION") == "on",
	}
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
			// n5 修复：不再按 # 截断值——口令/密钥中合法 # 会被截断破坏。
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
// 时显式设 XUANKE_CAPTCHA_ENGINE=vision 并填 SF_API_KEY（R71 需求：默认改 ddddocr）。
// 说明：ddddocr 引擎可用性依赖本机 Python + ddddocr 包（或内嵌模型），未装时
// 登录会报识别引擎不可用——管理员可在后台热切换回 vision。
func CaptchaEngineDefault() string {
	if v := os.Getenv("XUANKE_CAPTCHA_ENGINE"); v == "vision" || v == "ddddocr" {
		return v
	}
	return "ddddocr"
}

// dataDir 数据目录：可执行文件同目录下的 data/（保证双击 exe 即可用，不依赖 cwd）。
func dataDir() string {
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

// ensureEnvFile 确保 .env 存在：不存在则自动生成随机管理员口令与默认配置。
// 已存在（含真实环境变量）一律不改动，保证冰封可复现配置。
func ensureEnvFile(path string) {
	if os.Getenv("XUANKE_ADMIN_TOKEN") != "" {
		return // 真实环境变量已提供管理口令，无需写文件
	}
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return // 已有配置
	}
	admin := randomAdminToken()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	tpl := `# 至道选课自动化 - 环境配置文件（首次运行自动生成）
# 管理员登录账号（可选，默认 admin；改成任意名字即为管理员登录账号）
# XUANKE_ADMIN_NAME=admin
# 管理员口令（首次运行自动生成；删除本行后重启可重新生成随机口令）
XUANKE_ADMIN_TOKEN=` + admin + `

# 教务登录验证码识别密钥（可选留空；默认识别引擎 vision 才需要密钥，ddddocr 免密钥）
SF_API_KEY=
# 识别引擎（ddddocr=默认，本地免密钥无外网；vision=硅基流动云识别，需填 SF_API_KEY）
XUANKE_CAPTCHA_ENGINE=ddddocr

# 激活码机制开关：on=启用激活码（分发用）；默认关闭（off），本地双击 exe 账号登录直接进入系统
XUANKE_ACTIVATION=off

# 服务端口与数据库路径（可选）
# XUANKE_PORT=3091
# XUANKE_DB=data/xuanke.db
`
	_ = os.WriteFile(path, []byte(tpl), 0o600)
}
