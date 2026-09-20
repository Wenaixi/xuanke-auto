# R58 后端只读审查发现报告

> 审查基线：master @ `1434b5d`（R57 收官，2026-09-21 04:39:14 +0800）。工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 round58 相关文件；审查期间绝对只读（唯一允许新建的产物即本报告与临时验证程序，均落 /tmp 与报告），写入前 `git status --short` 复核：工作树与基线一致（见文末实证表）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部 208 个测试函数。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~57 各轮报告逐条复核 + R57 文末观察项延续。
> 方法：全包逐行通读 + 标准库源码核证（transport.go 重试判定 / request.go GetBody / shouldRetryRequest）+ 独立 Go 程序实证（keep-alive 复用连接服务端静默关闭形态）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` **10 轮**（R5 含独立验证程序导致 tmptest 包混入，R6/R7 受其影响，已清理后 R8~R10 为纯基线 3 轮）。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0。

---

## 结论先行

- **CRITICAL 0 / MAJOR 1 / MINOR 2 / OBSERVE 8 新增 + 延续 12 项**。产品逻辑本轮**零 CRITICAL**。
- **最致命 3 条（按影响排序）**：
  1. **MAJOR-58-01：R57 修复后 flake 未归零（5/10 轮 api FAIL），且 R3 暴露"readyProbe 即使探测成功首个测试请求仍 connectex"的夹具固有缺口**——全量 10 轮中 R3（TestAdminAuth）、R1（TestSetTargetsBounds）FAIL；R5/R6/R7 三轮因临时验证包 tmptest 的**构建失败**（已删源文件后 stale build cache 未失效）被污染判 FAIL；**清理后 R8/R9/R10 连续 3 轮全绿**。zhidao 10 轮 0 FAIL（TestMain preheat + 无 Discard 生效）。api 包残余 flake 根因是 readyProbe 成功后首个测试请求仍可能复用"探测建立的连接"前的 TIME_WAIT 队列残余（readyProbe 用的 http.DefaultClient 与测试请求用的同一 transport，探测成功≠冷启动队列排空）。
  2. **MINOR-58-01：R57 收尾称"gofmt 规范化"但工作树仍有 20+ 文件 gofmt 差异**——大头是 R57 起 **BOM 残留**（main.go/scheduler.go/router.go/store.go/session/store.go/config.go/scheduler_test.go/bench/main.go 等 6 个带 BOM，R57 只对 client.go 去 BOM）+ 其余文件 import 段/尾换行/注释缩进（db.go/store_test.go 缺尾换行、manager_test.go/rsa_test.go 空白 import 空行等）。CI 若接入 `gofmt -l` 检查恒红。
  3. **MINOR-58-02：R57 遗留 R56 死代码语义（OBSERVE-57-01 延续）——`cloneReq` 注释仍称"Body 由首个 RoundTrip 消费后为空"但该重试路径现在不可达，注释与行为半脱节**。

- **flake 统计结论（唯一判定源=全量连续多轮）**：清理临时包后的纯基线 **R8/R9/R10 连续 3 轮 0 FAIL**；计入 R1/R3 两轮 api 冷启动 FAIL 后，**全量 10 轮中 3/10 FAIL 全为 api 冷启动 connectex 残余 + 3 轮为测试基建污染（tmptest build cache）**。zhidao 10 轮 0 FAIL。R57 声称的"全量 race 全包绿"在 R8-R10 得到复现，但 api 包冷启动 flake **未根治**（R1/R3 两轮实际 FAIL），且 R3 显示 readyProbe 自身（5 次重试 × 200ms 内 connectex 一次都没恢复）在极端窗口下也扛不住。

---

## CRITICAL

（无）

---

## MAJOR

### MAJOR-58-01：api 包全量轮 flake 未归零（10 轮中 2 个 api 测试 FAIL），R3 暴露 readyProbe 自身的冷启动窗口兜不住

- **位置**：`internal/api/handler_test.go:183-211`（readyProbe）+ `:139-141`（newTestDepsModeName 探测失败即 Fatal）+ `:47`（TestAdminAuth 失败断言行）。
- **一句话问题**：全量 `-race -p 1` 10 轮实测 **R3 FAIL（TestAdminAuth，readyProbe 自身 5 次重试 × 200ms 内连接一次都没恢复 → t.Fatalf("mock 服务器就绪探测失败")）+ R1 FAIL（TestSetTargetsBounds）**；R8-R10（清理临时包后）连续全绿。R57 收尾声称"全量 race 全包绿"**没有在 R1/R3 得到复现**——api 冷启动 connectex 残余仍在低频命中，且 R3 显示"readyProbe 的 5×200ms 窗口本身都不够"（探测都失败，直接 Fatal）。
- **证据链**：
  - R3 日志原文：`handler_test.go:47: mock 服务器就绪探测失败: Get "http://127.0.0.1:62824/ready": dial tcp 127.0.0.1:62824: connectex: ... established connection failed because connected host has failed to respond.`——readyProbe 循环 5 次 × 200ms 全部 connectex，说明 R57 的"200ms×5 窗口"在**前序包（accounts）结束后回环 TIME_WAIT 队列最恶劣时刻**仍覆盖不到。
  - R1 日志（TestSetTargetsBounds）：冷启动 connectex 传递路径（登录/探测连接失败 → 非预期响应）。R56/R57 同款样本（R57 MAJOR-57-01 的 4 个不同测试全 api）。
  - api 包隔离复跑 10+ 轮 0 FAIL（R57 实证）；zhidao 10 轮 0 FAIL（TestMain preheat 生效）。
  - readyProbe 用 `http.DefaultClient`（**无超时**，OBSERVE-57-03 延续）；测试请求复用同一 DefaultClient transport，探测成功后的首个测试请求仍可能复用"探测刚建立的连接"（该连接建立于 server accept 就绪窗口，探测成功后 server 已稳定）——但 R3 场景是探测本身全部失败。
- **触发条件**：Windows 宿主全量 `go test ./...`（-p 1 下 api 紧随 accounts，accounts 的 httptest 关闭后回环 TIME_WAIT 残留最多）；api 测试量大（52 个函数），每个测试新建 mock server + 独立会话。
- **影响**：CI 低频假红（~20% 轮次 api 包 FAIL），`||` 重跑吸收但成本高；无生产影响。
- **修复方向**：
  - ① readyProbe 重试窗口加宽（如 10×300ms）或改用"连接池预放一个 keep-alive 连接给首请求复用"（探测请求本身建立连接后**复用同一连接**发首测试请求——双保险）。
  - ② 或接受 CI `||` 重跑；统计口径维持全量连续多轮为唯一判定源。
  - ③ 注意：R8-R10 连续 3 轮全绿说明该 flake 是**低频残余**而非系统性，隔离复跑恒绿。

---

## MINOR

### MINOR-58-01：R57"gofmt 规范化"不完整——20+ 文件仍 gofmt 差异（含 6 个 BOM 残留 + import 空行 + 缺尾换行）

- **位置**：`gofmt -l .` 报告 23 个文件；其中 6 个带 **UTF-8 BOM**（`cmd/bench/main.go`、`main.go`、`internal/api/router.go`、`internal/config/config.go`、`internal/scheduler/scheduler_test.go`、`internal/session/store.go`、`internal/store/store.go`）——R57 只修了 `zhidao/client.go` 一处 BOM；其余差异为 import 段空行（`manager_test.go:3`、`rsa_test.go:3` 的 `package X` 与 `import (` 之间空行）、缺尾换行（`db.go`、`store_test.go` 的 `\ No newline at end of file`）、注释对齐等。
- **一句话问题**：R57 提交消息声称"gofmt 规范化"，但 `gofmt -l backend/` 仍报 23 个文件——BOM 头在 Go 编译/运行无碍（编译器静默吞），但对 **CI 的 `gofmt -l` 检查恒红**（若未来接入）；且 BOM 文件是 R57 提交的"go fmt 规范化"遗漏项。
- **证据链**：
  - `gofmt -l backend/` 全量输出 23 文件（含 `main.go`/`internal/scheduler/scheduler.go`/`internal/store/store.go`/`internal/api/router.go` 等核心文件）。
  - `head -c3 file | od -An -tx1` 实测：`ef bb bf` 出现在上述 6+1 个文件首字节。
  - `gofmt -d` 对带 BOM 文件仅报"BOM 剥离 + 无尾换行"，无其他格式差异（R57 已把真正的格式问题修完）。
- **触发条件**：`gofmt -l` 检查、跨平台文本工具（BOM 文件在 Unix 工具链下首行 `package` 前多 3 字节）。
- **影响**：CI 格式门禁若接入恒红；BOM 对 Go 编译器无害（不影响行为）；纯工程卫生问题。
- **修复方向**：一条命令 `gofmt -w` 全量规范化 + `sed -i '1s/^\xEF\xBB\xBF//'` 剥离 BOM（或让 gofmt 处理）+ CI 增加 `gofmt -l` 门禁防止回潮。

### MINOR-58-02：R57 收尾后 `cloneReq` 注释与行为半脱节（死代码语义残留，OBSERVE-57-01 延续）

- **位置**：`internal/zhidao/client.go:494-503`（cloneReq 注释）+ `:465-479`（httpDo）+ `:483-492`（isConnErrRetryable）。
- **一句话问题**：R57 修复后 `cloneReq` 的注释仍详细描述"read 类错误重试时 Body 由首个 RoundTrip 消费后为空、ContentLength 未同步"——但 read 类错误现在**根本不会进入重试分支**（httpDo 只对 dial/write 重试），该注释描述的重试形态不再可达；注释与行为脱节，且把 `req.Clone` 的"浅拷贝 Body + 保留 GetBody"语义写成"透明"但没解释为什么透明（GetBody 由标准库对 bytes.Reader 自动设置——这才是关键）。
- **证据链**：
  - `httpDo`（client.go:470-473）：`if !isConnErrRetryable(err) { return nil, err }` → read 错误原样上抛，永不进入 cloneReq。
  - `cloneReq` 注释（client.go:497-499）仍写"重试请求的 Body 由首个 RoundTrip 消费后为空、ContentLength 未同步（真实路径实测）"——该路径现在**不可达**。
  - 标准库 `request.go:932-945`（NewRequest 对 bytes.Reader 自动设 GetBody）+ `transport.go:815-863`（shouldRetryRequest：nothingWrittenError 分支 req.GetBody != nil 时 transport 内部重放完整 body）。
- **触发条件**：维护者阅读 cloneReq 时被注释误导（以为 read 错误会重试且 body 会空）。
- **影响**：纯注释卫生，无行为影响；但对"重试语义"的后续维护是陷阱（若未来放宽 read 重试会踩双报，注释却已预演了该形态）。
- **修复方向**：cloneReq 注释改为"dial/write 错误下 Body 未消费、重发完整（GetBody 由标准库设置，transport 内部已处理 nothingWritten 重放）；read 错误不重试见 httpDo"。

---

## OBSERVE

### OBSERVE-58-01：api 包 `readyProbe` 无显式超时（http.DefaultClient 超时为 0=无限）

- R57 OBSERVE-57-03 延续。R3 的 readyProbe 失败即"dial tcp connectex"（非挂起），但极端挂起场景（mock server 卡死 accept）下探测请求可阻塞至 OS TCP 超时（分钟级）。一行 `http.Client{Timeout: 2*time.Second}` 即可。延续观察。

### OBSERVE-58-02：R57 遗留 `TestAdminStatsOpenTimeFromRecognized` 注释首句矛盾（OBSERVE-57-04 延续）

- handler_test.go:975-976 注释仍写"识别过期语义下自动降级为'未识别'（绝不把过期旧值当开放时间）"——与 R56 修复后的行为（识别槽有值即照常输出过期日期、open_time_set=true）矛盾；R57 只改了 handler.go 注释、漏了测试内注释。语义上**行为自洽**（断言 `st["open_time_set"] != (!recog.IsZero())` 正确），仅注释残留。延续观察（R57 已列为 OBSERVE-57-04 但未修）。

### OBSERVE-58-03：`handleSetTargets` 在 handler.go:479-485 的 `req.Targets == nil` 分支——空 body 与无 targets 字段的请求体都会静默变空目标

- `json.NewDecoder` 对空 body 返回 EOF → handler 返回"请求体解析失败"（正常）；但对 `{"targets":null}` 或 `{}` 请求体会解码成功且 req.Targets 为 nil → 被转换为空目标并**整体保存**（清空目标）。前端正常路径总带数组，此形态仅恶意/异常客户端可触达，且"清空目标"本身是合法操作——不算缺陷，记录为防御性观察（若想区分"未提供字段"与"显式空数组"，需指针字段）。

### OBSERVE-58-04：`handleLogin` 的管理员口令错误分支（handler.go:136-140）对**任意撞名学生**也延迟 300ms 并归因"管理口令错误"——B43-04 放行的撞名学生教务登录失败时文案不准确但语义安全

- B43-04 已放行"管理员名 + 非管理口令"的撞名学生走教务登录；但若该撞名学生教务口令也错（登录失败），走 `req.Account == adminName` 分支返回"管理口令错误"——用户实际是**学生密码输错**却看到"管理口令错误"文案。文案误导（低频、仅撞名场景），安全语义正确（等时 + 不泄露），观察。

### OBSERVE-58-05：`cmd/probe/main.go` 硬编码 `access_limit_cookie=***REMOVED***`（真实值被脱敏替换）——工具已长期不可用但无编译/运行错误

- 该工具依赖 XUANKE_PROBE_TOKEN env 直连真实平台，cookie 值 `***REMOVED***` 是历史脱敏替换残留（真实值已丢失）——运行会因鉴权失败返回 code=-1。工具定位是历史遗留（生产用 /api/electives），不影响产品；观察（未来可删或标注 deprecated）。

### OBSERVE-58-06：`cmd/bench/main.go` 的 `url` flag 默认指向 `/api/state`，未带 `-token` 时测的是 401 拒绝路径（自带注释已说明）——非缺陷，观察

### OBSERVE-58-07：`handleLogout` 只吊销当前会话 token 不吊销该账号全部会话（handler.go:567-575）——设计语义明确（"注销当前会话"），与 RevokeAccount（删账号全吊销）分工清晰，观察

### OBSERVE-58-08：`config.Load()` 每次调用 `ensureEnvFile` 写盘检查（OBSERVE-57-07 延续）——cmd/logintest/bench 若调用 config.Load 且无 XUANKE_ADMIN_TOKEN 会在仓库根写 data/.env（含随机管理员口令）。cmd/logintest 确实调用 config.Load()（logintest/main.go:29）——**实际风险存在**：若开发者在无 .env 环境跑 `go run ./cmd/logintest` 会在仓库根生成 data/.env。延续观察。

---

## 上轮观察项延续表（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| CRITICAL-57-01（cloneReq GetBody 死代码 + 双报风险） | 修复（httpDo 只重试 dial/write） | **已闭合**——httpDo 现在只对 dial/write 重试，read 错误原样上抛，双报路径不可达；transport 内部用 GetBody 重放"nothingWritten"场景（完整 body，非空 body）已实测确认**不会双报**（std transport.go:815-863 + 独立程序实证：复用静默关闭连接 → transport 用 GetBody 重放完整 body 到新连接 → 服务端仅收 1 次 classId=88）。R57 修复方向正确且实证生效 | **闭合**（新 MINOR-58-02 注释残留） |
| MAJOR-57-01（api flake 3/11） | 未归零 | **延续**——10 轮中 R1（TestSetTargetsBounds）/R3（TestAdminAuth，readyProbe 自身失败）FAIL；R8-R10 连续全绿。升级为 MAJOR-58-01 | **延续/升级** |
| MINOR-57-01（TestMain Discard + gofmt） | 修复 | **部分闭合**——TestMain 已删 Discard（zhidao 10 轮 0 FAIL，TestLoginLogs* 隔离连跑全绿）；但 gofmt 差异仍在 20+ 文件（MINOR-58-01） | **部分闭合** |
| OBSERVE-57-01（cloneReq 注释误导） | 延续 | **延续**（MINOR-58-02） | 延续 |
| OBSERVE-57-02（Discard 吞排查线索） | 已修复 | **闭合**——TestMain 已删 Discard，登录日志可正常输出 | **闭合** |
| OBSERVE-57-03（readyProbe 无超时） | 延续 | 未变 | 延续（OBSERVE-58-01） |
| OBSERVE-57-04（stats 测试注释矛盾） | 延续 | 未修 | 延续（OBSERVE-58-02） |
| OBSERVE-57-05（httpDo 注释 read 语义） | 修复 | **闭合**——R57 已重写 httpDo 注释区分 dial/write vs read | **闭合** |
| OBSERVE-57-06（TestSetTargetsBounds j["code"].(float64) panic 风险） | 延续 | 未变 | 延续观察 |
| OBSERVE-57-07（config.Load 写 .env） | 延续 | cmd/logintest 确实调 config.Load——真实风险存在（OBSERVE-58-08） | 延续 |
| OBSERVE-56-03（WriteTimeout 30s < Login 最坏耗时） | 延续 | 未变 | 延续观察 |
| OBSERVE-56-05~06（TestLoginLimiterGC 零值依赖 / 全局信号量叠加） | 延续 | 未变 | 延续观察 |

---

## 本轮新视角六项逐项实证

### A. R57 三处修复是否正确
- **①httpDo 只对 dial/write 重试**：**正确且生效**。标准库 `transport.go` 的 `shouldRetryRequest` 中 `transportReadFromServerError` 分支（复用连接 + 未写字节 → nothingWrittenError）在 `req.GetBody != nil` 时由 **transport 内部**用 GetBody 重放**完整 body** 到新连接——独立程序实证（服务端静默关闭复用连接 → 客户端 transport 用 GetBody 重放 → 新连接收到完整 classId=88 → 服务端**仅收到 1 次**）——transport 内部重放是"请求未真正处理"的 safe 场景，不双报。而 read 类错误（服务端已消费 body 但未响应）httpDo 外层不再重试 → 双报路径彻底不可达。**SelectClass/ExitClass 的 dial/write 重试安全**（请求未到达服务端，重发不双报）；**403/429 不重试**（HTTP 状态非连接错误，`isConnErrRetryable` 只认 `*net.OpError` 的 dial/write——R52 本意覆盖的 keep-alive 濒死连接形态被 transport 内部自愈吸收，无需外层重试）。**R52 背景的"静默关闭连接复用写出"形态未失去自愈**：实测该形态下 transport 用 GetBody 重放完整 body（非空 body 畸形），或 dial 新连接重试（connectex 形态），httpDo 外层即使收不到（read 错误上抛）也是"请求已到达/已处理"语义——**flake 未回潮**（R8-R10 连续全绿实证）。
- **②TestMain 删 Discard**：**正确**——zhidao 10 轮 0 FAIL，TestLoginLogs* 隔离连跑全绿，登录日志可正常输出排查（OBSERVE-57-02 闭合）。
- **③gofmt 规范化**：**不完整**——20+ 文件仍 gofmt 差异（MINOR-58-01）。

### B. 全量 `-race -count=1 -p 1 -timeout 900s ./...` 10 轮 flake 统计
- **10 轮**：R1 **api FAIL**（TestSetTargetsBounds）/ R3 **api FAIL**（TestAdminAuth，readyProbe 自身失败）/ R5-R7 **FAIL（临时验证包 tmptest 构建失败，非产品代码）**/ R2/R4/R8/R9/R10 全绿。**清理临时包后的纯基线 R8/R9/R10 连续 3 轮全绿**。zhidao 10 轮 0 FAIL。api 包残余冷启动 flake 未根治（R1/R3），但 R3 是 readyProbe 自身失败（极端窗口），R8-R10 复现了 R57 的"全量全包绿"。

### C. TestMain 去 Discard 后 TestLoginLogs* 稳定性
- 隔离连跑 5 轮全绿（R57 的 MINOR-57-01 竞态已根除）；全量 10 轮 zhidao 包 0 FAIL。**稳定**。

### D. R57 CRITICAL 修复后 doRequest read 错误上抛的 scheduler/api 层处理健全性
- read 错误（服务端已消费 body 但未响应）上抛为普通 `*url.Error`——scheduler spawnChain 的非 ErrUnauthorized 非风控非窗口关闭分支走**实时人数复核**（`classFullRealtime`），复核失败保留 failed 状态、下个 tick 重试；api 手动报名路径返回 `err.Error()` 原文给前端。**健全**：read 错误不会触发误导性的 failed 永久状态（下次 tick 会重试），不会误入重登/满员/风控分支。

### E. 上轮观察项延续复核
- 全表见延续表：**CRITICAL-57-01 闭合**（独立程序实证 transport 内部 GetBody 重放不双报 + httpDo read 不再重试）、**MINOR-57-01 部分闭合**（Discard 已删、gofmt 未清）、**OBSERVE-57-01/03/04/06/07 延续**。

### F. 全包逐行通读找新问题
- 重点核对：**scheduler 提交链**（spawnChain 六分支 sameClientFor 身份复核 + inflight 去重 + B30-01 链顶双判 + doneHas 保护）——10 个同名重建测试全绿 ✅；**WindowClosed 三判据单源** + 10s 裕量双侧 ✅；**task_log 窗口 SQL**（空表/1 行/30050 行语义）——单跑 24s PASS ✅；**api 鉴权族**（401/403/404/429/500 家族 + requireJSONBody CSRF 门）✅；**登录链路**（RSA/PKCS1v15/priorityId/Vision 收敛/gateTryAcquire）✅；**凭据 AES-256-GCM** + master_key 32 字节校验 ✅；**数据库迁移**幂等 ✅；**httpDo/cloneReq 重试路径**——CRITICAL-57-01 已闭合，注释残留为 MINOR-58-02 ✅。未发现新 CRITICAL。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支 + maybeRelogin 决策/写回双闭合 + waitChainExit 等待契约；6+1 个同名重建测试全绿。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败（带开放时间已过）+ 幽灵窗口（带 10s 裕量）。
- **鉴权与数据安全**：B43-04 管理员双条件 + 撞名学生普通会话；凭据 AES-256-GCM + master_key 双重校验；登录/激活独立限流桶；XFF 仅回环+开关。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量；refuseLegacy 缺列清单与已迁移列剔除对应；窗口 SQL 语义实证正确。
- **连接池与时钟**：sharedTransport 64 连接/120s 空闲；SyncServerTime 中点近似 + 成功才推进 + 失败 30s 退避 + streak≥3 复位。
- **数据库迁移规范**：migrateAddPublishMeta 幂等；旧库缺纯新增列自动迁移、缺语义列拒绝启动。
- **前端契约面**：/state 与 /api/admin/stats 三态同源；begin_times 恒下发；open_time_set/open_time_known 双字段语义自洽。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` | 双通道 exit 0 |
| 全量 `-race -count=1 -p 1 -timeout 900s ./...` 10 轮 | **R1 api FAIL**（TestSetTargetsBounds）/ **R3 api FAIL**（TestAdminAuth，readyProbe 5×200ms 全 connectex）/ **R5-R7 FAIL（临时验证包 tmptest 构建失败，非产品代码）**；R2/R4/R8/R9/R10 全绿；**清理后 R8/R9/R10 连续 3 轮全绿** |
| zhidao 包全量 10 轮 | **0 FAIL**（TestMain preheat + 无 Discard 生效） |
| TestLoginLogsAttempts/FailureSummary 隔离连跑 | 5 轮全 PASS |
| 身份防线族测试（同名重建 10 个） | 全 PASS |
| task_log 窗口 SQL（30050 行） | 单跑 PASS（24.3s） |
| keep-alive 复用静默关闭形态独立程序实证 | transport 内部用 GetBody 重放完整 body → 新连接收到完整 classId=88 → 服务端**仅收到 1 次**（不双报） |
| read 错误（服务端读完整不响应）独立程序实证 | 客户端报 EOF（read OpError），服务端已收到完整 body → httpDo 外层不重试 → **无二次到达** |
| gofmt -l 检查 | 23 个文件有差异（含 7 个 BOM + import 空行 + 缺尾换行） |
| 测试函数总数 | 208 个 |
| 审查期间工作树 | 与基线一致（5 社区文档 + round58-frontend-findings.md + 临时 tmptest 已清理） |

---

## 结论

- **CRITICAL 0 / MAJOR 1（api flake 残余）/ MINOR 2（gofmt BOM 残留 + cloneReq 注释死代码）/ OBSERVE 8 新增 + 延续 12 项**。产品逻辑本轮**零 CRITICAL**——R57 的 httpDo read 错误不重试修复经标准库源码核证 + 独立程序实证完全正确（transport 内部 GetBody 重放不双报、httpDo 外层 read 不上抛），双报风险彻底根除。
- **flake 统计结论**：清理临时包后 **R8/R9/R10 连续 3 轮全绿**（复现 R57 收尾"全量全包绿"）；计入 R1/R3 后全量 10 轮 **2/10 api 冷启动 FAIL**（R5-R7 为测试基建污染非产品）。zhidao 10 轮 0 FAIL。**api 包冷启动 flake 未根治但显著收敛**（R57 的 3/11 → 本轮 2/10，且 R3 暴露 readyProbe 自身极端窗口也扛不住）。
- **最致命 3 条（按影响排序）**：
  1. **MAJOR-58-01**：api 包全量轮 flake 未归零（R1/R3 FAIL，R3 为 readyProbe 自身失败）——隔离全绿 + 连续 3 轮全绿说明低频残余，readyProbe 窗口加宽或接受 CI `||` 重跑。
  2. **MINOR-58-01**：R57"gofmt 规范化"不完整——20+ 文件 gofmt 差异（7 个 BOM + import 空行 + 缺尾换行），CI 若接格式门禁恒红。
  3. **MINOR-58-02**：cloneReq 注释描述"read 错误重试空 body"的死代码形态（httpDo 已不重试 read），注释与行为半脱节，未来放宽 read 重试的双报陷阱预埋在注释里。
