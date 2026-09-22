# Round 79 前端只读审查报告

基线：commit 2aefe2f（R78 双 findings + 收尾总结，R78 收官 HEAD）。本轮为 **R79 前端只读审查 + M-1 延续管理（第十五轮）**，重点为 **R78 变更（commit 14f655e）后的回归复核**。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 五组守护脚本。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false`（tsBuildInfoFile 落 node_modules/.tmp，不触碰 dist）+ 守护脚本只读复跑），工作区基线 HEAD=2aefe2f、`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动。

## 概述

**R78 两项修复逐一核证：①Admin 四 Tab（Accounts/Logs/Stats/Config）isError 错误卡 + refetch 重试按钮全部就位、空态 `!isError` 前提全部成立、五 Tab 对称闭合——但 StatsTab 的"error-first"渲染次序与同族三 Tab（Codes/Accounts/Logs）的"data-first"不一致，轮询瞬时失败会把最近一次有效数据整卡替换成错误卡（MINOR-79-01，R78 自身引入的次序回归，非遗留）；②Select 终局 toast 新增 `revRef.current > 0` 判据核证正确闭合了 MINOR-78-02 的"无改动残留脏误报"分支，但 commit message 声称的"守卫拦下的假清空脏块不再误归因为保存失败"只在 rev=0 分支成立——rev>0 + 守卫拦下假清空（窗口关闭/发布缺席）时 dirtyRef 双语义未拆分，终局 toast 仍弹"目标保存失败"（MINOR-79-02，修复不完整）。契约 20 扫描：行号族残留零命中；但轮次前缀标签族发现 1 处残留（web/scripts/target-guard-check.ts:68 注释"R63 M-1"头，MINOR-79-03）。本轮零 CRITICAL、零 MAJOR、3 MINOR、1 新 OBSERVE。构建验证全绿（tsc -b / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs）。**

---

## 一、R78 修复正确性复核（commit 14f655e）

### 1.1 Admin 四 Tab isError 错误卡 + 重试按钮（逐 Tab 核证）

| Tab | 分支顺序 | 空态 `!isError` 前提 | 重试路径 | 核证结论 |
|---|---|---|---|---|
| CodesTab（R77 已修） | `isLoading → isError → data&&len>0 → 空态`（:445-483） | 结构性满足（isError 分支在空态之前短路） | `codesQuery.refetch()` :450 | 正确 |
| ConfigTab（R78） | 加载提示 `!loaded && !isError`（:687）/ isError 错误卡（:692-699）/ 保存按钮 `disabled={saving \|\| !loaded}` | 加载提示带 `!isError`；isError 时 `loaded===undefined` → save 按钮 `!loaded` 锁定 + 兜底 `if (!loaded) return`（:523） | `configQuery.refetch()` :695 | 正确，error-first 与返回成功后的 effect（:510-518 依赖 [loaded, configEpoch]）回填路径成立 |
| StatsTab（R78） | **isError → !s → rows（:757-766）** | 无空态 | `statsQuery.refetch()` :760 | **次序问题见 MINOR-79-01** |
| AccountsTab（R78） | `isLoading → data&&len>0 → isError → 空态`（:823-898） | 结构性满足（isError :889 在空态 :897 之前） | `accountsQuery.refetch()` :892 | 正确（data-first） |
| LogsTab（R78） | `logs&&len>0 → isError → 空态`（:922-943） | 结构性满足 | `logsQuery.refetch()` :936 | 正确（data-first）；缺 isLoading 见 OBSERVE-79-01 |

- **isLoading 前短路核证**：Codes/Accounts 显式 isLoading 分支在前；Config 用 `!loaded && !isError` 等价；Stats 首帧 `!s` → "加载中"。错误卡均不会在加载中误显。✅
- **refetch() 功能路径核证**：react-query `refetch()` 永不 reject（失败置 error 态而非抛错），四个 `onClick={() => xxxQuery.refetch()}` 无 unhandled rejection；Config 重试成功 → `loaded` 置真 → effect 回填；Stats/Accounts/Logs 重试成功 → data 置真 → 恢复正常渲染。✅
- **与 R77 修复的 CodesTab 五 Tab 对称闭合**：五个 Tab 现在全部具备 isError 错误卡 + 重试入口，MINOR-78-01 闭合。✅（唯一不对称是 StatsTab 的次序，见 MINOR-79-01。）
- **失败态吞并空态零残留**：四个 Tab 的 isError 分支均在空态之前短路，错误态绝不被空态吞并；Config 的"配置加载中"提示已加 `!isError` 前提不再误导。✅

### 1.2 Select.tsx 终局 toast 判据 `revRef.current > 0 && (dirtyRef || savingRef || retryState.timer !== null)`

逐要素核证（Select.tsx:644-653 + 注释 :639-643）：

- **a) revRef > 0 前置正确**：rev 仅在 pick()（:371）与账号复位（:199）变更，回显/轮询/清理绝不触碰 rev——rev>0 精确等于"用户真实点选过"。纯浏览（rev=0）+ 残留 dirty 的 MINOR-78-02 场景二已闭合：flushTargets :489 `latestRev===0` 提前 return 不清脏、saveNow :428 json 去重短路，dirtyRef 残留 true 但终局判据被 revRef 前置拦住，绝不误报。✅
- **b) 守卫拦下的假清空脏块（MINOR-78-02 场景一）——修复不完整，见 MINOR-79-02**。dirtyRef 的双重语义（保存失败置脏 :447 vs 守卫拦下保留脏 五处 :501/:508/:520/:548/:554 + 防抖同族五处 :700/:711/:719/:735/:741）在终局判据中仍未拆分；rev>0 场景下守卫命中 → 三轮 flush 同命中 → dirtyRef 恒 true → 终局 toast 照常弹出。
- **c) 与 onDone 卸载竞态（可疑-1 结论）**：revRef>0 判据不影响可疑-1 的物理不可达论证——三轮循环的 pendingSaving 等待（:629-636）已把 savingRef/timer 收敛静止，"循环退出后到 onDone 间新发 PUT"理论上仅在"flush 末轮触发 saveNow 且 PUT 在 onDone 前一刻置位 savingRef"存在，但末轮 flush 后 `flushedRev===revRef` 复查（:620）保证无新改动 → 无新 PUT，且末轮 flush 触发的 saveNow 会置 savingRef 被 :618 判据捕获进入 pendingSaving 等待直至静止。**可疑-1"物理不可达"结论维持成立。**
- **d) 同 title 去重合并（MINOR-78-03）无回归**：终局 toast title 仍为"目标保存失败"，与失败红条（saveNow catch :440-444 同 title）走后同一条 Toast 去重合并路径（Toast.tsx:40-55，findIndex 命中 → 更新 description/variant、duration 保留首次）。R78 改动未触碰 Toast.tsx，文案突变窗口照旧，作为合并机制的固有权衡维持。

### 1.3 残留扫描

- `git diff HEAD --stat -- web/src web/scripts`：空（工作区洁净，master，本轮前端零修改——R78 修复仅改动 Admin.tsx 与 Select.tsx 两文件，已由 14f655e 落地）。
- 行号引用族（`见 \d+ 行`/`第 \d+ 行`/`行号引用`）：web/src + web/scripts 全仓零命中。
- **轮次前缀标签族（契约 20）**：命中 1 处，见 MINOR-79-03。
- XSS 危险模式（`dangerouslySetInnerHTML`/`innerHTML`/`eval(`）：零命中。

---

## 二、M-1 延续管理（第十五轮）复核

- **shouldDeferSave 三消费点全传 `echoedRef.current` 第三参**（逐字符比对，零回潮）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :500 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :593 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
  - handleBack while :601 同参 `hasSelectedNow()`（:575-576 读 selectedRef 计算，第三参 echoedRef.current）
- **targetGuard.ts:64-71 实现与注释逐一对应**：`stateData===undefined → true` / `echoed → false` / `courses 非空 && hasSelected → true`；第三参 echoed 无默认值，三个消费点均显式传值；脚本内两参调用（`shouldDeferSave(undefined, true)` 等）第三参 undefined 即 falsy、语义等价，无 TS 报错（scripts 不在 tsc -b 覆盖内）。
- **echoedRef 置位三路径 + 首帧不置位边界完整**：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，另含 :287-294 回显内 stale 兜底清理 + 同 title toast）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位
  - 独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` + eslint-disable 延续
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；防抖 effect 依赖 :760 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`——stateData/echoDone/hasPublishes 三路解锁闭合，F42-M1 自愈链零回潮。
- 第十五轮结论：M-1 稳态语义（echoed=true 放行 / 未回显推迟 / 首帧未到推迟）三消费点与 targetGuard 纯函数实现一致，第十七轮维持。

---

## 三、六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + effect 依赖 :760 | 判据只读 stateDataRef 非 echoedRef；/state 到达触发重跑自愈 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用（:42）；依赖含 selected，合并后重跑清理；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :699-702/:710-712/:719-729/:735-737/:741-744 + flushTargets :500-503/:507-510/:519-529/:547-550/:553-556 | 五判据逐条重读：回显未完成推迟 / 发布缺席+已有选中→置脏 / stale 残留→置脏+toast / 联查空集+已有选中→置脏 / 发布 id 漂移→置脏——全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并（已触碰发布保留用户现状含空数组、未触碰补旧目标）、`:254 rev > 0 && !anyHas` 不合并、`:277 !hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :735 / flush :547 全清空放行 + 回显 :254 全清空不合并 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 内兜底守卫 :195-202（TDZ 注释正确） | 无回归 |

防抖 effect 细节重走读：resetRetry 放 effect 顶部（:668，新改动立即清退避 timer）；setTimeout 回调闭包读 `selected` 为 effect 创建时快照、publishesRef/stateDataRef 消费时刻最新；saveNow 四门卸载保护（:423/:434/:439/:451）延续；handleBack 三轮循环 :607-638 + pendingSaving :629 覆盖退避 timer 排队，21s 兜底不无限挂起——全部对照 R78 基线零回潮。

---

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR（3 条）

**MINOR-79-01 — R78 StatsTab 失败态"error-first"次序与同族三 Tab 的"data-first"不一致：5s 轮询瞬时失败会把最近一次有效数据整卡替换成错误卡**

- 位置：Admin.tsx:757-763（StatsTab 渲染分支 `statsQuery.isError ? errorCard : !s ? 加载中 : rows`）对比 CodesTab:447-453 / AccountsTab:825-898 / LogsTab:922-942 的 `data && len>0` 优先于 `isError`。
- 触发场景推演：StatsTab 每 5s refetchInterval 轮询。某次轮询恰逢网络抖动/后端重启（retry:1 重试后仍失败）→ react-query `status='error'`、`isError=true`，但 `.data` 仍持有 5s 前最后一次成功快照（含"窗口状态/识别开放时间/教务令牌"等管理员紧盯字段）。R78 的 isError 前置分支直接渲染错误卡，**最近一次有效数据被整卡隐藏**；下一轮轮询成功才恢复 rows。同屏的 AccountsTab/LogsTab 在同样条件下（经 data 优先）照常显示旧数据，管理员会困惑"为什么账号/日志还在，运行状态却消失"。
- 修复建议：把次序改为 data 优先——`s ? rows : isError ? errorCard : 加载中`，与同族三 Tab 一致；错误卡仅在后端从未成功（data 缺席）时渲染。
- 严重度论证：非 R77 遗留，是 R78 修复动作引入的渲染次序回归；R78 的意图是"失败态可视化"，但 error-first 对"轮询型有历史数据"的查询是降级——错误卡信息量＜最近有效快照。属 UX 误导级，无数据风险。

**MINOR-79-02 — Select 终局 toast 修复只闭合了 MINOR-78-02 场景二（rev=0 残留脏），未闭合场景一（rev>0 + 守卫拦下假清空脏块仍归因为"目标保存失败"）**

- 位置：Select.tsx:644-653（终局 toast 判据）与 dirtyRef 双重语义（保存失败置脏 :447 vs 守卫拦下保留脏 四类 :501/:508/:520/:548/:554 + 防抖同族 :700/:711/:719/:735/:741）。
- 触发场景推演：窗口已关闭 / 开窗瞬间发布缺席（publishes 恒空）时用户改动过目标（rev>0）→ 防抖/flush 命中"发布缺席 + 已有选中"守卫（:508/:548）置脏跳过 → handleBack 三轮循环每次 flush 命中同一守卫不落库（saveNow 从未被调用，pendingSaving 恒 false 不等待）→ 循环结束 dirtyRef 仍 true → 终局 toast「目标保存失败，改动未落库」照常弹出。此时没有任何"保存失败"——是守卫刻意拦下"数据缺席"防假清空覆盖；且用户返回前大概率已收到"发布已更新，旧批次目标已失效，已停止保存"的 warning toast（:521-527/:721-727），两条与之矛盾的"目标保存失败"红色错误卡同屏冲突。
- commit message 声称"守卫拦下的假清空脏块不再误归因为保存失败"——实际仅 rev=0 分支成立（无改动残留脏被 revRef 前置拦截），rev>0 守卫脏块路径的误归因原样保留。R78 findings 推荐的修复方案"②守卫脏与保存失败脏拆分标记"未落地。
- 修复建议：新增独立标记（如 `guardBlockedRef`）承载守卫置脏（:501/:508/:520/:548/:554/:700/:711/:719/:735/:741），saveNow 失败仍写 `dirtyRef`；终局 toast 判据改 `revRef.current > 0 && dirtyRef.current`（只对真实保存失败触发），守卫分支若有提示需求改 warning 文案"目标改动暂未保存（平台数据尚未就绪/窗口已关闭），返回后以服务端目标为准"，与"发布已更新"toast 语义一致、不冲突。
- 严重度论证：信息内容"改动未落库，返回后以服务端保存的目标为准"在守卫场景下字面为真（改动确实未落库），故非数据级缺陷；但 title"目标保存失败"暗示网络/接口故障，与守卫语义（系统主动拦截）矛盾，且与用户刚收到的"发布已更新"warning 直接冲突——R78 声称修复的判据精度问题实质未闭合。

**MINOR-79-03 — 契约 20 轮次前缀标签残留 1 处：web/scripts/target-guard-check.ts:68 注释带"R63"轮次锚点**

- 位置：web/scripts/target-guard-check.ts:68「// R63 M-1：已回显完成（echoed=true）稳态——courses 永驻非空 + 有选中，旧目标已合并进 selected……」。
- 触发说明：R77/R78 两轮"契约 20 扫描零残留"结论仅覆盖行号引用族（`见 N 行`），未覆盖轮次前缀标签族。本轮按任务清单对 web/src + web/scripts 双侧扫描，行号族零命中，轮次标签族命中此 1 处（`R63` 为明确轮次锚点；同文件其余形如 B43-04/F43 的契约 ID 属于决策手册的合法交叉引用，不算轮次标签）。
- 修复建议：剥离 "R63 " 前缀，保留"已回显完成（echoed=true）稳态"的 why 语义（即删除后为「// 已回显完成（echoed=true）稳态——……」，M-1 决策历史统一落 CLAUDE.md 工程决策手册）。
- 严重度论证：纯注释卫生，但处于"发现即剥离"的显式契约范围内，且与历轮"清零"结论冲突，属复查遗漏。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-79-01 — Admin LogsTab 缺 isLoading 短路：Tab 首次进入短暂闪"暂无日志"**

- 位置：Admin.tsx:922-943。LogsTab 分支顺序 `logs && len>0 ? list : isError ? errorCard : 空态`，无 isLoading 前置——首帧在途时 `logs===undefined`、未失败 → 误入空态闪一帧"暂无日志"，首帧到达即恢复。CodesTab（:445）、AccountsTab（:823）均有 isLoading 分支，StatsTab 有 `!s → 加载中`，唯 LogsTab 漏。修复建议：加 `!logsQuery.isLoading &&` 前提或补 isLoading 分支。

**OBSERVE-78-01（延续）**：handleBack 终局 toast 与退出前 flush 失败红条同 title 合并的文案突变窗口（超时路径与即时返回路径同源），去重已防轰炸，维持「续」。

**OBSERVE-77-01（延续）**：Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存——普通学生场景语义等价零影响。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / **setSelected 调用点清点（本轮 grep 实测七处：:158 useState / :198 account reset / :250 回显合并函数式 / :288 回显 cleanStale 函数式 / :320 独立清理函数式 / :351/:361 pick 对象式快照——与 R78 基线一致，无新增、无第三来源）**——均维持「续」。

### 可疑待核

**可疑-1（延续）** — 终局 toast 与 onDone 卸载竞态：R78 的 revRef 判据不影响物理不可达论证（末轮 flush 触发的 saveNow 置位 savingRef 会被 :618 pendingSaving 等待捕获收敛），维持"物理不可达、无需修复"结论。

---

## 五、已核无缺陷清单

- R78 修复 ①Admin 四 Tab isError 错误卡 + `refetch()` 重试按钮全部就位，isLoading 前短路正确、空态 `!isError` 前提全部成立、与 R77 CodesTab 五 Tab 对称闭合（StatsTab 次序问题除外，见 MINOR-79-01）；②终局 toast `revRef.current > 0` 判据精确表达"用户真实改动过"，rev=0 误报完全闭合；③同 title 去重合并无回归、可疑-1 不可达结论仍成立。
- M-1 延续管理（第十五轮）：shouldDeferSave 三消费点（防抖/flush/handleBack 判定+while）全传 echoedRef.current 第三参、echoedRef 置位三路径 + 首帧不置位边界完整、target-guard 断言 18/18 全绿。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + 消费时刻双闸）：逐条判据与注释对应，stateData/echoDone/hasPublishes 三路解锁闭合，零回潮。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：行号引用族全仓零残留；XSS 危险模式（innerHTML/dangerouslySetInnerHTML/eval）零残留。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、撞名学生管理态（isCurrentAdminSession 六判定）、aria 无障碍、btn_type 三向、max_count=0 四处同源、倒计时打底——复跑全量通过，零差异化、零回潮。
- localStorage 读写全 try/catch 降级；视觉护栏 audit.mjs 全绿（画布双宽度/玻璃工具类/实心黑洞清零）。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，tsBuildInfoFile 落 node_modules/.tmp 只读校验） | ✅ EXIT 0（类型全通过） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 全部通过（视觉护栏） |
| 全仓残留扫描（行号族 / innerHTML / dangerouslySetInnerHTML / eval(） | ✅ 零命中 |
| 契约 20 轮次前缀标签扫描 | ⚠️ 命中 1 处（target-guard-check.ts:68，见 MINOR-79-03） |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第十五轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01/77-02**：同 key 不同 URL 缓存身份 / 窗口关闭无目标管理入口——维持「续」。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false`（构建产物只写 node_modules/.tmp，未触碰 web/dist / backend/web/dist）+ 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=2aefe2f），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round79-frontend-findings.md）。
- 走读推断与实测冲突处理：MINOR-79-02 的触发路径（守卫置脏 → 三轮 flush 同命中 → dirtyRef 恒 true → 终局 toast 弹出）先经闭环走读确认，再核对 R78 commit message 的修复声明与代码差异（14f655e 仅改判据加 revRef>0，未动 dirtyRef 语义）——以代码实测为准：该修复只闭合 rev=0 分支。
- 本轮定级口径：零 CRITICAL/MAJOR；3 条 MINOR（StatsTab error-first 次序回归[R78 引入] / 终局 toast 守卫脏误归因未闭合[M-1 延续] / 契约 20 轮次标签残留 1 处）+ 1 条新 OBSERVE（LogsTab 无 isLoading 短路）+ 延续观察与可疑项；连续第二十五轮无严重级发现。