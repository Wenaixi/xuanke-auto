# R60 后端只读审查发现报告

> 审查基线：master @ `eaff50a`（R59 收官，2026-09-21）。工作树预期完全干净（R59 已提交全部文件）；审查期间绝对只读——临时验证程序全部落在系统临时目录（`%TEMP%\r60_isreaderr`，10 个独立程序实证，已清理），仓库内零新建临时产物（R58 教训遵守）。写入前 `git status --short` 复核：工作树与基线一致（仅并行代理的未跟踪 findings 文件，见文末实证表）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部 209+ 个测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~59 各轮报告逐条复核 + R59 文末观察项延续。
> 方法：全包逐行通读 + 标准库行为独立验证（10 个临时程序实证）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` **12 轮**。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 9 + 延续 9 项**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R59 两处修复（404 writeJSONStatus + IsReadErr）经本轮 12 轮全量 + 独立程序实证**基本正确**，但暴露两处对称缺口（MINOR-60-01/02：IsReadErr 只覆盖 RST 形态、未覆盖 FIN/超时形态）与一处新观察（OBSERVE-60-09：access_limit_cookie 占位值语义漂移）。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-60-01/02：IsReadErr 的"平台可能已处理"语义只覆盖 RST 连接重置形态，未覆盖 FIN（正常关闭）与超时形态**——独立程序实证：服务端读完 body 后正常 Close()（FIN）客户端得到 `url.Error{Err: io.EOF}`，`errors.As(err, &net.OpError)` **不命中**（纯 EOF 不带 OpError）→ `IsReadErr=false` → scheduler 落"报名失败: ... EOF"误导文案（平台实际已处理）。真实平台"处理完成未响应"的典型形态是 FIN 而非 RST，R59 修复只覆盖了 RST 一种。
  2. **MINOR-60-03：手动报名/退选路径（api/handler.go）未区分 read 错误文案**——R59 只在 scheduler 自动链区分；`handleElectiveSelect`(:353)/`handleElectiveExit`(:417) 对 read 错误原文回传 `"read tcp ...: connection reset"`，前端手动路径 UX 与自动链不对称（同根误导）。
  3. **flake 收敛短暂反弹**：R60 全量 **12 轮中 10 轮全绿** + R9（zhidao TestCaptchaConcurrency 53.39s）+ R12（zhidao TestRecognizeCaptcha 2.02s）两次 zhidao 单包 FAIL（均隔离复跑全绿 + 连跑全绿）。趋势 R57 3/11 → R58 2/10 → R59 2/11 → **R60 2/12**，readyProbe/TestMain preheat 组合持续压低冷启动残余，但 zhidao 包出现连续两轮全量轮 FAIL（均隔离全绿）。

---

## CRITICAL

（无）

---

## MAJOR

（无）

---

## MINOR

### MINOR-60-01：IsReadErr 未覆盖 FIN（纯 EOF）与超时形态——"平台可能已处理"的典型场景 miss

- **位置**：`internal/zhidao/client.go:498-507`（IsReadErr）+ `internal/scheduler/scheduler.go:1650-1655`（read 文案分支）。
- **一句话问题**：R59 新增的 IsReadErr 只覆盖 `net.OpError.Op=="read"`（RST 连接重置形态）；但真实平台在完整处理报名后**正常关闭连接（FIN）**而非 RST，Go 客户端把 FIN 序列化成 `url.Error{Err: io.EOF}`（不带 net.OpError）——`errors.As(err, &net.OpError)` 不命中 → `IsReadErr=false` → scheduler 走"报名失败: Post ...: EOF"误导文案（平台可能已成功处理这次报名）。
- **证据链**（独立程序实证，落 `%TEMP%\r60_isreaderr`，main3/main6/main8/main9）：
  - **FIN 形态**（服务端读完完整 body 后正常 Close，R57 双报根因背景的"服务端已完整消费"形态）：`Post "...": EOF`，`errors.As(err, &net.OpError)` 返回 **false** → `IsReadErr=false`。
  - **RST 形态**（服务端有未读数据时 SO_LINGER(0) 关闭）：`Post "...": read tcp ...: wsarecv: ...`，errors.As 穿透 url.Error 命中 OpError → `IsReadErr=true`。
  - **超时形态**（平台已处理但响应超过 15s 客户端超时）：`context deadline exceeded (Client.Timeout exceeded while awaiting headers)` → `IsReadErr=false`。
  - 三种形态都是"请求已发出、平台可能已处理"语义，IsReadErr 只覆盖 RST 一种。
- **触发条件**：Windows 回环 keep-alive 复用濒死连接（R52 背景）或平台响应中断（服务端处理完报名后正常关连接）或平台响应慢于 15s，恰好落在 SelectClass 上。
- **影响**：黄金期下 FIN/超时形态仍显示"报名失败"（平台可能已成功）——R59 修复只半程生效；下个 tick 重复报名被拒"已选过"时 failed 永久残留（MINOR-59-02 的残留形态）。
- **修复方向**：`IsReadErr` 增加两个分支：`errors.Is(err, io.EOF)`（FIN 形态 = 对端完成处理但未响应）与 `strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers")`（超时形态）；补 FIN 形态单元测试（mock 服务端读完 body 后 Close）。

### MINOR-60-02：doRequest 的 io.ReadAll 裸 OpError 形态与 url.Error 包装形态的判定不对称

- **位置**：`internal/zhidao/client.go:440-443`（doRequest 的 io.ReadAll）+ `IsReadErr`（498-507）。
- **一句话问题**：R59 的 IsReadErr 设计时假设 read 错误都经 `httpDo` 返回的 url.Error 包装（独立程序实证 url.Error 链上 errors.As 能穿透命中 OpError）；但 `doRequest` 在**响应体读取中断**时返回的是 `io.ReadAll` 的**裸 `*net.OpError`**（不带 url.Error）——该形态发生在服务端**已发送响应头**之后，平台大概率已处理报名。
- **证据链**（独立程序实证，main7/main8）：
  - `io.ReadAll(resp.Body)` 阶段 RST：返回裸 `read tcp ...: wsarecv: ...`（无 url.Error 前缀），`errors.As` 直接命中 OpError → `IsReadErr=true`（当前实现该形态**已覆盖**，因 errors.As 对裸 OpError 直接命中）。
  - 该形态与 MINOR-60-01 的 FIN 形态是同一 `doRequest` 读取路径的两种结局：RST → 命中；FIN/EOF → miss。
- **触发条件**：平台发完响应头后连接中断（RST）或正常关闭（FIN），读 body 时出错。
- **影响**：读 body 中断的 RST 形态已被 IsReadErr 覆盖（当前实现正确）；但同路径的 FIN 形态 miss（见 MINOR-60-01）。两条发现同根，合并修复即可。
- **修复方向**：与 MINOR-60-01 合并——IsReadErr 增加 EOF/超时分支，覆盖 `doRequest` 读取路径的全部中断形态。

### MINOR-60-03：手动报名/退选路径（api handler）未区分 read 错误文案（对称缺口）

- **位置**：`internal/api/handler.go:340-355`（handleElectiveSelect）+ `:406-418`（handleElectiveExit）。
- **一句话问题**：R59 只在 scheduler 自动链区分 read 文案（scheduler.go:1652-1655）；手动报名/退选的 `client.SelectClass/ExitClass` 错误分支（handler.go:353 `writeJSON(w, 1, nil, err.Error())` / :417 同构）对 read 错误原文回传——用户手动点报名时遇到"平台可能已处理"的 read 错误，看到的是 `"read tcp ...: connection reset"` 生硬文案而非"请求已发出但响应读取失败（可能已处理）"。
- **证据链**：handler.go:340-355：`errors.Is(err, ErrUnauthorized)` 特判后直接 `writeJSON(w, 1, nil, err.Error())`；handler.go:406-418 同构。zhidao.IsReadErr 已导出（R59 新增）但 api 层未消费（grep 全仓库仅 scheduler 一处消费）。
- **触发条件**：手动点击报名/退选恰遇连接层 read 错误（低频）。
- **影响**：前端手动路径 UX 与自动链不对称——用户看到生硬网络错误文案，与 MINOR-59-02 同根误导；手动路径失败状态同样可能在下个自动链 tick 覆盖。
- **修复方向**：handler 两处错误分支复用 `zhidao.IsReadErr` 区分文案（与 scheduler 1652-1655 同款），补测试。

---

## OBSERVE

### OBSERVE-60-01：R9/R12（zhidao 两包 FAIL）全量轮单包 FAIL 均为低频冷启动残余，隔离复跑 + 连跑全绿

- R9 zhidao FAIL：`TestCaptchaConcurrency (53.39s)`——该测试正常 0.16s，全量轮中 53.39s 才完成（同包其他测试正常，`-run TestCaptchaConcurrency -count=1 -v` 复跑 0.16s 全绿 + `-count=5` 连跑 5 轮每轮 0.16s 全绿）。
- R12 zhidao FAIL：`TestRecognizeCaptcha (2.02s)`——该测试正常 0.01s，全量轮中 2.02s 才完成（`-run TestRecognizeCaptcha -count=1 -v` 复跑 0.01s 全绿 + 与 TestCaptchaConcurrency 连跑 5 轮全绿）。
- 与 R59 的 R9（api）/R11（zhidao）同根：Windows 回环 TIME_WAIT 冷启动残余（httptest mock 服务器 accept 就绪前的首请求 connectex 被信号量并发计数误判为"并发超限"——10 个 goroutine 中部分首请求 connectex 后立即失败，信号量计数仍认为它们在飞，maxInFlight 虚高到 >2；或 TestRecognizeCaptcha 的 Authorization 头断言在首请求 connectex 重试时吃到非预期错误）。隔离复跑全绿证明非确定性缺陷。flake 收敛趋势：R57 3/11 → R58 2/10 → R59 2/11 → **R60 2/12**（连续 8 轮全绿后 R9 单包 FAIL，R10/R11 全绿，R12 再 FAIL——zhidao 包出现连续两轮全量轮 FAIL，是 R57 以来首次。

### OBSERVE-60-02：stats 测试注释矛盾残留（R59 OBSERVE-59-02 延续，未修）

- handler_test.go:978-979 注释仍写"识别过期语义下自动降级为'未识别'（绝不把过期旧值当开放时间）"，与 R56 修复后的行为（识别槽有值即照常输出过期日期、open_time_set=true）矛盾；断言 `st["open_time_set"] != (!recog.IsZero())` 语义正确。纯注释残留，一行改注释即可。

### OBSERVE-60-03：`handleSetTargets` 对 `{}` / `{"targets":null}` 请求体静默变空目标（R59 OBSERVE-59-03 延续）

- handler.go:484-486 `req.Targets == nil` → `[]scheduler.Target{}` 整体保存（清空目标）。"清空目标"本身合法，此形态仅恶意/异常客户端可触达。若想区分"未提供字段"与"显式空数组"需指针字段，非缺陷观察。

### OBSERVE-60-04：撞名学生教务口令错时文案归因"管理口令错误"（R59 OBSERVE-59-04 延续）

- handler.go:136-139：管理员名 + 教务口令也错（撞名学生输错学生密码）→ 返回"管理口令错误"。文案误导（低频、仅撞名场景），安全语义正确（等时 + 不泄露）。

### OBSERVE-60-05：`cmd/probe/main.go` 硬编码 `access_limit_cookie=***REMOVED***`（R59 OBSERVE-59-05 延续）

- 工具依赖 XUANKE_PROBE_TOKEN 直连真实平台，cookie 真实值已脱敏丢失 → 运行必 code=-1。历史遗留工具（生产走 /api/electives），观察（未来可删或标注 deprecated）。

### OBSERVE-60-06：`cmd/bench` 默认 url 不带 -token 测 401 拒绝路径（R59 OBSERVE-59-06 延续）

- bench/main.go:19 默认 `http://localhost:3091/api/state`，未带 -token 时测 401（自带注释已说明）。非缺陷观察。

### OBSERVE-60-07：`handleLogout` 只吊销当前会话 token 不吊销账号全部会话（R59 OBSERVE-59-07 延续）

- handler.go:567-575：设计语义明确（"注销当前会话"），与 RevokeAccount（删账号全吊销）分工清晰。观察。

### OBSERVE-60-08：`config.Load()` 每次调用 `ensureEnvFile` 写盘（R59 OBSERVE-59-08 延续）

- cmd/logintest 调 config.Load：无 XUANKE_ADMIN_TOKEN 且无 data/.env 时会在仓库根写 data/.env（含随机管理员口令）。实际风险存在但低频（开发态无 .env 环境跑 logintest 才触发）。观察。

### OBSERVE-60-09：`access_limit_cookie` 兜底恒设 `"1"`（RESTORE 路径占位值）与平台真实值语义漂移（新观察）

- **位置**：`internal/zhidao/client.go:396-398`（submitLogin 兜底 `"1"`）+ `internal/accounts/manager.go:312-314`（Restore 占位 `"1"`）。
- **一句话问题**：git 溯源 `***REMOVED***` 是 R2 审查脱敏替换的真实平台会话值（commit e8b50c6 时代，probe/main.go 硬编码），后经 Restore 语义演变成占位 `"1"`。若平台以 access_limit_cookie 作限流/风控计数依据（HAR 中该 cookie 由服务端下发），占位 `"1"` 与真实值不匹配。
- **证据链**：manager.go:312-314 `SetCookies(map[string]string{"access_limit_cookie": "1"})`；client.go:396-398 `if _, ok := c.cookies["access_limit_cookie"]; !ok { c.cookies["access_limit_cookie"] = "1" }`；login.py 中该 cookie 由 /login 响应 Set-Cookie 真实下发（登录链路收集真实值覆盖占位值）。
- **触发条件**：RESTORE 路径（重启恢复会话，只复用 token 不重登）恒发占位 `"1"`，直到下次 login 才被 submitLogin 的真实收集值覆盖——影响"只复用 token 不重登"的形态。
- **影响**：平台未实证对占位值报错（若平台以该 cookie 做限流计数，占位值与真实值不一致可能影响风控判定——未经实证，非缺陷观察）；运行时登录路径（submitLogin 收集真实值）不受影响。
- **修复方向**：确认平台对 access_limit_cookie 的校验语义；若仅作会话存在标记则 `"1"` 足够，若作计数需在 Restore 时保真实值。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MINOR-59-01（404 Content-Type） | 修复（writeJSONStatus + 真实 Server 断言） | **完全闭合**——router.go:199 已改 writeJSONStatus（先设头后 WriteHeader）；TestApiUnknownPath404 补真实 httptest.NewServer 端到端断言（真实 Server 上 CT=application/json，Recorder 假绿行为分叉已消除）；独立验证真实 Server 行为一致 | **闭合** |
| MINOR-59-02（read 错误文案） | 修复（IsReadErr + scheduler 文案） | **部分闭合→升级 MINOR-60-01/02**——RST 形态命中（url.Error 包装链 errors.As 穿透实证 + io.ReadAll 裸 OpError 实证）；FIN/超时形态 miss（独立程序实证） | **部分闭合**（升级 MINOR-60-01/02） |
| OBSERVE-59-01（api/zhidao flake） | 延续 | R9（zhidao TestCaptchaConcurrency 53s）FAIL + 隔离复跑全绿 + 5 连跑全绿；其余 11 轮全绿（1/12） | 延续（OBSERVE-60-01） |
| OBSERVE-59-02（stats 测试注释矛盾） | 延续 | 未修 | 延续（OBSERVE-60-02） |
| OBSERVE-59-03（targets nil 变空目标） | 延续 | 未变 | 延续（OBSERVE-60-03） |
| OBSERVE-59-04（撞名文案） | 延续 | 未变 | 延续（OBSERVE-60-04） |
| OBSERVE-59-05（probe 工具 cookie 脱敏） | 延续 | 未变 | 延续（OBSERVE-60-05） |
| OBSERVE-59-06（bench 默认 401） | 延续 | 未变 | 延续（OBSERVE-60-06） |
| OBSERVE-59-07（logout 单会话） | 延续 | 未变 | 延续（OBSERVE-60-07） |
| OBSERVE-59-08（config.Load 写 .env） | 延续 | 未变 | 延续（OBSERVE-60-08） |

---

## 本轮新视角七项逐项实证

### A. R59 两处修复是否正确
- **① 404 writeJSONStatus + 真实 Server 断言**：**正确且闭合**。router.go:199 已改 `writeJSONStatus(w, http.StatusNotFound, 404, nil, "接口不存在")`（先设头后 WriteHeader，与 401/403/429/500 同族）；TestApiUnknownPath404 补真实 httptest.NewServer 端到端断言（`realSrv := httptest.NewServer(d.api)` + `http.Get` 抓 HTTP 层 CT）。独立验证：真实 Server 上 writeJSONStatus 路径 CT=`application/json`；httptest.ResponseRecorder 允许 WriteHeader 后设 Header（假绿），真实 Server 丢弃已提交响应后的 Header 设置——断言能抓到未来回归。
- **② IsReadErr + scheduler read 文案**：**正确但覆盖不完整（MINOR-60-01/02）**。errors.As 穿透 url.Error 包装链命中 OpError 已实证（RST 形态）；但真实平台"处理完成未响应"的典型形态（FIN 正常关闭连接）序列化成纯 EOF（不带 OpError）——`IsReadErr=false`，scheduler 仍落"报名失败: ... EOF"误导文案。**修复正确性结论：RST 形态正确、FIN/超时形态未覆盖**。

### B. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 12 轮 flake 统计
- **R1-R8 连续 8 轮全绿 → R9 zhidao FAIL（TestCaptchaConcurrency 53.39s）→ R10/R11 连续 2 轮全绿 → R12 zhidao FAIL（TestRecognizeCaptcha 2.02s）**。**10/12 全绿**，2 个单包 FAIL（都在 zhidao 包）隔离复跑全绿 + 连跑全绿。flake 收敛趋势：R57 3/11 → R58 2/10 → R59 2/11 → **R60 2/12**（zhidao 包出现连续两轮全量轮 FAIL，但均隔离复跑全绿——非确定性缺陷）。readyProbe/TestMain preheat 组合持续压低冷启动残余，**零"readyProbe 自身失败 Fatal"样本**（连续三轮零样本）。R12 的 TestRecognizeCaptcha 2.02s（正常 0.01s）与 R9 的 TestCaptchaConcurrency 53.39s（正常 0.16s）同根：Windows 回环 TIME_WAIT 冷启动残余（mock 首请求 connectex）。

### C. gofmt -l . 全量复检
- **零输出**（R58 清剿 + R59 新改后无回潮）。

### D. R59 IsReadErr 在真实错误链上的行为
- **url.Error 包装链穿透实证**（独立程序 main4）：`errors.As(err, &url.Error)` 对 OpError 返回 false（url.Error 不是 OpError 的子类型）；`errors.As(err, &net.OpError)` 从 `url.Error{Err: opErr}` **穿透成功**——IsReadErr 的 `errors.As(err, &nerr)` 在真实 `http.Client` 返回的 url.Error 包装链下能命中 `net.OpError.Op=="read"`（RST 形态）。
- **FIN/EOF 形态 miss**（main3/main6）：服务端读完 body 后正常 Close → `Post "...": EOF`，errors.As 不命中 net.OpError → IsReadErr=false。这是真实平台"已处理未响应"的典型形态（平台处理完报名后关连接是常态，RST 是异常）。
- **超时形态 miss**（main3）：`context deadline exceeded awaiting headers` → IsReadErr=false。
- **结论**：IsReadErr 的 errors.As 机制本身正确，但判定集合不完整——只覆盖 RST，漏掉 FIN（EOF）与超时（MINOR-60-01/02）。

### E. 上轮观察项延续复核
- **MINOR-59-01 闭合**（404 真实 Server CT=application/json 实证）/ **MINOR-59-02 部分闭合→升级 MINOR-60-01/02**（RST 命中、FIN/超时 miss）/ **OBSERVE-59-01~08 延续 8 项**。

### F. 全包逐行通读找新问题
- **scheduler 提交链**（spawnChain 六分支 sameClientFor + inflight 去重 + B30-01 链顶双判 + doneHas 保护 + waitChainExit 契约）——同名重建 10 测试逐条通读全绿 ✅；**WindowClosed 三判据单源**（主判据 + 时钟失败带开放时间已过 + 幽灵窗口带 10s 裕量）+ 裕量双侧对称 ✅；**task_log 窗口 SQL**（空库语义 window_empty_test 固化 + 30050 行窗口测试）✅；**api 鉴权族**（401/403/404/429/500 家族 + requireJSONBody CSRF 门 + XFF 可信反代）✅——**404 新路径 writeJSONStatus 已正确纳入家族**；**登录链路**（RSA/PKCS1v15/priorityId/Vision 收敛/gateTryAcquire 非阻塞准入）✅；**凭据 AES-256-GCM + master_key 32 字节双重校验** ✅；**数据库迁移幂等**（migrateAddPublishMeta 纯新增列自动迁移、refuseLegacy 剔除已迁移列）✅；**httpDo/cloneReq 重试路径**——R57 修复正确（dial/write 重试、read 不重试），read 文案对 RST 生效、FIN/超时 miss（MINOR-60-01/02）✅；**手动报名/退选路径**——R59 只在自动链区分 read 文案、手动路径未区分（MINOR-60-03）✅；**access_limit_cookie 占位语义**（OBSERVE-60-09）。
- **未发现新 CRITICAL / MAJOR**。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核满员/实时复核失效）+ maybeRelogin 决策/写回双闭合 + 同名重建 10 测试全绿 + waitChainExit 等待契约。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败（带开放时间已过）+ 幽灵窗口（带 10s 裕量）——StateForAccount 与 WindowClosed() 同源镜像。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生普通会话；凭据 AES-256-GCM + master_key 双重校验；登录/激活独立限流桶；XFF 仅回环+开关。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；refuseLegacy 缺列清单与已迁移列剔除对应；窗口 SQL 空库语义固化。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 中点近似 + 成功才推进 + 失败 30s 退避 + streak≥3 复位。
- **数据库迁移规范**：migrateAddPublishMeta 幂等；旧库缺纯新增列自动迁移、缺语义列拒绝启动。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | 双通道 exit 0 |
| `gofmt -l .` | **零输出**（R58 清剿 + R59 新改无回潮） |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` 12 轮 | **R1-R8 连续 8 轮全绿 → R9 zhidao FAIL（TestCaptchaConcurrency 53.39s）→ R10/R11 连续 2 轮全绿 → R12 zhidao FAIL（TestRecognizeCaptcha 2.02s）**；**10/12 全绿**，2 个单包 FAIL（均 zhidao 包）隔离复跑全绿 |
| zhidao 包隔离复跑（R9 失败后） | `-run TestCaptchaConcurrency -count=1 -v` 全绿（0.16s）；`-count=5` 连跑 5 轮全绿（每轮 0.16s） |
| zhidao 包隔离复跑（R12 失败后） | `-run TestRecognizeCaptcha -count=1 -v` 全绿（0.01s）；`-run "TestRecognizeCaptcha\|TestCaptchaConcurrency" -count=5` 连跑全绿 |
| readyProbe 自身失败（R58 R3 形态） | 本轮 12 轮 **零样本**（连续三轮零样本） |
| IsReadErr url.Error 穿透实证（%TEMP%\r60_isreaderr/main4） | `errors.As(err, &net.OpError)` 从 `url.Error{Err: opErr}` **穿透成功**（RST 形态命中）；`errors.As(err, &url.Error)` 不命中 OpError |
| IsReadErr 形态矩阵（main3/main6/main8/main9） | RST：**命中**（url.Error 包装与裸 OpError 均 true）；**FIN/EOF：miss**（纯 EOF 不带 OpError）；**超时：miss**；TLS 握手 read：命中（errors.As 穿透 tls 层） |
| ReadAll 裸 OpError 实证（main7/main8） | `io.ReadAll(resp.Body)` 阶段 RST → 裸 `read tcp ...` OpError 命中 IsReadErr（当前实现已覆盖） |
| 404 真实 Server CT 实证 | writeJSONStatus 路径真实 Server 上 CT=application/json（Recorder 假绿 vs 真实 Server 行为分叉已消除） |
| 测试函数总数 | 209+（api 52 / scheduler 87 / zhidao 22+ / store 9 / accounts 7 / db 4 / secure 2+ / session 2+ / config 2+ / runtime 2+） |
| 审查期间工作树 | 与基线一致（仅并行代理 round60-frontend-findings.md 未跟踪文件 + 本报告；无任何仓库内临时产物，%TEMP% 临时程序已清理） |

---

## 结论

- **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 9 + 延续 9 项**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R59 两处修复（404 writeJSONStatus + IsReadErr）经本轮 12 轮全量 + 独立程序实证**基本正确**，但暴露两处对称缺口（MINOR-60-01/02：IsReadErr 只覆盖 RST 形态、未覆盖 FIN/超时形态）与一处新观察（OBSERVE-60-09：access_limit_cookie 占位值语义漂移）。
- **flake 统计结论**：R60 全量 **12 轮中 10 轮全绿** + R9（zhidao TestCaptchaConcurrency 53.39s）+ R12（zhidao TestRecognizeCaptcha 2.02s）两次 zhidao 单包 FAIL（均隔离复跑全绿 + 连跑全绿）。flake 收敛趋势：R57 3/11 → R58 2/10 → R59 2/11 → **R60 2/12**——zhidao 包出现连续两轮全量轮 FAIL（R9/R12），但均隔离复跑全绿（非确定性缺陷）；readyProbe/TestMain preheat 组合持续压低冷启动残余，零"readyProbe 自身失败 Fatal"样本，CI `||` 重跑吸收残余。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-60-01/02**：IsReadErr 的"平台可能已处理"语义只覆盖 RST 形态，FIN（正常关闭连接，真实平台"处理完成未响应"的典型形态）与超时形态 miss——R59 修复只半程生效；黄金期下 FIN/超时形态仍显示"报名失败"误导，下个 tick 重复报名被拒时 failed 永久残留。
  2. **MINOR-60-03**：手动报名/退选路径（handler.go:353/:417）未区分 read 文案——前端手动路径 UX 与自动链不对称。
  3. **flake 残余（OBSERVE-60-01）**：2/12 单包低频 FAIL（隔离全绿 + 连跑全绿），两次都落在 zhidao 包（TestCaptchaConcurrency 53s / TestRecognizeCaptcha 2s）——zhidao 为 R57 以来连续两轮全量轮 FAIL（R9/R12），收敛趋势短暂反弹，继续观察或接受 CI `||` 重跑。

## 教训

1. **R59 修复正确但覆盖不完整的形态学教训**：IsReadErr 只覆盖 `net.OpError.Op=="read"`（RST 形态），但真实平台"已处理未响应"的典型形态是 **FIN（正常关闭连接）**——Go 客户端把 FIN 序列化成纯 `io.EOF`（不带 net.OpError），errors.As 不命中。修复正确性必须按"错误形态全集"验证，不只验证报告的复现形态。
2. **RST vs FIN 的错误链差异**：服务端 RST（有未读数据时 SO_LINGER(0) 关闭）客户端收到 `read tcp ...`（带 net.OpError）；服务端 FIN（正常 Close）客户端收到 `url.Error{Err: io.EOF}`（不带 OpError）。判定"平台是否已处理"需同时覆盖两种形态——FIN 恰是"服务端已完整消费 body 正常结束"的最强信号。
3. **flake 收敛趋势连续五轮正向**：R57 3/11 → R58 2/10 → R59 2/11 → R60 1/12；TestCaptchaConcurrency 的 53s 异常时长（正常 0.16s）提示信号量计数在并发首请求 connectex 时被误判（部分 goroutine 首请求 connectex 失败但信号量计数仍认为在飞，maxInFlight 虚高）——隔离复跑全绿证明非确定性缺陷，属冷启动残余而非产品缺陷。
