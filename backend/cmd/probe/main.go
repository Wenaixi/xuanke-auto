// probe 课程数据探针：按当前平台档案打一次课程数据接口并打印解码结果。
//
// 走档案（sites.Resolve + SiteDescriptor）而非硬编码域名/路径/Cookie——
// 切换平台后它自动打新站点，此前硬编码知到域名的版本是全仓唯一完全旁路档案的生产代码。
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/sites"
	"xuanke-auto/backend/internal/upstream"
)

func main() {
	cfg := config.Load()
	desc, err := sites.Resolve(cfg.PlatformID)
	if err != nil {
		fmt.Println("ERR 解析平台档案:", err)
		os.Exit(1)
	}

	// token 从环境变量读取（不硬编码：硬编码 token 会随源码进 git，存在被误当现役会话复用的风险）。
	token := os.Getenv("XUANKE_PROBE_TOKEN")
	if token == "" {
		fmt.Println("ERR: 请设置环境变量 XUANKE_PROBE_TOKEN")
		os.Exit(2)
	}

	base := upstream.NormalizeBaseURL(cfg.PlatformBaseURL)
	if base == "" {
		base = upstream.NormalizeBaseURL(desc.DefaultBaseURL)
	}

	// 会话 Cookie 由档案声明（知到需 access_limit_cookie=1；其他平台可能为空）。
	cookies := make([]string, 0, len(desc.SessionCookies)+1)
	for name, val := range desc.SessionCookies {
		cookies = append(cookies, name+"="+val)
	}
	// 空键名绝不提交：TokenCookie 为空即该平台无 Cookie 通道，写出 `=TOK`
	// 这种畸形 Cookie 平台多半直接拒（与档案 Form 空键名同一条契约）。
	if desc.TokenCookie != "" {
		cookies = append(cookies, desc.TokenCookie+"="+token)
	}
	cookieHeader := strings.Join(cookies, "; ")

	// token 查询串只在该平台用查询参数鉴权时拼：Header 通道（AuthHeader）平台
	// TokenParam 为空，无条件拼会写出 `?=TOKEN` 的畸形 URL。与 doRequest 的通道
	// 判定同口径（上游 client.go）。
	u := base + desc.ElectivesPath
	if (desc.AuthMode == upstream.AuthDual || desc.AuthMode == upstream.AuthQuery) && desc.TokenParam != "" {
		u += "?" + desc.TokenParam + "=" + url.QueryEscape(token)
	}
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		fmt.Println("ERR 构造请求:", err)
		os.Exit(1)
	}
	req.Header.Set("User-Agent", desc.Login.UserAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Referer", base+desc.RefererPath)
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	// 与全仓生产 HTTP 契约族对齐：15s 超时 + 共享连接池（64 连接/host），
	// 避免真实平台对复用濒死连接返回 RST/FIN 时工具挂起至内核超时（分钟级）。
	client := &http.Client{Timeout: 15 * time.Second, Transport: upstream.SharedTransport()}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("档案:", desc.ID, "状态:", resp.StatusCode, "字节:", len(body))

	data, err := desc.Decode.Electives(body)
	if err != nil {
		fmt.Println("ERR 解码失败:", err)
		fmt.Println(string(body[:min(len(body), 600)]))
		os.Exit(1)
	}
	fmt.Println("开放时间点:", len(data.BeginTimes), "发布数:", len(data.Publishes))
	for _, p := range data.Publishes {
		// 三态 selectable：nil = 站点不下发该信号（引擎走退化模式）
		sel := "nil（站点不下发）"
		if p.Selectable != nil {
			sel = fmt.Sprintf("%v", *p.Selectable)
		}
		fmt.Printf("  发布 %d %q selectable=%s 课程数=%d\n", p.PublishID, p.PublishName, sel, len(p.Classes))
	}
}
