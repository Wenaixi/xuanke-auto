package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"xuanke-auto/backend/internal/zhidao"
)

// Target 目标课程（同发布多门备选，Priority 越小越先提交）。
// PublishName/BeginDate 发布元数据（平台快照带入，随目标持久化）：
// 窗口关闭后 /electives 空发布、前端分组所需映射丢失，这两列保证 /state.courses
// 自带日期/发布名，分组与展示不依赖 /electives（关闭≠数据消失）。
type Target struct {
	PublishID   int    `json:"publish_id"`
	ClassID     int    `json:"class_id"`
	CourseName  string `json:"course_name"`
	Priority    int    `json:"priority"`
	PublishName string `json:"publish_name,omitempty"`
	BeginDate   string `json:"begin_date,omitempty"`
}

// CourseStatus 单课程任务状态。
type CourseStatus struct {
	Account     string `json:"account"`
	PublishID   int    `json:"publish_id"`
	ClassID     int    `json:"class_id"`
	CourseName  string `json:"course_name"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"` // pending|in_range|submitted|success|failed
	Result      string `json:"result"`
	PublishName string `json:"publish_name,omitempty"` // 发布名（发布元数据透传，窗口关闭仍可显示）
	BeginDate   string `json:"begin_date,omitempty"`   // 发布日期（YYYY-MM-DD 前缀，窗口关闭仍可分组）
}

// SchedulerState 对外状态快照。
type SchedulerState struct {
	// OpenTime 当前账号的"预计开放时间"——来自该账号自己探测识别的 beginTimes
	//（全校共享同一开窗时刻）；识别不到 = 未知（零值 + OpenTimeKnown=false），
	// 前端展示"未识别到开放时间"，绝不显示编造时间。
	OpenTime      time.Time      `json:"open_time"`
	OpenTimeKnown bool           `json:"open_time_known"` // 是否已识别到开放时间（平台 beginTimes 自动识别）
	WindowOpened  bool           `json:"window_opened"`
	WindowClosed  bool           `json:"window_closed"` // 探测为空快照且从未开过窗 = 选课窗口已关闭
	TokenValid    bool           `json:"token_valid"`   // 当前账号教务 token 有效性（有效=true）
	Courses       []CourseStatus `json:"courses"`
	// EmptyProbeRuns "空快照且从未开窗"的连续探测轮数——WindowClosed
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
// 注意：仅测试锚定的便利包装（内部自取 open 单快照）——生产 tick 刻意用
// probeIntervalForOpen(now, open) 传 tick 开头取一次的快照（热改亚毫秒读取一致），
// 绝不在此再取一次 open。13 处测试用它测"零值 open 降频"等语义，保留有测试价值。
func (s *Scheduler) probeIntervalFor(now time.Time) time.Duration {
	// 零值/无识别 = 远间隔——识别值已过去（识别过期）也落在零值判定，
	// 绝不在"未知开窗点"下仍 2s 高频轰炸平台（"访问过于频繁"熔断形态）。
	return s.probeIntervalForOpen(now, s.openTimeFor(""))
}

// probeIntervalForOpen 判定探测间隔的纯函数——open 由调用方统一传入（tick 开头取一次
// 快照复用），open 零值（未识别/识别过期）= 远间隔，识别值未来才临门 2s 盯守。
func (s *Scheduler) probeIntervalForOpen(now time.Time, open time.Time) time.Duration {
	if open.IsZero() {
		return probeIntervalFar
	}
	if now.After(open.Add(-nearWindow)) {
		// 从未开过窗 + 开放时间已过 + 空快照 = 幽灵窗口（平台窗口从未
		// 开启或已关闭且从未被探测确认）——2s 高频盯守只剩烧平台（"访问过于频繁"熔断
		// 形态）。以空快照 + 时钟失败裕量判定幽灵窗口，探测降回 30s 常态（窗口若真开、
		// 平台下发新一轮 beginTimes，临门判断自然重新收紧）。黄金期不受影响：开窗瞬间探测
		// 确认 opened=true，绝不走此分支。
		if now.After(open) && s.WindowClosed() {
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
	UpdateIDToken(acct, idToken string) error                 // 自动重登后落库新 token
	DeleteSuccess(acct string, classID int) error             // 手动退选后删除 success 行
	SaveRefused(acct string, classID int) error               // 手动退选记库，重启后自动引擎仍不抢回
	DeleteRefused(acct string) error                          // 重设目标清空该账号全部退选标记
	DeleteRefusedClass(acct string, classID int) error        // 手动重报成功清单条退选行（与内存侧解除对称）
	SetTargetsForAccount(acct string, targets []Target) error // 保存目标（含发布元数据持久化）
}

// Scheduler 定时抢课引擎。多账号目标与已完成状态均按账号隔离。
type Scheduler struct {
	clients  AccountClients
	store    Store
	openTime time.Time
	interval time.Duration

	mu               sync.Mutex
	acctTargets      map[string][]Target // 按账号隔离的目标课程
	state            SchedulerState
	ws               *windowState // 窗口状态机（C6：openTimeDetected/opened/closed/emptyProbeRuns/syncFailStreak 写侧收敛，见 window_state.go）
	inflight         map[string]map[int]bool // [账号][classID] 正在提交
	done             map[string]map[int]bool // [账号][classID] 已成功
	full             map[string]map[int]bool // [账号][classID] 已确认满员（快照显示不满时解除）
	refused          map[string]map[int]bool // [账号][classID] 用户手动退选（自动引擎绝不抢回，直到重设目标）
	lastProbe        time.Time               // 全校正规探测节流闸门：只归 probe()/ProbeNow 写入
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
	warnedNoTargets  bool                             // 无目标空转警告只打一次

	clockOffset      time.Duration                // 服务端时钟对齐偏差 (server - local)
	lastSyncTime     time.Time                    // 上次时钟对齐成功采样时间（仅成功推进）
	lastSyncStart    time.Time                    // 当前正在进行的同步发起时刻（成功时回写 lastSyncTime）
	lastSyncFailAt   time.Time                    // 上次同步失败时刻（失败退避计时基准）
	syncFailedWindow time.Time                    // 时钟失败/恢复时刻留档（写而不读，判据用 syncFailStreak，见 maybeSyncClock）
	syncing          bool                         // 同步进行中标记（防 tick 叠加发起并发同步）
	probing          bool                         // 探测进行中标记（单飞：同一时刻全校只允许一次 probe 在跑）
	syncFailStreak   int                          // 时钟同步连续失败次数（≥3 时回退 offset=0）
	lastPrewarm      time.Time                    // 上次连接池预热时间
	rateLimited      map[string]map[int]time.Time // [账号][classID] 风控退避截止时刻

	chainMu sync.Mutex
	chains  map[string]bool // 链活跃标记：key=acct+"\x00"+publishID

	probeSem chan struct{} // per-account 探测并发信号量（cap 4）

	ctx    context.Context
	cancel context.CancelFunc
	start  bool
}

// sameClientFor 复核账号在注册表中的客户端是否仍是发起提交时的同一身份（需持 s.mu）。
// 仅判"账号名存在"挡不住同名重建——删号后同名重建会用新 *zhidao.Client 顶替，
// 旧链返回后 ClientFor(acct) 仍 ok 却指向新身份。接口值比对用反射的指针身份（unpack
// 具体类型指针取 Pointer 值），nil 视为非同一身份；账号已删（ClientFor 不存在）也非同一。
// 调用点：spawnChain 成功/失效分支写状态与落库前。
func (s *Scheduler) sameClientFor(acct string, chainClient Client) bool {
	current, ok := s.clients.ClientFor(acct)
	if !ok || current == nil {
		return false
	}
	return clientIdentity(current) == clientIdentity(chainClient)
}

// clientIdentity 返回客户端接口动态值的唯一身份标识（指针值）。
// scheduler.Client 是接口，*zhidao.Client 与测试的 *fakeClient 都是具体指针实现——
// reflect.ValueOf(x).Pointer() 对指针动态类型返回底层指针值，同一实例恒等。
func clientIdentity(c Client) uintptr {
	if c == nil {
		return 0
	}
	v := reflect.ValueOf(c)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0
	}
	return v.Pointer()
}

// New 创建调度器。openTime 为选课窗口开启时间（本地时区）。
func New(clients AccountClients, store Store, openTime time.Time, interval time.Duration) *Scheduler {
	// interval 非正数兜底——time.NewTicker(非正) 直接 panic（实测 NewTicker(0)
	// 抛 non-positive interval），生产 main 恒传 300ms、测试全部传正，此处防御未来
	// 配置化/时间操控传入 0|负值导致 Start() 协程整崩且无 recover 兜底（与 tick 无
	// recover 同族防御缺口）。
	if interval <= 0 {
		interval = 300 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		clients:          clients,
		store:            store,
		openTime:         openTime,
		interval:         interval,
		acctTargets:      make(map[string][]Target),
		inflight:         make(map[string]map[int]bool),
		done:             make(map[string]map[int]bool),
		full:             make(map[string]map[int]bool),
		refused:          make(map[string]map[int]bool),
		tokenValid:       make(map[string]bool),
		reloginAt:        make(map[string]time.Time),
		reloginFail:      make(map[string]int),
		relogging:        make(map[string]bool),
		rateLimited:      make(map[string]map[int]time.Time),
		chains:           make(map[string]bool),
		probeSem:         make(chan struct{}, 4), // per-account 探测并发上限
		acctData:         make(map[string]*zhidao.ElectivesData),
		acctDataAt:       make(map[string]time.Time),
		ws:               newWindowState(),
		ctx:              ctx,
		cancel:           cancel,
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
// 同步连续失败 3 次即复位 clockOffset=0（窗口判定回到本地时钟），
// 绝不带着一个过期偏差长期误判开窗点；单次成功立即清零失败计数，瞬断不累计。
// 同步闸门推进改为"同步成功才推进 lastSyncTime"——此前在锁内、
// 发起异步 goroutine 前就把 lastSyncTime=now：网络抖动导致 SyncServerTime 挂起 >300ms
// 时（等于上一个 tick 间隔），并发 goroutine 回写会跳过一个完整的 60s 窗口，且
// 连续失败 3 次复位 clockOffset 后该分钟整段不再校准。现在的推进语义：启动同步
// 即记录发起时刻（lastSyncStart），只有成功采样才把 lastSyncTime 推到发起时刻——
// 失败绝不吃闸门、也绝不复位 lastSyncTime，下一轮 tick 立即可重试，校准窗口最多
// 丢失一个 tick 间隔（300ms）而非整分钟。
func (s *Scheduler) maybeSyncClock(now time.Time) {
	s.mu.Lock()
	// 无账号注册表（空库/账号全删）：直接放弃——AnyClient 拿不到客户端，同步无从发起。
	// 防御 clients 为 nil 时若继续走 `s.clients.AnyClient()` 会空指针 panic。
	if s.clients == nil {
		s.mu.Unlock()
		return
	}
	// 快路径（现在被成功推进）：距上次成功采样 ≥1min 才发起新同步
	if !s.lastSyncTime.IsZero() && now.Sub(s.lastSyncTime) < time.Minute {
		s.mu.Unlock()
		return
	}
	// 失败退避——上次同步失败后 30s 内不得再发起。
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
	// 此前 `if !s.syncing{...}; inflight:=s.syncing; if !inflight{return}`
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

	// 发起异步时钟校准；goroutine 完成回调复位 syncing / 推进 lastSyncTime。
	// 先确认有可同步客户端再置位——此前无账号（空库/账号全删）或
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
					s.lastSyncFailAt = s.nowAlignedLocked() // 失败落地即记录，退避 30s（对齐钟，判读侧 :342 同基准——LOW-132-01 混用孤岛收敛）
					log.Printf("[scheduler] 时钟对齐失败（连续 %d 次）：%v", s.syncFailStreak, err)
					// 时钟失败时刻留档——幽灵窗口判定只读
					// syncFailStreak（≥3 且开放时间已过），本字段写而不读，与成功路径的
					// 清零对称保留（失败/恢复时刻留档，便于未来按时间差精细调参）。
					s.syncFailedWindow = s.nowAlignedLocked() // 对齐钟同上（失败留档与 lastSyncFailAt 同基准）
					// 到达 3 次后只把校准偏差复位（回退到本地时钟），
					// 绝不在此清零 streak——旧实现同一临界区先 ++ 再清零，外部读取方
					// （WindowClosed 持同一把锁）永远读不到 3（值域恒 {0,1,2}），时钟兜底
					// 判据实为不可达死代码（对应测试手动注入 3 恒假绿）。
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
				s.lastSyncFailAt = time.Time{}   // 成功即清失败退避（瞬断不拖延后续校准）
				s.lastSyncTime = s.lastSyncStart // 只有成功才推进成功采样闸门
				s.syncFailedWindow = time.Time{} // 与失败写点对称（写而不读，留档自愈语义）
				log.Printf("[scheduler] 服务端时钟对齐成功，校准偏差: %v", offset)
			}()
			return
		}
	}
	// 无客户端或不支持时钟同步：本次不发起，复位在途标记，等账号就绪后再同步。
	// 此前此处直接 return，syncing 被置 true 后无人复位——空库部署的首个 300ms tick
	// 就让时钟校准链路永久休眠（根因）。
	s.mu.Lock()
	s.syncing = false
	s.lastSyncStart = time.Time{}
	s.mu.Unlock()
}

// openTimeFor 返回指定账号的"已识别开放时间"（平台 beginTimes 自动识别，唯一事实源）：
// 优先级 = 该账号识别槽 openTimeDetected[acct] → 全校识别槽 openTimeDetected["*"] → 零值。
// 返回已识别的开窗时刻本身（不做过期截断）——"识别过期"语义由展示层 owner：
// StateForAccount/RecognizedOpenTime 依 now 判定 open_time_known，识别值已过去 = 展示"未识别"。
// 窗口关闭后识别槽保留旧值（"关闭≠时间消失"契约：空快照不删槽，展示层据此区分
// "批次已结束"与"从未识别"）。调度判定（tick 提交守卫/probeIntervalFor/windowClosedLocked）
// 直接用返回的开窗时刻比较 now ——"已到点"天然放行提交、由 WindowClosed 兜底降频。
func (s *Scheduler) openTimeFor(acct string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openTimeForLocked(acct)
}

// openTimeForLocked 需持 s.mu 的 openTimeFor 实现（tick/windowClosedLocked 锁内复用）。
// 返回"已识别的开窗时刻"（无论未来/过去）——**"识别过期"只影响展示层**（StateForAccount/
// RecognizedOpenTime 依 now 判定 open_time_known/open_time_set），绝不在此把过期值截断成
// 零值：tick 提交守卫的第二判据 `!now.After(open)` 天然放行"已到点"的过期识别值，
// 这里截断会使开窗瞬间起 open 恒零 → 提交循环被第一守卫永久挂起（黄金期自动抢课失效）。
func (s *Scheduler) openTimeForLocked(acct string) time.Time {
	// C6：识别槽读写收权进 ws（window_state.go）。tick/StateForAccount 持 s.mu 调本方法，
	// ws 自持锁在 s.mu 内获取，无嵌套冲突。识别值已过去也照常返回（见上注释：
	// 绝不截断零值——挂起/展示解耦，识别过期只影响展示层）。
	return s.ws.openTimeFor(acct)
}

// RecognizedOpenTime 返回全校识别的开放时间——**未识别（识别槽无值）返回零值**；
// 识别值已过期（已过去）由展示方依 need 判定：stats 传回给前端靠 open_time_set
// 表达"识别失效"。管理员 stats 展示用（纯只读，不加锁内部读；调用方不持锁）。
func (s *Scheduler) RecognizedOpenTime() time.Time {
	return s.openTimeFor("")
}

// SetTargetsForAccount 为指定账号替换目标并重建状态（账号必填，非空）。
// 用户重新设定目标即"主动重新选它"：清空该账号 refused 标记（含库内持久化行）——
// 被手动退选的课程只有在用户重新设为目标时才被自动引擎重新接管（绝不静默抢回）。
// 定案：**绝不**清 done/full/rateLimited/inflight——done 是跨目标的
// 持久历史事实（重启恢复 RestoreDone 注入），重设目标清掉会把已成功课程重新提交；
// full/rateLimited/inflight 是本次窗口内的真实防轰炸/防双包状态，清了让自动链立刻重打
// 刚被平台拒绝的课。删账号路径的**全量清理**走专用 PurgeAccount（见下）。
func (s *Scheduler) SetTargetsForAccount(acct string, targets []Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acctTargets[acct] = targets
	delete(s.refused, acct)
	// 定案：不清 done/full/rateLimited/inflight——done 是跨目标的
	// 持久历史事实（RestoreDone 注入/手动报名 MarkDone 写入/落库 SaveSuccess），清掉会
	// 把已成功课程重新提交（TestRestoreDoneSkipsResubmit 固化契约）；full/rateLimited
	// 是真实满员/风控退避状态（自愈由快照解封/退避过期提供），清了让自动链立刻重打刚被
	// 平台拒绝的课（触发熔断）；inflight 防并发双发包（网络往返完成自清）。删账号的
	// 全量清理（含 acctData/tokenValid/relogin 族）归 PurgeAccount——管理员删除路径
	// handleAdminDeleteAccount 必须调它而非本方法。
	if s.store != nil {
		// 发布元数据补全：把 publish_id 对应的 publish_name/begin_date 合并进目标，随库
		// 持久化。窗口关闭后 /electives 空发布、后端快照也空（解析清空），此批补全是
		// 窗口关闭后日期/发布名仍可显示的关键机会窗（下一次保存只剩内存残留）。
		// enrichTargetPubMetaLocked 数据源：该账号专属帧 acctData[acct] 优先，兜底全校帧
		// lastData——HTTP 直存时该账号专属帧常过期/为空（目标保存不触发探测），
		// 而全校探测帧开窗前 30s 常态保鲜、更可能带着本轮批次。
		targets = s.enrichTargetPubMetaLocked(acct, targets) // 内存态与 store 均用补全后的目标
		s.acctTargets[acct] = targets
		// 落库带发布元数据：窗口关闭后 /state.courses 仍自带日期/发布名。
		// 落库失败必须 error 上抛（C1 契约：持久化失败绝不静默吞错）——此前只 log，
		// handler 预写成功 + 这里写失败时前端拿"已保存"而库内是缺元数据版本，
		// 目标持久化与用户感知分叉。错误上抛后调用方（handler）可正常报错、前端可重试。
		if err := s.store.SetTargetsForAccount(acct, targets); err != nil {
			log.Printf("[scheduler] 账号 %s 保存目标落库失败: %v", acct, err)
			return err
		}
		// 重设目标同步清空库内退选行——用户主动重新接管，退选标记不再需要。
		// 绝不静默吞错——库内 refused 行残留时，重启恢复序 LoadRefused +
		// RestoreRefused（RestoreTargets 不清 refused 契约）会把已重新接管的课程
		// 恢复成"已手动退选（自动引擎不再接管）"，用户意图与持久化分叉。内存侧 delete
		// 代表"当前运行期用户意图"正确保留，仅留日志供运维在重启错位时排查。
		if err := s.store.DeleteRefused(acct); err != nil {
			log.Printf("[scheduler] 账号 %s 重设目标清空退选记录落库失败（重启后该课会被恢复成'已手动退选'）: %v", acct, err)
		}
	}
	s.rebuildCoursesForAccountLocked(acct, targets)
	return nil
}

// PurgeAccount 全量清空指定账号在调度器中的一切状态：
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
	s.ws.purge(acct) // 开放时间识别槽随账号全量清理，绝不残留旧批次识别值
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

// RestoreTargets 重启恢复目标：与 SetTargetsForAccount 唯一区别是不清
// refused（内存 + 库行）——重启恢复的目标不是"用户主动重选"，若清库行会把已持久化的
// 手动退选记录删掉（被恢复顺序抵消）。恢复顺序：RestoreDone → 循环 RestoreTargets
// → LoadRefused + RestoreRefused（main.go）。
func (s *Scheduler) RestoreTargets(acct string, targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 重启恢复同样兜底补全发布元数据（与 SetTargetsForAccount 同源）：历史库若因
	// HTTP 直存快照缺失留下空元数据行，重启后恢复时兜底全校帧 lastData 补全——
	// 启动后 probe 很快填充全校帧，补全后 /state.courses 分组不再落"未知日期"。
	s.acctTargets[acct] = s.enrichTargetPubMetaLocked(acct, targets)
	s.rebuildCoursesForAccountLocked(acct, s.acctTargets[acct])
}

// enrichTargetPubMetaLocked 为目标补全 publish_name/begin_date（需持 s.mu）。
// 数据源：该账号专属帧 acctData[acct] 优先，兜底全校帧 lastData（见
// SetTargetsForAccount 注释——HTTP 直存时专属帧常过期/为空，全校帧更可能带本轮批次）。
// 两者都无 → 保持原样（semantics：窗口重开后重存目标自愈）。
func (s *Scheduler) enrichTargetPubMetaLocked(acct string, targets []Target) []Target {
	var pubs []zhidao.Publish
	if data := s.acctData[acct]; data != nil {
		pubs = data.Publishes
	} else if data := s.lastData; data != nil {
		pubs = data.Publishes
	}
	out := make([]Target, len(targets))
	for i, t := range targets {
		if t.PublishName != "" && t.BeginDate != "" {
			out[i] = t
			continue
		}
		for _, p := range pubs {
			if p.PublishID == t.PublishID {
				if t.PublishName == "" {
					t.PublishName = p.PublishName
				}
				if t.BeginDate == "" {
					t.BeginDate = p.BeginDate
				}
				break
			}
		}
		out[i] = t
	}
	return out
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
		// 重设目标且该课仍被拒绝（用户退选）时，done 恢复成功文案会掩盖
		// 退选意图——refused 优先，强制 pending。
		if s.refusedHas(acct, t.ClassID) {
			status = "pending"
			result = "已手动退选（自动引擎不再接管，可重新设为目标恢复）"
		}
		s.state.Courses = append(s.state.Courses, CourseStatus{
			Account:     acct,
			PublishID:   t.PublishID,
			ClassID:     t.ClassID,
			CourseName:  t.CourseName,
			Priority:    t.Priority,
			Status:      status,
			Result:      result,
			PublishName: t.PublishName, // 发布元数据随目标透传（窗口关闭后分组/展示仍在）
			BeginDate:   t.BeginDate,
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

// RestoreRefused 注入重启前已手动退选的 (账号, 课程) 记录（顺序修正）：
// 必须**先于** SetTargetsForAccount 循环调用——后者（用户重设目标恢复路径）会
// `delete(s.refused, acct)` + `DeleteRefused(acct)` 清空该账号退选，若先循环再注入，
// 注入的标记被恢复路径覆盖、持久化行也被删（退选被重启顺序抵消）。
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
	log.Printf("[scheduler] 已启动，轮询间隔 %v（课程探测节流 30 秒），开放时间自动识别开启", s.interval)
}

// Stop 停止轮询。
func (s *Scheduler) Stop() {
	s.cancel()
}

// StateForAccount 返回指定账号的状态快照（Courses 仅含该账号目标；WindowOpened 全校共享）。
// WindowClosed 字段用 windowClosedLocked() 实时计算——此前只写
// s.state.WindowClosed（probe 主判据），WindowClosed() 方法的两条兜底判据（时钟
// 连续失败、幽灵窗口 EmptyProbeRuns）返回 true 时不回写字段：幽灵窗口/时钟失败
// 场景下 /api/state 下发 window_closed=false，前端横幅仍显示倒计时/"已开放"、日志与课程
// 轮询维持 10s/2s 高频（降频机制失效）——展示与实际挂起状态分叉。
func (s *Scheduler) StateForAccount(acct string) SchedulerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	st.WindowClosed = s.windowClosedLocked() // 三条判据单源（含兜底），与 WindowClosed() 同真相
	// 开放时间解析（唯一事实源 = 平台 beginTimes 自动识别）：
	// 优先级 = 该账号自识别 openTimeDetected[acct] → 全校识别槽 openTimeDetected["*"]。
	// 识别值必须"仍在未来"才算有效（识别过期 = 上一批次/窗口结束，平台未再下发新
	// beginTimes → 视为未识别，前端显示"未识别到开放时间"而非把过期旧值挂出来）。
	// 识别槽本身不删（"关闭≠时间消失"契约：窗口关闭后空快照只覆盖非空 beginTimes，
	// 保留已识别的开窗事实），过期与否由这里的有效时刻判定区分。
	st.OpenTime = s.openTimeForLocked(acct)
	// 识别过期语义（展示层 owner）：识别值已过去（批次已结束/窗口关闭）→ open_time_known=false，
	// 前端显示"未识别到开放时间"，绝不把过期旧值挂出来当"当前开放时间"；识别槽保留
	// （关闭≠时间消失）。
	if !st.OpenTime.IsZero() && st.OpenTime.After(s.nowAlignedLocked()) {
		st.OpenTimeKnown = true
	}
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
// 列表必须排序——api 层管理员不带 ?account= 时取 targetAccts[0] 对齐
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
	// 目标账号（已配置目标）必须用该账号专属年级快照渲染，
	// 绝不无条件回退全局 lastData——probe() 只对 AccountsWithTargets() 遍历刷新
	// per-account 帧，有目标的账号 30s 内必有其专属帧；专属帧缺失 = 账号刚登录/
	// 探测尚未完成，此时回退返回的全局帧可能是首个注册账号（m.order[0]）的年级帧，
	// 混合年级部署下（高二 82 门/高三 1 门文档实证）会把错年级 1 门课渲染给高二学生，
	// 且永不触发本账号 ProbeForAccount——年级串线全开。这里返回 false 让
	// handleElectives 走 ProbeForAccount 真取该账号年级帧；无目标账号（仅浏览/手动
	// 报名）保持回退全局帧（快、无网络开销，且手动复核 CheckClassSelectable 不读全局帧）。
	// 判据用 len(acctTargets[acct]) > 0 而非 map key 存在性——handler 层清空目标时
	// 落 `SetTargetsForAccount(acct, []Target{})` 留下空 slice（key 存在），若按 key 判断
	// 该账号会一直走"有目标账号"专属路径：专属帧过期后每次浏览都返回 (nil,false) 触发
	// ProbeForAccount 网络探测，与"无目标账号回退全局帧（快、无网络开销）"契约相悖，
	// 且与 AccountsWithTargets()（返回 len>0）判定不一致——probe() 不会刷新该账号帧。
	if acct != "" && len(s.acctTargets[acct]) > 0 {
		if d, ok := s.acctData[acct]; ok && d != nil {
			snappedAt := s.acctDataAt[acct]
			if !snappedAt.IsZero() && s.nowAlignedLocked().Sub(snappedAt) <= snapshotTTL {
				return d, true
			}
		}
		return nil, false
	}
	if acct != "" && s.acctData != nil {
		if data, ok := s.acctData[acct]; ok && data != nil {
			// 有专属帧：新鲜即返回；已过期则返回 (nil,false) 让读取方走 ProbeForAccount
			// 真取该账号最新年级帧——否则此刻全局 lastData 恰被其他账号（如首个注册账号
			// m.order[0]）刷新为新鲜帧时，下面会回退这份**错年级**数据且 ok=true，读取方
			// 不会触发本账号刷新，浏览者持续看到别的年级课程（标称
			// "无目标账号回退全局帧"的"快、无网络开销"只对**从未有过专属帧**的纯浏览成立；
			// 对曾有过专属帧但已过期的中间态，回退并不比刷新快，且数据是错年级）。
			// 修复：过期专属帧 → 必须返回 false 触发刷新，绝不回退全局帧。
			if s.nowAlignedLocked().Sub(s.acctDataAt[acct]) <= snapshotTTL {
				return data, true
			}
			return nil, false
		}
	}
	if s.lastData == nil || s.nowAlignedLocked().Sub(s.lastDataAt) > snapshotTTL {
		return nil, false
	}
	return s.lastData, true
}

// ProbeForAccount 使用指定账号的专属客户端执行课程探测并刷新该账号快照。
// 命中 token 失效（ErrUnauthorized）时触发该账号自动重登。
//
// 账号不存在时返回明确错误，绝不回退 ProbeNow 直打教务上游——
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
	// 时间基准统一——探测时间戳写入侧改用对齐钟（与 tick 判读
	// `now.Sub(lastProbe)` / `ElectivesSnapshotFor` 的 `time.Since(acctDataAt)` / 提交节流
	// `submitIntervalFor` 同一时间基）。此前写入用本地 `time.Now()`、读用对齐钟，
	// 与已消灭的"写入本地/读对齐"混用模式同族（偏差 ~640ms 对 2s/30s 节流与
	// `now.After(open)` 窗口判点无实质错误，但契约不自洽）。
	now := s.nowAligned()
	// 开放时间识别入账：平台顶层 beginTimes 毫秒数组（HAR 实证全校共享单值，
	// select.js 只做 1500ms 开窗探测、不按发布/账号区分）。识别时间 = 平台已训示的
	// 开窗时刻（事实），窗口关闭后空快照不带 begin_times 但识别值必须保留——
	// 关闭≠时间消失，前端倒计时归零/显示已结束而非"未知"。空快照不删除识别槽，
	// 只在下发非空 begin_times 时覆盖（新批次热更仍生效）。
	// 写入必须在 s.mu 锁内（与 tick/openTimeForLocked 持锁读并发）——map 无锁并发
	// 读写是 Go 数据竞争（runtime 可 throw），识别槽是调度器核心读路径（每 300ms tick）。
	// M88-01 身份防线：本函数入口已取 client 指针，但网络往返（FindElectives 最长
	// 15s）期间账号可能被删号 + 同名重建（新 *zhidao.Client 顶替）。回写段锁内必须
	// 复核"当前注册表客户端仍是发起探测时的同一身份"（与 spawnChain 六分支
	// sameClientFor 同族）——否则旧链会把过期快照写进重建账号的 acctData 条目、
	// 或覆盖其 openTimeDetected 识别槽（年级串线/过期数据一帧可见，下个探测自愈）。
	// 已删（ClientFor 不存在）或指针身份已变 → 整体放弃回写。
	s.mu.Lock()
	if !s.sameClientFor(acct, client) {
		s.mu.Unlock()
		log.Printf("[scheduler] 账号 %s 探测返回时身份已变或已删除，放弃写回快照", acct)
		return data, nil
	}
	if len(data.BeginTimes) > 0 {
		s.ws.setOpenTime(acct, data.BeginTimes[0])
		log.Printf("[scheduler] 账号 %s 识别到开放时间 %s（平台 beginTimes）", acct, time.UnixMilli(data.BeginTimes[0]).Format("2006-01-02 15:04:05"))
	}
	if s.acctData == nil {
		s.acctData = make(map[string]*zhidao.ElectivesData)
		s.acctDataAt = make(map[string]time.Time)
	}
	s.acctData[acct] = data
	s.acctDataAt[acct] = now
	// ProbeForAccount 只写该账号专属快照，不动全局 lastData/lastDataAt。
	// 管理员 ?account=A 穿透探测若写全局帧会污染全局快照（年级不同的帧），
	// 页面 ElectivesSnapshot 读全局帧时看到错年级课程——年级串线根因之一。
	// 这里不再写 lastProbe——lastProbe 是全局探测节流闸门（tick 用），
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
	if s.lastData == nil || s.nowAlignedLocked().Sub(s.lastDataAt) > snapshotTTL {
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
// 除"至少开过窗 + 空快照 + 开放时间已过"主判据外，
// 追加"时钟连续失败 ≥3 且开放时间已过"兜底——从未开过窗的幽灵窗口（平台空快照）
// 无法靠主判据判定关闭，长期 2s 高频探测烧平台；时钟失败是"网络/平台异常"的可靠
// 信号，连续失败即视同关闭，挂起提交 + 探测降频（自愈由 syncFailStreak 归零提供）。
func (s *Scheduler) WindowClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.windowClosedLocked()
}

// windowClosedLocked 计算窗口关闭判定（需持 s.mu）——三条判据单源：
// 1) 主判据 s.state.WindowClosed（至少开过窗 + 空快照 + 已过开窗点 10s，probe 写入）；
// 2) 时钟连续失败 ≥3（平台不可达信号）且开放时间非零；
// 3) 从未开过窗 + EmptyProbeRuns≥3（幽灵窗口量变）且开放时间非零。
// StateForAccount 与 WindowClosed() 共用同一实现，杜绝两套真相分叉。
func (s *Scheduler) windowClosedLocked() bool {
	if s.state.WindowClosed {
		return true
	}
	// open 单快照对三条判据统一（判据2 取一次复用 + 判据3 同快照）——识别槽是唯一
	// 事实源，一次读取避免"识别值在两次读取间被新批次覆盖"的不一致窗口。
	open := s.openTimeForLocked("")
	if s.syncFailStreak >= 3 && !open.IsZero() && s.nowAlignedLocked().After(open) {
		return true
	}
	// 视同关闭的探测持续判定——开放时间已过 + 窗口从未开过（prevOpened
	// 恒 false，时钟兜底覆盖不到）+ 探测返回空快照 ≥3 次：平台窗口从未开启/已关闭且
	// 从未被确认开过（主判据 requirement 不满足 + 时钟正常时兜底不触发），
	// 2s 探测/1s 提交恒高频轰炸 findElectivesData + 报名接口（防轰炸契约闭环缺口）。
	// 首次探测（acctDataAt 全空）不计数、不误伤；runs≥3 即连续三轮空快照确证"从未开过"，
	// 进入幽灵窗口挂起，探测/提交同步降频。窗口若真开、平台下发新一轮 beginTimes，success 探测
	// 数据后势必推开始 open 实况、emptyRuns 归零自愈。
	if !open.IsZero() && !s.state.WindowOpened && s.state.EmptyProbeRuns >= 3 {
		return true
	}
	return false
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
	// 时间基准统一——与 ProbeForAccount 同款，写入侧用对齐钟
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
	open := s.openTimeForLocked("")
	s.mu.Unlock()

	s.maybePrewarm(now, open)
	s.maybeSyncClock(now)

	// 探测闸门：距上次成功探测不足当前阶段间隔且非首次则跳过
	// open 已在本函数开头取过单次快照（973 行）——probeIntervalFor 内部不再重取，
	// 与"探测间隔判定取单次 open 快照"同策略：热改亚毫秒窗口内立即探测判定与
	// 节流间隔若各自取 open，可能读到新旧两个不同值（一次放行、一次被节流或反之）。
	probe := last.IsZero() || now.Sub(last) >= s.probeIntervalForOpen(now, open)
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
	//   2) 对齐后的时间已过开窗点（识别值）——兜底：平台在到点瞬间把课程列表拉空
	//      （熔断/学期异常）或探测恰好失败时，不依赖探测确认也放行提交，黄金期不容浪费。
	// 注意 WindowOpened 只在"探测成功且列表非空"时更新；探测失败或 Publishes 被平台熔断拉空时
	// 维持上一轮值，因此这里不会把已开启的窗口误判为关闭。
	// open 为零值（未识别 / 识别过期）且窗口未被探测确证开启时恒满足
	// !now.After(open) → 提交循环永续放行。此时无有效开窗点——挂起提交，绝不放行。
	// 例外：WindowOpened=true（probe 已用发布级 inDateRange 确证平台开窗）但识别槽为空
	// （平台批次未下发非空 beginTimes）时，零值守卫不得挂起提交——否则前端显示
	// window_opened=true 而黄金期 250ms 冲刺 0 次，产品语义分叉（开窗时刻的缺失只应
	// 影响展示层"未识别到开放时间"，绝不影响已确证开启的窗口提交）。
	if open.IsZero() && !opened {
		return
	}
	if !opened && !now.After(open) {
		return
	}
	// 窗口已确认关闭（探测到空快照且开放时间已过，state.WindowClosed）
	// 时挂起提交——平台对关闭后的报名返回 code=1"无效的课程ID"（真实关闭文案），不在
	// isWindowClosedError 的"关闭/未开启/报名时间/已结束"匹配集合内 → 不记 full → 走实时
	// 复核 → 窗口关闭后 countList 空 → IsClassFull 报"课程无人数数据" → 下个 tick 重打
	// SelectClass，对未成功目标形成每 1s（黄金期 250ms）永续轰炸（防轰炸契约缺口）。
	// 以探测状态而非脆弱错误文案作为关闭判定：WindowClosed 已确认即挂起提交，与零值
	// 守卫并列，黄金期（开窗瞬间 WindowClosed=false）绝不影响。
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
// 也只视为"尚未确认窗口开启"，绝不把已开启的窗口误判为关闭（安全审计）——
// prevWindowOpened 在探测失败/空数据路径保持原值，窗口一旦开过就维持已开状态，
// 提交循环（spawnChain）仍会继续尝试目标课程，黄金期不因数据异常而停摆。
func (s *Scheduler) probe() {
	// 探测单飞守卫——probe() 的最长耗时是 FindElectives 网络往返
	// （15s 超时），HTTP 侧 /api/electives 在快照过期时并发的 ProbeForAccount/ProbeNow
	// 会与 tick 探测同时打上游，N 账号部署下开窗全期形成 N+1 并发 findElectivesData。
	// probing 在持 s.mu 时置位，保证"置位-检查"原子（零值守卫同款窗口）。
	// 命中单飞直接放弃本次探测：最长推迟一个 tick（300ms），临门/黄金期无实质损失。
	s.mu.Lock()
	if s.probing {
		s.mu.Unlock()
		return
	}
	s.probing = true
	s.mu.Unlock()

	// 时间基准统一——与 ProbeForAccount/ProbeNow 同款，写入侧用
	// 对齐钟（判读侧 tick `now.Sub(lastProbe)` 已在对齐钟下，避免两套时间基混用）
	now := s.nowAligned()
	// 独立维护：并发探测所有已配置目标的账号，独立刷新各自年级的专属快照。
	// 的 probing 单飞只保护"probe() 主体（任意客户端 FindElectives）"，这里
	// 每账号各起 goroutine 调 ProbeForAccount 不受保护——临门/开窗期 probeIntervalNear
	// 2s 周期触发时，N 账号部署每 2s 变 N+1 并发 findElectivesData 直打上游，与
	// "访问过于频繁 1 分钟熔断"实证契约冲突。
	// 修复：probeSem 结构化信号量（cap 4）封顶 per-account 并发——峰值从 N 降到 4，
	// 跨批（2s 周期短于一批耗时）受同一信号量约束绝不叠加；全局 FindElectives 主体
	// 不受影响（probe() 在 per-account 全部入场后执行）。
	// ponytail: cap=4 常驻，若平台放宽熔断或账号数 >50 再调。
	for _, a := range s.AccountsWithTargets() {
		go func(acct string) {
			// 结构化信号量封顶 per-account 探测并发——峰值从 N
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
	// 开放时间识别入账（全局探测载体更新）：平台顶层 beginTimes 毫秒数组（HAR 实证
	// 全校共享单值，select.js 只做 1500ms 开窗探测、不按发布/账号区分）。识别时间 =
	// 平台已训示的开窗时刻（事实），窗口关闭后空快照不带 begin_times 但识别值必须
	// 保留——关闭≠时间消失。空快照不删除识别槽，只在下发非空 begin_times 时覆盖。
	// 写入必须在 s.mu 锁内（与 tick/openTimeForLocked 持锁读并发，见 ProbeForAccount 同款注释）。
	if len(data.BeginTimes) > 0 {
		s.mu.Lock()
		s.ws.setOpenTime("*", data.BeginTimes[0])
		s.mu.Unlock()
		log.Printf("[scheduler] 识别到开放时间 %s（平台 beginTimes）", time.UnixMilli(data.BeginTimes[0]).Format("2006-01-02 15:04:05"))
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
	// 先捕获上一轮 WindowOpened 状态，再覆写本轮——关闭判定需要
	// "至少开过窗"作为前提（见下），若在覆写后读取 prevOpened 拿到的恒是本次 opened 值。
	prevOpened := s.state.WindowOpened
	s.state.WindowOpened = opened
	// 窗口关闭判定：快照为空（code:0 空 publishes，平台选课窗口关闭特征）
	// 且开放时间已过 → 明确标记窗口已关闭，日志输出供排查"课程为空"原因。
	// 去掉 !prevWindowOpened 条件——"开过再关"是窗口关闭最常见场景，
	// 若只认"从未开过窗"则开过再关后 WindowClosed 恒 false，probeIntervalFor 的
	// "开放时间已过 + WindowClosed → 降回 30s"分支永不命中，窗口关闭后仍 2s 高频探测。
	// 只在"至少开过窗"（opened 曾经为 true）后才标记关闭——
	// 否则未开窗即空快照（学期无发布/平台异常）会误标已关闭，tick 提交守卫按
	// WindowClosed 挂起提交，把"还没开窗待开"误停成"永不提交"（开窗瞬间探测推进、
	// 黄金期全停摆）。已开过窗再关 = 窗口关闭的实质语义，未开过不算关闭。
	// 判定侧补"已过开窗点 10s 裕量"，与 EmptyProbeRuns 入账侧
	// 入账的 10s 裕量对称——开窗确证（探测非空 + InDateRange）后平台若短暂返回空快照
	// （数据刷新/切学期过渡态记录的预清空现象，非永久关闭），旧判据下一拍探测
	// （临门 2s）即置关闭：tick 守卫挂起提交 + 探测降回 30s，若平台在 30s 内恢复，黄金期
	// 提交已停摆。10s 裕量覆盖过渡态；真关仅推迟 10s 判定（超裕量仍按原判据关闭，
	// TestWindowClosedState 的 -time.Hour 场景不受影响）。
	// 裕量基准 open 取一次快照复用——同一探测内两处 10s 裕量判定若各自取 open，热改
	// 亚毫秒窗口内主判据与 EmptyProbeRuns 入账可能基于新旧两个不同 open（同族）。
	open := s.openTimeForLocked("")
	s.state.WindowClosed = prevOpened && !opened && len(data.Publishes) == 0 && now.After(open.Add(10*time.Second))
	// 探测量变入账——空快照 + 从未开窗 + 开放时间已过 → 连续轮数 +1；
	// 否则（非空快照 / 本轮被确证开窗 / 未到开放时间）归零。窗开 shift probe 会自然重置。
	// 注意绝不触碰 state.WindowClosed（由主判据/时钟判据独占）：这里只维护量变计数，
	// WindowClosed() 读取它做"视同关闭"兜底——不写 state.WindowClosed 避免误标真实关闭。
	// 入账另加"已过开窗点 10s 裕量"——平台在开窗前会预清空 publishes
	// （记录的真实现象，切学期/数据迁移），若开窗瞬间清空过渡态持续 ≥6 秒（3 次探测
	// × 2s 临门间隔），旧判据会在真实窗口已开时误挂起黄金期提交+降频探测；以"开窗点后
	// 10s 内不计空快照轮数"错开过渡态，窗口真开（10s 黄金期结束）后连续空才确证幽灵窗口。
	if !opened && len(data.Publishes) == 0 && now.After(open.Add(10*time.Second)) {
		s.state.EmptyProbeRuns++
	} else {
		s.state.EmptyProbeRuns = 0
	}
	// prevWindowOpened 是写而不读的死字段（已去掉 !prevWindowOpened 条件），
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
// 不会拖延其他账号的重登与提交（安全审计：全局锁跨长 Login 的修复）。
func (s *Scheduler) maybeRelogin(acct string) {
	s.reloginMu.Lock()
	defer s.reloginMu.Unlock()
	s.mu.Lock()
	// 入口先做账号存在性复核——重登 goroutine 的"写回侧"有复核，
	// 决策侧裸露：探测定时三处（ProbeForAccount/ProbeNow/probe）对 ErrUnauthorized 直调本入口，
	// 若删号与在飞探测返回 ErrUnauthorized 同帧（窗口约 15s），下方会重新把
	// tokenValid/reloginFail/reloginAt/relogging 写进已删账号的 map key（PurgeAccount 已清）——
	// 同名重建后新账号 tokenValid 残留 true（前端"已失效"）+ spawnChain 整链挂起 + 首登无辜退避 30s。
	// 账号已删不发起重登、不写任何 map；与 spawnChain 失效分支的先身份复核同族防线。
	if _, ok := s.clients.ClientFor(acct); !ok {
		s.mu.Unlock()
		return
	}
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
	// tokenValid 失效标记在此处置位、且与决策同持两把锁——发起重登即"token
	// 已知失效"，本就不该等 goroutine 开头再补写。此前 goroutine 开头才置位：用户手动
	// 登录成功（MarkTokenValid 清 tokenValid/relogging/reloginFail）与在途重登并发时，
	// 置位会覆写已被清理的标记，前端 /state 短暂回"已失效·自动恢复中"后 jitter。
	s.tokenValid[acct] = true
	s.mu.Unlock()

	go func() {
		relogged, err := s.clients.Relogin(acct)
		s.mu.Lock()
		delete(s.relogging, acct) // 清重登中标记（失败也清，才能再试）
		// 账号已删竞态防线——自动链/手动路径已有同款复核，
		// 重登成功分支仍缺：管理员 DeleteAccount（清凭据表+Accounts.Remove）与
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
					if s.store != nil {
						if uerr := s.store.UpdateIDToken(acct, tok); uerr != nil {
							log.Printf("[scheduler] 账号 %s 新 token 落库失败: %v", acct, uerr)
						}
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
		// 不再误报"有效"（安全审计）。
		// 修复：失败后 reloginFail 保留本次发起时递增到的次数，杜绝无条件复位 1——
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

// MarkTokenValid 手动登录成功时恢复该账号的 token 有效性标记：
// tokenValid 唯一的自动清零路径是 maybeRelogin 自动重登成功分支；若自动重登
// 长期失败（Vision 故障 / 无保存账密"请手动重新登录"），用户手动登录成功后仍显示
// "已失效·自动恢复中"无恢复路径。手动登录成功路径（issueSession）调用本方法，
// 清 tokenValid 失效标记与重登失败计数，前端 /state 立即恢复"有效"。
func (s *Scheduler) MarkTokenValid(acct string) {
	// MarkTokenValid 与 TokenValidFor/maybeRelogin 对齐锁序（reloginMu→s.mu）——
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
// （与自动链路径对称），命名上明确它是幂等门控的。
func (s *Scheduler) MaybeRelogin(acct string) { s.maybeRelogin(acct) }

// reloginResult 重登结果（异步回传到 tick 主循环统一处理）。
type reloginResult struct {
	acct     string
	relogged bool
	err      error
}

// maskedToken 脱敏打印教务 token：只显示前 8 位，绝不输出完整值。
// 长度 ≤8 的短 token 不足以掩盖身份，一律输出 "***"。
func maskedToken(tok string) string {
	if len(tok) > 8 {
		return tok[:8]
	}
	return "***"
}

// submitAll 并发提交所有账号所有发布的目标链（每链独立 goroutine，链内按人数确认满员依次退避）。
func (s *Scheduler) submitAll() {
	// 本地时钟写 lastSubmit 会与对齐时钟判定（tick 内 submitIntervalFor）
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
		// 冷启动（尚无目标）时每 tick 静默空转，运维无法区分
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
	// 链顶先判客户端存在——submitAll 已过滤 ClientFor 不存在的账号，
	// 但账号可在 submitAll 过滤后、本链启动前被删（Accounts.Remove memory-first），
	// 此前 !ok 只在每门课锁内兜底且会建 inflight+AppendLog 落库（删账号后继续堆积）。
	// 前置到链顶：已删账号静默放弃整链，绝不为幽灵账号建任何内存态/写任何日志。
	if _, ok := s.clients.ClientFor(acct); !ok {
		return
	}
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
		// 删除可发生在 spawnChain 入口判据之后、本 goroutine 取 client
		// 之前（毫秒窗口）——此处必须再判一次，nil client 绝不能进循环调 SelectClass
		// （nil 指针 panic）。已删账号静默放弃整链，不建 inflight、不写日志。
		if !ok || client == nil {
			return
		}
		// 链顶捕获发起提交的客户端指针——同名校验只保证"账号名当前在注册表"，
		// 不保证"仍是发起时的同一身份"。删号后同名重建（换绑/误删加回）会用新客户端顶替，
		// 旧链在 SelectClass 网络往返期间被顶替，返回后 ClientFor 仍 ok（新身份）却把成功
		// 状态与 success 行写进重建身份（重启假成功/已删账号状态复活）。后续成功分支与
		// 失效分支复核处都要用"指针身份比对"而非仅"账号名存在"。
		chainClient := client
		for _, t := range ts {
			s.mu.Lock()
			// 重登期间跳过该账号全部提交（无效 token 请求纯浪费 + 熔断风险）
			if s.relogging[acct] {
				s.mu.Unlock()
				return
			}
			// token 已知失效且重登进入退避期（relogging 已被失败路径清掉、
			// reloginAt 未过退避窗口）时，链顶只有 relogging 短路挡不住——每 tick 仍对每门目标
			// 真实发起 SelectClass（必然 code=-1"教务令牌失效"）并每题 AppendLog，烧平台请求
			// 额度 + 日志表堆积。tokenValidForLocked（tokenValid=true || relogging=true）即
			// "已知失效"，重登成功/手动登录后清 false 才恢复提交。前端 /state
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
			// 手动退选后被用户拒绝的课程：自动引擎绝不抢回，直到用户重新设为目标
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
			// 绝不并发双发包（评审项：inflight 去重落地）。
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
			msg, err = client.SelectClass(t.ClassID)
			// 该账号 token 失效：标记失效并异步重登（非探测账号也能触发），终止本链等恢复
			if errors.Is(err, zhidao.ErrUnauthorized) {
				s.mu.Lock()
				// 指针身份复核——失效分支此前只判账号名存在（ClientFor ok），
				// 同名重建后旧链命中 ErrUnauthorized 也会把"教务令牌失效"状态写进新身份。
				// 发起时捕获的 chainClient 与注册表现指针比对：非同一身份即静默放弃整链
				//（不写状态、不落日志），与成功分支同族防线（账号名存在复核保留
				// 在 sameClientFor 内部——已删账号首先就不通过）。
				if !s.sameClientFor(acct, chainClient) {
					delete(s.inflight[acct], t.ClassID)
					s.mu.Unlock()
					return
				}
				// 重登必须落在身份复核之后——旧链命中 ErrUnauthorized 但身份已变
				//（删号/同名重建）时不得触发 maybeRelogin：Manager.Relogin 对已删账号虽然
				// 报"未注册"，但失败计数仍写进已删账号 map，污染同名重建账号的首次自动重登
				//（无辜退避）。身份已验证为同一发起链，重登才真正作用于该账号。
				s.mu.Unlock()
				s.maybeRelogin(acct)
				s.mu.Lock()
				delete(s.inflight[acct], t.ClassID) // 清提交标记（避免残留占用）
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "教务令牌失效，自动重登中")
				if s.store != nil {
					if err := s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 教务令牌失效，自动重登中", false); err != nil {
						log.Printf("[scheduler] 账号 %s 课程 %d 失效日志落库失败: %v", acct, t.ClassID, err)
					}
				}
				s.mu.Unlock()
				return
			}

			s.mu.Lock()
			delete(s.inflight[acct], t.ClassID)
			if err == nil {
				// 写成功/落库前复核"账号仍存在且仍是发起时的同一身份"——
				// 管理员 DeleteAccount（清凭据表 + Accounts.Remove）与在飞 spawnChain 网络往返
				// （SelectClass 最长 15s）竞态时，本链在删除完成后才返回成功；同名重建（换绑/
				// 误删加回）后注册表现指针已换成新客户端，若只判账号名存在（ClientFor ok）
				// 会把旧链成功写进重建身份（重启后重新登录被 RestoreDone 恢复成"已报名成功"
				// 假状态）。指针身份比对：非同一身份即静默放弃写 done/状态/库行。
				if !s.sameClientFor(acct, chainClient) {
					log.Printf("[scheduler] 账号 %s 客户端身份已变更（同名重建/删除），放弃写成功落库", acct)
					s.mu.Unlock()
					return
				}
				if s.done[acct] == nil {
					s.done[acct] = map[int]bool{}
				}
				s.done[acct][t.ClassID] = true
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "success", msg)
				if s.store != nil {
					if err := s.store.AppendLog(acct, t.ClassID, "select", msg, true); err != nil {
						log.Printf("[scheduler] 账号 %s 课程 %d 报名成功日志落库失败: %v", acct, t.ClassID, err)
					}
					if err := s.store.SaveSuccess(acct, t.ClassID); err != nil {
						// 落库失败会让重启后 RestoreDone 漏掉这条成功记录、已成功课被重新提交——
						// 记录在案供运维排查（SQLite 单写者仅在磁盘满/IO 故障时失败）。
						log.Printf("[scheduler] 账号 %s 课程 %d 成功记录落库失败: %v", acct, t.ClassID, err)
					}
				}
				log.Printf("[scheduler] 账号 %s 课程 %d（%s）报名成功: %s", acct, t.ClassID, t.CourseName, msg)
				s.mu.Unlock()
				return
			}
			// 平台风控退避：识别到"频繁"或 429 相关错误，为该课程设置 30s 退避，跳过轰炸
			switch classifyPlatformError(err) {
			case errRateLimit:
				// 与成功/失效分支同族防线：风控退避也是"写重建身份"的污染点——账号在
				// SelectClass 往返期间被删并同名重建（注册表现指针已换），陈旧链命中风控
				// 文案会把 rateLimited 退避写进重建身份（假"退避中"让该课黄金期被静默跳过）。
				// 指针身份比对：非同一身份即静默放弃整链，绝不为新身份落退避/状态/日志。
				if !s.sameClientFor(acct, chainClient) {
					s.mu.Unlock()
					return
				}
				s.markRateLimitedLocked(acct, t.ClassID, 30*time.Second)
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "触发平台风控退避 30 秒: "+err.Error())
				if s.store != nil {
					if err := s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 触发平台风控退避 30s: "+err.Error(), false); err != nil {
						log.Printf("[scheduler] 账号 %s 课程 %d 风控退避日志落库失败: %v", acct, t.ClassID, err)
					}
				}
				s.mu.Unlock()
				return
			case errWindowClosed:
				// 平台对"选课窗口已关闭"的报名请求返回 code=1 错误（窗口关闭后课程列表已清空）。
				// 此时课程已无法再报，直接按满员处理记入 full 集合，
				// 避免每个 tick 都带着失败状态反复刷平台报名接口（窗口关闭后的最后一层防线）。
				// 与风控退避/成功分支同族防线：陈旧链命中窗口关闭错误会 markFullLocked
				// 把"已满员"永久退避写进同名重建身份——指针身份比对，非同一身份静默放弃整链。
				if !s.sameClientFor(acct, chainClient) {
					s.mu.Unlock()
					return
				}
				s.markFullLocked(acct, t)
				s.mu.Unlock()
				return
			}
			// 非满员失败：改为实时人数复核确认是否真满员
			// （用户要求：不解析平台"满"字错误文案，直接对比总数与已报数）
			// 注意：!ok（账号会话未建立）分支已被前置到链顶——go routine 启动时
			// 客户端不存在即静默放弃整链，本处不可能再遇到 !ok，无需再判。
			// 复核前主动释放 s.mu——此前整段网络请求（最长 15 秒）都攥着
			// 全局锁，黄金冲刺期里其它账号的探测/提交/时钟对齐全被锁死；锁外复核完再回锁收尾。
			// 锁内 SQLite 写（AppendLog/SaveSuccess 等，SetMaxOpenConns=1
			// 串行）只发生在持锁段、不跨此网络段——黄金期不因 DB 写停顿网络往返；持锁写
			// 窗口仅成功分支两行（微秒级），彻底消除需独立 DB goroutine，边际不动（观察项）。
			s.mu.Unlock()
			full, cErr := s.classFullRealtime(acct, t.ClassID)
			s.mu.Lock()
			// 实时复核网络段（最长 15s）期间管理员可能删除账号——回锁后先复核
			// 账号仍存在再进三路分支写状态/落日志/重建 full 族 map。已删账号静默放弃整块
			// （inflight 已在 SelectClass 返回后的统一清位处删掉，无残留），与链顶 /失效分支/成功分支
			// 同族防线——实时复核结果块是删号竞态最后一块裸露写点
			// （setStateLocked 的 idx<0 守卫只挡数组越界，挡不住落库与 map 写）。
			// 存在性复核升级为指针身份复核——同名重建（注册表现指针已换）后旧链
			// 经 classFullRealtime 命中新身份的 ErrUnauthorized，返回时 ClientFor 仍 ok（新身份
			// 存在）却会把 maybeRelogin/failed 状态写进新身份（无辜消耗登录预算）。与同函数
			// 成功/失效/风控/窗口关闭/确证满员五分支对称，sameClientFor 内含存在性判定。
			if !s.sameClientFor(acct, chainClient) {
				s.mu.Unlock()
				return
			}
			// 实时复核命中 token 失效（学生数接口同样鉴权）——
			// 与 SelectClass 分支对称触发自动重登（ErrUnauthorized 才是"重登中"语义），
			// 否则本次失败被当普通失败处理、下个 tick 又重打报名接口（token 已失效的
			// 报名必然再失败），失效恢复路径被延迟到探测/手动路径才发现。
			if cErr != nil && errors.Is(cErr, zhidao.ErrUnauthorized) {
				s.mu.Unlock()
				s.maybeRelogin(acct)
				s.mu.Lock()
				s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", "教务令牌失效，自动重登中")
				if s.store != nil {
					if err := s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 实时复核命中 token 失效，自动重登中", false); err != nil {
						log.Printf("[scheduler] 账号 %s 课程 %d 实时复核失效日志落库失败: %v", acct, t.ClassID, err)
					}
				}
				s.mu.Unlock()
				return
			}
			if cErr == nil && full {
				// 锁外复核窗口（最长 15s）内手动路径可能已抢到 inflight 位
				// 并 MarkDone 置 done+success（用户真实报名成功）——此时"确证满员"分支若直接
				// markFullLocked 会把 success 覆盖成"failed/已满员"并追加一条假"已满员"日志，
				// 与紧邻的"未现满员"分支（下方 doneHas 复核"绝不覆盖胜利状态"）不对称。
				// done 一旦置位（手动成功），满员分支必须让位，绝不覆盖胜利状态。
				if s.doneHas(acct, t.ClassID) {
					s.mu.Unlock()
					return
				}
				// 与风控退避/成功分支同族防线：复核网络段（最长 15s）内账号可能被删并同名
				// 重建（入口处的存在性复核只挡"账号不存在"，挡不住"新身份存在但指针不同"）——
				// 陈旧链确证满员后 markFullLocked 会把"已满员"永久退避写进重建身份。
				// 指针身份比对：非同一身份即静默放弃整链，绝不为新身份落 full/状态/日志。
				if !s.sameClientFor(acct, chainClient) {
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
			// read 类错误（服务端已完整消费请求体但响应读取中断）说明
			// 平台可能已成功处理这次报名（已抢到课但客户端没收到响应）——状态/日志不能
			// 标"报名失败"误导用户排查（黄金期下个 tick 重复报名被拒"已选过"时 failed 永久
			// 残留）。区分文案为"请求已发出但响应读取失败（平台可能已处理，以大厅状态为准）"，
			// 其余非归类错误维持原文。
			failMsg := err.Error()
			logMsg := "账号 " + acct + ": " + err.Error()
			if zhidao.IsReadErr(err) {
				failMsg = "报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）"
				logMsg = "账号 " + acct + ": " + failMsg
			}
			s.setStateLocked(s.statusIndexLocked(acct, t.ClassID), "failed", failMsg)
			if s.store != nil {
				if err := s.store.AppendLog(acct, t.ClassID, "select", logMsg, false); err != nil {
					log.Printf("[scheduler] 账号 %s 课程 %d 报名失败日志落库失败: %v", acct, t.ClassID, err)
				}
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
	return classifyPlatformError(err) == errRateLimit
}

// isWindowClosedError 平台在选课窗口关闭后对报名请求的返回特征（code=1 且提示已关闭/未开启）。
// 与"课程满员"同样不可再报，调度器按满员记录避免窗口关闭后无限轰炸报名接口。
func isWindowClosedError(err error) bool {
	if err == nil {
		return false
	}
	return classifyPlatformError(err) == errWindowClosed
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

// 退避截止基准与读侧统一为对齐钟——此前用本地钟 time.Now() 写入、
// spawnChain 用 nowAlignedLocked() 判期，两套时间基（实测相差 ~640ms）边界同一语义。
// 读侧 isRateLimitedLocked 已用 nowAlignedLocked()（见下方实现），写入必须同源，语义自洽。
func (s *Scheduler) markRateLimitedLocked(acct string, classID int, d time.Duration) {
	if s.rateLimited[acct] == nil {
		s.rateLimited[acct] = map[int]time.Time{}
	}
	s.rateLimited[acct][classID] = s.nowAlignedLocked().Add(d)
}

// classFullInSnapshot 快照人数确认满员（需持锁）。优先匹配该账号专属快照，无快照时回退全局快照。
// 满员判定用快照级字段 max_count/selected_count（findElectivesData 课程级，真实 select.js 实证，
// 表列定义 max_count/selected_count/audited_count 同屏）——这是调度器"真满员退避"的实证主路径；
// 实时接口 findElectivesStudentCount 未实证 maxCount（CountEntry 注释），实时复核实际不可判满员。
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
// 注意：实时接口未实证 maxCount（CountEntry 注释），IsClassFull 实际恒 false——
// 本复核保留为"平台未来下发 maxCount 时自动生效"的防御性路径，当前真满员判定
// 以 classFullInSnapshot（快照 max_count，实证）为主路径，spawnChain 已先于实时复核
// 用快照判满员跳过（见 spawnChain 内 classFullInSnapshot 调用），实时复核仅兜底不破坏防轰炸契约。
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
		if err := s.store.AppendLog(acct, t.ClassID, "select", "账号 "+acct+": 课程 "+t.CourseName+" 已满员，切换备选", false); err != nil {
			log.Printf("[scheduler] 账号 %s 课程 %d 满员日志落库失败: %v", acct, t.ClassID, err)
		}
	}
}

// releaseFullIfFreedLocked 快照显示不满时解除 full 标记并回 pending（需持锁）。
// 只要快照显示有名额空余（如其他同学退选），立即解除 full 标记，黄金期 250ms 冲刺立即捡漏。
// 守卫：只有快照**明确**显示该课程名额空余才解封——
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

// CheckClassSelectable 手动报名前的服务端复核：
// 基于该账号最近快照判定课程是否可报名——课程所在发布窗口未开放或课程已满员时
// 提前拒绝并返回友好原因，避免无谓打教务平台拿生硬错误码。
// 快照缺失（从未探测）或课程不在快照中（无法判定）时放行，由平台最终把关。
// 过期快照（超过 snapshotTTL 未刷新）一律按"无快照"放行——
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
			fresh = s.acctDataAt[acct] != (time.Time{}) && s.nowAlignedLocked().Sub(s.acctDataAt[acct]) <= snapshotTTL
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
// 与 spawnChain 成功分支同款防线——管理员 DeleteAccount（先清凭据/库行 +
// Accounts.Remove）与在飞手动报名（SelectClass 最长 15s）竞态时，删除完成后本请求才返回成功，
// 若不复核会把已删账号的 success 行写回，重启后重新登录被 RestoreDone 恢复成"已报名成功"假状态
// （自动链已根治，手动路径同样竞态整链开放）。账号已删则静默放弃落库，绝不写回。
func (s *Scheduler) MarkDone(acct string, classID int, courseName, msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 删除账号与在飞手动报名竞态防线——账号不再存在于客户端注册表即视为已删，
	// 放弃全部状态写入（真实平台报名已发生，但账号已删，写回只会制造幽灵 success 行）。
	if _, ok := s.clients.ClientFor(acct); !ok {
		log.Printf("[scheduler] 账号 %s 已被删除，放弃手动报名落库", acct)
		return nil
	}
	if s.done[acct] == nil {
		s.done[acct] = make(map[int]bool)
	}
	s.done[acct][classID] = true
	// 手动报名成功同样解除 refused——用户手动重选（成功）即表达
	// "我要这门课"，自动引擎应恢复接管（此前 refused 只在 SetTargetsForAccount 清空，
	// 手动重选成功但未重设目标时 refused 卡死，窗口重开后自动引擎永久跳过该课）。
	if s.refused[acct] != nil {
		delete(s.refused[acct], classID)
		// 库内 refused 行同步清除：删除只清内存标记是半套——手动退选落库的 refused
		// 行残留时，重启恢复序 RestoreTargets（不清 refused）+ LoadRefused + RestoreRefused
		// 会把这门已报名成功的课恢复成"已手动退选（自动引擎不再接管）"假象（状态文案误导
		// + spawnChain 永久跳过）。落库失败记录在案供运维排查（零吞错规范）。
		if s.store != nil {
			if err := s.store.DeleteRefusedClass(acct, classID); err != nil {
				log.Printf("[scheduler] 账号 %s 课程 %d 手动重报清退选记录落库失败（重启后该课会被恢复成'已手动退选'）: %v", acct, classID, err)
			}
		}
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
		if err := s.store.SaveSuccess(acct, classID); err != nil {
			log.Printf("[scheduler] 账号 %s 课程 %d 手动报名成功记录落库失败: %v", acct, classID, err)
		}
		if err := s.store.AppendLog(acct, classID, "select", msg, true); err != nil {
			log.Printf("[scheduler] 账号 %s 课程 %d 手动报名日志落库失败: %v", acct, classID, err)
		}
	}
	log.Printf("[scheduler] 账号 %s 课程 %d 手动标记成功: %s", acct, classID, msg)
	return nil
}

// RemoveDone 手动退选成功后同步调度器状态：从 done 移除、置 pending 状态并记日志。
// 同步清理 inflight 位：退选进行中占用的提交锁位必须释放。
// 同时记入 refused 集合并置"已用户退选"文案：后台 spawnChain 从此对该课程绝不再自动
// 接管——用户手动退出的课，自动引擎下一 tick（≤1s）就抢回是错误行为，
// 只有用户重新把它设为目标（SetTargetsForAccount 清空 refused）才恢复自动接管。
func (s *Scheduler) RemoveDone(acct string, classID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 与 MarkDone 同款防线——账号已删时手动退选成功同样不能写回
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
		// 删除 success 行——否则重启后该课被 RestoreDone 恢复成
		// "已报名成功"，用户当日的退选决定被静默撤销（与 CLAUDE.md 契约文档对齐）
		if err := s.store.DeleteSuccess(acct, classID); err != nil {
			log.Printf("[scheduler] 账号 %s 课程 %d 退选清除成功记录落库失败: %v", acct, classID, err)
		}
		// 持久化 refused——此前只写内存，重启后 refused 全丢，
		// SetTargetsForAccount（重启恢复路径）会 delete(s.refused, acct)，
		// 自动引擎把用户手动退选掉的课当新目标重新抢回，退选意图丢失。
		if err := s.store.SaveRefused(acct, classID); err != nil {
			log.Printf("[scheduler] 账号 %s 课程 %d 退选记录落库失败（重启后自动引擎可能抢回）: %v", acct, classID, err)
		}
		if err := s.store.AppendLog(acct, classID, "exit", "手动退选成功（自动引擎不再接管，重新设为目标可恢复）", true); err != nil {
			log.Printf("[scheduler] 账号 %s 课程 %d 退选日志落库失败: %v", acct, classID, err)
		}
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

// ManualSelect 手动报名深方法（C4 收权：把 handler 手动决策树收进调度器）。
// 内部完成：取排他锁 → 快照复核 → 同账号客户端 → 平台调用 → classify 分类 →
// 重登/退避/记 full/落库。与 spawnChain 自动链共用 classifyPlatformError 与 inflight 位。
// 差异（刻意保留，核实确认）：
//   - CheckClassSelectable 手动专属：无快照/过期快照一律放行交给平台（与自动链的
//     内联退避判定不同源——手动是"真实用户即时操作"，拿旧数据拦用户是错的）；
//   - 同步执行（同一请求内持锁网络往返），无跨请求身份顶替窗口——身份防线
//     由 MarkDone 内部的 ClientFor 复核覆盖（删号竞态写回防线，决策 B21）。
// 补核实挖出的缺口：重登退避期（tokenValidForLocked=false）手动点报名必须短路——
// 自动链重登退避期内手动路径此前照发 SelectClass 烧平台请求，这里前置检查。
func (s *Scheduler) ManualSelect(acct string, classID int, courseName string) (string, error) {
	client, ok := s.clients.ClientFor(acct)
	if !ok || client == nil {
		return "", errors.New("账号会话未建立或未登录")
	}
	// 退避期短路：token 已知失效（tokenValid=true || relogging=true）时手动点报名
	// 平台必然 code=-1，且自动链正在重登——绝不放行烧平台请求。
	s.mu.Lock()
	if !s.tokenValidForLocked(acct) {
		s.mu.Unlock()
		return "", errors.New("教务令牌已失效，正在自动重登，请稍后重试")
	}
	s.mu.Unlock()
	// 快照复核（手动专属语义：无快照/过期快照放行，交给平台把关）
	if reason, ok := s.CheckClassSelectable(acct, classID); !ok {
		return "", errors.New(reason)
	}
	// 取排他锁（inflight 位，与自动链共享互斥——绝不并发双发包）
	release, ok := s.TryAcquireSubmit(acct, classID)
	if !ok {
		return "", errors.New("该课程正在提交中，请勿重复操作")
	}
	defer release()
	// 平台调用
	msg, err := client.SelectClass(classID)
	if err != nil {
		// 分类处理（手动/自动共用 classifyPlatformError）
		switch classifyPlatformError(err) {
		case errAuth:
			// 只触发重登，绝不 MarkTokenValid（决策 42-4：后者会击穿指数退避）
			s.MaybeRelogin(acct)
			if s.store != nil {
				if aErr := s.store.AppendLog(acct, classID, "select", "账号 "+acct+": 教务令牌失效，自动重登中", false); aErr != nil {
					log.Printf("[scheduler] 手动报名失效日志落库失败: %v", aErr)
				}
			}
			return "", errors.New("教务令牌已失效，正在自动重登，请稍后重试")
		case errRead:
			if s.store != nil {
				if aErr := s.store.AppendLog(acct, classID, "select", "账号 "+acct+": 报名请求已发出但响应读取失败（平台可能已处理，以大厅状态为准）", false); aErr != nil {
					log.Printf("[scheduler] 手动报名 read 日志落库失败: %v", aErr)
				}
			}
			return "", errors.New("报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）")
		case errRateLimit:
			s.mu.Lock()
			s.markRateLimitedLocked(acct, classID, 30*time.Second)
			s.mu.Unlock()
			if s.store != nil {
				if aErr := s.store.AppendLog(acct, classID, "select", "账号 "+acct+": 触发平台风控退避 30s: "+err.Error(), false); aErr != nil {
					log.Printf("[scheduler] 手动报名风控日志落库失败: %v", aErr)
				}
			}
			return "", errors.New("触发平台风控退避，请稍后再试")
		case errWindowClosed:
			s.mu.Lock()
			s.markFullLocked(acct, Target{ClassID: classID, CourseName: courseName})
			s.mu.Unlock()
			return "", errors.New("选课窗口已关闭")
		}
		// errOther：原文案透传（含"课程不存在"等平台业务错误）+ 审计日志
		if s.store != nil {
			if aErr := s.store.AppendLog(acct, classID, "select", "账号 "+acct+": 手动报名失败: "+err.Error(), false); aErr != nil {
				log.Printf("[scheduler] 手动报名失败日志落库失败: %v", aErr)
			}
		}
		return "", err
	}
	// 成功：MarkDone（清 refused/inflight/full + 置 success + SaveSuccess + AppendLog）
	if err := s.MarkDone(acct, classID, courseName, msg); err != nil {
		return "", errors.New("报名成功但状态落库失败: " + err.Error())
	}
	return msg, nil
}

// ManualExit 手动退选深方法（C4 收权，与 ManualSelect 对称）。
// 内部完成：取排他锁 → 同账号客户端 → 平台调用 → classify 分类 → 重登/落库。
// 刻意不含 CheckClassSelectable——退选不该被快照满员/窗口拦截（用户可随时退自己已选的课，
// 与旧 handler 语义一致）；窗口关闭时退选请求平台会正常处理（退选窗口通常长于报名）。
func (s *Scheduler) ManualExit(acct string, classID int) (string, error) {
	client, ok := s.clients.ClientFor(acct)
	if !ok || client == nil {
		return "", errors.New("账号会话未建立或未登录")
	}
	// 退避期短路（与 ManualSelect 同款：token 失效期绝不放行烧平台请求）
	s.mu.Lock()
	if !s.tokenValidForLocked(acct) {
		s.mu.Unlock()
		return "", errors.New("教务令牌已失效，正在自动重登，请稍后重试")
	}
	s.mu.Unlock()
	release, ok := s.TryAcquireSubmit(acct, classID)
	if !ok {
		return "", errors.New("该课程正在操作中，请勿重复操作")
	}
	defer release()
	msg, err := client.ExitClass(classID)
	if err != nil {
		switch classifyPlatformError(err) {
		case errAuth:
			s.MaybeRelogin(acct)
			if s.store != nil {
				if aErr := s.store.AppendLog(acct, classID, "exit", "账号 "+acct+": 教务令牌失效，自动重登中", false); aErr != nil {
					log.Printf("[scheduler] 手动退选失效日志落库失败: %v", aErr)
				}
			}
			return "", errors.New("教务令牌已失效，正在自动重登，请稍后重试")
		case errRead:
			if s.store != nil {
				if aErr := s.store.AppendLog(acct, classID, "exit", "账号 "+acct+": 退选请求已发出但响应读取失败（平台可能已处理，以大厅状态为准）", false); aErr != nil {
					log.Printf("[scheduler] 手动退选 read 日志落库失败: %v", aErr)
				}
			}
			return "", errors.New("退选请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）")
		}
		if s.store != nil {
			if aErr := s.store.AppendLog(acct, classID, "exit", "账号 "+acct+": 手动退选失败: "+err.Error(), false); aErr != nil {
				log.Printf("[scheduler] 手动退选失败日志落库失败: %v", aErr)
			}
		}
		return "", err
	}
	// 成功：RemoveDone（清 done/inflight + 记 refused + 置"已退选"状态 + 落库）
	if err := s.RemoveDone(acct, classID); err != nil {
		return "", errors.New("退选成功但状态落库失败: " + err.Error())
	}
	return msg, nil
}
