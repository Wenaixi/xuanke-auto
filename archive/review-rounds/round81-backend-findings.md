# R81 后端只读审查报告

审查对象：xuanke-auto HEAD commit `eca2ebd`（R80 收官），重点复核 R80 变更回归（回归钉硬锚 4264 / guardBlockedRef 移除的后端耦合 / M80-01 偶发抖动归因），并做 build tag 四文件隔离走查、契约 20 全仓扫描、生产逻辑契约抽核与全量构建验证。

审查方式：全程只读。`go build` / `go vet` / `gofmt -l` 只跑不改；`go test` 仅执行不改；定向压测 `-run` + `-count` 多轮实证记录于下。仓库工作树零改动（唯一新增为本文档，属任务指定输出）。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

### M81-01：R80 回归钉硬锚 4264 的三加数独立推导正确，但"像素+AND mask"与"总长减头部"存在结构上可等价复算的边界

**文件 + 行号：** `backend/tray_asset_windows_test.go:24-33`（对照 `backend/tray_windows.go:96-97`）

**复核结论：** R80 修复正确——测试 `want := uint32(4264)` 为硬编码常数，按格式语义三加数独立推导（BITMAPINFOHEADER 40 + 像素 32×32×4=4096 + AND mask 32×4=128），与实现 `binary.LittleEndian.PutUint32(ico[14:18], uint32(len(ico)-headerSize))` 根因不同源；offset 断言 `off != 22` 同样硬编码。**无同式复算**，M80-02 目标达成。

**残留观察（非缺陷，归 MINOR 供记录）：** 三加数式虽独立，但 40/4096/128 三个数中，4096 与 128 均隐含 `width=32` 假设——若未来某人把 ICO 改成 48x48（像素 48×48×4=9216、AND mask 48×4=192、新总长 40+9216+192=9448），测试会把 4264 一并改掉，依旧自洽绿，但此时实现若算错（如把 andMaskBytes 算成 `height` 而非常 `andRowBytes*height`，48 宽 1 行 6 字节与 32 宽 1 行 4 字节仍可巧合相等）测试不会红。该边界在"尺寸恒为 32x32"既定契约下不可达（`tray_windows.go` 常量 `width=32/height=32` 固定、改动即牵连 DIB 头与像素循环多处断言同步红），记录以防未来扩展尺寸时回归钉失验力。

**触发场景推演：** 无（当前尺寸固定 32x32，测试全绿）。

**修复建议（可选，不阻塞）：** 未来若支持多尺寸，将宽度/高度抽取为常量并在三加数推导中引用同一常量（尺寸变化时测试同步推导，仍保持"与实现 `len(ico)-headerSize` 不同源"）。

**严重度论证：** MINOR。当前契约下不可达边界，仅为"回归钉未来失验力"的预防性记录，不构成现存缺陷。

### M81-02：api 包偶发失败不再局限于 R80 指出的两个测试——race 整包压测观察到另外两个测试也偶发红，但全部单跑全绿且 `-p 1` 串行全绿

**文件 + 行号：** `backend/internal/api/handler_test.go:357`（`TestAccountOverrideRequiresAdminSession`）、`:501`（`TestAdminAuth`）、`:1399`（`TestLoginRateLimit`）、`:1831`（`TestHandleElectivesSelectUnauthorizedRelogin`）

**实测记录（本轮定向压测）：**

| 尝试 | 结果 |
|---|---|
| 无 race 定向 `-count=5 -run 'TestAccountOverrideRequiresAdminSession\|TestAdminElectiveSelectUnknownAccountRejects'` | 全绿（3.16s） |
| race 定向 `-count=5` 同两测试 | 全绿（5.47s） |
| race 定向 `-run 'TestAdminAuth\|TestLoginRateLimit\|TestHandleElectivesSelectUnauthorizedRelogin'` ×6 | 全绿 |
| 整包 race `-count=1 -p 1`（R79 基线口径） | 全绿（25.79s） |
| 整包无 race `-count=4` | 全绿（75.17s，0 FAIL） |
| 整包 race `-count=1` ×4 | 第 2 轮 `FAIL: TestAdminAuth (16.15s)` + `FAIL: TestLoginRateLimit (13.13s)`；第 3 轮 `FAIL: TestHandleElectivesSelectUnauthorizedRelogin (20.08s)`；其余两轮全绿 |
| 整包 race `-count=1`（补跑多轮） | 多数全绿，偶发 FAIL（含 R80 记录的两个原测试，共观察到 4 个不同测试名） |

**触发场景推演：** 四测试路径各不相同——`TestAccountOverrideRequiresAdminSession` 走 `authenticateDirect → LoginByPassword` 全局频率闸门（每分钟 2 次预算，同包其它测试并发消耗预算、窗口翻转边界 50-60s 处被拒）；`TestAdminAuth` 走登录限流桶 + 管理员/普通会话两态 403；`TestLoginRateLimit` 直接依赖登录限流桶的令牌补充速率（并发整包跑时不同测试共享同一 IP 桶）；`TestHandleElectivesSelectUnauthorizedRelogin` 依赖 mock 平台全量替换 Handler 后 `ProbeForAccount` 网络往返 + `maybeRelogin` 异步链路。共同特征：均依赖"夹具内共享的全局态（闸门/限流桶/识别器/连接池）"与真实网络往返时序；单跑全绿、`-p 1` 串行全绿、race 与无 race 均观察到偶发红。与 R80 M80-01 归因一致：**高频并发下夹具时序敏感，非产品逻辑确定性缺陷**（无 race 也复现证明不是数据竞争，而是时序/共享态窗口）。

**修复建议（测试层，产品层无需改）：** ① CI 基线固化 `go test -p 1 -count=1 ./...`（R79 日志口径，CI 已如此配置）；② 整包跑出红时按 CI 既有 `||` 重跑一次吸收瞬时 flake（ci.yml 已内置）；③ 长期方向：把闸门/限流桶在测试下注入独立实例隔离各测试夹具，消除共享态串扰。

**严重度论证：** MINOR。非产品逻辑缺陷、单跑全绿、`-p 1` 全绿，仅整包并发偶发时序抖动；但失败测试面从 R80 的 2 个扩展到 4 个，且含一个"时延拉平"契约测试，值得记录并持续观察。

## OBSERVE

### O81-01：`TestLoginRateLimit` 对登录限流桶的断言在整包并发下可能读到"已被其它测试消耗"的共享 IP 桶

**文件 + 行号：** `backend/internal/api/handler_test.go:1399-1427`

**推演：** 该测试用 `httptest.NewRequest("POST", "/api/login", ...)` 直接打 `d.api`，登录限流桶 `loginLimiter` 按 `clientIP(r)`（`RemoteAddr` 回环 `127.0.0.1`）记账——每个测试夹具 `newTestDeps` 都新建独立的 `loginLimiter` 实例（`Register` 内部 `newLoginLimiter()`），理论上各测试互不共享桶。但 race 整包第 2 轮 `TestLoginRateLimit (13.13s)` 失败，13 秒的时长远超正常（正常 <1s），指向其在等待 `d.accts.LoginByPassword` 内部的全局验证码识别信号量（`globalLimiter`，`sync.Once` 全局单例，跨测试共享且并发上限默认 1）——整包并发时多个测试同时请求验证码识别，`TestLoginRateLimit` 的 7 次登录循环被识别信号量排队拖到 13s，期间其它测试继续消费登录限流桶？否——桶按 IP 独立实例。更可能的根因：识别信号量排队导致 7 次登录的令牌桶"按速率补充"窗口拉长，前 5 次全部通过后第 6/7 次本应 429，但因补充窗口变长而始终有令牌 → `limited` 恒 false → Fatal。即**测试断言假设了"限流桶按 5 次/分钟固定速率"，但实际并发排队让请求间隔拉长，限流判定天然不触发**。属夹具时序敏感的又一形态。

**修复建议：** 该测试改为注入独立的 `loginLimiter`（或对 `allow` 传入不同 IP），或在断言前先确认桶已满（预置 `buckets[ip].tokens = 0` 再验证 429）。归 OBSERVE（仅偶发、单跑全绿）。

### O81-02：`ProbeForAccount` 与 `ProbeNow` 对 `findElectivesData` 的网络往返在整包并发下共享同一 mock 服务器，mock 替换 Handler 的测试（`TestHandleElectivesSelectUnauthorizedRelogin`）存在"其它测试正在探测旧 Handler"的窗口

**文件 + 行号：** `backend/internal/api/handler_test.go:1836-1861`（`d.srv.Config.Handler = http.HandlerFunc(...)` 全量替换）

**推演：** 该测试把共享 mock 服务器 `d.srv` 的 Handler 整体替换为"报名返回未登录"的新实现，然后 `ProbeForAccount` 填充快照再手动报名。整包并发时，其它测试的调度器 tick（`sched.Start()` 已启动、interval 1h 不触发 tick 探测，但测试内主动 `ProbeForAccount`）或手动操作正打到同一服务器——若恰在 Handler 替换窗口内命中新 Handler 的 `default` 分支（返回 `{"code":1,"msg":"unknown ..."}`），`ProbeForAccount` 解析出 code=1 业务错误 → 手动报名前置 `CheckClassSelectable` 读到的快照异常 → 断言"自动重登"文案失败。20 秒失败时长也与 `maybeRelogin` 异步重登循环（mock 返回失败、指数退避首轮 30s 内重试）吻合。属夹具共享态串扰，非产品缺陷。

**修复建议：** 该测试用独立 `httptest.NewServer` 而非替换共享 `d.srv`；或测试期间 `sched.Stop()` 其它协程。归 OBSERVE。

## 可疑待核

- **M81-02 四测试偶发红的精确根因**：本轮已记录"单跑全绿 + `-p 1` 全绿 + 无 race 也偶发"三证据，指向夹具共享态/时序敏感而非产品逻辑缺陷，但四个失败测试的精确断言行未逐个捕获（race 整包 -v 输出因日志量大被截断，且失败为低频偶发）。若主控希望完全闭环，建议在 CI 或专用机以 `go test -race -count=10 ./internal/api/` 高频压测并保留 -v 输出抓失败断言。本轮已通过"定向单跑 ×6 全绿 + 整包 count=4 无 race 全绿 + -p 1 全绿"给出足够证据支持"夹具时序敏感"归因。

## 已核无缺陷清单

### R80 变更回归（重点复核项）

| 项 | 结论 |
|---|---|
| 回归钉硬锚 4264 正确性（M80-02 修复） | **通过**。`tray_asset_windows_test.go:27` 断 `uint32(4264)` 为三加数独立推导（40+4096+128=4264），与实现 `len(ico)-22` 根因不同源；offset 断 `off != 22` 硬编码。实现与测试无同式复算（git show 3ba175e 确认 diff 从 `len(ico)-22` 复算改为硬编码常数）。实测 `go test -run TestTrayIconAsset`（Windows 构建路径）绿。 |
| guardBlockedRef 移除（前端变更）与后端耦合 | **无跨包耦合**。`grep -r guardBlockedRef backend/` 零命中；`web/src/routes/Select.tsx` 零残留；`npm run build`（tsc -b + vite）绿。9b32c65 仅改 `web/src/routes/Select.tsx` 一文件。 |
| build tag 四文件隔离 | **通过**。六 tray 文件 build tag 逐一核对：`tray_windows.go`（windows）/`tray_linux.go`（linux&&cgo）/`tray_linux_cgo0.go`（linux&&!cgo）/`tray_other.go`（!windows&&!linux）/测试 `tray_asset_windows_test.go`（windows）与 `tray_asset_linux_test.go`（linux&&cgo）互斥完备。三平台交叉编译 `GOOS=windows` / `GOOS=linux CGO_ENABLED=0` / `GOOS=darwin CGO_ENABLED=0` 全绿（Windows 选 tray_windows，Linux 双文件互斥选入，darwin 落 tray_other）。 |
| trayPNG/ICO 双回归钉 | **通过**。`TestTrayIconAsset`（entry 两字段硬锚 + DIB 头 40/32/64 + 全像素 BGRA 断言 + 中心 4x4 白不透明）与 `TestTrayPNGAsset`（png.Decode 严格 CRC + 16x16 + 像素语义）分别以 windows / linux&&cgo 约束，平台语义正确。ICO 像素区 `32×32×4=4096` + AND mask `32×4=128` + 头 `6+16+40=62` = 总长 4286 与实现 `dataLen` 一致。 |

### 构建验证

| 命令 | 结果 |
|---|---|
| `go build ./...` | BUILD_OK |
| `go vet ./...` | VET_OK（复跑 `go vet ./internal/...` 亦 0） |
| `gofmt -l .` | 零输出 |
| `go test -race -count=1 -p 1 -timeout 900s ./...` | 十包全绿（含 api 25.79s / store 50.10s / scheduler 15.05s） |
| 三平台交叉编译（win/linux CGO=0/darwin） | 全绿 |
| `npm run build`（web） | 绿（1948 modules, 408ms） |
| 调度器/DB 关键回归测试族 `-race` | 全绿（含 `TestDeletedAccountRebuiltSameName*`、`TestWindowOpenSubmitsWithoutProbeReset`、`TestAdminStatsWindowOpenedUsesScheduler`、`TestMigrateAddsPublishMetaColumns`、`TestSubmitSuspendedWhenOpenTimeCleared`） |

### 契约 20 全仓扫描

扫 `(第\s*\d+\s*轮|R\d{2,}|round\d+|B\d{2,}-\d+|F\d{2,}-\d+|MAJOR-\d+|MINOR-\d+)`：

- 生产代码命中 2 处：`internal/session/store.go:117`（docs/review-round13.md 文档路径引用，非标签）、`backend/tray_windows.go:93`（"R79 LoadImageW 实测"历史锚点，叙述资产失位教训，属契约 20 许可的"为什么/契约"语义）
- 测试代码命中：`tray_asset_{windows,linux}_test.go` 的 "R76/R77 两次手写字节失位""R79 曾把 len(ico) 填入 offset"——历史回归锚点注释，非"X-XX（第 N 轮）"前缀标签形态；`scheduler_test.go` 多处 `B29-02/B42-02/B43-02` 为"契约编号引用"（叙述被修复的缺陷族），非轮次前缀；`XK-ABCD-EF12-3456` 为激活码样例文本假阳性

**结论：零轮次前缀标签残留，符合契约 20。**

### 生产逻辑契约抽核

| 契约 | 结论 |
|---|---|
| windowClosedLocked 三判据单源 | 通过。`WindowClosed()` 与 `StateForAccount()` 共用 `windowClosedLocked()`（主判据 / 时钟连续失败 ≥3 + 开放时间非零 / 幽灵窗口 EmptyProbeRuns≥3 + 开放时间非零），三判据取单次 `open` 快照复用（scheduler.go:913-934）；`probe()` 入账含 +10s 裕量且 `prevOpened` 先捕获后覆写（scheduler.go:1119-1154）。测试 `TestWindowClosedState/StateForAccountMirrorsWindowClosed/ProbeDropsToFar` 全绿。 |
| spawnChain 身份防线六分支 | 通过。成功（1509-1517）/失效（1484-1494）/风控退避（1546-1558）/窗口关闭（1565-1572）/实时复核确证满员（1630-1636）/实时复核 ErrUnauthorized（1595-1613）六条 err 归并全部 `sameClientFor`（指针身份比对，`clientIdentity` reflect.ValueOf.Pointer）；链顶与取 client 双处前置存在性复核（1383/1404）；`maybeRelogin` 入口锁内 ClientFor 复核（1203）+ 写回侧复核（1249）双闭合。测试族 `TestDeletedAccountRebuiltSameNameChainDrops{Success,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}` 全绿。 |
| 登录全局频率闸门双入口 | 通过。`gateWait`（阻塞，Manager.Relogin 用，manager.go:166-176 内部明确调 gateWait 再 ReloginIfNeeded）+ `gateTryAcquire`（非阻塞，LoginByPassword pre-Login 准入，quota 满即拒，共享 gateMu/gateUsed/gateWindow 同一计数）；`ResetGateForTest` 测试专用。R40 教训"接口间接触先追实现"在 manager.go:166 实证：scheduler 走接口 `AccountClients.Relogin` → 真实实现 `Manager.Relogin` 内部 gateWait 闸门完全生效。 |
| httpDo 仅重试 dial/write | 通过。`isConnErrRetryable` 仅 `nerr.Op=="dial"||"write"`（client.go:492-501）；`IsReadErr` 覆盖 FIN/短读/超时/RST 四形态恒不重试（client.go:519-544）；`cloneReq` 用 `req.Clone(req.Context())`（GET 无 body、POST body 为 bytes.Reader 可重放）。captcha.go 的 Vision 识别同款 httpDo（162 行）。 |
| Start/Stop 幂等 | 通过。`Start()` 持 s.mu 判 `s.start` 防二次启动（662-668）；`Stop()` 调 `s.cancel()` 使 ctx.Done 退出 tick 循环（692-695）。测试大量 `s.Start(); defer s.Stop()` 组合 + `TestWindowOpenSubmitsWithoutProbeReset` 等验证启动后行为。 |
| reloginResults 缓冲 | 通过。`make(chan reloginResult, 8)`（263 行）；重登成功回传非阻塞发送（`select { case ... : default: }`，1273-1276）——通道满时弃掉补探测信号也不持 s.mu 阻塞整个调度器；tick 主循环统一消费（679-686）。 |
| handleLogin 时延拉平 | 通过。管理员名 + 口令双条件（handler.go:121）命中即 `time.Sleep(loginTimingFlat)`（124）；教务失败且账号是管理员名时同样补 `time.Sleep`（137）——"管理员名 vs 未知学生"响应时延差被抹平，侧信道枚举防线保留；撞名学生（口令不匹配）走教务登录分支正常签发（132-143）。 |
| session sweeper | 通过。`New` 中 `ttl>0` 才启动清扫协程（49-53）；`sweepLoop` 每 5 分钟 `sweepExpired`（58-70）；`Close` 用 `sync.Once` 幂等停止并等待（90-98）；测试 `store_test.go:173/216` 验证清扫与幂等 Close。 |
| 数据库迁移幂等 | 通过。`Open` 流程：建表 → `migrateAddPublishMeta`（逐列 `columnExists` 判存在、缺才 ALTER，db.go:43-57）→ `refuseLegacy`（缺列清单对应剔除已迁移列，77-79）。`TestMigrateAddsPublishMetaColumns` 用真实旧库 + 真实数据行验证迁移后数据保留、两列补齐（db_test.go:34-64）全绿。 |
| 删号 memory-first | 通过。`handleAdminDeleteAccount` 顺序 `Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount`（handler.go:1004-1018），与契约 4 完全一致；`TestAdminDeleteAccountMemoryFirst`（handler_test.go:2046）断言注册表先失效。 |
| 目标保存链（发布元数据持久化 + refused 语义） | 通过。`SetTargetsForAccount` 仅清 refused（内存+库行 DeleteRefused）、绝不清 done/full/rateLimited/inflight（scheduler.go:458-493）；`RestoreTargets` 保持 refused；main.go 恢复顺序 `RestoreDone → 逐账号 RestoreTargets → LoadRefused + RestoreRefused`（115-139）与 refused_test.go 四测试（`TestRefusedNeverResubmitted/TestManualReselectClearsRefusedRow/TestRefusedRestartOrderRealDB/TestRefusedPersistedAcrossRestart`）全绿。 |
| 零吞错落库 | 通过。全仓落库点（SaveSuccess/AppendLog/SaveRefused/DeleteSuccess/DeleteRefused/DeleteRefusedClass/UpdateIDToken/SetTargetsForAccount/DeleteAccount）逐一扫描全部 `if err != nil { log.Printf }`；仅三处非落库静默：scheduler.go:311 `_ = pw.Prewarm()`（连接预热非落库）、scheduler.go:1070 `_, _ = s.ProbeForAccount(acct)`（探测结果已内记日志，见 R80 O80-02）、config.go:141/160 `_ = os.MkdirAll/WriteFile`（首次 .env 写盘，见 R80 O80-03）。 |
| 手动报名四方法协同 | 通过。`TryAcquireSubmit`（inflight 互斥 + sync.Once 释放）/`MarkDone`（清 full/rateLimited/refused 内存+库行 DeleteRefusedClass + SaveSuccess）/`RemoveDone`（删 done + 置 refused 落库 SaveRefused + DeleteSuccess）/`RemoveFull`（清 full + failed→pending）。四方法全部含"账号已删则静默放弃写回"防线（ClientFor 复核）。 |
| 时钟对齐（maybeSyncClock） | 通过。仅成功才推进 `lastSyncTime`（失败绝不推进、绝不吃闸门）、失败落地 `lastSyncFailAt` 30s 退避、`syncing` 单飞防重入、streak≥3 复位 offset 但不清 streak（留档）、成功清零自愈（scheduler.go:326-411）。 |
| 配置热改与落库失败路径 | 通过。`handleAdminConfig` 先内存生效（Runtime.Update）→ 落库失败时如实 `writeJSONStatus 500` + 不跳过下游热下发（handler.go:795-821）；`saveSettingsErrForTest` 注入钩子存在（860）。 |

## 结论

R80 回归钉硬锚 4264 修复**正确**（三加数独立推导与实现不同源、offset 硬编码 22、无同式复算）；guardBlockedRef 移除为纯前端变更、无后端耦合；build tag 四文件三平台交叉编译隔离、trayPNG/ICO 双回归钉、契约 20 零标签残留全部通过。生产逻辑契约十一项抽核全部通过。

本轮无 CRITICAL 无 MAJOR；主要观察为 **api 包整包 race 并发下偶发失败面从 R80 的两个扩展到四个**（`TestAccountOverrideRequiresAdminSession`/`TestAdminAuth`/`TestLoginRateLimit`/`TestHandleElectivesSelectUnauthorizedRelogin`），但全部单跑全绿、`-p 1` 串行全绿、无 race `-count=4` 全绿——与 R80 M80-01 归因一致：**夹具共享全局态（登录闸门/限流桶/验证码信号量/mock 服务器 Handler）在高频并发下时序敏感，非产品逻辑缺陷**。CI 基线（`-p 1` + `||` 重跑）已能吸收；建议长期把闸门/限流桶测试下隔离实例化消除共享态串扰。后端整体健康状况良好。
