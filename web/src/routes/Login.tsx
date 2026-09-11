import { useState } from "react"
import { api } from "../api/client"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { ShieldCheck, ArrowRight, Loader2, Terminal } from "lucide-react"

interface Props {
  onLogin: (token: string) => void
}

export default function Login({ onLogin }: Props) {
  const [account, setAccount] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  const submit = async () => {
    if (!account || !password) {
      setError("请输入账号与密码")
      return
    }
    setLoading(true)
    setError("")
    try {
      const data = await api<{ token: string }>("/login", {
        method: "POST",
        body: JSON.stringify({ account, password }),
      })
      onLogin(data.token)
    } catch (e: any) {
      setError(e.message || "登录认证失败，请检查账号密码")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-[var(--bg)] select-none">
      {/* 居中画廊海报卡片 */}
      <div className="w-full max-w-md relative">
        {/* 背景发丝十字装饰 */}
        <div className="absolute -top-3 -left-3 text-[10px] font-mono text-[var(--fg-dim)] tracking-widest">+</div>
        <div className="absolute -top-3 -right-3 text-[10px] font-mono text-[var(--fg-dim)] tracking-widest">+</div>
        <div className="absolute -bottom-3 -left-3 text-[10px] font-mono text-[var(--fg-dim)] tracking-widest">+</div>
        <div className="absolute -bottom-3 -right-3 text-[10px] font-mono text-[var(--fg-dim)] tracking-widest">+</div>

        <Card className="border border-[var(--border)] bg-[var(--surface)] relative overflow-hidden">
          {/* 顶栏规格条 */}
          <div className="flex items-center justify-between px-5 py-2.5 border-b border-[var(--border)] bg-[var(--surface-soft)]">
            <div className="flex items-center gap-2">
              <Terminal className="h-3.5 w-3.5 text-[var(--fg-dim)]" />
              <span className="text-[10px] font-mono tracking-widest uppercase text-[var(--fg-dim)]">
                AUTHENTICATION // SYSTEM GATE
              </span>
            </div>
            <Badge variant="outline" className="text-[9px] px-1.5 py-0">
              RSA-1024
            </Badge>
          </div>

          <CardHeader className="space-y-2 p-6 pb-4">
            <div className="flex items-center gap-2">
              <Badge variant="default" className="text-[9px]">V2.0</Badge>
              <span className="text-[10px] font-mono text-[var(--fg-dim)] tracking-widest">ZHIDAO.EDU</span>
            </div>
            <CardTitle className="text-xl tracking-[0.2em] font-normal pt-1">
              至道选课自动化
            </CardTitle>
            <CardDescription className="text-xs text-[var(--fg-dim)] leading-relaxed">
              单二进制并发抢课调度引擎 · 智能验证码识别与会话持久化
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
                <label className="text-[10px] font-mono uppercase tracking-widest text-[var(--fg-dim)] flex justify-between">
                  <span>ACCOUNT // 账号</span>
                  <span>STD_ID</span>
                </label>
                <Input
                  placeholder="请输入学号或账号"
                  value={account}
                  onChange={(e) => setAccount(e.target.value)}
                  autoComplete="username"
                  disabled={loading}
                  className="font-mono text-xs"
                />
              </div>

              {/* 密码输入框 */}
              <div className="flex flex-col gap-1.5">
                <label className="text-[10px] font-mono uppercase tracking-widest text-[var(--fg-dim)] flex justify-between">
                  <span>PASSWORD // 密码</span>
                  <span>ENCRYPTED</span>
                </label>
                <Input
                  type="password"
                  placeholder="请输入登录密码"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  disabled={loading}
                  className="font-mono text-xs"
                />
              </div>

              {/* 错误提示框 */}
              {error && (
                <div className="p-3 border border-white text-xs text-[var(--fg)] bg-black font-mono tracking-wider flex items-center gap-2">
                  <span className="inline-block w-1.5 h-1.5 bg-white" />
                  <span>{error}</span>
                </div>
              )}

              {/* 提交按钮 */}
              <Button
                type="submit"
                variant="invert"
                size="lg"
                disabled={loading}
                className="w-full mt-2 flex items-center justify-center gap-2 text-xs"
              >
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>正在鉴权与视觉验证码识别...</span>
                  </>
                ) : (
                  <>
                    <span>进入调度控制台</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </>
                )}
              </Button>
            </form>

            {/* 底部保障提示 */}
            <div className="mt-6 pt-4 border-t border-[var(--border)] flex items-center justify-between text-[10px] font-mono text-[var(--fg-dim)]">
              <div className="flex items-center gap-1.5">
                <ShieldCheck className="h-3.5 w-3.5" />
                <span>会话自动保存于本地存储</span>
              </div>
              <span className="tracking-widest">ZERO-RADIUS</span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
