package accounts

import (
	"testing"

	"xuanke-auto/backend/internal/sites"
	"xuanke-auto/backend/internal/upstream"
)

// TestSetProfileRebuildsClientsAndDropsTokens 切平台语义（顺序即契约）：
//  1. memory-first：旧客户端立即不可达、被新实例替换（在飞链的锁内身份复核随之失败）；
//  2. 新客户端只带账密、token 置空——跨平台会话必废，物理上不存在「A 平台 token 发往 B 平台」；
//  3. 落库一次 SaveCredential（空 token + 新平台 ID），既清库内旧 token 又打上新标记。
func TestSetProfileRebuildsClientsAndDropsTokens(t *testing.T) {
	srv := loginRejectSrv(t)
	st := &fakeStore{}
	m := visionSrvManager(t, srv.URL, st)
	seedValidClient(t, m, srv, "acct1")
	old, ok := m.ClientFor("acct1")
	if !ok {
		t.Fatal("夹具未注册客户端")
	}

	desc := testSite(t)
	creds := []Credential{{Account: "acct1", PasswordEnc: "ENC-PWD", IDToken: "tok-old-platform", PlatformID: "other-platform"}}
	m.SetProfile(desc, srv.URL, creds, func(s string) (string, error) { return "pwd", nil })

	cur, ok := m.ClientFor("acct1")
	if !ok {
		t.Fatal("切档案后该账号客户端必须已重建")
	}
	if cur == old {
		t.Fatal("旧客户端必须被替换（memory-first：绝不复用旧档案实例）")
	}
	if tok := cur.Token(); tok != "" {
		t.Fatalf("切档案后 token 必须为空（跨平台会话必废），实际 %q", tok)
	}
	if !st.saved {
		t.Fatal("切档案必须落库（清 token + 打新平台标记），否则重启后又把旧平台 token 当自己的用")
	}
	if st.platform != sites.DefaultID {
		t.Fatalf("落库应打上新平台标记，实际 %q", st.platform)
	}
	if st.idToken != "" {
		t.Fatalf("落库应清空 token，实际 %q", st.idToken)
	}
}

// TestRestoreDropsForeignPlatformToken 重启恢复 / 切档后重启：库内 token 属于别的平台
// 时绝不注入（既防凭据外泄，也避免必然 401 白跑一轮），账密照常注入以支持自动重登。
func TestRestoreDropsForeignPlatformToken(t *testing.T) {
	m := visionSrvManager(t, "http://127.0.0.1:1", &fakeStore{})
	m.Restore([]Credential{
		{Account: "foreign", PasswordEnc: "ENC", IDToken: "tok-from-other-site", PlatformID: "other-platform"},
		{Account: "same", PasswordEnc: "ENC", IDToken: "tok-same-site", PlatformID: sites.DefaultID},
	}, func(s string) (string, error) { return "pwd", nil })

	foreign, ok := m.ClientFor("foreign")
	if !ok {
		t.Fatal("凭据仍应恢复出客户端（账密可用于自动重登）")
	}
	if tok := foreign.Token(); tok != "" {
		t.Fatalf("跨平台 token 必须丢弃，实际 %q", tok)
	}
	same, ok := m.ClientFor("same")
	if !ok {
		t.Fatal("同平台客户端应恢复")
	}
	if tok := same.Token(); tok != "tok-same-site" {
		t.Fatalf("同平台 token 必须保留（否则每次重启都白重登一次），实际 %q", tok)
	}
}

// TestRestoreTreatsLegacyEmptyPlatformAsForeign 旧库迁移出的空 platform_id（平台未知）
// 必须按跨平台处理：宁可多一次自动重登，也不能把可能属于别的站点的会话发出去。
func TestRestoreTreatsLegacyEmptyPlatformAsForeign(t *testing.T) {
	m := visionSrvManager(t, "http://127.0.0.1:1", &fakeStore{})
	m.Restore([]Credential{{Account: "legacy", PasswordEnc: "ENC", IDToken: "tok-legacy", PlatformID: ""}},
		func(s string) (string, error) { return "pwd", nil })
	c, ok := m.ClientFor("legacy")
	if !ok {
		t.Fatal("旧库凭据仍应恢复出客户端")
	}
	if tok := c.Token(); tok != "" {
		t.Fatalf("平台未知的 token 必须丢弃，实际 %q", tok)
	}
}

// TestSetProfileUsesProfileDefaultWhenBaseURLEmpty 空地址覆盖 = 用档案默认地址
// （后台把地址输入框留空保存的语义），绝不是"空主机名"。
func TestSetProfileUsesProfileDefaultWhenBaseURLEmpty(t *testing.T) {
	m := visionSrvManager(t, "http://127.0.0.1:1", &fakeStore{})
	desc := testSite(t)
	m.SetProfile(desc, "", nil, func(s string) (string, error) { return "pwd", nil })
	m.mu.Lock()
	got := m.baseURL
	m.mu.Unlock()
	if got != upstream.NormalizeBaseURL(desc.DefaultBaseURL) {
		t.Fatalf("空覆盖应回落到档案默认地址 %q，实际 %q", desc.DefaultBaseURL, got)
	}
}
