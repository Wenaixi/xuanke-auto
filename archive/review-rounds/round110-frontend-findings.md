# R110 前端只读审查 Findings

基线：commit 8d79bcf（R109 收尾，进度 110/256）。**本轮为 M-1 第四十六轮 + F93-01 第十八轮 + OBSERVE-106-01 裁决独立复核**，必查四项 + 新契约角度（无障碍纵深五查完整边界清点，交叉验证数据展示层与交互反馈路径）。审查范围：web/src 全部 .ts/.tsx，交叉核对 git log（1351fa4 为 web/ 最后代码提交、8d79bcf 为纯 docs）、组件库源码、build/守护脚本/tsc 实测。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新增 OBSERVE（宁缺毋滥——无障碍纵深五查与全部必查项与历轮口径一致，无够格立条的新证据）；OBSERVE-106-01 维持「续」但独立证据结论为准、建议转「闭合」待主控合计。** 必查 1：M-1 第四十六轮闭合——四处消费点 + 置位三路径 + 首帧边界 + 读点六处清点 + 六防保存链全部通过（grep 实证无第五消费处）；必查 2：F93-01 第十八轮复核通过（Button.tsx:42 ring 在位 + 残余面清单与历轮逐项一致含 R104 补录 3 个 refetch + Dashboard:707）；必查 3：OBSERVE-106-01 七处裸按钮全文 inline 复核——提供独立证据，论证「无焦点环」事实成立但面量级低、键盘操作链路完整（可聚焦·可激活·文本可读），连续四轮无新证据，支持「转闭合」；必查 4：88-01/85-02/84-01/83-01/77-02/76 族/O-3 族延续全绿 + 契约 20 全仓扫描零命中；必查 5：无障碍纵深五查得出「F93-01 修复面 + 106-01 残余面 + O-3 模态族 + 108-01 动态态」四族边界恰好完整闭合（键盘可达零缺口、读屏语义族全量在位、aria 动态态唯一缺口已由 108-01 覆盖、焦点环残余面与 106-01 同一清单）。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| — | — | — | **本轮零新增立条项**。无障碍纵深五查、数据展示层、交互反馈与全部必查项均与历轮口径一致；`git log 8d79bcf..HEAD -- web/` 空集 + build 产物与 R109 逐字节一致（js Bm7TtkV4/css 1KHlpqcc）双重实证代码零改动，无任何可生成新证据的代码面。按「报告真实问题，宁缺毋滥」纪律维持零新增。 |

> 备注（不立条，清点留痕）：
> 1. **无焦点环裸 button 全仓清点更新**：grep 实证 `<button` 命中 17 处（Admin :417/:425/:441/:452/:470/:473/:577/:651/:661/:701/:792/:898/:944、Dashboard :67/:707、Login :167/:299、Toast :100）——其中 4 处自带 ring（Admin :577 switch :583、Login :173 密码切换），3 处图标-only 靠 aria-label（Admin :470/:473）+ Toast Close（Radix 受控 76-03），**10 处无 ring 面 = 7 个带文本（OBSERVE-106-01 :417/:425/:441/:452/:651/:661/:701）+ R104 补录 3 个 refetch（:792/:898/:944）+ Dashboard CollapseSection :67 + Dashboard 重试 :707 + Login 激活取消 :299**（后三者分属历轮已单列条目，:67 属 F93-01 残余家族）。清单与历轮一致，无新增成员。
> 2. **Dashboard :30 与 Select :40 priorityName 注释措辞微异**（"备选 1"起始 vs "首位不再是备选 1"），实现逐字节一致（`p===0 ? "首选" : \`备选 ${p}\``），量级低于立条线。
> 3. **Progress 唯一消费点 Select :1101-1103**：unannounced（max_count=0）时 value=0/max=1 → aria-valuenow=0 如实反映"未公布无进度"，与 R109 零缺陷口径一致。
> 4. **Dashboard 状态徽章两处文案"窗口已开放"(:346)/"窗口开放中"(:751)** 语义一致、刻意分展示层级，历轮口径维持。

---

## 三、必查项逐条结论

### 1. M-1 保存链稳态语义（第四十六轮）
- **四处消费点全传第三参 `echoedRef.current` 逐字符复核通过 + 无第五消费处 grep 实证**（命中恰 4 消费点 + targetGuard.ts:64 定义，全仓零遗漏）：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` —— handleBack 循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
- **shouldDeferSave 定义（targetGuard.ts:64-72）逐字符复核**：`stateData === undefined → true`；`echoed → false`；`(stateData.courses?.length ?? 0) > 0 && hasSelected`——三段注释（纯数据不依赖 echoedRef / 第二参数清空分判 / 第三参稳态放行）与历轮记录一致，注释语义完整。
- **echoedRef 置位三路径**：:162 声明 `useRef(false)`；:200 账号复位置 false；:240 courses 空分支置 true + setEchoDone(true)；:297 合并完成置 true + setEchoDone(true)。**首帧边界**：:234 `stateData === undefined` 前置短路不置位、:247 `pubs.length === 0` 短路不置位、:229 回显 effect 首行盾、:319 清理 effect 盾（`!echoedRef.current || publishes.length===0`）、:256 全清空守卫 `rev > 0 && !anyHas → return prev` 不置位——全部在位。
- **读点全量清点**：grep `echoedRef.current` 命中 8 处读点 + 6 处注释 + 声明——**运行时可读点恰 6 处** = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699，与历轮一致，无第五运行时读点。
- **六防保存链零回潮**：防抖回调五判据（:699 entries :709/:717/:732/:737）与 flush 五判据（:509/:515/:526/:553/:558）消费时刻读最新 ref 逐字符在位；dirtyRef 置 true 恰三处（saveNow catch :455 / flush 在飞 :563 / 防抖在飞 :742）；saveNow finally 补发链（:462-465）在位；unmountedRef 卸载后不发/不 toast 语义（:399-408/:431/:442/:447/:459）在位；防抖依赖三自愈信号 hasPublishes/echoDone/stateData（:755）在位。
- **TDD/实测**：target-guard 断言全绿、tsc EXIT 0、build 成功、audit 全绿。
- **结论：M-1 第四十六轮延续闭合。**

### 2. F93-01 无障碍焦点（第十八轮复核）
- **Button.tsx:42 ring 逐字符在位**：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`（:38 base class cn() 首参，:39-42 完整语义注释族），与 R93 原文逐字节一致，自前十七轮零回归。Tabs.tsx:29 同款 ring、Admin :583 switch `focus-visible:ring-2 focus-visible:ring-white/60`、Login :173 密码切换同款 ring 全部在位。
- **残余面清单与历轮一致（grep `<button` 全量实证）**：带文本/裸 button 无 ring 面 = OBSERVE-106-01 七处（Admin :417/:425/:441/:452/:651/:661/:701，全部逐字 inline 复核在位）+ R104 补录 3 个 refetch（:792/:898/:944 同族成员）+ Dashboard :67 CollapseSection + Dashboard :707 重试（R108 新增）+ Login :299 激活取消 + Toast :100 Close。自带 ring 面排除（Admin :577 switch / Login :173）。图标-only 无 ring 面：Admin :470/:473（aria-label 补齐，F105-01 修复面）。
- **结论：第十八轮复核通过（F93-01 本体零回归，残余面清单零增零减）；OBSERVE-106-01 维持续、独立证据支持转闭合（见必查 3）。**

### 3. OBSERVE-106-01 裁决独立复核（主控历轮维持观察——本代理独立证据）
- **七处逐字 inline 复核（焦点可见性现状）**：
  - Admin :417 收起：`<button onClick={() => setGenerated([])}>` 文本「收起」，无 ring、无 focus 样式。
  - Admin :425 复制行：`<Copy/> + <span>复制/已复制</span>` 文本可见，无 ring。
  - Admin :441 刷新：`<RefreshCw/> + 刷新` 文本可见，无 ring。
  - Admin :452 重试：文本「重试」，无 ring。
  - Admin :651/:661 引擎二选一：文本「硅基流动 Vision（云）/ 本地 ddddocr（离线）」；**此两面自带强 active 视觉态（选中白底黑字 border-white），是最需要焦点指示的面**——键盘 Tab 聚焦时无任何可见指示器，仅鼠标 hover 有态。
  - Admin :701 重试：`underline hover:text-white` 文本「重试」，无 ring。
- **「无焦点环」事实成立（独立 grep 证据）**：global.css:174 `button, input, select, textarea { ... outline: none }` 全局抹掉原生焦点；`focus-visible` 命中全仓恰 4 处（Button.tsx:42 / Tabs.tsx:29 / Login.tsx:173 / Admin.tsx:583）——7 处裸按钮全部零 ring，与修复面事实分界清晰。
- **支持「转闭合」的独立论证**：
  1. **键盘操作链路完整**：7 处均为原生 `<button>`（未被 disabled 常态禁用）、可 Tab 聚焦、Enter/Space 可激活、文本直显（读屏可经 content 读出，非图标-only 缺口）——缺口仅限"聚焦时无可见指示器"这一纯 display 维度，无行为面/读屏名面/键盘操控面缺环。
  2. **修复面已成族围合**：F93-01 一行收敛全站 Button 组件路径；OBSERVE-105-01 收敛图标-only aria-label；O-3 族已指向 F6-02 模态迁移单一出口。残余面只剩"带文本裸按钮的焦点环"，量级低于立条线（键盘可用、仅视觉焦点缺失）。
  3. **连续四轮无新证据**：R106-R109 均建议转闭合，历轮本代理零回归、零新增成员（本轮 grep 实证 R104 补录 3 个 refetch + :944 同族成员为唯一增量且已入册）。
  4. **`:651/:661` 带强 active 态两面**——这是七处中最具量化缓解意义的观察点，但同样属于 106-01 家族（无焦点环），不构成新面；若未来修复，此两面应优先（显式记录于修复面清单）。
- **独立结论**：依据「走读 vs 实测、宁缺毋滥、无新证据不重复立条」纪律，**维持「续」但建议转「闭合」**——转闭合条件明确：残余面清点已入 F93-01 家族册（含 651/661 优先面），F6-02 模态迁移时随裸 button 焦点环统一收敛。最终裁决由主控合计。

### 4. 既往观察项延续
- **OBSERVE-106-01（本轮独立复核）**：见必查 3。维持「续」，建议转「闭合」待主控合计。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode）。「续」。
- **OBSERVE-85-02（延续）**：useTickingCountdown 整秒 setNow 如实口径在位（web/src/lib/useTickingCountdown.ts :4-7 注释族 + :13 setInterval 1000ms）。「续」。
- **OBSERVE-84-01（延续）**：Login 清票三路径在位（:80/:87-90/:92-98 success 与两 catch 分支、:232-238 Esc、:301-306 取消按钮）。「续」。
- **OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 前置短路分支在位（回显 effect 首帧数据缺席不置位 echoedRef）。「续」。
- **OBSERVE-77-02（延续）**：窗口关闭无处可点入口——Dashboard 目标矩阵 dateGroups 空态 :536 引导「前往挑选课程」+ 窗口关闭空态由 :739/Select :919 双空态文案兜底。「续」。
- **OBSERVE-76-03（延续）**：Toast Close :100 无 aria-label（Radix 受控）维持。「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：三模态（Login :223 / Admin :213 / Select :1204）role=dialog/aria-modal/aria-labelledby/Esc/autoFocus（Admin :241 / Login :267 / Select :1234）全部在位；无焦点陷阱/无关闭后焦点恢复的历轮口径不变，指向 F6-02 Radix Dialog 迁移单一出口。「续」。
- **契约 20 全仓扫描（src + scripts）零命中**：轮次标签族零命中；XSS 族（dangerouslySetInnerHTML/innerHTML=/outerHTML/new Function/eval(/document.write）零命中；tabIndex 全仓零命中；导航能力（window.open/location./history.）零命中（注：Admin:86 `document.execCommand("copy")` 为复制兜底、非导航）；localStorage 八处读写（App.tsx:20/:28/:39/:46/:57/:64 + types.ts:84 注释 + Admin.tsx:44/:55 注释）全 try/catch 降级——零命中。

### 5. 新契约角度——无障碍纵深五查（完整边界清点，交叉验证数据展示层/交互反馈）
- **第一查·键盘可达（全量清点）**：focus-visible ring 恰 4 处令牌归一（Button/Tabs/Login 密码切换/Admin switch）；tabIndex 全仓零命中（无破坏原生 Tab 序）；全部可交互元素为原生 button/Input 语义；三模态 Esc 全部在位；autoFocus 三处全落模态内合理。**结论：键盘操控零缺口。**
- **第二查·读屏语义（ARIA 属性族全量 grep 实证）**：role=dialog/aria-modal/aria-labelledby（三模态 :215/:228/:1206）✅、aria-expanded/aria-controls（CollapseSection :69-70，useId 唯一 id）✅、role=switch/aria-checked（Admin :579-580 + aria-label :581）✅、aria-pressed（Login :172）✅、aria-label（Select 搜索 :870 / Admin 复制删除 :470/:473 / Admin 表格 :833）✅、role=progressbar/aria-valuemin/max/now（Progress :25-28）✅、role="alert" 恰 3 处（Dashboard :320 / Login :182/:273，108-01 修复面）。**结论：读屏语义族完整，零新增缺口。**
- **第三查·焦点管理**：三手写 modal 无焦点陷阱/无关闭后焦点恢复——历轮 O-3 族观察已在册，本轮无新增，维持。
- **第四查·aria 动态态**：aria-busy 零命中（加载态全有显式文案覆盖：Dashboard「日志加载中」:715 / Select「正在同步最新课程列表与名额」:901 / Admin 著色 :448/:797/:903/:949 与「配置加载中」:694）；aria-live 全仓零显式命中（唯一播报通道 = Radix Toast Provider 内置 aria-live）；页面内嵌动态错误态已由 108-01 三处 role="alert" 覆盖。**结论：108-01 修复后动态态缺口闭合，残余 6 处失败态（Admin :451/:792/:898/:944、Dashboard :701、Select :909）均带独立失败文案 + refetch 出口，R108 归因量级低于立条线成立且本轮复核在位。**
- **第五查·焦点环残余面（边界闭合验证）**：可聚焦元素 = Button 组件（自带 ring）+ TabsTrigger（自带 ring）+ 裸 button 17 处（4 自带 ring + 3 aria-label + Toast Close + 3 单列族 + 10 无 ring 残余面与 106-01/F93-01 家族一致）。**四族边界恰好完整闭合：F93-01 修复面（组件环）∪ OBSERVE-106-01 残余面 ∪ O-3 模态族 ∪ 108-01 动态态 = 全风险面，无未分类焦点缺口。**
- **数据展示层交叉验证**：window_closed 三态（Dashboard :343-347 主徽章 + :751 移动页脚 / Admin StatsTab :740 注释在册）同源 type SchedulerState.window_closed?: boolean（types.ts:60，undefined 走"待命中"绝不假报关闭）；Select max_count=0 判据簇七处同源（fillRate :36 / 筛选 :967 / 排序键 :977 / isFull :1006 / remaining :1009 / unannounced :1010 / 文案+Progress :1093-1103——除法保护与后端 IsClassFull 同源）；Dashboard dateGroups 兜底链 :225-228（begin_date 自带 → /electives 映射 → "未知"）在位；Dashboard 主倒计时 openTimeStr → begin_times[0] 兜底（:191-196）与 Select :767-772 同构。**结论：数据展示层零缺陷。**
- **交互反馈五查（toast/弹窗/在飞/错误恢复/键盘反馈）交叉验证**：成功路径（Select :96/:123、Admin :270/:337、Login onLogin）✅；失败路径（各 catch 全 toast + 守卫不置 dirtyRef 不误报）✅；在飞幂等（Select actionLoading Set / Admin removing Set / Login loading / Admin deleting，入口短路 + disabled 双闸）✅；错误恢复（5 处 refetch + Dashboard refetchLogs :707 + 30s 降频失败自愈 + Select saveNow 指数退避 + 卸载后停手）✅；键盘反馈（Esc/autoFocus/disabled 指针行为）✅。**结论：交互反馈全路径一致性保持零缺陷。**
- **新角度结论**：无障碍纵深五查四族闭合、数据展示层与交互反馈零缺陷，无新立条项。

---

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十六轮闭合，见必查 1。下轮常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十八轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单一致。维持「续」。
- **OBSERVE-106-01（Admin 带文本裸按钮无焦点环）**：维持「续」，独立证据支持转「闭合」待主控合计（见必查 3）。
- **OBSERVE-108-01（role=alert）**：R109 已转闭合，本轮复核 3 处仍在位，不再报。
- **OBSERVE-88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log --oneline 8d79bcf..HEAD -- web/` | ✅ 空集（web/ 自 R109 零代码提交；1351fa4 为最后代码提交） |
| `git log --oneline -15 -- web/` | ✅ 1351fa4 头名实物，无 R110 前代码动线 |
| `shouldDeferSave(` 全仓 grep | ✅ 恰 4 消费点 + targetGuard.ts:64 定义，无第五消费处 |
| `echoedRef.current` 全仓 grep | ✅ 8 读点 + 6 注释 + 声明，运行时读点恰 6 处（:229/:319/:509/:597/:605/:699） |
| `<button` 全仓 grep | ✅ 17 处裸 button，残余面无 ring 面清单与历轮逐项一致 |
| `focus-visible` 全仓 grep | ✅ 恰 4 处令牌归一（Button :42 / Tabs :29 / Login :173 / Admin :583） |
| 契约 20 残留扫描（轮次标签族/XSS 族/tabIndex/导航能力/localStorage try/catch） | ✅ 零命中 |
| `role="alert"` | ✅ 恰 3 处（Dashboard :320 / Login :182/:273，108-01 面） |
| `aria-live`/`aria-busy` | ✅ 零命中（Toast Provider 内置为唯一 aria-live） |
| `npx tsc -b --pretty false`（web/） | ✅ EXIT 0 |
| `npm run build`（web/） | ✅ built in 371ms，index-1KHlpqcc.css 41.72 kB / index-Bm7TtkV4.js 420.77 kB——与 R109 记录逐字节一致，double 实证零改动 |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 断言全绿（18 断言，含 F43 清空放行 + 稳态 echoed=true 放行） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 断言全绿 |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致） |
| `git status --short --branch` | ✅ `## master`（clean） |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + git log/show/status + tsc -b + npm run build + 四守护脚本），未修改任何仓库代码文件，唯一写入为本报告。
- 实测证据：四处消费点/读点/六防判据 grep 全量、`<button` 与 `focus-visible` 与 ARIA 族全量 grep、契约 20 五类零命中、tsc/build 退出码与产物哈希（与 R109 逐字节一致的双重零改动证据）、四守护脚本断言、git log/status。走读推断：106-01 裁决论证（焦点可见性事实 → 键盘链路完整性 → 修复面围合 → 转闭合建议，全部基于 grep 实证的代码事实）；数据展示层/交互反馈同源语义评估。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR/新增 OBSERVE；M-1 第四十六轮闭合；F93-01 第十八轮通过；OBSERVE-106-01 维持「续」但独立证据支持转「闭合」待主控合计；连续第五十六轮无严重级发现。