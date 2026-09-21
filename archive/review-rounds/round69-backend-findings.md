# R69 后端只读审查报告

## 概述

栅栏三包配平终态（200ms×10 + 显式 2s 超时）在本轮**维持成立**：3 轮全量 `cd backend && go test -race -count=1 -p 1 -timeout 900s ./...` 零 FAIL、zhidao 残余面归零且未流动；但经一次**隔离并行复跑**（api+scheduler 双包同跑 -count=2），api 包暴露出 3 例 connectex/timeout 冷启动残余——残余面暂歇仍非根除，包序或并发形态变化可让它再次流动（详见「可观测 flake 面」与「OBSERVE-69-04」）。

## R68 卫生修复复核（commit 0342a3a 逐行核证）

三处修复全部**核实正确**：

1. **accounts/manager_test.go readyProbe 宽栅栏**（0342a3a）：逐行核证
   - 循环：`for i := 0; i <= probeRetries; i++`，`probeRetries=10`、`probeDelay=200ms`，成功分支 `io.Copy(io.Discard, resp.Body); resp.Body.Close(); return`——与 api/zhidao 逐字段一致；
   - 每次 `http.NewRequest(http.MethodGet, baseURL+"/login", nil)` 重建请求、`client := &http.Client{Timeout: 2 * time.Second}` 显式超时——与 zhidao 完全同构；
   - 失败分支 `lastErr = err` 覆盖 + 最后 `t.Fatalf` 上抛——一致；
   - 三个调用点（loginRejectSrv:87 / gateSrv:164 / TestNewClientAfterSetRecognizerGetsEngine:247）全部仍接上；注释准确（含 R68 OBSERVE-68-01 包序保护说明）。
   - 差异点（非缺陷）：api 的 readyProbe 探测 `/ready` 路径且对 io.Copy 失败也进入 lastErr，zhidao/accounts 探测 `/login` 且成功后直接 return（不判 io.Copy 失败）——三家栅栏"几何"一致（200ms×10+2s），探测路径与读体失败判定存在极小型差异，历史已按各家 mock 语义固化，不属于需要对齐的契约面。

2. **zhidao/client_test.go:47 loginMockServer 注释**：已同步为"200ms×10 + 显式 2s 超时，总窗口 ~2s"，与实现（同文件 readyProbe 88-110 行与 accounts 同构）一致；socketPreheat 双保险保留。

3. **store/store_test.go:32 批插量化值**：注释已改为"批事务实测 ~33s（-race 下，R68 复测；非 -race 更快）"，与本次 3 轮全量实测（store 45.7s / 71.6s / 36.6s，含 30050 行批插）量级吻合，注释方向正确。

**栅栏终态独立复测**：3 轮全量全部通过、无 FAIL、无 race。**（详见「flake 统计」）**

## 历轮决策手册抽样闭合复核（契约 1-41）

抽查 6 条关键契约，全部成立：

| 契约 | 实现位置 | 复核结果 |
|------|----------|----------|
| 契约1 开放时间唯一事实源 = beginTimes 自动识别 | scheduler.go:421-443 openTimeFor(Locked)、ProbeForAccount:848-853 / probe:1102-1107 写入、config.go 已无 OpenTime 字段 | 成立。openTimeForLocked 识别槽映射到 openTimeDetected[acct]→["*"]→零值回退（仅测试兼容）；识别过期只影响展示层 open_time_known，不截断零值；空快照不删槽。 |
| 契约2 窗口关闭三判据单源 | scheduler.go:909-935 windowClosedLocked，StateForAccount/WindowClosed/admin stats 同源 | 成立。主判据 +10s 裕量 / 时钟 ≥3 带"开放时间已过" / EmptyProbeRuns≥3 同快照 open；入账侧 B21-02 +10s 裕量对称；测试 TestWindowClosedState/TestGhostWindow* 固化。 |
| 契约4 删账号 memory-first 四步 | handler.go:1005-1019（Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount） | 成立。顺序与契约完全一致；B26-01/B19-02/B20-01 注释完整。 |
| 契约41 撞名学生管理态判定绑定"带 adminName 签发" | handler.go:121-129（管理员双条件 + adminName 回显）、session.CreateAdmin | 成立（后端侧）。管理令牌由 CreateAdmin(Admin=true) 携带，requireAdminSession 走 IsAdmin 判定；登录响应带 adminName 供前端判定。 |
| B42-01 doLogin 全入口收口闸门 | manager.go:223-235 gateTryAcquire 非阻塞准入（LoginByPassword:244 前置）、Relogin:173 gateWait 阻塞 | 成立。手动登录与排队重登共享 gateMu/gateUsed；配合 api 测试 ResetGateForTest（仅测试用）不污染正式代码。 |
| B43-01/B43-02/B41-01 sameClientFor 六分支 | scheduler.go:1485（失效）/1517（成功）/1547（风控）/1567（窗口关闭）/1596（实时复核入口）/1631（确证满员） | 成立。同一身份防线覆盖 spawnChain 全部分支（含 B43-02 的实时复核 ErrUnauthorized 分支）；另有 maybeRelogin 决策侧存在性复核（1204）。测试 TestDeletedAccountRebuiltSameNameChain* 系列（Success/Relogin/RateLimitBackoff/WindowClosedFull/RealtimeRecheckFull/RealtimeUnauthorized/SuccessDropsInflight）成族覆盖。 |

## 跨轮修复闭合抽查（R66/R67/R68 关键修复）

- **recoverMiddleware 防御登记（B39-02）**：router.go:202 wrap 最外层；panic 写真实 HTTP 500 + body 500（handler.go:1197-1208）；随附 CSRF-403 / 会话 401 / 管理 403 / 限流 429 家族齐全（router.go:102/107/114-123 等），未注册 /api/ 前缀显式 404 + 真实 404 状态（router.go:194-200）。
- **Client.SetVision else 兜底护栏（B29-01）**：client.go:167-177——当前引擎为 Vision 或 nil 才重建，本地引擎保留；manager.SetVision 保留模板引擎（manager.go:194）。测试 TestSetVisionKeepsLocalRecognizer/TestSetVisionRebuildsWhenCurrentIsVisionOrNil + accounts TestNewClientAfterSetRecognizerGetsEngine 双绿。
- **zhidao/accounts readyProbe 宽栅栏**：本报告首节已复核，三包对齐成立。

## 新发现

### MINOR-69-01（调度器注释行号错位）
- 文件：backend/internal/scheduler/scheduler.go:1086-1087（`s.probing` 复位注释引用 `// 失败同样计入节流闸门` 对应逻辑正确；同函数 1141 行 `open := s.openTimeForLocked("")` 注释此前引用"本函数开头已取过单次快照（830 行前）"——实际 tick() 中取 open 在 973 行；且 probe() 内 1141 行本身在主判据里独立取 open 快照，符合单快照复用语义）。属注释行号陈旧，无逻辑影响。
- 依据：`grep -n` 定位 tick/probe 的 open 采样行，注释引用的"830 行前"与当前 973 行不符；1139-1141 注释本身描述的"单快照复用"语义正确。
- 影响面：维护误导（未来改行号易失锚）；零运行时影响。
- 机制：probe() 的开启判定与 EmptyProbeRuns 入账共用同一 open（1141），语义正确，仅注释锚点行号过期。
- 建议：将注释行号修正为当前实际行（973）；纯注释卫生，可并入后续任意卫生 commit。

### MINOR-69-02（accounts readyProbe 无探活路径差异注释缺失）
- 文件：backend/internal/accounts/manager_test.go:26-55
- 问题：accounts 的 readyProbe 与 api 版存在两处微小形态差异——探测路径 api 用 `/ready`（factory 内 mock 对未知路径返回 `{"code":1,...}`），zhidao/accounts 用 `/login`；api 对 io.Copy 失败计入 lastErr 继续轮询，zhidao/accounts 成功后直接 return 不判读体失败。三家"几何"一致但判据微差，属账面差异而非缺陷。
- 影响面：仅可维护性提示；由于各自对 mock 语义固化，行为无分叉。
- 建议：可在三处 readyProbe 的注释中补一行"路径/读体判定各家按 mock 语义微差，栅栏几何（200ms×10+2s）是配平契约本体"，避免未来维护者误当不一致而改造。**不修**（低价值，仅记档）。

### OBSERVE-69-03（cfg.Decrypt 停用字段仍在 Router 签名）
- 文件：backend/internal/api/router.go:81 注释「Decrypt 字段仅注入备用（B10-08 起未消费）」；Deps.Decrypt 字段仍传参、main.go:52-53 仍构造 decrypt。
- 影响面：结构性冗余——Decrypt 自 B10-08 起未消费且 api 层无解密需求（vision_key 明文只在 main 读取后进入 runtime）。删除会牵动 Register 签名与 main 调用点；属"保留字段仅供备份"的显式契约，非死代码误报。
- 建议：观察项，若未来做 API 签名整理可一并移除；当前保留与注释自洽，**不修**。

### OBSERVE-69-04（api 包隔离并行复跑暴露冷启动残余——栅栏终态流动样本）
- 文件：backend/internal/api/handler_test.go:47（newTestDeps 就绪探测）及相关夹具；api 包测试素有 30s 登录限流桶（router.go 每测试新建 limiter，无残留）。
- 现象：`go test -race -count=2 ./internal/api/ ./internal/scheduler/`（api+scheduler 同命令并行，无 -p 1）复跑时 api 包 3 例失败：
  - TestLogoutRevokesToken：`/login/captcha ... context deadline exceeded (Client.Timeout exceeded while awaiting headers)`（handler_test.go:333）
  - TestLoginOKIssuesSession：`/ready 就绪探测失败: connectex`（handler_test.go:47）
  - TestElectiveSelectRejectsFullClass：`/findElectivesData ... connectex`（handler_test.go:1079）
- 机制：隔离并行双包共争 CPU/端口/连接管理，Windows 回环冷启动窗口在并发形态下重现（包括 readyProbe 自身 11 次重试仍 connectex，以及 await-headers 超时形态）；全量 -p 1 3 轮全绿、api 单独 -p 1 3 轮全绿 → 残余面仍在，宿主 = 并发包形态而非单一包序。
- 影响面：CI/本机非 -p 1 跑全量（或 go test ./... 不带 -p 1 的并行包形态）仍可能偶发红；管理界面/加速监控之外的纯测试影响。
- 建议：契约层面维持"栅栏几何已收敛 + 残余面仍不可见可流动"的既有观察；若要根治并行形态，可在 CI 固定 `-p 1`（release.yml/ci.yml 已隐含串行）。当前**列为观察项不修**（栅栏已最大化覆盖，包并行冷启动属 Windows 回环 OS 级窗口，非应用层能根治）。

### OBSERVE-69-05（"登出也频繁"不适用：登出无限流、无 CSRF JSON 门）
- router.go:186-188 `POST /api/logout` 未挂 `requireJSONBody` 也未挂限流。复核认定不适用——登出仅吊销自身会话，无跨站表单可挟持的高利害（CSRF 也仅在具备会话时生效），且 handleLogout 已由 requireAuth 保护；前端 fetch 恒带 JSON。属刻意的低风险面，记档不修。

## flake 统计

**3 轮全量**（cd backend && go test -race -count=1 -p 1 -timeout 900s ./...）：

| 轮次 | 结果 | 备注 |
|------|------|------|
| RUN1 | 全绿（10 包 ok，store 45.7s 最慢） | EXIT=0 |
| RUN2 | 全绿（store 71.6s） | EXIT=0 |
| RUN3 | 全绿（store 36.6s / zhidao 22s） | EXIT=0 |

**隔离复跑**：
- zhidao 包 -count=3：全绿
- accounts 包 -count=3：全绿
- api + scheduler（同命令并行，未 -p 1，-count=2）：**api 包 3 例失败**（详见 OBSERVE-69-04）
- api 单独 -p 1 -count=3：全绿

**残余面判断**：栅栏三包配平终态在"全量 -p 1 / 单包隔离"下维持成立（残余面不可见）；但"多包同命令并行"形态下残余面再次流动（api 3 例 connectex/await-timeout）。判定——残余面暂歇非根除，当前最可能出现宿主的形态 = **非 -p 1 的多包并行运行**（Windows 回环冷启动 + CPU 争用）；单一包序全量 -p 1 下仍不可见。建议 CI 固定 -p 1 以锁定终态。

## 历轮观察延续

- R68 栅栏三包配平：**闭环延续**（本报告首节+RUN1-3）。
- R66/R67 前端卫生：本轮后端未见影响。
- OBSERVE-65-03（accounts 无 socketPreheat 双保险）：accounts 仅 readyProbe 单层。8 轮全量+本轮 3 轮全量+accounts -count=3 隔离全绿，未再出现 accounts 冷启动样本；反而 api 包在多包并行形态暴露残余。维持"若未来 accounts 再出冷启动 flake 第一候选即补 socketPreheat"记录。

## 其它走读确认（无缺陷项）

- `go build ./...` 通过；`go vet ./internal/... ./cmd/...` 零告警。
- go.mod go 1.26.8，`min()` 内建使用合法。
- web/embed.go 的 /api 前缀兜底 404 与 router.go /api/ 显式 404 构成双保险，SPA 回退不含 /api。
- Nav：-race 下 clinics 全部 ≥0；无 goroutine 泄漏（sched.Stop / sessions.Close / zhi.Close Cleanup 齐全）。
- store 27 个方法、handler 26 个方法逐一核对无不消费的死方法；fakeStore/fakeClient/fakeAccts 与接口完整匹配。
- 会话/密钥/AES-GCM 路径无明文落盘；maskKey 只泄后 4 位；登录限流按 IP 独立桶 + 激活独立桶。
- 安全：登录/激活 CSRF JSON 门 + 限流 429；管理接口 requireAdminSession；/api/electives/select|exit 与 /api/targets 全部 requireJSONBody；Delete 空 body 语义保留（REST）且仍离权。

## 结论

后端在本轮无 CRITICAL/MAJOR 缺陷。发现 2 个 MINOR（注释卫生）+ 3 个 OBSERVE（停用字段、登出 CSRF 面不适用、包并行冷启动残余流动样本）。栅栏终态在三轮全量下保持绿色，残余面仅在多包并行形态观测到流动，已定位宿主并给出 CI -p 1 收口建议。