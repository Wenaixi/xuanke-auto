# review-round137 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第五十二轮**闭合 + OBSERVE-117-01 知识位第二十轮确认在位 + B110-01 审计链第二十七轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 零新增 OBSERVE**（连续第七十四轮零严重级）。**双端零修复需求——连续第二十七轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第五十二轮闭合（无锁写点宿主唯一性射证换类延续）+ 手动 4 方法协同族/激活码票据/删号四序纵深 + 前端 M-1 第七十三轮闭合 + 管理后台会话态流转/复制降级链纵深**。

## 审查发现（写入 archive/review-rounds/round137-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + 纵深族走查）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第五十二轮 | ✅ 闭合：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一确认零漂移，每条追到写状态/落库终局（成功 :1521→done+SaveSuccess / 风控 :1551→markRateLimitedLocked / 窗口关闭 :1571→markFullLocked / 实时复核三路 :1600/:1635 / 失效 :1489→maybeRelogin 落在复核之后）；写点换类 5 类全持锁；*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证；**无锁写点换类 warnedNoTargets（:1371-1372）宿主 goroutine 唯一性证明**；手动五路 accountExists + maybeRelogin 双侧 |
| OBSERVE-117-01 知识位第二十轮 | ✅ 在位：写回侧 :1254 复核 → :1265-1273 二次 ClientFor 重取当前 token 落库（恒取注册表重登后新 token，不串旧身份） |
| B110-01 审计链第二十七轮 | ✅ 零漂移：手动 6 失败位 AppendLog 全部显式 err 分支；零吞错穷举仅 5 处非库写忽略（合规）；token 脱敏 sanitizeError/maskedToken 延续 |
| O105-01 抖动基线 | ✅ 实测绿：借 mingw64 四包定向 race 全绿（zhidao 2.4s / accounts 1.7s / scheduler 15.2s / api 21.4s）+ 身份防线族十测（3.5s）+ 双回归锚全绿 |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：git show 白线核对（975dc2b 两行、d75f38c 四处判读侧全对齐钟）；时间基残留仅 reloginAt 与 gateWindow 两处读写同基自洽合规 |
| 新契约角度 ×2 | ✅ 手动 4 方法协同族（TryAcquireSubmit/MarkDone/RemoveDone/RemoveFull 竞态与胜利状态闭环）+ 激活票据 ConsumeTicket 先于校验 + 删号 memory-first 四序（Remove→PurgeAccount→DeleteAccount→RevokeAccount）均契约在位 |
| 观察维持 | ⚠️ maybePrewarm 无独立单测、probeSem cap=4 常驻、Prewarm 错误静默、reloginAt 本地钟（自洽写读） |

### 前端（M-1 第七十三轮闭合 + 零新 OBSERVE）
- **M-1 第七十三轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71（判据本体 :69-71）+ 恰 4 消费点（Select.tsx:509/:597/:605/:699）三参形态逐字符一致；echoedRef 三置位 + 首帧四边界（:229/:234/:247/:319）行号与 R136 一致；cleanStaleSelected 原引用返回 / hasSelected 清空分判在位；target-guard 18/18 全绿实测。
- **OBSERVE-93-01 第二十七轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第四十五轮**：`git log/diff 0cf543a..HEAD -- web/` 双空实证成立（主控复现）。
- **OBSERVE-116-01 第二十一轮**：注释口径统一；五路轮询契约（Select /electives 2000/10000 + /state 2000，Dashboard /state 3000 + /logs 3000 + /electives 30000，error 与 window_closed 降 30000）逐键零漂移。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号与 R136 完全一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持成立（/state 2s 轮询边际成本吸收），记录不实现。
- **新契约角度 ×2**：管理后台会话态流转全路径（isCurrentAdminSession 双绑定判定 + adminToken 标记写入单点 + 清除五路径成对全清：logout/迁移 effect/onDeleted/onUnauthorized/onBackToStudent）+ 激活码复制降级链（clipboard→execCommand→完整码展示兜底，反馈定时器无泄漏）+ 生成/单码删除在飞守卫三钳复核均无缺口。
- **观察维持**：Select.tsx:204-207 注释口径残留（R124 起延续）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包定向 race（借 mingw64 gcc） | 全绿（scheduler 15.2s / api 21.4s 最重全 ok） |
| 身份防线族十测 | 全绿（3.5s） |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，1.65s） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| XSS 面 dangerouslySetInnerHTML 全变体 | 零命中 |
| 零吞错穷举 | 零命中（测试外仅 5 处非库写合规忽略） |
| 契约20轮次标签扫描 | 零命中（产品代码） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round137-backend-findings.md`（18507 字节 / 126 行）
- 前端 findings：`archive/review-rounds/round137-frontend-findings.md`（19760 字节）
- 收尾 commit：`docs(review): R137 双 findings + 收尾总结`（进度 138/256）