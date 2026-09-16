# 第 21 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler）+ web 全部模块。两个只读 opus 子代理并行产出发现，
> 主 gate 逐条核实（读源码 + 读产品代码推演真实触发路径），确认 4 后端 + 5 前端真实缺陷，
> 全部 TDD 修复后独立中文 commit。

## 后端（4 项）

### B21-01（MAJOR）syncFailStreak 死代码——时钟兜底判据不可达
**缺陷**：`maybeSyncClock` 失败分支在**同一临界区**内先 `syncFailStreak++` 再
`if >=3 { ...; syncFailStreak=0 }` 清零。外部读取方 `WindowClosed()` 持同一把锁，
永远读不到 3——streak 值域恒 `{0,1,2}`，B19-01 的"时钟失败≥3 视同幽灵窗口"判据
实为不可达死代码（对应测试手动注入 3 恒假绿）。开窗后时钟持续失败时幽灵窗口兜底
永不触发，探测永久 2s 烧平台。
**修复**：达到 3 只复位校准偏差 `clockOffset=0`（回退本地时钟），streak 持续累计
至同步成功才清零自愈；瞬断 1 次只记 1 次绝不误触发。`WindowClosed()` 判据首次真实生效。
**TDD**：`TestGhostWindowClockFailuresSuspend` 注释更新对齐"判据真实可达"。
**commit**：`348698d`

### B21-02（MINOR）EmptyProbeRuns 入账误挂黄金期——开窗瞬间平台预清空
**缺陷**：B20-02 入账判据 `now.After(open)` 对"开窗点后平台预清空 publishes"
（F7-01 记录的真实现象：切学期/数据迁移瞬间 publishes 全部消失）会立刻开始计数。
若过渡态持续 ≥6 秒（3 次 × 2s 临门间隔），EmptyProbeRuns 累计到 3 →
`WindowClosed()` 第三条判据在**真实窗口已开**时误触发 → 黄金期提交挂起 + 探测降频，
抢课静默停摆。
**修复**：入账侧加 10s 裕量——`now.After(open+10s)` 才 `EmptyProbeRuns++`，开窗点后
10s 内的空快照探测不计轮数（黄金期 10s 不受影响），黄金期结束仍连续空才确证幽灵窗口；
开窗/非空快照仍即时归零自愈。裕量必须在入账侧（`WindowClosed()` 不读时间只读 count）。
**TDD**：`TestGhostWindowEmptyProbesSuspend` 新增反向断言 3——真实 `probe()` 验证裕量内
（open-5s）不得入账（红灯 got 1 → 绿灯 0）、裕量外（open-12s）正常入账=1。
**commit**：`67ab8af`

### B21-03（MINOR）重登成功分支缺账号已删复核——凭据幽灵复活
**缺陷**：B18-M2（自动链）/B20-01（手动路径）都有"落库前锁内复核 `ClientFor`"防线，
唯独**重登成功分支**缺同款：管理员 DeleteAccount（清凭据表 + Accounts.Remove）与在途
Login（Vision 最坏 2 分钟）竞态，成功返回后**无条件**写回 `tokenValid`/`reloginAt`
内存态 + `UpdateIDToken` 落库把已删账号新 token 写回 credentials 表——重启后 Restore
重建客户端、凭据幽灵复活。
**修复**：成功分支写入前先持锁复核 `s.clients.ClientFor(acct)` 仍存在，已删则整段
（内存写与落库）静默放弃，只清 `relogging` 标记。
**TDD**：`TestDeletedAccountReloginSuccessDropsState`——阻塞重登 → PurgeAccount +
removed → 放行 → 断言 `tokenValid`/`reloginAt` 均无 key 残留。
**commit**：`df613aa`

### B21-04（MINOR）非法 open_time 混改半假成功
**缺陷**：旧实现 `else if FormatOpenTime(...)==nil` 校验失败**静默忽略**该项：混改 PUT
（同时改 activation_enabled + 非法 open_time）返回"配置已更新并生效"但 open_time
实际未变、落库旧值（与 F7-02 空串假成功同族）。若把 `writeJSON+return` 写进
`Runtime.Update` 闭包内，return 只退出闭包不退出 handler——双写响应 + 前置字段
部分应用，"整体拒绝"名存实亡。
**修复**：格式校验前置到 `Runtime.Update` 闭包之外（与非法引擎/越界并发同策略），
非空非法值整体拒绝、绝不落库也不下发。
**TDD**：`TestAdminConfigHotReload` 混改 PUT 断言 code!=0 + 格式报错 + `vision_model`
保持 new-model 未被应用（红灯"activation_enabled 被错误修改"→ 绿灯）。
**commit**：`33de4e8`

## 前端（5 项）

### F21-01（MAJOR）脏标记跨卸载丢失——返回按钮直接卸载吞掉最后一批改动
**缺陷**：返回按钮直接 `flushTargets(); onDone()` 同步卸载。场景 A：publishes 为空时
flushTargets 置脏 return → 最后点选永不落库；场景 B：savingRef 为 true 时置脏 return →
飞行 PUT 完成后 finally 因 unmountedRef 已真跳过补发。两种路径都让用户真实改动静默丢失
（F15/F16/F17 守卫拦下的假清空是安全方向，但用户改动也被误伤吞掉）。
**修复**：返回按钮改异步 `handleBack`——flush 后若在飞 PUT 结束会经 finally 自动补发
（补发链在卸载前自接），循环等 dirty 清空才 `onDone()`；守卫拦下的假清空脏块
（publishes 恒空）刻意放行（等无可等，绝不强行假清空）；api 20s 超时兜底绝不无限挂起。
**commit**：`a5a4901`

### F21-02（MINOR）activeTab 失效值——`??` 只在 null 时回退
**缺陷**：`value={activeTab ?? String(tabs[0].publish_id)}`——发布集合整体重建
（开窗瞬间清空、publish_id 全变）后 activeTab 是 **stale-non-null**，`??` 不回退，
Tabs value 指向不存在的 Trigger，主区悬空（F18-03 只修了"null 悬空"）。
**修复**：activeTab 必须存在于当前 tabs 才生效，否则回退 `tabs[0]`。
**commit**：`a5a4901`

### F21-03（MINOR）退选弹窗补 Esc 关闭
**缺陷**：有 `role=dialog/aria-modal/aria-labelledby` 却无 onKeyDown Escape（F7-03
承诺"Esc 回归键盘可达性"在 Select 退选弹窗从未落地），键盘用户只能 Tab 到按钮。
**修复**：补 Esc——与取消同逻辑，退选中（actionLoading）不响应防误关（F20-03/04
对称闭环）。
**commit**：`a5a4901`

### F21-04（MINOR）激活入口缺幂等守卫
**缺陷**：`activate()` 无 `if (activating) return` 守卫（与 submit() 的 F19-02
`if (loading) return` 不对称）——disabled 依赖渲染落地延迟，连按两次可发两个重复
激活请求（后到响应处理 onLogin 会把会话挤成旧值）。
**修复**：入口先查 activating 在飞标记短路幂等。
**commit**：`844e9de`

### F21-05（MINOR）管理员"学生端"无账号假登出
**缺陷**：`onBackToStudent` 在 `others.length===0` 时仅 `setCurrent("")`，但
account-reselect effect（App.tsx:72-80）立即 `setCurrent(accounts[0])`（仍是 admin）→
渲染条件 `inAdmin || current === adminName` 恒真，Admin 页继续显示——D4 注释声称的
"回登录页"从未生效。
**修复**：无学生账号 = 管理员退出全部会话（清空 sessions + current + targetAccount），
渲染落到 Login 页且 effect 不再弹回；有学生账号则直接切到它。
**commit**：`844e9de`

## 观察项（维持观察，不修）
- 21-01 HTTP 探测单飞（B14-I2/B16-M2/B19-03 延续）：触发窗口已精确化，平台更敏感再落地
- 21-02 Decrypt 死字段（B16-I1 延续）
- 21-03 RemoveFull 死方法（B17-02 延续）
- 21-04 probe per-account 错误静默（频率受 30s 节流约束）
- 21-05 幽灵课程条目无 UI 提示（回显已过滤，安全方向不假清空）
- 21-06 `emptyRunsFor` 辅助方法死代码（WindowClosed 直接读 `state.EmptyProbeRuns`）

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿
- frontend：`npm run build`（tsc -b + vite）绿
