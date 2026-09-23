# R109 前端只读审查 Findings

基线：commit 47fe87e（R108 收尾，进度 109/256）。**本轮为 OBSERVE-108-01 修复回首轮 + M-1 第四十五轮 + F93-01 第十七轮**，必查四项 + 新契约角度 a（无障碍纵深四查残余面清点 + 数据展示层三查同源一致性）。审查范围：web/src 全部 .ts/.tsx，交叉核对 1351fa4 提交 diff、git log、组件库源码、build/守护脚本/tsc 实测。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新增 OBSERVE（宁缺毋滥——数据展示层三查与无障碍残余面清点全部与历轮口径一致，无够格立条的新证据）；OBSERVE-108-01 确认修复并转「闭合」；OBSERVE-106-01 维持（建议转闭合待主控合计）。** 必查 1：三处 `role="alert"` 逐字符复核在位、1351fa4 后 web/ 零新提交、build 产物 css 41.72 kB 与历轮逐字节一致；必查 2：M-1 第四十五轮闭合（四处消费点 + 置位三路径 + 首帧边界 + 读点清点 + 六防保存链零回潮全部通过）；必查 3：F93-01 第十七轮复核通过（Button.tsx:42 ring 在位 + 残余面清单一致含 Dashboard:707）；必查 4：OBSERVE-106-01/88-01 等既往观察延续 + 契约 20 全仓扫描零命中；新角度 a：108-01 修复后残余失败态 6 处全量清点留痕（量级低于立条线）+ 数据展示层学生端/Admin 端 window_closed 三态同源一致、Select 数字处理 max_count=0 判据簇六处同源零缺陷。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| — | — | — | **本轮零新增立条项**。无障碍残余面（Admin 五 Tab 失败态 :451/:791/:897/:943、Dashboard 日志失败态 :701、Select 拉取失败态 :909）与数据展示层全部与历轮口径一致；108-01 修复覆盖面刻意小而准，无新够格疑点。按「报告真实问题，宁缺毋滥」纪律维持零新增。 |

> 备注（不立条，清点留痕）：
> 1. **108-01 修复后的残余失败态面**：`role="alert"` 全仓 grep 命中仅 3 处（即 108-01 的三处 :320/:182/:273）；`aria-live` 与 `aria-busy` 仍全仓零命中。剩余 6 处失败/拉取异常态（Admin 激活码 :451、Config :791、Stats :897、Logs :943、Dashboard 日志 :701、Select 课程 :909）均无 live region——R108 已明确归因"有显式失败文案 + refetch 按钮显式呈现，量级低于立条线"，本轮复核确认该归因成立（六处全部带独立失败文案与可交互重试/可见按钮，读屏用户可经 Tab 感知），维持不入条。
> 2. Dashboard 窗口状态徽章两处文案微异：:346"窗口已开放"（主状态徽章）/ :751"窗口开放中"（页脚系统态），语义一致、刻意区分展示层级，量级低于立条线。
> 3. Toast Close :100 无 aria-label（Radix 受控，历轮 76-03 口径）维持。

---

## 三、必查项逐条结论

### 1. OBSERVE-108-01 修复回首轮
- **三处 `role="alert"` 逐字符在位**（grep 实证，全仓 `role="alert"` 命中恰 3 处）：
  - Dashboard.tsx:320 `<div role="alert" className="rounded-[var(--radius-sm)] border border-neutral-800 glass-strong p-4 text-xs flex items-center justify-between text-neutral-300">`——会话失效条，与 1351fa4 diff 逐字符一致。
  - Login.tsx:182 `<div role="alert" className="p-3 rounded-[var(--radius-sm)] glass border border-neutral-700 text-xs text-neutral-300 flex items-center gap-2">`——登录失败条，与 diff 逐字符一致。
  - Login.tsx:273 同款 `role="alert"`——激活失败条，与 diff 逐字符一致。
- **1351fa4 后 web/ 零新提交**：`git log --oneline 1351fa4..HEAD -- web/` 空集（仅 47fe87e docs 收尾非代码提交）。
- **build 哈希**：`npm run build` 产出 `index-1KHlpqcc.css 41.72 kB`——与 R105-R108 五轮记录逐字节一致；JS `index-Bm7TtkV4.js 420.77 kB` 每次构建重摇（R106=CXO4oHGZ/R107=DnlP8miw/R108=C_ZSK3dI/本轮=Bm7TtkV4）属 Vite 正常哈希抖动，源码 diff 已实证零改动。git show 1351fa4 --stat 确认改动仅 Dashboard.tsx(2L)/Login.tsx(4L) 3 处插入。
- **结论：OBSERVE-108-01 → 转「闭合」。**

### 2. 保存链 M-1 稳态语义（第四十五轮）
- **四处消费点全传第三参 `echoedRef.current` 逐字符复核通过 + 无第五消费处 grep 实证**：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` —— handleBack 循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
  - 全仓 grep `shouldDeferSave(`（web/src）命中仅此四处 + targetGuard.ts:64 定义 + 注释，**无第五消费处确证**。
- **shouldDeferSave 纯函数定义（targetGuard.ts:64-72）逐字符复核**：`stateData === undefined → true`；`echoed → false`；`(stateData.courses?.length ?? 0) > 0 && hasSelected`——三段注释（纯数据不依赖 echoedRef / 第二参数清空分判 / 第三参稳态放行）与 R106-R108 记录完全一致。
- **echoedRef 置位三路径**：:162 声明 `useRef(false)`；:200 账号复位 effect 置 false；:240 courses 空分支置 true + setEchoDone；:297 合并完成置 true + setEchoDone。**首帧边界**：:234 `stateData === undefined` 前置短路不置位、:247 `pubs.length === 0` 短路不置位、:229 回显 effect 首行盾、:319 清理 effect 盾、:256 全清空守卫 `rev > 0 && !anyHas → return prev` 不置位——全部在位。
- **读点全量清点**：运行读写点 6 处 = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699，grep echoedRef 命中清单与历轮逐行一致。
- **六防保存链零回潮**：防抖回调五判据（shouldDeferSave :699 → 发布缺席 :709 → selectedHasStalePublish :717 → 联查空 :732 → targetsUseCurrentPublishes :737）与 flush 五判据（:509/:515/:526/:553/:558）消费时刻读最新 ref 逐字符在位；dirtyRef 置 true 三处（saveNow catch :455 / flush :563 / 防抖 :742）、补发链、unmountedRef 守卫、防抖依赖三自愈信号（echoDone/stateData/hasPublishes）全部在位。
- **TDD/实测**：target-guard 断言全绿（18 断言含 F43 清空放行 + 稳态 echoed=true 放行）、admin-auth 全绿、unauthorized 全绿、audit 全绿、tsc EXIT 0。
- **结论：第四十五轮延续闭合。**

### 3. F93-01 无障碍焦点（第十七轮复核）
- **Button.tsx:42 ring 逐字符在位**：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`（base class cn() 首参，:39-41 全局 outline:none 补偿注释语义完整），与 R93 原文逐字节一致，自前十六轮零回归。Tabs.tsx:29 同款 ring、Admin :577 switch ring、Login :173 密码切换 ring 全部在位。
- **残余面清单与历轮一致**：带文本/裸 button 无 ring 面全量 grep——Admin :417 收起 / :425 复制 / :441 刷新 / :452 重试 / :651/:661 引擎二选一 / :701 重试 / :792/:898/:944 三 refetch、Dashboard :67 CollapseSection + **:707 重试按钮（R108 新增成员）**、Login :299 激活取消、Toast :100 Close，全部在位。**OBSERVE-106-01 的 7 个带文本裸按钮（:417/:425/:441/:452/:651/:661/:701）在位更新**；R104 补录 3 个 refetch（:701/:792/:898）+ :944 同族成员在位。自带 ring 面排除（Admin :577 switch / Login :173）。
- **结论：第十七轮复核通过（F93-01 本体零回归）；OBSERVE-106-01 维持续、建议转闭合待主控合计。**

### 4. 既往观察项延续
- **OBSERVE-108-01（上一轮新增，本轮回首）→ 转「闭合」**：见必查 1。
- **OBSERVE-106-01（上一轮新增，本轮回首）**：7 个带文本裸按钮仍在位、分界线与历轮一致；无新证据、无回归，建议转「闭合」待主控合计。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode，无 error boundary）。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：85-02 useTickingCountdown 整秒 setNow 如实口径在位（:11/:13）；84-01 清票三路径复核在位（Login 取消 :232-238/:301-306 清 pendingAccount/ticket/error，Esc 同逻辑 :229-237 注释由"承诺"升级为已落地实现）；83-01 Select :247 `pubs.length === 0` 分支在位；77-02 窗口关闭无目标入口；76-03 Toast Close 无 aria-label（Radix 受控）维持。全部「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。三模态 role=dialog/aria-modal/aria-labelledby/Esc/autoFocus 复核在位（Login :223 / Admin :208 / Select :1198）。
- **契约 20 全仓扫描零命中**：轮次标签族（`第 ?N ?轮` 多形态）web/src + web/scripts 零命中；XSS 族（dangerouslySetInnerHTML/innerHTML=/outerHTML/new Function/eval(/document.write）零命中；tabIndex 全仓零命中；导航能力（window.open/location.*/history.*）零命中；localStorage 八处读写（App.tsx:20/:28/:39/:46/:57/:64 + client.ts 注释）全 try/catch 降级——全仓零命中。

### 5. 新契约角度 a——无障碍残余面清点 + 数据展示层三查（同源一致性）
- **无障碍残余面清点（108-01 修复后）**：`role="alert"` 命中恰 3 处（修复面）；`aria-live`/`aria-busy` 全仓零命中。6 处残余失败态（Admin :451/:791/:897/:943、Dashboard :701、Select :909）均带独立失败文案 + 可交互重试出口、读屏可经 Tab 感知——R108 "量级低于立条线"归因成立，维持不入条。aria 动态态唯一播报通道仍为 Radix Toast 自带 aria-live。**结论：108-01 修复覆盖精准无遗漏面漏项，无新增观察。**
- **数据展示层三查**：
  - **window_closed 三态同源一致性**：学生端 Dashboard :346 主徽章 + :751 页脚系统态，与 Admin StatsTab :740 `s.window_closed ? "已关闭" : s.window_opened ? "已开放" : "待命中"` 三态同源（后端 WindowClosed 单判据下发，B39-05 契约），undefined 走"待命中"绝不假报关闭——两端口径一致。
  - **Select 数字/名额显示 max_count=0 判据簇六处同源**：isFull :1006 `max_count > 0 && selected_count >= max_count`、remaining :1009 `max_count > 0 ? max(max-0, 0) : 0`、unannounced :1010 `max_count <= 0`、筛选 :967 `max_count === 0 || selected_count < max_count`、排序键 :977 `max_count > 0 ? max - selected : 0`、文案 :1093-94 `未公布 ? "N 人已报 · 名额未公布" : "selected / max 人 (rate%)"`——六处全部 `max_count` 除法保护 + 语义与后端 IsClassFull 同源，与 localStorage 契约 15 四处同源承诺一致。fillRate :36 `!c.max_count → 0` 与 Progress :1102-1103 `unannounced 时 value=0/max=1`（aria-valuenow=0 如实反映未公布）零缺陷。
  - **加载失败态**：react-query 失败后 data 保留最后一次成功值（:137/:64-68 注释）→ 失败态降频 30s 与成功态降/升频分离，/electives 与 /state 同款，轮询间隔回调读缓存（:77 queryClient.getQueryData）绕 TDZ 的注释承诺落实。Dashboard 日期分组 begin_date 兜底链（:225-228 自带 → /electives 映射 → "未知"）与 Select/Tabs 空态文案全部在位。
  - **结论：数据展示层零缺陷——学生端与 Admin 端同数据源呈现一致，数字处理判据簇同源，无够格立条项。**

---

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十五轮闭合，见必查 2。下轮常规核对。
- **OBSERVE-108-01（页面内嵌错误条 role=alert）**：本轮确认修复 → 转「闭合」，移出观察列表。
- **OBSERVE-106-01（Admin 7 个带文本裸按钮无焦点环）**：维持「续」，建议转「闭合」待主控合计。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十七轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单一致含 Dashboard:707。维持「续」。
- **OBSERVE-88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log --oneline 1351fa4..HEAD -- web/` | ✅ 空集（1351fa4 即 web/ 最后代码提交） |
| 三处 `role="alert"`（Dashboard:320 / Login:182 / Login:273） | ✅ grep 全仓命中恰 3 处，与 1351fa4 diff 逐字符一致 |
| `git show 1351fa4 --stat -- web/` | ✅ 仅 Dashboard.tsx(+1-1)/Login.tsx(+2-2)，3 处插入 |
| `npx tsc -b --pretty false`（web/） | ✅ EXIT 0 |
| `npm run build`（web/） | ✅ built in 498ms，产物 index-1KHlpqcc.css 41.72 kB（与 R105-R108 记录逐字节一致）/ index-Bm7TtkV4.js 420.77 kB（Vite 哈希抖动属正常） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 断言全绿（18 断言） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 断言全绿 |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致） |
| 契约 20 残留扫描（轮次标签族/XSS 族/tabIndex/导航能力/localStorage try/catch） | ✅ 零命中 |
| shouldDeferSave 消费点全量 grep | ✅ 恰 4 消费点 + 定义，无第五消费处 |
| `git status --short --branch` | ✅ `## master`（clean） |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + git log/show/status + tsc -b + npm run build + 四守护脚本），未修改任何仓库代码文件，唯一写入为本报告。
- 实测证据：三处 role=alert grep + diff 逐字符、tsc/build 退出码与产物、四守护脚本断言、契约 20 扫描各类、git log/show/status、M-1 消费点/置位点 grep 全量。走读推断：数据展示层窗口三态与数字判据簇的同源语义评估（代码事实均已行号引用 + grep 实证）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR/新增 OBSERVE；OBSERVE-108-01 转闭合；OBSERVE-106-01 维持续（建议转闭合待主控合计）；M-1 第四十五轮闭合；F93-01 第十七轮通过；连续第五十五轮无严重级发现。