# R122 前端只读审查报告

## 头部

- **审查对象 HEAD**：`2665647`（docs(review): R121 双 findings + 收尾总结，进度 122/256）
- **审查范围**：`web/`（React 18 + Vite + TS + Radix UI + Tailwind）
- **审查方式**：绝对只读；Read/Grep 逐点实证 + `npm run build`（tsc -b 真校验）+ 三个纯函数断言脚本 + audit.mjs；未修改任何仓库文件（仅写本报告）
- **审查时间**：R122 轮，聚焦清单 7 项逐条走查

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**

连续五十九轮零严重级状态延续。本轮聚焦清单 7 项全部「通过/维持」，无新问题引入。

## 必查项逐条结论

### 1. M-1 第五十八轮闭合复核 —— 通过

- **shouldDeferSave 四消费点逐字符一致**：`:509`（flushTargets）、`:597`（handleBack 首闸）、`:605`（handleBack 5s 等待循环）、`:699`（防抖回调），四处均为 `shouldDeferSave(stateDataRef.current, <选中布尔>, echoedRef.current)` 三参形态逐字符一致。布尔实参：`:509` `latestSelectedCount > 0`、`:597`/`:605` `hasSelectedNow()`、`:699` `selectedCount > 0`。grep 确认全仓 `shouldDeferSave(` 恰 4 消费点 + 1 定义点（targetGuard.ts:64-72），零增零减。
- **shouldDeferSave 纯函数逻辑**（targetGuard.ts:64-72）：`stateData===undefined` 无条件推迟 → `echoed` 已回显放行 → 否则 `courses 非空 && hasSelected` 才推迟。三判据顺序与契约 35/57 一致（清空语义与数据缺席分判：`hasSelected=false` 放行，首帧未到仍推迟），注释契约 45-63 逐字保留「三参语义」（第三参 echoed 澄清已回显稳态放行普通编辑）。
- **echoedRef 置位三路径**：`:200`（账号切换复位 `echoedRef.current = false` + setEchoDone(false)）、`:240`（/state 首帧 courses 空 → 置位 + setEchoDone(true)）、`:297`（回显合并完成 → 置位 + setEchoDone(true)）。首帧不置位四边界：`:229`（`if (echoedRef.current) return` 已回显短路）、`:234`（`stateData === undefined` 直接 return，绝不提前置位）、`:247`（`pubs.length === 0` return）、`:319`（独立清理 effect 首行 `!echoedRef.current || publishes.length === 0` 短路）。
- **target-guard-check.ts 18 断言全绿**：`node --import jiti/register scripts/target-guard-check.ts`，尾行「target-guard 断言全绿」。

**结论：M-1 第五十八轮闭合通过，四消费点三参形态与置位三路径逐字符在位，18 断言全绿。**

### 2. OBSERVE-93-01 残余面复核（第十二轮家族册） —— 维持

- **grep `<button` 全仓清点：17 处、仅 3 文件**（Admin.tsx 13 / Login.tsx 2 / Dashboard.tsx 2），残余面清单零增零减。逐处行号：Admin 417（清空生成）/ 425（复制）/ 441（刷新）/ 452（重试）/ 470（复制 aria-label="复制"）/ 473（删除 aria-label="删除"）/ 577（switch）/ 651 / 661（引擎二选一）/ 701（重试）/ 792（stats 刷新）/ 898（accounts 刷新）/ 944（logs 刷新）；Login 167（密码可见性切换）/ 299；Dashboard 67（CollapseSection 展开收起）/ 707（logs 重试）。
- **引擎二选一 651/661 强 active 态仍在位**（`engine===... ? "border-white bg-white text-black font-medium" : "border-neutral-800 glass-input text-neutral-400 hover:text-white"`），仍是优先修复面（无 `aria-pressed`，active 态仅视觉表达）。
- icon-only 按钮均有 aria-label（Admin:470/473），文本按钮天然可读，缺口仍纯 display 维度，与二勘定案维持一致。

**结论：OBSERVE-93-01 残余面清单零增零减（17 处 3 文件），引擎二选一 651/661 仍为优先修复面（维持观察）。**

### 3. F93-01 第三十轮 —— 通过

- `git log --oneline 1351fa4..HEAD -- web/`：**空输出**（EXIT:0）。
- `git diff --stat 1351fa4 HEAD -- web/`：**空**。
- Button.tsx:42 focus-visible ring 逐字符在位：`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"` 零回归。

**结论：web/ 自 1351fa4 起零代码漂移双实证成立，F93-01 修复面连续第三十轮无回归。**

### 4. OBSERVE-116-01 跟踪项第六轮复核 —— 维持（不修）

- useTickingCountdown（lib）内自愿注释如实口径在位：`每秒 setNow 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——本注释已按实现如实口径，不再声称局部渲染）`——逐字在位。
- 无卡顿证据：Dashboard/Select 轮询驱动的是 3s/2s 查询级刷新，每秒 setNow 由 React 差分承担；两条路由组件树规模可控。
- 「勿动轮询链路契约」确认未被动：Select `/state` 函数式 refetchInterval（`:58-81`）：error/`window_closed` → 30s、`inRange || window_opened` → 2s、否则 10s；Select `/electives`（`:148-154`）：error → 30s、`window_closed` → 30s、否则 2s。Dashboard `/state`（`:140`）与 `/logs`（`:156`）：error/`window_closed` → 30s、否则 3s；`/electives`（`:174`）30s。全站轮询节奏与上轮逐字符一致，零漂移。

**结论：OBSERVE-116-01 维持，潜在优化注释在位，轮询链路契约零漂移。**

### 5. OBSERVE-115-01 弹窗族复核 —— 通过

三处裸 div 弹窗逐处核对：

| 弹窗 | role=dialog | aria-modal | aria-labelledby | Esc 关闭 | autoFocus | 焦点陷阱声明 |
|------|------------|------------|-----------------|---------|-----------|--------------|
| Select 退选 `:1198-1250` | `:1204` | `:1205` (true) | `exit-modal-title` | `:1207-1208`（退选中禁用） | 取消按钮 `:1234` | `:1198` 注释 |
| Login 激活 `:220-267` | `:226` | `:227` (true) | `activate-dialog-title` | `:232-233`（激活中禁用） | 输入框 `autoFocus`(267) | `:220-221` 注释 |
| Admin 删除 `:208-241` | `:213` | `:214` (true) | `delete-acct-modal-title` | `:216-217`（删除中禁用） | 取消按钮 `:241` | `:208` 注释 |

- 三处最小语义门全部在位；无焦点陷阱均为注释声明刻意取舍。
- Select 退选弹窗仍为优先修复面（唯一带 Esc 三重语义门 + `aria-labelledby` 最完整，与 Login/Admin 同构）——维持观察。

**结论：OBSERVE-115-01 弹窗族零漂移，最小语义门与 Esc 关闭三处齐整。**

### 6. OBSERVE-117-02 timer 类型卫生观察 —— 维持

- Select.tsx:393：`const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })`——timer 字段类型化到位，`resetRetry`（`:409-413`）与 scheduleRetry（`:420-429`）配对清零/判空中均检查 `retryState.current.timer` 非空才 clearTimeout；卸载 cleanup（`:400-408`）同步清退避 timer 并复位 unmountedRef（StrictMode 重挂载自愈）。
- `npm run build`（tsc -b）通过——纯类型卫生非缺陷确认。

**结论：OBSERVE-117-02 维持，纯类型卫生观察成立。**

### 7. 新契约角度（本轮自选）—— 错误文案在前端的可辨性呈现

对照后端端到端可辨性结论（窗口关闭/满员/风控/read 类/失效五类，round121 后端报告第 6 项），走查 Dashboard 事件流/徽章/说明在前端的类别区分度：

| 类别 | 后端 result 文案（实证） | 前端呈现 | 可辨性 |
|------|--------------------------|----------|--------|
| 成功 | `"选课成功！"` 系 | Badge「已确认选课」primary + CheckCircle2 + 「席位已确认」 | 独立 |
| 满员 | `"该课程已满员，退避至下一备选"`（scheduler.go:1768） | `isFullFallback`（Dashboard:36-38）`result.includes("已满员")` → Badge「已满员·退避备选」destructive + 「该门已满，自动退避至下一备选」 | 独立，判据后端文案挂钩 |
| 风控 | `"触发平台风控退避 30 秒: <err>"`（scheduler.go:1556） | **归入 `isFailed` → Badge「报名异常」+「提交未通过」，与 read 类失败混同** | **混同（弱）** |
| read/网络类失败 | `status:"failed"` 其余 | 同上「报名异常」 | 与风控不可分 |
| 会话失效 | `token_valid===false` | Dashboard:490-497「已失效 · 自动恢复中」（白点脉冲）+ Dashboard:319-323 `isSessionError` 时「当前登录凭据已失效，请重新进行账户认证」 | 独立 |
| 窗口关闭 | `state.window_closed` | Dashboard:343-346 顶栏「窗口已关闭」outline + :751「窗口已关闭」 | 独立 |

**结论**：五类中四类（成功/满员/失效/窗口关闭）前端均有差异化徽章/文案，唯「风控 vs 其他 failed」前端口径均为「报名异常」无细分。但需如实归因：**前端 CourseStatus 仅 `status + result` 两个通道**（types.ts:39-47），后端 `status="failed"` 的单值已封顶该层表达力；`result` 全文虽可读（Dashboard 已用于 `includes("已满员")` 分拣），风控文案同样可以 `result.includes("风控")` 复用同款 `isFullFallback` 手法细分，属**纯 display 级可选优化**。且后端归因（scheduler.go:1556）`result` 携带原始 err 含 URL 脱敏后文本，无信息丢失。与 R121「满员 vs 窗口关闭混同」核实同理，此处**无混同但存在可辨性颗粒度**，维持观察不升级。

## 契约 20 检查（轮次前缀标签）

grep `第 *[0-9]\+ *轮` / `轮次` / `第 N 轮` 形态（web/src + web/scripts + web/package.json）：**全仓零命中**。全部注释为「为什么/契约/陷阱」本身口径（本轮通读 Select.tsx 轮询/回显/重试/防抖段注释 + targetGuard.ts 全文注释 + useTickingCountdown 注释 + Dashboard 事件流注释确认）。

## 验证表

| 命令 | 结果 |
|------|------|
| `npm run build`（tsc -b + vite） | 通过，1948 modules，产物 index-1KHlpqcc.css / index-Bm7TtkV4.js（与 R121 逐字节同名，双实证） |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全部通过，视觉表面协调一致 |
| `git log --oneline 1351fa4..HEAD -- web/` | 空输出（零漂移实证一） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证二） |
| grep `shouldDeferSave(` / `echoedRef` / `<button` / `focus-visible:ring` / `refetchInterval` / 轮次标签 / `role="dialog"` / 风控-满员文案 | 逐点比对确认，零增零减 |

## 已核无缺陷清单

- M-1 五十八轮：shouldDeferSave 四消费点三参形态逐字符在位 + echoedRef 置位三路径/首帧不置位四边界完整 + 18 断言全绿
- OBSERVE-93-01：裸按钮 17 处 3 文件零增零减、缺口纯 display、引擎二选一 651/661 强 active 态在位（维持优先修复面）
- F93-01：Button ring 逐字符在位 + web/ 零代码漂移双实证
- OBSERVE-115-01：三弹窗最小语义门 + Esc 关闭 + autoFocus 逐处在位
- OBSERVE-116-01：潜在优化注释在位、无卡顿证据、轮询契约（/state /logs 3s/30s、Select /electives 2s/10s/30s、Dashboard /electives 30s）零漂移
- OBSERVE-117-02：retryState timer 类型卫生非缺陷、构建通过
- 契约 20：轮次前缀标签全仓零命中
- 新契约角度：错误文案可辨性走查——五类中四类差异化呈现，风控 vs 其他 failed 归「报名异常」同口径（display 级颗粒度、非混同非缺陷，维持观察）

## 结论

R122 前端只读审查完成，聚焦清单 7 项全部「通过/维持」。关键复核结论：

- **M-1 第五十八轮闭合通过**——shouldDeferSave 四消费点三参形态逐字符一致，echoedRef 置位三路径（账号复位/空首帧/合并完成）与首帧不置位边界（undefined return / pubs 空 return / 已回显短路）完整，18 断言全绿。
- **OBSERVE-93-01 残余面维持**——裸按钮清单零增零减（17 处、3 文件），缺口仍纯 display 维度；引擎二选一 651/661 带强 active 态仍是优先修复面；Button.tsx:42 ring 逐字在位。
- **OBSERVE-116-01 跟踪项维持**——每秒 setNow 潜在优化注释如实口径在位、无卡顿证据、轮询链路契约未被动，第六轮维持不修。
- **OBSERVE-117-02 维持**——timer 类型卫生纯观察，构建通过非缺陷。
- **错误文案可辨性走查无新缺陷**——成功/满员/失效/窗口关闭四类差异化呈现；风控 vs 其他 failed 同归「报名异常」为 display 级颗粒度（result 字段可细分、后端无信息丢失），维持观察不升级。

连续五十九轮零严重级状态延续，web/ 自 1351fa4 起零代码漂移（R122 双实证），稳定期审查纪律未见松动。
