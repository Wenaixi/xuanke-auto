# review-round55 总结（2026-09-21）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端零缺陷轮（MAJOR 0 / MINOR 0 / OBSERVE 8 全延续）；后端聚焦 **R54 task_log 档①的验证结论被实证推翻**——R54 标称的"flake 归零"未复现，且发现 R54 引入的窗口测试是确定性性能超时缺陷。

**修复 2 处**（全部测试基建层，非产品逻辑）：`store_test.go` 窗口测试事务批插（225s→0.5s）+ `handler_test.go` readyProbe 轮询重试（5×200ms）。均先红后绿 TDD 闭环。

## 审查发现（写入 archive/review-rounds/round55-{backend,frontend}-findings.md）

### 后端（MAJOR 2 / OBSERVE 6 新增 + 延续 13）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-55-01 | MAJOR | R54"flake 归零"结论被本轮 3/12 轮实证推翻：① `TestLoadLogsWindowKeepsRecent` 逐条 INSERT 30050 行每行一次 WAL 同步 commit，单测实测 ~225s（10 轮 193-249s），CI 默认 `-timeout 10m` 必红（`-timeout 100s` 稳定 panic）——**确定性性能超时**；② api 夹具 readyProbe 单次重试未根治冷启动 connectex（隔离 5 轮 2 FAIL / 全量 3 轮 2 FAIL） | ✅ 修复（批插 + 轮询） |
| MAJOR-55-02 | MAJOR（潜在） | 与 55-01 同根：CI 无 `-timeout` 覆盖时窗口测试必红 | ✅ 随 55-01 闭合 |
| 新视角 A | — | **task_log 空库窗口语义实证：`max(id)` NULL → `id>NULL` 恒 false → 返回 0 行 = 应有行为**（空库本无日志可展示），非缺陷；无需 COALESCE 补丁 | ✅ 契约固化测试 |
| 新视角 B-F | — | credentials 全表毫秒级无竞态 / session sweeper ttl<=0 无泄漏 / runtime 热改无半态 / 锁内 SQLite 写 <5ms / refuseLegacy 空库正确——全无缺陷 | ✅ |
| OBSERVE-55-01 | OBSERVE | 空表行为已实证正确但测试未覆盖——建议补空表契约断言 | ✅ 修复（window_empty_test.go） |
| OBSERVE-55-02~06 | OBSERVE | credentials 全表加载 / sweeper / runtime 热改 / 锁内写 / refuseLegacy——均延续观察 | ⚠️ 延续 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 8）
R54 前端五修（横幅兜底 / duration 合并 / viewport 上限 / 右栏兜底 / open_time_set）全路径复核通过；F43/F42/F40/F39/F36/F48-M1 六防保存链逐字符零回归；tsc+build 全绿。零修复项。新视角 A（Toast viewport 滚动）结论：M-C 修复达成目标，残余交互（滚轮需悬停 toast、滚动条不可拖）为 Radix 官方 pointer-events 模式自然产物，定 OBSERVE。

## 修复（主控核实后直修，全部测试基建）
| 文件 | 内容 | TDD |
|---|---|---|
| `backend/internal/store/store_test.go` | 窗口测试事务批插（每 1000 行一批，仿独立程序实证 0.48s）替代逐条 INSERT | 先红（30s 超时 panic 复现）→ 后绿（0.51s） |
| `backend/internal/api/handler_test.go` | readyProbe 单次重试升级为轮询重试（200ms × 5 次，总窗口 ~1s 覆盖冷启动 TIME_WAIT 排空） | api 隔离 5 轮 0 FAIL |
| `backend/internal/store/window_empty_test.go` | 新增：空库窗口语义契约固化（LoadAllLogs/LoadLogs 空库返回空不报错） | 绿（0.08s） |

## 收尾全量回归
- `go build ./... && go vet ./...` → exit 0
- `go test -race -count=1 -p 1 -timeout 900s ./...` → **全包绿**（store 40.7s 从 223s 降 82%；api 25.1s）
- api 包隔离 race 5 轮 → **5/5 全绿 0 FAIL**（readyProbe 轮询修复实证生效）
- store 窗口测试单测 → 0.48s（原 225s）
- 前端 `npm run build` → 全绿（1948 modules / 417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：R54 档①已闭合 + 空库契约已固化 / accountExists 全表（55-02）/ tick 无 recover / 孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / failingTargetsStore / 429 头 / XUANKE_PORT / 本地钟；前端：N-2~N-3 / O-2~O-8 / Dashboard key 不对称 / ui 模板残宽 / useTickingCountdown 每秒重渲 / 多标签页一致性 / "NaN"防御。

## 教训
1. **R54"flake 归零"是窗口期巧合**——R54 收尾 7 轮 0 FAIL 与 R55 全量 2/3 轮 FAIL 同环境不同结论，flake 复现频率本身有波动性，单轮次"归零"不能作为降级关闭依据，需连续多轮统计口径（后续以"非 api 包 0 FAIL + api 隔离多轮"报告）。
2. **测试数据准备方式本身就是性能缺陷**——逐条 INSERT 的测试在开发机慢、在 CI 更慢，写大量数据行时必须批事务（本轮 30050 行 225s→0.5s 实证）。
3. **readyProbe 单次重试不够**——冷启动窗口宽于单次探测-重试间隙时仍 connectex，轮询重试（有界 5×200ms）才是"把冷启动窗口前移到夹具构造期"的完整实现。
