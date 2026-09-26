//go:build darwin && arm64 && cgo

// 注意：本文件当前不参与任何构建——release 流水线的 macOS 双架构统一以 CGO=0
// 交叉编译（识别能力走云端 Vision 兜底），故此 build tag 在发布产物中永不成立。
// 它是「macOS 本机识别」的完整实现而非占位符：assets 管道已就位
// （scripts/fetch-onnxruntime.sh 的 darwin-arm64 分支会取到
// libonnxruntime_darwin_arm64.dylib）。启用它需要先让 macOS 走 CGO=1 构建，
// 而托管的 macOS runner 均为 Intel，无法交叉编译到 arm64，故需要自建
// arm64 runner。在那之前，macOS 用户请配置云端 Vision 识别。

package zhidao

import _ "embed"

//go:embed assets/libonnxruntime_darwin_arm64.dylib
var platformDll []byte

func init() {
	embeddedDll = platformDll
	nativeDllFilename = func() string { return "libonnxruntime_darwin_arm64.dylib" }
}