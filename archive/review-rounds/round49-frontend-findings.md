# round49 前端审查原始发现

> 审查基线：master @ `cae3735`（round48 总结落盘：后端 2 修 + 前端 1 修 F48-M1 Toast 定位）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && git status --short` 干净；`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，产物 416.74 kB js / 44.73 kB css）；`npx oxlint src/` 仅既有 warning（全部历轮已核对过非功能 false-positive）。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/api/handler.go`（writeJSONStatus 家族 / handleAdminConfig PUT 500 / handleState 账号不存在拒绝 / admin stats window_closed 同源）、`backend/internal/scheduler/scheduler.go`（StateForAccount 开放时间槽 / tokenValid / markFullLocked 满员文案）、`backend/internal/zhidao/client.go`。
> 上轮（round48）MINOR-1（Toast 定位）→ 本轮 F48-M1 已修复，逐项复核见「重点核对 1」。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无新增 MINOR。）

- **N-2 / N-3 延续**（见 OBSERVE-2，维持 MINOR 级观察延续，历轮已确立，无新的功能影响）。

---

## OBSERVE（观察项，未加重）

### O-1.【延续 | round48 O-1 复证】五件零消费残件仍在：ui/Dialog、ui/Sheet、ui/Table、@radix-ui/react-select、@radix-ui/react-switch

- **文件路径**：`web/src/components/ui/Dialog.tsx`、`Sheet.tsx`、`Table.tsx`（grep 全 src 无一处外部 import）；`package.json` `@radix-ui/react-select ^2.3.7` / `@radix-ui/react-switch ^1.3.7`（node_modules 已装，src 零使用）。
- **一句话问题**：与 round48 完全相同的五件残件。项目实际弹层仍为三处手写裸 div 模态（Select 退选 :1177 / Admin 删除 :212 / Login 激活 :225，均无焦点陷阱）；激活码开关仍手写 `role="switch"` button（Admin.tsx:568）；账号表格仍手写 `<table>`。零功能影响，纯遗留债务。round48 建议的「要么迁移要么删」裁决仍未发生——本轮维持观察，不升级（宁缺毋滥）。
- **严重级**：OBSERVE。

### O-2.【延续 | round47 N-2/N-3 + O-3~O-5 复证】四组观察均维持原判

- **N-2（官网按钮 + 冲刺按钮双形态并存）**——复证仍成立（Select.tsx:1083-1150，btn_type`===1/===2` 渲染官网按钮、窗口信号决定冲刺按钮形态），双轨设计意图、持久化路径互不写冲突。维持 MINOR 级观察延续。
- **N-3（App 401 管理员代理保护窗口闭包 current）**——复证仍成立（App.tsx:147-192，`onUnauthorized` effect 依赖 `[adminName, inAdmin]`，:174 保护窗口读闭包 current；保守方向）。维持 MINOR 级观察延续。
- **O-3（三处手写模态无焦点陷阱）**、**O-4（401 保护窗口数据面）**、**O-5（Admin 五 Tab 后台轮询常跑：codes/stats/logs 5000 + accounts 10000 无失败态降频）**——复证仍在（Admin.tsx:319/705/791/879），维持 OBSERVE。

### O-3.【延续历轮 core-review】useTickingCountdown 每秒整页重渲 + oxlint warning 复核

- **文件路径**：`web/src/lib/useTickingCountdown.ts:8-12`、`web/src/routes/Dashboard.tsx:198`（渲染期 `Date.now()` 驱动 relativeCountdown）、Select.tsx:749。
- **一句话问题**：注释「只重渲染倒计时一处」与事实分叉（整组件每秒重渲，82 门量级实测无碍）。oxlint `react(purity)`（Dashboard.tsx:198）与 `react(set-state-in-effect)`（useTickingCountdown.ts:17 / Admin.tsx:505 / Dashboard.tsx:271）均为同一件事的静态印证，历轮已核过全部非功能（cleanup 读 ref 安全、effect 内 setState 为必须的自愈链）。延续，不报。
- **其余 oxlint warning**（Select.tsx:337 useMemo 依赖 / :403 ref-in-cleanup / :452 no-unsafe-finally / :676 missing deps `selectedCount`/`saveNow`）——历轮核过均非功能 false-positive。本轮特别复核 :676 缺依赖清单：防抖 effect 依赖数组含 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`，不引 `saveNow`/`selectedCount` 是有意设计（闭包捕获 + ref 镜像消费，见 F42-M1 注释），逐字符复核无误。

---

## 重点核对结论（任务书逐条裁决）

### 1. F48-M1（Toast 定位）——正确闭合，本轮逐文件核证

- **变更面**（git show eaa762f 确认）：外层空壳 wrapper `<div className="fixed bottom-4 right-4 z-50 ...">` 已删；布局类并入 viewport 的 className（`fixed bottom-4 right-4 z-50 flex w-full max-w-[380px] flex-col gap-2 p-4 pointer-events-none outline-none`，Toast.tsx:87，原 wrapper 的 `max-h-screen` 未随迁，但 ol 无 max-h 时内容超高靠 `w-full max-w-[380px] flex-col` 自然包裹、toast 数量级小，无功能影响）；`toasts.map` 位置移到 Provider 内 children 之后、Render Viewport 之前（:52-86）。
- **portal 机制逐行核证**（node_modules/@radix-ui/react-toast/dist/index.mjs）：`ToastImpl2` return 为 `ReactDOM.createPortal(<ToastInteractiveProvider>…</…>, context.viewport)`（:380-381）——作者写的 `toasts.map(<ToastRoot>)` 全部创建后 portal 进 context.viewport（= viewport 的 ol dom 节点）。`ToastViewport2` return 为 `<DismissableLayer.Branch role=region … style={{pointerEvents: hasToasts ? void 0 : "none"}}><FocusProxy/><Collection.Slot><Primitive.ol tabIndex={-1} {...viewportProps} /></Collection.Slot><FocusProxy/></DismissableLayer.Branch>`（:173-208）——Branch（react-dismissable-layer：`Primitive.div` 透传）是外包裹、className 经 viewportProps 落在 `<ol>` 上。因此 F48 布局并入 viewport className 后，**ol 本体即 `fixed bottom-4 right-4 z-50`**——定位类确凿生效，R48 MINOR-1 的修复方向与 Radix 源码完全对齐。
- **构建产物真实性**：`../backend/web/dist/assets/index-*.css` 含 `.animate-in[data-state=open]` / `.slide-in-from-bottom-full[data-state=open]` / `.fade-out-80[data-state=closed]` / `.slide-out-to-right-full[data-state=closed]` + `@keyframes enter/exit`（tailwindcss-animate v1.0.7 `matchUtilities` 生成），toast 进出场动画可用。✓
- **Swipe/pause 交互（本轮新视角）**：Swipe 逻辑挂在 toast Root 自有的 `onPointerDown/Move/Up`（index.mjs:409-468），与 viewport 的 `pointer-events-none`（只影响 Branch 的 pause 事件采集）无关——`pointer-events-auto` 在 Root 上（Toast.tsx:60），touch drag 正常触发 `data-swipe` 与 `swipeEnd/type=duration 3500` 关闭。Pause/resume：viewport 的 Branch div 无定位、内容被 ol(fixed) 拿走，Branch 自身尺寸≈0——hover pause 事件（pointermove on wrapper）现实中几乎不触发，与 F48 前 wrapper 空壳同款，非回归；pause 实际由 `focusin`（Tab 进 toast）与 `window blur/focus` 兜底。结论：**Swipe 交互正常、close-on-duration 正常、pause 依赖 focus/blur 而非 hover——行为与修复前后一致，无回归。**

### 2. 上轮观察项逐条裁决

- **O-1（五件零消费）**→ 延续 OBSERVE-1（范围与 round48 完全一致，未扩大）。
- **N-2 / N-3 / O-2~O-5**→ 延续 OBSERVE-2（全部维持原判，无继承回归）。
- **M-1（Toast 定位）**→ 已修复，见重点核对 1，关闭。

### 3. round42~48 关键契约复证（防回归抽样）

- **F43-N1 btn_type 渲染**（Select.tsx:1090-1115）——`===1` 退选 / `===2` 报名 / 其他值不渲染按钮，逐行复证。✓
- **F42-M1 shouldDeferSave 判据解耦**（targetGuard.ts:57-62 纯函数 + Select.tsx 三消费点 flushTargets:499 / handleBack:589 / 防抖:676 同源）——依赖数组含 `stateData`（:737）自愈链完整，resetRetry 在 effect 体首行（:648）、不引依赖是有意（F42-M1 注释「/state 数据到达触发 effect 重跑自愈」），oxlint missing-deps 为设计选择。✓
- **B45-N1 401 双广播收敛**（client.ts:64-96）——`r.status !== 401` 前置 gate 包住 body 层二次广播，逐字复证。✓
- **targetGuard 三函数**（selectedHasStalePublish / cleanStaleSelected / shouldDeferSave）——与 Select 消费点逐字符同步（防抖:696 / flush:518 / 回显 effect:287），TDD 纯函数断言脚本未变。✓

### 4. 本轮新视角——逐条核实

- **Toast swipe/pause（布局并入 viewport 后）**：见重点核对 1——Swipe 不受影响、pause 依赖 focus/blur 兜底，无回归。✓
- **Select priorityName/优先级排序（清空重建后）**：`pick()`（Select.tsx:346-372）用 `selected[publishId]` 数组保序，`selIdx`（findIndex）即 priority，toast 文案「备选 ${arr.length}」与 Dashboard/卡片 `priorityName(selIdx)`（:1009）一致；`targetsUseCurrentPublishes` 的 `priority: i` 按数组序落库——清空（arr=[]）后重建 arr 从 0 起排，优先级正确、无残留秩。✓
- **Dashboard relativeCountdown（切账号/代理态后重置）**：Dashboard 挂载点无 `key={account}`（App.tsx:286-291，仅 Select 两处有 key），但账号切换即 App 层 current 变化 → Dashboard 组件**实例整体重挂载**（条件渲染分支的同位置组件在 props 变化时保留实例？——React reconciliation：同类型组件 props 变化**不会**卸载重挂，而是保留实例仅更新 props！）。实测推演：Dashboard prop account A→B 变化时组件实例保留，`useQuery` key 含 account 自动取新数据，`expandedDates`/`extrasOpen` 状态跨账号残留（用户折叠状态带过账号）——**极小 UX 灰尘（折叠状态跨账号复用）**，无数据错乱（分组/倒计数据均来自 query 数据，account 变化即时刷新）。不报（宁缺毋滥），与 Select 的 key={account} 属不对称但无功能影响。
- **Admin config 表单（落库失败 HTTP 500）**：后端 `handleAdminConfig` PUT 落库失败现走 `writeJSONStatus(500, 500, ...)`（handler.go:810，F48-O3 家族整风）；前端 `api()` **从不读 HTTP status**，只读 body code（client.ts:71-98 只 `r.json().code` 判 `401`/`0`）——500 路径实际拿到 `code=500` 抛 `ApiError`，ConfigTab 的 catch（Admin.tsx:545-546）toast「保存失败」+ `e.message`（含「配置已生效但落库失败（重启后将回退）」后缀，可见）。✓ 正确显示。
- **App logout 与 onUnauthorized 并发双跳**：两路径各自快照式三连（loadSessions → 删 → saveSessions → setSessions），都是幂等；logout 主动删除 account 后，并发 in-flight 的 401 事件若携带同一会话令牌反查出已被删的账号 → `if (snap[lostAccount] === undefined) return` 静默跳过（App.tsx:183），无双删/误杀。`setInAdmin(false)`/`setTargetAccount`/`setPage` 幂等。✓
- **components/ui import 路径一致（no-unused）**：10 个 ui 组件 `cn` 全部 `from "../../lib/utils"`（utils.ts 唯一实现）；`tsconfig.app.json` 开 `noUnusedLocals` + `noUnusedParameters` 且 tsc exit 0，无死引用。Badge.tsx 用 `import * as React`（未直接引用但 jsx:react-jsx 下合法，tsc 不报）。✓

---

## 结论

- **MAJOR 0 / MINOR 0（新增）/ OBSERVE 3（延续）**。无功能级错误，tsc exit 0、build 成功。
- 最需主控留意 3 条（本轮全部为既有观察的延续，无新修复项）：
  1. **O-1 五件零消费残件（Dialog/Sheet/Table + react-select/react-switch 依赖）**——跨轮积累第四轮，仍未裁决「迁移 or 删除」；键盘可达性（三处手写模态无焦点陷阱）与依赖清洁度是该观察的真实成本。推荐下一轮给终裁（删除或迁移，二选一）。
  2. **N-2/N-3 + O-5（双按钮 / 401 保护窗口 / Admin 后台轮询无失败态降频）**——延续 MINOR/OBSERVE，纯视觉与保守方向。
  3. **O-6 useTickingCountdown 整页重渲与 oxlint warning 集群**——延续历轮 no-op，82 门量级无实际影响。
- 本轮核心产出：**F48-M1 闭合并逐文件核证**（viewport className 落点经 Radix 源码确认无误、构建产物含完整动画类、Swipe 交互不受 pointer-events-none 影响），六项新视角全部核实无果（无新 bug），发现 Dashboard 与 Select 的 `key={account}` 不对称（纯 UX 灰尘，不报）。