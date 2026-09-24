# R139 后端审查报告（绝对只读，身份防线矩阵第五十四轮）

- 日期：2026-09-24
- 基线：R138 归档（92f0f06）；工作树 `git status --short --branch` 显示 `## master` + 唯一未跟踪文件 `?? archive/review-rounds/round139-frontend-findings.md`（前端并行代理产物）。backend/ 产品代码自 R138 后零改动（`git diff HEAD --stat` 空，仅未跟踪前端产物）。
- 模式：绝对只读（唯一允许写文件为本报告；全仓库其余零修改）
- 聚焦清单六项全实测 + 2 个自选新纵深（时间盒 35 分钟内完成，取窗口状态三判据单源 open 快照复用 + 登录时序攻击族）
- 结论前置：**身份防线矩阵第五十四轮闭合，全部验证项实测绿，新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW，裁定 APPROVE**

## 验证表（全部实测）

| 项目 | 结果 | 证据（实测时间/数据） |
|------|------|------|
| go build ./... | 绿（exit 0） | 全包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race 四包 | 全绿 | 借 `/d/mingw64/bin/gcc.exe`（实测存在，gcc 15.1.0）注入 PATH `-count=1` 强制重跑：zhidao 2.184s / accounts 1.509s / scheduler 15.106s / api 14.807s 全 ok 零竞态 |
| 身份防线族 race 十测 | 全绿 | `-race -run 'TestProbeDeletedThenRebuiltSameNameDropsSnapshot\|TestProbeForAccountDropsWriteWhenRemoved\|TestProbeChainSameClientIdentity\|TestDeletedAccountRebuiltSameNameChainDrops(S..\\|Relogin\\|RateLimitBackoff\\|WindowClosedFull\\|RealtimeRecheckFull)\|TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin\|TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight'` scheduler 包 1.967s 十测全 PASS |
| 回归锚 | 全绿 | TestWindowOpenSubmitsWithoutProbeReset 1.05s + TestSubmitSuspendedWhenOpenTimeCleared 1.28s（scheduler 包）+ TestAdminStatsWindowOpenedUsesScheduler 0.42s（api 包）定向 race 全 PASS |
| 时钟时间基族 race | 全绿 | TestSnapshotTTLUsesAlignedClock + TestClockSyncFailureBackoffUsesAlignedClock（scheduler 包）1.415s 全 PASS |
| 登录闸门族 race | 全绿 | TestLoginByPasswordRejectsWhenGateBudgetExhausted + TestLoginByPasswordAllowedWhenGateBudgetAvailable + TestLoginByPasswordEncryptFailLogs（accounts 包）1.433s 全 PASS |
| 手动审计族 race | 全绿 | TestManualElectiveFailureAppendsLog + TestManualElectiveReadErrAppendsLog + TestHandleElectivesSelectUnauthorizedRelogin + TestHandleElectivesSelectReadErrMessage（api 包）全 PASS |
| 落库零吞错族 + 重登日志族 race | 全绿 | TestStoreFailuresLogged + TestSetTargetsDeleteRefusedFailureLogged + TestReloginLogs + TestReloginFailureLogs + TestRealtimeRecheckDeletedAccountDropsLog + TestMaybeReloginDeletedAccountSkipsMaps（scheduler 包）2.199s 全 PASS |
| 基础设施状态码族 race | 全绿 | TestApiUnknownPath404 + TestRequireJSONBodyRejectsFormContentType + TestRecoverMiddlewareHidesPanicDetail（api 包）全 PASS |
| 全后端非 race 全量回归 | 全绿 13/13 | backend 0.898s / accounts / api / config 0.830s / db 1.655s / runtime 0.798s / scheduler / secure 0.759s / session 0.903s / store 2.793s / zhidao 全 ok |
| 契约轮次标签扫描 | 产品代码零命中 | 全仓扫 `第.[0-9]*.轮/R[0-9]*.轮/round [0-9]/Round/review-round` 产品代码唯一命中 session/store.go:117 注释引用历史文档名 `docs/review-round13.md`（历史引用，合规，同 R137/R138 反证）；测试叙述/历史文档引用合规 |
| 工作区零漂移 | 成立 | `git status --short --branch` 仅 `?? archive/review-rounds/round139-frontend-findings.md`，无任何已跟踪文件改动 |

## 1. 身份防线矩阵第五十四轮闭合

### 1.1 sameClientFor 定义与 7 调用点逐一确认零漂移 + 写状态/落库终局

- 定义：scheduler.go:204 `sameClientFor(acct, chainClient)` → :205-209 `!ok || current == nil` 存在性内含（已删即 false）+ 指针身份比对；:215 `clientIdentity` 用 `reflect.ValueOf(c).Pointer()` 取具体指针类型底层指针值，:220-223 nil/非指针保护返回 0 恒非同一。grep 全仓 `clientIdentity(` 仅 :209/:215 出现，无旁路比对。零漂移。
- 7 调用点逐一追到"写什么状态 / 落什么库行"终局：
  - **:850-853**（ProbeForAccount 回写段）——FindElectives 最长 15s 后锁内复核；不放行即放弃写 `acctData[acct]`/`acctDataAt[acct]`（:863-864）与 `openTimeDetected[acct]` 识别槽（:856）。注释 :843-848 覆盖同名重建顶替语义。
  - **:1489-1493**（spawnChain 失效分支）——身份不符锁内 `delete(s.inflight[acct], t.ClassID)` 后静默弃整链（不写状态/不落日志）；**:1499 `maybeRelogin` 落在复核之后**（注释 :1494-1497 明言"重登必须落在身份复核之后"）。TestDeletedAccountRebuiltSameNameChainDropsRelogin 实测绿（relogCalls==0 + reloginFail 无残留）。
  - **:1521-1544**（成功分支）——通过才 `done[acct][id]=true`（:1529）+ setStateLocked(success)（:1530）+ AppendLog(select,true)（:1532）+ SaveSuccess 落库（:1535）。注释 :1515-1520 覆盖重启 RestoreDone 假成功语义。
  - **:1551-1563**（风控退避分支）——通过才 markRateLimitedLocked 30s（:1555）+ setStateLocked(failed)（:1556）+ AppendLog（:1558）。注释 :1547-1550 覆盖假退避让黄金期静默跳过语义。TestDeletedAccountRebuiltSameNameChainDropsRateLimitBackoff 实测绿。
  - **:1571-1577**（窗口关闭分支）——通过才 markFullLocked（:1575 写 full 集合 + AppendLog）。TestDeletedAccountRebuiltSameNameChainDropsWindowClosedFull 实测绿。
  - **:1600-1603**（实时复核回锁后三路总闸）——存在性内含于 sameClientFor（注释 :1596-1599 六路对称声明）。不放行锁内直接 return，绝不让三路（失效重登 / 确证满员 / 普通失败）写重建身份。
  - **:1635-1638**（确证满员分支内的二重复核）——回锁后 :1600 总闸之外，:1635 在落 markFullLocked（:1639）前再复核一次（注释 :1631-1634）；:1627 `doneHas` 让位于手动报名胜利状态。TestDeletedAccountRebuiltSameNameChainDropsRealtimeRecheckFull 实测绿。
  - 第六分支实时复核 ErrUnauthorized（:1608-1619）：sameClientFor 总闸 :1600 在前，通过后才 maybeRelogin + setStateLocked(failed) + AppendLog（:1612-1617）。TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin 实测绿（0.51s，含 500ms 轮询窗口防假绿）。
  - 成功分支 inflight 清位归因契约测试 TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight 实测绿（公共清位路径已在 SelectClass 返回后统一执行，含身份复核失败路径）。

### 1.2 maybeRelogin 双侧 + 二次 ClientFor 终局（OBSERVE-117-01 关键性质）

- **决策侧 :1208-1210**——reloginMu + s.mu 双持锁内、任何 map 写入前 `ClientFor(acct)` 存在性复核（注释 :1202-1207 明言"探测定时三处对 ErrUnauthorized 直调本入口……账号已删不发起重登、不写任何 map"）；已删静默 return。随后才读写 relogging/reloginFail/reloginAt/tokenValid（:1212-1241）。TestMaybeReloginDeletedAccountSkipsMaps 实测绿（tokenValid/reloginFail/reloginAt/relogging 四 map 零残留）。
- **写回侧 :1254-1258**——goroutine 开头锁内 `ClientFor(acct)` 复核，已删整个成功分支（内存写 + UpdateIDToken 落库）静默放弃只清 relogging。TestDeletedAccountReloginSuccessDropsState 在位。
- **二次 ClientFor 终局 :1265-1273**——success 分支清完内存标记后 `if client, ok := s.clients.ClientFor(acct); ok` 重取当前注册表 client → :1266 `client.Token()`（client.go:195-199 锁内读当前 token 实值）→ 非空才 :1269 `UpdateIDToken(acct, tok)` 落库。落库 token 恒为重登完成后注册表内该账号客户端的实值，绝非 goroutine 发起时捕获的旧身份/旧 token——同名重建场景绝不串旧身份。落库失败 `log.Printf`（:1270-1271）零吞错。
- 结构性保证链完整：ReloginIfNeeded（client.go:602）→ Login（client.go:217）→ submitLogin 成功在 c.mu 内写 c.token + zd_edu_cookie（client.go:392-394）→ Token 锁内读 → UpdateIDToken 落库。

### 1.3 手动五路 accountExists（判据同源，handler.go）

| 路径 | 行号 | 语义 |
|------|------|------|
| handleElectives（课程读） | :255 | 凭据表查无 → "账号不存在，无法查看课程" |
| handleElectiveSelect（手动报名） | :305 | 查无 → "账号不存在，无法执行报名操作" |
| handleElectiveExit（手动退选） | :397 | 查无 → "账号不存在，无法执行退选操作" |
| handleSetTargets（目标写） | :497-512 | 显式 LoadCredentials 循环比对（注释 :494-496 明言 authenticateDirect 直连建立会话的账号须被放行） |
| handleState（状态读） | :573 | 查无 → "账号不存在，无法读取状态" |

判据同源：`accountExists`（:1109-1120）= LoadCredentials 逐账号比对（凭据表 = "确实登录过"的更强真理源，注释 :1106-1108）。`allowAccountOverride`（:1102-1104）仅管理员会话可穿透。五路全覆盖零漂移。

### 1.4 写点换类 5 类全持锁 + *Locked 族与外部写函数双向射证

- **lastSubmit**：写点 submitAll 首行 :1346（`s.lastSubmit = s.nowAlignedLocked()`），持 s.mu；tick 判读 :1029 持锁取。
- **lastSyncStart**：写点 maybeSyncClock :356 持锁；读 :393（sync goroutine 持锁推进 lastSyncTime）；复位 :405（无客户端 fallback 持锁）。
- **syncing**：置位 :355（持锁）、复位 :369（sync goroutine 持锁）/ :404（fallback 持锁）。
- **lastProbe**：写点四路全部持锁——ProbeNow :964 / probe 失败 :1089 / probe 成功 :1114 / Start 主循环重登回传 :679（:678 持 s.mu）。ProbeForAccount :873 刻意不写（管理员穿透探测不吞全校节流）。注释 :868-871 明言"lastProbe 只归 probe() 与 ProbeNow 管理"。
- **state.EmptyProbeRuns**：写点 probe :1156/:1158 持锁（入账侧 +10s 裕量，注释 :1151-1154）。
- **\*Locked 写函数族**（持锁被调方，13 个）：nowAlignedLocked（:273 自实现不取锁，调用方持 s.mu）/ openTimeForLocked（:427）/ enrichTargetPubMetaLocked（:539）/ rebuildCoursesForAccountLocked（:570）/ rebuildCoursesLocked（:647）/ tokenValidForLocked（:739 只读）/ isRateLimitedLocked（:1691 读写）/ markRateLimitedLocked（:1710）/ releaseFullIfFreedLocked（:1781）/ statusIndexLocked（:1835）/ setStateLocked（:1846）/ markFullLocked（:1760）/ windowClosedLocked（:918，三条判据单源 open 快照复用，:924 单次取 open 对判据 2/3 统一）——grep 全仓无"外部直接调 *Locked"形态，名字即锁契约零漂移。
- **外部写函数首行取锁**双向射证：SetTargetsForAccount :455 / PurgeAccount :496 / RestoreTargets :526 / RestoreDone :608 / RestoreRefused :627 / MarkDone :1923 / RemoveDone :1990 / RemoveFull :2039 全部首行 `s.mu.Lock()`。
- **无锁写点宿主唯一性射证**：`warnedNoTargets`（:1371-1372，submitAll 内写）唯一调用链 = tick（:1035）→ Start goroutine for-select 主循环（:673-675 tick 唯一调用点），天然串行；`start` 字段 :664 持 s.mu 写；`chains` map 由独立 chainMu 保护（:1392-1403 三处）。无第二宿主。grep 确认 `warnedNoTargets` 全仓仅 :176 声明 + :1371/:1372 写，零读（一次性警告语义）。

## 2. OBSERVE-117-01 知识位第二十二轮确认在位

- 写回侧先 :1254 ClientFor(acct) 复核（已删整体放弃，含内存写与落库）→ success 分支 :1265-1266 另取一次当前注册表 client → client.Token() → :1269 UpdateIDToken 落库。落库 token 恒为重登完成后注册表内该账号客户端的实值。
- 同名重建场景：删号后新 *Client 注册，旧重登 goroutine 返回时 :1254 正常通过（新身份存在），:1265 重取的正是**新身份**的当前 token——不串旧身份（连续第二十二轮零漂移）。
- 活化条件（未来改动破坏"落库取当前注册表 token"关键性质）未触发。

## 3. B110-01 审计链第二十九轮零漂移

- 手动 6 失败位 AppendLog 逐一就位：**handleElectiveSelect** :362（ErrUnauthorized）/ :374（read 类"结果未知"最需留痕）/ :381（业务失败原文）；**handleElectiveExit** :443 / :452 / :459 三路对称。全部 `if err := ...; err != nil { log.Printf }` 零吞错。
- 成功审计行：手动 select → MarkDone 内 AppendLog（:1976）；手动 exit → RemoveDone 内 AppendLog（:2029）；login 日志 issueSession :237（含管理员 :136）；set_targets :558；config :864；delete_account :1055；logout :616。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 确证满员 markFullLocked :1770 / 普通失败 :1662，全部落库失败记日志。
- 零吞错穷举（测试外，grep `_ =`/`_, _ =`/直接赋值忽略形态全仓扫）：`_ =` 仅防御性忽略——handler.go:387 MarkDone / :467 RemoveDone（库写在方法内部已处理）、config.go:96 os.Setenv 与 :141/:160 文件写入（模板/环境文件非审计链）、client.go:107/:124 io.Copy（响应体排空）、scheduler.go:307 Prewarm / :1075 ProbeForAccount（非库写，错误已在各自路径打印）。grep 落库族调用（AppendLog/SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefusedClass/SetTargetsForAccount/UpdateIDToken）**零 `_ =` 命中**，20 处调用全包裹 `if err != nil { log.Printf }`。零库写忽略。
- 网络层 token 脱敏延续抽查：doRequest :422 拼 `?idToken=` → :447-450 网络错误走 sanitizeError（client.go:581-593 剥 *url.Error 完整 URL 文本、Unwrap 下沉底层错误保 isConnErrRetryable/IsReadErr 判型穿透）→ 消费侧 maskedToken（scheduler.go:1283-1339，len>8 只显前 8 位）。B101-01 延续无回归。

## 4. O105-01 抖动基线实测绿

- 夹具在位：zhidao/client_test.go:25 socketPreheat（包级 TestMain 预预热 + :40 逐测试调用）+ :88 readyProbe（200ms×10 + 2s 显式宽栅栏）；captcha_test.go:17/:52 同款 socketPreheat + :29/:79 readyProbe；sanitize_test.go:97 同款 socketPreheat；api/handler_test.go:139+:185 readyProbe；accounts/manager_test.go:26 readyProbe（注释明言"三处夹具未有 socketPreheat 双保险，8 轮全绿实证无残余；若未来 accounts 再出冷启动 flake 第一候选即补"）。scheduler 包无 mock 服务器夹具（纯 fake），不需就绪探测。
- 定向 race 四包实测全绿（借 `/d/mingw64/bin/gcc.exe` 注入 PATH，`-count=1` 强制重跑），含身份防线族 + 回归锚 + 时钟时间基族 + 登录闸门族 + 手动审计族 + 基础设施状态码族 + 落库零吞错族——全部一次通过，无 flake。

## 5. LOW-132/133 回首核

- **git show 白线核对**：基线 92f0f06 相对 R138 = 纯 docs（review-round138.md 双 findings + 收尾总结），backend 产品代码零改动。历史修复提交 975dc2b（LOW-132-01，:372 lastSyncFailAt / :377 syncFailedWindow 写入 `time.Now()` → `nowAlignedLocked()`）与 d75f38c（LOW-133-01，ElectivesSnapshotFor/ElectivesSnapshot/CheckClassSelectable 判读侧四处 `time.Since` → `nowAlignedLocked().Sub`）diff 白线核对逐字一致，身份防线零改动。
- **时间基全量扫零残留**（产品代码，grep time.Since/time.Now() 全仓）：
  - scheduler.go:1220/:1227 `time.Since(reloginAt)` → 写 :1231/:1261 同 time.Now() 本地钟，读写同基自洽（重登退避是"距上次重登 30s"相对度量，与对齐钟无关），**合规**。
  - accounts/manager.go:71/:226 `time.Since(gateWindow)` → 写 :53/:74/:227 同本地钟，读写同基自洽（登录闸门"每窗口 2 次"相对度量、同一 gateMu 保护），**合规**。
  - client.go:113/:135/:137 SyncServerTime 的 time.Now() 本就是 RTT/clockOffset 计算需要本地钟（`start := time.Now()` → `time.Since(start)` → `estimatedServerTime.Sub(time.Now())`），非调度判读侧，**合规**。
  - client.go:355 uniqueId 同本地毫秒钟，与 getUniqueDeviceId 逆向契约对齐，**合规**。
  - session/store.go:74/:106/:128/:156/:168/:183/:198 与 handler.go:1176/time.Since 无——会话/票据/限流桶过期判定（秒级精度 TTL 相对度量，非调度敏感路径），**合规**。
  - 残余仅 reloginAt 与 gateWindow 两处写读同基自洽（同 R137/R138 复确认），零混用孤岛。

## 6. 新契约角度纵深（自选 ×2）

**选此方向的理由**：上轮已覆盖探测定时族/年级隔离快照；项目近几轮已横向锁死身份防线与审计链，剩两个契约浓度最高的纵向面——窗口状态三判据是"幽灵窗口挂起/黄金期放行"的判据中枢（正确性核心），登录时序族是"平台锁号防线 + 侧信道"的对抗面（安全核心）。两者此前仅有零星走读记录、无系统性射证，值得本轮完整盘点。时间盒 35 分钟内完成。

### 6.1 窗口状态三判据单源 open 快照复用（windowClosedLocked）

- **三判据单源**（scheduler.go:918-939）：`WindowClosed()`（:907-911）与 `StateForAccount`（:699，:703 `st.WindowClosed = s.windowClosedLocked()`）共用同一实现，杜绝两套真相分叉。
  - **判据 1（主判据）**：`s.state.WindowClosed` 由 probe 写入（:1146 `prevOpened && !opened && len(data.Publishes)==0 && now.After(open.Add(10*time.Second))`，注释 :1128-1142 覆盖"开过再关才算关" + 10s 裕量过渡态语义）。
  - **判据 2（时钟兜底）**：`:925 syncFailStreak >= 3 && !open.IsZero() && nowAlignedLocked().After(open)`——必带"开放时间已过"防未来开窗点误挂（决策契约 19）。
  - **判据 3（幽灵窗口量变）**：`:935 !open.IsZero() && !state.WindowOpened && state.EmptyProbeRuns >= 3`——从未开过窗 + 连续三轮空快照视同关闭。
- **open 单快照复用**：:924 `open := s.openTimeForLocked("")` 全函数只取一次，判据 2/3 共用——识别槽是唯一事实源，一次读取避免"识别值在两次读取间被新批次覆盖"的不一致窗口（注释 :922-923）。入账侧 probe 亦在 :1145 取单次 open 快照，同时供主判据 :1146 与 EmptyProbeRuns 入账 :1155 复用（注释 :1143-1144"同一探测内两处 10s 裕量判定若各自取 open，热改亚毫秒窗口内主判据与 EmptyProbeRuns 入账可能基于新旧两个不同 open"）。
- **外围读侧同快照纪律**：windowClosedLocked 内复用的 `nowAlignedLocked()` 亦统一对齐钟（:925/:1146 读 vs 判据写入同基）；tick 提交守卫 :1024 `s.WindowClosed()` 与 :1011 零值守卫并列——开窗瞬间 WindowClosed=false 绝不影响黄金期（决策契约 32 B41-02 语义在位，TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 实测绿）。
- **实测**：全后端 race 全绿 + TestAdminStatsWindowOpenedUsesScheduler（stats window_closed 与学生端 /state 同源，逐字段判定）实测绿。
- **结论：三判据单源闭合、open 快照复用纪律全链统一，无分叉/无混基，与决策契约 1/2/3/19 逐字对齐。**

### 6.2 登录时序攻击族（B43-04 撞名双条件 + gateTryAcquire 非阻塞准入）

- **B43-04 撞名双条件**（handler.go:131）：`req.Account == adminName && subtle.ConstantTimeCompare([]byte(req.Password), []byte(d.AdminToken)) == 1`——仅"管理员名 + 管理口令都匹配"才签发管理员会话；不匹配的撞名学生走教务登录（:142 LoginByPassword 成功正常签发，绝不吞）。ssl 时延语义：
  - 正确口令分支 :134 `time.Sleep(loginTimingFlat)`（注释 :133"与错误分支等时——抹平'口令对错'时延差"）；
  - 管理员名 + 教务登录也失败分支 :146-149 `time.Sleep(loginTimingFlat)` + 归因"管理口令错误"（注释 :142-145：管理员账号名不再能靠响应快慢被侧信道枚举出口令正确性）；
  - 撞名学生（≠管理员逻辑身份的普通学生账号）教务成功 :168 走 issueSession 正常签发普通会话，与管理员会话隔离（CreateAdmin :135 → IsAdmin :186 会话级标记）。
  - `loginTimingFlat = 300ms`（:1161-1164 常量定义，与教务登录 Vision+网络往返同量级）。
- **gateTryAcquire 非阻塞准入**（manager.go:223-235）：与 gateWait（:49-64）共享同一 `gateMu` 与 `gateUsed` 计数（注释 :218-222"窗口内 quota 充足则消耗并返回 true，已满则返回 false——绝不阻塞等待下个窗口"）。LoginByPassword :244 在 `c.Login` 前调用，quota 满返回"登录尝试过于频繁，请稍后再试"（:245-246）——**绝不用阻塞 gateWait**（排队会挂起用户登录响应数分钟）。管理员入口换绑（:243 注释"管理员入口同样收口到此闸门"）同走收口；管理员本身登录走 handleLogin 单独分支不触碰教务闸门（注释同款）。
- **被拦后撤摊**：LoginByPassword 失败只摘除"本次新建的空壳"（:264-274 `wasShell := c.Token() == ""`），注册表里已持有效 token 的工作客户端绝不误删——黄金期手滑输错密码不至于全程失联。
- **实测**：TestLoginByPasswordRejectsWhenGateBudgetExhausted + TestLoginByPasswordAllowedWhenGateBudgetAvailable（accounts 包 race 1.433s 全 PASS）；登录 CSRF 门 (router.go:99-124 双限流桶独立记账) + TestRequireJSONBodyRejectsFormContentType / 管理员会话鉴权 TestAdminStatsWindowOpenedUsesScheduler 全绿。
- **结论：撞名双条件 + 常数时间口令比对 + 时延拉平三件套在位；gateTryAcquire 与 gateWait 共享计数收口全校 doLogin 预算，无阻塞旁路、无预算透支路径。**

## 7. 维持观察项

- 无新增。既有观察项本轮零漂移（accounts 包三夹具无 socketPreheat 双保险的既有缺口已注释锁定为未来 flake 第一候选；restore 完成后补探测与补同步的观测语义维持；classFullRealtime 保留为"平台未来下发 maxCount 时自动生效"的防御性路径维持）。

## 8. 结论

**裁定：APPROVE**

身份防线矩阵第五十四轮闭合（sameClientFor 定义 scheduler.go:204 + 7 调用点逐一追到写状态/落库终局零漂移；maybeRelogin :1208 决策侧 ClientFor 前置复核 / :1254 写回侧复核 / :1265-1273 二次 ClientFor 重取当前注册表 Token 落库终局射证；手动五路 accountExists 判据同源；写点换类 lastSubmit/lastSyncStart/syncing/lastProbe/state.EmptyProbeRuns 五类全持锁；*Locked 写函数族 13 个 + 外部写函数 8 个首行取锁双向射证；无锁写点 warnedNoTargets 宿主唯一性证明）。OBSERVE-117-01 知识位第二十二轮在位。B110-01 审计链第二十九轮零漂移（手动 6 失败位 + 成功审计行 + 自动链失败族 + 零吞错穷举零命中 + token 脱敏延续）。O105-01 抖动基线定向 race 四包 + 身份防线族十测 + 回归锚 + 时钟时间基族 + 登录闸门族 + 手动审计族 + 基础设施状态码族 + 落库零吞错族全绿。LOW-132/133 回首白线核对通过 + 时间基残留全部读写同基自洽。两个自选纵深（窗口状态三判据单源 open 快照复用 / 登录时序攻击族）契约在位。契约轮次标签产品代码零命中。工作区除本报告与前端并行产物外零漂移。全轮无 CRITICAL/HIGH/MEDIUM/LOW 发现。