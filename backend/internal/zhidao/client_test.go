package zhidao

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// loginMockServer 构造登录链路 mock。
// failRecognize: 前 N 次识别返回空串（识别失败）；failSubmit: 前 M 次提交被拒。
func loginMockServer(t *testing.T, failRecognize, failSubmit int) (*httptest.Server, *int32, *int32) {
	t.Helper()
	var captchas int32
	var submits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			n := atomic.AddInt32(&captchas, 1)
			if int(n) <= failRecognize {
				w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`)) // 识别失败：返回空
				return
			}
			w.Write([]byte(`{"choices":[{"message":{"content":" abcd "}}]}`)) // 识别成功
		case strings.HasSuffix(r.URL.Path, "/login/doLogin"):
			n := atomic.AddInt32(&submits, 1)
			if int(n) <= failSubmit {
				w.Write([]byte(`{"code":1,"isOk":false,"msg":"验证码错误"}`)) // 提交被拒
			} else {
				w.Write([]byte(`{"code":0,"isOk":true,"token":"tok-ok"}`))
			}
		default: // /login 与 /login/captcha
			w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &captchas, &submits
}

// TestLoginRetryWithinLimits 验证：识别失败可刷新重试，提交被拒会刷新验证码，最终成功。
func TestLoginRetryWithinLimits(t *testing.T) {
	srv, captchas, submits := loginMockServer(t, 1, 1) // 识别 1 次失败 + 提交 1 次被拒后成功
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	tok, err := c.Login("acct", "pwd")
	if err != nil {
		t.Fatalf("登录应成功: %v", err)
	}
	if tok != "tok-ok" {
		t.Fatalf("token 错误: %s", tok)
	}
	// 时序：识别失败(1) → 识别成功(2)+提交被拒 → 刷新重识别(3)+提交成功
	if got := atomic.LoadInt32(captchas); got != 3 {
		t.Fatalf("识别应 3 次（1失败+1被拒后刷新+1成功），实际 %d", got)
	}
	if got := atomic.LoadInt32(submits); got != 2 {
		t.Fatalf("提交应 2 次（1被拒+1成功），实际 %d", got)
	}
	if c.Token() != "tok-ok" {
		t.Fatalf("客户端 token 未登记: %s", c.Token())
	}
}

// TestLoginStopsAfterCaptchaExhausted 验证：识别连败到上限即停止，不无限重试。
func TestLoginStopsAfterCaptchaExhausted(t *testing.T) {
	srv, captchas, submits := loginMockServer(t, 99, 99) // 永远失败
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	_, err := c.Login("acct", "pwd")
	if err == nil {
		t.Fatal("登录应失败")
	}
	if got := atomic.LoadInt32(captchas); got != 3 {
		t.Fatalf("识别最多 3 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(submits); got != 0 {
		t.Fatalf("识别失败不应提交，实际 %d", got)
	}
}

// TestLoginNetworkErrorAbortsImmediately 验证：网络/配置错误立即返回，绝不重试。
func TestLoginNetworkErrorAbortsImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // 登录页 500
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	if _, err := c.Login("acct", "pwd"); err == nil {
		t.Fatal("应报错")
	}
}

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

// TestExitClass 验证退选接口：路径/请求体与真实 HAR 一致（form classId）。
func TestExitClass(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		gotPath = r.URL.Path
		gotBody = string(body)
		json.NewEncoder(w).Encode(map[string]any{"code": 0, "isOk": true, "msg": "退选成功"})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "tok")
	msg, err := c.ExitClass(61115)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "退选成功" {
		t.Fatalf("期望返回平台消息，实际 %q", msg)
	}
	if gotPath != "/electives/select/exitElectivesClass" {
		t.Fatalf("路径错误: %s", gotPath)
	}
	if gotBody != "classId=61115" {
		t.Fatalf("请求体错误: %s", gotBody)
	}
}

// TestExitClassFailsOnCodeNotZero 验证退选业务失败（code!=0）时报错。
func TestExitClassFailsOnCodeNotZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"code": 1, "isOk": false, "msg": "已过退选时间"})
	}))
	defer srv.Close()

	c := New(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.SetCredentials("acct", "pwd", "tok")
	if _, err := c.ExitClass(61115); err == nil {
		t.Fatal("期望业务失败时报错")
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