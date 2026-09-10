import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import type { LogEntry, SchedulerState } from '../types'
import { Button } from '../components/ui/Button'

interface Props {
  onLogout: () => void
  onGoSelect: () => void
}

const statusText: Record<string, { label: string; color: string }> = {
  pending: { label: '等待窗口', color: 'var(--fg-dim)' },
  in_range: { label: '窗口已开', color: 'var(--warn)' },
  submitted: { label: '提交中', color: 'var(--warn)' },
  success: { label: '已报名', color: 'var(--ok)' },
  failed: { label: '失败', color: 'var(--danger)' },
}

function countdown(target: string): string {
  const t = new Date(target).getTime()
  const diff = t - Date.now()
  if (diff <= 0) return '已到开放时间'
  const s = Math.floor(diff / 1000)
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return `${d}天 ${h}时 ${m}分 ${sec}秒`
}

export default function Dashboard({ onLogout, onGoSelect }: Props) {
  const { data: state, isError: stateErr } = useQuery({
    queryKey: ['state'],
    queryFn: () => api<SchedulerState>('/state'),
    refetchInterval: 3000,
  })
  const { data: logs } = useQuery({
    queryKey: ['logs'],
    queryFn: () => api<LogEntry[]>('/logs'),
    refetchInterval: 3000,
  })

  return (
    <div className="max-w-4xl mx-auto p-6 flex flex-col gap-6">
      <header className="flex items-center justify-between border-b pb-4">
        <h1 className="text-xl tracking-[0.3em]">至道选课自动化</h1>
        <div className="flex gap-3">
          <Button onClick={onGoSelect}>选择课程</Button>
          <Button onClick={onLogout}>退出登录</Button>
        </div>
      </header>

      {/* 窗口状态 */}
      <section className="border p-4 flex flex-col gap-2">
        <div className="flex items-center gap-3">
          <span
            className="inline-block w-2 h-2"
            style={{ background: state?.window_opened ? 'var(--ok)' : 'var(--fg-dim)' }}
          />
          <span className="text-sm">
            选课窗口：{state?.window_opened ? '已开放' : '未开放'}
          </span>
          {state?.open_time && state.open_time !== '0001-01-01T00:00:00Z' && (
            <span className="text-sm text-[var(--fg-dim)] ml-auto">
              开放时间 {new Date(state.open_time).toLocaleString('zh-CN', { hour12: false })}
            </span>
          )}
        </div>
        {state?.open_time && state.open_time !== '0001-01-01T00:00:00Z' && !state.window_opened && (
          <div className="text-sm text-[var(--warn)]">距离开放还有 {countdown(state.open_time)}</div>
        )}
        {stateErr && <div className="text-sm text-[var(--danger)]">状态查询失败，请检查后端</div>}
      </section>

      {/* 三课程卡片 */}
      <section className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {(state?.courses?.length ? state.courses : []).map((c) => {
          const st = statusText[c.status] || { label: c.status, color: 'var(--fg-dim)' }
          return (
            <div key={c.class_id} className="border p-4 flex flex-col gap-2">
              <div className="text-sm text-[var(--fg-dim)]">发布 {c.publish_id}</div>
              <div className="font-medium">{c.course_name || ('课程 ' + c.class_id)}</div>
              <div className="text-sm" style={{ color: st.color }}>
                {st.label}
              </div>
              {c.result && <div className="text-xs text-[var(--fg-dim)] break-all">{c.result}</div>}
            </div>
          )
        })}
        {(!state?.courses || state.courses.length === 0) && (
          <div className="border p-4 text-sm text-[var(--fg-dim)] col-span-full">
            尚未设置目标课程
          </div>
        )}
      </section>

      {/* 日志 */}
      <section className="border p-4 flex flex-col gap-2">
        <h2 className="text-sm tracking-widest text-[var(--fg-dim)]">报名日志</h2>
        {logs?.length ? (
          <ul className="flex flex-col gap-1 text-sm">
            {logs.map((l) => (
              <li key={l.id} className="flex gap-3 items-baseline border-b border-[var(--border)]/40 pb-1">
                <span className="text-xs text-[var(--fg-dim)]">{l.created_at}</span>
                <span style={{ color: l.is_ok ? 'var(--ok)' : 'var(--danger)' }}>
                  {l.is_ok ? '成功' : '失败'}
                </span>
                <span className="text-xs text-[var(--fg-dim)]">{l.action}</span>
                <span className="flex-1 truncate">{l.result}</span>
              </li>
            ))}
          </ul>
        ) : (
          <div className="text-sm text-[var(--fg-dim)]">暂无日志</div>
        )}
      </section>
    </div>
  )
}
