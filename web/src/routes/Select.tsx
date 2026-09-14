import { useMemo, useState, useEffect, useRef } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { api, selectElective, exitElective } from "../api/client"
import type { Account, ClassItem, ElectivesData, Target, SchedulerState } from "../types"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { Progress } from "../components/ui/Progress"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "../components/ui/Tabs"
import { useToast } from "../components/ui/Toast"
import {
  ArrowLeft,
  ArrowDownWideNarrow,
  BookMarked,
  Check,
  Clock,
  Filter,
  LogOut,
  MapPin,
  Search,
  User,
  Users,
  AlertTriangle,
} from "lucide-react"

interface Props {
  account: Account
  sessionToken: string
  onDone: () => void
}

function fillRate(c: ClassItem): number {
  if (!c.max_count) return 0
  return Math.min(100, Math.round((c.selected_count / c.max_count) * 100))
}

// 优先级序号转展示名：0=首选，1=备选 1，2=备选 2（首位不再是"备选 1"）
function priorityName(p: number): string {
  return p === 0 ? "首选" : `备选 ${p}`
}

// 选课开放倒计时解析：返回天/时/分/秒与是否已过期
function parseCountdown(target: string | null): {
  days: string
  hours: string
  minutes: string
  seconds: string
  isExpired: boolean
} {
  if (!target) {
    return { days: "00", hours: "00", minutes: "00", seconds: "00", isExpired: true }
  }
  const diff = new Date(target).getTime() - Date.now()
  if (diff <= 0) {
    return { days: "00", hours: "00", minutes: "00", seconds: "00", isExpired: true }
  }
  const pad = (n: number) => n.toString().padStart(2, "0")
  const total = Math.floor(diff / 1000)
  return {
    days: pad(Math.floor(total / 86400)),
    hours: pad(Math.floor((total % 86400) / 3600)),
    minutes: pad(Math.floor((total % 3600) / 60)),
    seconds: pad(total % 60),
    isExpired: false,
  }
}

export default function Select({ account, sessionToken, onDone }: Props) {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const [actionLoading, setActionLoading] = useState<number | null>(null)
  const [exitModalClass, setExitModalClass] = useState<ClassItem | null>(null)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["electives", account, sessionToken],
    queryFn: () => api<ElectivesData>("/electives?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: (query) => {
      // 轮询判定来源：in_date_range 与 window_opened 双信号合并（MAJOR-G）。
      // 窗口即将开启的瞬间平台会短暂返回空 publishes，此时仅凭 in_date_range 会把
      // 10s 慢轮询带到黄金期——必须并入调度器侧 window_opened 信号，一开窗立即升频 2s。
      // F5-05（第 5 轮）：窗口已关闭（window_closed）并入降频——关闭后课程列表已被平台
      // 清空，继续 10s 高频打 findElectivesData 纯浪费；与 /state 同信号降 30s，全站统一。
      const pubs = query.state.data?.publishes ?? []
      const inRange = pubs.some((p) => p.in_date_range)
      if (stateData?.window_closed) return 30000
      return inRange || stateData?.window_opened ? 2000 : 10000
    },
  })

  // 手动报名指定课程
  const handleSelectClass = async (c: ClassItem) => {
    setActionLoading(c.id)
    try {
      const res = await selectElective(c.id, sessionToken, account, c.course_name)
      toast({ title: "报名成功", description: res.msg || "已成功选报该课程", variant: "success" })
    } catch (err: any) {
      toast({ title: "报名失败", description: err.message || "请求被拒绝", variant: "destructive" })
    } finally {
      // N2：无论成功失败都强制失效 electives/state 缓存——
      // 报名失败（满员/窗口关闭）后名额与按钮状态同样已变化，必须立即刷新，
      // 否则前端显示"还可报名"实则已满，用户看到的是过期数据。
      queryClient.invalidateQueries({ queryKey: ["electives"] })
      queryClient.invalidateQueries({ queryKey: ["state"] })
      setActionLoading(null)
    }
  }

  // 手动退选指定课程二次确认提交
  const handleConfirmExit = async (c: ClassItem) => {
    setActionLoading(c.id)
    try {
      const res = await exitElective(c.id, sessionToken, account)
      toast({ title: "退选成功", description: res.msg || "已成功退选该课程", variant: "success" })
      setExitModalClass(null)
    } catch (err: any) {
      toast({ title: "退选失败", description: err.message || "请求被拒绝", variant: "destructive" })
    } finally {
      queryClient.invalidateQueries({ queryKey: ["electives"] })
      queryClient.invalidateQueries({ queryKey: ["state"] })
      setActionLoading(null)
    }
  }

  // 查询当前调度器已保存的目标课程并自动回显（会话绑定当前账号）。
  // M-8（第 3 轮）：加 2s 自轮询——electives 的升频判定依赖 window_opened 信号，
  // 若此查询被动等 electives invalidate 才刷新，开窗瞬间（publishes 短暂为空）会把
  // 10s 慢轮询带进黄金期；独立轮询让 window_opened 一开窗立即升频 2s，两信号同源。
  // n12（第 4 轮）：窗口已关闭后降回 30s——与 Dashboard 同一信号同一次序，
  // 避免窗口关闭后仍 2s 高频打 /state 刷屏日志。
  const { data: stateData } = useQuery({
    queryKey: ["state", account, sessionToken],
    queryFn: () => api<SchedulerState>("/state?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: (query) => (query.state.data?.window_closed ? 30000 : 2000),
  })

  const [selected, setSelected] = useState<Record<number, ClassItem[]>>({})
  // 用户真实改动计数：驱动自动保存的 400ms 防抖；回显数据不经过它，故不会触发无意义保存。
  // 注意：F7 修复后它只归 pick()/清空操作自增——轮询拉回的 publishes 变化绝不触发保存。
  const [rev, setRev] = useState(0)
  const [search, setSearch] = useState("")
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [sortTightest, setSortTightest] = useState(false)

  // 本地每秒刷新倒计时，确保数字秒级平滑跳动
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(timer)
  }, [])

  // 进入页面时自动回显已保存的目标课程（含多备选优先级）。
  // M-9（第 3 轮）：函数体内统一用 prev 构造初始值，杜绝 `const initial` 遮蔽
  // 外部 `selected` 导致数据重取后回显永久失效的问题。
  // 第 4 轮：用户已编辑过目标（rev>0）时跳过回显——用户"清空全部目标"后 2s 轮询
  // 返回的旧 courses 若再次回填，会把清空静默撤销并重新保存旧目标（回显与防抖保存竞态）。
  useEffect(() => {
    if (rev > 0) return
    if (stateData?.courses && stateData.courses.length > 0) {
      setSelected((prev) => {
        if (Object.values(prev).some((arr) => arr.length > 0)) return prev
        const initial: Record<number, ClassItem[]> = {}
        const ordered = [...stateData.courses].sort((a, b) => a.priority - b.priority)
        for (const c of ordered) {
          const item: ClassItem = { id: c.class_id, publish_id: c.publish_id, course_name: c.course_name } as ClassItem
          ;(initial[c.publish_id] ??= []).push(item)
        }
        return initial
      })
    }
  }, [stateData, rev])

  const publishes = data?.publishes ?? []

  const tabs = useMemo(
    () =>
      publishes.map((p) => ({
        ...p,
        label: p.publish_name || `发布 #${p.publish_id}`,
        open: p.in_date_range,
        tip: `可选 ${p.can_select} 门 · 已选 ${p.has_selected} 门 · 共 ${p.total_count} 门班次`,
      })),
    [publishes]
  )

  // 备选目标挑选：同发布下按点击顺序排优先级，取消后后续自动升级
  const pick = (publishId: number, classItem: ClassItem) => {
    setSelected((prev) => {
      const arr = [...(prev[publishId] ?? [])]
      const idx = arr.findIndex((c) => c.id === classItem.id)
      if (idx >= 0) {
        arr.splice(idx, 1)
        toast({
          title: "已取消目标",
          description:
            arr.length > 0
              ? `已移出【${classItem.course_name}】，后续备选自动升级（当前首选：${arr[0].course_name}）`
              : `已移出【${classItem.course_name}】，该发布已无预选目标`,
          variant: "default",
        })
        return { ...prev, [publishId]: arr }
      }
      toast({
        title: arr.length === 0 ? "已设为首选" : "已设为备选目标",
        description:
          arr.length === 0
            ? `【${classItem.course_name}】为首选，可继续添加同发布备选`
            : `已选中【${classItem.course_name}】为备选 ${arr.length}，可继续添加同发布备选`,
        variant: "default",
      })
      return { ...prev, [publishId]: [...arr, classItem] }
    })
    setRev((r) => r + 1) // 标记选课改动，触发自动保存防抖
  }

  // 自动保存：选课一变（仅用户点击），400ms 防抖后整包 PUT 到后端；成功静默，失败仅提示
  // MAJOR-H：保存串行化——飞行中的 PUT 完成后立即补发一次最新快照，绝不出现
  // "旧 PUT 后到覆盖新数据"的乱序丢失；内存 target 与后端最终一致。
  // C-1（第 3 轮）：body 必须包成后端 TargetsRequest 期望的 {"targets":[...]} 对象——
  // 此前发裸数组 100% 解码失败（后端 json 解码进 struct 直接报错），目标永远存不进库。
  const lastJson = useRef("")
  const targetRef = useRef<Target[]>([])
  const savingRef = useRef(false)
  const dirtyRef = useRef(false)
  // n14（第 3 轮）：失败重发状态——attempt 累计连续失败次数、timer 为退避重发定时器。
  // 成功或用户产生新改动都会清零；连续失败 5 次停手，等下一次改动重新驱动。
  const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })
  // 卸载清理：中断仍在排队的退避重发定时器，防止 onDone 返回后副作用残留
  useEffect(
    () => () => {
      if (retryState.current.timer) clearTimeout(retryState.current.timer)
    },
    []
  )
  const resetRetry = () => {
    if (retryState.current.timer) clearTimeout(retryState.current.timer)
    retryState.current.timer = null
    retryState.current.attempt = 0
  }
  const scheduleRetry = () => {
    const attempt = retryState.current.attempt
    if (attempt >= 5) return // 连续失败 5 次后停止自动重发（等用户改动触发新一轮）
    const delay = Math.min(2 ** attempt, 16) * 2000 // 指数退避：2s / 4s / 8s / 16s / 16s
    retryState.current.attempt = attempt + 1
    retryState.current.timer = setTimeout(() => {
      retryState.current.timer = null
      void saveNow()
    }, delay)
  }
  const saveNow = async () => {
    savingRef.current = true
    try {
      const targets = targetRef.current
      const json = JSON.stringify(targets)
      if (json === lastJson.current) return // 回显等非用户改动：跳过重复保存
      await api("/targets?account=" + encodeURIComponent(account), {
        method: "PUT",
        body: JSON.stringify({ targets }),
        session: sessionToken,
      })
      lastJson.current = json
      resetRetry() // 保存成功：清掉退避重发状态
    } catch (e: any) {
      toast({
        title: "目标保存失败",
        description: e.message || "通信异常，请重试",
        variant: "destructive",
      })
      //n14（第 3 轮）：失败保留 dirty（内存目标仍未持久化），并安排带退避的重发——
      //网络抖动/瞬时故障下不再退化为"尽力而为"，直至成功或用户新改动接管。
      dirtyRef.current = true
      scheduleRetry()
    } finally {
      savingRef.current = false
      // 保存期间用户又改了目标（且非失败重试态）：立即补发一次最新快照，
      // 避免旧 PUT 后到覆盖新数据。失败重发走上面的退避定时器，不在此紧循环。
      if (dirtyRef.current && retryState.current.attempt === 0) {
        dirtyRef.current = false
        void saveNow()
      }
    }
  }
  // 保存期间用户又改了目标：立即补发一次（带最新快照），避免旧 PUT 后到覆盖新数据。
  // F7-01（第 7 轮）：自动保存只由"用户改动"驱动——依赖只有 rev（用户点选自增），
  // publishes 改为经 ref 读取而非依赖。此前 publishes（轮询新对象引用）进依赖导致每次
  // 轮询都重跑 effect：窗口开启瞬间平台短暂清空 publishes → build() 产出 [] → 防抖 PUT
  // {"targets":[]} 把服务端/调度器内存目标整体抹除，黄金期 250ms 冲刺空转；
  // 窗口关闭后目标被永久抹除。发布列表收缩绝不等于用户意图清空目标。
  const publishesRef = useRef<readonly Publish[]>(publishes)
  publishesRef.current = publishes
  useEffect(() => {
    if (rev === 0) return
    // n14：用户新改动接管——中断失败重发退避，下一轮保存由正常防抖路径驱动
    resetRetry()
    const build = (): Target[] => {
      const targets: Target[] = []
      for (const p of publishesRef.current) {
        const list = selected[p.publish_id] ?? []
        list.forEach((cls, i) => {
          targets.push({
            publish_id: p.publish_id,
            class_id: cls.id,
            course_name: cls.course_name,
            priority: i,
          })
        })
      }
      return targets
    }
    const timer = setTimeout(async () => {
      targetRef.current = build()
      if (savingRef.current) {
        dirtyRef.current = true // 保存进行中：标记脏，完成后补发
        return
      }
      void saveNow()
    }, 400)
    return () => clearTimeout(timer)
  }, [rev, selected, sessionToken, toast])

  const selectedCount = Object.values(selected).reduce((n, arr) => n + arr.length, 0)

  // 选课开放时间倒计时（来自调度器状态，窗口 2026-09-13 09:00:00）
  const openTimeStr =
    stateData?.open_time && stateData.open_time !== "0001-01-01T00:00:00Z"
      ? stateData.open_time
      : null
  const cd = parseCountdown(openTimeStr)

  return (
    <div className="min-h-screen text-white p-4 sm:p-6 lg:p-8 select-none pb-28 sm:pb-24">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部纯黑白极简顶栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-neutral-900 pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <h1 className="text-lg sm:text-xl font-medium tracking-tight text-white">
                选修课程大厅
              </h1>
              <Badge variant="outline" className="text-[10px] font-mono uppercase">
                COURSES
              </Badge>
            </div>
            <p className="text-xs text-neutral-400">
              各批次预选课程配置与实时名额
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Badge variant="outline" className="text-xs py-1 px-3 font-mono">
              {account ? account : "默认"} · SELECTED {selectedCount}/{publishes.length}
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={onDone}
              className="flex items-center gap-1.5 text-xs text-neutral-400 hover:text-white"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>返回控制台</span>
            </Button>
          </div>
        </header>

        {/* 选课开放倒计时（人性化：一眼看清距开放还有多久） */}
        <div className="glass rounded-[var(--radius-lg)] border border-neutral-800 px-4 py-2.5 flex items-center justify-between gap-2 text-xs">
          <div className="flex items-center gap-2 text-neutral-400 min-w-0">
            <Clock className="h-3.5 w-3.5 text-neutral-500 shrink-0" />
            {/* N8：已开放状态以调度器 window_opened 为准（服务端有 ~640ms 校准偏差，
                本地倒计时到点 ≠ 平台开窗）；window_opened 才显示"已开放"高亮 */}
            {!stateData || !openTimeStr ? (
              <span>正在同步选课开放时间...</span>
            ) : stateData.window_opened || cd.isExpired ? (
              <span className="text-white font-medium flex items-center gap-1.5 whitespace-nowrap">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-white animate-pulse" />
                选课窗口已开放
              </span>
            ) : (
              <span className="truncate">
                距开放还有{" "}
                <span className="text-white font-mono tabular-nums">
                  {cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒
                </span>
              </span>
            )}
          </div>
          <span className="text-neutral-500 font-mono hidden sm:block shrink-0">
            {openTimeStr
              ? new Date(openTimeStr).toLocaleString("zh-CN", { hour12: false })
              : "SYNC"}
          </span>
        </div>

        {/* 搜索与条件过滤栏 */}
        <div className="flex flex-col sm:flex-row items-center gap-3 p-3 rounded-[var(--radius-lg)] glass border border-neutral-800">
          <div className="relative flex-1 w-full">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
            <Input
              placeholder="搜索课程名称、教师或教室"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10 text-xs sm:text-sm h-10 glass-input border-neutral-800 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
            />
          </div>

          <div className="flex items-center gap-2 w-full sm:w-auto">
            <Button
              variant={sortTightest ? "primary" : "outline"}
              size="sm"
              onClick={() => setSortTightest(!sortTightest)}
              className="flex items-center gap-1.5 text-xs whitespace-nowrap h-10 px-4 flex-1 sm:flex-none"
            >
              <ArrowDownWideNarrow className="h-3.5 w-3.5" />
              <span>{sortTightest ? "剩余名额正序" : "按剩余排序"}</span>
            </Button>

            <Button
              variant={onlyAvailable ? "primary" : "outline"}
              size="sm"
              onClick={() => setOnlyAvailable(!onlyAvailable)}
              className="flex items-center gap-1.5 text-xs whitespace-nowrap h-10 px-4 flex-1 sm:flex-none"
            >
              <Filter className="h-3.5 w-3.5" />
              <span>{onlyAvailable ? "仅看有余量" : "显示全部"}</span>
            </Button>
          </div>
        </div>

        {/* 加载中与错误反馈 */}
        {isLoading && (
          <div className="rounded-[var(--radius-lg)] glass border border-neutral-800 p-16 text-center text-xs text-neutral-400 flex flex-col items-center justify-center gap-3">
            <span className="inline-block w-2 h-2 rounded-full bg-white animate-ping" />
            <span>正在同步最新课程列表与名额...</span>
          </div>
        )}

        {isError && (
          <div className="rounded-[var(--radius-sm)] glass border border-neutral-800 p-4 text-center text-xs text-neutral-300">
            拉取课程数据异常: {(error as Error).message}
          </div>
        )}

        {/* 主选课 Tab 分段控制器 */}
        {!isLoading && !isError && tabs.length > 0 && (
          <Tabs defaultValue={String(tabs[0].publish_id)} className="space-y-4">
            <TabsList className="w-full sm:w-auto flex flex-wrap h-auto gap-1.5 p-1 glass border border-neutral-800 rounded-[var(--radius-lg)]">
              {tabs.map((t) => (
                <TabsTrigger
                  key={t.publish_id}
                  value={String(t.publish_id)}
                  className="flex items-center gap-2 py-2 px-4 text-xs sm:text-sm font-medium"
                >
                  <span className="font-semibold text-white">{t.label}</span>
                  <span
                    className={`inline-block w-2 h-2 rounded-full ${
                      t.open ? "bg-[var(--emerald)]" : "bg-[var(--fg-dim)]"
                    }`}
                  />
                  {(selected[t.publish_id] ?? []).length > 0 && (
                    <Badge variant="primary" className="text-[10px] px-1.5 py-0 ml-1">
                      已锁定 {(selected[t.publish_id] ?? []).length}
                    </Badge>
                  )}
                </TabsTrigger>
              ))}
            </TabsList>

            {tabs.map((t) => {
              let filteredClasses = t.classes.filter((c) => {
                const matchSearch =
                  !search ||
                  c.course_name.toLowerCase().includes(search.toLowerCase()) ||
                  (c.teacher_name_list &&
                    c.teacher_name_list.toLowerCase().includes(search.toLowerCase())) ||
                  (c.class_room_name &&
                    c.class_room_name.toLowerCase().includes(search.toLowerCase()))

                const matchAvailable = !onlyAvailable || c.selected_count < c.max_count
                return matchSearch && matchAvailable
              })

              // 剩余名额正序：名额越少越靠前，抢手课程一眼可见
              if (sortTightest) {
                filteredClasses = [...filteredClasses].sort(
                  (a, b) => a.selected_count - b.selected_count
                )
              }

              return (
                <TabsContent key={t.publish_id} value={String(t.publish_id)} className="space-y-4">
                  {/* 分类说明与概况 */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between text-xs text-neutral-400 p-3 rounded-[var(--radius-lg)] glass border border-neutral-800 gap-2 font-mono">
                    <span className="flex items-center gap-2">
                      <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
                      <span>{t.tip}</span>
                    </span>
                    <span className="text-neutral-500">
                      SHOWING {filteredClasses.length}/{t.classes.length}
                    </span>
                  </div>

                  {/* 课程卡片网格阵列（桌面 4 列更密集） */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                    {filteredClasses.map((c) => {
                      const selArr = selected[t.publish_id] ?? []
                      const selIdx = selArr.findIndex((x) => x.id === c.id)
                      const isSelected = selIdx >= 0
                      const rate = fillRate(c)
                      const isFull = c.selected_count >= c.max_count
                      const remaining = Math.max(0, c.max_count - c.selected_count)

                      // 进度条与徽章色彩分配
                      let progressColor: "emerald" | "amber" | "rose" | "cyan" = "emerald"
                      if (isFull) progressColor = "rose"
                      else if (remaining <= 5) progressColor = "amber"
                      else if (isSelected) progressColor = "cyan"

                      return (
                        <Card
                          key={c.id}
                          className={`relative rounded-[var(--radius-lg)] border transition-all duration-200 flex flex-col justify-between shadow-none ${
                            isSelected
                              ? "border-white bg-black/15"
                              : "bg-black/10 border-neutral-800/70 hover:border-neutral-600"
                          }`}
                        >
                          <CardContent className="p-3.5 flex flex-col gap-2.5">
                            {/* 顶部标签行 */}
                            <div className="flex items-center justify-between">
                              <span className="text-xs text-neutral-500 font-mono">
                                ID: {c.id}
                              </span>
                              {isSelected ? (
                                <Badge variant="primary" className="text-[11px] font-medium">
                                  {priorityName(selIdx)}
                                </Badge>
                              ) : isFull ? (
                                <Badge variant="outline" className="text-[11px] text-neutral-500 border-neutral-800">
                                  已满额
                                </Badge>
                              ) : remaining <= 5 ? (
                                <Badge variant="outline" className="text-[11px] text-neutral-400 border-neutral-700">
                                  余 {remaining} 席
                                </Badge>
                              ) : (
                                <Badge variant="outline" className="text-[11px] text-neutral-400 border-neutral-800">
                                  名额充足
                                </Badge>
                              )}
                            </div>

                            {/* 课程名称 */}
                            <div>
                              <h3 className="font-medium text-sm text-white tracking-tight line-clamp-1">
                                {c.course_name}
                              </h3>
                              {c.class_name && c.class_name !== c.course_name && (
                                <p className="text-[11px] text-neutral-500 truncate mt-0.5 font-mono">
                                  {c.class_name}
                                </p>
                              )}
                            </div>

                            {/* 地点与教师信息 */}
                            <div className="space-y-1 text-xs text-neutral-400 pt-2 border-t border-neutral-800">
                              <div className="flex items-center gap-2 truncate">
                                <User className="h-3.5 w-3.5 text-neutral-500 shrink-0" />
                                <span className="truncate">
                                  教师：{c.teacher_name_list || "待定"}
                                </span>
                              </div>
                              <div className="flex items-center gap-2 truncate">
                                <MapPin className="h-3.5 w-3.5 text-neutral-500 shrink-0" />
                                <span className="truncate">
                                  地点：{c.class_room_name || "待教室分配"}
                                </span>
                              </div>
                            </div>

                            {/* 容量统计 */}
                            <div className="space-y-1.5 pt-1.5">
                              <div className="flex items-center justify-between text-[11px]">
                                <span className="text-neutral-500 flex items-center gap-1 font-mono">
                                  <Users className="h-3 w-3" />
                                  <span>已报容量</span>
                                </span>
                                <span className="text-white font-mono tabular-nums">
                                  {c.selected_count} / {c.max_count} 人 ({rate}%)
                                </span>
                              </div>
                              <Progress
                                value={c.selected_count}
                                max={c.max_count || 1}
                                indicatorColor={progressColor}
                              />
                            </div>

                            {/* 操作按钮区 */}
                            <div className="pt-2 border-t border-neutral-800/70 mt-0.5 flex flex-col gap-1.5">
                              {/* 窗口开启后呈现官网同款【报名】或【退选】主操作按钮 */}
                              {t.in_date_range || stateData?.window_opened ? (
                                <>
                                  {c.btn_type === 1 ? (
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      disabled={actionLoading === c.id || !c.can_select}
                                      onClick={() => setExitModalClass(c)}
                                      title={c.title || (c.can_select ? "点击退选此课程" : "当前无法退选")}
                                      className="w-full flex items-center justify-center gap-1.5 text-xs h-8 border-red-500/40 text-red-400 hover:bg-red-500/10 hover:border-red-500/60 transition-colors"
                                    >
                                      <LogOut className="h-3.5 w-3.5" />
                                      <span>{actionLoading === c.id ? "退选中..." : (c.btn_text || "退选")}</span>
                                    </Button>
                                  ) : (
                                    <Button
                                      variant="primary"
                                      size="sm"
                                      disabled={actionLoading === c.id || !c.can_select}
                                      onClick={() => handleSelectClass(c)}
                                      title={c.title || (c.can_select ? "点击立即报名" : "不在选修报名时间范围内，无法选课！")}
                                      className="w-full flex items-center justify-center gap-1.5 text-xs h-8 disabled:opacity-40"
                                    >
                                      <Check className="h-3.5 w-3.5" />
                                      <span>{actionLoading === c.id ? "报名中..." : (c.btn_text || "报名")}</span>
                                    </Button>
                                  )}
                                  <Button
                                    variant={isSelected ? "outline" : "ghost"}
                                    size="sm"
                                    onClick={() => pick(t.publish_id, c)}
                                    className="w-full flex items-center justify-center gap-1 text-[11px] h-7 text-neutral-400 hover:text-white"
                                  >
                                    <BookMarked className="h-3 w-3" />
                                    <span>{isSelected ? `已设为后台冲刺${priorityName(selIdx)}` : "设为后台冲刺目标"}</span>
                                  </Button>
                                </>
                              ) : (
                                /* 窗口开启前：标准自动预选设置 */
                                <Button
                                  variant={isSelected ? "outline" : "primary"}
                                  size="sm"
                                  onClick={() => pick(t.publish_id, c)}
                                  className="w-full flex items-center justify-center gap-1.5 text-xs h-8"
                                >
                                  {isSelected ? (
                                    <>
                                      <Check className="h-3.5 w-3.5 text-white" />
                                      <span>{priorityName(selIdx)}</span>
                                    </>
                                  ) : (
                                    <>
                                      <BookMarked className="h-3.5 w-3.5" />
                                      <span>设为预选目标</span>
                                    </>
                                  )}
                                </Button>
                              )}
                            </div>
                          </CardContent>
                        </Card>
                      )
                    })}

                    {filteredClasses.length === 0 && (
                      <div className="col-span-full rounded-[var(--radius-lg)] border border-dashed border-neutral-700 bg-black/10 p-10 text-center text-xs text-neutral-400">
                        没有符合当前搜索或筛选条件的选修课程
                      </div>
                    )}
                  </div>
                </TabsContent>
              )
            })}
          </Tabs>
        )}

        {/* 选课改动自动保存，无需手动按钮；底部留白避免内容被遮挡 */}
        <div className="h-20 sm:h-16" aria-hidden />

        {/* 退选二次确认极简黑白 Modal (复刻官网 layer.confirm("确认退选该选修课?")) */}
        {/* F7-03（第 7 轮）：补对话语义——role=dialog/aria-modal/aria-labelledby，读屏可识别 */}
        {exitModalClass && (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-in fade-in duration-150"
            role="dialog"
            aria-modal="true"
            aria-labelledby="exit-modal-title"
          >
            <div className="relative w-full max-w-sm rounded-[var(--radius-lg)] border border-neutral-800 bg-[#09090b] p-5 shadow-2xl space-y-4">
              <div className="flex items-start gap-3">
                <div className="p-2 rounded-full bg-red-500/10 text-red-400 border border-red-500/20 shrink-0">
                  <AlertTriangle className="h-4 w-4" />
                </div>
                <div className="space-y-1">
                  <h3 id="exit-modal-title" className="text-sm font-medium text-white tracking-wide">确认退选该选修课？</h3>
                  <p className="text-xs text-neutral-400 leading-relaxed">
                    课程：<span className="text-white font-mono">{exitModalClass.course_name}</span>
                    <br />
                    退选后名额将被立即释放，您可以重新选择其他空余课程。
                  </p>
                </div>
              </div>
              <div className="flex items-center justify-end gap-2 pt-2 border-t border-neutral-900">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setExitModalClass(null)}
                  disabled={actionLoading === exitModalClass.id}
                  className="text-xs h-8"
                >
                  取消
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => handleConfirmExit(exitModalClass)}
                  disabled={actionLoading === exitModalClass.id}
                  className="text-xs h-8 bg-red-600 hover:bg-red-500 text-white border-none"
                >
                  {actionLoading === exitModalClass.id ? "退选中..." : "确认退选"}
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
