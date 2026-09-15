package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 串行延迟测试：200 次连续请求。
// B11-A4：/api/state 挂 requireAuth（无令牌返回 401），此前无 Authorization 头
// 测的是 401 拒绝路径而非真实业务延迟——结果虚低且无意义。现支持 -token 注入 Bearer 令牌
// （从 data/.env 的 XUANKE_ADMIN_TOKEN 或运行日志中取得）；未提供时仍可测 401 路径但输出标注。
func main() {
	token := flag.String("token", "", "会话令牌（Bearer），缺省时测的是 401 拒绝路径")
	times := flag.Int("n", 200, "请求次数")
	url := flag.String("url", "http://localhost:3091/api/state", "目标接口")
	flag.Parse()

	client := &http.Client{Timeout: 5 * time.Second}
	var total time.Duration
	var max time.Duration
	for i := 0; i < *times; i++ {
		req, err := http.NewRequest("GET", *url, nil)
		if err != nil {
			fmt.Println("ERR", err)
			return
		}
		if *token != "" {
			req.Header.Set("Authorization", "Bearer "+*token)
		}
		t0 := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERR", err)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lat := time.Since(t0)
		total += lat
		if lat > max {
			max = lat
		}
	}
	mode := "未带令牌（401 拒绝路径）"
	if *token != "" {
		mode = "带 Bearer 令牌（真实业务路径）"
	}
	fmt.Printf("%d 次串行（%s）: 平均 %v, 最大 %v\n", *times, mode, total/time.Duration(*times), max)
	if *token == "" {
		fmt.Fprintln(os.Stderr, "提示：传入 -token 才能测真实业务延迟")
	}
}
