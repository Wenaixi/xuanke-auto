package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// Target 目标课程（每个发布 1 门）。
type Target struct {
	PublishID  int    `json:"publish_id"`
	ClassID    int    `json:"class_id"`
	CourseName string `json:"course_name"`
}

// CourseStatus 单课程任务状态。
type CourseStatus struct {
	PublishID  int    `json:"publish_id"`
	ClassID    int    `json:"class_id"`
	CourseName string `json:"course_name"`
	Status     string `json:"status"` // pending|in_range|submitted|success|failed
	Result     string `json:"result"`
}

// SchedulerState 对外状态快照。
type SchedulerState struct {
	OpenTime     time.Time      `json:"open_time"`
	WindowOpened bool           `json:"window_opened"`
	Courses      []CourseStatus `json:"courses"`
}

// Store 调度器依赖的最小持久化接口（由 store 包实现）。
type Store interface {
	AppendLog(classID int, action, result string, isOK bool) error
}

// Client 调度器依赖的至道客户端能力（zhidao.Client 隐式实现，测试可注入 mock）。
type Client interface {
	FindElectives() (*zhidao.ElectivesData, error)
	SelectClass(classID int) (string, error)
}

// Scheduler 定时抢课引擎。
type Scheduler struct {
	client   Client
	store    Store
	openTime time.Time
	interval time.Duration

	mu       sync.Mutex
	targets  []Target
	state    SchedulerState
	inflight map[int]bool // 正在提交的 classID
	done     map[int]bool // 已成功的 classID（重启恢复注入）
	lastSuccessProbe time.Time // 上次成功探测课程数据的时间（限速保护）

	ctx    context.Context
	cancel context.CancelFunc
	start  bool // 是否已启动 Start()
}

// New 创建调度器。openTime 为选课窗口开启时间（本地时区）。
func New(client Client, store Store, openTime time.Time, interval time.Duration) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		client:   client,
		store:    store,
		openTime: openTime,
		interval: interval,
		inflight: make(map[int]bool),
		done:     make(map[int]bool),
		ctx:      ctx,
		cancel:   cancel,
	}
	s.state.OpenTime = openTime
	return s
}

// SetTargets 替换目标课程并重建状态。
func (s *Scheduler) SetTargets(targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets = targets
	courses := make([]CourseStatus, 0, len(targets))
	for _, t := range targets {
		status := "pending"
		result := ""
		if s.done[t.ClassID] {
			status = "success"
			result = "重启恢复：已报名成功"
		}
		courses = append(courses, CourseStatus{
			PublishID:  t.PublishID,
			ClassID:    t.ClassID,
			CourseName: t.CourseName,
			Status:     status,
			Result:     result,
		})
	}
	s.state.Courses = courses
}

// RestoreDone 注入重启前已成功的课程 id（来自 store 的持久化状态）。
func (s *Scheduler) RestoreDone(classIDs []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range classIDs {
		s.done[id] = true
	}
	s.rebuildCoursesLocked()
}

// rebuildCoursesLocked 依据 done 集合重建课程状态（需持有锁）。
func (s *Scheduler) rebuildCoursesLocked() {
	for i := range s.state.Courses {
		if s.done[s.state.Courses[i].ClassID] {
			s.state.Courses[i].Status = "success"
			s.state.Courses[i].Result = "重启恢复：已报名成功"
		}
	}
}

// Start 启动轮询协程。
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.start {
		s.mu.Unlock()
		return
	}
	s.start = true
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
	log.Printf("[scheduler] 已启动，轮询间隔 %v，窗口开启时间 %s", s.interval, s.openTime.Format("2006-01-02 15:04:05"))
}

// Stop 停止轮询。
func (s *Scheduler) Stop() {
	s.cancel()
}

// State 返回状态快照（拷贝）。
func (s *Scheduler) State() SchedulerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	st.Courses = append([]CourseStatus(nil), s.state.Courses...)
	return st
}

// tick 单次轮询：查课程数据，判断窗口是否开启，开启则并发提交未完成目标。
// 智能降速保护：Token 未登录或窗口未开时，只每 2 秒探测一次，避免触发平台限速。
func (s *Scheduler) tick() {
	// 距上次成功探测不到 2 秒：静默跳过（限速保护）
	if time.Since(s.lastSuccessProbe) < 2*time.Second {
		return
	}

	data, err := s.client.FindElectives()
	if err != nil {
		s.mu.Lock()
		s.state.WindowOpened = false
		s.mu.Unlock()
		// Token 失效：静默跳过，不打日志不刷屏，等主人在网页登录后自动恢复
		if !errors.Is(err, zhidao.ErrUnauthorized) {
			log.Printf("[scheduler] 查询课程失败: %v", err)
		}
		return
	}
	s.lastSuccessProbe = time.Now()

	opened := false
	for _, p := range data.Publishes {
		if p.InDateRange {
			opened = true
			break
		}
	}

	s.mu.Lock()
	s.state.WindowOpened = opened
	s.mu.Unlock()
	if !opened {
		return
	}
	s.submitAll()
}

// submitAll 并发提交所有未完成目标（每课程一个 goroutine）。
func (s *Scheduler) submitAll() {
	s.mu.Lock()
	targets := append([]Target(nil), s.targets...)
	s.mu.Unlock()
	for _, t := range targets {
		s.submit(t)
	}
}

// submit 提交单个课程。已成功（done）或正在提交（inflight）则跳过。
func (s *Scheduler) submit(t Target) {
	s.mu.Lock()
	if s.done[t.ClassID] || s.inflight[t.ClassID] {
		s.mu.Unlock()
		return
	}
	s.inflight[t.ClassID] = true
	idx := s.statusIndexLocked(t.PublishID, t.ClassID)
	if idx >= 0 && s.state.Courses[idx].Status != "success" {
		s.state.Courses[idx].Status = "submitted"
	}
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[scheduler] 提交课程 %d panic: %v", t.ClassID, r)
				s.mu.Lock()
				s.inflight[t.ClassID] = false
				s.mu.Unlock()
			}
		}()
		msg, err := s.client.SelectClass(t.ClassID)
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.inflight, t.ClassID)
		idx := s.statusIndexLocked(t.PublishID, t.ClassID)
		if err != nil {
			s.setStateLocked(idx, "failed", err.Error())
			if s.store != nil {
				s.store.AppendLog(t.ClassID, "select", err.Error(), false)
			}
			return
		}
		s.done[t.ClassID] = true
		s.setStateLocked(idx, "success", msg)
		if s.store != nil {
			s.store.AppendLog(t.ClassID, "select", msg, true)
		}
		log.Printf("[scheduler] 课程 %d（%s）报名成功: %s", t.ClassID, t.CourseName, msg)
	}()
}

// statusIndexLocked 查找课程状态下标（需持有锁）。
func (s *Scheduler) statusIndexLocked(publishID, classID int) int {
	for i := range s.state.Courses {
		c := s.state.Courses[i]
		if c.PublishID == publishID && c.ClassID == classID {
			return i
		}
	}
	return -1
}

// setStateLocked 更新课程状态（需持有锁）。
func (s *Scheduler) setStateLocked(idx int, status, result string) {
	if idx < 0 {
		return
	}
	s.state.Courses[idx].Status = status
	s.state.Courses[idx].Result = result
}

// SuccessClassIDs 返回已成功课程 id 列表（持久化用）。
func (s *Scheduler) SuccessClassIDs() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int, 0, len(s.done))
	for id := range s.done {
		out = append(out, id)
	}
	return out
}

// FormatOpenTime 解析开放时间字符串（本地时区）。
func FormatOpenTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("开放时间格式错误: %w", err)
	}
	return t, nil
}
