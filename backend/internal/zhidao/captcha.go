package zhidao

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CaptchaRecognizer 验证码识别引擎统一接口：ddddocr 本地 / 硅基流动 Vision 二选一。
type CaptchaRecognizer interface {
	Recognize(img []byte) (string, error)
}

// normalizeCaptchaText 统一规范化验证码识别结果：
// 只保留字母/数字（平台验证码为纯英数字，净化可剔除视觉模型拼接的噪声符号/空格），
// 长度不足 3 或超过 5 视为识别无效（平台验证码字符数 3~5）。
func normalizeCaptchaText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// captchaLimiter 动态线程安全验证码识别并发限流器（使用 Mutex + Cond 协同，彻底杜绝 channel 替换引发的死锁与竞态）。
type captchaLimiter struct {
	mu      sync.Mutex
	cond    *sync.Cond
	limit   int
	running int
}

var (
	globalLimiter     *captchaLimiter
	globalLimiterOnce sync.Once
)

func getGlobalLimiter() *captchaLimiter {
	globalLimiterOnce.Do(func() {
		globalLimiter = newCaptchaLimiter(1)
	})
	return globalLimiter
}

func newCaptchaLimiter(limit int) *captchaLimiter {
	if limit < 1 {
		limit = 1
	}
	l := &captchaLimiter{limit: limit}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *captchaLimiter) Acquire() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for l.running >= l.limit {
		l.cond.Wait()
	}
	l.running++
}

func (l *captchaLimiter) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running > 0 {
		l.running--
	}
	l.cond.Broadcast()
}

func (l *captchaLimiter) SetLimit(n int) {
	if n < 1 {
		n = 1
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = n
	l.cond.Broadcast()
}

// SetCaptchaConcurrency 动态调整识别并发上限（管理员热重载，默认 1）。
func SetCaptchaConcurrency(n int) {
	getGlobalLimiter().SetLimit(n)
}

// NewCaptchaSemaphore 初始化识别并发信号量（默认并发 1，启动时调用一次）。
func NewCaptchaSemaphore(concurrency int) {
	getGlobalLimiter().SetLimit(concurrency)
}

// withConcurrency 在信号量许可下执行识别（串行化识别请求，返回识别结果）。
func withConcurrency(fn func() (string, error)) (string, error) {
	limiter := getGlobalLimiter()
	limiter.Acquire()
	defer limiter.Release()
	return fn()
}

// recognizeCaptcha 调用硅基流动 Vision 模型识别验证码图片，返回识别的字符。
// 使用独立 http.Client，避免与主客户端的 token 请求互相影响。
func recognizeCaptcha(cfg VisionConfig, img []byte) (string, error) {
	// 空识别器：未配置时直接报错（Vision 缺 key / ddddocr 未初始化）
	if cfg.recognizer == nil {
		if cfg.APIKey == "" {
			return "", fmt.Errorf("未配置验证码识别引擎：既无 Vision API Key 也未启用本地 ddddocr")
		}
		return "", fmt.Errorf("验证码识别器未初始化")
	}
	return cfg.recognizer.Recognize(img)
}

// recognizeViaVision 硅基流动 Vision 识别核心实现（recognizeCaptcha 的底层实际调用）。
func recognizeViaVision(cfg VisionConfig, img []byte) (string, error) {
	if cfg.APIKey == "" {
		return "", fmt.Errorf("未配置硅基流动 API Key")
	}
	b64 := base64.StdEncoding.EncodeToString(img)
	payload := map[string]any{
		"model": cfg.Model,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:image/jpeg;base64," + b64,
						},
					},
					map[string]any{
						"type": "text",
						"text": "请识别这张图片中的验证码字符，只输出字符本身，不要输出任何其他内容。",
					},
				},
			},
		},
		"temperature": 0,
		"max_tokens":  32,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost,
		strings.TrimRight(cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	// R52：识别请求同样走活性自愈（httpDo 对连接层错误重试一次）——Vision mock 服务器
	// 在长时间连跑下同样可能复用濒死 keep-alive 连接导致 connectex，测试 flake 同根。
	resp, err := httpDo(client, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("硅基流动接口 HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 200)]))
	}
	var j struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &j); err != nil {
		return "", fmt.Errorf("硅基流动响应解析失败: %w", err)
	}
	if len(j.Choices) == 0 {
		return "", fmt.Errorf("硅基流动响应无 choices")
	}
	return strings.TrimSpace(j.Choices[0].Message.Content), nil
}

// VisionRecognizer 硅基流动 Vision 识别引擎（实现 CaptchaRecognizer 接口）。
type VisionRecognizer struct {
	cfg VisionConfig
}

// Recognize 实现 CaptchaRecognizer 接口（带并发限流 + 结果规范化）。
func (v *VisionRecognizer) Recognize(img []byte) (string, error) {
	return withConcurrency(func() (string, error) {
		raw, err := recognizeViaVision(v.cfg, img)
		if err != nil {
			return "", err
		}
		norm := normalizeCaptchaText(raw)
		if len(norm) < 3 || len(norm) > 5 {
			// A6：不再把净化前的识别原文拼进错误（原文回传客户端是信息外泄面，
			// 多租户/NAT 共享出口场景尤甚）——只回传字符数，调试痕迹留在进程日志。
			log.Printf("[zhidao] Vision 识别字符数 %d 不匹配平台 3~5 位（原文已脱敏不回传）", len(norm))
			return "", fmt.Errorf("识别长度为 %d，不匹配平台 3~5 位字符", len(norm))
		}
		return norm, nil
	})
}

// NewVisionRecognizer 创建 Vision 识别引擎（APIKey 为空时识别会立即报错）。
func NewVisionRecognizer(cfg VisionConfig) *VisionRecognizer {
	return &VisionRecognizer{cfg: cfg}
}
