package zhidao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// sharedTransport 全局共享的高性能 HTTP 传输层：
// 1. MaxIdleConnsPerHost 扩容至 64（默认仅 2），多账号多发布并发提交零阻塞；
// 2. 启用 TCP KeepAlive 与 HTTP/2，闲置连接保持 120 秒，避免反复经历 1.9s TLS 握手。
var sharedTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          128,
	MaxIdleConnsPerHost:   64,
	IdleConnTimeout:       120 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// VisionConfig 硅基流动 Vision 验证码识别配置。
type VisionConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Client 至道平台 API 客户端。
// 所有请求在 URL 后附加 idToken 参数；token 失效（code=-1）时用保存的账密自动重登（最多一次）。
type Client struct {
	baseURL   string
	http      *http.Client
	account   string // 已保存账密（用于过期自动重登）
	password  string
	mu        sync.Mutex // 保护 token/cookie 读写
	token     string
	cookies   map[string]string // 附加 Cookie（access_limit_cookie 等）
	visionCfg VisionConfig
}

// New 创建客户端。绑定全局高性能连接池 sharedTransport。
func New(baseURL string, visionCfg VisionConfig) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout:   15 * time.Second,
			Transport: sharedTransport,
		},
		cookies:   make(map[string]string),
		visionCfg: visionCfg,
	}
}

// Prewarm 静默轻量请求预热底层 TCP 与 TLS 连接池。
// 发送一条轻量 GET /login，只为完成握手并在连接池保留热连接。
func (c *Client) Prewarm() error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/login", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", loginUserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// SyncServerTime 请求服务器轻量接口并解析响应头 Date，计算服务器时钟与本地时间的偏差（serverTime - localTime）。
func (c *Client) SyncServerTime() (time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/login", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", loginUserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	dateStr := resp.Header.Get("Date")
	if dateStr == "" {
		return 0, fmt.Errorf("响应头缺失 Date 字段")
	}
	serverTime, err := http.ParseTime(dateStr)
	if err != nil {
		return 0, fmt.Errorf("解析服务器 Date 失败: %w", err)
	}
	// 中点时间近似：请求发出与响应到达的中间时刻
	rtt := time.Since(start)
	estimatedServerTime := serverTime.Add(rtt / 2)
	offset := estimatedServerTime.Sub(time.Now())
	return offset, nil
}

// SetCredentials 设置保存的账密与 token（重启恢复时调用）。
// token 即 zd_edu_cookie 的值，同步维护 cookie。
func (c *Client) SetCredentials(account, password, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.account = account
	c.password = password
	c.token = token
	c.cookies["zd_edu_cookie"] = token
}

// SetCookies 设置附加 Cookie（如 access_limit_cookie），用于复用现有会话。
func (c *Client) SetCookies(cookies map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range cookies {
		c.cookies[k] = v
	}
}

// SetVision 热更新验证码识别配置（管理员运行时修改立即生效，下次登录生效）。
func (c *Client) SetVision(cfg VisionConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.visionCfg = cfg
}

// Token 返回当前 token。
func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

// YearTerm 学年学期。
type YearTerm struct {
	SchoolYear int  `json:"schoolYear"`
	SchoolTerm int  `json:"schoolTerm"`
	Selected   bool `json:"selected"`
}

// Login 完整登录链路：GET /login 初始化会话，GET /login/captcha 取验证码，
// Vision 识别后 POST /login/doLogin。
//
// 平台限流安全设计（避免触发"登录失败次数过多"熔断）：
//   - 识别共 maxCaptchaAttempts 次：识别失败/识别码提交被拒，刷新验证码重新识别；
//   - 提交共 maxSubmitAttempts 次（提交被拒多为验证码过期，重试意义大）；
//   - 任一环节网络/配置错误立即返回，绝不无谓重试；识别结果为空视为识别失败，不提交。
//
// 成功后将 token 写入客户端并返回。
func (c *Client) Login(account, password string) (string, error) {
	const (
		maxCaptchaAttempts = 3 // 验证码识别最大次数（识别失败/提交被拒各刷新一次）
		maxSubmitAttempts  = 2 // 提交登录最大次数
	)
	var lastErr error
	for attempt := 1; attempt <= maxCaptchaAttempts; attempt++ {
		// 每个 attempt 使用独立会话：登录页 Cookie 与验证码绑定
		jar, _ := cookiejar.New(nil)
		sess := &http.Client{Timeout: 15 * time.Second, Jar: jar}
		ua := loginUserAgent

		// 1. 初始化会话（失败即返回：无谓重试只会累积平台限流）
		if err := fetchLoginPage(sess, ua, c.baseURL); err != nil {
			return "", fmt.Errorf("初始化登录会话失败: %w", err)
		}

		// 2. 取验证码图片
		img, err := fetchCaptchaImage(sess, ua, c.baseURL)
		if err != nil {
			return "", fmt.Errorf("获取验证码失败: %w", err)
		}

		// 3. Vision 识别（识别失败 → 刷新验证码换一次，最多 maxCaptchaAttempts 次）
		c.mu.Lock()
		vc := c.visionCfg
		c.mu.Unlock()
		captchaText, err := recognizeCaptcha(vc, img)
		if err != nil || strings.TrimSpace(captchaText) == "" {
			if err == nil {
				err = fmt.Errorf("识别结果为空")
			}
			lastErr = fmt.Errorf("第%d次验证码识别失败: %w", attempt, err)
			continue
		}

		// 4. 提交登录（提交被拒多为验证码过期，最多 maxSubmitAttempts 次）
		for submit := 1; submit <= maxSubmitAttempts; submit++ {
			identification, err := encryptIdentification(account, password)
			if err != nil {
				return "", err
			}
			token, err := c.submitLogin(sess, ua, captchaText, identification)
			if err != nil {
				lastErr = fmt.Errorf("第%d次验证码提交被拒: %w", attempt, err)
				break // 验证码可能已失效：刷新重识别
			}
			return token, nil
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("登录失败：验证码识别 %d 次均未通过（%s）", maxCaptchaAttempts, lastErr)
	}
	return "", fmt.Errorf("登录失败")
}

// ErrUnauthorized token 失效（code=-1）错误。
var ErrUnauthorized = fmt.Errorf("未登录，token 已失效")

// loginUserAgent 教务登录统一 UA（与页面 /login 一致）。
const loginUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"

// fetchLoginPage 初始化登录会话：GET /login 种下会话 Cookie。
func fetchLoginPage(sess *http.Client, ua string, baseURL string) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/login", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	resp, err := sess.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

// fetchCaptchaImage 取验证码图片（会话绑定校验码）。
func fetchCaptchaImage(sess *http.Client, ua string, baseURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/login/captcha?v=%d", baseURL, time.Now().UnixMilli()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", baseURL+"/login")
	resp, err := sess.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	img, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(img) == 0 {
		return nil, fmt.Errorf("验证码图片为空")
	}
	return img, nil
}

// submitLogin 提交登录表单并登记成功后的 token/cookie。
// 返回错误表示提交被拒（多为验证码过期），可由调用方刷新验证码重试。
func (c *Client) submitLogin(sess *http.Client, ua, captchaText, identification string) (string, error) {
	form := url.Values{}
	form.Set("captcha", captchaText)
	form.Set("identification", identification)
	form.Set("uniqueId", uniqueDeviceID(ua, time.Now()))
	form.Set("priorityId", "")
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/login/doLogin",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.baseURL+"/login")
	resp, err := sess.Do(req)
	if err != nil {
		return "", fmt.Errorf("提交登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
	c.mu.Lock()
	c.token = j.Token
	c.cookies["zd_edu_cookie"] = j.Token
	if u, _ := url.Parse(c.baseURL); u != nil {
		for _, ck := range sess.Jar.Cookies(u) {
			if ck.Name != "" && ck.Value != "" {
				c.cookies[ck.Name] = ck.Value
			}
		}
	}
	if _, ok := c.cookies["access_limit_cookie"]; !ok {
		c.cookies["access_limit_cookie"] = "***REMOVED***"
	}
	c.mu.Unlock()
	return j.Token, nil
}

// doRequest 统一请求入口：转发到至道并附加 idToken 与 Cookie。
// 不做自动重登：code=-1（token 失效）时返回 ErrUnauthorized，由调用方决定处理。
func (c *Client) doRequest(method, path string, body []byte, contentType string) ([]byte, error) {
	c.mu.Lock()
	tok := c.token
	cookies := make(map[string]string, len(c.cookies))
	for k, v := range c.cookies {
		cookies[k] = v
	}
	c.mu.Unlock()
	u := c.baseURL + path + "?idToken=" + url.QueryEscape(tok)
	req, err := http.NewRequest(method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if len(cookies) > 0 {
		parts := make([]string, 0, len(cookies))
		for k, v := range cookies {
			parts = append(parts, k+"="+v)
		}
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/admin.html")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var j struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	if j.Code == -1 {
		return data, fmt.Errorf("%w（%s）", ErrUnauthorized, extractMsg(data))
	}
	return data, nil
}

// ReloginIfNeeded 若当前 token 已失效，用保存账密重新登录并换新 token。
// 最多重登一次：返回 (是否已重登, 错误)。
func (c *Client) ReloginIfNeeded() (bool, error) {
	c.mu.Lock()
	acct, pwd := c.account, c.password
	c.mu.Unlock()
	if acct == "" {
		return false, fmt.Errorf("未登录且无保存账密")
	}
	if _, err := c.Login(acct, pwd); err != nil {
		return false, fmt.Errorf("自动重登失败: %w", err)
	}
	return true, nil
}

func extractMsg(data []byte) string {
	var j struct {
		Msg string `json:"msg"`
	}
	json.Unmarshal(data, &j)
	return j.Msg
}

// YearTerms 获取可选学年学期列表。
func (c *Client) YearTerms() ([]YearTerm, error) {
	body, err := c.doRequest(http.MethodPost, "/electives/select", nil, "")
	if err != nil {
		return nil, err
	}
	var j struct {
		Code              int        `json:"code"`
		Msg               string     `json:"msg"`
		CurrentYearTermList []YearTerm `json:"currentYearTermList"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, err
	}
	if j.Code != 0 {
		return nil, fmt.Errorf("学期列表错误 code=%d: %s", j.Code, j.Msg)
	}
	return j.CurrentYearTermList, nil
}

// Class 课程/选修班。
type Class struct {
	ID              int    `json:"id"`
	PublishID       int    `json:"publish_id"`
	CourseName      string `json:"course_name"`
	ClassName       string `json:"class_name"`
	TeacherNameList string `json:"teacher_name_list"`
	ClassroomName   string `json:"class_room_name"`
	LessonsDate     string `json:"lessons_date"`
	SelectedCount   int    `json:"selected_count"`
	AuditedCount    int    `json:"audited_count"`
	MaxCount        int    `json:"max_count"`
	PlanCount       int    `json:"plan_count"`
	CanSelect       bool   `json:"can_select"`
	BtnType         int    `json:"btn_type"`
	BtnText         string `json:"btn_text"`
	Title           string `json:"title"`
	ApplyDate       string `json:"apply_date"`
}

// Publish 选课发布。
type Publish struct {
	PublishID   int     `json:"publish_id"`
	PublishName string  `json:"publish_name"`
	BeginDate   string  `json:"begin_date"`
	InDateRange bool    `json:"in_date_range"`
	CanSelect   int     `json:"can_select"`
	HasSelected int     `json:"has_selected"`
	GroupCount  int     `json:"group_count"`
	TotalCount  int     `json:"total_count"`
	Classes     []Class `json:"classes"`
}

// ElectivesData 课程数据（含开放时间戳）。
type ElectivesData struct {
	BeginTimes []int64   `json:"begin_times"`
	Publishes  []Publish `json:"publishes"`
}

// FindElectives 查询当前学期课程数据。
// FindElectives 查询当前学期课程数据。
// 根据真实浏览器抓包分析：直接以空 POST 请求请求 findElectivesData，平台会自动返回当前激活学期的全量课程数据。
func (c *Client) FindElectives() (*ElectivesData, error) {
	// 1. 优先采用真实浏览器原生行为：直接 POST 空请求体，获取当前默认学期课程数据
	body, err := c.doRequest(http.MethodPost, "/electives/select/findElectivesData", nil, "")
	if err == nil {
		data, parseErr := parseElectives(body)
		if parseErr == nil && len(data.Publishes) > 0 {
			return data, nil
		}
	}

	// 2. 备选重试方案：尝试从学期列表获取当前选中学年学期后携带参数请求
	// 真实浏览器（select.js）下拉切换学期时由 jQuery $.ajax 将对象编码为
	// application/x-www-form-urlencoded，这里完全对齐该格式（而非 JSON body）。
	terms, termErr := c.YearTerms()
	if termErr == nil {
		for _, t := range terms {
			if t.Selected {
				termForm := url.Values{}
				termForm.Set("schoolYear", fmt.Sprintf("%d", t.SchoolYear))
				termForm.Set("schoolTerm", fmt.Sprintf("%d", t.SchoolTerm))
				retryBody, rErr := c.doRequest(http.MethodPost, "/electives/select/findElectivesData",
					[]byte(termForm.Encode()), "application/x-www-form-urlencoded")
				if rErr == nil {
					return parseElectives(retryBody)
				}
				break
			}
		}
	}

	if err != nil {
		return nil, err
	}
	return parseElectives(body)
}

// parseElectives 解析 findElectivesData 原始响应。
func parseElectives(body []byte) (*ElectivesData, error) {
	var raw struct {
		BeginTimes          []int64 `json:"beginTimes"`
		SelectElectivesData []struct {
			PublishID   int    `json:"publishId"`
			PublishName string `json:"publishName"`
			BeginDate   string `json:"beginDate"`
			InDateRange bool   `json:"inDateRange"`
			CanSelect   int    `json:"canSelect"`
			HasSelected int    `json:"hasSelected"`
			GroupCount  int    `json:"groupCount"`
			TotalCount  int    `json:"totalCount"`
			Classes     []Class `json:"electivesClassList"`
		} `json:"selectElectivesData"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := &ElectivesData{BeginTimes: raw.BeginTimes}
	for _, p := range raw.SelectElectivesData {
		out.Publishes = append(out.Publishes, Publish{
			PublishID:   p.PublishID,
			PublishName: p.PublishName,
			BeginDate:   p.BeginDate,
			InDateRange: p.InDateRange,
			CanSelect:   p.CanSelect,
			HasSelected: p.HasSelected,
			GroupCount:  p.GroupCount,
			TotalCount:  p.TotalCount,
			Classes:     p.Classes,
		})
	}
	return out, nil
}
// SelectClass 报名。返回平台消息（isOk 时含成功信息）。
func (c *Client) SelectClass(classID int) (string, error) {
	form := url.Values{}
	form.Set("classId", fmt.Sprintf("%d", classID))
	body, err := c.doRequest(http.MethodPost, "/electives/select/selectElectivesClass",
		[]byte(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", err
	}
	var j struct {
		Code int    `json:"code"`
		IsOk bool   `json:"isOk"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", err
	}
	if j.Code != 0 || !j.IsOk {
		return "", fmt.Errorf("报名失败: %s", j.Msg)
	}
	return j.Msg, nil
}

// ExitClass 退选。请求体与报名一致（form classId），路径为 exitElectivesClass。
func (c *Client) ExitClass(classID int) (string, error) {
	form := url.Values{}
	form.Set("classId", fmt.Sprintf("%d", classID))
	body, err := c.doRequest(http.MethodPost, "/electives/select/exitElectivesClass",
		[]byte(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", err
	}
	var j struct {
		Code int    `json:"code"`
		IsOk bool   `json:"isOk"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", err
	}
	if j.Code != 0 || !j.IsOk {
		return "", fmt.Errorf("退选失败: %s", j.Msg)
	}
	return j.Msg, nil
}

// CountEntry 实时人数（findElectivesStudentCount）。
type CountEntry struct {
	ID             int `json:"id"`
	SelectedCount  int `json:"selectedCount"`
	AuditedCount   int `json:"auditedCount"`
	MaxCount       int `json:"maxCount"`
}

// StudentCounts 查询课程实时人数。
func (c *Client) StudentCounts(ids []int) ([]CountEntry, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	form := url.Values{}
	form.Set("ids", strings.Join(parts, ","))
	body, err := c.doRequest(http.MethodPost, "/electives/select/findElectivesStudentCount",
		[]byte(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, err
	}
	var j struct {
		Code      int           `json:"code"`
		CountList []CountEntry  `json:"countList"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, err
	}
	if j.Code != 0 {
		return nil, fmt.Errorf("人数查询错误: %s", extractMsg(body))
	}
	return j.CountList, nil
}

// IsClassFull 实时查询该课程是否已满（已报人数 >= 可报人数）。
// 满员判定不依赖平台错误文案，直接对比人数——用户指定方案。
func (c *Client) IsClassFull(classID int) (bool, error) {
	counts, err := c.StudentCounts([]int{classID})
	if err != nil {
		return false, err
	}
	for _, ce := range counts {
		if ce.ID == classID {
			return ce.MaxCount > 0 && ce.SelectedCount >= ce.MaxCount, nil
		}
	}
	return false, fmt.Errorf("课程 %d 无人数数据", classID)
}