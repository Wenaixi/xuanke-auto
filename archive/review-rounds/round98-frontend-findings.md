# Round 98 前端只读审查报告

基线：commit 86522e8（R97 双 findings + 收尾总结，HEAD，进度 98/256）。本轮为 R98 前端只读审查 + M-1 延续管理（第三十四轮）。核心为 M-1 第三十四轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：定时器/轮询资源消耗 / 全局状态与本地状态边界 / 表单完整交互流 / 代码组织可维护性）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级）。审查范围：web/src 全部 .ts/.tsx（8 组件原语 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs），交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / handleAdminCodes / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log/check-ignore、`npx tsc -b --pretty false`、`npm run build`、四守护脚本只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round98-frontend-findings.md。结束态 `git status --short --branch` = `## master`（无 untracked）；build 产物落 backend/web/dist（git check-ignore 实测 backend/web/dist/web 与 index.html 均被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十四轮闭合：四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过：Button.tsx:42 ring 行逐字符在位、语义正确、全站收敛、残余面与历轮清单一致（git log 实测自 f08937e F93 提交后 web/ 目录无代码提交，该行未被改动）。六防保存链逐条重读零回潮；setSelected 六代码调用点与 dirtyRef 置 true 仅三处（:455/:563/:742）复核零漂移。连续第四十四轮无严重级发现。

---

## 一、M-1 延续管理（第三十四轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R97 记录逐行一致，零漂移）：
  - 防抖回调 :699 `if (shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current))`
  - flushTargets :509 `if (shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current))`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——全 src 目录 `shouldDeferSave(` 调用点 grep 仅命中此四处 + targetGuard.ts 自身定义 + scripts/target-guard-check.ts 测试断言；echoedRef 读点与 R97 完全一致（grep 全量命中含注释核对无漂移）。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项 = shouldDeferSave 8 + selectedHasStalePublish 5 + cleanStaleSelected 5，含 echoed 第三参 2 条）。
- 第三十四轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

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

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :343/:347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **消费时刻读最新 ref**（grep 实测）：selectedRef（:173-174）/ revRef（:175-176）/ stateDataRef（:180-181）/ publishesRef（:663-664）四镜像的消费点全部位于防抖回调/flush/handleBack 消费时刻；pick 直接用渲染 selected（:349/:353/:363）因其「事件处理器内 selected 恒为最近已提交渲染值」论证（:346-347）成立（React 事件批处理只在单事件内合并，两次独立点击间必有渲染提交）。零旧闭包路径。

## 三、F93-01 Button focus-visible 持续复核

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R97 记录逐字符一致。git log 实测 f08937e（F93 提交）→ HEAD 无任何 `web/` 代码提交（`git log --oneline -- web/` 最新代码提交即 f08937e，之后仅 docs 提交），该行未被改动。
- **语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发。global.css:169「基础交互重置」对 button 统一 `outline: none`（本轮重读逐字在位 :169-176），ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处经 base class 一次收敛；Admin 开关（Admin.tsx:583 `focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login.tsx:173 同款）、Input 自带 focus ring（Input.tsx:14 `focus:ring-1 focus:ring-[var(--cyan)]`）并存无冲突。Button ring 用 `--cyan`（= #ffffff 纯白）+ offset 2px，既有用 `ring-white/60`（半透明白）无 offset——均清晰可见，纯参数差异（OBSERVE-93-01 备注，不单独立条）。
- **残余面确认**（与 R93-R97 清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。本轮逐一 grep 复核这些行号对应裸 button 仍无 ring（与历轮清单一致）。OBSERVE-93-01 延续。
- F93-01 第六轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新视角扫查（本轮选定 4 项）

### 1. 定时器/轮询的资源消耗（react-query refetchInterval 叠加 useTickingCountdown 每秒重渲染）

- **互斥挂载峰值盘点**（grep 实测）：
  - Dashboard 挂载态：/state 3s（窗口关闭/失败 30s，:140-145）+ /logs 3s（关闭/失败 30s，:156-161）+ /electives 恒 30s（:174）+ useTickingCountdown 1s setInterval（useTickingCountdown.ts:13）——合计 3 个轮询 + 1 个定时器。
  - Select 挂载态：/electives 2s/10s/30s 动态（:58-82）+ /state 2s/30s（:148-155）+ 1s 定时器——合计 2 个轮询 + 1 个定时器。
  - Dashboard 与 Select 在 App 渲染互斥（page 条件渲染 + targetAccount 分支优先），绝不同时挂载——峰值恒为「3 轮询 + 1 定时器」。
- **react-query 后台标签页行为**（走读推断）：refetchInterval 轮询默认受 document.visibilityState 约束（tab 不可见时暂停 interval refetch，回到前台恢复）——后台挂机不空转网络。useTickingCountdown 的 setInterval(1000) 是裸 effect 不受此约束，后台 tab 仍每秒 setNow → 触发宿主整树重渲染；浏览器对后台 tab 定时器本身有最低节流（约 1s 合并/降频），叠加效果进一步收敛。窗口关闭后（isExpired 恒 true）定时器仍每秒跑、仍整树重渲染——此形态已在 useTickingCountdown.ts:4-7 / Dashboard.tsx:178-180 注释如实口径为「潜在优化、非当前承诺」，历轮 OBSERVE-85-01 收尾后未承诺。低端设备单用户选课场景影响可忽略，**维持历轮口径不新立条**。
- 结论：资源消耗处于合理量级（互斥挂载 + 后台暂停 + 注释如实），无新增发现。

### 2. 全局状态与本地状态的边界（App 层 state vs 路由组件 state 职责划分）

- **App 层全局态**（sessions/page/current/inAdmin/targetAccount/adminName/adminToken，:71-83）：会话/视图态跨组件生命周期，归属 App 正确；渲染优先级 targetAccount（代理态）> inAdmin（管理态）> page（学生视图），:291-348 分支结构清晰。
- **路由层本地态**：Select 的 selected/rev/echoDone/actionLoading 等编辑态、Dashboard 的 expandedDates/extrasOpen、Admin 的 activeTab/pendingDelete 等 UI 态——全部组件内私有，无跨路由泄漏。activeTab 受控化（Select :189-192 / Admin :56）解决发布重建/Tab 消失悬空，未提升到 App 层（进出页面重置 Tab 是合理行为，Admin.tsx:54-55 注释已述「跨挂载保留需提升」非承诺）。
- **跨组件共享点**：queryClient 缓存按 [entity, account, sessionToken] 分键——Dashboard/Select 同 key 同 URL 共享 /electives 缓存（Dashboard.tsx:165-166 注释），切页零重复请求；账号维度键隔离保证多账号互不串数据。划分合理。
- 结论：全局/本地状态边界清晰，无越权提升或不当下沉，无新增发现。

### 3. 表单完整交互流（输入→校验→提交→loading→结果→清空全链路）

- **Login 主表单**：输入（受控 account/password）→ 校验（:37-39 空账号/空密码提示）→ 提交（:31-62 loading 置位 + 在飞幂等短路 :36）→ loading 反馈（按钮 Loader2 + 文案「正在连接教务认证...」+ Input disabled 硬防重）→ 结果（成功 onLogin 写入会话；失败 error 条，:57 非 1001 分支）→ 清空（成功切页组件卸载；失败保留输入供修改，error 条展示）。闭环完整。
- **激活码 modal**：1001 弹窗（:51-55 存账号+票据）→ 输入校验（:69-71 空码提示）→ 激活（:64-101 activating 幂等 :68 + disabled）→ 失败分文案（:87-97 票据过期/激活码错误双分支，均清票避免滞留旧票重试误导）→ 取消（:299-311 清账号/票/错误）。Esc 关闭（:232-238 守卫 activating）在位。
- **Admin 表单**：生成（count/uses 输入 :381-393 实时夹取 1-100/1-1000 → generating 幂等 → 成功 setGenerated + toast + refetch）、配置（loaded 前保存锁定 :526/:710、saving 幂等 :528、留空不改 key 语义 :539、保存后 refetch 用生效真值回填 :546-549、成功后清空密钥框 :553）、删除账号（pendingDelete → deleting 幂等 :252 → 成功 invalidate + onDeleted 本地同步 + toast）。全部在飞/校验/反馈闭环。
- 结论：表单全链路闭环无缺口，无新增发现。

### 4. 代码组织的可维护性（组件拆分粒度 / hook 封装 / DRY）

- **拆分粒度**：ui/ 八原语（Button/Input/Card/Badge/Progress/Tabs/Toast）+ lib/ 三 hook（useTickingCountdown/adminAuth/targetGuard）+ api/ 客户端 + 四路由——职责单一、复用收敛。Card 五件套（CardHeader/Title/Description/Content/Footer）标准 shadcn 形态。
- **DRY 检查**：`priorityName` 在 Dashboard.tsx:31-33 与 Select.tsx:41-43 各定义一份（两处 3 行纯函数重复）；`localTodayMs` 仅 Dashboard 使用但 export 供测试（:120-124，无重复）；`extractAccountFromPath` 在 client.ts:16-22 已抽纯函数供 scripts 测试复用（unauthorized-check 5/5 实证）。priorityName 两处重复属轻微（各自 3 行、语义自明、改动面独立），历轮未立条，维持。
- **注释密度**：Select.tsx 保存链注释极详（六防防线每判据一段"为什么/契约"），可读性与工程记忆库契约一一对应；契约 20 轮次标签族零残留（grep 实测，见契约 20 扫描）。
- 结论：组织可维护性良好，priorityName 两处重复不足以立条，无新增发现。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理）

**OBSERVE-93-01（延续，第六轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核）+ 语义正确（键盘触发/鼠标不显示/全站收敛）；残余面（Dashboard CollapseSection :67、Admin 复制/刷新/删除/收起/引擎切换/重试 :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）本轮逐一 grep 复核与 R93-R97 清单逐项一致无漂移；ring 参数未统一（纯白+offset vs 半透明白）值均可见。保持「续」。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:697-718）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化（:697 `{logs && logs.length > 0 ? ... : :714-717 <NO RECENT LOGS>}`，无 isLoading/isError 解构分支；对照 Admin LogsTab :928-950 完整四态）。零功能危害维持留档。**「一行条件对齐」候选评估（第二轮）**：对齐需解构 isLoading/isError + JSX 两分支（约 8 行），非一行改动；且 /logs 失败时后端通常不可达（其他同信道查询也显示错误条），日志区误导影响面极小。维持不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演 handleBack 三轮 flush 收敛（:611-642）与等待窗口内 pick 新改动（等待期间 setRev → effect 挂 timer 异步 → 等一帧复查 revRef :622-624 已覆盖）无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（票据无效分支 :87-90 / 通用失败分支 :91-97 / 取消 Esc :232-238 + 按钮 :301-306）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害：等发布恢复/用户改动重跑）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）+ Tab 顺序自然——焦点进入侧闭环，陷阱 + 归还侧仍缺口。

**无障碍残留面备注（延续，非新立条）**：(a) Toast 无显式 aria-live 容器（走读推断：Radix ToastPrimitive Viewport 内建 aria-live=polite，待浏览器实测）——本轮重读 Toast.tsx:105 Viewport 组件存在、语义由 Radix 内部实现，与前轮一致；(b) prefers-reduced-motion 无分支；(c) Progress 缺 aria-valuetext。三项均维持。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十四轮）：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第六轮复核，git log 实证 f08937e 后 web/ 零代码提交）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新视角四项（本轮）：定时器/轮询资源合理（互斥挂载峰值 3 轮询+1 定时器、react-query 后台暂停、倒计时注释如实）；全局/本地状态边界清晰（App 会话态 + 路由编辑态、渲染优先级 targetAccount>inAdmin>page）；表单全链路闭环（Login/激活/Admin 生成配置删除在飞+校验+反馈+清空）；组织可维护性良好（ui/lib/api/routes 分层、priorityName 两处 3 行重复不足以立条）。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}|M-1|F\d{2}A?|B\d{2}-` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。
- 后端交叉契约复核（本轮重读关键点）：handleSetTargets 凭据表 accountExists 校验（handler.go:444-486）→ 无透传取核心账号兜底（:478-485）→ 条数上限 100（:499-502）→ 字段校验（:503-516）→ 双 SetTargets + AppendLog 失败记日志零吞错（:517-524）；handleState 透传账号存在性校验 + 无透传核心账号兜底（handler.go:529-548）；windowClosedLocked 三条判据单源 + open 单快照复用（scheduler.go:918-939，判据 2 syncFailStreak≥3 带「开放时间已过」、判据 3 EmptyProbeRuns≥3 带 prevOpened 守卫）；StateForAccount 同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤（scheduler.go:699-725）；handleAdminCodes 三态（GET 列表 / POST 生成单事务 + 随机 panic recover 语义 / DELETE 空 body 合法，handler.go:615-678）与 router 的 requireJSONBody 分层（router.go:134-139）。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、useTickingCountdown target 变化校正 now（useTickingCountdown.ts:19-21）、Dashboard 日期分组本地零点（parseDateKey/localTodayMs）、Toast 同 title 去重合并、Admin 复制 clipboard 降级链、Tabs 受控化（Admin :161 / Select :929）、Progress max=1 空条兜底（Select :1102-1103）。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 495ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✖=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=86522e8）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十四轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第六轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零代码提交）；残余面清单逐字无漂移。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:697-718），对齐候选评估第二轮维持不落地。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲，本轮推演 handleBack 三轮收敛已覆盖。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三项 autoFocus 落位 + Esc 全链已闭环 + Tab 顺序自然，陷阱/归还仍缺口）。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 四守护脚本 + git status/rev-parse/log/check-ignore）；工作区 `git status` 零改动（HEAD=86522e8，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单、O-3 族焦点陷阱/焦点恢复、新视角「react-query 后台标签页暂停 interval 轮询」「浏览器后台 tab 定时器节流」「render 分支互斥挂载」、无障碍残留面 (a) 为走读推断；M-1 逐字符、四组断言计数（18/18、6/6、5/5、audit 77 项）、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git rev-parse/log/check-ignore、后端契约 grep 定位、setSelected/dirtyRef 清点均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第三十四轮闭合；连续第四十四轮无严重级发现。
