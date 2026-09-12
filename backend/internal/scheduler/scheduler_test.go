package scheduler

import (
	"errors"
	"sync"
	"testing"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// fakeStore 内存日志存储。
type fakeStore struct {
	mu  sync.Mutex
	log []string
}

func (f *fakeStore) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, result)
	return nil
}

func (f *fakeStore) SaveSuccess(acct string, classID int) error { return nil }
func (f *fakeStore) UpdateIDToken(acct, idToken string) error    { return nil }

// fakeClient 可编程 mock：控制课程数据与报名结果。
type fakeClient struct {
	mu          sync.Mutex
	data        *zhidao.ElectivesData
	err         error
	selectErr   map[int]error
	selectCalls map[int]int
	relogCalls  int // 重登回调调用次数（测试用）
}

func (f *fakeClient) setOpen(open bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.data.Publishes {
		f.data.Publishes[i].InDateRange = open
	}
}

func (f *fakeClient) FindElectives() (*zhidao.ElectivesData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	// 深拷贝后返回：调用方在锁外遍历 Publishes，避免与 setAllOpened 并发写 InDateRange 触发数据竞争
	cp := *f.data
	cp.Publishes = make([]zhidao.Publish, len(f.data.Publishes))
	for i := range f.data.Publishes {
		p := f.data.Publishes[i]
		p.Classes = append([]zhidao.Class(nil), f.data.Publishes[i].Classes...)
		cp.Publishes[i] = p
	}
	return &cp, nil
}

func (f *fakeClient) SelectClass(classID int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selectCalls[classID]++
	if err, ok := f.selectErr[classID]; ok && err != nil {
		return "", err
	}
	return "报名成功", nil
}

func (f *fakeClient) Token() string { return "new-token-999" }

func (f *fakeClient) IsClassFull(classID int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.data.Publishes {
		for _, c := range p.Classes {
			if c.ID == classID {
				return c.MaxCount > 0 && c.SelectedCount >= c.MaxCount, nil
			}
		}
	}
	return false, nil
}

func newFakeClient(open bool) *fakeClient {
	return &fakeClient{
		data: &zhidao.ElectivesData{
			Publishes: []zhidao.Publish{
				{PublishID: 1, PublishName: "高二年体育", InDateRange: open, Classes: []zhidao.Class{
					{ID: 61115, CourseName: "健美操", SelectedCount: 0, MaxCount: 36},
				}},
				{PublishID: 2, PublishName: "高二年校本1", InDateRange: open, Classes: []zhidao.Class{
					{ID: 61205, CourseName: "篮球", SelectedCount: 0, MaxCount: 29},
				}},
				{PublishID: 3, PublishName: "高二年校本2", InDateRange: open, Classes: []zhidao.Class{
					{ID: 61276, CourseName: "健身瑜伽", SelectedCount: 0, MaxCount: 29},
				}},
			},
		},
		selectErr:   map[int]error{},
		selectCalls: map[int]int{},
	}
}

// fakeAccts 伪账号注册表：所有账号共享一个 fakeClient（测试用）。
type fakeAccts struct {
	c             *fakeClient
	relogErr      error  // 重登错误（可编程）
	relog         func() // 重登钩子（可编程，记录是否被调用）
	relogBlocking bool   // 重登失败时钩子先阻塞一次（让测试断言"重登中"状态）

	mu      sync.Mutex   // 保护 removed（测试并发读写）
	removed map[string]bool // 已删除账号（ClientFor 返回不存在）
}

func (f *fakeAccts) ClientFor(acct string) (Client, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removed != nil && f.removed[acct] {
		return nil, false
	}
	return f.c, true
}
func (f *fakeAccts) AnyClient() (Client, bool) { return f.c, true }
func (f *fakeAccts) AnyClientWithAccount() (string, Client, bool) {
	return "acct1", f.c, true
}
func (f *fakeAccts) Relogin(acct string) (bool, error) {
	f.c.mu.Lock()
	f.c.relogCalls++
	f.c.mu.Unlock()
	if f.relogErr != nil {
		// 重登失败路径：钩子先阻塞一次（让测试断言"重登中"），随后返回错误
		if f.relog != nil && f.relogBlocking {
			f.relog()
		}
		return false, f.relogErr
	}
	if f.relog != nil {
		f.relog()
	}
	return true, nil
}

// setAllOpened 打开所有发布的选课窗口。
func setAllOpened(fc *fakeClient) {
	fc.setOpen(true)
}

// resetProbe 手动复位探测节流计时，跳过 30 秒等待以测试窗口打开后的立即提交。
func (s *Scheduler) resetProbe() {
	s.mu.Lock()
	s.lastProbe = time.Time{}
	s.mu.Unlock()
}

// resetReloginAtForTest 清空指定账号的重登节流、失败计数与进行中标记（测试专用）。
// 锁序与 maybeRelogin 决策段一致（reloginMu 外层 + s.mu 内层），避免测试与调度器并发死锁。
func (s *Scheduler) resetReloginAtForTest(acct string) {
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloginAt[acct] = time.Time{}
	delete(s.reloginFail, acct)
	delete(s.relogging, acct)
}

func targets() []Target {
	return []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
		{PublishID: 2, ClassID: 61205, CourseName: "篮球", Priority: 0},
		{PublishID: 3, ClassID: 61276, CourseName: "健身瑜伽", Priority: 0},
	}
}

// waitStatusAcct 轮询指定账号的状态直至课程达到期望状态。
func waitStatusAcct(t *testing.T, s *Scheduler, acct string, classID int, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, c := range s.StateForAccount(acct).Courses {
			if c.ClassID == classID && c.Status == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	st := s.StateForAccount(acct)
	for _, c := range st.Courses {
		if c.ClassID == classID {
			t.Fatalf("课程 %d 状态 %q，期望 %q（结果 %q）", classID, c.Status, want, c.Result)
		}
	}
	t.Fatalf("课程 %d 不在账号 %s 目标中", classID, acct)
}

func TestWindowOpenRetriesWithoutWaitingProbe(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("connection reset") // 第一次提交失败（网络类）
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	// 清除错误：下一次 1 秒重试应成功（不需要等待 30s 探测闸门）
	fc.mu.Lock()
	delete(fc.selectErr, 61115)
	fc.mu.Unlock()
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
}

func TestStateMachine(t *testing.T) {
	fc := newFakeClient(false)
	openTime := time.Now().Add(time.Hour)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, openTime, 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", targets())
	s.Start()
	defer s.Stop()

	// 窗口未开：状态 pending
	time.Sleep(50 * time.Millisecond)
	for _, c := range s.StateForAccount("acct1").Courses {
		if c.Status != "pending" {
			t.Fatalf("窗口未开时课程 %d 状态应为 pending，实际 %q", c.ClassID, c.Status)
		}
	}
	if s.StateForAccount("acct1").WindowOpened {
		t.Fatal("窗口应未开放")
	}

	// 窗口开启：应自动提交并 success（探测节流 30s，手动复位 lastProbe 触发立即探测）
	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
	waitStatusAcct(t, s, "acct1", 61205, "success", 3*time.Second)
	waitStatusAcct(t, s, "acct1", 61276, "success", 3*time.Second)

	fc.mu.Lock()
	calls := map[int]int{}
	for k, v := range fc.selectCalls {
		calls[k] = v
	}
	fc.mu.Unlock()
	for _, id := range []int{61115, 61205, 61276} {
		if calls[id] != 1 {
			t.Fatalf("课程 %d 应恰好提交 1 次，实际 %d", id, calls[id])
		}
	}
}

func TestTargetsByAccountIsolation(t *testing.T) {
	fc := newFakeClient(false)
	openTime := time.Now().Add(time.Hour)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, openTime, 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}})
	s.SetTargetsForAccount("acct2", []Target{{PublishID: 2, ClassID: 61205, CourseName: "篮球"}})
	s.Start()
	defer s.Stop()

	st1 := s.StateForAccount("acct1")
	st2 := s.StateForAccount("acct2")
	if len(st1.Courses) != 1 || st1.Courses[0].ClassID != 61115 {
		t.Fatalf("acct1 状态异常: %+v", st1)
	}
	if len(st2.Courses) != 1 || st2.Courses[0].ClassID != 61205 {
		t.Fatalf("acct2 状态异常: %+v", st2)
	}

	// 窗口开启后两账号目标都应被提交
	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
	waitStatusAcct(t, s, "acct2", 61205, "success", 3*time.Second)
}

func TestSameClassParallelAcrossAccounts(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	// 两账号选中同一门课程——各自独立提交，互不阻塞
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}})
	s.SetTargetsForAccount("acct2", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
	waitStatusAcct(t, s, "acct2", 61115, "success", 3*time.Second)

	fc.mu.Lock()
	calls := fc.selectCalls[61115]
	fc.mu.Unlock()
	if calls != 2 {
		t.Fatalf("同课程两账号应提交 2 次（各自独立），实际 %d", calls)
	}
}

func TestSubmitFailureRetries(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("网络中断")
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
}

func TestRestoreDoneSkipsResubmit(t *testing.T) {
	fc := newFakeClient(true)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	// 重启恢复：注入 acct1 已成功的课程 id
	s.RestoreDone(map[string][]int{"acct1": {61115}})
	s.SetTargetsForAccount("acct1", targets())
	s.Start()
	defer s.Stop()

	time.Sleep(100 * time.Millisecond)
	fc.mu.Lock()
	calls61115 := fc.selectCalls[61115]
	calls61205 := fc.selectCalls[61205]
	fc.mu.Unlock()

	if calls61115 != 0 {
		t.Fatalf("已成功课程不应重复提交，实际 %d 次", calls61115)
	}
	// 其余两门应提交
	if calls61205 == 0 {
		t.Fatal("未完成课程应提交")
	}
	// 状态显示 success
	waitStatusAcct(t, s, "acct1", 61115, "success", 1*time.Second)
}

// TestProbeNowUnauthorizedTriggersRelogin ProbeNow 命中 token 失效（用户刷新课程页场景）
// 也应立即触发该账号自动重登（而非等调度器下个 30s 周期）。
func TestProbeNowUnauthorizedTriggersRelogin(t *testing.T) {
	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized // ProbeNow 探测返回失效
	relogStart := make(chan bool)   // 重登开始信号
	relogDone := make(chan bool)    // 重登完成信号（阻塞重登，让测试断言已触发）
	fa := &fakeAccts{c: fc, relog: func() {
		relogStart <- true
		<-relogDone
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), time.Hour) // 调度器不轮询（interval=1h）

	// 不 Start 调度器：仅调用 ProbeNow，证明它自己就会触发重登
	_, err := s.ProbeNow()
	if err == nil {
		t.Fatal("失效时 ProbeNow 应返回 ErrUnauthorized")
	}
	// 重登应已被触发（ProbeNow 内部同步调用 maybeRelogin → 异步 goroutine）
	select {
	case <-relogStart:
	case <-time.After(2 * time.Second):
		t.Fatal("ProbeNow 命中失效后应触发自动重登")
	}
	close(relogDone) // 放行重登完成
	time.Sleep(100 * time.Millisecond)
	if !s.TokenValidFor("acct1") {
		t.Fatal("重登成功后 token 应显示有效")
	}
}

// TestElectivesSnapshot 快照命中与过期后刷新。
func TestElectivesSnapshot(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)

	// 未探测：命中失败
	if _, ok := s.ElectivesSnapshot(); ok {
		t.Fatal("未探测时快照应不可用")
	}
	// ProbeNow 填充快照
	data, err := s.ProbeNow()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 3 {
		t.Fatalf("快照数据异常: %+v", data)
	}
	if _, ok := s.ElectivesSnapshot(); !ok {
		t.Fatal("探测后快照应命中")
	}
	// 无账号时应报错（clients 为 nil）
	s2 := New(nil, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	if _, err := s2.ProbeNow(); err == nil {
		t.Fatal("无账号时应报错")
	}
}

func TestFormatOpenTime(t *testing.T) {
	tt, err := FormatOpenTime("2026-09-13 09:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if tt.Year() != 2026 || tt.Month() != 9 || tt.Day() != 13 || tt.Hour() != 9 {
		t.Fatalf("解析错误: %v", tt)
	}
}

// TestBackupFallbackOnFull 同发布多备选：第一备选人数满员（快照对比 selected>=max）→ 自动退避第二备选并成功。
func TestBackupFallbackOnFull(t *testing.T) {
	fc := newFakeClient(false)
	// 第一备选健美操已满 36/36；第二备选篮球空
	fc.mu.Lock()
	fc.data.Publishes[0].Classes = []zhidao.Class{
		{ID: 61115, CourseName: "健美操", SelectedCount: 36, MaxCount: 36},
		{ID: 61205, CourseName: "篮球", SelectedCount: 0, MaxCount: 36},
	}
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
		{PublishID: 1, ClassID: 61205, CourseName: "篮球", Priority: 1},
	})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	waitStatusAcct(t, s, "acct1", 61205, "success", 3*time.Second)
}

// TestBackupNotAdvancedOnNetworkError 非满员错误（网络中断）不得切换备选——只有人数确认满员才退避。
func TestBackupNotAdvancedOnNetworkError(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("connection reset")
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
		{PublishID: 1, ClassID: 61205, CourseName: "篮球", Priority: 1},
	})
	s.Start()
	defer s.Stop()

	setAllOpened(fc)
	s.resetProbe()
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	// 备选不应被提交（未确认满员）
	time.Sleep(150 * time.Millisecond)
	fc.mu.Lock()
	calls := fc.selectCalls[61205]
	fc.mu.Unlock()
	if calls != 0 {
		t.Fatalf("未确认满员时不应切备选，备选被提交 %d 次", calls)
	}
}

// TestWindowOpenedWithEmptyPublishes 窗口到点瞬间探测返回空 Publishes（平台拉空学期数据）
// 不得浪费黄金期：提交循环仍应启动并尝试报名。
func TestWindowOpenedWithEmptyPublishes(t *testing.T) {
	fc := newFakeClient(false)
	// 窗口已到点（openTime 设在过去），但探测返回空 Publishes（平台熔断/学期异常被拉空）
	fc.mu.Lock()
	fc.data.Publishes = nil
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Minute), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 即使 Publishes 为空（WindowOpened 无法确认为 true），本地时间已过开窗点也应尝试提交
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
	fc.mu.Lock()
	calls := fc.selectCalls[61115]
	fc.mu.Unlock()
	if calls == 0 {
		t.Fatal("窗口到点空列表时也应尝试提交（黄金期不容浪费）")
	}
}

// TestDeletedAccountStopsSubmitting 账号被管理员删除后，调度器不再为其生成提交链：
// 残留内存目标不得继续真实报名（删账号 = 彻底隔离）。
func TestDeletedAccountStopsSubmitting(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开，探测正常
	s := New(&fakeAccts{c: fc, removed: map[string]bool{}}, &fakeStore{}, time.Now().Add(-time.Minute), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 账号未删：正常提交成功
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
	fc.mu.Lock()
	calls := fc.selectCalls[61115]
	fc.mu.Unlock()
	if calls == 0 {
		t.Fatal("账号存在时应正常提交")
	}

	// 管理员删除账号：ClientFor 返回不存在 → 调度器跳过该账号目标
	fa := s.clients.(*fakeAccts)
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()
	fc.mu.Lock()
	callsBefore := fc.selectCalls[61115]
	fc.mu.Unlock()
	time.Sleep(300 * time.Millisecond) // 等待若干 tick
	fc.mu.Lock()
	callsAfter := fc.selectCalls[61115]
	fc.mu.Unlock()
	if callsAfter != callsBefore {
		t.Fatalf("删除后不应再提交：删除前 %d 次，删除后 %d 次", callsBefore, callsAfter)
	}
}

// TestProbeIntervalFor 分阶段探测间隔：平日 30s、临门与已到点 5s 收紧。
func TestProbeIntervalFor(t *testing.T) {
	open := time.Date(2026, 9, 13, 9, 0, 0, 0, time.Local)
	s := New(&fakeAccts{c: &fakeClient{}}, &fakeStore{}, open, time.Second)

	// 平日：距开放 >5 分钟 → 30 秒
	far := open.Add(-6 * time.Minute)
	if got := s.probeIntervalFor(far); got != probeIntervalFar {
		t.Fatalf("平日应 30s，实际 %v", got)
	}
	// 临门：距开放 4 分钟 → 5 秒
	near := open.Add(-4 * time.Minute)
	if got := s.probeIntervalFor(near); got != probeIntervalNear {
		t.Fatalf("临门应 5s，实际 %v", got)
	}
	// 已到点：开放后 1 分钟 → 5 秒盯守
	passed := open.Add(time.Minute)
	if got := s.probeIntervalFor(passed); got != probeIntervalNear {
		t.Fatalf("已到点应 5s，实际 %v", got)
	}
}

// TestTokenInvalidTriggersRelogin 探测命中 ErrUnauthorized → 标记失效 → 自动重登 → 恢复有效。
func TestTokenInvalidTriggersRelogin(t *testing.T) {
	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized // 探测返回失效
	relogStart := make(chan bool)   // 重登开始信号（在标记失效后触发）
	relogDone := make(chan bool)    // 重登完成信号
	fa := &fakeAccts{c: fc, relog: func() {
		relogStart <- true // 通知已进入重登（此时 token 已标记失效但尚未恢复）
		<-relogDone        // 阻塞重登完成，让测试断言"重登中"状态
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 等探测触发重登（进入重登回调）
	select {
	case <-relogStart:
	case <-time.After(2 * time.Second):
		t.Fatal("探测命中失效后应触发自动重登")
	}
	// 重登进行中：token 应显示无效
	if s.TokenValidFor("acct1") {
		t.Fatal("重登进行中 token 应显示无效")
	}
	// 放行重登完成
	close(relogDone)
	// 等重登完成后恢复有效
	time.Sleep(200 * time.Millisecond)
	if !s.TokenValidFor("acct1") {
		t.Fatal("重登成功后 token 应显示有效")
	}
}

// TestReloginFailureRecoversNextCycle 重登失败不卡死：失效标记保持（UI 不再误报"有效"），
// 下个 30s 节流窗口过后仍可再触发重登。
func TestReloginFailureRecoversNextCycle(t *testing.T) {
	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized // 探测命中失效
	relogStart := make(chan bool, 10)
	relogDone := make(chan bool)
	fa := &fakeAccts{c: fc, relogErr: errors.New("验证码识别失败"), relogBlocking: true, relog: func() {
		relogStart <- true
		<-relogDone // 阻塞重登回调，让重登 goroutine 进入"失败"路径之前测试先断言
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()
	select {
	case <-relogStart:
	case <-time.After(2 * time.Second):
		t.Fatal("探测命中失效后应触发重登")
	}
	// 重登中：token 显示失效
	if s.TokenValidFor("acct1") {
		t.Fatal("重登进行中 token 应显示失效")
	}
	// 放行：重登返回失败 → 失效标记保持（安全审计 MINOR 7：不得误报"有效"）
	close(relogDone)
	time.Sleep(200 * time.Millisecond)
	if s.TokenValidFor("acct1") {
		t.Fatal("重登失败后失效标记应保持（UI 显示『已失效·自动恢复中』，不误报有效）")
	}
	// 下个 30s 节流窗口后可再触发重登（探测仍命中失效；第二次钩子已非阻塞，重登快速返回失败）
	resetReloginDone := make(chan bool)
	fa.relog = func() {
		relogStart <- true
		<-resetReloginDone // 非阻塞：第二次重登快速通过
	}
	s.resetReloginAtForTest("acct1")
	s.resetProbe() // 探测节流也复位，让探测立即再次发起、再次命中失效
	time.Sleep(300 * time.Millisecond)
	// 第二次重登应触发（relogCalls 累计到 2，说明失败后确实还能再试）
	fc.mu.Lock()
	relogCalls := fc.relogCalls
	fc.mu.Unlock()
	if relogCalls < 2 {
		t.Fatal("30s 节流窗口过后应再次触发重登")
	}
	close(resetReloginDone) // 收尾：若第二次重登仍在阻塞则放行，避免 Scheduler.Stop 泄漏 goroutine
}

// TestSubmitUnauthorizedTriggersRelogin 提交链命中 token 失效（非探测路径）也触发自动重登：
// 探测只走 order[0] 账号，其他账号的 token 失效靠报名提交命中 ErrUnauthorized 感知——
// 这是 M1（非探测账号失效无感知）的专项回归测试。
func TestSubmitUnauthorizedTriggersRelogin(t *testing.T) {
	fc := newFakeClient(true)                    // 窗口已开，探测正常
	fc.selectErr[61115] = zhidao.ErrUnauthorized // 报名返回失效
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 探测正常但提交命中失效 → 状态置 failed（教务令牌失效，自动重登中）
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	time.Sleep(100 * time.Millisecond)
	fc.mu.Lock()
	relogCalls := fc.relogCalls
	fc.mu.Unlock()
	if relogCalls < 1 {
		t.Fatal("提交链命中 token 失效后应触发自动重登")
	}
}

// TestWindowOpenSubmitsWithoutProbeReset 窗口开启后提交不依赖探测节流复位：
// lastProbe 保持较新（30s 未到）时，提交重试仍每 1 秒进行——证明提交与探测节流解耦。
func TestWindowOpenSubmitsWithoutProbeReset(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("connection reset") // 提交失败（网络类，会走实时人数复核路径）
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 等首次探测完成（窗口未开，状态 pending，探测正常跑过一次）
	waitStatusAcct(t, s, "acct1", 61115, "pending", 2*time.Second)
	// 窗口开启：此时 lastProbe 仍是最新（未 resetProbe）——探测被 30s 节流挡住，但提交必须每 1 秒重试
	setAllOpened(fc)
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)

	// 清除错误：下一次 1 秒重试应成功（全程不 resetProbe，纯粹靠提交闸门）
	fc.mu.Lock()
	delete(fc.selectErr, 61115)
	fc.mu.Unlock()
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)
}


// TestRateLimitBackoff 平台返回“操作频繁”或 429 类风控文案时，自动退避 30s，后续轮次跳过该课程。
func TestRateLimitBackoff(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开
	fc.selectErr[61115] = errors.New("操作过于频繁，请稍后重试")
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
		{PublishID: 1, ClassID: 61205, CourseName: "篮球", Priority: 1},
	})
	s.Start()
	defer s.Stop()

	// 第一门课因风控失败
	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)

	// 验证退避生效：在退避期内该课不应被反复提交轰炸
	fc.mu.Lock()
	callsBefore := fc.selectCalls[61115]
	fc.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	fc.mu.Lock()
	callsAfter := fc.selectCalls[61115]
	fc.mu.Unlock()

	if callsAfter > callsBefore {
		t.Fatalf("处于风控退避期的课程不应被重复提交: 之前 %d 次, 之后 %d 次", callsBefore, callsAfter)
	}
}

// TestSubmitIntervalSprint 验证开窗后前 10 秒提交间隔收紧至 250ms，10 秒后恢复 1s。
func TestSubmitIntervalSprint(t *testing.T) {
	s := New(nil, &fakeStore{}, time.Now(), time.Second)
	open := time.Now().Add(-2 * time.Second) // 开窗 2 秒内（处于 10s 冲刺期）
	if d := s.submitIntervalFor(time.Now(), open); d != 250*time.Millisecond {
		t.Fatalf("黄金期提交间隔应为 250ms, 实际: %v", d)
	}
	openOld := time.Now().Add(-15 * time.Second) // 开窗已过 15 秒（常规期）
	if d := s.submitIntervalFor(time.Now(), openOld); d != time.Second {
		t.Fatalf("常规期提交间隔应为 1s, 实际: %v", d)
	}
}

// TestServerClockAlignment 验证服务端时钟偏移校准生效。
func TestServerClockAlignment(t *testing.T) {
	s := New(nil, &fakeStore{}, time.Now(), time.Second)
	s.SetClockOffsetForTest(5 * time.Second)
	aligned := s.nowAligned()
	if diff := aligned.Sub(time.Now()); diff < 4*time.Second || diff > 6*time.Second {
		t.Fatalf("校准后时间应快约 5 秒, 实际差值: %v", diff)
	}
}
