import { describe, it, expect } from "vitest"
import { selectedHasStalePublish, cleanStaleSelected, shouldDeferSave } from "./targetGuard"
import type { SchedulerState, ClassItem } from "../types"

// C3-1：targetGuard 三纯函数正式单测（把 guard 脚本断言固化，成为 vitest 测试面的第一批）。
// 语义契约（CLAUDE.md F15/F40/F42/F43）：显式清空≠数据缺席 / 消费时刻读 ref /
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