package upstream

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
// 覆盖 doRequest 全链路（凭据快照 + 响应体读取 + code 校验）；站点线格式的解码基准
// 归 internal/sites/zhidao（那边用真实字段名，自带 3 发布 82 门课的 shape 自检）。
// 用法：go test -run='^$' -bench=. -benchmem -count=3 ./internal/upstream/
// 注意：Client.Timeout=15s，超大 N 下小响应高并发 mock 偶发 header 等待超时属环境噪声，
// 非优化引入的回归——计数以 count=3 的中位数为准。

// benchLargePayload 生成与真实课程响应同量级（约 30KB）的 JSON 体，供传输层基准使用。
// 刻意用中立数据模型序列化：传输基准只关心「体量 + 已知 Content-Length」——
// 解码路径的分配基准归 internal/sites/zhidao（用真实字段名 + shape 自检）。
func benchLargePayload() []byte {
	counts := []int{3, 40, 39}
	pubs := make([]Publish, 0, len(counts))
	id := 61000
	for i, n := range counts {
		classes := make([]Class, 0, n)
		for j := 0; j < n; j++ {
			id++
			classes = append(classes, Class{
				ID:              id,
				CourseName:      fmt.Sprintf("课程名称示例%d", id),
				ClassName:       fmt.Sprintf("教学班%d", id),
				TeacherNameList: "张三,李四",
				ClassroomName:   "教学楼A101",
				SelectedCount:   17,
				MaxCount:        36,
				CanSelect:       true,
				Action:          "enroll",
				ActionText:      "报名",
			})
		}
		pubs = append(pubs, Publish{
			PublishID:   100 + i,
			PublishName: fmt.Sprintf("发布%d", i+1),
			BeginDate:   "2026-09-13 09:00:00",
			Selectable:  boolPtrBench(true),
			CanSelect:   1,
			GroupCount:  n,
			TotalCount:  n,
			Classes:     classes,
		})
	}
	body, err := json.Marshal(map[string]any{
		"begin_times": []int64{1789261200000},
		"publishes":   pubs,
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
	c := newTestClient(baseURL, VisionConfig{})
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
	payload := benchLargePayload()
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

// 同 server 同形态（显式 Content-Length）的 apples-to-apples 前后对照：
// readBody 按 CL 预分配（ReadFull）vs 旧 io.ReadAll。doRequest 的 HTTP 层其他分配
// （请求构造/Header 克隆/响应对象）会淹没两者差异，隔离在 httpDo 层测才能显形：
// before = 显式 io.ReadAll（readBody 出现前形态），after = readBody。
func BenchmarkDoRequestLargeReadAllBefore(b *testing.B) {
	payload := benchLargePayload()
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
	payload := benchLargePayload()
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

// boolPtrBench 基准夹具辅助：三态 selectable 的指针形式。
func boolPtrBench(v bool) *bool { return &v }
