# R103 后端只读审查报告

审查对象：xuanke-auto HEAD `a98d5e3`（R102 收官纯观察轮：B101-01 sanitizeError 落地 + 身份防线矩阵第十七轮闭合 + B102-01 逐点扫描同族缺口不存在，进度 103/256）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round103-backend-findings.md）。

审查方式：Read / Grep / Bash 只读命令（定向 go test -race / go vet / 逐点走读）。核心走读范围：身份防线矩阵第十八轮（spawnChain 七分支 + maybeRelogin 双侧 + ProbeForAccount 回写段 + MarkDone/RemoveDone/SubmitAll/Restore + PurgeAccount 全量清）、O102-01 抖动基线第十八轮夹具走读、B101-01/B102-01 持续盯守（六测试在位 + 无携 token URL 遗漏透传）、既往观察项延续（O102-02/O92-02/M87-01/O90-01 + 契约 20 强扫）。新契约角度本轮聚焦：**调度器 tick 主循环完整状态机复查（探测→提交→状态写回→时钟同步→退避的时序与锁序）**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## MINOR（知识位固化，非缺陷）

### B103-01：实时人数复核（classFullRealtime）确证满员分支在真实平台下实质不可达——平台未下发 maxCount，CountEntry.MaxCount 恒 0（走读评估）

**位置**：`scheduler.go:1589 :1621-1641`（spawnChain 实时复核三路）+ `zhidao/client.go:820-825/861-872`（CountEntry/IsClassFull）。

**契约链实证**（与 CLAUDE.md「平台真相」锚点一致）：
- 真实 `select.js` 轮询回调只读 `id/selectedCount/auditedCount` 三键，全文件 0 处消费 `maxCount`，两 HAR 无该接口响应样本——`CountEntry.MaxCount` 经 JSON 静默忽略恒 0。
- `IsClassFull`（client.go:868）判据 `MaxCount > 0 && SelectedCount >= MaxCount` 恒 false。
- 因此 `classFullRealtime`（scheduler.go:1751）实际恒返回 `(false, nil)` 或 `(false, err)`——spawnChain 实时复核块的三路中：
  - `cErr == nil && full`（`scheduler.go:1621`）即「确证满员」分支在真实平台下**恒不进入**（无人数数据形态 `scheduler.go:871` 返回 err→走 `full=false` 兜底保留失败状态路径）；
  - 主体落在「未现满员」分支 `scheduler.go:1643-1667`：保留 failed 状态、下个 tick 重试。
- 真满员判定主路径是快照 `classFullInSnapshot`（`max_count` 实证，scheduler.go:1457/1721-1744），实时复核只是"若平台下发 maxCount"的防御性兜底。

**结论**：非缺陷，是钉死的平台实证契约（与「maxCount 判定不存在」CLAUDE.md 锚点完全一致）。列知识位保持观察：**若未来平台向 countList 下发 maxCount，实时复核三路立即活化为真实满员判定路径**——届时其身份防线（:1600/:1635 two sameClientFor + :1627 doneHas 胜利状态让位）已在位，可直接受益；唯一需确认的是标记满员的业务语义（与快照判满的 `markFullLocked` 共用，无需新代码）。

## OBSERVE（延续观察）

### O103-01（O102-01/O101-01/O100-01/O99-01/O98-01 抖动基线第十八轮）：定向跑全绿无 flake，夹具走读无新脆弱点（实测）

走读 socketPreheat/readyProbe 双保险使用正确且本轮无变化：client_test.go:25-40 包级 socketPreheat（预创建-关闭回环套接字排空冷启动 TIME_WAIT 窗口）+ sanitize_test/captcha_test 逐点复用；api 包 handler_test.go:139 readyProbe 宽栅栏（10 次 × 200ms + 显式 2s 超时，:185-210）把冷启动窗口前移到夹具构造期；client_test.go:47-52 有版本确认 readyProbe 轮询（200ms×10）补充 socketPreheat 单端口盲区。**未见新时序脆弱点**。本轮定向跑（zhidao 全包 + scheduler 身份防线族+窗口守卫 + api 关键路径 + session + store/db + accounts + admin config 族）全绿零 flake。基线口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（八轮 30 跑 4 单包单 FAIL ~13%）。主控将跑全量 race 吸收结论。

### O103-02（O102-02/O101-02 删除保护撞名延续复核）：handler.go:992 单判据与 B43-04 双条件不对称——维持观察，无新依据提级（走读）

handleAdminDeleteAccount（handler.go:992）`if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只在账号名等于 `AdminNameValue()` 时拦截。撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下该学生被删除保护永久覆盖。历轮归"低优先级不修"的三条理由本轮逐一复核仍成立（删除保护是防空删管理员自己的硬护栏语义 / requireAdminSession 前置 / B43-04 双条件签发让撞名学生正常登录）。**维持观察**。

### O103-03（O92-02 logintest 引擎判定源分叉延续复核）：维持关闭（走读）

cmd/logintest 走 `CaptchaEngineDefault()` 环境变量、主程序走 settings 持久化值覆盖——三态回退链完整，诊断工具语义固有分叉。R92 归档关闭、历轮维持，无新依据。**维持关闭**。

### O103-04（M87-01 窗口延续复核）：维持 MINOR + 注释兜底（走读）

main.go:192-200 5s Shutdown 超时与在飞 spawnChain 尽力优雅语义仍由注释完整覆盖；quit_shared.go 双 nil 防御 + setExitActions 两半段 + tray_quit_test.go 三钉仍在位（grep 契约锚点 M86-01 三处实证）。**历轮结论仍准确，无新依据**。

### O103-05（O90-01 CRLF 延续复核）：维持（实测）

`go vet` 对 scheduler/zhidao/api/session/accounts/store 六包全干净（VET_EXIT=0）。CRLF 纯工作区转换噪音，仓库内容 LF 合规。**维持**。

### O103-06（契约 17 零吞错延续扫查）：通过（走读）

spawnChain 各分支 AppendLog/SaveSuccess、MarkDone / RemoveDone / markFullLocked、SetTargetsForAccount 落库、maybeRelogin UpdateIDToken、handleAdminDeleteAccount、main 恢复各表、handleAdminConfig 落库失败（writeJSONStatus 500 不吞）——全部 `if err != nil { log.Printf }` 零吞错。本轮未扰动，维持历轮结论。

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第十八轮闭合**：`sameClientFor`（scheduler.go:204-210 nil→false + reflect 指针）调用点**主控 grep 实证 7 处**（:850 ProbeForAccount 回写段 / :1489 失效 / :1521 成功 / :1551 风控 / :1571 窗口关闭 / :1600 实时复核入口统一复位 / :1635 确证满员）。本轮补复核：第七分支实时复核 ErrUnauthorized（:1608 maybeRelogin）**落入 :1600 统一 sameClientFor 之后才触发**、非裸调；:1621 确证满员分支在 :1635 身份复核后才 markFullLocked；:1645 末端 doneHas 胜利状态让位与 :1627 满员分支 doneHas 守卫对称。maybeRelogin 决策侧存在性复核 :1208 + 写回侧 :1254（先清 relogging 再复核、成功分支内存写+UpdateIDToken 落库同门）。ProbeForAccount 回写段 :850 持锁先 sameClientFor + 识别槽双条件 `len(data.BeginTimes)>0`（:855-858，空快照绝不删槽）。MarkDone :1927 / RemoveDone :1995 ClientFor 存在性、SubmitAll :1355 过滤已删账号、spawnChain 链顶 :1388 + 取 client 两处 :1405/:1409、TryAcquireSubmit :1896-1914 inflight 去重。RestoreDone/RestoreTargets/RestoreRefused 无网络不需身份；PurgeAccount :495-519 全量清含识别槽+状态行。api 层手动报名/退选 ErrUnauthorized 走 maybeRelogin（:349）且入口有凭据表 accountExists 校验（:293-301）。**无新裸露写点**。round41/43 测试族 refresh 走读（scheduler_test.go:3031-3556 七测试：DropsSuccess/Relogin/RateLimitBackoff/WindowClosedFull/RealtimeRecheckFull/RealtimeUnauthorized/SuccessDropsInflight）夹具同款、waitChainExit :3109-3122 chains 活跃标记等待契约（绝不用 inflight 等待）、reloginFail 残留断言 :3186-3191/:3498-3503、500ms 轮询窗口放行 goroutine 启动 :3487-3496——第七条独立红绿（实测绿）。 | 通过（走读 + 定向 race 实测） |
| **B101-01 修复持续盯守**：sanitizeError（client.go:581-593）`Op + " " + ue.Err.Error()` + sanitizerErr Unwrap 下沉；doRequest 接入点 :450 单一收口。**六测试全绿实测**（TestSanitizeErrorStripsTokenFromDialError / PreservesJudgment / NilSafe / OriginalErrorPreserved / NeverEmptyError + TestDoRequestSanitizesDialError 端到端）。判型函数（isConnErrRetryable/IsReadErr）errors.As/Is 穿透未破坏。**无新回归**。 | 通过（实测 0.872s 全绿） |
| **B102-01 遗漏透传通道逐点扫描（第九轮）**：本轮再全量 grep 仓库 http 直调点：Prewarm（GET /login :102）/ SyncServerTime（GET /login :119）/ fetchLoginPage（:300）/ fetchCaptchaImage（:334）/ submitLogin（POST doLogin form body :370）五处裸 `c.http.Do`/`sess.Do`——URL 均为静态路径**无 idToken**；doRequest 全家五接口是唯一携 token URL 通道且已由 :450 sanitizeError 闭环；captcha.go vision 请求走独立 client（APIKey 头通道、URL 静态）不经 sanitizeError 但无需脱敏。**同族缺口不存在**。 | 通过（走读 + 逐点枚举） |
| **新契约角度：tick 主循环状态机复查**：tick（:973-1036）时序 = 对齐时钟 → maybePrewarm（连接池热态）→ maybeSyncClock（时钟对齐）→ 探测闸门（probeIntervalForOpen 单快照 + 过点后首 tick 立即探测 :989）→ 提交守卫（零值挂起 :1011 / 未到点 :1014 / WindowClosed 挂起 :1024 / 节流 250ms-1s :1032）。锁序一致（reloginMu→s.mu 全路径：maybeRelogin :1199 / MarkTokenValid :1306 / TokenValidFor :730 对齐）。supply：probe() per-account 并发由 probeSem cap 4 封顶 :1069-1076、单飞 probing :1049-1054；spawnChain 网络往返全锁外（:1476 释放 s.mu 前提交、:1588 实时复核锁外）——黄金期 DB 写仅在持锁微秒段。窗口判据单源 windowClosedLocked 三判据 :918-939（主判据 + 时钟 ≥3 + 幽灵 EmptyProbeRuns≥3）。reloginResults 通道回传后置 lastProbe 零值触发补探测 :675-681（非阻塞 default 弃信号 :1279-1281 持锁安全）。时钟同步 syncing 单飞 + 30s 失败退避 + 成功才推进闸门、失败清 offset 保 streak :322-407。**状态机自洽、无新漏洞**。 | 通过（走读） |
| **契约 20 强扫**：全仓生产代码 grep `\b(B|M|O|R|F)[0-9]{2}-[0-9]{2}\b` 命中 5 处（scheduler.go:843 M88-01；main.go:189 / quit_shared.go:7 / tray_windows.go:53/:65 M86-01），测试注释 B42-02/B43-02/M88-01 为测试场景锚点；"第 3 轮/本轮"字面仅测试注释与代码注释（scheduler_test.go:1583/:2442、scheduler.go:1124/:1148）为场景描述非"第 N 轮"决策历史。历轮裁定"协议锚点（契约号）符合规范、轮次决策历史（'第 N 轮'）禁止"，本轮强扫一致。**无违规需上报**。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test ./internal/zhidao/ -run 'TestSanitizeErrorStrips\|TestSanitizeErrorPreserves\|TestSanitizeErrorNil\|TestSanitizeErrorOriginal\|TestSanitizeErrorNeverEmpty\|TestDoRequestSanitizes\|TestIsReadErr\|TestRecognizeCaptcha' -count=1` | 全绿（脱敏六测试 + 判型 + 识别） |
| `go test ./internal/scheduler/ -run 'TestDeletedAccountRebuiltSameName\|TestProbeForAccount\|TestProbeChainSameClient\|TestWindowClosed\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestSubmit\|TestSetTargets\|TestPurgeAccount\|TestOpenTimeRetained\|TestTargetPublishMeta\|TestRestoreDoneSkipsResubmit' -count=1 -race` | 全绿（身份防线族 + 窗口守卫 + 提交 + 目标/purge，多批跑） |
| `go test ./internal/session/ -count=1 -race` | 全绿 |
| `go test ./internal/store/ ./internal/db/ -count=1 -race` | 全绿（store 29s 含激活并发） |
| `go test ./internal/api/ -count=1 -race -run 'TestAdminDelete\|TestLogin\|TestSetTargets\|TestState\|TestAccountOverride\|TestAdminConfig\|TestCSRF\|TestRenamedAdmin\|TestAdminStatsWindowOpenedUsesScheduler'` | 全绿（管理后台 + 会话 + 撞名 + 删除保护 + config 热重载族） |
| `go vet ./internal/scheduler/ ./internal/zhidao/ ./internal/api/ ./internal/session/ ./internal/accounts/ ./internal/store/` | 通过（VET_EXIT=0） |
| `git status --short --branch` | 干净（工作树后端零改动，唯一未跟踪为前端 findings 文件） |

## 结论

1. **身份防线矩阵第十八轮闭合**：7 调用点主控 grep 实证 + 本轮补复核第七分支 maybeRelogin 落入 :1600 统一复核之后、api 手动路径凭据表 accountExists 前置、PurgeAccount 全量清——无新裸露写点。
2. **O102-01 抖动基线第十八轮**：定向跑多批全绿零 flake，socketPreheat/readyProbe 夹具走读无新脆弱点，口径维持（~13% 低频残余由包序+冷启动窗口主导）。
3. **B101-01/B102-01 持续盯守**：六测试实测全绿无回归；第九轮全量 grep 确认 doRequest 仍是唯一携 token URL 通道且已闭环。
4. **既往观察项延续**：O102-02 删除保护撞名维持观察；O92-02 / M87-01 / O90-01 / 契约 17 / 契约 20 均维持历轮结论（零新依据）。
5. **新契约角度**：tick 主循环状态机复查通过——探测/提交/时钟同步/退避时序与锁序自洽，无新漏洞。
6. **新发现仅 B103-01 一条「知识位」级**：实时复核确证满员分支在真实平台下不可达（maxCount 恒 0），属钉死的实证契约而非缺陷，未来平台下发 maxCount 时该分支无需新代码即可活化（身份防线已在位）。

工作树后端文件洁净。