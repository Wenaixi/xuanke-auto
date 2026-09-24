# R150 前端审查报告（M-1）——第八十六次核验

基线：df1f5ce（R149 归档，M-1 第八十五轮闭合） | 审查时间：2026-09-24 | 审查代理：R150 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。四条维持观察项依旧维持（Select 注释口径残留自 R124 起延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选维持候选）。两条新契约纵深（Admin 复制降级链 + 可访问性基线全家福 / 倒计时 begin_times 兜底四位同源）均无发现。建议 **APPROVE**。

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
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css index-1KHlpqcc.css 41.72 kB（与 R149/R148/R147/R146 同哈希同尺寸——R144 后 web/ 零变更连续佐证，本轮又得一轮） | 2026-09-24 |
| target 守卫 | `cd web && npx tsx scripts/target-guard-check.ts` | 18 断言全绿（8 defer + 5 stale + 5 clean），输出末行"target-guard 断言全绿"，exit 0 | 2026-09-24 |
| perf 守卫 | `cd web && npx tsx scripts/perf-countdown-guard.ts` | 3 断言全绿（自 tick / memo 叶子 / 顶层 0 裸消费），exit 0 | 2026-09-24 |
| admin-auth 守卫 | `cd web && npx tsx scripts/admin-auth-check.ts` | 6 断言全绿（会话一致/撞名学生/吊销/无标记/普通学生/自定义管理员名），exit 0 | 2026-09-24 |
| unauthorized 守卫 | `cd web && npx tsx scripts/unauthorized-check.ts` | 5 断言全绿（居中/末尾/无参数/非 query/空路径），exit 0 | 2026-09-24 |
| 视觉审计 | `cd web && node scripts/audit.mjs` | 全部断言全绿（exit 0），"全部通过，视觉表面协调一致" | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src` | 全仓零命中（0 行） | 2026-09-24 |
| 工作区 | `git status --short` + `git diff --stat` | 双空（零改动零差异），本报告为唯一写文件 | 2026-09-24 |
| git 双空 | `git log --oneline df1f5ce..HEAD -- web/`（0 行）+ `git diff --stat df1f5ce..HEAD -- web/`（0 行） | 双空实证（F93-01 第五十八轮） | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(" web/src/routes/Select.tsx` | 调用级恰 4（:509/:597/:605/:699），grep 全量含注释恰 10 | 2026-09-24 |
| 原生 button | `grep -c "<button" web/src/routes/*.tsx` | 17 处 3 文件（Admin 13 / Dashboard 2 / Login 2），零增零减 | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 第八十六轮闭合（保存链守卫） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）逐字符核验，判据本体三行与契约逐行一致：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

**恰 4 消费点**（调用级 `grep -c "shouldDeferSave("` 实测 = 4；全量 grep 恰 10 含注释 6 处）逐字符一致（第一参数均为最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门，前置 `revRef.current > 0` 条件）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位**：写点恰 3 处——:200 `= false`（账号切换复位守卫，严格置 false 不触第四处写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。发布重建独立清理 effect（:318-329）只读不写 echoedRef，全仓 grep 确认 `echoedRef.current =` 赋值形态仅 :200/:240/:297 三处，读点兜底守卫同帧。

**首帧四边界**：:229（`if (echoedRef.current) return` 短路）、:234（`if (stateData === undefined) return` 绝不提前置位回显完成）、:247（`if (pubs.length === 0) return`，effect 声明于 const publishes 之前、TDZ 由 data 自推导的安全注释确认）、:319（清理 effect `if (!echoedRef.current || publishes.length === 0) return` 未回显短路）四处全部在位（行号文本逐行核对）。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本「无变更 → 返回原引用」断言（第 18 条）绿。双路径到位：回显 effect 内残留清理（:290-295）+ 发布重建独立清理 effect（:318-329，selected 加入依赖防交错时序 :307-313 注释确认）。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 有选中 + 未回显 → 推迟」（true）与「courses 非空 + 全清空 → 放行」（false）成对确证；第四参数 echoed 两向「已回显稳态 → 放行」「首帧未到 + 已回显 → 仍推迟」成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，`revRef.current > 0` 才判）+ 5s 等待 while（:605，轮询 50ms）双闸齐备。防抖回调尾段消费时刻双闸（:732 联查空 / :737 发布 id 漂移）与 flush 侧守卫族五层（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）本轮再次全核对。dirtyRef 写点实测仍仅 :455（saveNow catch 真实失败）+ :563/:742（飞行中标记补发），守卫路径绝不置 dirtyRef，终局 toast 只归 pendingUnsaved 三信号（:647）。防抖 effect 依赖数组（:755 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`）完好，echoDone + stateData 双自愈信号在位。

### 2. OBSERVE-93-01 第四十轮 ✅ 维持

`grep -c "<button"` 逐文件实测 = 17 处零增零减（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2），与 R149 记录逐项一致。651/661 候选实测行号吻合（:651 硅基流动 Vision / :661 本地 ddddocr），语义完整（引擎二选按钮，选中态白底黑字，不可选项缺失不影响语义读屏——无 JS 防呆则无 aria-pressed 也未成问题）。另 :577 激活码机制开关已带 `role="switch" + aria-checked + aria-label="激活码机制"`（:578-580）。每处裸 button 语义完整核对：Dashboard CollapseSection 折叠段带 aria-expanded/aria-controls/useId（:67-70/:64）、Login 密码可见性切换带 aria-label/aria-pressed（:171-172）、Admin 复制/删除按钮带 aria-label（:470/:473）。维持候选名义，无新增裸渲面。

### 3. F93-01 第五十八轮双空实证 ✅ 实证

`git log --oneline df1f5ce..HEAD -- web/` 输出空、`git diff --stat df1f5ce..HEAD -- web/` 输出空两路实证。R149 归档（df1f5ce）后 web/ 零提交零差异。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第三十四轮 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-7（「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」全文读取实测）、Dashboard.tsx:90-94（CountdownMatrix memo 组件头注释「由『拆 memo 叶子组件潜在优化』转为落地实现，注释口径同步」）、Dashboard.tsx:119-121（relativeCountdown 注释）、:206-212（整页 setTick 已删注释）四处同源。自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致）：
- Select /electives（:76-81）：error→30000 / window_closed→30000 / inRange||window_opened→2000 / 常态→10000
- Select /state（:153-154）：error→30000 / window_closed→30000 / 常态→2000
- Dashboard /state（:169-174）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /logs（:185-190）：error→30000 / window_closed→30000 / 常态→3000
- Dashboard /electives（:200-204）：恒 30000（后端快照 TTL 40s > 30s）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R149 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R149 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 三处落位实测（Select:1234 取消 / Login:267 激活码输入 / Admin:241 取消），grep 三路由 = 3 处无遗漏无多增。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵 memo 化（:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`，源码 :95-116 全文读取确认四格叶子只依赖四个数字 props），② Select 顶栏该处 :844-852 单行文本插值、DOM 差分成本可忽略，③ useTickingCountdown 每秒 setNow 语义未改。守卫盲区契约零漂移：perf-countdown-guard 断言「路由组件顶层无裸 cd.* 消费（当前 0 处，应为 0）」实测绿——断言只量「裸消费」（正则过滤 memo/Matrix/props 行后不量条件渲染内 cd.* 文本插值如 Select:850），Select:850 属条件渲染内文本插值、非裸 `<cd.x>` 直渲，断言保持绿。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：Admin 复制降级链（clipboard → execCommand → toast 全链）**

- 主路径（Admin.tsx:76-80）：`navigator.clipboard.writeText` 可用直接复制 → setCopied + 1500ms 定时器清反馈（:78-82，定时器句柄先 clearTimeout 防旧定时器清新码）
- 降级路径（:83-99）：clipboard API 整体不可用（非安全上下文 http://内网 / iframe 嵌入 / 权限被拒）时抛错入 catch → 手动构造 `document.createElement("textarea")`（`readOnly=true` 加固防可编辑区干扰选中复制，选中/复制成功同步移除）→ `document.execCommand("copy")` 兜底
- 双失败终局（:100-107）：execCommand 也失败 → toast「复制失败，请手动抄录」并把完整激活码展示给管理员抄录（激活码整链不失效的信息兜底）
- 兜底成功反馈（:88-93）：execCommand 成功同样 setCopied + 定时器清反馈，与主路径统一
- 定时器清理（:116 附近 `useEffect(() => () => ...)`）：组件卸载时清理挂起复制反馈定时器（切 Tab/退出不 setState），与卸载后不轰炸族契约同源。全链无发现

**纵深 B：可访问性基线复核（aria 标注 + 弹窗语义族 + useId 唯一性）+ 倒计时 begin_times 兜底四位同源**

- aria/role 全仓盘点：Select 搜索框 aria-label（:870）、Login 密码切换 aria-label/aria-pressed（:171-172）、Admin 复制/删除 aria-label（:470/:473）、Admin 激活码机制 role="switch" + aria-checked（:578-580）、Admin 账号列表 `<table aria-label="账号列表">`（:833）、Dashboard CollapseSection aria-expanded/aria-controls + useId（:67-70/:64）。Tabs.tsx 内部由 Radix 自带 tablist/tab 语义（grep 注释零命中属代理组件内建，非缺标）。三 Dialog 全部带 role=+aria-modal+aria-labelledby，useId 无重复 id 风险
- 倒计时 begin_times 兜底四位同源（F39-N1）全核对：
  - Select 顶栏横幅（:767-771 cd 构造 + :839 `!openTimeStr && data?.begin_times?.[0] == null` 空态判定 + :858-860 右侧时间戳文本兜底）三处吃同一 data.begin_times[0] 兜底
  - Dashboard 主倒计时（:222-227 cd 构造）+ 文案行（:410-416）+ 折叠段 primaryMs（:237-239）三处吃同一 electives.begin_times[0] 兜底
  - 两路由共六位全部「识别真值优先、兜底只在识别缺席时生效」，且 Select 用 `data?.begin_times?.[0]`（/electives?account= 穿透）、Dashboard 用 `electives?.begin_times?.[0]`（同 key 同 URL 的 react-query 共享缓存，:195-196 注释确认共享零重复请求），两路由兜底源语义同归一
  - 注释口径两路由一致（Select:765-766「与 Dashboard 同构」、Dashboard:219-221「避免主矩阵未知但折叠列表有值……识别槽建立后仍以识别真值为准」）。无发现

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:850），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询（codes 5000 / config 5000 / stats 10000 / accounts 5000 / logs 5000）维持，全部手动 refetch 出口（9 处）+ invalidateQueries（删除后立即失效）到位。子 Tab 切换不销毁查询组件实例，轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言「路由组件顶层无裸 cd.* 消费（0 处）」实测继续绿但量纲较粗（正则过滤 memo/Matrix/props 行后不量条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装 candidate 维持候选**：Button.tsx 本体 `grep -c "<button"` = 0（组件内部通过 React.forwardRef 用 Slot 抽象输出 button 元素），OBSERVE-93-01 17 处 `<button` 全在消费侧（Admin/Dashboard/Login），语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）+ 视觉审计全绿、构建通过（vite v8.3.0，产物与 R149/R148/R147/R146 同尺寸同哈希）、XSS 面零命中、工作区零漂移、F93-01 第五十八轮双空实证（df1f5ce 后 web/ 零提交零差异）。两条新契约纵深（Admin 复制降级链 clipboard→execCommand→toast 全链 / 可访问性基线复核 + 倒计时 begin_times 兜底四位同源）均无发现。维持观察项不构成合并阻塞，无需本轮修复。