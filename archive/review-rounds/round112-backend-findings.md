# R112 后端只读审查报告

## 头部信息

- **审查对象 HEAD**：`88fcee09c7f39c1ac5e23cc8e88671d7b0c6aac0`（master，工作区干净）
- **审查方式**：全程只读（grep/blame/Read + 定向 `go test -race`），零仓库文件修改
- **聚焦范围**：身份防线矩阵第二十七轮闭合 · OBSERVE-111-01 盯守 · B110-01 审计链第二轮盯守 · B110-02 观察复核 · O105-01 抖动基线 · 新契约角度（手动报名/退选 handler 全链路、store/数据库 schema/迁移契约、accounts 注册表生命周期综合走查）
- **审查耗时**：约 32 分钟（含三组后台 race 测试）

## 分级发现

无 CRITICAL / MAJOR / MINOR（本轮零新缺陷入账）。

### OBSERVE（延续观察）

**OBSERVE-112-01（延续 O105-01 抖动基线，zhidao 包全量 race 首轮 1 FAIL 低频残余实证）**
- 首轮 `go test -race -count=1 ./internal/zhidao/...` FAIL（1 个登录日志类测试失败，输出仅 4 行中文日志 + FAIL，无标准 FAIL 测试行头），随后同命令非 race 全量 PASS、`-race -count=5` 定向 Login 日志族 PASS、`-count=1` 二次全量 PASS。判为非产品缺陷的低频测试残余（F52-M2/M3/M6 根治后残余，`||` 重跑吸收）。
- 诊断日志为 TestLoginLogsFailureSummary 风格（"登录失败（共 3 次识别尝试）" 与 3 行识别成功日志并存，归属日志缓冲捕获断言与登录重试竞态）。本轮不立条，维持既有 O105-01 抖动基线观察——若后续多轮复现（≥2 轮不同根因）再升格。

**OBSERVE-112-02（延续 O111-01 观察项，MarkDone/RemoveDone 返回值丢弃防回归唯一锚点确认）**
- `handler.go:377` `_ = d.Sched.MarkDone(...)` 与 `handler.go:457` `_ = d.Sched.RemoveDone(...)` 返回值丢弃。两个函数体内落库点全 `if err != nil { log.Printf }` 零吞错：MarkDone（SaveSuccess :1973 / AppendLog :1976）、RemoveDone（DeleteSuccess :2020 / SaveRefused :2026 / AppendLog :2029）。返回值为 nil 恒量（函数全路径仅 `return nil`），`_ =` 当前不构成吞错。
- 防回归锚点确认：若未来某落库点改返回 err 需要同步两处调用点（:377/:457），并补充 StoreFailuresLogged 族测试把手动路径的落库错误。测试 `TestStoreFailuresLogged` 已覆盖 MarkDone/RemoveDone 落库失败必记日志。

**OBSERVE-112-03（新，B110-01 审计链持续行为观察）**
- B110-01 修复落地后，手动报名/退选失败三分支（ErrUnauthorized / read 类 / 其余业务失败）在 `handleElectiveSelect`（:348-376）与 `handleElectiveExit`（:430-453）各落 `Store.AppendLog(..., false)`，全部 `if err != nil { log.Printf }` 不吞错（6 处）。成功路径 MarkDone/RemoveDone 审计行（AppendLog isOK=true）保持无事前回归（:1976/:2029）。
- 观察点：手动路径失效分支落库日志文案 "教务令牌失效，自动重登中"（isOK=false）与自动链同文案会造出两条同文案日志（一条手动、一条自动），但动作维度不同（分别 `select`/`exit`），审计可区分，不构成重复留痕问题。维持观察，不立条。

## 必查项逐条结论

### 1. 身份防线矩阵第二十七轮闭合 —— 通过

**`git log 20c5882..HEAD -- backend/` COUNT = 1**，唯一改动 `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志），零产品改动链延续（R101-01 起）。API handler 纯增量（+26 行 AppendLog 补位），不涉身份防线本体。

- **sameClientFor 7 调用点逐点实证，行号零漂移**：
  - 定义 :199-210 注释 + :204 `func sameClientFor`；clientIdentity 定义 :212-223
  - :850 ProbeForAccount 回写段（openTimeDetected/acctData/acctDataAt 写前复核）
  - :1489 spawnChain 失效分支（ErrUnauthorized）
  - :1521 spawnChain 成功分支
  - :1551 风控退避分支（isRateLimitError）
  - :1571 窗口关闭分支（isWindowClosedError）
  - :1600 实时复核统一入口（回锁后先复查）
  - :1635 实时复核确证满员分支
  - 注释 :1599 "成功/失效/风控/窗口关闭/确证满员五分支对称" + :1631 注释当面印刻同族防线
- **写点全家福 12 类逐一对应防线**：
  - `openTimeDetected[acct]`（写 :856 ProbeForAccount，复核 sameClientFor + 锁内）、`openTimeDetected["*"]`（写 :1108 probe()，由 AnyClient 探测载体天然不带账号身份）
  - `acctData[acct]`/`acctDataAt[acct]`（写 :863/:864，复核 :850；删 :504/:505 PurgeAccount）
  - `tokenValid`（写 :1241 决策侧置位 / :1262 成功清零 / :1316 MarkTokenValid；删 :507 PurgeAccount）——:1241 在决策段先 ClientFor 复核（:1208），:1262 在写回段有 ClientFor 复核（:1254）
  - `reloginAt/reloginFail/relogging`（写 :1231/:1232/:1236 决策侧，入口 :1208 ClientFor 复核；:1261 成功写回在 :1254 复核之后；PurgeAccount 删 :508-510）
  - `inflight`（TryAcquireSubmit :1905 占位由 handler 锁内自持；spawnChain :1471 占位、:1490/:1501/:1513 清位均在 err 分支复核后/链顶身份捕获后；MarkDone :1951/RemoveDone :2003）
  - `done`（:1529 成功分支，:1521 身份复核后；:1934 MarkDone 处有 ClientFor 复核）
  - `refused`（:634 快照恢复 SetTargetsForAccount；:1939 MarkDone 解 refused；:2011 RemoveDone 前 ClientFor 复核 :1995）
  - `full`（:1714/:1767 markFullLocked 均从 :1555/:1575/:1639/:1458 受同族身份复核的分支调用；:1800 releaseFullIfFreedLocked 是防御性解封，按快照数据读删不涉及账号身份写回；MarkDone/RemoveDone/PurgeAccount 清）
  - `rateLimited`（:1714 仅在 :1555 风控分支经 :1551 身份复核后写；MarkDone :1957 清）
  - Courses 状态行（写点全部持锁；:1961 MarkDone 在 :1927 ClientFor 复核后；spawnChain :1474 submitted / :1502 failed / :1530 success / :1556 failed 各自在对应身份复核后；PurgeAccount :512-518 清）
- **偶发写点专项**：
  - MarkTokenValid（:1306）——仅手动登录成功 issueSession 调用，锁序 reloginMu→s.mu 对齐决策段，删除语义不写任何业务身份态
  - TryAcquireSubmit（:1896）——inflight 位仅占位，同步 Once 释放，不跨网络往返
  - releaseFullIfFreedLocked（:1781）——按快照数据型读删，不解封守卫：空快照/课程不在快照/明确余量才解封，不涉及身份写回
  - SubmitAll（:1342）——收集链前 ClientFor 过滤（:1355），尾部统一走 spawnChain 身份防线
- **手动四路 accountExists**：:245（课程读）/ :295（手动报名）/ :387（手动退选）/ :563（状态读）全部就位，`:1096-1105` 凭据表逐账号比对，判据同源。handleSetTargets 域内联复核 :479-499 与 accountExists 同源（另外内联口径）。
- **maybeRelogin 双侧复核**：决策侧 :1208 `ClientFor(acct)` 在一切 map 写入前；写回侧 :1254 重登成功后 `ClientFor(acct)`。偶发写点 MarkTokenValid/SubmitAll 不涉重登 map 写（SubmitAll 只 CollectFor）。测试族 `TestMaybeReloginDeletedAccountSkipsMaps`（:3401）与 `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（:3435）均当前绿（本轮定向 race 通过）。

**身份防线矩阵第二十七轮复核结论**：矩阵保持完全闭合。七大 err 归并写路径（成功/失效/风控/窗口关闭/实时复核统一/确证满员 + 探测定时三处决策侧）全部由 sameClientFor 或该点的存在性复核=/指针复核覆盖，写点与防线一一对应，零裸露写点。

### 2. OBSERVE-111-01 盯守 —— 维持（观察成立，当前不构成缺陷）

- 函数体内部落库点全 `if err != nil { log.Printf }`（实证见 OBSERVE-112-02 落库点清单）。
- `_ =` 当前恒 nil（两者函数全路径仅 `return nil`，`_ =` 不吞错）。
- "若未来补错误返回需同步两处调用点"仍是唯一防回归点：handler.go:377 + :457 两处 `_ = d.Sched.MarkDone/RemoveDone`。测试 `TestStoreFailuresLogged`（scheduler_test.go:86）已覆盖落库失败必记日志（含 MarkDone/RemoveDone 路径）。

### 3. B110-01 审计链持续盯守（第二轮）—— 通过（零漂移）

- 手动失败三分支 AppendLog 全在位：报名（:352 失效 / :364 read 类 / :371 业务失败）、退选（:433 / :442 / :449），全部 `if err != nil { log.Printf }` 不吞错。
- 成功路径审计行未回归：MarkDone :1976（select, isOK=true）、RemoveDone :2029（exit, isOK=true）。
- 双测试在位且当前通过：`TestManualElectiveFailureAppendsLog`（handler_test.go:1800，业务失败+退选双断言）、`TestManualElectiveReadErrAppendsLog`（:1866，read 类 FLUSH+Hijack 直断形态断言）。

### 4. B110-02 观察复核 —— 通过

- `reloginAt` 读写同基自洽：写（决策侧 :1231 `time.Now()` 本地钟；成功写回 :1261 `time.Now()` 本地钟），读（:1219 退避窗口判定 `time.Since(t)`、:1227 30s 节流 `time.Since(t)`），全部基于同一本地钟语义。本地钟仅用于"退出到下一次尝试的间隔"判定，不参与窗口开放判断/时间基对齐（后者由 `nowAligned` 统一），故无基准混用。失败计数与 30s 防抖间隔同为本地钟量纲，自洽。

### 5. O105-01 抖动基线 —— 维持（低频残余实证，不立条）

- `socketPreheat`/`readyProbe` 逐字符零漂移：socketPreheat（zhidao/client_test.go:25-30）net.Listen+Close；readyProbe api 版（handler_test.go:185-216）200ms×10 + 显式 2s 超时；zhidao 版（client_test.go:88-90）同宽栅栏；loginMockServer 构造后同步 readyProbe（:78）。
- 验证表（见下）：api 包 race 首轮通过；zhidao 包 race 首轮 1 FAIL（登录日志类），后续 3 组定向重新均 PASS → 低频残余确认，CI `||` 重跑吸收策略继续有效。

### 6. 新契约角度 —— 手动报名/退选全链路 + db/schema/迁移 + accounts 生命周期综合走查 —— 无新发现

- **手动报名/退选全链路状态码与 body 契约**：缺账（:296/:388）→ 缺有效账号（:303/:395）→ 解析失败（:309/:401）→ 无效课程ID（:313/:405）→ TryAcquireSubmit 在飞（:320/:412）→ CheckClassSelectable 快手复核（:328 报名专属）→ ClientFor 缺（:335/:420）→ 平台失败三支（:348-376/:430-453）→ 成功 `writeJSON(0, {msg, class_id}, msg)`（:378/:458）。全路径皆为 `writeJSON`（HTTP 200 + body.code），基础设施状态码仅用于 401/403/500/429/CSRF（B39-02/B40-01 家族，已抽查 :632/:845/:950/:1123/:1229）。zhidao 层 SelectClass/ExitClass 以 `Code!=0 || !IsOk` 双判成百返回（client.go:782/:807），doRequest 对 `code=-1` 统一 `ErrUnauthorized`（:464-465），handler 三支归类正确。
- **db/schema 迁移规范**：`Open`（db.go:15-37）流程 = 建表 → `migrateAddPublishMeta`（:43-57）→ `refuseLegacy`（:59-98）。`migrateAddPublishMeta` 逐列 `columnExists`（pragma_table_info）缺才 ALTER，纯增量不拒启动；`refuseLegacy` 缺列清单正在 :80（priority/allow_swap/task_log.account）已把 publish_name/begin_date 剔除（:77 注释明示边界）。`TestMigrateAddsPublishMetaColumns`（db_test.go:34）真实旧行数据迁移保留 + 两列补齐断言，本轮定向通过。
- **accounts 注册表生命周期**：`ensure`（:102）分配/注册、`ClientFor`（:115）/`AnyClient`（:138）/`AnyClientWithAccount`（:148）/`Registered`（:178）读取、`Remove`（:125 memory-first 摘除 client + order）、`Relogin`（:166 走 gateWait 全局闸门 + ReloginIfNeeded）、`gateTryAcquire`（:223 非阻塞准入门，LoginByPassword :244 收口）、`Restore`（:295 重启恢复 + 解密失败留痕 :306）、`LoginByPassword`（:243 失败只摘本次新建空壳 :264-273）。全部锁内读写（m.mu），与调度器身份防线（ClientFor 存在性 + sameClientFor 指针身份）衔接完整。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 20c5882..HEAD -- backend/` | 1 条（57bf401），COUNT=1 确认 |
| `git status --short --branch` + `git rev-parse HEAD` | 干净，HEAD=88fcee0 |
| `go build ./... && go vet ./...` | PASS（build+vet 零输出） |
| `go test -race -count=1 ./internal/api/...` | PASS（27.749s） |
| `go test -race -count=1 ./internal/zhidao/...`（首轮） | FAIL（登录日志类，低频残余，见 OBSERVE-112-01） |
| `go test -race -count=1 ./internal/zhidao/...`（再跑） | PASS（3.405s） |
| `go test -race -count=5 -run "TestLoginLogsFailureSummary\|TestLoginRetryWithinLimits\|TestLoginLogsAttempts" ./internal/zhidao/...` | PASS（1.892s） |
| `go test -count=1 ./internal/zhidao/...`（非 race 全量） | PASS（2.874s） |
| `go test -count=1 -run "TestLogin" ./internal/zhidao/... -v` | 6/6 PASS |
| `go test -race -count=1 -run "TestDeletedAccountRebuiltSameNameChainDrops\|TestMaybeReloginDeletedAccountSkipsMaps\|TestManualDoneClearsInflight\|TestStoreFailuresLogged\|TestWindowOpenSubmitsWithoutProbeReset\|TestAdminStatsWindowOpenedUsesScheduler\|TestSubmitSuspendedWhenOpenTimeCleared" ./internal/scheduler/...` | PASS（4.025s） |
| `go test -race -count=1 -run "TestMigrateAddsPublishMetaColumns\|TestOpenAndSchema" ./internal/db/...` | PASS（2.138s） |
| `go test -race -count=1 -run "TestSanitize\|TestIsReadErr" ./internal/zhidao/...` | PASS（2.184s） |

## 已核无缺陷清单

- 手动报名/退选失败三分支 AppendLog 落库均 `if err != nil { log.Printf }`，零吞错（6 处）。自动链 10 处 AppendLog 同规。（契约 17）
- `_ =` 仅存 :377/:457 两处（OBSERVE-112-02 维持），全仓无其他落库点吞错。
- reloginAt 本地钟读写同基，不参与窗口时间基。
- B101-01 sanitizeError 契约在位：doRequest :450 连接失败统一 `sanitizeError`，剥 url.Error 文本不含 idToken 段，Unwrap 保留判型；sanitize_test 断言 token 与 idToken= 均被剥除。
- httpDo 只重试 dial/write（isConnErrRetryable），read 错误不重试（幂等防线），业务/取消原样上抛。
- db 迁移规范：目标行数据不因缺列拒绝启动；refuseLegacy 缺列清单与迁移列边界注释清晰。
- gateWait/gateTryAcquire 双通道共享 gateUsed 计数，手动登录收口全局 doLogin 预算。
- tick 提交守卫（:1011-1035）：open.IsZero() && !opened 挂起 / WindowOpened=true 例外 / WindowClosed 挂起 / 黄金期 250ms 冲刺，与决策契约 1/31/32 完全对齐。
- windowClosedLocked 三判据单源（主判据 + 时钟≥3 + 幽灵窗口 EmptyProbeRuns≥3），StateForAccount 与 WindowClosed() 共用同一实现。
- markFullLocked/releaseFullIfFreedLocked 的解封守卫（明确余量才解、空快照/未知保持 full）与其注释契约一致。

## 结论

- **身份防线矩阵第二十七轮闭合（零改动前提核位通过）**：20c5882..HEAD 后端唯一改动为 B110-01 审计修复（57bf401），零产品改动链延续；sameClientFor 7 调用点行号零漂移，写点全家福 12 类逐一对应防线，偶发写点专项（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll）不涉身份写回缺口，手动四路 accountExists 与 maybeRelogin 双侧复核全在位。矩阵保持完全闭合。
- **OBSERVE-111-01 盯守确认**：`_ =` 当前恒 nil 不构成吞错，函数体内落库点全日志化；"未来补错误返回需同步 :377/:457"仍是唯一防回归点（测试已覆盖），维持观察。
- **B110-01 审计链第二轮通过（零漂移）**：手动失败三分支 AppendLog 六处全在位零吞错，成功路径审计行未回归，双测试当前绿。
- **本轮无 CRITICAL / MAJOR / MINOR**。OBSERVE 仅延续观察（zhidao 低频残余实证 + 111-01 + 112-03 审计行为）。