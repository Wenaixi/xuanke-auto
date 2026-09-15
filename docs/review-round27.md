# 第 27 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读 opus 子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径），确认后端 2 项 MINOR + 前端 1 项 MINOR（均核实为真并 TDD 修复）。
> 修复走 TDD（先红灯后绿灯）后独立中文 commit。

## 后端（2 项，确认修复，commit `7eec4aa`）

### B27-01（MINOR）手动报名/退选 `?account=` 透传无凭据表存在性校验
**缺陷**：`handleElectiveSelect`/`handleElectiveExit` 的 `allowAccountOverride` 分支拿到
`?account=` 直接 `acct=q` 放行——B15-M4（目标写）/B26-02（课程读）对任意串已凭据表整体
拒绝，手动操作两路是同一契约的下沉缺口。触发链路：管理员在 Select 页代理某学生时内部
状态残留一个已删/打错的账号名（typo）→ `TryAcquireSubmit` 为该幽灵账号占 inflight 锁 →
`CheckClassSelectable` 无专属帧放行 → `ClientFor` 返回不存在 → 报"账号会话未建立或未
登录"误导文案（无数据破坏：ClientFor 在打平台前拦截，不留孤儿行，但文案误导且与
B15-M4/B26-02 三层防线不对称）。
**修复**：override 分支复用 `accountExists()`（LoadCredentials 逐账号比对，与 B15-M4/
B26-02 判据同源），查无此账号 → 明确"账号不存在，无法执行报名/退选操作"。
**TDD**：`TestAdminElectiveSelectUnknownAccountRejects`。修复前红灯（旧行为返回
"账号会话未建立或未登录"）→ 修复后绿灯 + 反向防线（真实账号 authenticateDirect
落凭据透传报名放行，选择 61115 走 mock 报名成功）。

### B27-02（MINOR）`handleState` 的 `?account=` 透传同样无存在性校验
**缺陷**：B26-02 只修了 `handleElectives` 读路径，`handleState` 透传任意字符串照常放行
——`StateForAccount(ghost)` 对不存在的账号返回空 Courses + `token_valid=true` +
window 状态，与"账号存在但确实无目标"返回形状完全相同。管理员无法分辨"账号不存在"
与"账号没目标"——恰是 B26-02 修掉的"全局帧假装成功"的轻量版（不返回课程数据、危害
小，但契约不对称）。
**修复**：同款 `accountExists()` 比对，查无此账号 → 明确"账号不存在，无法读取状态"；
无 `?account=` 时的核心账号兜底（B20-04）保持不动。
**TDD**：`TestAdminStateUnknownAccountRejects`。修复前红灯（幽灵账号返回 code:0 +
courses null + token_valid true 假象）→ 修复后绿灯 + 真实账号透传放行。

## 前端（1 项，确认修复，commit `59eeb74`）

### R27-01（MINOR）ConfigTab 保存成功后回填陈旧值——二次保存可静默回滚已生效配置
**缺陷**：`ConfigTab.save()` 成功后只 `setConfigEpoch` 自增代际，effect 重跑时读的
`loaded` 仍是**挂载时陈旧快照**（`configQuery` 无 refetchInterval 且全文件无 refetch
调用，react-query 全局 `staleTime:0` 只在组件重挂载/窗口聚焦才可能刷新，同页连续保存
期间缓存 data 从未更新）——表单被覆盖回旧值，管理员二次保存即把刚生效的 open_time/
激活码开关等静默回滚（open_time 回退尤其危险：新一轮抢窗时间被改回旧值，B11-A1 零值
守卫下窗口机制被破坏）。
**修复**：PUT 成功后先 `await configQuery.refetch()` 拉后端"实际生效值"，refetch 成功
才自增代际；refetch 失败则表单保留用户输入不被陈旧值覆盖（本次保存并未回滚），toast
照常确认。
**验证**：`npm run build`（tsc -b）全绿（前端无测试框架，逻辑推演 + 编译门，与 F 系列
历轮一致）。

### S27-01/02/03 可疑项裁决
- **S27-01（401 吊销链 localStorage 不可用降级失效）定非缺陷**：`loadSessions`/
  `saveSessions`（App.tsx:17-31）均有 try/catch 静默降级内存态（N4 注释：隐私模式/
  配额受限时可读不可写），如因网络不清,会话已由内存态继续保持,属有意降级。
- **S27-02（tsconfig.app.json 缺 `strict:true`）**：配置级弱校验弱点，历轮定调维持
  观察，开启会引入大量既有代码修复成本，本轮不启。
- **S27-03（`web/src/index.css` 死文件）已清理**：确认全库无人 import，且
  `global.css` 44-46 行已含同款 `box-sizing` reset、50-62 行 html/body/#root 尺寸
  规则覆盖更完整；删除后构建仍全绿，并入 commit `59eeb74`。

## 主 gate 核实结论
- **B27-01 确认为真实缺陷**：B15-M4/B26-02 修复时只覆盖"目标写 + 课程读"，手动报名/
  退选两路是同契约的下沉缺口；`accountExists` 辅助消除四路重复代码，无新抽象。
- **B27-02 确认为真实缺陷**：B26-02 只修了课程读，`handleState` 状态读是同族缺口；
  token_valid=true 假象会让管理员误判"账号已正常"。
- **R27-01 确认为真实缺陷**：`configQuery.refetch()` 是 F7-02"回填生效值"注释承诺
  但从未真正落地的完整化——旧实现只自增代际、回填源头从未刷新，注释与实现分叉。
- 已核无缺陷：后端 config/db/secure/session/runtime/accounts/zhidao/scheduler/api/
  main 九大模块（review27-backend 报告详列，含时钟/窗口领域之外的 scheduler 各锁纪律、
  Main 恢复顺序契约、FormatOpenTime 空串零值契约复核）+ 前端目标保存链/假清空守卫链/
  401 吊销/Tabs 兜底/幂等入口/倒计时/Admin 五 Tab/api client 全部对齐。
- 可疑待核 2 条转观察：PUT config 全 null 字段空变更先落库（历轮 23-11 延续）、
  VisionAPIKey 掩码形态非空落库（前端明文输入域"留空=不改 key"已避免该形态，无实际
  触发路径）。
- 后端观察项维持：22-01 HTTP 探测单飞、22-02 Decrypt 死字段、22-03 RemoveFull 死方法、
  22-04 probe per-account 错误静默、22-06 emptyRunsFor 死代码、22-07 syncFailedWindow
  写而不读、24 系列（task_log 容量上限等）。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿。
- frontend：`npm run build`（tsc -b + vite build）通过。