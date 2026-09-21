package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

func main() {
	// token 从数据库读取（不再硬编码：安全审计——硬编码实测 token 已随源码提交 git，
	// 即使当时已失效也存在被误当现役会话复用的风险）。真实运行以数据库当前 token 为准。
	token := os.Getenv("XUANKE_PROBE_TOKEN")
	if token == "" {
		fmt.Println("ERR: 请设置环境变量 XUANKE_PROBE_TOKEN 或先执行数据提取")
		os.Exit(2)
	}
	u := "https://www.zhidao.fj.cn/electives/select/findElectivesData?idToken=" + url.QueryEscape(token)
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Cookie", "access_limit_cookie=1; zd_edu_cookie="+token)
	// 与全仓生产 HTTP 契约族对齐——不再用 http.DefaultClient
	//（Timeout=0 无兜底 + 默认 2 连接池），改用 15s 超时 + 共享连接池（64 连接/host）、
	// 避免真实平台对复用濒死连接返回 RST/FIN 时工具挂起至内核超时（分钟级）。
	client := &http.Client{Timeout: 15 * time.Second, Transport: zhidao.SharedTransport()}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("LEN:", len(body))
	fmt.Println(string(body[:min(len(body), 600)]))
	var j map[string]any
	json.Unmarshal(body, &j)
	fmt.Println("keys:", len(j))
}
