import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import * as Tabs from "@radix-ui/react-tabs"
import { api } from "../api/client"
import type { ClassItem, ElectivesData, Target } from "../types"
import { Button } from "../components/ui/Button"

interface Props {
  onDone: () => void
}

// 人数占比进度条
function fillRate(c: ClassItem): number {
  if (!c.max_count) return 0
  return Math.min(100, Math.round((c.selected_count / c.max_count) * 100))
}

// 课程卡片
function CourseCard({
  c,
  active,
  onClick,
}: {
  c: ClassItem
  active: boolean
  onClick: () => void
}) {
  const rate = fillRate(c)
  const selectable = c.can_select
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "px-4 py-3 border-b text-left flex flex-col gap-2 w-full " +
        (active ? "bg-[var(--fg)] text-[var(--bg)]" : "hover:bg-[var(--bg-soft)]")
      }
    >
      {/* 第一行：课程名 + 报名状态 */}
      <span className="flex items-baseline gap-2">
        <span className="font-medium">{c.course_name}</span>
        {c.class_name && c.class_name !== c.course_name && (
          <span className="text-xs opacity-70">{c.class_name}</span>
        )}
        <span
          className="ml-auto text-xs px-2 py-0.5 border whitespace-nowrap"
          style={{
            borderColor: selectable ? "var(--ok)" : "var(--fg-dim)",
            color: selectable ? "var(--ok)" : "var(--fg-dim)"
          }}
        >
          {selectable ? "可报名" : "不可报名"}
        </span>
      </span>

      {/* 第二行：老师 / 地点 / 课节 */}
      <span className="flex flex-wrap items-baseline gap-x-4 gap-y-1 text-xs opacity-80">
        {c.teacher_name_list && <span>老师：{c.teacher_name_list}</span>}
        {c.class_room_name && <span>地点：{c.class_room_name}</span>}
        {c.lessons_date && <span>课节：{c.lessons_date}</span>}
      </span>

      {/* 第三行：报名时间 + 人数 */}
      <span className="flex items-center gap-4 text-xs">
        <span className="opacity-70">报名：{c.apply_date}</span>
        <span className="opacity-70">
          {c.selected_count}/{c.max_count} 人
        </span>
        {/* 人数进度条 */}
        <span className="flex-1 h-1 bg-[var(--border)] overflow-hidden">
          <span
            className="block h-full transition-all"
            style={{
              width: rate + "%",
              background: rate >= 90 ? "var(--danger)" : rate >= 70 ? "var(--warn)" : "var(--ok)",
            }}
          />
        </span>
      </span>

      {/* 不可报名原因 */}
      {!selectable && c.title && (
        <span className="text-xs opacity-60">{c.title}</span>
      )}
    </button>
  )
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

  const publishes = data?.publishes ?? []

  const tabs = useMemo(
    () =>
      publishes.map((p) => ({
        ...p,
        label: (p.publish_name.match(/高二年(.+)$/) || [])[1] || "发布 " + p.publish_id,
        open: p.in_date_range,
        tip: `可选 ${p.can_select} 门，已选 ${p.has_selected} 门，共 ${p.total_count} 门`,
      })),
    [publishes],
  )

  const pick = (publishId: number) => (classId: number) => {
    setSelected((prev) => ({ ...prev, [publishId]: classId }))
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
      setErrorMsg("请至少选择一门课程")
      return
    }
    setSaving(true)
    setErrorMsg("")
    try {
      await api("/targets", { method: "PUT", body: JSON.stringify({ targets }) })
      setSaved(true)
    } catch (e: any) {
      setErrorMsg(e.message || "保存失败")
    } finally {
      setSaving(false)
    }
  }

  const selectedCount = Object.keys(selected).length

  return (
    <div className="max-w-5xl mx-auto p-6 flex flex-col gap-6">
      <header className="flex items-center justify-between border-b pb-4">
        <div>
          <h1 className="text-xl tracking-[0.3em]">选择目标课程</h1>
          <p className="text-xs text-[var(--fg-dim)] mt-1">
            每个发布最多选 1 门，保存后抢课窗口开启时将自动报名
          </p>
        </div>
        <Button onClick={onDone}>返回面板</Button>
      </header>

      {isLoading && (
        <div className="border p-12 text-center text-[var(--fg-dim)]">课程数据加载中…</div>
      )}
      {isError && (
        <div className="border p-12 text-center text-[var(--danger)]">
          课程数据加载失败：{(error as Error).message}
        </div>
      )}
      {!isLoading && !isError && tabs.length === 0 && (
        <div className="border p-12 text-center text-[var(--fg-dim)]">暂无可选的选修课程</div>
      )}

      {tabs.length > 0 && (
        <Tabs.Root defaultValue={String(tabs[0].publish_id)}>
          <Tabs.List className="flex border-b">
            {tabs.map((t) => (
              <Tabs.Trigger
                key={t.publish_id}
                value={String(t.publish_id)}
                className="px-4 py-2 text-sm border-r data-[state=active]:bg-[var(--fg)] data-[state=active]:text-[var(--bg)] flex items-center gap-2"
              >
                {t.label}
                <span
                  className="inline-block w-1.5 h-1.5"
                  style={{ background: t.open ? "var(--ok)" : "var(--fg-dim)" }}
                />
              </Tabs.Trigger>
            ))}
          </Tabs.List>
          {tabs.map((t) => (
            <Tabs.Content key={t.publish_id} value={String(t.publish_id)} className="py-4">
              <p className="text-xs text-[var(--fg-dim)] mb-3">{t.tip}</p>
              <div className="flex flex-col border">
                {t.classes.map((c) => (
                  <CourseCard
                    key={c.id}
                    c={c}
                    active={selected[t.publish_id] === c.id}
                    onClick={() => pick(t.publish_id)(c.id)}
                  />
                ))}
              </div>
            </Tabs.Content>
          ))}
        </Tabs.Root>
      )}

      <footer className="border-t pt-4 flex items-center gap-4">
        <span className="text-sm text-[var(--fg-dim)]">已选 {selectedCount} 门</span>
        {saved && <span className="text-sm text-[var(--ok)]">已保存，抢课将自动执行</span>}
        {errorMsg && <span className="text-sm text-[var(--danger)]">{errorMsg}</span>}
        <Button
          onClick={save}
          disabled={saving || selectedCount === 0}
          className="ml-auto"
        >
          {saving ? "保存中…" : "保存目标"}
        </Button>
      </footer>
    </div>
  )
}