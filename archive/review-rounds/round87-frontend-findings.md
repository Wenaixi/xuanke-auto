# Round 87 前端只读审查报告

基线：commit 0cccc35（R86 双 findings + 收尾总结，HEAD，进度 87/256）。本轮为 R87 前端只读审查 + M-1 延续管理（第二十三轮），核心为 M-1 第二十三轮 shouldDeferSave 三消费点（判定 + while 全传 echoedRef 第三参、置位三路径 + 首帧不置位边界）逐字符闭合、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归 + setSelected/dirtyRef 清点、F86-01 注释口径修正复核（git 提交范围与工作区三处注释逐字符核对）、OBSERVE-86-01 延续提级评估（Admin 表单 label 无 htmlFor）、新视角扫查（useTickingCountdown target 变化校正 effect 与宿主重渲染交互、路由组件定时器 visibilitychange 行为、Toast 并发合并去重、表单提交慢网络 loading、React key 稳定性）、契约 20 全仓扫描、三组断言 + 构建复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 三脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 相关契约（handleAdminCodes / handleSetTargets / windowClosedLocked / handleAdminStats）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + `npx tsc -b --force --pretty false` + `npm run build` + 三守护脚本只读复跑），`git status --short --branch` 恒为 `## master` 洁净，`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、零新 OBSERVE（OBSERVE-86-01 提级评估结论维持不升级）+ 延续观察管理。** M-1 延续管理第二十三轮闭合：shouldDeferSave 四消费点（防抖 :699 / flush :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234 前置 return / :247 pubs 空不置位）逐字符完整、读写点全量清点（置位 3 + 读点 6：:229/:319 守卫 + 四消费点，无第七处）、target-guard 18/18 实测全绿。F86-01 注释口径修正复核：git 提交 0b7a0ba 逐字符核对三处注释（useTickingCountdown.ts:3-8 / Select.tsx:204-207 / Dashboard.tsx:177-181）已全部改为如实口径（每秒 setNow 触发宿主路由组件整树重渲染、React 语义不可绕过、memo 叶子组件标为潜在优化非当前承诺），全仓零残留旧口径（`只重渲染|绝不带动整页|不带动整页|每帧重建` 零命中），git diff 零意外改动。OBSERVE-86-01 提级评估：纯可访问性弱项、无功能/键盘/数据影响（隐式 label 包裹点击可聚焦）、Login 显式 htmlFor 与 Admin 裸 label 的差异不构成缺陷、低于升级阈值，维持 OBSERVE。新视角五组扫查全部收敛：useTickingCountdown target 变化校正 effect 与宿主重渲染交互正确（校正 effect :19-21 独立于每秒 tick，target null→有效/有效→null 过渡无陈显微秒窗口——单次渲染内 state 更新先于读取生效）；路由组件定时器（react-query refetchInterval + setInterval）全部由 React effect 生命周期管理、refetchOnWindowFocus 未开启属有意设计（/state 3s 轮询 + window_opened 升频契约覆盖恢复即时性），无 visibilitychange 堆积/恢复空窗；Toast 并发合并去重正确（同 title 更新文案不新增、duration 保持首次挂载不随高频合并无限延寿、变体取最新）；表单提交慢网络 loading 全部幂等（login/activate/select/exit/generate/remove/save 七处 loading 在飞短路 + disabled 双闸）；React key 稳定性（课程 id / publish_id / 激活码 / 账号名 / 日志 id / 日期键 / 时间戳）在数据重取后保持稳定、滚动位置与折叠状态零意外重置。契约 20 全仓扫描：轮次前缀标签族/行号引用族/XSS 危险模式/web/src 与 web/scripts 零命中、localStorage 读写全 try/catch 降级（App.tsx:18-68 六处 + adminAuth 无存储依赖）。构建验证全绿（tsc -b EXIT 0 / tsc -b --force EXIT 0 / npm run build EXIT 0 产物 419.54 kB JS + 41.11 kB CSS / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 77 项通过）。连续第三十三轮无严重级发现。

---

## 一、M-1 延续管理（第二十三轮）

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
- 第二十三轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

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

- **setSelected 调用点清点**（grep 实测七处，与 R84/R85/R86 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——无第三来源。pick 对象式快照的「跨事件读旧闭包」担忧不成立：React 18/19 离散事件各自独立 flush，两次 click 之间状态已提交渲染，快照语义与函数式等价（注释 :341-345 已论证，toast 副作用移出 updater 规避 StrictMode 双调）。
- **dirtyRef 置位语义清点**（grep 实测六处读写）：:419（pendingUnsaved 三信号读）/ :455（saveNow catch 真实失败置 true）/ :462-463（finally 飞行中标记补发清 false）/ :563（flush 飞行中标记补发置 true）/ :622（handleBack 静止判定读）/ :742（防抖飞行中标记补发置 true）——置 true 仅三处且全部为真实失败/飞行中补发语义，守卫十分支纯 return 零置位，终局 toast（:647 `revRef.current > 0 && pendingUnsaved()`）只对真实失败触发。

## 三、F86-01 注释口径修正复核

- **git 提交范围核验**：`git show 0b7a0ba --stat` 确证 F86-01 仅动 3 文件 13 增 7 删（useTickingCountdown.ts +5、Dashboard.tsx +9-4、Select.tsx +6-2）；`git diff 9f17057 0cccc35 --stat` 显示 R86 收官提交仅含 archive/review-rounds/ 三报告 + backend 托盘修复（main.go/quit_shared.go/tray_*）——**前端代码在 R86 收官提交中零改动**（useTickingCountdown/Dashboard/Select 的改动全部属 F86-01 提交 0b7a0ba），当前工作区无未提交 diff。
- **三处注释逐字符复核**（已读工作区当前文件 + git show 提交 diff 双向核对）：
  - useTickingCountdown.ts:3-8：「内部自 tick（每秒 setNow）。注意：hook 在路由组件顶层被消费，每秒 setNow 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——本注释已按实现如实口径，不再声称局部渲染）。」
  - Select.tsx:204-207：「注意：hook 在路由组件顶层调用，每秒 setNow 触发的是本路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；如需真正做到「只重渲染倒计时一处」需拆独立 memo 叶子组件（潜在优化，非当前承诺）。」
  - Dashboard.tsx:177-181：「注意：hook 在路由组件顶层调用，每秒 setNow 实际触发本路由组件整树重渲染（React 语义不可绕过），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺）。」
- **旧口径零残留**：grep `只重渲染|绝不带动整页|不带动整页|每帧重建` 全仓零命中（旧口径语句「绝不带动整页重建」「整页只重渲染倒计时一处」已全部清除）。三处新口径均明确「每秒 setNow 触发宿主路由组件整树重渲染（React 语义）」「DOM 差分成本可忽略」「memo 叶子组件=潜在优化非当前承诺」三要素，与实现逐字符一致。Dashboard:91-92 折叠行注释（「lib effect 无条件 setInterval(1000)」）与新口径语义一致（不再声称局部渲染）。
- **F86-01 闭合结论**：OBSERVE-85-01 修复落地完整、零行为变更（仅注释）、零 git 意外改动，建议将该观察项从延续清单移入归档（见第七节）。

## 四、新视角扫查（换方向，五组逐项）

1. **useTickingCountdown target 变化校正 effect 与宿主重渲染交互**：
   - 校正 effect（useTickingCountdown.ts:19-21 `useEffect(() => setNow(Date.now()), [target])`）独立于每秒 tick effect（:12-15 `setInterval`），每次 target 引用变化（null→有效开窗时刻 / 有效→null / 有效→不同有效时刻）触发一次 setNow 校正，恰好在渲染 diff 计算前把 now 拉回当前时刻——注释 :16-18 声称的「target 从 null 变为有效开窗时刻最多 1 秒陈旧偏差」修复落地正确。target 过渡对宿主重渲染的额外成本为「每次 target 变化多一次宿主渲染」——target 来源是 stateData.open_time 或 electives.begin_times[0]，属低频（识别槽建立/窗口切换时变化一次），叠加在每秒 tick 上可忽略。
   - **陈显微秒窗口推演**：target null→有效时，校正 effect 在提交后 effect 阶段 setNow；diff 计算用旧 now（可能已过期 1 秒内）——React 会在 effect 触发 setState 后重渲染一次，第二次渲染 diff 用新 now，最终显示正确。理论上存在「首次渲染用旧 now 计算出 isExpired 全 00 → 校正后重渲染显示正确」的瞬时闪烁（不超过一帧），但校正 effect 在同一提交批内完成、浏览器 paint 前已重渲染——**无可见陈显窗口**（React 18/19 对 effect 内同步 setState 在 passive effect flush 阶段即合并渲染，不会让中间态 paint）。有效→null 过渡：diff 计算 target=null → 直接返回全 00 + isExpired=true（不抛错、不显示编造时间），setNow 校正同步执行——无越界。**结论：target 校正交互无缺陷（走读推断 + React 调度语义）**。
2. **路由组件定时器 visibilitychange 行为**：
   - 全部轮询走 react-query `refetchInterval`（Select /state 2s、/electives 升频 2s 降频 10s/30s、Dashboard /state 3s、/logs 3s、/electives 30s、Admin 各 Tab 5s/10s）——react-query 在组件卸载时停止、document 不可见时后台 interval 仍继续（React 18/19 对后台 tab 的 interval 由浏览器节流到最低 1s/几分钟，但请求频率已由 refetchInterval 收敛）。**页面切后台堆积推演**：react-query 轮询是「每拍一次 refetch」非累积队列，后台不堆积请求（浏览器节流 interval + 单飞 refetch 语义）；恢复前台时下一拍 interval 立即触发 refetch（不等待旧节流周期）——**无堆积、无恢复空窗**。Select 升频契约依赖 window_opened 信号（/state 2s 轮询），后台期间开窗由服务端调度器持续探测、前台恢复后 2s 内拿到最新 window_opened 高亮——**覆盖恢复即时性**。useTickingCountdown 每秒 setInterval 后台节流后恢复立即重渲染（校正 effect 非 target 变化不触发，但 interval 恢复即 setNow 校准）——倒计时恢复准确。
   - **refetchOnWindowFocus 未显式开启（react-query 默认 true 会重新聚焦即 refetch）**——本应用轮询间隔已覆盖恢复即时性，且 refetchOnWindowFocus 由全局 defaultOptions（App.tsx:12-14）未改默认、继承 react-query true 默认——聚焦即额外 refetch 一次，与轮询叠加无冲突（幂等 GET）。**结论：定时器 visibilitychange 行为零缺陷**。
3. **Toast 系统并发批量合并/去重**：
   - Toast.tsx:40-54 同 title 去重合并：已存在同 title toast 时只更新 description + variant（`merged[idx] = { ...prev[idx], description: msg.description, variant: msg.variant ?? prev[idx].variant }`）——**duration 保持首次挂载配置**（注释 :47-49 明示「Radix duration 变化会重启计时器，覆盖会让高频合并无限延寿」），高频合并只更新文案不重置计时器；variant 取最新（视觉即时反馈）。
   - **并发 toast 风暴推演**：目标保存失败退避链 46s 内最多 6 个同文红 toast 全被合并成一条（title「目标保存失败」恒定）；多课程同时失败（不同 title）各自独立堆叠——Radix Toast viewport 有 max-h-[80vh] 滚动（Toast.tsx:105），极端堆叠视觉可滚不溢出。批量保存链（flush/handleBack 三连 flush）每次成功 toast 只弹一次（saveNow 静默成功 + lastJson 去重跳过重复 PUT）。id 自增计数器（:31）无 Math.random 碰撞。**结论：Toast 并发合并/去重零缺陷**。
4. **表单提交防抖/节流慢网络 loading 与可点性**：
   - Login submit :31-62：`if (loading) return` 幂等短路 + `disabled={loading}` 双闸；慢网络下按钮显示「正在连接教务认证...」不可点。activate :64-101 同款（activating 双闸）。Select handleSelectClass/handleConfirmExit :86-137：actionLoading Set 按课程独立跟踪 + `disabled={actionLoading.has(c.id)}` 双闸；慢网络下本课程按钮显示「报名中.../退选中...」不可点、其他课程不受影响（Set 结构优势）。
   - Admin CodesTab generate/remove、ConfigTab save、AccountsTab 删除：全带 loading 幂等短路 + disabled 双闸；ConfigTab save 额外 `!loaded` 锁定（配置加载完成前绝不保存）。
   - **慢网络可点性评估**：全表单在飞期间按钮 disabled + 文案反馈，无「双发重复请求」路径（入口幂等短路兜底 disabled 渲染延迟）；取消路径（退选 Modal 取消/激活取消）在飞期间 disabled 防误关。**零缺陷**。
5. **React key 稳定性对子组件状态保留的影响**：
   - 列表项 key 清点：课程卡片 key=c.id（Select:1020，course id 稳定，数据重取后同课同 key——选中状态在 selected state 而非组件内部，key 稳定不重置）、TabsTrigger key=t.publish_id（:936，发布 id 稳定）、TabsContent key=t.publish_id（:984）、Dashboard 日期组 key=g.key（日期字符串稳定）、发布组 key=pub.publish_id、课程卡 key=c.class_id（:595）、Admin 激活码 key=c.code、账号行 key=a.account、日志 key=l.id。
   - **数据重取后状态保留推演**：react-query refetch 返回新数组但同 id 元素 key 不变 → React reconcile 保留 DOM 子树与组件状态（搜索词/折叠态/滚动位置不重置）；唯一 key 集合变化场景是发布集合重建（publish_id 全变，TabsTrigger key 全换 → activeTab 受控回落 tabs[0]，:929 `value={activeTab && tabs.some(...) ? activeTab : String(tabs[0].publish_id)}` 兜底——Select 注释 :185-189 已论证「发布重建即回落首个 Tab，绝不悬空」）。**结论：key 稳定性零缺陷**。

## 五、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮零新增，OBSERVE-86-01 提级评估 + 延续项管理）

**OBSERVE-86-01（提级评估，维持 OBSERVE 不升级）— Admin 五 Tab 表单 label 与输入框无 htmlFor 程序化关联**

- 位置：web/src/routes/Admin.tsx CodesTab :374-391（「生成数量（1-100）」/「每个可用次数」裸 label 包裹 Input）、ConfigTab :603-630（「接口地址」/「密钥」/「模型」裸 label 包裹 Input）、:675-682（「识别并发上限」裸 label）。对照 Login.tsx:131/:150 全用 `htmlFor="login-account"/"login-password"` 显式关联、Select.tsx:870 搜索框 aria-label——Admin 各 Tab 未沿用同款程序化关联模式。
- 触发场景推演（本轮专项复核）：裸 `<label>` 包裹 `<input>` 的 HTML 隐式关联下，点击标签仍可聚焦输入框（浏览器原生行为，键盘可达性不受损）；读屏对隐式包裹 label 的关联识别在现代主流读屏（NVDA/VoiceOver）下基本可用，显式 for 关联是 WCAG 1.3.1 推荐做法、在 label 与 input 分离布局（网格/响应式重排）时是唯一可靠方式。Admin 全部表单输入均可 Tab 聚焦、均有可读 placeholder，label 文案与输入框视觉紧邻——**真实影响极低，低于升级阈值**。
- 修复建议（低优先级候选，与前轮一致）：CodesTab/ConfigTab 的 label 补 `htmlFor` + Input 补对应 `id`（与 Login.tsx:131-145 同款），零行为改变、纯可访问性增强。
- 严重度论证：纯可访问性弱项、无功能/键盘/数据影响（隐式关联保底），且不构成本轮回归（历轮一直如此），维持 OBSERVE。

**OBSERVE-85-01（归档）**：F86-01 三处注释已修（见第三节逐字符复核），观察项建议移出延续清单归档。

**OBSERVE-85-02（延续，确认无新触发面）**：黄金期结尾窗口关闭瞬间（publishes 转空）与用户「最后一次点选」落在同一 400ms 防抖窗口内时，防抖/flush 双闸（发布缺席 + 已有选中）拦截本次保存且不置 dirtyRef、终局 toast 不弹——该批改动静默丢失且无任何反馈。本轮复推：触发需「publishes 非空用户可点选 → 400ms 内转空并保持」同一子秒级竞态，可达性极低；且属「绝不假清空」安全方向刻意牺牲（守卫不弹是契约 4 条注释明确承诺）。无新触发面，维持「续」观察。

**OBSERVE-84-01（延续）**：Login.tsx:64-101 激活失败（票据已销毁）后激活按钮仍可用、再点发 ticket="" 请求、错误文案从「激活码错误」突变「票据不能为空」——UX 文案困惑、无安全影响（后端 handleActivate 空票拒绝 + ConsumeTicket 单次销毁），低优先级候选。维持「续」。

**OBSERVE-83-01（延续）**：Select.tsx:247 `pubs.length === 0` return 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前 tabs.length===0 无编辑入口、rev 恒 0、行为无害），未来开放「窗口关闭后管理目标」入口需一并处理守卫解锁。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间 Select 无目标管理入口（设计边界，与 OBSERVE-83-01 关联）。维持「续」。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈。维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端兜底 handler.go:641-647 uses 1-1000）。维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮无 aria-label/title（Toast.tsx:100-102，Radix Toast.Close 渲染为 button、无默认可访问名）。维持「续」。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：末轮 flush 触发 saveNow 置位 savingRef 同步（首个 await 前）、pendingSaving 必捕获、api 20s abort 保证失败落地先于 21s 兜底，结论维持。

## 六、已核无缺陷清单

- M-1 延续管理（第二十三轮）：shouldDeferSave 三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）逐字符完整、读写点全量清点（置位 3 + 读点 6）零回潮、target-guard 18/18 实测全绿。
- 六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:455/:563/:742 真实失败/飞行中补发语义），零回潮。
- F86-01 注释口径修正：git 提交 0b7a0ba 仅动 3 文件、三处注释（useTickingCountdown.ts:3-8 / Select.tsx:204-207 / Dashboard.tsx:177-181）逐字符为如实口径（整树重渲染 + React 语义不可绕过 + memo 潜在优化）、旧口径全仓零残留（`只重渲染|绝不带动整页|不带动整页|每帧重建` 零命中）、R86 收官提交 0cccc35 前端零改动、工作区零 diff。
- 新视角五组：useTickingCountdown target 校正 effect（:19-21）交互正确、null→有效/有效→null 过渡无陈显微秒窗口（React 提交批内合并渲染不 paint 中间态）；路由组件定时器全由 effect 生命周期管理、无 visibilitychange 堆积/恢复空窗（react-query 单飞 refetch + 轮询契约覆盖恢复即时性）；Toast 并发合并去重正确（同 title 更新文案不新增、duration 保持首次挂载不无限延寿、id 自增无碰撞、viewport 可滚）；表单提交七处 loading 幂等短路 + disabled 双闸全覆盖、慢网络无双发路径；React key 稳定性全列表正确（课程 id/publish_id/激活码/账号名/日志 id/日期键全稳定、发布重建 activeTab 受控回落兜底）。
- 后端交叉契约复核：handleSetTargets（handler.go:443-526）accountExists 凭据表校验（:461-476）、无透传对齐核心账号（:478-485）、maxTargetsPerAccount 100（:499-502）+ 逐目标 class_id/publish_id/priority 校验（:503-516）与前端构建契约对齐；handleAdminCodes（handler.go:615-684）count 1-100 / uses 1-1000 / 先全量生成后单事务落库无半批滞留、空 body DELETE 明确错误不 403（:662-680）与 CodesTab 对齐；windowClosedLocked（scheduler.go:913-934）三判据单源（主判据/时钟 ≥3 带开放时间已过/幽灵窗口 EmptyProbeRuns≥3）+ open 单快照复用（:919）、与 WindowClosed()（:902-906）/StateForAccount 共用——前端 window_opened/window_closed 信号源无分叉；handleAdminStats（handler.go:874-955）window_opened=WindowOpened()（:930）+ window_closed=WindowClosed()（:954）与学生端 /state 同源、open_time 零值输出空串（:899-902）与前端 StatsTab 三态展示（Admin.tsx:724-745）对齐。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿。
- 契约 20：轮次前缀标签族/行号引用族/XSS 危险模式（dangerouslySetInnerHTML/innerHTML=/eval(/document.write/new Function）web/src 与 web/scripts 零命中；localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级（注释 :17 明示隐私模式/配额受限静默降级内存态）、adminAuth.ts 纯函数零存储依赖；视觉护栏 audit.mjs 77 项全绿（画布双宽度/玻璃工具类/实心黑洞清零）。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时）、登出吊销、onDeleted、onUnauthorized、btn_type 三向、max_count=0 四处同源、倒计时兜底（F39-N1 begin_times 打底）、契约 23 window_closed 三态——复跑全量通过，零差异化、零回潮。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `npx tsc -b --force --pretty false`（强制全量类型检查） | ✅ EXIT 0（无增量缓存漏检） |
| `npm run build`（web/ 下，生产构建 tsc -b + vite build） | ✅ EXIT 0（1948 modules transformed，产物 419.54 kB JS / 41.11 kB CSS 成功落 backend/web/dist） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 77 项全部通过（A/B/C 三组：画布背景 / 玻璃工具类 / 实心黑洞清零） |
| 全仓残留扫描（guardBlockedRef / 轮次标签族 / 行号族 / XSS 危险模式 / TODO-stub / 旧倒计时口径） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master` 洁净（含 npm run build 落盘的 backend/web/dist 在内零改动，dist 已被 web/.gitignore 忽略） |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，本轮零改动） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十三轮闭合，见上。
- **OBSERVE-86-01**（提级评估，维持 OBSERVE）：Admin 表单 label 无 htmlFor 程序化关联——纯可访问性弱项、无功能影响，低于升级阈值；建议后续与任意可访问性改动一并补 htmlFor+id（Login 同款）。
- **OBSERVE-85-01**（归档）：F86-01 注释口径修正复核通过，建议移出延续清单。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态改动静默丢弃——确认仍属「绝不假清空」安全方向刻意牺牲、无新触发面。
- **OBSERVE-84-01**：激活失败后票据空请求文案突变，维持「续」。
- **OBSERVE-83-01 / 77-02**：courses 非空 + publishes 空 + echoedRef 未置位稳态组合 / 窗口关闭无目标管理入口——维持「续」，二者关联，未来开放「关闭后管理目标」入口需一并处理守卫解锁。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **OBSERVE-77-01**：归档确认（R84/R85/R86 三连证 URL 一致），退出待办观察项。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，本轮延续论证，留存为稳定性契约注释候选。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npx tsc -b --force --pretty false` + `npm run build` + 三守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=0cccc35，master），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round87-frontend-findings.md）。npm run build 的 dist 产物落 backend/web/dist 属 git 忽略路径，工作区仍洁净。
- 走读推断与实测冲突处理：OBSERVE-86-01 提级评估先按「HTML 隐式 label 包裹可聚焦」走读、再对照 Login 显式 htmlFor 模式确认属「未沿用同款程序化关联」的弱项而非缺陷、评估真实影响（现代读屏隐式关联基本可用）后维持 OBSERVE；useTickingCountdown target 校正交互按 React 18/19 调度语义走读（effect 内同步 setState 提交批内合并、不 paint 中间态）确认无陈显窗口——属走读推断，标注如上；F86-01 注释复核用 git show + 工作区 Read 双向逐字符实证（非推断）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；零新 OBSERVE（1 条提级评估维持 + 1 条归档建议 + 延续观察）；连续第三十三轮无严重级发现。
