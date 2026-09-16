# 第 14 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道），发现全部经主通道逐一读码推演 + 实证测试定案；修复按"测试契约修正/补断言/最小生产改动"独立 commit。
> 本轮结论：**后端 2 项 MAJOR 均为"测试假象"经实证定案（B14-M1 恒红失效契约、B14-M2 恒绿无断言契约）、1 项 MINOR 日志硬编码（B14-N1）、1 项 INFO 无效测试段（B14-I1）；前端 1 项 MINOR 空态兜底（F14-03）**；无 CRITICAL，无新增安全漏洞；M1 遗留（HTTP 侧探测未接入单飞）复核维持观察项。

---

## 一、后端发现与修复状态（第 14 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B14-M1** | MAJOR | **TestWindowOpenSubmitsWithoutProbeReset 首段断言是"失效契约"恒红**：openTime 为未来 1 小时 + 全程不 resetProbe。tick 守卫（scheduler.go:702）`!opened && !now.After(open)` 对未确认开启且未到点恒 return → 提交循环根本无法抵达，首段 `pending→failed` 断言死等 3 秒超时必红。**实证**：`go test -run TestWindowOpenSubmitsWithoutProbeReset` 非 race 与 race 均必红（scheduler_test.go:835 pending→failed，3.03s）；其余 5 个 tick 守卫测试（TestWindowOpenRetriesWithoutWaitingProbe/TestStateMachine/TestSubmitUnauthorizedTriggersRelogin/TestRateLimitBackoff/TestSubmitSuspendedWhenOpenTimeCleared）全部恒绿——证实是本测试自身与守卫脱节，而非守卫回归。**修复**：openTime 改为过去 1 秒（守卫放行提交路径，探测仍被 lastProbe 30s 节流挡住），真实覆盖"提交不依赖探测节流"语义并转绿（0.29s PASS）。附 B14-M1 注释落盘 |
| **B14-M2** | MAJOR | **TestAdminStatsWindowOpenedUsesScheduler 后半段无断言，stats 契约从未验证**：869 行 `_ = st` 把 stats 结果直接丢弃，注释宣称"inDateRange=false 无法置真"与 mock server（handler_test.go:69 发布 A inDateRange=true）自相矛盾。**实证**：补 `==true` 断言首次必红（handler_test.go:869 window_opened=false）——探明 ProbeNow 走全局 `probe()` 同款主循环（非 ProbeForAccount 账号级路径），随后由 ProbeForAccount 写全局 lastProbe/快照，但 WindowOpened 只有 `probe()` 置真、本次提交（scheduler_test.go:823）早已把它刷成 false——接口语义确为"stats 与调度器探测状态同源而非本地时钟直判"，且**该断言随执行顺序漂移**（先于 tick 执行则 false）。**修复**：断言改为"stats.window_opened === d.sched.WindowOpened()"同源判定——与调度器实际探测状态对齐，不依赖 ProbeNow 立即置真的时序假设，验证"admin stats 与调度器探测同源"的真实契约（PASS） |
| **B14-N1** | MINOR | **管理端日志账号硬编码字面量 "admin"**（handler.go:746/891）：`AppendLog("admin",...)` 写死——管理员用 XUANKE_ADMIN_NAME 改名后日志仍记 admin，与前端管理员昵称展示错位（两套真相）。**修复**：改用 `d.AdminNameValue()`（默认 admin，改名即生效），与 CreateAdmin 绑定配置名一致。build+vet 通过 |
| **B14-I1** | INFO | **TestReloginBackoffCappedAndReset 后半段测 Go map 语义**（scheduler_test.go:1255-1266）：手写 `s.reloginFail[acct]=5; delete(...); 断言 0` 直接测 `delete` 标准语义恒绿（F13-i1 同款"手写实现语义当断言"）。真实"失败保留增长/成功清零"路径由 TestReloginFailureKeepsBackoff/TestReloginSuccessResetsBackoff 覆盖。**修复**：删除后半段 + 注释指向（与已删的 TestReloginFailureResetsCounter 同链条收敛） |
| B14-N2 | 观察 | 时钟校准计时用本地钟（scheduler.go:301/311 time.Now），提交/探测用对齐钟——两套时间基相差实测 ~640ms，失败退避/成功闸门边界偏移 ≤1s 可忽略。观察不修 |
| B14-I2 | 观察 | **M1 定案**：HTTP 侧 ProbeForAccount/ProbeNow 未接入 probing/lastProbe 单飞（F12-B2 只护 probe()）。精确触发窗口推演：正常时序 30s 探测间隔 < 快照 40s TTL 几乎从不过期；最坏 15s 超时挂起 + 快照过期拉宽到 ~25s 且恰逢前端刷新才直打一次，频率"每 30s 周期至多多 1 次"，不足以触发平台熔断。接入单飞会破坏"管理员刷新强制拿最新数据"语义 + 临门 2s 盯守期误命中卡顿。**维持观察，下轮按需** |

**第 13 轮遗留专项回归（本后端 agent 覆盖）**：F13-C1 前作 flushTargets rev 守卫、F13-C2 卸载孤儿链、m1 票据消费顺序、m2 锁内 DB 写——均收敛，无新问题。

## 二、前端发现与修复状态（第 14 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F14-03** | MINOR | **选课大厅窗口关闭/学期无发布时空态缺失**：publishes 恒空 → tabs.length===0 → 主选课区整个不渲染，页面只剩搜索栏+倒计时+空白主体，无任何"窗口已关闭/暂无可选课程"提示；顶部徽章误显 `SELECTED n/0`。**修复**：`!isLoading && !isError && tabs.length===0` 渲染一行中性空态卡（"当前无可选课程批次（选课窗口未开放或已关闭）"+ 次行"窗口开放后课程列表将自动出现"）；徽章分母 `publishes.length>0` 才显示（不再 n/0）；主区注释标注 F14-03。tsc + vite build 通过 |
| F14-01 | 观察 | **防抖保存 build() 保存内容侧缺口**：F7-01/F13-C1 只防"触发侧"，`build()`（400ms 后）实时读 publishesRef 为空时 `targets=[]` 仍可整包 PUT。核实：唯一实触发窗口是"开窗瞬间平台短暂清空 publishes + 用户最后动作在 400ms 内"的微窗口（窗口关闭后 pick() 不可达、平台清空与用户操作无因果）。**决策观察不修**（F7-01"绝不因 publishes 变化触发保存"已覆盖触发侧；该缺口与 F13-C1 修复场景同源，若未来需要可加"selected 非空 + publishes 空则跳过本次"守卫） |
| F14-02 | 观察 | **返回按钮"飞行中 PUT + 最后一次改动"丢最后快照**：savingRef=true 分支只置 dirty，finally 的脏补发被 unmountedRef 截断。核实成立但概率极低（百毫秒窗口 + 恰在飞行中再改再返回），数据由下次进入重新加载（不改会丢的那份是"最后一次改动"，非整批）。观察记录 |
| F14-04 | 观察 | **App.tsx 派生 effect `accounts` 依赖每渲染变化**（Object.keys 新数组）：effect 每渲染执行，内部条件式 setState 无重渲染副作用，仅轻微多余。观察不修 |

## 三、修复细节（本轮 4 项生产改动 + 1 项测试收敛，独立 commit）

- **B14-M1**（commit 32383cc 后端）：TestWindowOpenSubmitsWithoutProbeReset 契约修正（openTime 过去 1 秒 + 注释）——恒红失效契约转绿
- **B14-M2**（commit 41e4b3d 后端）：TestAdminStatsWindowOpenedUsesScheduler 后半段补"与 WindowOpened() 同源"断言——恒绿无断言契约真实验证
- **B14-N1**（commit 3c34ea7 后端）：handler.go 两处 AppendLog("admin") → d.AdminNameValue()——日志账号硬编码擦除
- **B14-I1**（并入 32383cc 后端）：TestReloginBackoffCappedAndReset 删除测 map 语义的无效后半段 + 注释指向真实路径测试
- **F14-03**（commit 3923fea 前端）：Select.tsx 空态卡 + 徽章分母兜底——tsc + vite build 通过

## 四、回归证据（提交时点 + 收尾复跑）

- `cd backend && go build ./... && go vet ./...` — 通过
- `cd backend && go test -race ./...` — 全包绿（api 23.65s / scheduler 23.47s / store 12.87s；config/db/runtime/secure/session/zhidao cached）
- 专项：TestWindowOpenSubmitsWithoutProbeReset（修复后 0.29s PASS）/ TestAdminStatsWindowOpenedUsesScheduler（修复后 0.39s PASS）/ TestReloginBackoffCappedAndReset / TestReloginBackoffWindowBlocksManualTriggers / TestReloginFailureKeepsBackoff 全 PASS
- `cd web && npx tsc --noEmit` — 通过；`npx vite build` — 通过（406.98 kB / gzip 121.99 kB）

---

## 提交索引（本轮 4 个 commit）

```
32383cc fix(backend): 第14轮B14-M1测试契约修正(openTime过去1秒, tick守卫放行提交而探测仍被lastProbe节流挡)+B14-I1删除测map语义的无效段
41e4b3d fix(backend): 第14轮B14-M2补全TestAdminStatsWindowOpenedUsesScheduler后半段断言(与WindowOpened同源, 契约从未被验证)
3c34ea7 fix(backend): 第14轮B14-N1管理端日志账号硬编码擦除(AppendLog("admin")→AdminNameValue())
3923fea feat(web): 第14轮F14-03选课大厅空态兜底(窗口关闭无发布提示卡+徽章分母兜底)
(review-round14.md + CLAUDE.md 沉淀)
```

## 下轮待办

- HTTP 侧探测接入全校单飞/节流（B14-I2 定案维持观察，触发条件已精确化——最坏每 30s 周期至多多 1 次直打；若未来平台熔断更敏感再落地）
- F14-01 防抖保存内容侧守卫（selected 非空 + publishes 空则跳过——微窗口数据丢失，记录不修）
- F14-02 返回链路最后快照截断（极低概率，记录不修）
- 激活弹窗焦点陷阱（F6-02 长期遗留，Radix Dialog 迁移）
- Dashboard 消费 window_closed 显示"已关闭"（F13-B3 观察延续）
