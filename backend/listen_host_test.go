package main

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"xuanke-auto/backend/internal/config"
)

// freePort 取一个当前空闲的回环端口（关掉后交给 runServer；测试串行执行，
// 竞态窗口可忽略）。
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// localNonLoopbackIP 本机第一个非回环 IPv4（「只绑回环」的反证需要它）。
func localNonLoopbackIP(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
			return ipn.IP.String()
		}
	}
	return ""
}

// waitDialable 轮询式探测（条件等待而非固定睡眠）：budget 内任一次连上即真。
func waitDialable(addr string, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestRunServerBindsListenHost 实测监听行为（不是读代码推断）：
// ListenHost=127.0.0.1（默认）时只有回环可达、本机非回环 IP 不可达；
// ListenHost=0.0.0.0 时非回环 IP 也可达（局域网/公网部署路径）。
func TestRunServerBindsListenHost(t *testing.T) {
	nonLoop := localNonLoopbackIP(t)
	for _, tc := range []struct {
		host        string
		wantNonLoop bool
	}{
		{"127.0.0.1", false},
		{"0.0.0.0", true},
	} {
		t.Run(tc.host, func(t *testing.T) {
			port := freePort(t)
			cfg := config.Config{
				Port:            port,
				DBPath:          t.TempDir() + "/t.db",
				PlatformBaseURL: "https://example.invalid",
				SFBaseURL:       "https://example.invalid",
				SFModel:         "m",
				AdminToken:      "tok",
				ListenHost:      tc.host,
			}
			srv := runServer(cfg)
			defer srv.Shutdown()
			defer srv.Cleanup()

			if !waitDialable("127.0.0.1:"+port, 3*time.Second) {
				t.Fatal("回环地址必须可达")
			}
			if nonLoop == "" {
				t.Skip("本机无可用非回环 IPv4，跳过非回环反证")
			}
			if got := waitDialable(nonLoop+":"+port, 600*time.Millisecond); got != tc.wantNonLoop {
				t.Fatalf("ListenHost=%s 时 %s:%s 可达=%v，期望 %v", tc.host, nonLoop, port, got, tc.wantNonLoop)
			}
		})
	}
}

// TestStartedRebindHotSwitch 端到端验证监听热切换：新地址起来后旧地址必须关闭、
// 重绑失败时旧监听继续服务（不能"配置改了、服务没了"）、关服仍能通过 ErrCh 收尾。
func TestStartedRebindHotSwitch(t *testing.T) {
	port1, port2 := freePort(t), freePort(t)
	for port2 == port1 {
		port2 = freePort(t)
	}
	cfg := config.Config{
		Port:            port1,
		DBPath:          t.TempDir() + "/t.db",
		PlatformBaseURL: "https://example.invalid",
		SFBaseURL:       "https://example.invalid",
		SFModel:         "m",
		AdminToken:      "tok",
		ListenHost:      "127.0.0.1",
	}
	srv := runServer(cfg)
	defer srv.Cleanup()
	if srv.Rebind == nil {
		t.Fatal("Started.Rebind 必须已装配")
	}
	if !waitDialable("127.0.0.1:"+port1, 3*time.Second) {
		t.Fatal("初始监听必须可达")
	}

	if err := srv.Rebind("127.0.0.1", port2); err != nil {
		t.Fatalf("热重绑应成功: %v", err)
	}
	if !waitDialable("127.0.0.1:"+port2, 3*time.Second) {
		t.Fatal("重绑后新地址必须可达")
	}
	if waitDialable("127.0.0.1:"+port1, 800*time.Millisecond) {
		t.Fatal("重绑后旧地址必须不可达（旧监听未关闭）")
	}

	// 不可用地址（域名解析失败）必须报错，且旧监听继续服务
	if err := srv.Rebind("256.256.256.256", port2); err == nil {
		t.Fatal("非法地址必须返回错误")
	}
	if !waitDialable("127.0.0.1:"+port2, 2*time.Second) {
		t.Fatal("重绑失败后旧监听必须仍在服务")
	}

	srv.Shutdown()
	select {
	case e := <-srv.ErrCh:
		if !errors.Is(e, http.ErrServerClosed) {
			t.Fatalf("关服后 ErrCh 期望 ErrServerClosed，实际 %v", e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown 后 ErrCh 未收到结束信号（main 会永久阻塞）")
	}
}

// TestDisplayHost 提示/日志主机名：通配地址与空值显示 localhost，其余照实。
func TestDisplayHost(t *testing.T) {
	for in, want := range map[string]string{
		"":                   "localhost",
		"0.0.0.0":            "localhost",
		"::":                 "localhost",
		"[::]":               "localhost",
		"127.0.0.1":          "127.0.0.1",
		"192.168.1.10":       "192.168.1.10",
		"xuanke.example.com": "xuanke.example.com",
	} {
		if got := displayHost(in); got != want {
			t.Fatalf("displayHost(%q)=%q，期望 %q", in, got, want)
		}
	}
}
