# R123 前端只读审查报告

## 头部

- **审查对象 HEAD**：`7b63aff`（docs(review): R122 双 findings + 收尾总结，进度 123/256）
- **审查范围**：`web/`（React 18 + Vite + TS + Radix UI + Tailwind）
- **审查方式**：绝对只读；Read/Grep 逐点实证 + `npm run build`（tsc -b 真校验）+ 三个纯函数断言脚本 + audit.mjs；未修改任何仓库文件（仅写本报告）
- **审查时间**：R123 轮，聚焦清单 7 项逐条走查

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**

连续六十轮零严重级状态延续。本轮聚焦清单 7 项全部「通过/维持」，无新问题引入。

## 必查项逐条结论

### 1. M-1 闭合复核（第五十九轮） —— 通过

- **shouldDeferSave 四消费点逐字符一致**：`:509`（flushTargets）、`:597`（handleBack 首闸）、`:605`（handleBack 5s 等待循环）、`:699`（防抖回调），四处均为 `shouldDeferSave(stateDataRef.current, <选中布尔>, echoedRef.current)` 三参形态逐字符一致。布尔实参：`:509` `latestSelectedCount > 0`、`:597`/`:605` `hasSelectedNow()`、`:699` `selectedCount > 0`。grep 确认全仓 `shouldDeferSave(` 恰 4 消费点 + 1 定义点（targetGuard.ts:64），零增零减。
- **shouldDeferSave 纯函数逻辑**（targetGuard.ts:64-72）：`stateData === undefined` 无条件推迟 → `echoed` 已回显放行 → 否则 `courses 非空 && hasSelected` 才推迟。三判据顺序与契约 35/57 一致（清空语义与数据缺席分判：`hasSelected=false` 放行、首帧未到仍推迟），注释契约 45-63 逐字保留「三参语义」（第三参 echoed 澄清已回显稳态放行普通编辑）。
- **echoedRef 置位三路径**：`:200`（账号切换复位 `echoedRef.current = false` + setEchoDone(false)）、`:240`（/state 首帧 courses 空 → 置位 + setEchoDone(true)）、`:297`（回显合并完成 → 置位 + setEchoDone(true)）。首帧不置位四边界：`:229`（`if (echoedRef.current) return` 已回显短路）、`:234`（`stateData === undefined` 直接 return，绝不提前置位）、`:247`（`pubs.length === 0` return）、`:319`（独立清理 effect 首行 `!echoedRef.current || publishes.length === 0` 短路）。
- **target-guard-check.ts 断言全绿**：`node --import jiti/register scripts/target-guard-check.ts` 精确 18 个 ✓，尾行「target-guard 断言全绿」。

**结论：M-1 闭合通过，四消费点三参形态与置位三路径逐字符在位，18 断言全绿。**

### 2. OBSERVE-93-01 残余面复核（第十三轮家族册） —— 维持

- **grep `<button` 全仓清点：17 处、仅 3 文件**（Admin.tsx 13 / Login.tsx 2 / Dashboard.tsx 2），残余面清单零增零减。逐处行号与上轮完全一致：Admin 417 / 425 / 441 / 452 / 470 / 473 / 577 / 651 / 661 / 701 / 792 / 898 / 944；Login 167 / 299；Dashboard 67 / 707。
- **引擎二选一 651/661 强 active 态仍在位**（`engine===... ? "border-white bg-white text-black font-medium" : "border-neutral-800 glass-input text-neutral-400 hover:text-white"`），仍是优先修复面（无 `aria-pressed`，active 态仅视觉表达）。
- icon-only 按钮均有 aria-label（Admin:470/473），缺口仍纯 display 维度，与二勘定案维持一致。

**结论：OBSERVE-93-01 残余面清单零增零减（17 处 3 文件），引擎二选一 651/661 仍为优先修复面（维持观察）。**

### 3. F93-01 第三十一轮 —— 通过

- `git log --oneline 1351fa4..HEAD -- web/`：**空输出**（EXIT:0）。
- `git diff --stat 1351fa4 HEAD -- web/`：**空**。
- Button.tsx:42 focus-visible ring 逐字符在位：`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"` 零回归。

**结论：web/ 自 1351fa4 起零代码漂移双实证成立，F93-01 修复面连续第三十一轮无回归。**

### 4. OBSERVE-116-01 跟踪项第七轮复核 —— 维持（不修）

- useTickingCountdown（lib）内自愿注释如实口径在位：`每秒 setNow 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——本注释已按实现如实口径，不再声称局部渲染）`——逐字在位。
- 无卡顿证据：Dashboard/Select 轮询驱动的是 3s/2s 查询级刷新，每秒 setNow 由 React 差分承担；两条路由组件树规模可控。
- 「勿动轮询链路契约」确认未被动：Select `/state` 函数式 refetchInterval（`:58-81`）：error/`window_closed` → 30s、`inRange || window_opened` → 2s、否则 10s；Select `/electives`（`:148-154`）：error → 30s、`window_closed` → 30s、否则 2s。Dashboard `/state`（`:140-145`）与 `/logs`（`:156-161`）：error/`window_closed` → 30s、否则 3s；`/electives`（`:174`）恒 30s。全站轮询节奏与上轮逐字符一致，零漂移。

**结论：OBSERVE-116-01 维持，潜在优化注释在位，轮询链路契约零漂移。**

### 5. OBSERVE-115-01 弹窗族复核 —— 通过

三处裸 div 弹窗逐处核对：

| 弹窗 | role=dialog | aria-modal | aria-labelledby | Esc 关闭 | autoFocus | 焦点陷阱声明 |
|------|------------|------------|-----------------|---------|-----------|--------------|
| Select 退选 `:1198-1250` | `:1204` | `:1205` (true) | `exit-modal-title` | `:1207-1208`（退选中禁用） | 取消按钮 `:1234` | `:1198-1199` 注释 |
| Login 激活 `:220-267` | `:226` | `:227` (true) | `activate-dialog-title` | `:232-233`（激活中禁用） | 输入框 `autoFocus`(267) | `:220-222` 注释 |
| Admin 删除 `:208-241` | `:213` | `:214` (true) | `delete-acct-modal-title` | `:216-217`（删除中禁用） | 取消按钮 `:241` | `:208-209` 注释 |

- 三处最小语义门全部在位；无焦点陷阱均为注释声明刻意取舍。
- Select 退选弹窗仍为优先修复面（唯一带 Esc 三重语义门 + `aria-labelledby` 最完整，与 Login/Admin 同构）——维持观察。

**结论：OBSERVE-115-01 弹窗族零漂移，最小语义门与 Esc 关闭三处齐整。**

### 6. OBSERVE-117-02 timer 类型卫生观察 —— 维持

- Select.tsx:393：`const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })`——timer 字段类型化到位，`resetRetry`（`:409-413`）与 scheduleRetry（`:420-429`）配对清零/判空中均检查 `retryState.current.timer` 非空才 clearTimeout；卸载 cleanup（`:399-408`）同步清退避 timer 并复位 unmountedRef（StrictMode 重挂载自愈）。
- `npm run build`（tsc -b）通过——纯类型卫生非缺陷确认。

**结论：OBSERVE-117-02 维持，纯类型卫生观察成立。**

### 7. 新契约角度（本轮自选）—— 轮询调度降频/升频状态机可行性

对照后端 `windowClosedLocked` 三判据（scheduler.go:918-939）逐条映射前端 refetchInterval 分支覆盖度，重点验证「降频后升不回来」与「升频后降不下去」两个单向卡死风险：

| 后端状态转移 | 前端信号 | 降频分支 | 解除（升回）路径 |
|-------------|---------|---------|-----------------|
| 窗口关闭（主判据：曾开窗+空快照+已过开窗点 10s） | `/state` `window_closed=true`（StateForAccount 同源 windowClosedLocked） | Select /state + /electives 30s、Dashboard /state + /logs 30s | /state 成功返回 false → 下一次调度升回 2s/3s；/electives 经组件重渲染闭包更新升回 2s（:69-71 注释确认） |
| 时钟连续失败 ≥3（scheduler.go:925） | 同样下沉为 `window_closed=true`（三判据单源） | 同上 30s | 同主判据路径 |
| 幽灵窗口 EmptyProbeRuns≥3（scheduler.go:935） | 同样 `window_closed=true` | 同上 30s | 后端注释自愈：新一轮 beginTimes 归零 emptyRuns、判定解除 → /state 返回 false 升回 |
| 查询失败 | `query.state.error/status==="error"` | 30s | react-query 成功后函数式回调按当前状态重排，error 消失即升回 |
| 开窗（inRange / window_opened） | 2s 升频 | — | window_closed 置位后降 30s，非单向 |

**核对结论**：

- **无「降频后永远升不回来」**：三类降频（error / window_closed / 固定 30s）均有明确解除路径。error 态靠 react-query 成功重排自愈；window_closed 态靠后端三判据各自的归零自愈（syncFailStreak 归零 / EmptyProbeRuns 归零 / 新一轮开窗）反射回 /state false 值；/electives 升频依赖「/state 每 2s/3s 刷新触发组件重渲染 → react-query 用最新闭包重调度间隔」的既有注释承诺（Select :69-71、Dashboard :151-154），该链路在本次走查中未被破坏。
- **无「升频后无法降频」**：2s 升频条件（inRange || window_opened）在窗口关闭后随 window_closed 置位统一降 30s；主判据自带 10s 裕量，裕量内保持 2s 属设计内行为（防误挂黄金期），非缺陷。
- **一个可观察的中间态（非缺陷）**：Select /electives 的降频读 `/state` 缓存（`getQueryData`），当 /state 查询失败而 /electives 成功时，缓存可能陈旧为 window_closed=false → /electives 落入 10s 而非 30s。此中间态限于「/state 失败且窗口已关」双条件同现，10s 轮询成本低（后端 /electives 空结构体响应），且 /state 恢复后缓存即刷新收敛。与 R122「风控可辨性颗粒度」同属轻量兜底，维持观察不升级。

**结论：轮询状态机覆盖后端全部状态转移，降频均带解除路径，无单向卡死；仅存 /state 失败期间的 10s 轻量中间态，不构成缺陷。**

## 契约 20 检查（轮次前缀标签）

grep `第 ?[0-9]+ ?轮` / `轮次` / `第 N 轮` / `round[0-9]+` / `R1[0-9][0-9]` 形态（web/src + web/scripts + web/package.json）：**全仓零命中**。全部注释为「为什么/契约/陷阱」本身口径（本轮通读 Select.tsx 轮询/回显/防抖/重试段注释 + targetGuard.ts 全文注释 + useTickingCountdown 注释 + Dashboard 轮询段注释 + 三弹窗语义注释确认）。

## 验证表

| 命令 | 结果 |
|------|------|
| `npm run build`（tsc -b + vite） | 通过，1948 modules，产物 index-1KHlpqcc.css / index-Bm7TtkV4.js（与 R121/R122 逐字节同名，双实证延续） |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全部通过，视觉表面协调一致 |
| `git log --oneline 1351fa4..HEAD -- web/` | 空输出（零漂移实证一） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证二） |
| grep `shouldDeferSave(` / `echoedRef` / `<button` / `focus-visible:ring` / `refetchInterval` / 轮次标签 / `role="dialog"` / `windowClosedLocked` | 逐点比对确认，零增零减 |

## 已核无缺陷清单

- M-1：shouldDeferSave 四消费点三参形态逐字符在位 + echoedRef 置位三路径/首帧不置位四边界完整 + 18 断言全绿
- OBSERVE-93-01：裸按钮 17 处 3 文件零增零减、缺口纯 display、引擎二选一 651/661 强 active 态在位（维持优先修复面）
- F93-01：Button ring 逐字符在位 + web/ 零代码漂移双实证
- OBSERVE-115-01：三弹窗最小语义门 + Esc 关闭 + autoFocus 逐处在位
- OBSERVE-116-01：潜在优化注释在位、无卡顿证据、轮询契约（Select /state 2s/30s、/electives 2s/10s/30s、Dashboard /state /logs 3s/30s、/electives 30s）零漂移
- OBSERVE-117-02：retryState timer 类型卫生非缺陷、构建通过
- 契约 20：轮次前缀标签全仓零命中
- 新契约角度：轮询降频/升频状态机——后端三判据全覆盖、降频均有解除路径、无单向卡死；仅 /state 失败期间 Select /electives 读旧缓存 window_closed 的 10s 轻量中间态（维持观察不升级）

## 结论

R123 前端只读审查完成，聚焦清单 7 项全部「通过/维持」。关键复核结论：

- **M-1 闭合通过**——shouldDeferSave 四消费点三参形态逐字符一致，echoedRef 置位三路径（账号复位/空首帧/合并完成）与首帧不置位边界完整，18 断言全绿。
- **OBSERVE-93-01 残余面维持**——裸按钮清单零增零减（17 处、3 文件），缺口仍纯 display 维度；引擎二选一 651/661 带强 active 态仍是优先修复面；Button.tsx:42 ring 逐字在位。
- **OBSERVE-116-01 跟踪项维持**——每秒 setNow 潜在优化注释如实口径在位、无卡顿证据、轮询链路契约未被动，第七轮维持不修。
- **OBSERVE-117-02 维持**——timer 类型卫生纯观察，构建通过非缺陷。
- **轮询状态机走查无新缺陷**——前端 refetchInterval 分支覆盖后端 windowClosedLocked 全部三判据，降频均有解除路径（react-query 成功重排 / syncFailStreak 归零 / EmptyProbeRuns 归零 / 新一轮开窗），无单向卡死；仅 /state 失败期间 Select /electives 用旧缓存 window_closed 判定落入 10s 的轻量中间态，维持观察不升级。

连续六十轮零严重级状态延续，web/ 自 1351fa4 起零代码漂移（R123 双实证），稳定期审查纪律未见松动。
