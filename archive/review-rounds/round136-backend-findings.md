# R136 后端审查报告（绝对只读，身份防线矩阵第五十一轮）

- 日期：2026-09-24
- 基线：R135 归档（665bbb6）；工作树 `git status --short --branch` 显示仅 `?? archive/review-rounds/round136-frontend-findings.md`（前端代理并行产物），backend/ 产品代码自 R135 后零改动（git log backend/internal/scheduler/scheduler.go 最近提交 = d75f38c / 975dc2b，属时钟时间基域，身份防线零改动）。
- 模式：绝对只读（唯一允许写文件为本报告；全仓库其余零修改）
- 聚焦清单六项全实测 + 2 个自选新纵深（时间盒 35 分钟内完成，取连接活性自愈族 + HTTP 状态码家族核对）
- 结论前置：身份防线矩阵第五十一轮闭合，全部验证项实测绿，新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW，进度 137/256

## 验证表（全部实测）

| 项目 | 结果 | 证据（实测时间/数据） |
|------|------|------|
| go build ./... | 绿（exit 0） | 全包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race 四包 | 全绿 | 借 /d/mingw64/bin/gcc.exe 实测 CGO_ENABLED=1：zhidao 2.031s / accounts 1.425s / scheduler 15.405s / api 15.484s 全 ok 零竞态 |
| Linux 交叉编译 | 绿 | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` 成功（发布流水线均走 CGO=0 交叉路径，时间基代码无平台差异） |
| 身份防线族 race | 全绿 | `-race -run 'TestDeletedAccountRebuiltSameNameChainDrops\|TestWindowOpenSubmitsWithoutProbeReset\|TestAdminStatsWindowOpenedUsesScheduler'`：TestWindowOpenSubmitsWithoutProbeReset 1.03s + 五分支 TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} 全 PASS（2.330s） |
| 成功分支 inflight 位实测归因 | 全绿 | TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight（scheduler_test.go:3594）race 1.346s PASS |
| 实时复核第六分支（OBSERVE-117 同族） | 全绿 | TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin（scheduler_test.go:3511）race 2.399s PASS；TestDeletedAccountRebuiltSameNameChainDropsRealtimeRecheckFull race 2.152s PASS |
| 时钟/窗口/闸门/恢复全族 race | 全绿 | 定向 13 测（TestRestoreDoneSkipsResubmit / TestSnapshotTTLUsesAlignedClock / TestClockSyncFailureBackoffUsesAlignedClock / TestClockSyncNoRetryWithinBackoff / TestClockSyncFailureResetsOffset / TestWindowClosedState / TestGhostWindowEmptyProbesSuspend / TestRealtimeRecheckUnauthorizedTriggersRelogin / TestRealtimeFullRecheck{KeepsManualSuccess,WithNoManualDoneMarksFull} / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime / TestMaybeReloginDeletedAccountSkipsMaps / TestScheduleIntervalClamped）2.211s 全 PASS |
| 闸门族 | 全绿 | TestLoginByPasswordRejectsWhenGateBudgetExhausted（accounts 包）race PASS（7.403s 含全包） |
| HTTP 状态码家族 | 全绿 | TestRecoverMiddlewareHidesPanicDetail / TestAdminStatsTargetsLoadFailureReturns500 / TestLoginRateLimit / TestApiUnknownPath404 / TestHandleElectivesSelectUnauthorizedRelogin race 3.855s 全 PASS |
| AdminStats 家族 | 全绿 | TestAdminStatsWindowOpenedUsesScheduler + {TargetsLoadFailureReturns500,AccountsLogs,OpenTimeFromRecognized} 6.463s 全 PASS |
| gcc 探测 | /d/mingw64/bin/gcc.exe 存在 | 全轮实测定向 race 绿 |
| 契约轮次标签扫描 | 产品代码零命中 | 全仓扫 `第N轮/R..轮/round N/轮次`（含 go 文件）产品代码零命中 |
| 零吞错穷举 | 零命中 | 双下划线 `_, _ =`/`_, _ :=` 与直接赋值忽略形态全仓（测试外）零命中（全部 3 处 `_, _ =` 为非库写防御性忽略：scheduler.go:1075 ProbeForAccount、client.go:107/:124 io.Copy） |
| 工作区零漂移 | 成立 | 除本报告外唯一未跟踪文件为前端代理产物 round136-frontend-findings.md |

## 1. 身份防线矩阵第五十一轮闭合

### 1.1 sameClientFor 定义（scheduler.go:204）与 7 调用点逐一确认零漂移

- 定义：scheduler.go:204 `sameClientFor(acct, chainClient)` → :209 `ClientFor(acct)` 存在性内含 + :215 `clientIdentity`（reflect.ValueOf(c).Pointer() 指针身份；:216-224 nil/非指针保护返回 0 恒非同一）。注释 :199-203 覆盖同名重建语义 + "需持 s.mu" 声明。grep 全仓 `clientIdentity(` 仅在 :209/:215 出现，无旁路比对。零漂移。
- 7 调用点逐一追到"写什么状态 / 落什么库行"终局：
  - **:850** —— ProbeForAccount（探测回写段）：网络 FindElectives 最长 15s 后，锁内 `!sameClientFor` 放弃写 `acctData[acct]`/`acctDataAt[acct]`/`openTimeDetected[acct]`（:855-864），绝不覆盖重建账号识别槽/年级串线。M88-01 防线单点覆盖。
  - **:1489** —— spawnChain ErrUnauthorized 失效分支：身份不符 `delete(s.inflight[acct], t.ClassID)`（:1490）后静默弃链；**重登触发 :1499 落在复核之后**（注释 :1494-1497 明言"重登必须落在身份复核之后"）——旧链绝不消费新身份登录预算。
  - **:1521** —— 成功分支：`sameClientFor` 通过才写 `done[acct][classID]=true`（:1529）+ `setStateLocked success`（:1530）+ `SaveSuccess` 落库（:1535）+"报名成功"日志（:1532）。注释 :1515-1520 覆盖重启 RestoreDone 假成功语义。
  - **:1551** —— 风控退避分支：通过后才 `markRateLimitedLocked`（:1555 写 30s 退避截止）+ setStateLocked failed + AppendLog（:1558）。注释 :1547-1550 覆盖假退避让黄金期被静默跳过语义。
  - **:1571** —— 窗口关闭分支：通过后才 `markFullLocked`（:1575 写 full 集合 + AppendLog :1770）。注释 :1565-1569 覆盖"无效的课程ID"按满员记 full 语义。
  - **:1600** —— 实时复核回锁后三路总闸（存在性判定内含于 sameClientFor，注释 :1596-1599 六路对称点）。不放行则锁内直接 return，绝不让三路分支（失效重登 :1608-1619 / 确证满员 :1627-1641 / 普通失败 :1645-1666）写重建身份。
  - **:1635** —— 实时复核确证满员 `markFullLocked`（:1639）前；前置 `doneHas` 胜利让位（:1627，手动成功绝不覆盖）。
- 家族全景闭合：spawnChain 六 err 归并分支（失效/成功/风控/窗口关闭/失效-cErr/满员-cErr）+ 探测回写 = 7 个身份闸，全部为"网络往返之后、状态/库行写入之前"。grep 全仓 `sameClientFor(` 调用点 = 7（不含定义），与注释承诺一致。

### 1.2 写点换类 5 类全持锁 + *Locked 写函数族双向射证

- **lastSubmit**：:1346 submitAll 首行持锁写入（`s.nowAlignedLocked()`，与 tick :1032 判读基准同源）；:1029-1031 判读侧持锁读。
- **lastSyncStart**：:356 maybeSyncClock 持锁写入；:405 空库复位持锁；:393 成功推进 lastSyncTime = lastSyncStart（锁内）。
- **syncing**：:355 置位 / :369 goroutine 回调锁内复位 / :404 空库锁内复位——单飞守卫族全持锁。
- **lastProbe**：:964 ProbeNow / :1114 probe 持锁写入；:676-681 Start() 主循环 reloginResults 消费者锁内写零值——唯一无锁写点宿主 goroutine 唯一性射证成立（for+select 单 goroutine）。
- **state.EmptyProbeRuns**：:1156 递增 / :1158 归零（probe 锁内，幽灵窗口量变计数，判读侧 :935 持锁读）。
- *Locked 写函数族全列（13 个）：nowAlignedLocked:273 / openTimeForLocked:427 / enrichTargetPubMetaLocked:539 / rebuildCoursesForAccountLocked:570 / rebuildCoursesLocked:647 / tokenValidForLocked:739 / windowClosedLocked:918 / isRateLimitedLocked:1691 / markRateLimitedLocked:1710 / markFullLocked:1760 / releaseFullIfFreedLocked:1781 / statusIndexLocked:1835 / setStateLocked:1846——全部仅在持 s.mu 调用点出现；外部写函数首行取锁双向射证：PurgeAccount:495 / SetTargetsForAccount:454 / RestoreTargets:525 / RestoreDone:607 / RestoreRefused:626 / MarkDone:1922 / RemoveDone:1989 / RemoveFull:2038 / CheckClassSelectable:1863 / TryAcquireSubmit:1896 / StateForAccount:699 全首行 s.mu.Lock。零裸写点。

### 1.3 手动五路 accountExists 零漂移

- :255 课程读 / :305 手动报名 / :397 手动退选 / :497-512 目标写（内联 LoadCredentials 逐账号比对）/ :573 状态读——五路全数在位；:1109-1120 定义判据同源（凭据表 = 更强真理源）。handleSetTargets 缺透传且无核心账号时整体拒绝（:518-520），管理员名绝不落孤儿行。targets/electives/select/exit/state 五路对未知账号一律"账号不存在"整体拒绝，绝不回退全局帧假装成功。

### 1.4 maybeRelogin 双侧终局确认

- 决策侧：:1201 `reloginMu.Lock` → :1201 `s.mu.Lock` 锁序对齐（MarkTokenValid:1312-1319 / TokenValidFor:730-736 三方同序）；:1208 ClientFor 存在性复核在任何 map 写入前（注释 :1202-1207 覆盖探测定时三处对 ErrUnauthorized 直调入口的决策侧裸露），已删账号不发起重登不写任何 map。
- 写回侧：:1246 锁内先 delete relogging（:1247）→ :1254 ClientFor 存在性复核，已删则整个成功分支（含内存写 + UpdateIDToken 落库）静默放弃只清标记；:1265-1273 二次 ClientFor 重取当前注册表 `client.Token()`，非空才 `UpdateIDToken` 落库（:1269）——同名校验只保证"账号名当前在注册表"，二次取 Token 拿到的是重登后的新客户端值，绝不串旧身份。失败/未重登保持 tokenValid=true（:1286-1297，reloginFail 保留递增不击穿指数退避）。

## 2. OBSERVE-117-01 知识位第十九轮 —— 确认在位

写回侧先 ClientFor 复核（:1254）→ 重取当前注册表 client.Token() 落库（:1265-1273），同名重建场景绝不串旧身份；成功分支 delete reloginFail（:1260）+ 刷新 reloginAt（:1261）+ tokenValid=false（:1262）。决策侧/写回侧双闭合（B43-01 语义延续），实测 TestDeletedAccountRebuiltSameNameChainDropsRelogin race 绿。在位零漂移。

## 3. B110-01 审计链第二十六轮 —— 零漂移

- 手动 6 失败位 AppendLog 全数在位：报名三处 handler.go :362（ErrUnauthorized）/ :374（read）/ :381（其他）；退选三处 :443（ErrUnauthorized）/ :452（read）/ :459（其他），全部带显式 err 分支记日志。
- 成功审计行：手动报名成功 MarkDone 内部 :1976 AppendLog + :1973 SaveSuccess；手动退选 RemoveDone 内部 :2029 AppendLog + :2020 DeleteSuccess + :2026 SaveRefused；set_targets :558。全库共 12 处 AppendLog 调用点（含登录 :237 / 登出 :616 / 配置 :864 / 删账号 :1055 / 管理员登录 :136 / 激活签 :237 同源）。
- 自动链失败族：scheduler :1504 / :1532 / :1535 / :1558 / markFullLocked :1770 / :1614 / :1662 全部带错误日志；MarkDone :1945/:1973/:1976、RemoveDone :2020/:2026/:2029、SetTargetsForAccount :476/:484、UpdateIDToken :1269 全部持显式 err 分支。保险丝 DeleteRefusedClass :1945 同步。
- 零吞错穷举：`_ =` / `_, _ =` / `_, _ :=` / 直接赋值忽略形态全仓（测试外）零命中——唯一 3 处 `_, _ =` 均为非库写合规静默（scheduler.go:1075 ProbeForAccount 信号量内丢弃、client.go:107/:124 io.Copy Discard）。
- 网络层 token 脱敏延续抽查：zhidao/client.go:581 sanitizeError（B101-01）剥 `*url.Error` URL 文本（:566-572 sanitizerErr 保留 Unwrap 穿透判型）；scheduler.go:1283 maskedToken（:1334 只显前 8 位，短 token 恒 ***）；登录链日志 :250/:274 只显账号与识别次数不显 token。

## 4. O105-01 抖动基线 —— 夹具在位 + 定向 race 全绿

- socketPreheat：zhidao/client_test.go:25 定义（预创建-关闭 127.0.0.1 回环套接字），TestMain :39-42 包级调用 + 逐测试保留（:40/:47/:52）+ captcha_test.go:17/:52 + sanitize_test.go:97 同款。
- readyProbe：zhidao（client_test.go:88，200ms×10 + 2s 显式超时、socketPreheat 双保险）、api（handler_test.go:139 + :185）、accounts（manager_test.go:26，注释 :24-26 声明未 socketPreheat 双保险）三包在位。
- 回归双锚：TestWindowOpenSubmitsWithoutProbeReset（scheduler_test.go:1277，未来 open + opened=false 首段 pending 断言构造，防连接 reset 抖动）与 TestAdminStatsWindowOpenedUsesScheduler（api/handler_test.go:1011，window_opened/window_closed 必须与调度器三判据同源）race 全绿。
- 四包定向 race 全绿（见验证表），本轮无新增 flake。

## 5. LOW-132/133 回首核 —— 修复在位 + 时间基全量扫零残留

- git show d75f38c 白线核对：判读侧四处全 `nowAlignedLocked().Sub`（ElectivesSnapshotFor :780/:795/:801、ElectivesSnapshot :881、CheckClassSelectable :1871），与 R135 记录逐位一致。
- 全文件 `time.Since`/`time.Now()` 全量扫：残余精确收敛——`time.Since(t)` 仅 reloginAt 退避判读 :1220/:1227（写点 :1231/:1261 用 time.Now() 本地钟，写读同基准自洽为合规，注释 :1226 自明）；`time.Now()` 仅 nowAligned 定义 :269/:274 与 reloginAt 写点。scheduler.go:831 注释内引号用词非代码。zhidao/client.go:135 `time.Since(start)`（SyncServerTime RTT/2）为单次本地无态近似、非 scheduler 时钟状态机。accounts gateWait/gateTryAcquire（manager.go:53/:71/:226）本地钟自洽（每分钟翻页语义）。零混用残留。
- 测试族绿：TestSnapshotTTLUsesAlignedClock / TestClockSyncFailureBackoffUsesAlignedClock / TestClockSyncNoRetryWithinBackoff / TestClockSyncFailureResetsOffset / TestClockSyncNoClientResetsSyncing / TestClockSyncSuccessClearsFailStreak 全 PASS（race）。

## 6. 新契约角度（自选纵深 ×2）

### 6.1 连接活性自愈族 httpDo 判型族全量对齐（F52-M4/M5 纵深延续）

zhidao/client.go:478-492 httpDo 重试语义与 IsReadErr（:523-552）判型完全互补：重试仅限 `isConnErrRetryable`（:496-505，仅 *net.OpError.Op=="dial"/"write"，请求体未达服务端，重发安全）；read 型（RST/FIN/短读/超时四形态 + io.EOF）一律不重试（注释 :474-477 明言重发 POST 会双报，SelectClass/ExitClass 幂等防线被凿穿）。`cloneReq` :559-561 保证重发是深拷贝不共享 Body（GetBody 由标准库自动设置）。sanitizeError 在该特有错误链上 Unwrap 下沉，isConnErrRetryable/IsReadErr 的 errors.As/Is 穿透不受 URL 剥除影响。判型族自洽零漂移——连接活性自愈与身份防线在同一条 err 链上分域（自愈只认 dial/write、身份防线认业务/失效错误），无重叠无遗漏。

### 6.2 登录闸门族 gateWait + gateTryAcquire 双侧收口核对（B42-01 纵深延续）

accounts/manager.go :49-64 gateWait（阻塞，Relogin 自动重登用）与 :223-235 gateTryAcquire（非阻塞，LoginByPassword 手动登录用）共享同一 `gateMu` + `gateUsed` 计数 + `gateLoginPerMin=2` 分钟预算，严格按同一出口 IP 收敛全部账号 doLogin 流量。锁序：gateTryAcquire 先 Lock 再检查/消耗，与 gateWait 同一 gateMu 无死锁；手动登录 :244 预算满立即返回"登录尝试过于频繁"绝不挂起用户响应。`ResetGateForTest` :82-87 仅测试专用（api 夹具批量注册用）。实测 TestLoginByPasswordRejectsWhenGateBudgetExhausted（manager_test.go:174）race 绿——预算耗尽拒绝 + 消耗计入共享计数双断言成立。手动登录侧（B42-01 修的关键旁路）收口在位。

## 发现汇总（分级）

- CRITICAL：0
- HIGH：0
- MEDIUM：0
- LOW：0（本轮无新增）
- 维持观察（非新增，与前轮记录一致）：maybePrewarm 无独立单测（R127 首提，probeSem 信号量仍为唯一约束）；probeSem cap=4 常驻（R131 ponytail 注释，账号数 >50 再调）；Prewarm 错误静默（下个 15s 周期自然重试）；reloginAt 用本地钟时间基（写读同基准自洽、量级无感，LOW-132 全量扫确认为唯一自洽残余）。探测定时三处对 ErrUnauthorized 静默调用 maybeRelogin（内部已记日志，历轮维持观察）。

## 结论

身份防线矩阵第五十一轮闭合，零漂移零新增：sameClientFor 定义（scheduler.go:204）与 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一实测确认，每条追到"写什么状态/落什么库行"终局，全部为网络往返后持锁写入前最后一道身份闸。maybeRelogin 决策侧 :1208 与写回侧 :1254/:1265-1273 双闭合；手动五路 accountExists（:255/:305/:397/:497-512/:573）；写点换类五类全持锁、*Locked 写函数族 13 个双侧射证、无锁写点宿主 goroutine 唯一性成立。OBSERVE-117-01 第十九轮、B110-01 第二十六轮（手动 6 失败位 + 成功审计行 + 自动链失败族 + 零吞错穷举零命中 + token 脱敏抽查）、O105-01（四包定向 race 全绿 + 双回归锚绿）、LOW-132/133 回首核（时间基全量扫零残留，残余仅 reloginAt 写读同基自洽）全部实测绿。纵深两向收口无新发现。工作区零漂移。**建议 APPROVE**。