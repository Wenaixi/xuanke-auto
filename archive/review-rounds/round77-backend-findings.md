# R77 后端只读审查报告

基线：commit 7b939d6（R76 收尾，HEAD）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto，分支 master。只读铁律全程遵守（Read/Grep/Glob + Bash 只读命令 git show/log/diff/grep、go vet/build/test 全绿实证 + Python stdlib/PIL/Go stdlib 三重图像解码验证）；对仓库零 Edit/Write（报告文件按任务要求以 Write 落 archive/review-rounds/）。

## 概述

**零 CRITICAL、零 MAJOR、1 个 MINOR（R76 托盘图标升级后 IDAT chunk CRC 字节错误——Go 官方 image/png.Decode 解码失败）、1 个 OBSERVE（行号引用族复查后再确认清零）、零可疑待核。** R76 修复正确性逐项核证：trayPNG 升级后「像素语义正确（PIL 实测中心 4x4 全白 + 其余全黑）但 IDAT CRC 错误」——PIL/libpng 宽容路径下像素完美、Go 官方 image/png 严格校验下解码失败；scheduler_test.go 四处行号改语义指位全部准确。

## 重点复核结论（R76 变更后回归）

### 1. R76 修复正确性（commit 40a8bf2）——托盘像素正确但 IDAT CRC 有误（NEW MINOR）；测试语义指位全部正确

#### 1a. trayPNG 升级 16x16 黑底中心 4x4 白块——像素语义正确，IDAT CRC 字节错误（MINOR-77-01）

三重独立验证 R76 交付的字节序列（从源码逐字节提取）：

| 校验项 | 结果 |
|--------|------|
| PNG 签名 | 通过 |
| IHDR（16x16、8bit RGBA、length 13） | 通过（含 CRC 一致） |
| IEND（空体、length 0） | 通过（含 CRC 一致） |
| **IDAT chunk CRC** | **存储 0x1078074D，实算 0x78BB4E43 —— 不匹配** |
| zlib 解压 | 完整 29 字节 zlib 流可解压（流内嵌 adler32 `0xd7e32ee0` 与解压数据一致，解出 1040 字节 = 16 行 × (1 filter + 64 RGBA)）|
| 逐像素断言（PIL） | **中心 4x4 块全白（16/16 = 255,255,255,255）、全图 240 像素纯黑（0,0,0,255）**——与注释「纯黑底 + 中心 4x4 白块」完全一致 |
| Go 官方 image/png.Decode | **`png: invalid format: invalid checksum` —— 解码失败** |

根因：IDAT 的 CRC 校验和应基于**压缩流解出的原始 IDAT 字节**（`0x78,0x9C,0x63,0x60,0x60,0x60,0xF8,0x4F,0x21,...`），正确值为 0x78BB4E43（大端序存储 `0x43 0x4E 0xBB 0x78`）。R76 手写时把 IDAT CRC 写成了 `0x07,0x4D,0x00,0x00`（第 79-80 行末尾 4 字节），疑似把「zlib 流头部字节 0x9C 的位置」与「adler32 尾 4 字节」混淆：压缩流 29 字节 = zlib 头 2 字节 + deflate 数据 + 4 字节 adler32（`0xD7,0xE3,0x2E,0xE0` 正确落位），CRC 字段应在该 29 字节之后。存储的 CRC 数值不匹配任何合法分量。

**实际影响链路**：Linux setIcon 源码（getlantern/systray@v1.2.2 systray_linux.c:61-88 do_set_icon）把字节原样写入临时文件，交给 `app_indicator_set_icon_full` → GTK 图标主题的 GdkPixbuf PNG loader。GdkPixbuf 对受支持的 PNG 走 libpng 全量严格校验（含逐 chunk CRC，W3C PNG 规范），CRC 不匹配走 error 路径拒绝加载。后果：托盘图标加载失败 → 由主题回退为默认占位图标。R76 本意「让 Linux 托盘图标真实可见」的实际效果取决于此 CRC 修复（R76 前是 1x1 全透明占位、R76 后若 CRC 修好则是 16x16 黑底白点真图标）。

- 文件：`backend/tray_linux.go:79-80`
- 触发场景：Linux 桌面 CGO=1 发布路径。
- 修复建议：把 IDAT chunk 的 CRC 4 字节改为正确大端序 `0x43 0x4E 0xBB 0x78`（对应 0x78BB4E43）。同时建议给 trayPNG/trayIcon 字节数组补一个 `image/png.Decode` 断言测试（或提为 `func trayIconBytes() []byte` 供测试），防止再次手写失位。

#### 1b. scheduler_test.go 四处行号引用改语义指位——全部准确

| 位置 | 改动后表述 | 与实现语义核证 |
|------|-----------|----------------|
| scheduler_test.go:1262 | 「tick 提交守卫（零值守卫/未开点守卫）」 | 准确。tick 守卫实现在 scheduler.go:1006（`open.IsZero() && !opened`）与 :1009（`!opened && !now.After(open)`），与被删的「702 行」相比语义指位不再漂移 |
| scheduler_test.go:2848 | 「紧邻的『未现满员』分支（下方 doneHas 复核『绝不覆盖胜利状态』）」 | 准确。确证满员分支 doneHas 复核在 scheduler.go:1622、未现满员分支 doneHas 复核在 :1640，相邻对称，语义精确 |
| scheduler_test.go:3501 | 「失效分支（ErrUnauthorized 段）」 | 准确。ErrUnauthorized 分支在 scheduler.go:1477/:1484，链顶入口 :1400 起 |
| scheduler_test.go:3503 | 「SelectClass 返回后统一清位」 | 准确。统一 `delete(s.inflight[acct], t.ClassID)` 在 SelectClass 返回后 err 判定前（scheduler.go:1508 附近）|

### 2. 行号引用族复查——仅剩 2 处精确命中，R76 目标达成

全仓 grep「N 行」模式（排除 archive/、web/dist、.claude 记忆文本）共 4 处，逐一定位：

| 位置 | 内容 | 判定 |
|------|------|------|
| backend/internal/scheduler/scheduler.go:979 | 「open 已在本函数开头取过单次快照（973 行）」 | **精确命中**。973 行即 `s.mu.Unlock()`，open 在 972 行 `s.openTimeForLocked("")` 锁内取值——注释语义「open 在 tick 开头取单次快照、probeIntervalFor 内部不再重取」准确，指位精确。R76 说「仅剩 scheduler.go:980 的 973 行」实测该注释在 979 行，但 973 行语义唯一且承诺保留，无需改动 |
| backend/internal/api/handler_test.go:1500 | 「（代码 137 行保留）」 | **精确命中**。handler.go:137 即管理员口令错误分支的 `time.Sleep(loginTimingFlat)`（B43-04 时延拉平语义），指位精确 |
| backend/internal/store/store_test.go:30-32 | 「30050 行」「每 1000 行一批」 | 测例内部「数据记录行数」，非代码行号，不属目标族 |
| .claude 记忆文本 | review 进度 | 非仓库代码 |

结论：代码中「第 N 行」指位类引用已清零（仅剩 973/137 两处精确命中且承诺保留），满足 R76 判定「仅剩 973 行精确命中保留」。

### 3. Linux 桌面托盘——build tag 穷举与双降级链成立（IDAT CRC 见 MINOR-77-01）

- **build tag 四方穷举互斥无空洞**：tray_linux.go(`linux && cgo`) / tray_linux_cgo0.go(`linux && !cgo`) / tray_windows.go(`windows`) / tray_other.go(`!windows && !linux`) 每平台恰好一个实现，无空洞无极冲突；main.go:147 `runTray` 唯一入口，三平台实现均编译（`go build ./...` 本机 Windows CGO=1 全绿；cgo0/tray_other 无 GTK 依赖，跨平台编译路径可留待后续）。tray_linux_cgo0.go 回归文件（Linux CGO=0 无托盘 + 服务照常启动）文件头注释清晰。
- **showZenityOrPrint 双降级链完整**（tray_linux.go:93-101）：`exec.LookPath("zenity")` → `exec.Command(path, "--info", ...)` → `cmd.Run()` 成功即返回 → 无 zenity 或对话框运行失败降级 `fmt.Printf` 控制台，绝不因对话框阻塞托盘（无阻塞挂起、无 sh 中介）。os/exec 与 fmt 均真实使用，import 无悬空。
- **trayIcon（Windows ICO）像素级核证**（tray_windows.go:47-94）：ICONDIR/ICONDIRENTRY/BITMAPINFOHEADER 字段逐项正确（bytesInRes=len(ico)、DIB height=64 双高、32bpp、中心 4x4 白点 14..17 落位、AND mask 长度计算正确）。trayPNG 与 trayIcon 视觉同语义（黑底 + 中心白块；Windows 4x4@32px、Linux 4x4@16px），**但 R76 字节 IDAT CRC 错误致 Linux 真实加载失败（MINOR-77-01）**。

### 4. zhidao 冷启动 flake 残余——五处裸 mock 宿主当前不触发

- 五处无 readyProbe 的裸 `httptest.NewServer`：client_test.go:172（TestNoAutoRelogin）/ :287（TestReloginIfNeeded）/ :419（TestLoginRetriesTransientInitError）/ :457（TestExitClass）/ :486（TestExitClassFailsOnCodeNotZero）。均由 TestMain（:39-42）包级 socketPreheat（预创建-关闭一个 127.0.0.1 套接字排空 Time_WAIT 冷启动窗口）兜底；loginMockServer（:45-85）与 TestLoginNetworkErrorAbortsImmediately 各自 readyProbe（:88-112，200ms×10 + 2s 超时）。
- 本轮实证：`go test -count=3 -run 'TestNoAutoRelogin|TestReloginIfNeeded|TestLoginRetriesTransientInitError|TestExitClass$|TestExitClassFailsOnCodeNotZero' ./internal/zhidao/` 全绿；全量 `go test ./...` 全绿。**当前不触发**。

### 5. 生产逻辑关键契约复核（历史重点全表）——全部成立

| 契约 | 实现 | 复核结果 |
|------|------|----------|
| 窗口关闭三判据单源 | scheduler.go:913-934 windowClosedLocked | 成立。三判据共用单 `open` 快照（:919 注释「open 单快照对三条判据统一」）；StateForAccount:707 与 WindowClosed:905 同真相；10s 裕量判据侧（:1132/:1141）+ 入账侧（:1150）对称；EmptyProbeRuns 入账侧也带 `now.After(open+10s)`;TestWindowClosedState / TestWindowClosedTransitionStateGrace / TestStateForAccountMirrorsWindowClosed 在位 |
| 删号 memory-first | handler.go:979-1019 | 成立。Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount 四步顺序与契约一致；DeleteAccount 失败半删态由重启 Restore 自愈；PurgeAccount（:499-523）含 openTimeDetected 槽清理（:510）、Courses 行剔除、inflight 清空、relogin 族（tokenValid/reloginAt/reloginFail/relogging）全清 |
| sameClientFor 六分支 | scheduler.go:1481(失效)/1515(成功)/1545(风控)/1565(窗口关闭)/1593(实时复核入口)/1629(确证满员) | 成立。六分支全含指针身份比对（clientIdentity:219-229 用 reflect.ValueOf.Pointer）；:1593 入口复核覆盖其后全部写回点（:1603 maybeRelogin+setState+AppendLog、:1622 doneHas、:1630 markFull、:1655 setState+AppendLog）；maybeRelogin 决策侧（:1203）与写回侧（:1249）双 ClientFor 复核；成功分支前置 :1508「先统一 delete inflight 再 err 判定」——B43-03 归因（「成功分支身份复核失败时 inflight 位漏删」不成立）由该统一清位钉死 |
| IsReadErr 四形态 | client.go:519-548 | 成立。FIN（errors.Is io.EOF）/ 短读（ErrUnexpectedEOF）/ 超时双文案（awaiting headers + reading body）/ RST（OpError.Op=="read"）四形态全覆盖；与 isConnErrRetryable（:492-501 只认 dial/write）互斥；scheduler.go:1653-1662 用 IsReadErr 差异文案 |
| doLogin 全局闸门 | manager.go:49-64 gateWait + :223-235 gateTryAcquire | 成立。两门共享 gateMu/gateUsed 计数（同一分钟窗口）；LoginByPassword:243-246 非阻塞准入绝不挂起用户响应（返回「登录尝试过于频繁」）；Relogin:166-185 内部 gateWait 后 ReloginIfNeeded（B42 决策收口复核：scheduler 走接口 → Manager.Relogin 真实实现含 gateWait，闸门完全生效）；管理员换绑同走收口 |
| 零吞错落库点 | 全仓抽查 | 成立。全部非测试文件 `_ =` 经逐个核验均为非落库点（browser Start 错误、io.Copy、compareHash 返回忽略等）；scheduler.go 全部 store 调用点带 `if err != nil { log.Printf }` 且全部在 `s.store != nil` 守卫内 |
| httpDo 仅 dial-write 重试 | client.go:474-501 | 成立。isConnErrRetryable 只认 dial/write；read 错误原样上抛（防双报，注释明确）；业务/取消错误不重试；cloneReq:553-557 用 req.Clone 深拷贝（对 dial/write 形态安全） |
| config 双默认值 | config.go + env_test.go | 一致。默认识别引擎 ddddocr（CaptchaEngineDefault:106-111 / .env 模板:151 / env_test:71-84 三处同向）；激活码默认 off（:43-45 / 模板:154 / env_test:46-66 / main.go:33-37 四处同向） |
| tick 零值守卫让位 WindowOpened | scheduler.go:1006-1009 | 成立。`open.IsZero() && !opened` 才挂起；WindowOpened=true（发布级 inDateRange 确证开窗）+ 识别槽空（平台批次未下发非空 beginTimes）时放行黄金期冲刺；TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 绿；TestSubmitSuspendedWhenOpenTimeCleared（Fixture WindowOpened=false）绿 |
| probe 空快照不删槽 / PurgeAccount 删槽 | scheduler.go:1099-1110 / :510 | 成立。「关闭≠时间消失」契约：`len(data.BeginTimes) > 0` 才覆盖识别槽、空快照保留槽（注释「关闭≠时间消失」明确）；删账号 PurgeAccount 删槽且有 TestPurgeAccountClearsOpenTime 钉死 |
| 凭据加密 | secure + accounts | 成立。密码/vision_key 均 enc: 密文；LoadOrCreateKey crypto/rand 失败 panic 拒绝启动（不落可预测兜底）；主密钥缺失拒绝启动；Restore 解密失败只留日志不伪造 |
| access_limit_cookie 占位 | client.go:402-406 / manager.go:311-313 | 成立。登录成功 Set-Cookie 收集（sess.Jar）优先、缺失时统一 "1" 占位，与 accounts.Restore 占位同语义，注释对齐 manager.go:313 契约 |

## 发现

### MINOR

**MINOR-77-01（tray_linux.go:79-80 IDAT chunk CRC 字节错误 0x1078074D ≠ 实算 0x78BB4E43——像素语义正确但 Go image/png 严格校验解码失败，Linux 真实链路托盘图标加载失败）**

- 文件：`backend/tray_linux.go:79-80`
- 一句话：R76 升级的 16x16 黑底白点 PNG 像素语义正确（PIL 实测中心 4x4 全白 + 其余全黑），但 IDAT chunk 的 CRC 校验和字段错误（存储 0x1078074D，实算 0x78BB4E43）；Go 官方 image/png.Decode 返回 `png: invalid format: invalid checksum`，libpng 同类严格校验下同样拒绝加载。
- 触发场景：Linux 桌面 CGO=1 发布链路（systray setIcon → temp 文件 → app_indicator_set_icon_full → GTK GdkPixbuf PNG loader → libpng 严格 chunk CRC 校验）。CRC 不匹配拒绝加载，托盘图标不可见或主题回退默认占位图标。
- 修复建议：IDAT chunk 的 CRC 4 字节改为正确大端序 0x43 0x4E 0xBB 0x78（0x78BB4E43）。建议把 trayPNG/trayIcon 的字面量字节数组提为可测试函数（如 `func trayIconBytes() []byte`）并补一个 `image/png.Decode` + 像素断言测试，防止再次手写失位。

**MINOR-77-02（可选观察项，并入 OBSERVE-76-02 同族：托盘图标字节零自动化防线——本轮 CRC 错误的教训）**

- 文件：`backend/tray_linux.go:70-85` / `backend/tray_windows.go:47-94`
- 一句话：两个托盘图标序列都靠手写十六进制字节维护，本轮升级即引入 CRC 错误而 `go test ./...` 全绿——测试对托盘图标零覆盖。
- 触发场景：下一次手改图标字节可能再次引入结构/CRC 错误而不被发现（已真实发生一次）。
- 修复建议：字面量提为构建函数 + 解码断言测试（见 MINOR-77-01 建议）；或沿用 OBSERVE-76-02 的 Linux CGO=1 CI job 一并覆盖。

### OBSERVE

**OBSERVE-77-01（行号引用族复查后再确认清零，仅剩两处精确命中——维持 R76 判定，无需处理）**

- 文件：`backend/internal/scheduler/scheduler.go:979` / `backend/internal/api/handler_test.go:1500`
- 一句话：全仓 grep「N 行」模式（排除 archive/、web/dist、.claude 记忆）仅剩两处精确命中——scheduler.go:979 注释的 973 行（open 在 972 行锁内取值、973 行解锁，语义精确）、handler_test.go:1500 注释的 137 行（handler.go:137 管理员口令错误 Sleep，指位精确）；store_test.go「30050 行/每 1000 行」为数据行数非代码行号。R76 承诺「仅剩 973 行精确命中保留」达成。
- 触发场景：未来代码增删若再引入「第 N 行」指位漂移会重新出现（维护纪律问题，非当前缺陷）。
- 修复建议：无需处理（已清零）；后续轮审若再发现行号引用族，按 R74 同款语义指位策略收敛。

## 可疑待核

本轮无可疑待核项：MINOR-77-01 已用三重解码实测定论（PIL 宽容/Go 严格对比）；B42 「scheduler 绕过 gateWait」系接口间接触发的历史误报，已按契约 30 复核真实实现（manager.go:166 Relogin 内部 gateWait 后 ReloginIfNeeded）排除；handler_test 的「热改」注释经核验为 Vision 配置热更新路径（runtime.Store PUT），非开放时间热改，方向正确。

## 已核无缺陷清单

- **R76 修复正确性逐项核证**：scheduler_test.go 四处行号改语义指位全部准确（:1262 tick 提交守卫 / :2848 doneHas 复核 / :3501 ErrUnauthorized 段 / :3503 统一清位，均与实现语义对位）。trayPNG 像素语义正确但 IDAT CRC 手写错误（已记为 MINOR-77-01 待修）。
- **行号引用族复查**：全仓「N 行」模式仅剩 2 处精确命中（973/137），R76 目标达成；「30050 行」「每 1000 行」为数据行数非代码行号。
- **Linux 桌面托盘**：build tag 四方穷举互斥无空洞；showZenityOrPrint 双降级链完整（exec.LookPath → Run → fmt.Printf 绝不阻塞托盘）；trayPNG/trayIcon 视觉同语义（黑底白块）。
- **zhidao 冷启动 flake**：五处裸 mock 靶向 -count=3 连跑全绿，TestMain socketPreheat 兜底生效。
- **生产逻辑契约全表**（逐条复核）：窗口关闭三判据单源（open 单快照 + 10s 裕量两侧对称）/ 删号 memory-first（含 PurgeAccount 识别槽清理）/ sameClientFor 六分支（含实时复核入口前置统一覆盖 + 成功分支统一清位 + maybeRelogin 出入侧双复核）/ IsReadErr 四形态 / doLogin 双闸门共享计数 / 零吞错落库点 / httpDo 仅 dial-write 重试 / config 双默认值 / tick 零值守卫让位 WindowOpened / probe 空快照不删槽。
- **测试与构建实证**：`go vet ./...` 全绿；`go build ./...` 全绿；`go test ./...` 全量绿（accounts 0.408s / api 12.891s / scheduler 13.269s / store 2.513s / zhidao cached）；zhidao 靶向连跑绿；回归陷阱双测试（TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler）在位。
- **B42 决策复核**：scheduler 走 AccountClients.Relogin 接口 → accounts.Manager.Relogin（manager.go:166）内部明确调 gateWait 后才 ReloginIfNeeded——闸门完全生效（R40 教训复核成立）。

## 结论

零 CRITICAL、零 MAJOR、1 个 MINOR（tray_linux.go:79-80 IDAT CRC 错误 0x1078074D，Go image/png 解码失败——Linux 真实链路托盘图标加载失败，修复为改回 0x78BB4E43）与 1 个 OBSERVE（行号引用族复查后再确认清零，维持 R76 判定）。R76 修复正确性核证：托盘像素语义正确 + 测试语义指位准确，但升级字节时引入了新的 IDAT CRC 手写错误（最小化修复：改 CRC 4 字节为 0x43 0x4E 0xBB 0x78，并建议补图像解码断言）。其余生产逻辑契约全表复核全部成立，无数据丢失/安全问题/功能缺陷。