package scheduler

// 本文件是本轮架构核实（候选 1）的红测试：证明 ManualSelect/ManualExit 的写回侧
// 只做"账号名存在"复核，缺 spawnChain 那道 sameClientFor 指针身份比对。
//
// 场景：手动报名发起 → SelectClass 网络往返（最长 15s）→ 期间账号被删除并同名重建
// （注册表换成新 *zhidao.Client，指针不同但账号名仍存在）→ 报名成功返回 → MarkDone
// 只判 ClientFor 存在性（ok，新身份确实存在）→ 把 success 写进新身份 + 落库 success 行。
//
// 后果与 spawnChain 第六分支（自动链实时复核）防的完全同源：重启后 RestoreDone 把
// 重建账号恢复成"已报名成功"假状态。而 TestDeletedAccountManualInFlightDropsState
// 只覆盖"账号被删"（ClientFor 不存在）形态，未覆盖"同名重建"（ClientFor 仍 ok）形态。
//
// 修复前：红——新身份的 successRows == 1。
// 修复后：绿——successRows == 0，且新身份 done 内存态为空。
import (
	"testing"
	"time"
)

func TestManualSelectSameNameRebuiltDropsSuccess(t *testing.T) {
	oldClient := newFakeClient(true)
	fa := &fakeAccts{c: oldClient, perAccount: map[string]*fakeClient{"acct1": oldClient}}
	store := &fakeStore{}
	s := New(fa, store, time.Now(), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	// 手动报名在 SelectClass 网络往返期间阻塞
	entered := make(chan struct{})
	release := make(chan struct{})
	oldClient.mu.Lock()
	oldClient.selectBlock = func() {
		close(entered)
		<-release
	}
	oldClient.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := s.ManualSelect("acct1", 61115, "健美操")
		done <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("手动报名未进入网络往返段")
	}

	// 往返期间：账号被删，随后同名重建——perAccount 换成新客户端（新身份）。
	// 新身份注册表里确实存在（ClientFor ok），但指针与发起时的旧客户端不同。
	newClient := newFakeClient(true)
	fa.mu.Lock()
	fa.perAccount["acct1"] = newClient
	fa.mu.Unlock()

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("手动报名应成功返回: %v", err)
	}

	// 陈旧的手动请求不得把 success 写进同名重建后的新身份
	store.mu.Lock()
	rows := store.successRows["acct1\x0061115"]
	store.mu.Unlock()
	if rows != 0 {
		t.Fatalf("删号同名重建后，陈旧手动报名不得把 success 行写进新身份（重启恢复成假成功），实际 %d 行", rows)
	}
	s.mu.Lock()
	_, inDone := s.done["acct1"][61115]
	s.mu.Unlock()
	if inDone {
		t.Fatal("删号同名重建后，陈旧手动报名不得写新身份的 done 内存态")
	}
}

// TestManualExitSameNameRebuiltDropsRefused 与上条同源：手动退选路径的 RemoveDone
// 同样只做存在性复核，删号同名重建后会把 refused 写进新身份——重启后 RestoreRefused
// 把该课恢复成"已手动退选"，自动引擎对一门用户从未退选的课永久跳过。
func TestManualExitSameNameRebuiltDropsRefused(t *testing.T) {
	oldClient := newFakeClient(true)
	fa := &fakeAccts{c: oldClient, perAccount: map[string]*fakeClient{"acct1": oldClient}}
	store := &fakeStore{}
	s := New(fa, store, time.Now(), time.Hour)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})

	// 预置该课已报名成功（退选路径的真实前提）
	if err := s.markDone("acct1", 61115, "健美操", "手动报名成功", nil); err != nil {
		t.Fatalf("预置 MarkDone 失败: %v", err)
	}
	store.mu.Lock()
	store.refusedRows = map[string]int{}
	store.successRows = map[string]int{}
	store.mu.Unlock()

	// 手动退选在 ExitClass 网络往返期间阻塞（与 SelectClass 场景同款时序编排）
	entered := make(chan struct{})
	release := make(chan struct{})
	oldClient.mu.Lock()
	oldClient.exitBlock = func() {
		close(entered)
		<-release
	}
	oldClient.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := s.ManualExit("acct1", 61115)
		done <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("手动退选未进入网络往返段")
	}

	// 往返期间：删号后同名重建，注册表换成新客户端（新身份存在但指针不同）
	newClient := newFakeClient(true)
	fa.mu.Lock()
	fa.perAccount["acct1"] = newClient
	fa.mu.Unlock()

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("手动退选应成功返回: %v", err)
	}

	// 陈旧的退选不得把 refused 写进同名重建后的新身份
	store.mu.Lock()
	rows := store.refusedRows["acct1\x0061115"]
	store.mu.Unlock()
	if rows != 0 {
		t.Fatalf("删号同名重建后，陈旧手动退选不得把 refused 行写进新身份（重启后自动引擎永久跳过该课），实际 %d 行", rows)
	}
}
