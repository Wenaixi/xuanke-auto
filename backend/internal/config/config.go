package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

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
	// ActivationCodesEnabled 激活码机制开关（XUANKE_ACTIVATION，默认 on；off 完全禁用激活码）
	ActivationCodesEnabled bool
}

// Load 从环境变量与 data/.env 文件组装配置。
// 数据库统一固定在仓库根目录 ../data（仓库根 data/，含 .env/.master_key/xuanke.db，随仓库一起备份迁移）；
// 真实环境变量优先，data/.env 文件兜底。
func Load() Config {
	dbPath := envOr("XUANKE_DB", "../data/xuanke.db")
	loadDotEnv(filepath.Join(filepath.Dir(dbPath), ".env"))
	return Config{
		Port:       envOr("XUANKE_PORT", "3091"),
		DBPath:     dbPath,
		OpenTime:   "2026-09-13 09:00:00",
		BaseURL:    "https://www.zhidao.fj.cn",
		SFBaseURL:  envOr("SF_BASE_URL", "https://api.siliconflow.cn/v1"),
		SFAPIKey:   os.Getenv("SF_API_KEY"),
		SFModel:    envOr("SF_MODEL", "Qwen/Qwen3-VL-30B-A3B-Instruct"),
		AdminToken: os.Getenv("XUANKE_ADMIN_TOKEN"),
		ActivationCodesEnabled: os.Getenv("XUANKE_ACTIVATION") != "off",
	}
}

// loadDotEnv 读取 KEY=VALUE 配置文件并注入环境变量（不覆盖已存在的真实环境变量）。
// 文件不存在时自动生成带注释的模板，方便部署填写。
func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeEnvTemplate(path)
		}
		return
	}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if k == "" || os.Getenv(k) != "" {
			continue // 跳过空键；真实环境变量优先
		}
		os.Setenv(k, v)
	}
}

// writeEnvTemplate 生成 data/.env 模板（带注释说明每个配置项）。
func writeEnvTemplate(path string) {
	tpl := `# 至道选课自动化 - 环境配置文件（与 data/ 一起备份迁移）
# 真实环境变量优先于本文件；留空的项使用默认值

# 管理口令：激活码管理接口必填，缺失拒绝启动
XUANKE_ADMIN_TOKEN=

# 教务登录验证码识别密钥（可选，缺失时验证码识别不可用）
SF_API_KEY=

# 激活码机制开关：on=启用（默认）；off=完全关闭，登录直接进入系统
XUANKE_ACTIVATION=on

# 数据加密主密钥（64 位十六进制；可选，不填自动生成 ../data/.master_key）
# XUANKE_MASTER_KEY=

# 服务端口与数据库路径（默认 3091 / ../data/xuanke.db，仓库根目录 data/）
# XUANKE_PORT=3091
# XUANKE_DB=../data/xuanke.db
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(tpl), 0o600)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
