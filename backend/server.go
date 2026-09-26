package main

import (
	"context"
	"errors"
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

// Started 服务启动结果：监听地址 + 优雅关服入口 + 退出原因通道。
// 桌面 main 与安卓平台入口共用同一套引擎初始化，仅形态收尾不同。
type Started struct {
	// Addr 实际监听地址（":3091"）；供日志与服务启动后提示。
	Addr string
	// Shutdown 优雅关服（等服务在飞请求结束），托盘「退出」与安卓销毁时调用。
	Shutdown func()
	// Cleanup 关闭 DB/会话等底层资源（Android 常驻共享库 main 永不返回，
	// 库内 defer 不触发，必须由 Java 销毁路径显式调用；桌面 main 的 defer 仍兜底）。
	Cleanup func()
	// ErrCh 服务退出原因（ErrServerClosed 表示被 Shutdown 主动关闭）。
	ErrCh <-chan error
}

// runServer 组装并启动整套服务引擎：数据库 / 凭据加密 / 多账号注册表 / 运行时配置
// / 调度器（开窗探测与按账号提交）/ 会话库 / API 路由 / 前端 SPA。
// 与桌面 main 的分工：本函数只管「引擎 + HTTP 服务」；托盘 / 开浏览器等桌面形态
// 专属物由 main 负责（安卓由 platform_android.go 的入口负责）。
func runServer(cfg config.Config) *Started {
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

	// d/err 生命周期归 Started.Cleanup（Android 常驻共享库：main 不返回，
// 库内 defer 永不触发，DB/会话需随 Java 销毁显式关闭）。桌面 main 用 defer
// 仍稳妥——Cleanup 由托盘退出路径调用，两形态共用同一收尾。
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
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

	// 进程内配置中心（管理员可热重载：激活码开关 / Vision / 识别引擎与并发）
	rt := runtime.New(runtime.Config{
		ActivationEnabled:  cfg.ActivationCodesEnabled,
		VisionBaseURL:      cfg.SFBaseURL,
		VisionAPIKey:       cfg.SFAPIKey,
		VisionModel:        cfg.SFModel,
		CaptchaEngine:      config.CaptchaEngineDefault(), // 默认 ddddocr 本地识别（免密钥），vision 云识别需显式配置
		CaptchaConcurrency: 1,
	})
	// 从数据库恢复管理员上次的运行时配置（优先于环境变量，覆盖持久化值）。
	// 键名与解析规则由 runtime 包的配置表统一定义——落库侧（api PUT）与本还原侧
	// 读同一张表，新增字段不可能只改一处而漏另一处。
	if kv, err := st.LoadSettings(); err != nil {
		log.Printf("[main] 读取运行时配置失败: %v", err)
	} else if len(kv) > 0 {
		rt.Update(func(c *runtime.Config) {
			runtime.ApplySettings(c, kv, decrypt)
		})
	}

	// 调度器（窗口到点立即探测 + 课程快照 + 按账号并发提交）
	// 开放时间不做任何配置注入：平台 beginTimes 自动识别是唯一事实源（open_time 零值）。
	sched := scheduler.New(accts, st, time.Time{}, 300*time.Millisecond)

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
			// 重启恢复目标用 RestoreTargets（不清 refused）——
			// 此前用 SetTargetsForAccount 会 delete refused + 删库行，重启后手动退选
			// 记录全丢、自动引擎重新抢回（恢复顺序抵消）。
			sched.RestoreTargets(a, ts)
		}
	}
	// 恢复已手动退选记录——RestoreTargets 不清库行，此处 LoadRefused
	// 仍能读到全部退选；RestoreRefused 注入内存后不被任何后续步骤覆盖。
	if refused, err := st.LoadRefused(); err != nil {
		log.Printf("[main] 读取已退选记录失败: %v", err)
	} else if len(refused) > 0 {
		sched.RestoreRefused(refused)
	}
	sched.Start()

	// 会话库（12 小时过期；后台周期清扫过期会话与票据）
	sessions := session.New(12 * time.Hour)

	// 全局重登频率闸门的分钟推进器：每 30 秒检查一次窗口翻页，翻页时放行队列中的重登。
	// 桌面与安卓共用此初始化路径，故闸门推进也统一放在这里（accts 在此创建）。
	go func() {
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		for range tk.C {
			accts.GatePump()
		}
	}()

	mux := http.NewServeMux()
	apiHandler := api.Register(api.Options{
		Mux: mux, Store: st, Sched: sched, Accounts: accts, Sessions: sessions,
		AdminToken: cfg.AdminToken, AdminName: cfg.AdminName,
		ActivationEnabled: rt.Get().ActivationEnabled, Encrypt: encrypt, Decrypt: decrypt, Runtime: rt,
	})
	mux.Handle("/", web.SpaHandler())

	addr := ":" + cfg.Port
	log.Printf("[main] 至道选课自动化服务启动: http://localhost%s（激活码机制: %v）", addr, cfg.ActivationCodesEnabled)
	log.Printf("[main] 管理员登录：账号 %s，口令见 data/.env 的 XUANKE_ADMIN_TOKEN", adminNameOrDefault(cfg.AdminName))

	// http.Server 显式超时——公网部署时 slowloris/慢速 POST
	// 不再能占用 goroutine 与连接池饿死调度器 tick 与健康检查。
	srv := &http.Server{
		Addr:              addr,
		Handler:           apiHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	started := &Started{Addr: addr}
	started.Cleanup = func() {
		// DB 与会话库的关闭收口：桌面死库 defer 与安卓 Java 销毁都会到达这里。
		// 幂等（多次调仅首回收）——sessions.Close 与 d.Close 内部都保证幂等。
		sessions.Close()
		d.Close()
	}
	started.Shutdown = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[main] 服务优雅关闭异常: %v", err)
		}
	}
	ch := make(chan error, 1)
	started.ErrCh = ch
	go func() {
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			ch <- http.ErrServerClosed
			return
		}
		if err != nil {
			log.Fatalf("服务启动失败: %v", err)
		}
		ch <- err
	}()
	return started
}

// adminNameOrDefault 管理员账号名（配置为空时默认 admin）。
func adminNameOrDefault(name string) string {
	if name == "" {
		return "admin"
	}
	return name
}
