# 第 22 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler）+ web 全部模块。两个只读 opus 子代理并行产出发现，
> 主 gate 逐条核实（读源码 + 读产品代码推演真实触发路径），确认 1 条后端 MAJOR + 1 条
> 后端 MINOR，前端本轮无新增真实缺陷。修复走 TDD 后独立中文 commit。

## 后端（2 项）

### B22-01（MAJOR）目标账号快照无条件回退全局帧——混合年级部署年级串线全开
**缺陷**：`ElectivesSnapshotFor(acct)` 在"该账号专属快照缺失"时**无条件**回退全局快照
`s.lastData`，且只要全局帧新鲜（`probe()` 每 30s 由 `AnyClient()` = 首个注册账号刷新）
就返回 ok=true——`handleElectives` 拿到的数据直接渲染给用户、**根本不会触发该账号的
`ProbeForAccount`**。而 `probe()` 的 per-account 探测只遍历 `AccountsWithTargets()`
（有目标课程的账号）。因此一个已登录且已配置目标、但专属帧尚未建立的账号（如登录后
立即开大厅，probe 周期还没跑到），乃至更常见的"手动报名/浏览"形态，其大厅可能永远显示
全局帧（年级 = 首个注册账号的年级），而不是它自己年级的课程发布。
**触发场景**：混合年级部署（文档实证：高二下发 82 门、高三仅 1 门）。部署者先登录
"高三测试账号"→ `m.order[0]` 恒为高三 → `lastData` 恒为高三帧。高二学生登录、打开选课
大厅：无专属帧 → 回退高三帧（仅 1 门课）→ 永不发 per-account 探测 → 大厅**永久**只显示
高三那 1 门体育课，高二 82 门全部不可见；据此去手动报名/设目标时课程 ID 全错。
**修复**：`ElectivesSnapshotFor` 对**已配置目标**的账号，专属帧缺失/过期即返回
`(nil, false)` 让 `handleElectives` 走 `ProbeForAccount(acct)` 真取该账号年级帧；
**无目标账号**（仅浏览/手动报名）保持回退全局帧（快、无网络开销，且手动复核
`CheckClassSelectable` 本就只读本账号专属帧，不用全局帧兜底——B18-m1 已注明）。
**TDD**：`TestManualSnapshotFallbackOnlyWhenOwnFresh`——断言 1 目标账号专属帧缺失
不得回退全局帧（红：修复前回退返回 ok=true）；断言 2 专属帧写入后返回该账号帧。
**commit**：`e31a288`

### B22-02（MINOR）时钟幽灵窗口测试恒假绿形态——手动注入 3 补不了"判据真实可达"
**缺陷**：B21-01 把 `syncFailStreak` 死代码根治（判据首次真实可达）后，对应测试
`TestGhostWindowClockFailuresSuspend` 仍是**手动进样** `syncFailStreak=3`——手动注入
永不经过 `maybeSyncClock` 失败分支，测试即使全绿也**证明不了**判据真实触发路径。
**修复**：改为真实 `maybeSyncClock` 连续失败 3 次（每轮清 `lastSyncFailAt` 突破 30s
退避 + 轮询等 `syncing` 落地，模拟三次独立失败的时间流逝）；场景 B 同步成功 streak
归零→幽灵窗口解除（自愈）；场景 C 同一形态下 `probeIntervalFor` 必须 30s。
**TDD**：改造后测试单跑绿，`-race` 全量绿。
**commit**：`e31a288`

## 前端（0 项）
本轮前端审查员通读 Select.tsx 目标保存链路 / App.tsx 会话 401 / Login 激活 /
Admin 五 Tab / api client / UI 原语 / useTickingCountdown，并对照后端契约，确认
**无新增真实缺陷**（高风险区域 12 项逐一核过：防抖闭包、假清空守卫链、unmountedRef
生命周期、飞行 PUT 补发链、StrictMode 双挂载、react-query v5 轮询闭包、401 归属、
受控悬空、资源泄漏、弹窗 Esc、幂等入口、类型层）。

## 主 gate 核实结论
- **ElectivesSnapshotFor 是唯一读取侧漏洞**：写入侧 B5-10 已保证 ProbeForAccount 不写
  全局帧、B6-04 保证不写 lastProbe，读取侧回退路径是年级串线最后一块拼图；本轮已堵。
- 内部消费方 `classFullInSnapshot`/`releaseFullIfFreedLocked` 的全局帧回退仅用于
  **满员判定**（课程不存在即返回 false / 保持 full），不构成年级串线，维持现状。
- 观察项（HTTP 探测单飞、Decrypt 死字段、RemoveFull 死方法、probe per-account
  错误静默、emptyRunsFor、syncFailedWindow 写而不读）沿用，无新增。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿