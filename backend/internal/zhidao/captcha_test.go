package zhidao

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecognizeCaptcha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header 缺失或错误: %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\" abcd \"}}]}"))
	}))
	defer srv.Close()

	got, err := recognizeCaptcha(VisionConfig{BaseURL: srv.URL, APIKey: "test-key", Model: "m"}, []byte("fake-jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "abcd" {
		t.Fatalf("got %q, want %q", got, "abcd")
	}
}

func TestRecognizeCaptchaNoKey(t *testing.T) {
	_, err := recognizeCaptcha(VisionConfig{}, []byte("x"))
	if err == nil {
		t.Fatal("期望无 API Key 时报错")
	}
}
