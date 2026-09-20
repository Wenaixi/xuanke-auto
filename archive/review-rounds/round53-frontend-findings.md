# round53 前端审查原始发现

> 审查基线：master @ `79d25aa`（round52 收官）+ `archive/review-rounds/review-round52.md` 决策锚 40/41。工作树预期仅根目录 5 个未跟踪社区文档；本轮唯一新建文件为本报告（`git status` 实测确认）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，1948 modules 产物 417.05 kB js / 41.04 kB css，构建后 `git status --short` 确认 web 目录删除 dist 前端后零 git 变更）；TDD 断言脚本 `npx jiti scripts/admin-auth-check.ts` **6 项全绿**、`npx jiti scripts/target-guard-check.ts` **16 项全绿**。范围 `web/src/` 全部 `.ts/.tsx`（18 文件），绝对只读。
> 后端契约对照：`backend/internal/api/handler.go`（handleLogin B43-04 双条件 handler.go:121、`issueSession` 普通签发 handler.go:218 响应无 adminName、`handleActivate` 签发 handler.go:214）、`backend/internal/session/store.go`（CreateAdmin Admin=true / IsAdmin 会话级）、`backend/internal/scheduler/scheduler.go`（StateForAccount 识别态 / RecognizedOpenTime）。
>
> 本轮结论先行：**MAJOR 0 / MINOR 3 / OBSERVE 6 校正 + 延续**。F52-M1 撞名学生管理态修复**全路径复核通过（8 条核验零缺陷）**；round52 除 F52-M1 外零修复后的回归复证（F43/F42/F40/F39/F36/F48-M1 六件套逐字符核对）**零回归**。最大实质产出是 **O-5（Admin 五 Tab 轮询常跑）前提修正**——Radix TabsContent 懒渲染实证下，同一时刻至多 1 个 Tab 查询在轮询，round52 描述的"四查询同跑"不成立，关切降级。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

### M-1.【跨页展示不一致】Select 选课大厅倒计时未同步 F39-N1 begin_times 兜底

**一句话问题**：Dashboard 在识别缺席（`open_time_known=false`）时用 `begin_times[0]` 兜底倒计时输入（F39-N1 决策锚 26 落地），Select 同场景仍 `target=null` 显全 00，两页在"识别未建立但平台已下发开窗点"时展示互相矛盾，用户需切页看真实倒计时。

**证据链**：
1. Dashboard.tsx:185-195：`openTimeStr = state?.open_time_known && state.open_time ? state.open_time : null`，`cd = useTickingCountdown(openTimeStr ?? (electives?.begin_times?.[0] != null ? new Date(electives.begin_times[0]).toISOString() : null))`——识别缺席时吃 `begin_times[0]` 兜底（F39-N1，决策锚 26："主倒计时吃 begin_times 兜底……主矩阵全 00 过期态与『预计开放时间』行未来时刻自相矛盾"）。
2. Select.tsx:743-749：`openTimeStr = stateData?.open_time_known && stateData.open_time ? stateData.open_time : null`；`cd = useTickingCountdown(openTimeStr)`——**无兜底**。
3. Select 自身持有 `data.begin_times`（Select.tsx:55-83 查询 /electives，`ElectivesData.begin_times: number[]`），完全没有数据可及性障碍——F39-N1 兜底只做了一半。
4. 触发条件：平台已下发 `beginTimes`（开窗点已知）但识别槽未建立/识别值过期（`open_time_known=false` 期间）。同屏横幅逻辑（Select.tsx:816-820）此时显"未识别到开放时间"、主矩阵全 00——横幅文案自洽，但**倒计时信息缺失**：同一浏览器切到 Dashboard 却能看到确切倒计时。
5. 影响：识别缺席窗口内（开窗前夕至开窗点之间，识别槽尚未建立的最关键盯守期），Select 页用户无法直接获知距开窗剩余时间；无数据破坏，纯展示层缺口。定 MINOR（F39-N1 跨页对称性未收口，而非新引入的缺陷）。

**修复方向**：Select 的 `cd` 输入与 Dashboard 同构：`useTickingCountdown(openTimeStr ?? (data?.begin_times?.[0] != null ? new Date(data.begin_times[0]).toISOString() : null))`（注意 Select 侧 `data` 为 /electives 查询结果）。与横幅"本地已到开窗点"分支（Select.tsx:821-822）互不干扰（`openTimeStr=null` 时横幅已走 `!openTimeStr` 分支）。

**TDD 形态**：纯函数断言脚本（`web/scripts/` 惯例）：抽 `resolveCountdownInput(openTimeStr, beginTimes)` 返回 `string | null`，断言"识别缺席 + beginTimes 非空 → 返回 ISO 字符串"、"识别有效 → 返回识别值"、"两者皆缺 → null"。tsc -b 验证即可，无需动 react 树。

### M-2.【体验】目标保存失败重试链连续红 toast 轰炸（无去重）

**一句话问题**：`saveNow` 网络失败每次重试都弹 destructive toast，指数退避最多 5 次 → 单次网络抖动 46 秒内最多 6 个"目标保存失败"红 toast 堆叠；Toast 层无任何去重/合并机制，黄金期网络抖动时轰炸最凶。

**证据链**：
1. Select.tsx:423-460 `saveNow`：catch 分支（438-449）`toast({ title: "目标保存失败", description: ..., variant: "destructive" })` → `dirtyRef.current = true` → `scheduleRetry()`。
2. Select.tsx:413-422 `scheduleRetry`：`attempt >= 5 return`（最多 5 次）、delay `2/4/8/16/16s`——每次失败必然 re-toast，无"同内容 toast 去重"。
3. Select.tsx:664-728 防抖回调：任何一次用户改动触发新一轮防抖 → `resetRetry()`（647）把 attempt 归零 → 失败重试链可被反复激活，长期网络故障 + 用户反复点选 = 无限循环轰炸。
4. Toast.tsx 无去重：`toast()`（Toast.tsx:32-38）纯 append，`removeToast` 只按 id 删，无按内容合并/替换逻辑；`duration={3500}`（Toast.tsx:58）固定。
5. 触发条件：目标保存持续失败（网络抖动/服务端重启/后端 5xx），黄金期（开窗瞬间 2s 高频轮询 + 用户高频点选目标）概率最高。
6. 影响：用户体验级轰炸（红条互相遮挡、误以为操作失败需重试），无数据破坏（dirtyRef 保证内存目标不丢，卸载后 unmountedRef 已停（F13-C2）——仅"挂载期间"的重复轰炸）。定 MINOR。

**修复方向**（摇梯最小档）：(a) ToastProvider 内按 `title` 去重——相同 title 的 toast 在 viewport 已存在时替换其 description 不新增；(b) 或 `saveNow` 失败分支仅在 `retryState.current.attempt === 1`（首次失败）toast，重试静默（已在 UI 提示过）+ 最终失败（attempt 达 5 时）再补一条"仍在重试，已停止自动重发"。推荐 (a)——一把锁在所有调用点。

**TDD 形态**：ToastProvider 抽 `pushToast(toasts, msg)` 纯函数（按 title 去重/合并），`web/scripts/toast-dedup-check.ts` 断言：同 title 重复 → 列表不增长、description 取最新；不同 title → 正常追加。

### M-3.【语义一致】成功类 toast 未用 success variant + duration 不可配

**一句话问题**：学生端成功操作统一 `variant="success"`（绿白图标），管理端成功操作（账号删除/激活码生成/配置保存）默认 `default`（信息图标），同一设计语言内成功语义分叉；且 `useToast` 不暴露 duration，全站 3.5s 一刀切。

**证据链**：
1. Select.tsx:96（"报名成功" success）、123（"退选成功" success）vs Admin.tsx:270（"已删除"）、337（"激活码已生成"）、541（"配置已保存"）——后者无 variant → 默认 `default`，Toast.tsx:71 渲染 Info 图标（非 Success CheckCircle2）。
2. Admin.tsx:105/272/339/359/546 失败场景有 destructive ✓——仅成功路径未对齐。
3. Toast.tsx:58 `duration={3500}` 硬编码；`useToast` 的 `ToastMessage`（Toast.tsx:8-13）无 duration 字段——"目标保存失败"这类需要更长阅读时间的失败文案与"已复制"这类瞬时反馈同寿。
4. 触发条件：管理员在管理后台执行任一成功操作。
5. 影响：视觉语义不一致（成功操作无成功符反馈），体验单调。定 MINOR（一致性，非功能性）。

**修复方向**：Admin 三处成功 toast 补 `variant: "success"`；`ToastMessage` 加可选 `duration` 字段透传 `ToastPrimitive.Root`（默认 3500 保持兼容）。

**TDD 形态**：纯函数级断言不适用（纯展示）；tsc -b 类型门 + 构建验证即可，无需额外脚本。

---

## OBSERVE（观察项，未加重）

### O-1.【修正】【核心】round52 O-5 前提不成立——Admin 五 Tab 为"单激活查询轮询"，非四查询同跑

**复核结论**：round52 O-5 描述"Admin 五 Tab 后台轮询常跑（codes 5s + stats 5s + logs 5s + accounts 10s 四查询的请求量级）"**前提被 Radix Tabs 的懒渲染机制推翻**——同一时刻至多 1 个 Tab 的查询在轮询，其余 Tab 组件未挂载、其 useQuery 观察者 inactive、`refetchInterval` 停止。**关切降级（资源友好事实），无修复必要，修正观察记录。**

**证据链（逐环实证）**：
1. Admin.tsx:185-204：五个 `TabsContent`（codes/config/stats/accounts/logs）各自包一个子 Tab 组件（CodesTab/ConfigTab/StatsTab/AccountsTab/LogsTab），挂载于 Radix `Tabs`（Admin.tsx:161）。
2. Radix TabsContent 源码实证（`web/node_modules/@radix-ui/react-tabs/dist/index.js:205`）：`React.createElement(Presence, { present: forceMount || isSelected, ... })`，children 为渲染 prop `({ present }) => Primitive.div(hidden: !present)`。`react-presence` 在 `present=false` 且无退出动画时**返回 null、渲染 prop 不被调用** → 非激活 Tab 的 content 子树（子组件）**从 React 树卸载**，非"隐藏但挂载"。
3. 卸载副作用：非激活 Tab 的 `useQuery`（codes 316-320 / stats 702-706 / accounts 788-792 / logs 876-880，key 各含 `["admin-*", account, sessionToken]`）观察者 detach → 查询 inactive → react-query v5 对 inactive 且无其他观察者的查询**停止执行 refetchInterval**。切换 Tab 时旧查询卸载停止、新查询挂载启动（缓存保留，回切立即显旧数据 + 挂载即 refetch）。
4. 资源账：管理员停留任一 Tab 时实际轮询 = 该 Tab 单查询（5s/5s/10s/5s 之一）。与 round52 报告的口径差异由"模板直觉（五 Tab JSX 都在渲染 = 五查询都在跑）"vs"Radix 运行时懒渲染"造成——本报告以产物源码为准。
5. 附带确认：ConfigTab 无 `refetchInterval`（Admin.tsx:493-496，只在保存后 refetch），本就不参与轮询面；`activeTab` 受控（Admin.tsx:56）切换时旧的卸载 / 新的挂载 clean，无残留监听。

**裁决**：O-5 从"后台轮询常跑（关注量级）"**降级为已纠正事实**（单查询轮询），无任何修复动作。

### O-2.【继续观察】useTickingCountdown 每秒整页重渲染——量化收敛分析，可接受

**复核结论**：`useTickingCountdown` 内部 `setInterval(1000) → setNow`（useTickingCountdown.ts:9-12）令消费处组件（Dashboard/Select 主组件）每秒重渲染。定量估算：
- React 渲染 bailout 机制：`now` 只在倒计时函数内部被消费（Dashboost.tsx:366-395 数字、Select.tsx:826-827），其余卡片/列表 props 引用不变 → reconcile 时子树引用相同被跳过，**实际 DOM 写入仅倒计时文本节点**。
- Dashboard 组件树 ~20 个 component 层 / ~100 DOM 节点，`dateGroups`/`extrasMs` 有 useMemo 缓存命中不重算；`nowMs = Date.now()`（Dashboard.tsx:198）每拍新值，但 `relativeCountdown`（Dashboard.tsx:94-108）只在 extrasMs 行消费（当前学期单值 → 段不渲染）。
- Select 组件树 ~82 门课程卡片（每卡 ~30 节点），每秒 reconcile 一次 fiber 树（props 引用不变整体 bailout），实测量级远低于 16ms 帧预算。
- **裁决**：维持在 OBSERVE，不升级——渲染强度小、无性能边界被击穿（O-5/O-6 原判延续）。可选的纯局部化方案（把倒计时抽成 memoized 子组件）收益极低，宁缺毋滥。

### O-3.【延续】Dashboard expandedDates 跨账号残留 + 挂载 key 不对称

**复核结论**：App.tsx:331-337 Dashboard 挂载点无 `key={account}`（Select 两处挂载点 App.tsx:293/339 有 `key={account}` 双保险）——账号切换（401 被动对调/onDeleted 后自动切号）不经过卸载，Dashboard 原地重渲染，`expandedDates`（Dashboard.tsx:268）残留旧账号折叠态。效果：新账号同日期组的展开/折叠状态随旧账号漂移，纯展示层、无数据串线（`dateGroups` 随 `courses` 重建）。**存活边界确认**：`expandedDates` 的 null/[] 语义分离（Dashboard.tsx:270-273 仅在 `null` 时种子，`[]` = 用户主动全折叠绝不重种）正确，种子逻辑无缺陷。**维持 OBSERVE 不升级。**

### O-4.【延续】手写模态无焦点陷阱（F6-02 Radix Dialog 锚点）

**复核结论**：三处手写 modal——Login 激活码（Login.tsx:223-316）、Select 退选（Select.tsx:1175-1224）、Admin 删除（Admin.tsx:210-284）——均有 `role=dialog/aria-modal=autoFocus+Esc`，但**均无焦点陷阱**（Tab 可逃出弹层进入背景表单）。round52 已确认"完整焦点陷阱迁移到 Radix Dialog 属 F6-02 后续候选注释内"。**维持 OBSERVE**（F6-02 锚点注释 Login.tsx:222 仍在，见 O-8）。

### O-5.【延续】N-2 双形态按钮 / N-3 401 保护窗口闭包

- **N-2（官网 btn_type 按钮 + 冲刺按钮双形态并存）**：Select.tsx:1090-1115（官网 btn_type 1/2 渲染判定，can_select 置灰）+ 1119-1149（本项目冲刺/预选按钮，窗口信号决定 primary/ghost 形态）——双轨设计意图（窗口未开时可预选、开窗后官网报名为主）。**维持 MINOR 级观察延续**（非缺陷，展示双通道）。
- **N-3（App 401 管理员代理保护窗口闭包 current，App.tsx:187-235）**：onUnauthorized 回调闭包捕捉 `current`（229 行 detail 兜底 fallback），effect 依赖 `[adminName, inAdmin, adminToken]`（235 行）重建监听器刷新闭包——401 事件与账号切换竞态下的闭包 current 保守方向（多账号 401 乱序时宁慢不误杀，F10-05 反查机制已覆盖正常路径）。**维持观察。**

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体

**复核结论**：grep 全仓 `CardFooter` 唯一命中 Card.tsx:46-51 自身定义（shadcn 五件套模板残留）；Button 的 `invert/success/warning/secondary/icon` 变体、Badge 的 `default/success/warning/destructive/secondary/active` 变体实际 `variant=` 传值仅命中 `primary/outline/ghost/dark/destructive` 子集（Button 实测：primary/outline/ghost/dark 四值；Badge 实测：primary/outline 两值）。属通用组件模板残宽（组件本体被消费），扩大清剿收益极低。**维持 OBSERVE 不列修复。**

### O-7.【边缘】xk_admin_token/xk_admin_name 多标签页生命周期一致性

**复核结论**：`saveAdminToken/saveAdminName` 仅写 `localStorage`，**无 `storage` 事件监听**——标签页 A 管理员登录（xk_admin_token=T1）后，标签页 B 撞名学生登录/学生登出时的 `logout()`（App.tsx:121-122 无条件清 adminToken）会清掉 B 可见的共享 localStorage 标记 → A 刷新后 `adminToken=""` → 回学生端（isCurrentAdminSession false），管理员需重新管理登录。**影响为体验级（管理员重登），方向安全**（永不误进管理页）；多标签页同浏览器是本项目 localSession 模型（xk_sessions 同样不跨页同步）的既有边界。**OBSERVE，不列修复。**

### O-8.【延续】F51-O1 注释残留现状缩至一处

**复核结论**：round52 记载 "Admin.tsx:207 与 Login.tsx:222 两处 Radix Dialog 注释"——本轮实测 grep `Radix Dialog|F6-02` 仅命中 **Login.tsx:222 一处**（Admin.tsx:207 现为 N3/F7-03/F20-04 注释组，措辞已不含 Radix Dialog）；残留属 F6-02 未来迁移锚点注释，零功能影响。**维持观察，不列修复。**

---

## 重点核对结论（任务书逐条裁决）

### 1. F52-M1 撞名学生管理态修复全路径复核（8 条核验全过）

| 核验点 | 证据链 | 裁决 |
|---|---|---|
| ①　login 带 adminName 才标记 | App.tsx:92-100 `if (adminName) { setAdminName; saveAdminName; setAdminToken(token); saveAdminToken(token) }`；Login.tsx:49 `onLogin(data.token, data.account, data.adminName)`；后端 handleLogin 响应带 adminName 仅管理签发（handler.go:129），撞名学生走 issueSession 响应 `{token,account}`（handler.go:230）→ `data.adminName===undefined` → 标记分支不执行 | 正确 |
| ②　刷新恢复判据 | adminAuth.ts:11 `adminToken !== "" && sessions[adminName] === adminToken`；App.tsx:299 渲染 `inAdmin \|\| isCurrentAdminSession(...)` 双通道；`loadAdminToken`（App.tsx:55-61）localStorage 恢复 | 正确 |
| ③　四路径清 adminToken+inAdmin 对称性 | logout（App.tsx:118-122 清）+ onDeleted（App.tsx:173-177 仅 `acct===adminName` 清，后端拒删管理员名故撞名清除场景实际不触发）+ onUnauthorized（App.tsx:210-215 `lostAccount===adminName` 清）+ onBackToStudent（App.tsx:309-313 无条件清）——四条路径全部落盘清 + 态清，无遗漏 | 正确 |
| ④　原 current===adminName 判据无残留 | grep `=== adminName` 全仓命中仅 App.tsx:172/205/210 三处——全部是"管理员账号名身份识别"（onDeleted 删管理员自身 / onUnauthorized 被吊销的是管理员自身 / 代理保护 lostAccount 比对），**无一用于管理态判定**；唯一管理态判据 = isCurrentAdminSession（App.tsx:299） | 正确 |
| ⑤　xk_admin_token 与 xk_admin_name 生命周期一致性 | 写入条件同源（登录响应的 adminName 字段）、清除条件同源（四路径均双清）；唯一edge = 多标签页共享存储无 storage 同步（见 O-7），同会话模型既有边界 | 正确（见 O-7 edge） |
| ⑥　旧版本升级（无 xk_admin_token）刷新安全 | adminAuth.ts 首个判据 `adminToken !== ""` 前置短路——无标记 → 恒 false → 回学生端；admin-auth-check.ts 场景 D/E 实测绿（`无标记管理 token → 学生端` / `普通学生无标记 → 学生端`） | 正确 |
| ⑦　XUANKE_ADMIN_NAME 自定义名 | App.tsx:78 `adminName = useState(loadAdminName)` 恢复 + login 同步（App.tsx:92-95）+ 全路径 isCurrentAdminSession 用 state 名；admin-auth-check.ts 场景 F（`boss` 自定义名 token 一致 → 恢复）实测绿 | 正确 |
| ⑧　useMemo/依赖数组引用稳定性 | adminToken 为 string 值类型——App.tsx:148（accounts 迁移 effect）/235（onUnauthorized effect）依赖含 adminToken，string 值比较稳定，无多余 effect 重建；无 useMemo 包裹必要 | 正确 |

**附带边界核对三则**（无问题）：
- 撞名学生 sessions["admin"] 覆盖管理员旧会话 token（`{...loadSessions(), [account]: token}`，App.tsx:101）→ 新旧 token 不等 → isCurrentAdminSession false →学生端；管理员重登覆盖回来。自洽。
- onUnauthorized 中被吊销账号为撞名学生（lostAccount==="admin"==adminName）时清 adminToken——若同浏览器残留真实管理员标记（多标签页 edge）反而向安全方向清除。
- 管理员会话 401（lostAccount===adminName）→ 保护窗口（217 行）`lostAccount !== adminName` 为 false → 正常剔除不误保。✓

### 2. R52 除 F52-M1 外零修复后的回归复证（逐字符零回归）

| 防护族 | 复证结论 |
|---|---|
| **F43 全清空不等于数据缺席** | shouldDeferSave 第二参数 hasSelected（targetGuard.ts:57-61）——三消费点同步传入：防抖回调 676 行 `selectedCount > 0`、flushTargets 499 行 `latestSelectedCount > 0`、handleBack 589 行 `revRef.current > 0 && shouldDeferSave(..., hasSelectedNow())`（handleBack 多 fact revRef>0 前置符合"纯浏览零等待"语义）；`courses 非空 && hasSelected` 才推迟，全清空放行 PUT []。**一致，无回归** |
| **F42-M1 判据与数据源解耦** | shouldDeferSave 纯数据判据（stateData undefined \|\| courses 非空 && hasSelected）、不依赖 echoedRef；防抖 effect 依赖含 `stateData`（Select.tsx:737）→ /state 到达触发重跑自愈；flushTargets 用 `stateDataRef.current`（499 行）读最新。**一致，无回归** |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected/shouldDeferSave 消费点全量清点：回显 effect 287 行 + 独立清理 effect 316-327 行（F41-M1 下沉，deps 含 selected 防时序） + 防抖回调 696 行 + flushTargets 518 行——四处均前置 `selectedHasStalePublish` 于构建之前、命中置脏不 PUT、空数组 key 保留清空语义；16 项纯函数断言全绿。**一致，无回归** |
| **F39-C1 selectedHasStalePublish 纯函数** | 判"非空且不在当前发布集合"（targetGuard.ts:9-19），空 key 绝不判过期；target-guard-check 场景 C/D 实测绿。**一致，无回归** |
| **F36-01 key={account} 双保险** | App.tsx:293（targetAccount 代理分支）+ 339（学生大厅分支）双 keyed；Select 内部 accountKey 守卫（Select.tsx:195-202）声明位于 echoedRef/rev 之后（TDZ 安全）兜底未来不重置挂载。**一致，无回归** |
| **F48-M1 Toast 定位** | Toast.tsx:87 viewport `fixed bottom-4 right-4 z-50` 定位类落在 viewport 上（非空壳 wrapper），无 inset 类退化为 static-position 的 bug。**一致，无回归** |

### 3. 本轮新视角六项——逐条裁决

- **A. Toast 使用点语义+生命周期** → 见 M-2/M-3 + O-2：duration 全站 3.5s 不可配、无去重（M-2 轰炸场景）、管理端成功 toast 未用 success variant（M-3）。
- **B. useTickingCountdown 渲染收敛度** → 见 O-2：量化分析（React bailout 使叶子不重渲、实际 DOM 写入仅倒计时节点）可接受，不升级。
- **C. Dashboard 折叠态生命周期** → 见 O-3：expandedDates 跨账号残留（无 key 不对称）+ dateGroups 分组键（CourseStatus.begin_date 优先 → /electives 映射 → "未知"兜底）正确；排序 `|日期-今天|` 升序 + "未知"恒最后、平局按日期值升序稳定。数据正确性边界无击穿。
- **D. 壁纸滚动机制** → App.tsx:245-277：mount 量测缓存（getBoundingClientRect 不进滚动路径）、scroll/resize/img-load 三监听 cleanup 成对、rAF 单飞（frame 置 0 后合帧）、`passive:true`、无 SSR 场景（纯客户端 Vite）；`aria-hidden` 背景无焦点风险。**资源释放无问题**。唯一细节：`apply` 直接写 `img.style.transform` 覆盖 CSS 的 `translateY(var(--bg-shift))`（global.css:98），但 `--bg-shift` 从未被 JS 使用（属无 JS 兜底），无冲突。**无问题**。
- **E. Select 防抖保存链残缺口** → 三纯函数与三消费点（防抖回调/flushTargets/handleBack）逐字符一致（见 2 回归复证表）；五 ref（unmountedRef/echoedRef/echoDone/selectedRef/revRef）生命周期交错复核：unmountedRef StrictMode 挂载复位（Select.tsx:398-407 F20-01）→ 0826 卸载置位 + cleanup 清重试 timer；echoedRef 由回显 effect 置位 + accountKey 守卫复位（195-202）；echoDone 进防抖 effect 依赖（737 行）驱动场景 B（courses 空）自愈——双路自愈均通；handleBack 三轮 flush 收敛（socket 等待 + savingRef/retryState pendingSaving 检测）无死锁。**唯一边缘**：stale 守卫命中时三轮 flush 可能重复 toast（极低概率，OBSERVE 级，被 independent 清理 effect 自愈窗口覆盖）。**无缺陷**。
- **F. Admin 五 Tab 后台轮询开销量化** → 见 O-1 修正：单激活查询轮询，四查询"同跑"为前提错误；失败态降频（react-query 失败不触发 interval？——实测 v5 失败后 interval 仍按原计划跑，但组件层 `query.state.error` 守卫在各处 refetchInterval 已见）——**核对失败态**：Admin 四查询的 `refetchInterval` 为静态数字（Admin.tsx:319/705/791/879），**无错误态降频**（Dashboard/Select 的函数式 error 短路在 Admin 侧缺失）。不过：单 Tab 轮询 + 移除 5s/10s × 单查询，失败态 5s 重打单接口压力可忽略，且注入的 interval 不会把 error 恢复成 30s——关联 O-1 修正后定为可接受（请求量级小、单端点）。

---

## 上轮观察项延续复核表

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| N-2（官网 btn_type + 冲刺按钮双形态） | MINOR 观察延续 | Select.tsx:1090-1149 双轨设计意图确认 | 延续（O-5/观察） |
| N-3（App 401 保护窗口闭包 current） | MINOR 观察延续 | App.tsx:187-235 闭包 + deps 重建确认 | 延续 |
| O-2（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱，F6-02 锚点未动 | 延续 |
| O-3（401 保护窗口数据面） | OBSERVE | targetAccount 清代理态 + lostAccount 反查正确 | 延续 |
| **O-4（Admin 五 Tab 后台轮询常跑）** | **OBSERVE（四查询同跑）** | **Radix TabsContent 懒渲染实证单查询轮询，前提不成立** | **修正/降级（O-1）** |
| O-5（useTickingCountdown 每秒整页重渲） | OBSERVE | 量化分析 bailout 机制可接受 | 延续 |
| Dashboard key 不对称 | OBSERVE | 无 key 挂载 + expandedDates 残留确认 | 延续 |
| ui 模板残宽（CardFooter 零消费 + 死变体） | OBSERVE | grep 复证 | 延续 |
| F51-O1 注释残留（Admin.tsx:207 / Login.tsx:222） | OBSERVE | Admin.tsx:207 已不含 Radix Dialog 措辞，仅 Login.tsx:222 一处残留 | 收窄至一处 |

---

## 结论

- **MAJOR 0 / MINOR 3（M-1 跨页兜底不一致 / M-2 失败重试 r toast 轰炸 / M-3 成功语义 variant + duration）/ OBSERVE 6 校正 + 延续**。tsc exit 0、build 成功、构建后 web 零 git 变更、adminAuth 6 项 + targetGuard 16 项 TDD 断言全绿。
- **最致命 3 条（按影响排序）**：
  1. **O-1 修正——Admin 五 Tab 实为单激活查询轮询**（round52 O-5 的四查询同跑描述被 Radix TabsContent 懒渲染推翻，资源账降约 75%，无需任何修复）；这是本轮唯一实质"新认知"，非缺陷。
  2. **M-2 目标保存失败重试链红 toast 轰炸**（网络持续故障 + 用户高频点选时 46s 内最多 6 个堆叠，黄金期高发）——体验缺陷，修复（按 title 去重）一把锁所有调用点。
  3. **M-1 Select 倒计时未同步 F39-N1 兜底**（识别缺席窗口内两页展示矛盾、Select 无倒计时），对齐一行即可联动 Dashboard 语义。
- **已核对无缺陷的高风险区域**：F52-M1 撞名学生管理态修复 8 条全路径（含旧版升级/自定义名/撞名/多账号四族）；F43/F42/F40/F39/F36/F48-M1 六防保存链零回归；Select 防抖保存链五 ref 生命周期交错（含 StrictMode 双挂载）；壁纸滚动机制资源释放；adminAuth/targetGuard 纯函数契约（TDD 全绿）。Dashboard begin_date 分组 + 折叠种子逻辑数据正确性无击穿。
- **建议优先修复方向**：Toast 去重（M-2，体验收益最大且一把锁全站）→ Select 倒计时兜底（M-1，一行联动）→ 管理端 success variant（M-3，纯样式）。O-1~O-8 均无需动作。