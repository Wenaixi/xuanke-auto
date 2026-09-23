# review-round124 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（OBSERVE-117-01 知识位第七轮确认在位 + OBSERVE-111-01/112-03 盯守第十三轮 + B110-01 审计链第十四轮零漂移 + O105-01 抖动基线实测绿）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续六十一轮零严重级）。**双端零修复需求——连续第十四轮纯观察轮**。核心产出：**身份防线矩阵第三十九轮闭合 + OBSERVE-117-01 知识位第七轮 + 多账号并发调度一致性新角度 + 前端 M-1 第六十轮闭合 + 手动操作在飞守卫族全路径新角度**。

## 时序叠加说明（审查后主控操作，非审查产物）
R124 报告落盘后、归档前，主控叠加了两项性能优化（独立 commit、可回退）：`f0f9bfc`（web 倒计时 memo 叶子，P-1）与 `c4a0b36`（api writeJSON struct，P-2）。另 `filter-repo` 重写历史致旧哈希（20c5882/57bf401 等）失效，基线核验改为语义重定位（B110-01 审计 commit 新哈希 `3ed1689`，确认为 HEAD 祖先）。R124 报告的「count=1 零产品改动」结论仅在审查时刻成立；叠加后 backend 含 c4a0b36、web 含 f0f9bfc，二者均非审查产物、均可 `git revert`。

## 审查发现（写入 archive/review-rounds/round124-{backend,frontend}-findings.md）

### 后端（零缺陷轮，OBSERVE 盯守第十三轮 + 知识位第七轮）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-117-01 | OBSERVE 知识位第七轮 | maybeRelogin 写回侧（scheduler.go:1254-1274）先 ClientFor 复核 → 重取**当前注册表** client 的 Token() 落库——token 来源是「写入时刻的注册表当前身份」，已删账号整体退出，绝不串旧身份 | ✅ 在位（连续七轮零漂移） |
| OBSERVE-111-01 | OBSERVE 延续 | MarkDone/RemoveDone `_ =`（handler.go:377/:457）恒 nil 非吞错（错误在函数内部记日志） | ⚠️ 维持（第十三轮零漂移） |
| OBSERVE-112-03 | OBSERVE 延续 | 手动失效分支与自动链同文案双日志——动作维度 select/exit 可区分 | ⚠️ 维持（第十三轮零漂移） |
| 身份防线矩阵延续 | — | **第三十九轮闭合**：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移；写点抽查换类 5 类（acctDataAt/openTimeDetected/refused/reloginAt/tokenValid）全持锁 + 复核后写入；偶发写点专项四类零裸写；手动四路 accountExists（handler.go:246/:295/:388/:500 + 设目标 :559）+ maybeRelogin 双侧（决策侧 :1208 + 写回侧 :1254） | ✅ 闭合 |
| B110-01 审计链 | — | **第十四轮零漂移**：手动失败六处 AppendLog（select :352/:364/:371 + exit :433/:442/:449）+ 成功路径审计行 + 自动链失败族齐位 + 零吞错穷举扫描（`grep _ = .*(AppendLog|Save*|Delete*|UpdateIDToken)`）全仓零命中 | ✅ 零漂移 |
| O105-01 抖动基线 | — | 夹具 socketPreheat/readyProbe 零漂移 + 定向 race（zhidao/accounts 双包）全绿 | ✅ 实测绿 |
| 新契约角度 | — | 多账号并发调度一致性：探测 per-account 独立刷新 + probeSem(cap4) 信号量 + lastProbe 全校节流闸门；提交每 tick 每账号每发布各一条链 + 全局 lastSubmit（对齐钟）公平时间片——无某账号霸占提交带宽；确定性边界（账号>50 时 probeSem cap 需上调，scheduler.go:1068 ponytail 注释已标注）已知非本轮触发 | ✅ 通过 |

### 前端（M-1 第六十轮闭合 + 零新 OBSERVE）
- **M-1 第六十轮闭合**：shouldDeferSave 定义 targetGuard.ts:64 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致 + echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位四边界 + target-guard 18/18 实测全绿 + tsc -b 构建通过双实证。
- **OBSERVE-93-01 第十四轮家族册复核**：grep `<button` 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减 + 651/661 引擎二选一优先修复面维持。
- **F93-01 第三十二轮**：Button.tsx:42 ring 逐字符在位 + git log/diff 1351fa4..HEAD -- web/ 双空实证。
- **OBSERVE-116-01 跟踪项第八轮维持不修**：每秒 setNow 注释如实口径在位 + 轮询链路契约零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门（role=dialog/aria-modal/aria-labelledby/Esc 在飞守卫/autoFocus 安全默认项）逐处在位零漂移。
- **OBSERVE-117-02 timer 类型卫生**：RetryState `ReturnType<typeof setTimeout>` 类型配对 + 卸载 cleanup + StrictMode 复位注释在位，构建全绿非缺陷。
- **新契约角度（手动操作在飞守卫族全路径）**：actionLoading 用 ReadonlySet 按课程独立跟踪（单值互踩根因已消）+ 入口短路双处（:92/:119）堵住 disabled 渲染落地前双击双发 + finally 函数式清除（:108/:133，只删自己 id 绝不抹他课）+ 弹窗关闭双闸（取消 disabled + Esc 在飞不响应）——**无「置位后永不复位」卡死路径**，TryAcquireSubmit 冲突边界为同一防线族两道闸无遗漏。

## 验证表

| 验证项 | 结果 |
|--------|------|
| 后端全量 11 包 `go test ./...` | 全绿（api 含 P-2 struct 序列化等价性测试） |
| 前端 `npm run build`（tsc -b + vite） | 通过（474ms，产物 index-27qti0_B.js 420.90 kB） |
| perf-countdown-guard / target-guard / admin-auth / unauthorized 四守卫 | 全绿 |
| 契约 20 轮次标签扫描 | 零命中 |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round124-backend-findings.md`
- 前端 findings：`archive/review-rounds/round124-frontend-findings.md`
- 收尾 commit：`docs(review): R124 双 findings + 收尾总结`（进度 125/256）
