# R135 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `37be8b2`（R134 归档提交「docs(review): R134 双 findings + 收尾总结……进度 135/256」）。`git log --oneline 37be8b2..HEAD -- web/` 空输出、`git diff --stat 37be8b2 HEAD -- web/` 空输出——**R134 归档后 web/ 零提交、零产品改动**。f0f9bfc（P-1 倒计时 memo）仍为最近唯一 web 产品改动，已由 perf-countdown-guard 长期守卫。全链路只读，唯一写入为本报告。

## 必查项结论

### 1. M-1 第七十一轮闭合 — 通过（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点三参形态逐字符一致**（grep 全仓实证：定义 1 + 消费 4 + 零残留）：

| 消费点 | 位置 | 三参形态 |
|--------|------|----------|
| 定义 | targetGuard.ts:64-71 | `(stateData: SchedulerState | undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` |
| handleBack 首闸 | Select.tsx:597 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` |
| handleBack 等待循环 | Select.tsx:605 | `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && ...` |
| 防抖回调守卫 | Select.tsx:699 | `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合：`stateData===undefined→true`；`echoed→false`（稳态放行）；否则 `(stateData.courses?.length ?? 0) > 0 && hasSelected`。注释 :45-63 五段式（首帧未到/纯数据判据/hasSelected 清空语义/echoed 稳态放行）与实现逐条对应，零漂移。
- **echoedRef 置位三路径**：`:200`（账号切换复位 false）、`:240`（首帧 courses 空置 true）、`:297`（回显合并完成置 true）——行号与 R134 完全一致。
- **首帧不置位四边界**：`:229`（echoedRef 短路只合并一次）、`:234`（stateData===undefined 提前返回不置位）、`:247`（pubs.length===0 不置位）、`:319`（独立清理 effect 未回显不清理不置位）——行号与 R134 完全一致。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts`（web/ 目录）**18 条断言全绿、exit 0**（8 条 shouldDeferSave + 5 条 selectedHasStalePublish + 5 条 cleanStaleSelected，含"首帧未到 + 已回显标志 → 仍推迟"、"无变更 → 返回原引用"），输出逐条与 R134 一致。

### 2. OBSERVE-93-01 残余面第二十五轮 — 通过（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R134 基线完全一致，零增零减。明细行号逐一比对：
- Admin.tsx raw 13 处：`:417`（清空生成）、`:425`（复制批量码）、`:441/:452`（激活码刷新/重试）、`:470/:473`（单码复制/删除 aria-label+title）、`:577`（激活开关 role=switch + aria-checked）、`:651/:661`（引擎二选一，仍为强 active 态 `border-white bg-white text-black` 强姿态、非 `ui/Button` 收敛，优先修复面候选维持）、`:701/:792/:898/:944`（四 Tab 失败态重试）。
- Dashboard.tsx raw 2 处：`:67`（CollapseSection 折叠钮）、`:709`（/logs 重试）。
- Login.tsx raw 2 处：`:167`（可见性切换 aria-label + aria-pressed）、`:299`（激活弹窗取消，disabled={activating} 在飞守卫）。
- Admin 651/661 引擎二选一按钮仍为 raw `<button>` 载体、强 active 姿态，`ui/Button` 收敛候选维持（纯样式统一候选、无正确性/无障碍缺口——active/非 active 双态差异显著，视觉状态无需 aria-pressed 补充即可自明）。

### 3. F93-01 第四十三轮 — 通过（双空实证，对比基准=R134 归档 commit 37be8b2）

- `git log --oneline 37be8b2..HEAD -- web/`：**空输出**。
- `git diff --stat 37be8b2 HEAD -- web/`：**空输出**。
- 37be8b2 经 `git log -1 --format="%h %s"` 实证为 R134 归档提交（"docs(review): R134 双 findings + 收尾总结……"，前端零改动）。
- `git status --short` 工作树全净（报告写入前）；npm run build 产物落 `backend/web/dist/`（web/.gitignore 确认 dist 忽略）未污染工作树。

### 4. OBSERVE-116-01 跟踪项第十九轮维持 — 通过（注释如实口径，五路轮询契约零漂移）

- `web/src/lib/useTickingCountdown.ts:3-7` 注释口径为 P-1 落地后如实版本：「消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）……宿主因自身 props/state 无变化而快速 bail out」；`Dashboard.tsx:90-93`（CountdownMatrix 注释「已拆」）、`:206-212`（「每秒变化的 cd.* 已收敛到 MemoCountdownMatrix 叶子」）、`:394-398`（主矩阵整格抽成 memo 叶子）三处同为落地后口径——useTickingCountdown 与 Dashboard 对同一事实口径统一，零漂移。`const MemoCountdownMatrix = memo(CountdownMatrix)`（:117）在位。
- **五路轮询契约零漂移**（逐行实证，与 R134 键值逐位比对）：Select /electives `:58-83`（error→30000 :76；`st.window_closed→30000` :80；`inRange || st?.window_opened→2000` :81；其余 10000 :81）、Select /state `:148-155`（error/status==="error"→30000 :153；`window_closed→30000` :154；其余 2000 :154）；Dashboard /state `:169-175`（error→30000 :171；`window_closed→30000` :173；其余 3000 :174）、/logs `:185-191`（error→30000 :187；`state?.window_closed→30000` :189；其余 3000 :190）、/electives 恒 30000（:203）。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门逐字段在位，行号与 R134 完全一致）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1234 | :1204 | :1205 | `exit-modal-title` :1206 | :1207-1211（`!actionLoading.has(exitModalClass.id)` 防误关） | 取消 :1234 |
| Login 激活 | Login.tsx:223-267 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`） | 输入框 :267 |
| Admin 删除 | Admin.tsx:210-241 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 取消 :241 |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项。行号与 R134 完全一致（R134 后零提交），逐字段语义零漂移。三处弹窗均为裸 div/裸按钮补语义（真实 Radix Dialog 未使用），注释均含对话语义承诺及 Esc 补全的诚实口径，未僭越承诺。

### 6. R125 候选复核 — 维持成立（R125 首立、R126-R135 连续维持）

Select.tsx:844/:850 倒计时内联 `cd.*` 仍为路由组件 JSX 顶层裸消费（`:844 cd.isExpired` 分支、`:850` 天数/时/分/秒内联）。

- **缓解因子复核成立**：Select /state 2s 轮询本就每秒级触发本路由组件重渲染（同样每秒 setNow 重渲染本组件无边际新增），tick 的 1s 间隔只省一次约 1255 行函数体执行——**性能层无正确性影响**。
- **守卫盲区复核**：`perf-countdown-guard.ts` 第三断言正则 `cd.(days|hours|minutes|seconds)` 在 Dashboard 侧被 `MemoCountdownMatrix` 关键字过滤后 rawCdUse=0（绿灯）；Select.tsx 不在守卫扫描范围（守卫注释明确承诺范围=「路由组件（Dashboard）顶层」），结构断言与实现契约一致，无守卫漂移。
- **本轮是否立条**：维持候选记录（与 R125-R134 同口径），不立条、不做实现。收敛价值仅在 tick 间隔内省一次约 1255 行函数体重渲染；若落地需同步扩展 perf-countdown-guard 覆盖 Select 与 Select.tsx:204-207 注释口径升级，属改一增二，收益低于成本（P-1 只承诺 Dashboard）。
- **同文件注释口径残留（R124 起延续）**：Select.tsx:204-207 本地注释仍停在上轮「（潜在优化，非当前承诺）」口径，而 useTickingCountdown.ts 顶层注释已升级为「Dashboard 已拆并 memo」——同一仓库两处注释对同一事实口径不一致。本轮因零改动延续，维持观察记录不立条。

### 7. 新契约角度（自选纵深，R131-134 已走在飞守卫/折叠/激活码/XSS，本轮转向 react-query 缓存共享跨路由 + Toast 语义/aria + XSS 面复核）

**react-query 缓存共享跨路由复核（无缺口）**：queryKey 格式 `["electives", account, sessionToken]`（Select.tsx:56 / Dashboard.tsx:201）与 `["state", account, sessionToken]`（Select.tsx:146 / Dashboard.tsx:161）——**Select 与 Dashboard 同 account+sessionToken 下 key 完全一致，react-query 全局缓存按 key 共享**：从 Select 切到 Dashboard（同账号同 token）零重复拉取直接命中缓存；Select 手动报名/退选成功后的 `invalidateQueries({ queryKey: ["electives"] })` + `["state"]`（:103-104 / :128-129 前缀匹配）同时失效两路由的对应缓存，Dashboard 下次轮询/聚焦即取新数据，跨路由状态一致性由缓存层保障。QueryClient defaultOptions 仅 retry:1 + staleTime:0（App.tsx:12-14，cacheTime 未显式设置）——staleTime:0 恒 stale、轮询驱动刷新，缓存缺失窗口无陈旧风险。Select 的 /electives refetchInterval 回调读 `queryClient.getQueryData(["state", account, sessionToken])`（:77）——与 stateData 同源零时序依赖（TDZ 注释 :72-75 自证），跨查询读缓存不引发任何额外请求。无缺口。

**Toast 语义/aria 复核（低置信度观察，无新发现）**：Toast.tsx 基于 `@radix-ui/react-toast`——Radix ToastPrimitive.Root 自带 `role="status"` 无障碍语义（aria-live 由 Radix 内建），Title/Description 用语义化 Radix primitives，视觉与语义分离正确；toast 文案最长为确认类（保存失败/发布更新/报名结果），非 timestamp 类连续广播，`role=status` 够用、无需 `role=alert`。Close 按钮无显式 aria-label（:100）——Radix ToastPrimitive.Close 默认渲染 button 且带 i18n 默认 aria-label，无缺口。同 title 去重合并（toast 计数 id 自增 + 同 title 只更新 description，:43-62）无 aria 副作用。仅记录，无新发现。

**手动操作在飞守卫 + 弹窗 Esc（R134 已走全路径，本轮抽查无漂移）**：`actionLoading` `ReadonlySet<number>` 按课程 id 独立跟踪（Select.tsx:49-52）；报名/退选入口短路（:92/:119）、按钮 disabled（:1120/:1133）同吃课程级在飞；退选弹窗 Esc/取消/确认 共用 `actionLoading.has(exitModalClass.id)`（:1208/:1232/:1242）。成功/失败/finally 函数式清除只删自己的 id（:106-110 / :128-132）。无缺口。

**XSS/注入面复核（无新增面）**：全站用户输入（课程名/账号名/错误消息）经 React JSX 自动转义渲染，`dangerouslySetInnerHTML` 全仓**零命中**（grep 实证），无反射/存储型 XSS 注入点。无安全发现。

**console.log/debugger 残留扫描（无新增）**：`grep -rn "console\.log\|debugger" web/src/` 零命中。

## 契约 20 轮次标签扫描

`grep -rn "第.*轮\|(round\|R1[0-9][0-9]\|R2[0-9][0-9]" web/src/ web/scripts/ web/index.html`：**零命中**（grep exit 1）。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测，node --import jiti/register） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测，node --import jiti/register） |
| npm run build（tsc -b + vite） | **通过**（1948 modules transformed，built in 1.12s，产物落 backend/web/dist，exit 0） |
| `git log 37be8b2..HEAD -- web/` | 空（F93-01 双空实证之一，R134 归档 commit 起零提交） |
| `git diff --stat 37be8b2 HEAD -- web/` | 空（F93-01 双空实证之二） |
| `git status --short` | 工作树全净（构建产物落 backend/web/dist，web/.gitignore 确认 dist 忽略） |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| 轮次标签扫描 | 零命中 |
| 37be8b2 归档确认 | `git log -1` 实证 = R134 归档提交（进度 135/256） |

## 分级发现

- **必查项**：7/7 全部通过（M-1 第七十一轮 / OBSERVE-93-01 第二十五轮 / F93-01 第四十三轮 / OBSERVE-116-01 第十九轮 / OBSERVE-115-01 弹窗族 / R125 候选复核 / 新契约角度 react-query 缓存共享 + Toast 语义 + XSS 面 + console 残留复核）。
- **观察项维持**：OBSERVE-93-01（Admin 651/661 引擎二选一非 ui/Button 收敛候选）、R125 候选（Select.tsx:850 内联 cd.* 未收敛 memo 叶子，被 2s 轮询边际成本吸收，性能层无正确性影响）、注释口径残留（Select.tsx:204-207 vs useTickingCountdown.ts 顶层注释同事实不同口径，R124 起延续）。
- **新发现**：**零**。无 CRITICAL/HIGH/MEDIUM/LOW 级新问题，无 L1/L2 新问题。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/`（gitignore）与报告本身）。

## 结语

R134 归档 commit 37be8b2 即当前 HEAD——web/ 零提交、零产品改动，七项必查全部在位且零漂移，四守卫全绿（18/3/6/5），构建通过（1948 modules，1.12s），工作树全净。本轮为**延续性确认轮**：所有历史锚点（M-1/OBSERVE-93-01/F93-01/OBSERVE-116-01/OBSERVE-115-01/R125 候选）持续闭合，无新增分级发现，无需 REQUEST。新契约角度（react-query 缓存共享/Toast 语义）复核无缺口。进度推进至 136/256。
