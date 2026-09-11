package config

import "os"

// Config 应用配置：环境变量可覆盖端口与数据库路径，其余为编译期常量。
type Config struct {
	Port     string // HTTP 监听端口
	DBPath   string // SQLite 数据库路径
	OpenTime string // 选课开放时间（本地时区）
	BaseURL  string // 至道平台根地址
	// 硅基流动 Vision 验证码识别配置
	SFBaseURL string
	SFAPIKey  string
	SFModel   string
	// 复用已有会话：环境变量 XUANKE_TOKEN 与 XUANKE_COOKIE（"k=v; k2=v2" 格式）
	Token   string
	Cookies string
}

// Load 从环境变量与常量组装配置。
func Load() Config {
	return Config{
		Port:      envOr("XUANKE_PORT", "3091"),
		DBPath:    envOr("XUANKE_DB", "data/xuanke.db"),
		OpenTime:  "2026-09-13 09:00:00",
		BaseURL:   "https://www.zhidao.fj.cn",
		SFBaseURL: "https://api.siliconflow.cn/v1",
		SFAPIKey:  envOr("SF_API_KEY", "***REMOVED***"),
		SFModel:   "Qwen/Qwen3-VL-30B-A3B-Instruct",
		Token:     os.Getenv("XUANKE_TOKEN"),
		Cookies:   os.Getenv("XUANKE_COOKIE"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
