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
	"time"
)

// CaptchaRecognizer 验证码识别引擎统一接口：ddddocr 本地 / 硅基流动 Vision 二选一。
type CaptchaRecognizer interface {
	Recognize(img []byte) (string, error)
}

// captchaSemaphore 全局验证码识别并发限流信号量（默认并发 1）。
// 多个账号同时失效重登时，识别请求严格串行——平台验证码接口与登录接口
// 对高并发敏感，串行识别从根因杜绝"登录失败次数过多"熔断。
var captchaSemaphore chan struct{}

// SetCaptchaConcurrency 动态调整识别并发上限（管理员热重载，默认 1）。
// 并发只能收敛到更小（信号量无法扩容），扩容需重启服务。
func SetCaptchaConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	cur := cap(captchaSemaphore)
	if cur == n {
		return
	}
	if cur < n {
		// 扩容需重建信号量，但重建无法等待在飞请求——这里保守不做（重启后生效）
		log.Printf("[captcha] 识别并发扩容至 %d 需重启服务生效，当前保持 %d", n, cur)
		return
	}
	// 收敛：用局部信号量逐步替换，直到容量降至目标（并发请求可在旧信号量上继续）
	down := make(chan struct{}, n)
	captchaSemaphore = down
}

// NewCaptchaSemaphore 初始化识别并发信号量（默认并发 1，启动时调用一次）。
func NewCaptchaSemaphore(concurrency int) {
	if concurrency < 1 {
		concurrency = 1
	}
	captchaSemaphore = make(chan struct{}, concurrency)
}

// withConcurrency 在信号量许可下执行识别（串行化识别请求，返回识别结果）。
func withConcurrency(fn func() (string, error)) (string, error) {
	captchaSemaphore <- struct{}{}
	defer func() { <-captchaSemaphore }()
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
	resp, err := client.Do(req)
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

// Recognize 实现 CaptchaRecognizer 接口（带并发限流）。
func (v *VisionRecognizer) Recognize(img []byte) (string, error) {
	return withConcurrency(func() (string, error) {
		return recognizeViaVision(v.cfg, img)
	})
}

// NewVisionRecognizer 创建 Vision 识别引擎（APIKey 为空时识别会立即报错）。
func NewVisionRecognizer(cfg VisionConfig) *VisionRecognizer {
	return &VisionRecognizer{cfg: cfg}
}
