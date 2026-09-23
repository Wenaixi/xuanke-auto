# R132 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `073f7fd`（R131 收尾提交，报告对照 `archive/review-rounds/round131-frontend-findings.md`）。`git log --oneline 073f7fd..HEAD -- web/` 空输出、`git diff --stat 073f7fd HEAD -- web/` 空输出、HEAD 即 073f7fd——**R131 归档后 web/ 零提交、零产品改动**。f0f9bfc（P-1 倒计时 memo）仍为最近唯一 web 产品改动，已被 perf-countdown-guard 长期守卫。全链路只读，唯一写入为本报告。

## 必查项结论

### 1. M-1 第六十八轮闭合 — 通过（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点三参形态逐字符一致**（grep 全仓实证：定义 1 + 消费 4 + 零残留）：

| 消费点 | 位置 | 三参形态 |
|--------|------|----------|
| 定义 | targetGuard.ts:64-71 | `(stateData: SchedulerState | undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` |
| handleBack 首闸 | Select.tsx:597 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` |
| handleBack 等待循环 | Select.tsx:605 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && ...` |
| 防抖回调守卫 | Select.tsx:699 | `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合：`stateData===undefined→true`；`echoed→false`（稳态放行）；否则 `(stateData.courses?.length ?? 0) > 0 && hasSelected`。注释 :45-63 五段式（首帧无条件推迟 / 纯数据判据不依赖 echoedRef / hasSelected 区分清空意图 / echoed 稳态放行 / 首帧未到保留）与实现逐条对应。
- **echoedRef 置位三路径**：`:200`（账号切换复位 false）、`:240`（首帧 courses 空置 true）、`:297`（回显合并完成置 true）。
- **首帧不置位四边界**：`:229`（echoedRef 短路只合并一次）、`:234`（stateData===undefined 提前返回不置位）、`:247`（pubs.length===0 不置位）、`:319`（独立清理 effect 未回显不清理不置位）。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts`（web/ 目录）**18 条断言全绿、exit 0**（8 条 shouldDeferSave + 5 条 selectedHasStalePublish + 5 条 cleanStaleSelected，含"首帧未到 + 已回显标志 → 仍推迟"、"无变更 → 返回原引用"），输出与 R130/R131 逐条一致。

### 2. OBSERVE-93-01 残余面第二十二轮 — 通过（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R131 基线完全一致，零增零减。明细行号逐一比对：
- Admin.tsx raw 13 处：`:417`（清空生成）、`:425`（复制批量码）、`:441/:452`（激活码刷新/重试）、`:470/:473`（单码复制/删除 aria-label+title）、`:577`（激活开关 role=switch + aria-checked）、`:651/:661`（引擎二选一，仍为强 active 态 `border-white bg-white text-black`，非 `ui/Button` 收敛，优先修复面候选维持）、`:701/:792/:898/:944`（四 Tab 失败态重试）。
- Dashboard.tsx raw 2 处：`:67`（CollapseSection 折叠钮 aria-expanded + aria-controls + useId 唯一 id）、`:709`（/logs 重试）。
- Login.tsx raw 2 处：`:167`（可见性切换 aria-label + aria-pressed）、`:299`（激活弹窗取消，disabled={activating} 在飞守卫）。
- `ui/Button` 家族元素 16 处（Admin 8 / Dashboard 5 / Login 3，`<Button` 与 `</Button>` 计数逐文件配对；Select 侧 18 处为路由内部消费不跨文件）；raw button 均带 aria 语义或为纯文本弱样式按钮，无无障碍缺口；Admin 651/661 收敛候选维持。

### 3. F93-01 第四十轮 — 通过（双空实证，对比基准=R131 归档 commit 073f7fd）

- `git log --oneline 073f7fd..HEAD -- web/`：**空输出**。
- `git diff --stat 073f7fd HEAD -- web/`：**空输出**。
- 073f7fd 经 `git log -1 --format="%h %s"` 实证为 R131 归档提交（"docs(review): R131 双 findings + 收尾总结……进度 132/256"）。
- `git status --short` 工作树全净（报告写入前）；npm run build 产物落 `backend/web/dist/`（web/.gitignore 确认 dist 忽略）未污染工作树。

### 4. OBSERVE-116-01 跟踪项第十六轮维持 — 通过（注释如实口径，五路轮询契约零漂移）

- `web/src/lib/useTickingCountdown.ts:3-8` 注释口径为 P-1 落地后如实版本：「消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）……宿主因自身 props/state 无变化而快速 bail out」；`Dashboard.tsx:90-93`（CountdownMatrix 注释「已拆」）、`:206-212`（「每秒变化的 cd.* 已收敛到 MemoCountdownMatrix 叶子」）、`:394-398`（主矩阵整格抽成 memo 叶子）三处同为落地后口径——useTickingCountdown 与 Dashboard 对同一事实口径统一，零漂移。
- **五路轮询契约零漂移**（逐行实证，与 R131 键值逐位比对）：Select /electives `:58-83`（error→30000 :76；`st.window_closed→30000` :80；`inRange || window_opened→2000` :81；其余 10000 :81）、Select /state `:148-155`（error/status==="error"→30000 :153；`window_closed→30000` :154；其余 2000 :154）；Dashboard /state `:169-175`（error→30000 :171；`window_closed→30000` :173；其余 3000 :174）、/logs `:185-191`（error→30000 :187；`state?.window_closed→30000` :189；其余 3000 :190）、/electives 恒 30000（:203）。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门逐字段在位，行号与 R131 完全一致）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1234 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(...)` 防误关） | 取消 :1234 |
| Login 激活 | Login.tsx:223-267 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`） | 输入框 :267 |
| Admin 删除 | Admin.tsx:210-241 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 取消 :241 |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项。行号与 R131 完全一致（R131 后零提交），逐字段语义零漂移。三处注释均含「后续迁移 Radix Dialog 属候选/补齐最小语义门」的诚实口径，未僭越承诺。

### 6. R125 候选复核 — 维持成立（R125 首立、R126-R132 连续维持）

Select.tsx:844/:850 倒计时内联 `cd.*` 仍为路由组件 JSX 顶层裸消费（`:844 cd.isExpired` 分支、`:850` 天数/时/分/秒内联）。

- **缓解因子复核成立**：Select /state 2s 轮询本就每秒级触发本路由组件重渲染（同样每秒 setNow 重渲染本组件无边际新增），tick 的 1s 间隔只省一次约 1255 行函数体执行——**性能层无正确性影响**。
- **守卫盲区复核**：`perf-countdown-guard.ts` 第三断言正则 `cd.(days|hours|minutes|seconds)` 在 Dashboard 侧被 `MemoCountdownMatrix` 关键字过滤后 rawCdUse=0（绿灯）；Select.tsx 不在守卫扫描范围（守卫注释明确承诺范围=「路由组件（Dashboard）顶层」），结构断言与实现契约一致，无守卫漂移。
- **本轮是否立条**：维持候选记录（与 R125-R131 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1255 行函数体重渲染；若落地需同步扩展 perf-countdown-guard 覆盖 Select 与 Select.tsx:204-207 注释口径升级，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。
- **同文件注释口径残留（R124 起延续）**：Select.tsx:204-207 本地注释仍停在上轮「（潜在优化，非当前承诺）」口径，而 useTickingCountdown.ts 顶层注释已升级为「Dashboard 已拆并 memo」——同一仓库两处注释对同一事实口径不一致。本轮因零改动延续，维持观察记录不立条。

### 7. 新契约角度（自选纵深，R131 已走在飞守卫组件级/弹窗级分界 + extrasOpen + 激活码可访问性，本轮转向手动操作在飞守卫 + 弹窗 Esc 交互全路径复核 + 目标保存 PUT 防抖/flush 竞态时序复核）

**手动操作在飞守卫 + 弹窗 Esc 交互全路径走查（无缺口）**：`actionLoading` 为 `ReadonlySet<number>` 按课程 id 独立跟踪（Select.tsx:52），报名/退选按钮 disabled（:1120/:1133）与入口短路（:92/:119）同吃课程级在飞；退选弹窗取消/确认/Esc 三路径共用 `actionLoading.has(exitModalClass.id)`（:1208 Esc / :1232 取消 disabled / :1241-1242 确认 disabled+短路）——确认按钮在飞时取消与 Esc 同锁（防半途退出产生歧义），双按钮同源单源守卫。报名在飞时对应课程退选按钮 disabled（弹窗不可开），报名/退选并发互不覆盖（函数式清除 `n.delete(c.id)` :106-110，只删自己的 id 绝不抹其他课程标记）。退选弹窗打开本身（setExitModalClass :1121）不置在飞——打开无网络请求，确认提交才置位；双击退选列按钮对同一课程重复 set 同一 state 无副作用。Esc 三处全部 keydown 于 overlay 根 div、fired 于任何聚焦位置、与取消按钮同置位清空，无缺口。

**目标保存 PUT 防抖/flush 竞态时序复核（无缺口，与 R130 在飞补发总账零漂移）**：`void saveNow()` 全仓仅 3 处（scheduleRetry :427 / flushTargets :566 / 防抖 :745），finally 内 `dirtyRef && attempt===0` 自动补发 :462-464 是唯一自动续发出口；dirtyRef 置位仅 saveNow catch :455 与"飞行中标记补发" :463/:742 两处——终局 toast（:647-653 pendingUnsaved 三信号）绝不误报守卫拦下的安全拦截（守卫全路径不置 dirtyRef，:503/:510/:516/:554/:559/:700/:710/:726/:733/:738 走查）；卸载后 finally 前序 return 短路补发 :459、退避新 timer 由 unmountedRef 兜底清理 :404。flush 消费时刻读 ref（:491-492）与防抖消费时刻读 publishesRef/stateDataRef（:663/:699）同源镜像，串行化绝不乱序覆盖。补发出口契约自洽，无缺口。

## 契约 20 轮次标签扫描

`grep -rn "第.*轮\|(round\|R1[0-9][0-9]\|R2[0-9][0-9]" web/src/ web/scripts/ web/index.html`：**零命中**（grep exit 1）。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测，node --import jiti/register） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测，node --import jiti/register） |
| npm run build（tsc -b + vite） | **通过**（1948 modules，398ms，产物落 backend/web/dist，exit 0） |
| `git log 073f7fd..HEAD -- web/` | 空（F93-01 双空实证之一，R131 归档 commit 起零提交） |
| `git diff --stat 073f7fd HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净（构建产物落 backend/web/dist，web/.gitignore 确认 dist 忽略） |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| 轮次标签扫描 | 零命中 |
| 073f7fd 归档确认 | `git log -1` 实证 = R131 收尾提交（进度 132/256） |

## 分级发现

- **必查项**：7/7 全部通过（M-1 第六十八轮 / OBSERVE-93-01 第二十二轮 / F93-01 第四十轮 / OBSERVE-116-01 第十六轮 / OBSERVE-115-01 弹窗族 / R125 候选复核 / 新契约角度在飞守卫+弹窗 Esc 全路径 + 保存链防抖/flush 竞态时序复核）。
- **观察项维持**：OBSERVE-93-01（Admin 651/661 引擎二选一非 ui/Button 收敛候选）、R125 候选（Select.tsx:850 内联 cd.* 未收敛 memo 叶子，被 2s 轮询边际成本吸收，性能层无正确性影响）、注释口径残留（Select.tsx:204-207 vs useTickingCountdown.ts 顶层注释同事实不同口径，R124 起延续）。
- **新发现**：零 CRITICAL/HIGH/MEDIUM 级；无 L1/L2 新问题。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/`（gitignore）与报告本身）。

## 结语

R131 归档 commit 073f7fd 即当前 HEAD——web/ 零提交、零产品改动，七项必查全部在位且零漂移，四守卫全绿（18/3/6/5），构建通过（1948 modules），工作树全净。本轮为**延续性确认轮**：所有历史锚点（M-1/OBSERVE-93-01/F93-01/OBSERVE-116-01/OBSERVE-115-01/R125 候选）持续闭合，无新增分级发现，无需 REQUEST。进度推进至 133/256。
