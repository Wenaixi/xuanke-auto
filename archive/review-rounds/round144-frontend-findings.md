# R144 前端审查报告（M-1）——第八十次核验

基线：6179674（R143 归档） | 审查时间：2026-09-24 | 审查代理：R144 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。三条维持观察项依旧维持（Select 注释口径残留自 R124 延续、Admin 轮询带宽、perf 守卫弱断言本轮实测再次坐实盲区）。建议 **APPROVE**。

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
| 构建 | `npm run build`（tsc -b + vite build） | 通过，vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css 41.72 kB（与 R143 逐字节同尺寸同哈希——R143 后 web/ 零变更的直接佐证） | 2026-09-24 |
| target 守卫 | `npx tsx web/scripts/target-guard-check.ts` | 18 断言全绿（8 defer + 5 stale + 5 clean，输出末行"target-guard 断言全绿"） | 2026-09-24 |
| perf 守卫 | `npx tsx web/scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 裸消费） | 2026-09-24 |
| admin-auth 守卫 | `npx tsx web/scripts/admin-auth-check.ts` | 6 断言全绿（会话一致/撞名学生/吊销/无标记/自定义管理员名） | 2026-09-24 |
| unauthorized 守卫 | `npx tsx web/scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中 | 2026-09-24 |
| 工作区 | `git status --short` | 零改动零未跟踪文件（backend/web/dist 经 `git check-ignore` 确认被忽略，不入账） | 2026-09-24 |
| git 双空 | `git log --oneline 6179674..HEAD -- web/` + `git diff --stat 6179674..HEAD -- web/` | 双空（无 web/ 提交、无 web/ 差异） | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" src/routes/Select.tsx` | 4（调用级）；全文`shouldDeferSave` 命中 16 行（import 1 + 注释 11 + 调用 4） | 2026-09-24 |

> 注：`grep -c "shouldDeferSave("` 实测为调用级恰 4（:509/:597/:605/:699），与 R143 记录一致。全文 `shouldDeferSave` 关键字命中 16 行含 import 1 + 注释 11 + 调用 4，注释占比高于 R143 记录的 5（因本轮消费点上下文注释更详尽），语义计数不变。

## 聚焦清单逐项裁决

### 1. M-1 第八十次闭合（保存链守卫） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）逐字符核验：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

三段判据与 CLAUDE.md 契约逐行一致。注释（:45-63）忠实记录了 F43-M1 全清空分判动机（:53-56「显式清空绝不与慢首帧混判」）与已回显稳态放行动机（:57-63「courses 永驻非空 + selected 无变化 bailout = 无解锁信号」），三参语义与实现逐字对应。

**恰 4 消费点**（调用级实测 = 4）：四消费点三参数逐字符一致（第一参数均为秒最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位**：写点恰 3 处——:200 `= false`（账号切换复位守卫，绝不写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。发布重建独立清理 effect（:318-329）只读不写 echoedRef，与既有契约一致，无第四处写 true。

**首帧四边界**：:229（`if (echoedRef.current) return` 短路）、:234（`if (stateData === undefined) return`）、:247（`if (pubs.length === 0) return`）、:319（`if (!echoedRef.current || publishes.length === 0) return` 清理 effect 未回显短路）四处全部在位（实测行号文本逐行核对）。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本「无变更 → 返回原引用」断言（第 18 条）绿。双路径到位：回显 effect 内残留清理（:289-296，随 stateData 首帧执行）与发布重建独立清理 effect（:318-329）。useState 双向状态 `echoDone`（:166）与防抖 effect 依赖（:755 `echoDone, stateData`）构成完整自愈链——场景 B（后端确证无旧目标 courses 空）只置位不改 selected，无 echoDone 依赖置脏改动永不重试落库，注释 :748-751 完整陈述。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 全清空 → 放行」（false）与「courses 非空 + 有选中 + 未回显 → 推迟」（true）成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，revRef.current > 0 才判）+ 5s 等待 while（:605）双闸，轮询间隔 50ms（React 一帧约 16ms，注释说明 10ms 会让 5s 窗口开约 500 个定时器）。防抖回调尾段（:727-739）消费时刻双闸齐备：联查空（`next.length === 0 && selectedCount > 0`）+ 发布 id 漂移（`targetsUseCurrentPublishes`）双守卫，与 flush 侧 :553/:558 对称。flush 侧消费时刻守卫族（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）五层全齐。守卫命中只置脏不置 dirtyRef（:503/:516 注释显式「守卫不置 dirtyRef——终局绝不误报保存失败」），toast 只在卸载判断后弹（`!unmountedRef.current`）。防抖 400ms 窗口（:746）与 stats 锁死自愈（:755 依赖数组含 echoDone + stateData）零漂移。

### 2. OBSERVE-93-01 第三十四次核验 ✅ 维持

`grep -rc "<button" web/src` 实测 = Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2，全仓 17 处零增零减。651/661 候选（Admin.tsx 引擎二选按钮，:651 `<button onClick={() => setEngine("vision")}`、:661 `onClick={() => setEngine("ddddocr")}`）实测行号吻合。每处 button 语义完整（文本内容或 aria-label 齐备，Dashboard 折叠段带 aria-expanded/aria-controls）。维持候选名义，无新增 button 文本裸渲面。

### 3. F93-01 第五十二次双空实证 ✅ 实证

`git log --oneline 6179674..HEAD -- web/` 输出空、`git diff --stat 6179674..HEAD -- web/` 输出空。R143 归档后 web/ 零提交零差异。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第二十八次核验 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-7（「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」，全文读取实测）、Dashboard.tsx:90-94（CountdownMatrix memo 组件头注释「由『拆 memo 叶子组件潜在优化』转为落地实现，注释口径同步：不再声称"只重渲染倒计时一处"需拆分，已拆」）。三处口径同源（含 Dashboard.tsx:206-212 整页 setTick 已删）。useTickingCountdown 自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致）：
- Select /electives（:58-82）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000（:72-75 注释明示绝不直读下方 stateData 防 TDZ，改从 react-query 缓存按 key 读）
- Select /state（:148-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:203）：恒 30000（后端快照 TTL 40s > 30s，注释 :196-199 明示）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R143 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R143 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 语义各自适配且与 R143 一致：Select:1234 取消按钮（手写弹窗「取消」autoFocus 契约，CLAUDE.md 契约 16）、Login:267 激活码输入框（弹窗首操作是输码）、Admin:241 取消按钮（删除确认双按钮，取消为安全默认）。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵已 memo 化（CountdownMatrix memo 组件），② Select 顶栏该处是单行文本插值、DOM 差分成本可忽略（Select.tsx:204-207 注释自身陈述，行为与架构兼容），③ useTickingCountdown 每秒 setNow 语义未改（:13）。守卫盲区契约零漂移：perf-countdown-guard 断言「路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）」实测绿——断言只量「裸消费」（正则过滤掉 memo/Matrix/props 行），Select:850 属条件渲染内文本插值、非裸 `<cd.x>` 直渲，故断言保持绿。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅

**纵深 A：Dashboard 折叠键盘路径 + 折叠状态族（CollapseSection 手写极简折叠 + extrasOpen + expandedDates）**

CollapseSection（Dashboard:50-88）：手写 `button` 带 type="button" / aria-expanded / aria-controls（useId 单实例唯一 id，:64），正文 `div` 带同 id 落位（:82），键盘可聚焦、读屏可感知展开状态。组件为「按钮 + 条件渲染」极简实现，注释（:47-49）明示「项目无 Radix Collapsible（未安装），手写最小实现符合简洁优先」——未引入依赖的契约保持。折叠族三类实例齐备且状态各自独立：
- 其他开放时间折叠段（extrasOpen, :244 + :429-456）与主倒计时「故意不与 begin_times 合并」语义协调（注释 :427-428「识别态唯一事实源」），extrasMs 仅排「其余从近到远」时间；
- 日期分组折叠（expandedDates, :300-305）：null=尚未初始化（首帧数据到达种下最近日期组）/ [] = 用户主动全折叠，null/[] 语义分离保证绝不重种（:297-299 注释「折叠状态与用户操作打架」根除）；onToggle 函数式更新（:556-562）绝无 stale 闭包；
- 折叠种子 effect（:301-305）首帧种最近组后不追踪漂移，「用户操作即最终话语权」。

**纵深 B：401 吊销切号整体重建 + 管理令牌五路径成对全清（契约 13 + F39-M1 + F52-M1）**

挂载点双保险实测：App.tsx:293-298（代理分支 `key={targetAccount}`）与 :339-344（学生分支 `key={current}`）齐备。Select.tsx:195-202 兜底复位守卫：`accountKey !== account` 时 `setSelected({})` + `setRev(0)` + `echoedRef.current = false` + `setEchoDone(false)` 四态全复位，注释明示声明于 echoedRef/rev/setRev/setEchoDone 之后（:194，TDZ 不触发）。

管理令牌 adminToken 标记与五路径成对全清（setAdminToken("") + saveAdminToken("") 恒成对）：
1. login 写入标记（App:98-99，仅「登录响应带 adminName」——撞名学生无 adminName 绝不误标，:95-97 注释）
2. logout（:121-122）
3. onDeleted 删除管理员自己（:175-176，F39-M1 防残留 inAdmin 学生令牌拉五 Tab 连环 401）
4. onUnauthorized 吊销管理员会话（:213-214）+ 吊销学生账号时 isCurrentAdminSession && lostAccount !== adminName 绝不误杀管理员（:217）
5. onBackToStudent 切回学生端（:312-313）

isCurrentAdminSession 纯函数（adminAuth.ts:6-11）「adminToken !== "" && sessions[adminName] === adminToken」与渲染判据同源（App:299），admin-auth 守卫 6 断言全绿（含撞名学生、吊销、无标记、自定义管理员名四向负样本）。账号迁移 effect（App:147）复核失败即退出管理态，与五路径闭环对称。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且本文件 behavior 层 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:844-852 倒计时渲染处），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，无回归风险，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询 5000/5000/10000/5000/5000 实测维持，全部手动 refetch 出口 + invalidateQueries 到位。子 Tab 切换不销毁查询组件实例（同一组件内条件渲染），轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言「路由组件顶层无裸 cd.* 消费（0 处）」实测继续绿但量纲较粗（排序不查条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）、构建通过（vite v8.3.0，产物与 R143 同尺寸同哈希）、XSS 面零命中、工作区零漂移、F93-01 第五十二轮双空实证。两条新契约纵深（Dashboard 折叠键盘路径、401 切号重建与管理令牌五路径成对全清）均无发现。维持观察项不构成合并阻塞，无需本轮修复。