//go:build windows

package main

// Windows 托盘实现（exe 启动常驻系统托盘，右键菜单 打开浏览器/关于/退出）。
// 用 getlantern/systray（成熟跨平台托盘库，Windows 原生 SysTrayIcon）。
// 关于对话框用 Win32 原生 MessageBox（深色系统主题自动暗色，符合项目纯黑极简风格）。

import (
	"encoding/binary"
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
	systray.SetTooltip("至道选课自动化")

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

// trayIcon 托盘图标：32x32 纯黑底 + 中心白点（与项目纯黑极简风格一致）。
// 程序内生成合法 ICO（ICONDIR + ICONDIRENTRY + BITMAPINFOHEADER + 32bpp DIB）。
func trayIcon() []byte {
	const (
		width  = 32
		height = 32
	)
	pixelBytes := width * height * 4 // 32bpp BGRA
	andRowBytes := (width + 7) / 8
	andMaskBytes := andRowBytes * height
	headerSize := 6 + 16 // ICONDIR(6) + ICONDIRENTRY(16)
	dibHeaderSize := 40  // BITMAPINFOHEADER
	dataLen := headerSize + dibHeaderSize + pixelBytes + andMaskBytes
	ico := make([]byte, dataLen)

	// ICONDIR
	ico[0], ico[1] = 0, 0
	ico[2], ico[3] = 1, 0 // type = 1 (icon)
	ico[4], ico[5] = 1, 0 // count = 1
	// ICONDIRENTRY（偏移 6）
	ico[6] = width
	ico[7] = height
	ico[10], ico[11] = 1, 0  // planes
	ico[12], ico[13] = 32, 0 // bitcount
	// ICONDIRENTRY[14:22] 两字段（Microsoft ICO 布局）：[14:18]=dwBytesInRes（数据区
	// 全量 = len(ico)-22）、[18:22]=dwImageOffset（目录后首个数据字节 = 22）。
	// LoadImageW 实测：两值写反/写错即加载失败（图标不可见）；此前代码把 22 填进
	// bytesInRes、4286 填进 offset 双重错位。
	binary.LittleEndian.PutUint32(ico[14:18], uint32(len(ico)-headerSize)) // dwBytesInRes
	binary.LittleEndian.PutUint32(ico[18:22], uint32(headerSize))          // dwImageOffset
	// BITMAPINFOHEADER（偏移 22）：biSize=40 / biWidth=32 / biHeight=64(XOR+AND 双高) /
	// biPlanes=1 / biBitCount=32。字段各 32/32/16/16 位，逐字节铺不越界不串位。
	dib := ico[22:]
	dib[0], dib[1], dib[2], dib[3] = 40, 0, 0, 0 // biSize
	dib[4], dib[5], dib[6], dib[7] = width, 0, 0, 0
	dib[8], dib[9] = byte(height*2), byte((height*2)>>8) // XOR + AND 双高
	dib[10], dib[11] = 0, 0
	dib[12], dib[13] = 1, 0  // biPlanes
	dib[14], dib[15] = 32, 0 // biBitCount
	// 像素：全黑（alpha 255 不透明） + 中心 3x3 白点
	pixStart := headerSize + dibHeaderSize
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			off := pixStart + (y*width+x)*4
			white := x >= 14 && x <= 17 && y >= 14 && y <= 17
			if white {
				ico[off], ico[off+1], ico[off+2], ico[off+3] = 255, 255, 255, 255
			} else {
				ico[off], ico[off+1], ico[off+2], ico[off+3] = 0, 0, 0, 255 // 纯黑
			}
		}
	}
	return ico
}

// showAbout 关于对话框：Win32 原生 MessageBox（深色系统主题自动暗色，符合项目风格）。
// 显示程序名、数据库绝对路径、监听地址与选课大厅网址。
func showAbout(td trayData) {
	dbAbs := td.dbPath
	if abs, err := filepath.Abs(td.dbPath); err == nil {
		dbAbs = abs
	}
	body := "至道选课自动化\n\n数据库路径：\n" + dbAbs + "\n\n监听地址：http://localhost:" + td.port + "\n\n选课大厅：\n" + td.url
	showMessageBox("关于 至道选课自动化", body)
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
