# R146 前端审查报告（M-1）——第八十二次核验

基线：68c0bfc（R145 归档，M-1 第八十一轮闭合） | 审查时间：2026-09-24 | 审查代理：R146 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。四条维持观察项依旧维持（Select 注释口径残留自 R124 起延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选维持候选）。两条新契约纵深（管理令牌五路径成对全清 + Dashboard 折叠 extrasOpen 键盘路径 / client.ts checkResp 状态码机 + Admin 三态展示 + 倒计时四位同源复核）均无发现。建议 **APPROVE**。

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
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css index-1KHlpqcc.css 41.72 kB（与 R145/R143/R144 同哈希同尺寸——R144 后 web/ 零变更的连续佐证，本轮又得一轮） | 2026-09-24 |
| target 守卫 | `cd web && npx tsx scripts/target-guard-check.ts` | 18 断言全绿（8 defer + 5 stale + 5 clean），输出末行"target-guard 断言全绿" | 2026-09-24 |
| perf 守卫 | `cd web && npx tsx scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 裸消费） | 2026-09-24 |
| admin-auth 守卫 | `cd web && npx tsx scripts/admin-auth-check.ts` | 6 断言全绿（会话一致/撞名学生/吊销/无标记/自定义管理员名） | 2026-09-24 |
| unauthorized 守卫 | `cd web && npx tsx scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中（0 行） | 2026-09-24 |
| 工作区 | `git status --porcelain --untracked-files=all` | 零改动零未跟踪文件 | 2026-09-24 |
| git 双空 | `git log --oneline 68c0bfc..HEAD -- web/`（0 行）+ `git diff --stat 68c0bfc..HEAD -- web/`（0 行） | 双空实证 | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" web/src/routes/Select.tsx` | 调用级恰 4（:509/:597/:605/:699） | 2026-09-24 |
| 基线同文件 | `git show 68c0bfc:web/src/routes/Select.tsx` diff 当前 + targetGuard.ts 同法 | 两文件均逐字节零差异（R145 后 Select.tsx 与 targetGuard.ts 未动一行的直接佐证） | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 第八十二轮闭合（保存链守卫） ✅ 在位

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

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 全清空 → 放行」（false）与「courses 非空 + 有选中 + 未回显 → 推迟」（true）成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，revRef.current > 0 才判）+ 5s 等待 while（:605，轮询 50ms）双闸。防抖回调尾段（:727-739）消费时刻双闸齐备（联查空 + 发布 id 漂移）；flush 侧消费时刻守卫族（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）五层全齐。守卫命中只置脏不置 dirtyRef（dirtyRef 写点实测仅 saveNow catch 段 :455 + 飞行补发两处 :563/:742——:463 为补发成功清位，守卫路径绝不置 dirtyRef），终局 toast 只归真实失败。echoDone 双向 state（:166）+ 防抖 effect 依赖（:755 `echoDone, stateData`）自愈链完整。

### 2. OBSERVE-93-01 第三十六轮 ✅ 维持

`grep -rn "<button" web/src` 实测 = Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2，全仓 17 处零增零减（计数与 R145 记录逐项一致）。651/661 候选（Admin.tsx 引擎二选按钮）实测行号吻合。每处 button 语义完整（文本内容或 aria-label 齐备，Dashboard CollapseSection 折叠段带 aria-expanded/aria-controls，Admin 开关带 role="switch" + aria-checked）。维持候选名义，无新增 button 文本裸渲面。

### 3. F93-01 第五十四轮双空实证 ✅ 实证

`git log --oneline 68c0bfc..HEAD -- web/` 输出空、`git diff --stat 68c0bfc..HEAD -- web/` 输出空。R145 归档后 web/ 零提交零差异，且 Select.tsx 与 targetGuard.ts 双文件逐字节与 68c0bfc 相同（构建产物与 R145/R143/R144 同哈希双佐证）。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第三十轮 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-8（「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」全文读取实测）、Dashboard.tsx:90-94（CountdownMatrix memo 组件头注释「由『拆 memo 叶子组件潜在优化』转为落地实现，注释口径同步」）、Dashboard.tsx:119-121（relativeCountdown 注释）、:206-212（整页 setTick 已删注释）四处同源。useTickingCountdown 自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致）：
- Select /electives（:58-82）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000
- Select /state（:148-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:200-204）：恒 30000（后端快照 TTL 40s > 30s）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R145 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R145 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 三处落位实测（Select:1234 取消 / Login:267 激活码输入 / Admin:241 取消），grep 三路由 = 3 处无遗漏无多增。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵已 memo 化（:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`），② Select 顶栏该处是单行文本插值、DOM 差分成本可忽略，③ useTickingCountdown 每秒 setNow 语义未改（:13）。守卫盲区契约零漂移：perf-countdown-guard 断言「路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）」实测绿——断言只量「裸消费」（正则过滤掉 memo/Matrix/props 行），Select:850 属条件渲染内文本插值、非裸 `<cd.x>` 直渲，故断言保持绿。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：管理令牌五路径成对全清（isCurrentAdminSession 纯函数）+ 401 吊销切号整体重建 + 删除账号迁移 effect（setInAdmin false 对称）**

管理令牌五路径清位成对实证（App.tsx）——每处清 inAdmin 必同步清 adminToken 三连（setAdminToken("") + saveAdminToken("")），无单边残留：
1. login（:91-108）：adminName 存在才标记（setAdminToken/saveAdminToken），撞名学生响应无 adminName 绝不误标
2. logout（:112-132）：:118 setInAdmin(false) + :121-122 adminToken 三连清
3. 账号迁移 effect（:135-148）：:147 `if (!isCurrentAdminSession(loadSessions(), adminName, adminToken)) setInAdmin(false)`——刷新后管理页恢复唯一判据 =「当前管理员名账号的会话 token === 标记 token」
4. onDeleted（:160-178）：删除列表含管理员自己时 :172-177 双清（setInAdmin(false) + adminToken 三连），并清 targetAccount（:167）/current（:168）——绝不调用 logout()（删除他人账号语义错误，注释 :157-159 显式）
5. onUnauthorized（:188-235）：lostAccount === adminName 时 :210-214 双清 + targetAccount 代理态清（:204-206），「管理员在后台管理态代理查看学生大厅时若发生 401 绝不误杀管理员自身会话」分支（:217）读 isCurrentAdminSession 纯函数判定

isCurrentAdminSession（adminAuth.ts:6-11）单行纯函数 `adminToken !== "" && sessions[adminName] === adminToken`，admin-auth 守卫 6 断言全绿。Select 挂载点 key 双保险（App.tsx:294 `key={targetAccount}` / :340 `key={current}`）维持——401 吊销切号/代理切换/重登即整体重建实例，绝不复用旧账号 selected/echoedRef/rev。

**纵深 B：client.ts checkResp 状态码机 + Admin 三态展示 + 倒计时 begin_times 兜底（F39-N1）四位同源 + Dashboard 折叠 extrasOpen 键盘路径**

client.ts（web/src/api/client.ts:38-109）状态码机实测：非零 code 统一抛 ApiError 携带 code/msg/data；HTTP 401 在 r.json() 之前先广播 UNAUTHORIZED_EVENT（防网关非 JSON 401 体事件永不广播，:64-69），body 401 分支只在 `r.status !== 401` 时补广播（:85-94，防同一响应双发事件风暴），新式「HTTP 401 + body 401」与旧式「HTTP 200 + body 401」三形态各单次广播。ApiError.data 透传 1001 激活票据（:26-33）——Login.tsx:51 `e.code === 1001` 分支读取 `e.data?.ticket`（:55）。code=-2 超时/响应异常兜底文案（:73/:103）。Dashboard isSessionError 只认业务码 401（:43，网络错误/5xx 不误判为会话失效）。

Admin 三态展示（Admin.tsx:736-741）：`value: s.window_closed ? "已关闭" : s.window_opened ? "已开放" : "待命中"`——window_closed 依赖后端 /api/admin/stats 补发（B39-05），补发前 undefined 走「待命中」绝不假报关闭。

倒计时 begin_times 兜底（F39-N1）四位同源复核：Select.tsx:761-770（openTimeStr ?? begin_times[0]）与 Dashboard.tsx:217-227（openTimeStr ?? electives?.begin_times[0]）逐字符同构，primaryMs 兜底（Dashboard:237-240）与 cd 输入同源，注释（Select:765）明示「与 Dashboard 同构——同一浏览器」。

Dashboard 折叠 extrasOpen 键盘路径：CollapseSection（:50-88）useId 唯一 id（:64）→ button 带 aria-expanded/aria-controls（:69-70），useId 修复「正文 div 无 id 落位 + 多处实例化重复 id 违反 DOM 唯一性」（注释 :61-63）。extras 折叠段（:430-433）onToggle 函数式 `setExtrasOpen((o) => !o)`；日期分组段（:553-563）onToggle 函数式 setExpandedDates 增删，两族折叠键盘路径齐备。extrasMs 为 hooks 层 useMemo 派生常量（:240-244，fallback 空数组在 useMemo 内部绝不渲染期新建数组），杜绝每秒 tick 整页重渲染中的重复计算。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:844-852），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询（stats 5000 / 账号 10000 / 其余按 Tab 各自节奏）实测维持，全部手动 refetch 出口 + invalidateQueries 到位。子 Tab 切换不销毁查询组件实例，轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言「路由组件顶层无裸 cd.* 消费（0 处）」实测继续绿但量纲较粗（正则过滤 memo/Matrix/props 行后不量条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装 candidate 维持候选**：Button.tsx 本体 `grep -c "<button"` = 0（组件内部不直接写裸 button，属包装 React.forwardRef 的抽象），OBSERVE-93-01 17 处 `<button` 全在消费侧（Admin/Dashboard/Login），语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）、构建通过（vite v8.3.0，产物与 R145/R143/R144 同尺寸同哈希）、XSS 面零命中、工作区零漂移、F93-01 第五十四轮双空实证、Select.tsx 与 targetGuard.ts 双文件与 68c0bfc 逐字节相同。两条新契约纵深（管理令牌五路径成对全清 + 401 吊销切号整体重建 + 删除账号迁移 effect 对称 / client.ts 状态码机 + Admin 三态展示 + 倒计时四位同源 + 折叠键盘路径）均无发现。维持观察项不构成合并阻塞，无需本轮修复。
