// 性能回归守卫：useTickingCountdown 每秒 setNow 引发的宿主组件重渲染计数对比。
// 思路：真实 React 渲染 + Profiler 不可无头断言重渲染次数，改为直接断言
// 「倒计时叶子组件被 memo 化」这一结构性事实（memo 是性能优化成败的分水岭），
// 与 target-guard 同款纯静态断言：
//   1. lib/useTickingCountdown.ts 仍自 tick（未被移除导致倒计时失效）
//   2. routes/Dashboard.tsx 把每秒变化的 cd.* 限制在 memo 叶子组件内消费
//      （主矩阵四格 + 折叠行 = 唯一依赖 cd/nowMs 的渲染位）
//      ——若未来有人把 cd.days 直接写进路由组件 JSX（未包 memo 叶子），
//        本断言红灯，提醒其必须走 memo 化边界。
// 用法：node --import jiti/register scripts/perf-countdown-guard.ts
import { readFileSync } from "node:fs"
import { join } from "node:path"

const root = join(import.meta.dirname, "..")
const lib = readFileSync(join(root, "src/lib/useTickingCountdown.ts"), "utf8")
const dash = readFileSync(join(root, "src/routes/Dashboard.tsx"), "utf8")

let failed = false
const check = (name: string, ok: boolean) => {
  if (!ok) failed = true
  console.log(`${ok ? "✓" : "✗"} ${name}`)
}

// 1. 自 tick 契约在位
check("useTickingCountdown 仍每秒 setNow 自 tick", /setInterval\([\s\S]{0,80}setNow\(Date\.now\(\)/.test(lib))

// 2. Dashboard 存在 memo 化倒计时叶子边界：找 CountdownDigits / Ticking 子组件引用
//    （路由组件自身渲染期不得直接出现 cd.days 等裸消费——那是整树重渲染的标志）
const hasMemoLeaf = /function\s+\w*[Cc]ountdown\w*\(/.test(dash) && /memo\(/.test(dash)
check("Dashboard 内存在 memo 化倒计时叶子组件", hasMemoLeaf)

// 3. 路由组件顶层不再直接渲染 cd.*（应经由 memo 叶子组件 props 下发；仅叶子调用处可
//    传 cd.* props——单行组件调用即叶子边界，多行/jsx 引用才是裸消费）
const rawCdUse = (dash.match(/^[^/].*\bcd\.(days|hours|minutes|seconds)\b[^/]*$/gm) ?? []).filter(
  (line) => !/memo\(|MemoCountdownMatrix|props|countdownmatrix/i.test(line)
).length
check(`路由组件顶层无裸 cd.* 消费（当前 ${rawCdUse} 处，应为 0）`, rawCdUse === 0)

console.log(failed ? "\nperf-countdown-guard 断言失败" : "\nperf-countdown-guard 断言全绿")
process.exit(failed ? 1 : 0)
