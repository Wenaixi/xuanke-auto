package session

import (
	"testing"
	"time"
)

func TestCreateAndAccount(t *testing.T) {
	s := New(time.Hour)
	tok := s.Create("acct1")
	if tok == "" {
		t.Fatal("令牌不应为空")
	}
	acct, ok := s.Account(tok)
	if !ok || acct != "acct1" {
		t.Fatalf("会话绑定异常: %q %v", acct, ok)
	}
	// 随机性：两次签发不同
	if s.Create("acct1") == tok {
		t.Fatal("令牌应唯一随机")
	}
	// 无效令牌
	if _, ok := s.Account("not-a-token"); ok {
		t.Fatal("无效令牌不应通过")
	}
}

func TestExpiry(t *testing.T) {
	s := New(50 * time.Millisecond)
	tok := s.Create("acct1")
	time.Sleep(120 * time.Millisecond)
	if _, ok := s.Account(tok); ok {
		t.Fatal("过期会话应失效")
	}
}

func TestDelete(t *testing.T) {
	s := New(time.Hour)
	tok := s.Create("acct1")
	s.Delete(tok)
	if _, ok := s.Account(tok); ok {
		t.Fatal("删除后会话应失效")
	}
}
