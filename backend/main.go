package main

import (
	"log"
	"net/http"
	"time"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/api"
	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/secure"
	"xuanke-auto/backend/internal/session"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
	"xuanke-auto/backend/web"
)

func main() {
	cfg := config.Load()

	// 公网安全：管理口令必填（用于生成激活码），否则拒绝启动
	if cfg.AdminToken == "" {
		log.Fatal("未设置管理口令：请在 data/.env 中填写 XUANKE_ADMIN_TOKEN（或设置同名环境变量）后启动")
	}
	if cfg.SFAPIKey == "" {
		log.Println("[main] 警告：未设置 SF_API_KEY，教务登录验证码识别将不可用")
	}
	if cfg.ActivationCodesEnabled {
		log.Printf("[main] 激活码机制已启用（XUANKE_ACTIVATION=off 可完全关闭）")
	} else {
		log.Printf("[main] 激活码机制已关闭：账号登录后直接进入系统")
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

	// 进程内配置中心（管理员可热重载：激活码开关 / Vision / 开放时间）
	rt := runtime.New(runtime.Config{
		ActivationEnabled: cfg.ActivationCodesEnabled,
		VisionBaseURL:     cfg.SFBaseURL,
		VisionAPIKey:      cfg.SFAPIKey,
		VisionModel:       cfg.SFModel,
		OpenTime:          cfg.OpenTime,
	})
	// 从数据库恢复管理员上次的运行时配置（优先于环境变量，覆盖持久化值）
	if kv, err := st.LoadSettings(); err != nil {
		log.Printf("[main] 读取运行时配置失败: %v", err)
	} else if len(kv) > 0 {
		rt.Update(func(c *runtime.Config) {
			if v, ok := kv["activation_enabled"]; ok {
				c.ActivationEnabled = v == "true"
			}
			if v, ok := kv["vision_base_url"]; ok {
				c.VisionBaseURL = v
			}
			if v, ok := kv["vision_key"]; ok {
				c.VisionAPIKey = v
			}
			if v, ok := kv["vision_model"]; ok {
				c.VisionModel = v
			}
			if v, ok := kv["open_time"]; ok {
				c.OpenTime = v
			}
		})
	}

	// 调度器（窗口到点立即探测 + 课程快照 + 按账号并发提交）
	openTime, err := scheduler.FormatOpenTime(rt.Get().OpenTime)
	if err != nil {
		log.Fatalf("开放时间配置错误: %v", err)
	}
	sched := scheduler.New(accts, st, openTime, 300*time.Millisecond)
	// 打开时间走运行时配置中心：管理员热改后无需重启，调度器立即按新时间判断窗口
	sched.SetOpenTimeFn(func() time.Time { return rt.Get().OpenTimeParsed })

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
	apiHandler := api.Register(mux, st, sched, accts, sessions, rt.Get().OpenTime, cfg.AdminToken,
		rt.Get().ActivationEnabled, encrypt, rt)
	mux.Handle("/", web.SpaHandler())

	addr := ":" + cfg.Port
	log.Printf("[main] 至道选课自动化服务启动: http://localhost%s（激活码机制: %v）", addr, cfg.ActivationCodesEnabled)
	if err := http.ListenAndServe(addr, apiHandler); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
