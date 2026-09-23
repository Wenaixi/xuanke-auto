import { useState } from "react"
import { api } from "../api/client"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/Card"
import { ArrowRight, Loader2, User, Lock, KeyRound, Eye, EyeOff, ShieldCheck } from "lucide-react"

interface Props {
  onLogin: (token: string, account: string, adminName?: string) => void
}

export default function Login({ onLogin }: Props) {
  const [account, setAccount] = useState("")
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  // 激活码模态框状态：登录返回 code=1001 时弹出
  const [pendingAccount, setPendingAccount] = useState("")
  // 票据必须随激活请求回传——后端 handleActivate 要求 ticket 非空
  // 且 ConsumeTicket 校验绑定账号；此前 1001 分支只存账号把 data.ticket 丢弃，激活永远
  // 返回"激活票据无效或已过期"，激活码机制整链不可用。取消激活时一并清掉。
  const [pendingTicket, setPendingTicket] = useState("")
  const [activationCode, setActivationCode] = useState("")
  const [activating, setActivating] = useState(false)
  // 激活错误独立状态：激活失败不清污染主登录表单的 error——
  // 取消/成功返回登录后，主表单错误条保持干净
  const [activateError, setActivateError] = useState("")

  const submit = async () => {
    // 提交入口先查进行中标记——按钮 disabled 靠 React 状态渲染
    // 落地有延迟，连按两次 Enter/快速双击可在 `disabled` 生效前发出两个重复登录请求：
    // 教务多份并发登录会互相挤掉会话（旧 token 失效），且验证码识别并发放大平台限流
    // 压力。loading 本身就是"上一发在飞"的可靠判据，入口幂等短路。
    if (loading) return
    if (!account.trim() || !password) {
      setError("请完整输入教学账号与登录密码喵~")
      return
    }
    setLoading(true)
    setError("")
    try {
      const data = await api<{ token: string; account: string; adminName?: string; ticket?: string }>("/login", {
        method: "POST",
        body: JSON.stringify({ account: account.trim(), password }),
      })
      // 管理员登录响应带 adminName；普通学生（含撞名学生）登录没有 → 传入可选的额外参数
      onLogin(data.token, data.account, data.adminName)
    } catch (e: any) {
      if (e.code === 1001) {
        // 账号未激活：弹出激活码输入模态框（同时保存票据供激活回传）
        setPendingAccount(account.trim())
        setPendingTicket((e.data?.ticket as string) || "")
        setActivationCode("")
      } else {
        setError(e.message || "登录认证失败，请检查账号密码是否正确")
      }
    } finally {
      setLoading(false)
    }
  }

  const activate = async () => {
    // 激活入口补幂等守卫——与 submit() 的 `if (loading) return`
    // 对称：激活按钮 disabled 依赖 React 渲染落地有延迟，连按两次可在 disabled 生效前
    // 发出两个重复激活请求（后到的响应处理 onLogin 会把会话挤成旧值）。
    if (activating) return
    if (!activationCode.trim()) {
      setActivateError("请输入激活码喵~")
      return
    }
    setActivating(true)
    setActivateError("")
    try {
      const data = await api<{ token: string; account: string }>("/activate", {
        method: "POST",
        body: JSON.stringify({ account: pendingAccount, code: activationCode.trim(), ticket: pendingTicket }),
      })
      setPendingAccount("")
      setPendingTicket("")
      onLogin(data.token, data.account)
    } catch (e: any) {
      // 激活失败分两个场景引导——后端票据 5 分钟单次，
      // 过期/已用会报"激活票据无效或已过期"；此时账号其实已激活，重新登录即可
      // 直进，用户可能误以为激活码有问题而反复点激活（恒失败）。
      if (/激活票据无效或已过期/.test(e.message || "")) {
        setPendingTicket("")
        setActivateError("激活票据已过期或已使用，请取消后重新登录即可进入（本账号已开通）")
        setActivationCode("")
      } else {
        // 激活码错误等非「过期/已用」失败：服务端先 ConsumeTicket 再校验激活码，失败
        // 也销毁票据；滞留旧票重试只会拿到"激活票据无效"的误导文案。与过期/已用分支
        // 同款清空，下次登录再遇 1001 由服务端下发新票覆盖，UI 永不显示已消费票据。
        setPendingTicket("")
        setActivateError(e.message || "激活失败，请检查激活码是否正确")
      }
    } finally {
      setActivating(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 sm:p-6 text-white select-none">
      {/* 纯黑白极简艺术卡片容器（磨砂玻璃透出水墨背景） */}
      <div className="w-full max-w-sm">
        <Card className="rounded-[var(--radius-lg)] glass-strong border border-neutral-700 shadow-2xl overflow-hidden">
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
              {/* 账号输入框（label 经 htmlFor 关联输入框，点击标签即聚焦） */}
              <div className="flex flex-col gap-1.5">
                <label htmlFor="login-account" className="text-xs text-neutral-400 flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <User className="h-3.5 w-3.5 text-neutral-500" />
                    <span>账号 / 学号</span>
                  </span>
                </label>
                <Input
                  id="login-account"
                  placeholder="输入教务学号或账号"
                  value={account}
                  onChange={(e) => setAccount(e.target.value)}
                  autoComplete="username"
                  disabled={loading}
                  className="h-10 text-sm glass-input border-neutral-700 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
                />
              </div>

              {/* 密码输入框 */}
              <div className="flex flex-col gap-1.5">
                <label htmlFor="login-password" className="text-xs text-neutral-400 flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <Lock className="h-3.5 w-3.5 text-neutral-500" />
                    <span>登录密码</span>
                  </span>
                </label>
                <div className="relative">
                  <Input
                    id="login-password"
                    type={showPassword ? "text" : "password"}
                    placeholder="输入教务登录密码"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    disabled={loading}
                    className="h-10 pr-10 text-sm glass-input border-neutral-700 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    // 可见性切换按钮语义化——aria-label 说明作用、aria-pressed 报状态
                    aria-label={showPassword ? "隐藏密码" : "显示密码"}
                    aria-pressed={showPassword}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-neutral-500 hover:text-white p-1 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60 rounded"
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              {/* 错误提示框 */}
              {error && (
                <div role="alert" className="p-3 rounded-[var(--radius-sm)] glass border border-neutral-700 text-xs text-neutral-300 flex items-center gap-2">
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
            <div className="mt-6 pt-4 border-t border-neutral-800 flex items-center justify-between text-[11px] text-neutral-500">
              <span>账号隔离</span>
              <span>实时调度</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* 激活码输入模态框（登录返回 1001 时弹出） */}
      {/* 裸 div 补无障碍语义——role=dialog/aria-modal/aria-labelledby/
          Esc 关闭回归键盘可达性（此前背景表单可 Tab 穿出、读屏不识别对话语义）。
          完整焦点陷阱迁移到 Radix Dialog 属后续候选，这里先补最小语义门。 */}
      {pendingAccount && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 glass-overlay"
          role="dialog"
          aria-modal="true"
          aria-labelledby="activate-dialog-title"
          // 补 Esc 关闭——注释承诺"Esc 关闭回归键盘可达性"但
          // 实现从未落地（裸 div 无 keydown 处理），键盘用户只能 Tab 到"取消"按钮；
          // 与取消按钮同逻辑（清待激活账号/票据/错误），激活中不响应防误关。
          onKeyDown={(e) => {
            if (e.key === "Escape" && !activating) {
              setPendingAccount("")
              setPendingTicket("")
              setActivateError("")
            }
          }}
        >
          <div className="w-full max-w-sm rounded-[var(--radius-lg)] glass-strong border border-neutral-700 shadow-2xl">
            <div className="p-6 pb-4">
              <div className="flex items-center gap-2 text-xs tracking-wider uppercase text-neutral-500 font-mono">
                <KeyRound className="h-3.5 w-3.5 text-neutral-500" />
                <span>ACTIVATE ACCOUNT</span>
              </div>
              <h2 id="activate-dialog-title" className="text-lg font-medium tracking-tight text-white pt-2">
                输入激活码
              </h2>
              <p className="text-xs text-neutral-400 pt-1">
                账号 {pendingAccount} 尚未激活，请输入管理员发放的激活码以开通使用权限
              </p>
            </div>

            <div className="p-6 pt-2">
              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <label htmlFor="activation-code" className="text-xs text-neutral-400 flex items-center gap-1.5">
                    <ShieldCheck className="h-3.5 w-3.5 text-neutral-500" />
                    <span>激活码</span>
                  </label>
                  <Input
                    id="activation-code"
                    value={activationCode}
                    onChange={(e) => setActivationCode(e.target.value)}
                    placeholder="XK-XXXX-XXXX-XXXX"
                    disabled={activating}
                    autoFocus
                    className="h-10 text-sm font-mono tracking-widest glass-input border-neutral-700 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
                  />
                </div>

                {activateError && (
                  <div role="alert" className="p-3 rounded-[var(--radius-sm)] glass border border-neutral-700 text-xs text-neutral-300 flex items-center gap-2">
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-white shrink-0" />
                    <span>{activateError}</span>
                  </div>
                )}

                <Button
                  variant="primary"
                  size="lg"
                  disabled={activating}
                  onClick={activate}
                  className="w-full h-10 flex items-center justify-center gap-2 text-sm font-semibold rounded-[var(--radius-sm)]"
                >
                  {activating ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin text-black" />
                      <span>正在激活...</span>
                    </>
                  ) : (
                    <>
                      <span>激活并登录</span>
                      <ArrowRight className="h-4 w-4 text-black" />
                    </>
                  )}
                </Button>

                <button
                  type="button"
                  onClick={() => {
                    // 取消激活——清除待激活账号/票据与激活错误，主表单错误保持干净
                    setPendingAccount("")
                    setPendingTicket("")
                    setActivateError("")
                  }}
                  disabled={activating}
                  className="text-xs text-neutral-500 hover:text-white transition-colors mx-auto py-1"
                >
                  取消，返回登录
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
