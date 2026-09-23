# R121 前端只读审查报告

## 头部

- **审查对象 HEAD**：`90a76d1b52c8b458ec34310fd29ac8a86013b7b1`（docs(review): R120 双 findings + 收尾总结，进度 121/256）
- **审查范围**：`web/`（React 18 + Vite + TS + Radix UI + Tailwind）
- **审查方式**：绝对只读；Read/Grep 逐点实证 + `npm run build`（tsc -b 真校验）+ 三个纯函数断言脚本 + audit.mjs；未修改任何仓库文件（仅写本报告）
- **审查时间**：R121 轮（补位），聚焦清单 7 项逐条走查

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**

连续五十八轮零严重级状态延续。本轮聚焦清单 7 项全部「通过/维持」，无新问题引入。

## 必查项逐条结论

### 1. M-1 第五十七轮闭合复核 —— 通过

- **shouldDeferSave 四消费点逐字符一致**：`:509`（flushTargets）、`:597`（handleBack 首闸）、`:605`（handleBack 5s 等待循环）、`:699`（防抖回调），四处均为 `shouldDeferSave(stateDataRef.current, <选中布尔>, echoedRef.current)` 三参形态逐字符一致。布尔实参：`:509` `latestSelectedCount > 0`、`:597`/`:605` `hasSelectedNow()`、`:699` `selectedCount > 0`（消费点内局部量名不同但形态同构）。grep 确认全仓 `shouldDeferSave(` 恰 4 消费点 + 1 定义点（targetGuard.ts:64-72），零增零减。
- **shouldDeferSave 纯函数逻辑**（targetGuard.ts:64-72）：`stateData===undefined` 无条件推迟 → `echoed` 已回显放行 → 否则 `courses 非空 && hasSelected` 才推迟。三判据顺序与契约 35/57 一致（清空语义与数据缺席分判：`hasSelected=false` 放行，首帧未到仍推迟）。
- **echoedRef 置位三路径**：`:200`（账号切换复位 `echoedRef.current = false` + setEchoDone(false)）、`:240`（/state 首帧 courses 空 → 置位 + setEchoDone(true)）、`:297`（回显合并完成 → 置位 + setEchoDone(true)）。首帧不置位四边界：`:229`（`if (echoedRef.current) return` 已回显短路）、`:234`（`stateData === undefined` 直接 return，绝不提前置位）、`:247`（`pubs.length === 0` return）、`:319`（独立清理 effect 首行 `!echoedRef.current || publishes.length === 0` 短路）。
- **target-guard-check.ts 18 断言全绿**：`node --import jiti/register scripts/target-guard-check.ts`，尾行「target-guard 断言全绿」，覆盖 shouldDeferSave + selectedHasStalePublish + cleanStaleSelected 三族。

**结论：M-1 第五十七轮闭合通过，四消费点三参形态与置位三路径逐字符在位，18 断言全绿。**

### 2. OBSERVE-93-01 残余面复核（第十一轮家族册） —— 维持

- **grep `<button` 全仓清点：17 处、仅 3 文件**（Admin.tsx 13 / Login.tsx 2 / Dashboard.tsx 2），残余面清单零增零减。逐处行号：Admin 417（清空生成）/ 425（复制）/ 441（刷新）/ 452（重试）/ 470（复制 aria-label="复制"）/ 473（删除 aria-label="删除"）/ 577（switch）/ 651 / 661（引擎二选一）/ 701（重试）/ 792（stats 刷新）/ 898（accounts 刷新）/ 944（logs 刷新）；Login 167（密码可见性切换）/ 299；Dashboard 67（CollapseSection 展开收起）/ 707。
- **引擎二选一 651/661 强 active 态仍在位**（`engine===... ? "border-white bg-white text-black font-medium" : "border-neutral-800 glass-input text-neutral-400 hover:text-white"`），仍是优先修复面（无 `aria-pressed`，active 态仅视觉表达）。
- **Toast.tsx:100 Close**：Radix `ToastPrimitive.Close`，`focus:outline-none` 无 focus ring，缺口仍纯 display 维度（Radix 基层按钮键盘可达）。
- icon-only 按钮均有 aria-label（Admin:470/473），文本按钮天然可读，与 OBSERVE-76-03 二勘定案措辞「缺口纯 display 维度」维持一致。

**结论：OBSERVE-93-01 残余面清单零增零减（17 处 3 文件），引擎二选一 651/661 仍为优先修复面（维持观察）。**

### 3. F93-01 第二十九轮 —— 通过

- `git log --oneline 1351fa4..HEAD -- web/`：**空输出**。
- `git diff --stat 1351fa4 HEAD -- web/`：**空**。
- Button.tsx:42 focus-visible ring 逐字符在位：`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"` 零回归。

**结论：web/ 自 1351fa4 起零代码漂移双实证成立，F93-01 修复面连续第二十九轮无回归。**

### 4. OBSERVE-115-01 弹窗族复核 —— 通过

三处裸 div 弹窗逐处核对：

| 弹窗 | role=dialog | aria-modal | aria-labelledby | Esc 关闭 | autoFocus | 焦点陷阱声明 |
|------|------------|------------|-----------------|---------|-----------|--------------|
| Select 退选 `:1201-1250` | `:1204` | `:1205` (true) | `exit-modal-title` | `:1207-1211`（退选中禁用） | 取消按钮 `:1234` | `:1199-1200` 注释 |
| Login 激活 `:223-240+` | `:226` | `:227` (true) | `activate-dialog-title` | `:232-238`（激活中禁用） | 输入框 `autoFocus`(267) | `:220-222` 注释 |
| Admin 删除 `:210-218+` | `:213` | `:214` (true) | `delete-acct-modal-title` | `:216-218`（删除中禁用） | 取消按钮 `:241` | `:208-209` 注释 |

- 三处最小语义门全部在位；无焦点陷阱均为注释声明刻意取舍（Login 注释明确「完整焦点陷阱迁移到 Radix Dialog 属后续候选」）。
- Select 退选弹窗仍为优先修复面（唯一带 Esc 三重语义门 + `aria-labelledby` 最完整，与 Login/Admin 同构，已非缺陷仅是未迁移 Radix）——维持观察。

**结论：OBSERVE-115-01 弹窗族零漂移，最小语义门与 Esc 关闭三处齐整。**

### 5. OBSERVE-116-01 跟踪项第五轮复核 —— 维持（不修）

- useTickingCountdown（lib）内自愿注释如实口径在位：`每秒 setNow 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——本注释已按实现如实口径，不再声称局部渲染）`。
- 无卡顿证据：Dashboard/Select 轮询驱动的是 3s/2s 查询级刷新，每秒 setNow 由 React 差分承担；两条路由组件树规模可控。
- 活化条件未触发：无新增性能指标/用户感知卡顿报告，维持不修。
- 「勿动轮询链路契约」确认未被动：Select `/state` 函数式 refetchInterval（`:58-81`）：error/`window_closed` → 30s、`inRange || window_opened` → 2s、否则 10s；Select `/electives`（`:148-154`）：error → 30s、`window_closed` → 30s、否则 2s。Dashboard `/state`（`:140`）与 `/logs`（`:156`）：error/`window_closed` → 30s、否则 3s；`/stats`（`:174`）30s。全站轮询节奏与上轮逐字符一致，零漂移。

**结论：OBSERVE-116-01 维持，潜在优化注释在位，轮询链路契约零漂移。**

### 6. OBSERVE-117-02 timer 类型卫生观察 —— 维持

- Select.tsx:393：`const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })`——timer 字段类型化到位，`resetRetry`（`:409-413`）与 scheduleRetry（`:420-429`）配对清零/判空中均检查 `retryState.current.timer` 非空才 clearTimeout；卸载 cleanup（`:400-408`）同步清退避 timer 并复位 unmountedRef（StrictMode 重挂载自愈）。
- `npm run build`（tsc -b）通过——纯类型卫生非缺陷确认。

**结论：OBSERVE-117-02 维持，纯类型卫生观察成立。**

### 7. 新契约角度（本轮自选）—— 目标保存链异常路径自愈纵深

选「保存链守卫命中后的自愈路径全链路」走查，确认三契约（F40-M1 解锁路径 / F42-M1 纯数据判据 / F43-M1 清空语义分判）合拢无死锁：

- **自愈主链（F42-M1 核心）**：防抖回调 `:699` 命中 shouldDeferSave → 置脏跳过不 PUT（不置 dirtyRef——终局绝不误报保存失败）→ 防抖 effect 依赖数组含 `stateData`（`:775`），/state 数据到达触发 effect 重跑 → 新 400ms timer → 守卫通过 → 落库自愈。唯一解锁不求整页刷新，与注释「判据为纯数据（shouldDeferSave 不依赖 echoedRef）… /state 数据到达触发防抖 effect 重跑自愈」逐字对应。
- **清空语义分判（F43-M1）**：`:699` 布尔实参 `selectedCount > 0`——用户显式全清空（selectedCount=0）放行 PUT []；首帧携带旧目标但用户未选 = 清空意图确凿，与回显 effect 的 `rev>0 && !anyHas` 守卫（`:256`）同判据，清空永不混判数据缺席。
- **已回显稳态放行（第三参 echoedRef）**：echoed=true 时 `courses 非空` 不再推迟——selected 已含后端旧目标，整包 PUT 与后端一致，稳态编辑不闷死（注释「已回显完成的稳态（courses 永驻非空）下防抖保存不闷死」）。
- **F40-M1 解锁双路**：① 发布重建随建随清——回显 effect 内 `:289-296`（selectedHasStalePublish 命中 → cleanStaleSelected + toast「发布已更新」）+ 独立重建清理 effect `:318-329`（echoed 后发布重建自愈，依赖含 selected 防合并交错）；② 守卫命中 toast 兜底——防抖 `:730-739` 与 flush `:525-534` 命中 stale 守卫时弹「旧批次目标已失效，已停止保存。请刷新页面重新选择」，两处均判 `!unmountedRef.current` 才弹（卸载后不轰炸）。
- **flush 链同构**：`:509` shouldDeferSave → 发布缺席守卫 `:516-519` → stale 守卫 `:525-534` → build 联查空守卫 `:539-543` → 漂移 id 守卫 `:546-550`——五道防线消费时刻串行，任一命中置脏跳过保数据，均有自愈出口（数据到达/发布恢复/用户改动重试）。
- **handleBack 首闸与等待循环**：`:597` 首闸命中 → `:605` while 循环 50ms 轮询等回显（5s 兜底）→ `:609` 等合并渲染落地，判据与防抖/flush 同源纯数据，未发现死锁窗口。

**结论：三契约在防抖/flush/handleBack 三条路径全部合拢，守卫命中均有自愈出口（stateData 到达重跑 / 发布重建清理 / 用户改动重试 / 5s 兜底），无静默死锁路径。**

### 8. 格式卫生快扫 —— 通过

- 全仓 grep 模板字符串/相邻 JSX 无半程态残留；逐段 Read 覆盖 Select/Admin/Login 主要渲染区，缩进级差一致；`(next[c.publish_id] ??= []).push(...)` 行首分号逃逸手法（Select:275）合法且带空行前置，无违和。
- 无 TODO/FIXME/XXX/HACK 残留。

## 契约 20 检查（轮次前缀标签）

grep `第 *[0-9]\+ *轮` / `轮次` / `第 N 轮` 形态：**全仓零命中**。全部注释为「为什么/契约/陷阱」本身口径（本轮通读 Select.tsx 防抖/flush/回显/弹窗/重试段注释 + targetGuard.ts 全文注释 + useTickingCountdown 注释确认）。

## 验证表

| 命令 | 结果 |
|------|------|
| `npm run build`（tsc -b + vite） | 通过，1948 modules，产物 index-1KHlpqcc.css / index-Bm7TtkV4.js |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全部通过，视觉表面协调一致 |
| `git log --oneline 1351fa4..HEAD -- web/` | 空输出（零漂移实证一） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证二） |
| grep `shouldDeferSave(` / `echoedRef` / `<button` / `focus-visible:ring` / `refetchInterval` / 轮次标签 / `role="dialog"` | 逐点比对确认，零增零减 |

## 已核无缺陷清单

- M-1 五十七轮：shouldDeferSave 四消费点三参形态逐字符在位 + echoedRef 置位三路径/首帧不置位四边界完整 + 18 断言全绿
- OBSERVE-93-01：裸按钮 17 处 3 文件零增零减、缺口纯 display、引擎二选一 651/661 强 active 态在位（维持优先修复面）
- F93-01：Button ring 逐字符在位 + web/ 零代码漂移双实证
- OBSERVE-115-01：三弹窗最小语义门 + Esc 关闭 + autoFocus 逐处在位
- OBSERVE-116-01：潜在优化注释在位、无卡顿证据、轮询契约（/state /logs 3s/30s、Select /electives 2s/10s/30s）零漂移
- OBSERVE-117-02：retryState timer 类型卫生非缺陷、构建通过
- 契约 20：轮次前缀标签全仓零命中
- 格式卫生：零半程态、零 TODO 残留

## 结论

R121 前端只读审查完成，聚焦清单 7 项全部「通过/维持」。关键复核结论：

- **M-1 第五十七轮闭合通过**——shouldDeferSave 四消费点三参形态逐字符一致，echoedRef 置位三路径（账号复位/空首帧/合并完成）与首帧不置位边界（undefined return / pubs 空 return / 已回显短路）完整，18 断言全绿。
- **OBSERVE-93-01 残余面维持**——裸按钮清单零增零减（17 处、3 文件），缺口仍纯 display 维度；引擎二选一 651/661 带强 active 态仍是优先修复面；Button.tsx:42 ring 逐字在位。
- **OBSERVE-116-01 跟踪项维持**——每秒 setNow 潜在优化注释如实口径在位、无卡顿证据、轮询链路契约未被动，第五轮维持不修。
- **OBSERVE-117-02 维持**——timer 类型卫生纯观察，构建通过非缺陷。
- **保存链自愈纵深无死锁**——F40-M1/F42-M1/F43-M1 三契约在防抖/flush/handleBack 三条路径合拢，守卫命中均有自愈出口。

连续五十八轮零严重级状态延续，web/ 自 1351fa4 起零代码漂移（R121 双实证），稳定期审查纪律未见松动。
