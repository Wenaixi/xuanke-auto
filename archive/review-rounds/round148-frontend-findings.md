# R148 前端审查报告（M-1）——第八十四次核验

基线：237edb8（R147 归档，M-1 第八十三轮闭合） | 审查时间：2026-09-24 | 审查代理：R148 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。四条维持观察项依旧维持（Select 注释口径残留自 R124 起延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选维持候选）。两条新契约纵深（管理令牌五路径成对全清与 isCurrentAdminSession 纯函数 / Dashboard 折叠 extrasOpen 键盘路径 aria-expanded/aria-controls/useId）均无发现。建议 **APPROVE**。

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
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css index-1KHlpqcc.css 41.72 kB（与 R147/R146/R145 同哈希同尺寸——R144 后 web/ 零变更连续佐证，本轮又得一轮） | 2026-09-24 |
| target 守卫 | `cd web && npx tsx scripts/target-guard-check.ts` | 18 断言全绿（8 defer + 5 stale + 5 clean），输出末行"target-guard 断言全绿" | 2026-09-24 |
| perf 守卫 | `cd web && npx tsx scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 裸消费） | 2026-09-24 |
| admin-auth 守卫 | `cd web && npx tsx scripts/admin-auth-check.ts` | 6 断言全绿（会话一致/撞名学生/吊销/无标记/自定义管理员名） | 2026-09-24 |
| unauthorized 守卫 | `cd web && npx tsx scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径） | 2026-09-24 |
| 视觉审计 | `cd web && node scripts/audit.mjs` | 77 断言全绿（exit 0，视觉表面协调一致） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中（0 行） | 2026-09-24 |
| 工作区 | `git status --porcelain --untracked-files=all` | 零改动（含未跟踪空，本轮无并行代理产物需排除） | 2026-09-24 |
| git 双空 | `git log --oneline 237edb8..HEAD -- web/`（0 行）+ `git diff --stat 237edb8..HEAD -- web/`（0 行） | 双空实证（F93-01 第五十六轮） | 2026-09-24 |
| 基线同文件 | `git diff 237edb8` 对比当前 + Select.tsx / targetGuard.ts / useTickingCountdown.ts 三文件 | 逐字节零差异（R147 后未动一行直接佐证）；另以 3c3e915（R146）同法复比亦逐字节零差异，双基线佐证 | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" web/src/routes/Select.tsx` | 调用级恰 4（:509/:597/:605/:699） | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 第八十四轮闭合（保存链守卫） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）逐字符核验，判据本体三行与契约逐行一致：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

**恰 4 消费点**（调用级 grep -c 实测 = 4）逐字符一致（第一参数均为最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位**：写点恰 3 处——:200 `= false`（账号切换复位守卫，绝不写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。发布重建独立清理 effect（:318-329）只读不写 echoedRef，无第四处写 true。

**首帧四边界**：:229（`if (echoedRef.current) return` 短路）、:234（`if (stateData === undefined) return`）、:247（`if (pubs.length === 0) return`）、:319（`if (!echoedRef.current || publishes.length === 0) return` 清理 effect 未回显短路）四处全部在位（行号文本逐行核对）。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本「无变更 → 返回原引用」断言（第 18 条）绿。双路径到位：回显 effect 内残留清理（:289-296）与发布重建独立清理 effect（:318-329）。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 全清空 → 放行」（false）与「courses 非空 + 有选中 + 未回显 → 推迟」（true）成对确证；第四参数 echoed 的两向「已回显稳态 → 放行」「首帧未到 + 已回显 → 仍推迟」也成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，revRef.current > 0 才判）+ 5s 等待 while（:605，轮询 50ms）双闸。防抖回调尾段（:727-739）消费时刻双闸齐备（联查空 + 发布 id 漂移）；flush 侧消费时刻守卫族（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）五层全齐。守卫命中只置脏不置 dirtyRef（dirtyRef 写点实测仅 saveNow catch 段 :455 + 飞行补发两处 :563/:742——:463 为补发成功清位，守卫路径绝不置 dirtyRef），终局 toast 只归真实失败。echoDone 双向 state（:166）+ 防抖 effect 依赖（:755 `echoDone, stateData`）自愈链完整。

### 2. OBSERVE-93-01 第三十八轮 ✅ 维持

`grep -rn "<button" web/src` 实测 = Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2，全仓 17 处零增零减（计数与 R147 记录逐项一致）。651/661 候选（Admin.tsx 引擎二选按钮）实测行号吻合（:651 硅基流动 Vision / :661 本地 ddddocr）。每处 button 语义完整：Dashboard CollapseSection 折叠段带 aria-expanded/aria-controls（:69-70），Login 密码可见性切换按钮带 aria-label + aria-pressed（:171-172），Admin 激活码开关带 role="switch" + aria-checked + aria-label（:579-581）。维持候选名义，无新增 button 文本裸渲面。

### 3. F93-01 第五十六轮双空实证 ✅ 实证

`git log --oneline 237edb8..HEAD -- web/` 输出空、`git diff --stat 237edb8..HEAD -- web/` 输出空两路实证。R147 归档后 web/ 零提交零差异，且 Select.tsx / targetGuard.ts / useTickingCountdown.ts 三文件逐字节与 237edb8（R147）相同、另与 3c3e915（R146）复比亦逐字节相同（构建产物与 R147/R146 同哈希双佐证）。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第三十二轮 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-8（「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」全文读取实测）、Dashboard.tsx:90-94（CountdownMatrix memo 组件头注释「由『拆 memo 叶子组件潜在优化』转为落地实现，注释口径同步」）、Dashboard.tsx:119-121（relativeCountdown 注释）、:206-212（整页 setTick 已删注释）、:394-398（memo 叶子消费处注释）五处同源。useTickingCountdown 自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致）：
- Select /electives（:58-82）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000
- Select /state（:148-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:200-204）：恒 30000（后端快照 TTL 40s > 30s）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R147 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R147 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 三处落位实测（Select:1234 取消 / Login:267 激活码输入 / Admin:241 取消），grep 三路由 = 3 处无遗漏无多增。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵已 memo 化（:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`），② Select 顶栏该处是单行文本插值、DOM 差分成本可忽略（顶栏横幅上下文 :818-862 全文读取确认），③ useTickingCountdown 每秒 setNow 语义未改（:13）。守卫盲区契约零漂移：perf-countdown-guard 断言「路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）」实测绿——断言只量「裸消费」（正则过滤 memo/Matrix/props 行后不量条件渲染内 cd.* 文本插值如 Select:850），Select:850 属条件渲染内文本插值、非裸 `<cd.x>` 直渲，断言保持绿。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：管理令牌五路径成对全清 + isCurrentAdminSession 纯函数判定闭环**

管理令牌生命周期实测（App.tsx + adminAuth.ts）：
- 唯一写入点：login()（:98-99 `setAdminToken(token)` + `saveAdminToken(token)`），且仅当登录响应带 adminName（:92 判据）——撞名学生教务登录响应无 adminName（Login.tsx:49 透传第三可选参数），绝不误标
- 纯函数判据（adminAuth.ts:6-11）：`adminToken !== "" && sessions[adminName] === adminToken`——「曾标记为管理会话的 token」与「当前管理员名账号的会话 token」一致才算，账号名等于管理员名绝不是管理标志（撞名学生永锁 403 死锁根因收敛）
- 五路径成对全清（setAdminToken("") + saveAdminToken("") 同帧，一实一持）：
  - logout()（:121-122，主动登出）
  - onDeleted()（:173-176，管理员删除自己账号，含 setInAdmin(false) 对称：F39-M1 已落地）
  - onUnauthorized()（:210-214，管理员会话被 401 吊销，含 setInAdmin(false) 对称）
  - onBackToStudent()（:310-313，主动切回学生端，含 setInAdmin(false)）
  - 账号迁移 effect（:147 判据复位）——`!isCurrentAdminSession(loadSessions(), adminName, adminToken)` 即 setInAdmin(false)，无显式 setAdminToken("") 清位但有「sessions[adminName] 已删 → 判据恒 false」覆盖
- 六路消费/依赖全核对：:147（账号迁移 effect）、:217（401 误杀守卫，`isCurrentAdminSession && inAdmin && lostAccount !== adminName` 才 return）、:299（渲染判据 `inAdmin || isCurrentAdminSession(...)`）
- 守卫脚本 6 断言全绿覆盖关键两向：撞名学生 token ≠ 管理 token → 学生端（false）；管理会话已吊销 → 学生端（false）

**纵深 B：Dashboard 折叠 extrasOpen 键盘路径 + useId 单实例唯一 + 折叠种子语义分离**

Dashboard CollapseSection 折叠组件（:46-88）实测：useId 保证 aria-controls 指向唯一 id（:64 `const id = useId()`，:82 正文 div 挂 `id={id}`）——替代此前硬编码 "collapse-body"（多处实例化重复 id 违反 DOM 唯一性，注释 :61-63 明示缺陷背景）；aria-expanded/aria-controls 落在触发按钮（:69-70），原生 button type=button 键盘 Tab + Enter 可达，onClick 即 onToggle 天然键盘路径（无需额外 keydown）。extrasOpen（:244）+ 日期分组 expandedDates（:300）两族 onToggle 均函数式 setState（:432 `setExtrasOpen((o) => !o)` / :556-563 纯函数式改 prev），折叠状态用户操作即最终话语权。折叠种子语义分离（:297-305 注释 + 实现）：null = 尚未初始化（首帧种最近日期组）、[] = 用户主动全折叠，null/[] 分离保证绝不重种。

无障碍基线全复核：全路由 aria-label / role / aria-checked / aria-expanded / aria-pressed 共 25 处以上（Select 搜索框 :870、Login 密码可见性按钮 aria-pressed :171-172、Dashboard CollapseSection useId + aria-expanded/aria-controls、Admin 开关 role="switch" + aria-checked + aria-label :579-581、Select 顶栏 aria 布局）。Login 激活码弹窗裸 div 补 role=dialog + aria-modal + Esc 关闭（:220-238）；退选/删除两弹窗同族齐备。水墨画布契约（App.tsx:284-289 + global.css）维持。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:850），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询（codes 5000 / config 5000 / stats 10000 / accounts 5000 / logs 5000，:319/:726/:820/:915 实测）维持，全部手动 refetch 出口（9 处）+ invalidateQueries（:262 admin-accounts 删除后立即失效、:336/:357 codes 生成后、:541 config 保存后代际自增）到位。子 Tab 切换不销毁查询组件实例，轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言「路由组件顶层无裸 cd.* 消费（0 处）」实测继续绿但量纲较粗（正则过滤 memo/Matrix/props 行后不量条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装 candidate 维持候选**：Button.tsx 本体 `grep -c "<button"` = 0（组件内部不直接写裸 button，属包装 React.forwardRef 的抽象），OBSERVE-93-01 17 处 `<button` 全在消费侧（Admin/Dashboard/Login），语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）+ 视觉审计 77 断言全绿、构建通过（vite v8.3.0，产物与 R147/R146 同尺寸同哈希）、XSS 面零命中、工作区零漂移、F93-01 第五十六轮双空实证、Select.tsx / targetGuard.ts / useTickingCountdown.ts 三文件与 237edb8 逐字节相同（另与 3c3e915 复比相同，双基线佐证）。两条新契约纵深（管理令牌五路径成对全清 + isCurrentAdminSession 纯函数判定闭环 / Dashboard 折叠键盘路径 + useId 单实例唯一 + 折叠种子语义分离）均无发现。维持观察项不构成合并阻塞，无需本轮修复。
