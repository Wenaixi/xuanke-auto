# Round 101 前端只读审查报告

基线：commit 16d1aec（R100 双 findings + 收尾总结，HEAD，进度 101/256；连续七轮双端零代码修改的纯观察轮）。本轮为 R101 前端只读审查 + M-1 延续管理（第三十七轮）。核心为 M-1 第三十七轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核（第九轮）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76 族 / O-3 族延续管理；新契约角度走读（本轮选定两项：a) aria 动态态完整覆盖 + b) 性能边界复核，附 c) 视觉 token/tabular-nums 全量清点）；契约 20 扫描（轮次标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级 / 导航能力全形态）。审查范围：web/src 全部 .ts/.tsx（8 组件原语 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs），交叉核对 App.tsx 状态机与 client.ts 401 契约。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log、`npx tsc -b --pretty false`、`npm run build`、四守护脚本只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round101-frontend-findings.md。结束态 `git status --short --branch` = `## master`（无 untracked）；build 产物落 backend/web/dist（git check-ignore 确认被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十七轮闭合：三消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符复核通过、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过（第九轮）：Button.tsx:42 ring 行逐字符在位、git log 实测自 f08937e 后 web/ 目录零代码提交（`git log f08937e..HEAD -- web/` 空集）。新契约角度（本轮两项 + 一项清点）：无障碍 aria 动态态完整覆盖复核发现 **Progress 的 aria-valuenow 在「名额未公布」（max_count=0）场景按 Select.tsx:1100 注释语义传 0**，但因 Progress.tsx:28 对 undefined 的默认值为 0、且 unannounced 分支显式传 0，其与后端「0=名额未公布」契约的分界在无障碍读屏侧被抹平——语义正确（未公布无进度），读屏与视觉同步，**无缺陷，仅作为「注释承诺已由显式传参落实」的核验结论**（不立条）。性能边界复核：每秒 setNow 的宿主路由整树重渲染面与历轮记录一致（Dashboard 整页 + Select 整页），useMemo 清点（App :87 / Dashboard :209/:220 / Select :331 / Toast useCallback 两处）无一处是「无必要的过早优化」，重渲染面平衡（无新发现）；列表 key 稳定性全量清点（课程卡 c.id / 日期组 key / 日志 l.id / TabsTrigger publish_id / TabsContent publish_id / 生成码字符串）无不稳 key。tablular-nums 全量清点：倒计时四格 + 容量统计 + 指标两行 + 管理表格数值 6 处均覆盖，唯一可议项 Dashboard 运行指标「心跳轮询周期 30 秒」与「预选目标课程 N 门」已带 tabular-nums（:479/:485），其余 font-mono 静态文案（font-variant 不受影响）无需覆盖——**零缺漏**。契约 20 扫描零命中。连续第四十七轮无严重级发现。

---

## 一、M-1 延续管理（第三十七轮）

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R100 记录逐行一致，零漂移）：
  - 防抖回调 :699 `if (shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current))`
  - flushTargets :509 `if (shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current))`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
  - grep 全量命中与 R100 完全一致：`shouldDeferSave(` 全 src 仅 targetGuard.ts:64 定义 + 上四处消费 + scripts 测试引用，无第五消费处。
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；第三参 echoed 只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**（grep 实测与 R100 记录逐字符一致）：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——echoedRef 全量 grep 命中 24 行（含注释）与 R100 一致，代码级 9 处零漂移。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项 = shouldDeferSave 8 + selectedHasStalePublish 5 + cleanStaleSelected 5，含 echoed 第三参 2 条）。
- 第三十七轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 三消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :343/:347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **消费时刻读最新 ref**（grep 实测）：selectedRef（:173-174）/ revRef（:175-176）/ stateDataRef（:180-181）/ publishesRef（:663-664）四镜像的消费点全部位于防抖回调/flush/handleBack 消费时刻；pick 直接用渲染 selected（:349/:353/:363）因其「事件处理器内 selected 恒为最近已提交渲染值」论证（:346-347）成立。零旧闭包路径。

## 三、F93-01 Button focus-visible 持续复核（第九轮）

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R100 记录逐字符一致。git log 实测 f08937e（F93 提交）→ HEAD 无任何 `web/` 代码提交（`git log --oneline -- web/` 最新代码提交即 f08937e，之后仅 docs 提交；`git log f08937e..HEAD -- web/` 实测空集），该行未被改动。
- **语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发。global.css:169「基础交互重置」对 button 统一 `outline: none`（本轮重读逐字在位 :169-176），ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处经 base class 一次收敛；Admin 开关（Admin.tsx:583 `focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login.tsx:173 同款）、Input 自带 focus ring（Input.tsx:14 `focus:ring-1 focus:ring-[var(--cyan)]`）并存无冲突。Button ring 用 `--cyan`（= #ffffff 纯白）+ offset 2px，既有用 `ring-white/60`（半透明白）无 offset——均清晰可见，纯参数差异（OBSERVE-93-01 备注，不单独立条）。
- **残余面确认**（与 R93-R100 清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。本轮逐一 grep 复核这些行号对应裸 button 仍无 ring（与历轮清单一致）。OBSERVE-93-01 延续。
- F93-01 第九轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新契约角度（本轮选定 a + b + 附 c 清点）

### a) 无障碍 aria 动态态完整覆盖复核（expanded/checked/pressed/valuenow 随 state 同步、焦点管理进/出模态）

**动态态清点（grep + 逐字符）**：
- **aria-expanded**：Dashboard CollapseSection 折叠头（:69 `aria-expanded={open}`）——open 状态（useState）驱动，随折叠/展开即时同步，语义正确（R100 复核延续）。
- **aria-checked**：Admin 激活码机制开关（:580 `aria-checked={activationOn}` + role=switch :579）——随 setActivationOn(!activationOn) 即时同步，语义正确。
- **aria-pressed**：Login 密码可见性切换（:172 `aria-pressed={showPassword}`）——随 setShowPassword(!showPassword) 即时同步，语义正确。
- **aria-valuenow**：Progress 组件（Progress.tsx:26-28）——`aria-valuemin={0}` / `aria-valuemax={max}` / `aria-valuenow={value}` 三属性随 value/max 渲染期即时同步，无独立 state 滞后。**max_count=0「名额未公布」场景**：Select.tsx:1102 `value={unannounced ? 0 : c.selected_count}` + :1103 `max={unannounced ? 1 : c.max_count}` 显式传参，注释（:1097-1100）「aria-valuenow 亦如实反映未公布无进度语义」已由显式传参落实——读屏读到 0/1 即「无进度」，与视觉空条同步，无分叉。**核验结论：该注释承诺确已落地，无缺陷**（历轮残留面备注 (c)「Progress 缺 aria-valuetext」维持——属于「读屏得到 0/1 但无「名额未公布」文字」的可选增强，非状态不同步）。
- **aria-modal / aria-labelledby**：三处手写模态（Select 退选 :1205-1206 / Admin 删账号 :214-215 / Login 激活 :227-228）均显式声明，无动态态滞后。

**焦点管理进/出模态**：
- 三模态初始焦点落位：autoFocus 全覆盖——Select 退选取消（:1234）/ Admin 删账号取消（:241）/ Login 激活码输入（:267），与历轮一致。
- **焦点归还缺口的完整论证（本轮走读复核）**：三处手写模态均为「条件渲染 + role=dialog」，卸载即从 DOM 移除——浏览器对「聚焦元素随焦点所在子树被移除」的标准行为是焦点回落 body（无归还到触发按钮）。Tab 焦点环（Button ring）在位，键盘用户关闭模态后从 body 重新 Tab 起点导航——功能可达、体验是「焦点落在页面顶部而非回到触发处」。此缺口已归属 O-3 族（指向 F6-02 Radix Dialog 迁移单一出口），历轮维持观察。**本轮补充观察：三处模态的触发按钮均位于页面中部（Select 卡片操作区 / Admin 操作列 / Login 表单内），焦点回落 body 后用户需 Tab 回触发位置——在长课程列表场景（Select）该成本可达数十次 Tab。此为新证据支撑 O-3 族「焦点归还侧缺口」的实际影响面，但仍属体验级非功能级，维持观察不升级**。
- **Esc 关闭**：三模态全部带 onKeyDown Escape 处理（Select :1207-1211 / Admin :216-218 / Login :232-238），且激活/删除/退选在飞时禁用（activating/deleting/actionLoading 守卫），与历轮一致。
- **键盘可达性边界（Tab 顺序）**：手写模态背景表单未做焦点陷阱——Login 激活模态打开时背景表单仍可 Tab 穿出（历轮已注明「完整焦点陷阱迁移到 Radix Dialog 属后续候选，这里先补最小语义门」Login.tsx:220-222 注释）。三模态内按钮 Tab 顺序自然（取消→确认）。维持历轮观察。

**结论**：aria 动态态全量（expanded/checked/pressed/valuenow）与 state 同步无滞后；Progress 未公布场景注释承诺已落实；焦点管理「进」侧闭环、「出」侧（归还）缺口维持 O-3 族观察（本轮补充影响面论证）。无新增发现。

### b) 性能边界复核（重渲染来源 / 列表 key 稳定性 / useMemo 平衡）

**每秒 setNow 的宿主路由重渲染面**（grep + 走读）：
- useTickingCountdown（lib/useTickingCountdown.ts:13 `setInterval(() => setNow(Date.now()), 1000)`）被 Dashboard（:191）与 Select（:767）各自顶层消费——每秒 setNow 归属宿主路由组件，触发该路由整树重渲染。历轮已确认 React 语义不可绕过、DOM 差分成本可忽略；R86 注释已如实口径（「只重渲染倒计时一处需拆 memo 叶子组件，潜在优化非当前承诺」useTickingCountdown.ts:4-7）。本轮复核：
  - **Dashboard 重渲染面**：整页（四卡 + 日志区 + 折叠段）。relativeCountdown 折叠行（Dashboard.tsx:90-99 注释）利用该整页重渲染「渲染期直接算 Date.now() 即新鲜」——无额外定时器，设计自洽。
  - **Select 重渲染面**：整页（顶栏 + 倒计时 + 搜索栏 + 全部课程卡）。课程卡 map 内无 React.memo 包裹——每秒全量重渲染卡片网格（数百卡片），DOM 差分以 textContent 比对为主，成本可忽略；且 Select 在黄金期（窗口开放后 2s 轮询）本就在高频刷新数据，每秒重渲染相对轮询重渲染属同级成本。
  - **结论**：重渲染面是「每秒整树」而非「局部」，历轮评估「DOM 差分成本可忽略 + 拆 memo 是潜在优化非承诺」维持；无性能回归证据（黄金期抢课核心路径的提交逻辑不依赖重渲染性能，提交由 ref 镜像消费，重渲染不干扰）。无新增发现。
- **列表 key 稳定性全量清点**（grep + 走读）：
  - Select 课程卡 `key={c.id}`（:1020）——课程 id 服务端下发稳定，排序/搜索变化不影响 key 身份，无重挂载；
  - TabsTrigger `key={t.publish_id}`（:936）与 TabsContent `key={t.publish_id}`（:984）——publish_id 稳定；发布重建（开窗瞬间平台清空又恢复、publish_id 全变）时 key 全集变化导致整组重挂载，属数据源重建的预期行为（与 F40 清理链协同，无状态残留——受控 activeTab 已兜底回落 :929）；
  - Dashboard 日志 `key={l.id}`（:700）——后端自增 id 稳定；
  - Dashboard dateGroups `key={key}`（日期分组键）——日期字符串稳定；
  - Admin 生成码列表 `key={c}`（:423）——激活码字符串全局唯一；
  - 其余 map（账号行/配置/状态）同以唯一服务端字段为 key。
  - **结论**：全站列表 key 无一使用 index 或易变值，零不稳 key。
- **useMemo/useCallback 清点（是否平衡）**：App :87（accounts 稳定引用，防 effect 重跑）/ Dashboard :209 extrasMs / :220 dateGroups / Select :331 tabs / Toast :33/:58 useCallback（toast/removeToast 稳定引用，防 Provider 重渲染扰动）。五个 useMemo/useCallback 均有明确「稳定引用」目的（依赖防抖或 effect 重跑），无一为「过早优化」；其余渲染期派生（selectedCount/过滤/排序）因消费方为渲染本身、无 useMemo 收益，未加是正确的简洁。**平衡确认：无该用未用、亦无不该用却用**。

### c) 视觉 token 一致性 + tabular-nums 等宽数字覆盖全量清点（附）

- **视觉 token**（grep 实测）：颜色全部经 CSS 变量（--fg/--fg-muted/--fg-dim/--border 族/--emerald 等灰阶映射，global.css:4-42）+ Tailwind 中性色阶（neutral-xxx/black/white），无游离十六进制（唯一显式 `#09090b` 为三处模态内层卡片底色，与 `--surface` #0c0c0e 同族近似，属刻意层次区分，历轮已核）；圆角全走 --radius-* 变量（sm/md/lg/xl/full 五档），无游离圆角值；按钮/输入/进度条三原语均消费同一 token 集。audit.mjs 77 项（A 画布族 + B 工具类 + C 实心黑洞清零 15 文件 × 5 项）全绿。
- **tabular-nums 全量清点**（grep 实测命中 9 处）：
  - 倒计时：Dashboard 主矩阵四格（:366/:374/:382/:390）带 `tabular-nums`；Select 横幅倒计时（:849）带 `tabular-nums`。
  - 容量统计：Select 已报容量（:1091）带 `tabular-nums`。
  - 指标行：Dashboard 心跳 30 秒（:479）/ 预选 N 门（:485）带 `tabular-nums`。
  - 管理表格：Admin 激活码余量（:768）带 `tabular-nums`；账号表格两列（:850/:864）带 `tabular-nums`。
  - 全站 font-mono 命中 53 处（Dashboard 24 + Select 10 + Admin 19）——纯静态文案（ID 前缀、标签、徽章、单位、时间戳）等宽字体本身即为「每字符同宽」字形，font-variant-numeric: tabular-nums 主要影响比例数字的纵向对齐；**静态文案无「每秒刷新的动态数字」对齐需求，不带 tabular-nums 不构成缺漏**；真正动态数字（倒计时/容量/指标）全部已带。**结论：等宽数字覆盖零缺漏**。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理 + 焦点归还影响面补充论证）

**OBSERVE-93-01（延续，第九轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核）+ 语义正确（键盘触发/鼠标不显示/全站收敛）；残余面（Dashboard CollapseSection :67、Admin 复制/刷新/删除/收起/引擎切换/重试 :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）本轮逐一 grep 复核与历轮清单逐项一致无漂移。保持「续」。

**OBSERVE-90-01（延续，里程碑第四轮评估）**：Dashboard.tsx 日志区（:697-718）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化（:697 `{logs && logs.length > 0 ? ... : :714-717 <NO RECENT LOGS>}`，无 isLoading/isError 解构分支；对照 Admin LogsTab :928-950 完整四态）。**里程碑视角评估（第四轮）**：影响面 = /logs 失败时仅此一区文案误导（其余同信道查询各有错误条/加载态）；真实场景中 /logs 与 /state 同 Backend 同信道，/state 失败时 Dashboard 顶部错误条已提示「凭据失效」或 react-query 重试静默，日志区 NO RECENT LOGS 属**同故障的次位表现**；修复成本 ≈ 8 行（解构 isLoading/isError + 两分支），收益 = 故障时原语文案准确。连续四轮无新依据（无用户报告该误导、无真实故障样本），且「NO RECENT LOGS」在真无日志时语义正确——**维持观察不落地**。

**OBSERVE-88-01（延续，里程碑第五轮评估）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分。**里程碑视角评估（第五轮）**：历轮「观察不落地」依据 = 渲染期异常源几无（所有 data 均经 api() 校验 + 类型契约）、且引入 ErrorBoundary 需新增组件 + 包裹层（破坏现有零 wrapper 的纯粹性）——真实影响 ≈ 0 且修复引入复杂度，**维持观察不落地**。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演 handleBack 三轮 flush 收敛（:611-642）与等待窗口内 pick 新改动（等待期间 setRev → effect 挂 timer 异步 → 等一帧复查 revRef :622-624 已覆盖）无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（票据无效分支 :87-90 / 通用失败分支 :91-97 / 取消 Esc :232-238 + 按钮 :299-311）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害：等发布恢复/用户改动重跑）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖 :241/:267/:1234）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）+ Tab 顺序自然——**新增补充论证：三处模态触发按钮均位于页面中部，卸载后焦点回落 body、键盘用户需重新 Tab 数十次回到触发位置（Select 长课程列表场景），「焦点归还」缺口的影响面由此得到更具体的量级支撑，但仍属体验级，维持观察不升级**。

**无障碍残留面备注（延续，非新立条）**：(a) Toast 无显式 aria-live 容器（走读推断：Radix ToastPrimitive Viewport 内建 aria-live=polite，待浏览器实测）——本轮重读 Toast.tsx:105 Viewport 组件存在、语义由 Radix 内部实现，与前轮一致；(b) prefers-reduced-motion 无分支；(c) Progress 缺 aria-valuetext（本轮核验「未公布场景 valuenow=0 注释承诺已落实」，aria-valuetext 为「读屏得 0/1 无文字」的可选增强，维持备注）；(d) index.html 根部无首屏骨架（SPA 通用形态，<1s 级白屏）。四项均维持。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十七轮）：三消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第九轮复核，git log 实证 f08937e 后 web/ 零代码提交）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新契约角度 a（aria 动态态）：expanded（Dashboard :69）/ checked（Admin :580）/ pressed（Login :172）/ valuenow（Progress :26-28）四类全部随 state 即时同步，零滞后；Progress 未公布场景（Select :1102-1103）显式传 0/1，注释承诺已由实现落实；三模态 aria-modal/labelledby 齐备。
- 新契约角度 b（性能边界）：每秒 setNow 重渲染面（Dashboard/Select 各整树）与历轮记录一致、DOM 差分成本可忽略、relativeCountdown 利用整页重渲染零额外定时器设计自洽；列表 key 全量（课程卡 c.id / 日志 l.id / 日期组 key / 生成码字符串 / TabsTrigger+TabsContent publish_id）零不稳；useMemo/useCallback 五处全有稳定引用目的、无过早优化。
- 新契约角度 c（视觉 token + tabular-nums）：颜色/圆角全走 CSS 变量与中性色阶，无游离值；tabular-nums 9 处命中、动态数字（倒计时/容量/指标/管理表格）全覆盖、font-mono 静态文案不构成缺漏——零缺漏。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}|M-1|F\d{2}A?|B\d{2}-` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、useTickingCountdown target 变化校正 now（:19-21）、Dashboard 日期分组本地零点（parseDateKey/localTodayMs）、Toast 同 title 去重合并、Admin 复制 clipboard 降级链（:67-111，readOnly 加固在位）、Tabs 受控化（Admin :56 / Select :929）、Progress max=1 空条兜底（Select :1101-1105）、App 状态机九路径（登录/登出/401/删除含管理员自身/代理切换/撞名学生/onBackToStudent/登出重登/无账号回登录）复位链路全闭环。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 616ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist（哈希与 R100 逐字节一致，web/ 零代码改动实证） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✖=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=16d1aec）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十七轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第九轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零代码提交）；残余面清单逐字无漂移。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:697-718），里程碑第四轮评估维持不落地（影响面=同信道故障次位表现，无真实反馈依据）。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——里程碑第五轮评估维持观察不落地（渲染期异常源几无 + 引入复杂度，无真实影响）。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲，本轮推演 handleBack 三轮收敛已覆盖。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三项 autoFocus 落位 + Esc 全链已闭环 + Tab 顺序自然 + 补充「焦点归还影响面量级论证」，陷阱/归还仍缺口）。
- **无障碍残留面备注（延续）**：(c) Progress 缺 aria-valuetext 本轮核验「未公布场景 valuenow=0 注释承诺已落实」，aria-valuetext 为可选增强仅备注；(d) index.html 根部无首屏骨架——SPA 通用形态 <1s 级白屏，仅备注。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 四守护脚本 + git status/rev-parse/log）；工作区 `git status` 零改动（HEAD=16d1aec，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单、O-3 族焦点归还影响面量级论证（模态卸载焦点回落 body + Tab 重导航成本）、新契约角度 a 的「模态卸载浏览器焦点回落 body」行为、b 的「DOM 差分成本可忽略」为走读推断；M-1 逐字符、三组断言计数（18/18、6/6、5/5）、audit 77 项、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git rev-parse/log（含 `git log f08937e..HEAD -- web/` 空集实证）、aria 动态态四类（expanded/checked/pressed/valuenow）grep 逐字符、tabular-nums 9 处 grep 全量、useMemo/useCallback 5 处 grep 全量、列表 key 全量 grep 均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第三十七轮闭合；连续第四十七轮无严重级发现；R101 双 opus 审查轮前导（前端侧保持纯观察零落地）。
