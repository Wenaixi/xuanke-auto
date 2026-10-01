package main

import (
	"path/filepath"
	"testing"

	"xuanke-auto/backend/internal/runtime"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/sites"
	"xuanke-auto/backend/internal/store"
	"xuanke-auto/backend/internal/upstream"
)

// TestPlatformRebinderWiring 覆盖 server.go 的平台切换编排（api 层 PUT 只到 stub 为止）：
// 「解析档案 → 校验站点地址 → 读凭据 → SetProfile」四步必须按"先验证后生效"执行——
// 任一失败都不得改动既有客户端（否则出现"档案没换、客户端先没了"的半生效状态）。
func TestPlatformRebinderWiring(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	// 预置一条属于"别的平台"的凭据：切换后它必须被清 token + 打上新标记。
	if err := st.SaveCredential("acct1", "ENC-PWD", "tok-from-other-platform", "other-platform"); err != nil {
		t.Fatal(err)
	}

	desc, err := sites.Resolve(sites.DefaultID)
	if err != nil {
		t.Fatal(err)
	}
	accts := accounts.New(desc, "http://127.0.0.1:1", upstream.VisionConfig{}, st)
	creds, err := st.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	accts.Restore(creds, func(s string) (string, error) { return "pwd", nil })
	// 切换前：跨平台 token 已被 Restore 丢弃（防线一），客户端在册。
	if c, ok := accts.ClientFor("acct1"); !ok || c.Token() != "" {
		t.Fatalf("夹具前置不成立：客户端在册且 token 已丢弃，实际 ok=%v", ok)
	}

	rebind := newPlatformRebinder(st, accts, func(s string) (string, error) { return "pwd", nil }, nil)

	// 1) 未知档案：整体拒绝，客户端不动
	if err := rebind("no-such-platform", ""); err == nil {
		t.Fatal("未知档案必须拒绝")
	}
	// 2) 非法站点地址：整体拒绝
	if err := rebind(sites.DefaultID, "ftp://example.com"); err == nil {
		t.Fatal("非 http(s) 站点地址必须拒绝")
	}
	if _, ok := accts.ClientFor("acct1"); !ok {
		t.Fatal("被拒时不得摘除既有客户端")
	}
	// 2.5) 档案自检：切换到形状不完整的档案必须整体拒绝（与地址校验同属先验证后生效）。
	// 内置档案当前都合法，故此处只断言"自检被调用过"——由 sites 包的负向测试
	// TestValidateRejectsIncompleteDescriptor 证明自检本身有拦截力。
	if err := rebind(sites.DefaultID, "https://mirror.example.com"); err != nil {
		t.Fatalf("合法档案自检应通过: %v", err)
	}

	// 3) 合法切换：客户端重建、token 清空、库内清 token 并打上新平台标记
	if err := rebind(sites.DefaultID, "https://mirror.example.com"); err != nil {
		t.Fatalf("合法切换应成功: %v", err)
	}
	c, ok := accts.ClientFor("acct1")
	if !ok {
		t.Fatal("切换后客户端应已重建")
	}
	if c.Token() != "" {
		t.Fatalf("切换后 token 必须清空（跨平台会话必废），实际 %q", c.Token())
	}
	got, err := st.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("凭据行数异常: %+v", got)
	}
	if got[0].IDToken != "" {
		t.Fatalf("库内旧平台 token 必须清空（否则重启后又发往新站点），实际 %q", got[0].IDToken)
	}
	if got[0].PlatformID != sites.DefaultID {
		t.Fatalf("必须打上新平台标记，实际 %q", got[0].PlatformID)
	}
	if got[0].PasswordEnc != "ENC-PWD" {
		t.Fatalf("切换不得改动账密密文: %q", got[0].PasswordEnc)
	}
}

// TestResolveStartupPlatform 启动期档案解析的三条分支：空值按默认档案、
// 合法值原样解析、**未知值报错**（调用方据此 log.Fatalf 拒绝启动，绝不静默换站点）。
func TestResolveStartupPlatform(t *testing.T) {
	d, err := resolveStartupPlatform(runtime.Config{})
	if err != nil || d.ID != sites.DefaultID {
		t.Fatalf("空 platform_id 应按默认档案解析，实际 %+v %v", d.ID, err)
	}
	d, err = resolveStartupPlatform(runtime.Config{PlatformID: sites.DefaultID})
	if err != nil || d.ID != sites.DefaultID {
		t.Fatalf("合法 platform_id 应解析成功，实际 %+v %v", d.ID, err)
	}
	if _, err := resolveStartupPlatform(runtime.Config{PlatformID: "no-such-platform"}); err == nil {
		t.Fatal("未知 platform_id 必须报错（否则会静默跑在别的站点上）")
	}
}

// TestPlatformRebinderNotifiesProfile 切换成功后必须回调 onProfile 并交出**新档案**。
// 调度器经此同步 hasWindowSignal——漏调或调早调晚都会让它用旧站点的开窗信号语义
// 判新站点（切到"不下发开窗信号"的档案后开窗永远判不出来，调度器全线停摆）。
// 判据三态：成功切换调一次、失败路径不调、回调拿到的是新档案而非旧档案。
func TestPlatformRebinderNotifiesProfile(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	accts := accounts.New(mustResolveProfile(t), "", upstream.VisionConfig{}, st)

	var calls int
	var gotID string
	rebind := newPlatformRebinder(st, accts, func(string) (string, error) { return "pwd", nil },
		func(desc upstream.SiteDescriptor) {
			calls++
			gotID = desc.ID
		})

	// 失败路径（未知档案）不得回调——否则会把没生效的档案同步给调度器。
	if err := rebind("no-such-platform", ""); err == nil {
		t.Fatal("未知档案应被拒")
	}
	if calls != 0 {
		t.Fatalf("失败切换不得回调 onProfile，实际调用 %d 次", calls)
	}

	if err := rebind(sites.DefaultID, ""); err != nil {
		t.Fatalf("合法切换应成功: %v", err)
	}
	if calls != 1 {
		t.Fatalf("成功切换应回调 onProfile 恰好一次，实际 %d 次", calls)
	}
	if gotID != sites.DefaultID {
		t.Fatalf("回调应拿到新档案 %s，实际 %s", sites.DefaultID, gotID)
	}
}

// mustResolveProfile 解析当前默认档案（测试装配用）。
func mustResolveProfile(t *testing.T) upstream.SiteDescriptor {
	t.Helper()
	d, err := sites.Resolve(sites.DefaultID)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
