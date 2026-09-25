//go:build android

package main

// Android 壳入口：Go 代码编译为 c-shared 库（-buildmode=c-shared + NDK
// aarch64-linux-android clang），由 Java 侧 System.loadLibrary("xuanke") 加载后
// 显式调 XuankeSetDataDir / XuankeStart / XuankeStop。
// 与桌面 main 的分工：main() 只服务桌面形态；Android 无 main 入口（c-shared
// 库的 main 不执行），全部导出函数由 Java JNI 调用。

/*
#include <stdlib.h>
*/
import "C"

import (
	"log"
	"runtime"
	"sync/atomic"

	"xuanke-auto/backend/internal/config"
)

// export XuankeSetDataDir
// 注入 App 沙箱数据目录（Java filesDir）：config.dataDir 即 <filesDir>/data。
// Java 壳 onCreate 最先调用（config.Load 之前）。
func XuankeSetDataDir(dirCString *C.char) {
	dir := C.GoString(dirCString)
	log.Printf("[android] 数据目录注入: %s", dir)
	config.SetDataDirForPlatform(dir)
}

// export XuankeStart
// 启动整套选课引擎（runServer：API + 调度器 + 数据库），监听 127.0.0.1:3091。
// 返回后 Android 壳的 WebView 加载 http://127.0.0.1:PORT/ 即可使用完整前端。
// 服务生命周期随进程常驻（前台服务保活）；Java onDestroy 调 XuankeStop 收尾。
func XuankeStart() {
	cfg := config.Load()
	log.Printf("[android] 启动选课引擎（runtime %s/%s）", runtime.GOOS, runtime.GOARCH)
	globalStarted.Store(runServer(cfg))
}

// export XuankeStop
// 优雅收尾（DB/会话关闭）；Java onDestroy 时调用。
func XuankeStop() {
	// Started 由 runServer 返回后经 globalStarted 持有；Android 常驻路径 main
	// 不返回、库内 defer 不触发，DB/会话必须由本函数显式关闭。
	if s := globalStarted.Load(); s != nil {
		s.Shutdown()
		s.Cleanup()
	}
}

// globalStarted 服务句柄（runServer 返回后回填，供 XuankeStop 收尾）。
var globalStarted atomic.Pointer[Started]
