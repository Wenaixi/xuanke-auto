//go:build android && arm64 && cgo

package zhidao

import _ "embed"

//go:embed assets/libonnxruntime_android_arm64.so
var platformDll []byte

func init() {
	embeddedDll = platformDll
	nativeDllFilename = func() string { return "libonnxruntime_android_arm64.so" }
}
