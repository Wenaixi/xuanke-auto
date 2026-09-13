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
  // 管理员账号名：登录返回 adminName 时同步（后端 XUANKE_ADMIN_NAME 决定，默认 admin）
  const [adminName, setAdminName] = useState("admin")

  const accounts = Object.keys(sessions)
  const sessionToken = current ? sessions[current] : undefined

  // 登录成功：写入（或覆盖）该账号会话，登录即自动加入账号列表
  const login = (token: string, account: Account, adminName?: string) => {
    if (adminName) setAdminName(adminName)
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
    // 当前账号已不是管理员时退出管理态
    if (current !== adminName) setInAdmin(false)
  }, [accounts, current, adminName])

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
            inAdmin || current === adminName ? (
              <Admin
                account={current}
                sessionToken={sessionToken}
                onLogout={logout}
                onBackToStudent={() => {
                  // 切回学生端：改用其他已登录账号，否则退出 admin
                  const others = accounts.filter((a) => a !== adminName)
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
