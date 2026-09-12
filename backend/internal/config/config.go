package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// randomAdminToken 生成 24 位十六进制随机管理口令（首次运行自动生成，用户可在 .env 修改）。
func randomAdminToken() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "xk-admin-change-me-2026"
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
	Port     string // HTTP 监听端口
	DBPath   string // SQLite 数据库路径
	OpenTime string // 选课开放时间（本地时区）
	BaseURL  string // 至道平台根地址
	// 硅基流动 Vision 验证码识别配置（登录必需）
	SFBaseURL string
	SFAPIKey  string
	SFModel   string
	// AdminToken 管理口令（用于生成激活码；缺失拒绝启动）
	AdminToken string
	// AdminName 管理员账号名（默认 admin）
	AdminName string
	// ActivationCodesEnabled 激活码机制开关（XUANKE_ACTIVATION，默认 on；off 完全禁用激活码）
	ActivationCodesEnabled bool
}

// Load 从环境变量与 data/.env 文件组装配置；首次运行自动生成全部配置。
// 数据目录固定为可执行文件同目录下的 data/（data/.env 含管理员账密/激活码开关等）。
// 首次运行没有 data/.env 时自动写入随机管理员账密与默认配置，之后每次读取。
func Load() Config {
	dbPath := envOr("XUANKE_DB", filepath.Join(dataDir(), "xuanke.db"))
	ensureEnvFile(filepath.Join(dataDir(), ".env"))
	return Config{
		Port:       envOr("XUANKE_PORT", "3091"),
		DBPath:     dbPath,
		OpenTime:   envOr("XUANKE_OPEN_TIME", "2026-09-13 09:00:00"),
		BaseURL:    "https://www.zhidao.fj.cn",
		SFBaseURL:  envOr("SF_BASE_URL", "https://api.siliconflow.cn/v1"),
		SFAPIKey:   os.Getenv("SF_API_KEY"),
		SFModel:    envOr("SF_MODEL", "Qwen/Qwen3-VL-30B-A3B-Instruct"),
		AdminToken: os.Getenv("XUANKE_ADMIN_TOKEN"),
		AdminName:  os.Getenv("XUANKE_ADMIN_NAME"),
		ActivationCodesEnabled: os.Getenv("XUANKE_ACTIVATION") != "off",
	}
}

// CaptchaEngineDefault 默认验证码识别引擎：本地 ddddocr（免密钥），无环境则自动回退 Vision。
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

# 教务登录验证码识别密钥（可选留空；默认识别引擎 ddddocr 不需要密钥）
SF_API_KEY=
# 识别引擎（ddddocr=本地默认，免密钥；vision=硅基流动云识别，需填 SF_API_KEY）
XUANKE_CAPTCHA_ENGINE=ddddocr

# 激活码机制开关：on=启用（默认）；off=完全关闭，登录直接进入系统
XUANKE_ACTIVATION=on

# 选课开放时间（可选，留空用内置默认值）
# XUANKE_OPEN_TIME=2026-09-13 09:00:00

# 服务端口与数据库路径（可选）
# XUANKE_PORT=3091
# XUANKE_DB=data/xuanke.db
`
	_ = os.WriteFile(path, []byte(tpl), 0o600)
}
