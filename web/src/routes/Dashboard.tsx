import { useEffect, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { Account, LogEntry, SchedulerState } from "../types"
import { Button } from "../components/ui/Button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import {
  Activity,
  ArrowUpRight,
  BookOpen,
  CheckCircle2,
  Clock,
  Layers,
  LogOut,
  RefreshCw,
  XCircle,
  HelpCircle,
} from "lucide-react"

interface Props {
  account: Account
  accounts: Account[]
  onSwitchAccount: (acct: Account) => void
  onLogout: () => void
  onGoSelect: () => void
}

interface ParsedCountdown {
  days: string
  hours: string
  minutes: string
  seconds: string
  isExpired: boolean
}

function parseCountdown(target: string | null): ParsedCountdown {
  if (!target) {
    return { days: "00", hours: "00", minutes: "00", seconds: "00", isExpired: true }
  }
  const t = new Date(target).getTime()
  const diff = t - Date.now()
  if (diff <= 0) {
    return { days: "00", hours: "00", minutes: "00", seconds: "00", isExpired: true }
  }
  const totalSeconds = Math.floor(diff / 1000)
  const d = Math.floor(totalSeconds / 86400)
  const h = Math.floor((totalSeconds % 86400) / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60

  const pad = (n: number) => n.toString().padStart(2, "0")
  return {
    days: pad(d),
    hours: pad(h),
    minutes: pad(m),
    seconds: pad(s),
    isExpired: false,
  }
}

export default function Dashboard({ account, accounts, onSwitchAccount, onLogout, onGoSelect }: Props) {
  const { data: state, isError: stateErr, isLoading: stateLoading } = useQuery({
    queryKey: ["state", account],
    queryFn: () => api<SchedulerState>("/state", { account }),
    refetchInterval: 3000,
  })

  const { data: logs } = useQuery({
    queryKey: ["logs"],
    queryFn: () => api<LogEntry[]>("/logs"),
    refetchInterval: 3000,
  })

  // 本地每秒刷新倒计时，确保数字秒级平滑跳动
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(timer)
  }, [])

  const courses = state?.courses ?? []
  const openTimeStr =
    state?.open_time && state.open_time !== "0001-01-01T00:00:00Z" ? state.open_time : null
  const cd = parseCountdown(openTimeStr)

  return (
    <div className="min-h-screen bg-black text-white p-4 sm:p-6 lg:p-8 select-none pb-24 sm:pb-8">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部纯黑白极简控制台顶栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-neutral-900 pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <h1 className="text-lg sm:text-xl font-medium tracking-tight text-white">
                选课自动化控制中心
              </h1>
              <Badge variant="outline" className="text-[10px] uppercase font-mono tracking-wider">
                CORE
              </Badge>
            </div>
            <p className="text-xs text-neutral-400">
              实时监听教务选课开放时间节点与席位状态
            </p>
          </div>

          <div className="flex items-center gap-2.5">
            {/* 账号切换下拉（纯黑白极简） */}
            {accounts.length > 0 && (
              <select
                value={account}
                onChange={(e) => onSwitchAccount(e.target.value)}
                className="h-8 px-2 bg-neutral-950 border border-neutral-800 text-xs text-white rounded-[var(--radius-sm)] focus:border-white transition-colors"
              >
                {accounts.map((a) => (
                  <option key={a} value={a} className="bg-neutral-950 text-white">
                    {a}
                  </option>
                ))}
              </select>
            )}
            <Button
              variant="primary"
              size="sm"
              onClick={onGoSelect}
              className="flex items-center gap-1.5 text-xs"
            >
              <BookOpen className="h-3.5 w-3.5 text-black" />
              <span>选课大厅</span>
              <ArrowUpRight className="h-3.5 w-3.5 text-black" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={onLogout}
              className="flex items-center gap-1.5 text-xs text-neutral-400 hover:text-white"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>退出</span>
            </Button>
          </div>
        </header>

        {/* 错误提示条 */}
        {!stateLoading && stateErr && (
          <div className="rounded-[var(--radius-sm)] border border-neutral-800 bg-neutral-950 p-4 text-xs flex items-center justify-between text-neutral-300">
            <div className="flex items-center gap-2">
              <XCircle className="h-4 w-4 shrink-0 text-white" />
              <span>当前登录凭据已失效，请重新进行账户认证</span>
            </div>
            <Button variant="outline" size="sm" onClick={onLogout}>
              重新登录
            </Button>
          </div>
        )}

        {/* 核心 Bento Grid 双翼展板 */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-5 sm:gap-6">
          {/* 左侧两大跨度：等宽秒级跳动巨幕倒计时 */}
          <Card className="lg:col-span-2 rounded-[var(--radius-lg)] border border-neutral-900 bg-[#09090b] flex flex-col justify-between shadow-none">
            <CardHeader className="flex flex-row items-center justify-between pb-2 border-b border-neutral-900">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-neutral-400" />
                <CardTitle className="text-sm font-medium tracking-wide text-white">
                  选课时间窗口
                </CardTitle>
              </div>
              <Badge variant={state?.window_opened ? "primary" : "outline"}>
                {state?.window_opened ? "窗口已开放" : "待命中"}
              </Badge>
            </CardHeader>

            <CardContent className="py-4">
              {state?.window_opened ? (
                <div className="py-6 flex flex-col items-center justify-center gap-3 rounded-[var(--radius-sm)] border border-neutral-800 bg-neutral-950 p-6 text-center">
                  <div className="flex items-center gap-2.5 text-white font-medium text-lg">
                    <span className="inline-block w-2 h-2 rounded-full bg-white animate-ping" />
                    <span>选课窗口现已开放</span>
                  </div>
                  <p className="text-xs text-neutral-400 max-w-md">
                    系统已进入自动报名阶段，正在持续同步您的预选目标课程。
                  </p>
                </div>
              ) : (
                <div className="flex flex-col gap-4">
                  {/* 四格等宽大字数字矩阵：纯黑白极简雕刻质感 */}
                  <div className="grid grid-cols-4 gap-2 sm:gap-4 text-center">
                    <div className="rounded-[var(--radius-sm)] border border-neutral-900 bg-neutral-950 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.days}
                      </span>
                      <span className="text-[11px] text-neutral-500 font-mono uppercase mt-1">
                        天
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-sm)] border border-neutral-900 bg-neutral-950 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.hours}
                      </span>
                      <span className="text-[11px] text-neutral-500 font-mono uppercase mt-1">
                        时
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-sm)] border border-neutral-900 bg-neutral-950 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.minutes}
                      </span>
                      <span className="text-[11px] text-neutral-500 font-mono uppercase mt-1">
                        分
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-sm)] border border-neutral-800 bg-neutral-900 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.seconds}
                      </span>
                      <span className="text-[11px] text-neutral-400 font-mono uppercase mt-1">
                        秒
                      </span>
                    </div>
                  </div>

                  {/* 极简纯粹时间提示 */}
                  <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between text-xs text-neutral-400 pt-3 border-t border-neutral-900 gap-2 font-mono">
                    <span className="flex items-center gap-1.5">
                      <span>预计开放时间：</span>
                      <span className="text-white">
                        {openTimeStr
                          ? new Date(openTimeStr).toLocaleString("zh-CN", { hour12: false })
                          : "正在同步教务平台时间配置..."}
                      </span>
                    </span>
                    <span className="text-neutral-500">
                      30 秒查询节流
                    </span>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* 右侧跨度：运行状态指标卡片 */}
          <Card className="rounded-[var(--radius-lg)] border border-neutral-900 bg-[#09090b] flex flex-col justify-between shadow-none">
            <CardHeader className="pb-2 border-b border-neutral-900">
              <div className="flex items-center gap-2">
                <Activity className="h-4 w-4 text-neutral-400" />
                <CardTitle className="text-sm font-medium tracking-wide text-white">
                  运行指标
                </CardTitle>
              </div>
              <CardDescription className="text-xs text-neutral-500">
                后台调度心跳与通信机制
              </CardDescription>
            </CardHeader>

            <CardContent className="space-y-3 text-xs pt-3">
              <div className="divide-y divide-neutral-900">
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-neutral-400">心跳轮询周期</span>
                  <span className="text-white font-mono tabular-nums">30 秒</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-neutral-400">预选目标课程</span>
                  <span className="text-white font-mono tabular-nums">{courses.length} / 3 门</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-neutral-400">认证通信链路</span>
                  <span className="text-white">双通道会话</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-neutral-400">频控保护策略</span>
                  <span className="text-white">单次熔断冷却</span>
                </div>
              </div>

              <div className="p-3 rounded-[var(--radius-sm)] bg-neutral-950 border border-neutral-900 flex items-center justify-between text-xs">
                <div className="flex items-center gap-2">
                  <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
                  <span className="text-white font-medium">后台就绪</span>
                </div>
                <span className="text-[11px] text-neutral-500 font-mono">STANDBY</span>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 预选目标矩阵（3 门重点看护课程卡片） */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-neutral-400" />
              <h2 className="text-sm font-medium text-white">
                预选目标课程
              </h2>
            </div>
            <span className="text-xs text-neutral-500 font-mono">
              3 COURSES MAX
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {courses.map((c) => {
              const isSuccess = c.status === "success"
              const isFailed = c.status === "failed"
              const isInRange = c.status === "in_range" || c.status === "submitted"

              return (
                <Card
                  key={c.class_id}
                  className={`rounded-[var(--radius-lg)] border transition-all duration-200 shadow-none ${
                    isSuccess
                      ? "border-white bg-[#111114]"
                      : "border-neutral-900 bg-[#09090b] hover:border-neutral-800"
                  }`}
                >
                  <CardHeader className="p-4 pb-2 flex flex-row items-center justify-between border-b border-neutral-900">
                    <Badge variant="outline" className="text-[10px] font-mono">
                      #{c.publish_id}
                    </Badge>
                    <Badge
                      variant={isSuccess ? "primary" : isFailed ? "destructive" : isInRange ? "primary" : "outline"}
                      className="text-[11px]"
                    >
                      {isSuccess
                        ? "已确认选课"
                        : isFailed
                        ? "报名异常"
                        : isInRange
                        ? "提交中"
                        : "待命"}
                    </Badge>
                  </CardHeader>

                  <CardContent className="p-4 pt-3 space-y-3">
                    <div>
                      <div className="text-[11px] text-neutral-500 font-mono">
                        ID: {c.class_id}
                      </div>
                      <h3 className="font-medium text-sm text-white tracking-tight line-clamp-1 mt-0.5">
                        {c.course_name || `选修课程 ${c.class_id}`}
                      </h3>
                    </div>

                    {/* 状态与进度提示 */}
                    <div className="text-xs pt-2 border-t border-neutral-900 flex items-center justify-between">
                      <div className="flex items-center gap-1.5">
                        {isSuccess && <CheckCircle2 className="h-3.5 w-3.5 text-white" />}
                        {isFailed && <XCircle className="h-3.5 w-3.5 text-neutral-400" />}
                        {isInRange && <RefreshCw className="h-3.5 w-3.5 text-white animate-spin" />}
                        {!isSuccess && !isFailed && !isInRange && (
                          <span className="inline-block w-1.5 h-1.5 rounded-full bg-neutral-600" />
                        )}
                        <span className="text-neutral-400">
                          {isSuccess
                            ? "席位已确认"
                            : isFailed
                            ? "提交未通过"
                            : isInRange
                            ? "冲刺提交中"
                            : "开放时自动提交"}
                        </span>
                      </div>
                    </div>

                    {c.result && (
                      <div className="p-2.5 rounded-[var(--radius-sm)] border border-neutral-900 bg-neutral-950 text-xs text-neutral-400 font-mono break-all leading-relaxed">
                        {c.result}
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}

            {courses.length === 0 && (
              <div className="col-span-full rounded-[var(--radius-lg)] border border-neutral-900 border-dashed bg-[#09090b] p-8 text-center flex flex-col items-center justify-center gap-3">
                <HelpCircle className="h-7 w-7 text-neutral-600" />
                <p className="text-xs text-neutral-400">
                  当前未添加任何预选课程
                </p>
                <Button variant="outline" size="sm" onClick={onGoSelect}>
                  前往挑选课程
                </Button>
              </div>
            )}
          </div>
        </section>

        {/* 调度活动动态流 */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-neutral-400" />
              <h2 className="text-sm font-medium text-white">
                调度日志
              </h2>
            </div>
            <div className="flex items-center gap-2 text-xs text-neutral-500 font-mono">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
              <span>LIVE</span>
            </div>
          </div>

          <Card className="rounded-[var(--radius-lg)] border border-neutral-900 bg-[#09090b] overflow-hidden shadow-none">
            <div className="px-4 py-2.5 border-b border-neutral-900 text-xs text-neutral-500 flex items-center justify-between font-mono">
              <span>EVENT STREAM</span>
              <span>RECENT 50</span>
            </div>

            <div className="p-4 max-h-64 overflow-y-auto text-xs space-y-2">
              {logs && logs.length > 0 ? (
                logs.map((l) => (
                  <div
                    key={l.id}
                    className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4 border-b border-neutral-900/60 pb-2 text-xs font-mono"
                  >
                    <span className="text-neutral-500 text-[11px] shrink-0">
                      {l.created_at}
                    </span>
                    <span className="text-white shrink-0 font-medium">
                      [{l.action}]
                    </span>
                    <span className="text-neutral-400 break-all">
                      {l.result}
                    </span>
                  </div>
                ))
              ) : (
                <div className="py-8 text-center text-neutral-600 text-xs font-mono">
                  NO RECENT LOGS
                </div>
              )}
            </div>
          </Card>
        </section>
      </div>

      {/* 手机移动端底部悬浮操作栏 (纯黑白极简艺术) */}
      <div className="sm:hidden fixed bottom-4 inset-x-4 z-40 bg-neutral-950/95 backdrop-blur-md border border-neutral-800 rounded-[var(--radius-lg)] p-3 flex items-center justify-between shadow-none">
        <div className="flex items-center gap-2">
          <span className="inline-block w-2 h-2 rounded-full bg-white" />
          <div className="flex flex-col">
            <span className="text-xs font-medium text-white">
              {state?.window_opened ? "窗口开放中" : "系统待命中"}
            </span>
            <span className="text-[10px] text-neutral-500 font-mono">
              {account ? `ACCOUNT ${account}` : "DEFAULT"} · TARGETS {courses.length}/3
            </span>
          </div>
        </div>
        <Button
          variant="primary"
          size="sm"
          onClick={onGoSelect}
          className="h-8 px-3 text-xs"
        >
          <BookOpen className="h-3.5 w-3.5 mr-1 text-black" />
          <span>选课大厅</span>
        </Button>
      </div>
    </div>
  )
}
