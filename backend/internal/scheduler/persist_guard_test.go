package scheduler

import (
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// 落库失败必须留痕、且绝不回滚内存态——把记忆库第 17 条从人工纪律变成可执行断言。
//
// 背景：19 个落库点各自手写 if err != nil {...}，新增落点漏判 err 时编译器不
// 报错、既有测试也不一定覆盖。failStore 夹具此前只拦三个写方法
// （SaveSuccess/SaveRefused/DeleteRefused），其余落库点失败时走 fakeStore 的
// 成功路径，"失败必记日志"在那几处零覆盖。本文件补齐这层覆盖。
//
// 刻意不做「落库点统一分类」重构：四处非记日志点（上抛 / 上抛但半删态）各有
// 场景化理由（内存是否已正确、用户能否感知分叉），统一会删掉真实信息。

// failAllStore 让全部写方法返回错误，模拟磁盘满/IO 故障。
type failAllStore struct {
	*fakeStore
}

func (f *failAllStore) SaveSuccess(acct string, classID int) error {
	return errors.New("sqlite disk full")
}
func (f *failAllStore) SaveRefused(acct string, classID int) error {
	return errors.New("sqlite disk full")
}
func (f *failAllStore) DeleteRefused(acct string) error {
	return errors.New("sqlite disk full")
}
func (f *failAllStore) DeleteRefusedClass(acct string, classID int) error {
	return errors.New("sqlite disk full")
}
func (f *failAllStore) DeleteSuccess(acct string, classID int) error {
	return errors.New("sqlite disk full")
}
func (f *failAllStore) UpdateIDToken(acct, idToken string) error {
	return errors.New("sqlite disk full")
}
func (f *failAllStore) SetTargetsForAccount(acct string, targets []Target) error {
	return errors.New("sqlite disk full")
}

// logFailingStore 只让审计日志表失败——模拟"日志表写不进去"这一局部故障。
type logFailingStore struct {
	*fakeStore
}

func (f *logFailingStore) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	return errors.New("sqlite disk full")
}

// captureLogs 把标准 logger 重定向到加锁缓冲区，返回读取函数。
func captureLogs(t *testing.T) func() string {
	t.Helper()
	old := log.Writer()
	t.Cleanup(func() { log.SetOutput(old) })
	var mu sync.Mutex
	var b strings.Builder
	log.SetOutput(&lockedWriter{mu: &mu, b: &b})
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return b.String()
	}
}

// TestMarkDoneLogsFailuresAcrossTables markDone 的多个落库点（成功记录 / 退选清理
// / 审计日志）任一失败都必须留痕——全表失败注入，确认日志出现。
func TestMarkDoneLogsFailuresAcrossTables(t *testing.T) {
	logs := captureLogs(t)
	s := New(&fakeAccts{c: newFakeClient(true)}, &failAllStore{fakeStore: &fakeStore{}},
		time.Now().Add(-time.Hour), time.Hour)
	s.mu.Lock()
	s.state.Courses = append(s.state.Courses, CourseStatus{
		Account: "acct1", ClassID: 1, CourseName: "健美操",
	})
	s.mu.Unlock()
	// origin 传 nil = 只判账号存在性（夹具语义，生产路径传真客户端指针）
	if err := s.markDone("acct1", 1, "健美操", "选课成功", nil); err != nil {
		t.Fatalf("markDone 不应因落库失败返回错误（平台动作已完成）：%v", err)
	}
	if out := logs(); !strings.Contains(out, "落库失败") {
		t.Fatalf("落库失败必须留痕，实际输出: %q", out)
	}
}

// TestFailAllStoreCoversEveryWriteMethod failAllStore 必须真的让每个写方法失败，
// 否则「失败必记日志」在某张表上仍是零覆盖的假象。
func TestFailAllStoreCoversEveryWriteMethod(t *testing.T) {
	f := &failAllStore{fakeStore: &fakeStore{}}
	cases := []struct {
		name string
		call func() error
	}{
		{"SaveSuccess", func() error { return f.SaveSuccess("a", 1) }},
		{"SaveRefused", func() error { return f.SaveRefused("a", 1) }},
		{"DeleteRefused", func() error { return f.DeleteRefused("a") }},
		{"DeleteRefusedClass", func() error { return f.DeleteRefusedClass("a", 1) }},
		{"DeleteSuccess", func() error { return f.DeleteSuccess("a", 1) }},
		{"UpdateIDToken", func() error { return f.UpdateIDToken("a", "tok") }},
		{"SetTargetsForAccount", func() error { return f.SetTargetsForAccount("a", nil) }},
	}
	for _, tc := range cases {
		if err := tc.call(); err == nil {
			t.Errorf("failAllStore 未拦 %s：该落库点的失败路径无法被测试触达", tc.name)
		}
	}
}

// TestAuditLogFailureKeepsMemoryState 审计日志落库失败绝不阻断成功路径：
// 平台报名已成事实，回滚内存比不落库更糟（用户会看到"报名成功"却要重选）。
func TestAuditLogFailureKeepsMemoryState(t *testing.T) {
	logs := captureLogs(t)
	s := New(&fakeAccts{c: newFakeClient(true)}, &logFailingStore{fakeStore: &fakeStore{}},
		time.Now().Add(-time.Hour), time.Hour)
	s.mu.Lock()
	s.state.Courses = append(s.state.Courses, CourseStatus{
		Account: "acct1", ClassID: 1, CourseName: "健美操",
	})
	s.mu.Unlock()
	if err := s.markDone("acct1", 1, "健美操", "选课成功", nil); err != nil {
		t.Fatalf("审计日志落库失败不得让 markDone 返回错误（平台动作已完成）：%v", err)
	}
	s.mu.Lock()
	_, done := s.done["acct1"][1]
	s.mu.Unlock()
	if !done {
		t.Fatal("审计日志落库失败时成功记录仍必须进内存（不可回滚已发生的平台动作）")
	}
	if !strings.Contains(logs(), "落库失败") {
		t.Error("审计日志落库失败必须留痕")
	}
}

// TestTargetPersistFailurePropagates 目标持久化失败必须上抛（与其它落库点相反）：
// 库内缺元数据版本而前端已显示"已保存"，属用户可感知的承诺分叉，必须让调用方
// 知道。这条是"落库失败两种处理方式"里"上抛"那一档的唯一守护。
func TestTargetPersistFailurePropagates(t *testing.T) {
	s := New(&fakeAccts{c: newFakeClient(true)}, &failAllStore{fakeStore: &fakeStore{}},
		time.Now().Add(-time.Hour), time.Hour)
	if err := s.SetTargetsForAccount("acct1", []Target{{ClassID: 61115, PublishID: 1, CourseName: "健美操"}}); err == nil {
		t.Fatal("目标持久化失败必须上抛（前端会显示已保存而库内缺失，属承诺分叉）")
	}
}
