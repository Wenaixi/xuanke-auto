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
func (f *fakeStore) UpdateIDToken(acct, idToken string) error  { return nil }

// fakeClient 可编程 mock：控制课程数据与报名结果。
type fakeClient struct {
	mu          sync.Mutex
	data        *zhidao.ElectivesData
	err         error
	selectErr   map[int]error
	selectCalls map[int]int
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

func (f *fakeClient) ClassDetail(classID int) (*zhidao.ClassDetail, error) {
	return &zhidao.ClassDetail{ID: classID, CourseName: "健美操"}, nil
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
	c        *fakeClient
	relogErr error  // 重登错误（可编程）
	relog    func() // 重登钩子（可编程，记录是否被调用）
}

func (f *fakeAccts) ClientFor(acct string) (Client, bool) { return f.c, true }
func (f *fakeAccts) AnyClient() (Client, bool)            { return f.c, true }
func (f *fakeAccts) AnyClientWithAccount() (string, Client, bool) {
	return "acct1", f.c, true
}
func (f *fakeAccts) Relogin(acct string) (bool, error) {
	if f.relogErr != nil {
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
	relogStart := make(chan bool)     // 重登开始信号（在标记失效后触发）
	relogDone := make(chan bool)      // 重登完成信号
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
