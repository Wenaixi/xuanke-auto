# R93 后端只读审查报告

审查对象：xuanke-auto HEAD `03b428a`（R92 收官，进度 93/256；本轮回合包含 allow_swap 注释澄清 ed6e237 / Select 锚点剥离 623c1ce）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round93-backend-findings.md），结束时 git status 洁净验证（并行前端代理产出 round93-frontend-findings.md，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 测试 / 定向回归 / go vet / go build / gofmt / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量 2053 行),api,accounts,store,session,zhidao(config,crypto,runtime,db 族),secure,config,runtime},db}、quit_shared.go、tray_quit_test.go、cmd/{bench,probe,logintest}。新契约角度本轮聚焦：**数据库 schema 与 store 读写列一致性 + 错误处理分支完整性 + 会话生命周期对称性 + 配置读写路径三方一致性 + 多账号探测调度完备性**。

> 注：工作树 git status 见 `?? archive/review-rounds/round93-frontend-findings.md`（并行前端代理产出，非本代理改动）；后端仓库文件零改动。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O93-01：probeIntervalFor 为仅测试锚定的生产死方法（scheduler.go:83，走读 + grep 实测）

**证据**：`grep -rn "probeIntervalFor(" backend --include="*.go" | grep -v _test` 全部命中仅三处——定义（:83）、内部转发到 probeIntervalForOpen（:86）、以及只出现在注释中的引用（:418 `probeIntervalFor` 提及、:1135 注释、:988 注释"probeIntervalFor 内部不再重取"）。**生产调用链是 tick 直接调 `s.probeIntervalForOpen(now, open)`（:991），从不经 probeIntervalFor**；probeIntervalFor 仅被 13 处 scheduler 测试锚定（scheduler_test.go:1010/1014/1020 等，测试该行为语义）。这是 B41 系重构把"单快照复用"经由 probeIntervalForOpen 收敛后的残留：方法签名保留 `s.openTimeFor("")` 内部取锁读取（与 tick 开头的单快照复用语义冲突），但因从不被生产调用而无实际影响。

**触发场景推演**：无运行时触发——零生产调用。未来维护者若想复用"探测间隔判定"而调 probeIntervalFor，会拿到一个与 tick 实际路径（probeIntervalForOpen + 开头快照）**语义略有不同**的实现（内部自行重取 open 单值，可能读到热改亚毫秒窗口内的新旧值，正是 :988-990 注释要避免的）。**危害面是维护误导，非行为缺陷**。

**修复建议（可选）**：删除 probeIntervalFor 并把 13 处测试改指 probeIntervalForOpen（纯函数更可测）；或保留并在注释标明"仅测试锚定，生产走 probeIntervalForOpen + 调用方单快照"。属清理级建议，零行为变更。**走读 + grep 实测**。

### O93-02：单二进制内嵌 ddddocr 资产体积权衡（native_ocr.go:15-22 + assets，走读）

**证据**：`//go:embed` 三资产实测 29.7MB——onnxruntime.dll 16,048,160 字节 / common_old.onnx 13,606,051 字节 / charsets_old.json 57,249 字节。配合 release.yml 的平台分流（Windows x64 CGO=1 内嵌 / Linux macOS CGO=0）与 native_ocr_stub.go（!windows‖!cgo → Available=false），Windows 用户得到"双击单 exe 开箱即用"，非 Windows 回退本地 Python 或 Vision。**这是双轨设计的既有定案**（CLAUDE.md「防逆向与引擎二选一冲突」契约），体积是换取"免 Python 免密钥"的既定代价。

**触发场景推演**：无行为问题——dumpIfDiff 按大小比对幂等释出（:95-103）、initMu.Once 懒加载只在首次识别才释出、release 分平台构建不重复携带。仅记录体量事实供未来体积优化决策（如需瘦身可把 onnxruntime.dll 抽出为随包下载，但会破坏"单文件便携"卖点）。**走读 + 实测统计**，非缺陷。

### O93-03（O92-02 延续复核）：logintest 识别引擎判定源与主程序分叉，维持低优先级不修

**证据**：logintest/main.go:57 仍走 `config.CaptchaEngineDefault()`（环境变量 + 编译默认），主程序 main.go:77-111 以 settings 持久化值覆盖。但三态回退链完整（logintest:59-68：ddddocr → 原生 → 本地 → Vision；vision → 直接云识别），且在 ddddocr 本地可用但管理员刻意切 vision 时才会分叉（运维诊断工具不参与抢课）。本轮复核确认 **O92-02 结论仍准确，无新触发面**——logintest 是纯诊断工具，且三态回退保证多数场景可用；对齐 settings 源的改动属可选增强，收益有限，**不推荐现在改**。记录在册关闭。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无新可疑项 | 本轮走读未发现需主控深度核实的存疑点；O93-01/02/03 均为已闭环的观察项。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续**（scheduler.go 全量逐点复核）：spawnChain 链顶双 ClientFor（:1392 + :1409-1415 二次判 + :1421 指针捕获）、六分支 sameClientFor（失效 :1493 / 成功 :1525 / 风控 :1555 / 窗口关闭 :1575 / 实时复核 ErrUnauthorized :1604 / 确证满员 :1639），maybeRelogin 决策侧（:1212 ClientFor 存在性）+ 写回侧（:1258），ProbeForAccount 回写段（:854 sameClientFor + :859-862 识别槽双条件），ProbeNow 全局帧（:967-971）/ probe() 全局帧（:1110-1120 无身份维度，全局语义正确），MarkDone（:1931）/ RemoveDone（:1999）/ SubmitAll（:1359）ClientFor 存在性，Restore 路径（RestoreDone:611 / RestoreTargets:529 / RestoreRefused:630 无网络不需身份）。`sameClientFor` 判 nil 恒 false（:208-214）、`clientIdentity` 用 reflect.ValueOf(c).Pointer()（:219-228）。**无新裸露写点**。 | 通过（实测 + 走读） |
| **B88-01 修复持续复核**：ProbeForAccount 回写段持锁先 sameClientFor（:854），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:859-862）；`s.acctData` nil 守卫在身份复核后写入前（:863-866）。probe_identity_test.go 三钉（:16/:116/:176）本轮定向 + 全量 race 回归全绿（第八轮）。 | 通过（实测） |
| **O92-01 allow_swap 注释澄清复核（ed6e237）**：schema.sql:29 注释「历史遗留死列（换课引擎已移除，保留以兼容旧库形状）」+ db.go:78-79 注释族「缺它=比换课引擎更旧的库」语义精确——git 溯源 6ac5978 引入（当时 store INSERT/SELECT 含该列 + maybeSwapLocked 换课引擎），后继移除后 schema 列与缺列清单保留，注释如实表述。全仓 grep 仅此两处 + db_test.go:40 造旧表。零行为变更（纯注释），db 测试全绿（TestMigrateAddsPublishMetaColumns 旧库还预置 allow_swap 列，:40）。 | 通过（实测 + git 溯源） |
| **数据库 schema↔store 读写列一致性**（本轮新角度）：schema.sql 7 表全列 vs store.go 全部 INSERT/SELECT 逐一核对——targets 读写 7 列（publish_id/class_id/course_name/priority/publish_name/begin_date，无 allow_swap 正确）；credentials/accounts/success/refused/task_log/settings/activation_codes/activations 全对齐；DeleteAccount 六表清行（:388-414）。settings 列 5 键全替换（SaveSettings:351-366 单事务）。**无列名错位/遗漏**。 | 通过（走读） |
| **O86-01 api 抖动基线第八轮**：后台全量 `go test -race -count=1 -p 1 -timeout 900s ./...` **11 包 exit 0**（api 21.204s / store 26.597s / scheduler 15.387s / zhidao 2.759s / accounts 1.927s / db 2.024s / config 1.268s，实测）；connectex 零样本（R86-R93 连续八轮零复现）；另跑非 race 全量家 ~26s 全绿、定向回归（迁移族+窗口状态+身份防线+恢复族）全绿。 | 通过（实测） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **错误处理分支完整性**（本轮新角度）：zhidao 层 httpDo 只重试 dial/write（:492-501，请求未到达安全重发），read 错误绝不重试（防双报）；IsReadErr 五形态（:519-548 EOF/短读/awaiting-headers 超时/reading-body 超时/RST-read）；scheduler 层 err 归并六分支（ErrUnauthorized/rateLimit/windowClosed/read /普通/实时复核 cErr），read 类文案区分「已发出可能已处理」；api 层手动报名/退选 ErrUnauthorized → MaybeRelogin 但不 MarkTokenValid（:348/:416 击穿退避防线注释）；CheckClassSelectable 快照缺失/过期放行交平台（:1878）。Windows 回环冷启动双保险（socket 预创建 + readyProbe 10 次轮询，handler_test.go:53-71/:139）。 | 通过（实测 + 走读） |
| **会话与凭证生命周期对称性**（本轮新角度）：session.Store 12h TTL + 5min 清扫（:73-87）、票据 5min 单次防重放（:118-138）、RevokeAccount 吊销账号全部会话（:207-215）；登上（issueSession MarkTokenValid :226 / LoginByPassword 落库 + 空壳失败摘除 :264-273）/重登（maybeRelogin 写回侧 + UpdateIDToken :1273）/吊销（DeleteAccount → RevokeAccount :1018）/删除（Accounts.Remove→PurgeAccount→Store.DeleteAccount→RevokeAccount memory-first，handler.go:1004-1018）全路径对称。**credentials 表与 sessions 库生命周期对齐无泄漏**。 | 通过（走读） |
| **配置读写路径三方一致性**（本轮新角度）：runtime.Config（Store.Get 拷贝 :35-39 / Update 原子 :42-48）↔ store.SaveSettings 单事务全替换（:351-366）↔ handleAdminConfig PUT 热改（:730-842）+ dispatchRuntimeConfig 热下发（:848-856）+ main 启动 LoadSettings 恢复（main.go:77-111）。vision_key 严格 enc: 前缀 + 明文拒绝加载（main.go:87-97）；captcha_concurrency 越界整体拒绝（:762）。**.env 模板含说明注释（config.go:142-161），无死配置字段残留（B40-02 已归档）**。 | 通过（走读） |
| **多账号探测调度完备性**（本轮新角度）：probe()（:1047）单飞 probing 置位 → 逐账号 AccountsWithTargets 起 goroutine（probeSem cap 4 结构化信号量 :1073-1081，ponytail 注释标注升级阀值）→ 全局 FindElectives 主体；ProbeNow（:948）HTTP 快照过期兜底（不被 per-account 信号量约束，注释明确"管理员强制最新 + 频率已受节流约束"）；ProbeForAccount 绝不写 lastProbe（:869-877 注释：防旁路全校节流）。三路在节流/单飞/并发上互补无冲突。 | 通过（走读） |
| **cmd 三工具契约一致性**（本轮复核）：bench 边界防御（:23-25 -n≤0 回退）、probe 用 XUANKE_PROBE_TOKEN env + SharedTransport()（:18-32）、logintest 三态引擎回退 + -limit 边界（:76-79）。与 O92-02/O93-03 记录一致。 | 通过（走读） |
| **契约 20 注释无轮次标签**（本轮复核）：grep `（第 N 轮）|(round\d|R\d*-` 全仓零残留，注释含协议锚点（B42-01/M86-01 等以契约号标注，非轮次历史）符合规范。 | 通过（实测） |

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:431-442），空快照不删槽、非空 beginTimes 才覆盖（:859-862 每账号 / :1110-1114 全校）；识别过期只影响展示层（StateForAccount:718 After 判定 → OpenTimeKnown），tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1015-1017）；TestOpenTimeRetainedAfterWindowClosed 固化为绿。 | 通过 |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:922-943）+ 主判据 10s 裕量（:1150）+ EmptyProbeRuns 入账同 10s 裕量（:1159）；StateForAccount:707 与 WindowClosed() 共用同一实现（:921-943）；关闭后识别槽保留（open_retain_test.go:43-46）+ 目标发布元数据持久化（:68-117）双钉。 | 通过 |
| **21（防寄生 = 指针身份比对）**：sameClientFor + clientIdentity（reflect.ValueOf(c).Pointer()）与矩阵 1-16 全闭合；round41 七分支测试族（TestDeletedAccountRebuiltSameNameChainDrops{RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull,RealtimeUnauthorizedDropsRelogin} + SuccessDropsInflight + DropsRelogin）全绿。 | 通过（回归实测绿） |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（LoginByPassword:244）+ gateWait 阻塞（Relogin:173）共享 gateMu/gateUsed 同一窗口计数；ResetGateForTest 仅测试（:82-87）；api 夹具批量注册撞闸门由 ResetGateForTest 兜底。 | 通过 |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handler 登录/配置/删除、accounts SaveCredential/加密失败、main 恢复各表失败）；TestStoreFailuresLogged（:86）/TestSetTargetsDeleteRefusedFailureLogged（:115）钉死。 | 通过 |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御（:29-35）；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:200-206）；tray_quit_test.go 三钉（顺序/解锁/nil-safe）。 | 通过 |
| **B40-02 + O92-01（死配置字段清理闭环）**：Config.OpenTime/TIME env 已移除 + .env 模板无该键（config.go:55-57 注释明确）；本轮复核 schema/db.go 注释族对 allow_swap 死列的标注，语义准确、缺列清单含它=比换课引擎更旧的库（ed6e237 零行为变更）。 | 通过 |
| **B41-02（tick 零值守卫让位于 WindowOpened）**：`open.IsZero() && !opened` 双条件（:1015-1017）；TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime（:3373）+ TestSubmitSuspendedWhenOpenTimeCleared 夹具 WindowOpened 默认 false（:1347）双钉，race 全绿。 | 通过（实测） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `cd backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，11 包 exit 0（api 21.204s / store 26.597s / scheduler 15.387s / zhidao 2.759s / accounts 1.927s / db 2.024s / config 1.268s / runtime 1.363s / secure 1.531s / session 1.452s / 根包 1.473s）；connectex 零样本 |
| 非 race 全量家（api/store/zhidao/session/accounts/secure/runtime/config/db） | 全绿（api 20.267s / store 2.241s / zhidao 1.342s 等） |
| 定向回归（迁移族 + 窗口状态 + 身份防线 + 恢复族 + 幽灵窗口 + 调度间隔） | `db 3.578s / scheduler 13.238s` 全绿 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，R90-01 延续；git 仓库版本 LF 合规零检出） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round93-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段八轮 race 回归全绿；probe_identity_test.go 三钉实测通过。
2. **O86-01 api 抖动基线第八轮零复现**（race 全绿、connectex 零样本）；M87-01 维持 MINOR + 注释兜底仍准确；O92-01 allow_swap 注释澄清经 git 溯源 + 测试复核语义精确、零行为变更；O92-02 logintest 引擎分叉维持低优先级不修（本轮复核关闭）。
3. **新角度扫查全绿**：schema↔store 读写列四表逐一核对零错位、错误处理分支完整性（read 不重试/双报防线/read 类文案区分）、会话与凭证全路径对称、配置三方一致（runtime/store/handler/main）、多账号探测三路调度完备——唯一发现 **probeIntervalFor 生产零调用死方法**（O93-01，测试锚定清理级）+ **内嵌资产体积权衡记录**（O93-02）。
4. **新增 OBSERVE 两条**（O93-01/02），无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求。

工作树后端文件洁净。本轮为纯观察轮（与 R90/R91/R92 同型），无代码修改建议提交。