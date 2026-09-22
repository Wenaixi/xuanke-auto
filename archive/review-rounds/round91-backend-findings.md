# R91 后端只读审查报告

审查对象：xuanke-auto HEAD `7173970`（R90 收官，进度 91/256）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round91-backend-findings.md），工作树保持 `## master` 洁净。

审查方式：Read / Grep / Glob / Bash 只读命令（bg 全量 race 测试 / go build / go vet / gofmt / 定向走读）。核心走读范围：backend/main.go、quit_shared.go、web/embed.go、internal/{api(handler+router),scheduler,accounts,store,session,zhidao} 全量。新契约角度本轮聚焦：静态资源与 SPA 兜底边界、sweeper/会话/票据生命周期、writeJSON 家族响应完整性、settings 全量替换与单账号原子性、ConcurrentMap 语义。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O91-01：runtime.Config 中心「Get 返回拷贝」语义走读记录（进程内配置热重载读一致性），无缺陷

**证据（本轮走读，runtime/config.go 全量）**：`runtime.Store` 是 `sync.RWMutex + Config 单值` 的最小配置中心——`Get`（:35-39）RLock 下返回 `s.c` 值拷贝，调用方拿到的快照不受后续 Update 影响（**读一致性：每次读必为完整一致的最新快照，无撕裂读**）；`Update`（:42-48）写锁内应用修改函数，多字段并发热改原子整体生效。全仓读点集中在 api 层（handleLogin/handleActivate 经 `activationEnabled()` 读开关、handleAdminConfig/handleAdminStats 读引擎与并发、applyCaptchaRecognizerFor 读引擎），写点唯一=handleAdminConfig PUT——读快照与热改时间安排无冲突（改完下一条读即新值，旧请求持旧快照继续处理属正常语义，无脏读）。**非缺陷，仅记录防未来误改**（若未来要"按 key 分片配置 + 原子更新部分字段"，此单值结构不适用，需换 map 结构并注意快照语义；当前不触发）。

### O91-02：settings 表 SaveSettings 全量替换 + LoadSettings 全读的事务语义，重启一致性成立（走读推断）

**证据（本轮走读）**：`SaveSettings` 在**单事务**内 `DELETE FROM settings` + 循环 INSERT 全量覆写（store.go:351-366），原子性由 SQLite 单写者 + Begin/Commit + defer Rollback 保证——期间任一 INSERT 失败整体回滚，旧值完整保留，绝无"删了一半 + 回滚一半"的中途态；`LoadSettings`（store.go:369-384）只读全量 k/v 由 main 恢复（main.go:79-111）。**restart 一致性**：配置每次热更新走同一全替换事务，重启后 LoadSettings 拿到的永远是"最近一次成功提交的完整快照"，无部分配置碎片。**边界**：SaveSettings 的 DELETE 是全表级，会清掉未来可能新增的 settings 键——当前 settings 只有本族 5 键（activation_enabled/vision_*/captcha_engine/captcha_concurrency），全替换语义无缺失；未来若引入须持久化且与热更新无关的新配置键，需改为"增量 Upsert"或纳入 SaveSettings 清单，当前不触发。**走读推断**，未实测（无直接复现场景）。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无新可疑项 | 本轮走读未发现需主控深度核实的存疑点；全部走读闭环。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续**（scheduler.go 全量逐点复核）：spawnChain 链顶双 ClientFor（:1392 + 取 client 后 :1409-1415 二次判 + :1421 指针捕获）、六分支 sameClientFor（失效 :1493 / 成功 :1525 / 风控 :1555 / 窗口关闭 :1575 / 实时复核满员 :1639 / 实时复核 ErrUnauthorized :1604），maybeRelogin 决策侧（:1212 ClientFor 存在性）+ 写回侧（:1258），ProbeForAccount 回写段（:854 sameClientFor + :859-862 识别槽双条件覆盖），ProbeNow 全局帧（:967-971）/ probe() 全局帧（:1110-1120 无身份维度，全局语义正确），MarkDone（:1931）/ RemoveDone（:1999）/ SubmitAll（:1359）ClientFor 存在性，Restore 路径（:611/:529/:630 无网络不需身份）。`clientIdentity` 用 `reflect.ValueOf(c).Pointer()`（:219-228）；`sameClientFor` 判 nil 恒 false（:208-214）。**无新裸露写点** | 通过 |
| **B88-01 修复持续复核**：ProbeForAccount 回写段持 s.mu 内先 `sameClientFor`（:854），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:859-862）；`s.acctData` nil 守卫在身份复核后、写入前（:863-866）；锁序无变化（单把 s.mu，无嵌套）。probe_identity_test.go 三钉本轮 race 全量回归通过（见构建验证表）。 | 通过（实测 + 走读） |
| **O86-01 api 抖动基线第六轮**：bg 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` **11 包 exit 0**（api 27.446s / store 47.274s / scheduler 15.292s / zhidao 3.176s 实测）；**connectex 零样本**（R86-R91 连续六轮零复现，grep 日志零匹配）。 | 通过（实测） |
| **O90-01 CRLF 噪音复核**：工作区 `internal/scheduler/scheduler.go` 仍为全行 CRLF（python 实测 2053 CRLF / 2053 LF），`git show HEAD:backend/...` 仓库内容为全 LF；`gofmt -l .` 仅检出该文件（工作区转换噪音），对 git 仓库版本 gofmt 零输出。probe_identity_test.go 已干净（尾随换行完好）。**无新变化、无需动作**。 | 通过（实测） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：进程退出强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **静态资源与 SPA 兜底边界**（本轮新角度）：web/embed.go `//go:embed all:dist` + fs.Sub 解析（构建期失败 panic）；SpaHandler 先判 `/api` 前缀整体 404（含精确 `/api`），文件存在走 FileServer（自动 Content-Type 与缓存协商），不存在回退 index.html，index 读失败 404。与 router.go 的显式 `/api/` 404（:194-199 writeJSONStatus 先设头再 WriteHeader）双门闭合——所有 API 路径绝不落 SPA。**无泄露 index.html 当 API 响应的路径**。 | 通过 |
| **会话/票据生命周期**（本轮新角度）：session.Store 12h TTL + sweepLoop 每 5 分钟清扫过期会话与票据（:57-70）；`ttl<=0` 不启动清扫（零 TTL 测试场景，存储无泄漏风险，表小）；Ticket 5 分钟 TTL + 单次消费（ConsumeTicket 消费即删 + 已用/过期/账号不匹配三态返回错误，:118-138）；RevokeAccount 锁内遍历吊销账号全部会话（删账号立即失效，等不到 12h）；Close sync.Once 防双关。**票据防重放 + 会话吊销边界闭合**。 | 通过 |
| **writeJSON 家族响应完整性**（本轮新角度）：writeJSON（:76-79）恒 200 + body.code 业务语义；writeJSONStatus（:85-89）先设头再 WriteHeader（前一轮 404 错标修复点，:198 注释明确 httptest 假绿陷阱）；recoverMiddleware panic 恢复统一 500（:1203）；securityHeaders 三头（nosniff/DENY/no-referrer，:1211-1218）；router 六类基础设施路径状态码成族（401/403/429/404/500，router.go:72-198）。**客户端契约纯读 body.code、HTTP 状态码纯增强，双通道无矛盾**。 | 通过 |
| **多账号资源边界**（本轮走读）：spawnChain 单链独立 goroutine、chainMu 活跃标记防双链（key=acct+publishID，:1395-1408）；probeSem cap 4 封顶 per-account 探测并发（跨批不叠加，:1073-1079）；reloginResults chan cap 8 非阻塞发送（:1282-1285）；chains delete 在 defer 内（:1404-1408）；PurgeAccount 全清该账号 12 张 map + Courses 行（:499-523）。**账号维度隔离无泄漏**。 | 通过 |
| **配置持久化对称性**（本轮走读）：settings 写入=PUT /api/admin/config 单事务全替换（store.go:351-366）；读取=main 启动 LoadSettings 全读 + Runtime.Update 恢复（main.go:77-111）；vision_key 明确 enc: 前缀 + 明文拒绝加载（main.go:87-97）；目标凭据/成功/退选各表读写对称（SetTargetsForAccount/LoadTargetsForAccount、SaveSuccess/LoadSuccess/DeleteSuccess、SaveRefused/LoadRefused/DeleteRefused 全对齐）。 | 通过 |
| **会话安全细节**（本轮走读）：randToken crypto/rand 32 字节 hex + 熵源故障 panic 拒绝签发可预测令牌（session/store.go:224-231）；newActivationCode 8 字节 64bit 熵（handler.go:690-697）；maskKey 敏感值回显脱敏（:704-712）；登录/激活独立限流桶（loginLimiter 每 IP 5/分钟，互不锁死）+ GatePump 30s 窗口推进 + gateTryAcquire/gateWait 共享预算。 | 通过 |

### 身份防线全员清点矩阵（R91 复核——R90 矩阵延续）

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
| 10 | ProbeForAccount 回写段（:854，B88-01） | FindElectives 返回后 | sameClientFor | 通过（六轮实测绿） |
| 11 | ProbeNow 全局帧（:967-971） | FindElectives 返回后 | 无身份维度（全局帧） | 通过 |
| 12 | probe() 全局帧（:1110-1120） | FindElectives 返回后 | 同 ProbeNow | 通过 |
| 13 | MarkDone（:1931-1934） | 手动报名网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 14 | RemoveDone（:1999-2002） | 手动退选网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 15 | SubmitAll（:1359-1361） | 无网络（内存链构建） | ClientFor 存在性（下发链前过滤） | 通过 |
| 16 | Restore 路径（RestoreDone:611 / RestoreTargets:529 / RestoreRefused:630） | 无网络 | 不需（账号都在注册表） | 通过 |

## 契约抽查表（抽查 6 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:431-442），空快照不删槽、非空 beginTimes 才覆盖（:859-862 probe 全局 + :1110-1114）；识别过期只影响展示层（StateForAccount:714-720 After 判定），tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1015-1017）。 | 通过 |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:922-943）+ 主判据带 10s 裕量（:1150）+ EmptyProbeRuns 入账同 10s 裕量（:1159）；StateForAccount 与 WindowClosed() 共用同一实现（:707/:914）。 | 通过 |
| **21（防寄生 = 指针身份比对）**：sameClientFor + clientIdentity（reflect.ValueOf(c).Pointer()）与 Matrix 1-16 全闭合（spawnChain 六分支 + 实时复核 + 探测回写 + 决策/写回双侧）。 | 通过（回归实测绿） |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（LoginByPassword:244）+ gateWait 阻塞（Relogin）共享 gateMu/gateUsed 同一窗口计数；ResetGateForTest 仅测试。 | 通过 |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（spawnChain 六分支 / MarkDone / RemoveDone / markFullLocked / SetTargetsForAccount 落库与 DeleteRefused / maybeRelogin UpdateIDToken / handler 登录/配置/删除 / accounts SaveCredential 失败 / config LoadSettings 失败 log 记录不 panic）。 | 通过 |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御（:29-35）；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:201-205）；tray_quit_test.go 回归钉存在。 | 通过 |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `cd backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，11 包 exit 0（api 27.446s / store 47.274s / scheduler 15.292s / zhidao 3.176s）；connectex 零样本 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音；git 仓库内容 LF 合规零检出——O91 对齐 O90-01） |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build ./...` | 通过（内嵌 ddddocr 双轨 + 托盘路径） |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` | 通过（纯 Go 交叉编译 + embed 资产） |
| `git status --short --branch` | `## master` + 仅本轮报告文件未跟踪；零改动既有文件 |

## 结论

1. **身份防线 16 项矩阵延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段复核（sameClientFor + 识别槽双条件）六轮 race 回归全绿。
2. **O86-01 api 抖动基线第六轮零复现**（race 全绿、connectex 零样本）；O90-01 CRLF 噪音无新变化、无需动作；M87-01 窗口再评估维持 MINOR + 注释兜底。
3. **新角度扫查全绿**：静态资源与 SPA 兜底双门闭合、会话/票据生命周期（12h TTL + 5min 票据单次防重放 + RevokeAccount 立即吊销）、writeJSON 家族响应完整性（先设头再 WriteHeader + 成族状态码）、多账号资源边界、配置持久化读写对称性；会话安全细节抽查闭环。
4. **新增 OBSERVE 两条**：O91-01 ConcurrentMap `numGet` 恒 1 语义记录（info 共享读，历史定案，非缺陷防未来误改）、O91-02 settings 全替换事务的重启一致性走读推断（当前 5 键全替换无缺失）。
5. **无 CRITICAL / MAJOR / MINOR**，本轮零代码修改需求。

工作树 `## master` 洁净。本轮为纯观察轮（与 R90 同型），无代码修改建议提交。