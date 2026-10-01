// 轮询间隔收敛纯函数：把多处 refetchInterval 回调里"错误/窗口关闭 → 降频、开窗 → 升频"
// 的判据收成单点（契约 54 窄版——信号解析留在调用方，本函数只做「信号 → 间隔」决策）。
// 语义：error > windowClosed > open > idle 优先级链；idle 缺席时稳态落 far。
// 三个调用方具休档位各自不同（Select electives 30000/2000/10000、Select state 30000/2000、
// Dashboard state/logs 30000/3000），由消费点传 tiers 表达——本函数不预设任何站点事实。
export type PollSignals = { error: boolean | Error | null; windowClosed: boolean; open: boolean }
export type PollTiers = { far: number; near: number; idle?: number }

export function pickInterval(s: PollSignals, t: PollTiers): number {
  if (s.error) return t.far
  if (s.windowClosed) return t.far
  if (s.open) return t.near
  return t.idle ?? t.far
}
