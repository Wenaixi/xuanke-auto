const BASE = "/api"

// 未登录（会话失效）事件：全局通知 App 移除对应账号会话
export const UNAUTHORIZED_EVENT = "xk:unauthorized"

export class ApiError extends Error {
  code: number
  constructor(code: number, msg: string) {
    super(msg)
    this.code = code
  }
}

// api 统一请求：非零 code 抛 ApiError；session 为服务端签发的会话令牌（Bearer 认证）
export async function api<T>(path: string, opts?: RequestInit & { session?: string }): Promise<T> {
  const { session, ...rest } = opts ?? {}
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (session) headers.Authorization = `Bearer ${session}`
  const r = await fetch(BASE + path, { headers, ...rest })
  let j: { code: number; data: T; msg: string }
  try {
    j = await r.json()
  } catch {
    throw new ApiError(-2, "服务器响应异常（HTTP " + r.status + "）")
  }
  if (j.code === 401) {
    // 会话过期：广播事件，由 App 移除该账号会话
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    throw new ApiError(j.code, j.msg || "会话已失效")
  }
  if (j.code !== 0) throw new ApiError(j.code, j.msg || "请求失败")
  return j.data
}
