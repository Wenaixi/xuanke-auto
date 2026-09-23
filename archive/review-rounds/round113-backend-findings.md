# R113 后端只读审查报告

## 头部信息

- **审查对象 HEAD**：`e68d869`（master，工作区干净，`git status` clean）
- **审查方式**：全程只读（grep/Read + 定向 `go test -race` + `go build`/`go vet`），零仓库文件修改，唯一写入本报告
- **聚焦范围**：身份防线矩阵第二十八轮闭合 · OBSERVE-111-01/112-03 盯守 · B110-01 审计链第三轮盯守 · B110-02 观察复核 · O105-01 抖动基线 · 新契约角度（zhidao 层 doRequest 全调用点契约逐点核对 + scheduler 锁序复核）
- **审查耗时**：约 25 分钟（含三组定向 race 测试）

## 分级发现

无 CRITICAL / MAJOR / MINOR（本轮零新缺陷入账）。

### OBSERVE（延续观察，零新立条）

**OBSERVE-105-01（延续抖动基线观察，本轮 zhidao 包全量 race 零 FAIL 实证）**
- 本轮 `go test -race -count=1 ./internal/api ./internal/zhidao` 双双 PASS（api 41.291s / zhidao 4.958s），R112 首轮出现的 zhidao 登录日志类 1 FAIL 本轮未复现。低频测试残余维持既有观察，后续若多轮复现（≥2 轮不同根因）再升格，本轮不立条。

**OBSERVE-111-01 / 112-03（延续盯守，维持）**
- `handler.go:377` `_ = d.Sched.MarkDone` / `handler.go:457` `_ = d.Sched.RemoveDone` 返回值丢弃：两函数体内部落库点全 `if err != nil { log.Printf }` 零吞错（MarkDone SaveSuccess :1973 / AppendLog :1976；RemoveDone DeleteSuccess :2020 / SaveRefused :2026 / AppendLog :2029），全路径仅 `return nil`，`_ =` 恒 nil 不构成吞错。手动失效分支（select :352 / exit :433）与自动链同文案「教务令牌失效，自动重登中」两条日志动作维度 select/exit 可区分，不构成重复留痕。维持观察，零漂移确认。

## 必查项逐条结论

### 1. 身份防线矩阵第二十八轮闭合 —— 通过

**`git log 20c5882..HEAD -- backend/` COUNT = 1**，唯一改动 `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志），零产品改动链延续（R101-01 起）。API handler 纯增量（AppendLog 补位），不涉身份防线本体。

- **sameClientFor 7 调用点逐点实证，行号零漂移**：
  - 定义 :204 + 注释 :199-202（clientIdentity :212-223）
  - :850 ProbeForAccount 回写段（openTimeDetected/acctData/acctDataAt 写前复核）
  - :1489 spawnChain 失效分支（ErrUnauthorized，重登落 :1499 在复核后）
  - :1521 spawnChain 成功分支
  - :1551 风控退避分支（isRateLimitError）
  - :1571 窗口关闭分支（isWindowClosedError）
  - :1600 实时复核统一入口（回锁后先复查，:1608 cErr ErrUnauthorized 在复核后触发 maybeRelogin）
  - :1635 实时复核确证满员分支（:1627 doneHas 先让位胜利状态，:1635 身份复核后 :1639 markFullLocked）
- **写点全家福 12 类逐一对应防线**：
  - `openTimeDetected[acct]`（写 :856，复核 :850 持锁内；PurgeAccount :506 清）
  - `openTimeDetected["*"]`（写 :1108 probe()，AnyClient 探测载体天然无账号身份，全校单值契约）
  - `acctData[acct]`/`acctDataAt[acct]`（写 :863/:864 复核后；删 :504/:505）
  - `tokenValid`（写 :1241 决策侧置位在 :1208 ClientFor 复核后 / :1262 成功清零在 :1254 复核后 / :1316 MarkTokenValid 手动登录清）
  - `reloginAt`/`reloginFail`/`relogging`（写 :1231/:1232/:1236 决策侧入口 :1208 复核；:1261 成功写回在 :1254 复核后；PurgeAccount 删 :508-510）
  - `inflight`（TryAcquireSubmit :1905 占位 handler 锁内自持；spawnChain :1471 占位、:1490/:1501/:1513 清位在 err 分支复核后/链顶身份捕获后；MarkDone :1951/RemoveDone :2003 清）
  - `done`（:1529 成功分支 :1521 复核后；MarkDone :1934 有 ClientFor 复核 :1927）
  - `refused`（:634 快照恢复；MarkDone :1939 解 refused；RemoveDone :2011 前 ClientFor 复核 :1995；SetTargetsForAccount :458 清）
  - `full`（markFullLocked :1760 从 :1555/:1575/:1639/:1458 受同族复核的分支调用；releaseFullIfFreedLocked :1781 数据型解封不涉身份写回）
  - `rateLimited`（:1714 仅 :1555 风控分支经 :1551 复核后写；MarkDone :1957 清）
  - Courses 状态行（写点全持锁；MarkDone :1961 在 :1927 复核后；PurgeAccount :512-518 清）
- **偶发写点专项**：
  - MarkTokenValid（:1306）——仅手动登录成功 issueSession 调用，锁序 reloginMu→s.mu 对齐，只清失效族不写业务身份态
  - TryAcquireSubmit（:1896）——inflight 位占位 + sync.Once 同步释放，不跨网络往返
  - releaseFullIfFreedLocked（:1781）——按快照数据型读删，空快照/课程不在快照不解封
  - SubmitAll（:1342）——收集链前 ClientFor 过滤（:1355），尾部统一走 spawnChain 身份防线；`lastSubmit = s.nowAlignedLocked()` 对齐钟写入（:1346）
- **手动四路 accountExists**：:245（课程读）/ :295（手动报名）/ :387（手动退选）/ :563（状态读）全部就位，`:1096-1105` 凭据表逐账号比对判据同源；handleSetTargets 域内联复核 :479-499 同源。
- **maybeRelogin 双侧复核**：决策侧 :1208 `ClientFor(acct)` 在一切 map 写入前（注释 :1202-1207 当面印刻）；写回侧 :1254 重登成功后 `ClientFor(acct)`（含 :1261/:1262 内存写 + :1269 UpdateIDToken 落库全覆盖）。测试族 TestMaybeReloginDeletedAccountSkipsMaps / TestDeletedAccountRebuiltSameNameChain 系列本轮定向 race 全绿。

### 2. OBSERVE-111-01 / 112-03 盯守 —— 维持（零漂移）

- handler.go:377/:457 `_ =` 恒 nil（两函数全路径仅 `return nil`）；函数体内部落库点全 `if err != nil { log.Printf }` 零吞错。
- 手动失效分支与自动链同文案「教务令牌失效，自动重登中」两条日志动作维度 select/exit 可区分，不构成重复留痕（上一轮裁决，本轮确认零漂移）。

### 3. B110-01 审计链第三轮盯守 —— 通过（零漂移）

- 手动失败三分支 AppendLog 六处全在位：报名 :352（失效）/ :364（read）/ :371（业务失败）、退选 :433（失效）/ :442（read）/ :449（业务失败），全部 `if err != nil { log.Printf }` 零吞错。
- 成功路径 MarkDone/RemoveDone 审计行未回归：MarkDone :1976（select, isOK=true）/ RemoveDone :2029（exit, isOK=true）。
- 自动链侧 AppendLog 落库点（:1504/:1532/:1558/:1614/:1662/:1770）同规零吞错。

### 4. B110-02 观察复核 —— 通过

- `reloginAt` 读写同基自洽：写（决策侧 :1231 `time.Now()`、成功写回 :1261 `time.Now()`）与读（:1219 退避窗口 `time.Since(t)`、:1227 30s 节流 `time.Since(t)`）全部基于同一本地钟；窗口开放判点由 `nowAlignedLocked()`（对齐钟）独立承担，两时间基职责分离、无混用。注释 :1226 删除语义与 :1231 新间隔写入自洽。

### 5. O105-01 抖动基线 —— 通过（本轮零 FAIL）

- 夹具 socketPreheat/readyProbe 逐字符零漂移：socketPreheat（zhidao/client_test.go:25-30）net.Listen+Close 预占回环；readyProbe（api/handler_test.go:185 + zhidao/client_test.go:82 同款 200ms×10 + 显式 2s 超时）；zhidao TestMain :39-42 包级预热兜底；loginMockServer 构造后 readyProbe 前移冷启动窗口。
- 定向 `go test -race -count=1 ./internal/api ./internal/zhidao` 双双 PASS（api 41.291s / zhidao 4.958s），本轮零 FAIL（对比 R112 zhidao 首轮 1 FAIL），低频残余未复现。

### 6. 新契约角度：zhidao 层 doRequest 全调用点契约 + scheduler 锁序 —— 通过（无新发现）

- **doRequest 全调用点收口**：FindElectives（:686/:705）、YearTerms（:625）、SelectClass（:769）、ExitClass（:794）、StudentCounts（:829）全部经 doRequest；无旁路裸 http.Client.Do。契约三函数覆盖：
  - `sanitizeError`（:581-593）在 :450 统一剥 url.Error URL 文本（含 idToken 参数通道），非 url.Error 原样透传不误伤业务文案，Unwrap 下沉保留判型；sanitize_test.go 断言 token/idToken 均剥除
  - `httpDo`（:478-492）只重试 dial/write（isConnErrRetryable :496-505，与 IsReadErr 互斥），read 错误不重试（防双报幂等防线）
  - `IsReadErr`（:523-552）覆盖 RST/FIN(io.EOF)/短读(ErrUnexpectedEOF)/超时四形态，scheduler :1656 与 api :361/:440 读侧区分文案全对齐
- **scheduler 锁序全路径复核**：reloginMu→s.mu 顺序恒定（maybeRelogin 决策段 :1199-1201、MarkTokenValid :1312-1314、TokenValidFor :731-733 全部 reloginMu 先于 s.mu）；无任何反向加锁路径。chainMu（:1392-1403 链活跃标记）独立于两把主锁不嵌套。sameClientFor 在 s.mu 持锁下调用。类FullRealtime 明确锁外网络（:1589 先 Unlock 再复核回锁，注释 :1583-1587 当面印刻）。零锁序死锁路径，无新观察。

## 验证表

| 命令 | 结果 |
|------|------|
| `git status --short --branch` + `git rev-parse HEAD` | 干净，HEAD=e68d869 |
| `git log --oneline 20c5882..HEAD -- backend/` | 1 条（57bf401），COUNT=1 确认 |
| `go build ./...` | PASS（exit 0） |
| `go vet ./...` | PASS（exit 0） |
| `go test -race -count=1 -run "TestDeletedAccountRebuiltSameNameChain\|TestSubmitSuspendedWhenOpenTimeCleared\|TestWindowOpenSubmitsWithoutProbeReset\|TestAdminStatsWindowOpenedUsesScheduler\|TestMigrateAddsPublishMetaColumns" ./internal/scheduler ./internal/api` | PASS（scheduler 4.780s / api 2.753s） |
| `go test -race -count=1 ./internal/api ./internal/zhidao` | PASS（api 41.291s / zhidao 4.958s），本轮零 FAIL |

## 已核无缺陷清单

- 手动四路 accountExists 判据同源（凭据表逐账号比对 :1096-1105），四路全部在 `allowAccountOverride` 门内
- MarkDone/RemoveDone 内部 ClientFor 复核（:1927/:1995），删号与在飞手动报名竞态防线完整
- PurgeAccount 12 类写点全清（:495-519 含 openTimeDetected 识别槽），memory-first 删账号链路与调度器状态彻底隔离
- 全仓库零 `_ = s.store` / `_ = d.Store` 落库丢弃点（grep 实证 27 处落库调用全 `if err != nil { log.Printf }`）
- 契约 20 轮次标签：全 backend 代码扫描仅 session/store.go:117 一处"review-round13.md"文档文件名引用（非轮次前缀标签），无残留
- 基础设施状态码家族（B39-02/B40-01）：401（:1123）/ 403（:632 管理 + requireJSONBody CSRF-403 :72）/ 500（:845/:950/:1229）/ 429 限流 + 登录/激活 CSRF 门（router.go:101-104）全部落真实 HTTP 状态码；router 19 端点鉴权核对无裸 writeJSON
- zhidao 层 FindElectives 空快照解析（:745-747 code:0 空 publishes 直接返回）与窗口关闭契约对齐，不误入学期列表兜底

## 结论

- **身份防线矩阵第二十八轮闭合**：`20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，非产品改动链，零改动前提核位通过）；sameClientFor 7 调用点行号零漂移；写点全家福 12 类逐一对应防线；偶发写点专项四类全部核过；手动四路 accountExists + maybeRelogin 双侧复核全在位。矩阵保持完全闭合。
- **OBSERVE-111-01 / 112-03 盯守确认**：`_ =` 恒 nil 不构成吞错，落库点全日志化零漂移；双文案动作维度可区分不构成重复留痕。维持观察。
- **B110-01 审计链第三轮通过（零漂移）**：手动失败三分支 AppendLog 六处全在位，成功路径审计行未回归。
- **B110-02 观察通过**：reloginAt 本地钟读写同基自洽，对齐钟独立承担窗口判点，两时间基职责分离。
- **O105-01 抖动基线通过**：api/zhidao 全量 race 本轮双双 PASS，zhidao 低频残余未复现（对比 R112 首轮 1 FAIL），维持既有观察。
- **本轮无 CRITICAL / MAJOR / MINOR，无新 OBSERVE**。
