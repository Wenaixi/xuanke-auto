package scheduler

import (
	"context"
	"errors"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// Target 目标课程（同发布多门备选，Priority 越小越先提交）。
type Target struct {
	PublishID  int    `json:"publish_id"`
	ClassID    int    `json:"class_id"`
	CourseName string `json:"course_name"`
	Priority   int    `json:"priority"`
}

// CourseStatus 单课程任务状态。
type CourseStatus struct {
	Account    string `json:"account"`
	PublishID  int    `json:"publish_id"`
	ClassID    int    `json:"class_id"`
	CourseName string `json:"course_name"`
	Priority   int    `json:"priority"`
	Status     string `json:"status"` // pending|in_range|submitted|success|failed
	Result     string `json:"result"`
}

// SchedulerState 对外状态快照。
type SchedulerState struct {
	OpenTime     time.Time      `json:"open_time"`
	WindowOpened bool           `json:"window_opened"`
	Courses      []CourseStatus `json:"courses"`
}

// 探测分阶段间隔：平日 30 秒；临门（距开放 ≤5 分钟）与已到点未开 5 秒盯守。
const (
	probeIntervalFar  = 30 * time.Second
	probeIntervalNear = 5 * time.Second
	nearWindow        = 5 * time.Minute // 临门窗口：开放前 5 分钟起收紧
)

// probeIntervalFor 按当前时刻与开放时间的距离选择探测间隔。
// 平日 30 秒；临门（距开放 ≤5 分钟）与已到点未开 5 秒盯守，保证平台一开立即被发现。
func (s *Scheduler) probeIntervalFor(now time.Time) time.Duration {
	if now.After(s.openTimeNow().Add(-nearWindow)) {
		return probeIntervalNear
	}
	return probeIntervalFar
}

// submitInterval 窗口开启后提交重试最小间隔：1 秒（黄金期高频但不打爆平台）。
const submitInterval = time.Second

// snapshotTTL 课程快照有效期（大于探测间隔，保证 /electives 总能有数据可读）。
const snapshotTTL = 40 * time.Second

// Client 调度器依赖的至道客户端能力（*zhidao.Client 隐式满足）。
type Client interface {
	FindElectives() (*zhidao.ElectivesData, error)
	SelectClass(classID int) (string, error)
	ClassDetail(classID int) (*zhidao.ClassDetail, error)
	IsClassFull(classID int) (bool, error)
}

// AccountClients 多账号客户端注册表（真实实现 accounts.Manager）。
type AccountClients interface {
	ClientFor(acct string) (Client, bool)
	AnyClient() (Client, bool)
}

// Store 调度器依赖的最小持久化接口（由 store 包实现）。
type Store interface {
	AppendLog(acct string, classID int, action, result string, isOK bool) error
	SaveSuccess(acct string, classID int) error
}

// Scheduler 定时抢课引擎。多账号目标与已完成状态均按账号隔离。
type Scheduler struct {
	clients  AccountClients
	store    Store
	openTime time.Time
	interval time.Duration

	// openTimeFn 运行时打开时间读取器（热重载时代替启动期固化的 openTime；nil 时用 openTime）
	openTimeFn func() time.Time

	mu          sync.Mutex
	acctTargets map[string][]Target // 按账号隔离的目标课程
	state       SchedulerState
	inflight    map[string]map[int]bool // [账号][classID] 正在提交
	done        map[string]map[int]bool // [账号][classID] 已成功
	full        map[string]map[int]bool // [账号][classID] 已确认满员（快照显示不满时解除）
	lastProbe   time.Time
	lastSubmit  time.Time // 上次提交时间（submitAll 节流）
	lastData    *zhidao.ElectivesData // 内存课程快照（超高性能：/electives 直读）
	lastDataAt  time.Time

	chainMu sync.Mutex
	chains  map[string]bool // 链活跃标记：key=acct+"\x00"+publishID

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
		full:        make(map[string]map[int]bool),
		chains:      make(map[string]bool),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.state.OpenTime = openTime
	return s
}

// SetOpenTimeFn 设置运行时打开时间读取器（管理员热改配置后立即生效；传入 nil 恢复启动值）。
func (s *Scheduler) SetOpenTimeFn(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openTimeFn = fn
	if fn != nil {
		s.state.OpenTime = fn()
	}
}

// openTimeNow 返回当前生效的打开时间（运行时读取器优先）。
func (s *Scheduler) openTimeNow() time.Time {
	if s.openTimeFn != nil {
		return s.openTimeFn()
	}
	return s.openTime
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
			Priority:   t.Priority,
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
	st.OpenTime = s.openTimeNow() // 运行时配置优先（热重载立即反映）
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

// tick 单次轮询：先按需探测刷新窗口状态与课程快照；窗口开启后按 1 秒间隔持续提交。
// 探测与提交解耦：窗口开启后提交重试不受探测 30 秒节流限制（黄金期高频重试）。
func (s *Scheduler) tick() {
	now := time.Now()
	s.mu.Lock()
	last := s.lastProbe
	open := s.openTimeNow()
	s.mu.Unlock()

	// 探测闸门：距上次成功探测不足当前阶段间隔且非首次则跳过
	probe := last.IsZero() || now.Sub(last) >= s.probeIntervalFor(now)
	// 超高性能：窗口到点后的首次 tick 立即探测（不等待节流闸门放过）
	if !probe && now.After(open) && last.Before(open.Add(-time.Second)) {
		probe = true
	}
	if probe {
		s.probe()
	}

	s.mu.Lock()
	opened := s.state.WindowOpened
	s.mu.Unlock()
	if !opened {
		return
	}
	// 提交重试闸门：距上次提交不足 1 秒则跳过本轮（提交不被探测节流卡死）
	s.mu.Lock()
	lastSubmit := s.lastSubmit
	s.mu.Unlock()
	if !lastSubmit.IsZero() && now.Sub(lastSubmit) < submitInterval {
		return
	}
	s.submitAll()
}

// probe 执行一次课程探测并刷新快照与窗口状态。
func (s *Scheduler) probe() {
	now := time.Now()
	client, ok := s.clients.AnyClient()
	if !ok {
		return // 尚无账号登录，安静等待
	}
	data, err := client.FindElectives()
	if err != nil {
		s.mu.Lock()
		s.lastProbe = now // 失败同样计入节流闸门，网络故障时不会每 300ms 疯狂重试
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
	opened := false
	for _, p := range data.Publishes {
		if p.InDateRange {
			opened = true
			break
		}
	}
	s.state.WindowOpened = opened
	s.mu.Unlock()
}

// submitAll 并发提交所有账号所有发布的目标链（每链独立 goroutine，链内按人数确认满员依次退避）。
func (s *Scheduler) submitAll() {
	s.mu.Lock()
	s.lastSubmit = time.Now()
	type chain struct {
		acct string
		ts   []Target
	}
	var chains []chain
	for acct, ts := range s.acctTargets {
		byPub := map[int][]Target{}
		for _, t := range ts {
			byPub[t.PublishID] = append(byPub[t.PublishID], t)
		}
		for _, list := range byPub {
			sort.SliceStable(list, func(i, j int) bool { return list[i].Priority < list[j].Priority })
			chains = append(chains, chain{acct, list})
		}
	}
	s.mu.Unlock()
	for _, c := range chains {
		s.spawnChain(c.acct, c.ts)
	}
}

// spawnChain 逐备选提交：确认满员（快照或实时人数）才切下一备选；成功即终止。
func (s *Scheduler) spawnChain(acct string, ts []Target) {
	key := acct + "\x00" + strconv.Itoa(ts[0].PublishID)
	s.chainMu.Lock()
	if s.chains[key] {
		s.chainMu.Unlock()
		return
	}
	s.chains[key] = true
	s.chainMu.Unlock()
	go func() {
		defer func() {
			s.chainMu.Lock()
			delete(s.chains, key)
			s.chainMu.Unlock()
		}()
		client, ok := s.clients.ClientFor(acct)
		for _, t := range ts {
			s.mu.Lock()
			// 已成功：本发布目标完成，终止
			if s.doneHas(acct, t.ClassID) {
				s.mu.Unlock()
				return
			}
			s.releaseFullIfFreedLocked(acct, t.ClassID)
			if s.fullHas(acct, t.ClassID) {
				s.mu.Unlock()
				continue
			}
			// 快照人数确认满员（selected >= max）→ 记入 full，跳过本备选
			if s.classFullInSnapshot(t.ClassID) {
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				continue
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

			var msg string
			var err error
			if !ok {
				err = errors.New("账号会话未建立，等待重新登录")
			} else {
				msg, err = client.SelectClass(t.ClassID)
			}

			s.mu.Lock()
			delete(s.inflight[acct], t.ClassID)
			if err == nil {
				if s.done[acct] == nil {
					s.done[acct] = map[int]bool{}
				}
				s.done[acct][t.ClassID] = true
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "success", msg)
				if s.store != nil {
					s.store.AppendLog(acct, t.ClassID, "select", msg, true)
					_ = s.store.SaveSuccess(acct, t.ClassID)
				}
				log.Printf("[scheduler] 账号 %s 课程 %d（%s）报名成功: %s", acct, t.ClassID, t.CourseName, msg)
				s.mu.Unlock()
				return
			}
			// 非满员失败：改为实时人数复核确认是否真满员
			// （用户要求：不解析平台"满"字错误文案，直接对比总数与已报数）
			if !ok {
				// 账号会话未建立：保留状态，终止本链，下个 tick 重试
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", err.Error())
				if s.store != nil {
					s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": "+err.Error(), false)
				}
				s.mu.Unlock()
				return
			}
			full, cErr := s.classFullRealtime(acct, t.ClassID)
			if cErr == nil && full {
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				continue
			}
			// 实时复核未现满员（网络抖动/人未满但报名被拒）：保留失败状态，终止本链，下个 tick 重试
			s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", err.Error())
			if s.store != nil {
				s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": "+err.Error(), false)
			}
			s.mu.Unlock()
			return
		}
	}()
}

// classFullInSnapshot 快照人数确认满员（需持锁）。
func (s *Scheduler) classFullInSnapshot(classID int) bool {
	if s.lastData == nil {
		return false
	}
	for _, p := range s.lastData.Publishes {
		for _, c := range p.Classes {
			if c.ID == classID {
				return c.MaxCount > 0 && c.SelectedCount >= c.MaxCount
			}
		}
	}
	return false
}

// classFullRealtime 实时人数复核（锁外调用，禁止持锁时发起网络请求）。
func (s *Scheduler) classFullRealtime(acct string, classID int) (bool, error) {
	client, ok := s.clients.ClientFor(acct)
	if !ok {
		return false, errors.New("账号会话未建立")
	}
	return client.IsClassFull(classID)
}

// markFullLocked 确认满员：记入 full 集合并置 failed 状态（每课程只记录一次）。
func (s *Scheduler) markFullLocked(acct string, t Target) {
	if s.full[acct] == nil {
		s.full[acct] = map[int]bool{}
	}
	if s.full[acct][t.ClassID] {
		return
	}
	s.full[acct][t.ClassID] = true
	s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "该课程已满员，退避至下一备选")
	if s.store != nil {
		s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 课程 "+t.CourseName+" 已满员，切换备选", false)
	}
}

// releaseFullIfFreedLocked 快照（新鲜且显示不满）时解除 full 标记并回 pending（需持锁）。
func (s *Scheduler) releaseFullIfFreedLocked(acct string, classID int) {
	if !s.fullHas(acct, classID) {
		return
	}
	if s.lastData == nil || time.Since(s.lastDataAt) > snapshotTTL {
		return // 快照过期，等下一次有效快照再判断
	}
	if s.classFullInSnapshot(classID) {
		return
	}
	delete(s.full[acct], classID)
	idx := s.statusIndexLocked(acct, classID)
	if idx >= 0 {
		s.state.Courses[idx].Status = "pending"
		s.state.Courses[idx].Result = ""
	}
}

func (s *Scheduler) fullHas(acct string, classID int) bool {
	m, ok := s.full[acct]
	return ok && m[classID]
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
