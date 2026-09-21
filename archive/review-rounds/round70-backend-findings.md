# R70 后端只读审查报告

## 概述

**TestCaptchaConcurrency readyProbe 双保险逐行核证正确，但本轮十轮全量 -p 1 复跑（RUN1-RUN10）出现 2 例 FAIL（R1 zhidao TestRecognizeCaptcha / R7 api TestAdminStatsAccountsLogs），均为 Windows 回环冷启动 connectex 形态——"并发形态残余"宿主判断成立但**全量 -p 1 串行形态暴露新残余**，栅栏终态首次在串行形态出现流动样本（此前历轮串行全量零 FAIL）。** 双保险对 TestCaptchaConcurrency 本身归零，但残余宿主已从"该测试"重定位到"包内第一个起服务器且无探活夹具的测试 + api 全包并发才出现的 readyProbe 自身全败"。历轮决策手册契约抽查全部成立，无 CRITICAL/MAJOR 缺陷，共 6 个 MINOR + 3 个 OBSERVE。**

## R69 收尾修补复核（commit b619c89 逐行核证）

### TestCaptchaConcurrency readyProbe 双保险

- **核实结果：正确**
- b619c89 在 `TestCaptchaConcurrency` 的 `defer srv.Close()` 之后、`NewVisionRecognizer` 之前（captcha_test.go:69-74）插入 `readyProbe(t, srv.URL)`。调用位置核实无误。
- **探活与 10 个识别请求互不干扰**：readyProbe 发 GET `/ready`（captcha mock 对未知路径走默认分支返回 JSON），在 10 个并发识别请求之前完成。探活请求命中 mock 默认分支、`inFlight` 计数器加 1 后立即减 1，其 30ms 模拟耗时与并发识别请求互不重叠（探活先完成才起 10 并发 goroutine）——`maxInFlight≤2` 断言不受影响。信号量（`withConcurrency`）只保护识别路径，探活走客户端 http.Client 直连、不经信号量，不占 limiter 许可。**互不干扰成立**。
- **注释准确**：引用 R60/R64 历史宿主（并发首请求 connectex 时信号量计数被误判）+ socketPreheat 只预占单端口语义准确；"api/zhidao 同款双保险"的 cross-ref 准确。
- **独立复跑结果**：TestCaptchaConcurrency 隔离 5 轮 + -count=10 密集复跑**全部通过**（正常 0.19s）；R1 全量中该测试未出现在 FAIL 名单（该轮 FAIL 是 TestRecognizeCaptcha）。**双保险对目标测试归零成立。**

## 历轮决策手册抽样闭合复核（契约 1-41）

抽查 6 条关键契约，全部成立：

| 契约 | 实现位置 | 复核结果 |
|------|----------|----------|
| 契约2 窗口关闭三判据单源 | scheduler.go:914-935 windowClosedLocked | 成立。主判据 +10s 裕量（probe 写入）/ 时钟 ≥3 带"开放时间已过"单快照 open / EmptyProbeRuns≥3 视同关闭；StateForAccount（708）/WindowClosed（903-906）/admin stats（955）同源；TestWindowClosedState/TestGhostWindow* 固化。 |
| 契约4 删账号 memory-first 四步 | handler.go:1005-1019（Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount） | 成立。顺序与契约完全一致；DeleteAccount 失败留"半删态自愈"注释（1011-1015）；Sessions.RevokeAccount 立即吊销（1019）。 |
| 契约31+37 sameClientFor 全分支（B41-01 六分支 + B43-02 实时复核） | scheduler.go:1485 失效 /1517 成功 /1547 风控 /1567 窗口关闭 /1596 实时复核入口 /1631 确证满员 | 成立。六处同族防线成族；maybeRelogin 决策侧 ClientFor 复核（1204）+ 写回侧复核（1250）双闭合；测试族 TestDeletedAccountRebuiltSameNameChain{DropsSuccess,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull,RealtimeUnauthorized,SuccessDropsInflight} 定向复跑全部 PASS。 |
| 契约40 IsReadErr 四形态消费点 | client.go:519-548 IsReadErr（FIN/短读/超时/RST）+ isConnErrRetryable 互斥 | 成立。消费点全路径：scheduler 1652（自动链文案）/ api 356（手动报名）；测试 TestIsReadErrCoversAllForms 复跑 PASS。 |
| B42-01 doLogin 全入口收口闸门 | manager.go:223-235 gateTryAcquire 非阻塞准入（LoginByPassword:244 前置）+ Relogin:173 gateWait | 成立。共享 gateMu/gateUsed；ResetGateForTest 仅测试调用（authenticateDirect 夹具），正式代码零调用。 |
| 契约1 开放时间唯一事实源 | scheduler.go:432-443 openTimeForLocked + config.go 无 OpenTime 字段 | 成立。识别槽 [acct]→["*"]→零值回退（openTime 字段仅测试兼容）；空快照不删槽；识别过期只影响展示层；main.go:113 New 传零值。 |

## 跨轮修复闭合抽查（R67/R68/R69 关键修复）

- **zhidao/accounts readyProbe 宽栅栏**（33722d3/0342a3a）：三家 readyProbe（api handler_test.go:184 / zhidao client_test.go:88 / accounts manager_test.go:26）栅栏几何一致（200ms×10 + 2s）。R67 OBSERVE-67-01 的"窄栅栏 5 次全败"已不存在——本轮 R1 api TestAdminAuth readyProbe 自身全败仍存在（11 次重试仍 connectex，即 4s 总窗口仍不够），但几何已对齐、非窄栅栏回归。
- **MINOR-69-01 注释锚点 973**（efb2f4c）：tick 内 `s.probing` 复位注释已改为"open 已在本函数开头取过单次快照（973 行）"，与实现（scheduler.go:973 `open := s.openTimeForLocked("")`）逐字段核对一致。闭合。
- **scheduler 跨度修复**（tick 零值守卫让位于 WindowOpened B41-02）：scheduler.go:1007 `if open.IsZero() && !opened { return }`——零值&&未开窗才挂起，WindowOpened=true 放行；TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime/TestSubmitSuspendedWhenOpenTimeCleared 复跑双绿。闭合。

## 新发现

### MINOR-70-01（api 包 readyProbe 的 2s 超时在极端 CPU 争用下不足以覆盖 connectex 窗口）
- 文件：backend/internal/api/handler_test.go:184-204（readyProbe）
- 问题：本轮 R7 全量（api 包 62s 高负载）中 TestAdminStatsAccountsLogs 的 mock 服务器首请求 `Get "/login": connectex`（2s 超时触发）；R9 双包并行（api+zhidao -count=2）中 TestAdminAuth 的 readyProbe **自身 11 次重试全部 connectex**（每次 2s 超时，总窗口 ~4s+仍失败）。两样本均为 Windows 回环冷启动在包高负载/并发形态下的残余流动。
- 影响面：非 -p 1 多包并行或高负载全量轮偶发红；纯测试影响。
- 机制：readyProbe 单请求超时 2s、重试 10 次；在高 CPU 争用或并发包形态下，单次 dial 超时即判失败重来，总窗口 4s 仍可能覆盖不了 OS 级回环队列排空。R7 样本中后续同 mock 的其它请求全部成功（仅首个 GET /login 失败），说明是"mock 尚未就绪 + 客户端超时"双重因素。
- 建议：readyProbe 对"首请求"形态可把单次 `http.Client.Timeout` 从 2s 放宽到 3-4s 并保持 10 次重试（总窗口近似 → 更稳）；或保持现状按 OBSERVE 记档（CI 固定 -p 1 即隐含规避）。观察项解，**低优先修**。

### MINOR-70-02（TestLoginNetworkErrorAbortsImmediately 是 zhidao 包内唯一裸 httptest 无 readyProbe 的"冷启动第一宿主"）
- 文件：backend/internal/zhidao/client_test.go:157-166
- 问题：全包仅该测试与 captcha_test 的 TestRecognizeCaptcha 自带 httptest 而无 readyProbe。R1 全量 zhidao 包首轮（6.05s，store 前序 39s 高耗时后）TestRecognizeCaptcha connectex FAIL；TestLoginNetworkErrorAbortsImmediately 无探活——若它在包序最前同样可 connectex。TestCaptchaConcurrency 已有 readyProbe 双保险、独立性经 10 次复跑确证。
- 影响面：全量首轮偶发 red；TestCaptchaConcurrency 已新增双保险，但 TestRecognizeCaptcha/TestLoginNetworkErrorAbortsImmediately 仍是**包内首探活缺口**（R60/R64 记录的历史宿主正是"并发首请求 connectex"）。
- 机制：TestRecognizeCaptcha 与 TestCaptchaConcurrency 同文件同把 NewCaptchaSemaphore init、共享包内 readyProbe 快照；但 TestRecognizeCaptcha 自身未调 readyProbe——R1 失败正是它（首请求 connectex→httpDo 重试→仍失败→recognizeCaptcha 报识别字符数 0）。包序中 store 高耗时拉伸 TIME_WAIT 队列后首个无探活 mock 首请求即为残余宿主。
- 建议：给 TestRecognizeCaptcha 与 TestLoginNetworkErrorAbortsImmediately 补 readyProbe（同款 6 行），与 b619c89 双保险成族闭环。**低优先修**（可与 MINOR-70-01 一起顺手）。

### MINOR-70-03（bench/logintest 命令行工具无 go vet 告警但 bench 无自愈重试）
- 文件：backend/cmd/bench/main.go:27（`client := &http.Client{Timeout: 5 * time.Second}`）
- 问题：bench 用裸 http.Client 无 httpDo 自愈——loadtest 旨在测真实业务延迟，瞬时 connectex 即 `fmt.Println("ERR")` 终止整轮，无重试；与生产 httpDo 自愈契约不一致（工具可观测性面）。
- 影响面：仅运维工具；基准测试在瞬断下提前中止。
- 机制：开发工具未经生产 httpDo 收口；日志/计数天然缺失。
- 建议：工具场景可保持原样（观察项）；若未来把 bench 并入 CI 基准，再接 zhidao.SharedTransport + httpDo。**低优观察**。

### MINOR-70-04（cmd/logintest 对 token 长度未做前 8 位截断安全边界）
- 文件：backend/cmd/logintest/main.go:102（`token[:min(8, len(token))]`）
- 问题：`min(8, len(token))` 已对短 token 安全（不越界），但输出的是**前 8 位明文**——与全仓"token 只显前 8 位"契约一致（maskedToken 同款），非缺陷；但该工具输出到 stdout 可能被 CI 日志持久化。核对 maskedToken 语义（scheduler.go:1330-1335）后确认一致。
- 影响面：无（契约一致）。
- 机制：与 maskedToken 同型，属于工具侧日志脱敏标准应用。
- 建议：记事确认，**不修**。

### MINOR-70-05（scheduler OpenTime 顶层字段仅测试兼容、注释已载明但存在轻微文档歧义）
- 文件：backend/internal/scheduler/scheduler.go:158 + 440-442
- 问题：`openTime time.Time` 顶层字段在生产路径恒零值（main 传 time.Time{}），仅测试兼容。openTimeForLocked 回退到该字段的注释自洽（"识别槽是唯一事实源，生产路径传零值后此回退恒零值（配置链路已整体移除），保留字段仅为测试兼容"）。无逻辑缺陷。
- 影响面：维护认知成本（后续维护者可能误以为 openTime 可配置）。
- 机制：已注释声明；测试直构 New(openTime) 场景仍依赖该字段回退（如 TestProbeIntervalFor 直传 open）。
- 建议：观察项，**不修**（删除会牵动全部测试构造点，低价值）。

### MINOR-70-06（admin 删除接口对 DELETE 请求体无 Content-Type 检查）
- 文件：backend/internal/api/router.go:153-159（DELETE /api/admin/accounts 未挂 requireJSONBody）+ handler.go:980-1023
- 问题：DELETE /api/admin/accounts 放行空 body 合法（REST 语义），但攻击者可用 `Content-Type: application/x-www-form-urlencoded` 的跨站表单 DELETE 触发账号删除——CSRF 缓解（requireJSONBody）对该端点不生效。
- 影响面：安全面。同族：codes DELETE 同样无 JSON 门（B7-M8/M9 刻意放行空 body）。**但**登录/激活之外的删除接口本受 requireAdminSession 保护（攻击者无法跨站获得管理员 token），且浏览器跨站表单不发 Authorization 头——CSRF 必需"已登录受害会话"，而管理会话 token 在 localStorage 不随 cookie 自动携带、跨站表单不可能带 Bearer 头。故实际 CSRF 不可能触达。与前 R69 OBSERVE-69-05（logout 无 JSON 门）同族论证。
- 机制：管理接口鉴权完全依赖会话 token（非 cookie），跨站表单无法伪造 Bearer → CSRF 威胁面天然不存在；JSON 门仅防御"同源 XSS 场景下的表单伪造"，该场景下攻击者已有 token、无需 CSRF。
- 建议：与 OBSERVE-69-05/06 同族记档，**不修**。

### OBSERVE-70-01（R7 全量测试中 api 包 TestAdminStatsAccountsLogs 失败——串行全量首现残余流动）
- 文件：backend/internal/api/handler_test.go:912（TestAdminStatsAccountsLogs）
- 现象：`go test -race -count=1 -p 1 ./...` 第七轮 api 包 TestAdminStatsAccountsLogs FAIL——首个 mock 请求 `Get "/login"` connectex（29s 耗时、2s 客户端超时）。该测试隔离复跑 + admin stats 族复跑全部 PASS。
- 机制：api 包 62s 高负载（含大量登录链路 mock），TestAdminStatsAccountsLogs 的 newTestDeps 前序已有多个测试建 mock——Windows 回环 TIME_WAIT 队列在高负载下排空变慢，该测试自身独立 mock 首请求恰落窗口内。readyProbe 对自身 mock 已做，但 FAIL 是**客户端 Login 路径**（zhidao.Client.Login 的 fetchLoginPage），其自愈只重试一次（client.go:318-322），重试下的 2s 超时仍失败。
- 建议：与本轮统计归并到 OBSERVE-69-04 的宿主家族（Windows 回环冷启动 + 高负载），记档后 CI 固定 -p 1；若要根治可在 api 高负载形态下给 Login 首请求自愈加一次窗口（或提高 httpDo 重试阈值形同测试专用）。**观察项**。

### OBSERVE-70-02（readyProbe 自身 11 次重试全败样本——api 包并发形态残余在 R9 复现）
- 文件：backend/internal/api/handler_test.go:47（TestAdminAuth 的 readyProbe）
- 现象：`go test -race -count=2 -p 2 ./internal/api/ ./internal/zhidao/` 中 TestAdminAuth readyProbe `Get "/ready"` connectex，11 次重试全败（总窗口 ~4s+），该测试在串行 -p 1 下从未失败。
- 机制：双包并行 + -count=2 时 api 与 zhidao 共享 CPU 争用，mock server accept 就绪前 OS 回环 connectex 窗口被拉长——readyProbe 的 2s 单次超时×10 次仍覆盖不了并发形态的窗口（与 OBSERVE-69-04 api 3 例失败同根）。
- 建议：维持"残余面 = Windows 回环冷启动 + 高负载/并发形态"的既有判定，CI 固定 -p 1 即收口；栅栏几何已最大化（200ms×10+2s），应用层无法根治 OS 级队列窗口。**观察项不修**。

### OBSERVE-70-03（store 包批插耗时波动大——R10 出现 180s 极值）
- 文件：backend/internal/store/store_test.go（批插测试）
- 现象：十轮中 store 包耗时 39.4/24.2/43.9/39.6/29.6/100.7/95.2/33.4/48.3/180.6s——R10 出现 180s 极值（此前最多 100s）。批插 30050 行在 -race 下受宿主 IO/CPU 波动影响大，测试通过但耗时特征不稳定。
- 影响面：CI 时长不稳定（360ms→60s 波动）；非功能缺陷。
- 机制：-race + 批事务 + 30050 行在 Windows 回环/防病毒扫描下 IO 波动放大。
- 建议：可考虑 `t.Skip` 在 -race 下跳过批插耗时断言（或降批插量），但会削弱语义覆盖——观察项，**不修**（记档）。

## flake 统计

### 十轮全量（cd backend && go test -race -count=1 -p 1 -timeout 900s ./...）

| 轮次 | 结果 | FAIL 样本 |
|------|------|-----------|
| RUN1 | **FAIL**（zhidao） | TestRecognizeCaptcha connectex（2.02s，store 39s 前序后） |
| RUN2 | 全绿 | |
| RUN3 | 全绿 | |
| RUN4 | 全绿 | |
| RUN5 | 全绿 | |
| RUN6 | 全绿 | |
| RUN7 | **FAIL**（api） | TestAdminStatsAccountsLogs connectex（29s，2s 超时） |
| RUN8 | 全绿 | |
| RUN9 | 全绿 | |
| RUN10 | 全绿 | |

**统计：10 轮中 8 绿 2 红（20% 红率）——全量 -p 1 串行形态首次出现流动样本（R69 之前该形态 9 轮零 FAIL）。**

### CaptchaConcurrency 双保险独立复跑
- captcha 三测试隔离 5 轮：全绿（1.79-2.79s）
- TestCaptchaConcurrency -count=10：全绿（3.68s 总）
- 双包并行 -count=2（api+zhidao）：zhidao 全绿、TestCaptchaConcurrency 未失败
- **判定：双保险对 TestCaptchaConcurrency 归零成立。**

### 并行形态复跑（OBSERVE-69-04 验证）
- api+scheduler 双包 -count=2（R69 复现 3 例）：本轮**全绿**（api 46.2s）
- api+zhidao 双包 -count=2：**TestAdminAuth readyProbe 11 次重试全败 FAIL**（R9）
- store+zhidao 双包 -count=2：全绿（store 95s 高耗时 + zhidao 3.99s）

**残余面判断**：TestCaptchaConcurrency 双保险确证归零（该测试本身不再出失败）；但残余宿主已重定位——①**串行全量 -p 1 形态出现 2 例**（R1 zhidao TestRecognizeCaptcha / R7 api TestAdminStatsAccountsLogs），宿主=包内第一/高危首个无探活 mock 首请求 + store 前序高耗时拉伸窗口；②**并发并行形态仍在**（api readyProbe 自身全败样本）。双保险解决的是"TestCaptchaConcurrency 并发首请求"这一具体宿主，未覆盖全部 mock 首请求形态。

## 历轮观察延续

- OBSERVE-69-04（api 包隔离并行冷启动残余）：**本轮在 api+zhidao 双包并行复现**（TestAdminAuth readyProbe 11 次全败，同 OBSERVE-69-04 的三测试族同根）。宿主家族确认 = Windows 回环冷启动 + 高负载/并发形态。
- OBSERVE-69-05（logout 无 CSRF JSON 门不适用）：延续成立（admin DELETE 同理侧载于 MINOR-70-06）。
- OBSERVE-65-03（accounts 无 socketPreheat 双保险）：accounts 两轮全量 + 隔离复跑全绿，未再出现样本。维持记录。
- OBSERVE-69-03（cfg.Decrypt 停用字段）：延续成立（本轮回读 handler 102-103 仍传参，注释自洽）。
- **新观察**：R7/R1 的串行形态 FAIL 是本轮新的残余流动方向——全量 -p 1 不再是"绝对绿"保证。CI 若固定 -p 1 仍可能偶发红（2/10 概率级）。

## 结论

后端无 CRITICAL/MAJOR 缺陷。发现 6 个 MINOR（均低优先）+ 3 个 OBSERVE。TestCaptchaConcurrency 双保险（b619c89）逐行核证正确且独立复跑归零；历轮决策手册契约抽查全部成立；sameClientFor 六分支 + maybeRelogin 双闭合复核无脱节。R1/R7 两例串行全量 FAIL 均为 Windows 回环冷启动 connectex 形态，说明栅栏终态残余面仍在流动（宿主=包内无探活 mock 首请求 + 高负载窗口），建议对 TestRecognizeCaptcha / TestLoginNetworkErrorAbortsImmediately 补 readyProbe 双保险成族（MINOR-70-02），并维持 CI -p 1 收口 + 记档并行残余。