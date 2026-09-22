# Round 92 前端只读审查报告

基线：commit 62d8da7（R91 双 findings + 收尾总结，HEAD，进度 92/256）。本轮为 R92 前端只读审查 + M-1 延续管理（第二十八轮）。核心为 M-1 第二十八轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（组件卸载与定时器清理的完整性 / 账号切换时的缓存与状态隔离 / 数字格式与进度语义 / 视觉与交互细节人性化 / 深链与初始状态五组）；契约 20 扫描（含历轮扫描模式盲区揭示）；三组断言 + audit.mjs + tsc + build 复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / handleState / windowClosedLocked / StateForAccount / handleAdminStats 开放时间序列化）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git log/status/rev-parse、`npx tsc -b --pretty false`、`npm run build`、三守护脚本 + audit.mjs 只读复跑、grep 各类扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round92-frontend-findings.md（不存在，本轮新建）。结束态 `git status --short --branch` = `## master`，`git rev-parse --short HEAD` = 62d8da7；build 产物 backend/web/dist 已被 git 忽略（实测 `git check-ignore backend/web/dist` 命中）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新真实缺陷 + 1 条新 OBSERVE（注释卫生类，走读推断）+ 延续观察管理。** M-1 延续管理第二十八轮闭合：shouldDeferSave 三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符复核通过、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）完整、读写点全量清点（置位 3 + 读点 6）无第七处、target-guard 18/18 实测全绿。六防保存链逐条重读零回潮。新视角五组扫查全部收敛：① 组件卸载与定时器清理（useTickingCountdown / Admin copyTimer / Select unmountedRef / client.ts 超时四族全量对称清理，react-query 轮询无裸 setInterval）；② 账号切换缓存与状态隔离（queryKey 含 account+sessionToken 五处、key={account} 重建、adminToken 标记一致性、sessions 快照式三连）；③ 数字格式与进度语义（fillRate 100% 封顶 / isFull 同源判据 / max_count=0 未公布三处 / Progress value+max / 排序 remaining 映射 0 / 倒计时全 00 过期态边界全对）；④ 视觉与交互人性化（加载/空/错误三态各列表齐全、剪贴板 execCommand 双兜底、Toast 合并、按钮在飞守卫、aria 横向一致）；⑤ 深链与初始状态（空 sessions 回登录、无账号进页、管理令牌持久化降级、StrictMode 双挂载吸收）。契约 20 扫描：轮次前缀标签族（字面 "(第N轮)" 零命中）、行号引用族、XSS、localStorage try/catch（六处全包）、SPA 导航能力（零命中）全仓零命中；**同时揭示历轮扫描模式盲区**——Select.tsx 有 5 处轮次编号引用（F7 / 32-01×2 / 33-01 / 32-02）从未被历轮"轮次标签族"扫描记录，按契约 20"历史残留发现即剥离"意图评为新 OBSERVE-92-01。连续第三十八轮无严重级发现。

---

## 一、M-1 延续管理（第二十八轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处调用 + Read 逐字符核对，与 R91 记录一致）：
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
- **读点全量清点**（grep 排除注释行后代码级 6 处）：:200（写）/ :229（回显 effect 首行短路）/ :240、:297（写）/ :319（独立清理 effect 守卫）/ 四消费点（:509/:597/:605/:699）——代码级读写 6 点、无第七处。本轮另核 :168/:172/:507/:594/:603/:696 等注释性引用均不含代码级读写。
- **target-guard 断言 18/18** 实测全绿（grep -c "✓" = 18，含 echoed 第三参 2 条 :70/:76）。
- 第二十八轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

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

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **unmountedRef 全读写点**（grep 实测）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。

## 三、新视角扫查（本轮五组）

1. **组件卸载与定时器/轮询清理的完整性**：
   - 全仓 setInterval 仅 1 处（lib/useTickingCountdown.ts:13 每秒自 tick），cleanup 对称 clearInterval（:14）——react-query refetchInterval 由库托管，无裸 setInterval 残留。
   - setTimeout 全量清点：client.ts:56 超时 abort → finally clearTimeout（:107）；Admin copyTimer 三重（:80/:101 覆盖旧 timer + :114-119 卸载清理）；Select retryState.timer（:404 卸载清理 + :410 resetRetry + :747 防抖 timer cleanup）；handleBack 两处 await 内联 setTimeout 属一次性等待（无需清理）。
   - App 滚动/resize rAF 全量 removeEventListener + cancelAnimationFrame（:271-276）；UNAUTHORIZED_EVENT 挂载/卸载对称（:232-233）。**结论**：零缺陷，定时器/事件无泄漏路径。
2. **账号切换时的缓存与状态隔离**：
   - queryKey 全维核对（grep 全量 7 条查询键）：`["state", account, sessionToken]`（Select :146 / Dashboard :132）/ `["electives", account, sessionToken]`（Select :56 / Dashboard :172）/ `["logs", sessionToken]` / Admin 五 Tab 全含 account+sessionToken（:317/:503/:724/:818/:913）——账号与令牌双维度隔离，切账号零串扰。
   - Select 双挂载点 key={account}（App :294/:340）+ 兜底守卫 :195-202；Admin 注释明言 queryKey 必须含 account（:313-315），账号切换即重建实例。
   - 管理令牌隔离：`isCurrentAdminSession` 仅「adminToken 非空 && sessions[adminName]===adminToken」判据，撞名学生普通会话 token 永不匹配；logout/onDeleted/onUnauthorized/onBackToStudent 全路径清 adminToken。**结论**：零缺陷。
3. **数字格式与进度语义（边界值 0/满员/过期）**：
   - fillRate `!max_count 返回 0` + `Math.min(100, ...)` 封顶；isFull `max_count>0 && selected_count>=max_count` 与后端 IsClassFull 同源判据（Select :1006）；unannounced `max_count<=0` 三分支（徽章/进度色/排序 remaining 映射 0）四处同源。
   - Progress value=0 max=1 空条（名额未公布不渲染假满条）、aria-valuenow 如实反映 0；倒计时 diff<=0 全 00 + isExpired，24 小时制 hour12:false 锁死。**结论**：零缺陷。
4. **视觉与交互细节人性化**：
   - 各列表加载/空/错误三态齐全（Admin 五 Tab 全带 isLoading/isError/空态分支 + Retry 按钮；Dashboard 日志区缺口见 OBSERVE-90-01 延续）。
   - 剪贴板 `execCommand("copy")` 双兜底 + readOnly 加固（Admin :75-99）；Toast 同 title 合并防轰炸（:40-56）；按钮 `active:scale-[0.98]` 与 disabled 语义；空态引导（"前往挑选课程"按钮 / "窗口开放后课程列表将自动出现"）。**结论**：零缺陷。
5. **深链与初始状态**：空 sessions 进页渲染 Login（App :346-348）；account-reselect effect 无账号回登录 + page 复位（:136-142）；管理令牌持久化六处 localStorage 全 try/catch 降级内存态（App :18-68）；StrictMode 双挂载由 unmountedRef 挂载复位吸收（:399-401）。**结论**：零缺陷。

## 四、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮新增 1 条 + 延续项管理）

**OBSERVE-92-01（新增，注释卫生，走读推断）**：Select.tsx 存在 5 处历史轮次编号引用——:168 `F7 修复后`、:172 `（32-01）`、:612 `33-01：`、:617 `32-01：`、:965 `32-02：`。契约 20 字面禁止的是"轮次前缀标签（X-XX（第 N 轮））"形态——这 5 处不带"（第 N 轮）"字样，因此历轮契约 20 扫描（正则 `第 ?[0-9]+ ?轮`）与此类零命中、报告一贯声称"轮次前缀标签族零命中"技术上自洽。但契约 20 的意图是「轮次决策历史统一落本手册，代码注释只写"为什么/契约/陷阱"本身，历史残留发现即剥离」——`:172` 行尾挂号与 :612/:617/:965 行首前缀属"轮次编号锚点残留"形态，与历史 commit（122bad2 F7-01 / 第 32 轮 32-01/32-02 / 第 33 轮 33-01）一一对应。**历轮扫描模式未覆盖此形态，属审计盲区揭示**。功能零危害（注释级），且每处注释本身已承载完整"为什么"语义（剥离编号后仍可读）。评估：低优先注释卫生项，维持「续」不落地（与 OBSERVE-90-01 同级处置）；建议未来轮次在契约 20 扫描中追加行尾编号锚点正则，或在清理轮次统一剥离时一并处理。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:697-718）缺「加载中」与「拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined，均显示 "NO RECENT LOGS"。本轮重读位置无变化（:696-719 两态结构原样）、零功能危害维持留档。低优先级候选（一行条件对齐其余列表三态）评估：本轮无触发证据（/logs 每 3s 轮询、失败已 30s 降频，纯展示误导），维持「续」不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分（全站列表 `?? []` 兜底 + isError 分支），本轮无新依据提级。维持「续」。顺带观察：react-query 5.x 的 QueryErrorResetBoundary 语义不适用（查询未包裹，无渲染期抛错路径），提级依据依旧缺席。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演无新触发面（末次点选与平台清空同落窗口属子秒级物理窄窗，守卫拦下后 hasPublishes 恢复驱动重试）。维持「续」。

**OBSERVE-84-01（延续）**：激活失败票据空请求文案突变——Login.tsx:87-97 两种失败分支均已清 pendingTicket，但清票据后用户再点「激活并登录」会用空 ticket 再打 /activate（后端拒"激活票据无效"）。F43-N2 已处理误导主链路，残余仅主动重复点击时第二次空请求文案；低优先级 UX 候选。本轮重读 :64-101 确认清票三路径（/票据无效分支 :88 / 通用失败分支 :95 / 取消 :303/:235）与 R91 记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前行为无害：pubs 空时窗口未开/已关、用户无目标改动则无保存链路，非空改动走守卫置脏 + hasPublishes 恢复驱动）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口（与 83-01 关联）。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，全部指向 F6-02 Radix Dialog 迁移单一出口。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 五、已核无缺陷清单

- M-1 延续管理（第二十八轮）：三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参、置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（代码级置位 3 + 读点 4 消费 + 守卫 2 处，无第七处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅两处生产语义（:455 真实失败 + :563/:742 飞行中补发对称对），unmountedRef 七读写点全与「卸载后不 fire/不 toast」契约对齐。
- 新视角五组：定时器/事件全对称清理无泄漏；queryKey 账号+令牌双维隔离无串扰；数字边界（0/满员/过期全 00）四处同源无越界；三态/剪贴板/Toast 合并/aria 人性化横向一致；深链初始状态全路径无死角。
- 后端交叉契约复核：handleSetTargets（handler.go:444-526）凭据表 accountExists 校验 → Store.SetTargetsForAccount → Sched.SetTargetsForAccount 顺序、nil→[] 规整、条数/class_id/publish_id/priority 四校验、空 targets 合法清空；handleState（:529-548）透传账号存在性校验 + AccountsWithTargets 兜底；windowClosedLocked（scheduler.go:918-943）三判据单源（主判据 state.WindowClosed / 时钟连续失败≥3+开放时间已过 / 幽灵窗口 EmptyProbeRuns≥3+开放时间已过，open 单快照复用），StateForAccount（:703-728）同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤；handleAdminStats（:891-965）RecognizedOpenTime → Format("2006-01-02 15:04:05") 本地时区序列化 + open_time_set=!IsZero() 与前端 undefined→"未识别"保守兜底对齐（Admin.tsx:735 `!== true` 双保险）、window_closed 与学生端同源三态展示。
- 契约 20：轮次前缀标签族（字面形态）/ 行号引用族（`Xxx.tsx:N`）全仓零命中；XSS 危险模式（innerHTML/outerHTML/document.write/dangerouslySetInnerHTML/new Function/eval 零命中）；localStorage 六处读写全 try/catch 降级；SPA 无导航能力（window.open/location.*/history.* 零命中、纯组件内存 state 拼 API path）。5 处轮次编号引用形态属模式盲区（见 OBSERVE-92-01）。
- TDD 三组断言：target-guard 18/18、admin-auth 6/6、unauthorized 5/5 全绿；audit.mjs 77 项全绿（✓=77，✗=0）。
- 手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137 全路径）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95 防双发）、btn_type 三向、max_count=0 四处同源、open_time 空格串 V8 解析（R89/R91 结论，本轮由 handler.go:901 序列化点与前端 new Date 解析链路逐级对齐复证）——复跑无回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0（TSC_EXIT=0，实测） |
| `cd web && npm run build` | ✅ EXIT 0（built in 552ms，产物 419.87 kB JS / 41.11 kB CSS 落 backend/web/dist，git check-ignore 确认 dist 被忽略） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（grep -c "✓" = 18） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（grep -c "✓" = 6） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（grep -c "✓" = 5） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（grep -c "✓" = 77，✗ = 0） |
| 契约 20 残留扫描（字面轮次标签族/行号族/XSS/localStorage try/catch/导航能力） | ✅ 零命中（5 处轮次编号锚点形态见 OBSERVE-92-01） |
| `git status --short --branch` | ✅ `## master`（HEAD=62d8da7）；除本报告外零改动 |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十八轮闭合，见上。下轮继续常规核对。
- **OBSERVE-92-01**（新增，注释卫生）：Select.tsx 5 处轮次编号锚点（F7 / 32-01×2 / 33-01 / 32-02）——历轮契约 20 扫描模式盲区揭示，功能零危害，维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——走读推断，展示误导，不动作。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地（react-query 5.x QueryErrorResetBoundary 不适用，无提级依据）。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口。
- **OBSERVE-75-03**（延续）：Admin uses 无前端上限（Infinity→null→后端拒绝路径亦被后端值域校验兜底）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本 + audit.mjs + node 纯只读实测）；工作区 `git status` 零改动（HEAD=62d8da7，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-92-01（注释卫生/契约 20 意图判定）、OBSERVE-90-01（日志区三态）、O-3 聚焦陷阱、F7 编号对应历史 commit 考证（git log -S 实测 122bad2）为走读/实测混合；M-1 逐字符、三组断言计数（18/18、6/6、5/5、77）、tsc/build 退出码、契约 20 扫描、git check-ignore、git rev-parse、后端契约 grep 定位均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新 OBSERVE-92-01（注释卫生）；延续观察管理；连续第三十八轮无严重级发现。