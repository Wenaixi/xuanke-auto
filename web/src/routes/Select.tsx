import { useMemo, useState, useEffect, useRef } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { Account, ClassItem, ClassDetail, ElectivesData, Target, SchedulerState } from "../types"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { Progress } from "../components/ui/Progress"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "../components/ui/Tabs"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "../components/ui/Dialog"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "../components/ui/Sheet"
import { useToast } from "../components/ui/Toast"
import {
  ArrowLeft,
  ArrowDownWideNarrow,
  BookMarked,
  Check,
  Clock,
  Filter,
  Info,
  MapPin,
  Search,
  User,
  Users,
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
  const { toast } = useToast()
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["electives", sessionToken],
    queryFn: () => api<ElectivesData>("/electives", { session: sessionToken }),
    refetchInterval: 10000,
  })

  // 查询当前调度器已保存的目标课程并自动回显（会话绑定当前账号）
  const { data: stateData } = useQuery({
    queryKey: ["state", account, sessionToken],
    queryFn: () => api<SchedulerState>("/state", { session: sessionToken }),
  })

  const [selected, setSelected] = useState<Record<number, ClassItem[]>>({})
  // 选课改动自增计数：驱动自动保存的 400ms 防抖；回显数据不经过它，故不会触发无意义保存
  const [rev, setRev] = useState(0)
  const [search, setSearch] = useState("")
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [sortTightest, setSortTightest] = useState(false)
  const [detailClass, setDetailClass] = useState<ClassItem | null>(null)
  const [detail, setDetail] = useState<ClassDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailErr, setDetailErr] = useState("")

  // 打开详情弹窗：先显示列表已知数据，同时向后端拉取 classDetail 详情
  const openDetail = (c: ClassItem) => {
    setDetailClass(c)
    setDetail(null)
    setDetailErr("")
    setDetailLoading(true)
    api<ClassDetail>(`/electives/detail?id=${c.id}`, { session: sessionToken })
      .then(setDetail)
      .catch((e: any) => setDetailErr(e.message || "详情加载失败"))
      .finally(() => setDetailLoading(false))
  }

  // 详情值统一回退：弹窗内容优先用后端详情，缺字段用列表已知值
  const dv = (k: keyof ClassDetail): string | number =>
    detail ? (detail[k] as string | number) : ""

  // 本地每秒刷新倒计时，确保数字秒级平滑跳动
  const [, setTick] = useState(0)
  useEffect(() => {
    const timer = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(timer)
  }, [])

  // 进入页面时自动回显已保存的目标课程（含多备选优先级）
  useEffect(() => {
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
  }, [stateData])

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
  const lastJson = useRef("")
  useEffect(() => {
    if (rev === 0) return
    const build = (): Target[] => {
      const targets: Target[] = []
      for (const p of publishes) {
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
      const targets = build()
      const json = JSON.stringify(targets)
      if (json === lastJson.current) return // 回显等非用户改动：跳过重复保存
      try {
        await api("/targets", {
          method: "PUT",
          body: json,
          session: sessionToken,
        })
        lastJson.current = json
      } catch (e: any) {
        toast({
          title: "目标保存失败",
          description: e.message || "通信异常，请重试",
          variant: "destructive",
        })
      }
    }, 400)
    return () => clearTimeout(timer)
  }, [rev, selected, publishes, sessionToken, toast])

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
            {!stateData || !openTimeStr ? (
              <span>正在同步选课开放时间...</span>
            ) : cd.isExpired ? (
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
                  <span>{t.label}</span>
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
                              ? "border-white glass-strong"
                              : "glass border-neutral-800 hover:border-neutral-600"
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
                            <div className="grid grid-cols-2 gap-2 pt-2 border-t border-neutral-800 mt-0.5">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => openDetail(c)}
                                className="flex items-center justify-center gap-1.5 text-xs h-8 text-neutral-400 hover:text-white hover:border-white/60"
                              >
                                <Info className="h-3.5 w-3.5" />
                                <span>详情</span>
                              </Button>

                              <Button
                                variant={isSelected ? "outline" : "primary"}
                                size="sm"
                                onClick={() => pick(t.publish_id, c)}
                                className="flex items-center justify-center gap-1.5 text-xs h-8"
                              >
                                {isSelected ? (
                                  <>
                                    <Check className="h-3.5 w-3.5 text-white" />
                                    <span>{priorityName(selIdx)}</span>
                                  </>
                                ) : (
                                  <>
                                    <BookMarked className="h-3.5 w-3.5" />
                                    <span>设为目标</span>
                                  </>
                                )}
                              </Button>
                            </div>
                          </CardContent>
                        </Card>
                      )
                    })}

                    {filteredClasses.length === 0 && (
                      <div className="col-span-full rounded-[var(--radius-lg)] border border-dashed border-neutral-700 glass p-10 text-center text-xs text-neutral-400">
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

        {/* 📱 手机端专用抽屉详情 (Bottom Sheet)：先从列表已知值渲染，详情接口返回后热替换 */}
        <div className="sm:hidden">
          <Sheet open={!!detailClass} onOpenChange={(open) => !open && setDetailClass(null)}>
            {detailClass && (
              <SheetContent side="bottom">
                <SheetHeader>
                  <div className="flex items-center gap-2">
                    <Badge variant="primary" className="text-[10px]">
                      课程编号 #{detailClass.id}
                    </Badge>
                    <span className="text-xs text-[var(--fg-dim)]">班次详情</span>
                  </div>
                  <SheetTitle className="text-lg font-bold text-[var(--fg)] pt-1">
                    {detailClass.course_name}
                  </SheetTitle>
                  <SheetDescription>
                    {detail?.class_name || detailClass.class_name || "标准选修班级"}
                  </SheetDescription>
                </SheetHeader>

                {detailLoading && (
                  <div className="flex items-center justify-center gap-2 py-8 text-xs text-[var(--fg-dim)]">
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-white animate-ping" />
                    正在同步课程详情...
                  </div>
                )}

                {detailErr && !detailLoading && (
                  <div className="py-6 text-center text-xs text-[var(--fg-muted)] space-y-2">
                    <p>详情加载失败：{detailErr}</p>
                    <p className="text-[var(--fg-dim)]">以下为列表同步的已知信息</p>
                  </div>
                )}

                <div className="space-y-3 text-xs divide-y divide-[var(--border)] py-3">
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">任课教师</span>
                    <span className="text-[var(--fg)] font-medium">
                      {String(dv("teacher_name") || detailClass.teacher_name_list || "暂无教师信息")}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课教室</span>
                    <span className="text-[var(--fg)] font-medium">
                      {String(dv("classroom_name") || detailClass.class_room_name || "待教室分配")}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课课节</span>
                    <span className="text-[var(--fg)]">{String(dv("lessons_date") || detailClass.lessons_date || "课表编排中")}</span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">课程状态</span>
                    <span className="text-[var(--fg)] font-medium">{String(dv("class_status_str") || detailClass.btn_text || "未开始")}</span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">计划容量比</span>
                    <span className="text-[var(--fg)] font-semibold tabular-nums">
                      {detail?.audited_count ?? detailClass.selected_count} /{" "}
                      {detail?.plan_count ?? detailClass.plan_count ?? detailClass.max_count} 人
                    </span>
                  </div>
                  {detail && (
                    <>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">学年学期</span>
                        <span className="text-[var(--fg)]">{String(dv("school_year_term") || "—")}</span>
                      </div>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">选修类型</span>
                        <span className="text-[var(--fg)]">{String(dv("course_type_name") || "—")}</span>
                      </div>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">教学方式</span>
                        <span className="text-[var(--fg)]">{String(dv("method_name") || "—")}</span>
                      </div>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">评价方式</span>
                        <span className="text-[var(--fg)]">{String(dv("evaluate_type_name") || "—")}</span>
                      </div>
                    </>
                  )}
                  {detailClass.title && (
                    <div className="py-2.5 text-xs text-[var(--fg-muted)] leading-relaxed bg-[var(--surface)] p-3 rounded-[var(--radius-md)]">
                      说明：{detailClass.title}
                    </div>
                  )}
                </div>

                <div className="pt-3">
                  <Button
                    variant="outline"
                    size="default"
                    onClick={() => setDetailClass(null)}
                    className="w-full h-11 text-xs font-medium"
                  >
                    关闭详情
                  </Button>
                </div>
              </SheetContent>
            )}
          </Sheet>
        </div>

        {/* 💻 PC 电脑端专用模态弹窗 (Dialog)：详情接口返回后热替换，失败仍展示列表已知信息 */}
        <div className="hidden sm:block">
          <Dialog open={!!detailClass} onOpenChange={(open) => !open && setDetailClass(null)}>
            {detailClass && (
              <DialogContent className="max-w-md rounded-[var(--radius-xl)]">
                <DialogHeader>
                  <div className="flex items-center gap-2">
                    <Badge variant="primary" className="text-[10px]">
                      ID: {detailClass.id}
                    </Badge>
                    <span className="text-xs text-[var(--fg-dim)]">课程详细参数</span>
                  </div>
                  <DialogTitle className="text-lg font-bold pt-1">
                    {detailClass.course_name}
                  </DialogTitle>
                  <DialogDescription>
                    {detail?.class_name || detailClass.class_name || "标准选修班级"}
                  </DialogDescription>
                </DialogHeader>

                {detailLoading && (
                  <div className="flex items-center justify-center gap-2 py-8 text-xs text-[var(--fg-dim)]">
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-white animate-ping" />
                    正在同步课程详情...
                  </div>
                )}

                {detailErr && !detailLoading && (
                  <div className="py-6 text-center text-xs text-[var(--fg-muted)] space-y-2">
                    <p>详情加载失败：{detailErr}</p>
                    <p className="text-[var(--fg-dim)]">以下为列表同步的已知信息</p>
                  </div>
                )}

                <div className="space-y-3 text-xs divide-y divide-[var(--border)] py-2">
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">任课教师</span>
                    <span className="text-[var(--fg)] font-medium">
                      {String(dv("teacher_name") || detailClass.teacher_name_list || "暂无教师信息")}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课教室</span>
                    <span className="text-[var(--fg)] font-medium">
                      {String(dv("classroom_name") || detailClass.class_room_name || "待教室分配")}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课课节</span>
                    <span className="text-[var(--fg)]">{String(dv("lessons_date") || detailClass.lessons_date || "课表编排中")}</span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">课程状态</span>
                    <span className="text-[var(--fg)] font-medium">{String(dv("class_status_str") || detailClass.btn_text || "未开始")}</span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">计划容量比</span>
                    <span className="text-[var(--fg)] font-semibold tabular-nums">
                      {detail?.audited_count ?? detailClass.selected_count} /{" "}
                      {detail?.plan_count ?? detailClass.plan_count ?? detailClass.max_count} 人
                    </span>
                  </div>
                  {detail && (
                    <>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">学年学期</span>
                        <span className="text-[var(--fg)]">{String(dv("school_year_term") || "—")}</span>
                      </div>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">选修类型</span>
                        <span className="text-[var(--fg)]">{String(dv("course_type_name") || "—")}</span>
                      </div>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">教学方式</span>
                        <span className="text-[var(--fg)]">{String(dv("method_name") || "—")}</span>
                      </div>
                      <div className="flex items-center justify-between py-2">
                        <span className="text-[var(--fg-dim)]">评价方式</span>
                        <span className="text-[var(--fg)]">{String(dv("evaluate_type_name") || "—")}</span>
                      </div>
                    </>
                  )}
                  {detailClass.title && (
                    <div className="py-2.5 text-xs text-[var(--fg-muted)] leading-relaxed bg-[var(--surface-soft)] p-3 rounded-[var(--radius-md)]">
                      说明：{detailClass.title}
                    </div>
                  )}
                </div>

                <div className="pt-3 flex justify-end">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setDetailClass(null)}
                    className="text-xs font-medium px-4"
                  >
                    关闭
                  </Button>
                </div>
              </DialogContent>
            )}
          </Dialog>
        </div>
      </div>
    </div>
  )
}
