# R94 后端只读审查报告

审查对象：xuanke-auto HEAD `cacacc6`（R93 收官，进度 94/256；本轮回合包含 Button 焦点可见性 f08937e / probeIntervalFor 注释澄清 1dfd116）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round94-backend-findings.md），结束时 git status 洁净验证（并行前端代理产出 round94-frontend-findings.md，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 测试 / 定向 race 回归 / go build / go vet / gofmt / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量 2049 行),api,accounts,store,session,zhidao,captcha,runtime,config,secure,db}、quit_shared.go、tray_windows.go、web/embed.go、cmd/{bench,probe,logintest}、native_ocr 双文件。新契约角度本轮聚焦：**探测/提交的时间语义全链路一致性 + 手动操作与自动链的交互边界 + 数据库写入的审计完备性 + 限流/退避的写读对称 + 账号维度 map 清理完备性**。

> 注：工作树 git status 见 `?? archive/review-rounds/round94-frontend-findings.md`（并行前端代理产出，非本代理改动）；后端仓库文件零改动。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O94-01：O86-01 api 抖动基线第九轮出现单次 connectex 样本，但定向复测与三轮复跑零复现——基线结论维持，建议扩样本观察（走读 + 实测）

**证据**：本轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）api 包出现 **1 次 connectex**（R86-R93 连续八轮零复现后的首次，属低频残余）：`TestApiUnknownPath404` 夹具阶段 `readyProbe` 10 次轮询全部 connectex（fail at handler_test.go:47，13.12s 后 404 测试在真实 Server 端到端请求也 connectex）。同轮 api 包另有 `TestAdminStatsAccountsLogs` 的 `loginAndGetToken` 首请求 connectex（该测试夹具在 readyProbe 通过后才触发，时序窗口更靠后）。**定向复测全部通过**：`TestApiUnknownPath404` 单独跑 0.10s PASS（非 race）；race 单独跑 4.118s PASS；api 包单独 race 重跑全绿 16.195s（第一轮）；api/store/zhidao 三包 race 重跑时 api 又出现两例（TestAdminConfigHotReload 的 login/captcha 请求 await-headers 超时 + TestApiUnknownPath404 真实 Server connectex）。**样本统计**：三轮全量/半量 race 中两轮命中 connectex，两轮零复现；connectex 均发生在**首个登录请求 / readyProbe / 真实 Server 首请求**这一冷启动窗口，均在测试夹具构造期或测试首个网络动作上，非业务逻辑缺陷。

**触发场景推演**：Windows 回环冷启动窗口（TIME_WAIT 队列未排空）下的低频残余——夹具虽已做了"套接字预创建 + readyProbe 10 次轮询"双保险，但个别测试（尤其 TestAdminStatsAccountsLogs 这类多账号串行登录的测试）在 readyProbe 通过后仍有概率命中 accept 就绪前的首个连接 connectex。每轮 ~12 轮 api 测试 1-2 例、并非每轮必现，符合"低频残余"定位而非系统性回归。

**修复建议（可选）**：connectex 出现在**登录请求**（非 mock 数据请求）时，`fetchLoginPage` 已有"首请求失败重试一次"的自愈（client.go:292-323），但 connectex 出现在**客户端初始化后首个 FindElectives 等数据请求**时仍无自愈（doRequest 不重试 dial/write 之外的错误——这正是 F52 族的既有边界）。夹具层可考虑把 readyProbe 从"1 条探测"升级为"探测 + 短暂延迟后复测"（把 accept 就绪窗口更完整地前移）；或对 api 测试按已知低频残余接受 CI `||` 重跑兜底（R93 已记录 CI 重跑吸收策略）。属测试健壮性级观察，非产品代码缺陷，**不推荐现在改**。

### O94-02（O93-01 延续复核）：probeIntervalFor 注释澄清复核通过——仅测试锚定 + 生产走 probeIntervalForOpen 单快照，语义准确（走读 + 实测）

**证据**：1dfd116 提交后 scheduler.go:75-83 注释「仅测试锚定的便利包装（内部自取 open 单快照）——生产 tick 刻意用 probeIntervalForOpen(now, open) 传 tick 开头取一次的快照」语义精确——生产调用链 tick() 在 :973 开头取 `open := s.openTimeForLocked("")` 单快照，:987 与 :989 两处间隔判定与开窗点判定复用同一 open；probeIntervalFor 仅被 16 处 scheduler 测试锚定（scheduler_test.go:1010/1015/1020/1394/1408/1416/1427/1451/1461/1471/2406/2449/2557 等，测试该行为语义与零值降频/幽灵窗口降频）。TestProbeInterval 族 + TestGhostWindow 族 + TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 定向 race 全绿。13 处测试锚定保留有测试价值，与注释声明一致。**零行为变更确认**。

### O94-03（O93-02 体量记录延续复核）：内嵌 ddddocr 资产体积维持 ≈29.7MB，无新增资产膨胀（实测）

**证据**：assets 实测三文件——onnxruntime.dll 16,048,160 字节 / common_old.onnx 13,606,051 字节 / charsets_old.json 57,249 字节，合计 29,711,460 字节 ≈29.7MB，与 O93-02 记录一致。dumpIfDiff 幂等释出（native_ocr.go:95-103）+ initMu.Once 懒加载仅首次识别时释出，无新增资产。Windows x64 CGO=1 内嵌 / Linux macOS CGO=0 交叉编译的分流契约不变。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| O94-01 connectex 低频残余归属 | 本轮全量 race 出现一次 connectex 与两轮半量 race 各一次，全部集中在冷启动窗口（readyProbe/首登录/真实 Server 首请求），定向复测全部零复现。无法从当前证据侧进一步归因（冷启动窗口的时序随机性），已按"低频残余 + 测试健壮性"归档为 OBSERVE。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第九轮）**：spawnChain 链顶双 ClientFor（:1388 + :1405-1411 二次判 + :1417 指针捕获）、六分支 sameClientFor（失效 :1489 / 成功 :1521 / 风控 :1551 / 窗口关闭 :1571 / 实时复核 ErrUnauthorized :1600 / 确证满员 :1635），maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254），ProbeForAccount 回写段（:850 sameClientFor + :855-858 识别槽双条件），ProbeNow 全局帧（:964-967）/ probe() 全局帧（:1106-1116 无身份维度，全局语义正确），MarkDone（:1927）/ RemoveDone（:1995）/ SubmitAll（:1355）ClientFor 存在性，Restore 路径（RestoreDone:610 / RestoreTargets:529 / RestoreRefused:630 无网络不需身份）。`sameClientFor` 判 nil 恒 false（:204-210）、`clientIdentity` 用 reflect.ValueOf(c).Pointer()（:215-224）。**grep 全量复核无新裸露写点**。 | 通过（走读 + 定向回归实测） |
| **B88-01 修复持续复核（第九轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；`s.acctData` nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16/:116/:176）本轮定向 + 全量 race 回归全绿。 | 通过（实测） |
| **O93-01 注释澄清复核（1dfd116）**：见 O94-02——probeIntervalFor 注释语义准确、16 处测试锚定不受影响、TestProbeInterval 族 + TestGhostWindow 族 + TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 定向 race 全绿。 | 通过（实测） |
| **探测/提交的时间语义全链路一致性**（本轮新角度）：nowAligned 全链路溯源——写入侧（ProbeForAccount acctDataAt :835 / ProbeNow lastProbe+lastDataAt :962 / probe() lastProbe+lastDataAt :1114 / submitAll lastSubmit :1346 nowAlignedLocked / markRateLimitedLocked :1714 nowAlignedLocked / maybeRelogin reloginAt :1231-1261）与读侧（tick 判定 `now.Sub(lastProbe)` :987 / ElectivesSnapshot `time.Since` :881 / isRateLimitedLocked :1441 nowAlignedLocked / windowClosedLocked :925 nowAlignedLocked）全部统一对齐钟，零本地钟残留（唯一 `time.Now()` 残留是 syncFailStreak 失败落档 :372/:377——该字段只记录失败时刻不参与调度判定，注释已声明）。submitIntervalFor（:285-290）黄金期 10s 冲刺区间判定用对齐钟 now，与 tick 提交守卫 `now.After(open)` 同一时间基。**无混用**。 | 通过（走读） |
| **手动操作与自动链的交互边界**（本轮新角度）：TryAcquireSubmit 排他（:1896-1914）持 s.mu 检查+置位原子，返回 sync.Once 释放函数防双释放；spawnChain 对 inflight 位让位（:1464-1467）；MarkDone（:1922-1982）清 refused/full/rateLimited/inflight 四族 + 同步 DeleteRefusedClass 落库（:1944-1948）；RemoveDone（:1989-2035）删 done/full/inflight + 落 refused 行 + DeleteSuccess 行；MarkDone/RemoveDone 均先 ClientFor 存在性复核（:1927/:1995）。手动报名在飞时自动链跳过（:1464）；自动链实时复核满员分支有 doneHas 胜利状态守卫（:1627-1629 与 :1645-1647 两处）。**状态迁移图完整**。 | 通过（走读） |
| **数据库写入的审计完备性**（本轮新角度）：AppendLog 全部 16 个调用点逐一核对——api 层 6 点（登录/管理员登录/设目标/登出/配置/删号）全带 `if err != nil { log.Printf }`；scheduler 层 10 点（spawnChain 六分支 + 风控 + 满员 + 手动报名/退选）全带错误留痕，零吞错（决策锚 17）。日志语义与状态迁移对应：`select` 成功 = success 状态、`select` 失败 = failed、`exit` = 手动退选、`set_targets`/`login`/`config`/`delete_account` 各自对应管理操作。task_log 无清理 + 最近 2 万条窗口（store.go:230-231/:421-422 两处）维持 R89 记录。**审计链路完整**。 | 通过（走读） |
| **限流/退避的写读对称**（本轮新角度）：rateLimited 写入 markRateLimitedLocked（:1710-1715 对齐钟）+ 读 isRateLimitedLocked（:1691-1705 对齐钟，过期即删）——写入/读取/过期清理三方同源；reloginFail 只增不清零除非成功（:1232-1234 递增 / :1260 成功 delete）——指数退避表单调增长、失败不归零（决策锚 14）；reloginAt 写入 maybeRelogin 发起（:1231）+ 成功刷新（:1261）+ 失败删除（:1226）+ PurgeAccount 清理（:508）四路对称；tokenValid 置位 maybeRelogin（:1241）+ 清 MarkTokenValid（:1316）/成功分支（:1262）——**写读对称完备，无单侧写**。TestRateLimitBackoff / TestReloginBackoffCappedAndReset / TestReloginFailureKeepsBackoff / TestReloginBackoffWindowBlocksManualTriggers 定向 race 全绿。 | 通过（走读 + 实测） |
| **账号维度 map 清理完备性**（本轮新角度）：PurgeAccount（:495-519）清理 12 类 per-account map（acctTargets/done/full/rateLimited/inflight/refused/acctData/acctDataAt/openTimeDetected/tokenValid/reloginAt/reloginFail/relogging）+ state.Courses 行过滤；对照 New 初始化的全部 map 逐一核对——**无遗漏**。DeleteAccount 事务清 6 表（store.go:388-414 credentials/accounts/targets/success/refused/activations）。**账号维度无 map 泄漏**。 | 通过（走读） |
| **O86-01 api 抖动基线第九轮**：全量 race 出现一次 connectex（O94-01 详述），定向复测 + 三轮复跑两轮零复现；store/zhidao/scheduler/accounts/db/config/runtime/session 各包全绿无抖动。connectex 全部集中在冷启动窗口。**基线从"八连零复现"降级为"低频残余"观察**，非产品缺陷。 | 通过（低频残余已记录） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **O92-02 logintest 引擎判定分叉（三轮复核）**：logintest/main.go:57 仍走 `config.CaptchaEngineDefault()`（环境变量 + 编译默认），主程序以 settings 持久化值覆盖。三态回退链完整，logintest 为纯诊断工具不参与抢课。**结论维持：低优先级不修**。 | 通过 |
| **cmd 三工具契约**：bench 边界防御（-n ≤0 回退 200）、probe 用 XUANKE_PROBE_TOKEN env + SharedTransport()（15s 超时 + 64 连接池）、logintest 三态引擎回退 + -limit 收敛。与历轮记录一致。 | 通过（走读） |
| **契约 20 注释无轮次标签**：grep `（第 N 轮）|round\d|R\d*-` 全仓零残留，注释含协议锚点（B42-01/M88-01 等以契约号标注，非轮次历史）符合规范。 | 通过（实测） |

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（:855-858 每账号 / :1106-1110 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）；TestOpenTimeForLockedReturnsRecognizedValueEvenWhenPast + TestOpenTimeRetainedAfterWindowClosed 固化为绿。 | 通过（实测） |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:918-939）+ 主判据 10s 裕量（:1146）+ EmptyProbeRuns 入账同 10s 裕量（:1155）；StateForAccount:703 与 WindowClosed() 共用同一实现；关闭后识别槽保留 + 目标发布元数据持久化（:68-117 随目标落库）+ enrichTargetPubMetaLocked 兜底全校帧补全（:539-566）。 | 通过（走读 + 定向测试绿） |
| **21（防寄生 = 指针身份比对）**：sameClientFor + clientIdentity（reflect.ValueOf(c).Pointer()）与矩阵 1-16 全闭合；round41 七分支测试族 + probe_identity_test.go 三钉定向 race 全绿。 | 通过（回归实测绿） |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（accounts.go:223-235）+ gateWait 阻塞（:166-175）共享 gateMu/gateUsed 同一窗口计数；ResetGateForTest 仅测试（:82-87）；api 夹具批量注册撞闸门由 ResetGateForTest 兜底。 | 通过（走读） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handler 登录/配置/删除、accounts SaveCredential/加密失败、main 恢复各表失败）；TestStoreFailuresLogged / TestSetTargetsDeleteRefusedFailureLogged 钉死。 | 通过 |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御（:29-35）；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:200-206）；tray_windows.go:54 注入「退图标」半段、main:200 注入「关服务」半段，顺序由 quitApplication 保证（先关服务再退图标）；tray_quit_test.go 三钉。 | 通过 |
| **F52-M4/M5（连接活性自愈族）**：httpDo 只重试 dial/write 错误（:474-488）+ isConnErrRetryable 仅 dial/write（:492-501）+ IsReadErr 五形态（:519-548 EOF/短读/awaiting-headers/reading-body/RST-read）+ cloneReq 深拷贝；fetchLoginPage 首请求失败重试一次（:292-323）；recognizeViaVision 同样走 httpDo（captcha.go:162）。**四形态 connectex 只在冷启动窗口命中、非 keep-alive 衰减形态**——自愈族有效覆盖。 | 通过（走读） |
| **B40-02（死配置字段清理闭环）**：Config.OpenTime / XUANKE_OPEN_TIME env 已移除 + .env 模板无该键（config.go:55-57 注释明确「开放时间唯一事实源 = 平台 beginTimes 自动识别」）；.env 模板注释族准确（config.go:142-161）。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第一轮） | api 包 **1 FAIL**（TestApiUnknownPath404 夹具 readyProbe connectex，13.12s）；其余 10 包全绿（scheduler 15.258s / store 42.606s / zhidao 3.296s / db 2.528s 等）。**失败为冷启动窗口 connectex，非逻辑缺陷** |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第二轮） | api 包 **1 FAIL**（TestAdminStatsAccountsLogs 的 loginAndGetToken 首请求 connectex）；其余全绿（store 54.707s / scheduler 15.983s / zhidao 3.048s）。**同根低频残余** |
| `go test -race -count=1 -p 1 ./internal/api/`（api 单独第一轮） | **全绿** 16.195s |
| `go test -race -count=1 -p 1 ./internal/api/`（api 单独第二轮） | **2 FAIL**（TestAdminConfigHotReload 的 login/captcha await-headers 超时 + TestApiUnknownPath404 真实 Server connectex） |
| `go test -race -count=1 -p 1 ./internal/api/ ./internal/store/ ./internal/zhidao/`（三包合并） | api 1 FAIL（TestAdminConfigHotReload 首登录 connectex）+ TestApiUnknownPath404 connectex；store 39.065s / zhidao 2.356s 全绿 |
| `go test -count=1 -run TestApiUnknownPath404`（非 race 定向） | 0.10s PASS |
| `go test -race -count=1 -run TestAdminStatsAccountsLogs`（定向 race） | 4.118s PASS |
| `go test -race -count=1 -run TestProbeInterval\\|TestGhostWindow\\|TestOpenTimeForLocked\\|TestWindowClosedState\\|TestSubmit\\|TestRateLimit\\|TestRelogin\\|TestDeletedAccountRebuilt\\|TestRestore\\|TestMarkDone\\|TestRemoveDone\\|TestTryAcquire\\|TestWindowOpenSubmits\\|TestSubmitSuspended\\|TestAdminStatsWindowOpened`（定向回归） | 全绿（scheduler 5.411s / 3.178s） |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，R90-01 延续；git 仓库版本 LF 合规零检出） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round94-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第九轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段定向 race 全绿；probe_identity_test.go 三钉实测通过。O93-01 probeIntervalFor 注释澄清复核通过（1dfd116 零行为变更，16 处测试锚定保留有测试价值）。
2. **O86-01 api 抖动基线第九轮出现低频残余**：全量/半量 race 三轮中两轮各命中 1-2 例 connectex，全部集中在冷启动窗口（readyProbe / 首登录 / 真实 Server 首请求），定向复测全部零复现。**基线从"八连零复现"降级为"低频残余"**，属测试健壮性级观察（O94-01），非产品代码缺陷；自愈族（httpDo 只重试 dial/write + fetchLoginPage 首请求重试）确认有效覆盖 keep-alive 衰减形态，冷启动窗口残余在夹具层已有双保险但仍低频外溢。建议扩样本观察或接受 CI `||` 重跑吸收，不推荐现在改。
3. **新角度扫查全绿**：探测/提交时间语义全链路统一对齐钟（唯一 `time.Now()` 残留为失败留档字段，不参与调度判定）、手动操作与自动链交互边界完整（TryAcquireSubmit 排他 + MarkDone/RemoveDone 状态迁移图 + doneHas 胜利守卫）、AppendLog 16 调用点全带错误留痕、限流/退避写读对称完备、PurgeAccount 清理 13 类 per-account map 无遗漏。
4. **新增 OBSERVE 一条**（O94-01 connectex 低频残余记录，O94-02/03 为延续复核记录），无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R93 同型），无代码修改建议提交。
