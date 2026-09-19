# review-round40 总结（2026-09-20）

## 概述
按主控协议走完整循环：两个 opus 只读审查代理（后端/前端）并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 派两个 sonnet 修复代理串行 TDD 修复 → 收尾全量回归。发现后端 8 条 + 前端 8 条，确认修复后端 2 条 + 前端 5 条，1 条 MAJOR 经核实为误报，其余维持观察。本轮特色：审查代理对 R39 修复做了复审（发现 R39 C-1 引入静默死锁、N-1 只改一半），且 1 条 MAJOR 被主控核实阶段识破为误报（体现核实环节价值）。

## 审查发现（写入 archive/review-rounds/round40-{backend,frontend}-findings.md）

### 后端 8 条（MAJOR 2 / MINOR 3 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M40-01 | MAJOR | maybeRelogin 退避/节流判定混用本地钟 time.Since（与对齐钟族整体偏移 ≤640ms/30s） | ⚠️ 观察（读写同基内部自洽，与 m39-02 同族） |
| M40-02 | MAJOR | "自动重登绕过 gateWait 闸门" | ❌ **误报**：scheduler 走接口 Relogin → accounts.Manager.Relogin **内部有 gateWait**（manager.go:163）→ ReloginIfNeeded，闸门完全生效。审查代理被接口间接层误导 |
| m40-01 | MINOR | requireJSONBody CSRF-403 仍走 writeJSON 恒 200（B39-02 改漏） | ✅ 修复（B40-01） |
| m40-02 | MINOR | spawnChain 锁内多次取 now（亚毫秒统计噪音） | ⚠️ 观察 |
| m40-03 | MINOR | config.OpenTime 死字段 + 硬编码 2026 日期 + env 残留 | ✅ 修复（B40-02） |
| o40-01~04 | OBSERVE | 未激活不发会话无残留 / reloginResults channel 满丢弃刻意 / columnExists 拼接全常量 / interval≤0 无防御（生产恒 300ms） | ⚠️ 观察（已核实无真实触发或已文档化） |

### 前端 8 条（MAJOR 3 / MINOR 3 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-1 | MAJOR | R39 C-1 守卫命中即静默置脏 + 旧发布残留 key 无 UI 清除入口 = 保存链静默死锁（黄金期触发概率最高） | ✅ 修复（F40-M1） |
| M-2 | MAJOR | R39 N-1 只改倒计时矩阵输入、文案行未同步 → 矩阵倒数 + 同屏"未识别"自相矛盾 | ✅ 修复（F40-M2） |
| M-3 | MAJOR 复证 | /state 失败时 refetchInterval 恒 2s（R39 M-2 未修） | ✅ 修复（F40-M3） |
| N-1 | MINOR | Dashboard UTC 今天基准致 UTC+8 凌晨排序错档 | ✅ 修复（F40-N1） |
| N-2 | MINOR | aria-controls 悬空 + 多实例 id 冲突 | ✅ 修复（F40-N2） |
| N-3 | MINOR 复证 | resetRetry 清零指数退避（R39 已观察） | ⚠️ 观察（危害有限） |
| O-1/2 | OBSERVE | useTickingCountdown 全树重渲染（结构与注释承诺分叉）/ 多 Tab 会话等 | ⚠️ 观察 |

## 修复（TDD 严格模式，独立 commit，未 push）

### 后端 2 条（修复代理 accb0f0de23e26346）
| 缺陷 | commit | 测试形态 | 红→绿 |
|---|---|---|---|
| B40-01 | `4c876fb` | TestRequireJSONBodyRejectsFormContentType | http=200 红→403 绿 |
| B40-02 | `9980a47` | TestConfigDoesNotInjectOpenTime（reflect 命中字段） | 字段存在红→已移除绿 |

### 前端 5 条（修复代理 acd0c890aa373f703）
| 缺陷 | commit | 测试形态 |
|---|---|---|
| F40-M1 | `808fae7` | cleanStaleSelected 纯函数 + target-guard-check.ts 追加 5 场景（5→10 全绿，先红 SyntaxError） |
| F40-M2 | `533ccde` | 类型校验 + 逻辑走查（四级降级同源） |
| F40-M3 | `7618a26` | 逻辑走查 + 类型校验 |
| F40-N1 | `63bee78` | 抽 localTodayMs 本地零点（可测）+ 逻辑走查 |
| F40-N2 | `dc74d7a` | 类型校验 + npm run build |

## 新增决策锚（已沉淀进根 CLAUDE.md）
27. **目标保存守卫必须带解锁路径**（F40-M1）：守卫命中仅"置脏跳过"是半截——若无 UI 提示/无删除 stale key 的自愈，用户被锁死在"看起来保存成功实则永存不上"的静默死锁（C-1 防数据丢失却引入无出口）；补 `cleanStaleSelected`（只删非空且不在当前发布的 key、空 key 清空语义保留、无变更返回原引用）随发布重建清理 + 防抖/flush 守卫命中补 toast（判 unmountedRef）
28. **基础设施状态码必须成家族核对**（B40-01）：B39-02 改 4 类路径但漏 requireJSONBody CSRF 门——同类基础设施错误路径（鉴权/CSRF/限流/panic）改真实状态码时逐家族核对
29. **死配置字段必须连同文档清除**（B40-02）：Config.OpenTime 无消费点 + 硬编码过期日期 + env 残留并存是维护陷阱——删除字段时同步删 env 读取与 .env 模板行

## 核实教训（主控阶段识破 1 条 MAJOR 误报）
- **M40-02 误报**：R40 后端代理称"调度器走 s.clients.Relogin 绕过 accounts.Manager gateWait 闸门"。核实：scheduler.AccountClients 接口由 accounts.Manager 实现，Manager.Relogin（manager.go:156-165）内部明确调 gateWait 才 ReloginIfNeeded → 闸门完全生效。审查代理被接口间接层误导（看到接口方法名就以为直连 ReloginIfNeeded）。教训：涉及接口间接触发的"绕过/未收敛"指控，必须先追到真实实现再下结论。

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `cd backend && go test -race -count=1 ./...` → 9 包全绿
- `cd web && npx tsc -b && npm run build` → 前端代理验证全绿（tsc exit 0 + vite build 成功）

## 观察项延续（下轮复核）
后端：M40-01 本地钟混用（读写同基维持）/ m40-02 锁内多取 now / o40-01~04 / round39 延续 m39-02、o39-01~05；前端：N-3 resetRetry 清零 / O-1 useTickingCountdown 全树重渲染 / O-2 多 Tab / round39 延续 M-2（已修）外全部观察项
