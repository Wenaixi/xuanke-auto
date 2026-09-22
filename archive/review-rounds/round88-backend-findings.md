# R88 后端只读审查报告

审查对象：xuanke-auto HEAD `8ee1490`（R87 收官，进度 88/256；本轮不存在代码改动，起点即 R87 收官提交）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round88-backend-findings.md），工作树保持 `## master` 洁净（git status 实证：仅前端并列报告的未跟踪新文件，零改动既有文件）。

审查方式：Read / Grep / Glob / Bash 只读命令（bg 全量 race 测试 / go build / go vet / gofmt / Windows 目标交叉编译）；对照本机 Go（`C:\Program Files\Go`）标准库 net/http `Shutdown → closeIdleConns` 源码逐行复证 B86-01/M87-01 的退出链路窗口语义。核心走读：quit_shared.go、main.go 全量、tray_windows/linux/linux_cgo0/other 四文件、tray_quit_test.go 三回归钉、scheduler.go（PurgeAccount/spawnChain 六身份分支/maybeRelogin/captcha limiter/runtime/config）、accounts/manager.go（gateWait/gateTryAcquire 双通道共享计数）、api/router.go（writeJSONStatus 家族 + JSON 门覆盖 + 限流）、api/handler.go（手动报名/退选/删除账号/配置热改）、session/store.go（sweeper/票据）、store.go（事务/DeleteAccount 六表）、db.go（迁移/refuseLegacy）、zhidao/client.go（doRequest/httpDo/FindElectives 兜底/StudentCounts）、web/embed.go（SPA /api 门）、cmd/ 三工具、config.go（默认引擎）。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

### M88-01（走读推断）：`ProbeForAccount`（scheduler.go:816-870）探测期间管理员执行"删号 + 同名重建"，在飞 `FindElectives` 网络往返（最长 15s）返回后仍把课程快照写进回收站，同名重建后的新账号会"上一秒已删、下一秒课程恢复"

**文件 + 行号：** `backend/internal/scheduler/scheduler.go:824-869`（ProbeForAccount 的 `client.FindElectives()` 网络段：839-868 回写 `acctData[acct]=data` / 写 openTime 识别槽 / 日志）；与 PurgeAccount（:499-523）清理族对照。

**触发场景推演（具体输入序列）：**
1. 管理员在账号管理页删除账号 A（`handleAdminDeleteAccount`：Remove → PurgeAccount → DeleteAccount → RevokeAccount，memory-first）。
2. 删除发生在账号 A 的签名下；但注意——**探测定时三处（ProbeForAccount 819-829 前、ProbeNow、probe 内 per-account goroutine）都只在上游（FindElectives 返回 ErrUnauthorized）时查 `ClientFor(acct)`，且 `probe()` 里的 per-account 每账号 goroutine **没有账号存在性预检**，仅拿到 `AccountsWithTargets()` 的账号名**。若该 goroutine 恰在 1 之后、删号完成**前**已调用 `ProbeForAccount`（网络往返中），删号后 `PurgeAccount` 清空 `acctData/acctDataAt`，同名重建（换绑/误删加回）后新账号注册表重装 → 该在飞探测**此时**返回，`acctData[acct]` 写回**重建后的新账号名条目**，重建账号"课程立刻显示"。
3. 更直接：同名重建**后**、新账号自己的首轮 `ProbeForAccount` 又在旧链回写**前**进入网络段（两链并发）——旧链数据与新链数据争夺同一 `acctData[acct]` 条目，旧链可能覆盖新链（年级串线/过期数据）。
4. 删除本身无身份比对需求（`ProbeForAccount` 的网络往返短暂于典型重登 2 分钟，且快照仅是内存展示层）——**不是落库污染、不是 done 污染**，重启 Restore 读库不受影响。影响面 = 内存展示态的一次过期快照 + openTime 识别槽可能被旧批次时间覆盖（识别槽覆盖风险同族）。

**修复方向（供主控评估）：** 参考 B43-01 族已有防线（relogin 决策侧复核、spawnChain 全分支指针身份比对），为 `ProbeForAccount` 在"锁内回写前"补 `ClientFor(acct)` 存在性复核 + 可选的 `sameClientFor` 指针身份比对（与 spawnChain 同构）；`probe()` 每账号 goroutine 入口补账号存在性预检。或维持现状并按"展示层快照允许短暂过期、同名重建账号下个探测周期自愈"记录语义（自动链的 course 状态不读 acctData 写 done——无实际竞态纵深）。

**严重度论证：** MINOR——删号+同名重建"金牌期外"是低频运维组合（正常部署不收紧），影响限内存展示层课程列表一帧，重启/下个探测自愈；无 done/库行/会话污染（自动链的成功分支才关乎真实选课，且那里已有六分支防线）。仅"课程恢复一瞬间看到旧课"这一过期内存残留与识别槽错值可观察。

## OBSERVE

### O88-01：`sharedTransport` 显式 `TLSClientConfig` 恒 nil——连接活性自愈族（契约 40 / F52 族）唯一未显式声明的边缘项，走读核算持默认语义、无需补

**依据：** `zhidao/client.go:22-34` 无 `TLSClientConfig` 字段，标准库 `https.Transport.Clone`/DefaultTransport 走 nil → 内部 defaultVerifyTLS / 系统证书池；与契约 43/F52 的 `httpDo` 只重试 dial/write、读错误上抛完全正交。`SyncServerTime:Prewarm:doRequest` 全部复用同池。权威过侧（mac/Linux CGO=0 交叉编译）与全仓测试均带真实 TLS 通过——无缺陷，仅记录"显式性"观察。

### O88-02：`srvShutdown` 超时 5s vs `SpShutdown` 进程尽退出在"反向慢连"形态下极端尾部（走读，不实测）：Shutdown 返回后、main 返回路径上，`time.Sleep(loginTimingFlat)` 等 300ms 级短持锁窗口可能再跑一拍 tick——权当未来"无缝退出流程"设计时的半度校准基线

**依据：** `main.go:193-199` 与调度器 tick（300ms 间隔 ticker）无停止信号（M87-01 记录）。Shutdown 返回 ErrServerClosed 后 main 走 `return`，tick 若此刻恰在 `submitAll` 内已把新链 spawn 出去 → 进程强杀吞掉该次提交（重启重试兜底）。前 M87-01 已记录同族 db 一致性窗口，此处无需新建缺陷——仅确认该窗口的**理论内客体边界**（300ms tick + 5s 超时实测量级比：窗口宽度稳定），供任何未来退出流程重构参考。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| M88-01 触发频率 | 删号+同名重建"与在飞探测同帧"的触发门槛——主控按部署运维节奏需裁决是否需按 B43-01 同族补防线（低概率但结构同构，建议仓库内以走读记录 + 语义注释兜底即可报纯观察）。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **B86-01 托盘退出闭环复证**（含 M87-01 窗口）：`quitApplication` 见 tray_windows.go:63-66/quit_shared.go；`srv.Server.Shutdown` 源码（`C:\Program Files\Go\src\net/http\server.go:3121-3176`）复证：closeListenersLocked → listenerGroup.Wait → closeIdleConns 幂等循环，**StateActive 连接不强制中断**（:3213 `if st != StateIdle …continue`），5s 超时后 ctx.Err 返回、不关闭在飞 conn；main 解锁由 ListenAndServe 的 `trackListener` 的 `shuttingDown` 判定（:3417-3418/:3582）返回 ErrServerClosed 完成——窗口由"在飞若超时前恰好返回则写状态、否则进程强杀"两分支共同界定，M87-01 语义准确。 | 实测通过（标准库源码行号实证 + tray_quit_test.go 三钉绿） |
| **M87-01 注释兜底充分性**：main.go:191-192 注释"尽力优雅语义…最后时刻的提交结果以重启后重试为准（RestoreDone 只恢复已落库的成功）"——未来轮次（R89+）读到该窗口边界时可直接据注释理解"Shutdown 期间 tick 仍可跑、进程最终强杀在飞"的边界。 | 通过（注释完整含两分支语义） |
| **Shutdown 期间 tick 交互**：tick goroutine 不感知 Shutdown（Go 标准库语义：tick 独立于 http.Server），停摆仅由进程强杀完成——窗口仅在"Shutdown 返回→main 返回"这一 main 线程途经态短暂开合；无闸门可加（低成本收益趋零，R87 已裁决）。Ctrl+C/杀进度均走系统强制路径，与托盘退出的"尽力优雅"同边界。 | 通过（走读） |
| **O86-01 api 抖动基线延续**：`go test -race -count=1 -p 1 -timeout 900s ./...` 全 12 包 exit 0（store 39.840s / api 23.355s / scheduler 15.319s 实测）；connectex 零样本。 | 通过（实测） |
| **O87-01/02 记录复核**：快照满员分支三重论证（scheduler.go:1451-1455）：锁内即刻写、无网络往返慢写窗口（1475-1504 同锁段）、PurgeAccount 清 full——与 R87 记录一致，复证成立；托盘退出时序同 R87，B86-01 三钉当前仍绿。 | 通过（走读复证） |
| **srv.Shutdown 与在飞 spawnChain/手动报名交互再推演（黄金期 250ms 冲刺 + 15s SelectClass + 托盘退出）**：概率极低（用户点退出与在飞报名同帧）；分叉窗口（超时前请求恰好返回）内链写 failed/success 与平台结果可能不一致，受 M87-01 + 注释兜底 + 重启重试三重语义覆盖。风险等级维持 MINOR（理论可观察、无恶化）。 | 通过 |
| **JSON CSRF 门覆盖完整性（B40-01 家族）**：`requireJSONBody` 覆盖 POST /api/electives/select+exit、PUT /api/targets、POST admin codes/config（带 body 侧）；DELETE codes/accounts 经"空 body 合法"语义wedge 去门（router.go:154-156 注释明确 REST 语义）；登录/激活两处独立手写 jsonContentType 门 + 429 限流（router.go:99-125）。五类基础设施状态码（panic 500/会话 401/管理 403/限流 429/CSRF-403/未知 API 404）全走 writeJSONStatus，业务路径 writeJSON 恒 200——成族核对无缺口。 | 通过 |
| **writeJSONStatus 家族与业务路径分界**：writeJSONStatus 六处调用点（panic/需求 401/admin 403/限流 429/CSRF-403/未知 API 404 + stats 目标数失败 500 + config 落库失败 500）全为基础设施/服务端持久化错误；100+ 业务调用点仍 writeJSON 恒 200。前端 client.ts 零处读 HTTP status（历史核实，前端报告同轮复证）。 | 通过 |
| **don't-gate 一致性**：gateTryAcquire（manager.go:223-235）与 gateWait（49-64）共享 gateMu/gateUsed 计数、先 Lock 查再消耗、无死锁；B42-01 全入口收口（LoginByPassword:244）+ Relogin:173 阻塞收敛。 | 通过 |
| **Promo-active 热重载边界**：runtime.Store Get/Update 读写锁；Update 闭包持写锁执行、闭包内不嵌套锁；handleAdminConfig PUT 值域校验（引擎/并发/温度），落库失败下 dispatch 不跳过——"落库失败必记日志 + 500"族。失败回退窗口（Vision 引擎热切换）无残留（manager.SetVision 前保留引擎 + SetRecognizer 先处理模板）。 | 通过 |
| **session sweeper / 票据 / 会话 TTL**：Close 用 sync.Once + 等 sweepDone 防泄漏（12h TTL）；票据 5 分钟单次防重放对标 CLIENT 契约。 | 通过 |
| **store.DeleteAccount 六表全清 + 审计日志保留**：credentials/accounts/targets/success/refused/activations 六表 + 激活码不删；删除账号时 AppendLog 保留审计行。 | 通过 |
| **db.go 迁移/拒绝启动**：迁移先 refuseLegacy 才执行（Open 顺序）、缺列清单剔除已迁移列、`TestMigrateAddsPublishMetaColumns` 实测通过；refuseLegacy 的"旧 v2/v3 形状拒绝"清单完整。 | 通过（实测 db 包绿） |
| **home/activate 一致性**：issueSession 同时 SaveAccountName + Create + MarkTokenValid；未激活才 CreateTicket（ticket 绑定账号）；激活必携 ticket 且一致性。 | 通过 |
| **handleAdminStats 缺数据源失败整体 500**（含 token_valid 逐账号 TokenValidFor 查询）；window_opened/window_closed 与学生端 /state 同源。 | 通过 |
| **mock-client? 校验**：scheduler.Client 接口最小（5 方法），fakeClient 满足接口；无超量面。 | 通过（构建 + race 全绿佐证） |
| **temp/native_ocr 双轨**：native_ocr.go（windows+cgo）//go:embed 三资源 + dumpIfDiff 幂等释出；native_ocr_stub.go（!windows!cgo）回退；CrossT 构建经 CGO=1 win build 验证。 | 通过 |

## 契约抽查表（抽查 8 条相关路径，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）** | 通过（probe/ProbeForAccount 非空 beginTimes 才覆盖、空快照不删槽；openTimeForLocked 恒返回识别值、展示层判 known） |
| **15（btn_type 状态机/满员同源）**：选中 as 本路由 `classFullInSnapshot` 用 `max_count>0 && selected>=max` | 通过（走读 classFullInSnapshot:1716-1723 + releaseFullIfFreedLocked:1786-1804 双源一致；CountEntry.MaxCount 恒 0 兜底注释明确） |
| **17（落库失败零吞错）** | 通过（spawnChain 六分支 / MarkDone:1967-1974 / RemoveDone:2012-2027 / handler / accounts 全部 `if err != nil { log.Printf }`；非落库静默仅 Prewarm/ProbeForAccount 探测丢弃，历轮已记录） |
| **21（hutuchain 防寄生 = 指针身份比对）** | 通过（sameClientFor 208-214 + clientIdentity 219-228 + 六分支全部闭合 + B41-01 族 7 条同名重建测试绿） |
| **36/37/43（maybeRelogin 决策侧复核、全分支清点、身份防线写回侧）** | 通过（maybeRelogin:1203 存在性复核 + 六分支 sameClientFor + ProbeForAccount 网络往返不写 map 的锁定） |
| **B42-01（doLogin 全入口统一频率闸门）** | 通过（LoginByPassword:244 gateTryAcquire 非阻塞 + Relogin:173 gateWait 阻塞，共享 gateMu/gateUsed 计数一致） |
| **B39-02/B40-01（writeJSONStatus 家族 + 状态码成族核对）** | 通过（六类基础设施路径全 true 状态码 + 业务 100+ 点恒 200；delete 无 body 合法英文注释） |
| **43（连接活性自愈只覆盖连接层）** | 通过（httpDo:474-488 + isConnErrRetryable:492-501 只重试 dial/write 一次、read/业务错误上抛；doRequest 与 captcha 两处复用） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `CD backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，12 包 exit 0（store 39.840s / api 23.355s / scheduler 15.319s）；connectex 零样本 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 零输出 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build` | 通过（内嵌 ddddocr 双轨 + 托盘路径） |
| `go test -count=1 ./internal/db ./internal/session`（定向迁移/会话） | 通过（db 0.642s / session 0.515s） |

## 结论

1. **B86-01/M87-01 复核闭环**：标准库 `Shutdown` 源码逐行（server.go:3121-3176 + closeIdleConns:3204-3223）复证——Shutdown **不强制中断在飞连接**（StateActive 分支 continue），5s 超时返回后进程强杀由 main 返回承担，M87-01 的窗口语义准确；main.go 注释完整覆盖两分支，未来轮次可据此理解边界。三回归钉实测绿。
2. **O86-01 api 抖动本轮零复现**（race 全绿、connectex 零样本、无 flake 重跑）；**O87-01/02 快照满员三重论证复证通过**。
3. **新发现 1 条 MINOR（M88-01，走读推断）**：`ProbeForAccount` 在"删号+同名重建+在飞探测"同帧时会把旧快照写进重建账号的内存展示帧（识别槽亦可能被旧批次时间覆盖）——影响限内存展示层、重启/下个探测自愈，无 done/库行/会话污染。建议按 B43-01 同族补防线（锁内回写前 ClientFor + 指针身份复核）或记录语义兜底，由主控评估。
4. 新角度扫查全绿：JSON 门成族、writeJSONStatus 分界、gate 双通道一致性、runtime 热重载边界、store 事务/DeleteAccount 六表、db 迁移、session 票据、web SPA 门、cmd 三工具、config 默认引擎。

工作树 `## master` 洁净（git status 仅前端并列报告未跟踪，零改动既有文件）。本轮无 CRITICAL/MAJOR。