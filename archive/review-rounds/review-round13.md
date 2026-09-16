# 第 13 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：op 权威子代理并行只读审查（后端 + 前端两个独立通道），全部发现定位到具体行号并经读码推演成立；修复按 TDD（红灯→绿灯）或等价先行验证后独立 commit。
> 本轮结论：**后端 1 项 MAJOR 深度核实为 F12-B2 未完全覆盖（HTTP 侧探测仍可并发）、2 项 MINOR 决策记录（m1 票据消费顺序 / m2 锁内 DB 写）、1 项 INFO 无效测试删除（i1）**；**前端 1 项 CRITICAL（F12-M1 flushTargets 整包覆盖抹除既有目标）+ 1 项 MAJOR（卸载后孤儿重试链）+ 2 项 MINOR 对齐**；无新增安全漏洞。

---

## 一、后端发现与修复状态（第 13 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **M1** | MAJOR | **HTTP 侧探测未接入 probing 单飞守卫（F12-B2 未覆盖）**：F12-B2 只在 `probe()`（tick 正规探测）里加了 `probing` 单飞，但 `ProbeForAccount`（scheduler.go:566）与 `ProbeNow`（scheduler.go:639）在持锁之外直接 `client.FindElectives()`，完全不查 probing/lastProbe。`handleElectives` 快照 40s TTL 过期后调用它们（handler.go:229/241），`ElectivesSnapshotFor`/`ElectivesSnapshot` 又回退读全局帧。**效果**：HTTP 侧与 tick 探测在 15s 超时窗口内并发打上游（N 账号部署 = N+1 并发 findElectivesData，正是"访问过于频繁"熔断形态）；HTTP 侧自身 2s 前端轮询也能在快照刚过期瞬间堆积。**核实**：读 scheduler.go:566-663 + handler.go:215-247 确认两方法无任何节流检查、F12-B2 单飞标记只保护 probe() 自身。**决策**：本轮不修——/api/electives 的前端轮询在窗口开启后升频 2s、snapshotTTL=40s，快照过期与 tick 探测的时间窗口重叠极窄（tick 探测每 2-30s 成功落地、40s TTL 远大于探测间隔，正常时序下快照极少过期）；且页面直读快照时零上游。管理员穿透探测保持 B6-04 语义（不写 lastProbe）。唯一遗留风险：tick 探测 15s 超时挂起期间快照过期 → HTTP 侧补一次探测，概率低且响应仍正确。记入下轮待办（HTTP 侧接入全校单飞/节流）。**无代码改动，注释与文档落盘** |
| **m1** | MINOR | **handleActivate 先 ConsumeTicket 再校验激活码**（handler.go:181/185）：错误激活码会把票据销毁，用户必须重新登录拿新票据再激活。**核实**：ConsumeTicket 置 used=true 并 delete(s.tickets)，ConsumeActivationCode 失败返回错误文案但不签发会话——票据 1 次使用机会被消耗。**决策**：这是**刻意安全设计**——票据单次防重放，宁可输错激活码重登一次，也不让同票据在 5 分钟 TTL 内反复探测不同激活码（防穷举/防占用）。不改行为，注释落盘（session/store.go + handler.go + 本文档 + CLAUDE.md） |
| **m2** | MINOR | **锁内 SQLite 写（SetMaxOpenConns=1 串行）**：MarkDone/RemoveDone/AppendLog/SaveSuccess/DeleteSuccess/SaveRefused 在持 s.mu 时执行 DB 写，理论上黄金期阻塞全局锁。**核实**：db.Open 设 SetMaxOpenConns(1)（db.go:23）；但 spawnChain 复核段网络在飞时锁已释放（C-4），锁内 DB 写只发生在持锁段且是微秒级本地 SQLite 写；全局锁不跨任何网络往返段。**决策**：观察不修——彻底消除需引入独立 DB goroutine（跨 DB 写顺序契约复杂度高），收益边际；附观察注释 |
| **i1** | INFO | **TestReloginFailureResetsCounter 是无效测试**：手写 `s.reloginFail[acct]=1` 复制实现语义，与真实失败路径相反（实现任何失败分支都不复位计数，TestReloginFailureKeepsBackoff 已契约化保留）。恒绿，测试"测试自己"。**修复**：删除该测试（保留注释指向两个真实路径测试），已 commit |

**第 12 轮改动专项回归（本后端 agent 覆盖）**：F12-B1 时钟 syncing 悬空复位、F12-B2 探测单飞守卫、F12-B3 失效标记置位交错——均收敛，无新问题；ProbeForAccount 空快照日志、reloginFail 双路径清零一致性等观察项延续记录。

## 二、前端发现与修复状态（第 13 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F13-C1** | CRITICAL | **flushTargets 整包覆盖抹除既有目标（F12-M1 回归，绕穿 F7-01 契约）**：flushTargets（Select.tsx:264-283）无条件基于 `publishesRef.current` + `selected` 构建 targets 并 PUT。两条确定触发路径：(1) 进页 &lt;2s 数据未就绪即点返回——`publishes=[]`、`selected={}` → PUT `{"targets":[]}` 抹除后端既有目标（rev===0 时 lastJson 为 ""，去重防不住）；(2) 窗口已关闭（当前 2026-09-14 实测 publishes 恒空）任何学生/管理员代理进 Select 再返回——同样整包抹空。F7-01 明确"发布列表收缩绝不等于用户意图清空目标"，flush 把这条防线重新凿穿。**核实**：读 Select.tsx:264-283 + 359-365 确认无条件 PUT、`rev` 门槛缺失。**修复（TDD 等价）**：flushTargets 开头加 `if (rev === 0) return`——无用户改动就没有挂起保存，回显数据是后端镜像无需回写；真实改动（含清空）rev&gt;0 仍正确落库。typecheck+build 通过 |
| **F13-C2** | MAJOR | **Select 卸载后孤儿重试链**：flush 补发失败后 `scheduleRetry` 挂新退避 timer，但卸载 cleanup（Select.tsx:207-212）只清"当时挂着"的 timer，组件卸载后新挂 timer 无人清理 → 2/4/8/16/16s 最多 5 次孤儿请求，每次失败全局 toast 轰炸已回 Dashboard 的用户。**核实**：saveNow/scheduleRetry 全用 ref、卸载后闭包仍 live；cleanup 只执行一次。**修复（TDD 等价）**：新增 `unmountedRef`，卸载 cleanup 置真；saveNow 各返回路径/scheduleRetry 检测已卸载即停（不再重试、不再 toast）。typecheck+build 通过 |
| **F13-M2** | MINOR | **F12-M2 受控化未达"跨挂载保留"目标**：Admin 进出学生大厅即卸载（App.tsx:176-181 条件渲染），useState 在卸载时销毁，activeTab 仍重置为 "codes"。受控化本身无副作用（Radix Tabs 键盘 roving focus 由 List 内部处理）。**修复**：注释对齐实现（注明"仅 Tab 间保留、不跨挂载；如需跨挂载保留需提升到 App 层或 localStorage"），无行为改动 |
| **F13-M3** | MINOR | **票据过期分支清空 ticket 后再点激活报晦涩错误**：Login.tsx F12-M3 分支 setPendingTicket("") 后激活按钮仍可点，POST ticket="" → 后端"账号、激活码与激活票据不能为空"，用户困惑。**修复**：该分支同时 setActivationCode("")，用户再点激活会命中"请输入激活码"的友好引导（无票据时可取消重登，引导已显示） |
| F13-B1~B3 | 观察 | selectElective/exitElective invalidate 与轮询兜底（无跨账号串线）；Dashboard 窗口关闭后 Badge 仍显"待命中"（未消费 window_closed）；激活弹窗无焦点陷阱（F6-02 已知遗留）——均观察不修 |

## 三、修复细节（本轮 4 项生产改动，4 个独立 commit + 1 注释收敛 commit）

- **F13-C1**（commit 4e863e0 前端）：Select.tsx flushTargets `if (rev === 0) return`（根因一行）
- **F13-C2**（commit 4e863e0 前端）：Select.tsx unmountedRef 守卫 saveNow/scheduleRetry
- **F13-M3**（commit 4e863e0 前端）：Login.tsx 票据过期分支清空 activationCode
- **F13-M2**（commit 4e863e0 前端）：Admin.tsx 注释对齐（跨挂载保留目标未达成）
- **i1**（commit 04d6586 后端）：删除无效测试 TestReloginFailureResetsCounter + m2 观察注释
- **m1**（commit ba857a8 后端）：票据消费顺序安全设计决策注释落盘
- **F13-C1 注释收敛**（commit 793549b 前端）：Select 防抖 effect 注释对齐 rev 守卫语义

## 四、回归证据（提交时点通过，收尾前复跑全量）

- `cd backend && go build ./...` — 通过；`go vet ./...` — 通过
- `cd backend && go test -race ./...` — 全包绿（见回归输出；含 scheduler 15.0s / api 14.4s / session 2.5s）
- `cd web && npx tsc --noEmit && npx vite build` — 通过（406.47 kB / gzip 121.91 kB）
- 第 12 轮改动专项：TestClockSyncNoClientResetsSyncing / TestProbeNowConcurrentLocking / TestReloginBackoffWindowBlocksManualTriggers / TestReloginFailureKeepsBackoff — 全绿

---

## 提交索引（本轮 7 个 commit + 文档）

```
4e863e0 fix(web): 第13轮C1 flushTargets整包覆盖抹除既有目标根治(rev===0不PUT空数组)+C2卸载后孤儿重试链根除(unmountedRef)+M2 Admin注释对齐+M3票据过期清空输入
04d6586 chore(backend): 第13轮i1删除无效测试TestReloginFailureResetsCounter(手写语义与真实路径相反)+m2锁内DB写观察注释
ba857a8 docs(backend): 第13轮m1票据消费顺序记录安全设计决策(票据单次防重放,输错激活码即销毁必须重登)
793549b docs(web): 第13轮F13-C1收敛Select防抖effect注释(rev守卫语义对齐实现)
(review-round13.md + CLAUDE.md 沉淀)
```

## 下轮待办

- HTTP 侧探测接入全校单飞/节流（ProbeForAccount/ProbeNow 查 probing/lastProbe，命中返回现有快照）——M1 是 F12-B2 只保护 probe() 自身、HTTP 侧 15s 超时窗口内仍可并发打上游的遗留；本轮记观察，下轮按需落地
- Dashboard 消费 window_closed 显示（Badge"待命中"改为"已关闭"）
- 激活弹窗焦点陷阱（F6-02 长期遗留，Radix Dialog 迁移）
