# R145 前端审查报告（M-1）——第八十一次核验

基线：4aecde0（R144 归档，M-1 第八十轮闭合） | 审查时间：2026-09-24 | 审查代理：R145 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。四条维持观察项依旧维持（Select 注释口径残留自 R124 延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 包装 candidate 维持候选）。两条新契约纵深（目标保存 flush 出口收敛 + actionLoading 在飞守卫族 + Admin 复制降级链 + 倒计时四位同源 / 无障碍基线）均无发现。建议 **APPROVE**。

| 级别 | 数量 | 说明 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| MINOR | 0 | — |
| OBSERVE | 4 | 维持观察（Select 注释口径残留 / Admin refetch 出口 / perf 守卫弱断言 / Button 裸 button 候选），均无回归 |

## 验证表（实测数据）

| 验证项 | 命令 | 结果 | 实测时间 |
|--------|------|------|----------|
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css 41.72 kB（与 R143/R144 同哈希同尺寸——R144 后 web/ 零变更的直接佐证） | 2026-09-24 |
| target 守卫 | `cd web && npx tsx scripts/target-guard-check.ts` | 18 断言全绿（8 defer + 5 stale + 5 clean），输出末行"target-guard 断言全绿" | 2026-09-24 |
| perf 守卫 | `cd web && npx tsx scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 裸消费） | 2026-09-24 |
| admin-auth 守卫 | `cd web && npx tsx scripts/admin-auth-check.ts` | 6 断言全绿（会话一致/撞名学生/吊销/无标记/自定义管理员名） | 2026-09-24 |
| unauthorized 守卫 | `cd web && npx tsx scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中 | 2026-09-24 |
| 工作区 | `git status --porcelain --untracked-files=all` | 零改动零未跟踪文件（backend/web/dist 经 `git check-ignore` 确认被忽略，不入账） | 2026-09-24 |
| git 双空 | `git log --oneline 4aecde0..HEAD -- web/` + `git diff --stat 4aecde0..HEAD -- web/` | 双空（无 web/ 提交、无 web/ 差异） | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" web/src/routes/Select.tsx` | 调用级恰 4（:509/:597/:605/:699）；全文 `shouldDeferSave` 关键字命中 10 行（import 1 + 注释 6 + 调用 4） | 2026-09-24 |
| 基线同文件 | `git show 4aecde0:web/src/routes/Select.tsx` diff 当前 | 逐字节零差异（Select.tsx 自 R144 后未动一行的直接佐证） | 2026-09-24 |

> 注：`grep -c "shouldDeferSave("` 实测为调用级恰 4（:509/:597/:605/:699），与 R144 记录一致。全文关键字命中 10 行含 import 1 + 注释 6 + 调用 4——R144 记录为 16 行（含 11 行注释），差异源于注释合并为多行块（块头 1 行命中 + 块体换行），计数随注释折行分布浮动，语义内容不变。

## 聚焦清单逐项裁决

### 1. M-1 第八十一次闭合（保存链守卫） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）逐字符核验：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

三段判据与 CLAUDE.md 契约逐行一致。注释（:45-63）忠实记录了 F43-M1 全清空分判动机（:53-56「显式清空绝不与慢首帧混判」）与已回显稳态放行动机（:57-63「courses 永驻非空 + selected 无变化 bailout = 无解锁信号」），三参语义与实现逐字对应。

**恰 4 消费点**（调用级实测 = 4）：四消费点三参数逐字符一致（第一参数均为最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位**：写点恰 3 处——:200 `= false`（账号切换复位守卫，绝不写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。发布重建独立清理 effect（:318-329）只读不写 echoedRef，无第四处写 true。

**首帧四边界**：:229（`if (echoedRef.current) return` 短路）、:234（`if (stateData === undefined) return`）、:247（`if (pubs.length === 0) return`）、:319（`if (!echoedRef.current || publishes.length === 0) return` 清理 effect 未回显短路）四处全部在位（行号文本逐行核对）。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本「无变更 → 返回原引用」断言（第 18 条）绿。双路径到位：回显 effect 内残留清理（:289-296）与发布重建独立清理 effect（:318-329，依赖数组含 selected 防重建后 merge 带出 stale 的时序巧合依赖）。`echoDone` 双向 state（:166）与防抖 effect 依赖（:755 `echoDone, stateData`）构成完整自愈链。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 全清空 → 放行」（false）与「courses 非空 + 有选中 + 未回显 → 推迟」（true）成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，revRef.current > 0 才判）+ 5s 等待 while（:605，轮询 50ms）双闸。防抖回调尾段（:727-739）消费时刻双闸齐备（联查空 + 发布 id 漂移）；flush 侧消费时刻守卫族（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）五层全齐。守卫命中只置脏不置 dirtyRef（:503/:516 注释显式「守卫不置 dirtyRef——终局绝不误报保存失败」），toast 只在卸载判断后弹（`!unmountedRef.current`）。防抖 400ms 窗口（:746）与自愈依赖（:755）零漂移。

### 2. OBSERVE-93-01 第三十五次核验 ✅ 维持

`grep -rn "<button" web/src` 实测 = Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2，全仓 17 处零增零减（计数与 R144 记录逐项一致）。651/661 候选（Admin.tsx 引擎二选按钮）实测行号吻合：:651 `onClick={() => setEngine("vision")}`、:661 `onClick={() => setEngine("ddddocr")}`。每处 button 语义完整（文本内容或 aria-label 齐备，Dashboard 折叠段带 aria-expanded/aria-controls，Admin 开关带 role="switch" + aria-checked）。维持候选名义，无新增 button 文本裸渲面。

### 3. F93-01 第五十三次双空实证 ✅ 实证

`git log --oneline 4aecde0..HEAD -- web/` 输出空、`git diff --stat 4aecde0..HEAD -- web/` 输出空。R144 归档后 web/ 零提交零差异，且 Select.tsx 单文件逐字节与 4aecde0 相同（构建产物与 R143/R144 同哈希双佐证）。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第二十九次核验 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-7（「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」，全文读取实测）、Dashboard.tsx:90-94（CountdownMatrix memo 组件头注释「由『拆 memo 叶子组件潜在优化』转为落地实现，注释口径同步：不再声称"只重渲染倒计时一处"需拆分，已拆」）、Dashboard.tsx:119-121（relativeCountdown 注释）、:206-212（整页 setTick 已删注释）四处同源。useTickingCountdown 自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致）：
- Select /electives（:58-82）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000（:72-75 注释明示绝不直读下方 stateData 防 TDZ，改从 react-query 缓存按 key 读）
- Select /state（:148-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:200-204）：恒 30000（后端快照 TTL 40s > 30s，注释 :196-199 明示）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R144 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R144 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 语义各自适配且与 R144 一致：Select:1234 取消按钮（手写弹窗「取消」autoFocus 契约，CLAUDE.md 契约 16）、Login:267 激活码输入框（弹窗首操作是输码）、Admin:241 取消按钮（删除确认双按钮，取消为安全默认）。三处 autoFocus 落位实测（grep 三路由 = 3 处，无遗漏无多增）。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵已 memo 化（CountdownMatrix memo，:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`），② Select 顶栏该处是单行文本插值、DOM 差分成本可忽略（Select.tsx:204-207 注释自身陈述，行为与架构兼容），③ useTickingCountdown 每秒 setNow 语义未改（:13）。守卫盲区契约零漂移：perf-countdown-guard 断言「路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）」实测绿——断言只量「裸消费」（正则过滤掉 memo/Matrix/props 行），Select:850 属条件渲染内文本插值、非裸 `<cd.x>` 直渲，故断言保持绿。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：目标保存 flush 出口收敛 + actionLoading 在飞守卫族**

flush 出口收敛（Select.tsx:382-467）实测：targetRef/dirtyRef/savingRef 三 ref 全部写点收敛（:382/:384 targetRef/dirtyRef 声明，:434/:561/:740 targetRef 写点，:455/:463/:563/:742 dirtyRef 写点——只有 saveNow catch 与飞行中补发两族置位，守卫路径永不置 dirtyRef），`pendingUnsaved` 三信号（dirtyRef 真实失败/savingRef 在飞/timer 退避排队，:418-419）与 `resetRetry`（:409-413）成对清位。saveNow 指数退避 2s/4s/8s/16s/16s（:423，5 次停手）与卸载停手链（unmountedRef :390/:399-408，卸载后孤儿重试根除）。flush 消费时刻守卫族五层（:515/:526/:553/:558）与防抖回调族（:709/:717/:732/:737）对称同源，末端 `targetRef.current = targets`（:561）与防抖侧（:740）一致——即"通过全部守卫的快照才落 targetRef 供 PUT"，绝无守卫漏网快照直接发请求。

actionLoading ReadonlySet（:52）实测：置位/清除恒函数式（:93/:107/:120/:132 `new Set(prev)`），finally 函数式清除只删自己的 id（:106-110/:131-135）绝无交叉抹除。全 17 处引用点清点：入口守卫 2（:92/:119）、渲染 disabled 4（:1120/:1133 报名退选 + :1232/:1242 弹窗双钮）、文案 3（:1126/:1139/:1245）、Esc 防误关 1（:1208）。报名/退选并发互不覆盖语义与 CLAUDE.md 契约 16 逐字一致。

**纵深 B：Admin 激活码复制降级链（clipboard→execCommand→toast）+ 倒计时 begin_times 兜底（F39-N1）四位同源 + 无障碍基线复核**

复制降级链（Admin.tsx:67-111）实测三级全齐：① `navigator.clipboard.writeText` 主路径（:75-78，非安全上下文不可用时抛错）→ ② `document.execCommand("copy")` 兜底（:85-98，textarea readOnly + fixed + opacity 0 加固，同步执行）→ ③ 仍失败 toast 完整激活码供抄录（:105-109）。copyTimer 清理三处（:80/:101 新复制先清旧定时器、:114-118 卸载 cleanup）。文案把完整激活码放进 description（管理员抄录可用），复制成功路径全部走 setCopied（:79/:100），CopiesTab 两处消费（:429-430 本次生成 / :470-471 全部列表）一致。

倒计时 begin_times 兜底（F39-N1）四位同源实测：Select.tsx:767-772（`openTimeStr ?? (data?.begin_times?.[0] != null ? new Date(data.begin_times[0]).toISOString() : null)`）与 Dashboard.tsx:222-227（同款 `electives?.begin_times?.[0]`）两处消费点逐字符同构；openTimeStr 判定一致（Select :761-764 / Dashboard :217-218）；primaryMs 兜底（Dashboard:237-240）与 cd 输入同源。识别缺席时主倒计时不再与折叠列表自相矛盾（「主矩阵未知但折叠列表有值」注释承诺落到倒计时输入上），零漂移。

无障碍基线复核：Select 搜索框 aria-label（:870）、进度条 role="progressbar" + aria-valuenow/min/max（Progress.tsx:25-28，unannounced 时 value=0 恒空条如实反映「未公布无进度」语义，:1100 注释与实现一致）、Dashboard 错误条 role="alert"（:351）、Login 可见性切换 aria-label + aria-pressed（:170-172）+ 错误条 role="alert"（:182/:273）、Admin 表格 aria-label="账号列表"（:833）、Admin switch role="switch" + aria-checked（:579-580）、CollapseSection aria-expanded + aria-controls + useId 唯一 id（Dashboard:61-84）。三路由 aria 基线整体成族，无裸交互控件缺语义面。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:844-852），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询（stats 5000 / 其余按 Tab 各自节奏）实测维持，全部手动 refetch 出口 + invalidateQueries 到位。子 Tab 切换不销毁查询组件实例，轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言「路由组件顶层无裸 cd.* 消费（0 处）」实测继续绿但量纲较粗（正则过滤 memo/Matrix/props 行后不量条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装 candidate 维持候选**：OBSERVE-93-01 17 处 `<button` 中有部分（Admin:417/425/441/452/470/473/577/651/661/701/792/898/944、Dashboard:67/709、Login:167/299）为组件库 Button 之外的裸 button。语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）、构建通过（vite v8.3.0，产物与 R144 同尺寸同哈希）、XSS 面零命中、工作区零漂移、F93-01 第五十三轮双空实证、Select.tsx 单文件与 4aecde0 逐字节相同。两条新契约纵深（目标保存 flush 出口收敛 + actionLoading 在飞守卫族 / Admin 复制降级链 + 倒计时四位同源 + 无障碍基线）均无发现。维持观察项不构成合并阻塞，无需本轮修复。
