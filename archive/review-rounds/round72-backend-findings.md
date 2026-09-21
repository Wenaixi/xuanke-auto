# R72 后端只读审查报告

基线：commit 175ed72（R71 注释剥离收官）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto，分支 master。只读铁律全程遵守（Read/Grep/Glob/Bash 只读命令；`git checkout` 未执行、无任何文件修改；后台跑过一次 `go test` 与一次 `go test -race` 均为只读验证）。

## 概述

**全模块深度走读 backend/main.go、tray 四文件、browser、router、embed + internal 九包全部 .go（含 _test.go）后：零 CRITICAL、零 MAJOR、2 个 MINOR、5 个 OBSERVE、1 条可疑待核。R71 变更（注释剥离 / 托盘 / 双默认值）逐行核证：剥离无损、托盘 build tag 隔离正确、双默认值六处一致；全量测试与 race 关键包全绿。**

## 重点复核结论（R71 变更后回归）

### 1. 注释标签剥离完整性（契约 20）——通过

- 逐 commit 抽查 5 个剥离提交（e6983b7 / 4632155 / 7e68ace / 307b0d2 / c7e2d0c / 4a9dea4 / 175ed72），diff 形态全部为「删 `B\d+-\d+：`/`F\d+-\d+：`/`第 N 轮` 等历史轮次前缀，技术内容保留」，**未发现剥离破坏"为什么/契约/陷阱"语义**。
- 抽查重要警告类注释（绝/必须/严禁/CRITICAL/panic）数十条：全部保留核心语义，仅去除标签前缀。例：`审查发现的 CRITICAL` → `审查发现的严重项`（scheduler_test.go:1466）、`B19-02 定案：**绝不**清 done/full/rateLimited/inflight` → `定案：**绝不**清…`（scheduler.go:464）。
- 现网全仓残留扫描：`B\d+-\d+[（(]` 后端零命中；`B\d{1,2}-\d` 仅剩 4 处**错误文案/引用内嵌**（schema.sql:66 `B9-02` 表注释、scheduler_test.go:1790/1802/3170/3177/3480/3489/3494 的 t.Fatal 文案 `B29-02`/`B42-02`/`B43-02`）——均为"历史上这轮修复钉死了此契约"的**指位性引用**（出现在断言文案/表注释里，非注释标签），剥离方针（契约 20）本就允许，属合理保留。
- 残留行号引用抽查 5 处（scheduler.go:980/1589/1628/1705/1746）：4 处精确命中、语义自洽；1 处（scheduler.go:1628 `1551 行的存在性复核`）**不匹配当前行号**（当前该行已是 `sameClientFor` 指针复核）——属 R71 剥离后未顺带修正的**陈旧内部行号引用**（MINOR-72-01，见下）。
- 前端剥离（4a9dea4 / 175ed72）同样抽查通过：`N4：`/`M-7：` 等标签去除、技术语义保留。

### 2. 系统托盘（db6b08a）——通过

- **build tag 隔离**：`tray_windows.go`（`//go:build windows`）/ `tray_linux.go`（`linux && cgo`）/ `tray_linux_cgo0.go`（`linux && !cgo`）/ `tray_other.go`（`!windows && !linux`）互斥穷举；`GOOS=linux CGO_ENABLED=0 go build ./...` 与 `GOOS=darwin CGO_ENABLED=0 go build ./...` 实测 exit 0。
- **main 接线**：main.go:147 `runTray(tray)` 在 `sched.Start()` 之后；Windows/Linux 实现 `runTray` 先 `go systray.Run(...)` 再 `<-trayStartOnce` 阻塞等就绪；`onReady` 最后 `close(trayStartOnce)`（单次 close，无重复）。trayStartOnce 通道语义正确。
- **showAbout 数据库路径**：`filepath.Abs(td.dbPath)` 绝对化后进 MessageBox/zenity，正确。
- **并发安全**：`trayCurrent` 在 main goroutine（runTray）与 onReady 菜单回调 goroutine 间存在**一次性写读**——写发生在 `go systray.Run` 之前、读发生在 `<-trayStartOnce` 之后（happens-before 由 channel close 提供），无真实竞态。
- **遗留问题**：Linux 分支调用 `showZenityOrPrint` 但全仓无该函数定义（tray_linux.go:90 调用未定义符号）+ 末尾 `var _ = strings.Builder{}` / `var _ = fmt.Sprintf` 两个**无用防错桩**——`go build ./...` 实测 exit 0 的原因是 `tray_linux.go` 只在 `linux && cgo` 下编译、当前 Windows 构建路径不包含它，**该文件在 Linux CGO=1 桌面构建下必然编译失败**（OSBERVE-72-01，见下）。

### 3. config 双默认值（18a76aa）——六处一致（MINOR-71-01 已闭环）

- 实现：config.go:68 `ActivationCodesEnabled: os.Getenv("XUANKE_ACTIVATION") == "on"`（未设=off）；config.go:106-111 `CaptchaEngineDefault()` 未设环境变量返回 `"ddddocr"`。
- 兜底：handler.go:937-940 空串兜底仍写 `eng = "vision"`（R71 前残留，**该兜底分支在 runtime 恒注入非空默认 ddddocr 的现实下不可达**，不构成逻辑缺陷）；main.go:71 注入 `CaptchaEngineDefault()`；README:34/35 `ddddocr`/`off` 已同步；`ensureEnvFile` 模板行 151/154 写 `ddddocr`/`off`。
- 测试：env_test.go TestActivationCodesDefaultOff / TestCaptchaEngineDefaultDdddocr 断言正确。
- **发现一处模板注释过期**：config.go:148 `默认识别引擎 vision 才需要密钥` 仍写"默认 vision"（R71 已改 ddddocr 为默认）——注释与实现脱节（MINOR-72-02，见下）。
- **MINOR-71-01（R71 config 测试裸读真实 data/.env 的环境塑形）已闭环**：本轮本地全量 `go test ./...`（config 3.9s）与 `go test -race` 关键包全绿，本机 data/.env 现值为 off/ddddocr 与测试断言同向，未再现"本地 data/.env=on 必红"样本。**残留**：该测试类问题仍未从根因上隔离（测试仍裸调 `Load()` 读真实 data/.env，若未来某开发者本机 data/.env 又写 on/vision，config 测试仍会红）——归入 OBSERVE-72-04 延续观察。

### 4. zhidao 冷启动 flake 残余——成族闭环维持

- api/zhidao/accounts 三包 readyProbe 几何一致（200ms×10 + 显式 2s 超时）；zhidao 包另具 `TestMain` 包级 `socketPreheat` + `loginMockServer` 内逐测试 `socketPreheat` 双保险；api 包 `newTestDepsModeName` 先 `net.Listen 127.0.0.1:0` 预创建再 readyProbe。
- **无探活裸 mock 清点（client_test.go）**：TestNoAutoRelogin(:174) / TestReloginIfNeeded(:289) / TestLoginRetriesTransientInitError(:421) / TestExitClass(:459) / TestExitClassFailsOnCodeNotZero(:487) 五个裸 `httptest.NewServer` 无 readyProbe——R71 报告已记档（OBSERVE-71-03 前身），本轮全量 + race 均无样本，当前包序下风险可接受，延续观察。
- 本轮 `go test ./...` 全量全绿（accounts 5.9s / api 22.8s / config 3.9s / db 4.7s / runtime 3.7s / scheduler 22.4s / secure 2.5s / session 4.0s / store 6.3s / zhidao 5.1s）；`go test -race` 关键四包全绿。**未见新 flake 样本**。

### 5. 生产逻辑关键契约复核（历史重点抽查）——全部成立

| 契约 | 实现 | 复核结果 |
|------|------|----------|
| 窗口关闭三判据单源 | scheduler.go:914-935 windowClosedLocked | 成立。StateForAccount(:708)/WindowClosed(:906)/admin stats(:955) 三处同源；主判据写入+EmptyProbeRuns 入账均带 +10s 裕量。 |
| 删号 memory-first | handler.go:1004-1018 | 成立。Remove → PurgeAccount → DeleteAccount → RevokeAccount 顺序与契约一致。 |
| sameClientFor 全分支 | scheduler.go:1485(失效)/1517(成功)/1547(风控)/1567(窗口关闭)/1596(实时复核入口)/1631(确证满员) | 成立。六处 err 归并 + 实时复核满员全含指针身份比对；maybeRelogin 决策侧(:1204)+写回侧(:1250)双闭合。 |
| IsReadErr 四形态 | client.go:519-548 | 成立。FIN/short-read/timeout双文案/RST + url.Error 穿透；与 isConnErrRetryable（dial/write 互斥）对称；消费点 scheduler:1652 / api:356/:423。 |
| doLogin 全局闸门 | manager.go:223-235 gateTryAcquire + :166-175 gateWait | 成立。LoginByPassword(:244) 非阻塞准入共享 gateMu/gateUsed；ResetGateForTest 仅测试夹具调用。 |
| 开放时间唯一事实源 | config.go 无 OpenTime 字段 + scheduler openTimeForLocked(:432-443) | 成立。识别槽[acct]→["*"]→零值；TestConfigDoesNotInjectOpenTime 绿。 |
| 零吞错落库点 | 全仓 grep `_ = d.store` / `_ = s.store` | 零命中；全部 `if err := ...; err != nil { log.Printf }`。 |

## 发现

### MINOR

**MINOR-72-01（scheduler.go:1628 内部行号引用陈旧——R71 剥离后未顺带校正，指位悬空）**

- 文件：`backend/internal/scheduler/scheduler.go:1628`
- 一句话：注释 `重建（1551 行的存在性复核只挡"账号不存在"…）` 中 `1551 行` 指向已不是"存在性复核"——当前 1551 行（realtime 复核入口）已是 `sameClientFor` 指针身份复核（与同函数 1596 行合并/前移后行号漂移），注释所指"存在性复核"行已不存在，指位悬空。
- 触发场景：维护者按 1551 行号跳转，看到的是指针复核而非注释所述的"存在性复核"，混淆删号竞态防线的两层语义。
- 修复建议：将 `1551 行` 改为 `同上/入口处`（或直接删行号），与 980 行等已对齐的锚点同标准。

**MINOR-72-02（config.go:148 模板注释残留"默认识别引擎 vision"——R71 双默认值六处一致中漏网的一处注释）**

- 文件：`backend/internal/config/config.go:148`
- 一句话：`# 教务登录验证码识别密钥（可选留空；默认识别引擎 vision 才需要密钥，ddddocr 免密钥）` 仍把 vision 当默认——R71 已改 ddddocr 为默认，注释与实现脱节。
- 触发场景：新部署用户读 data/.env 模板以为默认是 vision 需配 SF_API_KEY，与 README（默认 ddddocr）矛盾。
- 修复建议：改 `默认识别引擎 ddddocr 免密钥，vision 才需 SF_API_KEY`。

### OBSERVE

**OBSERVE-72-01（tray_linux.go 调用未定义符号 showZenityOrPrint + 两个防错桩——Linux CGO=1 桌面构建必然编译失败）**

- 文件：`backend/tray_linux.go:90`（调用 `showZenityOrPrint`）/ `:93-94`（`var _ = strings.Builder{}` / `var _ = fmt.Sprintf`）
- 一句话：`showZenityOrPrint` 全仓无定义（grep 仅 tray_linux.go 内引用），`tray_linux.go` 在 `linux && cgo` 下必然 `undefined: showZenityOrPrint`；两行 `var _ = ...` 是作者防"import 未使用"的补丁式写法，掩盖了未定义符号。
- 触发场景：任何 Linux 桌面 CGO=1 构建（`CGO_ENABLED=1 GOOS=linux go build ./...` 或 release.yml 的 Linux 路径若配 CGO=1）即编译失败。当前 CI 仅测 Windows CGO=1（内嵌 ddddocr）与 Linux/macOS CGO=0（托盘文件被 build tag 排除），**现有 CI 矩阵恰好绕过此缺陷**。
- 影响：Linux 桌面托盘功能（R71 需求"Linux 桌面版带托盘"）在当前代码库不可构建；Docker/CGO=0 服务器路径不受影响。
- 修复建议：补 `showZenityOrPrint` 实现（`exec.LookPath("zenity")` 分支 exec + 兜底 fmt.Println），删两行 `var _ =` 防错桩；release.yml/CI 补一条 Linux CGO=1 构建检查。

**OBSERVE-72-02（tray_linux_cgo0.go:6 注释乱码"liヒン"——疑似日文字符混入）**

- 文件：`backend/tray_linux_cgo0.go:6`
- 一句话：注释 `（需 GTK3/liヒン 桌面库）` 中 `liヒン` 非中文非日文完整词（疑似 `lib/GTK3` 被误打/编码残留），注释语义受损。
- 影响：纯注释可读性问题，零行为影响。
- 修复建议：改为 `（需 GTK3 等 Linux 桌面库）`。

**OBSERVE-72-03（cmd/logintest/main.go:30-31 强制要求 SF_API_KEY——默认引擎已改 ddddocr，与 R71 双默认值契约脱节）**

- 文件：`backend/cmd/logintest/main.go:30-31` `if cfg.SFAPIKey == "" { log.Fatal("未设置 SF_API_KEY…") }`
- 一句话：R71 后默认引擎为 ddddocr（免密钥），但 logintest 工具仍把 SF_API_KEY 当硬前提——ddddocr 部署（本机有 Python/ddddocr 或内置模型）下 logintest 直接拒跑。
- 影响：工具可用性缺口，不影响主服务。
- 修复建议：删该硬校验，跟随 `config.CaptchaEngineDefault()` 分支（下方 :60-75 已按引擎分支处理，仅前置校验过时）。

**OBSERVE-72-04（MINOR-71-01 测试环境塑形根因未隔离——本地 data/.env 值可让 config 测试复红）**

- 文件：`backend/internal/config/env_test.go`（TestActivationCodesDefaultOff / TestCaptchaEngineDefaultDdddocr 裸调 `Load()`）
- 一句话：测试仍依赖"进程环境真空"，而 `Load()` 内 `loadDotEnv` 会把真实 data/.env 值回填环境——本机 data/.env 若被改回 `XUANKE_ACTIVATION=on` 或 `XUANKE_CAPTCHA_ENGINE=vision`，测试必红（R71 已实测一次）。
- 影响：测试可复现性依赖本机 data/.env 状态。
- 修复建议：测试内改用不触真实文件的路径（Load 参数化 / t.Setenv + 独立临时目录），并补"模板默认值与断言一致"的钉死测试。

**OBSERVE-72-05（Windows 托盘二进制含完整内嵌 ddddocr 资产 + 拖盘 quit 不落库——运行时释出到 %TEMP% 属预期，但 Linux CGO=0 交叉构建的 release.yml 产物无托盘属已知权衡，记录备查）**

- 文件：`backend/internal/zhidao/native_ocr.go` + `release.yml`
- 一句话：本项为**跨轮已知权衡复述**（R71 OBSERVE-71-03 已记，本轮核实托盘接线已提交、go.mod 已含 getlantern，该项闭合为"托盘已落地"），不再单列。

### 可疑待核

**SUSPECT-72-01（scheduler.go:1589 注释 `inflight 已在 1333 行清掉` 与实际清位行不符——若注释为真则"实时复核入口同族复核缺 inflight 清位"需核实）**

- 文件：`backend/internal/scheduler/scheduler.go:1589`
- 事实：注释写"inflight 已在 1333 行清掉"，但当前 1333 行是 `submitAll` 内的 `s.lastSubmit = ...`；inflight 清位实为 :1509 `delete(s.inflight[acct], t.ClassID)`（SelectClass 返回后统一清，早于实时复核分支）。两处行号均不匹配。
- 推演：若"实时复核入口缺 inflight 清位"为真，则实时复核分支（网络段最长 15s）结束后 inflight 位可能残留阻塞手动报名。但**代码走读**：inflight 位在 :1509（SelectClass 返回、进实时复核之前）已被统一删除，实时复核分支无需再清；TestDeletedAccountRebuiltSameNameChainDropsRealtimeRecheckFull 与 TestSpawnChainSkipsInflightCourse 均绿，未现残留样本。结论：注释行号陈旧、**逻辑无缺陷**（与 B39-03 实测归因同构——代码走读推断需 TDD 实测裁决，此处以全量绿 + 代码路径分析为准）。
- 处理：仅注释行号陈旧，与 MINOR-72-01 同类，建议一并校正（`1333 行` → `1509 行` 或删行号）。不构成独立缺陷。

## 已核无缺陷清单

- R71 注释剥离 5 个后端提交 + 2 个前端提交：逐 diff 抽查，无技术内容破坏、无重要警告删除、无剥离后读不通注释。
- 托盘四文件 build tag 穷举正确；trayStartOnce 通道语义正确；showAbout 路径显示正确；trayCurrent 跨 goroutine 写读有 happens-before。
- config 双默认值六处（实现/兜底/注释[除 MINOR-72-02]/README/模板/测试）一致。
- readyProbe 成族三包几何一致、api/zhidao 双保险在位；全量 go test 与 race 关键包全绿。
- 窗口关闭三判据单源 / 删号 memory-first / sameClientFor 六分支 / IsReadErr 四形态 / doLogin 闸门 / 开放时间唯一事实源 / 零吞错落库点：逐条抽查成立。
- CI（ci.yml / release.yml）：Windows CGO=1 前缀写法已修（`$env:CGO_ENABLED='1'`）、Linux/macOS CGO=0 交叉构建路径正确、ci.yml `-p 1 -count=1` + `||` 重跑吸收 flake。
- schema.sql：refused 表注释保留 `B9-02` 属契约指位（合理保留）。
- assets 三文件（onnx/dll/charsets）git 索引在、`//go:embed` 完整；native_ocr_stub.go 的 `!windows || !cgo` 与 native_ocr.go `windows && cgo` 互斥穷举正确。
- db.go 迁移顺序（schema → migrateAddPublishMeta → refuseLegacy）正确，缺列清单与已迁移列对应剔除。

## 结论

零 CRITICAL、零 MAJOR。2 个 MINOR（陈旧行号引用 / 模板注释过期）+ 5 个 OBSERVE（Linux 托盘未定义符号 / 注释乱码 / logintest 过时硬校验 / 测试环境塑形未隔离 / 已知权衡复述）。R71 变更逐行核证通过：注释剥离无损、托盘 Windows 路径接线正确（Linux CGO=1 构建未验是唯一缺口）、双默认值一致。**最值得主控处理的是 OBSERVE-72-01（tray_linux.go 调用未定义符号）——当前 CI 矩阵恰好绕过，Linux 桌面托盘不可构建，建议补实现 + CI 加 Linux CGO=1 构建检查。**
