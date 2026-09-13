package scheduler

import (
	"testing"
	"time"
)

// TestRefusedNeverResubmitted 用户手动退选后自动引擎绝不抢回（第 4 轮 MAJOR A2）：
// RemoveDone 记入 refused 集合，spawnChain 命中即跳过——即使窗口开放、课程未满、
// 快照显示可报，也绝不再次调用 SelectClass；重新设为目标后 refused 解除，恢复自动接管。
//
// 提交闸门说明：submitInterval 在开窗黄金期后为 1s，手动调用 s.tick() 受闸门限制——
// 阶段 1 用"持续 tick + 足够长等待"证明被 refused 拦下（SubmitAll 恒更新 lastSubmit，
// 拦截路径不会产生误断言）；阶段 2 用轮询等闸门放行后确认 SelectClass 重新被调用。
func TestRefusedNeverResubmitted(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开，课程可报
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	// 填充快照（窗口开启，61115 可报、未满）
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// 模拟用户手动退选（RemoveDone 由 /api/electives/select/exit 调用）
	if err := s.RemoveDone("acct1", 61115); err != nil {
		t.Fatalf("RemoveDone 失败: %v", err)
	}
	if !s.refusedHas("acct1", 61115) {
		t.Fatal("RemoveDone 后应记入 refused 集合")
	}

	// 阶段 1：refused 拦截。开窗过去 1 小时 → 提交闸门 1s，跑足够多轮 tick。
	// 每次都命中 refused 而 skip，SelectClass 必须始终保持 0 次调用。
	tEnd := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(tEnd) {
		s.tick()
		time.Sleep(50 * time.Millisecond)
	}
	if n := fc.SelectClassCalls(61115); n != 0 {
		t.Fatalf("用户手动退选后自动引擎不应再次报名，实际调用 SelectClass %d 次", n)
	}

	// 重新设为目标：refused 解除，自动引擎恢复接管（下个 tick 重新提交）
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if s.refusedHas("acct1", 61115) {
		t.Fatal("重新设为目标后 refused 应被解除")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.tick()
		if fc.SelectClassCalls(61115) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("重新设为目标后自动引擎应在下个提交窗口恢复该课程")
}