# R119 后端只读审查报告（身份防线矩阵第三十四轮）

## 头部

- **审查对象**：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`（Go 后端 `backend/`）
- **审查 HEAD**：`71b2a95`（R118 收尾总结，master，工作区 clean，`git status --short --branch` 为空）
- **审查方式**：绝对只读——零仓库文件修改。`git log` 实证提交历史、`grep`/`Read` 逐点核位、后台定向 `go test -race` 实证行为、`go build`/`go vet` 实证编译。唯一写入 = 本报告文件。
- **聚焦范围**：聚焦清单 7 项全量执行（身份防线矩阵第三十四轮闭合 / OBSERVE-111-01+112-03 盯守第八轮 / B110-01 审计链第九轮 / OBSERVE-117-01 知识位第二轮 / B110-02 观察复核 / O105-01 抖动基线 / 新契约角度 = accounts 管理器注册表生命周期 + doRequest token/cookie 通道，前后端契约 /state+/electives+/logs 字段级对照）。

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**
**OBSERVE：延续 3 项（111-01/112-03 维持、117-01 知识位确认在位、B110-02 维持）**

本轮 7 项必查全部通过/维持/在位，无证据够格立任何缺陷条——稳定期审查价值确认为「无新证据」。

## 必查项逐条结论

### 1. 身份防线矩阵第三十四轮闭合 —— 通过（零产品改动链延续第十四轮，COUNT=1）

**COUNT 实证**：`git log --oneline 20c5882..HEAD -- backend/` 仅 1 条：

```
57bf401 fix(api): B110-01 手动报名/退选失败分支补库内审计日志
```

为 B110-01 审计修复（非产品改动），与 R110-R118 连续十三轮同一条。**零产品改动链延续第十四轮，COUNT=1**。当前 HEAD 的 R118 收尾总结对本轮已有先验，复核印证与前轮逐字符一致。

**sameClientFor 7 调用点逐点实证（行号与任务清单逐字符零漂移）**：

| 调用点 | 行号 | 身份防线角色 | 实证 |
|--------|------|--------------|------|
| 定义 | `scheduler.go:204` | 注释 :199-203 接口值反射指针身份比对 + nil/已删非同一 | Read 实证 `current==nil→false` + `clientIdentity` 比较 |
| ProbeForAccount 回写段 | :850 | M88-01：网络往返后锁内复核，非同一身份整体放弃快照/识别槽写回 | Read 实证 `if !s.sameClientFor(acct, client) { Unlock; log; return data, nil }`（:851-854） |
| 失效分支 | :1489 | 复核不通过先 delete(inflight) 再 return；maybeRelogin 调用落在复核之后（:1499） | Read 实证 |
| 成功分支 | :1521 | 写 done/state/SaveSuccess/AppendLog 前复核，非同一身份静默放弃写成功落库 | Read 实证（:1526-1540） |
| 风控分支 | :1551 | markRateLimitedLocked 前复核，绝不落退避/状态/日志到重建身份 | Read 实证 |
| 窗口关闭分支 | :1571 | markFullLocked 前复核，防假满员永久退避写进重建身份 | Read 实证 |
| 实时复核统一 | :1600 | classFullRealtime 网络段回锁后先复核再进三路分支 | Read 实证（注释 :1596-1599 明确与五分支对称） |
| 确证满员分支 | :1635 | doneHas 胜利状态让位后、markFullLocked 前复核 | Read 实证 |

**写点全家福 12 类逐项对应防线**（主控裁定维持全量核位后逐点实证）：

- `openTimeDetected[acct]`：写 :856（ProbeForAccount 锁内 sameClientFor 复核后，读实证 :850→:855-858）、全校槽写 :1108（probe()，读实证 :1104-1107 持 s.mu）；清 :506 PurgeAccount。无裸写。
- `acctData[acct]`/`acctDataAt[acct]`：写 :863/:864（锁内复核后）；清 :504/:505；PurgeAccount 注释 :506「识别槽随账号全量清理，绝不残留旧批次识别值」。无裸写。
- `tokenValid[acct]`：写 :1241 置位（maybeRelogin 决策侧，:1208 ClientFor 存在性复核之后）、:1262 清零（重登写回侧 :1254-1258 ClientFor 复核之后）；清 :511 + MarkTokenValid :1316。双闭合。
- `reloginAt[acct]`：写 :1231（决策侧 time.Now()）/ :1261（写回侧刷新）；清 :512 + :1226 退避窗口已过。见必查项 5。
- `reloginFail[acct]`：写 :1233（决策侧 ++，锁内复核后）；清 :1259（成功 delete）+ :513 PurgeAccount + MarkTokenValid :1317。无裸写。
- `relogging[acct]`：写 :1236 置位（决策侧）；清 :1247（goroutine 开头恒清）+ :514 PurgeAccount + MarkTokenValid :1318。无裸写。
- `inflight[acct]`：写 :1469-1471（spawnChain 锁内置位）/ :1896-1905（TryAcquireSubmit）；清 :1490（失效分支复核前）/ :1501/:1513（锁内）+ MarkDone :1950-1951 + RemoveDone :2002-2003 + PurgeAccount :502。无裸写。
- `done[acct]`：写 :1526-1529（成功分支 sameClientFor 复核后）/ MarkDone :1931-1934（ClientFor 复核 :1927 后）；清 RemoveDone :1999-2000 + PurgeAccount :499。无裸写。
- `refused[acct]`：写 :630-634（SetTargetsForAccount）/ RemoveDone :2008-2011（ClientFor 复核 :1995 后）；清 MarkDone :1938-1940（同步 DeleteRefusedClass 清库内行）+ PurgeAccount :503。无裸写。
- `full[acct]`：写 markFullLocked :1760-1767（封面同一身份防线：:1555/:1575/:1639 均有 sameClientFor，:1458 在 spawnChain 持锁内）；清 releaseFullIfFreedLocked :1800 + MarkDone :1953-1954 + RemoveDone :2005-2006 + PurgeAccount :500。无裸写。
- `rateLimited[acct]`：写 markRateLimitedLocked :1710-1714（对齐钟写，仅在 :1551 风控分支同身份复核后）；清 MarkDone :1956-1957 + PurgeAccount :501。无裸写。
- `state.Courses` 状态行：写点全部持锁且在身份/存在性复核之后——spawnChain :1474 submitted / :1502 failed / :1530 success / :1556 failed / :1660 failed / 满员 :1768；MarkDone :1961-1970 success（复核后）；RemoveDone :2013-2015 pending（复核后）；ReleaseFull :1803-1804；清 :518 目标重建。无裸写。

**偶发写点专项**：
- `MarkTokenValid`（:1306-1319）：锁序 reloginMu→s.mu 对齐 maybeRelogin 决策段；只清 tokenValid/reloginFail/relogging 三个恢复标记、不写任何业务身份态；唯一合法入口 = issueSession 手动登录成功（handler.go:226）。spawnChain/手动报名退选路径只调 MaybeRelogin 绝不先调 MarkTokenValid（注释「delete reloginFail 击穿指数退避」防线仍然在位，:345-347/:427-429）。无裸写。
- `TryAcquireSubmit`（:1896-1914）：持 s.mu，inflight 位占位 + `sync.Once` 幂等释放；`(nil,false)` 分支 handler 侧 `defer release()` 未注册不 panic（走读 handler.go:318-323/:410-415 实证）。无裸写。
- `releaseFullIfFreedLocked`（:1781-1807）：持锁；fullHas 守卫；只有快照明确余量（MaxCount>0 且 Selected<Max）才解封，空快照/课程不在快照恒保持 full（窗口关闭防轰炸）。无裸写。
- `SubmitAll`（:1342-1380）：持锁构建链，首行 per-account `ClientFor(acct)` 存在性过滤（:1355，已删账号彻底跳过）；`lastSubmit` 用 `nowAlignedLocked()`（:1346，注释明确与 tick 判读对齐钟同基）；冷启动无目标一次性警告（:1371-1374）。尾部统一走 spawnChain 身份防线。无裸写。

**手动四路 accountExists**（handler.go，全部实证在位，判据同源 `accountExists` :1096-1109 LoadCredentials 逐账号比对）：
- :245 课程读（handleElectives）
- :295 手动报名（handleElectiveSelect）
- :387 手动退选（handleElectiveExit）
- :563 状态读（handleState）
查无账号整体拒绝（"账号不存在，无法..."明确文案），四路全覆盖。

**maybeRelogin 双侧**：
- 决策侧 :1208 锁内 `ClientFor(acct)` 存在性复核（不通过即 unlock return，不写任何 map）。探测定时三处直调入口实证：ProbeForAccount :823 / ProbeNow :956 / probe :1094 + spawnChain 失效分支 :1499（前置 :1489 sameClientFor）/ 实时复核失效 :1610（前置 :1600 sameClientFor）+ handler 手动两处 MaybeRelogin :349/:431——全部入口收口到顶层锁内复核。
- 写回侧 :1254-1258（goroutine 开头先复核客户端仍存在，已删则整个成功分支含 `UpdateIDToken` 落库静默放弃、只清 relogging）+ :1263-1274（成功分支再取当前注册表 client 及其 `.Token()` 才落库）。

### 2. OBSERVE-111-01/112-03 盯守（第八轮）—— 维持（零漂移）

- `handler.go:377` `_ = d.Sched.MarkDone(...)` / `:457` `_ = d.Sched.RemoveDone(...)` 返回值丢弃**仍非吞错**：Read 两函数全函数实证（MarkDone :1922-1982 / RemoveDone :1989-2033），内部全部落库点 `if err != nil { log.Printf }` 包裹（SaveSuccess / DeleteSuccess / SaveRefused / AppendLog），返回 nil 恒成立；`:100` 处 `_ = s.RemoveDone("acct1",1)` 仅出现在测试 `TestStoreFailuresLogged`（scheduler_test.go:100），非产品代码吞错点。
- 手动失效分支与自动链**同文案**：`"账号 <acct>: 教务令牌失效，自动重登中"` 手动（handler.go:352 select / :433 exit）与自动（scheduler.go:1504/:1614）逐字符一致；日志动作维度 `select`/`exit` 可区分（handler.go 全部 exit 动作 :433/:442/:449 + scheduler.go:2029 手动退选成功）。

### 3. B110-01 审计链持续盯守（第九轮）—— 维持（零漂移）

- 手动失败三分支 AppendLog **六处在位，零漂移**：
  - 报名（select）：:352 失效 / :364 read 类 / :371 其余业务失败（handlers 全路径 Read 实证在 R118 行号体系内）
  - 退选（exit）：:433 失效 / :442 read 类 / :449 其余业务失败
  六处全部 `if err != nil` 包裹记日志。
- 成功路径审计行未回归：MarkDone :1976（`AppendLog(acct, classID, "select", msg, true)` 读实证）+ SaveSuccess :1973；RemoveDone :2020 DeleteSuccess + :2026 SaveRefused + :2029 `AppendLog(...,"exit","手动退选成功...",true)` 读实证。
- 自动链失败族审计行在位：失效 :1504 / 风控 :1558 / 实时复核失效 :1614 / 普通失败（含 read 类）:1662 / 满员 :1770。
- **零吞错点穷举扫描**：grep `_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken)` 全仓（含测试）**零命中**；契约 17（落库失败记日志绝不静默吞错）全仓持续成立。

### 4. OBSERVE-117-01 知识位盯守（第二轮）—— 确认在位

- **关键性质核验**：maybeRelogin 写回侧（scheduler.go:1263-1274）成功分支先清内存标记（:1259-1262），**随后**才重新取当前注册表客户端（:1265 `client, ok := s.clients.ClientFor(acct)`）并读其 `.Token()`（:1266 `tok := client.Token()`），非空才 `UpdateIDToken(acct, tok)` 落库（:1269）。
- **本轮补充取证（结构性保证链完整）**：
  1. `ReloginIfNeeded`（zhidao/client.go:602-613）成功路径调 `c.Login(acct, pwd)`；`Login` 成功后在 `c.mu` 内更新 `c.token = j.Token` 且同步 `c.cookies["zd_edu_cookie"]`（:387-395）——因此重登完成后同一实例 `.Token()` 已为新值，与写回侧取当前 token 语义自洽。
  2. `UpdateIDToken`（store.go:56-59）`UPDATE credentials SET id_token=?` 按 account 主键 UPDATE，值来自当前注册表客户端 token——不论客户端指针身份如何，落库值恒为重登后新 token，**不串旧身份/旧 token**。
  3. 既有残余面分析维持：删号同名重建场景下旧重登 goroutine 成功写回只清新身份的回收标记（tokenValid/reloginFail），不写回旧 token；残余为展示层瞬态，下轮探测/重登自愈。设计合理无需改动。
- 知识位关键性质持续在位；若未来改动破坏「落库取当前注册表 token」需警觉（延续观察）。

### 5. B110-02 观察复核 —— 维持

`reloginAt` 写（:1231 决策侧 `time.Now()` / :1261 成功分支刷新 `time.Now()`）与读（:1219 退避窗口 `time.Since(t)` / :1227 30s 节流 `time.Since(t)`）**同基本地钟**：Read 完整函数实证，两写两读均在本地钟 `time.Now()` 基下；本地钟仅用于「退出到下一次尝试的间隔」相对比较，不参与开窗点判定/时间基对齐（后者归 `nowAligned`）。无混用，维持观察。

### 6. O105-01 抖动基线 —— 通过（定向 race 全绿）

- `socketPreheat()`（zhidao/client_test.go:25-30）：`net.Listen("tcp","127.0.0.1:0")` 立即 Close，逐字符实证；调用点 :40（TestMain）/ :52（loginMockServer）+ captcha_test.go:17/:52/:76 + sanitize_test.go:97 全在位。
- `readyProbe` 双版本：api 版（handler_test.go:185-，10 次×200ms 轮询 + 显式 2s 超时总窗口 ~2s，调用 :139）；zhidao 版（client_test.go:88-，同宽栅栏，调用 :78/:164）；accounts 版（manager_test.go:26-，三处调用）。逐字符零漂移。
- **定向 `go test -race -count=1` 实测全绿**（详见验证表）：scheduler 15.062s / zhidao 2.341s / api 18.423s / 身份防线家族定向 test 全绿——**零 connectex 抖动残余、零超时**。

### 7. OBSERVE-117-01 知识位 + 新契约角度（本轮自选）：accounts 管理器注册表生命周期 + doRequest token/cookie 通道 + /state+/electives+/logs 字段级对照 —— 通过（无新缺陷）

**accounts 管理器注册表生命周期（综合走查零漂移）**：
- `ensure`（:102-112）分配/注册（client + order）；`ClientFor`（:115-120）锁内读取；`Remove`（:125-136，读实证完整函数）memory-first 摘除 client + order——与「删账号必须 memory-first」契约 4 对齐，Remove 后调度器 `ClientFor` 立即不 ok，在飞链后续身份复核全部失败静默放弃。
- `Relogin`（:166-175）先锁内取 client 再 `gateWait()`（B42-01 全局 doLogin 频率闸门，注释「受 GatePump 2026-09-23 修复」）再 `ReloginIfNeeded`——R40 证实「接口→真实实现收敛」知识点位。
- `gateWait`（:49）/ `gateTryAcquire`（:223 非阻塞）/ `LoginByPassword`（:243 收口）/ `Restore`（:295 重启恢复 + 解密失败留痕 :306）/ `ResetGateForTest`（:82 仅测试）全在位。锁序与调度器身份防线衔接完整。

**doRequest token/cookie 双通道契约（B101-01 相关深度复核）**：
- `doRequest`（client.go:414-468）：锁内快照 token+cookies → URL 拼 `?idToken=`（:422）→ Cookie 头统一注入 `zd_edu_cookie` 等全部键（:430-435，注释明确平台鉴权严格按 Cookie 头比对、缺 Cookie 即使带 idToken 也被拒）→ `httpDo`（:444）→ 连接层失败 `sanitizeError` 剥 URL（:450）→ code=-1 归 `ErrUnauthorized`（:464-465）。与 CLAUDE.md「token 真实机制」契约一致。
- `httpDo`（:478-492）：只对 `isConnErrRetryable`（dial/write，:496-505）重试一次，read 错误/业务/取消原样上抛——`SelectClass/ExitClass` 幂等防线在位（F52-M4/M5 自愈族）。

**前后端契约字段级对照（/state + /electives + /logs）**：`handleElectives`（学院端 admin 透传 + 学生会话）经 ElectivesSnapshotFor（决策 8：`len(acctTargets)>0` 目标账号专属帧 + 过期专属帧绝不回退全局帧，读实证 :761-810）→ 无专属帧才 ProbeForAccount；`handleState` 三判据单源 windowClosedLocked；Logger 经 LoadLogs 窗口查询（store.go:223-，max(id)-20000 索引走位、limit 钳制）。前端契约字段（open_time/open_time_known/window_opened/window_closed/token_valid/courses/logs）与后端结构体（SchedulerState/LogEntry）字段名对齐——无缺口。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline -8` / `git rev-parse HEAD` | HEAD=`71b2a95`（R118 收尾），最近 8 条均 docs(review) 收尾 |
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 `57bf401` 一条（B110-01 审计修复），COUNT=1（零产品改动链第十四轮） |
| `git status --short --branch` | 空（clean） |
| `go build ./...` | BUILD_OK |
| `go vet ./...` | VET_OK（零输出） |
| `go test -race -count=1 ./internal/scheduler/` | ok 15.062s，exit 0（全量 race） |
| `go test -race -count=1 ./internal/zhidao/` | ok 2.341s，exit 0（全量 race，零 connectex 残余） |
| `go test -race -count=1 ./internal/api/` | ok 18.423s，exit 0（全量 race） |
| `go test -race -count=1 -run 'TestDeletedAccountRebuiltSameNameChainDrops\|TestMaybeReloginDeletedAccountSkipsMaps\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestStoreFailuresLogged\|TestManualElectiveFailureAppendsLog\|TestManualElectiveReadErrAppendsLog\|TestAdminStatsWindowOpenedUsesScheduler' ./internal/scheduler/ ./internal/api/` | 后台完成 exit 0（身份防线家族 + 审计链双测试 + 回归守护全绿） |
| Grep 证据族（sameClientFor 7 点 / 写点 12 类 / maybeRelogin 6 入口 / accountExists 4 路 / 手动 AppendLog 6 处 / exit 动作维度 / 零吞错穷举扫描 / 契约20 轮次前缀扫描） | 全命中，行号与任务清单零漂移；零吞错与契约20 扫描全空 |

## 已核无缺陷清单

- 手动路径 TryAcquireSubmit `(nil,false)` 时 handler 侧 `defer release()` 未注册、无 nil 调用 panic；成功路径 deferred release 幂等（sync.Once）。
- `releaseFullIfFreedLocked` 对空快照/课程不在快照恒不解封，窗口关闭防轰炸残留闭环（注释与实现一致）。
- PurgeAccount 成败齐全性（scheduler.go:495-519）：12 类写点对应 map 键 + state.Courses 该账号行 + openTimeDetected 识别槽一并清除。
- maybeRelogin 决策侧入口先 ClientFor 存在性复核（:1208），写回侧 :1254 复核，双侧闭合；Writeback UpdateIDToken 取当前注册表 token（OBSERVE-117-01 关键性质在位）。
- doRequest code=-1 → ErrUnauthorized（统一鉴权归口）；httpDo 只重试 dial/write、read 错误不重试（幂等防线）。
- 探测节流三件套（probing 单飞 + 30s 节流闸门 + probeSem cap 4）在位（读 :1045-1089 区）。
- 契约 17（token/密码只显前 8 位）：maskedToken（:1334，len>8 才截前 8，否则 "***"）+ :1283 `new token %s...` 脱敏在位。
- 契约 20（注释无轮次前缀标签）：全仓 `（第 N 轮）` 扫描零命中（含测试）。

## 结论

1. **身份防线矩阵第三十四轮闭合**：`20c5882..HEAD` 后端仅 57bf401（B110-01 审计修复），**零产品改动链延续第十四轮，COUNT=1**。sameClientFor 7 调用点行号零漂移；写点全家福 12 类逐一对应防线（含偶发写点专项 MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 四类零裸写复证）；手动四路 accountExists + maybeRelogin 双侧（决策侧 :1208 + 写回侧 :1254）全在位；六 err 归并分支身份复核成族齐备（B41-01 家族 10 测试定向 -race 全绿）。**矩阵保持完全闭合，无新证据够格立条。**
2. **OBSERVE-111-01/112-03 盯守（第八轮）**：维持。handler.go:377/:457 `_ = MarkDone/RemoveDone` 恒 nil 非吞错（函数体落库点全日志化，:100 的 `_ =` 仅测试内，非产品吞错点）；失效文案与自动链同文案、select/exit 动作维度可区分。**零漂移。**
3. **B110-01 审计链（第九轮）**：手动失败三分支 AppendLog 六处在位，成功路径审计行（MarkDone/RemoveDone）未回归，自动链失败族审计行齐位，零吞错穷举扫描全空。**零漂移。**
4. **OBSERVE-117-01 知识位（第二轮）**：确认在位。重登写回侧取当前注册表 token 落库、不串旧身份，本轮补取证结构性保证链完整（ReloginIfNeeded→Login 更新实例 token→UpdateIDToken 按主键写新值）。
5. **B110-02**：reloginAt 本地钟读写同基自洽，维持观察。
6. **O105-01 抖动基线**：socketPreheat/readyProbe 逐字符零漂移；定向 -race 全量（scheduler/zhidao/api）首轮即绿，零抖动残余。
7. **新契约角度（accounts 注册表生命周期 + doRequest 双通道 + 前端契约字段对照）**：零漂移，无新缺陷。

**审视结论：无 CRITICAL/MAJOR/MINOR，新增 OBSERVE = 零。稳定期价值 = 确认无新证据够格立条，全部延续项维持零漂移。**