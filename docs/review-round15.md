# 第 15 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道），发现全部经主通道逐一读码推演 + 实证测试定案；修复按"测试契约修正/补断言/最小生产改动"独立 commit。
> 本轮结论：**后端 3 项修复（B15-M2 open_time 零值探测降频 / B15-M4 管理员透传目标孤儿行 / B15-M5 管理员删除空格账号假成功）；前端 4 项修复（F15-01 flushTargets 假清空 / F15-02 卸载失败 toast / F15-03 横幅状态机 / F15-07 登出清代理）**；无 CRITICAL；B15-M1/M3 与 F15-04/05 等观察项记录见下。

---

## 一、后端发现与修复状态（第 15 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B15-M2** | MAJOR | **open_time 为零值（全新部署未配置 / 管理员 PUT open_time="" 显式解除窗口机制，runtime.reparse 置 OpenTimeParsed 零值）时，探测永久 2s 高频轰炸 findElectivesData**：`probeIntervalFor` 里 `now.After(open.Add(-nearWindow))` 对零值 open 恒 true 落入临门 2s 分支，且快照非空（开窗前平台有课程但 in_dateRange 全 false）时 WindowClosed 恒 false → 探测 2s 高频永续，正是第 6 轮根因修复的"访问过于频繁"1 分钟熔断触发形态。提交已被 B11-A1 零值守卫挂起，但探测仍在 2s 高频，行为割裂。**修复**：`probeIntervalFor` 入口 `IsZero() → return probeIntervalFar`（30s 常态）。**TDD**：`TestProbeIntervalZeroOpenTime`（scheduler_test.go）红灯 `got 2s` → 绿灯，反向对照"恢复未来开窗时间→临门期仍 2s 盯守"不误伤新一轮 |
| **B15-M4** | MAJOR | **管理员透传目标到不存在账号 → store.targets 孤儿行 + 幽灵目标复活**：`handleSetTargets` 对 `?account=` 任意字符串无条件 `SetTargetsForAccount`。未知账号不在 accounts 表、也不在 Accounts 客户端注册表，目标被写进无主行；重启恢复 `LoadTargetsForAccount` 读回 → 目标幽灵复活；调度器按 targets 遍历时该账号 `ClientFor` 返回不存在 → 目标永不执行、静默失败。**修复**：透传分支先以 `LoadCredentials`（凭据表，所有登录路径都写）校验账号真实存在，不存在整体拒绝"账号不存在，无法设置目标"。**TDD**：`TestSetTargetsUnknownAccountDoesNotFabricate`（handler_test.go）红灯 `code:0 目标已保存` → 绿灯拒绝。判据历经 `ListAccounts` 误伤 `authenticateDirect` 已登录账号（accounts 表仅完整登录 issueSession 写）→ 改 `LoadCredentials`；既有 `TestAccountOverrideRequiresAdminSession`/`TestSetTargetsAndState`/`TestSetTargetsEmptyAllowed` 回归全绿 |
| **B15-M5** | MAJOR | **管理员删除带尾随空格账号 = 假删除成功**：`handleAdminDeleteAccount` 只 `TrimSpace != ""` 判空，未把 trim 后的账号回写——前端一次空格失手（" 12345 " 粘贴带换行）后，trim 出的 "12345" 被当作删除目标，真实账号被删、响应却显示"已删除账号 12345"。前端仍挂 " 12345 " 标签，store 中 12345 已消失，调度器照旧尝试提交空客户端。**修复**：`acct := TrimSpace` 后校验 `acct != req.Account`（trim 后必须与原值逐字节一致），含首尾空白整体拒绝"账号无效或不可删除"。**TDD**：`TestAdminDeleteRejectsUnnormalizedAccount`（handler_test.go）红灯 `trim 误删真账号` → 绿灯（拒绝 + 凭据幸存）。断言通道历经修正：403（缺 JSON Content-Type）→ 管理员会话+Content-Type → 判据从 `Registered()` 改 `LoadCredentials`（前置 store 真实落凭据） |
| B15-M1 | 观察 | **inflight 所有权竞态（agent 自降 MINOR）**：`TryAcquireSubmit` 抢飞锁由调用方线程持有、`spawnChain` 提交链在 goroutine 内持有——极端重入下 release 顺序可能交错。核实：调用方在手动/自动之间互斥串行化，release 为一次性、无上下文复用，锁的"所有权"语义在当前调用下不可破坏。观察不修，注释记录 |
| B15-M3 | 观察 | **观察时间基准混用**：提交/探测用对齐钟（nowAligned），退避/闸门计时用本地钟（time.Now）——两套时间基相差实测 ~640ms，失败退避/成功闸门边界偏移 ≤1s 可忽略。观察不修 |
| B15-I1 | 观察 | 配置类边界（open_time 超范围/非法格式在 format 阶段已拒绝，admin/config 回显已脱敏）。观察不修 |
| B15-I2 | 观察 | 持锁日志（AppendLog 在锁内调用）。核实：SQLite 写为本地文件操作毫秒级，不跨网络段；锁内写不放大竞态窗口。观察不修 |
| B15-I3 | 观察 | `TokenValidFor`/`MaybeRelogin` 读 `reloginFail` 等重登态未持 reloginMu（只读快照，锁纪律不破坏）。观察不修 |
| B15-I4 | 观察 | `TestLocalDdddOcrAvailable`（本地 ddddocr 可用性探测测试）依赖本机安装，CI 无 Python 时跳过。观察不修 |

**第 14 轮遗留专项回归**：B14-M1/M2 契约修正、B14-N1 日志硬编码、B14-I1 删段——均收敛无新问题。

## 二、前端发现与修复状态（第 15 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F15-01** | MAJOR | **flushTargets 与防抖保存的"发布缺席+已有选中=假清空"覆盖**：`rev>0`（用户改过）但 `publishes` 已空（窗口开启瞬间平台短暂清空 / 关闭后恒空）时，targets 由 `[publishes × selected]` 联查构建 → `targets=[]` 是一个"假清空"，PUT 会把已落库目标整包永久抹除（与 F7-01"仅 rev 驱动"契约同源但内容侧失守）。**修复**：`flushTargets` 与防抖 `setTimeout` 两条腿都加 `publishesMissing = publishesRef.current.length===0 && selectedCount>0` 守卫——数据缺席绝非用户意图，置脏保留不覆盖；只有 selected 全空（用户明确清空全部）才合法 PUT `[]`。只修一条腿仍会漏 |
| **F15-02** | MINOR | **saveNow 卸载后失败也 toast 轰炸**：catch 分支 `toast(...)` 在 `unmountedRef` 检查之前——组件已卸载时仍弹"目标保存失败"孤儿 toast。**修复**：`unmountedRef.current` 检查移到 toast 之前（与 F13-C2"卸载后不轰炸"意图对齐） |
| **F15-03** | MINOR | **窗口横幅状态机自相矛盾**：原条件 `stateData.window_opened || cd.isExpired` 让窗口关闭后（open_time 为过去时刻 → `cd.isExpired` 恒 true）横幅永远显示"已开放"，与同屏 F14-03 空态卡矛盾。isExpired 只是"本地倒计时走到 0"，不说明窗口开放或已关闭。**修复**：状态机改为 `window_closed` 优先显"已关闭" → `window_opened` 显"已开放" → `isExpired` 显"本地已到开窗点，等待平台窗口开放..." → 默认倒计时 |
| **F15-07** | MINOR | **登出未清除代理目标账号**：管理员代理查看学生大厅退出后，重登同一管理员不重置 `targetAccount` → 渲染直接命中代理分支再次掉进学生 Select，跳过管理页。**修复**：`logout()` 里 `setTargetAccount(null)`——登出即放弃代理态 |
| F15-04 | 观察 | **focus 陷阱（F6-02 延续）**：激活弹窗关闭后焦点不回到触发元素。核实：Radix Dialog 内嵌 Modal 未接 Dialog 迁移，焦点仍留在关闭的弹窗内。观察不修（迁移 Radix Dialog 时一并落地） |
| F15-05 | 观察 | **Toast viewport**：Radix Toast 未指定 viewport，默认渲染位置可能与设计不符。核实：ToastProvider 用默认 viewport，视觉上位置固定可接受。观察不修 |

## 三、修复细节（本轮 3 后端 + 4 前端生产改动，独立 commit）

- **B15-M2**（commit b592039 后端）：scheduler.go `probeIntervalFor` 入口 IsZero 返 far + `TestProbeIntervalZeroOpenTime`
- **F15 系列**（commit c3decee 前端）：Select.tsx F15-01/02/03 + App.tsx F15-07
- **B15-M5**（commit ac8eb62 后端）：handler.go `handleAdminDeleteAccount` trim 后逐字节一致校验 + `TestAdminDeleteRejectsUnnormalizedAccount`
- **B15-M4**（commit de6f68f 后端）：handler.go `handleSetTargets` 透传前 LoadCredentials 存在性校验 + `TestSetTargetsUnknownAccountDoesNotFabricate`

## 四、回归证据（提交时点 + 收尾复跑）

- `cd backend && go build ./... && go vet ./...` — 通过
- `cd backend && go test -race ./...` — 全包绿（api 10.6s / scheduler 16.9s / store 6.9s；config/db/runtime/secure/session/zhidao cached）
- 专项：TestProbeIntervalZeroOpenTime（0.00s PASS）/ TestProbeIntervalWindowClosed / TestSubmitSuspendedWhenOpenTimeCleared / TestAdminDeleteRejectsUnnormalizedAccount / TestAdminDeleteProtectsRenamedAdmin / TestSetTargetsUnknownAccountDoesNotFabricate / TestAccountOverrideRequiresAdminSession / TestSetTargetsAndState / TestSetTargetsEmptyAllowed 全 PASS
- `cd web && npx tsc --noEmit` — 通过；`npx vite build` — 通过

---

## 提交索引（本轮 4 个 commit）

```
b592039 fix(backend): 第15轮B15-M2 open_time零值探测降频（probeIntervalFor入口IsZero返30s远间隔）
c3decee fix(web): 第15轮F15系列四项（F15-01假清空守卫/F15-02卸载toast/F15-03横幅状态机/F15-07登出清代理）
ac8eb62 fix(backend): 第15轮B15-M5管理员删除空格账号假成功根治（trim后逐字节一致校验）
de6f68f fix(backend): 第15轮B15-M4管理员透传目标孤儿行根治（LoadCredentials存在性校验）
(review-round15.md + CLAUDE.md 沉淀)
```

## 下轮待办

- HTTP 侧探测接入全校单飞/节流（B14-I2 定案维持观察，触发条件已精确化——最坏每 30s 周期至多多 1 次直打；若未来平台熔断更敏感再落地）
- F14-01 防抖保存内容侧守卫（selected 非空 + publishes 空则跳过——微窗口数据丢失，记录不修）
- F14-02 返回链路最后快照截断（极低概率，记录不修）
- 激活弹窗焦点陷阱（F6-02 长期遗留，Radix Dialog 迁移）
- Dashboard 消费 window_closed 显示"已关闭"（F13-B3 观察延续）
