// 路由懒加载回归守卫：锁定 P-4 优化（Select/Admin 必须在 App.tsx 走 React.lazy，
// 不得回退为静态顶层 import）。懒加载把登录首屏主 bundle 从 422.57kB 压到
// 363.15kB（首屏省 59.42kB）；若有人改回 `import Select from ...` 静态引入，
// 单 bundle 回归膨胀、首屏与并行加载双败。纯静态断言，与 perf-countdown-guard 同款。
// 用法：node --import jiti/register scripts/lazy-route-guard.ts
import { readFileSync } from "node:fs"
import { join } from "node:path"

const root = join(import.meta.dirname, "..")
const app = readFileSync(join(root, "src/App.tsx"), "utf8")

let failed = false
const check = (name: string, ok: boolean) => {
  if (!ok) failed = true
  console.log(`${ok ? "✓" : "✗"} ${name}`)
}

// 1. Select/Admin 必须走 React.lazy（动态 import），不得静态 import。
//    注意 import 行自带 `import Select from "./routes/Select"` 与
//    `const Select = lazy(() => import("./routes/Select"))` 两种形态——
//    守卫只断言 lazy pool 存在 + 静态 import 行不存在。
const staticSelect = /^import\s+Select\b.*from\s+["'].\/routes\/Select["']/m.test(app)
const staticAdmin = /^import\s+Admin\b.*from\s+["'].\/routes\/Admin["']/m.test(app)
check("Select 未静态 import（改回静态=单 bundle 回归）", !staticSelect)
check("Admin 未静态 import（改回静态=单 bundle 回归）", !staticAdmin)

// 2. lazy pool 定义在位（两条 const Select/Admin = lazy(...)）
const hasLazySelect = /const\s+Select\s*=\s*lazy\(\(\)\s*=>\s*import\(["']\.\/routes\/Select["']\)\)/.test(app)
const hasLazyAdmin = /const\s+Admin\s*=\s*lazy\(\(\)\s*=>\s*import\(["']\.\/routes\/Admin["']\)\)/.test(app)
check("App 定义 lazy Select", hasLazySelect)
check("App 定义 lazy Admin", hasLazyAdmin)

// 3. Suspense 包裹 app-content（懒挂起时有兜底 fallback）——Suspense 紧邻
//    app-content 之后、闭合在其内部（`<main className="app-content">` 后 10 行内）。
check("app-content 紧邻 <Suspense>（包裹路由渲染）", /className="app-content"[\s\S]{0,120}<Suspense/.test(app))

console.log(failed ? "\nlazy-route-guard 断言失败" : "\nlazy-route-guard 断言全绿")
process.exit(failed ? 1 : 0)
