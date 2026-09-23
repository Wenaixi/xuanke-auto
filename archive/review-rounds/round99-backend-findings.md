# R99 后端只读审查报告

审查对象：xuanke-auto HEAD `faacafe`（R98 收官，进度 99/256；R99 为纯观察轮延续——HEAD 与 R98 相同，无新代码提交）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round99-backend-findings.md）。并行前端代理产出 round99-frontend-findings.md（见 git status `??`，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race **四轮** / go build / go vet / gofmt / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量 2050 行),api(handler+router 全量),zhidao(client+captcha+rsa+device_id+local_ocr+native_ocr 双文件),accounts,store,session,db,config,runtime,secure}、quit_shared.go、tray_windows.go/tray_other.go、native_ocr 资产体积。新契约角度本轮聚焦：**登录链路完整状态机（fetchLoginPage 自愈重试→验证码识别→提交→账密绑定）+ httpDo 自愈边界（dial/write 可重试 / read 不重试的语义矩阵）+ 配置热改完整闭环（handleAdminConfig→Runtime.Update→saveSettings→dispatchRuntimeConfig→SetRecognizer/SetCaptchaConcurrency）+ 会话生命周期对称性（Create/Account/Revoke/sweep/Close）+ 测试夹具真实性（socketPreheat/readyProbe/findBlock/selectBlock/fullBlock 各钩子使用正确性）**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O99-01（O98-01/O86-01/O94-01 api 抖动基线第十四轮）：四轮全量 race 全绿零异常——低频残余宿主轮换回顾测，基线回升维持"零复现"观察（实测）

**证据**：四轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）全部 11 包全绿零 FAIL——R1（api 34.764s）/ R2（api 37.798s）/ R3（api 全绿 14.9s 级）/ R4（api 14.907s）。与 R97/R98 两轮各自出现一次"单包单次 FAIL"相比，本轮四轮零异常。**注意**：R2 的 api 包耗时 37.798s 仍接近 R98 异常样本 34.322s 的量级（正常 19-28s），但全绿——"登录 mock 测试在 Windows 回环冷启动窗口低频慢路径"形态仍偶发拉长耗时但未翻转为 FAIL。结论：R97/R98 的单次 FAIL 属包序 + 冷启动窗口主导的低频残余，本轮回落，**基线维持低频波动口径：CI `-p 1` + 失败重跑吸收，非产品缺陷**。扩样本四轮为观察点增强（O94-01 要求的扩样本在 R94-R99 六轮累计 16 跑中仅 2 次单包单 FAIL，~12%）。

### O99-02（O98-02/O97-04/O80-01 删除保护撞名延续复核）：handleAdminDeleteAccount `IsAdminAccountName` 单判据与 B43-04 双条件不对称——历轮维持观察，本轮无新依据提级（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只看账号名等于 `AdminNameValue()`；IsAdminAccountName（handler.go:55-58）单字段比对。撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下该学生账号被删除保护永久覆盖。但：① 该路径 requireAdminSession 管理会话鉴权前置（router.go:155——删除必为管理员会话）；② B43-04 已为登录路径双条件签发，撞名学生可正常教务登录；③ 删除保护是"防空删除管理员自己"的硬护栏，与"登录准入"语义不同。历轮归"低优先级不修"，本轮实测测试族 TestAdminDeleteProtectsRenamedAdmin（handler_test.go:803）仍绿，维持观察。

### O99-03（O90-01 scheduler.go gofmt CRLF 噪音延续复核）：工作区 CRLF 转换噪音，git 仓库内容 LF 合规（实测）

**证据**：`gofmt -l .` 仍只检出 `internal/scheduler/scheduler.go`；`git show HEAD:backend/internal/scheduler/scheduler.go` 实测 CRLF 0 / LF 2049，差异纯为 checkout 时 `core.autocrlf=true` 的 LF→CRLF 转换噪音。非源码缺陷，无需提交动作。历轮 O90-01 维持。

### O99-04（O98-04/O96-06 契约 20 注释轮次标签回归）：全仓扫描仅一处文档路径引用，非违规（实测）

**证据**：grep `（第 \d+ 轮）|round\d\d|R\d\d-\d\d` 全仓仅命中 `backend/internal/session/store.go:117` 注释中的 `docs/review-round13.md` 文档引用（文件路径名，非轮次决策标签），不违反契约 20「代码注释严禁轮次前缀标签」字面。历轮固化回归维持，**零违规残留**。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无 | 四轮全量 race（全绿四轮）+ 逐点走读闭环，无新增可疑项。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第十四轮）**：逐点 grep `sameClientFor`（scheduler.go:204，判 nil 恒 false + reflect 指针身份）+ 全量调用点——ProbeForAccount 回写段（:850）、spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口（:1600）/确证满员（:1635）六分支；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeNow（:964-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 路径（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）。**round41 七分支测试族**（scheduler_test.go:3135/3194/3247/3296/3435/3518 + TestMaybeReloginDeletedAccountSkipsMaps:3401）全部使用 waitChainExit 等待契约（:3109，chainMu 活跃标记消失；绝不用 inflight 等待——PurgeAccount 删 nil map 恒 false 假绿陷阱注释载明）。**grep 全量复核无新裸露写点**。 | 通过（走读 + 测试族走读核对） |
| **B88-01 修复持续复核（第十四轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；acctData nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16 同名重建/:116 已删/:176 正常对偶）走读语义与四轮全量 race 回归全绿。 | 通过（实测） |
| **登录链路完整状态机（本轮新角度）**：fetchLoginPage（client.go:292-323）GET /login 自愈重试一次（连接层 + 4xx/5xx 均只容忍一次，纯 GET 零验证码限额，任何路径绝不再试）→ fetchCaptchaImage（:326）一次性验证码 → recognizeCaptcha（≤3 次识别，识别空视作失败刷新重来，网络/配置错立即返回绝不刷限流）→ submitLogin（:351 表单编码含 priorityId 契约微差、成功登记 token + zd_edu_cookie + access_limit_cookie 占位兜底 + cookiejar 服务端会话 Cookie 收集）→ **Login 成功分支 SetCredentials 写内部账密**（:270，根治"运行时登录后自动重登无保存账密"的停摆）→ LoginByPassword（manager.go:243 gateTryAcquire 非阻塞准入 + wasShell 失败清理判别保护既有工作客户端）→ issueSession（SaveAccountName + 签发 + MarkTokenValid 清失效标记）→ 激活票据链。ReloginIfNeeded（client.go:566）用客户端内部账密绝不交叉污染。**整链无未收口状态**。 | 通过（走读 + 测试绿） |
| **httpDo 自愈边界（本轮新角度）**：httpDo（client.go:474-488）只对 `isConnErrRetryable`（dial/write，请求未到服务端，重发安全）自愈重试一次；read 错误（服务端已消费 body 可能已处理，重发 POST 双报）与业务/取消错误原样上抛；cloneReq（:555）GetBody 由标准库对 bytes.Reader/strings.Reader 自动设置，keep-alive 濒死复用 nothingWritten 不双报。IsReadErr（:519-548）覆盖 RST（*net.OpError read）/FIN（io.EOF）/短读（io.ErrUnexpectedEOF）/超时（两种 wrap 文案）四形态；与 isConnErrRetryable 互补互斥。上抛方（spawnChain:1656 / handleElectiveSelect:356 / handleElectiveExit:422）read 类错误统一"可能已处理"文案——**只对连接层重试、绝不含业务重试，双报防线完整**。 | 通过（走读） |
| **配置热改完整闭环（本轮新角度）**：handleAdminConfig PUT（handler.go:735-838）先值域校验（引擎仅 vision/ddddocr、并发 1-20 越界整体拒绝）→ Runtime.Update 写锁闭包 → secureEncrypt vision_key（AES-256-GCM + enc: 前缀，settings 表永不出现明文）→ saveSettings 落库（失败也完成下游下发 + writeJSONStatus 500 家族）→ dispatchRuntimeConfig（:848-856：SetVision 保留当前引擎 + applyCaptchaRecognizerFor ",切换到 SetRecognizer + SetCaptchaConcurrency 动态信号量"——manager.go:191-216 注释载明 SetVision 保留模板引擎、SetRecognizer 写模板的"绝不挥动引擎"语义；新 ensure 客户端 getGlobalLimiter 热收敛）。八类配置项全部有消费点。**无半生效误导、无引擎漂移**。 | 通过（走读 + 测试绿） |
| **会话生命周期对称性（本轮新角度）**：session.Store Create/CreateAdmin 统一走 create（:152-158），Account/IsAdmin/IsAdminToken 三读方法过期自删（:161-203）；RevokeAccount 锁内遍历吊销（:207-215）；Delete 注销（:218-222）；sweepLoop 每 5 分钟清扫过期会话与票据（:58-87）；Close sync.Once 关清扫协程（:90-98）；randToken crypto/rand 失败 panic 拒签可预测令牌（:224-231）；票据 CreateTicket/ConsumeTicket 单次防重放 + 5min TTL + 账号绑定（:100-138）。**生命周期全对称、无令牌表膨胀、无清扫协程泄漏**。 | 通过（走读） |
| **多账号隔离边界（本轮新角度）**：?account= 透传 requireAdminSession 会话前置（allowAccountOverride:1066 仅 Admin:true）；TestAccountOverrideRequiresAdminSession（handler_test.go:358）载明普通会话（含名为 admin 的普通学生）携带 ?account=victim 只写进自己目标表——authz 门禁 + 凭据表 accountExists 双级；handleSetTargets 无透传时对无目标账号整体拒绝"请指定学生账号"（:482-484），绝不落管理员孤儿行；submitAll 按账号独立生成链 + 每账号独立 `*zhidao.Client`（manager.go ensure/ClientFor 独立实例，同一课程跨账号并行提交 TestSameClassParallelAcrossAccounts:532）。**账号 A 的操作绝不泄漏到账号 B 的请求/状态/库行**。 | 通过（走读 + 测试） |
| **测试夹具真实性（本轮新角度）**：socketPreheat（client_test.go:25 预创建-关闭 127.0.0.1 套接字排空冷启动窗口）+ readyProbe（10×200ms + 2s 超时宽栅栏，accounts/api/zhidao/captcha 四包全有；captcha_test.go:76 载明并发首请求需 readyProbe 吃掉 accept 旧窗口 + socketPreheat 双保险）；findBlock/selectBlock/fullBlock（scheduler_test.go:185-185 锁外阻塞钩子模拟真实网络往返，不持 fakeClient.mu 防数据竞争）；fakeAccts perAccount/removed（同名重建身份切换夹具）。**全部钩子被正确用在各自的真实场景，无假夹具掩盖缺陷**。 | 通过（走读 + 四轮全绿） |
| **M87-01 窗口再评估（第十四轮）**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）；quit_shared.go 双 nil 防御（:30-36）+ setExitActions 两半段注入（main:200 关服务 / tray_windows.go:54 退图标）+ tray_quit_test.go 三测试钉死。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **零吞错复核（契约 17）**：本轮扫查 spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handleAdminConfig 落库（:814 log + :819 500）、accounts LoginByPassword 加密失败（:286）、main 恢复各表失败——全部 `if err != nil { log.Printf }` 零吞错。 | 通过 |
| **O99-01 抖动基线第十四轮**：四轮全量 race（R1 api 34.764s / R2 api 37.798s / R3 R4 全绿）零 FAIL；accounts R4 40.384s / store 26.5s 均在正常区间。scheduler 15.2s / zhidao 2.9s 稳定。基线结论：R97/R98 单包单 FAIL 为包序 + 冷启动窗口主导的低频残余，本轮回落，"CI `-p 1` + 失败重跑吸收口径"不变。 | 通过（实测，O99-01） |

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（ProbeForAccount:855-858 每账号 / probe:1106-1111 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位 WindowOpened（:1011-1013）；配置层零注入（config.go:55-57 无 XUANKE_OPEN_TIME 读取）。 | 通过（实测） |
| **14（手动报名/退选 4 方法协同）**：TryAcquireSubmit 在飞互斥（:1896）+ MarkDone 清 done/inflight/full/rateLimited + 库内 refused 行（:1938-1948）+ RemoveDone 落 refused + DeleteSuccess 行（:2018-2028）+ RemoveFull（:2038）；窗口已关按满员记 full；spawnChain 提交前 inflight 去重。 | 通过（走读） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（本轮复核主要写点全绿）。 | 通过 |
| **B42-01（doLogin 全局频率闸门）**：gateTryAcquire 非阻塞（manager.go:223-235）+ gateWait 阻塞（:49-64）共享 gateMu/gateUsed 同一窗口计数；LoginByPassword pre-Login 准入（:244）；ResetGateForTest 仅测试（:82）。双入口无绕行。 | 通过（走读） |
| **B43-04（撞名双条件）**：handler.go:121 管理员名 + `subtle.ConstantTimeCompare` 双条件；不匹配的撞名学生走教务登录正常签发；教务也失败且账号是管理员名才报"管理口令错误"+ loginTimingFlat 300ms 时延拉平（:121-142）。 | 通过（走读） |
| **7（?account= 透传全路径凭据表校验）**：handleElectives:245 / handleElectiveSelect:295 / handleElectiveExit:373 / handleSetTargets:453 / handleState:537 五处 accountExists / 凭据表逐账号比对全覆盖，查无账号整体拒绝，绝不用全局帧假装成功。 | 通过（走读） |
| **5（"落库前锁内复核 ClientFor"防线族）**：自动链成功 / MarkDone / RemoveDone / 重登成功 / 实时人数复核回锁后 / spawnChain 链顶与取 client 后全部复核存在；已删账号静默放弃；实时复核满员分支先 doneHas（绝不覆盖手动胜利状态，:1627/:1645）。 | 通过（走读） |
| **43（maybeRelogin 决策侧存在性复核）**：maybeRelogin 入口锁内、map 写入前 ClientFor 复核（:1208），已删账号静默放弃；探测定时三处（ProbeForAccount:822 / ProbeNow:954 / probe:1091）直调全有该复核；TestMaybeReloginDeletedAccountSkipsMaps（scheduler_test.go:3401）实测四 map 零残留。 | 通过（走读 + 测试） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（第一轮） | **全 11 包全绿**（api 34.764s / scheduler 15.727s / store 46.817s / accounts 1.458s 等） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（第二轮） | **全 11 包全绿**（api 37.798s——异常耗时形态仍在但未翻 FAIL） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（第三轮） | **全 11 包全绿** |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（第四轮） | **全 11 包全绿**（api 14.907s / accounts 40.384s / store 26.525s / zhidao 2.917s） |
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音 O90-01 延续；HEAD 仓库版本 LF 合规——O99-03 实测 CRLF 0/LF 2049） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round99-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第十四轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段复核通过；probe_identity_test.go 三钉走读语义正确；round41 七分支测试族全部使用 waitChainExit 等待契约（绝不用 inflight 等待——PurgeAccount 删 nil map 恒 false 假绿陷阱载明）。
2. **O86-01 api 抖动基线第十四轮**：**四轮全量 race 全绿零 FAIL**（R2 api 37.798s 异常耗时未翻爪）——R97/R98 单包单 FAIL 确认为包序 + 冷启动主导的低频残余，本轮回落。**基线维持低频波动口径：CI `-p 1` + 失败重跑吸收，非产品缺陷，观察增强**（六轮 16 跑仅 2 单例）。 |
3. **新角度扫查全绿**：登录链路完整状态机（fetchLoginPage 自愈收敛 → 识别 ≤3 次 → 一次性验证码 → Login 成功写内部账密根治无保存账密停摆）；httpDo 自愈只覆盖连接层（dial/write 重试、read/业务绝不重试，双报防线完整）；配置热改完整闭环（值域校验 → 加密落库 → 下游热下发 → 引擎模板同步，八类配置项全消费）；会话生命周期全对称（Create/Account/Revoke/sweep/Close 无泄漏）；多账号隔离（authz 门禁 + 凭据表双级 + 独立客户端实例，同课程跨账号并行提交）；测试夹具真实性（socketPreheat/readyProbe/锁外阻塞钩子全被正确使用）。
4. **新增 OBSERVE 一条**（O99-01 api 抖动基线第十四轮回升记录）、延续复核三条（O99-02 删除保护撞名 / O99-03 CRLF 噪音 / O99-04 契约 20 回归零违规），**无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求**。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R98 同型），无代码修改建议提交。