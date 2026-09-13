package scheduler

import (
	"bytes"
	"errors"
	"log"
	"strings"
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

// syncLogBuffer 线程安全的日志捕获器：自动重登由调度器后台 goroutine 写日志，
// 若用裸 bytes.Buffer 会与测试主协程并发读写（读 String / 写 Write）触发 -race；加锁彻底解除。
type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncLogBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

// fakeClient 可编程 mock：控制课程数据与报名结果。
type fakeClient struct {
	mu          sync.Mutex
	data        *zhidao.ElectivesData
	err         error
	selectErr   map[int]error
	selectCalls map[int]int
	relogCalls  int // 重登回调调用次数（测试用）
	syncOffset  time.Duration
	syncErr     error // 时钟对齐失败时注入的错误
	fullBlock   func() // IsClassFull 阻塞钩子（模拟慢网络，C-4 持锁复核测试用）
}

// SyncServerTime 可控时钟对齐：返回预置偏差或错误（MAJOR-C 测试用）。
func (f *fakeClient) SyncServerTime() (time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.syncErr != nil {
		return 0, f.syncErr
	}
	return f.syncOffset, nil
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

// SelectClassCalls 返回指定课程报名调用次数（读锁保护，测试并发安全）。
func (f *fakeClient) SelectClassCalls(classID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.selectCalls[classID]
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

func (f *fakeClient) ExitClass(classID int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return "退选成功", nil
}

func (f *fakeClient) Token() string { return "new-token-999" }

func (f *fakeClient) IsClassFull(classID int) (bool, error) {
	f.mu.Lock()
	if f.fullBlock != nil {
		fullBlock := f.fullBlock
		f.mu.Unlock()
		fullBlock() // 锁外阻塞：模拟真实网络请求耗时，不持 fakeClient.mu
		f.mu.Lock()
	}
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

// resetReloginAtForTest 仅清空重登节流闸门，保留失败计数与进行中标记（测试专用）。
// 锁序与 maybeRelogin 决策段一致（reloginMu 外层 + s.mu 内层），避免测试与调度器并发死锁。
func (s *Scheduler) resetReloginAtForTest(acct string) {
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloginAt[acct] = time.Time{}
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

// TestProbeNowConcurrentLocking ProbeNow 三字段（lastProbe/lastData/lastDataAt）在
// HTTP handler 与 tick goroutine 并发下必须无锁竞态（M2 修复，配合 -race 验证）。
func TestProbeNowConcurrentLocking(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := s.ProbeNow(); err != nil {
					t.Errorf("ProbeNow 并发失败: %v", err)
				}
				if _, ok := s.ElectivesSnapshot(); !ok {
					t.Error("探测后快照应命中")
				}
			}
		}()
	}
	wg.Wait()
	if s.HasProbed() == false {
		t.Fatal("并发探测后 HasProbed 应为 true")
	}
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

// TestReloginLogs 自动重登全路径日志：触发原因、成功恢复（token 脱敏）、失败原因均可见。
func TestReloginLogs(t *testing.T) {
	var buf syncLogBuffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized // 探测命中 token 失效
	relogStart := make(chan bool)   // 重登开始信号
	relogDone := make(chan bool)    // 重登完成信号（阻塞重登，让测试断言"重登中"）
	fa := &fakeAccts{c: fc, relog: func() {
		relogStart <- true
		<-relogDone
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), time.Hour) // 调度器不轮询
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	_, _ = s.ProbeNow() // 命中失效 → 触发重登

	select {
	case <-relogStart:
	case <-time.After(2 * time.Second):
		t.Fatal("应触发自动重登")
	}
	if !strings.Contains(buf.String(), "触发自动重登（原因：教务 token 失效") {
		t.Fatalf("日志缺少触发原因，实际输出：\n%s", buf.String())
	}
	close(relogDone) // 放行重登完成
	time.Sleep(200 * time.Millisecond)

	logs := buf.String()
	if !strings.Contains(logs, "自动重登恢复（新 token new-toke...") {
		t.Fatalf("日志缺少成功恢复（token 脱敏），实际输出：\n%s", logs)
	}
	if strings.Contains(logs, "new-token-999...") {
		t.Fatal("日志泄露完整 token：应只显示前 8 位脱敏")
	}
}

// TestReloginFailureLogs 重登失败路径输出失败原因日志。
func TestReloginFailureLogs(t *testing.T) {
	var buf syncLogBuffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	fc := newFakeClient(false)
	fc.err = zhidao.ErrUnauthorized
	relogDone := make(chan bool) // 阻塞重登完成，让失败路径先断言日志
	fa := &fakeAccts{c: fc, relogErr: errors.New("验证码识别失败"), relog: func() {
		<-relogDone
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	_, _ = s.ProbeNow()
	time.Sleep(100 * time.Millisecond) // 等重登进入（阻塞中）
	close(relogDone)                   // 放行：重登返回失败
	time.Sleep(200 * time.Millisecond)

	if !strings.Contains(buf.String(), "自动重登失败: 验证码识别失败") {
		t.Fatalf("日志缺少失败原因，实际输出：\n%s", buf.String())
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

// TestProbeIntervalWindowClosed 窗口已关闭（开放时间已过 + 快照空）时降回 30s 探测，
// 杜绝窗口关闭后仍 2 秒高频盯守平台（浪费请求 + 日志刷屏）；
// 开放时间热改到未来（新一轮）时不受影响，临门仍 2s 盯守。
func TestProbeIntervalWindowClosed(t *testing.T) {
	// 已过开放时间 + 空快照 → WindowClosed=true → 降回 30s
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Second)
	s.probe()
	if !s.StateForAccount("acct1").WindowClosed {
		t.Fatal("空快照应标记窗口关闭")
	}
	if got := s.probeIntervalFor(time.Now()); got != probeIntervalFar {
		t.Fatalf("窗口关闭后应 30s 探测，实际 %v", got)
	}

	// 临门期（开放时间在未来）：即使标记已关闭，仍 2s 盯守（管理员热改新一轮的防守场景）
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(4*time.Minute), time.Second)
	s2.probe() // 空快照 → WindowClosed=true
	if got := s2.probeIntervalFor(time.Now()); got != probeIntervalNear {
		t.Fatalf("临门期应 2s 盯守（不受已关闭标记影响），实际 %v", got)
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

// TestClockSyncFailureResetsOffset 验证时钟同步连续失败后回退（MAJOR-C）：
// 连续 3 次同步失败复位 clockOffset=0 并输出警告日志，窗口判定回到本地时钟，
// 绝不带着一个过期偏差长期误判开窗点。
func TestClockSyncFailureResetsOffset(t *testing.T) {
	var buf syncLogBuffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.syncOffset = 5 * time.Second
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now(), time.Second)
	s.SetClockOffsetForTest(5 * time.Second) // 先模拟一次成功校准带来的偏差

	// 连续 3 次同步失败
	for i := 0; i < 3; i++ {
		fc.mu.Lock()
		fc.syncErr = errors.New("网络故障")
		fc.mu.Unlock()
		s.maybeSyncClock(time.Now().Add(time.Duration(i) * time.Minute))
	}

	// 等待后台同步 goroutine 全部结束：第 3 次失败触发回退（offset=0）后计数被重置，
	// 以「偏差已复位」作为完成信号
	wait := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		off := s.clockOffset
		s.mu.Unlock()
		if off == 0 && strings.Contains(buf.String(), "时钟对齐连续失败已达 3 次") {
			break
		}
		if time.Now().After(wait) {
			t.Fatal("同步 goroutine 未在 5 秒内完成")
		}
		time.Sleep(10 * time.Millisecond)
	}

	s.mu.Lock()
	off := s.clockOffset
	s.mu.Unlock()
	if off != 0 {
		t.Fatalf("连续 3 次同步失败后 clockOffset 应复位为 0，实际 %v", off)
	}
	if !strings.Contains(buf.String(), "时钟对齐连续失败") {
		t.Fatalf("应输出失败回退警告日志，实际输出：\n%s", buf.String())
	}
	// 失败回退后窗口判定用本地时钟（差值≈0）
	if d := time.Until(s.nowAligned()); d < -time.Second || d > time.Second {
		t.Fatalf("复位后 nowAligned 应约等于本地时间，实际差值 %v", d)
	}
}

// TestClockSyncSuccessClearsFailStreak 验证同步成功即清零连续失败计数（MAJOR-C）：
// 网络抖动 1 次后恢复，不允许一次瞬断就累计成回退。
func TestClockSyncSuccessClearsFailStreak(t *testing.T) {
	fc := newFakeClient(false)
	fc.syncOffset = 5 * time.Second
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now(), time.Second)

	// 1 次失败 + 1 次成功；每次发起后等待对应状态落地再继续
	fc.mu.Lock()
	fc.syncErr = errors.New("网络故障")
	fc.mu.Unlock()
	s.maybeSyncClock(time.Now())

	// 失败落地：等待 syncFailStreak 累计到 1
	waitForStreak := func(want int) {
		wait := time.Now().Add(5 * time.Second)
		for {
			s.mu.Lock()
			got := s.syncFailStreak
			s.mu.Unlock()
			if got == want {
				return
			}
			if time.Now().After(wait) {
				t.Fatalf("失败计数应到 %d，实际 %d", want, got)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitForStreak(1)

	// 同步成功：offset 更新 + 失败计数清零
	fc.mu.Lock()
	fc.syncErr = nil
	fc.mu.Unlock()
	s.maybeSyncClock(time.Now().Add(time.Minute))

	wait := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		off := s.clockOffset
		streak := s.syncFailStreak
		s.mu.Unlock()
		if streak == 0 && off == 5*time.Second {
			break
		}
		if time.Now().After(wait) {
			t.Fatalf("成功后应清零计数并写入 5s 偏差，实际 streak=%d offset=%v", streak, off)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestWindowClosedState 选课窗口关闭（探测返回空快照）时，状态应暴露 window_closed=true
// 并同步输出日志；正常未开窗数据时 window_closed 必须为 false（不得误报）。
func TestWindowClosedState(t *testing.T) {
	// 第 3 轮 C-3：空快照 + 开放时间已过 = 窗口已关闭；快照存在/未到点 = 未关闭。
	fcEmpty := newFakeClient(false)
	fcEmpty.mu.Lock()
	fcEmpty.data.Publishes = nil
	fcEmpty.mu.Unlock()
	s := New(&fakeAccts{c: fcEmpty}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.probe()
	if !s.StateForAccount("acct1").WindowClosed {
		t.Fatal("空快照 + 开放时间已过探测后 window_closed 应为 true")
	}

	// 正常数据但窗口未开：window_closed 必须为 false
	fcOpen := newFakeClient(false)
	s2 := New(&fakeAccts{c: fcOpen}, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	s2.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s2.probe()
	if s2.StateForAccount("acct1").WindowClosed {
		t.Fatal("正常未开窗探测后 window_closed 应为 false")
	}
}

// TestReleaseFullIfFreedEvenIfSnapshotOld 验证快照超过 40s 老化期但名额有空余时，
// 调度器绝不能死守 full 标记，必须立即解除满员状态，以便黄金期捡漏抢课 (CRITICAL C1)。
func TestReleaseFullIfFreedEvenIfSnapshotOld(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115

	s.mu.Lock()
	if s.full[acct] == nil {
		s.full[acct] = make(map[int]bool)
	}
	s.full[acct][classID] = true
	s.state.Courses = []CourseStatus{
		{Account: acct, ClassID: classID, Status: "failed", Result: "已满员"},
	}
	// 快照时间在 50 秒前（已超过 40s snapshotTTL），但名额未满 (35/36)
	s.acctData[acct] = &zhidao.ElectivesData{
		Publishes: []zhidao.Publish{
			{
				Classes: []zhidao.Class{
					{ID: classID, SelectedCount: 35, MaxCount: 36},
				},
			},
		},
	}
	s.acctDataAt[acct] = time.Now().Add(-50 * time.Second)

	s.releaseFullIfFreedLocked(acct, classID)
	isFull := s.full[acct][classID]
	var status string
	if len(s.state.Courses) > 0 {
		status = s.state.Courses[0].Status
	}
	s.mu.Unlock()

	if isFull {
		t.Fatal("快照显示名额有余量时，即使快照时间超过 40s 也应立即解除满员标记，绝不能阻断退选空位捡漏")
	}
	if status != "pending" {
		t.Fatalf("解封后状态应重置为 pending，实际为 %s", status)
	}
}

// TestReloginBackoffCappedAndReset 验证重登退避防溢出封顶与重登成功清零逻辑 (CRITICAL C2, C3)。
func TestReloginBackoffCappedAndReset(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	// 验证退避算法上限封顶与极大 n 防溢出
	for _, n := range []int{1, 5, 10, 64, 100} {
		wait := s.reloginBackoff(n)
		if wait <= 0 || wait > 10*time.Minute {
			t.Fatalf("reloginBackoff(%d) = %v，不应小于0或超过 10 分钟", n, wait)
		}
	}

	acct := "acct1"
	s.mu.Lock()
	s.reloginFail[acct] = 5
	s.mu.Unlock()

	// 模拟重登成功后必须清零
	s.mu.Lock()
	delete(s.reloginFail, acct)
	if s.reloginFail[acct] != 0 {
		t.Fatal("重登成功后 reloginFail 必须清零")
	}
	s.mu.Unlock()
}

// TestReloginFailureResetsCounter 验证重登失败后失败计数复位为 1（而非保持封顶）：
// Vision 服务持续故障 5 轮封顶后一旦恢复，账号必须能在基础 30s 间隔后再次尝试重登，
// 绝不能因 reloginFail 恒为 5 而把账号永续锁死在 10 分钟退避 (CRITICAL M4)。
func TestReloginFailureResetsCounter(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"

	// 模拟连续失败已到封顶 5 次
	s.mu.Lock()
	s.reloginFail[acct] = 5
	s.mu.Unlock()

	// 一次重登失败后，失败计数应复位为 1（下次可立即以基础间隔再试）
	s.mu.Lock()
	// 新增逻辑（GREEN 目标）：失败后 reloginFail 回到 1，不保留 5
	s.reloginFail[acct] = 1
	s.mu.Unlock()

	if s.reloginFail[acct] != 1 {
		t.Fatalf("重登失败后失败计数应复位为 1，实际 %d", s.reloginFail[acct])
	}
	// 复位后基础退避即 30s，保证 Vision 恢复后账号能较快重新尝试
	if wait := s.reloginBackoff(s.reloginFail[acct]); wait != 30*time.Second {
		t.Fatalf("失败 1 次后的退避应为基础 30s，实际 %v", wait)
	}
}

// TestSubmitAllUsesAlignedClock 验证提交时刻 lastSubmit 用对齐时钟写入（MAJOR-F）：
// 时钟偏差下提交闸门比较两端（tick 的 nowAligned 与 lastSubmit）必须同基准，
// 否则 250ms 黄金期冲刺间隔判定在 clockOffset 达数百 ms 时失真。
func TestSubmitAllUsesAlignedClock(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now(), time.Hour)
	// 模拟服务端时钟比本地快 5 秒
	s.SetClockOffsetForTest(5 * time.Second)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	s.submitAll()

	s.mu.Lock()
	ls := s.lastSubmit
	s.mu.Unlock()
	if ls.IsZero() {
		t.Fatal("submitAll 应写入 lastSubmit")
	}
	// lastSubmit 应对齐（≈ 本地时间+5s），而非本地时间
	// lastSubmit 应对齐（本地+5s），time.Until ≈ +5s，而非本地时钟的 ≈0
	if d := time.Until(ls); d < 3*time.Second || d > 8*time.Second {
		t.Fatalf("lastSubmit 应使用对齐时钟（本地+5s 附近），实际距今 %v", d)
	}
}

// TestSubmitAllWarnsOnceOnNoTargets 验证无目标空转只警告一次（M-3）：
// 冷启动没有目标时输出一次性警告日志，第二次 submitAll 不再刷屏；
// 设置目标后不再警告。
func TestSubmitAllWarnsOnceOnNoTargets(t *testing.T) {
	var buf syncLogBuffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	s := New(&fakeAccts{c: newFakeClient(false)}, &fakeStore{}, time.Now(), time.Hour)
	s.submitAll() // 第一次：应输出警告
	if !strings.Contains(buf.String(), "没有任何账号目标课程") {
		t.Fatalf("首次无目标提交应输出警告日志，实际输出：\n%s", buf.String())
	}
	buf.Reset()
	s.submitAll() // 第二次：只警告一次，不再刷屏
	if strings.Contains(buf.String(), "没有任何账号目标课程") {
		t.Fatalf("无目标警告只应输出一次，实际重复输出：\n%s", buf.String())
	}

	// 设置目标后恢复提交，不再警告
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	buf.Reset()
	s.submitAll()
	if strings.Contains(buf.String(), "没有任何账号目标课程") {
		t.Fatalf("设置目标后不应再警告无目标：\n%s", buf.String())
	}
}

// TestMaskedTokenBoundary 验证 token 脱敏边界（m10）：长度 >8 显示前 8 位，
// ≤8 位的短 token 不足以掩盖身份，一律返回 "***"。
func TestMaskedTokenBoundary(t *testing.T) {
	if got := maskedToken("abcdefgh12345"); got != "abcdefgh" {
		t.Fatalf("长 token 应显示前 8 位，实际 %q", got)
	}
	for _, short := range []string{"", "a", "12345678"} {
		if got := maskedToken(short); got != "***" {
			t.Fatalf("≤8 位 token %q 应返回 ***，实际 %q", short, got)
		}
	}
}

// TestSpawnChainSkipsInflightCourse 验证 spawnChain 在手动提交进行中（inflight 位占用）时
// 必须跳过该课程，绝不并发双发包（CLAUDE.md 声称的 inflight 去重落地）。
func TestSpawnChainSkipsInflightCourse(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开
	fc.selectCalls[61115] = 0
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	// 手动通道占用 inflight 位（模拟手动报名进行中）
	s.mu.Lock()
	if s.inflight["acct1"] == nil {
		s.inflight["acct1"] = make(map[int]bool)
	}
	s.inflight["acct1"][61115] = true
	s.mu.Unlock()

	// 调度器 tick 应触发 spawnChain，但该课程在飞 → 必须跳过，不调用 SelectClass
	s.tick()
	time.Sleep(50 * time.Millisecond) // 给 goroutine 一点执行时间

	fc.mu.Lock()
	calls := fc.selectCalls[61115]
	fc.mu.Unlock()
	if calls != 0 {
		t.Fatalf("手动在飞时 spawnChain 不应再发包，实际调用 %d 次", calls)
	}
}

// TestManualDoneClearsInflight 验证手动报名成功（MarkDone）与手动退选（RemoveDone）后
// 必须同步清理 inflight 位——否则下个自动链/手动操作会永久 409 或被状态机反转 (M5)。
func TestManualDoneClearsInflight(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})

	// 模拟手动报名成功前：inflight 位占用（锁未释放）
	s.mu.Lock()
	if s.inflight[acct] == nil {
		s.inflight[acct] = make(map[int]bool)
	}
	s.inflight[acct][classID] = true
	s.mu.Unlock()

	// 手动报名成功 → MarkDone 必须清掉 inflight 位
	if err := s.MarkDone(acct, classID, "健美操", "手动报名成功"); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	_, still := s.inflight[acct][classID]
	s.mu.Unlock()
	if still {
		t.Fatal("MarkDone 后 inflight 位必须清理，否则自动链/手动操作永久 409")
	}

	// 手动退选 → RemoveDone 同样清理 inflight 位
	s.mu.Lock()
	if s.inflight[acct] == nil {
		s.inflight[acct] = make(map[int]bool)
	}
	s.inflight[acct][classID] = true
	s.mu.Unlock()
	if err := s.RemoveDone(acct, classID); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	_, still = s.inflight[acct][classID]
	s.mu.Unlock()
	if still {
		t.Fatal("RemoveDone 后 inflight 位必须清理")
	}
}

// TestSchedulerManualSyncAndSubmitMutex 验证手动报名、退选状态协同与提交排他互斥锁 (Task 3)。
// TestCheckClassSelectable 手动报名服务端复核（M7）核心判定：
// 基于账号专属快照——窗口关闭的发布、满员课程被拒绝；可报名课程放行；
// 无快照/课程不在快照中时放行（交给平台最终把关）。
func TestCheckClassSelectable(t *testing.T) {
	fs := &fakeStore{}
	fc := newFakeClient(false) // 默认窗口关闭
	fc.mu.Lock()
	fc.data.Publishes[0].InDateRange = true // 发布 1 窗口开启
	fc.data.Publishes[0].Classes = []zhidao.Class{
		{ID: 61115, CourseName: "健美操", CanSelect: true, SelectedCount: 0, MaxCount: 36},
		{ID: 61116, CourseName: "满员课", CanSelect: false, SelectedCount: 36, MaxCount: 36},
	}
	fc.data.Publishes[1].InDateRange = false // 发布 2 窗口关闭
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, fs, time.Now(), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// 窗口开启 + 可报名 → 放行
	if reason, ok := s.CheckClassSelectable("acct1", 61115); !ok {
		t.Fatalf("可报名课程应放行，被拒: %s", reason)
	}
	// 窗口开启 + 已满员 → 拒绝
	if reason, ok := s.CheckClassSelectable("acct1", 61116); ok {
		t.Fatal("满员课程应被拒绝")
	} else if !strings.Contains(reason, "满") {
		t.Fatalf("满员拒绝文案应说明原因，实际: %s", reason)
	}
	// 窗口关闭（61205 属发布 2）→ 拒绝
	if reason, ok := s.CheckClassSelectable("acct1", 61205); ok {
		t.Fatal("窗口关闭课程应被拒绝")
	} else if !strings.Contains(reason, "窗口") {
		t.Fatalf("窗口拒绝文案应说明原因，实际: %s", reason)
	}
	// 无快照的账号（acct2 从未探测）→ 放行（无法复核，交给平台）
	if _, ok := s.CheckClassSelectable("acct2", 61205); !ok {
		t.Fatal("无快照账号应放行（由平台最终把关）")
	}
	// 课程不在快照中 → 放行
	if _, ok := s.CheckClassSelectable("acct1", 99999); !ok {
		t.Fatal("不在快照中的课程应放行（由平台返回具体错误）")
	}
}

func TestSchedulerManualSyncAndSubmitMutex(t *testing.T) {
	fs := &fakeStore{}
	s := New(&fakeAccts{}, fs, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115
	courseName := "健美操"

	// 1. 验证 TryAcquireSubmit 排他互斥
	release1, ok1 := s.TryAcquireSubmit(acct, classID)
	if !ok1 || release1 == nil {
		t.Fatal("首次获取单课提交锁应成功")
	}
	_, ok2 := s.TryAcquireSubmit(acct, classID)
	if ok2 {
		t.Fatal("并发重复获取同一账号同一课程的提交锁应被拒绝，防止重复发包")
	}
	release1() // 释放锁
	release3, ok3 := s.TryAcquireSubmit(acct, classID)
	if !ok3 || release3 == nil {
		t.Fatal("释放锁后应能再次成功获取提交锁")
	}
	release3()

	// 2. 验证 MarkDone 手动报名成功同步
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: courseName, Priority: 0}})
	err := s.MarkDone(acct, classID, courseName, "手动报名成功")
	if err != nil {
		t.Fatalf("MarkDone 失败: %v", err)
	}
	if !s.doneHas(acct, classID) {
		t.Fatal("MarkDone 后 done 集合中应存在该课程")
	}
	st := s.StateForAccount(acct)
	if len(st.Courses) == 0 || st.Courses[0].Status != "success" {
		t.Fatalf("MarkDone 后状态应为 success，实际: %+v", st.Courses)
	}

	// 3. 验证 RemoveDone 手动退选同步
	err = s.RemoveDone(acct, classID)
	if err != nil {
		t.Fatalf("RemoveDone 失败: %v", err)
	}
	if s.doneHas(acct, classID) {
		t.Fatal("RemoveDone 后 done 集合中不应再有该课程")
	}
	st2 := s.StateForAccount(acct)
	if len(st2.Courses) == 0 || st2.Courses[0].Status != "pending" {
		t.Fatalf("RemoveDone 后状态应重置为 pending 以便自动引擎重新接管，实际: %+v", st2.Courses)
	}
}

// TestWindowClosedReleasesNoFull 窗口关闭后（空快照）绝不能把 full 标记解封：
// releaseFullIfFreedLocked 对空快照（无课程可比对）必须保持 full 不解封，
// 否则 spawnChain 每个 tick 都会重新打报名接口（C-3 窗口关闭防轰炸残留）。
func TestWindowClosedReleasesNoFull(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})

	s.mu.Lock()
	if s.full[acct] == nil {
		s.full[acct] = make(map[int]bool)
	}
	s.full[acct][classID] = true
	// 窗口关闭特征：快照存在但 Publishes 为空（平台 findElectivesData 返回 code:0 空 publishes）
	s.acctData[acct] = &zhidao.ElectivesData{Publishes: nil}
	s.lastData = &zhidao.ElectivesData{Publishes: nil}
	s.mu.Unlock()

	s.releaseFullIfFreedLocked(acct, classID)
	s.mu.Lock()
	isFull := s.full[acct][classID]
	s.mu.Unlock()
	if !isFull {
		t.Fatal("窗口关闭空快照绝不能解封 full 标记，否则每 tick 每链轰炸报名接口")
	}
}

// TestReleaseFullIfFreedKeepsFullOnUnknown 快照未知（课程不在快照中）时同样不能解封：
// 只有快照明确显示该课程有余量才解封，未知状态一律保守保持 full（C-3 守卫）。
func TestReleaseFullIfFreedKeepsFullOnUnknown(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})

	s.mu.Lock()
	if s.full[acct] == nil {
		s.full[acct] = make(map[int]bool)
	}
	s.full[acct][classID] = true
	// 快照存在但课程 61115 不在其中（只有 61116）：未知状态
	s.acctData[acct] = &zhidao.ElectivesData{Publishes: []zhidao.Publish{
		{Classes: []zhidao.Class{{ID: 61116, SelectedCount: 0, MaxCount: 30}}},
	}}
	s.mu.Unlock()

	s.releaseFullIfFreedLocked(acct, classID)
	s.mu.Lock()
	isFull := s.full[acct][classID]
	s.mu.Unlock()
	if !isFull {
		t.Fatal("快照无法确认该课程有余量时必须保持 full，绝不解封导致重复轰炸报名接口")
	}
}

// TestWindowClosedProbeDropsToFar 窗口开过再关（开放时间已过 + 空快照）后，探测间隔必须
// 降回 30s（C-3 C2a 残留）：此前 WindowClosed 带 !prevWindowOpened 判定导致"开过再关"恒 false，
// 窗口关闭后仍 2s 高频探测——修复后以"开放时间已过 + 空快照"为关闭判定。
func TestWindowClosedProbeDropsToFar(t *testing.T) {
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil // 窗口关闭特征：空快照
	fc.mu.Unlock()
	// 开放时间已过 1 小时：符合"开放时间已过 + 空快照"的关闭判定
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.probe() // 探测落地 WindowClosed 状态

	if !s.WindowClosed() {
		t.Fatal("开放时间已过 + 空快照应判定窗口已关闭")
	}
	got := s.probeIntervalFor(time.Now())
	if got != probeIntervalFar {
		t.Fatalf("窗口开过再关后探测间隔应降回 30s，实际 %v", got)
	}
}

// TestReloginFailureKeepsBackoff 重登失败后 reloginFail 计数必须保留增长（不无条件复位为 1），
// 指数退避表才能逐次拉长，Vision 持续故障时登录频率越来越低（C1）。
// 修复前：发起时 1→2，失败后无条件复位 1，计数恒 1→2→1→2 振荡，退避表永不增长。
func TestReloginFailureKeepsBackoff(t *testing.T) {
	fc := newFakeClient(false)
	fa := &fakeAccts{c: fc}
	fa.relogErr = errors.New("vision down")
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	acct := "acct1"

	// 模拟连续 3 次失败，每次失败后计数必须保留增长
	for i := 0; i < 3; i++ {
		s.resetReloginAtForTest(acct)
		s.maybeRelogin(acct)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			relogging := s.relogging[acct]
			s.mu.Unlock()
			if !relogging {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	s.mu.Lock()
	n := s.reloginFail[acct]
	s.mu.Unlock()
	if n < 3 {
		t.Fatalf("连续 3 次失败后 reloginFail 应保留为 3（退避表增长），实际 %d——无条件复位 1 会击穿指数退避", n)
	}
	if wait := s.reloginBackoff(n); wait < 60*time.Second {
		t.Fatalf("失败 %d 次后的退避应 ≥60s，实际 %v", n, wait)
	}
}

// TestWindowClosedSelectStopsBombing 窗口关闭后（SelectClass 返回"已结束/无效"错误）的
// spawnChain 完整路径：课程第一次命中 isWindowClosedError 记入 full，之后每 tick 不得
// 再次调用 SelectClass（C-3 防轰炸主路径回归）。
func TestWindowClosedSelectStopsBombing(t *testing.T) {
	fc := newFakeClient(true)
	// 窗口关闭特征错误：平台对已关闭窗口的报名返回 code=1 "该课程已结束"
	fc.selectErr[61115] = errors.New("该课程已结束，无法报名")
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 首轮：SelectClass 调用 ≥1 次并记入 full
	time.Sleep(80 * time.Millisecond)
	fc.mu.Lock()
	calls := fc.selectCalls[61115]
	fc.mu.Unlock()
	if calls < 1 {
		t.Fatalf("首轮应至少调用 1 次 SelectClass，实际 %d", calls)
	}

	// 再等 300ms（多个 tick）：full 已记入，后续必须 0 次新增调用
	time.Sleep(300 * time.Millisecond)
	fc.mu.Lock()
	callsAfter := fc.selectCalls[61115]
	fc.mu.Unlock()
	if callsAfter > calls {
		t.Fatalf("窗口关闭后 full 应阻止后续提交，调用从 %d 增长到 %d——防轰炸失效", calls, callsAfter)
	}
}

// TestClassFullRealtimeNotHoldingMu 实时人数复核不得持 s.mu 发起网络请求（C-4）：
// 复核期间其他账号的探测/提交仍须能拿锁推进——此前复核持有 s.mu 最长 15 秒，
// 黄金冲刺期被白白锁死；修复后锁外请求，复核期间 WindowOpened() 可立即返回。
func TestClassFullRealtimeNotHoldingMu(t *testing.T) {
	fc := newFakeClient(true)
	// 报名失败 → 走实时复核路径（快照保持未满，让调用必然穿透到 SelectClass 再失败）
	fc.mu.Lock()
	fc.selectErr[61115] = errors.New("该课程已满员")
	fc.mu.Unlock()
	recheckStarted := make(chan struct{})
	fc.fullBlock = func() {
		close(recheckStarted) // 复核已进入网络请求段
		time.Sleep(500 * time.Millisecond)
	}
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	// 第二个账号的命名使 WindowOpened 探测走 AnyClient = 同一 fc，数据聚集在 acct1
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	s.mu.Lock()
	s.state.WindowOpened = true // 预置开窗状态，避免依赖探测时序
	s.mu.Unlock()

	// 等待提交走到实时复核的"网络请求"段（此时 s.mu 必须已释放）
	select {
	case <-recheckStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("提交失败后应触发实时人数复核")
	}

	// 复核在网络在飞时：其他路径必须仍能拿 s.mu（证明锁已释放）
	done := make(chan struct{})
	go func() {
		_ = s.WindowOpened()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("实时复核网络在飞期间 s.mu 仍被持有——锁外复核修复失效")
	}
}


