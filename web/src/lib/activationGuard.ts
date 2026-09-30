import { ApiError } from "../api/client"

// 业务码（与后端 internal/api 的常量一一对应）：前端据 code 分流，绝不匹配 msg
// 中文文案。后端改文案不该让前端分支静默失效，而 code 是稳定契约。
//
// code=0 成功、code=401 会话失效、code=1 通用失败（"重试可能有用"）；
// 下列专属码表示"重试没有意义"或"下一步操作不同"。
export const BIZ_CODE = {
  /** 教务登录成功但该账号未激活，data 携带门票。 */
  notActivated: 1001,
  /** 激活码机制被关闭（XUANKE_ACTIVATION=off）。 */
  activationDisabled: 1002,
  /** 门票无效、已用尽或与本次账号不匹配（单次防重放）。 */
  ticketInvalid: 1003,
} as const

function codeOf(err: unknown): number | null {
  return err instanceof ApiError ? err.code : null
}

// 激活码机制已关闭：重试永远不成功（机制确实关着），界面必须换成"去系统配置
// 打开"的出口，而不是"加载失败 + 重试"——后者会让管理员当成故障反复点。
export function isActivationDisabled(err: unknown): boolean {
  return codeOf(err) === BIZ_CODE.activationDisabled
}

// 门票无效：用户该做的是重新登录拿新门票，而不是检查激活码。两者下一步操作
// 完全不同，混淆会把激活码输错的人送去重登并掩盖真实原因。
export function isTicketInvalid(err: unknown): boolean {
  return codeOf(err) === BIZ_CODE.ticketInvalid
}
