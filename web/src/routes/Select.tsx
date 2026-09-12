import { useMemo, useState, useEffect } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { Account, ClassItem, ElectivesData, Target, SchedulerState } from "../types"
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
  BookMarked,
  Check,
  CheckCircle,
  Filter,
  Info,
  MapPin,
  Save,
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
  const [saving, setSaving] = useState(false)
  const [errorMsg, setErrorMsg] = useState("")
  const [saved, setSaved] = useState(false)
  const [search, setSearch] = useState("")
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [detailClass, setDetailClass] = useState<ClassItem | null>(null)

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
    setSaved(false)
  }

  const save = async () => {
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

    setSaving(true)
    setErrorMsg("")
    try {
      await api("/targets", {
        method: "PUT",
        body: JSON.stringify({ targets }),
        session: sessionToken,
      })
      setSaved(true)
      toast({
        title: targets.length > 0 ? "目标保存成功" : "目标已清空",
        description: targets.length > 0 ? `已锁定 ${publishes.length} 个发布的多备选目标` : "已清空所有预选目标",
        variant: "default",
      })
      onDone() // 保存成功直接返回控制台
    } catch (e: any) {
      setErrorMsg(e.message || "目标保存失败，请检查网络通信")
      toast({
        title: "保存失败",
        description: e.message || "通信异常",
        variant: "destructive",
      })
    } finally {
      setSaving(false)
    }
  }

  const selectedCount = Object.values(selected).reduce((n, arr) => n + arr.length, 0)

  return (
    <div className="min-h-screen bg-black text-white p-4 sm:p-6 lg:p-8 select-none pb-28 sm:pb-24">
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

        {/* 搜索与条件过滤栏 */}
        <div className="flex flex-col sm:flex-row items-center gap-3 p-3 rounded-[var(--radius-lg)] bg-[#09090b] border border-neutral-900">
          <div className="relative flex-1 w-full">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
            <Input
              placeholder="搜索课程名称、教师或教室"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10 text-xs sm:text-sm h-10 bg-neutral-950 border-neutral-800 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
            />
          </div>

          <Button
            variant={onlyAvailable ? "primary" : "outline"}
            size="sm"
            onClick={() => setOnlyAvailable(!onlyAvailable)}
            className="flex items-center gap-1.5 text-xs whitespace-nowrap h-10 px-4 w-full sm:w-auto"
          >
            <Filter className="h-3.5 w-3.5" />
            <span>{onlyAvailable ? "仅看有余量" : "显示全部"}</span>
          </Button>
        </div>

        {/* 加载中与错误反馈 */}
        {isLoading && (
          <div className="rounded-[var(--radius-lg)] border border-neutral-900 bg-[#09090b] p-16 text-center text-xs text-neutral-400 flex flex-col items-center justify-center gap-3">
            <span className="inline-block w-2 h-2 rounded-full bg-white animate-ping" />
            <span>正在同步最新课程列表与名额...</span>
          </div>
        )}

        {isError && (
          <div className="rounded-[var(--radius-sm)] border border-neutral-800 bg-neutral-950 p-4 text-center text-xs text-neutral-300">
            拉取课程数据异常: {(error as Error).message}
          </div>
        )}

        {/* 主选课 Tab 分段控制器 */}
        {!isLoading && !isError && tabs.length > 0 && (
          <Tabs defaultValue={String(tabs[0].publish_id)} className="space-y-4">
            <TabsList className="w-full sm:w-auto flex flex-wrap h-auto gap-1.5 p-1 bg-[var(--surface)] border border-[var(--border)] rounded-[var(--radius-lg)]">
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
              const filteredClasses = t.classes.filter((c) => {
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

              return (
                <TabsContent key={t.publish_id} value={String(t.publish_id)} className="space-y-4">
                  {/* 分类说明与概况 (纯黑白极简) */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between text-xs text-neutral-400 p-3 rounded-[var(--radius-lg)] border border-neutral-900 bg-[#09090b] gap-2 font-mono">
                    <span className="flex items-center gap-2">
                      <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
                      <span>{t.tip}</span>
                    </span>
                    <span className="text-neutral-500">
                      SHOWING {filteredClasses.length}/{t.classes.length}
                    </span>
                  </div>

                  {/* 课程卡片网格阵列 */}
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
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
                              ? "border-white bg-[#111114]"
                              : "border-neutral-900 bg-[#09090b] hover:border-neutral-800"
                          }`}
                        >
                          <CardContent className="p-4 sm:p-5 flex flex-col gap-3">
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
                              <h3 className="font-medium text-base text-white tracking-tight line-clamp-1">
                                {c.course_name}
                              </h3>
                              {c.class_name && c.class_name !== c.course_name && (
                                <p className="text-xs text-neutral-500 truncate mt-0.5 font-mono">
                                  {c.class_name}
                                </p>
                              )}
                            </div>

                            {/* 地点与教师信息 */}
                            <div className="space-y-1.5 text-xs text-neutral-400 pt-2.5 border-t border-neutral-900">
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
                            <div className="space-y-1.5 pt-2">
                              <div className="flex items-center justify-between text-xs">
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
                            <div className="grid grid-cols-2 gap-2 pt-2.5 border-t border-neutral-900 mt-1">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setDetailClass(c)}
                                className="flex items-center justify-center gap-1.5 text-xs h-9 text-neutral-400 hover:text-white"
                              >
                                <Info className="h-3.5 w-3.5" />
                                <span>详情</span>
                              </Button>

                              <Button
                                variant={isSelected ? "outline" : "primary"}
                                size="sm"
                                onClick={() => pick(t.publish_id, c)}
                                className="flex items-center justify-center gap-1.5 text-xs h-9"
                              >
                                {isSelected ? (
                                  <>
                                    <Check className="h-3.5 w-3.5 text-white" />
                                    <span>{priorityName(selIdx)}</span>
                                  </>
                                ) : (
                                  <>
                                    <BookMarked className="h-3.5 w-3.5 text-black" />
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
                      <div className="col-span-full rounded-[var(--radius-lg)] border border-[var(--border)] border-dashed bg-[var(--surface)] p-12 text-center text-xs text-[var(--fg-dim)]">
                        没有符合当前搜索或筛选条件的选修课程
                      </div>
                    )}
                  </div>
                </TabsContent>
              )
            })}
          </Tabs>
        )}

        {/* 底部吸底保存工具栏 (纯黑白极简艺术) */}
        <footer className="fixed bottom-4 inset-x-4 max-w-6xl mx-auto z-40 p-4 rounded-[var(--radius-lg)] border border-neutral-800 bg-neutral-950/95 backdrop-blur-md shadow-none flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3 text-xs font-mono">
            <span className="text-white font-medium">
              已选备选: {selectedCount} 门 · 发布 {Object.keys(selected).length} 个
            </span>
            {saved && (
              <span className="text-white flex items-center gap-1 text-[11px]">
                <CheckCircle className="h-3.5 w-3.5 text-white" />
                <span>已保存预选</span>
              </span>
            )}
            {errorMsg && <span className="text-neutral-400">{errorMsg}</span>}
          </div>

          <Button
            variant="primary"
            size="default"
            onClick={save}
            disabled={saving}
            className="w-full sm:w-auto h-10 px-6 flex items-center justify-center gap-2 text-xs font-medium"
          >
            <Save className="h-3.5 w-3.5 text-black" />
            <span>{saving ? "正在保存..." : "保存预选课程"}</span>
          </Button>
        </footer>

        {/* 📱 手机端专用抽屉详情 (Bottom Sheet) */}
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
                    {detailClass.class_name || "标准选修班级"}
                  </SheetDescription>
                </SheetHeader>

                <div className="space-y-3 text-xs divide-y divide-[var(--border)] py-3">
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">任课教师</span>
                    <span className="text-[var(--fg)] font-medium">
                      {detailClass.teacher_name_list || "暂无教师信息"}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课教室</span>
                    <span className="text-[var(--fg)] font-medium">
                      {detailClass.class_room_name || "待教室分配"}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课课节</span>
                    <span className="text-[var(--fg)]">{detailClass.lessons_date || "课表编排中"}</span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">计划容量比</span>
                    <span className="text-[var(--fg)] font-semibold tabular-nums">
                      {detailClass.selected_count} / {detailClass.max_count} 人 (计划上限{" "}
                      {detailClass.plan_count || detailClass.max_count})
                    </span>
                  </div>
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

        {/* 💻 PC 电脑端专用模态弹窗 (Dialog) */}
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
                    {detailClass.class_name || "标准选修班级"}
                  </DialogDescription>
                </DialogHeader>

                <div className="space-y-3 text-xs divide-y divide-[var(--border)] py-2">
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">任课教师</span>
                    <span className="text-[var(--fg)] font-medium">
                      {detailClass.teacher_name_list || "暂无教师信息"}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课教室</span>
                    <span className="text-[var(--fg)] font-medium">
                      {detailClass.class_room_name || "待教室分配"}
                    </span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">上课课节</span>
                    <span className="text-[var(--fg)]">{detailClass.lessons_date || "课表编排中"}</span>
                  </div>
                  <div className="flex items-center justify-between py-2">
                    <span className="text-[var(--fg-dim)]">计划容量比</span>
                    <span className="text-[var(--fg)] font-semibold tabular-nums">
                      {detailClass.selected_count} / {detailClass.max_count} 人 (上限{" "}
                      {detailClass.plan_count || detailClass.max_count})
                    </span>
                  </div>
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
