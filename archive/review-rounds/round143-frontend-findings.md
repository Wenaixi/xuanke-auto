# R143 前端审查报告（M-1）——第七十九次核验

基线：654de67（R142 归档） | 审查时间：2026-09-24 | 审查代理：R143 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。三条维持观察项依旧维持（第三条 perf 守卫弱断言本轮实测进一步坐实：Select:850 内联倒计时是条件渲染内文本插值，perf 守卫只量「裸消费」故不报警，属既有已知盲区）。建议 **APPROVE**。

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
| 构建 | `npm run build`（tsc -b + vite build） | 通过，vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css 41.72 kB | 2026-09-24 |
| target 守卫 | `npx tsx web/scripts/target-guard-check.ts` | 18 断言全绿（输出末行"target-guard 断言全绿"） | 2026-09-24 |
| perf 守卫 | `npx tsx web/scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 裸消费） | 2026-09-24 |
| admin-auth 守卫 | `npx tsx web/scripts/admin-auth-check.ts` | 6 断言全绿（会话一致/撞名学生/吊销/无标记/自定义管理员名） | 2026-09-24 |
| unauthorized 守卫 | `npx tsx web/scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中 | 2026-09-24 |
| 工作区 | `git status --porcelain` | 零改动零未跟踪文件（构建产物 web/dist 与 backend/web/dist 均被 check-ignore，不入账） | 2026-09-24 |
| git 双空 | `git log --oneline 654de67..HEAD -- web/` + `git diff --stat 654de67..HEAD -- web/` | 双空（无 web/ 提交、无 web/ 差异） | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" src/routes/Select.tsx` | 4（调用级）；全文 grep-c 为 10（含 import 1 + 定义处不存在 + 注释 5 + 调用 4） | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 第七十九次闭合（保存链守卫） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

三段判据与 CLAUDE.md 契约逐行一致，注释忠实记录了 F43-M1 全清空分判动机（targetGuard.ts:53-56「显式清空绝不与慢首帧混判」、:57-63「已回显稳态下继续推迟会把后续编辑永久闷死」）。注释三参语义与实现逐字对应。

**恰 4 消费点**（`grep -c "shouldDeferSave("` 实测 = 4，调用级调用点恰 4 处，全文 10 行命中含 import 1 + 注释 5 + 调用 4）：
- Select.tsx:509（flushTargets）`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门）`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

四消费点三参数逐字符一致（第一参数均为秒最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）。语义分属防抖/flush/handleBack 双闸，无第五处遗漏。

**echoedRef 三置位**：写点恰 3 处，:200 `= false`（账号切换复位守卫，绝不写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。无第四处写 true——发布重建独立清理 effect（:318-329）只读不写 echoedRef，与既有契约一致。

**首帧四边界**：:229（echoed 短路）、:234（stateData undefined 提前 return）、:247（pubs 空 return）、:319（清理 effect 未回显短路）四处全部在位。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本第 18 断言「无变更 → 返回原引用」绿。清理 effect（Select:318-329）与回显 effect 内残留清理（:289-296）双路径均在位，注释肩注「清理已下沉为独立 effect」与实现一致。

**F43-M1 hasSelected 清空分判**：守卫脚本 6 条 shouldDeferSave 用例全绿，关键两向「courses 非空 + 全清空 → 放行」与「courses 非空 + 有选中 + 未回显 → 推迟」成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597）+ 5s 等待 while（:605）双闸：入场门 rev>0 才判，while 轮询 50ms 间隔（React 一帧约 16ms 足以感知合并完成，注释说明 10ms 会让 5s 窗口开约 500 个定时器）。5s 后 flush 内守卫仍拦截发布缺席覆盖。等待只在「首帧未到或首帧携带旧目标」发生（courses 空已置 echoedRef 立即放行），纯浏览（rev=0）零推迟。三闸判据同源零漂移。

### 2. OBSERVE-93-01 第三十三次核验 ✅ 维持

`grep -rc "<button" web/src --include="*.tsx"` = Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2，全仓 17 处零增零减。651/661 候选（Admin.tsx 引擎二选按钮）维持候选名义。

### 3. F93-01 第五十一次双空实证 ✅ 实证

`git log --oneline 654de67..HEAD -- web/` 输出空、`git diff --stat 654de67..HEAD -- web/` 输出空。R142 归档后 web/ 零提交零差异。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第二十七次核验 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-7（hook 顶层消费 + memo 叶子收敛承诺）、Dashboard.tsx:90-94（CountdownMatrix memo 边界、「不再声称只重渲染倒计时一处需拆分，已拆」）、Dashboard.tsx:206-212（「删除整页每秒 setTick……已收敛到 MemoCountdownMatrix 叶子」）。三处口径同源。

五路轮询契约逐键零漂移（error 与 window_closed 双降 30000 于 Select/Dashboard 全键一致）：
- Select /electives（:58-82）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000（经 queryClient 缓存按 key 读 /state，规避 TDZ，注释 :72-75 明示）
- Select /state（:148-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:203）：恒 30000（后端快照 TTL 40s > 30s，低于 40s 轮询大概率取同一份快照）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R142 一致

三处最小语义门行号实测：Select.tsx:1204（role="dialog"）、Login.tsx:226（role="dialog"）、Admin.tsx:213（role="dialog"），与 R142 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题。Esc 关闭实现三处齐备：Select:1207-1212（!actionLoading 防误关）、Login:232-238（!activating 防误关）、Admin:216-220（!deleting 防误关）。autoFocus 语义各自适配：Select:1234 取消按钮（手写弹窗「取消」autoFocus 契约，与 CLAUDE.md 契约 16 一致）、Login:267 激活码输入框（弹窗首操作是输码，语义正确）、Admin:241 取消按钮（删除确认双按钮，取消为安全默认）。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 内联 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（顶栏横幅倒计时）维持站在「路由顶层非叶子」位置（单行文本插值，非 memo 叶子）。缓解因子实测无回归：① Dashboard 主矩阵已 memo 化（CountdownMatrix memo 包裹），② Select 顶栏该处是单行文本片段、DOM 差分成本可忽略（Select.tsx:204-207 注释自身陈述「DOM 差分成本可忽略」），③ useTickingCountdown 每秒 setNow 语义未改。守卫盲区契约零漂移：perf-countdown-guard 断言「路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）」绿——该断言只量「裸消费」（正则 /(days|hours|minutes|seconds)/ 过滤掉 memo/Matrix/props 行），Select:850 属条件渲染内文本插值、非裸 `<cd.x>` 直渲，故断言保持绿，与 R142 记录一致。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅

**A. 倒计时 begin_times 兜底四位同源核验（F39-N1，纵深保持）**：主矩阵（Dashboard:222-227）、主时间文案行（Dashboard:410-416）、Select 顶栏倒计时（Select:767-772）、Select 时间戳展示（Select:856-860）四处均执行「openTimeStr 优先，识别缺席回落 begin_times[0] 转 UTC ISO」同构。识别槽建立后仍以识别真值为准、兜底只在缺席时生效。Dashboard:240-244 extrasMs 故意不与主时间合并（识别态唯一事实源——多时间 exif 碎单值时主位保持单一真值），注释明示。契约「避免主矩阵未知但折叠列表有值」的承诺实际落到两页四处输入上，零漂移。

**B. Select 挂载点 key={account} + 跨账号复位守卫双保险（契约 13）**：App.tsx:293-298（代理分支 `key={targetAccount}`）与 :339-344（学生分支 `key={current}`）两处挂载点 key 齐备；Select.tsx:195-202 兜底复位守卫（accountKey!==account 时 setSelected({}) + setRev(0) + echoedRef=false + setEchoDone(false)），注释明示声明位置在 echoedRef/rev/setRev/setEchoDone 之后（TDZ 不触发，契约 13 后段）。渲染分支四路（:292-345：targetAccount→Select / inAdmin||isCurrentAdminSession→Admin / page==="dashboard"→Dashboard / else→Select）互斥无重叠。onDeleted 删除管理员自身时 setInAdmin(false)+setAdminToken("")+saveAdminToken("")（App:172-177）与 logout/onUnauthorized 对称（契约 F39-M1）；账号迁移 effect 复核（:147）isCurrentAdminSession 失败即退出管理态，全部清除路径七处（:118/:121-122/:147/:172-177/:211-214/:309-313/logout 首行 setInAdmin(false)）齐备。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Dashboard.tsx:90-94 注释自身就是「已拆」落地说明。Select 未拆顶栏倒计时但行为与「收敛到 hook 自 tick + DOM 差分可忽略」的当前架构兼容，注释属「历史承诺未同步更新」而非行为缺陷（146 行注释与 94 行实现并存的轻微口径分歧），无回归风险，维持观察。
2. **Admin refetch 出口**：五 Tab 查询全部手动 refetch 出口 + queryClient.invalidateQueries 到位，轮询 5000/5000/10000/5000/5000 维持。子 Tab 切换不销毁查询组件实例（同一组件内条件渲染），轮询为后台常驻 5s/10s，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言「路由组件顶层无裸 cd.* 消费（0 处）」通过但量纲较粗（只查裸消费，不查条件渲染内的 cd.* 文本插值如 Select:850），本轮实测再次坐实盲区存在但行为无回归，维持记录。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（4+3+6+5=18 断言）、构建通过（vite v8.3.0）、XSS 面零命中、工作区零漂移、F93-01 双空实证。维持观察项不构成合并阻塞，无需本轮修复。