//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// openBrowser 在服务启动后用系统默认浏览器打开前端页面 (Windows 实现)。
func openBrowser(url string) {
	start := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	start.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = start.Start()
}
