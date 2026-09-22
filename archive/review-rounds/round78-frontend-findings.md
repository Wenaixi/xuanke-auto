# Round 78 前端只读审查报告

基线：commit 15ad1c7（R77 双 findings + 收尾总结，R77 收官 HEAD）。本轮为 **R78 前端全模块只读审查**，重点为 **R77 变更（commit a9f7166）后的回归复核**。审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 五组守护脚本。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + tsc -b + 守护脚本只读复跑），工作区洁净零改动。

## 概述

**R77 变更（a9f7166）三项修复逐一核证：①Admin CodesTab 错误卡语义正确，但发现其「空态分支加 !isError 前提」存在结构性覆盖遗漏——AccountsTab/LogsTab/StatsTab/ConfigTab 四个兄弟 Tab 的失败态仍被吞并（MINOR-78-01 升级实证，非 R77 引入但本轮明确「尚未修复完毕」）；②Select 终局 toast 判据存在 dirtyRef 双语义误报窗口（MINOR-78-02），且触发场景比 R77 报告推演的更宽——不依赖退避停手、任何「无用户改动 + 残留 dirty」状态都会误报；③终局 toast 与失败期间红条的同 title 去重合并在超时窗口下存在一瞬双显（MINOR-78-03）。三条约均属 UX 误导级，无数据风险。本轮零 CRITICAL、零 MAJOR。**

---

## 一、R77 修复正确性复核（commit a9f7166）

### 1.1 Admin.tsx CodesTab 错误卡（新增 isError 分支）

- 位置：Admin.tsx:447-453。
- 语义核证：`isLoading → isError → data&&data.length>0 → 空态` 四层分支顺序正确；`isError` 分支独立渲染错误文案 + `codesQuery.refetch()` 重试按钮；isLoading 在前短路，加载中不误显错误卡。
- **触发方式差异**：此处是点击「刷新」或 Tab 重进触发 `refetch()`（非受控 query 的显式重试），不是 react-query 自动 retry——但功能路径成立。
- **覆盖遗漏（MINOR-78-01）**：AccountsTab（Admin.tsx:808-876）失败→"暂无账号"、LogsTab（:900-915）失败→"暂无日志"、StatsTab（:749-750）失败→"加载中..."永转、ConfigTab（:687-691）失败→表单空白 + "配置加载中"误导——四个兄弟 Tab 的失败态仍全部吞并。R77 只修了 CodesTab 一个，其余四 Tab 处于同一缺陷族。空态分支（AccountsTab :874-876 "暂无账号"、LogsTab :912 "暂无日志"、StatsTab :749-750 "加载中..."、ConfigTab :687-691 "配置加载中"）均未加 `!isError` 前提。属 R77 未覆盖完毕的遗留面。

### 1.2 Select.tsx handleBack 终局 toast（新增 :639-648）

- 位置：Select.tsx:642-648。判据 `dirtyRef.current || savingRef.current || retryState.current.timer !== null`。
- **判据语义核证（MINOR-78-02）**：`dirtyRef` 承担两重语义——「保存失败置脏」（saveNow catch :447）与「守卫拦下保留脏」（防抖/flush 七处 :501/:508/:520/:548/:554/:695/:706/:715/:731/:737）。二者在终局 toast 判据中不区分：
  - **守卫脏块误报**：用户改动命中「发布缺席 + 已有选中」等守卫（:508/:548/:554，即"数据缺席绝非用户意图"刻意拦下的脏块），循环重试均命中同一守卫（数据缺席未解除 → 每次都置脏不落库）→ 三轮超时（21s/42s/63s）后 dirtyRef 恒 true → 终局 toast「目标保存失败，改动未落库」弹出。但此时没有任何"保存失败"——是守卫在刻意拦假清空。文案"目标保存失败"对用户是**错误归因**。
  - **触发场景拓宽**：R77 报告推演依赖"退避 5 次停手"（约 46s 失败链），但**任何一次守卫命中置脏**（如开窗瞬间发布短暂清空、窗口已关闭）都会让 dirtyRef 在返回时仍为 true——不依赖退避，触发面远大于 R77 推演。
  - **无用户改动的脏残留误报**：dirtyRef 只在 saveNow finally（:455）与 resetRetry（:407-411）被清，**没有伴随 rev=0 的清理路径**——用户全程未改动（rev=0）但 dirtyRef 残留 true（例如上一次会话失败链后返回再进入，或防抖回调延迟执行置脏），返回时误报。虽然 handleBack :617 的三轮 flush 中 `flushTargets` 在 `latestRev===0` 时直接 return（:489）不清脏、saveNow 也被 `json===lastJson` 短路（:428），dirtyRef 保持 true → 终局误报。「目标保存失败，改动未落库」对从未改动的用户是硬伤。
- 修复建议：终局判据加 `revRef.current > 0`（用户真实改动过才可能报"改动未落库"），并将"守卫假清空脏"（:508/:548/:554 等）与"保存失败脏"（:447）分标记，终局 toast 只对后者触发；守卫脏块应改文案"目标改动暂未保存（平台数据尚未就绪），稍后重试"或干脆静默（守卫有既定的自愈链）。
- **正确性核证部分**：savingRef / retryState.timer 两个判据语义正确（在飞 PUT 或退避排队 = 确实未落库）；toast 在 onDone 前触发，无卸载后 setState；与失败红条同 title "目标保存失败" 走 Toast 去重合并（见 1.3），不会堆叠。

### 1.3 终局 toast 与失败红条去重（Toast.tsx:40-55）

- 机制核证：`key = typeof title === "string" ? msg.title : null`；同 title 命中 `findIndex` → 合并更新 description/variant，**保留首次 duration**（:50 注释：Radix duration 变化会重启计时器）。即「红条（失败期间）」与「终局 toast（onDone 前）」标题同为"目标保存失败"时，终局 toast 会合并进**仍在展示的失败红条**（duration 保留首次 3500ms），不新增堆叠。
- **残余窗口（MINOR-78-03）**：失败红条自身 duration 3500ms；若用户点击返回时失败红条**已过期消失**（持续失败期间 3.5s 间隔无新 toast——scheduleRetry 每次失败都 toast，46s 退避链中失败间空隙 > 3.5s 即旧条先消失）→ 终局 toast 作为"新条"正常插入，无去重问题；但若用户恰在"失败红条仍在展示的 3.5s 窗口内"点返回 → 终局 toast 合并进旧条（description 更新为终局文案、duration 保持 3500ms 剩余），**视觉上红条内容突变**（从"通信异常"变"改动未落库"）——一瞬双显不成立（同 title 不会堆叠），但"文案突变"是轻微 UX 瑕疵。**此为去重机制的固有权衡**（与 R77 注释"同 title 去重合并机制防轰炸"一致），无轰炸，不影响正确性。
- 复核结论：去重机制正确，防轰炸目标达成；"文案突变"属可接受的 UX 妥协。

### 1.4 残留扫描与前端零改动确认

- `git diff HEAD --stat -- web/src`：空（工作区洁净，master）。前端最近提交 a9f7166（R77 修复本身），本轮前端零修改。
- 契约 20 行号引用残留扫描（`见 \d+ 行`/`第 \d+ 行`/`行号引用`）：web/src + web/scripts 全仓零命中。
- XSS 危险模式扫描（`dangerouslySetInnerHTML`/`innerHTML`/`eval(`）：零命中。

---

## 二、M-1 延续管理（第十四轮）复核

- **shouldDeferSave 三消费点**（全部传 `echoedRef.current` 第三参）：
  - 防抖回调 :694 `selectedCount > 0`。
  - flushTargets :500 `latestSelectedCount > 0`。
  - handleBack 判定 :593 + while :601 两处均消费时刻 `hasSelectedNow()`（:575-576 读 selectedRef 计算）。
  - 逐字符比对与 R77 基线一致，零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - courses 空分支 :238-239（置 true + echoDone 同步）。
  - 合并完成分支 :295-296（置 true + echoDone 同步 + :287-294 回显内 stale 清理 toast 兜底）。
  - account reset :200（置 false + setSelected({}) + rev 清零 + echoDone false）。
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位。
  - 独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable 延续。
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`——data/rev/selected 随轮询重建引用但首行短路零开销。
- **TDD 复跑**：target-guard 18/18 全绿（含 M-1 稳态两条）；admin-auth 6/6；unauthorized 5/5。
- **OBSERVE-66-03 setSelected 调用点清点**（grep 复跑）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（用户 pick 对象式快照）——七处与 R77 基线一致，**无新增、无第三来源**。

---

## 三、六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归复核

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true`/`echoed→false`/`courses 非空 && hasSelected`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :680-687 防抖回调 stateDataRef + effect 依赖 :755 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]` | 判据只读 stateDataRef（非 echoedRef）；/state 到达触发 effect 重跑自愈；stateData/echoDone/hasPublishes 三路解锁闭合 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用（:42）；effect 依赖含 selected，合并后重跑清理；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :694-697/:705-708/:714-724/:726-733/:736-739 + flushTargets :500-503/:507-510/:519-529/:547-550/:553-556 | 五判据逐条重读：回显未完成推迟 / 发布缺席+已有选中→置脏 / stale 残留→置脏+toast / 联查空集+已有选中→置脏 / 发布 id 漂移→置脏——全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并（已触碰发布保留用户现状含空数组、未触碰补旧目标）、`:254 rev > 0 && !anyHas` 不合并；:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :730 / flush :547 全清空放行 + 回显 :254 全清空不合并 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT []；清空语义绝不复活 |
| key={account} | App.tsx:293-298（targetAccount 分支）/ :339-344（current 分支） | 双挂载点 key 绑定账号；Select 内兜底守卫 :195-202 延续，TDZ 注释（:193-194）正确 |
| Toast 定位/去重 | Toast.tsx:105 viewport + :40-55 同 title 合并 | 无回归；合并更新 description 保留 duration，variant 取最新 |

### 防抖 effect 细节复核（重点风险位重走读）

- **resetRetry 放 effect 顶部**：:660-663 `if (rev === 0) return; resetRetry()`——新改动立即清旧退避 timer。
- **setTimeout async 回调闭包**：回调读 `selected`（:667/:704）为 effect 创建时快照，`publishesRef`/`stateDataRef` 消费时刻最新；selected 快照正确性由"effect 依赖含 selected"（:755）兜底。
- **saveNow 去重/串行化**：:426-428 `json === lastJson.current` 跳过重复；:434 卸载后成功不落 lastJson；:437-448 catch 置 dirty + scheduleRetry（attempt≥5 停）；:449-457 finally `savingRef=false` + `dirtyRef && attempt===0` 补发最新快照；四门卸载保护（:423/:434/:439/:451）拦截卸载后一切交互，无孤儿请求。
- **handleBack 保存链静止等待**：:607-638 三轮循环；`:618 if (!dirtyRef.current && !savingRef.current)` 等一帧复查 revRef 一致才 break；`:629-636 pendingSaving()`（savingRef || retryState.timer）覆盖退避 timer 排队；21s 兜底绝不无限挂起。**R77 新增终局 toast 挂在循环外 :639-648（onDone 前）——判据包含 dirtyRef，把守卫假清空脏块也计入（见 MINOR-78-02）。**

---

## 四、登录/激活链 + 撞名学生管理态 + 人性化细节核（复跑）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 登录幂等守卫 | Login.tsx:36 `if (loading) return` | 连按两次 Enter/快速双击只发一请求 |
| 票据贯通 | :54 `setPendingTicket((e.data?.ticket as string) || "")` + :78 激活 body 带 `ticket` | 1001 分支保存 data.ticket（ApiError.data 透传 client.ts:30-33）随激活回传；取消清票；"过期/已用"与"激活码错误"两分支同款清票、文案差异化引导——完整 |
| 激活幂等守卫 | :68 `if (activating) return` | 与 submit 对称 |
| 401 单广播 | client.ts:64 HTTP 状态码前置广播 + :85 body 层 `r.status !== 401` 兜底 | HTTP 401 单次、HTTP 200+body 401 单次、双形态互斥不重复；HTTP 非 JSON 体先广播再抛 -2 |
| 20s 超时兜底 | client.ts:56 `setTimeout(() => ctrl.abort(), 20000)` + :103 AbortError→"请求超时，请重试" | 与注释"20 秒"一致；signal 显式接入（:58 `rest.signal ?? ctrl.signal`）调用方优先 |
| 登出吊销 | App.tsx:112-132 | apiLogout 作废服务端令牌 + 快照式三连 + 清 adminToken/targetAccount/page 全路径 |
| onDeleted | App.tsx:160-178 | 快照式三连 + 清 targetAccount/current + 删管理员自身退管理态+清标记（:172-177） |
| onUnauthorized | App.tsx:187-235 | 按令牌反查归属（:193-197）、管理代理态不误杀（:217 提前 return）、删管理自身退管理态+清标记（:210-215）、快照式落盘（:225-230）——全链路完整 |

### 撞名学生管理态

- `isCurrentAdminSession` 纯函数：`adminToken !== "" && sessions[adminName] === adminToken`；渲染判据 App :299 与账号迁移 effect :147 同源；admin-auth 6/6 全绿。**第十四轮零回潮。**
- 管理令牌持久化 `xk_admin_token` 全路径清除点：logout :121-122 / onDeleted :175-176 / onUnauthorized :213-214 / onBackToStudent（App :312-313）四处全覆盖，无泄漏路径。

### 人性化细节核

| 项 | 位置 | 核证结论 |
|---|---|---|
| 倒计时 begin_times 兜底 | Select :767-772 / Dashboard :190-195 | 识别缺席（open_time_known=false）用 `begin_times[0]` 兜底，识别槽建立后以识别真值为准；F39-N1 承诺已落地到倒计时输入 |
| 倒计时横幅五态 | Select :827-853 | `!stateData`→同步中 / window_closed→已关闭 / window_opened→已开放 / 双缺席→未识别到开放时间 / isExpired→本地已到点等待 / 正常倒数；`|| cd.isExpired` 已清除 |
| 倒计时自校正 | useTickingCountdown :16-18 | target 变化立即回正，消除 1s 陈旧偏差 |
| btn_type 三向 | Select :1116 `===1` 退选 / :1129 `===2` 报名 / 其他不渲染 | 与官网逆向契约一致；can_select 双守卫 disabled+title |
| max_count=0 名额未公布 | :967 筛选不过滤 / :1006 isFull / :1009-1010 unannounced / :1092-1094 文案 / :1101-1105 Progress value=0 max=1 | 四处同源 `max_count>0` 判据；不误显"已满额/余0席/满条" |
| 空态 | Select :916-922 无可选批次 / Dashboard :535-545 无预选课程 | 均有明确文案 + 行动引导；顶栏徽章分母 `publishes.length > 0 ? '/'+len : ''` 杜绝 "/0" |
| aria 无障碍 | 退选弹窗 role=dialog+Esc+autoFocus（Select :1201-1250）/ 删除弹窗同款（Admin :210-284）/ 激活弹窗+Esc（Login :223-316）/ switch role+aria-checked（Admin :575-590）/ progressbar（Progress :23-28）/ CollapseSection useId（Dashboard :64-67） | 全站完备 |
| 在飞幂等全站点 | 登录/激活/报名/退选/删账号/删激活码/生成码/保存配置 | Set 独立跟踪无互踩，disabled 渲染延迟前入口短路齐全 |

---

## 五、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR（3 条）

**MINOR-78-01 — R77 只修了 CodesTab 一个 Tab 的错误卡，其余四个 Tab（Accounts/Logs/Stats/Config）失败态仍被吞并成业务空态/永"加载中"**

- 位置：Admin.tsx CodesTab :447-453（已修）/ AccountsTab :808-876（失败→"暂无账号"）/ LogsTab :900-915（失败→"暂无日志"）/ StatsTab :749-750（失败→"加载中..."永转）/ ConfigTab :687-691（失败→表单空白 + "配置加载中"）。
- 触发场景推演：管理端切到「账号管理/日志总览/运行状态/系统配置」任一 Tab，恰逢网络抖动/后端重启/反代超时 → react-query `retry:1`（App.tsx:13）重试后仍失败 → `data===undefined` → Accounts 表显示"暂无账号"（误导管理员以为账号全没了，甚至影响误删判断）、Logs 显示"暂无日志"、Stats 永"加载中..."、Config 表单空白（保存按钮已 disabled 安全但无解释）。四个 Tab 均无任何重试入口，只能切 Tab/刷新页面碰运气。CodesTab 因 R77 已修而存在不对称——同一缺陷族内部修复进度不一致，继续遗留会误导管理员在"真无数据"与"请求失败"间无从区分。
- 修复建议：将 R77 的 isError 错误卡模式复制到四个兄弟 Tab（各 Tab 的 isLoading/isError/data 分支重排 + `refetch()` 重试按钮），或抽一个统一 `QueryGuard` 组件收敛五个 Tab。属可操作性/误导性缺陷，无数据风险。

**MINOR-78-02 — 终局 toast「目标保存失败」判据把"守卫拦下的假清空脏块"也计入，且不区分"用户有无改动"——守卫场景误报 + 无改动残留误报**

- 位置：Select.tsx:639-648（终局 toast 判据 `dirtyRef.current || savingRef.current || retryState.current.timer !== null`）与 dirtyRef 双重语义（保存失败置脏 :447 vs 守卫拦下保留脏 :501/:508/:520/:548/:554/:695/:706/:715/:731/:737）。
- 触发场景推演（两路，均不依赖 R77 报告的"退避 5 次停手"）：
  1. **守卫脏块误报**：窗口关闭/开窗瞬间发布缺席（publishes 恒空）时用户改动目标 → 防抖/flush 命中"发布缺席 + 已有选中"守卫（:508/:548）置脏跳过 → 三轮 flush 循环均命中同一守卫不落库 → 循环超时后 dirtyRef 仍 true → 终局 toast「目标保存失败，改动未落库」弹出。但此刻没有任何保存失败——是守卫刻意拦下"数据缺席"防止假清空，目标是安全的（后端旧目标未被抹除）。文案把"系统安全拦截"错误归因为"保存失败"，用户被误导以为网络故障。
  2. **无改动残留误报**：dirtyRef 没有任何「rev=0 时清理」路径（仅 saveNow finally :455 与 resetRetry :407-411 清除）——上一次失败链/防抖延迟置脏后残留 true，用户本轮全程未改动（rev=0），返回时误报"改动未落库"，而用户根本没改动。
- 复核结论：文案误报与错误归因，非数据丢失（目标实际安全，后端旧目标保持）。但"目标保存失败"对无改动用户是硬伤级 UX 缺陷。
- 修复建议：①终局判据加 `revRef.current > 0`（无改动绝不说"改动未落库"）；②守卫脏（:508/:548/:554 等"数据缺席"类）与保存失败脏（:447）拆分标记，终局 toast 只对后者触发；③守卫场景若需提示，改文案"目标改动暂未保存（平台数据尚未就绪），返回后以服务端目标为准"并轻量化（或复用既有"发布已更新"warning toast 语义）。

**MINOR-78-03 — 终局 toast 与失败红条同 title 去重合并的"文案突变"窗口：用户恰在失败红条展示期点返回，红条内容瞬间从"通信异常"变"改动未落库"**

- 位置：Select.tsx:642-648（终局 toast）与 Toast.tsx:40-55（同 title 合并：findIndex 命中则更新 description/variant、**保留首次 duration**）。
- 触发场景推演：持续失败链中用户点返回 → 若此刻失败红条（title="目标保存失败"）仍在 3500ms 展示窗口内 → 终局 toast 同 title 命中合并 → 红条 description 从"通信异常，请重试"突变终局文案"改动未落库，返回后将以服务端保存的目标为准"（duration 保留剩余时间）。非堆叠、非轰炸，但视觉上"内容跳变"且终局文案与"立即返回"动作同帧出现，用户可能来不及阅读。
- 复核结论：去重机制本身正确（防轰炸目标达成），"文案突变"是合并机制固有副作用，轻微 UX 瑕疵，不影响正确性。若终局文案希望独立呈现，可改 title 为"目标保存失败（最终状态）"规避合并；但会牺牲去重防轰炸——权衡取舍，留待修复轮裁决。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-78-01 — handleBack 终局 toast 与退出前 flush 失败红条可能同时存在（63s 超时路径）**

- 位置：Select.tsx:629-648。持续失败链（每轮 flush 失败 → scheduleRetry toast → 退避 timer 挂起 → pendingSaving 等待超时）三轮后，失败红条可能仍在展示且终局 toast 同 title 合并——与 MINOR-78-03 同源但触发在超时路径（非用户即时返回）。观察项，去重已防轰炸。

**OBSERVE-77-01（延续）**：Dashboard 与 Select 同 queryKey 不同 URL，跨路由首帧命中旧 URL 缓存——普通学生场景语义等价零影响；管理代理路径不经 Dashboard 无触发。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界，非缺陷）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端 1000 兜底）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title，读屏朗读默认 "Close"。维持「续」。

**OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 七调用点无新增——均维持「续」。

### 可疑待核

**可疑-1 — R77 终局 toast 判据 `savingRef.current` 在 onDone 卸载竞态下可能误报"在飞"（理论窗口）**

- 位置：Select.tsx:642-648 与 onDone（:649）之间的同步序列。
- 触发场景推演：三轮循环退出后若 saveNow 恰在 onDone 前一刻置位 savingRef=true（飞行 PUT），终局判据命中 savingRef → 弹 toast"改动未落库"——但该飞行 PUT 可能在 onDone 后成功落库（fire-and-forget，无 await）。toast 文案与真实结果分叉。
- 复核结论：三轮循环的 pendingSaving 等待（:629-636）已把 savingRef 收敛到静止后才退出循环，理论窗口仅存在于"循环退出后到 onDone 之间新发 PUT"——flushTargets 三轮最后一次调用后无新改动（revRef 复查 :620）则无新 PUT，此窗口物理不可达。仅理论梳理，**无需修复**。

---

## 六、已核无缺陷清单

- R77 修复 ①Admin CodesTab 错误卡四层分支顺序正确、isLoading 在前短路；②终局 toast 的 savingRef/timer 两判据语义正确、无卸载后 setState；③同 title 去重防轰炸机制正确（duration 保留首次、variant 取最新）。R77 三修复整体语义正确，遗留缺陷为覆盖面（MINOR-78-01）与判据精度（MINOR-78-02/03）。
- 前端 R77 后零修改确认（git diff HEAD --stat -- web/src 空），M-1 第十四轮三消费点逐字符比对零回潮、echoedRef 置位三路径 + 首帧不置位边界完整。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + F15/F16/F17 消费时刻双闸）：逐条判据与注释逐一对应，自愈链（stateData/echoDone/hasPublishes 三路解锁）完整闭合，handleBack 收敛循环无挂起路径。
- 撞名学生管理态判定：admin-auth 6/6 全绿，刷新恢复判据正确，令牌标记四清除点全覆盖。
- 票据贯通 / 401 单广播（HTTP 401 前置 + body 401 兜底互斥） / 20s 超时兜底 / 登出吊销 / onDeleted / onUnauthorized：全链路完整，快照式三连一致无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布四处同源 / 倒计时兜底 / 空态 / aria 全站完备。
- 轮询降频全站统一（window_closed/failure 30s、黄金期 2s、其余 10s、Dashboard 3s/30s），refetchInterval TDZ 规避正确（Select :77 经 queryClient.getQueryData 读 /state 缓存）。
- XSS/敏感数据/硬编码开放时间：零命中（无 innerHTML/dangerouslySetInnerHTML/eval；token 只存 localStorage + Bearer 头发送；开放时间唯一事实源注释清晰）。CSS 无危险 URL 注入。
- localStorage 读写全 try/catch 降级（App.tsx load/save 八处含 adminName/adminToken），隐私模式静默降级不崩。
- 视觉一致性护栏（audit.mjs）：全绿（画布双宽度/统一玻璃工具类/实心黑洞清零）。

## 七、构建验证

| 项 | 结果 |
|---|---|
| `npx tsc -b`（web/ 下） | ✅ EXIT 0（类型全通过，无 TDZ 误触） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉护栏） |
| 全仓残留扫描（`见 \d+ 行`/`dangerouslySetInnerHTML`/`innerHTML`/`eval(`） | ✅ 零命中 |
| `git diff HEAD --stat -- web/src` | ✅ 空（工作区洁净，master，零改动） |

## 八、历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第十四轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——本轮复跑无升级证据，维持「续」。
- **OBSERVE-77-01/77-02**：同 key 不同 URL 缓存身份 / 窗口关闭无目标管理入口——维持「续」。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，留存为稳定性契约注释候选。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + 守护脚本只读复跑）；工作区 `git status` 洁净，未修改任何仓库文件（唯一写入为本报告文件，属 team-lead 明确指定的输出路径）。
- 复核与既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态、target-guard 判据、删账号 memory-first、指针身份比对族、401 单广播双形态互斥）。
- 本轮定级口径：零 CRITICAL/MAJOR；3 条 MINOR（R77 覆盖面遗留 / 终局 toast dirty 双语义误报 / 去重文案突变）+ 1 条新 OBSERVE + 1 条理论可疑（不可达）；连续第二十四轮无严重级发现。
