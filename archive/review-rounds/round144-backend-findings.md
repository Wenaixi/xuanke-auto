# R144 后端只读审查 findings（身份防线矩阵第五十九轮）

> 审查代理：r144-backend 只读代理。项目根 `E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`，工作树基线 = 6179674（R143 归档，身份防线矩阵第五十八轮闭合）。本轮全仓库零修改（仅本报告写入 archive/）。
> 实测环境：Windows 11，`export PATH=/d/mingw64/bin:$PATH` 启用 gcc 后 `go test -race`。

## 结论前置

- **CRITICAL：0**
- **HIGH：0**
- **MEDIUM：0**
- **LOW：0（无新增）**

双端零修复需求——**连续第二十九轮零 MAJOR，本轮纯观察**。身份防线矩阵第五十九轮闭合；OBSERVE-117-01 知识位第二十七轮确认在位；B110-01 审计链第三十四轮零漂移；O105-01 抖动基线实测绿；LOW-132/133 回首核通过。建议 **APPROVE**。

## 验证表（实测数据）

| 项 | 命令 | 结果 | 实测耗时 |
|---|---|---|---|
| 编译 | `cd backend && go build ./...` | exit 0 | - |
| 静态检查 | `go vet ./...` | exit 0 | - |
| race zhidao | `go test -race -count=1 ./internal/zhidao/` | ok | 2.011s |
| race accounts | `go test -race -count=1 ./internal/accounts/` | ok | 1.386s |
| race scheduler | `go test -race -count=1 ./internal/scheduler/` | ok | 15.070s |
| race api | `go test -race -count=1 ./internal/api/` | ok | 14.835s |
| 其余后端包 | `go test -race -count=1 ./internal/store/ ./internal/db/ ./internal/session/ ./internal/config/ ./internal/runtime/` | 五包全 ok | store 11.5s / db 2.2s / session 1.7s / config 1.6s / runtime 1.6s |
| 身份防线族十测 | `go test -race -run "TestDeletedAccountRebuiltSameName|TestMaybeReloginDeletedAccount|TestRealtimeRecheck" ./internal/scheduler/` | 十测全 PASS（明细见下） | 1.934s |
| 回归锚 1 | `TestWindowOpenSubmitsWithoutProbeReset`（含 count=3 复跑） | PASS | 1.04s / 3 次 4.3s |
| 回归锚 2 | `TestAdminStatsWindowOpenedUsesScheduler`（含 count=3 复跑） | PASS | 0.41s / 3 次 1.9s |
| 脱敏+连接自愈族 | `go test -race -run "TestSanitize|TestDoRequestSanitizes|TestIsReadErr|TestLoginNetworkError|TestLoginRetriesTransientInit" ./internal/zhidao/` | 8 测全 PASS | 1.500s |
| 迁移规范 | `TestMigrateAddsPublishMetaColumns` + `TestRefuseLegacyDB` | 全 PASS | 0.12s/0.11s |
| 轮次标签 | grep "第 N 轮\|R..轮\|round N" 产品代码 | 仅 1 条历史文档引用（session/store.go:117 "docs/review-round13.md"），合规 | - |
| 工作区 | `git status --short --branch` | 空（零漂移） | - |

身份防线族十测明细（全部 PASS）：
`TestRealtimeRecheckDeletedAccountDropsLog` / `TestRealtimeRecheckUnauthorizedTriggersRelogin` / `TestDeletedAccountRebuiltSameNameChainDropsSuccess` / `...DropsRelogin` / `...DropsRateLimitBackoff` / `...DropsWindowClosedFull` / `...DropsRealtimeRecheckFull` / `TestMaybeReloginDeletedAccountSkipsMaps` / `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin` / `...SuccessDropsInflight`。

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第五十九轮闭合 — ✅ 在位（零漂移）

**sameClientFor 定义**（scheduler.go:204，注释 :199-203）+ `clientIdentity`（:215-224）逐行核对零漂移：接口值经 `reflect.ValueOf(c).Pointer()` 取指针身份，nil/非指针返回 0；定义注释明确"仅判账号名存在挡不住同名重建，接口值比对用反射指针身份，需持 s.mu"。

**7 调用点逐一追到写状态/落库终局**（全部在持锁段内）：

| 调用点 | 分支 | 写什么状态 | 落什么库行 |
|---|---|---|---|
| :850（ProbeForAccount 回写段） | 探测返回 | 不写 acctData/acctDataAt/openTimeDetected | 不落库（纯内存快照） |
| :1489（spawnChain 失效分支） | ErrUnauthorized | 清 inflight、写 failed"教务令牌失效"、maybeRelogin 决策 | AppendLog（select 失效，:1504） |
| :1521（成功分支） | err==nil | 置 done、写 success、清 inflight | AppendLog(success, :1532) + SaveSuccess(:1535) |
| :1551（风控退避分支） | isRateLimitError | markRateLimitedLocked 30s、写 failed | AppendLog（风控，:1558） |
| :1571（窗口关闭分支） | isWindowClosedError | markFullLocked（记 full 防轰炸） | 不落库（markFullLocked 内 AppendLog 满员行 :1770） |
| :1600（实时复核三路入口） | 回锁后统一复核 | 三路分支各自写 | 见下 |
| :1635（实时复核确证满员） | cErr==nil && full | markFullLocked | AppendLog（满员行） |

实时复核三路内还含 :1608 ErrUnauthorized 分支（maybeRelogin + failed + AppendLog :1614）与 :1660 普通失败分支（setStateLocked failed + AppendLog :1662）。**每条调用点都是"写前先同 client 指针复核，非同一身份即静默放弃整段"**，无裸露写点。第十条 spawnChain 成功分支 inflight 公共清位路径在身份复核失败时也已统一执行（scheduler.go:1513 `delete(s.inflight[acct], t.ClassID)` 位于 `if err == nil` 判定前），回归测试 `TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight` 已固化。

**maybeRelogin 双侧**：决策侧 :1208（锁内 `ClientFor(acct)` 存在性复核，不存在直接返回不写任何 map）+ 写回侧 :1254（goroutine 完成时先复核）+ :1265-1273 二次 `ClientFor(acct)` 重取当前注册表客户端 `client.Token()` 落库 `UpdateIDToken`。二次重取的核心语义：即使同名重建在写回侧复核与落库之间发生（亚毫秒窗口），取到的也是新身份的 Token，落库的 token 恒是"当前注册表身份"的，绝不落旧身份陈旧值。OBSERVE-117-01 知识位第二十七轮确认在位。

**手动五路 accountExists**（handler.go，凭据表 = "确实登录过"真理源，判据同源）：课程读 :255、手动报名 :305、手动退选 :397、目标写 :497-512（此路因 handleSetTargets 需要同时读凭据用于换绑场景，就地 LoadCredentials 遍历而非 accountExists，语义同源）、状态读 :573。五路全在位且判据一致。

**写点换类 5 类全持锁/唯一性射证**：
- `lastSubmit` 唯一写点 submitAll 首行 :1346 `s.lastSubmit = s.nowAlignedLocked()` 位于 :1345 s.mu.Lock() 段内（对齐钟写、tick :1032 对齐钟判读同基）；
- `lastSyncStart`/`syncing` 两写点均在 maybeSyncClock 锁内（:355/:356 决策段、:404/:405 无客户端回落段；goroutine 内 :369 回调复位锁内）；
- `lastProbe` 三写点全部锁内（ProbeNow :964、probe :1089 失败分支、probe :1114 成功分支，:1112 s.mu.Lock() 段内）+ Start 主循环 :679 重登补探测也锁内；tick 读取 :976 锁内取单快照；
- `state.EmptyProbeRuns` 唯一写点 probe :1155-1158 位于 :1112 持锁段内；
- 无锁写点 `warnedNoTargets`（:1371-1372）宿主唯一性射证：submitAll 唯一调用点 = tick :1035（scheduler.go 全仓唯一 `s.submitAll()`），submitAll 在 `len(chains)==0` 分支（锁已释放后）读-写 warnedNoTargets，无第二宿主 → 单写者、tick 主循环单 goroutine 顺序执行，无需持锁。与决策契约 20 一致（此字段只负责"打一次日志"，非状态机，读侧无消费）。

**\*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证**：13 个 `func (s *Scheduler) xxxLocked` 定义逐一核对（nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846），全部调用点位于持锁段（逐调用点前 6 行内存在 s.mu.Lock()，spawnChain 各分支从 :1419 锁内一路持锁至 markFull/setState/落库）。外部写函数族首行取锁射证：SetTargetsForAccount :455、PurgeAccount :496、RestoreTargets :526、RestoreDone :608、RestoreRefused :627、StateForAccount :700、TokenValidFor :733、ElectivesSnapshotFor :762、ProbeForAccount :849（回写段）、ElectivesSnapshot :879、HasProbed :889、WindowOpened :897、WindowClosed :908、ProbeNow :963、tick :975 段、submitAll :1345、MarkTokenValid :1314、MarkDone :1923、RemoveDone :1990、RemoveFull :2039 全在首行/段首取锁。

### 2. OBSERVE-117-01 知识位第二十七轮 — ✅ 在位

写回侧先 ClientFor 复核（:1254）→ 二次 `ClientFor(acct)` 重取当前注册表客户端 `client.Token()`（:1265-1273）落库新 token，同名重建场景绝不串旧身份：二次重取保证即使重建发生在复核后亚毫秒窗口，取到的 Token 也属于新身份客户端。实测 `TestDeletedAccountRebuiltSameNameChainDropsRelogin` 绿（relogCalls==0，旧链命中失效不再触发 maybeRelogin 污染重建身份）。

### 3. B110-01 审计链第三十四轮 — ✅ 零漂移

- **手动 6 失败位 AppendLog**：handler.go :362（报名失效）/ :374（报名 read 类）/ :381（报名其余失败）/ :443（退选失效）/ :452（退选 read 类）/ :459（退选其余失败）逐一核对，6 位全落 `if err := ...; err != nil { log.Printf }`，无一吞错；
- **成功审计行**：手动报名 MarkDone 内 AppendLog :1976 + SaveSuccess :1973；手动退选 RemoveDone 内 AppendLog :2029 + DeleteSuccess :2020 + SaveRefused :2026；目标保存 :558；登录 :237；注销 :616；管理员登录/删号/配置 三行 :136/:1055/:864；scheduler 自动链族失败全部落 AppendLog（失效 :1504、风控 :1558、实时复核失效 :1614、普通失败 :1662、满员 :1770）；
- **零吞错穷举**：全仓 grep `_ =`/`_, _ =`/直接赋值忽略形态——store 写调用点 29 处全部 `if err := ...; err != nil` 挂接；`_ = d.Sched.MarkDone`（:387）/`_ = d.Sched.RemoveDone`（:467）属 scheduler 内存态方法（返回 error 仅为接口签名一致性，成功路径写 JSON 响应前已统一处理，MarkDone/RemoveDone 内部落库自身不吞错），非落库点；`_, _ = s.ProbeForAccount`（probe :1075，探测结果已由 ProbeForAccount 内部自行写快照，返回值仅为传递数据）与 `_ = pw.Prewarm()`（:307，预热失败无业务副作用）为刻意忽略且非持久化层。**零命中落库层吞错**；
- **网络层 token 脱敏延续抽查**：zhidao/client.go :581 sanitizeError（`*url.Error` 文本剥 URL、Unwrap 下沉底层错误保判型穿透）+ :450 doRequest 统一上抛前调用；scheduler.go :1283 maskedToken 只显前 8 位。`TestSanitizeErrorPreservesJudgment`（7 子测：dial/write/read-rst/fin/shortread/timeout/business 判型穿透）+ `TestDoRequestSanitizesDialError` 全绿，非 url.Error 原样透传不误伤。

### 4. O105-01 抖动基线 — ✅ 实测绿

- **夹具在位**：zhidao/client_test.go:25 socketPreheat + TestMain :39 包级预热 + loginMockServer :50 readyProbe 双保险；accounts/manager_test.go:26 readyProbe；api/handler_test.go:67 套接字预创建 + :185 readyProbe；sanitize_test.go:97 socketPreheat；
- **定向 race 四包**：zhidao 2.0s / accounts 1.4s / scheduler 15.1s / api 14.8s 全绿；
- **回归锚**：TestWindowOpenSubmitsWithoutProbeReset 与 TestAdminStatsWindowOpenedUsesScheduler 均 PASS，且 `-count=3` 复跑稳定（3 次 4.3s / 1.9s），未现冷启动 flake。

### 5. LOW-132/133 回首核 — ✅ 通过

- git 白线核对：LOW-132-01 修复点 scheduler.go:372 `lastSyncFailAt = s.nowAlignedLocked()` + :377 syncFailedWindow 同基准，判读侧 :342 `now.Sub(lastSyncFailAt)` 同对齐钟——写读同基自洽；
- **时间基全量扫零残留**：生产代码 `time.Since`/`time.Now()` 全量 grep，残余仅两族——
  - `reloginAt`（scheduler.go :1220/:1227 读、:1231/:1261 写，本地钟写读同基自洽）；
  - `gateWindow`（accounts/manager.go :71/:74/:226/:227 + :53/:54/:55 与 gateTryAcquire，本地钟写读同基自洽）。
  两族均为"纯本地节流/退避语义，不涉及服务端时钟对齐"的自洽设计，合规。scheduler 内部时间基全量对齐钟（nowAlignedLocked），session/store/api 层用本地钟为自身 TTL/限流语义，非混用孤岛。

### 6. 新契约角度纵深（自选 ×2）

**角度 A：B42-01 登录闸门族双侧收口终局（gateTryAcquire 与 gateWait 共享单计数）**
追踪登录流量全部入口：`Manager.Relogin`（自动重登，gateWait 阻塞排队，manager.go:173）与 `LoginByPassword`（学生手动登录，gateTryAcquire 非阻塞准入 :244）+ 管理员换绑（handleAdminConfig? 不——换绑实为 handleLogin 撞名学生的教务登录路径，同样经 LoginByPassword 收口）。两路共享同一 gateMu/gateUsed 计数，全账号合计每分钟 doLogin 严格 ≤ gateLoginPerMin(2)。实测 `TestLoginByPasswordRejectsWhenGateBudgetExhausted`（红→绿：拒绝且 doLogin 0 次）与 `TestLoginByPasswordAllowedWhenGateBudgetAvailable`（跨分钟旧窗口重置放行）绿。配套 `ResetGateForTest` 仅测试用、正式代码不调用。闸门族的完整闭环确认：**任何直发平台 doLogin 的入口只剩 `ReloginIfNeeded`（自动）与 `submitLogin`（在 Login 内部，而 Login 只能由 LoginByPassword/gateWait 后的 ReloginIfNeeded 触达）**。

**角度 B：凭据 AES-256-GCM 加密链 + 状态码家族复盘**
- 加密链：secure/crypto.go `LoadOrCreateKey`（环境变量 64hex 或 DB 旁 .master_key，32 字节校验，损坏即显式报错拒绝启动）+ `Encrypt`（AES-256-GCM，随机 nonce 前置，hex 输出）+ `Decrypt`。main.go :50-55 注入 encrypt/decrypt 闭包 → accounts.LoginByPassword :279 SaveCredential（password_enc 密文）+ zhidao 层绝不落明文；api/handler.go secureEncrypt 加 `enc:` 前缀；main.go :87-96 vision_key 严格要求 enc: 前缀，旧版未加密明文彻底拒绝加载。主密钥缺失/损坏 `log.Fatalf("初始化数据加密密钥失败")` 拒绝启动（决策契约 18 对齐）。
- 状态码家族核对（B40-01 逐家族）：panic 500（:1239 recoverMiddleware）/ 会话 401（:1133 requireAuth）/ 管理 403（:642 requireAdminSession + router :72/:102/:116 requireJSONBody 三处 CSRF 门）/ 限流 429（router :107/:121）/ 未知 /api 404（router :198）/ 配置落库失败 500（:855）/ stats 目标数失败 500（:960）。**五大家族（鉴权/CSRF/限流/panic/持久化层）全部真实 HTTP 状态码 + body code 双通道**，前端契约只读 body code 不受影响。实测 `TestLoginRejectsFormContentType`（CSRF 403）、`TestRecoverMiddlewareHidesPanicDetail`（500）、`TestAdminStatsTargetsLoadFailureReturns500`（500）覆盖。httptest 断言真实状态码而非仅看 body。

## 维持观察项（与前轮一致，无恶化）

- `maybePrewarm` 无独立单测（纯 GET /login 静默预热，错误静默，收益为性能非正确性）；
- `probeSem` cap=4 常驻（ponytail 注释标注：账号数 >50 或平台放宽熔断再调）；
- `reloginAt`/`gateWindow` 本地钟时间基（自洽设计，非混用孤岛）；
- `syncFailedWindow` 写而不读（判据用 syncFailStreak，留档字段）；
- accounts 包夹具仅有 readyProbe 无 socketPreheat（8 轮全绿实证无残余，第一候选补丁已在注释标记）；
- realtime 人数复核 `IsClassFull` 恒 false（平台未下发 maxCount，防御性保留路径）。

## 结论

**建议：APPROVE**

身份防线矩阵第五十九轮闭合（7 调用点零漂移 + maybeRelogin 双侧 + 手动五路 + 写点换类全持锁/唯一性射证 + \*Locked 族双向射证）；OBSERVE-117-01 知识位第二十七轮确认在位；B110-01 审计链第三十四轮零漂移；O105-01 实测绿；LOW-132/133 回首核通过。零 CRITICAL/HIGH/MEDIUM/LOW，无修复需求。工作区零漂移。
