package scheduler

import (
	"sync"
	"testing"
	"time"
)

// 窗口关闭后识别槽保留契约（主人反馈：关闭≠时间消失）。
// 平台 beginTimes 是"已训示的开窗时间"——窗口关闭后探测返回空快照，
// 识别槽必须保留（底层 openTimeDetected 不清空），绝不删除变"未知"。
// 但"识别过期"语义下 open_time_known 反映识别值是否仍有效：
// 识别值已落入过去（窗口已关闭）→ open_time_known=false，前端显示"未识别"，
// 绝不把 5 天前的旧值继续当开放时间挂出来。
// 日期/发布名等窗口关闭后仍可展示的元数据由目标发布元数据持久化承载（见下一测试）。
func TestOpenTimeRetainedAfterWindowClosed(t *testing.T) {
	fc := newFakeClient(false)
	// 首探识别一个"未来"开窗点（T0+2h），随后用 SetClockOffsetForTest 把对齐时钟
	// 推进过该识别值（T0+3h）——模拟真实时间流逝：窗口开启又关闭后识别值已落入过去。
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)

	// 开窗前探测：识别到开放时间
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("首探失败: %v", err)
	}
	if !s.StateForAccount("acct1").OpenTimeKnown {
		t.Fatal("前置：识别应成功")
	}

	// 时间推进到识别值之后（开窗批次已结束）
	s.SetClockOffsetForTest(3 * time.Hour)

	// 窗口关闭：平台返回空快照（code:0 空 publishes，begin_times 同时清空）
	fc.data.Publishes = nil
	fc.data.BeginTimes = nil
	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("关闭后探测失败: %v", err)
	}

	// 识别槽必须保留（底层 map 不清空）——关闭≠时间消失
	s.mu.Lock()
	kept := s.openTimeDetected["acct1"]
	s.mu.Unlock()
	if kept == 0 {
		t.Fatal("窗口关闭后识别槽必须保留在 openTimeDetected 中（关闭≠时间消失）")
	}
	// 但识别值已过期（窗口已关闭、平台未再下发新 beginTimes）→ open_time_known=false：
	// 前端显示"未识别到开放时间"，绝不把过期旧值继续当开放时间挂出来。
	if st := s.StateForAccount("acct1"); st.OpenTimeKnown {
		t.Fatal("识别值已过期必须 open_time_known=false（不把旧值当开放时间）")
	}
	// 同时：openTimeForLocked 仍返回已识别的开窗时刻（识别过期只影响 known/展示，
	// 不截断为零值）——调度判定（tick 提交守卫的第二判据 !now.After(open)）依赖它
	// 放行"已到点"提交，窗口关闭形态由 WindowClosed 挂起而非零值守卫。
	if st := s.StateForAccount("acct1"); st.OpenTime.IsZero() {
		t.Fatal("识别过期只影响 open_time_known，open_time 仍应保留识别值（挂起/展示解耦）")
	}
}

// 目标课程发布元数据持久化契约（主人反馈：窗口关闭后日期/发布名必须仍可显示）。
// 目标选定时后端把 publish_name/begin_date 持久化进 targets 表，重建课程状态时
// 透传给 CourseStatus——/state.courses 自带日期/发布名，前端分组不再依赖 /electives
// （窗口关闭后 /electives 空发布、映射丢失是"未知日期"根因）。
// 修复前：Target 只有 publish_id/class_id/course_name/priority，无发布名/日期。
// 兜底补全：专属帧缺失/过期时回退全校帧 lastData——HTTP 直存目标不触发探测，
// 专属帧常空，靠全校帧（开窗前 30s 常态保鲜）补全。
func TestTargetPublishMetaPersisted(t *testing.T) {
	fs := &recordingStore{fakeStore: &fakeStore{}}
	fc := newFakeClient(false)
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
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

// TestTargetPublishMetaFallbackToGlobalFrame 兜底补全契约：该账号专属帧缺失/过期时，
// SetTargetsForAccount 必须回退全校帧 lastData 补全发布元数据——HTTP 直存目标不触发
// 探测（B6-04/B20-04 契约），专属帧常空；窗口关闭后 /electives 空发布，若全校帧也
// 不带本轮批次则重存空元数据落库（窗口重开后重存自愈）。修复前：只读 acctData[acct]，
// 专属帧空 → 空元数据落库 → "未知日期"。
func TestTargetPublishMetaFallbackToGlobalFrame(t *testing.T) {
	fs := &recordingStore{fakeStore: &fakeStore{}}
	fc := newFakeClient(false)
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
	s := New(&fakeAccts{c: fc}, fs, time.Time{}, time.Hour)

	// 只写全校帧 lastData（模拟"专属帧不存在"的浏览账号形态）：
	// probe() 内部 FindElectives 成功后 s.lastData 被填、acctData 不填（无目标账号
	// 不被 per-account 探测），与 B22-01/B28-01 的"无目标账号回退全校帧"同构。
	s.probe()

	// HTTP 直存目标（不触发探测，专属帧仍空）
	s.SetTargetsForAccount("acct1", []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0},
	})

	// 兜底补全生效：落库行带全校帧的发布元数据
	fs.mu.Lock()
	rows := append([]targetRow(nil), fs.targets...)
	fs.mu.Unlock()
	if len(rows) != 1 {
		t.Fatalf("应落库 1 行，实际 %d", len(rows))
	}
	if rows[0].publishName != "高二年体育" {
		t.Fatalf("落库 publish_name 应回退全校帧'高二年体育'，实际 %q", rows[0].publishName)
	}
	if rows[0].beginDate == "" {
		t.Fatal("落库 begin_date 应回退全校帧毕日期（不得为空）")
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
			PublishID:   i + 1,
			ClassID:     61115 + i,
			CourseName:  "课程",
			Priority:    i,
			PublishName: row.publishName,
			BeginDate:   row.beginDate,
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
