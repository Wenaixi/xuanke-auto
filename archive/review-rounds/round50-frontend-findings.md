# round50 前端审查原始发现

> 审查基线：master @ `7e89cfd`（round49 总结落盘：后端 1 修 + 前端零修，第六收敛轮）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && git status --short` 干净；`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，产物 416.74 kB js / 44.73 kB css，构建后 web 目录零 git 变更）。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/api/handler.go`（handleAdminConfig PUT 500 / handleState & handleElectives 账号不存在拒绝 / handleActivate 1001+ticket）、`backend/internal/scheduler/scheduler.go`（beginTimes 识别槽写入 / openTimeDetected 持锁）、`backend/internal/zhidao/client.go`。
> 上轮（round49）无前端修复项，F48-M1（Toast 定位）在 R49 已闭合；本轮继续换角落核查，全部核实无果。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无新增 MINOR。）

- **N-2 / N-3 延续**（见 OBSERVE-2，维持 MINOR 级观察延续，历轮已确立，无新的功能影响）。

---

## OBSERVE（观察项，未加重）

### O-1.【延续 | round46 O-1 起跨轮第 5 次复证】五件零消费残件仍在：ui/Dialog、ui/Sheet、ui/Table、@radix-ui/react-select、@radix-ui/react-switch

- **文件路径**：`web/src/components/ui/Dialog.tsx`、`Sheet.tsx`、`Table.tsx`（Grep 全 src 无一处外部 import）；`package.json:14/16` `@radix-ui/react-select ^2.3.7` / `@radix-ui/react-switch ^1.3.7`（node_modules 已装，src 零使用）。
- **一句话问题**：与 round46~49 完全相同的五件残件，第 5 次逐文件复证。项目实际弹层仍为三处手写裸 div 模态（Select 退选 Select.tsx:1175 / Admin 删除 Admin.tsx:210 / Login 激活 Login.tsx:223，均已有 role=dialog/aria-modal/aria-labelledby/Esc 关闭但**无焦点陷阱**，Tab 可穿出）；激活码开关仍手写 `role="switch"` button（Admin.tsx:568）；账号表格仍手写 `<table>`（Admin.tsx:804）。零功能影响，纯遗留债务。
- **跨轮记录**：round48 建议「要么迁移要么删」、round49 建议「下轮给终裁」，均未发生——本轮第 5 次复证后**正式提请下轮终裁（删除或迁移二选一）**。
- **严重级**：OBSERVE。

### O-2.【延续 | 历轮 N-2/N-3 + O-3~O-6 复证】五组观察均维持原判

- **N-2（官网按钮 + 冲刺按钮双形态并存）**——复证仍成立（Select.tsx:1083-1149，btn_type`===1/===2` 渲染官网按钮、窗口信号决定冲刺按钮形态），双轨设计意图、持久化路径互不写冲突。维持 MINOR 级观察延续。
- **N-3（App 401 管理员代理保护窗口闭包 current）**——复证仍成立（App.tsx:148-192，`onUnauthorized` effect 依赖 `[adminName, inAdmin]`，:174 保护窗口读闭包 current；保守方向）。维持 MINOR 级观察延续。
- **O-3（三处手写模态无焦点陷阱）**、**O-4（401 保护窗口数据面）**、**O-5（Admin 五 Tab 后台轮询常跑：codes/stats/logs 5000 + accounts 10000 无失败态降频）**、**O-6（useTickingCountdown 每秒整页重渲 + oxlint warning 集群）**——复证仍在（Admin.tsx:319/705/879/791；useTickingCountdown.ts 全量重读），维持 OBSERVE。

---

## 重点核对结论（任务书逐条裁决）

### 1. F48-M1（Toast 定位）——正确闭合，本轮新增多 toast 堆叠视角核证

- **变更面**（`git show eaa762f` 逐行确认）：外层空壳 wrapper `<div className="fixed bottom-4 right-4 z-50 ...">` 已删；布局类并入 viewport 的 className（`fixed bottom-4 right-4 z-50 flex w-full max-w-[380px] flex-col gap-2 p-4 pointer-events-none outline-none`，Toast.tsx:87）；`toasts.map` 移到 Provider 内 children 之后、Render Viewport 之前（:52-86）。
- **portal 机制**：Radix `ToastImpl2` return 为 `createPortal(<ToastInteractiveProvider>…, context.viewport)`——作者写的 `toasts.map(<ToastRoot>)` 全部 portal 进 viewport（ol dom 节点）；`ToastViewport2` 的 className 经 viewportProps 落在 `<ol>` 本体上。布局类并入后 **ol 即 `fixed bottom-4 right-4 z-50`**，定位确凿生效，R48 MINOR-1 方向与 Radix 源码对齐。
- **构建产物真实性**：`backend/web/dist/assets/index-*.css` 含 `.animate-in[data-state=open]` / `.slide-in-from-bottom-full[data-state=open]` / `.fade-out-80[data-state=closed]` / `.slide-out-to-right-full[data-state=closed]` + `@keyframes enter/exit`，toast 进出场动画可用。✓
- **多 toast 并发堆叠（本轮新视角）**：`toast()` 用 `setToasts(prev => [...prev, {…}])` 追加序（Toast.tsx:37）——数组序即 React key 序、即视觉渲染序（flex-col 自上而下），**新 toast 出现在底部、旧 toast 在上**，为常见通知栈约定；每条 toast 独立 `duration=3500` 各自计时 onOpenChange 删除，删除只按 id filter 不影响其余排序；无任何倒序/覆盖逻辑。F48 改动只动 wrapper→viewport 的 className 落点，堆叠序与修复前一致。✓ 非回归。
- **Swipe/pause 交互**：Swipe 挂在 toast Root 自有 pointer 事件（index.mjs），`pointer-events-none` 只在 viewport 上、`pointer-events-auto` 在 Root 上（Toast.tsx:60），touch drag 正常；pause 实际依赖 focus/blur 兜底（Branch 无定位尺寸≈0），与修复前行为一致。✓

### 2. 上轮观察项逐条裁决

- **O-1（五件零消费）**→ 延续 OBSERVE-1，第 5 次复证（范围与 round48 完全一致），本轮**正式提请下轮终裁**。
- **N-2 / N-3 / O-2~O-6**→ 延续 OBSERVE-2（全部维持原判，无继承回归）。
- **M-1（Toast 定位）**→ 已修复（R49 闭合），本轮复核见重点核对 1，维持关闭。

### 3. 本轮六项新视角——逐条核实

- **Select 过滤排序组合（onlyAvailable + sortTightest + search 三合一）发布重建后空态**：筛选链（Select.tsx:930-955）纯局部 `filteredClasses` 变量，不动 publishes/selected；发布重建仅影响 tabs 列表，`filteredClasses.length===0` 分支渲染虚线空态卡「没有符合当前搜索或筛选条件的选修课程」（:1156-1160），位于 `tabs.length > 0` 主区之内。三过滤组合全灭 = 空态卡正确展示，非"白屏/主区消失"。仅注意：排序发生在筛选之后，sortTightest 对 max_count=0 映射为 0（最紧张）——重建后新发布名额未公布课程在「按剩余排序」下排最前，与 F30/32-02 同源语义（0=未公布≠满员），规则一致非 bug。✓
- **Dashboard 日志列表窗口关闭后（30s 低频）加载体验**：`/state` 与 `/logs` 双查询 `refetchInterval` 均为「失败 30s / window_closed 30s / 常态 3s」（Dashboard.tsx:140-162），窗口关闭后降到 30s 低频；失败态同款降频不再轰炸。加载中无 skeleton——`logs===undefined` 时渲染「NO RECENT LOGS」空态（:712-716，`logs && logs.length > 0` 判断，undefined 走空态分支），非骨架屏，属极简设计风格（与全站一致），内容不闪烁（react-query 保留最后一次成功 data）。唯一尘埃：`LIVE` 徽章恒亮（:683），窗口关闭降频后仍显"实时"，无功能影响，不报（宁缺毋滥）。✓
- **Admin ConfigTab 在 F48-O3 后（落库失败 HTTP 500）错误文案一致性**：后端 `handleAdminConfig` PUT 落库失败现走 `writeJSONStatus(500, 500, nil, "配置已生效但落库失败（重启后将回退）："+sErr)`（handler.go:810）；前端 `api()` **从不读 HTTP status**，只 `r.json().code`（client.ts:97）→ 拿到 body `code=500` 抛 `ApiError(500, "配置已生效但落库失败…")` → ConfigTab catch（Admin.tsx:545-546）toast「保存失败」+ e.message。**文案与实际行为一致**（"配置已生效但落库失败"正是 PUT 已热更 runtime.Store、仅 SaveSettings 失败的真相）。✓
- **Login 激活成功后跳转与 session 写入时序**：`activate()`（Login.tsx:64-101）先 `setPendingAccount("")`/`setPendingTicket("")` 关模态框，再 `onLogin(data.token, data.account)` → App.login 同步写 loadSessions+saveSessions+setSessions+setCurrent。React 批处理下模态关闭与 session 写入同帧提交；`sessionToken = current ? sessions[current] : undefined` 在重渲染时才求值，**token 存根在跳转渲染前已完整写入**——不存在「跳转先行、token 后到」的时序洞。失败路径 catch 里清 ticket 不调 onLogin，不污染会话。✓
- **App onUnauthorized 在管理员代理态时学生账号被剔的跳转（lostAccount 流程）**：App.tsx:148-192。管理员 `inAdmin && current===adminName` 代看学生大厅（targetAccount=学生）期间，学生会话 401 → 事件携带的 session 是**管理员令牌**（Select 请求带 `session: sessionToken` 即管理员 Bearer），lostRaw 反查出 lostAccount=adminName → 不满足「`current===adminName && inAdmin && lostAccount!==adminName`」保护条件（lostAccount===adminName）→ 走完整剔除：清 targetAccount + setInAdmin(false) + 删 sessions[admin] → 回登录页。**管理员自身被剔 = 代理凭据连带失效，退出代理+退出管理态语义正确**；学生账号被剔而管理员未失效时保护窗口成立（丢 targetAccount，管理页保留）。无误杀/无卡死。✓
- **components/ui/Toast group 行为（多 toast 并发堆叠）**：见重点核对 1——追加序堆叠、各自计时、无覆盖，无 group 语义需求。✓

### 4. round42~48 关键契约复证（防回归抽样）

- **F43-N1 btn_type 渲染**（Select.tsx:1090-1115）——`===1` 退选 / `===2` 报名 / 其他值不渲染按钮，逐行复证。✓
- **F42-M1 shouldDeferSave 判据解耦**（targetGuard.ts:57-62 纯函数 + Select.tsx 三消费点 flushTargets:499 / handleBack:589 / 防抖:676 同源）——依赖数组含 `stateData`（:737）自愈链完整，resetRetry 在 effect 体首行（:648）不引依赖是有意设计，oxlint missing-deps 为设计选择。✓
- **B45-N1 401 双广播收敛**（client.ts:64-96）——`r.status !== 401` 前置 gate 包住 body 层二次广播；HTTP 401 在 r.json() 前先广播（F41-N2），body 层仅覆盖「HTTP 200 + body 401」旧形态，三种形态各单次广播。✓
- **targetGuard 三函数**（selectedHasStalePublish / cleanStaleSelected / shouldDeferSave）——与 Select 消费点逐字符同步（防抖:696 / flush:518 / 回显 effect:287），F40-M1 清理 effect（:316-327）与 F41-M1 注释完整。✓
- **beginTimes 识别槽**：后端 scheduler.go:848 `if len(data.BeginTimes) > 0` 持锁写入识别槽、空快照不删槽（关闭≠时间消失）——与前端 `open_time_known` 三硬契约对齐。✓

---

## 结论

- **MAJOR 0 / MINOR 0（新增）/ OBSERVE 2（延续）**。无功能级错误，tsc exit 0、build 成功、工作树零变更。
- 最需主控留意 3 条（本轮全部为既有观察的延续，无新修复项）：
  1. **O-1 五件零消费残件（Dialog/Sheet/Table + react-select/react-switch 依赖）**——跨轮第 5 次复证，round48/49 连续两轮建议终裁未发生；三处手写模态无焦点陷阱与依赖清洁度是该观察的真实成本。**本轮正式提请下轮终裁（删除或迁移二选一）。**
  2. **N-2/N-3 + O-5（双按钮 / 401 保护窗口 / Admin 后台轮询无失败态降频）**——延续 MINOR/OBSERVE，纯视觉与保守方向。
  3. **O-6 useTickingCountdown 整页重渲与 oxlint warning 集群**——延续历轮 no-op，82 门量级无实际影响。
- 本轮核心产出：**F48-M1 闭合复核新增多 toast 堆叠视角（追加序堆叠、各自计时、无覆盖）**，六项新视角全部核实无果（Select 过滤空态 / Dashboard 降频加载 / ConfigTab 500 文案 / Login 激活时序 / onUnauthorized 代理态 lostAccount / Toast group），发现 Dashboard `LIVE` 徽章窗口关闭后恒亮（纯 UX 尘埃，不报）与 Select/Dashboard `key={account}` 不对称复证（历轮已记录，不报）。
