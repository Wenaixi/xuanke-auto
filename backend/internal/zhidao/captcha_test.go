package zhidao

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func init() {
	// 测试信号量：识别并发上限为 2（验证并发限流行为），全局初始化一次
	NewCaptchaSemaphore(2)
}

func TestRecognizeCaptcha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header 缺失或错误: %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\" abcd \"}}]}"))
	}))
	defer srv.Close()

	got, err := recognizeCaptcha(VisionConfig{BaseURL: srv.URL, APIKey: "test-key", Model: "m", recognizer: NewVisionRecognizer(VisionConfig{BaseURL: srv.URL, APIKey: "test-key", Model: "m"})}, []byte("fake-jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "abcd" {
		t.Fatalf("got %q, want %q", got, "abcd")
	}
}

func TestRecognizeCaptchaNoKey(t *testing.T) {
	// recognizer 为 nil：直接报"未配置识别引擎"
	_, err := recognizeCaptcha(VisionConfig{}, []byte("x"))
	if err == nil {
		t.Fatal("期望未配置识别引擎时报错")
	}
}

// TestCaptchaConcurrency 验证信号量并发限流：并发 10 个识别请求，
// 但同一时刻实际进入识别核心的至多 2 个（并发上限）。
func TestCaptchaConcurrency(t *testing.T) {
	NewCaptchaSemaphore(2) // 上限 2

	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()

		time.Sleep(30 * time.Millisecond) // 模拟识别耗时，拉开并发窗口

		mu.Lock()
		inFlight--
		mu.Unlock()
		w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\"ok3x\"}}]}"))
	}))
	defer srv.Close()

	r := NewVisionRecognizer(VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = r.Recognize([]byte("fake"))
		}(i)
	}
	wg.Wait()

	if maxInFlight > 2 {
		t.Fatalf("并发上限应为 2，实测最高并发 %d", maxInFlight)
	}
	for i, e := range errs {
		if e != nil {
			t.Fatalf("识别 %d 应成功: %v", i, e)
		}
	}
}
