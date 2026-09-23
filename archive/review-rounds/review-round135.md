# review-round135 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第五十轮——里程碑轮**闭合 + OBSERVE-117-01 知识位第十八轮确认在位 + B110-01 审计链第二十五轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第七十二轮零严重级）。**双端零修复需求——连续第二十五轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第五十轮（里程碑轮）全家福闭合 + 重登写回侧最终安全网纵深 + 恢复链全序核对 + 前端 M-1 第七十一轮闭合 + F93-01 双空实证延续**。

## 审查发现（写入 archive/review-rounds/round135-{backend,frontend}-findings.md）

### 后端（零缺陷里程碑轮 + 全家福复盘）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第五十轮 | ✅ 闭合（里程碑全家福）：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一确认零漂移，六分支 + 探测回写全为网络往返后持锁写入前最后一道身份闸；写点换类 5 类（lastSubmit/lastSyncStart/syncing/lastProbe/EmptyProbeRuns）全持锁零裸写；*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证延续；无锁写点宿主 goroutine 唯一性射证延续；手动五路 accountExists + maybeRelogin 双侧 |
| OBSERVE-117-01 知识位第十八轮 | ✅ 在位：写回侧先 ClientFor 复核 :1254 → 重取当前注册表 client.Token() 落库 :1265-1273，同名重建场景绝不串旧身份 |
| B110-01 审计链第二十五轮 | ✅ 零漂移：手动 6 失败位 + 成功审计行 + 自动链失败族 + 零吞错穷举（`_ =` 双形式）零命中 + 网络层 token 脱敏延续抽查 |
| O105-01 抖动基线 | ✅ 实测绿：夹具在位；借 mingw64 gcc 六包 race 全绿 + 身份防线族十测 + 探测定时族 + 闸门族 + interval clamp 全绿 |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：git show 白线核对 + 时间基全量扫零残留（残余 time.Since 仅 reloginAt :1220/:1227 写读同基自洽；time.Now() 仅 nowAligned 定义与 reloginAt 写点） |
| 新契约角度 ×2 | ✅ 重登写回侧最终安全网（二次 ClientFor 重取当前 Token 落库，防火墙语义正确——同名重建下旧链绝不写新身份）+ 恢复链全序核对（main.go:117-141 RestoreDone → RestoreTargets → RestoreRefused 与决策契约 6 逐字对齐，RestoreTargets 绝不清 refused） |
| 观察维持 | ⚠️ maybePrewarm 无独立单测、probeSem cap=4 常驻、Prewarm 错误静默、reloginAt 本地钟时间基（自洽写读） |

### 前端（M-1 第七十一轮闭合 + 零新 OBSERVE）
- **M-1 第七十一轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 + 恰 4 消费点（Select.tsx:509/:597/:605/:699）逐字符一致；echoedRef 三置位 + 首帧四边界全在位；target-guard 18/18 实测全绿。
- **OBSERVE-93-01 第二十五轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第四十三轮**：`git log/diff 37be8b2..HEAD -- web/` 双空实证成立。
- **OBSERVE-116-01 第十九轮**：注释口径统一；五路轮询契约逐行零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R134 完全一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持成立（缓解因子 + 守卫盲区复核通过），记录不实现。
- **新契约角度**：react-query 缓存共享复核（多 key 独立无跨账号串）+ Toast 语义复核 + **XSS/注入面复核（dangerouslySetInnerHTML 全仓零命中）** 均无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 六包 race + 身份防线族十测（借 mingw64 gcc） | 全绿（scheduler 15.209s 最重全 ok） |
| 探测定时族 + 闸门族 + interval clamp | 全绿 |
| 时钟族 + 迁移族 | 全绿（含 TestMigrateAddsPublishMetaColumns） |
| 前端 npm run build（tsc -b + vite） | 通过 |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿 |
| 契约20轮次标签扫描 | 零命中（产品代码，仅测试叙述三处 + 文档引用两处合规） |
| 零吞错穷举（`_ =` 双形式） | 零命中（唯一 4 处非库写合规静默） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round135-backend-findings.md`（16039 字节 / 84 行）
- 前端 findings：`archive/review-rounds/round135-frontend-findings.md`
- 收尾 commit：`docs(review): R135 双 findings + 收尾总结`（进度 136/256）
