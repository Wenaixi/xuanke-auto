# 第 16 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道，均附"宁缺毋滥 + 文件行号 + 触发场景"模板），发现全部经主通道逐一读码推演定案；修复独立 commit。
> 本轮结论：**前端 1 项 MAJOR（F16-01 目标保存发布 id 漂移假清空）+ 后端 1 项 MINOR 整洁收尾（B16-M1 退避基准统一对齐钟）**；无 CRITICAL，无新安全漏洞；第 15 轮遗留观察项经深度复查多数收敛为"不成立/接受"。

---

## 一、后端发现与修复状态（第 16 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B16-M1** | MINOR | **退避截止基准混用（B15-M3 的收敛点）**：`markRateLimitedLocked`（scheduler.go:1191）写退避截止用本地钟 `time.Now().Add(d)`，`spawnChain`（1021）判到期用 `nowAlignedLocked()`——同一退避期两套时间基（实测相差 ~640ms）边界同一语义，30s 风控退避偏移 ≤640ms 无实质影响。**修复**：写入改用 `nowAlignedLocked()`，与读侧 `isRateLimitedLocked` 同源（scheduler.go:1171）。build+vet+TestRateLimitBackoff 通过 |
| B16-M2 | 观察 | HTTP 侧 `ProbeForAccount`/`ProbeNow` 未接入 probing/lastProbe 单飞（B14-I2 延续）：正常时序 30s 探测 < 快照 40s TTL 几乎从不过期，最坏每 30s 周期至多多 1 次直打，不足以熔断；接入单飞会破坏"管理员刷新强制拿最新"语义 + 临门 2s 误命中卡顿。**维持观察不修**（agent 亦判接受） |
| B16-I1 | 观察 | `Deps.Decrypt` 死字段（handler.go:39-40，注释自 B10-08 起"仅注入备用未消费"）。纯死代码无触发；删除需动 constructor 签名与多处调用，按"精准修改"观察不修 |
| B16-I2 | 观察 | `probe()` 每目标账号各起 goroutine 调 `ProbeForAccount`（scheduler.go:745-749）不受 probing 保护，N 账号开窗期每 30s N+1 并发——"按账号年级隔离快照"的刻意设计代价，已记录接受 |

**第 15 轮遗留专项复查（agent 逐项推演，全部收敛）**：
- **B15-M1（inflight 所有权竞态）→ 不成立**：`TryAcquireSubmit`（1366-1384）与 `spawnChain`（1044-1051）对 `inflight` 所有访问均持 `s.mu`，网络调用（SelectClass 1063）在锁外，sync.Once release 幂等——手动/自动通过 s.mu + 原子化设置/释放正确同步，无并发双发包。
- **B15-M3（时间基准混用）→ 收敛为 B16-M1**（已修）。tick 提交闸门/submitAll/isRateLimitedLocked 已全部用对齐钟，唯一混用点就在 markRateLimitedLocked 写入基准。
- HTTP 侧探测节流 → 维持 B16-M2 观察。
- flushTargets 后端对应缺口 → 无。`handleSetTargets` 有凭据存在性校验（B15-M4）+ 事务化 SetTargets + 100 条上限，普通会话账号来自已验证会话绑定，无孤儿行路径。
- 激活码/票据/会话 TTL 残留边界 → 无缺陷。票据单次+绑定+5min TTL 消费即删、会话 12h+sweeper、激活码原子扣次、RevokeAccount 即时吊销；TTL 用本地钟与调度器对齐钟解耦（会话层不依赖教务时钟，正确）。

## 二、前端发现与修复状态（第 16 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F16-01** | MAJOR | **目标保存"发布 id 漂移"假清空（publishesMissing 渲染期旧值）**：`publishesMissing`（Select.tsx:284）是渲染期常量，防抖回调（333-347）在 400ms 后读旧闭包值。触发链：开窗瞬间平台 `electives` 返回 `publishes=[]` → `publishesMissing=true`；用户此刻点选（按钮可点）→ rev+1、selected 按旧发布 id 布局；2s 轮询发布恢复（id 全变但字段一样）→ 重渲染 publishesMissing 翻 false；但**本次 400ms 防抖回调持有旧闭包 publishesMissing=false** → 守卫不触发 → `build()` 拿最新 publishesRef（全新发布 id）联查 `selected[旧 publish_id]` → 全 undefined → **PUT 空 targets 抹掉后端所有目标**——黄金期空转 + 后续重配置成本极高。这是全站碎片保存机制全力守护的失败模式，防御点漏在"判定时刻取值陈旧"。**修复**：抽纯函数 `targetsUseCurrentPublishes(targets, publishes)`——消费时刻对 build 结果做"每个 publish_id 属于当前 publishesRef"全数校验，任一漂移即 `dirtyRef=true` 跳过绝不 PUT；`flushTargets()` 与防抖回调两条腿同闸（比把 publishesMissing 改同步 ref 更短，且覆盖"集合非空但 key 漂移"偏态）。tsc 通过 |
| F16-02 | 观察 | 横幅 `window_closed` 优先分支把"窗口从未开过"与"开过已关闭"混为一类（open_time 过去 + 平台从未置 opened + 空快照）。已不再与 F14-03 空态卡矛盾，仅文案含糊。观察不修 |
| F16-03 | 观察 | 管理员删最后一个学生账号的 UX 警示不足（RevokeAccount 即时吊销全校会话）。行为正确，仅文案。观察记录 |
| F16-04 | 观察 | `/api/logout` 未 await（登出后立刻刷新时请求可能被中断，服务端令牌到 12h 才过期）。按既有威胁模型"刷新即自毁"低影响。观察不修 |
| F16-I1~I4 | 观察 | INFO：401 归属第三级 current 无触发行、服务端规范化回执对齐、config refetch 时序闪动无危害、Toast/组件纯 UI 无泄漏 |

**无害性结论**：F15-01（假清空守卫）/ F15-03（横幅状态机）/ F15-07（登出清代理）本轮复查全部收干净；F7-01 publishesRef 读点全受保护；F8-01/F13-C2/F12-M1 一致；F10-05 401 归属无遗漏。

## 三、修复细节（本轮 1 前端 MAJOR + 1 后端 MINOR，独立 commit）

- **F16-01**（commit d301820 前端）：Select.tsx `targetsUseCurrentPublishes` 全数校验——build 结果 publish_id 必属当前 publishesRef，防抖 + flushTargets 双闸
- **B16-M1**（commit 46c0145 后端）：scheduler.go `markRateLimitedLocked` 写入基准改 `nowAlignedLocked()`——退避读写同源对齐钟

## 四、回归证据（提交时点 + 收尾复跑）

- `cd backend && go build ./... && go vet ./...` — 通过
- `cd backend && go test -race ./...` — 全包绿（api 16.6s / scheduler 13.6s / store 3.1s；config/db/runtime/secure/session/zhidao cached）
- 专项：TestRateLimitBackoff / TestSubmitSuspendedWhenOpenTimeCleared / TestProbeInterval* 全 PASS（scheduler 7.5s）
- `cd web && npx tsc --noEmit` — 通过；`npx vite build` — 通过（407.42 kB / gzip 122.07 kB）

---

## 提交索引（本轮 2 个 commit）

```
d301820 fix(web): 第16轮F16-01目标保存发布id漂移假清空根治（targetsUseCurrentPublishes全数校验, 防抖+flush双闸）
46c0145 fix(backend): 第16轮B16-M1退避截止基准与读侧统一对齐钟（markRateLimitedLocked写入改nowAlignedLocked）
(review-round16.md + CLAUDE.md 沉淀)
```

## 下轮待办

- HTTP 侧探测接入全校单飞/节流（B14-I2/B16-M2 定案维持观察，最坏每 30s 周期至多多 1 次直打；若平台熔断更敏感再落地）
- B16-I1 Decrypt 死字段删除（需动 constructor 签名，观察）
- F16-02 横幅混合文案 / F16-03 删最后账号警示 / F16-04 logout await（均观察）
- 激活弹窗焦点陷阱（F6-02 长期遗留，Radix Dialog 迁移）
- Dashboard 消费 window_closed 显示"已关闭"（F13-B3 观察延续）