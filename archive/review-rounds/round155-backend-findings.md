# R155 后端只读审查 Findings —— 身份防线矩阵第七十轮闭合（里程碑轮全家福）

> 审查基线：`f1d4b37`（R154 归档，身份防线矩阵第六十九轮闭合）。模式：绝对只读，唯一写文件为本报告。
> 时间：2026-09-24。实测工具：`go build` / `go vet` / `go test -race -count=1`（MinGW gcc，`export PATH=/d/mingw64/bin:$PATH`）+ 走读追写终局。

## 结论前置（分级）

**CRITICAL 0 / HIGH 0 / MEDIUM 0 / LOW 0（无新增缺陷）**

身份防线矩阵第七十轮（里程碑轮）闭合成立、零漂移。sameClientFor 定义（scheduler.go:204）与 clientIdentity（:215）逐字符确认，7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一追到"写什么状态/落什么库行"的终局——六分支 + 探测回写全族闭合，网络往返后持锁写入前均为最后一道身份闸，无裸露写点。maybeRelogin 双侧完整（决策侧 :1208 存在性复核 / 写回侧 :1254 复核 + :1265-1273 二次 ClientFor 重取当前注册表 Token 落库）。手动五路 accountExists（handler.go :255/:305/:397/:497-512/:573）判据同源。写点换类 5 类全持锁、无锁写点 warnedNoTargets（:1371-1372）宿主唯一性射证。OBSERVE-117-01 知识位第三十八轮、B110-01 审计链第四十五轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（探测定时族三件套 / 登录闸门族 B42-01 双侧收口）无漂移。

---

## 验证表(实测时间与数据)

| 验证项 | 结果 | 实测证据/数据 |
|--------|------|---------------|
| `go build ./...` | ✅ | EXIT=0，无输出 |
| `go vet ./...` | ✅ | EXIT=0，无输出 |
| 定向 race 四包全量（强制非缓存） | ✅ 全绿 | zhidao 41.584s / accounts 12.715s / scheduler 15.094s / api 30.585s，全 `ok` EXIT=0 |
| 身份防线族（scheduler 定向） | ✅ 全绿 | TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}（各 0.01s）+ TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime（0.15s）+ TestMaybeReloginDeletedAccountSkipsMaps（0.00s）+ TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin（0.51s）+ TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight（0.01s），包总 4.438s |
| api accountExists 五路族 | ✅ 全绿 | TestAdminElectivesUnknownAccountRejects（0.10s）/ TestAdminElectiveSelectUnknownAccountRejects（0.10s）/ TestAdminStateUnknownAccountRejects（0.09s）/ TestSetTargetsUnknownAccountDoesNotFabricate（0.08s），包总 2.346s |
| 回归锚双测复跑 | ✅ 全绿 | TestWindowOpenSubmitsWithoutProbeReset + TestSubmitSuspendedWhenOpenTimeCleared（scheduler 4.064s）+ TestAdminStatsWindowOpenedUsesScheduler（api 2.023s） |
| 时钟/窗口判据族 | ✅ 全绿 | TestClockSyncFailureBackoffUsesAlignedClock / TestClockSyncNoRetryWithinBackoff / TestClockSyncFailureResetsOffset / TestClockSyncNoClientResetsSyncing / TestClockSyncSuccessClearsFailStreak / TestWindowClosedState / TestGhostWindowEmptyProbesSuspend / TestGhostWindowClockFailuresSuspend（scheduler 1.812s） |
| 手动协同族 | ✅ 全绿 | TestManualReselectClearsRefusedRow / TestManualDoneClearsInflight / TestManualSnapshotFallbackOnlyWhenOwnFresh（1.399s）+ TestDeletedAccountManualInFlightDropsState / TestSchedulerManualSyncAndSubmitMutex / TestRealtimeFullRecheckKeepsManualSuccess / TestRealtimeFullRecheckWithNoManualDoneMarksFull（1.317s）+ TestRefusedNeverResubmitted / TestRefusedRestartOrderRealDB / TestRefusedPersistedAcrossRestart（7.526s） |
| 登录/鉴权/状态码族 | ✅ 全绿 | TestLoginUnactivatedNeedsCode / TestActivateRequiresTicketAndBinding / TestLoginActivationDisabledSkipsCheck / TestLoginRateLimit / TestLoginLimiterGC / TestLoginActivateSeparateBuckets / TestLoginAdminWrongPasswordTimingFlat / TestLoginAdminNameCollisionStudentCredential / TestLoginRejectsFormContentType（api 3.206s）+ TestAuthRequired / TestAccountOverrideRequiresAdminSession / TestAdminAuth / TestRecoverMiddlewareHidesPanicDetail / TestAdminDeleteProtectsRenamedAdmin（2.583s） |
| 删账号 memory-first 族 | ✅ 全绿 | TestAdminDeleteAccountMemoryFirst（0.11s）/ TestAdminDeleteAccountNoBodyOK（0.43s，api 2.063s）+ TestRevokeAccount（session 1.288s） |
| 登录闸门族 B42-01 | ✅ 全绿 | TestLoginByPasswordRejectsWhenGateBudgetExhausted / TestLoginByPasswordAllowedWhenGateBudgetAvailable / TestLoginByPasswordEncryptFailLogs（accounts 1.351s）+ TestLoginFailKeepsExistingValidClient / TestLoginFailRemovesFreshShell / TestNewClientAfterSetRecognizerGetsEngine（1.401s） |
| 脱敏判型族 | ✅ 全绿 | TestSanitizeErrorPreservesJudgment 七形态（dial/write/read-rst/fin/shortread/timeout/business/nil 子测）+ TestSanitizeErrorNilSafe / TestSanitizeErrorOriginalErrorPreserved / TestSanitizeErrorNeverEmptyError / TestIsReadErrCoversAllForms（zhidao 1.592s） |
| 重登退避族 | ✅ 全绿 | TestReloginBackoffCappedAndReset / TestReloginBackoffWindowBlocksManualTriggers（scheduler 1.463s） |
| 数据迁移/加密链族 | ✅ 全绿 | TestMigrateAddsPublishMetaColumns（0.11s）/ TestRefuseLegacyDB（0.09s，db 1.576s）+ TestEncryptDecryptRoundTrip / TestLoadOrCreateKey / TestLoadOrCreateKeyRejectsTruncatedFile（secure 1.294s）+ store 全量 13.998s |
| 轮次标签扫描 | ✅ 零命中 | 产品代码零命中；`internal/session/store.go:117` 仅引用历史文档路径 `docs/review-round13.md`（合规）；`scheduler_test.go:1659/:1662/:2518` 三处"第 3 轮"行为叙述（测试叙述合规） |
| 零吞错穷举 | ✅ 零命中 | 四处 `_ =`/`_, _ =`（scheduler.go:307 Prewarm 异步 goroutine / :1075 ProbeForAccount 冗余 goroutine、handler.go:387 MarkDone / :467 RemoveDone）逐一确认全部非落库 |
| 工作区零漂移 | ✅ | `git diff f1d4b37 -- backend/` 0 行；未跟踪仅 `?? archive/review-rounds/round155-frontend-findings.md`（并行前端代理产物） |

---

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第七十轮闭合（里程碑全家福）—— ✅ 在位，零漂移

**sameClientFor / clientIdentity 定义逐字符核对**

- 定义 scheduler.go:204-210：锁内 `ClientFor(acct)` 存在 + nil 判 + `clientIdentity(current) == clientIdentity(chainClient)`；clientIdentity :215-224 用 `reflect.ValueOf(x).Pointer()` 取接口动态值底层指针，nil→0、非 Ptr 动态类型→0。Client 接口依赖含 `Token() string`（:118-135，重登后读取新 token 落库）。与契约一致。

**7 调用点坐标与写终局逐一追写（R154 坐标逐字符比对，零位移）**

| 调用点 | 位置 | 身份复核通过才执行的写入 | 复核失败时放弃的面 |
|--------|------|--------------------------|--------------------|
| 探测回写（M88-01 同族） | :850（ProbeForAccount 回写段） | `acctData[acct]`/`acctDataAt[acct]`（:863-864）+ `openTimeDetected[acct]` 识别槽（:856） | 新/旧年级帧不串写重建身份；识别槽不被陈旧链覆盖（关闭≠时间消失不受损） |
| 失效分支 | :1489 | `maybeRelogin` 触发（:1499）+ failed 状态 + AppendLog 失效日志（:1502-1507） | 不消耗新身份登录预算；Manager 失败计数 reloginFail 不被新身份污染 |
| 成功分支 | :1521 | `done[acct][id]=true` + setStateLocked success + AppendLog(success) + SaveSuccess 库行（:1526-1540） | 重启 RestoreDone 无假成功 |
| 风控退避 | :1551 | markRateLimitedLocked 30s 退避 + failed 状态 + AppendLog（:1555-1561） | 黄金期不被假退避静默跳过 |
| 窗口关闭 | :1571 | markFullLocked 永久满员退避 + 状态 + AppendLog（:1575） | 重建身份不落永久 full |
| 实时复核入口 | :1600 | 拦截下三路（ErrUnauthorized→maybeRelogin/失败写库 :1608-1619；确证满员→markFullLocked :1639；普通失败→failed+AppendLog :1660-1665） | 实时复核结果块（删号竞态最后一块裸露写点）整面闭合 |
| 实时复核确证满员 | :1635 | doneHas 让位（:1627 绝不覆盖手动胜利状态）之后的 markFullLocked（:1639） | 不覆盖手动报名成功 |

六分支 + 探测回写全族叠合，加上链顶 ClientFor（spawnChain :1388）、submitAll 遍历判 ClientFor（:1355）、goroutine 取 client 二次判（:1405-1409，含 nil 判防 panic）、chainClient 指针捕获（:1417）、inflight 双门口（:1464 判 /:1471 置）、doneHas/refusedHas/fullHas 门（:1436/:1447/:1452）、relogging 短路（:1421）、tokenValidForLocked 短路（:1431）——整链网络往返后持锁写入前无裸露写点。chains 活跃标记（:190 声明 / :1393 读 / :1397 置 / :1401-1403 defer 清）锁内管理，测试 waitChainExit（scheduler_test.go:3184）等待契约规避 inflight nil-map 假绿。

**maybeRelogin 双侧（:1198-1299）**

- 决策侧 :1208：`reloginMu.Lock()`+`s.mu.Lock()`（:1199/:1201）后先 `ClientFor(acct)` 存在性复核，!ok 整体 return——绝不写 tokenValid/reloginFail/reloginAt/relogging 任何 map（TestMaybeReloginDeletedAccountSkipsMaps 四 map 全空实证，0.00s 绿）。
- 写回侧 :1254：goroutine 内成功分支前先 ClientFor 复核 → :1259 `err==nil && relogged` → :1265 **二次 ClientFor 重取当前注册表 client** → :1266 `client.Token()` → :1269 UpdateIDToken 落库。同名重建场景：两次复核间注册表被顶替时，第二次重取即拿到新身份客户端，落库写入方恒为"当前最新注册的客户端身份"（OBSERVE-117-01）。失败侧 :1292-1297 只清 relogging、reloginFail 保留递增计数（指数退避表不振荡）。
- 重登族 map 写点全量盘点：tokenValid/reloginFail/reloginAt/relogging 写入仅在 maybeRelogin 决策段（:1231/:1236/:1241）、成功分支（:1260/:1261/:1262）、清理在 PurgeAccount（:507-510）、MarkTokenValid（:1316-1318）、失败清 relogging（:1247）、退避窗口过 delete reloginAt（:1226）——全部持 reloginMu+s.mu，零锁外写。

**手动五路 accountExists（判据同源）**

课程读 :255、手动报名 :305、手动退选 :397、状态读 :573、目标写 :497-512（内联 LoadCredentials 循环，注释明示"仅用 accounts 表会误伤 authenticateDirect 直连"）——五路全部凭据表判据；定义 :1109-1120（LoadCredentials 失败返回 false），穿透门 :1103 仅管理员会话。

**写点换类 5 类 + 无锁写点唯一性**

- lastSubmit：submitAll :1346 锁内 `s.nowAlignedLocked()` 写；tick :1029 锁内读。
- lastSyncStart：:356 锁内置位、:393 锁内读（成功推进 lastSyncTime）、:405 锁内复位。
- syncing：:355 锁内置位、:369 goroutine 锁内回写复位、:404 无客户端路径锁内复位。
- lastProbe：probe :1089（失败计入）/ :1114（成功计入）、ProbeNow :964（管理员强制刷新）、Start 主循环 :679（重登成功回传统一处清零补探测，注释明确"避免 goroutine 并发写 s.lastProbe 竞态"）——全部锁内写，tick :976 锁内读。ProbeForAccount 刻意不写（:868-872 注释：只归 probe/ProbeNow 管理，防止管理员穿透探测旁路全校节流）。
- state.EmptyProbeRuns：probe :1156（+1）/ :1158（归零）锁内；windowClosedLocked :935 锁内读。
- 无锁写点 warnedNoTargets :1371-1372：唯一宿主 = submitAll（判定在 :1368 `len(chains)==0` 之后、:1375 return 之前）；调用方唯一（tick :1035）；grep 全仓库仅此一处读写（scheduler.go:176 声明 + :1371/:1372 使用），单写者单读者连续执行，zero-value false 无目标必走到——语义自洽非漂移，维持观察。

***Locked 写函数族 13 个 + 外部写函数首行取锁双向射证**

- 13 个：nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846。逐一追调用上下文全在锁段内（spawnChain 锁段 / tick 锁段 / StateForAccount / WindowClosed / probe / purge / restore / TryAcquireSubmit / MarkDone / RemoveDone / RemoveFull），无锁外裸调。
- 外部写函数首行取锁核查：SetTargetsForAccount :455、PurgeAccount :496、RestoreTargets :526、RestoreDone :608、RestoreRefused :627、ProbeForAccount 回写段 :849（锁 + sameClientFor 前置复核锁内）、ProbeNow :963、ElectivesSnapshotFor :762、TryAcquireSubmit :1897、MarkDone :1923、RemoveDone :1990、RemoveFull :2039——全部首行取锁，双向射证成立。

### 2. OBSERVE-117-01 知识位第三十八轮 —— ✅ 在位

见第 1 节 maybeRelogin 写回侧。关键证据链：goroutine 内 `s.mu.Lock()`（:1246）→ 清 relogging（:1247）→ `ClientFor(acct)` 存在性复核（:1254，已删整体放弃只清标记）→ 成功分支（:1259）→ **二次 `ClientFor(acct)`**（:1265）→ `client.Token()`（:1266）→ `UpdateIDToken` 落库（:1269）。同名重建场景落库写入方恒为当前最新注册客户端身份——与 R154 逐字符一致，零漂移。TestReloginSuccessWithNilStoreNoPanic、TestDeletedAccountReloginSuccessDropsState、TestDeletedAccountRebuiltSameNameChainDropsRelogin 全绿。

### 3. B110-01 审计链第四十五轮 —— ✅ 零漂移

- 手动 6 失败位 AppendLog：handler.go :362（报名失效）/ :374（报名 read）/ :381（报名业务失败）/ :443（退选失效）/ :452（退选 read）/ :459（退选业务失败）——全部 `if ... != nil { log.Printf }` 零吞错。
- 成功审计行：报名成功走 MarkDone 内 AppendLog（scheduler.go:1976）、退选成功走 RemoveDone 内 AppendLog（:2029）、目标保存 :558、登录 :237、管理员登录 :136、删除账号 :1055、登出 :616——全部带错误处理。
- 自动链失败族：spawnChain 六分支 AppendLog 全部 `if err := ...; err != nil { log.Printf }`（:1504/:1532/:1558/:1614/:1662/:1770）+ markFullLocked（:1770）+ MarkDone 内 SaveSuccess/AppendLog（:1973/:1976）+ RemoveDone 内 DeleteSuccess/SaveRefused/AppendLog（:2020/:2026/:2029）+ SetTargetsForAccount 内 SetTargetsForAccount/DeleteRefused（:476/:484）+ 重登成功 UpdateIDToken（:1269）。
- 零吞错穷举：`_ =`/`_, _ =`/直接赋值忽略形态全量扫描——scheduler.go `_ =`（:307 Prewarm 异步 goroutine 丢返回值）、`_, _ =`（:1075 ProbeForAccount 探测 goroutine 丢返回值）、handler.go `_ =`（:387 MarkDone / :467 RemoveDone）——四处全部**非落库**：:307 预热结果不上抛、:1075 探测错误已在 ProbeForAccount 内部处理、:387/:467 手动成功路径业务错误全部前置 return。所有 `store.Save*/Delete*/AppendLog/UpdateIDToken/SetTargetsForAccount` 落库点 grep 穷举零"忽略错误"形态。
- 网络层 token 脱敏延续抽查：zhidao/client.go doRequest 统一 `sanitizeError`（:450 剥 url.Error URL 文本、:563-593 定义与 Unwrap 下沉保留 errors.As/Is 判型、`ue.Op==""` 防御），scheduler.go:1283 重登成功日志走 maskedToken（:1334，>8 位仅前 8 位）。仍沿用既定判型穿透契约。

### 4. O105-01 抖动基线 —— ✅ 实测绿

- socketPreheat/readyProbe 夹具在位：zhidao/client_test.go:25/:88（socketPreheat 注释自认"预创建并关闭一个 127.0.0.1 回环套接字排空冷启动窗口"）、captcha_test.go:17/:52（socketPreheat）+ :29/:79（readyProbe）、sanitize_test.go:97、api/handler_test.go:139/:179-185、accounts/manager_test.go:26（readyProbe，注释自认无 socketPreheat 双保险、多轮全绿实证无残余，延续观察项）。
- 定向 race 实测（`/d/mingw64/bin` gcc 路径下）：zhidao/accounts/scheduler/api 四包全量 `-race -count=1` 41.584s/12.715s/15.094s/30.585s 全绿；身份防线族十测 + 回归锚双测 + 时钟族 + 手动协同族定向全绿（见验证表）。
- 回归锚 TestWindowOpenSubmitsWithoutProbeReset（探测量变后黄金期外首次 tick 立即提交不被节流吞）与 TestAdminStatsWindowOpenedUsesScheduler（/api/admin/stats 与学生端 /state 同源 windowClosedLocked）双绿复跑确认。

### 5. LOW-132 / LOW-133 回首核 —— ✅ 通过

- `time.Since()/time.Now()` 产品代码全量扫：残余仅两族——① `reloginAt` 写（:1231/:1261 本地钟）读（:1220/:1227 `time.Since(t)` 本地钟）**同基自洽**（30s 重登节流语义为同流程内部时间差，不依赖对齐钟）；② accounts/manager.go `gateWindow`（gateWait :53/:54、gateTryAcquire :226/:227、GatePump :71/:74）本地钟写读**同基自洽**（分钟窗口闸门语义为本地时间差）。其余非节流语义独立点（zhidao SyncServerTime RTT 测量 :113/:135/:137、captcha ?v 随机种子 :328、uniqueDeviceID 时间戳 :355、session 会话/票据 TTL、api loginLimiter handler.go:1176）均为各自子系统内自洽。探测族时间戳全部对齐钟写、对齐钟读，零新增时间基孤岛。
- `git diff f1d4b37 -- backend/` 0 行，白线零漂移。

### 6. 新契约角度纵深（自选 ×2，时间盒内完成）

**角度 A：探测定时族三件套（选择理由：前三轮纵深侧重提交/身份/加密链，探测定时整族——全校 30s 节流闸门 / probing 单飞 / probeSem cap4 三件套与"管理员穿透探测不旁路全校节流"的交叉——是里程碑轮最后一块未系统覆盖的面）**

- **全校 30s 节流闸门 lastProbe**：只归 probe()（:1089 失败计入 / :1114 成功计入）与 ProbeNow（:964 管理员强制刷新）写入；tick :987 判 `last.IsZero() || now.Sub(last) >= probeIntervalForOpen(now, open)`。**关键契约**：ProbeForAccount :868-872 刻意不写 lastProbe——若账号级穿透探测写它，开窗前管理员手动点一次课程页就按下一次正规探测（30s→2s 临门盯守被吞），黄金期提交失去即时确认。
- **probing 单飞守卫**：probe() :1049-1055 持 s.mu 置位保证"置位-检查"原子（零值守卫同款窗口），命中单飞放弃本次（最长推迟一个 tick 300ms）；:1081/:1088/:1113 三处锁内复位。HTTP 侧 ProbeForAccount/ProbeNow 刻意不套单飞（管理员刷新要强制最新 + 频率已受节流约束）。
- **probeSem cap 4 信号量**：:1073-1074 per-account 探测并发封顶 N→4，跨批受同一信号量约束不叠加；`ponytail:` 注释明示"cap=4 常驻，若平台放宽熔断或账号数>50 再调"。
- **节奏纯函数**：probeIntervalForOpen :87-103（零值/识别过期→远间隔、临门 5 分钟→2s、已过开窗点+WindowClosed→降回 30s）；tick :989 追加"窗口到点后首次 tick 立即探测"覆盖（不等待节流闸门放过）。回归锚 TestWindowOpenSubmitsWithoutProbeReset 绿锁定量变不吞黄金期。
- **判据单源交叉**：scenario 判定 open 单快照复用（:987 与 probe :1145 同策略）杜绝热改亚毫秒不一致；EmptyProbeRuns 入账与 windowClosedLocked 判据 3 共享 open。

**角度 B：登录闸门族 B42-01 双侧收口（选择理由：R150 纵深过删号四序互操作，但门内"配额共享唯一性 + 消费方全量清单"未系统核对——闸门泄口（任何一条补记）会直接放大出口 IP 锁号风险）**

- **唯一计数共享**：gateWait（manager.go:49-64）与 gateTryAcquire（:223-235）共用 `gateMu`（:40）与 `gateUsed`（:42），同一分钟窗口内排队重登与手动登录严格共享全账号 doLogin 预算（gateLoginPerMin=2，:161）。
- **双侧消费方全量清单**：阻塞侧仅 `Manager.Relogin`（:166-175，自动重登）；非阻塞侧 `LoginByPassword`（:244 入口 `!m.gateTryAcquire()` 即拒，绝不阻塞排队——排队会挂起用户登录响应数分钟）；管理员换绑入口经 handleLogin :142 → LoginByPassword 同收口。无绕行旁路。
- **无死锁论证**：gateTryAcquire 先 Lock gateMu 再检查/消耗，与 gateWait 同一锁同一 path，无嵌套等待；ResetGateForTest（:82-87）测试专用，正式代码不调用。
- **测试锚定**：TestLoginByPasswordRejectsWhenGateBudgetExhausted / TestLoginByPasswordAllowedWhenGateBudgetAvailable 双绿（闸门耗尽拒绝 vs 配额充足放行）、api 夹具批量注册撞闸门用 ResetGateForTest 不误伤。

---

## 维持观察项（延续既有记录 + 本轮）

1. `IsClassFull` 实时复核受 CountEntry.MaxCount 未实证下发限制——恒 false 兜底路径，真满员主判据为快照 max_count（classFullInSnapshot），属既有契约非缺陷。
2. `RemoveFull`（:2038）目前仅定义无产品调用方（预留接口），未来消费前保持观察。
3. `syncFailedWindow`（:182/:377/:394）写而不读，判据用 syncFailStreak——留档字段语义自洽，注释明示。
4. 验证码识别引擎 ddddocr 本地推理在 CGO=0 交叉编译形态依赖 native_ocr_stub 回退，属既有双轨契约。
5. accounts 包测试夹具无 socketPreheat 双保险（manager_test.go:24 注释自认），多轮全绿实证无残余——若未来再出冷启动 flake 第一候选即补 socketPreheat。
6. 管理员 `DELETE /api/admin/accounts` 与 `DELETE /api/admin/codes`（router.go:153/:137）为兼容标准 REST 客户端刻意去掉 requireJSONBody 门——副产品：管理员 token 泄露场景下跨站表单无法伪造这些 DELETE（无 body 的表单 POST 不匹配 DELETE 方法），CSRF 风险面仍闭合。维持观察。
7. **warnedNoTargets 无锁写点（:1371-1372）**：位于 s.mu.Unlock() 之后、submitAll 的 `len(chains)==0` 分支内，宿主唯一（全文件仅此一处读写 grep 实证）、单写者单读者连续执行，语义自洽非漂移。首例并发写 Race 报告出现前维持观察。
8. `probeSem` cap=4 常驻（`ponytail:` 注释标记），平台熔断收紧或账号数突破 50 时需重估——当前无触发信号。

---

## 结尾建议

**APPROVE**

身份防线矩阵第七十轮（里程碑轮）闭合成立：sameClientFor 7 调用点终局逐一追写零漂移（六分支 + 探测回写全族对称，实时复核结果块为最后一块裸露写点已闭合），maybeRelogin 双侧完整（决策侧 :1208 存在性复核 + 写回侧 :1254 复核 + :1265-1273 二次 ClientFor 重取当前注册表 Token 落库）、手动五路 accountExists（:255/:305/:397/:497-512/:573）、写点换类 5 类 + warnedNoTargets 唯一性、*Locked 写函数族 13 个双向射证全部实测确认；OBSERVE-117-01 知识位第三十八轮、B110-01 审计链第四十五轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（探测定时族三件套 / 登录闸门族 B42-01 双侧收口）无漂移。CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。