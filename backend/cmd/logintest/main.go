package main

// login-test 批量登录测试工具：读取 data/xuanke.db 中加密保存的账密，
// 对指定账号逐个执行真实教务登录（RSA 加密 + Vision 验证码识别），
// 验证"保存的密码 + 登录链路"是否仍然有效，并输出每个账号的结果摘要。
// 用法：go run ./cmd/logintest --limit 5 --wait 40
// 说明：默认并发 1（平台对高并发登录敏感）；可选 --wait 控制两次登录间隔（秒）。

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/secure"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/zhidao"
)

func main() {
	limit := flag.Int("limit", 5, "最多测试几个账号")
	wait := flag.Int("wait", 40, "两次登录间隔秒数（平台登录限流，建议 ≥30）")
	flag.Parse()

	cfg := config.Load()

	// 打开数据库 + 解密主密钥（与主程序完全一致）
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()
	masterKey, err := secure.LoadOrCreateKey(cfg.DBPath)
	if err != nil {
		log.Fatalf("读取主密钥失败: %v", err)
	}
	decrypt := func(s string) (string, error) { return secure.Decrypt(s, masterKey) }

	st := store.New(d)
	creds, err := st.LoadCredentials()
	if err != nil {
		log.Fatalf("读取凭据失败: %v", err)
	}
	if len(creds) == 0 {
		log.Fatal("数据库没有任何账号凭据")
	}

	fmt.Printf("数据库共 %d 个账号，本次测试前 %d 个：\n", len(creds), *limit)
	// 识别引擎跟随运行时配置（与主程序 applyCaptchaRecognizerFor 一致）：
	// vision → 云识别；ddddocr → 优先内置原生 ddddocr，否则本地 Python，最后回退 Vision
	vision := zhidao.VisionConfig{BaseURL: cfg.SFBaseURL, APIKey: cfg.SFAPIKey, Model: cfg.SFModel}
	var recognizer zhidao.CaptchaRecognizer
	switch config.CaptchaEngineDefault() {
	case "ddddocr":
		if zhidao.NativeDdddOcrAvailable() {
			recognizer = zhidao.NewNativeDdddOcrRecognizer()
			fmt.Println("识别引擎：内置原生 ddddocr（免 Python / 免 API 密钥）")
		} else if zhidao.LocalDdddOcrAvailable("") {
			recognizer = zhidao.NewLocalDdddOcrRecognizer("")
			fmt.Println("识别引擎：本地 Python ddddocr")
		} else {
			recognizer = zhidao.NewVisionRecognizer(vision)
			fmt.Println("识别引擎：配置为 ddddocr 但无本地引擎，回退 Vision")
		}
	default:
		recognizer = zhidao.NewVisionRecognizer(vision)
		fmt.Println("识别引擎：硅基流动 Vision 云识别")
	}
	zhidao.NewCaptchaSemaphore(1) // 识别并发限流 1（与主程序默认一致）

	var passed, failed int
	// 参数边界防御——-limit ≤0 或超账号数时收敛到真实账号数（杜绝 slice 越界 panic）
	if *limit <= 0 || *limit > len(creds) {
		*limit = len(creds)
	}
	for i, cd := range creds {
		if i >= *limit {
			break
		}
		pwd, err := decrypt(cd.PasswordEnc)
		if err != nil {
			fmt.Printf("[%d/%d] 账号 %s 密码解密失败: %v\n", i+1, *limit, cd.Account, err)
			failed++
			continue
		}
		c := zhidao.New(cfg.BaseURL, vision)
		c.SetRecognizer(recognizer) // 显式注入当前生效识别引擎
		start := time.Now()
		token, err := c.Login(cd.Account, pwd)
		cost := time.Since(start)
		if err != nil {
			fmt.Printf("[%d/%d] 账号 %s 登录失败（%.1fs）: %v\n", i+1, *limit, cd.Account, cost.Seconds(), err)
			failed++
		} else {
			fmt.Printf("[%d/%d] 账号 %s 登录成功（%.1fs）: 新 token %s...\n", i+1, *limit, cd.Account, cost.Seconds(), token[:min(8, len(token))])
			passed++
		}
		if i < *limit-1 && *wait > 0 {
			fmt.Printf("    等待 %d 秒（平台登录限流）...\n", *wait)
			time.Sleep(time.Duration(*wait) * time.Second)
		}
	}
	fmt.Printf("\n结果：成功 %d，失败 %d\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func init() {
	// go run 时 cwd 可能不在仓库根：将数据目录定位到仓库根 data/
	if _, err := os.Stat(filepath.Join("data", "xuanke.db")); err == nil {
		_ = os.Setenv("XUANKE_DB", filepath.Join("data", "xuanke.db"))
	}
}
