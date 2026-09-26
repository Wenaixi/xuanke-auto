package runtime

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
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
	// CaptchaEngine 验证码识别引擎（"vision"=OpenAI 兼容视觉 API；"ddddocr"=本地 ddddocr）。
	CaptchaEngine string
	// CaptchaFallback 引擎兜底开关（默认关闭）。
	// 关闭时两引擎严格互不回退：配置的引擎不可用即视为"识别不可用"，绝不静默换引擎
	// （静默兜底会让管理员以为跑的是本地 ddddocr，实际每次登录都在打云端并计费）。
	// 开启后双向兜底：ddddocr 本机不可用 → Vision；Vision 无密钥 → 本机 ddddocr。
	CaptchaFallback bool
	// CaptchaConcurrency 验证码识别并发上限（默认 1，串行识别防平台熔断）。
	CaptchaConcurrency int
}

// Store 进程内配置中心：读写锁保护，Get 返回拷贝保证调用方拿到一致快照。
type Store struct {
	mu sync.RWMutex
	c  Config
}

// New 用初始配置创建配置中心。
func New(initial Config) *Store {
	return &Store{c: initial}
}

// Get 返回当前配置拷贝（读锁快照，调用方不受后续更新影响）。
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.c
}

// Update 在写锁内应用修改函数。
func (s *Store) Update(f func(*Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f != nil {
		f(&s.c)
	}
}

// configField 一个配置字段的持久化契约：settings 表键 ↔ Config 字段 ↔ 序列化/解析。
// 表是"配置 schema"的唯一事实源——落库（ToSettings）与启动还原（ApplySettings）
// 各自遍历同一张表，新增字段只在此加一行，两侧自动同步。
//
// 此前两侧各手写一份键名清单，漏改不报编译错；又因 SaveSettings 是
// "DELETE 全表 + 全量 INSERT" 的全量替换，漏改的后果是重启后该键从表中消失、
// 配置静默回退默认值且无任何日志或错误——表驱动即该缺口的根因修复。
type configField struct {
	Key string
	// Serialize 把字段值转成落库字符串。encrypt 用于 vision_key 的落库加密，
	// 由调用方注入（api 层持有加密器，runtime 层不依赖加解密实现）。
	Serialize func(c *Config, encrypt func(string) (string, error)) (string, error)
	// Apply 把落库字符串写回字段。decrypt 用于 vision_key 的读取解密。
	// 返回 false 表示该值非法/不应应用，调用方保留既有值。
	Apply func(c *Config, v string, decrypt func(string) (string, error)) bool
}

// encPrefix 加密值前缀——vision_key 落库必须带此前缀，读取时严格要求，
// 旧版明文一律拒绝加载（不兼容旧数据是既有安全决策，见 server.go 启动还原）。
const encPrefix = "enc:"

var configFields = []configField{
	{
		Key:       "activation_enabled",
		Serialize: func(c *Config, _ func(string) (string, error)) (string, error) { return strconv.FormatBool(c.ActivationEnabled), nil },
		Apply:     func(c *Config, v string, _ func(string) (string, error)) bool { c.ActivationEnabled = v == "true"; return true },
	},
	{
		Key:       "vision_base_url",
		Serialize: func(c *Config, _ func(string) (string, error)) (string, error) { return c.VisionBaseURL, nil },
		Apply:     func(c *Config, v string, _ func(string) (string, error)) bool { c.VisionBaseURL = v; return true },
	},
	{
		// 落库一律加密——settings 表内永不出现明文密钥（与凭据同强度 AES-256-GCM）。
		Key: "vision_key",
		Serialize: func(c *Config, encrypt func(string) (string, error)) (string, error) {
			if encrypt == nil {
				return "", errors.New("未注入加密器，拒绝明文落库 vision_key")
			}
			enc, err := encrypt(c.VisionAPIKey)
			if err != nil {
				return "", err
			}
			return encPrefix + enc, nil
		},
		Apply: func(c *Config, v string, decrypt func(string) (string, error)) bool {
			// 严格要求 enc: 前缀：无前缀即旧版明文，绝不加载（宁可不配不可泄密）
			if !strings.HasPrefix(v, encPrefix) {
				log.Printf("[runtime] 发现未加密的旧版 vision_key，已拒绝加载（不兼容旧数据）")
				return false
			}
			if decrypt == nil {
				return false
			}
			plain, err := decrypt(strings.TrimPrefix(v, encPrefix))
			if err != nil {
				log.Printf("[runtime] 解密 vision_key 失败，已忽略: %v", err)
				return false
			}
			c.VisionAPIKey = plain
			return true
		},
	},
	{
		Key:       "vision_model",
		Serialize: func(c *Config, _ func(string) (string, error)) (string, error) { return c.VisionModel, nil },
		Apply:     func(c *Config, v string, _ func(string) (string, error)) bool { c.VisionModel = v; return true },
	},
	{
		Key:       "captcha_engine",
		Serialize: func(c *Config, _ func(string) (string, error)) (string, error) { return c.CaptchaEngine, nil },
		Apply:     func(c *Config, v string, _ func(string) (string, error)) bool { c.CaptchaEngine = v; return true },
	},
	{
		Key:       "captcha_fallback",
		Serialize: func(c *Config, _ func(string) (string, error)) (string, error) { return strconv.FormatBool(c.CaptchaFallback), nil },
		Apply:     func(c *Config, v string, _ func(string) (string, error)) bool { c.CaptchaFallback = v == "true"; return true },
	},
	{
		// 并发上限必须为正数：非法值（0/负数/非数字）保留既有值而非写入垃圾
		Key:       "captcha_concurrency",
		Serialize: func(c *Config, _ func(string) (string, error)) (string, error) { return strconv.Itoa(c.CaptchaConcurrency), nil },
		Apply: func(c *Config, v string, _ func(string) (string, error)) bool {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return false
			}
			c.CaptchaConcurrency = n
			return true
		},
	},
}

// ToSettings 把 Config 序列化为 settings 表的全量键值对。encrypt 用于 vision_key
// 落库加密，为 nil 且存在非空密钥时报错（严禁明文入库）。
func ToSettings(c Config, encrypt func(string) (string, error)) (map[string]string, error) {
	kv := make(map[string]string, len(configFields))
	for _, f := range configFields {
		v, err := f.Serialize(&c, encrypt)
		if err != nil {
			return nil, err
		}
		kv[f.Key] = v
	}
	return kv, nil
}

// ApplySettings 从 settings 表还原配置到 c。缺键保留 c 既有值（旧库可能只存部分
// 键），非法值同样保留（见 configField.Apply）。decrypt 用于 vision_key 读取解密。
func ApplySettings(c *Config, kv map[string]string, decrypt func(string) (string, error)) {
	if c == nil {
		return
	}
	for _, f := range configFields {
		if v, ok := kv[f.Key]; ok {
			f.Apply(c, v, decrypt)
		}
	}
}
