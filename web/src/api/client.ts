const BASE = "/api"

// 未登录（会话失效）事件：全局通知 App 移除对应账号会话。
// detail.account 为失效请求的归属账号：优先取 URL 的 ?account= 参数；
// 无该参数（如 /state、/logs 按会话隔离的请求）时回退为发起请求时
// 注入的会话令牌（session）——App 持有 sessions 映射可反查账号。
// 避免 401 迟到返回时事件监听器闭包里的 current 已切到其他账号而误杀。
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
  // F7-09（第 7 轮）：abort 映射为友好文案——此前原生 AbortError("This operation was
  // aborted") 直接进 toast，用户看不懂。
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
      // 会话过期：广播事件，附带发生 401 的目标账号（避免代理查询时误杀管理员）。
      // account 优先取 URL 参数；无参数时用发起请求的会话令牌（App 侧反查账号），
      // 杜绝慢请求乱序返回时按闭包 current 误删其他账号（第 4 轮前端审查问题 3）。
      let account = ""
      if (path.includes("account=")) {
        const match = path.match(/[?&]account=([^&]+)/)
        if (match) account = decodeURIComponent(match[1])
      }
      window.dispatchEvent(
        new CustomEvent(UNAUTHORIZED_EVENT, { detail: { account: account || session } })
      )
      throw new ApiError(j.code, j.msg || "会话已失效")
    }
    if (j.code !== 0) throw new ApiError(j.code, j.msg || "请求失败")
    return j.data
  } catch (e) {
    // F7-09：AbortError 无法识别（调用方 signal 或超时 abort 均触发）——统一映射为
    // 超时友好文案，绝不把原生 "This operation was aborted" 泄漏给用户
    if (e instanceof Error && e.name === "AbortError") {
      throw new ApiError(-2, "请求超时，请重试")
    }
    throw e
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

// logout 注销当前会话（POST /api/logout，M-7）：
// 令牌被服务端立即作废——即使浏览器端 localStorage 被窃取/复制，令牌也已失效。
// 失败（网络抖动）不阻塞前端本地登出（会话即将过期，最终由服务端 12h TTL 兜底）。
export async function logout(session: string): Promise<void> {
  try {
    await api("/logout", { method: "POST", session, body: "{}" })
  } catch {
    // 静默：登出是尽力而为，本地已登出即达到目的
  }
}
