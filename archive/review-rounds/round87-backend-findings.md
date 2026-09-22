# R87 后端只读审查报告

审查对象：xuanke-auto HEAD `0cccc35`（R86 收官，进度 87/256；本轮包含托盘退出活挂后台 MAJOR 修复 B86-01 与前端注释修正 F86-01）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round87-backend-findings.md），工作树保持 `## master` 洁净（git status 实证仅本报告 + 前端并列报告的未跟踪新文件，零改动既有文件）。

审查方式：Read / Grep / Glob / Bash 只读命令（bg 全量 race 测试 / go build / go vet / gofmt / Windows 目标交叉编译）；对照 Go 标准库 net/http（本机 Go 1.22+ 源码）`Server.ListenAndServe → Serve → Shutdown` 源码逐行核实 B86-01 的退出链路；走读 systray v1.2.2 Windows 后端 nativeLoop/quit/wndProc 全链路。核心走读：quit_shared.go、main.go 全量、tray_windows.go/tray_linux.go/tray_linux_cgo0.go/tray_other.go、tray_quit_test.go 三回归钉、scheduler.go（tick/probe/probe 内 per-account/spawnChain/maybeRelogin/windowClosedLocked/ElectivesSnapshotFor/ProbeForAccount/手动报名四方法/锁序）、accounts/manager.go（gateTryAcquire/Relogin/SetVision/SetRecognizer/LoginByPassword）、api/handler.go（登录/激活/状态/目标/管理/限流/CSRF/凭据表判据）、api/router.go（路由装配与 404 门）、session/store.go（sweeper/票据）、store.go（事务）、runtime/config.go、web/embed.go（SPA 兜底）。

---

## CRITICAL

无。

## MAJOR

无。

## MINOR

### M87-01（走读推断）：托盘「退出」下 5s Shutdown 超时的**窗口临界竞态**——恰好跨 Shutdown 边界的在飞长请求在服务关闭后返回，spawnChain 在飞链把已开窗目标记为 `failed`、状态与真实选课结果可能分叉

**文件 + 行号：** `backend/main.go:191-197`（`srvShutdown` 5s 超时）；`backend/internal/scheduler/scheduler.go:1002-1008`（spawnChain 提交放行依赖**未来时刻开窗点之外的语义**——SpShutdown 关闭时 tick 已停摆，链靠网络往返回写）；`scheduler.go:1473-1662`（SelectClass 最长 15s，跨 Shutdown 返回后仍会 `setStateLocked(...,"failed",...)` `AppendLog`）。

**触发场景推演（具体输入序列）：**
1. 黄金期已开始（或任意开窗后）、用户在飞报名请求 `SelectClass(t.ClassID)` 已发出（网络往返可达 15s，也可遇 10s 读超时）。
2. 同一时刻用户点托盘「退出」→ `srvShutdown()`：`server.Shutdown(ctx)` 关闭 listener、等 `listenerGroup.Wait()`、随后 `closeIdleConns` 幂等扫描（**不强制中断在飞 conn**，仅等待其自然结束或 ctx 超时）。5s 后 ctx.Err() 触发——**net/http 在超时前不会主动断开在飞请求连接**（源码已实证：`closeIdleConns` 只在 `StateIdle` 条件成立时才 `c.rwc.Close()`，对 StateActive 连接 jen ）
3. `Shutdown` 返回（无论超时与否）→ main 越过 `ListenAndServe` 的 `ErrServerClosed` 检查 → **`main.main` 返回、进程全部 goroutine 被强制终止**——包括仍在网络往返中的 spawnChain 在飞 goroutine。
4. 平台端：该次 `SelectClass` 在 Shutdown 前已被平台处理并成功（返回前 1-2s 已被吃进 `[]byte` 缓冲），但客户端进程已死——**成功落库/state/done/inflight 清位全部丢失**。重启后该课未被 RestoreDone 恢复 → 重新抢课，若已满员则误判「抢失败 / 全满」。
5. 反向（超时前进程尚未退出、请求正好返回）→ 在飞链把该课标 `failed`「教务令牌失效/网络错误」或 `success`，与真实平台状态不一致（选课结果与状态文案分叉）。

注意：这是**数据一致性窗口**，非崩溃/死锁。历史上托盘「仅退图标」的形态（M86-01）下服务不关、链必继续，本竞态不触发；**B86-01 引入的优雅关闭路径首次开放该窗口**。触发概率低（点退出与在飞报名同帧概率）+ 影响面限单课状态，但黄金期冲刺+退出令临界最易命中（黄金期 250ms 冲刺 15s 往返链同时存活）。

**修复建议（供主控评估，非本轮修）：**
- 最小：srvShutdown 前置 `sched.Stop()`（`s.cancel()` 停 tick goroutine，`select` 已挡 reloginResults）——但会**额外引入「停止调度」与「在飞链继续写」的又一层平行约束**，需同步守护在飞链对 `ctx.Done()` 的感知与 in-flight 清位等待，实现成本不低。
- 或维持现状并在日志/文档明确：托盘退出 = 尽力优雅，在飞提交结果以重启后重新探测/手动状态为准（品牌语义「退出程序」本意就是终止，不承诺最后 5s 在飞链的落库原子性）。
- 评估结论：**建议维持现状 + 文档承诺我的触点，不实现 sched.Stop 前置**——spawnChain 10-15s 网络往返在 5s 超时下大概率已被进程终止吸收，若前置 Stop 反而让长期在飞链失去所有回写路径。真实数据风险只存在于「5s 超时前请求**恰好**返回」的超窄窗口，收益（理论消除）远小于复杂性与新竞态引入面。走读推断，无实测冲突；需主控按部署会话节奏最终裁决。

**严重度论证：** MINOR——非崩溃、无安全/一致性破坏，仅在极端时序下丢失最后一次提交结果或产生一帧状态分叉；与 M86-01 修复引入同步正交，且已由「重启恢复」语义兜底（RestoreDone 只恢复已落库的成功，未落库的重新尝试是既有的最终一致性设计）。

## OBSERVE

### O87-01：B86-01 托盘退出闭环已成立；srv.Serve 内 `trackListener` + `listenerGroup` 的 Shutdown 时序与 in-flight 连接的强制中断行为——走读实证 X（防后续审查误判该窗口为 CRITICAL）

**走读证据（标准库源码，实测定轨）：** `net/http/server.go`（本机 Go 版本）：
- `ListenAndServe`→`Serve`→`trackListener(&l,true)`（:3417）→ 若 `s.shuttingDown()` 已置位返回 `ErrServerClosed`（:3418）。
- `Shutdown` 先 `closeListenersLocked()`（:3154）→ 关闭 listener → 阻塞在 `listenerGroup.Wait()`（:3159）等待 Serve 退出 → 之后进入 `closeIdleConns` 幂等循环（:3176）**只关 `StateIdle` 的 conn，不强制中断在飞**（:3213-3221 源码：`st != StateIdle` → 继续置于非静止，等待 ctx 超时或自然结束）。
- 结论：**5s 超时前在飞请求不会被 net/http 强杀**；超时后进程退出由 `main.main` 返回（无 goroutine 等待）完成。B86-01 的「先 `srv.Shutdown` 再 `systray.Quit`」顺序在语义上完整：无论 5s 内是否处理完在飞，`ListenAndServe` 都已在 listener 关闭时刻返回 `ErrServerClosed`，main 的解锁不依赖 5s 超时是否等待完毕——**该窗口不会挂起主线程**（`srvShutdown` 在 mQuit goroutine 里阻塞，main 只等 ListenAndServe 返回）。

### O87-02：`Quit` 链回调确认——`systray.Quit()` 的 `systrayExit` 空回调 + nativeLoop 退出在 Shutdown 之后的时序（防误读）

systray v1.2.2 实证：`nativeLoop` 在 `systray.Quit()`（`quitOnce.Do(quit)`）触发 `wmSystrayMessage`/`WM_CLOSE` → `WM_DESTROY` → `PostQuitMessage(0)` → nativeLoop 收到 `WM_QUIT`（case 0）return（源码 :799-803）；`systray.Quit` 只结束该 goroutine，进程退出仍由 `main.main` 返回驱动。B86-01 把 `systray.Quit` 放在 `srvShutdown` 之后，避免「图标先没、服务活着」的窗口——时序正确（mQuit goroutine 内部顺序保证）。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| M87-01 窗口 | 黄金期冲刺+在飞报名+托盘退出的并发时序，需主控按部署节奏裁决是否接受现状（见上「修复建议」）。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **B86-01 托盘退出闭环**（核心）：`quitApplication` 先 `shutdownServer` 后 `systrayQuit`；`srv.Shutdown` 返回 → `ListenAndServe` 返回 `ErrServerClosed` → main 解锁 → 进程整体退出；`setExitActions(nil, systray.Quit)` 语义（判 nil 不覆盖）与无托盘平台（linux cgo0/darwin 占位）恒 nil 跳过的 nil-safe 行为 | 实测通过（tray_quit_test.go 三钉 + 标准库源码走读） |
| **tray 三/四文件 build tag 互斥**：windows / linux&&cgo / linux&&!cgo / !windows&&!linux | 通过（CGO=1 win build + 无托盘链路交叉编译） |
| **O86-01 api 抖动基线延续**：`go test -race -count=1 -p 1 -timeout 900s ./...` 全 11 包 exit 0（api 22.150s / store 31.654s），connectex 零样本 | 通过 |
| **O86-02 spawnChain 快照满员分支三重论证复证**（scheduler.go:1451-1455 + PurgeAccount :499-523）：锁内即刻写、无网络往返慢写窗口、删除竞态仅锁内微秒、PurgeAccount 清 full 故同名重建不继承 | 通过 |
| **srv.Shutdown 与在飞 spawnChain/手动报名交互**（新增）：Shutdown 只断 listener + 关 idle conn，不强杀在飞；进程全部 goroutine 的强制终止由 main 返回承担，状态一致性窗口见 M87-01 | 走读通过（net/http 源码实证），窗口已单独上报 |
| **defer 链完整性**：main 中 `defer d.Close()` / `defer sessions.Close()` 在正常返回与 Fatal 路径均执行；Shutdown 路径返回前两个 defer 均跑 | 通过 |
| **sessions sweeper 退出**：`Close()` 用 `sweeperClose sync.Once` + 等待 `sweeperDone`，正常与我们 Route 路径均不泄漏 | 通过 |
| **登录/激活限流桶并发**：loginLimiter/activateLimiter 各 `sync.Mutex` 守卫，`allow` 全程持锁（含 GC）；XFF 透传契约（localhost 才信） | 通过 |
| **runtime.Store 热重载并发**：`Get()`/`Update()` 读写锁操作统一；Update 闭包在线程持有写锁时执行（无重入——闭包内不带锁调用）；`dispatchRuntimeConfig` 在 Update 之外另行调用，无嵌套锁 | 通过 |
| **recoverMiddleware 覆盖全部注册端点**：`return recoverMiddleware(securityHeaders(mux))` 对所有 method+pattern 注册端点统一包裹；未知 /api/ 路径显式 404 但仍在 recover 之内 | 通过 |

## 契约抽查表（抽查 8 条相关路径，逐条验）

| 契约 | 结果 |
|---|---|
| **17（落库失败绝不静默吞错）** | 通过（scheduler/api/session 全仓 `if err != nil { log.Printf }` 零 `_ =`；store AppendLog/SaveSuccess/DeleteRefused/DeleteSuccess 全持有） |
| **5（落库前锁内复核 ClientFor 防线族）** | 通过（spawnChain 六写分支 + maybeRelogin 入口 + MarkDone/RemoveDone 三处存在性复核 + sameClientFor 指针身份全闭合） |
| **36/37/31（maybeRelogin 入口复核、身份防线分支闭合、purge 竞态族）** | 通过（maybeRelogin:1203 ClientFor 复核 + 六分支 sameClientFor + 快照满员分支结构性论证已复证） |
| **B42-01（doLogin 全入口收口全局频率闸门）** | 通过（LoginByPassword:244 gateTryAcquire 非阻塞准入与 gateWait 共享 gateMu/gateUsed；Relogin:173 gateWait 阻塞收敛；manager.go:223-235 单窗口语义无死锁——先 Lock 再查/消耗） |
| **43（连接活性自愈只覆盖连接层）** | 本次未改，走读持平（httpDo+isConnErr 只重试 dial/write 一次，业务/取消错误上抛） |
| **B39-02 / B40-01（writeJSONStatus 家族只改基础设施路径、状态码成族核对）** | 通过（panic 500 / 会话 401 / 管理 403 / 限流 429 / CSRF-403 / 未知 API 404 均 writeJSONStatus；业务路径仍走 writeJSON 恒 200；router.go 登录/激活 403 + 429 实测路径一致） |
| **F43-M1（保存链「用户意图 vs 数据缺席」）** | 前端契约，本轮走读 touch 到 handler 目标保存（handleSetTargets 空数组放行语义），后端视角通过 |
| **delete 账号 memory-first 顺序 + refuseLegacy 迁移** | 通过（handleAdminDeleteAccount 顺序 Remove→PurgeAccount→DeleteAccount→RevokeAccount；db.go migrateAddPublishMeta 迁移列判存在 + refuseLegacy 已剔除迁移列） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `CD backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，11 包 exit 0（api 22.150s / store 31.654s / scheduler 15.433s） |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 零输出 |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build` | 通过（内嵌 ddddocr 双轨 + 托盘路径） |

## 结论

1. **B86-01 本轮核心复核成立**：托盘「退出」先优雅关服务再退图标、`ListenAndServe` 返回 `ErrServerClosed`、main 解锁整进程退出——标准库源码逐行确认闭环，三回归钉全绿。无 CRITICAL/MAJOR 新缺陷。
2. **新增 M87-01（MINOR，走读推断）**：5s Shutdown 超时与在飞 spawnChain（最长 15s）的**窗口临界竞态**——超时前请求恰好返回时在飞链写「failed」，与真实平台结果可能分叉；超时后进程强杀则最后提交的落库丢失（重启重抢是既有兜底）。B86-01 首次开放该窗口（旧形态托盘仅退图标服务活挂、链必继续，无此窗口）。建议维持现状 + 文档明确「尽力优雅」语义，不实现 sched.Stop 前置（引新竞态收益趋零）。
3. **O86-01 api 抖动本轮零复现**（race 全绿、connectex 零样本）；**O86-02 三重论证复证通过**。
4. 新角度扫查全绿：defer 链完整性、sessions sweeper 无泄漏、限流桶并发安全、runtime.Store 无重入、recoverMiddleware 覆盖全部注册端点、/api 404 门正确、spawnChain 快照满员分支结构性正确。

工作树 `## master` 洁净（git status 仅本报告与前端并列报告未跟踪，零改动既有文件）。