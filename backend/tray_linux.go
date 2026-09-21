//go:build linux && cgo

package main

// Linux 桌面版托盘（桌面版带托盘 + Docker 无头）。
// 用 getlantern/systray（Linux 后端需要 CGO + GTK3 桌面库）——CGO=0 时本文件
// 被 build tag 排除（见 tray_linux_cgo0.go），Docker/服务器版无托盘。
// 关于对话框用 zenity 命令行对话框（多数桌面发行版自带；无 zenity 时打印到控制台）。

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/getlantern/systray"
)

// trayData 托盘共享上下文：URL（打开浏览器）、数据库路径（关于对话框）、端口。
type trayData struct {
	url    string
	dbPath string
	port   string
}

var (
	trayStartOnce = make(chan struct{}) // main 等待托盘就绪后继续
	trayCurrent   trayData
)

// runTray 启动托盘并阻塞等待就绪（systray.Run 在自己的消息循环里运行）。
// main 在托盘就绪后放行，避免托盘图标一闪而过就随进程退出。
func runTray(td trayData) {
	trayCurrent = td
	go func() {
		systray.Run(onReady, nil)
	}()
	<-trayStartOnce
}

// onReady 托盘图标就绪后的菜单装配回调。
func onReady() {
	systray.SetIcon(trayPNG())
	systray.SetTooltip("至道选课自动化")

	mOpen := systray.AddMenuItem("打开浏览器", "用默认浏览器打开选课大厅")
	mAbout := systray.AddMenuItem("关于", "查看版本与数据库路径")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser(trayCurrent.url)
			case <-mAbout.ClickedCh:
				showAboutLinux(trayCurrent)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
	close(trayStartOnce)
}

// trayPNG 托盘图标：最小合法 16x16 RGBA PNG（纯黑 + 中心白点）——
// systray linux 后端接受 PNG 字节（icon.GetIcon 经 AppIndicator 显示）。
func trayPNG() []byte {
	// 16x16 RGBA 手写最小 PNG（IHDR+IDAT+IEND），黑色背景 + 中心 4x4 白块。
	// 用内置极简 1x1 黑色 PNG 占位（AppIndicator 会自适应缩放）。
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG 签名
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR 长度+标识
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, // 8bit RGBA
		0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, // IDAT 长度+标识
		0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, // 黑色像素
		0x0D, 0x0A, 0x2D, 0xB4,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82, // IEND
	}
}

// showAboutLinux 关于对话框：优先 zenity（多数桌面发行版自带），无 zenity 时控制台打印。
func showAboutLinux(td trayData) {
	dbAbs := td.dbPath
	if abs, err := filepath.Abs(td.dbPath); err == nil {
		dbAbs = abs
	}
	body := "数据库路径：" + dbAbs + "\n监听地址：http://localhost:" + td.port + "\n选课大厅：" + td.url
	showZenityOrPrint("关于 至道选课自动化", "至道选课自动化\n\n"+body)
}

var _ = strings.Builder{}
var _ = fmt.Sprintf
