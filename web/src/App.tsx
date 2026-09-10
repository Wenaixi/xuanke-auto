import { useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Login from './routes/Login'
import Dashboard from './routes/Dashboard'
import Select from './routes/Select'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 0 } },
})

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('xk_token'))
  const [page, setPage] = useState<'dashboard' | 'select'>('dashboard')

  const login = (t: string) => {
    localStorage.setItem('xk_token', t)
    setToken(t)
  }
  const logout = () => {
    localStorage.removeItem('xk_token')
    setToken(null)
  }

  return (
    <QueryClientProvider client={queryClient}>
      {token ? (
        page === 'dashboard' ? (
          <Dashboard onLogout={logout} onGoSelect={() => setPage('select')} />
        ) : (
          <Select onDone={() => setPage('dashboard')} />
        )
      ) : (
        <Login onLogin={login} />
      )}
    </QueryClientProvider>
  )
}
