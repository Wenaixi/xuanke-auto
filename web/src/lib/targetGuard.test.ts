import { describe, it, expect } from "vitest"
import {
  selectedHasStalePublish,
  cleanStaleSelected,
  shouldDeferSave,
  guardCommit,
  buildTargets,
  targetsUseCurrentPublishes,
  commitTargetsGuards,
} from "./targetGuard"
import type { SchedulerState, ClassItem } from "../types"

// targetGuard 三纯函数正式单测（把 guard 脚本断言固化，成为 vitest 测试面的第一批）。
// 语义契约（见根 CLAUDE.md）：显式清空≠数据缺席 / 消费时刻读 ref /
// 发布重建清理 / 回显未完成推迟。

const cls = (id: number, publishId: number): ClassItem =>
  ({ id, publish_id: publishId, course_name: "x" } as ClassItem)

// 构造最小 SchedulerState（只关心 courses 字段，其余字段用 as any 避免类型完备性摩擦）
const st = (courses: unknown[]): SchedulerState => ({ courses } as SchedulerState)

describe("selectedHasStalePublish", () => {
  it("非空数组键不在当前发布集合 = 过期", () => {
    expect(selectedHasStalePublish({ 999: [cls(1, 999)] }, [{ publish_id: 1 }])).toBe(true)
  })
  it("空数组键绝不判过期（清空语义不复活）", () => {
    expect(selectedHasStalePublish({ 999: [] }, [{ publish_id: 1 }])).toBe(false)
  })
  it("全部合法键 = 不过期", () => {
    expect(selectedHasStalePublish({ 1: [cls(10, 1)] }, [{ publish_id: 1 }, { publish_id: 2 }])).toBe(false)
  })
})

describe("shouldDeferSave", () => {
  it("首帧未到（stateData undefined）无条件推迟", () => {
    expect(shouldDeferSave(undefined, true, false)).toBe(true)
  })
  it("已回显稳态（echoed=true）放行普通编辑", () => {
    expect(shouldDeferSave(st([{}]), true, true)).toBe(false)
  })
  it("courses 非空 + 有选中 + 未回显 = 推迟（回显未完成）", () => {
    expect(shouldDeferSave(st([{}]), true, false)).toBe(true)
  })
  it("courses 非空 + 无选中（显式清空）+ 未回显 = 放行 PUT []（清空≠数据缺席）", () => {
    expect(shouldDeferSave(st([{}]), false, false)).toBe(false)
  })
  it("courses 空 = 确证后端无旧目标 → 放行", () => {
    expect(shouldDeferSave(st([]), true, false)).toBe(false)
  })
})

describe("cleanStaleSelected", () => {
  it("清理非空过期 key + 保留空 key + 保留合法 key", () => {
    const input = { 999: [cls(1, 999)], 888: [], 1: [cls(10, 1)] }
    const out = cleanStaleSelected(input, new Set([1, 2]))
    expect(out).not.toHaveProperty("999")
    expect(out).toHaveProperty("888")
    expect(out[1]).toHaveLength(1)
  })
  it("无变更返回原引用（不触发不必要重渲染）", () => {
    const input = { 1: [cls(10, 1)] }
    expect(cleanStaleSelected(input, new Set([1]))).toBe(input)
  })
})

describe("guardCommit", () => {
  const pubs = [{ publish_id: 1 }, { publish_id: 2 }]
  const live = { 1: [cls(10, 1)] }

  it("正常路径放行（返回 ok）", () => {
    expect(guardCommit(st([]), true, true, live, pubs)).toEqual({ ok: true })
  })
  it("回显未完成 → defer（先于其余守卫，不提示）", () => {
    expect(guardCommit(st([{}]), true, false, live, pubs)).toEqual({ ok: false, reason: "defer" })
  })
  it("发布缺席 + 有选中 → missingPublishes（数据缺席非清空意图）", () => {
    expect(guardCommit(st([]), true, true, live, [])).toEqual({
      ok: false,
      reason: "missingPublishes",
    })
  })
  it("发布缺席 + 显式全清空（selected 空）→ 放行，绝不拦清空", () => {
    // 自洽形态：selected 为空对象 ⟹ 归约必然得出 hasSelected=false。
    // 用户主动清空全部目标时，即便发布集合缺席（平台尚未下发/开窗瞬间清空）也放行
    // PUT []——清空是明确用户意图，绝不可被任何守卫拦成"静默丢弃"。
    expect(guardCommit(st([]), false, true, {}, [])).toEqual({ ok: true })
  })
  it("残留旧 publish_id → stalePublish（须提示用户解锁）", () => {
    expect(guardCommit(st([]), true, true, { 999: [cls(1, 999)] }, pubs)).toEqual({
      ok: false,
      reason: "stalePublish",
    })
  })
  it("多个拦截同时成立时按 defer > missingPublishes > stalePublish 取首因", () => {
    // /state 首帧未到 + 发布缺席 + 旧 key 残留：defer 优先（等数据自愈，不打扰用户）
    expect(guardCommit(undefined, true, false, { 999: [cls(1, 999)] }, [])).toEqual({
      ok: false,
      reason: "defer",
    })
  })
  it("hasSelected 由调用方按 selected 归约得出，本函数不自行取数", () => {
    // 空 selected + 有发布 → 无残留、无缺席，放行（对应用户尚未勾选任何课）
    expect(guardCommit(st([]), false, true, {}, pubs)).toEqual({ ok: true })
  })
})

describe("buildTargets", () => {
  it("按 [发布 × 已选] 联查产出目标，priority 取发布内下标", () => {
    const sel = { 1: [cls(10, 1), cls(11, 1)], 2: [cls(20, 2)] }
    expect(buildTargets([{ publish_id: 1 }, { publish_id: 2 }], sel)).toEqual([
      { publish_id: 1, class_id: 10, course_name: "x", priority: 0 },
      { publish_id: 1, class_id: 11, course_name: "x", priority: 1 },
      { publish_id: 2, class_id: 20, course_name: "x", priority: 0 },
    ])
  })
  it("发布顺序决定产出顺序（不按 selected 键序）", () => {
    const sel = { 2: [cls(20, 2)], 1: [cls(10, 1)] }
    expect(buildTargets([{ publish_id: 1 }, { publish_id: 2 }], sel).map((t) => t.publish_id)).toEqual([1, 2])
  })
  it("已选课程所属发布不在当前发布集 → 不产出（交由漂移守卫拦发布侧问题）", () => {
    expect(buildTargets([{ publish_id: 1 }], { 999: [cls(99, 999)] })).toEqual([])
  })
  it("空发布集或空选中 → 空目标集", () => {
    expect(buildTargets([], { 1: [cls(10, 1)] })).toEqual([])
    expect(buildTargets([{ publish_id: 1 }], {})).toEqual([])
  })
})

describe("targetsUseCurrentPublishes", () => {
  it("全部目标属当前发布集 → 通过", () => {
    expect(targetsUseCurrentPublishes([{ publish_id: 1 }], [{ publish_id: 1 }, { publish_id: 2 }])).toBe(true)
  })
  it("携带漂移 publish_id → 不通过", () => {
    expect(targetsUseCurrentPublishes([{ publish_id: 1 }, { publish_id: 999 }], [{ publish_id: 1 }])).toBe(false)
  })
  it("空目标集恒真（空集防御不在此处，见 commitTargetsGuards）", () => {
    expect(targetsUseCurrentPublishes([], [])).toBe(true)
  })
})

describe("commitTargetsGuards", () => {
  it("联查为空但有选中 = 假清空 → 拦截（绝不整包 PUT [] 覆盖删除后端目标）", () => {
    expect(commitTargetsGuards([], 3, [{ publish_id: 1 }])).toEqual({
      ok: false,
      reason: "emptyWithSelection",
    })
  })
  it("联查为空且无选中 = 用户显式清空 → 放行", () => {
    expect(commitTargetsGuards([], 0, [{ publish_id: 1 }])).toEqual({ ok: true })
  })
  it("携带漂移 publish_id → 拦截", () => {
    expect(commitTargetsGuards([{ publish_id: 999, class_id: 1, course_name: "x" }], 1, [{ publish_id: 1 }])).toEqual({
      ok: false,
      reason: "stalePublishId",
    })
  })
  it("两因同时成立时取 emptyWithSelection（联查空是更根本的形态）", () => {
    // 顺序契约：先判空集再判漂移——空集时 every 恒真，漂移判据无意义
    expect(commitTargetsGuards([], 2, [])).toEqual({ ok: false, reason: "emptyWithSelection" })
  })
  it("正常路径放行", () => {
    expect(commitTargetsGuards([{ publish_id: 1, class_id: 10, course_name: "x", priority: 0 }], 1, [{ publish_id: 1 }])).toEqual({
      ok: true,
    })
  })
})