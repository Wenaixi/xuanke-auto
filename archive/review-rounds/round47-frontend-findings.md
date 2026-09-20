# round47 前端审查原始发现

> 审查基线：master @ `f1ea495`（round46 总结落盘，F46-F1/F2 Dashboard 三态 + 双查询失败态降频）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && git status --short` 干净；`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/{api/handler.go, scheduler/scheduler.go, zhidao/client.go}`（window_closed 三判据单源、/state 账号过滤、Courses 发布元数据透传、beginDate/beginDates 平台逐字透传、凭据表 accountExists、激活码生成校验、Portal 契约）。
> 上轮（round46）OBSERVE 3 / MINOR 2（N-2/N-3 延续）核对结论：
> - **O-1（Dashboard 窗口状态缺 window_closed 三态）**——**本轮确证已修复**：F46-F1 落地，Dashboard 状态卡（342-345）+ 手机悬浮栏（728）均已消费 `window_closed` 优先三态。R46 最需主控留意榜首关闭。
> - **O-2（/state 与 /logs 失败态不降频）**——**本轮确证已修复**：F46-F2 落地，两处 refetchInterval 顶部 `query.state.error || status==="error" → 30000`（140-145 / 156-161），与 Select 侧 F40/F42 契约对称。关闭。
> - **N-2（官网+冲刺双按钮）N-3（401 管理员保护窗口）**——复证仍成立，维持 MINOR 延续。
> - **O-3~O-5（Admin 后台轮询、resetRetry、Modal 无焦点陷阱、401 保护窗口数据面）**——复证仍在，维持 OBSERVE 延续。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无新 MINOR。）

---

## OBSERVE（观察项，未加重）

### O-1.【延续第 N 轮 | round40 前始】ToastProvider 的两层 `fixed` 容器叠加——Radix Toast Viewport 单层自组装定型、外层自制指 z-auto 的 wrapper 属 shadcn 残件，对功能无影响

- **文件路径:行号**：`web/src/components/ui/Toast.tsx:48`（外层 `<div className="fixed bottom-4 right-4 z-50 ...">`）+ `:85`（内层 `<ToastPrimitive.Viewport className="fixed ..." />`）
- **一句话问题**：Radix `ToastViewport` 源码自组的 DOM 是 `DismissableLayer.Branch(role=region)` 包 `Primitive.ol`，无 `position:absolute`、默认 static（仓库 1.2.23，`react-toast/dist/index.js:221-257` 逐行核实）。因此它的**子 toast 定位完全依赖 viewport 自身的 `fixed bottom-4 right-4`（:85 类名已在）**；外层 wrapper（:48）在同方向重复声明定位/间隙与同号 `z-50`，`z-index:auto`（内层 fixed 在 wrapper 之前开始渲染，渲染顺序决定堆叠上下文），实际只有设计意图中"一整屏区域、fixed bottom right 拉住半屏 max-h"的形态在起作用。
- **证据**：`viewport.className` 显式 `fixed`（:85）；`Toast.Root` 手写根类无 `fixed`（:56）；`max-h-screen` 只在 wrapper。wrapper 仅以 `z-index:auto` 参与堆叠，其 inner 渲染顺序（外后内）天然盖在 wrapper 之上；toast 实际可见无异常——历轮 E2E/构建从未报告"toast 不显示/永远在下层"。四个 variant 数量正确渲染、bottom-right 落位、swipe/pause 全由 viewport 承载。
- **触发场景推演**：无功能可复现的触发场景（推理链完整才报——若此条属实可测：低 z-index 背景层只在 `.canvas-bg`(z:0) 与 `.app-content`(z:1) 之上、sticky 手机悬浮栏 z-40，toast z "50" 恒赢）。属于 shadcn 模板"外包装饰 + Radix Viewport 自组装"叠加的残件，冗余但无害。
- **严重级**：OBSERVE（纯语义/结构冗余，零功能影响，既有视觉行为正确）。建议：把布局从 wrapper 移入 viewport `className`（`w-full max-w-[380px] p-4 gap-2` 等），删 wrapper；因项目内 Sheet/Dialog 从未被 import（见下），from-scaffold 的历史样式残留是此条同类。
- **宁缺毋滥自查**：raw toast 用 Radix 时"外 wrapper + 内 viewport 各自 bottom-right/fixed"是可以正常工作的双重定位——本项目正是落在此"能工作但重复"形态，故只记 OBSERVE 不升级。

### O-2.【延续历轮 core-review | F43-N3 已落】components/ui 的 Dialog/Sheet 系建成组件零消费——Sheet 从未 import，Dialog 除自引用外零使用；项目所有弹层均为手写裸 div 模态（Select 退选 / Login 激活 / Admin 删除）

- **文件路径**：`web/src/components/ui/Sheet.tsx`（整文件）、`web/src/components/ui/Dialog.tsx`（整文件）、使用方 `components/ui/Dialog.tsx` 无外部引入（grep `Sheet|Dialog` 仅 ui 目录内自引用；Admin.tsx:63/207/Login.tsx:222 注释中提及"迁移到 Radix Dialog"）
- **一句话问题**：两套（建成的）shadcn Radix 弹层组件从 scaffold 起即无消费；项目三处真实弹层（Select 退选确认 / Login 激活码 / Admin 删除账号）全是手写裸 div `fixed inset-0 z-50 role=dialog aria-modal` + Esc keydown + autoFocus——无焦点陷阱（Tab 可穿出至背景表单）与 title 继承 aria 可达性仅在"最小语义门"水平（Login.tsx:222 注释自认"完整焦点陷阱迁移到 Radix Dialog 属 F6-02 后续候选"）。这套组件本身连同其 typography 无 bug。
- **触发场景**：键盘用户点击"退选/删除/激活"弹层后按 Tab 会移到背景页面上（焦点陷阱缺失）——可访问性缺口，非功能错误；历轮 O-3（手写模态无焦点陷阱）同族记录在案。
- **严重级**：OBSERVE（每轮皆有的"已建成未使用"残件与最小语义门，设计意图层面的债务；有配套 OpenTime/M2 注释承诺"迁移"但从未发生）。建议下轮：要么换用 Radix Dialog 统一（复用既有组件，删两套手写模态+焦点陷阱同步落地），要么明确删掉未用组件。

### O-3./O-4./O-5.【round45 N-2/N-3 与 O-1~O-5 延续复证】

- **N-2（官网按钮 + 冲刺按钮双形态并存）**——复证仍成立（Select.tsx:1083-1150），双轨设计意图，持久化路径互不写冲突，维持 MINOR 延续。
- **N-3（App 401 管理员代理保护窗口闭包 current）**——复证仍成立（App.tsx:174-176，`current===adminName && inAdmin && lostAccount!==adminName → return`，保守方向），维持 MINOR 延续。
- **O-3（三处手写模态无焦点陷阱，见本文件 O-2）**、**O-4（401 保护窗口数据面）**、**O-5（Admin 五 Tab 无条件挂载后台轮询常跑，+ Dashboard 双查询失败态降频只覆盖"本查询自身失败"未覆盖"另一个查询失败"）**——均维持 OBSERVE。O-5 与 O-2 同族（Admin codes/stats/logs 固定 5000 / accounts 10000 无失败态降频），本轮再点一次名。

---

## 重点核对结论（任务书 1-3 节逐条裁决）

### 1. F46-F1/F2（Dashboard 三态 + 降频）——正确闭合

- **F46-F1 三态**：状态卡 `variant={state?.window_closed ? "outline" : window_opened ? "primary" : "outline"}` + 文案 `"窗口已关闭"|"窗口已开放"|"待命中"`（Dashboard.tsx:342-345）；手机悬浮栏同款三态（:728）。与 Select 横幅（F15-03）、Admin StatsTab（F39-N3"已关闭/已开放/待命中"）四兄弟同源同序，后端 `window_closed` 与学生端 /state 同源（handler.go:935 `d.Sched.WindowClosed()` 三判据单源 windowClosedLocked，scheduler.go:914）。关闭后状态卡不再悬在半空的心跳态。✓
- **F46-F2 失败态降频**：/state（:140-145）与 /logs（:156-161）refetchInterval 顶部均 `query.state.error || query.state.status === "error" → 30000`，成功态按 `window_closed ? 30000 : 3000`。与 Select 侧 F40-M3（/state）/F42-M3（/electives）判据逐字符同源。react-query 失败不清缓存 data 的轰炸路径已被封死。✓
- **边界核对**：两条 query 失败态各自独立判定（互不降频对方）——网络挂断期间 /state 失败 → /logs 仍 3s（另见 O-5-2 观察）；但 `/logs` 的 interval 回调读 `state?.window_closed`（组件闭包），/state 失败期间其值为最后一次成功快照，若 false 则 /logs 恒 3s。属 F40-M3 同族"只在本查询自己的失败态降频"的保守残留，非本轮回归，记 OBSERVE 不升级。

### 2. B45-N1（401 双广播收敛）——复证三形态各单次广播，上半轮裁决延续

- 前置 gate `client.ts:85 if (r.status !== 401)` 包住 body 层二次广播；三形态（网关 HTML 401 / 旧式 HTTP 200+body 401 / writeJSONStatus 双 401 requireAuth 唯一出口 handler.go:1068-1078）推演与 round46 完全一致，无新增形态。唯一补充：requireAuth 现在挂了 `writeJSONStatus(w, http.StatusUnauthorized, 401, ...)`，HTTP 401+body 401 双触发路径由 gate 精确消重。✓

### 3. round42-45 观察项（N-2~N-3 / O-1~O-5）——裁决见 OBSERVE/MINOR 节，无继承性回归

- **F43 四件套**（M1 全清空放行 / N1 btn_type / N2 票据生命周期 / N3 动效插件）——逐条复证全绿（Select.tsx:646-737 守卫链、Login.tsx:80-95 五清票点、global.css `@plugin tailwindcss-animate` 与 dist 实测 `.animate-in`+`@keyframes enter/exit` 已在构建产物，round46 已验）。✓
- **F42-M1 判据解耦**（shouldDeferSave 消费点：Select.tsx:499/589/676 三处同源；防抖 effect 依赖含 stateData 自愈）——复证正确。✓

### 4. 本轮新视角——逐条核实无果（宁缺毋滥）

- **Select 课程列表大数据渲染**：数百卡片 JSX 内嵌于 `filteredClasses.map`，内部派生 `selArr/rate/remaining` 均为纯变量无 hook；`key={c.id}`（课程 id 平台稳定、同一发布内唯一）。无 key 不稳定问题。整页每秒整帧重渲由 `useTickingCountdown` 全页级驱动（组件内 decimal 无 memo、`products` 稳定引用）——属 O-2 既有观察（每秒整帧重渲），数千 DOM 节点场景存在可感知开销，但 82 门量级实测无碍，维持 OBSERVE 不升级。真实问题只在「数百卡片因倒计时每秒重渲」的理论边界，宁缺毋滥不报 MAJOR。
- **Dashboard courses 长度徽标与 targets 一致性**：顶栏 `目标课程 {courses.length} 门`（:484）与手机悬浮栏 `TARGETS {courses.length}`（:731）均读 `/state.courses`（调度器已落库目标），不是防抖中的内存 selected ——数据源稳定、不随未落库状态抖动，防抖保存未落库期间的短暂偏差**在 Dashboard 侧不出现**（Dashboard 不读内存 selected），可接受。✓
- **Login 验证码**：本项目前后端均无验证码图片 URL 机制（登录由后端 zhidao.Client 完整闭环执行，前端只 POST /login 收 token ——Login.tsx:44-47 无任何 /captcha 请求）；`v=` 随机参数仅存在于 Python login.py（CLAUDE.md 契约，与 web 无关）。验证码相关竞态无代码面可查，不报。
- **Admin 激活码批量生成校验**：前端 count 输入 `Math.min(100, ...)` 钳制 + 后端两条硬校验（handler.go:628-639：`Count<1||Count>100` 拒绝、`Uses<1||Uses>1000` 拒绝）+ 在飞幂等短路（CodesTab `if (generating) return`，Admin.tsx:327）+ 空值 `Number(e.target.value)||1`。数量上限/空值/重复提交三路均有防御。唯一边界：`/admin/codes` 的 POST 由 `requireAdminSession`(403) 门住，无需额外前端管理态判定。✓
- **App sessions localStorage 快照式三连与容量**：login/logout/onDeleted/onUnauthorized 四路全部"读 loadSessions() 新快照 → 改 → saveSessions → setSessions"，无 setSessions updater 副作用残留（F8-01 已根除）；localStorage 配额（每个 token ~64 hex≈130B × 账号数）远低于 5MB 限制；try/catch 静默降级内存态（App.tsx:17-31）。容量无虞。✓
- **components/ui Toast/Sheet 的 portal 挂载点与 z-index**：见本文件 O-1/O-2。Sheet/Dialog 从未被 import（零消费）；Toast portal 全部在 document.body，z-index 恒胜于背景 z-0/z-1 与手机悬浮栏 z-40。功能层级无实际问题，仅结构冗余。

---

## 结论

- **MAJOR 0 / MINOR 0（新）/ OBSERVE 2（新）+ 延续若干**：无功能级错误。上轮最需注意的榜首（Dashboard 三态、降频）已由 F46-F1/F2 修复闭合；B45-N1 三形态单广播复证成立；F43 四件套、F42-M1 判据解耦复证全绿；round42-45 无继承性回归。
- 最需主控留意 3 条（均低风险，按真实影响排序）：
  1. **O-1 ToastProvider wrapper+viewport 双重 fixed 层叠**——福报归属 shadcn 残件，功能已证无碍；建议把布局并入 viewport className 删 wrapper，属纯清理。
  2. **O-2 Dialog/Sheet 零消费 + 三处手写模态无焦点陷阱**——跨轮遗留（Login.tsx 注释自认迁移候选），键盘可达性最低门槛待统一；与本轮「继续保持宁缺毋滥」基调完全贴合的观察项。
  3. **N-2/N-3 + O-5（双按钮 / 401 保护窗口 / Admin 后台轮询无失败态降频）**——延续 MINOR/OBSERVE，纯视觉与保守方向。
- tsc `--noEmit` 与 `git status` 全绿；本轮核心产出为**确证 F46-F1/F2 收敛 + 上轮榜首关闭**，另收两条 OBSERVE 级结构观察。