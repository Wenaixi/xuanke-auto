# round43 后端修复报告（TDD 严格模式）

日期：2026-09-20

## B43-01（MAJOR）：maybeRelogin 入口无账号存在性复核——探测定时三处直调可污染同名重建账号

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/scheduler/scheduler.go` maybeRelogin 入口（两把锁内只有 relogging/reloginFail/reloginAt 三组 map 检查，无 ClientFor 存在性复核）；调用点三处对 ErrUnauthorized 无条件直调：ProbeForAccount / ProbeNow / probe |
| 测试 | `TestMaybeReloginDeletedAccountSkipsMaps`（红→绿） |
| 修复前红 | FAIL——已删账号直调 maybeRelogin 后四 map 均残留 key（`tokenValid=true reloginFail=true reloginAt=true relogging=true`） |
| 修复后绿 | PASS——入口先 ClientFor 复核，已删账号静默放弃，四 map 均无 key |
| commit | `f75ddff` fix(scheduler): maybeRelogin 入口补账号存在性复核，探测定时路径不再污染同名重建账号（B43-01） |
| 说明 | 入口 `s.mu.Lock()` 后、任何 map 写入前补 `if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`。B21-03 只护重登 goroutine 写回侧，决策侧裸露；删号与在飞探测返回 ErrUnauthorized 同帧（约 15s）时会把 tokenValid/reloginFail/reloginAt/relogging 写进已删账号 map（PurgeAccount 已清），同名重建后新账号 tokenValid 残留 true（前端"已失效"）+ spawnChain 整链挂起 + 首登无辜退避 30s。与 spawnChain 失效分支 B42-02 先身份复核同族防线。 |

## B43-02（MAJOR）：spawnChain 实时复核 ErrUnauthorized 分支无指针身份复核

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/scheduler/scheduler.go` spawnChain 非满员失败 → classFullRealtime → cErr ErrUnauthorized 分支 |
| 测试 | `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（红→绿，复用 `TestDeletedAccountRebuiltSameNameChainDropsRealtimeRecheckFull` 同名重建夹具） |
| 修复前红 | FAIL——`relogCalls == 1`：同名重建后旧链命中新身份 ErrUnauthorized 仍触发 maybeRelogin |
| 修复后绿 | PASS——`relogCalls == 0`、reloginFail 无残留、重建身份 state 无 failed 残留 |
| commit | `f6fd89e` fix(scheduler): 实时复核失效分支补指针身份复核，闭合身份防线最后一块裸露写点（B43-02） |
| 说明 | 回锁后 1575 行从 `ClientFor(acct)` 存在性复核升级为 `sameClientFor(acct, chainClient)`（存在性已内含）——同名重建（注册表现指针已换）后旧链命中新身份的 ErrUnauthorized，ClientFor 仍 ok（新身份存在）却会把 maybeRelogin/failed 状态写进新身份（无辜消耗登录预算）。B39-01/B41-01 同族防线补上最后一块裸露写点，与成功/失效/风控/窗口关闭/确证满员五分支对称。 |

## B43-03（MAJOR/实测归因）：成功分支身份复核失败时 inflight 位漏删——实测不成立

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/scheduler/scheduler.go` 1502 行 |
| 测试 | `TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight`（恒绿契约回归，无红色可见） |
| 实测结论 | **红不了**——1502 行 `delete(s.inflight[acct], t.ClassID)` 在任何分支判定前统一清位（含身份复核失败路径），与失效分支 1479/1490 行对称，原审查"成功分支身份复核失败时 inflight 位漏删"不成立 |
| commit | `0812611` test(scheduler): 固化成功分支身份复核失败清 inflight 位契约回归（B43-03） |
| 说明 | 审查 MAJOR-43-03 基于"成功分支身份复核失败直接 return 未清 inflight"的代码走读推断；实测该行（SelectClass 返回后 `if err == nil` 判定前的公共清位）已存在，同名重建 + 链挂起 + 放行 + PurgeAccount 后 inflight 恒无残留。固化为契约回归测试落库（后续改动破坏该行会红）。无代码改动，非 TDD 项。 |

## B43-04（MAJOR）：handleLogin 管理员名撞名学生账号修正

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/api/handler.go` handleLogin |
| 测试 | `TestLoginAdminNameCollisionStudentCredential`（红→绿）<br>`TestLoginAdminWrongPasswordTimingFlat` / `TestAdminAuth`（随语义修正更新） |
| 修复前红 | FAIL——撞名学生用教务口令登录返回"管理口令错误"（永远无法登录，DoS） |
| 修复后绿 | PASS——撞名学生走教务登录正常签发普通会话、绝不带管理员权限；管理员名 + 错误口令仍返回"管理口令错误" |
| commit | `f1d6792` fix(api): 管理员名与教务学生账号撞名不再吞——仅管理口令匹配才走管理员分支（B43-04） |
| 说明 | 管理员分支条件从 `req.Account == adminName` 改为 `req.Account == adminName && subtle.ConstantTimeCompare(...) == 1`：仅"管理员名 + 管理口令都匹配"才走管理员签发；反向撞名（学生用自己的教务口令登碰巧叫 adminName 的号）放行教务登录分支正常登录。错误文案分支保留在 `req.Account == adminName && 教务登录失败` 后（管理口令错误的 n4/loginTimingFlat 语义保留）。既有测试 `TestLoginAdminWrongPasswordTimingFlat`/`TestAdminAuth` 原断言"管理员名+错口令必回管理口令错误"在 mock 平台教务全成功下不再成立，已按 B43-04 新契约更新（撞名学生绝不被吞）。 |

## B43-05（MINOR）：handleAdminStats targetsCount 吞错记日志报错

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/api/handler.go` handleAdminStats targetsCount 循环 |
| 测试 | `TestAdminStatsTargetsLoadFailureReturns500`（契约回归） |
| 修复前 | `LoadTargetsForAccount` 失败静默 continue → targets_count 缺算，DB 故障时误导管理员"无人设目标" |
| 修复后 | 失败记日志 + 累计 firstErr，循环后 `if firstErr != nil { writeJSONStatus(w, 500, 1, nil, "统计目标数失败: "+...) }`，对齐其他数据源"任一失败即 500"风格 |
| commit | `d30a562` fix(api): admin stats 目标数统计失败记日志并明确报错，不再静默计 0（B43-05） |
| 说明 | `Deps.Store` 为具体 `*store.Store`（非接口）无法注入失败替身，失败路径报错语义走实现复查 + 正常路径契约回归（`TestAdminStatsAccountsLogs` 既有 targets_count 计数 + 新增 stats 200/code=0 回归）；把 Store 改成接口属超范围改动（违反简洁优先）故意不为。 |

## 全量回归

`cd backend && go build ./... && go vet ./... && go test -race -count=1 ./...` → **9 包全绿**（accounts / api / config / db / runtime / scheduler / secure / session / store / zhidao，含 scheduler -race）。

回归副作用处理：api 包 `TestLoginAdminWrongPasswordTimingFlat` / `TestAdminAuth` 原断言"管理员名 + 错误口令必返回管理口令错误"，在 B43-04 后将"先试教务登录（mock 平台教务全成功 → 撞名学生登录成功/1001 颁发票据）"——按新契约更新断言（撞名学生绝不被管理员分支吞掉），管理口令错误文案分支保留（真实平台教务 doLogin 对该口令也失败时触达）。

## 变更文件

- `backend/internal/scheduler/scheduler.go`（B43-01 maybeRelogin 入口复核；B43-02 实时复核身份复核）
- `backend/internal/scheduler/scheduler_test.go`（TestMaybeReloginDeletedAccountSkipsMaps / TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin / TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight）
- `backend/internal/api/handler.go`（B43-04 撞名双条件；B43-05 stats 报错）
- `backend/internal/api/handler_test.go`（撞名测试 + 两个既有测试按新契约更新 + stats 契约回归）

未触碰 web/ 目录；未 push。