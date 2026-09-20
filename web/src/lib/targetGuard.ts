import type { SchedulerState } from "../types"

// 目标自动保存前置守卫（TDD 纯函数，供 Select.tsx 防抖回调与 flushTargets 两处消费时刻
// 复用，与回显 effect 的 currentIds 过滤同判据）：
//   selected 中残留"非空数组但键不属于当前发布集合"的条目 = 发布集合整体重建后旧
//   publish_id 的课程被静默丢失的形态。此时 build() 只产出新发布课程，整包 PUT 会
//   覆盖删除后端已保存的旧目标（数据丢失）——返回 true 置脏跳过，绝不产出可 PUT 的目标。
//   空数组键 = 用户主动清空该发布（清空语义绝不复活），一律不判过期。
export function selectedHasStalePublish(
  selected: Record<number, unknown[]>,
  publishes: readonly { publish_id: number }[]
): boolean {
  const ids = new Set(publishes.map((p) => p.publish_id))
  return Object.keys(selected).some((k) => {
    if (ids.has(Number(k))) return false
    const arr = selected[Number(k)]
    return arr !== undefined && arr.length > 0
  })
}

// F40-M1：发布集合重建时清理 selected 中"非空且不在当前发布集合"的残留 key——
// 旧 publish_id 对应 Tab 已消失、用户无法通过界面清除，若守卫只置脏跳过后保存链被
// 永久静默拦截（黄金期改目标永不落库）。随重建清理即解锁，防抖重跑自然落库当前目标。
// 只删"非空且不在集合"的 key：空数组键 = 用户主动清空（清空语义绝不复活），保留。
// 无任何变更时返回原对象引用（不触发不必要的重渲染）。
export function cleanStaleSelected<T>(
  selected: Record<number, T[]>,
  currentPublishIds: readonly number[] | Set<number>
): Record<number, T[]> {
  const ids = currentPublishIds instanceof Set ? currentPublishIds : new Set(currentPublishIds)
  let changed = false
  const next: Record<number, T[]> = {}
  for (const k of Object.keys(selected)) {
    const id = Number(k)
    const arr = selected[id]
    if (!ids.has(id) && arr !== undefined && arr.length > 0) {
      changed = true
      continue // 残留非空旧 key：清理
    }
    next[id] = arr
  }
  return changed ? next : selected
}

// F42-M1：防抖保存"回显未完成"守卫判据抽纯函数——/state 首帧未到（undefined）或
// 首帧携带旧目标（courses 非空）时，后端旧目标尚未经回显合并进 selected，此刻整包
// PUT 会把后端旧目标覆盖删除（"加一门"变"替换全部"）→ 推迟保存（返回 true），置脏
// 跳过等回显完成/数据到达自愈。courses 空 = 确证后端无旧目标（回显已完成语义）→
// 放行（返回 false）。纯数据判据、不依赖 echoedRef——防抖 effect 只在 selected 变化
// 时重跑，守卫若依赖"由 /state 首帧置位的 echoedRef"，/state 持续失败期间守卫命中
// 置脏后 selected 无变化 → React bailout → 防抖订阅永不重入，保存链死锁至整页刷新；
// 守卫改由 stateData 本身驱动后，/state 数据到达触发 effect 重跑即自愈解锁。
// 第二参数 hasSelected（当前是否有任何选中课程，布尔）：区分"回显未完成"与"用户显式
// 全清空"——首帧携带旧目标但用户一个都没选 = 清空意图确凿（回显 effect 的 rev>0 且
// 无任何条目守卫已承认清空语义绝不合并旧目标），放行 PUT []，绝不把清空当"待回显"
// 打回置脏（否则清空永不落库，返回后旧目标复活=静默撤销）。
export function shouldDeferSave(
  stateData: SchedulerState | undefined,
  hasSelected: boolean
): boolean {
  return stateData === undefined || ((stateData.courses?.length ?? 0) > 0 && hasSelected)
}