# Round 88 前端只读审查报告

基线：commit 8ee1490（R87 双 findings + 收尾总结，HEAD，进度 88/256）。本轮为 R88 前端只读审查 + M-1 延续管理（第二十四轮），核心为 M-1 第二十四轮 shouldDeferSave 三消费点（判定 + while 全传 echoedRef 第三参、置位三路径 + 首帧不置位边界）逐字符闭合、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归 + setSelected/dirtyRef 清点、F86-01 注释口径修正复核（git 提交范围 0b7a0ba 与工作区三处注释逐字符核对）、OBSERVE-86-01 延续评估（Admin 表单 label 无 htmlFor，低优先级零成本修复族）、新视角扫查（react-query v5 轮询间隔动态调整、表单/弹窗焦点管理与返回路径、ErrorBoundary 缺失、国际/文案一致性、快速切号与往返竞态——本轮聚焦后四组为新方向）、契约 20 全仓扫描、三组断言 + 构建复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleAdminStats / windowClosedLocked / StateForAccount / handleSetTargets）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本只读复跑），`git status --short --branch` 恒为 `## master` 洁净，`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（React ErrorBoundary 缺失，防御性改进低优先级候选）+ OBSERVE-86-01 落地建议（零成本 htmlFor 修复族）+ 延续观察管理。** M-1 延续管理第二十四轮闭合：shouldDeferSave 四消费点（防抖 :699 / flush :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234 前置 return / :247 pubs 空不置位）逐字符完整、读写点全量清点（置位 3 + 读点 6：:229/:319 守卫 + 四消费点，无第七处）、target-guard 18/18 实测全绿。R86→R87 区间前端代码改动仅 F86-01 提交 0b7a0ba（3 文件 13+/7-，注释口径修正），工作区零 diff、三处注释与提交逐字符一致，旧口径（`只重渲染|绝不带动整页|不带动整页|每帧重建`）全仓零残留。新视角五组扫查全部收敛：react-query refetchInterval 动态升降频无泄漏无过度请求（失败态统一 30s 降频、成功态按 window_closed/window_opened 升 2s/3s 降 10s/30s，函数式回调由 react-query 重算下一拍、组件卸载即停）；手写 modal 焦点管理全部为历轮 O-3 同族（无焦点陷阱/无 scroll lock/卸载后焦点落 body，均为已记录观察项，无新缺陷）；ErrorBoundary 缺失（全仓零命中）为渲染期异常白屏隐患——正常路径有充分防御、多年零渲染期异常记录，评 OBSERVE-88-01 低优先级（防御性改进候选，非本轮回归）；文案一致性「预选/冲刺/报名/选课」语义分层层级分明（planning/sprint/manual），零「预约」混用；快速切号/往返竞态防线完整（key={account} 重建 + unmountedRef 全路径守护 + lastJson/savingRef 串行 + 401 按 token 归属反查）。契约 20 扫描零命中。构建验证全绿（tsc -b EXIT 0 / npm run build EXIT 0 产物 419.54 kB JS + 41.11 kB CSS / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 77 项通过）。连续第三十四轮无严重级发现。

---

## 一、M-1 延续管理（第二十四轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处调用，零回潮、逐字符核对）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
  - targetGuard.ts:64-72 三参三分支（`stateData === undefined → true` / `echoed → false` / `(courses.length ?? 0) > 0 && hasSelected → true`）与注释逐条对应；`echoed` 第三参只在稳态放行（echoed=true 直接 false）、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界逐字符复核**：
  - courses 空分支 :240-241（置 true + setEchoDone(true)）
  - 合并完成分支 :297-298（置 true + setEchoDone(true)，合并与 stale 兜底清理均在 :252-296）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)，声明于 echoedRef/rev/setRev/setEchoDone 之后、TDZ 不触发）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return 绝不置位；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
  - 读点全量清点（grep 全量 6 处）：:229（回显 effect 首行短路）、:319（独立清理 effect 守卫）、四消费点（:509/:597/:605/:699）——无第七处；:307/:314 为注释引用非读取
- **target-guard 断言 18/18** 实测全绿（含 echoed 第三参 2 条：稳态编辑不闷死 / 首帧未到 + 已回显标志仍推迟）。
- 第二十四轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（rev/selected/sessionToken/toast/hasPublishes/echoDone/stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行、绝不驱动守卫判据 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖「清理先于回显合并」时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 全清空放行 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 等状态之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测七处，与 R84-R87 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——无第三来源。pick 对象式快照的「跨事件读旧闭包」担忧不成立（React 19 离散事件独立 flush、双击之间状态已提交渲染，注释 :341-345 已论证）。
- **dirtyRef 置位语义清点**（grep 实测六处读写）：:419（pendingUnsaved 三信号读）/ :455（saveNow catch 真实失败置 true）/ :462-463（finally 飞行中标记补发清 false）/ :563（flush 飞行中标记补发置 true）/ :622（handleBack 静止判定读）/ :742（防抖飞行中标记补发置 true）——置 true 仅三处且全部为真实失败/飞行中补发语义，守卫十分支纯 return 零置位，终局 toast（:647 `revRef.current > 0 && pendingUnsaved()`）只对真实失败触发。

## 三、F86-01 注释口径修正复核（第三轮）

- **git 提交范围核验**：`git log --oneline 9f17057..8ee1490 -- web/` 仅命中 0b7a0ba（F86-01），`git diff 9f17057 8ee1490 --stat -- web/` 仅 3 文件 13+/7-（useTickingCountdown.ts +5、Dashboard.tsx +9-4、Select.tsx +6-2）；8ee1490 自身仅动 archive/review-rounds 三报告。当前工作区 `git diff HEAD --stat -- web/src web/scripts` 为空。**前端代码自 R86 基线后零改动（除注释修正），本轮回归面为纯注释层。**
- **三处注释逐字符复核**（工作区 Read + git show 双向核对，与 R87 记录逐字一致）：
  - useTickingCountdown.ts:3-8：「内部自 tick（每秒 setNow）。注意：hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——本注释已按实现如实口径，不再声称局部渲染）。」
  - Select.tsx:204-207：「注意：hook 在路由组件顶层调用，每秒 setNow 触发的是本路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；如需真正做到「只重渲染倒计时一处」需拆独立 memo 叶子组件（潜在优化，非当前承诺）。」
  - Dashboard.tsx:177-181：「注意：hook 在路由组件顶层调用，每秒 setNow 实际触发本路由组件整树重渲染（React 语义不可绕过），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺）。」
- **旧口径零残留**：grep `只重渲染|绝不带动整页|不带动整页|每帧重建` 全仓零命中（Note：三处新口径自身包含「只重渲染倒计时一处」的 memo 潜在优化提法，属正确引用非旧口径，grep 语义为「'绝不带动整页'/『整页只重渲染』」等旧断言，实际零命中）。
- **F86-01 结算论**：OBSERVE-85-01 修复持续有效、零行为变更、零 git 意外改动。观察项保持归档（连续两轮复核无回潮）。

## 四、OBSERVE-86-01 延续评估（Admin 表单 label 无 htmlFor）

- 位置复证：Admin.tsx CodesTab :374-391（「生成数量（1-100）」/「每个可用次数」裸 label 包裹 Input）、ConfigTab :603-630（「接口地址」/「密钥」/「模型」裸 label）、:675-682（「识别并发上限」裸 label）——对照 Login.tsx:131/:150 `htmlFor="login-account"/"login-password"` 显式关联 + Select.tsx:869 搜索框 aria-label，Admin 未沿用同款程序化关联模式（与 R86/R87 记录一致）。
- 更新评估（本轮专项）：该修复为零成本修复族——CodesTab/ConfigTab 共 5 个 label 补 `htmlFor` + 对应 Input 补 `id`（Login 同款，纯 JSX 属性增量、零行为改变、TypeScript 无类型影响、无 eslint 配置冲突），改动面不超过 10 行、风险趋零。触发场景复核维持低影响（隐式 label 包裹下点击标签仍可聚焦输入框、现代主流读屏对隐式关联基本识别，显式 for 是 WCAG 1.3.1 推荐）。**严重度论证：纯可访问性弱项、无功能/键盘/数据影响，仍低于升级阈值维持 OBSERVE；但按「零成本修复族」标准（vs 历轮仅评估不落地），本轮建议主控顺手落地（下轮任一前端改动时段），不建议升 MINOR 强制。**
- 相关 OBSERVE-76-03（Toast Close 无 aria-label/title）：本轮复证仍在（Toast.tsx:100-102 无 aria-label/title/aria-hidden），Radix Toast.Close 默认渲染 button 无默认可访问名——同类低优先级可访问性弱项，维持延续。

## 五、新视角扫查（换方向，五组逐项）

1. **react-query v5 轮询间隔动态调整（refetchInterval 函数式回调）**：
   - Select /electives :58-82 四态：失败→30000 / window_closed→30000 / inRange||window_opened→2000 / else→10000。回归评估：① 升/降频切换是否泄漏——react-query 的 refetchInterval 是调度器内置机制（内部 clear + re-schedule），非组件手写 setInterval，组件卸载即 query 销毁、无泄漏路径（grep 确认 Select/Dashboard/Admin 全部 refetchInterval 均无配套手写 timer/cleanup，即无资源残留）；② 是否过度请求——失败态全站统一 30s 降频（Select /electives :76、/state :153、Dashboard /state :141、/logs :157 四查询同款），黄金期前端 2s 轮询为展示刷新语义（调度器提交冲刺不受前端轮询驱动），无 250ms 级高频请求；③ 闭包陈旧——函数式回调从 query.state.data 读取（不引组件闭包 state，规避循环初始化推断），与 R86/R87 复核一致。
   - Dashboard /state :140-145 三态（失败 30s / window_closed 30s / else 3s）、/logs :156-162 读组件闭包 state 降频（注释 :151-154 论证 /state 刷新驱动组件重渲染 → 回调重建，无停旧值风险）、/electives 固定 30000（:174）；Admin 五 Tab 固定 5000/10000（历轮 O-5 已记录无条件挂载轮询、本轮复证无失败态降频，维持观察）。**结论：动态间隔调度无泄漏、无过度请求（走读推断 + react-query v5 语义）。**
2. **表单/弹窗的焦点管理与返回路径（Esc 关闭后的焦点恢复、Modal 卸载后的 body scroll lock 释放）**：
   - 三处手写 modal（Select 退选 :1201-1250、Login 激活 :223-316、Admin 删除 :210-284）逐一复证：role=dialog + aria-modal + aria-labelledby + Esc keydown（退选/激活/删除三处全有）+ autoFocus 取消按钮（Select:1232、Admin:241）——历轮 F7-03/F20-02 修复族零回潮。
   - 焦点陷阱：三处均无完整 focus trap（Tab 可穿出至背景），历轮 O-3 已记录（F6-02 Radix Dialog 迁移候选注释 Login.tsx:222 仍在）——非新缺陷。
   - **body scroll lock**：grep 确认无任何 `document.body.style.overflow` / `overflow-hidden` 动态注入（仅 App.tsx:184-191 CSS class 语义与 global.css:51-65 body 固定 overflow，属壁纸滚动机制），三处 modal 打开时背景仍可滚动（滚动穿透）——历轮未单独记录，属 O-3 手写 modal 同族缺口（Radix Dialog 迁移时一并由 Radix 内部 scroll lock 解决）。
   - **Esc 关闭后焦点恢复**：三处 Esc 均直接 setState(null) 卸载 modal，不做焦点返回（不把焦点送还打开前元素）——键盘用户 Esc 后焦点落 body 顶部，下一 Tab 从页面开头走；autoFocus 取消按钮在 Esc 卸载时已卸载无聚焦残留。Radix Dialog 迁移解决（含 focus restore）。属 O-3 同族，不单列。
   - **返回路径**：Select handleBack 等待保存链静止（最大 3×21s，OBSERVE-76-01 记录零进度反馈）；Login 激活取消清票据+错误三态；Admin 删除取消清 pendingDelete。返回后组件卸载、react-query 查询销毁。**结论：全部为历轮 O-3 同族（焦点陷阱/滚动穿透/焦点恢复三缺），无新缺陷；建议并入 F6-02 Radix Dialog 迁移一并解决，不单列。**
3. **错误边界（ErrorBoundary 缺失）**：
   - 实测：grep `ErrorBoundary|componentDidCatch|getDerivedStateFromError` 全仓零命中；main.tsx 仅 StrictMode + createRoot 直接挂载 App。React 渲染期异常（含 effect 内同步异常驱动的 ancestry 失败）→ 卸载整树 → 白屏且无任何反馈（无 fallback UI、无错误提示，仅控制台报错）。
   - 触发场景评估：正常路径渲染期防御充分（courses??[]、publishes??[]、c.class_name 条件渲染、Dashboard course_name 兜底 ||、Admin 各 Tab isError 分支全覆盖 data-first 次序）；多年轮审无渲染期异常真实记录。真实白屏候选仅剩「后端契约破坏性变更 → 某字段 undefined → .map/.filter 崩溃」（如 /electives 响应结构变化）。**严重度论证：防御性改进、非本轮回归、无具体触发路径实证，评 OBSERVE-88-01 低优先级（候选：App 外包裹一个最小 ErrorBoundary + 全屏重载提示，约 20 行）。宁缺毋滥原则下不升 MINOR。**
4. **国际/文案一致性（「选课/报名/预约」混用）**：
   - grep `预约` 全仓零命中（UI 无「预约」字样）。语义层级清点：「预选/后台冲刺」=planning 目标设置（Select 按钮「设为预选目标」「设为后台冲刺目标」、Dashboard「预选目标课程」）→「报名」=manual 官网操作（Select:1139「报名」按钮 + toast「报名成功/失败」）→「选课」=广义域（Select:782 标题「选修课程大厅」、Dashboard:283「选课自动化控制中心」）→「冲刺」=golden-sprint 后台语义（Dashboard:650「冲刺提交中」）。各词指向不同层级（目标规划 vs 手动操作 vs 领域名 vs 调度阶段），非同一功能混用。Dashboard 卡状态五态（已确认选课/已满员·退避备选/报名异常/提交中/待命）与后端 markFullLocked 文案「已满员」对齐。**结论：文案一致性良好零缺陷。**
5. **竞态（快速切换账号/快速往返页面的请求乱序）**：
   - 切号：App 双挂载点 key={account}（:293-298/:339-344）+ Select 内部守卫 :195-202（account 变化复位 selected/rev/echoedRef/echoDone）——旧账号 selected 绝不污染新账号。react-query 查询键含 account+sessionToken 天然隔离（select/state 各账号独立缓存）。
   - 快速往返（Select→Dashboard→Select）：onDone → unmountedRef 置位（:403）→ 防孤儿请求/退避 timer/toast（saveNow :431/:440/:445/:457、防抖 toast :718/:527 判 !unmountedRef）。重建后 echo effect 复位（echoedRef=false 重新回显）——后端旧目标重新合并，无覆盖删除。saveNow lastJson 去重 + savingRef 串行 + dirty 补发，多轮 flush 不重复 PUT。
   - 慢请求乱序：401 事件按 session 令牌反查归属账号（client.ts:16-22 + App:191-197），?account= 仅展示线索——管理员代理页自身会话过期不误杀学生账号；旧 PUT 后到覆盖由串行化杜绝；收起 Select 时在飞 API 由 unmountedRef 短路副作用（api 层 20s 超时 abort 兜底）。
   - Dashboard 双查询同时失败：/state 与 /logs 各自降频 30s（相互独立），无联动死锁。**结论：竞态防线完整，零缺陷。**

## 六、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮 1 条新 + 1 条落地建议 + 延续项管理）

**OBSERVE-88-01（新）— React 渲染期异常无 ErrorBoundary 兜底，理论白屏且无反馈**

- 位置：web/src/main.tsx:6-10（createRoot 直接渲染 App，无任何 ErrorBoundary 包裹）；全仓 `ErrorBoundary|componentDidCatch|getDerivedStateFromError` 零命中。
- 触发场景推演：正常路径渲染期防御充分（全部列表数据源 `?? []` 兜底、条件渲染、isError 分支）且 88 轮审查零渲染期异常实证；真实余险 = 后端契约破坏性变更（/electives 响应结构变、字段删除）→ 某 `.map/.filter/.toLowerCase` 在 undefined 上抛错 → React 卸载整树 → 白屏、无 fallback、无提示（仅控制台堆栈）。
- 修复建议（低优先级候选，~20 行）：main.tsx 在 `<StrictMode>` 内包一个最小 ErrorBoundary（class 组件 + componentDidCatch → 全屏「页面渲染异常，请刷新」+ 重载按钮），或与 App 的 QueryClientProvider 之间包一层。零现状行为改变（正常路径不触发），纯防御。
- 严重度论证：防御性改进、非功能缺陷、无真实触发路径实证（88 轮零记录）、UI 全站已有防崩溃条件渲染；低于升级阈值，维持 OBSERVE。

**OBSERVE-86-01（延续，本轮附落地建议）**：Admin 表单 label 无 htmlFor——复证位置无变化，明确建议下轮主控顺手落地（5 个 label 补 htmlFor + 5 个 Input 补 id，Login 同款，~10 行零风险，纯可访问性增强），本轮只读不落地。

**OBSERVE-85-02（延续，确认无新触发面）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——触发需「publishes 非空可点选 → 400ms 内转空并保持」同一子秒级竞态，可达性极低；属「绝不假清空」安全方向刻意牺牲（守卫不置 dirtyRef 是契约 4 条注释明确承诺）。维持「续」。

**OBSERVE-84-01（延续）**：Login.tsx:64-101 激活失败（票据已销毁）后激活按钮仍可用、再点发 ticket="" 请求、文案从「激活码错误」突变「票据不能为空」——低优先级 UX 候选（后端空票拒绝 + ConsumeTicket 单次销毁，无安全影响）。维持「续」。

**OBSERVE-83-01（延续）**：Select.tsx:247 `pubs.length === 0` return 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前 tabs.length===0 无编辑入口、rev 恒 0、行为无害）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界，与 OBSERVE-83-01 关联）。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺，历轮 O-3）**：本轮专项复核归并为单条延续——三处手写 modal 无 focus trap、无 body scroll lock、Esc 卸载后无焦点恢复，全部指向 F6-02 Radix Dialog 迁移单一出口，维持 OBSERVE 不单列。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：末轮 flush 触发 saveNow 置位 savingRef 同步（首个 await 前）、pendingSaving 必捕获、api 20s abort 保证失败落地先于 21s 兜底，结论维持。

## 七、已核无缺陷清单

- M-1 延续管理（第二十四轮）：shouldDeferSave 三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）零回潮、target-guard 18/18 实测全绿。
- 六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:455/:563/:742 真实失败/飞行中补发语义），零回潮。
- F86-01 注释口径修正（第三轮）：git 提交范围确证 R86 后前端仅 0b7a0ba（纯注释）、三处注释逐字符为如实口径、旧口径全仓零残留、工作区零 diff。
- 新视角五组：react-query refetchInterval 动态升降频（失败 30s / 关窗 30s / 升频 2s-3s）无泄漏无过度请求（调度器内置机制 + query.state.data 闭包 + 组件卸载即停）；手写 modal 焦点管理（role/aria/Esc/autoFocus 齐全、focus trap/scroll lock/焦点恢复三缺均归并 O-3 族）；ErrorBoundary 缺失（新 OBSERVE-88-01）；文案语义分层无混用（预选/冲刺/报名/选课各指 planning/sprint/manual/域名层级、零「预约」）；快速切号/往返竞态防线完整（key={account} 重建 + unmountedRef 全路径 + lastJson/savingRef 串行 + 401 按 token 归属反查）。
- 后端交叉契约复核：handleAdminStats（handler.go:874-955）window_opened=WindowOpened()（:930）+ window_closed=WindowClosed()（:954）与学生端 /state 同源、open_time 零值输出空串（:899-902）与前端 StatsTab 三态/识别时间展示对齐；windowClosedLocked（scheduler.go:913-934）三判据单源（主判据/时钟 ≥3 带开放时间已过/幽灵窗口 EmptyProbeRuns≥3）+ open 单快照复用（:919）与 WindowClosed()（:902-906）/StateForAccount(:707-734 双锁 reloginMu+mu 对齐 tokenValidFor)共用——前端 window_opened/window_closed 信号源无分叉；handleSetTargets（handler.go:444-526）accountExists 凭据表校验 + maxTargetsPerAccount 100 + 逐目标校验与前端构建契约对齐。StateForAccount 按账号过滤 courses（:728-733）、st.OpenTimeAfter 有效时刻判定（open_time_known 语义）与 Select/Dashboard openTimeStr 判定一致。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：轮次前缀标签族（`\bR\d{2}\b|第\s*\d+\s*轮|round\s*\d+`）/ 行号引用族（`:\d{3,4}|见第\d+|行号`）/ XSS 危险模式（dangerouslySetInnerHTML/innerHTML=/eval(/document.write/new Function）web/src 与 web/scripts 零命中；localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级（含 getItem 解析 try/catch）；adminAuth.ts 纯函数零存储依赖；guardBlockedRef 全仓零残留。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、btn_type 三向、max_count=0 四处同源、倒计时兜底（F39-N1 begin_times 打底）、契约 23 window_closed 三态、select/exit 在飞 Set 幂等 + 双 invalidate——复跑全量通过，零差异化、零回潮。

## 八、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `cd web && npm run build`（生产构建 tsc -b + vite build） | ✅ EXIT 0（1948 modules transformed，产物 419.54 kB JS / 41.11 kB CSS 成功落 backend/web/dist，dist 属 git 忽略路径） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18（grep -c ✓ 实测 18 条） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（实测 6 条） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（实测 5 条） |
| `node scripts/audit.mjs`（web/ 下） | ✅ 77 项全部通过（实数核对：源文件 13 .tsx + 1 global.css = 14 个 × C 组每文件 5 断言[4 bg-black 类 + 1 min-h-screen] = 70 + A 组 5 + B 组 2 = 77 ✓，历轮 77 项口径吻合） |
| 全仓残留扫描（guardBlockedRef / 轮次标签族 / 行号族 / XSS 危险模式 / 旧倒计时口径） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master` 洁净（含 npm run build 落盘的 backend/web/dist 在内零改动，dist 已被 web/.gitignore 忽略） |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，本轮零改动） |

## 九、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十四轮闭合，见上。
- **OBSERVE-88-01**（新）：ErrorBoundary 缺失，防御性改进低优先级候选（见发现清单）。
- **OBSERVE-86-01**（延续 + 落地建议）：Admin 表单 label 无 htmlFor——建议下轮顺手落地（5 label+5 id，Login 同款，~10 行零风险）。
- **OBSERVE-85-01**（归档维持）：F86-01 注释口径复核连续三轮无回潮，维持归档。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲，无新触发面。
- **OBSERVE-84-01**：激活失败后票据空请求文案突变，维持「续」。
- **OBSERVE-83-01 / 77-02**：courses 非空 + publishes 空 + echoedRef 未置位稳态组合 / 窗口关闭无目标管理入口——维持「续」，二者关联，未来开放「关闭后管理目标」入口需一并处理守卫解锁。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **O-3 族**（focus trap/scroll lock/焦点恢复）：归并单条延续，全部指向 F6-02 Radix Dialog 迁移单一出口。
- **OBSERVE-77-01**：归档确认（连续四轮 URL 一致），退出待办观察项。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，本轮延续论证，留存为稳定性契约注释候选。

## 十、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=8ee1490，master），未修改任何仓库文件（唯一写入为本报告文件 archive/review-rounds/round88-frontend-findings.md）。npm run build 的 dist 产物落 backend/web/dist 属 git 忽略路径，工作区仍洁净。
- 走读推断与实测区分：refetchInterval 调度泄漏评估 / 渲染白屏触发推演 / focus trap 交互为走读推断（标注如上）；M-1 逐字符、注释逐字、三组断言计数、git 提交范围、dirtyRef 三处置位均为实测证据。OBSERVE-88-01 的「白屏」为理论推演非实测触发（88 轮零渲染期异常记录），报告如实标注风险等级为低。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（ErrorBoundary 缺失，低优先级）+ 1 条落地建议（OBSERVE-86-01 htmlFor）+ 延续观察；连续第三十四轮无严重级发现。