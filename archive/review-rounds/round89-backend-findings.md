# R89 后端只读审查报告

审查对象：xuanke-auto HEAD `8d1abe0`（R88 收官，进度 89/256）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round89-backend-findings.md），工作树保持 `## master` 洁净（git status 实证：仅前端并列报告的未跟踪新文件，零改动既有文件）。

审查方式：Read / Grep / Glob / Bash 只读命令（bg 全量 race 测试 / 双端交叉编译 / go build / go vet / gofmt / 全部定向测试分轮实测）。核心走读范围：backend/main.go、quit_shared.go、tray_quit_test.go、internal/{scheduler,accounts,api,store,zhidao,session,db,runtime,config,secure} 全量、cmd/ 三工具、web/embed.go；对比 B88-01 修复提交 ca08c46 diff。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O89-01：`probe_identity_test.go` 缺少尾部换行——git 提交内容即含 `\ No newline at end of file`（本轮实测，归属推断为提交器注入）

**证据：** `gofmt -l` 检出的两文件仅 2 处差异：(a) `scheduler.go` 整文件为 CRLF（.gitattributes 不存在 + `core.autocrlf=true` 下 gofmt 对 CRLF 整文件 diff——LF 归一后该文件 gofmt 合规，见构建验证表实证）；(b) `probe_identity_test.go` 的**最后一行末尾缺换行**（`gofmt -l` 检出的真实格式差异）。`git show HEAD:...` 检出的该文件同含 `\ No newline at end of file`——差异已随 B88-01 修复提交进入仓库（非工作区转换造成）。**影响：** 仅 cosmetic 格式项，`go vet`/`go test`/构建全不受影响。**修复方向：** 下次触碰该文件时顺手补一个尾部换行即可；不建议专门为此发提交。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| O89-01 归属 | `probe_identity_test.go` 缺尾换行的具体来源（本轮提交器/更早工具）——无法从 git 内容侧进一步归因；整体影响为纯格式噪音，建议按格式规范顺手修正即可。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **B88-01 修复行为面复核**（ProbeForAccount 回写段身份复核）：`scheduler.go:853-877` 回写段入口持 `s.mu` 先 `sameClientFor(acct, client)`，非同一身份（已删 / 同名重建）**整体放弃**写 acctData/acctDataAt/识别槽并返回 data（快照调用方仍拿数据、仅不写内存）；同数据源 `beginTimes` 识别槽覆盖只在身份通过后才发生——**身份复核通过但 beginTimes 为空不会误覆盖识别槽**（空分支不写、保留旧识别值，识别槽覆盖仅发生在 `len(data.BeginTimes)>0`）。锁序无变化（单把 s.mu，无嵌套）。三钉实测绿（TestProbeDeletedThenRebuiltSameNameDropsSnapshot / TestProbeForAccountDropsWriteWhenRemoved / TestProbeChainSameClientIdentity，见构建验证表）。 | 通过（实测 + 走读） |
| **probe()→ProbeForAccount 路径一致性**：probe() 内 per-account goroutine（scheduler.go:1073-1081）直接复用 `ProbeForAccount`——修复对新复核的覆盖自动全路径一致（该 goroutine 的 FindElectives 网络往返返回后同样走 sameClientFor）；不会裸直打上游。 | 通过 |
| **ProbeNow 同款缺口排查**：`ProbeNow`（:948-973）网络往返返回后写 `lastData/lastDataAt/lastProbe`（**全局**帧，非 per-account 专属），不写识别槽、只写全局帧；删号+同名重建的"年级串线/识别槽污染"风险不适用全局帧形态。HTTP 侧 `ProbeNow` 为管理员无目标账号全局浏览路径，运维语义=全局探测，无身份维度。 | 通过 |
| **身份防线全员清点矩阵（B43 族最后缺口的闭合复核）**——见下方详表 | 全部通过（走读 + 对应回归钉实测绿） |
| **O86-01 api 抖动基线延续（第四轮）**：bg 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` **12 包 exit 0**（api 28.964s / store 28.767s / scheduler 16.467s / zhidao 2.919s 实测）；后续 api/store 两包 race 二次复核全绿（22.477s/27.368s）；**connectex 零样本**（R86/R87/R88/R89 连续四轮零复现）。 | 通过（实测） |
| **O88-01/02 记录复核（sharedTransport TLS / Shutdown 反向慢连）**：`sharedTransport` 无 `TLSClientConfig`（client.go:22-34）——走标准库 nil→内部 defaultVerifyTLS / 系统证书池，与 `httpDo` 只重试 dial/write 的契约正交；权威过侧 Linux CGO=0 交叉编译带真实 TLS 通过。Shutdown 反向慢连极端尾部仍维持"Shutdown→main 返回"窗口结论，tick 无停止信号、在飞 final 提交以重启重试兜底（M87-01 注释完整覆盖两分支）。 | 通过（走读 + 实测） |
| **M87-01 窗口再评估（5s Shutdown vs 在飞 spawnChain）**：黄金期最大化场景下 Shutdown 不中断在飞 conn（StateActive continue），5s 超时后强杀；tick 每 300ms 一拍，窗口内少数 tick 可再 spawn 链——但进程强杀吞掉未落库 final、重启 RestoreDone 恢复已落 success，语义收敛。维持 MINOR + 注释兜底。 | 通过 |
| **SQLite 写路径锁纪律**：db.go Open 单连接（SetMaxOpenConns=1）+ 表结构/迁移/refuseLegacy 串行；store 事务（SetTargets/SaveSettings/DeleteAccount/CreateActivationCodes/ConsumeActivationCode）全部 `Begin→...→Commit` + `defer Rollback`，**无手工 `db.Exec` 于锁内跨网络**；scheduler 持 s.mu 时落库仅微秒级（成功分支两行 AppendLog/SaveSuccess），网络段一律先 Unlock（spawnChain 实时复核 1592）。无锁序反转（store 不持 scheduler 锁；scheduler 持锁调 store 是唯一方向）。 | 通过 |
| **探测/提交节流闸门一致性**：lastProbe 只归 probe()/ProbeNow 写（ProbeForAccount 不写）；lastSubmit 由 submitAll 对齐钟写、tick 读同源；probing 单飞在持 s.mu 时置位（零值守卫同款窗口）；probeSem cap 4 结构化信号量收敛 per-account 并发；openTimeForLocked 单快照复用防热改亚毫秒读取不一致（R74 同族）。 | 通过 |
| **日志容量与审计（task_log 增长边界）**：task_log 无清理（schema.sql 无 TRUNCATE），查询层已以 `id > (SELECT max(id)-20000)` 维持"最近 2 万条"窗口（store.go:227-231/422 两处）；无自动清理属已记录的运维边界（OBSERVE 级，历轮覆盖）。AppendLog 全部调用点带 `if err != nil`（spawnChain 六分支 / MarkDone / RemoveDone / markFull / handler / accounts）——零吞错。 | 通过 |
| **多账号共享状态隔离**：reloginResults chan cap 8 非阻塞发送（1283 select-default）；gateMu/gateUsed 全账号共享计数由 gateWait/gateTryAcquire/GatePump 三处一致维护（同一 gateWindow 复位、gateLoginPerMin 同步）；tokenValid/reloginAt/reloginFail/relogging/acctData/openTimeDetected/acctDataAt 全部 per-account map——PurgeAccount 全清。 | 通过 |
| **embed 资产与 SPA 兜底边界**：web/embed.go 的 SpaHandler 首行对 `/api/` 前缀整体 404（未注册 /api/xxx 绝不落 index.html）；新增路由后与 router.go 中 `/api/` catch-all（:194-199）一致——静态资源仍优先 mux 精确匹配，/api 下未知路径 JSON 404。 | 通过 |
| **撞名学生/管理员名判定（F52 族）**：占用 `adminName` 的撞名学生走教务登录正常签发普通会话，`Session.Admin=false`；前端仅依赖签发响应中的 `adminName` 字段标记管理令牌。后端 `IsAdminAccountName`/`requireAdminSession`/`allowAccountOverride` 全部走会话 `Admin` 标志而非账号名——撞名学生普通会话永远无法穿透管理员接口。 | 通过 |
| **登录/激活/票据/会话边界**：会话 12h TTL + sweeper（Close 用 sync.Once 等 sweepDone）；票据 5 分钟单次防重放（ConsumeTicket 消费即删）；激活 Profé 限流独立桶。 | 通过 |

### 身份防线全员清点矩阵（R88 教训落地——"每次补身份防线都要 grep 全部网络往返后持锁写点"）

| # | 写点 | 网络往返 | 复核类型 | 结论 |
|---|---|---|---|---|
| 1 | spawnChain 链顶（scheduler.go:1409-1421） | 取 client 前 | ClientFor 存在性 + 指针捕获 | 通过 |
| 2 | spawnChain 失效分支（:1493-1497） | SelectClass → ErrUnauthorized | sameClientFor | 通过 |
| 3 | spawnChain 成功分支（:1525-1529） | SelectClass 成功 | sameClientFor | 通过（写 done/状态/库行前） |
| 4 | spawnChain 风控退避分支（:1555-1558） | SelectClass 命中 rateLimit | sameClientFor | 通过 |
| 5 | spawnChain 窗口关闭分支（:1575-1578） | SelectClass 命中 closed | sameClientFor | 通过 |
| 6 | spawnChain 实时复核满员分支（:1639-1642） | classFullRealtime 网络段后回锁 | sameClientFor | 通过 |
| 7 | spawnChain 实时复核 ErrUnauthorized 分支（:1604-1624） | classFullRealtime 网络段后回锁 | sameClientFor | 通过 |
| 8 | maybeRelogin 决策侧（:1205-1215） | 发起前 | ClientFor 存在性 | 通过 |
| 9 | maybeRelogin 写回侧（:1258-1262） | Login 返回后 | ClientFor 存在性 | 通过 |
| 10 | ProbeForAccount 回写段（:853-858，B88-01 / M88-01） | FindElectives 返回后 | **sameClientFor（本轮修复）** | 通过（三钉实测） |
| 11 | ProbeNow 全局帧（:967-971） | FindElectives 返回后 | 无身份维度（全局帧非 per-account），B43 风险不适用 | 通过 |
| 12 | probe() 全局帧（:1110-1120） | FindElectives 返回后 | 同 ProbeNow（全局帧，无身份维度） | 通过 |
| 13 | MarkDone（:1931-1934） | 手动报名网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 14 | RemoveDone（:1999-2002） | 手动退选网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 15 | SubmitAll（:1359-1361） | 无网络（内存链构建） | ClientFor 存在性（下发链前过滤） | 通过 |
| 16 | Restore 路径（RestoreDone:611 / RestoreTargets:529 / RestoreRefused:630） | 无网络（启动恢复，注册表已就绪） | 不需（账号都在注册表） | 通过 |

grep 实证全部「网络往返后持锁写状态/落库」点均已闭合。无裸露写点。

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：`openTimeForLocked` 恒返回识别值本身（零值仅为无识别时）；识别过期只影响展示层（StateForAccount 的 `After(nowAlignedLocked())` 判定）；空快照不删槽、非空 beginTimes 才覆盖；B88-01 修复不改变该语义（身份通过 + beginTimes 非空双条件才覆盖）。 | 通过（实测 open_retain/open_detect 绿） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }`（spawnChain 六分支 / MarkDone:1976-1983 / RemoveDone:2024-2036 / markFullLocked:1773-1776 / handler 登录/配置/删除 / accounts 凭据加密 / maybeRelogin UpdateIDToken 等）；非落库静默仅 Prewarm/ProbeForAccount 探测丢弃（历轮已记录）。 | 通过 |
| **21（防寄生 = 指针身份比对）**：sameClientFor 208-214 + clientIdentity 219-228（reflect.ValueOf(c).Pointer()）；与 Matrix 1-10 闭合。 | 通过（回归钉实测绿） |
| **B41-01/B41-02（零值守卫让步 + 全分支身份族）**：spawnChain 六分支 + 实时复核第七分支 + 探测回写（第十一点）全部闭合；tick 零值守卫 `open.IsZero() && !opened` 让位于 WindowOpened 确认（scheduler.go:1015 例外分支注释明确）。 | 通过（TestWindowOpenSubmitsWithoutProbeReset / SubmitSuspended 双钉实测绿） |
| **36/37/43（maybeRelogin 决策侧复核、全分支清点、身份防线写回侧）**：决策侧存在性复核（:1212）+ 写回侧复核（:1258）+ MarkTokenValid 全路径锁序（reloginMu→s.mu）。 | 通过 |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（LoginByPassword）+ gateWait 阻塞（Relogin）+ GatePump 窗口推进，共享 gateMu/gateUsed；ResetGateForTest 仅测试夹具。 | 通过 |
| **B39-02/B40-01（writeJSONStatus 家族与状态码成族核对）**：六类基础设施路径（panic 500 / 会话 401 / 管理 403 / 限流 429 / CSRF-403 / 未知 API 404）+ config 落库失败 500；业务 100+ 点恒 200。 | 通过（router.go:194-199 `/api/` catch-all 404 含 HTTP 状态码家族为 2026-09 增量） |
| **18（识别引擎热切换必须同步模板）**：SetVision 保留当前引擎（WithRecognizer）+ SetRecognizer 写模板；applyCaptchaRecognizerFor 两处路由调用全走此一致性。 | 通过 |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `cd backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，12 包 exit 0（api 28.964s / store 28.767s / scheduler 16.467s / zhidao 2.919s）；connectex 零样本 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 两文件：scheduler.go（CRLF 整文件）/ probe_identity_test.go（缺尾换行）——见 O89-01；LF 归一后 scheduler.go gofmt 合规，实为工作区 CRLF 转换噪音 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build` | 通过（内嵌 ddddocr 双轨 + 托盘路径） |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` | 通过（纯 Go 交叉编译 + embed 资产） |
| 定向 race：B88-01 三钉（scheduler） | 实测全绿（1.434s） |
| 定向普通 + race：窗口判定族 | 全绿（TestWindowClosedState / TestSubmitSuspendedWhenOpenTimeCleared / TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler / 身份族） |
| 全量本地（-race 外四包二次） | api 31.942s / store 2.875s / zhidao 4.223s / accounts 0.682s 全绿 |
| db/config/secure/session/runtime 五包 | 全绿（1.465s/0.899s/0.949s/1.042s/0.866s） |

## 结论

1. **B88-01 复核闭环**：ProbeForAccount 回写段身份复核行为面正确——已删/同名重建整体放弃回写（acctData/acctDataAt/识别槽三不写），身份通过且 beginTimes 非空才覆盖识别槽（空 beginTimes 不破坏"关闭≠时间消失"），锁序无变化；probe() per-account goroutine 复用同一路径全一致。三钉实测绿。
2. **身份防线全员清点（R88 教训落地）**：逐点 grep 全部「网络往返后持锁写状态/落库」点（16 项矩阵），B88-01 为最后裸露写点，本轮闭合无新缺口。ProbeNow/probe() 全局帧无身份维度风险，正确性在全局语义侧成立。
3. **O86-01 api 抖动基线第四轮零复现**（race 全绿、connectex 零样本）；O88-01/02 记录复核通过；M87-01 窗口再评估维持 MINOR。
4. 新角度扫查全绿：store 锁纪律、节流闸门时间基一致性、task_log 容量边界、多账号共享状态隔离、embed/SPA 兜底、撞名判定族。
5. 唯一 OBSERVE：probe_identity_test.go 缺尾换行——随 B88-01 提交进入仓库的格式噪音，顺手修正即可。

工作树 `## master` 洁净（仅前端并列报告未跟踪）。本轮无 CRITICAL/MAJOR/MINOR。