# R85 后端只读审查报告

审查对象：xuanke-auto HEAD commit `617bd5a`（R84 收官，进度 85/256）。本轮为纯归档轮延续——R84 无代码修改（仅 3 份归档文档），重点：确认 worktree 状态与 R84 归档完整性、build tag 10 文件互斥矩阵 + trayPNG/ICO 双回归钉（含 R80 硬锚 4264/22）全量走查、构建/race 实测、契约 20 全仓扫描、生产逻辑契约新角度 8 项抽核（登录限流桶独立 / 激活票据消费 + 并发原子 / LoadRefused 空库 / SubmitAll 分组 / 识别槽全校单值 / Stop 幂等 / maybeRelogin 退避封顶 / 托盘退出链路）。

审查方式：全程只读。唯一写入文件为本报告，仓库工作树零改动（`git status --short --branch` 为 `## master` 洁净）。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O85-01：api 包 race 全量抖动仍可在首跑复现——`ReadyProbe`+套接字预创建缓解后低频残余仍在，多轮连跑收敛零 FAIL，归因维持（Windows 回环 TIME_WAIT 冷启动）

**文件 + 行号：** `backend/internal/api/handler_test.go:51-73`（`newTestDepsModeName` 套接字预创建 + `readyProbe`）`;:139`（`readyProbe(zhi.URL)`）对照 `handler_test.go:1251`（`TestAdminElectiveSelectUnknownAccountRejects`）、`:1449`（`TestLoginActivateSeparateBuckets`）

**实测记录（`-race`，`-p 1` 串行）：**

| 轮 | 命令 | 结果 |
|---|---|---|
| 全仓 `go test -race -count=1 -p 1 ./...` | 首跑 | FAIL `internal/api` 40.9s，失败测试名未回传 |
| 复跑 `./internal/api/ -failfast` | 首跑 | FAIL `TestAdminElectiveSelectUnknownAccountRejects` 12.3s，断言 `handler_test.go:1259`「管理员对不存在的账号报名应被拒绝」收到 `code=1`……实际是 `connectex` 日志旁带 `[api] panic recovered: 内部密钥泄露: sk-abcdef123456`（后者是 TestRecoverMiddlewareHidesPanicDetail 的同时段日志，与失败无因果） |
| 5 轮连跑 | #1 FAIL `TestLoginActivateSeparateBuckets` 22.4s（`handler_test.go:1481` 报 `dial tcp 127.0.0.1:58842: connectex`——登录页 GET /login 首连接冷启动）；#2-#5 全绿 | 1 FAIL / 5 |
| `-count=2` 连跑 | 全绿 | |
| `-count=3` 连跑（90.2s） | 全绿 | |
| 无 race（`-count=1` ×2） | 全绿 | |
| 两个已知抖动测试定向单跑 ×2（race） | 全绿（0.15s / 0.14s） | |

**触发场景推演：** `newTestDepsModeName` 已做双层缓解（预创建-关闭回环套接字 + 夹具构造期 `readyProbe` 把冷启动窗口前移），但每测试仍新建独立 `httptest.NewServer`，其 accept 就绪与首个测试请求之间在 Windows 回环 TIME_WAIT 队列未排空时仍有低频 `connectex`（`dial tcp 127.0.0.1:xxxxx: connectex` 形态）；失败都落在首批对外连接（`TestLoginActivateSeparateBuckets` 的 doLogin 前的 `fetchLoginPage`、`TestAdminElectiveSelectUnknownAccountRejects` 的 login）。单跑全绿、多轮连跑收敛、根因证据链与 R80 M80-01 / R82 归因（时序敏感测试族 + 全局态：登录频率闸门/限流桶）方向一致——产品逻辑无涉，纯夹具/宿主环境时序。

**修复建议：** 不需要产品代码修改。CI 基线固化为 `go test -race -count=1 -p 1 ./...`（串行）+ 失败重跑一次吸收冷启动残余（R80 已建议、R82 已实施缓解，本轮为延续记录）；若未来追求零残余，可在 `readyProbe` 成功后对首个测试请求再补 50-100ms 静默窗口（把 accept 竞争彻底前移），属测试层可选优化。严重度：OBSERVE（偶发抖动、单跑恒绿、多轮收敛、非确定性产品缺陷，基线口径已固化无误导）。

### O85-02：托盘「退出」关闭整个进程的链路完整但依赖第三方库隐式行为——main 无显式 Shutdown，退出路径未显式关闭监听 socket

**文件 + 行号：** `backend/main.go:176-186`（`srv.ListenAndServe()` 后无 `srv.Shutdown`）;`backend/tray_windows.go:59-60`（`mQuit.ClickedCh → systray.Quit()`）对照依赖库 `systray@v1.2.2` 的 `Quit`（systray.go:110 `quitOnce.Do(quit)`）→ `quit`（systray_windows.go:811 `PostMessageW(wt.window, WM_CLOSE)`）→ wndProc `WM_CLOSE→DestroyWindow`、`WM_DESTROY→defer PostQuitMessage+fallthrough systrayExit`（systray_windows.go:264-277）→ 消息循环 `GetMessageW` 返回 0（WM_QUIT）→ `nativeLoop`（systray_windows.go:781-800）以 `case 0: return` 退出 → `systray.Run` 返回 → `runTray` goroutine 结束 → main 代码全部执行完 → Go runtime 自然退出进程。

**推演：** 实测链路完整：托盘「退出」最终导致整个进程退出（后台 HTTP 服务一并关闭），而非"托盘没了、服务活挂后台"。这是正确的产品语义。两个边界值得记录（均非缺陷）：① 进程退出无 `srv.Shutdown`/`listener.Close()` 显式通路——路由层的优雅终结依赖进程退出由 OS 回收 socket（监听套接字在 Windows 上正确关闭），SQLite 数据安全性由 SQLite 单写者 + 进程退出刷盘保证，无数据丢失风险；② 退出链路依赖 `systray` 内部 `WM_CLOSE→WM_DESTROY→PostQuitMessage` 的隐式消息顺序（第三方库 v1.2.2 语义，乱序时 `nativeLoop` 无人建模 `case 0` 退出），main 无 `signal.Notify`/异常退出兜底——若库更新改变该链路，托盘退出可能静默变活挂。归 OBSERVE（现状正确、仅依赖库演进风险留档）。

**修复建议：** 可选加固：`mQuit` 分支在 `systray.Quit()` 前先 `srv.Close()`（或 main 用 `signal.NotifyContext` 挂兜底）——成本低、与库演进解耦；不作为本轮必须项。

### O85-03：spawnChain「快照满员」分支（`classFullInSnapshot` 命中即 `markFullLocked`）无 `sameClientFor` 身份复核——经分析确认安全，记录边界论证

**文件 + 行号：** `backend/internal/scheduler/scheduler.go:1451-1455`（锁内 `classFullInSnapshot → markFullLocked → continue`）

**推演：** 六条身份防线分支（失效/成功/风控/窗口关闭/实时复核失效/实时复核确证满员）覆盖的都是"网络往返之后"的写点——删号竞态窗口长达 SelectClass 15s，需要指针身份比对防同名重建污染。快照满员分支与其不同：它位于锁内、无网络往返、用快照数据即刻判定。其删除竞态窗口仅限持 `s.mu` 的微秒级临界：即便链 goroutine 恰在 `Accounts.Remove`（含 `ClientFor` 失效）之后、`PurgeAccount` 之前的那一段持锁执行到本分支，`markFullLocked` 写入 `s.full[acct]` 随后被 `PurgeAccount` 的 `delete(s.full, acct)`（scheduler.go:504）整体清除；同名重建后新身份从零开始（PurgeAccount 已清），`SetTargetsForAccount` 也不清 full 但删除+重建路径必经 PurgeAccount，故不存在污染旧链到新身份的通道。结论：该分支无需 `sameClientFor`，与六分支的部署差异是结构性合理的（锁内即刻写 vs 网络后慢写）。记录为"经论证安全"的边界，防后续审查误报。

---

## 可疑待核

- **api 首跑抖动残余（O85-01）是否值得根除**：本实测定性为 Windows 回环 TIME_WAIT 冷启动 + 每测试新建 httptest 的 accept 竞态（connectex 形态 + 定向单跑恒绿 + 多轮收敛）。若主控要在 CI 实现零残余，可在 `readyProbe` 已就绪后再补对首个操作请求的短静默窗口——但当前 `-p 1` + 失败重跑口径已实际零回归。待核项维持（非代码缺陷）。
- **linux-CGO1 交叉编译无法在 Windows 宿主验证**（延续历轮）：`GOOS=linux CGO_ENABLED=1` 缺 `grp.h` 为宿主环境限制，release.yml ubuntu runner 覆盖该形态。

## 已核无缺陷清单

### 1. R84 归档完整性（HEAD 617bd5a）

| 项 | 结论 |
|---|---|
| R84 提交内容 | 通过。`git show --stat 617bd5a` 仅 3 个归档文档（review-round84.md + round84-{backend,frontend}-findings.md），353 行全为文档，零代码改动——与"纯归档轮"声明一致 |
| R84 归档内容质量 | 通过。O84-01 记录 R83 未来 open 优于报告建议、O84-02 目标双落库中间态、O84-03 编号锚点；主控裁决与 findings 一致，收尾回归记录完整 |
| 工作树 | 通过。`git status --short --branch` 为 `## master` 洁净，无未提交变更 |

### 2. build tag 10 文件互斥矩阵 + tray 双回归钉全量走查

| 文件 | tag | 结论 |
|---|---|---|
| `tray_windows.go` | `windows` | Windows 活托盘；ICO 程序内生成 |
| `tray_linux.go` | `linux && cgo` | Linux 桌面托盘（systray+GTK） |
| `tray_linux_cgo0.go` | `linux && !cgo` | Docker 占位（与上互斥） |
| `tray_other.go` | `!windows && !linux` | darwin 等占位 |
| `tray_asset_windows_test.go` | `windows` | ICO 回归钉 |
| `tray_asset_linux_test.go` | `linux && cgo` | PNG 回归钉 |
| `browser_windows.go` / `browser_unix.go` | `windows` / `!windows` | 互补开浏览器 |
| `native_ocr.go` / `native_ocr_stub.go` | `windows && cgo` / `!windows \|\| !cgo` | ddddocr 内嵌 / 存根互斥 |

**R80 硬锚复核**：`tray_asset_windows_test.go:27` 三加数独立推导 `4264`（40+4096+128）、`:32` 硬编码 `22`，与实现 `tray_windows.go:96-97` 总长减头部 4264 / headerSize 22 双源不同式同值——字段算错仍能红；DIB 头双高 64、像素区 BGRA 逐像素断言、pixStart=62 与实现一致。`trayPNG` 经 `image/png.Decode` 严格 CRC 校验。四个回归钉测试在各自平台 build tag 下编译。**四组合交叉编译实测全绿**（见构建验证表）。

### 3. 契约 20 全仓扫描

- 生产代码零轮次前缀标签；唯一行号级引用为 `client.go:405/512/516/537/552` 的标准库源码语义锚（client.go/transfer.go 为 Go 标准库文件）与跨文件语义指位，许可。
- 测试代码命中：`scheduler_test.go:1794/1806/3174/3181/3484/3493/3498`（B29-02/B42-02/B43-02）、`tray_asset_*.go`（R76/R77/R79）、`handler_test.go:152`（O82-01）、`tray_windows.go:93`（R79）——均叙述性历史锚点，非「X-XX（第 N 轮）」前缀标签形态，许可。
- `XK-ABCD-EF12-3456` 激活码样例为假阳性；`internal/api/handler_test.go:1481`「第 3 轮后计数可能已被清零」为描述性中文非标签。
- **结论：零轮次前缀标签残留，符合契约 20。**

### 4. 生产逻辑契约抽核（新角度 8 项）

| 契约 | 结论 |
|---|---|
| handleLogin 登录限流桶独立 | 通过。router.go:91-92 `loginLim`/`activateLim` 各自 `newLoginLimiter()` 独立注册、各自闭包持有；`TestLoginActivateSeparateBuckets` 触发激活桶 429 后登录仍走业务层（`code=1001` 未激活）实测通过 |
| handleActivate 票据消费 + 激活码并发原子 | 通过。`ConsumeTicket`（session/store.go:118-138）单次防重放（used 置位即删）；`ConsumeActivationCode`（store.go:292-323）单条 `UPDATE ... WHERE used_uses < total_uses` 原子防超卖，已激活账号靠 `INSERT OR IGNORE` + 事务内先查 `activations` 不扣次（nAct 判定），事务回滚保证一致性 |
| LoadRefused/RestoreRefused 空库 | 通过。`LoadRefused`（store.go:182-198）空表返回空 map 不报错；恢复序 `RestoreDone → 逐账号 RestoreTargets → LoadRefused+RestoreRefused`（main.go:115-139）与契约 6 一致 |
| SubmitAll 多发布分组 | 通过。submitAll 内 `byPub` 按 PublishID 分组、组内按 Priority 稳定排序（scheduler.go:1353-1360）、组间并发 `spawnChain`（chains 活跃 map 防重复）；链顶与取 client 双处存在性复核 |
| openTimeDetected 全校单值写入 | 通过。probe 写 `["*"]`（scheduler.go:1103）、ProbeForAccount 写 `[acct]`（849），均持锁；`openTimeForLocked` 优先级 acct→"*"，识别过期不截断零值；关窗空快照不删槽（"关闭≠时间消失"） |
| scheduler.Stop 幂等 | 通过。Stop 即 `cancel()`（context.CancelFunc，幂等可重复调用），Start 有 `start` 标志防双启（scheduler.go:662-668） |
| maybeRelogin 指数退避封顶 | 通过。`reloginBackoff`（30s<<(n-1)）`maxReloginFail=5` 封顶 10m；失败计数只随成功分支 `delete(s.reloginFail, acct)` 清零，失败绝不无条件复位 1（1255/1283-1286 注释契约维持） |
| 托盘退出链路（e2e 一致性） | 通过。systray v1.2.2 源码核查：`Quit→WM_CLOSE→WM_DESTROY→PostQuitMessage→nativeLoop case 0→Run 返回→main 自然退出`——托盘退出即整体退出，无活挂后台；边界见 O85-02 |

### 5. 其他续核要点

| 项 | 结论 |
|---|---|
| spawnChain 六分支 `sameClientFor` 全闭合 | 通过。失效(1484)/成功(1516)/风控(1546)/窗口关闭(1566)/实时复核失效(1595)/实时复核确证满员(1630) 全先指针身份比对再写状态；快照满员分支（1453）无复核的结构合理性见 O85-03 |
| 手动路径竞态防线 | 通过。`MarkDone`(1922)/`RemoveDone`(1990) 写回前 `ClientFor` 存在性复核，已删账号静默放弃落库 |
| 窗口关闭三判据单源 | 通过。`windowClosedLocked` 主判据 +10s 裕量 / 时钟失败≥3 带"开放时间已过" / 幽灵 EmptyProbeRuns≥3 同 +10s；StateForAccount 与 WindowClosed() 同源；probe 入账侧 `prevOpened && !opened && len==0 && now.After(open+10s)` 一次快照复用 |
| 身份防线=反思 | 快照满员分支（1451-1455）无 sameClientFor——经分析确认安全（删除竞态窗口仅锁内微秒、PurgeAccount 清 full、同名重建不继承），记入 O85-03 防误报 |
| 凭据表 `accountExists` 四路 | 通过。课程读(245/294)/目标写(453)/手动报名(295)/手动退选(373)/状态读(536) 全覆盖，查无此账号整体拒绝 |

## 契约抽查表

| 契约编号 | 内容 | 结果 |
|---|---|---|
| 1 | 开放时间唯一事实源 beginTimes 自动识别、识别槽不截断零值 | 通过 |
| 2 | WindowClosed 判据单源 windowClosedLocked | 通过 |
| 3 | 关闭≠时间消失契约（空快照不删槽、目标发布元数据持久化） | 通过 |
| 4 | 删账号 memory-first（Remove→PurgeAccount→DeleteAccount→RevokeAccount） | 通过（handler.go:1004-1018 实测顺序一致） |
| 5 | 落库前锁内复核 ClientFor 防线族 | 通过（六分支 + 手动两路 + PurgeAccount 竞态窗口消解） |
| 6 | 重启恢复顺序契约 | 通过（RestoreDone→RestoreTargets→LoadRefused+RestoreRefused，main.go 一致） |
| 7 | ?account= 凭据表校验四路 | 通过 |
| 8 | ElectivesSnapshotFor 回退链 | 通过（目标账号过期专属帧返回 (nil,false) 绝不回退全局） |
| 17 | 落库失败必须记日志 | 通过（全仓扫描零吞错落库点） |
| 18 | 识别引擎热切换同步模板 | 通过（SetRecognizer 写模板 + SetVision 保留当前引擎） |
| 20 | 代码注释严禁轮次前缀标签 | 通过（零残留，仅叙述性历史锚点） |
| 31/36/37 | sameClientFor 全分支闭合 | 通过（六分支独立 + O85-03 快照分支结构论证） |
| 33 | doLogin 全入口全局频率闸门 | 通过（gateTryAcquire 非阻塞收口 LoginByPassword + Relogin 走 gateWait，共享 gateMu/gateUsed） |
| 42/43 | 连接活性自愈 / 撞名学生管理态 | 通过（httpDo 只重试 dial/write；fetchLoginPage 4xx/5xx 容忍一次；登录响应 adminName 标记管理令牌） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，零输出） |
| `gofmt -l .`（backend 全仓） | 零输出（GOFMT_EXIT=0） |
| `go test -race -count=1 -p 1 -timeout 900s ./...` | 全 11 包全绿除 api 首跑 FAIL（O85-01：connectex 冷启动抖动，复跑多轮收敛） |
| `go test -race -count=1 -p 1 ./internal/api/`（failfast 首跑 / count=2 / count=3 / 单跑定向 ×2） | 首跑复现 FAIL → count=2 全绿 → count=3 全绿（90.2s）→ 定向 ×2 全绿；5 轮连跑中 #1 FAIL #2-#5 全绿 |
| `go test ./internal/api/`（无 race ×2） | 双绿 |
| `go test -race -count=1 -p 1 ./internal/scheduler/` ×2 | 双 PASS（15.3s / 15.6s） |
| `go test -race ./internal/store/ ./internal/accounts/ ./internal/session/` | 三 PASS（store 35.5s） |
| `go test -race ./internal/zhidao/` | PASS（2.5s） |
| `GOOS=windows CGO_ENABLED=1 go build` | 通过（内嵌 ddddocr 路径） |
| `GOOS=windows CGO_ENABLED=0 go build` | 通过 |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` | 通过（Linux 无托盘占位） |
| `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build` | 通过（tray_other 占位） |

## 结论

R85 后端只读审查**零 CRITICAL、零 MAJOR、零 MINOR、3 条 OBSERVE**（均为延续/边界观察，无代码修改需求）。核心结论：

1. **R84 归档完整性确认**：HEAD 617bd5a 仅含 3 份归档文档零代码改动，`git status` 洁净，归档内容与主控裁决一致。
2. **build tag 10 文件互斥矩阵 + tray 双回归钉全量走查通过**：R80 硬锚 4264/22 双源不同式同值、回归钉语义方向正确、四组合交叉编译全绿。
3. **api 抖动归因维持**：本轮全仓 race 首跑复现 `TestAdminElectiveSelectUnknownAccountRejects` / `TestLoginActivateSeparateBuckets` 的 connectex 冷启动抖动（证据链与 R80/R82 归因一致），多轮连跑收敛零 FAIL、无 race 双轮全绿、定向单跑恒绿——产品逻辑无涉，属夹具/宿主环境时序（O85-01 延续记录）。
4. **契约 20 扫描零轮次前缀标签残留**；生产逻辑契约新角度 8 项抽核全部通过，含登录限流桶独立、激活票据消费 + 并发原子防超卖、LoadRefused 空库、SubmitAll 多发布分组、Stop 幂等、退避封顶。
5. **新角度确认**：托盘退出链路（WM_CLOSE 链）完整关闭整个进程（O85-02 记录库演进边界）；spawnChain 快照满员分支无需 sameClientFor 的结构合理性与删除竞态窗口消解论证（O85-03）。

后端整体健康状况良好，连续多轮零严重级发现。下轮重点候选：api 抖动的根除级处置（若主控追求 CI 零残余）、systray 库退出链路的主动加固选项。