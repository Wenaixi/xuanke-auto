# review-round139 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第五十四轮**闭合 + OBSERVE-117-01 知识位第二十二轮确认在位 + B110-01 审计链第二十九轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 3（维持）**（连续第七十六轮零严重级）。**双端零修复需求——连续第二十九轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第五十四轮闭合（窗口三判据/登录时序攻击族纵深）+ 前端 M-1 第七十五轮闭合 + 激活码生命周期/checkResp 状态码机纵深**。

## 审查发现（写入 archive/review-rounds/round139-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + 纵深族走查）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第五十四轮 | ✅ 闭合：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一确认零漂移，每条追到写状态/落库终局；maybeRelogin 双侧（:1208 决策侧前置复核 / :1254 写回侧 + :1265-1273 二次 ClientFor）；手动五路 accountExists；写点换类 5 类全持锁 + *Locked 写函数族 13 个 + 外部写函数 8 个双向射证 + warnedNoTargets 宿主唯一性（tick→Start 主循环） |
| OBSERVE-117-01 知识位第二十二轮 | ✅ 在位：落库 token 恒为重登完成后注册表内当前客户端实值，同名重建绝不串旧身份 |
| B110-01 审计链第二十九轮 | ✅ 零漂移：手动 6 失败位 + 成功审计行 + 自动链失败族全在；零吞错穷举零命中（落库族 20 处调用全包裹 if err，`_ =` 仅 8 处非库写防御性忽略）；token 脱敏延续 |
| O105-01 抖动基线 | ✅ 实测绿：借 mingw64 四包定向 race 全绿（zhidao 2.184s / accounts 1.509s / scheduler 15.106s / api 14.807s）+ 身份防线族十测 + 回归锚 + 时钟族 + 登录闸门族 + 手动审计族 + 基础设施状态码族 + 落库零吞错族全 PASS |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：git show 白线核对 975dc2b/d75f38c diff 逐字一致；时间基全量扫零残留仅 reloginAt 与 gateWindow 两处写读同基自洽 |
| 新契约角度 ×2 | ✅ 窗口状态三判据单源（windowClosedLocked :918-939 三判据 + 单次 open 快照对判据 2/3 统一 + probe 入账侧同纪律）+ 登录时序攻击族（B43-04 撞名双条件 + ConstantTimeCompare + loginTimingFlat 时延拉平 + gateTryAcquire 非阻塞准入共享 gateUsed 计数）契约全部在位 |
| 观察维持 | ⚠️ accounts 三夹具无 socketPreheat 既有缺口锁定待观察、classFullRealtime 防御性路径维持、maybePrewarm 无单测、probeSem cap=4、Prewarm 错误静默 |

### 前端（M-1 第七十五轮闭合 + OBSERVE 3 维持）
- **M-1 第七十五轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 与 R138 逐字符一致 + 恰 4 消费点（Select.tsx:509/:597/:605/:699，grep -c=4）；echoedRef 三置位（:200 复位=false / :240 空分支 / :297 合并完成）无第四处写 true；首帧四边界（:229/:234/:247/:319）在位；target-guard 18/18 全绿实测。
- **OBSERVE-93-01 第二十九轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第四十七轮**：`git log/diff 92f0f06..HEAD -- web/` 双空实证成立（主控复现）。
- **OBSERVE-116-01 第二十三轮**：注释口径统一（useTickingCountdown/Dashboard/Select 三处同源）；五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全数命中）。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号（Select:1204 / Login:226 / Admin:213）与 R138 一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持成立（2s 轮询吸收 + 整屏一体化），记录不实现。
- **新契约角度 ×2**：激活码生命周期四 closure（生成/复制/删除/消耗，在飞幂等守卫 + 票据回传 + 失败清票自愈全链路）+ client.ts checkResp 状态码机（401 防双发广播 + extractAccountFromPath 纯函数 + abort 统一映射 + 20s 超时兜底）逐行复核零偏离。
- **OBSERVE 新增 2（维持观察不立条）**：Admin refetch 出口（管理页刷新路径）、perf 守卫弱断言（perf-countdown-guard 断言强度评估）——与前轮 Select:850 候选同列待评估。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包定向 race（借 mingw64 gcc） | 全绿（scheduler 15.106s / api 14.807s 全 ok） |
| 身份防线族 + 回归锚 + 时钟族 + 闸门族 + 审计族 + 状态码族 | 全 PASS |
| 全后端非 race 13/13 包 | 全 ok |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，858ms） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| XSS 面 dangerouslySetInnerHTML | 零命中 |
| 零吞错穷举（`_ =` 双形式） | 零命中（落库族 20 处全包裹 if err，仅 8 处非库写防御性忽略） |
| 契约20轮次标签扫描 | 零命中（产品代码，唯一历史文档名引用合规） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round139-backend-findings.md`（23477 字节 / 137 行）
- 前端 findings：`archive/review-rounds/round139-frontend-findings.md`（15123 字节）
- 收尾 commit：`docs(review): R139 双 findings + 收尾总结`（进度 140/256）