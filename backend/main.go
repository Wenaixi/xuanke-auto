package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
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

	// 进程内配置中心（管理员可热重载：激活码开关 / Vision / 识别引擎与并发）
	rt := runtime.New(runtime.Config{
		ActivationEnabled:  cfg.ActivationCodesEnabled,
		VisionBaseURL:      cfg.SFBaseURL,
		VisionAPIKey:       cfg.SFAPIKey,
		VisionModel:        cfg.SFModel,
		CaptchaEngine:      config.CaptchaEngineDefault(), // 默认 ddddocr 本地识别（免密钥），vision 云识别需显式配置
		CaptchaConcurrency: 1,
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
				// vision_key 严格要求加密存储（enc: 前缀），彻底拒绝旧版未加密明文
				if strings.HasPrefix(v, "enc:") {
					if plain, err := decrypt(strings.TrimPrefix(v, "enc:")); err == nil {
						c.VisionAPIKey = plain
					} else {
						log.Printf("[main] 解密 vision_key 失败，已忽略: %v", err)
					}
				} else {
					log.Printf("[main] 警告：发现未加密的旧版 vision_key，已彻底拒绝加载（不兼容旧数据）")
				}
			}
			if v, ok := kv["vision_model"]; ok {
				c.VisionModel = v
			}
			if v, ok := kv["captcha_engine"]; ok {
				c.CaptchaEngine = v
			}
			if v, ok := kv["captcha_fallback"]; ok {
				c.CaptchaFallback = v == "true"
			}
			if v, ok := kv["captcha_concurrency"]; ok {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					c.CaptchaConcurrency = n
				}
			}
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

	// 系统托盘（Windows 桌面 / Linux 桌面 CGO=1）——常驻托盘，右键菜单
	// 打开浏览器/关于/退出；Docker/服务器（Linux CGO=0）无托盘，服务照常启动。
	// 托盘就绪后放行 main 继续（避免图标一闪而过）；关闭自动开浏览器，改为
	// 托盘「打开浏览器」手动打开（托盘应用习惯）。
	tray := trayData{url: "http://localhost:" + cfg.Port, dbPath: cfg.DBPath, port: cfg.Port}
	runTray(tray)
	log.Printf("[main] 托盘已就绪：右键「打开浏览器」访问选课大厅，或浏览器直接访问 http://localhost%s", ":"+cfg.Port)

	// 全局重登频率闸门的分钟推进器：每 30 秒检查一次窗口翻页，翻页时放行队列中的重登。
	go func() {
		tk := time.NewTicker(30 * time.Second)
		defer tk.Stop()
		for range tk.C {
			accts.GatePump()
		}
	}()

	// 会话库（12 小时过期；后台周期清扫过期会话与票据）
	sessions := session.New(12 * time.Hour)
	defer sessions.Close()

	mux := http.NewServeMux()
	apiHandler := api.Register(mux, st, sched, accts, sessions, cfg.AdminToken,
		cfg.AdminName, rt.Get().ActivationEnabled, encrypt, decrypt, rt)
	mux.Handle("/", web.SpaHandler())

	addr := ":" + cfg.Port
	log.Printf("[main] 至道选课自动化服务启动: http://localhost%s（激活码机制: %v）", addr, cfg.ActivationCodesEnabled)
	log.Printf("[main] 管理员登录：账号 %s，口令见 data/.env 的 XUANKE_ADMIN_TOKEN", adminNameOrDefault(cfg.AdminName))
	// 自动开浏览器改由托盘「打开浏览器」菜单触发（桌面带托盘场景）；
	// 无托盘场景（Docker/Linux CGO=0）仍自动打开一次（保持原行为）。
	openBrowser("http://localhost" + addr)
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
	// 托盘「退出」= 整进程退出：mQuit 点击 → quitApplication() 先优雅关服务
	// 再退图标。srv.Shutdown 使本行 ListenAndServe 返回 ErrServerClosed，main
	// 解锁自然退出（此前只 systray.Quit() 会让托盘消失而服务活挂后台，见
	// tray_quit_test.go M86-01 回归钉）。systrayQuit 另一半由托盘文件注入
	// （systray.Quit）；无托盘平台不注入、quitApplication 判 nil 跳过。
	// 尽力优雅语义：进程退出会强杀在飞 goroutine（含 spawnChain 网络往返），
	// 最后时刻的提交结果以重启后重试为准（RestoreDone 只恢复已落库的成功）。
	srvShutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[main] 服务优雅关闭异常: %v", err)
		}
	}
	setExitActions(srvShutdown, nil)
	if err := srv.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			log.Printf("[main] 托盘「退出」触发，服务已优雅关闭")
			return
		}
		log.Fatalf("服务启动失败: %v", err)
	}
}

// adminNameOrDefault 管理员账号名（配置为空时默认 admin）。
func adminNameOrDefault(name string) string {
	if name == "" {
		return "admin"
	}
	return name
}
