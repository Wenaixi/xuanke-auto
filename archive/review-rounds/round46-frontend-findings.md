# round46 前端审查原始发现

> 审查基线：master @ `338efb8`（round45 总结落盘，B45-N1 client.ts 401 双广播收敛）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **exit 0**（tsc -b && vite build 绿，产物 `backend/web/dist/assets/index-BvE-Yqfx.css`）。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/{api/handler.go, scheduler/scheduler.go, zhidao/client.go}`（401 唯一入口、window_closed 下发与 /state 同源、handleState 账号校验、Target/CourseStatus 发布元数据字段）。
> 上轮（round45）MINOR 4 / OBSERVE 5 核对结论：
> - **N-1（401 双广播）**——**本轮确证已收敛**：client.ts:85 `if (r.status !== 401)` 前置 gate 后，三形态各单次广播（见下节）。R45 的 MINOR 关闭。
> - **N-2/N-3**——复证仍成立（设计意图/保守方向，无数据分叉），维持 MINOR 延续。
> - **O-1~O-5**——逐条复证仍在，本轮各给裁决。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无新 MINOR。）

---

## OBSERVE（观察项，未加重）

### O-1.【延续第 14 轮｜round13 F13-B3 始】Dashboard 窗口状态标识只消费 `window_opened` 两态，不消费 `window_closed`——窗口关闭后状态卡与手机底部悬浮栏恒显"待命中"，与 Select 横幅、Admin 三态、数据层降频信号三处不一致

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:326-328`（窗口状态卡 `state?.window_opened ? "primary" : "outline"` + `"窗口已开放" : "待命中"`）、`:709-711`（手机底部悬浮栏 `"窗口开放中" : "系统待命中"`）
- **一句话问题**：F13-B3 观察（"Dashboard 窗口关闭后 Badge 仍显待命中，未消费 window_closed"）从 round13 至今连续复证、从未修复。同页数据层每处都已消费 window_closed（/state 与 /logs 的 refetchInterval 降频 30s，`Dashboard.tsx:136/146`）；Select 横幅有 `window_closed 优先→已关闭`（F15-03）；Admin StatsTab 有 `已关闭/已开放/待命中` 三态（F39-N3）。唯独 Dashboard 两处展示层仍按 `window_opened` 布尔两态渲染——窗口真正关闭后，用户停留的控制台主视图仍显示"待命中"/"系统待命中"，与同屏"选课窗口已关闭"的关闭事实、降频后的静止日志流自相矛盾。
- **触发场景推演**：选课窗口关闭（09:00-10:30 结束）后，管理员或学生打开 Dashboard：状态卡绿色心跳仍在、Badge 显示"待命中"（因为 `window_opened` 恒 false），而课程列表空态式微 + 日志已 30s 低频 → 用户会误以为系统异常"还没开始抢课"。
- **为什么给 OBSERVE 而非 MINOR**：纯展示一致性，无功能/数据错误；且是历史观察项（F13-B3 记录在案）。修复是两行（Badge `window_closed ? "outline destructive" : ...` 三态）。连续 14 轮未动，属积累，本轮仍不升级——但值得在 summary 提醒主控。

### O-2.【新观察】Dashboard 双查询 `/state` 与 `/logs` 的 refetchInterval 失败态不降频——与 Select 侧 F40-M3/F42-M3"失败即 30s"不对称

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:136`（/state `(query) => (query.state.data?.window_closed ? 30000 : 3000)`）、`:146`（/logs `(state?.window_closed ? 30000 : 3000)`）
- **一句话问题**：F40-M3（Select /state）与 F42-M3（Select /electives）已把"查询失败态降频 30s"收敛为全站轮询契约；Dashboard 两查询未并入同款判据。react-query 失败后 `query.state.data` 保留最后一次成功值（不清缓存），若该值 `window_closed=false`，回调恒取 3000ms —— 网络挂断/后端重启期间 Dashboard 的 /state + /logs 双查询按 3s 固定轰炸不可达后端（Select 侧同场景已 30s）。
- **触发场景推演**：登录后停留在控制台、后端进程崩溃/重启或局域网断连（公网部署/校园网波动是此项目真实环境）：react-query retry:1 失败后双查询继续每 3s 打一次失败请求，浏览器控制台持续红色 error，日志横幅停滞。窗口开放黄金期用户多半在 Select（已降频），但 Dashboard 作为默认落地页（App 默认 `page="dashboard"`）暴露峰值最高。
- **严重级**：OBSERVE（纯轮询轰炸强度，无状态/数据危害；且失败重试 5 次后 react-query `retry:1` 停手——真正的问题在「retry 停手后 interval 还继续 3s」，属于 F40-M3 同族未覆盖的另一半）。
- **建议修法**（两行，与 Select 同款对称）：两处 interval 回调顶部统一 `if (query.state.error || query.state.status === "error") return 30000`。

### O-3./O-4./O-5.【round45 N-1/N-2/N-3 与 O-1~O-5 延续复证】

- **N-2（官网按钮 + 冲刺按钮双形态并存）**——复证仍成立（`Select.tsx:1083-1150`），双轨设计意图，持久化路径互不写冲突，维持 MINOR 延续。
- **N-3（App 401 管理员代理保护窗口闭包 current）**——复证仍成立（`App.tsx:174-176`，`current===adminName && inAdmin && lostAccount!==adminName → return`，保守方向），维持 MINOR 延续。
- **O-1 resetRetry 退避清零**（Select.tsx:648）、**O-2 每秒整帧重渲**（useTickingCountdown.ts:8-12 + Select / Dashboard 消费点）、**O-3 手写模态无焦点陷阱**（Select 退选 / Admin 删除 / Login 激活三处）、**O-4 401 保护窗口数据面**、**O-5 Admin 五 Tab 无条件挂载后台轮询常跑**（Admin.tsx:161-205 + codes 5000 / stats 5000 / logs 5000 / accounts 10000）——均维持 OBSERVE。

---

## 重点核对结论（任务书 1-3 节逐条裁决）

### 1. B45-N1（401 双广播收敛）——正确闭合，三形态各单次广播，无遗漏形态

- **前置 gate**：`client.ts:85` `if (r.status !== 401)` 包住 body 层二次广播。
- **三形态推演**：
  - **网关 HTML/文本 401**（F41-N2 主场景）：`r.status===401` 前置广播一次 → `r.json()` 抛错走 -2 文案（body 层不执行）→ 单次。✓
  - **旧式 HTTP 200 + body 401**：前置不触发 → body 层 `r.status !== 401` 成立 → 广播一次 → throw ApiError(401)。✓
  - **writeJSONStatus 双 401（requireAuth，唯一入口 handler.go:1078）**：前置广播一次 → body 层 gate 跳过 → 0 次二次广播，`j.code===401` 分支继续 throw（业务侧仍识别到 401）。✓
- **遗漏形态扫描**：后端全仓 `StatusUnauthorized` 仅 handler_test.go 断言 + handler.go:1078 唯一路径；业务错误统一 code=1/1001/-1 等（`writeJSON(w, 1, ...)`），不触发 body 层第二段。R45 N-1 提出的"事件风暴翻倍"路径已被 gate 彻底消除，**MINOR 关闭**。

### 2. F43 四件套——全绿

- **F43-M1 全清空放行**：`shouldDeferSave(stateData, hasSelected)` 三消费点同源（防抖 676 / flush 499 / handleBack 589+595），纯函数 `stateData===undefined || (courses>0 && hasSelected)`；rev>0 全清空 → hasSelected=false → 放行，绝不把清空当"待回显"打回。✓
- **F43-N1 btn_type 判据**：`===1` 退选 / `===2` 报名无条件渲染，`disabled={actionLoading.has(c.id) || !c.can_select}` 双守卫，`title` 回退文案与平台实证一致；其他值不渲染官网按钮——与官网逆向前端（select.js）`1==t?...:2==t&&...` 双分支逐字符对齐。✓
- **F43-N2 票据生命周期**：Login.tsx 五清票点（成功 80-81 / 过期 88 / 其他失败 95 / Esc 233-235 / 取消 303-305）对称；后端 ConsumeTicket 失败即毁票对齐。✓
- **F43-N3 动效插件**：`global.css @plugin "tailwindcss-animate"`；本轮 build 产物实测 `.animate-in` + `@keyframes enter/exit` 在 dist CSS（grep 命中）。✓

### 3. 上轮观察项 N-2~N-4 / O-1~O-5——裁决见 OBSERVE 节

N-2（双按钮 / 动效语言）维持 MINOR；N-3（401 保护窗口）维持 MINOR；O-1~O-5 维持 OBSERVE；O-5 与 O-2 同族（Admin 四查询 refetchInterval 亦无失败态降频），一并延续。

### 4. 本轮新视角——逐条核实无果（宁缺毋滥）

- **Dashboard 实时数据一致性**：logs 轮询 key `["logs", sessionToken]`（会话级无 account 维度，Dashboard 仅 current 路径渲染，不跨账号串线）；操作后 invalidate 与轮询交错无竞态（data 引用稳定、react-query 缓存复用）；dateGroups 分组时区——`parseDateKey(k+"T00:00:00")` 显式锁本地零点、`localTodayMs` setHours(0,0,0,0) 本地零点，两处同基准，跨日/跨年边界正确（UTC+8 凌晨不偏移）。**唯一边界**：`(c.begin_date ?? "").slice(0,10)` 依赖平台 beginDate 为 `YYYY-MM-DD` 前缀——Go 端直接透传平台原值（client.go:566 解析 / scheduler 存 BeginDate），无样本证实平台格式，但 slice 前缀截断对 `YYYY-MM-DD HH:mm:ss` 形态无损、对纯日期形态正确，属"假设成立则全对"的保守路径，**不报**。
- **Select actionLoading 模态清理**：handleSelectClass 报错 toast 后 invalidate 双查询（真实状态刷新，无乐观值需回滚）；退选报错时 `setExitModalClass(null)` 仅在成功分支执行——报错后模态保留、用户可重试/取消，取消即清理 `exitModalClass`，`ReadonlySet` finally 函数式删除只删本课 id，无残留。✓
- **Login 竞态**：loading 前置幂等短路 + 提交中 disabled 双守卫；错误清理 `setError("")` 于每次提交开头；success 后 `onLogin` 同步 setSessions/setCurrent（React 批处理单帧切换），无旧 promise 覆盖路径（1001 分支不 onLogin、票据失效分支清 pendingTicket 防重试闭环）。✓
- **App 会话过期全局处理**：onUnauthorized 快照式三连，每次重新 `loadSessions()` 读 localStorage（同步落盘），多账号并发 401 逐次收敛、互不误杀；`accounts.length===0` effect 驱动回登录页 + `setPage("dashboard")`；管理员被吊销 `setInAdmin(false)` 已就位（F25-01/F39-M1 延续）。✓
- **Radix Tabs/Switch 受控与刷新交错**：Admin activeTab 为 useState 受控（五 Tab 集合恒定不重建，无需回落）；Select activeTab 受控值带 `tabs.some(...)` 校验回落 `tabs[0]`（发布整体重建不悬空）；ConfigTab 的 configEpoch 代际 + refetch 后回填闭环正确。无 Radix Select（项目无下拉）。✓
- **useMemo/useEffect 依赖与内存泄漏**：全仓库扫描无 setInterval 泄漏（all `[]` effect + cleanup）；unmountedRef 双向复位（F20-01）；Admin copyTimer 卸载清理 + 连续复制 clearTimeout 前清（n8 已修）；useTickingCountdown 的 interval cleanup 正确。✓

---

## 结论

- **MAJOR 0 / MINOR 0（新）/ OBSERVE 3（2 新 + 1 延续）**，无功能级错误。B45-N1 双广播确证收敛（R45 MINOR 关闭），F43 四件套全绿，round42-45 无继承性回归。
- 最需主控留意 3 条（均低风险，按真实影响排序）：
  1. **O-1 Dashboard 窗口状态标识缺失 window_closed**——F13-B3 已连续观察 14 轮，展示一致性缺口（数据层已消费、四兄弟路由/组件已消费、唯独控制台主视图未消费），窗口关闭后误显"待命中"是用户可见的误导文案，建议下轮顺手三行修掉。
  2. **O-2 Dashboard /state 与 /logs 失败态不降频**——与 Select 侧 F40-M3/F42-M3 已收敛不对称，网络波动/后端重启期间 3s 轰炸默认落地页，两行可修。
  3. **N-2/N-3（双按钮 + 401 保护窗口）**——延续 MINOR，纯视觉/保守方向。
- tsc `--noEmit` 与 `npm run build` 全绿；本轮核心产出一为**确证 B45-N1 三形态全部单次广播（无遗漏形态）**，二为两条待收口的观察项。