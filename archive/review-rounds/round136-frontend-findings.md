# R136 前端只读审查报告（web/，React 19 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `665bbb6`（R135 归档提交「docs(review): R135 双 findings + 收尾总结……进度 136/256」）。`git log --oneline 665bbb6..HEAD -- web/` 空输出、`git diff --stat 665bbb6 HEAD -- web/` 空输出——**R135 归档后 web/ 零提交、零产品改动**。全链路只读，唯一写入为本报告。

## 分级发现前置

- **CRITICAL / HIGH / MEDIUM / LOW**：**零**。无任何新分级问题。
- **观察项维持**（历史锚点零漂移）：
  - **OBSERVE-93-01**：Admin.tsx `:651/:661` 识别引擎二选一 raw `<button>` 强 active 姿态，`ui/Button` 收敛纯样式候选维持（无正确性/无障碍缺口：active/非 active 双态差异显著，视觉自明无需 aria-pressed）。
  - **R125 候选**：Select.tsx `:850` 倒计时内联 `cd.*` 未收敛 memo 叶子——被 /state 2s 轮询边际成本吸收（同 cost，零新增），性能层无正确性影响。维持记录不实现。
  - **注释口径残留（R124 起延续）**：Select.tsx `:204-207` 本地注释仍为「（潜在优化，非当前承诺）」旧口径，useTickingCountdown.ts 顶层注释已为「Dashboard 已拆 CountdownMatrix 并 memo」落地后口径——同仓库两处对同一事实口径不一致。本轮零改动延续，维持观察记录不立条。
- **新契约角度发现**：**零**。两个纵深方向均无缺口（详见下文）。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测，node --import jiti/register） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测，node --import jiti/register） |
| npm run build（tsc -b + vite） | **通过**（1948 modules transformed，built in 513ms，产物 `backend/web/dist/`，exit 0） |
| `git log 665bbb6..HEAD -- web/` | 空（F93-01 双空实证之一，R135 归档 commit 起零提交） |
| `git diff --stat 665bbb6 HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净（构建产物落 backend/web/dist，web/.gitignore 确认 dist 忽略） |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| dangerouslySetInnerHTML | **零命中**（grep 实证，含 Danger ignore-case 变体） |
| 轮次标签扫描（`"第.*轮"/(round/R1xx/R2xx`） | 零命中（grep exit 1） |
| console.log / debugger 残留 | 零命中（grep exit 1） |
| node / npm 可用性 | node 版本实测 `--import jiti/register` 运行全部脚本通过；tsx 未装、按 R135 既定用法 jiti 为准 |

## 聚焦清单逐项裁决

### 1. M-1 第七十二轮闭合（保存链守卫）— ✅ 在位（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点三参形态逐字符一致**（grep 全仓实证：定义 1 + 消费 4 + 零残留）：

| 消费点 | 位置 | 三参形态 |
|--------|------|----------|
| 定义 | targetGuard.ts:64-71 | `(stateData: SchedulerState \| undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` |
| handleBack 首闸 | Select.tsx:597 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` |
| handleBack 等待循环 | Select.tsx:605 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && ...` |
| 防抖回调守卫 | Select.tsx:699 | `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合：`stateData===undefined → true`；`echoed → false`（稳态放行）；否则 `(stateData.courses?.length ?? 0) > 0 && hasSelected`。注释 :45-63 五段式（首帧未到 / 纯数据判据与 bailout 死锁 / hasSelected 清空语义 / echoed 稳态放行不漏 / 无条件推迟保留）与实现逐条对应，零漂移。
- **echoedRef 置位三路径**（Select.tsx）：`:200`（账号切换复位 false / 插槽 key={account} 双保险）、`:240`（首帧 courses 空 → 确证后端无旧目标，置 true + setEchoDone）、`:297`（发布重建清理 toast 后置 true + setEchoDone）。
- **首帧不置位四边界**（Select.tsx）：`:229`（echoedRef 短路只合并一次）、`:234`（stateData === undefined 提前返回不置位、不 setEchoDone）、`:247`（pubs.length === 0 不置位）、`:319`（独立清理 effect 未回显不清理不置位）——行号与 R135 完全一致。
- **F40-M1 cleanStaleSelected / F43-M1 hasSelected 分判 / shouldDeferSave 纯数据判据**：
  - `selectedHasStalePublish`（targetGuard.ts:9-19）与 `cleanStaleSelected`（:26-43）——空数组 key 保留（清空语义绝不复活）、`cleanStaleSelected` 无变更返回原对象引用（原引用不触发表层重渲染）——语义与 R135 一致。
  - 假清空守卫链：flushTargets（:515/:526/:553/:558）与防抖回调（:709/:717/:732/:737）双闸六判据同序——发布缺席+已有选中 → 发布集合重建 stale 命中 + toast 兜底 → build() 联查空+选中 → targetsUseCurrentPublishes 漂移校验；守卫命中一律不置 dirtyRef（终局绝不误报保存失败）。
  - shouldDeferSave 四消费点均以「纯数据判据」驱动防抖 effect 重跑自愈（注释 :691-698 / :752-754 同步），echoed 仅作稳态放行信号。
- **flush / handleBack / 防抖三闸双闸等回显**：
  - handleBack 首闸（:597 `revRef>0 && shouldDeferSave` 才等）→ 50ms 轮询 + 5s 兜底（:605-607）+ 合并提交落地一帧（:609）；三参形态在 :605 等待循环逐字符一致。
  - flushTargets 首闸（:509）先于一切假清空守卫——/state 首帧持续失败超时后兜住置脏不 PUT，「安全方向：绝不静默丢改动」（注释 :498-508）。
  - 防抖 effect（:665-755）400ms 定时器 + `echoDone`/`stateData` 双自愈驱动依赖；`hasPublishes`（:662）布尔信号不重置防抖窗口。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts` **18 条断言全绿、exit 0**（8 条 shouldDeferSave 含「首帧未到 + 已回显标志 → 仍推迟」「courses 非空 + 有选中 + 已回显 → 放行（稳态编辑不闷死）」、5 条 selectedHasStalePublish、5 条 cleanStaleSelected 含「无变更 → 返回原引用」），输出逐条与 R135 一致。

### 2. OBSERVE-93-01 第二十六轮 — ✅ 在位（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin 13 / Dashboard 2 / Login 2），与 R135 基线完全一致。明细行号逐一比对：
- **Admin.tsx raw 13 处**：`:417`（清空生成）、`:425`（复制批量码）、`:441/:452`（激活码刷新/重试）、`:470/:473`（单码复制/删除 aria-label+title）、`:577`（激活开关 role=switch + aria-checked）、`:651/:661`（识别引擎二选一——仍为 raw `<button>` 载体、强 active 态 `border-white bg-white text-black font-medium`，`ui/Button` 收敛候选维持）、`:701/:792/:898/:944`（四 Tab 失败态重试）。
- **Dashboard.tsx raw 2 处**：`:67`（CollapseSection 折叠钮）、`:709`（/logs 重试）。
- **Login.tsx raw 2 处**：`:167`（可见性切换 aria-label + aria-pressed）、`:299`（激活弹窗「取消」按钮）。
- **651/661 候选维持**：纯样式统一候选、无正确性/无障碍缺口；active 双态（黑字白底 vs 灰字暗底）视觉差异显著，状态自明，无需 aria-pressed 补充。

### 3. F93-01 第四十四轮 — ✅ 在位（双空实证，对比基准 = R135 归档 commit 665bbb6）

- `git log --oneline 665bbb6..HEAD -- web/`：**空输出**（git 实测）。
- `git diff --stat 665bbb6 HEAD -- web/`：**空输出**（git 实测）。
- 基线确认：`git log --oneline 665bbb6 -1` = 「docs(review): R135 双 findings + 收尾总结……前端 M-1 第七十一轮闭合 + F93-01 第四十三轮双空实证…，进度 136/256」，且当前 HEAD 即 665bbb6（`git status --short --branch` = `## master` 无 ahead/behind 无未跟踪）。
- `git status --short` 工作树全净（报告写入前）；build 产物落 `backend/web/dist/`、web/.gitignore 确认 dist 忽略，未污染工作树。

### 4. OBSERVE-116-01 第二十轮 — ✅ 在位（注释口径统一，五路轮询契约零漂移）

- **注释口径**：`useTickingCountdown.ts:3-8`（「消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）……宿主因自身 props/state 无变化而快速 bail out」）与 `Dashboard.tsx:90-94`（「本文件顶部注释由…转为落地实现，注释口径同步：已拆」）、`:206-209`（「每秒变化的 cd.* 已收敛到 MemoCountdownMatrix 叶子」）、`:394-398`（主矩阵整格抽成 memo 叶子）同为落地后口径，两文件对同一事实统一，零漂移。`const MemoCountdownMatrix = memo(CountdownMatrix)`（Dashboard.tsx:117，主矩阵渲染点 :399）在位。
- **五路轮询契约逐键零漂移**（与 R135 逐行比对）：
  - Select /electives（Select.tsx:58-83）：error→30000；`st.window_closed`→30000（读 react-query 缓存 `["state", account, sessionToken]`，TDZ 注释 :72-75 自证）；`inRange || st?.window_opened`→2000；其余 10000。
  - Select /state（Select.tsx:148-155）：error / status==="error"→30000；`window_closed`→30000；其余 2000。
  - Dashboard /state（Dashboard.tsx:169-174）：error→30000；`window_closed`→30000；其余 3000。
  - Dashboard /logs（Dashboard.tsx:185-190）：error→30000；`state?.window_closed`→30000；其余 3000。
  - Dashboard /electives（Dashboard.tsx:203）：恒 30000。

### 5. OBSERVE-115-01 弹窗族 — ✅ 在位（三处最小语义门字段与行号一致）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1246 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(exitModalClass.id)`） | 「取消」:1234 |
| Login 激活 | Login.tsx:223-267 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`，三清：pendingAccount/pendingTicket/activateError） | 输入框 :267 |
| Admin 删除 | Admin.tsx:210-241 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 「取消」:241 |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关）；autoFocus 均落在最安全默认项（Select/Admin 落在「取消」，Login 落在激活输入框）。行号与 R135 完全一致（R135 后零提交），逐字段语义零漂移。三处弹窗仍为裸 div/裸 Button 补语义（真实 Radix Dialog 未使用），注释均含对话语义承诺与 Esc 补全的诚实口径，未僭越承诺。

### 6. R125 候选复核 — 维持成立（R125 首立、R126-R136 连续维持）

Select.tsx `:844`（`cd.isExpired` 分支渲染条件）/`:850`（`cd.days/hours/minutes/seconds` 内联）——路由组件 JSX 顶层裸消费，前后均未加注释（选课大厅 JSX 中无 memo 化叶子）。
- **缓解因子复核成立**：Select /state 恒 2s 轮询本就每秒级触发本路由组件重渲染，tick 的 1s setNow 相对零新增语义成本；且 props（account/sessionToken）在会话内恒定、宿主 bail out 路径完整。性能层无正确性影响。
- **守卫盲区复核**：`perf-countdown-guard.ts` 第三断言正则 `cd.(days|hours|minutes|seconds)` 在 Dashboard 侧被 `MemoCountdownMatrix` 关键字过滤后 rawCdUse=0（绿灯实测）；Select.tsx 不在守卫扫描范围（守卫注释明确承诺范围 = 「路由组件（Dashboard）顶层」），结构断言与实现契约一致，无守卫漂移。
- **本轮是否立条**：维持候选记录（与 R125-R135 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1256 行函数体重渲染；若落地需同步扩守卫 + Select.tsx:204-207 注释口径升级，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。
- **同文件注释口径残留（R124 起延续）**：Select.tsx:204-207 本地注释仍停在「（潜在优化，非当前承诺）」口径，加载于组件函数体内顶部——R125 起持续记录维持观察，不立条。

### 7. 新契约角度（自选纵深 ×2）

本轮选「flush 出口收敛路径」与「手动操作在飞守卫全路径」两个方向（nisan 上乘 M-1 保存链守卫总纲，补全出口侧与操作侧的既有契约盘点）。

**纵深 A：保存链 flush 出口收敛全路径复核（无缺口）**

- 三闸 → 串行化出口单链：flushTargets（:488-567）→ saveNow（:430-467）→ 飞行中 only 置 dirty、finally 补发（:462-465）+ lastJson 去重（:436）；handleBack（:575-655）三轮 flush + 同步卸载前等静止（pendingSaving 三信号:633）。
- 移除双路径保序：`targetRef/lastJson` 单调推进，`dirtyRef` 在 finally 补发前置位后清（:462）——先新后旧写序保证，无握手丢失窗口。
- 对齐记账核对：flush/防抖的同款守卫分支（:509/:515/:526/:553/:558 与 :699/:709/:717/:732/:737）十二处 return 全部「不置 dirtyRef」——圆柱校对无缺（终局 toast 双判 `revRef>0 && pendingUnsaved()` :647 只对真实失败）。unmount 三钳（:431/:442/:459）在 saveNow 出口全门。
- **缺口判断**：无 1 类缺口，无 2 类冗余出口。flush/防抖所有入口兜入串行化单链 + setSelected 镜像（selectedRef/revRef，:173-176）消费时刻读取语义正确（防抖 effect 闭包内 build 读渲染期 selected 为合法特权，保存链落库 targetRef 已正序）。

**纵深 B：手动操作在飞守卫全路径复核（无缺口）**

- 课程级 Set（Select.tsx:49-52）：入口双短路为报名/退选导出同一动作守卫（:92/:119 → `if (actionLoading.has(c.id)) return`），disabled 渲染（:1120/:1133）与 ACTION in-flight 共用；finally 函数式只删自己 id 不清他人（:106-110/:128-132）。
- 弹窗级：退选 Esc（:1208）/取消（:1232）/退出确认（:1242）共用 `actionLoading.has(exitModalClass.id)` 防误关；Login 激活（activating 布尔 + `if (activating) return` :68 + disabled :266/:282/:307）；Admin 删除（`if (deleting) return` :252 + 双 disabled :239/:248）、批量码删除 `removing.has(code)` 入口（:349）+ disabled（:473）+ finally 函数式清除（:354-360）。
- 跨面一致性复核：Login submit（`if (loading) return` :34）与 activate 双守卫拓扑同构；Admin removing Set 与 Select actionLoading Set 同构；三族均保证「第二次点击直接短路、不落网络」。
- **缺口判断**：无。所有副作用按钮（报名/退选/激活/删除/单码删除/登录提交）均证明在飞守卫 + finally 对称清除，未发现遗漏副作用入口。

**XSS/注入面巡回复核（无新增面）**：全站用户输入（课程名/教师名/账号名/错误消息）经 React JSX 自动转义渲染，`dangerouslySetInnerHTML` 全仓**零命中**（含 `Danger` 大小写变体），无反射/存储型 XSS 注入点。无安全发现。

## 维持观察项（不立条）

| 项 | 来源 | 当前状态 |
|----|------|----------|
| Admin.tsx:651/:661 引擎二选一非 `ui/Button` 收敛候选 | OBSERVE-93-01 | 维持（纯样式候选，无正确性/无障碍缺口） |
| Select.tsx:850 内联 cd.* 未收敛 memo 叶子 | R125 候选 | 维持（被 2s 轮询边际成本吸收，性能层无正确性影响） |
| Select.tsx:204-207 注释口径残留 vs useTickingCountdown.ts 顶层注释 | R124 起延续 | 维持观察记录（同事实不同口径，零改动延续，不立条） |

## 测试证据抽查（实测输出关键行）

- target-guard-check：`✓ 首帧未到 + 已回显标志 → 仍推迟（回显未发生整包覆盖）`、`✓ courses 非空 + 有选中 + 已回显 → 放行（稳态编辑不闷死）`、`✓ 无变更 → 返回原引用`……后跟 `target-guard 断言全绿`，exit=0。
- perf-countdown-guard：`✓ 路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）`……`断言全绿`，exit=0。
- admin-auth-check：6 条全绿（含「撞名学生 token ≠ 管理 token → 学生端」「自定义管理员名 token 一致 → 恢复」），exit=0。
- unauthorized-check：5 条全绿（含「非 query 段 account= → 空串」），exit=0。
- npm run build：`✓ built in 513ms`，产物 index-*.js 420.90 kB，exit=0。

## 全仓库文件改动

零（除构建产物 `backend/web/dist/`（gitignore）与本报告）。

## 结语

R135 归档 commit 665bbb6 即当前 HEAD——web/ 零提交、零产品改动，七项必查全部在位且零漂移，四守卫全绿（18/3/6/5），构建通过（1948 modules，513ms），工作树全净。本轮为**延续性确认轮**：所有历史锚点（M-1 / OBSERVE-93-01 / F93-01 / OBSERVE-116-01 / OBSERVE-115-01 / R125 候选）持续闭合，无新增分级发现。新契约角度（flush 出口收敛 + 手动操作在飞守卫全路径 + XSS 面巡回）复核无缺口。

**结论：APPROVE**