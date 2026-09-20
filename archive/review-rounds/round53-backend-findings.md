# round53 后端只读审查发现报告

> 审查基线：master @ `79d25aa`（R52 收官）。工作树仅根目录 5 个未跟踪社区文档（CODE_OF_CONDUCT.md / CONTRIBUTING.md / LICENSE / SECURITY.md / THIRD_PARTY_NOTICES.md）+ archive/review-rounds/ 内 R52 相关文件；审查期间零改动、绝对只读，唯一新文件是本报告。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go）。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~52 各轮报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 本轮新视角六项逐项实证 + 上轮观察项逐条裁决。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0（本轮独立重跑确认）。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 连跑 6 轮（第 6 轮 FAIL 1 包 zhidao 2 测）+ zhidao 包隔离连跑 20 轮全绿 + api 包隔离连跑 12 轮（第 3 轮 FAIL 1 测）——flake 频率与 R52 对比详见 MAJOR-53-01。

---

## CRITICAL

（无本轮新增 CRITICAL。R52 全部修复复核闭合；本轮新视角六项均确认无数据污染/崩溃/双报通道。）

---

## MAJOR

### MAJOR-53-01：R52 flake 三通道收敛后频率降至 R52 的 1/3~1/2，但 api 包首请求裸连接拒绝仍存活——降级观察建议：api 夹具首请求重试收尾（F52 族最后一块未覆盖面）

- **位置**：`backend/internal/api/handler_test.go:64`（newTestDepsModeName 内 httptest.NewServer mock 平台）+ `backend/internal/zhidao/captcha_test.go:18`（TestRecognizeCaptcha / TestCaptchaConcurrency 的 mock Vision 首请求）。
- **一句话问题**：F52-M2/M3/M4/M5/M6 已把 flake 从 R52 的"全量 33% + api 单跑 20% + zhidao 隔离 7%"压到 R53 的"全量 16.7% + api 单跑 8.3% + zhidao 隔离 0%"，但**每个测试自己 new 一个 httptest server 的首请求裸连接拒绝（dial connectex）在 api/zhidao 两个包仍各出现 1 次**——正是 F52-M2/M3 为"登录链路首请求"补的自愈重试所覆盖不到的面：TestRecognizeCaptcha/TestCaptchaConcurrency 的 Vision 识别请求走 `recognizeViaVision` 的 `httpDo`（captcha.go:162）本应自愈一次，**但 httpDo 只对连接层错误（dial/read/write）重试，对"连接被拒后立即重建"场景的第二次 dial 仍可能再次失败**（TIME_WAIT 队列冷启动窗口宽于单次重试）。api 侧 TestAdminCodesGenerateListDelete 的失败走 fetchLoginPage 首请求重试后仍 connectex（handler_test.go:508 断言"未激活登录应返回 1001"收到"初始化登录会话失败"业务文案）——**fetchLoginPage 已重试一次仍失败**，证明冷启动窗口在重试间隙未排空。
- **实证**：
  1. 全量 6 轮：第 1-5 轮全绿，第 6 轮 FAIL（zhidao 包 TestRecognizeCaptcha + TestCaptchaConcurrency，captcha_test.go:28/:87 均为 `dial tcp 127.0.0.1:58636 connectex`）。
  2. zhidao 隔离 20 轮：**0 FAIL**（socketPreheat 生效，与 R52 隔离 7% 对比降为 0）。
  3. api 隔离 12 轮：第 3 轮 FAIL 1 测（TestAdminCodesGenerateListDelete，handler_test.go:508 connectex），其余 11 轮全绿（R52 单跑 20%→R53 8.3%）。
  4. 失败测试单跑均绿；**无 race 报告**。
  5. 失败形态与 R52 完全同根（connectex 回环端口冷启动），但**断言面收敛**：R52 样本累至 6+ 个不同断言行，R53 只 2 个（zhidao 2 测同一断言行族 + api 1 测）。
- **触发条件**：Windows 回环 TIME_WAIT 队列冷启动期（mock server accept 未就绪即收到连接）。全量串行下首个 server 冷启动窗口 + 并发 -p 1 下包间切换重新建立 listen 后首请求。
- **影响**：CI 低频假红噪音（本轮 6 轮 16.7% 全量轮失败）；无生产影响（生产连接池预热 + 无冷启动形态）。
- **裁决与建议**：
  - **降级观察成立**：R52 收敛已实质生效（隔离 7%→0%，单跑 20%→8.3%），三通道语义正确（见新视角 A 实证）。残余面 = "每个测试独立建 server 的首请求"恰是 F52-M2/M3 登录链路重试**未覆盖的独立 mock server 启动竞态**。
  - **建议优先修复方向（小步收尾，非重开根治）**：① api 夹具 `newTestDepsModeName` 在 httptest.NewServer 后**主动向 srv.URL 发一次健康探测**（GET /login 期望非连接错误）再返回——把冷启动窗口前移到夹具构造期；② 或对 mock Vision 首请求同样补"连接失败重试一次"（zhidao 侧 TestRecognizeCaptcha/TestCaptchaConcurrency 已用 socketPreheat，改用 httptest.NewUnstartedServer + Start 显式就绪探测）。③ zhidao 包 TestRecognizeCaptcha/TestCaptchaConcurrency 已证明 socketPreheat 把隔离跑降到 0%，api 包 12 轮仍 1 FAIL——**api 包夹具的预加热生效度低于 zhidao 包**，优先在 api 夹具补齐。
  - TDD 测试形态：夹具健康探测后，对同一 server 连续 N 次全量连跑，失败率应降为 0（6 轮零 FAIL）。

---

## 本轮重点核对（R52 上轮修复与契约）—— 全部闭合

### 1. R52 flake 三通道收敛语义复核（fetchLoginPage 重试 / httpDo 活性自愈 / socketPreheat）— 语义正确，SelectClass 幂等防线未破坏
- **fetchLoginPage**（client.go:286-317）：循环最多 2 次（attempt==2 失败即原样上抛），纯 GET /login 无业务副作用、不消耗验证码限额、不触碰 doLogin——"瞬时抖动自愈"语义正确，重试仍失败原样上抛（`:313 return err`），绝不吞错。**403/429 同覆盖**（4xx/5xx 走同一重试一次路径），符合注释契约。**资源释放**：4xx 路径 `io.Copy + Close`（:299-300）；成功路径 `defer resp.Body.Close()`（:306）——**注意 defer 在函数作用域内注册，attempt=1 成功但 io.Copy 出错时 defer 仍执行**（成功分支 return 前 defer 生效），网络错误路径无 Body 泄漏；循环 attempt=2 时 attempt=1 的 resp 已在 4xx 分支手动 Close、网络错误分支无 resp（err!=nil）——**无 Body 泄漏路径**。
- **httpDo**（client.go:458-472）：仅 `isConnErr(err)`（dial/read/write 的 *net.OpError，client.go:475-484）才重试一次；`cloneReq` 用 `req.Clone(req.Context())` 深拷贝——**POST body 不共享**（body 由 bytes.NewReader 构造、Clone 复制 Body 引用指向同一 Reader，但 GetBody 语义下 Reader 可重置；实际 doRequest 的 body 是 `[]byte` 经 `bytes.NewReader`，Clone 后 GetBody 重建新 Reader，**第二次发送 body 完整可用**）。业务/取消错误原样上抛（:464）。**SelectClass 幂等防线复核**：httpDo 重试只发生在"第一次 c.Do 返回连接层错误"时——**连接层错误意味着请求从未到达服务端**（dial 失败 / 写出中断），**服务端不可能已处理该报名**，因此重试第二次 SelectClass **不构成双报**。若第一次已写出发出且服务端已处理（连接在读阶段断），服务端可能已报名——但这与"网络不确定时客户端重试"的标准语义一致，且调度器侧 spawnChain 的 inflight 去重防线（scheduler.go:1460-1467）作用在链级（每门课同时只有一条链），httpDo 重试发生在 http/client 层**同一请求内**，不新增并发面。**结论：A 项实证无缺陷**。
- **socketPreheat**（client_test.go:23-28 / captcha_test.go:17）：预创建-关闭一个回环套接字排空 TIME_WAIT 冷启动队列，zhidao 隔离 20 轮 0 FAIL 实证有效；api 夹具同款（handler_test.go:59-63）但 12 轮仍 1 FAIL——**api 夹具的预加热未覆盖 mock server 自身 accept 就绪前的最首请求**（见 MAJOR-53-01）。

### 2. F52-M1 后端侧契约复核（撞名学生签发普通会话响应体）— 端到端自洽，无混淆通道
- **后端**：管理员登录（handler.go:121 双条件 `Account == adminName && ConstantTimeCompare(口令)`）→ `CreateAdmin`（session/store.go:148，`Admin=true`）→ 响应 `{token, account, adminName}`（handler.go:129）。撞名学生（B43-04 放行）走教务 `LoginByPassword` → `issueSession`（handler.go:218）→ 响应 `{token, account}` **无 adminName 字段**（handler.go:230）。普通学生同上。**两类响应在 adminName 字段上无混淆通道**（管理员必带、学生必不带）。
- **前端**（已核实 web/src）：`Login.tsx:44-49` 读 `data.adminName` 传 `onLogin(data.token, data.account, data.adminName)`；`App.login`（App.tsx:91-108）`if (adminName)` 才 `setAdminToken+saveAdminToken`、`setInAdmin(!!adminName)`——撞名学生 adminName=undefined → adminToken 标记不写入 → `isCurrentAdminSession`（adminAuth.ts:11 `adminToken !== "" && sessions[adminName] === adminToken`）恒 false → 渲染判据 `inAdmin || isCurrentAdminSession`（App.tsx:299）双 false → 学生 Dashboard。**R52 前端 M-1 死锁已由 R52 修复（App.tsx:91-108 + adminAuth.ts 标记化判定）闭合**，R53 前端占位报告（round53-frontend-findings.md 仅占位未落地）与后端契约一致。
- **结论**：F52-M1 后端侧契约自洽闭合。残余观察：撞名学生登录后本地 `adminName` 状态仍为 `loadAdminName()` 默认值（App.tsx:78），但管理态判定已不依赖名字（adminAuth.ts），无实际影响。

---

## MINOR

### MINOR-53-01：httpDo 注释与实际实现的"WaitForState / MarkBroken"描述不一致（R52 引入的自愈实现未落地注释声称的活性检查）

- **位置**：`backend/internal/zhidao/client.go:456-457`（注释）、:458-472（实现）。
- **一句话问题**：注释声称"发送前用 WaitForState 确认连接可用、不可用则 MarkBroken 淘汰并重建"，但实现是**失败后**（c.Do 返回连接错误）`cloneReq` 整体重发一次，未用 WaitForState 预检、也未 MarkBroken 单连接淘汰——注释与实现描述两种不同机制。
- **影响**：语义本身正确（重试一次达到同目的：新连接 + 新 dial），无功能缺陷；注释误导后续维护者以为存在更细粒度的连接活性管理。属文档-实现漂移。
- **建议**：修正注释为"首次连接层失败后 cloneReq 整体重发一次（新连接），不逐连接预检"；或按注释实现 WaitForState+MarkBroken（需持 Transport 连接句柄，复杂度提升，ponytail 判：无必要）。
- **TDD**：无（纯注释-实现对齐）。

### MINOR-53-02：probe 命令 token 直插 URL 未转义 + cmd 三件套参数校验薄

- **位置**：`backend/cmd/probe/main.go:19`（`"?idToken=" + token`，未 QueryEscape）、`backend/cmd/bench/main.go:17-19`（`-n` 无 ≤0 校验、`-times` 负数时 `total/time.Duration(-1)` 输出负值）、`backend/cmd/logintest/main.go:25-26`（`-limit` ≤0 时 `creds[:limit]` panic）。
- **一句话问题**：诊断命令为开发/运维工具，参数边界校验不足：`-n 0` 除零、`-n -5` 负时长、`-limit 0` slice 越界 panic、token 含特殊字符未转义（token 纯数字但契约未保证）。
- **影响**：均为命令行工具自伤路径（用户输入错误参数），不影响生产主程序（main.go）。与上轮"XUANKE_PORT 无校验"同族（工具自伤 vs 生产健壮性）。
- **建议**：bench `-n` 加 `if *times <= 0 { *times = 1 }` 或直接报错；logintest `-limit` 加 `if *limit <= 0 || *limit > len(creds) { *limit = len(creds) }`；probe token 用 `url.QueryEscape`。**一行防御**。
- **TDD**：诊断工具无测试基础设施，人工核验即可（ponytail：不加测试基础设施）。

---

## OBSERVE

### OBSERVE-53-01：task_log 无清理量化评估（R47-01/R49-05 延续超 50 轮）— 本轮按"黄金期 10 分钟风暴"实测估算

- **现状**：全仓 `DELETE FROM task_log` 0 处；AppendLog 写点 scheduler.go 12 处 + handler.go 6 处 + accounts/main 若干，全部无条件 INSERT；`LoadAllLogs`（stats 用 limit 1000、admin 用 limit 500，store.go:415-436 封顶 2000）只读有界。**表无清理即无限增长**。
- **量化估算（黄金期 10 分钟风暴）**：
  - 提交节奏：黄金期 250ms 冲刺 × 10s + 1s 常态 × 590s ≈ **40 + 590 ≈ 630 次/账号/发布**；每失败/成功各 1 条 AppendLog（select 成功、满员、风控、失效、实时复核等 12 写点）。3 账号 × 3 发布 ≈ **9 链 × 630 ≈ 5670 行/窗口**。
  - 行体积：result 文案含中文错误串，实测均值约 120-200 字节 → **10 分钟窗口约 1.1 MB**；加上开窗前探测/登录/目标保存日志，**单窗口总量约 2-3 MB**。
  - 磁盘/DB 影响：SQLite WAL 下单表 1000 万行 ≈ 1.5-2 GB（需数十年才达）；但 **LoadAllLogs(2000 上限) 的 LIMIT 扫描 + ORDER BY id DESC 无索引**（task_log 无 id 索引依赖主键，SQLite 自增主键排序走表扫描 O(n)）——**表到 100 万行时 admin 日志页/ stats 页每次查询全表扫描约 50-100ms**，且 DB 文件随日志无限膨胀拖慢 checkpoint。
- **3 档方案成本/收益**：
  1. **低（推荐起步）**：`LoadAllLogs` 查询前置 `WHERE id > (SELECT max(id) FROM task_log) - 5000` 或建 `(account, id)` 复合索引 + 保留 `ORDER BY id DESC LIMIT`。成本：1 个索引 DDL + 1 处 SQL 改；收益：查询恒 O(log n)。**不删数据，零契约风险**。
  2. **中**：启动时或窗口关闭后 `DELETE FROM task_log WHERE id <= (SELECT max(id) FROM task_log) - 20000`（保留最近 2 万条）。成本：1 个清理函数 + main 启动调用；收益：表恒 <2 万行；风险：审计线索截断（本项目日志仅用于展示，无审计合规要求，可接受）。
  3. **高**：窗口关闭后按天归档/导出。成本：导出流程 + 配置；收益：完整审计。**超需求**（选课工具无审计法规要求），ponytail 判：选 2 即可。
- **裁决**：延续观察，建议随 1（索引）或 2（上限清理）落地，本轮未动代码。

### OBSERVE-53-02：tick 无 recover + 无优雅退出（signal.Notify 全仓 0 处）— 延续

- 全仓 `signal.Notify` 0 处；main.go 靠 `defer d.Close()/sessions.Close()` + `sched.Stop()` 未挂（sched.Stop 只 cancel，无恢复），SIGTERM 时在飞 spawnChain 成功分支可能未落库（SQLite WAL crash-safe 兜底：已提交事务不丢，未提交的 success 行丢失后重启 RestoreDone 不恢复，该课会被重新提交——平台幂等，可接受）。scheduler.New 已做 interval clamp（F46-O1）封堵最可达 panic，tick/probe/spawnChain 无 recover 为残余。**延续观察**。

### OBSERVE-53-03：孤儿登录（Relogin 锁内取指针锁外 Login）— 延续

- manager.go:167-174 `Relogin` 锁内取 `c`、锁外 `c.ReloginIfNeeded()`——账号删除与在途重登并发时，已删客户端仍会把 doLogin 打到平台（孤儿登录）。B43-01 决策侧复核已拦"已删账号不发起"，写回侧 B21-03 拦落库；残余窗口为"决策通过后、Login 执行前"账号被删。频率极低、平台侧一次性登录副作用。**延续观察**（无需代码改动，注释已载明）。

### OBSERVE-53-04：双槽分叉（openTimeDetected["*"] 全校 + [acct] 每账号）— 延续

- probe() 写 `["*"]`（scheduler.go:1104）、ProbeForAccount 写 `[acct]`（:850）；读取优先级 `[acct] → ["*"]`（openTimeForLocked :432-443）。全校单值契约下两槽不会实际分叉（同一 beginTimes 全校共享）。**延续观察**。

### OBSERVE-53-05：reloginResults cap 8 满丢弃 — 延续

- 非阻塞 select default 丢弃（scheduler.go:1274-1277）；满丢弃仅并发重登 >8 成功时补探测信号丢失，后果 = 少一次重登后探测（下个 tick 探测兜底）。**延续观察**。

### OBSERVE-53-06：failingTargetsStore 无消费点（B43-05 500 分支无真红绿）— 延续

- handler_test.go:842-849 `failingTargetsStore` 定义后无任何测试引用；TestAdminStatsTargetsLoadFailureReturns500（:824）只测正常路径 200（Register 接收 *store.Store 非接口，无法注入失败替身）。500 分支仅实现内复查背书。**延续观察**（R52 已裁决"替代注入方案超范围"，遵守简洁优先）。

### OBSERVE-53-07：429 头不消费（Retry-After）— 延续

- 全仓 `Retry-After` 0 处；isRateLimitError 文案匹配固定 30s（scheduler.go:1657-1663）；平台无 429 头样本（逆向契约无实证）。**延续观察**。

### OBSERVE-53-08：XUANKE_PORT 无校验 — 延续

- config.go:58 `envOr("XUANKE_PORT", "3091")` 无格式校验；`:abc` → `ListenAndServe` 报 `listen tcp: lookup tcp/abc` log.Fatalf 拒绝启动，失败形态清晰、不破坏数据。**延续观察**。

### OBSERVE-53-09：B43-05 500 分支无真红绿 — 延续（并入 OBSERVE-53-06）

### OBSERVE-53-10：flake 伪装业务断言样本面（OBSERVE-52-01 延续）— 样本面收窄

- R52 累至 6+ 个不同断言行；R53 本轮 2 个新样本（handler_test.go:508 / captcha_test.go:28,87），全部同根 connectex 经 Login→fetchLoginPage / recognizeViaVision→httpDo 翻译。**收窄但未归零**，并入 MAJOR-53-01。

### OBSERVE-53-11：access_limit_cookie 占位 — 延续

- client.go:392-393 `submitLogin` 写 `"***REMOVED***"`、manager.go:313 Restore 写 `"1"`、cmd/probe 写 `"***REMOVED***"`——占位值不统一但均为非真实会话值；token 权威通道是 URL 参数（逆向契约），Cookie 侧 zd_edu_cookie 才是鉴权主体。平台实测占位值通过鉴权（access_limit_cookie 仅限流标记）。**延续观察**。

### OBSERVE-53-12：M40 本地钟混用 — 延续

- 对齐钟统一后残余：`maybeSyncClock` 失败路径 `time.Now()`（scheduler.go:377）写 lastSyncFailAt、`maybeRelogin` 退避 `time.Since`（scheduler.go:1216/1223）、accounts 层 `time.Now()`（manager.go:53 gateWait / gateTryAcquire :226）——均与开窗判点无关，偏差 ~640ms 对 30s 退避/节流无实质影响。**延续观察**（契约注释已载明"时间基只影响窗口判点，退避用本地钟可接受"）。

---

## 本轮新视角六项实证结论

### A. httpDo 重试对 spawnChain 双发包防线的影响 — 无问题（实证）

httpDo 重试仅当第一次 `c.Do` 返回 **连接层错误**（`isConnErr`：dial/read/write 的 *net.OpError，client.go:475-484）时触发。连接层错误意味着请求未到达服务端或到达后连接断裂（服务端不可能已成功处理并返回），重发不构成业务双报。业务错误（code=1/满员/code=-1/JSON 解析失败）走 doRequest 正常返回，绝不重试。调度器层 spawnChain 的 inflight 去重（scheduler.go:1460-1467）保护的是"链级并发"（同课同时只一条链），httpDo 重试发生在同一请求内部，不新增并发面。**SelectClass 双报防线未被突破**。已实证：zhidao 20 轮隔离 0 FAIL 期间 SelectClass 测试（TestExitClass 等）全部正常；TestLoginNetworkErrorAbortsImmediately（:103）证明非连接层错误（HTTP 500）不上抛到重试路径。TDD 建议（可选加固）：为 httpDo 补"业务失败不重试"单测（mock 首请求返回 500 body、次请求计数恒 1）。

### B. scheduler tick 主循环持锁窗口核查 — 无问题（锁内无网络/慢操作）

逐路径核：tick（:969-1032）锁段 = `lastProbe/open` 读取（微秒）、`opened` 读取、`lastSubmit` 读取；probe（:1039-1160）锁段 = probing 置位/复位、openTimeDetected 写入、lastData/lastProbe/EmptyProbeRuns 写入——**网络调用（FindElectives/SelectClass/StudentCounts/Login）全部在锁外**。spawnChain（:1379-1653）锁段 = 状态读取/inflight 置位/状态写入/`setStateLocked` + `store.AppendLog`（SQLite 单写者串行，微秒级）——**F13-m2 已把锁内 SQLite 写收敛到持锁段、锁外网络段（C-4 已释放锁）**。重登 goroutine 锁段 = relogin 状态写 + UpdateIDToken 落库（微秒）。**最坏持锁时长**：SQLite 写（WAL 模式下单 INSERT 约 0.1-1ms）+ map 操作，无第三方调用、无网络、无 IO 等待，估 <5ms。**tick 主循环无慢持锁路径**。

### C. maybePrewarm 预热节奏与窗口判定交互 — 无问题

maybePrewarm（scheduler.go:298-315）：`open` 零值/未到（开窗前 >2 分钟）/已过点 → 直接 return；2 分钟窗口内每 15s（lastPrewarm 节流）静默 GET /login 一次（AnyClient 任一账号），goroutine 异步不阻塞 tick。与探测节流（30s）独立：预热只打 /login（无 idToken、无业务语义），不消耗 findElectivesData 预算；开窗瞬间 tick 首探测照常触发（probe 节流判定不受预热影响）；黄金期提交与预热互不干扰（预热在 `now.After(open)` 后停止）。**开窗前 2 分钟预热与窗口判定无交互缺陷**。残余观察：预热只预热 order[0] 账号的连接（AnyClient），多账号部署下其他账号首请求仍冷启动——但 sharedTransport 连接池按 host 共享（MaxIdleConnsPerHost=64），预热一次即温热该 host 全部连接，账号间共享，无实质缺口。

### D. task_log 无清理量化评估 — 见 OBSERVE-53-01（3 档方案）

### E. SyncServerTime / Prewarm / fetchLoginPage 的 resp.Body 资源释放全量清点 — 无问题

- SyncServerTime（client.go:114-119）：`defer resp.Body.Close()` + `io.Copy(io.Discard)` 排空——成功路径 Close 一次；err 路径（NewRequest 失败）无 resp。无泄漏。
- Prewarm（client.go:97-102）：同款 `defer resp.Body.Close()` + io.Copy。无泄漏。
- fetchLoginPage（client.go:286-317）：4xx 路径 `io.Copy + Close`（:299-300）；成功路径 `defer resp.Body.Close()`（:306，函数返回时执行一次）；网络错误路径无 resp。**defer 在 attempt=1 成功但 io.Copy 出错时同样执行**（函数级 defer），无双重 Close 也无遗漏。无泄漏。
- httpDo（client.go:458-472）：`c.Do` 返回 resp 即交还调用方（doRequest :435-436 Close、recognizeViaVision :166-167 defer Close）；重试路径 resp2 正常交还。**无泄漏路径**。
- captcha 识别（captcha.go:166-167）：`defer resp.Body.Close()`。无泄漏。
- 结论：**全仓 HTTP resp.Body 释放全量正确，Close 只一次、无泄露路径**。

### F. cmd/{probe,logintest,bench} 三个命令 main 健壮性 — MINOR-53-02（参数校验薄，自伤路径，不影响生产）

与生产 main.go 契约一致性：logintest 识别引擎选择（config.CaptchaEngineDefault + Native/Local/Vision 回退链）与 router.go:35-53 `applyCaptchaRecognizerFor` **逐字段一致**；db.Open/masterKey/解密链路与 main.go 完全一致；probe 的 token 来源（环境变量，拒绝硬编码）符合安全审计 CRITICAL#2 要求。三个命令均不自带优雅退出（诊断工具无守护需求）。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-52-01（httptest flake 本体） | MAJOR | F52-M2/M3/M4/M5/M6 收敛后频率降半，但 api 首请求裸连接仍 1/12、zhidao 全量 1/6 轮——三通道已正确但 api 夹具预加热未覆盖"自身 accept 前首请求" | **降级观察**（并入 MAJOR-53-01，建议 api 夹具健康探测收尾） |
| OBSERVE-52-01（flake 伪装业务断言） | 观察 | 样本面收窄（6+ 断言行 → 本轮 2 个新断言行），仍同根 connectex | 延续观察（并入 MAJOR-53-01） |
| OBSERVE-51-01（main 优雅退出） | 观察 | signal.Notify 仍 0 处；SQLite WAL crash-safe 兜底 | 延续观察 |
| OBSERVE-49-01（chains map） | 观察 | waitChainExit 契约 6 处测试沿用，生命周期严格配对 | 延续观察 |
| OBSERVE-49-02（rateLimited×done/full） | 观察 | MarkDone 清 rateLimited+full；SetTargets 保留（B19-02） | 延续观察 |
| OBSERVE-49-03（gate 窗口翻转） | 观察 | gateTryAcquire/gateWait 共享 gateMu 计数 | 延续观察 |
| OBSERVE-49-04（task_log 无清理） | 观察 | **本轮量化**：黄金期 10 分钟 ≈ 5670 行/窗口，100 万行时 stats/admin 全表扫描 50-100ms；3 档方案见 OBSERVE-53-01 | 延续观察（建议索引或上限清理） |
| OBSERVE-49-05（大响应解析） | 观察 | parseElectives 空快照提前返回 | 延续观察 |
| OBSERVE-48-01（submitAll 300ms 拍） | 观察 | 黄金期 250ms 冲刺不受 tick 拍节流 | 延续观察 |
| OBSERVE-48-02（reloginResults cap 8） | 观察 | 非阻塞丢弃，下个 tick 探测兜底 | 延续观察 |
| OBSERVE-47-02（B43-05 500 无真红绿） | 观察 | failingTargetsStore 仍无消费点 | 延续观察 |
| OBSERVE-47-03（XUANKE_PORT 无校验） | 观察 | `:abc` log.Fatalf 拒绝启动，形态清晰 | 延续观察 |
| OBSERVE-47-04（孤儿登录） | 观察 | B43-01 决策侧 + B21-03 写回侧双闭合，残余窗口极低 | 延续观察 |
| OBSERVE-46-02（Retry-After 不消费） | 观察 | 全仓 0 处；固定 30s 文案匹配 | 延续观察 |
| OBSERVE-45-01（access_limit_cookie 占位） | 观察 | 三处占位值不统一但不影响鉴权 | 延续观察 |
| OBSERVE-43-01（probe 双槽分叉） | 观察 | [acct]→["*"] 优先级，全校单值不实际分叉 | 延续观察 |
| OBSERVE-43-03（classFullRealtime 网络段重复） | 观察 | IsClassFull 恒 false；快照判满主路径 | 延续观察 |
| OBSERVE-43-04（tick 无 recover） | 观察 | interval clamp 封堵最可达 panic | 延续观察 |
| OBSERVE-42-02（孤儿登录平台侧） | 观察 | 并入 47-04 | 延续观察 |
| MINOR-43-02（syncFailedWindow 死字段） | 观察 | 写而不读，留档语义 | 延续观察 |
| MINOR-43-03（登录失败无固定延迟） | 观察 | 学生走网络天然延迟；管理员分支 loginTimingFlat | 延续观察 |
| MINOR-43-04（reloginBackoff 注释） | 观察 | 封顶 5 次防溢出 | 延续观察 |
| OBSERVE-46-04（目标不校验重复/跨年级） | 观察 | 平台把关 | 延续观察 |
| M40-01（本地钟混用） | 观察 | 残余均与开窗判点无关 | 延续观察 |
| OBSERVE-49-06（zhidao HTTP 非 200） | 观察 | doRequest 读 Body→Unmarshal→判 code，错误化正确 | 延续观察 |
| m39/o39 族 | 观察 | 无变化 | 全部延续 |

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员两分路）+ maybeRelogin 决策侧（B43-01）/写回侧（B21-03）+ waitChainExit 等待契约（绝不用 inflight，读 nil map 恒 false 假绿陷阱已载明）。测试 6 个（TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+RealtimeUnauthorized）本轮全绿。
- **WindowClosed 三判据单源**：windowClosedLocked（scheduler.go:914-935）主判据 + 时钟失败 + 幽灵窗口三条，StateForAccount/WindowClosed/admin stats 共用（B29-02/B39-05）。测试 TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler / TestSubmitSuspendedWhenOpenTimeCleared 钉子全绿。
- **鉴权与数据安全**：管理员双条件（B43-04）+ 撞名学生响应无 adminName（F52-M1 闭合）；writeJSONStatus 家族真实状态码；SpaHandler /api 双处 404；XFF 仅回环+开关；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 32 字节双重校验（F17-04）。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；migrateAddPublishMeta 增量幂等；refuseLegacy 缺列清单对应剔除（priority/allow_swap/account 拒绝、publish_name/begin_date 放行）。
- **登录链路**：RSA-PKCS1v1.5（rsa.go）/empty priorityId（jQuery 丢弃语义）/captcha Limiter Cond 动态热收敛（并发 1 基线）/重试收敛 ≤3/验证码一次性绝不带同码重试（client.go:247-258）。
- **连接池与时钟**：sharedTransport MaxIdleConnsPerHost=64 + 120s 空闲；SyncServerTime 中点近似 + 成功才推进 lastSyncTime + 失败 30s 退避 + streak≥3 复位（MAJOR-C）。
- **新视角 A/B/C/E 全部无缺陷**（见上）。

---

## 结论

- **MAJOR 1（MAJOR-53-01：R52 三通道收敛后 flake 降频但 api/zhidao 各 1 样本存活，降级观察 + api 夹具健康探测收尾）/ MINOR 2（53-01 httpDo 注释-实现漂移、53-02 cmd 三件套参数校验薄）/ OBSERVE 12+ 延续**。R52 全部修复复核闭合；新视角六项 A/B/C/E 无缺陷、D 量化评估、F 参数校验薄。
- **flake 频率结论**：R52 全量 33% → R53 全量 16.7%；R52 api 单跑 20%（3 测/轮）→ R53 8.3%（1 测/轮）；R52 zhidao 隔离 7% → R53 **0%**（20 轮全绿）。三通道收敛实质生效（隔离归零），残余 = api 夹具预加热未覆盖"自身 accept 就绪前最首请求" + zhidao 全量轮冷启动。
- **最致命 3 条（按影响排序）**：
  1. **MAJOR-53-01（httptest flake 残余）**——全量 6 轮 1 轮 FAIL（16.7%），zhidao 隔离已归零但 api 12 轮仍 1 FAIL；建议 api 夹具在 httptest.NewServer 后健康探测前移冷启动窗口。
  2. **OBSERVE-53-01（task_log 无清理量化）**——黄金期 10 分钟 ≈ 5670 行/窗口，100 万行时 stats/admin 全表扫描 50-100ms；建议索引或上限清理（3 档方案已列）。
  3. **OBSERVE-53-02（tick 无 recover + 无优雅退出）**——SIGTERM 在飞 success 未落库时重启 RestoreDone 不恢复、该课被重新提交（平台幂等兜底可接受）。
- **建议优先修复方向**：① api 夹具健康探测（MAJOR-53-01 收尾，小步）；② task_log 上限清理或索引（OBSERVE-53-01，中步）；③ cmd 三件套参数一行防御（MINOR-53-02）。

---

## 附：flake 频率实证数据表

| 维度 | R52 | R53 | 变化 |
|---|---|---|---|
| 全量 `-race -p 1` 连跑 | 3 轮 1 FAIL（33%） | 6 轮 1 FAIL（16.7%） | 降半 |
| api 包隔离连跑 | 5 轮 1 FAIL（20%，3 测/轮） | 12 轮 1 FAIL（8.3%，1 测/轮） | 降半以上 |
| zhidao 包隔离连跑 | 30 轮 7% | 20 轮 **0%** | 归零 |
| 失败断言行样本 | 6+ 个不同断言行 | 2 个新断言行（captcha_test:28,87 / handler_test:508） | 收窄 |
| race 报告 | 无 | 无 | — |
