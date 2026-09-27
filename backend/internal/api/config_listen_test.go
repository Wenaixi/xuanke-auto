package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xuanke-auto/backend/internal/runtime"
)

// putConfig 直调 handleAdminConfig（本文件只覆盖监听配置的校验与重绑顺序；
// 会话鉴权由 requireAdminSession 的既有测试覆盖）。
func putConfig(t *testing.T, d *Deps, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/admin/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.handleAdminConfig(w, req)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON: %v (%s)", err, w.Body.String())
	}
	return w.Code, resp
}

// listenDeps 构造只关心监听配置的 Deps（真实 store/runtime + 可注入 Rebind）。
func listenDeps(t *testing.T, rebind func(host, port string) error, embedded bool) (*Deps, *testDeps) {
	t.Helper()
	td := newTestDeps(t)
	td.rt.Update(func(c *runtime.Config) {
		c.ListenHost = "127.0.0.1"
		c.ListenPort = "3091"
	})
	return &Deps{
		Runtime: td.rt, Store: td.store, Encrypt: td.enc,
		Rebind: rebind, PlatformEmbedded: embedded,
	}, td
}

// TestAdminConfigListenRebind 合法监听配置：真重绑一次 + 写入运行时 + 落库 settings
// （重启后仍是新地址）。
func TestAdminConfigListenRebind(t *testing.T) {
	var calls int
	var gotHost, gotPort string
	d, td := listenDeps(t, func(host, port string) error {
		calls++
		gotHost, gotPort = host, port
		return nil
	}, false)

	code, resp := putConfig(t, d, `{"listen_host":"192.168.1.10","listen_port":"8080"}`)
	if code != http.StatusOK || resp["code"].(float64) != 0 {
		t.Fatalf("合法监听配置应成功: code=%d resp=%v", code, resp)
	}
	if calls != 1 || gotHost != "192.168.1.10" || gotPort != "8080" {
		t.Fatalf("必须以新地址重绑一次，实际 calls=%d %s:%s", calls, gotHost, gotPort)
	}
	cfg := td.rt.Get()
	if cfg.ListenHost != "192.168.1.10" || cfg.ListenPort != "8080" {
		t.Fatalf("运行时配置未更新: %+v", cfg)
	}
	kv, err := td.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if kv["listen_host"] != "192.168.1.10" || kv["listen_port"] != "8080" {
		t.Fatalf("监听配置必须落库（否则重启回退）: %v", kv)
	}
}

// TestAdminConfigListenRejectsBadInput 非法值（带端口/scheme、空主机、越界/非数字端口）
// 一律拒绝，且不得触发重绑、不得改动生效配置。
func TestAdminConfigListenRejectsBadInput(t *testing.T) {
	calls := 0
	d, td := listenDeps(t, func(host, port string) error { calls++; return nil }, false)

	for _, body := range []string{
		`{"listen_host":"127.0.0.1:3091"}`,
		`{"listen_host":"http://xuanke.example.com"}`,
		`{"listen_host":""}`,
		`{"listen_port":"0"}`,
		`{"listen_port":"65536"}`,
		`{"listen_port":"abc"}`,
	} {
		if _, resp := putConfig(t, d, body); resp["code"].(float64) == 0 {
			t.Fatalf("非法监听配置必须被拒: %s", body)
		}
	}
	if calls != 0 {
		t.Fatalf("非法值不得触发重绑，实际 %d 次", calls)
	}
	if cfg := td.rt.Get(); cfg.ListenHost != "127.0.0.1" || cfg.ListenPort != "3091" {
		t.Fatalf("非法值不得改动生效配置: %+v", cfg)
	}
	if kv, _ := td.store.LoadSettings(); kv["listen_host"] == "127.0.0.1:3091" {
		t.Fatal("非法监听配置不得落库")
	}
}

// TestAdminConfigListenRebindFailureKeepsOld 重绑失败：HTTP 400 + 生效值不变 + 不落库
// （新地址起不来时绝不能留下"配置改了、服务没了"）。
func TestAdminConfigListenRebindFailureKeepsOld(t *testing.T) {
	d, td := listenDeps(t, func(host, port string) error {
		return errors.New("bind: 地址不可用")
	}, false)

	code, resp := putConfig(t, d, `{"listen_host":"0.0.0.0","listen_port":"8080"}`)
	if code != http.StatusBadRequest || resp["code"].(float64) == 0 {
		t.Fatalf("重绑失败必须写真实 400：code=%d resp=%v", code, resp)
	}
	if cfg := td.rt.Get(); cfg.ListenHost != "127.0.0.1" || cfg.ListenPort != "3091" {
		t.Fatalf("重绑失败不得改动生效配置: %+v", cfg)
	}
	if kv, _ := td.store.LoadSettings(); kv["listen_host"] == "0.0.0.0" {
		t.Fatal("重绑失败不得落库")
	}
}

// TestAdminConfigListenPlatformEmbeddedLocksPort APK 内置形态：端口不可改
// （Java 壳页面按当前端口连接，改了 App 内页面失联），监听地址仍可改。
func TestAdminConfigListenPlatformEmbeddedLocksPort(t *testing.T) {
	var gotHost, gotPort string
	d, _ := listenDeps(t, func(host, port string) error {
		gotHost, gotPort = host, port
		return nil
	}, true)

	if _, resp := putConfig(t, d, `{"listen_port":"8080"}`); resp["code"].(float64) == 0 {
		t.Fatal("APK 形态下改端口必须被拒")
	}
	if gotPort != "" {
		t.Fatalf("被拒的端口改动不得触发重绑，实际重绑到 %s", gotPort)
	}
	if _, resp := putConfig(t, d, `{"listen_host":"0.0.0.0"}`); resp["code"].(float64) != 0 {
		t.Fatalf("APK 形态下改监听地址应放行: %v", resp)
	}
	if gotHost != "0.0.0.0" || gotPort != "3091" {
		t.Fatalf("APK 改地址须沿用原端口，实际 %s:%s", gotHost, gotPort)
	}
}

// TestAdminConfigListenRejectedWithoutRebind 未注入 Rebind（测试直构等形态）时
// 明确拒绝，绝不"只改配置不重绑"造成重启前后不一致。
func TestAdminConfigListenRejectedWithoutRebind(t *testing.T) {
	d, td := listenDeps(t, nil, false)
	code, resp := putConfig(t, d, `{"listen_host":"0.0.0.0"}`)
	if code != http.StatusOK || resp["code"].(float64) == 0 {
		t.Fatalf("无 Rebind 时必须明确拒绝: code=%d resp=%v", code, resp)
	}
	if cfg := td.rt.Get(); cfg.ListenHost != "127.0.0.1" {
		t.Fatalf("被拒时不得改动配置: %+v", cfg)
	}
}
