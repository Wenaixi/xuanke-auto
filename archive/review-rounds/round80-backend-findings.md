# R80 后端只读审查报告

审查对象：xuanke-auto HEAD commit `795bb4f`（R79 收官），重点复核 R79 变更回归（Windows ICO entry 字段修复 / 回归钉补齐 / build tag 四文件隔离 / trayPNG+ICO 双回归钉），并做契约 20 全仓扫描、生产逻辑契约抽核 6-8 项与全量构建验证。

审查方式：全程只读。对 ICO 字段用独立 `%TEMP%\r80_icocheck` 目录临时 Go 程序复刻字节并调 `CreateIconFromResourceEx` + `GetIconInfo` 实测（用后即删，未在仓库留任何文件）；对调度器关键路径跑受限/全量 `go test`（含 `-race`）。仓库工作树零改动（`git status` 清洁）。临时实测程序已删除，`git status` 复核无残留。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

### M80-01：api 包两个测试出现偶发抖动失败（非确定性），与 R79 日志中 `-count=1 -p 1` 全绿基线不一致

**文件 + 行号：** `backend/internal/api/handler_test.go:357`（`TestAccountOverrideRequiresAdminSession`）、`:1250`（`TestAdminElectiveSelectUnknownAccountRejects`）

**实测记录（`go test -race -count=1 ./internal/api/`）：**

| 尝试 | 结果 |
|---|---|
| 全量含 api（`go test ./...` 首次） | `FAIL  internal/api  59.614s`，失败测试名未回传（退出行只给包级 FAIL） |
| 复跑全量 ×2 | 全绿 |
| api 包内 `-count=1` ×4 | 全绿 |
| `-race -count=1 -p 1` 三包 | 全绿 |
| `-race -count=1 ./internal/api/` #1 | `--- FAIL: TestAccountOverrideRequiresAdminSession (45.74s)` |
| `-race -count=1 ./internal/api/` #2 | 全绿 |
| `-race -count=1 ./internal/api/` #3 | `--- FAIL: TestAdminElectiveSelectUnknownAccountRejects (2.23s)` |
| `-race -count=1 ./internal/api/` #4 | 全绿 |
| 定向 `-run` 失败测试单跑/组跑 ×5（含 race） | 全绿 |

**触发场景推演：** 两失败测试的具体方向不同：`TestAccountOverrideRequiresAdminSession`（45s 超长失败）依赖 `authenticateDirect → LoginByPassword`，后者经 `gateTryAcquire` 全局频率闸门（每分钟 2 次预算）——测试多次注册/登录同一分钟窗口内并发消费配额，窗口翻转边界（50-60s 之间 `time.Since(gateWindow) >= time.Minute` 重置）与同包并行测试共用 `gateMu` 时存在"整包跑 → 其它测试先消耗预算 → 本测试恰好卡在被重置的边界上被 `gateTryAcquire` 拒绝"的可能性（也有 `time.Duration` 恰好踩在单分钟窗口边缘的口径）。`TestAdminElectiveSelectUnknownAccountRejects`（2s 短失败）走 `doJSONAuth→handleElectiveSelect→CheckClassSelectable→ElectivesSnapshotFor` 快照链路与 `Sched.ProbeForAccount` 网络探测竞态（首帧 `snapshotTTL=40s` 语义下，全局帧新鲜度与探测单飞/节流闸门在并发时偶发错位）。两测试单跑全绿、全量偶发红——**高频并发下测试夹具时序敏感**，非产品逻辑确定性缺陷；R79 收尾日志以 `-p 1` 串行跑全绿也佐证此判断。

**修复建议：** 不定位产品 bug（产品层无关）：① 测试侧在 `TestAccountOverrideRequiresAdminSession` 内 `ResetGateForTest()` 已调用，但同包**其它测试**不受控地消耗全局闸门预算——建议把这些敏感测试串行约束（`t.Setenv` 无效果时用包内 `-p 1` 或独立一个 test binary），或将 `gateTryAcquire` 在测试下用注入接口隔离；② 对快照/探测竞态测试，用 `fakeAccts` 且不真正触达网络（本测试夹具已用假客户端，失败根因优先归因探测节流/时间窗）；③ 每轮 CI 基线明确以 `go test -race -count=1 -p 1 ./...` 为准（R79 日志口径），串行跑可消除上述时序抖动。严重度：MINOR（偶发抖动、单跑全绿、非产品逻辑缺陷，但 R79 声称"全 10 包全绿"与实测偶发红不符，基线口径需固化）。

### M80-02：R79 回归钉对 dwBytesInRes 的断言存在"与实现同源复算"半失验力——offset 断言硬编码、bytesInRes 断言复算同式

**文件 + 行号：** `backend/tray_asset_windows_test.go:24-33`（对照 `tray_windows.go:96-97`）

**缺陷推演：** 回归钉两处断言：
- `dwImageOffset`（`ico[18:22]`）断硬编码 `off != 22`——**语义方向正确、无同源复制**，未来任何人把 offset 改成别的值立刻红；
- `dwBytesInRes`（`ico[14:18]`）断 `want := uint32(len(ico)) - 22`，与实现 `binary.LittleEndian.PutUint32(ico[14:18], uint32(len(ico)-headerSize))` 是**同式推导**——若将来有人把实现误改为 `len(ico)-16`（headerSize 常量定义改值），测试同样跟着 `len(ico)-22` 复算仍绿；更关键的"字段值互置"反例：若实现与测试都改读同一常量，测试仍绿。R79 教训（"回归钉必须按语义各断言一个具体值 4264/22 而非只查非零"）在 offset 上做到了硬编码，在 bytesInRes 上退回了复算。

**修复建议（测试层，产品层无需改）：** 把 dwBytesInRes 断言从复算改为语义无关的**硬编码常数**（如 `want := uint32(4264)`，或按格式语义写成 `dibHeaderSize + pixelBytes + andMaskBytes = 40+4096+128 = 4264` 的**三加数独立推导**，而非 `len(ico)-22` 的整段复算）——三加数式与"总长减头部"在根因上不同源，字段值写错（如把像素算成 4095、或把 offset 值 22 打进 bytesInRes）仍能红。同时给实现加注释注明两字段的**独立硬锚值**（4264 / 22），作为未来维护对照。严重度：MINOR（当前回归钉已拦下"值互置"——`064/M64` 双字段判定下 bytesInRes 若被错填 22 仍会因与 4264 不符红；但"同式复算"削弱了测试对实现算术散失的失验力，属可改进非缺陷）。

## OBSERVE

### O80-01：`auth handleAdminDeleteAccount` 的 `IsAdminAccountName` 防护依赖"管理员名与账号名不可同串"的全局不变量，未在删除路径内做口令兜底

**文件 + 行号：** `backend/internal/api/handler.go:992`

**推演：** `handleAdminDeleteAccount` 以 `d.IsAdminAccountName(acct)` 拒绝删除管理员账号（删除保护）。`IsAdminAccountName` 只看账号名字符串等于 `AdminNameValue()`（默认 `admin`）。若部署时 `XUANKE_ADMIN_NAME` 被配置成与某真实学生学号相同的字符串，该学生账号即被删除保护永久覆盖——**B43-04 已为登录路径处理了同款"撞名"**（管理员分支改"账号+口令"双条件，撞名学生可正常教务登录），但删除/保护路径仍退回单判据。实际影响有限（管理员名需显式配成学生账号名才触发，非默认态），归 OBSERVE。

**修复建议：** 与 B43-04 对齐，删除保护改"管理员名 + 是管理员会话"（调用方已是 `requireAdminSession`，删除请求来自管理员会话本身，若管理员要删"与自己同名但并非自己"……语义上自洽性待业务定夺）；至少文档注明 `XUANKE_ADMIN_NAME` 不得与任何学生账号名同串。

### O80-02：`probe()` 内 `for _, a := range s.AccountsWithTargets()` 每账号 goroutine 的 `_, _ = s.ProbeForAccount(acct)` 吞掉探测错误

**文件 + 行号：** `backend/internal/scheduler/scheduler.go:1070`

**推演：** 每账号探测 goroutine 结果（含网络错误/被节流拒绝）整体丢弃。错误数据其实已由 `ProbeForAccount` 内部记日志（探测失败路径有 `log.Printf`），此处吞错不违反"零吞错落库"规范（不涉及落库），但失败的探测结果不被上层感知（如 `probe()` 主探测 `FindElectives` 成功、per-account 探测全失败时，`lastProbe` 仍推进）；属设计取舍（探测失败主路径已降频兜底），归 OBSERVE。

### O80-03：`backend/internal/config/config.go:160` 首次生成 `data/.env` 用 `_ = os.WriteFile` 静默吞错

**文件 + 行号：** `backend/internal/config/config.go:160`（`ensureEnvFile` 内）

**推演：** 首次运行（`XUANKE_ADMIN_TOKEN` 环境变量为空且 `data/.env` 不存在）时，`MkdirAll` 与 `WriteFile` 的返回值均被丢弃；若程序所在目录不可写（只读部署/权限受限），`.env` 写盘失败后配置不落盘，但本次进程 `loadDotEnv` 读不到仍以内存默认值运行（`XUANKE_ADMIN_TOKEN` 为空 → 后续 `AdminToken` 校验拒绝启动的路径兜底）。此写法符合 CLI 工具首次生成惯例，但属"落库失败必须记日志绝不静默吞错"契约族的一处远征残留——建议写失败时 `log.Printf` 提示"无法写入 data/.env，管理员口令仅本次进程有效"。归 OBSERVE。

## 可疑待核

- **api 包偶发抖动失败（M80-01）根因归属待证**：是"测试夹具时序敏感"还是"全局频率闸门/快照 TTL 在并发下有极小概率误拒绝"，需在 CI 稳定复现后由主控用 `-run` 定向 + `-count` 高轮次压测归因。两失败测试涉及路径互不相同（闸门 vs 快照），更倾向夹具层面（单跑久验全绿、`-p 1` 全绿、race 与无 race 均出现）。

## 已核无缺陷清单

### R79 变更回归（重点复核项）

| 项 | 结论 |
|---|---|
| `tray_windows.go:96-97` ICO entry 字段（R79 修复） | **实测通过**。独立程序复刻 `trayIcon()` 逐字段验证：`len=4286`，`dwBytesInRes=4264`（= `len-22`）、`dwImageOffset=22`（= `headerSize`），DIB 头 `biSize=40/w=32/h=64` 正确；`CreateIconFromResourceEx` + `GetIconInfo` 实测图标加载成功（HbmColor 位图非空），字段语义与 Microsoft ICO 规范逐字节一致。 |
| 回归钉语义方向与"无同源复制" | **offset 断言硬编码 22、方向正确**（M80-02 仅 bytesInRes 复算可改进，offset 无同源问题）；测试断言 Red/Green 对（先写反 4264 红、`[18:22]` 判 22 反例红，见 R79 TDD 记录）。 |
| build tag 四文件隔离 | **通过**。`GOOS=windows` / `GOOS=linux CGO=0` / `GOOS=darwin` 三平台 `go build ./...` 全绿——Windows 选 `tray_windows.go`（`//go:build windows`）、Linux 双文件按 `cgo/!cgo` 互斥选入、darwin 落 `tray_other.go`（`!windows && !linux`）。 |
| trayPNG/ICO 双回归钉全量走查 | **通过**。`TestTrayIconAsset`（含 entry 断言 + DIB 头 + 1024 像素全像素 BGRA 断言 + AND mask 区未越界）与 `TestTrayPNGAsset`（png.Decode 严格 CRC + 16x16 + 全像素语义）绿。ICO 像素区与 AND mask 字节数 `4096+128`，header `22+40`，总长 `4286` 与实现 `dataLen` 一致。Linux PNG 该测试以 `//go:build linux && cgo` 约束，Windows 下统计为 `[no tests]`——语义正确。 |

### 构建验证

| 命令 | 结果 |
|---|---|
| `go build ./...` | BUILD_OK |
| `go vet ./...` | VET_OK |
| `gofmt -l .` | 零输出 |
| `go test ./...`（缓存 + 复跑） | 全绿（偶发 api 抖动除外，见 M80-01） |
| `go test -race -p 1 -count=1 ./internal/accounts/ ./internal/api/ ./internal/session/` | 全绿 |
| `go test -race -count=1 -run 'TestDeletedAccountRebuiltSameName|TestStateForAccountMirrorsWindowClosed|TestWindowClosed|TestOpenTimeRetained' ./internal/scheduler/` | 全绿 |
| 三平台交叉编译（win/linux CGO=0/darwin） | 全绿 |

### 契约 20 全仓扫描

扫 `(第\s*\d+\s*轮|R\d{2,}|round\d+|B\d{2,}-\d+|F\d{2,}-\d+|MAJOR-\d+|MINOR-\d+)`，命中五点全部为"设计注释引用"而非轮次前缀标签：
- `tray_asset_{windows,linux}_test.go`：注释"R76/R77 两次手写字节失位""R79 曾误填"——历史回归锚点注释，属契约 20 许可的"为什么/契约"语义（叙述资产失位教训，非轮次前缀标签"X-XX 第 N 轮"形态）；
- `tray_windows.go:93` 行注释引 R79 LoadImageW 实测——同上锚点性质；
- `internal/session/store.go:117` 引 `docs/review-round13.md` 与 CLAUDE.md 落盘——文档路径引用非标签；
- `handler_test.go / store_test.go` 的 `XK-ABCD-EF12-3456` 为激活码样例文本，误中 `\d{4}-\d{4}` 假阳性。
**结论：零轮次前缀标签残留。（R79 剥离的 `R63 ` 前缀经复扫无再犯。）**

### 生产逻辑契约抽核 6-8 项

| 契约 | 结论 |
|---|---|
| `windowClosedLocked` 三判据单源 | 通过。`WindowClosed()` 与 `StateForAccount()` 共用 `windowClosedLocked()`（主判据 `state.WindowClosed` / 时钟连续失败 ≥3 + 开放时间已过 / 幽灵窗口 `EmptyProbeRuns≥3` + 开放时间非零），三判据共取单次 `open` 快照；`probe()` 主判据入账含 +10s 裕量（`now.After(open+10s)`），`EmptyProbeRuns` 计入账也带 +10s，与契约完全一致。对应测试 `TestWindowClosedState/TransitionStateGrace/ProbeDropsToFar/StateForAccountMirrorsWindowClosed` 全绿。 |
| spawnChain 身份防线族（B41-01 六分支） | 通过。成功/失效/风控/窗口关闭/实时复核确证满员/实时复核 ErrUnauthorized 六条 err 归并与状态写点全部 `sameClientFor`（指针身份比对，`clientIdentity` 反射取指针）；链顶与取 client 双处前置存在性复核；`maybeRelogin` 入口锁内 ClientFor 复核（B43-01）、写回侧复核（B21-03）齐备；实时复核 Int0215 回锁后三路身份复核对称。`TestDeletedAccountRebuiltSameNameChainDrops{Success,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}` 封锁。 |
| 登录全局频率闸门（B42-01） | 通过。`gateWait`（阻塞，Relogin 自动重登用）+ `gateTryAcquire`（非阻塞，`LoginByPassword` 手动登录 pre-Login 准入，quota 满即拒"登录尝试过于频繁"），共享 `gateMu/gateUsed/gateWindow` 同一计数；`LoginByPassword` 在 `m.ensure` 前调用 `gateTryAcquire`——管理员换绑同走收口；测试用 `ResetGateForTest` 重置。双入口无绕行。 |
| `httpDo` 仅重试 dial/write | 通过。`isConnErrRetryable` 仅 `nerr.Op=="dial"||"write"`；read 类（`IsReadErr` 覆盖 FIN/短读/超时/RST 四形态）恒不重试并区分文案；业务/取消原样上抛。`captcha.go` Vision 识别同款 `httpDo`。`bytes.Reader`/`GetBody` 重放语义正确（POST 不双报）。 |
| 零吞错落库 | 通过。全仓 `_ =` 落库点扫描仅两处：`scheduler.go:311` `_ = pw.Prewarm()`（连接预热非落库，失败下 tick 再试）与 `scheduler.go:1070` `_, _ = s.ProbeForAccount(acct)`（探测结果已内记日志，见 O80-02）；`config.go:160` 首次 .env 写盘静默（见 O80-03）。其余 SaveSuccess/AppendLog/DeleteRefused/UpdateIDToken 全部 `if err != nil { log.Printf }`。与 CLAUDE.md 契约 17"全仓库零吞错落库点"一致（上述三处均非"落库"）。 |
| 删号 memory-first | 通过。`handleAdminDeleteAccount` 顺序 `Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount`，与契约 4 完全一致；删除后 `DeleteAccount` 失败走"半删态"自愈注释准确；`TestAdminDeleteAccountMemoryFirst` 断言注册表先失效。 |

### 其他复核要点

| 项 | 结论 |
|---|---|
| `ElectivesSnapshotFor` 回退链（契约 8） | 通过。目标账号（`len(acctTargets[acct])>0`）专属帧缺失/过期必返回 `(nil,false)` 强制刷新，绝不回退全局帧；无目标且从未有专属帧回退全局；有专属帧但从"有目标"/浏览长帧过期"视角都返回 false（不误判）。 |
| `handleDenyMessage`/手动报名、退选 `?account=` 凭据表校验（契约 7 四路） | 通过。`/api/electives`、`/api/electives/select`、`/api/electives/select/exit`、`/api/state` 四路全覆盖 `accountExists`+凭据表。 |
| 识别槽保留（"关闭≠时间消失"） | 通过。`probe()` 空快照绝不 delete `openTimeDetected["*"]`；仅下发非空 `beginTimes` 覆盖；`openTimeForLocked` 单快照三判据复用。 |
| B43-04 登录双条件 | 通过。`handleLogin` 管理员分支 `Account==adminName && ConstantTimeCompare==1`，不匹配撞名学生走教务登录；错误分支固定 `loginTimingFlat` 时延。 |
| 目标保存链（发布元数据持久化 + refused 不清 done/full/rateLimited） | 通过。`SetTargetsForAccount` 仅清 refused（内存+库行 `DeleteRefused`），补全 publish_meta 后落库；`RestoreTargets` 保持 refused。 |
| 重登指数退避封顶 | 通过。`reloginBackoff` 封顶 `backoffMax=10m`、`maxReloginFail=5`；失败计数只在成功分支清零。 |

## 结论

R79 的 ICO entry 字段修复**经独立程序字节复刻 + `CreateIconFromResourceEx` 实测确认正确**（dwBytesInRes=4264/dwImageOffset=22 语义与 Microsoft ICO 规范逐字节吻合，图标可加载），回归钉方向正确但 bytesInRes 断言存在"同式复算"可改进点（M80-02）。build tag 四文件三平台交叉编译隔离、trayPNG/ICO 双回归钉、契约 20 零标签残留均通过。生产逻辑契约六项抽核（windowClosedLocked 三判据单源 / spawnChain 身份防线六分支 / 登录闸门双入口 / httpDo 仅 dial-write / 零吞错 / 删号 memory-first）全部通过。本轮无 CRITICAL 无 MAJOR；**主要观察为 api 包偶发抖动失败（M80-01），建议主控以 `-p 1` 串行基线固化 CI 口径并持续复现归因**。后端整体健康状况良好。