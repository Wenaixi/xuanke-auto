// courseView 跨路由共享的课程展示纯函数（Select / Dashboard 收敛共用）。
// 两个路由曾各自手抄一份，任何一份改动都会造成跨页矛盾：
//   - priorityName：0=首选，1=备选 1，2=备选 2（首位不再是"备选 1"）
//   - resolveCountdownTarget：主倒计时输入——调度器识别到的开放时间（唯一事实源）
//     优先；识别缺席时用平台 begin_times[0] 兜底（避免"主看板有确切倒计时、
//     选课大厅全 00"的跨页矛盾），识别与兜底都缺席才返回 null（组件显示全 00 过期态）。
//   - formatOpenMoment：展示用绝对时间串——识别真值优先，兜底 begin_times[0]；
//     都缺席返回调用方传入的 fallback（各路由状态文案不同，保留在调用方）。
// 纯函数：不触碰 React 状态/查询，可直接进 vitest。

export function priorityName(p: number): string {
  return p === 0 ? "首选" : `备选 ${p}`
}

// countdownTargetToISO 取 begin_times[0] 并转 ISO 串（平台下发毫秒时间戳）。
// undefined/空数组返回 null——调用方据此走"未识别"分支，绝不显示编造时间。
export function countdownTargetToISO(beginTimes: readonly number[] | undefined | null): string | null {
  if (!beginTimes || beginTimes.length === 0) return null
  return new Date(beginTimes[0]).toISOString()
}

// resolveCountdownTarget 主倒计时输入：识别真值优先，缺席时 begin_times[0] 兜底，
// 都缺席返回 null（组件显全 00 + 过期态，绝不显示编造时间）。
export function resolveCountdownTarget(
  openTimeStr: string | null | undefined,
  beginTimes: readonly number[] | undefined | null
): string | null {
  return openTimeStr ?? countdownTargetToISO(beginTimes)
}

// formatOpenMoment 展示用绝对时间串：识别真值优先，缺席时 begin_times[0] 兜底，
// 都缺席返回 fallback（各路由"未识别"/同步中"文案不同，由调用方传入）。
export function formatOpenMoment(
  openTimeStr: string | null | undefined,
  beginTimes: readonly number[] | undefined | null,
  fallback: string
): string {
  const target = openTimeStr ?? countdownTargetToISO(beginTimes)
  if (!target) return fallback
  return new Date(target).toLocaleString("zh-CN", { hour12: false })
}