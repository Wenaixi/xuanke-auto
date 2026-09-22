# Round 91 前端只读审查报告

基线：commit 7173970（R90 双 findings + 收尾总结，HEAD，进度 91/256）。本轮为 R91 前端只读审查 + M-1 延续管理（第二十七轮）。核心为 M-1 第二十七轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五处）、target-guard 18/18 断言复跑；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；OBSERVE-90-01（Dashboard 日志区三态）/ OBSERVE-88-01（ErrorBoundary）/ OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（open_time 字符串路径与 begin_times 时间戳路径的同点收敛复核 / 列表 key 稳定性 / 事件监听与清理 / 深链接与初始状态 / 表单可访问性）；契约 20 全仓扫描；三组断言 + audit.mjs + tsc + build 复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / windowClosedLocked / StateForAccount / openTimeForLocked / handler.go:901 序列化）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git log/status、`npx tsc -b --pretty false`、`npm run build`、三守护脚本 + audit.mjs 只读复跑、node 纯只读实测），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round91-frontend-findings.md（不存在，本轮新建）。结束态 `git status --short --branch` = `## master` + `?? archive/review-rounds/round91-backend-findings.md`（r91 后端代理报告文件，历轮同款并行产出物）——除两个报告文件外零改动，build 产物 backend/web/dist/ 已被 git 忽略（实测 `git check-ignore` 确认）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新真实缺陷 + 延续观察管理。** M-1 延续管理第二十七轮闭合：shouldDeferSave 三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符复核通过、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）完整、读写点全量清点（置位 3 + 读点 6）无第七处、target-guard 18/18 实测全绿（grep -c "✓" = 18，含 echoed 第三参 2 条）。六防保存链逐条重读零回潮（shouldDeferSave 三参三分支 / F42-M1 纯数据判据 / F40-M1 cleanStaleSelected / F39-M1 双闸消费时刻 / F36 回显真合并 / F48-M1 清空语义）。新视角五组扫查全部收敛：① open_time 字符串路径（后端 handler.go:901 `Format("2006-01-02 15:04:05")` 空格分隔）经 V8 实测按本地时区解析与 begin_times 时间戳路径同点收敛（1789261200000，R89 结论本轮复证），且 open_time 序列化点与调度器识别槽（time.UnixMilli 本地时区）逐级对齐无分叉；② 列表 key 稳定性（Select 课程 key=c.id / Tabs key=publish_id / Dashboard 日期 key 与 pub key / Admin codes/accounts/logs key 全部唯一稳定）；③ 事件监听与清理（App 滚动/resize rAF + 全量移除 + cancelAnimationFrame、UNAUTHORIZED_EVENT 全量移除、Toast 挂载卸载、unmountedRef 清理）；④ 深链接与初始状态（无账号进页、空 sessions、管理令牌持久化降级、StrictMode 双挂载）；⑤ 表单可访问性（aria 与 focus-visible 横向一致性，聚焦陷阱三弹窗裸 div 历史评估延续）。契约 20 扫描（轮次标签族 / 行号引用族 / XSS / localStorage try/catch / SPA 无导航能力）全仓零命中。连续第三十七轮无严重级发现。

---

## 一、M-1 延续管理（第二十七轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处调用 + Read 逐字符核对，与 R90 记录一致）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后，TDZ 不触发；F36-01 兜底守卫）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 全量 6 处）：:229（回显 effect 首行短路）、:319（独立清理 effect 守卫）、四消费点（:509/:597/:605/:699）——无第七处。本轮另核 :200（写）/ :240/:241 / :297/:298（写）——写点与读点互斥无漏网。
- **target-guard 断言 18/18** 实测全绿（grep -c "✓" = 18，含 echoed 第三参 2 条 :70/:76）。
- 第二十七轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | :26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:158（声明）/ :198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——七处，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发），全部为真实失败/飞行中补发语义；守卫十分支（:510/:516/:554/:559/:700/:710/:733/:738）纯 return 零置位。零回潮。
- **unmountedRef 全读写点**（grep 实测）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。

## 三、新视角扫查（本轮五组）

1. **时间格式化与倒计时的一致性（open_time 字符串路径 × begin_times 时间戳路径同点收敛）**：
   - 后端序列化点：handler.go:901 `open.Format("2006-01-02 15:04:05")`（空格分隔，本地时区）→ 前端 `new Date("2026-09-13 09:00:00")` 按本地时区解析（V8 实测 = 1789261200000，与 CLAUDE.md 记录平台开窗时间戳完全一致，isNaN=false）——与 begin_times 路径 `new Date(begin_times[0]).toISOString()`（UTC 串）→ `useTickingCountdown` 内 `new Date(target).getTime()` 同点收敛于同一绝对时刻（毫秒经 UTC ISO 串往返无损，R89 结论本轮复证）。
   - 识别槽链路逐级对齐：调度器 probe 入账 `time.UnixMilli(data.BeginTimes[0])`（:861/:1114 日志格式同为本地时区）→ `openTimeForLocked` 返回 time.Time（含秒精度）→ handler.go:901 `Format` 本地时区序列化。时间戳→time.Time→本地时区字符串→前端本地时区回解析，全链路同时区零分叉。
   - 倒计时消费点复核：Select :767-772 与 Dashboard :191-196 同源兜底（openTimeStr ?? begin_times[0] 格式化），useTickingCountdown 目标变化即校正 now（:19-21）且 diff<=0 全 00（:24-26）；Dashboard :206-212 primaryMs/extrasMs 同源、`filter(t !== primaryMs)` 去重主时间。24 小时制全站 `toLocaleString("zh-CN", { hour12: false })` 锁死。Dashboard parseDateKey/localTodayMs 本地零点无 UTC 偏移（R85 已核，本轮复证）。
   - **结论**：时间一致性零缺陷。
2. **列表渲染的 key 稳定性与 DOM 复用**：Select 课程卡片 key=c.id（:1020，课程 id 平台唯一）/ TabsTrigger+TabsContent key=publish_id（:936/:984）/ Dashboard 日期组 key=g.key、发布 key=pub.publish_id、课程 key=c.class_id（:551/:580/:595）/ Admin codes key=c.code、accounts key=a.account、logs key=l.id（:461/:848/:932）——全部唯一且随数据变化即重组（无 key 复用错配导致的状态污染风险）；react-query 数据不变引用不变（useMemo 依赖稳定，R89 已核）。**结论**：零缺陷。
3. **事件监听与清理**：App 滚动/resize rAF 全量 removeEventListener + cancelAnimationFrame（:271-276）；UNAUTHORIZED_EVENT 挂载/卸载对称（:232-233）；Toast Provider 无全局监听；Admin copyTimer 卸载清理（:114-119）；Select unmountedRef 挂载复位 + cleanup 置 true + retryState timer 清理（:399-408）。无泄漏路径。**结论**：零缺陷。
4. **深链接与初始状态**：无账号进页（sessions 空）→ Login 渲染（App :346-348）；空 sessions 时 account-reselect effect 回登录页（:136-142）；管理令牌持久化读 localStorage 全 try/catch 降级内存态（App :37-68 六读写全 try/catch，刷新后管理页凭「会话 token === 标记 token」恢复、撞名学生永不误进管理页）；StrictMode 双挂载由 unmountedRef 挂载复位吸收（Select :399-401 注释 + 实现）。**结论**：零缺陷。
5. **表单可访问性**：Input/Button/Tabs/Progress 全带 focus-visible 或 Radix 原生 focus 环（Input :14 `focus:ring-1`、Tabs :29/:44 focus-visible、Button 无自定义 focus 但继承原生 button focus + Radix Slot）；aria：Progress role=progressbar + aria-valuenow（Progress.tsx:24-27）、Admin 开关 role=switch + aria-checked（:578-581）、CollapseSection button aria-expanded/aria-controls + useId（Dashboard :64-71）、退选/删除/激活三弹窗 role=dialog + aria-modal + aria-labelledby + Esc 关闭 + 取消 autoFocus；密码可见性切换 aria-label + aria-pressed（Login :171-172）。聚焦陷阱三裸 div 弹窗历史评估延续（O-3 族 → F6-02 Radix Dialog 迁移单一出口，历轮未落地维持观察）。**结论**：横向一致，零缺陷。

## 四、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增，延续项管理）

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:697-718）缺「加载中」与「拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined，均显示 "NO RECENT LOGS"。本轮重读位置无变化（:696-719 两态结构原样）、零功能危害维持留档。低优先级候选（一行条件对齐其余列表三态）评估：本轮无触发证据（/logs 每 3s 轮询、失败已 30s 降频，纯展示误导），维持「续」不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——全仓零命中复证（componentDidCatch/getDerivedStateFromError 零命中，main.tsx:6-10 裸 createRoot）；90 轮零渲染期异常实证 + 正常路径防御充分（全站列表 `?? []` 兜底 + isError 分支），本轮无新依据提级。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演无新触发面（末次点选与平台清空同落窗口属子秒级物理窄窗，守卫拦下后 hasPublishes 恢复驱动重试）。维持「续」。

**OBSERVE-84-01（延续）**：激活失败票据空请求文案突变——Login.tsx:87-97 两种失败分支均已清 pendingTicket，但清票据后用户再点「激活并登录」会用空 ticket 再打 /activate（后端拒"激活票据无效"）。F43-N2 已处理误导主链路，残余仅主动重复点击时第二次空请求文案；低优先级 UX 候选。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前行为无害：pubs 空时窗口未开/已关、用户无目标改动则无保存链路，非空改动走守卫置脏 + hasPublishes 恢复驱动）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口（与 83-01 关联）。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，全部指向 F6-02 Radix Dialog 迁移单一出口。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 五、已核无缺陷清单

- M-1 延续管理（第二十七轮）：三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参、置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）无第七处、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:455/:563/:742）零回潮，unmountedRef 八读写点全与「卸载后不 fire/不 toast」契约对齐。
- 新视角：时间链路（open_time 空格字符串 V8 本地时区解析 = 1789261200000 与 begin_times 时间戳同点收敛、识别槽 time.UnixMilli → Format 本地时区序列化 → 前端本地回解析全链路零分叉、24 小时制锁死、本地零点分组）；列表 key 稳定性（c.id/publish_id/g.key/class_id/code/account/l.id 全唯一）；事件监听清理（rAF 滚动/load/resize/UNAUTHORIZED_EVENT/copyTimer/unmountedRef 全量对称清理）；深链接与初始状态（空 sessions 回登录、管理令牌持久化 try/catch 降级、StrictMode 双挂载吸收）；表单可访问性（Progress/Switch/CollapseSection/三弹窗 aria 完整、focus-visible 横向一致）。
- 后端交叉契约复核：handleSetTargets（handler.go:443-495）凭据表校验 → Store.SetTargetsForAccount → Sched.SetTargetsForAccount 顺序、条数/class_id/publish_id/priority 四校验、空 targets 合法清空；windowClosedLocked 三判据单源（:922-940：主判据 / 时钟失败≥3+已过开窗点 / 幽灵窗口 EmptyProbeRuns≥3+已过开窗点，open 单快照复用）；StateForAccount（:703-733）WindowClosed 实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤；openTimeForLocked（:431-441）识别槽优先级 acct→"*"→遗留字段；handler.go:901 序列化 `Format("2006-01-02 15:04:05")` 与前端解析零分叉。
- 契约 20：轮次前缀标签族 / 行号引用族 / XSS 危险模式（innerHTML/dangerouslySetInnerHTML/document.write/eval/New Function 零命中）/ localStorage try/catch（六处读写全 try/catch）/ SPA 无导航能力（window.open/location.*/history.* 零命中、纯组件内存 state 拼 API path）全零命中。
- TDD 三组断言：target-guard 18/18、admin-auth 6/6、unauthorized 5/5 全绿；audit.mjs 77 项全绿。
- 手动报名/退选 Set 在飞幂等 + 双 invalidate、登录/激活链幂等 + 票据贯通 + 401 三形态单广播、btn_type 三向、max_count=0 四处同源、open_time 空格串 V8 解析（R89 实测，本轮复证）——复跑无回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0（TSC_EXIT=0，实测） |
| `cd web && npm run build` | ✅ EXIT 0（1948 modules transformed，产物 419.87 kB JS / 41.11 kB CSS 落 backend/web/dist，git check-ignore 确认 dist 被忽略） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（grep -c "✓" = 18） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（grep -c "✓" = 6） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（grep -c "✓" = 5） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（grep -c "✓" = 77，✗ = 0） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master` + `?? archive/review-rounds/round91-backend-findings.md`（r91 后端并行产出，历轮同款）；除报告文件外零改动 |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十七轮闭合，见上。下轮继续常规核对。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——走读推断，展示误导，不动作。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口。
- **OBSERVE-75-03**（延续）：Admin uses 无前端上限（Infinity→null→后端拒绝路径亦被后端值域校验兜底）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本 + audit.mjs + node 纯只读实测）；工作区 `git status` 除两个报告文件外零改动（HEAD=7173970，master），未修改任何仓库代码文件。
- 走读推断与实测区分：OBSERVE-90-01（Dashboard 日志区三态）、handleBack 等待无反馈、O-3 聚焦陷阱为走读推断；M-1 逐字符、三组断言计数（18/18、6/6、5/5、77）、tsc/build 退出码、契约 20 扫描、open_time 空格字符串 V8 解析（1789261200000）、git check-ignore、后端契约 grep 定位均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；零新 OBSERVE；延续观察管理；连续第三十七轮无严重级发现。
