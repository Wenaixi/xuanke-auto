package runtime

import (
	"testing"
	"time"
)

func TestStoreGetUpdate(t *testing.T) {
	s := New(Config{
		ActivationEnabled: true,
		VisionBaseURL:     "https://a",
		VisionAPIKey:      "k1",
		VisionModel:       "m1",
	})
	if !s.Get().ActivationEnabled || s.Get().VisionAPIKey != "k1" {
		t.Fatalf("初始值异常: %+v", s.Get())
	}
	// 热更新：改开关与 Vision
	s.Update(func(c *Config) {
		c.ActivationEnabled = false
		c.VisionBaseURL = "https://b"
		c.VisionAPIKey = "k2"
		c.VisionModel = "m2"
	})
	got := s.Get()
	if got.ActivationEnabled || got.VisionAPIKey != "k2" || got.VisionModel != "m2" {
		t.Fatalf("更新未生效: %+v", got)
	}
	// 未改字段保持
	if got.VisionBaseURL != "https://b" {
		t.Fatalf("base url 未更新: %+v", got)
	}
}

// TestStoreConcurrentSafe 配置中心并发读写在 RWMutex 保护下不崩不漏。
func TestStoreConcurrentSafe(t *testing.T) {
	s := New(Config{})
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.Get()
				s.Update(func(c *Config) { c.VisionModel = "m" })
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	_ = time.Now()
}
