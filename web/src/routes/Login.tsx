import { useState } from "react"
import { api } from "../api/client"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { ShieldCheck, ArrowRight, Loader2, User, Lock, Eye, EyeOff, Sparkles } from "lucide-react"

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
    <div className="min-h-screen flex items-center justify-center p-4 sm:p-6 bg-[var(--bg)] select-none">
      {/* 居中温润现代卡片 */}
      <div className="w-full max-w-md relative">
        {/* 背景柔和微光环境氛围 */}
        <div className="absolute -top-12 -left-12 w-48 h-48 bg-[var(--cyan)]/10 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute -bottom-12 -right-12 w-48 h-48 bg-[var(--emerald)]/10 rounded-full blur-3xl pointer-events-none" />

        <Card className="rounded-[var(--radius-xl)] border border-[var(--border-hover)] bg-[var(--surface)] relative overflow-hidden shadow-2xl">
          {/* 顶栏规格条 */}
          <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--border)] bg-[var(--surface-soft)]">
            <div className="flex items-center gap-2">
              <Sparkles className="h-4 w-4 text-[var(--cyan)]" />
              <span className="text-xs font-medium text-[var(--fg)]">
                至道选课服务
              </span>
            </div>
            <Badge variant="success" className="text-[11px] px-2 py-0.5">
              RSA-1024 密钥安全
            </Badge>
          </div>

          <CardHeader className="space-y-2 p-6 pb-2">
            <div className="flex items-center gap-2">
              <Badge variant="primary" className="text-[10px]">ZHIDAO</Badge>
              <span className="text-xs text-[var(--fg-dim)]">福建省教育信息化平台</span>
            </div>
            <CardTitle className="text-2xl font-bold tracking-tight text-[var(--fg)] pt-1">
              欢迎登录选课助手
            </CardTitle>
            <CardDescription className="text-xs sm:text-sm text-[var(--fg-muted)] leading-relaxed">
              毫秒级定时并发抢选 · 智能 Vision 验证码识别 · 会话安全持久化
            </CardDescription>
          </CardHeader>

          <CardContent className="p-6 pt-4">
            <form
              className="flex flex-col gap-4"
              onSubmit={(e) => {
                e.preventDefault()
                submit()
              }}
            >
              {/* 账号输入框 */}
              <div className="flex flex-col gap-1.5">
                <label className="text-xs font-medium text-[var(--fg-muted)] flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <User className="h-3.5 w-3.5 text-[var(--cyan)]" />
                    <span>账号 / 学号</span>
                  </span>
                  <span className="text-[11px] text-[var(--fg-dim)]">手机号或学籍号</span>
                </label>
                <Input
                  placeholder="请输入您的学号或教务账号"
                  value={account}
                  onChange={(e) => setAccount(e.target.value)}
                  autoComplete="username"
                  disabled={loading}
                  className="h-11 text-sm bg-[var(--surface-soft)] border-[var(--border)] focus:border-[var(--cyan)]"
                />
              </div>

              {/* 密码输入框 */}
              <div className="flex flex-col gap-1.5">
                <label className="text-xs font-medium text-[var(--fg-muted)] flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <Lock className="h-3.5 w-3.5 text-[var(--cyan)]" />
                    <span>登录密码</span>
                  </span>
                  <span className="text-[11px] text-[var(--fg-dim)]">教务平台密码</span>
                </label>
                <div className="relative">
                  <Input
                    type={showPassword ? "text" : "password"}
                    placeholder="请输入您的教务登录密码"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    disabled={loading}
                    className="h-11 pr-10 text-sm bg-[var(--surface-soft)] border-[var(--border)] focus:border-[var(--cyan)]"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--fg-dim)] hover:text-[var(--fg)] p-1 rounded-md transition-colors"
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              {/* 人性化错误提示框 */}
              {error && (
                <div className="p-3 rounded-[var(--radius-md)] border border-[var(--rose-border)] bg-[var(--rose-bg)] text-xs text-[var(--rose)] flex items-center gap-2 animate-in fade-in-50 duration-200">
                  <span className="inline-block w-2 h-2 rounded-full bg-[var(--rose)] shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              {/* 提交主按钮 */}
              <Button
                type="submit"
                variant="primary"
                size="lg"
                disabled={loading}
                className="w-full mt-2 h-11 sm:h-12 flex items-center justify-center gap-2 text-sm font-semibold rounded-[var(--radius-md)] shadow-md"
              >
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>正在连接教务平台并识别验证码...</span>
                  </>
                ) : (
                  <>
                    <span>一键登录并进入控制台</span>
                    <ArrowRight className="h-4 w-4" />
                  </>
                )}
              </Button>
            </form>

            {/* 底部保障提示 */}
            <div className="mt-6 pt-4 border-t border-[var(--border)] flex items-center justify-between text-xs text-[var(--fg-dim)]">
              <div className="flex items-center gap-1.5">
                <ShieldCheck className="h-4 w-4 text-[var(--emerald)]" />
                <span>凭据本地加密，不上传第三方</span>
              </div>
              <span className="text-[11px] text-[var(--fg-muted)]">双端自适应 v2.0</span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
