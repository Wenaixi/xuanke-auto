# 第 9 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/main.go/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 + 前端两个独立通道），全部发现定位到具体行号并经读码推演（部分写最小复现测试验证）成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**后端 3 项真实修复（B9-01 重登退避被手动路径击穿、B9-02 手动退选 refused 不持久化、B9-03 时钟同步失败无退避每 tick 轰炸）连根拔除**，前端 1 项维护收敛（F9-07 倒计时双实现合并共享 hook）；前端 4 项经读码推演判定误报（F9-01/02/03/05 澄清），另有 1 CRITICAL 候选（B9-04 probe 并发）合并入既有待办、5 项记录/推迟（B9-05/06/08/13、B9-07 待办）。

---

## 一、后端发现与修复状态（B9 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B9-01** | MAJOR | **手动报名/退选路径击穿重登指数退避**：B8-M7 手动路径命中 `ErrUnauthorized` 时先调 `MarkTokenValid(acct)` 再调 `MaybeRelogin`——`MarkTokenValid` 会 `delete(s.reloginFail, acct)`，指数退避恒从 30s 重来。Vision 持续故障时（每次失败后 next 手动操作再触发）退避表永不增长，平台锁号防线失效 | ✅ 手动路径只保留 `MaybeRelogin(acct)`，绝不先调 `MarkTokenValid`（后者只该用于"手动登录成功"的 issueSession 恢复路径）；失效标记由 maybeRelogin 自身置位。新增 `TestReloginBackoffWindowBlocksManualTriggers`（RED 复现"退避窗内触发发起新重登+清退避表"→GREEN） |
| **B9-02** | MAJOR | **手动退选 refused 不持久化**：`RemoveDone` 只写内存 `s.refused[acct][classID]=true`，重启后 `SetTargetsForAccount`（main.go 恢复路径也调用）会 `delete(s.refused, acct)`——自动引擎把用户手动退选掉的课当新目标重新抢回，退选意图丢失（与 B8-M2"假成功"同根） | ✅ 全链路持久化：schema 新增 `refused` 表 → `store.SaveRefused/DeleteRefused/LoadRefused` → `RemoveDone` 落库 + `SetTargetsForAccount` 清库行 → main.go 恢复序 `LoadRefused`→`RestoreRefused`（必须在 SetTargetsForAccount 之后注入）。`DeleteAccount` 事务同步清 refused 行。新增 `TestRefusedRecords`（store 往返+隔离+清空）、`TestRefusedPersistedAcrossRestart`（重启后 tick 不抢回，重设目标恢复） |
| **B9-03** | MAJOR | **时钟同步失败无退避，每 tick 轰炸**：B7/B8 只修了"成功才推进 lastSyncTime"——一旦同步失败（网络故障/平台拒绝），`lastSyncTime` 恒零，快路径被绕过，每个 tick（300ms）都试图发起；syncing 被 B8-M1 修成调用侧单飞后每次失败落地即复位，下一 tick 立即再发起，SyncServerTime 以 300ms 节奏反复打（绕开登录频率闸门、堆积在途、烧 token 与限流预算） | ✅ 新增失败退避：失败落地即 `s.lastSyncFailAt = time.Now()`，`maybeSyncClock` 在失败后 30s 窗口内直接返回；成功清 `lastSyncFailAt`（瞬断不拖延后续校准）。与 relogin 的 30s 基础节流对齐。新增 `TestClockSyncNoRetryWithinBackoff`（RED：5 拍连续发起 6 次→GREEN：退避窗内保持 1 次）；回归 `TestClockSyncFailureResetsOffset` / `TestClockSyncSuccessClearsFailStreak` 同步更新绿 |
| **B9-04** | CRITICAL-candidate | **probe 并发无单飞**：`probe()` 为每个目标账号 `go ProbeForAccount` 无单飞——N 账号并发时每 tick 同时打 N 个教务上游（带宽浪费，快照各自独立无数据污染，非数据级 CRITICAL） | ⏳ 合并入第 7 轮 B7-M5 待办（与 B8-M4 同源）。黄金期探测集中度由 lastProbe 全校闸门兜底，叠加 per-account 单飞收益有限，留待 D 系列 |
| **B9-05** | MINOR | `probe()` 用 `time.Now()` 本地时钟记录 lastProbe 与窗口判定——统一改用对齐时钟 `nowAligned`（B9-05 修正：真正修复需 probe 全程对齐，涉及 tick 传参，收益边际） | ⏳ 记录不修（窗口判定由 tick 内对齐时钟 + 探测确认双通道兜底，30s 闸门误差可接受） |
| **B9-06** | MINOR | `handleElectives` 无快照且无目标账号时 `ProbeNow` 直打教务上游——管理员预选目标前的冷页浏览会触发一次全校探测（每 30s 闸门内至多一次，可控） | ⏳ 已确认无功能影响，记录不修 |
| **B9-08** | MINOR | `MarkDone` 成功落库 SaveSuccess 后若 AppendLog 失败静默忽略——SQLite 单写者串行化下失败概率极低，忽略可接受 | ⏳ 已确认可接受，记录不修 |
| **B9-13** | INFO | `ElectivesSnapshotFor` 专属快照过期回退全局 lastData——年级可能错位（管理员穿透场景） | ⏳ 记录不修（穿透是管理员显式行为，回退保可用性优先） |

## 二、前端发现与修复状态（F9 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F9-07** | MINOR/MAINT | **倒计时双实现漂移**：Dashboard（F8-04 自 tick）与 Select（parseCountdown + 本地 setTick）各有一套倒计时实现，行为/样式易漂移，Select 本地每秒 setTick 还带动 Tab 徽章无谓重算 | ✅ 收敛：新建 `web/src/lib/useTickingCountdown.ts` 共享 hook（内部自 tick，只重渲染消费处），Dashboard/Select 共用；删除 Select 本地 setTick + parseCountdown 死代码；回归 tsc + build 绿 |
| **F9-01** | 误报 | handleElectives 提前 return（handler.go:224）被疑"升频读 stateData 与 handler 无关"——实际提前 return 是刻意的（有 account 参数直接返回专属快照），前端升频读 `stateData.window_opened`（Select.tsx:87）与 handler 无耦合 | ✅ 误报，读码推演成立，记录不修 |
| **F9-02** | 误报 | electives queryKey 被疑缺 account 维度致串账号——实际 `["electives", account, sessionToken]` 已含 account（Select.tsx:76） | ✅ 误报，记录不修 |
| **F9-03** | 误报 | saveNow 被疑并发交错——实际 `savingRef` 串行化已存在，`finally` 内 `savingRef=false` 后同步 `void saveNow()` 无异步交错窗口 | ✅ 误报，记录不修 |
| **F9-05** | 误报 | refetchInterval 读组件闭包 state/stateData 被疑"永不降频"——实际 `/state` 查询数据更新触发组件重渲染，react-query 用最新闭包重调度轮询间隔，闭包永不陈旧；logs 降频读 state、electives 降频读 stateData 均正确 | ✅ 误报，代码完全回退 + 注释澄清（此前误改的 `query.state.data` 版本已修掉 TS2339） |

## 三、修复细节（本轮 4 项生产改动，4 个独立 commit，均验证后提交）

- **B9-01**：handler.go 报名/退选 ErrUnauthorized 分支删除 `MarkTokenValid(acct)` 调用（保留 `MaybeRelogin`）；`TestReloginBackoffWindowBlocksManualTriggers` 红灯→绿灯
- **B9-02**：schema.sql 新增 refused 表 → store 三方法 + DeleteAccount 清行 → scheduler.Store 接口扩展（fakeStore 同步补）→ RemoveDone 落库 / SetTargetsForAccount 清库 / RestoreRefused 注入 → main.go 恢复序；`TestRefusedRecords` + `TestRefusedPersistedAcrossRestart` 红灯→绿灯
- **B9-03**：`maybeSyncClock` 失败退避（`lastSyncFailAt` + 30s 窗口）+ 成功清标记；`TestClockSyncNoRetryWithinBackoff` 红灯→绿灯；旧时钟测试回归同步更新
- **F9-07**：`web/src/lib/useTickingCountdown.ts` 新建共享 hook；Dashboard/Select 替换本地实现，删除 Select setTick 死代码与 TS2339/TS6133 错误源

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `go build ./...` + `go test -race -count=1 ./...` — 全包绿（api / scheduler / store / db / zhidao 等，含新测试）
- `cd web && npx tsc -b --force` + `npm run build` — 通过（406.28 kB → 405.76 kB / gzip 121.87 → 121.70）
- 提交序列见提交索引（B9-03→B9-01→B9-02→F9-07 四个独立 commit + 文档）

---

## 提交索引（本轮 4 个独立 commit + 文档）

```
1a86a5d fix(web): 第9轮F9-07倒计时收敛lib共用hook+F9-05误报澄清回退——Dashboard/Select同一useTickingCountdown实现，删除Select冗余setTick与ElectivesData错误window_closed读取
ea937a0 fix(backend): 第9轮B9-03时钟同步失败退避——失败落地记lastSyncFailAt，30s窗口内tick不重复发起SyncServerTime（此前每300ms轰炸）
74d83cd fix(backend): 第9轮B9-01手动报名/退选ErrUnauthorized路径不再先调MarkTokenValid——保留MaybeRelogin自动重登，退避表不被打穿（指数退避恒从30s重来的锁号防线复原）
61bb76a fix(backend): 第9轮B9-02手动退选refused持久化——schema加refused表+store三方法+RemoveDone落库/SetTargets清库+main重启RestoreRefused，重启后自动引擎不再抢回已退选课
```
