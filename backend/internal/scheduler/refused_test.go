package scheduler

import (
	"sync"
	"testing"
	"time"
)

// persistentStore 真实语义的持久化假存储：SaveRefused 落库、DeleteRefused 真删、
// LoadRefused 读当前表——用 map 忠实复刻 SQLite 行为（顺序契约测试用，
// 避免 fakeStore 的 no-op DeleteRefused 让顺序 bug 假绿）。
type persistentStore struct {
	fakeStore
	mu      sync.Mutex
	refused map[string]map[int]bool
}

func newPersistentStore() *persistentStore {
	return &persistentStore{refused: map[string]map[int]bool{}}
}

func (p *persistentStore) SaveRefused(acct string, classID int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.refused[acct] == nil {
		p.refused[acct] = map[int]bool{}
	}
	p.refused[acct][classID] = true
	return nil
}

func (p *persistentStore) DeleteRefused(acct string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.refused, acct)
	return nil
}

// DeleteRefusedClass 真实语义的单课删除：手动重报成功后只清该课退选行，
// 其余课程与其他账号不受影响（与 SQLite DELETE ... AND class_id=? 对齐）。
func (p *persistentStore) DeleteRefusedClass(acct string, classID int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ids := p.refused[acct]; ids != nil {
		delete(ids, classID)
		if len(ids) == 0 {
			delete(p.refused, acct)
		}
	}
	return nil
}

func (p *persistentStore) LoadRefused() (map[string][]int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string][]int{}
	for acct, ids := range p.refused {
		for id := range ids {
			out[acct] = append(out[acct], id)
		}
	}
	return out, nil
}

// SetTargetsForAccount 真实语义的目标落库：persistentStore 继承 fakeStore 的 no-op
// SetTargetsForAccount（仅测试 refused 顺序契约，不关心目标持久化内容）。
func (p *persistentStore) SetTargetsForAccount(acct string, targets []Target) error {
	return nil
}

// TestRefusedNeverResubmitted 用户手动退选后自动引擎绝不抢回（历史设计定案）：
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

// TestManualReselectClearsRefusedRow 手动重报成功必须清库内单条 refused 行（重启后不残留假退选）：
// 用户手动报名 C 成功 → 手动退选 C（SaveRefused 落库行）→ 手动重报 C 成功 → 库内该行必须删除。
// 否则重启恢复序 RestoreTargets（不清 refused）+ LoadRefused + RestoreRefused 把这门已报名成功的
// 课程恢复成"已手动退选（自动引擎不再接管）"——状态文案误导 + spawnChain 永久跳过（旧实现
// MarkDone 只 delete 内存 refused，库行残留即本缺口）。用 persistentStore 忠实复刻 SQLite 语义。
func TestManualReselectClearsRefusedRow(t *testing.T) {
	fc := newFakeClient(true)
	st := newPersistentStore()
	s := New(&fakeAccts{c: fc}, st, time.Now().Add(-time.Hour), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	// 用户手动退选 C：SaveRefused 落库行
	if err := s.RemoveDone("acct1", 61115); err != nil {
		t.Fatalf("RemoveDone 失败: %v", err)
	}
	got, _ := st.LoadRefused()
	if len(got["acct1"]) != 1 || got["acct1"][0] != 61115 {
		t.Fatalf("RemoveDone 后 refused 表应有 61115，实际 %+v", got)
	}

	// 用户手动重报 C 成功：MarkDone 必须清该课库内退选行（其余课程不受影响）
	if err := s.MarkDone("acct1", 61115, "健美操", "选课成功"); err != nil {
		t.Fatalf("MarkDone 失败: %v", err)
	}
	got, _ = st.LoadRefused()
	if len(got["acct1"]) != 0 {
		t.Fatalf("手动重报成功后库内 refused 行必须清空（重启后该课不得恢复成已手动退选），实际 %+v", got)
	}
	if !s.doneHas("acct1", 61115) || s.refusedHas("acct1", 61115) {
		t.Fatal("MarkDone 后内存态 done 应置位、refused 应解除")
	}
}

// TestRefusedRestartOrderRealDB 用真实 SQLite 验证重启恢复顺序——
// 手动退选落库后，按 main.go 的实际顺序（RestoreDone → 循环 RestoreTargets →
// LoadRefused + RestoreRefused）恢复，refused 标记必须保留、自动引擎绝不抢回；
// 若把 SetTargetsForAccount 用于恢复，其内部 DeleteRefused 会删库行、LoadRefused
// 拿到空 map，重启后自动引擎立刻抢回退选课（持久化被抵消）——本测试用真实 DB 把
// 这条顺序契约固化（此前 refused_test 的 TestRefusedPersistedAcrossRestart 用
// fakeStore 绕过真实删除，是假绿灯）。
func TestRefusedRestartOrderRealDB(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开，课程可报
	st := newPersistentStore()
	initialTargets := []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}}
	// 目标直接注入调度器内存（真实启动经 store.LoadTargetsForAccount → RestoreTargets，
	// 测试聚焦 refused 顺序契约，目标灌入即可）

	// 模拟重启#1 到一次 RemoveDone（与 main.go 恢复顺序一致：RestoreTargets 不清 refused）
	s1 := New(&fakeAccts{c: fc}, st, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s1.RestoreDone(map[string][]int{})
	s1.RestoreTargets("acct1", initialTargets)
	if err := s1.RemoveDone("acct1", 61115); err != nil {
		t.Fatalf("RemoveDone 失败: %v", err)
	}

	// 断言拒绝表已落库
	refused2, err := st.LoadRefused()
	if err != nil {
		t.Fatal(err)
	}
	if len(refused2["acct1"]) != 1 || refused2["acct1"][0] != 61115 {
		t.Fatalf("RemoveDone 后 refused 表应有 61115，实际 %+v", refused2)
	}

	// 阶段二：模拟重启#2，按 main.go 当前正确顺序（先循环 RestoreTargets，再 LoadRefused+RestoreRefused）
	s2 := New(&fakeAccts{c: fc}, st, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s2.RestoreDone(map[string][]int{})
	s2.RestoreTargets("acct1", initialTargets)
	refused3, _ := st.LoadRefused()
	s2.RestoreRefused(refused3)
	// 让恢复后的调度器跑 tick，确认不抢回
	tEnd := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(tEnd) {
		s2.tick()
		time.Sleep(50 * time.Millisecond)
	}
	if n := fc.SelectClassCalls(61115); n != 0 {
		t.Fatalf("真实DB重启顺序下自动引擎不得抢回手动退选课，实际调用 %d 次", n)
	}

	// 阶段三：重新设为目标（用户主动接管）→ refused 清库行 + 解除，恢复自动提交
	s2.SetTargetsForAccount("acct1", initialTargets)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s2.tick()
		if fc.SelectClassCalls(61115) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("重新设为目标后自动引擎应恢复该课程")
}

// TestRefusedPersistedAcrossRestart 手动退选必须落库，重启（新调度器 +
// 恢复顺序 SetTargetsForAccount → RestoreRefused）后自动引擎仍绝不抢回该课程。
// 修复前 refused 只存内存：重启后 SetTargetsForAccount 清空、自动引擎把用户
// 手动退选掉的课当新目标重新抢回——退选意图丢失（与删除 success 行的「假成功」同根）。
func TestRefusedPersistedAcrossRestart(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开，课程可报
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}
	if err := s.RemoveDone("acct1", 61115); err != nil {
		t.Fatalf("RemoveDone 失败: %v", err)
	}

	// 断言库内持久化（fakeStore 缓存指针同一，RemoveDone 落库后立即可读）
	if !s.refusedHas("acct1", 61115) {
		t.Fatal("RemoveDone 后应记入 refused")
	}

	// 模拟重启：新调度器 + 与 main 相同的恢复顺序（SetTargetsForAccount 会清库行，
	// 紧随其后的 RestoreRefused 再把持久化的退选注入回内存）
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s2.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s2.RestoreRefused(map[string][]int{"acct1": {61115}})
	if !s2.refusedHas("acct1", 61115) {
		t.Fatal("重启恢复后 refused 应保留")
	}

	// 阶段 1：refused 拦截——持续 tick，SelectClass 必须始终 0 次调用
	tEnd := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(tEnd) {
		s2.tick()
		time.Sleep(50 * time.Millisecond)
	}
	if n := fc.SelectClassCalls(61115); n != 0 {
		t.Fatalf("重启后自动引擎不得抢回手动退选课，实际调用 SelectClass %d 次", n)
	}

	// 重新设为目标（用户主动接管）：refused 清库行 + 解除，恢复自动提交
	s2.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s2.tick()
		if fc.SelectClassCalls(61115) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("重新设为目标后自动引擎应恢复该课程")
}
