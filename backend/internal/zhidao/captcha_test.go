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
	socketPreheat() // 端口预加热，防冷启动 connectex（同 client_test）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authorization 断言只针对真实识别路径——readyProbe 探活 GET /login 无
		// 业务头，命中此断言会把探活当业务请求误报（补探活后实证）。
		if r.URL.Path == "/chat/completions" && r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header 缺失或错误: %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\" abcd \"}}]}"))
	}))
	defer srv.Close()
	// 包内仅剩的无探活 mock 首请求宿主（全量首轮该测试
	// connectex FAIL 实证）——与 TestCaptchaConcurrency 双保险成族闭环。
	readyProbe(t, srv.URL)

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
	socketPreheat()        // 端口预加热（同 client_test）

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
	// 主控收尾回归：-p 1 下该测试偶发 18.95s FAIL（正常 0.16s）——10 个并发
	// 识别请求的首请求撞上前序包 TIME_WAIT 冷启动窗口（历史宿主：
	// 并发首请求 connectex 时信号量计数被误判）。socketPreheat 只预占单端口，
	// 并发首请求形态需 readyProbe 把 accept 就绪前的最首请求吃掉（api/zhidao 同款
	// 双保险，「残余面宿主=并发形态」的实证）。
	readyProbe(t, srv.URL)

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

// TestSetCaptchaConcurrencyConcurrent 验证高并发下动态热调整并发度不会导致死锁或竞态（验证码并发度）。
func TestSetCaptchaConcurrencyConcurrent(t *testing.T) {
	NewCaptchaSemaphore(2)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 启动多个协程持续执行 withConcurrency
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = withConcurrency(func() (string, error) {
						time.Sleep(5 * time.Millisecond)
						return "ok", nil
					})
				}
			}
		}()
	}

	// 主协程在 200ms 内频繁动态调整并发度
	for i := 0; i < 20; i++ {
		SetCaptchaConcurrency((i % 4) + 1)
		time.Sleep(10 * time.Millisecond)
	}

	close(stop)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 成功退出，没有死锁
	case <-time.After(3 * time.Second):
		t.Fatal("SetCaptchaConcurrency 在高并发热调时发生死锁！")
	}
}
