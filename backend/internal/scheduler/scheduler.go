package scheduler

import (
	"context"
	"errors"
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
	Account    string `json:"account"`
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

// probeInterval 课程探测最小间隔：30 秒，避免触发平台"访问过于频繁"熔断。
const probeInterval = 30 * time.Second

// snapshotTTL 课程快照有效期（大于探测间隔，保证 /electives 总能有数据可读）。
const snapshotTTL = 40 * time.Second

// Client 调度器依赖的至道客户端能力（*zhidao.Client 隐式满足）。
type Client interface {
	FindElectives() (*zhidao.ElectivesData, error)
	SelectClass(classID int) (string, error)
}

// AccountClients 多账号客户端注册表（真实实现 accounts.Manager）。
type AccountClients interface {
	ClientFor(acct string) (Client, bool)
	AnyClient() (Client, bool)
}

// Store 调度器依赖的最小持久化接口（由 store 包实现）。
type Store interface {
	AppendLog(classID int, action, result string, isOK bool) error
	SaveSuccess(acct string, classID int) error
}

// Scheduler 定时抢课引擎。多账号目标与已完成状态均按账号隔离。
type Scheduler struct {
	clients  AccountClients
	store    Store
	openTime time.Time
	interval time.Duration

	mu          sync.Mutex
	acctTargets map[string][]Target // 按账号隔离的目标课程
	state       SchedulerState
	inflight    map[string]map[int]bool // [账号][classID] 正在提交
	done        map[string]map[int]bool // [账号][classID] 已成功
	lastProbe   time.Time
	lastData    *zhidao.ElectivesData // 内存课程快照（超高性能：/electives 直读）
	lastDataAt  time.Time

	ctx    context.Context
	cancel context.CancelFunc
	start  bool
}

// New 创建调度器。openTime 为选课窗口开启时间（本地时区）。
func New(clients AccountClients, store Store, openTime time.Time, interval time.Duration) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		clients:     clients,
		store:       store,
		openTime:    openTime,
		interval:    interval,
		acctTargets: make(map[string][]Target),
		inflight:    make(map[string]map[int]bool),
		done:        make(map[string]map[int]bool),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.state.OpenTime = openTime
	return s
}

// SetTargetsForAccount 为指定账号替换目标并重建状态（账号必填，非空）。
func (s *Scheduler) SetTargetsForAccount(acct string, targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acctTargets[acct] = targets
	// 仅重建该账号对应的课程状态（保留其他账号）
	keep := s.state.Courses[:0]
	for _, c := range s.state.Courses {
		if c.Account != acct {
			keep = append(keep, c)
		}
	}
	s.state.Courses = keep
	for _, t := range targets {
		status := "pending"
		result := ""
		if s.doneHas(acct, t.ClassID) {
			status = "success"
			result = "重启恢复：已报名成功"
		}
		s.state.Courses = append(s.state.Courses, CourseStatus{
			Account:    acct,
			PublishID:  t.PublishID,
			ClassID:    t.ClassID,
			CourseName: t.CourseName,
			Status:     status,
			Result:     result,
		})
	}
}

// RestoreDone 注入重启前已成功的 (账号, 课程) 记录。
func (s *Scheduler) RestoreDone(done map[string][]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for acct, ids := range done {
		if s.done[acct] == nil {
			s.done[acct] = map[int]bool{}
		}
		for _, id := range ids {
			s.done[acct][id] = true
		}
	}
	s.rebuildCoursesLocked()
}

// rebuildCoursesLocked 依据 done 集合重建课程状态（需持有锁）。
func (s *Scheduler) rebuildCoursesLocked() {
	for i := range s.state.Courses {
		c := &s.state.Courses[i]
		if s.doneHas(c.Account, c.ClassID) {
			c.Status = "success"
			c.Result = "重启恢复：已报名成功"
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
	log.Printf("[scheduler] 已启动，轮询间隔 %v（课程探测节流 30 秒），窗口开启时间 %s", s.interval, s.openTime.Format("2006-01-02 15:04:05"))
}

// Stop 停止轮询。
func (s *Scheduler) Stop() {
	s.cancel()
}

// StateForAccount 返回指定账号的状态快照（Courses 仅含该账号目标；WindowOpened 全校共享）。
func (s *Scheduler) StateForAccount(acct string) SchedulerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	st.Courses = nil
	for _, c := range s.state.Courses {
		if c.Account == acct {
			st.Courses = append(st.Courses, c)
		}
	}
	return st
}

// ElectivesSnapshot 返回内存课程快照（40 秒内有效）。超高性能核心：页面浏览零上游请求。
func (s *Scheduler) ElectivesSnapshot() (*zhidao.ElectivesData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastData == nil || time.Since(s.lastDataAt) > snapshotTTL {
		return nil, false
	}
	return s.lastData, true
}

// ProbeNow 立即执行一次课程探测并刷新快照（/api/electives 快照过期时调用）。
func (s *Scheduler) ProbeNow() (*zhidao.ElectivesData, error) {
	if s.clients == nil {
		return nil, errors.New("没有任何已登录账号")
	}
	client, ok := s.clients.AnyClient()
	if !ok || client == nil {
		return nil, errors.New("没有任何已登录账号")
	}
	data, err := client.FindElectives()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.lastProbe = time.Now()
	s.lastData = data
	s.lastDataAt = time.Now()
	s.mu.Unlock()
	return data, nil
}

// tick 单次轮询：查课程数据，判断窗口是否开启，开启则并发提交所有账号目标。
func (s *Scheduler) tick() {
	now := time.Now()
	s.mu.Lock()
	last := s.lastProbe
	s.mu.Unlock()

	// 探测闸门：距上次成功探测不足 30 秒且非首次则跳过
	probe := last.IsZero() || now.Sub(last) >= probeInterval
	// 超高性能：窗口到点后的首次 tick 立即探测（不等待 30s 闸门放过）
	if !probe && now.After(s.openTime) && last.Before(s.openTime.Add(-time.Second)) {
		probe = true
	}
	if !probe {
		return
	}

	client, ok := s.clients.AnyClient()
	if !ok {
		return // 尚无账号登录，安静等待
	}
	data, err := client.FindElectives()
	if err != nil {
		s.mu.Lock()
		s.state.WindowOpened = false
		s.mu.Unlock()
		if !errors.Is(err, zhidao.ErrUnauthorized) {
			log.Printf("[scheduler] 查询课程失败: %v", err)
		}
		return
	}
	s.mu.Lock()
	s.lastProbe = now
	s.lastData = data
	s.lastDataAt = now
	s.mu.Unlock()

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

// submitAll 并发提交所有账号的所有未完成目标（每账号每课程独立 goroutine）。
func (s *Scheduler) submitAll() {
	s.mu.Lock()
	type pair struct {
		acct string
		t    Target
	}
	var pairs []pair
	for acct, ts := range s.acctTargets {
		for _, t := range ts {
			pairs = append(pairs, pair{acct, t})
		}
	}
	s.mu.Unlock()
	for _, p := range pairs {
		s.submit(p.acct, p.t)
	}
}

// submit 提交单个课程（用目标账号自己的会话）。已成功（done）或正在提交（inflight）则跳过。
func (s *Scheduler) submit(acct string, t Target) {
	s.mu.Lock()
	if s.doneHas(acct, t.ClassID) || s.inflightHas(acct, t.ClassID) {
		s.mu.Unlock()
		return
	}
	if s.inflight[acct] == nil {
		s.inflight[acct] = map[int]bool{}
	}
	s.inflight[acct][t.ClassID] = true
	idx := s.statusIndexLocked(acct, t.ClassID)
	if idx >= 0 && s.state.Courses[idx].Status != "success" {
		s.state.Courses[idx].Status = "submitted"
	}
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[scheduler] 提交课程 %d（账号 %s）panic: %v", t.ClassID, acct, r)
				s.mu.Lock()
				delete(s.inflight[acct], t.ClassID)
				s.mu.Unlock()
			}
		}()
		client, ok := s.clients.ClientFor(acct)
		var msg string
		var err error
		if !ok {
			err = errors.New("账号会话未建立，等待重新登录")
		} else {
			msg, err = client.SelectClass(t.ClassID)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.inflight[acct], t.ClassID)
		idx := s.statusIndexLocked(acct, t.ClassID)
		if err != nil {
			s.setStateLocked(idx, "failed", err.Error())
			if s.store != nil {
				s.store.AppendLog(t.ClassID, "select", "账号 "+acct+": "+err.Error(), false)
			}
			return
		}
		if s.done[acct] == nil {
			s.done[acct] = map[int]bool{}
		}
		s.done[acct][t.ClassID] = true
		s.setStateLocked(idx, "success", msg)
		if s.store != nil {
			s.store.AppendLog(t.ClassID, "select", msg, true)
			_ = s.store.SaveSuccess(acct, t.ClassID)
		}
		log.Printf("[scheduler] 账号 %s 课程 %d（%s）报名成功: %s", acct, t.ClassID, t.CourseName, msg)
	}()
}

func (s *Scheduler) doneHas(acct string, classID int) bool {
	m, ok := s.done[acct]
	return ok && m[classID]
}

func (s *Scheduler) inflightHas(acct string, classID int) bool {
	m, ok := s.inflight[acct]
	return ok && m[classID]
}

// statusIndexLocked 按账号 + 课程查找状态下标（需持有锁）。
func (s *Scheduler) statusIndexLocked(acct string, classID int) int {
	for i := range s.state.Courses {
		c := s.state.Courses[i]
		if c.Account == acct && c.ClassID == classID {
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

// FormatOpenTime 解析开放时间字符串（本地时区）。
func FormatOpenTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		return time.Time{}, errors.New("开放时间格式错误: " + err.Error())
	}
	return t, nil
}
