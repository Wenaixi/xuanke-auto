//go:build darwin && arm64 && cgo

package zhidao

import _ "embed"

//go:embed assets/libonnxruntime_darwin_arm64.dylib
var platformDll []byte

func init() {
	embeddedDll = platformDll
	nativeDllFilename = func() string { return "libonnxruntime_darwin_arm64.dylib" }
}