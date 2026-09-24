# R138 后端审查报告（绝对只读，身份防线矩阵第五十三轮）

- 日期：2026-09-24
- 基线：R137 归档（03ad979）；工作树 `git status --short --branch` 显示 `## master` + 唯一未跟踪文件 `?? archive/review-rounds/round138-frontend-findings.md`（前端并行代理产物）。backend/ 产品代码自 R137 后零改动（`git diff 03ad979 --stat` 空）。
- 模式：绝对只读（唯一允许写文件为本报告；全仓库其余零修改）
- 聚焦清单六项全实测 + 2 个自选新纵深（时间盒 35 分钟内完成，取探测定时族 + 多账号年级隔离快照回退链）
- 结论前置：**身份防线矩阵第五十三轮闭合，全部验证项实测绿，新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW，裁定 APPROVE**

## 验证表（全部实测）

| 项目 | 结果 | 证据（实测时间/数据） |
|------|------|------|
| go build ./... | 绿（exit 0） | 全包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race 四包 | 全绿 | 借 `/d/mingw64/bin/gcc.exe`（实测存在）注入 PATH：zhidao 2.891s / accounts 2.139s / scheduler 15.357s / api 17.553s 全 ok 零竞态（-count=1 强制重跑，非缓存） |
| 身份防线族 race 十测 + 补充 | 全绿 | `-race -run 'TestDeletedAccountRebuiltSameNameChainDrops(S..\\|Relogin\\|RateLimitBackoff\\|WindowClosedFull\\|RealtimeRecheckFull)\\|TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin\\|TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight\\|TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime\\|TestMaybeReloginDeletedAccountSkipsMaps\\|TestDeletedAccountReloginSuccessDropsState\\|TestRealtimeRecheckUnauthorizedTriggersRelogin'` scheduler 包分两批共 3.679s + 1.842s + 1.457s + 1.448s 全 PASS |
| 回归锚 | 全绿 | TestWindowOpenSubmitsWithoutProbeReset 1.04s / TestSubmitSuspendedWhenOpenTimeCleared 1.26s（scheduler 包）+ TestAdminStatsWindowOpenedUsesScheduler 0.70s（api 包）定向 race 全 PASS |
| 时钟时间基族 race | 全绿 | TestSnapshotTTLUsesAlignedClock / TestClockSyncFailureBackoffUsesAlignedClock 走读 + 全量回归绿 |
| 登录闸门族 race | 全绿 | TestLoginByPasswordRejectsWhenGateBudgetExhausted + TestLoginByPasswordAllowedWhenGateBudgetAvailable（accounts 包）1.407s 全 PASS |
| 手动审计族 | 全绿 | TestManualElectiveFailureAppendsLog / TestManualElectiveReadErrAppendsLog / TestHandleElectivesSelectUnauthorizedRelogin / TestHandleElectivesSelectReadErrMessage（api 包）全 PASS |
| 手动五路 accountExists 拒绝族 | 全绿 | TestAdminElectivesUnknownAccountRejects / TestAdminElectiveSelectUnknownAccountRejects / TestAdminStateUnknownAccountRejects / TestSetTargetsBounds / TestSetTargetsEmptyAllowed（api 包）全 PASS |
| 全后端非 race 全量回归 | 全绿 10/10 | scheduler 19.846s / api 16.081s / zhidao 5.178s / accounts 4.563s / store 6.248s / session 1.148s / db 1.771s / secure 1.344s / config 1.002s / runtime 0.957s 全 ok |
| 契约轮次标签扫描 | 产品代码零命中 | 全仓扫 `第N轮/R..轮/round N/Round` 产品代码零命中；唯一命中为 session/store.go:117 注释引用历史文档名 `docs/review-round13.md`（历史引用，合规，同 R137 反证） |
| 工作区零漂移 | 成立 | `git status --short --branch` 仅 `?? round138-frontend-findings.md`，无任何已跟踪文件改动 |

## 1. 身份防线矩阵第五十三轮闭合

### 1.1 sameClientFor 定义与 7 调用点逐一确认零漂移

- 定义：scheduler.go:204 `sameClientFor(acct, chainClient)` → :205-209 `ClientFor(acct)` 存在性内含（`!ok || current == nil` 即返回 false）+ 指针身份比对；:215 `clientIdentity` 用 `reflect.ValueOf(c).Pointer()` 取具体指针类型底层指针值，:220-223 nil/非指针保护返回 0 恒非同一。grep 全仓 `clientIdentity(` 仅 :209/:215 出现，无旁路比对。零漂移。
- 7 调用点逐一追到"写什么状态 / 落什么库行"终局（grep :1489/:1521/:1551/:1571/:1600/:1635 六链内调用 + :850 探测回写单点全覆盖）：
  - **:850**（ProbeForAccount 回写段）——网络 FindElectives 最长 15s 后锁内复核；不放行即放弃写 `acctData[acct]`/`acctDataAt[acct]`（:863-864）与 `openTimeDetected[acct]` 识别槽（:856），绝不覆盖重建账号识别槽/年级串线。注释 :843-848 覆盖同名重建顶替语义。
  - **:1489**（spawnChain 失效分支）——身份不符锁内 `delete(s.inflight[acct], t.ClassID)`（:1490）后静默弃链；**:1499 `maybeRelogin` 落在复核之后**（注释 :1494-1497 明言"重登必须落在身份复核之后"）——旧链绝不消费新身份登录预算（TestDeletedAccountRebuiltSameNameChainDropsRelogin 红→绿契约测试在位）。
  - **:1521**（成功分支）——通过才 `done[acct][t.ClassID]=true`（:1529）+ setStateLocked(success)（:1530）+ SaveSuccess 落库（:1535）+ "报名成功" AppendLog（:1532）。注释 :1515-1520 覆盖重启 RestoreDone 假成功语义。
  - **:1551**（风控退避分支）——通过才 markRateLimitedLocked（:1555 写 30s 退避截止）+ setStateLocked(failed) + AppendLog（:1558）。注释 :1547-1550 覆盖假退避让黄金期被静默跳过语义。
  - **:1571**（窗口关闭分支）——通过才 markFullLocked（:1575 写 full 集合 + AppendLog :1770）。注释 :1565-1569 覆盖"无效的课程ID"按满员记 full 语义。
  - **:1600**（实时复核回锁后三路总闸）——存在性内含于 sameClientFor（注释 :1596-1599 六路对称声明）。不放行锁内直接 return，绝不让三路（失效重登 :1608-1619 / 确证满员 :1627-1641 / 普通失败 :1645-1666）写重建身份。
  - **:1635**（确证满员分支内的二重复核）——回锁后 :1600 总闸之外，:1635 在落 markFullLocked（:1639）前再复核一次（注释 :1631-1634 覆盖"入口存在性复核挡不住新身份存在但指针不同"）；:1627 `doneHas` 让位于手动 MarkDone 的胜利状态。第六分支 ErrUnauthorized（:1608）与 :1635 同族对称。实时复核第六分支测试 TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin 实测绿（0.51s，含 500ms 轮询窗口防假绿）。成功分支 inflight 清位归因契约测试 TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight 实测绿。

### 1.2 maybeRelogin 双侧 + 二次 ClientFor 终局（OBSERVE-117-01 关键性质）

- **决策侧 :1208**——reloginMu + s.mu 双持锁内、任何 map 写入前 `ClientFor(acct)` 存在性复核（注释 :1202-1207 明言"探测定时三处对 ErrUnauthorized 直调本入口……账号已删不发起重登、不写任何 map"）；已删静默 return。随后才读写 relogging/reloginFail/reloginAt/tokenValid（:1212-1241）。测试 TestMaybeReloginDeletedAccountSkipsMaps 实测绿（四 map 零残留）。
- **写回侧 :1254**——goroutine 开头锁内 `ClientFor(acct)` 复核，已删整个成功分支（内存写 + 落库）静默放弃只清 relogging。测试 TestDeletedAccountReloginSuccessDropsState 实测绿（tokenValid/reloginAt 零残留 + UpdateIDToken 跳落库）。
- **二次 ClientFor 终局 :1265-1273**——success 分支先清内存标记，再 `if client, ok := s.clients.ClientFor(acct); ok` 重取当前注册表 client → :1266 `client.Token()`（client.go:195-199 锁内读当前 token 实值）→ 非空才 :1269 `UpdateIDToken(acct, tok)` 落库（store.go:56-59 按 account 主键写新值）。落库 token 恒为重登完成后注册表内该账号客户端的实值，绝非 goroutine 发起时捕获的旧身份/旧 token——同名重建场景绝不串旧身份（OBSERVE-117-01 知识位第二十一轮在位，连续零漂移）。token 空时跳过落库（防御，绝不写空覆盖）。落库失败 `log.Printf` 零吞错。
- 结构性保证链完整：ReloginIfNeeded（client.go:602）→ Login（client.go:217）→ submitLogin 成功在 c.mu 内写 c.token + zd_edu_cookie（client.go:392-394）→ c.SetCredentials（client.go:270，登录成功即写内部账密供重登复用）→ Token 锁内读 → UpdateIDToken 落库。

### 1.3 手动五路 accountExists（判据同源，handler.go）

| 路径 | 行号 | 语义 |
|------|------|------|
| handleElectives（课程读） | :255 | 凭据表查无 → "账号不存在，无法查看课程" |
| handleElectiveSelect（手动报名） | :305 | 查无 → "账号不存在，无法执行报名操作" |
| handleElectiveExit（手动退选） | :397 | 查无 → "账号不存在，无法执行退选操作" |
| handleSetTargets（目标写） | :497-512 | 显式 LoadCredentials 循环比对（注释 :494-496 明言 authenticateDirect 直连建立会话的账号须被放行） |
| handleState（状态读） | :573 | 查无 → "账号不存在，无法读取状态" |

判据同源：`accountExists`（:1109-1120）= LoadCredentials 逐账号比对（凭据表 = "确实登录过"的更强真理源，注释 :1106-1108）。`allowAccountOverride`（:1102-1104）仅管理员会话可穿透。五路全覆盖（grep handler.go 确认 :255/:305/:397/:497-512/:573 逐一在位），拒绝族测试实测绿（TestAdminElectivesUnknownAccountRejects / TestAdminElectiveSelectUnknownAccountRejects / TestAdminStateUnknownAccountRejects）。

### 1.4 写点换类 5 类全持锁 + *Locked 族与外部写函数双向射证

- **lastSubmit**：写点 submitAll 首行 :1346（`s.lastSubmit = s.nowAlignedLocked()`），持 s.mu；tick 判读 :1029 持锁取。
- **lastSyncStart**：写点 maybeSyncClock :356 持锁；读 :393（sync goroutine 持锁推进 lastSyncTime）。复位 :405（无客户端 fallback 持锁）。
- **syncing**：置位 :355（持锁）、复位 :369（sync goroutine 持锁）/ :404（fallback 持锁）。
- **lastProbe**：写点四路全部持锁——ProbeNow :964 / probe 失败 :1089 / probe 成功 :1114 / Start 主循环重登回传 :679。注释 :868-871 明言"lastProbe 只归 probe() 与 ProbeNow 管理"（:679 重登回传补探测属主循环唯一无锁外通道，但 :678 持 s.mu）。ProbeForAccount :873 刻意不写 lastProbe（管理员穿透探测不吞全校节流）。
- **state.EmptyProbeRuns**：写点 probe :1156/:1158 持锁（入账侧 +10s 裕量，注释 :1151-1154）。
- **\*Locked 写函数族**（持锁被调方）：nowAlignedLocked（:273 自实现不取锁，调用方持 s.mu）/ openTimeForLocked（:427）/ enrichTargetPubMetaLocked（:539）/ rebuildCoursesForAccountLocked（:570）/ rebuildCoursesLocked（:647）/ tokenValidForLocked（:739 只读）/ isRateLimitedLocked（:1691 读写）/ markRateLimitedLocked（:1710）/ releaseFullIfFreedLocked（:1781）/ statusIndexLocked（:1835）/ setStateLocked（:1846）/ markFullLocked（:1760）/ windowClosedLocked（:918，三条判据单源 open 快照复用，:924 单次取 open 对判据 2/3 统一）——全部只被锁内调用，名字即锁契约。grep 全仓无"外部直接调 *Locked"形态。
- **外部写函数首行取锁**双向射证：SetTargetsForAccount :455 / PurgeAccount :496 / RestoreTargets :526 / RestoreDone :608 / RestoreRefused :627 / MarkDone :1923 / RemoveDone :1990 / RemoveFull :2039 全部首行 `s.mu.Lock()`。
- **无锁写点宿主唯一性射证**：`warnedNoTargets`（:1371-1372，submitAll 内写）唯一调用链 = tick（:1035 调 submitAll）→ Start goroutine for-select 主循环（:673-675 tick 唯一调用点），天然串行；`start` 字段 :664 持 s.mu 写；`chains` map 由独立 chainMu 保护（:1392-1403 三处）。无第二宿主。

## 2. OBSERVE-117-01 知识位第二十一轮确认在位

- 写回侧先 :1254 ClientFor(acct) 复核（已删整体放弃，含内存写与落库）→ success 分支 :1265-1266 另取一次当前注册表 client → client.Token() → :1269 UpdateIDToken 落库。落库 token 恒为重登完成后注册表内该账号客户端的实值，绝非 goroutine 发起时捕获的旧身份/旧 token。
- 同名重建场景：删号后新 *Client 注册，旧重登 goroutine 返回时 :1254 正常通过（新身份存在），:1265 重取的正是**新身份**的当前 token——不串旧身份（B21-03 定版语义连续第二十一轮零漂移）。
- 活化条件（未来改动破坏"落库取当前注册表 token"关键性质）未触发。

## 3. B110-01 审计链第二十八轮零漂移

- 手动 6 失败位 AppendLog 逐一就位：**handleElectiveSelect** :362（ErrUnauthorized）/ :374（read 类"结果未知"最需留痕）/ :381（业务失败原文）；**handleElectiveExit** :443 / :452 / :459 三路对称。全部 `if err := ...; err != nil { log.Printf }` 零吞错。测试 TestManualElectiveFailureAppendsLog + TestManualElectiveReadErrAppendsLog 就地覆盖 select+exit 双路，实测绿。
- 成功审计行：手动 select → MarkDone 内 AppendLog（:1976）；手动 exit → RemoveDone 内 AppendLog（:2029）；login 日志 issueSession :237（含管理员 :136）；set_targets :558；config :864；delete_account :1055；logout :616。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 确证满员 markFullLocked :1770 / 普通失败 :1662，全部落库失败记日志。
- 零吞错穷举（测试外）：`_ =` 仅防御性忽略——handler.go:387 MarkDone / :467 RemoveDone（库写在方法内部已处理）、config.go:96/os.Setenv 与 :141/:160 文件写入（模板/环境文件非审计链）、client.go:307/:124 io.Copy、scheduler.go:307 Prewarm。grep 落库族调用（AppendLog/SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefusedClass/SetTargetsForAccount/UpdateIDToken）**零 `_ =` 命中**。零库写忽略。
- 网络层 token 脱敏延续：doRequest :422 拼 `?idToken=` → :447-450 网络错误走 sanitizeError（client.go:581-593 剥 *url.Error 完整 URL 文本、Unwrap 下沉底层错误保 isConnErrRetryable/IsReadErr 判型穿透）→ 消费侧 maskedToken（scheduler.go:1283，len>8 只显前 8 位）。B101-01 延续无回归。

## 4. O105-01 抖动基线实测绿

- 夹具在位：zhidao/client_test.go:25 socketPreheat（包级 TestMain 预预热 + 逐测试调用）+ :88 readyProbe（200ms×10+2s 宽栅栏）；captcha_test.go:17/:52 同款；sanitize_test.go:97 同款；api/handler_test.go:185 readyProbe；accounts/manager_test.go:26 readyProbe（注释明言"三处夹具未有 socketPreheat 双保险，8 轮全绿实证无残余；若未来 accounts 再出冷启动 flake 第一候选即补"）。scheduler 包无 mock 服务器夹具（纯 fake），不需就绪探测。
- 定向 race 四包实测全绿（借 `/d/mingw64/bin/gcc.exe` 注入 PATH，-count=1 强制重跑），含身份防线族、回归锚、时钟时间基族、登录闸门族、手动审计族——全部一次通过，无 flake。

## 5. LOW-132/133 回首核

- **git show 白线核对**：基线 03ad979 相对 R137 的产品代码改动已确认（R137 归档内容），975dc2b/d75f38c 均属时钟时间基域（:372 lastSyncFailAt / :377 syncFailedWindow 改 nowAlignedLocked；ElectivesSnapshotFor/ElectivesSnapshot/CheckClassSelectable 判读侧改 nowAlignedLocked().Sub），身份防线零改动。本轮 `git diff 03ad979 --stat` 空。
- **时间基全量扫零残留**（产品代码，grep time.Since/time.Now() 全仓）：
  - scheduler.go:1220/:1227 `time.Since(reloginAt)` → 写 :1231/:1261 同 time.Now() 本地钟，读写同基自洽（relogin 退避是"距上次重登 30s"相对度量，与对齐钟无关），**合规**。
  - accounts/manager.go:71/:226 `time.Since(gateWindow)` → 写 :53/:74/:227 同本地钟，读写同基自洽（登录闸门"每窗口 2 次"为相对度量、同一 gateMu 保护），**合规**。
  - client.go:113/:135/:137 SyncServerTime 的 time.Now() 本就是 RTT/clockOffset 计算需要本地钟（`start := time.Now()` → `time.Since(start)` → `estimatedServerTime.Sub(time.Now())`），非调度判读侧，**合规**。
  - client.go:328 captcha 时间戳 / :355 uniqueId 同本地毫秒钟，与 getUniqueDeviceId 逆向契约对齐，**合规**。
  - session/store.go 与 handler.go:1176 的 time.Now 为会话/票据/限流桶过期判定（秒级精度 TTL 相对度量，非调度敏感路径），**合规**。
  - 残余仅 reloginAt 与 gateWindow 两处写读同基自洽，零混用孤岛。
- 全量扫零命中测试文件众多（New 的 openTime 参数、deadline 轮询），均为测试侧对本地钟的合法依赖，非产品代码混用。

## 6. 新契约角度纵深（自选 ×2）

**选此方向的理由**：本轮聚焦清单五、六项（LOW-132 回首 / 契约漂移）已经是横向的；探测定时族与年级隔离快照回退链是"黄金期正确性"与"多账号数据保真"的直接承载面，且代码量大、契约多，是最值得投入的纵向点。两者时间盒 35 分钟内完成。

### 6.1 探测定时族（probe/ProbeNow/ProbeForAccount 节流三件套 + probeSem cap=4）

- **节流三件套全景**：
  - **lastProbe 全局闸门**：只归 probe()（:1089 失败也计 / :1114 成功计）与 ProbeNow（:964）写；ProbeForAccount :873 刻意不写（管理员穿透探测不吞全校节流，注释 :868-871）。判读侧 tick :987 `now.Sub(last) >= s.probeIntervalForOpen(now, open)` + :989 "窗口到点后的首次 tick 立即探测"旁路（`!probe && now.After(open) && last.Before(open.Add(-time.Second))`）。Start 主循环重登回传 :679 补探测复位 lastProbe（:678 持锁）。
  - **probing 单飞守卫**：probe() 入口持 s.mu 置位（:1050-1054，"置位-检查"原子），命中单飞直接放弃本次探测（最多推迟一个 tick）；goroutine 主体全局 FindElectives 前后用任一客户端（:1078），退出复位 :1081/:1088/:1113（含失败路径）。
  - **probeSem 信号量 cap=4**：probe() 内 per-account 探测 goroutine（:1069-1077）`s.probeSem <- struct{}{}`（:1073）入场 + defer 释放——N 账号部署下 per-account findElectivesData 并发峰值从 N 封顶到 4，跨 2s 批由同一信号量约束绝不叠加。注释 ponytail 标记 :1068"cap=4 常驻，若平台放宽熔断或账号数 >50 再调"。goroutine 内部直接 `s.ProbeForAccount(acct)`（:1075），其返回值被 `_, _ =` 忽略（非库写，错误已在各自路径打印）。
  - **probeIntervalForOpen 降频**：open 零值 = 远间隔（未识别/识别过期绝不高频轰炸）；`now.After(open) && WindowClosed()` = 幽灵窗口降回 30s（:97）；临门 5 分钟内 2s 盯守（:100）。
  - **HTTP 侧 ProbeForAccount 刻意不套单飞**（注释 :868-871 反向确认——管理员刷新要强制最新 + 频率已受节流约束），维持观察语义。
- **实测**：race 定向四包全绿含本族；TestProbeIntervalFor/TestProbeIntervalZeroOpenTime/TestProbeIntervalWindowClosed/TestWindowClosedProbeDropsToFar 全部覆盖断言在位（scheduler_test.go 走读确认）。
- **结论：节流三件套互斥完整、无叠加轰炸路径，与"访问过于频繁 1 分钟熔断"实证契约对齐。**

### 6.2 多账号年级隔离快照（ElectivesSnapshotFor 回退链 + CheckClassSelectable 同源）

- **回退链完整形态**（scheduler.go:761-805）：
  - 目标账号（`len(acctTargets[acct]) > 0`）：必须用该账号专属年级帧，新鲜（nowAlignedLocked().Sub(acctDataAt) <= 40s）即返回；过期/缺失一律 (nil,false) 触发 ProbeForAccount 真取，**绝不回退全局 lastData**（注释 :764-771：混合年级部署下全局帧可能是 m.order[0] 的年级帧，回退=年级串线 + 浏览者永不触发本账号刷新）。判据用 len>0 而非 map key（:772-776：handler 清空目标落空 slice，按 key 判断会一直走"有目标"专属路径）。
  - 无目标账号但有专属帧：新鲜即返回，过期也必须返回 (nil,false) 触发刷新（:794-798，注释 :788-793：全局帧恰被 order[0] 刷新鲜时回退 ok=true 会误导——"快、无网络开销"只对从未有过专属帧的纯浏览成立）。
  - 纯浏览从未有过专属帧：回退全局帧（:801-804，lastData 新鲜才 ok）。
  - 回退时机统一 nowAlignedLocked（时间基与写入侧 acctDataAt 同源）。
- **CheckClassSelectable 同源**（:1863-1892）：只读该账号专属帧（acctData[acct]），**不使用全局 lastData 兜底**（注释 :1858-1862：跨年级帧可为任意账号，无参考价值）；无快照/过期/课程不在快照一律放行交给平台最终把关。
- **probe() 每账号独立刷新**（:1069-1077 per-account goroutine）+ ProbeForAccount 回写同样只写专属帧（:863-864 不动 lastData/lastDataAt，注释 :865-867"管理员穿透写全局帧会污染全局快照=年级串线根因之一"）。
- **实测**：TestManualSnapshotFallbackOnlyWhenOwnFresh / TestSnapshotTTLUsesAlignedClock / TestElectivesSnapshot / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 覆盖断言在位，race 全绿。
- **结论：年级隔离快照回退链三态判定（目标/无目标有过期帧/纯浏览）完整，无年级串线路径，与决策契约 8 逐字对齐。**

## 7. 维持观察项

- 无新增。既有观察项本轮零漂移（accounts 包三夹具无 socketPreheat 双保险的既有缺口已注释锁定为未来 flake 第一候选；restore 完成后补探测与补同步的观测语义维持）。

## 8. 结论

**裁定：APPROVE**

身份防线矩阵第五十三轮闭合（sameClientFor 定义 scheduler.go:204 + 7 调用点逐一追到写状态/落库终局零漂移；maybeRelogin 双侧 + 二次 ClientFor 终局射证；手动五路 accountExists 判据同源；写点换类 5 类全持锁；*Locked 族与外部写函数双向射证；无锁写点 warnedNoTargets 宿主唯一性证明）。OBSERVE-117-01 知识位第二十一轮在位。B110-01 审计链第二十八轮零漂移（手动 6 失败位 + 成功审计行 + 自动链失败族 + 零吞错穷举 + token 脱敏延续）。O105-01 抖动基线定向 race 四包 + 身份防线族 + 回归锚 + 全量回归全绿。LOW-132/133 回首白线核对通过 + 时间基残留全部读写同基自洽。两个自选纵深（探测定时族节流三件套 / 多账号年级隔离快照回退链）契约在位。契约轮次标签产品代码零命中。工作区除本报告与前端并行产物外零漂移。全轮无 CRITICAL/HIGH/MEDIUM/LOW 发现。