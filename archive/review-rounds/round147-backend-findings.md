# R147 后端只读审查 —— 身份防线矩阵第六十二轮

审查基线与工作树
- 基线提交：3c3e915（R146 归档，身份防线矩阵第六十一轮闭合）
- 审查时间：2026-09-24
- 审查范围：backend/internal/{scheduler,api,accounts,zhidao,secure,session,store} 全部相关源码/测试 + backend/main.go
- 模式：绝对只读，零代码改动（唯一写文件即本报告）

## 结论前置

| 级别 | 数量 | 汇总 |
|------|------|------|
| CRITICAL | 0 | 无 |
| HIGH | 0 | 无 |
| MEDIUM | 0 | 无 |
| LOW | 2 | 见"维持观察项"（均非本轮引入、无实际危害，延续 R146 历史遗留） |

**最终裁决：APPROVE（通过归档）**

## 验证表（全部实测）

| 验证项 | 命令 | 结果 | 实测耗时 |
|--------|------|------|----------|
| 编译 | `go build ./...` | 双绿（exit 0） | 实测 |
| 静态检查 | `go vet ./...` | 双绿（exit 0） | 实测 |
| 竞态 zhidao | `export PATH=/d/mingw64/bin:$PATH` + `go test -race -count=1 ./internal/zhidao/` | ok 2.095s | 实测 |
| 竞态 accounts | 同上 `./internal/accounts/` | ok 1.383s | 实测 |
| 竞态 api | 同上 `./internal/api/` | ok 14.590s | 实测 |
| 竞态 scheduler | 同上 `./internal/scheduler/` | ok 15.258s | 实测 |
| 身份防线族十测（-race 定向） | `go test -race -run 'TestDeletedAccount\|TestProbeDeletedThenRebuilt\|TestProbeForAccountDropsWriteWhenRemoved\|TestProbeChainSameClientIdentity\|TestMaybeReloginDeletedAccountSkipsMaps\|TestRealtimeRecheckDeletedAccountDropsLog\|TestUnauthorizedBranchDeletedAccountSkipsState\|TestSpawnChainSkipsWhenTokenInvalid' ./internal/scheduler/` | ok 2.405s（含同名重建五分支族） | 实测 |
| 回归锚 | `go test -run 'TestWindowOpenSubmitsWithoutProbeReset\|TestAdminStatsWindowOpenedUsesScheduler' ./internal/scheduler/ ./internal/api/` | 双 PASS（1.02s / 0.45s） | 实测 |
| 时钟/窗口族 | TestClockSyncNoClientResetsSyncing / TestWindowClosedState / TestProbeNowConcurrentLocking | 三 PASS | 实测 |
| 脱敏判型族 | TestSanitizeErrorStripToken / PreservesJudgment / TestIsReadErrCoversAllForms | 全 PASS | 实测 |
| 全量测试（CGO=1 含 -race） | `CGO_ENABLED=1 go test -race -count=1 ./...` | 13 包全 ok（store 24.791s 最重） | 实测 |
| 轮次标签扫描（产品） | `grep -rn "第.*轮\|round [0-9]\|R[0-9][0-9]轮"` internal/**/*.go（非测试） | 零命中 | 实测 |
| 轮次标签扫描（测试） | 同正则 test 文件 | 2 处"第 3 轮"属测试内轮次轮询叙述（时钟失败计数/snapshot 轮数语义），非轮次前缀标签，合规 | 实测 |
| 工作区漂移 | `git status --short` + `git diff --stat HEAD` | 全空（零漂移，本报告提交前） | 实测 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十二轮闭合 —— ✅ 在位，零漂移

**sameClientFor 定义与 7 调用点逐一核实（scheduler.go，本轮行号与 R146 一致）**

- 定义 :204-210：`ClientFor(acct)` 存在性 + `clientIdentity(current) == clientIdentity(chainClient)` 反射指针比对（:215-224，`reflect.ValueOf(c).Pointer()`，nil 返回 0）。注释明确"需持 s.mu"。
- 7 调用点全部位于"网络往返后持锁写入前"最后一道身份闸，逐条追到终局写点：

| 调用点 | 分支 | 写什么状态/落什么库行 | 终局 |
|--------|------|----------------------|------|
| :850 | ProbeForAccount 回写段（M88-01 探测身份防线） | `openTimeDetected[acct]`（非空 beginTimes 才写）+ `acctData[acct]` + `acctDataAt[acct]`（nowAligned）。复核失败整体放弃回写返回 `(data,nil)` 不含过期数据 | 年级串线/过期快照一帧可见被拦 |
| :1489 | spawnChain 失效分支（ErrUnauthorized） | 复核失败：清 inflight + 静默 return，不写任何状态/日志/不触发 maybeRelogin（注释 :1494-1498 明确"重登必须落在身份复核之后"——防旧链把失败计数写进重建身份） | 重登决策落在身份闸之后 |
| :1521 | spawnChain 成功分支 | `done[acct][t.ClassID]=true` + `setStateLocked("success",msg)` + AppendLog + SaveSuccess（:1526-1540 全 `if err != nil { log.Printf }` 不吞错）。失败静默放弃 | 假成功/重启假状态被拦 |
| :1551 | 风控退避 isRateLimitError | `markRateLimitedLocked(30s)` + `setStateLocked("failed")` + AppendLog。失败静默放弃 | 假"退避中"吞黄金期被拦 |
| :1571 | 窗口关闭 isWindowClosedError | `markFullLocked`。失败静默放弃 | 假"已满员"永久退避被拦 |
| :1600 | 实时复核回锁入口（B43-02 第六分支） | 三路判据（cErr ErrUnauthorized→maybeRelogin / 确证满员→markFullLocked / 未现满员 failed）之前的统一身份闸 | 实时复核命中新身份 ErrUnauthorized 写进新身份被拦 |
| :1635 | 实时复核确证满员分支 | `markFullLocked`；与前置 `doneHas`（:1627 手动成功让位，绝不覆盖胜利状态）组合 | 陈旧链反向写 full 被拦 |

零漂移确认：grep 全仓库 `sameClientFor` 命中定义 1 + 调用 7 处（:850/:1489/:1521/:1551/:1571/:1600/:1635），无第四处（合同注释 :846 非调用）。测试侧两处引用（scheduler_test.go :3506/:3510）均为同名重建族测试的注释叙述。

**maybeRelogin 双侧（OBSERVE-117-01 知识位第三十轮）确认上位**

- 决策侧 :1208：`ClientFor(acct)` 存在性复核，已删账号不发起、不写任何 relogin 族 map（TestMaybeReloginDeletedAccountSkipsMaps 固化四 map 零 key 契约，实测 PASS）。
- 写回侧 :1254：重登完成先 `ClientFor(acct)` 复核，已删则整个成功分支（含内存写 + UpdateIDToken 落库）静默放弃、只清 relogging。
- :1265-1273 二次 `ClientFor(acct)` 重取**当前注册表** client → `client.Token()` 非空才 `store.UpdateIDToken` 落库。同名重建场景：前置 :1254 在同一把锁同一次持有内先拦（注册表指针已换即放弃整个成功分支），二次重取属于"重登成功者自身身份"的 token——旧链新身份组合不可能发生。知识位在位，三十轮无漂移。

**手动五路 accountExists（handler.go）**

- :255 课程读（`handleElectives`）
- :305 手动报名（`handleElectiveSelect`）
- :397 手动退选（`handleElectiveExit`）
- :497-512 目标写（`handleSetTargets`）——内联 `LoadCredentials` 逐账号比对（注释 :494-497 明确理由：仅 accounts 表会误伤 authenticateDirect 直连的已登录账号；判据与 accountExists :1109-1120 同源——凭据表真理源）
- :573 状态读（`handleState`）

五路全到位，目标写内联形态与 accountExists 语义等价。

**写点换类 5 类 + warnedNoTargets 宿主唯一性**

- `lastSubmit`：submitAll :1346 锁内 `nowAlignedLocked()` 写入；读侧 tick :1029-1030 锁内。读写同基（对齐钟）自洽。
- `lastSyncStart`：:356 maybeSyncClock 锁内写入；成功分支 :393 锁内回读。写读同持 s.mu。
- `syncing`：:355 锁内置位 / :369（goroutine 完成回调）、:404（无客户端不支持同步复位路径）锁内复位——复位路径全持锁，TestClockSyncNoClientResetsSyncing 固化。
- `lastProbe`：:679 Start 主循环锁内回零 / :964 ProbeNow 锁内 / :1089 probe 失败锁内 / :1114 probe 成功锁内。四写点全部持 s.mu。
- `state.EmptyProbeRuns`：:1156/:1158 probe 锁内入账/归零。
- `warnedNoTargets`（:1371-1372）——唯一无锁写点。宿主唯一性射证：submitAll 仅由 tick（:1035）单 goroutine 调用（Start :666-684 主循环 select），全仓库 grep 无第二处读/写；实际无并发竞争。延续 R146 维持观察。

**`*Locked` 写函数族 13 个 + 外部写函数首行取锁双向射证**

- `*Locked` 函数全量 grep 命中 13 个：nowAlignedLocked(:273，纯读)/openTimeForLocked(:427，读)/enrichTargetPubMetaLocked(:539，读)/rebuildCoursesForAccountLocked(:570，写)/rebuildCoursesLocked(:647，写)/tokenValidForLocked(:739，读)/windowClosedLocked(:918，读)/isRateLimitedLocked(:1691，读含过期删 key)/markRateLimitedLocked(:1710，写)/markFullLocked(:1760，写)/releaseFullIfFreedLocked(:1781，写)/statusIndexLocked(:1835，读)/setStateLocked(:1846，写)。写类 5 个全部只在锁内被调用（调用方逐点核过：SetTargetsForAccount/RestoreTargets 锁内 enrich；PurgeAccount 锁内无 *Locked 写；probe 锁内 windowClosedLocked/isRateLimitedLocked/markFullLocked/setStateLocked 全持 s.mu；MarkDone/RemoveDone 锁内 markFull 族）。
- 外部写函数首行取锁：SetTargetsForAccount(:455)/PurgeAccount(:496)/RestoreTargets(:526)/RestoreDone(:608)/RestoreRefused(:627)/TryAcquireSubmit(:1897)/MarkTokenValid(:1312-1315 reloginMu→s.mu 双锁，与 maybeRelogin 锁序一致)/MarkDone(:1923)/RemoveDone(:1990)/RemoveFull(:2039)。MarkTokenValid 双锁序与 maybeRelogin/TokenValidFor 对齐（R146 已证，本轮复验在位）。
- 双向射证闭环：无锁外调用 *Locked 写函数路径；无外部写函数漏首行取锁。

### 2. OBSERVE-117-01 知识位 —— ✅ 在位（第三十轮，见上 maybeRelogin 双侧明细）

### 3. B110-01 审计链第三十七轮 —— ✅ 零漂移

- **手动 6 失败位 AppendLog 逐一在位**：:362（报名 token 失效）、:374（报名 read 类）、:381（报名其余业务失败）、:443（退选 token 失效）、:452（退选 read 类）、:459（退选其余业务失败）。每处 `if err != nil { log.Printf }` 不吞错。
- **成功审计行**：手动报名成功经 MarkDone 内部 :1976 AppendLog；手动退选成功经 RemoveDone 内部 :2029；登录成功 :237 / 管理员登录 :136 / 登出 :616 / 设目标 :558 / 删账号 :1055 / 配置更新 :864 全在位。
- **自动链失败族**：spawnChain 全分支（失效/成功/风控/窗口关闭/实时复核失效/其余失败）全部 AppendLog + 落库失败 log.Printf（:1504/:1532/:1558/:1570族/:1614/:1662）。
- **零吞错穷举**：产品代码 `_ =`/`_, _ =` 命中仅两处——handler.go :387 `_ = d.Sched.MarkDone(...)` 与 :467 `_ = d.Sched.RemoveDone(...)`。核证两函数恒返回 nil（内部落库失败已 log，账号已删静默返回 nil），忽略返回值无害。其余全部 `_ =` 在测试文件或无害路径（scheduler.go :307 prewarm goroutine、:1075 probe goroutine、client.go :107/:124 io.Copy drain、config 模板写、cmd/*）。**零吞错契约成立，第三十七轮延续**。
- **网络层 token 脱敏延续**：zhidao/client.go :450 doRequest 连接失败统一 `sanitizeError`（:581-593，剥 *url.Error 完整 URL、Unwrap 保留判型链）；scheduler.go :1283 重登成功日志 `maskedToken(newTok)`（:1334-1339，长度 ≤8 输出 `***`）。sanitize_test.go 判型穿透族（dial/write/read-rst/fin/shortread/timeout/business/nil 八形态）实测全 PASS。

### 4. O105-01 抖动基线 —— ✅ 在位且实测绿

- socketPreheat 夹具：zhidao/client_test.go :25-30（预创建-关闭 127.0.0.1 套接字）+ :40 包级调用 + captcha_test/sanitize_test 逐调用；api handler_test.go newTestDeps :67-71 套接字预创建。
- readyProbe 夹具：zhidao/client_test.go :88-113（200ms×10 + 2s 显式超时宽栅栏）+ :78/:164 逐测试调用；api/handler_test.go :179-203 同款 + :139 就绪探测；accounts/manager_test.go :26-40 readyProbe（无 socketPreheat 双保险，注释明确"8 轮全绿实证无残余，若未来再出冷启动 flake 第一候选即补"——延续观察）。
- 定向 `-race` 四包（zhidao/accounts/scheduler/api）实测全绿 + 全仓库 `CGO_ENABLED=1 go test -race ./...` 13 包全绿（store 24.791s 最重，DB 全量）。双回归锚实测 PASS。

### 5. LOW-132/133 回首核 —— ✅ 通过

`time.Since`/`time.Now()` 全量扫描（产品代码）分类裁决：
- scheduler.go :269/:274 = `nowAligned`/`nowAlignedLocked` 内部实现（对齐钟基准本身），合规。
- scheduler.go reloginAt 族：:1220/:1227 读 `time.Since(t)`（reloginAt 存值），:1231/:1261 写 `time.Now()`——同一本地基写读自洽（R146 已证同款）。
- accounts/manager.go gateWindow 族：:53/:74/:227 写 `time.Now()` + :71/:226 读 `time.Since(gateWindow)`——同一本地基写读自洽（B42-01 闸门族内部）。
- handler.go :1176 loginLimiter tokenBucket 纯本地自洽。
- session/store.go :74/:106/:156/:168/:183/:198 会话/票据 TTL 纯本地自洽（与调度器无交互）。
- zhidao/client.go :113-137 SyncServerTime RTT 计时/clockOffset 计算：`time.Since(start)` 差值与 `time.Now()` 的绝对值——时钟对齐算法自身基准，写完即被调度器用 `nowAligned` 同基消费，合规。
- :328/:355 captcha v 时间戳 / uniqueId 设备指纹 = 平台契约要求的"当前毫秒/36 进制时间戳"，非节流判读，合规。
- 时钟族写入侧（lastSyncFailAt/syncFailedWindow）已收敛到对齐钟（:372/:377 nowAlignedLocked，读侧 :342 传对齐 now）——LOW-132 混用孤岛消亡，延续零残留。

**裁决：残余时间基（reloginAt/gateWindow/limiter/session TTL/SyncServerTime）均为写读同基自洽或平台契约需求，合规。**

### 6. 新契约角度纵深（自选 ×2，选因说明）

**（a）窗口状态三判据单源 open 快照（windowClosedLocked + probe 入账）**

为什么选：R146 纵深选了两条身份防线主战场，本轮补位调度器的"窗口关闭判定"——这是提交挂起/探测降频的命门，且三判据单源是最近轮次反复强调的"热改亚毫秒读取一致"契约落点。

实测核证（scheduler.go :918-939 + :1145-1159）：
- 三判据单源：`windowClosedLocked()` 先查主判据 `s.state.WindowClosed`（:919），再统一取 `open := s.openTimeForLocked("")` 单快照（:924）供时钟失败判据（:925 `syncFailStreak>=3 && !open.IsZero() && nowAlignedLocked().After(open)`）与幽灵窗口判据（:935 `!open.IsZero() && !state.WindowOpened && EmptyProbeRuns>=3`）复用——一次读取，三判据共享，杜绝"识别值在两次读取间被新批次覆盖"的不一致窗口。
- `StateForAccount`（:703）与 `WindowClosed()`（:910）共用同一 `windowClosedLocked`，两套真相分叉已根除（R143 起契约，本轮复验在位）。
- probe 入账侧（:1145-1158）：`open` 也是单次快照（:1145 `s.openTimeForLocked("")`），主判据（:1146 `prevOpened && !opened && len(Publishes)==0 && now.After(open.Add(10s))`）与 EmptyProbeRuns 入账（:1155 `!opened && len==0 && now.After(open.Add(10s))`）两处 10s 裕量用同一 open——同一探测内两处裕量判定基于同一个识别值。
- 回归实测：TestWindowClosedState（-time.Hour 主判据场景）PASS；定向 -race 下 windowClosedLocked 读路径（StateForAccount/WindowClosed/RecognizedOpenTime 并发）无竞争。
- 边界自洽：关闭≠时间消失——probe 空快照只覆盖非空 beginTimes、绝不 delete 识别槽（:855-858 仅在 `len(data.BeginTimes)>0` 时写），windowClosedLocked 判据①的"已过开窗点 10s 裕量"与入账侧对称（R144 契约延续）。

**（b）连接活性自愈族 httpDo 判型（isConnErrRetryable vs IsReadErr 互斥边界）**

为什么选：这是"重试语义正确性"的最后一道防线——误重试 POST（SelectClass/ExitClass）会双报，误不重试会炸黄金期。判型边界必须实测而非走读。

实测核证（zhidao/client.go :478-552 + sanitize_test.go）：
- `httpDo`：首 `c.Do` 仅当 `isConnErrRetryable`（dial/write 且 errors.As 穿透 url.Error 链）才 `cloneReq` 重发一次（:484-491）——read/业务/取消错误原样上抛。
- 互斥边界实测：`isConnErrRetryable` 只认 `*net.OpError.Op=="dial"|"write"`（:500-504）；`IsReadErr` 覆盖 FIN(io.EOF)/短读(ErrUnexpectedEOF)/两种超时文案/read-Op 四形态（:523-552）。两函数正交——retryable 含 dial/write、read 恒 false，注释明示互斥。TestIsReadErrCoversAllForms + TestSanitizeErrorPreservesJudgment（八形态：dial/write/read-rst/fin/shortread/timeout/business/nil）实测全 PASS，且判型穿透 `sanitizerErr.Unwrap` 链不被脱敏破坏。
- 双报防线的第二道：`cloneReq` 用 `req.Clone(ctx)`（:559-561）——GetBody 由标准库对 bytes.Reader 自动设置（注释 :556-557），keep-alive 复用连接静默关闭时的 nothingWritten 场景由 transport 内部用 GetBody 重放完整 body，不双报。
- 边界盲区确认：read 错误不重试 → 平台可能已处理，调用方收到 IsReadErr 后记"可能已处理"文案（handler :374/:452、spawnChain :1656-1658 同款）——语义链完整。

## 维持观察项（LOW）

1. **handler.go :387/:467 两处 `_ =`** 忽略 MarkDone/RemoveDone 返回 error——实测两函数恒返回 nil（内部落库失败已在函数内 log，账号已删静默返回 nil），语义无害；未来若演化出调用方需感知的错误类型应顺手改 `if err != nil { log.Printf }`（延续 R146 观察，不阻塞归档）。
2. **scheduler.go:1371-1372 `warnedNoTargets` 无锁写**——宿主唯一性已证（submitAll 仅由 tick 主循环单 goroutine 调用），全仓库 grep 无第二处读/写；延续 R146 观察，未来若引入并发调用 submitAll 的路径须先收口。
3. **accounts/manager_test.go 无 socketPreheat 双保险**（只有 readyProbe）——注释明确"8 轮全绿实证无残余"；延续观察，若未来该包再出冷启动 flake 第一候选即补 socketPreheat。

## 结尾建议

**APPROVE**。身份防线矩阵第六十二轮闭合：sameClientFor 7 调用点零漂移且每条追到终局写点；maybeRelogin 双侧 + 二次 ClientFor 重取当前 token 落库（OBSERVE-117-01 知识位第三十轮）在位；手动五路 accountExists 与目标写内联判据同源；写点换类 5 类 + warnedNoTargets 唯一无锁点全收口；B110-01 审计链第三十七轮零吞错 + 脱敏延续零漂移；O105-01 四包定向 -race + 全仓库 CGO=1 -race 13 包全绿 + 双回归锚实测通过；LOW-132/133 回首核通过（时间基残余全为写读同基自洽或平台契约需求）。新契约纵深（窗口三判据单源 open 快照、httpDo 连接自愈判型互斥边界）均实证在位。工作树除本报告外零改动。
