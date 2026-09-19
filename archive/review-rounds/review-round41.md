# review-round41 总结（2026-09-20）

## 概述
按主控协议走完整循环：两个 opus 只读审查代理（后端/前端）并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 派两个 sonnet 修复代理串行 TDD 修复 → 收尾全量回归。发现后端 2 条（含 1 CRITICAL）+ 前端 4 条，全部确认真缺陷并修复。本轮特色：后端 CRITICAL 是 R39 B39-01 同类防线的不可达分支残余（成功/失效分支复核了、err 归并路径漏了），体现"成族核对"的价值。

## 审查发现（写入 archive/review-rounds/round41-{backend,frontend}-findings.md）

### 后端 2 条（CRITICAL 1 / MAJOR 1，宁缺毋滥）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| CRITICAL-41-01 | CRITICAL | spawnChain 风控退避/窗口关闭/实时复核满员三分支缺 B39-01 指针身份复核——删号同名重建后陈旧链把 full/rateLimited 写进重建身份（假满员永久退避黄金期） | ✅ 修复（B41-01） |
| MAJOR-41-02 | MAJOR | tick 零值守卫 `open.IsZero()` 在 WindowOpened=true（发布级 inDateRange 确证开窗）且识别槽空时仍挂起提交，黄金期 0 提交 | ✅ 修复（B41-02） |

### 前端 4 条（MAJOR 2 / MINOR 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-1 | MAJOR | R40 cleanStaleSelected 被回显 effect 首行 echoedRef 短路——只覆盖首次回显，已回显账号发布重建后残留旧 publish_id 无自动清理（实现与决策锚"随重建清理"分叉） | ✅ 修复（F41-M1） |
| M-2 | MAJOR | flushTargets 缺"回显未完成"守卫（防抖回调有、flush 没有）——handleBack 5s 超时后整包 PUT 覆盖删后端旧目标 | ✅ 修复（F41-M2） |
| N-1 | MINOR | 名额未公布（max_count=0）时 Progress 满条误导 | ✅ 修复（F41-N1） |
| N-2 | MINOR | 网关返回非 JSON 401 时 r.json() 抛错、不广播 UNAUTHORIZED_EVENT，失效会话不摘除 | ✅ 修复（F41-N2） |

## 修复（TDD 严格模式，独立 commit，未 push）

### 后端 2 条（修复代理 ac93814e00e2bdc6a）
| 缺陷 | commit | 测试形态 | 红→绿 |
|---|---|---|---|
| B41-01 CRITICAL | `46a991a` | TestDeletedAccountRebuiltSameNameChainDrops{RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + waitChainExit 辅助 | 三分支重建身份被写红→静默放弃绿 |
| B41-02 MAJOR | 同上 | TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime + TestSubmitSuspendedWhenOpenTimeCleared 仍绿 | 0 次 SelectClass 红→放行绿 |

### 前端 4 条（修复代理 a62f4d1bd432bd91c）
| 缺陷 | commit | 测试形态 |
|---|---|---|
| F41-M1 | `16d37cb` | 独立 effect（TDZ 安全、回显块删除）+ 时序论证 |
| F41-M2 | `8ac4563` | flush 回显守卫 + 逻辑走查 |
| F41-N1 | `bc77e58` | 空条 + aria-valuenow 如实 |
| F41-N2 | `7b5028c` | 401 前置广播 + extractAccountFromPath 纯函数 + unauthorized-check.ts 5 断言红绿 |

## 新增决策锚（已沉淀进根 CLAUDE.md）
31. **B39-01 身份复核必须成族覆盖 spawnChain 全部分支**（B41-01）：成功/失效分支复核后，风控退避（isRateLimitError）/窗口关闭（isWindowClosedError）/实时复核满员三条 err 归并路径同样要 `sameClientFor`——否则删号同名重建的陈旧链反向写 full/rateLimited（假满员永久退避黄金期）。测试必须为每条分支独立红绿（3 个测试），并复用 waitChainExit 等待契约（PurgeAccount 后 inflight map 已删、读 nil map 恒 false 会假绿）。
32. **tick 零值守卫必须让位于 WindowOpened**（B41-02）：`open.IsZero() && !opened` 才挂起提交——B11-A1 防轰炸本意是"未开窗"场景，WindowOpened=true（发布级 inDateRange 确证）时轰炸目标已消失，过宽守卫误伤黄金期 250ms 冲刺；probe 的"开窗"判定（发布级）与 tick 的"提交放行"（顶层级时刻）语义必须统一，前端显 window_opened=true 而引擎从未提交的分叉绝不允许。

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `cd backend && go test -race -count=1 ./...` → 9 包全绿
- `cd web && npx tsc -b && npm run build` → 前端代理验证全绿

## 观察项延续（下轮复核）
后端：M40-01 本地钟混用（读写同基维持）/ m40-02 锁内多取 now / o40-01~04 / round39/40 延续全部；前端：N-3 resetRetry 清零 / O-1 useTickingCountdown 全树重渲染 / O-2 多 Tab 结案（无互踢实现不构成缺陷）/ round40 延续全部
