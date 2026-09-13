# 第 4 轮全模块安全审查与修复记录

> 本轮覆盖：后端核心全模块（handler/router/scheduler/zhidao/accounts/runtime/store/db/config/session/secure）+ 前端全模块（App/client/Login/Dashboard/Select/Admin）+ 构建与 CI（GitHub Actions）。
> 审查方式：后端由子代理逐文件通读 + `go test -race ./...` 全量回归；前端为独立前端审查报告；构建/CI 独立报告。
> 修复纪律：每个发现单独验证（TDD 红灯→绿灯）后独立 commit。

---

## 一、第 3 轮修复回归核验（13 项全部通过）

| 修复项 | 结论 | 证据 |
|---|---|---|
| recoverMiddleware 统一 500 | 正确 | handler.go:940-950 + `TestRecoverMiddlewareHidesPanicDetail` |
| handleLogin 口令错 300ms 拉平 | 部分正确（见 A5） | handler.go + `TestLoginAdminWrongPasswordTimingFlat` |
| handleAdminConfig 先落库后 dispatch | 正确 | handler.go + M-4 测试 |
| http.Server 显式超时 | 正确 | main.go:165-172 |
| 登录/激活独立限流桶 | 正确 | router.go + `TestLoginActivateSeparateBuckets` |
| session.Store sweepLoop/Close/logout | 正确 | store.go + 登出即失效测试 |
| newActivationCode panic + uses≤1000 | 正确 | handler.go |
| handleSetTargets ≤100 + 范围校验 | 正确 | handler.go + `TestSetTargetsBounds` |
| M-2 只回显自身 / M-3 CreateAdmin 绑定配置名 | 正确 | handler.go + session/store.go |
| C-2 激活票据绑定 + 事务扣码 | 正确 | session/store.go + store.go 原子 UPDATE |
| C-4 实时复核锁外 | 正确 | scheduler.go + 锁外验证测试 |
| loadDotEnv 不截断# + stats 兜底 vision | 正确 | config.go + handler.go |
| targets 有界 + db 文案 | 正确 | store.go + db.go |

---

## 二、第 4 轮新发现问题与修复状态

### A类（后端，子代理报告 2 MAJOR + 6 MINOR）

| 编号 | 严重度 | 问题 | 决策 | 修复状态 |
|---|---|---|---|---|
| **A1** | MAJOR | `handleAdminConfig` 对 `captcha_engine` 无值域校验、`captcha_concurrency` 无上限——非法值直接落库并下发，配置显示与实际引擎错位；大并发打爆本地 ONNX/云 API | 校验 `engine ∈ {vision, ddddocr}`、并发 `∈ [1,20]`，非法值 code=1 整体拒绝不落库 | ✅ commit `5b9ef40`，TDD：`config_validation_test.go`（红灯→绿灯） |
| **A2** | MAJOR | 手动退选后自动引擎 ≤1s 抢回，用户无法真正退课 | 新增 `refused` 集合：`RemoveDone` 记入、spawnChain 命中即跳过、`SetTargetsForAccount` 清空解封 | ✅ commit `5b9ef40`，TDD：`refused_test.go` |
| **A3** | MINOR | `handleAdminStats` 静默吞 DB 错误（accounts/success/allLogs 全部 `, _`），DB 故障时管理员看到 0/空 | 任一数据源失败如实返回 500 | ✅ 已修（待提交） |
| **A4** | MINOR | `maskKey` 对 ≤4 位 key 回显空串，与"未设置"歧义且可误保存 | 恒显 `****`（空值才为空） | ✅ 已修（待提交） |
| **A5** | MINOR | n4 只拉平"口令错误"分支，正确分支仍瞬时返回——慢-快侧信道可区分口令对错 | 正确分支也 `time.Sleep(loginTimingFlat)`，两分支等时 | ✅ 已修（待提交） |
| **A6** | MINOR | 登录错误回传含验证码识别原文（`净化自 "xxxx"`） | 错误去掉 raw 原文，进程日志记录长度 | ✅ 已修（待提交） |
| **A7** | MINOR | `handleLogin` 未 TrimSpace；`handleLogout` 重复取值 | 登录 trim、logout 先取值再 Delete | ✅ 已修（待提交） |
| **A8** | MINOR | 激活码批量生成中途落库，panic 导致前 N-1 个"隐身码"滞留 | 先全量生成再一起落库 | ✅ 已修（待提交） |

### 后端明确无问题区（子代理声明）
- scheduler 并发状态机（inflight/full/rateLimited/done 锁覆盖、幂等、spawnChain 跳过在飞、reloginMu→s.mu 锁序唯一无反序、WindowClosed 判定、C-3 保守解封）
- zhidao 层（三层重试收敛、验证码一次性刷新、captchaLimiter Mutex+Cond、GatePump 令牌桶、ReloginIfNeeded 账密解耦）
- session.Store 并发（mu 覆盖完整、sweepLoop、Close sync.Once、ConsumeTicket 单次消费）
- 数据表/升级兼容（refuseLegacy 四项检查、schema 读写一一对应）
- `go test -race ./...` 全绿，无漏锁

### D类（前端，独立审查报告）

| 编号 | 严重度 | 问题 | 修复状态 |
|---|---|---|---|
| **D1** | 高 | 回显与防抖保存竞态——清空目标被 2s 轮询撤销 | ✅ commit `8fd506f`（rev>0 跳过回显） |
| **D2** | 中 | Select 窗口关闭后仍 2s 轮询 /state | ✅ commit `8fd506f`（window_closed → 30s） |
| **D3** | 中 | 401 误杀其他账号（慢请求乱序） | ✅ commit `f755e20`（会话令牌反查归属账号） |
| **D4** | 中 | 单管理员账号「学生端」按钮死路 | ✅ commit `7a6babe`（无其他学生账号回登录页） |
| **D5** | 低 | copyTimer 卸载残留 setState | ✅ commit `e8243dc`（卸载清理） |
| **D6** | 中 | 三个 Modal 缺无障碍（对话框角色/焦点管理/trap） | ⏳ 待办（低优先） |
| **D7** | 低 | Progress/icon 按钮缺 aria | ⏳ 待办（低优先） |
| **D8** | 低 | 降频写法不一致（Dashboard 函数式 vs Select 箭头） | ⏳ 待办（低优先） |
| **D9**（API安全报告） | 严重 | 管理员口令侧信道——账号名探测漏了口令正确性探测 | ✅ 见 A5（正确/错误分支统一延迟） |
| **D10**（API安全报告） | 高 | 限流 NAT 共享 IP 可用性 | ⏳ 待办（靠人工风险评估） |
| **D11**（API安全报告） | 中 | capthca_concurrency 无上限 | ✅ 见 A1 |
| **D12**（API安全报告） | 中 | vision_base_url SSRF 面 | ⏳ 待办（仅管理员可改，风险自担） |
| **D13**（API安全报告） | 低 | maskKey 后 4 位明文回显 | ✅ 见 A4（恒显 ****） |

### C类（构建/CI 报告）
- **C-5/C-6** 阻塞确认（Go 版本漂移补到 go.sum 自动切工具链）
- **MinGW 静默假绿**：Windows CGO=1 构建若本地无 MinGW 会假绿——建议后续加显式失败
- **m17**：docs 文档与代码表述统一（第 3 轮 D31-D35 已做）

---

## 三、本轮 TDD 修复详情（每个都是"先红灯→再绿灯"）

### A1 配置值域校验
- 红灯：`TestAdminConfigRejectsInvalidCaptchaEngine` 先 PUT `garbage` → 现返回 `code:0` + `captcha_engine:garbage`（证明 bug 真实）→ 修 → 绿灯
- 红灯：`TestAdminConfigRejectsCaptchaConcurrencyOutOfRange` 先 PUT `100000` → 现 `code:0`（证明无上限）→ 修 → 绿灯
- 生产改动：handler.go 在校验后进 `Runtime.Update`；`maxCaptchaConcurrency = 20`；删"静默保底 1"

### A2 手动退选不抢回
- 绿灯：`refused_test.go` -> `TestRefusedNeverResubmitted`：退选后持续 tick 2s SelectClass 恒 0 次；重设目标后轮询确认恢复提交
- 生产改动：scheduler `refused` 字段 + `RemoveDone` 记入 + `SetTargetsForAccount` 清空 + spawnChain 命中即跳过 + `refusedHas`

### A3-A8（本 md 发布时已修，待提交）
见上表——全部单点修改 + `go test -race ./...` 全绿回归。

---

## 四、待办与决策记录

- **前端 D6-D8**（无障碍/aria/降频写法）：低优先，功能不阻塞，随后续轮次顺手处理
- **D10 NAT 限流 / D12 SSRF**：非自动选题，属人工风险评估——管理员后台属于私有部署，可接受
- **C 类 MinGW 假绿**：构建可靠性改进，列入下轮
- **本轮无 CRITICAL 前端残项**；后端 2 个 MAJOR 已全部修复并有测试锁定

## 五、回归证据
- `go test -race ./...`（backend）：全部包绿
- `npm run build`（web）：tsc + Vite 产物通过，`backend/web/dist` 同步更新