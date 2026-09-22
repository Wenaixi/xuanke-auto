# R75 后端只读审查报告

基线：commit 79cd469（R74 收尾）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto，分支 master，HEAD=79cd469。只读铁律全程遵守（Read/Grep/Glob/Bash 只读命令；后台跑过 `go vet ./...`、`go build ./...`、zhidao 全量两个包、五处裸 mock 靶向连跑、scheduler/api/accounts/config/db/store/session/secure/runtime 十包全量、scheduler 关键契约 -race 靶向）。

## 概述

**零 CRITICAL、零 MAJOR、1 个 MINOR、2 个 OBSERVE。** R74 两项修复逐项核证全部正确；「收净 reparse/OpenTimeParsed 死引用」目标达成（全仓零残留）；Linux 托盘 build tag 穷举互斥 + trayPNG 字节实测合法；zhidao 五处裸 mock 靶向无 flake；生产逻辑契约全表复核成立。新发现仅一处：R74 收净族漏网——scheduler.go 仍有 :99/:928 两处点名「管理员热改开放时间」残留注释（配置链路 B40-02 已整体移除后无任何此路径），以及测试 :1442 一处同款残留。

## 重点复核结论（R74 变更后回归）

### 1. R74 修复正确性（commit bb619a1 两项）——全部通过

**① scheduler.go 三处管理员配置残留注释改唯一事实源——语义准确、无死引用。**

| 位置 | 修复后表述 | 核证结果 |
|------|-----------|----------|
| scheduler.go:46-48 SchedulerState.OpenTime 注释 | 「来自该账号自己探测识别的 beginTimes（全校共享同一开窗时刻）；识别不到 = 未知」 | 准确。与 openTimeForLocked（:431-442，优先级 账号槽→全校槽→回退遗留 openTime 恒零值）逐字一致；已删「优先取管理员配置」表述。 |
| scheduler.go:50 OpenTimeKnown 注释 | 「平台 beginTimes 自动识别」 | 准确。与 StateForAccount:714-720 的识别值未来判定语义一致。 |
| scheduler.go:79-80 probeIntervalFor 注释 | 「全新部署未识别到开放时间 / 识别槽被显式清除」+「平台若开放新一轮选课（识别槽刷新到未来）」 | 准确。已删 reparse/OpenTimeParsed/管理员 PUT 死引用；与 :84-86 零值判远间隔实现一致。 |

**② scheduler_test.go「1575 行」改「实时复核入口」语义指位——准确。**

:3415-3421 注释改为「修复前（实时复核入口只有 ClientFor）…修复后（实时复核入口换 sameClientFor）」。实测 scheduler.go:1595 正是实时复核入口的第一处 sameClientFor 复核（在 cErr 分支 :1603、doneHas :1622、满员 :1630、普通失败 :1655 全部后续写回点之前），语义指位与实现逐行一致，不再绑定漂移行号。与 R74 报告 MINOR-74-01 闭环。

### 2. 开放时间唯一事实源契约深化——reparse/OpenTimeParsed 死引用零残留

- 全仓 grep `reparse` / `OpenTimeParsed`（含测试）：**零命中**。
- `XUANKE_OPEN_TIME` 仅存在于 env_test.go:36（测试断言其不被消费）与 config.go:56 注释（说明勿再读取），生产代码零消费。
- `Config` 无 OpenTime 字段（config.go:31-46 结构与实现核对），main.go:113 `scheduler.New(accts, st, time.Time{}, 300ms)` 传零值，识别槽是唯一事实源。
- handler_test.go 夹具与 main 同款传零值（注释「开放时间不做任何配置注入」），handleAdminConfig 的 settings 落库不含 open_time 键。
- **残留确认**：scheduler.go:99/:928 两处 + scheduler_test.go:1442 一处仍点名「管理员热改开放时间」，属 R74 收净族的 sibling 漏网（详见 MINOR-75-01）。

### 3. Linux 桌面托盘——build tag 穷举互斥 + trayPNG 合法性实测通过

- **build tag 穷举**：tray_linux.go(`linux && cgo`) / tray_linux_cgo0.go(`linux && !cgo`) / tray_windows.go(`windows`) / tray_other.go(`!windows && !linux`) 四方互斥无空洞，每平台恰好一个实现被选入、trayData 定义不重复冲突（go build 全绿实证）。
- **showZenityOrPrint 双降级链**（tray_linux.go:95-103）：`exec.LookPath("zenity")` → `cmd.Run()` 成功即返回 → 无 zenity / 对话框失败降级 `fmt.Printf` 控制台，绝不因对话框阻塞托盘；os/exec 与 fmt 均真实使用无 import 悬空。
- **trayPNG 字节合法性**：用 Go 标准库 `image/png.Decode` 实测该字节序列——签名/IHDR/IDAT/IEND 四段全部合法，解码为 1x1 RGBA 黑像素（`{0,0,0,0}`），与注释「1x1 黑色像素占位（AppIndicator 会自适应缩放）」一致，可被 systray/AppIndicator 接受。

### 4. zhidao 冷启动 flake 残余——五处裸 mock 靶向连跑无样本

- 五处无 readyProbe 的裸 `httptest.NewServer` 宿主：client_test.go:172（TestNoAutoRelogin）/ :287（TestReloginIfNeeded）/ :419（TestLoginRetriesTransientInitError）/ :457（TestExitClass）/ :486（TestExitClassFailsOnCodeNotZero）。均由 TestMain 包级 socketPreheat（client_test.go:39-42）兜底。
- 本轮实证：`go test ./internal/zhidao/` 全量 1.948s 绿；五处靶向 `-count=1` 连跑 0.411s 五条全绿。**当前不触发**。
- 与 R74 报告 OBSERVE-74-02 结论一致，维持延续观察。

### 5. 生产逻辑关键契约复核（历史重点全表）——全部成立

| 契约 | 实现 | 复核结果 |
|------|------|----------|
| 窗口关闭三判据单源 | scheduler.go:913-934 windowClosedLocked | 成立。三条判据（主判据 state.WindowClosed / 时钟失败≥3+已过点 / 幽灵窗口 EmptyProbeRuns≥3+已过点）共用同一 `open := s.openTimeForLocked("")` 单快照（:919）；StateForAccount:707 与 WindowClosed:905 同源；10s 裕量判据侧（:1141）+入账侧（:1150）对称；probeIntervalForOpen:102 复用同一实现降频。TestWindowClosedState / TestWindowClosedTransitionStateGrace / TestStateForAccountMirrorsWindowClosed 全绿。 |
| 删号 memory-first | handler.go:1004-1018 | 成立。Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount 顺序与契约一致；PurgeAccount:499-523 含 openTimeDetected 识别槽清理（:510）、Courses 行剔除、inflight 清空；DeleteAccount 失败半删态由重启 Restore 自愈（注释明文）。 |
| sameClientFor 六分支 | scheduler.go:1484(失效)/1516(成功)/1546(风控)/1566(窗口关闭)/1595(实时复核入口)/1630(确证满员) | 成立。六分支全含指针身份比对；:1595 入口复核覆盖其后全部写回分支（:1603 maybeRelogin+setState+AppendLog、:1622 doneHas、:1630 markFull、:1655 setState+AppendLog），无裸露写点；maybeRelogin 决策侧（:1203）与写回侧（:1249）双 ClientFor 复核 + MarkDone/RemoveDone 手动路径复核（:1922/:1990）。靶向含 -race 连跑全绿（TestDeletedAccountRebuiltSameName{…} / TestRealtimeRecheckDeletedAccountDropsLog 等）。 |
| IsReadErr 四形态 | client.go:519-548 | 成立。FIN（errors.Is io.EOF）/ 短读（ErrUnexpectedEOF）/ 超时双文案（awaiting headers + reading body）/ RST（OpError.Op=="read"）四形态全覆盖；与 isConnErrRetryable（:492-501 只认 dial/write）互斥。 |
| doLogin 全局闸门 | manager.go:49-64 gateWait + :223-235 gateTryAcquire | 成立。两门共享 gateMu/gateUsed 计数（同一分钟窗口）；LoginByPassword:243-246 非阻塞准入绝不挂起用户响应；gateWait 只被 Relogin（自动重登 goroutine）使用；管理员换绑同走收口（handler.go:132 单一调用点）；ResetGateForTest 仅测试夹具。TestLoginByPasswordRejectsWhenGateBudgetExhausted / AllowedWhenGateBudgetAvailable 全绿。 |
| 零吞错落库点 | 全仓抽查 | 成立。scheduler.go 全部 store 调用点（成功 1527/1530、失效 1499、风控 1553、实时复核失效 1609、普通失败 1657、MarkDone 1940/1968/1971、RemoveDone 2015/2021/2024、SetTargets 481/489）全部 `if err != nil { log.Printf }`，零 `_ =`；全部在 `s.store != nil` 守卫内。TestStoreFailuresLogged / TestSetTargetsDeleteRefusedFailureLogged 双测试钉死。 |
| httpDo 仅 dial-write 重试 | client.go:474-501 | 成立。isConnErrRetryable 只认 dial/write；read 错误原样上抛（防双报）；业务/取消错误不重试；cloneReq:555-557 用 req.Clone，GetBody 由 stdlib 对 bytes.Reader 自动设置。 |
| config 双默认值 | config.go + env_test.go | 一致。默认识别引擎 ddddocr（实现 CaptchaEngineDefault:106-111 / .env 模板:151 / env_test:71-84 三处同向）；激活码默认 off（:43-45 / 模板:154 / env_test:46-66 / main.go:33-37 四处同向）。 |
| tick 零值守卫让位 WindowOpened | scheduler.go:1006 | 成立。`open.IsZero() && !opened` 才挂起；WindowOpened=true（发布级 inDateRange 确证）且识别槽空时放行黄金期冲刺——TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 绿；既有 TestSubmitSuspendedWhenOpenTimeCleared（Fixture WindowOpened=true）接 B41-02 语义保持绿（预置开窗后零 open 抽查确认挂起路径不被误伤）。 |
| probe 空快照不删识别槽 / PurgeAccount 删槽 | scheduler.go:1101-1105 / :510 | 成立。「关闭≠时间消失」契约：只覆盖非空 beginTimes、空快照保留槽（open_detect_test / open_retain_test 钉死）；删账号删槽（TestPurgeAccountClearsOpenTime 绿）。 |

## 发现

### MINOR

**MINOR-75-01（scheduler.go:99/:928 + scheduler_test.go:1442 三处「管理员热改开放时间」残留注释——R74 收净族漏网的 sibling）**

- 文件：`backend/internal/scheduler/scheduler.go:99、:928`；`backend/internal/scheduler/scheduler_test.go:1442`
- 一句话：R74 已把 scheduler.go:46-48/:50/:79-80 三处「管理员配置/open_time/reparse/OpenTimeParsed」残留注释收净，但同族仍有 :99「管理员热改开放时间，临门判断自然重新收紧」、:928「窗口若真开、管理员热改开放时间，success 探测…」点名管理员、:1442 测试注释「管理员热改新一轮的防守场景」——配置链路（Config.OpenTime 删除 + reparse/OpenTimeParsed 零残留）整体移除后，管理员没有任何热改开放时间的路径，真实触发源是平台 beginTimes 新批次覆盖识别槽（ProbeForAccount:849 / probe:1103）。
- 触发场景：维护者 grep「管理员热改」会看到两条指向已不存在管理路径的注释，与 R74 刚收净的 sibling 族不一致；按 :99/:928 的字面以为还有「管理员面板改开窗点」的运维口子，实际唯一事实源是平台自动识别。
- 修复建议：:99 改「平台若开放新一轮选课（识别槽刷新到未来），临门判断自然重新收紧」；:928 改「窗口若真开、平台下发新一轮 beginTimes，success 探测数据后势必推开始 open 实况」；:1442 改「（识别槽刷新到未来新一轮的防守场景）」。同族泛指措辞（:980「热改亚毫秒窗口」、:1138「热改亚毫秒窗口内主判据与 EmptyProbeRuns 入账」、:1421「开放时间热改到未来」）描述的是「open 值被新批次动态覆盖」的通用语义、不点名管理员，方向未误导，可一并收敛或从轻保留。

### OBSERVE

**OBSERVE-75-01（CI 矩阵仍无 Linux CGO=1 构建检查——R73/R74 报告建议仍未落实，Linux 桌面托盘零自动化编译验证）**

- 文件：`.github/workflows/ci.yml`、`backend/tray_linux.go`
- 一句话：ci.yml 仅 Linux/macOS CGO=0 与 Windows CGO=1 构建；tray_linux.go（linux&&cgo）依赖 systray 的 GTK3 头文件，本机无 Linux 交叉工具链（grp.h/sys/mman.h 缺失），其编译正确性持续零自动化防线。
- 触发场景：未来对 tray_linux.go 的改动若引入未定义符号/import 错误，现有 CI 矩阵恰好全部绕过（R72 前 showZenityOrPrint 事件同型）。
- 修复建议：给 ci.yml 加 Linux CGO=1 构建 job（`apt-get install libgtk-3-dev` + `go build`；不便装 GTK 时至少验证 cgo 前置编译错误）。仍属可留待后续的可选小项。

**OBSERVE-75-02（zhidao 五处无 readyProbe 裸 mock 宿主延续观察——靶向连跑无样本，包序风险可接受）**

- 文件：`backend/internal/zhidao/client_test.go:172/:287/:419/:457/:486`
- 一句话：本轮 zhidao 全量 + 五处靶向 `-count=1` 连跑均无 flake 样本，TestMain 包级 socketPreheat（:39-42）兜底生效；TestLoginNetworkErrorAbortsImmediately 已补 readyProbe 后 zhidao 无边角裸宿主。
- 触发场景：仅在 Windows 回环冷启动窗口恰好叠加到裸 mock 首请求时可能复现（多轮未现）。
- 修复建议：暂不处理，归入 flake 演进观察。若 CI 出现「首轮 zhidao connectex」，优先给这五处补 readyProbe。

## 可疑待核

无（本轮所有疑似均追到证据收尾；无造作臆断）。

## 已核无缺陷清单

- **R74 修复逐项核证通过**：scheduler.go 三处注释语义与实现逐字一致、无死引用残留；scheduler_test.go「实时复核入口」语义指位（scheduler.go:1595 实测正是入口 first sameClientFor，覆盖后文全部写回分支）精确不漂移。
- **死引用收净目标达成**：reparse / OpenTimeParsed 全仓（含测试）零残留；XUANKE_OPEN_TIME 无生产消费；Config 无 OpenTime 字段；main/handler_test 同传零值识别槽唯一事实源；env_test 守护测试在位。
- **Linux 托盘**：build tag 四方穷举互斥无空洞；showZenityOrPrint 双降级链完整；trayPNG 字节经 Go stdlib 实测合法（1x1 RGBA 黑像素）。
- **zhidao 冷启动 flake**：五处裸 mock 靶向 + 全量均绿，无样本。
- **生产逻辑契约全表**：窗口关闭三判据单源（10s 裕量两侧对称）/ 删号 memory-first（含识别槽清理）/ sameClientFor 六分支（含实时复核入口前置统一覆盖）/ IsReadErr 四形态 / doLogin 全局闸门（双门共享计数）/ 零吞错落库点（抽查十处全带日志、全带 nil 守卫）/ httpDo 仅 dial-write 重试 / config 双默认值 / tick 零值守卫让位 WindowOpened（B41-02 双向测试全绿）/ probe 空快照不删槽——逐条成立。
- **测试与构建实证**：`go vet ./...` 全绿；`go build ./...` 全绿；zhidao 全量 + 五处裸 mock 靶向绿；scheduler 关键契约含 -race 靶向绿；api/accounts/config/db/store/session/secure/runtime 八包全量绿；回归陷阱双测试（TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler）绿。
- **spawnChain 实时复核分支同一性**：:1595 入口 sameClientFor 复核覆盖其后全部写回点（:1603/:1616/:1638 三条 err 归并路径 + :1622 doneHas 胜利状态守卫），B43-02 六分支闭环核证。
- **CSRF 核查**：POST /api/logout 虽未强制 JSON，但跨站表单无法携带 Authorization 头（前端用 Bearer），实际无法通过 requireAuth——非缺陷，已排除。

## 结论

零 CRITICAL、零 MAJOR。1 个 MINOR（scheduler.go:99/:928 + scheduler_test.go:1442 三处「管理员热改开放时间」残留，R74 收净族漏网）+ 2 个 OBSERVE（CI 无 Linux CGO=1 检查 / 五处裸 mock 延续观察）。R74 修复逐项核证正确、死引用收净目标达成、测试与构建全绿。最值得主控处理的是 MINOR-75-01 的三处注释同族收尾——与 R74 已收净的 sibling 对齐后，全仓「管理员热改开放时间」表述将彻底清零。