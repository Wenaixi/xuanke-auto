import { useEffect, useMemo, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import Login from "./routes/Login"
import Dashboard from "./routes/Dashboard"
import Select from "./routes/Select"
import Admin from "./routes/Admin"
import { ToastProvider } from "./components/ui/Toast"
import { logout as apiLogout, UNAUTHORIZED_EVENT } from "./api/client"
import type { Account, Sessions } from "./types"
import { isCurrentAdminSession } from "./lib/adminAuth"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 0 } },
})

// 本地会话映射存取：账号名 -> 服务端签发令牌（多账号互不干扰）
// N4：localStorage 在隐私模式/配额受限时可读不可写，try/catch 静默降级为内存态
function loadSessions(): Sessions {
  try {
    return JSON.parse(localStorage.getItem("xk_sessions") || "{}")
  } catch {
    return {}
  }
}

function saveSessions(next: Sessions) {
  try {
    localStorage.setItem("xk_sessions", JSON.stringify(next))
  } catch {
    // N4：存储不可用（隐私模式/配额受限）时静默降级——会话仅存内存，刷新即需重新登录
  }
}
// 管理员账号名持久化：后端 XUANKE_ADMIN_NAME 可自定义，登录响应回 adminName——本地
// 保存下来，刷新后仍能识别"当前账号即管理员"（否则自定义名刷新落回默认 admin，管理员
// 被判定成普通学生掉回学生看板）。与 xk_sessions 同款 try/catch 降级（存不了就内存态，
// 每次登录响应重新同步，行为不损坏）。
function loadAdminName(): string {
  try {
    return localStorage.getItem("xk_admin_name") || "admin"
  } catch {
    return "admin"
  }
}
function saveAdminName(name: string) {
  try {
    localStorage.setItem("xk_admin_name", name)
  } catch {
    // N4：存储不可用时静默降级，下次登录重新同步
  }
}
// 管理员会话令牌持久化：仅由「登录响应带 adminName」写入。刷新后管理页恢复的
// 唯一判据 =「当前管理员名账号的会话 token === 此标记 token」——撞名学生（账号名
// 恰等于管理员名、放行的普通教务会话）登录响应的会话 token 永远匹配不上
// 此标记，刷新不会误进管理页。与 xk_admin_name 同款 try/catch 降级。
function loadAdminToken(): string {
  try {
    return localStorage.getItem("xk_admin_token") || ""
  } catch {
    return ""
  }
}
function saveAdminToken(tok: string) {
  try {
    localStorage.setItem("xk_admin_token", tok)
  } catch {
    // N4：存储不可用时静默降级，下次管理登录重新同步
  }
}

export default function App() {
  const [sessions, setSessions] = useState<Sessions>(loadSessions)
  const [page, setPage] = useState<"dashboard" | "select">("dashboard")
  const [current, setCurrent] = useState<Account>("")
  const [inAdmin, setInAdmin] = useState(false)
  const [targetAccount, setTargetAccount] = useState<Account | null>(null)
  // 管理员账号名：登录返回 adminName 时同步（后端 XUANKE_ADMIN_NAME 决定，默认 admin）；
  // 初始值读 localStorage 恢复，刷新后自定义管理员名不丢
  const [adminName, setAdminName] = useState<string>(loadAdminName)
  // 曾标记为管理会话的会话令牌：仅由「登录响应带 adminName」写入。刷新后管理页
  // 恢复的唯一判据 = 当前管理员名账号的会话 token === 此标记 token；账号名等于
  // 管理员名绝不是管理标志（撞名学生放行的普通教务会话账号名也可等于
  // 管理员名，拿名字判定会把普通会话误认成管理会话，永锁 403 管理页）。
  const [adminToken, setAdminToken] = useState<string>(loadAdminToken)

  // 账号列表派生值用 useMemo 稳定引用：Object.keys 每次渲染新建数组会让下方账号迁移
  // effect 每次渲染都重跑（体内仅条件 setState，当前无功能影响，属反模式）。
  const accounts = useMemo(() => Object.keys(sessions), [sessions])
  const sessionToken = current ? sessions[current] : undefined

  // 登录成功：写入（或覆盖）该账号会话，登录即自动加入账号列表
  const login = (token: string, account: Account, adminName?: string) => {
    if (adminName) {
      setAdminName(adminName)
      saveAdminName(adminName)
      // 登录响应带 adminName 即确证"本会话是管理会话"——标记该 token 为管理令牌，
      // 刷新后凭此恢复管理页（判定见渲染处 adminToken 态）。撞名学生教务登录响应
      // 无 adminName（放行），绝不误标。
      setAdminToken(token)
      saveAdminToken(token)
    }
    const next = { ...loadSessions(), [account]: token }
    saveSessions(next)
    setSessions(next)
    setCurrent(account)
    // 管理员登录（响应带 adminName）后直接进入管理员界面；撞名学生虽账号名等于
    // 管理员名但响应无 adminName → 不进入（下 render 渲染判据再兜底一次）
    setInAdmin(!!adminName)
  }
  // 退出当前账号：仅移除该账号会话，其他账号保留。
  // M-7：登出前调用后端 /api/logout 作废服务端令牌（浏览器本地删除只是第一步，
  // 令牌被复制/窃取后仍在服务端有效——登出即吊销，杜绝令牌外流残留）。
  const logout = (token: string) => {
    apiLogout(token)
    const next = { ...loadSessions() }
    delete next[current]
    saveSessions(next)
    setSessions(next)
    setInAdmin(false)
    // 管理令牌标记随登出一并清除（本地会话已删，标记 token 失去意义；
    // 残留不影响判定——sessions[admin] 已删，isCurrentAdminSession 恒 false）
    setAdminToken("")
    saveAdminToken("")
    // 清除代理目标账号——管理员代理查看学生大厅退出后，重登同一
    // 管理员若不重置 targetAccount，渲染会直接命中代理分支再次掉进学生 Select，
    // 跳过管理页；登出即放弃代理态，重登后回到管理页。
    setTargetAccount(null)
    // 同步重置 page 视图态——App 组件永挂载（Login 只是条件渲染
    // 分支），page 不随 sessions 清空而重置：学生在选课大厅被 401 吊销（教务 token
    // 过期）后重新登录，page 残留 "select" 会跳过 Dashboard 直接掉进选课大厅；
    // 与 targetAccount 同族，登出即回到初始 dashboard 视图。
    setPage("dashboard")
  }

  // 当前账号的会话被剔除后自动切到剩余账号（无账号则回登录页）
  useEffect(() => {
    if (accounts.length === 0) {
      setCurrent("")
      // 会话全部清空回登录页时必须重置 page——401 被动吊销
      // 不经过 logout()：学生在选课大厅被吊销（唯一账号）→ 回登录页 → 重新登录后
      // page 残留 "select" 跳过 Dashboard 直接掉进选课大厅（与主动登出同族）。
      // 登出/吊销即回到初始 dashboard 视图。
      setPage("dashboard")
    } else if (!current || !accounts.includes(current)) {
      setCurrent(accounts[0])
    }
    // 当前账号已不是管理员时退出管理态（判据同渲染：仅会话 token 与标记一致才算）
    if (!isCurrentAdminSession(loadSessions(), adminName, adminToken)) setInAdmin(false)
  }, [accounts, current, adminName, adminToken])

  // 管理员删除账号后同步清理本地会话——后端 DeleteAccount 只清
  // 服务端（6 表事务 + RevokeAccount 吊销会话），localStorage 的 xk_sessions 若残留该
  // 账号条目，会继续占账号槽位、刷新复活，直到下次请求 401 才被吊销链摘除。快照式
  // 三连（与 logout/onUnauthorized 同款）：从最新快照删该账号 → 落盘 → setState。
  // 同时若被删账号恰是"当前正在查看的账号"（current）或"代理中的学生账号"
  // （targetAccount），必须同步退出对应视图态：渲染 `targetAccount ? <Select>` 在
  // Admin 之前，targetAccount 残留会让管理员卡死在已删账号的代理页（代理态同族）。
  // 注意：绝不调用 logout()——删除的是他人账号，用当前管理员令牌调 /logout 语义
  // 完全错误（会登出管理员自己）；且服务端会话已由后端 RevokeAccount 吊销，本地
  // 清除不会造成"令牌仍有效"的残留。
  const onDeleted = (acct: string) => {
    const snap = loadSessions()
    if (snap[acct] === undefined) return
    const next = { ...snap }
    delete next[acct]
    saveSessions(next)
    setSessions(next)
    setTargetAccount((prev) => (prev === acct ? null : prev))
    if (current === acct) setCurrent("")
    // 删除的是管理员自己：必须同步退出管理态——否则 inAdmin 残留 true，账号迁移
    // effect 把 current 切到剩余学生账号后，渲染仍命中 Admin 分支，拿学生令牌去拉
    // 五个管理 Tab 连环 401（与 onUnauthorized 同族）。
    if (acct === adminName) {
      setInAdmin(false)
      // 管理令牌标记一并清除——被删的是管理员账号本身，会话已消失，标记失去意义
      setAdminToken("")
      saveAdminToken("")
    }
  }

  // 后端返回 401（会话过期）：剔除失效账号的令牌（前端失效防御）。
  // detail.account 已由 client.ts 统一为"账号名 或 会话令牌"——按令牌反查
  // 不到账号时（如本地已注销）跳过，杜绝慢请求乱序返回时按闭包 current 误杀其他账号。
  // 归属判定一律以 detail.session（发起请求的 Bearer 令牌）为准——
  // 它才是 401 的真实主体；detail.account（?account= 穿透目标）仅在 session 缺失时兜底，
  // 且先按"账号名直查"再按"令牌反查"，反查无果直接跳过（管理员代理页停留期间自身会话
  // 过期：事件带管理员令牌 → 反查出管理员 → 正常剔除回登录页，不再卡死在代理页）。
  useEffect(() => {
    const onUnauthorized = (e: Event) => {
      const detail = (e as CustomEvent)?.detail
      const lostRaw = detail?.session || detail?.account || current
      // 反查归属账号：lostRaw 可能是会话令牌（无 ?account= 的请求）也可能是账号名；
      // 先按账号名直查 sessions 映射，查不到再按令牌值反查，仍无则跳过（非本机会话）。
      const sessionsSnap = loadSessions()
      const lostAccount =
        sessionsSnap[lostRaw] === undefined
          ? Object.keys(sessionsSnap).find((k) => sessionsSnap[k] === lostRaw)
          : lostRaw
      if (!lostAccount) return
      // 被吊销的账号恰是管理员当前代理查看的学生账号时，先退出代理视图——
      // 该学生会话已失效，继续停留只会拿着管理员令牌替它代操作。
      // 被吊销的是管理员自身会话时同样必须退出代理态——
      // 代理凭据随管理员会话一起没了，targetAccount 残留会让管理员重登后渲染直接
      // 命中代理 Select 跳过管理页（主动 logout 只覆盖主动退出，被动吊销不对称）。
      setTargetAccount((prev) =>
        prev === lostAccount || lostAccount === adminName ? null : prev
      )
      // 被吊销的是管理员自身会话：除退出代理态外必须同步退出管理态——否则
      // sessions[admin] 删除后 inAdmin 仍为 true，current 切到剩余学生账号时渲染
      // 命中 Admin 分支，拿学生令牌去拉管理接口（403 假象 + 连锁触发 401 误删学生会话）。
      if (lostAccount === adminName) {
        setInAdmin(false)
        // 管理令牌标记一并清除——被吊销的就是管理员会话，标记失去意义
        setAdminToken("")
        saveAdminToken("")
      }
      // 管理员处于后台管理态代理查看学生大厅时，若发生 401 绝不误杀管理员自身会话
      if (isCurrentAdminSession(loadSessions(), adminName, adminToken) && inAdmin && lostAccount !== adminName) {
        return
      }
      // 把落盘放进 setSessions updater 之后的 if——但 React 的
      // updater 不在 setState 调用栈内同步执行（render 阶段才跑），removedNext 求值时
      // 恒为 false，localStorage 永不更新 → 401 剔除的失效账号刷新后复活。改为与
      // login/logout 完全一致的"快照→改→落盘→setState"同步链路（幂等，无覆盖丢失），
      // updater 无副作用的要求通过"根本不写 updater"达成。
      const snap = loadSessions()
      if (snap[lostAccount] === undefined) return
      const next = { ...snap }
      delete next[lostAccount]
      saveSessions(next)
      setSessions(next)
    }
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [adminName, inAdmin, adminToken])

  // 壁纸滚动统一机制（手机 + 电脑同一套，仅放大比例不同）：
  //   背景随下滑“往上走”：滚动页面时图片以 1:1 速度上移（露出图片下方），
  //   当图片底边(图顶+图高)到达屏幕底边(视口高)即冻结，继续下滑图片不动、
  //   底边恰贴屏幕底绝不越出底部；上滑则 1:1 回落。
  // 数学表达（唯一公式）：shift = −clamp(scrollY, 0, 图片高 − 视口高)（负值=上移）
  //   移动端 180% / 桌面 160% 仅 CSS 宽度不同，JS 直接量 img 真实高度，无需区分设备。
  // 性能：图片高度纯 mount/resize/图片加载时量测一次缓存，滚动回调零布局读取；
  //   位移直接写 transform 具体像素（GPU 合成轨道），滚动全程流畅无抖动。
  useEffect(() => {
    const img = document.querySelector<HTMLElement>(".canvas-bg-img")
    if (!img) return
    let maxShift = 0
    let frame = 0
    /* 量测图片真实渲染高度 − 视口高（仅装载/窗口变化/图片加载时执行，绝不进滚动路径） */
    const measure = () => {
      maxShift = Math.max(0, img.getBoundingClientRect().height - window.innerHeight)
      apply()
    }
    const apply = () => {
      /* 取整：浏览器滚动本质整数像素，消除亚像素插值导致的抖动 */
      const y = Math.round(window.scrollY)
      /* shift = −clamp(y, 0, maxShift)：负值上移，未触底前 1:1 跟随，触底冻结 */
      const shift = -Math.min(Math.max(0, y), maxShift)
      img.style.transform = `translateY(${shift}px)`
      frame = 0
    }
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(apply)
    }
    measure()
    /* 图片未加载完前 height 可能为 0，加载完成后重新量测一次 */
    img.addEventListener("load", measure)
    window.addEventListener("scroll", onScroll, { passive: true })
    window.addEventListener("resize", measure)
    return () => {
      img.removeEventListener("load", measure)
      window.removeEventListener("scroll", onScroll)
      window.removeEventListener("resize", measure)
      if (frame) cancelAnimationFrame(frame)
    }
  }, [])

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        {/* 水墨画布背景层：固定全屏于内容之下（z-index 0），路由页面在 .app-content 层（z-index 1）上 */
        }
        <div className="canvas-bg" aria-hidden>
          {/* 水墨图本体（真实 <img>，方便量测实际渲染高度） */ }
          <img className="canvas-bg-img" src="/bg.jpg" alt="" draggable={false} />
          {/* 深黑渐变遮罩：盖住图片但不随其位移，保证白字任意亮度可读 */ }
          <div className="canvas-bg-mask" />
        </div>
        <main className="app-content">
          {sessionToken ? (
            targetAccount ? (
              <Select
                key={targetAccount}
                account={targetAccount}
                sessionToken={sessionToken}
                onDone={() => setTargetAccount(null)}
              />
            ) : inAdmin || isCurrentAdminSession(sessions, adminName, adminToken) ? (
              <Admin
                account={current}
                sessionToken={sessionToken}
                onLogout={() => logout(sessionToken)}
                onSelectAccount={(acct) => setTargetAccount(acct)}
                onDeleted={onDeleted}
                onBackToStudent={() => {
                  // 切回学生端：改用其他已登录账号，否则退出 admin
                  const others = accounts.filter((a) => a !== adminName)
                  setInAdmin(false)
                  // 管理令牌标记一并清除——退出管理态后标记失去意义（下次管理登录
                  // 重新标记；不清则 isCurrentAdminSession 仍命中，Admin 继续显示）
                  setAdminToken("")
                  saveAdminToken("")
                  // 无其他学生账号时回登录页（完整登出语义）：
                  // 否则 current 仍是 adminName，渲染 Admin 分支恒真，D4 的"回登录页"从未
                  // 生效。但"仅 setCurrent("")"会被 account-reselect effect 立即弹回
                  // accounts[0]（仍是 admin），Admin 继续显示。改为完整登出语义：无学生
                  // 账号 = 管理员退出全部会话，清空 sessions + current → 渲染落到登录页
                  // 且 effect 不再弹回；有学生账号则直接切到它（setInAdmin 已 false）。
                  if (others.length > 0) {
                    setCurrent(others[0])
                  } else {
                    const next: Sessions = {}
                    saveSessions(next)
                    setSessions(next)
                    setCurrent("")
                    setTargetAccount(null)
                  }
                }}
              />
            ) : page === "dashboard" ? (
              <Dashboard
                account={current}
                sessionToken={sessionToken}
                onLogout={() => logout(sessionToken)}
                onGoSelect={() => setPage("select")}
              />
            ) : (
              <Select
                key={current}
                account={current}
                sessionToken={sessionToken}
                onDone={() => setPage("dashboard")}
              />
            )
          ) : (
            <Login onLogin={login} />
          )}
        </main>
      </ToastProvider>
    </QueryClientProvider>
  )
}
