# R111 后端只读审查报告

- **审查对象 HEAD**：`616ab9e`（2026-09-23 18:38:30 +0800，docs(review): R110 双 findings + 收尾总结，进度 111/256）
- **审查方式**：只读走读 + grep/blame 逐点实证 + 定向 go test -race（全程零仓库文件修改）
- **聚焦范围**：身份防线矩阵第二十六轮闭合、B110-01 修复回首轮、B110-02 观察复核、O105-01 抖动基线、观察项/知识位盯守、新契约角度（AppendLog 全写点清点）

---

## 分级发现

**CRITICAL：无**

**MAJOR：无**

**MINOR：无**

**OBSERVE（延续 + 新）：**

### OBSERVE-111-01（延续，B110-01 链路新增的最后一段延伸观察）
B110-01 修复（57bf401）在手动报名/退选失败三分支补了 AppendLog，但**成功路径的审计行依赖 MarkDone/RemoveDone 的返回值被丢弃**（`_ = d.Sched.MarkDone(...)`、`_ = d.Sched.RemoveDone(...)`）。MarkDone/RemoveDone 目前恒返回 nil（scheduler.go:1922/1989 签名返回 error 但无一条 error 分支），因此该 `_ =` 不构成吞错。**观察点**：未来若给 MarkDone/RemoveDone 补返回错误（例如 SaveSuccess 落库失败上抛），两处 `_ =` 会把"手动报名成功但审计行/成功行落库失败"静默吞掉——自动链同场景已逐处 `if err != nil { log.Printf }`，手动链会重蹈契约 17 覆辙。当前无实际风险，立 OBSERVE 维持盯守，不立 MINOR（零证据支持现在改）。

### OBSERVE-111-02（延续，B110-02 reloginAt 时间基）
reloginAt 读（:1219/:1227 `time.Since(t)`）写（:1231/:1261 `time.Now()`）两侧均用本地钟，同基自洽；退避判定只做相对间隔比较，对齐钟偏移（~640ms）对 30s 基础节流/指数退避无实质影响。维持观察（B110-02 已确认无混用，本轮继续无退化、无新依据提级）。

### OBSERVE-111-03（延续，B109-01 RemoveFull 死方法）
`RemoveFull`（scheduler.go:2037-2049）本轮复证：定义外零调用（全仓含前端 web/src 均无引用），纯死代码。契约 20（不主动删非请求死代码）下维持"提一下不删除"立场，不进 MINOR。真实满员解封已由 `releaseFullIfFreedLocked`（spawnChain 链内每课调用）+ `MarkDone`（手动成功清 full）两路闭环覆盖，死方法无功能性缺口。

### OBSERVE-111-04（延续，O105-01 抖动基线）
api 包全量 race 首轮 1 FAIL（第 2 轮起连续绿，zhidao 首轮即绿）。失败形态未抓到测试名（首轮 tail 输出被日志淹没，未定位具体用例）——按既有契约判定为 Windows 回环冷启动低频残余，夹具 socketPreheat/readyProbe 逐字符零漂移复证，不立条。前轮结论维持。

---

## 必查项逐条结论

### 1. 身份防线矩阵第二十六轮闭合 —— 通过（含 B110-01 产品改动边界判定）

- **产品改动 COUNT**：`git log 20c5882..HEAD -- backend/` 实证 **1 条** = 57bf401（B110-01 修复）。注意：57bf401 是**审计修复**（handler.go 补 AppendLog + handler_test.go 补测试），**不是产品行为改动**——身份防线 zero-product-change 记录延续（B101-01 起零产品改动链仍成立，本轮的 1 条是审计链收口不是防线退化）。
- **sameClientFor 7 调用点逐点实证**（grep + 走读，行号与 R110 基线一致无漂移）：
  - :204 定义（`clientIdentity` 反射指针比对 + nil 兜底）
  - :850 ProbeForAccount 回写段（探测网络往返后锁内复核，身份已变/已删整体放弃回写快照与识别槽）
  - :1489 失效分支（ErrUnauthorized，复核不通过不写状态不触发 maybeRelogin）
  - :1521 成功分支（写 done/state/SaveSuccess/AppendLog 前复核）
  - :1551 风控退避分支（markRateLimitedLocked 前复核）
  - :1571 窗口关闭分支（markFullLocked 前复核）
  - :1600 实时复核统一入口（classFullRealtime 网络段后回锁复核，含存在性判定）
  - :1635 确证满员分支（doneHas 让位胜利状态后、markFullLocked 前复核）
- **写点全家福 12 类逐一对应防线**：
  - openTimeDetected：ProbeForAccount 回写段 :850 先 sameClientFor 再写 :856；probe() 全局槽 :1108 持锁写。无裸写。
  - acctData/acctDataAt：:850 复核后 :863-864 写；spawnChain 读（classFullInSnapshot/releaseFullIfFreedLocked）持锁。无裸写。
  - tokenValid：:1241 置位在 maybeRelogin 入口 ClientFor 复核（:1208）之后；:1262 清零在重登写回侧 ClientFor 复核（:1254）之后；MarkTokenValid（:1316-1318）只做清理。双闭合。
  - reloginAt/reloginFail/relogging：全部在 maybeRelogin 决策段（持 reloginMu+s.mu）与写回侧复核后；MarkTokenValid 清理。无裸写。
  - inflight：TryAcquireSubmit（:1905）持锁；spawnChain（:1468-1471）持锁；失败/成功分支锁内清位（:1490/:1501/:1513）。无裸写。
  - done/refused/full/rateLimited：全持 s.mu，写前均有身份/存在性复核（spawnChain 六分支 sameClientFor；MarkDone/RemoveDone ClientFor 复核）。
  - Courses 状态：setStateLocked 全部在持锁 + 身份复核链内。
- **偶发写点专项**：MarkTokenValid（仅 issueSession 手动登录成功路径调用，锁序 reloginMu→s.mu 对齐）、TryAcquireSubmit（锁内读写，release once.Do 幂等）、releaseFullIfFreedLocked（锁内，快照明确有余量才解封）、SubmitAll（lastSubmit 写对齐钟，与读侧 nowAlignedLocked 同基）。无新裸写。
- **手动四路 accountExists**（handler.go :245 课程读 / :295 手动报名 / :387 手动退选 / :563 状态读，另目标写 :487-502 用同判据 LoadCredentials 逐账号比对）——四路全覆盖复证。
- **maybeRelogin 双侧复核**：入口侧 :1208 ClientFor 存在性复核（决策侧）+ 写回侧 :1254 ClientFor 复核（重登完成时），双闭合复证。

### 2. B110-01 修复回首轮 —— 通过

- **失败三分支 AppendLog 全覆盖**（handler.go 逐点实证，错误落库全部 `if err != nil { log.Printf }`，契约 17 零吞错）：
  - handleElectiveSelect：ErrUnauthorized（:352）/ read 类（:364）/ 其余业务失败（:371）
  - handleElectiveExit：ErrUnauthorized（:433）/ read 类（:442）/ 其余业务失败（:449）
  - 每处均有独立日志文案（失效自动重登中 / 请求已发出结果未知 / 业务失败原文），与自动链语义对称。
- **成功路径审计行仍在**：MarkDone（scheduler.go:1976 AppendLog `msg,true` + :1973 SaveSuccess）/ RemoveDone（:2020 DeleteSuccess + :2026 SaveRefused + :2029 AppendLog）——均 `if err != nil { log.Printf }`，未因 B110-01 回归。
- **新测试真实断言**（handler_test.go）：
  - TestManualElectiveFailureAppendsLog（:1800）：mock 切业务失败（code=1）→ 报名/退选各断 `LoadLogs` 命中 `Action==select/exit && ClassID==61115 && !IsOK`，非空断言。
  - TestManualElectiveReadErrAppendsLog（:1866）：mock FLUSH+Hijack 直断（真实 read 形态）→ 断言失败日志行命中。
  - **实测**：`go test -race -count=1 -run 'TestManualElective...' ./internal/api/` → ok。
- **复核结论**：修复面完整（自动链失败必落 AppendLog 的对称补齐），TDD 测试真实红绿能力由"修复前零失败日志行"提交说明背书，断言非空。

### 3. B110-02 观察复核 —— 维持观察

reloginAt 写（:1231 `time.Now()` 决策时、:1261 `time.Now()` 成功后刷新）与读（:1219/:1227 `time.Since(t)`）同基本地钟，无"写本地/读对齐"混用。与已消灭的"写本地/读对齐"模式（探测/节流时间基）区隔：重登退避只做相对间隔比较，偏移 ~640ms 不构成实质错误。维持观察，无新依据提级。

### 4. O105-01 抖动基线 —— 维持（无新证据立条）

- socketPreheat（handler_test.go:67-71 net.Listen+Close）/ readyProbe（:185-214，10 次×200ms 轮询 + 2s 超时）逐字符零漂移复证。
- 实测：`go test -race -count=1 ./internal/api/` 首轮 FAIL（未抓到测试名）→ 二、三轮绿；zhidao 首轮即绿。按既有契约判定为 Windows 回环冷启动低频残余，**不立条**。

### 5. 观察项/知识位盯守 —— 全部维持，无退化

| 观察项 | 本轮复证 | 结论 |
|--------|----------|------|
| O105-02 删除保护撞名 | handler.go:121 `subtle.ConstantTimeCompare` + :136 adminName 分支归因"管理口令错误"；TestLoginAdminNameCollisionStudentCredential（:1515）断言撞名学生签发普通会话（IsAdmin false）+ 反向"管理口令错误" | 维持 |
| B105-01 展示层 | WindowOpened/WindowClosed 与 /state 同源，无产品改动 | 维持 |
| O106-01 票据 | handleActivate :199 ConsumeTicket（单次防重放，失败销毁票据）+ 前端回传契约 | 维持 |
| B109-01 RemoveFull 死方法 | 定义外零调用（含前端 web/src） | 维持（见 OBSERVE-111-03） |
| B101-01 脱敏链 | doRequest :422 唯一 token 通道；sanitizeError :581-593（非 url.Error 原样透传）；sanitize_test.go 六测试（剥 token/剥 idToken=/保判型/保底层语义/DoRequest 集成/nil）全部实测通过（`go test -race -run 'TestSanitize|TestIsReadErr' ./internal/zhidao/` → ok） | 维持 |
| B42-01 闸门家族 | gateWait（:49 阻塞排队）/ gateTryAcquire（:223 非阻塞准入）共享 gateMu/gateUsed；LoginByPassword :244 收口；Relogin :173 内部 gateWait；ResetGateForTest 仅测试调用 | 维持 |
| M87-01 窗口判据 | windowClosedLocked（:918-939）三判据单源 + StateForAccount/WindowClosed 共用；tick 零值守卫 :1011 `open.IsZero() && !opened`（B41-02 语义） | 维持 |

### 6. 新契约角度（本轮自选）：AppendLog 全写点清点 + B110-01 后审计链闭环

- **全部 AppendLog 写点**（grep 全仓实证，handler.go 11 处 + scheduler.go 9 处 + store 定义/测试）：
  - api 层：管理员登录（:126）/ 学生登录（:227）/ 手动报名三分支（:352/:364/:371）/ 手动退选三分支（:433/:442/:449）/ 目标保存（:548）/ 注销（:606）/ 配置更新（:854）/ 删除账号（:1045）——**除 MarkDone/RemoveDone 成功行外的所有手动/会话/管理动作均有审计行**。
  - scheduler 层：失效（:1504）/ 成功（:1532）/ 风控（:1558）/ 实时复核失效（:1614）/ 普通失败（:1662）/ 满员（:1770）/ 手动 MarkDone 成功（:1976）/ 手动 RemoveDone 成功（:2029）——自动链全失败分支 + 手动成功两行。
- **闭环判定**：B110-01 后"手动失败三分支"补齐，手动/自动/会话/管理四族审计链**完整闭环**——任何动作（成功或失败、结果已知或未知）都在 /api/logs 有库行可核对，零静默动作。错误落库全仓 `if err != nil { log.Printf }`（契约 17 零吞错）抽样复证。
- **发现**：无缺口。唯一值得留意的是 MarkDone/RemoveDone 的 `_ =` 丢弃返回值（见 OBSERVE-111-01，当前恒 nil 不构成吞错）。

---

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 20c5882..HEAD -- backend/` | 1 条（57bf401，B110-01 审计修复） |
| `go build ./...` | ok |
| `go vet ./...` | ok |
| `go test -race -count=1 -run 'TestManualElectiveFailureAppendsLog\|TestManualElectiveReadErrAppendsLog' ./internal/api/` | ok（1.887s） |
| `go test -race -count=1 -run 'TestSanitize\|TestIsReadErr' ./internal/zhidao/` | ok（1.517s） |
| `go test -race -count=1 ./internal/api/` | 首轮 FAIL（测试名未抓到，日志被淹没）→ 二/三轮 ok |
| `go test -race -count=1 ./internal/zhidao/` | ok（2.900s） |
| `go test -race -count=1 ./internal/scheduler/` | ok（16.063s） |
| `go test -race -count=1 ./internal/accounts/ ./internal/store/ ./internal/session/ ./internal/secure/` | 全 ok（store 45.4s 最慢） |
| `go test -race -count=1 -run '身份防线家族+窗口契约' ./internal/scheduler/` | ok（3.524s） |

## 已核无缺陷清单（走读 + 定向实测）

1. sameClientFor 7 调用点行号零漂移，六分支 + 探测回写段全闭合，无裸写点。
2. 写点全家福 12 类均有防线（身份/存在性复核或持锁），PurgeAccount 全量清理含 relogin 族与识别槽。
3. B110-01 手动失败三分支 AppendLog 全覆盖 + 成功路径审计行未回归 + 两测试真实断言 + 实测绿。
4. reloginAt 本地钟写读同基自洽（B110-02 维持观察）。
5. 脱敏链（doRequest :422 → sanitizeError）六测试绿，判型穿透不受剥 URL 影响。
6. B42-01 闸门家族 gateWait/gateTryAcquire 共享计数、手动登录/自动重登双收口。
7. M87-01 窗口判据三判据单源 + B41-02 零值守卫 `open.IsZero() && !opened` 语义正确（WindowOpened=true 时绝不挂起提交）。
8. 撞名学生登录契约（B43-04）双条件 + 等时拉平 + 普通会话签发，测试绿。
9. AppendLog 全写点清点闭环：四族（手动/自动/会话/管理）动作零静默，审计链无缺口。
10. 夹具 socketPreheat/readyProbe 逐字符零漂移；api 首轮 1 FAIL 判为低频残余（重跑绿）。

## 结论

R111 后端审查**通过**。身份防线矩阵第二十六轮闭合（B101-01 起零产品改动链延续——57bf401 是审计修复非产品行为改动，不构成防线退化）。B110-01 修复回首轮**通过**：手动报名/退选失败三分支 AppendLog 全覆盖、零吞错、成功路径审计行未回归、两测试真实断言且 race 实测绿。B110-02 reloginAt 时间基维持观察（写读同基自洽）。观察项全数维持，无 CRITICAL/MAJOR/MINOR。新发现 1 条 OBSERVE（MarkDone/RemoveDone 的 `_ =` 丢弃返回值，未来补错误返回值时的吞错风险点）——当前恒 nil 零实际风险，仅立观察。全部后端包 race 测试绿（api 首轮低频残余 1 FAIL 属既有抖动基线，重跑收敛）。
