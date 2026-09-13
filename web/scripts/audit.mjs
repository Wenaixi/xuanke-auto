// 视觉一致性回归护栏（TDD 守护）：
//   A. 水墨画布：<img> 本体 + 独立遮罩，全端共用同一条 --bg-shift 滚动冻结逻辑（图片永不越出屏幕底，用户的明确要求）
//   B. 全站 UI 表面须走统一半透明工具类（glass-overlay / glass-input）
//   C. 禁止遗留实心不透明黑洞：bg-black/25|30|70|75、bg-neutral-950、整页 bg-black
// 用法：node scripts/audit.mjs （退出码非 0 即存在违例）
import { readFileSync, readdirSync, statSync } from "node:fs"
import { fileURLToPath } from "node:url"
import path from "node:path"

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../src")
let failed = 0

const ok = (msg) => console.log("  ✓ " + msg)
const bad = (msg) => {
  console.error("  ✖ " + msg)
  failed++
}

// 收集 src 下全部 .tsx 与 .css 源码文本
const sources = []
const walk = (dir) => {
  for (const e of readdirSync(dir)) {
    const p = path.join(dir, e)
    if (statSync(p).isDirectory()) walk(p)
    else if (/\.(tsx|css)$/.test(e)) sources.push([p, readFileSync(p, "utf8")])
  }
}
walk(srcRoot)
const css = sources.filter(([p]) => p.endsWith(".css")).map(([, t]) => t).join("\n")
const all = sources.map(([, t]) => t).join("\n")

// ---------- A. 画布背景规格 ----------
console.log("A. 画布背景：<img> 本体 + 独立遮罩，全端共用同一条 --bg-shift 滚动冻结逻辑（移动 180% / 桌面 160% 仅宽度不同）")
assertFile(css.includes("width: 180%"), "canvas-bg-img 移动端图片层放大至 180%")
assertFile(css.includes("width: 160%"), "canvas-bg-img 桌面端图片层放大至 160%")
assertFile(css.includes("transform: translateY(var(--bg-shift, 0px))"), "canvas-bg-img 全端消费 --bg-shift 位移（未触底 1:1 跟随，触底冻结不越出底部）")
assertFile(css.includes(".canvas-bg-mask"), "canvas-bg-mask 深黑渐变遮罩存在且独立于图片位移")
assertFile(css.includes(".canvas-bg-img"), "canvas-bg-img 使用真实 <img>（便于量测真实渲染高度）")

// ---------- B. 统一半透明工具 ----------
console.log("B. 统一半透明工具类已就位")
assertFile(css.includes(".glass-overlay"), "存在 .glass-overlay（弹窗遮罩统一）")
assertFile(css.includes(".glass-input"), "存在 .glass-input（输入框/下拉统一）")

// ---------- C. 实心不透明表面清零 ----------
console.log("C. 实心不透明黑洞清零")
for (const [file, text] of sources) {
  const rel = path.relative(srcRoot, file).replace(/\\/g, "/")
  for (const cls of ["bg-black/25", "bg-black/30", "bg-black/70", "bg-black/75"]) {
    assertFile(!text.includes(cls), `${rel} 不含 ${cls}`)
  }
  assertFile(!text.includes("min-h-screen bg-black"), `${rel} 无整页 bg-black（不遮挡画布）`)
  if (text.includes("bg-neutral-950") || text.includes("bg-neutral-900")) {
    // 允许出现在 <option> 行（原生下拉列表无法毛玻璃，必须实色兜底），其余位置视为黑窟窿
    for (const line of text.split("\n"))
      if ((line.includes("bg-neutral-950") || line.includes("bg-neutral-900")) && !line.trim().startsWith("<option")) {
        bad(`${rel} 含 bg-neutral-95x 实色：${line.trim().slice(0, 60)}`)
      }
  }
}

console.log(failed === 0 ? "\n全部通过，视觉表面协调一致。" : `\n发现 ${failed} 处违例。`)
process.exit(failed === 0 ? 0 : 1)

function assertFile(cond, msg) {
  if (cond) ok(msg)
  else bad(msg)
}