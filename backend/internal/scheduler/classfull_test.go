package scheduler

import (
	"testing"
	"time"

	"xuanke-auto/backend/internal/upstream"
)

// newClassFullSched 构造带指定课程快照的调度器（课程满员由 MaxCount/SelectedCount 派生）。
// 返回已写入专属帧的调度器，调用方无需持锁。
func newClassFullSched(t *testing.T, classes []upstream.Class) *Scheduler {
	t.Helper()
	fc := newFakeClient(true)
	fc.data.Publishes[0].Classes = classes
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.mu.Lock()
	s.acctData["acct1"] = fc.data
	s.acctDataAt["acct1"] = time.Now()
	s.rebuildCoursesForAccountLocked("acct1", []Target{
		{ClassID: 61115, PublishID: 1, CourseName: "健美操"},
	})
	s.mu.Unlock()
	return s
}

// TestRebuildCoursesCarriesClassFull 钉死「满员事实随课程状态下发」契约。
//
// 背景：Dashboard 曾用 result.includes("已满员") 匹配中文文案判满员，而
// 契约要求满员判据单源（快照 ClassFull 派生字段，Select 路由早已改读
// class_full）。本测试确保 CourseStatus 直接携带满员布尔，使前端无需
// 匹配任何中文文案，也不必反查嵌套快照结构。
//
// 三态覆盖：名额未公布（MaxCount=0）/ 未满 / 已满。
func TestRebuildCoursesCarriesClassFull(t *testing.T) {
	cases := []struct {
		name          string
		selected, max int
		want          bool
	}{
		{"名额未公布", 0, 0, false},
		{"未满", 3, 36, false},
		{"已满", 36, 36, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newClassFullSched(t, []upstream.Class{{
				ID: 61115, CourseName: "健美操",
				SelectedCount: tc.selected, MaxCount: tc.max, ClassFull: tc.want,
			}})
			courses := s.StateForAccount("acct1").Courses
			if len(courses) != 1 {
				t.Fatalf("状态行数应为 1，实际 %d", len(courses))
			}
			if courses[0].ClassFull != tc.want {
				t.Fatalf("ClassFull 期望 %v，实际 %v", tc.want, courses[0].ClassFull)
			}
		})
	}
}

// TestRebuildCoursesClassFullFalseWhenSnapshotEmpty 窗口关闭后快照为空，
// 满员事实无从算起——必须退化为 false（不显满员徽章），绝不能默认满员。
func TestRebuildCoursesClassFullFalseWhenSnapshotEmpty(t *testing.T) {
	s := New(&fakeAccts{c: newFakeClient(true)}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.mu.Lock()
	s.rebuildCoursesForAccountLocked("acct1", []Target{
		{ClassID: 99999, PublishID: 1, CourseName: "查不到的课"},
	})
	s.mu.Unlock()
	courses := s.StateForAccount("acct1").Courses
	if len(courses) != 1 {
		t.Fatalf("状态行数应为 1，实际 %d", len(courses))
	}
	if courses[0].ClassFull {
		t.Fatal("快照为空时 ClassFull 必须为 false（退化为不显满员），实际 true")
	}
}

// TestMarkFullAndReleaseSyncClassFull 钉死满员运行时写入与解除时的状态同步：
// markFullLocked 判满员后状态行必须带 ClassFull=true（前端显满员徽章），
// releaseFullIfFreedLocked 解封后必须回落 false（徽章消失）——否则手动
// 报名触发的满员/捡漏两条路径与 rebuild 路径不一致。
func TestMarkFullAndReleaseSyncClassFull(t *testing.T) {
	fc := newFakeClient(true)
	fc.data.Publishes[0].Classes = []upstream.Class{
		{ID: 61115, CourseName: "健美操", SelectedCount: 36, MaxCount: 36, ClassFull: true}, // 满员
	}
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.mu.Lock()
	s.acctData["acct1"] = fc.data
	s.acctDataAt["acct1"] = time.Now()
	s.rebuildCoursesForAccountLocked("acct1", []Target{
		{ClassID: 61115, PublishID: 1, CourseName: "健美操"},
	})
	s.mu.Unlock()

	if !s.StateForAccount("acct1").Courses[0].ClassFull {
		t.Fatal("前置：快照显示满员，rebuild 后 ClassFull 应为 true")
	}
	// 先经 markFullLocked 建立 full 标记（releaseFullIfFreedLocked 的首行守卫
	// 要求 full 已标记，否则直接早退——手动报名判满员的真实路径）。
	s.mu.Lock()
	s.markFullLocked("acct1", Target{ClassID: 61115, PublishID: 1, CourseName: "健美操"})
	s.mu.Unlock()
	if !s.StateForAccount("acct1").Courses[0].ClassFull {
		t.Fatal("markFullLocked 判满员后 ClassFull 必须为 true（前端应显满员徽章）")
	}
	// 判满员时状态行必须同时是 failed（徽章与失败态一致，不能一个亮一个不亮）
	if got := s.StateForAccount("acct1").Courses[0].Status; got != "failed" {
		t.Fatalf("markFullLocked 应同时置 failed 状态，实际 %q", got)
	}

	// 解除 full：快照显示有名额空余（其他同学退选）——解封判据读派生字段
	// ClassFull 而非数字（数字矛盾时以派生字段为准），故必须同时清掉它。
	fc.mu.Lock()
	fc.data.Publishes[0].Classes[0].SelectedCount = 10
	fc.data.Publishes[0].Classes[0].ClassFull = false
	fc.mu.Unlock()
	s.mu.Lock()
	s.releaseFullIfFreedLocked("acct1", 61115)
	s.mu.Unlock()
	if s.StateForAccount("acct1").Courses[0].ClassFull {
		t.Fatal("快照显示有名额后解封，ClassFull 必须回落 false（徽章应消失）")
	}

}

// TestClassFullRealtimeConstFalseWhenMaxCountZero 实时复核"永不确证满员"的负向钉：
// zhidao 档案 countList 不下发 maxCount（T1 已锁档案事实），MaxCount 恒 0 形态下
// classFullRealtime 必须恒返回 false——不挂 full、不置 failed，保留失败重试分支。
// 契约 131：正向恒真（MaxCount=0 恒 false）必须配负向断言，防未来误给实时接口
// 加判据或把 maxCount 当满员判据造成防轰炸语义破坏。
func TestClassFullRealtimeConstFalseWhenMaxCountZero(t *testing.T) {
	fc := newFakeClient(true)
	// 模拟 zhidao 档案形态：实时人数数据只有 selectedCount（MaxCount 恒 0）
	fc.mu.Lock()
	fc.data.Publishes[0].Classes[0] = upstream.Class{
		ID: 61115, CourseName: "健美操", SelectedCount: 30, MaxCount: 0, ClassFull: false,
	}
	fc.mu.Unlock()
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Now().Add(-time.Hour), time.Hour)
	s.mu.Lock()
	s.acctData["acct1"] = fc.data
	s.acctDataAt["acct1"] = time.Now()
	s.rebuildCoursesForAccountLocked("acct1", []Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}})
	s.mu.Unlock()

	full, err := s.classFullRealtime("acct1", 61115)
	if err != nil {
		t.Fatalf("实时复核不应报错: %v", err)
	}
	if full {
		t.Fatal("MaxCount 恒 0 形态下实时复核必须永不确证满员（zhidao 档案事实）——否则满员退避/防轰炸语义被破坏")
	}
	// 判据 false 时不挂 full、不置 failed（保留失败重试分支）
	if s.fullHas("acct1", 61115) {
		t.Fatal("实时复核判据 false 时不得记 full")
	}
	if got := s.StateForAccount("acct1").Courses[0].Status; got != "pending" {
		t.Fatalf("实时复核判据 false 时状态行不得置 failed，实际 %q", got)
	}
}
