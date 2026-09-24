# R152 前端审查报告（M-1）

基线：6768bc4（R151 归档，前端上轮闭合） | 审查时间：2026-09-24 | 审查代理：R152 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM 级别发现。全部聚焦清单项在位（✅），无任何漂移。四条维持观察项依旧维持（Select.tsx:204-207 注释口径残留自 R124 起延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选维持候选）。两条新契约纵深（目标保存 flush 出口收敛 / Admin 复制降级链 + 手动操作在飞守卫 + 删除账号迁移 effect 对称成族核查）均无发现。建议 **APPROVE**。

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
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 index-27qti0_B.js 420.90 kB / css index-1KHlpqcc.css 41.72 kB（与 R151 至 R146 连续六轮同哈希同尺寸——R144 后 web/ 零变更连续佐证再得一轮） | 2026-09-24 |
| target 守卫 | `cd web && node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿，输出末行"target-guard 断言全绿"，exit 0（无 tsx，node_modules 已含 jiti，脚本头注释 :8 明确该用法为等价路径） | 2026-09-24 |
| perf 守卫 | `cd web && node --import jiti/register scripts/perf-countdown-guard.ts` | 3 断言全绿，exit 0 | 2026-09-24 |
| admin-auth 守卫 | `cd web && node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿，exit 0 | 2026-09-24 |
| unauthorized 守卫 | `cd web && node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿，exit 0 | 2026-09-24 |
| 视觉审计 | `cd web && node scripts/audit.mjs` | 全部断言全绿（exit 0），"全部通过，视觉表面协调一致" | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src/` | 源码零命中（0 行；全仓粗扫 6 行命中全部来自 backend/web/dist 构建产物与 backend.exe 二进制，非源码） | 2026-09-24 |
| 工作区 | `git status --short` | 仅未跟踪 `archive/review-rounds/round152-backend-findings.md`（后端代理写文件，非本次审查所致）；web/ 相关零改动 | 2026-09-24 |
| git 双空 | `git log --oneline 6768bc4..HEAD -- web/`（0 行）+ `git diff --stat 6768bc4..HEAD -- web/`（0 行） | 双空实证 | 2026-09-24 |
| 倒计数 | `grep -c "shouldDeferSave(stateDataRef" web/src/routes/Select.tsx` | 调用级恰 4（:509/:597/:605/:699），全量 grep 含注释恰 11（R151 同值重新实测） | 2026-09-24 |
| 原生 button | `grep -c "<button" web/src/routes/*.tsx web/src/components/ui/*.tsx` | 17 处 3 文件（Admin 13 / Dashboard 2 / Login 2），Select 0 / Button.tsx 0，零增零减 | 2026-09-24 |
| dirtyRef 写点 | `grep -c "dirtyRef.current = true" web/src/routes/Select.tsx` | 恰 3（:455 真实失败 / :563/:742 飞行中标记补发），守卫路径绝不置 dirtyRef | 2026-09-24 |
| stale 消费点 | `grep -n "selectedHasStalePublish" web/src/` | 语义调用恰 4（:289 回显 effect 残留清理 / :321 发布重建独立清理 / :526 flush 消费时刻 / :717 防抖消费时刻）+ 定义 1 + 导入 1，无第 5 处语义调用 | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 保存链守卫闭合（shouldDeferSave） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）全文读取逐字符核验，判据本体三行与契约逐行一致：
- :69 `if (stateData === undefined) return true` — 首帧未到无条件推迟（含已回显标志仍在首帧未到时推迟，第三参数 echoed 不推翻：回显未发生时 selected 只含用户新改动）
- :70 `if (echoed) return false` — 已回显稳态放行（编辑不闷死）
- :71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected` — courses 非空且有选中才推迟；全清空（hasSelected=false）放行 PUT []

**恰 4 消费点**（调用级 `grep -c "shouldDeferSave(stateDataRef"` 实测 = 4）逐字符一致（第一参数均为最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门，前置 `revRef.current > 0` 条件）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位**：写点恰 3 处——:200 `= false`（账号切换复位守卫，严格置 false 不触第四处写 true）、:240 `= true`（首帧 courses 空确证后端无旧目标）、:297 `= true`（首帧合并完成）。全仓 grep `echoedRef.current =` 赋值形态确认仅此三处，发布重建独立清理 effect（:318-329）只读不写 echoedRef。echoDone 置位亦恰 3（:201/:241/:298，与 echoedRef 置位一一对应）。

**首帧四边界**：:229（`if (echoedRef.current) return` 短路）、:234（`if (stateData === undefined) return` 绝不提前置位回显完成）、:247（`if (pubs.length === 0) return`，effect 声明于 const publishes 之前、TDZ 由 data 自推导的安全注释确认）、:319（清理 effect `if (!echoedRef.current || publishes.length === 0) return` 未回显短路）四处全部在位（行号文本逐行核对）。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认，守卫脚本「无变更 → 返回原引用」断言（第 18 条）绿。三消费点核对：回显 effect 内残留清理（:290-295，含 toast 提示）+ 发布重建独立清理 effect（:318-329 + 原引用短路时无重渲）+ 防抖/flush 消费时刻与 selectedHasStalePublish 同族守卫（:526/:717）。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿，关键两向「courses 非空 + 有选中 + 未回显 → 推迟」（true）与「courses 非空 + 全清空 → 放行」（false）成对确证；第三参数 echoed 两向「已回显稳态 → 放行」「首帧未到 + 已回显 → 仍推迟」成对确证（首帧未到优先级高于回显标志，回显未发生时整包覆盖风险仍在）。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597，`revRef.current > 0` 才判）+ 5s 等待 while（:605，轮询 50ms）+ 合并后一帧落地（:609）双闸齐备。防抖回调尾段消费时刻双闸（:732 联查空 / :737 发布 id 漂移）与 flush 侧守卫族五层（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）全核对。dirtyRef 写点实测仍恰 3（:455 saveNow catch 真实失败 + :563/:742 飞行中标记补发），守卫路径绝不置 dirtyRef。防抖 effect 依赖数组（:755 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`）完好，echoDone + stateData 双自愈信号在位。

### 2. OBSERVE-93-01 第四十二轮 ✅ 维持

`grep -c "<button"` 逐文件实测 = 17 处零增零减（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2 / Select.tsx 0 / Button.tsx 0），与 R151 记录逐项一致。651/661 候选实测行号吻合（:651 硅基流动 Vision / :661 本地 ddddocr），语义完整（引擎二选按钮，选中态白底黑字，不可选项缺失不影响语义读屏）。:577 激活码机制开关已带 `role="switch" + aria-checked + aria-label="激活码机制"`。其余裸 button 语义完整核对：Dashboard CollapseSection 折叠段带 aria-expanded/aria-controls/useId（:67-70/:64，CollapseSection 组件体 :50-84 全文读取确认 id 由 useId 唯一生成并落到正文 div :82）、Login 密码可见性切换带 aria-label/aria-pressed（:171-172）、Admin 复制/删除按钮带 aria-label（:470/:473）。维持候选名义，无新增裸渲面。

### 3. F93-01 第六十轮双空实证 ✅ 实证

`git log --oneline 6768bc4..HEAD -- web/` 输出空、`git diff --stat 6768bc4..HEAD -- web/` 输出空两路实证。R151 归档（6768bc4）后 web/ 零提交零差异。Select 倒计时候选维持不实现。

### 4. OBSERVE-116-01 第三十六轮 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-8（「hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件重渲染……消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」）、Dashboard.tsx:206-212（整页 setTick 已删注释）、:229（折叠行实时标签由主矩阵 tick 驱动）、Select.tsx:204-207（倒计时注释）四处同源。自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致，全部 refetchInterval 源码原文读取）：
- Select /electives（:58-82）：error→30000（:76）/ window_closed→30000（:80）/ inRange||window_opened→2000（:81 三元尾）/ 常态→10000（:81）
- Select /state（:148-155）：error→30000（:153）/ window_closed→30000（:154）/ 常态→2000（:154）
- Dashboard /state（:169-174）：error→30000（:170-171）/ window_closed→30000（:172-173）/ 常态→3000（:174）
- Dashboard /logs（:185-190）：error→30000（:186-187）/ window_closed→30000（:188-189）/ 常态→3000（:190）
- Dashboard /electives（:200-204）：恒 30000（后端快照 TTL 40s > 30s 注释 :196-199 确认）

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R151 一致

三处最小语义门行号实测：Select.tsx:1204（`role="dialog"` 退选确认）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认），与 R151 记录完全一致。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby 关联标题（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫：Select:1208（`!actionLoading.has(exitModalClass.id)`，注释 :1199-1200 确认退选中不响应防误关）、Login:233（`!activating`）、Admin:217（`!deleting`）。autoFocus 三处落位实测（Select:1234 取消 / Login:267 激活码输入 / Admin:241 取消）。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（:844-853 单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵 memo 化（:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`），② Select 该处 :844-853 单行文本插值、DOM 差分成本可忽略，③ useTickingCountdown 每秒 setNow 语义未改。守卫盲区契约零漂移：perf-countdown-guard 脚本第 3 条断言（:34-37）只量 Dashboard.tsx 的裸 `cd.*` 行消费、正则过滤 memo/MemoCountdownMatrix/props/countdownmatrix 行——实测绿（当前 0 处），断言不量条件渲染内 Select:850 文本插值属既有盲区维持。维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：目标保存 flush 出口收敛（flushTargets 五层守卫族 + handleBack 三轮收敛 + 退避 timer 在飞等待）**

- flushTargets 守卫族五层全核对：:515 发布缺席（`publishesRef.current.length === 0 && latestSelectedCount > 0` → 保留脏绝不 PUT []）、:526 stale（toast 兜底提示恢复路径给用户「刷新后重新选择」）、:553 联查空、:558 发布 id 漂移、:509 shouldDeferSave 回显守卫在最前（:498-511 注释链确认「/state 首帧持续失败超时后，守卫在这里兜住：置脏跳过、不 PUT，脏块保留在内存 selected（守卫不置 dirtyRef——终局绝不误报保存失败）」）。
- handleBack 三轮收敛（:611-642）：`flushedRev` 快照比对判静止（:621-624）、`pendingSaving()` 三信号合一（:633 `savingRef.current || retryState.current.timer !== null`）等待退避 timer 在飞（:635-639，注释 :627-632 确认「退避重试 timer 排队中同样表示内存与后端分叉、改动未落库」——此前只等 savingRef 的漏网路径已补）、21s 超时兜底绝不无限挂起。终局 toast（:647-653）只对「rev>0 且 pendingUnsaved 三信号」弹、守卫拦下的假清空脏块绝不误报。
- 补发窗口关断（:568-574 注释契约）：先 flush、再等飞行中 PUT 结束（其 finally 会在卸载前自动补发最新快照），直到保存链静止才真正卸载。api 20s 超时兜底。无发现。

**纵深 B：Admin 复制降级链 + 手动操作在飞守卫 + 删除账号迁移 effect 对称**

- 复制降级链（Admin.tsx:67-104）：主路径 `navigator.clipboard.writeText`（:75-78，非安全上下文/iframe/权限拒绝抛错落入 catch）→ 兜底手动构造 textarea + `document.execCommand("copy")`（:86-94，`readOnly` 固化防软键盘/选择异常，:83-84 注释「text area 仅内存驻留不入 DOM 树」语义为 appended then removed）→ 双失败才提示并展示完整激活码供管理员抄录（:99-104）。无发现。
- 手动操作在飞守卫（Select.tsx:48-52）：actionLoading 为 `ReadonlySet<number>` 按课程 id 独立跟踪（注释 :48-51 确认「单值 actionLoading 被并发不同课程操作互相覆盖」的缺陷形态），消费点全核对——:92/:119 报名退选入口短路、:1120/:1133 渲染 disabled、:1139/:1126 按钮文案在飞态、:1208 Esc 防误关、:1232/:1242/:1245 退选弹窗按钮。finally 函数式清除只删自己的 id（:106-110）。无发现。
- 删除账号迁移 effect 对称（App.tsx:160-177 onDeleted）：删管理员自己 → `setInAdmin(false)` + `setAdminToken("")` + `saveAdminToken("")` 三连清（:172-177，与 logout/onUnauthorized/onBackToStudent 四路径对称，注释 :169-171 确认残留 inAdmin 会让账号迁移 effect 切到学生后用学生令牌渲染 Admin 五 Tab 连环 401）；删当前查看账号 → `setCurrent("")`（:168）；删代理账号 → `setTargetAccount(null)`（:167）；且绝不调 logout()（注释 :157-159 语义：删除的是他人账号、用管理员令牌调 /logout 会登出管理员自己）。onBackToStudent（:306-325）`others` 筛选 + 无学生账号时完整登出（清空 sessions + current → 渲染落到登录页且 effect 不再弹回）。无发现。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），且 Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:850），注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口**：五 Tab 查询轮询（codes 5000 :319 / config 5000 :726 / stats 10000 :820 / accounts 5000 :915 / logs 5000）维持，全部手动 refetch 出口 + invalidateQueries 到位。子 Tab 切换不销毁查询组件实例，轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：perf-countdown-guard 第 3 条断言（:34-37）实测继续绿但量纲较粗（只量 Dashboard.tsx 裸 cd.* 行消费、正则过滤 memo 系列标签，完全不量条件渲染内的 cd.* 文本插值如 Select:850），本轮再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装候选维持候选**：Button.tsx 本体 `grep -c "<button"` = 0（组件内部通过 React.forwardRef 用 Slot 抽象输出 button 元素，:31-51 全文读取确认），OBSERVE-93-01 17 处 `<button` 全在消费侧（Admin/Dashboard/Login），语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）+ 视觉审计全绿、构建通过（vite v8.3.0，产物与 R151 至 R146 连续六轮同尺寸同哈希）、XSS 面源码零命中、web/ 工作区零漂移、F93-01 双空实证（6768bc4 后 web/ 零提交零差异）。两条新契约纵深（目标保存 flush 出口收敛 / Admin 复制降级链 + 手动操作在飞守卫 + 删除账号迁移 effect 对称成族核查）均无发现。维持观察项不构成合并阻塞，无需本轮修复。
