// 平台档案化回归守卫：锁定"站点事实只存在于后端档案里"这条边界，以及页脚命名。
// 站点域名/接口路径/字段名一旦漏进前端源码，就等于把某个年份的接口形态写死进 UI——
// 换平台时必须改前端，正是本次重构要消灭的东西。
// 纯静态断言，与 font-guard / lazy-route-guard 同款。
// 用法：node --import jiti/register scripts/platform-guard.ts
import { readFileSync, readdirSync, statSync } from "node:fs"
import { join } from "node:path"

const root = join(import.meta.dirname, "..")
const srcRoot = join(root, "src")

let failed = false
const check = (name: string, ok: boolean) => {
  if (!ok) failed = true
  console.log(`${ok ? "✓" : "✗"} ${name}`)
}

const walk = (dir: string): string[] => {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) out.push(...walk(full))
    else if (/\.[tj]sx?$/.test(full)) out.push(full)
  }
  return out
}
const files = walk(srcRoot)

// 1. 「至道」在用户可见面与前端源码里彻底缺席（平台名只出现在后端档案里）。
const legacyName: string[] = []
for (const f of files) {
  if (readFileSync(f, "utf8").includes("至道")) legacyName.push(f.replace(root, ""))
}
check(`前端源码无「至道」字样${legacyName.length ? `（命中：${legacyName.join("、")}）` : ""}`, legacyName.length === 0)

// 2. 页脚文案 = 「© 2026 自动选课」（主人明令：不要平台名，就叫自动选课）。
const footer = readFileSync(join(srcRoot, "components/Footer.tsx"), "utf8")
check("页脚文案为「© 2026 自动选课」", footer.includes("© 2026 自动选课"))

// 3. 站点事实（域名 / 接口路径 / 站点侧字段名）不得出现在前端源码：
//    它们属于 backend/internal/sites/<平台>/ 的档案，前端只消费中立数据模型。
// 注意：自家 API 的 /api/electives/... 是中立路由（不属于站点事实），故不在此列；
// 列的是站点侧专有物——域名、站点接口方法名、站点鉴权载体名。
const siteFacts = [
  "zhidao.fj.cn",
  "findElectivesData",
  "selectElectivesClass",
  "exitElectivesClass",
  "findElectivesStudentCount",
  "idToken",
  "zd_edu_cookie",
  // 站点按钮编码与开窗信号：已中立化为 action/selectable，前端绝不该再看到原始键名。
  // 这两项当初漏进黑名单，正是「btn_type 泄漏进 UI 却守卫全绿」的原因。
  "btn_type",
  "in_date_range",
]
const leaks: string[] = []
for (const f of files) {
  const code = readFileSync(f, "utf8")
  for (const fact of siteFacts) {
    if (code.includes(fact)) leaks.push(`${f.replace(root, "")} → ${fact}`)
  }
}
check(`前端源码无站点事实（域名/接口路径/站点侧字段名）${leaks.length ? `（命中：${leaks.join("、")}）` : ""}`, leaks.length === 0)

// 4. 平台档案字段必须贯通：类型声明 PlatformInfo、配置页提交 platform_id 与地址覆盖。
const types = readFileSync(join(srcRoot, "types.ts"), "utf8")
check("types.ts 声明 PlatformInfo", /export interface PlatformInfo\b/.test(types))
check("AdminConfig 声明 platform_id", /platform_id:\s*string/.test(types))
check("AdminConfig 声明 platform_base_url", /platform_base_url:\s*string/.test(types))

const admin = readFileSync(join(srcRoot, "routes/Admin.tsx"), "utf8")
check("配置页提交 platform_id", /body\.platform_id\s*=\s*platformId/.test(admin))
check("配置页提交 platform_base_url", /body\.platform_base_url\s*=\s*platformBaseUrl\.trim\(\)/.test(admin))
check("配置页渲染后端下发的档案列表", /loaded\?\.platforms\s*\?\?\s*\[\]/.test(admin))

// 5. 中立字段必须贯通：类型声明与消费点都读中立名，绝不回退站点原始键名。
// 正向断言与上面的黑名单配对——只有黑名单会在「字段被删掉」时误判为通过。
const types5 = readFileSync(join(srcRoot, "types.ts"), "utf8")
check(
  "types.ts 声明 action 中立枚举",
  /action:\s*"enroll"\s*\|\s*"withdraw"\s*\|\s*"none"/.test(types5),
)
check("types.ts 声明 selectable 三态", /selectable:\s*boolean\s*\|\s*null/.test(types5))
check("types.ts 不再声明 btn_type", !/\bbtn_type\b/.test(types5))

const selectSrc = readFileSync(join(srcRoot, "routes/Select.tsx"), "utf8")
check(
  "Select 消费 action 而非站点按钮编码",
  /c\.action === "enroll"/.test(selectSrc) && !/c\.btn_type/.test(selectSrc),
)
check(
  "Select 消费 selectable 而非站点开窗字段",
  /p\.selectable === true/.test(selectSrc) && !/p\.in_date_range/.test(selectSrc),
)

console.log(failed ? "\nplatform-guard 断言失败" : "\nplatform-guard 断言全绿")
process.exit(failed ? 1 : 0)
