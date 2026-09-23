# Round 94 前端只读审查报告

基线：commit cacacc6（R93 双 findings + 收尾总结，HEAD，进度 94/256）。本轮为 R94 前端只读审查 + M-1 延续管理（第三十轮）。核心为 M-1 第三十轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 实质修复复核（git show f08937e 逐 diff + global.css 重置交互 + 语义正确性）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：F93-01 落地后无障碍焦点管理横向复核 / 视觉与状态反馈一致性 / 数据请求并发边界）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch / 导航能力全形态）。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / handleState / handleAdminCodes / windowClosedLocked / StateForAccount / ElectivesSnapshotFor）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/show/log、`npx tsc -b --pretty false`、`npm run build`、三守护脚本 + audit.mjs 只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round94-frontend-findings.md（存在则覆盖）。结束态 `git status --short --branch` = `## master`；build 产物落 backend/web/dist（git check-ignore 历轮已确认被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十轮闭合：shouldDeferSave 四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级置位 3 + 守卫 2 处 :229/:319 + 消费点 4 处 = 全部代码级引用 9 处，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 实质修复复核通过：git show f08937e 仅 Button.tsx +4 行零意外改动；focus-visible ring 语义正确（键盘触发/鼠标不显示/全站 Button 收敛）；残余面（10+ 处裸按钮无焦点环）与 R93 报告口径一致，属 OBSERVE-93-01 已声明延续而非新发现。六防保存链逐条重读零回潮；setSelected 六代码调用点与 dirtyRef 置 true 仅三处（:455/:563/:742）复核零漂移。连续第四十轮无严重级发现。

---

## 一、M-1 延续管理（第三十轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R93 记录逐行一致）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。测试 18/18 中 echoed 第三参 2 条（:70/:76 场景）覆盖稳态放行与首帧未到仍推迟两极端。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后，TDZ 不触发；F36-01 兜底守卫）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 排除注释行后代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——全 src 目录 `shouldDeferSave(` 调用点 grep 仅命中此四处 + targetGuard.ts 自身定义；echoedRef 读点与 R93 完全一致，无漂移。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项，含 echoed 第三参 2 条）。
- 第三十轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :343/:347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **unmountedRef 全读写点**（grep 实测）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。

## 三、F93-01 Button focus-visible 实质修复复核

- **git show f08937e 逐 diff**（实测）：单文件 Button.tsx +4/-0，唯一改动为 base class 补一行 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`（:42）+ 3 行注释（:39-41）。零意外改动，与提交统计一致。
- **global.css 重置交互上下文**（实测）：:169-176「基础交互重置」对 `button, input, select, textarea` 统一 `outline: none`——原生 focus 环确被全量抹掉，F93-01 补偿确有必要。Button.tsx 现 `focus` 关键字命中 4 处（:40/:41 注释 + :42 样式行），从 R93 的 0 命中变为有补偿。
- **语义正确性验证**：`focus-visible:` 变体由浏览器 `:focus-visible` 伪类驱动——键盘 Tab 聚焦时触发（键盘导航焦点环可见），鼠标点击聚焦时不触发（鼠标点击不显示），与 Admin 开关（:583 `focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（:173 同款）语义一致。全站所有 `<Button>` 实例（grep `variant="` 命中 30+ 处，四路由全覆盖）经 base class 一次收敛。
- **色值与既有裸按钮的参数差异**（观察级）：Button 用 `ring-[var(--cyan)]`（= #ffffff 纯白不透明）+ `ring-offset-[var(--bg)]`（纯黑 offset 2px）；Admin 开关/Login 密码切换用 `ring-white/60`（半透明白）无 offset。二者视觉上都清晰可见（纯白环在纯黑背景），差异仅为透明度/offset 参数，属设计参数未完全统一而非缺陷。归入 OBSERVE-93-01 备注，不单独立条（宁缺毋滥）。
- **残余面确认**：裸按钮（非 Button 组件）无焦点环清单与 R93 报告完全一致——Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100，`focus:outline-none` 无补偿）。这是 OBSERVE-93-01 已声明延续（「其余裸按钮另择时处理」），非新发现。
- F93-01 复核通过：实质修复正确落地，零意外改动，语义与既有焦点可见性样式一致，残余面维持观察。

## 四、新视角扫查（本轮选定 3 项）

### 1. 无障碍焦点管理横向复核（F93-01 落地后）

- **全站 Tab 键盘导航焦点顺序**：四路由 DOM 结构线性（顶栏 → 倒计时/状态 → 主内容 → 操作区），无固定定位元素插入 Tab 序（移动端底部悬浮栏 :725-746 是 `sm:hidden` 移动专属，桌面不参与 Tab 序；底部留白 :1195 为 aria-hidden 空 div 不参与）。焦点顺序 = 视觉顺序，无异常跳转。
- **focus-visible 覆盖范围**：Button 组件全站收敛 + Input 自带 focus ring + Admin 开关/Login 密码切换自带——覆盖「主要交互元素」。残余面为裸按钮 10+ 处（见第三节），OBSERVE-93-01 延续。
- **弹窗焦点陷阱与焦点恢复**（O-3 族延续）：三个模态（Login 激活 :223-316 / Admin 删除 :210-284 / Select 退选 :1201-1250）均为裸 div role=dialog + aria-modal，无焦点陷阱（Tab 可穿出背景表单）、无关闭后焦点归还（关闭后焦点落 body，键盘用户需重新 Tab 找回）。初始焦点落位已补：Login 激活输入框 autoFocus（:267）、Admin 删除取消按钮 autoFocus（:241）、Select 退选取消按钮 autoFocus（:1234）——初始落位正确、Esc 关闭全链可用（三模态均有 onKeyDown Escape 处理，退选中/删除中/激活中不响应防误关）。焦点陷阱/归还仍指向 F6-02 Radix Dialog 迁移单一出口，本轮无新依据提级。
- 结论：无新增实质发现；O-3 族 + OBSERVE-93-01 残余面延续。

### 2. 视觉与状态反馈横向一致性

- **Dashboard 日志区（:696-719）与 Admin 日志总览（:927-951）三态差异**：Admin LogsTab 有完整四态（加载中 / 有日志 / 失败+重试 / 空），Dashboard 日志区只有两态（有日志 / "NO RECENT LOGS"）——/logs 加载中与失败态均落到 "NO RECENT LOGS" 误导。OBSERVE-90-01 延续项，位置无变化（:696-719 原样）。
- **「一行条件对齐」低优先级候选评估**：对齐需从 useQuery 解构 isLoading/isError + 两分支 JSX（非严格「一行」），且 /logs 失败已 30s 降频、纯展示误导零功能危害、Dashboard 顶部已有 state 错误条（isSessionError）覆盖主要失效路径——维持「续」不落地，理由充分。
- **其余状态反馈横查**：Select 加载中/错误/空态三态齐全（:901/:908/:916）、Admin 各 Tab 三态齐全、Dashboard 目标空态带引导按钮（:536-546）——一致性无新缺口。
- 结论：仅 OBSERVE-90-01 延续，无新发现。

### 3. 数据请求的并发边界

- **fetch 通道收敛**：全仓唯一 fetch 点为 client.ts:58（`await fetch(BASE + path, ...)`），无 XMLHttpRequest/axios/裸 `fetch` 散落。AbortController 20s 超时 + 调用方 signal 优先（:55-58）全覆盖。
- **手动操作超时语义**：selectElective/exitElective（client.ts:112-138）未传 signal，依赖 20s 兜底超时；超时抛「请求超时，请重试」友好文案（:102-104），且 Select 报名/退选 finally 恒 invalidateQueries（:103-104/:128-129）——超时但服务端实际已处理的状态由刷新自愈，无悬挂竞态。
- **react-query 全查询 retry:1**（App.tsx:13）+ 失败态降频（三路由 refetchInterval 函数式，/state 30s 降频、/logs 30s 降频、/electives 30s 降频）——失败轰炸与超时堆积已双防。
- **logs 查询 key 无 account 的正确性复核**：Dashboard :148 `queryKey: ["logs", sessionToken]` 不含 account——后端 /logs 按 sessionAccount 过滤、sessionToken 是会话令牌（恒唯一绑定账号），多账号各有不同 token 即缓存天然隔离；与 Admin logs 含 account（管理员跨账号场景）语义一致，无串线。
- 结论：无新增发现。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理）

**OBSERVE-93-01（F93-01 已实质修复主通道，残余面延续）**：Button 组件 focus-visible ring 已于 commit f08937e 实质落地（+4 行零意外，本轮 git show 复核通过），键盘 Tab 聚焦全站 `<Button>` 焦点环可见。**残余面**：10+ 处裸按钮（非 Button 组件）仍无焦点环——Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。另有焦点环参数未完全统一（Button `ring-[var(--cyan)]` 纯白无 offset vs 既有 `ring-white/60` 半透明）——视觉均可见，纯参数差异。修复建议：裸按钮可在 F6-02 Radix Dialog 迁移时一并收敛或另行低优先统一（若统一可顺手把 ring 参数并到同一 token）。触发场景：键盘用户 Tab 导航折叠段/管理列表操作时上述元素无焦点定位。严重度论证：纯键盘可用性残余面，无数据/安全/功能危害，OBSERVE 级延续。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:696-719）缺「加载中」与「拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined，均显示 "NO RECENT LOGS"。本轮重读位置无变化（:696-719 两态结构原样）、零功能危害维持留档。低优先级候选「一行条件对齐其余列表三态」评估：需解构两个新变量 + 两分支 JSX，非严格一行且纯展示误导，维持「续」不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分，本轮无新依据提级。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 :64-101 确认清票三路径（/票据无效分支 :88 / 通用失败分支 :95 / 取消 :303/:235）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前行为无害）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。本轮核对 handleBack 双击并发（返回按钮无在飞守卫，双击并发两实例）：两实例 flush 均走 savingRef 串行化、onDone 幂等（setPage/setTargetAccount 均幂等）、卸载后 unmountedRef 短路后续 saveNow——幂等无害，列入已核无缺陷而非新发现。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，全部指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）——「焦点进入」侧已闭环，「焦点陷阱 + 焦点归还」侧仍缺口。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十轮）：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 七读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：git show f08937e 单文件 +4 行零意外；focus-visible 语义正确（键盘触发/鼠标不显示）；全站 Button 收敛；Admin 开关/Login 密码切换/Input 自带焦点样式不受影响。
- 契约 20 扫描：轮次前缀标签族 / 行号引用族（`\b\w+\.tsx:\d+\b|\b\w+\.ts:\d+\b` 全形态）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML / outerHTML / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中，纯组件内存 state 拼 API path）全仓零命中。
- 后端交叉契约复核（沿用 R92/R93 定位，本轮重读关键点）：handleSetTargets（handler.go:444-526）凭据表 accountExists 校验 → 条数上限 100 → class_id/publish_id/priority 四校验 → 双 SetTargets → AppendLog 失败记日志零吞错；handleState（:529-548）透传账号存在性校验 + AccountsWithTargets 兜底；windowClosedLocked（scheduler.go:918-939）三条判据单源 + open 单快照复用（:924）；StateForAccount（:699-725）同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤；ElectivesSnapshotFor（:761-789）目标账号判据 len>0 + 专属帧过期即 (nil,false) 绝不回退全局帧。
- 数据并发边界横查（本轮新视角）：全仓唯一 fetch 通道（client.ts:58）+ AbortController 20s 超时 + 调用方 signal 优先全覆盖；手动操作超时由 finally invalidateQueries 自愈；react-query 全查询 retry:1 + 失败态降频双防轰炸；Dashboard logs key 无 account 的正确性（令牌即账号隔离）复核成立。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、open_time 空格串解析、Progress aria-valuenow 与 unannounced 语义一致、Dashboard 日期分组本地零点无时区偏移、Toast 同 title 去重合并防轰炸。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0（TSC_EXIT:0，后台任务实测） |
| `cd web && npm run build` | ✅ EXIT 0（BUILD_EXIT:0，built in 490ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✗=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中（唯一误命中为 Dashboard.tsx 注释中的日期串 "2026-09-13" 与 target-guard-check.ts 测试数据 "2026-09-20"，均 ISO 日期字面量） |
| `git status --short --branch` | ✅ `## master`（HEAD=cacacc6）；除本报告外零改动 |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：F93-01 已实质修复主通道（Button 组件 +4 行零意外，git show 复核通过）；残余面 = 裸按钮 10+ 处（与 R93 清单逐项一致）+ ring 参数未统一（纯白 vs 半透明白，均可见）。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——「一行条件对齐」候选评估维持不落地（需解构两变量 + 两分支 JSX，纯展示误导零功能危害）。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三模态初始焦点落位 + Esc 全链已闭环，陷阱/归还仍缺口）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。
- **handleBack 双击并发**（本轮新核）：无在飞守卫但幂等无害（savingRef 串行化 + onDone 幂等 + unmountedRef 短路），列入已核无缺陷。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本 + audit.mjs + node 纯只读实测）；工作区 `git status` 零改动（HEAD=cacacc6，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单（裸按钮焦点环）、O-3 族聚焦陷阱/焦点恢复、OBSERVE-90-01 三态差异为走读推断（静态链确凿）；M-1 逐字符、三组断言计数（18/18、6/6、5/5）、audit.mjs 77 项、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git show f08937e diff、git rev-parse、后端契约 grep 定位、Dashboard logs key 无 account 正确性推演均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥——裸按钮残余面与 ring 参数差异均为 OBSERVE-93-01 已声明延续面，不重复立条）；M-1 第三十轮闭合；连续第四十轮无严重级发现。
