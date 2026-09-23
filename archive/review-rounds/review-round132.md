# review-round132 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 1**（身份防线矩阵第四十七轮闭合 + OBSERVE-117-01 知识位第十五轮确认在位 + B110-01 审计链第二十二轮零漂移 + O105-01 实测绿 + **LOW-132-01 时钟失败退避时间基混用孤岛**）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第六十九轮零严重级）。**双端零 MAJOR 修复需求（连续第二十二轮），唯一 LOW 契约打磨已 TDD 修复**。核心产出：**身份防线矩阵第四十七轮闭合 + LOW-132-01 TDD 修复（时间基统一对齐钟）+ 登录闸门族/时间基一致性纵深 + 前端 M-1 第六十八轮闭合 + 在飞守卫/弹窗 Esc 交互全路径**。

## 审查发现（写入 archive/review-rounds/round132-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + LOW-132-01）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第四十七轮 | ✅ 闭合：sameClientFor :204（clientIdentity :215）+ 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）零漂移；写点换类 5 类（done :1529/full :1767/inflight :1471/:1490/:1501/:1513/rateLimited :1714/state.Courses）全持锁 + TryAcquireSubmit sync.Once 防双施放；*Locked 写函数族 + 外部写函数首行取锁双向射证延续；reloginResults 单 goroutine 消费射证；手动五路 accountExists + maybeRelogin 双侧 |
| OBSERVE-117-01 知识位第十五轮 | ✅ 在位：写回侧 :1254 复核 → :1265-1272 重取当前注册表 client.Token() 落库不串旧身份；锁序 reloginMu→s.mu 与 TokenValidFor/MarkTokenValid 对齐 |
| B110-01 审计链第二十二轮 | ✅ 零漂移：手动 6 失败位（:362/:374/:381/:443/:452/:459）+ 成功审计行 + 自动族；零吞错穷举零命中（唯一 `_ =` 为 Prewarm :307/ProbeForAccount :1075/MarkDone:RemoveDone 非库写合规） |
| O105-01 抖动基线 | ✅ 实测绿：夹具在位；借 mingw64 gcc 四组 race 全绿 + 全量 13 包非 race 全 ok + 回归锚双测在位 |
| **LOW-132-01** | ⚠️→✅ **修复**：`lastSyncFailAt` 写入侧 :372 用本地钟 `time.Now()`、判读侧 :342 用对齐钟 `now.Sub`——同一函数内时间基混用孤岛（偏差 ~640ms 对 30s 退避无实质影响，契约打磨级）。**TDD 修复**：TestClockSyncFailureBackoffUsesAlignedClock 先红（实测差 5.013s=clockOffset 混用量级）后绿；`:372` 改 `nowAlignedLocked()` + `:377` syncFailedWindow 一并统一；时钟族六测 + 回归锚三测 + scheduler/api 全量回归全绿 |
| 新契约角度（登录闸门族 + 时间基一致性） | ✅ gateWait/gateTryAcquire 共享 gateMu 预算严格收敛（B42-01 在位）；恢复顺序契约与 CLAUDE.md 契约 6 一致 |
| 观察维持 | ⚠️ maybePrewarm 无独立单测（R127 首提）、probeSem cap=4 常驻（R131） |

### 前端（M-1 第六十八轮闭合 + 零新 OBSERVE）
- **M-1 第六十八轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位（:200/:240/:297）+ 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第二十二轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第四十轮**：`git log/diff 073f7fd..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第十六轮**：注释口径统一；五路轮询契约逐行零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R131 一致；Esc 均在飞守卫。
- **R125 候选复核**：Select.tsx:844/:850 内联 cd.* 维持成立（2s 轮询吸收、守卫盲区无漂移），不立条不实现。
- **新契约角度**：在飞守卫 + 弹窗 Esc 交互全路径（actionLoading Set 独立、三路径单源同锁、函数式清除不互踩）+ 目标保存 PUT 防抖/flush 竞态时序（saveNow 3 处 + 补发出口唯一 + 终局 toast 不误报）均无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四组 race（借 mingw64 gcc） | 全绿 |
| scheduler/api 全量回归 | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，398ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| 契约20轮次标签扫描 | 零命中（产品 2 处历史归档引用合规） |
| 工作区 | 干净 |

## 归档
- 后端 findings：`archive/review-rounds/round132-backend-findings.md`
- 前端 findings：`archive/review-rounds/round132-frontend-findings.md`
- 修复 commit：`fix(scheduler): LOW-132-01 时钟失败退避时间基统一对齐钟`
- 收尾 commit：`docs(review): R132 双 findings + 收尾总结`（进度 133/256）