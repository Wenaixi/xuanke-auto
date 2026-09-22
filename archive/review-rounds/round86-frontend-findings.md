# Round 86 前端只读审查报告

基线：commit 9f17057（R85 双 findings + 收尾总结，HEAD）。本轮为 R86 前端只读审查 + M-1 延续管理（第二十二轮），核心为 M-1 第二十二轮 shouldDeferSave 三消费点（判据 + while 全传 echoedRef 第三参、置位三路径 + 首帧不置位边界）逐字符闭合、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归 + setSelected/dirtyRef 清点 + 三组断言与构建复跑、新视角扫查（react-query 查询键稳定性与 invalidate 覆盖、登录/激活/登出失败路径 state 残留、卸载后 setState 守卫、无障碍基线抽查、tabular-nums 一致性）、OBSERVE-85-01/85-02 提级评估、契约 20 全仓扫描。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 相关契约（handleAdminCodes / handleSetTargets / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + `npm run build` + 守护脚本只读复跑），`git status --short --branch` 为 `## master` 洁净，`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（Admin CodesTab/ConfigTab 的 label 与输入框无 htmlFor 关联，表单可访问性弱项）+ 1 条 OBSERVE 归档建议（OBSERVE-85-01 注释口径分叉评估维持 OBSERVE 不升级）+ 延续观察管理。** M-1 延续管理第二十二轮闭合：shouldDeferSave 四消费点（防抖 :697 / flush :507 / handleBack 判定 :595 + while :603）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:238/:295）+ 首帧不置位边界（:232 前置 return / :245 pubs 空不置位）逐字符完整、读写点全量清点（置位 3 + 读点 6：:227/:317 守卫 + 四消费点，无第七处）、target-guard 18/18 实测全绿。六防保存链各判据逐条重读与注释对应，setSelected 七调用点（:158/:198/:250/:288/:320/:351/:361）无第三来源，dirtyRef 置 true 仅三处（:453/:561/:740 真实失败/飞行中补发语义），零回潮。新视角五组扫查全部收敛：react-query 查询键含 account+sessionToken 无跨账号污染、改数据路径 invalidate 全覆盖（electives/state 手动操作 + admin-* 各 Tab）、登录/激活/登出失败路径 loading/error 复位完整、卸载后 setState 守卫（unmountedRef + 延迟 setState 全路径）、无障碍基线抽查（本轮新发现见发现清单，历轮修复族零回潮）、tabular-nums 一致性（全仓倒计时/人数/进度数字均含 tabular-nums）。OBSERVE-85-01（useTickingCountdown 注释口径分叉）评估：维持 OBSERVE 不升级——React 19 同构渲染的 setNow 每 tick 触发宿主组件整树重渲染是注释与实际不符的语义错误，但无功能/数据风险、非本轮回归、DOM 差分成本低；抽 memo 叶子组件改动面大且当前收益不明显，建议后续与任意性能触发的改动一并处理。OBSERVE-85-02（黄金期末尾 400ms 防抖竞态改动静默丢弃）确认仍属「绝不假清空」安全方向刻意牺牲、无新触发面。OBSERVE-84-01/83-01/77-02/76-01/76-02/76-03 延续。契约 20 全仓扫描：轮次前缀标签族/行号引用族/XSS 危险模式/web/src 与 web/scripts 零命中、localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级、视觉护栏 audit.mjs 77 项全绿。构建验证全绿（tsc -b EXIT 0 / tsc -b --force EXIT 0 / npm run build 全量成功 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 77 项通过）。连续第三十二轮无严重级发现。

---

## 一、M-1 延续管理（第二十二轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处调用，零回潮、逐字符核对）：
  - 防抖回调 :697 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :507 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :595 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
  - handleBack while :603 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`（与 :595 同参同判据）
  - targetGuard.ts:64-72 纯函数三参三分支（`stateData===undefined → true` / `echoed → false` / `(courses.length ?? 0) > 0 && hasSelected → true`）与注释逐条对应；`echoed` 第三参只在稳态放行（echoed=true 直接 false）、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界逐字符复核**：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)，合并与 stale 兜底清理均在 :250-294）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)，声明于 echoedRef/rev/setRev/setEchoDone 之后、TDZ 不触发）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
  - 读点全量清点（grep 全量 6 处）：:227（回显 effect 首行短路）、:317（独立清理 effect 守卫）、四消费点（:507/:595/:603/:697）——无第七处；:305/:312 为注释引用非读取
- **target-guard 断言 18/18** 实测全绿（含 echoed 第三参 2 条：稳态编辑不闷死 / 首帧未到 + 已回显标志仍推迟）。
- 第二十二轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :697 stateDataRef + 防抖 effect 依赖 :753（rev/selected/sessionToken/toast/hasPublishes/echoDone/stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行、绝不驱动守卫判据 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖「清理先于回显合并」时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :697-737 + flush :507-558 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 rev>0 且无任何条目不合并、:277 !hasTouched && !merged 保持现状不返新引用、:249 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :730 / flush :551 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 等状态之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测七处，与 R84/R85 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。pick 对象式快照的「跨事件读旧闭包」担忧不成立：React 18/19 离散事件各自独立 flush，两次 click 之间状态已提交渲染，快照语义与函数式等价（注释 :341-345 已论证，toast 副作用移出 updater 规避 StrictMode 双调）。
- **dirtyRef 置位语义清点**（grep 实测六处读写）：:417（pendingUnsaved 三信号读）/ :453（saveNow catch 真实失败置 true）/ :460-461（finally 飞行中标记补发清 false）/ :561（flush 飞行中标记补发置 true）/ :620（handleBack 静止判定读）/ :740（防抖飞行中标记补发置 true）——置 true 仅三处且全部为真实失败/飞行中补发语义，守卫十分支纯 return 零置位，终局 toast（:645 `revRef.current > 0 && pendingUnsaved()`）只对真实失败触发。

## 三、新视角扫查（换方向，五组逐项）

1. **react-query v5 查询键稳定性与失效时机**：
   - 查询键全部含 `account` + `sessionToken`（App 层不涉及）：`["electives", account, sessionToken]`（Select:56 / Dashboard:172 同 key 同 URL）、`["state", account, sessionToken]`（Select:146 / Dashboard:132）、`["logs", sessionToken]`（Dashboard:149）、`["admin-codes", account, sessionToken]` / `["admin-config", ...]` / `["admin-stats", ...]` / `["admin-accounts", ...]` / `["admin-logs", ...]`（Admin:317/501/719/813/908）——账号切换（含 401 吊销自动切号）即 key 变化、缓存天然隔离，无跨账号数据污染；管理员切换目标账号（targetAccount 复用管理员令牌）key 含目标账号名，同样隔离。
   - **invalidate 覆盖全部改数据路径**（grep 实测）：手动报名/退选 `selectElective`/`exitElective` finally 双 invalidate `["electives"]` + `["state"]`（Select:103-104/:128-129，前缀匹配含 account 维度，全账号实例的列表与状态即时失效）；Admin 删除账号后 `["admin-accounts"]`（Admin:262）；激活码生成/删除 `codesQuery.refetch()`（Admin:336/:357）；Config 保存后 `configQuery.refetch()`（Admin:544）——每个改数据动作都有对应查询失效/重拉，无遗漏。
   - 查询客户端默认 `retry: 1, staleTime: 0`（App:12-14）——失败重试一次即停、轮询每次 refetch 均真实打接口，无陈旧缓存复用风险。refetchInterval 函数式回调（Select:58-82/:148-155、Dashboard:140-145/:156-162）失败态降频 30s、window_closed 降频，闭包经 query.state.data 读取（不引组件闭包 state，规避循环推断），无稳定性问题。**结论：查询键与失效时机无缺陷。**
2. **登录/激活/登出失败路径 state 残留**：
   - Login submit：`setLoading(true)` → catch（1001 弹激活窗 / 其他 setError）→ finally `setLoading(false)`——失败路径 loading 必复位；error 在下次提交入口先清（:42）再置（:57），无残留。激活失败：`setActivateError` 分两支清票（票据过期分支引导文案 / 非票据分支同款清票），`setActivating(false)` finally 复位——失败路径 activating 必复位。
   - 登出：App.logout 同步清 inAdmin/adminToken/targetAccount/page + 后端 apiLogout 尽力而为（client.ts:143-149 静默 catch）——本地登出不依赖网络成功，失败路径无 state 残留。onDeleted/onUnauthorized/onBackToStudent 全路径清 adminToken 与代理态（R85 复核过的四处清点）本轮复证无回潮。
   - 管理员五 Tab 数据失败：各 Tab 均有 isError 分支渲染「加载失败 + 重试」卡片/文案（Codes:447-453 / Config:692-699 / Stats:783-789 / Accounts:889-895 / Logs:935-941），数据缺席不误显空态（StatsTab data-first 次序与 LogsTab isLoading 短路延续 R84 复核）——**失败路径 state 残留零缺陷。**
3. **组件卸载后的状态写入（setState-after-unmount）**：
   - Select：卸载标记 unmountedRef（:388-406）全路径守护——saveNow 首行短路（:429）、成功/失败后短路（:440/:445/:457）、防抖 toast 判 `!unmountedRef.current`（:716/:525）、handleBack 终局 toast 在 onDone 卸载前同步完成（:645-651 在 onDone() 之前）。saveNow 的 await 后 setState 均带卸载检查——零残留。
   - Admin：copyTimer 卸载清理（:114-119）；删除账号在飞 `deleting` 为组件卸载前同步链路（onDeleted 通知 App），无卸载后 setState；Tabs unmount 停询（Radix TabsContent 条件渲染卸载 useQuery 停止轮询）。CodesTab generate/remove 的 finally setState 在组件卸载后触发——但 CodesTab 是 Admin 内嵌子组件，其 setState 只在组件自身存活期间有消费者；Admin 卸载（退出管理页）时若生成/删除在飞，setState 落到已卸载子组件——React 18+ 对卸载后 setState 不再告警（已移除该 warning）、无内存泄漏（闭包随组件实例 GC），且按钮 disabled 由 generating/removing 驱动、卸载即无视觉残留。**零缺陷（React 19 卸载后 setState 为无操作、无警告）。**
   - Login：submit/activate 的 finally setState 在组件卸载后触发——Login 只在无会话时渲染（App:346-348），卸载时机仅「登录成功 onLogin 后 App 切换路由」——onLogin 在 try 内、finally 的 setLoading(false) 在组件卸载后执行，同样为 React 19 无操作。**零缺陷。**
4. **无障碍基线抽查（对齐历轮 F7-03/F20-02 修复族）**：
   - 弹窗三处（Select 退选 Modal、Login 激活 Modal、Admin 删除 Modal）均含 role=dialog + aria-modal + aria-labelledby + Esc 关闭（Select:1205-1209、Login:226-238、Admin:216-218），取消按钮 autoFocus（Select:1232、Admin:241）——历轮修复族零回潮。
   - 开关语义化：ConfigTab 激活码 switch role="switch" + aria-checked（Admin:578-579）；密码可见性切换 aria-label + aria-pressed（Login:171-172）；CollapseSection aria-expanded/aria-controls + useId（Dashboard:68-72）；Progress role="progressbar" + aria-valuenow（Progress.tsx:25-27）；搜索框 aria-label（Select:868）——历轮 F7-03/F20-02 修复族全覆盖无回潮。
   - **本轮新发现见发现清单 OBSERVE-86-01**：Admin CodesTab（:374-391）与 ConfigTab（:603-630）的 label 用裸 `<label className>`（无 htmlFor）包裹 Input——无程序化关联（HTML 隐式包裹关系实际上可让点击 label 聚焦 input，但无 for 属性的 label 与 input 无显式 aria 关联；隐式嵌套 HTML 行为上仍可聚焦，仅读屏关联弱）。对比 Login 表单 label 均用 `htmlFor="login-account"/"login-password"`（Login:131/:150）、Select 搜索框 aria-label——Admin 各 Tab 未沿用该模式，属可访问性弱项。
5. **数值格式与 tabular-nums 一致性**：
   - 倒计时：Dashboard 巨幕矩阵天/时/分/秒（:365-395）与 Select 横幅（:848）全含 `tabular-nums`；等宽数字无跳动。
   - 人数/进度：Select 容量统计（:1089 `text-white font-mono tabular-nums`）、Dashboard 指标卡预选目标 `tabular-nums`（:484）、Admin StatsTab 全 rows `tabular-nums`（:762）、AccountsTab 目标与成功列 `tabular-nums`（:844/:858）——全仓数字展示统一 tabular-nums，无遗漏。
   - 例外核验：Dashboard 运行指标「心跳轮询周期 30 秒」（:478）与「频控保护策略」（:504）为纯文案非动态数字、不需要 tabular-nums；日志时间戳/created_at 为等宽 font-mono 文本（mono 天然等宽）——**一致性零缺陷。**

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 评估结论 + 延续项）

**OBSERVE-86-01（新）— Admin 五 Tab 表单 label 与输入框无 htmlFor 程序化关联，表单可访问性弱项**

- 位置：web/src/routes/Admin.tsx CodesTab :374-391（「生成数量（1-100）」/「每个可用次数」裸 label 包裹 Input）、ConfigTab :603-630（「接口地址」/「密钥」/「模型」裸 label 包裹 Input）、:675-682（「识别并发上限」裸 label）。对照 Login.tsx:131/:150 全用 `htmlFor="login-account"/"login-password"` 显式关联、Select.tsx:868 搜索框 aria-label——Admin 各 Tab 未沿用同款程序化关联模式。
- 触发场景推演：裸 `<label>` 包裹 `<input>` 的 HTML 隐式关联下，点击标签仍可聚焦输入框（浏览器原生行为），键盘可达性不受损；但读屏软件对无 for 属性的 label 与 input 的关联识别弱（隐式包裹的 label 关联在多数读屏下仍可用，显式 for 关联是 WCAG 1.3.1 推荐做法），且显式关联在 label 与 input 分离布局（网格/响应式重排）时是唯一可靠方式。真实影响极低——所有 Admin 表单输入均可 Tab 聚焦、均有可读 placeholder。
- 修复建议（低优先级候选）：CodesTab/ConfigTab 的 label 补 `htmlFor` + Input 补对应 `id`（与 Login.tsx:131-145 同款），零行为改变、纯可访问性增强。
- 严重度论证：纯可访问性弱项、无功能/键盘/数据影响（隐式关联保底），低于升级阈值，维持 OBSERVE。

**OBSERVE-85-01（评估，维持 OBSERVE 不升级）**：useTickingCountdown 三处注释（useTickingCountdown.ts:3-5 / Select.tsx:204-206 / Dashboard.tsx:177-180）声称「整页只重渲染倒计时一处、绝不带动整页重建」与实现不符——hook 在路由组件顶层调用，每秒 setNow 触发整个路由组件整树重渲染（React 语义不可绕过：useState 归属宿主组件即重渲染宿主）。本轮专项评估是否提级：① 功能/数据零风险（DOM 差分成本低，基线自 R 早期即存在非本轮回归）；② 抽 `<TickingCountdown target />` memo 叶子组件改动面大（两路由消费处替换 + 相对时间摘要的 nowMs 依赖一并重设计）、当前收益不明显（每秒整树 re-render 对桌面端可忽略、低端移动端理论 jank 无实测佐证）；③ 真实代价是维护者被注释误导——属注释口径分叉（同族「注释先行、实现未跟上」曾多轮修复，如 api 注释 20s/2s 分叉）。裁决：**维持 OBSERVE 不升级**——注释口径分叉应修（改三处注释为如实口径即可，零行为变更），抽 memo 叶子组件留待任何性能触发的改动时一并处理。

**OBSERVE-85-02（延续，确认无新触发面）**：黄金期结尾窗口关闭瞬间（publishes 转空）与用户「最后一次点选」落在同一 400ms 防抖窗口内时，防抖/flush 双闸（发布缺席 + 已有选中）拦截本次保存且不置 dirtyRef、终局 toast 不弹——该批改动静默丢失且无任何反馈。本轮复推：触发需「publishes 非空用户可点选 → 400ms 内转空并保持」同一子秒级竞态（平台清空与点选同帧），可达性极低；且属「绝不假清空」安全方向刻意牺牲（守卫不弹是契约 4 条注释明确承诺）。handleBack 终局对「rev>0 且守卫曾命中」给中性提示的修复候选需新命中标记（或复用 OBSERVE-76-01 等待期反馈一并设计），**无新触发面、维持「续」观察**。

**OBSERVE-84-01（延续）**：Login.tsx:64-101 激活失败（票据已销毁）后激活按钮仍可用、再点发 ticket="" 请求、错误文案从「激活码错误」突变「票据不能为空」——UX 文案困惑、无安全影响（后端 handler.go 空票拒绝 + ConsumeTicket 单次销毁），低优先级候选。维持「续」。

**OBSERVE-83-01（延续）**：Select.tsx:245 `pubs.length === 0` return 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前 tabs.length===0 无编辑入口、rev 恒 0、行为无害），未来开放「窗口关闭后管理目标」入口需一并处理守卫解锁。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界，与 OBSERVE-83-01 关联）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底 handler.go:641-647）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title（Toast.tsx:100-102，Radix Toast.Close 渲染为 button、无默认可访问名）。维持「续」。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：末轮 flush 触发 saveNow 置位 savingRef 同步（首个 await 前）、pendingSaving 必捕获、api 20s abort 保证失败落地先于 21s 兜底，结论维持。

## 五、已核无缺陷清单

- M-1 延续管理（第二十二轮）：shouldDeferSave 三消费点 + while（:507/:595/:603/:697）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:238/:295）+ 首帧不置位边界（:232/:245）逐字符完整、读写点全量清点（置位 3 + 读点 6）零回潮、target-guard 18/18 实测全绿。
- 六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:453/:561/:740 真实失败/飞行中补发语义），零回潮。
- 新视角五组：react-query 查询键含 account+sessionToken 无跨账号污染、改数据路径 invalidate/refetch 全覆盖零遗漏；登录/激活/登出失败路径 loading/error 复位完整（Activating/Loading finally 必复位、error 提交前先清）；卸载后 setState 全为 React 19 无操作/无警告（Select unmountedRef 全路径守护、Admin copyTimer 清理、子组件卸载后 setState 无消费者）；无障碍历轮修复族（弹窗 role=dialog+Esc+autoFocus、switch 语义化、CollapseSection aria、Progress aria）零回潮（新弱项见 OBSERVE-86-01）；tabular-nums 一致性全仓统一（倒计时/人数/进度全含、静态文案不适用除外）。
- R80-R85 修复持续复核：guardBlockedRef 全仓零残留、注释口径（守卫不置 dirtyRef / 20s 超时 / window_closed 三态）无回潮、Admin StatsTab data-first + LogsTab isLoading 短路无回潮、OBSERVE-77-01 归档结论复证（Dashboard:173 与 Select:57 /electives URL 逐字符一致）。
- 后端交叉契约复核：handleSetTargets（handler.go:443-526）accountExists 凭据表校验（:461-476）、无透传对齐核心账号（:478-485）、maxTargetsPerAccount 100（:499-502）与前端构建契约对齐；handleAdminCodes（handler.go:615-684）count 1-100 / uses 1-1000 / 先全量生成后单事务落库无半批滞留、空 body DELETE 不 403（:663-680）与 CodesTab 对齐；windowClosedLocked（scheduler.go:913-934）三判据单源（主判据/时钟 ≥3/幽灵窗口 EmptyProbeRuns≥3）与 WindowClosed()/StateForAccount(:707) 共用、open 单快照复用（:919）——前端 window_opened/window_closed 信号源无分叉。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：轮次前缀标签族（`\bR\d{2}\b|第\s*\d+\s*轮|round\s*\d+|（第\s*\d+\s*次?`）web/src 与 web/scripts 零命中；行号引用族（`\b\d+行|见第\d+|行号|:\d{3,4}`）零命中；XSS 危险模式（dangerouslySetInnerHTML/innerHTML=/eval(/document.write/new Function）零命中；localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级；视觉护栏 audit.mjs 77 项全绿（画布双宽度/玻璃工具类/实心黑洞清零）。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、btn_type 三向、max_count=0 四处同源、倒计时兜底（F39-N1 begin_times 打底）、契约 23 window_closed 三态——复跑全量通过，零差异化、零回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `npx tsc -b --force --pretty false`（强制全量类型检查） | ✅ EXIT 0（无增量缓存漏检） |
| `npm run build`（web/ 下，生产构建 tsc -b + vite build） | ✅ EXIT 0（1948 modules transformed，产物 419.54 kB JS / 41.11 kB CSS 成功落 backend/web/dist） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 77 项全部通过（A/B/C 三组：画布背景 / 玻璃工具类 / 实心黑洞清零） |
| 全仓残留扫描（guardBlockedRef / 轮次标签族 / 行号族 / XSS 危险模式 / TODO-stub） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master` 洁净（含 npm run build 落盘的 backend/web/dist 在内零改动，dist 已被 web/.gitignore 忽略） |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十二轮闭合，见上。
- **OBSERVE-86-01**（新）：Admin 表单 label 无 htmlFor 程序化关联，可访问性弱项（见发现清单）。
- **OBSERVE-85-01**（评估）：useTickingCountdown 注释口径分叉——维持 OBSERVE 不升级；建议修三处注释为如实口径（零行为变更），抽 memo 叶子组件留待性能触发改动一并处理。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态改动静默丢弃——确认仍属「绝不假清空」安全方向刻意牺牲、无新触发面。
- **OBSERVE-84-01**：激活失败后票据空请求文案突变，维持「续」。
- **OBSERVE-83-01 / 77-02**：courses 非空 + publishes 空 + echoedRef 未置位稳态组合 / 窗口关闭无目标管理入口——维持「续」，二者关联，未来开放「关闭后管理目标」入口需一并处理守卫解锁。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01**：归档确认（R84 证 URL 一致、R85 复证、本轮再复证），退出待办观察项。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，本轮延续论证，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=9f17057，master），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round86-frontend-findings.md）。npm run build 的 dist 产物落 backend/web/dist 属 git 忽略路径，工作区仍洁净。
- 走读推断与实测冲突处理：OBSERVE-85-01 提级评估先按 React 19 调度语义实测（useState 归属宿主组件、setNow 必重渲染整树）确认为注释口径分叉、再评估修复候选成本（抽 memo 叶子改动面 vs 修注释零成本）——以实测为准、收益论证后维持 OBSERVE 不升级；OBSERVE-86-01 先按「HTML 隐式 label 包裹可聚焦」走读、再对照 Login 显式 htmlFor 模式确认属「未沿用同款程序化关联」的弱项而非缺陷。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（Admin 表单 label 无 htmlFor）+ 1 条 OBSERVE-85-01 提级评估（维持不升级）+ 延续观察；连续第三十二轮无严重级发现。
