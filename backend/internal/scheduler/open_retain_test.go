package scheduler

import (
	"sync"
	"testing"
	"time"
)

// 窗口关闭后开放时间保留契约（主人反馈：关闭≠时间消失）。
// 平台 beginTimes 是"已训示的开窗时间"——窗口关闭后探测返回空快照，
// 识别槽必须保留（倒计时归零/显示已结束），绝不删除变"未知"。
// 修复前：probe()/ProbeForAccount 空快照分支 delete 识别槽，重探后 open_time 消失。
func TestOpenTimeRetainedAfterWindowClosed(t *testing.T) {
	fc := newFakeClient(false)
	fc.data.BeginTimes = []int64{1789261200000} // 2026-09-13 09:00:00 +0800
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)
	s.SetOpenTimeFn(func() time.Time { return time.Time{} }) // 无管理员配置：识别是唯一来源

	// 开窗前探测：识别到开放时间
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("首探失败: %v", err)
	}
	if !s.StateForAccount("acct1").OpenTimeKnown {
		t.Fatal("前置：识别应成功")
	}

	// 窗口关闭：平台返回空快照（code:0 空 publishes，begin_times 同时清空）
	fc.data.Publishes = nil
	fc.data.BeginTimes = nil
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("关闭后探测失败: %v", err)
	}

	// 修复要求：开放时间必须保留（已训示过的事实），且 open_time_known 仍为 true
	st := s.StateForAccount("acct1")
	if !st.OpenTimeKnown {
		t.Fatal("窗口关闭后开放时间必须保留（open_time_known=true）——关闭≠时间消失")
	}
	want := "2026-09-13 09:00:00"
	if got := st.OpenTime.Format("2006-01-02 15:04:05"); got != want {
		t.Fatalf("窗口关闭后开放时间应为 %s，实际 %s", want, got)
	}
}

// 目标课程发布元数据持久化契约（主人反馈：窗口关闭后日期/发布名必须仍可显示）。
// 目标选定时后端把 publish_name/begin_date 持久化进 targets 表，重建课程状态时
// 透传给 CourseStatus——/state.courses 自带日期/发布名，前端分组不再依赖 /electives
// （窗口关闭后 /electives 空发布、映射丢失是"未知日期"根因）。
// 修复前：Target 只有 publish_id/class_id/course_name/priority，无发布名/日期。
func TestTargetPublishMetaPersisted(t *testing.T) {
	fs := &recordingStore{fakeStore: &fakeStore{}}
	fc := newFakeClient(false)
	fc.data.BeginTimes = []int64{1789261200000}
	s := New(&fakeAccts{c: fc}, fs, time.Time{}, time.Hour)

	// 前置：先探测一次，让 acctData 带上平台快照（fakeClient 内置发布 1 = "高二年体育"
	// 带 begin_date）。SetTargetsForAccount 的发布元数据补全依赖这份快照——窗口关闭后
	// 这份快照也会被解析清空，所以"选课那一刻"补全落库是唯一持久化机会窗。
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("前置探测失败: %v", err)
	}

	// 用户选课：目标带发布元数据（发布名/日期来自平台快照）
	s.SetTargetsForAccount("acct1", []Target{
		{
			PublishID:  1,
			ClassID:    61115,
			CourseName: "健美操",
			Priority:   0,
		},
	})

	// 修复要求 1：落库时 targets 行必须携带 publish_name/begin_date
	fs.mu.Lock()
	rows := append([]targetRow(nil), fs.targets...)
	fs.mu.Unlock()
	if len(rows) == 0 {
		t.Fatal("目标必须落库")
	}
	if rows[0].publishName != "高二年体育" {
		t.Fatalf("落库 publish_name 应为'高二年体育'，实际 %q", rows[0].publishName)
	}
	if rows[0].beginDate == "" {
		t.Fatal("落库 begin_date 不应为空（发布日期必须持久化）")
	}

	// 修复要求 2：CourseStatus 透传发布名/日期——前端 /state 分组零依赖 /electives
	st := s.StateForAccount("acct1")
	if len(st.Courses) != 1 {
		t.Fatalf("应重建 1 门课程状态，实际 %d", len(st.Courses))
	}
	c := st.Courses[0]
	if c.PublishName != "高二年体育" {
		t.Fatalf("CourseStatus.PublishName 应为'高二年体育'，实际 %q", c.PublishName)
	}
	if c.BeginDate == "" {
		t.Fatal("CourseStatus.BeginDate 不应为空")
	}
}

// targetRow 记录 store 侧落库的 targets 行（含发布元数据）。
type targetRow struct {
	publishName string
	beginDate   string
}

// recordingStore 扩展 fakeStore：记录 SetTargetsForAccount 落库的目标行。
type recordingStore struct {
	*fakeStore
	mu      sync.Mutex
	targets []targetRow
}

func (r *recordingStore) SetTargetsForAccount(acct string, targets []Target) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.targets = nil
	for _, t := range targets {
		r.targets = append(r.targets, targetRow{publishName: t.PublishName, beginDate: t.BeginDate})
	}
	return nil
}

func (r *recordingStore) LoadTargetsForAccount(acct string) ([]Target, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Target, 0, len(r.targets))
	for i, row := range r.targets {
		out = append(out, Target{
			PublishID:  i + 1,
			ClassID:    61115 + i,
			CourseName: "课程",
			Priority:   i,
			PublishName: row.publishName,
			BeginDate:  row.beginDate,
		})
	}
	return out, nil
}

// 断言辅助：targets 行数量
func (r *recordingStore) targetCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.targets)
}
