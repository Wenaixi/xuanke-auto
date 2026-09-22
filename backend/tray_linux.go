//go:build linux && cgo

package main

// Linux 桌面版托盘（桌面版带托盘 + Docker 无头）。
// 用 getlantern/systray（Linux 后端需要 CGO + GTK3 桌面库）——CGO=0 时本文件
// 被 build tag 排除（见 tray_linux_cgo0.go），Docker/服务器版无托盘。
// 关于对话框用 zenity 命令行对话框（多数桌面发行版自带；无 zenity 时打印到控制台）。

import (
	"fmt"
	"os/exec"
	"path/filepath"

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

// trayPNG 托盘图标：16x16 RGBA PNG（纯黑底 + 中心 4x4 白块）——
// systray linux 后端接受 PNG 字节（icon.GetIcon 经 AppIndicator 显示）。
// 字节经 PIL/Go stdlib 双重验证：签名/IHDR/IDAT/IEND 合法、zlib 解压 256 像素
// RGBA、深色面板上黑底白点可见（与 tray_windows.go 的图标视觉同语义）。
func trayPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG 签名
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR 长度+标识
		0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10, // 16x16
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, // 8bit RGBA
		0x00, 0x00, 0x00, 0x1D, 0x49, 0x44, 0x41, 0x54, // IDAT 长度+标识
		0x78, 0xDA, 0x63, 0x60, 0x60, 0x60, 0xF8, 0x4F, 0x21, 0x1E, 0x35, 0x80, 0x66, 0x06, 0xA0, // 压缩流（78 DA = zlib 9 级）
		0x83, 0x91, 0x68, 0xC0, 0x68, 0x3A, 0xA0, 0xA3, // 黑底 + 中心白块
		0x01, 0x00, 0xD7, 0xE3, 0x2E, 0xE0, 0x10, 0x78, // （后续为 zlib 压缩数据）
		0x07, 0x4D, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, // IEND 长度+标识
		0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82, // IEND
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

// showZenityOrPrint 用 zenity 弹关于对话框（多数 Linux 桌面发行版自带）；
// 无 zenity（最小化桌面/无 X 环境）时降级打印到控制台，绝不因对话框失败阻塞托盘。
func showZenityOrPrint(title, body string) {
	if path, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command(path, "--info", "--title", title, "--text", body)
		if err := cmd.Run(); err == nil {
			return
		}
	}
	fmt.Printf("[tray] 关于（无 zenity 降级打印）：%s\n%s\n", title, body)
}
