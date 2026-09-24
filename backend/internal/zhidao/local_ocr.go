package zhidao

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// LocalDdddOcrRecognizer 本地 ddddocr 验证码识别引擎。
//
// 运行原理：调用本机 Python 环境中的 ddddocr 包完成识别（子进程一次调用）。
// 该引擎不需要任何 API 密钥，识别完全发生在本地，隐私与速度俱佳；
// 单 exe 交付策略：ddddocr 依赖本机 Python（+pip install ddddocr），
// 若本机无 Python/ddddocr，是否回退到 Vision 由管理员后台的"引擎兜底"开关决定
// （默认关闭，两引擎严格互不回退；解析决策见 api 包 resolveCaptchaRecognizer）。
type LocalDdddOcrRecognizer struct {
	// pythonExe 指定的 Python 解释器路径；为空时用系统 PATH 的 "python"。
	pythonExe string
	// ddddocr 识别超时（首次加载 ONNX 模型较慢，给足缓冲）。
	timeout time.Duration
}

// NewLocalDdddOcrRecognizer 创建本地 ddddocr 识别引擎（pythonExe 为空则走 PATH 的 python）。
func NewLocalDdddOcrRecognizer(pythonExe string) *LocalDdddOcrRecognizer {
	return &LocalDdddOcrRecognizer{pythonExe: pythonExe, timeout: 30 * time.Second}
}

// Recognize 实现 CaptchaRecognizer 接口（本地识别，带并发限流）。
func (r *LocalDdddOcrRecognizer) Recognize(img []byte) (string, error) {
	return withConcurrency(func() (string, error) {
		return r.runOnce(img)
	})
}

// ddddocrScript 子进程内执行的 Python 脚本：读 stdin 的 base64 图片，输出识别字符。
// show_ad=False 关闭广告输出；beta 关闭后为稳定识别模式。
const ddddocrScript = `
import base64, sys
import ddddocr
img = base64.b64decode(sys.stdin.buffer.read().strip())
ocr = ddddocr.DdddOcr(show_ad=False)
print(ocr.classification(img))
`

// runOnce 单次子进程识别调用。
func (r *LocalDdddOcrRecognizer) runOnce(img []byte) (string, error) {
	py := r.pythonExe
	if py == "" {
		py = "python"
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, py, "-c", ddddocrScript)
	cmd.Stdin = bytes.NewReader([]byte(base64.StdEncoding.EncodeToString(img)))
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("本地 ddddocr 子进程失败: %w（%s）", err, strings.TrimSpace(errBuf.String()))
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return "", fmt.Errorf("本地 ddddocr 识别结果为空")
	}
	return text, nil
}

// LocalDdddOcrAvailable 检测本机是否具备 ddddocr 能力（Python + ddddocr 包）。
// 管理员在后台切换引擎时会先探测，缺失则拒绝切换并提示安装指引。
func LocalDdddOcrAvailable(pythonExe string) bool {
	py := pythonExe
	if py == "" {
		py = "python"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, "-c", "import ddddocr")
	return cmd.Run() == nil
}
