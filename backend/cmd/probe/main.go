package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	// 使用数据库当前 token
	token := "***REMOVED***"
	u := "https://www.zhidao.fj.cn/electives/select/findElectivesData?idToken=" + token
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Cookie", "access_limit_cookie=***REMOVED***; zd_edu_cookie="+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { fmt.Println("ERR", err); return }
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("LEN:", len(body))
	fmt.Println(string(body[:min(len(body), 600)]))
	var j map[string]any
	json.Unmarshal(body, &j)
	fmt.Println("keys:", len(j))
}
