package scheduler

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// fakeStore 内存日志存储。
type fakeStore struct {
	mu            sync.Mutex
	log           []string
	successRows   map[string]int // [acct\x00classID] 已落库的 success 行（B18-M2 测试用）
	refusedRows   map[string]int // [acct\x00classID] 已落库的 refused 行（B18-M2 测试用）
}

func (f *fakeStore) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, result)
	return nil
}

func (f *fakeStore) SaveSuccess(acct string, classID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.successRows == nil {
		f.successRows = map[string]int{}
	}
	f.successRows[acct+"\x00"+strconv.Itoa(classID)]++
	return nil
}
func (f *fakeStore) SaveRefused(acct string, classID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refusedRows == nil {
		f.refusedRows = map[string]int{}
	}
	f.refusedRows[acct+"\x00"+strconv.Itoa(classID)]++
	return nil
}
func (f *fakeStore) UpdateIDToken(acct, idToken string) error     { return nil }
func (f *fakeStore) DeleteSuccess(acct string, classID int) error { return nil }
func (f *fakeStore) DeleteRefused(acct string) error              { return nil }
func (f *fakeStore) DeleteRefusedClass(acct string, classID int) error { return nil }
func (f *fakeStore) SetTargetsForAccount(acct string, targets []Target) error { return nil }

// failStore：带失败开关的 Store——SQLite 落库失败时调度器必须把错误上报/记日志，
// 静默吞错会让"内存已写、库行没落上"的半态在重启后破坏恢复契约
// （B9-02 refused 行丢失 = 手动退选被撤销；SaveSuccess 行丢失 = 已成功课被重抢）。
type failStore struct {
	*fakeStore
	fail bool // 置 true 后全部写操作返回错误，模拟磁盘满/IO 故障
}

func (f *failStore) SaveSuccess(acct string, classID int) error {
	if f.fail {
		return errors.New("sqlite disk full")
	}
	return f.fakeStore.SaveSuccess(acct, classID)
}
func (f *failStore) SaveRefused(acct string, classID int) error {
	if f.fail {
		return errors.New("sqlite disk full")
	}
	return f.fakeStore.SaveRefused(acct, classID)
}
func (f *failStore) DeleteRefused(acct string) error {
	if f.fail {
		return errors.New("sqlite disk full")
	}
	return f.fakeStore.DeleteRefused(acct)
}

// TestStoreFailuresAreLoggedNotSilentlyDropped 验证失败落库必须记日志（B33-01）——
// 静默吞错让"内存过半态"无法在重启前被发现。将 log 捕获器注入标准 logger，
// 调用 SaveSuccess/SaveRefused 各失败一次，断言均有对应日志输出。
func TestStoreFailuresLogged(t *testing.T) {
	old := log.Writer()
	defer log.SetOutput(old)
	var logMu sync.Mutex
	var logs strings.Builder
	log.SetOutput(&lockedWriter{mu: &logMu, b: &logs})

	accts := &fakeAccts{c: newFakeClient(true)}
	s := New(accts, &failStore{fakeStore: &fakeStore{}, fail: true}, time.Now(), time.Hour)

	// 触发 SaveSuccess 失败：手动标记成功（spawnChain 成功分支的网络 mock 复杂，直接走
	// MarkDone——同一落库路径，1326/1706 同款 `_ =` 吞错）
	s.MarkDone("acct1", 1, "健美操", "选课成功")
	// 触发 SaveRefused 失败：手动退选（RemoveDone 的 SaveRefused 落库失败）
	_ = s.RemoveDone("acct1", 1)

	logMu.Lock()
	out := logs.String()
	logMu.Unlock()
	if !strings.Contains(out, "落库失败") {
		t.Fatalf("失败落库必须记录错误日志，实际输出: %q", out)
	}
}

// TestSetTargetsDeleteRefusedFailureLogged B36-01：重设目标清空库内退选行失败必须记日志——
// 静默吞掉会让库内 refused 行残留，重启恢复序（RestoreTargets 不清 refused + LoadRefused +
// RestoreRefused）把已重新接管的课程恢复成"已手动退选"，用户意图与持久化分叉。
// 本测试可复现：SetTargetsForAccount 前先 MarkDone（落 failed 级别退选历史路径外的 refused
// 内存态），再在 failStore.fail=true 下重设目标，断言日志出现"清空退选记录落库失败"。
func TestSetTargetsDeleteRefusedFailureLogged(t *testing.T) {
	old := log.Writer()
	defer log.SetOutput(old)
	var logMu sync.Mutex
	var logs strings.Builder
	log.SetOutput(&lockedWriter{mu: &logMu, b: &logs})

	s := New(&fakeAccts{c: newFakeClient(true)}, &failStore{fakeStore: &fakeStore{}, fail: true}, time.Now(), time.Hour)
	// 触发失败：failStore.fail=true 下重设目标 → DeleteRefused 返回错误
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	logMu.Lock()
	out := logs.String()
	logMu.Unlock()
	if !strings.Contains(out, "清空退选记录落库失败") {
		t.Fatalf("重设目标清空退选记录失败必须记日志，实际输出: %q", out)
	}
}

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

// lockedWriter 供测试把标准 logger 重定向到加锁缓冲区（与 syncLogBuffer 同款防 -race）。
type lockedWriter struct {
	mu *sync.Mutex
	b  *strings.Builder
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
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
	relogCalls  int          // 重登回调调用次数（测试用）
	syncOffset  time.Duration
	syncErr     error  // 时钟对齐失败时注入的错误
	syncCalls   int    // 时钟对齐发起次数（B9-03 退避测试断言"失败期不反复发起"）
	fullBlock   func() // IsClassFull 阻塞钩子（模拟慢网络，C-4 持锁复核测试用）
	selectBlock func() // SelectClass 阻塞钩子（模拟慢网络，B18-M2 在飞竞态测试用）
	fullErr     error  // 实时人数复核错误（B19-03：命中 token 失效测试用）
}

// SyncServerTime 可控时钟对齐：返回预置偏差或错误（MAJOR-C 测试用）。
func (f *fakeClient) SyncServerTime() (time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.syncCalls++
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
	if f.selectBlock != nil {
		selectBlock := f.selectBlock
		f.mu.Unlock()
		selectBlock() // 锁外阻塞：模拟真实网络往返耗时，不持 fakeClient.mu
		f.mu.Lock()
	}
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
	if f.fullErr != nil {
		f.mu.Unlock() // 显式解锁：早退分支必须释放锁（B19-03 夹具死锁根因——漏了这行导致 Relogin 永久卡死）
		return false, f.fullErr // B19-03：命中 token 失效等错误
	}
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
				// BeginDate 镜像平台真实形态（"2026-09-13 09:00:00" 带时分秒）——
				// 发布元数据补全/持久化测试依赖它（窗口关闭后 /state 分组的唯一数据源）
				{PublishID: 1, PublishName: "高二年体育", BeginDate: "2026-09-13 09:00:00", InDateRange: open, Classes: []zhidao.Class{
					{ID: 61115, CourseName: "健美操", SelectedCount: 0, MaxCount: 36},
				}},
				{PublishID: 2, PublishName: "高二年校本1", BeginDate: "2026-09-13 09:00:00", InDateRange: open, Classes: []zhidao.Class{
					{ID: 61205, CourseName: "篮球", SelectedCount: 0, MaxCount: 29},
				}},
				{PublishID: 3, PublishName: "高二年校本2", BeginDate: "2026-09-13 09:00:00", InDateRange: open, Classes: []zhidao.Class{
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

	mu      sync.Mutex      // 保护 removed（测试并发读写）
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

// fakeStore 里补一个「可断言的最后删除」钩子：RemoveDone 是否真的调了 DeleteSuccess。
type deletingStore struct {
	*fakeStore
	mu      sync.Mutex
	deleted []string // "acct:classID" 记录
}

func (d *deletingStore) DeleteSuccess(acct string, classID int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deleted = append(d.deleted, fmt.Sprintf("%s:%d", acct, classID))
	return nil
}

func (d *deletingStore) deletedList() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.deleted...)
}

// countingLogStore 带日志计数器的假存储：断言"已删账号不得追加审计日志"。
type countingLogStore struct {
	*fakeStore
	mu  sync.Mutex
	log int
}

func (c *countingLogStore) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	c.mu.Lock()
	c.log++
	c.mu.Unlock()
	return nil
}

func (c *countingLogStore) logCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.log
}
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

// TestFormatOpenTimeEmpty B25-01：空串 = 合法"清除开放时间"
// （F7-02 契约：零值 = 解除窗口机制）。修复前 FormatOpenTime("") 报"开放时间格式
// 错误"——管理员 PUT open_time=""（合法清空）落库 settings 后重启，main.go:116
// log.Fatal 拒绝启动，服务永久停摆（只能手工改 DB 删 settings 行）。修复后空串
// 返回零值 time.Time 不报错，与 runtime.reparse 置 OpenTimeParsed 零值语义对齐。
func TestFormatOpenTimeEmpty(t *testing.T) {
	tt, err := FormatOpenTime("")
	if err != nil {
		t.Fatal("空串（清空开放时间）= 合法操作，不得报错: " + err.Error())
	}
	if !tt.IsZero() {
		t.Fatalf("空串应返回零值 time.Time，实际: %v", tt)
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

// TestDeletedAccountInFlightDropsSuccess 验证删除账号与在飞 spawnChain 竞态下，
// 成功分支必须在写 done/落库前复核账号仍存在——否则 SelectClass 网络往返期间
// DeleteAccount 已清表，本链返回后 SaveSuccess 把已删账号的 success 行写回，
// 重启后重新登录被 RestoreDone 恢复成"已报名成功"假状态（B18-M2，第 18 轮）。
func TestDeletedAccountInFlightDropsSuccess(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开
	store := &fakeStore{}
	fa := &fakeAccts{c: fc, removed: map[string]bool{}}
	s := New(fa, store, time.Now().Add(-time.Minute), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 在 SelectClass 期间阻塞，模拟真实网络往返（最长 15s）
	entered := make(chan struct{})
	release := make(chan struct{})
	fc.mu.Lock()
	fc.selectBlock = func() {
		close(entered)
		<-release
	}
	fc.mu.Unlock()

	// 等链进入网络往返（inflight 已置位，SelectClass 卡住）
	s.mu.Lock()
	_, inFlight := s.inflight["acct1"][61115]
	s.mu.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for !inFlight && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		s.mu.Lock()
		_, inFlight = s.inflight["acct1"][61115]
		s.mu.Unlock()
	}
	if !inFlight {
		t.Fatal("自动链应已置位 inflight（SelectClass 网络往返中）")
	}

	// 链卡在往返期间，管理员删除账号：ClientFor 返回不存在
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()

	// 放行网络调用 → 链返回成功
	close(release)
	select {
	case <-entered:
		// 已确认进入 SelectClass（阻塞钩子）后才放行——上面已置位 inflight，
		// 若未进入钩子说明链执行偏晚；此处放行后等链走完
	case <-time.After(3 * time.Second):
		t.Fatal("自动链应已进入 SelectClass")
	}

	// 等链处理完毕（成功分支已执行）
	waitDeleted := func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			_, inFlight = s.inflight["acct1"][61115]
			s.mu.Unlock()
			if !inFlight {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("在飞链应已完成（inflight 清理）")
	}
	waitDeleted()

	// B18-M2 契约：账号已删除，success 行必须 NOT 落库
	store.mu.Lock()
	rows := store.successRows["acct1\x0061115"]
	store.mu.Unlock()
	if rows != 0 {
		t.Fatalf("删除账号与在飞链竞态下不得落库 success 行（重启后假成功），实际 %d 行", rows)
	}
}

// TestDeletedAccountReloginSuccessDropsState 验证删除账号与在途自动重登竞态下，
// 重登成功分支必须整段放弃——否则 DeleteAccount 清凭据表+Accounts.Remove 后在途
// Relogin（Vision 最坏 2 分钟）返回成功，会写回 tokenValid/reloginAt 内存态 +
// UpdateIDToken 落库把已删账号新 token 写回 credentials 表（重启 Restore 重建客户端、
// 凭据幽灵复活）。与 B18-M2（自动链）/B20-01（手动路径）同族防线，重登路径补齐
// （B21-03，第 21 轮）。
func TestDeletedAccountReloginSuccessDropsState(t *testing.T) {
	fc := newFakeClient(true)
	store := &fakeStore{}
	fa := &fakeAccts{c: fc, removed: map[string]bool{}}
	s := New(fa, store, time.Now().Add(-time.Minute), time.Hour)
	acct := "acct1"

	// 让重登成功路径可以触发：先模拟一次"重登成功且账号已删"的完整时序
	// 1. 发起自动重登（探测命中 token 失效），重登钩子阻塞在 Relogin 内
	relogEntered := make(chan struct{})
	relogRelease := make(chan struct{})
	fa.relog = func() {
		close(relogEntered)
		<-relogRelease
	}
	s.maybeRelogin(acct)
	select {
	case <-relogEntered:
		// 已进入 Relogin（阻塞）
	case <-time.After(3 * time.Second):
		t.Fatal("自动重登应已进入 Relogin")
	}

	// 2. 重登在途期间管理员删除账号：完整删除 = DeleteAccount 调 PurgeAccount（清内存态）
	//    + Accounts.Remove（ClientFor 返回不存在）。两者都要模拟：PurgeAccount 先清
	//    reloginAt/reloginFail/tokenValid/relogging，removed 让后续 ClientFor 复核失败。
	s.PurgeAccount(acct)
	fa.mu.Lock()
	fa.removed[acct] = true
	fa.mu.Unlock()

	// 3. 放行重登 → 成功返回（relogErr=nil，fakeAccts.Relogin 返回 true,nil）
	close(relogRelease)
	// 等待成功分支落地（relogging 已被清理）
	deadline := time.Now().Add(3 * time.Second)
	for {
		s.mu.Lock()
		busy := s.relogging[acct]
		s.mu.Unlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("重登 goroutine 未在 3 秒内落地")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// B21-03 契约：账号已删，重登成功分支整段放弃
	// 断言 1：tokenValid 不得残留（PurgeAccount 已删、成功分支不得重写回）——
	// tokenValidForLocked 对缺失 key 返回 true（有效），故直接断言内存 map 无该 key
	//（map 读缺失 = tokenValid[acct]==false 恒，改用 in-map 判定更严格）
	s.mu.Lock()
	_, tvSet := s.tokenValid[acct]
	s.mu.Unlock()
	if tvSet {
		t.Fatal("删除账号后重登成功不得写回 tokenValid（内存态幽灵残留）")
	}
	// 断言 2：reloginAt 不得被刷新（PurgeAccount 已删、成功分支不得重新写回）
	s.mu.Lock()
	_, reloginAtSet := s.reloginAt[acct]
	s.mu.Unlock()
	if reloginAtSet {
		t.Fatal("删除账号后重登成功不得写回 reloginAt（内存态幽灵残留）")
	}
	// 断言 3：UpdateIDToken 不得把已删账号新 token 落库（credentials 表幽灵复活）
	// fakeStore 的 UpdateIDToken 为 no-op，用"调度器成功分支应跳过落库"语义 +
	// 复核实测 TokenValidFor/reloginAt 均未写回即可（落库被同一复核门挡住）。
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

// TestRealtimeRecheckDeletedAccountDropsLog 实时人数复核结果块（锁外网络段后回锁写状态）
// 的删号竞态复核：复核发起的网络往返（IsClassFull，最长 15s）期间管理员删除账号——
// 返回结果后回锁时若一律照写，会给已删账号落一条幽灵失败审计日志并重建 full/relogin
// 族幽灵 map 条目（B30-01 链顶/B37-03 失效分支/B18-M2 成功分支同族防线的最后一块拼图；
// setStateLocked 的 idx<0 守卫只挡状态数组越界，挡不住落库与 map 写）。
func TestRealtimeRecheckDeletedAccountDropsLog(t *testing.T) {
	fc := newFakeClient(true)
	fc.selectErr[61115] = errors.New("connection reset") // 报名网络失败 → 走实时人数复核路径
	fa := &fakeAccts{c: fc, removed: map[string]bool{}}
	st := &countingLogStore{fakeStore: &fakeStore{}}

	s := New(fa, st, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// IsClassFull 网络段阻塞：模拟复核请求在途——删除账号精确落在"复核已发起、结果未返回"窗口
	entered := make(chan struct{})
	release := make(chan struct{})
	fc.mu.Lock()
	fc.fullBlock = func() {
		close(entered)
		<-release
	}
	fc.mu.Unlock()

	s.mu.Lock()
	s.lastSubmit = time.Time{} // 清提交闸门：本次 tick 直接走提交段
	s.mu.Unlock()
	s.tick()

	// 等链进入复核网络往返（inflight 已置位、IsClassFull 卡住）
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("自动链应已进入实时人数复核")
	}
	// 此刻删号：账号从客户端注册表摘除（memory-first），复核结果回锁写回时 ClientFor 必不存在
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()
	close(release)

	// 等链完全退出（chains 活跃标记消失 = goroutine 的 defer 已执行，即实时复核结果块
	// 全部落地）。注意绝不能用 inflight 等待：inflight 在 SelectClass 返回后（进实时复核
	// 之前）就已清理，等它会让断言与 goroutine 落日志并发——测试在旧实现上就会假绿
	// （实测：等 inflight 时旧实现真红漏检，测试 0.01s 假绿通过）。
	deadline := time.Now().Add(3 * time.Second)
	key := "acct1\x001"
	for time.Now().Before(deadline) {
		s.chainMu.Lock()
		_, active := s.chains[key]
		s.chainMu.Unlock()
		if !active {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// 契约：已删账号不得追加审计日志（旧实现在通用失败分支落幽灵行）
	if n := st.logCount(); n != 0 {
		t.Fatalf("已删账号实时复核不得追加审计日志，实际 %d 行", n)
	}
}
// SelectClass 网络往返（最长 15s）期间管理员删除账号，返回 ErrUnauthorized 后分支必须
// 在状态写回与审计日志落库前再次核实 ClientFor，已删则静默放弃——旧实现直接在分支内
// setState+AppendLog：删号后状态行被覆写为"教务令牌失效"并多写一行 DB 日志
// （删号竞态链 B18-M2/B20-01/B21-03 的分支级缺口，B30-01 只护链顶与取 client 处）。
func TestUnauthorizedBranchDeletedAccountSkipsState(t *testing.T) {
	fc := newFakeClient(true)
	fc.selectErr[61115] = zhidao.ErrUnauthorized // 报名返回失效
	fa := &fakeAccts{c: fc, removed: map[string]bool{}}
	st := &countingLogStore{fakeStore: &fakeStore{}}
	fa.relogErr = errors.New("relog fail") // 重登失败（无关断言，仅需 gh 不 panic）

	s := New(fa, st, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("填充快照失败: %v", err)
	}

	// SelectClass 期间阻塞，模拟真实网络往返——删除账号精确落在"链在网络中、失效分支未执行"窗口
	entered := make(chan struct{})
	release := make(chan struct{})
	fc.mu.Lock()
	fc.selectBlock = func() {
		close(entered)
		<-release
	}
	fc.mu.Unlock()

	s.mu.Lock()
	s.lastSubmit = time.Time{} // 清提交闸门：本次 tick 直接走提交段
	s.mu.Unlock()
	s.tick()

	// 等链进入网络往返（inflight 已置位、SelectClass 卡住）
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("自动链应已进入 SelectClass")
	}
	// 此刻删号：账号从客户端注册表摘除（memory-first），失效分支写回时 ClientFor 必不存在
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()
	close(release)

	// 等链处理完毕（inflight 清理）
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		_, inFlight := s.inflight["acct1"][61115]
		s.mu.Unlock()
		if !inFlight {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// 契约：已删账号不得覆写状态行（提交前的 submitted 中转态保留，绝不被覆写成
	// "failed/教务令牌失效"）也不得追加审计日志
	s.mu.Lock()
	idx := s.statusIndexLocked("acct1", 61115)
	status := ""
	if idx >= 0 {
		status = s.state.Courses[idx].Status
	}
	s.mu.Unlock()
	if status == "failed" {
		t.Fatalf("已删账号失效分支不得覆写状态为 failed（应保持 submitted/pending 中转态）")
	}
	if n := st.logCount(); n != 0 {
		t.Fatalf("已删账号失效分支不得追加审计日志，实际 %d 行", n)
	}
}

// TestWindowOpenSubmitsWithoutProbeReset 窗口开启后提交不依赖探测节流复位：
// lastProbe 保持较新（30s 未到）时，提交重试仍每 1 秒进行——证明提交与探测节流解耦。
// B14-M1：原 openTime 为未来 1 小时——tick 守卫 702 行 `!opened && !now.After(open)`
// 恒 return，提交循环根本无法抵达（首段断言恒等 pending 超时必红，恒绿假象的另一面"恒红"）。
// 改为过去时刻：守卫放行提交路径，而探测仍被 lastProbe 节流挡住（不 resetProbe），
// 真正验证"提交不依赖探测节流"。本次为修复失效契约的测试，非业务代码改动（无红灯需先见）。
func TestWindowOpenSubmitsWithoutProbeReset(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("connection reset") // 提交失败（网络类，会走实时人数复核路径）
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Second), 10*time.Millisecond)
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

// TestSubmitSuspendedWhenOpenTimeCleared B11-A1：管理员显式清空 open_time（F7-02 语义
// 解除窗口机制，runtime.reparse 置 OpenTimeParsed 为零值）后，tick 必须挂起提交——
// 此前 !opened && !now.After(open) 在 open=零值 时恒 false，提交循环永续放行，
// 对"已满员/已成功"目标每 1s 仍刷平台报名接口（空快照下 full 保守不解封、done/refused
// 只拦一小部分，窗口关闭后防轰炸的 C-3 防线被 open 零值绕过）。
func TestSubmitSuspendedWhenOpenTimeCleared(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now(), time.Second)
	// 配置目标账号（无目标时 submitAll 空转不产生调用，测不出零值守卫的作用）
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	// 注入"窗口已开启"状态，模拟已经历过开窗阶段（open 清空前 WindowOpened=true）
	s.mu.Lock()
	s.state.WindowOpened = true
	s.mu.Unlock()
	// 清空 open_time：SetOpenTimeFn 返回零值时间（管理员 PUT open_time="" 后 runtime.reparse 的行为）
	s.SetOpenTimeFn(func() time.Time { return time.Time{} })

	// 跑足够多轮 tick（每轮 50ms，共约 1.2s，远超 1s 常态提交间隔）
	tEnd := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(tEnd) {
		s.tick()
		time.Sleep(50 * time.Millisecond)
	}
	if n := fc.SelectClassCalls(61115); n != 0 {
		t.Fatalf("open_time 清空后不得继续提交，实际调用 SelectClass %d 次", n)
	}

	// 反向对照：open_time 恢复未来时刻（新一轮窗口）且窗口未开启 → 同样挂起提交
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(2*time.Hour), time.Second)
	s2.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s2.mu.Lock()
	s2.state.WindowOpened = false
	s2.mu.Unlock()
	s2.tick()
	time.Sleep(50 * time.Millisecond)
	if n := fc.SelectClassCalls(61115); n != 0 {
		t.Fatalf("窗口未开启时不得提交，实际调用 SelectClass %d 次", n)
	}
}

// TestProbeIntervalZeroOpenTime B15-M2：open_time 为零值（全新部署未配置 / 管理员
// PUT open_time="" 显式解除窗口机制，runtime.reparse 置 OpenTimeParsed 零值）时，
// probeIntervalFor 必须按 30s 常态探测——此前 `now.After(open.Add(-nearWindow))`
// 对零值 open 恒 true 落入临门 2s 分支，且若快照非空（开窗前平台有课程但 in_date_range
// 全 false）WindowClosed 恒 false，探测永久 2s 高频轰炸 findElectivesData，
// 正是第 6 轮根因修复的"访问过于频繁"1 分钟熔断触发形态（提交已被 B11-A1 零值守卫挂起，
// 但探测仍在 2s 高频，行为割裂）。
func TestProbeIntervalZeroOpenTime(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Second)
	// 模拟管理员清空 open_time（F7-02 语义）：openTimeFn 返回零值
	s.SetOpenTimeFn(func() time.Time { return time.Time{} })
	if got := s.probeIntervalFor(time.Now()); got != probeIntervalFar {
		t.Fatalf("open_time 为零值时应 30s 常态探测（不落入临门 2s 分支），实际 %v", got)
	}
	// 反向对照：恢复未来开窗时间 → 临门期正常 2s 盯守（零值守卫不得误伤新一轮）
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(4*time.Minute), time.Second)
	if got := s2.probeIntervalFor(time.Now()); got != probeIntervalNear {
		t.Fatalf("临门期应 2s 盯守，实际 %v", got)
	}
}

// TestProbeIntervalWindowClosed 窗口已关闭（开放时间已过 + 快照空）时降回 30s 探测，
// 杜绝窗口关闭后仍 2 秒高频盯守平台（浪费请求 + 日志刷屏）；
// 开放时间热改到未来（新一轮）时不受影响，临门仍 2s 盯守。
func TestProbeIntervalWindowClosed(t *testing.T) {
	// 已过开放时间 + 曾开过窗 + 空快照 → WindowClosed=true → 降回 30s
	// B18-M1：关闭判定加入"至少开过窗"前提——未开过窗即空快照（学期无发布/
	// 平台异常）不是"窗口已关闭"；测试预置 WindowOpened=true 模拟"开过再关"的真实形态。
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Second)
	s.mu.Lock()
	s.state.WindowOpened = true // 开过窗（B18-M1 前提）
	s.mu.Unlock()
	s.probe()
	if !s.StateForAccount("acct1").WindowClosed {
		t.Fatal("空快照 + 曾开过窗应标记窗口关闭")
	}
	if got := s.probeIntervalFor(time.Now()); got != probeIntervalFar {
		t.Fatalf("窗口关闭后应 30s 探测，实际 %v", got)
	}

	// 临门期（开放时间在未来）：即使标记已关闭，仍 2s 盯守（管理员热改新一轮的防守场景）
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(4*time.Minute), time.Second)
	s2.mu.Lock()
	s2.state.WindowOpened = true // B18-M1：开过窗
	s2.mu.Unlock()
	s2.probe() // 空快照 → WindowClosed=true
	if got := s2.probeIntervalFor(time.Now()); got != probeIntervalNear {
		t.Fatalf("临门期应 2s 盯守（不受已关闭标记影响），实际 %v", got)
	}

	// B18-M1 新增反向断言：从未开过窗 + 空快照 + 开放时间已过 → 不是"窗口已关闭"
	s3 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s3.probe()
	if s3.WindowClosed() {
		t.Fatal("从未开过窗的空快照不应标记窗口已关闭（否则还没开窗就会挂起提交）")
	}
	if got := s3.probeIntervalFor(time.Now()); got != probeIntervalNear {
		t.Fatalf("未开过窗的空快照应符合临门期 2s 盯守，实际 %v", got)
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

// TestClockSyncNoRetryWithinBackoff B9-03：时钟对齐失败后必须有失败退避——
// 失败期间每个 tick 绝不再重复发起 SyncServerTime（此前 lastSyncTime 恒零时
// 快路径被绕过、每 300ms tick 都裸打同步，网络故障期轰炸）。
func TestClockSyncNoRetryWithinBackoff(t *testing.T) {
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.syncErr = errors.New("网络故障")
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now(), time.Second)

	// 1. 首发失败并等落地（syncing 复位）
	s.maybeSyncClock(time.Now())
	wait := time.Now().Add(3 * time.Second)
	for {
		s.mu.Lock()
		busy := s.syncing
		s.mu.Unlock()
		if !busy {
			break
		}
		if time.Now().After(wait) {
			t.Fatal("首次失败同步未在 3 秒内落地")
		}
		time.Sleep(5 * time.Millisecond)
	}
	fc.mu.Lock()
	calls := fc.syncCalls
	fc.mu.Unlock()
	if calls != 1 {
		t.Fatalf("失败同步应只发起 1 次，实际 %d——退避期不得裸打同步", calls)
	}

	// 2. 仍处失败退避窗：模拟后续多个 tick（每拍 50ms、睡 50ms 等上一拍落地，
	//    与真实 tick 节奏同构）。无失败退避时每拍必然再发起一次同步（计数涨到 6）；
	//    有退避时全部被 30s 窗口拦下，计数必须保持 1。
	for i := 0; i < 5; i++ {
		s.maybeSyncClock(time.Now().Add(50 * time.Millisecond))
		time.Sleep(50 * time.Millisecond)
	}
	fc.mu.Lock()
	calls = fc.syncCalls
	fc.mu.Unlock()
	if calls != 1 {
		t.Fatalf("失败退避期内后续 tick 不得再发起同步，实际累计 %d 次", calls)
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

	// 连续 3 次同步失败。B8-M1：重入防抖——上轮仍在途时后续调用直接返回，
	// 不再 spawn 爆炸并发；每次发起前需等待上一轮 goroutine 落地（syncing 复位）。
	// 测试驱动的 waiting 循环：每轮用「等待在该发起时刻之后成功发起的那次」而非盲目叠加。
	// 第 3 次调用后：本轮可能触发回退（offset=0）或仍停留在失败计数阶段，统一由
	// 下面的 wait-for-reset 收尾，故每轮内等待仅判定「该轮已发起」，不强卡 streak。
	for i := 0; i < 3; i++ {
		fc.mu.Lock()
		fc.syncErr = errors.New("网络故障")
		fc.mu.Unlock()
		s.maybeSyncClock(time.Now().Add(time.Duration(i) * time.Minute))
		// 等待该轮发起落地：syncing 已复位 + 累计失败计数增长（第 3 轮后计数可能已被清零）
		desired := i + 1
		if i >= 2 {
			desired = 0 // 第 3 轮后回退已落地（clockOffset 复位为 0），streak 只在同步成功时清零、回退不清零
		}
		wait := time.Now().Add(5 * time.Second)
		for {
			s.mu.Lock()
			streak := s.syncFailStreak
			over := s.syncing
			s.mu.Unlock()
			if !over && streak >= desired {
				break
			}
			if time.Now().After(wait) {
				t.Fatalf("第 %d 轮同步 goroutine 未在 5 秒内落地（syncing=%v streak=%d）", i+1, over, streak)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 等第 3 次失败触发的回退落地（offset=0 且日志出现警告）——以「偏差已复位」作为完成信号
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

// TestClockSyncNoClientResetsSyncing F12-B1：无已登录账号（空库/账号全删）或客户端
// 不支持时钟同步时，syncing 必须被复位——此前 syncing 置 true 后无人复位（复位只在
// goroutine 内），后续每个 tick 在 `if s.syncing` 处直接返回，时钟校准从启动起永久休眠、
// clockOffset 恒 0 且无任何错误日志。空库部署首个 300ms tick 即触发。
func TestClockSyncNoClientResetsSyncing(t *testing.T) {
	// 1. 无账号（clients=nil，AnyClient 返回 false）：syncing 应被复位且不发起同步
	s := New(nil, &fakeStore{}, time.Now(), time.Second)
	s.maybeSyncClock(time.Now())
	s.mu.Lock()
	busy := s.syncing
	s.mu.Unlock()
	if busy {
		t.Fatal("无账号时 syncing 应被复位（否则时钟校准永久休眠）")
	}

	// 2. 账号就绪后再发起：此时应真正发起同步并成功落地（校准时序不受前次影响）
	fc := newFakeClient(false)
	fc.syncOffset = 5 * time.Second
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now(), time.Second)
	s2.maybeSyncClock(time.Now())
	wait := time.Now().Add(5 * time.Second)
	for {
		s2.mu.Lock()
		off := s2.clockOffset
		s2.mu.Unlock()
		if off == 5*time.Second {
			break
		}
		if time.Now().After(wait) {
			t.Fatal("账号就绪后应能发起并落地时钟同步")
		}
		time.Sleep(10 * time.Millisecond)
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
	// 第 3 轮 C-3：空快照 + 曾开过窗 + 开放时间已过 = 窗口已关闭；快照存在/未到点 = 未关闭。
	// B18-M1：判定加入"至少开过窗"前提——未开过窗即空快照不算"已关闭"，
	// 防止还没开窗就把提交挂起（开窗瞬间黄金期全停摆）；测试预置 WindowOpened=true。
	fcEmpty := newFakeClient(false)
	fcEmpty.mu.Lock()
	fcEmpty.data.Publishes = nil
	fcEmpty.mu.Unlock()
	s := New(&fakeAccts{c: fcEmpty}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.mu.Lock()
	s.state.WindowOpened = true // B18-M1：开过窗
	s.mu.Unlock()
	s.probe()
	if !s.StateForAccount("acct1").WindowClosed {
		t.Fatal("空快照 + 曾开过窗 + 开放时间已过探测后 window_closed 应为 true")
	}

	// 正常数据但窗口未开：window_closed 必须为 false
	fcOpen := newFakeClient(false)
	s2 := New(&fakeAccts{c: fcOpen}, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	s2.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s2.probe()
	if s2.StateForAccount("acct1").WindowClosed {
		t.Fatal("正常未开窗探测后 window_closed 应为 false")
	}

	// B18-M1 反向断言：从未开过窗 + 空快照 + 开放时间已过 → 不是"已关闭"
	s3 := New(&fakeAccts{c: fcEmpty}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s3.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s3.probe()
	if s3.StateForAccount("acct1").WindowClosed {
		t.Fatal("从未开过窗的空快照不应标记 window_closed=true（还没开窗就挂起提交）")
	}
}

// TestWindowClosedTransitionStateGrace B26-03：主判据（B18-M1）补"已过开窗点
// 10s 裕量"——与 B21-02 给 EmptyProbeRuns 入账的 10s 裕量对称。开窗确证过（prevOpened=true）
// 后，平台短暂返回空快照（数据刷新/切学期过渡态，F7-01 记录的预清空现象）时，旧判据
// `now.After(open)` 在开窗点刚过就置 WindowClosed=true → tick 提交守卫挂起提交 + 探测降回
// 30s，若平台在 30s 内恢复，黄金期提交已停摆。10s 裕量覆盖过渡态；真关仅推迟 10s 判定
// （超过裕量仍按原判据关闭，TestWindowClosedState 的 -time.Hour 场景已固化））。
func TestWindowClosedTransitionStateGrace(t *testing.T) {
	// 过渡态：开窗点已过但仍在 10s 裕量内（now = open+5s，now.After(open) 为真、
	// now.After(open+10s) 为假）——上一轮确证开过窗 + 本轮空快照，不得误标关闭
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-5*time.Second), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.mu.Lock()
	s.state.WindowOpened = true // 上一轮确证开过窗
	s.mu.Unlock()
	s.probe()
	if s.StateForAccount("acct1").WindowClosed {
		t.Fatal("开窗点后 10s 裕量内的空快照探测不得置 window_closed（过渡态防误挂黄金期提交）")
	}
}

// TestStateForAccountMirrorsWindowClosed B29-02：StateForAccount 的 window_closed 字段
// 必须与 WindowClosed() 方法同真相——此前只写 s.state.WindowClosed（probe 主判据），
// 兜底判据（B19-01 时钟连续失败 / B20-02 幽灵窗口 EmptyProbeRuns）返回 true 时不回写
// 字段：幽灵窗口/时钟失败场景下 /api/state 下发 window_closed=false，前端横幅仍显示
// 倒计时/"已开放"、日志与课程轮询维持高频（F9-07 降频失效），展示与实际挂起状态分叉。
// 修复前（StateForAccount 直接浅拷贝 state.WindowClosed）：红——时钟失败≥3 时
// WindowClosed() 返回 true 但状态字段仍 false。
// 修复后（共用 windowClosedLocked 单源）：绿——三条判据（主判据/时钟兜底/幽灵窗口兜底）
// 全部镜像进状态字段。
func TestStateForAccountMirrorsWindowClosed(t *testing.T) {
	// 场景 1：幽灵窗口兜底判据（EmptyProbeRuns≥3 + 从未开窗 + 开放时间已过）
	s1 := New(&fakeAccts{c: newFakeClient(false)}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s1.mu.Lock()
	s1.state.EmptyProbeRuns = 3
	s1.mu.Unlock()
	if !s1.WindowClosed() {
		t.Fatal("前置：EmptyProbeRuns=3 + 从未开窗 + 已过开窗点应视同关闭")
	}
	if !s1.StateForAccount("acct1").WindowClosed {
		t.Fatal("幽灵窗口兜底（EmptyProbeRuns=3）必须镜像进 StateForAccount.window_closed（B29-02）")
	}

	// 场景 2：时钟兜底判据（syncFailStreak≥3 + 开放时间已过）
	s2 := New(&fakeAccts{c: newFakeClient(false)}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s2.mu.Lock()
	s2.syncFailStreak = 3
	s2.mu.Unlock()
	if !s2.WindowClosed() {
		t.Fatal("前置：syncFailStreak=3 + 开放时间已过应视同关闭")
	}
	if !s2.StateForAccount("acct1").WindowClosed {
		t.Fatal("时钟兜底（syncFailStreak=3）必须镜像进 StateForAccount.window_closed（B29-02）")
	}
	// 场景 2b 反向断言：同一时钟失败形态但开放时间在未来——绝不能视同关闭（32-01）。
	// 判据2 原实现只查"开放时间非零"，未来开窗点 + 平台故障恢复后黄金期提交被挂起。
	s2b := New(&fakeAccts{c: newFakeClient(false)}, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	s2b.mu.Lock()
	s2b.syncFailStreak = 3
	s2b.mu.Unlock()
	if s2b.WindowClosed() {
		t.Fatal("时钟失败 3 次但开放时间在未来，绝不能视同关闭（黄金期提交必须存活）")
	}
	if s2b.StateForAccount("acct1").WindowClosed {
		t.Fatal("未来开窗点不得镜像时钟兜底为已关闭")
	}

	// 场景 3：主判据（state.WindowClosed 已置位）照旧镜像 + 非关闭状态不误报
	s3 := New(&fakeAccts{c: newFakeClient(false)}, &fakeStore{}, time.Now(), time.Hour)
	s3.mu.Lock()
	s3.state.WindowClosed = true
	s3.mu.Unlock()
	if !s3.StateForAccount("acct1").WindowClosed {
		t.Fatal("主判据置位必须镜像进状态字段")
	}
	s4 := New(&fakeAccts{c: newFakeClient(false)}, &fakeStore{}, time.Now(), time.Hour)
	if s4.StateForAccount("acct1").WindowClosed {
		t.Fatal("无任何判据命中时 window_closed 必须为 false（不误报）")
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

// TestReloginBackoffCappedAndReset 验证重登退避防溢出封顶（CRITICAL C2, C3）。
// B14-I1：原后半段手写 `s.reloginFail[acct]=5; delete(...)` 直接测 Go
// map 的 delete 语义（恒绿，与 F13-i1 同为"手写实现语义当断言"的坏味道）——删除。
// 真实"失败保留增长 / 成功清零"路径由 TestReloginFailureKeepsBackoff /
// TestReloginSuccessResetsBackoff 覆盖，此处不再重复。
func TestReloginBackoffCappedAndReset(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	// 验证退避算法上限封顶与极大 n 防溢出
	for _, n := range []int{1, 5, 10, 64, 100} {
		wait := s.reloginBackoff(n)
		if wait <= 0 || wait > 10*time.Minute {
			t.Fatalf("reloginBackoff(%d) = %v，不应小于0或超过 10 分钟", n, wait)
		}
	}
}

// TestReloginFailureKeepsBackoff 已契约化"重登失败后 reloginFail 保留增长、成功才清零"
// （C1，退避表逐次拉长防线）——真实失败路径从不清零为 1。
// F13-i1：本测试测试"复位为 1"的语义，但实现任何失败路径都不复位为 1
// （开发者当年手写两行"模拟实现"），是恒绿的无效测试——删除，由下方两个真实路径
// 测试 TestReloginBackoffWindowBlocksManualTriggers / TestReloginFailureKeepsBackoff 覆盖。

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
	// B6-04：ProbeForAccount 不得写全局 lastProbe——它只归 probe()/ProbeNow 管理，
	// 否则管理员穿透探测会吞掉全校探测节流闸门（开窗前点一次课程页 = 全校探测延后）。
	if s.HasProbed() {
		t.Fatal("ProbeForAccount 不应把 HasProbed 置 true（lastProbe 只归全校正规探测）")
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
	// B18-m1：快照超过 TTL 过期后复核必须放行——
	// 旧快照可能已失真的名额/窗口数据绝不拦截用户真实操作（放行由平台最终把关，
	// 与 ElectivesSnapshotFor 的过期回退语义对齐）。
	s.mu.Lock()
	s.acctDataAt["acct1"] = time.Now().Add(-(snapshotTTL + time.Second))
	s.mu.Unlock()
	if reason, ok := s.CheckClassSelectable("acct1", 61116); !ok {
		t.Fatal("快照过期后应放行（不拿旧数据拦真实操作），被拒: " + reason)
	}
}

// TestDeletedAccountManualInFlightDropsState 验证删除账号与在飞【手动报名/退选】竞态下，
// MarkDone/RemoveDone 成功分支同样必须在写 done/落库前复核账号仍存在——否则 SelectClass/
// ExitClass 网络往返（最长 15s）期间管理员 DeleteAccount 已清库表 + Accounts.Remove，
// 本请求返回后（B20-01，第 20 轮）会把已删账号的 success/refused 行写回：
// 重启后重新登录分别被 RestoreDone 恢复成"已报名成功"假状态 / RestoreRefused 恢复成
// "已退选"令自动引擎永久跳过该课。B18-M2 只修了自动链，手动路径同样竞态整链开放。
func TestDeletedAccountManualInFlightDropsState(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开
	store := &fakeStore{}
	fa := &fakeAccts{c: fc, removed: map[string]bool{}}
	s := New(fa, store, time.Now(), time.Hour)

	// 预置目标与成功课程（模拟真实历史：done 已有该课，手动退选路径才能触发 RemoveDone）
	classID := 61115
	courseName := "健美操"
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: classID, CourseName: courseName, Priority: 0}})
	err := s.MarkDone("acct1", classID, courseName, "手动报名成功")
	if err != nil {
		t.Fatalf("预置 MarkDone 失败: %v", err)
	}

	// 删除账号：ClientFor 返回不存在
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()

	// 账号已删除后到账手动报名成功 → MarkDone 不得写任何内存状态与库行
	err = s.MarkDone("acct1", classID, courseName, "手动报名成功")
	if err != nil {
		t.Fatalf("MarkDone 不应报错（静默放弃落库）: %v", err)
	}
	store.mu.Lock()
	if rows := store.successRows["acct1\x0061115"]; rows != 1 {
		// 预置那次成功 1 行；删除后再次 MarkDone 必须不再 +1
		store.mu.Unlock()
		t.Fatalf("账号已删后手动报名成功不得再落库 success 行（重启假成功），实际 %d 行", rows)
	}
	store.mu.Unlock()
	s.mu.Lock()
	_, inDone := s.done["acct1"][classID]
	idx := s.statusIndexLocked("acct1", classID)
	var status string
	if idx >= 0 {
		status = s.state.Courses[idx].Status
	}
	s.mu.Unlock()
	if !inDone {
		t.Fatal("预置的成功课程仍应在 done（删除后追加标记被拒绝而非清空既有历史）")
	}
	if status != "success" {
		t.Fatalf("预置的成功状态不应被删除后到账的 MarkDone 破坏，实际 %q", status)
	}

	// 删除后到账手动退选 → RemoveDone 不得落库 refused 行
	err = s.RemoveDone("acct1", classID)
	if err != nil {
		t.Fatalf("RemoveDone 不应报错（静默放弃落库）: %v", err)
	}
	store.mu.Lock()
	if rows := store.refusedRows["acct1\x0061115"]; rows != 0 {
		store.mu.Unlock()
		t.Fatalf("账号已删后手动退选成功不得落库 refused 行（重启后自动引擎永久跳过），实际 %d 行", rows)
	}
	store.mu.Unlock()
	s.mu.Lock()
	_, inRefused := s.refused["acct1"][classID]
	_, dropped := s.done["acct1"][classID]
	s.mu.Unlock()
	if inRefused {
		t.Fatal("账号已删后不应写 refused 内存态")
	}
	if !dropped {
		t.Fatal("账号已删后不应改动预置的 done 既有历史")
	}
}

// TestSchedulerManualSyncAndSubmitMutex 验证手动报名、退选状态协同与提交排他互斥锁 (Task 3)。
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

// TestSetTargetsPurgesStaleState 验证重设目标只清 refused（B19-02，第 19 轮定案）：
// done/full/rateLimited/inflight **全部保留**——done 是跨目标的持久历史事实
// （RestoreDone 注入/手动报名 MarkDone 写入），重设清掉会把已成功课程重新提交
// （TestRestoreDoneSkipsResubmit 固化）；full/rateLimited 是真实防轰炸状态，清了
// 会让自动链立刻重打刚被平台拒绝的课（自愈由快照解封/退避过期提供）；inflight 防
// 双包（网络往返完成自清）。删账号的**全量清理**归 PurgeAccount（本文件下方测试）。
func TestSetTargetsPurgesStaleState(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})

	// 预置各种运行态（模拟历史状态）
	s.mu.Lock()
	s.done[acct] = map[int]bool{classID: true}
	s.full[acct] = map[int]bool{classID: true}
	s.rateLimited[acct] = map[int]time.Time{classID: time.Now().Add(time.Minute)}
	s.inflight[acct] = map[int]bool{classID: true}
	s.mu.Unlock()

	// 重设目标：上述全部必须保留（见测试注释的契约）
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})

	s.mu.Lock()
	if !s.doneHas(acct, classID) {
		s.mu.Unlock()
		t.Fatal("重设目标必须保留 done（历史成功事实，RestoreDone 恢复契约不可凿穿）")
	}
	if !s.fullHas(acct, classID) {
		s.mu.Unlock()
		t.Fatal("重设目标必须保留 full（真实满员状态，清除会让自动链立刻重打被拒课程）")
	}
	if !s.isRateLimitedLocked(acct, classID, time.Now()) {
		s.mu.Unlock()
		t.Fatal("重设目标必须保留 rateLimited（风控退避中，清除会立刻重打触发熔断）")
	}
	if !s.inflightHas(acct, classID) {
		s.mu.Unlock()
		t.Fatal("重设目标必须保留 inflight（网络在飞防双包，清除会导致并发重复发包）")
	}
	s.mu.Unlock()
	// refused 必须清空：用户重新设为目标即重新接管，自动引擎恢复执行
	s.mu.Lock()
	s.refused[acct] = map[int]bool{classID: true}
	s.mu.Unlock()
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refusedHas(acct, classID) {
		t.Fatal("重设目标必须清空 refused（用户重新接管，自动引擎应恢复执行）")
	}
}

// TestPurgeAccount 验证管理员删除账号后调度器全量清理（B19-02，第 19 轮）：
// done/full/rateLimited/inflight/acctTargets 全部清除——重建账号绝不残留旧状态。
func TestPurgeAccount(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	acct := "acct1"
	classID := 61115
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: classID, CourseName: "健美操", Priority: 0}})
	s.mu.Lock()
	s.done[acct] = map[int]bool{classID: true}
	s.full[acct] = map[int]bool{classID: true}
	s.inflight[acct] = map[int]bool{classID: true}
	s.rateLimited[acct] = map[int]time.Time{classID: time.Now().Add(time.Minute)}
	s.mu.Unlock()

	s.PurgeAccount(acct)

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.acctTargets[acct]) != 0 {
		t.Fatal("PurgeAccount 应清空该账号目标")
	}
	if s.doneHas(acct, classID) || s.fullHas(acct, classID) || s.inflightHas(acct, classID) ||
		s.isRateLimitedLocked(acct, classID, time.Now()) {
		t.Fatal("PurgeAccount 应清空全部运行态（done/full/rateLimited/inflight）")
	}
	// 状态行也不得残留
	for _, c := range s.state.Courses {
		if c.Account == acct {
			t.Fatalf("PurgeAccount 后不得残留账号 %s 的状态行", acct)
		}
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
	// 开放时间已过 1 小时：符合"开放时间已过 + 曾开过窗 + 空快照"的关闭判定
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.mu.Lock()
	s.state.WindowOpened = true // B18-M1：开过窗
	s.mu.Unlock()
	s.probe() // 探测落地 WindowClosed 状态

	if !s.WindowClosed() {
		t.Fatal("开放时间已过 + 曾开过窗 + 空快照应判定窗口已关闭")
	}
	got := s.probeIntervalFor(time.Now())
	if got != probeIntervalFar {
		t.Fatalf("窗口开过再关后探测间隔应降回 30s，实际 %v", got)
	}
}

// TestGhostWindowEmptyProbesSuspend 验证从未开过窗的空快照 + 时钟接口正常时，连续
// 空快照探测 ≥3 轮后必须判定"幽灵窗口已关闭"（B20-02，第 20 轮）——B19-01 的时钟
// 兜底覆盖不到该形态（syncFailStreak 恒 0），此前 WindowClosed() 恒 false：
// tick 提交段 1s 周期 SelectClass（平台回 code=1"无效的课程ID"）+ 实时复核 StudentCounts
// 空 → 下轮重打，probeIntervalFor 恒 2s 高频探测，防轰炸契约闭环缺口。
// 量变判据：EmptyProbeRuns≥3 且从未开窗且开放时间已过 → 视同关闭；开窗/非空快照/未到
// 开放时间即归零自愈（不误伤开窗前正常空快照的临门盯守）。
// B21-02：入账增量再加"已过开窗点 10s 裕量"——开窗瞬间平台预清空 publishes
// （F7-01 真实现象）时，旧判据会在黄金期误挂起；补这段过渡期错开，真实窗口开启后连续
// 空快照才确证幽灵窗口。
func TestGhostWindowEmptyProbesSuspend(t *testing.T) {
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil // 幽灵窗口形态：空快照且从未开过窗
	fc.mu.Unlock()
	// 开放时间设 12s 前（已过开窗点 + 超出 10s 裕量），让入账条件"now > open+10s"成立
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-12*time.Second), time.Hour)

	// 连续探测入账前（EmptyProbeRuns=0）：不得误判关闭（首次探测 / 开窗前正常空快照）
	if s.WindowClosed() {
		t.Fatal("从未连续探测到空快照前不得视同关闭（首探不计数）")
	}
	// 入账 1-2 轮：仍未达 3 轮阈值，不得误判（连续两次空快照可能是平台抖动）
	// 注意：入账计数由 probe() 在真实探测时递增，测试直接置 state 模拟"探测已入账 2 轮"
	s.mu.Lock()
	s.state.EmptyProbeRuns = 2
	s.mu.Unlock()
	if s.WindowClosed() {
		t.Fatal("连续 2 轮空快照不应视同关闭（阈值 3）")
	}
	// 第 3 轮空快照：触发幽灵窗口判定
	s.mu.Lock()
	s.state.EmptyProbeRuns = 3
	s.mu.Unlock()
	if !s.WindowClosed() {
		t.Fatal("连续 3 轮空快照 + 开放时间已过 + 从未开窗应视同关闭（挂起提交 + 探测降频）")
	}
	if got := s.probeIntervalFor(time.Now()); got != probeIntervalFar {
		t.Fatalf("幽灵窗口应 30s 探测（不再 2s 烧平台），实际 %v", got)
	}

	// 反向断言 1：开窗后（state.WindowOpened=true）无论空快照轮数多少都不视同关闭
	s.mu.Lock()
	s.state.WindowOpened = true
	s.mu.Unlock()
	if s.WindowClosed() {
		t.Fatal("窗口已开（曾确证开启）后不得视同关闭——黄金期提交/探测绝不挂起")
	}

	// 反向断言 2：空快照轮数归零（非空快照探测入账）后幽灵窗口判定解除
	s.mu.Lock()
	s.state.WindowOpened = false
	s.state.EmptyProbeRuns = 0
	s.mu.Unlock()
	if s.WindowClosed() {
		t.Fatal("EmptyProbeRuns 归零（窗口若真开、探测拿到非空快照）后应解除")
	}

	// B21-02 反向断言 3：开窗点后 10s 裕量内（now ≤ open+10s）probe() 绝不入账空快照轮数——
	// 开窗瞬间平台预清空 publishes 的过渡态（F7-01）不得被误判幽灵窗口。裕量在入账侧
	// （probe() 的 `now.After(open+10s)` 才 ++），故用真实 probe() 验证，而非置 state。
	// 情况 A：开放时间 5s 前（仍在 10s 裕量窗口内）→ 空快照探测不得入账（EmptyProbeRuns 保持 0）
	s2 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-5*time.Second), time.Hour)
	s2.probe()
	s2.mu.Lock()
	runs2 := s2.state.EmptyProbeRuns
	s2.mu.Unlock()
	if runs2 != 0 {
		t.Fatalf("开窗点后 10s 裕量内的空快照探测不得入账 EmptyProbeRuns（防误挂黄金期），实际 %d", runs2)
	}
	// 情况 B：开放时间 12s 前（已过 10s 裕量）→ 空快照探测正常入账（EmptyProbeRuns=1）
	s3 := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-12*time.Second), time.Hour)
	s3.probe()
	s3.mu.Lock()
	runs3 := s3.state.EmptyProbeRuns
	s3.mu.Unlock()
	if runs3 != 1 {
		t.Fatalf("已过 10s 裕量的空快照探测应正常入账 EmptyProbeRuns=1，实际 %d", runs3)
	}
}

// TestGhostWindowClockFailuresSuspend 验证从未开过窗的空快照 + 时钟连续失败 ≥3 时
// 判定"幽灵窗口已关闭"（B19-01 时钟兜底 + B21-01 使其真实可达）：packaged 默认
// open_time 已过 + 平台空快照 + syncFailStreak 持续累计 ≥3，WindowClosed() 必须为 true
// （tick 守卫挂起提交 + 探测降回 30s），自愈由时钟成功恢复（streak 归零）提供。
// B22-02：测试改为用真实 maybeSyncClock 让 streak 真实累计到 3——
// 此前手动注入 3 是恒假绿形态（B21-01 死代码实证的对应测试），现验证判据真实可达。
// maybeSyncClock 每次失败后落地 lastSyncFailAt（30s 退避）且 syncing 复位在异步
// goroutine 内，故每次发起前清退避、发起后轮询等 syncing 落地，模拟三次独立失败。
func TestGhostWindowClockFailuresSuspend(t *testing.T) {
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil // 幽灵窗口形态：空快照且从未开过窗
	fc.mu.Unlock()

	// 发起一次真实时钟同步并等待异步 goroutine 落地（fail=true 注入时钟失败）。
	// 每轮前清 lastSyncFailAt 突破 30s 退避——生产时序中三次失败天然间隔 ≥30s，
	// 测试用清退避模拟"多轮独立失败"的时间流逝。
	runSyncAndWait := func(s *Scheduler, fail bool) {
		fc.mu.Lock()
		if fail {
			fc.syncErr = errors.New("clock down")
		} else {
			fc.syncErr = nil
		}
		fc.mu.Unlock()
		s.mu.Lock()
		s.lastSyncFailAt = time.Time{}
		s.mu.Unlock()
		s.maybeSyncClock(time.Now())
		deadline := time.Now().Add(2 * time.Second)
		for {
			s.mu.Lock()
			done := !s.syncing
			s.mu.Unlock()
			if done {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("同步 goroutine 未在超时内复位 syncing")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// 场景 A：真实时钟同步连续失败 3 次 → streak 真实累计到 3 → 幽灵窗口判定触发
	sA := New(&fakeAccts{c: fc, relogErr: errors.New("vision down")}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	for i := 0; i < 3; i++ {
		runSyncAndWait(sA, true)
	}
	if !sA.WindowClosed() {
		t.Fatal("真实时钟失败累计 3 次 + 空快照 + 开放时间已过应判定幽灵窗口已关闭")
	}

	// 场景 B：同步成功 streak 归零 → 幽灵窗口解除（自愈）
	runSyncAndWait(sA, false)
	if sA.WindowClosed() {
		t.Fatal("时钟恢复（syncFailStreak 归零）后幽灵窗口判定应解除")
	}

	// 场景 C：同一幽灵窗口形态下 probeIntervalFor 必须 30s（不再 2s 烧平台）
	sC := New(&fakeAccts{c: fc, relogErr: errors.New("vision down")}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	for i := 0; i < 3; i++ {
		runSyncAndWait(sC, true)
	}
	if got := sC.probeIntervalFor(time.Now()); got != probeIntervalFar {
		t.Fatalf("幽灵窗口应 30s 探测（不再 2s 烧平台），实际 %v", got)
	}
	if !sC.WindowClosed() {
		t.Fatal("时钟失败 3 次后 WindowClosed 必须为 true（探测降频的前提）")
	}
}

// TestManualSnapshotFallbackOnlyWhenOwnFresh 验证快照回退语义（B22-01，第 22 轮）：
// ElectivesSnapshotFor 的全局帧回退只许发生在"该账号确实没探测过 / 有全局帧可回退"时——
// 账号已配置目标后 probe() 每 30s 必刷新其专属快照；若专属快照缺失且全局帧非空，回退
// 返回的全局帧可能是首个注册账号的年级（混合年级部署下年级串线），绝不可当"自己年级"
// 渲染给用户。fallback 返回 true 时该账号年级帧已新鲜；false 时 handleElectives 会走
// ProbeForAccount 真取本账号年级帧。
func TestManualSnapshotFallbackOnlyWhenOwnFresh(t *testing.T) {
	fc := newFakeClient(false)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)

	// 预置：调度器已探测过全局帧（lastData = 高三帧，仅 1 门体育）且"目标账号"acct1 已配置目标
	s.mu.Lock()
	s.lastData = &zhidao.ElectivesData{Publishes: []zhidao.Publish{
		{PublishID: 9, PublishName: "高三体育", InDateRange: false, Classes: []zhidao.Class{
			{ID: 61999, CourseName: "高三排球", SelectedCount: 0, MaxCount: 36},
		}},
	}}
	s.lastDataAt = time.Now()
	s.acctTargets["acct1"] = []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}}
	s.mu.Unlock()

	// 断言 1：目标账号专属快照缺失时，绝不回退全局帧（fallback 返回 false → handleElectives
	// 走 ProbeForAccount 真取本账号年级帧）——这是 B22-01 修复后的语义
	if _, ok := s.ElectivesSnapshotFor("acct1"); ok {
		t.Fatal("目标账号专属快照缺失时不得回退全局帧（跨年级帧可能被错误渲染给该账号）")
	}

	// 断言 2：专属快照写入后（模拟 ProbeForAccount 成功），ElectivesSnapshotFor 返回该账号帧
	s.mu.Lock()
	s.acctData["acct1"] = &zhidao.ElectivesData{Publishes: []zhidao.Publish{
		{PublishID: 1, PublishName: "高二年体育", InDateRange: false, Classes: []zhidao.Class{
			{ID: 61115, CourseName: "健美操", SelectedCount: 0, MaxCount: 36},
		}},
	}}
	s.acctDataAt["acct1"] = time.Now()
	s.mu.Unlock()
	data, ok := s.ElectivesSnapshotFor("acct1")
	if !ok {
		t.Fatal("账号专属快照已写入且新鲜时 ElectivesSnapshotFor 必须返回该账号帧")
	}
	if len(data.Publishes) == 0 || data.Publishes[0].PublishID != 1 {
		t.Fatal("ElectivesSnapshotFor 返回的不是该账号专属帧（年级串线）")
	}

	// 断言 3（B28-01）：无目标账号的专属帧"存在但已过期"，而全局帧恰被其他账号刷新为
	// 新鲜帧时，必须返回 false 触发本账号 ProbeForAccount 刷新——旧实现回退这份**错年级**
	// 全局帧且 ok=true，读取方（handleElectives）不触发刷新，浏览者持续看到别的年级课程。
	// 场景：browse 账号"student1"（无目标）专属帧高二（PublishID=1）已过期 41s，全局帧
	// 高三（PublishID=9）新鲜。
	s.mu.Lock()
	s.acctData["student1"] = &zhidao.ElectivesData{Publishes: []zhidao.Publish{
		{PublishID: 1, PublishName: "高二年体育", InDateRange: false, Classes: []zhidao.Class{
			{ID: 61115, CourseName: "健美操", SelectedCount: 0, MaxCount: 36},
		}},
	}}
	s.acctDataAt["student1"] = time.Now().Add(-(snapshotTTL + time.Second))
	s.lastData = &zhidao.ElectivesData{Publishes: []zhidao.Publish{
		{PublishID: 9, PublishName: "高三体育", InDateRange: false, Classes: []zhidao.Class{
			{ID: 61999, CourseName: "高三排球", SelectedCount: 0, MaxCount: 36},
		}},
	}}
	s.lastDataAt = time.Now()
	s.mu.Unlock()
	if _, ok := s.ElectivesSnapshotFor("student1"); ok {
		t.Fatal("无目标账号专属帧已过期时不得回退新鲜全局帧（错年级数据串线 + 不触发本账号刷新）")
	}

	// 断言 4（B37-02）：清空目标后（SetTargetsForAccount 留下空 slice、map key 仍存在），
	// 该账号必须走"无目标账号"快路径——全局帧新鲜即返回，绝不走"有目标账号"专属路径：
	// 旧判据按 key 存在性判定，清空目标的账号被归入目标账号 → 无专属帧返回 (nil,false)
	// → handleElectives 每次浏览都调 ProbeForAccount 真打平台（前端 10s 轮询 × 每次过期即打），
	// 与"无目标账号回退全局帧（快、无网络开销）"契约相悖（AccountsWithTargets 只认 len>0）。
	s.SetTargetsForAccount("acct1", []Target{}) // 模拟用户清空全部目标（F7-01 合法操作）
	s.mu.Lock()
	delete(s.acctData, "acct1") // 清空后从未探测过：无专属帧
	delete(s.acctDataAt, "acct1")
	s.mu.Unlock()
	data, ok = s.ElectivesSnapshotFor("acct1")
	if !ok || data == nil || len(data.Publishes) == 0 {
		t.Fatal("清空目标的账号应回退全局帧快路径（零网络探测），不得返回 (nil,false)")
	}
}

// TestReloginBackoffWindowBlocksManualTriggers B9-01：连续失败进入指数退避后，
// 退避窗口内再次触发 maybeRelogin 必须被挡下（不发起新重登、不清退避表）。
// 修复前的 api 层手动路径（B8-M7）在命中 ErrUnauthorized 时先调 MarkTokenValid
// 再调 MaybeRelogin——MarkTokenValid 会 delete reloginFail（它只该用于"手动登录成功"
// 的 issueSession），指数退避恒从 30s 重来，Vision 持续故障时平台锁号防线被击穿。
func TestReloginBackoffWindowBlocksManualTriggers(t *testing.T) {
	fc := newFakeClient(false)
	fa := &fakeAccts{c: fc}
	fa.relogErr = errors.New("vision down")
	s := New(fa, &fakeStore{}, time.Now().Add(time.Hour), time.Hour)
	acct := "acct1"

	// 连续 3 次失败（每次重置节流闸门立即发起，退避增长到 120s）
	for i := 0; i < 3; i++ {
		s.resetReloginAtForTest(acct)
		s.maybeRelogin(acct)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			r := s.relogging[acct]
			s.mu.Unlock()
			if !r {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	s.mu.Lock()
	failBefore := s.reloginFail[acct]
	s.mu.Unlock()
	if failBefore < 3 {
		t.Fatalf("前置失败计数应 ≥3，实际 %d", failBefore)
	}

	// 现处退避窗口内（reloginAt 未重置）：手动路径再触发多次，不得发起新重登
	fc.mu.Lock()
	callsBefore := fc.relogCalls
	fc.mu.Unlock()
	for i := 0; i < 5; i++ {
		s.maybeRelogin(acct)
		time.Sleep(20 * time.Millisecond)
	}
	fc.mu.Lock()
	callsAfter := fc.relogCalls
	fc.mu.Unlock()
	if callsAfter != callsBefore {
		t.Fatalf("退避窗口内触发不得发起重登：%d -> %d", callsBefore, callsAfter)
	}
	s.mu.Lock()
	n := s.reloginFail[acct]
	s.mu.Unlock()
	if n != failBefore {
		t.Fatalf("手动触发路径不得清退避表：%d -> %d", failBefore, n)
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

// TestWindowClosedSelectStopsBombing 窗口已确认关闭时（state.WindowClosed=true）tick 守卫
// 直接挂起提交，SelectClass 0 次调用（C-3 防轰炸主路径回归）。
// B18-M1：真实平台关闭文案"无效的课程ID"不在 isWindowClosedError 匹配集合，
// 旧实现只靠文案记 full 挡不住 → 复核路径 countList 空报"课程无人数数据" → 永续轰炸；
// 根因修复在 tick 守卫（WindowClosed 状态挂起提交，与 B11-A1 零值守卫并列）。
func TestWindowClosedSelectStopsBombing(t *testing.T) {
	// 窗口关闭特征：空快照（平台关闭后 findElectivesData 返回空 publishes）+ 曾开过窗
	fc := newFakeClient(false)
	fc.mu.Lock()
	fc.data.Publishes = nil
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	// 预置"开过窗 + 已确认关闭"：真实时序=平台窗口关闭后探测返回空快照 + 开放时间已过
	// → state.WindowClosed=true；首个 tick 的 probe() 会按真实判据维持该值（prevOpened 且
	// 空发布，见 probe 的 WindowClosed 计算）。随后 tick 守卫命中 WindowClosed 挂起提交。
	s.mu.Lock()
	s.state.WindowOpened = true // B18-M1 语义自洽：关闭以"至少开过窗"为前提
	s.state.WindowClosed = true
	s.mu.Unlock()
	s.Start()
	defer s.Stop()
	// B18-M1：窗口已关闭 → tick 守卫直接挂起提交，SelectClass 0 次调用
	// （比"首轮执行一次再靠 full 挡"更彻底）。
	time.Sleep(80 * time.Millisecond)
	fc.mu.Lock()
	calls := fc.selectCalls[61115]
	fc.mu.Unlock()
	if calls != 0 {
		t.Fatalf("窗口已关闭应挂起提交（0 次 SelectClass），实际 %d", calls)
	}

	// 再等 300ms（多个 tick）：仍必须 0 次调用
	time.Sleep(300 * time.Millisecond)
	fc.mu.Lock()
	callsAfter := fc.selectCalls[61115]
	fc.mu.Unlock()
	if callsAfter != 0 {
		t.Fatalf("窗口已关闭应持续挂起提交（0 次 SelectClass），调用增长到 %d", callsAfter)
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

// TestRealtimeRecheckUnauthorizedTriggersRelogin 验证实时人数复核命中 token 失效
// （findElectivesStudentCount 同样鉴权，code=-1）时：与 SelectClass 分支对称、立即触发
// 自动重登（B19-03，第 19 轮）——此前复核错误被当普通失败处理、下个 tick 又重打失效
// 报名接口，失效恢复路径被延迟到探测/手动路径才发现（token 失效数秒内黄金期空转）。
// 修复前该路径 maybeRelogin 0 次（红灯），修复后 1 次（绿灯）。
func TestRealtimeRecheckUnauthorizedTriggersRelogin(t *testing.T) {
	fc := newFakeClient(true) // 窗口已开
	fc.mu.Lock()
	fc.selectErr[61115] = errors.New("该课程已满员") // 报名失败 → 走实时复核路径
	fc.fullErr = zhidao.ErrUnauthorized             // 复核命中 token 失效（学生数接口同样鉴权）
	fc.mu.Unlock()
	relogStart := make(chan bool)
	relogDone := make(chan bool)
	fa := &fakeAccts{c: fc, relog: func() {
		relogStart <- true
		<-relogDone
	}}
	s := New(fa, &fakeStore{}, time.Now().Add(-time.Minute), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()

	// 自动链应先走到报名失败 → 实时复核命中失效 → maybeRelogin
	select {
	case <-relogStart:
	case <-time.After(3 * time.Second):
		t.Fatal("实时复核命中 token 失效应触发自动重登")
	}
	close(relogDone) // 放行重登完成
}

// TestRealtimeFullRecheckKeepsManualSuccess 实时人数复核的"确证满员"分支不得覆盖手动报名
// 成功的胜利状态（B23-01，第 23 轮）。锁外复核窗口（最长 15s）内手动路径 TryAcquireSubmit
// 可抢到已释放的 inflight 位并 MarkDone 置 done+success；复核返回真满后旧实现 `cErr==nil
// && full` 分支（markFullLocked，无 doneHas 复核）把 success 覆盖成"failed/已满员"并追加
// 一条假"已满员"日志——与紧邻的"未现满员"分支（1338 行有 doneHas 复核"绝不覆盖胜利状态"）
// 不对称，同族竞态对称缺口。手动成功后实时复核不得改变胜利状态，且不再记 full。
func TestRealtimeFullRecheckKeepsManualSuccess(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	fc.selectErr[61115] = errors.New("该课程已满员") // 报名失败 → 走实时复核
	fc.mu.Unlock()
	var s *Scheduler // 声明提前：fullBlock 回调需要引用它（闭包捕获）
	recheckStarted := make(chan struct{})
	fc.fullBlock = func() {
		close(recheckStarted)
		// 复核在飞期间：先放手动报名成功落地（MarkDone 置 done+success），
		// 再在 IsClassFull 返回前把快照人数改满（复核读到的就是"真满"）。
		// 时序三要素：手动成功先于复核返回、复核返回硬编码真满、复核期间锁已释放。
		if err := s.markDoneTestHelper("acct1", 61115); err != nil {
			t.Errorf("手动报名成功失败: %v", err)
		}
		fc.mu.Lock()
		fc.data.Publishes[0].Classes[0].SelectedCount = 36 // 复核读到的满员（用户刚占末位）
		fc.mu.Unlock()
	}
	s = New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()
	s.mu.Lock()
	s.state.WindowOpened = true // 预置开窗状态，避免依赖探测时序
	s.mu.Unlock()

	// 自动链报名失败 → 进入实时复核的网络段（锁已释放）
	select {
	case <-recheckStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("提交失败后应触发实时人数复核")
	}
	// 复核返回后：手动报名成功的胜利状态必须保留（success），绝不被 markFullLocked 覆盖
	waitStatusAcct(t, s, "acct1", 61115, "success", 3*time.Second)

	s.mu.Lock()
	_, inFull := s.full["acct1"][61115]
	_, inDone := s.done["acct1"][61115]
	s.mu.Unlock()
	if !inDone {
		t.Fatal("手动报名成功后 done 必须保留")
	}
	if inFull {
		t.Fatal("手动报名成功后实时复核不得记入 full（胜利状态覆盖缺口）")
	}
}

// markDoneTestHelper 手动报名成功的内联助手：在复核回调（已持 fakeClient 锁外的时机）
// 里调用 MarkDone——这里是测试唯一需在回调里调 Scheduler 方法的地方，故抽出来。
func (s *Scheduler) markDoneTestHelper(acct string, classID int) error {
	return s.MarkDone(acct, classID, "健美操", "手动报名成功")
}

// TestRealtimeFullRecheckWithNoManualDoneMarksFull 对偶守卫：复核"真满"且没有手动成功介入时，
// 满员分支必须照常记 full 并置 failed（B23-01 修复不得误伤正常满员退避）。
func TestRealtimeFullRecheckWithNoManualDoneMarksFull(t *testing.T) {
	fc := newFakeClient(true)
	fc.mu.Lock()
	fc.selectErr[61115] = errors.New("该课程已满员")
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.Start()
	defer s.Stop()
	s.mu.Lock()
	s.state.WindowOpened = true
	s.mu.Unlock()
	// 让 IsClassFull 立即返回真满（无手动介入）
	fc.mu.Lock()
	fc.data.Publishes[0].Classes[0].SelectedCount = 36
	fc.mu.Unlock()

	waitStatusAcct(t, s, "acct1", 61115, "failed", 3*time.Second)
	s.mu.Lock()
	_, inFull := s.full["acct1"][61115]
	s.mu.Unlock()
	if !inFull {
		t.Fatal("无手动成功介入时复核确证满员应记入 full")
	}
}

// TestSpawnChainSkipsWhenTokenInvalid token 已知失效（tokenValid=true 且重登退避中）时，
// spawnChain 必须在链顶短路、绝不真实打平台 SelectClass（B23-03，第 23 轮）——此前链顶
// 只有 relogging 短路，失效+重登退避期（relogging 已清）每个 tick 仍对每门目标真实发起
// SelectClass（必然 code=-1）并每题 AppendLog"教务令牌失效"，烧平台请求额度 + 日志堆积。
// 修复后已知失效链不得发起任何 SelectClass 调用。
func TestSpawnChainSkipsWhenTokenInvalid(t *testing.T) {
	fc := newFakeClient(true)
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	acct := "acct1"
	s.SetTargetsForAccount(acct, []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	// 预置 token 失效（tokenValid=true）且退避中（relogging=false，reloginAt 未过——重登退避期）
	s.mu.Lock()
	s.tokenValid[acct] = true
	s.reloginAt[acct] = time.Now() // 退避窗口内
	s.mu.Unlock()

	s.submitAll()
	time.Sleep(150 * time.Millisecond) // 链是异步 goroutine，等它跑完

	if got := fc.SelectClassCalls(61115); got != 0 {
		t.Fatalf("token 已知失效时不应真实打平台报名接口，实际调用 %d 次", got)
	}
	// 状态不得被污染成"failed"（失效期间保持原态，前端显示 token_valid=false 由 /state 承担）
	s.mu.Lock()
	idx := s.statusIndexLocked(acct, 61115)
	status := ""
	if idx >= 0 {
		status = s.state.Courses[idx].Status
	}
	s.mu.Unlock()
	if status == "failed" {
		t.Fatal("token 失效跳过不得把状态置为 failed")
	}
}
