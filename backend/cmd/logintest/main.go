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
	// 识别引擎跟随配置与 XUANKE_CAPTCHA_FALLBACK env（主程序同款解析逻辑，
	// 但主程序的开关来自管理员后台配置并落库 runtime.Config.CaptchaFallback，
	// 本独立命令行工具无后台概念，用 env 控制同款行为）：
	// 兜底默认关闭（互不回退）；开启后 ddddocr 不可用 → Vision、Vision 无密钥 → ddddocr。
	vision := zhidao.VisionConfig{BaseURL: cfg.SFBaseURL, APIKey: cfg.SFAPIKey, Model: cfg.SFModel}
	fallback := os.Getenv("XUANKE_CAPTCHA_FALLBACK") == "true"
	recognizer := resolveLoginTestEngine(vision, fallback)
	if recognizer == nil {
		log.Fatalf("识别不可用：无可用识别引擎（配置 %s，兜底 %v）。请安装 ddddocr 或配置 SF_API_KEY。", config.CaptchaEngineDefault(), fallback)
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

// resolveLoginTestEngine 登录测试工具本地的引擎解析（与主程序 resolveCaptchaRecognizer 同构，
// 但独立实现避免把 api 包牵进 cmd 依赖）：ddddocr → 原生尝试 → 本机 Python → 兜底；
// vision → 有密钥直接用 → 兜底回退 ddddocr。返回 nil 表示无可用引擎。
func resolveLoginTestEngine(vision zhidao.VisionConfig, fallback bool) zhidao.CaptchaRecognizer {
	local := func() zhidao.CaptchaRecognizer {
		if zhidao.NativeDdddOcrAvailable() {
			if r := zhidao.NewNativeDdddOcrRecognizer(); r != nil {
				fmt.Println("识别引擎：内置原生 ddddocr（免 Python / 免 API 密钥）")
				return r
			}
		}
		if zhidao.LocalDdddOcrAvailable("") {
			fmt.Println("识别引擎：本地 Python ddddocr")
			return zhidao.NewLocalDdddOcrRecognizer("")
		}
		return nil
	}
	if config.CaptchaEngineDefault() == "vision" {
		if vision.APIKey != "" {
			fmt.Println("识别引擎：OpenAI 兼容视觉 API 云识别")
			return zhidao.NewVisionRecognizer(vision)
		}
		if fallback {
			if r := local(); r != nil {
				fmt.Println("（配置为 Vision 但无密钥，按兜底开关回退 ddddocr）")
				return r
			}
		}
		fmt.Println("识别不可用：配置为 Vision 但无 API 密钥，且本机无 ddddocr 可回退")
		return nil
	}
	// 配置为 ddddocr
	if r := local(); r != nil {
		return r
	}
	if fallback && vision.APIKey != "" {
		fmt.Println("识别引擎：配置为 ddddocr 但本机无引擎，按兜底开关回退 Vision")
		return zhidao.NewVisionRecognizer(vision)
	}
	fmt.Println("识别不可用：配置为 ddddocr 但引擎缺失（未开启兜底或 Vision 无密钥）")
	return nil
}
