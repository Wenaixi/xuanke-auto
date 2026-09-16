# 第 20 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道，均附"宁缺毋滥 + 文件行号 + 触发场景 + 已知观察项去重"模板），发现全部经主通道逐一读码推演定案；修复独立 commit，全部 TDD。
> 本轮结论：**后端 4 项（B20-01 删除账号与在飞手动报名/退选竞态写回库行 / B20-02 幽灵窗口 EmptyProbeRuns 防轰炸闭环缺口 / B20-03 探测时间戳时间基准混用 / B20-04 管理员不带 account 设置目标落入孤儿行）**；**前端 1 项 MAJOR + 3 项即刻修（F20-01 StrictMode 重挂载后目标自动保存整链静默失效 / F20-02 复制兜底 textarea readOnly 加固 / F20-03 激活码弹窗补 Esc 关闭 / F20-04 删除确认弹窗补 Esc 关闭）**。

---

## 一、后端发现与修复状态（第 20 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B20-01** | MAJOR | **删除账号与在飞手动报名/退选竞态写回库行**：B18-M2 只修了自动链 spawnChain，手动路径同款竞态整链开放——管理员 `DeleteAccount`（清凭据/库行 + `Accounts.Remove`）与在飞手动 `SelectClass`/`ExitClass`（HTTP 最长 15s）竞态，删除完成后请求才返回成功，handler 直接 `MarkDone`/`RemoveDone` 无复核 → `SaveSuccess` 把已删账号 success 行写回（重启 `RestoreDone` 假成功）/ `SaveRefused` 写回 refused 行（重启 `RestoreRefused` 令自动引擎永久跳过该课）。**修复**：`MarkDone`/`RemoveDone` 持锁段复核 `s.clients.ClientFor(acct)` 仍存在，已删静默放弃落库（与 B18-M2 自动链同款防线，手动报名/退选两路对称补齐）。TDD：`TestDeletedAccountManualInFlightDropsState` 红灯（实际 2 行）→ 绿灯（success 1 行 + refused 0 行） |
| **B20-02** | MINOR | **幽灵窗口防轰炸闭环缺口**：B19-01 兜底只覆盖时钟失败路径（`syncFailStreak>=3`），**从未开过窗 + 空快照 + 时钟接口正常 + 开放时间已过**时 `WindowClosed()` 恒 false——tick 提交段 1s 周期 `SelectClass` 打平台（返回 code=1"无效的课程ID"，`isWindowClosedError` 不命中 → 实时复核 StudentCounts 空 → 下轮重打）+ `probeIntervalFor` 恒落临门 2s 高频 findElectivesData，防轰炸契约闭环缺口。**修复**：新增 `EmptyProbeRuns` 探测量变（空快照 + 未开窗 + 开放时间已过每轮 +1，否则归零，json 省略），`WindowClosed()` 加第三条判据（从未开窗 && EmptyProbeRuns>=3 视同关闭），提交挂起 + 探测降回 30s；开窗前正常空快照首探不计数、开窗/非空快照即时归零自愈。TDD：`TestGhostWindowEmptyProbesSuspend` 先红（3 轮不触发）→ 绿灯 + 反向断言（已开窗绝不挂起/归零解除） |
| **B20-03** | MINOR | **探测时间戳时间基准混用**：`probe()`/`ProbeForAccount`/`ProbeNow`（含失败路径）写入 `lastProbe`/`lastDataAt`/`acctDataAt` 用本地 `time.Now()`，而 tick 判读 `now.Sub(lastProbe)`/快照过期 `time.Since(acctDataAt)`/提交节流全走对齐钟——两套时间基（实测偏差 ~640ms）边界同一语义但契约不自洽（与 B16-M1 已消灭的"写入本地/读对齐"混用同族）。**修复**：三处探测时间戳写入侧统一改 `s.nowAligned()`/`nowAlignedLocked`，判读判据零改动。全量 `go test -race` + `go vet` 绿 |
| **B20-04** | MINOR | **管理员不带 account 设置目标落入孤儿行**：`handleSetTargets` 对 `allowAccountOverride` 为真且未传 `?account=` 时 acct 停在 `sessionAccount(r)` = 管理员名，校验放行后 `SetTargetsForAccount(admin, ts)` 写进 store.targets 无主行（重启 `LoadTargetsForAccount` 幽灵复活）+ 污染 `AccountsWithTargets` 首账号选择（B10-04 排序后 admin 可能成核心账号）。**修复**：与 `handleElectives`/`handleState` 对齐——无透传时取 `targetAccts[0]` 兜底，无任何有目标账号则整体拒绝（管理员自己不是学生，目标只该属于学生账号）。TDD：`TestAdminSetTargetsWithoutAccountRejects` 红灯（返回成功 + 孤儿行）→ 绿灯（拒绝 + store 无 admin 行）；`TestStudentSetTargetsWithoutAccountOK` 反向防线（普通学生会话不受影响） |

## 二、前端发现与修复状态（第 20 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F20-01** | MAJOR | **StrictMode 重挂载后目标自动保存整链静默失效**：`unmountedRef` 只有 cleanup-effect 置 true（`useEffect(() => () => { unmountedRef.current = true ... }, [])`），**没有任何 remount 复位**——开发态 `<StrictMode>`（web/src/main.tsx）对组件执行 mount→unmount→remount 两遍，二次挂载后 `unmountedRef.current` 恒真，`saveNow()`（行 254 `if (unmountedRef.current) return`）/`scheduleRetry`/补发全部短路，**用户改选目标永不 PUT 且无任何报错**（开发调式期间改动静默丢失，只能靠刷新 state 回显发现）。**修复**：挂载 effect 复位 `unmountedRef.current = false` + cleanup 置 true 双向，重挂载后保存链路恢复；真正卸载仍停手（F13-C2 卸载后防孤儿重试契约不变） |
| **F20-02** | MINOR | **复制兜底 textarea 缺 readOnly 加固**：F19-03 的 execCommand 兜底手动构造 textarea 未设 readOnly——部分浏览器从可编辑区弹出软键盘或选择行为异常导致 `execCommand("copy")` 返回 false，复制兜底不稳定。**修复**：`ta.readOnly = true` 固化，防可编辑区干扰选中/复制 |
| **F20-03** | MINOR | **激活码弹窗补 Esc 关闭（键盘可达性闭环）**：F7-03 注释承诺"Esc 关闭回归键盘可达性"，但实现从未落地——裸 div 无 keydown 处理，键盘用户只能 Tab 到"取消"按钮。**修复**：补 `onKeyDown` Esc 处理，与取消按钮同逻辑（清待激活账号/票据/激活错误），激活中（activating）不响应防误关 |
| **F20-04** | MINOR | **管理员删除确认弹窗补 Esc 关闭**：与 F20-03 对称——`pendingDelete` 确认 Dialog 同样无 Esc 关闭，键盘可达性闭环缺失。**修复**：补 `onKeyDown` Esc → `setPendingDelete(null)`，删除中（deleting）不响应防误关 |

## 三、修复细节（本轮后端 4 个 commit + 前端 2 个 commit，独立 commit）

- **B20-01**（commit cfac667）：手动报名/退选与删账号竞态——MarkDone/RemoveDone 持锁复核 ClientFor
- **B20-02**（commit 77ba699）：幽灵窗口 EmptyProbeRuns 防轰炸闭环缺口（WindowClosed 第三条判据）
- **B20-03**（commit 9f2d1ea）：探测时间戳写入侧统一对齐钟（nowAligned()/nowAlignedLocked）
- **B20-04**（commit 1ea95d0）：管理员不带 account 设置目标孤儿行——targetAccts[0] 兜底 + 无目标账号整体拒绝
- **F20-01**（commit 826d069）：StrictMode 重挂载 unmountedRef 复位（挂载 effect 双向）
- **F20-02/F20-03/F20-04**（commit 9a2aa1f）：复制兜底 readOnly 加固 + 激活码/删除确认弹窗 Esc 关闭

## 四、回归证据（提交时点）

- `cd web && npm run build`（`tsc -b` + `vite build`）— 通过（408.66 kB / gzip 122.46 kB），F20-01 修复后 tsc -b 零错
- `cd backend && go build ./... && go vet ./... && go test -race ./...` — 全绿（api / config / db / runtime / scheduler / secure / session / store / zhidao）
- B20-01 TDD：`TestDeletedAccountManualInFlightDropsState` 红灯（实际 2 行写回）→ 绿灯（success 1 行 + refused 0 行）
- B20-02 TDD：`TestGhostWindowEmptyProbesSuspend` 红灯（3 轮不触发）→ 绿灯 + 反向断言
- B20-04 TDD：`TestAdminSetTargetsWithoutAccountRejects` 红灯（成功 + 孤儿行）→ 绿灯（拒绝 + 无 admin 行）；`TestStudentSetTargetsWithoutAccountOK` 反向防线同步绿

---

## 提交索引（本轮后端 4 个 commit + 前端 2 个 commit）

```
cfac667 fix(backend): 第20轮B20-01 MAJOR 删除账号与在飞手动报名/退选竞态写回库行
77ba699 fix(backend): 第20轮B20-02 MINOR 幽灵窗口防轰炸闭环缺口——EmptyProbeRuns 第三条判据
9f2d1ea fix(backend): 第20轮B20-03 MINOR 探测时间戳时间基准混用——写入侧统一对齐钟
1ea95d0 fix(backend): 第20轮B20-04 MINOR 管理员不带account设置目标落入孤儿行
826d069 fix(web): 第20轮F20-01 MAJOR StrictMode重挂载后目标自动保存整链静默失效
9a2aa1f fix(web): 第20轮F20-02/F20-03/F20-04 复制兜底readOnly加固 + 激活码/删除确认弹窗Esc关闭
```

## 第 20 轮观察项（未修复，留档）

- **HTTP 探测单飞**（B14-I2/B16-M2/B19 延续）：`ProbeForAccount`/`ProbeNow` 未接入 probing/lastProbe 节流——触发条件已精确化（30s 周期至多多 1 次），维持观察
- **Decrypt 死字段**（B16-I1 延续）：加密工具中未使用的解密函数字段，删除待定
- **RemoveFull 死方法**：手动退选解封后已无调用方，删除待定
- **probe per-account 错误静默**：`ProbeForAccount` 返回值被丢弃（`_, _ =`），频率受 30s 节流约束，维持观察
- **幽灵课程条目无 UI 提示**（F20-02 agent 自述"可留待后续"）：/state courses 与当前 publishes 不匹配时用户看不到任何提示，属契约内行为（回显已过滤，安全方向不假清空），维持观察

## 下轮待办

- 第 21 轮：发射两枚 opus 只读 agent（后端 + 前端），沿用同一模板（读全部模块、宁缺毋滥、已知观察项去重）