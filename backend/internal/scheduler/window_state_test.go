package scheduler

import (
	"testing"
	"time"
)

// windowState 状态机独立实例化测试（脱离 scheduler 时序，直测 noteXxx 入账与
// isClosed 三判据）。迁移等价性的硬基线 = 既有 18 个窗口测试（scheduler_test.go），
// 本文件只测 struct 自身契约。

func TestWindowStateNoteProbeLifecycle(t *testing.T) {
	w := newWindowState()
	if w.isClosed(time.Time{}, time.Now()) {
		t.Fatal("初始未开过窗不应判关闭")
	}
	w.setOpened(true)
	if !w.isOpened() {
		t.Fatal("setOpened(true) 应置 opened")
	}
	// 空快照入账 3 次 → 幽灵窗口判据（须带开放时间已过，isClosed 内判 now.After(open)）
	open := time.Now().Add(-2 * time.Minute)
	for range 3 {
		w.noteProbeEmpty()
	}
	if w.isClosed(open, time.Now()) {
		t.Fatal("opened=true 时 emptyProbeRuns 计数不得触发幽灵窗口判据（幽灵判据要求从未开窗）")
	}
	// 开过窗但当前轮 opened 回落 false（开再关）+ 空快照轮数仍 3 → 判据③
	//（!opened && runs>=3 && 开放时间已过）成立视同关闭——这正是"开再关"形态的持久
	// 关闭信号（主判据 closed 只认本轮的 prevOpened 表达式，跨轮持久性由量变兜底）。
	w.setOpened(false)
	if !w.isClosed(open, time.Now()) {
		t.Fatal("开过窗但当前轮回落 false + 空快照 3 轮 + 开放时间已过应视同关闭（量变判据兜底持久关闭）")
	}
	// 非空快照探测入账 → 归零自愈
	w.noteProbeReset()
	if w.emptyProbeRuns != 0 {
		t.Fatal("noteProbeReset 应清零 emptyProbeRuns")
	}
	if w.isClosed(open, time.Now()) {
		t.Fatal("空快照轮数归零后幽灵窗口判据应解除")
	}
	// 主判据：setClosed(true) 置位即判关闭（不依赖开放时间）
	w.setClosed(true)
	if !w.isClosed(time.Time{}, time.Now()) {
		t.Fatal("closed 置位即判关闭（不依赖开放时间）")
	}
	// 幽灵窗口：从未开过窗 + 3 次空快照 + 开放时间已过
	w2 := newWindowState()
	for range 3 {
		w2.noteProbeEmpty()
	}
	if !w2.isClosed(open, time.Now()) {
		t.Fatal("从未开窗 + emptyProbeRuns>=3 + 开放时间已过应判关闭")
	}
	if w2.isClosed(time.Now().Add(2*time.Minute), time.Now()) {
		t.Fatal("开放时间在未来不应判关闭（判据须带开放时间已过）")
	}
}

func TestWindowStateSyncFailureStreak(t *testing.T) {
	w := newWindowState()
	open := time.Now().Add(-2 * time.Minute)
	for range 3 {
		w.noteSyncFailure()
	}
	if !w.isClosed(open, time.Now()) {
		t.Fatal("syncFailStreak>=3 应判关闭（须带开放时间已过）")
	}
	if w.isClosed(time.Now().Add(2*time.Minute), time.Now()) {
		t.Fatal("时钟失败判据须带开放时间已过（未来开窗点不误挂）")
	}
	if w.syncFailStreakCount() != 3 {
		t.Fatal("syncFailStreakCount 应返回 3")
	}
	w.noteSyncSuccess()
	if w.syncFailStreakCount() != 0 {
		t.Fatal("同步成功应清零 streak")
	}
}

func TestWindowStateOpenTimeSlot(t *testing.T) {
	w := newWindowState()
	if !w.openTimeFor("acctA").IsZero() {
		t.Fatal("识别槽未写入时应返回零值")
	}
	ms := time.Now().Add(-time.Minute).UnixMilli()
	w.setOpenTime("acctA", ms)
	if got := w.openTimeFor("acctA"); got.UnixMilli() != ms {
		t.Fatalf("识别槽 acctA 应返回写入值，got %v", got)
	}
	// 账号槽优先于全校槽
	w.setOpenTime("*", ms-1000)
	if got := w.openTimeFor("acctA"); got.UnixMilli() != ms {
		t.Fatal("账号自识别槽应优先于全校槽")
	}
	// 未写入账号回退全校槽
	if got := w.openTimeFor("acctB"); got.UnixMilli() != ms-1000 {
		t.Fatal("未识别账号应回退全校槽 openTimeDetected[\"*\"]")
	}
	// 关闭≠时间消失：purge 只删账号槽，全校槽保留；全部清空后零值
	w.purge("acctA")
	if got := w.openTimeFor("acctA"); got.UnixMilli() != ms-1000 {
		t.Fatal("purge 账号槽后应回退全校槽")
	}
	w.purge("*")
	if !w.openTimeFor("acctA").IsZero() {
		t.Fatal("全校槽也清空后应返回零值")
	}
}
