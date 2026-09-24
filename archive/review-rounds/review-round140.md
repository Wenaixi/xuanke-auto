# review-round140 总结（2026-09-24）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + LOW 0（无新增）**（身份防线矩阵**第五十五轮**闭合 + OBSERVE-117-01 知识位第二十三轮确认在位 + B110-01 审计链第三十轮零漂移 + O105-01 实测绿 + LOW-132/133 修复回首通过）；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + OBSERVE 3（维持）**（连续第七十七轮零严重级）。**双端零修复需求——连续第三十轮零 MAJOR，本轮纯观察**。核心产出：**身份防线矩阵第五十五轮闭合（凭据加密链/状态码家族纵深）+ 前端 M-1 第七十六轮闭合 + 管理令牌五路径/折叠键盘路径纵深**。

## 审查发现（写入 archive/review-rounds/round140-{backend,frontend}-findings.md）

### 后端（零缺陷轮 + 纵深族走查）
| 项 | 核实裁决 |
|---|---|
| 身份防线矩阵第五十五轮 | ✅ 闭合：sameClientFor 定义 :204-210（clientIdentity 反射指针身份比对）+ 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一追到写状态/落库终局（识别槽+专属快照 / inflight+失效态+重登触发 / done+success 库行 / 风控退避 / 永久满员 / 实时复核未授权 / 确证满员），全部在「回锁后、写内存与落库前」做身份复核；maybeRelogin 双侧（:1208 决策侧 + :1254 写回侧 + :1265-1273 锁内重取当前 Token 落库）；手动五路 accountExists；写点换类 5 类全持锁 + warnedNoTargets（:1371-1372）宿主唯一性（submitAll 唯一生产调用点 tick:1035）+ *Locked 写函数族 13 个 + 外部写函数首行取锁双向射证 |
| OBSERVE-117-01 知识位第二十三轮 | ✅ 在位：写回侧 ClientFor 复核 → 锁内二次重取当前 Token 落库，`.Token()` 为空才不落库，零串身份 |
| B110-01 审计链第三十轮 | ✅ 零漂移：手动 6 失败位全部 AppendLog + if err 记账；零吞错穷举全仓仅 8 处合规 `_ =`（全部非 store 落库）+ store 落库 29 调用点零 `_ =` 形态；sanitizeError/maskedToken 脱敏延续 |
| O105-01 抖动基线 | ✅ 实测绿：借 mingw64 十包定向 race 全绿（zhidao 1.271s / scheduler 15.081s / api 15.343s）+ 扩展六包（db/secure/session/runtime/config/store）+ 回归锚双测恒绿 |
| **LOW-132-01 / LOW-133-01 回首核** | ✅ 修复在位且绿：git show 白线逐字核对；时间基全量扫零残留仅 reloginAt 与 gateWindow 两处写读同基自洽 + 独立本地钟孤岛闭环 |
| 新契约角度 ×2 | ✅ 凭据 AES-256-GCM 加密链 + 会话级账号绑定（密钥源→encrypt 注入→password_enc/enc: 落库→读回严格 enc: 前缀拒绝旧明文→Restore 解密失败降级 → refresh 会话绑定 + ?account= 经 IsAdminToken + RevokeAccount 吊销，全链路零断点）+ writeJSONStatus 基础设施状态码家族复盘（鉴权 401 / 管理与 CSRF 403 / 限流 429 / panic 500 / 未知路由 404 五家族逐项在位，SPA 兜底先设头再写 body） |
| 观察维持 | ⚠️ accounts 三夹具无 socketPreheat、classFullRealtime 防御性路径、maybePrewarm 无单测、probeSem cap=4、Prewarm 错误静默 |

### 前端（M-1 第七十六轮闭合 + OBSERVE 3 维持）
- **M-1 第七十六轮闭合**：shouldDeferSave 定义 targetGuard.ts:64-71 三参判据逐字符一致 + 恰 4 消费点（Select.tsx:509/:597/:605/:699，grep -c=4）；echoedRef 三置位（:200=false 复位 / :240/:297=true）无第四处；首帧四边界（:229/:234/:247/:319）；cleanStaleSelected 原引用返回 / hasSelected 清空分判在位；target-guard 18/18 全绿实测。
- **OBSERVE-93-01 第三十轮**：`<button` 全仓 17 处 3 文件（Admin 13/Dashboard 2/Login 2）零增零减；651/661 候选维持。
- **F93-01 第四十八轮**：`git log/diff 16bb702..HEAD -- web/` 双空实证成立（主控复现）。
- **OBSERVE-116-01 第二十四轮**：注释口径统一（useTickingCountdown.ts:3-8 与 Dashboard 同源）；五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全数命中）。
- **OBSERVE-115-01 弹窗族**：三处最小语义门行号（Select:1204 / Login:226 / Admin:213）与 R139 一致。
- **R125 候选复核**：Select.tsx:850 内联 cd.* 维持不实现（整屏集成式布局拆 memo 收益低，缓解因子与守卫盲区零漂移）。
- **新契约角度 ×2**：管理令牌 xk_admin_token 五路径成对全清（logout / 账号迁移判据驱动退 / onDeleted / onUnauthorized / onBackToStudent 四条显式清 + 一条判据自动退，撞名学生永不误标）+ Dashboard 折叠键盘路径（CollapseSection useId + aria-expanded/aria-controls 多实例 id 唯一，关闭态面板不渲染读屏无隐藏内容）均零偏离。
- **交叉覆盖**：key={account} 挂载点（App:294/:340）与 F39-N1 倒计时兜底（Select:767-772 / Dashboard:222-227）同构无漂移。
- **OBSERVE 维持 3**：Select.tsx:204-207 注释口径残留（R124 起延续）、Admin refetch 出口候选、perf-countdown-guard 弱断言属性（正则结构匹配非渲染剖面）。

## 验证表
| 验证项 | 结果 |
|--------|------|
| 后端 go build/vet | 通过 |
| 十包定向 race（借 mingw64 gcc） | 全绿（scheduler 15.081s / api 15.343s 全 ok） |
| 身份防线族 + 回归锚双测 | 全绿 |
| 前端 npm run build（tsc -b + vite） | 通过（1948 modules，1.02s） |
| 四守卫（target/perf-countdown/admin-auth/unauthorized） | 全绿（18/3/6/5） |
| XSS 面 dangerouslySetInnerHTML | 零命中 |
| 零吞错穷举（`_ =` 双形式） | 零命中（仅 8 处非库写合规忽略，store 落库 29 调用点零形态） |
| 契约20轮次标签扫描 | 零命中（产品代码） |
| 工作区 | 干净（仅两份新 findings 未跟踪） |

## 归档
- 后端 findings：`archive/review-rounds/round140-backend-findings.md`（19342 字节 / 144 行）
- 前端 findings：`archive/review-rounds/round140-frontend-findings.md`（15948 字节）
- 收尾 commit：`docs(review): R140 双 findings + 收尾总结`（进度 141/256）