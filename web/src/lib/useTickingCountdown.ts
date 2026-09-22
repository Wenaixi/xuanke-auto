import { useEffect, useState } from "react"

// useTickingCountdown 选课开放倒计时（Dashboard/Select 收敛共用）：
// 内部自 tick（每秒 setNow）。注意：hook 在路由组件顶层被消费，每秒 setNow
// 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），
// DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，
// 非当前承诺——本注释已按实现如实口径，不再声称局部渲染）。
// - target 为开窗时刻字符串（null 表示未同步到/已清空，直接视为过期）
// - 过期（diff<=0）返回全 00 + isExpired=true；数字带前导零（等宽雕刻感）
export function useTickingCountdown(target: string | null) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])
  // 目标变化时立刻校正 now——此前 diff 用上一拍的 now 计算，
  // target 从 null 变为有效开窗时刻（首次同步完成 / 窗口开启瞬间）时最多有 1 秒
  // 陈旧偏差，可能短暂误显为过期全 00；target 一变即回到当前时刻，注释意图落实。
  useEffect(() => {
    setNow(Date.now())
  }, [target])
  // 目标变化时回到 "now"（首次挂载 / 窗口开启瞬间）；过期即全 00
  const diff = target ? new Date(target).getTime() - now : 0
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