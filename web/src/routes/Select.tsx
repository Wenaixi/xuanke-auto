import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { ClassItem, ElectivesData, Target } from "../types"
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
  Sparkles,
  User,
  Users,
  Flame,
} from "lucide-react"

interface Props {
  onDone: () => void
}

function fillRate(c: ClassItem): number {
  if (!c.max_count) return 0
  return Math.min(100, Math.round((c.selected_count / c.max_count) * 100))
}

export default function Select({ onDone }: Props) {
  const { toast } = useToast()
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["electives"],
    queryFn: () => api<ElectivesData>("/electives"),
    refetchInterval: 10000,
  })

  const [selected, setSelected] = useState<Record<number, number>>({})
  const [saving, setSaving] = useState(false)
  const [errorMsg, setErrorMsg] = useState("")
  const [saved, setSaved] = useState(false)
  const [search, setSearch] = useState("")
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [detailClass, setDetailClass] = useState<ClassItem | null>(null)

  const publishes = data?.publishes ?? []

  const tabs = useMemo(
    () =>
      publishes.map((p) => ({
        ...p,
        label: (p.publish_name.match(/高二年(.+)$/) || [])[1] || `发布 #${p.publish_id}`,
        open: p.in_date_range,
        tip: `可选 ${p.can_select} 门 · 已选 ${p.has_selected} 门 · 共 ${p.total_count} 门班次`,
      })),
    [publishes]
  )

  const pick = (publishId: number, classId: number, courseName: string) => {
    setSelected((prev) => {
      if (prev[publishId] === classId) {
        const next = { ...prev }
        delete next[publishId]
        toast({
          title: "已取消目标课程",
          description: `已将【${courseName}】移出预选队列`,
          variant: "default",
        })
        return next
      }
      toast({
        title: "已锁定预选目标！",
        description: `已成功选择【${courseName}】，记得点击底栏保存哦喵~`,
        variant: "success",
      })
      return { ...prev, [publishId]: classId }
    })
    setSaved(false)
  }

  const save = async () => {
    const targets: Target[] = []
    for (const p of publishes) {
      const cid = selected[p.publish_id]
      if (!cid) continue
      const cls = p.classes.find((c) => c.id === cid)
      if (!cls) continue
      targets.push({ publish_id: p.publish_id, class_id: cid, course_name: cls.course_name })
    }
    if (targets.length === 0) {
      setErrorMsg("请至少挑选一门预选目标课程喵~")
      toast({
        title: "提示",
        description: "请至少勾选一门您心仪的课程后再保存",
        variant: "warning",
      })
      return
    }
    setSaving(true)
    setErrorMsg("")
    try {
      await api("/targets", { method: "PUT", body: JSON.stringify({ targets }) })
      setSaved(true)
      toast({
        title: "🎉 预选目标保存成功！",
        description: `已锁定 ${targets.length} 门目标，选课窗口开放时系统将以 300ms 极速自动抢报！`,
        variant: "success",
      })
    } catch (e: any) {
      setErrorMsg(e.message || "目标保存失败，请检查网络通信")
      toast({
        title: "保存遇到问题",
        description: e.message || "通信异常，请检查后端运行状态",
        variant: "destructive",
      })
    } finally {
      setSaving(false)
    }
  }

  const selectedCount = Object.keys(selected).length

  return (
    <div className="min-h-screen bg-[var(--bg)] text-[var(--fg)] p-4 sm:p-6 lg:p-8 select-none pb-28 sm:pb-24">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部标题与返回按钮 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[var(--border)] pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <Sparkles className="h-5 w-5 text-[var(--cyan)]" />
              <h1 className="text-lg sm:text-xl font-bold tracking-tight text-[var(--fg)]">
                选修课程精选大厅
              </h1>
              <Badge variant="primary" className="text-[10px]">
                课程配置
              </Badge>
            </div>
            <p className="text-xs sm:text-sm text-[var(--fg-muted)] leading-relaxed">
              每个发布批次锁定 1 门心仪目标 · 保存后开放窗口自动并发秒级抢报
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Badge variant="success" className="text-xs py-1 px-3">
              已选定目标：{selectedCount} / {publishes.length} 门
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={onDone}
              className="flex items-center gap-1.5 text-xs text-[var(--fg-muted)] hover:text-white"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>返回控制看板</span>
            </Button>
          </div>
        </header>

        {/* 搜索与条件过滤工具栏 */}
        <div className="flex flex-col sm:flex-row items-center gap-3 p-3.5 rounded-[var(--radius-lg)] bg-[var(--surface)] border border-[var(--border)] shadow-sm">
          <div className="relative flex-1 w-full">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-[var(--fg-dim)]" />
            <Input
              placeholder="快速搜索课程名称、授课教师、上课教室..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10 text-xs sm:text-sm h-10 bg-[var(--surface-soft)] border-[var(--border)] focus:border-[var(--cyan)]"
            />
          </div>

          <Button
            variant={onlyAvailable ? "primary" : "outline"}
            size="sm"
            onClick={() => setOnlyAvailable(!onlyAvailable)}
            className="flex items-center gap-1.5 text-xs whitespace-nowrap h-10 px-4 w-full sm:w-auto font-medium"
          >
            <Filter className="h-3.5 w-3.5" />
            <span>仅看有余量 ({onlyAvailable ? "已开启" : "全部"})</span>
          </Button>
        </div>

        {/* 加载中与错误反馈 */}
        {isLoading && (
          <div className="rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] p-16 text-center text-sm text-[var(--fg-muted)] flex flex-col items-center justify-center gap-3">
            <span className="inline-block w-3 h-3 rounded-full bg-[var(--cyan)] animate-ping" />
            <span>正在为您同步教务平台最新选修课程与实时名额...</span>
          </div>
        )}

        {isError && (
          <div className="rounded-[var(--radius-lg)] border border-[var(--rose-border)] bg-[var(--rose-bg)] p-6 text-center text-sm text-[var(--rose)]">
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
                  {selected[t.publish_id] && (
                    <Badge variant="primary" className="text-[10px] px-1.5 py-0 ml-1">
                      已锁定
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
                  {/* 分类说明与容量概况 */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between text-xs text-[var(--fg-muted)] p-3.5 rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface-soft)] gap-2">
                    <span className="flex items-center gap-2">
                      <Sparkles className="h-3.5 w-3.5 text-[var(--cyan)]" />
                      <span>{t.tip}</span>
                    </span>
                    <span className="text-[var(--fg-dim)]">
                      当前显示: {filteredClasses.length} / {t.classes.length} 门班次
                    </span>
                  </div>

                  {/* 课程卡片网格阵列 */}
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {filteredClasses.map((c) => {
                      const isSelected = selected[t.publish_id] === c.id
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
                          className={`relative rounded-[var(--radius-lg)] border transition-all duration-200 flex flex-col justify-between shadow-sm ${
                            isSelected
                              ? "border-[var(--cyan-border)] bg-[var(--surface)] shadow-[0_0_20px_rgba(14,165,233,0.15)] ring-1 ring-[var(--cyan)]"
                              : "border-[var(--border)] bg-[var(--surface)] hover:border-[var(--border-hover)]"
                          }`}
                        >
                          <CardContent className="p-4 sm:p-5 flex flex-col gap-3">
                            {/* 顶部标签行 */}
                            <div className="flex items-center justify-between">
                              <span className="text-xs text-[var(--fg-dim)] font-mono">
                                ID: {c.id}
                              </span>
                              {isSelected ? (
                                <Badge variant="primary" className="text-[11px] font-semibold">
                                  ✨ 已设为目标
                                </Badge>
                              ) : isFull ? (
                                <Badge variant="destructive" className="text-[11px]">
                                  🔒 已满额
                                </Badge>
                              ) : remaining <= 5 ? (
                                <Badge variant="warning" className="text-[11px]">
                                  <Flame className="h-3 w-3 mr-0.5" />
                                  仅剩 {remaining} 席
                                </Badge>
                              ) : (
                                <Badge variant="success" className="text-[11px]">
                                  🌿 名额充裕
                                </Badge>
                              )}
                            </div>

                            {/* 课程名称 */}
                            <div>
                              <h3 className="font-semibold text-base text-[var(--fg)] tracking-tight line-clamp-1">
                                {c.course_name}
                              </h3>
                              {c.class_name && c.class_name !== c.course_name && (
                                <p className="text-xs text-[var(--fg-dim)] truncate mt-0.5">
                                  {c.class_name}
                                </p>
                              )}
                            </div>

                            {/* 地点与教师信息 */}
                            <div className="space-y-1.5 text-xs text-[var(--fg-muted)] pt-2.5 border-t border-[var(--border)]">
                              <div className="flex items-center gap-2 truncate">
                                <User className="h-3.5 w-3.5 text-[var(--fg-dim)] shrink-0" />
                                <span className="truncate">
                                  教师：{c.teacher_name_list || "待定"}
                                </span>
                              </div>
                              <div className="flex items-center gap-2 truncate">
                                <MapPin className="h-3.5 w-3.5 text-[var(--fg-dim)] shrink-0" />
                                <span className="truncate">
                                  地点：{c.class_room_name || "待教室分配"}
                                </span>
                              </div>
                            </div>

                            {/* 容量统计与胶囊进度条 */}
                            <div className="space-y-1.5 pt-2">
                              <div className="flex items-center justify-between text-xs">
                                <span className="text-[var(--fg-dim)] flex items-center gap-1">
                                  <Users className="h-3 w-3" />
                                  <span>已报容量</span>
                                </span>
                                <span className="text-[var(--fg)] font-medium tabular-nums">
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
                            <div className="grid grid-cols-2 gap-2 pt-2.5 border-t border-[var(--border)] mt-1">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setDetailClass(c)}
                                className="flex items-center justify-center gap-1.5 text-xs h-9"
                              >
                                <Info className="h-3.5 w-3.5" />
                                <span>详情</span>
                              </Button>

                              <Button
                                variant={isSelected ? "default" : "primary"}
                                size="sm"
                                onClick={() => pick(t.publish_id, c.id, c.course_name)}
                                className="flex items-center justify-center gap-1.5 text-xs h-9 font-semibold"
                              >
                                {isSelected ? (
                                  <>
                                    <Check className="h-3.5 w-3.5 text-[var(--cyan)]" />
                                    <span>已锁定</span>
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

        {/* 底部吸底保存工具栏（PC 电脑与手机端自适应悬浮岛） */}
        <footer className="fixed bottom-4 inset-x-4 max-w-6xl mx-auto z-40 p-4 rounded-[var(--radius-xl)] border border-[var(--border-hover)] bg-[var(--surface-soft)]/95 backdrop-blur-md shadow-2xl flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3 text-xs">
            <span className="text-[var(--fg)] font-semibold">
              当前目标阵容: {selectedCount} / {publishes.length} 门
            </span>
            {saved && (
              <span className="text-[var(--emerald)] flex items-center gap-1 font-medium">
                <CheckCircle className="h-4 w-4" />
                <span>目标已保存在调度引擎，开网将极速抢报</span>
              </span>
            )}
            {errorMsg && <span className="text-[var(--rose)] font-medium">{errorMsg}</span>}
          </div>

          <Button
            variant="primary"
            size="default"
            onClick={save}
            disabled={saving || selectedCount === 0}
            className="w-full sm:w-auto h-10 px-6 flex items-center justify-center gap-2 text-xs sm:text-sm font-semibold shadow-lg"
          >
            <Save className="h-4 w-4" />
            <span>{saving ? "正在同步至抢课引擎..." : "保存预选目标阵容"}</span>
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
