# R121 后端只读审查报告（身份防线矩阵第三十六轮）

## 头部

- **审查对象**：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`（Go 后端 `backend/`）
- **审查 HEAD**：`90a76d1`（R120 收尾总结，master，`git status --short --branch` 仅 `round121-frontend-findings.md` 未跟踪为并行前端代理产物、无产品改动）
- **审查方式**：绝对只读——零仓库文件修改。`git log` 实证提交历史、`grep`/`Read` 逐点核位、后台定向 `go test -race` 实证行为、`go build`/`go vet` 实证编译。唯一写入 = 本报告文件。
- **聚焦范围**：聚焦清单 6 项全量执行（身份防线矩阵第三十六轮闭合 / OBSERVE-117-01 知识位第四轮 / OBSERVE-111-01+112-03 盯守第十轮 / B110-01 审计链第十一轮 / O105-01 抖动基线 / 新契约角度 = 错误文案端到端可辨性对照）。

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**
**OBSERVE：延续 4 项（111-01/112-03 维持、117-01 知识位第四轮确认在位、O105-01 维持）**

本轮 6 项必查全部通过/维持/在位，无证据够格立任何缺陷条。（注：`round121-frontend-findings.md` 为并行前端代理产物，非本报告。）

## 必查项逐条结论

### 1. 身份防线矩阵第三十六轮闭合 —— 通过（零产品改动链延续第十六轮，COUNT=1）

**COUNT 实证**：`git log --oneline 20c5882..HEAD -- backend/` 仅 1 条 `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志），与 R110-R120 连续十五轮同一条。**零产品改动链延续第十六轮，COUNT=1**。

**sameClientFor 7 调用点逐点实证（行号与任务清单逐字符零漂移）**：

| 调用点 | 行号 | 身份防线角色 | 实证 |
|--------|------|--------------|------|
| 定义 | `scheduler.go:204` | 注释 :199-203 接口值反射指针身份比对 + nil/已删非同一 | Read 实证 `current==nil→false`（:206）+ `clientIdentity` 比较（:209） |
| ProbeForAccount 回写段 | :850 | 网络往返后锁内复核，非同一身份整体放弃快照/识别槽写回 | Read 实证（:851-854 区） |
| 失效分支 | :1489 | 复核不通过先 delete(inflight) 再 return；maybeRelogin 调用落在复核之后（:1499） | Read 实证 :1482-1510 |
| 成功分支 | :1521 | 写 done/state/SaveSuccess/AppendLog 前复核，非同一身份静默放弃写成功落库 | Read 实证 :1521-1543 |
| 风控分支 | :1551 | markRateLimitedLocked 前复核 | Read 实证（:1546-1559 区） |
| 窗口关闭分支 | :1571 | markFullLocked 前复核，防假满员永久退避写进重建身份 | Read 实证（:1565-1578 区） |
| 实时复核统一 | :1600 | classFullRealtime 网络段回锁后先复核再进三路分支 | 实证（:1596-1599 注释与五分支对称） |
| 确证满员分支 | :1635 | doneHas 胜利状态让位后、markFullLocked 前复核 | 实证（:1623-1640 区） |

**写点全家福 12 类抽查 5 类**（主控维持全量核位，本轮抽查后读代码实证持锁 + 身份/存在性复核后写入）：
- `tokenValid[acct]`：写 :1241 置位（maybeRelogin 决策侧 :1208 ClientFor 存在性复核之后）、:1262 清零（重登写回侧 :1254-1258 复核之后）；清 :511 + MarkTokenValid :1316。双闭合。**读实证**：Read :1198-1242 决策侧锁序 reloginMu→s.mu + :1208 `ClientFor(acct)` 复核（已删 unlock return 不写任何 map）；Read :1306-1319 MarkTokenValid 同锁序 delete 三标记。
- `inflight[acct]`：写 :1469-1471（spawnChain 锁内置位）/ :1905（TryAcquireSubmit）；清 :1490（失效分支复核前）/ :1501/:1513（锁内）+ MarkDone :1950-1951 + RemoveDone :2002-2003 + PurgeAccount :502。**读实证**：Read :1896-1914 TryAcquireSubmit 持 s.mu + sync.Once 幂等释放；Read :1521-1524 成功分支复核失败静默放弃写 done/状态/库行。
- `done[acct]`：写 :1526-1529（成功分支 sameClientFor 复核后）/ MarkDone :1931-1934（ClientFor 复核 :1927 后）；清 RemoveDone :1999-2000 + PurgeAccount :499。**读实证**：Read :1922-1982 MarkDone 全函数持锁 + ClientFor 复核 + 落库点全 `if err != nil { log.Printf }` 包裹。
- `full[acct]`：写 markFullLocked :1760-1767（封面 sameClientFor 族 :1555/:1575/:1639）；清 releaseFullIfFreedLocked :1800 + MarkDone :1953-1954 + RemoveDone :2005-2006 + PurgeAccount :500。**读实证**：Read :1781-1811 releaseFullIfFreedLocked 持锁 + fullHas 守卫 + 空快照恒保持 full（窗口关闭防轰炸）。
- `rateLimited[acct]`：写 markRateLimitedLocked :1710-1714（对齐钟写，仅在 :1551 风控分支同身份复核后）；清 MarkDone :1956-1957 + PurgeAccount :501。实证同上族。

**偶发写点专项四类全部 Read 实证（零裸写）**：
- `MarkTokenValid`（:1306-1319）：锁序 reloginMu→s.mu 对齐 maybeRelogin 决策段；只清 tokenValid/reloginFail/relogging 三个恢复标记、不写任何业务身份态；唯一合法入口 = issueSession 手动登录成功（handler.go:226）。
- `TryAcquireSubmit`（:1896-1914）：持 s.mu，inflight 位占位 + `sync.Once` 幂等释放；`(nil,false)` 分支 handler 侧 `defer release()` 未注册不 panic（handler.go:318-323/:410-415 走读实证）。
- `releaseFullIfFreedLocked`（:1781-1811）：持锁；fullHas 守卫；只有快照明确余量（MaxCount>0 且 Selected<Max）才解封，空快照/课程不在快照恒保持 full（窗口关闭防轰炸）。
- `SubmitAll`（:1342-1380）：持锁构建链，首行 per-account `ClientFor(acct)` 存在性过滤（:1355，已删账号彻底跳过）；`lastSubmit` 用 `nowAlignedLocked()`（:1346）；冷启动无目标一次性警告（:1371-1374）。尾部统一走 spawnChain 身份防线。

**手动四路 accountExists**（handler.go，全部实证在位，判据同源 `accountExists` :1096-1109 LoadCredentials 逐账号比对）：
- :245 课程读（handleElectives）/ :295 手动报名（handleElectiveSelect）/ :387 手动退选（handleElectiveExit）/ :563 状态读（handleState）。
查无账号整体拒绝，四路全覆盖。

**maybeRelogin 双侧**：
- 决策侧 :1208 锁内 `ClientFor(acct)` 存在性复核（不通过即 unlock return，不写任何 map）。探测定时三处直调入口实证：ProbeForAccount :823 / ProbeNow :956 / probe :1094 + spawnChain 失效分支 :1499（前置 :1489 sameClientFor）/ 实时复核失效 :1610（前置 :1600 sameClientFor）+ handler 手动两处 MaybeRelogin :349/:431——全部入口收口到顶层锁内复核。
- 写回侧 :1254-1258（goroutine 开头先复核客户端仍存在，已删则整个成功分支含 `UpdateIDToken` 落库静默放弃、只清 relogging）+ :1263-1274（成功分支再取当前注册表 client 及其 `.Token()` 才落库）。

### 2. OBSERVE-117-01 知识位盯守（第四轮）—— 确认在位

- **关键性质核验**：maybeRelogin 写回侧（scheduler.go:1263-1274）成功分支先清内存标记（:1259-1262），随后才重新取当前注册表客户端（:1265 `client, ok := s.clients.ClientFor(acct)`）并读其 `.Token()`（:1266），非空才 `UpdateIDToken(acct, tok)` 落库（:1269）。Read 实证逐行在位。
- 支撑链持续成立：`ReloginIfNeeded`（zhidao/client.go:602-613）成功路径调 `c.Login(acct, pwd)`；`Login` 成功后在 `c.mu` 内更新 `c.token` 且同步 `c.cookies["zd_edu_cookie"]`（client.go:392-394）——重登完成后同一实例 `.Token()` 已为新值。`UpdateIDToken`（store.go:56-59）按 account 主键 UPDATE，落库值恒为重登后新 token，**不串旧身份/旧 token**。
- 活化条件（未来改动破坏「落库取当前注册表 token」关键性质）未触发，连续四轮零漂移。

### 3. OBSERVE-111-01/112-03 盯守（第十轮）—— 维持（零漂移）

- `handler.go:377` `_ = d.Sched.MarkDone(...)` / `:457` `_ = d.Sched.RemoveDone(...)` 返回值丢弃**仍非吞错**：Read 两函数全函数实证（MarkDone :1922-1982 / RemoveDone :1989-2035），内部全部落库点 `if err != nil { log.Printf }` 包裹（SaveSuccess :1973 / DeleteSuccess :2020 / SaveRefused :2026 / AppendLog :1976/:2029），返回 nil 恒成立；`:100` 处 `_ = s.RemoveDone("acct1",1)` 仅出现在测试 `TestStoreFailuresLogged`（scheduler_test.go:100，断言落库失败必记日志），非产品代码吞错点。
- 手动失效分支与自动链**同文案**：`"账号 <acct>: 教务令牌失效，自动重登中"` 手动（handler.go:352 select / :433 exit）与自动（scheduler.go:1504/:1614）逐字符一致；日志动作维度 `select`/`exit` 可区分（handler.go 全部 exit 动作 :433/:442/:449 + scheduler.go:2029 手动退选成功）。**零漂移。**

### 4. B110-01 审计链持续盯守（第十一轮）—— 维持（零漂移）

- 手动失败三分支 AppendLog **六处在位，零漂移**：
  - 报名（select）：:352 失效 / :364 read 类 / :371 其余业务失败
  - 退选（exit）：:433 失效 / :442 read 类 / :449 其余业务失败
  六处全部 `if err != nil` 包裹记日志。
- 成功路径审计行未回归：MarkDone :1976（`AppendLog(acct, classID, "select", msg, true)`）+ SaveSuccess :1973；RemoveDone :2020 DeleteSuccess + :2026 SaveRefused + :2029 `AppendLog(...,"exit","手动退选成功...",true)`。Read 全函数实证。
- 自动链失败族审计行在位：失效 :1504 / 风控 :1558 / 实时复核失效 :1614 / 普通失败（含 read 类）:1662 / 满员 :1770。
- **零吞错点穷举扫描**：grep `_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)` 全仓（含测试）**零命中**；契约 17（落库失败记日志绝不静默吞错）全仓持续成立。

### 5. O105-01 抖动基线 —— 通过（定向 race 全绿）

- `socketPreheat()`（zhidao/client_test.go:25-30）：`net.Listen("tcp","127.0.0.1:0")` 立即 Close，逐字符实证；调用点 :40（TestMain）/ :52（loginMockServer）+ captcha_test.go:17/:52/:76 + sanitize_test.go:97 全在位。
- `readyProbe` 三版本逐字符零漂移：api 版（handler_test.go:179-216，10 次×200ms 轮询 + 显式 2s 超时总窗口 ~2s，调用 :139）；zhidao 版（client_test.go:82-，同宽栅栏，调用 :78/:164）；accounts 版（manager_test.go:26-，三处调用 :87/:164/:247，注释明确「若未来 accounts 再出冷启动 flake 第一候选即补 socketPreheat」）。
- **定向 `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` 实测全绿**（详见验证表）：zhidao 3.217s / accounts 2.090s，exit 0——**零 connectex 抖动残余、零超时、零 race**。

### 6. 新契约角度（本轮自选）：错误文案端到端可辨性对照 —— 通过（无新缺陷）

**全链路语义清点（zhidao 层产生 → scheduler/api 归并 → task_log 落库 → 前端回显）**：

| 类别 | zhidao 层产生 | scheduler 归并 | api 手动归并 | task_log 落库 | 前端回显 |
|------|---------------|----------------|--------------|---------------|----------|
| 风控 | 文案含「频繁/429/稍后重试」（scheduler.go:1672-1678 `isRateLimitError` 匹配集） | spawnChain :1556 置 failed + :1558 AppendLog | 手动路径其余业务失败 :371/:449 原文透传 | :1558 落「触发平台风控退避 30s」 | Dashboard failed 徽章「报名异常」+ c.result 原文；频控卡「分级退避 · 自动恢复」 |
| 窗口关闭 | `isWindowClosedError`（:1682-1688，匹配「关闭/未开启/报名时间/已结束」） | spawnChain :1570 分支按满员记 full :1571 复核 → markFullLocked :1768「该课程已满员，退避至下一备选」+ AppendLog :1770 | handler.go:327 `CheckClassSelectable` 预检返回「该课程已满员或不可选」（:1886 区） | :1770 落「已满员，切换备选」 | Dashboard `isFullFallback`（:35-38 `status=="failed" && result.includes("已满员")`）→ 徽章「已满员·退避备选」+ 说明「该门已满，自动退避至下一备选」 |
| 满员 | `classFullInSnapshot`（:1717 快照级 max_count/selected_count 实证判据）+ 实时复核（:1748 防御性路径） | :1456-1459 快照确认满员 → markFullLocked | 手动预检 CheckClassSelectable 拒绝 | 同上 :1770 满员行 | 同上 isFullFallback 语义簇 |
| read 类 | `IsReadErr`（zhidao/client.go:523-） | :1656-1659 统一文案「报名请求已发出但响应读取失败（平台可能已处理，请以选课大厅状态为准）」+ :1662 AppendLog | :361-367/:440-445 同款文案 + 落库 | 六处 read 类日志行（select/exit × 3 分支） | c.result 原文回显（read 文案可辨「平台可能已处理」），/api/logs 可核对 |
| 失效 | `ErrUnauthorized`（client.go:280-281 code=-1 归口，:465 `%w` 包装） | :1482 分支 → :1502 置「教务令牌失效，自动重登中」+ :1504 AppendLog + maybeRelogin | :348-356/:430-438 同文案 + MaybeRelogin | 自动 :1504 / 手动 :352/:433 同文案行 | Dashboard :490-494 token_valid 徽章「已失效 · 自动恢复中」+ c.result 回显 |

**可辨性结论**：
- 五类错误在「状态行 Result 文案 + task_log 动作/结果 + 前端徽章/说明」三个展示维度均有独立可辨语义；满员类前后端同源判据（Dashboard `isFullFallback` 与后端 markFullLocked「已满员」文案、前端手动 CheckClassSelectable 预检）对齐，无「满员 vs 窗口关闭」文案混同。
- read 类与失效类手动/自动双路径文案逐字符同源，`select`/`exit` 动作维度贯穿全链（AppendLog 动作参数 → 前端日志分组）。
- 契约 5 的真实站点文案（「不在选修报名时间范围内」「选课处理中」「选课成功」）由 zhidao 层原样透传，不做前端硬编码改写——端到端可辨性由平台文案 + 归并层「已满员」统一文案双源保证，无硬编码耦合风险。
- **结论：错误文案端到端可辨性对照通过，无新缺陷。**

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 `57bf401` 一条（B110-01 审计修复），COUNT=1（零产品改动链第十六轮） |
| `git rev-parse HEAD` | `90a76d1`（R120 收尾总结） |
| `git status --short --branch` | 仅 `?? archive/review-rounds/round121-frontend-findings.md`（并行前端代理产物，非本仓库产品改动） |
| `go build ./...` + `go vet ./...` | BUILD_VET_OK（exit 0，vet 零输出） |
| `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` | ok（zhidao 3.217s / accounts 2.090s），exit 0 |
| Grep 证据族（sameClientFor 7 点 / 写点 5 类抽查 / maybeRelogin 6 入口 / accountExists 4 路 / 手动 AppendLog 6 处 / 零吞错穷举扫描 / 契约20 轮次前缀扫描） | 全命中，行号与任务清单零漂移；零吞错扫描全空 |

**契约 20 扫描**：全仓 grep `（第 \d+ 轮|第〔0-9〕轮|\(R\d+\)` 仅命中 scheduler_test.go:1583「第 3 轮」测试轮次领域描述，产品代码零轮次前缀标签。

## 已核无缺陷清单

- 手动路径 TryAcquireSubmit `(nil,false)` 时 handler 侧 `defer release()` 未注册、无 nil 调用 panic；成功路径 deferred release 幂等（sync.Once）。
- `releaseFullIfFreedLocked` 对空快照/课程不在快照恒不解封，窗口关闭防轰炸残留闭环（注释与实现一致）。
- maybeRelogin 决策侧入口先 ClientFor 存在性复核（:1208），写回侧 :1254 复核，双侧闭合；Writeback UpdateIDToken 取当前注册表 token（OBSERVE-117-01 关键性质第四轮在位）。
- MarkTokenValid 锁序 reloginMu→s.mu 对齐、只清三恢复标记不写业务态；唯一入口 issueSession 手动登录成功。
- 手动失效分支只调 MaybeRelogin 绝不先调 MarkTokenValid（「delete reloginFail 击穿指数退避」防线注释在位，handler.go:345-347/:427-429）。
- doRequest code=-1 → ErrUnauthorized（统一鉴权归口）；sanitizeError 剥 URL（B101-01）在位。
- 契约 17（token/密码只显前 8 位）：maskedToken（:1334）+ :1283 `new token %s...` 脱敏在位。
- 错误文案五类（风控/窗口关闭/满员/read/失效）端到端可辨性：状态行 Result + task_log + 前端徽章/说明三维独立可辨，满员前后端同源。

## 结论

1. **身份防线矩阵第三十六轮闭合**：`20c5882..HEAD` 后端仅 57bf401（B110-01 审计修复），**零产品改动链延续第十六轮，COUNT=1**。sameClientFor 7 调用点行号零漂移；写点全家福 12 类抽查 5 类（tokenValid/inflight/done/full/rateLimited）全持锁 + 身份/存在性复核后写入，偶发写点专项四类（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll）零裸写复证；手动四路 accountExists + maybeRelogin 双侧全在位；六 err 归并分支身份复核成族齐备。**矩阵保持完全闭合，无新证据够格立条。**
2. **OBSERVE-117-01 知识位（第四轮）**：确认在位。重登写回侧取当前注册表 token 落库、不串旧身份，支撑链（ReloginIfNeeded→Login 更新实例 token→UpdateIDToken 按主键写新值）持续成立，活化条件未触发。
3. **OBSERVE-111-01/112-03 盯守（第十轮）**：维持。handler.go:377/:457 `_ = MarkDone/RemoveDone` 恒 nil 非吞错（函数体落库点全日志化，:100 的 `_ =` 仅测试内）；失效文案与自动链同文案、select/exit 动作维度可区分。**零漂移。**
4. **B110-01 审计链（第十一轮）**：手动失败三分支 AppendLog 六处在位，成功路径审计行（MarkDone/RemoveDone）未回归，自动链失败族审计行齐位，零吞错穷举扫描全空。**零漂移。**
5. **O105-01 抖动基线**：socketPreheat/readyProbe 三版本逐字符零漂移；定向 -race 两包首轮即绿（zhidao 3.217s / accounts 2.090s），零抖动残余、零超时。
6. **新契约角度（错误文案端到端可辨性对照）**：风控/窗口关闭/满员/read/失效五类从 zhidao 产生 → scheduler/api 归并 → task_log 落库 → 前端回显全链可辨性清点通过，满员前后端同源，无新缺陷。

**审视结论：无 CRITICAL/MAJOR/MINOR，新增 OBSERVE = 零。稳定期价值 = 确认无新证据够格立条，全部延续项维持零漂移。**
