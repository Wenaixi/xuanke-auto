# R146 后端只读审查 —— 身份防线矩阵第六十一轮

审查基线与工作树
- 基线提交：68c0bfc（R145 归档，身份防线矩阵第六十轮里程碑闭合）
- 审查时间：2026-09-24
- 审查范围：backend/internal/{scheduler,api,accounts,zhidao,session} 全部相关源码/测试 + backend/main.go
- 模式：绝对只读，零代码改动（唯一写文件即本报告）

## 结论前置

| 级别 | 数量 | 汇总 |
|------|------|------|
| CRITICAL | 0 | 无 |
| HIGH | 0 | 无 |
| MEDIUM | 0 | 无 |
| LOW | 2 | 见"维持观察项"（均非本轮引入、无实际危害） |

**最终裁决：APPROVE（通过归档）**

## 验证表（全部实测）

| 验证项 | 命令 | 结果 | 实测耗时 |
|--------|------|------|----------|
| 编译 | `go build ./...` | 双绿（exit 0） | ~15s |
| 静态检查 | `go vet ./...` | 双绿（exit 0） | ~15s |
| 竞态 zhidao | `go test -race -count=1 ./internal/zhidao/...` | ok 1.919s | 实测 |
| 竞态 accounts | `go test -race -count=1 ./internal/accounts/...` | ok 1.337s | 实测 |
| 竞态 scheduler | `go test -race -count=1 ./internal/scheduler/...` | ok 15.013s | 实测 |
| 竞态 api | `go test -race -count=1 ./internal/api/...` | ok 13.885s | 实测 |
| 身份防线族十测（-race） | TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + TestMaybeReloginDeletedAccountSkipsMaps + 回归锚 | 7/7 PASS（3.652s） | 实测 |
| 回归锚 api | TestAdminStatsWindowOpenedUsesScheduler | PASS 0.43s | 实测 |
| 闸门族双向 | TestLoginByPasswordRejectsWhenGateBudgetExhausted / TestLoginByPasswordAllowedWhenGateBudgetAvailable | PASS×2 | 实测 |
| 全量测试 | `go test -count=1 ./...` | 12 包全 ok | 实测 |
| 轮次标签扫描（产品） | `grep -rn "第.*轮\\|round [0-9]\\|R[0-9][0-9] 轮\\|R[0-9][0-9]轮" internal/**/*.go`（非测试） | 零命中 | 实测 |
| 轮次标签扫描（测试） | 同正则 test 文件 | 5 处"第 N 轮"均属测试叙述/轮次轮询计数（时钟同步循环 i+1），非轮次前缀标签 | 实测 |
| 工作区漂移 | `git status --short` | 仅 r146-frontend findings 未跟踪（前端审查方产物）；后端零改动 | 实测 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十一轮闭合 —— ✅ 在位，零漂移

**sameClientFor 定义与 7 调用点逐一核实（scheduler.go）**

- 定义 :204-210：`ClientFor(acct)` 存在性 + `clientIdentity(current) == clientIdentity(chainClient)` 指针比对，注释明确"需持 s.mu"。helper `clientIdentity`（:215-224）用 `reflect.ValueOf(c).Pointer()` 取指针身份，nil 返回 0。
- 7 个调用点内容与终局确认：

| 调用点 | 分支 | 写什么状态/落什么库行 | 终局 |
|--------|------|----------------------|------|
| :850 | ProbeForAccount 回写段（M88-01 探测身份防线） | `openTimeDetected[acct]`（非空 beginTimes）+ `acctData[acct]` + `acctDataAt[acct]`。复核失败整体放弃回写，返回 `(data, nil)` 不含过期数据 | 年级串线/过期快照一帧可见场景被拦 |
| :1489 | spawnChain 失效分支（ErrUnauthorized） | 复核失败：清 inflight + 静默 return，**不写任何状态/日志/不触发 maybeRelogin**（防幽灵 reloginFail 污染重建身份） | 重登决策落在身份复核之后 |
| :1521 | spawnChain 成功分支 | `done[acct]` + `setStateLocked("success")` + AppendLog + SaveSuccess。失败静默放弃整链 | 假成功/重启假状态场景被拦 |
| :1551 | 风控退避 isRateLimitError | `markRateLimitedLocked` + `setStateLocked("failed")` + AppendLog。失败静默放弃 | 假"退避中"吞黄金期场景被拦 |
| :1571 | 窗口关闭 isWindowClosedError | `markFullLocked`。失败静默放弃 | 假"已满员"永久退避场景被拦 |
| :1600 | 实时复核回锁入口（B43-02 第六分支） | 三路判据（ErrUnauthorized→maybeRelogin / 确证满员→markFullLocked / 未现满员 failed）之前的统一身份闸 | 实时复核 ErrUnauthorized 写进新身份场景被拦 |
| :1635 | 实时复核确证满员分支 | `markFullLocked`。与 `doneHas` 前置（手动成功让位）组合 | 陈旧链反向写 full 场景被拦 |

7 调用点全部位于"网络往返后持锁写入前"的最后一道身份闸，零漂移；合约注释（B41-01 族）与实现逐字一致。

**maybeRelogin 双侧（OBSERVE-117-01 知识位第二十九轮）确认上位**
- 决策侧 :1208：`ClientFor(acct)` 存在性复核，已删账号不发起不写任何 relogin 族 map（TestMaybeReloginDeletedAccountSkipsMaps 固化四 map 零 key 契约）。
- 写回侧 :1254：重登完成先 `ClientFor(acct)` 复核，已删静默放弃整个成功分支（含内存写 + UpdateIDToken 落库）。
- :1265-1273 二次 `ClientFor(acct)` 重取**当前注册表** client → `client.Token()` 落库新 token。同名重建场景下第二次 ClientFor 拿到的是新身份指针，但旧链此时已在前置 :1254 被拦（同一把锁、同一次持有内复核），第二次重取属于成功分支内的"重登成功者自身"语义——绝不可能出现旧身份写新身份 token 的组合。知识位在位。

**手动五路 accountExists（handler.go）**
- :255 课程读（`handleElectives`）
- :305 手动报名（`handleElectiveSelect`）
- :397 手动退选（`handleElectiveExit`）
- :497-512 目标写（`handleSetTargets`）——**内联 LoadCredentials 逐账号比对**而非调用 accountExists：注释明确理由（仅 accounts 表会误伤 authenticateDirect 直连的已登录账号），与 accountExists（:1109-1120）判据同源（凭据表真理源）
- :573 状态读（`handleState`）

五路全到位，目标写内联形态与账户不存在整体拒绝语义无差异。

**写点换类 5 类 + warnedNoTargets 宿主唯一性**
- `lastSubmit`（:1346 submitAll 锁内 nowAlignedLocked 写入，读侧 tick :1029 锁内）
- `lastSyncStart`（:356 maybeSyncClock 锁内）
- `syncing`（:355 锁内置位 / :369,:404 锁内复位——含"无客户端不支持同步"复位路径 :403-406，TestClockSyncNoClientResetsSyncing 固化）
- `lastProbe`（:679 Start 主循环锁内回零 / :964 ProbeNow 锁内 / :1089 probe 失败锁内 / :1114 probe 成功锁内——全部持 s.mu，TestProbeNowConcurrentLocking 固化）
- `state.EmptyProbeRuns`（:1156/:1158 probe 锁内入账/归零）
- `warnedNoTargets`（:1371-1372）——**唯一无锁写点**。宿主唯一性射证：submitAll 仅由 tick（:1035）单 goroutine 调用（Start :666-684 主循环），全仓库无第二处写/读；实际无并发竞争，定向 -race 全绿实证。维持观察项（见下）。

**`*Locked` 写函数族 13 个 + 外部写函数首行取锁双向射证**
- `*Locked` 写函数（需持锁调用，全部只在锁内被调用）：markRateLimitedLocked（:1710）、markFullLocked（:1760）、releaseFullIfFreedLocked（:1781）、setStateLocked（:1846）、rebuildCoursesForAccountLocked（:570）、enrichTargetPubMetaLocked（:539）、rebuildCoursesLocked（:647）。只读 *Locked（开时间/判型/tokenValid 族）不写入。
- 外部写函数首行取锁：SetTargetsForAccount（:454）、PurgeAccount（:495）、RestoreTargets（:525）、RestoreDone（:607）、RestoreRefused（:626）、TryAcquireSubmit（:1896）、CheckClassSelectable（:1863）、MarkTokenValid（:1306 reloginMu→s.mu 双锁，与 maybeRelogin 锁序一致）、MarkDone（:1922）、RemoveDone（:1989）、RemoveFull（:2038）。
- 双向射证闭环：无锁外调用 *Locked 写函数的路径；无外部写函数漏首行取锁。

### 2. OBSERVE-117-01 知识位 —— ✅ 在位（见上 maybeRelogin 双侧明细）

### 3. B110-01 审计链第三十六轮 —— ✅ 零漂移

- **手动 6 失败位 AppendLog 逐一在位**：:362（报名 token 失效）、:374（报名 read 类）、:381（报名其余业务失败）、:443（退选 token 失效）、:452（退选 read 类）、:459（退选其余业务失败）。每处 `if err != nil { log.Printf }` 不吞错。
- **成功审计行**：手动报名成功经 MarkDone 内部 :1976 AppendLog；手动退选成功经 RemoveDone 内部 :2029；登录成功 :237 / 管理员登录 :136 / 登出 :616 / 设目标 :558 / 删账号 :1055 / 配置更新 :864 全在位。
- **自动链失败族**：spawnChain 六个分支（失效/成功/风控/窗口关闭/实时复核失效/其余失败）全部 `AppendLog` + `log.Printf` 落库失败留痕（:1504/:1532/:1558/:1570族/:1614/:1662）。
- **零吞错穷举**：产品代码 `_ =`/`_, _ =` 命中仅两处——handler.go :387 `_ = d.Sched.MarkDone(...)` 与 :467 `_ = d.Sched.RemoveDone(...)`。核证这两处返回 nil error（账号已删也静默返回 nil；内部落库失败已 log），忽略返回值无害，不违反契约。其余全部 `_ =` 在测试文件或无害路径（scheduler.go :307 prewarm、:1075 probe goroutine 探测、client.go :107/:124 io.Copy drain、config 写模板）。**零吞错契约成立**。
- **网络层 token 脱敏延续**：zhidao/client.go :450 doRequest 连接失败统一 `sanitizeError`（:581-593，剥 *url.Error 中完整 URL，Unwrap 保留判型链）；scheduler.go :1283 重登成功日志 `maskedToken(newTok)`（:1334 定义，幂等边界 ≤8 位打 `***`）。

### 4. O105-01 抖动基线 —— ✅ 在位且实测绿

- socketPreheat 夹具：zhidao/client_test.go :25-30（预创建-关闭 127.0.0.1 套接字）+ TestMain :39-42（包级兜底）+ loginMockServer 逐调用调用。
- readyProbe 夹具：zhidao/client_test.go :88-113（200ms×10 + 2s 显式超时宽栅栏）；api/handler_test.go :185 同款 + newTestDepsModeName :67-71 套接字预创建 + :139 就绪探测。
- 定向 `-race` 四包（zhidao/accounts/scheduler/api）实测全绿（见验证表）。双回归锚 TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler 实测 PASS。

### 5. LOW-132/133 回首核 —— ✅ 通过

`time.Since`/`time.Now()` 全量扫描结论：
- scheduler.go 残余 `time.Now()`/`time.Since` 仅在 reloginAt 写读同基处（:1220/:1227 读 `time.Since(t)`，:1231/:1261 写 `time.Now()`——同一本地基自洽，relogin 节流与退避窗口判读在同一时间基下）。
- accounts/manager.go gateWindow 族（gateWait :53-57 / gateTryAcquire :226-229 / GatePump :71-76）全部 `time.Now()` 写 + `time.Since` 读同基自洽。
- 时钟族写入侧已收敛到对齐钟：lastSyncFailAt（:372）、syncFailedWindow（:377）用 `nowAlignedLocked()`，对应读侧（:342）`now.Sub(lastSyncFailAt)` 传对齐 now——LOW-132 混用孤岛已消亡。
- loginLimiter（handler.go :1176-1195 tokenBucket）纯本地自洽。
- LOW-133（日志脱敏）由 maskedToken/sanitizeError 双保险延续覆盖——上述第三条已证。

**裁决：残余两项（reloginAt、gateWindow）均为写读同基自洽，合规。**

### 6. 新契约角度纵深（自选 ×2，选因说明）

**（a）登录时序攻击族（B43-04 撞名双条件 + loginTimingFlat）**

为什么选：登录是身份防线的对外边界，B42-01 闸门族（gateTryAcquire 非阻塞准入）与 B43-04 撞名分支是最近两个契约轮次的收口点，必须实证收敛而非停留在走读。

实测核证（handler.go :107-169 + :1161-1167）：
- 管理员入口双条件 `req.Account == adminName && subtle.ConstantTimeCompare(...)` 在位（:131）。
- 时延拉平完整性：正确口令分支 `time.Sleep(loginTimingFlat)`（:134）；撞名学生走教务登录、登录失败且名为 adminName 时 `time.Sleep(loginTimingFlat)` 后归因"管理口令错误"（:146-149）。管理员口令"对/错"两路响应均 ≈300ms 常数，侧信道无法从时延区分口令正确性。
- 撞名学生处理正确性：教务登录成功（LoginByPassword 通过）则走激活检查正常签发，绝不因撞名被吞（:142 不匹配时自然落入教务登录链路）。
- 闸门收口双向实证：TestLoginByPasswordRejectsWhenGateBudgetExhausted（quota 满 → 拒绝且 doLogin 0 次、无空壳残留）+ TestLoginByPasswordAllowedWhenGateBudgetAvailable（quota 充足 → 放行且 gateUsed=1 共享计数）实测双绿。
- 穿透确认：`LoginByPassword`（:244）与 `Relogin`（:173）共享 gateMu/gateUsed 同一计数，手动登录与排队重登严格共享每分钟 doLogin 预算。

**（b）删账号 memory-first 四序**

为什么选：身份防线矩阵的根是"删号同名重建"威胁模型，删账号时序的顺序正确性是全部同族防线的前置条件。

实测核证（handler.go :1015-1058）：
- 四序完整：① `Accounts.Remove`（:1040，memory-first——客户端先消失，在飞链锁内复核立即失败）→ ② `Sched.PurgeAccount`（:1045，全量清 done/tokenValid/relogin 族/acctData，绝不用 SetTargetsForAccount(nil) 半替代）→ ③ `Store.DeleteAccount`（:1046，清库行）→ ④ `Sessions.RevokeAccount`（:1054，已持有浏览器令牌立即失效）。
- 半删态自愈：DeleteAccount 失败时注册表与内存已清，重启 Restore 重建客户端自愈，注释明确"远优于假删除成功"。
- 与调度器侧防线闭环：PurgeAccount 后 `acctData`/`reloginAt`/`reloginFail`/`tokenValid`/`relogging` 全清（scheduler.go :495-518），同名重建后新身份从零开始，与 possiblyRelogin 决策侧复核、spawnChain 六分支 sameClientFor 形成完整闭环。

## 维持观察项（LOW）

1. **handler.go :387/:467 两处 `_ =`** 忽略 MarkDone/RemoveDone 返回 error——实测两函数恒返回 nil（内部落库失败已在函数内 log），语义无害；但若未来 MarkDone 演化出需要调用方感知的错误类型，`_ =` 会静默。建议未来改动触达时顺手改为 `if err := ...; err != nil { log.Printf }`（不阻塞归档）。
2. **scheduler.go:1371-1372 `warnedNoTargets` 无锁写**——宿主唯一性已证（submitAll 仅由 tick 主循环单 goroutine 调用），定向 -race 四轮实测无竞争；作为历史遗留形态维持观察，未来若引入并发调用 submitAll 的路径须先收口。

## 结尾建议

**APPROVE**。身份防线矩阵第六十一轮闭合：sameClientFor 7 调用点零漂移且每条追到终局写点；maybeRelogin 双侧 + 二次 ClientFor 重取当前 token 落库（OBSERVE-117-01 第二十九轮）在位；手动五路 accountExists 与目标写内联判据同源；写点换类 5 类 + warnedNoTargets 唯一无锁点全收口；B110-01 第三十六轮零吞错 + 脱敏延续零漂移；O105-01 四包 -race + 双回归锚实测绿；LOW-132/133 回首核通过。新契约纵深、登录时序拉平、撞名双条件与删号四序均实证在位。工作树除本报告外零改动。