import { useState } from "react"
import { api } from "../api/client"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { ArrowRight, Loader2, User, Lock, Eye, EyeOff } from "lucide-react"

interface Props {
  onLogin: (token: string) => void
}

export default function Login({ onLogin }: Props) {
  const [account, setAccount] = useState("")
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  const submit = async () => {
    if (!account.trim() || !password) {
      setError("请完整输入您的账号与密码喵~")
      return
    }
    setLoading(true)
    setError("")
    try {
      const data = await api<{ token: string }>("/login", {
        method: "POST",
        body: JSON.stringify({ account: account.trim(), password }),
      })
      onLogin(data.token)
    } catch (e: any) {
      setError(e.message || "登录认证失败，请检查账号密码是否正确")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 sm:p-6 bg-black text-white select-none">
      {/* 纯黑白极简艺术卡片容器 */}
      <div className="w-full max-w-sm">
        <Card className="rounded-[var(--radius-lg)] border border-neutral-800 bg-[#09090b] shadow-2xl overflow-hidden">
          <CardHeader className="space-y-1.5 p-6 pb-4">
            <div className="flex items-center justify-between text-xs tracking-wider uppercase text-neutral-500 font-mono">
              <span>ZHIDAO EDU</span>
              <span>SYSTEM</span>
            </div>
            <CardTitle className="text-xl font-medium tracking-tight text-white pt-2">
              账户登录
            </CardTitle>
            <CardDescription className="text-xs text-neutral-400">
              请输入教务平台账号与密码以同步选课数据
            </CardDescription>
          </CardHeader>

          <CardContent className="p-6 pt-2">
            <form
              className="flex flex-col gap-4"
              onSubmit={(e) => {
                e.preventDefault()
                submit()
              }}
            >
              {/* 账号输入框 */}
              <div className="flex flex-col gap-1.5">
                <label className="text-xs text-neutral-400 flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <User className="h-3.5 w-3.5 text-neutral-500" />
                    <span>账号 / 学号</span>
                  </span>
                </label>
                <Input
                  placeholder="输入教务学号或账号"
                  value={account}
                  onChange={(e) => setAccount(e.target.value)}
                  autoComplete="username"
                  disabled={loading}
                  className="h-10 text-sm bg-neutral-950 border-neutral-800 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
                />
              </div>

              {/* 密码输入框 */}
              <div className="flex flex-col gap-1.5">
                <label className="text-xs text-neutral-400 flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <Lock className="h-3.5 w-3.5 text-neutral-500" />
                    <span>登录密码</span>
                  </span>
                </label>
                <div className="relative">
                  <Input
                    type={showPassword ? "text" : "password"}
                    placeholder="输入教务登录密码"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    disabled={loading}
                    className="h-10 pr-10 text-sm bg-neutral-950 border-neutral-800 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-neutral-500 hover:text-white p-1 transition-colors"
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              {/* 错误提示框 */}
              {error && (
                <div className="p-3 rounded-[var(--radius-sm)] border border-neutral-800 bg-neutral-950 text-xs text-neutral-300 flex items-center gap-2">
                  <span className="inline-block w-1.5 h-1.5 rounded-full bg-white shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              {/* 提交主按钮：纯白反转底高反差 */}
              <Button
                type="submit"
                variant="primary"
                size="lg"
                disabled={loading}
                className="w-full mt-2 h-10 flex items-center justify-center gap-2 text-sm font-semibold rounded-[var(--radius-sm)]"
              >
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin text-black" />
                    <span>正在连接教务认证...</span>
                  </>
                ) : (
                  <>
                    <span>登录系统</span>
                    <ArrowRight className="h-4 w-4 text-black" />
                  </>
                )}
              </Button>
            </form>

            {/* 底部信息：极简纯粹 */}
            <div className="mt-6 pt-4 border-t border-neutral-900 flex items-center justify-between text-[11px] text-neutral-500">
              <span>本地安全隔离</span>
              <span>单机运行</span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
