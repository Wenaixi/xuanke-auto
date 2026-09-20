# round56 后端只读审查发现报告

> 审查基线：master @ `363d8e4`（R55 收官，2026-09-21）。工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 R55 相关文件；审查期间绝对只读（唯一允许新建的产物即本报告），写入前 `git status --short` 复核：工作树除 5 个社区文档 + 前端对等审查报告的 `round56-frontend-findings.md` 外零差异（见文末实证表）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部 208 个测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~55 各轮报告逐条复核 + R55 文末观察项延续。
> 方法：全包逐行通读 + 竞态/锁序推演 + 独立 Go 程序实证（httpDo 重试 body 双发）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` **12 轮** + api 包隔离 10 轮 + zhidao 隔离复跑。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0（本轮独立重跑确认）。

---

## 结论先行

- **CRITICAL 0 / MAJOR 1 / MINOR 1 / OBSERVE 6 新增 + 延续 9 项**。
- **产品逻辑零缺陷**：scheduler 提交链、身份防线族（sameClientFor 六分支 + maybeRelogin 决策/写回双闭合）、WindowClosed 三判据单源、task_log 窗口 SQL、api 鉴权族（401/403/404/429/500 家族）、登录链路（RSA/priorityId/Vision 热收敛）、凭据 AES-256-GCM、数据库迁移——全包逐行通读 + 12 轮 race 实证，全部正确。
- **MAJOR-56-01（测试基建，非产品）**：R55 的"api 隔离 5 轮 0 FAIL"统计口径**不可推广到全量轮**——本轮全量 12 轮 3 FAIL（api×2、**zhidao×1**），而 api 包隔离 10 轮 **0 FAIL**。R55 声称"非 api 包 0 FAIL"被本轮 R1 的 zhidao FAIL 实证**违反**。readyProbe 轮询修复只覆盖 api 包，zhidao 包自身 Login 链路的冷启动 connectex 仍裸露（socketPreheat + fetchLoginPage 单次重试在**全量轮**前序包 TIME_WAIT 残留下不够）。
- **最致命 3 条（按影响排序）**：
  1. **MAJOR-56-01：全量轮 flake 未归零且新增 zhidao 包 FAIL 样本**——R55"非 api 包 0 FAIL"结论本轮不成立；全量 3/12 轮 FAIL（R1 zhidao TestLoginStopsAfterCaptchaExhausted 断言行 captchas==0 = Login 初始化 connectex 冷启动；R3/R11 api FAIL），api 隔离 10/10 全绿——**全量 vs 隔离口径分裂是本轮最重要的实证发现**（R55 教训 1 的延续深化：单包隔离归零 ≠ 全量轮归零）。
  2. **MINOR-56-01：admin stats `open_time_set` 语义与注释/学生端 `open_time_known` 分叉**——`RecognizedOpenTime()` 返回识别槽过期值（非零），`!open.IsZero()` 恒 true；窗口关闭后管理员后台显示过期日期 + open_time_set=true，与学生端"识别过期 → open_time_known=false"语义不一致，且 handler.go:884 注释"识别过期 → 零值"与实测行为矛盾。
  3. **OBSERVE-56-01（httpDo 重试 body 双发实证）**：独立程序确认——首个 RoundTrip 已把请求体完整读出（服务端已消费），read 类连接错误重试时**第二个请求 body 为空**。对 SelectClass/ExitClass 这意味着重试是"空 body 畸形 POST"，平台必然拒绝（安全方向），但重试请求本身无效且会追加一条误导性失败日志。

---

## MAJOR

### MAJOR-56-01：R55"非 api 包 0 FAIL + api 隔离归零"结论本轮被 3/12 全量轮实证推翻——zhidao 包 Login 冷启动 flake 仍裸露，且全量轮与包隔离轮 flake 率系统性分裂

- **位置**：`backend/internal/zhidao/client_test.go:87-101`（TestLoginStopsAfterCaptchaExhausted，断言行 95 `识别最多 3 次，实际 0`）+ `backend/internal/zhidao/client.go:286-317`（fetchLoginPage 单次重试）+ `backend/internal/api/handler_test.go:183-211`（readyProbe 轮询——只覆盖 api 包）。
- **一句话问题**：R55 修复只覆盖 api 包夹具，zhidao 包自身登录链路的冷启动 connectex 未被任何夹具覆盖（socketPreheat 只预占一个端口、fetchLoginPage 只重试一次）；且**全量轮 flake 率系统性高于包隔离轮**。
- **证据链**：
  - 全量 `-race -count=1 -p 1 -timeout 900s ./...` **12 轮**：R1 **zhidao FAIL**（TestLoginStopsAfterCaptchaExhausted，断言行"识别最多 3 次，实际 0"——Login 首次 fetchLoginPage 连击冷启动 connectex，3 次识别 0 次触达 mock）/ R3 **api FAIL** 45.4s / R11 **api FAIL** 48.6s；其余 9 轮全绿。
  - api 包隔离 **10 轮 0 FAIL**（含 R55 遗留口径 5 轮 25-54s/轮全绿 + 额外 5 轮全绿）。
  - zhidao 包单用例复跑绿（TestLoginStopsAfterCaptchaExhausted 0.07s PASS）、整包隔离 2.2-5.1s 绿。
  - **全量 vs 隔离分裂的根因假设**：`-p 1` 下包按序运行，api 紧随 accounts（accounts 的 httptest 服务器刚关闭，回环 TIME_WAIT 残留最多）；store 包（40-85s）在 api 之后，其 SQLite 长跑不产生端口残留但拉长整体时间窗口。api 隔离时无前序包残留 → 0 FAIL；全量时前序包残留叠加 → 2/12 FAIL。zhidao 在 store 之后运行，同样吃前序残留 → 1/12 FAIL。
- **触发条件**：Windows 宿主全量 `go test ./...`（无 -p 1 包隔离）；CI 的 `||` 重跑可以吸收，但每次重跑是全量重来、成本高。
- **影响**：CI 低频假红（全量 ~25% 轮次 FAIL 任一包）；无生产影响。R55 教训 1（"单轮次归零不可信"）需要升级为"**包隔离归零不可信**"——必须连续多轮**全量**统计。
- **修复方向（小步）**：
  - ① zhidao 包夹具对齐 api 的 readyProbe：loginMockServer/各 httptest 服务器构造后主动发一条探测请求把冷启动窗口前移（与 api newTestDeps 同款轮询）。
  - ② 或全量命令固定 `-p 1` 已是如此——根因在每包首个 httptest server 的 accept 就绪前首请求 connectex；统一提取一个 `pkg` 级 `TestMain` 一次性 preheat（预创建-关闭一个回环套接字）即可。
  - ③ 统计口径：后续轮以"**全量 `-p 1` 连续 N 轮**"为唯一 flake 判定源，包隔离只作定位手段不作归零证据。
  - TDD 形态：zhidao 包加夹具探测后，全量连续 5 轮 0 FAIL。

---

## MINOR

### MINOR-56-01：admin stats `open_time_set` 对"识别过期"语义与学生端 `open_time_known` 分叉，且注释与行为矛盾

- **位置**：`backend/internal/api/handler.go:886-889, 952` + `backend/internal/api/handler_test.go:956-998` + `backend/internal/scheduler/scheduler.go:445-450`。
- **一句话问题**：`handleAdminStats` 用 `!d.Sched.RecognizedOpenTime().IsZero()` 判定 open_time_set，而 `RecognizedOpenTime()` 对"识别槽有值但已过期"返回**过期值本身（非零）**——窗口关闭后（识别值落入过去）admin stats 显示 `open_time_set=true` + 一串过去日期；学生端 `/state` 的 `open_time_known` 用 `OpenTime.After(nowAligned)` 正确判定过期为 false。两处对同一"识别已过期"状态给出相反布尔。
- **证据链**：handler.go:884 注释写"未识别 / 识别过期 → 零值"，但 scheduler.go:445-450 明确"识别值已过期由展示方依 need 判定"且 `openTimeForLocked` 刻意不截断过期值（决策锚 1 契约）——**注释描述的行为不存在**；handler_test.go:956 注释同样写"识别过期 → 空串 + open_time_set=false"，但同文件 997 行断言 `open_time_set == !recog.IsZero()`（mock beginTimes=1789261200000 已过期时 recog 非零 → 断言 open_time_set=true），**测试注释与断言自相矛盾**。
- **触发条件**：窗口批次结束（识别值已过去）后管理员打开 stats 页。
- **影响**：管理后台"开放时间"行显示过期日期且 open_time_set=true，与窗口已关闭三态（window_closed=true）并列时易误导（管理员可能认为"开放时间识别仍然有效"）；纯展示层，无功能/数据影响。
- **修复方向（二选一）**：
  - a) 语义对齐：`handleAdminStats` 对 open_time_set 也做过期判定（`!open.IsZero() && open.After(time.Now())`），把 884/956 注释改正——与学生端 open_time_known 同源；
  - b) 若保留"展示识别事实"语义，则改注释（884 行"识别过期 → 零值"改为"识别槽无值 → 零值；过期值照常展示"）并让测试注释自洽。
  - TDD：改后 TestAdminStatsOpenTimeFromRecognized 红→绿（过期场景断言 open_time_set 翻转）。

---

## OBSERVE

### OBSERVE-56-01：httpDo 连接层重试在"read 类错误"下第二个请求 body 为空（独立程序实证）——重试请求畸形但方向安全

- **位置**：`backend/internal/zhidao/client.go:465-491`（httpDo + cloneReq）。
- **实证**：独立 Go 程序（fake RoundTripper：首次完整读出 body 后返回 `read tcp ... connection reset`，重试打印收到的 body）——**RoundTrip#1 body="classId=61115"，RoundTrip#2 body=""**。即 read 类连接错误发生在服务端已完整消费请求体之后，`req.Clone` 重发时 body 已被首个 RoundTrip 消费（Clone 浅拷贝 Body，`GetBody` 未设置）→ 第二个请求体为空。
- **影响**：对 SelectClass/ExitClass，read 错误（请求已到达平台、平台可能已处理）后的重试是空 body POST——平台要么拒绝（"无效请求"）要么按缺参处理，**不会构成"平台已成功处理 + 重试再报一次"的双报成功**（安全方向）。但重试请求本身无效、会追加一条误导性"报名失败"日志，且平台可能已真实处理首次请求（成功报名后客户端收到 read 错误重试空 body 失败——用户看到失败但平台已报上，状态与 UI 分叉的极小窗口）。
- **结论**：F52-M4 的"重试即等效换新连接"对 read 错误不成立（重试的是空 body）；dial/write 错误不受影响（请求未到达）。延续观察，不修复——但若未来要收口，可为 cloneReq 补充 `req.GetBody` 重生成（`bytes.NewReader` 还原 body），一行。

### OBSERVE-56-02：`open_time_set` 注释链整体过时（handler.go:884 / handler_test.go:956）——"识别过期→零值"是不存在的语义

- 与 MINOR-56-01 同根：两处注释描述的行为与实现（scheduler.go:445-450 返回过期槽值）矛盾。即使按 b 方案保留展示语义，注释也必须改——当前注释会误导后续维护者按"零值"预期修改 RecognizedOpenTime 从而凿穿决策锚 1（识别值不截断零值）。

### OBSERVE-56-03：server WriteTimeout(30s) < Login 最坏耗时(Vision 60s×3 尝试) —— 慢 Vision 下登录响应可被服务端截断

- **位置**：`backend/main.go:166-173`（WriteTimeout 30s）+ `backend/internal/zhidao/captcha.go:159`（Vision 客户端 60s 超时）+ `backend/internal/zhidao/client.go:212-273`（Login 最多 3 次识别尝试）。
- 最坏单次 Login = 3 × (fetchLoginPage ≤15s + captcha ≤15s + Vision ≤60s) ≈ 数分钟，远超 30s WriteTimeout；超时后服务端截断响应、客户端收到连接重置，但 handler 已执行 issueSession（会话已签发）——**平台已登录成功而客户端显示失败**，重试又消耗登录闸门预算。典型识别 1-5s 时不触发，仅慢 Vision 服务/识别排队（并发限流 1 时多个登录排队）可触及。延续观察。

### OBSERVE-56-04：readyProbe 探测请求无显式超时——mock accept 异常挂起时夹具构造可阻塞至 OS TCP 超时

- **位置**：`backend/internal/api/handler_test.go:183-211`（`http.DefaultClient.Do` 无 timeout，DefaultClient 超时为 0=无限）。
- 正常情况下 connectex 立即失败不耗时；但若 httptest server 的 listener 接受连接后处理挂起（极端），探测请求会挂到 OS 层 TCP 超时（分钟级）才失败。建议给探测请求设 `http.Client{Timeout: 2*time.Second}`（一行）。频率低，观察。

### OBSERVE-56-05：`TestLoginLimiterGC` 造 1500 桶触发惰性清理的判定路径依赖 `lastGC` 零值（非显式构造）

- 测试把 `l.lastGC = time.Time{}` 后 `l.allow` 触发清理——语义上依赖"now.Sub(零值) > 1min 恒成立"，是隐式依赖而非显式构造冷启动（与 R54 窗口测试同类"测试数据准备方式"味道但轻得多）。观察，无修复必要。

### OBSERVE-56-06：api 包 `initCaptchaAtStartup` 经 `sync.Once` 全局信号量在 zhidao 包 `init()` 设的并发 2 之外叠加一次 SetLimit(1)

- api 包测试二进制首次 Register 调 `NewCaptchaSemaphore(cfg.CaptchaConcurrency)`（router.go:28）把全局识别并发限为 1；zhidao 包二进制 init() 设 2。两包互不共享进程（各自测试二进制），无实际干扰；但若未来单二进制集成测试（同进程跑两包测试）会出现静默覆盖。观察。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-55-01（flake 残余） | 修复（批插+轮询） | **批插闭合**（无 race 0.57s、race 43-85s，10m 超时无忧）；**轮询只修了 api 包**——全量 12 轮 3 FAIL（api×2、**zhidao×1**），api 隔离 10 轮 0 FAIL | **部分闭合**：批插闭合；flake 升级为 MAJOR-56-01（新增 zhidao 样本 + 全量/隔离分裂） |
| MAJOR-55-02（CI 默认超时） | 随 55-01 闭合 | 批插后 store 全量 40-85s，10m 默认超时安全 | **闭合** |
| OBSERVE-55-01（空表契约断言） | 修复（window_empty_test.go） | TestLoadLogsEmptyDBWindowSemantics 独立文件、0.62s、空库 LoadAllLogs/LoadLogs 双断言，**不与 TestLoadLogsWindowKeepsRecent（30050 行）/TestLogs/TestLogsByAccount（小数据集）重复**，钉死"空库返回空不报错" | **闭合** |
| OBSERVE-55-02（accountExists 全表） | 延续观察 | handler.go:452/1062 每请求 LoadCredentials 全表，<100 账号毫秒级，SQLite 单连接+RWMutex 无竞态 | 延续观察 |
| OBSERVE-55-03（sweeper ttl<=0） | 延续观察 | session.New(0) 不启协程、Close 幂等、TestSweeperLoopStopsOnClose 全绿 | 延续观察 |
| OBSERVE-55-04（runtime 热改） | 延续观察 | Update/Get 快照拷贝 + 落库同事务全量替换，无半态 | 延续观察 |
| OBSERVE-55-05（锁内 SQLite 写） | 延续观察 | spawnChain 各分支锁段 map+setState+AppendLog/SaveSuccess，单 INSERT <5ms | 延续观察 |
| OBSERVE-55-06（refuseLegacy 空库） | 延续观察 | 全新库建表→迁移幂等→refuseLegacy 全放行；TestOpenAndSchema 绿 | 延续观察 |
| OBSERVE-53-02（tick 无 recover） | 延续观察 | signal.Notify 仍 0 处；WAL crash-safe 兜底 | 延续观察 |
| OBSERVE-53-03~08 / 53-11 / 53-12 / M40 | 全部延续观察 | 无变化 | 全部延续 |

---

## 本轮新视角六项逐项实证

### A. R55 readyProbe 轮询重试（200ms×5）是否引入新问题 —— 不掩盖真实故障，但覆盖范围仅 api 包
- 是否掩盖真实 server 故障：**否**。轮询有界（≤6 次、~1s），mock server 真挂了全部失败 → `t.Fatalf` 红而非假绿。假绿只可能发生在"探测请求成功但后续测试请求失败"的窗口——探测成功说明 accept 已就绪，后续请求复用同一 server，此窗口不存在。
- 轮询期间并发影响：每次失败仅 `time.Sleep(200ms)`，处于本测试 goroutine；各测试独立 mock server，无共享端口/状态。无影响。
- 局限：**只覆盖 api 包夹具**。zhidao 包 Login 链路无同款探测（socketPreheat 单端口预占 + fetchLoginPage 单次重试），全量轮 R1 实证 zhidao 仍 connectex（→ MAJOR-56-01）。

### B. 窗口测试事务批插是否形成 AppendLog 覆盖缺口 —— 否
- `TestLoadLogsWindowKeepsRecent` 用 `tx.Exec` 直插绕过 AppendLog（store_test.go:34-55），但 **AppendLog 公共路径有独立测试覆盖**：TestLogs（store_test.go:401-420）与 TestLogsByAccount（262-284）均经 `s.AppendLog` 写入再读回断言，TestAdminStore（460-469）亦覆盖。批插只测"窗口裁剪最近 20000 条"语义，不重复测 AppendLog 本体。**无覆盖缺口**。

### C. window_empty_test.go 空库契约测试 —— 不重复、真钉死
- 与既有测试无重叠：TestLoadLogsWindowKeepsRecent 测 30050 行窗口语义；TestLogs/TestLogsByAccount 测小数据集账号隔离；本测试专门钉"空库 max(id)=NULL → id>NULL 恒 false → 返回空"行为（空库 LoadAllLogs 与 LoadLogs 双断言），且防未来把窗口 SQL 改成 `COALESCE(max(id),0)` 后空库退化返回全表而不红。语义正确（`-race` 0.62s 绿，与 R55 独立 SQLite 程序实证一致）。

### D. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 12 轮 + api 隔离 10 轮 —— 真实 flake 率统计
- 全量 12 轮：**3 FAIL（25%）**——R1 zhidao（TestLoginStopsAfterCaptchaExhausted）/ R3 api / R11 api；9 轮全绿。
- api 隔离 10 轮：**0 FAIL**（含 5 轮 25-54s 口径轮 + 5 轮补充）。
- zhidao 隔离：单用例复跑绿、整包 2.2-5.1s 绿。
- **口径结论：全量轮 flake 率（~25%）远高于 api 隔离轮（0%）**——"api 隔离多轮归零"不能作为全量归零证据（R55 教训 1 升级）。非 api 包本轮有 zhidao FAIL，**R55 文末"非 api 包 0 FAIL"声明本轮不成立**。

### E. 上轮观察项延续复核 —— 全表见延续表，9 项延续、2 项闭合、1 项部分闭合
- task_log 窗口 SQL / append 覆盖 / 空表语义 / scheduler 身份防线 / WindowClosed 三判据 / api 鉴权家族 / 登录链路 / 凭据加密 / 数据库迁移——全部复核正确，无新缺陷。

### F. 全包逐行通读找新问题 —— 见 MAJOR/MINOR/OBSERVE 列表
- 重点核对：scheduler 提交链（submitAll→spawnChain 六分支身份复核全闭合、inflight 去重、B30-01 链顶双判、C-4 锁外实时复核、B23-01 doneHas 胜利状态保护）✅；窗口判据（windowClosedLocked 三判据单源 + 10s 裕量双侧 + 零值守卫让位 WindowOpened）✅；task_log 窗口 SQL（空表/1 行/2020 行语义实证 + 批插 0.5s）✅；api 鉴权族（401/403/404/429/500 家族 + requireJSONBody CSRF 门）✅。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员）+ maybeRelogin 决策侧 B43-01/写回侧 B21-03 + waitChainExit 等待契约；6+1 个同名重建测试全绿。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败（带开放时间已过）+ 幽灵窗口（带 10s 裕量）；StateForAccount/WindowClosed/admin stats 三路共用。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生普通会话；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 32 字节双重校验；登录/激活独立限流桶；XFF 仅回环+开关。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；refuseLegacy 缺列清单与已迁移列剔除对应；窗口 SQL 语义实证正确。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 中点近似 + 成功才推进 + 失败 30s 退避 + streak≥3 复位（B21-01 不回零语义）。
- **数据库迁移规范**：migrateAddPublishMeta 幂等；旧库缺纯新增列自动迁移、缺语义列拒绝启动——与 CLAUDE.md 契约一致。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义——除 MINOR-56-01 的 open_time_set 过期判定分叉外全部自洽。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | 双通道 exit 0 |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` 12 轮 | R1 **zhidao FAIL** 24.3s（TestLoginStopsAfterCaptchaExhausted，captchas==0） / R3 **api FAIL** 45.4s / R11 **api FAIL** 48.6s；R2/R4/R5/R6/R7/R8/R9/R10/R12 全绿（9/12，75%） |
| 全量各包耗时（绿轮范围） | accounts 1.4-2.0s / api 21-50s / config 1.3-2.6s / db 2.6-4.0s / runtime 1.3-2.0s / scheduler 14.5-15.5s / secure 1.3-3.4s / session 1.5-1.9s / store 39-85s / zhidao 2.4-5.1s |
| api 隔离 `-race` 10 轮 | **0 FAIL**（25.4 / 25.1 / 51.9 / 54.3 / 26.7 / 18.6 / 26.3s 等） |
| zhidao 单用例复跑 | TestLoginStopsAfterCaptchaExhausted 0.07s PASS |
| 窗口测试耗时 | 无 race 0.57s（R55 批插实证成立）；race 下并入 store 全包 39-85s，10m 默认超时安全 |
| 空库契约测试 | TestLoadLogsEmptyDBWindowSemantics 0.62s 绿 |
| httpDo 重试 body 双发实证 | RoundTrip#1 body="classId=61115" → RoundTrip#2 body=""（独立程序） |
| 测试函数总数 | 208 个（全仓 `func Test` 计数） |
| 审查期间工作树 | 与基线一致：仅 5 个社区文档 + round56-frontend-findings.md（前端对等审查产物）未跟踪 |
| R55"非 api 包 0 FAIL" | **本轮不成立**：R1 zhidao FAIL |

---

## 结论

- **MAJOR 1（测试基建 flake 未归零 + zhidao 新样本 + 全量/隔离口径分裂）/ MINOR 1（open_time_set 注释-行为分叉）/ OBSERVE 6 新增 + 延续 9 项**。产品逻辑零缺陷。
- **flake 统计结论**：全量 `-race -p 1` 12 轮 **3/12 FAIL**（api×2、zhidao×1，全部 Windows 回环冷启动 connectex 家族）；api 隔离 10 轮 0 FAIL。**全量轮是唯一可信 flake 判定源，包隔离归零不可信**。
- **最致命 3 条（按影响排序）**：
  1. **MAJOR-56-01**：R55"非 api 包 0 FAIL"结论被 zhidao 样本打破；全量 flake 率 ~25% 高于隔离口径——修复方向：zhidao 包夹具对齐 readyProbe 或包级 TestMain 一次性 preheat，后续轮以全量连续多轮统计。
  2. **MINOR-56-01**：admin stats open_time_set 忽略识别过期（与学生端 open_time_known 分叉），handler.go:884/handler_test.go:956 注释与实现矛盾——修注释或修判定，二选一。
  3. **OBSERVE-56-01**：httpDo read 类错误重试空 body（实证）——方向安全但重试无效、日志误导；若收口一行 GetBody 即可。
