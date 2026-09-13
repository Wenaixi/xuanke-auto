//go:build !windows || !cgo

package zhidao

// NativeDdddOcrAvailable 在非 Windows 或禁用 CGO 环境下返回 false，自动回退到本地 Python 桥接或 Vision 云端。
func NativeDdddOcrAvailable() bool {
	return false
}

// NewNativeDdddOcrRecognizer 存根实例。
func NewNativeDdddOcrRecognizer() CaptchaRecognizer {
	return nil
}
