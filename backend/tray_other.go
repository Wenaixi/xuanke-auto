//go:build !windows && !linux

package main

// 非 Windows 非 Linux 平台（darwin 等）托盘占位：只维护 win/linux/docker，
// 不做其他——darwin 不在维护清单，本占位保证编译通过（无托盘 + 服务照常启动）。

// trayData 托盘共享上下文（与 tray_windows/tray_linux 同构，保证 main 引用一致）。
type trayData struct {
	url    string
	dbPath string
	port   string
}

// runTray 其他平台：无托盘，直接返回。
func runTray(td trayData) {
	_ = td
}
