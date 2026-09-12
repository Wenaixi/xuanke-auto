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
	TokenValid   bool           `json:"token_valid"` // 当前账号教务 token 有效性（有效=true）
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
	IsClassFull(classID int) (bool, error)
	Token() string // 重登后读取新 token 落库
}

// AccountClients 多账号客户端注册表（真实实现 accounts.Manager）。
type AccountClients interface {
	ClientFor(acct string) (Client, bool)
	AnyClient() (Client, bool)
	// AnyClientWithAccount 返回任一已登录账号的客户端与账号名（失效时定位账号用）。
	AnyClientWithAccount() (string, Client, bool)
	// Relogin 对指定账号自动重登（返回是否已重登与错误）。
	Relogin(acct string) (bool, error)
}

// Store 调度器依赖的最小持久化接口（由 store 包实现）。
type Store interface {
	AppendLog(acct string, classID int, action, result string, isOK bool) error
	SaveSuccess(acct string, classID int) error
	UpdateIDToken(acct, idToken string) error // 自动重登后落库新 token
}

// Scheduler 定时抢课引擎。多账号目标与已完成状态均按账号隔离。
type Scheduler struct {
	clients  AccountClients
	store    Store
	openTime time.Time
	interval time.Duration

	// openTimeFn 运行时打开时间读取器（热重载时代替启动期固化的 openTime；nil 时用 openTime）
	openTimeFn func() time.Time

	mu               sync.Mutex
	acctTargets      map[string][]Target // 按账号隔离的目标课程
	state            SchedulerState
	inflight         map[string]map[int]bool // [账号][classID] 正在提交
	done             map[string]map[int]bool // [账号][classID] 已成功
	full             map[string]map[int]bool // [账号][classID] 已确认满员（快照显示不满时解除）
	lastProbe        time.Time
	lastSubmit       time.Time             // 上次提交时间（submitAll 节流）
	prevWindowOpened bool                  // 上一次探测的窗口状态（用于窗口刚开启时清提交闸门）
	lastData         *zhidao.ElectivesData // 内存课程快照（超高性能：/electives 直读）
	lastDataAt       time.Time
	tokenValid       map[string]bool      // [账号] token 失效标记（false=有效，缺失即有效）
	reloginAt        map[string]time.Time // [账号] 上次重登时间（30s 节流 + 退避计时基准）
	reloginFail      map[string]int       // [账号] 连续重登失败次数（指数退避：fail 次后间隔 30s<<fail，封顶 10min）
	relogging        map[string]bool      // [账号] 重登进行中标记（区别于"已失效待重登"，保证失败后可再试）
	reloginMu        sync.Mutex           // 重登决策串行化（持锁时间极短，仅 map 读写；Login 在锁外执行）
	reloginResults   chan reloginResult   // 重登结果回传（异步结果在 tick 主循环统一处理）

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
		tokenValid:  make(map[string]bool),
		reloginAt:   make(map[string]time.Time),
		reloginFail: make(map[string]int),
		relogging:   make(map[string]bool),
		chains:      make(map[string]bool),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.reloginResults = make(chan reloginResult, 8)
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
			case res := <-s.reloginResults:
				// 重登成功回传（主循环统一处，避免 goroutine 并发写 s.lastProbe 竞态）
				if res.relogged && res.err == nil {
					s.mu.Lock()
					s.lastProbe = time.Time{} // 补一次探测
					s.mu.Unlock()
				}
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
	st.TokenValid = !s.tokenValid[acct] && !s.relogging[acct]
	st.Courses = nil
	for _, c := range s.state.Courses {
		if c.Account == acct {
			st.Courses = append(st.Courses, c)
		}
	}
	return st
}

// TokenValidFor 查询指定账号教务 token 有效性（未记录失效标记即视为有效）。
// relogging（重登进行中）也视为失效——重登尚未完成时对外显示"已失效·自动恢复中"。
func (s *Scheduler) TokenValidFor(acct string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.tokenValid[acct] && !s.relogging[acct]
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

// WindowOpened 返回当前窗口开启状态（以调度器实际探测结果为准）。
// 返回 nil 表示调度器尚未产生任何探测结论（学生端 /state 同源字段）。
func (s *Scheduler) WindowOpened() *bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &s.state.WindowOpened
}

// ProbeNow 立即执行一次课程探测并刷新快照（/api/electives 快照过期时调用）。
// 命中 token 失效（ErrUnauthorized）时同步触发该账号自动重登——用户刷新课程页
// 不必等调度器下个 30s 周期探测才发现并恢复（异步重登，不阻塞响应）。
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
		if errors.Is(err, zhidao.ErrUnauthorized) {
			if acct, _, ok := s.clients.AnyClientWithAccount(); ok {
				s.maybeRelogin(acct)
			}
		}
		return nil, err
	}
	s.lastProbe = time.Now()
	s.lastData = data
	s.lastDataAt = time.Now()
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
	// 提交触发条件（或关系）：
	//   1) 探测已确认窗口开启（WindowOpened）；
	//   2) 本地时间已过开窗点（openTimeNow）——兜底：平台在到点瞬间把课程列表拉空
	//      （熔断/学期异常）或探测恰好失败时，不依赖探测确认也放行提交，黄金期不容浪费。
	// 注意 WindowOpened 只在"探测成功且列表非空"时更新；探测失败或 Publishes 被平台熔断拉空时
	// 维持上一轮值，因此这里不会把已开启的窗口误判为关闭。
	if !opened && !now.After(open) {
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
// 窗口状态判定与提交状态解耦：即使快照 Publishes 为空（平台熔断/学期数据异常被拉空），
// 也只视为"尚未确认窗口开启"，绝不把已开启的窗口误判为关闭（安全审计 MAJOR#4）——
// prevWindowOpened 在探测失败/空数据路径保持原值，窗口一旦开过就维持已开状态，
// 提交循环（spawnChain）仍会继续尝试目标课程，黄金期不因数据异常而停摆。
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
		if errors.Is(err, zhidao.ErrUnauthorized) {
			// 探测账号的 token 失效 → 自动重登（网络类失败绝不重登）
			if acct, _, ok := s.clients.AnyClientWithAccount(); ok {
				s.maybeRelogin(acct)
			}
			return
		}
		log.Printf("[scheduler] 查询课程失败: %v", err)
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
	// 窗口状态变化（关→开）时清空提交闸门：热改 openTime 提前/回拨后，首个 tick 立即提交而不被 1s 闸门卡掉
	if opened && !s.prevWindowOpened {
		s.lastSubmit = time.Time{}
	}
	s.prevWindowOpened = opened
	s.mu.Unlock()
}

// reloginInterval 重登节流：30 秒内最多重登一次（与探测节流同频，避免频繁登录触发平台限流）。
// 这是同一账号重登的最短间隔；排它控制交给下面的失败退避表（连续失败时间隔指数拉长）。
const reloginInterval = 30 * time.Second

// 重登失败退避（防平台锁号的最后防线）：连续失败 n 次后，距离下次重试为 backoffMin << n，
// 封顶 backoffMax，所以 Vision 服务持续故障时登录频率只会越来越低，绝不会把账号刷到锁号。
const (
	backoffMin = 30 * time.Second
	backoffMax = 10 * time.Minute
)

func (s *Scheduler) reloginBackoff(n int) time.Duration {
	d := backoffMin << n // 每次失败翻倍
	if d > backoffMax || d <= 0 {
		return backoffMax
	}
	return d
}

// maybeRelogin 对指定账号异步自动重登：防重入 + 30s 节流 + 失败指数退避（安全审计要求），
// 成功后落库新 token 并补一次探测。失败/未重登会复位失效标记，退避窗口过后仍可再试。
// 锁纪律：reloginMu 只串行化"决策是否发起"这一段（纯 map 读写，微秒级），
// 实际重登（Login）在锁外 goroutine 执行——一个账号重登慢（Vision 最坏 2 分钟）
// 不会拖延其他账号的重登与提交（安全审计 MINOR 8：全局锁跨长 Login 的修复）。
func (s *Scheduler) maybeRelogin(acct string) {
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()

	s.mu.Lock()
	if s.relogging[acct] {
		s.mu.Unlock()
		return // 已有重登 goroutine 在跑，绝不再开第二条
	}
	// 失败退避：连续失败次数 >0 时，按指数间隔等待，Vision 故障期不轰炸登录接口
	if n := s.reloginFail[acct]; n > 0 {
		wait := s.reloginBackoff(n)
		if t, ok := s.reloginAt[acct]; ok {
			if time.Since(t) < wait {
				s.mu.Unlock()
				return // 退避窗口内不再发起
			}
			log.Printf("[scheduler] 账号 %s 自动重登失败 %d 次，已过 %v 退避窗口，再次尝试", acct, n, wait)
		}
		delete(s.reloginAt, acct) // 退避窗口已过：清等待时间，走本次新间隔
	} else if t, ok := s.reloginAt[acct]; ok && time.Since(t) < reloginInterval {
		s.mu.Unlock()
		return // 30s 基础节流
	}
	s.reloginAt[acct] = time.Now()
	s.reloginFail[acct]++
	s.relogging[acct] = true // 标记重登中
	s.mu.Unlock()

	go func() {
		// 重登期间对平台屏蔽该账号提交（无效 token 请求纯浪费 + 熔断风险）
		s.mu.Lock()
		s.tokenValid[acct] = true
		s.mu.Unlock()

		relogged, err := s.clients.Relogin(acct)
		s.mu.Lock()
		delete(s.relogging, acct) // 清重登中标记（失败也清，才能再试）
		if err == nil && relogged {
			delete(s.reloginFail, acct) // 成功清零失败计数，退避表归零
			s.tokenValid[acct] = false
			// 新 token 落库（持久化，重启后恢复不丢）
			if client, ok := s.clients.ClientFor(acct); ok {
				if tok := client.Token(); tok != "" {
					if uerr := s.store.UpdateIDToken(acct, tok); uerr != nil {
						log.Printf("[scheduler] 账号 %s 新 token 落库失败: %v", acct, uerr)
					}
				}
			}
			// 重登成功后立即补一次探测（换新 token 后窗口可能已开）。
			// 非阻塞发送：通道满（并发重登全部成功）时宁可弃掉补探测信号，
			// 也不持 s.mu 阻塞整个调度器（安全审查发现的持锁阻塞风险）。
			select {
			case s.reloginResults <- reloginResult{acct: acct, relogged: true, err: nil}:
			default:
			}
			s.mu.Unlock()
			log.Printf("[scheduler] 账号 %s 教务 token 已自动重登恢复", acct)
			return
		}
		// 失败/未重登：保持失效标记（tokenValid 仍 true），前端显示"已失效·自动恢复中"，
		// 不再误报"有效"（安全审计 MINOR 7）。退避窗口过后下个探测周期会再次尝试恢复。
		s.mu.Unlock()
		if err != nil {
			log.Printf("[scheduler] 账号 %s 自动重登失败: %v", acct, err)
		} else {
			log.Printf("[scheduler] 账号 %s 无保存账密，无法自动重登（请手动重新登录）", acct)
		}
	}()
}

// reloginResult 重登结果（异步回传到 tick 主循环统一处理）。
type reloginResult struct {
	acct     string
	relogged bool
	err      error
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
		// 账号已被管理员删除（accounts.Remove）：不再为其生成提交链，
		// 残留的目标/客户端不经此路径继续报名（删账号后彻底隔离）。
		if _, ok := s.clients.ClientFor(acct); !ok {
			continue
		}
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
			// 重登期间跳过该账号全部提交（无效 token 请求纯浪费 + 熔断风险）
			if s.relogging[acct] {
				s.mu.Unlock()
				return
			}
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
			// 该账号 token 失效：标记失效并异步重登（非探测账号也能触发），终止本链等恢复
			if errors.Is(err, zhidao.ErrUnauthorized) {
				s.maybeRelogin(acct)
				s.mu.Lock()
				delete(s.inflight[acct], t.ClassID) // 清提交标记（避免残留占用）
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "教务令牌失效，自动重登中")
				if s.store != nil {
					s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 教务令牌失效，自动重登中", false)
				}
				s.mu.Unlock()
				return
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
