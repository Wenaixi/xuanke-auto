//go:build linux && cgo && !notray && !android

package main

// Linux 桌面版托盘（桌面版带托盘 + Docker 无头）。
// 用 getlantern/systray（Linux 后端需要 CGO + GTK3 桌面库）——CGO=0 时本文件
// 被 build tag 排除（见 tray_linux_cgo0.go），Docker/服务器版无托盘。
// release 的 Linux 服务器构建带 -tags notray（CGO=1 只为内嵌 ddddocr，服务器
// 无桌面托盘是纯负担；且 zig 交叉编 GTK 头会因 regparm 属性失败），见
// tray_linux_notray.go（linux && cgo && notray 占位，与服务同跑无托盘）。
// android：GOOS=android 隐含 linux build tag，但 Android 无桌面托盘且不编 GTK
// （systray 需 GTK3 头，NDK 无），用 tray_android.go 占位（同 notray 语义）。
// 关于对话框用 zenity 命令行对话框（多数桌面发行版自带；无 zenity 时打印到控制台）。

import (
	_ "embed"
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

	// 注入「退图标」半段（「关服务」半段由 main 注入，见 quit_shared.go 注释）。
	// 退出 = 整进程退出：quitApplication() 先关服务再退图标，与 Windows 同语义。
	setExitActions(nil, systray.Quit)

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser(trayCurrent.url)
			case <-mAbout.ClickedCh:
				showAboutLinux(trayCurrent)
			case <-mQuit.ClickedCh:
				// 退出 = 整进程退出：先优雅关服务再退图标（两个注入点按
				// quit_shared.go 顺序保证），与 Windows 托盘同一条退出语义。
				quitApplication()
				return
			}
		}
	}()
	close(trayStartOnce)
}

// trayIconPNG 托盘图标资源：16x16 RGBA PNG，由设计稿派生的方形图标（黑方块 + 白爪，
// 白底已去净）。
//
// 为什么 16x16 而不是更大：systray 的 Linux 后端（AppIndicator）按面板需求自行缩放，
// 16x16 是该链路既有验证过的尺寸；放大到 22/24 在本机无 Linux 桌面环境可验证，
// 属未经验证的改动，不做。
//
//go:embed tray_linux.png
var trayIconPNG []byte

// trayPNG 返回托盘图标 PNG 字节（systray linux 后端直接接受 PNG 字节）。
func trayPNG() []byte { return trayIconPNG }

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
