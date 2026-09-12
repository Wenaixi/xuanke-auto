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
		OpenTime:          "2026-09-13 09:00:00",
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

func TestUpdateOpenTimeParse(t *testing.T) {
	s := New(Config{OpenTime: "2026-09-13 09:00:00"})
	if s.Get().OpenTimeParsed.IsZero() {
		t.Fatalf("合法开放时间应解析出时间: %+v", s.Get())
	}
	if s.Get().OpenTimeParsed.Year() != 2026 || s.Get().OpenTimeParsed.Hour() != 9 {
		t.Fatalf("解析结果异常: %v", s.Get().OpenTimeParsed)
	}
	// 非法值保持零值
	s.Update(func(c *Config) { c.OpenTime = "not-a-time" })
	if !s.Get().OpenTimeParsed.IsZero() {
		t.Fatalf("非法开放时间应为零值: %v", s.Get().OpenTimeParsed)
	}
	// 修正后恢复
	s.Update(func(c *Config) { c.OpenTime = "2026-09-13 10:30:00" })
	if s.Get().OpenTimeParsed.Minute() != 30 {
		t.Fatalf("修正后未恢复: %v", s.Get().OpenTimeParsed)
	}
}

func TestStoreConcurrentSafe(t *testing.T) {
	s := New(Config{OpenTime: "2026-09-13 09:00:00"})
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
