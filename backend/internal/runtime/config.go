package runtime

import (
	"sync"
	"time"
)

// Config 系统运行配置（管理员热重载，任何改动立即生效无需重启）。
type Config struct {
	// ActivationEnabled 激活码机制开关（off 完全禁用，登录直接签发会话）。
	ActivationEnabled bool
	// VisionBaseURL 验证码识别 OpenAI 兼容服务地址（如 https://api.siliconflow.cn/v1）。
	VisionBaseURL string
	// VisionAPIKey 验证码识别密钥（明文只在进程内存，落库经数据加密密钥）。
	VisionAPIKey string
	// VisionModel 验证码识别模型。
	VisionModel string
	// CaptchaEngine 验证码识别引擎（"vision"=硅基流动 Vision；"ddddocr"=本地 ddddocr）。
	CaptchaEngine string
	// CaptchaConcurrency 验证码识别并发上限（默认 1，串行识别防平台熔断）。
	CaptchaConcurrency int
	// OpenTime 选课开放时间（本地时区字符串，如 2026-09-13 09:00:00）。
	OpenTime string
	// OpenTimeParsed 解析后的开放时间（调度器直接使用）。
	OpenTimeParsed time.Time
}

// Store 进程内配置中心：读写锁保护，Get 返回拷贝保证调用方拿到一致快照。
type Store struct {
	mu sync.RWMutex
	c  Config
}

// New 用初始配置创建配置中心。
func New(initial Config) *Store {
	s := &Store{}
	s.c = initial
	s.reparse()
	return s
}

// Get 返回当前配置拷贝（读锁快照，调用方不受后续更新影响）。
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.c
}

// Update 在写锁内应用修改函数，随后重解析开放时间。
func (s *Store) Update(f func(*Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f != nil {
		f(&s.c)
	}
	s.reparse()
}

// reparse 解析 OpenTime 到 OpenTimeParsed（非法值重置为零值，由调度器兜底）。
func (s *Store) reparse() {
	s.c.OpenTimeParsed = time.Time{}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s.c.OpenTime, time.Local); err == nil {
		s.c.OpenTimeParsed = t
	}
}
