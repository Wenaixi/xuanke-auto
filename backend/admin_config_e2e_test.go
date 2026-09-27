package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/store"
)

// loadSettingsFromDB 关服后重开同一个库读 settings（验证"落库即重启仍生效"）。
func loadSettingsFromDB(t *testing.T, path string) map[string]string {
	t.Helper()
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	kv, err := store.New(d).LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	return kv
}

// TestAdminConfigListenHotReloadEndToEnd 真实 HTTP 链路验证「管理员端改监听地址
// 立即生效」：管理员登录拿会话 → PUT /api/admin/config 改端口 → 新端口立刻可用、
// 旧端口关闭、响应回显新值、配置落库（重启仍生效）。
func TestAdminConfigListenHotReloadEndToEnd(t *testing.T) {
	port1, port2 := freePort(t), freePort(t)
	for port2 == port1 {
		port2 = freePort(t)
	}
	dbPath := t.TempDir() + "/t.db"
	cfg := config.Config{
		Port: port1, DBPath: dbPath,
		BaseURL: "https://example.invalid", SFBaseURL: "https://example.invalid", SFModel: "m",
		AdminToken: "admintok", ListenHost: "127.0.0.1",
	}
	srv := runServer(cfg)
	defer srv.Cleanup()
	if !waitDialable("127.0.0.1:"+port1, 3*time.Second) {
		t.Fatal("初始监听不可达")
	}
	base1 := "http://127.0.0.1:" + port1

	// 1) 管理员登录
	loginResp, err := http.Post(base1+"/api/login", "application/json",
		strings.NewReader(`{"account":"admin","password":"admintok"}`))
	if err != nil {
		t.Fatalf("登录请求失败: %v", err)
	}
	defer loginResp.Body.Close()
	var login struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil {
		t.Fatalf("登录响应解析失败: %v", err)
	}
	if login.Code != 0 || login.Data.Token == "" {
		t.Fatalf("管理员登录失败: %+v", login)
	}

	// 2) PUT 改监听端口（host 沿用 127.0.0.1）
	req, err := http.NewRequest(http.MethodPut, base1+"/api/admin/config",
		strings.NewReader(`{"listen_port":"`+port2+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.Data.Token)
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT 配置失败: %v", err)
	}
	defer putResp.Body.Close()
	var put struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ListenHost string `json:"listen_host"`
			ListenPort string `json:"listen_port"`
		} `json:"data"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&put); err != nil {
		t.Fatalf("PUT 响应解析失败: %v", err)
	}
	if put.Code != 0 {
		t.Fatalf("改监听端口应成功: %+v", put)
	}
	if put.Data.ListenHost != "127.0.0.1" || put.Data.ListenPort != port2 {
		t.Fatalf("响应必须回显新监听地址: %+v", put.Data)
	}

	// 3) 新端口立刻可用、旧端口已关闭（热重载，不重启）
	if !waitDialable("127.0.0.1:"+port2, 3*time.Second) {
		t.Fatal("新端口必须立刻可用")
	}
	if waitDialable("127.0.0.1:"+port1, 800*time.Millisecond) {
		t.Fatal("旧端口必须已关闭")
	}

	// 4) 落库：关服后重开同一 DB，settings 里已是新端口
	srv.Shutdown()
	srv.Cleanup()
	kv := loadSettingsFromDB(t, dbPath)
	if kv["listen_port"] != port2 {
		t.Fatalf("监听端口必须落库（重启仍生效）: %v", kv)
	}
}
