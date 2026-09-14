# 第 12 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 + 前端两个独立通道），全部发现定位到具体行号并经读码推演（部分写最小复现测试验证起）成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**后端 3 项真实修复（F12-B1 时钟校准 syncing 悬空 / F12-B2 探测单飞守卫 / F12-B3 失效标记置位交错）**，**前端 4 项真实修复（F12-M1 退出 flush 目标 / F12-M2 Admin Tab 受控 / F12-M3 激活失败引导 / F12-M4 次要文字灰阶）**；无 CRITICAL；probe 并发合并链与 reloginFail 清零链在本轮主体收敛。

---

## 一、后端发现与修复状态（F12-B 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F12-B1** | MAJOR | **时钟校准 syncing 无账号场景悬空永久休眠**：`maybeSyncClock` 发起段置 `syncing=true` 后，复位只在异步 goroutine 内（scheduler.go:283-308 原）。空库部署/账号全删/客户端非 TimeSyncer 时 `AnyClient` 失败或断言失败 → goroutine 从不 spawn → syncing 恒 true，后续每个 tick 在 `if s.syncing` 处直接返回，**时钟校准从启动起永久失效、clockOffset 恒 0 且无任何错误日志**。空库部署首个 300ms tick 即触发。既有四时钟测试的 fakeAccts.AnyClient() 恒 true，无账号场景未覆盖。**修复**：先确认有可同步客户端再置位，无客户端时复位 syncing（并补 `clients==nil` 空指针防御）；新测试 `TestClockSyncNoClientResetsSyncing` 验证复位 + 账号就绪后再同步仍能落地 |
| **F12-B2** | MAJOR | **probe 无在途守卫：N+1 并发向上游轰炸**：`probe()` 为每目标账号 go ProbeForAccount 合并发 N+1 个 findElectivesData（scheduler.go probe() 循环）；HTTP 侧 `/api/electives` 快照 40s TTL 过期后 `ProbeForAccount`/`ProbeNow`（scheduler.go:549/622）完全不查节流闸门并发直打；`probeIntervalFor` 只在 `now.After(open) && WindowClosed()` 时降 30s——整个可报名窗口（快照非空）探测恒 2s 节奏，N 账号开窗全期 5.5 req/s 直打平台，正是此前踩过的"访问过于频繁 1 分钟熔断导致课程拉空"形态。**修复**：`probing` 单飞守卫，持锁置位保证置位-检查原子（与 B11-A1 零值守卫同款窗口），命中直接放弃本次（最长推迟一个 300ms tick，临门/黄金期无实质损失），任何返回路径统一复位 |
| **F12-B3** | MINOR | **MarkTokenValid 与在途重登 goroutine 的 tokenValid 置位交错**：`tokenValid=true` 原在重登 goroutine 开头（无 reloginMu），`MarkTokenValid`（手动登录成功，issueSession 调）只持 s.mu——手动登录清除 tokenValid/relogging/reloginFail 后，在途重登 goroutine 完成后覆写 tokenValid=true，前端 /state 短暂回"已失效·自动恢复中"后 jitter。无提交闸门危害（spawnChain 只查 relogging），仅显示层闪动 + 多余一次登录（受 gateWait 2 次/分钟收敛）。**修复**：tokenValid 置位从 goroutine 开头移入决策段（与发起同持 reloginMu+s.mu 两把锁，语义"发起重登即 token 已知失效"），MarkTokenValid 对齐锁序补取 reloginMu——两条路径串行化杜绝半态读 |
| B12-B4 | 观察 | isWindowClosedError 文案匹配面过窄（不含"无效的课程ID"）：与 C-3/B11-A1 重叠收窄，观察不修 |
| B12-B5 | 观察 | openTimeNow() 单 tick 内多路非原子读取（probeIntervalFor 两次 + probe 一次）：管理员热改瞬间 1-2 tick 基准不一致，低频无实质，观察 |
| B12-B6 | 观察 | probe() 账号 goroutine 无生命周期管理（Stop 不等待收敛）：单进程常驻无行动影响，观察 |
| B12-B7 | 观察 | HasProbed 语义边界（大量账号级探测后仍可能 false）：字段定义语义，观察 |
| B12-B8 | 观察 | reloginResults 缓冲 8 非阻塞丢补探测信号：黄金期由 tick"到点即提交"兜底，观察 |

## 二、前端发现与修复状态（F12-M 系列）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F12-M1** | MAJOR | **Select 页退出未 flush 防抖中目标保存，用户改动可丢失**：自动保存走 400ms 防抖 effect，卸载 `clearTimeout` 清掉未到期 timer；顶栏「返回控制台」直调 onDone，路径无任何"立即保存当前 selected"的 flush。最后一次点选到返回间隔 <400ms 时整批目标永不 PUT——抢课场景配置完马上回控制台看状态是高频操作。**修复**：返回按钮外包 handleDone，内先 flushTargets（build() 出 targetRef 当前快照，复用 lastJson 去重 + savingRef/dirtyRef 串行化，绝不与飞行中 PUT 乱序），再调 onDone |
| **F12-M2** | MINOR | **Admin 五 Tab 状态在进出学生大厅后丢失**：非受控 `<Tabs defaultValue="codes">`（Admin.tsx:116）只在首次挂载生效，进出学生大厅（Admin 卸载重挂）后 Tab 恒回落"激活码"，会话性现场丢失。**修复**：改为受控 `value={activeTab} onValueChange={setActiveTab}`，useState 提升到顶层跨挂载保留 |
| **F12-M3** | MINOR | **激活失败（票据过期/已被消费）弹窗无重新登录引导**：后端票据 5 分钟单次，激活响应在网络层丢失后票据已被消费、账号已激活，用户反复点激活恒失败（后端"激活票据无效或已过期"），误以为激活码有问题卡死胡同。**修复**：捕获该文案时提示"票据已过期/已使用，请取消后重新登录即可进入"并清 pendingTicket |
| **F12-M4** | MINOR | **--fg-muted 误标纯白 #ffffff 与 --fg 同色**：设计规范声明的次要冷灰（#a1a1aa）与实现分叉，Card 描述/TabsList/Table 表头/Toast 描述/outline 按钮全站次要信息失去视觉层级差。**修复**：--fg-muted 对齐 #a1a1aa、--fg-dim 补灰 #71717a |

## 三、修复细节（本轮 7 项生产改动，7 个独立 commit，均验证后提交）

- **F12-B1**（commit 8f8251b）：maybeSyncClock 先确认客户端再置位 + clients==nil 防御；TDD：TestClockSyncNoClientResetsSyncing 修复前 panic（nil 解引用）→修复后 PASS
- **F12-B2**（commit 14e2546）：probe() probing 单飞守卫，任一返回路径复位
- **F12-B3**（commit 9ad1e19）：tokenValid 置位入决策段 + MarkTokenValid 补取 reloginMu
- **F12-M1**（commit 27b1788）：Select.tsx flushTargets + 返回按钮 handleDone
- **F12-M2**（commit 5180413）：Admin.tsx Tabs 受控化
- **F12-M3**（commit 0673143）：Login.tsx 激活失败分场景引导
- **F12-M4**（commit af4d43a）：global.css --fg-muted/--fg-dim 灰阶对齐

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `cd backend && go test -race ./...` — 全包绿（见回归输出）
- `cd web && npx tsc -b && npx vite build` — 通过（406.34 kB / gzip 121.87 kB）
- 提交序列见提交索引

---

## 提交索引（本轮 7 个独立 commit + 文档）

```
8f8251b fix(backend): 第12轮F12-B1时钟校准syncing无账号场景悬空复位...
27b1788 fix(web): 第12轮F12-M1选课大厅退出前flush挂起的目标保存...
5180413 fix(web): 第12轮F12-M2管理员Tab受控化...
0673143 fix(web): 第12轮F12-M3激活失败分场景引导...
af4d43a fix(web): 第12轮F12-M4次要文字灰阶落地...
14e2546 fix(backend): 第12轮F12-B2探测单飞守卫...
9ad1e19 fix(backend): 第12轮F12-B3失效标记置位交错收敛...
(review-round12.md + CLAUDE.md 沉淀)
```

## 下轮待办（合并链收敛后只剩轻量追踪）

- HTTP 侧探测接入全校节流闸门（ProbeForAccount/ProbeNow 快照过期时先查 lastProbe，命中返回现有快照 + "探测节流中"提示）——F12-B2 单飞守卫已拦同一时刻的并发，但 15s 超时窗口内快照过期请求仍会直打一次，收益边际观察/下轮按需落地
- relaoginFail 双路径清零一致性残余观察（成功分支与 MarkTokenValid 的 reloginAt 刷新差异）——已推演无提交闸门危害，下轮不再主动追
- F12-M1 flushTargets 的端到端回归观察（连点 + 立即返回场景）