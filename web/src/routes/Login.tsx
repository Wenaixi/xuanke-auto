import { useState } from 'react'
import { api } from '../api/client'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'

interface Props {
  onLogin: (token: string) => void
}

export default function Login({ onLogin }: Props) {
  const [account, setAccount] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async () => {
    if (!account || !password) {
      setError('请输入账号与密码')
      return
    }
    setLoading(true)
    setError('')
    try {
      const data = await api<{ token: string }>('/login', {
        method: 'POST',
        body: JSON.stringify({ account, password }),
      })
      onLogin(data.token)
    } catch (e: any) {
      setError(e.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form
      className="flex flex-col gap-4 w-80 mx-auto mt-24"
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
    >
      <h1 className="text-2xl tracking-[0.3em] mb-4">至道选课自动化</h1>
      <Input
        placeholder="账号"
        value={account}
        onChange={(e) => setAccount(e.target.value)}
        autoComplete="username"
      />
      <Input
        type="password"
        placeholder="密码"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete="current-password"
      />
      {error && <p className="text-[var(--danger)] text-sm">{error}</p>}
      <Button type="submit" disabled={loading}>
        {loading ? '登录中…' : '登录'}
      </Button>
    </form>
  )
}
