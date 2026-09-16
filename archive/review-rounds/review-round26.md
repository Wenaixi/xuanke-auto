# 第 26 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读 opus 子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径），确认后端 3 项 MINOR + 前端 4 项 MINOR（均核实为真并 TDD 修复）。
> 修复走 TDD（先红灯后绿灯）后独立中文 commit。

## 后端（3 项，确认修复）

### B26-01（MINOR）删账号顺序空窗——memory-first 摘除前置
**缺陷**：`handleAdminDeleteAccount` 原顺序 `Store.DeleteAccount`（清 credentials/accounts/
targets/success/refused/activations 六表事务）→ `Sched.PurgeAccount` → `Accounts.Remove`
之间存在毫秒级空窗：在飞提交链（`SelectClass` 最长 15s）恰在空窗完成时做 B18-M2/B20-01/B21-03
的"落库前锁内复核 `ClientFor(acct)` 仍存在"——客户端尚未摘除，复核放行 → `SaveSuccess`
把刚清掉的 success 行写回 / `SaveRefused` 写回 refused 行；重启 `RestoreDone` 读出 success
行 → "已报名成功"假成功；恢复后自动引擎永久跳过退选课。
**修复**：`Accounts.Remove` 前置到最前（memory-first）——客户端先消失，在飞链复核立即失败、
静默放弃落库，空窗从根因消除。`DeleteAccount` 失败时库行未清但注册表已摘（半删态），账号
重启后由 `Restore` 重建客户端自愈，远优于"假删除成功"（客户端仍在注册表继续在飞写回）。
**TDD**：`TestAdminDeleteAccountMemoryFirst` 走真实登录注册（`loginAndGetToken` 落凭据 +
`ensure` 注册客户端），断言删除成功后 `ClientFor` 必须返回不存在。初始实现（memory-first）
直接绿 → **还原旧顺序**临时跑 → 先绿（ClientFor 的 (nil,true) 导致断言误判）→ 修复断言为
`ok&&c!=nil` 判定后红灯（库行已清但客户端仍在注册表）→ 恢复 memory-first 绿灯。
**commit**：`fce76fa`

### B26-02（MINOR）管理员 ?account= 读路径未知账号静默回退全局帧
**缺陷**：`handleElectives` 管理员穿透 `?account=<任意串>` 无条件 `acct=q` 后走
`ElectivesSnapshotFor`——`B22-01` 只对**有目标账号**缺失专属帧返回 `(nil,false)`；**无目标
账号**仍回退全局 `lastData` 帧且返回 ok=true。typo / 已删账号残留 URL 参数被静默当成
"该账号的课程"返回全局数据，管理员无法区分；而写路径 `handleSetTargets`（B15-M4）对未知
账号凭据表校验、整体拒绝——读路径假装成功不对称。
**修复**：透传分支复用 B15-M4 同款凭据表校验（`LoadCredentials` 逐账号比对），查无此账号
→ 明确"账号不存在，无法查看课程"，绝不用全局帧假装成功。
**TDD**：`TestAdminElectivesUnknownAccountRejects`。**关键铺垫**：先 `ProbeNow()` 填充全局帧
（否则未知账号恰好因 `ProbeForAccount` 报错而掩盖缺陷，测试假绿）。旧行为红灯（全局帧回退
返回 code:0 课程数据）→ 修复后绿灯 + 真实账号穿透放行 + 学生会话自读不受影响。
**commit**：`fce76fa`

### B26-03（MINOR）WindowClosed 主判据缺过渡态 10s 裕量
**缺陷**：`probe()` 的 `WindowClosed` 主判据（B18-M1）用 `now.After(openTimeNow())`，而
`EmptyProbeRuns` 入账（B21-02）已用 `now.After(open+10s)`——两判据裕量不对称。开窗确证
（探测非空 + InDateRange）后平台短暂返回空快照（数据刷新/切学期过渡态，F7-01 记录的预清空
现象），旧判据下一拍探测（临门 2s）即置 `WindowClosed=true` → tick 提交守卫挂起提交 + 探测
降回 30s；若平台在 30s 内恢复，黄金期提交已停摆（与 B21-02 修复前同族危害）。
**修复**：主判据改为 `now.After(openNow+10s)`，与 B21-02 裕量对称；真关仅推迟 10s 判定
（超裕量仍按原判据关闭，`-time.Hour` 测试不受影响）。
**TDD**：`TestWindowClosedTransitionStateGrace`（open = now−5s、prevOpened=true、空快照 →
WindowClosed 必须 false）。修复前红灯（true）→ 修复后绿灯；全窗口关闭/幽灵窗口测试族
（TestWindowClosedState / GhostWindowEmptyProbesSuspend / GhostWindowClockFailuresSuspend /
WindowClosedReleasesNoFull / WindowClosedProbeDropsToFar / WindowClosedSelectStopsBombing）
回归全绿。
**commit**：`fce76fa`

## 前端（4 项，确认修复，commit `beb6aef`）

### F26-01（MAJOR）返回按钮漏覆盖退避重试→最后一批改动静默丢失
`handleBack` 只等 `savingRef.current` 静止即卸载；退避 timer 在飞时 `cleanup clearTimeout`
取消排队重试 → 用户改动静默丢失。修复：`pendingSaving() = savingRef.current || retryState.
current.timer !== null` 循环等到 pending 清空才 onDone。

### F26-02（MINOR）连 5 次保存失败后重试沉默
`scheduleRetry` 5 次后停止（已知设计）但 `handleBack` 误判静止直接卸载。融于同一
`pendingSaving` 判据 + 21s deadline 超时兜底。

### F26-03（MINOR）手动报名/退选缺在飞幂等
双击并发后端 `TryAcquireSubmit` 拒绝第二发但 `finally` 仍 `invalidateQueries` → 假失败
toast。入口补 `actionLoading === c.id` 短路与 F19-02 同款。

### F26-04（MINOR）保存成功后 apiKey 清空早于确认
保存中用户已开始输入的新密钥被静默吞掉。`setApiKey("")` 挪至 `configEpoch` 自增 + toast
之后，收窄清空窗口。

## 主 gate 核实结论
- **B26-01 确认为真实缺陷**：顺序空窗是 B18-M2/B20-01/B21-03 复核防线的**共同盲区**——
  复核只挡"账号已删"，没能挡"账号正在被删的毫秒窗口"。memory-first 从根因消除；偏差风险
  （DeleteAccount 失败半删态）由重启 Restore 自愈，可接受。
- **B26-02 确认为真实缺陷**：B15-M4 修复时只盯写路径，读路径 `handleElectives` 同款透传
  未校验——写路径拒绝、读路径假装成功的不对称在同一轮未闭合。凭据表校验复用避免新抽象。
- **B26-03 确认为真实缺陷**：B21-02 裕量只加在 EmptyProbeRuns 入账侧，主判据沿用裸
  `now.After(open)`——两判据裕量不对称违反"开窗过渡态防误挂"的统一契约。主判据 +10s 对称
  补齐，真关判定延迟 10s 可接受。
- **F26-01/02/03/04 均确认为真实缺陷**（前端通道回执已逐项核过源码，见 beb6aef）。
- 已核无缺陷：B26-02 触发前提（全局帧填充）已封进测试防假绿；`ElectivesSnapshotFor` 无目标
  账号回退全局帧契约保持（browse-only 合法路径）；B26-01 memory-first 对现有删除测试（
  TestAdminDeleteProtectsRenamedAdmin / TestAdminDeleteCodeNoBodyOK /
  TestAdminDeleteRejectsUnnormalizedAccount / TestAdminStatsAccountsLogs）无回归。
- 后端观察项维持观察：22-01 HTTP 探测单飞（B14-I2 延续）、22-02 Decrypt 死字段、
  22-03 RemoveFull 死方法、22-04 probe per-account 错误静默、22-06 emptyRunsFor 死代码、
  22-07 syncFailedWindow 写而不读、24-01 task_log 容量上限 / 快照 TTL 双钟偏差 / PUT config
  空变更。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿（api 11.8s 非 race
  亦绿）。
- frontend：`npm run build`（tsc -b + vite build）通过。