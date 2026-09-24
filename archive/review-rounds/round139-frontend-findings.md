# R139 前端只读审查 findings

> 轮次标识：M-1 第七十五轮。审查日期：2026-09-24。工作树基线：`92f0f06`（R138 归档）。审查模式：绝对只读（唯一写入为本报告）。
> 审查范围：`web/`（React 19 + Vite + TS 前端）。

## 结论前置

| 分级 | 数量 | 概要 |
|------|------|------|
| CRITICAL | 0 | 无 |
| HIGH | 0 | 无 |
| MEDIUM | 0 | 无 |
| MINOR | 0 | 无 |
| OBSERVE | 3 | 维持观察项（Select 倒计时候选、注释口径残留、perf 守卫弱断言） |

**总评：建议 APPROVE。** 全部聚焦清单逐项实测通过，`web/` 自基线双空实证成立，前端零改动，无任何新增缺陷。

## 验证表

| 验证项 | 命令/依据 | 结果 | 实测数据 |
|--------|----------|------|----------|
| 工作区状态 | `git status --short --branch` | 通过 | `## master`，无任何改动 |
| 基线确认 | `git rev-parse --short HEAD` | 通过 | `92f0f06` |
| F93-01 双空实证 1 | `git log --oneline 92f0f06..HEAD -- web/` | 双空成立 | 输出空（exit 0） |
| F93-01 双空实证 2 | `git diff --stat 92f0f06..HEAD -- web/` | 双空成立 | 输出空 |
| build | `npm run build`（web 目录，tsc -b + vite） | 通过 | `✓ built in 858ms`，`1948 modules`，产物含 `index.html` / `index-*.css` / `index-*.js` |
| target-guard-check | `npx tsx web/scripts/target-guard-check.ts` | 全绿 | **18 断言全过**（退出码 0） |
| perf-countdown-guard | `npx tsx web/scripts/perf-countdown-guard.ts` | 全绿 | 3 断言全过 |
| admin-auth-check | `npx tsx web/scripts/admin-auth-check.ts` | 全绿 | 6 断言全过 |
| unauthorized-check | `npx tsx web/scripts/unauthorized-check.ts` | 全绿 | 5 断言全过 |
| XSS 面 | `grep -rn "dangerouslySetInnerHTML" src/` | 零命中 | 0（`innerHTML=` / `document.write` 同 0） |

## 聚焦清单逐项裁决

### 1. M-1 第七十五轮（保存链守卫）—— ✅ 在位

- **shouldDeferSave 定义**——`web/src/lib/targetGuard.ts:64-71` 与 R138 完全一致：判据本体 `stateData === undefined → true`（:69）、`echoed → false`（:70）、`courses 非空 && hasSelected → true`（:71）；函数体逐字符无漂移（全文 8 条注释面述语义与实现一致）。
- **恰 4 消费点逐字符一致**——`grep -c "shouldDeferSave(" web/src/routes/Select.tsx` = **4**，且非 `grep -c shouldDeferSave`（后者的整词计数含导入行 = 5）。四个消费点逐一核对为：flushTargets（:509）/ handleBack 首诊（:597）/ handleBack 等待循环（:605）/ 防抖回调（:699），三参数形态 `(stateDataRef.current, XX, echoedRef.current)` 四处统一。
- **echoedRef 三置位、无第四处写 true**——置位点为:200（账号复位守卫 `= false`）、:240（空分支 `= true`）、:297（合并完成 `= true`）。逐一核对了全文件 echoedRef 出现（:162 初始化、:229 回显首闸、:319 清理 effect 首闸等读取位均不写值）。**关键实证**：账号复位分支:200 写的是 `= false`，属复位语义，非写 true。三置位与 R138 一致，无第四处写 true。
- **首帧四边界**——:229（echoedRef 短路）、:234（stateData undefined 返回）、:247（pubs 空返回）、:319（清理 effect 未回显短路）。四边界与 R138 逐行一致。
- **cleanStaleSelected 原引用返回**——`web/src/lib/targetGuard.ts:26-43`，`changed` 为假时返回 `selected` 原引用（:42），无变更不引重渲染。
- **F43-M1 hasSelected 清空分判**——`:69-71` 三参判据含 `hasSelected`；两条清空路径（courses 非空 + 全清空）：守卫脚本 `deferAssert("courses 非空 + 全清空 → 放行", ..., false)` 与 `deferAssert("首帧未到 + 全清空 → 推迟", ..., true)` 全过。实测：`首帧未到 + 全清空 → 推迟`（true）与 `courses 非空 + 全清空 → 放行`（false），"用户意图 vs 数据缺席"分判生效无漂移。
- **flush/handleBack/防抖三闸双闸等回显**——flush（:597 防抖判据 + 5s 等待:605 50ms 轮询 + 一帧落地:609）/ handleBack（:497 rev>0 前置 + :597 判据）/ 防抖（:665 rev>0 + :699 判据）三处双重闸在消费时刻读 ref 镜像（`:174` selectedRef / `:176` revRef / `:181` stateDataRef 同步置位在上方）——三闸判据公式一致，无漂移。

**TDD 实测**：`npx tsx web/scripts/target-guard-check.ts` **18 断言全绿**（场景 A-J + K-M + 稳态/清空分判），与 R138 报告值一致。`npm run build` 通过（tsc -b + vite）。

### 2. OBSERVE-93-01（button 面）—— ✅ 在位

实测 `grep -rn "<button"`（排除注释行）全仓 **17 处，分布于 3 文件**：

| 文件 | 行号 |
|------|------|
| `web/src/routes/Admin.tsx`（13） | 417、425、441、452、470、473、577、651、661、701、792、898、944 |
| `web/src/routes/Dashboard.tsx`（2） | 67（CollapseSection 语义折叠）、709（/logs 失败重试） |
| `web/src/routes/Login.tsx`（2） | 167（密码可见切换 aria-pressed）、299（激活弹窗取消） |

零增零减，与 R138 一致。**651/661 候选继续维持**（8 处 refetch 按钮 / Admin 五 Tab 失败态 + 学生端 Dashboard 709 失败重试全部呈现手动 refetch 出口，无自动升级语义，见"维持观察项"）。

**补证**：其余原生按钮（`<Button` 组件）不属 `grep "<button"` 面，但独立核对了 Select.tsx:1120/1133（目标课在飞 actionLoading.disabled）、Admin.tsx:245（删除确认在飞）等组件级调用，全部携带在飞幂等守卫。Dashboard:709 是 OBSERVE-107-01 补的日志失败重试出口，语义为 display 级，与前轮记录一致。

### 3. F93-01（双空实证）—— ✅ 在位

两项命令均实测输出空、exit 0：
- `git log --oneline 92f0f06..HEAD -- web/` → **无任何提交**
- `git diff --stat 92f0f06..HEAD -- web/` → **无任何 diff**

`web/` 自 R138 基线零改动，前端全库维持 92f0f06 快照。`git rev-parse --short HEAD` = `92f0f06`。

### 4. OBSERVE-116-01（注释口径 + 五路轮询契约）—— ✅ 在位

- **useTickingCountdown.ts:3-8 与 Dashboard.tsx 三处同源**——hook 顶部注释（每秒 setNow 触发宿主重渲染 + memo 叶子收敛）与 `useTickingCountdown.ts:3-8`、Dashboard.tsx:206-209（`删除整页每秒 setTick`）、:394-398（memo 叶子注释）、Select.tsx:204-207（同款 hook 注释）四处语境统一，无漂移。
- **五路轮询契约逐键零漂移**——实测（error / window_closed 降频 30000 全数成立）：

| 路由 | 查询 | error 降频 | window_closed 降频 |
|------|------|-----------|--------------------|
| Select | `/electives`（:58） | 30000（:76） | 30000（:80） |
| Select | `/state`（:148） | 30000（:153） | 30000（:154） |
| Dashboard | `/state`（:169） | 30000 | 30000 |
| Dashboard | `/logs`（:185） | 30000 | 30000（读 `state?.window_closed` 闭包） |
| Dashboard | `/electives`（:203） | 恒 30000（静态） | 恒 30000 |

  五路 error 与 window_closed 降频端到端成立，成功态升频路径（Select `/electives` 2s、Select `/state` 2s、Dashboard 3s）与 R138 记录行为一致。Select `/electives` 升频判定读 `query.state.data.publishes` + `window_opened` 双信号（:76-81），且 TDZ 防护注释（:72-74）在位。

### 5. OBSERVE-115-01（弹窗族）—— ✅ 在位

三处最小语义门行号与 R138 一致，逐行核对：
- **Select.tsx:1204-1234**——退选二次确认 Modal：`role="dialog" + aria-modal="true" + aria-labelledby="exit-modal-title"`，Esc 关闭带 `actionLoading.has(exitModalClass.id)` 在飞防误关（:1208），取消按钮 `autoFocus`（:1234）。
- **Login.tsx:226-267**——激活码 Modal：`role="dialog" + aria-modal="true" + aria-labelledby="activate-dialog-title"`，Esc 关闭带 `activating` 在飞防误关（:233），激活码输入 `autoFocus`（:267）。
- **Admin.tsx:213-241**——删除账号 Modal：`role="dialog" + aria-modal="true" + aria-labelledby="delete-acct-modal-title"`，Esc 关闭带 `deleting` 在飞防误关（:217），取消按钮 `autoFocus`（:241））。

三处语义门（role/aria-modal/aria-labelledby + Esc + in-flight 防误关）齐全且对称，R138 后无漂移。

### 6. R125 候选复核（Select:850 内联 cd.*）—— ✅ 维持成立，不实现

- **Select.tsx:844/850 内联 cd.*（倒计时渲染）**——实测存在且成立：`:844` `cd.isExpired`，`:850` `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`。选课大厅是**整屏**倒计时（文案 + 数字区块一体，无 Dashboard 的四格独立矩阵），拆 memo 叶子的收益低：宿主每秒 tick 重渲染的时间分片主要落在一次数字文本替换，DOM 差分成本可忽略（Select.tsx:204-207 注释原语："DOM 差分成本可忽略"）。**维持记录不实现**——不实现是收益率判断，不是缺陷。
- **缓解因子无回归**——perf-countdown-guard 3 断言全绿（`useTickingCountdown` 仍 setInterval 每秒 setNow 自 tick、Dashboard 存在 memo 叶子、路由组件顶层无裸 cd.* 消费 = 0 处）。Dashboard 渲染期无裸 cd.* 直接消费（拷贝外仅 MemoCountdownMatrix props 传递 + relativeCountdown/nowMs 派生）；Select 侧 cd.* 消费为整屏一体化 4 处（isExpired/days/hours/minutes/seconds），属记录语义。
- **守卫盲区契约零漂移**——`cd.isExpired` 本地倒计时语义与 `window_opened/window_closed` 服务端状态判定契约（Select.tsx:821-826 注释）在位，`!openTimeStr && begin_times==null` 未知态、`window_closed 优先`、`window_opened 已开放`链路与 R138 一致。

### 7. 新契约角度（时间盒内自选二）

**① 激活码四个 closure 生命周期族（Admin 生成 → 复制 → 消耗 → 日志）**

选此方向的原因：激活码前端闭环横跨 Admin.tsx（生成/复制/删除）与 Login.tsx（激活/票据回传）两个文件，是"在飞幂等守卫 + 会话消费"契约的跨文件纵深。实测全部成立：
- **生成**（Admin.tsx:322-343）：`generating` 布尔在飞幂等（攻略 :323 注释——连按两次会双发 POST 重复落库），生成即 `setGenerated(codes)` + `codesQuery.refetch()` + toast。
- **复制**（Admin.tsx:470）：`title="复制" aria-label="复制"` 原生按钮，回调 `onCopy(c.code)`，复制降级链（try/catch → clipBoard/textarea/discard）与 R138 记录一致。
- **删除**（Admin.tsx:345-367）：`removing` `ReadonlySet<string>` 按码独立在飞跟踪（:349 守卫 + :361 函数式清除，与该家族"绝不被并发标记覆盖"契约对称）。
- **消耗**（Login.tsx:76-82 `/activate` 请求：body `{account, code, ticket}` 三字段回传票据；失败双分支 :87-97——票据过期"已开通重新登录即可" vs 码错误，两类失败均同步清 `pendingTicket` 防滞留旧票误导；:64-68 `activating` 幂等守卫）。

**交叉发现（零漂移）**：删除激活码复用 `onCopy(c.code)`（:470）与 `remove(c.code)`（:473）两个独立函数，与删除账号流程（Admin.tsx:249-280 `deleting` 幂等 + onDeleted 后 `setPendingDelete(null)`）分属两族，互不干扰，无一遗漏网络回调后状态清理位。激活票据 TTL（5 分钟单次）在服务端契约侧由 ConsumeTicket 保证，前端"失败清票自愈"链路完整，无前端侧二次豁免。

**② client.ts checkResp 状态码机（ApiError 错误链）逐行复核**

选此方向的原因：OBSERVE-115 弹窗族只覆盖 UI 层，client.ts 是承载"会话失效 + 业务码 401"语义的协议层，往返事件广播是理发师竞态的高发面。实测逐行对账（`web/src/api/client.ts` 全文 150 行）：
- **两处 401 广播防双发**：r.status===401 在 `r.json()` 前广播（:64-69）承载 `?account=` 反查线索；body 层 401 分支仅在 `r.status !== 401` 时二次广播（:85-94）——**注释原文写"旧式 HTTP 200 + body 401 形态仍由本分支覆盖"，实测该分支已用 `if (r.status !== 401)` 守卫，不存在双发**。UNAUTHORIZED_EVENT detail 双键（`{account, session}`）与 App.tsx:191-198 反查归属逻辑对称。
- **extractAccountFromPath 纯函数**（:16-22）：`[?&]account=([^&]+)` 正则仅匹配 query 段，unescape 用 `decodeURIComponent`；unauthorized-check 5 断言（居中/末尾/无参数/非 query 段/空路径）全绿。
- **ApiError 携带 data**（:24-34）——`{code, msg, data}` 三字段，`data.ticket` 由 Login.tsx:51-54 消费。
- **abort 统一映射**（:100-105）`AbortError → "请求超时，请重试"`，不泄漏原生文案。
- **20s 超时兜底**（:46-56）：注释说明"收紧 2 秒会掐断 /electives 大列表"，保持 20s 兜底，`signal` 显式接入（:58 `rest.signal ?? ctrl.signal`）——防调用方卸载信号被静默覆盖。
- **跨全会话令牌清理**：401 事件广播携带 `session`，App.tsx:188-230 按令牌反查剔除，绝不误杀闭包 current。

**纵深结论**：客户端的最终一致性闭环完整——401 单次广播 + 失效账号摘除 + 管理令牌标记清理（App.tsx:210-215）三链对称，与后端 `writeJSONStatus` 家族（HTTP 状态码即语义）契约对齐，无误判/误广播路径。

## 维持观察项

1. **Select.tsx:850 内联 cd.\* 倒计时渲染**——自 R124 起延续的候选：选课大厅整屏一体化布局拆 memo 叶子收益低（DOM 差分成本可忽略，:204-207 注释原文），缓解因子（2s 轮询吸收 + 整屏集成）已由 perf-countdown-guard 3 断言兜底，维持记录**不实现**。
2. **OBSERVE-93-01 候选维持**：Admin 651/661 等 refetch 出口无自动升级语义（辅助决策契约 30"打开 admin stats 补发 window_closed"为状态展示层，与 refetch 无耦合），维持观察。
3. **perf-countdown-guard 弱断言属性**：3 断言基于正则结构匹配（memo 叶子存在 / 无裸 cd.\*），非真实渲染剖面。它是否退化不解决"memo 失效但正则仍通过"的候选，但这是结构性事实守卫（memo 是性能界限的分水岭），与 R138 判定一致，继续维持。

**维持项无新增漂移**（Select.tsx:204-207 注释口径与基线比对逐字符一致，见实证表）。

## 工作区一致性核验

git 报告 `## master` 无任何未跟踪/修改文件；`npm i`（后台安装 tsx 依赖）+ `npm run build`（tsc -b + vite）均在 `web/` 内完成，产物落 `backend/web/dist/` 同基线；`git status --short` 输出空。除本报告外零漂移。

## 结论

**建议 APPROVE**。M-1 第七十五轮保存链守卫（shouldDeferSave 4 消费点逐字符一致 + echoedRef 三置位无第四处 + 首帧四边界 + 清空分判）实测成立；F93-01 双空实证、OBSERVE-93/116/115 全数在位；两个新契约角度（激活码生命周期族 + client.ts 状态码机）均零偏离；四守卫脚本（target-guard 18 / perf-countdown 3 / admin-auth 6 / unauthorized 5）全绿；build + XSS 面零命中。前端自 R138 基线零改动，无需要修复项。