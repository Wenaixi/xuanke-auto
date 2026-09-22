package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// 托盘「退出」语义 = 整进程退出（M86-01 回归钉）。
// 修复前：mQuit 分支只 systray.Quit()，systray.Run 返回仅结束非 main goroutine，
// main 仍阻塞在 ListenAndServe——托盘没了、服务活挂后台、无图标可再打开。
// 修复后：mQuit 分支先 shutdownServer（srv.Shutdown）再退图标，ListenAndServe
// 返回 ErrServerClosed，main 解锁、进程退出。

// TestQuitApplicationCallsShutdownThenSystray：退出处理必须先关服务再退图标，
// 顺序倒置会留下服务未关完就消失图标的窗口。注入 fake 断言顺序与次数。
func TestQuitApplicationCallsShutdownThenSystray(t *testing.T) {
	var order []string
	shutdownServer = func() { order = append(order, "shutdown") }
	systrayQuit = func() { order = append(order, "systray") }
	t.Cleanup(func() {
		shutdownServer, systrayQuit = nil, nil
	})

	quitApplication()

	if len(order) != 2 || order[0] != "shutdown" || order[1] != "systray" {
		t.Fatalf("期望先 shutdown 再 systray，实际顺序 %v", order)
	}
}

// TestShutdownServerUnblocksListenAndServe：注入真实 srv.Shutdown 后，
// Serve（ListenAndServe 的等价形态）必须返回 ErrServerClosed——
// main 的阻塞点解锁、进程随 main.main 返回而退出。这是退出链路的可测核心。
func TestShutdownServerUnblocksListenAndServe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	srv := &http.Server{Handler: http.NewServeMux()}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	shutdownServer = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
	t.Cleanup(func() { shutdownServer = nil })

	quitApplication()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("期望 ErrServerClosed，实际 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("srv.Shutdown 后 Serve 未返回——main 将永久阻塞在 ListenAndServe")
	}
}

// TestQuitApplicationNilSafe：无托盘平台（linux CGO=0 / darwin）不注入
// 两个回调，quitApplication 判 nil 跳过，绝不 panic。
func TestQuitApplicationNilSafe(t *testing.T) {
	shutdownServer, systrayQuit = nil, nil
	quitApplication() // 不 panic 即通过
}
