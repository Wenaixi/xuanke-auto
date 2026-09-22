# R90 后端只读审查报告

审查对象：xuanke-auto HEAD `87b1d53`（R89 收官，进度 90/256）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round90-backend-findings.md），工作树保持 `## master` 洁净（git status 实证：仅前端并列报告的未跟踪文件，零改动既有文件）。

审查方式：Read / Grep / Glob / Bash 只读命令（bg 全量 race 测试 / 双端交叉编译 / go build / go vet / gofmt / 定向走读）。核心走读范围：backend/main.go、quit_shared.go、tray_windows.go、internal/{scheduler,accounts,api,store,zhidao,session,db,runtime,config,secure} 全量、native_ocr 双文件、local_ocr.go。新契约角度：配置热重载边界、手动报名/退选状态同步、多账号登录闸门、时间基准一致性抽核、错误文案用户可理解性。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O90-01：scheduler.go gofmt 差异确认为纯 CRLF 工作区转换噪音（git 仓库内容 LF 合规）

**证据（本轮实测）**：`gofmt -l .` 检出 `internal/scheduler/scheduler.go` 整文件——但逐字节核验：工作区该文件为 `CRLF: 2053 LF: 2053`（全行 CRLF）；`git show HEAD:...scheduler.go` 仓库内容为 `CRLF: 0 LF: 2053`（全 LF 合规），`gofmt -l` 对 git 版本**零输出**。差异来源：仓库 `.gitattributes` 不存在 + `core.autocrlf=true`（git config 实测），checkout 时 LF→CRLF 转换，gofmt 对 CRLF 整文件 diff 属已知行为。**影响**：纯格式噪音，`go vet`/`go test`/构建全不受影响；**不是**源码缺陷，也不需任何提交。**修复方向**：无需动作；若想根治可加 `.gitattributes` 声明 `*.go text eol=lf`，但不建议专门为本项发提交。

**注**：R89 报告中的 `probe_identity_test.go 缺尾换行`（O89-01）本轮已不存在——git 版本末尾字节实证 `...\n\t}\n}\n`（尾随换行完好），工作区与 git 内容 diff 为空、gofmt 零输出。R89 检出的差异属当时工作区转换状态，仓库内容从未缺尾换行，该观察项自动闭合。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无新可疑项 | 本轮走读未发现需主控深度核实的存疑点；配置热重载、手动操作同步、闸门、时间基、文案全部走读闭环。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **B88-01 修复持续复核**（ProbeForAccount 回写段身份复核，scheduler.go:816-879）：回写段持 `s.mu` 先 `sameClientFor(acct, client)`（:854），非同一身份整体放弃写 acctData/acctDataAt/识别槽并返回 data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:859-862）；锁序无变化（单把 s.mu，无嵌套）；`s.acctData` nil 守卫（:863）在身份复核后、写入前，仍安全。probe_identity_test.go 三钉（删除重建丢弃 / 已删丢弃 / 同身份对偶）——本轮定向重跑**全绿**（见构建验证表）。 | 通过（实测 + 走读） |
| **身份防线 16 项矩阵延续**：逐点复核 spawnChain 链顶（:1392 ClientFor + :1409 取 client 后二次判 + :1421 指针捕获）、六分支 sameClientFor（失效:1493 / 成功:1525 / 风控:1555 / 窗口关闭:1575 / 实时复核满员:1639 / 实时复核 ErrUnauthorized:1604）、maybeRelogin 决策侧（:1212 ClientFor）+ 写回侧（:1258）、ProbeForAccount 回写段（:854）、ProbeNow 全局帧（:967-971 无身份维度）、probe() 全局帧（:1110-1120 同）、MarkDone（:1931）/RemoveDone（:1999）/SubmitAll（:1359）/Restore 路径（无网络）。**grep 全部「网络往返后持锁写状态/落库」点无新裸露写点，矩阵保持准确**。 | 通过 |
| **probe()→ProbeForAccount 路径一致性**：probe() 内 per-account goroutine（:1073-1081）经 `s.ProbeForAccount(acct)` 走同一复核路径；probeSem cap 4 封顶 per-account 并发，跨批不叠加。probe() 主体全局帧（:1110-1120）同 ProbeNow 无身份维度，正确性在全局语义侧成立。 | 通过 |
| **O86-01 api 抖动基线第五轮**：bg 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` **13 包 exit 0**（api 72.928s / store 31.738s / scheduler 15.579s / zhidao 2.509s 实测）；**connectex 零样本**（R86-R90 连续五轮零复现）。 | 通过（实测） |
| **O88-01/02 记录复核（sharedTransport TLS / Shutdown 反向慢连）**：sharedTransport（client.go:22-34）无 `TLSClientConfig`——走标准库 nil→系统证书池；`httpDo` 只重试 dial/write（isConnErrRetryable:492-501，与 IsReadErr:519-548 互斥），业务/取消错误原样上抛。Shutdown 5s 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖，无新触发面。 | 通过 |
| **M87-01 窗口再评估（5s Shutdown vs 在飞 spawnChain）**：黄金期场景 Shutdown 不中断在飞 conn（StateActive continue），5s 超时强杀；在飞未落库 final 以重启重试兜底（RestoreDone 只恢复已落库 success）。维持 MINOR + 注释兜底，历轮结论仍准确。 | 通过 |
| **配置热重载边界**（本轮新角度）：PUT /api/admin/config（handler.go:742-842）先值域校验（engine 仅 vision/ddddocr、concurrency 1-20，非法整体拒绝不落库不下发）→ Runtime.Update 闭包原子应用 → secureEncrypt 加密 vision_key → saveSettings 落库 → dispatchRuntimeConfig（:848-856）热下发。**失败回滚语义**：落库失败仍返回 500 但已完成内存生效与下游下发（绝不半生效误导，:811-821）；识别引擎热切换同步模板（router.go:33-55 applyCaptchaRecognizerFor + SetRecognizer 写模板 + SetVision 保留当前引擎，manager.go:191-216），新 ensure 客户端恒拿到引擎；SetCaptchaConcurrency 热收敛信号量（captcha.go:92-94，Mutex+Cond 无 channel 替换死锁）。act 热改对调度器开放时间零影响（配置层已整体移除，唯一事实源=beginTimes）。**未发现运行中生效与失败回滚缺口**。 | 通过 |
| **手动报名/退选状态同步**（本轮新角度）：handleElectiveSelect（:286-365）/handleElectiveExit（:368-433）经 TryAcquireSubmit 排他锁（inflight 互斥）→ CheckClassSelectable 快照复核（过期快照放行交平台，:1867-1896）→ 平台真实请求 → MarkDone/RemoveDone 同步状态（清 inflight/full/rateLimited + 置 success/pending + 落库 + 清 refused 单课行）。手动操作后 tick 重新探测/提交行为：RemoveDone 置 refused → spawnChain 该课永久跳过直到重设目标；MarkDone 清 refused → 自动引擎恢复接管。窗口已关平台"无效课程ID"→ isWindowClosedError 记 full 不轰炸。ErrUnauthorized → MaybeRelogin（只调不 MarkTokenValid，指数退避不被击穿）。**闭环无缺口**。 | 通过 |
| **多账号登录闸门**（本轮新角度）：gateWait（阻塞，Manager.Relogin）+ gateTryAcquire（非阻塞，LoginByPassword:244）共享 gateMu/gateUsed 同一窗口计数；GatePump 30s 推进窗口（main.go:153-159）；ResetGateForTest 仅测试。gateTryAcquire 不重入 gateWait（独立快路径先 Lock 再检查，无死锁）。多账号集中失效排队重登 + 手动登录并发时出口 IP doLogin 收敛到 2/min。**窗口推进与边界走读闭环**。 | 通过 |
| **时间基准一致性抽核**（本轮抽核）：scheduler 全部时间读写点 grep 实证——lastProbe（probe:1118/ProbeNow:968 写，均对齐钟；tick:991 读）、lastSubmit（submitAll:1350 对齐钟写、tick:1036 读）、rateLimited 退避（markRateLimitedLocked:1718 对齐钟写、isRateLimitedLocked:1706 读）、识别槽覆盖（ProbeForAccount:860 对齐钟同帧 now）、窗口判定（probe:1150/1159 用对齐钟 now + open 单快照复用）。**残余本地钟点**：maybeRelogin reloginAt 用 time.Now()（:1235/1265）与 reloginBackoff 判期 time.Since 同本地基自洽（重登节流无对齐需求）；maybeSyncClock lastSyncFailAt 本地钟（:376）与 maybeSyncClock 读 time.Since 同基；session/store 过期时间本地钟自洽。**无混用**。 | 通过 |
| **错误文案用户可理解性**（本轮新角度）：手动报名/退选 read 类错误文案"报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）"（:356-359/:422-425）、token 失效"教务令牌已失效，正在自动重登，请稍后重试"、风控"触发平台风控退避 30 秒"、满员"该课程已满员，退避至下一备选"——全部面向终端用户且与平台实证文案（选课成功/不在选修报名时间范围内/重复提交）对齐；日志侧 maskedToken 只显前 8 位、Vision 识别原文脱敏不回传。**面向用户语义成立**。 | 通过 |
| **SQLite 写路径锁纪律**：db.go Open 单连接（SetMaxOpenConns=1）+ schema/migrate/refuseLegacy 串行；store 事务（SetTargetsForAccount/CreateActivationCodes/ConsumeActivationCode/SaveSettings/DeleteAccount）全部 `Begin→Commit + defer Rollback`，无手工 db.Exec 于锁内跨网络；scheduler 持 s.mu 时落库仅微秒级。**无锁序反转**。 | 通过 |
| **重登链/并发安全**：reloginResults chan cap 8 非阻塞发送（:1282-1285）；maybeRelogin 指数退避（backoffMin<<n 封顶 10min，maxReloginFail=5 防溢出）；MarkTokenValid 与 maybeRelogin 对齐 reloginMu→s.mu 锁序；grep 实证失败分支保留 reloginFail 计数（绝不无条件复位 1，振荡根治）。 | 通过 |
| **恢复序/数据安全**：main.go RestoreDone → 逐账号 LoadTargetsForAccount+RestoreTargets（不清 refused）→ LoadRefused+RestoreRefused；SetTargetsForAccount 只清 refused 绝不清 done/full/rateLimited/inflight；RestoreRefused 注入"已手动退选"文案且拒绝覆盖 success。数据库迁移规范：migrateAddPublishMeta 先于 refuseLegacy、缺列才 ALTER、refuseLegacy 清单剔除已迁移列（db.go:28-32）。 | 通过 |
| **凭据/密钥加密**：凭据 AES-256-GCM 加密（secureEncrypt + enc: 前缀）；vision_key 严格要求 enc: 前缀、未加密旧值拒绝加载（main.go:87-97）；主密钥缺失拒绝启动；maskKey 回显脱敏（handler.go:704-712）；不存明文。 | 通过 |
| **会话/激活/票据**：会话 12h TTL + sweeper（Close sync.Once 等 sweepDone）；票据 5 分钟单次防重放（ConsumeTicket 消费即删）；激活码原子扣减（单条 UPDATE used_uses<total_uses 条件防超卖）+ 已激活不扣次；登录/激活独立限流桶（互不锁死）。 | 通过 |
| **HTTP 基础设施状态码**：六类基础设施路径（panic 500 / 会话 401 / 管理 403 / 限流 429 / CSRF-403 / 未知 API 404）+ config 落库失败 500 全写真实状态码；业务 100+ 点恒 200（前端契约只读 body code）。`/api/` catch-all（router.go:194-199）先设头再 WriteHeader，不重蹈 404 错标 text/plain。 | 通过 |

### 身份防线全员清点矩阵（R90 复核——R89 矩阵延续）

| # | 写点 | 网络往返 | 复核类型 | 结论 |
|---|---|---|---|---|
| 1 | spawnChain 链顶（:1392 + :1409-1415） | 取 client 前 | ClientFor 存在性（两次）+ 指针捕获 :1421 | 通过 |
| 2 | spawnChain 失效分支（:1493-1497） | SelectClass → ErrUnauthorized | sameClientFor | 通过 |
| 3 | spawnChain 成功分支（:1525-1529） | SelectClass 成功 | sameClientFor | 通过 |
| 4 | spawnChain 风控退避分支（:1555-1558） | SelectClass 命中 rateLimit | sameClientFor | 通过 |
| 5 | spawnChain 窗口关闭分支（:1575-1578） | SelectClass 命中 closed | sameClientFor | 通过 |
| 6 | spawnChain 实时复核满员分支（:1639-1642） | classFullRealtime 网络段后回锁 | sameClientFor | 通过 |
| 7 | spawnChain 实时复核 ErrUnauthorized 分支（:1604-1624） | classFullRealtime 网络段后回锁 | sameClientFor | 通过 |
| 8 | maybeRelogin 决策侧（:1212） | 发起前 | ClientFor 存在性 | 通过 |
| 9 | maybeRelogin 写回侧（:1258-1262） | Login 返回后 | ClientFor 存在性 | 通过 |
| 10 | ProbeForAccount 回写段（:854，B88-01） | FindElectives 返回后 | sameClientFor | 通过（三钉实测） |
| 11 | ProbeNow 全局帧（:967-971） | FindElectives 返回后 | 无身份维度（全局帧） | 通过 |
| 12 | probe() 全局帧（:1110-1120） | FindElectives 返回后 | 同 ProbeNow | 通过 |
| 13 | MarkDone（:1931-1934） | 手动报名网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 14 | RemoveDone（:1999-2002） | 手动退选网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 15 | SubmitAll（:1359-1361） | 无网络（内存链构建） | ClientFor 存在性（下发链前过滤） | 通过 |
| 16 | Restore 路径（RestoreDone:611 / RestoreTargets:529 / RestoreRefused:630） | 无网络 | 不需（账号都在注册表） | 通过 |

grep 实证全部「网络往返后持锁写状态/落库」点均已闭合，无新裸露写点。

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身；空快照不删槽、非空 beginTimes 才覆盖；B88-01 双条件（身份通过 + beginTimes 非空）不改变语义；识别过期只影响展示层（StateForAccount:718-720 After 判定）。 | 通过（open_retain/open_detect 测试绿） |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:922-943），StateForAccount 与 WindowClosed() 共用；目标发布元数据随 targets 持久化，/state.courses 自带日期/发布名。 | 通过 |
| **21（防寄生 = 指针身份比对）**：sameClientFor（:208-214）+ clientIdentity（:219-228，reflect.ValueOf(c).Pointer()）与 Matrix 1-10 闭合。 | 通过（回归钉实测绿） |
| **B41-01/B41-02（零值守卫让步 + 全分支身份族）**：spawnChain 六分支 + 实时复核第七分支 + 探测回写全部闭合；tick 零值守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1015-1017）。 | 通过 |
| **36/37/43（决策侧/写回侧复核、全分支清点）**：maybeRelogin 入口存在性复核（:1212）+ 写回侧复核（:1258）+ MarkTokenValid 锁序（reloginMu→s.mu）；grep 全分支清点无新缺口。 | 通过 |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（LoginByPassword:244）+ gateWait 阻塞（Relogin）+ GatePump 窗口推进共享同一 gateMu/gateUsed；ResetGateForTest 仅测试夹具。 | 通过 |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }`（spawnChain 六分支 / MarkDone / RemoveDone / markFull / handler 登录/配置/删除 / accounts 凭据加密 / maybeRelogin UpdateIDToken 等）。 | 通过 |
| **B39-02/B40-01（writeJSONStatus 家族与状态码成族核对）**：六类基础设施路径 + config 落库失败 500；业务 100+ 点恒 200。 | 通过 |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `cd backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，13 包 exit 0（api 72.928s / store 31.738s / scheduler 15.579s / zhidao 2.509s）；connectex 零样本 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 整文件，工作区转换噪音；git 仓库内容 LF 合规零检出——见 O90-01）；probe_identity_test.go 已干净 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build ./...` | 通过（内嵌 ddddocr 双轨 + 托盘路径，输出单 exe 成功） |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` | 通过（纯 Go 交叉编译 + embed 资产） |
| 定向 race：B88-01 三钉 + 窗口判定族 + 身份族（scheduler） | 全绿（含 TestProbeDeletedThenRebuiltSameNameDropsSnapshot / TestProbeForAccountDropsWriteWhenRemoved / TestProbeChainSameClientIdentity） |
| `git status --short --branch` | `## master` + 仅前端并列报告未跟踪文件；零改动既有文件 |

## 结论

1. **B88-01 持续复核闭环**：ProbeForAccount 回写段身份复核行为面正确（已删/同名重建整体放弃回写、识别槽双条件覆盖、锁序无变化），probe() 路径复用全一致；三钉实测绿。R89 的 O89-01（probe_identity_test.go 缺尾换行）本轮实证为工作区转换噪音，仓库内容尾随换行完好，自动闭合。
2. **身份防线 16 项矩阵延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点，矩阵保持准确。
3. **O86-01 api 抖动基线第五轮零复现**（race 全绿、connectex 零样本）；O88-01/02 记录复核通过；M87-01 窗口再评估维持 MINOR + 注释兜底。
4. **新角度扫查全绿**：配置热重载边界（值域校验 + 落库失败仍内存生效 + 引擎模板同步 + 并发热收敛）、手动报名/退选状态同步闭环、多账号登录闸门窗口推进、时间基准一致性抽核（残余本地钟点均同基自洽）、错误文案用户可理解性。
5. **唯一 OBSERVE**：scheduler.go 的 gofmt 检出为纯 CRLF 工作区转换噪音（仓库内容 LF 合规），无需任何提交动作。

工作树 `## master` 洁净。本轮无 CRITICAL/MAJOR/MINOR。
