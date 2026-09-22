# Round 85 前端只读审查报告

基线：commit 617bd5a（R84 双 findings + 收尾总结，HEAD）。本轮为 R85 前端只读审查 + M-1 延续管理（第二十一轮），核心为 M-1 第二十一轮 shouldDeferSave 三消费点（判据 + while）全传 echoedRef 第三参、置位三路径 + 首帧不置位边界、target-guard 18/18 复跑；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归 + setSelected 调用点清点 + 三组断言复跑；新视角扫查（Select 倒计时横幅五态与黄金期交互、Dashboard 卡片折叠内存/重渲染、Login 撞名学生管理态、Admin CodesTab 激活码生成 useMemo 依赖、Toast 去重合并 duration 行为）。审查范围：web/src 全部 .ts/.tsx + web/scripts 四脚本 + audit.mjs，交叉核对 backend/internal/{api,scheduler} 相关契约（handleAdminCodes / handleTargets / windowClosedLocked / tick 提交守卫）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑），`git status --short --branch` 为 `## master` 洁净，`git diff HEAD --stat -- web/src web/scripts` 为空，全程零仓库改动（唯一写入为本报告文件）。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR、1 条新 OBSERVE（useTickingCountdown 注释声称的"整页只重渲染倒计时一处"与实现不符——hook 在路由组件顶层调用，每秒 setNow 重渲染整棵组件树）+ 延续观察管理（OBSERVE-77-01 归档确认复证、OBSERVE-84-01 留档延续）。** M-1 延续管理第二十一轮闭合：shouldDeferSave 四消费点（防抖 :697 / flush :507 / handleBack 判定 :595 + while :603）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:238/:295）+ 首帧不置位边界（:232/:245）完整、读写点全量清点零回潮、target-guard 18/18 实测全绿。六防保存链各判据逐条重读与注释对应，setSelected 七调用点（:158/:198/:250/:288/:320/:351/:361）无第三来源，dirtyRef 置 true 仅三处（:453/:561/:740 真实失败/飞行中补发语义）。新视角五项扫查：Select 横幅五态判定次序（window_closed > window_opened > 未知 > 已到点 > 倒计时）逐场景成立，黄金期交互（打底 shadow 小按钮 + 官网 btn_type 主操作 + 修改目标经 hasPublishes 自愈链落库）无缺陷；Dashboard 折叠 unmount 释放、collapse seed 单次、useMemo 依赖稳定；Login 撞名学生管理态判据与 admin-auth 6/6 一致；Admin CodesTab 无任何 useMemo（审查点天然落空，generated 为普通 state、挂载渲染 map 成本可忽略）；Toast 同 title 去重合并保持原 duration（注释语义与实现一致，合并不重置 Radix 计时器、variant 取最新）。契约 20 全仓扫描：轮次标签族/行号族/XSS 危险模式/web 全零命中、localStorage 六处读写全 try/catch 降级、视觉护栏 audit.mjs 全绿。构建验证全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 77 项通过）。连续第三十一轮无严重级发现。

---

## 一、M-1 延续管理（第二十一轮）

- **shouldDeferSave 三消费点 + handleBack while 全传 echoedRef.current 第三参**（grep 实测四处调用，零回潮）：
  - 防抖回调 :697 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :507 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :595 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
  - handleBack while :603 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`（与 :595 同参同判据）
  - targetGuard.ts 纯函数三参三分支（undefined→true / echoed→false / courses 非空 && hasSelected→true）与注释逐条对应；两处消费点（:507/:697）继续延续 R84 确立的"判据由纯数据 stateData 驱动、echoed 只作稳态放行"语义。
- **echoedRef 置位三路径 + 首帧不置位边界完整**：
  - courses 空分支 :238-239（置 true + setEchoDone(true)）
  - 合并完成分支 :295-296（置 true + setEchoDone(true)）
  - 账号复位 :200（置 false + setSelected({}) + setRev(0) + setEchoDone(false)，声明于 echoedRef/rev 等状态之后、TDZ 不触发）
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；:245 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
  - 读点全量清点：:227（回显 effect 首行短路）、:317（独立清理 effect 守卫）、四消费点（:507/:595/:603/:697）——无第五处
- **target-guard 断言 18/18** 实测全绿（含 echoed 第三参 2 条：稳态编辑不闷死 / 首帧未到 + 已回显标志仍推迟）。
- 第二十一轮结论：M-1 稳态语义三消费点与 targetGuard 纯函数实现一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四处消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :697 stateDataRef + 防抖 effect 依赖 :753（rev/selected/sessionToken/toast/hasPublishes/echoDone/stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行、绝不驱动守卫判据 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :316-327 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖"清理先于回显合并"时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :697-737 + flush :507-558 | 五判据（回显未完成/发布缺席/残留旧发布/联查为空/id 漂移）逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :250-279 | 按 publish_id 真合并（已触碰保留现状含空数组、未触碰补旧目标）、:254 rev>0 且无任何条目不合并、:277 !hasTouched && !merged 保持现状不返新引用、:249 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :730 / flush :551 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev/setRev/setEchoDone 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测七处，与 R84 基线一致无新增）：:158（useState 声明）/ :198（account reset）/ :250（回显合并函数式）/ :288（回显内 cleanStale 函数式）/ :320（独立清理 effect 函数式）/ :351/:361（pick 对象式快照）——无第三来源。pick 对象式快照的"跨事件读旧闭包"担忧不成立：React 18 离散事件各自独立 flush，两次 click 之间状态已提交渲染，快照语义与函数式等价（注释 :341-345 已论证，toast 副作用移出 updater 规避 StrictMode 双调）。
- **dirtyRef 置位语义清点**（grep 实测四处）：:453（saveNow catch 真实失败置 true）/ :461（finally 飞行中标记补发清 false）/ :561（flush 飞行中标记补发置 true）/ :740（防抖飞行中标记补发置 true）——置 true 仅三处且全部为真实失败/飞行中补发语义，守卫分支纯 return 零置位，终局 toast 只对真实失败触发。

## 三、新视角扫查（换方向，五项按既有清单逐项）

1. **Select 倒计时横幅五态与黄金期交互**（Select.tsx:815-860）：
   - 判定次序（if-else 链）：!stateData 同步中 → window_closed 已关闭 → window_opened 已开放 → 无 openTimeStr 且无 begin_times 未识别 → cd.isExpired 已到点 → 倒计时。优先级 hierarchy 正确：closed > opened > 未知 > 已到点 > 倒计时，任何状态组合不重叠歧义。
   - 黄金期交互推演：平台开窗瞬间预清空 publishes（probe 记录的真实现象）时，probe 以 InDateRange 判开窗、publishes 空则 opened=false、window_opened 不翻转——横幅在预清空期显"本地已到开窗点，等待平台窗口开放..."，与同屏空态卡「窗口开放后课程列表将自动出现」文案一致，不产生"已开放 + 空态卡"自相矛盾。窗口关闭期 window_closed 优先显"已关闭"，old `|| cd.isExpired` 误显"已开放"路径已在注释 :819-824 钉死移除。
   - 黄金期按钮形态切换 :1143 `t.in_date_range || stateData?.window_opened ? ghost 小按钮 : primary 预选`——开窗后官网报名按钮（btn_type=2 主操作）与后台冲刺目标按钮（ghost 次要）层次正确；目标修改经 setSelected → 防抖 → hasPublishes 自愈链落库（publishes 短暂清空期间守卫拦下、发布恢复 effect 重跑新 timer 补存，绝不丢失）。
   - 文案行 :853-859 begin_times 兜底与 Dashboard 同构（F39-N1 承诺落位）。
2. **Dashboard 卡片折叠内存/重渲染**（Dashboard.tsx:219-273 / 547-668）：
   - CollapseSection `{open && ...}` 条件渲染——折叠即 unmount children，课程卡片无任何本地 hook 状态、DOM 完整释放，无内存残留。
   - `expandedDates` null（未种）/ []（用户全折叠）双哨兵语义：种子 effect :269-273 只在 null 且 dateGroups 非空时种 `[dateGroups[0].key]`，用户全折叠后绝不重拉；窗口关闭后 dateGroups 分组仍由 CourseStatus 自带元数据（契约 3）驱动、无重种。
   - dateGroups/extrasMs useMemo 依赖（courses / electives?.publishes / electives?.begin_times / primaryMs）均为稳定引用，/state 与 /electives 轮询的数据引用不变时重算被跳过。
   - 本项衍生发现见发现清单 OBSERVE-85-01（useTickingCountdown 顶层调用导致整页每秒重渲染，注释声称与实际不符）。
3. **Login 撞名学生管理态**（App.tsx:70-148 / adminAuth.ts 全 / Login.tsx:44-62）：
   - 判据链完整复读：登录响应带 adminName 才 `setInAdmin(true) + setAdminToken(token) + saveAdminToken`；撞名学生（账号名恰等于管理员名、教务登录放行）响应无 adminName → inAdmin=false、adminToken 不清不动；刷新恢复唯靠 `isCurrentAdminSession`（sessions[adminName] === adminToken），撞名学生令牌永远匹配不上标记，六场景 admin-auth-check 6/6 实测全绿。
   - 登出/onUnauthorized/onDeleted/onBackToStudent 全路径清 adminToken（:122/:213/:312/:174）——无残留路径。
4. **Admin CodesTab 激活码生成 useMemo 依赖**：
   - 实测 Admin.tsx 无 useMemo import（仅 useEffect/useRef/useState），CodesTab 内不存在任何 useMemo——该审查点天然落空，无需审计依赖。generated 为普通 state（生成后展示一次、收起置空），`setGenerated(codes)` 引用即最新，全列表 `codesQuery.data.map` 每渲染重建（元素为轻量行、数据量小，memorize 无收益）。生成/删除在飞幂等（generating 布尔 + removing ReadonlySet<string>）与后端 handler.go:615-684 校验（count 1-100 / uses 1-1000 / 先全量生成再单事务落库）对齐。
5. **Toast 去重合并 duration 行为**（Toast.tsx:33-56）：
   - 去重合并只更新 `description` 与 `variant`，`duration` 保留首次挂载配置（`{ ...prev[idx], description, variant }`）——Radix duration prop 不变则计时器不重启、高频合并不无限延寿，与注释 :47-49 逐一对应。
   - variant 取最新（`msg.variant ?? prev[idx].variant`）——截图留证对同 title 合并反馈的新状态视觉即时表征。
   - 去重键为 title 字符串（ReactNode 不合并，本仓全部 toast title 为字符串，无泄漏），自增 id 根除碰撞，合并路径不新增堆叠。

## 四、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮新增 1 条 + 延续项）

**OBSERVE-85-01 — useTickingCountdown 的"只重渲染倒计时一处、绝不带动整页重建"注释与实现不符，实际每秒重渲染整棵路由组件树**

- 位置：web/src/lib/useTickingCountdown.ts:3-5（注释「内部自 tick（每秒 setNow），只重渲染这一个 hook 的消费处，绝不带动整页重建」）、web/src/routes/Select.tsx:204-206（「整页只重渲染倒计时一处」）、web/src/routes/Dashboard.tsx:177-180（「倒计时内部自 tick……不再每秒全量重建」）。
- 触发场景推演：useTickingCountdown 是普通 hook（`const [now, setNow] = useState(Date.now())` + `setInterval(setNow, 1000)`），在 Dashboard/Select 路由组件顶层调用——state 归属路由组件本身，每秒 setNow 触发的是整个路由组件（含 header、超极倒计时矩阵、日期分组折叠段、最多 100 条日志卡、最多数十张课程卡）的完整重渲染与整树 reconciliation，并不是"只有倒计时的消费处"。React 若要实现"整页只重渲染倒计时一处"，需把倒计时拆成独立的 memo 化叶子组件（hook 放到该叶子内部、父组件订阅 context 或传递 target），当前实现从未达到注释声称的隔离。砍掉旧整页 setTick 后每秒重渲染次数与旧方案相同，属"注释先行、实现未跟上"的口径分叉（同族问题此前多轮已修，如 api 注释"2 秒与 20 秒分叉"）。
- 实际影响：无功能/无数据风险；性能上每秒整树 re-render 对桌面端可忽略，低端移动端在黄金期（1s tick + 2s /electives + 2s /state 双轮询叠加约 0.5-1 次/秒重渲染）理论上有 jank 可能，但 DOM reconciliation 差分成本低、基线自 R 早期即存在，非本轮回归。真实代价是维护者被注释误导、误以为该处已做过局部渲染优化。
- 修复建议（低优先级候选）：① 修正三处注释为如实口径（"每秒 tick 触发路由组件整树重渲染，DOM 差分成本可忽略；如需局部化再拆独立倒计时叶子组件"）；② 若确需优化，把倒计时抽成 `<TickingCountdown target={...} />` memo 叶子组件（Dashboard/Select 各自消费处替换），父组件删除 useTickingCountdown 顶层调用。二者均不改变行为。
- 严重度论证：纯注释口径分叉 + 非本轮引入的预存在行为，低于升级阈值，维持 OBSERVE。

**OBSERVE-84-01（延续）**：Login.tsx:64-101 激活失败（票据已销毁）后激活按钮仍可用、再点发 ticket="" 请求、错误文案从「激活码错误」突变「票据不能为空」——UX 文案困惑、无安全影响（后端 handler.go 空票拒绝 + ConsumeTicket 单次销毁），低优先级候选。维持「续」。

**OBSERVE-77-01（归档确认）**：R84 已证 Dashboard.tsx:172 与 Select.tsx:57 的 /electives URL 完全一致（`/electives?account=` + encodeURIComponent(account)，queryKey 亦同 `["electives", account, sessionToken]`），"不同 URL"前提不成立。本轮复证 URL 逐字符一致，维持归档结论，不再作为待办观察项。

**OBSERVE-83-01（延续）**：Select.tsx:245 `pubs.length === 0` return 分支仍在，「courses 非空 + publishes 空 + echoedRef 未置位」稳态组合为潜在陷阱（当前 tabs.length===0 无编辑入口、rev 恒 0、行为无害），未来开放「窗口关闭后管理目标」入口需一并处理守卫解锁。维持「续」。

**OBSERVE-85-02（新，极窄窗口）**：黄金期结尾窗口关闭瞬间（publishes 转空）与用户"最后一次点选"落在同一 400ms 防抖窗口内时，防抖/flush 双闸（发布缺席 + 已有选中）拦截本次保存且不置 dirtyRef、终局 toast 不弹——该批改动静默丢失且无任何反馈。子秒级竞态实际可达性极低（publishes 非空用户可点选 → 400ms 内转空并保持 = 平台清空与点选同帧），且属「绝不假清空」安全方向的刻意牺牲（守卫不弹是契约 4 条注释明确承诺）。修复候选：handleBack 终局对"rev>0 且守卫曾命中（publishes 空）"给一条中性提示"窗口已关闭，本次目标调整未落库"（需新的命中标记，或复用 OBSERVE-76-01 等待期反馈一并设计）。维持「续」观察，定级低于现有 OBSERVE 门槛。

**OBSERVE-78-01 / 83-01 / 76-01 / 76-02 / 76-03 / 75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03（延续）**：handleBack 终局 toast 去重合并文案突变窗口 / Select 无目标管理入口（与 83-01 关联）/ handleBack 保存静默等待期零进度反馈 / Admin CodesTab uses 输入无前端上限（后端兜底）/ Toast Close 无 aria-label / extrasMs 引用重建 / 空态卡与 window_closed 弱相关 / 防抖 selectedCount 闭包快照 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 调用点清点（本轮 grep 实测七处与基线一致）——均维持「续」。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：末轮 flush 触发 saveNow 置位 savingRef 同步（首个 await 前）、pendingSaving 必捕获、api 20s abort 保证失败落地先于 21s 兜底，结论维持。

## 五、已核无缺陷清单

- M-1 延续管理（第二十一轮）：shouldDeferSave 三消费点 + while（:507/:595/:603/:697）全传 echoedRef.current 第三参、echoedRef 置位三路径（:200/:238/:295）+ 首帧不置位边界（:232/:245）完整、读写点全量清点零回潮、target-guard 18/18 实测全绿。
- 六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 七调用点无第三来源，dirtyRef 置 true 仅三处（:453/:561/:740），零回潮。
- 新视角五项：Select 横幅五态判定次序 hierarchy 正确、黄金期按钮形态切换与预清空期文案自洽；Dashboard 折叠 unmount 释放 + collapse seed 单次 + useMemo 依赖稳定；Login 撞名学生管理态六场景与 admin-auth 实测一致、四处清 adminToken 路径无残留；Admin CodesTab 无 useMemo 依赖面可审（generated 普通 state、在飞幂等与后端校验对齐）；Toast 去重合并保持原 duration 不重置计时器、variant 取最新、去重键为 string 无 ReactNode 泄漏。
- R80-R84 修复持续复核：guardBlockedRef 全仓零残留（grep 全量含 backend）、注释口径（:412-415/:501-502/:507 守卫不置 dirtyRef 语义）无回潮、Admin StatsTab data-first 次序（:757-792）、LogsTab isLoading 短路（:922-944）无回潮。
- 后端交叉契约复核：handleAdminCodes（handler.go:615-684）count/uses 校验与 CodesTab 输入（1-100 / 无上限前端兜底）对齐、先全量生成后单事务落库无半批滞留；handleTargets 校验（:482-526）与前端 targets 构造契约对齐、maxTargetsPerAccount 100 门；windowClosedLocked（scheduler.go:913-934）三判据单源与 StateForAccount 共用、/state window_closed 字段完整镜像；tick 守卫（:1006-1021）零值守卫让位于 WindowOpened（契约 32）、关闭后挂起提交不轰炸——前端 window_opened/window_closed 信号源无分叉。
- 三组守护脚本：target-guard 18/18、admin-auth 6/6、unauthorized 5/5，全部实测全绿（18 + 6 + 5 分散复跑确认）。
- 契约 20：轮次前缀标签族（`\bR\d{2}\b|第\s*\d+\s*轮|round\s*\d+|（第\s*\d+次?`）web/src 与 web/scripts 零命中；行号引用族（`\b\d+行|见第\d+|行号|:\d{3,4}`）零命中（Select.tsx:265 的 "XK-XXXX-XXXX-XXXX" 为激活码 placeholder，非行号簇误报）；XSS 危险模式（dangerouslySetInnerHTML/innerHTML=/eval(/document.write/new Function）零命中；localStorage 六处读写（App.tsx:20/28/39/46/57/64）全 try/catch 降级；视觉护栏 audit.mjs 全绿（77 项通过，画布双宽度/玻璃工具类/实心黑洞清零）。
- 登录/激活链（幂等守卫/票据贯通/401 单广播/20s 超时化简）、登出吊销、onDeleted、onUnauthorized、btn_type 三向、max_count=0 四处同源、倒计时兜底（F39-N1）、契约 23 window_closed 三态——复跑全量通过，零差异化、零回潮。

## 六、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下，只读校验） | ✅ EXIT 0（类型全通过，noUnusedLocals 实证无死代码） |
| `node --import jiti/register scripts/target-guard-check.ts`（web/ 下） | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs`（web/ 下） | ✅ 77 项全部通过（视觉护栏，A/B/C 三组全项） |
| 全仓残留扫描（guardBlockedRef / 轮次标签族 / 行号族 / XSS 危险模式 / TODO-stub） | ✅ 零命中 |
| `git diff HEAD --stat -- web/src web/scripts` | ✅ 空（工作区洁净，master，本轮零改动） |

## 七、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第二十一轮闭合，见上。
- **OBSERVE-85-01**（新）：useTickingCountdown 注释口径分叉，低优先级候选（见发现清单）。
- **OBSERVE-85-02**（新，极窄窗口）：黄金期末尾与清空 400ms 防抖竞态的改动静默丢弃，安全方向刻意牺牲，维持「续」观察。
- **OBSERVE-84-01**：激活失败后票据空请求文案突变，维持留档延续。
- **OBSERVE-77-01**：归档确认（R84 证 URL 一致，本轮复证），退出待办观察项。
- **OBSERVE-83-01 / 77-02**：courses 非空 + publishes 空 + echoedRef 未置位稳态组合 / 窗口关闭无目标管理入口——维持「续」，二者关联，未来开放「关闭后管理目标」入口需一并处理守卫解锁。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——维持「续」。
- **可疑-1**（终局 toast 与 onDone 卸载竞态）：物理不可达，本轮延续论证，留存为稳定性契约注释候选。

## 八、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + 守护脚本只读复跑）；工作区 `git status` 洁净（HEAD=617bd5a，master），未修改任何仓库文件（唯一写入为本报告文件，属主控明确指定的输出路径 archive/review-rounds/round85-frontend-findings.md）。
- 走读推断与实测冲突处理：OBSERVE-85-01 先按注释声称的"局部重渲染"推演、后按 React 调度语义实测（hook 顶层调用 setState 必重渲染宿主组件整树）确认为注释口径分叉——以实测为准，不按注释声称下结论。OBSERVE-85-02 触发路径经「pick → setSelected → 防抖 400ms → 平台清空 → 双闸 return → 终局 pendingUnsaved 短路」闭环走读确认。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；1 条新 OBSERVE（useTickingCountdown 注释口径对照）+ 1 条新极窄窗口 OBSERVE + OBSERVE-77-01 归档确认 + 延续观察；连续第三十一轮无严重级发现。