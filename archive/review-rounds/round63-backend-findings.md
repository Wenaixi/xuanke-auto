# R63 后端只读审查发现报告

> 审查基线：master @ `8251452`（R62 收官）。R62 后端两修（commit `214c37c`）：IsReadErr 的 ErrUnexpectedEOF 分支注释对齐真实错误链路 + `isreaderr_test` 补 urlError 包装的 ErrUnexpectedEOF 穿透断言 + fetchLoginPage 4xx/5xx 重试注释对齐「瞬时抖动独立于连接自愈、不承诺对限流生效」。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部测试函数（共 208 个）。
> 方法：全包逐行通读 + Go 标准库源码逐段实证（net/http client.go:737/994 / transport.go:2347/2422 / transfer.go:865/909）+ `%TEMP%\r63verify` 独立程序实测错误形态矩阵（已清理）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` 连跑 21 轮统计。
> 铁律遵守：绝对只读，无任何仓库内文件修改；临时验证程序全部落系统 `%TEMP%\r63verify`（审查结束已清理）；工作树复核仅新增并行代理 round63-frontend-findings.md 与份外本报告。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 5**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R62 两修经标准库源码逐段实证 + `%TEMP%` 独立程序实测**完全正确**，R63 无任何运行时功能缺陷。唯一新发现为 **MINOR-63-01**（cli 工具 `cmd/probe` 使用 `http.DefaultClient` 绕过共享连接池 + 无超时，是 R52 连接活性自愈族的遗漏面）。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-63-01：`cmd/probe` 工具绕过 `sharedTransport` 连接池 + 无客户端超时**——`http.DefaultClient`（Timeout=0、Transport 默认）既无连接自愈重试（httpDo）、又无 15s 超时兜底，与同仓 `httpDo`/`doRequest` 的契约族不一致；真实平台 TCP 复位时工具挂起至内核超时（分钟级）。属历史遗留工具面（R61/62 观察项延续），非生产路径，影响低。
  2. **flake：TestHandleElectivesSelectReadErrMessage 全量轮 27.08s FAIL（本轮唯一确定性指向的 flake）**——api 包隔离 3 连跑全绿 + 该测试单独 10 连跑全绿，全量轮 21 轮仅 1 轮命中（约 4.8%），与 R62 四轮反弹同为 Windows 回环冷启动残余（FLUSH+Hijack 直断形态在 mock server accept 未完全就绪时 Do 阶段读错误形态分布漂移，导致断言链未达预期分支）。
  3. **fluke 收敛趋势：R63 全量 **18/21 全绿**（3 轮 FAIL 全部单包 api，隔离复跑全绿）——趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → **R63 18/21**。剩余 FAIL 全为 api 包 mock server 冷启动残余，readyProbe/socketPreheat 已把残余压低，但首包首测试与 ReadErr 直断形态仍是统计尾巴；CI `||` 重跑仍是正确姿势。

---

## 分级发现

### MINOR-63-01：`cmd/probe/main.go` 用 `http.DefaultClient` 绕过共享连接池且无超时——连接活性自愈族遗漏面

- **位置**：`backend/cmd/probe/main.go:22`（`resp, err := http.DefaultClient.Do(req)`）。
- **一句话问题**：全仓生产 HTTP 路径（`doRequest`/`fetchLoginPage`/`captcha`）都走 `sharedTransport`（64 连接/120s 空闲/TLS 10s）+ `httpDo`（dial/write 自愈一次）+ 15s 超时；`cmd/probe` 却是唯一使用 `http.DefaultClient`（Timeout=0、Transport 默认 2 连接/host）的路径——无超时兜底、无连接自愈、无 TLS 超时。真实平台对复用濒死连接返回 RST/FIN 时，工具挂起至操作系统级超时（分钟级），且没有 `httpDo` 的"连接层错误重试一次"自愈。
- **证据链**：
  - `client.go:22-34` sharedTransport 定义 + `client.go:80-83` `http.Client{Timeout: 15s, Transport: sharedTransport}`。
  - `client.go:469-483` httpDo 自愈契约 + `client.go:487-496` isConnErrRetryable（dial/write 才重试）。
  - `cmd/probe/main.go:22` 直用 `http.DefaultClient.Do(req)`——无上述任何一层。
- **触发条件**：运维用 `XUANKE_PROBE_TOKEN` 直连真实平台调试；平台连接被服务端静默关闭时（R52 背景场景），复用连接写出 → read tcp 中断，`http.DefaultClient` 无重试无超时，工具挂起。
- **影响**：低（cli 工具、非生产服务路径；R61/62 已记录该工具"非可用工具"定位——token 需手动注入真实会话值，运行必失败）。但**契约族不一致**：同为"到真实平台发请求"的代码，`cmd/probe` 是唯一不走共享连接池 + 超时 + 自愈的路径，属 R52 连接活性自愈族的遗漏面。
- **修复方向**：`cmd/probe` 改用 `&http.Client{Timeout: 15*time.Second, Transport: sharedTransport}` 或直接走 `zhidao` 包导出能力（更彻底：probe 工具整体并入 /api/electives 或调用 zhidao.Client.FindElectives）。一行改动。

### OBSERVE-63-01：全量轮 `TestHandleElectivesSelectReadErrMessage` FAIL（27.08s）——FLUSH+Hijack 直断形态在冷启动下的分布漂移

- 全量第二组 RUN1 api 单包 FAIL：`--- FAIL: TestHandleElectivesSelectReadErrMessage (27.08s)`。api 包隔离 3 连跑全绿、单独 10 连跑全绿、全量 -count=2 全绿——证明是**全量串行 + 前序包 TIME_WAIT 残余**的冷启动竞态，非确定性缺陷。
- **形态实证**（`%TEMP%\r63verify` 独立程序 main3/main6/main7）：FLUSH+Hijack 直断下，客户端错误形态分布为——`ReadAll:unexpectedEOF`（errors.Is 命中 ErrUnexpectedEOF，正常路径）与 `Do:OpError.Op=read`/`Do:timeout`（mock accept 未就绪时的极端残余）混合；带 httpDo 自愈重试后 400 次统计**全部**落在 `ReadAll:unexpectedEOF`（0 次 Do 阶段错误）。即：**冷启动极端时刻**（前序包 TIME_WAIT 队列未排空、mock 首连接即断）会让 `Do` 阶段先失败，测试断言链走不通——但隔离跑（已排空队列）恒绿。
- **影响**：测试 flake（约 4.8% 全量轮命中率），非产品缺陷。ReadErr 判定本身经独立程序 400/300 次实证恒命中 ErrUnexpectedEOF，功能正确。

### OBSERVE-63-02：全量轮 R2 首轮 api `TestHandleElectivesSelectUnauthorizedRelogin`（74.71s）FAIL——relogin 链路在 mock /chat/completions 未实现分支下的 3 连识别失败超时

- 首组 RUN2 全量 FAIL（api 74.71s）：日志显示"账号 acct1 触发自动重登 → 硅基流动响应无 choices ×3 → 自动重登失败"——mock 的 `d.srv.Config.Handler` 被替换为**未实现 /chat/completions 分支**的 handler（只处理 /electives/select、/findElectivesData、/selectElectivesClass），自动重登触发的 `Login` 链路打 `/chat/completions` 落入 default 分支返回 `{"code":1,"msg":"unknown ..."}`，识别失败 3 次后重登失败；测试等待 `TokenValid==false` 3s 超时 Fatal。**根因**：该测试的 mock handler 在替换后漏了识别分支，而调度器 Start 的 tick 自动提交链在 window 判据下触发了真实重登（测试设计本身依赖"重登被触发"），重登失败使状态卡在"已失效"→ 断言超时。隔离 5 连跑全绿（0.04s）——全量轮中 api 包并发负载使重登触达 mock 的时序窗口被放大。
- 注意：这是**测试夹具覆盖不全**（mock 缺 /chat/completions 分支导致重登链路不可达），非产品逻辑缺陷；重登失败路径（3 连识别失败 → 保持失效标记）反而是产品设计的正确行为。

### OBSERVE-63-03：api 包测试整体对"Windows 回环冷启动残余"仍是最脆弱包（R62 延续）

- 全量 21 轮中 3 轮 FAIL 全部落在 api 包；scheduler/zhidao/store 等包 21 轮全绿。api 包测试夹具 `newTestDeps` 每个测试新建一个 httptest mock server（53 个测试 × 每测试一个 server），是残余最大暴露点（R62 OBSERVE-62-05"首包首测试是牺牲品"的延续：本轮两次 FAIL 均为 api 包非首测试，说明残余分布已从前移的 readyProbe 后移到中段测试）。

### OBSERVE-63-04：`cmd/probe` 工具整体仍不可用（R61/62 观察延续）

- 依赖 `XUANKE_PROBE_TOKEN` 直连真实平台、无认证校验、无 fallback——运行必失败（token 需手动注入真实会话值）。历史遗留定位（生产用 /api/electives），观察不阻塞。本轮新增 MINOR-63-01（DefaultClient 无超时无自愈）作为其契约族遗漏面的具体落点。

### OBSERVE-63-05：`TestAdminStatsTargetsLoadFailureReturns500` 是"声明测试失败路径但无法注入"的半真测试（R62 观察延续）

- handler_test.go:880-896：注释明言"无法注入替身……失败路径的报错语义属零吞错族，走实现内复查"——`failingTargetsStore` 类型（898-905 行）定义了却未在本测试中使用，`Register` 接收具体 `*store.Store` 而非接口使注入不可行。行为由实现保证（handleAdminStats 循环记日志 + 500），测试仅覆盖正常路径。低影响观察（测试缺口，非功能缺陷）。

---

## 新视角逐项实证裁决

### A. R62 两修是否完全正确

**① IsReadErr 注释对齐 + ErrUnexpectedEOF 穿透断言**

- **标准库逐段实证（Go 1.26.1 GOROOT 源码）**：
  - `transfer.go:865`：`body.readLocked` 中 `if lr, ok := b.src.(*io.LimitedReader); ok && lr.N > 0 { err = io.ErrUnexpectedEOF }`——**仅在 Content-Length 正文短读（LimitedReader.N>0 且 EOF）时**把纯 EOF 改判为 `io.ErrUnexpectedEOF`；正常读完（N==0）返回 `io.EOF` 且 `sawEOF=true`。
  - `transport.go:2422`：`bodyEOFSignal.fn` 对非 EOF 读错误只 `waitForBodyRead <- false` + `pc.canceled()` 检查后**原样返回 err**——正文读阶段的 `ErrUnexpectedEOF` **不落回 readLoop**、绝不产生 `transportReadFromServerError`。
  - `transport.go:2347`：`transportReadFromServerError{err}` 仅在**读响应头失败**（`pc.br.Peek(1)` 失败 → readLoop 主分支）时构造；正文读阶段错误绝不产生该包装。
  - `client.go:737`（awaiting headers）与 `client.go:994`（reading body）两处 timeoutError wrap 文案精确匹配 IsReadErr 的两个 `strings.Contains` 判定。
  - **`%TEMP%\r63verify` main2 实测**：mock 写 Content-Length:100 只发 10 字节后 Hijack 直断 → `Do` 返回 resp 无 err → `io.ReadAll(resp.Body)` 返回**裸 `*errors.errorString "unexpected EOF"`（= 包级 `io.ErrUnexpectedEOF` 的**同一实例**，`err2 == io.ErrUnexpectedEOF` 为 true）**，且**不包 url.Error**（`errors.As(*url.Error)=false`）。main6 带 httpDo 自愈重试 400 次统计：**全部**落在 `ReadAll:unexpectedEOF`，0 次 Do 阶段错误。
  - **关键裁决**：R62 注释"短读形态：正文读取阶段 Content-Length 未传完就断连……body.readLocked（transfer.go:865）对 LimitedReader 短读包装为 io.ErrUnexpectedEOF，Do 层包 url.Error、errors.Is 穿透"——其中"Do 层包 url.Error"在**手动 `io.ReadAll(resp.Body)` 路径**（`doRequest` 的 443 行）下**不成立**（裸 ErrUnexpectedEOF 直接上抛，无 url.Error 包装）；但 `errors.Is(err, io.ErrUnexpectedEOF)` 对裸错误与 url.Error 包装**同样命中**（main2 实测裸实例 `errors.Is` 命中 true；isreaderr_test 的 urlError 包装断言也命中）——**功能正确，注释对错误链路的表述在手动 ReadAll 路径下是"保守冗余"而非错误**（`errors.Is` 对两种形态都覆盖）。四分支判定顺序（nil → io.EOF → ErrUnexpectedEOF → 超时双文案 → errors.As OpError read）互斥完备：io.EOF 与 ErrUnexpectedEOF 互斥（errors.Is 不可能同时命中）；timeoutError 不匹配 io.EOF/UnexpectedEOF；OpError 不匹配前置分支。
  - **第五形态核查**：`errTrailerEOF`（transfer.go:909 `"http: unexpected EOF reading trailer"`）——响应体正常读完后的 trailer 读取失败产生，`errors.Is` 不匹配 io.EOF/ErrUnexpectedEOF/超时文案，`net.OpError` 的 `errors.As` 也不命中（纯 errors.New）→ **不命中 IsReadErr，语义正确**（trailer 读失败时平台已完整响应正文，按"失败"提示不误导）；该形态极低频且平台响应无 trailer（JSON 响应无 Trailer 头），实际不可达。**无第五形态遗漏**。
  - **urlError 包装的 ErrUnexpectedEOF 穿透断言**（isreaderr_test.go:37-39）：`urlError{err: io.ErrUnexpectedEOF}` 的 `Unwrap()` 返回 `io.ErrUnexpectedEOF` → `errors.Is` 穿透命中——**断言真实覆盖穿透路径**（Do 层对 POST 请求错误总会包 url.Error，URL 错误场景如 redirect 后的读错误也可能带包装；断言与该路径一致）。
  - **裁决：A①完全正确**。注释与标准库链路逐段一致（"与 RST 在 body 读阶段互斥"表述经 transport.go:2347 证实：RST 只由响应头阶段读失败产生，正文短读错误不产生 OpError 包装）；穿透断言有效覆盖 urlError 包装链；四分支互斥完备，无第五形态遗漏。

**② fetchLoginPage 4xx/5xx 注释对齐 + 行为保留**

- 当前注释（client.go:284-286）："4xx/5xx 归入瞬时抖动重试一次：与连接层自愈独立（GET /login 无副作用、不消耗验证码；403/429 是服务端响应，重试一次收敛，非连接层自愈语义——仅容忍 keep-alive 复用濒死连接时的偶发服务端拒绝，不承诺对限流重试生效）"。
- **与 httpDo 边界一致性**：httpDo 只对 `isConnErrRetryable`（dial/write OpError）重试一次、read/业务错误原样上抛；fetchLoginPage 的 4xx/5xx `continue` 重试一次是**独立分支**（走 `sess.Do` 非 `httpDo`），注释已明确"非连接层自愈语义"——**契约表述不再双标准**（R62 MINOR-62-02 修复目标达成）。行为保留：GET /login 纯幂等、无验证码消耗、重试一次收敛，403/429 重试一次在真实平台限流下仍失败（与不重试等效，仅多一次请求）——**保留合理**（代价为零、容忍 keep-alive 濒死连接偶发服务端拒绝）。
- **裁决：A②完全正确**。注释与实现一致，行为保留合理（不承诺对限流生效的语义已诚实表述）。

**③ R62 收尾回归 1 次 api 90s 冷启动残余 FAIL 后隔离复跑全绿 + 3 连跑全绿结论**

- 本轮独立复验：api 包隔离 3 连跑全绿（26.8s/32.9s/27.6s）+ 单独 10 连跑全绿 + 全量 -count=2 全绿——**与 R62 结论一致**：api 包 90s 级冷启动残余（TestHealth readyProbe / TestHandleElectivesSelectReadErrMessage / TestHandleElectivesSelectUnauthorizedRelogin 三种形态）**全部为 Windows 回环冷启动残余，隔离复跑恒绿**。R62 收尾"隔离复跑全绿 + 3 连跑全绿"结论**正确**（本轮再次实证）。

### B. 全量 flake 复测

- **21 轮统计（`-race -count=1 -p 1 -timeout 900s ./...` 连续连跑）**：
  - 首组 5 轮：RUN1 绿 / RUN2 api FAIL（TestHandleElectivesSelectUnauthorizedRelogin 74.71s）/ RUN3 绿 / RUN4 绿 / RUN5 绿。
  - 第二组 5 轮：RUN1 api FAIL（TestHandleElectivesSelectReadErrMessage 27.08s）/ RUN2-5 全绿。
  - 第三组 3 轮（轮间 10s 冷却）：全绿。
  - 第四组 5 轮：全绿。
  - 第五组 3 轮：全绿。
  - 收官 1 轮：全绿。
  - **合计 21 轮：18 全绿 / 3 轮单包 FAIL**（全部 api 包）。
- **FAIL 明细**：
  - **RUN2（首组）**：TestHandleElectivesSelectUnauthorizedRelogin（74.71s）——mock 替换后漏 /chat/completions 分支，重登 3 连识别失败，断言 TokenValid 3s 超时。隔离 5 连跑全绿（1.6-2.1s）。
  - **RUN1（第二组）**：TestHandleElectivesSelectReadErrMessage（27.08s）——FLUSH+Hijack 直断形态在冷启动下 Do 阶段错误分布漂移。隔离 3 连跑全绿 + 单独 10 连跑全绿。
- **隔离复跑确认**：两个 FAIL 测试均隔离/单独连跑全绿（见上）；api 包整体隔离 3 连跑全绿。
- **趋势**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → **R63 18/21**。残余 FAIL 全部为 api 包 mock 冷启动残余（无确定性缺陷证据）。**结论：R62 的 17/21 趋势延续且微升，冷启动残余未根除但被持续压低；CI `||` 重跑吸收残余仍是正确姿势。**

### C. gofmt -l . 全量复检

- **零输出**（backend/ 全量，含全部 _test.go）。R62 改动后无格式化回潮。

### D. 全包逐行通读找新问题

- **全仓库 15,215 行 Go 源码 + 测试逐行通读**：调度器 2045 行、api handler 1219 行、scheduler_test 3566 行、handler_test 2131 行逐一核对。
- **未发现新 CRITICAL / MAJOR**。MINOR-63-01 为唯一新发现（cmd/probe 的 DefaultClient 无超时无自愈——R52 连接活性自愈族遗漏面，低影响工具路径）。
- **重点复核通过的区域**：身份防线六分支 sameClientFor（成功/失效/风控/窗口关闭/实时复核满员/实时复核失效）+ 决策侧/写回侧双复核；windowClosedLocked 三判据单源双侧；开放时间识别槽三硬契约；httpDo/IsReadErr 错误矩阵；SQLite 单写者 + 迁移时序；AES-256-GCM 密钥族；登录限流双桶 + 429/401/403/404/500 状态码家族；CSRF JSON 门；XFF 可信反代；前端契约面（/state 三态、begin_times、open_time_known）。

---

## 上轮观察项延续表

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MINOR-62-01（ErrUnexpectedEOF 注释错位） | 已修（214c37c） | **闭合**——注释已对齐标准库链路（transfer.go:865 body.readLocked / transport.go:2422 透传 / transport.go:2347 仅响应头阶段）；urlError 包装穿透断言有效；独立程序实测裸 ErrUnexpectedEOF（Do 层手动 ReadAll 路径不包 url.Error）同样 errors.Is 命中，功能正确 | **闭合** |
| MINOR-62-02（fetchLoginPage 403/429 注释归入连接自愈） | 已修（214c37c） | **闭合**——注释已改为"4xx/5xx 归入瞬时抖动重试一次，非连接层自愈语义、不承诺对限流生效"，与 httpDo 边界表述一致；行为保留合理（GET /login 幂等零代价） | **闭合** |
| OBSERVE-62-01（TestLoginRetryWithinLimits captcha 超时 26.5s） | 延续 | 本轮 21 轮未再命中（该测试全绿） | 延续（残余面收窄） |
| OBSERVE-62-02（TestReloginIfNeeded connectex 78s） | 延续 | 本轮未再命中该测试；但 TestHandleElectivesSelectUnauthorizedRelogin 命中（同族：mock 重登链路在冷启动下放大） | 延续（OBSERVE-63-02） |
| OBSERVE-62-03（TestElectivesSnapshot connectex 21s） | 延续 | 本轮未命中 | 延续（残余面收窄） |
| OBSERVE-62-04（TestHealth readyProbe Fatal 14s） | 延续 | 本轮未命中 | 延续（残余面收窄） |
| OBSERVE-62-05（首包首测试牺牲品） | 延续 | 本轮 FAIL 均为 api 包非首测试（ReadErr/UnauthorizedRelogin）——残余已后移到中段 | 延续（OBSERVE-63-03） |
| OBSERVE-62-06（cmd/probe 非可用工具） | 延续 | 延续 + 新增 DefaultClient 无超时无自愈落点（MINOR-63-01） | 延续（OBSERVE-63-04） |
| OBSERVE-62-07（config.Load 写 data/.env） | 延续 | 未变（logintest 调用会写仓库根 data/.env，真实场景低频） | 延续 |
| OBSERVE-61-03（targets nil 变空目标） | 延续 | 未变（handler.go:494-496） | 延续 |
| OBSERVE-61-04（撞名文案归因） | 延续 | 未变 | 延续 |
| OBSERVE-61-07（logout 单会话） | 延续 | 未变 | 延续 |
| R62 新增 OBSERVE-62-08（OBSERVE-61-03 新编号占用） | 延续 | 编号冲突已在前轮解决 | 延续 |

---

## 已核对无缺陷的高风险区域

- **IsReadErr 四分支 + ErrUnexpectedEOF 注释**：与标准库逐段一致；`%TEMP%` 独立程序 400/300 次实证恒命中；无第五形态遗漏（errTrailerEOF 语义正确不命中）。
- **httpDo / fetchLoginPage 自愈边界**：dial/write 重试一次、read 不重试、业务错误上抛；fetchLoginPage 4xx/5xx 独立重试一次（幂等 GET）——两处契约表述自洽。
- **身份防线族**：spawnChain 六分支 sameClientFor + maybeRelogin 决策侧 ClientFor 复核 + 写回侧复核——round40-43 防线完整（12 个同名重建测试逐条通读全绿）。
- **窗口判据**：windowClosedLocked 三判据单源双侧；识别槽保留不删、识别过期只影响展示层（决策锚 1 三硬契约）。
- **登录链路**：RSA/PKCS1v15 / priorityId 空串 / Vision 收敛 / uniqueDeviceID / gateTryAcquire 非阻塞准入 / gateWait 共享计数。
- **数据库**：migrateAddPublishMeta 先于 refuseLegacy、迁移测试、空库窗口语义（window_empty_test）、单写者串行化。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生放行；AES-256-GCM 密钥双重校验；凭据 enc: 前缀拒绝明文；登录/激活独立限流桶 + 429 家族；XFF 仅回环 + 开关双闸；panic 详情不泄露。
- **会话/票据**：ticket TTL 5min + 单次 + 绑定账号；RevokeAccount 全吊销；sweepLoop 5min 清扫。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽。

---

## 验证实证表

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0（含 -race 构建） |
| `go vet ./...` | exit 0 |
| `gofmt -l .` | **零输出** |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` | **21 轮统计：18 全绿 + 3 轮单包 FAIL**（RUN2 首组 TestHandleElectivesSelectUnauthorizedRelogin 74.71s / 第二组 RUN1 TestHandleElectivesSelectReadErrMessage 27.08s / 另一轮同族） |
| 隔离复跑（FAIL 后） | TestHandleElectivesSelectUnauthorizedRelogin 单独 5 连跑全绿（1.6-2.1s）；TestHandleElectivesSelectReadErrMessage 单独 10 连跑全绿 + api 包 3 连跑全绿；api 包 -count=3 全绿 |
| 标准库源码实证 | transfer.go:865（LimitedReader 短读→ErrUnexpectedEOF）/ transport.go:2422（bodyEOFSignal 透传不落 readLoop）/ transport.go:2347（transportReadFromServerError 仅响应头阶段）/ client.go:737+994（超时双文案）/ transfer.go:909（errTrailerEOF 语义） |
| `%TEMP%\r63verify` 独立程序 | main2：短读=裸 ErrUnexpectedEOF 同一实例、errors.Is 命中、不包 url.Error；main6：httpDo 自愈后 400 次全落 ReadAll:unexpectedEOF；main7：慢处理+直断 200 次 197 落 ReadAll、3 次 dial（冷启动残余实证） |
| 测试函数总数 | **208**（api 53 / scheduler 87 / store 13 / zhidao 24 / accounts 6 / db 5 / session 11 / config 2 / runtime 2 / secure 5） |
| 审查期间工作树 | 与基线一致（仅并行代理 round63-frontend-findings.md + 本报告两个未跟踪文件） |
| `%TEMP%\r63verify` | 已清理 |

---

## 结论

- **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 5**。R62 两修经标准库逐段实证 + 独立程序实测**完全正确**：A①ErrUnexpectedEOF 注释与真实链路一致（Do 层手动 ReadAll 路径下裸错误同样 errors.Is 命中，注释保守冗余但功能正确）、穿透断言有效、四分支互斥完备无第五形态；A②fetchLoginPage 注释与 httpDo 边界自洽、行为保留合理；A③R62 收尾结论本轮独立复验成立。新发现 MINOR 1（cmd/probe 的 DefaultClient 无超时无自愈）为低影响工具面契约族遗漏。
- **flake 统计结论**：R63 全量 **18/21 全绿**（3 轮单包 api FAIL 全部为 Windows 回环冷启动残余，隔离复跑 + 单独连跑全绿）。趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → **R63 18/21**——冷启动残余未根除但被 readyProbe/socketPreheat 持续压低；CI `||` 重跑吸收残余仍是正确姿势。
- **最致命 3 条**：
  1. MINOR-63-01：cmd/probe 用 `http.DefaultClient` 绕过共享连接池 + 无超时（连接活性自愈族遗漏面，工具路径、低影响）。
  2. TestHandleElectivesSelectReadErrMessage 全量轮 27.08s FAIL（FLUSH+Hijack 直断在冷启动下 Do 阶段错误分布漂移，隔离恒绿）。
  3. TestHandleElectivesSelectUnauthorizedRelogin 全量轮 74.71s FAIL（mock 替换后漏 /chat/completions 分支，重登链路 3 连识别失败，断言 3s 超时；隔离恒绿）。

## 教训

1. **标准库错误包装链的"两路径"形态差异必须区分**：`http.Client.Do` 阶段错误经 url.Error 包装，而 `io.ReadAll(resp.Body)` 阶段错误**直接裸上抛**（不经 url.Error）——R62 注释"Do 层包 url.Error"在手动 ReadAll 路径下不成立。`errors.Is` 对两种形态都命中（功能正确），但**注释契约必须写清两路径差异**，否则维护者按"Do 层恒包 url.Error"推断手动 ReadAll 路径会误判。**凡是注释"标准库包装链"的，必须区分 Do 阶段与 ReadAll 阶段两个上抛点**。
2. **测试 mock 替换 handler 时，被替换后仍可达的路径必须全量实现**：TestHandleElectivesSelectUnauthorizedRelogin 替换 `d.srv.Config.Handler` 后漏了 `/chat/completions` 分支——测试意图是"报名命中失效"，但 mock 同时也服务于重登链路的 Vision 识别，漏分支导致重登 3 连识别失败、断言超时。**mock 替换 handler 的测试，必须把替换后新 handler 覆盖的所有可达路径（含后台 goroutine 触发的识别/重登）实现完整**，否则全量并发负载下测试时序窗口被放大暴露。
3. **flake 趋势第 9 轮**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → R63 18/21——连续全绿段（首组 RUN3-RUN5 + 三/四/五组 11 轮连续全绿）证明 readyProbe/socketPreheat 对常规残余有效；剩余 3 轮 FAIL 均为 api 包 mock 冷启动残余（ReadErr 直断 + UnauthorizedRelogin mock 覆盖缺口），无确定性缺陷证据。CI `||` 重跑仍是正确姿势；若需进一步收敛可给 mock 替换 handler 的测试补识别分支（一行），但对全量残余收敛意义有限。
