package scheduler

import (
	"testing"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// TestProbeDeletedThenRebuiltSameNameDropsSnapshot（回归钉）：
// 删号 + 同名重建 + 在飞探测同帧时，ProbeForAccount 的旧链不得把快照写进
// 重建后的新账号内存条目（acctData/acctDataAt），也不得覆盖其识别槽。
// 旧实现：FindElectives 网络往返返回后无任何复核直接写 map——左链（旧身份）与
// 右链（新身份）争夺同一 acctData[acct]，重建账号可能短暂看到旧年级课程。
// 修后：在网络往返返回后的回写段（持锁）复核 ClientFor 仍存在且指针身份一致。
func TestProbeDeletedThenRebuiltSameNameDropsSnapshot(t *testing.T) {
	oldFc := newFakeClient(true)
	newFc := newFakeClient(true)
	// 新身份返回不同数据（模拟换绑年级/新批次热更）+ 带 beginTimes 识别槽
	newFc.mu.Lock()
	newFc.data = &zhidao.ElectivesData{
		BeginTimes: []int64{1789261200000},
		Publishes:  []zhidao.Publish{{PublishID: 99}},
	}
	newFc.mu.Unlock()

	// 旧身份 FindElectives 阻塞：模拟旧链网络往返（删号竞态窗口）
	entered := make(chan struct{})
	release := make(chan struct{})
	oldFc.mu.Lock()
	oldFc.findBlock = func() {
		close(entered)
		<-release
	}
	oldFc.mu.Unlock()

	fa := &fakeAccts{c: oldFc, perAccount: map[string]*fakeClient{"acct1": oldFc}, removed: map[string]bool{}}
	s := New(fa, &fakeStore{}, time.Time{}, 10*time.Millisecond)
	// 预置识别槽（旧值，随后 PurgeAccount 清）
	s.mu.Lock()
	s.ws.setOpenTime("acct1", int64(1111111111111))
	s.mu.Unlock()

	// 旧链发起探测（goroutine）——进入 FindElectives 阻塞段
	done := make(chan struct{})
	var snapErr error
	go func() {
		defer close(done)
		_, snapErr = s.ProbeForAccount("acct1")
	}()

	// 等旧链进入网络段
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("旧链未进入 FindElectives 网络段")
	}

	// 网络往返期间：删号（removed + PurgeAccount 清全部 map）
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()
	s.PurgeAccount("acct1")

	// 同名重建：perAccount 换新客户端
	fa.mu.Lock()
	delete(fa.removed, "acct1")
	fa.perAccount["acct1"] = newFc
	fa.mu.Unlock()

	// 放行旧链网络往返 → 回写段必须判身份已变丢弃
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("旧链探测未返回")
	}
	if snapErr != nil {
		t.Fatalf("旧链探测返回错误（应正常返回数据仅丢弃回写）: %v", snapErr)
	}

	// 回写被拦：acctData 应仍空（PurgeAccount 后无人写回）、识别槽仍空
	s.mu.Lock()
	_, hasData := s.acctData["acct1"]
	_, hasAt := s.acctDataAt["acct1"]
	hasOpen := !s.ws.openTimeFor("acct1").IsZero()
	s.mu.Unlock()
	if hasData || hasAt || hasOpen {
		t.Fatal("旧身份探测回写应被丢弃（身份已变），acctData/acctDataAt/openTimeDetected 必须全空")
	}

	// 新身份随后探测正常落新帧（数据 99 + 识别槽被新值覆盖）
	data, err := s.ProbeForAccount("acct1")
	if err != nil {
		t.Fatalf("新身份探测失败: %v", err)
	}
	if data == nil || len(data.Publishes) == 0 || data.Publishes[0].PublishID != 99 {
		t.Fatalf("新身份探测返回异常: %+v", data)
	}
	s.mu.Lock()
	snap := s.acctData["acct1"]
	openNow := s.ws.openTimeFor("acct1").UnixMilli()
	s.mu.Unlock()
	if snap == nil || len(snap.Publishes) == 0 || snap.Publishes[0].PublishID != 99 {
		t.Fatalf("acctData 应落新身份帧（99），实际: %+v", snap)
	}
	if openNow != int64(1789261200000) {
		t.Fatalf("识别槽应被新身份 beginTimes 覆盖，实际 %d", openNow)
	}
}

// TestProbeForAccountDropsWriteWhenRemoved（已删分支）：
// 在飞探测返回后账号已删（ClientFor 不存在）——回写段必须整体放弃
// （不写 acctData/acctDataAt/识别槽），与 ClientFor 复核族"写回侧复核"对称。
// 触发路径：探测发起后（进入网络段）账号被删，返回后回写段判 ClientFor 不存在。
func TestProbeForAccountDropsWriteWhenRemoved(t *testing.T) {
	fc := newFakeClient(true)
	entered := make(chan struct{})
	release := make(chan struct{})
	fc.mu.Lock()
	fc.findBlock = func() {
		close(entered)
		<-release
	}
	fc.mu.Unlock()

	fa := &fakeAccts{c: fc, removed: map[string]bool{}}
	s := New(fa, &fakeStore{}, time.Time{}, 10*time.Millisecond)
	s.SetTargetsForAccount("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}})
	s.mu.Lock()
	s.ws.setOpenTime("acct1", int64(1111111111111))
	s.mu.Unlock()

	// 旧链发起探测（goroutine）——进入 FindElectives 阻塞段
	done := make(chan struct{})
	var snapErr error
	go func() {
		defer close(done)
		_, snapErr = s.ProbeForAccount("acct1")
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("探测未进入网络段")
	}

	// 网络往返期间删号（PurgeAccount 清全部 map）
	fa.mu.Lock()
	fa.removed["acct1"] = true
	fa.mu.Unlock()
	s.PurgeAccount("acct1")

	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("探测未返回")
	}
	if snapErr != nil {
		t.Fatalf("探测返回错误（应正常返回仅丢弃回写）: %v", snapErr)
	}
	// 回写被拦：全空
	s.mu.Lock()
	_, hasData := s.acctData["acct1"]
	_, hasAt := s.acctDataAt["acct1"]
	hasOpen := !s.ws.openTimeFor("acct1").IsZero()
	s.mu.Unlock()
	if hasData || hasAt || hasOpen {
		t.Fatal("已删账号探测应整体放弃写回（acctData/acctDataAt/openTimeDetected 全空）")
	}
}

// TestProbeChainSameClientIdentity（身份不变正常路径对偶）：
// 同名重建后新身份的探测正常落新帧、识别槽被新 beginTimes 覆盖——
// 同身份复核绝不误伤正常路径（无删号时探测照常回写）。
func TestProbeChainSameClientIdentity(t *testing.T) {
	newFc := newFakeClient(true)
	newFc.mu.Lock()
	newFc.data = &zhidao.ElectivesData{
		BeginTimes: []int64{1789261200000},
		Publishes:  []zhidao.Publish{{PublishID: 77}},
	}
	newFc.mu.Unlock()

	fa := &fakeAccts{c: newFc, perAccount: map[string]*fakeClient{"acct1": newFc}, removed: map[string]bool{}}
	s := New(fa, &fakeStore{}, time.Time{}, 10*time.Millisecond)
	s.mu.Lock()
	s.ws.setOpenTime("acct1", int64(1111111111111))
	s.mu.Unlock()

	data, err := s.ProbeForAccount("acct1")
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	if data == nil || len(data.Publishes) == 0 || data.Publishes[0].PublishID != 77 {
		t.Fatalf("探测返回异常: %+v", data)
	}
	s.mu.Lock()
	snap := s.acctData["acct1"]
	openNow := s.ws.openTimeFor("acct1").UnixMilli()
	s.mu.Unlock()
	if snap == nil || len(snap.Publishes) == 0 || snap.Publishes[0].PublishID != 77 {
		t.Fatalf("acctData 应落新身份帧（77），实际: %+v", snap)
	}
	if openNow != int64(1789261200000) {
		t.Fatalf("识别槽应被 beginTimes 覆盖，实际 %d", openNow)
	}
}
