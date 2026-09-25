package scheduler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// failTargetsStore 带失败开关的目标落库假存储：验证 SetTargetsForAccount
// 落库失败必须 error 上抛（C1 契约：持久化失败绝不静默吞错，前端要拿到真实失败）。
// 与既有 failStore 不同：failStore 只拦 SaveSuccess/SaveRefused（重登/手动路径用），
// 这里拦 SetTargetsForAccount 本身。
type failTargetsStore struct {
	*fakeStore
	fail bool // 置 true 后 SetTargetsForAccount 返回错误，模拟磁盘满/IO 故障
}

func (f *failTargetsStore) SetTargetsForAccount(acct string, targets []Target) error {
	if f.fail {
		return errors.New("sqlite disk full")
	}
	return f.fakeStore.SetTargetsForAccount(acct, targets)
}

// TestSetTargetsForAccountPropagatesStoreError 落库失败必须 error 上抛（C1-1 TDD 红灯）。
// 背景：scheduler.SetTargetsForAccount 当前无返回、内部只 log（scheduler.go:476-478），
// handler 预写成功 + 调度器写失败时前端拿"已保存"而库内是缺元数据版本——零吞错契约缺口。
func TestSetTargetsForAccountPropagatesStoreError(t *testing.T) {
	fc := newFakeClient(true)
	accts := &fakeAccts{c: fc}
	store := &failTargetsStore{fakeStore: &fakeStore{}}
	s := New(accts, store, time.Time{}, time.Hour)

	store.fail = true
	err := s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
	})
	if err == nil {
		t.Fatal("落库失败必须 error 上抛，当前实现返回 nil（静默吞错）")
	}
	if !strings.Contains(err.Error(), "sqlite disk full") {
		t.Fatalf("期望传播 store 落库错误原文，got %q", err.Error())
	}
}

// TestSetTargetsForAccountSuccessClearsRefusedOnly 重设目标成功路径契约（C1-1 绿灯守卫）：
// 只清 refused、绝不清 done/full/rateLimited/inflight（决策 6）。落库成功返回 nil。
func TestSetTargetsForAccountSuccessClearsRefusedOnly(t *testing.T) {
	fc := newFakeClient(true)
	accts := &fakeAccts{c: fc}
	store := &fakeStore{}
	s := New(accts, store, time.Time{}, time.Hour)

	// 前置：先探测一次，让 acctData 带上平台快照（与 open_retain_test.go 同源——
	// enrichTargetPubMetaLocked 依赖快照帧补全发布元数据）
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("前置探测失败: %v", err)
	}

	// 预置"必须保留"的状态
	s.done["acct1"] = map[int]bool{61115: true}
	s.full["acct1"] = map[int]bool{61115: true}
	s.rateLimited["acct1"] = map[int]time.Time{61115: time.Now().Add(time.Minute)}
	s.inflight["acct1"] = map[int]bool{61115: true}

	err := s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
	})
	if err != nil {
		t.Fatalf("落库成功不应返回错误，got %v", err)
	}
	if !s.doneHas("acct1", 61115) {
		t.Fatal("done 是跨目标持久历史事实，重设目标绝不清 done（决策 6）")
	}
	if !s.fullHas("acct1", 61115) {
		t.Fatal("full 是真实满员状态，重设目标绝不清 full（决策 6）")
	}
	if !s.inflightHas("acct1", 61115) {
		t.Fatal("inflight 防并发双发包，重设目标绝不清 inflight（决策 6）")
	}
	if s.refusedHas("acct1", 61115) {
		t.Fatal("refused 是唯一应被重设目标清空的状态（决策 6）")
	}
	// 发布元数据补全契约：落库目标必须带 publish_name/begin_date（关闭≠数据消失）
	if len(s.acctTargets["acct1"]) != 1 || s.acctTargets["acct1"][0].PublishName != "高二年体育" {
		t.Fatalf("目标应经 enrich 补全发布元数据，got %+v", s.acctTargets["acct1"])
	}
}

// TestSetTargetsForAccountNilStoreSkipsPersist store=nil 时跳过落库不 panic、成功返回 nil。
func TestSetTargetsForAccountNilStoreSkipsPersist(t *testing.T) {
	fc := newFakeClient(true)
	accts := &fakeAccts{c: fc}
	s := New(accts, nil, time.Time{}, time.Hour)
	err := s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
	})
	if err != nil {
		t.Fatalf("store=nil 应静默跳过落库返回 nil，got %v", err)
	}
}

// 确保 zhidao 包被引用（夹具依赖），避免误删 import 时编译错。
var _ = zhidao.ElectivesData{}
