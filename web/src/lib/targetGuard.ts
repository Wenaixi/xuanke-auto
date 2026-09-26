import type { SchedulerState, Target } from "../types"

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

// 发布集合重建时清理 selected 中"非空且不在当前发布集合"的残留 key——
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

// 防抖保存"回显未完成"守卫判据抽纯函数——/state 首帧未到（undefined）或
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
// 第三参数 echoed（回显是否已完成，消费点传 echoedRef.current）：
// "courses 非空 && 有选中"只该在"回显尚未完成"时推迟——已回显完成的账号（echoed=true），
// 后端旧目标已合并进 selected、selected 完整无缺，整包 PUT 与后端一致，继续推迟
// 会把后续所有编辑永久闷死（courses 永驻非空 + selected 无变化 bailout = 无解锁信号，
// 编辑永不落库、返回后旧目标覆盖=静默撤销）。稳态（echoed=true）放行普通编辑；
// 暂态（echoed=false）仍推迟（回显未完成）。首帧未到（stateData===undefined）无条件
// 推迟保留——回显未发生时 selected 只含用户新改动，整包 PUT 会覆盖删除后端旧目标。
export function shouldDeferSave(
  stateData: SchedulerState | undefined,
  hasSelected: boolean,
  echoed: boolean
): boolean {
  if (stateData === undefined) return true
  if (echoed) return false
  return (stateData.courses?.length ?? 0) > 0 && hasSelected
}

// 消费时刻守卫编排：把 flushTargets 与防抖 effect 两条保存路径上逐字重复的
// 三段前置守卫（回显未完成 → 发布缺席 → 旧 publish_id 残留）收成单一判据，
// 使"判据顺序"与"拦截语义"各自只有一处定义。
//
// 顺序是契约而非实现细节：defer 优先于其余两因——/state 数据缺席时任何拦截都
// 属"等自愈"，此时弹 stale 提示会把一次正常等待误报成"发布已更新"；而
// missingPublishes 优先于 stalePublish，因为发布集合整体重建会同时造成两种形态
// （集合为空 + 旧 key 残留），此时真正的主因是数据尚未到达。
//
// 返回拦截原因而非仅布尔：调用方据此决定是否提示用户。stalePublish 是唯一
// 需要提示的一因（残留目标用户无法通过界面自行清除，须告知刷新解锁）；
// defer 与 missingPublishes 均为安全拦截的静默跳过（显式清空不等于数据缺席，
// 用户主动清空绝不判缺席）。
export type CommitBlockReason = "defer" | "missingPublishes" | "stalePublish"
export type CommitVerdict = { ok: true } | { ok: false; reason: CommitBlockReason }

export function guardCommit(
  stateData: SchedulerState | undefined,
  hasSelected: boolean,
  echoed: boolean,
  selected: Record<number, unknown[]>,
  publishes: readonly { publish_id: number }[]
): CommitVerdict {
  if (shouldDeferSave(stateData, hasSelected, echoed)) {
    return { ok: false, reason: "defer" }
  }
  // "发布缺席 + 已有选中" = 数据缺席绝非用户清空意图，保留脏绝不 PUT [] 假清空；
  // selectedCount 偏保守安全。无选中时属用户主动清空，放行。
  if (publishes.length === 0 && hasSelected) {
    return { ok: false, reason: "missingPublishes" }
  }
  // 发布集合整体重建后 selected 残留旧 publish_id 的非空条目——build() 只
  // 遍历当前发布集合会静默丢弃它们，产出"仅含新发布课程"的整包 PUT 覆盖删除
  // 后端旧目标（数据丢失）。空数组键 = 用户主动清空，绝不判过期。
  if (selectedHasStalePublish(selected, publishes)) {
    return { ok: false, reason: "stalePublish" }
  }
  return { ok: true }
}

// 目标集构建：由 [发布 × 已选课程] 联查生成后端 PUT 用的目标数组。
// priority 取课程在该发布内的下标（前端 Tab 内顺序即优先级）。
// 两个消费点（flushTargets 与防抖 effect）的唯一差异是 selected 的数据源
// （ref 镜像 vs 渲染闭包），故 sel 由调用方传入，函数内部绝不自行取数——
// 消费时刻必须读调用方的镜像，这是「async 闭包捕获悖论」的防线。
export function buildTargets(
  pubs: readonly { publish_id: number }[],
  sel: Record<number, { id: number; publish_id: number; course_name: string }[]>
): Target[] {
  const targets: Target[] = []
  for (const p of pubs) {
    const list = sel[p.publish_id] ?? []
    list.forEach((cls, i) => {
      targets.push({
        publish_id: p.publish_id,
        class_id: cls.id,
        course_name: cls.course_name,
        priority: i,
      })
    })
  }
  return targets
}

// 校验目标集的 publish_id 全属当前发布集——防「渲染→回调」窗口内平台重建
// 发布集合造成的错位（此时联查会静默产出只含新发布的目标，整包 PUT 覆盖删除
// 后端旧目标）。
// 空 targets 时 every 恒真——空集防御不在这里，而在「联查产物为空 + 已有选中
// = 假清空」那条独立判据（见 commitTargetsGuards）。
export function targetsUseCurrentPublishes(
  targets: readonly { publish_id: number }[],
  pubs: readonly { publish_id: number }[]
): boolean {
  const ids = new Set(pubs.map((p) => p.publish_id))
  return targets.every((t) => ids.has(t.publish_id))
}

// 联查产物的两条错位防线，返回拦截原因供调用方决定是否提示（均静默跳过，
// 不置 dirtyRef——终局绝不误报保存失败）。
export type BuildBlockReason = "emptyWithSelection" | "stalePublishId"

export function commitTargetsGuards(
  next: readonly Target[],
  selectedCount: number,
  pubs: readonly { publish_id: number }[]
): { ok: true } | { ok: false; reason: BuildBlockReason } {
  // "联查产物为空 = 假清空"——every 校验对空集恒真，必须独立判
  // 「selectedCount>0 却产出空集」。仅在发布全缺席的极限情况 selectedCount 可能滞后，
  // 为用户误伤守卫（仅多等一次防抖），安全方向；真实假清空绝不放过。
  if (next.length === 0 && selectedCount > 0) {
    return { ok: false, reason: "emptyWithSelection" }
  }
  // 发布 id 漂移：回调窗口内发布重建会让 next 携带漂移 id，错位假清空绝不 PUT。
  if (!targetsUseCurrentPublishes(next, pubs)) {
    return { ok: false, reason: "stalePublishId" }
  }
  return { ok: true }
}