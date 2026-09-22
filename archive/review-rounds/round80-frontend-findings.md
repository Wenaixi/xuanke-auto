# Round 80 前端只读审查报告

基线：commit 795bb4f（R79 双 findings + 收尾总结，R79 收官 HEAD）。本轮为 **R80 前端只读审查 + M-1 延续管理（第十六轮）**，重点为 **R79 四项修复（f1d98de + d14303a）的回归复核**。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 三组守护脚本。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑），工作区基线 HEAD=795bb4f、`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**R79 四项修复逐一核证：①StatsTab data-first 次序核证成立——`s ? rows : isError ? errorCard : 加载中`（Admin.tsx:757-792），轮询瞬时失败时最近一次有效数据（窗口状态/令牌有效性）照常显示，错误卡仅在后端从未成功（data 缺席）时渲染，与 Codes/Accounts/Logs 三 Tab 的 data-first 次序对称闭合；②guardBlockedRef 拆分完整性核证成立——十处守卫置脏（flush 五处 :513/:520/:532/:560/:566 + 防抖五处 :713/:724/:733/:749/:755）全部改 guardBlockedRef，dirtyRef 仅保留 saveNow catch :459 与两处"飞行中标记补发"（:571/:760）三处真实失败语义，grep 全量清点无未归类 dirtyRef 置位；pendingUnsaved() 判据与 handleBack 入口清标记时机均正确，但发现 guardBlockedRef 短路在拆分后语义冗余且引入一条极端"该弹没弹"路径（OBSERVE-80-01）；③LogsTab isLoading 短路核证成立（:922-924 分支顺序正确）；④契约 20 剥离核证成立——target-guard-check.ts:68 "R63 " 前缀已剥，全仓轮次标签族 + 行号族扫描零残留。M-1 延续管理第十六轮三消费点逐字符比对零回潮、echoedRef 置位三路径 + 首帧不置位边界完整、target-guard 18/18 全绿。本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE。构建验证全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs）。**

---

## 一、R79 修复正确性复核（commit f1d98de + d14303a）

### 1.1 Admin StatsTab data-first 次序（MINOR-79-01 修复核证）

- 位置：Admin.tsx:757-792。分支次序 `s ? rows : statsQuery.isError ? errorCard : 加载中`。
- **旧有效数据照常显示**：react-query 轮询失败时 data 保留最后一次成功值（失败不清缓存 data）→ `s` 仍持有 5s 前快照 → 直接渲染 rows。5s 轮询瞬时失败不再把有效数据整卡替换成错误卡。✅
- **错误卡仅在后端从未成功时渲染**：首帧失败（`s===undefined` + isError）→ 错误卡 + `statsQuery.refetch()` 重试。✅
- **与同族三 Tab 对称**：CodesTab（:445-483，`isLoading → isError → data&&len>0 → 空态`）、AccountsTab（:823-898，`isLoading → data&&len>0 → isError → 空态`）、LogsTab（:922-944，`isLoading → logs&&len>0 → isError → 空态`）——四 Tab 现全部 data-first 同构。✅
- ConfigTab（:687-699）为"加载提示 `!loaded && !isError` / isError 错误卡"的 error-first 形态——属"表单回填"语义差异（错误卡在加载提示前短路，保存按钮 `!loaded` 锁定），非缺陷，与 R79 修复意图（轮询型有历史数据的查询用 data-first）不冲突。
- 结论：MINOR-79-01 闭合。

### 1.2 Select.tsx guardBlockedRef 拆分完整性与正确性（MINOR-79-02 修复核证）

#### a) 十处守卫置脏全量清点 + dirtyRef 语义收敛

grep 全量清单（Select.tsx）：

| 行 | 语义 | 归类 |
|---|---|---|
| :421 | pendingUnsaved() 内 guardBlockedRef 短路 | 消费 |
| :513 / :713 | shouldDeferSave 回显未完成守卫（flush / 防抖） | guardBlockedRef ✅ |
| :520 / :724 | 发布缺席 + 已有选中 | guardBlockedRef ✅ |
| :532 / :733 | selectedHasStalePublish 残留旧发布 | guardBlockedRef ✅ |
| :560 / :749 | 联查产物为空 + 已有选中 | guardBlockedRef ✅ |
| :566 / :755 | 发布 id 漂移错位假清空 | guardBlockedRef ✅ |
| :587 | handleBack 入口清标记 | 消费 |
| :459 | saveNow catch 保存失败置脏 | dirtyRef ✅（真实失败唯一源头） |
| :571 / :760 | 保存进行中标记脏补发（flush / 防抖） | dirtyRef ✅（飞行中改动待补发） |
| :466-467 | finally 补发时清 dirtyRef | dirtyRef 消费 |

**结论**：十处守卫置脏全部改 guardBlockedRef，dirtyRef 仅保留三处真实"改动未落库"语义。**无未归类的 `dirtyRef.current = true`**（grep 实测仅 :459/:571/:760 三处，全部符合注释承诺的"保存失败置脏 + 飞行中标记补发"）。MINOR-79-02 的"守卫脏块被误归因为保存失败"闭合。

#### b) pendingUnsaved() 判据正确性

```
const pendingUnsaved = () => {
  if (guardBlockedRef.current) return false
  return dirtyRef.current || savingRef.current || retryState.current.timer !== null
}
```
- dirtyRef（真实保存失败）/ savingRef（在飞 PUT）/ timer（退避排队）三信号均为"确实未落库"，语义正确。
- guardBlockedRef 短路保证守卫拦截场景（数据缺席/发布重建/联查空/漂移）绝不弹"目标保存失败"。
- **关键发现（OBSERVE-80-01）**：守卫拆分后 dirtyRef 已精确表达"真实保存失败"，guardBlockedRef 短路在绝大多数场景是冗余防御（守卫拦截时 dirtyRef 恒 false，pendingUnsaved 本就 false）；但它同时引入一条极端"该弹没弹"路径——见 1.2d。

#### c) handleBack 入口清标记时机

- :587 `guardBlockedRef.current = false` 位于 hasSelectedNow 定义（:591）与三轮 flush 循环（:623）之前。三轮 flush 内守卫重新命中会再置位（:513/:520/:532/:560/:566）；循环结束后若仍 true 即"本次返回被守卫拦下"，终局 toast 不弹。时机正确。
- handleBack 每次调用只走一次（返回按钮 onClick 直接挂 handleBack），无重复调用问题；onDone 后组件卸载，ref 生命周期结束。
- **语义验证**：guardBlockedRef 只被 handleBack 消费（pendingUnsaved 仅在终局判定 :660 调用），入口清理保证"本次返回会话"的守卫命中才生效，历史残留（守卫命中后发布恢复、保存成功，标记残留 true 直到下次返回）在下次 handleBack 入口被清，绝不压制后续真实失败。

#### d) 关键推演逐条验证

1. **守卫拦下 + 用户返回 → 终局不弹（正确）**：flush 命中守卫置 guardBlockedRef=true，dirtyRef 恒 false → 循环第 1 轮 `!dirtyRef && !savingRef` 成立（守卫不置 dirtyRef、不调 saveNow）→ 等帧复查 revRef 无变化 → break → 终局 pendingUnsaved 短路 false → 不弹。✅
2. **真实保存失败 → 终局弹（正确）**：saveNow catch 置 dirtyRef=true + scheduleRetry → flush 后 dirtyRef=true → 不满足 :634 静止 → pendingSaving 等待退避 timer（21s 兜底）→ 超时后循环结束 dirtyRef 仍 true → guardBlockedRef false（从未命中守卫）→ 终局弹。✅
3. **守卫拦截后用户又点返回且飞行中 PUT 成功 → 终局不弹（正确）**：守卫命中 guardBlockedRef=true → 若期间另有真实保存成功，dirtyRef 被 finally 补发清零（:466-467）→ 循环退出 → pendingUnsaved 短路不弹。✅
4. **守卫命中后发布恢复、页面继续、保存成功，再返回 → 不弹（正确）**：handleBack 入口 :587 清 false → 循环 flush 守卫通过（发布已恢复）→ saveNow 成功 → dirtyRef false → 终局不弹。✅
5. **守卫命中后发布恢复、保存失败（dirtyRef=true），再返回 → 弹（正确）**：入口清 false → flush 守卫通过 → saveNow 失败 → dirtyRef true → pendingSaving 等待超时 → 终局弹。✅
6. **OBSERVE-80-01（该弹没弹）**：**同一次 handleBack 内**先守卫命中（guardBlockedRef=true）→ 循环等帧期间用户新点（rev 变化 → continue 进入下一轮）→ 下一轮 flush 守卫通过（发布恢复）→ saveNow 网络失败（dirtyRef=true）→ pendingSaving 等待 21s 超时 → 循环结束 guardBlockedRef 仍 true（第 1 轮置的，守卫通过轮不清它）→ 终局短路不弹，尽管存在真实保存失败。**触发条件**：守卫拦截 + 返回循环等待帧内用户继续点选 + 后续保存失败三条件同帧，概率极低；且 saveNow catch :452-456 已弹过"目标保存失败"红条，用户已收到失败反馈，仅终局总结提示缺失。定级 OBSERVE（信息冗余缺失，无数据风险）。

#### e) 词法陷阱核查

- **guardBlockedRef 是 ref 不触发渲染**：grep 确认消费点仅 pendingUnsaved()（:421）、入口清标记（:587）、十处置位——无 JSX/effect 依赖中误用，无渲染路径读取。✅
- **resetRetry 不清 guardBlockedRef**：语义上无需——guardBlockedRef 与重试状态正交（重试是 dirtyRef 域，守卫是 guardBlockedRef 域），且只在 handleBack 消费、入口已清。✅
- **账号复位（:196-202 accountKey 守卫分支）不清 guardBlockedRef**：App 双挂载点 key={account}（:294/:340）账号切换即整体重建实例、ref 全复位 false；兜底分支触发时 guardBlockedRef 残留 true 也无影响（handleBack 入口清 + pendingUnsaved 只在返回时消费）。✅

### 1.3 Admin LogsTab isLoading 短路（OBSERVE-79-01 修复核证）

- 位置：Admin.tsx:922-944。分支次序 `logsQuery.isLoading ? 加载中 : logs && len>0 ? list : logsQuery.isError ? errorCard : 空态`。
- 首帧在途显"加载中"（不再闪一帧"暂无日志"）；数据到达显列表；isError 在空态之前短路（失败态不被空态吞并）；空态仅零数据渲染。与 Codes/Accounts 同构。✅
- 结论：OBSERVE-79-01 闭合。

### 1.4 契约 20 剥离（MINOR-79-03 修复核证）

- target-guard-check.ts:68 现为「// 已回显完成（echoed=true）稳态——courses 永驻非空 + 有选中，旧目标已合并」——"R63 " 前缀已剥离，why 语义保留。✅
- 全仓轮次标签族 + 行号族扫描：`R[0-9]+` / `第N轮` / `round` / `见N行` / `第N行` 在 web/src + web/scripts 双侧零命中（Math.round 与 rounded- 为代码语义非轮次标签，round 族正则命中后逐条人工排除）。✅
- 结论：MINOR-79-03 闭合。

---

## 二、M-1 延续管理（第十六轮）复核

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（逐字符比对，零回潮）：
  - 防抖回调 :712 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :512 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :609 + while :617 同参 `hasSelectedNow()`（:591-592 读 selectedRef 计算）
- **echoedRef 置位三路径 + 首帧不置位边界完整**：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，另含 :287-294 回显内 stale 兜底清理 + toast）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；防抖 effect 依赖 :773 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`——stateData/echoDone/hasPublishes 三路解锁闭合，F42-M1 自愈链零回潮。
- 第十六轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

---

## 三、六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true`/`echoed→false`/`courses 非空 && hasSelected→true`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :712 stateDataRef + effect 依赖 :773 | 判据只读 stateDataRef 非 echoedRef；/state 到达触发 effect 重跑自愈 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用（:42）；依赖含 selected 合并后重跑；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :712-756 五处 + flush :512-568 五处 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 `rev > 0 && !anyHas` 不合并、:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :747 / flush :559 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:294-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回归 |

**setSelected 调用点清点**（grep 实测七处，与 R79 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。

**防抖 effect 细节重走读**：resetRetry 放 effect 顶部（:681，新改动立即清退避 timer）；setTimeout 回调闭包读 selected 为 effect 创建时快照、publishesRef/stateDataRef 消费时刻最新；saveNow 四门卸载保护（:435/:446/:451/:463）延续；handleBack 三轮循环 :623-654 + pendingSaving :645 覆盖退避 timer 排队，21s 兜底绝不无限挂起——全部对照 R79 基线零回潮。

---

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-80-01 — guardBlockedRef 短路在拆分后语义冗余，且引入一条极端"该弹没弹"路径：同一次 handleBack 内先守卫命中、后真实保存失败时，终局 toast 被残留标记短路**

- 位置：Select.tsx:420-423（pendingUnsaved）与 :587（handleBack 入口清标记）、十处守卫置位。
- 触发场景推演：R79 拆分后守卫分支只置 guardBlockedRef、不置 dirtyRef——守卫拦截场景 dirtyRef 恒 false，pendingUnsaved 的 `dirtyRef || savingRef || timer` 本就 false，guardBlockedRef 短路在正常场景是冗余防御。但其"会话内任何一次命中即永久短路"语义（守卫通过轮不清标记）在交叉场景产生该弹没弹：handleBack 循环第 1 轮 flush 命中守卫（guardBlockedRef=true）→ 循环内 50ms 等帧（:635）期间用户又真实点选（rev 变化 → :637 continue 进入下一轮）→ 第 2 轮 flush 守卫通过（发布恢复）→ saveNow 网络失败（dirtyRef=true + scheduleRetry）→ pendingSaving 等待 21s 超时 → 循环结束 guardBlockedRef 仍 true → 终局短路不弹，尽管 dirtyRef 确证真实保存失败。
- 触发条件：守卫拦截 + 返回循环等待帧内用户继续点选 + 后续保存失败三条件同帧，概率极低；且 saveNow catch :452-456 已弹过"目标保存失败"红条，用户已收到失败反馈，仅终局总结提示缺失（不弹"改动未落库，返回后将以服务端保存的目标为准"）。
- 修复建议（候选，非必须）：终局判定前只查"最后状态"——把 pendingUnsaved 的 guardBlockedRef 短路改为 `dirtyRef.current || savingRef.current || retryState.current.timer !== null`（dirtyRef 拆分后已精确表达真实失败，短路可整体删除）；或在每轮 flush 入口守卫通过时同步清 guardBlockedRef（但会与守卫命中轮次互相干扰，语义更绕）。前者更简洁。
- 严重度论证：信息冗余缺失（失败红条已给过反馈），无数据风险；触发条件三重复合概率极低。维持 OBSERVE，不构成 MINOR。

**OBSERVE-79-01（延续，已闭合）**：LogsTab isLoading 短路已由 R79 修复落地（见 1.3），观察项转闭。

**OBSERVE-78-01（延续）**：handleBack 终局 toast 与退出前 flush 失败红条同 title 合并的文案突变窗口（超时路径与即时返回路径同源），去重已防轰炸，维持「续」。

**OBSERVE-77-01（延续）**：Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存——普通学生场景语义等价零影响。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 调用点清点（本轮 grep 实测七处与 R79 基线一致）——均维持「续」。

### 可疑待核

无新增。历轮"可疑-1"（终局 toast 与 onDone 卸载竞态）物理不可达论证在 R79 改判据后不受影响（末轮 flush 触发的 saveNow 置位 savingRef 会被 :645 pendingSaving 等待捕获收敛），结论维持。

---

## 五、已核无缺陷清单

- R79 修复 ①StatsTab data-first 次序（s 优先 / 错误卡仅 data 缺席渲染 / 与三 Tab 对称）；②guardBlockedRef 拆分完整性（十处守卫置脏全量清点 + dirtyRef 三处语义收敛 + 无未归类置位）；③LogsTab isLoading 短路分支顺序；④契约 20 R63 标签剥离——四项全部核证成立。
- 终局 toast 关键推演五条（守卫拦截不弹 / 真实失败弹 / 守卫后飞行成功不弹 / 守卫后恢复成功返回不弹 / 守卫后恢复失败返回弹）全部走读闭环，仅交叉极端场景留 OBSERVE-80-01。
- M-1 延续管理（第十六轮）：shouldDeferSave 三消费点（防抖/flush/handleBack 判定+while）全传 echoedRef.current 第三参、echoedRef 置位三路径 + 首帧不置位边界完整、target-guard 断言 18/18 全绿。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + 消费时刻双闸）：逐条判据与注释对应，stateData/echoDone/hasPublishes 三路解锁闭合，setSelected 七调用点无第三来源，零回潮。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：行号引用族 + 轮次前缀标签族全仓零残留；XSS 危险模式（dangerouslySetInnerHTML/innerHTML/eval(）零残留。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、撞名学生管理态（isCurrentAdminSession 六判定）、aria 无障碍、btn_type 三向、max_count=0 四处同源、倒计时打底——复跑全量通过，零差异化、零回潮。
- localStorage 读写全 try/catch 降级；视觉护栏 audit.mjs 全绿（画布双宽度/玻璃工具类/实心黑洞清零）。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 全部通过（视觉护栏） |
| 全仓残留扫描（行号族 / 轮次标签族 / innerHTML / dangerouslySetInnerHTML / eval(） | ✅ 零命中 |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第十六轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01/77-02**：同 key 不同 URL 缓存身份 / 窗口关闭无目标管理入口——维持「续」。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=795bb4f），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round80-frontend-findings.md）。
- 走读推断与实测冲突处理：OBSERVE-80-01 的触发路径经闭环走读确认（守卫命中 break 条件 :634 `!dirtyRef && !savingRef` 下守卫轮直接 break，需 rev 变化才进入下一轮——先按"继续循环"推演、后按"守卫命中即 break"修正，实测代码为准）。guardBlockedRef 短路冗余性经 grep 全量清点（dirtyRef 三处置位均为真实失败语义）后确认。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（guardBlockedRef 短路冗余 + 极端该弹没弹路径）+ 延续观察；连续第二十六轮无严重级发现。
