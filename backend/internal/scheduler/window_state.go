package scheduler

import (
	"sync"
	"time"
)

// windowState 窗口状态机：把 openTimeDetected / opened / closed / emptyProbeRuns /
// syncFailStreak 五个状态位与"三判据关闭判定"收进一个 struct（C6 写侧收敛），
// 写侧只经 noteXxx 入账、读侧经 isClosed/isOpened 查询——调度器 Scheduler 持
// 一个 ws *windowState 而非裸字段，杜绝写侧多写点各自散落。
// 读侧 isClosed 迁入 windowClosedLocked 的三判据逻辑（逐字等价，见下）。
type windowState struct {
	mu               sync.Mutex
	openTimeDetected map[string]int64 // 识别槽：openTimeDetected[acct] → ["*"]（关闭≠时间消失）
	opened           bool             // 曾开窗（探测非空快照 + 发布级 InDateRange 确证）
	closed           bool             // 已确认关闭（state.WindowClosed 迁移）
	emptyProbeRuns   int              // 空快照连续探测轮数（幽灵窗口判据）
	syncFailStreak   int              // 时钟同步连续失败次数（≥3 判关闭）
}

func newWindowState() *windowState {
	return &windowState{openTimeDetected: make(map[string]int64)}
}

func (w *windowState) noteProbeOpened() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.opened = true
	w.emptyProbeRuns = 0
}

func (w *windowState) noteProbeClosed() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

func (w *windowState) noteProbeEmpty() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emptyProbeRuns++
}

// noteProbeReset 非空快照/未到开放时间时归零空快照轮数（入账侧语义：probe 的
// else 分支 reset）。开窗（noteProbeOpened）本身也会清零，此方法供不置 opened
// 的普通归零路径使用。
func (w *windowState) noteProbeReset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emptyProbeRuns = 0
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

func (w *windowState) openTimeFor(acct string) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.UnixMilli(w.openTimeForLocked(acct))
}

// openTimeForLocked 需持 w.mu 的毫秒级读取（调度器 s.mu 锁内复用）。
// 优先级 = 该账号识别槽 openTimeDetected[acct] → 全校识别槽 openTimeDetected["*"] → 0。
func (w *windowState) openTimeForLocked(acct string) int64 {
	ms := w.openTimeDetected[acct]
	if ms == 0 {
		ms = w.openTimeDetected["*"]
	}
	return ms
}

func (w *windowState) purge(acct string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.openTimeDetected, acct)
}

// isOpened 返回"曾开过窗"状态（对外 DTO state.WindowOpened 填充）。
func (w *windowState) isOpened() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.opened
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
