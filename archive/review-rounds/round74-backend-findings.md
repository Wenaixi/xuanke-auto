# R74 后端只读审查报告

基线：commit 410ca4c（R73 收尾）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto，分支 master，HEAD=410ca4c。只读铁律全程遵守（Read/Grep/Glob/Bash 只读命令；后台跑过 `go vet ./...`、`go build ./...`、Linux/macOS/windows 三种平台交叉 vet/编译、`go test -count=1 ./...` 十包、`go test -race` scheduler/api/accounts 三包、zhidao 五处裸 mock 连跑 3 轮，均为只读验证）。

## 概述

**零 CRITICAL、零 MAJOR、2 个 MINOR、2 个 OBSERVE。R73 四项修复逐项核证全部正确；Linux 托盘 build tag 穷举互斥 + trayPNG 字节合法性实证通过；zhidao 冷启动 flake 五处裸 mock 连跑 3 轮无样本；全量测试 + race + 三种平台交叉编译全绿。新发现集中在两处轻微注释陈旧（scheduler.go:80/46-48 "管理员配置"表述与唯一事实源契约不再一致）+ CI 无 Linux CGO=1 构建检查的延续观察。**

## 重点复核结论（R73 变更后回归）

### 1. R73 修复正确性（commit 9f5c217 四项）——全部通过

| 项 | 核证结果 |
|----|----------|
| ①tray_linux.go 删两行防错桩 + strings import | **编译干净。** 删 `var _ = strings.Builder{}`/`var _ = fmt.Sprintf` 与 `"strings"` import 后：`os/exec` 被 showZenityOrPrint(:96) 的 `exec.LookPath` 真实使用；`fmt` 被 :95-103 的 `fmt.Printf` 真实使用；无任何 import 悬空。Linux CGO=1 交叉 vet 绕过 GTK 系统头失败（grp.h/sys/mman.h 缺失为宿主机缺交叉工具链，非业务代码问题）后 exit=0 共享语义，不掩盖业务编译问题。Windows CGO=1 本机构建全绿。 |
| ②scheduler.go:1705 改「见下方实现」 | **语义指位准确。** 1703-1705 注释讲的是"写侧 markRateLimitedLocked 必须与读侧 isRateLimitedLocked 同用 nowAlignedLocked"；读侧实现就在 1687-1701（isRateLimitedLocked 末行 `if now.Before(until)` 用调用方传入的 now），下方 markRateLimitedLocked 的 1710 行 `s.nowAlignedLocked().Add(d)` 正是该句的语义落点。"见下方实现"指位合理。与 R73 报告 MINOR-73-02 闭环。 |
| ③scheduler.go:1746 改「见 spawnChain 内 classFullInSnapshot 调用」 | **语义指位准确。** :1742-1746 注释所述"spawnChain 已先于实时复核用快照判满员跳过"实际对应 :1452-1457（spawnChain 链内 `if s.classFullInSnapshot(...)` → markFullLocked → continue）。spawnChain 定义在 :1379，函数内部 1453 行正是该调用，"见 spawnChain 内 classFullInSnapshot 调用"精确定位。与 R73 报告 MINOR-73-03 闭环。 |
| ④全仓行号引用族扫净 | **应仅剩 :980「(973 行)」一处精确命中。** 全仓 grep `（N 行）` 精确格式仅剩 scheduler.go:980。该处 973 行实测正是 `open := s.openTimeForLocked("")`（tick 函数开头取的单次快照），注释所述"open 已在本函数开头取过单次快照（973 行）"与实现逐行一致——精确命中不悬空。scheduler_test.go:3420/3421 残留两处 `（1575 行…）` 为测试注释（"修复前（1575 行只有 ClientFor）"），指代的是"当时修复前该测试写的行号"，属历史回归叙述而非代码指位引用（该测试现位于 :3422，行号确已漂移），为 MINOR-74-01（见下）。R73 报告"仅剩 980 精确命中"与"另两处悬空"均闭环。 |

### 2. Linux 桌面托盘——build tag 穷举互斥 + trayPNG 合法性实证通过

- **build tag 穷举**（Linux CGO=0 `go list` → [browser_unix.go main.go tray_linux_cgo0.go]；Linux CGO=1 ◀ cross vet 显示 systray 文件集在 build 时被选入；Windows 恒选 tray_windows.go native 双轨）：`linux && cgo`（tray_linux.go）/ `linux && !cgo`（tray_linux_cgo0.go）互斥无空洞；`!windows && !linux`（tray_other.go）覆盖 darwin。四方穷举结论与 R73 一致。
- **showZenityOrPrint 双降级链**：`showAboutLinux(:84-91)` → `check zenity 存在`（LookPath）→ Run 成功即返回 → 失败/无 zenity 降级 `fmt.Printf` 控制台打印，绝不因对话框失败阻塞托盘。`exec.LookPath`/`exec.Command` 均真实使用，os/exec import 合理。
- **trayPNG 字节合法性**：独立逐字节校验 1x1 RGBA PNG——签名 ok、IHDR/IDAT/IEND 三 chunk 长度正确、三 CRC 全部匹配、IDAT zlib 解压 5 字节 0000000000（= 1 像素 RGBA 黑 DEFLATE），结构完全合法、可被 AppIndicator 接受。注释"1x1 黑色像素占位（AppIndicator 会自适应缩放）"与字节实际一致（无中心白块，注释已如实说明为纯黑占位）。

### 3. zhidao 冷启动 flake 残余——五处裸 mock 宿主连跑 3 轮无样本

- five 处无 readyProbe 裸 `httptest.NewServer`（client_test.go:172/:287/:419/:457/:486 + captcha_test.go:16/:50）：TestMain 包级 socketPreheat + 本机 `go test ./...`（十包全绿）与 `-count=1` 靶向连跑 3 轮（同 R73 靶向 run 集）均无 flake。报告中 5 处专测靶向（含 TestLoginNetworkErrorAbortsImmediately）+ 连跑 3 轮全部 0.35s~0.55s 通过。
- **结论：五处裸 mock 当前不触发**（与 R73 相同）。TestLoginNetworkErrorAbortsImmediately 已补 readyProbe（R73 修正），当前 zhidao 全包已无边角裸宿主。

### 4. 生产逻辑关键契约复核（历史重点全表）——全部成立

| 契约 | 实现 | 复核结果 |
|------|------|----------|
| 窗口关闭三判据单源 | scheduler.go:914-935 windowClosedLocked | 成立。主判据（曾开窗+空快照+已过点+10s 裕量）/ 时钟失败≥3+已过点 / 幽灵窗口 EmptyProbeRuns≥3 三判据单源；StateForAccount(:704-729)/WindowClosed(:903-907)/admin stats(:954) 三处同源；probeIntervalForOpen(:102) 复用同一实现降频；裕量 10s 判据侧（:1142）与入账侧（:1151）对称。 |
| 删号 memory-first | handler.go:1004-1018 | 成立。Remove → PurgeAccount → DeleteAccount（库行）→ RevokeAccount 顺序与契约一致；DeleteAccount 失败半删态由重启 Restore 自愈并显式注释；PurgeAccount(:500-524) 含 openTimeDetected 识别槽清理。 |
| sameClientFor 六分支 | scheduler.go:1485(失效)/1517(成功)/1547(风控)/1567(窗口关闭)/1596(实时复核入口)/1631(确证满员) | 成立。六分支全含指针身份比对；TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+RealtimeUnauthorized 六个测试全绿（-race）。另有 maybeRelogin 决策侧(:1204)与写回侧(:1250)双 ClientFor 复核 + MarkDone/RemoveDone 手动路径(:1923/:1991)复核。 |
| IsReadErr 四形态 | client.go:519-548 | 成立。RST（*net.OpError.Op=read）/ FIN（io.EOF）/ 短读（ErrUnexpectedEOF）/ 超时双文案（awaiting headers + reading body，与 Go 1.26 stdlib client.go:737/994 逐字一致）四形态全覆盖；isreaderr_test.go 断言含 url.Error 包装链穿透与 dial/write 反例；与 isConnErrRetryable 互斥（read 恒 false）。 |
| doLogin 全局闸门 | manager.go:49-64 gateWait + :223-235 gateTryAcquire | 成立。两门共享 gateMu/gateUsed 计数（同一分钟窗口）；LoginByPassword(:243-290) 非阻塞准入绝不挂起用户响应；gateWait 只被 Relogin（自动重登 goroutine）使用；ResetGateForTest 仅测试夹具。from api handler_test 撞名学生/闸门用例全绿。 |
| 开放时间唯一事实源 | config.go 无 OpenTime 字段 + openTimeForLocked(:432-443) | 成立。TestConfigDoesNotInjectOpenTime 断言 Config 结构体与 Load 返回值均无 OpenTime 字段、XUANKE_OPEN_TIME 环境变量不被消费；TestAdminStatsOpenTimeFromRecognized 断言 stats 输出识别值/open_time_set 语义；handleAdminConfig 不再含 open_time 键（settings 落库断言）。 |
| 零吞错落库点 | 全仓 grep `_ = d.store`/`_ = s.store`/`_ = st.store` | 零命中。全部 `if err := ...; err != nil { log.Printf }`；TestStoreFailuresLogged/TestSetTargetsDeleteRefusedFailureLogged 双测试钉死。scheduler 全部 store 访问点（grep 16 处）都有 `s.store != nil` 守卫。 |
| httpDo 仅 dial-write 重试 | client.go:474-501 | 成立。isConnErrRetryable（:492-501）只认 `nerr.Op=="dial"`/`=="write"`；read 错误上抛（双报防线）；业务/取消错误原样上抛；cloneReq（:555-557）用 req.Clone 轻拷贝、GetBody 由 stdlib 对 bytes.Reader 自动设置、重放完整 body 无双报；recognizeViaVision（captcha.go:162）同走 httpDo 复用活性自愈。 |
| config 双默认值七处一致 | config.go：实现/注释/模板/模板注释/env_test 断言 | 一致。默认 ddddocr / off 七处（实现 CaptchaEngineDefault、模板行、模板注释、R72 修后注释、README、env_test 断言）全同向。 |

## 发现

### MINOR

**MINOR-74-01（scheduler_test.go:3420/3421 两处 `（1575 行…）` 测试注释行号已漂移）**

- 文件：`backend/internal/scheduler/scheduler_test.go:3420-3421`
- 一句话：`// 修复前（1575 行只有 ClientFor）：红…` 与 `// 修复后（1575 行换 sameClientFor）：绿…` 中 `1575 行` 指代的是该测试修复当时 scheduler.go 的行号（当时 1575 是 spawnChain 实时复核入口复查点）。当前 scheduler.go 中该 sameClientFor 复核已在 :1596（实时复核入口），:1575 现为"非满员失败：改为实时人数复核"的注释行——行号已漂移。这是测试注释对"修复前后行为"的历史叙述引用，非代码指位（不指向跳转），漂移不影响理解，但同属"行号引用不随代码演进"族。
- 触发场景：维护者按 1575 跳到 scheduler.go 看到的是注释行而非实时复核入口，与注释所述"换 sameClientFor"实际位置（:1596）有 21 行偏移。
- 修复建议：与 scheduler.go:980 同标准收敛——改为语义指位（如"修复前实时复核入口只有 ClientFor"）或补行号更新。属收尾可选小项（测试注释，不影响任何行为）。

**MINOR-74-02（scheduler.go:46-48/:50/:79-80 三处"管理员配置/管理员 PUT"表述与唯一事实源契约不再一致——残留注释）**

- 文件：`backend/internal/scheduler/scheduler.go:46-48、:50、:79-80`
- 一句话：`:46-48` "优先取管理员配置，未配置时取该账号自己探测识别的 beginTimes"、`:50` "管理员配置或平台 beginTimes"、`:79-80` "管理员 PUT open_time=''...runtime.reparse 置 OpenTimeParsed 零值"——三处主题（管理员配置 open_time / reparse / OpenTimeParsed）在 2026-09 配置链路整体移除后均已无实体：Config 无 OpenTime 字段、runtime.Config 无 reparse、config.go 无 XUANKE_OPEN_TIME 读取（handler_test:691-727 已断言 open_time 键从 settings 落库移除、PUT 携带被静默忽略）。存续注释会让维护者误以为仍有"配置层注入开放时间"的路径。
- 触发场景：维护者 grep 管理员配置/open_time 时看到旧语义注释，可能误按旧文档在 .env 添加配置（已被忽略产生误导），或误以为可以热改设置开窗点。
- 修复建议：三处注释改为与唯一事实源一致的表述——如「开放时间唯一事实源 = 平台 beginTimes 自动识别，配置层无注入路径」；`上:46-48` 删去"优先取管理员配置"；`:50` 改"平台 beginTimes 自动识别"；`:79-80` 例子改"识别槽无值"（删除 reparse/OpenTimeParsed 死引用）。属注释陈旧（无行为影响）但值得扫净。

### OBSERVE

**OBSERVE-74-01（CI 矩阵仍无 Linux CGO=1 构建检查——R73 报告建议仍未落实，Linux 桌面托盘零自动化编译验证）**

- 文件：`.github/workflows/ci.yml:66-74` / `.github/workflows/release.yml:142-150`、`backend/tray_linux.go`
- 一句话：ci.yml 仅 Linux/macOS CGO=0 与 Windows CGO=1 构建；release.yml Linux 产物走 CGO=0。tray_linux.go（linux&&cgo）的编译正确性、systray 的 GTK 依赖满足度仍无任何自动化防线；本机 Linux CGO=1 交叉 vet 复现代码路径被宿主机缺 grp.h/sys/mman.h 交叉工具链阻断（R73 报告 OBSERVE-73-01 同论证）。
- 触发场景：未来对 tray_linux.go 的改动若引入未定义符号/import 错误，现有 CI 矩阵恰好全部绕过（同 R73 前 showZenityOrPrint 事件）。Linux 桌面托盘当前是"改坏了也不知道"的状态。
- 修复建议：给 ci.yml 加一条 Linux CGO=1 构建 job（`apt-get install libgtk-3-dev` + `go build`；若 CI runner 不便装 GTK，可至少验证 go vet 到 cgo 前置编译错误）。release.yml 保持 CGO=0（无桌面分发），不受影响。

**OBSERVE-74-02（zhidao 五处无 readyProbe 裸 mock 宿主延续观察——连跑 3 轮无样本，包序风险可接受）**

- 文件：`backend/internal/zhidao/client_test.go:174/:289/:419/:457/:486`（同 R73 报告五处）
- 一句话：本轮全量十包与 race 三包 + 专门靶向连跑 3 轮均无 flake 样本，TestMain 包级 socketPreheat（:39-42）+ zhidao 包内 readyProbe（:50/:88）兜底生效。TestLoginNetworkErrorAbortsImmediately 已补 readyProbe 后，zhidao 无边角裸宿主。
- 触发场景：仅在 Windows 回环冷启动窗口恰好叠加到裸 mock 首请求时可能复现（多轮未现）。
- 修复建议：暂不处理，归入 flake 演进观察。若 CI 出现"首轮 zhidao connectex"类失败，优先给这五处补 readyProbe。

## 可疑待核

无（本轮所有疑似均追到证据收尾；无造作臆断）。

## 已核无缺陷清单

- **R73 四项修复逐项核证通过**：tray_linux.go 编译干净（fmt/os/exec 均真实使用、无 import 悬空）/ 1705「见下方实现」语义指向 isRateLimitedLocked-now 参数与 markRateLimitedLocked-nowAlignedLocked 落点完整 / 1746「见 spawnChain 内 classFullInSnapshot 调用」精确指向 1453 / 行号引用族仅剩 980 精确命中（973=`openTimeForLocked` 行，逐字一致）。
- **Linux 托盘**：build tag 穷举（linux+cgo / linux+!cgo / windows / !windows&&!linux）互斥无空洞；showZenityOrPrint 双降级链完整；trayPNG 字节逐字节校验合法（签名/CRC/zlib 全对）。
- **zhidao 冷启动 flake**：五处裸 mock 连跑 3 轮全部 0.35s~0.55s 通过，无样本；TestLoginNetworkErrorAborts 已补 readyProbe。
- **生产逻辑契约全表复核**：窗口关闭三判据单源（含 10s 裕量两侧对称）/ 删号 memory-first / sameClientFor 六分支（六测试含 -race 全绿）/ IsReadErr 四形态（超时双文案与 Go 1.26 stdlib 逐字一致）/ doLogin 全局闸门（双门共享计数）/ 开放时间唯一事实源 / 零吞错落库点（16 处 store 访问全带 nil 守卫）/ httpDo 仅 dial-write 重试（cloneReq GetBody 无双报）/ config 双默认值七处一致——逐条成立。
- **测试与构建实证**：`go test -count=1 ./...` 十包全绿；`go test -race` scheduler(14.2s)/api(13.7s)/accounts(1.4s) 全绿；`go vet ./...` 全绿；Linux CGO=0 / macOS CGO=0 / Windows CGO=1 编译全绿；Linux CGO=1 交叉 vet 复现仅宿主机缺交叉工具链头文件（非业务代码缺陷）。
- **零吞错落库点深化**：scheduler.go 全部 16 处 store 访问（SetTargets 2 / UpdateIDToken 1 / AppendLog 7 / SaveSuccess 2 / SaveRefused 1 / DeleteRefusedClass 1 / DeleteSuccess 1 / DeleteRefused 1）均在 `s.store != nil` 守卫内；MarkDone 手动路径同守护。
- **probeIntervalFor 死代码判断**：非死代码——被 TestProbeIntervalFor/TestProbeIntervalZeroOpenTime 等测试直接调用，prod 路径统一走更紧凑的 probeIntervalForOpen 分支，保留为测试/外部可读语义。不算缺陷（OBSERVE 也不值，函数有真实消费者）。
- **httpDo 超时文案版本一致性**：Go 1.26.1 stdlib client.go:737/994 两处 timeoutError 文案与 IsReadErr 双 Contains 逐字匹配——超时形态判据在该 Go 版本可用；未来升级 Go 若改 stdlib 文案需同步回归（关注项，非缺陷）。
- **托盘启动阻塞语义**：runTray 阻塞至托盘装配完成（trayStartOnce），main 在托盘就绪后放行；Docker/无头 CGO=0 占位直接返回不阻塞。openBrowser 双平台实现（windows rundll32 / unix xdg-open|open）在位。
- **session 票据**：TestActivationTicket 覆盖颁发/绑定/单次/换账号/不存在五种；sweepLoop 5 分钟清扫按 ttl 防御（ttl=0 不启动）无泄漏。
- **store 日志窗口**：LoadLogs/LoadAllLogs 空库 NULL 窗口语义由 TestLoadLogsEmptyDBWindowSemantics 钉死（空 slice 不报错不 panic）。

## 结论

零 CRITICAL、零 MAJOR。2 个 MINOR（scheduler_test.go:3420/3421 测试注释行号漂移 / scheduler.go 三处"管理员配置"残留注释）+ 2 个 OBSERVE（CI 无 Linux CGO=1 检查 / 五处裸 mock 延续观察）。R73 修复逐项核证全部正确、测试与构建全绿。最值得主控处理的是 MINOR-74-02（三处注释与唯一事实源契约对齐，删 reparse/OpenTimeParsed 死引用）——同族注释收尾；MINOR-74-01 与 OBSERVE-74-01 为可留待后续的可选小项。