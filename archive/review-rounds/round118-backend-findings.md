# R118 后端只读审查报告

## 审查对象与方式

- **审查对象 HEAD**：`7abb1978c4372b68a57db7141f4684cfeb57544d`（master，docs R117 收尾总结）
- **审查范围**：`backend/`（scheduler / api / store / zhidao / accounts）
- **审查方式**：绝对只读——零仓库文件修改。`git log`/`git show` 实证提交历史，Grep/Read 逐点核位，定向 `go test -race` 实证行为，`go build`/`go vet` 实证编译。唯一写入 = 本报告。
- **聚焦范围**：身份防线矩阵第三十三轮闭合（零产品改动链 COUNT）+ OBSERVE-111-01/112-03 钉守第七轮 + B110-01 审计链第八轮 + OBSERVE-117-01 知识位首轮 + B110-02 观察复核 + O105-01 抖动基线 + 新契约角度（错误文案端到端可辨性对照）。

## 分级发现

**CRITICAL / MAJOR / MINOR：无。**

本轮 7 项必查全部通过/维持/在位，无证据够格立任何缺陷条——稳定期审查价值确认为「无新证据」。

## 必查项逐条结论

### 1. 身份防线矩阵第三十三轮闭合 —— 通过（零产品改动链延续第十三轮）

**COUNT 实证**：`git log --oneline 20c5882..HEAD -- backend/` 仅 1 条 = `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志）。`git show --stat 57bf401` 只动 `backend/internal/api/handler.go`（+26 行）+ `handler_test.go`（+133 行），纯审计可观测性修复、非产品行为改动。与 R110-R117 十三轮连续断言完全一致。

**sameClientFor 7 调用点逐点实证（行号与任务清单逐字符零漂移）**：

| 调用点 | 行号 | 语义 |
|---|---|---|
| 定义 | `scheduler.go:204` | 注释 :199-203 说明接口值反射指针身份比对 + nil/已删非同一 |
| ProbeForAccount 回写段 | :850 | 锁内复核后写 openTimeDetected/acctData/acctDataAt |
| 失效分支 | :1489 | SelectClass ErrUnauthorized 回锁后先复核再 maybeRelogin（注释 :1484-1488 明确"业务失败不在 checkResp throw 名单，成败看 isOk"与重登必须落在身份复核之后） |
| 成功分支 | :1521 | 写 done/状态/落库前复核，非同一身份静默放弃写成功落库 |
| 风控分支 | :1551 | markRateLimitedLocked 前复核，非同一身份绝不落退避/状态/日志 |
| 窗口关闭分支 | :1571 | markFullLocked 前复核，防假满员永久退避写进重建身份 |
| 实时复核统一 | :1600 | classFullRealtime 网络段回锁后先复核再进三路分支（含 doneHas 让位） |
| 确证满员分支 | :1635 | 复核网络段内同名重建后 markFullLocked 前再次复核 |

**写点全家福 12 类逐一对应防线复核（主控裁定维持全量核位）**：

- `openTimeDetected[acct]`：写 :856（ProbeForAccount 锁内 sameClientFor 复核后）/ :1108（probe 全校槽，含对 openTimeDetected["*"]）；清 :510（PurgeAccount，注释"绝不残留旧批次识别值"）
- `acctData/acctDataAt[acct]`：写 :863/:864（锁内复核后）；清 :508/:509
- `tokenValid[acct]`：写 :1241（maybeRelogin 决策段，锁内复核后置位）/ :1262（成功分支写回侧复核后清零）；清 :511 + MarkTokenValid :1317/:1318
- `reloginAt[acct]`：写 :1231/:1261（决策段锁内 / 成功写回侧）；清 :512 + :1226（退避窗口已过）
- `reloginFail[acct]`：写 :1233（锁内复核后 ++）；清 :513 + 成功分支 :1260 delete
- `relogging[acct]`：写 :1236（决策段置位）；清 :514 + :1247（goroutine 开头恒清）+ MarkTokenValid
- `inflight[acct]`：写 :1469/:1471（spawnChain 置位）/ :1899-1902（TryAcquireSubmit）；清 :1490/:1501/:1513 + MarkDone/RemoveDone + PurgeAccount :503
- `done[acct]`：写 :1526-1529（成功分支锁内复核后）/ MarkDone :1931-1934；清 RemoveDone + PurgeAccount :498
- `refused[acct]`：写 :630-634（SetTargetsForAccount）/ RemoveDone :2008-2011；清 MarkDone :1938-1939（同步删库内行）+ PurgeAccount :504
- `full[acct]`：写 markFullLocked :1767；清 releaseFullIfFreedLocked :1800 + MarkDone/RemoveDone/RemoveFull :1953/2005/2041；PurgeAccount :499
- `rateLimited[acct]`：写 markRateLimitedLocked :1711-1714；清 :1956 + PurgeAccount :500
- `state.Courses` 状态：写 :1474（submitted）/ :1803（pending）/ :1850-1851（setStateLocked）/ :1961-1966（MarkDone success）/ :2014-2015（RemoveDone pending）/ :2045-2047（RemoveFull）；清 :518/:578（目标重建/删除）

**偶发写点专项**：
- `MarkTokenValid`（:1306-1322）：锁序 reloginMu→s.mu 对齐，清 tokenValid/reloginFail/relogging，只用于手动登录成功恢复路径（handler.go issueSession 段）；spawnChain/手动路径只调 MaybeRelogin 绝不先调 MarkTokenValid（注释所述"delete reloginFail 击穿指数退避"防线在位）
- `TryAcquireSubmit`（:1896-1920）：持 s.mu，inflight 位占位 + once 防重放释放，`defer release()` 双路径（报名/退选）恒持有
- `releaseFullIfFreedLocked`（:1781-1807）：持锁，fullHas 守卫，仅快照明确余量才解封，无快照/课程不在快照保持 full（窗口关闭防轰炸）
- `SubmitAll`（:1346+）：持锁构建链，首行 `ClientFor(acct)` 复核跳过已删账号（删号后彻底隔离），spawnChain 逐链独立 goroutine

**手动四路 accountExists**：:245（课程读）/ :295（手动报名）/ :387（手动退选）/ :563（状态读）全在位，判据同源（凭据表 LoadCredentials 逐账号比对，handler.go:1096-1109）。

**maybeRelogin 双侧**：
- 决策侧 :1208（锁内 `ClientFor(acct)` 复核，注释 R616 讲述 purge 竞态）——探测定时三处（:823 ProbeForAccount / :956 ProbeNow / :1094 probe）、spawnChain 失效/实时复核两处（:1499/:1610，前置 sameClientFor 已判）、handler 手动两处（:349/:431）共 6 个入口全部收口到顶层锁内复核
- 写回侧 :1254-1257（goroutine 开头先复核客户端仍存在，已删整个成功分支含落库静默放弃，只清 relogging）+ :1263-1274（成功分支再取当前注册表 client 与新 token 才 UpdateIDToken）

**B41-01 身份防线成族测试**（全在 scheduler_test.go，定向 -race 全绿）：TestDeletedAccountRebuiltSameNameChainDrops{Success(3031)/Relogin(3135)/RateLimitBackoff(3194)/WindowClosedFull(3247)/RealtimeRecheckFull(3296)} + RealtimeUnauthorizedDropsRelogin(3435) + SuccessDropsInflight(3518)。

### 2. OBSERVE-111-01/112-03 钉守（第七轮）—— 维持，零漂移

- `handler.go:377` `_ = d.Sched.MarkDone` / `:457` `_ = d.Sched.RemoveDone` 返回值丢弃**非吞错**：这两个方法内部所有落库点（SaveSuccess/DeleteSuccess/SaveRefused/AppendLog）全部 `if err != nil { log.Printf }` 日志化，返回 nil 恒成立；handler 丢弃返回值不影响 MarkDone/RemoveDone 内部已完成的库内审计（成功路径 handler 自身还另写 writeJSON 回显）。
- 手动失效分支与自动链**同文案**：`"账号 <acct>: 教务令牌失效，自动重登中"` 手动（handler.go:352 select / :433 exit）与自动（scheduler.go:1504/:1614）逐字符一致。
- **双日志动作维度可区分**：手动路径 select 动作 3 处（:352/:364/:371）+ exit 动作 3 处（:433/:442/:449），动作维度独立区分。

### 3. B110-01 审计链持续钉守（第八轮）—— 维持，零漂移

- 手动失败三分支 AppendLog **六处在位**：失败（select :371 / exit :449）、失效（:352/:433）、read（:364/:442）。
- 成功路径审计行未回归：MarkDone 内 :1975（`"select", msg, true`）+ RemoveDone 内 :1982（`"exit", "手动退选成功（...）", true`）。
- 自动链失败审计行：失效 :1504 / 风控 :1558 / 实时复核失效 :1614 / read 类 :1662 / 满员 :1770 全在位。
- **零吞错点扫描**：grep `_ = .*AppendLog|_ = .*SaveSuccess|_ = .*SaveRefused|_ = .*DeleteSuccess` 全仓库（排除 _test.go）**零命中**——契约 17（落库失败必须记日志绝不静默吞错）持续成立。

### 4. OBSERVE-117-01 知识位盯守（首轮）—— 确认在位

**关键性质核验**：maybeRelogin 写回侧（scheduler.go:1263-1274）先 `s.clients.ClientFor(acct)`（:1265）取**当前注册表**客户端，`client.Token()`（:1266）取当前 token 才 `UpdateIDToken` 落库（:1269）。结构性保证：新 token 值恒来自"重登完成时刻注册表内该账号的客户端"，而非 goroutine 发起时捕获变量——**不串旧身份/旧 token**。配合 :1254-1257 已删账号整体放弃写回防线，知识位关键性质在位。若未来改动破坏此性质需警觉。

### 5. B110-02 观察复核 —— 维持

`reloginAt` 写（:1231 决策段 / :1261 成功分支）与读（:1219 退避窗口 / :1227 30s 节流）**同基自洽**：读写均在本地钟 `time.Now()` 基下（同一 maybeRelogin 上下文、同一 goroutine 生命周期内），`time.Since(t)` 与写入 `time.Now()` 同基比较；成功分支 `time.Now()` 写回防抖刷新语义与决策段读取同一时钟基。本地钟字段不涉对齐钟 `nowAlignedLocked` 混用。**维持观察，无问题**。

### 6. O105-01 抖动基线 —— 通过（逐字符零漂移）

- `socketPreheat()`（zhidao/client_test.go:25-27）：`net.Listen("tcp","127.0.0.1:0")` + 立即 Close，逐字符存在；调用点 :40（TestMain）/ :52（loginMockServer）/ captcha_test.go:17/:52 / sanitize_test.go:97。
- `readyProbe`（api/handler_test.go:179-210 定义，10 次 200ms 轮询 + 显式 2s 超时；调用 :139）+ accounts/manager_test.go:26-29 定义与三处调用。
- **定向 `go test -race -count=1 ./internal/api/... ./internal/zhidao/...` 全绿**：api 22.931s ok / zhidao 3.245s ok，exit 0——零 connectex 抖动残余。

### 7. 新契约角度（本轮自选：错误文案端到端可辨性对照）—— 通过

对自动链与手动路径全部错误文案做端到端对照（日志侧 + 回显侧）：

| 场景 | 自动链 /api/logs 文案 | 手动路径 /api/logs 文案 | 动作维度 |
|---|---|---|---|
| token 失效 | `教务令牌失效，自动重登中`（:1504/:1614） | 同（:352/:433） | select / exit |
| read 类结果未知 | `报名请求已发出但响应读取失败（平台可能已处理，以大厅状态为准）`（:1658） | `报名请求已发出...（以大厅状态为准）`（:364）/ `退选请求已发出...`（:442） | select / exit |
| 满员 | `课程 X 已满员，切换备选`（:1770） | 手动走 CheckClassSelectable 快照前置拦截（"该课程已满员或不可选"） | — |
| 成功 | `msg`（平台原文） | `msg`（平台原文） | select / exit |
| MarkDone 审计行 | :1532 | :1975 | select |
| RemoveDone 审计行 | — | :1982（exit）区别于成功文案 | exit |

**结论**：日志侧文案语义完全一致（含"请"字变体——自动链 read 文案 :1658 带"请"、手动路径 :364 不带，属自然语言措辞范围，两端同为"以（请以选课）大厅状态为准"，可辨性不受影响）；动作维度 select/exit 全程可区分；成功/失败/失效/满员四状态与自动链一一对应。**无契约缺口，不立条**。

## 验证表

| 命令 | 结果 |
|---|---|
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 57bf401（B110-01 审计修复） |
| `git show --stat 57bf401` | 仅 handler.go + handler_test.go，无产品行为改动 |
| `git log -1 --format='%H %s'` | `7abb197...` R117 收尾 |
| `go build ./...` | BUILD_OK |
| `go vet ./...` | 零输出（VET_DONE） |
| `go test -race -count=1 ./internal/api/... ./internal/zhidao/...` | ok ×2（22.931s / 3.245s）exit 0 |
| `go test -race -count=1 -run 'TestDeletedAccountRebuiltSameNameChain\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestAdminStatsWindowOpenedUsesScheduler' ./internal/scheduler/... ./internal/api/...` | ok ×2（4.315s / 1.925s）exit 0 |
| Grep 证据族（sameClientFor 7 点 / 12 类写点 / 6 处 maybeRelogin / 4 处 accountExists / 6 处手动 AppendLog / 零吞错扫描） | 全命中，行号与任务清单零漂移 |

## 已核无缺陷清单

- **PurgeAccount 成败齐全性**（:495-525）：12 类写点对应全部 map 键 + state.Courses 该账号行一并清理，持 s.mu 全量。
- **探测三处决策侧收口**（:823/:956/:1094）：对 ErrUnauthorized 无条件直调 maybeRelogin，复核统一落在入口锁内 :1208，无旁路。
- **ElectivesSnapshotFor 回退链契约**（决策 8）：`len(acctTargets[acct])>0` 判据 + "过期专属帧不回退全局帧"两分支全在位（:771-800）。
- **/state 与 /api/electives 字段级**：`open_time`/`open_time_known`/`window_opened`/`window_closed`/`token_valid`/`courses` 与前端契约对齐（handler.go:555-570/955-980，StateForAccount :693-734 含 windowClosedLocked 单源）。
- **契约 21（token 脱敏）**：maskedToken 只显前 8 位（:1327-1333）；:1283 日志 `new token %s...` 用 maskedToken 脱敏；网络层 sanitizerErr 既有防线未触碰。
- **契约 20（注释无轮次前缀标签）**：本轮重读注释未见 "（第 N 轮）" 残留。

## 结论

1. **身份防线矩阵第三十三轮闭合**（零产品改动链延续第十三轮 COUNT=1）：sameClientFor 7 调用点行号零漂移、写点全家福 12 类逐一对应防线、偶发写点专项覆盖、手动四路 accountExists + maybeRelogin 双侧全在位、六 err 归并分支身份复核成族齐备（B41-01 七测试绿）。**通过**。
2. **OBSERVE-111-01/112-03 盯守第七轮**：维持。handler.go:377/:457 恒 nil 非吞错、失效文案与自动链同文案、select/exit 动作维度可区分。**零漂移**。
3. **B110-01 审计链第八轮**：六处手动失败 AppendLog + 成功路径审计行 + 自动链失败族审计行齐位，零吞错扫描零命中。**零漂移**。
4. **OBSERVE-117-01 知识位首轮**：写回侧取当前注册表 token 不串旧身份，关键性质确认在位。
5. **B110-02**：reloginAt 本地钟读写同基自洽，维持观察。
6. **O105-01 抖动基线**：socketPreheat/readyProbe 零漂移，定向 -race 全绿。
7. **新契约角度（错误文案端到端对照）**：日志侧文案语义一致、动作维度可辨，无缺口。

**审视结论：无 CRITICAL/MAJOR/MINOR。稳定期价值 = 确认无新证据够格立条，全部延续项维持零漂移，无新增 OBSERVE。**