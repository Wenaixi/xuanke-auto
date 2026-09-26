package main

// 托盘「退出」动作的共享注入点（无 build tag，四个托盘文件共用）。
// mQuit 点击 → quitApplication()：先 shutdownServer（srv.Shutdown，让 main 的
// ListenAndServe 返回 ErrServerClosed 解锁）再 systrayQuit（图标消失）。
// 顺序绝不可颠倒——先退图标会留下「服务还没关完图标就先没了」的窗口；
// 只退图标不关服务 = 托盘没了、HTTP 服务活挂后台（该缺陷修复后的形态）。
// 两个注入点均可测试替换；无托盘平台（linux CGO=0 / darwin 占位）恒 nil，
// quitApplication 判 nil 跳过，绝不 panic。

var (
	shutdownServer func()
	systrayQuit    func()
)

// setExitActions 注入真实现：main 注入「关服务」（srv.Shutdown），托盘文件注入
// 「退图标」（systray.Quit）。传 nil 表示不覆盖——无托盘平台不注入任何动作，
// quitApplication 判 nil 跳过，绝不 panic。
func setExitActions(shutdown func(), quit func()) {
	if shutdown != nil {
		shutdownServer = shutdown
	}
	if quit != nil {
		systrayQuit = quit
	}
}

// quitApplication 托盘「退出」菜单的统一出口：先优雅关服务、再退图标。
func quitApplication() {
	if shutdownServer != nil {
		shutdownServer()
	}
	if systrayQuit != nil {
		systrayQuit()
	}
}
