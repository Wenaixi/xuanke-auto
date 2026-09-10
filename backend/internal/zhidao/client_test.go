package zhidao

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestAutoRelogin 验证：第一次请求返回 code=-1（token 失效），
// 客户端自动用保存账密重登后重试成功。
func TestAutoRelogin(t *testing.T) {
	var reloginCalls int32
	// mock 登录接口
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证码识别（硅基流动 mock）
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{"content": " abcd "},
				}},
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
			// 登录初始化/验证码：返回空
			w.Write([]byte("ok"))
			return
		}
		// 业务接口：根据 token 判断
		tok := r.URL.Query().Get("idToken")
		w.Header().Set("Content-Type", "application/json")
		if tok == "old-token" {
			json.NewEncoder(w).Encode(map[string]any{"code": -1, "msg": "您未登录,请刷新页面重新登录"})
			return
		}
		if tok == "new-token-999" {
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "unknown token " + tok})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "old-token")

	// 请求一个简单接口触发 doRequest
	body, err := c.doRequest(http.MethodPost, "/electives/select", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var j struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	if j.Code != 0 {
		t.Fatalf("期望重登后 code=0，实际 %d", j.Code)
	}
	if atomic.LoadInt32(&reloginCalls) != 1 {
		t.Fatalf("期望重登 1 次，实际 %d", reloginCalls)
	}
	if c.Token() != "new-token-999" {
		t.Fatalf("token 未更新: %q", c.Token())
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
