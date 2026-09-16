# 第 25 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读 opus 子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径），确认后端 1 项 MAJOR + 前端 1 项 MINOR（均核实为真并 TDD 修复）。
> 修复走 TDD（先红灯后绿灯）后独立中文 commit。

## 后端（1 项，确认修复）

### B25-01（MAJOR）open_time 清空后重启拒绝启动（F7-02 持久化契约与启动校验矛盾）
**缺陷**：管理员 `PUT /api/admin/config` 提交 `{"open_time":""}` 是 F7-02 明确支持、
文档承诺"内存/落库/回显三处对齐"的合法操作——`handleAdminConfig`（handler.go:713-718）
对空串跳过格式校验，`Runtime.Update` 落 `OpenTime=""`（746-756），`SaveSettings` 把
`open_time=""` 写入 settings 表（774-782）。但服务重启时 main.go:109-112 从 settings
**无条件**恢复 `c.OpenTime = v`（空串无空值过滤），随后 main.go:116
`scheduler.FormatOpenTime("")` → `time.ParseInLocation("2006-01-02 15:04:05", "")` 报错
（scheduler.go:1545-1551 无空串特判）→ `log.Fatal` 拒绝启动，服务**永久停摆**，只能
手工改 DB 删 settings 行才能恢复。
**触发路径**：
1. 管理员在运行配置面板清空"开放时间"保存（合法解除窗口机制，重启后仍应保持清空）；
2. 远端部署一次例行重启 → main.go 启动路径把合法配置当致命错误，全服务失联；
3. 唯一恢复手段是手工进 SQLite 删 settings 行或删库重建（远端不可达即灾难）。
**契约冲突**：F7-02/B11-A1/B15-M2 全部把"open_time 零值 = 合法解除窗口机制"作为一等
语义（runtime.reparse 置 OpenTimeParsed 零值、tick 零值守卫挂起提交、probeIntervalFor
零值降回 30s），唯独启动路径 FormatOpenTime 硬校验拒绝空串——清空动作落库持久化了，
重启却拒不认账。
**修复**：`FormatOpenTime` 空串返回零值 `time.Time` 不判错，与 `runtime.reparse` 对空串
置 `OpenTimeParsed` 零值的语义完全对齐；非空值仍严格校验格式（B21-04 契约不变）。
**TDD**：`TestFormatOpenTimeEmpty` 修复前红（"开放时间格式错误"）→ 修复后绿（空串
返回零值）。**commit**：`e8d86d5`

## 前端（1 项，确认修复）

### F25-01（MINOR）管理员自身会话被 401 吊销后 targetAccount 残留——重登卡死代理页
**缺陷**：`onUnauthorized`（App.tsx:103）只清"被吊销账号恰是当前代理目标"
（`prev === lostAccount`）；当**管理员自身会话**被 401 吊销（`lostAccount === adminName`）
且正代理查看学生账号时，`sessions[admin]` 被删 → account-reselect effect 把 `current`
切到剩余学生账号 → 渲染 `targetAccount ? <Select>` 直接命中代理 Select 分支；管理员
**重新登录**后 `current === adminName` 且 `targetAccount` 仍非空 → 页面跳过 Admin 直接
掉进代理学生 Select（F15-07 只覆盖主动 logout 清 targetAccount，被动吊销不对称）。
**修复**：`lostAccount === adminName` 时同步 `setTargetAccount(null)`（代理凭据随管理员
会话一起没了，残留代理态毫无意义）。**commit**：`f7df37c`

## 主 gate 核实结论
- **B25-01 确认为真实缺陷**（全量 grep 确认 TestFormatOpenTime 仅常规值、settings 恢复
  路径无空值过滤，历轮未覆盖；修复方向与 runtime.reparse 契约对齐）。走 TDD 修复。
- **F25-01 确认为真实缺陷**（读 App.tsx:89-123 逐行确认触发路径：lostAccount 反查逻辑、
  第 103 行清除条件、第 105 行 admin 保护分支、渲染分支 targetAccount 优先级）。
- 后端通道观察项 7 项均维持观察不修：reloginBackoff backoffMax=10min 死分支（防御性
  代码）、RestoreRefused 注释过时 + refused_test 用 SetTargetsForAccount 模拟 main 恢复
  顺序（真契约由 TestRefusedRestartOrderRealDB 固化）、lastSyncFailAt 本地钟 vs 判定对齐
  钟（偏差 ~640ms/30s 无实质影响）、settings 恢复 captcha_concurrency 无上限校验（外部
  篡改才越界）、probe per-account 排队堆积窄窗（量级有限）、PUT config 空变更先落库
  （23-11 延续）、历轮定案观察项复核仍在。
- 已核无缺陷：并发锁序（reloginMu→s.mu、chainMu、probeSem、probing 单飞）、B18/B20/
  B21/B23/B24 的 ClientFor 复核链、窗口关闭三判据、激活码原子扣减、票据单次防重放、
  handleElectives 管理员穿透全局帧（契约内设计）、ReloginIfNeeded 内部账密自洽、
  spawnChain 链顶 tokenValidForLocked 锁序。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿。
- frontend：`npm run build`（tsc -b + vite build）通过。
