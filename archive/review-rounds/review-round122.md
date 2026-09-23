# review-round122 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（OBSERVE-111-01/112-03 盯守第十一轮 + OBSERVE-117-01 知识位第五轮确认在位 + **B110-01 观察项转正**）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续五十九轮零严重级）。**双端零修复需求——连续第十二轮纯观察轮**。核心产出：**身份防线矩阵第三十七轮闭合（零产品改动链第十七轮延续）+ OBSERVE-117-01 知识位第五轮确认在位 + B110-01 观察项转正为契约（自动/手动审计链完全对称）+ 限流退避可观测持久化走查 + 前端 M-1 第五十八轮闭合 + 错误文案前端可辨性对照**。

## 审查发现（写入 archive/review-rounds/round122-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 盯守第十一轮 + 知识位第五轮 + 观察项转正）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 知识位第五轮 | maybeRelogin 写回侧（scheduler.go:1254-1274）:1254 先 ClientFor 复核 → :1265 再次取**当前注册表** client 的 Token()（:1266）落库 UpdateIDToken——落库取的是复核后当前身份新 token，绝非 goroutine 发起时旧身份快照 | ✅ 在位（连续五轮零漂移） |
| B110-01 转正 | 观察项→契约 | 手动失败审计从「零留痕」转为「六分支全落库」（select :352/:364/:371 + exit :433/:442/:449），自动链（scheduler 7 处）与手动链（handler 6 处）失败审计完全对称 | ✅ 第十二轮保持并强化，转正为契约 |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（handler.go:377/:457）恒 nil 非吞错（函数体落库点全 if err 日志化） | ⚠️ 维持（第十一轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第十一轮零漂移） |
| 身份防线矩阵延续 | — | **第三十七轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十七轮延续，代理实证改动仅 handler.go +26 / handler_test.go +133 非产品逻辑）；sameClientFor 定义 :204 + 7 调用点零漂移；写点抽查换类 5 类（openTimeDetected/acctData/refused/reloginAt/reloginFail）全持锁 + 复核后写入；偶发写点专项四类零裸写；手动四路 accountExists + maybeRelogin 双侧 + 6 入口收口 | ✅ 闭合 |
| 新契约角度 | — | 限流与退避的可观测持久化：rateLimited 30s 风控退避（markRateLimitedLocked :1710 + AppendLog :1558 + /state status=failed 三通道）/ reloginAt 30s 重登节流（trigger 日志 + AppendLog + token_valid=false）/ probe 30s 节流闸门（纯测量节流无需日志）——两类业务性退避均 task_log + state 双通道可观测，管理员可核对完整退避时间线，无缺口 | ✅ 通过 |

### 前端（M-1 第五十八轮闭合 + 零新 OBSERVE）
- **M-1 第五十八轮闭合**：shouldDeferSave 四消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致 + echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位四边界（:229/:234/:247/:319）+ target-guard 18/18 实测全绿 + tsc -b 构建通过双实证。
- **OBSERVE-93-01 第十二轮家族册复核**：grep `<button` 17 处 3 文件零增零减 + 651/661 引擎二选一优先修复面维持。
- **F93-01 第三十轮**：Button.tsx:42 ring 逐字符在位 + git log/diff 1351fa4..HEAD -- web/ 双空 + 构建产物与 R121 逐字节同名双实证。
- **OBSERVE-116-01 跟踪项第六轮维持不修**：每秒 setNow 潜在优化注释如实口径在位 + 轮询链路契约（Select /state 2s/10s/30s、/electives 2s/30s；Dashboard /state /logs 3s/30s、/electives 30s）零漂移。
- **OBSERVE-115-01 弹窗族**：三处裸 div 弹窗最小语义门（role=dialog/aria-modal/aria-labelledby/Esc/autoFocus）逐处在位零漂移。
- **新契约角度（错误文案前端可辨性呈现）**：对照后端 R121 端到端可辨性五类结论走查前端——成功/满员/失效/窗口关闭四类差异化徽章/文案到位；「风控 vs 其他 failed」同归「报名异常」（后端单值 status="failed" 封顶该层表达力，result 字段可 `includes("风控")` 复用 isFullFallback 手法细分，纯 display 级可选优化非混同非缺陷）——维持观察不升级。

## 核实记录（关键）
- **OBSERVE-117-01 知识位第五轮主控独立复核**：主控 grep 实证 sameClientFor 计 7 调用点 + 定义 :204；COUNT=1；产品代码零吞错扫描全空——矩阵第三十七轮闭合成立。
- **B110-01 观察项转正**：代理实证 57bf401 改动仅 handler.go（+26 行）与 handler_test.go（+133 行），且自动链失败 7 处 AppendLog 与手动链 6 处语义完全对称——观察项在第十一轮由「盯守维持」转为「转正为契约」，审计链覆盖缺口闭合。
- **零吞错穷举扫描**：全仓零命中；非测试代码 `_ =` 残留 10 处逐一归类（handler 2 处恒 nil + config 基础设施 3 处 + Prewarm io 2 处 + ProbeForAccount 1 处 + io.Copy Discard 2 处）无一为业务落库吞错。
- **TDD 验证链**：后端 build/vet PASS + 全量 11 包 race 回归首轮即全绿（exit 0，零抖动残余，store 53.5s）；前端 npm run build EXIT 0 + 三守护脚本全绿（18/6/5 断言）。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十八轮）/ OBSERVE-111-01 + 112-03 盯守（第十二轮）/ **OBSERVE-117-01 知识位（第六轮）** / B110-01 审计链（第十三轮，转正契约后继续）/ B110-02 / O105-01 抖动基线 / task_log 审计保留；前端：M-1 延续管理（第五十九轮）/ OBSERVE-93-01 残余面（651/661 优先修复面）/ OBSERVE-116-01 双层重渲染跟踪项（第七轮）/ OBSERVE-115-01 弹窗族 / OBSERVE-117-02 timer 类型卫生 / **风控 vs failed 可辨性（display 级颗粒度，观察不升级）** / 其余家族册续。

## 教训
1. **观察项生命周期「盯守→转正」的完整周期实证**：B110-01 从 R110 首立（补库内审计日志的实质修复）→ R110-R121 连续十一轮「审计链盯守维持」→ R122 因「手动失败六分支全落库 + 自动/手动审计链完全对称」而**转正为契约**——观察项生命周期「新立→跟踪→修复/升级/转闭合」中的「升级」路径完整走通；转正的判据不是时间而是「缺口是否已被结构性闭合」。
2. **审查代理时间盒内定向验证 vs 主控收尾全量回归的分工**：R122 后端代理明确注明「未跑全仓 go test ./...（时间盒内定向核验），定向两包 + build/vet 已覆盖清单要求」——这恰恰是主控收尾必须跑全量 11 包 race 的原因：审查代理的职责是「聚焦清单的定向实证」，全量回归由主控在决策后收尾，两者分工互补不重叠。
3. **「风控 vs failed」可辨性颗粒度的归因链清晰化**：前端 CourseStatus 仅 status+result 双通道封顶表达力，后端单值 status="failed" 是根因——**跨端可辨性问题的归因要追到「数据通道的封顶形态」而不是前端渲染遗漏**；后端改多值（如 status="ratelimited"）才需前端配套，display 级 `includes()` 细分属可选优化不强行。与 R121「满员 vs 窗口关闭」核实同法：先问数据通道再判前端缺口。