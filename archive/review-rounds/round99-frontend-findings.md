# Round 99 前端只读审查报告

基线：commit faacafe（R98 双 findings + 收尾总结，HEAD，进度 99/256）。本轮为 R99 前端只读审查 + M-1 延续管理（第三十五轮）。核心为 M-1 第三十五轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核（第七轮）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：状态一致性 / 视觉 token 一致性 / 数据请求模式 / 交互细节）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级）。审查范围：web/src 全部 .ts/.tsx（8 组件原语 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs），交叉核对 backend/internal/{api,scheduler} 契约（handleAdminCodes / handleSetTargets / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log/check-ignore、`npx tsc -b --pretty false`、`npm run build`、四守护脚本只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round99-frontend-findings.md。结束态 `git status --short --branch` = `## master`（无 untracked）；build 产物落 backend/web/dist（git check-ignore 实测 index.html 与 assets 下两个产物文件均被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十五轮闭合：四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过（第七轮）：Button.tsx:42 ring 行逐字符在位、语义正确、全站收敛、残余面清单与历轮一致（git log 实测自 f08937e F93 提交后 web/ 目录无代码提交，该行未被改动）。六防保存链逐条重读零回潮；setSelected 六代码调用点与 dirtyRef 置 true 仅三处（:455/:563/:742）复核零漂移。连续第四十五轮无严重级发现。

---

## 一、M-1 延续管理（第三十五轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R98 记录逐行一致，零漂移）：
  - 防抖回调 :699 `if (shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current))`
  - flushTargets :509 `if (shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current))`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
  - grep 全量命中与 R98 完全一致：`shouldDeferSave(` 全 src 仅 targetGuard.ts:64 定义 + 上四处消费 + scripts 测试引用，无第五消费处。
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应，第三参 echoed 只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**（grep 实测与 R98 记录逐字符一致）：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——echoedRef 全量 grep 命中 24 行（含注释）与 R98 一致，代码级 9 处零漂移。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项 = shouldDeferSave 8 + selectedHasStalePublish 5 + cleanStaleSelected 5，含 echoed 第三参 2 条）。
- 第三十五轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **消费时刻读最新 ref**（grep 实测）：selectedRef（:173-174）/ revRef（:175-176）/ stateDataRef（:180-181）/ publishesRef（:663-664）四镜像的消费点全部位于防抖回调/flush/handleBack 消费时刻；pick 直接用渲染 selected（:349/:353/:363）因其「事件处理器内 selected 恒为最近已提交渲染值」论证（:346-347）成立（React 事件批处理只在单事件内合并，两次独立点击间必有渲染提交）。零旧闭包路径。

## 三、F93-01 Button focus-visible 持续复核（第七轮）

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R98 记录逐字符一致。git log 实测 f08937e（F93 提交）→ HEAD 无任何 `web/` 代码提交（`git log --oneline -- web/` 最新代码提交即 f08937e，之后仅 docs 提交），该行未被改动。
- **语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发。global.css:169「基础交互重置」对 button 统一 `outline: none`（本轮重读逐字在位 :169-176），ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处经 base class 一次收敛；Admin 开关（Admin.tsx:583 `focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login.tsx:173 同款）、Input 自带 focus ring（Input.tsx:14 `focus:ring-1 focus:ring-[var(--cyan)]`）并存无冲突。
- **残余面确认**（与 R93-R98 清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。本轮逐一 grep 复核这些行号对应裸 button 仍无 ring（与历轮清单一致）。OBSERVE-93-01 延续。
- F93-01 第七轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新视角扫查（本轮选定 4 项）

### 1. 状态一致性（派生状态 vs 服务端数据源的同步时机）

- **Select 受控 Tabs 值 vs 服务端发布集合重建**：受控 value 表达式 `activeTab && tabs.some(...) ? activeTab : String(tabs[0].publish_id)`（:929）——发布集合重建（publish_id 全变）后 tabs.some 失败即回落首个 Tab，绝不悬空（R99 重读 :185-189 注释与实现一致）。值恒为 tabs 成员，服务端数据源变化驱动的 Tabs 状态与本地 activeTab 同步无缺口。
- **Dashboard 折叠种子 vs dateGroups 重建**：`expandedDates===null && dateGroups.length>0` 只种一次（:270-274），null/[] 语义分离（:266-268）——用户全折叠后数据刷新不重拉；重建后旧日期组消失、新日期组出现时 expandedDates 残留旧 key，CollapseSection 的 `expandedDates?.includes(g.key) ?? false` 对新组恒 false（默认折叠，安全方向），不悬空。
- **ConfigTab 表单回填 vs 后端生效值**：加载成功回填（:512-520 依赖 loaded）；保存成功后 `refetch → 成功才 setConfigEpoch`（:546-549）——refetch 失败不自增代际、表单保留用户输入不被陈旧值覆盖，后端已生效但表单显示用户输入（下次进入/刷新对齐），同步时序设计完整。
- **Progress value/max vs 服务端名额**：`unannounced ? 0 : selected_count` / `unannounced ? 1 : max_count`（Select :1101-1105）——max_count=0 未公布名额时 value=0/max=1 恒空条，aria-valuenow 如实反映（注释 :1097-1100），服务端数值到渲染的派生链完整。Progress 组件内部 `percentage = clamp(value/max*100)`（Progress.tsx:12）max=1 时 0 除 0 被 clamp 兜底（0/1=0）。
- 结论：派生状态与数据源同步时机完整，无状态一致性缺陷。

### 2. 视觉细节（token 一致性与 tabular-nums 覆盖）

- **调色板 token 一致性**（global.css:4-42 逐字重读）：全部颜色均走 CSS 变量（--bg/--surface/--surface-soft/--surface-muted/--border 族/--fg 族/单色语义灰阶映射），组件内无裸十六进制色值散落（grep 实测全仓 `#[0-9a-fA-F]{3,8}` 零命中于组件 className——唯一例外 Select.tsx:1213/Admin.tsx:220 弹窗内容卡 `bg-[#09090b]` 为近似 --surface 的面板底色，与 --surface(#0c0c0e) 属同一黑白族，历轮已定级为「弹窗深底不透明分隔」不立条）。
- **tabular-nums 覆盖清点**（grep 实测 9 处）：Dashboard 倒计时四格 + 运行指标 30 秒/courses.length + Admin Stats 行值 + Accounts 目标/成功两列 + Select 倒计时 + 容量统计——全部倒计时/人数/状态数字均带 tabular-nums（或父级 font-mono 恒宽）。select 无数字跳动失对齐残留（"30 秒查询节流"、Badge 内数字等单值/静态场景无跳动需求，不属遗漏）。
- **圆角体系**：--radius-sm/md/lg/xl 全站统一，弹窗（lg）、按钮（md/sm）、输入框（md）、徽章（sm）、进度条（full 圆）映射一致；无大圆角/重阴影（shadow-none 或 shadow-xl 仅弹窗），符合纯黑白极简设计语言。
- 结论：视觉 token 一致性良好，tabular-nums 全覆盖，无新增发现。

### 3. 数据请求模式（api 封装的一致性 / 错误分类 / 超时 / abort）

- **单一 fetch 封装**（client.ts:38-109）：全站请求一律走 api()，`Content-Type: application/json` 统一、`Authorization: Bearer <session>` 统一、20s 超时兜底 + 调用方 signal 显式优先（:55-58，注释 :51-53 说明 rest.signal 优先防 ctrl.signal 被覆盖）、abort 映射友好文案（:102-104）。无散落裸 fetch。
- **错误分类链路**：HTTP 401 先广播（r.json() 前，:64-69）→ body code=401 补广播（:75-95，HTTP 200 旧形态）→ 非零 code 抛 ApiError（:97）→ 解析失败 -2（:73）。三类形态各单次广播、abort 单映射。链完整。
- **超时覆盖**：GET /electives 大列表与 PUT /targets 报名均走同一 20s 兜底；targets 保存另有指数退避重发兜底（saveNow scheduleRetry）。admin 各 Tab 轮询走 react-query 默认（无单独超时，由同一 api 兜底）。
- **invalidate/refetch 使用一致性**：手动报名/退选后 invalidate electives+state 双查询（Select :103-104/:128-129）；Admin 删除后 invalidate admin-accounts + onDeleted 快照清除（:262/:268）；各 Tab 重试按钮 refetch() 同款。无遗漏失效的写后查询。
- 结论：请求模式统一、错误分类完整、超时/abort 全覆盖，无新增发现。

### 4. 交互细节（多输入支持 / hover/focus/active 反馈）

- **键盘导航**：三个模态（Login 激活 / Select 退选 / Admin 删除）全部 role=dialog + aria-modal + aria-labelledby + Esc 关闭（激活中/退选中/删除中不响应防误关）+ 取消 autoFocus（:241/:267/:1234）——键盘可达性闭环（O-3 族焦点进入侧）。
- **Tabs 键盘 roving focus**：Admin/Select Tabs 走 Radix TabsPrimitive（Tabs.tsx:5 原语转发），键盘方向键/Home/End roving focus 由 Radix 内部实现；受控 value 不破坏（Admin.tsx:52-55 注释明确）。
- **hover/focus/active 反馈**：Button active:scale 收缩（:38）；全站 hover 白化/边框变亮（--border-hover 体系）；Input focus ring（Input.tsx:14）；移动端 tap-highlight 清除 + 平滑（global.css:194-198）。玻璃表面 hover 状态与纯黑白语言一致。
- **点击/触摸**：所有操作按钮带 disabled 渲染 + 在飞守卫双闸（submit/activate/generate/remove/select/exit/delete/save 全入口核对：:36/:68/:327/:349/:92/:119/:252/:528）；搜索/筛选/排序交互即时响应。
- 结论：多输入支持与状态反馈完整，无新增发现。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理）

**OBSERVE-93-01（延续，第七轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核）+ 语义正确（键盘触发/鼠标不显示/全站收敛）；残余面（Dashboard CollapseSection :67、Admin 复制/刷新/删除/收起/引擎切换/重试 :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）本轮逐一 grep 复核与 R93-R98 清单逐项一致无漂移。保持「续」。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:697-718）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化（:697 `{logs && logs.length > 0 ? ... : :714-717 <NO RECENT LOGS>}`，无 isLoading/isError 解构分支；对照 Admin LogsTab :928-950 完整四态）。**「一行条件对齐」候选评估（第三轮）**：对齐需解构 isLoading/isError + JSX 两分支（约 8 行），非一行改动；且 /logs 失败时后端通常不可达（其他同信道查询也显示错误条），日志区误导影响面极小。维持不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演 handleBack 三轮 flush 收敛（:611-642）与等待窗口内 pick 新改动（等待期间 setRev → effect 挂 timer 异步 → 等一帧复查 revRef :622-624 已覆盖）无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（票据无效分支 :87-90 / 通用失败分支 :91-97 / 取消 Esc :232-238 + 按钮 :301-306）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害：等发布恢复/用户改动重跑）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖 :241/:267/:1234）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）+ Tab 顺序自然——焦点进入侧闭环，陷阱 + 归还侧仍缺口。

**无障碍残留面备注（延续，非新立条）**：(a) Toast 无显式 aria-live 容器（走读推断：Radix ToastPrimitive Viewport 内建 aria-live=polite，待浏览器实测）——本轮重读 Toast.tsx:105 Viewport 组件存在、语义由 Radix 内部实现，与前轮一致；(b) prefers-reduced-motion 无分支；(c) Progress 缺 aria-valuetext。三项均维持。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十五轮）：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第七轮复核，git log 实证 f08937e 后 web/ 零代码提交）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新视角四项（本轮）：状态一致性（Select 受控 Tabs 值回落 / Dashboard 折叠种子 null-[] 分离 / ConfigTab 回填代际 / Progress max=1 空条兜底）；视觉 token 一致性（全 CSS 变量、tabular-nums 9 处全覆盖、圆角体系统一、唯一 #09090b 弹窗底属黑白族历轮已定级）；数据请求模式（单一 api 封装、401 三形态单广播、20s 超时 + signal 优先 + abort 映射、invalidate/refetch 一致性）；交互细节（三模态键盘闭环、Radix roving focus、hover/focus/active 反馈全、操作按钮双闸全覆盖）。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}|M-1|F\d{2}A?|B\d{2}-` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。
- 后端交叉契约复核（本轮重读关键点）：handleSetTargets 凭据表 accountExists 校验（handler.go:444-486）→ 无透传取核心账号兜底（:478-485）→ 条数上限 100（:499-502）→ 字段校验（:503-516）→ 双 SetTargets + AppendLog 失败记日志零吞错（:517-524）；handleState 透传账号存在性校验 + 无透传核心账号兜底（handler.go:529-548）；windowClosedLocked 三条判据单源 + open 单快照复用（scheduler.go:918-939，判据 2 syncFailStreak≥3 带「开放时间已过」、判据 3 EmptyProbeRuns≥3 带 prevOpened 守卫）；StateForAccount 同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤（scheduler.go:699-725）；handleAdminCodes 三态（GET 列表 / POST 生成单事务 + 先全量生成再落库防半批滞留 / DELETE 空 body 合法，handler.go:615-684）与 router 的 requireJSONBody 分层（router.go:134-139）。
- Dashboard「当前未添加任何预选课程」空态历史追溯闭合（本轮新考据）：该空态在历轮中已被完整论证为「确证无目标」路径——dateGroups.length===0 只由 courses 空驱动（:536），window_closed 后 courses 非空（/state.courses 为持久化目标状态不随窗口关闭清空）→ 正常显示终态卡片，空态只在真无目标时出现。首帧在途（/state 未到、courses=[]）与真无目标在 dateGroups 层无法区分 → 「当前未添加任何预选课程」+ 前往挑选按钮误显约 1-2 秒（加载期噪声，到达即自愈），无害。历史 R84/R73/R75 已确认同类秒级窗属加载期噪声。不立条。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、useTickingCountdown target 变化校正 now（useTickingCountdown.ts:19-21）、Dashboard 日期分组本地零点（parseDateKey/localTodayMs）、Toast 同 title 去重合并、Admin 复制 clipboard 降级链、Tabs 受控化（Admin :161 / Select :929）、Progress max=1 空条兜底（Select :1102-1103）。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 537ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist（哈希与 R98 逐字节一致，web/ 零代码改动实证） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✖=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=faacafe）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十五轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第七轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零代码提交）；残余面清单逐字无漂移。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:697-718），对齐候选评估第三轮维持不落地。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲，本轮推演 handleBack 三轮收敛已覆盖。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三项 autoFocus 落位 + Esc 全链已闭环 + Tab 顺序自然，陷阱/归还仍缺口）。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 四守护脚本 + git status/rev-parse/log/check-ignore）；工作区 `git status` 零改动（HEAD=faacafe，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单、O-3 族焦点陷阱/焦点恢复、新视角「Dashboard 空态首帧在途秒级窗」「react-query 后台标签页行为」、无障碍残留面 (a)、Dashboard 空态历史追溯中「/state 首帧在途与真无目标无法区分」为走读推断；M-1 逐字符、四组断言计数（18/18、6/6、5/5、audit 77 项）、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git rev-parse/log/check-ignore、后端契约 grep 定位、setSelected/dirtyRef/echoedRef 清点、tabular-nums 9 处覆盖均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第三十五轮闭合；连续第四十五轮无严重级发现。
