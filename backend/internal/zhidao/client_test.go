package zhidao

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestNoAutoRelogin 验证：token 失效（code=-1）时直接返回错误，不自动重登。
func TestNoAutoRelogin(t *testing.T) {
	var reloginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/login/doLogin") {
			atomic.AddInt32(&reloginCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "new-token-999"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/login") || strings.HasSuffix(r.URL.Path, "/login/captcha") {
			w.Write([]byte("ok"))
			return
		}
		tok := r.URL.Query().Get("idToken")
		w.Header().Set("Content-Type", "application/json")
		if tok == "old-token" {
			json.NewEncoder(w).Encode(map[string]any{"code": -1, "msg": "您未登录,请刷新页面重新登录"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "old-token")

	// token 失效：应返回 ErrUnauthorized，且不触发重登
	_, err := c.doRequest(http.MethodPost, "/electives/select", nil, "")
	if err == nil {
		t.Fatal("期望 code=-1 时报错")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("期望 ErrUnauthorized，实际 %v", err)
	}
	if atomic.LoadInt32(&reloginCalls) != 0 {
		t.Fatalf("不应自动重登，实际 %d 次", reloginCalls)
	}
}

// TestReloginIfNeeded 验证显式重登：一次即可，最多一次。
func TestReloginIfNeeded(t *testing.T) {
	var reloginCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证码识别（硅基流动 mock）
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": " abcd "}}},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/login/doLogin") {
			atomic.AddInt32(&reloginCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "token": "new-token-999"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/login") || strings.HasSuffix(r.URL.Path, "/login/captcha") {
			w.Write([]byte("ok"))
			return
		}
		tok := r.URL.Query().Get("idToken")
		w.Header().Set("Content-Type", "application/json")
		if tok == "old-token" {
			json.NewEncoder(w).Encode(map[string]any{"code": -1, "msg": "您未登录,请刷新页面重新登录"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "old-token")

	// 第一次请求失败（token 失效）
	if _, err := c.doRequest(http.MethodPost, "/electives/select", nil, ""); err == nil {
		t.Fatal("期望失败")
	}
	// 显式重登一次
	relogged, err := c.ReloginIfNeeded()
	if err != nil || !relogged {
		t.Fatalf("重登失败: %v %v", relogged, err)
	}
	// 重登后 token 更新，再次请求成功
	body, err := c.doRequest(http.MethodPost, "/electives/select", nil, "")
	if err != nil {
		t.Fatalf("重登后请求失败: %v", err)
	}
	var j struct {
		Code int `json:"code"`
	}
	json.Unmarshal(body, &j)
	if j.Code != 0 {
		t.Fatalf("重登后 code=%d", j.Code)
	}
	if atomic.LoadInt32(&reloginCalls) != 1 {
		t.Fatalf("最多重登 1 次，实际 %d", reloginCalls)
	}
}

// TestParseElectives 用 HAR 真实响应片段验证解析
func TestParseElectives(t *testing.T) {
	// 精简的 findElectivesData 响应（字段与真实一致）
	raw := `{"code":0,"beginTimes":[1789261200000],"selectElectivesData":[
	  {"publishId":3225,"publishName":"高二年体育","beginDate":"2026-09-13 09:00:00","inDateRange":false,
	   "canSelect":1,"hasSelected":0,"groupCount":1,"totalCount":3,
	   "electivesClassList":[{"id":61115,"publish_id":3225,"course_name":"健美操","class_name":"健美操1、2班",
	     "teacher_name_list":"陈跃强","class_room_name":"操场","lessons_date":null,"selected_count":0,
	     "max_count":36,"plan_count":36,"can_select":false,"btn_type":2,"btn_text":"报名"}]}
	]}`
	data, err := parseElectives([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 1 {
		t.Fatalf("期望 1 个发布，实际 %d", len(data.Publishes))
	}
	p := data.Publishes[0]
	if p.PublishID != 3225 || p.PublishName != "高二年体育" {
		t.Fatalf("发布解析错误: %+v", p)
	}
	if p.InDateRange {
		t.Fatal("窗口应未开放")
	}
	if len(p.Classes) != 1 || p.Classes[0].ID != 61115 || p.Classes[0].CourseName != "健美操" {
		t.Fatalf("课程解析错误: %+v", p.Classes)
	}
}