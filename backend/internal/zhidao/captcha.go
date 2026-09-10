package zhidao

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// recognizeCaptcha 调用硅基流动 Vision 模型识别验证码图片，返回识别的字符。
// 使用独立 http.Client，避免与主客户端的 token 请求互相影响。
func recognizeCaptcha(cfg VisionConfig, img []byte) (string, error) {
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
