package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	token := "***REMOVED***"
	u := "https://www.zhidao.fj.cn/electives/select?idToken=" + token
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Origin", "https://www.zhidao.fj.cn")
	req.Header.Set("Referer", "https://www.zhidao.fj.cn/admin.html")
	req.Header.Set("Cookie", "access_limit_cookie=***REMOVED***; zd_edu_cookie=" + token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { fmt.Println("ERR", err); return }
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var j struct{ Code int; Msg string; IsOk bool }
	json.Unmarshal(body, &j)
	fmt.Printf("code=%d isOk=%v msg=%q\n", j.Code, j.IsOk, j.Msg)
	fmt.Println("BODY:", string(body[:min(len(body), 200)]))
}