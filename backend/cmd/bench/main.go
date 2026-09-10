package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// 串行延迟测试：200 次连续请求
func main() {
	url := "http://localhost:8080/api/state"
	client := &http.Client{Timeout: 5 * time.Second}
	var total time.Duration
	var max time.Duration
	for i := 0; i < 200; i++ {
		t0 := time.Now()
		resp, err := client.Get(url)
		if err != nil { fmt.Println("ERR", err); return }
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lat := time.Since(t0)
		total += lat
		if lat > max { max = lat }
	}
	fmt.Printf("200 次串行: 平均 %v, 最大 %v\n", total/200, max)
}
