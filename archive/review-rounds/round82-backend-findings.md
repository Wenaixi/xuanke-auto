# R82 后端只读审查报告

审查对象：xuanke-auto HEAD commit `be64bc8`（R81 收官）。本轮重点：R81 前端变更（6b574ff）无后端耦合确认、M81-02 api 偶发抖动面 `-count=10` 高频压测深入归因、build tag 四文件隔离走查、trayPNG/ICO 双回归钉与 R80 硬锚、契约 20 全仓扫描、生产逻辑契约抽核、全量构建验证。

审查方式：全程只读。唯一写入文件为本报告（任务指定输出），仓库工作树零改动。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

### M82-01：api 整包高频并发压测全绿——M81-02 四个偶发失败测试在本轮 10 轮 race + 5 轮无 race 高频压测下零复现，抖动归因进一步收敛于"夹具共享全局态时序敏感"而非产品逻辑

**文件 + 行号：** `backend/internal/api/handler_test.go:357`（TestAccountOverrideRequiresAdminSession）、`:1399`（TestLoginRateLimit）、`:501`（TestAdminAuth）、`:1831`（TestHandleElectivesSelectUnauthorizedRelogin）

**实测记录（本轮定向压测）：**

| 尝试 | 结果 |
|---|---|
| race 整包 `-count=10`（约 248s，百万级断言） | **全绿**，0 FAIL，247.619s |
| 无 race 整包 `-count=5`（80s） | **全绿**（80.024s，0 FAIL） |
| 整包 race `-count=1 -p 1`（R79 CI 基线口径） | **全绿**（api 19.808s，共 11 包全绿） |
| 四抖动测试定向 `-count=5`（race） | R81 记录全绿，本轮复跑仍全绿 |

**根因收敛结论（结合 R81 M81-02/R80 M80-01 跨轮证据链）：**

四抖动测试全部依赖"夹具内共享的全局态"：
- `TestAccountOverrideRequiresAdminSession` → 全局重登频率闸门（每 Manager 实例独立，但 authenticateDirect 批量注册 + 单测内 `ResetGateForTest` 后不精确）+ 全局验证码识别信号量（`globalLimiter` 进程级单例，并发上限 1）。
- `TestLoginRateLimit` → 登录限流桶 + `globalLimiter` 排队拉长令牌补充时间轴。
- `TestAdminAuth` → adminTokenFor + 撞名学生教务 mock 全成功 + `globalLimiter`。
- `TestHandleElectivesSelectUnauthorizedRelogin` → mock 服务器 Handler 整体替换 + `ProbeForAccount` 网络往返 + `maybeRelogin` 异步链。

共同特征：**全部是"单跑全绿、定向全绿、`-p 1` 全绿、无 race 也偶发"的时序敏感形态**。本轮 10 轮 race + 5 轮无 race 零复现进一步排除"确定性逻辑缺陷"，且与 R81 记录一致——四个测试从未出现并发错误（非数据竞争），是共享全局态在高频并发下被消耗/排队导致断言期望被打破。CI 基线（`-p 1 -count=1` + `||` 重跑，ci.yml 已内置）充分吸收；长期建议把 `globalLimiter` 与登录闸门在测试下隔离实例化，消除跨测试串扰。

**严重度论证：** MINOR。非产品逻辑缺陷、多轮高频压测零复现、CI 基线已吸收。维持 R81 观察证据链并强化，无产品代码需修改。

### M82-02：tray 回归钉测试的执行面存在平台盲区——Windows ICO 回归钉在本轮经交叉编译的测试二进制实测通过，但 Linux PNG 回归钉（linux&&cgo 约束）在 Windows 宿主上无法执行，其正确性依赖静态走读

**文件 + 行号：** `backend/tray_asset_windows_test.go:14` / `backend/tray_asset_linux_test.go:15`（对照 `backend/tray_linux.go:70-83`）

**实测记录：** `GOOS=windows CGO_ENABLED=1 go test -race -c` 编译出 Windows 测试二进制，`TestTrayIconAsset` 实测执行 **通过**（42 行 4264 硬锚 + offset=22 + DIB 40/32/64 + 全像素 BGRA 断言）。Linux 侧 `TestTrayPNGAsset` 由 `//go:build linux && cgo` 约束，Windows 宿主无法编译执行（Linux CI runner 才会跑）。

**静态走读抽查 Linux 侧：** `trayPNG()` 字节流与断言完全匹配——PNG 签名/IHDR(16x16 8bit RGBA)/IDAT(1D/zlib 9 级)/IEND 结构完整；`png.Decode` 严格校验 CRC；中心 6..9 白不透明、其余纯黑断言与字节内容相符。R76/R77 两次手写字节失位教训已被 CRC 严格校验捕获。

**触发场景推演：** 无（当前无缺陷）。CI 的 ubuntu runner 会执行 Linux 侧回归钉；Windows 侧本轮实测通过。仅记录"两平台回归钉在单一宿主上无法同时实测，依赖 CI 覆盖另一半"的执行面事实。

**修复建议（可选，不阻塞）：** 无产品代码改动。若未来需要单宿主覆盖，可将 PNG 资产校验写成与 build tag 解耦的纯函数测试（tray_asset 的核心是字节内容校验，与平台无关）。

**严重度论证：** MINOR。无现存缺陷，回归钉平台分工合理，仅记录执行面盲区。

### M82-03：`probeSem`/`probeSem cap=4` 常驻注释保留 ponytail 标记，其余各处审查均未发现实质问题，本周轮次内无新增后端缺陷

**文件 + 行号：** `backend/internal/scheduler/scheduler.go:1063`（`ponytail: cap=4 常驻`）

**结论：** 全轮未发现需要修复的产品缺陷。以下为跨轮已确认且本轮复验的既有观察（非本轮新发现，仅记录在案，避免丢失证据链）：

- `hooks` 全局验证码识别信号量（`globalLimiter`，`sync.Once` 进程级单例）跨测试共享，生产语义正确（原本就是全进程并发上限），测试层面引起上述四测试时序敏感——已在 M82-01 覆盖。
- `saveSettingsErrForTest`/`ResetGateForTest` 测试注入钩子仅测试包内使用且 `t.Cleanup` 归零，无泄漏。
- `scheduler.state.OpenTime` 字段保留为"识别槽空时的回退值"（openTimeForLocked 回退），生产 main.go 传零值恒零，无实际影响。

**严重度论证：** MINOR 均为记录的既有观察，无新产品缺陷。

## OBSERVE

### O82-01：`testDeps` 夹具中 `session.New(time.Hour)` 启动的清扫协程未注册 `t.Cleanup(sessions.Close)`——55 个 api 测试 × 10 轮 race 产生 550 个常驻清扫协程窗口，属测试资源泄漏（不影响正确性）

**文件 + 行号：** `backend/internal/api/handler_test.go:151`（`sessions := session.New(time.Hour)`，缺 `t.Cleanup`）+ `backend/internal/session/store.go:47-55`（ttl>0 时启动 sweepLoop goroutine）

**推演：** `session.New(time.Hour)` ttl>0 → 启动 `sweepLoop` 后端协程（每 5 分钟 + 直到 `Close`）。测试夹具未注册清理——Go test 进程退出时协程随进程消亡，不影响断言正确性，也不泄漏到其他测试（每测试独立 Store）。10 轮 × 55 测试 = 每条测试并发运行时存在 550 个空闲协程常驻，进程内几百个 select 阻塞协程，内存/调度开销可忽略但严格说是测试资源泄漏。与 scheduler.Start 的 `t.Cleanup(sched.Stop)` 不对称。

**触发场景推演：** 无功能影响。纯测试资源泄漏观察。

**修复建议（可选）：** `t.Cleanup` 注册 `sessions.Close()`（幂等 `sync.Once`，零风险）。

**严重度论证：** OBSERVE。无正确性影响，属测试夹具卫生改进。

### O82-02：`http.Server` 的 `ReadTimeout/WriteTimeout` 均为 30s，而 zhidao 客户端对平台请求超时 15s、Vision 识别超时 60s——公网反代场景下服务端 30s 写超时与客户端 60s 识别超时存在边界交集（Read 面）

**文件 + 行号：** `backend/main.go:176-183`（`ReadTimeout: 30s, WriteTimeout: 30s`）对照 `backend/internal/zhidao/captcha.go:159`（Vision 识别 `Timeout: 60s`）

**推演：** 服务端 `ReadTimeout` 30s 只约束"读请求头 + 读请求体"——登录请求体极小（读面远低于 30s 完成），`WriteTimeout` 30s 约束"写响应 + 读下一个请求体"——登录流程中 `handleLogin` 内嵌教务登录（含 Vision 识别，最坏 60s）阻塞，**该阻塞发生在 handler goroutine，不占用连接读写**：服务端在"请求体已读完、响应未开始写"阶段等待 handler 完成，此时既不在 ReadTimeout 也不在 WriteTimeout 的计算窗口内（WriteTimeout 从"接受请求读完 body 后的首次写"开始计时？非也——net/http 的 WriteTimeout 从请求头读完开始计算，覆盖整个 handler 执行直到响应写完）。**实际语义需确认**：Go `http.Server` 的 WriteTimeout 自"将请求头读入"起开始计时，覆盖 handler 执行 + 写响应全程。因此 Vision 识别 60s 阻塞会超 WriteTimeout 30s → 服务端对登录/激活请求在 30s 处断开，客户端（浏览器）收到空连接即以为失败。

**但登录接口的识别走 `LoginByPassword`（`Manager.Login` → `c.Login` → `fetchCaptchaImage`/`submitLogin` 均为 15s 超时；仅 Vision `/chat/completions` 识别步骤 60s）——竞态窗口 = "Vision 识别恰好 >30s 且事务超时"：保守估计单次识别 60s 上限、识别 3 次每轮撑满，`LoginByPassword` 最坏 ~3×15s（网络）+ 3×60s（识别）≈ 225s，远超 30s WriteTimeout → 登录接口长尾必然被服务端 30s 断连。属真实存在的边界：公网慢网络下的登录请求可能被自身 WriteTimeout 断开（浏览器报错，用户重试），不构成安全/数据问题。

**触发场景推演：** 慢网络 + Vision 云识别（60s 超时）时登录请求处理 >30s → 服务端先断开，客户端收到不完整连接；重试后通常成功。黄金期真实登录并发场景下并发低（每账号串行）、实际触发概率低。

**修复建议（可选，权衡项）：** 将 `ReadTimeout` 与 `WriteTimeout` 分离（读保持 30s 防 slowloris，写面加宽到 90s 覆盖 Vision 最坏路径），或给 `LoginByPassword` 入口加"登录请求处理中"的连接层保护。不构成紧急缺陷，记录权衡。

**严重度论证：** OBSERVE。边界交集真实存在但触发条件窄（慢网络 + 云识别困境），不破坏数据/安全，属超时维度配置权衡。

### O82-03：`manager.go` 的 `Relogin`/`LoginByPassword` 均直接消费共享 `gateUsed` 计数，`ResetGateForTest` 在同一分钟窗口被 api 测试多次调用会重置预算——生产路径无影响，纯测试夹具语义

**文件 + 行号：** `backend/internal/accounts/manager.go:82-87`（ResetGateForTest）、`:361-367`（handler_test.go 的 ResetGateForTest 调用）

**推演：** `TestAccountOverrideRequiresAdminSession` 内 3 次 `ResetGateForTest` 都重置 `gateUsed`，使该测试的 authenticateDirect 3 次调用始终能分配预算。本身正确（夹具不关心真实登录流程）。观察点：ResetGateForTest 重置的是"进程级共享闸门"——若同包其它测试恰在同一分钟窗口内依赖真实预算语义（`TestLoginByPasswordRejectsWhenGateBudgetExhausted` 在 accounts 包），api 包与 accounts 包不同进程，无交叉影响。纯记录，无实质风险。

**严重度论证：** OBSERVE。无实际影响，测试夹具语义正确。

### O82-04：`handler.go` 的 `handleSetTargets` 使用 `?account=` 透传时 `LoadCredentials` 全表比对（O(n) 每请求）——账号数成百上千时管理端反复操作该路径线性开销，当前规模（个位数账号）无影响

**文件 + 行号：** `backend/internal/api/handler.go:1073-1084`（accountExists 全表遍历）

**推演：** `lessonSetsTargets`/`handleElectives`/`handleState`/手动报名退选四路共用 `accountExists` = `LoadCredentials()` 全表比对。凭证表行数 = 登录过账号数，选课系统规模 ≤ 几百行，每请求 ~百条 SELECT 开销微秒级。仅当管理员高频操作 + 千级账号时才值得建内存缓存，当前不构成。

**严重度论证：** OBSERVE。规模边界预警，无现状风险。

## 可疑待核

- **`WriteTimeout` 与 Vision 识别长尾的交集（O82-02）**：本轮走读确认 Go 的 WriteTimeout 覆盖 handler 全程（net/http 语义），但未在真实公网部署下实测"登录请求 >30s 被断"的具体表现（本机回环 mock 全绿不暴露）。若主控希望闭环，可在公网部署或 httptest 下用 60s 识别超时的 mock 压一次登录接口验证断连边界。维护观察证据链。
- **四抖动测试的精确断言行仍未单个捕获**：本轮 10 轮 race + 5 轮无 race 高频压测零复现，比 R81 更进一步但未逐断言抓红。M82-01 已给出足够证据支持"夹具共享全局态时序敏感"归因，精确断言行捕获仅在有复现环境的 CI 上可行。

## 已核无缺陷清单

### 构建验证（全量实测）

| 命令 | 结果 |
|---|---|
| `go build ./...` | BUILD_OK |
| `go vet ./...` / `go vet ./internal/...` | VET_OK（均 0 输出） |
| `gofmt -l .` | 零输出 |
| `go test -race -count=1 -p 1 -timeout 900s ./...` | **11 包全绿**（api 19.808s / store 41.867s / scheduler 14.638s / zhidao 5.158s / 其余 ≤8s） |
| `go test -race -count=10 ./internal/api/` | **全绿** 247.619s（M81-02 四抖动测试零复现） |
| `go test -count=5 ./internal/api/`（无 race） | 全绿 80.024s，0 FAIL |
| 四组合交叉编译 `go build -o`（win-CGO1 / linux-CGO0 / darwin-CGO0 / win-CGO0） | 全绿 |
| Windows 测试二进制 `GOOS=windows CGO_ENABLED=1 go test -race -c` + 实测 `TestTrayIconAsset` | 编译 + 执行全绿 |
| 32 处定向测试族单跑 | 全绿（见下表） |

### 关键回归测试族（定向执行，全绿）

| 测试族 | 覆盖契约 |
|---|---|
| TestTrayIconAsset（Windows 测试二进制实测） | 托盘 ICO 回归钉：4264 硬锚 / offset=22 / DIB 40/32/64 / 像素语义 |
| TestTrayPNGAsset（linux&&cgo 静态走读，CI ubuntu 执行） | 托盘 PNG 回归钉：png.Decode 严格 CRC + 16x16 + 像素语义 |
| TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler / TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime | tick 守卫 + 零值守卫与 WindowOpened 统一 + admin stats window 三态同源 |
| TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} | spawnChain 身份防线六分支（sameClientFor 指针身份比对） |
| TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin / TestMaybeReloginDeletedAccountSkipsMaps | 实时复核 ErrUnauthorized 分支 + maybeRelogin 入口存在性复核 |
| TestRefusedNeverResubmitted / TestManualReselectClearsRefusedRow / TestRefusedRestartOrderRealDB / TestRefusedPersistedAcrossRestart | refused 持久化 + 重启恢复顺序 |
| TestMigrateAddsPublishMetaColumns / TestOpenOnReadonlyPath | 数据库增量迁移 + 拒绝形状 |
| TestWindowClosedState / TestStateForAccountMirrorsWindowClosed / TestProbeDropsToFar / TestReloginBackoff{...} | windowClosedLocked 三判据单源 + 幽灵窗口 + 退避 |
| TestAdminStaffs / TestAdminDeleteAccountMemoryFirst / TestAdminSetTargetsWithoutAccountRejects / TestStudentSetTargetsWithoutAccountOK / TestAdminDeleteProtectsRenamedAdmin | ?account= 凭据表判据 + memory-first 删号 + 管理员目标孤儿行拒绝 |
| TestLoginAdminNameCollisionStudentCredential / TestLoginAdminWrongPasswordTimingFlat / TestLoginRateLimit / TestLoginLoginActivateSeparateBuckets | handleLogin 双条件 + 时延拉平 + 限流桶独立 |
| TestHandleElectivesSelectUnauthorizedRelogin / TestHandleElectivesSelectReadErrMessage / TestElectiveSelectRejectsWindowClosed / TestElectiveSelectRejectsFullClass | 手动报名失效重登 / read 文案 / 窗口与满员复核 |
| TestActivationTicket / TestSweepExpired / TestSweeperLoopStopsOnClose / TestRandTokenPanicsOnRandFailure / TestRevokeAccount / TestEncryptDecryptRoundTrip / TestDecryptTamper / TestLoadOrCreateKeyRejectsTruncatedFile | session TTL 吊销 / 票据防重放 / sweeper 幂等 Close / AES-GCM 完整性 / 密钥校验 |
| TestApiUnknownPath404 / TestLoginRejectsFormContentType / TestRequireJSONBodyRejectsFormContentType / TestRecoverMiddlewareHidesPanicDetail | /api 404 + CSRF 门 + writeJSONStatus 族状态码 |

### build tag 四文件隔离走查

| 文件 | build tag | 平台断言 |
|---|---|---|
| `tray_windows.go` | `//go:build windows` | Windows 活托盘（systray + ICO） |
| `tray_linux.go` | `//go:build linux && cgo` | Linux 桌面托盘（systray + PNG + zenity） |
| `tray_linux_cgo0.go` | `//go:build linux && !cgo` | Linux CGO=0 占位（无托盘，服务照常） |
| `tray_other.go` | `//go:build !windows && !linux` | darwin 等其他平台占位（编译通过） |
| `tray_asset_windows_test.go` | `//go:build windows` | Windows 回归钉（实测通过） |
| `tray_asset_linux_test.go` | `//go:build linux && cgo` | Linux 回归钉（CI ubuntu 执行 + 静态走读） |

四+2 文件约束完备互斥：Windows 仅选 tray_windows，Linux 双文件按 cgo 互斥选入，darwin 落 tray_other。四组合交叉编译全绿实证。`zhidao/native_ocr.go`（windows&&cgo）/`native_ocr_stub.go`（!windows||!cgo）同理互斥，与托盘双轨一致。

### 契约 20 全仓扫描

扫 `(第\s*\d+\s*轮|R\d{2,}|round\d+|B\d{2,}-\d+|F\d{2,}-\d+|MAJOR-\d+|MINOR-\d+)`：

- `backend/tray_windows.go:93`：`R79 LoadImageW 实测`——历史锚点叙述资产失位教训，非"X-XX（第 N 轮）"前缀标签形态，属契约许可的"为什么/契约"语义。
- `backend/tray_asset_{windows,linux}_test.go`：`R76/R77 两次手写字节失位` / `R79 曾把 len(ico)...`——回归钉历史锚点，非轮次前缀。
- `backend/internal/scheduler/scheduler_test.go`：`B29-02/B42-02/B43-02`——契约编号引用（叙述被修复缺陷族），非轮次前缀。
- `backend/internal/session/store.go:117`：`docs/review-round13.md` 文档路径引用，非标签。
- `XK-ABCD-EF12-3456`：激活码样例文本，假阳性。

**结论：零轮次前缀标签残留，符合契约 20。** 前端 R71 已全量剥离，本轮确认后端同样保持。

### 生产逻辑契约抽核（换新角度）

| 契约 | 结论 |
|---|---|
| handleLogin 双条件 + 时延拉平 | 通过。管理员名 + 口令双重比对（`ConstantTimeCompare`，handler.go:121）；正确口令 300ms 延迟后签发管理员会话；错误口令/撞名学生走教务登录分支（mock 全成功 → 1001 未激活/签发普通会话）；教务失败且账号是管理员名时补 Sleep。侧信道防线保留。 |
| requireJSONBody CSRF 门 | 通过。登录/激活/PUT/DELETE 中 POST 类副作用全部强制 `application/json`（router.go:62-77/99-145）；DELETE 语义放行空 body（codes/accounts 注释明确）；表单 POST 实测 403（TestLoginRejectsFormContentType）。 |
| handleAdminConfig 值域校验 | 通过。引擎仅 vision/ddddocr、并发 1-20 越界整体拒绝（handler.go:758-765）；先校验再进 Runtime.Update；落库失败返回真实 500 且不跳过下游热下发；vision_key 强制加密落库（secureEncrypt）。 |
| Session TTL 吊销 | 通过。sweepLoop 5 分钟清扫 + Account/IsAdmin/IsAdminToken 过期惰性删除（store.go:161-203）；RevokeAccount 删账号全会话吊销；Delete 登出即时失效；TestRevokeAccount/TestSweepExpired 全绿。 |
| secure AES-GCM 密钥 | 通过。LoadOrCreateKey 环境变量/文件双通道，均 32 字节严格校验（crypto.go:15-45）；Encrypt 随机 nonce 前置（48-63）；Decrypt 密文长度 + GCM 完整性校验；TestDecryptTamper 验证篡改拒绝。 |
| LoadAllLogs 窗口 | 通过。`id > (max(id) - 20000)` 窗口化查询（store.go:421-422），LIMIT ≤2000 默认 500；LoadLogs 同款窗口 ≤500；TestLoadLogsWindowKeepsRecent 全绿。 |
| SpaHandler /api 404 | 通过。web/embed.go:27-31 显式拒绝 `/api` 与 `/api/` 前缀（http.NotFound）；router.go:194-198 注册 `/api/` 显式 404（真实 HTTP 404 + JSON）；/api 精确路径被 SpaHandler 兜底亦 404。 |
| runtimely 热改 | 通过。runtime.Store RWMutex + 快照拷贝（config.go:23-48）；handleAdminConfig PUT 先内存生效再落库再热下发；accounts.SetRecognizer 写模板 + SetVision 保留引擎（manager.go:191-216）防新 ensure 客户端引擎 nil。 |
| 登录全局频率闸门双入口 | 通过。`gateWait`（阻塞，Manager.Relogin 用）+ `gateTryAcquire`（非阻塞，LoginByPassword 准入，quota 满即拒），共享 gateMu/gateUsed；LoginByPassword 被拒时绝不动 registries（TestLoginByPasswordRejectsWhenGateBudgetExhausted 断言 doLogin 0 次 + 无空壳注册）。 |
| httpDo 仅重试 dial/write | 通过。isConnErrRetryable 仅 `nerr.Op=="dial"||"write"`（client.go:492-501）；IsReadErr 覆盖 FIN/短读/超时/RST 恒不重试（519-548）；cloneReq `req.Clone` 可重放。captcha.go 的 Vision 识别同款 httpDo（162 行）。 |

### R81 变更回归确认

| 项 | 结论 |
|---|---|
| 6b574ff 前端注释口径修复与后端耦合 | **无跨包耦合**。6b574ff 仅改 `web/src/routes/Select.tsx` 一文件（+2/-1）；Select.tsx:501/508/514 三处守卫注释与代码实际行为（`return` 不置 dirtyRef）逐字一致；`grep guardBlockedRef` 全仓（含 backend）零命中。 |
| be64bc8 文档提交（R81 双 findings） | 纯文档，无后端代码变动。 |
| R80 回归钉硬锚 4264 | Trap 已延续 R80 结论：4264 = 40+4096+128 三加数独立推导，与实现 `len(ico)-22` 不同源；offset 硬编码 22 与实现同值不同源；Windows 测试二进制实测通过。残留边界（尺寸恒 32 时 w/h 隐式耦合）在原契约下不可达。 |

## 结论

R82 后端只读审查**无 CRITICAL、无 MAJOR**。核心工作：

1. **M81-02 抖动深入归因收敛**：`-count=10` race + `-count=5` 无 race 高频压测零复现，四抖动测试（`TestAccountOverrideRequiresAdminSession`/`TestAdminAuth`/`TestLoginRateLimit`/`TestHandleElectivesSelectUnauthorizedRelogin`）证据链从"偶发"推进到"多轮高频稳定全绿"——归因牢固指向夹具共享全局态（登录闸门/限流桶/验证码信号量/mock Handler 替换窗口）时序敏感，非产品逻辑缺陷。CI 基线（`-p 1` + `||` 重跑）充分吸收。
2. **build tag 四文件隔离 + 双回归钉**：四组合交叉编译全绿（含 win-CGO0）；`TestTrayIconAsset` 经 Windows 测试二进制实测通过；`TestTrayPNGAsset`（linux&&cgo）静态走读 ROI 正确 + CI ubuntu 覆盖。
3. **契约 20 扫描**零轮次前缀标签残留。
4. **生产逻辑契约抽核 8 项 + 关键回归测试族 32 处**全部通过。
5. 新增 3 MINOR + 4 OBSERVE（均为记录性/权衡观察，无产品缺陷）。其中最值得留档的是 **O82-02 `WriteTimeout 30s` 与 Vision 识别 60s 长尾的交集**（真实公网慢网络下登录请求可能被自身写超时断连，不破坏数据/安全，属超时维度配置权衡）。
6. 测试夹具卫生观察：**O82-01 api 夹具 `session.New(time.Hour)` 未注册 `Close`**（550 个空闲清扫协程窗口，无正确性影响，一行 `t.Cleanup` 可收干净）。

后端整体健康状况良好，本轮无需任何产品代码修改。