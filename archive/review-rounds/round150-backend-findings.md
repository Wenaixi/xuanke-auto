# R150 后端只读审查发现（身份防线矩阵闭合第六十五轮）

> 审查基线：commit df1f5ce（docs(review): R149 双 findings 收尾——身份防线矩阵第六十四轮闭合）。
> 模式：除本 findings 报告外零文件写入，工作区零漂移（唯一未跟踪 = 并行前端代理的 round150-frontend-findings.md）。
> 实测时间：2026-09-24；环境：Windows 11 / go1.26.8 / gcc(mingw64 /d/mingw64/bin)。

## 结论前置

- **CRITICAL：无**
- **HIGH：无**
- **MEDIUM：无**
- **LOW（观察项，不阻塞）：**
  1. `accounts` 包测试夹具仍只有 readyProbe 就绪探测、无 socketPreheat 双保险（manager_test.go:24 注释自述"若未来再出 flake 第一候选即补 socketPreheat"）——本轮含 `-race -count=1` 强制非缓存全包复跑 accounts 绿（1.426s），8+ 轮实证无残余，维持观察。
  2. `ProbeForAccount` 写 `openTimeDetected[acct]` 识别槽刻意不回写全局 `lastProbe`（注释 :868-871 自述"lastProbe 只归 probe()/ProbeNow 的全局维度"）——防管理员穿透探测旁路全校节流闸门，刻意设计，维持观察。
  3. `classFullRealtime`（scheduler.go:1751）仍保留为"平台未来下发 maxCount 时自动生效"防御路径（当前真满员主路径为快照判满 classFullInSnapshot，实证 maxCount 恒 0）——维持观察。
- **建议：APPROVE**

## 验证表（全部实测）

| 项目 | 结果 | 数据 |
|---|---|---|
| 工作树基线 | ✅ | git log 顶部 = df1f5ce（R149 归档，commit 仅改动 3 个 docs/review 文档，白线核对零产品代码） |
| 工作区漂移 | ✅ | `git status --short` 仅 `?? archive/review-rounds/round150-frontend-findings.md`（并行代理产物，非本次） |
| `go build ./...` | ✅ | 退出码 0 |
| `go vet ./...` | ✅ | 退出码 0 |
| 定向 `-race` 四包（含强制非缓存） | ✅ | zhidao：`-race -count=1` 2.101s；accounts：`-race -count=1` 1.426s；scheduler：`-race -count=1` 15.007s；api：`-race -count=1` 20.618s——全部 `ok`。race 定向身份防线族十五测 1.913s 绿。全量后端 `go test -race -count=1 ./internal/...` 十包全绿（scheduler 15.815s / api 13.992s） |
| 回归锚双绿 | ✅ | `TestWindowOpenSubmitsWithoutProbeReset`（scheduler_test.go:1277）显式 `-count=1` 7.134s 绿；`TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go:1011）显式 api 包 0.670s 绿（PASS 0.39s） |
| 契约轮次标签扫描 | ✅ | 产品代码 `grep "第.*轮\|R..轮\|round N"` 零命中（scheduler.go:235 `ctx, cancel` 已排除；session/store.go:117 引用 `docs/review-round13.md` 为文档引用合规）。测试文件 scheduler_test.go 仅 3 处"第 N 轮"叙述（等待轮次/空快照轮数语义），属测试叙述合规 |
| 时间基残留扫描 | ✅ | 见 LOW-132/133 回首核 |
| `_ =` / `_, _ =` / 落库忽略形态穷举 | ✅ | 见 B110-01 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十五轮闭合 —— ✅ 在位

**sameClientFor 定义（scheduler.go:204）与 7 调用点零漂移。** 定义：`ClientFor(acct)` 取注册表现客户端 → `clientIdentity` 反射指针比对（`reflect.ValueOf(c).Pointer()`，:215-224）；nil / 客户端不存在恒 false。逐一核对 7 调用点"写什么状态/落什么库行"的终局：

- **:850（ProbeForAccount 回写段）**：不通过 → 放弃写 `openTimeDetected[acct]` + `acctData/acctDataAt` 三 map。终局：旧探测链（FindElectives 长 15s 往返）不污染重建账号识别槽与年级快照。
- **:1489（失效分支）**：不通过 → `delete(inflight[acct], classID)` 后静默 return，**且不触发 maybeRelogin**（:1498-1499 注释明确：旧链命中 ErrUnauthorized 但身份已变不得重登，防幽灵 reloginFail 计数污染重建身份首登退避）。终局：不写 tokenValid/reloginFail/reloginAt/relogging 四 map、不落 AppendLog。
- **:1521（成功分支）**：不通过 → 放弃写 `done[acct]` + `setStateLocked(success)` + `SaveSuccess` 库行 + AppendLog。终局：重建身份重启后无假成功行。
- **:1551（风控退避）**：不通过 → 放弃 `markRateLimitedLocked` + failed 状态 + AppendLog。终局：重建身份无假退避（黄金期不被静默跳过）。
- **:1571（窗口关闭按满员）**：不通过 → 放弃 `markFullLocked`。终局：重建身份无假满员永久退避。
- **:1600（实时复核三路共用入口）**：回锁后先指针身份复核，再进 ErrUnauthorized / 确证满员 / 普通失败三路。终局：复核网络段（最长 15s）内删号重建后，旧链不写 failed 状态、不触发 maybeRelogin、不写 full。
- **:1635（确证满员分支内层复核）**：不通过 → 放弃 `markFullLocked`。终局：与 doneHas 守卫（:1627 绝不覆盖手动胜利状态）叠成双层防线。

**maybeRelogin 双侧闭合：**
- 决策侧（:1208）：入口锁内 `ClientFor` 存在性复核，已删账号不发起、不写任何 relogin 族 map。实测锚 `TestMaybeReloginDeletedAccountSkipsMaps`（scheduler_test.go:3477，race 定向组通过）验证四 map 均无残留。
- 写回侧（:1254）：goroutine 完成后先 `ClientFor` 存在性复核（已删则只清 relogging 静默放弃，:1254-1258）。
- **二次 ClientFor 重取（:1265-1273）OBSERVE-117-01**：见第 2 节详述。

**手动五路 accountExists（handler.go）**：:255 课程读 / :305 手动报名 / :397 手动退选 / :497-512 目标写（此路内联 LoadCredentials 循环与 accountExists 同源，判据"凭据表 = 确实登录过的更强真理源"）/:573 状态读——五路全覆盖。`accountExists` 定义 :1109-1120（LoadCredentials 逐账号比对，读取失败返回 false = 拒绝路径安全侧）。

**写点换类 5 类全持锁/唯一性射证（逐写点核对持锁段）：**
- `lastSubmit`：submitAll:1346 锁内置 `nowAlignedLocked()` 写入；读侧 tick:1029 锁内读 + submitIntervalFor 判读。写读同对齐钟。
- `lastSyncStart`/`syncing`：maybeSyncClock:355-356 锁内置 `s.syncing=true` + `lastSyncStart=now`；完成回调 :369/:393 锁内复位；无客户端分支 :404-405 锁内复位（:400-406 持锁段实测确认）。
- `lastProbe`：probe() :1089/:1114 锁内写；ProbeNow:964 锁内；Start 主循环 reloginResults 回传 :678-679 锁内（注释自述"避免 goroutine 并发写 s.lastProbe 竞态"），是唯一外围写点且持锁。
- `state.EmptyProbeRuns`：probe() :1156-1158 锁内（入账 +10s 裕量 `now.After(open.Add(10s))`，:1145 起同持锁段）。
- **无锁写点 `warnedNoTargets`（:1371-1372）宿主唯一性射证**：字段声明 :176，唯一读写发生在 submitAll 的 `len(chains)==0` 顶级分支内（该路径无 goroutine 并发）——submitAll 被 tick 串行调用且 chains 为空时无并发子链，单写单读标记合规。测试锚 `TestSubmitAllWarnsOnceOnNoTargets`（:2020）固化"只警告一次"。

**`*Locked` 写函数族 13 个 + 外部写函数首行取锁双向射证**：13 个 Locked 函数（nowAlignedLocked/openTimeForLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/rebuildCoursesLocked/tokenValidForLocked/windowClosedLocked/isRateLimitedLocked/markRateLimitedLocked/markFullLocked/releaseFullIfFreedLocked/statusIndexLocked/setStateLocked）全部带"需持 s.mu"契约注释且调用方均处持锁段（实测核对 markFullLocked 三调用点 :1458/:1575/:1639、markRateLimitedLocked :1555 均在持锁段，isRateLimitedLocked/releaseFullIfFreedLocked :1442/:1451 均在链内持锁段）。外部写函数（SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkDone/RemoveDone/RemoveFull/submitAll/ProbeNow/ProbeForAccount/StateForAccount/MarkTokenValid/TryAcquireSubmit）首行均为 `s.mu.Lock()` + `defer s.mu.Unlock()`——双向无例外（TryAcquireSubmit:1897 / MarkTokenValid:1312 起 reloginMu→s.mu 双锁序对齐）。

### 2. OBSERVE-117-01 知识位第三十三轮 —— ✅ 在位

maybeRelogin 写回侧完整合同逐行核对（scheduler.go:1244-1284）：
- 先 `ClientFor` 存在性复核（:1254）——已删账号整个成功分支（含内存写与落库）静默放弃，只清 relogging。
- 成功分支（err==nil && relogged）内**二次 `ClientFor`（:1265）重取当前注册表客户端指针** → `client.Token()`（zhidao/client.go:195 锁内读当前 token）→ 非空才 `UpdateIDToken` 落库（store.go:29 SaveCredential UPSERT）。
- 同名重建场景（旧链触发重登、重登耗时期间删号重建）下二次重取的是**新身份自带的新 token**——重建身份自身所属，绝不串旧身份。写序（先复核存在性、后重取 Token）杜绝"复核通过后被删除"毫秒窗口：复核与重取同持 s.mu，删除的 PurgeAccount 同锁互斥。
- 落库失败记日志（:1270），零吞错。日志走 `maskedToken`（:1283/:1332，>8 位仅前 8 位、≤8 位 `***`，测试锚 `TestMaskedTokenBoundary` :2048）。

### 3. B110-01 审计链第四十轮 —— ✅ 在位

- **手动 6 失败位 AppendLog 全覆盖**：handler.go:362（报名失效）/ :374（报名 read）/ :381（报名业务失败）/ :443（退选失效）/ :452（退选 read）/ :459（退选业务失败）——全部 `if err := AppendLog(...); err != nil { log.Printf }` 记日志。TDD 锚：`TestManualElectiveFailureAppendsLog`（:1800）断言业务失败落库失败行（select+exit 双动作）、`TestManualElectiveReadErrAppendsLog`（:1866）、`TestHandleElectivesSelectUnauthorizedRelogin`（:1965）断言手动路径失效触发 maybeRelogin 且 state.TokenValid 推进。
- **成功审计行**：报名成功（MarkDone scheduler.go:1976）+ 退选成功（RemoveDone:2029）+ 手动/管理员登录（:136/:237）+ 登出（:616）+ 目标保存（:558）+ 删账号（:1055）——全部在位。
- **自动链失败族**：spawnChain 各分支 AgentLog（:1504/:1532/:1558/:1614/:1662 markFull:1770），探测定时触发 maybeRelogin 由重登日志覆盖；MarkDone/RemoveDone 落库失败日志（:1973/:2020/:2026）在位。零吞错 TDD 锚：`TestStoreFailuresLogged`（:86，用 failStore 断言 SaveSuccess/SaveRefused 落库失败必须输出日志）、`TestSetTargetsDeleteRefusedFailureLogged`（:115）。
- **零吞错穷举（`_ =`/`_, _ =`/直接赋值忽略形态）**：全后端产品代码 `grep "_ = "` 命中仅 6 处且全部为安全形态：handler.go:387/`_ = d.Sched.MarkDone`、:467/`_ = d.Sched.RemoveDone`（内存态，其内部已处理落库错误并记日志，非吞错）；config.go:96/141/160（os.Setenv/MkdirAll/WriteFile 写 .env 模板的已处理形态）；scheduler.go:307/`_ = pw.Prewarm()`（预热失败静默，节奏无关）。**全部落库点（SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefused/UpdateIDToken/SetTargetsForAccount/AppendLog）零 `_ =` 命中**。
- **网络层 token 脱敏延续抽查**：sanitizeError（zhidao/client.go:581-593）剥 `*url.Error` 完整请求 URL 文本（`?idToken=` 即认证通道），`sanitizerErr` Unwrap 下沉底层错误（:566-572）保 isConnErrRetryable/IsReadErr 判型穿透（sanitize_test.go 断言"脱敏后判定同原始"，`TestSanitizeErrorPreservesJudgment` :42）；scheduler.go:1283 重登成功日志走 maskedToken。captcha.go:162 识别请求也共用 httpDo（无 token 静态 URL 不需 sanitize，切片语义正确）。

### 4. O105-01 抖动基线 —— ✅ 在位

- `socketPreheat`：zhidao/client_test.go:25（net.Listen 127.0.0.1:0 后 Close），包级 TestMain 预加热（:39）+ 逐测试双保险（:40/:52）+ captcha_test.go:17/:52；api/handler_test.go:67 套接字预创建 + :139 readyProbe。
- `readyProbe`：zhidao/client_test.go:88（200ms×10 + 2s 超时）、api/handler_test.go:182、accounts/manager_test.go:26。
- **定向 race 实测**（借 /d/mingw64/bin/gcc）：zhidao/accounts/scheduler/api 四包强制非缓存全绿（见验证表），全量十包 `-race -count=1` 也绿。身份防线族十五测（6 个删号同名重建分支 + TestDeletedAccountInFlightDropsSuccess + TestDeletedAccountReloginSuccessDropsState + TestUnauthorizedBranchDeletedAccountSkipsState + TestRealtimeRecheckDeletedAccountDropsLog + TestMaybeReloginDeletedAccountSkipsMaps + TestRealtimeFullRecheckKeepsManualSuccess + TestRealtimeRecheckUnauthorizedTriggersRelogin + TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin + TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight）定向 `-race -count=1` 1.913s 绿。
- 回归锚双绿（含显式非缓存单跑）：`TestWindowOpenSubmitsWithoutProbeReset` + `TestAdminStatsWindowOpenedUsesScheduler`。

### 5. LOW-132/133 回首核 —— ✅ 通过

- `git show df1f5ce` 白线：基线提交仅 R149 三文档（review-round149.md + 双 findings），产品代码零改动。
- 时间基全量扫 `time.Since()/time.Now()` 产品代码 + 排除注释后，残余仅两处"写读同基自洽"，与契约断言一致：
  - `reloginAt`（scheduler.go:1220/:1227/:1231/:1261）写 `time.Now()`、读 `time.Since(t)`——同本地钟自洽（relogin 防抖与时钟对齐无关）。
  - `gateWindow`/`gateUsed`（manager.go:53/:71/:74/:226/:227）写读同本地钟——自洽。
  - 其余调度器时间判定（`now.Sub(lastSubmit)` :1032 / `probeIntervalFor` / `windowClosedLocked` / `isRateLimitedLocked` 退避 :1714 / `acctDataAt` TTL :795/:801/`lastDataAt` :881）全部对齐钟；`lastSyncFailAt`/`syncFailedWindow` 失败落地即 `nowAlignedLocked()`（:372/:377），与判读侧 :342 同基准。SyncServerTime（client.go:113/:135/:137）本地钟测 RTT、减法得 offset，属测量基准本身不可对齐，合规。handler.go:1176 loginLimiter 令牌桶自用独立本地钟自洽。session/store.go 票据/会话到期用本地钟，与登录限流同域，合规。

## 新契约角度纵深（自选 ×2）

选择理由：这两处是本轮聚焦清单以外两条安全敏感链路的最新复核面——登录时序攻击族（B42-01 闸门双侧收口 + B43-04 撞名双条件）是认证面最后一道防线，删号 memory-first 四序是身份防线矩阵"写回侧复核"的最前置源头。二者均有多轮沉淀、且此前的审查只断"在位"未断"族内互操作"。

### 角度一：登录闸门族双侧收口（gateWait 阻塞 + gateTryAcquire 非阻塞）与手工登录网络往返链

- **双侧共享同一 gateMu/gateUsed 计数**（manager.go:39-42）：gateWait（:49-66）：每次 `time.Now()` 检查窗口，窗口内 quota 满则 `gateCond.Wait()` 阻塞等 GatePump 广播（main.go:155 每 30s 调一次，:71-77 窗口到点广播）；gateTryAcquire（:223-235）：非阻塞申请一次预算，quota 满立即返回 false 不上平台（LoginByPassword:244 `if !m.gateTryAcquire() { return "登录尝试过于频繁..." }`）。
- **双侧时间基一致性**：gateWait/gateTryAcquire/GatePump 三处均写 `time.Now()` + 读 `time.Since(m.gateWindow)`，同一本地钟，一分钟窗口语义一致。
- **手工登录网络往返链**（LoginByPassword:243-272）：非阻塞准入 → `ensure(acct)`（:102-112 注册表不存在则新建 `zhidao.New` 并注册→ order 首位）→ wasShell 判别（`c.Token()==""` 快照，判"纯新建空壳"）→ `c.Login(acct, password)` → 失败分支只摘除"本次新建的空壳"（wasShell true 才 `delete(clients, acct)` + 顺序移除，:264-272），不误删已持有效 token 的既有工作客户端。成功分支 `c.SetCredentials(account, password, token)` 写内部账密（登录后自动重登可用），`encrypt(password)` 加 `enc:` 前缀落库（SaveCredential），加密失败记日志不落库（自动重登将无保存账密，日志唯一审计线索）。
- **fulfill 时序攻击族链**：管理员双条件（B43-04，handler.go:131 `Account == adminName && ConstantTimeCompare(Password, AdminToken)==1`）→ 正确口令等时 `loginTimingFlat` 延迟；撞名学生走 `LoginByPassword` 教务登录（mock 全成功 → 未激活颁发票据 / 激活后普通会话，绝不带管理权限）；管理员名+口令错 → 教务登录失败 → 归因"管理口令错误" + 等时延迟。TDD 锚：`TestLoginByPasswordRejectsWhenGateBudgetExhausted`（:174，quota 满拒绝且 doLogin 0 次）、`TestLoginByPasswordAllowedWhenGateBudgetAvailable`（:198，quota 充足放行且消耗 1 次预算）、`TestLoginFailKeepsExistingValidClient`/`TestLoginFailRemovesFreshShell`（:108/:126）、`TestLoginAdminNameCollisionStudentCredential`（:1515，撞名学生签发纯普通会话）、`TestLoginAdminWrongPasswordTimingFlat`（:1493）。
- **裁决：gateTryAcquire 与 gateWait 共享同一 gateMu/gateUsed 计数、同一窗口复位逻辑，双侧收口无旁路；登录失败空壳清理"判别新建 vs 既有"语义正确，绝不误删既有客户端。B42-01/B43-04 族内互操作无漂移。**

### 角度二：删号 memory-first 四序 + 重启恢复序（RestoreTargets 不清 refused）闭环

- **四序**（handler.go:1038-1064）：`Accounts.Remove`（摘注册表，在飞链复核立即失败）→ `Sched.PurgeAccount`（调度器内存全量清理：acctTargets/done/full/rateLimited/inflight/refused/acctData/acctDataAt/openTimeDetected/tokenValid/reloginAt/reloginFail/relogging + 该账号课程状态行，:495-522）→ `Store.DeleteAccount`（事务 6 表清空：credentials/accounts/targets/success/refused/activations，:388-408）→ `Sessions.RevokeAccount`（吊销全部会话）。DeleteAccount 失败时注册表已摘、库行未清（半删态），由重启 Restore 重建客户端自愈。TDD 锚：`TestAdminDeleteAccountMemoryFirst`（:2180，断言内存注册表先失效 + 凭据表清空）`TestAdminDeleteAccountNoBodyOK`（:1933）`TestAdminDeleteRejectsUnnormalizedAccount`（:2103，空格名整体拒绝绝无 trim 后误删）`TestAdminDeleteProtectsRenamedAdmin`（:803，改名管理员同样受删除保护）。
- **重启恢复序（main.go:115-141）**：`RestoreDone(success)` → 逐账号 `LoadTargetsForAccount` + `RestoreTargets`（:525-532，与 SetTargetsForAccount 唯一区别是不清 refused，且兜底 `enrichTargetPubMetaLocked` 补全发布元数据）→ `LoadRefused` + `RestoreRefused`（:626-639，注入 refused 时对命中 pending 的状态文案对齐手动退选语义）。`SetTargetsForAccount` 只在用户主动重设目标时调用且只清 refused 绝不清 done/full/rateLimited/inflight（:459-463 注释定案，`TestRestoreDoneSkipsResubmit` :567 固化"已成功课程绝不重复提交"）。refused 优先于 done（恢复序 refused 在 targets 之后注入，RestoreRefused 的防御注入在 courses pending 文案对齐）。
- **与身份防线的互操作点**：四序的 Remove 前置 + PurgeAccount 让"落库前锁内复核 ClientFor/sameClientFor"防线族从一开始即生效（:1038-1043 注释）；同名重建（重建身份 perAccount 新指针）后 in-flight 旧链全部被 sameClientFor 拦截——与角度一的手工登录 ensure 语义互洽（换绑走 LoginByPassword，ensure 拿到的 rebuilt 客户端即新身份）。
- **裁决：四序与恢复序闭环完整，PurgeAccount 清集清单无遗漏（openTimeDetected 随账号清理，绝不残留旧批次识别值），refused 优先语义与 sameClientFor 身份防线无冲突。**

## 维持观察项

1. accounts 包 readyProbe 无 socketPreheat 双保险（见结论前置 LOW-1）。
2. `ProbeForAccount` 刻意不回写全局 lastProbe（见结论前置 LOW-2）。
3. 实时复核 `classFullRealtime` 防御路径未触发（见结论前置 LOW-3）。
4. `syncFailedWindow`/`syncFailStreak` 写而不读留档字段（注释自述，供未来时间差调参）——维持观察。

## 结尾

聚焦清单六项全部 ✅ 在位，定向 race 四包（含强制非缓存）全绿，身份防线矩阵第六十五轮闭合成立（sameClientFor 7 调用点终局逐一追到"写什么"、双侧 maybeRelogin、手动五路 accountExists、写点换类 5 类全持锁、warnedNoTargets 唯一性、*Locked 族双向射证），OBSERVE-117-01 知识位第三十三轮在位（二次 ClientFor 重取当前 token 落库），B110-01 审计链第四十轮零漂移（6 失败位 + 成功行 + 零吞错穷举），O105-01 实测绿，LOW-132/133 回首核通过（残余仅 reloginAt/gateWindow 两处写读同基自洽），新契约纵深两角度（登录闸门族双侧收口 / 删号四序恢复闭环）无漂移。

**建议：APPROVE**