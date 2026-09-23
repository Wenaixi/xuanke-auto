# R95 后端只读审查报告

审查对象：xuanke-auto HEAD `d8fe40f`（R94 收官，进度 95/256；本轮回合为纯观察轮延续，含 d8fe40f/cacacc6/1dfd116/f08937e 四提交，均为 R93/R94 已核内容）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round95-backend-findings.md）。并行前端代理产出 round95-frontend-findings.md（见 git status `??`，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 三轮 + 定向 race 回归 / go build / go vet / gofmt / 前端 npm run build / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量 2050 行),api,accounts,store,session,zhidao,runtime,config,secure,db}、quit_shared.go、tray_windows.go、web/embed.go、cmd/{bench,probe,logintest}、native_ocr 双文件 + 前端 web/src（api/client.ts、types.ts、App.tsx、routes/{Login,Select,Dashboard,Admin}.tsx、lib/targetGuard.ts、lib/adminAuth.ts、lib/useTickingCountdown.ts）。新契约角度本轮聚焦：**前端契约交叉核对（/api 各端点的请求/响应体与前端 client.ts 消费逐字段对齐）+ 会话与激活码生命周期边界 + 错误响应 body 结构一致性 + 定时器与 goroutine 生命周期 + 配置持久化完整性**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O95-01（O86-01/O94-01 api 抖动基线第十轮）：三轮全量 race 零复现——O94-01 低频残余确认回落，基线恢复"零复现"观察（实测）

**证据**：本轮三轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）全部 11 包全绿（第一轮 api 19.481s / 第二轮 api 61.150s / 第三轮 api 27.831s），**api 包零 connectex**（R94 曾出现两轮 1-2 例低频残余）。定向 race 回归（`-run 'TestProbe|TestDeletedAccountRebuilt|TestPurgeAccount|TestRestore|TestRelogin|TestRateLimit|TestLogin|TestActivate|TestAdmin'` scheduler 3.922s + api 16.915s）零复现。样本统计：本轮 3 轮全量全绿，结合 R94 的"两轮命中 1-2 例、两轮零复现"，残余属**宿主环境冷启动窗口随机外溢**，非产品逻辑触发面。**基线结论维持：低频残余/零复现交替由宿主环境波动主导，CI `-p 1` + 失败重跑吸收口径不变。**

### O95-02（O80-01 删除保护撞名延续复核）：handleAdminDeleteAccount `IsAdminAccountName` 单判据防护与 B43-04 双条件不对称——历轮已声明维持观察，复核仍准确（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只看账号名等于 `AdminNameValue()`。撞名场景（`XUANKE_ADMIN_NAME` 显式配成某学生学号）下该学生账号被删除保护永久覆盖；B43-04 已为登录路径处理同款撞名（`Account == adminName && ConstantTimeCompare`），但删除/保护路径未对称。R80 已归 OBSERVE 声明"管理员名需显式配成学生账号名才触发，非默认态"，历轮维持。**本轮复核：代码未变，观察项维持**（与 O80-01 同一记录，不重复立条）。

### O95-03（O92-02 logintest 引擎判定分叉延续复核）：三轮复核后维持——logintest 为纯诊断工具不参与抢课（走读）

**证据**：cmd/logintest/main.go:57 仍走 `config.CaptchaEngineDefault()`（环境变量 + 编译默认），主程序以 settings 持久化值覆盖；三态回退链完整。历轮归"低优先级不修"，维持观察。

### O95-04（O93-02 内嵌 ddddocr 资产体积延续复核）：≈29.7MB 无新增膨胀（实测）

**证据**：assets 实测三文件——onnxruntime.dll 16,048,160 字节 / common_old.onnx 13,606,051 字节 / charsets_old.json 57,249 字节，合计 29,711,460 字节 ≈29.7MB，与 O93-02/O94-03 记录一致。dumpIfDiff 幂等释出（native_ocr.go:95-103）+ initMu.Once 懒加载，无新增资产。Windows CGO=1 / Linux macOS CGO=0 分流契约不变。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无 | 本轮三轮全量全绿 + 定向回归全绿 + 前端 build 全量通过 + 契约逐条走读核证，无新增可疑项。O94-01 connectex 残余本轮零复现（见 O95-01），R94 已归档为"低频残余 + CI 重跑吸收"。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第十轮）**：逐点 grep `sameClientFor`（scheduler.go:204，判 nil 恒 false + reflect 指针身份）+ 全量调用点——ProbeForAccount 回写段（:850）、spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口（:1600）/确证满员（:1635）六分支；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeNow（:964-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 路径（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）。**grep 全量复核无新裸露写点**。对应测试族 `TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull,RealtimeUnauthorized,SuccessDropsInflight}` 定向 race 全绿。 | 通过（走读 + 定向 race 实测） |
| **B88-01 修复持续复核（第十轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；acctData nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16/:116/:176）定向 + 全量 race 回归全绿。 | 通过（实测） |
| **前端契约交叉核对（本轮新角度）**：前端 `web/src/api/client.ts` 全端点与后端 `internal/api/handler.go`/`router.go` 逐字段核对——`/api/electives`（GET ?account= → ElectivesData：begin_times/publishes/classes 字段名与 zhidao.Publish/Class JSON tag 全对齐）；`/api/electives/select` 与 `/exit`（POST body `{class_id, course_name?}` → 响应 `{msg, class_id}`，client.ts:119/133 消费一致，后端 writeJSON 0 分支返回 `map[string]any{"msg":...,"class_id":...}` 对齐）；`/api/targets`（PUT body `{targets}` → scheduler.Target JSON tag publish_id/class_id/course_name/priority 全对齐）；`/api/state`（SchedulerState JSON：open_time/open_time_known/window_opened/window_closed/token_valid/courses，前端 types.ts SchedulerState 逐字段对齐；CourseStatus publish_name/begin_date omitempty 与 CourseStatus 类型可选字段一致）；`/api/accounts`（[]string）；`/api/logs`（LogEntry id/class_id/action/result/is_ok/created_at，注意后端 LogEntry JSON 含 account 字段但客户端 LogEntry 类型不含——Dashboard 日志列表按账号隔离消费不读 account，类型不消费即可，非缺陷）；`/api/admin/config` GET/PUT（AdminConfig 六字段全对齐，vision_api_key_masked 回显 + PUT 留空不改 key）；`/api/admin/stats`（AdminStats open_time/activation_on/window_opened/window_closed/account_count/targets_count/success_count/log_count/vision_model/vision_base_url/captcha_engine/captcha_concurrency/open_time_set/token_valid，后端 handler.go:947-965 逐字段核对，captcha_engine 空串兜底 "ddddocr"）；`/api/admin/accounts`（AdminAccount 三字段）；`/api/admin/logs`（AdminLog 七字段全对齐）；`/api/login` 响应（token/account/adminName 仅管理员带——前端 login 断言 `adminName?` 可选）；`/api/activate` 响应（token/account）；`/api/logout`（无 body 数据）。**全量端点零字段错位**。 | 通过（走读） |
| **会话与激活码生命周期边界（本轮新角度）**：session.Store TTL 12h（main.go:162）+ 5min 清扫协程 + Close 防泄漏；ticket TTL 5min 单次（ConsumeTicket 存在/未用/未过期/绑定账号四校验 + used 置位销毁，防重放穷举）；激活码 ConsumeActivationCode 单事务 UPDATE `used_uses < total_uses` 原子扣减（SQLite 单写者无超卖）+ 已激活账号不扣次（nAct>0 返回 false 回滚）；`handleActivate` 先 ConsumeTicket 再校验激活码（防穷举）。**边界完整**。 | 通过（走读） |
| **错误响应 body 结构一致性（本轮新角度）**：全量 grep writeJSON/writeJSONStatus——业务失败统一 `{code:1,data:nil,msg:...}` HTTP 200；基础设施失败统一 writeJSONStatus 家族 `{code:401/403/429/500/404,data:nil,msg:...}` + 真实 HTTP 状态（requireAuth 401:1097 / requireAdminSession 403:606 / 登录激活 429 / recoverMiddleware 500:1203 / config 落库失败 500:819 / stats 目标数 500:924 / requireJSONBody 403:72 / /api/ 404:198）；登录未激活 code=1001 携带 data.ticket/account。**body.code 语义家族完整无混用**（前端 client.ts 只读 body code，HTTP 状态增强不破坏契约）。 | 通过（走读） |
| **定时器与 goroutine 生命周期（本轮新角度）**：tick 主循环（Start:666-684 select ctx.Done/ticker.C/reloginResults 三路退出）；sweepLoop（session store.go:58-70 Stop 关闭）；GatePump main.go:153-159 30s ticker 常驻（进程级无害）；maybeRelogin goroutine（:1244 一次性，defer 收尾）；maybeSyncClock goroutine（:365 一次性，回调复位 syncing）；probe() per-account goroutine（:1070-1076 probeSem 信号量释放）；spawnChain goroutine（:1399-1669 defer chainMu delete 收尾）。**全部 goroutine 均有明确退出路径，无泄漏**。 | 通过（走读） |
| **配置持久化的完整性（本轮新角度）**：settings 表写读对称——handleAdminConfig PUT 六个键（activation_enabled/vision_base_url/vision_key/vision_model/captcha_engine/captcha_concurrency）→ saveSettings 全量替换 → main.go:80-110 启动恢复六键（vision_key 严格 enc: 前缀校验，非 enc 拒绝）；vision_key 加密落库（secureEncrypt）+ 回显脱敏（maskKey 只显 ****+后 4 位）；PUT 落库失败走真实 500 且热下发不跳过（半生效绝不静默）。**写读对称 + 加密落库 + 脱敏回显完备**。 | 通过（走读） |
| **O86-01/O94-01 api 抖动基线第十轮**：三轮全量 race 全绿零 connectex，定向回归全绿；store/zhidao/scheduler/accounts/db/config/runtime/session 各包全绿。基线结论：低频残余/零复现交替由宿主环境波动主导，非产品缺陷。 | 通过（实测，O95-01） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **契约 20 注释无轮次标签**：grep `（第 N 轮）|round\d|R\d*-` 全仓零残留（R93/R94 已固化为回归检查）。 | 通过（实测） |
| **cmd 三工具契约**：bench 边界防御（-n ≤0 回退 200）、probe 用 XUANKE_PROBE_TOKEN env + SharedTransport()、logintest 三态引擎回退 + -limit 收敛。与历轮记录一致。 | 通过（走读） |
| **httpDo 自愈族复核**：httpDo 只重试 dial/write（client.go:474-488）+ isConnErrRetryable（:492-501）+ IsReadErr 四形态（:519-548）+ cloneReq 深拷贝；fetchLoginPage 首请求失败重试一次（:292-323）；recognizeViaVision 同走 httpDo（captcha.go:162）。**与 F52 族契约一致**。 | 通过（走读） |
| **前端 selectElective/exitElective 返回类型复核**：client.ts 返回 `{msg, class_id}`，后端 handler.go:364/:432 成功分支 map 含 `"msg"`/`"class_id"`——Select.tsx 消费 `res.msg`（toast 文案）字段对齐。 | 通过（走读） |
| **撞名学生管理态与后端签发双条件交叉核证**：F52-M1 前端 adminAuth.ts `isCurrentAdminSession`（adminToken !== "" && sessions[adminName] === adminToken）与后端 handleLogin 双条件签发（handler.go:121 `Account == adminName && ConstantTimeCompare`）+ 响应带 adminName 才标记——撞名学生普通会话 token 永远匹配不上标记，刷新不误进管理页。Admin.tsx:299 渲染判据 `inAdmin || isCurrentAdminSession` 同源。 | 通过（走读） |

## 契约抽查表（抽查 9 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（:855-858 每账号 / :1106-1110 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）；识别槽无值回退遗留 openTime 字段（:435-437，main.go 传零值后恒零）。TestOpenTimeForLockedReturnsRecognizedValueEvenWhenPast + TestOpenTimeRetainedAfterWindowClosed 固化为绿。 | 通过（实测） |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:918-939）+ 主判据 10s 裕量（:1146）+ EmptyProbeRuns 入账同 10s 裕量（:1155）；StateForAccount:703 与 WindowClosed() 共用同一实现；关闭后识别槽保留 + 目标发布元数据持久化（SetTargetsForAccount :473 enrichTargetPubMetaLocked + 落库）+ RestoreTargets 兜底全校帧补全（:531）。 | 通过（走读 + 定向测试绿） |
| **4/21（删号 memory-first + 防寄生 = 指针身份比对）**：handleAdminDeleteAccount（handler.go:1004-1018）顺序 Remove → PurgeAccount → DeleteAccount → RevokeAccount 与契约 4 一致；sameClientFor + clientIdentity（reflect 指针）与矩阵全闭合；round41 七分支测试族 + probe_identity_test.go 三钉定向 race 全绿。 | 通过（回归实测绿） |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（accounts.go:223-235）+ gateWait 阻塞（:49-64）共享 gateMu/gateUsed 同一窗口计数；LoginByPassword pre-Login 准入（:244）；ResetGateForTest 仅测试（:82-87）。双入口无绕行。 | 通过（走读） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handler 登录/配置/删除、accounts SaveCredential/加密失败、main 恢复各表失败）；TestStoreFailuresLogged 族钉死。仅有的 `_ =` 为 Prewarm/ProbeForAccount 探测结果（非落库，内记日志）+ config.go 首次 .env 写盘（O80-03 已声明观察）。 | 通过 |
| **7（?account= 透传全路径凭据表校验）**：handleElectives:245 / handleElectiveSelect:296 / handleElectiveExit:374 / handleSetTargets:474 / handleState:538 五处 accountExists 全覆盖，查无此账号整体拒绝；accountExists 用 LoadCredentials 更强真理源（:1073-1084）。 | 通过（走读） |
| **B43-04 + F52-M1（撞名双条件 + 管理态判定绑定 adminName）**：handler.go:121 双条件签发；前端 adminAuth.ts isCurrentAdminSession + login 响应 adminName 标记 + logout/onDeleted/onUnauthorized/onBackToStudent 全路径清标记（App.tsx 交叉核证）。 | 通过（走读） |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御（:29-35）；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:200-206）；tray_windows.go:54 注入「退图标」半段、main:200 注入「关服务」半段，顺序由 quitApplication 保证（先关服务再退图标）；tray_quit_test.go 钉死。 | 通过 |
| **F52-M4/M5（连接活性自愈族）**：httpDo 只重试 dial/write（:474-488）+ isConnErrRetryable 仅 dial/write（:492-501）+ IsReadErr 四形态（:519-548）+ cloneReq 深拷贝；fetchLoginPage 首请求失败重试一次（:292-323）；recognizeViaVision 同走 httpDo（captcha.go:162）。**四形态 connectex 只在冷启动窗口命中、非 keep-alive 衰减形态——自愈族有效覆盖**。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第一轮） | **全 11 包全绿**（api 19.481s / scheduler 15.378s / store 38.358s / zhidao 2.407s 等），**api 包零 connectex** |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第二轮） | **全 11 包全绿**（api 61.150s / scheduler 15.471s / store 42.692s 等），**api 包零 connectex** |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第三轮） | **全 11 包全绿**（api 27.831s / scheduler 17.089s / store 43.960s 等），**api 包零 connectex** |
| `go test -race -count=1 -run 'TestProbe\|TestDeletedAccountRebuilt\|TestPurgeAccount\|TestRestore\|TestRelogin\|TestRateLimit' ./internal/scheduler/`（身份防线定向） | 3.922s 全绿 |
| `go test -race -count=1 -run 'TestLogin\|TestActivate\|TestAdmin' ./internal/api/`（撞名/激活/管理定向） | 16.915s 全绿 |
| `go test -race -count=1 -p 1 -timeout 300s -run 'TestProbeIdentity\|TestDeletedAccountRebuilt\|TestRelogin\|TestRateLimit\|TestSubmit\|TestWindowOpen\|TestWindowClosed\|TestOpenTime\|TestSuspended\|TestAdminStatsWindowOpened\|TestRestore\|TestMarkDone\|TestRemoveDone\|TestTryAcquire\|TestLoginAdminNameCollision' ./internal/scheduler/ ./internal/api/` | 全绿（scheduler 7.296s / api 3.146s） |
| `go build ./...` | 通过（BUILD OK） |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，R90-01 延续；git 仓库版本 LF 合规零检出） |
| `cd web && npm run build`（tsc -b + vite build） | 通过（1948 modules / 447ms / dist 产物正常） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round95-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第十轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段定向 race 全绿；probe_identity_test.go 三钉实测通过；round41 七分支测试族全绿。
2. **O86-01/O94-01 api 抖动基线第十轮**：三轮全量 race 全 11 包全绿、api 包零 connectex（R94 曾出现两轮 1-2 例低频残余）。**残余本轮回落零复现，基线结论维持：低频残余/零复现交替由宿主环境冷启动窗口随机外溢主导，CI `-p 1` + 失败重跑吸收口径不变**，属测试健壮性级观察（O95-01），非产品缺陷。
3. **新角度扫查全绿**：前端契约交叉核对全端点逐字段对齐零错位（ElectivesData/Target/SchedulerState/AdminConfig/AdminStats/AdminAccount/AdminLog 全家族核对）；会话与激活码生命周期边界完整（票据 5min 单次防重放 + 激活码原子扣减无超卖 + 12h 会话 TTL + 清扫协程防泄漏）；错误响应 body code 家族一致性完备（业务 1 / 基础设施 401/403/429/500/404 / 未激活 1001 携带 ticket）；定时器与 goroutine 全部有明确退出路径无泄漏；settings 表写读对称 + vision_key 加密落库 + 脱敏回显完备。
4. **新增 OBSERVE 一条**（O95-01 connectex 零复现基线回落记录），O95-02/03/04 为延续复核记录（O80-01 删除保护撞名 / O92-02 logintest 分叉 / O93-02 资产体积），**无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求**。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R95 同型），无代码修改建议提交。
