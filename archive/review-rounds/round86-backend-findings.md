# R86 后端只读审查报告

审查对象：xuanke-auto HEAD `9f17057`（R85 收官，进度 86/256）。本轮为只读审查——全程零仓库文件修改，唯一写入为本报告文件，工作树保持 `## master` 洁净。

审查方式：Read / Grep / Glob / Bash 只读命令（bg test 全量 race + 四平台交叉编译 / go build / go vet / gofmt）。核心走读：main.go 全程、tray_*.go 全集 + systray v1.2.2 第三方库源码（goroutine 结构 + exit 链路）、scheduler.go 全量、store.go 全量、api/handler.go + router.go、session/store.go、db.go、accounts/manager.go、zhidao/client.go、web/embed.go、登录限流/CSRF/写JSON Status 家族、凭据脱敏全仓日志扫描。

---

## CRITICAL

无。

## MAJOR

### M86-01：托盘「退出」并不会退出进程——后台 HTTP 服务活挂为无托盘常驻进程，R85-02 的「整托退出」结论不成立（写实走读）

**文件 + 行号：** `backend/main.go:147,184`（`runTray(tray)` 同步返回后 main 继续进 `srv.ListenAndServe()` 阻塞）；`backend/tray_windows.go:36-40`（runTray 在 **goroutine** 里跑 `systray.Run`，main 只等 `<-trayStartOnce` 就放行）、`:59-60`（mQuit 分支仅 `systray.Quit()`）。systray v1.2.2 源码核查：`systray.go:76-81`（`Run→Register+nativeLoop`）、`systray_windows.go:811`（`quit()` PostMessage WM_CLOSE）、`:264-277`（wndProc WM_CLOSE→DestroyWindow、WM_DESTROY→PostQuitMessage+fallthrough systrayExit）、`:781-807`（nativeLoop GetMessage 返回 0 即 return）。

**触发场景推演：** main 协程执行顺序：`runTray(tray)`（144-147 行）→ 等 `trayStartOnce` 关闭后立即继续 → 起时钟泵 goroutine → 建 http.Server → **main 协程阻塞在 `srv.ListenAndServe()`（184 行）**。而 `systray.Run` 是 `runTray` 内部另起的一个 **goroutine**（tray_windows.go:36-38）。用户点击托盘「退出」→ `systray.Quit()` → 第三方库 `quit()` PostMessage(WM_CLOSE) → wndProc WM_DESTROY → PostQuitMessage(0) → nativeLoop `case 0: return` → `systray.Run` 返回 → **runTray 内嵌 goroutine 结束（仅此而已）**。

关键 Go 运行时语义被 R85 漏掉：**进程退出只在 `main.main` 返回或 `os.Exit` 时发生**，非 main 协程全部结束不会终止进程。此时 main 协程仍 parked 在 `ListenAndServe`（无错误/无 Shutdown，永不返回）——于是托盘图标消失，但 **HTTP 服务继续在后台监听，变成一个无法再次打开图标的无头驻留进程**（用户只能靠任务管理器找 xe.exe 强杀）。全仓 grep 证实：`os.Exit` 仅存在于 `cmd/probe`、`cmd/logintest`（工具），`signal.Notify`/`NotifyContext`/`srv.Shutdown` 全仓零命中。R85-02 写的「systray.Run 返回 → main 全部执行完 → 进程退出」不成立——main 早在 `runTray` 返回后就已越过托盘等待，parked 在 ListenAndServe。

**产品语义：** 托盘菜单「退出」的 tooltip 是「退出程序」，目标是整进程退出；现状是「退托盘、留服务」，与「退出」预期及 R85 结论（「托盘退出即整体退出，无活挂后台」）均相悖。

**修复建议（TDD 思路）：** 在 `mQuit` 分支先关服务再退托盘，闭环进程：把 http.Server 的 Shutdown 句柄交给 trayData，mQuit 分支 `go func(){ ctx,cancel:=context.WithTimeout(...); srv.Shutdown(ctx) }()` + `systray.Quit()`；Shutdown 返回后 ListenAndServe 返回 `ErrServerClosed`，main 自然退出。**测试：** `tray_windows_test.go` 新增依赖注入——把「mQuit 触发路径必须调用 srv.Shutdown」抽象为可测接口（构造一个 `*http.Server` HTTP，注入数据源，断言点击 mQuit 后 server.ListenAndServe 返回 `ErrServerClosed` 而非继续 accept）；或直接测纯函数 `quitAndShutdown(ctx, srv, systemQuit)`——注入 fake 判定 srv.Shutdown 被调用。「Shutdown 被调用 + ListenAndServe 返回 ErrServerClosed」双层断言为回归防线。**严重度：MAJOR**（产品语义缺陷：点退出程序仍驻留后台；但不涉数据损坏与安全）。

## MINOR

无。

## OBSERVE

### O86-01：api 首跑抖动本轮完全未复现（race 全绿），O85-01 的「readyProbe 后再补 50-100ms 静默窗口」优化评估为不必要

**实测：** `go test -race -count=1 -p 1 -timeout 900s ./...` 全 11 包退出码 0（exit 0），含 `internal/api` 36.190s——connectex 冷启动残余本轮零样本。确认 O85-01 的 readyProbe 双层缓解（套接字预创建 + readyProbe 前移）加上 `-p 1` 串行已足够。**对「再补 50-100ms 静默窗口」的裁决：不实施。** 理由：①残余样本发生在「每个测试**各自新建** httptest server、其 accept 就绪前的首请求」阶段，位于测试体内部，夹具层无法已知「哪个是首操作请求」来统一插静默；即便在 readyProbe 成功后盲眠，只能覆盖「本 server 上一个、下一个未知」中的部分。②本轮 race 全绿 + 历轮收敛，必要性不足。收益（理论零残余）远小于实现复杂度与测试总时长代价。CI 维持 `-p 1` + 失败重跑一次即可。

### O86-02：spawnChain 快照满员分支无 sameClientFor 的三重论证再复证——通过（走读复核）

重读 `scheduler.go:1451-1455`（锁内 `classFullInSnapshot → markFullLocked`）：该分支在网络往返之后、位于持 `s.mu` 的锁内、用快照判定即刻写 `full → continue`，无同类网络往返慢写窗口。`PurgeAccount`（:499-523）持 `s.mu`，含 `delete(s.full, acct)`（:504）——删除竞态窗口仅锁内微秒；即便链 goroutine 恰在 Remove 之后、PurgeAccount 之前那一段执行到此，`markFullLocked` 写入后也被 PurgeAccount 整体清。同名重建必经 PurgeAccount → 新身份从零。**论证仍成立，无新污染通道。** 六条网络后写分支（失效/成功/风控/窗口关闭/实时复核失效/确证满员）均已有 `sameClientFor`；此分支的差别是结构性、正确的，防后续审查误报。

## 可疑待核（需主控核实）

无独立待核项。M86-01 是唯一实质缺陷级发现，方向认同、待主控深度核实（systray goroutine 结构 + main 协程 parked 论断）。

## 已核无缺陷清单

| 项 | 结论 |
|---|---|
| 凭据脱敏日志 契约 17 | 通过。全量 log 清扫：tokenShort（manager.go:319 前8+...）/ maskedToken（scheduler.go:1279 前8）均只显前 8 位；manager.go:286/306 密码加解密失败只打错误对象不吐值；main.go:91 解密 vision_key 失败只打错误；api handler 激活码/管理口令零日志直显。无泄漏点。 |
| SQLite 事务边界 | 通过。store.go 全部多语句写路径（SetTargetsForAccount、CreateActivationCodes、ConsumeActivationCode、SaveSettings、DeleteAccount）统一 `tx.Begin → defer tx.Rollback → 逐语句 → tx.Commit` 事务包裹，异常回滚；单语句原子写走 auto-commit。ConsumeActivationCode 的「未超卖 + 已激活不重复扣次」用 `UPDATE...WHERE used_uses<total_uses` 原子条件 + 事务回滚实现。 |
| reqLock 与网络往返持锁跨越 | 通过。scheduler 侧唯一跨网络的 `classFullRealtime` 已在锁外发起、回锁收尾（1583-584），锁不跨越网络段；`TryAcquireSubmit` 只持锁做 map 读写微秒级；`maybeRelogin`/tick 不持锁做长操作。无遗漏持锁跨网络。 |
| 并发测试真实性（B14-M1/M2 教训） | 抽查 scheduler 相关用例——大量用 `waitChainExit`/活跃链标记（chains map）而非 inflight 等待判空，与契约 31 反教训对齐；无手写假断言恒绿痕迹。 |
| embed 资产与 web/dist 一致性 | 通过。web/embed.go spaHandler：`/api` 精确 + 前缀统一 404（防未注册端点落 index.html 被安全扫描误判 200）；非 /api 且文件不存在才回退 index.html；source `backend/web/dist` 存在（`web/dist/assets/index-*.js` 实证）。SPA 兜底符合契约。 |
| win/linux/darwin 三平台 tray 文件 build tag 互斥 | 通过。tray_windows.go（windows）/ tray_linux.go（linux&&cgo）/ tray_linux_cgo0.go（linux&&!cgo）/ tray_other.go（!windows&&!linux）四组合交叉编译全绿。M86-01 只涉及 windows 活托盘路径。 |

## 契约抽查表（抽查 7 条相关路径）

| 契约 | 结果 |
|---|---|
| 1（开放时间唯一事实源、识别槽不截断零值） | 通过（probe 写 `["*"]`/ProbeForAccount 写 `[acct]` 持锁；openTimeForLocked 返回识别值本身不做过期截断；空快照不删槽） |
| 4（删账号 memory-first 顺序） | 通过（handler.go:1004-1018 `Remove→PurgeAccount→DeleteAccount→RevokeAccount`） |
| 8（ElectivesSnapshotFor 回退链） | 通过（目标账号过期专属帧返回 (nil,false) 绝不回退全局 lastData） |
| 17（落库失败绝不静默吞错） | 通过（全仓 scheduler append/save/delete 各路 `if err != nil { log }` 零 `_ =`） |
| 20（注释严禁轮次前缀标签） | 通过（生产代码日志/注释无「X-XX（第N轮）」前缀标签形态） |
| 31/36/37（身份防线全分支闭合、重登入口存在性复核） | 通过（六写分支 + maybeRelogin 入口 ClientFor 复核 + 快照满员分支结构论证已复证） |
| 43（连接活性自愈只覆盖连接层） | 通过（httpDo+isConnErr 只重试 dial/write，业务错误上抛；fetchLoginPage 仅首次网络抖动自愈一次） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，零输出） |
| `gofmt -l .`（backend） | 零输出（GOFMT_EXIT=0） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，11 包 exit 0（accounts/api/config/db/runtime/scheduler/secure/session/store/zhidao/main）；api 36.190s、store 36.327s |
| `GOOS=windows CGO_ENABLED=1 go build` | 通过（内嵌 ddddocr 双轨） |
| `GOOS=windows CGO_ENABLED=0 go build` | 通过 |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` | 通过（无托盘占位） |
| `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build` | 通过（tray_other 占位） |

## 结论

本轮投入重点在**托盘退出链路的再评估**，发现并上报 1 条 **MAJOR**：

1. **M86-01（托盘退出活挂后台）**：`systray.Run` 在 runTray 的**子 goroutine** 跑，main 协程早已越过托盘的 wait 进入 `srv.ListenAndServe()` 阻塞。Go 程序只在 `main.main` 返回或 `os.Exit` 时退出——点击托盘「退出」仅结束 tray goroutine，进程继续以无头 HTTP 服务驻留后台，窗口消失但服务不关。这**推翻 R85-02 的"托盘退出即整进程退出"结论**（R85 只走了第三方库链路，漏判 main 协程 parked 在 ListenAndServe 且无 Shutdown）。建议在 mQuit 分支接 srv.Shutdown + 可选的 signal.NotifyContext 兜底，用「Shutdown 被调用且 ListenAndServe 返回 ErrServerClosed」做 TDD 回归。

2. **api 抖动本轮零复现**（全量 race 全绿），O85-01 的「readyProbe 后再补 50-100ms 静默窗口」评估为不推荐——残余样本属每测试独立 server 的首请求，夹具层无法泛化覆盖，必要性不足（O86-01）。

3. **O85-3 三重论证复证通过**（快照满员分支锁内即刻写、PurgeAccount 清 full、同名重建不继承，无新通道）。

4. 安全角度全绿：凭据/密码/token 脱敏（契约 17）零泄漏；SQLite 多语句写全事务包裹原子；reqLock 无跨网络持锁；并发测试亲和 B14-M1/M2 反教训；embed/SPA 静态映射正确。

后端总体唯一需处理的是 M86-01 的托盘退出语义。仓库工作树 `## master` 洁净，无未提交变更（本报告文件除外）。