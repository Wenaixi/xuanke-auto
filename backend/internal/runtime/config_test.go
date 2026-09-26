package runtime

import (
	"errors"
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

// TestConfigRoundTripThroughSettings 配置 schema 单一映射的往返契约：Config 经
// ToSettings 落库、再经 ApplySettings 还原，必须逐字段相等。
// 这是"漏改一处即静默丢配置"的防线——SaveSettings 是 DELETE 全表 + 全量 INSERT，
// 新增字段若只改落库不还原，重启后该键从表中消失且无任何报错。
func TestConfigRoundTripThroughSettings(t *testing.T) {
	want := Config{
		ActivationEnabled:  true,
		VisionBaseURL:      "https://api.example.com/v1",
		VisionAPIKey:       "sk-secret",
		VisionModel:        "model-x",
		CaptchaEngine:      "vision",
		CaptchaFallback:    true,
		CaptchaConcurrency: 4,
	}
	// 落库侧 vision_key 必经注入的加密器、且带 enc: 前缀；还原侧注入 decrypt 还原。
	// runtime 不依赖也不验证具体加密算法——它只保证"值经加密器处理后才落库"，
	// 密文强度是加密器的职责（生产用 AES-256-GCM，由 api 层注入）。
	var encrypted string
	kv, err := ToSettings(want, func(s string) (string, error) {
		encrypted = s
		return "CIPHERTEXT", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if encrypted != want.VisionAPIKey {
		t.Fatalf("加密器入参应为明文密钥，实际 %q", encrypted)
	}
	if kv["vision_key"] != encPrefix+"CIPHERTEXT" {
		t.Fatalf("vision_key 落库应为加密器输出加 enc: 前缀，实际 %q", kv["vision_key"])
	}

	// 还原侧 decrypt 是解密器（密文 → 明文），与落库侧 encrypt 互逆；
	// ApplySettings 已剥掉 enc: 前缀，decrypt 只拿到密文本身
	var got Config
	ApplySettings(&got, kv, func(s string) (string, error) {
		if s != "CIPHERTEXT" {
			return "", errors.New("密文不匹配")
		}
		return want.VisionAPIKey, nil
	})
	if got != want {
		t.Fatalf("往返不一致:\n 写入=%+v\n 读回=%+v", want, got)
	}
}

// TestToSettingsWritesEveryKey 表覆盖性：键数必须与表长度一致，防止新增字段
// 只加进表却漏了序列化分支（表驱动本身是唯一事实源，测试钉住它不缩水）。
func TestToSettingsWritesEveryKey(t *testing.T) {
	kv, err := ToSettings(Config{}, func(s string) (string, error) { return s, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(kv) != len(configFields) {
		t.Fatalf("ToSettings 输出 %d 键，配置表有 %d 字段", len(kv), len(configFields))
	}
	for _, f := range configFields {
		if _, ok := kv[f.Key]; !ok {
			t.Errorf("配置表字段 %s 未出现在落库 map 中", f.Key)
		}
	}
}

// TestApplySettingsSkipsAbsentKeys 库中缺某键时保留调用方传入的既有值（不得
// 归零）——旧版本库只存部分键是既有现实，缺键必须"不改"而非"清空"。
func TestApplySettingsSkipsAbsentKeys(t *testing.T) {
	got := Config{VisionModel: "preexisting", CaptchaConcurrency: 3}
	ApplySettings(&got, map[string]string{"vision_model": "from-db"},
		func(s string) (string, error) { return s, nil })
	if got.VisionModel != "from-db" {
		t.Fatalf("存在的键应被覆盖: %q", got.VisionModel)
	}
	if got.CaptchaConcurrency != 3 {
		t.Fatalf("缺键不得清空既有值，实际 %d", got.CaptchaConcurrency)
	}
}

// TestApplySettingsRejectsUnencryptedVisionKey vision_key 严格要求 enc: 前缀，
// 旧版明文一律拒绝加载（不兼容旧数据是既有安全决策，不得因重构而放松）。
func TestApplySettingsRejectsUnencryptedVisionKey(t *testing.T) {
	got := Config{VisionAPIKey: "keep"}
	var decryptCalled bool
	ApplySettings(&got, map[string]string{"vision_key": "plaintext-leak"},
		func(s string) (string, error) { decryptCalled = true; return s, nil })
	if decryptCalled {
	t.Error("明文 vision_key 不得进入 decrypt")
	}
	if got.VisionAPIKey != "keep" {
		t.Fatalf("明文 vision_key 应被拒绝并保留原值，实际 %q", got.VisionAPIKey)
	}
}

// TestApplySettingsIgnoresInvalidConcurrency 非法并发值（0/负数/非数字）保留
// 既有值而非写入垃圾——原 server.go 逐键解析里的正数校验不得在重构中丢失。
func TestApplySettingsIgnoresInvalidConcurrency(t *testing.T) {
	for _, bad := range []string{"0", "-3", "abc", ""} {
		got := Config{CaptchaConcurrency: 5}
	ApplySettings(&got, map[string]string{"captcha_concurrency": bad},
			func(s string) (string, error) { return s, nil })
		if got.CaptchaConcurrency != 5 {
			t.Errorf("非法并发值 %q 应保留既有值，实际 %d", bad, got.CaptchaConcurrency)
		}
	}
	got := Config{CaptchaConcurrency: 5}
	ApplySettings(&got, map[string]string{"captcha_concurrency": "7"},
		func(s string) (string, error) { return s, nil })
	if got.CaptchaConcurrency != 7 {
		t.Fatalf("合法并发值应生效，实际 %d", got.CaptchaConcurrency)
	}
}
