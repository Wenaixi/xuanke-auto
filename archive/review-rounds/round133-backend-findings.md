# R133 后端审查报告（绝对只读，身份防线矩阵第四十八轮）

- 日期：2026-09-24
- 基线：R132（975dc2b）已归档；backend/ 自 B110-01 修复后唯一产品改动 = LOW-132-01 时钟退避时间基修复（git diff 073f7fd --stat -- backend/ = scheduler.go 2 行 + scheduler_test.go 49 行，身份防线零改动）
- 模式：绝对只读（除本报告外零修改；唯一写文件为 archive/review-rounds/round133-backend-findings.md）
- 聚焦清单按记忆锚点逐项核验 + 2 个自选新契约纵深（时间盒 35 分钟）

## 验证表

| 项目 | 结果 | 证据 |
|------|------|------|
| go build ./... | 绿（exit 0） | 全部包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race（six 包联合） | 绿 | 借 /d/mingw64 gcc，CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/{zhidao,accounts,scheduler,store,session,api}/：3.590s/1.801s/15.444s/38.490s/1.835s/14.697s 全 ok；单包复跑 zhidao+accounts 2.205s/1.535s、scheduler 23.057s、api 48.441s、store+session+api 43.891s/3.612s/43.250s，全部零竞态 |
| 全量回归（无 race） | 绿 | scheduler 19.940s / api 16.390s ok；时钟族六测 + 回归锚双测 5.240s ok |
| gcc 探测 | /d/mingw64/bin/gcc.exe 存在 | where gcc 不在 PATH，按清单 ls 探测命中，全轮实测绿 |
| 契约 20 轮次标签扫描 | 零命中（产品代码） | 全仓扫 `第 *N* 轮`/`R..轮`/`round *N*`：仅测试断言叙述三处（scheduler_test.go 第 3 轮后等）与两处历史文档引用（session/store.go:117 docs/review-round13.md、tray_windows.go:99 R79）——均非决策历史轮次前缀标签，语义即 为什么/契约 本身，合规延续 |
| 零吞错穷举 | 零命中 | `_ =`（含 .*(AppendLog|Save|Delete|UpdateIDToken) 双形式）全仓（测试外）零命中；唯一 `_ =` 为 handler.go :387/:467 的 MarkDone/RemoveDone（调度器内存态方法返回恒 nil，非库写吞错）；scheduler/handler 两文件 29+ 处库写全数 if err := ...; err != nil { log } |
| LOW-132-01 回首核 | 修复在位且绿 | 见 §5 |

## 1. 身份防线矩阵第四十八轮闭合（无新增，延续纯观察）

- **sameClientFor 定义**：scheduler.go:204（clientIdentity :215，reflect.ValueOf(c).Pointer() 指针身份，nil 返回 0 恒非同一；注释 199-203 含同名重建/反射指针身份完整语义）。零漂移。
- **7 调用点逐一确认**（与 R130-R132 记录逐一对齐，全部为 spawnChain 六分支 + ProbeForAccount 探测回写，均位于网络往返后持锁写入前最后一道身份闸）：
  - :850 —— ProbeForAccount 回写段锁内，!sameClientFor(acct, client) 放弃写 acctData/openTimeDetected
  - :1489 —— ErrUnauthorized 分支，身份不符 delete inflight + 静默弃链（重登触发 :1499 落在身份复核之后，绝不为新身份消耗登录预算)
  - :1521 —— 成功分支写 done/state/SaveSuccess 前（:1529 s.done[acct][t.ClassID] = true)
  - :1551 —— 风控退避分支 markRateLimitedLocked 前
  - :1571 —— 窗口关闭分支 markFullLocked 前
  - :1600 —— 实时复核回锁后三路总闸（含存在性判定，第六个对称点）
  - :1635 —— 实时复核确证满员 markFullLocked 前（:1627 doneHas 胜利让位在前，绝不动手动成功状态）
  - 六分支 + 探测回写 = 7，零漂移。
- **写点换类抽查 5 类（本轮取前几轮未重复类别）**：
  - acctTargets：:457（SetTargetsForAccount）/ :474 / :531（RestoreTargets）全持锁；PurgeAccount :498 持锁全清
  - state.EmptyProbeRuns：:1156 递增 / :1158 归零（probe 锁内，幽灵窗口量变计数）
  - chains：:1393 / :1397 置位 + :1402 清位（独立 s.chainMu 保护，spawnChain 在飞互斥专属锁，非 s.mu——独立 map 独立锁正确配对）
  - state.WindowOpened / state.WindowClosed：:1127 / :1146 全持锁（probe 锁内）
  - openTimeDetected 全校槽：:1108 持锁写入（*）槽，与账号槽 :856 同族）
  - 五类全部持锁零裸写。
- **类级结构证据延续**：*Locked 后缀写函数族在列（openTimeForLocked/nowAlignedLocked/windowClosedLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/tokenValidForLocked/isRateLimitedLocked/markFullLocked/markRateLimitedLocked/releaseFullIfFreedLocked/statusIndexLocked/setStateLocked 等，全部持锁私有实现）；外部可调写函数（SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkTokenValid/TryAcquireSubmit/MarkDone/RemoveDone/RemoveFull/ProbeForAccount/ProbeNow/RecognizedOpenTime）全部函数体首行取锁。
- **无锁写点宿主 goroutine 唯一性射证延续**：reloginResults 通道唯一消费者 = Start() :666-684 for+select 单 goroutine 循环（:675-681 持 s.mu 写 lastProbe 补探测复位）；通道发送 :1279 非阻塞 select+default（通道满弃号不持锁阻塞）。唯一无锁写点 = maybeRelogin goroutine 开头 :1246-1247（锁内 delete relogging + ClientFor）。全域锁覆盖。
- **手动五路 accountExists**：handler.go :255（课程读）/ :305（手动报名）/ :397（手动退选）/ :497-510（目标写，内联 LoadCredentials 逐账号比对）/ :573（状态读）——五路全数在位；:1109-1120 定义判据同源（凭据表）；handleSetTargets 缺透传无核心账号时整体拒绝（:518-520），管理员名绝不落孤儿行。零漂移。
- **maybeRelogin 双侧**：决策侧 :1208 ClientFor(acct) 存在性复核（reloginMu 到 s.mu 锁序、任何 map 写入前，注释覆盖删号与在飞探测 ErrUnauthorized 同帧污染场景）；写回侧 :1254 ClientFor 复核 → :1265 重取当前注册表 client.Token() 落库（:1269 UpdateIDToken），已删账号整个成功分支静态放弃、只清 relogging（:1247）。与记录一致零漂移。

## 2. OBSERVE-117-01 知识位第十六轮 —— 确认在位

写回侧先 ClientFor 复核（:1254，已删整块放弃）→ 重取当前注册表 client.Token() 落库（:1265-1272，非空才 UpdateIDToken :1269，失败仅 log 不吞），绝不串旧身份；成功分支 delete reloginFail + 刷新 reloginAt + tokenValid=false（:1260-1262）。失败/未重登保持 tokenValid=true 展示已失效自动恢复中（:1286-1292 reloginFail 保留递增到次数、绝不无条件复位 1——指数退避不被击穿）。决策侧锁序 reloginMu 到 s.mu 与 TokenValidFor/MarkTokenValid 对齐（:1307-1318），手动登录与在途重登并发半态读杜绝。在位零漂移。

## 3. B110-01 审计链第二十三轮 —— 零漂移

- 手动失败六处 AppendLog 全数在位：handler.go :362（报名失效）/ :374（报名 read）/ :381（报名其余）/ :443（退选失效）/ :452（退选 read）/ :459（退选其余）。
- 成功路径审计行：:387（MarkDone 内部 :1976 AppendLog success）、:467（RemoveDone 内部 :2029 exit success）、:558（set_targets）。
- 自动链失败族：scheduler.go :1504（失效）/ :1532 / :1535（成功 AppendLog+SaveSuccess）/ :1558（风控）/ markFullLocked :1770（满员）/ :1614（实时复核失效）/ :1662（read/其余失败）全数带错误日志。MarkDone :1945 / :1973 / :1976、RemoveDone :2020 / :2026 / :2029、SetTargetsForAccount :476 / :484、UpdateIDToken :1269 全部持显式 err 分支。
- 零吞错穷举：`_ =`（含双下划线双形式）全仓（测试外）零命中，见验证表。

## 4. O105-01 抖动基线 —— 夹具在位 + 定向 race 全绿

- socketPreheat 定义于 zhidao/client_test.go:25，TestMain :39 包级调用 + 逐测试调用保留（:40/:52）；readyProbe 三包在位：zhidao（client_test.go:88，10x200ms 宽栅栏 + 2s 超时）、accounts（manager_test.go:26/:87/:164/:247，注释声明未 socketPreheat 双保险、8 轮全绿无残余）、api（handler_test.go:139/:185，轮询就绪）。captcha_test.go 亦调用。
- 定向 race 六包联合全绿（见验证表）。gcc 状态：where gcc 不在 PATH，/d/mingw64/bin/gcc.exe 命中。
- 回归陷阱双锚在位：TestWindowOpenSubmitsWithoutProbeReset（scheduler_test.go:1277）与 TestAdminStatsWindowOpenedUsesScheduler（api/handler_test.go:1011，含 window_closed 同源校验 :1036-1043）。

## 5. LOW-132-01 修复回首核 —— 修复在位且绿，无新混用

- :372 已改 s.lastSyncFailAt = s.nowAlignedLocked()（GO 修复核心，锁内取对齐钟）；:377 s.syncFailedWindow = s.nowAlignedLocked() 一并统一（留档写而不读，与失败/成功清零对称）。
- 判读侧 :342 now.Sub(s.lastSyncFailAt) < 30s 的 now 为调用方传入对齐钟 now（tick :974 now := s.nowAligned()）——写读同基准。
- 成功清位 :392/:394 用 time.Time{}（清零不混时间基）；syncFailStreak :371 ++ / :391 成功归零 / :384 达到 3 复位 clockOffset，判据使用 :925 单快照 open + nowAlignedLocked().After(open)。
- 全文件 lastSyncFailAt 引用仅 :342（判读）/ :372（写入）/ :392（清位），无新增混用点。
- 新测试 TestClockSyncFailureBackoffUsesAlignedClock（scheduler_test.go:1516）在位：预置 clockOffset=5s → 触发失败同步落地 → 双断言（failAt 约等于 alignedNow 偏差小于 500ms + 反证本地钟写入 localDiff 大于 -500ms 显形）；随时钟族六测一并通过（5.240s ok）。

## 6. 新契约角度（自选未覆盖纵深 ×2）

R131 走并发调度、R132 走登录闸门族/时间基，本轮选「手动 MarkDone/RemoveDone 与 doneHas 胜利让位」+「失败分级退避网络层自愈」双纵深：

- **doneHas 胜利让位全分支核对**：实时复核确证满员分支 :1627（done 一旦置位即让位，绝不被 markFullLocked 覆盖 success）+ 实时复核未现满员分支 :1645（不覆盖胜利状态）双闸；测试钉死：TestRealtimeFullRecheckKeepsManualSuccess（:2912，复核在飞期间 MarkDone 落地 + 快照改满 → 断言 success 保留 + done 保留 + full 不写入）+ 对偶 TestRealtimeFullRecheckWithNoManualDoneMarksFull（:2968，无手动介入照常记 full）。MarkDone :1940-1948 同步清理库内 refused 行（零吞错规范）+ :1949-1958 清 inflight/full/rateLimited；RemoveDone :1990 首行取锁 + :1995 非 ClientFor 防线（幽灵 refused 行不回写）+ :2017-2032 DeleteSuccess/SaveRefused/AppendLog 三落库全带 err 分支。契约 14/16 全数在位。
- **失败分级退避 + 连接活性自愈一次重试核对**：zhidao/client.go httpDo 只对 dial/write 错误重试一次（:474-489，注释重试即等效达成换新连接），read 错误不重试（服务端已消费 body 可能已处理，重发 POST 双报）——与 IsReadErr（:522 对称互斥）判型穿透 sanitizeErr Unwrap 后不变（sanitize_test.go 断言）。fetchLoginPage 瞬时抖动自愈一次（:287-291，纯 GET /login 不消耗验证码限额）。scheduler 侧失败分级：风控 → markRateLimitedLocked 30s（:1555）、窗口关闭 → markFullLocked（:1575）、token 失效 → maybeRelogin 异步重登（:1499）、其余 → 实时复核三路（:1589-1667），逐级收敛不轰炸。全数实证在位。

## 发现汇总（分级）

- CRITICAL：0
- HIGH：0
- MEDIUM：0
- **LOW：1（新增，契约观察级，非缺陷）**
- 维持观察（非新增）：maybePrewarm 无独立单测（R127 首提，行为经 tick 集成路径覆盖）、probeSem cap=4 常驻（R131）、go func() { _ = pw.Prewarm() }() 错误静默（预热失败由下个 15s 周期自然重试，与库写入零吞错规范不同域）。

### 新增发现

[Scheduler.go:780/795/801/1871] 快照 TTL 判读侧用本地钟 time.Since，写入侧用对齐钟——反向基准混用孤岛（与 R132 消灭的 写本地/读对齐 方向相反）
Confidence: MEDIUM
Issue: acctDataAt/lastDataAt 写入侧已统一对齐钟（:835/:864/:1116/:966 的 now := s.nowAligned()），而 snapshotTTL（40s）判读侧四处 time.Since(snappedAt)：:780/:795/:801/:1871 按本地钟计算——time.Since = time.Now().Sub()，未加 clockOffset。语义后果：acctDataAt 落在对齐钟、判读落在本地钟，经时评估 = 实际时长 + clockOffset（约 640ms），对 40s TTL 相对误差不超过 1.6%，快照过期边界约 0.6s 不可感，属契约打磨级（与 R132 LOW-132-01 同族、方向相反，R132 只收口 lastSyncFailAt 一处、本族未系统性收敛）。不产生实质错误行为，列为契约观察维持。
Fix: 如需与全族时间基准完全一致，四处均已在锁内（ElectivesSnapshotFor :780/:795/:801 持锁；CheckClassSelectable :1871 持锁），改 s.nowAlignedLocked().Sub(acctDataAt) 即可；或显式注释 TTL 容量性规则用真实墙钟表达、不参与对齐钟基准——二选一消除混用印象。无行为预期变化。

## 结论

身份防线矩阵第四十八轮闭合：7 个 sameClientFor 调用点逐一对齐零漂移，本轮换类 5 类写点（acctTargets/EmptyProbeRuns/chains/WindowOpened+Closed/openTimeDetected 全校槽）全持锁（chains 由独立 chainMu 保护正确配对）；*Locked 写函数族类级结构证据延续；无锁写点宿主 goroutine 唯一性射证延续（reloginResults 单一消费者 + 非阻塞发送）；手动五路 accountExists、maybeRelogin 双侧在位。OBSERVE-117-01 第十六轮、B110-01 第二十三轮、O105-01 均零漂移与实测绿。LOW-132-01 修复回首核在位且测试绿，无新混用。零产品改动链延续（第十一轮纯观察，唯一产品改动 = LOW-132-01 修复本身，归属时钟契约非身份防线）。新增 1 项 LOW 契约观察（快照 TTL 判读侧 time.Since 本地钟 vs 写入对齐钟，方向相反的反向混用，量级无感），无 CRITICAL/HIGH/MEDIUM。进度 134/256。
