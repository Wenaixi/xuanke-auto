# R73 后端只读审查报告

基线：commit 06e23f7（R72 收尾）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto，分支 master。只读铁律全程遵守（Read/Grep/Glob/Bash 只读命令；后台跑过 `go test ./...`、`go test -race` 四包、`go vet ./...`、Linux/macOS CGO=0 交叉编译与交叉 vet、Windows CGO=1 本机构建，均为只读验证）。

## 概述

**全模块深度走读 backend/main.go、tray 四文件、browser、router、embed、cmd/{logintest,probe,bench} + internal 九包全部 .go（含 _test.go）后：零 CRITICAL、零 MAJOR、3 个 MINOR、3 个 OBSERVE。R72 五项修复逐一核证全部正确；Linux 托盘 build tag 穷举互斥实证通过；全量测试 + race + 交叉编译全绿。遗留为两处"同族未扫净"的行号引用悬空 + 托盘防错桩未删的收尾小项。**

## 重点复核结论（R72 变更后回归）

### 1. R72 修复正确性（commit 4f10764 五项）——全部通过

| 项 | 核证结果 |
|----|----------|
| ①tray_linux.go 新补 showZenityOrPrint（:96-104） | **实现正确**。`exec.LookPath("zenity")` 探测 → `exec.Command(...).Run()` 成功即返回，失败/无 zenity 降级 `fmt.Printf` 控制台打印，绝不阻塞托盘。`os/exec` import 现在确被真实使用（先前是预埋桩）。降级链完整（showAboutLinux→showZenityOrPrint→print）。 |
| ②scheduler.go:1589 与 :1628 陈旧行号改语义指位 | **准确**。1589 `（inflight 已在 SelectClass 返回后的统一清位处删掉，无残留）`——实际清位在 :1509（SelectClass 返回后、实时复核前），语义指位正确；1628 `（入口处的存在性复核只挡"账号不存在"…）`——入口复核实际在 :1404（spawnChain 链顶 ClientFor），语义指位正确。R72 报告 MINOR-72-01/SUSPECT-72-01 已闭环。 |
| ③config.go:148 模板注释改 ddddocr | **一致**。`默认识别引擎 ddddocr 免密钥，vision 云识别才需填` 与 CaptchaEngineDefault()（:106-111 默认 ddddocr）及 README:34 完全一致。 |
| ④cmd/logintest 删 SF_API_KEY 硬校验 | **完整可跑**。删后按 `CaptchaEngineDefault()` 分支：ddddocr → NativeDdddOcrAvailable → LocalDdddOcrAvailable → 回退 Vision，四级引擎链完整；vision 分支正常。ddddocr 部署下不再被硬校验拦死。 |
| ⑤tray_linux_cgo0.go:6 乱码注释 | **已修**。`（需 GTK3/liヒン 桌面库）` → `（需 GTK3 等 Linux 桌面库）`，注释可读。 |

### 2. Linux 桌面托盘完整性——build tag 穷举实证通过，CI 覆盖仍是缺口

- **build tag 互斥穷举实证**（go list 平台文件集）：`GOOS=linux CGO_ENABLED=0 go list` → `[browser_unix.go main.go tray_linux_cgo0.go]`；`GOOS=linux CGO_ENABLED=1 go list` → `[browser_unix.go main.go tray_linux.go]`。`linux && cgo` / `linux && !cgo` 与 native_ocr 双轨、tray_windows/tray_other 四方穷举无重叠无空洞。
- **showAboutLinux→showZenityOrPrint 链路完整**：showAboutLinux(:85-92) 调 showZenityOrPrint(:96-104)，函数有定义、import 齐备，交叉 vet（Linux/macOS CGO=0）零未定义符号。
- **唯一不可本地验证点**：Linux CGO=1 桌面构建需 GTK3 头文件与交叉 gcc，本机无此环境（复现编译失败在 grp.h/sys/mman.h 缺失，非业务代码问题）。systray Linux 后端的 cgo 依赖满足度、appindicator 运行时行为均无任何自动化验证——CI 矩阵（ci.yml / release.yml）至今不含任何 Linux CGO=1 构建步骤，R72 报告 OBSERVE-72-01 修复建议"CI 补一条 Linux CGO=1 构建检查"未落实（见 OBSERVE-73-01）。
- 附带核证：tray_linux.go trayPNG 手写 1x1 PNG 字节经独立校验（IHDR/IDAT/IEND 三 chunk 的 CRC 与 zlib 流全部合法），是可被 AppIndicator 接受的最小合法 PNG。

### 3. zhidao 冷启动 flake 残余——五处裸 mock 宿主当前不触发

- 五处无 readyProbe 裸 `httptest.NewServer`（client_test.go TestNoAutoRelogin:174 / TestReloginIfNeeded:289 / TestLoginRetriesTransientInitError:421 / TestExitClass:459 / TestExitClassFailsOnCodeNotZero:487）：TestMain 包级 socketPreheat + 本机全量 `go test ./...` 与 `go test -race` 四包全绿，当前包序下无样本（与 R71/R72 结论一致）。
- api/zhidao/accounts 三包 readyProbe 几何一致（10 次×200ms + 显式 2s 超时）；api 包 newTestDepsModeName 套接字预创建 + readyProbe 双保险在位。

### 4. 生产逻辑关键契约复核（历史重点全表）——全部成立

| 契约 | 实现 | 复核结果 |
|------|------|----------|
| 窗口关闭三判据单源 | scheduler.go:914-935 windowClosedLocked | 成立。主判据（曾开窗+空快照+已过点+10s 裕量）/ 时钟失败≥3 / 幽灵窗口 EmptyProbeRuns≥3 三判据单源；StateForAccount(:708)/WindowClosed(:906)/admin stats(:954) 三处同源；裕量写入侧与判据侧对称。 |
| 删号 memory-first | handler.go:1004-1018 | 成立。Remove → PurgeAccount → DeleteAccount → RevokeAccount 顺序与契约一致；TestAdminDeleteAccountMemoryFirst 绿。 |
| sameClientFor 六分支 | scheduler.go:1485(失效)/1517(成功)/1547(风控)/1567(窗口关闭)/1596(实时复核入口)/1631(确证满员) | 成立。六分支全含指针身份比对；TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+RealtimeUnauthorized 六测试绿。 |
| IsReadErr 四形态 | client.go:519-548 | 成立。RST(*net.OpError.Op=read)/FIN(io.EOF)/短读(ErrUnexpectedEOF)/超时双文案 四形态全覆盖，isreaderr_test.go 断言含 url.Error 包装链穿透与 dial/write 反例。 |
| doLogin 全局闸门 | manager.go:49-64 gateWait + :223-235 gateTryAcquire | 成立。两门共享 gateMu/gateUsed 计数；LoginByPassword(:244) 非阻塞准入绝不挂起用户响应；ResetGateForTest 仅测试夹具。 |
| 开放时间唯一事实源 | config.go 无 OpenTime 字段 + openTimeForLocked(:432-443) | 成立。TestConfigDoesNotInjectOpenTime 守护；env 读取残留仅在注释/测试文案中；settings 不再落 open_time 键（handler_test 断言）。 |
| 零吞错落库点 | 全仓 grep `_ = d.store` / `_ = s.store` / `_ = st.store` | 零命中。全部 `if err := ...; err != nil { log.Printf }`；TestStoreFailuresLogged/TestSetTargetsDeleteRefusedFailureLogged 双测试钉死。 |
| httpDo 仅 dial-write 重试 | client.go:474-501 | 成立。isConnErrRetryable 只认 dial/write；read 错误上抛（双报防线）；业务/取消错误原样上抛；recognizeViaVision 同走 httpDo 复用。 |
| config 双默认值七处一致 | 实现/兜底/注释/README/模板/模板注释/测试 | 一致。R72 修的 config.go:148 注释后七处全同向（ddddocr/off）。 |

## 发现

### MINOR

**MINOR-73-01（tray_linux.go:106-107 两行防错桩未删——R72 补 showZenityOrPrint 后 fmt 桩已成死代码、strings 桩是 import 遮羞布）**

- 文件：`backend/tray_linux.go:106-107`
- 一句话：`var _ = strings.Builder{}` 与 `var _ = fmt.Sprintf` 两行"防未用 import"桩在 R72 补 showZenityOrPrint（真实使用 `fmt.Printf`）后：fmt 桩已成纯死代码（fmt 已被真实引用），strings 桩是"strings 包仍未被真实使用"的补丁式遮羞。与 R72 前掩盖 showZenityOrPrint 未定义符号的补丁同族，属于 R72 修复的**未收尾残留**。
- 触发场景：维护者 grep strings 在 tray_linux.go 的使用只见桩，误以为有业务引用；后续删 strings import 时若不同时删桩会编译失败。
- 修复建议：删两行桩 + `"strings"` import（tray_linux.go 无任何 strings 真实使用，grep 实证仅此桩一处）。

**MINOR-73-02（scheduler.go:1705 注释行号悬空——`（1021 行）` 错指 WindowClosed 守卫的 return）**

- 文件：`backend/internal/scheduler/scheduler.go:1705`
- 一句话：注释 `读侧 isRateLimitedLocked 已用 nowAlignedLocked()（1021 行）` 中 `1021 行` 指向 tick 内 WindowClosed 守卫的 `return`（:1021），而 isRateLimitedLocked 内 nowAlignedLocked 调用的实际位置在 :1437（spawnChain 内 `now := s.nowAlignedLocked()` 传参）。与 R72 修复的 1589/1628 两处同族——R72 只修了报告点名的两处，未全仓扫净。
- 触发场景：维护者按 1021 行跳转看到的是窗口守卫 return，与注释所述"isRateLimitedLocked 读侧"语义脱节。
- 修复建议：与 1589/1628 同标准——改为语义指位（如"见 isRateLimitedLocked 实现"）或修正为实际行号。

**MINOR-73-03（scheduler.go:1746 注释行号悬空——`（1273 行 classFullInSnapshot）` 错指 reloginResults select）**

- 文件：`backend/internal/scheduler/scheduler.go:1746`
- 一句话：注释 `spawnChain 已先于实时复核用快照判满员跳过（1273 行 classFullInSnapshot）` 中 `1273 行` 实际是 maybeRelogin goroutine 内的 `reloginResults` 非阻塞发送 select（:1274-1277）；classFullInSnapshot 定义在 :1717，spawnChain 内调用在 :1453。行号全错指。
- 触发场景：同上，按注释跳转看到的是重登回传通道 select，误导排查满员退避逻辑。
- 修复建议：同 MINOR-73-02。建议全仓执行一次"行号引用"族扫描（scheduler.go 内仅剩 3 处行号引用，其中 :980 的 `（973 行）` 精确命中正确，另两处悬空）。

### OBSERVE

**OBSERVE-73-01（CI 矩阵仍无 Linux CGO=1 构建检查——R72 报告建议未落实，Linux 桌面托盘零自动化编译验证）**

- 文件：`.github/workflows/ci.yml` / `.github/workflows/release.yml` + `backend/tray_linux.go`
- 一句话：R72 报告 OBSERVE-72-01 的修复建议"release.yml/CI 补一条 Linux CGO=1 构建检查"未落地。ci.yml 仅测 Linux/macOS CGO=0 与 Windows CGO=1；release.yml Linux 产物走 CGO=0。tray_linux.go（linux&&cgo）的编译正确性、systray 的 GTK 依赖满足度无任何自动化防线，本机也无交叉验证环境。
- 触发场景：未来对 tray_linux.go 的改动若引入未定义符号/import 错误，现有 CI 矩阵恰好全部绕过（同 R72 前的 showZenityOrPrint 事件）。Linux 桌面托盘当前是"改坏了也不知道"的状态。
- 修复建议：给 ci.yml 加一条 Linux CGO=1 构建 job（仅 `go build`，需安装 libgtk-3-dev 等构建依赖；若 CI runner 不便装 GTK，可至少验证 Go 侧文件集/编译到 cgo 前置）。

**OBSERVE-73-02（zhidao 五处无 readyProbe 裸 mock 宿主延续观察——本机全量+race 无样本，包序风险可接受）**

- 文件：`backend/internal/zhidao/client_test.go:174/:289/:421/:459/:487`
- 一句话：R71 记档的 OBSERVE-71-03 前身延续。本轮全量十包与 race 四包均无 flake 样本，TestMain 包级 socketPreheat 兜底生效；若未来 CI 出现"首轮 zhidao connectex"类失败，优先给这五处补 readyProbe。
- 触发场景：仅在 Windows 回环冷启动窗口恰好叠加到裸 mock 首请求时可能复现（多轮未现）。
- 修复建议：暂不处理，归入 flake 演进观察。

**OBSERVE-73-03（logintest 识别引擎读环境变量而非 DB settings——管理员后台热改的引擎在工具中不生效）**

- 文件：`backend/cmd/logintest/main.go:57-72`
- 一句话：logintest 引擎选择用 `config.CaptchaEngineDefault()`（读环境变量 + data/.env），而管理员在后台热改的引擎持久化在 DB settings 表。后台把引擎改成 vision 后，data/.env 的 `XUANKE_CAPTCHA_ENGINE=ddddocr` 行未变 → logintest 仍走 ddddocr 分支（无本地引擎时日志显示"配置为 ddddocr 但无本地引擎，回退 Vision"，实际仍用 Vision 识别成功，仅日志语义偏差）。
- 影响：诊断工具与实际生效引擎的标注可能不一致；识别功能本身不受影响（四级回退链兜底）。非功能缺陷。
- 修复建议：可选。让 logintest 复用 `LoadSettings` 读 captcha_engine 键，或在日志中注明"引擎取自环境变量，管理员后台热改值见 settings 表"。

## 已核无缺陷清单

- **R72 五项修复逐项核证通过**：showZenityOrPrint 实现正确（LookPath+exec+双降级，os/exec import 合理）/ 1589+1628 语义指位准确 / config.go:148 模板注释与实现一致 / logintest 删硬校验后 ddddocr 四级引擎链完整 / tray_linux_cgo0.go 乱码已修。
- **Linux 托盘 build tag 穷举**：go list 平台文件集实证 `linux&&cgo`→tray_linux.go、`linux&&!cgo`→tray_linux_cgo0.go 互斥无空洞；与 tray_windows（windows）/tray_other（!windows&&!linux）四方穷举；交叉 vet（Linux/macOS CGO=0）零未定义符号。
- **trayPNG 手写 PNG**：独立校验 IHDR/IDAT/IEND chunk 长度、CRC32、zlib 流全部合法，是可加载的最小 1x1 RGBA PNG。
- **窗口关闭三判据单源 / 删号 memory-first / sameClientFor 六分支（六测试绿）/ IsReadErr 四形态（isreaderr_test.go 断言矩阵）/ doLogin 全局闸门（gateTryAcquire 非阻塞）/ 开放时间唯一事实源 / 零吞错落库点 / httpDo dial-write 互斥 / config 双默认值七处一致**：逐条复核成立。
- **测试与构建实证**：`go test -p 1 -count=1 ./...` 十包全绿（含 scheduler 13.4s、api 17.0s）；`go test -race` scheduler/accounts/api/zhidao 四包全绿；`go vet ./...` 全绿；Linux/macOS CGO=0 交叉编译与交叉 vet 全绿；Windows CGO=1 本机构建绿。
- **session.IsAdmin 与 IsAdminToken 重复**：为 2 处调用的微小内部重复，实现一致无行为分叉，不构成缺陷（不动）。
- **前端构建产物一致性**：vite outDir=`../backend/web/dist`、emptyOutDir=true，与 `//go:embed all:dist` 目录指向一致。
- **probeSem cap=4 结构化信号量**：per-account 探测并发封顶在位；账户级探测不写全局 lastProbe（管理员穿透不吞全校节流闸门）。
- **db.Open 迁移顺序**（schema → migrateAddPublishMeta → refuseLegacy）与缺列清单对应剔除正确；TestMigrateAddsPublishMetaColumns 在位。
- **build 目录零脏**：backend/web/dist 未纳入 git（.gitignore:46），`//go:embed all:dist` 依赖构建时产物，CI 前端构建先行顺序正确。

## 结论

零 CRITICAL、零 MAJOR。3 个 MINOR（tray_linux.go 两行防错桩未删 / scheduler.go 两处行号引用悬空）+ 3 个 OBSERVE（CI 无 Linux CGO=1 检查 / 五处裸 mock 延续观察 / logintest 引擎读取源偏差）。R72 修复逐项核证全部正确、测试与构建全绿。**最值得主控处理的是 MINOR-73-01（删两行防错桩+strings import）与 MINOR-73-02/03（两处行号引用改为语义指位）——三者均为 R72 修复的收尾残留，一行改动即闭环；OBSERVE-73-01（CI 补 Linux CGO=1 构建检查）是 R72 报告未落实的长期建议。**
