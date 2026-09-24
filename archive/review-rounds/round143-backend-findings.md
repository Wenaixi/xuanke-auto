# round143 后端只读审查 findings（身份防线矩阵第五十八轮复核）

- 审查基点：`654de67`（R142 归档，身份防线矩阵第五十七轮闭合）
- 模式：绝对只读（唯一写文件 = 本报告）
- 实测窗口：本机 Windows 11 + mingw64 gcc（`/d/mingw64/bin`，gcc 16.1.0），`go test -race` 定向跑
- 结论前置：**无 CRITICAL / 无 HIGH / 无 MEDIUM / 无 LOW 级缺陷，建议 APPROVE**

## 验证表（实测时间与数据）

| 项目 | 命令 | 实测结果 |
|------|------|---------|
| 构建 | `go build ./...` | exit 0 |
| 静态检查 | `go vet ./...` | exit 0 |
| race 定向四包（zhidao/accounts/api/scheduler 全量） | `go test -race -count=1 ./internal/{zhidao,accounts,api,scheduler}/` | zhidao ok，5.048s；accounts ok，1.419s（全量，含闸门四测）；api ok，16.551s；scheduler ok，15.115s |
| race 附加包（store/session/secure/runtime/config/db） | `go test -race -count=1 ./internal/{store,session,secure,runtime,config,db}/` | 全 ok（21.429s / 2.354s / 4.849s / 2.935s / 3.740s / 6.948s） |
| 回归锚二测（race） | `-run "TestWindowOpenSubmitsWithoutProbeReset\|TestAdminStatsWindowOpenedUsesScheduler"` | scheduler ok 9.585s + api ok 9.416s（与身份防线同批） |
| 身份防线族七测（race） | `-run "TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}\|...RealtimeUnauthorizedDropsRelogin\|...SuccessDropsInflight"` | scheduler ok，1.972s |
| 身份防线族附加（race） | `-run "TestProbeDeletedThenRebuiltSameNameDropsSnapshot\|TestProbeChainSameClientIdentity\|TestDeletedAccountReloginSuccessDropsState\|TestMaybeReloginDeletedAccountSkipsMaps\|TestRealtimeRecheckDeletedAccountDropsLog\|TestUnauthorizedBranchDeletedAccountSkipsState\|TestDeletedAccountManualInFlightDropsState"` | scheduler ok，1.371s |
| 登录闸门四测（race） | `-run "TestLoginByPassword{RejectsWhenGateBudgetExhausted,AllowedWhenGateBudgetAvailable}\|TestLoginFailKeepsExistingValidClient\|TestLoginFailRemovesFreshShell"` | accounts ok，1.329s |
| 时间基/恢复语义（race） | `-run "TestSnapshotTTLUsesAlignedClock\|TestPurgeAccount\|TestRestoreDoneSkipsResubmit\|TestSetTargetsPurgesStaleState"` | scheduler ok，1.454s |
| 契约轮次标签扫描 | grep `第 [0-9]+ 轮\|R[0-9]{2,3} 轮\|round [0-9]+\|R[0-9]{2,3}-` 于 internal/ 产品代码（排除 *_test.go） | 零命中；测试叙述中的合法引用（scheduler_test.go 四处 "第 N 轮"轮次形态、Bxx-xx 标识）属于前后端共用历史锚，合规非产品代码 |
| 工作区漂移 | `git status --porcelain` | 仅 `archive/review-rounds/round143-frontend-findings.md`（前端代理并行产出，非本代理写入）；后端零修改 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第五十八轮闭合 — 判定：在位（零漂移）

**sameClientFor 定义与 7 处调用点逐一核对**（scheduler.go）：

- 定义 :204-210：`ClientFor(acct)` 存在性 + `clientIdentity(:215-224)` 反射解接口动态类型指针、nil/非指针恒 0。调用点清单 :850 / :1489 / :1521 / :1551 / :1571 / :1600 / :1635，与清单行号逐一匹配，零漂移。
- :850 `ProbeForAccount` 回写段（M88-01）：网络往返（FindElectives 最长 15s）后锁内复核身份，非同一身份整体放弃 acctData/acctDataAt/openTimeDetected 回写（:850-854，`return data, nil`）。对应测试 `TestProbeDeletedThenRebuiltSameNameDropsSnapshot`（probe_identity_test.go:16）+ `TestProbeChainSameClientIdentity`（:176）。
- :1489 失效分支（ErrUnauthorized）：**先身份复核再 maybeRelogin**（:1499）——已删/同名重建旧链绝不触发重登（不污染同名重建账号 reloginFail 首登退避）；清 inflight → setStateLocked("failed") → AppendLog。对应 `TestDeletedAccountRebuiltSameNameChainDropsRelogin`。
- :1521 成功分支：身份复核后才写 done + setStateLocked("success") + AppendLog + SaveSuccess（库行终局 success 行）。对应 `TestDeletedAccountRebuiltSameNameChainDropsSuccess` + `...SuccessDropsInflight`。
- :1551 风控退避分支：身份复核后才 markRateLimitedLocked + setStateLocked + AppendLog。对应 `...DropsRateLimitBackoff`。
- :1571 窗口关闭分支：身份复核后才 markFullLocked。对应 `...DropsWindowClosedFull`。
- :1600 实时复核回锁后三路入口统一复核（在 doneHas 判满之前执行），:1635 确证满员分支二次复核。对应 `...DropsRealtimeRecheckFull` + `TestRealtimeRecheckDeletedAccountDropsLog` + `TestRealtimeRecheckUnauthorizedTriggersRelogin`（原始路径）+ `...RealtimeUnauthorizedDropsRelogin`。

每条调用点"写什么"终局追踪完毕：非同一身份一律静默 return 或 `continue`，不写内存态、不落库行、不写日志。所有绕行路径（:1494-1508 失效分支重登归账、:1608-1619 实时复核 ErrUnauthorized 分支）都在身份复核通过后才触发 maybeRelogin。

**maybeRelogin 双侧**（:1198-1299）：
- 决策侧 :1208 `ClientFor` 存在性复核，已删账号不发起重登、不写任何 relogin 族 map（tokenValid/reloginFail/reloginAt/relogging 四组写点全部在其后方 :1212-1241）；锁序 reloginMu→s.mu 对齐。对应 `TestMaybeReloginDeletedAccountSkipsMaps`（契约：四组 map 全不写）。
- 写回侧 :1254 二次 `ClientFor` 存在性复核（成功分支整体放弃内存写与库写，只清 relogging）；:1265-1273 落库前再取 `ClientFor` 的当前 client.Token() 落库（OBSERVE-117-01 第二十六轮，见下），新 token 以 maskedToken 只显前 8 位日志记录。对应 `TestDeletedAccountReloginSuccessDropsState`。

**手动五路 accountExists**（handler.go）：
- :255 课程读；:305 手动报名；:397 手动退选；:497-512 目标写（内联循环 LoadCredentials 比对，与 :1109 `accountExists` 同判据）；:573 状态读。判据同源（凭据表 = "确实登录过"的更强真理源），查无此账号整体拒绝"账号不存在"，绝无对全局帧假成功路径。另核 ：1028 删账号入口 `acct != req.Account` 空格失手判空、:1026 admin 名保护，与常规教务会话放行（B43-04）两路判据分离。

**写点换类 5 类**：
- lastSubmit：仅 :1346 `submitAll` 开头（s.mu 内 nowAlignedLocked 写入），调用点唯一（:1035 tick）→ 宿主唯一性射证。
- lastSyncStart / syncing：:355-356（maybeSyncClock 决策段 s.mu 内）+ :369/:404-405（goroutine 回写 s.mu 内），全部持锁；syncFailStreak 全持 s.mu（:371/:384/:391）。
- lastProbe：:679（Start 主循环 reloginResults 通道 s.mu 内）、:964（ProbeNow）、:1089/:1114（probe，s.mu 内），全部持锁；reLogAfter:1089 失败同样计入节流。
- EmptyProbeRuns：:1156/:1158（probe，s.mu 内，判据"已过开窗点 10s 裕量"入账侧），读侧 windowClosedLocked :935 同锁。
- 无锁写点 warnedNoTargets：:1371-1372 由 `tick():1035 → submitAll():1368-1374` 唯一路径触发，宿主唯一性射证成立（tick 对 goroutine 串行，无并发写）；读侧仅 log 判断，无竞态窗口。

**\*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证**：
- Locked 族清单：`nowAlignedLocked/openTimeForLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/rebuildCoursesLocked/tokenValidForLocked/windowClosedLocked/isRateLimitedLocked/markRateLimitedLocked/markFullLocked/releaseFullIfFreedLocked/statusIndexLocked/setStateLocked`（13 个），逐一确认在 s.mu 持有下被调用（内部不再取锁，无嵌套死锁；nowAlignedLocked 被 maybeSyncClock 等持锁调用点复用）。
- 外部写函数 `MarkDone/RemoveDone/RemoveFull/TryAcquireSubmit/SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkTokenValid/TokenValidFor` 首行即 `s.mu.Lock()/defer Unlock()`（MarkTokenValid/TokenValidFor 额外先取 reloginMu），双向射证无缺口。另核 `CheckClassSelectable` 读取路径同样持锁。

### 2. OBSERVE-117-01 知识位第二十六轮 — 判定：在位

写回侧 :1254 先 ClientFor 存在性复核 → :1265 再次 `ClientFor` 重取当前注册表 client → :1266 `client.Token()` 非空才落库（:1269-1273 UpdateIDToken，失败即 log 留痕绝吞错）。同名重建场景：决策侧/写回侧/落库前 Token 重取三处全部基于"当前注册表"而非发起时客户端快照，绝不串旧身份。

### 3. B110-01 审计链第三十三轮 — 判定：在位（零漂移）

- 手动 6 失败位 AppendLog：:362（报名失效）、:374（报名 read）、:381（报名普通失败）、:443（退选失效）、:452（退选 read）、:459（退选普通失败）——全部 `if err := ...; err != nil { log.Printf }`。
- 成功审计行：:388 MarkDone 内 AppendLog success（含 MarkDone 自身 :1976）；:558 set_targets；:1055 delete_account；:616 logout；:136/:237 login；:864 config——全链路。
- 自动链失败族：spawnChain 六分支每分支失败均 AppendLog（:1504 失效 / :1532 成功 / :1558 风控 / :1614 实时复核失效 / :1662 普通失败 + markFullLocked 内 :1770 满员），全部 `if err != nil { log }` 绝不 `_ =`。
- 零吞错穷举：全产品 Go 文件 `_ =`/`_, _ =`/直接赋值忽略形态仅 6 处，逐一为合规豁免——scheduler.go:307 `_ = pw.Prewarm()`（goroutine 内预热广播，失败无消费方）；scheduler.go:1075 `_, _ = s.ProbeForAccount(acct)`（probe 每账号探测结果仅作快照刷新，错误在下游统一处理，加注释明确契约）；handler.go:387/467 `_ = MarkDone/RemoveDone`（内部已记录错误日志，返回 nil 恒）；client.go:107/124 `_, _ = io.Copy(io.Discard, resp.Body)`（响应体排空，读失败无审计价值）。handler_test.go:582 等 `_ =` 全在测试断言路径，非产品代码。
- handler.go 该轮新增器物：全部 accountExists / AppendLog 补写点都带错误留痕，无新吞错形态。
- 网络层 token 脱敏延续抽查：zhidao/client.go :563-593 sanitizeError/sanitizerErr（剥完整 URL、Unwrap 下沉底层错误判型穿透）、scheduler.go :1283 maskedToken + :1332-1339（前 8 位）。doRequest :450 统一走 sanitizeError——所有 `?idToken=` 通道错误 %s 格式化上抛前必先脱敏，消费侧（scheduler/api 的 `err.Error()`/`logMsg := ...err.Error()`）不自行透传 url.Error 原文。

### 4. O105-01 抖动基线 — 判定：在位（实测绿）

- socketPreheat（zhidao/client_test.go:25-30）+ TestMain 包级预热（:39-42）；api 包 handler_test.go readyProbe（:179-185，5 次轮询/2s 超时）+ 套接字预创建；accounts 包 readyProbe（manager_test.go:26，三处调用）；sanitize_test/captcha_test 均有 socketPreheat——夹具全部在位。
- 定向 race 实测：zhidao/accounts/api/scheduler 四包全量全绿（见验证表），且有附加六包，远超"至少四包"要求。
- 回归锚 `TestWindowOpenSubmitsWithoutProbeReset`（scheduler_test.go:1277）+ `TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go:1011，断言 stats.window_closed 与 `d.Sched.WindowClosed()` 同源 + window_opened 与 `WindowOpened()` 同源）均绿。
- gcc 环境确认：`/d/mingw64/bin/gcc.exe`（MinGW-W64 x86_64-ucrt-posix-seh r1）在位，race 构建经 `export PATH=/d/mingw64/bin:$PATH` 可用。

### 5. LOW-132/133 回首核 — 判定：通过

- `git show 654de67` 白线核对：基准提交仅 3 个 review 文档文件（无代码文件），工作区后端零漂移确认。
- time.Since/time.Now() 全量扫（scheduler.go 产品代码，剔除注释与测试文件）残余写点穷举：
  - :269/:274 `nowAligned()`（time.Now()+clockOffset，对齐钟基自洽）；:1220/:1227/:1231/:1261 `reloginAt` 读写同基（本地钟）——relogin 频率节流独立自洽，退避/节流判期不涉窗口点，合规；:1707/:1714 rateLimited 族已收敛至 nowAlignedLocked 写入、isRateLimitedLocked 读侧同为对齐钟。
  - accounts/manager.go :53/:71/:74/:226/:227 全部为 `gateWindow` 节奏（gateWait/gateTryAcquire/GatePump 三处写读同基本地钟）——登录频率闸门独立语义，合规。
  - zhidao/client.go :113/:135/:137 SyncServerTime 中点估算（刻意独立时间基）；:328/:355 captcha/uniqueId 请求元数据。
  - api/handler.go :1176 loginLimiter 令牌桶（独立语义）。
  - 残余仅 `reloginAt` 与 `gateWindow` 两族，均为"写读同基自洽"，无新混用孤岛；LOW-133-01 快照 TTL 判读侧四处 time.Since→nowAlignedLocked().Sub 已在基线内（git show 白线确认），`TestSnapshotTTLUsesAlignedClock`（scheduler_test.go:1516）race 绿。

### 6. 新契约角度纵深（自选 ×2）

**角度 A：登录闸门族 B42-01 双侧收口（gateWait 阻塞 + gateTryAcquire 非阻塞共享计数）**
- accounts/manager.go 结构：gateMu/gateWindow/gateUsed :40-43；gateWait :49-64（阻塞语义，for+Cond.Wait，窗口滚到 quota 满等广播）；GatePump :68-77（main 每 30s 广播）；gateTryAcquire :223-235（非阻塞，窗口过期重置，quota 满返 false）；Relogin :166-175 走 gateWait；LoginByPassword :243-246 先 gateTryAcquire 非阻塞，quota 满立即返错"登录尝试过于频繁"，绝不阻塞用户响应。共享同一 gateUsed 计数，手动登录与排队重登严格同预算。
- 实测：`TestLoginByPasswordRejectsWhenGateBudgetExhausted`（quota 满 → 拒绝 + doLogin 0 次 + 不注册空壳）+ `TestLoginByPasswordAllowedWhenGateBudgetAvailable`（旧窗口跨分钟重置分支放行 + 恰好 1 次 doLogin + gateUsed 恰 1）race 双绿。`ResetGateForTest`（:82-87）为测试专用、正式代码不调用，注释明确。
- 失效保护完整性：Relogin 的 `c.ReloginIfNeeded()`（client.go :602-613）在账号未登录无保存账密时返回明确错误，scheduler maybeRelogin 写回侧对数处理失败分支（:1293-1297 区分"重登失败/无保存账密"文案）。闸门内层 reloginFail 指数退避（backoffMin<<n 封顶 10min，:1172-1176）+ 外层每分钟 2 次双保险，平台锁号防线完整。
- 决策锚 40 续验：`TestReloginBackoffCappedAndReset` / `TestReloginFailureKeepsBackoff` / `TestReloginBackoffWindowBlocksManualTriggers`（scheduler 全量归入）均在位。

**角度 B：手动 4 方法协同族（TryAcquireSubmit/MarkDone/RemoveDone/RemoveFull）终局核对**
- TryAcquireSubmit :1896-1914：s.mu 内 inflight 位互斥，`sync.Once` 包裹 release 保证幂等释放；在飞冲突返 false 由 api 层报"该课程正在提交中"。
- MarkDone :1922-1982：入口 `ClientFor` 存在性复核（已删放弃写回）→ done 置位 → **清 refused（含库行 DeleteRefusedClass，删失败 log）** → 清 inflight/full/rateLimited → setStateLocked success → SaveSuccess + AppendLog（均 log 留痕）。语义：手动重选成功 = 自动引擎恢复接管。
- RemoveDone :1989-2035：入口复核同款 → done 移除 → 清 inflight/full → refused 置位 → DeleteSuccess + SaveRefused + AppendLog。语义：手动退选 = 自动引擎绝不抢回，重启后 RestoreRefused 维持。
- RemoveFull :2038-2049：s.mu 内清 full 置 pending。**维持观察**：grep 全仓库产品代码无调用方（仅定义 + 测试），属"外部预留 API"——现有解除满员的实际路径是 scheduler 内部 `releaseFullIfFreedLocked`（快照判余量解封），RemoveFull 独立路径不影响当前实现正确性。
- 4 方法对应的 api 层编排（handler.go :328-467）：Select 先 TryAcquireSubmit → CheckClassSelectable（窗口/满员复核）→ ClientFor → SelectClass → ErrUnauthorized 触发 MaybeRelogin；Exit 对称。在飞幂等由 inflight 位 + sync.Once + api 层在飞守卫三处覆盖。
- spawnChain 对 4 方法的协同：:1464 inflightHas 跳过手动在飞、:1627 doneHas 让位手动成功、:1645 doneHas 不覆盖胜利状态、TryAcquireSubmit 占用位让自动链让行——双向协同闭环。

## 维持观察项（无阻断）

1. `RemoveFull`（scheduler.go:2038）全仓库产品代码零调用方，属"外部预留 API"；当前满员解除实际走内部 `releaseFullIfFreedLocked`。不构成缺陷，只是死代码面提示，未来可在清理契约时一并裁决。
2. `AccountsWithTargets()` 排序输出（:755 sort.Strings）是 api 层"核心账号"选择依赖，若未来账号名含不可比较序的 Unicode 需复核稳定性——当前全数字学号场景无影响。
3. Prewarm goroutine 失败静默（`_ = pw.Prewarm()`）是刻意取舍（预热失败不阻塞任何路径），若未来要审计预热成功率可加通道留痕——非本轮缺陷。

## 建议

**APPROVE**。聚焦清单六项全部判定为在位且实测绿：身份防线矩阵第五十八轮闭合（sameClientFor 定义 + 7 调用点逐一追到写状态/落库终局、maybeRelogin 双侧 + 二次 Token 重取、手动五路 accountExists 判据同源、写点换类持锁/唯一性 + warnedNoTargets 宿主唯一射证、Locked 族 13 个双向射证）；OBSERVE-117-01 知识位第二十六轮 / B110-01 审计链第三十三轮零漂移 / O105-01 四包以上 race 全绿 / LOW-132/133 时间基回首核通过（残余仅 reloginAt/gateWindow 两族写读同基自洽）。构建双绿、十个身份防线族测试全绿（waitChainExit 等待契约杜绝假绿）、回归锚钉守绿、登录闸门族双向实测绿、工作区后端零漂移。无 CRITICAL/HIGH/MEDIUM/LOW 级发现。