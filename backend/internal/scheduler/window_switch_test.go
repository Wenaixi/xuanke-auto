package scheduler

import (
	"testing"
	"time"
)

// 本文件钉住「切平台清空窗口状态位」这一契约。
//
// 回归动机：accounts.SetProfile 只重建客户端、切平台 token 置空，从不通知调度器；
// server.go 的 onProfile 回调曾只调 SetHasWindowSignal。窗口状态是**平台事实**——
// 开放时间识别值按既定契约「关闭≠时间消失」永久保留、openTimeDetected 无 TTL 兜底，
// 于是切到开窗时间不同的平台后，仍按旧平台时刻判开窗/关闭与提交守卫（实测残留最坏
// 约 30 秒，靠下一轮探测自愈，但自愈不是契约）。
//
// 变异验证：删掉 ResetWindowState 里的 ws.setOpened(false) 或 purgeAll，
// 本文件必须红。

// TestResetWindowStateClearsPlatformFacts 切换平台后旧平台的窗口事实立即不可外显。
//
// 必须先 probe()：窗口状态位只由包内 probe() 更新（见既有分工约定），
// StateForAccount 只是读侧。
func TestResetWindowStateClearsPlatformFacts(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	// 未来时刻：StateForAccount 的 open_time_known 依 now 判定「识别值已过期 = 未识别」，
	// 用过去时间会让该标志恒 false（分不清「没清干净」与「本来就过期」）。
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
	fc.mu.Unlock()

	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)
	s.probe()

	st := s.StateForAccount("acct1")
	if !st.WindowOpened {
		t.Fatalf("前置条件：探测 selectable=true 后窗口应判为已开，实际 %+v", st)
	}
	if !st.OpenTimeKnown {
		t.Fatalf("前置条件：识别到 beginTimes 后 open_time_known 应为 true，实际 %+v", st)
	}

	// 切平台：装配面在 onProfile 回调里调它（server.go）。
	s.ResetWindowState()

	if s.WindowOpened() {
		t.Error("切平台后 WindowOpened 仍为 true：会按旧平台继续放行提交")
	}
	st = s.StateForAccount("acct1")
	if st.WindowOpened {
		t.Error("state.WindowOpened 仍为 true：/state 下发的状态与 ws 自相矛盾")
	}
	if st.WindowClosed {
		t.Error("切平台后 WindowClosed 应复位（旧平台的关闭标记对新平台无意义）")
	}
	if st.OpenTimeKnown {
		t.Error("切平台后 open_time_known 应为 false（旧平台识别值对新平台无意义）")
	}
	if !st.OpenTime.IsZero() {
		t.Errorf("切平台后 OpenTime 应归零，实际 %v", st.OpenTime)
	}
	// 识别槽必须整槽清空：openTimeForLocked 的优先级是「账号槽 → 全校槽」，
	// 只清全校槽会漏掉全部账号槽。两个槽都要断言。
	if got := s.openTimeForLocked(""); !got.IsZero() {
		t.Errorf("全校开放时间识别槽应清空，实际 %v", got)
	}
	if got := s.openTimeForLocked("acct1"); !got.IsZero() {
		t.Errorf("账号开放时间识别槽应清空，实际 %v", got)
	}
}

// TestResetWindowStateThenProbeReflectsNewPlatform 清理后下一轮探测必须按新平台
// 数据重新判定，而不是被残留状态污染。关键在于让新数据的判定结果与残留值相反
// （残留 opened=true，新数据 selectable=false），否则测试抓不住"没清干净"。
func TestResetWindowStateThenProbeReflectsNewPlatform(t *testing.T) {
	fc := newFakeClient(true) // 旧平台：开窗
	fc.mu.Lock()
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
	fc.mu.Unlock()

	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)
	s.probe()
	if !s.WindowOpened() {
		t.Fatal("前置条件：旧平台应判为已开窗")
	}

	s.ResetWindowState()

	// 新平台：窗口未开（selectable=false）。不清干净的话 opened 会残留 true。
	fc.setOpen(false)
	s.probe()

	if s.WindowOpened() {
		t.Error("清理后按新平台数据（selectable=false）重探测，仍判为已开窗：残留未清")
	}
	if !s.StateForAccount("acct1").OpenTimeKnown {
		t.Error("新平台重新识别到 beginTimes 后 open_time_known 应恢复为 true")
	}
}

// TestResetWindowStateClearsSyncFailStreak 时钟连续失败计数也是平台事实：
// 旧平台连续同步失败 3 次会把新平台的关闭判定直接拖成"时钟兜底关闭"。
func TestResetWindowStateClearsSyncFailStreak(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	fc.data.BeginTimes = []int64{time.Now().Add(-time.Hour).UnixMilli()}
	fc.mu.Unlock()

	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)
	s.mu.Lock()
	for i := 0; i < 3; i++ {
		s.ws.noteSyncFailure()
	}
	streak := s.ws.syncFailStreakCount()
	s.mu.Unlock()
	if streak < 3 {
		t.Fatalf("前置条件：时钟失败连续数应 ≥3，实际 %d", streak)
	}

	s.ResetWindowState()

	s.mu.Lock()
	got := s.ws.syncFailStreakCount()
	s.mu.Unlock()
	if got != 0 {
		t.Errorf("切平台后时钟失败连续数应归零，实际 %d（旧平台计数会把新平台拖成时钟兜底关闭）", got)
	}
}