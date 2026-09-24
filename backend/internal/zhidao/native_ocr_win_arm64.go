//go:build windows && arm64 && cgo

package zhidao

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yangbin1322/go-ddddocr/ddddocr"
)

//go:embed assets/common_old.onnx
var embeddedModel []byte

//go:embed assets/charsets_old.json
var embeddedCharsets []byte

//go:embed assets/onnxruntime_win_arm64.dll
var embeddedDll []byte

// NativeDdddOcrRecognizer 单二进制内置 ddddocr 识别引擎（Windows ARM64 版）。
// 与 amd64 版同构，仅内嵌的 ONNX Runtime 为官方 win-arm64 库
// （onnxruntime_win_arm64.dll，需构建时经 scripts/fetch-onnxruntime.sh 下载到 assets/）。
type NativeDdddOcrRecognizer struct {
	mu     sync.Mutex
	ocr    *ddddocr.DdddOcr
	initMu sync.Once
	err    error
}

// NewNativeDdddOcrRecognizer 创建原生内置 ddddocr 识别引擎。
func NewNativeDdddOcrRecognizer() *NativeDdddOcrRecognizer {
	return &NativeDdddOcrRecognizer{}
}

// ensureInit 懒加载初始化引擎：将内嵌资源释出到安全路径并建立 ONNX 推理会话。
func (r *NativeDdddOcrRecognizer) ensureInit() error {
	r.initMu.Do(func() {
		dir := filepath.Join(os.TempDir(), "xuanke_ddddocr_assets")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			r.err = fmt.Errorf("创建 ddddocr 资源目录失败: %w", err)
			return
		}

		dllPath := filepath.Join(dir, "onnxruntime_win_arm64.dll")
		modelPath := filepath.Join(dir, "common_old.onnx")
		charsetsPath := filepath.Join(dir, "charsets_old.json")

		// 释出动态库与模型（若已存在且大小一致则跳过，避免每次重启重复写入）
		if err := dumpIfDiff(dllPath, embeddedDll); err != nil {
			r.err = fmt.Errorf("释出 onnxruntime_win_arm64.dll 失败: %w", err)
			return
		}
		if err := dumpIfDiff(modelPath, embeddedModel); err != nil {
			r.err = fmt.Errorf("释出 common_old.onnx 失败: %w", err)
			return
		}
		if err := dumpIfDiff(charsetsPath, embeddedCharsets); err != nil {
			r.err = fmt.Errorf("释出 charsets_old.json 失败: %w", err)
			return
		}

		ddddocr.SetOnnxRuntimePath(dllPath)
		// 必须走官方 OCR 模式（ModelDir）：自定义模型分支用 ImageNet 归一化
		// (x-0.456)/0.224，与官方内置模型 (x-0.5)/0.5 不一致，识别结果错误。
		opts := ddddocr.Options{
			Ocr:      true,
			ModelDir: dir,
		}
		inst, err := ddddocr.New(opts)
		if err != nil {
			r.err = fmt.Errorf("初始化 Go 原生 ddddocr 实例失败: %w", err)
			return
		}
		r.ocr = inst
	})
	return r.err
}

// dumpIfDiff 校验文件大小，仅在缺失或大小变化时从内存写入磁盘。
func dumpIfDiff(target string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("内嵌资源为空: %s", target)
	}
	if st, err := os.Stat(target); err == nil && st.Size() == int64(len(data)) {
		return nil // 已经存在且完好
	}
	return os.WriteFile(target, data, 0o644)
}

// Recognize 实现 CaptchaRecognizer 接口（支持并发限制与极速推理）。
func (r *NativeDdddOcrRecognizer) Recognize(img []byte) (string, error) {
	return withConcurrency(func() (string, error) {
		if err := r.ensureInit(); err != nil {
			return "", err
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		res, err := r.ocr.Classification(img)
		if err != nil {
			return "", fmt.Errorf("原生 ddddocr 推理失败: %w", err)
		}
		if res == "" {
			return "", fmt.Errorf("原生 ddddocr 识别结果为空")
		}
		return res, nil
	})
}

// NativeDdddOcrAvailable 检查是否支持原生内置识别（内嵌切片有数据即 100% 可用）。
func NativeDdddOcrAvailable() bool {
	return len(embeddedDll) > 0 && len(embeddedModel) > 0 && len(embeddedCharsets) > 0
}