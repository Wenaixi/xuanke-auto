# round49 后端审查原始发现

> 审查基线：master @ `cae3735`（R48 收敛落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动；审查期间工作区零改动，绝对只读，未修改/创建/删除任何文件——唯一新文件是本报告）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 + legacy/website-source 逆向契约 + round39~48 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 上轮观察项逐一核实 + 本轮新视角扫查（chains map / rateLimited 交叉 / gateWait-gateTryAcquire 窗口翻转 / task_log 写入路径 / 大响应解析 / 优雅退出）。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 连跑 4 轮——第 1 轮 FAIL（api）、第 2 轮全绿、第 3 轮 FAIL（api 2 测 + zhidao）、第 4 轮 FAIL（api 2 测）。失败测试单跑/子集复跑全绿（api 单跑 15.8s 绿、zhidao 单跑 3.4s 绿、scheduler+accounts+store 组合绿、F46 钉子集 5 测组合绿、身份族 7 测在库）。详见 MAJOR 节。

---

## CRITICAL

（无本轮新增 CRITICAL。identity 复核三族（决策侧 B43-01 / 写回侧 B21-03 / 失效归并 B41-01）在调度器全路径仍闭合；删号内存优先四步序（Remove→PurgeAccount→DeleteAccount→RevokeAccount）无新裸露写点；本轮新视角 chains map / rateLimited 交叉 / gate 窗口翻转均确认无数据污染通道。）

---

## MAJOR

### MAJOR-49-01：F48-M1 `-p 1` 单包串行未根除 httptest 连接 flake——CI 假红源头延续，`-p 1` 目标落空

- **位置**：`backend/internal/api/handler_test.go`（TestAdminDeleteAccountNoBodyOK / TestAdminStatsAccountsLogs / TestAdminDeleteAccountMemoryFirst / TestStudentSetTargetsWithoutAccountOK 等）、`backend/internal/zhidao/captcha_test.go`（TestCaptchaConcurrency）
- **一句话问题**：R48 把 CI 从 `-p 2` 降到 `-p 1`（0b116a7）目标是"根除 Windows httptest flake 残余假红"。本轮实测 `-p 1` 单包串行下 flake 依然存在：4 轮全量 `-race -p 1` 连跑 3 轮 FAIL，失败全部为 httptest mock 的 `dial tcp 127.0.0.1:<port>: connectex: A connection attempt failed`（登录链路首次请求连接建立失败），失败测试单独复跑全部绿。**关键新增实证：`-p 1` 甚至不能消除 api 包内部测试间相互干扰——单包内部 20+ 个测试各自建 httptest server 并在同一进程内并发轮转时，Windows 宿主连接队列仍偶发拒绝 127.0.0.1 回环连接**。失败断言全部指向"mock server 请求未完成/超时"，无一业务断言失败、无 -race 报告。
- **归因**：`-p 1` 消除的是"包间并行"，但**不消除 api 包内部 20+ 测试串行执行时各自 `httptest.NewServer` 的端口/连接队列轮转**——每测试一个 server，Close 后 Windows TIME_WAIT/连接队列尚未彻底释放，下一测试立即在相近端口建新 server，首请求 `c.http.Do`（login 链路 15s 超时）偶发连接建立被拒。这与 R46 归因（MAJOR-46-01 同源）一致，`-p` 档位只能缓解包间并发、无法根除"单包内部 test 级连接 churn"。CI 上 Ubuntu 不触发（Linux 连接队列语义差异），Windows 本地持续假红。
- **裁决**：MAJOR（测试基础设施层）延续并升级确认——**F48-M1 的"根除假红"目标未达成**。建议下一步（修复代理可评估）：① 给登录链路测试的首次 `client.Do` 加重试（httptest 连接建立失败重试 1~2 次，Windows 回环 flake 本质是瞬时队列争抢，重试即绕过）；② 或 api 包测试用 `httptest.NewUnstartedServer` + 显式端口复用/延迟 Close 让连接池冷态平滑；③ 或接受 flake 配 CI 重跑机制。**业务代码零改动需求**。

---

## 本轮重点核对（上轮新契约，防回归）—— 3 条全部闭合，1 条未达成（见 MAJOR）

### 1. F48-O3 config 家族整风 —— 正确闭合，无回归
- **位置**：`backend/internal/api/handler.go:807-810`（落库失败 `writeJSONStatus(w, http.StatusInternalServerError, 500, nil, ...)`）+ handler_test.go:553-586（TestAdminConfigSaveFailStillDispatch）
- **核实**：① 落库失败分支现写真实 HTTP 500 + body code=500 双通道，与 B43-05 handleAdminStats 目标数失败（handler.go:908）家族统一；前端契约只读 body code，行为不变。② **测试断言已更新**：`if code != 500 { t.Fatalf(...) }`（handler_test.go:562-564）——上轮观察 47-02 的"500 分支无真断言"在此路径已被闭合（注入 `saveSettingsErrForTest` 恒失败 → 断言 HTTP 500 + body 500 + msg 含"落库失败" + 内存已生效 + 下游 Vision 热下发未跳过）。③ **其余路径不受影响**：成功路径仍 `writeJSON(w, 0, ...)` HTTP 200（handler.go:822-829）；无变更 PUT 仍 `writeJSON(w, 1, ...)` HTTP 200（816）；值域校验拒绝仍在 Runtime.Update 之前（749-755），`config_validation_test.go` 两测（非法引擎/越界并发）断言 code=1 + 运行时不被污染——本轮复跑绿。④ **唯一残余**：B43-05 的 `failingTargetsStore`（handler_test.go:829-836）定义后仍无消费点，TestAdminStatsTargetsLoadFailureReturns500 仍只测正常路径（注释自述超范围），该 500 分支仍无真红绿钉——见 OBSERVE-47-02 延续。

### 2. F48-M1 CI `-p 1` 语法正确、只需 ci.yml —— 但目标未达成（见 MAJOR-49-01）
- `.github/workflows/ci.yml:55-58`（`go test -p 1 -v ./...`）YAML 语法正确、注释与命令分隔合法；release.yml 各步骤只有 go build 无 go test，无需降并行——修改范围正确。**但 `-p 1` 未根除 Windows 假红**（本轮 4 轮全量 3 FAIL），"根除残余假红"的修复目标落空，见 MAJOR-49-01。

### 3. B45/B43/B44 + F46-O1 上轮实修 —— 全部复核正确闭合无回归
- B45-N3 加密失败回归钉 `TestLoginByPasswordEncryptFailLogs`（manager_test.go:240-269）真红绿；Restore 解密失败日志（manager.go:295-317）对称保留。本轮 accounts 包回归绿。
- B43-01 maybeRelogin 入口复核（scheduler.go:1204）仍在锁内、位于全部 relogin 族 map 写入前；探测定时三处（ProbeForAccount 821 / ProbeNow 944 / probe 1083）+ 手动 MaybeRelogin 全落统一入口。`TestMaybeReloginDeletedAccountSkipsMaps`（3387）在库绿。
- B43-02 实时复核失效分支 sameClientFor（1596）+ B41-01 err 归并三路（风控 1547 / 窗口关闭 1567 / 实时复核满员 1631）+ B43-03 成功分支清 inflight（1486）+ B39-01 成功/失效分支（1517/1485）——身份六分支全闭合。身份族 7 测（TestDeletedAccountRebuiltSameNameChain* 5 个 + RealtimeUnauthorizedDropsRelogin + DropsSuccess）全在库。
- F46-O1 interval clamp（scheduler.go:237-239）赋值前 clamp、`TestScheduleIntervalClamped` 真红绿；F46 钉子集 5 测（Clamped / WindowOpenSubmitsWithoutProbeReset / SubmitSuspendedWhenOpenTimeCleared / SubmitAllowedWhenWindowOpenedWithZeroOpenTime + api 的 TestAdminStatsWindowOpenedUsesScheduler）组合复跑绿。

---

## MINOR

（无本轮新增 MINOR。）

---

## OBSERVE

### OBSERVE-49-01：main 优雅退出无信号处理——SIGTERM/SIGINT 时 sched.Stop 与 sessions.Close 均不执行（新视角）
- **位置**：`backend/main.go:140`（sched.Start 无配套 Stop）、`:153`（`defer sessions.Close()`）、`:44`（`defer d.Close()`）
- **说明**：全仓 grep 无 `signal.Notify`/`signal.NotifyContext`/`os.Interrupt`。进程被 kill -TERM / Ctrl+C（Windows 控制台 Ctrl+C 默认 TerminateProcess）时：① 调度器 tick 循环靠 `s.ctx.Done()` 退出，但 `s.cancel()` 无人调用——不过进程退出本身会回收 goroutine，调度器无持久化副作用（所有落库点都是单条 SQL 事务、非批量跨条，进程中途退出不产生半事务），窗口判定/探测节流等内存态随进程消失，无遗留。② `sessions.Close()`（停清扫协程）与 `d.Close()`（SQLite WAL checkpoint）是 defer——Windows 下被 kill 时 defer 不执行，WAL 文件不 checkpoint、清扫协程不优雅收尾。**影响评估**：SQLite WAL 模式下进程被 kill 后 WAL 文件由下次打开自动恢复（SQLite crash-safe），`-wal` 文件残留由 modernc 打开时重放；会话表在内存、无持久化，无数据丢失。③ 唯一实质缺口：**SIGTERM 时 scheduler 不停止在飞行中的 spawnChain**（SelectClass 网络往返最坏 15s），进程 kill 后该请求可能已到平台但本地未落 success——重启后 RestoreDone 缺失，该课被重新提交（重报通常幂等：平台会返回"已报名"或 code=1）。属"进程终止语义"的边缘留档，非数据损坏。
- **裁决**：观察级。若未来做公网生产化（systemd/Windows Service 托管需要优雅停机），补 `signal.NotifyContext` + `sched.Stop()` + `sessions.Close()` 即可；当前 Windows 双击 exe + 任务管理器杀进程场景无实质危害。

### OBSERVE-49-02：chains map 溢出/泄漏核查——无泄漏、无溢出（新视角，确认无问题）
- **位置**：`scheduler.go:194-195`（chainMu/chains）、`:1387-1400`（spawnChain 入口置位 + defer 删除）
- **核实**：chains key = `acct+"\x00"+publishID`，生命周期严格配对——入口 `chainMu.Lock()` 判存在→置位→Unlock；goroutine `defer` 内 `chainMu.Lock(); delete(chains, key); Unlock()`（1396-1400）**保证任意 return 路径（成功/静默放弃/失败/网络段）都执行删除**。已删账号链顶 return（1384-1386 / 1405-1406）同样走 defer 删。PurgeAccount 不删 chains（链自身 defer 负责），且链顶身份复核静默放弃后 defer 必执行——无泄漏。溢出：chains 大小 = 活跃链数 = 并发 spawnChain 上限 = 账号数 × 发布数，同时被 tick 节流（1s 常态 / 黄金期 250ms）与 byPub 分组约束，峰值为账号×发布（百级），每链生命周期 ≤ 一次网络往返（15s 封顶）——有界无溢出。waitChainExit（scheduler_test.go:3091）依赖此 defer 语义，B41 身份族 7 测全绿实证。**结论：无问题。**

### OBSERVE-49-03：api 层 rateLimited 退避 map 与 done/full 交叉——手动重报成功后 rateLimited 已正确清除（新视角，确认无问题）
- **位置**：`scheduler.go:1941-1943`（MarkDone 内 `if s.rateLimited[acct] != nil { delete(s.rateLimited[acct], classID) }`）、`TryAcquireSubmit`（1881-1899）
- **核实**：手动报名成功 → MarkDone 依次清 refused / inflight / **full / rateLimited**（1935-1943）——手动重报成功后风控退避绝不残留，自动链下 tick 可立即重打（防"手动成功但退避残留让黄金期静默跳过"）。手动退选 → RemoveDone 同样清 full（1990-1992）+ refused 置位（自动链不抢回）。SetTargetsForAccount 重设目标**刻意保留** rateLimited（B19-02 定案 + TestSetTargetsPreservesRateLimited 在库 2319-2321 断言"重设目标必须保留 rateLimited"）；PurgeAccount 全清（506）。**边界核查**：TryAcquireSubmit 手动路径与 spawnChain 自动路径共享 inflight 位互斥；手动成功后 MarkDone 清 rateLimited 与自动链 `isRateLimitedLocked`（1676-1690）读侧同一把 s.mu——无读写撕裂。唯一残余语义：手动**失败**（平台仍风控中）不清 rateLimited——30s 退避到期后自动链恢复重打，正确。**结论：无问题。**

### OBSERVE-49-04：accounts 的 gateWait 与 gateTryAcquire 在窗口翻转瞬间的计数竞争（新视角，确认无问题）
- **位置**：`manager.go:49-64`（gateWait）/ `:223-235`（gateTryAcquire）/ `:68-77`（GatePump）
- **核实**：两函数共享同一 `gateMu` 与 `gateUsed` 计数，均在 Lock 内先判窗口是否过一分钟（`time.Since(m.gateWindow) >= time.Minute` → 重置窗口 + 归零）再消耗预算——**窗口翻转瞬间不存在"两个 goroutine 各自读到旧窗口、双双重置"的竞态**：重置发生在持有 gateMu 的临界区内，第二次进入的 goroutine 会看到已重置的新窗口与已归零的计数。GatePump 每 30s 检查+Broadcast 唤醒 gateCond.Wait 等待者（manager.go:62），等待者在 Broadcast 后重新竞争 budget（Lock→重新判窗→消耗）——与 gateTryAcquire 的抢占天然串行化在 gateMu 下。边界：gateTryAcquire 在窗口翻转瞬间可能比 gateWait 等待者先抢到预算（手动登录优先），这是"非阻塞准入优先于阻塞排队"的刻意语义（B42-01：手动登录不被排队挂起）。**结论：无计数竞态、无超发（每窗口 ≤ gateLoginPerMin=2 次 doLogin 由 gateMu 串行保证）。**

### OBSERVE-49-05：store 的 task_log 写入路径——非满员失败每 tick 一行，黄金期日志风暴确证（历轮观察延续，本轮补充量化）
- **位置**：`store.go:200-209`（AppendLog 无条件 INSERT）+ scheduler.go spawnChain 非满员失败分支（1646-1650）等 7 处失败写点
- **说明**：黄金期 10 秒冲刺提交间隔 250ms（tick 拍 300ms），非满员失败路径每 tick 一条 AppendLog（"账号 X: <err>"，result 含平台错误原文）。量化：单账号单发布失败重试期 ≈ 10s/0.3s ≈ 33 行/发布·黄金期；若平台熔断风控文案未命中 isRateLimitError（如"无效的课程ID"归类 isWindowClosedError 走 markFull 不再写），其余失败文案每 tick 一行。DB 侧：SQLite 单连接串行 + WAL，每行 INSERT 一次 WAL 落盘。长运行（数月、反复开窗批次）后 task_log 无上限增长（schema 无唯一键/无 TTL，全仓 grep 无 DELETE FROM task_log）。**读侧**：`ORDER BY id DESC`（rowid 主键倒序）走索引只扫 N 行，无读侧性能问题（R47 已修正此说辞）。**唯一实质问题仍是写侧无限增长**——与 R47-01/R46-03/o39-05 同源，无恶化、无清理路径。
- **裁决**：延续观察。建议 AppendLog 侧周期性 `DELETE FROM task_log WHERE id < (max_id - N)`（可挂 GatePump 同款后台协程）。

### OBSERVE-49-06：zhidao 的 selectElectivesData 大响应解析——内存峰值与字段缺省容错（新视角，确认无问题）
- **位置**：`client.go:557-599`（parseElectives）
- **核实**：① **内存峰值**：`doRequest` 先 `io.ReadAll` 整包入内存（413），findElectivesData 82 门课级响应约数百 KB 级；parseElectives 再 `json.Unmarshal` 第二份内存拷贝，峰值 ≈ 2× 响应体（数百 KB），对桌面部署微不足道；`acctData` 每账号常驻 1 帧（ProbeForAccount 覆写旧指针 GC 释放，R47 已确认内存有界）。② **字段缺省容错**：`Class`/`Publish` 结构体全部非指针字段，JSON 缺省键 → Go 零值（int=0/bool=false/string=""）——平台缺发 begin_date/teacher_name_list 等时零值不 panic；`raw.Code == 0 && len(raw.SelectElectivesData)==0` 空快照提前返回空 ElectivesData（581-583）；`json.Unmarshal` 失败返回 error 上层走学期列表兜底重试（FindElectives 524-549）。③ **btm 容错**：`BeginTimes []int64` 缺省 nil，len>0 判定（848/1102）天然跳过识别槽写入。**结论：无问题。**

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-48-01 httptest flake `-p 2` 残余 | MAJOR | **升级确认**：`-p 1` 仍 4 轮 3 FAIL（见 MAJOR-49-01）——F48-M1 目标落空。 | **升级为 MAJOR-49-01** |
| OBSERVE-48-01 submitAll 黄金期 300ms 拍 | 观察 | 本轮复核 submitIntervalFor（scheduler.go:290-295）黄金期 250ms 节流 + tick 300ms 拍，首提交延迟 ≤300ms；spawnChain 链内串行不受节流。非缺陷。 | 延续观察 |
| OBSERVE-48-02 reloginResults cap 8 满丢弃 | 观察 | 通道 cap 8（264），非阻塞发送 select default（1274-1277），tick 主循环消费（673-686）重置 lastProbe。满丢弃仅在并发重登爆炸（>8）时，刻意权衡。 | 延续观察 |
| OBSERVE-48-03 writeJSON 家族 handleAdminConfig 偏差点 | 观察 | **已被 F48-O3 闭合**：落库失败改走 writeJSONStatus(500)（handler.go:810），家族统一；测试断言已更新（TestAdminConfigSaveFailStillDispatch 断言 HTTP 500）。 | 已闭合 |
| OBSERVE-48-04 accounts.ensure 并发 | 观察 | 全程持 m.mu 双检+append order，同名并发登录不重复 order；cookie map 逐客户端实例隔离无串号。 | 延续观察 |
| OBSERVE-48-05 SaveSettings 全量写并发 | 观察 | 单事务 DELETE+INSERT 全量原子，并发 PUT 后写覆盖前写，无字段丢失。 | 延续观察 |
| OBSERVE-47-01 task_log 无清理 | 观察 | 全仓 grep 无 DELETE FROM task_log；AppendLog 无条件 INSERT 7+ 写点；读侧 ORDER BY id DESC 走主键索引 O(N)；写侧无限增长无清理路径（本轮补充黄金期日志风暴量化见 49-05）。 | 延续观察 |
| OBSERVE-47-02 B43-05 500 分支无真断言 | 观察 | `failingTargetsStore`（handler_test.go:829-836）定义后仍无消费点；TestAdminStatsTargetsLoadFailureReturns500 仍只测正常路径。B43-05 的 500 语义靠实现复查背书。F48-O3 的 500 分支（TestAdminConfigSaveFailStillDispatch）已有真断言。 | 延续观察 |
| OBSERVE-47-03 XUANKE_PORT 无校验 | 观察 | config.go:58 无数字/范围校验；`:abc` 时 ListenAndServe 报 `lookup tcp/abc: unknown port` log.Fatalf 拒绝启动——失败形态清晰非静默。XUANKE_MASTER_KEY 长度严格校验（secure/crypto.go:16-21）。 | 延续观察 |
| OBSERVE-47-04 孤儿登录 | 观察 | Manager.Relogin（manager.go:166-175）锁内取指针、锁外 gateWait+ReloginIfNeeded；删号+在途重登并发时平台侧一次孤儿登录。spawnChain classFullRealtime 同构窗口在 sameClientFor 归并路径后已封写回。频率极低无数据污染。 | 延续观察 |
| OBSERVE-46-01 interval clamp | 已闭合 | F46-O1 赋值前 clamp + TestScheduleIntervalClamped 真红绿；本轮钉子集复跑绿。 | 已闭合 |
| OBSERVE-46-02 Retry-After 不消费 | 观察 | doRequest 只解 body code；isRateLimitError 文案匹配 30s 固定退避；平台契约实证 body 文案无 429 头样本；防御性未来缺口。 | 延续观察 |
| OBSERVE-46-04 目标不校验重复/跨年级 | 观察 | byPub 分组+spawnChain 首成功即止；CheckClassSelectable 快照复核+平台最终把关；有意边界。 | 延续观察 |
| OBSERVE-45-01 access_limit_cookie 占位 | 观察 | probe/main.go:24 与 submitLogin（client.go:370-371）同款 `***REMOVED***`；Restore 用 `"1"`；token 权威通道 URL 参数。 | 延续观察 |
| OBSERVE-43-01 probe 双槽分叉 | 观察 | 主体写 `["*"]`（1104）、per-account 写 `[acct]`（850）；openTimeForLocked 先 [acct] 后 ["*"]；全校单值契约不实际分叉。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 网络段重复 | 观察 | IsClassFull 恒 false（maxCount 平台未下发，client.go:696-704 注释实证），快照判满主路径先生效；实时复核仅兜底。 | 延续观察 |
| OBSERVE-43-04 tick 无 recover | 观察 | tick/probe/submitAll/spawnChain goroutine 均无 recover；interval clamp 已堵最可达 panic 路径（F46-O1）；残余概率极低。 | 延续观察 |
| MINOR-43-02 syncFailedWindow 死字段 | 观察 | 写（382/399）不读，注释载明"留档语义"，判据用 syncFailStreak。 | 延续观察 |
| MINOR-43-03 登录失败无固定延迟 | 观察 | 学生走教务网络往返天然延迟；管理员错误口令分支 Sleep(loginTimingFlat) 保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释 | 观察 | 调用侧先 reloginFail++ 再传，n=1 返 30s、n=2 返 60s；注释一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录 | 观察 | 见 47-04。 | 延续观察 |
| M40-01 本地钟混用 | 观察 | maybeRelogin 退避（1209/1216/1220）同基内部自洽；≤640ms 对 30s 节流无实质危害。 | 延续观察 |
| m40-02 锁内多取 now | 观察 | submitAll 1437 锁内取一次 nowAlignedLocked。 | 延续观察 |
| o40-01~04 / m39-02 / o39-02~05 | 观察 | 未激活不发会话 / reloginResults 满丢弃（48-02）/ columnExists 全常量 / interval≤0 已 clamp（F46-O1）/ 快照 TTL 读侧本地钟 / submitAll 快照竞态 / vision key 空串无法清空 / NAT 合并登录限流 / tick 无 recover / task_log 无清理（47-01）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **chains map（活跃链标记）**：生命周期严格配对（入口置位 + defer 删除），任意 return 路径（含已删账号链顶静默放弃、网络段、身份复核失败）都执行 defer 删除——无泄漏；容量上界 = 账号数×发布数（百级），每链 ≤15s 网络往返——无溢出。waitChainExit 测试辅助依赖此 defer 语义，身份族 7 测全绿实证。见 OBSERVE-49-02。
- **rateLimited 与 done/full 交叉**：手动重报成功 MarkDone 清 rateLimited+full（1941-1943），绝不残留假退避；SetTargetsForAccount 重设目标刻意保留（B19-02 + TestSetTargetsPreservesRateLimited 钉）；PurgeAccount 全清。读侧 isRateLimitedLocked 与写侧同锁无撕裂。见 OBSERVE-49-03。
- **gateWait 与 gateTryAcquire 窗口翻转**：共享 gateMu 串行，窗口重置在锁内临界区，不存在双 goroutine 各自重置的计数竞态；每窗口 doLogin ≤2 次由锁保证。见 OBSERVE-49-04。
- **task_log 黄金期日志风暴**：非满员失败每 tick 一行（7 处失败写点），量化 ≈ 33 行/发布·黄金期 10s；写侧无限增长无清理。见 OBSERVE-49-05。
- **selectElectivesData 大响应解析**：2× 响应体峰值内存（数百 KB）+ 全字段零值缺省容错 + 空快照提前返回 + 解析失败兜底重试，无问题。见 OBSERVE-49-06。
- **main 优雅退出**：无 signal.Notify，SIGTERM/SIGINT 时 sched.Stop 与 sessions.Close 不执行；SQLite WAL crash-safe 恢复 + 会话内存态无持久化 → 无数据损坏，仅在飞行 spawnChain 的 success 可能未落库（重报幂等）。见 OBSERVE-49-01。
- **cmd 工具复核**：probe 读 XUANKE_PROBE_TOKEN（无硬编码 token）；logintest 跟随识别引擎（vision/ddddocr 三档回退）+ wait 默认 40s 符合平台限流；bench -token 可选注入（无 token 测 401 路径）。均只读、无副作用、不写库。
- **native_ocr build-tag 双轨**：native_ocr.go（windows && cgo）与 native_ocr_stub.go（!windows || !cgo）互斥完整，NativeDdddOcrAvailable 在三平台下正确回退；assets 三件套 //go:embed 齐全（dumpIfDiff 大小比对释出）。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查/测试**：`go build ./...`、`go vet ./...` 双通道 exit=0。全量 `-race -p 1` 4 轮 3 FAIL（MAJOR-49-01，全为 httptest connectex/超时，无业务断言失败无 race 报告）；失败测试单跑/子集复跑全绿：api 单跑 15.8s 绿、zhidao 单跑 3.4s 绿、scheduler+accounts+store 组合绿、F46 钉子集 5 测组合绿、身份族 7 测在库。
- **identity 复核族全闭合**：spawnChain 六分支（成功 1517 / 失效 1485 / 风控 1547 / 窗口关闭 1567 / 实时复核满员 1631 / 实时复核失效 1596）全部 sameClientFor 前置；maybeRelogin 入口决策侧（1204）+ 重登写回侧（1250）+ 手动路径 MarkDone/RemoveDone 写回侧（1912/1980）闭合。
- **WindowClosed 三判据单源** windowClosedLocked()（914-935）+ StateForAccount 共用（708）+ handleAdminStats window_closed（935）同源；开窗点 10s 裕量入账与判定单快照复用。TestWindowClosedProbeDropsToFar 绿。
- **openTime 识别槽三硬契约**：槽保留（空快照不删，ProbeForAccount 848-852 / probe 1102-1106 只在非空 beginTimes 时写）；展示层过期判定独立（StateForAccount 719）；写入持锁。PurgeAccount 全清只给现存账号。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1007）+ WindowOpened=true 例外（黄金期 250ms 冲刺放行）；既有守卫测试 TestSubmitSuspendedWhenOpenTimeCleared 保持绿（夹具 WindowOpened 默认 false）。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow（ProbeForAccount 不写 lastProbe，B6-04 正确）；probing 单飞锁内置位-检查（1045-1051）；probeSem cap 4（1069-1071）；tick 首探豁免（985）。
- **多账号年级隔离**：probe() per-account goroutine + 专属帧；ElectivesSnapshotFor 目标账号过期专属帧 → (nil,false) 绝不回退全局帧（782-810）。
- **doLogin 闸门全收口**：gateTryAcquire 非阻塞 + gateWait 阻塞共享 gateMu/gateUsed；LoginByPassword/Relogin 无旁路；GatePump 每 30s 推进广播。窗口翻转无计数竞态（OBSERVE-49-04）。
- **鉴权与数据安全**：requireAuth 401 / requireAdminSession 403 / recoverMiddleware 500 / 登录激活限流 429 / requireJSONBody CSRF 403 一族真实状态码；SpaHandler /api 精确+前缀 404（embed.go:28-31）；XFF 仅回环 + XUANKE_TRUSTED_PROXY=on 信任最右非空。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量拼接；migrateAddPublishMeta 增量幂等 + refuseLegacy 缺列清单对应剔除；SQLite 单连接串行 + WAL + busy_timeout(5000)。settings 全量替换事务原子（OBSERVE-48-05 延续）。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 1024 位公钥硬编码；uniqueDeviceID 复刻逐字段一致；captcha 信号量（Cond 动态热收敛，TestSetCaptchaConcurrencyConcurrent 防死锁）+ 重试收敛（≤3）；doRequest code=-1 统一 ErrUnauthorized；cookie/idToken 双通道与逆向契约一致；`Login` 内 sess 独立 http.Client + 每次 attempt 独立 jar——无共享连接池污染。
- **凭据加密**：AES-256-GCM + 随机 nonce 前置；`.master_key`/env 32 字节校验（F17-04 含预生成文件）；enc: 前缀；旧明文拒绝加载（main.go:93-95）。
- **session 层**：12h TTL 惰性失效 + 5min 清扫；ConsumeTicket 锁内四查后置 used+delete 无竞态；sweeperClose sync.Once + sweeperDone 收口无泄漏。

---

## 结论

- **MAJOR 1（测试基础设施 flake，升级确认）/ MINOR 0 / OBSERVE 6 新（49-01~06，其中 4 项纯确认无问题）/ 延续 18 项**，共 MAJOR 1 + OBSERVE 6。
- 最重 3 条（按影响排序）：
  1. **MAJOR-49-01（F48-M1 `-p 1` 未根除 httptest flake）**——4 轮全量 `-race -p 1` 3 FAIL，全部为 httptest `connectex`/超时（TestAdminDeleteAccountNoBodyOK / TestAdminStatsAccountsLogs / TestAdminDeleteAccountMemoryFirst / TestStudentSetTargetsWithoutAccountOK / zhidao TestCaptchaConcurrency），失败单跑全绿、无业务断言失败、无 race 报告。`-p` 档位无法根除"单包内部 test 级连接 churn"（Windows 回环 TIME_WAIT），F48-M1 的"根除假红"目标落空。建议登录链路测试首请求重试或接受 flake 配 CI 重跑；不改业务代码。
  2. **OBSERVE-49-01（main 优雅退出无信号处理）**——SIGTERM/SIGINT 时 sched.Stop/sessions.Close 不执行；SQLite WAL crash-safe + 会话内存态使数据安全，仅飞行 spawnChain 的 success 可能未落库（重报幂等）。观察留档，公网生产化时补 signal.NotifyContext。
  3. **OBSERVE-49-05（task_log 黄金期日志风暴无清理）**——非满员失败每 tick 一行（≈33 行/发布·黄金期 10s），长运行 DB 无限增长无清理路径。历轮观察延续，建议 AppendLog 侧周期性收敛。
- 上轮观察项 18 条全部复核：OBSERVE-48-03（writeJSON 家族偏差点）已被 F48-O3 闭合（测试断言已更新为 HTTP 500）；OBSERVE-46-01（interval clamp）已闭合（F46-O1）；MAJOR-48-01 **升级**为 MAJOR-49-01（-p 1 未根除）；其余 14 条延续观察，无升级。
- F48-O3 正确闭合（writeJSONStatus 500 家族统一 + TestAdminConfigSaveFailStillDispatch 真断言）；**F48-M1 未达成目标（-p 1 仍假红）**，见 MAJOR-49-01。F46-O1 clamp、B43/B44/B45 实修全部复核无回归。
- 残余风险集中在三条延续观察：孤儿登录平台侧副作用（MAJOR-42-02）、tick 无 recover（43-04，interval 已 clamp 封堵）、task_log 无清理（47-01 / 49-05）+ 本轮新增的优雅退出留档（49-01）。
