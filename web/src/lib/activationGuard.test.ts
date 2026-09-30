import { describe, expect, it } from "vitest"
import { ApiError } from "../api/client"
import { isActivationDisabled, isTicketInvalid } from "./activationGuard"

// 前端错误分流判据：只认后端业务码，绝不匹配 msg 中文文案。
//
// 背景：Admin 页曾用 codesErrMsg.includes("激活码机制已关闭")、登录页曾用
// /激活票据无效或已过期/.test(e.message) 决定 UI 分支。后端改一个字，分支就
// 静默退回"加载失败 + 重试"，而这两类失败重试永远不成功——用户会当成故障反复点。
//
// 关键断言：msg 完全相同但 code 不同，必须判为不同分支（这正是文案匹配的失败模式）。

describe("isActivationDisabled", () => {
  it("专属业务码 1002 = 机制已关闭", () => {
    expect(isActivationDisabled(new ApiError(1002, "激活码机制已关闭"))).toBe(true)
  })

  it("msg 相同但 code 是通用失败码 1 时不得误判为机制关闭", () => {
    // 这条是文案匹配方案的根本缺陷：同样的文案走不同分支，后端无法安全改字
    expect(isActivationDisabled(new ApiError(1, "激活码机制已关闭"))).toBe(false)
  })

  it("其它业务码（未激活 1001 / 票据无效 1003）都不是机制关闭", () => {
    expect(isActivationDisabled(new ApiError(1001, "该账号尚未激活"))).toBe(false)
    expect(isActivationDisabled(new ApiError(1003, "激活票据无效或已过期"))).toBe(false)
  })

  it("非 ApiError（网络异常 / 超时）一律判为真加载失败", () => {
    expect(isActivationDisabled(new Error("Failed to fetch"))).toBe(false)
    expect(isActivationDisabled(null)).toBe(false)
    expect(isActivationDisabled(undefined)).toBe(false)
    expect(isActivationDisabled("激活码机制已关闭")).toBe(false)
  })
})

describe("isTicketInvalid", () => {
  it("专属业务码 1003 = 门票无效", () => {
    expect(isTicketInvalid(new ApiError(1003, "激活票据无效或已过期，请重新登录后再激活"))).toBe(true)
  })

  it("msg 相同但 code 是通用失败码 1 时不得误判为门票无效", () => {
    // 该分支引导用户"重新登录"，误判会把激活码输错的人送去重登，掩盖真实原因
    expect(isTicketInvalid(new ApiError(1, "激活票据无效或已过期"))).toBe(false)
  })

  it("机制关闭（1002）不是门票无效", () => {
    expect(isTicketInvalid(new ApiError(1002, "激活码机制已关闭"))).toBe(false)
  })

  it("非 ApiError 一律 false", () => {
    expect(isTicketInvalid(new Error("timeout"))).toBe(false)
    expect(isTicketInvalid(null)).toBe(false)
  })
})
