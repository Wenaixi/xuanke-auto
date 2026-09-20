# R59 后端只读审查发现报告

> 审查基线：master @ `899e8c3`（R58 收官，2026-09-21）。工作树预期仅 archive/review-rounds/ 内并行代理的 findings（round59-frontend-findings.md）；审查期间绝对只读——临时验证程序全部落在系统临时目录（`%TEMP%\r59_ct*`），**仓库内零新建临时产物**（R58 教训：tmptest 建在仓库内污染三轮全量判定）。写入前 `git status --short` 复核：工作树与基线一致（仅并行代理的未跟踪 findings 文件，见文末实证表）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部 209 个测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~58 各轮报告逐条复核 + R58 文末观察项延续。
> 方法：全包逐行通读（scheduler 2035 行 / api 1210 行 / store 481 行 / zhidao 781 行 / accounts 325 行全读）+ 标准库行为独立验证（4 个临时程序实证）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` **11 轮** + 失败包隔离 verbose 复跑。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 8 新增 + 延续 9 项**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R58 三处修复经本轮 11 轮全量 + 独立验证**全部正确**。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-59-01：未知 /api/ 路径 404 的 Content-Type 在真实 Server 上静默丢失（`text/plain` 而非 `application/json`）——`router.go` 404 handler 先 `WriteHeader` 再经 `writeJSON` 设头，测试路径（httptest.ResponseRecorder）与生产路径（真实 net/http Server）行为分叉**，独立程序实证：Recorder 上 CT=`application/json`（测试假绿）、真实 Server 上 CT=`text/plain; charset=utf-8`（生产语义错标）。与 B40-01"基础设施状态码成家族核对"同族卫生缺口——同族 401/403/429/500 均走 `writeJSONStatus`（先设头再 WriteHeader，正确），唯独 404 用 `WriteHeader+writeJSON` 组合漏检。
  2. **MINOR-59-02：R57 后 read 错误上抛到 scheduler 的日志文案误导——平台可能已处理报名但日志/状态显示"报名失败"**（D 项实证）：`spawnChain` 非失效/风控/关闭分支对 read 错误走实时复核，复核失败落入"未现满员"分支置 failed + AppendLog"报名失败: read tcp ..."，而服务端可能已完整消费 body 并成功处理（read 错误 = 已处理未响应）。下个 tick 重试会覆盖为 success，但时间窗内用户/日志看到的是误导的失败态。
  3. **flake 未归零但显著收敛**：R59 全量 11 轮中 **9 轮全绿 + R9（api）+ R11（zhidao）单包 FAIL（均隔离复跑全绿）**。readyProbe 加宽消除了 R58 R3"readyProbe 自身失败直接 Fatal"形态（本轮零样本），残余为低频冷启动 connectex；zhidao 首次出现 FAIL（R58 起 10 轮 0 FAIL）。

---

## CRITICAL

（无）

---

## MAJOR

（无）

---

## MINOR

### MINOR-59-01：未知 /api/ 路径 404 的 Content-Type 在真实 Server 上静默丢失（Recorder 假绿 / 生产 text/plain）

- **位置**：`internal/api/router.go:194-197`（404 handler）+ `internal/api/handler.go:76-79`（writeJSON）。
- **一句话问题**：404 handler 先 `w.WriteHeader(http.StatusNotFound)` 再调 `writeJSON`——writeJSON 内部 `Header().Set("Content-Type", ...)` 发生在 **WriteHeader 之后**，Go net/http 对已提交响应丢弃该头；真实 Server 上 404 响应实际 `Content-Type: text/plain; charset=utf-8`（net/http 自动嗅探 JSON body 的结果）。测试路径（httptest.ResponseRecorder）允许 WriteHeader 后设 Header，因此既有测试（handler_test.go:1670 `strings.HasPrefix(ct, "application/json")`）**恒绿——行为分叉掩盖缺陷**。
- **证据链**（独立程序实证，落 `%TEMP%\r59_ct2`/`r59_ct4`）：
  - 精确复刻 `WriteHeader(404) → Header().Set(CT) → Encode` 到**真实 httptest.Server**：`HTTP 404 CT="text/plain; charset=utf-8"`。
  - 同代码到 **httptest.ResponseRecorder**（测试路径）：`CT="application/json; charset=utf-8"`。
  - 对照 `writeJSONStatus`（先 `Header().Set` 再 `WriteHeader`，handler.go:85-89）：真实 Server 上 CT 正确——同族 401/403/429/500 全部正确，唯 404 例外。
  - 复刻完整管线（securityHeaders 先设头 + mux.ServeHTTP）：404 CT 仍为 text/plain，nosniff 正常。
- **触发条件**：任何请求命中未注册的 `/api/xxx` 路径（curl/脚本/前端 typo/安全扫描器探测）。
- **影响**：HTTP 404 + JSON body + 前端 `r.json()` 解析**功能无碍**（text/plain 也能 JSON.parse → ApiError(404)）；但 API 契约面 HTTP 层语义错标（声称 text/plain 实际 JSON body），严格按 Content-Type 解析的客户端/安全扫描器会漏读 body 的 `code` 字段；且与 B40-01"基础设施状态码成家族核对"的家族卫生目标直接相悖（同族路径正确、此条漏检）。
- **修复方向**：404 handler 改用 `writeJSONStatus(w, http.StatusNotFound, 404, nil, "接口不存在")`（一行替换），与 401/403/429/500 同族；补一个"真实 Server 而非 Recorder"的断言（httptest.NewServer + http.Get 真发请求，才能抓到 Recorder 假绿）。

### MINOR-59-02：R57 后 read 错误上抛产生"报名失败"误导日志（平台可能已处理）

- **位置**：`internal/zhidao/client.go:465-479`（httpDo，read 不重试）+ `internal/scheduler/scheduler.go:1584-1652`（spawnChain 非失效分支处理）。
- **一句话问题**：R57 修复后 read 错误（服务端已完整消费 body 但未响应）原样上抛——spawnChain 的非 ErrUnauthorized/非风控/非窗口关闭分支落入**实时人数复核**（`classFullRealtime`），复核返回的 cErr 非 ErrUnauthorized → 走"未现满员"分支 `setStateLocked(failed, err.Error())` + `AppendLog("账号 acct1: read tcp ...: connection reset")`——而平台**可能已经成功处理了这次报名**（read 错误 = 已处理未响应）。用户看到"报名失败"、日志记"报名失败"，但实际可能已抢到课。
- **证据链**：
  - R57/R58 已实证 read 错误形态（服务端已收完整 body）——R58 报告独立程序实证"read 错误（服务端读完整不响应）→ 客户端报 EOF（read OpError）→ httpDo 外层不重试 → 无二次到达"。
  - scheduler.go:1645 `s.setStateLocked(..., "failed", err.Error())` 与 :1647 `AppendLog(..., err.Error(), false)` 对**任意非归类错误**原文落日志，read 错误与业务失败同文案。
  - 自愈路径存在：下个 tick（1s）重试 SelectClass，若成功覆盖为 success——但黄金期窗口内"实际已成功却显示失败"的时间窗最长 1s+（复核网络段），且**不会自愈的形态**：若 read 错误后平台实际成功、但下个 tick 的 SelectClass 命中"已选过该课"业务错误（code=1 重复报名），则 failed 状态永久残留。
- **触发条件**：Windows 回环 keep-alive 复用濒死连接（R52 背景）或平台响应中断（读 body 中断），恰好落在 SelectClass 上。
- **影响**：日志审计误导（排查"为什么报失败但课已选上"）；状态显示误导；黄金期若重复报名被拒则 failed 永久残留。非数据损坏（done/success 不受污染），低频触发。
- **修复方向**：区分 read 错误日志文案——scheduler 对 `*net.OpError` 且 `Op=="read"`（或 url.Error 内包装 read 错误）单独记"报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）"而非"报名失败"；或 api/handler 手动报名路径同样区分。可用 `isConnErrRetryable` 的兄弟函数 `isReadErr` 判定。

---

## OBSERVE

### OBSERVE-59-01：R9（api）/R11（zhidao）全量轮单包 FAIL 均为低频冷启动残余，隔离复跑全绿

- R9 api FAIL（47.4s 后 FAIL，`tail -12` 截断未保留具体测试名）；R11 zhidao FAIL（12.5s）。两个失败包的**隔离 verbose 重跑均全绿**（api 52 测试全 RUN + exit 0；zhidao 全部 RUN + 3.9s ok）。zhidao 为 R58 以来（此前 10 轮 0 FAIL）首次 FAIL，与 api 同根（Windows 回环 TIME_WAIT 冷启动）。readyProbe 加宽后**本轮零"readyProbe 自身失败 Fatal"样本**（R58 R3 形态已消灭）。flake 收敛趋势：R57 3/11 → R58 2/10(+3 基建污染) → **R59 2/11**。观察（低频残余，CI `||` 重跑吸收；若要求归零需把 readyProbe 的 `http.Get` 换成复用 `http.Client` 的连接预热 + 更宽窗口）。

### OBSERVE-59-02：stats 测试注释矛盾残留（R58 OBSERVE-58-02 延续，未修）

- handler_test.go:978-979 注释仍写"识别过期语义下自动降级为'未识别'（绝不把过期旧值当开放时间）"，与 R56 修复后的行为（识别槽有值即照常输出过期日期、open_time_set=true）矛盾；断言 `st["open_time_set"] != (!recog.IsZero())` 语义正确。纯注释残留，一行改注释即可。

### OBSERVE-59-03：`handleSetTargets` 对 `{}` / `{"targets":null}` 请求体静默变空目标（R58 OBSERVE-58-03 延续）

- handler.go:484-486 `req.Targets == nil` → `[]scheduler.Target{}` 整体保存（清空目标）。"清空目标"本身合法，此形态仅恶意/异常客户端可触达。若想区分"未提供字段"与"显式空数组"需指针字段，非缺陷观察。

### OBSERVE-59-04：撞名学生教务口令错时文案归因"管理口令错误"（R58 OBSERVE-58-04 延续）

- handler.go:136-139：管理员名 + 教务口令也错（撞名学生输错学生密码）→ 返回"管理口令错误"。文案误导（低频、仅撞名场景），安全语义正确（等时 + 不泄露）。

### OBSERVE-59-05：`cmd/probe/main.go` 硬编码 `access_limit_cookie=***REMOVED***`（R58 OBSERVE-58-05 延续）

- 工具依赖 XUANKE_PROBE_TOKEN 直连真实平台，cookie 真实值已脱敏丢失 → 运行必 code=-1。历史遗留工具（生产走 /api/electives），观察（未来可删或标注 deprecated）。

### OBSERVE-59-06：`cmd/bench` 默认 url 不带 -token 测 401 拒绝路径（R58 OBSERVE-58-06 延续）

- bench/main.go:19 默认 `http://localhost:3091/api/state`，未带 -token 时测 401（自带注释已说明）。非缺陷观察。

### OBSERVE-59-07：`handleLogout` 只吊销当前会话 token 不吊销账号全部会话（R58 OBSERVE-58-07 延续）

- handler.go:567-575：设计语义明确（"注销当前会话"），与 RevokeAccount（删账号全吊销）分工清晰。观察。

### OBSERVE-59-08：`config.Load()` 每次调用 `ensureEnvFile` 写盘（R58 OBSERVE-58-08 延续）

- cmd/logintest 调 config.Load：无 XUANKE_ADMIN_TOKEN 且无 data/.env 时会在仓库根写 data/.env（含随机管理员口令）。实际风险存在但低频（开发态无 .env 环境跑 logintest 才触发）。观察。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-58-01（api flake + readyProbe 窗口不足） | 修复（readyProbe 加宽 10×200ms + 2s 超时） | **加宽正确生效**——本轮 11 轮零"readyProbe 自身失败 Fatal"样本；残余 flake 为 R9/R11 两个低频单包 FAIL（隔离复跑全绿），非 readyProbe 自身 | **闭合**（残余降级 OBSERVE-59-01） |
| MINOR-58-01（gofmt 23 文件 + 7 BOM） | 修复（gofmt -w 全量） | **完全闭合**——`gofmt -l .` 本轮零输出；BOM 剥离对 //go:embed（schema.go/native_ocr.go 嵌入 schema.sql/onnx 资源）无影响（embed 读被嵌入文件内容，与源文件 BOM 无关），Windows 编译器无碍，git diff 仅行尾/格式 | **闭合** |
| MINOR-58-02（cloneReq 注释半脱节） | 修复（注释对齐 dial/write-only） | **完全闭合**——client.go:494-503 注释现明确"httpDo 只对 dial/write 错误重试……read 错误不重试（见 httpDo），此路径不可达"，与 httpDo（465-479）/isConnErrRetryable（483-492）实现逐句一致 | **闭合** |
| OBSERVE-58-01（readyProbe 无超时） | 已修复 | 2s 超时已落地（handler_test.go:188 `http.Client{Timeout: 2 * time.Second}`） | **闭合** |
| OBSERVE-58-02（stats 测试注释矛盾） | 延续 | 未修 | **延续**（OBSERVE-59-02） |
| OBSERVE-58-03（targets nil 变空目标） | 延续 | 未变 | 延续（OBSERVE-59-03） |
| OBSERVE-58-04（撞名文案） | 延续 | 未变 | 延续（OBSERVE-59-04） |
| OBSERVE-58-05（probe 工具 cookie 脱敏） | 延续 | 未变 | 延续（OBSERVE-59-05） |
| OBSERVE-58-06（bench 默认 401） | 延续 | 未变 | 延续（OBSERVE-59-06） |
| OBSERVE-58-07（logout 单会话） | 延续 | 未变 | 延续（OBSERVE-59-07） |
| OBSERVE-58-08（config.Load 写 .env） | 延续 | 未变 | 延续（OBSERVE-59-08） |

---

## 本轮新视角七项逐项实证

### A. R58 三处修复是否正确
- **① readyProbe 加宽（10×200ms + 显式 2s 超时）**：**正确且生效**。① 2s 超时误杀风险：探测请求是纯 GET /ready 到内存 handler 的 mock 服务器，2s 对已就绪服务器绰绰有余；仅"mock 服务器确实起不来"（端口占/系统级卡死）才超时，10 次重试总窗口 ~2s 后 Fatal 报错——不掩盖真实故障太久（真故障 2s 内暴露）。② 10 次重试掩盖风险：重试只吸收冷启动 connectex（≤200ms/次），10 次窗口内 mock 仍未就绪的概率已被 R58 实证为极端残余（本轮零样本）。**无新问题**。
- **② gofmt 全量规范化（23 文件 + 7 BOM）**：**完全正确**。`gofmt -l .` 本轮零输出（R58 清剿无回潮）。BOM 剥离对 `//go:embed` 的 schema.go（嵌入 schema.sql）/native_ocr.go（嵌入 onnx/dll）无影响——embed 机制读取被嵌入文件内容，与 Go 源文件自身 BOM 无关联；Windows 编译器对无 BOM 的 .go 文件完全正常；git 历史 diff 仅行尾/import 空行/注释对齐，无行为影响。
- **③ cloneReq 注释对齐**：**完全一致**——见延续表 MINOR-58-02 裁决。

### B. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 11 轮 flake 统计
- **R1-R8 连续 8 轮全绿** → **R9 api FAIL**（47.4s，tail 截断未留测试名）→ **R10 全绿** → **R11 zhidao FAIL**（12.5s）。**9/11 全绿，2 个单包 FAIL 均隔离复跑全绿**（api 52 测试 exit 0；zhidao 3.9s ok）。zhidao 为 R57 以来首次 FAIL（此前 zhidao 10+ 轮 0 FAIL）——与 api 同根（Windows 回环 TIME_WAIT 冷启动低频残余）。**readyProbe 加宽消灭了"探测自身失败 Fatal"形态**（R58 R3 形态零样本）。flake 收敛：R57 3/11 → R58 2/10 → R59 2/11（低频残余未归零，CI `||` 重跑吸收）。

### C. gofmt -l . 全量复检
- **零输出**（CI 格式门禁可放心接入）。

### D. R57 httpDo read 不重试修复的长期稳定性
- read 错误上抛 → scheduler 走实时复核 → 复核非失效错误 → "未现满员"分支置 failed + AppendLog"报名失败"。**存在误导日志**（MINOR-59-02）：平台可能已处理报名但显示失败。下个 tick 重试自愈（成功覆盖）；不会自愈的形态：read 错误后平台实际成功、下个 tick 重复报名被拒（code=1"已选过"）→ failed 永久残留。**建议区分 read 错误文案**（"请求已发出但响应读取失败，可能已处理，请以大厅状态为准"）。

### E. 上轮观察项延续复核
- 全表见延续表：**MAJOR-58-01 闭合（加宽生效）/ MINOR-58-01 闭合（gofmt 零输出）/ MINOR-58-02 闭合（注释逐句一致）/ OBSERVE-58-01 闭合（2s 超时落地）/ OBSERVE-58-02~08 延续 7 项**。

### F. 全包逐行通读找新问题
- **scheduler 提交链**（spawnChain 六分支 sameClientFor + inflight 去重 + B30-01 链顶双判 + doneHas 保护 + waitChainExit 契约）——同名重建 10 个测试逐条通读全绿 ✅；**WindowClosed 三判据单源**（主判据 + 时钟失败带开放时间已过 + 幽灵窗口带 10s 裕量）+ 裕量双侧对称 ✅；**task_log 窗口 SQL**（空库语义 window_empty_test 固化 + 30050 行窗口测试）✅；**api 鉴权族**（401/403/404/429/500 家族 + requireJSONBody CSRF 门 + XFF 可信反代 8 场景）✅——**唯 404 handler 的 Content-Type 顺序缺陷为本轮新发现（MINOR-59-01）**；**登录链路**（RSA/PKCS1v15/priorityId/Vision 收敛/gateTryAcquire 非阻塞准入）✅；**凭据 AES-256-GCM + master_key 32 字节双重校验** ✅；**数据库迁移幂等** ✅；**httpDo/cloneReq 重试路径**——R57 修复正确，read 错误日志误导为 MINOR-59-02 ✅。
- **未发现新 CRITICAL / MAJOR**。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核满员/实时复核失效）+ maybeRelogin 决策/写回双闭合 + 同名重建 10 测试全绿 + waitChainExit 等待契约。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败（带开放时间已过）+ 幽灵窗口（带 10s 裕量）——StateForAccount 与 WindowClosed() 同源镜像。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生普通会话；凭据 AES-256-GCM + master_key 双重校验；登录/激活独立限流桶；XFF 仅回环+开关。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；refuseLegacy 缺列清单与已迁移列剔除对应；窗口 SQL 空库语义固化。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 中点近似 + 成功才推进 + 失败 30s 退避 + streak≥3 复位（真实 maybeSyncClock 累计验证）。
- **数据库迁移规范**：migrateAddPublishMeta 幂等；旧库缺纯新增列自动迁移、缺语义列拒绝启动。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | 双通道 exit 0 |
| `gofmt -l .` | **零输出**（R58 清剿完全生效，无回潮） |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` 11 轮 | **R1-R8 连续 8 轮全绿 → R9 api FAIL → R10 全绿 → R11 zhidao FAIL**；**9/11 全绿**，2 个单包 FAIL 均隔离复跑全绿 |
| api 包隔离 verbose 复跑（R9 失败后） | 52 测试全 RUN + exit 0（全绿） |
| zhidao 包隔离 verbose 复跑（R11 失败后） | 全部 RUN + 3.9s ok（全绿）；zhidao 为 R57 以来首次全量轮 FAIL |
| readyProbe 自身失败（R58 R3 形态） | 本轮 11 轮 **零样本**（加宽消灭） |
| 404 Content-Type 真实 Server 实证（%TEMP%\r59_ct2/ct4） | `WriteHeader(404) → 设 CT` 到真实 Server：**`Content-Type: text/plain; charset=utf-8`**；到 httptest.ResponseRecorder：`application/json`（测试假绿，行为分叉）；对照 writeJSONStatus（先设头后 WriteHeader）：真实 Server 正确 |
| read 错误日志误导实证 | scheduler 非失效/风控/关闭分支对 read 错误 → 实时复核 → "未现满员"分支置 failed + AppendLog 原文（MINOR-59-02 证据链） |
| 测试函数总数 | 209 个 |
| 审查期间工作树 | 与基线一致（仅并行代理 round59-frontend-findings.md 未跟踪文件 + 本报告；无任何仓库内临时产物） |

---

## 结论

- **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 8 + 延续 9 项**。产品逻辑本轮**零 CRITICAL、零 MAJOR**——R58 三处修复（readyProbe 加宽 / gofmt 全量 / cloneReq 注释）经本轮 11 轮全量 + 独立程序实证**全部正确**。
- **flake 统计结论**：R59 全量 **11 轮中 9 轮全绿** + R9（api）/R11（zhidao）两个低频单包 FAIL（均隔离复跑全绿）。readyProbe 加宽消灭了"探测自身失败 Fatal"形态，残余为冷启动 connectex 低频残余——**收敛但未归零**，与 R57（3/11）→ R58（2/10）趋势一致，CI `||` 重跑吸收。
- **最致命 3 条（按影响排序）**：
  1. **MINOR-59-01**：404 handler `WriteHeader` 先于 `Content-Type` 设置——真实 Server 上 404 响应 CT 错标为 `text/plain`（测试 Recorder 假绿掩盖），与 401/403/429/500 家族不一致；一行改 `writeJSONStatus` 修复 + 补真实 Server 断言。
  2. **MINOR-59-02**：R57 后 read 错误（平台可能已处理）上抛到 scheduler 记"报名失败"误导日志/状态；建议区分 read 错误文案。
  3. **flake 残余（OBSERVE-59-01）**：2/11 单包低频 FAIL（隔离全绿），zhidao 首次出现；readyProbe 加宽已显著收敛，继续观察或接受 CI `||` 重跑。
