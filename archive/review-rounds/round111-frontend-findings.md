# R111 前端只读审查 Findings

基线：commit 616ab9e（R110 收尾，进度 111/256）。**本轮为 M-1 第四十七轮 + F93-01 第十九轮 + OBSERVE-93-01 残余面（106-01 转闭合后）复核**，必查六项 + 新契约角度（登录/激活/会话状态机纵深）。审查范围：web/src 全部 .ts/.tsx，交叉核对 git log（1351fa4 为 web/ 最后代码提交、616ab9e 为纯 docs）、组件库源码、build/守护脚本/tsc 实测。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新增 OBSERVE（宁缺毋滥）。** 必查 1：M-1 第四十七轮闭合——四处消费点全传第三参逐字符一致 + grep 实证无第五消费处 + 置位三路径 + 首帧边界 + 读点清点 + 18 断言全绿；必查 2：OBSERVE-93-01 残余面（106-01 转闭合后 7 个带文本裸按钮并入家族册）逐点位确认键盘链路完整、缺口仍纯 display 维度、F93-01 修复面零回归、残余面清单零增零减；必查 3：F93-01 第十九轮——Button.tsx:42 ring 逐字符在位 + `git log --oneline -5 -- web/` 实证前端零代码提交漂移；必查 4：无障碍纵深延续——108-01 三处 role=alert 全部在位 + Tabs/switch/密码切换同款 ring 在位；必查 5：新契约角度「会话状态机纵深」——四条会话清理链对称性 + 管理令牌标记全路径清除 + 401 双通道广播 + 激活票据链零缺陷；必查 6：格式卫生快扫——相邻 JSX 同行/缩进级差半程态零复发。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| — | — | — | **本轮零新增立条项**。全部必查项、会话状态机纵深与历轮口径一致；`git log --oneline -5 -- web/` 实证 R110 后前端零代码提交（1351fa4 为最后 web 提交），无任何可生成新证据的代码面。按「报告真实问题，宁缺毋滥」纪律维持零新增。 |

> 备注（不立条，清点留痕）：
> 1. **无焦点环裸 button 全仓清点复证**：grep 实证 `<button` 命中 17 处，清单与 R110 逐项一致（Admin :417/:425/:441/:452/:470/:473/:577/:651/:661/:701/:792/:898/:944、Dashboard :67/:707、Login :167/:299、Toast :100）——**无 ring 残余面 10 处零增零减** = 7 个带文本（OBSERVE-106-01 转闭合后入 OBSERVE-93-01 家族册 :417/:425/:441/:452/:651/:661/:701）+ R104 补录 3 个 refetch（:792/:898/:944）等。自帶 ring 4 处排除（Button :42 / Tabs :29 / Login :173 / Admin :583 switch）。
> 2. **R110 备注 2 措辞微异复证**：Dashboard :30-33 与 Select :40-43 priorityName 注释措辞微异、实现逐字节一致（`p===0 ? "首选" : \`备选 ${p}\``），已入册不重复立条。
> 3. **onUnauthorized effect 依赖数组含闭包 current 但不列依赖**（App.tsx:188-235，eslint-disable 注释在册）——detail.session 恒为 401 真实主体（client.ts 恒传 session），current 仅作 detail 双缺的兜底，量级低于立条线，维持观察。

---

## 三、必查项逐条结论

### 1. M-1 第四十七轮闭合
- **四处消费点全传第三参 `echoedRef.current` 逐字符复核通过 + 无第五消费处 grep 实证**（命中恰 4 消费点 + targetGuard.ts:64 定义，全仓零遗漏）：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` —— handleBack 等待循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
- **shouldDeferSave 定义（targetGuard.ts:64-72）逐字符复核**：`stateData === undefined → true`；`echoed → false`；`(stateData.courses?.length ?? 0) > 0 && hasSelected`——三段注释（纯数据不依赖 echoedRef / 第二参清空分判 / 第三参稳态放行）完整在位。
- **echoedRef 置位三路径**：:162 声明 `useRef(false)`；:200 账号复位置 false（account reset 分支）；:240 courses 空分支置 true + setEchoDone(true)；:297 合并完成置 true + setEchoDone(true)。**首帧不置位边界**：:234 `stateData === undefined` 前置短路不置位、:247 `pubs.length === 0` 短路不置位、:229 回显 effect 首行盾、:319 清理 effect 盾（`!echoedRef.current || publishes.length===0`）、:256 全清空守卫 `rev > 0 && !anyHas → return prev` 不置位——全部在位。
- **读点清点**：grep `echoedRef.current` 命中 8 读点 + 6 注释 + 声明；**运行时读点恰 6 处** = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699，无第五运行时读点，与历轮一致。
- **TDD/实测**：`node --import jiti/register scripts/target-guard-check.ts` 18 断言全绿（含首帧推迟双场景 + courses 空放行双场景 + echoed=true 稳态放行 + 首帧+已回显仍推迟 + stale/clean 全场景）。
- **结论：M-1 第四十七轮延续闭合。**

### 2. OBSERVE-93-01 残余面（R110 转闭合 OBSERVE-106-01 后首轮家族册复核）
- **7 个带文本裸按钮逐点位复核**（Admin :417/:425/:441/:452/:651/:661/:701，逐字 inline 复核在位）：
  - :417 收起 / :425 复制行（Copy 图标+文本）/ :441 刷新（RefreshCw+文本）/ :452 重试 / :701 重试——文本直显、原生 button、可 Tab 聚焦、Enter/Space 可激活。
  - **:651/:661 引擎二选一**：带强 active 态（选中白底黑字 border-white font-medium vs 未选 glass-input），仍是七处中最优先修复面——键盘 Tab 聚焦无可见指示器，仅鼠标 hover 有态。已随 106-01 转闭合入 OBSERVE-93-01 家族册，F6-02 模态/焦点迁移时统一收敛。
- **缺口仍纯 display 维度**：global.css:174 `button, input, select, textarea { outline: none }` 全局抹掉原生焦点；`focus-visible` 全仓命中恰 4 处（Button.tsx:42 / Tabs.tsx:29 / Login.tsx:173 / Admin.tsx:583），7 处裸按钮全部零 ring——修复面与残余面分界清晰，无行为面/读屏名面/键盘操控面缺环。
- **F93-01 修复面零回归**：Button.tsx:42 `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]` 逐字符在位（:38 base cn() 首参 + :39-42 注释族）。
- **残余面清单零增零减**：grep `<button` 17 处与 R110 逐项一致，无新增成员。
- **结论：OBSERVE-93-01 残余面复核通过（缺口事实成立、键盘链路完整、清单零漂移），维持「续」。**

### 3. F93-01 第十九轮
- **Button.tsx:42 ring 逐字符在位零回归**（见必查 2，自前十八轮零漂移）。
- **`git log --oneline -5 -- web/` 实证前端零代码提交漂移**：1351fa4（OBSERVE-108-01 role=alert）→ ae099cc → f5fdb36 → a12a09c → 037410a，与 R110 记录逐项一致；616ab9e/57bf401/8d79bcf 均为 docs/backend，web/ 自 R108 修复后零代码动线。
- **结论：第十九轮通过。**

### 4. 无障碍纵深延续（108-01 复核 + 同款 ring 在位）
- **108-01 三处 role="alert" 全部在位**：Dashboard :320（会话失效错误条，`!stateLoading && stateErr && isSessionError` 守卫）+ Login :182（主表单错误条）+ Login :273（激活错误条）——grep 实证恰 3 处，零增零减。
- **同款 ring 令牌在位**：Tabs.tsx:29 `focus-visible:ring-2 focus-visible:ring-[var(--cyan)]` 与 Button :42 同 token；Admin :583 switch `focus-visible:ring-2 focus-visible:ring-white/60`；Login :173 密码切换同款。
- **其他无障碍基线与历轮一致**：三模态（Login :223 / Admin :213 / Select :1204）role=dialog/aria-modal/aria-labelledby/Esc/autoFocus 全部在位；CollapseSection aria-expanded/aria-controls + useId（Dashboard :67-88）在位；Admin switch role=switch/aria-checked + aria-label 在位；Select 搜索 aria-label :870 在位。
- **结论：无障碍纵深延续零缺陷，108-01 修复面完整闭合。**

### 5. 新契约角度——登录/激活/会话状态机纵深（本轮自选）
- **四条会话清理链对称性核对**（App.tsx）：主动 logout :112-132 / 401 吊销 onUnauthorized :188-231 / 删账号 onDeleted :160-178 / 管理员退出 onBackToStudent :306-329——四条链均走「快照→改→落盘→setState」同步三连（无 updater 副作用）、均同步清理管理令牌标记（setAdminToken("")+saveAdminToken("")）、均重置 targetAccount/page 视图态，对称完整零缺链。
- **管理令牌判定链**：adminToken 仅由「登录响应带 adminName」写入（login :98-99）；`isCurrentAdminSession` = `adminToken !== "" && sessions[adminName] === adminToken`（adminAuth.ts:11）——撞名学生普通会话 token 永不匹配，刷新恢复判据唯一且正确；TDD 断言 6 项全绿。
- **401 双通道广播（client.ts）**：HTTP 401 在 r.json() 前置广播（防网关非 JSON 体 :64-69）+ body code 401 非 401 状态码分支单次广播（:75-96），`r.status===401` 前置已广播则跳过 body 重复——同响应单发不风暴；事件 detail 恒带 session（真实主体）+ account（展示线索），归属判定以 session 反查（App :196-198）。unauthorized-check 5 断言全绿。
- **激活票据链（Login.tsx）**：pendingTicket 随 1001 响应 data.ticket 保存并随激活请求回传（:54/:78）；清票三路径（成功 :80 / 过期/已用 :88 / 非过期失败 :95）；Esc :232-238 与取消按钮 :301-306 同逻辑清票。契约 14（票据防穷举）前端侧完整。
- **onUnauthorized 依赖数组**：effect 闭包捕获 current（:190 兜底）但依赖仅 [adminName, inAdmin, adminToken]（eslint-disable 在册）——detail.session 恒存在（api() 恒传），current 兜底几乎不触发；量级低于立条线（见备注 3）。
- **结论：会话状态机纵深零缺陷（四条链对称 + 令牌标记全路径清除 + 401 双通道 + 票据链完整），无新立条项。**

### 6. 格式卫生快扫
- **相邻 JSX 同行/缩进级差半程态零复发**：通读 Select/Dashboard/Admin/Login 全部 JSX，`/>` 后换行、缩进级差、`<>...</>` 空态均整洁；grep 无 `/>\s*<[A-Za-z]` 同行拼接形态；Dashboard :749 `flex flex-col">` 后 `</span>` 为正常嵌套（两层 flex-col 移动页脚栏，结构与 R110 一致）。
- **结论：R67-69 同类教训零复发。**

---

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十七轮闭合，见必查 1。下轮常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十九轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单 10 处无 ring 面零增零减（含 106-01 转闭合后并入的 7 个带文本按钮，651/661 优先修复面在册）。维持「续」。
- **OBSERVE-106-01**：R110 已转闭合，本轮 7 处按钮作为家族册成员复核在位，不再单列。
- **OBSERVE-108-01（role=alert）**：R109 已转闭合，本轮 3 处仍在位，不再报。
- **OBSERVE-88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：main.tsx 裸 createRoot 无 ErrorBoundary / useTickingCountdown 整秒 setNow 如实口径 / Login 清票三路径 / Select :247 首帧短路 / 窗口关闭空态双兜底 / Toast Close 无 aria-label——全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：三模态 role=dialog/aria-modal/aria-labelledby/Esc/autoFocus 全部在位，无焦点陷阱/无关闭后焦点恢复历轮口径不变，指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log --oneline -5 -- web/` | ✅ 1351fa4 头名实物（R108 最后 web 提交），R110/R111 前端零代码提交漂移 |
| `git log --oneline -15 -- web/` | ✅ 与 R110 记录逐项一致 |
| `shouldDeferSave(` 全仓 grep | ✅ 恰 4 消费点（:509/:597/:605/:699）+ targetGuard.ts:64 定义，无第五消费处 |
| `echoedRef.current` 全仓 grep | ✅ 运行时读点恰 6 处（:229/:319/:509/:597/:605/:699），无第五运行时读点 |
| `<button` 全仓 grep | ✅ 17 处裸 button，残余面清单与 R110 逐项一致零增零减 |
| `focus-visible` 全仓 grep | ✅ 恰 4 处令牌归一（Button :42 / Tabs :29 / Login :173 / Admin :583） |
| `role="alert"` | ✅ 恰 3 处（Dashboard :320 / Login :182/:273，108-01 面在位） |
| 契约 20 残留扫描（轮次标签族/Bxx/Mxx/Rxx 锚点） | ✅ 零命中（含 106-01/107-01 注释为 OBSERVE 文档化编号，非轮次前缀标签，历轮口径） |
| `npm run build`（web/，tsc -b 真校验） | ✅ tsc -b && vite build 全过，built in 686ms，index-1KHlpqcc.css 41.72 kB / index-Bm7TtkV4.js 420.77 kB——产物哈希与 R110 逐字节一致 |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 断言全绿（18 断言） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 断言全绿（6 断言） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 断言全绿（5 断言） |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致，A/B/C 三族 90+ 断言） |
| `git status --short --branch` | ✅ `## master`（clean，build 产物 dist 亦无漂移） |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + git log/status + npm run build + 四守护脚本）；**期间做过两笔对 Dashboard.tsx 的模拟编辑已立即原样回滚（git status 实证 clean），仓库零残留**，唯一写入为本报告。
- 实测证据：四处消费点/读点 grep 全量、`<button`/`focus-visible`/`role="alert"`/ARIA 族全量 grep、契约 20 扫描、tsc/build 退出码与产物哈希（与 R110 逐字节一致的双重零改动证据）、四守护脚本断言、git log/status。走读推断：OBSERVE-93-01 残余面缺口维度评估、会话状态机四条链对称性比对，全部基于 grep 实证的代码事实。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR/新增 OBSERVE；M-1 第四十七轮闭合；F93-01 第十九轮通过；OBSERVE-93-01 残余面零增零减维持「续」；连续第五十七轮无严重级发现。
