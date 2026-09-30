//go:build linux && amd64 && cgo && !android

package upstream

import _ "embed"

//go:embed assets/libonnxruntime_linux_amd64.so
var platformDll []byte

func init() {
	embeddedDll = platformDll
	nativeDllFilename = func() string { return "libonnxruntime_linux_amd64.so" }
}
