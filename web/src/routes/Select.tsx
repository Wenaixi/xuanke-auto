import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import * as Tabs from '@radix-ui/react-tabs'
import { api } from '../api/client'
import type { ElectivesData, Target } from '../types'
import { Button } from '../components/ui/Button'

interface Props {
  onDone: () => void
}

// 课程表格（单列）
function CourseTable({
  classes,
  selectedId,
  onSelect,
}: {
  classes: ElectivesData['publishes'][number]['classes']
  selectedId: number | null
  onSelect: (id: number) => void
}) {
  return (
    <div className="flex flex-col border">
      <div className="grid grid-cols-[1fr_auto] gap-2 px-3 py-2 text-xs text-[var(--fg-dim)] border-b">
        <span>课程</span>
        <span>人数</span>
      </div>
      {classes.map((c) => {
        const active = selectedId === c.id
        return (
          <button
            key={c.id}
            type="button"
            onClick={() => onSelect(c.id)}
            className={
              'px-3 py-2 border-b text-left flex flex-col gap-1 ' +
              (active ? 'bg-[var(--fg)] text-[var(--bg)]' : 'hover:bg-[var(--bg-soft)]')
            }
          >
            <span className="flex items-baseline gap-2">
              <span className="font-medium">{c.course_name}</span>
              <span className="text-xs opacity-70">{c.class_name}</span>
            </span>
            <span className="flex items-baseline gap-2 text-xs opacity-70">
              <span>{c.teacher_name_list}</span>
              <span>{c.class_room_name}</span>
              <span className="ml-auto">
                {c.selected_count}/{c.max_count}人
              </span>
            </span>
          </button>
        )
      })}
    </div>
  )
}

export default function Select({ onDone }: Props) {
  const { data: data } = useQuery({
    queryKey: ['electives'],
    queryFn: () => api<ElectivesData>('/electives'),
    refetchInterval: 10000,
  })
  const [selected, setSelected] = useState<Record<number, number>>({})
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  const publishes = data?.publishes ?? []

  const tabs = useMemo(
    () => publishes.map((p) => ({ ...p, label: p.publish_name.replace(/^.*高二年/, '') || ('发布 ' + p.publish_id) })),
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
      setError('请至少选择一门课程')
      return
    }
    setSaving(true)
    setError('')
    try {
      await api('/targets', { method: 'PUT', body: JSON.stringify({ targets }) })
      setSaved(true)
    } catch (e: any) {
      setError(e.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const selectedCount = Object.keys(selected).length

  return (
    <div className="max-w-5xl mx-auto p-6 flex flex-col gap-6">
      <header className="flex items-center justify-between border-b pb-4">
        <h1 className="text-xl tracking-[0.3em]">选择目标课程</h1>
        <div className="flex gap-3">
          <Button onClick={onDone}>返回面板</Button>
        </div>
      </header>

      {tabs.length === 0 && (
        <div className="border p-8 text-center text-[var(--fg-dim)]">课程数据加载中…</div>
      )}

      <Tabs.Root defaultValue={tabs[0]?.publish_id?.toString() || ''}>
        <Tabs.List className="flex border-b">
          {tabs.map((t) => (
            <Tabs.Trigger
              key={t.publish_id}
              value={String(t.publish_id)}
              className="px-4 py-2 text-sm border-r data-[state=active]:bg-[var(--fg)] data-[state=active]:text-[var(--bg)]"
            >
              {t.label}
            </Tabs.Trigger>
          ))}
        </Tabs.List>
        {tabs.map((t) => (
          <Tabs.Content key={t.publish_id} value={String(t.publish_id)} className="py-4">
            <CourseTable
              classes={t.classes}
              selectedId={selected[t.publish_id] ?? null}
              onSelect={pick(t.publish_id)}
            />
          </Tabs.Content>
        ))}
      </Tabs.Root>

      <footer className="border-t pt-4 flex items-center gap-4">
        <span className="text-sm text-[var(--fg-dim)]">已选 {selectedCount} 门</span>
        {saved && <span className="text-sm text-[var(--ok)]">已保存</span>}
        {error && <span className="text-sm text-[var(--danger)]">{error}</span>}
        <Button onClick={save} disabled={saving || selectedCount === 0} className="ml-auto">
          {saving ? '保存中…' : '保存目标'}
        </Button>
      </footer>
    </div>
  )
}
