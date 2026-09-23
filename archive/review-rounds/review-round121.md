# review-round121 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（OBSERVE-111-01/112-03 盯守第十轮 + OBSERVE-117-01 知识位第四轮确认在位）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续五十八轮零严重级）。**双端零修复需求——连续第十一轮纯观察轮**。核心产出：**身份防线矩阵第三十六轮闭合（零产品改动链第十六轮延续）+ OBSERVE-117-01 知识位第四轮确认在位 + 错误文案端到端可辨性对照 + 前端 M-1 第五十七轮闭合 + OBSERVE-116-01 跟踪项第五轮维持**。

## 审查发现（写入 archive/review-rounds/round121-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 盯守第十轮 + 知识位第四轮）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 知识位第四轮 | maybeRelogin 写回侧（scheduler.go:1263-1274）成功分支先清内存标记再取**当前注册表** client → client.Token() → UpdateIDToken 落库新 token 恒为重登后新值；支撑链持续成立（ReloginIfNeeded→Login 更新实例 token client.go:392-394→UpdateIDToken 按主键写新值 store.go:56-59） | ✅ 在位（活化条件未触发，连续四轮零漂移） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（handler.go:377/:457）恒 nil 非吞错（:100 的 `_ =` 仅测试内） | ⚠️ 维持（第十轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第十轮零漂移） |
| 身份防线矩阵延续 | — | **第三十六轮闭合**：`git log 20c5882..HEAD -- backend/` COUNT=1（57bf401 审计修复，零产品改动链第十六轮延续）；sameClientFor 定义 :204 + 7 调用点零漂移；写点全家福抽查 5 类（tokenValid/inflight/done/full/rateLimited）持锁 + 复核后写入；偶发写点专项四类零裸写；手动四路 accountExists + maybeRelogin 双侧 + 6 入口收口 | ✅ 闭合 |
| 新契约角度 | — | 错误文案端到端可辨性对照：风控/窗口关闭/满员/read/失效五类从 zhidao 层产生 → scheduler/api 归并 → task_log 落库 → 前端回显（Dashboard 徽章 + c.result）全链语义清点通过，满员前后端同源判据对齐，无新缺陷 | ✅ 通过 |

### 前端（M-1 第五十七轮闭合 + 零新 OBSERVE）
- **M-1 第五十七轮闭合**：shouldDeferSave 四消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致 + echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位四边界（:229/:234/:247/:319）+ target-guard 18/18 实测全绿 + tsc -b 构建通过双实证。
- **OBSERVE-93-01 第十一轮家族册复核**：grep `<button` 17 处 3 文件零增零减 + 651/661 引擎二选一优先修复面维持。
- **F93-01 第二十九轮**：Button.tsx:42 ring 逐字符在位 + git log/diff 1351fa4..HEAD -- web/ 双空。
- **OBSERVE-116-01 跟踪项第五轮维持不修**：每秒 setNow 潜在优化注释如实口径在位（「不再声称局部渲染」）+ 轮询链路契约（/state /logs 3s/30s、Select /electives 2s/10s/30s）零漂移。
- **OBSERVE-115-01 弹窗族**：三处裸 div 弹窗最小语义门（role=dialog/aria-modal/aria-labelledby/Esc/autoFocus）逐处在位零漂移。
- **新契约角度（目标保存链异常路径自愈纵深）**：F40-M1（发布重建随建随清 + 守卫命中 toast 双解锁）/ F42-M1（stateData 到达触发防抖 effect 重跑自愈）/ F43-M1（selectedCount>0 清空语义分判）三契约在防抖/flush/handleBack 三条路径全部合拢，守卫命中均有自愈出口（数据到达/发布恢复/用户改动重试/5s 兜底），无静默死锁路径。

## 核实记录（关键）
- **OBSERVE-117-01 知识位第四轮主控独立复核**：主控 grep 实证 sameClientFor 计 7 调用点 + 定义 :204；COUNT=1；手动 AppendLog 产品代码零吞错——身份防线矩阵第三十六轮闭合成立。
- **身份防线矩阵第三十六轮**：主控 `git log 20c5882..HEAD -- backend/` 仅 57bf401 一条（零产品改动链第十六轮延续）；grep 实证 7 调用点零漂移。
- **零吞错穷举扫描**：`_ = AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|UpdateIDToken|DeleteRefused`（产品代码）零命中——契约 17 持续成立。
- **TDD 验证链**：后端 build/vet PASS + 全量 11 包 race 回归首轮即全绿（exit 0，零抖动残余）；前端 npm run build EXIT 0 + 三守护脚本全绿（18/6/5 断言）+ 产物与上轮逐字节一致。

## 观察项延续（下轮复核）
后端：身份防线矩阵（第三十七轮）/ OBSERVE-111-01 + 112-03 盯守（第十一轮）/ **OBSERVE-117-01 知识位（第五轮）** / B110-01 审计链（第十二轮）/ B110-02 / O105-01 抖动基线 / task_log 审计保留；前端：M-1 延续管理（第五十八轮）/ OBSERVE-93-01 残余面（651/661 优先修复面）/ OBSERVE-116-01 双层重渲染跟踪项（第六轮）/ OBSERVE-115-01 弹窗族 / OBSERVE-117-02 timer 类型卫生 / 其余家族册续。

## 教训
1. **环境中断频发期的补位策略进入第二轮实操**：R121 前后端原代理均在报告落盘前被终止（前端 2.5min/后端 7.5min），前后各派一枚补位代理成功完成——**补位代理携带「完整聚焦清单 + 前代理最后输出提示（如契约 20 扫描要排除二进制）+ 提速要求（能 grep 不全文读）」时，收尾效率远高于重派全新代理**（补位 21~37 次工具调用即完成 vs 全新代理完整下证的 150+ 次）。
2. **新契约角度「错误文案端到端可辨性对照」与 R118 路线互补**：R118 是日志侧文案语义单点对照，R121 是五类错误全链（zhidao 产生→归并→task_log→前端回显）的可辨性清点表——**「留痕→可追踪→两端可辨」审计完备性第三维在五个错误类别上逐格填满**，满员前后端同源判据（Dashboard isFullFallback 与后端 markFullLocked 文案）是跨端一致性的关键对齐点。
3. **前端保存链自愈纵深的「五道防线+四个出口」结构**：flush 链五道守卫（shouldDeferSave/发布缺席/stale/build 联查空/漂移 id）串行命中置脏跳过，但每道都有自愈出口（数据到达重跑/发布恢复/用户重试/5s 兜底）——**守卫的价值不在拦住而在于拦住后必有出口**，F40-M1「带解锁路径」契约的延伸认知。