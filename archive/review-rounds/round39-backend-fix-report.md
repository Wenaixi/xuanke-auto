# Round39 后端缺陷修复报告

TDD 严格模式，先写测试跑红再实现跑绿。全部修复已提交（未 push），每处修复独立 commit。

## B39-05（主控追加）：admin stats 缺 window_closed 字段

- **测试**：`TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go 追加断言：stats 必须下发 `window_closed` 且与 `d.sched.WindowClosed()` 同源）
- **修复前红**：字段缺失（`window_closed` 不在响应 map 中）
- **修复后绿**：`api` 包全量回归通过
- **commit**：`8489d07`（feat(api): admin stats 补发 window_closed 字段）
- **说明**：`handleAdminStats` 响应 map 补 `"window_closed": d.Sched.WindowClosed()`，与学生端 /state 同源（WindowClosed 三判据单源 windowClosedLocked）。前端 Admin StatsTab 三态展示（待命中/已开放/已关闭）依赖此字段，缺失时窗口关闭后后台仍显"待命中"误导管理员。前端 `web/src/routes/Admin.tsx:714-718` 已就绪（读 stats.window_closed），本次纯后端补发。

## B39-04（MINOR）：emptyRunsFor 死代码

- **测试**：无需新测试（纯删除），grep 实证无调用点
- **修复后绿**：build + scheduler 全量测试通过
- **commit**：`5211761`（refactor(scheduler): 删除无调用点的 emptyRunsFor 死代码）
- **说明**：判据签名已收敛到 `windowClosedLocked()` 单源（主判据/时钟兜底/幽灵窗口兜底三判据），`emptyRunsFor` 带 `_ = open` 假签名，无任何调用点。

## B39-03（MINOR）：重登成功分支 UpdateIDToken 无 nil store 守卫

- **测试**：`TestReloginSuccessWithNilStoreNoPanic`（scheduler_test.go，store=nil + 重登成功路径不 panic 且 token 恢复有效）
- **修复前红**：`panic: runtime error: invalid memory address or nil pointer dereference`（scheduler.go:1215，改稿前实测）
- **修复后绿**：race 模式同样通过
- **commit**：`874cec3`（实现）+ `f32bf8c`（测试）
- **说明**：全仓库唯一裸写落库点，补 `if s.store != nil { ... }` 外衣，与 1469-1478 同款模式。

## B39-02（MINOR）：writeJSON 永不写 HTTP 状态码

- **测试**：
  - `TestRecoverMiddlewareHidesPanicDetail` 追加：panic 后 HTTP 必须 500（修复前红：实际 200）
  - `TestAuthRequired`：401 HTTP 状态码真实 401
  - `TestAdminAuth`：403 真实 403
  - `TestLoginRateLimit` / `TestLoginActivateSeparateBuckets`：429 真实 429
  - `TestLoginRejectsFormContentType`：CSRF 403 真实 403
  - `TestAdminStatsAccountsLogs` 中旧注释"HTTP 200 + body code 恒为约定"的 401 断言同步更新为真实 401
- **修复前红**：`panic 恢复应写 HTTP 500（修复前恒 200），实际 200`
- **修复后绿**：api 包全量回归通过
- **commit**：`db04008`
- **说明**：新增 `writeJSONStatus(w, status, code, data, msg)`，只改 4 类基础设施调用点：recoverMiddleware 500 / requireAuth 401 / requireAdminSession 403 / 登录+激活限流 429。`writeJSON` 本体与其余 100+ 业务调用点不动——前端契约只读 body code 字段（`web/src/api/client.ts` 从不读 HTTP status），HTTP 状态码为纯增强，不破坏现有前端。router.go 的 CSRF-403 与 requireJSONBody 同时改（同为基础设施路径）。

## B39-01（MAJOR）：删号后同名重建账号被陈旧在飞链寄生

- **测试**：`TestDeletedAccountRebuiltSameNameChainDropsSuccess`（scheduler_test.go，新夹具支持 per-account 客户端映射）
- **修复前红**：`同名重建后陈旧旧链成功不得写回重建身份（B39-01），实际 1 行 success`
- **修复后绿**：race 模式 + 删号竞态族全量回归通过
- **commit**：`5376bb6`
- **说明**：
  - 根因：spawnChain 成功/失效分支只判"账号名当前在注册表"（`ClientFor(acct)` ok），不判"客户端是否仍是发起提交时的同一身份"。删号后同名重建（换绑/误删加回）会用新 `*zhidao.Client` 顶替，旧链 SelectClass 往返期间被顶替，返回后 ClientFor 仍 ok（新身份）→ 旧链把 success 状态与 success 行写进**重建身份**（重启后 RestoreDone 恢复成假成功、已删账号状态复活）。
  - 修复：链 goroutine 启动时捕获发起提交的 client（`chainClient := client`），成功分支（1455 行区）与失效分支（1430 行区）复核均改为 `sameClientFor(acct, chainClient)`——`ClientFor(acct)` 与捕获指针做**指针身份比对**（`clientIdentity` 用 `reflect.ValueOf(c).Pointer()`，对 `*zhidao.Client` 与测试 `*fakeClient` 都取底层指针值）。
  - 已删账号（ClientFor 不存在）天然落在 `sameClientFor` false，B18-M2 原防线语义保留。
  - 重登分支（1208 行区）核实：重登 goroutine 内 Relogin 中途账号被删再重建的场景，重登结果属于"整个会话级"的新身份——但重登成功分支的落库 `UpdateIDToken` 是把新身份自己的新 token 写回同账号名，且客户端存在性复核已由 B21-03 覆盖（成功分支还通过 `客户端.Token()` 读回新身份自己的 token），不涉及"旧身份写新身份"的跨身份寄生，无需再加身份比对。
  - 夹具 `fakeAccts` 扩展 `perAccount map[string]*fakeClient`（按账号返回独立客户端指针，`AnyClientWithAccount` 同步适配），既有测试共享 `c` 的行为不变。

## 全量回归

```
cd backend && go build ./...   # OK
go vet ./...                   # OK
go test -race -count=1 ./...   # 9 包全绿（accounts/api/config/db/runtime/scheduler/secure/session/store/zhidao）
```

- 前端契约核验：`web/src/api/client.ts` 只读 body code，从不读 HTTP status；本轮 HTTP 状态码增强不破坏前端。
- 未发现有违反 `if err != nil { log.Printf }` 零吞错规范的新增落库点；既有代码中的落库点均合规。
- 提交历史（master，未 push，均中文信息）：
  - `8489d07` B39-05、`5211761` B39-04、`f32bf8c`+`874cec3` B39-03、`db04008` B39-02、`5376bb6` B39-01