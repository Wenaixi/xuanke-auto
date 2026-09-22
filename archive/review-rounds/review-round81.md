# review-round81 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 2**；前端 **MAJOR 0 / MINOR 0 / OBSERVE 1**。前端连续**二十七轮**零严重级；后端 R80 回归钉硬锚 4264 修复经独立审查验证正确。本轮后端零代码修改需求（两项 MINOR 均为验证确认 + 预防性记录），前端一项注释口径修复。

## 审查发现（写入 archive/review-rounds/round81-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 2，核心复核全通过）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M81-01 | MINOR | R80 回归钉硬锚 4264 三加数独立推导**验证正确**（与实现不同源、offset 硬编码 22）；残留边界仅"未来改 ICO 尺寸会失验力"（当前尺寸固定 32x32 不可达） | ⚠️ 预防性记录，不修（当前契约不可达） |
| M81-02 | MINOR | api 包偶发失败面 R80 2 个→4 个（新增 TestAdminAuth/TestLoginRateLimit/TestHandleElectivesSelectUnauthorizedRelogin），全部单跑全绿/`-p 1` 全绿/无 race count=4 全绿——夹具共享全局态（闸门/限流桶/识别信号量/mock Handler）并发时序敏感，非产品缺陷 | ⚠️ 维持观察（CI `-p 1` + `||` 重跑已吸收） |
| O81-01 | OBSERVE | TestLoginRateLimit 限流桶断言在整包并发下被验证码识别信号量排队拉长窗口，限流判定天然不触发 | ⚠️ 维持观察（建议测试注入独立 limiter） |
| O81-02 | OBSERVE | TestHandleElectivesSelectUnauthorizedRelogin 替换共享 mock Handler 存在"其它测试正在探测旧 Handler"窗口 | ⚠️ 维持观察（建议独立 server） |
| 复核全表 | — | 回归钉硬锚 4264 验证正确 / guardBlockedRef 无后端耦合 / build tag 四文件三平台交叉编译 / 双回归钉 / 契约 20 零标签 / 11 项生产契约抽核全过 / 十包 -race -p 1 全绿 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 1）
**R80 guardBlockedRef 移除六条核证全部成立**（零残留 / 终局六判据闭环、原 OBSERVE-80-01 该弹没弹路径物理消除 / dirtyRef 三处置位语义收敛 / 无死代码 tsc noUnusedLocals 实证 / 状态机双分支正确 / pendingUnsaved 双闭合）。**M-1 第十七轮闭合** + setSelected 七调用点无新增 + R79 三修无回潮。
- **OBSERVE-81-01**（Select.tsx:501 守卫命中注释"置脏（dirtyRef=true）"与 :507"守卫不置 dirtyRef"自相矛盾——纯注释口径残留，代码行为正确）✅ **修复**（改"脏块保留在内存 selected（守卫不置 dirtyRef——终局绝不误报保存失败）"统一口径，全仓 `dirtyRef=true` 注释绑定零残留）。

## 核实裁决（主控）
- M81-01：审查验证的是"我 R80 修复正确"，残留边界为未来扩展预防记录，当前尺寸固定不可达——不修，观察。
- M81-02 / O81-01 / O81-02：api 夹具共享态并发时序敏感的既有观察族延续，CI 基线已吸收；测试注入独立实例属长期方向，不阻塞本轮。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `6b574ff` | 前端：OBSERVE-81-01 注释口径统一（守卫命中"脏块保留在内存 selected"非 dirtyRef） |

**验证**：tsc -b 绿 + 三组断言全绿；后端审查独立实证十包 `-race -p 1` 全绿 + go build/vet/gofmt 零输出 + 三平台交叉编译全绿 + npm build 绿。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 十包全绿（api 25.79s / store 50.10s / scheduler 15.05s，审查代理实证）
- `go build` / `go vet` / `gofmt -l` 零输出 + 三平台交叉编译全绿
- 前端 `npm run build` 绿 + 三组断言全绿

## 观察项延续（下轮复核）
后端：M81-02 api 抖动（夹具共享态时序敏感，失败面 4 测试持续观察）/ O81-01/02 测试夹具改造建议 / M81-01 未来扩展尺寸预防记录；前端：M-1 延续管理（第十八轮）/ OBSERVE-76-01/02/03 + 77-01/02 + 78-01 + 66-03 全表续 / 可疑-1 物理不可达维持。

## 教训
1. **"验证确认"也是审查的重要产出**：M81-01 不是新缺陷，是独立证实我 R80 的硬锚修复正确（三加数独立推导 vs 实现同式的区别被复核确认）——审查链的价值既在抓缺陷也在证实修复，两者防止"修了又改回去"。
2. **夹具共享全局态是 api 并发测试抖动的高频根因**：登录闸门/限流桶/验证码信号量/mock Handler 全部跨测试共享，单跑全绿、整包并发偶发红是典型形态——归因证据链（单跑全绿 + `-p 1` 全绿 + 无 race 也偶发 → 时序敏感非数据竞争）比"修一个测试"更有价值，CI 固定串行 + 重跑是工程姿势。
3. **测试断言假设"固定速率"在并发排队下天然失效**：TestLoginRateLimit 假设 5 次/分钟限流，但并发识别信号量排队拉长间隔 → 限流判定不触发——测试夹具改造（注入独立 limiter）是长期方向。
4. **注释口径要随语义改动同步收敛**：R80 把"守卫置脏"改为"守卫不置任何标记"后，R79 时代遗留的"置脏（dirtyRef=true）"注释误导维护者——纯注释残留也属于"发现即修复"范畴，且审查明确指出后修复成本一行。