package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
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
	WindowClosed bool           `json:"window_closed"` // 探测为空快照且从未开过窗 = 选课窗口已关闭
	TokenValid   bool           `json:"token_valid"`   // 当前账号教务 token 有效性（有效=true）
	Courses      []CourseStatus `json:"courses"`
	// EmptyProbeRuns B20-02（第 20 轮）："空快照且从未开窗"的连续探测轮数——WindowClosed
	// 视同关闭判据之一（探测量变），由 probe() 入账推进、开窗/非空快照归零；零值=尚未连续
	// 探测到 3 轮（首探 1 次、二探 2 次都不算）。json 省略：前端/外部无需感知内部量变。
	EmptyProbeRuns int `json:"-"`
}

// 探测分阶段间隔：平日 30 秒；临门（距开放 ≤5 分钟）与已到点未开 2 秒紧密盯守。
const (
	probeIntervalFar  = 30 * time.Second
	probeIntervalNear = 2 * time.Second // 临门收紧至 2 秒，开窗探测更敏锐
	nearWindow        = 5 * time.Minute // 临门窗口：开放前 5 分钟起收紧
)

// 黄金期高频冲刺提交间隔
const (
	submitIntervalSprint = 250 * time.Millisecond // 黄金期（开窗后 10 秒内）高频冲刺：250ms
	submitIntervalNormal = time.Second            // 常规提交间隔：1 秒
	sprintDuration       = 10 * time.Second       // 黄金冲刺期持续时长
)

// probeIntervalFor 按当前时刻与开放时间的距离选择探测间隔。
// 平日 30 秒；临门（距开放 ≤5 分钟）与已到点未开 2 秒盯守，保证平台一开立即被发现。
// 窗口已关闭（开放时间已过且快照为空）时降回 30 秒——窗口结束后再高频盯守毫无意义，
// 只会浪费请求并刷屏日志；若管理员热改开放时间到未来（新一轮），临门判断仍优先生效。
// B15-M2（第 15 轮）：open 为零值（全新部署未配置 / 管理员 PUT open_time="" 显式解除
// 窗口机制，runtime.reparse 置 OpenTimeParsed 零值）时提前返回远间隔——此前
// `now.After(open.Add(-nearWindow))` 对零值 open 恒 true 落入临门 2s 分支，且快照非空时
// WindowClosed 恒 false，探测永久 2s 高频轰炸 findElectivesData（"访问过于频繁"熔断形态）；
// 提交已被 B11-A1 零值守卫挂起，探测也必须同步降频，行为不自相矛盾。
func (s *Scheduler) probeIntervalFor(now time.Time) time.Duration {
	if s.openTimeNow().IsZero() {
		return probeIntervalFar
	}
	if now.After(s.openTimeNow().Add(-nearWindow)) {
		// B19-01（第 19 轮）：从未开过窗 + 开放时间已过 + 空快照 = 幽灵窗口（平台窗口从未
		// 开启或已关闭且从未被探测确认）——2s 高频盯守只剩烧平台（"访问过于频繁"熔断
		// 形态）。以空快照 + 时钟失败裕量判定幽灵窗口，探测降回 30s 常态（窗口若真开、
		// 管理员热改开放时间，临门判断自然重新收紧）。黄金期不受影响：开窗瞬间探测
		// 确认 opened=true，绝不走此分支。
		if now.After(s.openTimeNow()) && s.WindowClosed() {
			return probeIntervalFar // 开放时间已过且窗口关闭：降回 30s
		}
		return probeIntervalNear
	}
	return probeIntervalFar
}

// snapshotTTL 课程快照有效期（大于探测间隔，保证 /electives 总能有数据可读）。
const snapshotTTL = 40 * time.Second

// TimeSyncer 客户端可选实现的服务端时钟对齐能力。
type TimeSyncer interface {
	SyncServerTime() (time.Duration, error)
}

// Prewarmer 客户端可选实现的连接池静默预热能力。
type Prewarmer interface {
	Prewarm() error
}

// Client 调度器依赖的至道客户端能力（*zhidao.Client 隐式满足）。
type Client interface {
	FindElectives() (*zhidao.ElectivesData, error)
	SelectClass(classID int) (string, error)
	ExitClass(classID int) (string, error)
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
	UpdateIDToken(acct, idToken string) error     // 自动重登后落库新 token
	DeleteSuccess(acct string, classID int) error // B8-M2（第 8 轮）：手动退选后删除 success 行
	SaveRefused(acct string, classID int) error   // B9-02（第 9 轮）：手动退选记库，重启后自动引擎仍不抢回
	DeleteRefused(acct string) error              // B9-02：重设目标清空该账号全部退选标记
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
	refused          map[string]map[int]bool // [账号][classID] 用户手动退选（自动引擎绝不抢回，直到重设目标）
	lastProbe        time.Time               // 全校正规探测节流闸门：只归 probe()/ProbeNow 写入（B6-04）
	lastSubmit       time.Time               // 上次提交时间（submitAll 节流）
	lastData         *zhidao.ElectivesData   // 内存课程快照（超高性能：/electives 直读）
	lastDataAt       time.Time
	acctData         map[string]*zhidao.ElectivesData // [账号] 专属课程快照（年级物理隔离）
	acctDataAt       map[string]time.Time             // [账号] 专属快照时间戳
	tokenValid       map[string]bool                  // [账号] token 失效标记（false=有效，缺失即有效）
	reloginAt        map[string]time.Time             // [账号] 上次重登时间（30s 节流 + 退避计时基准）
	reloginFail      map[string]int                   // [账号] 连续重登失败次数（指数退避：fail 次后间隔 30s<<fail，封顶 10min）
	relogging        map[string]bool                  // [账号] 重登进行中标记（区别于"已失效待重登"，保证失败后可再试）
	reloginMu        sync.Mutex                       // 重登决策串行化（持锁时间极短，仅 map 读写；Login 在锁外执行）
	reloginResults   chan reloginResult               // 重登结果回传（异步结果在 tick 主循环统一处理）
	warnedNoTargets  bool                             // M-3：无目标空转警告只打一次

	clockOffset    time.Duration                // 服务端时钟对齐偏差 (server - local)
	lastSyncTime   time.Time                    // 上次时钟对齐成功采样时间（仅成功推进，B7-M1）
	lastSyncStart  time.Time                    // 当前正在进行的同步发起时刻（成功时回写 lastSyncTime）
	lastSyncFailAt time.Time                    // 上次同步失败时刻（B9-03 失败退避计时基准）
	syncFailedWindow time.Time                  // B19-01：时钟失败/恢复时刻留档（写而不读，判据用 syncFailStreak，见 maybeSyncClock）
	syncing        bool                         // 同步进行中标记（防 tick 叠加发起并发同步，B7-M1）
	probing        bool                         // 探测进行中标记（单飞：同一时刻全校只允许一次 probe 在跑，F12-B2）
	syncFailStreak int                          // 时钟同步连续失败次数（≥3 时回退 offset=0，MAJOR-C）
	lastPrewarm    time.Time                    // 上次连接池预热时间
	rateLimited    map[string]map[int]time.Time // [账号][classID] 风控退避截止时刻

	chainMu sync.Mutex
	chains  map[string]bool // 链活跃标记：key=acct+"\x00"+publishID

	probeSem chan struct{} // F17-01（第 17 轮）：per-account 探测并发信号量（cap 4）

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
		refused:     make(map[string]map[int]bool),
		tokenValid:  make(map[string]bool),
		reloginAt:   make(map[string]time.Time),
		reloginFail: make(map[string]int),
		relogging:   make(map[string]bool),
		rateLimited: make(map[string]map[int]time.Time),
		chains:      make(map[string]bool),
		probeSem:    make(chan struct{}, 4), // F17-01：per-account 探测并发上限
		acctData:    make(map[string]*zhidao.ElectivesData),
		acctDataAt:  make(map[string]time.Time),
		ctx:         ctx,
		cancel:      cancel,
	}
	s.reloginResults = make(chan reloginResult, 8)
	s.state.OpenTime = openTime
	return s
}

// nowAligned 返回经过教务服务端时钟校准后的当前时刻。
func (s *Scheduler) nowAligned() time.Time {
	s.mu.Lock()
	offset := s.clockOffset
	s.mu.Unlock()
	return time.Now().Add(offset)
}

// nowAlignedLocked 在持有 s.mu 时返回校准时刻（禁止重入加锁）。
func (s *Scheduler) nowAlignedLocked() time.Time {
	return time.Now().Add(s.clockOffset)
}

// SetClockOffsetForTest 显式设置服务端时钟偏差（测试专用）。
func (s *Scheduler) SetClockOffsetForTest(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clockOffset = d
}

// submitIntervalFor 按当前时刻与开窗时刻计算动态提交间隔（开窗前 10 秒 250ms 冲刺）。
func (s *Scheduler) submitIntervalFor(now, open time.Time) time.Duration {
	if !open.IsZero() && now.After(open) && now.Before(open.Add(sprintDuration)) {
		return submitIntervalSprint
	}
	return submitIntervalNormal
}

// maybePrewarm 在临门窗口期内保持底层 HTTP 连接池热态。
func (s *Scheduler) maybePrewarm(now, open time.Time) {
	if open.IsZero() || now.Before(open.Add(-2*time.Minute)) || now.After(open) {
		return
	}
	s.mu.Lock()
	if !s.lastPrewarm.IsZero() && now.Sub(s.lastPrewarm) < 15*time.Second {
		s.mu.Unlock()
		return
	}
	s.lastPrewarm = now
	s.mu.Unlock()

	if client, ok := s.clients.AnyClient(); ok {
		if pw, ok := client.(Prewarmer); ok {
			go func() { _ = pw.Prewarm() }()
		}
	}
}

// maybeSyncClock 定期异步采样教务服务端时间，校准本地时钟偏差。
// MAJOR-C 回退：同步连续失败 3 次即复位 clockOffset=0（窗口判定回到本地时钟），
// 绝不带着一个过期偏差长期误判开窗点；单次成功立即清零失败计数，瞬断不累计。
// B7-M1（第 7 轮）：同步闸门推进改为"同步成功才推进 lastSyncTime"——此前在锁内、
// 发起异步 goroutine 前就把 lastSyncTime=now：网络抖动导致 SyncServerTime 挂起 >300ms
// 时（等于上一个 tick 间隔），并发 goroutine 回写会跳过一个完整的 60s 窗口，且
// 连续失败 3 次复位 clockOffset 后该分钟整段不再校准。现在的推进语义：启动同步
// 即记录发起时刻（lastSyncStart），只有成功采样才把 lastSyncTime 推到发起时刻——
// 失败绝不吃闸门、也绝不复位 lastSyncTime，下一轮 tick 立即可重试，校准窗口最多
// 丢失一个 tick 间隔（300ms）而非整分钟。
func (s *Scheduler) maybeSyncClock(now time.Time) {
	s.mu.Lock()
	// 无账号注册表（空库/账号全删）：直接放弃——AnyClient 拿不到客户端，同步无从发起。
	// 防御 F12-B1：clients 为 nil 时若继续走 `s.clients.AnyClient()` 会空指针 panic。
	if s.clients == nil {
		s.mu.Unlock()
		return
	}
	// 快路径（现在被成功推进）：距上次成功采样 ≥1min 才发起新同步
	if !s.lastSyncTime.IsZero() && now.Sub(s.lastSyncTime) < time.Minute {
		s.mu.Unlock()
		return
	}
	// B9-03（第 9 轮）：失败退避——上次同步失败后 30s 内不得再发起。
	// 此前无失败退避：lastSyncTime 只在成功时推进，一旦同步失败（网络故障/平台拒绝）
	// 该闸门永久"从未成功"，每个 tick（300ms）都试图发起、每次都被 syncing 挡下后
	// 立即重新置位，SyncServerTime goroutine 以 300ms 节奏轰炸，绕开登录频率闸门、
	// 堆积在途请求、烧 token 与平台限流预算。现在失败落地即记录 lastSyncFailAt，
	// 30s 窗口内 maybeSyncClock 直接返回；窗口过后才允许下一次尝试，
	// 与 relogin 的 30s 基础节流对齐——失败期时钟校准整体退化为按分钟重试。
	if !s.lastSyncFailAt.IsZero() && now.Sub(s.lastSyncFailAt) < 30*time.Second {
		s.mu.Unlock()
		return
	}
	// 防重入：上一轮同步仍在进行（未落地），本 tick 不叠加。
	// B8-M1（第 8 轮）：此前 `if !s.syncing{...}; inflight:=s.syncing; if !inflight{return}`
	// 中 syncing 恒被置 true、inflight 恒 true——死代码，每个 tick（300ms）在同步失败期
	// 都会再 spawn 一个 SyncServerTime goroutine（绕开登录频率闸门、堆积在途、streak 并发
	// 累加诱发瞬断复位风暴）。现在在途即直接返回，真正的单飞语义。
	if s.syncing {
		s.mu.Unlock()
		return
	}
	s.syncing = true
	s.lastSyncStart = now
	s.mu.Unlock()

	// 发起异步时钟校准；goroutine 完成回调复位 syncing / 推进 lastSyncTime（见 B7-M1）。
	// F12-B1（第 12 轮）：先确认有可同步客户端再置位——此前无账号（空库/账号全删）或
	// 客户端不支持同步时，syncing 被置 true 后无人复位，后续每个 tick 在 `if s.syncing`
	// 处直接返回，时钟校准从启动起永久休眠、clockOffset 恒 0 且无任何错误日志。
	if client, ok := s.clients.AnyClient(); ok {
		if syncer, ok := client.(TimeSyncer); ok {
			go func() {
				offset, err := syncer.SyncServerTime()
				s.mu.Lock()
				defer s.mu.Unlock()
				s.syncing = false
				if err != nil {
					s.syncFailStreak++
					s.lastSyncFailAt = time.Now() // B9-03：失败落地即记录，退避 30s
					log.Printf("[scheduler] 时钟对齐失败（连续 %d 次）：%v", s.syncFailStreak, err)
					// B19-01（第 19 轮）：时钟失败时刻留档——幽灵窗口判定只读
					// syncFailStreak（≥3 且开放时间已过），本字段写而不读，与成功路径的
					// 清零对称保留（失败/恢复时刻留档，便于未来按时间差精细调参）。
					s.syncFailedWindow = time.Now()
					// B21-01（第 21 轮）：到达 3 次后只把校准偏差复位（回退到本地时钟），
					// 绝不在此清零 streak——旧实现同一临界区先 ++ 再清零，外部读取方
					// （WindowClosed 持同一把锁）永远读不到 3（值域恒 {0,1,2}），B19-01
					// 的时钟兜底判据实为不可达死代码（对应测试手动注入 3 恒假绿）。
					// 保留 streak 持续增长，幽灵窗口判据成为真实可达状态；同步成功时
					// 统一清零自愈（见下），瞬断 1 次只记 1 次、绝不误触发。
					if s.syncFailStreak >= 3 {
						s.clockOffset = 0
						log.Printf("[scheduler] 时钟对齐连续失败已达 %d 次，校准偏差已复位（回退到本地时钟）", s.syncFailStreak)
					}
					return
				}
				s.clockOffset = offset
				s.syncFailStreak = 0
				s.lastSyncFailAt = time.Time{}                        // B9-03：成功即清失败退避（瞬断不拖延后续校准）
				s.lastSyncTime = s.lastSyncStart                      // 只有成功才推进成功采样闸门
				s.syncFailedWindow = time.Time{}                      // B19-01：与失败写点对称（写而不读，留档自愈语义）
				log.Printf("[scheduler] 服务端时钟对齐成功，校准偏差: %v", offset)
			}()
			return
		}
	}
	// 无客户端或不支持时钟同步：本次不发起，复位在途标记，等账号就绪后再同步。
	// 此前此处直接 return，syncing 被置 true 后无人复位——空库部署的首个 300ms tick
	// 就让时钟校准链路永久休眠（F12-B1 根因）。
	s.mu.Lock()
	s.syncing = false
	s.lastSyncStart = time.Time{}
	s.mu.Unlock()
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
// 用户重新设定目标即"主动重新选它"：清空该账号 refused 标记（含库内持久化行）——
// 被手动退选的课程只有在用户重新设为目标时才被自动引擎重新接管（A2：绝不静默抢回）。
// B19-02（第 19 轮）定案：**绝不**清 done/full/rateLimited/inflight——done 是跨目标的
// 持久历史事实（重启恢复 RestoreDone 注入），重设目标清掉会把已成功课程重新提交；
// full/rateLimited/inflight 是本次窗口内的真实防轰炸/防双包状态，清了让自动链立刻重打
// 刚被平台拒绝的课。删账号路径的**全量清理**走专用 PurgeAccount（见下）。
func (s *Scheduler) SetTargetsForAccount(acct string, targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acctTargets[acct] = targets
	delete(s.refused, acct)
	// B19-02（第 19 轮）定案：不清 done/full/rateLimited/inflight——done 是跨目标的
	// 持久历史事实（RestoreDone 注入/手动报名 MarkDone 写入/落库 SaveSuccess），清掉会
	// 把已成功课程重新提交（TestRestoreDoneSkipsResubmit 固化契约）；full/rateLimited
	// 是真实满员/风控退避状态（自愈由快照解封/退避过期提供），清了让自动链立刻重打刚被
	// 平台拒绝的课（触发熔断）；inflight 防并发双发包（网络往返完成自清）。删账号的
	// 全量清理（含 acctData/tokenValid/relogin 族）归 PurgeAccount——管理员删除路径
	// handleAdminDeleteAccount 必须调它而非本方法。
	if s.store != nil {
		// B9-02：重设目标同步清空库内退选行——用户主动重新接管，退选标记不再需要。
		_ = s.store.DeleteRefused(acct)
	}
	s.rebuildCoursesForAccountLocked(acct, targets)
}

// PurgeAccount 全量清空指定账号在调度器中的一切状态（B19-02，第 19 轮）：
// 管理员删除账号后调用，保证重建的账号（同学生换绑/重登）绝不残留旧状态——
// done 残留会显示"重启恢复：已报名成功"、full/rateLimited 残留会让自动链静默跳过、
// inflight 残留会阻塞手动报名。与 DeleteAccount 事务（清库行）配成"内存+库"双清。
func (s *Scheduler) PurgeAccount(acct string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.acctTargets, acct)
	delete(s.done, acct)
	delete(s.full, acct)
	delete(s.rateLimited, acct)
	delete(s.inflight, acct)
	delete(s.refused, acct)
	delete(s.acctData, acct)
	delete(s.acctDataAt, acct)
	delete(s.tokenValid, acct)
	delete(s.reloginAt, acct)
	delete(s.reloginFail, acct)
	delete(s.relogging, acct)
	// 该账号课程状态行一并清除——不再为其生成任何显示（重启后重建账号从零开始）
	keep := s.state.Courses[:0]
	for _, c := range s.state.Courses {
		if c.Account != acct {
			keep = append(keep, c)
		}
	}
	s.state.Courses = keep
}

// RestoreTargets 重启恢复目标（B10-01）：与 SetTargetsForAccount 唯一区别是不清
// refused（内存 + 库行）——重启恢复的目标不是"用户主动重选"，若清库行会把已持久化的
// 手动退选记录删掉（B9-02 被恢复顺序抵消）。恢复顺序：RestoreDone → 循环 RestoreTargets
// → LoadRefused + RestoreRefused（main.go）。
func (s *Scheduler) RestoreTargets(acct string, targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acctTargets[acct] = targets
	s.rebuildCoursesForAccountLocked(acct, targets)
}

// rebuildCoursesForAccountLocked 重建指定账号的课程状态（需持有锁）：
// 删除该账号旧状态行，按目标重建；命中 done 置"重启恢复：已报名成功"。
func (s *Scheduler) rebuildCoursesForAccountLocked(acct string, targets []Target) {
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
		// B10-03：重设目标且该课仍被拒绝（用户退选）时，done 恢复成功文案会掩盖
		// 退选意图——refused 优先，强制 pending。
		if s.refusedHas(acct, t.ClassID) {
			status = "pending"
			result = "已手动退选（自动引擎不再接管，可重新设为目标恢复）"
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

// RestoreRefused 注入重启前已手动退选的 (账号, 课程) 记录（B9-02 + B10-01 顺序修正）：
// 必须**先于** SetTargetsForAccount 循环调用——后者（用户重设目标恢复路径）会
// `delete(s.refused, acct)` + `DeleteRefused(acct)` 清空该账号退选，若先循环再注入，
// 注入的标记被恢复路径覆盖、持久化行也被删（B10-01：B9-02 被重启顺序抵消）。
// main.go 已改为"先 LoadRefused+RestoreRefused，再逐账号 SetTargetsForAccount"。
func (s *Scheduler) RestoreRefused(refused map[string][]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for acct, ids := range refused {
		if s.refused[acct] == nil {
			s.refused[acct] = map[int]bool{}
		}
		for _, id := range ids {
			s.refused[acct][id] = true
		}
	}
	// 已恢复为目标的课若命中 refused（理论不应发生，防御注入），状态文案对齐手动退选。
	for i := range s.state.Courses {
		c := &s.state.Courses[i]
		if s.refusedHas(c.Account, c.ClassID) && c.Status == "pending" {
			c.Result = "已手动退选（自动引擎不再接管，可重新设为目标恢复）"
		}
	}
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
	st.TokenValid = s.tokenValidForLocked(acct)
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
// 与 maybeRelogin 用同一把 reloginMu 串行化，避免读到"即将写入"的半态。
func (s *Scheduler) TokenValidFor(acct string) bool {
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenValidForLocked(acct)
}

// tokenValidForLocked 计算指定账号 token 有效性（需持 s.mu 与 s.reloginMu）。
func (s *Scheduler) tokenValidForLocked(acct string) bool {
	return !s.tokenValid[acct] && !s.relogging[acct]
}

// AccountsWithTargets 返回当前所有已配置有效目标的账号列表（排序）。
// B10-04：列表必须排序——api 层管理员不带 ?account= 时取 targetAccts[0] 对齐
// "核心账号"，map 迭代无序会让每次刷新看到不同学生的课程（多账号部署下）。
func (s *Scheduler) AccountsWithTargets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for a, ts := range s.acctTargets {
		if len(ts) > 0 {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

// ElectivesSnapshotFor 返回指定账号的内存课程快照（40 秒内有效）。
// 若该账号暂无专属快照或已过期，则回退全局快照。
func (s *Scheduler) ElectivesSnapshotFor(acct string) (*zhidao.ElectivesData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// B22-01（第 22 轮）：目标账号（已配置目标）必须用该账号专属年级快照渲染，
	// 绝不无条件回退全局 lastData——probe() 只对 AccountsWithTargets() 遍历刷新
	// per-account 帧，有目标的账号 30s 内必有其专属帧；专属帧缺失 = 账号刚登录/
	// 探测尚未完成，此时回退返回的全局帧可能是首个注册账号（m.order[0]）的年级帧，
	// 混合年级部署下（高二 82 门/高三 1 门文档实证）会把错年级 1 门课渲染给高二学生，
	// 且永不触发本账号 ProbeForAccount——年级串线全开。这里返回 false 让
	// handleElectives 走 ProbeForAccount 真取该账号年级帧；无目标账号（仅浏览/手动
	// 报名）保持回退全局帧（快、无网络开销，且手动复核 CheckClassSelectable 不读全局帧）。
	if _, hasTargets := s.acctTargets[acct]; acct != "" && hasTargets {
		if d, ok := s.acctData[acct]; ok && d != nil {
			snappedAt := s.acctDataAt[acct]
			if !snappedAt.IsZero() && time.Since(snappedAt) <= snapshotTTL {
				return d, true
			}
		}
		return nil, false
	}
	if acct != "" && s.acctData != nil {
		if data, ok := s.acctData[acct]; ok && data != nil {
			if time.Since(s.acctDataAt[acct]) <= snapshotTTL {
				return data, true
			}
		}
	}
	if s.lastData == nil || time.Since(s.lastDataAt) > snapshotTTL {
		return nil, false
	}
	return s.lastData, true
}

// ProbeForAccount 使用指定账号的专属客户端执行课程探测并刷新该账号快照。
// 命中 token 失效（ErrUnauthorized）时触发该账号自动重登。
//
// MAJOR-D：账号不存在时返回明确错误，绝不回退 ProbeNow 直打教务上游——
// 否则 ?account= 对任意不存在账号可绕过调度器 30s 探测节流 + 账号枚举。
func (s *Scheduler) ProbeForAccount(acct string) (*zhidao.ElectivesData, error) {
	if s.clients == nil {
		return nil, errors.New("没有任何已登录账号")
	}
	client, ok := s.clients.ClientFor(acct)
	if !ok || client == nil {
		return nil, fmt.Errorf("账号 %s 未登录或不存在，无法探测课程", acct)
	}
	data, err := client.FindElectives()
	if err != nil {
		if errors.Is(err, zhidao.ErrUnauthorized) {
			s.maybeRelogin(acct)
		}
		return nil, err
	}
	if data != nil && len(data.Publishes) == 0 {
		log.Printf("[scheduler] 账号 %s 探测返回空课程快照（选课窗口关闭或学期无发布），按空数据处理", acct)
	}
	// B20-03（第 20 轮）：时间基准统一——探测时间戳写入侧改用对齐钟（与 tick 判读
	// `now.Sub(lastProbe)` / `ElectivesSnapshotFor` 的 `time.Since(acctDataAt)` / 提交节流
	// `submitIntervalFor` 同一时间基）。此前写入用本地 `time.Now()`、读用对齐钟，
	// 与 B16-M1 已消灭的"写入本地/读对齐"混用模式同族（偏差 ~640ms 对 2s/30s 节流与
	// `now.After(open)` 窗口判点无实质错误，但契约不自洽）。
	now := s.nowAligned()
	s.mu.Lock()
	if s.acctData == nil {
		s.acctData = make(map[string]*zhidao.ElectivesData)
		s.acctDataAt = make(map[string]time.Time)
	}
	s.acctData[acct] = data
	s.acctDataAt[acct] = now
	// B5-10（第 5 轮）：ProbeForAccount 只写该账号专属快照，不动全局 lastData/lastDataAt。
	// 管理员 ?account=A 穿透探测若写全局帧会污染全局快照（年级不同的帧），
	// 页面 ElectivesSnapshot 读全局帧时看到错年级课程——年级串线根因之一。
	// B6-04（第 6 轮）：这里不再写 lastProbe——lastProbe 是全局探测节流闸门（tick 用），
	// 管理员穿透探测 / 账号探测若写它，会让"全校正规探测"节流被旁路：开窗前管理员
	// 手动点一次课程页，就按下一次正规探测（30s→2s 临门盯守被吞掉），窗口开启后
	// tick 探测被节流闸门挡到 30s，黄金期提交失去即时确认。lastProbe 只归 probe()
	// 与 ProbeNow 管理（全校维度），账号级探测不影响全校节流。
	s.mu.Unlock()
	return data, nil
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

// HasProbed 调度器是否已产生至少一次探测（区分"从未探测"与"探测结果为空"）。
func (s *Scheduler) HasProbed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.lastProbe.IsZero()
}

// WindowOpened 返回当前窗口开启状态（以调度器实际探测结果为准）。
// 返回布尔值快照（历史上返回 *bool 裸指针，改为值拷贝防止指针悬空读-写竞态）。
func (s *Scheduler) WindowOpened() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.WindowOpened
}

// WindowClosed 返回窗口是否已关闭（探测到空快照且从未开过窗）。
// B19-01（第 19 轮）：除 B18-M1 的"至少开过窗 + 空快照 + 开放时间已过"主判据外，
// 追加"时钟连续失败 ≥3 且开放时间已过"兜底——从未开过窗的幽灵窗口（平台空快照）
// 无法靠主判据判定关闭，长期 2s 高频探测烧平台；时钟失败是"网络/平台异常"的可靠
// 信号，连续失败即视同关闭，挂起提交 + 探测降频（自愈由 syncFailStreak 归零提供）。
func (s *Scheduler) WindowClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.WindowClosed {
		return true
	}
	// B19-01（第 19 轮）：时钟连续失败 ≥3（平台不可达信号）→ 幽灵窗口兜底判定——
	// 从未开过窗的空快照 + 开放时间已过时，2s 高频探测/1s 提交只烧平台（熔断形态）。
	// 自愈由 syncFailStreak 归零（同步成功）提供。注意 syncFailedWindow 字段写而不读
	// （判据只用 syncFailStreak），已在 maybeSyncClock 注释注明，避免误读为死代码。
	// B21-01（第 21 轮）：streak 现在真实可达——maybeSyncClock 失败分支不再清零
	// （见下），连续失败 ≥3 后持续累计、同步成功才归零；WindowClosed 判据首次真实生效。
	if s.syncFailStreak >= 3 && !s.openTimeNow().IsZero() {
		return true
	}
	// B20-02（第 20 轮）：视同关闭的探测持续判定——开放时间已过 + 窗口从未开过（prevOpened
	// 恒 false，B19-01 时钟兜底覆盖不到）+ 探测返回空快照 ≥3 次：平台窗口从未开启/已关闭且
	// 从未被确认开过（B18-M1 主判据 requirement 不满足 + B19-01 时钟正常时兜底不触发），
	// 2s 探测/1s 提交恒高频轰炸 findElectivesData + 报名接口（防轰炸契约闭环缺口）。
	// 首次探测（acctDataAt 全空）不计数、不误伤；runs≥3 即连续三轮空快照确证"从未开过"，
	// 进入幽灵窗口挂起，探测/提交同步降频。窗口若真开、管理员热改开放时间，success 探测
	// 数据后势必推开始 open 实况、emptyRuns 归零自愈。
	if !s.openTimeNow().IsZero() && !s.state.WindowOpened && s.state.EmptyProbeRuns >= 3 {
		return true
	}
	return false
}

// emptyRunsFor 统计该开放时间下"空快照且未开窗"的连续探测轮数——由 probe() 在每次探测
// 结果入账时同步推进，判据与探测轮数对齐（无时钟依赖）。闭源：仅有 WindowClosed/probe 使用。
func (s *Scheduler) emptyRunsFor(open time.Time) int {
	_ = open
	return s.state.EmptyProbeRuns
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
	// B20-03（第 20 轮）：时间基准统一——与 ProbeForAccount 同款，写入侧用对齐钟
	now := s.nowAligned()
	s.mu.Lock()
	s.lastProbe = now
	s.lastData = data
	s.lastDataAt = now
	s.mu.Unlock()
	return data, nil
}

// tick 单次轮询：先按需探测刷新窗口状态与课程快照；窗口开启后按动态间隔持续提交。
// 探测与提交解耦：窗口开启后提交重试不受探测 30 秒节流限制（黄金期 250ms 高频冲刺）。
func (s *Scheduler) tick() {
	now := s.nowAligned()
	s.mu.Lock()
	last := s.lastProbe
	open := s.openTimeNow()
	s.mu.Unlock()

	s.maybePrewarm(now, open)
	s.maybeSyncClock(now)

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
	//   2) 对齐后的时间已过开窗点（openTimeNow）——兜底：平台在到点瞬间把课程列表拉空
	//      （熔断/学期异常）或探测恰好失败时，不依赖探测确认也放行提交，黄金期不容浪费。
	// 注意 WindowOpened 只在"探测成功且列表非空"时更新；探测失败或 Publishes 被平台熔断拉空时
	// 维持上一轮值，因此这里不会把已开启的窗口误判为关闭。
	// B11-A1（第 11 轮）：open 为零值（管理员 PUT open_time="" 显式解除窗口机制，F7-02）
	// 时恒满足 !now.After(open) → 提交循环永续放行。此时窗口机制已被显式解除，
	// 但提交仍可能对"已满员/已成功"目标反复刷平台报名接口——挂起提交，绝不放行。
	if open.IsZero() {
		return
	}
	if !opened && !now.After(open) {
		return
	}
	// B18-M1（第 18 轮）：窗口已确认关闭（探测到空快照且开放时间已过，state.WindowClosed）
	// 时挂起提交——平台对关闭后的报名返回 code=1"无效的课程ID"（真实关闭文案），不在
	// isWindowClosedError 的"关闭/未开启/报名时间/已结束"匹配集合内 → 不记 full → 走实时
	// 复核 → 窗口关闭后 countList 空 → IsClassFull 报"课程无人数数据" → 下个 tick 重打
	// SelectClass，对未成功目标形成每 1s（黄金期 250ms）永续轰炸（防轰炸 C-3 契约缺口）。
	// 以探测状态而非脆弱错误文案作为关闭判定：WindowClosed 已确认即挂起提交，与 B11-A1
	// open 零值守卫并列，黄金期（开窗瞬间 WindowClosed=false）绝不影响。
	if s.WindowClosed() {
		return
	}
	// 提交重试闸门：开窗黄金期 250ms 高频冲刺，平时 1 秒
	s.mu.Lock()
	lastSubmit := s.lastSubmit
	s.mu.Unlock()
	submitInterval := s.submitIntervalFor(now, open)
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
	// F12-B2（第 12 轮）：探测单飞守卫——probe() 的最长耗时是 FindElectives 网络往返
	// （15s 超时），HTTP 侧 /api/electives 在快照过期时并发的 ProbeForAccount/ProbeNow
	// 会与 tick 探测同时打上游，N 账号部署下开窗全期形成 N+1 并发 findElectivesData。
	// probing 在持 s.mu 时置位，保证"置位-检查"原子（B11-A1 零值守卫同款窗口）。
	// 命中单飞直接放弃本次探测：最长推迟一个 tick（300ms），临门/黄金期无实质损失。
	s.mu.Lock()
	if s.probing {
		s.mu.Unlock()
		return
	}
	s.probing = true
	s.mu.Unlock()

	// B20-03（第 20 轮）：时间基准统一——与 ProbeForAccount/ProbeNow 同款，写入侧用
	// 对齐钟（判读侧 tick `now.Sub(lastProbe)` 已在对齐钟下，避免两套时间基混用）
	now := s.nowAligned()
	// 独立维护：并发探测所有已配置目标的账号，独立刷新各自年级的专属快照。
	// F12-B2 的 probing 单飞只保护"probe() 主体（任意客户端 FindElectives）"，这里
	// 每账号各起 goroutine 调 ProbeForAccount 不受保护——临门/开窗期 probeIntervalNear
	// 2s 周期触发时，N 账号部署每 2s 变 N+1 并发 findElectivesData 直打上游，与
	// "访问过于频繁 1 分钟熔断"实证契约冲突（F17-01，第 17 轮 MAJOR）。
	// 修复：probeSem 结构化信号量（cap 4）封顶 per-account 并发——峰值从 N 降到 4，
	// 跨批（2s 周期短于一批耗时）受同一信号量约束绝不叠加；全局 FindElectives 主体
	// 不受影响（probe() 在 per-account 全部入场后执行）。
	// ponytail: cap=4 常驻，若平台放宽熔断或账号数 >50 再调。
	for _, a := range s.AccountsWithTargets() {
		go func(acct string) {
			// F17-01（第 17 轮）：结构化信号量封顶 per-account 探测并发——峰值从 N
			// 降到 4；跨批（2s 周期短于一批耗时）由同一信号量约束，绝不叠加。
			s.probeSem <- struct{}{}
			defer func() { <-s.probeSem }()
			_, _ = s.ProbeForAccount(acct)
		}(a)
	}
	client, ok := s.clients.AnyClient()
	if !ok {
		s.mu.Lock()
		s.probing = false
		s.mu.Unlock()
		return // 尚无账号登录，安静等待
	}
	data, err := client.FindElectives()
	if err != nil {
		s.mu.Lock()
		s.probing = false
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
	s.probing = false
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
	// B18-M1（第 18 轮）：先捕获上一轮 WindowOpened 状态，再覆写本轮——关闭判定需要
	// "至少开过窗"作为前提（见下），若在覆写后读取 prevOpened 拿到的恒是本次 opened 值。
	prevOpened := s.state.WindowOpened
	s.state.WindowOpened = opened
	// 窗口关闭判定：快照为空（code:0 空 publishes，平台选课窗口关闭特征）
	// 且开放时间已过 → 明确标记窗口已关闭，日志输出供排查"课程为空"原因。
	// C-3（第 3 轮）：去掉 !prevWindowOpened 条件——"开过再关"是窗口关闭最常见场景，
	// 若只认"从未开过窗"则开过再关后 WindowClosed 恒 false，probeIntervalFor 的
	// "开放时间已过 + WindowClosed → 降回 30s"分支永不命中，窗口关闭后仍 2s 高频探测。
	// B18-M1（第 18 轮）：只在"至少开过窗"（opened 曾经为 true）后才标记关闭——
	// 否则未开窗即空快照（学期无发布/平台异常）会误标已关闭，tick 提交守卫按
	// WindowClosed 挂起提交，把"还没开窗待开"误停成"永不提交"（开窗瞬间探测推进、
	// 黄金期全停摆）。已开过窗再关 = 窗口关闭的实质语义，未开过不算关闭。
	s.state.WindowClosed = prevOpened && !opened && len(data.Publishes) == 0 && now.After(s.openTimeNow())
	// B20-02（第 20 轮）：探测量变入账——空快照 + 从未开窗 + 开放时间已过 → 连续轮数 +1；
	// 否则（非空快照 / 本轮被确证开窗 / 未到开放时间）归零。窗开 shift probe 会自然重置。
	// 注意绝不触碰 state.WindowClosed（由 B18-M1/B19-01 判据独占）：这里只维护量变计数，
	// WindowClosed() 读取它做"视同关闭"兜底——不写 state.WindowClosed 避免误标真实关闭。
	// B21-02（第 21 轮）：入账另加"已过开窗点 10s 裕量"——平台在开窗前会预清空 publishes
	// （F7-01 记录的真实现象，切学期/数据迁移），若开窗瞬间清空过渡态持续 ≥6 秒（3 次探测
	// × 2s 临门间隔），旧判据会在真实窗口已开时误挂起黄金期提交+降频探测；以"开窗点后
	// 10s 内不计空快照轮数"错开过渡态，窗口真开（10s 黄金期结束）后连续空才确证幽灵窗口。
	if !opened && len(data.Publishes) == 0 && now.After(s.openTimeNow().Add(10*time.Second)) {
		s.state.EmptyProbeRuns++
	} else {
		s.state.EmptyProbeRuns = 0
	}
	// B10-05：prevWindowOpened 是写而不读的死字段（C-3 已去掉 !prevWindowOpened 条件），
	// 删除避免误导后续维护者以为还有清提交闸门的路径。
	log.Printf("[scheduler] 探测成功：%d 个发布，窗口状态 %v（已关闭 %v）", len(data.Publishes), opened, s.state.WindowClosed)
	s.mu.Unlock()
}

// reloginInterval 重登节流：30 秒内最多重登一次（与探测节流同频，避免频繁登录触发平台限流）。
// 这是同一账号重登的最短间隔；排它控制交给下面的失败退避表（连续失败时间隔指数拉长）。
const reloginInterval = 30 * time.Second

// 重登失败退避（防平台锁号的最后防线）：连续失败 n 次后，距离下次重试为 backoffMin << n，
// 封顶 backoffMax，所以 Vision 服务持续故障时登录频率只会越来越低，绝不会把账号刷到锁号。
const (
	backoffMin     = 30 * time.Second
	backoffMax     = 10 * time.Minute
	maxReloginFail = 5 // 30s * 2^5 = 960s > 10m，封顶 5 次防溢出
)

// reloginBackoff 重登失败退避：连续失败 1 次 = 基础 30s，之后每次翻倍，封顶 10 分钟。
func (s *Scheduler) reloginBackoff(n int) time.Duration {
	if n <= 0 {
		return backoffMin
	}
	if n > maxReloginFail {
		return backoffMax
	}
	d := backoffMin << (n - 1) // 首次失败即基础间隔，之后翻倍
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
	if s.reloginFail[acct] < maxReloginFail {
		s.reloginFail[acct]++
	}
	log.Printf("[scheduler] 账号 %s 触发自动重登（原因：教务 token 失效，连续失败 %d 次）", acct, s.reloginFail[acct])
	s.relogging[acct] = true // 标记重登中
	// F12-B3：tokenValid 失效标记在此处置位、且与决策同持两把锁——发起重登即"token
	// 已知失效"，本就不该等 goroutine 开头再补写。此前 goroutine 开头才置位：用户手动
	// 登录成功（MarkTokenValid 清 tokenValid/relogging/reloginFail）与在途重登并发时，
	// 置位会覆写已被清理的标记，前端 /state 短暂回"已失效·自动恢复中"后 jitter。
	s.tokenValid[acct] = true
	s.mu.Unlock()

	go func() {
		relogged, err := s.clients.Relogin(acct)
		s.mu.Lock()
		delete(s.relogging, acct) // 清重登中标记（失败也清，才能再试）
		// B21-03（第 21 轮）：账号已删竞态防线——B18-M2/B20-01 只护自动链/手动路径，
		// 重登成功分支仍缺同款复核：管理员 DeleteAccount（清凭据表+Accounts.Remove）与
		// 在途 Login（Vision 最坏 2 分钟）竞态，成功分支会无条件写回 tokenValid/reloginAt
		// 内存态 + UpdateIDToken 落库把已删账号新 token 写回 credentials 表（重启后 Restore
		// 重建客户端、凭据幽灵复活）。先复核客户端仍存在：已删则整个成功分支（含内存写与
		// 落库）静默放弃，只清 relogging 标记。
		if _, ok := s.clients.ClientFor(acct); !ok {
			s.mu.Unlock()
			log.Printf("[scheduler] 账号 %s 重登完成时已被删除，放弃状态写回与新 token 落库", acct)
			return
		}
		if err == nil && relogged {
			delete(s.reloginFail, acct)    // 成功清零失败计数，退避表归零
			s.reloginAt[acct] = time.Now() // 成功后刷新完成时间，维持 30s 基础防抖限频
			s.tokenValid[acct] = false
			// 新 token 落库（持久化，重启后恢复不丢）
			var newTok string
			if client, ok := s.clients.ClientFor(acct); ok {
				if tok := client.Token(); tok != "" {
					newTok = tok
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
			log.Printf("[scheduler] 账号 %s 教务 token 已自动重登恢复（新 token %s...）", acct, maskedToken(newTok))
			return
		}
		// 失败/未重登：保持失效标记（tokenValid 仍 true），前端显示"已失效·自动恢复中"，
		// 不再误报"有效"（安全审计 MINOR 7）。
		// C1 修复（第 3 轮）：失败后 reloginFail 保留本次发起时递增到的次数，杜绝无条件复位 1——
		// 否则计数恒 1→2→1→2 振荡，指数退避表永不增长，Vision 持续故障时退避恒为 30s，
		// 平台锁号防线被击穿。失败次数只会随成功清零（上面成功分支 delete），
		// 由 maybeRelogin 的退避窗口自然隔开下一次失败尝试。
		s.mu.Unlock()
		if err != nil {
			log.Printf("[scheduler] 账号 %s 自动重登失败: %v", acct, err)
		} else {
			log.Printf("[scheduler] 账号 %s 无保存账密，无法自动重登（请手动重新登录）", acct)
		}
	}()
}

// MarkTokenValid 手动登录成功时恢复该账号的 token 有效性标记（B5-01）：
// tokenValid 唯一的自动清零路径是 maybeRelogin 自动重登成功分支；若自动重登
// 长期失败（Vision 故障 / 无保存账密"请手动重新登录"），用户手动登录成功后仍显示
// "已失效·自动恢复中"无恢复路径。手动登录成功路径（issueSession）调用本方法，
// 清 tokenValid 失效标记与重登失败计数，前端 /state 立即恢复"有效"。
func (s *Scheduler) MarkTokenValid(acct string) {
	// F12-B3：MarkTokenValid 与 TokenValidFor/maybeRelogin 对齐锁序（reloginMu→s.mu）——
	// 此前只持 s.mu：手动登录成功（issueSession 恢复路径）会清 tokenValid/reloginFail/
	// relogging，但"发起决策"那段仍在 reloginMu 下、且 goroutine 开头曾把 tokenValid
	// 覆写回 true，两条路径无法串行化 → 手动登录与在途自动重登并发时状态闪动。
	// 现在同步取锁，decision 段内完成清除，与重登发起/查询有效性串行，杜绝半态读。
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokenValid, acct)
	delete(s.reloginFail, acct)
	delete(s.relogging, acct)
}

// MaybeRelogin 导出别名：供 api 层在手动报名/退选命中 ErrUnauthorized 时触发重登
// （B8-M7，与自动链路径对称），命名上明确它是幂等门控的。
func (s *Scheduler) MaybeRelogin(acct string) { s.maybeRelogin(acct) }

// reloginResult 重登结果（异步回传到 tick 主循环统一处理）。
type reloginResult struct {
	acct     string
	relogged bool
	err      error
}

// maskedToken 脱敏打印教务 token：只显示前 8 位，绝不输出完整值。
// m10 修复：长度 ≤8 的短 token 不足以掩盖身份，一律输出 "***"。
func maskedToken(tok string) string {
	if len(tok) > 8 {
		return tok[:8]
	}
	return "***"
}

// submitAll 并发提交所有账号所有发布的目标链（每链独立 goroutine，链内按人数确认满员依次退避）。
func (s *Scheduler) submitAll() {
	// MAJOR-F 修复：本地时钟写 lastSubmit 会与对齐时钟判定（tick 内 submitIntervalFor）
	// 产生基准混用——统一以对齐时钟记录提交时刻，黄金期 250ms 冲刺间隔判定不再失真。
	s.mu.Lock()
	s.lastSubmit = s.nowAlignedLocked()
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
	if len(chains) == 0 {
		// M-3 修复：冷启动（尚无目标）时每 tick 静默空转，运维无法区分
		// 「没目标所以没提交」与「配置丢失/加载失败」。一次性警告日志点破真相。
		if !s.warnedNoTargets {
			s.warnedNoTargets = true
			log.Printf("[scheduler] 当前没有任何账号目标课程，提交链未启动（请先在选课大厅设置目标）")
		}
		return
	}
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
			// B23-03（第 23 轮）：token 已知失效且重登进入退避期（relogging 已被失败路径清掉、
			// reloginAt 未过退避窗口）时，链顶只有 relogging 短路挡不住——每 tick 仍对每门目标
			// 真实发起 SelectClass（必然 code=-1"教务令牌失效"）并每题 AppendLog，烧平台请求
			// 额度 + 日志表堆积。tokenValidForLocked（tokenValid=true || relogging=true）即
			// "已知失效"，重登成功/手动登录后清 false（B21-01 语义）才恢复提交。前端 /state
			// 只读 token_valid 显示"已失效·自动恢复中"，本链不发请求、状态保持原样。
			if !s.tokenValidForLocked(acct) {
				s.mu.Unlock()
				return
			}
			// 已成功：本发布目标完成，终止
			if s.doneHas(acct, t.ClassID) {
				s.mu.Unlock()
				return
			}
			// 平台风控退避中：跳过本课程
			now := s.nowAlignedLocked()
			if s.isRateLimitedLocked(acct, t.ClassID, now) {
				s.mu.Unlock()
				continue
			}
			// 手动退选后被用户拒绝的课程：自动引擎绝不抢回（A2），直到用户重新设为目标
			if s.refusedHas(acct, t.ClassID) {
				s.mu.Unlock()
				continue
			}
			s.releaseFullIfFreedLocked(acct, t.ClassID)
			if s.fullHas(acct, t.ClassID) {
				s.mu.Unlock()
				continue
			}
			// 快照人数确认满员（selected >= max）→ 记入 full，跳过本备选
			if s.classFullInSnapshot(acct, t.ClassID) {
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				continue
			}
			// 手动提交在飞（TryAcquireSubmit 占用 inflight 位）：自动链必须跳过，
			// 绝不并发双发包（评审 CRITICAL：inflight 去重落地）。
			if s.inflightHas(acct, t.ClassID) {
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
				// B18-M2（第 18 轮）：写成功/落库前复核账号仍存在——管理员 DeleteAccount
				// （先清 credentials/accounts/targets/success 表 + Accounts.Remove）与在飞
				// spawnChain 网络往返（SelectClass 最长 15s）竞态时，本链在删除完成后才返回
				// 成功，若不复核会 SaveSuccess/SaveRefused 把已删账号的 success 行写回，
				// 重启后重新登录被 RestoreDone 恢复成"已报名成功"假状态。重登完成路径已有
				// 同款 ClientFor 复核（891 行），提交链成功分支此前漏了同一防线。
				if _, ok := s.clients.ClientFor(acct); !ok {
					log.Printf("[scheduler] 账号 %s 已被删除，放弃写成功落库", acct)
					s.mu.Unlock()
					return
				}
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
			// 平台风控退避：识别到"频繁"或 429 相关错误，为该课程设置 30s 退避，跳过轰炸
			if isRateLimitError(err) {
				s.markRateLimitedLocked(acct, t.ClassID, 30*time.Second)
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "触发平台风控退避 30 秒: "+err.Error())
				if s.store != nil {
					s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 触发平台风控退避 30s: "+err.Error(), false)
				}
				s.mu.Unlock()
				return
			}
			// 平台对"选课窗口已关闭"的报名请求返回 code=1 错误（窗口关闭后课程列表已清空）。
			// 此时课程已无法再报，直接按满员处理记入 full 集合，
			// 避免每个 tick 都带着失败状态反复刷平台报名接口（窗口关闭后的最后一层防线）。
			if isWindowClosedError(err) {
				s.markFullLocked(acct, t)
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
			// C-4（第 3 轮）：复核前主动释放 s.mu——此前整段网络请求（最长 15 秒）都攥着
			// 全局锁，黄金冲刺期里其它账号的探测/提交/时钟对齐全被锁死；锁外复核完再回锁收尾。
			// F13-m2（第 13 轮）：锁内 SQLite 写（AppendLog/SaveSuccess 等，SetMaxOpenConns=1
			// 串行）只发生在持锁段、不跨此网络段——黄金期不因 DB 写停顿网络往返；持锁写
			// 窗口仅成功分支两行（微秒级），彻底消除需独立 DB goroutine，边际不动（观察项）。
			s.mu.Unlock()
			full, cErr := s.classFullRealtime(acct, t.ClassID)
			s.mu.Lock()
			// B19-03（第 19 轮）：实时复核命中 token 失效（学生数接口同样鉴权）——
			// 与 SelectClass 分支对称触发自动重登（ErrUnauthorized 才是"重登中"语义），
			// 否则本次失败被当普通失败处理、下个 tick 又重打报名接口（token 已失效的
			// 报名必然再失败），失效恢复路径被延迟到探测/手动路径才发现。
			if cErr != nil && errors.Is(cErr, zhidao.ErrUnauthorized) {
				s.mu.Unlock()
				s.maybeRelogin(acct)
				s.mu.Lock()
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "教务令牌失效，自动重登中")
				if s.store != nil {
					s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 实时复核命中 token 失效，自动重登中", false)
				}
				s.mu.Unlock()
				return
			}
			if cErr == nil && full {
				// B23-01（第 23 轮）：锁外复核窗口（最长 15s）内手动路径可能已抢到 inflight 位
				// 并 MarkDone 置 done+success（用户真实报名成功）——此时"确证满员"分支若直接
				// markFullLocked 会把 success 覆盖成"failed/已满员"并追加一条假"已满员"日志，
				// 与紧邻的"未现满员"分支（下方 doneHas 复核"绝不覆盖胜利状态"）不对称。
				// done 一旦置位（手动成功），满员分支必须让位，绝不覆盖胜利状态。
				if s.doneHas(acct, t.ClassID) {
					s.mu.Unlock()
					return
				}
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				continue
			}
			// 实时复核未现满员（网络抖动/人未满但报名被拒）：保留失败状态，终止本链，下个 tick 重试。
			// 复核期间锁被释放，可能已被手动报名并 MarkDone 置成功——绝不覆盖胜利状态。
			if s.doneHas(acct, t.ClassID) {
				s.mu.Unlock()
				return
			}
			s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", err.Error())
			if s.store != nil {
				s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": "+err.Error(), false)
			}
			s.mu.Unlock()
			return
		}
	}()
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "频繁") || strings.Contains(msg, "429") || strings.Contains(msg, "稍后重试")
}

// isWindowClosedError 平台在选课窗口关闭后对报名请求的返回特征（code=1 且提示已关闭/未开启）。
// 与"课程满员"同样不可再报，调度器按满员记录避免窗口关闭后无限轰炸报名接口。
func isWindowClosedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "关闭") || strings.Contains(msg, "未开启") ||
		strings.Contains(msg, "报名时间") || strings.Contains(msg, "已结束")
}

func (s *Scheduler) isRateLimitedLocked(acct string, classID int, now time.Time) bool {
	m, ok := s.rateLimited[acct]
	if !ok {
		return false
	}
	until, ok := m[classID]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(m, classID)
	return false
}

// B16-M1（第 16 轮）：退避截止基准与读侧统一为对齐钟——此前用本地钟 time.Now() 写入、
// spawnChain 用 nowAlignedLocked() 判期，两套时间基（实测相差 ~640ms）边界同一语义。
// 读侧 isRateLimitedLocked 已用 nowAlignedLocked()（1021 行），写入必须同源，语义自洽。
func (s *Scheduler) markRateLimitedLocked(acct string, classID int, d time.Duration) {
	if s.rateLimited[acct] == nil {
		s.rateLimited[acct] = map[int]time.Time{}
	}
	s.rateLimited[acct][classID] = s.nowAlignedLocked().Add(d)
}

// classFullInSnapshot 快照人数确认满员（需持锁）。优先匹配该账号专属快照，无快照时回退全局快照。
func (s *Scheduler) classFullInSnapshot(acct string, classID int) bool {
	if acct != "" && s.acctData != nil {
		if d, ok := s.acctData[acct]; ok && d != nil {
			for _, p := range d.Publishes {
				for _, c := range p.Classes {
					if c.ID == classID {
						return c.MaxCount > 0 && c.SelectedCount >= c.MaxCount
					}
				}
			}
		}
	}
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

// releaseFullIfFreedLocked 快照显示不满时解除 full 标记并回 pending（需持锁）。
// 只要快照显示有名额空余（如其他同学退选），立即解除 full 标记，黄金期 250ms 冲刺立即捡漏 (CRITICAL C1)。
// C-3 守卫（第 3 轮）：只有快照**明确**显示该课程名额空余才解封——
// 空快照（窗口关闭后平台清空课程列表）或快照中查不到该课程（无法判断）一律保持 full 不解封，
// 否则窗口关闭后 spawnChain 每个 tick 都因 full 被解封重新打报名接口（窗口关闭防轰炸残留）。
func (s *Scheduler) releaseFullIfFreedLocked(acct string, classID int) {
	if !s.fullHas(acct, classID) {
		return
	}
	data := s.acctData[acct]
	if data == nil {
		data = s.lastData
	}
	if data == nil || len(data.Publishes) == 0 {
		// 无快照或快照为空（窗口关闭特征）：无法确认余量，保持 full 不解封
		return
	}
	for _, p := range data.Publishes {
		for _, c := range p.Classes {
			if c.ID == classID {
				// 明确有余量才解封；课程不在快照中（未知）也保持 full
				if c.MaxCount > 0 && c.SelectedCount >= c.MaxCount {
					return
				}
				delete(s.full[acct], classID)
				idx := s.statusIndexLocked(acct, classID)
				if idx >= 0 {
					s.state.Courses[idx].Status = "pending"
					s.state.Courses[idx].Result = ""
				}
				return
			}
		}
	}
	// 课程不在快照中：无法判断，保持 full（保守不解封）
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

// refusedHas 用户是否已手动拒绝（退选）该课程（需持锁）。
func (s *Scheduler) refusedHas(acct string, classID int) bool {
	m, ok := s.refused[acct]
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

// CheckClassSelectable 手动报名前的服务端复核（评审 M7）：
// 基于该账号最近快照判定课程是否可报名——课程所在发布窗口未开放或课程已满员时
// 提前拒绝并返回友好原因，避免无谓打教务平台拿生硬错误码。
// 快照缺失（从未探测）或课程不在快照中（无法判定）时放行，由平台最终把关。
// B18-m1（第 18 轮）：过期快照（超过 snapshotTTL 未刷新）一律按"无快照"放行——
// 快照过期的判定依据是 acctDataAt 时间戳（与 ElectivesSnapshotFor 同源），
// 不使用 acct 外的全局 lastData 兜底（跨年级帧可为任意账号，无参考价值）。
// 过期快照放行语义：不拿旧数据拦用户真实操作（名额/窗口可能已变化），交给平台把关，
// 与 ElectivesSnapshotFor 的过期快照回退语义对齐。
func (s *Scheduler) CheckClassSelectable(acct string, classID int) (reason string, selectable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var data *zhidao.ElectivesData
	var fresh bool
	if acct != "" && s.acctData != nil {
		if d, ok := s.acctData[acct]; ok && d != nil {
			data = d
			fresh = s.acctDataAt[acct] != (time.Time{}) && time.Since(s.acctDataAt[acct]) <= snapshotTTL
		}
	}
	if data == nil || !fresh {
		return "", true // 无快照或已过期：无法复核，放行交给平台
	}
	for _, p := range data.Publishes {
		for _, c := range p.Classes {
			if c.ID != classID {
				continue
			}
			if !p.InDateRange {
				return "选课窗口未开放，暂不能报名", false
			}
			if !c.CanSelect || (c.MaxCount > 0 && c.SelectedCount >= c.MaxCount) {
				return "该课程已满员或不可选", false
			}
			return "", true
		}
	}
	return "", true // 课程不在快照中：交给平台返回具体业务错误
}

// TryAcquireSubmit 尝试获取对指定账号课程的提交排他锁（在飞互斥）。
// 若当前正在提交，返回 false；若成功获取，返回安全释放函数和 true。
func (s *Scheduler) TryAcquireSubmit(acct string, classID int) (release func(), ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight[acct] == nil {
		s.inflight[acct] = make(map[int]bool)
	}
	if s.inflight[acct][classID] {
		return nil, false
	}
	s.inflight[acct][classID] = true
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.inflight[acct], classID)
			s.mu.Unlock()
		})
	}, true
}

// MarkDone 手动或外部操作成功后同步调度器状态：记入 done、清 full 与退避、置 success 状态并持久化。
// 同步清理 inflight 位：手动报名成功前占用的提交锁位必须释放，否则下个自动链/手动操作永久 409。
// B20-01（第 20 轮）：与 spawnChain 成功分支同款防线——管理员 DeleteAccount（先清凭据/库行 +
// Accounts.Remove）与在飞手动报名（SelectClass 最长 15s）竞态时，删除完成后本请求才返回成功，
// 若不复核会把已删账号的 success 行写回，重启后重新登录被 RestoreDone 恢复成"已报名成功"假状态
// （B18-M2 在自动链已根治，手动路径同样竞态整链开放）。账号已删则静默放弃落库，绝不写回。
func (s *Scheduler) MarkDone(acct string, classID int, courseName, msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// B20-01：删除账号与在飞手动报名竞态防线——账号不再存在于客户端注册表即视为已删，
	// 放弃全部状态写入（真实平台报名已发生，但账号已删，写回只会制造幽灵 success 行）。
	if _, ok := s.clients.ClientFor(acct); !ok {
		log.Printf("[scheduler] 账号 %s 已被删除，放弃手动报名落库", acct)
		return nil
	}
	if s.done[acct] == nil {
		s.done[acct] = make(map[int]bool)
	}
	s.done[acct][classID] = true
	// B5-07（第 5 轮）：手动报名成功同样解除 refused——用户手动重选（成功）即表达
	// "我要这门课"，自动引擎应恢复接管（此前 refused 只在 SetTargetsForAccount 清空，
	// 手动重选成功但未重设目标时 refused 卡死，窗口重开后自动引擎永久跳过该课）。
	if s.refused[acct] != nil {
		delete(s.refused[acct], classID)
	}
	if s.inflight[acct] != nil {
		delete(s.inflight[acct], classID)
	}
	if s.full[acct] != nil {
		delete(s.full[acct], classID)
	}
	if s.rateLimited[acct] != nil {
		delete(s.rateLimited[acct], classID)
	}
	idx := s.statusIndexLocked(acct, classID)
	if idx >= 0 {
		s.state.Courses[idx].Status = "success"
		s.state.Courses[idx].Result = msg
	} else if courseName != "" {
		s.state.Courses = append(s.state.Courses, CourseStatus{
			Account:    acct,
			ClassID:    classID,
			CourseName: courseName,
			Status:     "success",
			Result:     msg,
		})
	}
	if s.store != nil {
		_ = s.store.SaveSuccess(acct, classID)
		_ = s.store.AppendLog(acct, classID, "select", msg, true)
	}
	log.Printf("[scheduler] 账号 %s 课程 %d 手动标记成功: %s", acct, classID, msg)
	return nil
}

// RemoveDone 手动退选成功后同步调度器状态：从 done 移除、置 pending 状态并记日志。
// 同步清理 inflight 位：退选进行中占用的提交锁位必须释放（M5）。
// 同时记入 refused 集合并置"已用户退选"文案：后台 spawnChain 从此对该课程绝不再自动
// 接管——用户手动退出的课，自动引擎下一 tick（≤1s）就抢回是错误行为（第 4 轮 MAJOR A2），
// 只有用户重新把它设为目标（SetTargetsForAccount 清空 refused）才恢复自动接管。
func (s *Scheduler) RemoveDone(acct string, classID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// B20-01（第 20 轮）：与 MarkDone 同款防线——账号已删时手动退选成功同样不能写回
	// refused 行（DeleteAccount 全量清理 + PurgeAccount 移除后的幽灵 refused 行会让
	// 重新登录的账号被 RestoreRefused 恢复成"已退选"，自动引擎永久跳过该课）。
	if _, ok := s.clients.ClientFor(acct); !ok {
		log.Printf("[scheduler] 账号 %s 已被删除，放弃手动退选落库", acct)
		return nil
	}
	if s.done[acct] != nil {
		delete(s.done[acct], classID)
	}
	if s.inflight[acct] != nil {
		delete(s.inflight[acct], classID)
	}
	if s.full[acct] != nil {
		delete(s.full[acct], classID)
	}
	if s.refused[acct] == nil {
		s.refused[acct] = map[int]bool{}
	}
	s.refused[acct][classID] = true
	idx := s.statusIndexLocked(acct, classID)
	if idx >= 0 {
		s.state.Courses[idx].Status = "pending"
		s.state.Courses[idx].Result = "已手动退选（自动引擎不再接管，可重新设为目标恢复）"
	}
	if s.store != nil {
		// B8-M2（第 8 轮）：删除 success 行——否则重启后该课被 RestoreDone 恢复成
		// "已报名成功"，用户当日的退选决定被静默撤销（与 CLAUDE.md 契约文档对齐）
		_ = s.store.DeleteSuccess(acct, classID)
		// B9-02（第 9 轮）：持久化 refused——此前只写内存，重启后 refused 全丢，
		// SetTargetsForAccount（重启恢复路径）会 delete(s.refused, acct)，
		// 自动引擎把用户手动退选掉的课当新目标重新抢回，退选意图丢失。
		_ = s.store.SaveRefused(acct, classID)
		_ = s.store.AppendLog(acct, classID, "exit", "手动退选成功（自动引擎不再接管，重新设为目标可恢复）", true)
	}
	log.Printf("[scheduler] 账号 %s 课程 %d 已手动退选，记入 refused——自动引擎不再接管", acct, classID)
	return nil
}

// RemoveFull 外部手动或快照更新时解除满员标记。
func (s *Scheduler) RemoveFull(acct string, classID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.full[acct] != nil {
		delete(s.full[acct], classID)
	}
	idx := s.statusIndexLocked(acct, classID)
	if idx >= 0 && s.state.Courses[idx].Status == "failed" {
		s.state.Courses[idx].Status = "pending"
		s.state.Courses[idx].Result = ""
	}
}
