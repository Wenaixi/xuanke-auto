const BASE = "/api"

// 未登录（token 失效）事件：全局通知 App 登出
export const UNAUTHORIZED_EVENT = "xk:unauthorized"

export class ApiError extends Error {
  code: number
  constructor(code: number, msg: string) {
    super(msg)
    this.code = code
  }
}

// api 统一请求：非零 code 抛 ApiError
export async function api<T>(path: string, opts?: RequestInit): Promise<T> {
  const r = await fetch(BASE + path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  })
  let j: { code: number; data: T; msg: string }
  try {
    j = await r.json()
  } catch {
    throw new ApiError(-2, "服务器响应异常（HTTP " + r.status + "）")
  }
  if (j.code === -1) {
    // token 失效：广播事件，由 App 统一登出
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    throw new ApiError(j.code, j.msg || "登录已失效")
  }
  if (j.code !== 0) throw new ApiError(j.code, j.msg || "请求失败")
  return j.data
}
