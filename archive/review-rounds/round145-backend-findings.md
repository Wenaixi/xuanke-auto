# R145 后端只读审查 findings（身份防线矩阵里程碑轮）

> 审查代理：r145-backend 只读代理。项目根 `E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`，工作树基线 = 4aecde0（R144 归档）。本轮全仓库零修改（仅本报告写入 archive/）。
> 实测环境：Windows 11，`export PATH=/d/mingw64/bin:$PATH` 启用 gcc 后 `go test -race`。

## 结论前置

- **CRITICAL：0**
- **HIGH：0**
- **MEDIUM：0**
- **LOW：0（无新增）**

身份防线矩阵里程碑轮闭合（同族防线从"追加分支修洞"演进为"按 R135 全员清点矩阵复核全部现存调用点"，六分支 + 探测回写 + 实时复核三路全为网络往返后持锁写入前最后一道身份闸，无裸露写点）；OBSERVE-117-01 知识位确认在位（同名前重建不串旧身份）；B110-01 审计链零漂移（零吞错穷举零命中）；O105-01 实测绿（定向四包 race + 回归锚 count=3 复跑）；LOW-132/133 回首核通过（白线核对 + 时间基全量扫零残留）。建议 **APPROVE**。

## 验证表（实测数据，2026-09-24）

| 项 | 命令 | 结果 | 实测耗时 |
|---|---|---|---|
| 编译 | `cd backend && go build ./...` | exit 0 | - |
| 静态检查 | `go vet ./...` | exit 0 | - |
| race zhidao | `go test -race -count=1 ./internal/zhidao/` | ok | 12.695s |
| race accounts | `go test -race -count=1 ./internal/accounts/` | ok | 1.332s |
| race scheduler | `go test -race -count=1 ./internal/scheduler/` | ok | 14.992s |
| race api | `go test -race -count=1 ./internal/api/` | ok | 18.959s |
| 其余后端包 | `go test -race -count=1 ./internal/store/ ./internal/db/ ./internal/session/ ./internal/config/ ./internal/runtime/ ./internal/secure/` | 六包全 ok | 见 R144 同量级 |
| 身份防线族十三测 | `go test -race -run "TestDeletedAccountRebuiltSameName\|TestMaybeReloginDeletedAccount\|TestRealtimeRecheck\|TestDeletedAccountInFlight\|TestDeletedAccountManualInFlight\|TestDeletedAccountRelogin" ./internal/scheduler/ -v` | 十三测全 PASS（明细见下） | 1.927s |
| 其余窗口/失效守卫 | `-run "TestDeletedAccountStopsSubmitting\|TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime\|TestSpawnChainSkipsInflightCourse\|TestManualDoneClearsInflight\|TestWindowOpenedWithEmptyPublishes\|TestTokenInvalidTriggersRelogin\|TestSubmitUnauthorizedTriggersRelogin\|TestUnauthorizedBranchDeletedAccountSkipsState\|TestWindowOpenRetriesWithoutWaitingProbe\|TestProbeNowUnauthorizedTriggersRelogin\|TestReloginBackoffCappedAndReset\|TestReloginBackoffWindowBlocksManualTriggers\|TestReloginFailureKeepsBackoff\|TestReloginSuccessWithNilStoreNoPanic"` | 14 测全 PASS | 3.425s |
| 回归锚 1 | `TestWindowOpenSubmitsWithoutProbeReset`（count=3） | PASS×3 | 4.394s |
| 回归锚 2 | `TestAdminStatsWindowOpenedUsesScheduler`（count=3） | PASS×3 | 6.441s |
| 脱敏+连接自愈族 | `TestSanitize*\|TestDoRequestSanitizes*\|TestIsReadErr\|TestLoginNetworkError\|TestLoginRetriesTransientInit\|TestHttpDo\|TestConnErrRetryable` | 9 测全 PASS | 1.464s |
| 迁移/闸门/契约回归 | `TestMigrateAddsPublishMetaColumns\|TestRefuseLegacyDB\|TestLoginByPasswordRejectsWhenGateBudgetExhausted\|TestLoginByPasswordAllowedWhenGateBudgetAvailable\|TestRestoreDoneSkipsResubmit\|TestSubmitSuspendedWhenOpenTimeCleared\|TestClassFullRealtimeNotHoldingMu\|TestRealtimeFullRecheckKeepsManualSuccess\|TestWindowClosedState` | 九测全 PASS（db 1.7s / accounts 1.5s / scheduler 2.9s） | - |
| 轮次标签 | grep "第 N 轮\|R..轮\|round N\|round[0-9]" 产品代码 | 仅 1 条历史文档引用（session/store.go:117 "docs/review-round13.md"），合规 | - |
| 工作区 | `git status --short --branch` | 空（零漂移） | - |

身份防线族十三测明细（全部 PASS）：
`TestDeletedAccountInFlightDropsSuccess` / `TestDeletedAccountReloginSuccessDropsState` / `TestRealtimeRecheckDeletedAccountDropsLog` / `TestDeletedAccountManualInFlightDropsState` / `TestRealtimeRecheckUnauthorizedTriggersRelogin` / `TestDeletedAccountRebuiltSameNameChainDropsSuccess` / `...DropsRelogin` / `...DropsRateLimitBackoff` / `...DropsWindowClosedFull` / `...DropsRealtimeRecheckFull` / `TestMaybeReloginDeletedAccountSkipsMaps` / `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（含 500ms 轮询窗口断言恒 0，防"断言早于重登 goroutine 启动"假绿）/ `...SuccessDropsInflight`。

## 聚焦清单逐项裁决

### 1. 身份防线矩阵里程碑轮闭合（全员清点矩阵式复盘）— ✅ 在位（零漂移）

**sameClientFor 定义 + clientIdentity**（scheduler.go:204 / :215）逐行核对零漂移：接口值经 `reflect.ValueOf(c).Pointer()` 取指针身份，nil/非指针返回 0；"仅判账号名存在挡不住同名重建"语义在定义注释 :199-203 明确。

**7 调用点逐一追到"写什么状态/落什么库行"终局**（全部在持锁段内，每处都是"网络往返后持锁写入前的最后一道身份闸"）：

| 调用点 | 分支 | 写什么状态 | 落什么库行 |
|---|---|---|---|
| :850（ProbeForAccount 回写段） | 探测返回 | 不写 acctData/acctDataAt/openTimeDetected 三处全放弃 | 不落库（纯内存快照） |
| :1489（spawnChain 失效分支） | ErrUnauthorized | 清 inflight、写 failed"教务令牌失效"、maybeRelogin 决策 | AppendLog（select 失效，:1504） |
| :1521（成功分支） | err==nil | 置 done、写 success、清 inflight | AppendLog(success, :1532) + SaveSuccess(:1535) |
| :1551（风控退避分支） | isRateLimitError | markRateLimitedLocked 30s、写 failed | AppendLog（风控，:1558） |
| :1571（窗口关闭分支） | isWindowClosedError | markFullLocked（记 full 防轰炸） | 不落库（markFullLocked 内 AppendLog 满员行 :1770） |
| :1600（实时复核三路入口） | 回锁后统一复核 | 三路各自写（见下） | 见下 |
| :1635（实时复核确证满员） | cErr==nil && full | markFullLocked（doneHas 胜利状态优先让位） | AppendLog（满员行） |

实时复核三路内：:1608 ErrUnauthorized 分支（maybeRelogin + failed + AppendLog :1614）、:1660 普通失败分支（setStateLocked failed 带 read 类区分文案 + AppendLog :1662）、:1627 doneHas 让位分支（不写）。**每条调用点的身份复核（sameClientFor 内含存在性判定）都在持锁写/落库之前**，非同一身份即静默放弃整段（含 maybeRelogin 决策——:1494-1499 明确"重登必须落在身份复核之后"，旧链命中失效但身份已变时绝不触发 maybeRelogin，杜绝污染重建身份的首登退避）。第十条 inflight 公共清位在成功分支身份复核失败时已统一执行（:1513 在 `if err==nil` 判定前），测试固化。

**maybeRelogin 双侧**：决策侧 :1208（锁内 `ClientFor(acct)` 存在性复核，不存在直接返回不写任何 map，含 tokenValid/reloginFail/reloginAt/relogging 四键）+ 写回侧 :1254（goroutine 完成时先复核客户端仍存在）+ :1265-1273 二次 `ClientFor(acct)` 重取当前注册表客户端 `client.Token()` 落库 `UpdateIDToken`。二次重取的核心语义：同名重建即使发生在写回侧复核与落库之间的亚毫秒窗口，取到的 Token 也恒属"当前注册表身份"，绝不落旧身份陈旧值。

**手动五路 accountExists**（handler.go，判据同源 = 凭据表"确实登录过"更强真理源）：课程读 :255、手动报名 :305、手动退选 :397、目标写 :497-512（此路因 handleSetTargets 需同时读凭据用于换绑场景，就地 `LoadCredentials` 遍历语义同源）、状态读 :573。五路全在位，`accountExists` 实现 :1109 逐行核对（LoadCredentials 失败返回 false 保守拒绝，非命中判定错误）。

**写点换类 5 类 + 无锁写点全持锁/唯一性射证**：
- `lastSubmit` 唯一写点 submitAll 首行 :1346 = `nowAlignedLocked()` 位于 :1345 锁内（对齐钟写、tick :1032 对齐钟判读同基）；
- `lastSyncStart` 两写点 :356/:405 均在锁内；`syncing` 三写点 :355/:369（goroutine 回调）/ :404 全锁内；
- `lastProbe` 三写点 :964（ProbeNow 锁内）/ :1089 / :1114（probe 锁内）+ Start 重登补探测 :679 锁内不并发；
- `state.EmptyProbeRuns` 唯一写点 :1155-1158 位于 :1112 持锁段内；
- 无锁写点 `warnedNoTargets`（:1371-1372）宿主唯一性射证：submitAll 唯一调用点 = tick（全仓唯一 `s.submitAll()` 调用 :1035），tick 主循环单 goroutine 顺序执行，`len(chains)==0` 分支锁已释放但无第二宿主 → 单写者无需持锁，与决策契约对齐。

***Locked 写函数族 13 个 + 外部写函数首行取锁双向射证**：13 个 `func (s *Scheduler) xxxLocked`（nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846）全部调用点位于持锁段（spawnChain 各分支从 :1419 锁内一路持锁至 markFull/setState/落库，实时复核段 :1588 显式释放后 :1590 回锁）。外部写函数族首行取锁：SetTargetsForAccount :455 / PurgeAccount :496 / RestoreTargets :526 / RestoreDone :608 / RestoreRefused :627 / StateForAccount :700 / TokenValidFor :732（reloginMu+mu） / ElectivesSnapshotFor :762 / ElectivesSnapshot :879 / HasProbed :889 / WindowOpened :897 / WindowClosed :908 / ProbeNow（取客户端后回写段 :963 锁内） / MarkTokenValid :1314 / CheckClassSelectable :1864 / TryAcquireSubmit :1897 / MarkDone :1923 / RemoveDone :1990 / RemoveFull :2039 全在函数体首行/段首取锁。

### 2. OBSERVE-117-01 知识位 — ✅ 在位

写回侧先 ClientFor 复核（:1254）→ 二次 `ClientFor(acct)` 重取当前注册表客户端 `client.Token()`（:1265-1273）落库新 token：同名重建场景下即使重建发生在复核后的亚毫秒窗口内，二次重取返回的必须是新身份客户端的 Token，落库绝不会串旧身份陈旧值。实测 `TestDeletedAccountRebuiltSameNameChainDropsRelogin`（relogCalls==0）与 `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（reloginFail 无残留）双绿。

### 3. B110-01 审计链 — ✅ 零漂移

- **手动 6 失败位 AppendLog**：handler.go :362（报名失效）/ :374（报名 read 类）/ :381（报名其余失败）/ :443（退选失效）/ :452（退选 read 类）/ :459（退选其余失败）逐一核对，6 位全挂 `if err := ...; err != nil { log.Printf }`，无一吞错；
- **成功审计行**：手动报名 MarkDone 内 AppendLog :1976 + SaveSuccess :1973；手动退选 RemoveDone 内 AppendLog :2029 + DeleteSuccess :2020 + SaveRefused :2026；目标保存 :558；登录 :237；注销 :616；管理员登录/删号/配置 :136/:1055/:864；scheduler 自动链族失败全落 AppendLog（失效 :1504、风控 :1558、实时复核失效 :1614、普通失败 :1662、满员 :1770、成功 :1532）；
- **零吞错穷举**：全仓 `_ =`/`_, _ =`/直接赋值忽略形态逐一归因——store 写调用点全部 `if err := ...; err != nil` 挂接；`_ = d.Sched.MarkDone`（:387）/`_ = d.Sched.RemoveDone`（:467）属 scheduler 内存态方法（成功路径写 JSON 响应前已统一处理，方法自身落库不吞错），非持久化层直写；`_, _ = s.ProbeForAccount`（probe :1075，结果已由方法内部自行写快照）与 `_ = pw.Prewarm()`（:307，预热失败无业务副作用）为刻意忽略且非落库。**零命中落库层吞错**；
- **网络层 token 脱敏延续抽查**：zhidao/client.go :581 sanitizeError（`*url.Error` 文本剥 URL、`sanitizerErr` Unwrap 下沉底层错误保 `errors.As/Is` 判型穿透）+ :450 doRequest 统一上抛前调用；scheduler.go :1283 maskedToken 只显前 8 位、短 token 显 "***"。`TestSanitizeErrorPreservesJudgment`（7 子测判型穿透）+ `TestSanitizeErrorStripsTokenFromDialError` + `TestDoRequestSanitizesDialError` + `TestSanitizeErrorOriginalErrorPreserved` + `TestSanitizeErrorNilSafe` + `TestSanitizeErrorNeverEmptyError` 全绿，非 url.Error 原样透传不误伤文案契约。

### 4. O105-01 抖动基线 — ✅ 实测绿

- **夹具在位**：zhidao/client_test.go:25 socketPreheat + TestMain :40 + loginMockServer readyProbe :88 双保险；zhidao/sanitize_test.go:97 socketPreheat；accounts/manager_test.go:26 readyProbe（无 socketPreheat，注释标注"8 轮全绿实证无残余，第一候选已在注释标记"）；api/handler_test.go:67 套接字预创建 + :185 readyProbe；
- **定向 race 四包**：zhidao 12.7s / accounts 1.3s / scheduler 15.0s / api 19.0s 全绿，含身份防线族十三测 + 窗口守卫 14 测；
- **回归锚**：TestWindowOpenSubmitsWithoutProbeReset 与 TestAdminStatsWindowOpenedUsesScheduler 均 count=3 复跑 PASS（4.4s/6.4s），未现冷启动 flake。

### 5. LOW-132/133 回首核 — ✅ 通过

- **LOW-132 白线核对**（修复提交含 :372/:377）：scheduler.go:372 `lastSyncFailAt = s.nowAlignedLocked()` + :377 `syncFailedWindow = s.nowAlignedLocked()` 写入全对齐钟；判读侧 :342 `now.Sub(lastSyncFailAt)` 的 now 为调用方 tick :974 传入的对齐钟——写读同基自洽；成功清位 :392/:394 用 `time.Time{}` 不混时间基；
- **LOW-133 白线核对**（修复提交 d75f38c）：四处 TTL 判读点 :780/:795/:801/:1871 全改 `nowAlignedLocked().Sub(snappedAt)`，与写入侧 :835/:864/:1116/:966 对齐钟同基准；`git show d75f38c` 逐差分核对四处 diff 形态与报告一致；
- **时间基全量扫零残留**：生产代码 `time.Since`/`time.Now()` 全量 grep，残余全为自洽设计——`reloginAt`（scheduler.go :1220/:1227 读、:1231/:1261 写，本地钟写读同基，纯重登节流语义）；`gateWindow`（accounts/manager.go :53/:54/:71/:74/:226/:227，本地钟写读同基自洽，登录闸门语义）；session/store.go :74/:106/:128/:156/:168/:183/:198 本地钟为自身会话 TTL 语义；api handler.go :1176 loginLimiter 本地钟为 IP 限流语义；zhidao/client.go :113/:135/:137 时钟对齐自身采样、:328/:355 captcha/deviceId 用墙钟无偏差要求。scheduler 内部时间基全量对齐钟，无混用孤岛。

### 6. 新契约角度纵深（自选 ×2）

**角度 A：窗口状态三判据单源 open 快照（windowClosedLocked 单源 + tick 判读单快照复用）**
纵向走查窗口判定的"单源"与"单快照"纪律全链：
- `windowClosedLocked`（:918-938）三条判据共用 `open := s.openTimeForLocked("")` 单次快照——判据 2（时钟失败 ≥3）与判据 3（幽灵窗口 EmptyProbeRuns≥3）的 `!open.IsZero() && nowAlignedLocked().After(open)` 复用一个 open 值，杜绝"识别值在两次读取间被新批次覆盖"的不一致窗口；判定侧 :1145 probe 入账同样 `open := s.openTimeForLocked("")` 一次快照复用两处 10s 裕量（主判据 + EmptyProbeRuns 入账），注释明确"热改亚毫秒窗口内主判据与入账可能基于新旧两个不同 open（同族）"；
- tick 判读 :974-978 开头取一次 open 快照 → `probeIntervalForOpen` :987 复用同一 open（不内重新取）；提交守卫 :1011-1024 亦用同一 open；`StateForAccount` :703 与 `WindowClosed()` :910 共用 `windowClosedLocked()` 实现，杜绝两套真相分叉；
- 兜底判据（时钟/幽灵窗口）返回 true 时不回写 `state.WindowClosed` 字段——展示层与挂起状态此前曾分叉（修复后 /api/state 仍下发 window_closed=false 的场景已收敛）。实测 `TestWindowClosedState`（-time.Hour 主判据）、`TestWindowClosedProbeDropsToFar`、`TestWindowOpenedWithEmptyPublishes`（开过窗不误标关闭）全绿。结论：判定侧与入账侧"单次 open 快照复用"纪律全覆盖，无两值不一致窗口。

**角度 B：年级隔离快照回退链三态（ElectivesSnapshotFor 语义矩阵）**
纵向走查 `ElectivesSnapshotFor`（:761-805）的三态回退语义：
- **态一（目标账号）**：`len(acctTargets[acct]) > 0` 判据（非 map key 存在性——handler 清空目标落空 slice 留下 key，按 key 判断会把空目标账号错误走专属路径、浏览即触发网络探测）：专属帧新鲜 → 返回；缺失/过期 → 返回 (nil,false) 触发真刷新，**绝不回退全局 lastData**（契约 8：全局帧恰被 order[0] 刷新为新鲜时回退 = 年级串线 + 浏览者永不触发本账号刷新）；
- **态二（非目标但曾有专属帧）**：有专属帧新鲜即返回；过期同样 (nil,false) 触发刷新绝不回退全局（含"曾有过专属帧但已过期的中间态"——对纯浏览"快、无网络开销"只对从未有过专属帧成立，过期中间态回退既不快又是错年级，注释 :786-798 明确此项）；
- **态三（非目标且从未有专属帧）**：允许回退全局帧 lastData（fresh 判定同 nowAlignedLocked 对齐钟 TTL）；
- 配套：probe() 只对 `AccountsWithTargets()` 遍历刷新 per-account 帧；`ProbeForAccount` 回写段 sameClientFor（:850）防删号同名重建写错年级帧；`CheckClassSelectable` :1863 不使用全局帧兜底（跨年级帧无参考价值），过期快照放行交平台把关。结论：三态边界与 TTL 判读（对齐钟）完全一致，年级串线的三条历史根因（全局帧回退 / 判据 key 存在性 / 过期帧回退）均已收敛，无漂移。

## 维持观察项（与本轮视角 A/B 伴随，无恶化）

- `maybePrewarm` 无独立单测（纯 GET /login 静默预热，错误静默，收益为性能非正确性）；
- `probeSem` cap=4 常驻（ponytail 注释标注：账号数 >50 或平台放宽熔断再调）；
- `reloginAt`/`gateWindow` 本地钟时间基（自洽设计，非混用孤岛）；
- `syncFailedWindow` 写而不读（判据用 syncFailStreak，留档字段）；
- accounts 包夹具仅有 readyProbe 无 socketPreheat（长期实证无残余，第一候选补丁已在注释标记）；
- realtime 人数复核 `IsClassFull` 恒 false（平台未下发 maxCount，防御性保留路径）；
- `warnedNoTargets` 单写者依赖 tick 主循环单 goroutine 唯一性（若未来引入第二个 submitAll 调用点需重审）。

## 结论

**建议：APPROVE**

身份防线矩阵里程碑轮闭合（7 调用点 + maybeRelogin 双侧 + 手动五路 + 写点换类全持锁 + *Locked 族双向射证全部零漂移）；OBSERVE-117-01 知识位在位；B110-01 审计链零漂移；O105-01 定向 race 四包实测绿（身份防线族十三测 + 回归锚 count=3 复跑）；LOW-132/133 白线核对 + 时间基全量扫零通过。零 CRITICAL/HIGH/MEDIUM/LOW，无修复需求。工作区零漂移（git status 空，仅本报告落盘 archive/）。