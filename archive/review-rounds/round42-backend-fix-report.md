# round42 后端修复报告（TDD 严格模式）

日期：2026-09-20

## B42-01（MAJOR）：学生手动登录绕行全局 doLogin 频率闸门

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/accounts/manager.go` `LoginByPassword` |
| 测试 | `TestLoginByPasswordRejectsWhenGateBudgetExhausted`（红→绿）<br>`TestLoginByPasswordAllowedWhenGateBudgetAvailable`（对偶守卫，红→绿） |
| 修复前红 | 两测试均 FAIL——quota 已满仍放行触达 doLogin；成功登录不消耗闸门预算 |
| 修复后绿 | 两测试均 PASS——quota 已满立即拒绝且 doLogin 0 次、无空壳客户端残留；quota 充足正常成功并消耗 1 次预算 |
| commit | `e1394d0` fix(accounts): 学生手动登录收口全局 doLogin 频率闸门（非阻塞准入） |
| 说明 | 新增 `Manager.gateTryAcquire() bool` 非阻塞准入（与 `gateWait` 同一把 `gateMu`、共享 `gateWindow`/`gateUsed` 计数，先 Lock 再检查/消耗无死锁），`LoginByPassword` 在 `c.Login` 前调用，`!ok` 返回"登录尝试过于频繁，请稍后再试"；管理员换绑同走此收口（低频操作被拦重试即可），管理员登录入口（handleLogin 单独分支）不触碰教务登录不受影响。 |

## B42-02（MINOR）：spawnChain 失效分支先 maybeRelogin 后身份复核

| 项 | 值 |
|----|----|
| 缺陷位置 | `backend/internal/scheduler/scheduler.go` spawnChain 失效分支 |
| 测试 | `TestDeletedAccountRebuiltSameNameChainDropsRelogin`（红→绿，参考 `TestDeletedAccountRebuiltSameNameChainDropsSuccess` 同名重建夹具） |
| 修复前红 | FAIL——`relogCalls == 1`：身份已变仍触发 maybeRelogin，幽灵 reloginFail 污染重建身份 |
| 修复后绿 | PASS——`relogCalls == 0`，重建身份 reloginFail 无残留 |
| commit | `a8f1856` fix(scheduler): spawnChain 失效分支先身份复核再触发自动重登 |
| 说明 | 失效分支将 `maybeRelogin` 移到 `sameClientFor` 指针身份复核之后：身份已变（删号/同名重建）静默放弃整链、绝不调 maybeRelogin（已删账号经 Manager.Relogin 的 ClientFor 失败仍会写 reloginFail 污染）；身份为同一发起链才触发重登。 |

## 全量回归

`cd backend && go build ./... && go vet ./... && go test -race -count=1 ./...` → **9 包全绿**（accounts / api / config / db / runtime / scheduler / secure / session / store / zhidao，含 scheduler -race）。

回归副作用处理：api 层夹具 `authenticateDirect`（语义"纯注册账号建会话"）在同一分钟窗口内批量注册多个账号会撞上 B42-01 准入闸门——新增 `Manager.ResetGateForTest()` 测试专用重置，`TestAccountOverrideRequiresAdminSession` 注册三个账号前逐个重置预算，正式代码不调用。

## 变更文件

- `backend/internal/accounts/manager.go`（gateTryAcquire + LoginByPassword 准入 + ResetGateForTest）
- `backend/internal/accounts/manager_test.go`（2 个新测试 + gateSrv 夹具）
- `backend/internal/scheduler/scheduler.go`（失效分支顺序调整）
- `backend/internal/scheduler/scheduler_test.go`（1 个新测试）
- `backend/internal/api/handler_test.go`（夹具 ResetGateForTest 调用）

未触碰 web/ 目录；未 push。
