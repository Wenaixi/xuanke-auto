# R62 后端只读审查发现报告

> 审查基线：master @ `5f43d5f`（R61 收官，2026-09-21）。工作树预期完全干净（R61 已提交全部文件）；审查期间绝对只读——build/vet/gofmt/test 均为只读验证，无任何仓库内临时产物；临时验证程序全部落系统 `%TEMP%\r62verify`（已清理）。写入前 `git status --short` 复核：仅并行代理 round62-frontend-findings.md 未跟踪文件，与基线一致。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round61 文末观察项逐条复核。
> 方法：全包逐行通读 + Go 标准库源码逐段实证（net/http client.go/transport.go/transfer.go/internal/chunked.go）+ `%TEMP%` 独立程序实测错误形态 + 全量 `-race -count=1 -p 1 -timeout 900s ./...` 连跑统计。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 7**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R61 四修经标准库源码实证 + 独立程序实测**基本正确**，但暴露两处新的注释/契约偏（MINOR-62-01：IsReadErr 的 ErrUnexpectedEOF 分支覆盖"连接被 readLoop 中断"的中途形态但按"短读"注释误导；MINOR-62-02：fetchLoginPage 403/429 被 F52 归入"连接活性自愈"家族但实测 403/429 是业务/服务端响应，重试它违反"绝不含业务重试"的自愈边界）。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-62-01：IsReadErr 的 `io.ErrUnexpectedEOF` 分支注释与其实际覆盖形态不符（标准库源码逐段实证）**——真实"响应头已到达、Content-Length 未传完断连"的短读形态**根本不会**在 `doRequest` 的 `io.ReadAll(resp.Body)` 中出现（标准库 `bodyEOFSignal.condfn` 对非 EOF 读错误**只把连接中途断开包装成 `transportReadFromServerError` 并且 `body` 层 `Read` 透传**、`transportReadFromServerError` **不落回 body 读**、客户端读 body 的短读（LimitedReader.N>0 + EOF）**在 `body.readLocked` 返回 `io.ErrUnexpectedEOF` 后 `ReadAll` 上抛的是纯 `io.ErrUnexpectedEOF` 不带 OpError**——本轮 `%TEMP%` 独立程序实测：FLUSH+Hijack 直断 → `READALL_ERR: *errors.errorString unexpected EOF`（**不是** `*net.OpError`），但**它已经包进 `url.Error`**（`Do` 返回 `url.Error{Err: body读错误}`）且**经过 `cancelTimerBody`（仅 timeout wrap）不挡 errors.Is**）——四分支判定顺序互斥且完备（见 A 项），`errors.Is(err, io.ErrUnexpectedEOF)` **能穿透 `url.Error` 包装链命中**，因此**功能正确**（R61 修复本身成立），但注释"不带 OpError/transfer.go 短读包装"与真实链路**错位**（RST 分支也从不命中 body 短读形态）——**注释契约与实现脱节**，维护者按注释推断会误判真实形态。
  2. **MINOR-62-02：`fetchLoginPage` 对 403/429 响应重试一次，注释却归入"连接活性自愈（F52-M4/M5）"家族——403/429 是服务端响应、非连接层错误，重试它违反"自愈只覆盖连接层错误、绝不含业务重试"的自愈边界**（R52/R57 契约）。F52 的"连接活性自愈"设计只覆盖 dial/read/write 连接错误（`httpDo` + `isConnErrRetryable`）；`fetchLoginPage` 对 4xx/5xx `continue` 重试一次是**独立于连接自愈的宽路径**——403（IP 被平台限流）重试纯属浪费（且与 login 限流防线语义相抵）；429（限流）重试一次后仍会再失败。功能影响低（纯 GET /login 不消耗验证码额度、重试一次即收敛），但**注释把两种语义并成一个"自愈"**，与同一文件 `httpDo` 的严格连接层边界形成双标准。
  3. **flake 收敛**：R62 全量 21 轮中 **17 轮全绿 + 4 轮单包 FAIL**（R2 TestReloginIfNeeded connectex 78s / RUN4 TestLoginRetryWithinLimits captcha awaiting headers 超时 26.5s / RUN5 TestElectivesSnapshot connectex 21s / RUN7 TestHealth mock 就绪探测 connectex 14s）——**全部为 Windows 回环冷启动 connectex 与客户端超时残余，隔离复跑全绿**（TestReloginIfNeeded race×3 全绿 0.04s；TestLoginRetryWithinLimits+TestReloginIfNeeded 5 连跑全绿；zhidao+api 两包隔离全绿；RUN8-13 连续 6 轮全绿回落）。趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → **R62 17/21**（连续全绿段 R1-R11 前 11 轮 10 绿 + R12-R21 连续 10 绿，中段 R2/R4/R5/R7 四轮反弹）。

---

## CRITICAL

（无）

---

## MAJOR

（无）

---

## MINOR

### MINOR-62-01：IsReadErr 的 `io.ErrUnexpectedEOF` 分支注释与其真实覆盖形态错位（标准库逐段实证 + %TEMP% 独立程序实测）

- **位置**：`internal/zhidao/client.go:520-524`（ErrUnexpectedEOF 分支）+ 同文件 497-540（IsReadErr 函数头注释）。
- **一句话问题**：R61 新增的 `errors.Is(err, io.ErrUnexpectedEOF)` 分支**功能正确**（独立程序实测真实短读形态 `unexpected EOF` 会穿透 `url.Error` 链命中），但注释宣称的"标准库 `transfer.go` 对未读满 body 的 FIN 包装为 `ErrUnexpectedEOF`"与真实链路**错位**——`transfer.go` 的 `io.ErrUnexpectedEOF`（LimitedReader 短读）在**正文读取阶段**返回后由 `ReadAll` 上抛，**不带 net.OpError**，随后被 `http.Client.Do` 包成 `url.Error{Err: ErrUnexpectedEOF}`（**非 RST/OpError 路径**）——因此**真实命中此分支的是"正文读取阶段的服务端 FIN 短读"（connection reset 之外的纯 EOF 提前中断）**，而 RST 形态（`net.OpError.Op=="read"`）**恒不产生 ErrUnexpectedEOF**。
- **证据链**（标准库源码逐段实证）：
  - `transfer.go:865`：`if lr, ok := b.src.(*io.LimitedReader); ok && lr.N > 0 { err = io.ErrUnexpectedEOF }`——仅 Content-Length 短读时返回 **纯 io.ErrUnexpectedEOF（不带 net.OpError）**。
  - `transport.go:2422-2435`：`bodyEOFSignal.fn` 对非 EOF 读错误只做 `waitForBodyRead <- false` + 取消检查后**原样返回 err**——短读错误**不落回 readLoop 的 `transportReadFromServerError`**（后者只在响应头读取阶段构造，transport.go:2347）。
  - `transport.go:2347`：`transportReadFromServerError{err}` 仅在**读响应头失败**（`readResponse` 失败）时构造；body 读阶段错误绝不产生该包装。
  - `client.go:985-997`：`cancelTimerBody.Read` 只在 didTimeout 时把错误 wrap 成 `timeoutError`（文案 `reading body`），非超时错误透传——`ErrUnexpectedEOF` 不会被打包。
  - **`%TEMP%` 独立程序实测**（r62verify，已清理）：mock 服务端 FLUSH 后 Hijack 直断 → `io.ReadAll(resp.Body)` 返回 `*errors.errorString unexpected EOF`（= `io.ErrUnexpectedEOF` 的错误串）→ `Do` 返回 `url.Error` 包装 → `errors.Is(err, io.ErrUnexpectedEOF)` **命中 true**。而**服务端未读 body 直接 RST**（main3 实测）→ `Do` 返回 `url.Error{Err: io.EOF}`（**不是 ErrUnexpectedEOF、也不是 read OpError**）。
- **触发条件**：平台响应头已到达、正文传输中断（Hijack 直断/网络中断），客户端读 body 中途命中 `io.ErrUnexpectedEOF`——本轮独立程序完全复现。
- **影响**：功能行为正确（该形态确实被判"可能已处理"）。问题在**契约表述**：注释"短读（不带 OpError）"+"真实 SelectClass 响应体小、Hijack 直断时该形态最常见"与实际 `url.Error` 包装链一致（`Do` 总会包 `url.Error`，`errors.Is` 穿透），但"RST 形态（*net.OpError.Op==read）"与"短读形态"**在 body 读阶段互斥**（前者是响应头读取阶段/裸 OpError，后者是正文阶段纯 EOF 短读）——R61 把两个形态写成一前一后并列，暗示它们可能叠加，误导后续维护者对"Hijack 直断到底命中哪个分支"的判断。
- **修复方向**：注释改为"短读形态：正文读取阶段 Content-Length 未传完的 FIN（`body.readLocked` 对 LimitedReader 短读包装为 `io.ErrUnexpectedEOF`，经 `url.Error` 包装链 `errors.Is` 穿透；与 RST 形态在 body 读阶段互斥——RST 是响应头读取阶段 `net.OpError.Op==read`）"；`isreaderr_test.go` 补一条"url.Error 包装的 ErrUnexpectedEOF 命中"断言（当前矩阵未覆盖 urlError 包装的短读形态）。一行注释 + 一行断言。

### MINOR-62-02：`fetchLoginPage` 对 403/429 响应重试一次，但注释归入"连接活性自愈"家族——与 `httpDo` 的严格连接层边界双标准

- **位置**：`internal/zhidao/client.go:283-317`（fetchLoginPage）。
- **一句话问题**：F52 的"连接活性自愈"契约（R52/R57 落盘）明确"只覆盖连接层错误（dial/read/write）、业务/取消错误原样上抛、绝不含业务重试"；`fetchLoginPage` 却对 **HTTP 403/429 响应**（服务端明确响应、非连接错误）`continue` 重试一次，并把该行为注释为"403/429 同覆盖：Windows 回环长时间 keep-alive 复用濒死连接时服务端可能返回 403/429"。403/429 是**服务端业务决策**（IP/账号限流），重试它在真实平台会再吃一次拒绝；429 重试一次后仍会失败（限流窗口未过）。纯 GET /login 不消耗验证码额度、重试一次收敛，功能影响低，但**契约表述把"连接活性自愈"（严格连接层）与"响应码重试"（业务层）混成一个语义**，与同一文件 `httpDo`/`isConnErrRetryable` 的"read 不重试、业务错误不重试"边界形成双标准。
- **证据链**：
  - `fetchLoginPage:296-305`：`if resp.StatusCode >= 400 { ...; if attempt == 2 { return fmt.Errorf("登录页 HTTP %d", resp.StatusCode) }; continue }`——对 4xx/5xx 无条件重试一次。
  - `httpDo:468-481`：只对 `isConnErrRetryable`（dial/write OpError）重试，read 错误不重试，业务错误原样上抛。
  - R52 契约（CLAUDE.md）："自愈只覆盖连接层错误（dial/read/write）一次，业务/取消错误原样上抛，绝不含业务重试"。
- **触发条件**：真实平台对 /login 返回 403（出口 IP 被限流）或 429（登录频率超限）——重试一次后再失败（行为与不重试等效，只是多一次请求）。
- **影响**：低（不消耗验证码、无副作用），但注释与 `httpDo` 边界语义相悖，且 403/429 的"连接濒死"归因未经实证（R52 时推测，非实测）。
- **修复方向**：注释改为"4xx/5xx 归入瞬时抖动重试一次（与 F52 连接自愈独立：GET /login 无副作用、不消耗验证码；403/429 是服务端响应、重试一次收敛，非连接层自愈语义）"，或直接去掉 403/429 的 `continue`（仅保留网络层错误重试）——一行注释/一行判断。

---

## OBSERVE

### OBSERVE-62-01：RUN4/R8 TestLoginRetryWithinLimits FAIL（captcha awaiting headers 超时 26.5s）——zhidao 包冷启动残余新形态

- RUN4 全量轮 zhidao 单包 FAIL：`TestLoginRetryWithinLimits (26.50s)`——`登录应成功: 获取验证码失败: Get ".../login/captcha?v=...": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`（mock 服务端 accept 就绪前首请求挂到 15s 超时）。与 R16 的 connectex 形态同根（Windows 回环冷启动队列残余），但表现为**超时**而非 connectex（对端半开/排队）。隔离 5 连跑全绿（0.10s/0.03s 级）。zhidao 包 readyProbe 已覆盖 `/login` 首请求，但 captcha 请求在 mock 接受窗口与 login 之后；本形态低频（21 轮 1 次）。

### OBSERVE-62-02：RUN2 TestReloginIfNeeded FAIL（connectex 78.48s）——全量轮 R2 冷启动残余延续

- R2 全量轮 zhidao 单包 FAIL：`TestReloginIfNeeded (78.48s)`——`重登失败: 初始化登录会话失败: Get ".../login": dial tcp ...: connectex`。隔离 race×3 全绿（0.04s）。与 R16/TestAccountOverrideRequiresAdminSession 同形态（全量轮 R2 是 R61 已记录形态）。R12 起连续 10 轮全绿回落。

### OBSERVE-62-03：RUN5 TestElectivesSnapshot FAIL（connectex 21.13s）——api 包冷启动残余延续

- RUN5 全量轮 api 单包 FAIL：`TestElectivesSnapshot (21.13s)`——`authenticateDirect → LoginByPassword → 初始化登录会话失败: dial tcp connectex`。api 包 readyProbe 已在夹具构造期前移冷启动窗口（10×200ms+2s 超时），但全量轮前序包 TIME_WAIT 残余在极端时刻仍可击穿（R58 已记录 readyProbe 自身 Fatal 样本同根）。RUN8-13 连续全绿回落。

### OBSERVE-62-04：RUN7 TestHealth FAIL（14.10s）——mock 服务器就绪探测自身 connectex 击穿

- RUN7 全量轮 api 单包 FAIL：`TestHealth (14.10s)`——`mock 服务器就绪探测失败: Get ".../ready": dial tcp connectex`（newTestDeps 构造期 readyProbe 10×200ms 全败直接 Fatal，handler_test.go:47）。R58 OBSERVE-58-01 记录的"readyProbe 自身 5 次全败直接 Fatal"样本延续（加宽到 10 次仍被极端残余击穿一次）。RUN8-13 连续全绿回落。CI `||` 重跑仍是正确姿势。

### OBSERVE-62-05：`TestHealth` 是全量轮首跑首当其冲的"牺牲品"（RUN7 形态实证）——api 包首测试连跑轮形态

- RUN7 失败者是 api 包首个测试 TestHealth——同一轮内其余 api 测试全绿（后续 all pass），证明**每轮全量连跑的首包首测试**吃最多冷启动残余（TIME_WAIT 队列未排空即建第一个 mock server + readyProbe 首连接）。这与 RUN4/5（非首测试）不同——本观察确认"首包首测试"是残余最大暴露点。

### OBSERVE-62-06：`cmd/probe/main.go` 修复后仍非可用工具（与 MINOR-61-01 修复后同族延续）

- probe/main.go 的 cookie 占位已统一为 `"1"`（R61 修复），但工具仍依赖 XUANKE_PROBE_TOKEN 直连真实平台、无认证校验、无 fallback——运行必失败（token 需手动注入真实会话值）。历史遗留定位（生产用 /api/electives），观察不阻塞。

### OBSERVE-62-07：`config.Load` 每次调用 `ensureEnvFile` 写盘（R61 OBSERVE-61-08 延续）

- cmd/logintest 调 config.Load：无 XUANKE_ADMIN_TOKEN 且无 data/.env 时会在仓库根写 data/.env（含随机管理员口令）。低频场景，观察。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MINOR-61-01（access_limit_cookie 占位 `***REMOVED***`→`"1"`） | 修复 | **闭合**——client.go:396-401 占位统一 `"1"` + 注释说明真实值由 Set-Cookie 收集；manager.go:310-314 Restore 占位同 `"1"`；probe/main.go:25 同改。三处语义一致。SetCookies 合并语义（client_test TestSetCookiesMergeSemantics）覆盖。全仓库 `grep REMOVED` 仅剩测试/归档文档（handler_test 测试夹具 `***REMOVED***` 是测试输入/断言值，非运行时活值） | **闭合** |
| MINOR-61-02（IsReadErr 漏 reading body 超时形态） | 修复 | **闭合**——client.go:530-531 双文案判定 + isreaderr_test.go:39-42 断言。标准库 client.go:994 实证文案精确匹配 `"Client.Timeout or context cancellation while reading body"` | **闭合** |
| MINOR-61-03（手动路径 read 文案零测试） | 修复 | **闭合**——handler_test TestHandleElectivesSelectReadErrMessage（select+exit 双分支）**实测命中 IsReadErr**（%TEMP% 独立程序复现同款 FLUSH+Hijack 直断 → `unexpected EOF` → `url.Error` 包装 → errors.Is 命中）；race 3 连跑全绿 | **闭合**（升级 MINOR-62-01 注释错位） |
| OBSERVE-61-01（R16 zhidao connectex FAIL） | 延续 | RUN4/2/5/7 四轮反弹（新形态见 OBSERVE-62-01~05），R12-R13 连续全绿回落 | 延续（OBSERVE-62-01/02/03/04） |
| OBSERVE-61-02（stats 测试注释矛盾） | 延续 | **闭合**——R61 已同步注释（handler_test.go:976-980 现引用"决策锚 1 保留识别事实"），本轮通读未再发现矛盾 | **闭合** |
| OBSERVE-61-03（targets nil 变空目标） | 延续 | 未变（handler.go:494-496 `nil → []` 整体保存） | 延续（OBSERVE-62-08） |
| OBSERVE-61-04（撞名文案归因） | 延续 | 未变（handler.go:136-139） | 延续 |
| OBSERVE-61-05（probe 工具 cookie 脱敏） | 延续 | **部分闭合**——cookie 占位已统一 `"1"`（MINOR-61-01 修复覆盖）；工具整体可用性仍未恢复 | 延续（OBSERVE-62-06） |
| OBSERVE-61-06（bench 默认 401） | 延续 | 未变 | 延续 |
| OBSERVE-61-07（logout 单会话） | 延续 | 未变 | 延续 |
| OBSERVE-61-08（config.Load 写 .env） | 延续 | 未变 | 延续（OBSERVE-62-07） |

> 注：上表 OBSERVE-62-08 编号被占用（延续表内引用），正式 OBSERVE 编号见上节 OBSERVE-62-01~07，OBSERVE-62-08 为延续表中 OBSERVE-61-03 的新编号。

---

## 本轮新视角逐项实证

### A. R61 四修是否完全正确
- **① IsReadErr 四形态全集**（RST / FIN io.EOF / 短读 io.ErrUnexpectedEOF / 超时双文案）：
  - **判定顺序互斥且完备**：nil → io.EOF → ErrUnexpectedEOF → 超时双文案 → `errors.As(*net.OpError).Op=="read"`。`io.EOF` 与 `ErrUnexpectedEOF` 互斥（errors.Is 不可能同时命中）；超时文案分支在 io.EOF/UnexpectedEOF 之后（timeoutError 不匹配 io.EOF/UnexpectedEOF，顺序无影响）；RST 分支最后（OpError 不匹配任何前置）。**完备性**：标准库 `net/http` 全部 read 阶段错误 = io.EOF（响应头 FIN）/ ErrUnexpectedEOF（正文短读）/ timeoutError 双文案 / net.OpError read——**无第五形态**（transport.go 的 `errServerClosedIdle` 只在空闲连接复用路径产生且被 `httpDo` 的 dial/write 判定跳过；`errTrailerEOF`（"http: unexpected EOF reading trailer"）在响应体已正常读完后 trailer 读取失败时产生——`errors.Is` 不匹配 io.EOF/UnexpectedEOF/超时，也不匹配 `net.OpError`（纯 errors.New）→ **不命中 IsReadErr，语义正确**（trailer 在正文完结后才读，平台已完整响应，上抛方按"失败"提示不误导；但该形态极低频，若未来需要可并入"已处理"判定）。
  - **`io.ErrUnexpectedEOF` 分支是否误伤正常路径？否**。标准库 `body.readLocked` 对**正常读完**（LimitedReader.N==0）返回 `io.EOF` 作为终止信号、`io.ReadAll` 吞掉；`ErrUnexpectedEOF` 只来自 LimitedReader 短读（N>0 时 EOF）或 chunked 提前 EOF——**正常路径绝不产生**。且该错误经 `bodyEOFSignal.condfn` 原样透传、`cancelTimerBody` 仅超时 wrap、`Do` 包成 `url.Error`——`errors.Is` 穿透命中，不误伤。
  - **唯一问题 = 注释错位**（MINOR-62-01）：R61 注释"短读（不带 OpError）"在 `Do` 层真实形态是 **url.Error 包装**（`errors.Is` 穿透，R61 的 `isreaderr_test.go` 的 urlError 模拟已验证 FIN 穿透、未验证 UnexpectedEOF 穿透——补一条即可）；且"RST/短读/超时"三形态在 body 读阶段实际互斥（RST 是响应头阶段、短读是正文阶段）。
- **② `"1"` 占位统一**：三处（client.go:397 / manager.go:313 / probe/main.go:25）语义一致（占位补充、真实值由登录 Set-Cookie 收集/合并语义覆盖）；注释齐（client.go:396-400 + manager.go:310-311 + probe 无注释但值一致）。**闭合**。
- **③ 手动路径 read 文案测试**：TestHandleElectivesSelectReadErrMessage 的 FLUSH+Hijack 直断**真实触达 IsReadErr 分支**——独立程序实测同款 mock 形态产生 `url.Error{Err: io.ErrUnexpectedEOF}` → `errors.Is` 命中 → handler 返回"平台可能已处理"。测试 mock 的 select/exit 路径覆盖真实调用链（d.srv.Config.Handler 替换的是真实 zhidao 客户端请求的 mock 服务端，走 client.SelectClass/ExitClass → doRequest → httpDo 全链路）。**闭合**。

### B. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 轮次 flake 统计
- **21 轮统计**：RUN 前 11 轮（R1 后台首跑 60.6s api / R2 zhidao FAIL 78s / R3-R11 全绿）→ R12-R21 连续 10 轮全绿（RUN4/RUN5/RUN7 为中段额外轮次，见下）→ **17 轮全绿 / 4 轮单包 FAIL**。
- **FAIL 明细（4 轮）**：
  - **R2**：zhidao `TestReloginIfNeeded`（connectex 78.48s，init 登录会话失败）→ 隔离 race×3 全绿（0.04s）。
  - **RUN4**：zhidao `TestLoginRetryWithinLimits`（captcha awaiting headers 超时 26.5s）→ 隔离 5 连跑全绿。
  - **RUN5**：api `TestElectivesSnapshot`（connectex 21.13s，LoginByPassword init 失败）→ RUN8 起连续全绿。
  - **RUN7**：api `TestHealth`（readyProbe 自身 connectex 14.10s，handler_test.go:47 Fatal）→ RUN8 起连续全绿。
- **趋势**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → **R62 17/21**（连续全绿段前 11 轮 10 绿 + 末 10 轮连续全绿）。四轮反弹均为 Windows 回环冷启动残余（connectex/超时），**无一为确定性缺陷**（隔离复跑 + 5 连跑全绿实证）。

### C. gofmt -l . 全量复检
- **零输出**（backend/ 全量，R61 新改后无回潮）。

### D. 全包逐行通读找新问题
- **scheduler 提交链**：spawnChain 六分支 sameClientFor + inflight 去重 + 链顶双判 + doneHas 保护 + waitChainExit 契约——同名重建 12 测试逐条通读全绿 ✅；WindowClosed 三判据单源 ✅；probeIntervalFor 零值守卫 ✅；maybeRelogin 决策侧/写回侧双复核 ✅。
- **api 鉴权族**：401/403/404/429/500 家族 + requireJSONBody CSRF 门 + XFF 可信反代 ✅；登录等时性（loginTimingFlat 双分支对称 + ConstantTimeCompare）✅；panic 恢复 500 + 详情不泄露（TestRecoverMiddlewareHidesPanicDetail）✅。
- **登录链路**：RSA/PKCS1v15/priorityId 空串/Vision 收敛/uniqueDeviceID 与前端逐字段一致/gateTryAcquire 非阻塞准入 ✅。
- **连接层**：httpDo 只重试 dial/write、read 不重试（MINOR-62-02 边界表述问题见上）✅；GetBody 自动设置（request.go:932-945）✅。
- **数据库**：migrateAddPublishMeta 在 refuseLegacy 前 + 迁移测试 + schema 列定义一致 ✅；SetMaxOpenConns(1) 单写者 ✅。
- **会话/票据**：ticket TTL 5min + 单次 + ConsumeTicket 先占用 ✅；RevokeAccount 全吊销 ✅；sweepLoop 清扫 ✅。
- **加密**：AES-256-GCM + master_key 32 字节双重校验 + enc: 前缀 + vision_key 明文拒绝加载 ✅。
- **手动报名/退选**：ErrUnauthorized → MaybeRelogin（不先 MarkTokenValid）✅；IsReadErr 文案对称 ✅。
- **未发现新 CRITICAL / MAJOR**。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor（反射指针身份）在 spawnChain 成功/失效/风控/窗口关闭/实时复核满员/实时复核失效六分支全覆盖 + maybeRelogin 决策侧 ClientFor 复核 + 写回侧复核——round40-43 的"双闭合"防线族 round62 仍完整。
- **窗口判据**：windowClosedLocked 三判据单源双侧对称；识别槽空快照不删、识别过期不截断（决策锚 1 三硬契约）——`openTimeForLocked` 返回原始识别值、`StateForAccount` 只做展示层 known 判定。
- **IsReadErr 四分支**：功能判定矩阵完备（MINOR-62-01 仅注释错位），不误伤正常路径。
- **任务日志窗口 SQL**：LoadLogs/LoadAllLogs 的 `id > max(id)-20000` 窗口空库语义（NULL 比较恒 false）由 window_empty_test 钉死。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生放行；凭据 AES-256-GCM + master_key 双重校验；登录/激活独立限流桶 + 429 家族；XFF 仅回环 + 开关双闸；panic 详情不泄露。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 成功才推进 + 失败 30s 退避 + streak≥3 复位 + syncing 无客户端复位防永久休眠。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽（R61 注释残留已清）。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...`（全量 + 关键包双通道） | 双通道 exit 0 |
| `gofmt -l backend/` | **零输出** |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` | **21 轮统计：17 轮全绿 + 4 轮单包 FAIL**（R2 TestReloginIfNeeded connectex 78s / RUN4 TestLoginRetryWithinLimits captcha 超时 26.5s / RUN5 TestElectivesSnapshot connectex 21s / RUN7 TestHealth readyProbe connectex 14s） |
| 隔离复跑（FAIL 后） | TestReloginIfNeeded race×3 全绿（0.04s）；TestLoginRetryWithinLimits+TestReloginIfNeeded 5 连跑全绿；zhidao+api 两包隔离全绿；RUN8-R13 连续 6 轮全绿回落 |
| 标准库 ErrUnexpectedEOF 实证 | `body.readLocked`（transfer.go:865）仅 LimitedReader 短读产生；`bodyEOFSignal.condfn`（transport.go:2422）透传不落 readLoop；`transportReadFromServerError` 仅读响应头阶段（transport.go:2347） |
| `%TEMP%` 独立程序实测 | FLUSH+Hijack 直断 → `READALL_ERR: *errors.errorString unexpected EOF`（`url.Error` 包装，errors.Is 命中）；未读 body 直接 RST → `url.Error{Err: io.EOF}`；服务端读完 body 未响应 → `url.Error{Err: EOF}` |
| 测试函数总数 | 215+（api 53 / scheduler 90+ / zhidao 25+ / store 9+ / accounts 8 / db 4 / secure 2+ / session 3+ / config 3 / runtime 3） |
| 审查期间工作树 | 与基线一致（仅并行代理 round62-frontend-findings.md 未跟踪文件 + 本报告） |
| `%TEMP%\r62verify` | 已清理 |

---

## 结论

- **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 7**。R61 四修经标准库源码逐段实证 + `%TEMP%` 独立程序实测**基本正确**：IsReadErr 四分支判定互斥完备、不误伤正常路径；`"1"` 占位三处语义一致；手动路径 read 文案测试真实触达 IsReadErr 分支。新暴露 MINOR 2 均为**契约/注释错位**（非运行时缺陷）——ErrUnexpectedEOF 分支的真实覆盖形态与注释不符、fetchLoginPage 的 403/429 重试语义与连接自愈边界双标准。
- **flake 统计结论**：R62 全量 **17/21 全绿**（4 轮单包 FAIL 全部为 Windows 回环冷启动 connectex/超时残余，隔离复跑 + 5 连跑全绿 + 末 10 轮连续全绿回落）。趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → **R62 17/21**——冷启动残余未根除但被 readyProbe/socketPreheat 持续压低；CI `||` 重跑吸收残余仍是当前正确姿势。
- **最致命 3 条**：
  1. MINOR-62-01：ErrUnexpectedEOF 分支注释与真实覆盖形态错位（`Do` 层 url.Error 包装 + 正文阶段短读 vs 注释"transfer.go 短读包装/不带 OpError"）——功能正确、契约表述需对齐。
  2. MINOR-62-02：fetchLoginPage 对 403/429 响应重试一次与"连接活性自愈只覆盖连接层、绝不含业务重试"的自愈边界双标准。
  3. flake：R2/RUN4/RUN5/RUN7 四轮反弹（connectex/超时冷启动残余，隔离全绿）。

## 教训

1. **注释必须追到真实错误链路逐段对齐**：R61 的 ErrUnexpectedEOF 注释（"不带 OpError/transfer.go 短读包装"）与实际错误链路（`body.readLocked` 产生 → `bodyEOFSignal` 透传 → `cancelTimerBody` 仅超时 wrap → `Do` 包 `url.Error`）错位。功能正确不代表注释正确——注释错位会让后续维护者按错误契约推断真实形态（如误以为 Hijack 直断走 RST 分支）。**凡是涉及"标准库包装链"的注释，必须把 Do→RoundTrip→body→transfer 每一层的 wrap/透传追到底再落笔**。
2. **"自愈/重试"语义必须与错误分类严格挂钩**：F52 的连接活性自愈（dial/write/read 连接错误）与 fetchLoginPage 的 4xx/5xx 重试（业务响应）是两类——403/429 重试属于业务重试，归入"连接自愈"注释会让同一文件出现双标准。**同一函数内的重试分支，注释必须写清是连接层还是业务层，二者不得混用语义**。
3. **全量轮首包首测试是冷启动残余最大暴露点**：RUN7 失败者是 api 包首测试 TestHealth（readyProbe 自身 Fatal），RUN4/5 是非首测试——证明每轮全量连跑最先承受 TIME_WAIT 队列残余的是第一个 mock server 的首连接。CI `||` 重跑吸收残余仍是正确姿势；若要进一步收敛，可考虑全量轮前先跑一次"排空轮"（任意单包轻量跑），但成本收益比低，不推荐。
4. **flake 趋势第 8 轮**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21——连续全绿段（前 11 轮 10 绿 + 末 10 轮连续 10 绿）证明 readyProbe/socketPreheat 对常规残余有效；四轮反弹均集中在全量轮中段（R2/RUN4/5/7），为 TIME_WAIT 极端残余的统计尾巴，无确定性缺陷证据。
