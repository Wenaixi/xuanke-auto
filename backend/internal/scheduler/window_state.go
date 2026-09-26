package scheduler

import (
	"sync"
	"time"
)

// windowState 窗口状态机：把 openTimeDetected / opened / closed / emptyProbeRuns /
// syncFailStreak 五个状态位与"三判据关闭判定"收进一个 struct（写侧收敛），
// 写侧只经 setOpened/setClosed/noteProbeEmpty/noteSyncXxx/setOpenTime 入账、
// 读侧经 isOpened/isClosed 查询——调度器 Scheduler 持一个 ws *windowState
// 而非裸字段，杜绝写侧多写点各自散落。
// 忠实契约（对照 probe 原实现）：opened/closed 是**每轮探测覆写**的当前值
// （开过再关后 opened 回落 false、closed 由 prevOpened 表达式写 true 后下一轮
// 因 prevOpened 已 false 回落——持久关闭信号由 emptyProbeRuns 判据兜底），
// 绝不是一次性的单调置位。
type windowState struct {
	mu               sync.Mutex
	openTimeDetected map[string]int64 // 识别槽：openTimeDetected[acct] → ["*"]（关闭≠时间消失）
	opened           bool             // 当前探测是否确证窗口开启（发布级 InDateRange）
	closed           bool             // 主判据关闭标记（prevOpened && 空快照 && 已过开窗点 10s）
	emptyProbeRuns   int              // 空快照连续探测轮数（幽灵窗口判据）
	syncFailStreak   int              // 时钟同步连续失败次数（≥3 判关闭）
}

func newWindowState() *windowState {
	return &windowState{openTimeDetected: make(map[string]int64)}
}

// setOpened 覆写本轮"窗口已开"判定（probe 每轮计算 opened 后调用，可 true→false）。
func (w *windowState) setOpened(v bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.opened = v
}

// setClosed 覆写主判据关闭标记（probe 每轮按
// prevOpened && !opened && 空快照 && 已过开窗点 10s 计算后调用）。
func (w *windowState) setClosed(v bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = v
}

func (w *windowState) noteProbeEmpty() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emptyProbeRuns++
}

// noteProbeReset 非空快照/本轮被确证开窗/未到开放时间时归零空快照轮数。
func (w *windowState) noteProbeReset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emptyProbeRuns = 0
}

// setEmptyProbeRuns 显式覆写空快照轮数（测试夹具用：模拟"探测已入账 N 轮"的
// 幽灵窗口形态，不触发真实探测）。生产路径不用——probe 每轮 noteProbeEmpty/Reset。
func (w *windowState) setEmptyProbeRuns(n int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emptyProbeRuns = n
}

func (w *windowState) noteSyncSuccess() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncFailStreak = 0
}

func (w *windowState) noteSyncFailure() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncFailStreak++
}

func (w *windowState) setOpenTime(acct string, ms int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.openTimeDetected[acct] = ms
}

// openTimeFor 返回指定账号的"已识别开放时间"（毫秒转 time.Time，未识别返回零值）。
// 识别值已过去也照常返回——绝不截断零值（识别过期只影响展示层，挂起/展示解耦）。
// 优先级 = 该账号识别槽 openTimeDetected[acct] → 全校识别槽 openTimeDetected["*"]。
func (w *windowState) openTimeFor(acct string) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return openTimeFromMs(w.openTimeForLocked(acct))
}

// openTimeForLocked 需持 w.mu 的毫秒级读取（调度器 s.mu 锁内复用，省 time.Time 转换）。
func (w *windowState) openTimeForLocked(acct string) int64 {
	ms := w.openTimeDetected[acct]
	if ms == 0 {
		ms = w.openTimeDetected["*"]
	}
	return ms
}

// openTimeFromMs 毫秒 → time.Time：ms<=0 视为"未识别"返回零值。
// time.UnixMilli(0) 返回 1970-01-01（IsZero()=false），会把"未写入槽"误判成已识别——
// 测试断言未写入槽必须返回零值（识别槽无值 ≠ epoch）。
func openTimeFromMs(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func (w *windowState) purge(acct string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.openTimeDetected, acct)
}

// isOpened 返回当前"窗口已开"状态（对外 DTO state.WindowOpened 填充）。
func (w *windowState) isOpened() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.opened
}

func (w *windowState) emptyProbeRunsCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.emptyProbeRuns
}

func (w *windowState) syncFailStreakCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.syncFailStreak
}

// isClosed 三条判据单源（迁移 windowClosedLocked，逐字等价）：
// ① closed 已置位（主判据，probe 写入：至少开过窗 + 空快照 + 已过开窗点 10s）；
// ② syncFailStreak≥3 且开放时间已过（时钟兜底）；
// ③ 从未开过窗 + emptyProbeRuns≥3 且开放时间已过（幽灵窗口量变）。
// +10s 裕量在入账侧（noteProbeEmpty 的触发条件 now.After(open+10s)，由 probe
// 判定）——isClosed 本体不重复加裕量，与既有 windowClosedLocked 语义一致。
// open 由调用方传入单快照（判据②③复用同一 open），热改亚毫秒窗口内一致。
func (w *windowState) isClosed(open time.Time, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return true
	}
	if open.IsZero() {
		return false
	}
	if w.syncFailStreak >= 3 && now.After(open) {
		return true
	}
	return !w.opened && w.emptyProbeRuns >= 3 && now.After(open)
}
