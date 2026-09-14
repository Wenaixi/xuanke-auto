# 第 8 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/main.go/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 + 前端两个独立通道），全部发现定位到具体行号并经读码推演（部分写最小复现测试验证）成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**前端 1 CRITICAL（F8-01 401 踢除账号落盘失真的隐藏标志位竞态）连根拔除**，后端 2 MAJOR（B8-M1 时钟校准防重入代码失效、B8-M2 手动退选后重启"假成功"恢复）、1 UX 修复（B8-M7 手动路径 token 失效自动重登）根治；前端 1 性能修复（F8-04 每秒整页重建）；另 5 处 MINOR/INFO 核实后记录。

---

## 一、后端发现与修复状态（B8 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B8-M1** | MAJOR | **时钟校准防重入代码是死代码**：原 `maybeSyncClock` 用 `if s.syncing { return }` 防重入，但 `syncing=true` 是在**异步 goroutine 内部**（`go func(){ ... s.syncing=true ... SyncServerTime() ... }()`）才置位——主 tick 循环检查 `s.syncing` 时 goroutine 尚未运行，恒为 false，**每次 tick（300ms）都生成一个 SyncServerTime goroutine**：SyncServerTime 挂起 >300ms 时并发 goroutine 叠加打爆教务服务器、校准结果回写错乱 | ✅ `maybeSyncClock` 同步段先检查 `if s.syncing { return }` 再置 `s.syncing = true` + `s.lastSyncStart = now`，异步 goroutine 只负责采样——真正的**调用侧单飞**（第二个调用直接放弃，不等待）；回归 `TestClockSyncFailureResetsOffset` 改为轮次制等待（每轮先等 syncing 复位再触发），race 绿 |
| **B8-M2** | MAJOR | **手动退选后重启被"恢复"成已报名**：退选成功只 `RemoveDone`（打日志+清状态），但 `success` 表里该 (account, class_id) 行还在——重启时 `RestoreDone` 按 success 表把课程重新标记为"已报名成功"，交付快照与用户真实退选意图分叉 | ✅ 新增 `Store.DeleteSuccess(acct, classID)`，`RemoveDone` 在 AppendLog 前调用删除 success 行；调度器 `Store` 接口扩展（fakeStore 同步补上）；回归：`deletingStore` 包装断言 RemoveDone 真的调了 DeleteSuccess |
| **B8-M7** | UX/MINOR | **手动报名/退选命中 token 失效不自愈**：手动路径 `SelectClass`/`ExitClass` 返回 `ErrUnauthorized` 时直接把"您未登录"原文抛给前端，不触发 `maybeRelogin`——用户手动点操作失败后只能干等探测/自动链发现失效才重登 | ✅ handler 两条分支统一 `MarkTokenValid(acct)` + `MaybeRelogin(acct)` + 返回友好提示"教务令牌已失效，正在自动重登，请稍后重试"；调度器导出 `MaybeRelogin` 别名（包内幂等门控，api 层无权碰私有 map）；新增 `TestHandleElectivesSelectUnauthorizedRelogin` 断言失效→自动重登状态推进 |
| **B8-M4** | MINOR | **per-account 探测单飞缺失**：`probe()` 内为每个目标账号 `go ProbeForAccount` 无单飞——N 账号并发时每 tick 同时打 N 个教务上游（带宽浪费，快照各自独立无数据污染） | ⏳ 列待办（与第 7 轮 B7-M5 同源）。黄金期探测集中度由 lastProbe 全校闸门兜底，叠加 per-account 单飞收益有限，记录待 D 系列 |
| **B8-M6** | MINOR | `openTimeNow`（开放时间毫秒级校准）读 `lastSyncStart` 用 `s.mu` 锁内的 `time.Now()` 快照——与并发采样间最多差一个 tick（300ms），属可接受误差 | ⏳ 已确认为 MINOR 可接受，记录不修 |
| **B8-M3** | INFO | `reloginResults` 缓冲 8：>8 账号并发重登成功时非阻塞投递丢弃（不影响崩溃，仅丢一次补探测） | ⏳ 与第 7 轮 B7-M2 同源，确认非阻塞 `select+default` 无死锁；容量 8 覆盖真实 ≤8 场景，记录不修 |
| **B8-M5** | INFO | **文档与实现分叉（SSE）**：根 CLAUDE.md 写"SSE 实时抢课日志流"，但后端实际实现是前端按 2s/10s 轮询 `/api/logs`，**全链路无 SSE 端点**——文档描述了一箱实际不存在的架构 | ✅ 决策：**以轮询为准修正 CLAUDE.md 文案**（SSE 需后端保持长连接 + 断线重连，相对轮询无收益且增加复杂度；轮询已被 window_closed 降频覆盖），文档不再声称 SSE |

## 二、前端发现与修复状态（F8 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F8-01** | CRITICAL | **401 踢除账号落盘失真——失效账号刷新后复活**：onUnauthorized 用 `setSessions(prev => { const removedNext = [...prev].filter(...); if(!removedPrev){ saveSessions(removedNext); } ... })`——`removedPrev`/`removedNext` 是 **updater 函数之外的闭包变量，在 updater 实际运行前就已求值**，恒为空/恒为 true 判断失真；React 并发下 updater 可能延迟执行，localStorage 写入记录与最终 state 分叉（踢除动作没落盘，刷新页面失效账号又从 localStorage 复活继续请求，反复 401 轰炸） | ✅ 放弃 updater 外部标志位，改为快照模式：`snap = sessions` 在外层取 → 检查 lostAccount 确实存在于 snap → `saveSessions(next)` 落盘 → `setSessions(next)`。state 与 localStorage 同一份 next，无闭包时序；effect deps 收敛为 `[adminName, inAdmin]`（eslint-disable 说明） |
| **F8-04** | MINOR/PERF | **每秒全页重建**：Dashboard 用 `setTick(t+1)` + `parseCountdown` 每秒把整页（含课程网格、日志表）重新渲染一次——用户在看别的页面时也在定时器空转 | ✅ 倒计时收敛进独立 `useTickingCountdown(target)` 自 tick 组件：内部自带 interval、返回 padding diff；过期返回 00；整页只渲染 Countdown 一处变化。回归：`tsc -b` + vite build 绿 |
| **F8-03** | MINOR | **删除账号后列表 10s 不更新**：Admin 账号 Tab DELETE 成功后只本地 setAccounts 过滤，没失效化 react-query 的 `["admin-accounts"]` 缓存——删除后立刻失效的账号仍显示，直到 10s 轮询拉回新列表 | ✅ DELETE 成功分支补 `queryClient.invalidateQueries({ queryKey: ["admin-accounts"] })`，删除即时生效不靠轮询兜底 |
| **F8-05** | INFO | **ConfigTab 死初始化引用**：Admin 配置 Tab 的 `initializedRef` 一次性门控已无存在意义（configEpoch 驱动回填，F7-02 已替换），残留死代码 + 偶发"首次加载不回填"假象 | ✅ 移除 `initializedRef` 及相关逻辑（随 F8-03 提交） |
| **F8-06** | INFO | 登录 20s 超时边界：`/api/login` fetch 超时后用户再点提交会拿到旧 abort signal——已确认仅影响自身请求（无跨请求污染），前端 catch AbortError 已映射友好文案（F7-09） | ⏳ 已确认边界自洽，记录不修 |
| **F8-02** | INFO | client.ts `AbortError` catch 分支打印 reason（本意区分超时与主动取消）——实际所有 abort 统一表现为超时语义，reason 分支是设计债 | ⏳ 已确认无功能影响，记录不修 |

### 前端明确无问题区（读码推演成立）
- 401 UNAUTHORIZED_EVENT 其余路径（account= 优先 + 令牌反查 + 管理员代理保护分支放行）独立于 F8-01 失真的移除逻辑，未受影响
- F8-04 修复后 Dashboard 其余轮询（2s/10s 动态轮询）与倒计时解耦，无交互回归
- 删除账号链路（scheduler 状态清理 + credentials/success/targets 事务删除）面面俱到，仅前端缓存失效缺失（F8-03）

## 三、修复细节（本轮 8 处生产改动，5 个独立 commit，均验证后提交）

- **B8-M1**：`maybeSyncClock` 同步段防重入（syncing 置位前检查），异步只采样；回归 `TestClockSyncFailureResetsOffset` 轮次制等待（先等 syncing 复位再触发）race 绿
- **B8-M2**：`Store.DeleteSuccess` 新增 + `RemoveDone` 调用 + Store 接口扩展；回归 `deletingStore` 断言删除推进
- **B8-M7**：`handler.go` 报名/退选 ErrUnauthorized 分支 MarkTokenValid+MaybeRelogin + 友好提示；调度器导出 `MaybeRelogin` 别名；新增 `TestHandleElectivesSelectUnauthorizedRelogin`
- **F8-01**：`App.tsx` onUnauthorized 改快照模式（外层 snap → 判断 → saveSessions → setSessions），移除 updater 闭包标志位
- **F8-03/F8-05**：`Admin.tsx` DELETE 后 invalidateQueries + 移除 ConfigTab initializedRef 死引用
- **F8-04**：`Dashboard.tsx` 倒计时收敛 `useTickingCountdown` 自 tick 组件，移除每 1s 全页 setTick

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `go build ./...` + `go test -race -count=1 ./...` — 全包绿（api / scheduler / store 等，含新测试）
- `cd web && npx tsc -b --force` + `npm run build` — 通过
- 提交序列见提交索引（B8-M1→F8-04 五个独立 commit + 文档）

---

## 提交索引（本轮 5 个独立 commit + 文档）

```
c65db6f fix(backend): 第8轮B8-M1时钟校准单飞根治+B8-M2手动退选删success行——防重入死代码&重启退选意图丢失
11ff139 fix(web): 第8轮F8-01 401踢除账号localStorage同步落盘——removedNext外部标志恒false致失效账号刷新复活
3888aee fix(web): 第8轮F8-03删除账号后立即刷新列表+F8-05清理ConfigTab死引用——10s轮询前行不消失
87f4eba fix(web): 第8轮F8-04倒计时收敛自tick组件——每秒setTick整页重建移除
da04dfe fix(backend): 第8轮B8-M7手动报名/退选token失效自动重登——手动路径与自动链对称自愈
```