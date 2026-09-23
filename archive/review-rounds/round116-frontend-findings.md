# R116 前端只读审查报告

## 头部

- **审查对象**：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto\web/`（React 18 + Vite + TS + Radix UI + Tailwind），HEAD `9c7a70b`
- **审查方式**：全程只读（grep 全仓清点 / 逐字比对 / `npm run build` tsc -b 真校验 / 四个前端验证脚本 / `git log`+`git diff` 漂移双实证）
- **聚焦范围**：M-1 第五十二轮闭合复核、OBSERVE-93-01 残余面第六轮复核、F93-01 第二十四轮、OBSERVE-115-01 弹窗族并入 O-3 族册后首轮、OBSERVE-112-02 react-query refetchInterval、无障碍纵深延续、新契约角度（自选：轮询资源账与定时器清点）、格式卫生快扫
- **耗时**：约 30 分钟（合规）

## 分级发现

无 CRITICAL / MAJOR / MINOR。OBSERVE 级 3 项（2 延续 + 1 新）。

### OBSERVE（延续）

- **OBSERVE-93-01 残余面（第六轮家族册复核）——维持**：7 个带文本裸按钮 + Toast Close 共 8 点位，全部键盘链路完整（可 Tab 聚焦、可激活、带 hover/focus 反馈），缺口仍纯 display 维度（无 `focus-visible` 可见焦点环），非功能性缺陷。651/661 引擎二选一带强 active 态仍是优先修复面。点位清单与本轮 grep 清点完全一致，零增零减。详见必查项 2。
- **OBSERVE-115-01 弹窗族（并入 O-3 族册后首轮）——维持**：三处裸 div 弹窗（Select 退选 / Login 激活 / Admin 删除）最小语义门逐处确认在位、零漂移；无焦点陷阱为注释声明的刻意取舍（F6-02 迁移方向）；Select 退选弹窗仍为优先修复面。详见必查项 4。

### OBSERVE（本轮新增）

- **OBSERVE-116-01：`refetchInterval` 函数式回调 + `useTickingCountdown` 每秒 setNow 双层驱动整树重渲染**。Dashboard/Select 的 /state 与 /logs 轮询间隔依赖 `query.state`（非闭包，正确）或闭包 `state`（依赖 /state 3s 刷新驱动重渲染→react-query 用最新闭包重调度，注释已如实声明且正确）；`useTickingCountdown` 每秒 `setNow` 触发宿主路由整树重渲染——两层叠加使 Dashboard 在窗口开放期每分钟约 80 次重渲染（60 次 tick + ~20 次轮询）。已注释为"潜在优化，非当前承诺"，且 DOM 差分成本可忽略，故非缺陷。**跟踪项**：若未来窗口开放期出现任何卡顿度量，第一候选即"拆分倒计时为 memo 叶子组件 + 折叠行相对时间改独立低频 tick"，勿动轮询链路（轮询契约经 B41/F52 系列钉死）。

## 必查项逐条结论

### 1. M-1 第五十二轮闭合复核 —— 通过

- **shouldDeferSave 四消费点逐字符比对**（第三参全为 `echoedRef.current`）：
  - `Select.tsx:509`：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` ✓
  - `Select.tsx:597`：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` ✓
  - `Select.tsx:605`：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`（while 轮询条件）✓
  - `Select.tsx:699`：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` ✓
- `shouldDeferSave(` grep 恰 4 消费点（均带第三参），另仅定义处 1 处 + 注释引用。`echoedRef.current` 运行时读点 9 处（:200/:229/:240/:297/:319/:509/:597/:605/:699），置位 3 路径（:200 账号复位置 false / :240 courses 空置 true / :297 合并完成置 true），首帧不置位边界（:234 `stateData===undefined` return / :247 `pubs.length===0` return / :229 已回显短路 / :319 清理 effect 未回显短路）全部在位。
- `targetGuard.ts:64-72` shouldDeferSave 纯函数与 F43-M1 语义注释一致（首帧未到无条件推迟 → echoed 放行 → courses 非空 && hasSelected 才推迟）。
- `node --import jiti/register scripts/target-guard-check.ts` → **18 断言全绿**（含"首帧未到 + 已回显标志 → 仍推迟"边界断言）。
- M-1 第五十二轮复核结论：**零漂移，闭合维持**。

### 2. OBSERVE-93-01 残余面复核（第六轮家族册）—— 维持

- 带文本裸按钮 7 处（Admin `:417` 收起 / `:425` 复制 / `:441` 刷新 / `:452` 重试 / `:651` 引擎 Vision / `:661` 引擎 ddddocr / `:701` 重试）+ Toast.tsx:100 Close（Radix 内建隐式 aria-label），逐点位走查：
  - 全部原生 `<button>` 可 Tab 聚焦；`hover:text-white` 等 hover 反馈在手写 className；**焦点反馈仅浏览器默认 dotted ring（global.css 对 button 统一 outline:none 抹除）——缺口纯 display 维度**。
  - `:651/:661` 引擎二选一带强 active 态（`border-white bg-white text-black font-medium` vs `border-neutral-800 glass-input text-neutral-400`），aria-pressed 缺失，仍为残余面中"选中态对读屏不可知"的最优先修复面。
  - `:425/:441/:452/:701/:417` 为低频管理操作/icon-only（470/473 有 aria-label/title），风险低。
- `grep "<button"` 全仓清点 23 处：Admin 13 + Dashboard 2 + Login 2 + Select 0（Select 全走 Button 组件）+ ui 组件 6（Button/Tabs/Toast/Progress）。裸 button 分布与上轮家族册完全一致，**残余面清单零增零减**。
- **OBSERVE-76-03 二勘定案措辞复核确认**：Toast.tsx:100 Close 无自动注入标签（Radix 内建隐式 aria-label，复读/混读时无可访问名），按钮无可访问名。R113/R114 勘校结论与源码现状一致。
- **F93-01 修复面复核**：`Button.tsx:42` `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"` 逐字符在位，零回归。注释（global.css outline 抹除补偿、focus-visible 语义）在位。

### 3. F93-01 第二十四轮 —— 通过

- `Button.tsx:42` focus-visible ring 逐字符在位（见上）。
- `git log --oneline 1351fa4..HEAD -- web/` → 空输出。
- `git diff --stat 1351fa4 HEAD -- web/` → 空输出。
- **web/ 零代码漂移双实证**（自 1351fa4 起前端无任何产品代码变更）。R115 轮对 web/ 零改动（`git diff --stat 4c32a81 9c7a70b -- web/` 空，纯 docs 轮）。

### 4. OBSERVE-115-01 弹窗族复核（并入 O-3 族册后首轮）—— 维持

三处裸 div 弹窗最小语义门逐处核对：

| 弹窗 | role=dialog | aria-modal | aria-labelledby | Esc+在飞守卫 | autoFocus | 焦点陷阱注释 |
|------|------------|-----------|----------------|-------------|-----------|-------------|
| Select 退选 (:1204) | ✓ | ✓ | ✓ (exit-modal-title) | ✓ `!actionLoading.has(id)` | ✓ 取消按钮 :1234 | ✓ |
| Login 激活 (:226) | ✓ | ✓ | ✓ (activate-dialog-title) | ✓ `!activating` | ✓ :267 | ✓ |
| Admin 删除 (:213) | ✓ | ✓ | ✓ (delete-acct-modal-title) | ✓ `!deleting` | ✓ 取消按钮 :241 | ✓ |

- 三处标题 `id` 均唯一（无重复 id 冲突）。
- 无焦点陷阱（Tab 可穿出到背景）为注释声明的刻意取舍（F6-02 迁移到 Radix Dialog 方向），非隐藏缺口。
- Select 退选弹窗 `onKeyDown` Esc 与取消按钮同逻辑（:1207-1211），退选中 `actionLoading` 不响应防误关；确认按钮 `handleConfirmExit` 带在飞守卫（:119）+ finally 清在飞标记 + invalidateQueries 双失效（electives/state）——**仍为优先修复面**（三个弹窗共用一套模式，修一个模板其余照抄）。
- **零漂移确认**：三处弹窗语义门、Esc、autoFocus 与 O-3 族册记录逐字一致。

### 5. OBSERVE-112-02 复核 —— 通过

- `Dashboard.tsx:140-146` /state 的 `refetchInterval: (query) => query.state.error || query.state.status === "error" ? 30000 : query.state.data?.window_closed ? 30000 : 3000`——从 `query.state`（react-query 传入的查询对象）读取，**非组件闭包 state，正确**；失败态降 30s 契约在位（注释链 :127-139 完整）。
- `/logs` 轮询（:156-161）读闭包 `state?.window_closed`——依赖 /state 3s 刷新驱动重渲染、react-query 用最新闭包重调度，与注释声明（"不存在闭包停旧值永不降频"）一致。窗口关闭瞬间 /state 先返回 true、下一次日志轮询即降 30s，时序成立。
- Select 侧 /state（:148-155）同为函数式，`query.state.error` 降 30s / `window_closed` 降 30s / 升频 2s 三态在位上轮已核。本轮确认零漂移。

### 6. 无障碍纵深延续 —— 通过

- **role="alert" 三处**：Dashboard :320（会话失效错误条）/ Login :182（主表单错误）/ Login :273（激活错误）全部在位，且 `role="alert"` 挂 div 上（role 不覆盖原生 div 语义，正确）。
- **同款 ring 令牌三族**：Tabs :29（`focus-visible:ring-2 focus-visible:ring-[var(--cyan)] ring-offset`）/ Admin switch :583（`focus-visible:ring-2 focus-visible:ring-white/60`）/ Login 密码切换 :173（同款 white/60）在位。Button :42 主 ring 令牌（cyan + offset）在位。
- **可选新角度（OBSERVE-116-01 已并入必查项 7 定时器清点）**。

### 7. 新契约角度（自选：轮询资源账与定时器清点）—— 通过（含 1 新观察）

全仓定时器/轮询资源逐项清点：

| 定时器 | 位置 | 清理 | 结论 |
|--------|------|------|------|
| `api/client.ts:56` fetch 20s abort | 函数内超时 | ctrl.abort 自然消亡 | ✓ |
| `useTickingCountdown` setInterval(1000) | lib:13 | effect cleanup clearInterval | ✓ |
| Select `retryState` 退避 setTimeout | :425 | cleanup :404 + resetRetry | ✓ |
| Select 5s 等待轮询 | :606 | 循环内 promise | ✓ |
| Select handleBack 三轮收敛轮询 | :609/:623/:637 | 有界循环 | ✓ |
| Select 防抖 setTimeout | :684 | effect cleanup :747 | ✓ |
| Admin copyTimer setTimeout | :81/:102 | cleanup :116 | ✓ |
| **Admin `pendingDelete`/`deleting` 确认弹窗：确认按钮 onClick 未做 async 幂等短路** | :249 | — | 见下方分析 |

- 定时器全部有界/有清理，零泄漏。每账号 `/state`+`/logs` 恒 2 个轮询查询 + `/electives` 1 个，窗口开放期 3s 间隔，资源账守恒。
- **新观察（display 级）**：Admin 删除确认按钮 onClick（:249-276）内有 `if (deleting) return` 守卫 + 按钮 `disabled={deleting}`，与项目幂等族（Login submit/activate、Select 报名/退选）同构，**守卫完整**；唯一差异是 Login `submit()` 用 `if (loading) return`、Select `handleSelectClass` 用 `if (actionLoading.has(c.id)) return`、Admin 用 `if (deleting) return`——三族在飞守卫形式一致，此处分析无缺陷，仅记录核对过程。

### 8. 格式卫生快扫 —— 通过

- 逐文件扫描未发现相邻 JSX 同行/缩进级差半程态。抽查的 JSX 块（Admin 激活码区、Dashboard 分组卡片、Select 课程卡片、App 路由分支）缩进一致、闭合完整。
- 全仓 `git status --short` 干净，无工作树残留。

## 契约 20（轮次前缀标签）检查 —— 通过

grep `第\s*\d+\s*轮|(第|]\(R\d+|F\d+-|M\d+-|C\d+-|B\d+-` → **零命中**。代码注释全部为"为什么/契约/陷阱"语体（如 "aria-controls 指向的 id 此前是硬编码"、"ddddocr 走本机 Python"），无轮次锚点残留。发现 0 处。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 1351fa4..HEAD -- web/` | 空（零漂移实证 1） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证 2） |
| `git diff --stat 4c32a81 9c7a70b -- web/` | 空（R115 纯 docs 轮） |
| `git status --short` | 干净 |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全绿（视觉表面一致性） |
| `npm run build`（tsc -b + vite build） | 通过，1948 模块，399ms，dist 落 backend/web/dist |
| grep `shouldDeferSave(` | 恰 4 消费点 + 1 定义 |
| grep `echoedRef.current` | 运行时读点 9 处 + 注释引用，置位 3 路径，边界完整 |
| grep `<button` | 全仓 23 处，残余面清单零增零减 |
| grep 轮次前缀标签 | 零命中 |

## 已核无缺陷清单

- M-1 保存链守卫五连（shouldDeferSave 4 消费点 + targetGuard 纯函数）——零漂移。
- echoedRef 生命周期（3 置位 + 4 首帧边界 + 账号复位守卫）——完整。
- F93-01 Button ring 令牌——逐字符在位。
- 三弹窗最小语义门——全部在位、Esc/autoFocus/在飞守卫完整。
- Dashboard/Select 双 refetchInterval 函数式回调——query.state 非闭包 + 闭包读依赖重渲染正确。
- role="alert" 三处、ring 令牌三族、Button 主令牌——在位。
- 全仓定时器——有界/有清理，零泄漏。
- 契约 20 轮次标签——零残留。
- 格式卫生——零半程态。

## 结论

- **M-1 第五十二轮**：**闭合维持**。四消费点第三参逐字符一致、grep 清点精确、18 断言全绿，保存链守卫族零漂移。
- **OBSERVE-93-01 残余面**：**维持**。8 点位键盘链路完整、缺口纯 display 维度；残余面清单零增零减；651/661 引擎二选一仍为优先修复面；OBSERVE-76-03 二勘定案措辞与源码现状吻合。
- **OBSERVE-115-01 弹窗族**：**维持**。三处最小语义门逐位在位、Esc/autoFocus/在飞守卫完整、焦点陷阱为注释声明取舍；Select 退选弹窗仍为优先修复面。
- 本轮 0 CRITICAL/MAJOR/MINOR，1 新 OBSERVE（OBSERVE-116-01 双层重渲染跟踪项）。连续六十一轮零严重级状态延续。
