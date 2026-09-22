# Round 83 前端只读审查报告

基线：commit dca1281（R82 双 findings + 收尾总结，R82 收官 HEAD）。本轮为 **R83 前端只读审查 + M-1 延续管理（第十九轮）**，核心为**保存链换方向扫查**（防抖闭包快照消费正确性 / lastJson 去重跨会话 / handleBack 21s 兜底超时路径 / echo effect 轮询引用重建开销 / hasPublishes 布尔发布重建交错）+ M-1 延续第十九轮 + R80/R81/R82 修复持续复核。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 四组守护脚本。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑），`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（echoedRef 未置位稳态组合的潜在陷阱）+ 延续观察。M-1 延续管理第十九轮闭合：shouldDeferSave 三消费点（防抖 :697 / flush :507 / handleBack 判定 :595 + while :603）全传 echoedRef.current 第三参、置位三路径 + 首帧不置位边界完整、target-guard 18/18 全绿。六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归，setSelected 七调用点（:158/:198/:250/:288/:320/:351/:361）无第三来源。保存链新视角五项扫查（换方向）全部收敛无新增缺陷：防抖闭包 selected 快照经「rev 变化 → effect 重跑 → 旧 timer 清理」机制保证消费时刻恒为最新渲染代际；lastJson 去重经组件实例级 useRef 天然隔离跨会话（重进 Select 全新实例 lastJson=""，回显不触发保存）；handleBack 21s 兜底在超时路径由 api 20s abort 兜底保证「PUT 至迟 20s 落地失败」→ pendingSaving 循环内必捕获 dirtyRef → 终局绝不漏报；echo effect 轮询重跑被首行 `echoedRef.current` 短路为零成本；hasPublishes 布尔在发布重建交错下按设计重跑防抖落库。R80/R81/R82 修复持续复核：guardBlockedRef 全仓零残留（含 backend）、注释口径三处统一、Admin StatsTab data-first（:757-792）、LogsTab isLoading 短路（:922-944）全部无回潮。契约 20 全仓扫描：轮次前缀标签族零命中（package-lock 内 R+双数字均为 base64 校验和字符、src/scripts 零命中）、行号族零命中、XSS 危险模式零命中、localStorage 六处读写全 try/catch 降级、视觉护栏 audit.mjs 全绿。构建验证全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 全项通过）。连续第二十九轮无严重级发现。**

---

## 一、M-1 延续管理（第十九轮）

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（逐字符比对，零回潮）：
  - 防抖回调 :697 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :507 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :595 + while :603 同参 `hasSelectedNow()`（:577-578 读 selectedRef 计算）
- **echoedRef 置位三路径 + 首帧不置位边界完整**（grep 全量读写点清点，与 R82 基线一致）：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，另含 :287-294 回显内 stale 兜底清理 + toast）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位
  - 读点仅三处：:227（回显 effect 首行短路）、:317（独立清理 effect 守卫）、三消费点（:507/:595/:603/:697）
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；防抖 effect 依赖 :753 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`——stateData/echoDone/hasPublishes 三路解锁闭合，F42-M1 自愈链零回潮。
- target-guard-check.ts 纯函数断言 18/18 实测全绿（含 echoed 第三参 2 条：稳态编辑不闷死 / 首帧未到+已回显仍推迟）。
- 第十九轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true`/`echoed→false`/`courses 非空 && hasSelected→true`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :697 stateDataRef + effect 依赖 :753 | 判据只读 stateDataRef 非 echoedRef；/state 到达触发 effect 重跑自愈 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :697-743 五处 + flush :507-562 五处 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 `rev > 0 && !anyHas` 不合并、:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :730 / flush :551 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回归 |

**setSelected 调用点清点**（grep 实测七处，与 R82 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。

**防抖 effect 细节重走读**：resetRetry 放 effect 顶部（:666）；setTimeout 回调闭包读 selected 为 effect 创建时快照、publishesRef/stateDataRef 消费时刻最新；saveNow 四门卸载保护（:429/:440/:445/:457）延续；handleBack 三轮循环 :609-639 + pendingSaving :631 覆盖退避 timer 排队——全部对照 R82 基线零回潮。

## 三、保存链新视角扫查（换方向，五项全收敛）

1. **防抖 effect 闭包 selected 快照在轮询/回显交错场景的消费正确性**：回调闭包捕获的 `selected`/`selectedCount` 是 effect 创建时快照，但消费正确性由「rev 依赖」闭环保证——400ms 窗口内任何用户改动（pick/清空）都触发 setRev → effect cleanup 清旧 timer → 重跑挂新 timer，新闭包捕获新 selected。不存在「旧闭包 selected 被消费而 rev 已是新值」的代际错配窗口。轮询刷新（stateData/publishes 变化）重跑 effect 只换守卫所需 ref、不覆盖 selected 快照（轮询不产生用户改动）。**实测推演无缺陷**。
2. **saveNow lastJson 去重在「保存成功后返回再进入」场景**：lastJson 是 useRef（组件实例级），返回控制台 → Select 卸载 → 重进时全新实例 lastJson=""。回显 effect 只 setSelected 不增 rev，防抖 effect 顶部 `rev===0` return，绝不因回显触发重复 PUT；用户新改动后才走正常保存。跨会话零泄漏。**另推演一个临界场景并确认自愈**：PUT 失败置 dirtyRef=true + scheduleRetry 后，用户把目标还原为与 lastJson 完全一致的载荷 → 防抖回调 saveNow 因 `json === lastJson` 提前 return → finally 中 `dirtyRef && attempt===0` 补发路径再次 saveNow（仍命中 lastJson 短路）→ dirtyRef 被清零、无实际 PUT——内存与后端一致，终局不误报。**自愈闭环成立**。
3. **handleBack 三轮 flush 的 21s 兜底在超时路径的行为**：第 3 轮 flush 触发的 saveNow 置位 savingRef 是同步的（在首个 await 之前）→ 同一轮 `pendingSaving()` 必捕获 → 等至多 21s；api 20s 超时保证 PUT 至迟 20s abort 落地（catch → dirtyRef=true + scheduleRetry），21s 兜底必然在 20s 失败落地后继续下一轮 → 循环结束 dirtyRef 确证失败 → 终局 toast 命中。**不存在「循环退出时 PUT 仍在飞而终局漏报」的窗口**（R82「可疑-1 物理不可达」论证在本轮换方向扫查下再确认不受影响）。
4. **echo effect 依赖 `[stateData, data, rev, selected, toast]` 的轮询刷新引用重建开销**：/state 2s 轮询（窗口开放期）每次返回新引用 → echo effect 每次重跑，但首行 `if (echoedRef.current) return` 短路（回显完成后恒 true），每次仅一次函数调用 + ref 读，无 setState、无 toast、无合并逻辑执行。`data`（/electives 30s/2s 轮询）与 `toast`（useCallback [] 稳定）同量级。**开销可忽略，无缺陷**。
5. **publishes 空态 hasPublishes 布尔在发布重建交错场景的防抖 effect 重跑**：开窗瞬间 publishes 清空 → hasPublishes false → effect 重跑（rev>0 时）挂新 timer → 消费时刻「发布缺席 + 已有选中」守卫拦下不 PUT → 发布恢复 → hasPublishes true → effect 重跑 → 守卫通过 → 落库。布尔只取「非空/空」两态、发布重建来回翻转期间每次翻转都重跑并重置 400ms 窗口——属 F40 设计内行为（防抖重置等待稳定态），非缺陷；发布稳定后一次落库收敛。**无缺陷**。

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-83-01 — 「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合是潜在陷阱（当前行为无害，未来窗口关闭后加目标管理入口即触发守卫闷死）**

- 位置：Select.tsx:245（回显 effect `if (pubs.length === 0) return`）、:162/:227/:697（echoedRef 声明/守卫短路/防抖守卫消费）。
- 触发场景推演：账号有已持久化目标（/state.courses 非空）且窗口已关闭（/electives 空 publishes）时进入选课大厅 → 回显 effect 首帧 `stateData !== undefined`、courses 非空 → 进 :245 `pubs.length === 0` → return，echoedRef 恒 false（非合并、非空分支、无置位路径）。该态下 tabs.length===0 → 渲染「当前无可选课程批次」空态卡（:914-920）、无 pick 按钮、rev 恒 0、防抖保存链不触发——**当前行为完全无害**（无可编辑入口，无保存需求，handleBack flush 因 latestRev===0 直接 return）。但 echoedRef 语义上停留在「回显未完成」：若未来按 OBSERVE-77-02 开放「窗口关闭后仍可管理目标」入口（有编辑能力后），防抖守卫 `shouldDeferSave(stateData 非空且 courses 非空, hasSelected, echoed=false)` 恒 true → 所有编辑被永久闷死、无自愈信号（pubs 永空、hasPublishes 永 false），与 F42/F43 已闭合的「守卫必须有解锁路径」原则冲突。
- 修复建议：可不修（当前无触发路径）。若未来加目标管理入口，需在「courses 非空 + 无发布」态显式判定回显已完成（echoedRef 置位不依赖发布），或随入口一并重审守卫解锁。
- 严重度论证：无功能/数据/安全影响，纯前瞻性陷阱观察，低于升级阈值，维持 OBSERVE。

**OBSERVE-78-01（延续）**：handleBack 终局 toast 与退出前 flush 失败红条同 title 合并的文案突变窗口（超时路径与即时返回路径同源），去重已防轰炸，维持「续」。

**OBSERVE-77-01（延续）**：Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存——普通学生场景语义等价零影响。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界，与 OBSERVE-83-01 关联）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 调用点清点（本轮 grep 实测七处与 R82 基线一致）——均维持「续」。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证在本轮新视角 3 推演下再确认（末轮 flush 置位 savingRef 同步、pendingSaving 必捕获、api 20s abort 保证失败落地不晚于 21s 兜底），结论维持。

## 五、已核无缺陷清单

- M-1 延续管理（第十九轮）：shouldDeferSave 三消费点（防抖/flush/handleBack 判定+while）全传 echoedRef.current 第三参、echoedRef 置位三路径 + 首帧不置位边界完整、读写点全量清点零回潮、target-guard 断言 18/18 全绿。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + 消费时刻双闸）：逐条判据与注释对应，stateData/echoDone/hasPublishes 三路解锁闭合，setSelected 七调用点无第三来源，零回潮。
- 保存链新视角五项：防抖闭包快照代际正确 / lastJson 跨会话隔离 + 还原场景自愈 / 21s 兜底超时路径不产生漏报窗口 / echo effect 轮询重跑零成本短路 / hasPublishes 发布重建按设计收敛——全部推演无缺陷。
- R80/R81/R82 修复持续复核：guardBlockedRef 全仓零残留（grep 全量含 backend）、注释口径三处统一（:412-415 / :501-502 / :507）、Admin StatsTab data-first 次序（:757-792，`s ? rows : isError ? errorCard : 加载中`）、LogsTab isLoading 短路（:922-944）——无回潮。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：轮次前缀标签族（`R[0-9]{2}`/round 形态）src/scripts 零命中（package-lock 中 `R10`/`R68` 等为 base64 校验和字符，非标签）；行号引用族（见N行/第N行/行N的）零命中；32-01/32-02/33-01/B43-04 四处工程决策溯源锚点与 R32/R33/R43 归档一致、无轮次前缀形态；XSS 危险模式零命中；localStorage 六处读写全 try/catch 降级；视觉护栏 audit.mjs 全绿（画布双宽度/玻璃工具类/实心黑洞清零）。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、撞名学生管理态（isCurrentAdminSession 六判定）、aria 无障碍、btn_type 三向、max_count=0 四处同源、倒计时打底——复跑全量通过，零差异化、零回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 全部通过（视觉护栏，A/B/C 三组全项） |
| 全仓残留扫描（轮次标签族 / 行号族 / XSS 危险模式 / TODO-stub） | ✅ 零命中（仅 Login.tsx:265 激活码占位文案 XK-XXXX-XXXX-XXXX 非占位代码） |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第十九轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01/77-02**：同 key 不同 URL 缓存身份 / 窗口关闭无目标管理入口——维持「续」。
- **OBSERVE-83-01**：courses 非空 + publishes 空 + echoedRef 未置位的稳态组合为潜在陷阱（当前行为无害），与 OBSERVE-77-02 关联，未来开放「关闭后管理目标」入口时需一并处理守卫解锁。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，本轮新视角推演再确认，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=dca1281），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round83-frontend-findings.md）。
- 走读推断与实测冲突处理：新视角扫查中「handleBack 21s 兜底超时路径漏报」先按假设推演（第 3 轮 flush 触发 PUT 在飞、循环退出漏报），实测代码确认末轮 flush 后同一轮 `pendingSaving()` 同步捕获 savingRef（saveNow 在首个 await 前置位），且 api 20s abort 保证失败落地先于 21s 兜底——假设不成立，以实测代码为准。OBSERVE-83-01 的「当前无害」经 `tabs.length===0 → 空态卡无 pick 入口 → rev 恒 0` 全链路走读确认。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（echoedRef 未置位稳态组合潜在陷阱）+ 延续观察；连续第二十九轮无严重级发现。
