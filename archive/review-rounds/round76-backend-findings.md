# R76 后端只读审查报告

基线：commit 7b1f441（R75 收尾，HEAD）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto，分支 master。只读铁律全程遵守（Read/Grep/Glob + Bash 只读命令 git log/show、go build/test/vet 全绿实证）；零 Edit/Write。

## 概述

**零 CRITICAL、零 MAJOR、1 个 MINOR（trayPNG 注释与字节不符）、3 个 OBSERVE。** R75 三项修复（commit 6f221c3）逐项核证全部正确：「管理员热改」死引用按 R75 既定目标收净、且方向判断可停（handler.go:168 激活码开关是真实运行时热改路径，保留正确）。开放时间唯一事实源契约、Linux 托盘、zhidao 冷启动 flake、生产逻辑契约全表复核全部成立。新增发现集中在托盘图标字节与注释、行号引用漂移两个小族。

## 重点复核结论（R75 变更后回归）

### 1. R75 修复正确性（commit 6f221c3 三项）——全部通过

| 位置 | 修复后表述 | 核证结果 |
|------|-----------|----------|
| scheduler.go:99 | 「平台下发新一轮 beginTimes，临门判断自然重新收紧」 | 准确。真实触发源=ProbeForAccount:849 / probe:1103 识别槽被平台新 beginTimes 批次覆盖（`s.openTimeDetected["*"] = data.BeginTimes[0]`），与「开放时间唯一事实源 = 平台 beginTimes」定向吻合；不再误指不存在的管理员热改路径。 |
| scheduler.go:928 | 「窗口若真开、平台下发新一轮 beginTimes，success 探测数据后势必推开始 open 实况」 | 准确。与 windowClosedLocked 判据 3（幽灵窗口 EmptyProbeRuns）的自愈语义一致——非空快照探测推进 opened、EmptyProbeRuns 归零。 |
| scheduler_test.go:1442 | 「平台下发新一轮 beginTimes 的防守场景」 | 准确。与 TestProbeIntervalWindowClosed 第二段 fixtures（future open + WindowOpened=true）语义对位。 |

**「管理员热改」残留判定可与 R75 共同收尾**：全仓 grep `管理员热改` / `管理员配置 / 管理员 PUT / reparse / OpenTimeParsed` 仅剩两族真实路径——① handler.go:168 `activationEnabled`「管理员热改立即生效」：激活码开关确经 runtime.Store 热改（handler.go:766-793 PUT），这是真实存在的运行时热改路径，表述正确，**保留**；② scheduler.go:980/:1138 与 scheduler_test.go:1421 三处「热改」泛指（「热改亚毫秒窗口」「开放时间热改到未来」）描述的是「open 值被新批次动态覆盖」的通用语义、不点名管理员，R75 已按 MINOR-75-01 建议一并收敛或从轻保留，方向未误导，**维持保留**。结论：全仓不再有指向「管理员可改开放时间」的单条死引用，R75 目标达成，无需再扫。

### 2. 开放时间唯一事实源契约深化——成立

- `reparse / OpenTimeParsed` 全仓零残留；`XUANKE_OPEN_TIME` 仅测试断言其不被消费（env_test.go:36）与 config.go:56 解释性注释，生产代码零消费、Config 无 OpenTime 字段。
- main.go:113 `scheduler.New(accts, st, time.Time{}, 300ms)` 传零值；openTimeForLocked:431-442 优先级=账号槽→全校槽→回退遗留 openTime（恒零值）；StateForAccount:714-720 识别过期只影响展示、槽保留不删。
- scheduler.go:46-50/:79-80/:99/:928/:1000-1008/:416-442 全部 OpenTime 相关注释与实现逐字一致；tick 零值守卫（:1006 `open.IsZero() && !opened`）让位 WindowOpened 语义；「关闭≠时间消失」由 ProbeForAccount:847-851 / probe:1101-1105 只覆盖非空 beginTimes、open_retain_test 钉死。

### 3. Linux 桌面托盘——build tag 穷举成立 + trayPNG 字节协议合法（发现一个小注释瑕疵，见 MINOR-76-01）

- **build tag 穷举**：tray_linux.go(`linux && cgo`) / tray_linux_cgo0.go(`linux && !cgo`) / tray_windows.go(`windows`) / tray_other.go(`!windows && !linux`) 四方互斥无空洞，每平台恰好一个实现；`go build ./...`（本机 Windows CGO=1）全绿实证。
- **showZenityOrPrint 双降级链**（tray_linux.go:95-103）：`exec.LookPath("zenity")` → `cmd.Run()` 成功即返回 → 无 zenity / 对话框失败降级 `fmt.Printf` 控制台，绝不因对话框阻塞托盘；os/exec 与 fmt 均真实使用无 import 悬空。
- **trayPNG 合法性（实测）**：字节序列签名/IHDR(1x1 RGBA8)/IDAT/IEND 四段 CRC 全部正确、zlib 解压出 5 字节（1 filter + 1x1x4 RGBA）。Linux setIcon 经 temp 文件 + `app_indicator_set_icon_full` 由 AppIndicator 依内容解码，PNG 合法即被接受——**协议层无缺陷**。
- **trayIcon（Windows ICO）合法性（实测）**：ICONDIR/ICONDIRENTRY/BITMAPINFOHEADER 字段逐项正确（`bytesInRes=4286==len(ico)`、`DIB height=64==双高`、中心 4x4 白像素落位）、AND mask 长度 128 正确；systray 经 `LoadImageW(IMAGE_ICON|LR_LOADFROMFILE|LR_DEFAULTSIZE)` + `DrawIconEx` 绘制，合成结构合法、可被正确加载。

### 4. zhidao 冷启动 flake 残余——五处裸 mock 靶向连跑无样本

- 五处无 readyProbe 的裸 `httptest.NewServer` 宿主：client_test.go:172（TestNoAutoRelogin）/ :287（TestReloginIfNeeded）/ :419（TestLoginRetriesTransientInitError）/ :457（TestExitClass）/ :486（TestExitClassFailsOnCodeNotZero）。均由 TestMain 包级 socketPreheat（:39-42，预创建-关闭一个 127.0.0.1 套接字排空 Time_WAIT 冷启动窗口）兜底。
- 本轮实证：`go test ./internal/zhidao/` 全量 1.848s 绿；五处靶向全绿。**当前不触发**。

### 5. 生产逻辑关键契约复核（历史重点全表）——全部成立

| 契约 | 实现 | 复核结果 |
|------|------|----------|
| 窗口关闭三判据单源 | scheduler.go:913-934 windowClosedLocked | 成立。三判据共用单 `open` 快照（:919）；StateForAccount:707 与 WindowClosed:905 同真相；10s 裕量判据侧（:1141）+入账侧（:1150）对称；TestWindowClosedState / TestWindowClosedTransitionStateGrace / TestStateForAccountMirrorsWindowClosed 全绿。 |
| 删号 memory-first | handler.go:1004-1018 | 成立。Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount 顺序与契约一致；PurgeAccount:499-523 含 openTimeDetected 槽清理（:510）、Courses 行剔除、inflight 清空；DeleteAccount 失败半删态由重启 Restore 自愈。 |
| sameClientFor 六分支 | scheduler.go:1484(失效)/1516(成功)/1546(风控)/1566(窗口关闭)/1595(实时复核入口)/1630(确证满员) | 成立。六分支全含指针身份比对；:1595 入口复核覆盖其后全部写回点（:1603 maybeRelogin+setState+AppendLog、:1622 doneHas、:1630 markFull、:1655 setState+AppendLog）；maybeRelogin 决策侧（:1203）与写回侧（:1249）双 ClientFor 复核；MarkDone/RemoveDone 手动路径复核（:1922/:1990）。 |
| IsReadErr 四形态 | client.go:519-548 | 成立。FIN（errors.Is io.EOF）/ 短读（ErrUnexpectedEOF）/ 超时双文案（awaiting headers + reading body）/ RST（OpError.Op=="read"）四形态全覆盖；与 isConnErrRetryable（:492-501 只认 dial/write）互斥；TestIsReadErrCoversAllForms 钉死。 |
| doLogin 全局闸门 | manager.go:49-64 gateWait + :223-235 gateTryAcquire | 成立。两门共享 gateMu/gateUsed 计数（同一分钟窗口）；LoginByPassword:243-246 非阻塞准入绝不挂起用户响应；gateWait 只被 Relogin 使用；管理员换绑同走收口；ResetGateForTest 仅测试夹具。 |
| 零吞错落库点 | 全仓抽查 | 成立。全部非测试文件 `_ =` 经逐个核验均为非落库点（browser Start 错误、io.Copy/ProbeForAccount 忽略返回值、messageBox 返回码）；scheduler.go 全部 store 调用点带 `if err != nil { log.Printf }` 且全部在 `s.store != nil` 守卫内。 |
| httpDo 仅 dial-write 重试 | client.go:474-501 | 成立。isConnErrRetryable 只认 dial/write；read 错误原样上抛（防双报）；业务/取消错误不重试；recognizeViaVision 同样走 httpDo 收口（captcha.go:162）。 |
| config 双默认值 | config.go + env_test.go | 一致。默认识别引擎 ddddocr（CaptchaEngineDefault:106-111 / .env 模板:151 / env_test:71-84 三处同向）；激活码默认 off（:43-45 / 模板:154 / env_test:46-66 / main.go:33-37 四处同向）。 |
| tick 零值守卫让位 WindowOpened | scheduler.go:1006 | 成立。`open.IsZero() && !opened` 才挂起；TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime（WindowOpened=true + 识别槽空 + tick 放行黄金期）绿；TestSubmitSuspendedWhenOpenTimeCleared（Fixture WindowOpened=默认 false）绿。 |
| probe 空快照不删槽 / PurgeAccount 删槽 | scheduler.go:1101-1105 / :510 | 成立。「关闭≠时间消失」契约：只覆盖非空 beginTimes、空快照保留槽；删账号删槽且有测试钉死（TestPurgeAccountClearsOpenTime）。 |

## 发现

### MINOR

**MINOR-76-01（tray_linux.go:66-70 trayPNG 注释宣称「纯黑 + 中心 4x4 白块」，字节实为 1x1 全透明像素——像素内容与注释不符，Linux 托盘在深色面板上图标可能不可见）**

- 文件：`backend/tray_linux.go:66-70`
- 一句话：注释「16x16 RGBA 手写最小 PNG（IHDR+IDAT+IEND），黑色背景 + 中心 4x4 白块」与实际字节不符——实测 PNG 为 1x1 像素、RGBA = `[0,0,0,0]` 全透明黑、无任何可见像素（zlib 解压 IDAT 得 `00 00000000`，filter=0、A=0）。
- 触发场景：Linux 桌面（systray 经 temp 文件 + AppIndicator 解码，协议层接受合法 PNG，功能不崩）；但 AppIndicator 渲染的托盘图标是 1x1 全透明像素——深色 GNOME/KDE 面板上不可见（或显示为纯占位图标）。这是「注释与实现不符」的非功能缺陷；个别发行版若 AppIndicator 对全透明图标有主题回退不同行为，视觉呈现各异但不会崩溃。
- 修复建议：两选一——① 改注释为「内置极简 1x1 全透明 PNG 占位（AppIndicator 会自适应/主题回退）」，承认现有字节；② 把字节升级为 16x16 黑底+中心白点（参照 tray_windows.go 的图形语义，约 13 字节 IDAT 改动），使托盘图标真实可见。倾向 ②（macOS/Windows 都有可见图标，Linux 全透明体验不一致）。

### OBSERVE

**OBSERVE-76-01（scheduler_test.go 残留 5 处行号引用：:1262/2848/3501/3503 指向 tick 守卫/spawnChain 分支行，当前与实际实现已漂移——R74 修过一次同族，此次为残余漏网）**

- 文件：`backend/internal/scheduler/scheduler_test.go:1262(:2848/:3501/:3503)` + `backend/internal/api/handler_test.go:1500`（:137 行）
- 一句话：:1262「tick 守卫 702 行」实测 tick 守卫在 :1006/:1009（702 是 SchedulerState 注释区行）；:2848「1338 行有 doneHas」实测 doneHas 复核在 :1622/:1640（1338 是 submitAll 函数定义区）；:3501「1479/1490 行」实测 ErrUnauthorized 分支在 :1477/:1484；:3503「1502 行统一清位」实测在 :1508。四个引用全部不在所指位置。handler_test.go:1500「代码 137 行保留」实测 handler.go:137 是登录错误文案行，指位正确（无漂移）。
- 触发场景：维护者按这些行号跳转代码时落到错误位置（提交守卫/身份复核正确路径被误定位），每次新增/删除会继续累积漂移；R74 已把「1575 行」改语义指位，「行号引用族」整风未含这几处。
- 修复建议：与 R74 同款戰略——:1262 改「tick 提交守卫（零值守卫/未开点守卫）」语义指位；:2848 改「紧邻的『未现满员』分支（下方 doneHas 复核『绝不覆盖胜利状态』）」；:3501/:3503 改「失效分支（errors.Is ErrUnauthorized 段）」「SelectClass 返回后的统一清位」。

**OBSERVE-76-02（CI 矩阵仍无 Linux CGO=1 构建检查——R73/R74/R75 连续三轮建议仍未落实）**

- 文件：`.github/workflows/ci.yml`、`backend/tray_linux.go`
- 一句话：ci.yml 仅 Linux/macOS CGO=0 与 Windows CGO=1 构建；tray_linux.go（linux&&cgo）依赖 systray 的 GTK3 头文件，本机无 Linux 交叉工具链，其编译正确性持续零自动化防线（R72 前 showZenityOrPrint 事件同型）。
- 触发场景：未来对 tray_linux.go 的改动若引入未定义符号/import 错误，现有 CI 矩阵恰好全部绕过。
- 修复建议：给 ci.yml 加 Linux CGO=1 构建 job（apt-get libgtk-3-dev + go build；不便装 GTK 时至少建 dummy 校验）。可留待后续的可选小项。

**OBSERVE-76-03（zhidao 五处无 readyProbe 裸 mock 宿主延续观察——靶向连跑无样本，包序 + TestMain 兜底已闭环）**

- 文件：`backend/internal/zhidao/client_test.go:172/:287/:419/:457/:486`
- 一句话：zhidao 全量 + 五处靶向 `-count=1` 连跑均无 flake 样本，TestMain（:39-42）包级 socketPreheat 兜底生效；TestLoginNetworkErrorAbortsImmediately 已补 readyProbe 后 zhidao 无边角裸宿主。
- 触发场景：仅在 Windows 回环冷启动窗口恰好叠加到裸 mock 首请求时可能复现（多轮未现）。
- 修复建议：暂不处理，归入 flake 演进观察。若 CI 出现「首轮 zhidao connectex」，优先给这五处补 readyProbe。

## 可疑待核

1. **handler_test.go:644/:694 注释「下游热下发未跳过：再次热改（仍失败）后登录…」**——「热改」指 Vision 配置热更新（runtime.Store PUT 路径，真实存在），非开放时间热改，方向正确、无残留嫌疑。**已排除**。
2. **windowClosedLocked 判据 2/3 的 `!open.IsZero()` 判定**：识别槽保留过期值（已过去）时 `IsZero` 恒 false，判据按「已过点」语义成立（`nowAlignedLocked().After(open)`）；识别槽清空（删除账号/全校从未识别）时 open=0，判据不触发、由主判据/EmptyProbeRuns 另行兜底。边界无缺陷。**已核无异常**。
3. **ConsumeActivationCode 事务内先 UPDATE 再查 activations**：并发同一激活码 + 同一账号双激活请求，UPDATE 原子防超卖、activations 主键约束防重复插（第二次事务 `INSERT OR IGNORE` 前先 SELECT count 拦截），双保险成立。**已核无异常**。

## 已核无缺陷清单

- **R75 修复逐项核证通过**：scheduler.go:99/:928 + scheduler_test.go:1442 三处「管理员热改开放时间」→「平台下发新一轮 beginTimes」表述与实现（识别槽覆盖路径 ProbeForAccount:849/probe:1103）逐字一致；全仓「管理员热改」仅剩 handler.go:168（激活码开关真实热改路径，方向正确应保留）与三处「热改」泛指（不点名管理员，方向未误导）。
- **开放时间唯一事实源契约**：reparse/OpenTimeParsed 零残留；XUANKE_OPEN_TIME 无生产消费；Config 无 OpenTime 字段；main.go 传零值；env_test 守护在位；scheduler.go 全部 OpenTime 注释与实现一致。
- **Linux 托盘**：build tag 四方穷举互斥无空洞；showZenityOrPrint 双降级链完整（exec.LookPath→Run→fmt.Printf）；trayPNG 字节协议合法（CRC/IHDR/IDAT/IEND 全正确、1x1 全透明像素——详见 MINOR-76-01）；trayIcon ICO 结构合法（header/DIB 双高/AND mask 全对，systray LoadImageW 可载）。
- **zhidao 冷启动 flake**：五处裸 mock 靶向 + 全量均绿，无样本。
- **生产逻辑契约全表**：窗口关闭三判据单源（10s 裕量两侧对称）/ 删号 memory-first（含识别槽清理）/ sameClientFor 六分支（含实时复核入口前置统一覆盖）/ IsReadErr 四形态 / doLogin 全局闸门（双门共享计数）/ 零吞错落库点（`_ =` 逐个核验非落库点）/ httpDo 仅 dial-write 重试（Vision 识别同收口）/ config 双默认值 / tick 零值守卫让位 WindowOpened / probe 空快照不删槽——逐条成立。
- **测试与构建实证**：`go vet ./...` 全绿；`go build ./...` 全绿；zhidao 1.848s + scheduler 13.396s + api 15.557s + accounts/config/db/store/session/secure/runtime 八包全量绿；回归陷阱双测试（TestWindowOpenSubmitsWithoutProbeReset:1266 / TestAdminStatsWindowOpenedUsesScheduler:1010）在位。
- **CSRF 核查**：POST /api/logout 未强制 JSON，但跨站表单无法携带 Authorization 头（前端用 Bearer），实际无法通过 requireAuth——非缺陷。DELETE /api/admin/* 去掉 requireJSONBody 是「标准 REST DELETE 无 body」的刻意放宽，由 requireAdminSession 鉴权兜底——可接受设计。

## 结论

零 CRITICAL、零 MAJOR。1 个 MINOR（tray_linux.go:66-70 trayPNG 注释「纯黑+中心白块」与字节实为 1x1 全透明不符）+ 3 个 OBSERVE（scheduler_test.go 五处行号引用漂移 / CI 无 Linux CGO=1 检查 / 五处裸 mock 延续观察）。R75 修复逐项核证正确、「管理员热改」收净目标达成、开放时间唯一事实源契约成立、托盘与 zhidao 回归全绿。最值得主控处理的是 MINOR-76-01（托盘图标与注释不符，升字节即可）与 OBSERVE-76-01（行号引用族 R74 整风后残余漏网）。