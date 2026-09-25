//go:build linux && !cgo && !android

package main

// Linux CGO=0（Docker/服务器版）托盘占位：systray 在 CGO=0 下无法编译
// （需 GTK3 等 Linux 桌面库），Docker 无桌面环境托盘无意义。本文件被 !cgo tag
// 选入，绕过 systray 的 CGO 依赖——服务照常启动 + 自动开浏览器（与原行为一致）。
// android：GOOS=android 隐含 linux build tag 但平台不同（tray_android.go 补位，
// 见其注释），这里 !android 排除避免重复声明。

type trayData struct {
	url    string
	dbPath string
	port   string
}

// runTray Linux CGO=0：无桌面无托盘，直接返回（main 不等待）。
func runTray(td trayData) {
	_ = td
}
