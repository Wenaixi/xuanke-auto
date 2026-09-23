# R126 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `51ce8b0`（R125 收尾提交）。`git log 51ce8b0..HEAD` 全仓共 5 项提交（c4a0b36=后端 P-2 无 web/、f0f9bfc=web/ P-1 已在 R125 闭环、d9627b5/fa174c2=archive/docs 纯文档）——**本轮无任何新 web 产品改动，R125 之后 web/ 零提交**。全链路只读，唯一写入为本报告。

## 必查项结论

### 1. M-1 第六十二轮闭合 — 通过（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点**（grep 全仓实证：定义 1 + 消费 4 + 零残留）：

| 消费点 | 位置 | 三参形态 `(stateData, hasSelected, echoed)` |
|--------|------|------------------------------------------------|
| 定义 | targetGuard.ts:64-72 | 签名 `(stateData: SchedulerState \| undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `stateDataRef.current, latestSelectedCount > 0, echoedRef.current` |
| handleBack 首闸 | Select.tsx:597 | `stateDataRef.current, hasSelectedNow(), echoedRef.current` |
| handleBack 等待循环 | Select.tsx:605 | `stateDataRef.current, hasSelectedNow(), echoedRef.current` |
| 防抖回调守卫 | Select.tsx:699 | `stateDataRef.current, selectedCount > 0, echoedRef.current` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合任务描述：`stateData===undefined→true`；`echoed→false`（已回显稳态放行普通编辑不闷死）；否则 `courses非空 && hasSelected`（显式全清空 put [] 放行，绝不与慢首帧混判）。注释 :45-63 五段式（纯数据判据 / hasSelected 区分清空意图 / echoed 稳态放行 / 首帧未到无条件推迟）与实现逐条对应。
- **echoedRef 置位三路径**全在位：`:200`（账号切换复位 false，兜底守卫）、`:240`（/state 首帧 courses 空分支置 true）、`:297`（回显合并完成置 true）。
- **首帧不置位四边界**全在位：`:229`（`if (echoedRef.current) return` 只合并一次短路）、`:234`（`stateData===undefined` 提前返回绝不置位）、`:247`（`pubs.length===0` return 不置位）、`:319`（独立清理 effect `if (!echoedRef.current || publishes.length===0) return` 未回显不清理、不置位）。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts`（web/ 目录）**18 条断言全绿、exit 0**（含"首帧未到 + 已回显标志 → 仍推迟"、尾条"无变更 → 返回原引用"实证）。

### 2. OBSERVE-93-01 残余面第十六轮 — 通过（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R125 基线（Admin 13 / Dashboard 2 / Login 2 = 17）完全一致，零增零减。明细：
- Admin.tsx raw 13 处：:417（收起）、:425（复制单码）、:441（刷新）、:452（重试）、:470（复制）、:473（删除）、:577（激活开关 role=switch）、:651/:661（引擎二选一，仍为强 active 态按钮 `border-white bg-white text-black`，非 `ui/Button` 收敛，优先修复面候选维持）、:701/:792/:898/:944（四 Tab 失败态重试）。
- Dashboard.tsx raw 2 处：:67（CollapseSection 折叠钮，手写 aria-expanded/aria-controls）、:709（/logs 重试）。
- Login.tsx raw 2 处：:167（可见性切换，aria-label/aria-pressed）、:299（激活弹窗取消）。
- 全部 raw button 均带 aria 语义或为纯文本弱样式按钮，无无障碍缺口；`ui/Button` 家族 26 处（Select 9 / Admin 8 / Dashboard 5 / Login 2）与上轮同口径。
- 本轮实测比对确认 R125 明细行号逐行一致，进一步佐证零改动。

### 3. F93-01 第三十四轮 — 通过（双空实证，对比基准=R125 归档 commit 51ce8b0）

- `git log --oneline 51ce8b0..HEAD -- web/`：**空输出**（无提交触及 web/）。
- `git diff --stat 51ce8b0 HEAD -- web/`：**空输出**（无差异）。
- 附：`git status --short` 工作树全净（含 dist 均零改动），npm run build 产物 1948 modules 落 `backend/web/dist/` 未污染工作树。

### 4. OBSERVE-116-01 跟踪项第十轮维持 — 通过（注释如实口径，轮询契约零漂移）

- `web/src/lib/useTickingCountdown.ts:3-8` 注释口径为 P-1 落地后如实版本：「内部自 tick（每秒 setNow）……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo），宿主因自身 props/state 无变化而快速 bail out」——不再声称"潜在优化需拆分"，与实现一致。`Dashboard.tsx:206-212` 侧注释（"删除整页每秒 setTick——倒计时收敛 useTickingCountdown 自 tick（Dashboard/Select 同实现）……主矩阵叶子化后宿主重渲染虽仍发生但 diff 范围已显著收窄"）同为落地后如实口径。
- **轮询链路契约零漂移**（逐行实证，与 R125 键值逐位一致）：Select /electives `:58-83`（error→30000；window_closed→30000；inRange||window_opened→2000；其余 10000）、Select /state `:148-155`（error/status==="error"→30000；window_closed→30000；其余 2000）；Dashboard /state `3000/30000`、/logs `3000/30000`、/electives 恒 30000——五路查询逐一比对零漂移。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门逐处在位，含在飞守卫实证）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1211 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(exitModalClass.id)` 防误关） | 取消 :1234 |
| Login 激活 | Login.tsx:223-239 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`） | 输入框 :267 |
| Admin 删除 | Admin.tsx:210-219 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 取消按钮（:238 区域） |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项。行号较 R125 有 +1~+3 位移（Select +3 / Login +3 / Admin +1），源于 R125 报告自身行号口径与本次实测行号口径的差异，非源码改动（R125 后无任何 web 提交）。逐字段语义比对零漂移。

### 6. 新候选观察复核（R125 新观察）— 维持成立、记录不实现

- **Select.tsx:850 倒计时仍内联消费 `cd.*`**（`{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒` + :844 `cd.isExpired`），未复用 Dashboard 已落地的 memo 叶子模式。复核结论：**观察成立**。grep 实证 `cd.*` 消费共 5 处——Dashboard.tsx:399（叶子调用 `MemoCountdownMatrix` props）+ Dashboard.tsx:451（折叠行 `relativeCountdown(ms, nowMs)`，叶子外唯一 cd 派生位）+ Select.tsx:844/:850（内联 JSX 裸消费）。Dashboard 侧已由 perf-countdown-guard 结构断言守护（"路由组件顶层无裸 cd.* 消费"仅扫 Dashboard.tsx），Select 侧不在守卫覆盖内。
- **缓解因子复核成立**：Select /state 2s 轮询本就每秒级触发重渲染（tick 边际成本被吸收），且 Select 的资源/名额卡每 2s 由轮询驱动重建，tick 的 1s 间隔只省一次函数体执行——**性能层无正确性影响**。`perf-countdown-guard.ts:34-36` 对 Select 裸消费无红灯（守卫只扫 Dashboard.tsx），无守卫漂移。
- **是否值得本轮立条**：维持候选记录（与 R125 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1255 行函数体重渲染；且若落地需同步扩展 perf-countdown-guard 第三断言覆盖 Select 与 useTickingCountdown 顶层注释口径，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。明确不构成本轮 REQUEST。

### 7. 新契约角度（自选纵深）：保存链"五道防线四个出口"全路径核对

对 `flushTargets`（Select.tsx:484-567）消费时刻守卫链逐道核对：
- **防线 1（:509）**：`shouldDeferSave` 回显未完成推迟——出口 A：置脏跳过不 PUT、不置 dirtyRef（终局绝不误报保存失败），等 /state 自愈。
- **防线 2（:515）**：发布缺席 + 已有选中 → 绝不 PUT [] 假清空——出口 A 同源（保留脏）。
- **防线 3（:526）**：`selectedHasStalePublish` 残留旧 publish_id → toast + 置脏跳过——出口 A；`cleanStaleSelected`（独立 effect :318-329）随发布重建清理解锁，防抖重跑落库当前目标（F40-M1 解锁路径在位）。
- **防线 4（:553）**：selectedCount>0 却构建出空集 = 假清空 → 置脏跳过——出口 A。
- **防线 5（:558）**：`targetsUseCurrentPublishes` 发布 id 漂移错位假清空 → 置脏跳过——出口 A。
- **出口 B（:562-564）**：`savingRef.current` 在飞 → 置 dirtyRef，飞行中 PUT finally 补发本次快照（补发窗口）。
- **出口 C（:566）**：全部守卫通过 → `saveNow()` 落库。
- **出口 D（handleBack :611-624）**：返回前 flush 收敛循环——dirty/saving 静止 + revRef 稳定才 break，飞行 PUT 完成才真正卸载（改动静默丢失防线在位）。
- 核对结论：五道防线判据全部与各自注释承诺一致，四出口（置脏跳过/补发标记/直接落库/返回收敛）均可达且无死锁路径；防线 3 的"置脏跳过"与 cleanStaleSelected 的"解锁路径"配对完整（F40-M1 自愈面确认在位）。同轮核对了 react-query 轮询升/降频双向可逆性：Select /electives refetchInterval 回调是纯函数式（每次 react-query 用最新闭包重调度），升频（inRange||window_opened→2000）与降频（window_closed/error→30000、其余 10000）双向均可逆、无高频粘滞残留；/state 同为双向。**五道防线四个出口全路径无新发现。**

## 契约 20 轮次标签扫描

`grep -rn "第.*轮\|(round\|R1[0-9][0-9]\|R2[0-9][0-9]" web/src/ web/scripts/`：**零命中**（grep exit 1）。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测） |
| npm run build（tsc -b + vite） | **通过**（1948 modules，654ms，tsc -b 零错误零警告，产物落 backend/web/dist） |
| `git log 51ce8b0..HEAD -- web/` | 空（F93-01 双空实证之一） |
| `git diff --stat 51ce8b0 HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净（含 dist） |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| 轮次标签扫描 | 零命中 |

## 分级发现

- **必查项**：6/6 全部通过（M-1 第六十二轮 / OBSERVE-93-01 第十六轮 / F93-01 第三十四轮 / OBSERVE-116-01 第十轮 / OBSERVE-115-01 弹窗族 / 新契约角度+新候选复核）。
- **观察项维持**：OBSERVE-93-01（Admin 651/661 引擎二选一仍为非 ui/Button 收敛候选）、OBSERVE-116-01（轮询契约零漂移；Select 侧注释口径 `Select.tsx:204-207` 停在上轮「潜在优化，非当前承诺」而 useTickingCountdown.ts 顶层注释已升级为「Dashboard 已拆并 memo」——同一仓库两处注释对同一事实口径不一致，R124 起维持的注释同步残余，本轮因零改动延续）、R125 新观察（Select.tsx:850 倒计时内联 cd.* 未收敛 memo 叶子）维持成立。
- **新发现**：零 CRITICAL/HIGH/MEDIUM 级；无 L1/L2 新问题。保存链五道防线四个出口全路径核对无缺口。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/` 与报告本身）。

## 结语

R125 之后 web/ 零提交、零产品改动，六项必查全部在位且零漂移，四守卫全绿，构建通过，工作树全净。本轮为**延续性确认轮**：所有历史锚点（M-1/OBSERVE-93-01/F93-01/OBSERVE-116-01/OBSERVE-115-01）持续闭合，无新增分级发现，无需 REQUEST。进度推进至 127/256。
