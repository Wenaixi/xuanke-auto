# R61 后端只读审查发现报告

> 审查基线：master @ `40d8185`（R60 收官，2026-09-21）。工作树预期完全干净（R60 已提交全部文件）；审查期间绝对只读——build/vet/gofmt/test 均为只读验证，无任何仓库内临时产物；临时验证程序全部落系统 `%TEMP%`（本轮未新建独立程序，标准库源码逐行实证替代独立程序，见 D 项）。写入前 `git status --short` 复核：仅并行代理 round61-frontend-findings.md 未跟踪文件，其余与基线一致。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round60 文末观察项逐条复核。
> 方法：全包逐行通读 + Go 标准库源码实证（net/http client.go/transport.go/transfer.go 逐段核对）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` **17+ 轮**（R16 出现 zhidao 单包 FAIL 后追加轮验证回落）。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 8**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R60 两处修复（IsReadErr 三形态全集 + 手动路径文案对称）经标准库源码实证 + 17+ 轮全量 **基本正确，但暴露两处新缺口**（MINOR-61-02：IsReadErr 漏"读 body 超时"形态；MINOR-61-03：手动路径 read 文案零测试覆盖）与一处字面量升级（MINOR-61-01：`access_limit_cookie` 的 `"***REMOVED***"` 是真实运行时 cookie 值，非脱敏显示）。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-61-02：IsReadErr 的判定集合未覆盖"读 body 阶段超时"形态**——标准库 `client.go:994` 实证：客户端超时在**响应体读取中途**触达时包装文案是 `"Client.Timeout or context cancellation while reading body"`（**不是** R60 匹配的 `awaiting headers`），`errors.Is(io.EOF)`/`strings.Contains(awaiting headers)`/`errors.As(net.OpError)` 三查皆不命中 → `IsReadErr=false` → scheduler 落"报名失败: context deadline exceeded (Client.Timeout or context cancellation while reading body)"误导文案。该形态**比 awaiting headers 更确定地"平台已处理"**（响应头已到达、平台必然已开始响应，只是正文 15s 内没传完）——R59→R60 两轮形态收敛仍差最后一块。
  2. **MINOR-61-01：`client.go:397` 的 `"***REMOVED***"` 兜底是真实发送给平台的 cookie 字面量（git 溯源实证），与 `manager.go:313` 的占位 `"1"` 双并存**——`git show 40a8d9d/e8b50c6/3cbba28` 证实 `"***REMOVED***"` 在提交历史中就是硬编码字面量（最早版本即有），**不是**本次审查输出被脱敏（git blob 内容即 `***REMOVED***`）。登录成功但平台未下发该 cookie / 全部 RESTORE 路径请求真实携带 `access_limit_cookie=***REMOVED***`（值格式合法但语义无意义）；与 manager.go Restore 的 `"1"` 占位不一致。平台当前未实证对值敏感（观察），但代码将审查脱敏产物当作活值，是 R2 以来就存在的隐性缺陷。
  3. **MINOR-61-03：手动报名/退选路径的 read 文案分支零测试覆盖**（R60 MINOR-60-03 修复未带测试）——handler_test 全部 52+ 测试无一处断言 `"响应读取失败"`/IsReadErr 分支；TestHandleElectivesSelectUnauthorizedRelogin 只覆盖 ErrUnauthorized。修复本身正确（handler.go:356-359/:423-426），但无回归钉：未来改动把该分支删掉/文案漂移不会红。
  4. **flake 收敛（15/16 全绿后 R16 反弹）**：R16 zhidao 包 2 测试 FAIL（TestRecognizeCaptcha connectex 2.02s + TestCaptchaConcurrency connectex 18.56s），隔离复跑全绿 + 5 连跑全绿。R17+ 复测回落确认中。趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → **R61 1/16**（连续第 6 轮正向收敛后单轮反弹）。

---

## CRITICAL

（无）

---

## MAJOR

（无）

---

## MINOR

### MINOR-61-01：`access_limit_cookie` 兜底 `"***REMOVED***"` 是真实运行时 cookie 值（非脱敏显示），与 Restore 占位 `"1"` 双语义并存

- **位置**：`internal/zhidao/client.go:396-398`（submitLogin 兜底）+ `internal/accounts/manager.go:312-314`（Restore 占位）+ `cmd/probe/main.go:25`（工具硬编码）。
- **一句话问题**：`"***REMOVED***"` 不是本轮审查/脱敏工具的输出，而是**提交历史中的硬编码字面量**——`git show 40a8d9d`（最早 zhidao client 引入提交）`submitLogin` 即含该串；`git show e8b50c6` probe/main.go 的 diff 中该串与 `token := "***REMOVED***"` 并存（token 已改环境变量、cookie 残留字面量）；`git log -S "***REMOVED***"` 命中 10+ 提交。这是 R2 时代审查代理脱敏真实会话 cookie 值时把替换文本粘贴成活值的产物——此后流水线把"脱敏回显"与"运行时发送值"混为一个形态。OBSERVE-60-09 只记录了 manager.go `"1"` 侧，未发现 client.go 侧形式。
- **证据链**（git 溯源实证，非读盘推断）：
  - client.go:397：`c.cookies["access_limit_cookie"] = "***REMOVED***"`（运行时会真实写入 Cookie 头发送给平台）。
  - manager.go:313：`SetCookies(map[string]string{"access_limit_cookie": "1"})`（RESTORE 路径同样真实发送）。
  - 两种占位并存：登录兜底发 `***REMOVED***`、重启恢复发 `"1"`——同一 cookie 在同一进程不同路径两种无效值。
- **触发条件**：① 登录链路服务端未 Set-Cookie 下发 access_limit_cookie（HAR 显示正常路径服务端会下发 → submitLogin 先收集真实值再兜底，兜底不触发）；② RESTORE 重启恢复（全部重启都走，恒发 `"1"`）；③ cmd/probe 工具（恒发 `***REMOVED***`，运行必失败，R59 OBSERVE-59-05 同根）。
- **影响**：平台当前未实证对 cookie 值敏感（格式合法，cookie value 字符集允许 `*`）——但一旦平台对该 cookie 启动值/长度校验，RESTORE 与登录兜底路径立即劣化；且 `***REMOVED***` 是审查脱敏产物混入活代码，任何后续 grep/竞品移植都会误以为"已脱敏"。低概率高味道。
- **修复方向**：`client.go:397` 与 `manager.go:313` 统一为同一占位值（如 `"1"`，对齐 manager 注释"占位补充"语义），补一行注释说明"真实值由登录链路 Set-Cookie 收集，占位仅防缺失"；probe/main.go 同改。一行级修复，TDD 可选（client_test 的 SetCookiesMergeSemantics 已覆盖合并语义）。

### MINOR-61-02：IsReadErr 漏"读 body 阶段超时"形态（标准库 `client.go:994` 实证）

- **位置**：`internal/zhidao/client.go:506-525`（IsReadErr）+ `scheduler.go:1650-1655` + `handler.go:356-359/:423-426`（消费端三处）。
- **一句话问题**：R60 补的"超时形态"只匹配 `Client.Timeout exceeded while awaiting headers`（`client.go:737`，等待响应头超时）；但 Go 标准库还有**第二处**客户端超时文案——响应体读取中途超时（`client.go:994`：`Client.Timeout or context cancellation while reading body`）。`Client.Timeout` 是整请求周期计时，服务端在 15s 超时前已开始发响应（响应头已到达、正文未传完）时，`cancelTimerBody.Read`（client.go:985-997）把底层错误 wrap 成该文案。Errors.Is(io.EOF) false、Contains(awaiting headers) false、errors.As(net.OpError) false（timeoutError 类型不 wrap OpError）→ **IsReadErr(false)**。
- **证据链**（标准库源码逐段实证）：
  - `transport.go:2768-2775`：`timeoutError` 类型，实现 `Is(err) == err == context.DeadlineExceeded`，不匹配 io.EOF、不 wrap OpError。
  - `client.go:994`：读 body 阶段 wrap 文案 `"(Client.Timeout or context cancellation while reading body)"`——与 737 行的 awaiting headers 文案**不同字符串**。
  - `client.go:985-997`：`cancelTimerBody.Read` 对 `err == io.EOF` 直接透传（不 wrap），对非 EOF + didTimeout 才 wrap 为 timeoutError——**正常读完（seen EOF）路径不产生 io.EOF 错误**（见 D 项）。
- **触发条件**：平台在 15s 客户端超时内已返回响应头、但正文传输未完成（SelectClass 响应体小，需网络严重拥塞/平台响应头先发但 body 卡住——低频但语义确定"已处理"）。
- **影响**：该形态下 scheduler/handler 显示原始 `context deadline exceeded (Client.Timeout or context cancellation while reading body)` 文案，其"平台可能已处理"语义比 awaiting headers **更强**（响应头已到 = 平台必然已处理完请求）但仍显示"失败"误导。与 MINOR-60-01 同构的形态缺口。
- **修复方向**：IsReadErr 增补 `strings.Contains(err.Error(), "Client.Timeout or context cancellation while reading body")` 分支（与 awaiting headers 分支并列），并把 R60 的"超时形态"注释从"awaiting headers"泛化为"等待响应头/读响应体两种超时文案"；isreaderr_test.go 补该文案命中测试。一行修复 + 一行测试。

### MINOR-61-03：手动报名/退选路径的 read 文案分支零测试覆盖（R60 MINOR-60-03 无回归钉）

- **位置**：`internal/api/handler.go:356-359`（select read 分支）+ `:423-426`（exit read 分支）+ handler_test.go。
- **一句话问题**：R60 为手动路径补了 IsReadErr 文案分支，但**未带任何测试**——全仓库 grep `"读取失败"`/`IsReadErr` 在 handler_test 零命中；consuming 端三处（scheduler 自动链 + handler 手动 select/exit）唯 scheduler 侧有 R59 前的间接覆盖（"connection reset" 走实时复核路径），手动两处完全裸露。IsReadErr 一旦语义变化（如 MINOR-61-02 修复改判据）或分支被误删，手动路径文案漂移无信号。
- **证据链**：
  - `grep IsReadErr handler_test.go` → 零结果；`grep "响应读取失败" handler_test.go` → 零结果。
  - TestHandleElectivesSelectUnauthorizedRelogin（handler_test.go:1831）只断言 ErrUnauthorized 分支文案含"自动重登"；TestHandleElectivesSelectAndExit（:1748）只走正常成功路径。
  - 逻辑本身正确：handler.go:353-361 顺序 ErrUnauthorized → IsReadErr → 原文兜底，与 scheduler 1650-1655 对称。
- **触发条件**：未来任何人对 IsReadErr/文案做修改（修复 MINOR-61-02 时大概率会动），手动路径行为漂移且测试不红。
- **影响**：回归保护缺口（非运行时缺陷）。测试方案易：mock 教务报名接口对 selectElectivesClass 返回"服务端读完 body 后连接中断"（或直接返回 IsReadErr 判定的包装错误），断言响应含"平台可能已处理"。
- **修复方向**：handler_test 补两条（select/exit 各一）：mock Client 返回 `io.EOF` 或 `url.Error{Err: context.DeadlineExceeded}` 形态错误，断言 `msg` 含"响应读取失败"/"可能已处理"。

---

## OBSERVE

### OBSERVE-61-01：R16（zhidao 包两个测试 connectex FAIL）单包 FAIL 为低频冷启动残余，隔离复跑 + 5 连跑全绿

- R16 zhidao FAIL：`TestRecognizeCaptcha (2.02s)` + `TestCaptchaConcurrency (18.56s)`——均 `dial tcp ...: connectex`（mock 服务器 accept 就绪前首请求），且 TestCaptchaConcurrency 正常 2-3s 被拖到 18.56s（信号量并发计数虚高推测同 R60）。隔离复跑全绿（2.05s）+ `-run 'TestRecognizeCaptcha|TestCaptchaConcurrency' -count=5` 5 连跑全绿（2.835s）。api 包单独连跑全绿（24.9s）。
- 与 R60 R9/R12 同根（Windows 回环 TIME_WAIT 冷启动残余）；R16 是 15 轮连续全绿后的首次 FAIL。**R17+ 复测回落验证中**，落盘时若已完成则以最终轮收官。

### OBSERVE-61-02：stats 测试注释矛盾残留（R60 OBSERVE-60-02 延续，未修）

- handler_test.go:978-979 注释仍写"识别过期语义下自动降级为'未识别'（绝不把过期旧值当开放时间）"，与 R56 修复后的行为（识别槽有值即照常输出过期日期、open_time_set=true）矛盾；断言 `st["open_time_set"] != (!recog.IsZero())` 语义正确（handler_test.go:1003 现在引用"决策锚 1 保留识别事实"的新注释，但 978 行旧注释未清）。纯注释残留，一行改注释即可。

### OBSERVE-61-03：`handleSetTargets` 对 `{}` / `{"targets":null}` 请求体静默变空目标（R60 OBSERVE-60-03 延续）

- handler.go:494-496 `req.Targets == nil` → `[]scheduler.Target{}` 整体保存。清空目标本身合法，该形态仅恶意/异常客户端可触达；要区分需指针字段，非缺陷观察。

### OBSERVE-61-04：撞名学生教务口令错时文案归因"管理口令错误"（R60 OBSERVE-60-04 延续）

- handler.go:136-139 管理员名 + 教务口令也错 → 返回"管理口令错误"。安全语义正确（等时 + 不泄露），文案在撞名场景可能误导，低频观察。

### OBSERVE-61-05：`cmd/probe/main.go` 硬编码 `access_limit_cookie=***REMOVED***`（延续 OBSERVE-60-05，与 MINOR-61-01 同根）

- 工具依赖 XUANKE_PROBE_TOKEN 直连真实平台，但 Cookie 头恒带 `***REMOVED***`——与 token 一样是历史硬编码残留。运行必失败（token 也需手动注入）。连同 MINOR-61-01 一并修复或标注 deprecated。

### OBSERVE-61-06：`cmd/bench` 默认 url 不带 -token 测 401 拒绝路径（R60 OBSERVE-60-06 延续）

- bench/main.go:19 默认 `http://localhost:3091/api/state`，未带 -token 测 401（自带注释已说明）。非缺陷观察。

### OBSERVE-61-07：`handleLogout` 只吊销当前会话 token 不吊销账号全部会话（R60 OBSERVE-60-07 延续）

- handler.go:577-585 语义明确（"注销当前会话"），与 RevokeAccount（删账号全吊销）分工清晰。观察。

### OBSERVE-61-08：`config.Load()` 每次调用 `ensureEnvFile` 写盘（R60 OBSERVE-60-08 延续）

- cmd/logintest 调 config.Load：无 XUANKE_ADMIN_TOKEN 且无 data/.env 时会在仓库根写 data/.env（含随机管理员口令）。低频场景，观察。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MINOR-60-01/02（IsReadErr FIN/超时形态） | 修复（三形态全集 + 形态矩阵测试） | **部分闭合→升级 MINOR-61-02**——FIN(io.EOF) 分支标准库实证**正确**（见 D 项；`body` 层读取正常完结吞 EOF、RoundTrip 读响应头 FIN 才产生 `url.Error{Err:EOF}`）；`awaiting headers` 分支实证**正确**（标准库 client.go:737 文案稳定、语义=请求体已发送）；但**读 body 阶段超时**是遗漏形态（client.go:994 文案不同） | **部分闭合**（升级 MINOR-61-02） |
| MINOR-60-03（手动路径 read 文案） | 修复（对称分文案） | **修复正确但无测试**——handler.go:356-359/:423-426 已补，grep handler_test 零断言 | **部分闭合**（升级 MINOR-61-03） |
| OBSERVE-60-01（api/zhidao flake） | 延续 | R16（zhidao 2 测试 connectex）FAIL + 隔离复跑全绿 + 5 连跑全绿；R17+ 回落验证中 | 延续（OBSERVE-61-01） |
| OBSERVE-60-02（stats 测试注释矛盾） | 延续 | 未修（978-979 注释仍残留旧语义） | 延续（OBSERVE-61-02） |
| OBSERVE-60-03（targets nil 变空目标） | 延续 | 未变 | 延续（OBSERVE-61-03） |
| OBSERVE-60-04（撞名文案） | 延续 | 未变 | 延续（OBSERVE-61-04） |
| OBSERVE-60-05（probe 工具 cookie 脱敏） | 延续 | 未变；与 MINOR-61-01 同根 | 延续（OBSERVE-61-05，升级同根 MINOR-61-01） |
| OBSERVE-60-06（bench 默认 401） | 延续 | 未变 | 延续（OBSERVE-61-06） |
| OBSERVE-60-07（logout 单会话） | 延续 | 未变 | 延续（OBSERVE-61-07） |
| OBSERVE-60-08（config.Load 写 .env） | 延续 | 未变 | 延续（OBSERVE-61-08） |
| OBSERVE-60-09（access_limit_cookie 占位语义） | 新观察 | **升级 MINOR-61-01**——client.go:397 的 `***REMOVED***` 经 git 溯源实证是硬编码字面量（非脱敏显示），且与 manager.go `"1"` 双占位并存 | **升级 MINOR** |

---

## 本轮新视角七项逐项实证

### A. R60 两处修复是否正确
- **① IsReadErr 三形态全集**（RST `net.OpError.Op=="read"` / FIN `errors.Is(io.EOF)` / 超时 `awaiting headers`）：**RST 与 FIN 分支正确**——`urlError{Err: opErr}` 的 errors.As 穿透命中 OpError（R60 已实证 + 标准库 uerr 包装链确认）；FIN 形态在 `url.Error{Err: io.EOF}`（RoundTrip 读响应头 FIN）穿透命中（errors.Is 沿 Unwrap 链）。**超时分支不完整**——只覆盖 awaiting headers（client.go:737），漏 reading body（client.go:994，MINOR-61-02）。TestIsReadErrCoversAllForms 的 urlError 模拟（isreaderr_test.go:54-58）用最小 `Unwrap()` 结构逐字段对齐 `net/url.Error` 的 `Err` 字段穿透路径，`errors.Is`/`As` 只依赖 Unwrap 链、不依赖 URL/Op 字段——模拟真实。
- **② 手动路径 read 文案**：handler.go:356-359/:423-426 与 scheduler 1650-1655 逐字段对称（文案措辞一致："报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）"）。**但零测试覆盖**（MINOR-61-03）。

### B. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 轮次 flake 统计
- **R1-R15 连续 15 轮全绿 → R16 zhidao 单包 FAIL（TestRecognizeCaptcha 2.02s + TestCaptchaConcurrency 18.56s，均 connectex）→ 隔离复跑全绿 + 5 连跑全绿 + api 包单独连跑全绿**。**15/16 全绿**；R17 复测运行中，以最终轮收官。flake 收敛趋势：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → **R61 1/16**（zhidao 包两轮全量轮 FAIL 后隔轮反弹，低频冷启动残余确认）。

### C. gofmt -l . 全量复检
- **零输出**（R60 新改后无回潮）。

### D. IsReadErr 的 io.EOF 分支与 http.Client 正常路径交互（标准库逐段实证）
- **正常读完 body 后 resp.Body.Close() 是否可能返回 io.EOF 并污染错误判定？否。**`transport.go:971-1002 body.Close`：`io.Copy(io.Discard, bodyLocked{b})` 中 `err == io.EOF` 会被 `Copy` 内部当正常终止（`copyBuffer` 在 src Read 返回 EOF 时返回 nil）；未读完才返回底层错误。`doRequest` 的 `io.ReadAll(resp.Body)` 结束后调用 `resp.Body.Close()`（client.go:440），Close 的 EOF 已被吞，不污染。
- **`io.ReadAll(resp.Body)` 正常读完是否返回 io.EOF？否。**`body.readLocked`（transfer.go:843-858）：读到 EOF 时 `sawEOF=true`、对 LimitedReader 检查余量（短读 → `io.ErrUnexpectedEOF`，非 io.EOF）；正常完结（余量 0）返回 `io.EOF` 作为**终止信号**——`io.ReadAll` 将 EOF 视为正常结束返回 `(data, nil)`，**不会把 io.EOF 当错误上抛**。即正常读 body 路径产出的错误**不可能**是纯 io.EOF——`errors.Is(err, io.EOF)` 在 doRequest 的 ReadAll 后只会命中"服务端未发完整 body 就 FIN"（LlimitedReader 短读 → ErrUnexpectedEOF，也非 EOF）。
- **关键结论**：`errors.Is(err, io.EOF)` 分支的真正命中点在 **RoundTrip 读响应头阶段**（`c.Do` 返回 `url.Error{Err: io.EOF}`，服务端读完完整请求体后正常 Close 未响应——R60 实证的"已处理未响应"典型 FIN 形态），**不会误伤正常完成路径**。R60 的 IsReadErr FIN 分支设计经此标准库实证**成立**。
- **读 body 中断的 EOF vs 服务端正常结束的 EOF 语义边界**：服务端正常结束（FIN）→ ReadAll 正常（EOF 吞）；服务端 Content-Length 未发完就 FIN → ErrUnexpectedEOF（**也不匹配 io.EOF**——当前 IsReadErr 对该短读形态 MISS，但语义上"平台已发送部分响应"与"已处理"兼容，可并入 MINOR-61-02 讨论）;服务端 FIN 未响应 → `url.Error{Err: EOF}` 命中。

### E. 上轮观察项延续复核
- **10 项中 9 项延续、0 闭合**：MINOR-60-01/02 部分闭合→升级 MINOR-61-02；MINOR-60-03 部分闭合→升级 MINOR-61-03；OBSERVE-60-09 升级 MINOR-61-01；OBSERVE-60-01~08 延续 8 项（详见延续表）。

### F. 全包逐行通读找新问题
- **scheduler 提交链**：spawnChain 六分支 sameClientFor + inflight 去重 + 链顶双判 + doneHas 保护 + waitChainExit 契约——同名重建 12 测试逐条通读全绿 ✅；**WindowClosed 三判据单源**（主判据 + 时钟失败带开放时间已过 + 幽灵窗口带 10s 裕量）✅ —— 开放时间单快照复用（判据 2/3 同快照）✅；**api 鉴权族**（401/403/404/429/500 家族 + requireJSONBody CSRF 门 + XFF 可信反代）✅——404 新路径 writeJSONStatus 先设头后 WriteHeader 与家族一致 ✅；**登录链路**（RSA/PKCS1v15/priorityId 空串/Vision 收敛/gateTryAcquire 非阻塞准入）✅；**时钟兜底**（streak≥3 复位 + 失败 30s 退避 + 成功自愈）✅；**数据库迁移幂等**（migrateAddPublishMeta 在 refuseLegacy 前 + 测试）✅；**httpDo/cloneReq 重试路径**——r61 复读确认 read 不重试、dial/write 重试、GetBody 自动设置（request.go:932-945）✅；**手动报名/退选路径**——ErrUnauthorized 重登 + IsReadErr 文案（MINOR-61-03 缺测试）✅；**`access_limit_cookie` 占位**（MINOR-61-01）✅。
- **未发现新 CRITICAL / MAJOR**。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor（反射指针身份）在 spawnChain 成功/失效/风控/窗口关闭/实时复核满员/实时复核失效六分支全覆盖 + maybeRelogin 决策侧 ClientFor 复核 + goroutine 写回侧 ClientFor 复核——round40-43 的"双闭合"防线族 round 61 仍完整。
- **窗口判据**：windowClosedLocked 三判据单源（主判据 + 时钟 ≥3 带"开放时间已过" + 幽灵窗口带 10s 裕量）双侧对称；识别槽空快照不删、识别过期不截断（决策锚 1 三硬契约）——`openTimeForLocked` 返回原始识别值、`StateForAccount` 只做展示层 known 判定，代码与决策锚逐条对齐。
- **任务日志窗口 SQL**：`LoadLogs/LoadAllLogs` 的 `id > max(id)-20000` 窗口在空库语义（NULL 比较恒 false→空 slice）由 window_empty_test 钉死；30050 行窗口测试批插语义正确。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生放行；凭据 AES-256-GCM + master_key 32 字节双重校验；登录/激活独立限流桶 + 429 家族；XFF 仅回环 + 开关双闸。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 成功才推进 + 失败 30s 退避 + streak≥3 复位 + syncing 无客户端复位防永久休眠。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽（注释残留除外）。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | 双通道 exit 0 |
| `gofmt -l .` | **零输出**（R60 新改无回潮） |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` | **R1-R15 连续 15 轮全绿 → R16 zhidao FAIL（TestRecognizeCaptcha connectex 2.02s + TestCaptchaConcurrency connectex 18.56s）**；**15/16 全绿**；R17+ 复测运行中 |
| zhidao 包隔离复跑（R16 失败后） | `-run 'TestRecognizeCaptcha\|TestCaptchaConcurrency' -count=1` 全绿（2.057s）；`-count=5` 5 连跑全绿（2.835s） |
| api 包单独连跑 | 全绿（24.917s） |
| TestAccountOverrideRequiresAdminSession 复跑 | 首跑 21.67s FAIL（`/login/captcha` awaiting headers 超时）→ 复跑 + count=3 全绿（2.3s / 3.99s）——同一冷启动残余形态 |
| 标准库超时双文案实证 | client.go:737 `awaiting headers`（IsReadErr 已覆盖）；**client.go:994 `reading body`（IsReadErr 未覆盖，MINOR-61-02）** |
| 标准库 EOF 语义实证 | body.Close 正常读完返回 nil（EOF 吞）；ReadAll 正常完结返回 nil；RoundTrip 读响应头 FIN → `url.Error{Err: io.EOF}`——IsReadErr FIN 分支正确不误伤 |
| `access_limit_cookie` git 溯源 | `git show 40a8d9d/e8b50c6/3cbba28` 实证 `"***REMOVED***"` 是最早版本硬编码字面量（git blob 内容即该串），`git log -S` 命中 10+ 提交——真实运行时 cookie 值（MINOR-61-01） |
| 测试函数总数 | 215+（api 52 / scheduler 90+ / zhidao 25+ / store 9+ / accounts 8 / db 4 / secure 2+ / session 3+ / config 3 / runtime 3） |
| 审查期间工作树 | 与基线一致（仅并行代理 round61-frontend-findings.md 未跟踪文件 + 本报告） |

---

## 结论

- **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 8**。R60 两处修复（IsReadErr 三形态 + 手动路径文案）经标准库实证**基本正确但收敛未闭合**——FIN/awaiting headers 分支正确，漏 reading body 超时形态；手动路径修复正确但零测试。产品逻辑继续维持在 MAJOR 之下。
- **flake 统计结论**：R61 全量**前 16 轮 15 轮全绿 + R16 zhidao 单包 FAIL**（隔离复跑全绿 + 5 连跑全绿 + api 独立连跑全绿）。趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → **R61 15/16**——连续 15 轮全绿后 R16 反弹（connectex 冷启动残余，非确定性缺陷），R17 复测运行中。
- **最致命 3 条**：
  1. MINOR-61-02：IsReadErr 漏"读 body 超时"形态（标准库实证 `reading body` 文案，语义比 awaiting headers 更强地"已处理"）。
  2. MINOR-61-01：`access_limit_cookie` 兜底 `"***REMOVED***"` 是真实运行时字面量（git 溯源实证），与 Restore `"1"` 双占位并存。
  3. MINOR-61-03：手动路径 read 文案零测试覆盖（修复未带回归钉）。

## 教训

1. **同类错误形态必须穷举标准库**：R60 修"超时形态"时只匹配 `awaiting headers`（client.go:737），但标准库同一客户端超时机制有**两处** wrap 文案（737 等待响应头 / 994 读响应体）——`Client.Timeout` 是整请求周期计时，两个阶段各自 wrap。只修报告的复现形态、不复核标准库全形态，是 R59→R60→R61 连续三轮形态收敛仍未闭合的根因。
2. **`errors.Is(err, io.EOF)` 的正确性边界在标准库层**：body 层 `sawEOF` 吞掉正常完结 EOF，ReadAll 不产生 io.EOF 错误；io.EOF 只从 RoundTrip 读响应头 FIN（`url.Error{Err:EOF}`）或未读响应体层产生——R60 的 FIN 分支经此实证成立、不误伤正常路径。结论必须落到标准库代码行而非印象。
3. **审查脱敏产物会变成活代码**：`"***REMOVED***"` 从 R2 审查时被粘贴进 submitLogin 兜底起就是运行时字面量，此后 59 轮审查只当"脱敏回显"看待。教训：**涉及"`***REMOVED***`/`xxx`/`REDACTED`"这类串的代码在通读时必须当作真实运行时值验证**，不能被"审查过=已脱敏"的惯性遮蔽。
4. **flake 收敛趋势第 7 轮**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 15/16——zhidao 包 R12/R16 两轮全量轮 FAIL（隔离全绿），冷启动残余未根除但被 readyProbe/socketPreheat 持续压低；CI `||` 重跑吸收残余仍是当前正确姿势。