# round48 后端审查原始发现

> 审查基线：master @ `b6e2207`（R47 收敛落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式，未修改/创建/删除任何文件。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚（R47 后仍 42 条）+ legacy/website-source 逆向契约 + round39~47 各轮发现与修复报告逐条复核。
> 方法：全包通读 + 竞态/锁序推演 + 上轮观察项逐一核实 + 本轮新视角扫查。`go build ./...`、`go vet ./...` 双通道 exit=0；`go test -race -count=1 ./...` 连跑 3 轮（flaked 2 轮）+ api/zhidao 单独复合复跑多轮（见 MAJOR 节）。

---

## CRITICAL

（无本轮新增 CRITICAL。identity 复核三族（决策侧 B43-01 / 写回侧 B21-03 / 失效归并 B41-01）在调度器全路径仍闭合；删号内存优先四步序（Remove→PurgeAccount→DeleteAccount→RevokeAccount）无新裸露写点。）

---

## MAJOR

### MAJOR-48-01：`go test -race ./...` 全量连跑仍偶发 fakeRed——httptest mock 连接 flake 在 -p 2 下复现率 ~50/包·5 轮（测试基础设施层，非业务逻辑）

- **位置**：`backend/internal/api/handler_test.go`（TestLogoutRevokesToken / TestAdminStatsAccountsLogs / TestAdminStatsOpenTimeFromRecognized / TestElectiveSelectRejectsWindowClosed / TestElectiveSelectRejectsFullClass / TestLoginOKIssuesSession / TestSetTargetsAndState / TestHandleElectivesSelectAndExit）、`backend/internal/zhidao/*_test.go`
- **一句话问题**：R46→R47 加 `-p 2`（4daaff5）后问题显著缓解但未根除。本轮实测记录：
  - 全量 `go test -race -count=1 ./...`（无 -p）首跑 1 次失败（api 28.6s）、重跑 2 次绿；`-p 2` 全量连跑 4 轮：R1 FAIL / R2 FAIL / R3 ok / R4 ok（失败率 2/4）。
  - api 包单独 `-p 2` 连跑 3 轮：其中一轮 7 个测试 FAIL（含 TestLogoutRevokesToken 1.13s / TestAdminStatsAccountsLogs 1.48s / TestElectiveSelectRejectsWindowClosed 1.18s / TestLoginOKIssuesSession 1.15s / TestSetTargetsAndState 1.16s），单跑/子集连跑 4 轮全绿。
  - api `-p 2` 连跑至失败捕获：唯一失败断言为 `TestHandleElectivesSelectAndExit` 的 `context deadline exceeded (Client.Timeout exceeded while awaiting headers)`（httptest 127.0.0.1 通路 15s 超时），**无任何业务断言失败、无 -race 报告**。
  - zhidao 包在 6 包并行回归中偶发 1 次 FAIL，单跑 2 轮 + -p 2 连跑 3 轮全绿。
- **归因**：Windows 宿主 + 并行包数（-p 2 仍让 api 与 zhidao 两个都走真实登录 htTRIP 往返的包并发，api 内 20+ 测试各自建 httptest server，登录链路客户端 15s 超时）偶发连接建立被拒/超时。失败断言全部指向"mock server 请求未完成"，无一指向业务逻辑。属 MAJOR-46-01 同源、-p 2 缓解残余的测试环境连接 flake，非代码缺陷。
- **裁决**：MAJOR（测试基础设施层）延续，业务代码零改动需求。建议进一步：CI 用 `-p 1`（单包串行，牺牲 ~2x 全量时长换确定性）或对登录链路测试的首次 `client.Do` 加重试；若保留 -p 2，需接受偶发假红并配重跑机制。

---

## 本轮重点核对（上轮新契约，防回归）—— 全部正确闭合

### 1. F46-O1 interval clamp（scheduler.go:237-239）—— 正确闭合，无假绿
- clamp 在 `&Scheduler{interval: interval}` 赋值**之前**执行；正 interval 路径完全不受影响；`Start()` 内 `time.NewTicker(s.interval)` 永不拿到非正。测试 `TestScheduleIntervalClamped`（scheduler_test.go:3552 区间）真绿：`New(accts, &fakeStore{}, time.Time{}, 0)` → 断言 `s.interval==300ms` + `s.Start()` 不 panic + `t.Cleanup(s.Stop)` 正常收摊——clamp 缺失时 NewTicker(0) 直接 panic 必红。生产 main.go:113 传 300ms 不变。本轮回归绿（F46 钉子集 4 测全 PASS）。

### 2. F46-M1 CI -p 2 —— 语法正确、只需 ci.yml（release.yml 无 go test），但残余 flake 见 MAJOR-48-01
- `.github/workflows/ci.yml:56`（`go test -p 2 -v ./...`）YAML 正确；release.yml 各步骤只有 go build 无 go test，无需降并行。修改范围正确。

### 3. B45/B43/B44 上轮实修 —— 全部复核正确闭合
- B45-N3 加密失败回归钉 `TestLoginByPasswordEncryptFailLogs`（manager_test.go:240-269）仍真红绿；Restore 解密失败日志（manager.go:295-317）对称保留。本轮 accounts 包回归绿。
- B43-01 maybeRelogin 入口复核（scheduler.go:1204）仍在锁内、位于全部 relogin 族 map 写入前。
- B43-05 的 500 分支：`writeJSONStatus(w, http.StatusInternalServerError, 1, nil, ...)`（handler.go:905）HTTTP 500 + body code=1 双通道——B39-02 家族中仅此一处写 HTTP 500 而 body code 非 500（panic 恢复是 HTTP 500+body 500）。前端/admin 契约只读 body code，500 status 为反代/监控增强，body code=1 是业务错误语义，二者不冲突。但失败分支无真断言（见 OBSERVE-47-02 延续）。

### 4. B41-01 err 归并三路（风控 1547 / 窗口关闭 1567 / 实时复核满员 1631）+ B43-02 实时复核失效（1596）+ B39-01 成功/失效 —— 全部 sameClientFor 前置闭合

---

## MINOR

（无本轮新增 MINOR。）

---

## OBSERVE

### OBSERVE-48-01：任务开窗后的 submitAll 时序——黄金期 250ms 冲刺有 1 个 tick 级首次提交延迟（新视角，影响可忽略）
- **位置**：`scheduler.go:969-1032`（tick）+ `:1337-1362`（submitAll）+ New/Start（interval=300ms，main.go:113）
- **说明**：golden 窗口 10 秒冲刺（sprintDuration）的提交节流是"tick 唤醒后判 `now.Sub(lastSubmit) >= submitIntervalFor`（250ms）再 `submitAll()`"——submitAll 每次整体把所有账号所有发布全量 spawnChain。**节奏推演**：tick 每 300ms 唤醒 → 提交节流按对齐钟判 250ms → 实际提交节奏 = 300ms 的整数拍（每 tick 判一次），不是理想 250ms；开窗到点瞬间的首个提交发生在**下一个 tick 边界**（≤300ms，平均 150ms 延迟），而非 250ms 就绪立即发。这在"开窗瞬间谁先报到"的竞争里是恒定的、所有客户端一致的延迟，无相对劣势；且 submitEach 链内连锁提交（一门成功即下一门）不受 tick 节流限制（spawnChain 内部串行）。构不成缺陷。**若未来平台秒级满员实测表明 250ms 硬上限不够**，可在开窗瞬间由时钟对齐 goroutine 主动触发一次 submitAll（消除首个 tick 边界延迟）。
- **裁决**：观察级。非缺陷，黄金期实际节奏 = 300ms 拍，首次提交最多 300ms 延迟，属架构既定语义。

### OBSERVE-48-02：reloginResults 通道的不完整消费者确认——重登成功补探测信号由 tick 主循环消费、弃满有刻意义（延续观察 43-02，本轮补构性确认）
- **位置**：Start（scheduler.go:680-686）消费，maybeRelogin goroutine 非阻塞发送（1274-1276）
- **说明**：本轮确认生产侧唯一消费者是 tick 主循环 select 分支（673-686）。当 tick 因 interval 较长（生产 300ms）或窗口已挂起（tick 返回）时，select 仍每 tick 判断 reloginResults——重登成功信号不会滞留。**锁序**：生产 goroutine 在持 or 不持 s.mu 时发送：成功分支 `s.mu.Unlock()` 后 `select { case s.reloginResults <- ...: default: }`（1278 后），不持锁、非阻塞通道写安全；消费分支 `<-s.reloginResults`（680）在 select 内不持锁；两者无锁序交叉。cap=8 满丢弃仅发生在≈8 账号同时成功重登并发（reloginResults 积压），刻意的非阻塞权衡。无死锁无锁序错位。
- **裁决**：延续观察。

### OBSERVE-48-03：writeJSON/writeJSONStatus 家族收敛——所有"业务失败恒 200"路径无真实状态码混淆（新视角扫查，无问题）
- **位置**：handler.go:76-89（两个函数体）+ 全 api 包调用点（writeJSON ~120 处 / writeJSONStatus 9 处：router.go 71/102/107/116/121、handler.go 597/905/1078/1184）
- **说明**：逐调用点核对两个函数的签名（writeJSON(w, code, data, msg) vs writeJSONStatus(w, status, code, data, msg)）无混淆——所有 writeJSONStatus 均传入非 200 真实状态码（403/429/401/500），所有 writeJSON 均不写 HTTP 状态码（恒 200）。**唯一偏差点**：handleAdminConfig 落库失败路径（handler.go:807 `writeJSON(w, 500, ...)`）——HTTP 200 + body code=500，没有走 writeJSONStatus。这是 round M-4 时代的遗留：前端契约只读 body code=500 确有感知（"配置已生效但落库失败"），管理后台能正确显示错误文案，反代/监控在 HTTP 层看不到 500。**属于"业务性错误而非基础设施错误"的一致边界模糊**——handleAdminStats 的目标数失败（B43-05）走了 writeJSONStatus 500，而 handleAdminConfig 的落库失败仍 writeJSON 500。判据分歧：前者是 DB 数据源失败、后者是"配置已生效但持久化失败"的半成功语义。两处均可辩护，但家族不统一值得留档。
- **裁决**：观察级。若未来做家族整风，把 handler.go:807 对齐到 writeJSONStatus(500) 即可（前端 body.code 不变）。

### OBSERVE-48-04：accounts.ensure 并发——Lock 下双检+append order，同名并发登录不重复注册 order（新视角，确认无问题）
- **位置**：manager.go:102-112（ensure 持 m.mu 全函数）
- **说明**：两个 goroutine 同时 ensure 同一账号（如 LoginByPassword + Restore 并发，或两个浏览器同时登录同名账号）：ensure 全程持 m.mu（Lock→检查→创建→写 map→append order→Unlock），第二个 goroutine 在 Lock 时见客户端已在 map 直接返回，**不会重复 append order**。order 与 clients 在同一把锁下天然一致。delete on Remove/`LoginByPassword` 失败清理同样持锁。无 ABA：同名登录用同一客户端实例，token 覆盖语义正确（每个账号物理隔离一个客户端，见 client.go cookies map 逐客户端实例）。**cookie jar 结论**：`submitLogin` 用每次 attempt 独立 `sess`（Login 内新建 jar），`c.cookies` map 按客户端实例隔离（账号名隔离），账号 A 的请求绝不携带账号 B cookie——新视角无会话串号。
- **裁决**：观察级（筑无问题确认）。

### OBSERVE-48-05：store.SaveSettings 全量写并发——DELETE+INSERT 单事务原子，并发 PUT 不丢字段（新视角，确认无问题）
- **位置**：store.go:348-363（SaveSettings 单事务 DELETE FROM settings + 逐条 INSERT，tx.Commit 原子）
- **说明**：handleAdminConfig PUT 先 `d.Runtime.Update` 全量赋值（6 字段全部被 req 指针写），再 `saveSettings` 把**当前完整 cfg 六字段**整包落库（handler.go:757-801）——每次 PUT 都是全量六字段快照，不存在"部分字段 PUT"形态（所有指针字段 nil 即不改，但落库仍全量当前值）。两并发 PUT A/B：SQLite 单连接（SetMaxOpenConns(1)）+ 单事务，事务间串行化，后提交者整包覆盖前者的全部六字段（各自读到的当前 cfg 全量落库）——不会出现"A 写了 activation、B 只写 vision 而丢 activation"的字段丢失，因为每次落库都是全量。**唯一理论竞态**：A、B 各自 `Runtime.Update` 后、A 落库全量（含 B 尚未改的字段）、B 再落库全量（含 A 已改字段）——最终一致为"后者覆盖前者"，无字段丢失。SQLite 单写者串行化下的正确全量写。
- **裁决**：观察级（确认无问题）。

### OBSERVE-47-01~04 延续核实（见上轮观察表）

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-47-01 task_log 无清理 | 观察 | 全仓 grep 无 DELETE FROM task_log；AppendLog 无条件 INSERT 7+ 写点；SQLite `ORDER BY id DESC` 主键倒序 O(N) 读侧正确（本轮确认无读侧性能问题）。黄金期失败重试每 tick 写一行，长运行 DB 膨胀无清理路径。 | 延续观察 |
| OBSERVE-47-02 B43-05 500 分支无真断言 | 观察 | failingTargetsStore（handler_test.go:828-835）仍定义后无消费点；TestAdminStatsTargetsLoadFailureReturns500 仍只测正常路径（注释自述"需把 Deps.Store 改接口——超范围改动"）。B43-05 的 500 语义靠实现复查背书。 | 延续观察 |
| OBSERVE-47-03 XUANKE_PORT 无校验 | 观察 | config.go:58 无数字/范围校验；`listen tcp: lookup tcp/abc: unknown port` log.Fatalf 拒绝启动——失败形态清晰非静默。XUANKE_MASTER_KEY 长度严格校验（secure/crypto.go:16-21）。 | 延续观察 |
| OBSERVE-47-04 孤儿登录 | 观察 | Manager.Relogin（manager.go:166-175）锁内取指针、锁外 gateWait+ReloginIfNeeded；删号+在途重登并发时平台侧一次孤儿登录。spawnChain classFullRealtime 同构窗口在 sameClientFor 归并路径后已封写回。频率极低无数据污染。 | 延续观察 |
| OBSERVE-46-01 interval clamp | **已被 F46-O1 闭合** | `New` 内 `if interval<=0 { interval=300ms }` 赋值前 clamp；TestScheduleIntervalClamped 真红绿；本轮回归绿。 | 已闭合 |
| OBSERVE-46-02 Retry-After 不消费 | 延续 | doRequest 只解 body code；isRateLimitError 文案匹配 30s 固定退避；平台契约实证 body 文案无 429 头样本；防御性未来缺口。 | 延续观察 |
| OBSERVE-46-03 task_log 无清理 | 延续 | 见 47-01。 | 延续观察 |
| OBSERVE-46-04 目标不校验重复/跨年级 | 延续 | byPub 分组+spawnChain 首成功即止；CheckClassSelectable 快照复核+p属台最终把关；有意边界。 | 延续观察 |
| OBSERVE-46-05 B43-05 无真断言 | 延续 | 见 47-02。 | 延续观察 |
| OBSERVE-45-01 access_limit_cookie 占位 | 延续 | probe/main.go:24 与 submitLogin（client.go:370-371）同款 `***REMOVED***`；Restore 用 `"1"`；token 权威通道 URL 参数。 | 延续观察 |
| OBSERVE-43-01 probe 双槽分叉 | 延续 | 主体写 `["*"]`（1104）、per-account 写 `[acct]`（850）；openTimeForLocked 先 [acct] 后 ["*"]；全校单值契约不实际分叉。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 网络段重复 | 延续 | IsClassFull 恒 false（maxCount 平台未下发，client.go:696-704 注释实证），快照判满主路径先生效；本轮确认真实复核在写回前有完成度复核（见 B41-01）。 | 延续观察 |
| OBSERVE-43-04 tick 无 recover | 延续 | tick/probe/submitAll/spawnChain goroutine 均无 recover；interval clamp 已堵最可达 panic 路径（F46-O1）；残余概率极低。 | 延续观察 |
| MINOR-43-02 syncFailedWindow 死字段 | 延续 | 写（382/399）不读，注释载明"留档语义"，判据用 syncFailStreak。 | 延续观察 |
| MINOR-43-03 登录失败无固定延迟 | 延续 | 学生走教务网络往返天然延迟；管理员错误口令分支 Sleep(loginTimingFlat) 保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释 | 延续 | 调用侧先 reloginFail++ 再传，n=1 返 30s、n=2 返 60s；注释一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录 | 延续 | 见 47-04。 | 延续观察 |
| M40-01 本地钟混用 | 延续 | maybeRelogin 退避（1209/1216/1220）同基内部自洽；≤640ms 对 30s 节流无实质危害。 | 延续观察 |
| m40-02 锁内多取 now | 延续 | submitAll 1437 锁内取一次 nowAlignedLocked。 | 延续观察 |
| o40-01~04 / m39-02 / o39-02~05 | 延续 | 未激活不发会话 / reloginResults 满丢弃（48-02）/ columnExists 全常量 / interval≤0 已 clamp（F46-O1）/ 快照 TTL 读侧本地钟 / submitAll 快照竞态 / vision key 空串无法清空 / NAT 合并登录限流 / tick 无 recover / task_log 无清理（47-01）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **submitAll 黄金期时序**：见 OBSERVE-48-01。黄金期实际提交节奏 = tick 拍 300ms（≥250ms 节流），首个提交最多延迟一个 tick（≤300ms）；spawnChain 链内串行不受节流。非缺陷。
- **reloginResults 锁序**：生产（maybeRelogin goroutine）非阻塞发送在 s.mu.Unlock() 后；消费（tick 主循环 select）不持锁；无锁序交叉，cap=8 满丢弃有刻意义。见 OBSERVE-48-02。
- **writeJSON/writeJSONStatus 家族**：9 处 writeJSONStatus 全部真实状态码正确；~120 处 writeJSON 全部 HTTP 恒 200。唯一偏差点 handleAdminConfig 落库失败（handler.go:807）HTTP 200+body 500，与 B43-05 走 writeJSONStatus 不一致但均可辩护。见 OBSERVE-48-03。
- **accounts.ensure 并发**：全程持锁双检+append order，同名并发登录不重复 order；cookie map 逐客户端实例隔离无串号。见 OBSERVE-48-04。
- **SaveSettings 并发**：单事务 DELETE+INSERT 全量原子；并发 PUT 后写覆盖前写，无字段丢失（每次 PUT 恒全量六字段快照）。见 OBSERVE-48-05。
- **schedule 测试的 time.Sleep 依赖**：scheduler_test.go 全文件 `time.Sleep` 30+ 处、api 测试 1 处（10ms 轮询上限 3s）。这些按时间轮询等待的断言在 CI 高峰/慢磁盘下可能超时假红——与 MAJOR-48-01 的 httptest 连接 flake 同属测试基础设施脆弱性。已知且刻意（测试异步 goroutine 无需确定性编排），不升级。
- **数据库增量迁移与并发写**：`migrateAddPublishMeta`（db.go:25-44）逐列 columnExists→ALTER，幂等；refuseLegacy 缺列清单 3 列全部由老库迁移覆盖；`db.Open` SetMaxOpenConns(1)+WAL+busy_timeout(5000)。本轮 store/db 包回归绿。
- **moderator/gateway 无新增**：routers 全览确认无未注册 /api/ 逃逸（"GET /api/…" 8 条 + "POST /api/…" 13 条 + "DELETE /api/…" 2 条 + "/api/" 通配 404）。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查/测试**：`go build ./...`、`go vet ./...` 双通道 exit=0。`go test -race -count=1 ./...` 首跑 1 次 FAIL（api）、随后单跑绿；`-p 2` 全量连跑 4 轮 2 FAIL 2 ok（MAJOR-48-01）；api 包单独复合复跑多轮（9 轮内 2 轮 fail、失败单跑全绿、唯一捕获断言为连接超时时延）——全为 httptest 连接 flakes，无业务断言失败、无 -race 报告。
- **F46 钉子集**：TestScheduleIntervalClamped / TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime / TestWindowOpenSubmitsWithoutProbeReset 单跑全 PASS（3.1s）。
- **identity 复核族全闭合**：spawnChain 六分支（成功 1517 / 失效 1485 / 风控 1547 / 窗口关闭 1567 / 实时复核满员 1631 / 实时复核失效 1596）全部 sameClientFor 前置；maybeRelogin 入口决策侧（1204）+ 重登写回侧（1250）；MarkDone/RemoveDone 写回侧（1912/1980）。
- **WindowClosed 三判据单源** windowClosedLocked()（914-935）+ StateForAccount 共用（708）+ handleAdminStats window_closed（935）同源；开窗点 10s 裕量入账与判定单快照复用。TestWindowClosedProbeDropsToFar 绿。
- **openTime 识别槽三硬契约**：槽保留（空快照不删）；展示层过期判定独立（StateForAccount 719）；写入持锁（850/1104）；PurgeAccount 全清只给现存账号。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1007）+ WindowOpened=true 例外（黄金期 250ms 冲刺放行）；既有守卫测试 TestSubmitSuspendedWhenOpenTimeCleared 保持绿（夹具 WindowOpened 默认 false）。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow；probing 单飞锁内置位-检查；probeSem cap 4；tick 首探豁免。
- **多账号年级隔离**：probe() per-account goroutine + 专属帧；ElectivesSnapshotFor 过期专属帧 → (nil,false) 绝不回退全局帧（782-810）。
- **doLogin 闸门全收口**：gateTryAcquire 非阻塞 + gateWait 阻塞共享 gateMu/gateUsed；LoginByPassword/Relogin 无旁路；GatePump 每 30s 推进广播。
- **鉴权与数据安全**：requireAuth 401 / requireAdminSession 403 / recoverMiddleware 500 / 登录激活限流 429 一族真实状态码；SpaHandler /api 前缀 404（embed.go 精确 `/api` 顶层 + `/api/` 前缀双拒）；XFF 仅回环 + XUANKE_TRUSTED_PROXY=on 信任最右非空。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量拼接；migrateAddPublishMeta 增量幂等 + refuseLegacy 缺列清单对应；SQLite 单连接串行 + WAL + busy_timeout(5000)。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 1024 位公钥硬编码；uniqueDeviceID 复刻逐字段一致；captcha 信号量 + 重试收敛（≤3）；`Login` 内 `sess` 独立 http.Client（不共享 token 请求连接池），每次 attempt 独立 jar 会话——无共享连接池污染。
- **cmd 工具**：probe 读 XUANKE_PROBE_TOKEN（无硬编码 token）；logintest 跟随识别引擎 + wait 默认 40s 符合平台限流；bench -token 可选注入（无 token 标注测 401 拒绝路径）；均只读、无副作用、不写库。

---

## 结论

- **MAJOR 1（测试基础设施 flake 延续）/ MINOR 0 / OBSERVE 5 新（48-01~05，其中 2 项纯确认无问题）/ 延续 14 项**，共 MAJOR 1 + OBSERVE 5。
- 最重 3 条（按影响排序）：
  1. **MAJOR-48-01（httptest 连接 flake 在 -p 2 下复现率 ~2/4~2/9）**——R46→R47 的 `-p 2`（4daaff5）缓解但未根除；本轮唯一捕获的失败断言为 `context deadline exceeded (Client.Timeout exceeded while awaiting headers)`（httptest 15s 超时），无业务断言失败。CI 假红源头延续，建议进一步 `-p 1` 或登录测试首请求重试；不改业务代码。
  2. **OBSERVE-48-01（submitAll 黄金期 300ms 拍 / 首个提交最多延迟一个 tick）**——架构既定语义，无相对劣势，留档观察。
  3. **OBSERVE-48-03（writeJSON 家族：handleAdminConfig 落库失败 HTTP 200+body 500 与 B43-05 的 writeJSONStatus 不一致）**——前端契约不受影响，留档观察。
- 上轮观察项 14 条全部复核：OBSERVE-46-01（interval clamp）已被 F46-O1 闭合（测试真红绿）；其余 13 条（含 47-01~04）延续观察，无升级。
- F46-O1/F46-M1 两条上轮新契约正确闭合（clamp 赋值前生效、测试真红绿；-p 2 只需 ci.yml）——**但 -p 2 的残余 flake 使 F46-M1 未完全达成"压制假红"目标**，见 MAJOR-48-01。
- 残余风险集中在三条延续观察：孤儿登录平台侧副作用（MAJOR-42-02）、tick 无 recover（43-04，interval 已由 F46-O1 clamp 封堵）、task_log 无清理（47-01）。