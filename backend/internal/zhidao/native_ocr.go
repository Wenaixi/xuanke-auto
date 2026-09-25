//go:build cgo

package zhidao

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yangbin1322/go-ddddocr/ddddocr"
)

// 跨平台公共模型与字符集（所有 native 平台共用，随 Q：平台文件经 //go:embed 各自注入 dll）。
//go:embed assets/common_old.onnx
var embeddedModel []byte

//go:embed assets/charsets_old.json
var embeddedCharsets []byte

// embeddedDll 当前平台内嵌的 ONNX Runtime 库（由平台文件在 init() 注入 build tag 对应切片）。
var embeddedDll []byte

// nativeDllFilename 当前平台 ONNX Runtime 库在释出目录中的文件名（平台文件注入）。
var nativeDllFilename = func() string { return "onnxruntime.bin" }

// NativeDdddOcrRecognizer 单二进制内置 ddddocr 识别引擎（纯 Go + ONNX 原生嵌入）。
//
// 运行原理：
//  1. 模型（common_old.onnx，13MB）、字符集（charsets_old.json，56KB）和各平台 ONNX Runtime
//     （dll/so/dylib，16MB）均通过 //go:embed 原生编译进单个可执行文件内部；
//  2. 运行时若本地不存在，以毫秒级自动从内存释放到系统临时目录；
//  3. 使用 ONNX Runtime 引擎在当前进程执行推理，单次识别 5~10 毫秒，宿主机零依赖。
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
// 释出文件：<tmp>/xuanke_ddddocr_assets/{nativeDllFilename, common_old.onnx, charsets_old.json}。
// 平台库名（dll/so/dylib）由当前平台文件经 nativeDllFilename 提供，与内嵌切片配套。
func (r *NativeDdddOcrRecognizer) ensureInit() error {
	r.initMu.Do(func() {
		dir := filepath.Join(os.TempDir(), "xuanke_ddddocr_assets")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			r.err = fmt.Errorf("创建 ddddocr 资源目录失败: %w", err)
			return
		}

		dllPath := filepath.Join(dir, nativeDllFilename())
		modelPath := filepath.Join(dir, "common_old.onnx")
		charsetsPath := filepath.Join(dir, "charsets_old.json")

		// 释出动态库与模型（若已存在且大小一致则跳过，避免每次重启重复写入）
		if err := dumpIfDiff(dllPath, embeddedDll); err != nil {
			r.err = fmt.Errorf("释出 %s 失败: %w", nativeDllFilename(), err)
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
		// modelPath/charsetsPath 必须与释出文件名一致：官方 OCR 模式按
		// ModelDir 下固定名 common_old.onnx / charsets_old.json 查找。

		ddddocr.SetOnnxRuntimePath(dllPath)
		// 关键：必须走官方 OCR 模式（ModelDir），绝不走自定义模型路径
		// （ImportOnnxPath+CharsetsPath）——移植库的自定义模型分支用 ImageNet 归一化
		// (x-0.456)/0.224 预处理，与官方内置模型训练时的 (x-0.5)/0.5 不一致，同一张
		// 英数字验证码识别结果完全错误（实测 cap1: 官方 'sjmh' vs 自定义 'S43'），
		// 登录链路验证码提交必被拒。官方 OCR 模式从 ModelDir 读取 common_old.onnx
		// 与 charsets_old.json，预处理与 Python 原版逐字段一致。
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

// NativeDdddOcrAvailable 检查是否支持原生内置识别（模型/字符集已内嵌且当前平台 dll 已注入）。
func NativeDdddOcrAvailable() bool {
	return len(embeddedDll) > 0 && len(embeddedModel) > 0 && len(embeddedCharsets) > 0
}