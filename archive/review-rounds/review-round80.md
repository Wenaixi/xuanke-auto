# review-round80 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 3**；前端 **MAJOR 0 / MINOR 0 / OBSERVE 1**。前端连续**二十六轮**零严重级；后端 R79 ICO 修复经 `CreateIconFromResourceEx` 实测确认正确。本轮修复聚焦"测试验证力"与"判据冗余精简"两类工程卫生。

## 审查发现（写入 archive/review-rounds/round80-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 3，生产契约全表核证通过）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M80-01 | MINOR | api 包两测试（TestAccountOverrideRequiresAdminSession 45s / TestAdminElectiveSelectUnknownAccountRejects 2s）偶发抖动失败——闸门预算并发消耗 + 快照 TTL 时序敏感，单跑/`-p 1` 全绿非产品缺陷 | ⚠️ 维持观察（CI 固化 `-p 1` 串行基线） |
| M80-02 | MINOR | 回归钉 bytesInRes 断言 `len(ico)-22` 与实现同式复算，削弱对"实现算术散失"的失验力 | ✅ **实修**（改三加数独立推导硬编码 4264） |
| O80-01 | OBSERVE | handleAdminDeleteAccount 删除保护单判 IsAdminAccountName，未与 B43-04 撞名双条件对齐（管理员名需显式配成学生账号才触发） | ⚠️ 维持观察 |
| O80-02 | OBSERVE | probe() 每账号 goroutine `_, _ = s.ProbeForAccount(acct)` 吞探测错误（内部已记日志，非落库吞错） | ⚠️ 维持观察 |
| O80-03 | OBSERVE | config.go ensureEnvFile `_ = os.WriteFile` 静默吞错（CLI 惯例） | ⚠️ 维持观察 |
| 复核全表 | — | R79 ICO entry 修复（独立程序复刻 + CreateIconFromResourceEx + GetIconInfo 实测加载成功）/ build tag 三平台交叉编译 / trayPNG+ICO 双回归钉 / 契约 20 零残留 / 六项生产契约全通过 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 1）
**R79 四项修复全部核证成立**（StatsTab data-first / guardBlockedRef 拆分完整十处无漏 / LogsTab isLoading / R63 标签剥离）。**M-1 第十六轮闭合** + setSelected 七调用点无新增。
- **OBSERVE-80-01**（guardBlockedRef 短路在 dirtyRef 拆分后语义冗余，且"会话内任何一次命中即永久短路"引入极端"该弹没弹"路径——同一次 handleBack 内先守卫命中、等帧期间用户继续点选、后续真实保存失败时终局 toast 被残留标记压制）✅ **修复**（移除 guardBlockedRef 声明/十处置位/入口清标记，pendingUnsaved 只查 dirtyRef/savingRef/timer 三真实失败信号——守卫不置 dirtyRef 自然不弹，语义更简洁且消除该弹没弹路径）。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `9b32c65` | 前端：移除 guardBlockedRef（OBSERVE-80-01）——pendingUnsaved 精简为三真实失败信号，十处守卫置脏改纯 return |
| `3ba175e` | 后端：回归钉 bytesInRes 断言改三加数独立推导硬编码 4264（M80-02） |

**验证**：tsc -b 绿 + `npm run build` 476ms 绿 + 三组断言全绿；TestTrayIconAsset（硬编码 4264/22）全绿 + gofmt 零输出。

## 收尾全量回归
- `go test -race -count=1 -p 1 -timeout 900s ./...` → 全 10 包全绿（待跑）
- `go build` / `go vet` / `gofmt -l` 零输出
- 前端 `npm run build` 476ms + 三组断言全绿

## 观察项延续（下轮复核）
后端：M80-01 api 偶发抖动（CI `-p 1` 基线固化）/ O80-01/02/03 维持观察；前端：M-1 延续管理（第十七轮）/ OBSERVE-76-01/02/03 + 77-01/02 + 66-03 全表续 / 可疑-1 物理不可达维持。

## 教训
1. **回归钉的验证力与"是否与实现同式推导"直接相关**：bytesInRes 断言 `len(ico)-22` 与实现 `len(ico)-headerSize` 同根，实现算术散失时测试跟着复算仍绿——按格式语义三加数独立推导（40+4096+128=4264）才与实现根因不同源。二进制资产回归钉的"硬锚值"应来自格式规范本身而非实现公式。
2. **拆分后的冗余防御会引入新的极端路径**：R79 加的 guardBlockedRef 短路在 dirtyRef 已精确表达真实失败后是纯冗余，且"会话内一次命中即永久短路"的语义带来该弹没弹。修复后守卫分支改为纯 return（守卫拦截本就是"不 PUT 等自愈"），判据回到最小正确形态——每轮修复都要问"这个防御还有必要吗"。
3. **测试偶发抖动与产品逻辑缺陷要分开定性**：M80-01 两测试（闸门预算/快照 TTL）单跑与 `-p 1` 全绿、并发偶发红，指向夹具时序敏感而非产品 bug；CI 基线固化 `-p 1` 串行是确定性优先的工程姿势。
4. **审查链连续正向验证**：R79 ICO 修复被 R80 用 CreateIconFromResourceEx 实测确认（与 R79 的 LoadImageW 双重独立实证），托盘资产从"两次失位"走向"双工具链实测+硬锚回归钉"的稳定态。