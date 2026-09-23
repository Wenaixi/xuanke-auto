# Round 93 前端只读审查报告

基线：commit 03b428a（R92 双 findings + 收尾总结，HEAD，进度 93/256）。本轮为 R93 前端只读审查 + M-1 延续管理（第二十九轮）。核心为 M-1 第二十九轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F92-01 轮次锚点剥离复核（git show 623c1ce 逐 diff + 全文件 grep 零残留）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：键盘焦点可见性全链路横查——唯一新增实质发现 OBSERVE-93-01）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch / SPA 导航能力/轮次锚点族全形态）；三组断言 + audit.mjs + tsc -b + npm run build 复跑。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / handleState / handleAdminCodes / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git log/status/rev-parse/show、`npx tsc -b --pretty false`、`npm run build`、三守护脚本 + audit.mjs 只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round93-frontend-findings.md（不存在，本轮新建）。结束态 `git status --short --branch` = `## master`，`git rev-parse --short HEAD` = 03b428a；build 产物 backend/web/dist 已被 git 忽略（实测 `git check-ignore backend/web/dist` 命中）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 1 条新 OBSERVE（键盘焦点可见性，走读推断）+ 延续观察管理。** M-1 延续管理第二十九轮闭合：shouldDeferSave 三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符复核通过、echoedRef 置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）完整、读写点全量清点（代码级置位 3 + 读点 6）无第七处、target-guard 18/18 实测全绿。六防保存链逐条重读零回潮；setSelected 六调用点与 dirtyRef 置 true 三处（:455/:563/:742）复核零漂移。F92-01 剥离复核通过：git show 623c1ce 仅 5 处注释行内编号锚点（F7 / 32-01×2 / 33-01 / 32-02）剥离、逐字符零行为 diff，全文件 grep 轮次锚点族（`\bF\d+\b|\b\d{2}-\d{2}\b|第 *\d+ *轮|round *\d+|R\d{2}`）零残留。新视角焦点可见性横查发现 OBSERVE-93-01（全站 Button 组件无键盘焦点环——全局 outline:none 已抹掉原生 focus 且 Button 仅 3 处手动补偿，历轮 round91 声称"继承原生 button focus"与 global.css:174 冲突，系审查盲区）。连续第三十九轮无严重级发现。

---

## 一、M-1 延续管理（第二十九轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R92 记录一致）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后，TDZ 不触发；F36-01 兜底守卫）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 排除注释行后代码级 6 处）：:200（写）/ :229（回显 effect 首行短路）/ :240、:297（写）/ :319（独立清理 effect 守卫）/ 四消费点（:509/:597/:605/:699）——代码级置位 3 + 读点 4 消费 + 守卫 2 处、无第七处。本轮另核 :168/:172/:507/:594/:603/:696 等注释性引用均不含代码级读写。
- **target-guard 断言 18/18** 实测全绿（grep -c "✓" = 18，含 echoed 第三参 2 条 :70/:76）。
- 第二十九轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | :26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **unmountedRef 全读写点**（grep 实测）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。

## 三、F92-01 轮次锚点剥离复核

- **git show 623c1ce 逐字符**（实测）：仅 5 处注释行内编号锚点剥离——:168 `F7 修复后` → `修复后`、:172 `（32-01）` → 删除、:612 `33-01：` → 删除、:617 `32-01：` → 删除、:965 `32-02：` → 删除，各行的语义注释本体完全保留（剥离后仍完整可读），+5/-5 与提交统计一致（10 行 diff，5 insertions/5 deletions）。零行为 diff（纯注释，本轮 tsc/build 验证侧面背书）。
- **全文件轮次锚点族零残留**（grep 实测，正则 `\bF\d+\b | \b\d{1,2}-\d{2}\b | 第[ ]*\d+[ ]*轮 | round[ ]*\d+ | R\d{2}`）：web/src + web/scripts 全仓唯一误命中为 Dashboard.tsx:110-118 注释里的日期串 "2026-09-13"（ISO 日期，非轮次锚点）与 target-guard-check.ts:23 `"2026-09-20"`（测试数据 begin_date，同属日期串）——均为日期字面量误报，**零真实轮次锚点残留**。
- **工作区 diff 零意外**：`git status --short --branch` = `## master`（HEAD=03b428a），前端零改动。
- F92-01 复核通过；契约 20「历史残留发现即剥离」意图在本轮起已全形态清干净（含行尾挂号与行首前缀两种历轮扫描盲区形态）。

## 四、新视角扫查（本轮选定：键盘焦点可见性全链路横查）

扫查路径：button/input/textarea 的 focus 样式从 global.css 全局重置 → UI 组件（Button/Input/Tabs）→ 四路由裸元素 → Tabs/Progress/Switch 等 Radix/自绘组件 → 全部按钮实例 className。

1. **全局 reset**（实测）：global.css:168-176「基础交互重置」对 `button, input, select, textarea` 统一 `outline: none`——**把原生按钮 focus 环全量抹掉**，且未在同一块补任何 focus-visible 替代样式。
2. **Button.tsx（全站主按钮）**（实测）：组件 base class 只含 `inline-flex ... active:scale-[0.98] disabled:...`，`focus` 关键字在 Button.tsx 全文件 **0 命中**（grep -c = 0）；variantStyles 全部只有 hover，无 focus-visible 补偿。→ 键盘 Tab 聚焦任一 `<Button>` 时**无任何焦点可见指示**。
3. **Input.tsx**（实测）：`focus:border-[var(--cyan)] focus:ring-1 focus:ring-[var(--cyan)] focus:bg-[var(--surface-soft)]`——有焦点环，正常。
4. **裸按钮**（实测）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/重试（:417/:425/:441/:452/:470/:473/:701）、Login 密码切换（:173）与激活取消（:299）——除 Admin 开关（:583）与 Login 密码切换（:173）自带 `focus-visible:ring-2 focus-visible:ring-white/60` 外，**其余全部无焦点环**。
5. **Radix 组件**（实测）：TabsTrigger/TabsContent 带 `focus-visible:outline-none`（Tabs 组件内部），Radix 自管 roving focus 环在此设计下同样不可见（无 ring 补丁）。
6. **历轮口径冲突实证**：round91 报告称「Button 无自定义 focus 但继承原生 button focus + Radix Slot」→ 该口径与 global.css:174 明文 `outline: none` 直接矛盾——**原生 focus 环已被全局 reset 抹掉，"继承原生 focus"不成立**。这是历轮无障碍横查扫描盲区（历轮 focus 审点只覆盖组件级 focus-visible 存在性，未核 global reset × Button 空补偿的组合）。

**结论**：构成 OBSERVE-93-01（无障碍缺口，走读推断——无运行时错误实证但静态链确凿）。修复极轻：Button.tsx base class 一行补 `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60` 即全站收敛（其余裸按钮另择时处理或归入 F6-02 Radix Dialog 迁移一并清理）。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮新增 1 条 + 延续项管理）

**OBSERVE-93-01（新增，无障碍/键盘焦点可见性，走读推断）**：全站 Button 组件与多数裸按钮在键盘 Tab 聚焦时无任何可见焦点环——global.css:174「基础交互重置」对 button 统一 `outline: none` 抹掉原生 focus，而 Button.tsx 全文件 `focus` 关键字 0 命中（无 focus-visible 补偿），variantStyles/sizeStyles 全部只有 hover。实测范围：Select 9 个 Button、Dashboard CollapseSection 裸按钮、Admin 复制/刷新/删除按钮、Login 主按钮等均无补偿；仅 Admin 开关（:583）与 Login 密码切换（:173）自带 `focus-visible:ring-2`。影响：纯键盘用户/读屏用户无法定位当前元素（WCAG 2.4.7 Focus Visible AA 违例），纯黑白高对比背景下尤甚。**历轮 round91 口径修正**：其「Button 继承原生 button focus」与实际代码冲突（global.css:174 明文 `outline: none`），属历轮无障碍横查盲区。修复建议：Button.tsx base class 加 `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60`（与 Admin/Linux 切换按钮同款样式，一行收敛全站）；裸按钮随 F6-02 Radix Dialog 迁移或另行低优先统一。触发场景：键盘用户 Tab 逐个导航选课大厅/管理后台时焦点不可见，黄金期盲操风险放大。严重度论证：无数据/安全/功能危害，纯键盘可用性缺失，OBSERVE 级（延续历史「UI 增强类立 OBSERVE」口径），建议低优先级落地。

**OBSERVE-92-01（已闭环，R92 落地）**：Select.tsx 5 处轮次编号锚点（F7 / 32-01×2 / 33-01 / 32-02）已于 commit 623c1ce 全部剥离（本轮复核通过），**本条从延续清单移除**。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:697-718）缺「加载中」与「拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined，均显示 "NO RECENT LOGS"。本轮重读位置无变化（:696-719 两态结构原样）、零功能危害维持留档。低优先级候选（一行条件对齐其余列表三态）评估：本轮无触发证据（/logs 每 3s 轮询、失败已 30s 降频，纯展示误导），维持「续」不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分（全站列表 `?? []` 兜底 + isError 分支），本轮无新依据提级。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演无新触发面（末次点选与平台清空同落窗口属子秒级物理窄窗，守卫拦下后 hasPublishes 恢复驱动重试）。维持「续」。

**OBSERVE-84-01（延续）**：激活失败票据空请求文案突变——Login.tsx:87-97 两种失败分支均已清 pendingTicket，但清票据后用户再点「激活并登录」会用空 ticket 再打 /activate（后端拒"激活票据无效"）。F43-N2 已处理误导主链路，残余仅主动重复点击时第二次空请求文案；低优先级 UX 候选。本轮重读 :64-101 确认清票三路径（/票据无效分支 :88 / 通用失败分支 :95 / 取消 :303/:235）与 R92 记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前行为无害：pubs 空时窗口未开/已关、用户无目标改动则无保存链路，非空改动走守卫置脏 + hasPublishes 恢复驱动）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口（与 83-01 关联）。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，全部指向 F6-02 Radix Dialog 迁移单一出口。本轮新增观察：O-3 族的焦点恢复（close 后焦点不归还触发表）与 OBSERVE-93-01 的焦点可见性属同一无障碍横切面，可在 F6-02 迁移时一并收敛。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第二十九轮）：三消费点 + while（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符完整、置位三路径（:200/:240/:297）+ 首帧不置位边界（:234/:247）、读写点全量清点（代码级置位 3 + 读点 4 消费 + 守卫 2 处，无第七处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处（:455 真实失败 + :563/:742 飞行中补发对称对），unmountedRef 七读写点全与「卸载后不 fire/不 toast」契约对齐。
- F92-01 剥离复核：git show 623c1ce 仅 5 处注释内编号剥离零行为 diff；全仓轮次锚点族（含行尾挂号/行首前缀两形态）零残留（唯一误命中为日期串字面量）。
- 数据获取与竞态横查（本轮顺带）：
  - Dashboard /state 3s-30s 函数式轮询（:140-145）与 /electives 30s（:174）/logs 同源降频（:156-161）——window_closed 降频契约与 Select 侧全站对称，无裸 setInterval、react-query 失败态降频优先级正确（query.error ⊃ state.window_closed，:141-143 判据无竞争）。
  - 快速切 Tab/切账号竞态：Select queryKey 含 account+sessionToken，key={account} 整实例重建兜底，App 401 事件按 token 反查归属绝不用闭包 current 误杀（:188-235）——旧响应 vs 新状态一致性由 queryKey 身份维度保证，零串扰。
  - 日期分组本地零点（parseDateKey/localTodayMs :113-124）无时区偏移；相对倒计时（relativeCountdown :94-108）与主矩阵每秒 tick 同拍无额外定时器。
  - Toast 同 title 去重合并防轰炸（Toast.tsx:40-56）、duration 保持首次配置不因合并无限延寿——高频失败/满员退避下不漏关键 toast 不轰炸。
- 后端交叉契约复核（沿用 R92 定位，本轮重读关键点）：handleSetTargets（handler.go:444-526）凭据表 accountExists 校验 → 条数上限 100 → class_id/publish_id/priority 四校验 → Store.SetTargetsForAccount → Sched.SetTargetsForAccount → AppendLog 失败记日志（:522-524）零吞错；handleState（:529-548）透传账号存在性校验 + AccountsWithTargets 兜底；handleAdminCodes（:615-697）count 1-100 / uses 1-1000 后端隔离 (uses 前端无上限为 OBSERVE-75-03 延续)、全量生成→单事务落库防半批滞留；windowClosedLocked（scheduler.go:922-943）三条判据单源（state.WindowClosed / 时钟连续失败≥3+开放时间已过 / 幽灵窗口 EmptyProbeRuns≥3+开放时间已过，open 单快照复用 :928）；StateForAccount（:703-728）同源实时计算 + OpenTimeKnown 过期判定（:718）+ Courses 按账号过滤；ElectivesSnapshotFor（:765-789）目标账号判据 len>0 + 专属帧过期即 (nil,false) 触发真刷新绝不回退全局帧，无目标账号回退全局帧（契约 8 延续）。
- 手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137 全路径）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95 防双发）、btn_type 三向、max_count=0 四处同源、open_time 空格串 V8 解析、Progress aria-valuenow 与 unannounced 语义一致——复跑无回潮。
- 契约 20：轮次前缀标签族 / 行号引用族（`Xxx.tsx:N` 全形态）/ XSS 危险模式（innerHTML/outerHTML/document.write/dangerouslySetInnerHTML/new Function/eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / SPA 导航能力（window.open/location.*/history.*/window.location 零命中、纯组件内存 state 拼 API path）全仓零命中。
- TDD 三组断言：target-guard 18/18（grep -c "✓" = 18）、admin-auth 6/6、unauthorized 5/5 全绿；audit.mjs 77 项全绿（✓=77，✗=0）。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0（TSC_EXIT=0，后台任务实测） |
| `cd web && npm run build` | ✅ EXIT 0（built in 637ms，产物 index-DouJnqTB.js 419.87 kB / index-bmSamphs.css 41.11 kB 落 backend/web/dist，git check-ignore 确认 dist 被忽略） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（grep -c "✓" = 18） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（grep -c "✓" = 6） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（grep -c "✓" = 5） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（grep -c "✓" = 77，✗ = 0） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力/轮次锚点全形态） | ✅ 零命中（唯一误命中为日期串字面量，非锚点） |
| `git status --short --branch` | ✅ `## master`（HEAD=03b428a）；除本报告外零改动 |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十九轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01**（新增，无障碍焦点可见性）：全站 Button 无键盘焦点环——global.css:174 `outline:none` 抹掉原生 focus + Button.tsx 零 focus 补偿，历轮 round91 口径修正；修复极轻（Button base class 一行），维持「续」。
- **OBSERVE-92-01**：已闭环（623c1ce 落地），从延续清单移除。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——走读推断，展示误导，不动作。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地（react-query 5.x QueryErrorResetBoundary 不适用，无提级依据）。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮将焦点可见性横切面并入同出口）。
- **OBSERVE-75-03**（延续）：Admin uses 无前端上限（Infinity→null→后端拒绝路径亦被后端值域校验兜底）。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，延续论证。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 三守护脚本 + audit.mjs + node 纯只读实测）；工作区 `git status` 零改动（HEAD=03b428a，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01（全局 outline reset × Button 零补偿 × round91 口径冲突——静态链确凿但无运行时错误实证，走读推断）、OBSERVE-90-01（日志区三态）、O-3 聚焦陷阱为走读推断；M-1 逐字符、三组断言计数（18/18、6/6、5/5、77）、tsc/build 退出码、契约 20 扫描各类（含轮次锚点 grep、XSS 模式、localStorage try/catch、导航能力）、git show 623c1ce diff、git check-ignore、git rev-parse、后端契约 grep 定位、Dashboard 轮询函数式降频判据重读均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新 OBSERVE-93-01（无障碍焦点可见性）；延续观察管理（OBSERVE-92-01 闭环移出）；连续第三十九轮无严重级发现。