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
  onDone: () => void
}

function fillRate(c: ClassItem): number {
  if (!c.max_count) return 0
  return Math.min(100, Math.round((c.selected_count / c.max_count) * 100))
}

export default function Select({ onDone }: Props) {
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

  const pick = (publishId: number, classId: number) => {
    setSelected((prev) => {
      // 允许点击已选取的项目进行反选取消
      if (prev[publishId] === classId) {
        const next = { ...prev }
        delete next[publishId]
        return next
      }
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
      setErrorMsg("请至少选择一门预选目标课程")
      return
    }
    setSaving(true)
    setErrorMsg("")
    try {
      await api("/targets", { method: "PUT", body: JSON.stringify({ targets }) })
      setSaved(true)
    } catch (e: any) {
      setErrorMsg(e.message || "目标保存失败，请检查网络通信")
    } finally {
      setSaving(false)
    }
  }

  const selectedCount = Object.keys(selected).length

  return (
    <div className="min-h-screen bg-[var(--bg)] text-[var(--fg)] p-4 sm:p-8 select-none">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部标题与返回控制台栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[var(--border)] pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <span className="inline-block w-2 h-2 bg-white" />
              <h1 className="text-lg font-mono tracking-[0.25em] uppercase text-[var(--fg)]">
                SELECT // TARGET COURSES
              </h1>
              <Badge variant="outline" className="text-[9px]">
                PRE-SELECTION
              </Badge>
            </div>
            <p className="text-xs text-[var(--fg-dim)] font-mono tracking-wide">
              每个发布批次选定 1 门目标课程 · 保存后开放窗口自动并发秒级抢报
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Badge variant="secondary" className="text-xs py-1 px-3">
              已选定目标: {selectedCount} / {publishes.length}
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={onDone}
              className="flex items-center gap-1.5 text-xs text-[var(--fg-dim)] hover:text-white"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>返回控制台</span>
            </Button>
          </div>
        </header>

        {/* 快速搜索与条件过滤工具栏 */}
        <div className="flex flex-col sm:flex-row items-center gap-3 p-3 bg-[var(--surface)] border border-[var(--border)]">
          <div className="relative flex-1 w-full">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-[var(--fg-dim)]" />
            <Input
              placeholder="快速搜索课程名称、授课教师、上课地点..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 font-mono text-xs h-9 bg-[var(--surface-soft)]"
            />
          </div>

          <Button
            variant={onlyAvailable ? "default" : "outline"}
            size="sm"
            onClick={() => setOnlyAvailable(!onlyAvailable)}
            className="flex items-center gap-1.5 text-xs whitespace-nowrap h-9 w-full sm:w-auto"
          >
            <Filter className="h-3.5 w-3.5" />
            <span>仅看有余量 ({onlyAvailable ? "已开启" : "全部"})</span>
          </Button>
        </div>

        {/* 加载中与错误反馈 */}
        {isLoading && (
          <div className="border border-[var(--border)] bg-[var(--surface)] p-16 text-center font-mono text-xs text-[var(--fg-dim)]">
            &gt; 正在拉取平台选课数据与实时名额...
          </div>
        )}

        {isError && (
          <div className="border border-white bg-black p-6 text-center font-mono text-xs text-white">
            拉取课程数据异常: {(error as Error).message}
          </div>
        )}

        {/* 主选课 Tab 分段控制器 */}
        {!isLoading && !isError && tabs.length > 0 && (
          <Tabs defaultValue={String(tabs[0].publish_id)} className="space-y-4">
            <TabsList className="w-full sm:w-auto flex flex-wrap h-auto gap-1 p-1">
              {tabs.map((t) => (
                <TabsTrigger
                  key={t.publish_id}
                  value={String(t.publish_id)}
                  className="flex items-center gap-2 py-2 px-4 text-xs font-mono"
                >
                  <span>{t.label}</span>
                  <span
                    className={`inline-block w-1.5 h-1.5 ${
                      t.open ? "bg-white" : "bg-[var(--fg-dim)]"
                    }`}
                  />
                  {selected[t.publish_id] && (
                    <Badge variant="default" className="text-[8px] px-1 py-0 ml-1">
                      已锁定
                    </Badge>
                  )}
                </TabsTrigger>
              ))}
            </TabsList>

            {tabs.map((t) => {
              // 筛选逻辑：按搜索词与仅看有剩余名额
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
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between text-xs font-mono text-[var(--fg-dim)] p-3 border border-[var(--border)] bg-[var(--surface-soft)]">
                    <span>{t.tip}</span>
                    <span>
                      当前显示: {filteredClasses.length} / {t.classes.length} 门
                    </span>
                  </div>

                  {/* 课程卡片网格阵列 */}
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {filteredClasses.map((c) => {
                      const isSelected = selected[t.publish_id] === c.id
                      const rate = fillRate(c)
                      const isFull = c.selected_count >= c.max_count

                      return (
                        <Card
                          key={c.id}
                          className={`relative border transition-all duration-150 flex flex-col justify-between ${
                            isSelected
                              ? "border-white bg-black shadow-lg"
                              : "border-[var(--border)] bg-[var(--surface)] hover:border-[var(--border-strong)]"
                          }`}
                        >
                          <CardContent className="p-5 flex flex-col gap-3">
                            {/* 顶部标签行 */}
                            <div className="flex items-center justify-between">
                              <span className="text-[10px] font-mono text-[var(--fg-dim)]">
                                CODE: {c.id}
                              </span>
                              <Badge
                                variant={isSelected ? "default" : isFull ? "secondary" : "outline"}
                                className="text-[9px]"
                              >
                                {isSelected
                                  ? "TARGET LOCKED"
                                  : isFull
                                  ? "CAPACITY FULL"
                                  : c.can_select
                                  ? "SELECTABLE"
                                  : "NOT OPEN"}
                              </Badge>
                            </div>

                            {/* 课程名称 */}
                            <div>
                              <h3 className="font-medium text-sm text-[var(--fg)] tracking-wide line-clamp-1">
                                {c.course_name}
                              </h3>
                              {c.class_name && c.class_name !== c.course_name && (
                                <p className="text-xs font-mono text-[var(--fg-dim)] truncate mt-0.5">
                                  {c.class_name}
                                </p>
                              )}
                            </div>

                            {/* 地点与教师信息 */}
                            <div className="space-y-1 text-xs font-mono text-[var(--fg-muted)] pt-2 border-t border-[var(--border)]">
                              <div className="flex items-center gap-2 truncate">
                                <User className="h-3 w-3 text-[var(--fg-dim)] shrink-0" />
                                <span className="truncate">
                                  {c.teacher_name_list || "待定任课教师"}
                                </span>
                              </div>
                              <div className="flex items-center gap-2 truncate">
                                <MapPin className="h-3 w-3 text-[var(--fg-dim)] shrink-0" />
                                <span className="truncate">
                                  {c.class_room_name || "待定授课地点"}
                                </span>
                              </div>
                            </div>

                            {/* 容量统计与几何进度条 */}
                            <div className="space-y-1.5 pt-2">
                              <div className="flex items-center justify-between text-xs font-mono">
                                <span className="text-[var(--fg-dim)] flex items-center gap-1">
                                  <Users className="h-3 w-3" />
                                  <span>已报容量</span>
                                </span>
                                <span className="text-[var(--fg)] tabular-nums">
                                  {c.selected_count} / {c.max_count} 人 ({rate}%)
                                </span>
                              </div>
                              <Progress value={c.selected_count} max={c.max_count || 1} />
                            </div>

                            {/* 操作按钮区 */}
                            <div className="grid grid-cols-2 gap-2 pt-2 border-t border-[var(--border)] mt-1">
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setDetailClass(c)}
                                className="flex items-center justify-center gap-1 text-[11px] h-8"
                              >
                                <Info className="h-3 w-3" />
                                <span>详情</span>
                              </Button>

                              <Button
                                variant={isSelected ? "default" : "invert"}
                                size="sm"
                                onClick={() => pick(t.publish_id, c.id)}
                                className="flex items-center justify-center gap-1 text-[11px] h-8"
                              >
                                {isSelected ? (
                                  <>
                                    <Check className="h-3 w-3" />
                                    <span>已锁定</span>
                                  </>
                                ) : (
                                  <>
                                    <BookMarked className="h-3 w-3" />
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
                      <div className="col-span-full border border-[var(--border)] border-dashed bg-[var(--surface)] p-12 text-center font-mono text-xs text-[var(--fg-dim)]">
                        没有符合筛选条件的选修课程
                      </div>
                    )}
                  </div>
                </TabsContent>
              )
            })}
          </Tabs>
        )}

        {/* 底部吸底保存工具栏 */}
        <footer className="sticky bottom-4 z-40 p-4 border border-white bg-black shadow-2xl flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3 text-xs font-mono">
            <span className="text-white font-medium">
              当前目标阵容: {selectedCount} / {publishes.length} 门
            </span>
            {saved && (
              <span className="text-white flex items-center gap-1">
                <CheckCircle className="h-3.5 w-3.5" />
                <span>目标已保存至调度引擎，开网将毫秒抢报</span>
              </span>
            )}
            {errorMsg && <span className="text-white bg-red-950 px-2 py-0.5">{errorMsg}</span>}
          </div>

          <Button
            variant="invert"
            size="default"
            onClick={save}
            disabled={saving || selectedCount === 0}
            className="w-full sm:w-auto flex items-center justify-center gap-2 text-xs"
          >
            <Save className="h-3.5 w-3.5" />
            <span>{saving ? "正在向引擎保存目标..." : "保存预选目标"}</span>
          </Button>
        </footer>

        {/* 课程详情画册级模态弹窗 */}
        <Dialog open={!!detailClass} onOpenChange={(open) => !open && setDetailClass(null)}>
          {detailClass && (
            <DialogContent className="max-w-md">
              <DialogHeader>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-[9px]">
                    ID: {detailClass.id}
                  </Badge>
                  <span className="text-[10px] font-mono text-[var(--fg-dim)]">CLASS DETAIL</span>
                </div>
                <DialogTitle className="text-base pt-1 font-mono tracking-wider">
                  {detailClass.course_name}
                </DialogTitle>
                <DialogDescription>
                  {detailClass.class_name || "标准选修班级"}
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-3 font-mono text-xs divide-y divide-[var(--border)] py-2">
                <div className="flex items-center justify-between py-1.5">
                  <span className="text-[var(--fg-dim)]">任课教师</span>
                  <span className="text-[var(--fg)]">
                    {detailClass.teacher_name_list || "暂无教师信息"}
                  </span>
                </div>
                <div className="flex items-center justify-between py-1.5">
                  <span className="text-[var(--fg-dim)]">上课地点</span>
                  <span className="text-[var(--fg)]">
                    {detailClass.class_room_name || "待教室分配"}
                  </span>
                </div>
                <div className="flex items-center justify-between py-1.5">
                  <span className="text-[var(--fg-dim)]">课节时间</span>
                  <span className="text-[var(--fg)]">{detailClass.lessons_date || "课表编排中"}</span>
                </div>
                <div className="flex items-center justify-between py-1.5">
                  <span className="text-[var(--fg-dim)]">报名时间区间</span>
                  <span className="text-[var(--fg)]">{detailClass.apply_date || "跟随发布周期"}</span>
                </div>
                <div className="flex items-center justify-between py-1.5">
                  <span className="text-[var(--fg-dim)]">计划容量比</span>
                  <span className="text-[var(--fg)] tabular-nums">
                    {detailClass.selected_count} / {detailClass.max_count} 人 (上限{" "}
                    {detailClass.plan_count || detailClass.max_count})
                  </span>
                </div>
                {detailClass.title && (
                  <div className="py-2 text-[11px] text-[var(--fg-muted)] leading-relaxed">
                    说明: {detailClass.title}
                  </div>
                )}
              </div>

              <div className="pt-2 flex justify-end">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setDetailClass(null)}
                  className="text-xs font-mono"
                >
                  关闭
                </Button>
              </div>
            </DialogContent>
          )}
        </Dialog>
      </div>
    </div>
  )
}
