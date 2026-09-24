# review-round144 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第五十九轮**闭合 + OBSERVE-117-01 知识位第二十七轮确认在位 + B110-01 审计链第三十四轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 3（维持）**（连续第八十一轮零严重级）。**双端零修复需求——连续第三十四轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第五十九轮闭合（登录闸门族终局/加密链/状态码家族纵深）+ 前端 M-1 第八十轮闭合（折叠键盘路径/401 吊销重建纵深）**。

## 审查发现（写入 archive/review-rounds/round144-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + 纵深族走查）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第五十九轮 | ✅ 闭合：sameClientFor 定义 :204 + 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）全部追到写状态/落库终局，每条都是「写前先同 client 指针复核，非同一身份即静默放弃整段」；maybeRelogin 双侧（:1208 / :1254 + :1265-1273）；手动五路 accountExists；写点换类 5 类全持锁 + warnedNoTargets（:1371-1372）宿主唯一性（submitAll 唯一调用点 = tick :1035）+ *Locked 写函数族 13 个 + 外部写函数 20 个首行取锁双向射证 |
| OBSERVE-117-01 知识位第二十七轮 | ✅ 在位：写回侧复核 → 二次 ClientFor 重取 token，同名重建绝不串旧身份 |
| B110-01 审计链第三十四轮 | ✅ 零漂移：手动 6 失败位 AppendLog 全在位；零吞错穷举 29 处 store 写调用全部 if err 挂接（`_ = d.Sched.MarkDone` 为内存态方法非落库点）；sanitizeError 脱敏 + maskedToken 7 子测判型穿透全绿 |
| O105-01 抖动基线 | ✅ 实测绿：socketPreheat/readyProbe 夹具四处全在位；四包定向 race 全绿（scheduler 15.1s / api 14.8s）+ 回归锚双测 count=3 复跑稳定 |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：:372/:377 已对齐钟；时间基全量扫零残留仅 reloginAt 与 gateWindow 两族本地钟写读同基自洽 |
| 新契约角度 ×2 | ✅ 登录闸门族 B42-01 双侧收口终局（gateTryAcquire/gateWait 共享单计数，任何直发 doLogin 入口只剩两路全被收口）+ 凭据 AES-256-GCM 加密链 + 状态码家族复盘（panic 500 / 401 / 403 CSRF 三处 / 429 / 404 / 持久化 500 五大家族全真实状态码） |
| 观察维持 | ⚠️ RemoveFull 零调用方（外部预留）、accounts 三夹具无 socketPreheat、classFullRealtime 防御性路径、maybePrewarm 无单测、probeSem cap=4 |

### 前端（M-1 第八十轮闭合 + OBSERVE 3 维持）
- **M-1 第八十轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 三段判据逐字核对 + 恰 4 调用点（Select.tsx:509/:597/:605/:699）三参数逐字符一致；echoedRef 写点恰 3（:200=false / :240 / :297 true）无第四处，echoDone 状态与防抖依赖 :755 构成自愈链；首帧四边界（:229/:234/:247/:319）逐行核对；cleanStaleSelected 原引用返回 + F43-M1 清空分判 + 三闸双闸齐备；target-guard 18/18 全绿实测。
- **OBSERVE-93-01 第三十四轮**：`<button` 全仓 17 处（Admin 13/Dashboard 2/Login 2）零增减；651/661 候选行号吻合。
- **F93-01 第五十二轮**：`git log/diff 6179674..HEAD -- web/` 双空实证成立（主控复现）；产物 index-27qti0_B.js 420.90 kB 与 R143 同尺寸同哈希直接佐证 web/ 零变更。
- **OBSERVE-116-01 第二十八轮**：注释口径统一（useTickingCountdown.ts:3-7 与 Dashboard 三处同源）；五路轮询契约逐键零漂移（error 与 window_closed 双降 30000）。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号（Select:1204 / Login:226 / Admin:213）与 R143 一致（Esc + aria-modal + autoFocus 齐备）。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持不实现（缓解因子无回归）。
- **新契约角度 ×2**：Dashboard 折叠键盘路径（CollapseSection 手写 aria-expanded/aria-controls + useId + extrasOpen/expandedDates null 一空语义分离绝不重种）+ 401 吊销整体重建（App 双挂载点 key + Select:195-202 四态复位）+ 管理令牌五路径成对全清（login/logout/onDeleted/onUnauthorized/onBackToStudent 均 setAdminToken+saveAdminToken 成对，isCurrentAdminSession 纯函数同源）均零偏离。
- **OBSERVE 维持 3**：Select.tsx:204-207 注释口径残留（R124 起延续）、Admin 五 Tab 轮询带宽（观察级）、perf 守卫弱断言。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 四包定向 race（借 mingw64 gcc） | 全绿（scheduler 15.1s / api 14.8s 最重两包） |
| 身份防线族十测 + 回归锚双测（count=3） | 全绿 |
| 迁移规范双测 | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（产物 420.90 kB JS 与 R143 同哈希） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5 = 32 断言） |
| XSS 面 dangerouslySetInnerHTML | 零命中 |
| 零吞错穷举 | 零命中（29 处 store 写调用全 if err 挂接） |
| 契约20轮次标签扫描 | 零命中（产品代码） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round144-backend-findings.md`（15806 字节 / 117 行）
- 前端 findings：`archive/review-rounds/round144-frontend-findings.md`（15009 字节）
- 收尾 commit：`docs(review): R144 双 findings + 收尾总结`（进度 145/256）