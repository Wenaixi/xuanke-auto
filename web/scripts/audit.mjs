// 视觉一致性回归护栏（TDD 守护）：
//   A. 实色分层：画布 / 面板 / 输入档必须是不透明实色，禁止回退半透明玻璃
//   B. 统一表面工具类（glass / glass-strong / glass-input / glass-overlay）
//   C. 禁止遗留实心不透明黑洞：bg-black/25|30|70|75、bg-neutral-950、整页 bg-black
//   D. 形态两轨制：按钮胶囊、面板直角，rounded-full 只允许出现在白名单内
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

// ---------- A. 实色分层（背景图机制已于 xAI 重构中整体移除） ----------
console.log("A. 实色分层：面板与输入表面走不透明实色，不再回退半透明玻璃")
assertFile(!/canvas-bg|bg\.jpg|bg-shift/.test(all), "全站无背景图机制残留（canvas-bg / bg.jpg / --bg-shift）")
assertFile(/--bg:\s*#0a0a0a/.test(css), "--bg 为 xAI 画布实色 #0a0a0a")
assertFile(/--surface:\s*#191919/.test(css), "--surface 为 xAI 面板实色 #191919")
assertFile(/--surface-soft:\s*#1a1c20/.test(css), "--surface-soft 为 xAI 柔和表面实色 #1a1c20")
assertFile(/--border:\s*#212327/.test(css), "--border 为 xAI 发丝线实色 #212327")

// ---------- B. 统一表面工具 ----------
console.log("B. 统一表面工具类已就位且为实色别名")
assertFile(/--surface:\s*#/.test(css), "--surface 已是实色（不再是 rgba 半透明白）")
assertFile(
  /\.glass\s*\{[^}]*background-color:\s*var\(--surface\)/.test(css),
  ".glass 指向实色 --surface",
)
assertFile(
  /\.glass-strong\s*\{[^}]*background-color:\s*var\(--surface-soft\)/.test(css),
  ".glass-strong 指向实色 --surface-soft",
)
assertFile(
  /\.glass-input\s*\{[^}]*background-color:\s*var\(--surface-soft\)/.test(css),
  ".glass-input 指向实色 --surface-soft",
)
assertFile(/--radius-sm:\s*0px/.test(css), "--radius-sm 归零（面板直角）")
assertFile(/--radius-full:\s*9999px/.test(css), "--radius-full 保留 9999px（按钮胶囊）")

// ---------- C. 实心不透明表面清零 ----------
console.log("C. 实心不透明黑洞清零")
for (const [file, text] of sources) {
  const rel = path.relative(srcRoot, file).replace(/\\/g, "/")
  for (const cls of ["bg-black/25", "bg-black/30", "bg-black/70", "bg-black/75"]) {
    assertFile(!text.includes(cls), `${rel} 不含 ${cls}`)
  }
  assertFile(!text.includes("min-h-screen bg-black"), `${rel} 无整页 bg-black（不遮挡画布）`)
  // bg-neutral-95x 实色只允许两类位置：原生下拉 <option>（无法毛玻璃、必须实色兜底）
  // 与 tailwind hover: 前缀（悬停过渡态，非静默黑洞）。其余位置视为黑窟窿。
  if (text.includes("bg-neutral-950") || text.includes("bg-neutral-900")) {
    for (const line of text.split("\n"))
      if (
        (line.includes("bg-neutral-950") || line.includes("bg-neutral-900")) &&
        !line.trim().startsWith("<option") &&
        !/(^|\s)hover:bg-neutral-9\d{2}/.test(line)
      ) {
        bad(`${rel} 含 bg-neutral-95x 实色：${line.trim().slice(0, 60)}`)
      }
  }
}

// ---------- D. 形态两轨制：按钮胶囊、面板直角 ----------
// 圆角分两轨，二者的分界是「是否可点」而非尺寸：
//   · 按钮 = 胶囊（rounded-full），xAI 规范 rounded.pill 的直接落地
//   · 面板 / 卡片 / 输入框 / 徽章 / 弹窗 / 进度条 = 直角（--radius-* 已归零）
// rounded-full 只允许两类：上表的功能性圆形（状态点 / 开关 / 图标底衬）与按钮本身。
// 守卫要点：裸 rounded（Tailwind 默认 4px）不受 --radius-* 令牌管辖，
// 一旦有人新写一个就会绕过归零，故此处按源码逐行兜底。
console.log("D. 形态两轨制：按钮胶囊、面板直角")
// 功能性圆形的行内特征；命中任一即视为合法圆形
const FUNCTIONAL_SHAPES = [
  /w-1\.5 h-1\.5/, // 状态小圆点
  /w-2 h-2/, // 脉冲 / 占位圆点
  /h-5 w-5/, // 开关滑块
  /h-6/, // 开关轨道、图标底衬
  /p-2 rounded-full/, // 警告图标圆形底衬
]
for (const [file, text] of sources) {
  if (!file.endsWith(".tsx")) continue
  const rel = path.relative(srcRoot, file).replace(/\\/g, "/")
  const stripped = text
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/.*$/gm, "")
    .split("\n")
  stripped.forEach((line, i) => {
    // 裸 rounded：既不是 rounded-full / rounded-none 也不是 rounded-xxx 的变体
    if (/(?<![\w-])rounded(?![\w-])/.test(line)) {
      bad(`${rel}:${i + 1} 裸 rounded 会绕过归零令牌，须显式写 rounded-full 或 rounded-none`)
    }
    if (line.includes("rounded-[") && !line.includes("rounded-full")) {
      // rounded-[var(--radius-*)] 合法：令牌已归零，视觉即直角
      if (!/rounded-\[var\(--radius-(sm|md|lg|xl)\)\]/.test(line)) {
        bad(`${rel}:${i + 1} rounded-[...] 只允许 --radius-sm/md/lg/xl（均已归零）`)
      }
    }
    if (line.includes("rounded-full")) {
      const functional = FUNCTIONAL_SHAPES.some((re) => re.test(line))
      // 按钮文件白名单：Button 组件全站唯一按钮来源，另两处是自定义按钮
      const isButtonFile = /components\/ui\/Button\.tsx$|components\/ui\/Toast\.tsx$|routes\/Login\.tsx$/.test(rel)
      if (!functional && !isButtonFile) {
        bad(`${rel}:${i + 1} rounded-full 仅限按钮与功能性圆形，此处疑似面板误用`)
      }
    }
  })
}
ok("D 段逐行检查完成")

console.log(failed === 0 ? "\n全部通过，视觉表面协调一致。" : `\n发现 ${failed} 处违例。`)
process.exit(failed === 0 ? 0 : 1)

function assertFile(cond, msg) {
  if (cond) ok(msg)
  else bad(msg)
}