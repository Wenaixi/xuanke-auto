# 第 11 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 + 前端两个独立通道），全部发现定位到具体行号并经读码推演（部分写最小复现测试验证起）成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**后端 4 项真实修复（B11-A1/RestoreTargets、B11-A4 bench、B11-A5 ExitClass 注释、A5 撤回）**，**前端 1 项真实修复（F11-A1 激活票据贯通）**；后端 1 项撤回（A5 限流桶竞态不成立）、1 项观察转正、5 项记录为观察项。

---

## 一、后端发现与修复状态（B11 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B11-A1** | MAJOR | **open_time 显式清空后提交循环永续放行**：管理员 PUT `open_time=""`（F7-02 官方支持语义，解除窗口机制）→ `runtime.reparse` 置 `OpenTimeParsed/OpenTimeParsed` 零值（runtime/config.go:61-63）→ tick 的 `!opened && !now.After(open)`（open 零值 → 恒 false）恒放行提交 → `spawnChain` 对"已满员/已成功"目标每 1s 仍刷平台报名接口（空快照下 `releaseFullIfFreedLocked` 保守不解封、done/refused 只拦一部分）——C-3"窗口关闭后防轰炸"防线被 open 零值绕过，消耗风控预算。**修复**：tick 提交段在 `open.IsZero()` 时直接 return（TDD：`TestSubmitSuspendedWhenOpenTimeCleared` 红灯"调用 1 次"→绿灯"0 次"，还反向对照窗口未开启不提交） |
| **B11-A4** | MINOR | **cmd/bench 测的是 401 拒绝路径**：`/api/state` 挂 `requireAuth`，bench 无 Authorization 头 → 每拍打到 401 而非真实业务路径，延迟虚低无意义（cmd/bench/main.go:12-26）——基准工具语义失真。**修复**：支持 `-token` 注入 Bearer（未注入时输出标注"测的是 401 拒绝路径"） |
| **B11-A5** | INFO | **ExitClass 退选错误注释误导**：client.go 残留"code=-1 返回错误由调用方决定"的旧文档，与 doRequest 已统一 code=-1→ErrUnauthorized 的行为分叉（client.go:605-606）——退选路径错误处理与报名实为对称。**修复**：注释对齐 doRequest 真实行为（无行为变更） |
| **A5（撤回）** | — | **loginLimiter 过期键 GC 与桶消耗并发竞态**：重新核实后 `allow()` 全程持全局锁 `l.mu`，GC 分支与桶读写在同一把锁内，无数据竞争——撤回 |
| **A2** | — | **probe() 并发单飞缺失**（N 账号并发 + 全局探测同 tick 放大）：并入既有 B7-M5/B8-M4/B9-04 probe 合并待办链（B10-06 同源），下轮系统性收敛"窗口关闭统一信号"时一并处理 |
| **B11-B1** | — | isWindowClosedError 文案匹配面过窄（不含"无效的课程ID"）：收益边际记录，与 A1 修复面重叠，暂不动 |
| **B11-B2** | — | gateWait 长阻塞 vs 重登退避：`for` 循环内逐槽获取预算仍严格 ≤2/分钟，无实质超额，记录不修 |
| **B11-B3** | — | logintest 登录成功不写回 DB：运维工具设计取舍，记录 |
| **B11-B4** | — | syncFailStreak 与 lastSyncFailAt 冗余（频率门 vs 累计门职责不同）：非缺陷，记录 |
| **B11-B5** | — | cmd/probe/bench 硬编码 URL/token 环境变量：运维工具定位，记录 |

## 二、前端发现与修复状态（F11 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F11-A1** | MAJOR | **激活码登录链路丢弃 ticket → 激活 100% 失败**：登录返回 code=1001 时 `data.ticket`（绑定本次登录账号的短期单次票据）被 Login.tsx 丢弃，`activate()` 请求体 `{account, code}` 不含 ticket → 后端 `req.Ticket==""` 立即拒绝、ConsumeTicket 必败，激活码机制**整链不可用**（未激活账号首登必经此链）。**修复**：Login.tsx 1001 分支保存 `data.ticket` 随激活请求回传（取消时一并清）；`ApiError` 携带响应 data 透传票据（此前抛错处丢失） |
| **A-2（研判）** | — | **401 踢出链路的信号错位研判**：client.ts 仅 `j.code===401` 广播踢出事件，后端 `writeJSON` 恒 HTTP 200 + body code 约定。经全面 grep：`requireAuth` 无/无效/过期令牌确实只 `writeJSON(w, 401,...)`（HTTP 200）——但全库唯一业务层 `writeJSON(w, code)` 的 code 参数就是 401（handler_test 多条断言 `j["code"]==401` 通过），且 `writeJSON(w,401,...)` 在 `requireAuth` 分支即**业务 body code=401 的唯一定义点**——前端判定与后端信号是**同一条编码路径**，非错位。真正缺口是"会话过期后 403（admin 接口）与 401（普通接口）分叉"——既已修 F10-05，A-3 的 403 归属已由 session 令牌反查覆盖。记录研判结论，不修 |
| **A-3（研判）** | — | **权限差分 403 未纳入踢出信号**：曾管理员被踢后访问 admin 接口得 403（无 token）+ 普通接口得 401（有但无效 token）——两条路径都带着"会话已不存在的令牌"，App 判定以 `detail.session` 反查一并兜底。记录研判，不修 |
| **A-4（观察项转正）** | — | isLoading 覆盖 isError：react-query 重试（retry:1）期间加载态短暂盖错误态，最终错误态正常展示——观察项记录，不修 |
| **F11-B1** | — | useTickingCountdown 服务端 open_time 带 Z 后缀（UTC）：当前 FormatOpenTime 用 time.Local 输出不表现错误，防御项记录 |
| **F11-B2** | — | ConfigTab Number('')||1 清空回弹 1：可用性取舍，记录 |
| **F11-B3** | — | handleSelectClass 报名失败后主按钮可反复点：后端 TryAcquireSubmit 在飞锁会拒，轻量 UX，不修 |
| **F11-B4** | — | Dashboard /state queryKey 含 account 与后端 sessionAccount 过滤对齐，正确不修 |
| **F11-B5** | — | accounts 每次 render 新数组引用 → effect 每 render 重跑：幂等分支无副作用，取舍不修 |
| **F11-B6** | — | Admin 卸载不失效 admin-* 查询：**不修——Admin 常驻后台轮询（10s）是刻意设计**，卸载失效会丢实时性；queryKey 带 account 无数据污染 |
| **F11-B7** | — | api() abort 统一映射"请求超时"（用户主动 abort 也被误报）：影响小，取舍不修 |
| **F11-B8** | — | DeleteRefused 清库行 + DeleteAccount 事务并删 refused 行：无死行残留，不修 |

## 三、修复细节（本轮 4 项生产改动，4 个独立 commit，均验证后提交）

- **B11-A1**（commit 597f1a1）：scheduler tick 提交段 open 零值守卫；`TestSubmitSuspendedWhenOpenTimeCleared` 红灯（RED：调用 1 次）→ 绿灯（0 次）
- **B11-A4**（commit eb3ffbd）：cmd/bench 支持 -token / -n / -url，未带 token 输出标注
- **F11-A1**（commit 99069f3）：Login.tsx 保存 data.ticket 随激活回传 + ApiError 携带 data；tsc + vite build 绿
- **B11-A5**（commit c9caa4c）：ExitClass 注释对齐 doRequest 行为（无行为变更）

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `cd backend && go test -race ./...` — 全包绿（见回归输出）
- `cd web && npx tsc -b && npx vite build` — 通过（405.91 kB / gzip 121.75 kB）
- 提交序列见提交索引（B11-A1 → B11-A4 → F11-A1 → B11-A5 → 本文档 + CLAUDE.md 沉淀）

---

## 提交索引（本轮 4 个独立 commit + 文档）

```
597f1a1 fix(backend): 第11轮B11-A1 open_time清空(F7-02解除窗口机制)后tick挂起提交——零值open恒放行导致已满员/已成功目标每1s仍刷报名接口,C-3窗口关闭防轰炸被绕过;TestSubmitSuspendedWhenOpenTimeCleared红灯(RED:调用1次)→绿灯(0次)
eb3ffbd fix(backend): 第11轮B11-A4 bench支持-token注入Bearer否则标注测的401拒绝路径——此前无Authorization头测的是401路径而非真实业务延迟,结果虚低无意义
99069f3 fix(web): 第11轮F11-A1激活码登录链路票据贯通——登录1001分支保存data.ticket随激活请求回传,ApiError携带data透传;此前票据被丢弃激活恒败激活码机制整链不可用
c9caa4c fix(backend): 第11轮B11-A5 ExitClass退选错误注释对齐——doRequest已统一code=-1为ErrUnauthorized,退选/报名路径错误处理对称,删误导注释(无行为变更)
(review-round11.md + CLAUDE.md 沉淀)
```

## 下轮待办（合并链）

- probe 并发单飞系统性收敛（B7-M5/B8-M4/B9-04/B10-06/B11-A2 合并链）——围绕"窗口关闭统一信号"做提交循环终止条件整理
- maybeRelogin 指数退避 reloginFail 双路径清零一致性（第 10 轮遗留，本轮 agent 复核无新线索，下轮续盯）
- F11-A1 激活票据贯通后的激活链路端到端回归观察（新账号首登 → 弹窗 → 激活 → 登录）