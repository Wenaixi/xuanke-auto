import { describe, expect, it } from "vitest"
import {
  countdownTargetToISO,
  formatOpenMoment,
  priorityName,
  resolveCountdownTarget,
} from "./courseView"

describe("priorityName", () => {
  it("0 显示首选，正数显示备选 N", () => {
    expect(priorityName(0)).toBe("首选")
    expect(priorityName(1)).toBe("备选 1")
    expect(priorityName(2)).toBe("备选 2")
  })
})

describe("countdownTargetToISO", () => {
  it("取 begin_times[0] 转 ISO 串", () => {
    expect(countdownTargetToISO([1700000000000])).toBe(new Date(1700000000000).toISOString())
  })
  it("空数组与 undefined 返回 null", () => {
    expect(countdownTargetToISO([])).toBeNull()
    expect(countdownTargetToISO(undefined)).toBeNull()
    expect(countdownTargetToISO(null)).toBeNull()
  })
})

describe("resolveCountdownTarget", () => {
  const times = [1700000000000]
  it("识别真值优先于兜底", () => {
    expect(resolveCountdownTarget("2026-09-01T09:00:00.000Z", times)).toBe("2026-09-01T09:00:00.000Z")
  })
  it("识别缺席时用 begin_times[0] 兜底", () => {
    expect(resolveCountdownTarget(null, times)).toBe(new Date(1700000000000).toISOString())
  })
  it("识别与兜底都缺席返回 null（不显示编造时间）", () => {
    expect(resolveCountdownTarget(null, [])).toBeNull()
    expect(resolveCountdownTarget(undefined, undefined)).toBeNull()
  })
})

describe("formatOpenMoment", () => {
  const times = [1700000000000]
  it("识别真值优先", () => {
    expect(formatOpenMoment("2026-09-01T09:00:00.000Z", times, "未知")).toBe(
      new Date("2026-09-01T09:00:00.000Z").toLocaleString("zh-CN", { hour12: false })
    )
  })
  it("识别缺席用 begin_times[0] 兜底", () => {
    expect(formatOpenMoment(null, times, "未知")).toBe(
      new Date(1700000000000).toLocaleString("zh-CN", { hour12: false })
    )
  })
  it("都缺席返回 fallback（未识别文案由调用方传）", () => {
    expect(formatOpenMoment(null, [], "未识别到开放时间")).toBe("未识别到开放时间")
    expect(formatOpenMoment(undefined, undefined, "正在同步教务平台时间配置...")).toBe(
      "正在同步教务平台时间配置..."
    )
  })
})