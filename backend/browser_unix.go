//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// openBrowser 在服务启动后用系统默认浏览器打开前端页面 (Linux / macOS 实现)。
func openBrowser(url string) {
	var start *exec.Cmd
	if runtime.GOOS == "darwin" {
		start = exec.Command("open", url)
	} else {
		start = exec.Command("xdg-open", url)
	}
	_ = start.Start()
}
