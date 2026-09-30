//go:build windows && arm64 && cgo

package upstream

import _ "embed"

//go:embed assets/onnxruntime_win_arm64.dll
var platformDll []byte

func init() {
	embeddedDll = platformDll
	nativeDllFilename = func() string { return "onnxruntime_win_arm64.dll" }
}
