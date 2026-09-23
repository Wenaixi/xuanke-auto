# R135 后端审查报告（绝对只读，身份防线矩阵第五十轮——里程碑轮）

- 日期：2026-09-24
- 基线：R134（37be8b2）已归档；backend/ 自 B110-01 修复后唯一产品改动 = LOW-132-01（时钟失败退避）+ LOW-133-01（快照 TTL 判读侧时间基）两个时钟契约修复（git log backend/internal/scheduler/scheduler.go：d75f38c + 975dc2b，均归属时钟时间基域，身份防线零改动）。
- 模式：绝对只读（唯一写文件为本报告；全仓库其余零修改）
- 聚焦清单按记忆锚点逐项核验 + 里程碑轮「全家福」视角复盘 + 2 个自选新纵深（时间盒 35 分钟内完成）
- 结论前置：身份防线矩阵第五十轮闭合，所有验证项全绿，新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW，进度 136/256

## 验证表

| 项目 | 结果 | 证据 |
|------|------|------|
| go build ./... | 绿（exit 0） | 全部包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race（六包） | 全绿 | 借 /d/mingw64 gcc：zhidao 2.663s / accounts 1.611s / scheduler 15.209s / store 16.404s / session 1.627s / api 17.116s 全 ok 零竞态 |
| 身份防线族 race | 全绿 | TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+TestRealtimeFullRecheck{KeepsManualSuccess,WithNoManualDoneMarksFull}+TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin+TestSuccessDropsInflight 十测 race（2.776s）全 PASS |
| 探测定时族+闸门族 race | 全绿 | TestMaybeReloginDeletedAccountSkipsMaps / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime / TestRealtimeRecheckUnauthorizedTriggersRelogin / TestLoginByPasswordRejectsWhenGateBudgetExhausted（accounts）2.056s 全 PASS |
| 回归锚双测+interval clamp | 全绿 | TestWindowOpenSubmitsWithoutProbeReset + TestAdminStatsWindowOpenedUsesScheduler + TestScheduleIntervalClamped（scheduler_test.go:3646）race 2.458s PASS |
| 时钟族+迁移族 | 全绿 | TestSnapshotTTLUsesAlignedClock / TestClockSyncFailureBackoffUsesAlignedClock / TestWindowClosedState / TestGhostWindowEmptyProbesSuspend / TestMigrateAddsPublishMetaColumns（db_test.go:34）PASS |
| gcc 探测 | /d/mingw64/bin/gcc.exe 存在 | where gcc 不在 PATH，按清单 ls 探测命中，全轮实测绿 |
| 契约 20 轮次标签扫描 | 零命中（产品代码） | 全仓扫 第 N 轮/R..轮/round N：仅测试断言叙述三处（scheduler_test.go:1659/:1662/:2518，描述测试逻辑轮次）+ 两处历史文档引用（session/store.go:117 docs/review-round13.md、tray_windows.go R79）——均非决策历史轮次前缀标签，合规延续 |
| 零吞错穷举 | 零命中 | 双形式（含直接赋值忽略）全仓（测试外）零命中；唯一非库写合规静默：handler.go :387/:467 MarkDone/RemoveDone + scheduler.go:307 Prewarm / :1075 ProbeForAccount |
| LOW-132-01 / LOW-133-01 回首核 | 修复在位且绿 | 见 6 |

## 1. 身份防线矩阵第五十轮闭合（里程碑轮全家福复盘，零新增零漂移）

### 1.1 sameClientFor 定义与 7 调用点逐一确认

- 定义：scheduler.go:204（sameClientFor）→ clientIdentity :215（reflect.ValueOf(c).Pointer() 指针身份比对；219-224 含 nil/非指针保护返回 0 恒非同一）。注释 199-203 覆盖同名重建语义与需持 s.mu 声明。grep 全仓 clientIdentity( 仅 :209/:215 两处，无旁路比对。零漂移。
- 7 调用点（里程碑轮全家福视角——每条追到写什么状态/落什么库终局）：
  - :850 —— ProbeForAccount 回写段锁内，!sameClientFor 放弃写 acctData/openTimeDetected（注释 843-848 覆盖删号+同名重建+年级串线语义，M88-01 防线）。探测回写侧唯一裸露写点单点覆盖。
  - :1489 —— ErrUnauthorized 失效分支，身份不符 delete inflight + 静默弃链。重登触发（:1499）落在复核之后，绝不消费新身份登录预算。
  - :1521 —— 成功分支写 done/state/SaveSuccess 前（:1529 done=true + :1535 SaveSuccess）。注释 1515-1520 覆盖重启 RestoreDone 恢复假成功语义。
  - :1551 —— 风控退避分支 markRateLimitedLocked（:1555）前。注释 1547-1550 假退避让黄金期被静默跳过语义。
  - :1571 —— 窗口关闭分支 markFullLocked（:1575）前。注释 1565-1569。
  - :1600 —— 实时复核回锁后三路总闸（存在性判定内含于 sameClientFor）。注释 1596-1599 六路对称点语义。
  - :1635 —— 实时复核确证满员 markFullLocked（:1639）前。前置 doneHas 胜利让位（:1627，手动成功绝不覆盖）。
  - 六分支 + 探测回写 = 7，逐一与 R131-R134 记录逐位对齐，零漂移。
- 家族全景闭合清点：spawnChain 六 err 归并路径 + 探测回写，共 7 个身份闸，全部位于网络往返之后、状态/库行写入之前，无一裸露写点。grep 全仓 sameClientFor( 调用点 = 7（不含定义），与注释承诺一致。

### 1.2 写点换类抽查 5 类（全家福视角——类级证据向 + 写点覆盖向双侧交叉）

- lastSubmit：:1346 持锁写入（submitAll 首行，nowAlignedLocked）+ :1029 判读侧持锁读，写读同基准。
- lastSyncStart：:356 持锁写入（maybeSyncClock 快路径推进语义）+ :405 持锁清零（空库复位）。
- syncing：:355 置位 / :369 复位 / :403-404 空库复位，全持锁，单飞守卫族。
- lastProbe：:964/:1114 持锁写入（ProbeNow/probe）+ :676-681 reloginResults 消费者锁内写零值（唯一无锁写点的宿主 goroutine 专享）。
- state.EmptyProbeRuns：:1156 递增 / :1158 归零（probe 锁内，幽灵窗口量变计数）。
- 五类全部持锁零裸写。
- 全家福结构证据延续：*Locked 后缀写函数族全列（nowAlignedLocked:273 / openTimeForLocked:427 / enrichTargetPubMetaLocked:539 / rebuildCoursesForAccountLocked:570 / rebuildCoursesLocked:647 / tokenValidForLocked:739 / windowClosedLocked:918 / isRateLimitedLocked:1691 / markRateLimitedLocked:1710 / markFullLocked:1760 / releaseFullIfFreedLocked:1781 / statusIndexLocked:1835 / setStateLocked:1846）= 13 个 + PurgeAccount（:495 首行取锁）+ RemoveFull（:2038）+ CheckClassSelectable/TryAcquireSubmit/MarkDone/RemoveDone 全首行取锁。双向射证成立，零裸写点。
- 无锁写点宿主 goroutine 唯一性射证延续：reloginResults 通道唯一消费者 = Start() :666-684 for+select 单 goroutine 循环（:675-681 持 s.mu 写 lastProbe）；唯一无锁写点 = maybeRelogin goroutine 开头 :1244-1247 锁内 delete relogging + ClientFor 复核。通道发送 :1278-1281 非阻塞 select+default。全域锁覆盖。

### 1.3 手动五路 accountExists（handler.go）

- :255（课程读）/ :305（手动报名）/ :397（手动退选）/ :497-512（目标写，内联 LoadCredentials 逐账号比对）/ :573（状态读）——五路全数在位。:1109-1120 定义判据同源（凭据表）。handleSetTargets 缺透传且无核心账号时整体拒绝（:518-520），管理员名绝不落孤儿行。零漂移。

### 1.4 maybeRelogin 双侧

- 决策侧 :1208 ClientFor(acct) 存在性复核（reloginMu 到 s.mu 锁序对齐、任何 map 写入前，注释 1202-1207）。测试 TestMaybeReloginDeletedAccountSkipsMaps（scheduler_test.go:3477）实测四 map 零残留。
- 写回侧 :1254 ClientFor 复核 → :1265-1273 重取当前注册表 client.Token() 落库（:1266 非空才 UpdateIDToken :1269），已删账号整个成功分支静态放弃、只清 relogging（:1247）。失败/未重登保持 tokenValid=true（:1286-1297 reloginFail 保留递增，指数退避不被击穿）。两路都绝不串旧身份。零漂移。

## 2. OBSERVE-117-01 知识位第十八轮 —— 确认在位

写回侧先 ClientFor 复核（:1254）→ 重取当前注册表 client.Token() 落库（:1265-1273，非空才 UpdateIDToken :1269），绝不串旧身份；成功分支 delete reloginFail + 刷新 reloginAt + tokenValid=false（:1260-1262）。决策侧锁序 reloginMu 到 s.mu 与 TokenValidFor（:730-736）/MarkTokenValid（:1312-1319）三方对齐——手动登录与在途重登并发半态读杜绝。在位零漂移。

## 3. B110-01 审计链第二十五轮 —— 零漂移

- 手动失败六处 AppendLog 全数在位：handler.go :362/:374/:381（报名）/ :443/:452/:459（退选）。
- 成功路径审计行：:387（MarkDone 内部 :1976）、:467（RemoveDone 内部 :2029）、:558（set_targets）；另全库统计共 12 处 AppendLog 调用点含登录/登出/配置/删号（handler.go :136/:237/:616/:864/:1055）全部带显式 err 分支。
- 自动链失败族：scheduler.go :1504/:1532/:1535/:1558/ markFullLocked :1770/ :1614/:1662 全数带错误日志。MarkDone :1945/:1973/:1976、RemoveDone :2020/:2026/:2029、SetTargetsForAccount :476/:484、UpdateIDToken :1269 全部持显式 err 分支。
- 零吞错穷举：`_ =`（含双下划线双形式、直接赋值忽略形态）全仓（测试外）零命中，见验证表。store.AppendLog :201-209 单 INSERT Exec + return err（无吞错路径）。
- 网络层 token 脱敏延续抽查：zhidao/client.go :581 sanitizeError 剥 URL（B101-01）；scheduler.go :1283 maskedToken（:1334 只显前 8 位）；登录链日志 :250/:274 只显账号/识别次数不显 token。

## 4. O105-01 抖动基线 —— 夹具在位 + 六包定向 race 全绿

- socketPreheat 定义于 zhidao/client_test.go:25，TestMain :39-42 包级调用 + 逐测试调用保留（:52、captcha_test.go 同款）；readyProbe 三包在位：zhidao（client_test.go:88，10x200ms + 2s 超时）、accounts（manager_test.go 各测试内全数就绪探测，注释 :24-26 声明未 socketPreheat 双保险）、api（handler_test.go:139 + readyProbe :185）。
- 六包定向 race 全绿（见验证表）。gcc 状态：where gcc 不在 PATH，/d/mingw64/bin/gcc.exe 命中，全轮实测 CGO_ENABLED=1 定向 race 绿。
- 回归陷阱双锚在位：TestWindowOpenSubmitsWithoutProbeReset（scheduler_test.go:1277）与 TestAdminStatsWindowOpenedUsesScheduler（api/handler_test.go:1011）。
- 本轮 race 增量纵深：身份防线族十测 + 探测定时族 + 闸门族 + interval clamp 在 -race 下全绿（2.776s / 2.056s / 2.458s / 1.966s）。O105-01 基线延续绿。

## 5. 新契约角度（里程碑轮自选纵深 x2）

### 5.1 重登写回侧最终安全网（OBSERVE-117-01 第十八轮的另一半）

maybeRelogin goroutine 写回段（:1244-1298）是网络往返后持锁写状态+落库的最后一个独立宿主——其防护拓扑为：入口锁内 delete relogging（:1247）+ ClientFor 复核（:1254）→ 成功分支再取一次 ClientFor（:1265）重取当前 client.Token()（非空才 UpdateIDToken :1269）。里程碑核对：WriteBack 是否可能写回旧身份 token？——不可能。复核（:1254）先确认账号仍存在；成功分支（:1265）二次 ClientFor 拿到的是当前注册表身份，其 Token() 是重登后的新值（zhidao.SetCredentials/Login 已更新），绝不是旧链/旧身份遗留值。即使同名重建发生（旧链挂起 2 分钟 Vision 期），重建身份重新注册后 ClientFor 已指向新客户端，:1269 UpdateIDToken 落的是新身份的新 token——防火墙语义正确。锁序 :1246-1292 全程先 s.mu 再 store 调用，绝无无锁链。单测 TestDeletedAccountRebuiltSameNameChainDropsRelogin 已固化（register rebuild + 幽灵 relogin 零消耗）。

### 5.2 恢复链全序核对（main.go:117-141）

- 恢复顺序：RestoreDone(success)（:117-121）→ 循环 LoadTargetsForAccount(a) + RestoreTargets(a, ts)（:122-134）→ LoadRefused() + RestoreRefused(refused)（:135-141）——注释 :129-131/:135-136 明言 RestoreTargets 不清 refused、RestoreRefused 注入后不被后续覆盖，与决策契约 6 逐字对齐。
- 关键语义核对：RestoreTargets（scheduler.go:525-533）首行取锁、只 enrichTargetPubMetaLocked + rebuildCoursesForAccountLocked，绝不清 refused 内存/库行（对比 SetTargetsForAccount :458 delete refused + :484 DeleteRefused 库行）。rebuildCoursesForAccountLocked :579-591 内 restored 课程命中 refused 时强制 pending+已手动退选文案（refused 优先于 done，注释 :586-587）。RestoreDone :607-619 首行取锁。RestoreRefused :626-644 首行取锁 + 注入后校对 pending 文案（:637-643）。main.go 顺序与三函数语义完全闭环，恢复链全序契约成立。
- 实测：TestRestoreDoneSkipsResubmit（scheduler_test.go:567）PASS。

## 6. LOW-132-01 / LOW-133-01 修复回首核 —— 修复在位且绿，无新混用

- LOW-132-01：:372 lastSyncFailAt = nowAlignedLocked()（失败落地）+ :377 syncFailedWindow = nowAlignedLocked()（写而不读留档）+ 成功清零 :392/:394 用 time.Time{}（清零不混时间基）；判读侧 :342 now.Sub(lastSyncFailAt) < 30s 的 now 为调用方传入对齐钟 now。写读同基准。
- LOW-133-01：判读侧四处全改 nowAlignedLocked().Sub——ElectivesSnapshotFor :780/:795/:801、ElectivesSnapshot :881、CheckClassSelectable :1871；写入侧 :835/:864/:1116/:966 已对齐钟。
- 时间基一致性全量扫：grep 全文件 time.Since/time.Now() 零混用——残余 time.Since 仅两处 reloginAt 退避判读（:1220/:1227，写入 :1231/:1261 用 time.Now() 本地钟，二者写读同基准自洽）；time.Now() 仅出现于 nowAligned 定义 :269/:274 与 reloginAt 写点。zhidao SYNC RTT（client.go:135）为单次 RTT/2 本地无态近似（读 server Date 头，非 scheduler 时钟状态机，不属同一族）。accounts gateWait/gateTryAcquire（manager.go:53/:71/:226）本地钟自洽（每分钟翻页语义，非开窗点判定）。零混用残留。
- 新测试两族在位且绿：TestSnapshotTTLUsesAlignedClock（:1516）/ TestClockSyncFailureBackoffUsesAlignedClock（:1543）/ TestClockSyncNoRetryWithinBackoff（:1589）/ TestClockSyncFailureResetsOffset（:1636）全 PASS。

## 7. 里程碑轮全家福清点汇总

- 身份防线矩阵：R135 起 50 轮，B110-01 修复后身份防线零产品改动（第十三轮纯观察延续）。7 调用点 + 定义 + 家族全景全绿。
- 前端 M-1 / F93-01：R134 前端报告已归档（round134-frontend-findings.md 第七十轮闭合），本轮为后端轮不再重复。
- 时间基、窗口三判据、闸门族、网络自愈、连接活性、登录时序攻击族（B43-04 / F52-M1）均已在前轮闭合，本轮复核接口无漂移。

## 发现汇总（分级）

- CRITICAL：0
- HIGH：0
- MEDIUM：0
- LOW：0（本轮无新增）
- 维持观察（非新增，与前轮记录一致）：maybePrewarm 无独立单测（R127 首提）、probeSem cap=4 常驻（R131 ponytail 注释）、Prewarm 错误静默（下个 15s 周期自然重试）、reloginAt 使用本地钟时间基（自洽写读、量级无感）。探测定时三处对 ErrUnauthorized 静默调用 maybeRelogin（内部已记日志，历轮维持观察）。

## 结论

身份防线矩阵第五十轮（里程碑轮）闭合：sameClientFor 定义（scheduler.go:204）与 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一确认零漂移；全家福视角下六分支 + 探测回写全部为网络往返后持锁写入前最后一道身份闸，无裸露写点。写点换类 5 类（lastSubmit/lastSyncStart/syncing/lastProbe/EmptyProbeRuns）全持锁；*Locked 写函数族类级结构证据延续（13 个 + 外部写函数首行取锁双向射证）；无锁写点宿主 goroutine 唯一性射证延续。手动五路 accountExists（:255/:305/:397/:497-512/:573）、maybeRelogin 双侧（:1208/:1254）在位。OBSERVE-117-01 第十八轮、B110-01 第二十五轮、O105-01 均零漂移与实测绿。LOW-132-01/LOW-133-01 修复回首核在位且测试绿，时间基全量扫无新混用点。零产品改动链延续（第十三轮纯观察）。新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW。进度 136/256。
