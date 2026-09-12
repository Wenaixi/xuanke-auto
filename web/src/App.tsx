import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import Login from "./routes/Login"
import Dashboard from "./routes/Dashboard"
import Select from "./routes/Select"
import Admin from "./routes/Admin"
import { ToastProvider } from "./components/ui/Toast"
import { UNAUTHORIZED_EVENT } from "./api/client"
import type { Account, Sessions } from "./types"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 0 } },
})

// 本地会话映射存取：账号名 -> 服务端签发令牌（多账号互不干扰）
function loadSessions(): Sessions {
  try {
    return JSON.parse(localStorage.getItem("xk_sessions") || "{}")
  } catch {
    return {}
  }
}

export default function App() {
  const [sessions, setSessions] = useState<Sessions>(loadSessions)
  const [page, setPage] = useState<"dashboard" | "select">("dashboard")
  const [current, setCurrent] = useState<Account>("")
  const [inAdmin, setInAdmin] = useState(false)
  // 管理员账号名（后端下发的当前管理员名，默认 admin）
  const [adminName, setAdminName] = useState("admin")

  const accounts = Object.keys(sessions)
  const sessionToken = current ? sessions[current] : undefined

  // 登录成功：写入（或覆盖）该账号会话，登录即自动加入账号列表
  const login = (token: string, account: Account) => {
    const next = { ...loadSessions(), [account]: token }
    localStorage.setItem("xk_sessions", JSON.stringify(next))
    setSessions(next)
    setCurrent(account)
    // 管理员账号登录后直接进入管理员界面（账号名与后端管理员名一致即管理员）
    setInAdmin(account === adminName)
  }

  // 退出当前账号：仅移除该账号会话，其他账号保留
  const logout = () => {
    const next = { ...loadSessions() }
    delete next[current]
    localStorage.setItem("xk_sessions", JSON.stringify(next))
    setSessions(next)
    setInAdmin(false)
  }

  // 当前账号的会话被剔除后自动切到剩余账号（无账号则回登录页）
  useEffect(() => {
    if (accounts.length === 0) {
      setCurrent("")
    } else if (!current || !accounts.includes(current)) {
      setCurrent(accounts[0])
    }
    // 当前账号已不是 admin 时退出管理态
    if (current !== "admin") setInAdmin(false)
  }, [accounts, current])

  // 后端返回 401（会话过期）：剔除当前账号的失效令牌
  useEffect(() => {
    const onUnauthorized = () => {
      setSessions((prev) => {
        if (!current || !prev[current]) return prev
        const next = { ...prev }
        delete next[current]
        localStorage.setItem("xk_sessions", JSON.stringify(next))
        return next
      })
    }
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [current])

  // 桌面版壁纸“边滚边露底、触底冻结”：滚动时先一格格露出水墨图下方，
  // 露到图片底边即停——再往下滚整张图冻住不动（用户明确要求）。
  // 实现：把位移写进 canvas-bg 上的 --bg-shift 变量（CSS 桌面媒体查询消费），
  // 移动端不消费该变量，fixed 贴顶钉住视口、纹丝不动，逻辑只在桌面生效。
  useEffect(() => {
    const el = document.querySelector<HTMLElement>(".canvas-bg")
    if (!el) return
    let frame = 0
    const apply = () => {
      const vh = window.innerHeight
      const imgH = vh * 1.6 /* 桌面端 160% 放大后图片高度 = 视口高 * 1.6 */
      const maxShift = imgH - vh /* 最多能露出的图片高度（160% 多出的 60%） */
      const ratio = Math.min(1, window.scrollY / maxShift) /* 滚动进度封顶 1 */
      el.style.setProperty("--bg-shift", `${ratio * maxShift}px`)
      frame = 0
    }
    const onScroll = () => {
      if (!frame) frame = requestAnimationFrame(apply)
    }
    apply()
    window.addEventListener("scroll", onScroll, { passive: true })
    window.addEventListener("resize", onScroll)
    return () => {
      window.removeEventListener("scroll", onScroll)
      window.removeEventListener("resize", onScroll)
      if (frame) cancelAnimationFrame(frame)
    }
  }, [])

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        {/* 水墨画布背景层：固定全屏于内容之下（z-index 0），路由页面在 .app-content 层（z-index 1）上 */
        }
        <div className="canvas-bg" aria-hidden />
        <main className="app-content">
          {sessionToken ? (
            inAdmin || current === "admin" ? (
              <Admin
                account={current}
                sessionToken={sessionToken}
                onLogout={logout}
                onBackToStudent={() => {
                  // 切回学生端：改用其他已登录账号，否则退出 admin
                  const others = accounts.filter((a) => a !== "admin")
                  setInAdmin(false)
                  if (others.length > 0) setCurrent(others[0])
                }}
              />
            ) : page === "dashboard" ? (
              <Dashboard
                account={current}
                sessionToken={sessionToken}
                onLogout={logout}
                onGoSelect={() => setPage("select")}
              />
            ) : (
              <Select
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
