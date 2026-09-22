# Round 84 前端只读审查报告

基线：commit 42508fe（R83 双 findings + 收尾总结，R84 起始 HEAD）。本轮为 **R84 前端只读审查 + M-1 延续管理（第二十轮）**，核心为 M-1 第二十轮三消费点核证、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归 + setSelected 调用点清点 + 三组断言复跑、新视角扫查（Dashboard 目标卡片 window_closed 后分组/排序/空态、App onUnauthorized 管理代理态判据、Login 激活码模态框票据生命周期、Admin 五 Tab 轮询降频交互）、R80-R83 修复持续复核 + OBSERVE-83-01 留档延续、契约 20 全仓扫描。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 三组守护脚本 + audit.mjs。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑），`git status --short --branch` 为 `## master` 洁净，`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（激活失败后票据空请求文案突变）+ 1 条观察管理项（OBSERVE-77-01 前提复核：Dashboard/Select 的 /electives URL 现完全一致）+ 延续观察。M-1 延续管理第二十轮闭合：shouldDeferSave 三消费点（防抖 :697 / flush :507 / handleBack 判定 :595 + while :603）全传 echoedRef.current 第三参、置位三路径（:200/:238/:295）+ 首帧不置位边界（:232/:245）完整、target-guard 18/18 实测全绿。六防保存链零回归，setSelected 七调用点（:158/:198/:250/:288/:320/:351/:361）无第三来源，dirtyRef 三处置位语义收敛（:453/:561/:740 置 true、:461 清零）。新视角四项扫查全部收敛：Dashboard window_closed 后目标卡片分组（契约 3 关闭≠时间消失）排序/空态全链路正确；App onUnauthorized 管理代理态判据（lostAccount 反查 + :217 防御性 return）逐场景成立；Login 激活码票据生命周期与后端 handleActivate 单次消费语义完全对齐（唯一新 OBSERVE 见发现清单）；Admin 五 Tab 轮询降频交互（Codes/Stats/Logs 5s、Accounts 10s、Config 无轮询 + TabsContent unmount 停询）无缺陷。R80-R83 修复持续复核：guardBlockedRef 全仓零残留（含 backend）、注释口径三处统一（:412-415/:501-502/:507）、Admin StatsTab data-first（:757-792）、LogsTab isLoading 短路（:922-944）全部无回潮；OBSERVE-83-01 留档延续（Select.tsx:245 `pubs.length===0` return 分支仍在）。契约 20 全仓扫描：轮次前缀标签族/行号族/XSS 危险模式/web/src 与 web/scripts 零命中、localStorage 六处读写全 try/catch 降级、视觉护栏 audit.mjs 全绿。构建验证全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 全项通过）。连续第三十轮无严重级发现。**

---

## 一、M-1 延续管理（第二十轮）

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（grep 实测四调用点，零回潮）：
  - 防抖回调 :697 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :507 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :595 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
  - handleBack while :603 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`（与 :595 同参同判据）
- **echoedRef 置位三路径 + 首帧不置位边界完整**（grep 全量读写点清点，与 R83 基线一致）：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，另含 :287-294 回显内 stale 兜底清理 + toast）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 的立足点仍在，见延续清单）
  - 读点仅四处：:227（回显 effect 首行短路）、:317（独立清理 effect 守卫）、三消费点（:507/:595/:603/:697）——:227 与 :317 为「已回显完成跳过合并/清理」守卫，:507/:595/:603/:697 为第三参传递，全量清点无第五处
- targetGuard.ts 纯函数实现三参三分支（undefined→true / echoed→false / courses 非空 && hasSelected→true）与注释逐条对应；target-guard-check.ts 含 echoed 第三参 2 条断言（稳态编辑不闷死 / 首帧未到+已回显仍推迟）实测全绿。
- 第二十轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 三消费点传第三参 | 纯数据判据三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :697 stateDataRef + 防抖 effect 依赖 :753（含 stateData/echoDone/hasPublishes） | 判据只读 stateDataRef 非 echoedRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅作稳态放行、绝不驱动守卫判据 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；独立 effect 依赖 [publishes, selected, echoedRef, toast]，selected 加入依赖覆盖「清理先于回显合并」时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :697-743 + flush :507-562 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 `rev > 0 && !anyHas` 不合并、:277 `!hasTouched && !merged` 保持现状不返新引用、:249 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :730 / flush :551 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回归（:195-202 声明于 echoedRef/rev/setRev/setEchoDone 之后，TDZ 不触发） |

**setSelected 调用点清点**（grep 实测七处，与 R83 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。

**dirtyRef 置位语义清点**（grep 实测四处）：:453（saveNow catch 真实失败置 true）/ :461（finally 飞行中标记补发清 false）/ :561（flush 飞行中标记补发置 true）/ :740（防抖飞行中标记补发置 true）——置 true 仅三处且全部为真实失败/飞行中补发语义，守卫十分支纯 return 零置位；pendingUnsaved 三信号（dirtyRef/savingRef/timer）与 R81-R83 基线一致。

## 三、新视角扫查（换方向，四项全收敛）

1. **Dashboard 目标卡片分组/排序/空态在 window_closed 后**（Dashboard.tsx:219-263 / :535-545）：
   - 分组键三级兜底：`c.begin_date.slice(0,10)` → `pubById.get(publish_id)?.begin_date` → `"未知"`。window_closed 后 /electives 空 publishes（pubById 空映射），分组全走 CourseStatus 自带元数据——契约 3「关闭≠时间消失」承诺落位，目标卡片仍按日期/发布正确分组，零依赖 /electives 映射。
   - 排序：日期按 `|parseDateKey - todayMs|` 升序（距今天最近在前，未来/已过统一成立），"未知"恒排最后；组内 publish_id 升序、items 按 priority 升序。比较器对 `a==="未知" && b==="未知"` 返回 1（非自反），但 byDate 仅可能有一个"未知"键（所有无日期课程合并同键），多"未知"组不可能并存，无实际影响。
   - 空态：dateGroups.length===0（确证无目标）→ 「当前未添加任何预选课程」+ 前往挑选按钮；window_closed 后已保存目标 courses 非空（/state.courses 为持久化目标状态，不随窗口关闭清空）→ 正常显示终态卡片，空态只在真无目标时出现。折叠种子 `expandedDates===null` 只种一次，用户全折叠不被重拉。**全链路无缺陷**。
2. **App onUnauthorized 管理代理态判据**（App.tsx:187-235）：
   - 归属判定：`lostRaw = detail.session || detail.account || current`，先按账号名直查再按令牌反查，查无跳过（:193-198）。
   - 管理代理态（targetAccount 渲染 Select）请求全用管理员 Bearer + ?account=穿透，401 时 lostRaw 恒为管理员令牌 → lostAccount=adminName → :204-206 清 targetAccount + :210-215 退管理态并清 adminToken 标记 → 回登录页，绝不卡死代理页。?account= 学生名只作展示线索（client.ts:8 注释）。
   - :217 `isCurrentAdminSession(...) && inAdmin && lostAccount !== adminName` 时 return 不剔除：防的是 detail.session 缺失（无 session 的公开接口 /login 与 /activate 不会返回 401）且 detail.account 为学生名的极端组合——此刻学生账号剔除被推迟、下次请求再广播，管理员会话绝不因学生名被误杀。防御性正确。
   - 多窗口隔离：UNAUTHORIZED_EVENT 是 window 级事件，不同标签页不共享；管理页内无学生令牌请求在飞，场景不交叉。**逐场景推演无缺陷**。
3. **Login 激活码模态框票据生命周期**（Login.tsx:44-101 vs backend/api/handler.go:150-215）：
   - 1001 分支：`setPendingTicket((e.data?.ticket as string) || "")`，与后端 `writeJSON(w,1001,{ticket,account})` 契约对齐；激活请求回传 ticket 与 ActivateRequest 字段逐一匹配。
   - 票据单次消费语义闭环：激活失败（激活码错误）后端 ConsumeTicket 已销毁票据 → 前端 catch 非票据文案分支同步清空 pendingTicket（:95-96），下次登录再遇 1001 由服务端下发新票覆盖，UI 永不显示已消费票据；「激活票据无效或已过期」分支清票 + 明确引导「取消后重新登录即可进入（本账号已开通）」（:87-90）——与后端"票据单次防重放"刻意决策对齐。
   - 取消/Esc 分支（:234-237/:302-306）统一清 pendingAccount+pendingTicket+activateError，与激活中禁用（activating）防误关。**主链路闭环无缺陷；唯一边角见发现清单 OBSERVE-84-01**。
4. **Admin 五 Tab 轮询降频交互**（Admin.tsx:316-320/717-721/811-815/906-910）：
   - 轮询配置：Codes 5s / Stats 5s / Logs 5s / Accounts 10s / Config 无轮询（加载一次+保存后 refetch+代际回填）。TabsContent 非激活时 unmount → useQuery 卸载停询，切回 Tab 重新挂载时 react-query 缓存 data 仍在（isLoading 只在无 data 时 true）→ 切回即时显示缓存无 loading 闪现、后台 refetch 增量刷新，无重复请求风暴。
   - StatsTab window_closed 三态（:732-735，契约 23 补发字段 undefined 走"待命中"不假报关闭）+ data-first 次序（:757-792）延续；CodesTab removing 用 ReadonlySet 按码独立跟踪、ConfigTab「加载完成前禁用保存」（:520-524/:704）双闸、删除账号幂等守卫（:252）——全部无回归。
   - 观察：四 Tab 固定轮询未按 window_closed 降频（管理接口成本低、stats 三态需 5s 刷新及时翻转），设计合理，无缺陷。

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-84-01 — 激活失败（票据已销毁）后激活按钮仍可用，再点发 ticket="" 请求、错误文案突变**

- 位置：Login.tsx:64-101（activate 幂等守卫 :68、失败清票 :95-96、按钮 disabled={activating} :283/:307）。
- 触发场景推演：未激活账号登录 → 1001 弹窗带票据 T → 用户输错激活码 → 后端 ConsumeTicket 销毁 T + 返回「激活码无效、已用尽或该账号已激活」→ 前端 catch 非票据文案分支置 `activateError="激活失败，请检查激活码是否正确"` + `setPendingTicket("")` → activating 复位 false、按钮重新可用 → 用户不取消、直接再点「激活并登录」→ 请求体 `ticket: ""` → 后端 handler.go:191 `req.Ticket == ""` → 返回「账号、激活码与激活票据不能为空」→ 前端错误文案从「激活码错误」突变为「票据不能为空」，用户困惑（"我明明输了激活码"）。重试路径本应走「取消→重新登录→拿新票」，但仅票据过期分支给了引导文案，激活码错误分支未提示。
- 修复建议（低优先级候选）：激活失败（非票据过期）分支的 activateError 文案附引导「激活码错误或票据已消费，请取消后重新登录再试」；或在票据已清空（pendingTicket===""）且弹窗仍开时禁用激活按钮。均不影响安全（后端已拒空票请求）。
- 严重度论证：纯 UX 文案困惑，无数据/安全/功能影响（空票请求被后端 :191 正确拒绝、绝不误扣激活码），低于升级阈值，维持 OBSERVE。

**OBSERVE-77-01（前提复核，建议归档）**：历轮延续的「Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存」观察——本轮实测 Dashboard.tsx:173 与 Select.tsx:57 的 /electives URL **完全一致**（均为 `/electives?account=` + encodeURIComponent(account)，queryKey 亦同 `["electives", account, sessionToken]`），「不同 URL」前提已不成立（可能于历轮被统一），跨路由首帧命中即为同一 URL 的正确缓存、普通学生场景本就零影响。建议降级归档，留待主控裁决。

**OBSERVE-83-01（延续）**：Select.tsx:245 `if (pubs.length === 0) return` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前 tabs.length===0 无编辑入口、rev 恒 0、行为无害），未来开放「窗口关闭后管理目标」入口需一并处理守卫解锁。维持「续」。

**OBSERVE-78-01（延续）**：handleBack 终局 toast 与退出前 flush 失败红条同 title 合并的文案突变窗口，去重已防轰炸。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界，与 OBSERVE-83-01 关联）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 调用点清点（本轮 grep 实测七处与基线一致）——均维持「续」。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：末轮 flush 触发 saveNow 置位 savingRef 同步（首个 await 前）、pendingSaving 必捕获、api 20s abort 保证失败落地先于 21s 兜底，结论维持。

## 五、已核无缺陷清单

- M-1 延续管理（第二十轮）：shouldDeferSave 三消费点（防抖 :697 / flush :507 / handleBack 判定 :595 + while :603）全传 echoedRef.current 第三参、echoedRef 置位三路径 + 首帧不置位边界完整、读写点全量清点零回潮、target-guard 断言 18/18 实测全绿。
- 六防保存链（F43/F42/F40/F39-M1/F36/F48-M1 + 消费时刻双闸）：逐条判据与注释对应，stateData/echoDone/hasPublishes 三路解锁闭合，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（真实失败/飞行中补发语义），零回潮。
- 新视角四项：Dashboard window_closed 后分组三级兜底/排序/空态正确；App onUnauthorized 管理代理态逐场景成立；Login 激活码票据生命周期主链路闭环（唯一边角见 OBSERVE-84-01）；Admin 五 Tab 轮询交互无缺陷。
- R80-R83 修复持续复核：guardBlockedRef 全仓零残留（grep 全量含 backend）、注释口径三处统一（:412-415 / :501-502 / :507）、Admin StatsTab data-first 次序（:757-792，`s ? rows : isError ? errorCard : 加载中`）、LogsTab isLoading 短路（:922-944，`isLoading → logs&&len>0 → isError → 空态`）——无回潮。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：轮次前缀标签族（`\bR\d{2}\b|第\s*\d+\s*轮|round\s*\d+|（第\s*\d+次?`）web/src 与 web/scripts 零命中；行号引用族（`\b\d+行|见第\d+|行号|:\d{3,4}`）零命中；XSS 危险模式（dangerouslySetInnerHTML/innerHTML=/eval(/document.write/new Function）零命中；localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级；视觉护栏 audit.mjs 全绿（画布双宽度/玻璃工具类/实心黑洞清零）。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、撞名学生管理态（isCurrentAdminSession 六断言）、aria 无障碍、btn_type 三向、max_count=0 四处同源、倒计时兜底（F39-N1 begin_times 打底）、契约 23 window_closed 三态——复跑全量通过，零差异化、零回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 全部通过（视觉护栏，A/B/C 三组全项） |
| 全仓残留扫描（guardBlockedRef / 轮次标签族 / 行号族 / XSS 危险模式 / TODO-stub） | ✅ 零命中 |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01**：前提复核——Dashboard/Select 的 /electives URL 现完全一致，「不同 URL」前提不成立，建议归档（留待主控裁决）。
- **OBSERVE-77-02 / OBSERVE-83-01**：窗口关闭无目标管理入口 / courses 非空+publishes 空+echoedRef 未置位稳态组合——维持「续」，二者关联，未来开放「关闭后管理目标」入口需一并处理守卫解锁。
- **OBSERVE-84-01**：激活失败后票据空请求文案突变，低优先级候选（见发现清单）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，本轮延续论证，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=42508fe，master），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round84-frontend-findings.md）。
- 走读推断与实测冲突处理：OBSERVE-77-01 前提复核先按历轮延续描述（"不同 URL"）推演、后 grep 实测两处 URL 逐字符比对确认一致——以实测为准，观察前提不成立建议归档。OBSERVE-84-01 触发路径经「后端 handler.go:191 空票校验 → 前端 catch 非票据文案分支」闭环走读确认，后端行为已用代码证据钉死（ConsumeTicket 单次销毁 + 空票拒绝）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（激活失败票据空请求）+ 1 条观察管理项（OBSERVE-77-01 前提复核）+ 延续观察；连续第三十轮无严重级发现。
