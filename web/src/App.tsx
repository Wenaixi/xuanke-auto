import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import Login from "./routes/Login"
import Dashboard from "./routes/Dashboard"
import Select from "./routes/Select"
import { UNAUTHORIZED_EVENT } from "./api/client"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 0 } },
})

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem("xk_token"))
  const [page, setPage] = useState<"dashboard" | "select">("dashboard")

  // 后端返回未登录（token 失效）时全局登出
  useEffect(() => {
    const onUnauthorized = () => setToken(null)
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [])

  const login = (t: string) => {
    localStorage.setItem("xk_token", t)
    setToken(t)
  }
  const logout = () => {
    localStorage.removeItem("xk_token")
    setToken(null)
  }

  return (
    <QueryClientProvider client={queryClient}>
      {token ? (
        page === "dashboard" ? (
          <Dashboard onLogout={logout} onGoSelect={() => setPage("select")} />
        ) : (
          <Select onDone={() => setPage("dashboard")} />
        )
      ) : (
        <Login onLogin={login} />
      )}
    </QueryClientProvider>
  )
}
