package scheduler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// C4-2 TDD：ManualSelect / ManualExit 深方法（手动决策树收进调度器）。
// 手动路径此前在 handler 独立实现：取锁→快照复核→ClientFor→平台调用→三失败分支。
// 收权后调度器内部完成全部，handler 瘦回参数解析；并补核实挖出的缺口——
// 重登退避期（relogging/tokenValid 失效）手动点报名必须短路，绝不烧平台请求。

// 构造手动路径测试调度器：interval 大值禁轮询，手动方法纯同步调用。
func manualTestSched(t *testing.T, fc *fakeClient) (*Scheduler, *fakeAccts) {
	t.Helper()
	accts := &fakeAccts{c: fc}
	s := New(accts, &fakeStore{}, time.Time{}, time.Hour)
	return s, accts
}

// TestManualSelectSuccessClearsInflightAndRefused 手动报名成功：记 done + 清 inflight +
// 清 refused（手动重报接管语义，决策 14）+ 清 full + 返回平台 msg。
func TestManualSelectSuccessClearsInflightAndRefused(t *testing.T) {
	fc := newFakeClient(true)
	s, _ := manualTestSched(t, fc)
	acct := "acct1"
	// 预置 refused：手动重报成功必须解除（自动引擎恢复接管）
	s.mu.Lock()
	s.refused[acct] = map[int]bool{61115: true}
	s.full[acct] = map[int]bool{61115: true}
	s.mu.Unlock()

	msg, err := s.ManualSelect(acct, 61115, "健美操")
	if err != nil {
		t.Fatalf("手动报名成功不应报错，got %v", err)
	}
	if msg != "报名成功" {
		t.Fatalf("应返回平台成功文案，got %q", msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done[acct][61115] {
		t.Fatal("成功应记 done")
	}
	if s.inflight[acct][61115] {
		t.Fatal("成功应清 inflight")
	}
	if s.refused[acct][61115] {
		t.Fatal("手动重报成功应清 refused（自动引擎恢复接管，决策 14）")
	}
	if s.full[acct][61115] {
		t.Fatal("成功应清 full")
	}
}

// TestManualSelectRateLimitedMarksBackoff 风控文案 → 记 30s 退避 + 返回友好错误。
func TestManualSelectRateLimitedMarksBackoff(t *testing.T) {
	fc := newFakeClient(true)
	fc.selectErr[61115] = errors.New("操作过于频繁，请稍后重试")
	s, _ := manualTestSched(t, fc)
	acct := "acct1"

	_, err := s.ManualSelect(acct, 61115, "健美操")
	if err == nil {
		t.Fatal("风控文案应返回错误，而不是成功")
	}
	if !s.isRateLimitedLocked(acct, 61115, s.nowAlignedLocked()) {
		t.Fatal("风控文案应记 30s 退避（与自动链同语义）")
	}
	if s.inflightHas(acct, 61115) {
		t.Fatal("失败路径也应清 inflight（defer release 保证）")
	}
}

// TestManualSelectAuthTriggersRelogin token 失效 → 触发自动重登 + 返回"自动重登中"友好文案。
// 注：fakeAccts.Relogin 同步成功 → 重登成功后 tokenValid 被清（真实语义：重登成功即恢复有效）。
// 断言核心是"MaybeRelogin 被触发"（relogCalls>0）+ 返回文案，而非失效标记残留。
func TestManualSelectAuthTriggersRelogin(t *testing.T) {
	fc := newFakeClient(true)
	fc.selectErr[61115] = zhidao.ErrUnauthorized
	s, accts := manualTestSched(t, fc)
	acct := "acct1"

	_, err := s.ManualSelect(acct, 61115, "健美操")
	if err == nil {
		t.Fatal("token 失效应返回错误")
	}
	// 重登应已被触发——但 maybeRelogin 是异步（go func，与自动链同设计：锁内绝不发
	// 网络请求），fakeAccts.Relogin 在 goroutine 里，断言前必须轮询等待其完成。
	deadline := time.Now().Add(time.Second)
	for {
		accts.c.mu.Lock()
		relogs := accts.c.relogCalls
		accts.c.mu.Unlock()
		if relogs > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("token 失效应触发自动重登（异步 goroutine 应在 1s 内完成）")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// 返回文案必须引导"正在自动重登，请稍后重试"（与旧 handler 手动路径同文案）
	if !strings.Contains(err.Error(), "自动重登") {
		t.Fatalf("文案应提示自动重登，got %q", err.Error())
	}
}

// TestManualSelectSkippedDuringRelogin relogging 期间手动点报名必须短路（核实挖出的缺口）——
// 自动链重登退避期手动点报名绝不烧平台请求。tokenValid[acct]=true（已知失效）时短路。
func TestManualSelectSkippedDuringRelogin(t *testing.T) {
	fc := newFakeClient(true)
	s, accts := manualTestSched(t, fc)
	acct := "acct1"
	s.mu.Lock()
	s.tokenValid[acct] = true // 已知失效（等效 relogging=true）
	s.mu.Unlock()

	_, err := s.ManualSelect(acct, 61115, "健美操")
	if err == nil {
		t.Fatal("token 已知失效时手动报名应短路返回错误，绝不放行")
	}
	// 平台 SelectClass 绝不应被调用（烧请求）
	if got := fc.SelectClassCalls(61115); got != 0 {
		t.Fatalf("token 失效期不应真实调用平台报名，实际 %d 次", got)
	}
	_ = accts
}

// TestManualExitSuccessRemovesDone 手动退选成功：清 done + 置 refused（自动引擎不再接管）+ 清 inflight + AppendLog。
func TestManualExitSuccessRemovesDone(t *testing.T) {
	fc := newFakeClient(true)
	s, _ := manualTestSched(t, fc)
	acct := "acct1"
	s.mu.Lock()
	s.done[acct] = map[int]bool{61115: true}
	s.mu.Unlock()

	msg, err := s.ManualExit(acct, 61115)
	if err != nil {
		t.Fatalf("手动退选成功不应报错，got %v", err)
	}
	if msg != "退选成功" {
		t.Fatalf("应返回平台退选文案，got %q", msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[acct][61115] {
		t.Fatal("退选成功应清 done")
	}
	if !s.refused[acct][61115] {
		t.Fatal("退选成功应记 refused（自动引擎不再接管，决策 14）")
	}
	if s.inflight[acct][61115] {
		t.Fatal("退选成功应清 inflight")
	}
}

// TestManualSelectRejectsGhostAccount 账号不存在（注册表已删除）→ 明确报错，不 panic。
// fakeAccts 对未注册账号默认回退共享 fakeClient（ClientFor 返回 (c,true)），
// 无法表达"未注册"；用 removed 标记模拟"账号已删除"，真实 accounts.Manager 语义对称。
func TestManualSelectRejectsGhostAccount(t *testing.T) {
	fc := newFakeClient(true)
	s, _ := manualTestSched(t, fc)
	// removed["ghost"]=true → ClientFor 返回 (nil,false)（fakeAccts:325-327 同真实语义）
	s.clients.(*fakeAccts).removed = map[string]bool{"ghost": true}
	_, err := s.ManualSelect("ghost", 61115, "健美操")
	if err == nil {
		t.Fatal("不存在的账号手动报名应报错")
	}
	if got := fc.SelectClassCalls(61115); got != 0 {
		t.Fatalf("不存在的账号不应调用平台，实际 %d 次", got)
	}
}