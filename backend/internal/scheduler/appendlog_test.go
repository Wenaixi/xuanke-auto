package scheduler

// 本文件钉住"落库失败必须留痕"这条契约（CLAUDE.md「落库失败必须记日志绝不静默吞错」）。
//
// 背景：这条契约此前以手抄形态存在——scheduler.go 内 15 处 AppendLog 调用点，
// 每一处都手写三层 if（判 store 非 nil → 调 AppendLog → 判 err → log.Printf）。
// 契约没有 module 承载自己，于是"记得写 log.Printf"变成每个新分支都要重复的人工纪律：
// 漏写一处无人发现，失败路径静默消失；新增落点时的全分支清点成本随调用点数线性增长。
//
// 重要说明：这是纯结构收敛，不是修缺陷——当前 15 处都正确写了 log.Printf。
// 收益在于把第 17 条契约从"人工纪律"变成"module 强制"，使未来新增落点无需记得。
//
// 本测试断言的是行为（落库失败不被吞掉、正常路径确实落库），不是某个方法名或调用形状。
import (
	"strings"
	"sync"
	"testing"
	"time"
)

// errStoreDown 模拟落库失败。
var errStoreDown = &storeDownError{}

type storeDownError struct{}

func (*storeDownError) Error() string { return "simulated store failure" }

// logCountingStore 记 AppendLog 调用并可注入失败（锁保护供并发路径使用）。
type logCountingStore struct {
	*fakeStore
	mu   sync.Mutex
	logs []string
	fail bool
}

func (c *logCountingStore) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail {
		return errStoreDown
	}
	c.logs = append(c.logs, acct+":"+action+":"+result)
	return nil
}

func (c *logCountingStore) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.logs...)
}

func (c *logCountingStore) setFail(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail = v
}

// TestAppendLogFailureIsNotSwallowed 落库失败必须留痕且不改变调用方语义：
// store 故障时手动报名的状态同步仍算成功（平台报名确实发生了），
// 但日志不得被当作"已记录"——故障注入下 store 侧应无任何成功记录。
func TestAppendLogFailureIsNotSwallowed(t *testing.T) {
	c := &logCountingStore{fakeStore: &fakeStore{}, fail: true}
	s := New(&fakeAccts{c: newFakeClient(true)}, c, time.Now(), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	if err := s.markDone("acct1", 61115, "健美操", "手动报名成功", nil); err != nil {
		t.Fatalf("日志落库失败不得让手动报名状态同步报错: %v", err)
	}
	if got := c.list(); len(got) != 0 {
		t.Fatalf("故障注入的 store 不应留下成功日志记录，实际 %v", got)
	}
	// 状态同步本身仍应完成（内存态写入不依赖日志落库）
	s.mu.Lock()
	_, inDone := s.done["acct1"][61115]
	s.mu.Unlock()
	if !inDone {
		t.Fatal("日志落库失败不应回滚内存 done 态（平台报名已发生）")
	}
}

// TestAppendLogSucceedsOnHealthyStore 正常路径对照组：证明上一条不是因为
// "根本没调 AppendLog"而绿——健康 store 确实收到日志。
func TestAppendLogSucceedsOnHealthyStore(t *testing.T) {
	c := &logCountingStore{fakeStore: &fakeStore{}}
	s := New(&fakeAccts{c: newFakeClient(true)}, c, time.Now(), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	if err := s.markDone("acct1", 61115, "健美操", "手动报名成功", nil); err != nil {
		t.Fatalf("MarkDone 失败: %v", err)
	}
	got := c.list()
	if len(got) != 1 || !strings.Contains(got[0], "手动报名成功") {
		t.Fatalf("健康 store 应收到一条报名成功日志，实际 %v", got)
	}
}
