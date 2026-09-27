// 排版回归守卫：锁定全站字体与排版令牌契约。
// 字体栈被手写硬编码回退、三个字体包被移除、字号令牌被删、或 woff2 被内联，
// 都会让「三平台一致 + 真实字重 + 按需加载」同时失效且不易察觉。
// 纯静态断言，与 lazy-route-guard 同款。
// 用法：node --import jiti/register scripts/font-guard.ts
import { readFileSync, readdirSync, statSync } from "node:fs"
import { join } from "node:path"

const root = join(import.meta.dirname, "..")
const css = readFileSync(join(root, "src/styles/global.css"), "utf8")
const main = readFileSync(join(root, "src/main.tsx"), "utf8")
const vite = readFileSync(join(root, "vite.config.ts"), "utf8")

let failed = false
const check = (name: string, ok: boolean) => {
  if (!ok) failed = true
  console.log(`${ok ? "✓" : "✗"} ${name}`)
}

// 收集组件源码（只扫 tsx，故不会命中 global.css 自身的 font-family）
const walk = (dir: string): string[] => {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) out.push(...walk(full))
    else if (full.endsWith(".tsx")) out.push(full)
  }
  return out
}
const tsxFiles = walk(join(root, "src"))

// 1. 字体栈令牌必须定义（--font-* 命名空间才生成 font-sans / font-mono 工具类）
check("global.css 定义 --font-sans", /--font-sans\s*:/.test(css))
check("global.css 定义 --font-mono", /--font-mono\s*:/.test(css))

// 2. body 字体必须读令牌，不得回退硬编码栈
check(
  "body font-family 读 var(--font-sans)",
  /body\s*\{[^}]*font-family\s*:\s*var\(--font-sans\)/.test(css),
)
check("body 不残留硬编码 Inter 字体栈", !/font-family\s*:\s*['"]Inter['"]/.test(css))

// 3. 三个字体包必须被引入
for (const pkg of [
  "@fontsource-variable/geist",
  "@fontsource-variable/geist-mono",
  "@fontsource-variable/noto-sans-sc",
]) {
  check(`main.tsx 引入 ${pkg}`, main.includes(pkg))
}

// 4. 字号令牌必须存在（排版系统接入标志）
for (const token of [
  "--text-3xs:",
  "--text-2xs:",
  "--text-xs:",
  "--text-sm:",
  "--text-base:",
  "--text-lg:",
  "--text-xl:",
  "--text-2xl:",
  "--text-4xl:",
]) {
  check(`global.css 定义 ${token}`, css.includes(token))
}

// 5. 等宽场景 OpenType 特性必须声明
check(
  "global.css 声明 font-feature-settings（关连字 + 斜杠零）",
  /font-feature-settings\s*:[^;]*"zero"/.test(css),
)

// 6. 跨平台字体膨胀修正必须存在（Android WebView 必需）
check("声明 text-size-adjust: 100%", /text-size-adjust\s*:\s*100%/.test(css))

// 7. woff2 必须排除内联（否则 unicode-range 按需加载失效）
check(
  "vite.config.ts 排除 woff2 内联",
  /assetsInlineLimit[\s\S]{0,240}woff2/.test(vite),
)

// 8. 组件层禁止手写 font-family（去注释后再判，避免注释误报）
const offenders: string[] = []
for (const file of tsxFiles) {
  const code = readFileSync(file, "utf8")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/\/\/.*$/gm, "")
  if (/font-family\s*:/.test(code)) offenders.push(file.replace(root, ""))
}
check(
  `组件层无手写 font-family${offenders.length ? `（命中：${offenders.join("、")}）` : ""}`,
  offenders.length === 0,
)

// 9. 任意值字号必须已收敛为令牌
const arb: string[] = []
for (const file of tsxFiles) {
  if (/text-\[\d+px\]/.test(readFileSync(file, "utf8"))) {
    arb.push(file.replace(root, ""))
  }
}
check(
  `无残留 text-[Npx] 任意值${arb.length ? `（命中：${arb.join("、")}）` : ""}`,
  arb.length === 0,
)

console.log(failed ? "\nfont-guard 断言失败" : "\nfont-guard 断言全绿")
process.exit(failed ? 1 : 0)
