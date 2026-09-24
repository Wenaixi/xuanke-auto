# R138 前端只读审查报告（web/，React 19 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `03ad979`（R137 归档提交「docs(review): R137 双 findings + 收尾总结……进度 138/256」）。实测 **HEAD 恰为 03ad979**（`## master` 无 ahead/behind、无未跟踪），`git log --oneline 03ad979..HEAD -- web/` 空输出、`git diff --stat 03ad979 HEAD -- web/` 空输出——**R137 归档后 web/ 零提交、零产品改动**。全链路只读，唯一写入为本报告。

## 分级发现前置

- **CRITICAL / HIGH / MEDIUM / LOW**：**零**。无任何新分级问题。四守卫全部实测绿、构建通过、工作区零漂移。
- **观察项维持**（历史锚点零漂移）：
  - **OBSERVE-93-01**：Admin.tsx `:651/:661` 识别引擎二选一 raw `<button>` 强 active 姿态，`ui/Button` 收敛纯样式候选维持（无正确性/无障碍缺口：active/非 active 双态差异显著，视觉自明无需 aria-pressed）。
  - **R125 候选**：Select.tsx `:850` 倒计时内联 `cd.*` 未收敛 memo 叶子——被 /state 2s 轮询边际成本吸收（同 cost，零新增），性能层无正确性影响。维持记录不实现。
  - **注释口径残留（R124 起延续）**：Select.tsx `:204-207` 本地注释仍为「（潜在优化，非当前承诺）」旧口径，useTickingCountdown.ts 顶层注释已为「Dashboard 已拆 CountdownMatrix 并 memo」落地后口径——同仓库两处对同一事实口径不一致。本轮零改动延续，维持观察记录不立条。
- **新契约角度发现**：**零**。两个纵深方向均无缺口（详见下文）。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测，`node v24.4.1 --import jiti/register scripts/target-guard-check.ts`） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测，node --import jiti/register） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测，node --import jiti/register） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测，node --import jiti/register） |
| npm run build（tsc -b + vite，vite v8.3.0） | **通过**（1948 modules transformed，built in 1.21s，产物 `backend/web/dist/` index-27qti0_B.js 420.90 kB，exit 0） |
| `git log 03ad979..HEAD -- web/` | 空（F93-01 双空实证之一，R137 归档 commit 起零提交） |
| `git diff --stat 03ad979 HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净（**0 条目**；构建产物落 backend/web/dist、`git check-ignore` 实证忽略；初次 `npm i` 补齐 node_modules 未产生 package-lock.json 提交面） |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减，明细行号逐一与 R137 基线比对一致 |
| dangerouslySetInnerHTML | **零命中**（grep 实测，含大小写变体 `dangerouslySetInnerHTML`/`innerHTML=`） |
| 轮次标签扫描（`第[一-十0-9]+轮`/`R1[0-9]{2}` 精确格式） | 零命中（grep exit 1，web/src 含 scripts/） |
| console.log / debugger / test.skip / .only 残留 | 零命中（目标守卫脚本内 console.log 为断言输出，非产品代码残留；无 test.skip/.only 形态） |
| node / npm 可用性 | node v24.4.1 实测；tsx 未装、jiti 为既定用法（脚本头注释自证），运行全部通过 |

## 聚焦清单逐项裁决

### 1. M-1 第七十四轮闭合（保存链守卫）— ✅ 在位（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点三参形态逐字符一致**（grep 全仓实证：定义 1 + 消费 4，`grep -c "shouldDeferSave("` = 4）：

| 消费点 | 位置 | 三参形态 |
|--------|------|----------|
| 定义 | targetGuard.ts:64-71 | `(stateData: SchedulerState \| undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` |
| handleBack 首闸 | Select.tsx:597 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`（前有 `revRef.current > 0 &&`） |
| handleBack 等待循环 | Select.tsx:605 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline` |
| 防抖回调守卫 | Select.tsx:699 | `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合：`stateData===undefined → true`（无条件推迟，回显未发生整包覆盖删除后端旧目标）；`echoed → false`（稳态放行，selected 已含后端旧目标、整包 PUT 与后端一致，编辑绝不闷死）；否则 `(stateData.courses?.length ?? 0) > 0 && hasSelected`（首帧携带旧目标且用户有选中 = 回显未完成才推迟；全清空 hasSelected=false 放行 PUT []，清空语义绝不与慢首帧混判）。注释 :45-63 五段式动机（首帧未到 / 纯数据判据与 bailout 死锁自愈 / hasSelected 清空分判 / echoed 稳态放行不漏 / 无条件推迟保留）与实现逐条对应。
- **echoedRef 置位三路径**（Select.tsx）：`:200`（账号切换复位 false，兜底 key={account} 整体重建）、`:240`（首帧 courses 空 → 确证后端无旧目标，置 true + setEchoDone(true)）、`:297`（发布重建清理 toast 后置 true + setEchoDone(true)）。grep 全量确认无第四处写 true。
- **首帧不置位四边界**（Select.tsx）：`:229`（echoedRef 短路只合并一次）、`:234`（stateData===undefined 提前 return，绝不提前置位回显完成——/state 晚于用户首次点击到达时防抖/flush 不会被误放行）、`:247`（pubs.length===0 return 不置位）、`:319`（独立清理 effect 未回显不清理不置位）——行号与 R137 完全一致。
- **F40-M1 cleanStaleSelected / F43-M1 hasSelected 清空分判**：`cleanStaleSelected`（targetGuard.ts:26-43）只删「非空且不在当前发布集合」key、空数组 key 保留（清空语义绝不复活）、无变更返回原对象引用（:42 `return changed ? next : selected`），断言脚本场景 J「无变更 → 返回原引用」实测绿；消费点（Select.tsx:290/:322 回显/独立双 effect）随发布重建清理 + stale 守卫命中 toast「发布已更新」（:291-295/:323-327，卸载后不轰炸以 `!unmountedRef.current` 判）。`shouldDeferSave` 第二参数 `hasSelected`（latestSelectedCount>0 / hasSelectedNow() / selectedCount>0）在四消费点全量承担清空分判。
- **flush / handleBack / 防抖三闸双闸等回显**：
  - handleBack 首闸（:597）`revRef>0 && shouldDeferSave` 才进等待 → 50ms 轮询 + 5s 兜底（:605-607）+ 合并提交落地一帧（:609）→ 三连 flush（:611-642）等 savingRef/dirtyRef/退避 timer 静止后 onDone。纯浏览（rev===0）不等待（:588-589 注释语义）。
  - flushTargets 首闸（:509）先于一切假清空守卫——/state 首帧持续失败超时后兜住置脏不 PUT（守卫不置 dirtyRef，终局绝不误报保存失败，:510 注释）；后续三守卫（发布缺席 :515 / stale :526 / 联查空 :553 / 漂移 :558）同序成立于。
  - 防抖 effect（:665-755）400ms 定时器，依赖 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`（:755）——`stateData` 与 `echoDone` 双自愈驱动（守卫命中置脏后 selected bailout 不重跑，/state 到达/回显完成触发 effect 重跑挂新 timer）；`hasPublishes`（:662）布尔信号在发布空→非空时重跑不重置 400ms 窗口。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts` **18 条断言全绿、exit 0**（8 条 shouldDeferSave 含「首帧未到 + 已回显标志 → 仍推迟」「courses 非空 + 有选中 + 已回显 → 放行（稳态编辑不闷死）」、5 条 selectedHasStalePublish 含「空数组 key = 用户清空 → 放行」、5 条 cleanStaleSelected 含「无变更 → 返回原引用」），输出逐条与 R137 一致。

### 2. OBSERVE-93-01 第二十八轮 — ✅ 在位（零增零减）

`grep -n "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin 13 / Dashboard 2 / Login 2），与 R137 基线完全一致。明细：
- **Admin.tsx raw 13 处**：`:417`（清空生成）、`:425`（复制批量码）、`:441/:452`（激活码刷新/重试）、`:470/:473`（单码复制/删除 aria-label+title）、`:577/:579`（激活开关 role=switch + aria-checked）、`:651/:661`（识别引擎二选一——仍为 raw `<button>` 载体、强 active 态 `border-white bg-white text-black font-medium`，`ui/Button` 收敛候选维持）、`:701/:792/:898/:944`（四 Tab 失败态重试）。
- **Dashboard.tsx raw 2 处**：`:67`（CollapseSection 折叠钮 aria-expanded/aria-controls）、`:709`（/logs 重试）。
- **Login.tsx raw 2 处**：`:167`（可见性切换 aria-label + aria-pressed）、`:299`（激活弹窗「取消」按钮）。
- **651/661 候选维持**：纯样式统一候选、无正确性/无障碍缺口；active 双态（黑字白底 vs 灰字暗底）视觉差异显著，状态自明。

### 3. F93-01 第四十六轮 — ✅ 在位（双空实证，对比基准 = R137 归档 commit 03ad979）

- `git log --oneline 03ad979..HEAD -- web/`：**空输出**（git 实测）。
- `git diff --stat 03ad979 HEAD -- web/`：**空输出**（git 实测）。
- 基线确认：`git log --oneline -2` = 03ad979（R137 归档）→ 0cf543a（R136 归档），且 HEAD 即 03ad979（`## master` 无 ahead/behind）。
- `git status --short --porcelain | wc -l` = **0**（报告写入前）；web/.gitignore 确认 dist 忽略、`git check-ignore backend/web/dist` 命中，`npm i` 补 node_modules 未进入提交面。

### 4. OBSERVE-116-01 第二十二轮 — ✅ 在位（注释口径统一，五路轮询契约零漂移）

- **注释口径**：`useTickingCountdown.ts:3-8`（「消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）……宿主因自身 props/state 无变化而快速 bail out」）与 `Dashboard.tsx:90-94`（「本文件顶部注释由「拆 memo 叶子组件潜在优化」转为落地实现，注释口径同步」）、`:206-211`（「每秒变化的 cd.* 已收敛到 MemoCountdownMatrix 叶子」）、`:394-398`（主矩阵整格抽成 memo 叶子）同为落地后口径，两文件对同一事实统一。`const MemoCountdownMatrix = memo(CountdownMatrix)`（Dashboard.tsx:117）在位，主矩阵渲染点 :399。
- **五路轮询契约逐键零漂移**（与 R137 逐行比对）：
  - Select /electives（Select.tsx:58-83）：error→30000；`st.window_closed`→30000（读 react-query 缓存 `["state", account, sessionToken]`，TDZ 注释 :72-75 自证）；`inRange || st?.window_opened`→2000；其余 10000。
  - Select /state（Select.tsx:148-155）：error→30000；`window_closed`→30000；其余 2000。
  - Dashboard /state（Dashboard.tsx:169-174）：error→30000；`window_closed`→30000；其余 3000。
  - Dashboard /logs（Dashboard.tsx:185-190）：error→30000；`state?.window_closed`→30000；其余 3000。
  - Dashboard /electives（Dashboard.tsx:203）：恒 30000。

### 5. OBSERVE-115-01 弹窗族 — ✅ 在位（三处最小语义门行号与 R137 一致）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1246 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(exitModalClass.id)`） | 「取消」:1234 |
| Login 激活 | Login.tsx:223-267 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`，三清：pendingAccount/pendingTicket/activateError） | 输入框 :267 |
| Admin 删除 | Admin.tsx:210-241 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 「取消」:241 |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关）；autoFocus 均落在最安全默认项。行号与 R137 完全一致（R137 后零提交），逐字段语义零漂移。

### 6. R125 候选复核 — 维持成立（R125 首立、R126-R138 连续维持）

Select.tsx `:844`（`cd.isExpired` 分支渲染条件）/`:850`（`cd.days/hours/minutes/seconds` 内联）——路由组件 JSX 顶层裸消费。
- **缓解因子复核成立**：Select /state 恒 2s 轮询本就每秒级触发本路由组件重渲染，tick 的 1s setNow 相对零新增语义成本；props（account/sessionToken）会话内恒定、宿主 bail out 路径完整。性能层无正确性影响。
- **守卫盲区复核**：`perf-countdown-guard.ts` 第三断言正则 `cd.(days|hours|minutes|seconds)` 在 Dashboard 侧被 `MemoCountdownMatrix` 关键字过滤后 rawCdUse=0（绿灯实测，当前 0 处应为 0）；Select.tsx 不在守卫扫描范围（守卫注释明确承诺范围 = 「路由组件（Dashboard）顶层」），结构断言与实现契约一致，无守卫漂移。
- **本轮是否立条**：维持候选记录（与 R126-R137 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1256 行函数体重渲染；若落地需同步扩守卫 + Select.tsx:204-207 注释口径升级，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。
- **同文件注释口径残留（R124 起延续）**：Select.tsx:204-207 本地注释仍停在「（潜在优化，非当前承诺）」口径，加载于组件函数体内顶部——R125 起持续记录维持观察，不立条。

### 7. 新契约角度（自选纵深 ×2）

本轮选「倒计时双端有效性纵深（F39-N1 begin_times 兜底在 Select/Dashboard 的双重实现）」与「401 吊销切号整体重建纵深（key={account} + 账号切换复位）」两个方向（纵向上承 R137 注释口径清澈性与 R135 会话态流转，聚焦用户最敏感的「开窗瞬间的时间展示正确性」与「多账号切换的会话隔离」）。

**纵深 A：倒计时 begin_times 兜底（F39-N1）双端逐字符复核（无缺口）**

F39-N1 契约原文承诺「识别缺席时主倒计时输入用 `begin_times[0]` 兜底（唯一未来开窗点），绝不显示编造时间」，实测两处实现：

- **Dashboard.tsx:217-227**：`openTimeStr = state?.open_time_known && state.open_time ? state.open_time : null`（识别缺席 = 未知语义）→ `useTickingCountdown(openTimeStr ?? (electives?.begin_times?.[0] != null ? new Date(electives.begin_times[0]).toISOString() : null))`。识别槽建立后识别真值优先，兜底只在识别缺席时生效。右下文案行（Dashboard.tsx:410-416）与主矩阵同源吃兜底（`openTimeStr ? ... : electives?.begin_times?.[0] != null ? ... : state ? "未识别到开放时间" : "正在同步教务平台时间配置..."`）——矩阵倒数、文案不再「未识别到开放时间」自相矛盾，承诺在文案行真的落地（注释 :406-409 自证）。`primaryMs` 与 `extrasMs`（:237-243）同样以 openTimeStr→begin_times[0] 为主时间，其余 begin_times 排入「其他开放时间」折叠段（:429-457），主时间绝不与 begin_times 合并（识别态唯一事实源，识别与纯平台数组同频存在时以识别真值为准，无重复展示）。
- **Select.tsx:759-772**：`openTimeStr = stateData?.open_time_known && stateData.open_time ? stateData.open_time : null` → `useTickingCountdown(openTimeStr ?? (data?.begin_times?.[0] != null ? new Date(data.begin_times[0]).toISOString() : null))`——与 Dashboard 同构逐字符一致。顶栏横幅（Select.tsx:827-853）五态分支（同步中 → 已关闭 → 已开放 → 未识别 → 本地已到点 → 倒计时）以 `window_closed`/`window_opened` 服务端信号优先，`cd.isExpired` 只表示「本地倒计时走到 0」不僭越窗口状态（注释 :821-826 自证）；右侧绝对时间（:855-861）同源吃兜底。
- **useTickingCountdown 算法层（:23-25）**：`target ? new Date(target).getTime() - now : 0`，`diff<=0` 全 00 + isExpired=true——兜底传 `new Date(begin_times[0]).toISOString()`（毫秒戳转 ISO），两处消费点统一格式，零解析错位。
- **缺口判断**：无。双端同一套「识别真值优先 → begin_times[0] 兜底 → 全空回落未知」三阶链，主矩阵/折叠列表/文案行/横幅绝对时间四位同源，不存在「主矩阵全 00 过期态与『预计开放时间』未来时刻自相矛盾」的旧症（R124 承诺闭环）。

**纵深 B：401 吊销切号整体重建（key={account} + 账号切换复位）逐路径清点（无缺口）**

- **挂载点 key={account}**（App.tsx:293 代理 Select / :340 学生 Select）——账号切换即整体重建实例（selected/echoedRef/rev 全复位），跨账号零状态残留。`onDone` 回 Dashboard 时 `key` 属路由组件间距 semantics 无影响（Select 卸载，queryClient 缓存按 key `["electives", account, sessionToken]` 与 `["state", account, sessionToken]` 隔离，切回原账号复用其缓存零重复请求——R135 已证缓存共享）。
- **账号切换复位守卫**（Select.tsx:195-202）**兜底**「未来改为不重置挂载」的意外回归：`if (accountKey !== account)` 四连复位（setSelected({}) / setRev(0) / echoedRef.current=false / setEchoDone(false)）——声明于 echoedRef/rev/setRev/setEchoDone 之后（TDZ 构建陷阱规避，注释 :194 自证），提交目标绝不让旧账号 selected 被防抖 PUT 整包覆盖掉新账号目标。
- **401 被动吊销链不破坏整体重建的时序**：client.ts `r.status===401` 前置广播（:64-69，HTTP 状态码优先于 r.json()，反代 HTML/文本 401 也能摘除失效会话）→ App.onUnauthorized（App.tsx:187-235）以 `detail.session`（Bearer 令牌）反查归属账号并剔除 → sessions 变化 → `current` 被 account-reselect effect（:135-148）切到剩余账号 → key 变 → Select 整体重建。悬挂在路上的旧账号在飞请求 return 后：手动操作 handleSelectClass/handleConfirmExit 的 in-flight finally 只 invalidateQueries + 函数式删自己的 actionLoading（Select.tsx:106-110/:131-135），不留跨账号副作用；防抖 saveNow 有 unmountedRef 卸载停手（:431/:442/:459）——旧组件卸载后绝不发起 PUT、绝不污染新账号。
- **onUnauthorized 代理态/管理态清点**（已有 R137 纵深 A 实证）：`setTargetAccount` 清代理（App.tsx:204-206）、管理员会话被吊销清管理态与标记（:210-214）、代理期间管理员自身会话失效不误杀（:216-219 `isCurrentAdminSession && lostAccount !== adminName` 前置 return）——401 切号链路与整体重建边界闭合。
- **缺口判断**：无。key={account} + 四连复位双保险封死跨账号状态污染；401 吊销链任何一环卸载旧 Select 均 unmountedRef 停手、不碰新账号；react-query 缓存按 (account, sessionToken) 双 key 隔离。

**XSS/注入面巡回复核（无新增面）**：全站用户输入（课程名/教师名/账号名/错误消息/激活码）经 React JSX 自动转义渲染，`dangerouslySetInnerHTML` 全仓**零命中**（grep 实测），无反射/存储型 XSS 注入点。

## 维持观察项（不立条）

| 项 | 来源 | 当前状态 |
|----|------|----------|
| Admin.tsx:651/:661 引擎二选一非 `ui/Button` 收敛候选 | OBSERVE-93-01 | 维持（纯样式候选，无正确性/无障碍缺口） |
| Select.tsx:850 内联 cd.* 未收敛 memo 叶子 | R125 候选 | 维持（被 2s 轮询边际成本吸收，性能层无正确性影响） |
| Select.tsx:204-207 注释口径残留 vs useTickingCountdown.ts 顶层注释 | R124 起延续 | 维持观察记录（同事实不同口径，零改动延续，不立条） |

## 测试证据抽查（实测输出关键行）

- target-guard-check：`✓ 首帧未到 + 已回显标志 → 仍推迟（回显未发生整包覆盖）`、`✓ courses 非空 + 有选中 + 已回显 → 放行（稳态编辑不闷死）`、`✓ 无变更 → 返回原引用`……后跟 `target-guard 断言全绿`，exit=0。
- perf-countdown-guard：`✓ useTickingCountdown 仍每秒 setNow 自 tick`、`✓ Dashboard 内存在 memo 化倒计时叶子组件`、`✓ 路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）`……`断言全绿`，exit=0。
- admin-auth-check：6 条全绿（含「撞名学生 token ≠ 管理 token → 学生端」「自定义管理员名 token 一致 → 恢复」），exit=0。
- unauthorized-check：5 条全绿（含「非 query 段 account= → 空串」），exit=0。
- npm run build：`✓ built in 1.21s`（1948 modules），产物 index-27qti0_B.js 420.90 kB，exit=0。
- node 版本：v24.4.1（四脚本全部以 `--import jiti/register` 实测成功）。

## 全仓库文件改动

零（除构建产物 `backend/web/dist/`（gitignore 实证忽略）与本报告）。

## 结语

R137 归档 commit 03ad979 即当前 HEAD——web/ 零提交、零产品改动，七项必查全部在位且零漂移，四守卫全绿（18/3/6/5），构建通过（1948 modules，1.21s），工作树全净（0 条目）。本轮为**延续性确认轮**：所有历史锚点（M-1 / OBSERVE-93-01 / F93-01 / OBSERVE-116-01 / OBSERVE-115-01 / R125 候选）持续闭合，无新增分级发现。新契约角度（倒计时 begin_times 兜底双端一致性 + 401 吊销切号整体重建闭环）复核无缺口。

**结论：APPROVE**