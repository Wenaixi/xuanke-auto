package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"xuanke-auto/backend/internal/api"
	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/scheduler"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/web"
	"xuanke-auto/backend/internal/zhidao"
)

func main() {
	cfg := config.Load()

	// 数据库
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()
	st := store.New(d)

	// 至道客户端
	client := zhidao.New(cfg.BaseURL, zhidao.VisionConfig{
		BaseURL: cfg.SFBaseURL,
		APIKey:  cfg.SFAPIKey,
		Model:   cfg.SFModel,
	})

	// 已有会话注入：环境变量 XUANKE_TOKEN 优先（复用现有 token 无需重新登录）
	if cfg.Token != "" {
		client.SetCredentials("", "", cfg.Token)
		client.SetCookies(parseCookies(cfg.Cookies))
		if err := st.SaveTokenOnly(cfg.Token); err != nil {
			log.Printf("[main] 持久化注入 token 失败: %v", err)
		}
		log.Printf("[main] 已使用环境变量注入会话 token %s", tokenShort(cfg.Token))
	}
	// 重启恢复：账密/token/目标/已成功课程（环境变量未注入时）
	if cfg.Token == "" {
		if acct, pwd, token, err := st.LoadAccount(); err == nil && token != "" {
			client.SetCredentials(acct, pwd, token)
			// access_limit_cookie 为会话级 Cookie，不敏感，使用固定默认值
			client.SetCookies(map[string]string{
				"access_limit_cookie": "***REMOVED***",
				"zd_edu_cookie":       token,
			})
			// 老账号迁移：同步记录到多账号名表，保证前端账号下拉可用
			if acct != "" {
				if err := st.SaveAccountName(acct); err != nil {
					log.Printf("[main] 迁移老账号名失败: %v", err)
				}
			}
			log.Printf("[main] 已恢复保存的会话 token %s", tokenShort(token))
		} else if err != nil {
			log.Printf("[main] 读取账号失败: %v", err)
		}
	}
	targets, err := st.LoadTargets()
	if err != nil {
		log.Printf("[main] 读取目标失败: %v", err)
	}
	_, _, doneIDs, err := st.LoadState()
	if err != nil {
		log.Printf("[main] 读取任务状态失败: %v", err)
	}

	openTime, err := scheduler.FormatOpenTime(cfg.OpenTime)
	if err != nil {
		log.Fatalf("开放时间配置错误: %v", err)
	}

	sched := scheduler.New(client, st, openTime, 300*time.Millisecond)
	sched.RestoreDone(doneIDs)
	sched.SetTargets(targets)
	// 恢复各账号目标（按账号隔离）
	accts, err := st.ListAccounts()
	if err != nil {
		log.Printf("[main] 读取账号列表失败: %v", err)
	}
	for _, a := range accts {
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

	// 路由：API + 前端嵌入
	mux := http.NewServeMux()
	apiHandler := api.Register(mux, st, client, sched, cfg.OpenTime)
	mux.Handle("/", web.SpaHandler())

	addr := ":" + cfg.Port
	log.Printf("[main] 至道选课自动化服务启动: http://localhost%s", addr)
	if err := http.ListenAndServe(addr, apiHandler); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// parseCookies 解析 "k=v; k2=v2" 格式的 Cookie 字符串。
func parseCookies(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func tokenShort(t string) string {
	if len(t) <= 8 {
		return t
	}
	return t[:8] + "..."
}