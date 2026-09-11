import { useEffect, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { LogEntry, SchedulerState } from "../types"
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
  Sparkles,
  Zap,
  XCircle,
  HelpCircle,
} from "lucide-react"

interface Props {
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

export default function Dashboard({ onLogout, onGoSelect }: Props) {
  const { data: state, isError: stateErr, isLoading: stateLoading } = useQuery({
    queryKey: ["state"],
    queryFn: () => api<SchedulerState>("/state"),
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
    <div className="min-h-screen bg-[var(--bg)] text-[var(--fg)] p-4 sm:p-6 lg:p-8 select-none pb-24 sm:pb-8">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部现代控制台顶栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[var(--border)] pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <Sparkles className="h-5 w-5 text-[var(--cyan)]" />
              <h1 className="text-lg sm:text-xl font-bold tracking-tight text-[var(--fg)]">
                至道选课自动化中心
              </h1>
              <Badge variant="primary" className="text-[10px]">
                并发调度引擎
              </Badge>
            </div>
            <p className="text-xs sm:text-sm text-[var(--fg-muted)] leading-relaxed">
              毫秒级定时并发抢报 · 零值守会话保护 · 双端自适应
            </p>
          </div>

          <div className="flex items-center gap-2.5">
            <Button
              variant="primary"
              size="sm"
              onClick={onGoSelect}
              className="flex items-center gap-1.5 text-xs font-semibold shadow-md"
            >
              <BookOpen className="h-4 w-4" />
              <span>进入选课大厅</span>
              <ArrowUpRight className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={onLogout}
              className="flex items-center gap-1.5 text-xs text-[var(--fg-muted)] hover:text-white"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>退出登录</span>
            </Button>
          </div>
        </header>

        {/* 错误提示条 */}
        {!stateLoading && stateErr && (
          <div className="rounded-[var(--radius-md)] border border-[var(--rose-border)] bg-[var(--rose-bg)] p-4 text-xs flex items-center justify-between text-[var(--rose)] animate-in fade-in-50">
            <div className="flex items-center gap-2">
              <XCircle className="h-4 w-4 shrink-0" />
              <span>调度器状态同步遇到问题：当前登录凭据可能已失效</span>
            </div>
            <Button variant="destructive" size="sm" onClick={onLogout}>
              重新登录
            </Button>
          </div>
        )}

        {/* 核心 Bento Grid 双翼展板 */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-5 sm:gap-6">
          {/* 左侧两大跨度：等宽秒级跳动巨幕倒计时 */}
          <Card className="lg:col-span-2 rounded-[var(--radius-xl)] border border-[var(--border)] bg-[var(--surface)] flex flex-col justify-between shadow-md">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-[var(--cyan)]" />
                <CardTitle className="text-sm font-semibold tracking-wide text-[var(--fg)]">
                  选课时间窗口监控
                </CardTitle>
              </div>
              <Badge variant={state?.window_opened ? "success" : "warning"}>
                {state?.window_opened ? "🟢 选课窗口已开放" : "⏳ 待命等待开启"}
              </Badge>
            </CardHeader>

            <CardContent className="py-4">
              {state?.window_opened ? (
                <div className="py-6 flex flex-col items-center justify-center gap-3 rounded-[var(--radius-lg)] border border-[var(--emerald-border)] bg-[var(--emerald-bg)] p-6 text-center">
                  <div className="flex items-center gap-2.5 text-[var(--emerald)] font-bold text-lg sm:text-xl">
                    <span className="inline-block w-3 h-3 rounded-full bg-[var(--emerald)] animate-ping" />
                    <span>选课窗口现已全面开放！</span>
                  </div>
                  <p className="text-xs text-[var(--fg-muted)] max-w-md">
                    系统已进入全力并发抢报模式，正在毫秒级高频检测并提交您的预选目标，请留意下方动态喵~
                  </p>
                </div>
              ) : (
                <div className="flex flex-col gap-4">
                  {/* 四格等宽大字数字矩阵 */}
                  <div className="grid grid-cols-4 gap-2 sm:gap-4 text-center">
                    <div className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-bold tabular-nums tracking-tight text-[var(--fg)]">
                        {cd.days}
                      </span>
                      <span className="text-[11px] text-[var(--fg-muted)] font-medium mt-1">
                        天
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-bold tabular-nums tracking-tight text-[var(--fg)]">
                        {cd.hours}
                      </span>
                      <span className="text-[11px] text-[var(--fg-muted)] font-medium mt-1">
                        时
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-bold tabular-nums tracking-tight text-[var(--fg)]">
                        {cd.minutes}
                      </span>
                      <span className="text-[11px] text-[var(--fg-muted)] font-medium mt-1">
                        分
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-lg)] border border-[var(--cyan-border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center shadow-[0_0_15px_rgba(14,165,233,0.1)]">
                      <span className="text-2xl sm:text-4xl font-bold tabular-nums tracking-tight text-[var(--cyan)]">
                        {cd.seconds}
                      </span>
                      <span className="text-[11px] text-[var(--cyan)] font-medium mt-1">
                        秒
                      </span>
                    </div>
                  </div>

                  {/* 人性化温馨小贴士 */}
                  <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between text-xs text-[var(--fg-muted)] pt-3 border-t border-[var(--border)] gap-2">
                    <span className="flex items-center gap-1.5">
                      <span>预计开放时间：</span>
                      <span className="text-[var(--fg)] font-medium">
                        {openTimeStr
                          ? new Date(openTimeStr).toLocaleString("zh-CN", { hour12: false })
                          : "正在同步教务平台时间配置..."}
                      </span>
                    </span>
                    <span className="text-[11px] text-[var(--cyan)] bg-[var(--cyan-bg)] px-2 py-0.5 rounded-full">
                      极速 300ms 自动并发冲刺
                    </span>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* 右侧跨度：引擎健康指标卡片 */}
          <Card className="rounded-[var(--radius-xl)] border border-[var(--border)] bg-[var(--surface)] flex flex-col justify-between shadow-md">
            <CardHeader className="pb-2">
              <div className="flex items-center gap-2">
                <Activity className="h-4 w-4 text-[var(--emerald)]" />
                <CardTitle className="text-sm font-semibold tracking-wide text-[var(--fg)]">
                  调度器运行状态
                </CardTitle>
              </div>
              <CardDescription className="text-xs text-[var(--fg-dim)]">
                后台监控心跳与防封保护参数
              </CardDescription>
            </CardHeader>

            <CardContent className="space-y-3 text-xs">
              <div className="divide-y divide-[var(--border)]">
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-[var(--fg-muted)]">心跳轮询周期</span>
                  <span className="text-[var(--fg)] font-medium tabular-nums">300 毫秒 (智能防频控)</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-[var(--fg-muted)]">已锁定目标课程</span>
                  <span className="text-[var(--cyan)] font-semibold tabular-nums">{courses.length} / 3 门</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-[var(--fg-muted)]">认证通信链路</span>
                  <span className="text-[var(--emerald)] font-medium">双通道安全握手</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-[var(--fg-muted)]">限流保护机制</span>
                  <span className="text-[var(--fg)]">单次熔断冷却开启</span>
                </div>
              </div>

              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--surface-soft)] border border-[var(--border)] flex items-center justify-between text-xs">
                <div className="flex items-center gap-2">
                  <span className="inline-block w-2 h-2 rounded-full bg-[var(--emerald)] animate-pulse" />
                  <span className="text-[var(--fg)] font-medium">后台静默待命中</span>
                </div>
                <Zap className="h-3.5 w-3.5 text-[var(--amber)]" />
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 预选目标矩阵（3 门重点看护课程卡片） */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-[var(--cyan)]" />
              <h2 className="text-sm font-semibold text-[var(--fg)]">
                重点看护目标课程
              </h2>
            </div>
            <span className="text-xs text-[var(--fg-dim)]">
              最多支持 3 门必抢目标
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
                  className={`rounded-[var(--radius-lg)] border transition-all duration-200 shadow-sm ${
                    isSuccess
                      ? "border-[var(--emerald-border)] bg-[var(--surface)] shadow-[0_0_20px_rgba(16,185,129,0.1)]"
                      : "border-[var(--border)] bg-[var(--surface)] hover:border-[var(--border-hover)]"
                  }`}
                >
                  <CardHeader className="p-4 pb-2 flex flex-row items-center justify-between">
                    <Badge variant="outline" className="text-[10px]">
                      发布 #{c.publish_id}
                    </Badge>
                    <Badge
                      variant={isSuccess ? "success" : isFailed ? "destructive" : isInRange ? "primary" : "warning"}
                      className="text-[11px]"
                    >
                      {isSuccess
                        ? "🎉 报名已确认"
                        : isFailed
                        ? "❌ 报名异常"
                        : isInRange
                        ? "⚡ 冲刺抢报中"
                        : "⏳ 待命中"}
                    </Badge>
                  </CardHeader>

                  <CardContent className="p-4 pt-1 space-y-3">
                    <div>
                      <div className="text-[11px] text-[var(--fg-dim)] font-mono">
                        课程 ID: {c.class_id}
                      </div>
                      <h3 className="font-semibold text-sm sm:text-base text-[var(--fg)] tracking-tight line-clamp-1 mt-0.5">
                        {c.course_name || `选修课程 ${c.class_id}`}
                      </h3>
                    </div>

                    {/* 状态与进度提示 */}
                    <div className="text-xs pt-2 border-t border-[var(--border)] flex items-center justify-between">
                      <div className="flex items-center gap-1.5">
                        {isSuccess && <CheckCircle2 className="h-4 w-4 text-[var(--emerald)]" />}
                        {isFailed && <XCircle className="h-4 w-4 text-[var(--rose)]" />}
                        {isInRange && <RefreshCw className="h-4 w-4 text-[var(--cyan)] animate-spin" />}
                        {!isSuccess && !isFailed && !isInRange && (
                          <span className="inline-block w-2 h-2 rounded-full bg-[var(--amber)]" />
                        )}
                        <span className="text-[var(--fg-muted)]">
                          {isSuccess
                            ? "已成功拿下席位"
                            : isFailed
                            ? "存在冲突或名额不足"
                            : isInRange
                            ? "正在以 300ms 冲刺"
                            : "开抢瞬间将自动报名"}
                        </span>
                      </div>
                    </div>

                    {c.result && (
                      <div className="p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface-soft)] text-xs text-[var(--fg-muted)] break-all leading-relaxed">
                        {c.result}
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}

            {courses.length === 0 && (
              <div className="col-span-full rounded-[var(--radius-lg)] border border-[var(--border)] border-dashed bg-[var(--surface)] p-8 text-center flex flex-col items-center justify-center gap-3">
                <HelpCircle className="h-8 w-8 text-[var(--fg-dim)] opacity-60" />
                <p className="text-sm text-[var(--fg-muted)]">
                  您尚未添加任何预选课程 · 调度器目前处于静默待命状态
                </p>
                <Button variant="primary" size="sm" onClick={onGoSelect}>
                  前往选课大厅挑选课程
                </Button>
              </div>
            )}
          </div>
        </section>

        {/* 人性化活动动态流 */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-[var(--cyan)]" />
              <h2 className="text-sm font-semibold text-[var(--fg)]">
                实时调度活动动态
              </h2>
            </div>
            <div className="flex items-center gap-2 text-xs text-[var(--fg-dim)]">
              <span className="inline-block w-2 h-2 rounded-full bg-[var(--emerald)] animate-pulse" />
              <span>自动 3 秒同步</span>
            </div>
          </div>

          <Card className="rounded-[var(--radius-xl)] border border-[var(--border)] bg-[var(--surface)] overflow-hidden shadow-sm">
            <div className="px-4 py-3 border-b border-[var(--border)] bg-[var(--surface-soft)] text-xs text-[var(--fg-dim)] flex items-center justify-between">
              <span>时间戳与动作反馈</span>
              <span>最新 50 条动态记录</span>
            </div>

            <div className="p-4 max-h-64 overflow-y-auto text-xs space-y-2.5">
              {logs && logs.length > 0 ? (
                logs.map((l) => (
                  <div
                    key={l.id}
                    className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4 border-b border-[var(--border)]/40 pb-2.5 text-xs"
                  >
                    <span className="text-[var(--fg-dim)] text-[11px] shrink-0 tabular-nums font-mono">
                      {l.created_at}
                    </span>
                    <Badge
                      variant={l.is_ok ? "success" : "destructive"}
                      className="text-[10px] px-1.5 py-0 shrink-0 w-14 justify-center"
                    >
                      {l.is_ok ? "正常" : "异常"}
                    </Badge>
                    <span className="text-[var(--fg)] font-medium shrink-0">
                      [{l.action}]
                    </span>
                    <span className="text-[var(--fg-muted)] truncate flex-1" title={l.result}>
                      {l.result}
                    </span>
                  </div>
                ))
              ) : (
                <div className="py-8 text-center text-xs text-[var(--fg-dim)]">
                  暂无日志，系统正在静默守候抢课时刻...
                </div>
              )}
            </div>
          </Card>
        </section>
      </div>

      {/* 📱 手机移动端专属底部悬浮快捷操作岛 (Mobile Dock) */}
      <div className="sm:hidden fixed bottom-4 inset-x-4 z-40 bg-[var(--surface-soft)]/90 backdrop-blur-md border border-[var(--border-hover)] rounded-[var(--radius-xl)] p-3 flex items-center justify-between shadow-2xl">
        <div className="flex items-center gap-2">
          <span className="inline-block w-2.5 h-2.5 rounded-full bg-[var(--emerald)] animate-ping" />
          <div className="flex flex-col">
            <span className="text-xs font-semibold text-[var(--fg)]">
              {state?.window_opened ? "选课已开放" : "监控就绪中"}
            </span>
            <span className="text-[10px] text-[var(--fg-muted)]">
              已选 {courses.length} / 3 门
            </span>
          </div>
        </div>
        <Button
          variant="primary"
          size="sm"
          onClick={onGoSelect}
          className="h-9 px-4 text-xs font-semibold shadow-md"
        >
          <BookOpen className="h-3.5 w-3.5 mr-1" />
          <span>选课大厅</span>
        </Button>
      </div>
    </div>
  )
}
