package scheduler

import (
	"xuanke-auto/backend/internal/upstream"
)

// SetHasWindowSignal 同步档案的开窗信号能力（平台切换时由装配面调用）。
//
// **平台切换必须同步它**，否则会用旧站点的信号语义判新站点：例如从知到
// （下发布级开窗布尔）切到无信号平台后，若仍按"必须见到 selectable=true"判定，
// 开窗将永远判不出来，调度器全线停摆（黄金期冲刺、提交守卫、关闭判定全部失效）。
func (s *Scheduler) SetHasWindowSignal(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasWindowSignal = v
}

// windowSignalLocked 由课程快照判定选课窗口是否开放——**开窗判定的唯一实现**。
// 两条路径（全校探测 probe() 与手动报名复核 checkClassSelectable）共用它，
// 避免同一事实两套判据在换平台后分叉。
//
// 判定顺序：
//  1. 快照为 nil → 未开窗（无数据可判）。
//  2. 档案 HasWindowSignal=false → **退化模式**：探测到非空课程数据即视为开窗。
//     绝不因平台缺此信号而拒绝工作；代价是失去黄金期 250ms 冲刺精度（退化为 1s 轮询）。
//  3. 任一发布 Selectable 指向 true → 开窗。
//  4. 有发布但全部为 false → 未开窗。
//  5. 空快照 → 未开窗（"窗口已关闭"由 windowState.isClosed 的三判据另行处理，
//     此处只答"是否开窗"，两者语义不同，不可合并）。
//
// **三态 nil（站点不下发该信号）不参与判定**——把它读成 false 会让无信号平台永远
// 判不出开窗，正是退化模式要避免的坑。
//
// 命名带 Locked 后缀是因为调用方已持 s.mu（探测热路径持锁期间调用，自带锁会死锁）。
func (s *Scheduler) windowSignalLocked(data *upstream.ElectivesData) bool {
	if data == nil {
		return false
	}
	if !s.hasWindowSignal {
		// 退化模式：站点无开窗信号，"有课程数据"即视为开窗。
		return len(data.Publishes) > 0
	}
	for _, p := range data.Publishes {
		if p.Selectable != nil && *p.Selectable {
			return true
		}
	}
	return false
}
