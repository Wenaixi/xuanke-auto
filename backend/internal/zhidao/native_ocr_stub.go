//go:build !windows || !cgo

package zhidao

// NativeDdddOcrAvailable 在非 Windows 或禁用 CGO 环境下返回 false
// （是否回退本地 Python ddddocr / Vision 由管理员"引擎兜底"开关决定，默认不回退）。
func NativeDdddOcrAvailable() bool {
	return false
}

// NewNativeDdddOcrRecognizer 存根实例。
func NewNativeDdddOcrRecognizer() CaptchaRecognizer {
	return nil
}
