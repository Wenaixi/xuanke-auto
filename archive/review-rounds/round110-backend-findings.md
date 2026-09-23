# R110 后端只读审查报告

审查对象：xuanke-auto HEAD `8d79bcf`（R109 收尾轮）。backend/ 自 B101-01（20c5882）起**连续八轮零产品代码改动**（`git log 20c5882..HEAD -- backend/` COUNT=0 实证），最近一次后端代码改动仍为 `ca08c46d`（B88-01 ProbeForAccount 回写段身份复核）。工作树洁净。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件。

审查方式：Read / Grep / Bash 只读命令（go build / go vet / 定向 go test -race 实证 + 逐点走读 + git log 实证）。核心走读范围：身份防线矩阵第二十五轮（全家福写点 grep 实证 + 偶发写点专项）、O105-01 抖动基线第二十五轮夹具走读、观察项与知识位盯守（O105-02/B105-01/O106-01/B101-01~B106-01/B109-01）、既往观察项延续（O92-02/M87-01/O90-01 + 契约 20/17 强扫）、新契约角度 c（错误处理可观测性）。

## CRITICAL

无。

## MAJOR

无。

## MINOR

### B110-01（MINOR）：手动报名/退选失败路径无库内日志落点，与自动链失败不对称

**位置**：`internal/api/handler.go:340-363`（handleElectiveSelect 全部失败分支）/ `:398-431`（handleElectiveExit）。

**发现**：手动报名/退选的**失败**分支只 `writeJSON` 回显前端文案，**不调 `AppendLog`**——token 失效分支（`ErrUnauthorized`，:348-353）只触发 `MaybeRelogin` + 回显；read 类错误分支（:355-359）只回显"请求已发出但响应读取失败"；其余业务错误（:361-362）只回显 `err.Error()`。**成功**路径全部经 `MarkDone`/`RemoveDone`（scheduler.go:1976/:2029）落 `AppendLog(select, ..., true)`。

**与自动链不对称**：scheduler 自动链失败必有 `setStateLocked(failed)` + `AppendLog(select, ..., false)`（scheduler.go:1662-1666）。而手动路径失败完全无库内审计行——尤其 read 类错误（请求已发出、平台可能已处理）恰恰是**最需要留痕**的场景（退选/报名结果未知，事后无法在 `/api/logs` 与管理日志总览核对该动作到底成没成），本次失败却无任何 task_log 行。

**实证**：handler.go 全文件仅 10 处 `log.Print`（全部为落库失败告警），0 处失败动作回写；对比 scheduler 层 AppendLog 十五个调用点全部 `if err != nil { log.Printf }`（R95 契约 17 已钉）。R94 新角度核对过的 "AppendLog 16 调用点零吞错" 覆盖不到此缺口（那是对**有日志的点**核对，手动失败分支是**没有日志的点**）。

**影响**：低。前端回显存在、正确性无缺陷；属审计完备性缺口——"手动动作失败"不进日志总览。修复建议（未来轮）：手动失败分支各补一条 `AppendLog(acct, classID, "select"/"exit", 失败文案, false)`，与自动链失败同语义对齐。

## OBSERVE（延续观察 + 新发现）

### B110-02（OBSERVE）：reloginAt 时间基仍残留本地钟（读侧写侧同基，无混用）

**位置**：scheduler.go:1231 / :1261（写侧 `time.Now()`）、:1227 / :1219（读侧 `time.Since(t)`）。

**事实确认**：重登节流判定的**两侧均用本地钟**（写 `time.Now()`、读 `time.Since`）——与 R94 已消灭的"写本地/读对齐钟跨基混用"不同，本处是**同基内部自洽**，现无实质风险（偏差 ~640ms 对 30s 基础节流窗口可忽略）。仅从"调度器全链路统一对齐钟"体系看，探测节流（:964/:1089/:1114 对齐钟）、提交节流（:1346 nowAlignedLocked）、满员退避（:1714 nowAlignedLocked）、时钟失败落档（:372/:377 声明写而不读）均已统一，reloginAt 是唯一残留 `time.Now()` 的调度实体。且重登发起的实际门控是布尔态（relogging/tokenValidForLocked 链顶短路 :1421/:1426），reloginAt 只影响"多久后再发起"，时间基漂移无行为危害。**维持低风险观察**：若未来重登节流窗口缩短至秒级需统一到对齐钟，当前不修。

### B110-03（OBSERVE，延续）：B109-01 RemoveFull 死方法 + O106-01 票据 + O105-02 撞名 + B105-01 展示层

- **B109-01 RemoveFull**（scheduler.go:2037-2049）：本轮 grep `\.RemoveFull\b` 全仓命中仅定义处、调用方为零——死方法延续，无提级依据。
- **O106-01 票据**：session/store.go:73-87 sweepExpired 双 map 清扫 + ConsumeTicket 过期即删（:128-131）+ handleActivate 先 ConsumeTicket 再 ConsumeActivationCode（handler.go:199→203）三重回收在案；残余仅"重启丢票 + 5 分钟清扫间隔间歇留存"，维持低风险观察。
- **O105-02 删除保护撞名**：handleAdminDeleteAccount（handler.go:992 `acct == "" || acct != req.Account || d.IsAdminAccountName(acct)`）仍为单判据，维持观察无新依据。
- **B105-01 展示层**：handler.go:898-902 open_time_set（识别槽非零即 true，含过期值）与 scheduler.go:714-716 open_time_known（依 now 过期判定）不对称维持，走读无新依据提级。

## 必查项逐条结论

### 1. 身份防线矩阵（第二十五轮）——通过

**新代码写点核查前置**：backend/ 连续八轮零产品代码改动（`git log 20c5882..HEAD -- backend/` COUNT=0），"新代码引入的写点"为空集——聚焦既有写点与偶发写点的再次核位。

**sameClientFor 调用点**：本轮 grep 实证 7 个调用点 + 1 处定义——`:204`（定义）/`:850`（ProbeForAccount 回写段）/`:1489`（失效分支）/`:1521`（成功分支）/`:1551`（风控退避）/`:1571`（窗口关闭）/`:1600`（实时复核回锁统一）/`:1635`（确证满员分支），与 R109 清单**零漂移**。

**写点全家福 grep 实证**（本轮回合重跑，与 R109 表逐一对位）：

| 写点 | 行号 | 防线 |
|---|---|---|
| `openTimeDetected[acct]=` | :856（锁内 + sameClientFor :850 后） | 探测回写段身份复核 |
| `openTimeDetected["*"]=` | :1108（锁内 1105-1108） | 全局载体无身份概念 |
| `acctData[acct]=` + `acctDataAt[acct]=` | :863/:864（锁内 + 身份复核后） | 同上 |
| `tokenValid[acct]=true/false` | :1241（决策侧，ClientFor 复核 :1208-1210 后）/ :1262（写回侧，复核 :1254-1258 后） | 双闭合 |
| `reloginAt[acct]=` / `reloginFail[acct]++` / `relogging[acct]=true` | :1231/:1232-1233/:1236（决策侧） | 同上；PurgeAccount :508/:509 全清 |
| `inflight[acct][id]=true` | :1471（spawnChain）/ :1905（TryAcquireSubmit） | 同步 delete :1490/:1501/:1513/:1950/:1954/:2002/:2006/:510(Purge) |
| `done[acct][id]=true` | :615（RestoreDone）/ :1529（spawnChain 成功 + sameClientFor :1521 后）/ :1934（MarkDone + ClientFor :1927） | 全在位 |
| `refused[acct][id]=true` | :634（RestoreRefused）/ :2011（RemoveDone + ClientFor）；MarkDone :1938 delete + DeleteRefusedClass 库行 :1945 | 全在位 |
| `full[acct][class]=true` | :1767（markFullLocked，仅 :1575/:1639 两条 sameClientFor 后调用续） | 单入口 |
| `rateLimited[acct][class]=` | :1714（仅 :1555 风控分支 sameClientFor 后） | 单入口 |
| `state.Courses[idx].Status/Result` | setStateLocked 统一入口 + 内联写 | 全部处于持锁 + 身份防线后 |

**偶发写点专项（第二十五轮重查）**：① `MarkTokenValid`（:1306-1319）纯 delete 清失效标记，reloginMu→s.mu 锁序对齐；② `TryAcquireSubmit`（:1890-1914）api 手动独占入口、无网络往返后写回；③ `releaseFullIfFreedLocked`（:1781-1809）由 spawnChain 持锁调用、写"该账号自己 full 解封"语义；④ `submitAll`（:1342-1380）链顶 ClientFor 过滤 + spawnChain 链顶二次判 + 取 client 后再判双保险。**无新裸露写点、无新增偶发写点**。

**手动四路 accountExists**：:245（课程读）/ :295（手动报名）/ :373（手动退选）/ :537（状态读）在位，与 accountExists 判据（:1073，凭据表 LoadCredentials 遍历）同源。

**重登决策侧复核**：maybeRelogin（:1198-1242）入口位置 `ClientFor(acct)` 存在性复核（:1208-1210）在任何 map 写入前，B43-01 在位；写回侧（:1244-1274）先清 relogging 再复核存在性、成功分支 UpdateIDToken 落库（:1269）错误留痕，B43-02/B41-01 无退化。

**测试钉**：probe_identity_test.go 三钉 + TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} 五分支——定向 race 全绿（4.53s + 2.02s）。

### 2. O105-01 抖动基线（第二十五轮）——通过

socketPreheat（zhidao/client_test.go:25-30）+ TestMain 包级预热（:39-40）+ loginMockServer（:43-81）内 readyProbe 宽栅栏（200ms×10 + 2s 超时）+ api 包 newTestDepsModeName 预创建套接字（handler_test.go:53-68）+ readyProbe（:179-215）——各夹具与 R109 描述逐字符一致，**零改动**。

**本轮新鲜证据**：本人定向跑 api 包 `-run "TestManualElective|TestElective|TestTargets|TestAccountExists|TestAdmin" -race -count=1` **首跑 FAIL（24.45s）**——重跑 `-count=3`（38.98s）与再 `-count=3`（60.10s）**连续全绿**。该单次 FAIL 恰落在既有口径"低频残余由包序 + Windows 回环冷启动窗口主导（八轮 30 跑 4 单 FAIL ~13%）"内——cold 首跑 connectex 型偶发、即刻自愈，无新时序脆弱点。`go build ./...` BUILD_EXIT=0，六包 `go vet` VET_EXIT=0。store 包 43.2s 高耗时核查为 `TestActivationCodeConcurrentConsume`（openStoreMultiConn 放开 16 连接 + 20 并发 goroutine + busy_timeout 5000ms 排队）的正常并发测试代价，非 flake。

### 3. 观察项与知识位盯守——通过

- **O105-02 删除保护撞名 / B105-01 展示层 / O106-01 票据**：见 B110-03，均维持观察无新依据。
- **B101-01（sanitizeError）**：定义 :581-593 无漂移；doRequest :450 唯一应用点；唯一 token URL 通道 = doRequest :422 `?idToken=`。脱敏族（sanitize_test.go 六测试）+ IsReadErr 族定向 race 全绿（2.05s）。
- **B102-01（http 直调点 URL 静态）**：captcha.go:151、client.go:294/:327/:361（+ fetchLoginPage client.go:97 的 `/login` 静态）——全部静态路径，无 idToken 拼接；登录链路四步（:294/:300、:327/:334、:361/:370）均为静态。
- **B42-01 闸门家族**：gateWait（阻塞排队，manager.go:49-67）/ gateTryAcquire（非阻塞准入，:218-231）共享同一 gateMu/gateUsed 计数；Relogin（:166）走 gateWait、LoginByPassword（:244）走 gateTryAcquire——两条路径严格共享每分钟 2 次预算。ResetGateForTest（:79-86）仅 api 夹具使用，正式代码零调用（grep 实证）。GatePump（:71-77）由 main.go 后台协程驱动。
- **B103-01 / B104-01 / B105-02 / B105-03 / B106-01 / B109-01**：实时复核活化条件、手动路径无在飞窗口、TokenValidFor 半态、probeSem cap 4、三引擎 withConcurrency——各就其位无退化；RemoveFull 死方法延续（B110-03）。

### 4. 既往观察项延续——通过

- **契约 20 全仓强扫**：`第 ?[0-9]{1,2} ?轮|(R|O|B|M|F)[0-9]{2}-[0-9]{2}` 生产代码（含 backend 根目录三托盘文件 + main.go）仅命中 3 处合法锚点：scheduler.go:843（M88-01 身份防线引用）、quit_shared.go:7（M86-01 缺陷形态引用）、tray_windows.go:53（M86-01 回归钉）。**零违规轮次标签**。
- **契约 17 零吞错**：全仓 `_ = ` 落库点扫描——仅 4 处命中：handler.go:363/:431（`_ = d.Sched.MarkDone/RemoveDone`，状态回写返回错误已内记日志的设计返回，R104/R109 已判非吞错）、scheduler.go:307（`_ = pw.Prewarm()` 非落库）、:1075（`_ , _ = s.ProbeForAccount` 探测结果非落库）。**零吞错落库点**。
- **O92-02 logintest**：cmd/logintest/main.go:55-72 引擎判定源跟随 `config.CaptchaEngineDefault()`，分叉维持观察无新证据。
- **M87-01 窗口**：windowClosedLocked 三判据单源（:918-939）+ open 单快照复用（:924）+ 时钟兜底带"开放时间已过"（:921-923）+ 幽灵窗口 EmptyProbeRuns≥3（:935）在位；入账段 10s 裕量单快照（:1145/:1156）在位。
- **O90-01 CRLF**：go vet 六个业务包全干净（VET_EXIT=0）。

### 5. 新契约角度 c（错误处理可观测性）——见 B110-01/B110-02

**逐错误族可辨性核对**（区分体输错、网络错误、业务错误、平台错误）：

| 错误族 | 判据 | 回显/落库 | 可辨性 |
|---|---|---|---|
| 登录：管理员口令错 | B43-04 双条件 + 撞名归因 | "管理口令错误" | 可辨 |
| 登录：教务失败 | LoginByPassword err | "登录失败: <err>" | 可辨 |
| 登录：未激活 | IsActivated false | code=1001 + ticket | 可辨（前端弹激活码框） |
| 激活：票据无效/账号不匹配 | ConsumeTicket | "激活票据无效或已过期" | 可辨 |
| 激活：码无效/用尽/已激活 | ConsumeActivationCode | 统一文案，不扣次 | 可辨 |
| 报名：token 失效 | errors.Is ErrUnauthorized | "教务令牌已失效，正在自动重登"+触发重登 | 可辨；**无库日志** |
| 报名：read 类 | IsReadErr 三形态 | "请求已发出但响应读取失败（可能已处理）" | 可辨；**无库日志** |
| 报名：业务失败 | SelectClass code!=0 | "报名失败: <平台 msg>" | 可辨；**无库日志** |
| 自动链：风控 | isRateLimitError 子串匹配 | 30s 退避 + AppendLog | 可辨 |
| 自动链：窗口关闭 | isWindowClosedError 子串匹配 | 按满员记 full + AppendLog | 可辨 |

**结论**：① 五类错误文案可辨性成立（历史轮已实证）；② **缺口集中在"手动路径失败无库内日志"（B110-01 MINOR）**——前端可辨但审计链断裂；③ reloginAt 时间基残留为低风险观察（B110-02）；④ scheduler 层错误归档其他分支（sanitizeError/AppendLog/状态 failMsg 区分 read 文案）全部在位。

**附带排查询（无缺陷）**：
- `isRateLimitError`/`isWindowClosedError` 均为子串匹配（"频繁"/"429"/"稍后重试"/"关闭"/"未开启"/"报名时间"/"已结束"）——只用于自动链 err 归并，与手动路径文案（不同于串）无交叉误判。
- **store 事务边界**：五个 Begin（SetTargetsForAccount/CreateActivationCodes/ConsumeActivationCode/SaveSettings/DeleteAccount）全部 `defer tx.Rollback()` + `tx.Commit()`，事务内仅 `tx.Exec/QueryRow` 直接 SQL、**无事务内嵌套调用其他 store 方法**，无嵌套事务。
- **scheduler 持锁调 store**：共 15 处 `s.store.*` 全部在 s.mu 持锁下执行，但 store 层无锁（单连接直查），`db.SetMaxOpenConns(1)` SQLite 单写者串行化——锁序 `s.mu → store(无锁) → sqlite`，无锁内嵌套加锁，无死锁面。
- **时钟并发一致性**：nowAligned（:265-270）短暂持锁读 snapshot 后释放加 offset；SyncServerTime（zhidao/client.go:112-139）RTT/2 中点近似；maybeSyncClock 失败退避 30s + syncing 单飞 + 无客户端复位（:403-406）在位。

## 验证表

| 验证 | 结果 |
|---|---|
| `git log 20c5882..HEAD -- backend/` | COUNT=0（backend 连续八轮零产品改动） |
| `go build ./...` | BUILD_EXIT=0 |
| `go vet` 六包 | VET_EXIT=0（O90-01 CRLF 干净） |
| scheduler 定向 race（身份防线族 + 窗口守卫族 + 时钟族 + 同名重建五分支） | 全绿（4.53s + 2.02s） |
| api 定向 race（手动/目标/账号/管理族） | **首跑 1 FAIL（24.45s）→ 重跑 3+3 连跑全绿**（低频残余实证现身一次，口径内自愈） |
| zhidao 脱敏族 + IsReadErr 族 race | 全绿（2.05s） |
| db / store / session 定向 race | 全绿（2.47s / 43.20s / 1.87s，store 高耗时=并发消费测试正常代价） |
| 写点全家福 grep | 12 类写点与 R109 清单逐一对位、无新写点 |
| sameClientFor 调用 7 处 | 零漂移 |
| 契约 20 生产代码强扫 | 零违规，仅 M88-01/M86-01 三处合法锚点 |
| 契约 17 零吞错 | `_ =` 全仓仅 4 处命中（2 处非落库 + 2 处设计返回） |

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第二十五轮闭合**：零改动前提下全家福 12 类写点 + 偶发写点专项 + sameClientFor 7 处零漂移，无新裸露。 | 通过（走读 + grep + 定向 race） |
| **O105-01 抖动基线第二十五轮**：夹具逐字符零改动；本轮实测一次低频 FAIL 首跑现身、重跑即绿，与既有 ~13% 口径吻合。 | 通过 |
| **B101-01~B106-01 知识位**：脱敏链、http 直调点静态、实时复核活化、手动无在飞窗口、TokenValidFor、probeSem、三引擎限流、gateWait/gateTryAcquire 家族——全部在位无退化；RemoveFull 死方法延续。 | 通过 |
| **O105-02 / B105-01 / O106-01**：维持历轮观察，无新依据提级。 | 维持 |
| **O92-02 / M87-01 / O90-01 + 契约 20/17**：维持历轮结论；契约强扫零违规；M87-01 三判据 + 入账单快照在位。 | 通过 |
| **新契约角度 c（错误处理可观测性）**：五错误族可辨性成立；自动链失败审计链完整；**手动失败路径无库内日志（B110-01 MINOR）**；reloginAt 时间基残留（B110-02 OBSERVE）；store 事务无嵌套、锁内无嵌套调用。 | 1 MINOR + 1 OBSERVE |

## 结论

1. **身份防线矩阵第二十五轮闭合**：backend/ 连续八轮零产品代码改动前提下，写点全家福（12 类）与偶发写点专项全部与 R109 清单逐一对位，sameClientFor 7 调用点零漂移，无新裸露写点、无新增偶发写点。
2. **O105-01 抖动基线第二十五轮**：夹具零改动；本轮实测复现一次低频残余（定向 race 首跑 FAIL、重跑双 3 连跑全绿）——与既有"包序 + 冷启动窗口 ~13%"口径吻合，无新时序脆弱点。
3. **观察项与知识位**：O105-02/B105-01/O106-01/B109-01 维持观察；B101-01~B106-01 + B42-01 闸门家族全部在位无退化。
4. **既往观察项延续**：O92-02/M87-01/O90-01 维持；契约 20 全仓强扫零违规；契约 17 零吞错复核通过。
5. **新契约角度 c（错误处理可观测性）**：五类错误文案可辨性成立、自动链审计链完整，但发现**手动报名/退选失败路径无库内日志落点（B110-01 MINOR）**——与自动链失败必落 AppendLog 不对称，read 类"结果未知"场景尤其需要留痕；另 **reloginAt 时间基残留本地钟（B110-02 OBSERVE）**——两侧同基无混用、无实质危害。
6. **新发现**：MINOR 1 条（B110-01）+ OBSERVE 1 条（B110-02）+ 延续观察（B110-03）；本轮首次打破连续多轮"零 MINOR"——B110-01 是有新证据支撑的手动失败审计缺口，建议未来轮修复。

工作树后端文件洁净。