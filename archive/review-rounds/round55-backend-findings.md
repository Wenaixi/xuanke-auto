# round55 后端只读审查发现报告

> 审查基线：master @ `93e07ef`（R54 收官，2026-09-20 22:31）。工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 R54 相关文件；审查期间零改动、绝对只读，唯一新文件是本报告（写入前 `git status --short` 复核工作树无新增差异）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go）。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~54 各轮报告逐条复核。
> 方法：全包逐行通读（约 1.48 万行 Go，含 6400 行测试）+ 竞态/锁序逐状态序列推演 + 本轮新视角六项逐项实证 + 上轮观察项逐条裁决。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0（本轮独立重跑确认）。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 3 轮 + api 包隔离 5 轮 + store 包窗口测试 10 轮——**真实 flake 率与 R54 的"7 轮 0 FAIL"结论有出入**，详见 MAJOR-55-01（本轮最重要的发现，是**测试基建层缺陷而非产品逻辑缺陷**）。
> 本轮额外实证：task_log 空库/新库窗口语义（SQLite 独立程序实证）、30050 行窗口查询性能、readyProbe/登录/激活竞态样本采集。

---

## CRITICAL

（无本轮新增 CRITICAL。R54 全部修复复核闭合；本轮新视角六项均确认无数据污染/崩溃/双报通道；身份防线族 6+1 测试与 WindowClosed 三判据全绿。）

---

## MAJOR

### MAJOR-55-01：R54 的"flake 归零降级关闭"结论被本轮 3/12 轮全量/隔离实证推翻——R53 残余夹具 flake 并未归零，且 R54 新增的 task_log 窗口测试引入**第二个确定性超时缺陷**

- **位置**：`backend/internal/store/store_test.go:22`（TestLoadLogsWindowKeepsRecent）+ `backend/internal/api/handler_test.go:47`（readyProbe 失败断言行）。
- **一句话问题**：R54 报告的"7 轮 0 FAIL、MAJOR-53-01 降级关闭"在本轮同环境、同命令下**未复现**：
  1. **task_log 窗口测试（R54 新增）是确定性性能超时**：`TestLoadLogsWindowKeepsRecent` 单测循环 30050 次**逐条** `AppendLog`（store_test.go:31-35），每次 INSERT 均触发一次完整 SQLite 事务 commit（WAL 同步落盘）。本机实测单测耗时 **193~249s**（10 轮），而 Go 测试默认超时 `-timeout 10m` 只给 600s——**在更慢的部署机/CI 上 30050 次同步 commit 必然击穿 10m 默认超时**。我实测用 `-timeout 100s`/`150s` 均稳定复现 `panic: test timed out`；即便用 `-timeout 600s` 全量轮（store 391s ok）也是**贴线通过**（还有 api/scheduler 等其他包排队）。
  2. **R53 夹具 flake 未归零**：api 包全量 5 轮隔离实测 **2 FAIL**（R1 TestActivateBadCode、R3 TestAdminConfigVisionKeyEncryptedAtRest+TestAdminDeleteProtectsRenamedAdmin），全部断言行 = `handler_test.go:47`（`readyProbe` mock 就绪探测失败 connectex）。失败用例单跑均绿、复跑全绿——**readyProbe 只前移了冷启动窗口、没有根治**：Windows 回环 TIME_WAIT 队列冷启动窗口可以宽于 readyProbe 的单次探测-重试（重试间隙未排空队列，与 R53 MAJOR-53-01 判断同根）。
  3. 三轮全量 `-race -p 1`：R1 api FAIL（30.4s）、R2 全绿（api 隔离复跑 38s）、R3 api FAIL（93.5s）+ store 贴线 ok（641s 超时边缘）——**真实 flake 频率 ≈ 2/3 全量轮、api 隔离 2/5**，与 R54 "0/7" 有数量级出入。失败形态与 R53 完全同根（connectex 冷启动）。
- **实证数据**：
  - `TestLoadLogsWindowKeepsRecent` 单测 10 轮耗时：193.005 / 203.712 / 208.385 / 212.855 / 221.476 / 231.228 / 241.577 / 243.212 / 247.193 / 248.963（均值 ~225s）。对照：我用独立 SQLite 程序（分 31 批事务、每批 1000 行）插入 30050 行仅 **0.48s**——**逐条 commit 是该测试 200 秒耗时根因**（每条 INSERT 一次 fsync）。
  - 30050 行窗口查询（`WHERE id > (SELECT max(id)-20000)`）实测 **~1ms**——档① SQL 性能正确，问题纯在测试数据准备方式。
  - 空表/新库窗口语义实证（独立 SQLite 程序）：空表 `SELECT count(*) FROM task_log WHERE id > (SELECT max(id)-20000)` 返回 **0**（NULL-20000 为 NULL，id>NULL 恒 false），与任务书推测一致；**LoadAllLogs 空库返回空行属"语义可接受"**（空库本就无日志可展示），非缺陷。1 行时 max(id)-20000 = -19999 → 窗口内，返回 1 行正确；2020 行全部在窗口内（-17980 起），返回 2020 行正确。
- **触发条件**：
  - 超时：任何运行 `go test ./...`（无 -timeout 覆盖）的部署/CI，store 包窗口测试 4 分钟以上；慢机/高负载机 >10m 直接 panic FAIL。
  - flake：Windows 回环冷启动（httptest mock server accept 未就绪即收到连接），与 R53 MAJOR-53-01 同根。
- **影响**：
  - 超时：**CI 假红确定性来源**（本轮全量 R3 store 641s 已到 10m 边缘）；若 CI job 无 `-timeout` 覆盖，R54 引入的该测试会稳定拖慢全量到 10m+ 并可能超时失败。R54 "最干净轮"的验证结论据此**降级**。
  - flake：CI 低频假红（本轮全量 2/3 轮、api 隔离 2/5），无生产影响。
- **修复方向（小步）**：
  - ① **窗口测试批量插入**：`TestLoadLogsWindowKeepsRecent` 改用事务批插（每 1000 行一个事务，仿独立程序实证），30050 行 0.5s 内完成——单测耗时从 ~225s 降到 <1s，超时缺陷从根因消除。**这是本轮最高优先修复**。
  - ② **窗口测试加显式超时/跳过**：`t.Short()` 跳过 30050 行重载，或至少 `-timeout` 显式声明（`//go:build` 无需，直接 `d.Exec` 批事务即可）。
  - ③ **api 夹具 readyProbe 升级**：失败时**轮询重试**（如 200ms 间隔 × 5 次，非单次重试）把冷启动窗口彻底前移；或对 mock 采用 `httptest.NewUnstartedServer + Start` 显式就绪。
  - TDD 形态：改完窗口测试后 `go test -count=1 -run TestLoadLogsWindowKeepsRecent ./internal/store/` 应在 2s 内绿；api 包 5 轮隔离 0 FAIL。

### MAJOR-55-02（潜在）：`TestLoadLogsWindowKeepsRecent` 超时在 CI 无 `-timeout` 覆盖时是**确定性失败**，需主控裁决是否升级

- 与 MAJOR-55-01 ① 同根，独立列号便于追踪。若主控接受"窗口测试就是慢"并统一给全量加 `-timeout 900s`，则降级为 MINOR（测试基建优化）；若 CI 走默认超时，这是**必红项**。建议按 MAJOR-55-01 ① 修批插即可一并闭合。

---

## 本轮重点三项核实

### 1. R54 task_log 档①窗口前置复核 —— SQL 语义正确、性能正确，测试基建有缺陷（MAJOR-55-01）

- **SQL 语义（空表/新库/删除账号后）**：逐项实证（独立 SQLite 程序，modernc.org/sqlite v1.59.0）：
  - 空表：`max(id)` 为 NULL → `max(id)-20000` 为 NULL → `id > NULL` 恒 false → **返回 0 行**。LoadAllLogs 空库返回空（无日志可展示），语义可接受，非缺陷。
  - 1 行：max(id)-20000 = -19999 → id=1 在窗口内 → 返回 1 行。正确。
  - 2020 行：max(id)-20000 = -17980 → 全部在窗口内 → 返回 2020 行。正确。
  - 删除账号：`DeleteAccount` 只删该账号日志，max(id) 随剩余日志正确收缩，窗口边界自洽（无账号残留日志时退化为空表语义）。
  - **结论：空库返回空行是"应有行为"而非缺陷**——无需 COALESCE/IS NULL 补丁（补了反而在空库时把窗口展开成全表，多此一举）。任务书推测的"空库误返空行"经实证归因：**空库本就没有可展示日志，返回空与"应有行"不冲突**。
- **性能**：30050 行窗口查询实测 ~1ms；`max(id)` 走自增主键 O(1)。正确。
- **测试覆盖缺口**：`TestLoadLogsWindowKeepsRecent` 只测"30050 行有数据"场景，**未覆盖空表/新库**——但经实证空表行为正确，测试覆盖缺口无实质风险。可补一条空表断言（LoadAllLogs 返回 0 行）作为契约固化，可选。

### 2. R54 前端五修后端契约复核 —— 无新契约依赖，全部自洽

- **M-A 横幅兜底 / M-B duration / M-C viewport / M-D 右栏兜底 / M-E open_time_set**：全部为前端侧改动。后端侧对应契约逐一复核：
  - `handleElectives` 直传 `data`（`ElectivesData{BeginTimes, Publishes}`，client.go:580-583），`begin_times` 恒下发（平台未下发时为 nil → JSON `null`，数组空时为 `[]`）。前端 `data?.begin_times?.[0] ?? null` 安全短路成立。
  - `/state` open_time 联动：`StateForAccount` 依识别槽有效时刻判定 `open_time_known`（识别过期 → false），识别槽保留不删（关闭≠时间消失）。Select 主倒计时 `openTimeStr ?? begin_times[0]` 两通道同源（都来自平台 beginTimes 探测），无分叉。
  - `handleAdminStats` 恒下发 `open_time_set`/`token_valid`/`captcha_engine`/`captcha_concurrency` 四键（handler.go:935-952 固定 map 键）——M-E 的"可选字段兜底"是纯前端防御，后端固定下发不破坏。
  - 后端无任何 R54 改动（R54 后端仅 commit ee0eafc/2831b6c 两处：store.go 窗口 SQL + client.go priorityId 注释），与前端五修零接口面交集。
- **结论：无新契约依赖，无缺陷**。

### 3. R55 新视角六项逐项实证 —— 见下（A 空库语义 / B credentials 全表 / C session sweeper / D runtime 热改 / E 锁内 SQLite 写 / F refuseLegacy 空库）

---

## MINOR

（本轮无新增 MINOR。R54 无遗留 MINOR；R53 两项已由 commit 22a61f5 实修闭合。）

---

## OBSERVE

### OBSERVE-55-01：`LoadAllLogs`/`LoadLogs` 空表语义已有实证支撑，建议补一条空表契约断言（可选）

- **位置**：`backend/internal/store/store_test.go`（TestLoadLogsWindowKeepsRecent 同文件）。
- **一句话问题**：空表/新库下窗口 SQL 返回 0 行已实证正确，但测试未覆盖——未来若有人改 SQL 把窗口改成 `COALESCE(max(id),0)` 或删掉窗口，空表行为会悄悄退化（空库返回全表）而测试不红。
- **修复方向（一行）**：`TestLoadLogsWindowKeepsRecent` 开头加 `all, _ := s.LoadAllLogs(10); if len(all) != 0 { t.Fatal(...) }`。
- **TDD**：同测试文件，绿即可。

### OBSERVE-55-02：credentials 表全表加载（OBSERVE-54-02 延续）——维持不缓存，无新变化

- `accountExists`/`handleSetTargets` 每请求 `LoadCredentials` 全表（handler.go:452/1062）。数据量 <100 账号、毫秒级；无写并发竞态（登录写凭据与读全表都被 SQLite 单写者 + RWMutex 保护，SetMaxOpenConns=1 串行化）。**延续观察，不修复**。

### OBSERVE-55-03：session.Store sweeperLoop ttl<=0 测试场景 —— 全量核验无缺陷

- `New(ttl<=0)` 不建 sweeperStop/sweeperDone 通道、不启协程；`Close()` 由 `sweeperClose sync.Once` 幂等、`sweeperStop==nil` 直接 return（不 close nil channel）。TestSweeperLoopStopsOnClose（New(time.Hour)+Close×2）与 TestExpiry（New(50ms)）全绿。**无缺陷**。

### OBSERVE-55-04：runtime.Store 热改并发 —— Update 闭包读-改-写 + 落库无半态

- `Update` 写锁内原子改 Config（纯值字段），`Get` 读锁返回值拷贝；handler 热改 `rt.Update` → `cfg := rt.Get()` → `saveSettings` 落库 → `dispatchRuntimeConfig` 下发。管理员双改并发时：Update 串行、Get 拿到更新后的快照、落库全量替换（settings 表 DELETE+INSERT 同事务）——**最坏情况为"两次落库各自全量"，无半态**。落库失败路径已 M-4 处理（内存生效 + 明确 500 + 下游仍下发）。**无缺陷**。

### OBSERVE-55-05：scheduler spawnChain 锁内 SQLite 写（F13-m2 收敛后）——最坏持锁时长再估算

- 逐路径核（scheduler.go）：成功分支锁段 = `sameClientFor` + `done` 写 + `setStateLocked` + `AppendLog` + `SaveSuccess`（store.go，SQLite 单写者串行，WAL 模式单 INSERT 约 0.1-1ms）——**持锁窗口 <5ms**（与 R53 新视角 B 一致）。实时复核网络段（`classFullRealtime` 最长 15s）已在 C-4 释放锁。锁内无网络/无第三方调用。**无缺陷**。

### OBSERVE-55-06：db.go refuseLegacy 空表场景 —— 全新库/空库启动正确

- 全新库：`schemaSQL` 建全表 → `migrateAddPublishMeta` 幂等跳过（列已存在）→ `refuseLegacy` 逐项检查：account 表无、targets 空账号行无、priority/allow_swap/account 列在、settings 表在 → 放行。空库/新库启动不被拒绝（TestOpenAndSchema 全绿）。迁移幂等（重跑全跳过）。**无缺陷**。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| **MAJOR-53-01（flake 残余）** | 降级关闭 | **api 隔离 5 轮 2 FAIL、全量 3 轮 2 FAIL（R1/R3），断言行 handler_test.go:47 connectex**——readyProbe 未根治冷启动窗口，R54 归零结论未复现 | **重新打开为 MAJOR-55-01** |
| **OBSERVE-54-01（priorityId 空串）** | 观察 | commit 2831b6c 已把注释对齐（client.go:350-353 载明 jQuery 丢弃 vs Go 空串语义） | **闭合**（注释已落地） |
| **OBSERVE-54-02（accountExists 全表）** | 观察 | 维持毫秒级、无竞态 | 延续观察 |
| **OBSERVE-54-03（task_log 档①）** | 观察→落地 | **commit ee0eafc 已落地档①**（store.go:231/422 窗口 SQL）+ 空表语义实证正确；但测试基建超时缺陷（MAJOR-55-01） | 闭合（功能）/ 新开测试缺陷 |
| OBSERVE-53-02（tick 无 recover + 优雅退出） | 观察 | signal.Notify 仍 0 处；SQLite WAL crash-safe 兜底 | 延续观察 |
| OBSERVE-53-03（孤儿登录） | 观察 | B43-01 决策侧 + B21-03 写回侧双闭合，残余窗口极低 | 延续观察 |
| OBSERVE-53-04（双槽分叉） | 观察 | [acct]→["*"] 优先级，全校单值不实际分叉 | 延续观察 |
| OBSERVE-53-05（reloginResults cap 8） | 观察 | 非阻塞丢弃，下个 tick 探测兜底 | 延续观察 |
| OBSERVE-53-06（failingTargetsStore 无消费点） | 观察 | 仍无消费点 | 延续观察 |
| OBSERVE-53-07（429 头不消费） | 观察 | 全仓 Retry-After 0 处 | 延续观察 |
| OBSERVE-53-08（XUANKE_PORT 无校验） | 观察 | `:abc` log.Fatalf 拒绝启动 | 延续观察 |
| OBSERVE-53-11（access_limit_cookie 占位） | 观察 | 三处占位值不统一但不影响鉴权 | 延续观察 |
| OBSERVE-53-12 / M40（本地钟混用） | 观察 | 残余均与开窗判点无关 | 延续观察 |
| OBSERVE-52 及更早全部观察项 | 观察 | 无变化 | 全部延续 |

---

## 本轮新视角六项实证结论

### A. task_log 空库/新库窗口语义 —— 实证正确，无需修复
- 空表 `max(id)` 为 NULL → `NULL - 20000` 为 NULL → `id > NULL` 恒 false → **返回 0 行**（独立 SQLite 程序实证：`空表 count=0`、`1 行 count=1`、`2020 行 count=2020`）。空库返回空 = 应有行为（无日志可展示），**非缺陷**；COALESCE/IS NULL 补丁反而把空库窗口展开成全表，多此一举。建议（可选）补一条空表契约断言（OBSERVE-55-01）。
- **任务书问题的直接回答：空库 LoadAllLogs 返回空行，与"应有行"不冲突——空库本无日志可返回。**

### B. credentials 表 accountExists 全表加载 —— 维持不缓存
- 每请求 `LoadCredentials` 全表（<100 账号，毫秒级）；SQLite SetMaxOpenConns=1 串行化 + RWMutex，读（accountExists）与写（登录落凭据）无竞态。**无缓存/索引必要性**。延续观察（OBSERVE-55-02）。

### C. session.Store sweeperLoop 在 ttl<=0 测试场景 —— 无缺陷
- `New(0)` 不建通道不启协程；`Close()` 幂等（sync.Once + nil 通道守卫）。全量核验无泄漏/无 panic。

### D. runtime.Store 热改并发 —— 无缺陷
- Update 写锁原子改 + Get 快照拷贝 + 落库全量替换（同事务）——无半态；管理员双改并发最坏为两次全量落库，最终一致。

### E. scheduler spawnChain 锁内 SQLite 写 —— 成立，最坏持锁 <5ms
- 成功/失效/风控/满员各分支锁段仅 map + `setStateLocked` + `AppendLog`/`SaveSuccess`（SQLite 单写者，WAL 单 INSERT ~0.1-1ms）；实时复核网络段已释放锁。**F13-m2 契约继续成立，无需独立 DB goroutine**。

### F. db.go refuseLegacy 空表场景 —— 无缺陷
- 全新库建表 → 迁移幂等 → refuseLegacy 全项放行；空库启动正确。迁移/WAL 语义无新问题。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员两分路）+ maybeRelogin 决策侧（B43-01）/写回侧（B21-03）+ waitChainExit 等待契约。测试 6+1 个（TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+RealtimeUnauthorized）scheduler 包 15s 全绿。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败 + 幽灵窗口三条，StateForAccount/WindowClosed/admin stats 共用。钉子测试（TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler / TestSubmitSuspendedWhenOpenTimeCleared）全绿。
- **鉴权与数据安全**：管理员双条件（B43-04）+ 撞名学生响应无 adminName；writeJSONStatus 家族（panic 500/会话 401/管理 403/限流 429/CSRF 403）；SpaHandler /api 双处 404；XFF 仅回环+开关；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 32 字节双重校验；登录/激活独立限流桶。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；refuseLegacy 缺列清单对应剔除；窗口 SQL 语义实证正确（空表/1 行/2020 行）。
- **登录链路**：RSA-PKCS1v1.5 / empty priorityId（注释已对齐）/ captcha Limiter Cond 动态热收敛 / 重试收敛 ≤3 / 验证码一次性。
- **连接池与时钟**：sharedTransport MaxIdleConnsPerHost=64 + 120s 空闲；SyncServerTime 中点近似 + 成功才推进 lastSyncTime + 失败 30s 退避 + streak≥3 复位。
- **新视角 A/C/D/E/F 全部无缺陷**（见上）。

---

## 结论

- **MAJOR 2（55-01 测试基建超时 + 夹具 flake 复活、55-02 同根 CI 必红裁决项）/ OBSERVE 6 新增 + 延续 13 项**。R54 后端两修（store.go 档① SQL + client.go priorityId 注释）语义复核全部正确；R54 "最干净轮"的验证结论（7 轮 0 FAIL）被本轮 3/12 轮实证推翻——**非产品逻辑回归，是测试基建缺陷**。
- **flake 频率结论**：全量 `-race -p 1` 3 轮 2 FAIL（R1/R3，api connectex）；api 隔离 5 轮 2 FAIL（R1 TestActivateBadCode、R3 TestAdminConfigVisionKeyEncryptedAtRest + TestAdminDeleteProtectsRenamedAdmin，全部 handler_test.go:47）；**scheduler/zhidao/session/db/secure/runtime/config 全部 0 FAIL**。readyProbe 失败用例单跑复跑均绿，冷启动窗口未根治。
- **task_log 空库语义结论**：空库 LoadAllLogs 返回空行 = 应有行为（`max(id)` 为 NULL → `id>NULL` 恒 false），非缺陷；无需 COALESCE 补丁。
- **最致命 3 条（按影响排序）**：
  1. **TestLoadLogsWindowKeepsRecent 确定性性能超时（~225s 单测、CI 默认超时必红）**——R54 引入，批插修复一行即可归零。
  2. **api 夹具 readyProbe 未根治冷启动 flake（全量 2/3 轮、隔离 2/5 轮）**——R53 收尾不完整，建议 readyProbe 改轮询重试。
  3. **R54 "flake 归零"验证结论需降级**——非代码缺陷但影响决策置信度，后续轮以"非 api 包 0 FAIL + api 包隔离多轮统计"口径报告。
- **建议优先修复方向**：① 窗口测试事务批插（store_test.go:31 循环改每千行一批事务，单测 225s→<1s）；② api 夹具 readyProbe 升级为轮询重试（handler_test.go:182-204）；③ 窗口测试补空表契约断言（OBSERVE-55-01）。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| 全量 race 3 轮 | R1 api FAIL 30.4s（TestActivateBadCode）/ R2 全绿 / R3 api FAIL 93.5s（TestAdminConfigVisionKeyEncryptedAtRest+TestAdminDeleteProtectsRenamedAdmin）+ store 贴线 641s |
| api 隔离 race 5 轮 | R1 FAIL（TestActivateBadCode）/ R2-R5 全绿（38-70s/轮，另完整日志轮 94s FAIL 一次） |
| store 窗口测试单测 10 轮 | 193.005~248.963s（均值 ~225s），`-timeout 100s/150s` 稳定超时 panic |
| store 其余全部测试 | 无窗口测试时 1.5~3.7s 全绿 |
| SQLite 空表窗口语义 | 空表 0 行 / 1 行 1 行 / 2020 行 2020 行（独立程序实证） |
| 30050 行批插 + 窗口查询 | 批插 0.48s / 窗口查询 ~1ms（独立程序实证） |
| scheduler/zhidao/session/db/secure/runtime/config race | 全部 0 FAIL |
