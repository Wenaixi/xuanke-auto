# Round 97 前端只读审查报告

基线：commit 57ec7cb（R96 双 findings + 收尾总结，HEAD，进度 97/256）。本轮为 R97 前端只读审查 + M-1 延续管理（第三十三轮）。核心为 M-1 第三十三轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 延续管理；新视角扫查（本轮选定：状态更新一致性（ref 镜像同步边界）/ 表单交互键盘可达性 / 视觉状态反馈层次 / 深链与浏览器行为）；契约 20 扫描（轮次前缀标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级）。审查范围：web/src 全部 .ts/.tsx（7 组件 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs），交叉核对 backend/internal/{api,scheduler} 契约（handleSetTargets / handleState / windowClosedLocked / StateForAccount）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log/check-ignore、`npx tsc -b --pretty false`、`npx vite build`、三守护脚本 + audit.mjs 只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round97-frontend-findings.md。结束态 `git status --short --branch` = `## master`；build 产物落 backend/web/dist（git check-ignore 实测 `backend/web/dist/web` 被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十三轮闭合：四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过：Button.tsx:42 ring 行在位、语义正确（键盘触发/鼠标不显示/全站 Button 收敛）、与 Admin 开关/Login 密码切换/Input 自带焦点样式共存无冲突、残余面与 R93-R96 清单逐项一致。六防保存链逐条重读零回潮；setSelected 六代码调用点与 dirtyRef 置 true 仅三处（:455/:563/:742）复核零漂移。git log 实测自 R93（1dfd116）以来 `web/` 目录零代码提交（仅 docs 提交），所有复核点在 R93-R96 定位的同一行号上。连续第四十三轮无严重级发现。

---

## 一、M-1 延续管理（第三十三轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R96 记录逐行一致）：
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  - flushTargets :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；echoed 第三参只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。测试 18/18 中 echoed 第三参 2 条覆盖稳态放行与首帧未到仍推迟两极端。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——全 src 目录 `shouldDeferSave(` 调用点 grep 仅命中此四处 + targetGuard.ts 自身定义 + scripts/target-guard-check.ts 测试断言；echoedRef 读点与 R96 完全一致（grep 全量命中含注释核对无漂移）。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项，含 echoed 第三参 2 条）。
- 第三十三轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 四消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用 + :343/:347 注释引用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫十分支纯 return 零置位。零回潮。
- **unmountedRef 全读写点**（grep 实测 8 处）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。
- **消费时刻读最新 ref**（grep 实测）：selectedRef（:173-174）/ revRef（:175-176）/ stateDataRef（:180-181）/ publishesRef（:663-664）四镜像的消费点全部位于防抖回调/flush/handleBack 消费时刻；pick 直接用渲染 selected（:349/:353/:363）因其「事件处理器内 selected 恒为最近已提交渲染值」论证（:346-347）成立。零旧闭包路径。

## 三、F93-01 Button focus-visible 持续复核

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R96 记录逐字符一致。git log 实测 R93（1dfd116）→ HEAD 无任何 `web/` 提交（`git diff 1dfd116 HEAD --stat -- web/` 空），该行未被改动。
- **语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发。global.css:174「基础交互重置」对 button 统一 `outline: none`（本轮重读逐字在位），ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处经 base class 一次收敛；Admin 开关（Admin.tsx:583 `focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login.tsx:173 同款）、Input 自带 focus ring（Input.tsx:14 `focus:ring-1 focus:ring-[var(--cyan)]`）并存无冲突。Button ring 用 `--cyan`（= #ffffff 纯白）+ offset 2px，既有用 `ring-white/60`（半透明白）无 offset——二者均清晰可见，纯参数差异（已归 OBSERVE-93-01 备注，不单独立条）。
- **残余面确认**（与 R93-R96 清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。本轮逐一 grep 复核这些行号对应裸 button 仍无 ring（与历轮清单一致）。OBSERVE-93-01 延续。
- F93-01 第五轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新视角扫查（本轮选定 4 项）

### 1. 状态更新的一致性（React 状态 vs ref 镜像同步边界 + 异步闭包捕获残余风险）

- **四镜像渲染期同步**（Select :173-181 / :663-664）：`xxxRef.current = xxx` 均位于渲染体顶部、声明即赋值，紧贴各自 useState/useQuery 声明——渲染期同步 ref 是「最后一次 committed render 值」语义，与 React 官方指引的差异点是「渲染期写 ref」属反模式，但赋值幂等、无副作用（不触发重渲染），本项目已历轮注释（:172/:177/:179）轮内论证，属可接受实践而非缺陷。
- **消费时刻读 ref vs 渲染期闭包捕获**（grep 实测）：全部 async 等待循环（flushTargets/handleBack while/saveNow finally 补发）消费点读 ref；pick 事件处理器直读渲染 selected 的论证（两次点击间必有渲染提交)成立——React 事件批处理只在单事件内合并多次 setState，双击是两次独立事件、中间必然有渲染提交。零「async 闭包捕获旧值覆盖新改动」剩余路径。
- **stateData 变化驱动 effect 重跑**（:755 依赖含 stateData）：防抖 effect 依赖 [rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]——F42-M1 的「数据到达触发重跑自愈」语义在位。hasPublishes（:662）布尔稳定（publishes 非空期间不变）绝不重置 400ms 窗口（F10 契约延续）。
- **handleBack while 等待期间的用户新改动**（走读推演）：等待循环内用户点 pick → setSelected/setRev → 下一渲染 selectedRef/revRef 同步 → 循环退出后三轮 flush 读最新 ref 全量提交（:616-640），随后 :622-624 等一帧复查 revRef「与本轮 flush 消费一致」才 break——旧改动被新 flush 追加覆盖的乱序窗口不存在。
- 结论：ref 镜像同步边界完整，无新增发现。

### 2. 表单交互的键盘可达性（Enter 提交 / Tab 顺序 / 焦点管理全链路）

- **Login 表单**：`<form onSubmit={preventDefault + submit}>`（Login.tsx:124-127），输入框内 Enter 触发 submit 天然成立；提交前 `if (loading) return` 幂等守卫拦截重复 Enter；loading 期间 Input disabled 硬性防重。密码可见性切换 `type="button"` 不触发 submit。激活码 modal 内 Input autoFocus（:267），激活/取消按钮在 modal 内，Enter 在激活码输入框触发「表单外按钮无隐式 submit」（modal 无 `<form>`、按钮 onClick 直调）——模态上下文无意外提交。
- **Tab 顺序**：Login 账号 → 密码 → 可见性 → 提交 → （modal：激活码 → 激活 → 取消）。Select 搜索 → 过滤 → 返回 → 卡片操作按钮 → 预选按钮——DOM 顺序即 Tab 序（无 tabIndex 显式覆写，走读推断）。Admin 配置 Tab 数量/次数/地址/密钥/模型/并发 → 引擎切换 → 保存——同源。
- **Modal Esc 全链**：Select 退选（:1208 守卫 actionLoading）/ Login 激活（:232 守卫 activating）/ Admin 删除（:216 守卫 deleting）三处 onKeyDown 在位、激活中/退选中/删除中不响应防误关；autoFocus 取消按钮（Select :1234 / Admin :241）保证 Esc/Enter 取消首选。
- **焦点恢复缺口**（延续 O-3 族）：三个裸 div modal 关闭后焦点不还给触发按钮（无 Radix Dialog focus-scope/restore），历轮已述指向 F6-02 迁移。无新增。
- 结论：表单键盘可达性闭环（Enter 提交 / 幂等守卫 / Tab 顺序 / Esc 关闭），无新增发现。

### 3. 视觉状态反馈的层次（加载/禁用/选中的区分度 + 倒计时与状态横幅层次）

- **四态加载反馈**：Select 主区 isLoading 空态 + isError 错误态 + tabs 空态 + 过滤空态四分支（:901-922/:1182-1186）层次完整；Admin 各 Tab 三态（load/error/data）全覆盖（CodesTab :447-485 / ConfigTab :693-705 / StatsTab :763-798 / AccountsTab :829-904 / LogsTab :928-950）。**Dashboard 日志区例外** = OBSERVE-90-01（缺 load/error 态，见发现清单）。
- **Button 禁用/在飞层次**：actionLoading Set 独立跟踪（Select :52/:92-93/:119-120）→ disabled + 文案「报名中.../退选中...」双反馈；Admin removing Set（:311/:349-350）同构；generating/saving/deleting/deactivating 全部 disabled + Loader2 旋转 + 文案。在飞事务（点击成功前的网络往返）有明确视觉占位，无「点了没反应」静默窗。
- **选中层次**：Select 卡片 isSelected `border-white bg-black/15` + badge「首选/备选 N」+ 主打按钮文案「已设为后台冲刺首选」；徽章层（isFull 已满额 / unannounced 名额未公布 / remaining<=5 余 N 席 / 充足）四档互斥（:1033-1053）+ 进度色 rose/amber/emerald/cyan 四档分配（:1012-1016）——选中/满员/快满/未公布视觉完全可区分。
- **倒计时与状态横幅层次**：Select 横幅 :827-853 六分支（同步中 → 已关闭 → 已开放 → 未识别 → 本地已到点 → 距开放倒数）互斥且优先级正确（window_closed > window_opened > 未识别 > isExpired > 倒数），与 Dashboard 徽章三态（待命中/已开放/已关闭）同源。主矩阵与文案行吃 begin_times 兜底（F39-N1 契约在位）。
- 结论：视觉反馈层次完整，OBSERVE-90-01 之外无新增。

### 4. 深链与浏览器行为（刷新 / 前进后退 / 多标签视图一致性）

- **刷新恢复**：page/targetAccount 纯内存不持久化 → 刷新落回 dashboard（维持历轮口径：SPA 无路由库、深链丢失不属承诺）。管理员刷新恢复判据 = `xk_admin_token` 标记 vs `xk_admin_name` 账号会话（App :299 `inAdmin || isCurrentAdminSession(...)`）——B43-04/F52-M1 契约在位，撞名学生普通令牌永不误进。
- **前进后退**：无 history API、无路由切换（page 状态驱动条件渲染）→ 浏览器后退跳离站点（SPA 单页无栈内跳转），不产生「后退返回幽灵页」风险（走读推断）。
- **多标签**：localStorage xk_sessions 单键共享——多标签登录同一账号会互相覆盖会话令牌（后者覆盖前者，前者令牌被吊销）——历轮已知设计（单用户单机部署、多标签本属边缘场景），且服务端 401 吊销链 + 前端 UNAUTHORIZED_EVENT 摘除自动收敛，无双写损坏风险。维持观察不立条。
- **React 19 双重渲染/StrictMode**：main.tsx:7 StrictMode 在位；unmountedRef 挂载复位（Select :401）兼容 StrictMode mount→unmount→remount（R93 前已修），updater 纯函数（pick 的 toast 移出）兼容双调——实测 Dev 模式行为正常（历轮实证）。
- 结论：刷新/导航行为一致，多标签令牌覆盖归已知设计，无新增发现。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项管理）

**OBSERVE-93-01（延续，第五轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核）+ 语义正确（键盘触发/鼠标不显示/全站收敛）；残余面（Dashboard CollapseSection :67、Admin 复制/刷新/删除/收起/引擎切换/重试 :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）本轮逐一行号复核与 R93-R96 清单逐项一致无漂移；ring 参数未统一（纯白+offset vs 半透明白）值均可见。保持「续」。

**OBSERVE-90-01（延续）**：Dashboard.tsx 日志区（:697-718）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化（:697 `{logs && logs.length > 0 ? ... : :714-717 <NO RECENT LOGS>}`，无 isLoading/isError 解构分支；对照 Admin LogsTab :928-950 完整四态）。零功能危害维持留档。**「一行条件对齐」候选评估**：对齐需 `const { data: logs }` → 解构 isLoading/isError + JSX 两分支（约 8 行），非一行改动；且 /logs 失败时后端通常不可达（其他同信道查询也显示错误条），日志区误导影响面极小。维持不落地。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx:6-10 裸 createRoot）；历轮零渲染期异常实证 + 正常路径防御充分。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重新推演 handleBack 三轮 flush 收敛（:611-642）与等待窗口内 pick 新改动（等待期间 setRev → effect 挂 timer 异步 → 等一帧复查 revRef :622-624 已覆盖）无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（票据无效分支 :87-90 / 通用失败分支 :91-97 / 取消 Esc :232-238 + 按钮 :301-306）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害：等发布恢复/用户改动重跑）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态初始焦点落位（autoFocus 全覆盖）+ Esc 关闭全链可用（退选中/删除中/激活中不响应）+ Tab 顺序自然（modal 后于背景、按钮聚焦可到）——焦点进入侧闭环，陷阱 + 归还侧仍缺口。

**无障碍残留面备注（延续，非新立条）**：(a) Toast 无显式 aria-live 容器（走读推断：Radix ToastPrimitive Viewport 内建 aria-live=polite，R96 待浏览器实测）——本轮重读 Toast.tsx:105 Viewport 组件存在、语义由 Radix 内部实现，与前轮一致；(b) prefers-reduced-motion 无分支；(c) Progress 缺 aria-valuetext。三项均维持（a）待浏览器实测、(b)(c) 非阻断。

### 可疑待核
无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续成立：:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载。

## 六、已核无缺陷清单

- M-1 延续管理（第三十三轮）：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 八读写点全与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第五轮复核，git log 实证 R93 后 web/ 零提交）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新视角四项（本轮）：ref 镜像同步边界完整（消费时刻读 ref 全覆盖、stateData 依赖驱动自愈、handleBack 等待期新改动被三轮 flush 收敛）；表单键盘可达性闭环（Enter 提交 + 幂等守卫 + Esc 关闭 + autoFocus）；视觉反馈层次完整（加载/禁用在飞/选中/满员四档互斥 + 横幅六分支）；深链行为一致（刷新恢复判据在位、多标签令牌覆盖归已知设计）。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}|M-1|F\d{2}A?|B\d{2}-` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / document.write / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。
- 后端交叉契约复核（本轮重读关键点）：handleSetTargets 凭据表 accountExists 校验（handler.go:444-486）→ 无透传取核心账号兜底（:478-485）→ 条数上限 100（:499-502）→ 四字段校验（:503-516）→ 双 SetTargets + AppendLog 失败记日志零吞错（:517-524）；handleState 透传账号存在性校验 + 无透传核心账号兜底（handler.go:529-548）；windowClosedLocked 三条判据单源 + open 单快照复用（scheduler.go:913-939，判据 2 syncFailStreak≥3 带「开放时间已过」、判据 3 EmptyProbeRuns≥3 带 prevOpened 守卫）；StateForAccount 同源实时计算 + OpenTimeKnown 过期判定 + Courses 按账号过滤（scheduler.go:699-725）。
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、useTickingCountdown target 变化校正 now（useTickingCountdown.ts:19-21）、Dashboard 日期分组本地零点（parseDateKey/localTodayMs）、Toast 同 title 去重合并、Admin 复制 clipboard 降级链、Tabs 受控化（Admin :161 / Select :929）、Progress max=1 空条兜底（Select :1102-1103）。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npx vite build`（web/ 下） | ✅ built in 546ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 77 项全部通过（✓=77，✗=0，A/B/C 三族全绿） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=57ec7cb）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十三轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第五轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零提交）；残余面清单逐字无漂移。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:697-718），对齐候选评估维持不落地。
- **OBSERVE-88-01**（延续）：ErrorBoundary 缺失——维持观察不落地。
- **OBSERVE-85-02**（延续）：黄金期末尾 400ms 防抖竞态——安全方向刻意牺牲，本轮推演 handleBack 三轮收敛已覆盖。
- **OBSERVE-84-01**（延续）：激活失败后票据空请求文案突变。
- **OBSERVE-83-01 / 77-02**（延续）：courses 非空 + publishes 空稳态组合 / 窗口关闭无目标管理入口。
- **OBSERVE-76-01/02/03**（延续）：handleBack 等待无反馈 / uses 无上限 / Toast Close 无 aria-label。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口（本轮确认三项 autoFocus 落位 + Esc 全链已闭环 + Tab 顺序自然，陷阱/归还仍缺口）。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npx vite build` + 三守护脚本 + audit.mjs + node 纯只读实测 + git log/check-ignore）；工作区 `git status` 零改动（HEAD=57ec7cb，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：OBSERVE-93-01 残余面清单、O-3 族焦点陷阱/焦点恢复、新视角「表单 Tab 顺序」「浏览器前禁后退」「React 事件批处理论证」「handleBack 等待期新改动收敛」与无障碍残留面 (a) 为走读推断；M-1 逐字符、三组断言计数（18/18、6/6、5/5）、audit.mjs 77 项、tsc/build 退出码、契约 20 扫描各类（轮次锚点/XSS/localStorage try/catch/导航能力）、git rev-parse/log/check-ignore、后端契约 grep 定位、setSelected/dirtyRef/unmountedRef 清点均为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第三十三轮闭合；连续第四十三轮无严重级发现。