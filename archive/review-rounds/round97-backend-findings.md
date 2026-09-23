# R97 后端只读审查报告

审查对象：xuanke-auto HEAD `57ec7cb`（R96 收官，进度 97/256；R97 为纯观察轮延续——HEAD 与 R96 相同，无新代码提交）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round97-backend-findings.md）。并行前端代理产出 round97-frontend-findings.md（见 git status `??`，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 三轮 + 定向复现 / go build / go vet / gofmt / 前端 npm run build / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量),api(全量),store(全量),session,zhidao,accounts,db}、web/src/{App.tsx,Admin.tsx,api/client.ts,types.ts,lib/adminAuth.ts}。新契约角度本轮聚焦：**探测数据缓存与 TTL 语义（snapshotTTL/acctDataAt/ElectivesSnapshotFor 回退链边界）+ 手动操作状态迁移完整图 + 数据库迁移 schema 演进 + 前端管理员四端点（激活码/配置/日志/账号）与 Admin.tsx 逐字段交叉核对 + 错误文案端到端一致性**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O97-01（O86-01/O94-01/O95-01/O96-01 api 抖动基线第十二轮）：三轮全量 race 二绿一"zhidao 单包单次 FAIL"——低频残余归因，基线维持（实测）

**证据**：三轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）结果：第一轮全 11 包全绿（api 41.425s / scheduler 21.541s / store 23.804s / zhidao 3.769s）；第二轮 **仅 zhidao 包 FAIL 一次**（34.396s，异常耗时 15 倍于正常 2.3s，日志尾段为正常登录 mock 的 Vision 识别交错日志 + 一行 FAIL，无 panic / 无 DATA RACE / 无明确测试名输出）；第三轮全 11 包全绿（zhidao 2.311s）。

**失败归因**：紧随其后定向复现——zhidao 包独立全量 race 一次全绿、`-run 'TestLogin'` 六项全绿、count=3 全绿、-json 模式全绿。34.396s 的异常耗时符合"某登录 mock 测试在 Windows 回环冷启动窗口首请求 connectex 后进入重试/超时慢路径"特征（与 R94 两轮 1-2 例低频残余同族；client_test.go:40-113 的 socketPreheat + readyProbe 双防线已覆盖该族，偶发残余仍可能穿透）。**非产品逻辑缺陷，CI `-p 1` + 失败重跑吸收口径不变**。

### O97-02（O90-01 scheduler.go gofmt CRLF 噪音延续复核）：工作区 CRLF 转换噪音，git 仓库内容 LF 合规（实测）

**证据**：工作区 `internal/scheduler/scheduler.go` 被 `gofmt -l` 检出；`git show HEAD:backend/internal/scheduler/scheduler.go` 仓库版本为纯 LF（历轮实证），差异纯为 checkout 时 `core.autocrlf=true` 的 LF→CRLF 转换噪音，非源码缺陷，无需提交动作。历轮 O90-01 维持。

### O97-03（O96-06 契约 20 注释轮次标签回归）：全仓扫描仅一处文档路径引用，非违规（实测）

**证据**：grep `（第 \d+ 轮）|round\d\d|R\d\d-\d\d` 全仓仅命中 `backend/internal/session/store.go:117` 注释中的 `docs/review-round13.md` 文档引用（文件路径名，非轮次决策标签），不违反契约 20「代码注释严禁轮次前缀标签」字面。历轮固化回归维持。

### O97-04（O96-04/O80-01 删除保护撞名延续复核）：handleAdminDeleteAccount `IsAdminAccountName` 单判据与 B43-04 双条件不对称——历轮已声明维持观察（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只看账号名等于 `AdminNameValue()`。撞名场景（`XUANKE_ADMIN_NAME` 显式配成某学生学号）下该学生账号被删除保护永久覆盖；B43-04 已为登录路径处理同款撞名（双条件签发），但删除路径未对称。历轮归"低优先级不修"，维持观察。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无 | 三轮全量（二绿 + zhidao 单包单次低频残余已归因）+ 定向复现全绿 + 前端 build 全过 + 逐点走读闭环，无新增可疑项。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第十二轮）**：逐点 grep `sameClientFor`（scheduler.go:204，判 nil 恒 false + reflect 指针身份）+ 全量调用点——ProbeForAccount 回写段（:850）、spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口（:1600）/确证满员（:1635）六分支；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeNow（:964-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 路径（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）。**grep 全量复核无新裸露写点**。 | 通过（走读） |
| **B88-01 修复持续复核（第十二轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；acctData nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16/:116/:176）定向 + 全量 race 回归全绿。 | 通过（实测） |
| **探测数据缓存与 TTL 语义（本轮新角度）**：snapshotTTL=40s（:106）> 探测间隔保证 /electives 有数据可读；acctDataAt 时间戳由 ProbeForAccount 用对齐钟写入（:835/:864）、ElectivesSnapshotFor 判据 `time.Since(snappedAt) <= snapshotTTL`（:780）；**目标账号（len(acctTargets[acct])>0）专属帧过期 → 返回 (nil,false) 绝不回退全局帧**（:777-784，契约 8）；无目标纯浏览且从未有专属帧才允许回退全局帧（:786-804）；过期专属帧同样返回 false 触发刷新（:795-798，杜绝"全局帧被 order[0] 刷新为新鲜错年级帧"的年级串线）；CheckClassSelectable 快照过期一律放行交平台把关（:1858-1862），与 ElectivesSnapshotFor 过期语义对齐。**TTL 边界完整**。 | 通过（走读） |
| **手动操作状态迁移完整图（本轮新角度）**：handleElectiveSelect 链——凭据表校验（:295）→ 拒管理员名（:302）→ TryAcquireSubmit 在飞互斥（:318，占用 inflight 位让自动链跳过）→ CheckClassSelectable 快照复核（:327，无快照/过期放行）→ ClientFor 取客户端（:333）→ SelectClass（:340）→ ErrUnauthorized 走 MaybeRelogin 绝不 MarkTokenValid（:348-352，击穿指数退避防线）→ IsReadErr 区分"可能已处理"文案（:356）→ 成功 MarkDone（:363）。handleElectiveExit 对称（:396/:411/:417/:422/:431）。**MarkDone 解除 refused（内存 + 库行 DeleteRefusedClass，:1938-1948）保证手动重选成功即恢复自动接管**；RemoveDone 落 refused + 删 success 行（:2018-2028）保证重启不复活假成功。doneHas 复核"绝不覆盖胜利状态"在实时复核满员/未现满员分支双处（:1627/:1645）。**状态迁移穷举闭环，无死锁/无双包/无假成功路径**。 | 通过（走读） |
| **数据库迁移与 schema 演进（本轮新角度）**：migrateAddPublishMeta（db.go:43-57）逐列 columnExists 判存在、缺才 ALTER（纯增量安全）；Open 顺序"建表 → 迁移 → refuseLegacy"（:24-35），refuseLegacy 缺列清单对应剔除已迁移列（:77-79 注释明确 publish_name/begin_date 不在清单）；`columnExists` 用 `pragma_table_info` 防注入（:102）。TestMigrateAddsPublishMetaColumns TDD 守护。**迁移模式可复用、边界清晰**。 | 通过（走读） |
| **前端契约交叉核对（激活码/配置/日志/账号管理四端点与 Admin.tsx 逐字段，本轮新角度）**：`/api/admin/codes` GET 返回 ActivationCode（code/total_uses/used_uses/created_at，store.go:250-256）↔ Admin.tsx CodesTab `c.used_uses >= c.total_uses` 耗尽判定 + Badge 展示（:459-466）对齐；POST 生成返回 `string[]` ↔ `setGenerated(codes)`（:335）；DELETE 无 body 返回"请指定要删除的激活码"（handler.go:680）↔ 前端 DELETE 恒带 body（:354）兼容。`/api/admin/config` 六字段（activation_enabled/vision_base_url/vision_api_key_masked/vision_model/captcha_engine/captcha_concurrency，handler.go:714-722）↔ types.ts AdminConfig（:99-106）逐字段对齐；PUT 留空 key = 不改（前端 :539 `if (apiKey.trim())`）↔ 后端 `if strings.TrimSpace(*req.VisionAPIKey) != ""`（handler.go:776）语义一致。`/api/admin/stats` 十一字段（handler.go:947-965）↔ types.ts AdminStats（:109-129）逐字段对齐，window_closed 可选 + open_time_set `!== true` 保守判未识别（Admin.tsx:735/:740）与后端 open_time_set=!open.IsZero() 对齐。`/api/admin/accounts`（store.go:448-480 ListAdminAccounts 账号名+目标+成功，Go 切片 nil 序列化 null → 前端 `a.targets ?? []` 兜底 :845-846）对齐；`/api/admin/logs` limit=200（前端 :914）↔ LoadAllLogs limit 2000 上限（store.go:418）兼容。**四端点零字段错位**。 | 通过（走读） |
| **错误文案端到端一致性（本轮新角度）**：zhidao 层错误分类（ErrUnauthorized code=-1 / IsReadErr 四形态 / isRateLimitError"频繁/429/稍后重试" / isWindowClosedError"关闭/未开启/报名时间/已结束"）→ handler 层文案（手动报名/退选对 ErrUnauthorized 统一"教务令牌已失效，正在自动重登"、对 IsReadErr 统一"可能已处理，以选课大厅状态为准"）→ 前端 toast 直显 msg。scheduler 自动链（scheduler.go:1656-1659）与手动路径（handler.go:356-358）read 文案同源。**端到端一致**。 | 通过（走读） |
| **O97-01 抖动基线第十二轮**：三轮全量（二绿 + zhidao 单包单次 FAIL 归因低频残余）+ 定向复现全绿；store/zhidao/scheduler/accounts/db/config/runtime/session 各包正常。基线结论：低频残余由宿主环境冷启动窗口主导，非产品缺陷。 | 通过（实测，O97-01） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **零吞错复核（契约 17）**：本轮扫查 SpawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handler 登录/配置/删除、accounts SaveCredential/加密失败、main 恢复各表失败——全部 `if err != nil { log.Printf }` 零吞错。 | 通过 |
| **凭据/密钥加密**：凭据 AES-256-GCM 加密（secureEncrypt + enc: 前缀）；vision_key 严格要求 enc: 前缀、未加密旧值拒绝加载（main.go:87-97）；主密钥缺失拒绝启动；maskKey 回显脱敏（handler.go:704-712）。 | 通过（走读） |
| **多账号登录闸门（B42-01）**：gateTryAcquire 非阻塞（accounts.go:223-235）+ gateWait 阻塞（:49-64）共享 gateMu/gateUsed 同一窗口计数；LoginByPassword pre-Login 准入（:244）；ResetGateForTest 仅测试（:82-87）。双入口无绕行。 | 通过（走读） |
| **撞名双条件 + 管理态判定绑定 adminName（B43-04/F52-M1）**：handler.go:121 双条件签发 + loginTimingFlat 300ms 错误分支拉平（:124/:137）；撞名学生教务登录正常签发普通会话；前端 adminAuth.ts isCurrentAdminSession + 登录响应 adminName 标记 + logout/onDeleted/onUnauthorized/onBackToStudent 全路径清标记（App.tsx:91-132/:160-178/:187-235/:306-329 交叉核证）。 | 通过（走读） |
| **连接活性自愈族（F52-M4/M5）**：httpDo 只重试 dial/write（client.go:474-488）+ isConnErrRetryable 仅 dial/write（:492-501）+ IsReadErr 四形态（:519-548）+ cloneReq 深拷贝；fetchLoginPage 首请求失败重试一次（:292-323）；recognizeViaVision 同走 httpDo（captcha.go:162）。 | 通过（走读） |

## 契约抽查表（抽查 10 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（:855-858 每账号 / :1106-1110 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）；识别槽无值回退遗留 openTime 字段（:435-437）。 | 通过（实测） |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:918-939）+ 主判据 10s 裕量（:1146）+ EmptyProbeRuns 入账同 10s 裕量（:1155）；StateForAccount:703 与 WindowClosed() 共用同一实现；关闭后识别槽保留 + 目标发布元数据持久化（SetTargetsForAccount :473 enrichTargetPubMetaLocked + 落库）+ RestoreTargets 兜底全校帧补全（:531）。TestWindowClosedState + TestGhostWindowEmptyProbesSuspend + TestGhostWindowClockFailuresSuspend 全绿。 | 通过（走读 + 定向测试绿） |
| **4/21（删号 memory-first + 防寄生 = 指针身份比对）**：handleAdminDeleteAccount（handler.go:1004-1018）顺序 Remove → PurgeAccount → DeleteAccount → RevokeAccount 与契约 4 一致；sameClientFor + clientIdentity（reflect 指针）与矩阵全闭合；round41 七分支测试族 + probe_identity_test.go 三钉定向全绿。 | 通过（回归实测绿） |
| **B41-01/B41-02（零值守卫让步 + 全分支身份族）**：spawnChain 六分支 + 实时复核第七分支 + 探测回写全部闭合；tick 零值守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）。TestSubmitSuspendedWhenOpenTimeCleared 保持绿。 | 通过 |
| **8（ElectivesSnapshotFor 回退链）**：目标账号专属帧过期 → (nil,false) 触发真刷新绝不回退全局帧（:777-784）；目标判据 `len(acctTargets)>0`（空 slice 不算有目标，:777）；无目标且从未有专属帧才允许回退全局帧（:786-804）。 | 通过（走读） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（历轮清点 + 本轮复核主要写点）。 | 通过 |
| **7（?account= 透传全路径凭据表校验）**：handleElectives:245 / handleElectiveSelect:296 / handleElectiveExit:373 / handleSetTargets:461 / handleState:537 五处 accountExists 全覆盖，查无此账号整体拒绝；accountExists 用 LoadCredentials 更强真理源（:1073-1084）。 | 通过（走读） |
| **B43-04 + F52-M1（撞名双条件 + 管理态判定绑定 adminName）**：handler.go:121 双条件签发；前端 adminAuth.ts isCurrentAdminSession + login 响应 adminName 标记 + 全路径清标记。 | 通过（走读） |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:200-206）；tray_windows.go 注入「退图标」半段、main:200 注入「关服务」半段，顺序由 quitApplication 保证。tray_quit_test.go 钉死。 | 通过 |
| **B42-01（doLogin 全局频率闸门）**：gateTryAcquire 非阻塞准入 + gateWait 阻塞共享 gateMu/gateUsed；LoginByPassword pre-Login 收口；ResetGateForTest 仅测试。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第一轮） | **全 11 包全绿**（api 41.425s / scheduler 21.541s / store 23.804s / zhidao 3.769s 等），api 包零 connectex |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第二轮） | 11 包中 10 包全绿，**zhidao 包单次 FAIL**（34.396s，无 panic/无 DATA RACE，O97-01 低频残余归因） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第三轮） | **全 11 包全绿**（zhidao 2.311s / api 24.464s / store 40.838s 等） |
| `go test -race -count=1 -p 1 ./internal/zhidao/ -v`（定向复现） | 全绿（2.790s，23 项 PASS + 1 SKIP TestNativeOcrClassifiesRealCaptcha） |
| `go test -race -count=3 -p 1 ./internal/zhidao/`（三连复现） | 全绿（TestLogin 六项每轮全 PASS） |
| `go test -race -count=1 -p 1 ./internal/zhidao/ -json`（json 定位） | 全绿，零 fail 事件 |
| `go test -race -count=1 -run 'TestLogin' ./internal/zhidao/`（登录定向） | 全绿（6 项 PASS） |
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，O90-01 延续；git 仓库版本 LF 合规——O97-02 实证） |
| `cd web && npm run build`（tsc -b + vite build） | 通过（1948 modules / 468ms / dist 产物正常） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round97-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第十二轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段复核通过；probe_identity_test.go 三钉实测通过；round41 七分支测试族定向全绿。
2. **O86-01/O94-01/O95-01/O96-01 api 抖动基线第十二轮**：三轮全量 race 二绿一"zhidao 单包单次 FAIL"（34.396s 异常耗时 + 定向复现全绿，归因 Windows 回环冷启动窗口低频残余，与 R94 同族）。**基线维持回升态：CI `-p 1` + 失败重跑吸收口径不变**，属测试健壮性级观察（O97-01），非产品缺陷。
3. **新角度扫查全绿**：探测数据缓存与 TTL 语义边界完整（snapshotTTL 40s / 目标账号专属帧过期绝不回退全局帧 / CheckClassSelectable 过期放行交平台把关）；手动操作状态迁移完整图穷举闭环（TryAcquireSubmit → CheckClassSelectable → SelectClass/ExitClass → MarkDone/RemoveDone 各分支含 ErrUnauthorized 走重登绝不清退避、read 类"可能已处理"文案、doneHas 绝不覆盖胜利状态、MarkDone 解除 refused 恢复自动接管）；数据库迁移模式可复用（migrateAddPublishMeta 纯增量安全 + refuseLegacy 边界清晰）；前端四端点契约逐字段零错位（激活码/配置/日志/账号管理 ↔ Admin.tsx/types.ts，含 Go 切片 nil 序列化 null 的前端 `?? []` 兜底）；错误文案端到端一致（zhidao 分类 → handler 文案 → 前端 toast 同源）。
4. **新增 OBSERVE 一条**（O97-01 zhidao 单包单次低频残余归因 + 基线记录）、延续复核三条（O97-02 CRLF 噪音 / O97-03 契约 20 回归 / O97-04 删除保护撞名），**无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求**。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R96 同型），无代码修改建议提交。
