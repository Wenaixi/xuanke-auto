//go:build linux && !cgo

package main

// Linux CGO=0（Docker/服务器版）托盘占位：systray 在 CGO=0 下无法编译
// （需 GTK3/liヒン 桌面库），Docker 无桌面环境托盘无意义。本文件被 !cgo tag
// 选入，绕过 systray 的 CGO 依赖——服务照常启动 + 自动开浏览器（与原行为一致）。

type trayData struct {
	url    string
	dbPath string
	port   string
}

// runTray Linux CGO=0：无桌面无托盘，直接返回（main 不等待）。
func runTray(td trayData) {
	_ = td
}
