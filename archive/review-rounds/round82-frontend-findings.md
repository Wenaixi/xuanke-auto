# Round 82 前端只读审查报告

基线：commit be64bc8（R81 双 findings + 收尾总结，R81 收官 HEAD）。本轮为 **R82 前端只读审查 + M-1 延续管理（第十八轮）**，重点为 **R81 注释口径修复（commit 6b574ff）的回归复核**。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 四组守护脚本。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑），工作区基线 HEAD=be64bc8、`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**R81 注释口径修复（commit 6b574ff）核证成立：Select.tsx:501 注释已从「置脏跳过、不 PUT，脏块保留（dirtyRef=true）」改为「置脏跳过、不 PUT，脏块保留在内存 selected（守卫不置 dirtyRef——终局绝不误报保存失败）」，与 :507 行内注释、:412-415 顶部 pendingUnsaved 注释三处口径统一；全仓「置脏字样」清点确认仅此一处曾把「置脏」与 `dirtyRef=true` 显式绑定，其余全部为「保留脏块不落库」语义、不涉 dirtyRef 赋值。M-1 延续管理第十八轮：shouldDeferSave 三消费点（防抖 :697 / flush :507 / handleBack 判定 :595 + while :603）全传 echoedRef.current 第三参、置位三路径 + 首帧不置位边界完整、target-guard 18/18 全绿。六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归，setSelected 七调用点无第三来源，三组断言全部实测通过。R79/R80 修复持续复核：Admin StatsTab data-first（:757-792）/ LogsTab isLoading（:922-944）全字段逐行核对无回潮，guardBlockedRef 全仓零残留（grep 实测）。契约 20 全仓扫描：轮次标签族含 32-01/32-02/33-01 行内锚点逐条核证其「工程决策溯源语义」且无 R 轮次前缀；行号族零残留；XSS 危险模式零残留；localStorage 六处读写全 try/catch 降级；视觉护栏 audit.mjs 全绿。本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（defensive 传播）。构建验证全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs）。**

---

## 一、R81 修复正确性复核（commit 6b574ff）

### 1.1 注释口径统一核证（OBSERVE-81-01 修复）

commit diff 确认：Select.tsx:501 两行注释从 `（dirtyRef=true）` 改为 `（守卫不置 dirtyRef——终局绝不误报保存失败）`。现三处口径逐字符核证：

| 位置 | 注释内容 | 口径 |
|---|---|---|
| :412-415（pendingUnsaved 顶部） | 「守卫命中不置 dirtyRef——终局 toast 只对『真实保存失败』（dirtyRef，仅 saveNow catch 与飞行中标记补发会置）触发」 | 守卫不置 dirtyRef ✅ |
| :501-502（flush 回显守卫注释） | 「脏块保留在内存 selected（守卫不置 dirtyRef——终局绝不误报保存失败）」 | 守卫不置 dirtyRef ✅ |
| :507 行内 return 注释 | 「守卫不置 dirtyRef——终局绝不误报保存失败」 | 守卫不置 dirtyRef ✅ |

**全仓「置脏」字样清点**（grep 全量，Select.tsx + targetGuard.ts + target-guard-check.ts 共 14 处含「置脏」）：逐一核对——:215/:236/:282（回显注释，「一路置脏跳过」指守卫拦截语义）、:501/:508/:514/:518/:552/:555/:557/:686/:691/:695/:698/:708/:731/:736（全部为「保留脏块不落库」语义）、targetGuard.ts:7/:22/:47/:51/:56、target-guard-check.ts:5/:7/:81/:83（脚本断言描述）。**仅 :501 曾把「置脏」与 `dirtyRef=true` 显式绑定，现已剥离**；其余「置脏」均指「守卫拦截保留脏块」，与 R80「守卫不置 dirtyRef」新语义不冲突。`dirtyRef.current = true` 置位仍仅三处（:453 保存失败 / :561 飞行中补发 / :740 防抖补发），与注释承诺完全一致。OBSERVE-81-01 闭合。

### 1.2 契约 20 轮次标签族复核（新增锚点逐条核证）

全仓扫描发现 Select.tsx 存在 4 处「数字横线」锚点：:172/:610/:615/:963 的 `32-01`、`33-01`、`32-02` 注释（「32-01：flush 已消费本轮最新 ref 快照」「33-01：首帧未到等满 5s 后」「32-02：max_count=0」）。逐条核证其来源：

- R32/R33 归档（review-round32.md:30/:44、review-round33.md:43）确认：**Select-32-01**（handleBack 等待循环闭包陈旧→selectedRef/revRef 镜像）、**Select-32-02**（max_count=0 筛选/isFull 两处缺口）、**Select-33-01**（返回 flush 在回显合并前整包覆盖→5s 回显等待）均为真实历史缺陷编号，代码注释中的锚点是「工程决策溯源」而非「轮次标签」。
- 契约 20 禁止的是「轮次前缀标签（X-XX（第 N 轮））」形态——上述锚点均**无 R 轮次前缀、无「第 N 轮」后缀、无 commit 引用**，属决策历史溯源注释，与「代码注释只写为什么/契约/陷阱本身，历史残留发现即剥离」的排除项（B35-01 归档明文记录「B19-01/B21-01/32-01 长注释块为精简『open 单快照对三条判据统一』注释，决策历史留在 CLAUDE.md」）同族。
- 全仓 `R[0-9]{2}`（大写 R + 双数字轮次）、`round[0-9]{2}`、`第N轮` 形态**零命中**（grep 实测）；`B43-04` 仅存在于 web/scripts/admin-auth-check.ts:4 注释（缺陷背景说明，指 R43 后端「管理员撞名」边界修复，非轮次前缀标签）。

### 1.3 行号族 / XSS 危险模式 / localStorage 降级复核

- 行号引用族（见N行/第N行/行N的/（N行））零命中（grep 实测）。
- XSS 危险模式（dangerouslySetInnerHTML/innerHTML 赋值/eval(/new Function/document.write/insertAdjacentHTML）零命中。
- localStorage 读写：App.tsx 六处（:20/:28/:39/:46/:57/:64）全带 try/catch 静默降级，注释与实现一致（:17/:33/:51 降级语义说明）。
- TODO/FIXME 扫描：仅两处命中均为文案「XK-XXXX-XXXX-XXXX」（Login.tsx:265 placeholder）与「注释承诺从未实现」（Select.tsx:657 为「hasPublishes 修复已实现」的注释措辞，非未实现代码），无 stub/占位代码。

---

## 二、M-1 延续管理（第十八轮）

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（逐字符比对，零回潮）：
  - 防抖回调 :697 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :507 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :595 + while :603 同参 `hasSelectedNow()`（:577-578 读 selectedRef 计算）
- **echoedRef 置位三路径 + 首帧不置位边界完整**：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，另含 :287-294 回显内 stale 兜底清理 + toast）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；防抖 effect 依赖 :753 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`——stateData/echoDone/hasPublishes 三路解锁闭合，F42-M1 自愈链零回潮。
- target-guard-check.ts 纯函数断言 18/18 实测全绿（含 echoed 第三参 2 条：稳态编辑不闷死 / 首帧未到+已回显仍推迟）。
- 第十八轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

---

## 三、六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true`/`echoed→false`/`courses 非空 && hasSelected→true`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :697 stateDataRef + effect 依赖 :753 | 判据只读 stateDataRef 非 echoedRef；/state 到达触发 effect 重跑自愈 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖含 selected 合并后重跑；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :697-743 五处 + flush :507-562 五处 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 `rev > 0 && !anyHas` 不合并、:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :730 / flush :551 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回归 |

**setSelected 调用点清点**（grep 实测七处，与 R81 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。

**防抖 effect 细节重走读**：resetRetry 放 effect 顶部（:666，新改动立即清退避 timer）；setTimeout 回调闭包读 selected 为 effect 创建时快照、publishesRef/stateDataRef 消费时刻最新；saveNow 四门卸载保护（:429/:440/:445/:457）延续；handleBack 三轮循环 :609-639 + pendingSaving :631 覆盖退避 timer 排队，21s 兜底绝不无限挂起——全部对照 R81 基线零回潮。

---

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-82-01 — Admin StatsTab 的「加载中」兜底渲染与五 Tab 对称、但首帧失败时三种失败态文案结构不同步（极弱：纯文案一致性问题，行为无缺陷）**

- 位置：Admin.tsx:757-792（StatsTab）、:445-447/:823-824/:922-924（其余 Tab）、:687-699（ConfigTab）。
- 触发场景推演：StatsTab 数据失败时显「运行状态加载失败（网络异常或服务端不可达）」，CodesTab/AccountsTab/LogsTab 同款失败态、ConfigTab 为「配置加载失败（网络异常或服务端不可达）」——文案结构一致；仅 StatsTab 首帧失败态与其 `refetch()` 重试按钮之间、空态兜底「加载中...」的语义在「首帧在途」与「首帧失败后 data 缺席」两态间切换，与 CodesTab 的 `isLoading → isError` 次序同构、无差异化缺陷。ConfigTab 的「加载提示 `!loaded && !isError`」语义不同属表单回填，非缺陷（R79 归档已明确定性）。**纯一致性观察，无行为风险，维持 OBSERVE**。
- 修复建议：可不修；如需完全对齐，可把 StatsTab 兜底文案统一为与同族三 Tab 完全一致的「加载中...」，纯文案微调非必须。
- 严重度论证：无功能/数据/安全影响，仅文案一致性；低于升级阈值，维持 OBSERVE。

**OBSERVE-78-01（延续）**：handleBack 终局 toast 与退出前 flush 失败红条同 title 合并的文案突变窗口（超时路径与即时返回路径同源），去重已防轰炸，维持「续」。

**OBSERVE-77-01（延续）**：Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存——普通学生场景语义等价零影响。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 调用点清点（本轮 grep 实测七处与 R81 基线一致）——均维持「续」。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证在 R80 改判据后不受影响（末轮 flush 触发的 saveNow 置位 savingRef 会被 :631 pendingSaving 等待捕获收敛），结论维持。

---

## 五、已核无缺陷清单

- R81 修复核证：Select.tsx:501 注释已改「守卫不置 dirtyRef」口径，与 :507 行内注释、:412-415 顶部注释三处统一；全仓「置脏」字样 14 处逐一核对，仅 :501 曾把「置脏」与 `dirtyRef=true` 显式绑定现已剥离；`dirtyRef.current = true` 置位仍仅三处（:453/:561/:740）与注释承诺一致。OBSERVE-81-01 闭合。
- M-1 延续管理（第十八轮）：shouldDeferSave 三消费点（防抖/flush/handleBack 判定+while）全传 echoedRef.current 第三参、echoedRef 置位三路径 + 首帧不置位边界完整、target-guard 断言 18/18 全绿。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + 消费时刻双闸）：逐条判据与注释对应，stateData/echoDone/hasPublishes 三路解锁闭合，setSelected 七调用点无第三来源，零回潮。
- R79/R80 修复持续复核：Admin StatsTab data-first 次序（:757-792，`s ? rows : isError ? errorCard : 加载中`）、LogsTab isLoading 短路（:922-944，`isLoading → logs&&len>0 → isError → 空态`）、guardBlockedRef 全仓零残留（grep 实测）——无回潮。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：轮次前缀标签族（R 轮次 + 第 N 轮形态）全仓零残留；行号引用族零残留；XSS 危险模式零残留；localStorage 六处读写全 try/catch 降级；视觉护栏 audit.mjs 全绿（画布双宽度/玻璃工具类/实心黑洞清零）。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、撞名学生管理态（isCurrentAdminSession 六判定）、aria 无障碍、btn_type 三向、max_count=0 四处同源、倒计时打底——复跑全量通过，零差异化、零回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 全部通过（视觉护栏） |
| 全仓残留扫描（轮次标签族 / 行号族 / XSS 危险模式 / TODO-stub） | ✅ 零命中（32-01/33-01 锚点经 R32/R33 归档溯源属工程决策编号） |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第十八轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01/77-02**：同 key 不同 URL 缓存身份 / 窗口关闭无目标管理入口——维持「续」。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=be64bc8），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round82-frontend-findings.md）。
- 走读推断与实测冲突处理：契约 20 扫描发现 32-01/32-02/33-01 数字横线锚点后，先按「轮次标签残留」假设 grep 全仓 + 追 R32/R33 归档原文（review-round32.md:30/:44、review-round33.md:43 确认 Select-32-01/32-02/33-01 为真实历史缺陷编号），后定锚为「工程决策溯源注释」而非轮次前缀标签；同族 B43-04 仅存在于守护脚本注释（缺陷背景说明），不构成轮次标签。以归档溯源实测为准。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（StatsTab 失败文案一致性）+ 延续观察；连续第二十八轮无严重级发现。
