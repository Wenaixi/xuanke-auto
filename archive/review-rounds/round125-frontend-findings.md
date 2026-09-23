# R125 前端只读审查报告（web/，React 18 + Vite + TS + Radix UI + Tailwind）

审查基线：commit `60bc15f`（R124 收尾提交，含 filter-repo 重写后新哈希体系）。R124 审查后主控叠加 P-1（f0f9bfc 倒计时 memo 叶子，web 产品改动）与 P-2（c4a0b36 后端 struct，与前端无关）、archive 目录搬移（d9627b5/fa174c2，纯文档零行为）。**f0f9bfc 是本轮 web 唯一产品改动**，且已由 perf-countdown-guard 守护。

全链路只读，唯一写入为本报告。

## 必查项结论

### 1. M-1 第六十一轮闭合 — 通过（零漂移）

**shouldDeferSave 定义 + 恰 4 消费点**（`grep -n shouldDeferSave` 全仓实证，定义 1 + 消费 4）：

| 消费点 | 位置 | 三参形态 `(stateData, hasSelected, echoedRef.current)` |
|--------|------|--------------------------------------------------------|
| 定义 | targetGuard.ts:64-72 | 签名 `(stateData: SchedulerState \| undefined, hasSelected: boolean, echoed: boolean)` |
| flushTargets 守卫 | Select.tsx:509 | `stateDataRef.current, latestSelectedCount > 0, echoedRef.current` |
| handleBack 首闸 | Select.tsx:597 | `stateDataRef.current, hasSelectedNow(), echoedRef.current` |
| handleBack 等待循环 | Select.tsx:605 | `stateDataRef.current, hasSelectedNow(), echoedRef.current` |
| 防抖回调守卫 | Select.tsx:699 | `stateDataRef.current, selectedCount > 0, echoedRef.current` |

- 判据本体 `targetGuard.ts:69-71` 逐字符吻合任务描述：`stateData===undefined→true`；`echoed→false`（已回显稳态放行普通编辑不闷死）；否则 `courses非空 && hasSelected`（显式全清空 put [] 放行，绝不与慢首帧混判）。
- **echoedRef 置位三路径**全在位：`:200`（账号切换复位 false，兜底守卫路径）、`:240`（/state 首帧 courses 空分支置 true）、`:297`（回显合并完成置 true）。
- **首帧不置位四边界**全在位：`:229`（`if (echoedRef.current) return` 只合并一次短路）、`:234`（`stateData===undefined` 提前返回绝不置位——回显未发生时 selected 只含用户改动，整包 PUT 覆盖删除后端旧目标）、`:247`（`pubs.length===0` return 不置位——发布缺席时幽灵条目过滤与合并均不触发）、`:319`（独立清理 effect `if (!echoedRef.current || publishes.length===0) return` 未回显不清理、不置位）。
- **断言脚本实测**：`node --import jiti/register scripts/target-guard-check.ts`（web/ 目录）**18 条断言全绿、exit 0**（含"首帧未到 + 已回显标志 → 仍推迟"、尾条"无变更 → 返回原引用"实证）。

### 2. OBSERVE-93-01 残余面第十五轮 — 通过（零增零减）

`grep -rn "<button" web/src/` 全仓实证：**17 处 raw `<button`、3 文件**（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R124 基线（Admin 13 / Dashboard 2 / Login 2 = 17）及上轮完全一致，零增零减。明细：
- Admin.tsx raw 13 处：:417（收起）、:425（复制单码）、:441（刷新）、:452（重试）、:470（复制）、:473（删除）、:577（激活开关 role=switch）、:651/:661（**引擎二选一，仍为强 active 态按钮 `border-white bg-white text-black`，非 `ui/Button` 收敛，优先修复面候选维持**）、:701/:792/:898/:944（四 Tab 失败态重试）。
- Dashboard.tsx raw 2 处：:67（CollapseSection 折叠钮，手写 aria-expanded/aria-controls）、:709（/logs 重试）。
- Login.tsx raw 2 处：:167（可见性切换，aria-label/aria-pressed）、:299（激活弹窗取消）。
- 全部 raw button 均带 aria 语义或为纯文本弱样式按钮，无无障碍缺口；`ui/Button` 家族 26 处（Select 9 / Admin 8 / Dashboard 5 / Login 2）与上轮同口径。

### 3. F93-01 第三十三轮 — 通过（双空实证，对比基准迁移至 R124 归档 commit）

- `git log --oneline 60bc15f..HEAD -- web/`：**空输出**（无提交触及 web/）。
- `git diff --stat 60bc15f HEAD -- web/`：**空输出**（无差异）。
- **f0f9bfc 为本轮唯一 web 改动**：60bc15f..HEAD 共 5 项提交（c4a0b36=后端 P-2 无 web/、f0f9bfc=web/ P-1、d9627b5/fa174c2=archive/docs 无 web/），`git log -1 -- web/` 即 f0f9bfc；`git diff --stat f0f9bfc` 仅触 `web/scripts/perf-countdown-guard.ts`（+40）、`web/src/lib/useTickingCountdown.ts`（±6）、`web/src/routes/Dashboard.tsx`（±82），Select/Admin/Login/types/client 均零触碰。
- perf-countdown-guard 实测全绿（见验证表），P-1 守卫对 f0f9bfc 压实闭环。
- （附注：R124 报告引用的旧锚 `0333009`/`1351fa4` 已被 filter-repo 重写清除，本报告改用 60bc15f 新锚，任务说明已确认此时序。）

### 4. OBSERVE-116-01 跟踪项第九轮维持 — 通过（注释口径已随 P-1 如实升级，轮询契约零漂移）

- `web/src/lib/useTickingCountdown.ts:3-8` 注释口径已由 f0f9bfc 升级为落地后如实版本：「内部自 tick（每秒 setNow）……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo），宿主因自身 props/state 无变化而快速 bail out，重渲染不会扩散到整树」——不再声称"潜在优化需拆分"，与 P-1 落地事实一致。
- `Dashboard.tsx:90-117` 叶子落地实证：`CountdownMatrix` 抽为独立函数组件 + `MemoCountdownMatrix = memo(...)`，主矩阵与折叠行是仅有的 cd/nowMs 消费位；`relativeCountdown` 由 tick 驱动整页重渲染顺带刷新（最坏 1s 陈旧，无额外定时器）。
- **轮询链路契约零漂移**（逐行实证）：Dashboard /state `3000/30000`（:169-174 含失败态降频 30000）、/logs `3000/30000`（:185-190）、/electives 恒 `30000`（:203-204）；Select /state `2000/30000`（:148-155 失败态 30000）、/electives `2000/10000/30000`（:58-81，window_closed→30000、inRange||window_opened→2000、其余 10000、error 30000）。全链路逐一键值比对无漂移。

### 5. OBSERVE-115-01 弹窗族 — 通过（三处最小语义门逐处在位）

| 弹窗 | 文件:行 | role=dialog | aria-modal | aria-labelledby | Esc 关闭（在飞守卫） | autoFocus |
|------|---------|------------|------------|------------------|----------------------|-----------|
| Select 退选 | Select.tsx:1201-1211 | :1203 | :1204 | `exit-modal-title` :1205 | :1207-1211（`!actionLoading.has` 防误关） | 取消 :1234 |
| Login 激活 | Login.tsx:223-238 | :226 | :227 | `activate-dialog-title` :228 | :232-238（`!activating`） | 输入框 :267 |
| Admin 删除 | Admin.tsx:209-218 | :213 | :214 | `delete-acct-modal-title` :215 | :216-218（`!deleting`） | 取消按钮（:238 区域） |

三处 Esc 均带在飞守卫（退选/激活/删除进行中不响应防误关），autoFocus 均落在最安全默认项（取消按钮 / 激活码输入框）。

### 6. 新契约角度（本轮自选纵深）：P-1 倒计时 memo 叶子落地后的重渲染路径复核

**Dashboard 叶子化结论：正确且闭环。** `MemoCountdownMatrix` 收 4 个字符串 props；字符串为 JS 原始值，`memo` 的 `Object.is` 浅比较按值等价（`Object.is("05","05")===true`），每秒 tick 只有实际变化的数字（如秒位 05→04）穿透 prop 触发叶子重渲染，其余三格与整树在宿主 bail out 下完全静止——770 行整树不再每秒重建的承诺兑现。guard 断言"路由组件顶层无裸 cd.* 消费（0 处）"实测成立，倒计时折叠行经 `relativeCountdown` 复用同一 tick（无额外定时器）。`primaryMs`/`extrasMs` 的 useMemo 依赖用稳定引用（`electives?.begin_times`、`primaryMs`），不在渲染期新建数组，依赖恒定性正确。

**新发现（M-1 级跳出框，L1 观察维持扩散到 Select）**：
- **Select.tsx 倒计时仍内联消费 `cd.*`（:850 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒` + :844 `cd.isExpired`），未复用 Dashboard 已落地的 memo 叶子模式**——每秒 tick 仍对整个约 1255 行 Select 路由组件执行一次函数体重渲染（含 tabs 构建、filteredClasses 过滤/排序、全部课程卡片 map）。P-1 只覆盖 Dashboard 未覆盖 Select。缓解因子：Select /state 2s 轮询本就每秒级触发重渲染，tick 边际成本部分被吸收；DOM 差分成本在题述下"可忽略"的旧注释仍有参考。严重度 L2（与 P-1 同类，但被 2s 轮询吸收，非黄金期关键路径的实时性瓶颈）。
- **Select.tsx:204-207 注释口径停在上轮「潜在优化，非当前承诺」**，而 P-1 已在 Dashboard 证明该优化落地价值且 useTickingCountdown.ts 顶层注释已升级为"Dashboard 已拆并 memo"——同一仓库内两处注释对同一事实（"memo 叶子是否已实现"）口径不一致，属 OBSERVE-116-01 族注释同步残余（本项在 R124 已以"如实口径"维持，本轮因 P-1 落地出现新的不一致面）。建议：Select 侧注释改"可参照 Dashboard CountdownMatrix 拆 memo 叶子收敛（当前未拆）"或直接落地同款叶子。
- 修复面评估均非必改：memo 收敛为性能层面（无正确性影响），Select 的资源/名额卡每 2s 本就由轮询驱动重渲染，改动收益仅在 tick 的 1s 间隔内省一次函数体执行。列入观察维持，不作本轮 REQUEST。

## 契约 20 轮次标签扫描

`grep -rn "第.*轮\|(round\|R1[0-9][0-9]\|R2[0-9][0-9]" web/src/`：**零命中**。代码注释无轮次前缀标签残留，全部为「为什么/契约/陷阱」本体语义。

## 验证表

| 验证项 | 结果 |
|--------|------|
| target-guard-check 断言 | **18/18 全绿**（exit 0，实测） |
| perf-countdown-guard 断言 | **3/3 全绿**（exit 0，实测） |
| admin-auth-check 断言 | **6/6 全绿**（exit 0，实测） |
| unauthorized-check 断言 | **5/5 全绿**（exit 0，实测） |
| npm run build（tsc -b + vite） | **通过**（1948 modules，2.75s，tsc -b 零错误零警告，产物后端 web/dist） |
| `git log 60bc15f..HEAD -- web/` | 空（F93-01 双空实证之一） |
| `git diff --stat 60bc15f HEAD -- web/` | 空（F93-01 双空实证之二） |
| `<button` 全仓清点 | **17 处 3 文件**（Admin 13 / Dashboard 2 / Login 2）零增零减 |
| 轮次标签扫描 | 零命中 |

## 分级发现

- **必查项**：6/6 全部通过（M-1 第六十一轮 / OBSERVE-93-01 第十五轮 / F93-01 第三十三轮 / OBSERVE-116-01 第九轮 / OBSERVE-115-01 弹窗族 / 新契约角度）。
- **观察项维持**：OBSERVE-93-01（Admin 651/661 引擎二选一仍为非 ui/Button 收敛候选）、OBSERVE-116-01（轮询契约零漂移；Select 侧注释口径与 Dashboard 落地事实出现新一轮不一致面）。
- **新发现（L2/观察维持级）**：Select.tsx:850/:844 倒计时内联消费 cd.* 未收敛 memo 叶子——P-1 仅覆盖 Dashboard，Select 每秒 tick 仍重渲染函数体；被 2s 轮询边际成本部分吸收，属性能层面无正确性影响，不阻塞。
- **无 CRITICAL/HIGH/MEDIUM 级新发现**——本轮 f0f9bfc 是唯一 web 产品改动且为纯性能优化，P-2/archive 搬移不触前端，必查六项全部在位。
- **全仓库文件改动**：零（除构建产物 `backend/web/dist/` 与报告本身）。
