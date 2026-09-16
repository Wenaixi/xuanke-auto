# 第 35 轮全模块审查记录（2026-09-16）

> 审查范围：backend 全部模块 + web 全部模块。并行子代理产出：后端全模块只读审查
> （review35-backend，报告已到）、前端全模块只读审查（review35-frontend，已到）。
> 主 gate 逐条现场核实（读源码 + 推演真实触发路径）后决策。本文件为最终版。

## 前端（Select-35 / Admin-35 系列）

### Select-35-01（MAJOR）同发布旧目标在慢首帧 + 用户先点击时被"添加"操作静默整体覆盖
**缺陷**（review35-frontend M-1）：回显合并对"用户已触碰（key 存在）的发布保留现状、绝不
补回旧目标"——若用户本次触碰的发布与后端旧目标**同属一个发布**，旧目标永不进 selected，
防抖保存整包 PUT 只剩用户新点课程 → 旧目标被静默删除。M30-03（/state 首帧晚到不回显）与
Select-31-01（合并逻辑）修复的是"跨发布补进"与"清空防复活"，"同发布 + 添加"分支恰好是
`continue` 落点，修复链未覆盖。
**触发路径**：上次会话发布 P 存了多备选目标 [A,B]（已保存成功）→ 本次进入 /electives 先到、
/state 首帧晚到 → 用户先点课程 C（同发布）→ selected={P:[C]}、rev=1，防抖 400ms → 回显未
完成守卫置脏暂不 PUT → /state 到达 courses=[A,B] → 回显 effect 合并：`P in next` → continue，
A/B 被跳过 → 防抖 effect 重跑 → 五道守卫全过（publishes 非空、next=[C] 非空、publish_id 合法、
回显已完成、selectedCount>0）→ PUT {targets:[C]} 整包替换。后端旧目标 [A,B] 被删为 [C]。
**影响**：用户本意"添加一门"，实际把同发布已保存的多备选整体替换成一门；开窗黄金期自动引擎
按 [C] 抢课，A/B 备选序列被破坏。
**修复**：回显未完成 + 用户首点同发布时，仍把后端旧目标补入该发布——已触碰发布且**非空数组**
= 用户添加了课程，把后端旧目标中用户未勾选的按 priority 序补进（同发布"添加一门"语义，慢首帧
下旧目标尚未回显不补就会被防抖 PUT 整包覆盖删掉）；**空数组** = 用户显式清空该发布（清空语义
绝不复活）。主 gate 用 merge-check.mts 七场景断言脚本验证合并行为（首帧慢/正常/清空/漂移/空
courses 等），修正断言脚本字段形状（ordered courses 用 `{class_id}` 而 selected 条目用 `{id}`）
后七场景全过——组件无 bug，是测试脚本形状错误。
**验证**：npm run build（tsc -b）通过。提交 `916cbab`。

### Admin-35-01（MINOR）ConfigTab save 缺在飞幂等守卫——双击保存发两个并发 PUT
**缺陷**（review35-frontend m-2）：save 只有 `if (!loaded) return`，无 `if (saving) return`；
按钮 disabled={saving || !loaded} 依赖渲染落地延迟，双击可在 disabled 生效前发两个 PUT。
**影响**：请求幂等（同 body）不造成配置回滚，行为危害低；与 F19-02/F21-04 各副作用按钮
"入口先查在飞标记短路"为同族模式，属补齐项。
**修复**：save 入口补 `if (saving) return`。
**验证**：npm run build（tsc -b）通过。

### 可疑项裁决（review35-frontend s-1/s-2/m-3）
- s-1（echoDone 依赖 + 防抖 effect 顶部 resetRetry 在退避期被轮询信号打断）：已推演数据不丢
  （rev>0 时 effect 必然重存），轮询不触发 effect（依赖仅 rev/selected/hasPublishes/echoDone），
  退避节奏打断无实质危害。维持现状，不修。
- s-2（手动报名在飞操作与 filteredClasses 重建的按钮禁用态差异）：子代理自身倾向不修
  （在飞标记存在期间按钮禁用，重建只影响文案不改变行为），维持观察。
- m-3（退避 attempt 计数在"同内容 flush 早退"后残留）：窄时序（flush 触发 + targets===lastJson
  早退），最大代价是失败重试少几档。维持观察，不修。

## 已核无缺陷清单（review35-frontend）

- TDZ/构建类：回显 effect 声明于 const publishes 之前，但用依赖中 data 自行推导，
  tsc -b（npm run build）零 TS2448/TS2454，无 F18-01/F19-01 复发。
- 防抖保存链：savingRef+lastJson 串行化、退避重试、假清空守卫链（F15/F16/F17）消费时刻
  selectedRef/stateDataRef 镜像齐全。
- 回显合并全清空守卫（31-04）、同发布补进（35-01 本轮）语义与 decision 锚一致。
- 手动报名/退选在飞 Set 独立跟踪（M29-01）；Esc/焦点（32-03）；退避 timer 收敛。
- 401/登出/删账号视图态（M29-02/App-34-02）；app 级会话快照清理（M28-01）。
- ConfigTab refetch 回填（R27-01）+ 幂等守卫（35-02 本轮）；btn_type 三向契约（34-03）。

## 后端（B35 系列，2 项确认修复 + 1 项可疑裁决 + 1 项维持观察）

### B35-01（MINOR）api 层 6 处裸 AppendLog 未捕获落库错误（B33-01 契约下沉缺口）
**缺陷**（review35-backend MINOR 1）：scheduler.go 全部落库点已在 B33-01/B34-03 收敛为
`if err := ...; err != nil { log.Printf }`，但 api/handler.go 仍有 6 处直接 `d.Store.AppendLog(...)`
裸调用（120 管理员登录 / 213 账号登录 / 499 目标保存 / 557 注销 / 837 配置更新 / 1002 删除账号）
——日志行丢失被静默吞掉，管理员排查"登录/删号/配置变更"审计线索无声缺失。
**主 gate 核实**：Grep 全文确认 6 处全部裸调用、且 api 包已 import log。审计日志不参与重启恢复
契约（RestoreDone/Refused 只读 success/refused 表），危害低于 B33-01 的落库点，但同一"绝不
静默吞错"规范不应在 api 层断裂。
**决策**：确认修复——6 处全部补 `if err := ...; err != nil { log.Printf("[api] ...落库失败: %v", err) }`，
与 B33-01/B34-03 日志规范对齐。
**验证**：go build/vet 全绿 + `go test -race ./...` 9 包全量通过。

### B35-02（MINOR）windowClosedLocked 判据3 在亚毫秒窗口内独立读取 openTimeNow()
**缺陷**（review35-backend MINOR 2）：B34-01 只修了判据2 的两次独立读取；判据3
`!s.openTimeNow().IsZero() && ... EmptyProbeRuns >= 3` 与判据2 各自调 openTimeNow()——
管理员热改开放时间的亚毫秒窗口内两判据可能基于新旧两个 open：判据2 触发挂起而判据3 解除
（或反之），幽灵窗口挂起/解除不一致（B34-01 同族）。
**主 gate 核实**：读 windowClosedLocked（753-780 行）确证判据2 内两次读取已在 B34-01 收敛
为单快照，判据3 仍独立调用。
**决策**：确认修复——`open := s.openTimeNow()` 取一次复用，判据2/3 共用同一快照（替换原
B19-01/B21-01/32-01 长注释块为精简"open 单快照对三条判据统一"注释，决策历史留在 CLAUDE.md）。
**验证**：go build/vet + scheduler/api 包 race 全绿 + 全量 9 包通过。

### 可疑项裁决（review35-backend 可疑 1 = 发现 3）：CheckClassSelectable 开窗瞬间旧快照误拦
**缺陷形态**（review35-backend 可疑 1）：CheckClassSelectable（1626-1655）用账号专属快照的
`p.InDateRange` 判窗口开放——开窗瞬间（≤snapshotTTL 40s 内快照仍新鲜）旧快照的 InDateRange=false
可能误拦一次手动报名"选课窗口未开放，暂不能报名"。
**主 gate 核实**：触发路径真实存在，但——(a) 快照 40s TTL + 30s 探测节流意味着快照最旧 40s，
开窗瞬间旧快照窗口至多 40s；(b) 前端 2s 轮询 /electives 会拉最新快照，且开窗前用户手动报名
本就罕见（前台有 1500ms 自动刷新 + 倒计时）；(c) 误拦是"一次友好拒绝"，用户重试即放行，绝不
造成数据损坏或黄金期阻塞；(d) 修法（InDateRange 改读实时 open 时间窗）会引入"平台数据 vs 本地
时钟"的判定分叉，反而偏离"以平台快照为准"的 B18-m1 契约。
**决策**：维持观察不修——触发窄（开窗瞬间 40s 旧快照窗口）+ 影响轻微（单次友好拒绝可重试），
修法收益不确定且可能引入契约分叉。观察项 35-04。

### 可疑项裁决（review35-backend 可疑 2 = 发现 4）：tick 内 probe 同步阻塞提交段
**缺陷形态**（review35-backend 可疑 2）：tick() 内 `s.probe()` 是同步调用（最长 FindElectives
15s 超时），提交段在 probe 之后——probe 挂起期间提交被阻塞。
**主 gate 核实**：这是**既有设计**而非新缺陷——tick 300ms 周期，probe 最长 15s 挂起只推迟
提交至多 15s；且 B18-M1 窗口关闭守卫 + B23-03 token 失效短路 + lastProbe 30s 节流 + probing
单飞（F12-B2）意味着正常时序 probe 每 30s 最多一次、挂起窗极小；把 probe 改异步会引入"探测
结果与提交状态不一致"的窗口（探测推进了 WindowOpened 但提交早于状态读取），准确性代价大于
收益。第 14 轮 B14-M1 已实证"提交不依赖探测节流"（TestWindowOpenSubmitsWithoutProbeReset）。
**决策**：维持观察不修——probe 同步是黄金期正确性的保障（探测确证窗口开→同 tick 提交），
挂起被 30s 节流 + 单飞天然有界。观察项 35-05。

### 已核无缺陷清单（review35-backend）
- windowClosedLocked 三条判据（主判据 / B32-01 判据2 带"开放时间已过" / 判据3 EmptyProbeRuns）
  与测试族（TestWindowClosedState/-time.Hour、TestWindowClosedTransitionStateGrace、
  TestGhostWindowEmptyProbesSuspend、TestGhostWindowClockFailuresSuspend、
  TestStateForAccountMirrorsWindowClosed）全部对齐；B34-01/02 的 open 单快照收敛完好。
- tick 提交守卫顺序（open 零值 → 未开未到点 → WindowClosed → lastSubmit 节流）正确；
  submitAll/spawnChain 的 B30-01 链顶 ClientFor 双复核、B18-M2/B20-01/B21-03 落库前复核齐全。
- maybeSyncClock 快路径/30s 失败退避/syncing 单飞/F12-B1 无客户端复位链路正确；
  B21-01 syncFailStreak 真实累计不清零（值域可达 3）。
- probe() 的 B20-03 对齐钟写入、B20-02 EmptyProbeRuns 入账（B21-02 10s 裕量）、B26-03 主判据
  10s 裕量、probeSem cap 4（F17-01）全部就位。
- scheduler.go 全部落库点（AppendLog/SaveSuccess/SaveRefused/DeleteSuccess）B33-01/B34-03 规范
  完好（唯一豁免：DeleteRefused 有意 `_ =`，140-145 行——重设目标全清场景，失败无恢复意义）。
- 手动报名/退选路径（TryAcquireSubmit/MarkDone/RemoveDone/MaybeRelogin）与 B8-M7/B9-01 对称。

## 契约一致性核对（对照 legacy/ 真实源码基线 + CLAUDE.md 逆向契约）

- findElectivesData 请求体 form 编码非 JSON ✓；轮询 `ids=b.join(",")` ✓；b 数组只在
  inDateRange=true 收集 ✓；10s 轮询 + 1500ms 开窗检查 ✓
- btn_type 1=退选/2=报名/其他不渲染、`i.can_select && postReq(...)` 双守卫 ✓（真实 select.js）
- 窗口关闭形态 code:0 空 publishes ✓；countList 只消费 id/selectedCount/auditedCount ✓
  （maxCount 仍未实证，观察项延续）
- 无新发现契约不符。

## 回归

- 后端：`go build ./... && go vet ./...` 全绿；`go test -race -count=1 ./...` 9 包全量通过
  （含 B35-01 + B35-02 修复后）。
- 前端：`npm run build`（tsc -b + vite）通过（含 35-01 + 35-02 修复后）。
- 合并逻辑：merge-check.mts 七场景断言脚本全过（修正断言字段形状后）。

## 观察项（本轮追加/延续）

- 观察 35-01：s-1 echoDone 依赖打断退避（数据不丢，维持现状）。
- 观察 35-02：s-2 在飞操作与重建按钮禁用态差异（行为正确，维持观察）。
- 观察 35-03：m-3 早退残留 attempt（窄时序，维持观察）。
- 观察 35-04：CheckClassSelectable 开窗瞬间旧快照误拦（40s TTL 窄窗 + 单次友好拒绝可重试，
  修法收益不确定，维持观察）。
- 观察 35-05：tick 内 probe 同步阻塞提交段（既有设计，30s 节流+单飞天然有界，维持观察）。
- 历轮观察项全表延续（countList.maxCount 未实证、task_log 无容量上限、死代码族、
  HTTP 探测单飞、probe per-account 错误静默、幽灵课程条目无 UI 提示等）。
