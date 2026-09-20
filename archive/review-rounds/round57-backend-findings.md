# round57 后端只读审查发现报告

> 审查基线：master @ `808a4e6`（R56 收官，2026-09-21）。工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 round57 相关文件；审查期间绝对只读（唯一允许新建的产物即本报告与临时验证程序，均落 /tmp 与报告），写入前 `git status --short` 复核：工作树与基线一致（见文末实证表）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部 208 个测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~56 各轮报告逐条复核 + R56 文末观察项延续。
> 方法：全包逐行通读 + 标准库源码核证（GetBody/Clone/transport 重试）+ 独立 Go 程序实证（真实 doRequest 路径 read 错误重试 body）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` **11 轮**。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0。

---

## 结论先行

- **CRITICAL 1 / MAJOR 1 / MINOR 1 / OBSERVE 7 新增 + 延续 9 项**。
- **最致命 3 条（按影响排序）**：
  1. **CRITICAL-57-01：R56 cloneReq "GetBody 重生成"修复在真实 doRequest 路径下不生效，且"若生效"会引入 SelectClass/ExitClass 双报**——标准库 `http.NewRequest` 对 `bytes.NewReader` 自动设置 GetBody（实测），R56 修复分支 `req.GetBody == nil` 对 doRequest 恒不命中；而 read 类连接错误恰好发生在"服务端已完整消费 body"之后（transport 非复用连接不重试、返回 read OpError，httpDo 外层 cloneReq 重试的是已空 body）。独立程序在真实路径上实证：`cloneReq` 重试请求 **body 为空 + ContentLength 恒 13**（畸形 POST），修复代码从未被执行。
  2. **MAJOR-57-01：全量轮 flake 未归零，api 包新增 4 个低频 FAIL 测试**——11 轮中 R1/R3/R8 FAIL（api×3），且 FAIL 测试分布广（TestLoginUnactivatedNeedsCode / TestAdminStatsOpenTimeFromRecognized / TestElectiveSelectRejectsWindowClosed / TestSetTargetsBounds 均样本），与 R56 收尾"2/14 全为 api 冷启动"一致——全部为 Windows 回环冷启动 connectex 家族残余，隔离复跑全绿。**zhidao 样本归零（TestMain preheat 生效）**。
  3. **MINOR-57-01：zhidao 包 `TestMain` 里 `log.SetOutput(io.Discard)` 破坏`TestLoginLogs*`日志捕获的"完整性"（隔离复跑实测偶发 FAIL）+ gofmt 违规（import 多一个 tab + 文件无尾换行）**——但隔离连跑 5 轮全部绿，全量轮 11 轮 zhidao 全部绿；该 FAIL 是 TestMain 与日志测试竞态（goroutine 刷 Discard 与测试缓冲切换）。

---

## CRITICAL

### CRITICAL-57-01：R56 cloneReq GetBody 修复在真实路径不生效（死代码），read 类错误重试 body 仍为空

- **位置**：`backend/internal/zhidao/client.go:499-512`（cloneReq）+ `client.go:465-479`（httpDo）+ `client.go:414-435`（doRequest）+ `captcha.go:151-152`（Vision 识别请求）。
- **一句话问题**：R56 为 `cloneReq` 补的 `req.GetBody == nil` 分支**对 doRequest 全部调用点恒不命中**——`http.NewRequest` 对 `bytes.NewReader`/`strings.NewReader` 自动设置 GetBody（标准库 request.go:932-945 实证），R56 注释声称"GetBody 未设时重试请求体为空"的语义只在**自构造原始 Reader 且未走 NewRequest** 的路径下成立，而该路径在真实代码中不存在。
- **证据链**：
  - **标准库源码核证**（`C:/Program Files/Go/src/net/http/request.go:932-945`）：`NewRequestWithContext` 对 `*bytes.Reader` 类型自动设置 `req.GetBody = func() ... snapshot`。`doRequest` 用 `http.NewRequest(method, u, bytes.NewReader(body))`（client.go:414）→ **GetBody 恒非 nil** → R56 修复分支 `req.Body != nil && req.GetBody == nil` 恒不进入。
  - **独立程序实证 1**（精确复刻 doRequest 构造 + httpDo + cloneReq）：`GetBody 是否已由标准库设置: true`；服务端第 1 次收到 body="classId=61115"；首 Do 报 read 类错误；httpDo 判定连接错误 → cloneReq 重试 → **重试失败 `http: ContentLength=13 with Body length 0`**（body 已空但 ContentLength 仍 13 → 发送端直接拒绝，重试请求根本没到服务端）。
  - **独立程序实证 2**（SO_LINGER=0 RST 场景，keep-alive 预热后首请求复用连接被 RST）：首 Do 报 `read tcp ... wsarecv: An existing connection was forcibly closed`（**read OpError**）→ httpDo 走 cloneReq 重试 → **`ContentLength=13 with Body length 0` 失败**，服务端只收到 1 次报名请求。
  - **transport 内部重试不兜底**（`transport.go:815-863` shouldRetryRequest）：`!pc.isReused()`（新连接）直接 `return false` 不重试；复用连接上 `nothingWrittenError` 分支要 `req.outgoingLength() == 0 || req.GetBody != nil`——**而 GetBody 已设置时 transport 会重放完整 body**（这正是 R56 想修的场景），但 read 类错误（transportReadFromServerError）只在 `req.isReplayable()` 为 true 时重试，而 `isReplayable` 对 POST 需要 GetBody 非 nil——**同样满足**。真实语义链：**transport 内部已通过 GetBody 把 read 类错误重试做掉了**，httpDo 外层 cloneReq 重试只会在"transport 自身决定不重试"时触发，而那时 body 已被消费。
  - **"若修复生效"的后果实证**（场景二）：用 GetBody 重放完整 body 重试 → 服务端第 2 次收到 body="classId=61115"，响应 200 OK——**平台会真实处理两次报名**。read 类错误（服务端已消费首次请求并可能已成功报名）后带完整 body 重试 = **双报风险**。
- **触发条件**：任何 `doRequest`（SelectClass/ExitClass/FindElectives/StudentCounts）在连接层 read/dial/write 错误下的 httpDo 外层重试。read 类错误概率最低但影响最大（服务端可能已处理）。
- **影响**：
  1. **R56 修复是死代码**（`req.GetBody == nil` 分支在真实路径恒 false）——声称修复的"重试空 body"缺陷**原样存在**（重试请求 `ContentLength=13 with Body length 0`，发送端直接报错、重试无效、追加一条误导性失败日志）。
  2. **修复方向错误**：若把分支条件放宽（如 `req.GetBody != nil` 时改用 `req.GetBody()` 重放），则 read 类错误重试会带完整 body——**SelectClass/ExitClass 双报**（平台已处理首次 + 重试再处理一次），违反 R52-M4 注释"重试不构成双报"的既有契约。R56 修复**方向安全但不生效**与"生效但双报"是同一枚硬币的两面。
  3. **正确语义**：read 类错误**不应该在 httpDo 层重试**（服务端已消费 body、可能已处理）；只有 dial/write 错误（请求未到达）才可安全重发。R52-M4 注释"连接层错误重试即等效换新连接、请求未到达/未完成"对 read 错误**不成立**（服务端已消费）。
- **修复方向（TDD）**：
  - ① **删除/收敛 httpDo 对 read 错误的自动重试**：`isConnErr` 保留 dial/write，read 错误**不上抛重试**——对齐"请求未到达才可重发"的真实契约，从根上消除"重试空 body"与"重试双报"两个方向的问题。`fetchLoginPage` 的 GET 自愈不受影响（GET 无 body，可安全重试）。
  - ② 若坚持重试 read 错误：cloneReq 需用 `req.GetBody()` 重放完整 body（`ContentLength` 同步正确）——但 SelectClass/ExitClass 必须**禁止**该重试（双报）。
  - ③ `captcha.go:162` Vision 识别请求同样受此影响（POST JSON body，read 错误重试空 body）——但识别幂等，双报无害，可保留或同收敛。
  - TDD：先写测试断言"read 类错误下 SelectClass 重试不触发双报 / 不空发"，再改代码。

---

## MAJOR

### MAJOR-57-01：全量轮 flake 未归零，api 包新增 4 个低频 FAIL 样本（R56 收尾"2/14"未收敛）

- **位置**：`internal/api/handler_test.go:417`（TestLoginUnactivatedNeedsCode）/ `:959`（TestAdminStatsOpenTimeFromRecognized）/ `:1044`（TestElectiveSelectRejectsWindowClosed）/ `:1302`（TestSetTargetsBounds）。
- **一句话问题**：全量 `-race -p 1` 11 轮 **3 FAIL**（R1/R3/R8，全为 api 包），4 个不同测试均出现样本，全部为 Windows 回环冷启动 connectex 家族；**隔离复跑全部恒绿**（登录族连跑 8 轮、admin stats+window 测试对连跑 5 轮、TestSetTargetsBounds 连跑 5 次全绿）。R56 收尾声称"flake 率降至 14%"仅以 14 轮统计，本轮 11 轮实测仍 ~27%（3/11），**未归零**。
- **证据链**：
  - 全量 11 轮统计：R1 **api FAIL**（TestSetTargetsBounds 前断言行"目标应被拒绝"收到非预期响应，隐式连接失败）/ R3 **api FAIL**（TestLoginUnactivatedNeedsCode，断言 code=1001，登录连接失败）/ R8 **api FAIL**（TestAdminStatsOpenTimeFromRecognized + TestElectiveSelectRejectsWindowClosed 双 FAIL）/ 其余 8 轮全绿。
  - api 包隔离 10+ 轮 **0 FAIL**（含各类登录/stats/window 测试组合）。
  - zhidao 包全量 11 轮 **0 FAIL**（R56 TestMain preheat 生效，R56 的 zhidao 样本归零）。
  - store 包全量 44-201s 波动巨大（前序包残留 + SQLite 长跑），10m 默认超时安全。
- **触发条件**：Windows 宿主全量 `go test ./...`（-p 1 下 api 紧随 accounts，accounts 的 httptest 关闭后回环 TIME_WAIT 残留最多）；api 包夹具 readyProbe 已前移冷启动窗口，但 api 测试量大（52 个函数）、每个测试都新建 mock server + 独立会话，残余连接错误散布在不同测试上。
- **影响**：CI 低频假红（全量 ~27% 轮次 FAIL 某一 api 测试），`||` 重跑吸收但成本高；无生产影响。
- **修复方向（小步）**：
  - ① 统计口径维持"全量连续多轮"为唯一判定源（R56 教训 1 已是共识），本轮数据证实 R56 收尾的"2/14 降级"**没有稳定**——应恢复持续观察。
  - ② api 包夹具再加一层兜底：loginMockServer 的 mock server 对 `default:` 分支的 `/ready` 请求已由 readyProbe 覆盖，但登录/报名请求在探测成功后的首请求仍可能复用"探测刚建立的连接"前的 TIME_WAIT 队列残余——可考虑把 readyProbe 的探测请求改为**复用同一个 keep-alive 连接发起测试首请求**（双保险升级）。
  - ③ 定位用 `-run` 隔离、修复用统一套接字 preheat + readyProbe 组合即可，不必逐测试打补丁。
  - TDD 形态：全量连续 5 轮 0 FAIL 才可宣称闭合。

---

## MINOR

### MINOR-57-01：zhidao 包 `TestMain` 的 `log.SetOutput(io.Discard)` 破坏日志测试完整性 + gofmt 违规

- **位置**：`internal/zhidao/client_test.go:8`（import 段 `"log"` 多一个 tab 缩进）+ `:39`（TestMain 内 `log.SetOutput(io.Discard)`）+ `:511`（文件无尾换行）。
- **一句话问题**：①`log.SetOutput(io.Discard)` 是**包级全局**副作用，`TestLoginLogsAttempts/TestLoginLogsFailureSummary` 依赖 `log.SetOutput(&buf)` 捕获登录日志——TestMain 设的 Discard 与测试的缓冲切换在 goroutine 日志刷出时间上存在竞态（隔离复跑实测一次 FAIL：登录日志被 Discard 吞掉、断言 `strings.Contains` 落空）；②import 段 `"log"` 前多一个 tab、文件缺尾换行，`gofmt -l` 报 zhidao/client_test.go（R56 提交未过 gofmt）。
- **证据链**：
  - `gofmt -d internal/zhidao/client_test.go`：唯一差异 = `\t\t"log"` 应为 `\t"log"` + 文件末尾缺换行。
  - TestMain 的 `log.SetOutput(io.Discard)` 只在**进程最开始时**生效，与测试内 `log.SetOutput(&buf)` 是同一全局——登录 goroutine 在 TestMain 阶段刷 Discard、测试阶段刷 buf，本无直接覆盖；但 zhidao 全量轮跑 11 轮 0 FAIL、隔离连跑 5 轮全绿，仅一次 3 用例组合跑 FAIL——**低频竞态**，归因 TestMain Discard 与测试缓冲切换的窗口（登录日志在 `old := log.Writer()` 读取与 `SetOutput(&buf)` 之间的 goroutine 间隙被 Discard 吞掉）。
- **触发条件**：zhidao 包测试在跑（尤其含登录链路 + 日志断言的测试），高频执行时窗口重叠。
- **影响**：测试偶发假红（日志断言落空），无生产影响；gofmt 违规对 CI `gofmt -l` 检查会红（若未来接入）。
- **修复方向**：TestMain 改 `if os.Getenv("XUANKE_ZHIDAO_TEST_LOG") == "" { log.SetOutput(io.Discard) }`（保留静默默认但测试可显式恢复）或直接把 `SetOutput` 收敛到各测试内部（删掉 TestMain 的 Discard，让测试自行控制输出）；同时 `gofmt -w` 修 import 缩进与尾换行。

---

## OBSERVE

### OBSERVE-57-01：R56 cloneReq 修复的实际效果 = 死代码 + 误导注释（与 CRITICAL-57-01 同根，记录为独立观察）

- 即使不做行为修复，R56 在 cloneReq 加的分支和注释（"GetBody 未设时重试请求体为空"）描述的语义在真实代码中**不存在**——后续维护者看到注释会误以为修复生效，或按"GetBody 已设时应重放"的假设继续改（会引入双报）。**注释必须修正**：真实行为是"read 类错误重试 body 已空（GetBody 已被标准库设置但未被 cloneReq 消费）"，且 transport 内部已用 GetBody 把该重放路径做掉了。

### OBSERVE-57-02：R56 `log.SetOutput(io.Discard)` 会吞掉排查测试失败的日志线索

- TestMain 把包内全部 `log.Printf`（登录链路失败原因、识别引擎类型、探测日志）静默丢弃——测试 FAIL 时开发者只能看到断言行，看不到登录/识别/探测的中间日志。对**只读审查**和未来调试都是成本。建议仅对"已知噪音"静默（如成功路径日志），失败路径日志保留。

### OBSERVE-57-03：api 包 `readyProbe` 无显式超时（http.DefaultClient 超时为 0=无限）

- 与 R56 OBSERVE-56-04 同款（zhidao 包 readyProbe 同样无超时）——mock server 极端挂起时探测请求可阻塞至 OS TCP 超时（分钟级）。一行 `http.Client{Timeout: 2*time.Second}` 即可。延续观察。

### OBSERVE-57-04：`TestAdminStatsOpenTimeFromRecognized` 对"识别过期"的断言与注释意图仍有语义张力

- 测试注释写"识别过期语义下自动降级为未识别（绝不把过期旧值当开放时间）"，但断言 `open_time_set == !recog.IsZero()`——mock 的 beginTimes=1789261200000 是过去值，`recog` 非零 → 断言 `open_time_set=true` + 输出过期日期。这与 R56 修复后的注释"识别槽有值即照常输出过期日期"一致，但与测试注释首句"降级为未识别"矛盾。**测试注释应删去"降级为未识别"字样**（历史遗留，R56 只改了 handler.go 注释、漏了测试内注释）。

### OBSERVE-57-05：`httpDo` 注释仍称"重试即等效达成换新连接、请求未到达/未完成"——对 read 错误不成立

- CRITICAL-57-01 的直接后果：该注释（client.go:461-464）描述"连接层错误意味着请求未到达/未完成，服务端不可能已成功处理，重发不构成双报"——**对 read 类错误是错的**（服务端已完整消费 body）。注释应区分 dial/write（未到达，可重发）vs read（可能已到达并处理，不可重发）。

### OBSERVE-57-06：`TestSetTargetsBounds` 第 1 段（101 门）断言 `j["code"].(float64) == 0` 时 Fatal——对非 JSON 响应会 panic

- 若 mock/网络故障返回 HTML/空响应，`j["code"]` 为 nil，`nil.(float64)` 类型断言 **panic**（recoverMiddleware 返回 500，测试得到"内部错误"而非明确 FAIL 信息）。同理多个测试用 `j["code"].(float64)` 直断言。隔离实测恒绿，仅 flake 场景下可触发，**非本轮新缺陷**，延续观察（R1 的 FAIL 恰发生在该断言行，可能是此形态）。

### OBSERVE-57-07：`config.Load()` 每次调用都 `ensureEnvFile` 写盘——cmd 工具（probe/logintest/bench）与 api 测试若触发会写 data/.env

- config 包 `Load` 恒调 `ensureEnvFile`（生成/检查 .env）。cmd/bench 等工具若调用 config.Load 且无 XUANKE_ADMIN_TOKEN，会**写入仓库 data/.env**（含随机管理员口令）——审查只读约束下未实测调用，但风险存在（cmd/probe 只读 token env 不调 config，暂安全）。观察。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-56-01（全量/隔离分裂 flake） | 修复（zhidao readyProbe+TestMain） | **zhidao 归零**（11 轮 0 FAIL，TestMain preheat 生效）；api 残余 3/11 FAIL（4 个不同测试样本）——R56 收尾"2/14"未稳定，升级为 MAJOR-57-01 | **部分闭合**：zhidao 闭合；api 残余延续 |
| MINOR-56-01（open_time_set 注释对齐） | 修复（注释对齐） | handler.go 注释已改为"识别槽有值即照常输出过期日期"，与行为一致 ✅；但 handler_test.go 注释首句仍残留"降级为未识别"（OBSERVE-57-04） | **闭合**（测试注释残留 → 新 OBSERVE） |
| OBSERVE-56-01（httpDo 重试空 body） | 修复（cloneReq GetBody） | **修复不生效（死代码）**，升级为 CRITICAL-57-01 | **升级 CRITICAL** |
| OBSERVE-56-02（open_time_set 注释链） | 随 56-01 闭合 | handler 注释已修；测试注释残留 | **部分闭合** |
| OBSERVE-56-03（WriteTimeout 30s < Login 最坏耗时） | 延续观察 | main.go WriteTimeout 仍 30s、Vision 60s×3 仍存在；低频 | 延续观察 |
| OBSERVE-56-04（readyProbe 无显式超时） | 延续观察 | zhidao/api 两处 readyProbe 仍无 timeout | 延续观察（并入 OBSERVE-57-03） |
| OBSERVE-56-05（TestLoginLimiterGC 零值依赖） | 延续观察 | 未变 | 延续观察 |
| OBSERVE-56-06（全局信号量叠加） | 延续观察 | 未变 | 延续观察 |
| OBSERVE-55-02~06 / 53-02~12 / M40 | 全部延续观察 | 本轮逐条复核无变化 | 全部延续 |

---

## 本轮新视角六项逐项实证

### A. R56 三处修复是否正确
- **①TestMain preheat**：zhidao 全量 11 轮 0 FAIL（R56 R1 的 zhidao 样本归零）→ **正确且生效**。但 `log.SetOutput(io.Discard)` 引入日志测试完整性竞态（MINOR-57-01）+ 吞排查线索（OBSERVE-57-02）；socketPreheat 逐测试调用与 TestMain 双保险**不冲突**（TestMain 只在进程最始 preheat 一次，socketPreheat 每测试再 preheat）。
- **②cloneReq GetBody 重生成**：**不正确且不生效**——标准库 `NewRequest(bytes.NewReader)` 已设 GetBody，修复分支 `req.GetBody == nil` 恒 false（CRITICAL-57-01）。且"若生效"会双报。
- **③handler.go open_time_set 注释对齐**：注释与行为已一致（识别槽有值即输出过期日期、open_time_set=!IsZero），测试断言自洽 ✅；仅测试注释首句残留矛盾（OBSERVE-57-04）。

### B. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 11 轮 flake 统计
- 11 轮：**3 FAIL（~27%）**——R1 api（TestSetTargetsBounds）/ R3 api（TestLoginUnactivatedNeedsCode）/ R8 api（TestAdminStatsOpenTimeFromRecognized + TestElectiveSelectRejectsWindowClosed 双 FAIL）；其余 8 轮全绿。api 隔离 10+ 轮 0 FAIL；zhidao 11 轮 0 FAIL（R1 为初始后台轮，R11 收官轮同样全绿，见实证表）。
- **结论：R56 收尾"14% flake"未稳定；全量连续多轮是唯一可信判定源（R56 教训 1 仍成立）**。

### C. api 包 `TestSetTargetsBounds`（R56 补跑第 6 轮 FAIL）
- 单跑/连跑 5 次全绿、隔离整包跑全绿。R1 全量轮 FAIL 时断言行"目标应被拒绝"收到非预期响应——**冷启动 connectex 传递路径**（登录/探测连接失败 → 响应非预期 JSON），非断言脆弱。裁决：**冷启动 flake，非断言脆弱**。

### D. httpDo 重试路径在真实 doRequest 下的行为
- **独立程序实证**：真实路径 read 错误重试 body 空 + ContentLength 恒 13 → 发送端直接拒绝（畸形 POST），重试从未到服务端；transport 内部已用 GetBody 把"复用连接 + nothingWritten"场景重放掉了（该场景 body 完整）；若人为用 GetBody 重放（修复生效）→ 双报。**幂等防线不完备**（详见 CRITICAL-57-01）。

### E. 上轮观察项延续复核 —— 全表见延续表，8 项延续、2 项闭合、1 项部分闭合、1 项升级 CRITICAL
- task_log 窗口 SQL / append 覆盖 / 空表语义 / scheduler 身份防线 / WindowClosed 三判据 / api 鉴权家族 / 登录链路 / 凭据加密 / 数据库迁移——全部复核正确。

### F. 全包逐行通读找新问题 —— 见 CRITICAL/MAJOR/MINOR/OBSERVE 列表
- 重点核对：scheduler 提交链（spawnChain 六分支 sameClientFor 身份复核、inflight 去重、B30-01 链顶双判、C-4 锁外复核、B23-01 doneHas 保护）✅；WindowClosed 三判据单源 + 10s 裕量双侧 ✅；task_log 窗口 SQL（空表/1 行语义）✅；api 鉴权族（401/403/404/429/500 家族 + requireJSONBody CSRF 门）✅；登录链路（RSA/PKCS1v15/priorityId/Vision 收敛/gateTryAcquire）✅；凭据 AES-256-GCM + master_key 32 字节校验 ✅；数据库迁移幂等 ✅；httpDo/cloneReq 重试路径 → **CRITICAL-57-01**。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员）+ maybeRelogin 决策/写回双闭合 + waitChainExit 等待契约；6+1 个同名重建测试全绿。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败（带开放时间已过）+ 幽灵窗口（带 10s 裕量）；StateForAccount/WindowClosed/admin stats 三路共用。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生普通会话；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 双重校验；登录/激活独立限流桶；XFF 仅回环+开关。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；refuseLegacy 缺列清单与已迁移列剔除对应；窗口 SQL 语义实证正确。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 中点近似 + 成功才推进 + 失败 30s 退避 + streak≥3 复位（B21-01 不回零语义）。
- **数据库迁移规范**：migrateAddPublishMeta 幂等；旧库缺纯新增列自动迁移、缺语义列拒绝启动。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽（除测试注释残留）。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | 双通道 exit 0 |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` 11 轮 | **R1 api FAIL**（TestSetTargetsBounds）/ **R3 api FAIL**（TestLoginUnactivatedNeedsCode）/ **R8 api FAIL**（TestAdminStatsOpenTimeFromRecognized + TestElectiveSelectRejectsWindowClosed）；其余 8 轮全绿（8/11，~73%）；R11 收官轮全绿 |
| 全量各包耗时（绿轮范围） | accounts 1.4-3.4s / api 25-71s / config 1.4-3.8s / db 2.4-6.9s / runtime 1.3-7.8s / scheduler 14.7-20.3s / secure 1.3-6.5s / session 1.5-5.1s / store 44-202s / zhidao 3.2-16.7s |
| zhidao 包全量 11 轮 | **0 FAIL**（TestMain preheat 生效） |
| api 包隔离 | 10+ 轮 0 FAIL（登录族/stats/window 测试对/TestSetTargetsBounds 组合） |
| TestSetTargetsBounds 单跑/连跑 | 单跑 PASS / 连跑 5 次 PASS / R1 全量轮 FAIL（冷启动 connectex） |
| TestLoginUnactivatedNeedsCode 单跑 | 8 轮全 PASS |
| TestAdminStatsOpenTimeFromRecognized + TestElectiveSelectRejectsWindowClosed | 连跑 5 次全 PASS / R8 全量轮双 FAIL |
| zhidao 日志测试（TestLoginLogsAttempts/FailureSummary） | 隔离 5 轮全 PASS / 3 用例组合 1 次 FAIL（TestMain Discard 竞态） |
| GetBody 标准库核证 | request.go:932-945：NewRequest 对 bytes.Reader/strings.Reader 自动设 GetBody |
| 真实路径 read 错误重试实证 | GetBody=true → cloneReq 分支恒不进 → 重试 `ContentLength=13 with Body length 0` 失败 |
| RST（SO_LINGER=0）场景 | read OpError → httpDo 重试 → 空 body 失败，服务端 1 次请求 |
| "若修复生效"后果实证 | GetBody 重放完整 body → 服务端 2 次报名请求 → 200 OK（双报） |
| gofmt 检查 | `gofmt -l` 报 zhidao/client_test.go（import 多 tab + 无尾换行） |
| 测试函数总数 | 208 个 |
| 审查期间工作树 | 与基线一致：5 个社区文档 + round57-frontend-findings.md 未跟踪 |

---

## 结论

- **CRITICAL 1（cloneReq 修复死代码 + 重试双报风险）/ MAJOR 1（api flake 残余 3/11）/ MINOR 1（TestMain Discard + gofmt）/ OBSERVE 7 新增 + 延续 9 项**。产品逻辑除 httpDo read 错误重试语义外零缺陷。
- **flake 统计结论**：全量 11 轮 **3/11 FAIL（~27%）全为 api 冷启动 connectex 残余**（R11 收官轮全绿），zhidao 11 轮归零（TestMain 生效），R56 收尾"14%"未稳定——全量连续多轮为唯一可信判定源（R56 教训 1 继续成立）。
- **最致命 3 条（按影响排序）**：
  1. **CRITICAL-57-01**：R56 cloneReq GetBody 修复对真实 doRequest 路径是死代码（标准库已设 GetBody），read 类错误重试仍空 body；若改成 GetBody 重放则 SelectClass/ExitClass **双报**——需删除 read 错误重试或仅保留 dial/write，并修正误导注释。
  2. **MAJOR-57-01**：api 包全量轮 flake 3/11 未归零，4 个不同测试样本，隔离全绿——全量连续多轮统计，readyProbe 再加固或接受 CI `||` 重跑。
  3. **MINOR-57-01**：zhidao TestMain `log.SetOutput(io.Discard)` 破坏日志测试完整性（偶发竞态 FAIL）+ gofmt 违规（import 缩进/尾换行）。
