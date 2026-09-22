# Round 89 前端只读审查报告

基线：commit 8d1abe0（R88 双 findings + 收尾总结，HEAD，进度 89/256）。本轮为 R89 前端只读审查 + M-1 延续管理（第二十五轮），核心为 M-1 第二十五轮 shouldDeferSave 四消费点（防抖 :699 / flush :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符闭合、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归 + setSelected 七调用点/dirtyRef 置 true 三处清点、F88-01 Admin htmlFor 修正复核（git diff 范围 ca08c46 逐字符 + 全仓 htmlFor 配对扫描）、OBSERVE-88-01 延续评估（ErrorBoundary 缺失）、OBSERVE-85-02/84-01/83-01/77-02/76-01/76-02/76-03 延续管理、新视角扫查（react-query 缓存清理时机 / 表单输入校验边界 / Admin 五 Tab 状态同步 / 时间显示一致性 / 请求取消覆盖——五组全部收敛）、契约 20 全仓扫描、三组断言 + 构建复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleAdminStats / windowClosedLocked / StateForAccount / handleSetTargets）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git show/log/status、`npx tsc -b --pretty false`、`npm run build`、三守护脚本 + audit.mjs 只读复跑），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round89-frontend-findings.md（不存在，本轮新建）。结束态 `git status --short --branch` = `## master` 洁净，`git diff HEAD --stat -- web/src web/scripts` = 空，build 产物 backend/web/dist/ 已被 web/.gitignore 忽略（`git check-ignore` 实证），零仓库改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新 OBSERVE + 延续观察管理。** M-1 延续管理第二十五轮闭合：shouldDeferSave 四消费点全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）、target-guard 18/18 实测全绿。F88-01 Admin htmlFor 复核通过：ca08c46 提交 6 对 label htmlFor + Input id 全配对、全唯一、与 Login 同款模式、git diff 零意外改动、工作区零 diff。新视角五组扫查全部收敛：react-query 缓存（gcTime 5 分钟自动回收非活跃查询、查询键含 account+sessionToken 天然隔离、invalidate 前缀覆盖无跨账号误伤）、表单输入校验（count/concurrency 完整 clamp 对 Infinity 安全、uses 无上限为 OBSERVE-75-03 延续）、Admin 五 Tab 状态同步（Radix Tabs 内容保持挂载、代际回填防陈旧值覆盖）、时间显示一致性（空格分隔时间字符串 V8 实测解析正确、begin_times 时间戳路径与字符串路径同点解析零分叉、本地零点分组无时区偏移）、请求取消（api 20s 超时 + 调用方 signal、卸载后副作用 unmountedRef 短路、手动报名在飞返回后 toast 落 App 层永挂载 Provider 属合理反馈非缺陷）。契约 20 扫描（轮次标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch）全仓零命中。连续第三十五轮无严重级发现。

---

## 一、M-1 延续管理（第二十五轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处调用，与 R88 记录完全一致，逐字符核对）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
  - targetGuard.ts:64-72 三参三分支（`stateData === undefined → true` / `echoed → false` / `(courses.length ?? 0) > 0 && hasSelected → true`）与注释逐条对应；`echoed` 第三参只在稳态放行（echoed=true 直接 false）、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界逐字符复核**：
  - courses 空分支 :240-241（置 true + setEchoDone(true)）
  - 合并完成分支 :297-298（置 true + setEchoDone(true)）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)，声明于 echoedRef/rev/setRev/setEchoDone 之后、TDZ 不触发）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return 绝不置位；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
  - 读点全量清点（grep 全量 6 处）：:229（回显 effect 首行短路）、:319（独立清理 effect 守卫）、四消费点（:509/:597/:605/:699）——无第七处；:307/:314 为注释引用非读取
- **target-guard 断言 18/18** 实测全绿（grep -c ✓ = 18，含 echoed 第三参 2 条）。
- 第二十五轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755 | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 等状态之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测七处，与 R88 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发），全部为真实失败/飞行中补发语义；守卫十分支（:510/:516/:554/:559/:700/:710/:733/:738）纯 return 零置位，终局 toast（:647 `revRef.current > 0 && pendingUnsaved()`）只对真实失败触发。零回潮。

## 三、F88-01 Admin htmlFor 修正复核

- **git 提交范围核验**：F88-01 为 ca08c46 提交的一部分（同一 commit 含 B88-01 后端修复 + Admin htmlFor 落地），`git show ca08c46 -- web/src/routes/Admin.tsx` 逐字符比对与当前工作区一致。
- **6 对 label htmlFor + Input id 配对逐字符核对**（grep -on 实测 + Read 复证）：
  - CodesTab：admin-code-count（label :374 / Input id :376）、admin-code-uses（label :386 / Input id :388）
  - ConfigTab：admin-config-base-url（label :605 / Input id :607）、admin-config-api-key（label :615 / Input id :619，密码框）、admin-config-model（label :628 / Input id :630）、admin-config-concurrency（label :679 / Input id :681）
  - **id 唯一性**：grep `id="admin-[^"]*"` 六处每 id 恰 1 次，无重复；
  - **配对完整性**：脚本遍历全仓 tsx 所有 `htmlFor="X"` 逐一验证存在 `id="X"`——零未配对（含 Login 的 login-account/login-password 与激活码 activation-code）。
  - **与 Login 同款模式**：Login.tsx:131/:150 `htmlFor="login-account"/"login-password"` 显式关联 + :257 `htmlFor="activation-code"`，Admin 六对沿用完全同构。
- **git diff 零意外改动**：ca08c46 的 Admin.tsx diff 仅 6 处 label 补 htmlFor + 6 处 Input 补 id，无任何其他改动；工作区 `git diff HEAD --stat -- web/src web/scripts` 为空。
- **F88-01 结算论**：OBSERVE-86-01 修复落地正确、零行为变更（纯 JSX 属性增量）、零意外改动。观察项可归档（本轮复核无回潮）。

## 四、OBSERVE-88-01 延续评估（ErrorBoundary 缺失）

- 位置复证：web/src/main.tsx:6-10 createRoot 直接渲染 App，无任何 ErrorBoundary；全仓 `ErrorBoundary|componentDidCatch|getDerivedStateFromError` 零命中（grep 退出码 1）。
- **本轮落地价值评估**：按「宁缺毋滥」+ ponytail YAGNI 原则评估——① 88 轮审查零渲染期异常实证（正常路径防御充分：全部列表 `?? []` 兜底、条件渲染、isError 分支覆盖）；② 真实白屏候选仅剩「后端契约破坏性变更」理论路径（/electives 响应结构变、字段删除 → 某 `.map/.filter/.toLowerCase` 在 undefined 上抛错）；③ 引入 ErrorBoundary 是纯预防性复杂度，~20 行 class 组件 + 全屏重载提示，正常路径零执行、测试价值趋零。**结论：维持 OBSERVE 延续，不建议本轮落地**——除非后端未来做破坏性契约变更，落地成本不高于 20 行、届时随手可加。若主控坚持低风险防御闭环，落地也零风险（建议包在 QueryClientProvider 外层或 StrictMode 内，fallback 全屏「页面渲染异常，请刷新」+ 重载按钮）。

## 五、新视角扫查（换方向，五组逐项）

1. **react-query 缓存清理时机（账号删除/切换后的残留缓存、invalidate 覆盖范围与内存泄漏）**：
   - 查询键维度：全站 12 个 useQuery 键全部含 account+sessionToken（Admin 五键含双维度、Dashboard/Select 三键含双维度、Dashboard /logs 含 sessionToken）——跨账号天然隔离，管理员代理切换 targetAccount 不串数据。
   - 账号删除（onDeleted / onUnauthorized）后无显式 removeQueries：残留缓存条目无 observer 后由 react-query 默认 gcTime（5 分钟）自动回收，非活跃查询不进内存泄漏路径；同名重建登录用新 token → 新键，旧条目独立 gc。**零泄漏**（走读推断，react-query v5 默认 gcTime 语义）。
   - invalidate 覆盖范围：select/exit 后 `invalidateQueries({queryKey:["electives"]})`/`["state"]` 前缀命中含 account 的键（同键即共享缓存，Dashboard 与 Select 同键互享缓存数据，切页零重复请求——契约注释 :166-168 已明）；前缀不含账号维度但键内含 account，实际只命中当前账号。**无跨账号误伤**。
   - 结论：缓存清理机制完备，零缺陷。
2. **表单输入校验的边界（number input 的 min/max 兜底、空值/NaN 处理、粘贴非法值）**：
   - 清点四处 number Input：CodesTab count（:377-381 `Math.max(1, Math.min(100, Number(...) || 1))`）、uses（:387-392 `Math.max(1, Number(...) || 1)`）、ConfigTab concurrency（:680-686 `Math.max(1, Math.min(16, ...) || 1)`）。
   - 边界实测：`Number("")`→0→`0||1`→1；`Number("abc")`→NaN→`NaN||1`→1；`Number("1e309")`→Infinity→`Math.min(100, Infinity)`→100 / `Math.min(16, Infinity)`→16——count 与 concurrency 的 clamp 对 Infinity 完整收敛，粘贴 `1e309`/`999999999` 均被钳制；**唯一例外 uses 只有下限无上限**（Infinity → `Math.max(1, Infinity)` → Infinity → 受控 value=Infinity → 提交 JSON.stringify → null → 后端 int 字段 0 → 后端值域校验拒绝 → 失败 toast）——此为 OBSERVE-75-03 已记录族（:132「uses 未对称限流」），本轮复证无变化、无新缺陷。
   - 结论：表单校验边界完备（uses 例外为延续观察），零新发现。
3. **组件间状态同步（Admin 五 Tab 间的共享状态、config 保存后各 Tab 回显一致性）**：
   - Radix Tabs Content 首次激活后保持挂载（非 forceMount 默认）→ 五 Tab 组件状态在切换间保留（Admin :52-55 注释已明）；CodesTab generated/removing、ConfigTab 表单值跨 Tab 切换不丢。
   - Config 保存 → refetch + configEpoch 代际自增 → effect 用后端「实际生效值」回填表单（:546-549 注释论证防陈旧值覆盖回滚）——Config 自身回显一致性闭环。
   - 跨 Tab 联动：config 的 activation_on/engine/concurrency 修改后 StatsTab 的对应行由 5s 轮询自然收敛（可接受延迟，无强制实时性契约）；删除账号 → AccountsTab invalidate 立即消失 + onDeleted 同步本地会话 → 账号迁移 effect 联动。**无状态不同步缺陷**。
4. **时间显示的一致性（时区处理、24 小时制、open_time 零值显示）**：
   - open_time 字符串格式实证：后端 handler.go:901 `open.Format("2006-01-02 15:04:05")`（空格分隔）→ 前端 `new Date("2026-09-13 09:00:00")`。**V8 实测**：空格分隔日期字符串按本地时区解析，`new Date("2026-09-13 09:00:00").getTime()` = 1789261200000（与 CLAUDE.md 记录的时间戳完全一致），`isNaN` = false——与 `"2026-09-13T09:00:00"` T 分隔同值。**前端零缺陷**（历史顾虑「空格分隔解析失败」在 V8/现代浏览器不成立）。
   - begin_times 路径：`new Date(begin_times[0]).toISOString()`（UTC 串）→ `new Date(target).getTime()` 回解析——毫秒时间戳经 UTC ISO 串往返无损，与 open_time 字符串路径同点收敛于同一绝对时刻。
   - 零值显示：open_time 空串（falsy）→ openTimeStr null → 兜底 begin_times[0] → 双缺才显「未识别到开放时间」（Select :839-843）/「正在同步...」（Dashboard :412-414），三态文案齐备。
   - 24 小时制：全站 `toLocaleString("zh-CN", { hour12: false })` 显式锁 24 小时制，无上午/下午歧义。Dashboard parseDateKey/localTodayMs 本地零点无 UTC 偏移（R85 已核）。
   - 结论：时间显示全链路一致，零缺陷。
5. **请求取消（AbortController / unmount 时的在飞请求是否泄漏、20s 超时的覆盖范围）**：
   - api() :55-57 兜底 20s AbortController + :51-52 `signal: rest.signal ?? ctrl.signal` 调用方信号优先；finally :107 clearTimeout。**超时覆盖全部 api 调用**（含 select/exit/targets PUT/login/activate/五管理接口）。
   - 卸载后在飞请求：Select/Dashboard/Admin 均无显式 AbortController（卸载不主动 abort）——fetch 继续跑完但结果由 react-query observer 丢弃（observer 卸载即忽略回调）或 unmountedRef 短路副作用（saveNow :431/:440/:445/:457、防抖 toast :718/:527）；**无泄漏**（无 unmounted setState——react-query 内部管理、手动操作 setActionLoading 在 React 18 无警告无内存泄漏）。
   - 手动报名/退选在飞返回：handleBack 只等保存链（dirty/saving/timer）不等 actionLoading——报名在飞时返回立即卸载，finally 的 toast 落 App 层 ToastProvider（永挂载）在 Dashboard 上弹出「报名成功/失败」，invalidate 无害、setActionLoading 丢弃。**语义评估**：手动操作是用户主动行为，其结果反馈跨越返回边界显示属合理（与 saveNow 的「卸载后不 toast」哲学区别：后台自动保存无操作者注意力，手动报名有明确操作者期待结果）——非缺陷，记录为已核无缺陷。
   - 结论：请求取消覆盖完备（超时兜底 + 调用方信号 + 副作用短路三件套），零缺陷。

## 六、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮 0 条新 + 延续项管理）

**OBSERVE-88-01（延续，本轮评估不建议落地）**：ErrorBoundary 缺失——复证位置无变化；落地价值评估见第四节，结论维持观察（正常路径防御充分、88 轮零渲染期异常实证、预防性复杂度不划算；后端契约破坏性变更时才值得落地，~20 行随时可加）。

**OBSERVE-86-01（归档）**：Admin htmlFor 已在 ca08c46 落地（6 对全配对），本轮复核通过，归档。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲，无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——低优先级 UX 候选，维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前行为无害）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口（与 83-01 关联）。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底，本轮复证 Infinity→null 路径亦由后端拒绝兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，全部指向 F6-02 Radix Dialog 迁移单一出口。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立。

## 七、已核无缺陷清单

- M-1 延续管理（第二十五轮）：shouldDeferSave 四消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）零回潮、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:455/:563/:742）零回潮。
- F88-01 Admin htmlFor（第三轮复核→归档）：ca08c46 提交 6 对 label+id 逐字符配对、id 全仓唯一、全仓 htmlFor 零未配对、与 Login 同款模式、git diff 零意外改动。
- 新视角五组：react-query 缓存（gcTime 5 分钟回收 + 键含账号隔离 + invalidate 前缀无跨账号误伤）；表单输入校验（count/concurrency Infinity clamp 安全、空值/NaN/粘贴非法值全收敛，uses 例外为 OBSERVE-75-03 延续）；Admin 五 Tab 状态同步（Radix 保持挂载 + 代际回填闭环 + 跨 Tab 轮询收敛）；时间显示（空格分隔字符串 V8 实测解析正确、begin_times 与 open_time 同点收敛、本地零点分组、24 小时制全站锁死）；请求取消（20s 超时全接口覆盖 + 调用方 signal 优先 + 卸载副作用 unmountedRef 短路 + 手动报名在飞返回的 toast 落永挂载 Provider 属合理反馈）。
- 后端交叉契约复核：handleAdminStats（handler.go:874-955）open_time 输出 `open.Format("2006-01-02 15:04:05")`（:901）与前端 `new Date()` 解析实测一致；windowClosedLocked / StateForAccount / handleSetTargets 与前轮记录一致，无契约漂移。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5 全部实测全绿；audit.mjs 77 项全部通过。
- 契约 20：轮次前缀标签族 / 行号引用族 / XSS 危险模式（dangerouslySetInnerHTML/innerHTML=/eval(/document.write/new Function）web/src 与 web/scripts 零命中；localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级；adminAuth.ts 纯函数零存储依赖；guardBlockedRef 全仓零残留（历轮 R80 移除后持续）。
- 手动报名/退选 Set 在飞幂等 + 双 invalidate、登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、btn_type 三向、max_count=0 四处同源、倒计时兜底——复跑全量通过，零差异化、零回潮。

## 八、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0（类型全通过） |
| `cd web && npm run build` | ✅ EXIT 0（1948 modules transformed，产物 419.87 kB JS / 41.11 kB CSS 落 backend/web/dist，dist 被 git 忽略） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（grep -c ✓ = 18） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（实测 6 条） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（实测 5 条） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过 |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/guardBlockedRef） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master` 洁净 |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（本轮零改动） |

## 九、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十五轮闭合，见上。
- **OBSERVE-88-01**（延续，本轮评估不建议落地）：ErrorBoundary 缺失——防御性改进候选，维持观察。
- **OBSERVE-86-01**（本轮归档）：Admin htmlFor 已落地复核通过。
- **OBSERVE-85-01**（归档维持）：F86-01 注释口径复核连续四轮无回潮。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**（focus trap/scroll lock/焦点恢复）：指向 F6-02 Radix Dialog 迁移单一出口。
- **OBSERVE-75-03**（延续，本轮复证）：Admin uses 无前端上限（Infinity→null→后端拒绝路径亦被后端值域校验兜底）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。

## 十、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=8d1abe0，master），未修改任何仓库文件（唯一写入为本报告文件 archive/review-rounds/round89-frontend-findings.md）。
- 走读推断与实测区分：react-query gcTime 回收语义 / 卸载后在飞请求行为 / ErrorBoundary 白屏推演为走读推断；M-1 逐字符、F88-01 逐字符配对、空格分隔时间字符串 V8 解析、三组断言计数、git 提交范围均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；零新 OBSERVE；1 条归档（OBSERVE-86-01 htmlFor 复核通过）；延续观察管理；连续第三十五轮无严重级发现。
