import { useEffect, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { LogEntry, SchedulerState } from "../types"
import { Button } from "../components/ui/Button"
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/Card"
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
  Terminal as TerminalIcon,
  XCircle,
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
    <div className="min-h-screen bg-[var(--bg)] text-[var(--fg)] p-4 sm:p-8 select-none">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部艺术级控制台顶栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[var(--border)] pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <span className="inline-block w-2 h-2 bg-white" />
              <h1 className="text-lg font-mono tracking-[0.25em] uppercase text-[var(--fg)]">
                ZHIDAO // DISPATCH CENTER
              </h1>
              <Badge variant="outline" className="text-[9px]">
                CONCURRENT ENGINE
              </Badge>
            </div>
            <p className="text-xs text-[var(--fg-dim)] font-mono tracking-wide">
              至道选课并发自动化调度系统 · 零值守抢报架构
            </p>
          </div>

          <div className="flex items-center gap-2.5">
            <Button
              variant="invert"
              size="sm"
              onClick={onGoSelect}
              className="flex items-center gap-1.5 text-xs"
            >
              <BookOpen className="h-3.5 w-3.5" />
              <span>选课大厅</span>
              <ArrowUpRight className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={onLogout}
              className="flex items-center gap-1.5 text-xs text-[var(--fg-dim)] hover:text-white"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>退出会话</span>
            </Button>
          </div>
        </header>

        {/* 错误提示条 */}
        {!stateLoading && stateErr && (
          <div className="border border-white bg-black p-4 text-xs font-mono flex items-center justify-between text-white">
            <div className="flex items-center gap-2">
              <span className="inline-block w-2 h-2 bg-white animate-ping" />
              <span>调度器状态同步失败：当前鉴权凭据可能已失效</span>
            </div>
            <Button variant="invert" size="sm" onClick={onLogout}>
              重新鉴权登录
            </Button>
          </div>
        )}

        {/* 核心 Bento Grid 顶层双翼展板 */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* 左侧两大跨度：等宽秒级跳动巨幕倒计时 */}
          <Card className="lg:col-span-2 border border-[var(--border)] bg-[var(--surface)] flex flex-col justify-between">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-[var(--fg-muted)]" />
                <CardTitle className="text-xs font-mono tracking-widest text-[var(--fg-muted)]">
                  COUNTDOWN // 开放时间窗口监测
                </CardTitle>
              </div>
              <Badge variant={state?.window_opened ? "default" : "outline"}>
                {state?.window_opened ? "WINDOW ACTIVE // 窗口已开" : "STANDBY // 等待开启"}
              </Badge>
            </CardHeader>

            <CardContent className="py-4">
              {state?.window_opened ? (
                <div className="py-6 flex flex-col items-center justify-center gap-3 border border-white bg-black p-6">
                  <div className="flex items-center gap-2.5">
                    <span className="inline-block w-3 h-3 bg-white animate-pulse" />
                    <span className="text-xl font-mono tracking-widest uppercase">
                      选课窗口现已全面开放
                    </span>
                  </div>
                  <p className="text-xs text-[var(--fg-muted)] font-mono">
                    并发秒级抢报引擎全力轮询中，请留意下方目标状态与执行日志
                  </p>
                </div>
              ) : (
                <div className="flex flex-col gap-4">
                  {/* 四格等宽大字数字矩阵 */}
                  <div className="grid grid-cols-4 gap-2 sm:gap-4 text-center">
                    <div className="border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-mono font-medium tabular-nums tracking-tight">
                        {cd.days}
                      </span>
                      <span className="text-[10px] font-mono text-[var(--fg-dim)] tracking-widest mt-1">
                        DAYS // 天
                      </span>
                    </div>
                    <div className="border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-mono font-medium tabular-nums tracking-tight">
                        {cd.hours}
                      </span>
                      <span className="text-[10px] font-mono text-[var(--fg-dim)] tracking-widest mt-1">
                        HOURS // 时
                      </span>
                    </div>
                    <div className="border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-mono font-medium tabular-nums tracking-tight">
                        {cd.minutes}
                      </span>
                      <span className="text-[10px] font-mono text-[var(--fg-dim)] tracking-widest mt-1">
                        MINS // 分
                      </span>
                    </div>
                    <div className="border border-[var(--border)] bg-[var(--surface-soft)] p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-mono font-medium tabular-nums tracking-tight text-white">
                        {cd.seconds}
                      </span>
                      <span className="text-[10px] font-mono text-[var(--fg-dim)] tracking-widest mt-1">
                        SECS // 秒
                      </span>
                    </div>
                  </div>

                  <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between text-xs font-mono text-[var(--fg-dim)] pt-2 border-t border-[var(--border)]">
                    <span>
                      目标开网时刻:{" "}
                      <span className="text-[var(--fg)]">
                        {openTimeStr
                          ? new Date(openTimeStr).toLocaleString("zh-CN", { hour12: false })
                          : "正在同步平台配置..."}
                      </span>
                    </span>
                    <span className="text-[10px] tracking-wider mt-1 sm:mt-0">
                      SWISS GRID TIMEKEEPER
                    </span>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* 右侧跨度：引擎规格与健康指示卡片 */}
          <Card className="border border-[var(--border)] bg-[var(--surface)] flex flex-col justify-between">
            <CardHeader className="pb-2">
              <div className="flex items-center gap-2">
                <Activity className="h-4 w-4 text-[var(--fg-muted)]" />
                <CardTitle className="text-xs font-mono tracking-widest text-[var(--fg-muted)]">
                  ENGINE TELEMETRY // 遥测指标
                </CardTitle>
              </div>
            </CardHeader>

            <CardContent className="space-y-4 text-xs font-mono">
              <div className="divide-y divide-[var(--border)]">
                <div className="py-2 flex items-center justify-between">
                  <span className="text-[var(--fg-dim)]">调度器心跳周期</span>
                  <span className="text-[var(--fg)] tabular-nums">300 ms (抗频控限流)</span>
                </div>
                <div className="py-2 flex items-center justify-between">
                  <span className="text-[var(--fg-dim)]">预选目标课程</span>
                  <span className="text-[var(--fg)] tabular-nums">{courses.length} / 3 门</span>
                </div>
                <div className="py-2 flex items-center justify-between">
                  <span className="text-[var(--fg-dim)]">通信鉴权通道</span>
                  <span className="text-[var(--fg)]">Cookie + idToken 双路</span>
                </div>
                <div className="py-2 flex items-center justify-between">
                  <span className="text-[var(--fg-dim)]">重登安全锁</span>
                  <span className="text-[var(--fg)]">单次熔断保护开启</span>
                </div>
              </div>

              <div className="p-3 bg-[var(--surface-soft)] border border-[var(--border)] flex items-center justify-between text-[11px]">
                <div className="flex items-center gap-2">
                  <span className="inline-block w-1.5 h-1.5 bg-white" />
                  <span className="text-[var(--fg)]">后台静默常驻中</span>
                </div>
                <RefreshCw className="h-3 w-3 text-[var(--fg-dim)] animate-spin" />
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 核心预选目标矩阵（3 门课程 Bento 卡片） */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Layers className="h-4 w-4 text-[var(--fg-muted)]" />
              <h2 className="text-xs font-mono tracking-widest uppercase text-[var(--fg-muted)]">
                TARGET COURSES // 预选监控目标
              </h2>
            </div>
            <span className="text-[10px] font-mono text-[var(--fg-dim)] tracking-widest">
              MAX 3 PUBLISHES
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
                  className={`border transition-all duration-150 ${
                    isSuccess
                      ? "border-white bg-black"
                      : "border-[var(--border)] bg-[var(--surface)] hover:border-[var(--border-strong)]"
                  }`}
                >
                  <CardHeader className="p-4 pb-2 flex flex-row items-center justify-between">
                    <Badge variant="outline" className="text-[9px]">
                      发布 #{c.publish_id}
                    </Badge>
                    <Badge
                      variant={isSuccess ? "default" : isFailed ? "active" : "secondary"}
                      className="text-[9px]"
                    >
                      {c.status.toUpperCase()}
                    </Badge>
                  </CardHeader>

                  <CardContent className="p-4 pt-1 space-y-3">
                    <div>
                      <div className="text-[10px] font-mono text-[var(--fg-dim)]">
                        ID: {c.class_id}
                      </div>
                      <h3 className="font-medium text-sm text-[var(--fg)] tracking-wide line-clamp-1 mt-0.5">
                        {c.course_name || `选修课程 ${c.class_id}`}
                      </h3>
                    </div>

                    {/* 状态徽章与反馈 */}
                    <div className="text-xs font-mono pt-2 border-t border-[var(--border)] flex items-center justify-between">
                      <div className="flex items-center gap-1.5">
                        {isSuccess && <CheckCircle2 className="h-3.5 w-3.5 text-white" />}
                        {isFailed && <XCircle className="h-3.5 w-3.5 text-white" />}
                        {isInRange && <RefreshCw className="h-3.5 w-3.5 text-white animate-spin" />}
                        {!isSuccess && !isFailed && !isInRange && (
                          <span className="inline-block w-1.5 h-1.5 bg-[var(--fg-dim)]" />
                        )}
                        <span className="text-[var(--fg-muted)]">
                          {isSuccess
                            ? "报名已确认"
                            : isFailed
                            ? "报名异常"
                            : isInRange
                            ? "并发抢报中"
                            : "待命守候中"}
                        </span>
                      </div>
                    </div>

                    {c.result && (
                      <div className="p-2 border border-[var(--border)] bg-[var(--surface-soft)] text-[10px] font-mono text-[var(--fg-dim)] break-all">
                        {c.result}
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}

            {courses.length === 0 && (
              <div className="col-span-full border border-[var(--border)] border-dashed bg-[var(--surface)] p-8 text-center flex flex-col items-center justify-center gap-3">
                <p className="text-xs font-mono text-[var(--fg-dim)] tracking-wider">
                  尚未设定任何预选课程 · 调度器暂处于空转待命状态
                </p>
                <Button variant="invert" size="sm" onClick={onGoSelect}>
                  前往选课大厅配置目标
                </Button>
              </div>
            )}
          </div>
        </section>

        {/* 极客终端风格事件日志流 */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <TerminalIcon className="h-4 w-4 text-[var(--fg-muted)]" />
              <h2 className="text-xs font-mono tracking-widest uppercase text-[var(--fg-muted)]">
                EVENT STREAM // 实时调度事件日志
              </h2>
            </div>
            <div className="flex items-center gap-2 text-[10px] font-mono text-[var(--fg-dim)]">
              <span className="inline-block w-1.5 h-1.5 bg-white" />
              <span>AUTO-POLLING 3s</span>
            </div>
          </div>

          <Card className="border border-[var(--border)] bg-black overflow-hidden">
            <div className="px-4 py-2 border-b border-[var(--border)] bg-[var(--surface-soft)] text-[10px] font-mono text-[var(--fg-dim)] flex items-center justify-between">
              <span>UNIX TIMESTAMP // ACTION // STATUS // PAYLOAD</span>
              <span>BUFFER LIMIT: 50</span>
            </div>

            <div className="p-4 max-h-64 overflow-y-auto font-mono text-xs space-y-2">
              {logs && logs.length > 0 ? (
                logs.map((l) => (
                  <div
                    key={l.id}
                    className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4 border-b border-[var(--border)]/40 pb-2 text-[11px]"
                  >
                    <span className="text-[var(--fg-dim)] text-[10px] shrink-0 tabular-nums">
                      [{l.created_at}]
                    </span>
                    <Badge
                      variant={l.is_ok ? "default" : "active"}
                      className="text-[8px] px-1 py-0 shrink-0 w-12 justify-center"
                    >
                      {l.is_ok ? "OK" : "ERR"}
                    </Badge>
                    <span className="text-white shrink-0 uppercase tracking-wider">
                      [{l.action}]
                    </span>
                    <span className="text-[var(--fg-muted)] truncate flex-1" title={l.result}>
                      {l.result}
                    </span>
                  </div>
                ))
              ) : (
                <div className="py-6 text-center text-xs text-[var(--fg-dim)] font-mono">
                  &gt; 等待调度引擎触发事件...
                </div>
              )}
            </div>
          </Card>
        </section>
      </div>
    </div>
  )
}
