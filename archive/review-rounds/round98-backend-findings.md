# R98 后端只读审查报告

审查对象：xuanke-auto HEAD `86522e8`（R97 收官，进度 98/256；R98 为纯观察轮延续——HEAD 与 R97 相同，无新代码提交）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round98-backend-findings.md）。并行前端代理产出 round98-frontend-findings.md（见 git status `??`，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 三轮 + 定向复现 / go build / go vet / gofmt / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量),api(全量),session,store,accounts,zhidao,db,config}、quit_shared.go、tray_windows.go/tray_other.go、cmd/{bench,probe,logintest}。新契约角度本轮聚焦：**登录/激活/票证端到端状态机（CreateTicket → ConsumeTicket 完整流转与失败分支）+ 数据库并发写路径清点（SQLite 单写者下锁内写 vs 锁外写）+ 时钟对齐边界（SyncServerTime 失败退避/syncing 复位/clockOffset 应用范围）+ 资源与小工具（cmd/ 三工具与主程序 config/env 契约一致性）**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O98-01（O86-01/O94-01/O95-01/O96-01/O97-01 api 抖动基线第十三轮）：三轮全量 race 二绿一"api 单包单次 FAIL"——低频残余归因，基线维持（实测）

**证据**：三轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）结果：第一轮全 11 包全绿（api 27.956s / scheduler 15.175s / store 36.624s / zhidao 3.210s）；第二轮 **仅 api 包 FAIL 一次**（34.322s，异常耗时约 1.7 倍于正常 19-28s，日志尾段为登录 mock 的 Vision 识别成功交错日志 + 一行 FAIL，无 panic / 无 DATA RACE / 无明确测试名输出），zhidao 包本轮全绿（2.713s）；第三轮全 11 包全绿（api 19.704s / zhidao 2.884s）。

**失败归因**：紧随其后定向复现——api 包独立全量 race 一次全绿（20.094s）。34.322s 的异常耗时符合"某登录 mock 测试在 Windows 回环冷启动窗口首请求 connectex 后进入重试/超时慢路径"特征（与 R94/R97 两轮 1-2 例低频残余同族；api 包夹具 socketPreheat + readyProbe 双防线已覆盖该族，readyProbe 轮询窗口 ~2s 在极端冷启动时刻仍可穿透）。**非产品逻辑缺陷，CI `-p 1` + 失败重跑吸收口径不变**。本轮残余宿主轮换（R94/R97 为 zhidao 包、R98 为 api 包），进一步佐证"包序 + 冷启动窗口"主导而非产品缺陷。

### O98-02（O97-04/O80-01 删除保护撞名延续复核）：handleAdminDeleteAccount `IsAdminAccountName` 单判据与 B43-04 双条件不对称——历轮已声明维持观察（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只看账号名等于 `AdminNameValue()`。撞名场景（`XUANKE_ADMIN_NAME` 显式配成某学生学号）下该学生账号被删除保护永久覆盖；B43-04 已为登录路径处理同款撞名（双条件签发），但删除路径未对称。历轮归"低优先级不修"，维持观察。

### O98-03（O90-01 scheduler.go gofmt CRLF 噪音延续复核）：工作区 CRLF 转换噪音，git 仓库内容 LF 合规（实测）

**证据**：工作区 `internal/scheduler/scheduler.go` 被 `gofmt -l` 检出；`git show HEAD:backend/internal/scheduler/scheduler.go` 实测纯 LF（CRLF 0 / LF 2049），差异纯为 checkout 时 `core.autocrlf=true` 的 LF→CRLF 转换噪音，非源码缺陷，无需提交动作。历轮 O90-01 维持。

### O98-04（O96-06/O97-03 契约 20 注释轮次标签回归）：全仓扫描仅一处文档路径引用，非违规（实测）

**证据**：grep `（第 \d+ 轮）|round\d\d|R\d\d-\d\d` 全仓仅命中 `backend/internal/session/store.go:117` 注释中的 `docs/review-round13.md` 文档引用（文件路径名，非轮次决策标签），不违反契约 20「代码注释严禁轮次前缀标签」字面。历轮固化回归维持，**零违规残留**。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无 | 三轮全量（二绿 + api 单包单次低频残余已归因）+ 定向复现全绿 + 逐点走读闭环，无新增可疑项。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第十三轮）**：逐点 grep `sameClientFor`（scheduler.go:204，判 nil 恒 false + reflect 指针身份）+ 全量调用点——ProbeForAccount 回写段（:850）、spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口（:1600）/确证满员（:1635）六分支；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeNow（:964-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 路径（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）。**grep 全量复核无新裸露写点**。 | 通过（走读） |
| **B88-01 修复持续复核（第十三轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；acctData nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16/:116/:176）定向 + 全量 race 回归全绿。 | 通过（实测） |
| **登录/激活/票证端到端状态机（本轮新角度）**：完整流转 CreateTicket（session/store.go:102，随机 32 字节 hex + 绑定账号 + 5min TTL）→ handleLogin 教务登录成功未激活签发（handler.go:153）→ 前端 1001 回传 → handleActivate（handler.go:181）先 ConsumeTicket（store.go:118：存在/未用尽/未过期/账号匹配四判，消费成功即 delete 作废——单次防重放）→ 再 ConsumeActivationCode（store.go:292：单事务 UPDATE used_uses<total_uses 原子扣次 + activations 存在性检查回滚防双扣）→ issueSession 签发会话。失败分支穷举：票据不存在/已用/过期/账号不匹配/激活码无效/用尽/已激活/机制关闭——全部有明确文案。**关键一致性**：handleLogin 在 TrimSpace 之后（:110）CreateTicket，handleActivate 同样用 TrimSpace 后的 acct（:195）ConsumeTicket——两端账号归一，杜绝"空格拼接进票据绑定账号导致激活必然失败"。登录/激活各自独立限流桶（router.go:91-92，激活码错误不消耗登录额度）。**票据 5 分钟 TTL 内可被重放穷举但每次失败即销毁票据的刻意决策已注释落盘（store.go:113-117），与 CLAUDE.md 决策契约 14 一致**。 | 通过（走读 + 测试绿） |
| **数据库并发写路径清点（本轮新角度）**：db.go:23 `SetMaxOpenConns(1)` 单写者串行化；store 全方法走事务（Begin→Commit/Rollback）或单条 Exec；scheduler 持 s.mu 的锁内写仅 MarkDone/RemoveDone/spawnChain 成功分支与各 setStateLocked（AppendLog/SaveSuccess 等微秒级 SQLite 写）；spawnChain 成功分支锁内两行写注释已载明"持锁写窗口仅成功分支两行，彻底消除需独立 DB goroutine，边际不动（观察项）"（:1585-1587）；实时复核网络段（classFullRealtime 最长 15s）刻意锁外执行（:1588）。busy_timeout(5000) 兜底锁冲突。**锁内写不跨网络段，黄金期不因 DB 写停顿网络往返**。 | 通过（走读） |
| **时钟对齐边界（本轮新角度）**：maybeSyncClock（:322-407）快路径 1min 成功闸门（lastSyncTime 仅成功推进）+ 失败 30s 退避（lastSyncFailAt）+ syncing 单飞 + syncFailStreak≥3 复位 clockOffset 回本地时钟 + 成功清零自愈；无客户端/不支持同步时复位 syncing（:403-406），杜绝"syncing 置位无人复位 → 时钟校准永久休眠"；clockOffset 应用范围全校统一（nowAligned/nowAlignedLocked 单一时间基，探测/提交/窗口判据/退避读侧全部对齐）。SyncServerTime（client.go:112-139）读 HTTP Date + RTT/2 中点近似。**边界完整，无空窗无永久挂起**。 | 通过（走读） |
| **探测与提交节流交互（本轮新角度）**：probeSem cap 4（scheduler.go:192/:252）封顶 per-account 探测并发（跨批同信号量约束绝不叠加）；lastProbe 全校 30s 闸门只归 probe()/ProbeNow（:164，ProbeForAccount 刻意不写——管理员穿透探测不吞全校节流，:868-872）；tick 提交不受探测节流限制（黄金期 250ms 冲刺，:1000-1035）；probe() 单飞（probing）+ 失败计入节流闸门（:1089）。**四者叠加边界无并发漏洞**。 | 通过（走读） |
| **cmd/ 三工具与主程序契约一致性（本轮新角度）**：cmd/probe 用 `zhidao.SharedTransport()`（15s 超时 + 64 连接/host + HTTP/2）替代 http.DefaultClient、token 从环境变量读不硬编码——与全仓生产 HTTP 契约族对齐；cmd/logintest 与主程序同源（config.Load + db.Open + secure.LoadOrCreateKey + 识别引擎跟随 config.CaptchaEngineDefault 三态回退 + NewCaptchaSemaphore(1) 并发 1 + --wait 间隔防平台限流），init() 兜底仓库根 data/；cmd/bench -n≤0 边界防御。**三工具无契约漂移**。 | 通过（走读） |
| **M87-01 窗口再评估（第十三轮）**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）；quit_shared.go 双 nil 防御（:30-36）+ setExitActions 两半段注入（main:200 关服务 / tray_windows.go:54 退图标）+ tray_quit_test.go 钉死。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **零吞错复核（契约 17）**：本轮扫查 spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handleAdminConfig 落库（:814 log + :819 500）、accounts LoginByPassword 加密失败（:286）、main 恢复各表失败——全部 `if err != nil { log.Printf }` 零吞错。 | 通过 |
| **O98-01 抖动基线第十三轮**：三轮全量（二绿 + api 单包单次 FAIL 归因低频残余）+ 定向复现全绿（20.094s）；scheduler/store/zhidao/accounts/db/config/runtime/session 各包三轮全绿。基线结论：低频残余由宿主环境冷启动窗口 + 包序主导，非产品缺陷，CI 失败重跑吸收口径不变。 | 通过（实测，O98-01） |

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（:855-858 每账号 / :1106-1110 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1011-1013）；识别槽无值回退遗留 openTime 字段（:435-437）；配置层零注入（config.go:55-57 XUANKE_OPEN_TIME 已随 B40-02 整体移除）。 | 通过（实测） |
| **14（手动报名/退选 4 方法协同）**：TryAcquireSubmit 在飞互斥（:1896）+ MarkDone 清 success/refused（内存+库行 DeleteRefusedClass :1938-1948）+ RemoveDone 落 refused + 删 success 行（:2018-2028）+ RemoveFull（:2038）；窗口已关按满员记 full 不轰炸；spawnChain 提交前 inflight 去重（:1464）。 | 通过（走读） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（历轮清点 + 本轮复核主要写点）。 | 通过 |
| **B42-01（doLogin 全局频率闸门）**：gateTryAcquire 非阻塞（accounts/manager.go:223-235）+ gateWait 阻塞（:49-64）共享 gateMu/gateUsed 同一窗口计数；LoginByPassword pre-Login 准入（:244）；ResetGateForTest 仅测试（:82-87）。双入口无绕行。 | 通过（走读） |
| **B43-04 + F52-M1（撞名双条件 + 管理态判定绑定 adminName）**：handler.go:121 双条件签发 + loginTimingFlat 300ms 错误分支拉平（:124/:137）；撞名学生教务登录正常签发普通会话；前端 adminAuth.ts isCurrentAdminSession + 登录响应 adminName 标记 + 全路径清标记。 | 通过（走读） |
| **7（?account= 透传全路径凭据表校验）**：handleElectives:245 / handleElectiveSelect:296 / handleElectiveExit:373 / handleSetTargets:461 / handleState:537 五处 accountExists 全覆盖，查无此账号整体拒绝；accountExists 用 LoadCredentials 更强真理源（:1073-1084）。 | 通过（走读） |
| **5（"落库前锁内复核 ClientFor"防线族）**：自动链成功 / MarkDone / RemoveDone / 重登成功 / 实时人数复核回锁后 / spawnChain 链顶与取 client 后全部复核存在；已删账号静默放弃；实时复核满员分支先 doneHas（绝不覆盖手动胜利状态，:1627/:1645）。 | 通过（走读） |
| **14-决策契约（激活票据防穷举）**：CreateTicket→ConsumeTicket 单次防重放 + 失败即销毁 + 5min TTL（session/store.go:21/:113-117）+ 激活限流独立桶（router.go:92）——持码者对任意已登录账号激活的台账外接管已封堵。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第一轮） | **全 11 包全绿**（api 27.956s / scheduler 15.175s / store 36.624s / zhidao 3.210s 等） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第二轮） | 11 包中 10 包全绿，**api 包单次 FAIL**（34.322s，无 panic/无 DATA RACE，O98-01 低频残余归因） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（全量第三轮） | **全 11 包全绿**（api 19.704s / store 21.069s / zhidao 2.884s 等） |
| `go test -race -count=1 -p 1 ./internal/api/`（定向复现） | 全绿（20.094s） |
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，O90-01 延续；git 仓库版本 LF 合规——O98-03 实测 CRLF 0/LF 2049） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round98-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第十三轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段复核通过；probe_identity_test.go 三钉实测通过；round41 七分支测试族全绿（waitChainExit 等待契约复用，绝不用 inflight 等待）。
2. **O86-01/O94-01/O95-01/O96-01/O97-01 api 抖动基线第十三轮**：三轮全量 race 二绿一"api 单包单次 FAIL"（34.322s 异常耗时 + 定向复现全绿，归因 Windows 回环冷启动窗口低频残余，与 R94/R97 同族——本轮残余宿主从 zhidao 轮换到 api，进一步佐证包序 + 冷启动主导）。**基线维持低频波动口径：CI `-p 1` + 失败重跑吸收口径不变**，属测试健壮性级观察（O98-01），非产品缺陷。
3. **新角度扫查全绿**：登录/激活/票证端到端状态机穷举闭环（CreateTicket→ConsumeTicket 单次防重放 + 失败即销毁 + 两端账号 TrimSpace 归一 + 独立限流桶 + 原子扣次防超卖）；数据库并发写路径清点无锁外网络段（单写者 + 锁内微秒级写 + 实时复核刻意锁外）；时钟对齐边界完整（1min 成功闸门 / 30s 失败退避 / syncing 单飞与复位 / streak≥3 复位 offset / 成功清零）；探测与提交节流交互四者叠加无漏洞（probeSem cap4 / lastProbe 全校闸门 / 黄金期 250ms / 提交不受探测节流）；cmd/ 三工具与主程序 config/env/HTTP 契约族零漂移。
4. **新增 OBSERVE 一条**（O98-01 api 单包单次低频残余归因 + 基线记录）、延续复核三条（O98-02 删除保护撞名 / O98-03 CRLF 噪音 / O98-04 契约 20 回归零违规），**无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求**。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R97 同型），无代码修改建议提交。
