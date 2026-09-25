//go:build linux && cgo && notray && !android

package main

// Linux 服务器版（release 构建 -tags notray）：CGO=1 只为内嵌原生 ddddocr
// 识别引擎，服务器无桌面环境托盘是纯负担（systray 需 GTK3 且 zig 交叉编 GTK 头
// 会因 regparm 属性失败）。与 tray_linux_cgo0.go 同构：服务照常启动 + 无托盘。

type trayData struct {
	url    string
	dbPath string
	port   string
}

// runTray Linux notray：无托盘，直接返回（main 不等待）。
func runTray(td trayData) {
	_ = td
}
