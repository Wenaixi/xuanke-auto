package zhidao

import (
	"testing"
	"time"
)

func TestEncryptIdentification(t *testing.T) {
	s, err := encryptIdentification("***REMOVED***", "***REMOVED***")
	if err != nil {
		t.Fatal(err)
	}
	// RSA 1024 位 PKCS1 v1.5 密文 base64 后至少 170+ 字符
	if len(s) < 100 {
		t.Fatalf("cipher too short: %d", len(s))
	}
}

func TestUniqueDeviceID(t *testing.T) {
	got := uniqueDeviceID("test-ua", mustTime(t, "2026-09-13T09:00:00+08:00"))
	if got == "" {
		t.Fatal("empty device id")
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.ParseInLocation("2006-01-02T15:04:05-07:00", s, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}
