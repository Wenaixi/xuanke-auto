package session

import (
	"errors"
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

func TestAdminSession(t *testing.T) {
	s := New(time.Hour)
	adminTok := s.CreateAdmin()
	if !s.IsAdmin(adminTok) {
		t.Fatal("管理员令牌应 IsAdmin=true")
	}
	if acct, ok := s.Account(adminTok); !ok || acct != "admin" {
		t.Fatalf("管理员会话账号应为 admin: %q %v", acct, ok)
	}
	// 普通会话不应被当作管理员
	userTok := s.Create("acct1")
	if s.IsAdmin(userTok) {
		t.Fatal("普通会话不应 IsAdmin=true")
	}
	if s.IsAdmin("bogus") {
		t.Fatal("无效令牌不应 IsAdmin=true")
	}
}

func TestIsAdminToken(t *testing.T) {
	s := New(time.Hour)
	adminTok := s.CreateAdmin()
	if !s.IsAdminToken(adminTok) {
		t.Fatal("管理员令牌 IsAdminToken 应为 true")
	}
	if s.IsAdminToken(s.Create("acct1")) {
		t.Fatal("普通会话 IsAdminToken 不应为 true")
	}
	if s.IsAdminToken("bogus") {
		t.Fatal("无效令牌 IsAdminToken 不应为 true")
	}
}

func TestRevokeAccount(t *testing.T) {
	s := New(time.Hour)
	tokA := s.Create("acctA")
	tokB := s.Create("acctB")
	adminTok := s.CreateAdmin()

	// 吊销 acctA：acctA 全部会话失效，其他账号不受影响
	s.RevokeAccount("acctA")
	if _, ok := s.Account(tokA); ok {
		t.Fatal("吊销后 acctA 会话应失效")
	}
	if acct, ok := s.Account(tokB); !ok || acct != "acctB" {
		t.Fatalf("吊销 acctA 不应影响 acctB: %q %v", acct, ok)
	}
	if !s.IsAdminToken(adminTok) {
		t.Fatal("吊销 acctA 不应影响管理员会话")
	}
	// 吊销不存在的账号不报错
	s.RevokeAccount("no-such-account")
	// 同名账号再登录（新令牌）不受吊销影响（旧令牌已失效、新令牌有效）
	reTok := s.Create("acctA")
	if _, ok := s.Account(reTok); !ok {
		t.Fatal("吊销后重新登录签发的令牌应有效")
	}
}

func TestIsAdminAccount(t *testing.T) {
	s := New(time.Hour)
	if !s.IsAdminAccount(s.Create("admin")) {
		t.Fatal("名为 admin 的普通会话账号应被识别为 admin 名")
	}
	if s.IsAdminAccount(s.Create("student")) {
		t.Fatal("名为 student 的会话不应被识别为 admin 名")
	}
	if s.IsAdminAccount("bogus") {
		t.Fatal("无效令牌不应被识别为任何账号")
	}
}

func TestRandTokenPanicsOnRandFailure(t *testing.T) {
	// 注入失败的 crypto/rand 读取器：rand.Read 必须 panic（与 config.randomAdminToken 同策略，
	// 绝不静默生成全零可预测令牌）。
	old := randReader
	randReader = func(b []byte) (int, error) { return 0, errors.New("entropy source down") }
	defer func() { randReader = old }()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("crypto/rand 失败时必须 panic，拒绝生成可预测令牌")
		}
	}()
	randToken()
}
