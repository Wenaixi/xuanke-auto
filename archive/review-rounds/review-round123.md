# review-round123 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（OBSERVE-111-01/112-03 盯守第十二轮 + OBSERVE-117-01 知识位第六轮确认在位）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续六十轮零严重级）。**双端零修复需求——连续第十三轮纯观察轮**。核心产出：**身份防线矩阵第三十八轮闭合（零产品改动链第十八轮延续）+ OBSERVE-117-01 知识位第六轮确认在位 + 调度状态机异常自愈会话走查 + 前端 M-1 第五十九轮闭合 + 轮询降频/升频状态机走查**。

## 审查发现（写入 archive/review-rounds/round123-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 盯守第十二轮 + 知识位第六轮）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 知识位第六轮 | maybeRelogin 写回侧（scheduler.go:1254-1274）先 ClientFor 复核 → 重取**当前注册表** client 的 Token() 落库——token 来源是「写入时刻的注册表当前身份」，已删账号整体退出，绝不串旧身份 | ✅ 在位（连续六轮零漂移） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（handler.go:377/:457）恒 nil 非吞错（错误在函数内部记日志 :1946/:1974/:2021/:2027） | ⚠️ 维持（第十二轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第十二轮零漂移） |
| 身份防线矩阵延续 | — | **第三十八轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十八轮延续，代理实证「本轮唯一 backend 变更」57bf401 未触碰任何身份防线代码）；sameClientFor 定义 :204 + 7 调用点零漂移；写点抽查换类 5 类（done/rateLimited/full/inflight/state.Courses）全持锁 + 复核后写入；偶发写点专项四类零裸写；手动四路 accountExists + maybeRelogin 双侧 + 6 入口收口 | ✅ 闭合 |
| 新契约角度 | — | 调度状态机异常自愈会话：rateLimited（入账 markRateLimitedLocked :1710 → 等待 isRateLimitedLocked :1691 超时自愈 → MarkDone/RemoveDone 显式清理）/ syncFailStreak（++ :371 → ≥3 复位 clockOffset=0 绝不在此清零 streak :384-386 → 同步成功统一清零 :391 + 判定侧带「开放时间已过」:925）/ EmptyProbeRuns（++ 含 +10s 裕量 :1155 → 非空快照归零 :1158 → 判定侧判据 3 :935 + 单次 open 快照复用 :924）——三机制均有「入账→等待→自愈/复位」完整生命周期且状态单一来源 | ✅ 通过 |

### 前端（M-1 第五十九轮闭合 + 零新 OBSERVE）
- **M-1 第五十九轮闭合**：shouldDeferSave 四消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致 + echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位四边界（:229/:234/:247/:319）+ target-guard 18/18 实测全绿 + tsc -b 构建通过双实证。
- **OBSERVE-93-01 第十三轮家族册复核**：grep `<button` 17 处 3 文件零增零减（行号与上轮完全一致）+ 651/661 引擎二选一优先修复面维持。
- **F93-01 第三十一轮**：Button.tsx:42 ring 逐字符在位 + git log/diff 1351fa4..HEAD -- web/ 双空 + 构建产物与 R121/R122 逐字节同名双实证延续。
- **OBSERVE-116-01 跟踪项第七轮维持不修**：每秒 setNow 潜在优化注释如实口径在位 + 轮询链路契约（Select /state 2s/10s/30s、/electives 2s/30s；Dashboard /state /logs 3s/30s、/electives 30s）零漂移。
- **OBSERVE-115-01 弹窗族**：三处裸 div 弹窗最小语义门（role=dialog/aria-modal/aria-labelledby/Esc/autoFocus）逐处在位零漂移。
- **新契约角度（轮询降频/升频状态机）**：前端 refetchInterval 分支覆盖后端 windowClosedLocked 全部三判据（主判据/时钟连败/幽灵窗口 + 查询失败），降频均有解除路径（react-query 成功重排 / syncFailStreak 归零 / EmptyProbeRuns 归零 / 新一轮开窗）——**无「降频后永远升不回来」、无「升频后无法降频」单向卡死**；仅 /state 失败期间 Select /electives 读旧缓存 window_closed 落入 10s 的轻量中间态（非缺陷，维持观察不升级）。

## 核实记录（关键）
- **OBSERVE-117-01 知识位第六轮主控独立复核**：主控 grep 实证 sameClientFor 计 7 调用点 + 定义 :204；COUNT=1；产品代码零吞错扫描全空——矩阵第三十八轮闭合成立。
- **B110-01 审计链第十三轮（转正契约后）**：手动失败六处 AppendLog + 两条 TDD 测试（TestManualElectiveFailureAppendsLog / TestManualElectiveReadErrAppendsLog）+ 成功路径审计行 + 自动链失败族齐位。
- **零吞错穷举扫描**：全仓零命中；仅存的 `_ =` 是 MarkDone/RemoveDone 两个不返回落库错误的封装调用（错误已内记日志），符合契约 17 语义。
- **TDD 验证链**：后端 build/vet PASS + 全量 11 包 race 回归首轮即全绿（exit 0，零抖动残余）；前端 npm run build EXIT 0 + 三守护脚本全绿（18/6/5 断言）+ 产物逐字节一致。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十九轮）/ OBSERVE-111-01 + 112-03 盯守（第十三轮）/ **OBSERVE-117-01 知识位（第七轮）** / B110-01 审计链（第十四轮）/ B110-02 / O105-01 抖动基线 / task_log 审计保留；前端：M-1 延续管理（第六十轮）/ OBSERVE-93-01 残余面（651/661 优先修复面）/ OBSERVE-116-01 双层重渲染跟踪项（第八轮）/ OBSERVE-115-01 弹窗族 / OBSERVE-117-02 timer 类型卫生 / 风控可辨性 / /state 失败 10s 中间态 / 其余家族册续。

## 教训
1. **调度状态机的「入账→等待→自愈/复位」生命周期是稳定期纵向走查的可靠角度**：rateLimited/syncFailStreak/EmptyProbeRuns 三机制逐一验证「入账点持锁 + 等待判据单源 + 自愈/复位路径」——`syncFailStreak` 的「复位只清 offset、streak 保留至同步成功」是契约 18 的自愈语义，`EmptyProbeRuns` 的非空快照即归零 + 单次 open 快照复用是防误挂黄金期的关键设计——三机制零旁路写入。
2. **前端轮询状态机的「双向可逆性」验证是新角度**：降频/升频双向风险（降了升不回 / 升了降不下）逐一对照后端状态转移——前端 refetchInterval 分支覆盖后端 windowClosedLocked 三判据全部转移，react-query 函数式回调「成功重排间隔」是解除路径的枢纽——**轮询节奏不是静态常量而是状态机，验证要追「每个降频分支的解除路径」**。
3. **轻量中间态观察项的积累形态**：R122 风控可辨性颗粒度 + R123 /state 失败 10s 中间态——均「不构成缺陷、维持观察不升级」，与「宁缺毋滥不堆不丢」原则一致；稳定期审查产出 = 每轮一个新角度的纵深走查 + 轻量观察项累积，均不升级为缺陷。