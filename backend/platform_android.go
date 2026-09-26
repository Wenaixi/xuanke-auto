//go:build android

package main

// Android 壳入口（Go c-shared 库）：Java System.loadLibrary("xuanke") 加载后，
// JNI_OnLoad（见 jni_android.c）先注册 native 方法，再调本文件导出的
// XuankeSetDataDir / XuankeStart / XuankeStop。
//
// cgo 前言限制：含 //export 的文件，其前言会被 cgo 复制到两份 C 输出文件，
// 故此处**只能放声明、绝不能放定义**（定义统一放 jni_android.c 的 JNI_OnLoad）。
// 这里甚至不需要任何 extern 声明——//export 本身就会生成同名 C 导出符号。
//
// **格式铁律（勿 gofmt 本文件的 export 行）**：`//export` 的 `//` 与 `export` 之间
// **绝不能有空格**——写成 `// export` cgo 会当成普通注释，导出胶水完全不生成，
// 产出的 so 里 Xuanke* 符号为 UNDEF，装机即闪退（UnsatisfiedLinkError: cannot
// locate symbol）。Go 1.19+ gofmt 会把 `//export` 规范成 `// export` 破坏导出
// （golang/go#63123），故本文件 export 行须保持原样，且 CI 有检查兜底。


/*
// jni_android.c 提供的 logcat 写入桥（Android 上 Go 的 stderr 不可见）。
void XuankeLogToAndroid(const char *msg);
*/
import "C"

import (
	"log"
	"runtime"
	"sync/atomic"

	"xuanke-auto/backend/internal/config"
)

// Android 上 c-shared 的 stderr 不接 logcat：Go 的 log.Printf 全部丢失，
// 引擎启动失败时进程秒退且日志无痕，无从排查。这里把标准 log 重定向到
// jni_android.c 的 XuankeLogToAndroid（经 __android_log_print 进 logcat，
// tag 固定 XuanKe，`adb logcat -s XuanKe` 过滤）。init 早于任何业务日志。
func init() {
	log.SetOutput(androidLogWriter{})
	log.SetFlags(0) // 去掉时间戳前缀：logcat 自带时间
}

// androidLogWriter 把每行日志转交 C 侧写入 logcat；C 函数声明（cgo 前言）。
type androidLogWriter struct{}

func (androidLogWriter) Write(p []byte) (int, error) {
	C.XuankeLogToAndroid(C.CString(string(p)))
	return len(p), nil
}

//export XuankeSetDataDir
// 注入 App 沙箱数据目录（Java filesDir）：config.dataDir 即 <filesDir>/data。
// Java 壳 onCreate 最先调用（config.Load 之前）。
func XuankeSetDataDir(dir *C.char) {
	dirStr := C.GoString(dir)
	log.Printf("[android] 数据目录注入: %s", dirStr)
	config.SetDataDirForPlatform(dirStr)
}

//export XuankeStart
// 启动整套选课引擎（runServer：API + 调度器 + 数据库），监听 127.0.0.1:3091。
// 返回后 Android 壳的 WebView 加载 http://127.0.0.1:PORT/ 即可使用完整前端。
// 服务生命周期随进程常驻（前台服务保活）；Java onDestroy 调 XuankeStop 收尾。
func XuankeStart() {
	cfg := config.Load()
	log.Printf("[android] 启动选课引擎（runtime %s/%s）", runtime.GOOS, runtime.GOARCH)
	globalStarted.Store(runServer(cfg))
}

//export XuankeStop
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
