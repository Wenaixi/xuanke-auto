//go:build android

package main

// Android 平台托盘占位：GOOS=android 隐含 linux build tag，但 Android 无桌面
// 托盘且不编 GTK（systray 需 GTK3 头，NDK 无）——tray_linux.go（linux && cgo
// && !notray && !android）与 tray_linux_notray.go（!android）都被排除后，
// 本文件补位保证 trayData/runTray 编译通过（服务照常启动 + 无托盘）。

type trayData struct {
	url    string
	dbPath string
	port   string
}

// runTray Android：无托盘，直接返回（main 不等待；Android 走 XuankeStart 入口）。
func runTray(td trayData) {
	_ = td
}
