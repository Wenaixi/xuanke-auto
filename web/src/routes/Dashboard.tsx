import { useMemo, useState, useEffect, useId } from "react"
import { useQuery } from "@tanstack/react-query"
import { ApiError, api } from "../api/client"
import type { Account, CourseStatus, ElectivesData, LogEntry, SchedulerState } from "../types"
import { Button } from "../components/ui/Button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { useTickingCountdown } from "../lib/useTickingCountdown"
import {
  Activity,
  ArrowUpRight,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  Clock,
  Layers,
  LogOut,
  RefreshCw,
  XCircle,
  HelpCircle,
} from "lucide-react"

interface Props {
  account: Account
  sessionToken: string
  onLogout: () => void
  onGoSelect: () => void
}

// 优先级序号转展示名：0=首选，1=备选 1，2=备选 2
function priorityName(p: number): string {
  return p === 0 ? "首选" : `备选 ${p}`
}

// 是否为满员退避：失败且原因明确写"已满员"（调度器 markFullLocked 文案）
function isFullFallback(c: { status: string; result: string }): boolean {
  return c.status === "failed" && c.result.includes("已满员")
}

// N10：错误条只在会话真正失效（业务码 401）时显示"请重新登录"。
// 网络错误/服务端 5xx 会被 react-query 重试，误判为会话失效会让用户无谓重登。
function isSessionError(err: unknown): boolean {
  return err instanceof ApiError && err.code === 401
}

// CollapseSection 极简折叠段：时间列表与日期分组共用。
// 项目无 Radix Collapsible（未安装），且折叠只是"按钮 + 条件渲染"，手写最小实现
// 符合简洁优先，绝不为此引入新依赖。button 带 aria-expanded/aria-controls，
// 键盘可聚焦、读屏可感知展开状态，零额外成本的可访问性。
function CollapseSection({
  open,
  onToggle,
  header,
  children,
}: {
  open: boolean
  onToggle: () => void
  header: React.ReactNode
  children: React.ReactNode
}) {
  // N2：aria-controls 指向的 id 此前是硬编码 "collapse-body"——正文 div 无 id 落位
  // （悬空，读屏收不到跟随目标），且组件多处实例化（extras 折叠段 + 每个日期组）
  // 时重复 id 违反 DOM 唯一性。useId 保证单实例唯一、零冲突。
  const id = useId()
  return (
    <div className="rounded-[var(--radius-sm)] glass border border-neutral-800 overflow-hidden">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={id}
        onClick={onToggle}
        className="w-full flex items-center justify-between gap-2 px-3 py-2.5 text-left hover:bg-white/[0.03] transition-colors"
      >
        <span className="text-xs text-neutral-300 flex items-center gap-2">{header}</span>
        <ChevronDown
          className={`h-3.5 w-3.5 text-neutral-500 shrink-0 transition-transform duration-200 ${
            open ? "rotate-180" : ""
          }`}
        />
      </button>
      {open && (
        <div id={id} className="px-3 pb-3 border-t border-neutral-900 pt-2">
          {children}
        </div>
      )}
    </div>
  )
}

// relativeCountdown 折叠行内联时间摘要：主矩阵 useTickingCountdown 每秒 tick 驱动
// 整页重渲染（lib effect 无条件 setInterval(1000)），折叠行渲染期直接算 Date.now()
// 即新鲜（最坏 1 秒陈旧），绝不为此再建额外定时器。
// 已过/已开放 → "已开放"；今天以内 → "X小时 X分" 紧凑摘要；更远 → 天/时/分全量。
function relativeCountdown(ms: number, nowMs: number): string {
  const diff = ms - nowMs
  if (diff <= 0) return "已开放"
  const totalMin = Math.floor(diff / 60000)
  if (totalMin < 60) return `${totalMin}分`
  const totalHour = Math.floor(totalMin / 60)
  if (totalHour < 24) {
    const min = totalMin % 60
    return `${totalHour}小时${min > 0 ? ` ${min}分` : ""}`
  }
  const d = Math.floor(totalHour / 24)
  const h = totalHour % 24
  const m = totalMin % 60
  return `${d}天 ${h}小时 ${m}分`
}

// parseDateKey 把分组键（"2026-09-13"）解析为本地零点时间戳。
// 裸字符串 "2026-09-13" 会被 JS 按 UTC 零点解析，与本地时区差 8 小时 →
// 今天凌晨附近的日期会偏移到昨天。显式拼 "T00:00:00" 锁本地零点。
function parseDateKey(k: string): number {
  return new Date(k + "T00:00:00").getTime()
}

// localTodayMs：本地日期零点（N1）。与 parseDateKey 的本地零点同基准——
// toISOString() 恒 UTC，UTC+8 凌晨 00:00-07:59 时拿到的是昨天日期，日期分组
// "距今天最近在前"排序整体错一档；setHours(0,0,0,0) 锁本地零点无此偏移。
export function localTodayMs(now: Date = new Date()): number {
  const today = new Date(now)
  today.setHours(0, 0, 0, 0)
  return today.getTime()
}

export default function Dashboard({ account, sessionToken, onLogout, onGoSelect }: Props) {
  // n12：窗口已关闭后把轮询降频到 30 秒——状态已定型（快照为空、
  // 不会再有新动静），继续 3 秒高频打 /state 与 /logs 纯属浪费请求与刷屏日志；
  // 窗口开放中仍保持 3 秒紧贴实时状态。函数式 refetchInterval 基于已获取的
  // state 数据实时决定下一拍间隔，窗口关闭瞬间自动切换，无需额外状态。
  const { data: state, isError: stateErr, isLoading: stateLoading } = useQuery({
    queryKey: ["state", account, sessionToken],
    queryFn: () => api<SchedulerState>("/state", { session: sessionToken }),
    // n12：函数式间隔从 query.state.data 读取已取回的窗口状态（不引用本闭包的 state，
    // 避免循环初始化推断）；窗口已关闭降频 30s，开放中保持 3s 紧贴实时状态
    refetchInterval: (query) => (query.state.data?.window_closed ? 30000 : 3000),
  })

  const { data: logs } = useQuery({
    queryKey: ["logs", sessionToken],
    queryFn: () => api<LogEntry[]>("/logs", { session: sessionToken }),
    // F9-05：澄清：logs 降频读组件闭包 state（/state 查询数据）即新鲜——
    // /state 每 3s 刷新（或窗口关闭降频 30s 后低频刷新），数据一变组件重渲染，
    // react-query 用最新闭包重调度本查询的轮询间隔，不存在"闭包停旧值永不降频"。
    // 窗口关闭瞬间 /state 先返回 window_closed=true，下一次日志轮询即按 30s 走。
    refetchInterval: () => (state?.window_closed ? 30000 : 3000),
  })

  // /electives 查询：Dashboard 新增数据源——多开放时间列表（begin_times 数组）
  // 与发布元数据（publish_id → publish_name/begin_date 映射）都只在这里有。
  // 与 Select.tsx 同 key 同 URL：两路由互斥挂载但共享 react-query 缓存，切页零重复请求。
  // 轮询恒 30 秒：本页只消费 begin_times（平台下发后几乎不变）与 publishes 元数据
  // （学期内静态），不消费 classes 实时名额；后端快照 TTL 40s > 30s，低于 40s 轮询
  // 大概率拿同一份快照纯耗带宽。窗口关闭后响应是空结构体（begin_times/publishes
  // 同时清空，契约见 plan），30s 轮询成本≈0 且能发现新一轮开窗。
  const { data: electives } = useQuery({
    queryKey: ["electives", account, sessionToken],
    queryFn: () => api<ElectivesData>("/electives?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: 30000,
  })

  // F8-04：删除整页每秒 setTick——倒计时内部自 tick（useTickingCountdown），
  // 日志列表/状态卡片不再每秒全量重建。数组改为 hooks 层的派生常量，杜绝重复计算
  // F9-07：useTickingCountdown 收敛到 lib/ 共用（Dashboard/Select 同一实现），
  // 且 effect 依赖 [] 时 interval 内读 Date.now() 而非闭包 state——绝不随 target 卡旧值。
  const courses = state?.courses ?? []
  // 开放时间唯一事实源 = 平台 beginTimes 自动识别（不可配置，识别槽已含"识别过期"
  // 语义：窗口关闭/批次已过后识别值视为无效）；识别不到 = 未知（open_time_known=false），
  // 倒计时组件 target=null 即显全 00 + 过期态，绝不显示编造时间。
  const openTimeStr =
    state?.open_time_known && state.open_time ? state.open_time : null
  // N1：主倒计时输入 openTimeStr 未识别时用 begin_times[0] 兜底（唯一未来开窗点），
  // 避免"主矩阵全 00 过期态 + 同屏『预计开放时间』显示未来时刻"的自相矛盾；识别槽
  // 建立（open_time_known=true）后仍以识别真值为准，兜底只在识别缺席时生效。
  const cd = useTickingCountdown(
    openTimeStr ??
      (electives?.begin_times?.[0] != null
        ? new Date(electives.begin_times[0]).toISOString()
        : null)
  )

  // 折叠行实时标签由主矩阵每秒 tick 的整页重渲染驱动（见 relativeCountdown 注释）。
  const nowMs = Date.now()

  // 主倒计时 = 调度器识别到的有效开放时间（平台 beginTimes 唯一事实源，与 openTimeStr
  // 同源）；其余 begin_times 按从近到远排进"其他开放时间"折叠段。openTimeStr 未识别时用
  // begin_times[0] 兜底当下主时间，避免"主矩阵未知但折叠列表有值"的悬空。
  // 依赖用 electives?.begin_times 稳定引用（react-query 数据不变引用不变），
  // fallback 空数组在 useMemo 内部——绝不在渲染期新建数组让依赖每次都变。
  const primaryMs = openTimeStr
    ? new Date(openTimeStr).getTime()
    : electives?.begin_times?.[0]
  const extrasMs = useMemo(() => {
    const list = electives?.begin_times ?? []
    return list.filter((t) => primaryMs !== undefined && t !== primaryMs).sort((a, b) => a - b)
  }, [electives?.begin_times, primaryMs])
  const [extrasOpen, setExtrasOpen] = useState(false)

  // 预选目标按日期分组：分组键优先 CourseStatus 自带 begin_date（调度器随目标持久化，
  // 窗口关闭后 /state 仍自带日期/发布名——关闭≠元数据丢失）；兜底 /electives 映射
  // （旧目标/历史课程无元数据时），映射也缺失（窗口关闭、幽灵 id）落"未知"。
  // 窗口关闭后即使映射全空，课程卡片仍按自带元数据正确分组，目标可读性不降级——
  // 这正是用户窗口关闭后要盯的终态。
  const dateGroups = useMemo(() => {
    const pubs = electives?.publishes ?? []
    const pubById = new Map(pubs.map((p) => [p.publish_id, p]))
    const byDate = new Map<string, Map<number, CourseStatus[]>>()
    for (const c of courses) {
      const dateKey =
        (c.begin_date ?? "").slice(0, 10) ||
        (pubById.get(c.publish_id)?.begin_date ?? "").slice(0, 10) ||
        "未知"
      let byPub = byDate.get(dateKey)
      if (!byPub) {
        byPub = new Map()
        byDate.set(dateKey, byPub)
      }
      const list = byPub.get(c.publish_id) ?? []
      list.push(c)
      byPub.set(c.publish_id, list)
    }
    // 日期排序：|日期 - 今天零点| 升序 = 距今天最近的天在前（未来/已过统一成立），
    // 平局按日期值升序；"未知"恒排最后（窗口关闭兜底组）。
    const todayMs = localTodayMs()
    const dateKeys = [...byDate.keys()].sort((a, b) => {
      if (a === "未知") return 1
      if (b === "未知") return -1
      const da = Math.abs(parseDateKey(a) - todayMs)
      const db = Math.abs(parseDateKey(b) - todayMs)
      return da !== db ? da - db : a < b ? -1 : 1
    })
    return dateKeys.map((key) => {
      const byPub = byDate.get(key)!
      const groups = [...byPub.entries()]
        .sort(([a], [b]) => a - b) // 发布按 publish_id 升序
        .map(([publishId, items]) => ({
          publish_id: publishId,
          // 发布名优先 CourseStatus 自带（窗口关闭仍可读），兜底 /electives 映射
          name:
            items[0].publish_name ||
            pubById.get(publishId)?.publish_name ||
            `发布 #${publishId}`,
          items: [...items].sort((a, b) => a.priority - b.priority), // 组内按优先级升序
        }))
      const count = groups.reduce((n, g) => n + g.items.length, 0)
      return { key, count, pubs: groups }
    })
  }, [courses, electives?.publishes])

  // 折叠种子：null = 尚未初始化（首次数据到达种下最近的日期组），[] = 用户主动
  // 全折叠。null/[] 语义分离保证绝不重种——否则用户收起全部后 effect 又把最近组拉出来，
  // 折叠状态与用户操作打架。首帧种子后不追踪最近组漂移，用户操作即最终话语权。
  const [expandedDates, setExpandedDates] = useState<string[] | null>(null)
  useEffect(() => {
    if (expandedDates === null && dateGroups.length > 0) {
      setExpandedDates([dateGroups[0].key])
    }
  }, [expandedDates, dateGroups])

  return (
    <div className="min-h-screen text-white p-4 sm:p-6 lg:p-8 select-none pb-24 sm:pb-8">
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
            <Button
              variant="dark"
              size="sm"
              onClick={onGoSelect}
              className="flex items-center gap-1.5 text-xs"
            >
              <BookOpen className="h-3.5 w-3.5 text-white" />
              <span>选课大厅</span>
              <ArrowUpRight className="h-3.5 w-3.5 text-white" />
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
        {!stateLoading && stateErr && isSessionError(stateErr) && (
          <div className="rounded-[var(--radius-sm)] border border-neutral-800 glass-strong p-4 text-xs flex items-center justify-between text-neutral-300">
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
          <Card className="lg:col-span-2 rounded-[var(--radius-lg)] glass border border-neutral-800 flex flex-col justify-between shadow-none">
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
                <div className="py-6 flex flex-col items-center justify-center gap-3 rounded-[var(--radius-sm)] glass border border-neutral-800 p-6 text-center">
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
                    <div className="rounded-[var(--radius-sm)] glass border border-neutral-800 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.days}
                      </span>
                      <span className="text-[11px] text-neutral-500 font-mono uppercase mt-1">
                        天
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-sm)] glass border border-neutral-800 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.hours}
                      </span>
                      <span className="text-[11px] text-neutral-500 font-mono uppercase mt-1">
                        时
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-sm)] glass border border-neutral-800 p-3 sm:p-5 flex flex-col items-center">
                      <span className="text-2xl sm:text-4xl font-light tabular-nums tracking-tight text-white">
                        {cd.minutes}
                      </span>
                      <span className="text-[11px] text-neutral-500 font-mono uppercase mt-1">
                        分
                      </span>
                    </div>
                    <div className="rounded-[var(--radius-sm)] glass border border-neutral-700 glass-strong p-3 sm:p-5 flex flex-col items-center">
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
                        {/* M2：文案行与主倒计时矩阵同源吃 begin_times 兜底——识别槽未建立
                            （open_time_known=false）但平台已下发开窗点时，矩阵已明确倒数，
                            文案不能再显"未识别到开放时间"自相矛盾。识别真值优先，兜底只
                            在识别缺席时生效，识别也缺席才回落到"未识别"/"同步中"。 */}
                        {openTimeStr
                          ? new Date(openTimeStr).toLocaleString("zh-CN", { hour12: false })
                          : electives?.begin_times?.[0] != null
                            ? new Date(electives.begin_times[0]).toLocaleString("zh-CN", { hour12: false })
                            : state
                              ? "未识别到开放时间"
                              : "正在同步教务平台时间配置..."}
                      </span>
                    </span>
                    <span className="text-neutral-500">
                      30 秒查询节流
                    </span>
                  </div>

                  {/* 多开放时间折叠段：平台 beginTimes 是数组契约（真实 select.js 遍历全
                      数组做 1500ms 开窗探测），当前学期只下发单值故本段大概率不出现——
                      单值路径与改前逐像素一致。其余时间从近到远已排入 extrasMs，
                      点开逐行显示绝对时间 + 实时摘要。主时间 = 调度器识别开窗真值，故意
                      不与 begin_times 合并（识别态唯一事实源，见 openTimeStr 注释）。 */}
                  {extrasMs.length > 0 && (
                    <CollapseSection
                      open={extrasOpen}
                      onToggle={() => setExtrasOpen((o) => !o)}
                      header={
                        <>
                          <Clock className="h-3.5 w-3.5 text-neutral-500" />
                          <span>其他开放时间</span>
                          <span className="text-neutral-600 font-mono">· {extrasMs.length}</span>
                        </>
                      }
                    >
                      <div className="space-y-2">
                        {extrasMs.map((ms) => (
                          <div
                            key={ms}
                            className="flex items-center justify-between text-xs font-mono py-1"
                          >
                            <span className="text-white">
                              {new Date(ms).toLocaleString("zh-CN", { hour12: false })}
                            </span>
                            <span className="text-neutral-500">
                              {relativeCountdown(ms, nowMs)}
                            </span>
                          </div>
                        ))}
                      </div>
                    </CollapseSection>
                  )}
                </div>
              )}
            </CardContent>
          </Card>

          {/* 右侧跨度：运行状态指标卡片 */}
          <Card className="rounded-[var(--radius-lg)] glass border border-neutral-800 flex flex-col justify-between shadow-none">
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
                  {/* F10-06：去掉"/ 3 门"——后端上限 100 门且每发布可配多条备选，
                      3 门是早期"每账号至多 3 门"旧约束残留，硬编码展示与真实能力分叉 */}
                  <span className="text-white font-mono tabular-nums">{courses.length} 门</span>
                </div>
                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-neutral-400">教务令牌</span>
                  <span className="flex items-center gap-1.5">
                    {state?.token_valid === false ? (
                      <>
                        <span className="inline-block w-1.5 h-1.5 rounded-full bg-white/40 animate-pulse" />
                        <span className="text-white/60 font-mono">已失效 · 自动恢复中</span>
                      </>
                    ) : (
                      <>
                        <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
                        <span className="text-white font-mono">有效</span>
                      </>
                    )}
                  </span>
                </div>                <div className="py-2.5 flex items-center justify-between">
                  <span className="text-neutral-400">频控保护策略</span>
                  <span className="text-white">分级退避 · 自动恢复</span>
                </div>
              </div>

              <div className="p-3 rounded-[var(--radius-sm)] glass border border-neutral-800 flex items-center justify-between text-xs">
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
              {/* F10-06：3 门是旧约束残留，现为动态目标集合，展示名改 TARGETS */}
              TARGETS
            </span>
          </div>

          <div className="space-y-3">
            {dateGroups.length === 0 && (
              <div className="rounded-[var(--radius-lg)] border border-neutral-800 border-dashed glass p-8 text-center flex flex-col items-center justify-center gap-3">
                <HelpCircle className="h-7 w-7 text-neutral-600" />
                <p className="text-xs text-neutral-400">
                  当前未添加任何预选课程
                </p>
                <Button variant="outline" size="sm" onClick={onGoSelect}>
                  前往挑选课程
                </Button>
              </div>
            )}

            {dateGroups.map((g) => {
              const expanded = expandedDates?.includes(g.key) ?? false
              return (
                <CollapseSection
                  key={g.key}
                  open={expanded}
                  onToggle={() =>
                    setExpandedDates((prev) => {
                      const cur = prev ?? []
                      return cur.includes(g.key)
                        ? cur.filter((k) => k !== g.key)
                        : [...cur, g.key]
                    })
                  }
                  header={
                    <>
                      {g.key === "未知" ? (
                        <HelpCircle className="h-3.5 w-3.5 text-neutral-500" />
                      ) : (
                        <Clock className="h-3.5 w-3.5 text-neutral-500" />
                      )}
                      <span className="text-white">
                        {g.key === "未知" ? "未知日期" : g.key}
                      </span>
                      <span className="text-neutral-600 font-mono">· {g.count} 门</span>
                      {g.key === "未知" && (
                        <span className="text-neutral-600">窗口已关闭，发布信息不可用</span>
                      )}
                    </>
                  }
                >
                  {g.pubs.map((pub) => (
                    <div key={pub.publish_id} className="mb-4 last:mb-0">
                      <div className="text-[11px] text-neutral-400 font-medium font-mono uppercase tracking-wider mb-2 flex items-center gap-2">
                        <Layers className="h-3 w-3 text-neutral-600" />
                        <span>{pub.name}</span>
                        <span className="text-neutral-600">· {pub.items.length} 门</span>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        {pub.items.map((c) => {
                          const isSuccess = c.status === "success"
                          const isFull = isFullFallback(c)
                          const isFailed = c.status === "failed" && !isFull
                          const isInRange = c.status === "in_range" || c.status === "submitted"

                          return (
                            <Card
                              key={c.class_id}
                              className={`rounded-[var(--radius-lg)] border transition-all duration-200 shadow-none ${
                                isSuccess
                                  ? "border-white glass-strong"
                                  : "glass border-neutral-800 hover:border-neutral-600"
                              }`}
                            >
                              <CardHeader className="p-4 pb-2 flex flex-row items-center justify-between border-b border-neutral-900">
                                <Badge variant="outline" className="text-[10px] font-mono">
                                  #{c.publish_id} · {priorityName(c.priority)}
                                </Badge>
                                <Badge
                                  variant={isSuccess ? "primary" : isFull ? "destructive" : isFailed ? "destructive" : isInRange ? "primary" : "outline"}
                                  className="text-[11px]"
                                >
                                  {isSuccess
                                    ? "已确认选课"
                                    : isFull
                                    ? "已满员·退避备选"
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
                                    {isFull && <XCircle className="h-3.5 w-3.5 text-white" />}
                                    {isInRange && <RefreshCw className="h-3.5 w-3.5 text-white animate-spin" />}
                                    {!isSuccess && !isFailed && !isInRange && !isFull && (
                                      <span className="inline-block w-1.5 h-1.5 rounded-full bg-neutral-600" />
                                    )}
                                    <span className="text-neutral-400">
                                      {isSuccess
                                        ? "席位已确认"
                                        : isFull
                                        ? "该门已满，自动退避至下一备选"
                                        : isFailed
                                        ? "提交未通过"
                                        : isInRange
                                        ? "冲刺提交中"
                                        : "开放时自动提交"}
                                    </span>
                                  </div>
                                </div>

                                {c.result && (
                                  <div className="p-2.5 rounded-[var(--radius-sm)] glass border border-neutral-800 text-xs text-neutral-400 font-mono break-all leading-relaxed">
                                    {c.result}
                                  </div>
                                )}
                              </CardContent>
                            </Card>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </CollapseSection>
              )
            })}
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

          <Card className="rounded-[var(--radius-lg)] glass border border-neutral-800 overflow-hidden shadow-none">
            <div className="px-4 py-2.5 border-b border-neutral-900 text-xs text-neutral-500 flex items-center justify-between font-mono">
              <span>EVENT STREAM</span>
              {/* F7-08：后端 LoadLogs 上限 100 条，文案与真实数量对齐（此前 RECENT 50 失真） */}
              <span>RECENT 100</span>
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

      {/* 手机移动端底部悬浮操作栏 */}
      <div className="sm:hidden fixed bottom-4 inset-x-4 z-40 glass-strong border border-neutral-700 rounded-[var(--radius-lg)] p-3 flex items-center justify-between shadow-none">
        <div className="flex items-center gap-2">
          <span className="inline-block w-2 h-2 rounded-full bg-white" />
          <div className="flex flex-col">
            <span className="text-xs font-medium text-white">
              {state?.window_opened ? "窗口开放中" : "系统待命中"}
            </span>
            <span className="text-[10px] text-neutral-500 font-mono">
              {account ? `ACCOUNT ${account}` : "DEFAULT"} · TARGETS {courses.length}
            </span>
          </div>
        </div>
        <Button
          variant="dark"
          size="sm"
          onClick={onGoSelect}
          className="h-8 px-3 text-xs"
        >
          <BookOpen className="h-3.5 w-3.5 mr-1 text-white" />
          <span>选课大厅</span>
        </Button>
      </div>
    </div>
  )
}
