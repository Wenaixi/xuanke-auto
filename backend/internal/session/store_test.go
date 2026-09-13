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
	adminTok := s.CreateAdmin("admin")
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
	adminTok := s.CreateAdmin("admin")
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
	adminTok := s.CreateAdmin("admin")

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

// TestIsAdminAccount 已随 M-3 移除 IsAdminAccount 方法而删除：
// 管理员判定统一走 Deps.IsAdminAccountName（配置名）与 Store.IsAdminToken（会话身份），
// 不再有"会话账号=字面量 admin"这套陈旧判定（避免两套真相错位）。

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

// TestActivationTicket 激活票据（C-2）：登录颁发、绑定账号、单次消费、短时过期。
func TestActivationTicket(t *testing.T) {
	s := New(time.Hour)

	// 1. 登录颁发：票据非空且有效
	tok := s.CreateTicket("acct1")
	if tok == "" {
		t.Fatal("票据不应为空")
	}
	// 2. 正确账号消费成功，且票据立即作废（单次）
	if err := s.ConsumeTicket(tok, "acct1"); err != nil {
		t.Fatalf("正确账号应能消费票据: %v", err)
	}
	if err := s.ConsumeTicket(tok, "acct1"); err == nil {
		t.Fatal("已使用的票据不得再次消费")
	}

	// 3. 换账号激活：票据绑定 acct1，用 acct2 消费必须失败
	tok2 := s.CreateTicket("acct1")
	if err := s.ConsumeTicket(tok2, "acct2"); err == nil {
		t.Fatal("票据与账号不匹配必须拒绝")
	}
	// 4. 不存在的票据
	if err := s.ConsumeTicket("bogus", "acct1"); err == nil {
		t.Fatal("不存在的票据必须拒绝")
	}
}

// TestActivationTicketExpiry 票据 5 分钟过期：过期后消费被拒，令牌可被重用。
func TestActivationTicketExpiry(t *testing.T) {
	s := New(time.Hour)
	tok := s.CreateTicket("acct1")
	// 直接改写过期时间（白盒测试访问票内部状态）
	s.mu.Lock()
	for _, tk := range s.tickets {
		tk.expires = time.Now().Add(-time.Minute)
	}
	s.mu.Unlock()
	if err := s.ConsumeTicket(tok, "acct1"); err == nil {
		t.Fatal("过期票据必须拒绝")
	}
}

// TestSweepExpired M-7：周期清扫删除过期会话与过期票据（后台协程 5 分钟一拍；
// 直接调 sweepExpired 验证清扫语义），且有效会话/票据不被误删。
func TestSweepExpired(t *testing.T) {
	s := New(time.Hour)
	defer s.Close()
	// 两个会话：一个已过期、一个有效
	expiredTok := s.Create("acct-expired")
	validTok := s.Create("acct-valid")
	// 两个票据：一个已过期、一个有效
	expiredTicket := s.CreateTicket("acct-expired")
	validTicket := s.CreateTicket("acct-valid")
	// 白盒改写过期时间
	s.mu.Lock()
	for tok, sess := range s.sessions {
		if tok == expiredTok {
			sess.Expires = time.Now().Add(-time.Minute)
		}
	}
	for tok, t := range s.tickets {
		if tok == expiredTicket {
			t.expires = time.Now().Add(-time.Minute)
		}
	}
	s.mu.Unlock()

	s.sweepExpired()

	// 过期项已被删除
	if _, ok := s.Account(expiredTok); ok {
		t.Fatal("过期会话应被清扫")
	}
	if err := s.ConsumeTicket(expiredTicket, "acct-expired"); err == nil {
		t.Fatal("过期票据应被清扫")
	}
	// 有效项完好
	if acct, ok := s.Account(validTok); !ok || acct != "acct-valid" {
		t.Fatalf("有效会话不应被误删: %q %v", acct, ok)
	}
	if err := s.ConsumeTicket(validTicket, "acct-valid"); err != nil {
		t.Fatalf("有效票据不应被误删: %v", err)
	}
}

// TestSweeperLoopStopsOnClose M-7：Close 停止清扫协程且不泄漏（-race 下运行，
// 若协程未退出会因访问已关闭 channel 报竞态/死锁）。
func TestSweeperLoopStopsOnClose(t *testing.T) {
	s := New(time.Hour)
	s.Close()
	s.Close() // 幂等
}
