package main

import (
	"log"
	"net/http"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/api"
	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/secure"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
	"xuanke-auto/backend/web"
)

func main() {
	cfg := config.Load()

	// 公网安全：部署访问口令必填，否则拒绝启动
	if cfg.AdminToken == "" {
		log.Fatal("未设置部署访问口令：请设置环境变量 XUANKE_ADMIN_TOKEN 后启动")
	}
	if cfg.SFAPIKey == "" {
		log.Println("[main] 警告：未设置 SF_API_KEY，教务登录验证码识别将不可用")
	}

	// 数据库（拒绝旧版数据形状）
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()
	st := store.New(d)

	// 凭据加密主密钥（环境变量或 DB 旁 .master_key）
	masterKey, err := secure.LoadOrCreateKey(cfg.DBPath)
	if err != nil {
		log.Fatalf("初始化数据加密密钥失败: %v", err)
	}
	encrypt := func(s string) (string, error) { return secure.Encrypt(s, masterKey) }
	decrypt := func(s string) (string, error) { return secure.Decrypt(s, masterKey) }

	// 多账号客户端注册表（每账号独立会话）+ 重启恢复
	accts := accounts.New(cfg.BaseURL, zhidao.VisionConfig{
		BaseURL: cfg.SFBaseURL, APIKey: cfg.SFAPIKey, Model: cfg.SFModel,
	}, st)
	if creds, err := st.LoadCredentials(); err != nil {
		log.Printf("[main] 读取凭据失败: %v", err)
	} else if len(creds) > 0 {
		accts.Restore(creds, decrypt)
	}

	// 调度器（窗口到点立即探测 + 课程快照 + 按账号并发提交）
	openTime, err := scheduler.FormatOpenTime(cfg.OpenTime)
	if err != nil {
		log.Fatalf("开放时间配置错误: %v", err)
	}
	sched := scheduler.New(accts, st, openTime, 300*time.Millisecond)

	if success, err := st.LoadSuccess(); err != nil {
		log.Printf("[main] 读取成功记录失败: %v", err)
	} else if len(success) > 0 {
		sched.RestoreDone(success)
	}
	for _, a := range accts.Registered() {
		ts, err := st.LoadTargetsForAccount(a)
		if err != nil {
			log.Printf("[main] 读取账号 %s 目标失败: %v", a, err)
			continue
		}
		if len(ts) > 0 {
			sched.SetTargetsForAccount(a, ts)
		}
	}
	sched.Start()

	// 会话库（12 小时过期）
	sessions := session.New(12 * time.Hour)

	mux := http.NewServeMux()
	apiHandler := api.Register(mux, st, sched, accts, sessions, cfg.OpenTime, cfg.AdminToken, encrypt)
	mux.Handle("/", web.SpaHandler())

	addr := ":" + cfg.Port
	log.Printf("[main] 至道选课自动化服务启动: http://localhost%s（部署口令已启用）", addr)
	if err := http.ListenAndServe(addr, apiHandler); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
