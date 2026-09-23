# R127 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `b42a539`（R126 收尾提交，报告对照 `archive/review-rounds/round126-frontend-findings.md`）。`git log b42a539..HEAD` 全仓 0 提交——**HEAD 即为 b42a539 本身**，R126 归档后 web/ 零提交，P-1（f0f9bfc）为最近且唯一 web 产品改动且已在 R125/R126 闭环。全链路只读，唯一写入为本报告。

## 必查项结论

### 1. M-1 第六十三轮闭合 — 通过（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点**（grep 全仓实证：定义 1 + 消费 4 + 零残留）：

| 消费点 | 位置 | 三参形态 `(stateData, hasSelected, echoed)` |
|--------|------|------------------------------------------------|
| 定义 | targetGuard.ts:64-72 | 签名 `(stateData: SchedulerState \| undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `stateDataRef.current, latestSelectedCount > 0, echoedRef.current` |
| handleBack 首闸 | Select.tsx:597 | `stateDataRef.current, hasSelectedNow(), echoedRef.current` |
| handleBack 等待循环 | Select.tsx:605 | `stateDataRef.current, hasSelectedNow(), echoedRef.current` |
| 防抖回调守卫 | Select.tsx:699 | `stateDataRef.current, selectedCount > 0, echoedRef.current` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合：`stateData===undefined→true`；`echoed→false`（稳态放行）；否则 `courses非空 && hasSelected`。注释 :45-63 五段式与实现逐条对应。
- **echoedRef 置位三路径**：`:200`（账号切换复位 false）、`:240`（首帧 courses 空置 true）、`:297`（回显合并完成置 true）。
- **首帧不置位四边界**：`:229`（echoedRef 短路只合并一次）、`:234`（stateData===undefined 提前返回）、`:247`（pubs.length===0 不置位）、`:319`（独立清理 effect 未回显不清理不置位）。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts`（web/ 目录）**18 条断言全绿、exit 0**（含"首帧未到 + 已回显标志 → 仍推迟"、"无变更 → 返回原引用"）。

### 2. OBSERVE-93-01 残余面第十七轮 — 通过（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R126 基线完全一致，零增零减。明细行号逐一比对：
- Admin.tsx raw 13 处：`:417`（收起）、`:425`（复制单码）、`:441`（刷新）、`:452`（重试）、`:470`（复制）、`:473`（删除）、`:577`（激活开关 role=switch + aria-checked）、`:651/:661`（引擎二选一，仍为强 active 态 `border-white bg-white text-black`，非 `ui/Button` 收敛，优先修复面候选维持）、`:701/:792/:898/:944`（四 Tab 失败态重试）。
- Dashboard.tsx raw 2 处：`:67`（CollapseSection 折叠钮）、`:709`（/logs 重试）。
- Login.tsx raw 2 处：`:167`（可见性切换）、`:299`（激活弹窗取消）。`ui/Button` 家族 26 处与上轮同口径。
- 驱动结论不变：全部 raw button 均带 aria 语义或为纯文本弱样式按钮，无无障碍缺口；Admin 651/661 收敛候选维持。

### 3. F93-01 第三十五轮 — 通过（双空实证，对比基准=R126 归档 commit b42a539）

- `git log --oneline b42a539..HEAD -- web/`：**空输出**（HEAD 即 b42a539，无提交）。
- `git diff --stat b42a539 HEAD -- web/`：**空输出**（无差异）。
- `git status --short` 工作树全净；npm run build 产物落 `backend/web/dist/`（gitignore 确认）未污染工作树。

### 4. OBSERVE-116-01 跟踪项第十一轮维持 — 通过（注释如实口径，轮询契约零漂移）

- `web/src/lib/useTickingCountdown.ts:3-8` 注释口径为 P-1 落地后如实版本：「消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）……快速 bail out」，与实现一致。`Dashboard.tsx:90-93/:206-212/:394-398` 侧注释同为落地后口径（不再声称"潜在优化需拆分"）——两处注释对同一事实口径统一，零漂移。
- **五路轮询契约零漂移**（逐行实证）：Select /electives `:58-83`（error→30000；window_closed→30000；inRange||window_opened→2000；其余 10000，判定行 :76/:80/:81）、Select /state `:148-155`（error/status==="error"→30000；window_closed→30000；其余 2000）；Dashboard /state `:169-175`（3000/30000 双态）、/logs `:185-191`（3000/30000 双态）、/electives 恒 30000（:203）——与 R126 键值逐位比对零漂移。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门逐处在位）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1211 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(...)` 防误关） | 取消 :1234 |
| Login 激活 | Login.tsx:223-239 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`） | 输入框 :267 |
| Admin 删除 | Admin.tsx:210-219 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 取消按钮 :241 |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项。行号与 R126 完全一致（R126 后零提交），逐字段语义零漂移。三处注释均含「后续迁移 Radix Dialog 属候选/补齐最小语义门」的诚实口径，未僭越承诺。

### 6. R125 新候选——Select.tsx:850 倒计时内联 cd.* 未收敛 memo 叶子 — 复核维持成立

- grep 实证 `cd.*` 实际消费位（排除定义/注释）共 4 处：Dashboard.tsx:399（`MemoCountdownMatrix` 叶子调用传 4 props）+ Dashboard.tsx:451（折叠行 `relativeCountdown(ms, nowMs)`，叶子外唯一 cd 派生位）+ Select.tsx:844（`cd.isExpired` 分支）+ Select.tsx:850（`{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒` 内联 JSX 裸消费）。Select 侧：844/850 仍为路由组件 JSX 顶层裸消费。
- **缓解因子复核成立**：Select /state 2s 轮询本就每秒级触发组件重渲染（同样每秒 setNow 重渲染本路由组件也无边际新增），tick 的 1s 间隔只省一次约 1255 行函数体执行——**性能层无正确性影响**。
- **守卫盲区复核**：`perf-countdown-guard.ts:34-36` 只 `readFileSync` Dashboard.tsx，第三断言正则 `cd.(days|hours|minutes|seconds)` 在 Dashboard 侧被 `MemoCountdownMatrix` 关键字过滤后 rawCdUse=0（绿灯）；Select.tsx 不在守卫扫描范围（守卫注释明确承诺范围=「路由组件（Dashboard）顶层」），结构断言与实现契约一致，无守卫漂移。
- **本轮是否立条**：维持候选记录（与 R125/R126 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1255 行函数体重渲染；若落地需同步扩展 perf-countdown-guard 覆盖 Select 与 Select.tsx:204-207 注释口径升级，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。
- **同文件注释口径残留（R124 起延续）**：Select.tsx:204-207 本地注释仍停在上轮「（潜在优化，非当前承诺）」口径，而 useTickingCountdown.ts 顶层注释已升级为「Dashboard 已拆并 memo」——同一仓库两处注释对同一事实口径不一致。本轮因零改动延续，维持观察记录不立条。

### 7. 新契约角度（自选纵深）：P-1 memo 落地后重渲染成本端到端复核 + Dashboard 分组链 begin_date 三源

**P-1 memo 后重渲染路径（f0f9bfc diff 全量核对）**：
- Dashboard 主矩阵四格抽成 `CountdownMatrix` 叶子 + `memo(CountdownMatrix)`（:95-117）；叶子仅依赖 4 个字符串 props——宿主每秒 setNow 重渲染时 props 引用未变即 bail out（React.memo 浅比较语义与注释承诺一致）。
- 宿主 Dashboard 每 tick **仍重渲染**（useState 归属宿主，React 语义不可绕过）——memo 把 DOM 重建范围从整树收窄到叶子 + 折叠行 `relativeCountdown`（:451 渲染期读 `Date.now()`，最坏 1s 陈旧，无额外定时器）。此语义在 :206-212/:394-398 注释如实披露，无过度承诺。
- `perf-countdown-guard` 三断言（自 tick 在位 / memo 叶子存在 / Dashboard 顶层零裸 cd.*）与实现逐条匹配，3/3 绿灯（实测 exit 0）。
- **结论**：P-1 前后端到端复核无正确性问题——bail out 语义、重渲染残余范围、守卫覆盖三个层面全部与注释/契约一致，无新发现。

**Dashboard 分组链 begin_date 三源核对**：分组键三源（`CourseStatus.begin_date` → `Publish.begin_date` 映射 → "未知"兜底）在 :251-295 逐行实证；`parseDateKey`（:142-144）显式拼 `"T00:00:00"` 锁本地零点、`localTodayMs`（:149-153）`setHours(0,0,0,0)` 同基准——UTC+8 凌晨 UTC 日期偏移陷阱在注释与实现双层封堵，三源降级次序（课程自带 → 映射 → 未知）与「关闭≠元数据丢失」契约一致。无新发现。

## 契约 20 轮次标签扫描

`grep -rn "第.*轮\|(round\|R1[0-9][0-9]\|R2[0-9][0-9]" web/src/ web/scripts/`：**零命中**（grep exit 1）。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测） |
| npm run build（tsc -b + vite） | **通过**（1948 modules，691ms，产物落 backend/web/dist，exit 0） |
| `git log b42a539..HEAD -- web/` | 空（F93-01 双空实证之一，HEAD 即 b42a539） |
| `git diff --stat b42a539 HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净 |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| 轮次标签扫描 | 零命中 |
| react-query 版本 | `@tanstack/react-query@5.102.8`（package.json + node_modules 双实证） |

## 分级发现

- **必查项**：7/7 全部通过（M-1 第六十三轮 / OBSERVE-93-01 第十七轮 / F93-01 第三十五轮 / OBSERVE-116-01 第十一轮 / OBSERVE-115-01 弹窗族 / R125 新候选复核 / P-1 端到端新角度）。
- **观察项维持**：OBSERVE-93-01（Admin 651/661 引擎二选一非 ui/Button 收敛候选）、R125 候选（Select.tsx:850 内联 cd.* 未收敛 memo 叶子，被 2s 轮询边际成本吸收，性能层无正确性影响）、注释口径残留（Select.tsx:204-207 vs useTickingCountdown.ts 顶层注释同事实不同口径，R124 起延续）。
- **新发现**：零 CRITICAL/HIGH/MEDIUM 级；无 L1/L2 新问题。P-1 端到端复核、分组链三源核对、轮询契约逐字比对均无缺口。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/`（gitignore）与报告本身）。

## 结语

R126 归档 commit b42a539 即当前 HEAD——web/ 零提交、零产品改动，七项必查全部在位且零漂移，四守卫全绿（18/3/6/5），构建通过（1948 modules），工作树全净。本轮为**延续性确认轮**：所有历史锚点（M-1/OBSERVE-93-01/F93-01/OBSERVE-116-01/OBSERVE-115-01/R125 候选）持续闭合，无新增分级发现，无需 REQUEST。进度推进至 128/256。
