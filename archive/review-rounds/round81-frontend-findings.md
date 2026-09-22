# Round 81 前端只读审查报告

基线：commit eca2ebd（R80 双 findings + 收尾总结，R80 收官 HEAD）。本轮为 **R81 前端只读审查 + M-1 延续管理（第十七轮）**，重点为 **R80 guardBlockedRef 移除（commit 9b32c65）的回归复核**。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 三组守护脚本。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑），工作区基线 HEAD=eca2ebd、`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**R80 guardBlockedRef 移除（commit 9b32c65）六条逐项核证全部成立：①`guardBlockedRef` 全仓零残留（grep 实测，声明/十处置位/入口清标记/注释族全删）；②终局 toast 六判据逐条推演闭环，原 OBSERVE-80-01 的"同一次 handleBack 内先守卫后真实失败被短路"路径已消除——守卫命中即 `return`、不置任何标记，真实失败信号（dirtyRef）独立表达，绝无跨轮压制；③dirtyRef 语义收敛实测成立——全量 grep 仅三处置位（saveNow catch :453 + 两处飞行中补发标记 :560/:739），十处守卫纯 return 零误置；④守卫分支改纯 return 后 handleBack 入口清理块与相关注释已同步清理，无死代码/无用变量残留（tsc 无 unused 报错实证）；⑤状态机解析：守卫命中时第一轮循环即 break 的路径与"next 改动继续下一轮"路径均正确收敛，三条注释仅一处沿用了"置脏"二字但语义指向"守卫拦截保留脏块"，与代码行为不冲突；⑥pendingUnsaved 只查三信号，`rev>0` 时守卫场景恒 false、真实失败恒 true，误报与漏报双闭合。M-1 延续管理第十七轮：shouldDeferSave 三消费点（防抖 :696 / flush :506 / handleBack 判定 :594 + while :602）全传 echoedRef.current 第三参，置位三路径 + 首帧不置位边界完整，target-guard 18/18 全绿。六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归，setSelected 七调用点无第三来源，三组断言全部实测通过。Admin StatsTab data-first / LogsTab isLoading / 契约 20（R79 三修）复核无回潮。XSS 危险模式与轮次标签/行号引用族全仓零残留，localStorage 六处读写全 try/catch 降级，视觉护栏 audit.mjs 全绿。本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（注释口径小修）。构建验证全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs）。**

---

## 一、R80 修复正确性复核（commit 9b32c65）

### 1.1 guardBlockedRef 移除完整性（grep 全量清点）

grep `guardBlockedRef|guard_blocked|guardBlocked` 全 web 目录（含 src/scripts）**零命中**。commit diff 对照确认删除完整：

| 删除项 | 证据 |
|---|---|
| 声明 `const guardBlockedRef = useRef(false)` | diff 中 :409-415 整段删除 |
| 十处守卫置位（flush 五处 + 防抖五处） | diff 全部改纯 `return`，无替代标记 |
| pendingUnsaved 内短路 `if (guardBlockedRef.current) return false` | diff 改为单表达式三信号 |
| handleBack 入口清标记 `guardBlockedRef.current = false` | diff :582-588 整段删除（含注释） |
| 相关注释族（"守卫命中不置 dirtyRef"语义描述） | diff 中 :409-415/:640-642 注释重写，无残留 |

结论：完整闭合，无任何残留引用。✅

### 1.2 pendingUnsaved 精简为三信号后终局 toast 判据逐条推演

现实现（Select.tsx:416-417）：

```
const pendingUnsaved = () =>
  dirtyRef.current || savingRef.current || retryState.current.timer !== null
```

终局判定（:644）`if (revRef.current > 0 && pendingUnsaved())`。六条判据逐条闭环：

1. **守卫拦下 + 返回 → 不弹（正确）**：守卫命中即 `return`（flush :507/:513/:531/:550/:555、防抖 :697/:707/:714/:729/:734），不调 saveNow、不置 dirtyRef。若守卫命中发生在 handleBack 循环第 1 轮 flush，则 dirtyRef=false 且 savingRef=false → 等帧复查 revRef 无变化 → break → 终局 `dirtyRef||savingRef||timer` 全 false → 不弹。守卫是安全拦截（目标安全、后端旧目标未被抹除），不弹"目标保存失败"符合语义。✅
2. **真实保存失败 → 弹（正确）**：saveNow catch :453 置 dirtyRef=true + scheduleRetry 挂退避 timer。flush 后 dirtyRef=true → 不满足 :619 静止 → pendingSaving 等待（21s 兜底）→ 超时后循环结束 dirtyRef 仍 true → 终局弹"目标保存失败"。与 R80 前行为一致。✅
3. **守卫后飞行成功 → 不弹（正确）**：守卫命中不置 dirtyRef；期间任何真实保存成功会经 saveNow finally :460-462 清 dirtyRef（若曾置位）+ resetRetry 清 timer → 终局三信号全 false → 不弹。✅
4. **守卫后恢复成功返回 → 不弹（正确）**：入口无标记残留问题（标记已删除）；发布恢复后 flush 守卫通过 → saveNow 成功 → dirtyRef false → 不弹。✅
5. **守卫后恢复失败返回 → 弹（正确）**：发布恢复后 flush 守卫通过 → saveNow 网络失败 → dirtyRef=true → pendingSaving 等待超时 → 终局弹。✅
6. **同一次 handleBack 内先守卫后真实失败（原 OBSERVE-80-01）→ 弹（已修复）**：第 1 轮 flush 守卫命中（纯 return，零标记）→ 等帧期间用户新点（rev 变化 → :621 continue 进入下一轮）→ 第 2 轮 flush 守卫通过（发布恢复）→ saveNow 失败置 dirtyRef=true → pendingSaving 等待 → 循环结束 dirtyRef 仍 true → **终局弹**。原"guardBlockedRef 残留标记短路终局 toast"的路径物理消除——守卫命中不再留下任何可残留的标记，真实失败信号只由 dirtyRef 独立表达。OBSERVE-80-01 闭合。✅

**语义收敛确认**：终局 toast 现在严格等价于"rev>0 且（真实保存失败 || 在飞 PUT || 退避排队中）"三真信号的析取；守卫拦截场景三信号恒 false（守卫不置 dirtyRef、不发起 saveNow、不挂 timer），天然不弹。R79 拆分引入的"守卫短路冗余"与"会话内一次命中永久压制"双重缺陷一并消除，移除是正确且必要的简化。

### 1.3 dirtyRef 语义收敛核证（grep 全量清点）

`dirtyRef.current = true` 仅三处置位（grep 实测）：

| 行 | 语义 |
|---|---|
| :453 | saveNow catch 真实保存失败置脏（唯一真实失败源头） |
| :560 | flushTargets 保存进行中标记脏（飞行中 PUT 完成后补发） |
| :739 | 防抖回调保存进行中标记脏（同语义） |

消费点：:460-461（finally 补发时清）、:619（handleBack 静止判据）、:417（pendingUnsaved）。**十处守卫分支全部纯 return、零误置**——grep `dirtyRef.current = true` 与十处守卫行逐一交叉核对，守卫行内无任何 dirtyRef 赋值。守卫路径与"改动未落库"信号完全解耦。✅

### 1.4 守卫改纯 return 后死代码/无用变量/注释清理核证

- **handleBack 入口清理块**：diff :582-588 整段删除（含 "返回前先清守卫拦截标记" 注释），入口现无任何标记清理——守卫命中不产生标记，无需清理。✅
- **注释族同步**：:412-415（pendingUnsaved 注释）与 :640-642（终局提示注释）均已重写为"守卫命中不置 dirtyRef"语义，无 "guardBlockedRef" 字样残留。✅
- **无死代码**：tsc -b（`noUnusedLocals:true`）EXIT 0——若残留未使用变量/函数必报错，实测全绿。✅
- **注释口径微瑕（OBSERVE-81-01）**：见四章。

### 1.5 边界语义确认（词法陷阱排查）

- **守卫命中 + 第一轮即 break 的收敛性**：守卫命中时 `!dirtyRef.current && !savingRef.current`（:619）成立 → 等帧复查 revRef——若无新改动 break（正确：守卫拦截是等自愈语义，目标安全）；若期间有新改动 continue 下一轮 flush（下一轮守卫状态重估）。两路径均正确收敛。✅
- **resetRetry 不清任何守卫标记**：守卫不再有标记，resetRetry 只清退避 timer/attempt，语义正交无遗漏。✅
- **账号复位（:196-202）**：不涉及守卫标记（已删除），无需清理。✅

---

## 二、M-1 延续管理（第十七轮）

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（逐字符比对，零回潮）：
  - 防抖回调 :696 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :506 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :594 + while :602 同参 `hasSelectedNow()`（:576-577 读 selectedRef 计算）
- **echoedRef 置位三路径 + 首帧不置位边界完整**：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，另含 :287-294 回显内 stale 兜底清理 + toast）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；防抖 effect 依赖 :752 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`——stateData/echoDone/hasPublishes 三路解锁闭合，F42-M1 自愈链零回潮。
- 第十七轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

---

## 三、六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true`/`echoed→false`/`courses 非空 && hasSelected→true`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :696 stateDataRef + effect 依赖 :752 | 判据只读 stateDataRef 非 echoedRef；/state 到达触发 effect 重跑自愈 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用（:42）；依赖含 selected 合并后重跑；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :696-742 五处 + flush :506-562 五处 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 `rev > 0 && !anyHas` 不合并、:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :729 / flush :550 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回归 |

**setSelected 调用点清点**（grep 实测七处，与 R80 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。

**防抖 effect 细节重走读**：resetRetry 放 effect 顶部（:665，新改动立即清退避 timer）；setTimeout 回调闭包读 selected 为 effect 创建时快照、publishesRef/stateDataRef 消费时刻最新；saveNow 四门卸载保护（:429/:440/:445/:457）延续；handleBack 三轮循环 :608-639 + pendingSaving :630 覆盖退避 timer 排队，21s 兜底绝不无限挂起——全部对照 R80 基线零回潮。

---

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-81-01 — 守卫命中注释沿用"置脏"二字，与 R80"守卫不置任何标记"的新语义口径不一致（纯注释口径，无行为影响）**

- 位置：Select.tsx:501（flushTargets 回显未完成守卫注释）"置脏跳过、不 PUT，脏块保留（dirtyRef=true）"——**注释声称 dirtyRef=true 与 :507 行内注释 "守卫不置 dirtyRef" 自相矛盾**；实际代码 :506-507 纯 return、不置 dirtyRef，:501 注释是 R79 时代"守卫置脏跳过"语义的残留。
- 触发场景推演：纯注释不一致——:501 的 `（dirtyRef=true）` 会让后续维护者误以为守卫命中会把脏块标成"真实保存失败"（进而误判终局 toast 语义），与 R80 提交的"守卫不置 dirtyRef"核心设计意图相悖。无任何运行时影响（代码行为正确）。
- 修复建议：把 :501 注释改为 "守卫拦截、不 PUT，脏块保留在内存 selected（等回显/发布恢复自愈；守卫不置 dirtyRef——终局绝不误报保存失败）"，与 :507 行内注释及 :412-415 顶部注释口径统一。同类检查：:215/:282/:685/:690/:694 等注释中的"置脏"均指"保留脏块不落库"语义、不涉 dirtyRef 赋值，不矛盾；仅 :501 一处把"置脏"与 `dirtyRef=true` 显式绑定，是唯一口径冲突点。
- 严重度论证：纯注释问题，零行为影响；但因 R80 的核心语义改动恰好围绕 dirtyRef 收敛，注释口径残留对后续维护的误导成本相对突出。维持 OBSERVE，不构成 MINOR。

**OBSERVE-78-01（延续）**：handleBack 终局 toast 与退出前 flush 失败红条同 title 合并的文案突变窗口（超时路径与即时返回路径同源），去重已防轰炸，维持「续」。

**OBSERVE-77-01（延续）**：Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存——普通学生场景语义等价零影响。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 调用点清点（本轮 grep 实测七处与 R80 基线一致）——均维持「续」。

### 可疑待核

无新增。历轮"可疑-1"（终局 toast 与 onDone 卸载竞态）物理不可达论证在 R80 改判据后不受影响（末轮 flush 触发的 saveNow 置位 savingRef 会被 :630 pendingSaving 等待捕获收敛），结论维持。

---

## 五、已核无缺陷清单

- R80 修复六条全部核证成立：①guardBlockedRef 全仓零残留（grep 实测）；②终局 toast 六判据逐条推演闭环，OBSERVE-80-01 该弹没弹路径物理消除；③dirtyRef 仅三处置位（saveNow catch + 两处飞行中补发标记）语义收敛；④守卫纯 return 无死代码/无用变量/注释残留（tsc noUnusedLocals 实证）；⑤状态机收敛路径（守卫命中 break / continue 下一轮）双分支正确；⑥pendingUnsaved 三信号与 rev>0 组合双闭合（守卫场景恒不弹、真实失败恒弹）。
- M-1 延续管理（第十七轮）：shouldDeferSave 三消费点（防抖/flush/handleBack 判定+while）全传 echoedRef.current 第三参、echoedRef 置位三路径 + 首帧不置位边界完整、target-guard 断言 18/18 全绿。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + 消费时刻双闸）：逐条判据与注释对应，stateData/echoDone/hasPublishes 三路解锁闭合，setSelected 七调用点无第三来源，零回潮。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：行号引用族（见N行/第N行/行N的）+ 轮次前缀标签族（R\d+（/第N轮/roundN））全仓零残留；XSS 危险模式（dangerouslySetInnerHTML/innerHTML/eval(/new Function/document.write/insertAdjacentHTML）零残留。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、撞名学生管理态（isCurrentAdminSession 六判定）、aria 无障碍、btn_type 三向、max_count=0 四处同源、倒计时打底——复跑全量通过，零差异化、零回潮。
- localStorage 读写全 try/catch 降级（App.tsx 六处全带降级）；视觉护栏 audit.mjs 全绿（画布双宽度/玻璃工具类/实心黑洞清零）。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 全部通过（视觉护栏） |
| 全仓残留扫描（行号族 / 轮次标签族 / XSS 危险模式） | ✅ 零命中 |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第十七轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01/77-02**：同 key 不同 URL 缓存身份 / 窗口关闭无目标管理入口——维持「续」。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=eca2ebd），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round81-frontend-findings.md）。
- 走读推断与实测冲突处理：OBSERVE-81-01（:501 注释口径）经 grep 全量比对（`dirtyRef.current = true` 三处置位 + "置脏"字样十处）后确认——仅 :501 一处把"置脏"与 `dirtyRef=true` 显式绑定，其余"置脏"均为"保留脏块不落库"语义；代码行为正确，纯注释口径冲突，定级 OBSERVE。R80 六判据推演以实测代码（:416-417/:506-562/:608-639/:644-650）为准逐条闭环。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（注释口径小修）+ 延续观察；连续第二十七轮无严重级发现。
