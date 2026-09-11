import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query"
import Login from "./routes/Login"
import Dashboard from "./routes/Dashboard"
import Select from "./routes/Select"
import { ToastProvider } from "./components/ui/Toast"
import { UNAUTHORIZED_EVENT, api } from "./api/client"
import type { Account } from "./types"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 0 } },
})

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem("xk_token"))
  const [page, setPage] = useState<"dashboard" | "select">("dashboard")
  const [current, setCurrent] = useState<Account>("")

  // 已登录账号列表（登录即自动加入；拉取失败保持空数组）
  const { data: accounts = [] as Account[] } = useQuery({
    queryKey: ["accounts"],
    queryFn: () => api<Account[]>("/accounts"),
    staleTime: 0,
  })

  // 后端返回未登录（token 失效）时全局登出
  useEffect(() => {
    const onUnauthorized = () => setToken(null)
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [])

  // 默认选中第一个账号
  useEffect(() => {
    if (!current && accounts.length > 0) {
      setCurrent(accounts[0])
    }
  }, [accounts, current])

  const login = (t: string, account: Account) => {
    localStorage.setItem("xk_token", t)
    setToken(t)
    setCurrent(account)
    queryClient.invalidateQueries({ queryKey: ["accounts"] })
  }
  const logout = () => {
    localStorage.removeItem("xk_token")
    setToken(null)
  }
  const switchAccount = (acct: Account) => setCurrent(acct)

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        {token ? (
          page === "dashboard" ? (
            <Dashboard
              account={current}
              accounts={accounts}
              onSwitchAccount={switchAccount}
              onLogout={logout}
              onGoSelect={() => setPage("select")}
            />
          ) : (
            <Select account={current} onDone={() => setPage("dashboard")} />
          )
        ) : (
          <Login onLogin={login} />
        )}
      </ToastProvider>
    </QueryClientProvider>
  )
}