# review-round82 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 3 / OBSERVE 4**；前端 **MAJOR 0 / MINOR 0 / OBSERVE 1**。前端连续**二十八轮**零严重级；后端本轮无产品代码缺陷，核心产出是 M81-02 抖动归因的高频压测收敛 + 一处测试夹具卫生修复。

## 审查发现（写入 archive/review-rounds/round82-{backend,frontend}-findings.md）

### 后端（MINOR 3 / OBSERVE 4，M81-02 归因收敛）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M82-01 | MINOR | **M81-02 抖动归因收敛**——`-race -count=10`（248s 百万断言）+ 无 race `-count=5` 高频压测零复现，四抖动测试（TestAccountOverride/TestAdminAuth/TestLoginRateLimit/TestHandleElectivesSelectUnauthorizedRelogin）证据链从"偶发"推进到"多轮高频稳定全绿"，归因牢固指向夹具共享全局态时序敏感 | ⚠️ 维持观察（CI 基线已吸收，证据链强化） |
| M82-02 | MINOR | tray 回归钉平台盲区——Windows ICO 实测通过、Linux PNG 依赖 CI ubuntu runner（单一宿主无法双平台实测） | ⚠️ 记录（平台分工合理，CI 覆盖另一半） |
| M82-03 | MINOR | 全轮无新产品缺陷，probeSem ponytail 标记 + 既有观察留档 | ⚠️ 记录 |
| O82-01 | OBSERVE | **api 夹具 `session.New(time.Hour)` 未注册 Close——550 空闲清扫协程窗口（测试资源泄漏）** | ✅ **实修**（补 `t.Cleanup(sessions.Close)` 一行，sync.Once 幂等零风险） |
| O82-02 | OBSERVE | `WriteTimeout 30s` 与 Vision 识别 60s 长尾交集——公网慢网络下登录请求可能被自身写超时断连（配置权衡，不破坏数据/安全） | ⚠️ 维持观察（权衡项，未来公网部署实测闭环） |
| O82-03 | OBSERVE | ResetGateForTest 测试语义（进程级闸门重置，包间无交叉） | ⚠️ 维持观察 |
| O82-04 | OBSERVE | handleSetTargets O(n) 全表比对（千级账号才值得缓存，当前规模无影响） | ⚠️ 维持观察 |
| 复核全表 | — | build tag 四组合交叉编译全绿 / Windows 测试二进制实测 TestTrayIconAsset 通过 / 契约 20 零标签 / 8 项新角度契约抽核 + 32 处定向测试族全过 / 11 包 -race -p 1 全绿 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 1）
**R81 注释口径修复核证成立**（:501 改"守卫不置 dirtyRef"后三处口径统一；全仓"置脏"字样 14 处清点仅原绑定点已剥离；`dirtyRef=true` 置位仍仅三处）。**M-1 第十八轮闭合** + setSelected 七调用点无新增 + 契约 20 含"32-01/33-01 数字横线锚点"经 R32/R33 归档溯源确认为工程决策编号非轮次标签。
- **OBSERVE-82-01**（StatsTab 失败文案与同族三 Tab 极弱一致性观察）⚠️ 按裁决"可不修"维持观察——纯文案无行为风险。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `336fce0` | 后端：api 夹具 session Cleanup 补注册（O82-01，一行 t.Cleanup(sessions.Close)） |

**验证**：go build/vet/gofmt 零输出 + 定向五测试 -race 全绿（2.99s）。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 11 包全绿（审查代理实证 api 19.8s / store 41.9s / scheduler 14.6s）
- `go build` / `go vet` / `gofmt -l` 零输出 + 四组合交叉编译全绿
- 前端 `npm run build` 绿 + 三组断言全绿（审查代理实证）

## 观察项延续（下轮复核）
后端：M82-01 抖动证据链（多轮高频零复现，精确断言行仍待有复现环境捕获）/ O82-02 WriteTimeout-Vision 长尾交集（公网部署实测闭环）/ O82-03/04 记录；前端：M-1 延续管理（第十九轮）/ OBSERVE-82-01 文案一致性 + 76/77/78 系列 + 66-03 全表续 / 可疑-1 物理不可达维持。

## 教训
1. **抖动归因的证据链强度 = 压测轮次 × 断言覆盖**：R80 记录 2 测试偶发 → R81 扩展到 4 → R82 高频 10+5 轮零复现——归因从"偶发"推进到"稳定全绿"，比单次复现更有说服力。夹具共享全局态（闸门/限流桶/验证码信号量/mock Handler）时序敏感的定性被三重证据（单跑全绿 + -p 1 全绿 + 无 race 也偶发）钉死。
2. **测试夹具卫生是长期债**：api 夹具 55 测试 × 10 轮 = 550 个常驻清扫协程，一行 `t.Cleanup(sessions.Close)`（幂等 Close 设计就是为了 Cleanup 复用）可收干净——审查报告提 OBSERVE 修复成本一行，但累积起来影响调度与内存账。
3. **"平台分工"要显式记录**：托盘双回归钉 Windows/Linux 各自平台约束，单一宿主无法同时实测——R82 用交叉编译测试二进制实测 Windows 侧 + CI ubuntu 覆盖 Linux 侧，平台执行面事实要写进结论避免"我验证过"的误解。
4. **配置权衡留档比动手改更重要**：WriteTimeout 30s vs Vision 60s 的交集是真实边界但不紧急——改超时会牵连 slowloris 防线（ReadTimeout 保持），权衡项记录触发条件与修复候选，等有公网复现环境再闭环。