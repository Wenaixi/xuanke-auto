# Round 96 前端只读审查报告

基线：commit 2cb84af（R95 双 findings + 收尾总结，HEAD，进度 96/256）。本轮为 R96 前端只读审查 + M-1 延续管理（第三十二轮）。核心为 M-1 第三十二轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核（ring 语义 + 全站一致性 + 与既有样式参数差异）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：数据绑定与表单状态 / 路由级状态持久化 / 视觉一致性 token 使用 / 可访问性 aria 动态态 / 性能优化边界）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级）。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / handleState / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log、`npx tsc -b --pretty false`、`npx vite build`、三守护脚本 + audit.mjs 只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round96-frontend-findings.md（不存在则创建）。结束态 `git status --short --branch` = `## master`；build 产物落 backend/web/dist（git check-ignore 历轮已确认被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十二轮闭合：四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过：Button.tsx:42 ring 行在位、语义正确（键盘触发/鼠标不显示/全站 Button 收敛）、与 Admin 开关/Login 密码切换/Input 自带焦点样式共存无冲突、残余面与 R93/R94/R95 清单逐项一致。六防保存链逐条重读零回潮；setSelected 六代码调用点与 dirtyRef 置 true 仅三处（:455/:563/:742）复核零漂移。连续第四十二轮无严重级发现。

---

## 一、M-1 延续管理（第三十二轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R95 记录逐行一致）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。测试 18/18 中 echoed 第三参 2 条覆盖稳态放行与首帧未到仍推迟两极端。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 排除注释行后代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——全 src 目录 `shouldDeferSave(` 调用点 grep 仅命中此四处 + targetGuard.ts 自身定义；echoedRef 读点与 R95 完全一致（grep 全量命中含注释核对无漂移）。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项，含 echoed 第三参 2 条）。
- 第三十二轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

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
- **unmountedRef 全读写点**（grep 实测 8 处）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。
- **消费时刻读最新 ref**（grep 实测）：selectedRef（:173-174）/ revRef（:175-176）/ stateDataRef（:180-181）/ publishesRef（:663-664）四镜像的消费点全部位于防抖回调/flush/handleBack 消费时刻；pick 直接用渲染 selected（:349/:353/:363）因其「事件处理器内 selected 恒为最近已提交渲染值」论证（:346-347）成立。零旧闭包路径。

## 三、F93-01 Button focus-visible 持续复核

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R95 记录逐字符一致。R94 后无新改动修改该行（git log 确认 HEAD 自 R93 起仅 docs/review 与 docs(backend) 提交，web 代码零改动）。
- **语义正确性**：`focus-visible:` 变体由浏览器 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发（不显示）。global.css:174「基础交互重置」对 button 统一 `outline: none`，ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处四路由全覆盖经 base class 一次收敛；Admin 开关（Admin.tsx:583 `focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login.tsx:173 同款）、Input 自带 focus ring（Input.tsx:14）并存无冲突。Button ring 用 `--cyan`（= #ffffff 纯白）+ offset 2px，既有用 `ring-white/60`（半透明白）无 offset——二者均清晰可见，纯参数差异（已归 OBSERVE-93-01 备注，不单独立条）。
- **残余面确认**（与 R93/R94/R95 清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。OBSERVE-93-01 延续。
- F93-01 第四轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新视角扫查（本轮选定 5 项）

### 1. 数据绑定与表单状态（受控/非受控边界 + 提交前校验）

- **受控输入覆盖**（grep + Read 实测）：Login 账号/密码/激活码（:137/:157/:261）、Select 搜索（:871）、Admin 数量/次数/地址/密钥/模型/并发（:375/:387/:606/:618/:629/:680）——全部 `value` + `onChange` 受控；无混用 `defaultValue` 的受控 input。Select activeTab 受控兜底（:929）与非受控 Tabs 子件共存符合 Radix 语义。
- **提交前即时校验**：Login submit 前 `!account.trim() || !password`（:37）+ 激活 `!activationCode.trim()`（:69）；Admin count/uses/concurrency 三输入 onChange 内 clamp（Math.max/min/||1），提交前再兜 `Math.max(1, concurrency || 1)`（:536）——边界（NaN/负数/0）双层闭合。输入框 min/max 与 onChange clamp 一致，无"DOM 值越界进提交"路径。
- **非受控遗留核查**：Tabs 受控 value（Admin :161 / Select :929）+ Radix 内部 roving focus 不受影响（走读推断）；无第三方非受控组件。表单 submit 均 preventDefault（Login :125）；Admin 无 <form>、按钮型提交 onClick 直调，无隐式 submit 风险。
- **搜索防抖缺失评估**：Select 搜索输入无防抖，每击键全量 filter（40 门×4 发布规模）——DOM 差分毫秒级可忽略，YAGNI 成立不落地。
- 结论：无新增发现。

### 2. 路由级状态持久化（账号切换/刷新/登出后视图恢复正确性）

- **会话恢复**：xk_sessions/xk_admin_name/xk_admin_token 三键 try/catch 降级（App.tsx:18-68 全部成对）；刷新后 `isCurrentAdminSession`（adminAuth.ts:6-12）凭「管理员名账号会话 token === 管理标记」唯一判据恢复管理页——撞名学生普通会话永不误进（B43-04/F52-M1 契约延续）。
- **账号切换复位**：Select key={account}（App.tsx:293/:339）+ 组件内兜底守卫（:195-202）双保险；Admin 五 Tab queryKey 全含 account（:317/:503/:724/:818/:912）防跨账号缓存串线。
- **登出/吊销视图复位**：logout（:126-131）清 targetAccount + page 复位 dashboard；401 onUnauthorized（:204-206/:210-214）退出代理态/管理态；account-reselect effect（:135-148）空会话回登录页 + page 复位——三路全向「初始 dashboard」收敛，无残留 view 态（R95 复核点延续，本轮重读位置未变）。
- **刷新后选课大厅回显**：targetAccount/page 不持久化（纯内存），刷新落回 dashboard——`/?` 深链丢失不属承诺（SPA 无路由库，历轮口径维持）。
- 结论：无新增发现。

### 3. 视觉一致性（token 使用与纯黑白设计语言）

- **色彩 token 覆盖**（Read global.css + grep 实测）：主色板五 token（--bg/--surface/--surface-soft/--surface-muted）在 Card/Button/Input/Badge/Tabs 五基础件内统一引用；语义色（--emerald/--amber/--rose/--cyan 全映射纯白系或玫瑰红）在 Progress/Toast/Badge 语义位使用一致；`--fg-muted/--fg-dim` 灰阶在文本弱化位（text-neutral-500/600 vs token 混用）存在——中性灰 `text-neutral-400/500/600` 直写与 token 并存属既有设计语汇（历轮已核），非本轮回归。
- **纯黑白偏离检查**（audit.mjs A/B/C 三族 77 项全绿实测）：bg-black/25|30|70|75 与 bg-neutral-950 实色黑洞零命中、画布遮罩梯度正确、整页 bg-black 零命中。rose 红仅出现在「错误/删除/退选」语义位（--rose token 单一来源），无杂色渗入。
- **间距/字号体系**：`gap-*` 3-6 步进、圆角全走 --radius token、字号 10-15px 体系稳定；倒计时 `tabular-nums` + font-mono 等宽体系（Dashboard :366/:390、Select :850）统一。
- 结论：无新增发现（audit 77/77 实测背书）。

### 4. 可访问性的 aria 动态态（disabled/expanded/checked 初始值与更新）

- **动态 aria 全覆盖**（grep + Read 实测）：CollapseSection `aria-expanded`（Dashboard :69）绑 open state、`aria-controls` 用 useId（:64/:82）避免多实例重复 id（先前的硬编码 id 悬空已修）；Admin 开关 `role="switch"` + `aria-checked={activationOn}`（:579-580）随点击翻转；Login 密码切换 `aria-pressed`（:171-172）随 showPassword 翻转；Modal 三处 `aria-modal/labelledby`（Select :1204-1206 / Login :227-228 / Admin :214-215）+ Esc 全链（各自 onKeyDown 守卫防误关）。
- **disabled 态语义**：全部 Button/Input/switch disabled 绑定 React state（loading/activating/deleting/actionLoading.has(id)），非静态硬编码；禁点态 `disabled:opacity-40 + pointer-events-none`（global.css:182-186 + Button base）双通道。
- **Progress**（:28）`aria-valuenow` 与 `aria-valuemax` 动态传值（unannounced 时 value=0/max=1 空条如实反映）；aria-valuetext 缺省（max 或 value 超界时读屏播报原始数字——非阻断，归无障碍残留面备注）。
- **aria-live 缺失**（延续 R95 无障碍残留面备注）：Toast/倒计时仍无显式 aria-live（Radix Toast Viewport 内建 aria-live=polite 可能已覆盖，待浏览器实测——走读推断，不立条）。
- 结论：动态态初始值（expanded=open 初值 false、checked=activationOn 初值 true）与 state 同步；无「静态写死」或「初始错位」点。无新增发现提级。

### 5. 性能优化边界（memo/useCallback/列表 key/DOM 复用）

- **memo 使用**（grep 实测）：仅 Toast 两 useCallback 稳定引用（:33/:58）；组件无 React.memo——路由/卡片当前规模（≤40 门×4 发布、≤100 目标）下 YAGNI 成立（R95 口径延续）。
- **useMemo 派生**：三处（App accounts :87 / Dashboard extrasMs :209 + dateGroups :220 / Select tabs :331）依赖稳定引用（sessions/electives?.begin_times/courses/publishes），无渲染期新建数组陷阱。Dashboard dateGroups 依赖 [courses, electives?.publishes] 每 /state 刷新重算一次（2-3s 级），规模小可忽略。
- **列表 key**（grep 实测）：全部唯一稳定——Dashboard dateGroups 用 g.key、pubs 用 publish_id、课程卡用 class_id、日志用 l.id、激活码用 code、Admin 账号行用 a.account、Select 课程卡用 c.id、Tabs 用 t.publish_id；Toast 自增 id（:54）。零 index key、零重复 key。
- **DOM 复用正确性**：卡片列表无条件分支重组（isSelected 只改 className 不变 key）；折叠切换只条件渲染 children 不重排列表。零风险。
- **re-render 边界**：useTickingCountdown 每秒 setNow 触发宿主路由整树重渲染（注释已如实声明 :4-7/Select :205-207/Dashboard :177-181），DOM 差分成本可忽略——「只重渲染倒计时一处」标为潜在优化非当前承诺，YAGNI 成立。
- 结论：无新增发现。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理）

**OBSERVE-93-01（延续，第四轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核）+ 语义正确（键盘触发/鼠标不显示/全站收敛）；残余面（Dashboard CollapseSection :67、Admin 复制/刷新/删除/收起/引擎切换/重试 :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）与 R93-R95 清单逐项一致无漂移；ring 参数未统一（纯白+offset vs 半透明白）值均可见。历轮已述修复可为「裸按钮另择时统一收敛」，维持「续」。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:696-719）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化（:697 `{logs && logs.length > 0 ? ... : :714-717 <NO RECENT LOGS>}`，无 isLoading/isError 解构分支；对照 Admin LogsTab :928-950 完整四态）。零功能危害维持留档，「一行条件对齐」候选维持不落地（需解构两变量 + 两分支 JSX，纯展示误导）。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演 handleBack 三轮 flush 收敛（:611-642：等待窗口刚有改动的 setRev → effect 挂 timer 异步 → 等一帧复查 revRef :622-624「与本轮 flush 消费的一致才 break」已覆盖该竞态）无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（/票据无效分支 :88 / 通用失败分支 :95 / 取消 Esc :234-236 + 按钮 :302-305）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害：等发布恢复/用户改动重跑）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）——焦点进入侧闭环，陷阱 + 归还侧仍缺口。

**无障碍残留面备注（更新，非新立条）**：(a) Toast 无显式 aria-live 容器（走读推断：Radix ToastPrimitive Viewport 可能内建 aria-live=polite，本轮重读 Toast.tsx:105 Viewport 组件存在、语义由 Radix 内部实现，待浏览器实测确认，故不立条）；(b) prefers-reduced-motion 无分支（动效敏感用户无降级，非阻断）；(c) Progress 缺 aria-valuetext（超界值时读屏播报原始数字，非阻断）。三项与 OBSERVE-93-01 无障碍横查族同频道，下轮评估时如被浏览器实测证实 (a) 有缺口再考虑是否提级。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十二轮）：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 八读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第四轮复核）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新视角五项（本轮）：受控输入全覆盖 + 提交前双层 clamp 无越界路径；会话/账号切换/登出三路视图复位全向收敛；token 色彩体系统一（audit 77/77 背书）；aria 动态态（expanded/checked/pressed/valuenow）全随 state 同步无静态写死；列表 key 全唯一稳定零 index key、memo 使用 YAGNI 成立。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b` 全形态）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中；navigator.clipboard 与 document.execCommand 仅 Admin 复制降级链、非导航能力）——全仓零命中。
- 后端交叉契约复核（沿用 R95 定位，本轮重读关键点）：handleSetTargets 凭据表 accountExists 校验（handler.go:461-476）→ 条数上限 100（:499）→ 四字段校验（:503-516）→ 双 SetTargets → AppendLog 失败记日志零吞错（:522-524）；handleState 透传账号存在性校验（:535-545）；windowClosedLocked 三条判据单源 + open 单快照复用（scheduler.go:913-939）；StateForAccount 同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤（scheduler.go:699-725）。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、open_time 空格串解析、Progress aria-valuenow 与 unannounced 语义一致、Dashboard 日期分组本地零点无时区偏移、Toast 同 title 去重合并防轰炸、Admin 复制 clipboard 降级链、Tabs 受控化（Admin :161 / Select :929）。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npx vite build`（web/ 下） | ✅ built in 1.35s，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✗=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=2cb84af）；除本报告外零改动 |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十二轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第四轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单逐字无漂移；无障碍残留面备注更新（Progress aria-valuetext 并入本线）。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:696-719），维持不落地。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲，本轮推演 handleBack 三轮收敛已覆盖。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三项 autoFocus 落位 + Esc 全链已闭环，陷阱/归还仍缺口）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npx vite build` + 三守护脚本 + audit.mjs + node 纯只读实测）；工作区 `git status` 零改动（HEAD=2cb84af，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单、O-3 族焦点陷阱/焦点恢复、OBSERVE-90-01 三态差异、无障碍残留面 (a) Toast aria-live 与 (c) Progress aria-valuetext 为走读推断（(a) 已注明待浏览器实测，因 Radix Viewport 内建语义可能已覆盖故不立条）；M-1 逐字符、三组断言计数（18/18、6/6、5/5）、audit.mjs 77 项、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git rev-parse、后端契约 grep 定位、新视角清点（受控输入计数 / aria 动态态 / 列表 key / memo 计数）均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥——无障碍残留面备注增 (c) 项并入 OBSERVE-93-01 线，不重复立条）；M-1 第三十二轮闭合；连续第四十二轮无严重级发现。
