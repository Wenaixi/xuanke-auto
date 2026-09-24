# R142 前端审查报告（M-1）——第七十九次核验

基线：2ca7e0c（R141 归档） | 审查时间：2026-09-24 | 审查代理：R142 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。三条维持观察项依旧维持。建议 **APPROVE**。

| 级别 | 数量 | 说明 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| MINOR | 0 | — |
| OBSERVE | 3 | 维持观察（Select 注释口径残留 / Admin refetch 出口 / perf 守卫弱断言），均无回归 |

## 验证表（实测数据）

| 验证项 | 命令 | 结果 | 实测时间 |
|--------|------|------|----------|
| 构建 | `npm run build`（tsc -b + vite build） | 通过，vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB | 2026-09-24 |
| target 守卫 | `npx tsx scripts/target-guard-check.ts` | 18 断言全绿（输出末行"target-guard 断言全绿"） | 2026-09-24 |
| perf 守卫 | `npx tsx scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 消费） | 2026-09-24 |
| admin-auth 守卫 | `npx tsx scripts/admin-auth-check.ts` | 6 断言全绿（含撞名学生/吊销/无标记/自定义管理员名） | 2026-09-24 |
| unauthorized 守卫 | `npx tsx scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中 | 2026-09-24 |
| 工作区 | `git status --porcelain` + `git diff --stat HEAD` | 零改动零未跟踪文件（本报告写入前） | 2026-09-24 |
| git 双空 | `git log --oneline 2ca7e0c..HEAD -- web/` + `git diff --stat 2ca7e0c..HEAD -- web/` | 双空（无 web/ 提交、无 web/ 差异） | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 第七十九次闭合（保存链守卫族） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

三段判据与 CLAUDE.md 契约逐行一致，注释忠实记录了 F43-M1 全清空分判动机（"显式清空绝不与慢首帧混判"）。

**恰 4 消费点**（grep 实测，if 级调用为 4，含 import/注释全文 9 行命中）：
- Select.tsx:509（flushTargets）`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门）`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

三参数逐字符一致（数据源均为 ref、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）。四消费点语义分属防抖/flush/handleBack 双闸、无遗漏第五处。

**echoedRef 三置位**（grep 全文 27 处引用中写点恰 4 处）：
- :200 `= false` — 账号切换复位守卫（accountKey !== account 分支），绝不写 true
- :240 `= true` — 首帧 courses 空（确证后端无旧目标）
- :297 `= true` — 首帧 courses 非空合并完成

确证无第四处写 true：发布重建清理独立 effect（:318-329）只读不写 echoedRef，与既有契约一致（已回显账号的发布重建后 stale 由独立 effect 随 publishes 变化清理，不靠重放回显 effect）。

**首帧四边界**：:229（echoed 短路）、:234（stateData undefined 提前 return，绝不提前置位）、:247（pubs 空 return）、:319（清理 effect 未回显短路）四处全部在位。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，原引用返回契约实测确认（:42 `return changed ? next : selected`——无变更返回 `selected` 原引用），守卫脚本第 18 断言"无变更 → 返回原引用"绿。

**F43-M1 hasSelected 清空分判**：守卫脚本 6 条 shouldDeferSave 用例全部绿，其中"courses 非空 + 全清空 → 放行"与"courses 非空 + 有选中 + 未回显 → 推迟"两个关键方向成对确证。

### 2. OBSERVE-93-01 第三十三次核验 ✅ 维持

`grep -rn "<button" web/src --include="*.tsx"` = 17 处，恰 3 文件：Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2。零增零减。651/661 候选（Dashboard CollapseSection 的两个 button：:67 折叠按钮 + 次 enum 按钮）维持成立。

### 3. F93-01 第五十次双空实证 ✅ 实证

`git log --oneline 2ca7e0c..HEAD -- web/` 输出空、`git diff --stat 2ca7e0c..HEAD -- web/` 输出空。R141 归档后 web/ 零提交零差异。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第二十七次核验 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-8（hook 顶层消费 + memo 叶子收敛）、Dashboard.tsx:206-212（"已收敛到 MemoCountdownMatrix 叶子"落地实现注释）、Dashboard.tsx:90-94（CountdownMatrix memo 边界、"不再声称只重渲染倒计时一处需拆分，已拆"）。三处口径同源。

五路轮询契约逐键零漂移：
- Select /electives（:58-82）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000（从缓存按 key 读 /state，规避 TDZ）
- Select /state（:148-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:203）：恒 30000

error 与 window_closed 双降 30000 契约于 Select/Dashboard 全键一致，无漂移。

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R141 一致

三处最小语义门行号实测：Select.tsx:1204（role="dialog"）、Login.tsx:226（role="dialog"）、Admin.tsx:213（role="dialog"），与 R141 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 内联 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（顶栏横幅倒计时）维持站在"路由顶层非叶子"位置。缓解因子实测无回归：① Dashboard 主矩阵已 memo 化（CountdownMatrix），② Select 顶栏该处是单行文本片段、DOM 差分成本可忽略，③ useTickingCountdown 本身每秒 setNow 语义（hook 收敛承诺）未改。守卫盲区契约零漂移：perf-countdown-guard 断言"路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）"绿——注意该断言量的是"裸消费"而非"cd.* 消费"，Select:850 内联属于"有条件收敛"的文本插值，非裸 `<cd.x>` 直渲，故断言保持绿。 维持记录不实现。

### 7. 新契约角度（自选 ×3，纵深） ✅

**A. Admin 复制降级链三阶纵深（Admin.tsx:67-111）**：clipboard API 可用 → writeText 成功置 copied + 1.5s 自动回清（copyTimer 去重旧定时器防跨码错乱）；不可用/抛错 → 构造 textarea（position:fixed + opacity:0 + **readOnly 加固**）走 document.execCommand("copy") 同步兜底，成功走同款反馈；再失败才 toast "复制失败，请手动抄录" + 展示完整激活码供抄录。卸载时清理挂起定时器（:114-119）。三阶语义闭环、时限功能（copyTimer 复用去抖）到位。判定最懒实现：全部为原生 Web API 最小链，无任何新增依赖。

**B. App 管理态会话流转全链路（App.tsx）**：adminToken 标记（:98-99，登录响应带 adminName 即标记）→ 渲染判据 inAdmin || isCurrentAdminSession(sessions, adminName, adminToken)（:299）→ 清除出口成对清单：logout（:121-122）、onDeleted 删管理员自己（:174-176）、onUnauthorized 吊销管理员（:213-214）、onBackToStudent（:312-313）、账号迁移 effect 判据失败（:147）。四处清理出口与被测 6 断言（admin-auth-check）语义全对：撞名学生 token 永不与标记一致、标记残留无害（sessions[admin] 已删恒 false）。isCurrentAdminSession 纯函数（adminAuth.ts:6-12）三段判据与原契约逐字符一致。

**C. 倒计时 begin_times 兜底四位同源核验（F39-N1）**：主矩阵（Dashboard:222-227）、主时间文案行（Dashboard:410-416）、Select 顶栏倒计时（Select:767-772）、Select 时间戳展示（Select:856-860）四处均执行"openTimeStr 优先，识别缺席回落 begin_times[0]"同构。识别槽建立后仍以识别真值为准、兜底只在缺席时生效。契约"避免主矩阵未知但折叠列表有值"的承诺实际落到两页四处输入上，且 Dashboard:237-243 extrasMs 故意不与主时间合并（识别态唯一事实源）。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：该注释仍写"如需真正做到「只重渲染倒计时一处」需拆独立 memo 叶子组件（潜在优化，非当前承诺）"——但 Dashboard 侧已落地拆分（CountdownMatrix memo）。Select 未拆顶栏倒计时但行为与"收敛到 hook 自 tick + DOM 差分可忽略"的当前架构兼容，注释属"历史承诺未更新"而非行为缺陷，无回归风险，维持观察。
2. **Admin refetch 出口**：五 Tab 查询（:316/:502/:723/:817/:912）全部手动 refetch 出口 + queryClient.invalidateQueries（:262）到位，轮询 5000/5000/10000/5000/5000 维持。子 Tab 切换不销毁查询组件实例（同一组件内条件渲染），轮询为后台常驻 5s/10s，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言"路由组件顶层无裸 cd.* 消费（0 处）"通过但量纲较粗（只查裸消费，不查条件渲染内的 cd.* 插值如 Select:850），实测维持默认忽略。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿、构建通过、XSS 面零命中、工作区零漂移、F93-01 双空实证。维持观察项不构成合并阻塞，无需本轮修复。