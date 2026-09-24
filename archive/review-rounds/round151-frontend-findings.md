# R151 前端审查报告（M-1）——第八十七次核验

基线：6045db1（R150 归档，M-1 第八十六轮闭合） | 审查时间：2026-09-24 | 审查代理：R151 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。四条维持观察项依旧维持（Select 注释口径残留自 R124 起延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选维持候选）。两条新契约纵深（401 吊销切号 key={account} 整体重建 / 管理令牌五路径成对全清成族核查）均无发现——key={account} 挂载点双侧实测在位，isCurrentAdminSession 六判定路径的六对 setAdminToken+saveAdminToken 清标记成对落位且各有语义注释。建议 **APPROVE**。

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
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css index-1KHlpqcc.css 41.72 kB（与 R150/R149/R148/R147/R146 同哈希同尺寸——R144 后 web/ 零变更连续佐证，本轮又得一轮） | 2026-09-24 |
| target 守卫 | `cd web && npx tsx scripts/target-guard-check.ts` | 18 断言全绿，输出末行"target-guard 断言全绿"，exit 0 | 2026-09-24 |
| perf 守卫 | `cd web && npx tsx scripts/perf-countdown-guard.ts` | 3 断言全绿，exit 0 | 2026-09-24 |
| admin-auth 守卫 | `cd web && npx tsx scripts/admin-auth-check.ts` | 6 断言全绿，exit 0 | 2026-09-24 |
| unauthorized 守卫 | `cd web && npx tsx scripts/unauthorized-check.ts` | 5 断言全绿，exit 0 | 2026-09-24 |
| 视觉审计 | `cd web && node scripts/audit.mjs` | 全部断言全绿（exit 0），"全部通过，视觉表面协调一致" | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中（0 行） | 2026-09-24 |
| 工作区 | `git status --short` + `git diff --stat` | 双空（零改动零差异），本报告为唯一写文件 | 2026-09-24 |
| git 双空 | `git log --oneline 6045db1..HEAD -- web/`（0 行）+ `git diff --stat 6045db1..HEAD -- web/`（0 行） | 双空实证（F93-01 第五十九轮） | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" web/src/routes/Select.tsx` | 调用级恰 4（:509/:597/:605/:699），grep 全量含注释恰 10（R150 同值重新实测） | 2026-09-24 |
| 原生 button | `grep -c "<button" web/src/routes/*.tsx` | 17 处 3 文件（Admin 13 / Dashboard 2 / Login 2），零增零减 | 2026-09-24 |
| dirtyRef 写点 | `grep -c "dirtyRef.current = true" web/src/routes/Select.tsx` | 恰 3（:455 真实失败 / :563/:742 飞行中标记补发），守卫路径绝不置 dirtyRef | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 第八十七轮闭合（保存链守卫） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）全文读取逐字符核验，判据本体三行与契约逐行一致：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟（含已回显标志仍在首帧未到时推迟，第四参数 echoed 不推翻：回显未发生时 selected 只含用户新改动）
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死；第三参数 echoed 引入的语义确认）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

**恰 4 消费点**（调用级 `grep -c "shouldDeferSave("` 实测 = 4；全量 grep 恰 10 含注释 6 处）逐字符一致（第一参数均为最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门，前置 `revRef.current > 0` 条件）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位**：写点恰 3 处——:200 `= false`（账号切换复位守卫，严格置 false 不触第四处写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。发布重建独立清理 effect（:318-329）只读不写 echoedRef，全仓 grep 确认 `echoedRef.current =` 赋值形态仅 :200/:240/:297 三处；echoDone state 仅由 :241/:298 与回声 effect 同步置位（:239 注释确认"echoDone 只做放行信号，不写 selected"）。

**首帧四边界**：:229（`if (echoedRef.current) return` 短路）、:234（`if (stateData === undefined) return` 绝不提前置位回显完成）、:247（`if (pubs.length === 0) return`，effect 声明于 const publishes 之前、TDZ 由 data 自推导的安全注释确认）、:319（清理 effect `if (!echoedRef.current || publishes.length === 0) return` 未回显短路）四处全部在位（行号文本逐行核对）。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本「无变更 → 返回原引用」断言（第 18 条）绿。三消费点核对：回显 effect 内残留清理（:290-295）+ 发布重建独立清理 effect（:318-329 + 原引用短路时无重渲）+ 防抖/flush 消费时刻与 selectedHasStalePublish 同族守卫（:526/:717）。stale 全量消费点实测恰 4（:289/:321/:526/:717）+ 导入 1 处 = 5，无第 5 处语义调用。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 有选中 + 未回显 → 推迟」（true）与「courses 非空 + 全清空 → 放行」（false）成对确证；第四参数 echoed 两向「已回显稳态 → 放行」「首帧未到 + 已回显 → 仍推迟」成对确证（首帧未到优先级高于回显标志，回显未发生时整包覆盖风险仍在）。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，`revRef.current > 0` 才判）+ 5s 等待 while（:605，轮询 50ms）+ 合并后一帧落地（:609）双闸齐备。防抖回调尾段消费时刻双闸（:732 联查空 / :737 发布 id 漂移）与 flush 侧守卫族五层（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）本轮再次全核对。dirtyRef 写点实测仍恰 3（:455 saveNow catch 真实失败 + :563/:742 飞行中标记补发），守卫路径绝不置 dirtyRef。防抖 effect 依赖数组（:755 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`）完好，echoDone + stateData 双自愈信号在位。

### 2. OBSERVE-93-01 第四十一轮 ✅ 维持

`grep -c "<button"` 逐文件实测 = 17 处零增零减（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R150 记录逐项一致。651/661 候选实测行号吻合（:651 硅基流动 Vision / :661 本地 ddddocr），语义完整（引擎二选按钮，选中态白底黑字，不可选项缺失不影响语义读屏——无 JS 防呆则无 aria-pressed 也未成问题）。另 :577 激活码机制开关已带 `role="switch" + aria-checked + aria-label="激活码机制"`。其余裸 button 语义完整核对：Dashboard CollapseSection 折叠段带 aria-expanded/aria-controls/useId（:67-70/:64，CollapseSection 组件体 :61-88 全文读取确认 id 由 useId 唯一生成并落到正文 div）、Login 密码可见性切换带 aria-label/aria-pressed（:171-172）、Admin 复制/删除按钮带 aria-label（:470/:473）。维持候选名义，无新增裸渲面。

### 3. F93-01 第五十九轮双空实证 ✅ 实证

`git log --oneline 6045db1..HEAD -- web/` 输出空、`git diff --stat 6045db1..HEAD -- web/` 输出空两路实证。R150 归档（6045db1）后 web/ 零提交零差异。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第三十五轮 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:1-40 全文读取实测（:3-8「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」）、Dashboard.tsx:90-95（CountdownMatrix memo 组件头注释）、:119-121（relativeCountdown 注释）、:206-212（整页 setTick 已删注释）四处同源。自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致，全部 refetchInterval 源码原文读取）：
- Select /electives（:58-82）：error→30000（:76）/ window_closed→30000（:80）/ inRange||window_opened→2000（:81 三元尾）/ 常态→10000（:81）
- Select /state（:148-155）：error→30000（:153）/ window_closed→30000（:154）/ 常态→2000（:154）
- Dashboard /state（:169-174）：error→30000（:171）/ window_closed→30000（:172）/ 常态→3000（:174）
- Dashboard /logs（:185-190）：error→30000（:186）/ window_closed→30000（:188）/ 常态→3000（:190）
- Dashboard /electives（:200-204）：恒 30000（后端快照 TTL 40s > 30s 注释 :196-199 确认）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R150 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R150 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`，注释 :1199-1200 确认退选中不响应防误关）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 三处落位实测（Select:1234 取消 / Login:267 激活码输入 / Admin:241 取消），grep 三路由 = 3 处无遗漏无多增。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵 memo 化（:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`，源码 :95-116 全文读取确认四格叶子只依赖四个数字 props），② Select 该处 :844-853 单行文本插值、DOM 差分成本可忽略，③ useTickingCountdown 每秒 setNow 语义未改。守卫盲区契约零漂移：perf-countdown-guard 脚本第 3 条断言（:34-37）只量 Dashboard.tsx 的裸 `cd.*` 行消费、正则过滤 memo/MemoCountdownMatrix/props/countdownmatrix 行——实测绿（当前 0 处），断言不量条件渲染内 Select:850 文本插值属既有盲区维持。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：401 吊销切号整体重建（key={account} 双侧挂载点 + 账号复位守卫）**

- key={account} 双侧挂载点：Select.tsx 两处消费在 App.tsx——代理分支（:293-294 `key={targetAccount}`，管理员代看学生大厅）+ 学生分支（:339-340 `key={current}`，主挂载）。Key 变化 = React 整体卸载旧实例重建新实例，Selected/echoedRef/rev/echoDone 全部随实例复位，前后端账号永不串线。
- 账号复位兜底守卫（Select.tsx:195-202）：跨账号实例复用防护——App 已加 key 的前提下此守卫为二次防线（注释 :190-194 明确"兜底未来改为不重置挂载的意外回归"），account 变化即 `setSelected({}) + setRev(0) + echoedRef.current = false + setEchoDone(false)`。声明顺序在 echoedRef/rev/setRev/setEchoDone 之后（:194 TDZ 注释确认）。
- 401 吊销链闭环（App.tsx:187-231 onUnauthorized）：吊销事件反查归属账号（:194-197 先账号名直查再令牌反查）→ 代理态退出（:204-206，含管理员自身会话被吊销对称）→ 管理员自身被吊销退出管理态 + 清标记（:210-215）→ 管理员代理态看到的 401 绝不误杀管理员会话（:217-219 guard）→ 快照式三连删除（:225-230，setSessions updater 无副作用注释 :220-224）。会话剔除后 accounts 变化驱动 effect（:135-148）自动切剩余账号 / 回登录页，且 page 复位 dashboard（:143-144）——从吊销到切号再到大厅重建链路闭合。
- 双向消费 `?account=` 全通路（App:293/339 key 值即 account prop）：Select /electives+targets+state 三查询 queryKey 均含 account（:56/:146，Dashboard :160/:200 同源），同学段内并发刷零串线。无发现。

**纵深 B：管理令牌五路径成对清标记（isCurrentAdminSession 纯函数成族核查）**

- isCurrentAdminSession 纯函数本体（web/src/lib/adminAuth.ts:6-12）：`adminToken !== "" && sessions[adminName] === adminToken`——判定唯一依据 =「曾标记为管理会话的 token」与「当前管理员名账号的会话 token」一致（:11）。撞名学生（账号名恰等于管理员名、放行的普通教务会话）token 永远匹配不上，注释 :2-5 + TDD 守护脚本 admin-auth-check 六断言全绿。全仓消费点恰 6 处（grep 实测）：
  - App.tsx:147（账号迁移/清空 effect）——管理态复位判据
  - App.tsx:217（401 处理器）——管理员代理态误杀防护
  - App.tsx:299（渲染分支）——inAdmin || isCurrentAdminSession 双判决定 Admin 分支
- 五条路径的 setAdminToken+saveAdminToken 清标记成对：
  - login（:107 setInAdmin(!!adminName)，仅响应带 adminName 才写 token）
  - logout（:118-122，登出即清，注释 :119-120 确认残留不影响判定）
  - onDeleted（:172-177，删除管理员自己 + 清标记）
  - onUnauthorized（:210-215，管理员被吊销 + 清标记）
  - onBackToStudent（:309-313，切回学生端 + 清标记）
- 五条清标记路径全成对实测落位（setAdminToken + saveAdminToken 均相邻同步调用，各自伴随语义注释），更新路径（login :98-99 带 adminName 才写）与更新专用恢复注释（:82-83 刷新恢复唯一判据）齐备。无发现。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:850），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询（codes 5000 :319 / config 5000 :726 / stats 10000 :820 / accounts 5000 :915 / logs 5000）维持，全部手动 refetch 出口（9 处：:336/:357/:441/:452/:546/:701/:792/:898/:944）+ invalidateQueries（:262 删除后立即失效）到位。子 Tab 切换不销毁查询组件实例，轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言（:34-37）实测继续绿但量纲较粗（只量 Dashboard.tsx 裸 cd.* 行消费、正则过滤 memo 系列标签，完全不量条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装 candidate 维持候选**：Button.tsx 本体 `grep -c "<button"` = 0（组件内部通过 React.forwardRef 用 Slot 抽象输出 button 元素，:31-51 全文读取确认），OBSERVE-93-01 17 处 `<button` 全在消费侧（Admin/Dashboard/Login），语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）+ 视觉审计全绿、构建通过（vite v8.3.0，产物与 R150 至 R146 连续五轮同尺寸同哈希）、XSS 面零命中、工作区零漂移、F93-01 第五十九轮双空实证（6045db1 后 web/ 零提交零差异）。两条新契约纵深（401 吊销切号 key={account} 整体重建双侧挂载 / 管理令牌五路径成对清标记成族核查）均无发现。维持观察项不构成合并阻塞，无需本轮修复。