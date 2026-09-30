package scheduler

import (
	"testing"
	"time"

	"xuanke-auto/backend/internal/upstream"
)

// TestDegradedModeEndToEnd 无开窗信号平台的端到端验收（解耦承诺的核心）。
//
// 场景：站点档案声明 HasWindowSignal=false（不下发任何开窗信号），但仍能正常
// 返回课程数据。这是「加平台不改引擎」的关键承诺——改造前引擎靠站点的
// in_date_range 判开窗，换成不下发该信号的平台会**永远判不出开窗**：
// 提交守卫挂起、黄金期冲刺停摆、手动报名被"窗口未开放"拒绝，调度器全线停摆。
//
// 判据四连：
//  1. 探测拿到非空快照；
//  2. WindowOpened()==true（退化模式把"有数据"当开窗）；
//  3. 手动报名**不被窗口信号拒绝**（改造前会失败的点）；
//  4. 满员课程仍按快照 ClassFull 拒绝（证明退化没有把复核整体放宽）。
func TestDegradedModeEndToEnd(t *testing.T) {
	fc := newFakeClient(true) // 夹具：窗口数据非空
	// 关键：把快照里所有发布的 selectable 置 nil（模拟"站点不下发该信号"），
	// 并让课程可选（夹具默认 CanSelect 零值 false，会被复核判"不可选"）。
	fc.mu.Lock()
	for i := range fc.data.Publishes {
		fc.data.Publishes[i].Selectable = nil
		for j := range fc.data.Publishes[i].Classes {
			fc.data.Publishes[i].Classes[j].CanSelect = true
			fc.data.Publishes[i].Classes[j].ClassFull = false
		}
	}
	fc.mu.Unlock()

	s := New(&fakeAccts{c: fc, perAccount: map[string]*fakeClient{"acct1": fc}}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	// 装配面的关键一行：按档案同步开窗信号能力。
	s.SetHasWindowSignal(false)

	// 窗口状态位只由包内 probe() 更新（ProbeNow / ProbeForAccount 只刷快照，
	// 这是既有分工：窗口是全校事实，账号级探测不参与判定）。
	s.probe()
	// checkClassSelectable 只读账号专属快照 acctData[acct]（年级物理隔离），
	// 故复核类断言还需账号级探测。
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("账号级探测应成功: %v", err)
	}

	// 1) 快照非空
	snap, ok := s.ElectivesSnapshotFor("acct1")
	if !ok || snap == nil || len(snap.Publishes) == 0 {
		t.Fatalf("应拿到非空快照，实际 ok=%v", ok)
	}
	// 三态 nil 必须如实保留（"站点不下发" ≠ "站点说未开"）
	if snap.Publishes[0].Selectable != nil {
		t.Fatalf("Selectable 应为 nil，实际 %v", *snap.Publishes[0].Selectable)
	}
	// 2) 退化模式判开窗
	if !s.WindowOpened() {
		t.Fatal("无开窗信号平台应走退化模式：探测到非空数据即视为开窗（否则调度器全线停摆）")
	}
	// 3) 手动报名不被"窗口未开放"拒绝
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if reason, selectable := s.checkClassSelectable("acct1", 61115); !selectable {
		t.Fatalf("无开窗信号平台不应被拒绝（改造前会返回 %q）", reason)
	}
	// 4) 满员课程仍按快照 ClassFull 拒绝（退化没有把复核整体放宽）。
	// 快照是探测时深拷贝的，故满员状态必须在**探测之前**就位——这里用夹具的
	// 第二个发布（61205 篮球）构造满员，验证复核仍按快照事实拒绝。
	fc.mu.Lock()
	fc.data.Publishes[1].Classes[0].ClassFull = true
	fc.mu.Unlock()
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("账号级探测应成功: %v", err)
	}
	if reason, selectable := s.checkClassSelectable("acct1", 61205); selectable {
		t.Fatal("满员课程应被拒绝（退化模式不得把复核整体放宽）")
	} else if reason == "" {
		t.Fatal("满员拒绝必须带原因文案")
	}
}

// TestHasWindowSignalPlatformStillPrecise 有开窗信号的档案必须走**精确**判定：
// 退化模式不得反向放宽——站点说未开（selectable=false）时仍要真的判未开窗。
func TestHasWindowSignalPlatformStillPrecise(t *testing.T) {
	fc := newFakeClient(false) // 夹具：窗口关闭
	s := New(&fakeAccts{c: fc, perAccount: map[string]*fakeClient{"acct1": fc}}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetHasWindowSignal(true)

	s.probe()
	// checkClassSelectable 只读账号专属快照 acctData[acct]（年级物理隔离），
	// 故复核类断言还需账号级探测。
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("账号级探测应成功: %v", err)
	}
	if s.WindowOpened() {
		t.Fatal("站点说未开（selectable=false）时不得判开窗（退化模式不得反向放宽）")
	}
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if reason, selectable := s.checkClassSelectable("acct1", 61115); selectable {
		t.Fatal("窗口未开时手动报名应被拒绝")
	} else if reason == "" {
		t.Fatal("拒绝必须带原因文案")
	}
}

// TestSelectableNilWithHasWindowSignalTrueHasWindowSignal 语义边界：
// 档案声明「有开窗信号」但某次快照的发布全是 nil（站点该次没给值）时，
// 按「未开窗」处理——**nil 不等于 true**（把它当 true 会让未开窗的平台抢跑）。
func TestSelectableNilWithHasWindowSignalTrueDoesNotOpen(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	for i := range fc.data.Publishes {
		fc.data.Publishes[i].Selectable = nil // 站点这次没给值
	}
	fc.mu.Unlock()

	s := New(&fakeAccts{c: fc, perAccount: map[string]*fakeClient{"acct1": fc}}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetHasWindowSignal(true)

	s.probe()
	// checkClassSelectable 只读账号专属快照 acctData[acct]（年级物理隔离），
	// 故复核类断言还需账号级探测。
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("账号级探测应成功: %v", err)
	}
	if s.WindowOpened() {
		t.Fatal("全 nil 快照不得判开窗（nil 是「站点没给值」，不是「站点说开」）")
	}
}

// TestDegradedModeEmptySnapshotNotOpened 退化模式的边界：空快照**不能**视为开窗。
// 否则窗口从未开启时就会不停提交，直到把平台打限流。
func TestDegradedModeEmptySnapshotNotOpened(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	fc.data.Publishes = nil // 空快照（窗口关闭形态）
	fc.mu.Unlock()

	s := New(&fakeAccts{c: fc, perAccount: map[string]*fakeClient{"acct1": fc}}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetHasWindowSignal(false)

	s.probe()
	// checkClassSelectable 只读账号专属快照 acctData[acct]（年级物理隔离），
	// 故复核类断言还需账号级探测。
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("账号级探测应成功: %v", err)
	}
	if s.WindowOpened() {
		t.Fatal("退化模式下空快照不得判开窗（否则会对从未开启的窗口持续提交）")
	}
}

// TestWindowSignalReadsNeutralActionField 开窗判定之外，报名动作类型也必须是中立的：
// 快照里的 Action 字段来自适配器映射，调度器不认任何站点的按钮编码。
func TestWindowSignalReadsNeutralActionField(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	for i := range fc.data.Publishes {
		for j := range fc.data.Publishes[i].Classes {
			fc.data.Publishes[i].Classes[j].Action = "enroll"
		}
	}
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	snap := &upstream.ElectivesData{Publishes: []upstream.Publish{{
		Selectable: func() *bool { b := true; return &b }(),
		Classes:    []upstream.Class{{ID: 1, Action: "withdraw"}},
	}}}
	if !s.windowSignalLocked(snap) {
		t.Fatal("selectable=true 应判开窗")
	}
	if snap.Publishes[0].Classes[0].Action != "withdraw" {
		t.Fatal("中立 Action 字段应原样透传")
	}
}
