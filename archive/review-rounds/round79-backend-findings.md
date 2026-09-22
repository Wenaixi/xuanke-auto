# R79 后端只读审查报告

审查对象：xuanke-auto HEAD commit 2aefe2f（R78 收官），重点复核 R58→R78 修复链（托盘资产回归钉 / Windows ICO 结构 / trayPNG CRC），并做生产逻辑契约抽查与全量构建验证。

审查方式：全程只读；对 ICO/PNG 资产用独立 `%TEMP%` 目录临时 `go run` 程序实测（用后即删），对调度器关键路径跑受限 `go test -run`。未修改任何仓库文件（工作树仅前端两个文件是 R78 遗留未提交，非本次审查所致，见脚注）。

---

## CRITICAL

无。

## MAJOR

### M79-01：tray_windows.go trayIcon() 的 ICONDIRENTRY dwBytesInRes / dwImageOffset 字段与值双重写错，Windows 托盘图标经 LoadImageW 必然加载失败

**文件 + 行号：** `backend/tray_windows.go:91-99`

**缺陷推演：**

`trayIcon()` 生成的 ICO 文件整体 4286 字节，其中 22 字节为 ICONDIR(6) + ICONDIRENTRY(16)，其后的 4264 字节为 BITMAPINFOHEADER(40) + 像素(4096) + AND mask(128)。

ICONDIRENTRY 的 16 字节布局（Microsoft ICO 格式规范，与 Vista/XP 一脉相承）为：

| 偏移 | 字段 | 长度 |
|---|---|---|
| 0-3 | bWidth / bHeight / bColorCount / bReserved | 4 |
| 4-5 | wPlanes | 2 |
| 6-7 | wBitCount | 2 |
| 8-11 | **dwBytesInRes**（该图像数据区的总字节数） | 4 |
| 12-15 | **dwImageOffset**（从文件头到图像数据的字节偏移 = 目录区结束后第一个字节 = ICONDIR 6 + 条目 16 = 22）| 4 |

当前实现：

```go
dataOff := headerSize           // 22
ico[14] = byte(dataOff) ...     // [14:18] 写入 22
ico[18] = byte(len(ico)) ...    // [18:22] 写入 4286
```

即 `[14:18]`（dwBytesInRes 位置）填了 `22`，`[18:22]`（dwImageOffset 位置）填了 `4286`。正确值应为 dwBytesInRes=4264（数据区总长 40+4096+128）、dwImageOffset=22。当前是两个字段的值都错，且 4286 越出数据区、22 不足像素区。

**触发场景（Windows 实测，非推断）：**

systray 的 Windows 链路 `SetIcon` → `iconBytesToFilePath` 写入临时 .ico → `LoadImageW(icon_path, IMAGE_ICON, 0, 0, LR_LOADFROMFILE|LR_DEFAULTSIZE)`（`systray_windows.go:705-736`）。本审查用独立临时程序复刻 trayIcon() 字节，调用 `user32.LoadImageW` 实测：

- 当前字段值（bytesInRes=22, imgOff=4286）→ **LoadImageW FAILED**
- 正确字段值（bytesInRes=4264, imgOff=22）→ **LoadImageW OK**；`GetIconInfo` 返回 32x32、bpp=32，逐像素 BGRA 黑底 + 中心白正确
- 系统基准 `C:/Windows/System32/OneDrive.ico`（bytesInRes=16936=数据全量、imgOff=134=目录后首个图像起点）→ OK，逐字节解析确认上述字段语义与参考实现一致
- AND mask 缺失时即使字段正确也 FAIL（当前实现含 AND mask，结构完整，仅字段值错）

结论：Windows 上 `runTray` 里 `systray.SetIcon(trayIcon())` 在 `onReady` 时被 LoadImageW 拒绝，图标加载失败。systray 对失败静默（`log.Errorf` 不对外），托盘图标显示系统默认图标或空白。

**为什么回归钉没抓住：** `tray_asset_windows_test.go:17-33` 只校验 ICONDIR 头（14-17 行）与 DIB 头（26-34 行）以及像素区，**完全不解析 ICONDIRENTRY 的 14-21 字节**，恰好漏掉两个错误字段。R78 修复把 DIB 头字段（biWidth 32 位/planes）修正确了，但 entry 字段错位延续自 R71 初始实现且回归钉不覆盖，是"修复不完全 + 测试盲区"双遗留。

**严重度论证：** 不影响选课主链路（调度器/报名/鉴权全部独立），不崩溃、不损坏数据；但 R71 引入的 Windows 托盘是桌面用户体验核心（常驻图标、右键菜单 打开浏览器/关于/退出），图标不可见即为该特性的功能性报废，真实、必然触发。判 MAJOR。

**修复建议：** 交换 `[14:18]` / `[18:22]` 的写入值，并让 dwBytesInRes = 数据区总长（`len(ico)-22`）：

```go
const headerSize2 = 6 + 16
dataLen := len(ico)
binary.LittleEndian.PutUint32(ico[14:18], uint32(dataLen-headerSize2)) // dwBytesInRes = 数据区总长
binary.LittleEndian.PutUint32(ico[18:22], uint32(headerSize2))        // dwImageOffset = 22
```

同步给 `tray_asset_windows_test.go` 补两行断言覆盖 entry 字段：读取 `binary.LittleEndian.Uint32(ico[14:18])==4264`、`binary.LittleEndian.Uint32(ico[18:22])==22`——目前该测试的假绿恰好掩盖了本条缺陷。

## MINOR

无。

## OBSERVE

### O79-01：tray_asset_windows_test.go 回归钉未覆盖 ICONDIRENTRY 字段（M79-01 的测试盲区）

随 M79-01 联动。`tray_asset_windows_test.go` 从字节 6-21 只校验了 bWidth/bHeight/planes/bitCount，对 `[14:18]`/`[18:22]`（dwBytesInRes/dwImageOffset）没有任何断言。正是这个漏项让 M79-01 存活到 R79。修复 M79-01 时应一并补上，纳入"只要 entry 字段回归即红"的范围。

### O79-02：R78 提交后前端两个文件工作树未提交

`git status` 显示 `web/src/routes/Admin.tsx` / `web/src/routes/Select.tsx` 处于 modified 状态（R78 提交 14f655e 之后又有改动），本轮后端审查不判定，但主控应知悉前端分支存在未落盘的改动。

## 可疑待核清单

（无未定级项；本轮的 LoadImageW 实测结论已落地为 MAJOR，其余走读项均有明确结论。）

## 已核无缺陷清单（本轮重点复核项逐项结论）

| 复核项 | 结论 |
|---|---|
| tray_asset_linux_test.go 用 `io.Reader` 包 `png.Decode`，与真实链路同口径 | 无缺陷。systray Linux 端 `SetIcon` → `g_bytes_new_static` → 写临时文件 → `app_indicator_set_icon_full`（GdkPixbuf 从文件解码），`image/png.Decode` 走同一 PNG chunk 解析，CRC 严格校验口径一致；`io.Reader` 包不影响包层语义。实测：trayPNG 字节经 `png.Decode` 解码通过（CRC/结构合法），16x16、中心 6..9 白(255)、四角黑(0)、alpha 全 255 |
| trayPNG 中心 4x4 与验证断言是否自洽 | 无缺陷。tray_linux.go:114 判据 `x>=6&&x<=9&&y>=6&&y<=9` 与测试断言一致；解码实测每个像素与断言一一吻合 |
| tray_asset_windows_test.go 像素偏移公式 | 无缺陷。pixStart=22+40=62；中心 14..17 BGRA=[255,255,255,255]、角落=[0,0,0,255]，逐字节铺位与 DIB BGRA 顺序完全吻合，且 `GetIconInfo` 实测确认 32x32 bpp=32（前提是修正 M79-01 的 entry 字段后） |
| DIB 头字段（biSize/biWidth/biHeight/biPlanes/biBitCount）逐位顺序 | 无缺陷。R78 修正后 biSize=40、biWidth=32（完整 4 字节）、biHeight=64（XOR+AND 双高）、biPlanes=1、biBitCount=32，逐字节解析与 GetIconInfo 实测一致，无串位无越界 |
| AND mask 存在性与像素偏移影响 | 无缺陷。`andMaskBytes=andRowBytes*height=32*4=128`，数据区 4264 = 40+4096+128；实测 AND mask 缺失时 LoadImageW 失败、在场时成功，结构完整 |
| build tag 隔离（linux&&cgo / windows / !cgo 占位）| 无缺陷。tray_windows.go 与 tray_asset_windows_test.go 均为 `windows`；tray_linux.go 与 tray_asset_linux_test.go 均为 `linux && cgo`；tray_linux_cgo0.go `linux && !cgo`、tray_other.go `!windows && !linux`；Windows 下 `go test -list TestTray` 仅见 TestTrayIconAsset。注：本机无 Linux 实际 cgo 环境，交叉 vet 到 cgo 头文件缺失为止，测试编译性以 tag 隔离语义确证 |
| trayPNG CRC 修复 | 无缺陷。独立临时程序解码验证（PASS），见上 |
| windowClosedLocked 三判据单源 | 无缺陷。主判据（曾开窗+空快照+已过开窗点+10s 裕量，probe 侧入账）/ 时钟连续失败≥3（带"开放时间已过"）/ 幽灵窗口 EmptyProbeRuns≥3（同样带 cutoff）；StateForAccount 与 WindowClosed() 均调用单源；10s 裕量与 EmptyProbeRuns 入账对称。`TestWindowClosed|TestSubmitSuspendedWhenOpenTimeCleared` 等回归全绿 |
| spawnChain 身份防线族（同 client 指针比对）| 无缺陷。成功/失效/风控/窗口关闭/实时复核三路（ErrUnauthorized、确证满员、普通失败）共 6 个 `sameClientFor` 分支全覆盖；链顶 ClientFor 双判 + chars 活跃标记去重；`TestDeletedAccountRebuiltSameNameChain*` 回归绿。`clientIdentity` 用 `reflect.ValueOf(c).Pointer()` 取指针身份，nil 与非指针返回 0 |
| maybeRelogin 入口存在性复核 / MarkTokenValid / 锁序 | 无缺陷。入口锁内 `ClientFor(acct)` 复核后才会写 tokenValid/reloginFail；MarkTokenValid 与决策侧同持 reloginMu→s.mu 锁序串行 |
| accounts 登录频率闸门（gateWait/gateTryAcquire/Rlogin 收口）| 无缺陷。`Manager.Relogin` 调 gateWait（阻塞排队），`LoginByPassword` 前 gateTryAcquire（非阻塞拒绝）共享同一 gateMu/gateUsed；api 夹具调用 ResetGateForTest；`TestLoginByPasswordRejectsWhenGateBudgetExhausted` 等绿 |
| httpDo 仅 dial/write 重试 | 无缺陷。`isConnErrRetryable` 仅 `nerr.Op=="dial"|"write"`，read/业务/取消上抛；`bytes.Reader` 经标准库自动设置 GetBody，Clone 重放完整 body（临时程序实测 13 字节完整到达），注释与实现自洽 |
| 零吞错落库点（契约 17）| 无缺陷。全仓库 `SaveSuccess/SaveRefused/DeleteSuccess/AppendLog/DeleteRefused/DeleteRefusedClass` 调用点全部 `if err != nil { log.Printf }`，grep 无 `_ =` 吞错 |
| 契约 20 轮次前缀标签扫描 | 无残留。`第 N 轮` / `X-XX（第 N 轮）` / `R[0-9]-` 前缀在 backend 全部 .go 内零命中（含测试文件） |
| main.go runTray 时序 / openBrowser 双路径 | 无缺陷。`trayStartOnce` 在 onReady 菜单装配完成后 close 放行；sched.Start() 在 runTray 之前已执行（提交/探测不受托盘阻塞）；openBrowser 桌面托盘场景经托盘菜单触发、无托盘场景自动打开一次，与注释一致 |
| 管理员双条件鉴权（B43-04）| 无缺陷。handleLogin 先 `Account==adminName && ConstantTimeCompare(Password,AdminToken)` 才签发管理员会话；撞名学生走教务登录；时延拉平保留 |
| 删账号 memory-first 与四步序 | 无缺陷。Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount，Remove 前置后落库前复核立即失败；PurgeAccount 全量清理含 done/full/rateLimited/inflight/refused/acctData/openTimeDetected/tokenValid/relogin 族 |
| SetTargetsForAccount 只清 refused、Restore 顺序 | 无缺陷。手动重设清 refused（内存+库行），保留 done/full/rateLimited/inflight；RestoreDone → RestoreTargets(不清 refused) → LoadRefused + RestoreRefused 顺序正确 |
| windowClosed error / isRateLimitError 文本匹配集合 | 无缺陷。与注释契约一致（关闭/未开启/报名时间/已结束 / 频繁/429/稍后重试）。注意：主判据挂在 WindowClosed 而非依赖错误文案（已核） |
| ElectivesSnapshotFor 回退链 | 无缺陷。目标账号帧过期必返 (nil,false) 不回退全局帧；无目标纯浏览从未有专属帧才回退全局帧；判据用 `len(acctTargets[acct])>0` |
| ?account= 全路径 accountExists 校验 | 无缺陷。课程读/目标写/手动报名/手动退选/状态读五条路径均走凭据表判据；无透传时对齐"首个有目标账号" |

## 契约抽查表（本轮抽核 8 项）

| 契约 | 位置 | 结论 |
|---|---|---|
| 开放时间唯一事实源 = beginTimes 自动识别（识别槽保留、过期仅展示层）| scheduler.go:413-449, 816-880 | 通过 |
| WindowClosed 三判据单源 + 10s 裕量入账/判读对称 | scheduler.go:902-934, 1138-1158 | 通过 |
| 删除账号 memory-first 四步序 | handler.go:998-1020 | 通过 |
| 落库失败零吞错 | 全仓库 grep | 通过 |
| httpDo 仅 dial/write 重试（连接活性自愈）| client.go:466-501 | 通过 |
| 身份防线族 spawnChain 全分支 6 处 sameClientFor | scheduler.go:1484-1632 | 通过 |
| 登录频率闸门全入口收口 | manager.go:46-66, 218-257 | 通过 |
| 严禁轮次前缀标签 | 全仓库 grep | 通过（零残留）|

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go build ./...`（backend）| 通过，exit 0 |
| `go vet ./...`（backend）| 通过，exit 0；GOOS=linux CGO=1=0 受限到 cgo 头文件缺失（本机无 GTK 链）|
| `gofmt -l .`（backend）| 零文件 |
| `go test ./...`（默认 CGO）| 全包 ok |
| `go test -race ./internal/scheduler/`（关键回归组）| ok |
| `CGO_ENABLED=1 go test ./...`（Windows+ddddocr 同口径）| 全包 ok |
| `CGO_ENABLED=0 go build ./...`（Linux CI 同口径）| exit 0 |
| `go test -run 'TestTrayIconAsset'`（Windows 回归钉）| PASS（但其盲区掩盖 M79-01，见上）|

## 结论

R78 修复链的托盘资产回归钉与 DIB 头修正整体方向正确、构建与全量测试全绿，但 **Windows 托盘图标的 ICONDIRENTRY 字段存在真实缺陷（M79-01）**，本审查用独立程序调用 `user32.LoadImageW` 实测：当前字节产出失败、字段修正后成功，且系统 OneDrive.ico 逐字节解析佐证字段语义。该缺陷自 R71 托盘落地起一直存在，R78 回归钉因不解析 entry 字段而假绿（O79-01 联动），属"修复不完全 + 测试盲区"双遗留。其余生产契约（三判据单源、身份防线族、登录闸门、httpDo 自愈、零吞错、删账号 memory-first、管理员双条件）抽查全部通过，契约 20 标签扫描清零，构建验证全绿。

（脚注：libgit2 工作树在 R79 开始前即显示 `web/src/routes/Admin.tsx`/`Select.tsx` 两个 modified 文件，非本轮审查产生。）