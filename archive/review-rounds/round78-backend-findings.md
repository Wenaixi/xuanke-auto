# R78 后端只读审查报告

- 审查轮次：r78（HEAD 15ad1c7，分支 master）
- 审查范围：backend/main.go、backend/（tray 四文件、browser 双文件、router、embed）+ backend/internal/{config,db,accounts,api,scheduler,secure,session,store,runtime,zhidao}/** 全部 .go（含 _test.go）
- 审查方式：全程只读（Read/Grep/Glob/Bash 只读命令），发现只记录不修改
- 验证手段：Go 官方 `image/png.Decode` + 手写 chunk 解析实测 trayPNG；`go test ./...` 全量通过一次；zhidao 五处裸 mock 宿主连续 8 轮聚焦测试；`go vet` 关键包无告警

---

## CRITICAL

无。

## MAJOR

### M-78-01 托盘资产无回归钉——R76/R77 同类散失在测试侧完全无防护

- 位置：backend/tray_linux.go:70（`trayPNG()`）、backend/tray_windows.go:69（`trayIcon()`），全仓无任何 `_test.go` 引用这两者
- 现象：全仓搜索确认不存在针对手写 PNG/ICO 字节的 Go 解码测试。本轮实证：R76 落地时数组 IDAT 尾 CRC 0x1078074D 与「0x9C zlib 头 + 压缩数据」真实 CRC（0x78BB4E43）不匹配，PIL 无法解码；R77 把 zlib 头改为 0xDA 后 CRC 恰变 0x1078074D、全 chunk 校验通过。两轮都只靠人工对照，没有任何机器检查能拦下"改压缩级忘同步 CRC"这类低级散失
- 触发场景推演：未来任一轮重排/美化这 86 字节手写流，或按"新图标"重新手写 IDAT，手动 CRC 校验疏忽即再次静默产出坏图标——Linux 托盘图标不可见（R76 实际影响）或直接解码失败（PIL 报错形态）。Windows 侧 ICO 同理（32x32 手写 DIB）
- 修复建议：给 `tray_linux.go`/`tray_windows.go` 补一个仅 `//go:build windows && cgo` 或独立于平台 tag 的 `tray_asset_test.go`——`image/png.Decode(trayPNG())` 断言 16x16、中心像素白、四角黑、`image/draw` 逐像素全扫非黑白/非 alpha=255 即 FAIL；Windows 侧用 `golang.org/x/image`（若需）或手写 ICO 头解析断言 32x32 中心 4x4 白。这是把本轮进程外验证固化成仓库内常态防线的落地方式。附带建议：把 `0x1D`（IDAT 长度）等魔数提为命名常量并在函数内自检（`len(idatData)==0x1D`），让"换了数据忘改长度/CRC"在运行时即 panic（托盘启动期）而非静默坏图标
- 严重度论证：虽然当前字节已全部正确、缺陷不可复现，但"零测试护城河 + 全手写二进制资产 + 已两次失位"的组合风险评级为 MAJOR（未来回归风险），非 CRITICAL（无当前数据损坏/安全隐患）

## MINOR

无独立成立项（以下归入"可疑待核"，见下）。

## OBSERVE

无。

## 可疑待核清单（证据不足，不判级）

1. **`handleAdminStats` 的 `log_count` 口径偏小（admin 界面展示值偏小）**：`backend/internal/api/handler.go:886` 用 `LoadAllLogs(1000)` 的结果长度当"全账号日志总数"发给前端（`Admin.tsx:732` 显示"日志条数"）。`LoadAllLogs` 内部把 `limit` 截断到 `max(500)`（store.go:418-419 上限 2000，传 1000 合法），且查询本身有 `WHERE id > max(id)-20000` 的 2 万条窗口。因此该"总数"其实是"最近 ≤1000 条"，窗口期日志超 1000 条时展示值会停滞在 1000。**影响评估**：这是纯展示字段（管理后台一眼看量的粗粒度指标），无任何决策/报警依赖，不构成功能缺陷；且若要准确应改用 `SELECT count(*)`，属于"注释/语义与展示不符"的边缘项。因影响极小且不触发错误行为，未判级，核实后若团队认同可降为 MINOR 或直接改口径。
2. **`fetchLoginPage` 对 4xx/5xx 重试一次的语义边界**：`backend/internal/zhidao/client.go:302-310` 注释声称 403/429 有服务端响应、重试"仅容忍 keep-alive 复用濒死连接时的偶发服务端拒绝，不承诺对限流重试生效"。但同一代码路径对 500 也做一次重试——500 是真实服务端故障而非 keep-alive 濒死形态，重试一次在纯 GET /login 上开销可忽略（不消耗验证码），**不构成缺陷**，仅记录语义边界供后续走读确认。
3. **`close()` 幂等**：`backend/internal/session/store.go:90` `Close()` 依赖 `sweeperClose sync.Once` 幂等，而 `defer sessions.Close()` 与主流程只调一次，无问题，只是顺手核对。

## 已核无缺陷清单（本轮全部重点复核项，逐项验证）

### R77 变更回归

1. **trayPNG IDAT CRC 修正正确性（进程外实测）**：Go 官方 `image/png.Decode` 解码成功、16x16、中心 (6..9,6..9) 共 16 白像素、四角纯黑、alpha 全 255；手写解析器逐 chunk 校验 IHDR/IDAT/IEND 的 CRC 全部 OK（IHDR=0x1FF3FF61、IDAT=0x1078074D、IEND=0xAE426082）；zlib 解压 1040 字节（16 行 x 65 字节，滤波 0），两次独立实现交叉吻合。**结构性结论**：R76 0x9C 头时 CRC 应为 0x78BB4E43（与存储 0x1078074D 错位），R77 换 0xDA 头后恰好修正为 0x1078074D——修复正确、像素级无误。
2. **R76 残留**：无（R77 已彻底修掉该缺陷）。

### 生产逻辑关键契约全表

3. **窗口关闭三判据单源 `windowClosedLocked`**（scheduler.go:913-934）：主判据（曾开窗+空快照+已过开窗点+10s 裕量）/ 时钟失败≥3+开放时间已过 / 幽灵窗口 EmptyProbeRuns≥3+开放时间已过，三路都取**单次 open 快照**；`StateForAccount` 与 `WindowClosed()` 同源调用。系统读侧 `probeIntervalFor`/`tick` 均已让位。
4. **`tick` 零值守卫让位 `WindowOpened`**（scheduler.go:1006-1011）：零值守卫 `open.IsZero() && !opened` 才能挂起提交，`WindowOpened=true`（发布级 inDateRange 确证）时即使识别槽为空也不挂起。守护测试 `TestSubmitSuspendedWhenOpenTimeCleared`（scheduler_test.go:1334）保持绿（正反对照两段）。
5. **删号 memory-first**（handler.go:1004-1018）：`Accounts.Remove` → `PurgeAccount` → `Store.DeleteAccount` → `Sessions.RevokeAccount` 顺序正确，在飞链锁内复核 ClientFor 立即失败。
6. **`sameClientFor` 六分支**（scheduler.go:208-228）：成功/失效/风控退避/窗口关闭/实时复核 ErrUnauthorized/实时复核确证满员六处全部指针身份比对；配套测试齐备（TestDeletedAccountRebuiltSameNameChain{DropsSuccess, DropsRelogin, DropsRateLimitBackoff, DropsWindowClosedFull, DropsRealtimeRecheckFull, RealtimeUnauthorizedDropsRelogin, SuccessDropsInflight}），waitChainExit 用 chains 活跃标记等待而非 inflight。
7. **`IsReadErr` 四形态**（client.go:519-548）：io.EOF / io.ErrUnexpectedEOF / 两种超时文案 / `net.OpError.Op=="read"` 全命中，dial/write 不命中；`isConnErrRetryable` 与它互斥（dial/write 重试、read 不重试）。`isreaderr_test.go` 形态矩阵测试覆盖 RST/FIN/短读/awaiting headers/reading body/dial/write/nil。
8. **doLogin 全局频率闸门**（manager.go:49/223/244）：`gateTryAcquire` 非阻塞准入收口手动登录与管理员换绑；`gateWait` 收口自动重登（scheduler 走 `AccountClients.Relogin` → `Manager.Relogin` 内部明确 gateWait）。api 夹具用 `ResetGateForTest()` 复位；我用 `go test` 三轮 + 全量验证无卡死。
9. **零吞错落库点**（全仓清点）：scheduler 侧 8 处 AppendLog + SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefused/DeleteRefusedClass/UpdateIDToken/SetTargetsForAccount/DeleteRefused 全部 `if err != nil { log.Printf }`；api 侧 6 处 AppendLog 同样记日志。唯一 `_ =` 是 `_ = d.Sched.MarkDone/RemoveDone`（handler.go:363/431）——其内部落库错误自身已记日志，且两处返回错误无额外调用方信息，属实收口。`TestStoreFailuresLogged` 覆盖 SaveSuccess/SaveRefused 失败路径。
10. **httpDo 仅 dial-write 重试**（client.go:474-501）：`isConnErrRetryable` 只认 `*net.OpError.Op` 为 dial/write；请求体由 `cloneReq` + bytes.Reader/strings.Reader 的 GetBody 重放。FetchLoginPage 的重试同样只覆盖连接层 + 4xx/5xx 一次（纯 GET /login 不消耗验证码限额）。
11. **config 双默认值**：`Config` 无 `OpenTime` 字段（B40-02 已清死配置）；`env_test.go` 有 `TestConfigDoesNotInjectOpenTime` 守护。
12. **probe 空快照不删识别槽**：ProbeForAccount（scheduler.go:847-852）与 probe（scheduler.go:1101-1106）只在 `len(data.BeginTimes)>0` 时覆盖写入识别槽，空快照绝不 delete；`open_retain_test.go` 的 `TestOpenTimeRetainedAfterWindowClosed` 固化。
13. **zhidao 五处裸 mock 宿主 flake**（client_test.go: TestNoAutoRelogin/TestReloginIfNeeded/TestLoginRetriesTransientInitError/TestExitClass/TestExitClassFailsOnCodeNotZero）：均已覆盖 socketPreheat + readyProbe 三包成族（TestMain 包级预加热 + loginMockServer 内建探测 + 全量轮兜底）；连续 8 轮聚焦运行（3 轮五合一 + 5 轮双合一）零 flake；全量包一次通过。
14. **http.Server 显式超时**（main.go）：ReadHeaderTimeout 10s / Read 30s / Write 30s / Idle 120s，slowloris 防护在位。
15. **凭据与会话安全**：AES-256-GCM（secure/crypto.go）+ token 32 字节 hex + 登录限流（按 IP 桶 + 可信反代开关）+ 票据 5 分钟单次 + 会话 12h TTL + 删除账号吊销会话；`handleAdminConfig` PUT 校验值域（引擎/并发越界整体拒绝）；`handleLogin` 管理员名+口令双条件 + 时延拉平。
16. **数据库迁移规范**：`migrateAddPublishMeta` 在 `refuseLegacy` 之前、逐列 `columnExists` 判存在、缺才 ALTER；`TestMigrateAddsPublishMetaColumns` 守护（真实旧库行保留）。`refuseLegacy` 缺列清单已剔除已迁移列。
17. **SPA 兜底 /api 404**（web/embed.go:28-31 + router.go:194）：`/api` 精确与 `/api/` 前缀都 404，不落 index.html；`writeJSONStatus` 先设头再 WriteHeader。
18. **go vet 通过**：`go vet ./internal/scheduler/ ./internal/zhidao/ ./internal/api/` 无告警；`go test ./... -count=1` 全部包 ok。

## 结论

唯一定级发现 M-78-01（托盘资产无回归钉）为**未来回归风险**而非当前缺陷；其余全部历史重点契约与 R77 修复核对通过。全量测试绿、单包多次聚焦无 flake、go vet 干净。建议下一轮在落地 M-78-01 时同步把 `cmd/probe` / `handler.go:1158` / `captcha.go:172` 的 `min` 内建复查一遍（Go 1.26 环境确认可用，非问题）。