package zhidao

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 性能专项基准：为「内存占用 / 响应速度」优化提供可复现的对照秤。
// 覆盖 doRequest 全链路（凭据快照 + 响应体读取 + code 校验）与 parseElectives 解析路径。
// 用法：go test -run='^$' -bench=. -benchmem -count=3 ./internal/zhidao/
// 注意：Client.Timeout=15s，超大 N 下小响应高并发 mock 偶发 header 等待超时属环境噪声，
// 非优化引入的回归——计数以 count=3 的中位数为准。

// benchElectivesJSON 生成接近真实规模的 findElectivesData 响应体。
// 真实平台当前激活学期为 3 个发布、约 82 门课（体育 3 + 校本1 40 + 校本2 39）。
// 夹具字段与产品 zhidao.Class 严格对齐（12 消费字段，LessonsDate/ApplyDate/PlanCount/
// AuditedCount 已随产品剔除）——夹具与产品同构，基准的 B/op 才有工程意义。
func benchElectivesJSON() []byte {
	type cls struct {
		ID              int    `json:"id"`
		CourseName      string `json:"course_name"`
		ClassName       string `json:"class_name"`
		TeacherNameList string `json:"teacher_name_list"`
		ClassroomName   string `json:"class_room_name"`
		SelectedCount   int    `json:"selected_count"`
		MaxCount        int    `json:"max_count"`
		CanSelect       bool   `json:"can_select"`
		BtnType         int    `json:"btn_type"`
		BtnText         string `json:"btn_text"`
		Title           string `json:"title"`
	}
	type pub struct {
		PublishID   int    `json:"publishId"`
		PublishName string `json:"publishName"`
		BeginDate   string `json:"beginDate"`
		InDateRange bool   `json:"inDateRange"`
		CanSelect   int    `json:"canSelect"`
		HasSelected int    `json:"hasSelected"`
		GroupCount  int    `json:"groupCount"`
		TotalCount  int    `json:"totalCount"`
		Classes     []cls  `json:"electivesClassList"`
	}
	names := []string{"体育", "校本1", "校本2"}
	counts := []int{3, 40, 39}
	pubs := make([]pub, 0, 3)
	id := 61000
	for i, n := range names {
		classes := make([]cls, 0, counts[i])
		for j := 0; j < counts[i]; j++ {
			id++
			classes = append(classes, cls{
				ID:              id,
				CourseName:      fmt.Sprintf("课程名称示例%d", id),
				ClassName:       fmt.Sprintf("教学班%d", id),
				TeacherNameList: "张三,李四",
				ClassroomName:   "教学楼A101",
				SelectedCount:   17,
				MaxCount:        36,
				CanSelect:       true,
				BtnType:         2,
				BtnText:         "报名",
				Title:           "",
			})
		}
		pubs = append(pubs, pub{
			PublishID:   100 + i,
			PublishName: n,
			BeginDate:   "2026-09-13 09:00:00",
			InDateRange: true,
			CanSelect:   1,
			HasSelected: 0,
			GroupCount:  counts[i],
			TotalCount:  counts[i],
			Classes:     classes,
		})
	}
	body, err := json.Marshal(map[string]any{
		"code":                0,
		"beginTimes":          []int64{1789261200000},
		"selectElectivesData": pubs,
	})
	if err != nil {
		panic(err)
	}
	return body
}

// benchServer 起一个回环 mock：按路径返回指定体。
// 显式写 Content-Length 头对齐平台实证形态（HAR：课程接口响应 content-encoding none、
// 直发已知长度）——若不设此头 httptest 走 chunked 传输，resp.ContentLength=-1，
// readBody 恒回退 io.ReadAll，基准测不到预分配路径（优化尽失）。
func benchServer(tb testing.TB, payload []byte) *httptest.Server {
	tb.Helper()
	socketPreheat()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.Write(payload)
	}))
}

// benchClient 构造带凭据的客户端（对齐生产：idToken + 双 Cookie 通道）。
func benchClient(tb testing.TB, baseURL string) *Client {
	tb.Helper()
	c := New(baseURL, VisionConfig{})
	c.SetCredentials("bench-acct", "bench-pwd", "1234567890123456")
	c.SetCookies(map[string]string{
		"zd_edu_cookie":       "1234567890123456",
		"access_limit_cookie": "abcdef0123456789",
	})
	return c
}

// BenchmarkDoRequestSmall 小响应路径（登录/报名类）：衡量每次请求的固定开销
// ——凭据快照拷贝 + 请求构造 + 响应体读取 + code 校验。
func BenchmarkDoRequestSmall(b *testing.B) {
	payload := []byte(`{"code":0,"isOk":true,"msg":"选课成功！"}`)
	srv := benchServer(b, payload)
	defer srv.Close()
	c := benchClient(b, srv.URL)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.doRequest(http.MethodPost, "/electives/select/selectElectivesClass",
			[]byte("classId=61115"), "application/x-www-form-urlencoded"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDoRequestLarge 大响应路径（findElectivesData 全量课程）：
// 衡量 io.ReadAll 在无 ContentLength 预分配下的 realloc + copy 成本。
func BenchmarkDoRequestLarge(b *testing.B) {
	payload := benchElectivesJSON()
	srv := benchServer(b, payload)
	defer srv.Close()
	c := benchClient(b, srv.URL)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.doRequest(http.MethodPost, "/electives/select/findElectivesData",
			nil, ""); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseElectives 纯解析路径（不含网络）：衡量匿名 raw struct → Publish 复制的分配成本。
func BenchmarkParseElectives(b *testing.B) {
	payload := benchElectivesJSON()
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parseElectives(payload); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCredentialSnapshot 单独量化 doRequest 开头的凭据快照开销
// （cookies map 深拷贝 + cookie header 拼接），是「Cookie 头预计算」优化的对照秤。
func BenchmarkCredentialSnapshot(b *testing.B) {
	c := benchClient(b, "http://127.0.0.1:1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.mu.Lock()
		tok := c.token
		cookies := make(map[string]string, len(c.cookies))
		for k, v := range c.cookies {
			cookies[k] = v
		}
		c.mu.Unlock()
		_ = tok
		parts := make([]string, 0, len(cookies))
		for k, v := range cookies {
			parts = append(parts, k+"="+v)
		}
		_ = strings.Join(parts, "; ")
	}
}

// TestParseElectivesBenchShape 基准夹具自检：生成的 payload 必须是 3 发布 82 门课，
// 否则 benchmark 的规模前提不成立（数字失去工程意义）。
func TestParseElectivesBenchShape(t *testing.T) {
	d, err := parseElectives(benchElectivesJSON())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Publishes) != 3 {
		t.Fatalf("发布数应为 3，实际 %d", len(d.Publishes))
	}
	total := 0
	for _, p := range d.Publishes {
		total += len(p.Classes)
	}
	if total != 82 {
		t.Fatalf("课程总数应为 82，实际 %d", total)
	}
}

// 同 server 同形态（显式 Content-Length）的 apples-to-apples 前后对照：
// readBody 按 CL 预分配（ReadFull）vs 旧 io.ReadAll。doRequest 的 HTTP 层其他分配
// （请求构造/Header 克隆/响应对象）会淹没两者差异，隔离在 httpDo 层测才能显形：
// before = 显式 io.ReadAll（readBody 出现前形态），after = readBody。
func BenchmarkDoRequestLargeReadAllBefore(b *testing.B) {
	payload := benchElectivesJSON()
	srv := benchServer(b, payload)
	defer srv.Close()
	c := benchClient(b, srv.URL)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/electives/select/findElectivesData", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := httpDo(c.http, req)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadAll(resp.Body); err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkDoRequestLargeReadBodyAfter(b *testing.B) {
	payload := benchElectivesJSON()
	srv := benchServer(b, payload)
	defer srv.Close()
	c := benchClient(b, srv.URL)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/electives/select/findElectivesData", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := httpDo(c.http, req)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := readBody(resp); err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
