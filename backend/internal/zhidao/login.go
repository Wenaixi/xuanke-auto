package zhidao

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// LoginEngine 登录引擎深模块（收权）：把「初始化会话 → 取验证码 → 识别 →
// RSA 加密 → 提交 → 刷新重试」完整登录链路收进一个模块，对外一个 Login 接口。
// 重试预算（识别≤3 + 提交≤2）与验证码刷新策略全在内部；网络/配置错误立即返回。
// 不共享 sharedTransport（与旧 Client.Login 同语义：每轮独立 jar 会话、回落
// DefaultTransport——双份网络栈是既有事实，本卷不改）；会话 Cookie 与 token 收在
// 引擎内部（成功回调会话访问器供 Client.Login 读取后 SetCredentials）。
type LoginEngine struct {
	BaseURL string
	Vision  VisionConfig

	mu          sync.Mutex
	recognizer  CaptchaRecognizer // 引擎注入点：SetRecognizer 驱动（热切换）
	token       string            // 最近一次登录成功 token（供 Client.Login 读取）
	cookies     map[string]string // 登录会话 Cookie（zd_edu_cookie 等）
}

// NewLoginEngine 创建登录引擎（绑定 baseURL 与识别配置；识别引擎后续 SetRecognizer 注入）。
func NewLoginEngine(baseURL string, vc VisionConfig) *LoginEngine {
	return &LoginEngine{BaseURL: baseURL, Vision: vc, cookies: map[string]string{}}
}

// SetRecognizer 热切换识别引擎（同步模板 VisionConfig.recognizer——与 accounts.Manager
// SetRecognizer 同款"绝不挥动引擎切换"语义，模板带引擎供后续新建客户端）。
func (e *LoginEngine) SetRecognizer(r CaptchaRecognizer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recognizer = r
	e.Vision = e.Vision.WithRecognizer(r)
}

// SetVision 更新视觉识别配置（保留当前引擎，与 Client.SetVision 同款语义）。
func (e *LoginEngine) SetVision(vc VisionConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	vc = vc.WithRecognizer(e.Vision.Recognizer())
	e.Vision = vc
}

// Token 返回最近一次登录成功 token（空=尚未登录成功）。
func (e *LoginEngine) Token() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.token
}

// Login 完整登录：每轮独立 jar 会话（登录页 Cookie 与验证码绑定）；识别失败/提交被拒
// 刷新验证码重试，最多 maxCaptchaAttempts 次；网络/配置错误立即返回（无谓重试只累积
// 平台限流）。成功返回 token（并写进引擎内部供 Token() 读取）。
func (e *LoginEngine) Login(account, password string) (string, error) {
	const maxCaptchaAttempts = 3 // 验证码识别最大次数（识别失败/提交被拒各刷新一次）
	var lastErr error
	for attempt := 1; attempt <= maxCaptchaAttempts; attempt++ {
		// 每个 attempt 使用独立会话：登录页 Cookie 与验证码绑定
		jar, _ := cookiejar.New(nil)
		sess := &http.Client{Timeout: 15 * time.Second, Jar: jar}
		ua := loginUserAgent

		// 1. 初始化会话（失败即返回：无谓重试只会累积平台限流）
		if err := fetchLoginPage(sess, ua, e.BaseURL); err != nil {
			return "", fmt.Errorf("初始化登录会话失败: %w", err)
		}

		// 2. 取验证码图片
		img, err := fetchCaptchaImage(sess, ua, e.BaseURL)
		if err != nil {
			return "", fmt.Errorf("获取验证码失败: %w", err)
		}

		// 3. 识别（识别失败 → 刷新验证码换一次，最多 maxCaptchaAttempts 次）
		e.mu.Lock()
		vc := e.Vision
		rec := e.recognizer
		e.mu.Unlock()
		if rec != nil {
			vc = vc.WithRecognizer(rec)
		}
		captchaText, err := recognizeCaptcha(vc, img)
		if err != nil || strings.TrimSpace(captchaText) == "" {
			if err == nil {
				err = fmt.Errorf("识别结果为空")
			}
			lastErr = fmt.Errorf("第%d次验证码识别失败: %w", attempt, err)
			log.Printf("[login] 账号 %s 第%d次验证码识别失败（引擎 %T）：%v", account, attempt, vc.recognizer, err)
			continue
		}
		log.Printf("[login] 账号 %s 第%d次验证码识别成功（引擎 %T，识别 %d 位字符）", account, attempt, vc.recognizer, len(captchaText))

		// 4. 提交登录。验证码一次性：提交被拒（多为验证码过期）绝不带同一验证码重试，
		//    直接 continue 刷新验证码重识别（重复提交只会浪费平台限流额度）。
		identification, identErr := encryptIdentification(account, password)
		if identErr != nil {
			return "", identErr
		}
		token, submitErr := e.submitLogin(sess, ua, captchaText, identification)
		if submitErr != nil {
			lastErr = fmt.Errorf("第%d次验证码提交被拒: %w", attempt, submitErr)
			log.Printf("[login] 账号 %s 第%d次验证码提交被拒：%v", account, attempt, submitErr)
			continue // 验证码可能已失效：刷新验证码重识别
		}
		return token, nil
	}
	if lastErr != nil {
		log.Printf("[login] 账号 %s 登录失败（共 %d 次识别尝试）：%v", account, maxCaptchaAttempts, lastErr)
		return "", fmt.Errorf("登录失败：验证码识别 %d 次均未通过（%s）", maxCaptchaAttempts, lastErr)
	}
	return "", fmt.Errorf("登录失败")
}

// submitLogin 提交登录表单（form 编码：captcha/identification/uniqueId/priorityId）。
// 成功把 token 与会话 Cookie 写入引擎内部（Client.Login 经 Token() 读取后 SetCredentials）。
// 返回错误表示提交被拒（多为验证码过期），调用方刷新验证码重试。
func (e *LoginEngine) submitLogin(sess *http.Client, ua, captchaText, identification string) (string, error) {
	form := url.Values{}
	form.Set("captcha", captchaText)
	form.Set("identification", identification)
	form.Set("uniqueId", uniqueDeviceID(ua, time.Now()))
	// 契约微差：真实网站 `priorityId: localStorage["priorityId"]` 在学生首次登录
	// （未进 /home/menus）时 undefined，jQuery 表单编码静默丢弃该键；Go 端恒发
	// `priorityId=` 空串。平台解析"空串"与"缺键"等价（不触发切换用户），学生登录
	// 本就无真值，两形态无实质差异——保留空串（行为零变化），载明语义即可。
	form.Set("priorityId", "")
	req, err := http.NewRequest(http.MethodPost, e.BaseURL+"/login/doLogin",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", e.BaseURL+"/login")
	resp, err := sess.Do(req)
	if err != nil {
		return "", fmt.Errorf("提交登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := readBody(resp)
	if err != nil {
		return "", fmt.Errorf("读取登录响应失败: %w", err)
	}

	var j struct {
		Code  int    `json:"code"`
		IsOk  bool   `json:"isOk"`
		Token string `json:"token"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", fmt.Errorf("登录响应解析失败: %w", err)
	}
	if !j.IsOk || j.Token == "" {
		return "", fmt.Errorf("登录被拒绝: %s", j.Msg)
	}
	e.mu.Lock()
	e.token = j.Token
	if e.cookies == nil {
		e.cookies = map[string]string{}
	}
	e.cookies["zd_edu_cookie"] = j.Token
	if u, _ := url.Parse(e.BaseURL); u != nil {
		for _, ck := range sess.Jar.Cookies(u) {
			if ck.Name != "" && ck.Value != "" {
				e.cookies[ck.Name] = ck.Value
			}
		}
	}
	if _, ok := e.cookies["access_limit_cookie"]; !ok {
		e.cookies["access_limit_cookie"] = "1" // 占位补充（与旧 submitLogin 同语义）
	}
	e.mu.Unlock()
	return j.Token, nil
}

// SnapshotCookies 返回引擎持有的登录会话 Cookie 副本（供 Client.Login 落 client.cookies）。
func (e *LoginEngine) SnapshotCookies() map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]string, len(e.cookies))
	for k, v := range e.cookies {
		out[k] = v
	}
	return out
}

// CurrentRecognizerExportedForTest 测试专用：返回引擎当前识别器（同包测试直读字段也可，
// 此处仅为 TestNewDoesNotSilentlyBuildVision 的跨文件清晰表达，正式代码不调用）。
func (e *LoginEngine) CurrentRecognizerExportedForTest() CaptchaRecognizer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.recognizer
}