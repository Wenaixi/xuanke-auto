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
// headers 可附加自定义头（如激活码管理接口的 X-Admin-Token）
export async function api<T>(
  path: string,
  opts?: RequestInit & { session?: string; headers?: Record<string, string> }
): Promise<T> {
  const { session, headers: extraHeaders, ...rest } = opts ?? {}
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (session) headers.Authorization = `Bearer ${session}`
  Object.assign(headers, extraHeaders ?? {})
  // 2 秒超时兜底：目标自动保存/报名等操作若服务端挂起，前端不无限转圈（MAJOR-H 配套）。
  // M-10（第 3 轮）：signal 显式接入——调用方传入 signal 时以其为准（卸载清理），
  // 否则用兜底超时信号；此前 `...rest` 会把 ctrl.signal 被调用方 signal 静默覆盖。
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), 20000)
  try {
    const r = await fetch(BASE + path, { headers, ...rest, signal: rest.signal ?? ctrl.signal })
    let j: { code: number; data: T; msg: string }
    try {
      j = await r.json()
    } catch {
      throw new ApiError(-2, "服务器响应异常（HTTP " + r.status + "）")
    }
    if (j.code === 401) {
      // 会话过期：广播事件，附带发生 401 的目标账号（避免代理查询时误杀管理员）
      let account = ""
      if (path.includes("account=")) {
        const match = path.match(/[?&]account=([^&]+)/)
        if (match) account = decodeURIComponent(match[1])
      }
      window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT, { detail: { account } }))
      throw new ApiError(j.code, j.msg || "会话已失效")
    }
    if (j.code !== 0) throw new ApiError(j.code, j.msg || "请求失败")
    return j.data
  } finally {
    clearTimeout(timer)
  }
}

// selectElective 手动报名指定选修课 (POST /api/electives/select)
export async function selectElective(
  classId: number,
  session: string,
  account?: string,
  courseName?: string
): Promise<{ msg: string; class_id: number }> {
  const q = account ? `?account=${encodeURIComponent(account)}` : ""
  return api<{ msg: string; class_id: number }>(`/electives/select${q}`, {
    method: "POST",
    session,
    body: JSON.stringify({ class_id: classId, course_name: courseName }),
  })
}

// exitElective 手动退选指定选修课 (POST /api/electives/select/exit)
export async function exitElective(
  classId: number,
  session: string,
  account?: string
): Promise<{ msg: string; class_id: number }> {
  const q = account ? `?account=${encodeURIComponent(account)}` : ""
  return api<{ msg: string; class_id: number }>(`/electives/select/exit${q}`, {
    method: "POST",
    session,
    body: JSON.stringify({ class_id: classId }),
  })
}
