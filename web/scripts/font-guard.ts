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

// 剥离注释后的源码：所有正则断言一律基于它。
// 否则注释里出现的说明字样（例如「刻意不声明 text-spacing-trim」）会触发误报。
const cssCode = css.replace(/\/\*[\s\S]*?\*\//g, "")

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
check("global.css 定义 --font-sans", /--font-sans\s*:/.test(cssCode))
check("global.css 定义 --font-mono", /--font-mono\s*:/.test(cssCode))

// 2. body 字体必须读令牌，不得回退硬编码栈
check(
  "body font-family 读 var(--font-sans)",
  /body\s*\{[^}]*font-family\s*:\s*var\(--font-sans\)/.test(cssCode),
)
check("body 不残留硬编码 Inter 字体栈", !/font-family\s*:\s*['"]Inter['"]/.test(cssCode))

// 3. 三个字体包必须被引入（中文于 2026-09-27 由 Noto Sans SC 换为 MiSans VF）
for (const pkg of [
  "@fontsource-variable/geist",
  "@fontsource-variable/geist-mono",
  "misans-vf/lib/MiSans.min.css",
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
  check(`global.css 定义 ${token}`, cssCode.includes(token))
}

// 5. 无效排版声明必须缺席。这些声明曾经写过，但 fontTools 实测证明字体不支持：
//    Geist Mono 的 GSUB 只有 ccmp/dnom/frac/locl/numr（无 liga/calt/tnum/zero），
//    当时的 Noto Sans SC 只有 ccmp/liga/locl/vert/vrt2（无 halt/chws）；
//    且 text-spacing-trim 与 text-autospace 的 normal 均为 CSS 初始值。加回来等于虚假承诺。
//    中文现已换为 MiSans VF——要加任何 OpenType 特性声明前，同样先解析字体文件实测。
check(
  "global.css 不含已被实测否定的无效声明（font-feature-settings / text-spacing-trim / text-autospace）",
  !/font-feature-settings|text-spacing-trim|text-autospace/.test(cssCode),
)

// 6. 跨平台字体膨胀修正必须存在（Android WebView 必需）
check("声明 text-size-adjust: 100%", /text-size-adjust\s*:\s*100%/.test(cssCode))

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
