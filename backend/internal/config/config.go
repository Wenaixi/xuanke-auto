package config

import "os"

// Config 应用配置：环境变量可覆盖端口与数据库路径，其余为编译期常量。
// 安全策略：SF_API_KEY 与 XUANKE_ADMIN_TOKEN 必须来自环境变量，代码内不含任何硬编码密钥。
type Config struct {
	Port     string // HTTP 监听端口
	DBPath   string // SQLite 数据库路径
	OpenTime string // 选课开放时间（本地时区）
	BaseURL  string // 至道平台根地址
	// 硅基流动 Vision 验证码识别配置（登录必需）
	SFBaseURL string
	SFAPIKey  string
	SFModel   string
	// AdminToken 部署访问口令（公网部署必备，启动时校验）
	AdminToken string
}

// Load 从环境变量与常量组装配置。
func Load() Config {
	return Config{
		Port:       envOr("XUANKE_PORT", "3091"),
		DBPath:     envOr("XUANKE_DB", "data/xuanke.db"),
		OpenTime:   "2026-09-13 09:00:00",
		BaseURL:    "https://www.zhidao.fj.cn",
		SFBaseURL:  envOr("SF_BASE_URL", "https://api.siliconflow.cn/v1"),
		SFAPIKey:   os.Getenv("SF_API_KEY"),
		SFModel:    envOr("SF_MODEL", "Qwen/Qwen3-VL-30B-A3B-Instruct"),
		AdminToken: os.Getenv("XUANKE_ADMIN_TOKEN"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
