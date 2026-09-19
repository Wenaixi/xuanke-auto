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