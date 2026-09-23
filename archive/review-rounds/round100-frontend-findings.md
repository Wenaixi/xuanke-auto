# Round 100 前端只读审查报告

基线：commit 9af1de0（R99 双 findings + 收尾总结，HEAD，进度 100/256；连续七轮双端零代码修改的纯观察轮**里程碑收官**）。本轮为 R100 前端只读审查 + M-1 延续管理（第三十六轮）。核心为 M-1 第三十六轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核（第八轮）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：首次加载与首屏体验 / 状态机完整图 / 用户输入的边界 / 响应式与多设备 / 代码健壮性最后一道）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级）。审查范围：web/src 全部 .ts/.tsx（8 组件原语 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs），交叉核对 backend/internal/{api,scheduler} 契约（handleAdminCodes / handleSetTargets / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log/check-ignore、`npx tsc -b --pretty false`、`npm run build`、四守护脚本只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round100-frontend-findings.md。结束态 `git status --short --branch` = `## master`（无 untracked）；build 产物落 backend/web/dist（git check-ignore 实测 index.html 与 assets 下两个产物文件均被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十六轮闭合：四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过（第八轮）：Button.tsx:42 ring 行逐字符在位、语义正确、全站收敛、残余面清单与历轮一致（git log 实测自 f08937e F93 提交后 web/ 目录零代码提交，该行未被改动）。六防保存链逐条重读零回潮；setSelected 六代码调用点与 dirtyRef 置 true 仅三处（:455/:563/:742）复核零漂移。里程碑视角评估：OBSERVE-90-01 与 OBSERVE-88-01 连续多轮观察后仍无落地依据，维持观察（详见第四节）。连续第四十六轮无严重级发现。

---

## 一、M-1 延续管理（第三十六轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R99 记录逐行一致，零漂移）：
  - 防抖回调 :699 `if (shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current))`
  - flushTargets :509 `if (shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current))`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
  - grep 全量命中与 R99 完全一致：`shouldDeferSave(` 全 src 仅 targetGuard.ts:64 定义 + 上四处消费 + scripts 测试引用，无第五消费处。
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；第三参 echoed 只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**（grep 实测与 R99 记录逐字符一致）：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——echoedRef 全量 grep 命中 24 行（含注释）与 R99 一致，代码级 9 处零漂移。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项 = shouldDeferSave 8 + selectedHasStalePublish 5 + cleanStaleSelected 5，含 echoed 第三参 2 条）。
- 第三十六轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

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

## 三、F93-01 Button focus-visible 持续复核（第八轮）

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R99 记录逐字符一致。git log 实测 f08937e（F93 提交）→ HEAD 无任何 `web/` 代码提交（`git log --oneline -- web/` 最新代码提交即 f08937e，之后仅 docs 提交；`git log f08937e..HEAD -- web/` 实测空集），该行未被改动。
- **语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发。global.css:169「基础交互重置」对 button 统一 `outline: none`（本轮重读逐字在位 :169-176），ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处经 base class 一次收敛；Admin 开关（Admin.tsx:583 `focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login.tsx:173 同款）、Input 自带 focus ring（Input.tsx:14 `focus:ring-1 focus:ring-[var(--cyan)]`）并存无冲突。Button ring 用 `--cyan`（= #ffffff 纯白）+ offset 2px，既有用 `ring-white/60`（半透明白）无 offset——均清晰可见，纯参数差异（OBSERVE-93-01 备注，不单独立条）。
- **残余面确认**（与 R93-R99 清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。本轮逐一 grep 复核这些行号对应裸 button 仍无 ring（与历轮清单一致）。OBSERVE-93-01 延续。
- F93-01 第八轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新视角扫查（本轮选定 5 项；里程碑轮扩展覆盖）

### 1. 首次加载与首屏体验（SpaHandler 兜底 / React 挂载时机 / 路由初始状态 / loading 骨架）

- **单页入口与 SPA 兜底**（embed.go 逐字重读）：`//go:embed all:dist` 嵌入前端产物（:11）；SpaHandler 首行 `/api` + 精确 `/api` 全部 404（:28-31，与 router.go:194 `/api/` 显式 404 双保险整链闭合——R92 契约「/api 前缀整体 404 不落 index.html」零回潮）；静态文件存在由 FileServer 正确 Content-Type 响应（:38-41）；index.html 读失败 404（:45-47）。**已知挂载点缺失首屏骨架注释**：index.html:9 仅 `<div id="root"></div>`（13 行总长，无 loading/骨架屏占位）；main.tsx:6 `createRoot(getElementById('root')!)` 非空断言 + StrictMode；createRoot 前无任何首屏绘制——白屏窗口 = JS 下载/解析/执行时间（生产产物 420KB gzip 125KB，555ms 内构建，加载 <1s 级）。对照：React 挂载后四个路由全部自带 loading 分支（Select :901-906 骨架 + :908-912 错误条 + :916-922 空态 / Dashboard :318-329 错误条 + 首帧空态 / Admin 各 Tab :447/:829/:928 loading + 错误重试），**挂载前**根部白屏是 SPA 通用形态（fallback content 属优化项）。`document.getElementById('root')!` 非空断言在 index.html 恒含 #root 下安全（TDD/契约无覆写 #root 路径）。
- **路由初始状态**：App page:72 初始 "dashboard"（登录后落 Dashboard，合理）；sessionToken 恢复自 localStorage 立即进入对应分支（渲染期同步判定 :291 `sessionToken ? ...`），无首帧闪烁。
- **Dashboard 首帧空态**：dateGroups.length===0 时「当前未添加任何预选课程」误显约 1-2s（/state 首帧在途）——历轮已论证为加载期噪声（可达即自愈），维持不立条。本轮独立重走一遍论证：`courses = state?.courses ?? []`（:182）→ dateGroups 空 → 空态渲染；/state 到达后 courses 非空或确证无目标 → 空态消失或保持（语义正确）。加载期噪声 <2s、无害、自愈，不立条。
- 结论：首屏链路完整（SPA 兜底 404 闭环 + 挂载后全路由 loading/错误/空态四态齐备），根部白屏属 <1s 级通用形态且无 abort 竞态，无新增发现。

### 2. 状态机完整图（登录/登出/401/删除/代理切换的视图迁移路径穷举）

- **状态维度**：`{ sessions: map, current, page, inAdmin, targetAccount, adminName, adminToken }` 七维。
- **渲染优先级**：targetAccount（代理态）> Admin（inAdmin || isCurrentAdminSession）> page=select > dashboard >（无 sessionToken）Login——:291-348 结构清晰。
- **路径穷举**（每条均有对应到达/退出代码，全部走审）：
  - **登录成功**（login :91-108）：写入会话 + setCurrent + setInAdmin(!!adminName)；管理员登录响应带 adminName 直进管理页（F52-M1 绑定「签发带 adminName」判据零回潮）。
  - **主动登出**（logout :112-132）：后端 /api/logout 吊销 + 本地删会话快照 + setInAdmin(false) + 清 adminToken 标记 + setTargetAccount(null) + setPage("dashboard")——五态全复位；Assess：`delete next[current]` 前 `const next = {...loadSessions()}` 从最新快照删（幂等，与 login 同步链路同款），无 stale 覆盖。
  - **401 被动吊销**（onUnauthorized :187-235）：反查归属账号（先账号名直查、再令牌反查 :194-197）→ targetAccount 清除（代理态退出）→ lostAccount===adminName 时 inAdmin(false) + 清管理标记 → **管理员自身会话在管理态代理学生时不清管理态**（:217-219 `isCurrentAdminSession && inAdmin && lostAccount !== adminName → return` 提前返回，不误杀管理员）；最后快照式删会话 :225-230。路径完整，无死锁/残留。
  - **删除账号**（onDeleted :160-178）：快照删 + targetAccount 若为被删账号置 null + current 若为被删账号置 "" + 被删是管理员时 inAdmin(false) + 清管理标记。与效应链（account-reselect :143-145 自动切剩余账号）衔接无冲突（F39-M1 对称处理零回潮）。
  - **代理切换**（Admin onSelectAccount → setTargetAccount → targetAccount 分支优先渲染 Select key={targetAccount} 整体重建；onDone → setTargetAccount(null) 回落）。key={account} 挂载点（App :293-298/:339-344）双分支零回潮。
  - **onBackToStudent**（:306-329）：切学生端，无其他学生账号时完整登出（清空 sessions + current + targetAccount）——「无学生账号 = 管理员退出全部会话」语义（注释 :316-319）到位，不会弹回 Admin（setCurrent("") 后 account-reselect 因 sessions 空不触发）。
  - **登出后重登**（page=select 残留防护）:130-131/:140-142——logout 与全清空 effect 双复位 page="dashboard" 零回潮。
  - **无账号回登录页**（accounts.length===0 effect :136-142）：setCurrent("") + setPage("dashboard") 对称复位。
- **撞名学生**（账号名=管理员名但普通教务会话）：登录响应无 adminName → setInAdmin(false) → 渲染 Admin 分支 `isCurrentAdminSession(sessions, adminName, adminToken)` 判据 = sessions[adminName](其普通 token) === adminToken(未标记, 空串) → false → 学生端。判据链完整（F52-M1 核心判据逐字复核），无 403 死锁。
- **循环依赖检查**：account-reselect effect 依赖 [accounts, current, adminName, adminToken]（:148）；onDeleted 中 setCurrent("") 后 effect 会因 accounts 不空把 current 设为 accounts[0]（首账号）——**若当前账号恰为首账号**：setCurrent("") 被 effect 弹回 accounts[0]（=被删账号已从 accounts 移除 → 弹到新首账号），**无悬浮 current**（point: 被删账号已从快照删、accounts 不再含它，弹回必是新账号）；若被删账号不是 current，setCurrent 不触发，无影响。无死锁路径。
- 结论：视图状态机九条路径全部穷举走审，登出/401/删除/代理/撞名/回退各链复位完整，无死锁、无残留、无 403 锁死，无新增发现。

### 3. 用户输入的边界（超长文本 / 特殊字符 / emoji / 空白符在各输入框的处理）

- **输入面盘点**（grep 实测全部 input 8 处 + 搜索 1 处）：Login 账号/密码/激活码、Select 搜索、Admin 生成数量/可用次数/并发上限/配置三框（baseURL/API key/模型）。
- **账号**（Login :141）：无 maxLength（后端凭据表定长，超长即登录失败提示，无注入面——纯字符串比较，无 SQL/命令拼接）。特殊字符/emoji 均作字符串透传，登录接口 body JSON 编码，无注入。
- **密码**（Login :159 type=password）：同账号，纯字符串透传，登出后控件保留值（组件卸载即销毁，无持久化——**密码绝不落 localStorage**，grep 实测无任何 password 写存储）。
- **激活码**（Login :264）：`font-mono tracking-widest` 排版、trim 后发送（:78），后端激活码比较为常量时间恒等，无注入。
- **搜索**（Select :869）：字符串 includes 匹配（:959-963 课程名/教师/教室），`toLowerCase()` 双方转小写；超长搜索串仅性能面（数千课以内 includes 微秒级，无 DoS 面）；空串恒真返回。**React 默认转义**——搜索词无 dangerouslySetInnerHTML，无 XSS 面（契约 20 扫描零命中复证）。
- **数字三框**（Admin :377-393/:682-687）：`Number(e.target.value)` + Math.min/max 每击夹取（count 1-100 / uses 1 以上 / concurrency 1-16），NaN || 1 兜底；`type=number` 阻止非数字（但可手输 → 夹取兜底）。uses 无前端上限（后端 1-1000 兜底，OBSERVE-76-02 延续）。
- **配置三框**（Admin :606-635）：文本透传，POST body JSON，无注入；API key 为 type=password。
- **空白符**：账号/激活码/配置留白 trim 后发送（:47/:78/:533-536/:539），密码不 trim（密码含空格合法）；搜索不 trim（子串匹配语义，首尾空格仅影响匹配行为，无害）。
- **特殊字符/emoji**：全部作字符串处理，无 HTML 拼接；Toast/Render 均 React 默认转义（保护 title/description）。尾部类内联 `title={c.title}`（Select :1122/:1135）为原生 HTML title 属性——React 转义属性值，无 XSS。
- 结论：输入边界处理统一、无注入面、无危险渲染，无新增发现。

### 4. 响应式与多设备（移动端 / 桌面端、窄屏断点、侧栏折叠）

- **断点架构**（grep 实测）：全站 `sm:`(640px)/`lg:`(1024px) 双断点 + Dashboard 加 `md:`(768px) 课程卡 3 列；Select 网格 `grid-cols-1 sm:2 lg:3 xl:4`；移动底栏 `sm:hidden` 悬浮操作（Dashboard :725-746）；顶栏 `flex-col sm:flex-row` 折叠；搜索栏 `flex-col sm:flex-row`。无 `flex-wrap` 遗漏（TabsList `flex flex-wrap` Select :933 / Admin 徽章 wrap :852）。
- **横向溢出检查**（走读推断 + 结构证据）：body `overflow-x:hidden`（global.css:70）兜底横向滚动；长课程名 `line-clamp-1`/`truncate`；日志/结果 `break-all`；**唯一允许横滚** = Admin 账号表格 `overflow-x-auto`（:832）与 select/video 类（无）——桌边表格窄屏横滚属正确模式，非缺陷。Select Tab 容器 `w-full sm:w-auto flex flex-wrap` 说明窄屏不换行而是横向堆叠（flex-wrap 兜底）。
- **移动端触控**：tap-highlight 清除（global.css:194-198）、底部悬浮栏 `inset-x-4` 边距（:725）、Select `pb-28 sm:pb-24` 底部留白防遮挡悬浮操作（:775）——两路由均处理。
- **背景画布**：移动 180% / 桌面 160%（global.css:90-114 @media(hover:hover)&(pointer:fine) 判定 + App.tsx:245-277 滚动冻结单机制）——跨屏一致。
- **骨架/空态**：Select 空态 :916-922 / Dashboard 空态 :536-546 在移动/桌面双形态下居中展示（p-16/p-8 + flex-col）。
- **倒计时四格**：`grid-cols-4` 恒 4 列（:364），`p-3 sm:p-5` 窄屏压缩内边距——384px 四格依然可读（每格约 90px，形 2xl:4xl 字号窄屏 1.5rem，无溢出）。
- 结论：响应式断点齐全、移动底栏/横滚白名单/背景画布三族全走查，无新增发现。

### 5. 代码健壮性的最后一道（可选链 / 空值兜底 / 类型完整性的全扫描）

- **可选链/空值兜底**（grep 实测全仓清单）：Dashboard `electives?.begin_times?.[0]`（:193/:208/:410）、`state?.window_closed`（:343/:346/:730）、`state?.open_time_known && state.open_time`（:186-187）、`pubById.get(p.publish_id)?.begin_date`（:227—**注意 `?.` 在 `.slice(0,10)` 之前**：`(c.begin_date ?? "").slice(0,10) || (pubById.get(...)?.begin_date ?? "").slice(0,10)`——紧接着 `?? ""` 兜底、空串 slice 安全，无 `undefined.slice` 崩溃面）；Select `stateData?.open_time_known && stateData.open_time`（:761-764）、`data?.publishes?.length ?? 0`（:662）、`data?.begin_times?.[0]`（:769/:858-859）；Admin `loaded?.vision_api_key_masked`（:616）、`s.open_time_set !== true`（:735）、`s.captcha_engine === "ddddocr"`（:747）、`Object.values(s.token_valid ?? {})`（:775）、`accountsQuery.data.length` 前置 `data &&`（:831）。全仓 `?.`/`??` 使用均在「未定义时保证安全默认」语义，无 try/catch 吞错空兜底。
- **类型完整性**：`tsc -b` EXIT 0（全量类型检查通过）；PostToolUse 幂等 `ta.readOnly = true`（Admin :91）readOnly 属性补全；`req.Targets == nil` → 空数组（后端 :493-495）与前端 `{targets: []}` 契约匹配。
- **DOM 边界**：`document.querySelector('.canvas-bg-img')` 防御式空判断（App :247-248）、textarea 内存驻留 + append/remove（Admin :86-95）。`getElementById('root')!` 断言在静态 HTML 恒含下成立（见第 1 项论证）。
- **fetch/abort**：api() 20s 超时 + 调用方 signal 优先（client.ts:55-58，`rest.signal ?? ctrl.signal` 防覆盖）；401 三种形态（HTTP 401 先广播 / HTTP 200 body 401 补广播 / 双码 401 单广播）各单次事件（:64-95 双守卫）——事件风暴与误杀双防护。
- 结论：可选链/空值兜底覆盖充分、无 `undefined.x` 崩溃面、tsc 全量通过、DOM 引用全防御，无新增发现。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理 + 里程碑视角评估）

**OBSERVE-93-01（延续，第八轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核）+ 语义正确（键盘触发/鼠标不显示/全站收敛）；残余面（Dashboard CollapseSection :67、Admin 复制/刷新/删除/收起/引擎切换/重试 :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）本轮逐一 grep 复核与历轮清单逐项一致无漂移。保持「续」。

**OBSERVE-90-01（延续，里程碑第三轮评估）**：Dashboard.tsx 日志区（:697-718）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化（:697 `{logs && logs.length > 0 ? ... : :714-717 <NO RECENT LOGS>}`，无 isLoading/isError 解构分支；对照 Admin LogsTab :928-950 完整四态）。**里程碑视角评估（第三轮）**：影响面 = /logs 失败时仅此一区文案误导（其余同信道查询各有错误条/加载态）；真实场景中 /logs 与 /state 同 Backend 同信道，/state 失败时 Dashboard 顶部错误条已提示「凭据失效」或 react-query 重试静默，日志区 NO RECENT LOGS 属**同故障的次位表现**；修复成本 ≈ 8 行（解构 isLoading/isError + 两分支），收益 = 故障时原语文案准确。连续三轮无新依据（无用户报告该误导、无真实故障样本），且「NO RECENT LOGS」在真无日志时语义正确——**维持观察不落地**；若未来收到「窗口开放但日志区空白无从判断原因」的真实反馈再落地（届时对齐 Admin LogsTab 四态即一行注释 + 三行 JSX）。

**OBSERVE-88-01（延续，里程碑第四轮评估）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分（前端除可选链空值兜底外、React 渲染数据源均为已校验类型）。**里程碑视角评估（第四轮）**：历轮「观察不落地」依据 = 渲染期异常源几无（所有 data 均经 api() 校验 + 类型契约）、且引入 ErrorBoundary 需新增组件 + 包裹层（破坏现有零 wrapper 的纯粹性）——真实影响 ≈ 0 且修复引入复杂度，**维持观察不落地**；若未来真发生渲染期异常（黑屏 + 无提示），届时再加（届时可顺带补 fallback UI = 「刷新」按钮，一并解决无提示面）。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演 handleBack 三轮 flush 收敛（:611-642）与等待窗口内 pick 新改动（等待期间 setRev → effect 挂 timer 异步 → 等一帧复查 revRef :622-624 已覆盖）无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（票据无效分支 :87-90 / 通用失败分支 :91-97 / 取消 Esc :232-238 + 按钮 :301-306）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害：等发布恢复/用户改动重跑）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖 :241/:267/:1234）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）+ Tab 顺序自然——焦点进入侧闭环，陷阱 + 归还侧仍缺口。

**无障碍残留面备注（延续，非新立条）**：(a) Toast 无显式 aria-live 容器（走读推断：Radix ToastPrimitive Viewport 内建 aria-live=polite，待浏览器实测）——本轮重读 Toast.tsx:105 Viewport 组件存在、语义由 Radix 内部实现，与前轮一致；(b) prefers-reduced-motion 无分支；(c) Progress 缺 aria-valuetext；(d) index.html 根部无首屏骨架（SPA 通用形态，<1s 级白屏）。四项均维持。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十六轮）：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第八轮复核，git log 实证 f08937e 后 web/ 零代码提交）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新视角五项（本轮）：首屏链路（SpaHandler /api 兜底 404 闭环 + 挂载后全路由四态齐备 + 根部白屏通用形态）；状态机完整图（九路径穷举走审：登录/登出/401/删除含管理员自身/代理切换/撞名学生/onBackToStudent/登出重登/无账号回登录，复位链路全闭环无死锁）；输入边界（8 输入 + 搜索面全盘点，无注入/XSS/密码持久化面，React 转义覆盖 title/toast/渲染）；响应式（双断点 + 移动底栏 + 横滚白名单 + 背景画布跨屏一致 + 倒计时窄屏可读）；健壮性最后一道（可选链/空值兜底全覆盖无崩溃面 + tsc 全量通过 + 401 三形态单广播）。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}|M-1|F\d{2}A?|B\d{2}-` 全形态 + `select|selectElectives|exit|electives|findElectivesData|targets` 业务关键词误命中排除——葫芦串确认隐性锚点：placeholder="选课大厅"/aria-label="搜索"等均为业务语义非轮次标签）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。
- 后端交叉契约复核（本轮重读关键点）：handleSetTargets 凭据表 accountExists 校验（handler.go:444-486）→ 无透传取核心账号兜底（:478-485）→ 条数上限 100（:499-502）→ 字段校验（:503-516）→ 双 SetTargets + AppendLog 失败记日志零吞错（:517-524）；handleState 透传账号存在性校验 + 无透传核心账号兜底（handler.go:529-548）；handleElectives 同款凭据表校验（:243-253）+ 有目标账号走 ElectivesSnapshotFor 专帧回退链 + 无目标账号 ProbeNow 全局帧（:254-276）；handleAdminStats window_opened/window_closed 与学生端 /state 同源 + 目标数失败 500 + 引擎兜底 ddddocr（handler.go:874-957）；windowClosedLocked 三条判据单源 + open 单快照复用（scheduler.go:918-939，判据 2 syncFailStreak≥3 带「开放时间已过」、判据 3 EmptyProbeRuns≥3 带 prevOpened 守卫）；StateForAccount 同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤（scheduler.go:699-725）；handleAdminCodes 三态（GET 列表 / POST 生成单事务 / DELETE 空 body 合法，router.go:127-139 与 handler.go 的 requireJSONBody 分层）+ Router 完整 method+pattern 表核对（/api/admin/config GET/PUT / stats GET / accounts GET/DELETE / logs GET / electives GET / select POST / exit POST / targets PUT / accounts GET / state GET / logs GET / logout POST，全部与前端 client.ts 调用点一一对应，零漂移）。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、useTickingCountdown target 变化校正 now（useTickingCountdown.ts:19-21）、Dashboard 日期分组本地零点（parseDateKey/localTodayMs）、Toast 同 title 去重合并、Admin 复制 clipboard 降级链、Tabs 受控化（Admin :161 / Select :929）、Progress max=1 空条兜底（Select :1102-1103）。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 555ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist（哈希与 R99 逐字节一致，web/ 零代码改动实证） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✖=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=9af1de0）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十六轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第八轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零代码提交）；残余面清单逐字无漂移。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:697-718），里程碑第三轮评估维持不落地（影响面=同信道故障次位表现，无真实反馈依据）。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——里程碑第四轮评估维持观察不落地（渲染期异常源几无 + 引入复杂度，无真实影响）。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲，本轮推演 handleBack 三轮收敛已覆盖。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三项 autoFocus 落位 + Esc 全链已闭环 + Tab 顺序自然，陷阱/归还仍缺口）。
- **无障碍残留面备注（延续）**：(d) 新增 index.html 根部无首屏骨架——SPA 通用形态 <1s 级白屏，不下沉为 OBSERVE 条，仅备注。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 四守护脚本 + git status/rev-parse/log/check-ignore）；工作区 `git status` 零改动（HEAD=9af1de0，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单、O-3 族焦点陷阱/焦点恢复、新视角「响应式横向溢出（overflow-x-hidden 兜底 + 横滚白名单）」「createRoot 非空断言在静态 HTML 下安全」「白屏窗口时长（加载产物 GZIP 125KB）」「react-query 数据快照展开（只隐藏子树、父容器保留）」为走读推断；M-1 逐字符、四组断言计数（18/18、6/6、5/5、audit 77 项）、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git rev-parse/log/check-ignore（含 `git log f08937e..HEAD -- web/` 空集实证）、后端契约 grep 定位（handleSetTargets/handleState/handleElectives/handleAdminStats/windowClosedLocked/StateForAccount/router 表）、setSelected/dirtyRef/echoedRef 清点、可选链/输入框/断点 grep 全量清单均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第三十六轮闭合；连续第四十六轮无严重级发现；R100 里程碑轮收官。