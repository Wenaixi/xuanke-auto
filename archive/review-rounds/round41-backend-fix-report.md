# round41 后端修复报告（TDD 严格模式）

> 修复基线：master @ eb750d8（round40 全量落盘）。只动 backend/ 目录（并行前端代理独占 web/）。
> 回归命令：`cd backend && go build ./... && go vet ./... && go test -race -count=1 ./...`（9 包全绿）。
> 决策历史归项目根 CLAUDE.md《工程决策手册》，代码注释只写"为什么/契约/陷阱"。

---

## B41-01（CRITICAL）spawnChain 风控退避/窗口关闭/实时复核满员三条 err 分支补 B39-01 指针身份复核

**缺陷本质**：B39-01 的"同一身份"防线只覆盖 spawnChain 成功分支与失效分支；`isRateLimitError`（风控退避）与 `isWindowClosedError`（窗口关闭）两条 err 归并分支，以及实时人数复核确证满员分支，只做账号名存在复核（`ClientFor` ok）——删号后同名重建时新身份存在但指针不同，陈旧链命中这三类错误会把 rateLimited/full 写进重建身份（假"已满员"永久退避黄金期 / 假"退避中"）。

**修复**：三处写 full/rateLimited 前补 `s.sameClientFor(acct, chainClient)`（持锁内方法，reflect 指针身份比对），false 即静默放弃整链（清 inflight 后 return），与成功/失效分支同族。

### 测试（TDD：先红后绿）

| 测试名 | 位置 | 修复前红 | 修复后绿 | 说明 |
|--------|------|---------|---------|------|
| `TestDeletedAccountRebuiltSameNameChainDropsRateLimitBackoff` | scheduler_test.go:3115 | `同名重建后陈旧旧链风控退避不得写回重建身份的 rateLimited` | PASS | 风控退避分支：旧链命中"操作过于频繁，请稍后重试"文案，重建身份不得落 rateLimited |
| `TestDeletedAccountRebuiltSameNameChainDropsWindowClosedFull` | scheduler_test.go:3168 | `同名重建后陈旧旧链窗口关闭不得写回重建身份的 full` | PASS | 窗口关闭分支：旧链命中"不在选修报名时间范围内，无法选课！"文案，重建身份不得落 full |
| `TestDeletedAccountRebuiltSameNameChainDropsRealtimeRecheckFull` | scheduler_test.go:3217 | `同名重建后陈旧旧链实时复核满员不得写回重建身份的 full` | PASS | 实时复核满员分支：旧链网络失败走复核，重建后新客户端 IsClassFull 确证满员，回锁 markFullLocked 前身份比对拦截 |

三测试复用 R39 的 `fakeAccts.perAccount` 同名重建夹具（删号后 perAccount 值被替换成新 *fakeClient，链顶捕获旧指针 → 身份比对必然不等）。新增 `waitChainExit` 等待辅助（chains 活跃标记消失 = goroutine defer 已执行，绝不用 inflight 等待——PurgeAccount 后 inflight map 已删，读 nil map 恒 false 会假绿，与 R39/TestRealtimeRecheckDeletedAccountDropsLog 同款等待契约）。

---

## B41-02（MAJOR）tick 零值守卫在 WindowOpened=true 时仍挂起提交

**缺陷本质**：`if open.IsZero() { return }` 先于 `!opened && !now.After(open)` 与 `WindowClosed()` 两条判据返回。probe 用发布级 `inDateRange` 确证开窗（WindowOpened=true）但识别槽为空（平台批次未下发非空 beginTimes）时，自动链被永久挂起、黄金期 250ms 冲刺 0 次——前端显 window_opened=true 而引擎从未提交，两处"开窗"判据产品语义分叉。

**修复**：零值守卫改 `if open.IsZero() && !opened { return }`（窗口确证开启后不再依赖开窗时刻）。B11-A1 的防轰炸本意针对"未开窗"场景，WindowOpened=true 时轰炸目标已消失。

### 测试（TDD：先红后绿）

| 测试名 | 位置 | 修复前红 | 修复后绿 | 说明 |
|--------|------|---------|---------|------|
| `TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime` | scheduler_test.go:3294 | `WindowOpened=true 且识别槽空时 tick 必须放行提交（黄金期 250ms 冲刺），实际 0 次 SelectClass` | PASS | InDateRange=true 且 BeginTimes=nil（识别槽恒空）+ 预置 WindowOpened=true，tick 必须进提交段调 SelectClass |

既有守卫测试 `TestSubmitSuspendedWhenOpenTimeCleared`（夹具 WindowOpened 默认 false）修复后仍绿，守卫语义对"从未开过窗"保持不变。

---

## 修改文件与 commit

| Commit | 说明 |
|--------|------|
| `46a991a` | fix(scheduler): spawnChain 风控退避/窗口关闭/实时复核满员补同名重建指针身份复核 + 窗口已开时零值守卫放行提交（B41-01 + B41-02，两缺陷同处 spawnChain/tick 两函数，一次提交；含 4 个 TDD 测试） |

- `backend/internal/scheduler/scheduler.go`（+32/-3）：零值守卫加 `&& !opened`；1516/1530/1582 三处 markRateLimitedLocked/markFullLocked 前补 `sameClientFor` 复核。
- `backend/internal/scheduler/scheduler_test.go`（+222）：4 个新测试 + `waitChainExit` 辅助。

**未触碰**：web/ 目录零改动；无新落库点（三处复核均为纯内存防线）；零吞错落库点契约未受影响。

---

## 全量回归（提交后实证）

```
cd backend && go build ./...        → 通过
cd backend && go vet ./...          → 通过
cd backend && go test -race -count=1 ./... → 9 包全绿
  ok  internal/accounts / api / config / db / runtime / scheduler / secure / session / store / zhidao
  ?   cmd/probe / web（无测试文件）
```

scheduler 包既有相关测试多轮重跑稳定：`TestRealtimeFullRecheckWithNoManualDoneMarksFull`、`TestRealtimeFullRecheckKeepsManualSuccess`、`TestRealtimeRecheckDeletedAccountDropsLog`、`TestWindowOpenSubmitsWithoutProbeReset`、`TestSubmitSuspendedWhenOpenTimeCleared` 全绿（含一次 api 包 race 偶发网络超时失败，已在无改动基线复现为环境抖动、非本轮引入）。
