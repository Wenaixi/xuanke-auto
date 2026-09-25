package main

import (
	"log"

	"xuanke-auto/backend/internal/config"
)

func main() {
	cfg := config.Load()

	// 桌面形态：引擎 + HTTP 服务由 runServer 启动（与安卓共用同一套初始化）。
	// runServer 失败即 log.Fatal；成功返回 Started（含监听地址 / 优雅关服 / 退出原因）。
	srv := runServer(cfg)

	// 系统托盘（Windows 桌面 / Linux 桌面 CGO=1）——常驻托盘，右键菜单
	// 打开浏览器/关于/退出；Docker/服务器（Linux CGO=0）无托盘，服务照常启动。
	// 托盘就绪后放行 main 继续（避免图标一闪而过）；关闭自动开浏览器，改为
	// 托盘「打开浏览器」手动打开（托盘应用习惯）。
	tray := trayData{url: "http://localhost:" + cfg.Port, dbPath: cfg.DBPath, port: cfg.Port}
	runTray(tray)
	log.Printf("[main] 托盘已就绪：右键「打开浏览器」访问选课大厅，或浏览器直接访问 http://localhost%s", ":"+cfg.Port)

	// 自动开浏览器改由托盘「打开浏览器」菜单触发（桌面带托盘场景）；
	// 无托盘场景（Docker/Linux CGO=0）仍自动打开一次（保持原行为）。
	openBrowser("http://localhost" + srv.Addr)

	// 托盘「退出」= 整进程退出：mQuit 点击 → quitApplication() 先优雅关服务
	// 再退图标。srv.Shutdown 使 runServer 的 ListenAndServe 返回 ErrServerClosed，
	// main 在退出原因通道收尾自然退出。尽力优雅语义：进程退出会强杀在飞
	// goroutine（含 spawnChain 网络往返），最后时刻的提交结果以重启后重试为准
	// （RestoreDone 只恢复已落库的成功）。
	<-srv.ErrCh
}
