# R96 后端只读审查报告

审查对象：xuanke-auto HEAD `2cb84af`（R95 收官，进度 96/256；本轮回合为纯观察轮延续，R96 无新代码提交——HEAD 与 R95 相同）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round96-backend-findings.md）。并行前端代理产出 round96-frontend-findings.md（见 git status `??`，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 三轮 + 定向 race 回归 / go build / go vet / gofmt / 前端 npm run build / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量),accounts,api,store,session,zhidao} 逐点、native_ocr 双文件资产体积、前端 web/src/api/client.ts + types.ts 契约交叉核对。新契约角度本轮聚焦：**登录链路完整状态机（LoginByPassword/Relogin/手动激活/MarkTokenValid 各状态迁移）+ 会话与票据生命周期 + doRequest/SelectClass/FindElectives 平台契约再核**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O96-01（O86-01/O94-01/O95-01 api 抖动基线第十一轮）：三轮全量 race 零复现——基线维持"零复现"观察（实测）

**证据**：本轮三轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）全部 11 包全绿（第一轮 api 22.954s / 第二轮 api 21.521s / 第三轮 api 23.759s），**api 包零 connectex**。定向 race 回归（`-run 'TestMaybeRelogin|TestDeletedAccountRebuilt|TestProbeIdentity|TestWindowOpen|TestWindowClosed|TestOpenTime|TestSuspended|TestAdminStatsWindowOpened'` scheduler 全绿 + api 全绿；`-run 'TestLogin|TestActivate|TestAdmin|TestRateLimit|TestCSRF|TestAPINotFound'` api 19.925s 全绿）。样本统计：本轮 3 轮全量全绿 + 定向全绿，结合 R94"两轮 1-2 例低频残余"与 R95"零复现"，残余确系宿主环境冷启动窗口随机外溢，非产品逻辑触发面。**基线结论维持：CI `-p 1` + 失败重跑吸收口径不变。**

### O96-02（O93-02 内嵌 ddddocr 资产体积延续复核）：≈29.7MB 无新增膨胀（实测）

**证据**：assets 实测三文件——onnxruntime.dll 16,048,160 字节 / common_old.onnx 13,606,051 字节 / charsets_old.json 57,249 字节，合计 29,711,460 字节 ≈29.7MB，与 O93-02/O94-03/O95-04 记录一致。dumpIfDiff 幂等释出 + initMu.Once 懒加载，无新增资产。Windows CGO=1 / Linux macOS CGO=0 分流契约不变。

### O96-03（O90-01 scheduler.go gofmt CRLF 噪音延续复核）：git 仓库内容 LF 合规，工作区 CRLF 转换噪音（实测）

**证据**：工作区 `internal/scheduler/scheduler.go` 为 `CRLF:2049 LF:0`（全行 CRLF）；`git show HEAD:...` 仓库版本为 `CRLF:0 LF:2049`（全 LF 合规），对 git 版本 `gofmt -l` **零输出**（GOFMT_GIT_EXIT=0）。差异纯为 checkout 时 `core.autocrlf=true` 的 LF→CRLF 转换，非源码缺陷，无需任何提交动作。历轮 O90-01 维持。

### O96-04（O95-02 删除保护撞名延续复核）：handleAdminDeleteAccount `IsAdminAccountName` 单判据与 B43-04 双条件不对称——历轮已声明维持观察（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只看账号名等于 `AdminNameValue()`。撞名场景（`XUANKE_ADMIN_NAME` 显式配成某学生学号）下该学生账号被删除保护永久覆盖；B43-04 已为登录路径处理同款撞名。与 O95-02 同记录，历轮维持观察。

### O96-05（O92-02 logintest 引擎判定分叉延续复核）：三轮复核后维持——logintest 为纯诊断工具不参与抢课（走读）

**证据**：cmd/logintest/main.go:57 仍走 `config.CaptchaEngineDefault()`（环境变量 + 编译默认），主程序以 settings 持久化值覆盖。历轮归"低优先级不修"，维持观察。

### O96-06（契约 20 注释轮次标签延续复核）：仅一处历史引用非违规（实测）

**证据**：grep `（第 \d+ 轮）|round\d|R\d*-` 全仓仅命中 `backend/internal/session/store.go:117` 注释中的 `docs/review-round13.md` 文档引用（文件路径名，非轮次决策标签），不违反契约 20「代码注释严禁轮次前缀标签」字面。R93/R94 固化回归检查维持。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无 | 本轮三轮全量全绿 + 定向回归全绿 + 前端 build 全量通过 + 登录状态机逐状态走读闭环 + 契约逐条核证，无新增可疑项。O94-01 connectex 残余本轮连续零复现（见 O96-01），已归档为"低频残余 + CI 重跑吸收"。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第十一轮）**：逐点 grep `sameClientFor`（scheduler.go:204，判 nil 恒 false + reflect 指针身份）+ 全量调用点——ProbeForAccount 回写段（:850）、spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口（:1600）/确证满员（:1635）六分支；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeNow（:964-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 路径（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）。**grep 全量复核无新裸露写点**。对应测试族（`TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull,RealtimeUnauthorized,SuccessDropsInflight}` + `TestMaybeReloginDeletedAccountSkipsMaps` + probe 三钉）定向 race 全绿。 | 通过（走读 + 定向 race 实测） |
| **B88-01 修复持续复核（第十一轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；acctData nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16/:116/:176）定向 + 全量 race 回归全绿。 | 通过（实测） |
| **登录链路完整状态机（本轮新角度）**：`handleLogin`（handler.go:102-159）管理员双条件签发（`Account==adminName && ConstantTimeCompare`，正确口令分支与错误分支等时 loginTimingFlat，:121-140）→ 学生 LoginByPassword（gateTryAcquire 非阻塞准入，:132）→ 已激活 issueSession（MarkTokenValid 清 tokenValid/reloginFail/relogging，:226）→ 未激活 CreateTicket（:153）+ handleActivate（:181-215）ConsumeTicket 先销毁再 ConsumeActivationCode → issueSession。状态迁移完整：LoginByPassword 成功后 wasShell 判定失败清理（manager.go:253-275 绝不误删持有效 token 的工作客户端）；ReloginIfNeeded 用客户端内部 account/password 重登自己（client.go:566-577，与调用方参数无关杜绝交叉污染）；maybeRelogin 决策侧（:1208 ClientFor 复核）+ 写回侧（:1254 复核后 UpdateIDToken 落库，:1269-1272）；MarkTokenValid 与 maybeRelogin 对齐 reloginMu→s.mu 锁序（:1307-1318）杜绝手动登录与在途重登并发状态闪动；tokenValid 唯一的自动清零路径是 maybeRelogin 成功分支（:1262）+ MarkTokenValid（手动登录恢复路径），失败保留 reloginFail 计数绝不无条件复位。**全部状态迁移正确，无绕行/击穿**。 | 通过（走读 + 定向测试绿） |
| **会话与票据生命周期**：session.Store TTL 12h（main.go:162）+ 5min 清扫协程（store.go:58-70）+ Close sync.Once 防泄漏；ticket TTL 5min 单次（ConsumeTicket 存在/未用/未过期/绑定账号四校验 + used 置位销毁，防重放穷举，store.go:118-138）；激活码 ConsumeActivationCode 单事务原子扣减（SQLite 单写者无超卖）+ 已激活账号不扣次；handleActivate 先 ConsumeTicket 再校验激活码（防穷举）。RevokeAccount 锁内遍历吊销被删账号全部会话。**边界完整**。 | 通过（走读） |
| **doRequest/SelectClass/FindElectives 平台契约再核（本轮新角度）**：doRequest（client.go:414-464）URL 后附 `?idToken=`（QueryEscape）+ Cookie 头统一注入（idToken 参数 + zd_edu_cookie 双通道，平台按 Cookie 严格鉴权）；code=-1 统一返回 ErrUnauthorized（:460-461）不再由调用方逐点解析；SelectClass（:730-750）form `classId=` + `Code!=0||!IsOk` 双判对齐平台 checkResp 契约；FindElectives（:648-683）空 body 首探 + 学期列表兜底重试 + 空快照直接返回不陷入兜底；parseElectives（:686-727）code:0 空 publishes 返回空快照 + 顶层 beginTimes 毫秒数组；StudentCounts（:792-818）`ids=` 逗号分隔 + CountEntry 三键实证（maxCount 平台未下发恒 0，IsClassFull 实际退化"永不确证满员"，真满员主路径是快照 classFullInSnapshot——与 CLAUDE.md 契约一致）。**平台契约全对齐**。 | 通过（走读） |
| **前端契约交叉核对（/api/electives、/state、/targets、/admin/* 与 client.ts/types.ts 逐字段）**：后端 ElectivesData 的 BeginTimes/Publishes/Class JSON tag 与前端 types.ts ClassItem/Publish/ElectivesData 逐字段对齐；SchedulerState（open_time/open_time_known/window_opened/window_closed/token_valid/courses，前端 window_closed 可选）对齐；Target JSON（publish_id/class_id/course_name/priority）对齐；AdminConfig（activation_enabled/vision_base_url/vision_api_key_masked/vision_model/captcha_engine/captcha_concurrency 六字段）对齐；AdminStats（open_time/activation_on/window_opened/window_closed/account_count/targets_count/success_count/log_count/vision_model/vision_base_url/captcha_engine/captcha_concurrency/open_time_set/token_valid，后端 handler.go:947-965）对齐（前端 captcha_engine/captcha_concurrency/open_time_set/token_valid 均可选，后端恒下发）；LogEntry/AdminLog 字段对齐（后端 LogEntry 含 account 字段但前端 LogEntry 类型不含——前端按账号隔离消费不读该字段，类型不消费即可，非缺陷）；selectElective/exitElective 返回 `{msg, class_id}` 与后端 map 对齐。**全量端点零字段错位**。 | 通过（走读） |
| **O95-01 api 抖动基线第十一轮**：三轮全量 race 全绿零 connectex，定向回归全绿；store/zhidao/scheduler/accounts/db/config/runtime/session 各包全绿。基线结论：低频残余/零复现交替由宿主环境波动主导，非产品缺陷。 | 通过（实测，O96-01） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **httpDo 自愈族复核**：httpDo 只重试 dial/write（client.go:474-488）+ isConnErrRetryable（:492-501）+ IsReadErr 四形态（:519-548）+ cloneReq 深拷贝；fetchLoginPage 首请求失败重试一次（:292-323）；recognizeViaVision 同走 httpDo（captcha.go:162）。**与 F52 族契约一致**。 | 通过（走读） |
| **零吞错复核（契约 17）**：spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handler 登录/配置/删除、accounts SaveCredential/加密失败、main 恢复各表失败——全部 `if err != nil { log.Printf }` 零吞错。 | 通过 |
| **凭据/密钥加密**：凭据 AES-256-GCM 加密（secureEncrypt + enc: 前缀）；vision_key 严格要求 enc: 前缀、未加密旧值拒绝加载（main.go:87-97）；主密钥缺失拒绝启动；maskKey 回显脱敏（handler.go:704-712）；不存明文。 | 通过（走读） |
| **恢复序/数据安全**：main.go RestoreDone → 逐账号 LoadTargetsForAccount+RestoreTargets（不清 refused）→ LoadRefused+RestoreRefused；SetTargetsForAccount 只清 refused 绝不清 done/full/rateLimited/inflight；RestoreRefused 注入"已手动退选"文案且拒绝覆盖 success。数据库迁移规范：migrateAddPublishMeta 先于 refuseLegacy、缺列才 ALTER。 | 通过 |
| **多账号登录闸门（B42-01）**：gateTryAcquire 非阻塞（accounts.go:223-235）+ gateWait 阻塞（:49-64）共享 gateMu/gateUsed 同一窗口计数；LoginByPassword pre-Login 准入（:244）；ResetGateForTest 仅测试（:82-87）。双入口无绕行。 | 通过（走读） |
| **HTTP 基础设施状态码**：六类基础设施路径（panic 500 / 会话 401 / 管理 403 / 限流 429 / CSRF-403 / 未知 API 404）全写真实状态码；业务 100+ 点恒 200（前端契约只读 body code）。 | 通过 |
| **时延拉平与撞名双条件（B43-04/F52-M1）**：handler.go:121 双条件签发 + loginTimingFlat 300ms 错误分支拉平（:124/:137）；撞名学生教务登录正常签发普通会话；前端 adminAuth.ts isCurrentAdminSession + 登录响应 adminName 标记 + 全路径清标记。 | 通过（走读） |

## 契约抽查表（抽查 9 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（:855-858 每账号 / :1106-1110 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）；识别槽无值回退遗留 openTime 字段（:435-437，main.go 传零值后恒零）。TestOpenTimeForLockedReturnsRecognizedValueEvenWhenPast（scheduler_test.go:1481）+ TestOpenTimeRetainedAfterWindowClosed（open_retain_test.go:16）固化为绿。 | 通过（实测） |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:918-939）+ 主判据 10s 裕量（:1146）+ EmptyProbeRuns 入账同 10s 裕量（:1155）；StateForAccount:703 与 WindowClosed() 共用同一实现；关闭后识别槽保留 + 目标发布元数据持久化（SetTargetsForAccount :473 enrichTargetPubMetaLocked + 落库）+ RestoreTargets 兜底全校帧补全（:531）。TestWindowClosedState（scheduler_test.go:1724）+ TestGhostWindowEmptyProbesSuspend（:2422）+ TestGhostWindowClockFailuresSuspend（:2501）全绿。 | 通过（走读 + 定向测试绿） |
| **4/21（删号 memory-first + 防寄生 = 指针身份比对）**：handleAdminDeleteAccount（handler.go:1004-1018）顺序 Remove → PurgeAccount → DeleteAccount → RevokeAccount 与契约 4 一致；sameClientFor + clientIdentity（reflect 指针）与矩阵全闭合；round41 七分支测试族 + probe_identity_test.go 三钉定向 race 全绿。 | 通过（回归实测绿） |
| **B41-01/B41-02（零值守卫让步 + 全分支身份族）**：spawnChain 六分支 + 实时复核第七分支 + 探测回写全部闭合；tick 零值守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）。TestSubmitSuspendedWhenOpenTimeCleared 保持绿（夹具 WindowOpened 默认 false）。 | 通过 |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（历轮清点 + 本轮复核主要写点）。 | 通过 |
| **7（?account= 透传全路径凭据表校验）**：handleElectives:245 / handleElectiveSelect:296 / handleElectiveExit:373 / handleSetTargets:453 / handleState:537 五处 accountExists 全覆盖，查无此账号整体拒绝；accountExists 用 LoadCredentials 更强真理源（:1073-1084）。 | 通过（走读） |
| **B43-04 + F52-M1（撞名双条件 + 管理态判定绑定 adminName）**：handler.go:121 双条件签发；前端 adminAuth.ts isCurrentAdminSession + login 响应 adminName 标记 + logout/onDeleted/onUnauthorized/onBackToStudent 全路径清标记（App.tsx 交叉核证）。 | 通过（走读） |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御（:29-35）；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:200-206）；tray_windows.go:54 注入「退图标」半段、main:200 注入「关服务」半段，顺序由 quitApplication 保证。tray_quit_test.go 钉死。 | 通过 |
| **F52-M4/M5（连接活性自愈族）**：httpDo 只重试 dial/write（:474-488）+ isConnErrRetryable 仅 dial/write（:492-501）+ IsReadErr 四形态（:519-548）+ cloneReq 深拷贝；fetchLoginPage 首请求失败重试一次（:292-323）；recognizeViaVision 同走 httpDo（captcha.go:162）。**四形态 connectex 只在冷启动窗口命中、非 keep-alive 衰减形态——自愈族有效覆盖**。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第一轮） | **全 11 包全绿**（api 22.954s / scheduler 15.901s / store 62.385s / zhidao 4.687s 等），**api 包零 connectex** |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第二轮） | **全 11 包全绿**（api 21.521s / scheduler 15.441s / store 28.783s 等），**api 包零 connectex** |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第三轮） | **全 11 包全绿**（api 23.759s / scheduler 15.324s / store 57.099s 等），**api 包零 connectex** |
| `go test -race -count=1 -run 'TestMaybeRelogin\|TestDeletedAccountRebuilt\|TestProbeIdentity' ./internal/scheduler/`（身份防线定向） | 全绿（8 项 PASS，3.038s） |
| `go test -race -count=1 -run 'TestLogin\|TestActivate\|TestAdmin\|TestRateLimit\|TestCSRF\|TestAPINotFound' ./internal/api/`（登录/激活/管理定向） | 全绿（19.925s） |
| `go test -race -count=1 -run 'TestWindowOpen\|TestWindowClosed\|TestOpenTime\|TestSuspended\|TestAdminStatsWindowOpened' ./internal/scheduler/ ./internal/api/`（窗口判定定向） | 全绿（scheduler 3.976s / api 1.844s） |
| `go test -race -count=1 -run 'TestProbeIdentity\|TestProbeDeleted\|TestProbeForAccountDrops\|TestProbeChainSame' ./internal/scheduler/`（probe 三钉定向） | 全绿（2.194s） |
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，O90-01 延续；git 仓库版本 LF 合规零检出——O96-03 实证） |
| `cd web && npm run build`（tsc -b + vite build） | 通过（1948 modules / 835ms / dist 产物正常） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round96-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第十一轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段定向 race 全绿；probe_identity_test.go 三钉实测通过；round41 七分支测试族全绿。
2. **O86-01/O94-01/O95-01 api 抖动基线第十一轮**：三轮全量 race 全 11 包全绿、api 包零 connectex（R94 曾出现两轮 1-2 例低频残余，R95/R96 连续两轮零复现）。**残余确系宿主环境冷启动窗口随机外溢，基线维持回升态：CI `-p 1` + 失败重跑吸收口径不变**，属测试健壮性级观察（O96-01），非产品缺陷。
3. **新角度扫查全绿**：登录链路完整状态机逐状态走读闭环（管理员双条件签发 + 时延拉平 / 学生 LoginByPassword 闸门 + wasShell 失败清理 / 手动激活票据绑定 / MarkTokenValid 锁序与手动登录恢复路径 / ReloginIfNeeded 客户端自绑定杜绝交叉污染）；会话与票据生命周期边界完整（12h 会话 TTL + 5min 清扫 + 票据 5min 单次防重放 + RevokeAccount 立即吊销）；doRequest/SelectClass/FindElectives/StudentCounts 平台契约逐条对齐 CLAUDE.md 基线；前端契约交叉核对（/electives、/state、/targets、/admin/* 与 client.ts/types.ts 逐字段）零错位。
4. **新增 OBSERVE 三条**（O96-01 connectex 连续零复现基线记录 / O96-03 gofmt CRLF 噪音实证 / O96-06 契约 20 注释回归检查），O96-02/04/05 为延续复核记录（O93-02 资产体积 / O80-01 删除保护撞名 / O92-02 logintest 分叉），**无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求**。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R95 同型），无代码修改建议提交。
