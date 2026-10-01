import { describe, it, expect } from "vitest"
import { pickInterval } from "./pollInterval"

// 轮询间隔收敛纯函数（契约 54 窄版）：错误/窗口关闭 → 远距（30s）、开窗 → 近距（2-3s）、
// 其余 → 中档（idle 有则用 idle，无则落 far）。信号解析留在调用方（react-query 跨查询
// 读缓存由调用点处理），本函数只做「信号 → 间隔」决策。
// 契约 131：正向（error→30s 等）必须配负向（非 error 不降频）对偶断言。
describe("pickInterval", () => {
  it("error=true 恒取 far（失败降频防轰炸，负向：无 error 不降频）", () => {
    expect(pickInterval({ error: true, windowClosed: false, open: true }, { far: 30000, near: 2000 })).toBe(30000)
    expect(pickInterval({ error: false, windowClosed: false, open: true }, { far: 30000, near: 2000 })).toBe(2000)
  })
  it("windowClosed=true 恒取 far（关闭后降频——即使 open 同时为真也以关闭为准，保守降频）", () => {
    expect(pickInterval({ error: false, windowClosed: true, open: true }, { far: 30000, near: 2000 })).toBe(30000)
  })
  it("open=true 且无错误无关闭 → 取 near（开窗升频紧贴黄金期）", () => {
    expect(pickInterval({ error: false, windowClosed: false, open: true }, { far: 30000, near: 2000 })).toBe(2000)
  })
  it("有 idle 时稳态（非开窗）取 idle 中档（平台未开窗慢轮询）", () => {
    expect(pickInterval({ error: false, windowClosed: false, open: false }, { far: 30000, near: 2000, idle: 10000 })).toBe(10000)
  })
  it("无 idle 时稳态（非开窗）落 far（与既有 Select state 30000/2000 两档行为一致）", () => {
    expect(pickInterval({ error: false, windowClosed: false, open: false }, { far: 30000, near: 2000 })).toBe(30000)
  })
  it("error 优先于 windowClosed（两者恒同值 30000，防将来分叉）", () => {
    // 负向对偶：error=false 时 windowClosed=true 仍是 far——不是只测 error 一条
    expect(pickInterval({ error: false, windowClosed: true, open: false }, { far: 30000, near: 2000, idle: 10000 })).toBe(30000)
  })
})
