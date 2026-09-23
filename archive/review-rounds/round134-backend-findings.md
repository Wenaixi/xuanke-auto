# R134 后端审查报告（绝对只读，身份防线矩阵第四十九轮）

- 日期：2026-09-24
- 基线：R133（d75f38c）已归档；backend/ 自 B110-01 修复后唯一产品改动 = LOW-132-01（时钟失败退避）+ LOW-133-01（快照 TTL 判读侧时间基）两个时钟契约修复，身份防线零改动（git show d75f38c：scheduler.go 仅 5 行 NOW/Sub 基准折叠；本报告走读亦核对改动白线）
- 模式：绝对只读（除本报告外零修改；唯一写文件为 archive/review-rounds/round134-backend-findings.md）
- 聚焦清单按记忆锚点逐项核验 + 2 个自选新契约纵深（时间盒 35 分钟内完成）
- 结论前置：零产品改动链延续（第十二轮纯观察），所有验证项全绿，新增发现 0，身份防线矩阵第四十九轮闭合

## 验证表

| 项目 | 结果 | 证据 |
|------|------|------|
| go build ./... | 绿（exit 0） | 全部包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race（六包联合两批 + 身份防线族） | 全绿 | 借 /d/mingw64 gcc：zhidao 2.204s / accounts 1.446s / scheduler 21.199s / store 21.680s / session 1.429s / api 14.440s 全 ok 零竞态 |
| 身份防线族 race | 全绿 | TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+TestRealtimeFullRecheck{KeepsManualSuccess,WithNoManualDoneMarksFull} 七测 race（1.616s）全 PASS |
| 时钟族+回归锚+窗口族（无 race） | 全绿 | scheduler 9.409s：TestSnapshotTTLUsesAlignedClock/TestClockSyncFailureBackoffUsesAlignedClock/TestWindowClosedState/TestGhostWindowEmptyProbesSuspend/TestWindowOpenSubmitsWithoutProbeReset 全 PASS；api 1.394s：TestAdminStatsWindowOpenedUsesScheduler PASS |
| gcc 探测 | /d/mingw64/bin/gcc.exe 存在 | where gcc 不在 PATH，按清单 ls 探测命中，全轮实测绿 |
| 契约 20 轮次标签扫描 | 零命中（产品代码） | 全仓扫 第 N 轮/R..轮/round N：仅测试断言叙述三处（scheduler_test.go:1659/:1662/:2518，描述测试逻辑轮次）+ 两处历史文档引用（session/store.go:117 docs/review-round13.md、tray_windows.go R79）——均非决策历史轮次前缀标签，语义即为什么/契约本身，合规延续 |
| 零吞错穷举 | 零命中 | `_ = .*(AppendLog|Save|Delete|UpdateIDToken)` 双形式（含 `_ = d.X.` / `_ = s.X.` 直接赋值忽略）全仓（测试外）零命中；唯一 `_ =` 为 handler.go :387/:467 的 MarkDone/RemoveDone（内存态返回恒 nil，非库写吞错）|
| LOW-132-01 / LOW-133-01 回首核 | 修复在位且绿 | 见 §5 |

## 1. 身份防线矩阵第四十九轮闭合（无新增，延续纯观察）

- sameClientFor 定义：scheduler.go:204（clientIdentity :215，reflect.ValueOf(c).Pointer() 指针身份比对，219-224 含 nil/非指针保护返回 0 恒非同一；注释 199-203 含同名重建/反射指针身份完整语义、需持 s.mu 声明）。零漂移。
- 7 调用点逐一确认（与 R131-R133 记录逐一对齐，全部为 spawnChain 六分支 + ProbeForAccount 探测回写，均位于网络往返后持锁写入前最后一道身份闸）：
  - :850 —— ProbeForAccount 回写段锁内，!sameClientFor(acct, client) 放弃写 acctData/openTimeDetected（注释 843-848 覆盖删号+同名重建场景，M88-01 身份防线）
  - :1489 —— ErrUnauthorized 分支，身份不符 delete inflight + 静默弃链（重登触发 :1499 落在身份复核之后，绝不为新身份消耗登录预算，注释 1494-1497）
  - :1521 —— 成功分支写 done/state/SaveSuccess 前（:1529 s.done[acct][t.ClassID]=true；注释 1515-1520 含重启 RestoreDone 恢复假成功完整语义）
  - :1551 —— 风控退避分支 markRateLimitedLocked 前（注释 1547-1550 假退避让黄金期被静默跳过语义）
  - :1571 —— 窗口关闭分支 markFullLocked 前（注释 1568-1569）
  - :1600 —— 实时复核回锁后三路总闸（注释 1596-1599 第六个对称点，内含存在性判定）
  - :1635 —— 实时复核确证满员 markFullLocked 前（:1627 doneHas 胜利让位在前，绝不动手动成功状态）
  - 六分支 + 探测回写 = 7，零漂移。
- 写点换类抽查 5 类（本轮取三族交替未重复类别）：
  - lastSubmit：:1346 持锁写入（submitAll 首行，nowAlignedLocked——对齐钟基准）+ :1029 判读侧读也持锁，写读同基准
  - lastSyncStart：:356 持锁写入（maybeSyncClock 快路径推进语义）
  - syncing：:355 置位 / :369 复位（全持锁，单飞守卫族）
  - lastProbe：:964/:1114 持锁写入（ProbeNow/probe）+ :676-679 reloginResults 消费者锁内复位 lastProbe=零值（唯一无锁写点的宿主 goroutine 专享）
  - state.EmptyProbeRuns：:1156 递增 / :1158 归零（probe 锁内，幽灵窗口量变计数）
  - 五类全部持锁零裸写。
- 类级结构证据延续：*Locked 后缀写函数族在列（nowAlignedLocked :273/openTimeForLocked :427/enrichTargetPubMetaLocked :539/rebuildCoursesForAccountLocked :570/rebuildCoursesLocked :647/tokenValidForLocked :739/windowClosedLocked :918/isRateLimitedLocked :1691/markRateLimitedLocked :1710/markFullLocked :1760/releaseFullIfFreedLocked :1781/statusIndexLocked :1835/setStateLocked :1846，全部持锁私有实现）；函数清单 grep 79 个方法中，全部外部可调写函数（SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkTokenValid/TryAcquireSubmit/MarkDone/RemoveDone/RemoveFull/ProbeForAccount/ProbeNow/RecognizedOpenTime 等）函数体首行取锁。
- 无锁写点宿主 goroutine 唯一性射证延续：reloginResults 通道唯一消费者 = Start() :666-684 for+select 单 goroutine 循环（:675-681 持 s.mu 写 lastProbe 补探测复位）；唯一无锁写点 = maybeRelogin goroutine 开头 :1244-1247 锁内 delete relogging + ClientFor 复核。通道发送 :1279 非阻塞 select+default（通道满弃号不持锁阻塞）。全域锁覆盖。
- 手动五路 accountExists：handler.go :255（课程读）/ :305（手动报名）/ :397（手动退选）/ :497-512（目标写，内联 LoadCredentials 逐账号比对）/ :573（状态读）——五路全数在位；:1109-1120 定义判据同源（凭据表）；accountExists 定义处 :1106-1108 注释"目标写/课程读判据同源，手动报名退选+状态读复用"。handleSetTargets 缺透传无核心账号时整体拒绝（:518-520），管理员名绝不落孤儿行。零漂移。
- maybeRelogin 双侧：决策侧 :1208 ClientFor(acct) 存在性复核（reloginMu 到 s.mu 锁序对齐、任何 map 写入前，注释 1202-1207 覆盖删号与在飞探测 ErrUnauthorized 同帧污染场景）；写回侧 :1254 ClientFor 复核 → :1265-1273 重取当前注册表 client.Token() 落库（:1269 UpdateIDToken，非空才落），已删账号整个成功分支静态放弃、只清 relogging（:1247）。失败/未重登保持 tokenValid=true 展示已失效自动恢复中（:1286-1297 reloginFail 保留递增到次数、绝不无条件复位 1——指数退避不被击穿）。与记录一致零漂移。

## 2. OBSERVE-117-01 知识位第十七轮 —— 确认在位

写回侧先 ClientFor 复核（:1254，已删整块放弃）→ 重取当前注册表 client.Token() 落库（:1265-1273，非空才 UpdateIDToken :1269，失败仅 log 不吞），绝不串旧身份；成功分支 delete reloginFail + 刷新 reloginAt + tokenValid=false（:1260-1262）。失败/未重登保持 tokenValid=true 展示已失效自动恢复中（:1286-1297 reloginFail 保留递增到次数、绝不无条件复位 1——指数退避不被击穿）。决策侧锁序 reloginMu 到 s.mu 与 TokenValidFor/MarkTokenValid 对齐（:1306-1319），手动登录与在途重登并发半态读杜绝。在位零漂移。

## 3. B110-01 审计链第二十四轮 —— 零漂移

- 手动失败六处 AppendLog 全数在位：handler.go :362（报名失效）/ :374（报名 read）/ :381（报名其余）/ :443（退选失效）/ :452（退选 read）/ :459（退选其余）。
- 成功路径审计行：:387（MarkDone 内部 :1976 AppendLog success）、:467（RemoveDone 内部 :2029 exit success）、:558（set_targets）；另全库统计共 12 处 AppendLog 调用点含登录/登出/配置/删号（handler.go :136/:237/:616/:864/:1055）全部带显式 err 分支。
- 自动链失败族：scheduler.go :1504（失效）/ :1532/:1535（成功 AppendLog+SaveSuccess）/ :1558（风控）/ markFullLocked :1770（满员）/ :1614（实时复核失效）/ :1662（read/其余失败）全数带错误日志。MarkDone :1945/:1973/:1976、RemoveDone :2020/:2026/:2029、SetTargetsForAccount :476/:484、UpdateIDToken :1269 全部持显式 err 分支。
- 零吞错穷举：`_ =`（含双下划线双形式、`_ = d.Sched.` / `_ = s.store.` 直接赋值忽略形态）全仓（测试外）零命中，见验证表。
- 网络层 token 脱敏延续抽查：zhidao/client.go :450 统一 sanitizeError 剥 URL（不泄露 ?idToken= 凭证）；scheduler.go :1283 maskedToken（:1334 只显前 8 位）登日志；登录链日志 :250/:274 只显账号/识别次数不显 token。

## 4. O105-01 抖动基线 —— 夹具在位 + 六包定向 race 全绿

- socketPreheat 定义于 zhidao/client_test.go:25（:19-23 注释含排空冷启动窗口语义），TestMain :40 包级调用 + 逐测试调用保留（:52、captcha_test.go:17/:52）；readyProbe 三包在位：zhidao（client_test.go:88，10x200ms 宽栅栏 + 2s 超时，:78/:164）、accounts（manager_test.go:26/:87/:164/:247，注释声明未 socketPreheat 双保险）、api（handler_test.go:139/:185，轮询就绪）、captcha_test.go:29/:79 亦调用。
- 六包定向 race（zhidao/accounts/scheduler/store/session/api）+ 身份防线族七测全绿（见验证表）。gcc 状态：where gcc 不在 PATH，/d/mingw64/bin/gcc.exe 命中。
- 回归陷阱双锚在位：TestWindowOpenSubmitsWithoutProbeReset（scheduler_test.go:1277）与 TestAdminStatsWindowOpenedUsesScheduler（api/handler_test.go:1011，含 window_closed 同源校验）。
- 本轮 race 增量纵深：跑 CGO_ENABLED=1 go test -race -run 身份防线族（1.616s）——身份防线族测试在 -race 下零竞态（覆盖全挂起/写回面）。O105-01 基线延续绿。

## 5. LOW-132-01 / LOW-133-01 修复回首核 —— 修复在位且绿，无新混用

- LOW-132-01（时钟失败退避）：:372 s.lastSyncFailAt = s.nowAlignedLocked()（失败落地，对齐钟）+ :377 syncFailedWindow = s.nowAlignedLocked()（写而不读留档，同基准）+ 成功清零 :392/:394 用 time.Time{}（清零不混时间基）；判读侧 :342 now.Sub(s.lastSyncFailAt) < 30s 的 now 为调用方传入对齐钟 now（maybeSyncClock 由 tick :981 传入 nowAligned）。写读同基准。
- LOW-133-01（快照 TTL 判读侧）：判读侧四处全改 nowAlignedLocked().Sub——ElectivesSnapshotFor :780/:795/:801、ElectivesSnapshot :881、CheckClassSelectable :1871（:780 快路径 / :795 专属帧过期路径 / :801 全局帧路径 / :881 全局读 / :1871 可报名复核）；写入侧 :835/:864/:1116/:966 已对齐钟。git show d75f38c 白线核对：5 处 time.Since 全折叠为 nowAlignedLocked().Sub，无其它改动。
- 时间基一致性全量扫：grep 全文件 time.Since/time.Now()（判读/写入混用残留）零命中——残余 time.Since 仅两处 reloginAt 退避判读（:1220/:1227，reloginAt 写入 :1231/:1261 用 time.Now() 本地钟，二者写读同基准自洽，不与对齐钟族混用）；time.Now() 仅出现于 nowAligned 定义 :269/:274 与 reloginAt 写点。
- 新测试两族在位且绿：TestSnapshotTTLUsesAlignedClock（:1516，43s 过 TTL 本地钟误判 fresh 正是 LOW-133-01 的 TDD 红化机制会话）+ TestClockSyncFailureBackoffUsesAlignedClock（:1543，预置 clockOffset=5s 断言 failAt≈alignedNow）。

## 6. 新契约角度（自选未覆盖纵深 ×2）

R131 走并发调度、R132 走登录闸门族/时间基、R133 走 doneHas 胜利让位/失败分级退避，本轮选「窗口状态三判据单源共用同一 open 快照 + StateForAccount 必读 windowClosedLocked 展示一致」与「登录闸门族 B42-01 双侧收口续核 + 网络层自愈判型对称互斥」双纵深：

- 窗口关闭判据单源纵深：windowClosedLocked（:918-946）对三条判据（主判据 state.WindowClosed / 时钟连续失败 >=3 且开放时间已过 / 幽灵窗口 EmptyProbeRuns>=3 且开放时间已过）统一取一帧 open 快照（:924 注释"一次读取避免识别值在两次读取间被覆盖"）——三判据不各自重取 open，热改亚毫秒一致性。probe() 写入侧 :1145 open 单快照复用两处 10s 裕量（:1146 主判据 / :1155 EmptyProbeRuns 入账），写入读同源。StateForAccount :703 必调 windowClosedLocked()（st.WindowClosed 实时计算）与 WindowClosed() 同真相——二层兜底判据（时钟/幽灵）命中时可视化 mirror 进 /api/state window_closed，悬挂与展示不分叉。测试族在位：TestWindowClosedState（:1800）、TestGhostWindowEmptyProbesSuspend（:2498）、TestStateWindowClosedMirror（幽灵窗口判据镜像断言上接 B29-02）。
- 登录闸门族双侧收口 + 网络自愈判型续核：accounts/manager.go gateTryAcquire（:223-235，非阻塞准入，与 gateWait 共享 gateMu/gateUsed 计数）在 LoginByPassword :244 收口；gateWait 仍只收口 Manager.Relogin（自动重登排队）；两侧共享全账号每分钟 doLogin 预算（gateLoginPerMin=2）。zhidao/client.go httpDo（:478-492）只对 dial/write 连接错误重试一次（isConnErrRetryable :496-505，net.OpError.Op 为 dial/write），read 错误绝不放行重发（IsReadErr :523-553 与 isConnErrRetryable 对称互斥——POST 双报/幂等凿穿防线）；sanitizeError :450 统一剥 URL 不泄露 idToken；fetchLoginPage 瞬时抖动自愈一次（:227）。全数实证在位。

## 发现汇总（分级）

- CRITICAL：0
- HIGH：0
- MEDIUM：0
- LOW：0（本轮无新增）
- 维持观察（非新增）：maybePrewarm 无独立单测（R127 首提，行为经 tick 集成路径覆盖）、probeSem cap=4 常驻（R131 ponytail 注释）、go func() 中 Prewarm 错误静默（预热失败由下个 15s 周期自然重试，与库写入零吞错规范不同域）、reloginAt 使用本地钟时间基（自洽写读，与对齐钟族分离设计，量级无感，R133 同族判断延续）。

## 结论

身份防线矩阵第四十九轮闭合：7 个 sameClientFor 调用点逐一对齐零漂移，本轮换类 5 类写点（lastSubmit/lastSyncStart/syncing/lastProbe/EmptyProbeRuns）全持锁；*Locked 写函数族类级结构证据延续（79 个方法清单核对外部写函数首行取锁）；无锁写点宿主 goroutine 唯一性射证延续（reloginResults 单一消费者 + 非阻塞发送）；手动五路 accountExists、maybeRelogin 双侧在位。OBSERVE-117-01 第十七轮、B110-01 第二十四轮、O105-01 均零漂移与实测绿。LOW-132-01/LOW-133-01 修复回首核在位且测试绿，时间基全量扫无新混用点。零产品改动链延续（第十二轮纯观察，产品改动仅为两个时钟契约修复，归属时间基域非身份防线）。新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW。进度 135/256。
