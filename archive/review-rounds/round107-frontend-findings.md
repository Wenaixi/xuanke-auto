# R107 前端只读审查 Findings

基线：commit 3fbd3bb（R106 收尾，进度 107/256）。**本轮为 OBSERVE-106-02 修复回首轮 + M-1 第四十三轮 + F93-01 第十五轮**，必查四项 + 新契约角度 b（数据展示层一致性纵深对照）。审查范围：web/src 全部 .ts/.tsx，交叉核对 f5fdb36 提交 diff、git log、组件库 .tsx、build/守护脚本实测。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR；新增 OBSERVE 一条（OBSERVE-107-01：Dashboard 学生端 `/logs` 失败态无重试出口）；OBSERVE-106-02 确认修复并转「闭合」态（对账 R106 判定矩阵全部成立）；OBSERVE-106-01 转「闭合」待定（见下）。** 必查 1：Dashboard.tsx:699-728 日志区四态逐字符复核通过、f5fdb36 后 web/ 零新提交（含 f5fdb36 本身为 web/ 最后一个代码提交）、build 独立产物命名与新哈希；必查 2：M-1 第四十三轮闭合（四处消费点 + 置位三路径 + 首帧边界 + 读点清点全部通过）；必查 3：F93-01 第十五轮复核通过（Button.tsx:42 ring 在位 + 残余面清单一致 + 带文本裸按钮 7 个成员在位）；必查 4：OBSERVE-106-01/90-01/88-01 等延续、契约 20 全仓扫描零命中；新角度 b（数据展示层 nil/空值/异常数字/失败态与加载态一致性）暴露 OBSERVE-107-01，其余全达标。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| OBSERVE-107-01 | OBSERVE | `web/src/routes/Dashboard.tsx:699-702`（学生端日志区失败态） | **/logs 查询失败态只有失败文案、无「重试」出口**——渲染 `日志加载失败（网络异常或服务端不可达）` 后靠 30s 自动降频轮询自愈，但用户无法主动重试；对照 Admin 五 Tab 的失败态均有「重试」裸按钮（Codes :452 / Config :701 / Stats :792 / Accounts :898 / Logs :944 全部是失败态旁 `logsQuery.refetch()` 出口），学生端独缺。触发面：网络挂断后手动恢复的场景下，学生端需干等最长 30s 才自动重拉——纯可用性降级，无数据/安全/功能危害。按历轮 OBSERVE 口径立条跟踪，修复为 display 级（失败态内补一个 refetch 按钮，随 records 栏）。 |

> 备注（不立条，清点留痕）：student 端 /logs 失败态 `!logs` 的加载中分支将 isLoading 与「失败无缓存」两态合并（post-fix 实际渲染均为"日志加载中..."），与 Admin LogsTab 独立四态（加载中/有数据/失败/空）在严格文案口径上仍有第 5 个半态差异，但 OBSERVE-106-02 的「绝不伪装空态」核心契约已由四态立住，该分叉量级低于立条线——已在对账 106-02 正文说明。

---

## 三、必查项逐条结论

### 1. OBSERVE-106-02 修复回首轮
- **四态逐字符复核在位（Dashboard.tsx:699-728）**：`{logsErr ?` 失败文案「日志加载失败（网络异常或服务端不可达）」/ `: !logs ?` 「日志加载中...」/ `: logs.length > 0 ?` 列表 / `: NO RECENT LOGS`——四态顺序、文案逐字符与 f5fdb36 diff 完全一致。:697-698 注释「失败/加载独立态：/logs 查询失败与加载期绝不伪装成"无日志"（OBSERVE-106-02）」在位。R106 判据矩阵对账：①失败不再伪装空态→成立；②加载中不再落空态→成立（但 `!logs` 未细分 isError 与 isLoading，两态文案同为"日志加载中..."，见 OBSERVE-107-01 备注）；③与 Admin 四态对齐→loading/失败/列表/空 四态结构对齐成立。
- **f5fdb36 后 web/ 零新提交**：`git log f5fdb36..HEAD -- web/` 空集（exit 0）；`git diff f5fdb36..HEAD -- web/` 空 diff——HEAD=3fbd3bb 为 R106 收尾（docs commit），web/ 最后一个代码提交即 f5fdb36。
- **build 哈希**：npm run build 产出 `index-DnlP8miw.js 420.54 kB / index-1KHlpqcc.css 41.72 kB`——css 体积与 R105/R106 记录逐字节一致（41.72 kB）；JS 名每次构建重摇（R105 记录 CZO4oHGZ、R106 记录 CXlKRCU9、本轮 DnlP8miw），源码 diff 已实证 f5fdb36 后零改动，产物差异不构成回归证据。
- **结论：OBSERVE-106-02 → 转「闭合」。**

### 2. 保存链 M-1 稳态语义（第四十三轮）
- **四处消费点全传第三参 echoedRef.current 逐字符复核通过**：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` —— handleBack 循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
  - 全仓 grep `shouldDeferSave(` 命中仅此四处消费点 + targetGuard.ts:64 定义，**无第五消费处确证**。
- **shouldDeferSave 纯函数定义（targetGuard.ts:64-72）逐字符复核**：`stateData === undefined → true`；`echoed → false`；`(stateData.courses?.length ?? 0) > 0 && hasSelected`——与 R105/R106 记录完全一致。
- **echoedRef 置位三路径**：:162 声明 `useRef(false)`；:200 账号复位 effect 置 false；:240 courses 空分支置 true + setEchoDone(true)；:297 合并完成置 true + setEchoDone(true)；**首帧边界**：:234 `if (stateData === undefined) return`、:247 `if (pubs.length === 0) return` 前置短路不置位；:229 回显 effect 首行 `if (echoedRef.current) return`、:319 清理 effect 守卫在位。:256 全清空守卫 `rev > 0 && !anyHas → return prev` 不置位。
- **读点全量清点**：运行时读点 6 处 = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699，与历轮完全一致。grep echoedRef 引用 22 行（声明 :162 + 复位 :200 + 置位 2 + 读 6 + 注释 10）无第五消费处。
- **六防保存链零回潮**：防抖回调五判据（shouldDeferSave :699 → 发布缺席 :709 → selectedHasStalePublish :717 → 联查空 :732 → targetsUseCurrentPublishes :737）与 flush 五判据（:509/:515/:526/:553/:558）同款消费时刻读最新 ref，逐字符复核在位；dirtyRef 置 true 三处（saveNow catch / flush 在飞 :563 / 防抖在飞 :742）全在；saveNow finally 补发链（:457-466）在位；unmountedRef 守卫与「卸载后不 fire/不 toast」对齐。target-guard-check 断言全绿（含 F43 全清空放行 PUT [] 与「无变更返回原引用」）。
- **结论：第四十三轮延续闭合。**

### 3. F93-01 无障碍焦点（第十五轮复核）
- **Button.tsx:42 ring 逐字符在位**：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，置于 base class（cn() 首参），与 R93 提交原文逐字符一致；Tabs.tsx:29 ring 同款 token 在位（R104 修复后无回归）；Admin :583 开关 `role="switch" aria-checked aria-label="激活码机制"` + `focus-visible:ring-2 focus-visible:ring-white/60` 在位；Login :173 密码切换 ring 在位。
- **残余面清单与历轮一致**：裸 `<button>` 无 ring 面全量 grep 复核——Dashboard CollapseSection :67、Admin :417/:425/:441/:452/:651/:661/:701/:792/:898/:944、Login 激活取消 :299、Toast Close :100，全部仍在。**OBSERVE-106-01 的 7 个带文本裸按钮（:417/:425/:441/:452/:651/:661/:701）确认在位更新**。同族自带 ring 面排除（Admin :577 switch / Login :173）。
- **结论：第十五轮复核通过（F93-01 本体零回归）；OBSERVE-106-01 转「闭合」待主控裁决。**

### 4. 既往观察项延续
- **OBSERVE-106-02（上一轮新增，本轮回首）→ 转「闭合」**：见必查 1。
- **OBSERVE-106-01（上一轮新增，本轮回首）**：7 个带文本裸按钮仍在位、分界线（带文本读屏可读无 aria-label 缺口）确认——OBSERVE 面上无新证据、无回归，建议转「闭合」待主控与 R107 后端合计。
- **OBSERVE-90-01（延续）**：Dashboard 学生端 /logs 已四态（R106 修复），90-01 原「日志区三态与 Admin 完整四态不对称」的失败/加载态部分已闭合；剩余对称性差异仅学生端无重试出口（新增 OBSERVE-107-01）与 `!logs` 半态合并——**建议 90-01 转部分闭合**，残余面由 107-01 承接。具体裁定交由主控综合。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode）。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：85-02 倒计时的 cycle 组件里注释在 ; 84-01 激活票据三路径清票（Login :87-97/:232-238/:296-311 复核在位）；83-01 Select :247 `pubs.length === 0` 分支仍在；77-02 窗口关闭无目标管理入口；76-03 Toast Close 无 aria-label（Radix 受控，历轮口径维持）。全部维持「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮三模态 role/aria-modal/aria-labelledby/Esc/autoFocus 复核在位。
- **契约 20 全仓扫描零命中**：轮次标签族（`第 ?[0-9]+ ?轮|R1?0?[0-9]|B[0-9]{2}-|F[0-9]{2}|M-1`）命中仅 Dashboard:697 一处 OBSERVE-106-02 **修复锚点注释**（readme 契约判定该锚点保留合法，非残留标签）；行号引用族（`\b\w+\.tsx?:\d+\b`）零命中；XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / new Function / eval）零命中；localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级；导航能力（window.open / location.* / history.*）零命中；tabIndex 全仓零命中——全仓零命中。

### 5. 新契约角度 b——数据展示层一致性纵深对照（nil/空值/异常数字/失败态在全部渲染点）
- **Admin 五 Tab 四态结构清点**：Codes（:447 loading / :449 isError+重试 / 有数据列表 / :483 空「暂无激活码」）、Config（:700 loading / :700 失败+重试 / 有数据）、Stats（:788 `s ?` 有数据 / :789 isError+重试 / 空 rows 渲染空白宿主）、Accounts（:829 loading / 有数据表格 / :895 isError+重试 / :903 空「暂无账号」）、Logs（:928 loading / 有数据 / :941 isError+重试 / :949 空「暂无日志」）。**统一「加载中 → 有数据 → 失败+重试 → 空」四态家族，学生端 Dashboard 全区对齐后**，仅 /logs 区暴露无重试缺口（OBSERVE-107-01）。
- **学生端数据展示层字段级对照**：主状态卡（/state）session 失效错误条 :319 `!stateLoading && stateErr && isSessionError(stateErr)`（仅 401 类触发「重新登录」出口，非 401 的纯网络失败静默隐于面板——历轮口径排除，维持）；运行指标卡 `state?.token_valid === false` 双 Domino 血灯语义到位；课程预选区 dateGroups 空态「当前未添加任何预选课程」+ 前往挑选按钮（:536-543）在位；多开放时间 extras 折叠 `extrasMs.length > 0` 守卫（:427）在位——**该层全部达标**。
- **异常数字面**：Select isFull 同源判据（:1006 `c.max_count > 0 && c.selected_count >= c.max_count`）；max_count=0 名额未公布三处同源兜底（筛选 :965-967 / 排序 :977 / remaining 警示 :1009）逐字符复核；Progress.tsx:12 percentage 双层 clamp `Math.min(100, Math.max((value / max) * 100, 0))` 覆盖 NaN 风险（value/max 数学推导出 NaN 被 clamp 直接压 0）——**数字异常面全达标**。
- **超长文案面**：Select 课程卡片 `break-all`（结果文案）、Dashboard 日志条目 result `break-all`、Admin logs result `break-all`、Admin 账号表格 course_name 用 Badge flex-wrap——无超长撑破；唯一无 `break-all/tabular-nums` 处为运行指标卡 `预选目标 / LogsTab 最近 N 条` 描述行（数值计入 `tabular-nums`，富文本描述行无 `break-all`）——量级为「描述行超长极端场景」，低于立条线，清点留痕。
- **加载态一致性**：Admin 五 Tab 均有「加载中...」；Dashboard 新区 /electives 30s 轮询 `data 为空源` 依赖 `?` 链优雅降级（ElectivesData 可选字段）——无崩溃面。
- **新角度结论**：数据展示层纵深对照暴露一个 OBSERVE（学生端 /logs 失败态无重试出口），其余面（递归空态/多态文案/异常数字/超长文案）全达标。

---

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十三轮闭合，见必查 2。下轮继续常规核对。
- **OBSERVE-106-02（日志区失败态伪装空态）**：本轮确认修复 → 转「闭合」，移出观察列表。
- **OBSERVE-106-01（Admin 7 个带文本裸按钮无焦点环）**：本位维持「续」，建议转「闭合」待主控合计。
- **OBSERVE-107-01（本轮新增）**：Dashboard 学生端 /logs 失败态缺重试出口，OBSERVE，修复 display 级（失败态内补 refetch 按钮）。
- **OBSERVE-90-01（日志区态序）**：R106 修复后失败/加载独立态已闭合，剩余学生端无重试出口由 107-01 承接——建议转部分闭合，裁定交主控。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十五轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单与历轮一致 + OBSERVE-106-01 成员更新。维持「续」。
- **OBSERVE-88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log f5fdb36..HEAD -- web/` | ✅ 空集（0 提交，web/ 在 OBSERVE-106-02 修复后无新代码） |
| `git diff f5fdb36..HEAD -- web/` | ✅ 空 diff（f5fdb36 即 web/ 最后一个代码提交） |
| Dashboard.tsx:699-728 四态 | ✅ 逐字符复核在位（logsErr/!logs/有数据/空） |
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 613ms，产物 index-DnlP8miw.js 420.54 kB / index-1KHlpqcc.css 41.72 kB（css 与历轮记录逐字节一致） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 断言全绿（F43 全清空放行 + 无变更原引用） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 断言全绿 |
| `node scripts/audit.mjs` | ✅ 77 断言全通过（视觉表面协调一致） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力/tabIndex 全形态） | ✅ 零命中（Dashboard:697 唯一锚点注释合法保留） |
| `git status --short --branch` | ✅ `## master`；除本报告外零改动 |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + npm run build + 三守护脚本 + audit.mjs + git log/show/status/diff），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：M-1 逐字符、三守护脚本断言、tsc/build 退出码、契约 20 扫描各类、git log/show/rev-parse/diff、OBSERVE-106-02 修复逐字符、build 产物均为实测证据；OBSERVE-107-01（学生端 /logs 失败态无重试出口的可用性量级）为走读推断。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新增 OBSERVE 一条（学生端 /logs 失败态无重试出口）；OBSERVE-106-02 转闭合；OBSERVE-106-01 建议转闭合待主控合计；OBSERVE-90-01 建议部分闭合（残余面由 107-01 承接）；M-1 第四十三轮闭合；F93-01 第十五轮通过；连续第五十三轮无严重级发现。