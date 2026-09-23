# R128 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `08b43da`（R127 收尾提交，报告对照 `archive/review-rounds/round127-frontend-findings.md`）。`git log 08b43da..HEAD -- web/` 空输出、`git diff --stat 08b43da HEAD -- web/` 空输出——**R127 归档后 web/ 零提交**，P-1（f0f9bfc）为最近且唯一 web 产品改动且已在 R125/R126/R127 闭环。全链路只读，唯一写入为本报告。

## 必查项结论

### 1. M-1 第六十四轮闭合 — 通过（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点**（grep 全仓实证：定义 1 + 消费 4 + 零残留）：

| 消费点 | 位置 | 三参形态 |
|--------|------|----------|
| 定义 | targetGuard.ts:64-72 | `(stateData: SchedulerState | undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` |
| handleBack 首闸 | Select.tsx:597 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` |
| handleBack 等待循环 | Select.tsx:605 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` |
| 防抖回调守卫 | Select.tsx:699 | `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合：`stateData===undefined→true`；`echoed→false`（稳态放行）；否则 `courses非空 && hasSelected`。注释 :45-63 五段式（首帧无条件推迟 / 纯数据判据不依赖 echoedRef / hasSelected 区分清空意图 / echoed 稳态放行 / 首帧未到保留）与实现逐条对应。
- **echoedRef 置位三路径**：`:200`（账号切换复位 false）、`:240`（首帧 courses 空置 true）、`:297`（回显合并完成置 true）。
- **首帧不置位四边界**：`:229`（echoedRef 短路只合并一次）、`:234`（stateData===undefined 提前返回不置位）、`:247`（pubs.length===0 不置位）、`:319`（独立清理 effect 未回显不清理不置位）。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts`（web/ 目录）**18 条断言全绿、exit 0**（含"首帧未到 + 已回显标志 → 仍推迟"、"无变更 → 返回原引用"），输出与 R127 逐条一致。

### 2. OBSERVE-93-01 残余面第十八轮 — 通过（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R127 基线完全一致，零增零减。明细行号逐一比对：
- Admin.tsx raw 13 处：`:417`（清空生成）、`:425`（复制批量码）、`:441/:452`（激活码刷新/重试）、`:470/:473`（单码复制/删除 aria-label+title）、`:577`（激活开关 role=switch + aria-checked）、`:651/:661`（引擎二选一，仍为强 active 态 `border-white bg-white text-black`，非 `ui/Button` 收敛，优先修复面候选维持）、`:701/:792/:898/:944`（四 Tab 失败态重试）。
- Dashboard.tsx raw 2 处：`:67`（CollapseSection 折叠钮）、`:709`（/logs 重试）。
- Login.tsx raw 2 处：`:167`（可见性切换 aria-label + aria-pressed）、`:299`（激活弹窗取消）。`ui/Button` 家族 26 处与上轮同口径。
- 驱动结论不变：全部 raw button 均带 aria 语义或为纯文本弱样式按钮，无无障碍缺口；Admin 651/661 收敛候选维持。

### 3. F93-01 第三十六轮 — 通过（双空实证，对比基准=R127 归档 commit 08b43da）

- `git log --oneline 08b43da..HEAD -- web/`：**空输出**。
- `git diff --stat 08b43da HEAD -- web/`：**空输出**。
- `git status --short` 工作树全净（报告写入前）；npm run build 产物落 `backend/web/dist/`（gitignore 确认）未污染工作树。

### 4. OBSERVE-116-01 跟踪项第十二轮维持 — 通过（注释如实口径，轮询契约零漂移）

- `web/src/lib/useTickingCountdown.ts:3-8` 注释口径为 P-1 落地后如实版本：「消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）……快速 bail out」，与实现一致。`Dashboard.tsx:88-93/:206-212/:394-398` 侧注释同为落地后口径（"已拆"而非"潜在优化需拆分"）——两处注释对同一事实口径统一，零漂移。
- **五路轮询契约零漂移**（逐行实证，与 R127 键值逐位比对）：Select /electives `:58-83`（error→30000；`st.window_closed→30000`；`inRange || window_opened→2000`；其余 10000，判定行 :76/:80/:81）、Select /state `:148-155`（error/status==="error"→30000；`window_closed→30000`；其余 2000）；Dashboard /state `:169-175`（error→30000；`window_closed→30000`；其余 3000）、/logs `:185-191`（error→30000；`state?.window_closed→30000`；其余 3000）、/electives 恒 30000（:203）。react-query 版本 `@tanstack/react-query@5.102.8`（package.json ^5.102.8 + node_modules 双实证）。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门逐字段在位，行号与 R127 完全一致）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1204-1234 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(...)` 防误关） | 取消 :1234 |
| Login 激活 | Login.tsx:226-267 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`） | 输入框 :267 |
| Admin 删除 | Admin.tsx:213-241 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 取消 :241 |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项。行号与 R127 完全一致（R127 后零提交），逐字段语义零漂移。三处注释均含「后续迁移 Radix Dialog 属候选/补齐最小语义门」的诚实口径，未僭越承诺。

### 6. R125 候选复核 — 维持成立（R125 首立、R126/R127 连续维持）

Select.tsx:844/:850 倒计时内联 `cd.*` 仍为路由组件 JSX 顶层裸消费（`:844 cd.isExpired` 分支、`:850` 天数/时/分/秒内联）。

- **缓解因子复核成立**：Select /state 2s 轮询本就每秒级触发本路由组件重渲染（同样每秒 setNow 重渲染本组件无边际新增），tick 的 1s 间隔只省一次约 1255 行函数体执行——**性能层无正确性影响**。
- **守卫盲区复核**：`perf-countdown-guard.ts:34-36` 只 `readFileSync` Dashboard.tsx，第三断言正则 `cd.(days|hours|minutes|seconds)` 在 Dashboard 侧被 `MemoCountdownMatrix` 关键字过滤后 rawCdUse=0（绿灯）；Select.tsx 不在守卫扫描范围（守卫注释明确承诺范围=「路由组件（Dashboard）顶层」），结构断言与实现契约一致，无守卫漂移。
- **本轮是否立条**：维持候选记录（与 R125/R126/R127 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1255 行函数体重渲染；若落地需同步扩展 perf-countdown-guard 覆盖 Select 与 Select.tsx:204-207 注释口径升级，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。
- **同文件注释口径残留（R124 起延续）**：Select.tsx:204-207 本地注释仍停在上轮「（潜在优化，非当前承诺）」口径，而 useTickingCountdown.ts 顶层注释已升级为「Dashboard 已拆并 memo」——同一仓库两处注释对同一事实口径不一致。本轮因零改动延续，维持观察记录不立条。

### 7. 新契约角度（自选纵深）：react-query 轮询升降频可逆性 + Admin 五 Tab 失败态错误卡

**react-query 轮询升降频可逆性（5.102.8 版本文档语义走查 + 实现对照）**：
- react-query `refetchInterval` 函数式回调返回新间隔即热重调度，且**每次数据到达/失败后都会以最新闭包重新评估**——函数式 interval 天然可逆（升频后返回的仍是同一个函数，下一次取 `query.state` 新值重新决策）。具体到本仓三处函数式 interval：Select /electives `:58-83` 读 `query.state` + `queryClient.getQueryData` 缓存（升/降频判定源均取自 react-query 自身状态，与组件生命周期解耦）；Select /state `:148-155` 读 `query.state.data?.window_closed`；Dashboard /state `:169-175` 同源。三处判定源全部取"react-query 自身最新状态"，失败态清缓存 data 后恒回退 30s 兜底，**升降频状态机无"卡死在某档"路径**——每轮 refetch 都以最新数据重新决策，天然双向可逆。
- Dashboard /logs `:185-191` 是唯一读**组件闭包** state 的降频（注释 `:180-183` 已如实披露：/state 每 3s 刷新触发组件重渲染 → react-query 用最新闭包重调度本查询间隔，"闭包停旧值永不降频"被数据驱动重渲染打破）。与 /electives 闭包依赖规避（TDZ 说明）对比，/logs 依赖的是一个已在同一渲染批次稳定存在、且每 3s 更新的组件变量——**不存在 TDZ 且陈旧窗口被自刷新机制封堵**，契约自洽。
- 与 Select 侧 `/electives` 的 TDZ 规避（`:72-77` 注释：refetchInterval 回调创建时同步调用、顶部 stateData const 未声明即直读抛 ReferenceError）对照：两路由对"回调能否读组件闭包"的处理路径不同（Select 规避 / Dashboard /logs 依赖刷新重调度），**各自契约自洽、互不冲突**。

**Admin 五 Tab 失败态错误卡核对**（CodesTab `:449-458` / StatsTab `:789-796` / AccountsTab `:895-901` / LogsTab `:941-947`，ConfigTab 内联 `:698-705`）：
- 五处失败态统一骨架「加载失败文案 + 重试按钮（raw `<button>`，refetch 直挂）」；configTab 因首帧锁定保存按钮另加「配置加载中——保存按钮已锁定」提示 `:693-696`，表单初始空值绝不覆盖生效配置。
- 每 Tab 独立 queryKey 含 account（管理员令牌复用同浏览器会话时数据按账号隔离，`CodesTab` 注释 :313-315）；`codesQuery`/`statsQuery`/`accountsQuery`/`logsQuery` 各有独立 refetch + 失败态重试——五 Tab 失败互不牵连、各自自愈（react-query 失败态自动重试 + 轮询升降频）。
- **失败态与成功态切换均由 react-query isError 派生**，与上节"可逆性"契约互为印证——本轮无缺口、无新发现。

## 契约 20 轮次标签扫描

`grep -rn "第.*轮\|(round\|R1[0-9][0-9]\|R2[0-9][0-9]" web/src/ web/scripts/`：**零命中**（grep exit 1）。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测） |
| npm run build（tsc -b + vite） | **通过**（1948 modules，908ms，产物落 backend/web/dist，exit 0） |
| `git log 08b43da..HEAD -- web/` | 空（F93-01 双空实证之一，R127 归档 commit 起零提交） |
| `git diff --stat 08b43da HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净 |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| 轮次标签扫描 | 零命中 |
| react-query 版本 | `@tanstack/react-query@5.102.8`（package.json + node_modules 双实证） |

## 分级发现

- **必查项**：7/7 全部通过（M-1 第六十四轮 / OBSERVE-93-01 第十八轮 / F93-01 第三十六轮 / OBSERVE-116-01 第十二轮 / OBSERVE-115-01 弹窗族 / R125 候选复核 / 新契约角度轮询可逆性 + Admin 五 Tab 错误卡）。
- **观察项维持**：OBSERVE-93-01（Admin 651/661 引擎二选一非 ui/Button 收敛候选）、R125 候选（Select.tsx:850 内联 cd.* 未收敛 memo 叶子，被 2s 轮询边际成本吸收，性能层无正确性影响）、注释口径残留（Select.tsx:204-207 vs useTickingCountdown.ts 顶层注释同事实不同口径，R124 起延续）。
- **新发现**：零 CRITICAL/HIGH/MEDIUM 级；无 L1/L2 新问题。react-query 升降频可逆性走查、Admin 五 Tab 失败态错误卡核对均无缺口。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/`（gitignore）与报告本身）。

## 结语

R127 归档 commit 08b43da 即当前 HEAD——web/ 零提交、零产品改动，七项必查全部在位且零漂移，四守卫全绿（18/3/6/5），构建通过（1948 modules），工作树全净。本轮为**延续性确认轮**：所有历史锚点（M-1/OBSERVE-93-01/F93-01/OBSERVE-116-01/OBSERVE-115-01/R125 候选）持续闭合，无新增分级发现，无需 REQUEST。进度推进至 129/256。
