package scheduler

import (
	"errors"
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

func (f *fakeStore) AppendLog(classID int, action, result string, isOK bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, result)
	return nil
}

// fakeClient 可编程 mock：控制课程数据与报名结果。
type fakeClient struct {
	mu          sync.Mutex
	data        *zhidao.ElectivesData
	err         error
	selectErr   map[int]error
	selectCalls map[int]int
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
	return f.data, f.err
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

func newFakeClient(open bool) *fakeClient {
	return &fakeClient{
		data: &zhidao.ElectivesData{
			Publishes: []zhidao.Publish{
				{PublishID: 1, PublishName: "体育", InDateRange: open},
				{PublishID: 2, PublishName: "校本1", InDateRange: open},
				{PublishID: 3, PublishName: "校本2", InDateRange: open},
			},
		},
		selectErr:   map[int]error{},
		selectCalls: map[int]int{},
	}
}

func targets() []Target {
	return []Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操"},
		{PublishID: 2, ClassID: 61205, CourseName: "篮球"},
		{PublishID: 3, ClassID: 61276, CourseName: "健身瑜伽"},
	}
}

func waitStatus(t *testing.T, s *Scheduler, classID int, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, c := range s.State().Courses {
			if c.ClassID == classID && c.Status == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	st := s.State()
	for _, c := range st.Courses {
		if c.ClassID == classID {
			t.Fatalf("课程 %d 状态 %q，期望 %q（结果 %q）", classID, c.Status, want, c.Result)
		}
	}
	t.Fatalf("课程 %d 不在目标中", classID)
}

func TestStateMachine(t *testing.T) {
	fc := newFakeClient(false)
	openTime := time.Now().Add(time.Hour)
	s := New(fc, &fakeStore{}, openTime, 10*time.Millisecond)
	s.SetTargets(targets())
	s.Start()
	defer s.Stop()

	// 窗口未开：状态 pending
	time.Sleep(50 * time.Millisecond)
	for _, c := range s.State().Courses {
		if c.Status != "pending" {
			t.Fatalf("窗口未开时课程 %d 状态应为 pending，实际 %q", c.ClassID, c.Status)
		}
	}
	if s.State().WindowOpened {
		t.Fatal("窗口应未开放")
	}

	// 窗口开启：应自动提交并 success
	fc.setOpen(true)
	waitStatus(t, s, 61115, "success", 3*time.Second)
	waitStatus(t, s, 61205, "success", 3*time.Second)
	waitStatus(t, s, 61276, "success", 3*time.Second)

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

func TestSubmitFailureRetries(t *testing.T) {
	fc := newFakeClient(false)
	fc.selectErr[61115] = errors.New("名额已满")
	s := New(fc, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	s.SetTargets([]Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}})
	s.Start()
	defer s.Stop()

	fc.setOpen(true)
	waitStatus(t, s, 61115, "failed", 3*time.Second)
}

func TestRestoreDoneSkipsResubmit(t *testing.T) {
	fc := newFakeClient(true)
	s := New(fc, &fakeStore{}, time.Now().Add(time.Hour), 10*time.Millisecond)
	// 重启恢复：注入已成功的课程 id
	s.RestoreDone([]int{61115})
	s.SetTargets(targets())
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
	waitStatus(t, s, 61115, "success", 1*time.Second)
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
