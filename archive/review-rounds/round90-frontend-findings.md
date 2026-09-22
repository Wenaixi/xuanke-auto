# Round 90 前端只读审查报告

基线：commit 87b1d53（R89 双 findings + 收尾总结，HEAD，进度 90/256）。本轮为 R90 前端只读审查 + M-1 延续管理（第二十六轮）。核心为 M-1 第二十六轮 shouldDeferSave 四消费点（防抖 :699 / flush :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符闭合、echoedRef 置位三路径 + 首帧不置位边界与读写点全量清点、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归、F88-01 Admin htmlFor 第三轮复核（本轮归档）、OBSERVE-88-01/85-02/84-01/83-01/77-02/76-01/76-02/76-03 延续管理、新视角扫查（登录/登出/401 会话状态一致性 / 倒计时与状态横幅五态 / 退选弹窗在飞取消 / Toast 高频合并与滚动 / 空态加载态错误态三态完整性）、契约 20 全仓扫描、三组断言 + 构建复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleAdminStats / handleSetTargets / StateForAccount 同源 / enrichTargetPubMetaLocked）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git show/log/status、`npx tsc -b --pretty false`、`npm run build`、三守护脚本 + audit.mjs 只读复跑），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round90-frontend-findings.md（不存在，本轮新建）。结束态 `git status --short --branch` = `## master` 洁净（本轮报告落盘前已复核工作区无 diff），build 产物 backend/web/dist/ 已被 git 忽略（历轮已实证）。零仓库改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新真实缺陷 + 1 条低优先展示层 OBSERVE（走读推断）。** M-1 延续管理第二十六轮闭合：shouldDeferSave 四消费点全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）无第七处、target-guard 18/18 实测全绿（含 echoed 第三参 2 条）。F88-01 Admin htmlFor 第三轮复核通过（6 对 label htmlFor + Input id 逐字符配对、id 全仓唯一、全仓 htmlFor 零未配对、与 Login 同款模式）→ 归档。新视角五组扫查：会话三态一致性（login/logout/onUnauthorized/onDeleted/onBackToStudent 全路径 adminToken/inAdmin/targetAccount/page 复位闭环）、倒计时五态横幅（window_closed → window_opened → 识别缺席 → isExpired → 倒计时分级与 Dashboard 同构）、退选弹窗在飞取消路径（Escape/取消/确认三闸全带 actionLoading 守卫）、Toast 合并与滚动（合并 duration 保留首值 + viewport 80vh 有界 + pointer-events 官方模式，R55/R57 评估延续），三态完整性唯一缺口 = Dashboard 日志区无加载/失败态区分（新 OBSERVE-90-01，走读推断，展示误导零功能危害）。契约 20 扫描（轮次标签族 / 行号引用族 / XSS / localStorage try/catch / SPA 无导航能力）全仓零命中。连续第三十六轮无严重级发现。

---

## 一、M-1 延续管理（第二十六轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处调用 + Read 逐字符核对，与 R89 记录一致）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `courses 非空 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应。echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后，TDZ 不触发；F36-01 兜底守卫）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 全量 6 处）：:229（回显 effect 首行短路）、:319（独立清理 effect 守卫）、四消费点（:509/:597/:605/:699）——无第七处。
- **target-guard 断言 18/18** 实测全绿（grep -c "✓" = 18，含 echoed 第三参 2 条）。
- 第二十六轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

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

## 三、F88-01 Admin htmlFor 第三轮复核（→ 归档）

- **6 对 label htmlFor + Input id 逐字符配对**（grep -on 实测 + Read 复证）：CodesTab admin-code-count（label :374 / id :376）、admin-code-uses（label :386 / id :388）、ConfigTab admin-config-base-url（label :605 / id :607）、admin-config-api-key（label :615 / id :619，密码框）、admin-config-model（label :628 / id :630）、admin-config-concurrency（label :679 / id :681）。
- **全仓 htmlFor 扫描零未配对**：Admin 6 对 + Login 3 对（login-account :131/:138、login-password :150/:158、activation-code :257/:262）全部有对应 id；id 全仓唯一（delete-acct-modal-title、activate-dialog-title、admin-*、login-*、activation-code 均单次出现）。
- **与 Login 同款模式、git 零意外改动**（ca08c46 已核，工作区零 diff）。
- **F88-01 归档**：三连续轮复核通过，观察项可归档。

## 四、新视角扫查（换方向，五组逐项）

1. **登录/登出/401 全链路会话状态一致性（adminToken / targetAccount / inAdmin / page 在各路径的复位）**：
   - login()（:91-108）：写 sessions + 条件写 adminToken（仅响应带 adminName）+ setInAdmin(!!adminName)；撞名学生（B43-04 放行）响应无 adminName → 不置管理标记 → 渲染判据 isCurrentAdminSession 恒 false → 不误进管理页（admin-auth-check 场景 B 实测）。
   - logout()（:112-132）：删 sessions[current] + setInAdmin(false) + setAdminToken("")（:121-122）+ setTargetAccount(null)（:126）+ setPage("dashboard")（:131）——代理态与视图态同族复位。
   - account-reselect effect（:135-148）：sessions 全空 → setCurrent("") + setPage("dashboard")（:137-142）；isCurrentAdminSession 失效 → setInAdmin(false)（:147）。
   - onUnauthorized（:188-231）：lostAccount === adminName → setInAdmin(false) + setAdminToken("")（:210-215）；代理态还原（:204-206）；管理员自身会话保护守卫（:217，仍由 isCurrentAdminSession 双条件把关——撞名学生场景 adminToken 已清或 token 不匹配 → 守卫不拦 → 正确剔除）。
   - onDeleted（:160-178）：删 sessions + targetAccount 还原（:167）+ current 还原（:168）+ 删管理员自身时 setInAdmin(false) + setAdminToken("")（:172-177）。
   - onBackToStudent（:306-329）：切学生账号或完整登出（无学生账号时 sessions 清空 + current="" + page 复位），adminToken 已清。
   - **结论**：六路径复位闭环（每次吊销/删除/登出都同步清管理标记与代理态与页面态），无残留路径。零缺陷。
2. **倒计时与状态横幅五态（window_opened/window_closed/open_time 在五态下显示一致性）**：
   - Select 横幅分支（:827-853）：`!stateData → 同步中 / window_closed → 已关闭 / window_opened → 已开放 / !openTimeStr && begin_times==null → 未识别 / cd.isExpired → 等待平台开放 / 其余倒计时`。分支顺序 window_closed 优先 → window_opened → 识别缺失 → 本地过期 → 倒计时，与 Dashboard 主矩阵 + 右侧"预计开放时间"文案（:408-414）同构分级；window_opened 以调度器信号为准（服务端 ~640ms 校准偏差注释 :821-826 如实）。
   - cd = useTickingCountdown(openTimeStr ?? begin_times[0] 格式化)（:767-772）与 Dashboard :191-196 同源兜底——两页展示在「识别缺席但有 begin_times」下均走兜底倒数，跨页矛盾已收口（F39-N1/F53-M1/F54-M-E 链）。
   - 识别槽有值即显示（含已过期值，决策锚 1 语义「关闭≠时间消失」）——openTimeStr 判定 `stateData?.open_time_known && stateData.open_time`（:761-764），与 stats open_time_set 同源（handler.go:964 `!open.IsZero()`）。
   - **结论**：五态显示一致，零缺陷。
3. **退选确认弹窗在飞时的取消路径**：Escape（:1208 `!actionLoading.has(id)` 守卫）、取消按钮（:1232 disabled）、确认按钮（:1242 disabled：退选中不可再点）；退选中 Esc 不响应防误关（注释 :1199-1200 与实现一致）；autoFocus 在取消按钮（:1234，Esc 打开即生效）。与 Login 激活弹窗（:232-237 Esc + activating 守卫）、Admin 删除弹窗（:216-218 Esc + deleting 守卫）三处对称。**结论**：在飞取消路径全闭，零缺陷。
4. **Toast 系统高频合并与视图滚动**：合并逻辑（Toast.tsx:42-55）同 title 只更新 description/variant、**duration 保留首次配置**（:47-49 注释 + :50 不覆盖 duration——R54-M★「合并 duration 取最新遇 Radix 计时器重启可无限延寿」修复复核成立）；viewport `max-h-[80vh] overflow-y-auto pointer-events-none`（:105）+ toast 自身 pointer-events-auto（:78）——高堆叠（目标保存失败退避最坏 6 个）受 80vh 有界，溢出滚动残余为 Radix pointer-events 官方模式产物（R55/R57 评估延续，无新依据）。**结论**：维持历轮观察，无新缺陷。
5. **空态/加载态/错误态三态完整性**：
   - Select 课程网格：isLoading（:901-906）/ isError（:908-912）/ tabs 空空态（:916-922）/ 每 Tab 过滤空（:1182-1186）——四态齐。
   - CodesTab（:447-485）：isLoading / isError（含重试）/ data 空 / data 有——四态齐。
   - StatsTab（:763-798）：data / isError / 加载中——三态齐。
   - AccountsTab（:829-904）：isLoading / data 空 / isError / data——四态齐。
   - LogsTab（:928-950）：isLoading / logs 空 / isError / data——四态齐。
   - **Dashboard 日志区（:697-718）**：`logs && logs.length > 0 ? map : "NO RECENT LOGS"`——**仅两态**，/logs 失败或加载中时 logs=undefined 均显 "NO RECENT LOGS"（"加载中"与"失败"与"真空"三态混为一谈，成功空数据亦同文案）。
   - **结论**：唯一缺口为 Dashboard 日志区（新 OBSERVE-90-01，走读推断）——纯展示层面误导（加载中/失败期用户看到"无日志"），零功能危害、不影响轮询或数据完整性。其余全部三态/四态齐备。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮 1 条新 + 延续项管理）

**OBSERVE-90-01（新，走读推断）**：Dashboard.tsx 日志区（:697-718）缺「加载中」与「拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined，均显示 "NO RECENT LOGS"，与"确实空日志"无区分。修复方向（若未来做）：`logsQuery.isLoading → "加载中..."` / `logsQuery.isError → 错误+重试`，对齐同文件其余列表三态；一行条件即可。触发频率：/logs 每 3s 轮询，失败仅网络不可达/后端重启期间（已由 30s 降频吸收）。**评级依据**：纯展示误导、无功能危害（轮询照常、数据完整、无异常堆栈），按「宁缺毋滥」列 OBSERVE 不动作，留档防未来被误报为真实缺陷。

**OBSERVE-88-01（延续，本轮评估不建议落地）**：ErrorBoundary 缺失——复证位置无变化（main.tsx:6-10 裸 createRoot 渲染，全仓 componentDidCatch/getDerivedStateFromError 零命中）；89 轮零渲染期异常实证 + 正常路径防御充分（全站列表 `?? []` 兜底 + isError 分支），预防性复杂度不划算；后端契约破坏性变更时才值得 ~20 行落地。维持「续」。

**OBSERVE-86-01（本轮归档）**：Admin htmlFor 连续三轮复核通过，归档。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演无新触发面（末次点选与平台清空同落窗口属子秒级物理窄窗，守卫拦下后 hasPublishes 恢复驱动重试）。维持「续」。

**OBSERVE-84-01（延续）**：激活失败票据空请求文案突变——Login.tsx:87-97 两种失败分支均已清 pendingTicket，但清票据后用户再点「激活并登录」会用空 ticket 再打 /activate（后端拒"激活票据无效"）。F43-N2 已处理误导主链路，残余仅主动重复点击时第二次空请求文案；低优先级 UX 候选。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前行为无害：pubs 空时窗口未开/已关、用户无目标改动则无保存链路，非空改动走守卫置脏 + hasPublishes 恢复驱动）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口（与 83-01 关联）。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，全部指向 F6-02 Radix Dialog 迁移单一出口（历轮已定：手写裸 div 弹层三处——Login 激活 / Select 退选 / Admin 删除——行为已由注释与守卫闭环）。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第二十六轮）：四消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参、置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）无第七处、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:455/:563/:742）零回潮，unmountedRef 八读写点全与「卸载后不 fire/不 toast」契约对齐。
- F88-01 Admin htmlFor（第三轮复核→归档）：6 对 label+id 逐字符配对、id 全仓唯一、Login 3 对同款、全仓零未配对、git 零意外。
- 新视角：登录/登出/401 六路径会话复位闭环（login/logout/account-reselect/onUnauthorized/onDeleted/onBackToStudent 全带 adminToken+inAdmin+targetAccount+page 复位）；倒计时五态横幅两页同构；退选弹窗三闸在飞守卫；Toast 合并 duration 保留首值 + viewport 80vh 有界（Radix pointer-events 官方模式）；Select/Codes/Stats/Accounts/LogsTab 三态四态齐备。
- 后端交叉契约复核：handleAdminStats（handler.go:948-964）open_time/open_time_set/window_opened/window_closed/token_valid 与学生端 /state 同源（open_time_set=!IsZero() 与前端 `!== true` 保守判定一致）；handleSetTargets 发布元数据补全 enrichTargetPubMetaLocked（scheduler.go:543-570）账号专属帧优先兜底全校帧、窗口关闭后仍可读；StateForAccount courses 只回本账号 + publish_name/begin_date 透传——Dashboard 日期分组零依赖 /electives 契约在当前实现成立。
- 契约 20：轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch（六处读写全 try/catch）/ SPA 无导航能力（window.open/location.*/history.* 零命中、纯组件内存 state 拼 API path）全零命中。
- TDD 三组断言：target-guard 18/18、admin-auth 6/6、unauthorized 5/5 全绿；audit.mjs 77 项全绿。
- 手动报名/退选 Set 在飞幂等 + 双 invalidate、登录/激活链幂等 + 票据贯通 + 401 三形态单广播、btn_type 三向、max_count=0 四处同源、open_time 空格串 V8 解析（R89 实测，本轮无改动）——复跑无回潮。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0（TSC_EXIT=0，后台任务实测） |
| `cd web && npm run build` | ✅ EXIT 0（1948 modules transformed，产物 419.87 kB JS / 41.11 kB CSS 落 backend/web/dist，dist 被 git 忽略） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（grep -c "✓" = 18） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（实测 6 条） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（实测 5 条） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（grep -c "✓" = 77） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/guardBlockedRef/导航能力） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master` 洁净（本轮报告落盘前零改动） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十六轮闭合，见上。下轮继续常规核对。
- **OBSERVE-90-01（新）**：Dashboard 日志区缺加载/失败态区分——走读推断，展示误导，不动作。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-86-01**（本轮归档）：Admin htmlFor 已复核通过。
- **OBSERVE-85-01**（归档维持）：F86-01 注释口径复核连续五轮无回潮。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口。
- **OBSERVE-75-03**（延续）：Admin uses 无前端上限（Infinity→null→后端拒绝路径亦被后端值域校验兜底，本轮复证）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=87b1d53，master），未修改任何仓库文件（唯一写入为本报告文件 archive/review-rounds/round90-frontend-findings.md）。
- 走读推断与实测区分：OBSERVE-90-01（Dashboard 日志区三态）、onUnauthorized 六路径闭环、Toast duration 保留首值机制为走读推断；M-1 逐字符、F88-01 逐字符配对、三组断言计数（18/18、6/6、5/5、77）、tsc/build 退出码、契约 20 扫描、id 唯一性对比均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（走读推断，展示层）；1 条归档（F88-01 htmlFor）；延续观察管理；连续第三十六轮无严重级发现。