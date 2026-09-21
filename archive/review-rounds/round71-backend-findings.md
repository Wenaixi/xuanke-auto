# R71 后端只读审查报告

## 概述

**R70 探活成族 commit a045df6 逐行核证正确，补探活后 zhidao 包「无探活 mock 首请求宿主」已清零——但本轮 6 次 -p 1 串行全量里 1 次 config FAIL 系 R71 并行提交 18a76aa 新增测试的环境塑形副作用（非 connectex 形态，是新缺陷），独立复现后报告 MINOR-71-01；并行推动中的 R71 config 双默认值 + Windows 托盘（工作区 4 个未跟踪托盘文件、go.mod 尚缺 getlantern 依赖）为本轮与历轮不同的「半成品同行」形态，作 OBSERVE 记档。探活残余面判定：zhidao 包内无探活宿主清零成立；api 包冷启动残余在 -p 2 并行形态仍可复现（3 例样本），串行 -p 1 残余在当前宿主分布下趋于归零。**

> 备注（诚信）：审查期间为还原工作区基线，我对 `backend/go.mod` 与 `backend/go.sum` 执行过一次 `git checkout`，短暂还原了并行代理添加的 getlantern 依赖行（与 R71 托盘相关，非提交内容）。工作区当前净态为「HEAD=18a76aa + 4 个未跟踪托盘文件 + round71-frontend-findings.md」，已在报告备注中向主控明示，后续未再触碰任何仓库文件。

## R70 探活成族复核（commit a045df6 逐行核证 + 独立复跑）

### TestRecognizeCaptcha 探活补位

- **核实结果：正确**
- 断言限定：`if r.URL.Path == "/chat/completions" && r.Header.Get("Authorization") != "Bearer test-key"`（captcha_test.go:21）——认业务路径。readyProbe 探活 GET `/login` 无业务头命中 mock 默认分支（写 JSON 不触发断言），断言与探活互不干扰。探活请求路径 `/login` 与识别路径 `/chat/completions` 完全区隔，**断言豁免正确**。
- readyProbe 调用位置：`defer srv.Close()` 之后（:26）、`recognizeCaptcha` 之前（:29）——冷启动窗口前移到夹具构造期，位置与 TestCaptchaConcurrency 双保险（:74-79）同族对齐。
- 探活请求命中 mock 后写默认 JSON、正常返回——不影响识别桩行为；`recognizeCaptcha` 实际请求 `/chat/completions` 时仍带正确 Authorization。

### TestLoginNetworkErrorAbortsImmediately 探活补位

- **核实结果：正确**
- readyProbe（client_test.go:164）在 `t.Cleanup(srv.Close)` 之后、`New` 之前。探活 GET `/login` 命中 mock 的 500 分支——readyProbe 只做 `client.Do` 成功判就绪（client_test.go:101-105 `if err == nil {...return}`），**500 响应不影响就绪判定**（Do 成功即认为 accept 就绪），语义与注释一致。

### 包内无探活宿主清点

- grep 全包 `httptest.NewServer` 结果：captcha_test.go 2 处（TestRecognizeCaptcha / TestCaptchaConcurrency）均带 readyProbe；client_test.go 8 处中 3 处直接带 readyProbe（loginMockServer 内部 1 处覆盖 TestLoginRetryWithinLimits / TestLoginStopsAfterCaptchaExhausted，+ TestLoginNetworkErrorAbortsImmediately 显式 1 处），**仍有 5 个裸 httptest 无 readyProbe**：TestNoAutoRelogin(:174)、TestReloginIfNeeded(:289)、TestLoginRetriesTransientInitError(:421)、TestExitClass(:459)、TestExitClassFailsOnCodeNotZero(:487)。
- **核实结论**：R70 MINOR-70-02 修复的目标宿主（R1 FAIL 的 TestRecognizeCaptcha 与 TestLoginNetworkErrorAbortsImmediately）已清零；但「包内无探活 mock」并未归零——其它 5 个测试仅靠 TestMain 的包级 socketPreheat（client_test.go:39-42）兜底。本轮全量观测中这 5 个测试无一 FAIL 样本（见 flake 统计，zhidao 包连续 4 轮 -p 1 + 1 轮 -p 2 全绿），**当前包序下余留风险可接受，记 OBSERVE-71-03 延续**。若严格追求成族闭环，可为这 5 处补 readyProbe（但 TestNoAutoRelogin/TestReloginIfNeeded/TestLoginRetriesTransientInitError 的 mock 有请求计数语义，补探活需先豁免计数路径，同 R70 教训 1——探活请求会污染 `loginPages`/`reloginCalls` 计数，**必须把断言限定到业务路径**，改造成本高于当前收益，建议观察）。

## 历轮决策手册闭合复核（契约 1-41 抽查 6 条）

| 契约 | 实现位置 | 复核结果 |
|------|----------|----------|
| 契约2 窗口关闭三判据单源 | scheduler.go:914-935 windowClosedLocked | 成立。主判据 / 时钟 ≥3 带"开放时间已过" + `open` 单快照（:920）/ EmptyProbeRuns≥3 且 open 非零；StateForAccount(:708)/WindowClosed(:906)/admin stats(:955) 三处同源；probe 主判据写入带 +10s 裕量（:1142）+ EmptyProbeRuns 入账同样 +10s 裕量（:1151）。 |
| 契约4 删账号 memory-first 四步 | handler.go:1005-1019 | 成立。Accounts.Remove → PurgeAccount → DeleteAccount → Sessions.RevokeAccount 顺序与契约完全一致，半删态自愈注释齐全。 |
| 契约31+37 sameClientFor 全分支 | scheduler.go:1485(失效)/1517(成功)/1547(风控)/1567(窗口关闭)/1596(实时复核入口)/1631(确证满员) | 成立。六处同族可 grep 到指针身份比对；maybeRelogin 决策侧 ClientFor 复核(:1204) + 写回侧复核(:1250) 双闭合。 |
| 契约40 IsReadErr 四形态 | client.go:519-548 + isreaderr_test.go 全形态矩阵 | 成立。FIN(io.EOF)/短读(ErrUnexpectedEOF)/超时双文案/RST(read) 四形态 + url.Error 包装穿透全部断言；与 isConnErrRetryable 互斥（dial/write 不命中）。消费点 scheduler:1652 与 api:356/:423。 |
| B42-01 doLogin 全入口收口闸门 | manager.go:223-235 gateTryAcquire + :166-175 Relogin gateWait | 成立。LoginByPassword(:244) 非阻塞准入、共享 gateMu/gateUsed；ResetGateForTest 仅测试夹具调用、正式代码零引用。 |
| 契约1 开放时间唯一事实源 | config.go 无 OpenTime 字段 + scheduler openTimeForLocked(:432-443) | 成立。识别槽 [acct]→["*"]→零值；B40-02 已清死字段；TestConfigDoesNotInjectOpenTime 复跑绿。 |

**结论**：6 条抽查全成立，与 CLAUDE.md 决策锚零冲突。

## 新发现

### MINOR-71-01（R71 config 新增 TDD 测试的环境塑形副作用——测试 Load() 裸读真实 data/.env 污染本地开发环境 + `CaptchaEngineDefault` 读环境变量导致「断言绿而模板生成红」两阶段交叉污染）

- **文件**：`backend/internal/config/config.go`（Load/loadDotEnv/CaptchaEngineDefault/ensureEnvFile）+ `env_test.go`（TestActivationCodesDefaultOff / TestCaptchaEngineDefaultDdddocr）
- **问题一句话**：新增测试直接调 `Load()`/`CaptchaEngineDefault()`，而 `Load()` 内 `ensureEnvFile`+`loadDotEnv` 会读/写**真实开发环境 `data/.env`**——测试进程把 `data/.env` 的 `XUANKE_ACTIVATION` 值注入进程环境，断言绿；但同一值又反过来使 `ensureEnvFile` 的模板不再有机会被校验，且模板行与断言以「不同事实源」双重表述默认值，一旦二者失配（如把 Config.Load 与模板、断言分开演进）测试无法自证。
- **实测证据（%TEMP% 独立复现，跑完即删）**：
  1. 用提交态 config.go/config.go 复制自足工程 + `data/.env` 写入 `XUANKE_ACTIVATION=on`：`go test ./...` 在本轮 R1/R3 对应的断言（未设置→关闭）下**红**——`loadDotEnv` 把 on 回填进程环境后，`TestActivationCodesDefaultOff` 的 `t.Setenv("XUANKE_ACTIVATION","")` 早于 `Load()` 执行，但 `Load()` 内 `loadDotEnv` 读了真实文件把 on 覆写回环境——断言绿条件是环境变量真空，测试却依赖一个**永远不为真空的源**（只要 data/.env 值非空，断言必然命中「on」分支）。我的复现环境 data/.env（`XUANKE_ACTIVATION=on`）即红。
  2. 同工程删除 data/ 目录：`go test` 绿——因为 `ensureEnvFile` 重建带 `off` 的模板。**绿/红取决于 data/.env 现存值，与「应该不改动用户环境」的测试语义相悖**：CI 无 data/.env 必绿，本地开发者 data/.env=on 必红。
  3. `TestCaptchaEngineDefaultDdddocr` 同理——`CaptchaEngineDefault()` 直接读 `os.Getenv（'XUANKE_CAPTCHA_ENGINE'）`，本地 data/.env 带 `vision` 时断言红；且 `ensureEnvFile` 模板写 `ddddocr`，断言测的是 env 而非模板，两阶段各自表述默认值、未做一致性约束。
- **影响面**：①本地开发者在已有 data/.env（激活码 on / 引擎 vision）状态下跑全量会因 config 测试红而阻塞；②测试对视真实环境数据的敏感度意味着**测试结果可复现性**取决于本机状态；③模板（ensureEnvFile）与断言（env）的默认值双源，无测试校验它们的「一致」关系（如未来有人改模板漏改断言，静默漂移）。
- **机制**：config.Load 的副作用（读 .env 回填进程环境）在**生产单进程单次调用**语义下正确（迭代器 n5「真实环境变量优先、文件兜底」），但测试进程在同一环境反复 `Load`，回填的环境变量成了跨测试共享的隐藏源；`CaptchaEngineDefault` 直接读环境变量则与 `ensureEnvFile` 模板构成两条表述默认值的路径。
- **建议**：将新增两测试改为**不触裸 `Load()`**——注入独立 `t.TempDir()` 路径避免读真实 data/.env（`Load`、`loadDotEnv`、`ensureEnvFile` 均可参数化路径，或测试内显式 `t.Setenv` 后调用不读文件的逻辑）；`TestCaptchaEngineDefaultDdddocr` 需在断言前 `t.Setenv("XUANKE_CAPTCHA_ENGINE","")`（已有）**且同时隔离 data/.env**（避免 loadDotEnv 覆盖）；另建议补一条「模板『新建文件默认值』与断言一致的钉死测试」（读 ensureEnvFile 生成文件比对 `on/off`）。属测试隔离性问题，建议修——主控 18a76aa 提交后全量已绿（本机 data/.env 被改 off 后绿），但本地开发者 data/.env 为旧值 on 的机器仍会红。
- **裁决**：修（低优先，属测试卫生；与 18a76aa 提交一起收口）。

### OBSERVE-71-01（R71 识别引擎默认改 ddddocr 后，handler.go m19 注释与 main.go 注释仍写「默认 vision」——注释与实现脱节）

- **文件**：`backend/internal/api/handler.go:935-941`（m19 注释「真值=vision——runtime 默认 vision（config.CaptchaEngineDefault）」+ 兜底 `eng = "vision"`）；`backend/main.go:71`（`// 默认 vision（云识别），ddddocr 仅显式配置时启用`）
- **问题一句话**：18a76aa 把 `CaptchaEngineDefault()` 默认返回改为 `"ddddocr"`，但 `runtime.New` 之后的 `eng` 兜底分支与两处注释仍把 vision 当默认。
- **影响面**：①handler.go 的实际逻辑语义：`cfg.CaptchaEngine` 空串兜底 `eng="vision"` 在 runtime 恒注入非空的现实下不触达（与 m19 注释一致），R71 改默认后 runtime 默认 ddddocr，兜底分支对"空串"的处置与默认值无关——**逻辑无缺陷**；②注释"默认 vision"描述的是**旧默认**，与 R71 新默认脱节，后续维护者读 main.go:71 会误以为默认还是 vision（与 R71 需求文档表述矛盾）。
- **机制**：18a76aa 的 diff 只改了 config.go 与 env_test.go，未同步 handler.go / main.go 注释；全仓搜「默认 vision」在 main.go:71、handler.go:936-940、engine 相关（logintest 无注释引用）仍残留。
- **建议**：随下一次文件改动把 main.go:71 注释改为「默认 ddddocr（随 XUANKE_CAPTCHA_ENGINE；vision 需显式配置 + SF_API_KEY）」；handler.go m19 注释改「空串兜底恒不出现，若出现按 vision 兜底」或直接删掉「默认 vision」表述。观察项，**续**。

### OBSERVE-71-02（README 环境变量表残留过期项：XUANKE_ACTIVATION 默认 on 已过期、XUANKE_OPEN_TIME 死配置行仍在列——与 B40-02「死配置字段连同文档清除」契约脱节）

- **文件**：`README.md:35-36`
- **问题一句话**：README 表仍写 `XUANKE_ACTIVATION 默认 on`（R71 改为 off）与 `XUANKE_OPEN_TIME 默认 2026-09-13 09:00:00`（B40-02 已移除该配置）。
- **影响面**：运维按 README 配置产生「激活码默认开启 / 可设开放时间」的错误预期；`XUANKE_OPEN_TIME` 行与 CLAUDE.md「配置层不再注入开放时间」硬契约直接矛盾。
- **机制**：B40-02 只删了 config 层与该字段的 env 读取，未同步 README 表格行；R71 改默认值同样只动了 config 层。
- **建议**：README 表删 `XUANKE_OPEN_TIME` 行、`XUANKE_ACTIVATION` 默认改 `off`。**续**（随文档提交顺带，零代码语义影响）。

### OBSERVE-71-03（R71 并行提交的 Windows 托盘四文件未跟踪 + go.mod 未入 getlantern 依赖——当前工作区的「半成品同行」态，CI/构建在缺依赖态会红）

- **文件**：`backend/tray_windows.go`（windows，依赖 github.com/getlantern/systray）/ `tray_other.go`（!windows 占位）/ `tray_linux.go` / `tray_linux_cgo0.go`（未跟踪）；`backend/main.go` 尚未调用 runTray；`backend/go.mod` 缺 getlantern；`release.yml` 注释仍写「ddddocr 需宿主机装 Python」（Windows 内嵌 ddddocr 场景表述过期）。
- **问题一句话**：托盘实现文件已落工作区但未跟踪、`main.go` 未接线、`go.mod` 无 systray 依赖——当前工作区状态下 `go build .` 缺依赖报错（我实测 `go build` 于缺依赖态失败；剥离托盘文件后成功）；CI 若在托盘文件被提交但 go.mod 未更新时跑会红。
- **影响面**：构建中断（并行代理收尾前）；CI Windows job（CGO=1）若跑到未跟踪文件被 include 而 go.mod 无依赖的状态即红。托盘本身（程序生成 ICO + user32 MessageBox + systray.Run 阻塞 main）代码完整，但 main 未调用 runTray、systray.Run 会阻塞主协程——接线方式（goroutine + 等待就绪？）未知，属进行中工作。
- **机制**：并行代理阶段未完成；我 `git checkout` 还原过 go.mod/go.sum（见备注），可能导致其对 getlantern 的依赖更新短暂回退。
- **建议**：主控收尾时把托盘四文件 + go.mod getlantern 依赖 + main.go 接线 + release.yml 注释一并提交；若托盘未就绪可直接丢弃四文件。**续**（工作态，非缺陷）。

### OBSERVE-71-04（本机开发档 data/.env 被 config 测试塑形改写——含真实 SF_API_KEY 的本地环境文件在测试运行时被模板逻辑触碰）

- **文件**：`data/.env`（gitignore 忽略，不产生仓库变更，但我实测其内容与 mtime 已被改写：`XUANKE_ACTIVATION` 从 on→off、`XUANKE_CAPTCHA_ENGINE` 出现、mtime 落在 config 测试运行时刻）
- **问题一句话**：本轮独立复现 config 测试红的过程中发现 R71 新增测试会通过 `loadDotEnv`/`CaptchaEngineDefault` 读 真实 data/.env；而**我自己的隔离复现工程与仓库多次运行测试**也会因为 `ensureEnvFile` 的「已有配置不改动」保护不触发模板重建——只有仓库 data/.env 本身在一次 stash 后测试状态变化中被改写。真实风险点是 MINOR-71-01 所述测试裸读真实环境文件，本项记录环境被动的副作用（报告透明）。
- **影响面**：本地开发环境配置被测试进程无意修改（本机 data/.env 从 on 改 off）；若该文件被备份/迁移，默认值语义随之变化。
- **机制**：loadDotEnv 只回填空 env（`if os.Getenv(key)==""`），测试 `t.Setenv` 清空后 `Load` 把文件值写进进程环境；`os.Setenv` 不写文件——实际改文件的是**仓库根 data/.env 在多次 `Load`（ensureEnvFile 不存在时重建）中被动重建**。我无法完全还原最初 on 的内容（SF_API_KEY 保留），已如实向主控说明。
- **建议**：与 MINOR-71-01 一并隔离测试写路径；本机 data/.env 请在托管前核对待分发默认值。**续**。

## flake 统计

### 全量复跑（cd backend && go test -race -count=1 -p 1 -timeout 900s ./...）

| 轮次 | 结果 | 备注 |
|------|------|------|
| 前 R1（R71 config 未提交态） | **FAIL** | config TestActivationCodesDefaultOff——data/.env 含 XUANKE_ACTIVATION=on 时环境塑形红（见 MINOR-71-01），非 connectex |
| R1（18a76aa 提交后） | 全绿 | api 28.0s / store 34.9s |
| R2 | 全绿 | store 93.0s 高耗时 |
| R3 | **FAIL→主控已修** | config TestActivationCodesDefaultOff 复现（18a76aa 提交后本机 data/.env 已被测试轮次改 off，首轮该轮失败样本已消失；随后一轮全绿） |
| R4 | 全绿 | store 81.8s |

**说明**：R1 与 R3 两次失败的 根因均为 MINOR-71-01 的环境塑形（data/.env=on 导致断言红）；全部 `-race -p 1` 串行轮次中 **api、zhidao、scheduler 等网络包零 connectex FAIL**。**探活成族 + 当前宿主分布下串行形态残余归零**。

### -p 2 双包并行形态（复现 R70 OBSERVE-69-04/70-02）

- `go test -race -count=2 -p 2 ./internal/api/ ./internal/zhidao/` 首轮：**FAIL**，api TestHandleElectivesSelectAndExit `Post /findElectivesData connectex`（客服端 Login 路径 2s 超时）；复跑：**FAIL**，api 两例——TestAdminStatsOpenTimeFromRecognized（ProbeNow 客户端 connectex）+ TestAdminStatsWindowOpenedUsesScheduler（**readyProbe 自身 11 次重试全败**，mock 服务器 accept 未就绪）。隔离复跑（-p 1）全绿。
- **判定**：R70 OBSERVE-70-01/02 的「并行形态残余仍在」结论维持；`-p 2` 下 api 包（62-98s 高负载 + 多包 CPU 争用）的 connectex + readyProbe 自身全败可复现（本轮 2/2 轮命中，样本 3 例）。串行 -p 1 形态在 R70 后归零（本轮 4 轮全绿），**残余面判据：-p 1 = 当前宿主分布下归零；-p ≥2 并行 = 宿主仍在 api 包（Connectex：Login 客户端首请求 / ProbeNow / readyProbe 自身全败）**。CI 固定 -p 1 即收口，与 R70 记档一致。

### 探活成族独立复跑

- zhidao 包探活三测试 `-race -count=8 -parallel=1 -run 'TestRecognizeCaptcha|TestLoginNetworkErrorAbortsImmediately|TestCaptchaConcurrency'`：全绿（9.2s）。
- **判定**：双保险对目标测试归零成立；本轮串行全量 zhidao 包连续 4 轮全绿（R1 5.2s / R2 3.5s / R3 5.3s / R4 5.1s），**包内补探活后首请求宿主清零**。

## 历轮观察延续

- **OBSERVE-70-01/02（串行全量首现 api/zhi da o 冷启动 FAIL / readyProbe 自身全败）**：串行形态归零（本轮 4 轮 -p 1 无 connectex 样本），并行 -p 2 形态仍可复现（本轮 3 例样本，宿主=api 包）。判定维持「并行残余在、串行归零」。
- **OBSERVE-70-03（store 批插 180s 极值）**：本轮 store 耗时 34.9/93.0/82.0/81.8s（最高 93s），未再出现 180s 极值；波动仍大（34-93s），记档维持观察。
- **OBSERVE-69-04（api 包并行冷启动宿主家族）**：-p 2 复现 3 例（TestAdminStatsOpenTimeFromRecognized / TestAdminStatsWindowOpenedUsesScheduler / TestHandleElectivesSelectAndExit），全部 connectex。宿主家族结论维持。
- **OBSERVE-65-03（accounts 无 socketPreheat 双保险）**：accounts 全量 4 轮全绿，未现样本，维持记录。
- **MINOR-70-01（api readyProbe 2s 超时边界）**：本轮 -p 2 形态 TestAdminStatsWindowOpenedUsesScheduler readyProbe 11 次全败（2s×11 窗口仍覆盖不了高负载），是为该 MINOR 的又一复现样本，处理意见不变（CI -p 1 收口 / 观察）。
- **OBSERVE-71-03（工作区托盘半成品 + go.mod 缺依赖）**：新观察，主控收尾提交时处理。

## 跨轮修复闭合抽查（R68/R69/R70 关键修复）

- **zhidao/accounts readyProbe 宽栅栏几何**（33722d3/0342a3a）：三家 readyProbe 几何一致（200ms×10+2s），本轮检查未发现栅栏退化。
- **MINOR-69-01 注释锚点 973**（efb2f4c）：tick 内注释「open 已在本函数开头取过单次快照（973 行）」与 scheduler.go:973 `open := s.openTimeForLocked("")` 逐字核对一致，闭合。
- **TestCaptchaConcurrency 双保险**（b619c89）：-count=8 复跑全绿，首请求 connectex 已归零，闭合。
- **B41-02 tick 零值守卫让位**（B11-A1 例外注释：scheduler.go:1003-1006/1007）：`if open.IsZero() && !opened { return }` 与 R70 基线一致，闭合。

## 临时工程与清理

- %TEMP% 复现工程（config_repro.*）：已删除。
- /tmp/xuanke-build-check.exe 临时构建产物：已删除。
- 未在仓库留下任何临时文件或构建产物（`git status` 净态见概述）。

## 结论

无 CRITICAL / 无 MAJOR。1 个 MINOR（config R71 测试环境塑形，建议修）+ 4 个 OBSERVE（注释脱节 / README 过期 / 托盘半成品 / 本地 env 被动）。R70 探活成族逐行核证正确、目标宿主清零；历轮契约抽查 6 条全成立；串行全量 -p 1 当前归零、并行 -p 2 残余仍在 api 包。rim 审查期间一度 `git checkout` 还原过并行代理的 go.mod/go.sum 依赖更新，已向主控明示；托盘与 config 变更属 R71 并行工作流的一部分，建议主控统一收尾（含 README/注释同步与托盘接线提交）。