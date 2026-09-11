import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import Login from "./routes/Login"
import Dashboard from "./routes/Dashboard"
import Select from "./routes/Select"
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

  const accounts = Object.keys(sessions)
  const sessionToken = current ? sessions[current] : undefined

  // 登录成功：写入（或覆盖）该账号会话，登录即自动加入账号列表
  const login = (token: string, account: Account) => {
    const next = { ...loadSessions(), [account]: token }
    localStorage.setItem("xk_sessions", JSON.stringify(next))
    setSessions(next)
    setCurrent(account)
  }

  // 退出当前账号：仅移除该账号会话，其他账号保留
  const logout = () => {
    const next = { ...loadSessions() }
    delete next[current]
    localStorage.setItem("xk_sessions", JSON.stringify(next))
    setSessions(next)
  }

  // 当前账号的会话被剔除后自动切到剩余账号（无账号则回登录页）
  useEffect(() => {
    if (accounts.length === 0) {
      setCurrent("")
    } else if (!current || !accounts.includes(current)) {
      setCurrent(accounts[0])
    }
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

  const switchAccount = (acct: Account) => setCurrent(acct)

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        {sessionToken ? (
          page === "dashboard" ? (
            <Dashboard
              account={current}
              sessionToken={sessionToken}
              accounts={accounts}
              onSwitchAccount={switchAccount}
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
      </ToastProvider>
    </QueryClientProvider>
  )
}
