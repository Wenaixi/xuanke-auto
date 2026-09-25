//go:build linux && arm64 && cgo

package zhidao

import _ "embed"

//go:embed assets/libonnxruntime_linux_arm64.so
var platformDll []byte

func init() {
	embeddedDll = platformDll
	nativeDllFilename = func() string { return "libonnxruntime_linux_arm64.so" }
}