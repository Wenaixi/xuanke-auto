# R116 后端只读审查报告

## 审查对象
- 仓库：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`（Go 后端 backend/）
- 审查 HEAD：`9c7a70b`（master，工作树干净）
- 审查方式：绝对只读，grep/Read/定向 go test 实证，全程未修改仓库任何文件（仅写本报告文件）
- 聚焦范围：身份防线矩阵第三十一轮闭合、OBSERVE-111-01/112-03 盯守第五轮、B110-01 审计链第六轮、B110-02 观察复核、O105-01 抖动基线、新契约角度（tick 状态机 + DB 迁移契约纵深走查）

## 分级发现

### CRITICAL / MAJOR / MINOR
无。本轮零产品改动链延续第十一轮，未发现够格立条的新缺陷。

### OBSERVE（延续 + 新）
- **OBSERVE-111-01 / OBSERVE-112-03 盯守（第五轮）——维持，零漂移**：`handler.go:377 _ = d.Sched.MarkDone(...)` 与 `:457 _ = d.Sched.RemoveDone(...)` 两处返回值丢弃非吞错——`MarkDone`（scheduler.go:1922-1982）与 `RemoveDone`（:1989-2035）内部全部落库点（SaveSuccess/DeleteSuccess/SaveRefused/AppendLog）均已 `if err != nil { log.Printf }` 日志化，函数恒返回 nil，丢弃无信息损失。手动失效分支（select:352 / exit:433）与自动链（scheduler:1504）同文案"教务令牌失效，自动重登中"，动作维度 select/exit 可区分。第五轮盯守确认零漂移。
- **OBSERVE（本轮新，纯记录）task_log 不随 DeleteAccount 事务删除**：`store.go:388-414` DeleteAccount 事务覆盖 credentials/accounts/targets/success/refused/activations 六表，`task_log` 刻意保留（审计线索，管理员日志总览 /api/admin/logs 可追溯已删账号历史动作）。这是审计保留的设计决策，非缺陷——读取侧 LoadLogs/LoadAllLogs 已用 `id > max(id)-20000` 窗口限制扫描成本，零删除零 DDL 契约文档化（store.go:226-233）。维持观察，不立条。

## 必查项逐条结论

### 1. 身份防线矩阵第三十一轮闭合——通过
- `git log --oneline 20c5882..HEAD -- backend/`：**仅 1 条 = `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志）**。零产品改动链延续第十一轮（R110-R115 后仅此一条，且为审计修复非产品改动），COUNT=1 确认。
- **sameClientFor 7 调用点逐点 grep 实证行号零漂移**：
  - `:204` 定义（`func (s *Scheduler) sameClientFor`）
  - `:850` ProbeForAccount 回写段（探测返回身份已变则放弃写快照/识别槽）
  - `:1489` spawnChain 失效分支（ErrUnauthorized）
  - `:1521` spawnChain 成功分支
  - `:1551` spawnChain 风控退避分支（isRateLimitError）
  - `:1571` spawnChain 窗口关闭分支（isWindowClosedError）
  - `:1600` spawnChain 实时复核统一入口（锁后先复核再分三路）
  - `:1635` spawnChain 实时复核确证满员分支
  七调用点行号与 R115 核位完全一致，定义注释（:199-210）声明"调用点：spawnChain 成功/失效分支写状态与落库前"，实际覆盖到六条 err 归并路径 + 探测回写，无漂移、无裸露新调用点。
- **写点全家福 12 类逐一对应防线（维持全量核位）**：
  - `openTimeDetected[acct]`（:856）/ `acctData`+`acctDataAt`（:863-864）：ProbeForAccount 回写段 `:850 sameClientFor` 前置，M88-01 身份防线在位。
  - `openTimeDetected["*"]`（:1108）/ 全局 `lastData`+`lastDataAt`+`lastProbe`（:1113-1116）：probe() 全局探测载体，用 `AnyClient()` 取任意客户端，无账号维度、不适用身份比对；其 ErrUnauthorized 分支（:1091-1097）先 `AnyClientWithAccount` 定位账号再 maybeRelogin（决策侧存在性复核兜底）。
  - `tokenValid`/`reloginAt`/`reloginFail`/`relogging`（:1231/:1232/:1236/:1241 决策侧 + :1260-1262 写回侧）：maybeRelogin **决策侧** `:1208 ClientFor(acct)` 存在性复核（B43-01 决策侧闭合）+ **写回侧** `:1254 ClientFor(acct)` 复核（B21-03/B43-01 写回侧闭合），双闭合在位。
  - `inflight`（:1471 置位、:1490/:1501/:1513 清位）：spawnChain 全分支 sameClientFor 同族覆盖。
  - `done`/`full`/`rateLimited`/`refused`（成功/风控/窗口关闭/确证满员分支写）：六分支身份复核前置。
  - `Courses` 状态（setStateLocked/rebuildCoursesForAccountLocked/MarkDone:1959-1971/RemoveDone:2012-2016）：全部随上述锁内写，网络往返后写分支均有身份/存在性复核前置；PurgeAccount（:495-519）全量清理 11 类 map + Courses 行。
- **偶发写点专项（维持）**：
  - `MarkTokenValid`（:1306-1319）：只 delete 不写，手动登录成功路径自身即新身份，无身份比对需求；锁序 reloginMu→s.mu 对齐在位。
  - `TryAcquireSubmit`（:1896-1914）：纯占位锁（sync.Once 释放），不写任何账号状态，无身份风险。
  - `releaseFullIfFreedLocked`（:1781-1811）：仅 spawnChain 持锁内调用，无网络往返前置、锁内写 full/状态，无需身份复核。
  - `submitAll`（:1342-1380）：只生成链，`:1355 ClientFor` 过滤已删账号，无状态写。
- **手动四路 accountExists + maybeRelogin 双侧**：`:245`（课程读 handleElectives）/`:295`（手动报名 handleElectiveSelect）/`:387`（手动退选 handleElectiveExit）/`:563`（状态读 handleState）逐行 grep 实证在位（handler.go:1099-1110 判据同源 LoadCredentials 逐账号比对）；maybeRelogin 决策侧 :1208 / 写回侧 :1254 在位。

**第三十一轮闭合结论：COUNT=1 零产品改动链延续第十一轮，seven 调用点行号零漂移，写点全家福 12 类防线全覆盖，主控裁定维持。**

### 2. OBSERVE-111-01/112-03 盯守（第五轮）——通过
见上文 OBSERVE 延续条目。`handler.go:377/:457` 两处 `_ =` 均非吞错（恒 nil 返回 + 落库点全日志化），手动失效分支与自动链同文案双日志动作维度 select/exit 可区分。零漂移。

### 3. B110-01 审计链（第六轮）——通过
- 手动失败三分支 AppendLog 六处全部在位：
  - select：`:352` 失效 / `:364` read / `:371` 业务失败
  - exit：`:433` 失效 / `:442` read / `:449` 业务失败
  全部 `if err := d.Store.AppendLog(...); err != nil { log.Printf }` 判断，零吞错。
- 成功路径审计行未回归：`MarkDone:1976`（"select" 成功日志）+ `RemoveDone:2029`（"exit" 成功日志）在位。
- 自动链 8 处 AppendLog（scheduler:1504/1532/1558/1614/1662/1770/1976/2029）全部 `if err != nil` 日志化，全仓库零吞错落库点（grep `_ = ` 仅命中 handler:377/457 两个恒 nil 返回点，非落库点）。
- 零漂移确认。

### 4. B110-02 观察复核——维持
reloginAt 本地钟写（`:1231 time.Now()` 决策侧 / `:1261 time.Now()` 写回侧）与读（`:1219/:1227 time.Since(t)`）同基自洽，未与对齐钟混用。`markRateLimitedLocked`（:1710-1715）已用对齐钟 `nowAlignedLocked()` 写入、`isRateLimitedLocked`（:1692-1705）用对齐钟读，写读同源。维持观察。

### 5. O105-01 抖动基线——通过
- `socketPreheat`（client_test.go:25-30）与 `readyProbe`（client_test.go:88+，api/handler_test.go:185-214）逐字符零漂移；accounts 包 readyProbe（manager_test.go:26）为三处夹具唯一例外（8 轮全绿实证，注释明示未来残余第一候选补 socketPreheat）。
- 定向测试实证（本轮全部绿）：
  - `go test -race -count=1 ./internal/zhidao/` → ok（3.0s）
  - `go test -race -count=1 ./internal/api/` → ok（20.2s）
  - `go test -race -count=1 -run "TestWindowOpenSubmitsWithoutProbeReset|TestAdminStatsWindowOpenedUsesScheduler|TestSubmitSuspendedWhenOpenTimeCleared|TestDeletedAccountRebuiltSameNameChain" ./internal/scheduler/` → ok（4.3s，含四个关键回归）
  - `go test -count=1 ./internal/db/` → ok（2.0s）
  - `go build ./...` → exit 0；`go vet ./...` → exit 0
  无失败、无低频残余。

### 6. 新契约角度（本轮自选：tick 主循环状态机 + 数据库迁移契约纵深走查）——通过
- **tick 主循环**（scheduler.go:973-1036）：nowAligned 单快照 → maybePrewarm/maybeSyncClock → 探测闸门（open 单快照复用 :987）→ 到点首 tick 立即探测（:989-991）→ 零值守卫让位于 WindowOpened（:1011 `open.IsZero() && !opened` 才挂起，B41-02 契约在位）→ WindowClosed 挂起（:1024）→ 提交节流（:1032）。全链路与契约 1/2/32 对齐。
- **windowClosedLocked 三判据单源**（:918-939）：主判据 state.WindowClosed（带 +10s 裕量，probe :1146 写入）/ 时钟连续失败 ≥3 + 开放时间已过 / 幽灵窗口 EmptyProbeRuns≥3 + 开放时间已过；open 取单次快照复用（:924）。WindowClosed()（:907-911）与 StateForAccount（:703）同源调用。契约 2 完整落地。
- **数据库迁移契约**（db.go:39-108 + db_test.go）：`migrateAddPublishMeta`（:43-57）在 refuseLegacy 之前调用（Open:28-34），逐列 columnExists 判存在缺才 ALTER；refuseLegacy 缺列清单（targets.priority / targets.allow_swap / task_log.account / settings 表）已剔除已迁移列（publish_name/begin_date 不在清单，注释:77-79 明确）；TestMigrateAddsPublishMetaColumns 覆盖旧库真实数据行 → Open 成功 → 两列补齐 + 旧行保留。契约与 TDD 守护零漂移。
- **accounts 管理器生命周期**（manager.go）：Remove（:125-135）清 clients+order；Relogin（:166-175）内部 gateWait（:173）收口全局 doLogin 闸门（B42-01 实测：scheduler 走接口 Relogin → 真实实现 manager.Relogin 内调 gateWait，闸门生效）；LoginByPassword（:243-246）经 gateTryAcquire 非阻塞准入（B42-01 手动旁路闭合）；ResetGateForTest 仅测试专用。契约 30/33 实锤。

## 验证表
| 命令 | 结果 |
|------|------|
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 57bf401 一条（COUNT=1） |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test -race -count=1 ./internal/zhidao/` | ok 3.0s |
| `go test -race -count=1 ./internal/api/` | ok 20.2s |
| `go test -race -count=1 -run "TestWindowOpenSubmitsWithoutProbeReset\|TestAdminStatsWindowOpenedUsesScheduler\|TestSubmitSuspendedWhenOpenTimeCleared\|TestDeletedAccountRebuiltSameNameChain" ./internal/scheduler/` | ok 4.3s |
| `go test -count=1 ./internal/db/` | ok 2.0s |
| grep sameClientFor 全部非测试调用点 | :204/:850/:1489/:1521/:1551/:1571/:1600/:1635 共 7 调用点，行号零漂移 |
| grep AppendLog 手动三分支六处 + 自动链八处 | 全部 if err 判断，零吞错 |
| grep `_ =` 落库点扫描 | 仅 handler:377/457 两个恒 nil 返回点 |

## 已核无缺陷清单
1. 身份防线矩阵第三十一轮：seven 调用点行号零漂移、写点全家福 12 类防线全覆盖、偶发写点四类无裸露。
2. B110-01 审计链第六轮：手动失败三分支六处 AppendLog + 成功路径审计行全部在位，零吞错。
3. B110-02 reloginAt 本地钟写读同基自洽。
4. O105-01 socketPreheat/readyProbe 夹具逐字符零漂移，定向 -race 全绿。
5. tick 主循环零值守卫/B41-02 让位、windowClosedLocked 三判据单源 + 单快照复用。
6. DB 迁移契约（migrateAddPublishMeta 先于 refuseLegacy、缺列清单已剔除迁移列、TDD 守护在位）。
7. accounts 管理器：Relogin 经 gateWait、LoginByPassword 经 gateTryAcquire，双路收口全局登录频率闸门。
8. 手动四路 accountExists + maybeRelogin 决策/写回双侧存在性复核在位。
9. 契约 20 合规：本轮抽查注释无"第 N 轮"轮次前缀标签；契约 17 合规：日志中 token 均 maskedToken/前 8 位。

## 结论
R116 后端审查通过。**身份防线矩阵第三十一轮闭合**：20c5882..HEAD 后端仅 57bf401 一条（B110-01 审计修复），零产品改动链延续第十一轮（R110-R115 后 COUNT=1 延续），sameClientFor 7 调用点行号零漂移，写点全家福 12 类防线全覆盖，偶发写点无裸露，手动四路 + maybeRelogin 双侧存在性复核在位。**OBSERVE-111-01/112-03 盯守第五轮维持**：handler.go:377/:457 `_ =` 非吞错（恒 nil + 落库点全日志化），手动失效分支与自动链同文案双日志动作维度 select/exit 可区分。**B110-01 审计链第六轮零漂移**：手动失败三分支六处 AppendLog 在位 + 成功路径审计行未回归，全仓库零吞错落库点。定向 -race 测试（zhidao/api/scheduler 关键回归/db）+ build + vet 全绿，无低频残余。本轮新契约角度（tick 状态机 + DB 迁移契约 + accounts 生命周期）纵深走查通过，未立新条。稳定期本轮无 CRITICAL/MAJOR/MINOR，延续 OBSERVE-111-01/112-03 盯守至第六轮。
