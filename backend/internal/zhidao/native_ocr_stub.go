//go:build !(windows && (amd64 || arm64) && cgo) && !(linux && (amd64 || arm64) && cgo) && !(darwin && arm64 && cgo) && !(android && arm64 && cgo)

package zhidao

// NativeDdddOcrAvailable 在无原生内置引擎的平台（非 cinnamon 平台或禁用 CGO）返回 false
// （是否回退本地 Python ddddocr / Vision 由管理员"引擎兜底"开关决定，默认不回退）。
func NativeDdddOcrAvailable() bool {
	return false
}

// NewNativeDdddOcrRecognizer 存根实例。
func NewNativeDdddOcrRecognizer() CaptchaRecognizer {
	return nil
}