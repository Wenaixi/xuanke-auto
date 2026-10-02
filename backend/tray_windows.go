//go:build windows

package main

// Windows 托盘实现（exe 启动常驻系统托盘，右键菜单 打开浏览器/关于/退出）。
// 用 getlantern/systray（成熟跨平台托盘库，Windows 原生 SysTrayIcon）。
// 关于对话框用 Win32 原生 MessageBox（深色系统主题自动暗色，符合项目纯黑极简风格）。

import (
	_ "embed"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
)

// trayData 托盘共享上下文：URL（打开浏览器）、数据库路径（关于对话框）、端口。
type trayData struct {
	url    string
	dbPath string
	port   string
}

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	messageBoxW   = user32.NewProc("MessageBoxW")
	trayStartOnce = make(chan struct{}) // main 等待托盘就绪后继续
	trayCurrent   trayData              // 菜单回调共享数据
)

// trayIconICO 托盘图标资源：多尺寸 ICO（16/20/24/32/48，覆盖 100%/125%/150% 缩放的
// 托盘尺寸），每条目为 32bpp DIB（BGRA + AND 掩码）。
//
// 为什么用嵌入资源而不是像历史上那样在代码里拼字节：图标已是设计稿派生的猫爪图形，
// 无法用几行几何公式表达。格式刻意沿用同一套 DIB 布局——该布局的字节序与
// dwBytesInRes/dwImageOffset 取值已在真机 LoadImageW 上验证过（历史上曾两次写反导致
// 图标不可见），换图只需换像素数据，不必重新验证格式。
//
//go:embed tray_windows.ico
var trayIconICO []byte

// trayIcon 返回托盘图标字节（systray 直接消费内存中的 ICO 数据）。
func trayIcon() []byte { return trayIconICO }

// runTray 启动托盘并阻塞等待就绪（systray.Run 在自己的消息循环里运行）。
// main 在托盘就绪后放行，避免托盘图标一闪而过就随进程退出。
func runTray(td trayData) {
	trayCurrent = td
	go func() {
		systray.Run(onReady, nil)
	}()
	<-trayStartOnce // 等托盘图标与菜单装配完成
}

// onReady 托盘图标就绪后的菜单装配回调。
func onReady() {
	systray.SetIcon(trayIcon())
	systray.SetTooltip("自动选课")

	mOpen := systray.AddMenuItem("打开浏览器", "用默认浏览器打开选课大厅")
	mAbout := systray.AddMenuItem("关于", "查看版本与数据库路径")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")

	// 注入「退图标」半段（「关服务」半段由 main 注入，见 quit_shared.go 注释）。
	// 退出 = 整进程退出：quitApplication() 先关服务再退图标（回归钉）。
	setExitActions(nil, systray.Quit)

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser(trayCurrent.url)
			case <-mAbout.ClickedCh:
				showAbout(trayCurrent)
			case <-mQuit.ClickedCh:
				// 退出 = 整进程退出：先优雅关服务（srv.Shutdown）再退图标，
				// 顺序由 quitApplication 保证（回归钉见 tray_quit_test.go）。
				quitApplication()
				return
			}
		}
	}()
	close(trayStartOnce) // 托盘就绪，放行 main 继续
}

// showAbout 关于对话框：Win32 原生 MessageBox（深色系统主题自动暗色，符合项目风格）。
// 显示程序名、数据库绝对路径、监听地址与选课大厅网址。
func showAbout(td trayData) {
	dbAbs := td.dbPath
	if abs, err := filepath.Abs(td.dbPath); err == nil {
		dbAbs = abs
	}
	body := "自动选课\n\n数据库路径：\n" + dbAbs + "\n\n监听地址：http://localhost:" + td.port + "\n\n选课大厅：\n" + td.url
	showMessageBox("关于 自动选课", body)
}

// showMessageBox 调用 Win32 MessageBoxW（MB_OK | MB_ICONINFORMATION）。
func showMessageBox(title, body string) {
	titleUTF16, _ := syscall.UTF16PtrFromString(title)
	bodyUTF16, _ := syscall.UTF16PtrFromString(body)
	const (
		mbOK              = 0x00000000
		mbIconInformation = 0x00000040
	)
	_, _, _ = messageBoxW.Call(0, uintptr(unsafe.Pointer(bodyUTF16)), uintptr(unsafe.Pointer(titleUTF16)), mbOK|mbIconInformation)
}
