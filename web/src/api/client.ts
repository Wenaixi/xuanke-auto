const BASE = "/api"

// 未登录（会话失效）事件：全局通知 App 移除对应账号会话。
// detail.session 为发起请求的 Bearer 会话令牌（401 的真实主体，App 据此反查归属账号）；
// detail.account 为 ?account= 穿透目标（管理员代看学生大厅时为学生名，仅作展示线索，
// F10-05：绝不用它判定归属——管理员自身令牌失效时 URL 上挂的是学生名，若优先取它
// 会把失效事件挂到本地无会话的学生名下，反查落空导致管理员会话永不被剔除）。
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
  // 20 秒超时兜底：目标自动保存/报名等操作若服务端挂起，前端不至于无限转圈（MAJOR-H 配套）。
  // F10-04（第 10 轮）：注释原文写"2 秒"而实现为 20 秒，行为与注释分叉。保持 20 秒——
  // 本 api() 是全站共用通道，/electives 大列表 GET 在开窗黄金期校园网下响应偏慢，
  // 收紧到 2 秒会掐断大列表刷新，在最关键的时刻引入回归；20 秒对"不无限转圈"的本意
  // 依然成立（abort 兜底），目标保存挂了还有指数退避重发兜底。
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
      // 会话过期：广播事件，附上「发起请求的会话令牌 + URL 穿透目标账号」。
      // F10-05（第 10 轮）：session 恒为 401 的真实主体，account 只作展示线索——
      // 管理员代看学生大厅时 URL account 是学生名，与失效的管理员令牌无映射；
      // 此前 account 优先导致 App 反查落空、管理员卡死在代理页。
      let account = ""
      if (path.includes("account=")) {
        const match = path.match(/[?&]account=([^&]+)/)
        if (match) account = decodeURIComponent(match[1])
      }
      window.dispatchEvent(
        new CustomEvent(UNAUTHORIZED_EVENT, { detail: { account, session } })
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
