# round48 前端审查原始发现

> 审查基线：master @ `b6e2207`（round47 总结落盘，第四收敛轮双线零 MAJOR/MINOR、观察延续）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && git status --short` 干净；`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npx oxlint src/` 仅既有 warning。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/api/handler.go`（requireAuth/activate/login/logout/stats/state 契约）、`backend/internal/scheduler/scheduler.go`（OpenTime RFC3339 / beginTimes 识别 / 元数据透传）、`backend/internal/zhidao/client.go`（beginDate 平台原值透传）。
> 本轮对 Radix 源码做逐行核证（react-toast / react-tabs / react-dialog / react-presence 的 dist/index.mjs），纠正 R47 对上轮 O-1 的机制错记（见 MINOR 1）。
> 上轮（round47）OBSERVE 2 核对结论：
> - **O-1（Toast wrapper+viewport 双重 fixed）**——**本轮升级 MINOR**：R47 对该条的事实前提有误，真实机制比记录更脆弱（详 MINOR-1）。
> - **O-2（Dialog/Sheet 零消费）**——复证仍在（无外部 import），且范围扩大：Table.tsx、@radix-ui/react-select、@radix-ui/react-switch 同样零消费（见 OBSERVE-1）。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

### M-1.【新发现｜上轮 O-1 机制错记修正 + 布局真实依赖未言明浏览器行为】Toast 定位：wrapper 是空壳、toast 全部 portal 进 viewport，而 viewport 无任何 inset 类——R47 «固化了「viewport 自带 bottom-4 right-4 自定位」» 系事实错误

- **文件路径:行号**：`web/src/components/ui/Toast.tsx:48`（wrapper `<div className="fixed bottom-4 right-4 z-50 ...">`，内部 `{toasts.map(...)}`）、`:85`（`<ToastPrimitive.Viewport className="fixed outline-none pointer-events-none" />`）
- **一句话问题**：R47 O-1 论断「viewport 的**子 toast 定位完全依赖 viewport 自身的 `fixed bottom-4 right-4`（:85 类名已在）**」与代码不符——`:85` 实际只有 `fixed outline-none pointer-events-none`，**没有任何 inset（bottom/right/left/top）与 z-index 类**。且经 Radix 源码核证（react-toast/dist/index.mjs）：`ToastImpl2` 的 return 是 `ReactDOM.createPortal(<DismissableLayer.Root><Primitive.li>…</Primitive.li></DismissableLayer.Root>, context.viewport)`——**所有 toast 元素被 portal 注入 viewport 元素内部，wrapper 渲染完后是空壳 div**（map 的 Root 全部被转移走，wrapper 无实际子节点）。viewport 的 DOM 结构经 `DismissableLayerBranch`（react-dismissable-layer/dist/index.mjs：`<Primitive.div {...props}>`）→ `role=region` 包裹 → `Primitive.ol {...viewportProps}`（**className 传进 ol**）。因此真实定位机制是：**host 是 ol（viewport），它 `position:fixed` 且全部 inset 为 auto → 元素落点交给浏览器 static-position 行为**（CSS Positioned Layout L3 §6.1：无 inset 的 fixed 元素保持其静态位置）。wrapper 的 `fixed bottom-4 right-4 z-50` 约束不了任何现实内容；toast 的「bottom-right max-w-[380px]」外观只依赖浏览器对「文档流末尾、无 inset、fixed」元素的静态位置解释——含 max-w 的 w-full 全宽、以及 `#root{position:relative}` 对 static-position 锚定的相互作用，均未言明、未经实测。
- **证据**：
  - `Toast.tsx:48` 与 `:85` 两行类名逐字（上面引用）。
  - Radix `ToastImpl2` return（index.mjs）：`ReactDOM.createPortal(jsx(...), context.viewport)`（调用末尾 `), context.viewport ) })`），viewport 通过 `context.onViewportChange` 注册自身 DOM。
  - Radix `ToastViewport2` return：`<DismissableLayer.Branch role=region aria-label tabIndex:-1 style=…><FocusProxy/><Collection.Slot><Primitive.ol tabIndex:-1 {...viewportProps} ref/></Collection.Slot><FocusProxy/></DismissableLayer.Branch>`——**Branch 是 `Primitive.div` 透传 props**（dismissable-layer），`className` 经 viewportProps 落在 `<ol>` 上。
  - R47 记录「wrapper 仅以 z-index:auto 参与堆叠，其 inner 渲染顺序天然盖在 wrapper 之上」「Inner Viewport 在 wrapper 渲染上天然盖在 wrapper 之上」——基于「toast 保持在 wrapper 内渲染」的错误前提；实际 toast 已 portal 出 wrapper，这些推演全部失效。
- **触发场景推演**：无维度可复现的现有 bug 场景（历轮视觉从未报告错位，说明浏览器 static-position 行为碰巧落点正确）；但该行为依赖未言明的引擎细节——任何注入点变化（#root 定位方式、其它 fixed rooted overlay 出现在 DOM 树后、Radix Minor 升级改变 Branch 结构）都可能让 toast 整窗错位。修复是两行类名（给 viewport 的 className 补 `bottom-4 right-4 z-50 w-full max-w-[380px] p-4 gap-2`）并删空壳 wrapper。
- **严重级**：MINOR（纯布局可达性与维护性；「上轮确证无碍」的结论建立在错误事实前提上，需要实测/固化，但当前无功能性错误）。

---

## OBSERVE（观察项，未加重）

### O-1.【延续 | round47 O-2 扩大】建成的组件与依赖零消费残件：ui/Dialog、ui/Sheet、ui/Table、@radix-ui/react-select、@radix-ui/react-switch

- **文件路径**：`web/src/components/ui/Dialog.tsx`、`web/src/components/ui/Sheet.tsx`、`web/src/components/ui/Table.tsx`（整文件，grep 无任何外部 import）；`package.json` `@radix-ui/react-select ^2.3.7` / `@radix-ui/react-switch ^1.3.7`（安装于 node_modules 但 src 零使用；Switch 的手写 `role=switch` 按钮在 Admin ConfigTab:568 自成一套）
- **一句话问题**：五件从 scaffold 起的「已建成未消费」残件（含 package 依赖）。项目实际弹层仍是三处手写裸 div 模态（Select 退选 / Admin 删除 / Login 激活，均无焦点陷阱）；激活码操作 UI 用 `role=switch` 手写开关；账号表格用手写 `<table>`。零功能影响，纯残留债务。
- **触发场景**：无功能触发场景；属长期「设计意图层面债务」（Login.tsx:222 注释自认「完整焦点陷阱迁移到 Radix Dialog 属 F6-02 后续候选」，从未发生）。
- **严重级**：OBSERVE。建议下轮统一裁决：要么全部迁移 Radix 并删手写模态/表格/switch，要么干脆删未用组件与未用依赖。

### O-2./O-3./O-4./O-5.【round47 N-2/N-3 + O-3~O-5 延续复证】

- **N-2（官网按钮 + 冲刺按钮双形态并存）**——复证仍成立（Select.tsx:1083-1150），双轨设计意图，持久化路径互不写冲突，维持 MINOR 延续。
- **N-3（App 401 管理员代理保护窗口闭包 current）**——复证仍成立（App.tsx:174-176，`current===adminName && inAdmin && lostAccount!==adminName → return`，保守方向），维持 MINOR 延续。
- **O-2（三处手写模态无焦点陷阱，见 O-1）**、**O-3（401 保护窗口数据面）**、**O-4（Admin 五 Tab 无条件挂载后台轮询常跑：codes/stats/logs 5000 + accounts 10000 无失败态降频）**——均维持 OBSERVE。

### O-6.【延续历轮 core-review】useTickingCountdown 每秒 setNow 驱动整页整帧重渲染 + oxlint warning 复核

- **文件路径**：`web/src/lib/useTickingCountdown.ts:8-12`、`web/src/routes/Dashboard.tsx:198`（`const nowMs = Date.now()` 渲染期调用）、Select.tsx:749
- **一句话问题**：注释「只重渲染倒计时一处」与实现的整帧重渲分叉（Dashboard 已自认依赖它驱动 relativeCountdown）。oxlint `react(purity): Cannot call impure function during render`（Dashboard.tsx:198）是同一件事的静态印证。82 门数量级实测无碍，纯性能观察。延续。
- 其余 oxlint warning（Select.tsx:403 ref-in-cleanup / :452 no-unsafe-finally / :676 missing deps 等）均是非功能 false-positive（cleanup 读 ref 安全、finally return 无副作用），核过不报。

---

## 重点核对结论（任务书逐条裁决）

### 1. F46-F1/F2（Dashboard 三态 + 降频）——正确闭合

- **F46-F1 三态**：状态卡 `variant={state?.window_closed ? "outline" : state?.window_opened ? "primary" : "outline"}` + 文案「窗口已关闭|窗口已开放|待命中」（Dashboard.tsx:342-345）；手机悬浮栏同款三态（:728）。与 Select 横幅（F15-03）、Admin StatsTab（F39-N3）四兄弟同源同序，后端 window_closed 与学生端 /state 同源（handler.go:935 同 `d.Sched.WindowClosed()` 单源 windowClosedLocked）。✓
- **F46-F2 失败态降频**：/state（:140-145）与 /logs（:156-161）refetchInterval 顶部均 `query.state.error || query.state.status === "error" → 30000`，成功态按 `window_closed ? 30000 : 3000`（state 自身）/ 组件闭包 state（logs）。与 Select 侧 F40-M3/F42-M3 逐字符同源。✓
- **边界**：双查询失败态各自独立判定（互不降频对方）——延续 O-2-5 观察不变。

### 2. B45-N1（401 双广播收敛）——复证三形态各单次广播

- 前置 gate `client.ts:85 if (r.status !== 401)` 包住 body 层二次广播，逐字复证仍在。三形态（网关 HTML 401 / 旧式 HTTP 200+body 401 / writeJSONStatus 双 401 requireAuth 唯一出口 handler.go:1078）与 round46/47 推演一致，无新增形态。✓

### 3. round47 观察项裁决 + F43 四件套复证

- **R47 O-1** → 本轮 MINOR（见 MINOR-1：事实错记 + 真实机制）。
- **R47 O-2** → 延续 OBSERVE-1（范围扩大）。
- **F43 四件套**（M1 全清空放行 / N1 btn_type / N2 票据生命周期 / N3 动效插件）——逐条复证全绿（targetGuard.shouldDeferSave 三消费点同源；Select.tsx:1090-1115 btn_type`===1/===2` 渲染；Login.tsx 五清票点：80-81/88/95/233-235/303-305；global.css:2 `@plugin "tailwindcss-animate"` + build 产物含 `.animate-in`/keyframes 已验）。✓
- **F42-M1 判据解耦**（shouldDeferSave 纯数据判据、防抖 effect 含 stateData 依赖自愈）——复证正确。✓

### 4. 本轮新视角——逐条核实

- **Select tabs 受控 value 与 URL hash 同步**：grep 全 src 无 `location.hash`/`history.`/`URLSearchParams` 任何匹配——**该项目无 URL hash 同步机制**（刷新恒回落 tabs[0] 首个发布，属设计选择而非缺陷）。activeTab 受控回落逻辑（Select.tsx:903）正确（发布重建不悬空）。不报。
- **Dashboard electives（发布数据）与 courses 的关联（发布重建时分组同步）**：dateGroups useMemo 依赖 `[courses, electives?.publishes]`（Dashboard.tsx:219-263）。分组键第一步取 CourseStatus 自带 `begin_date`（随目标持久化、窗口关闭不丢），/electives 映射仅兜底第二分支——发布重建后 courses 即使暂带旧 publish_id，分组键仍由自带日期命中，**零依赖时序巧合**，30s 后 publishes 引用更新自动对齐。✓
- **Admin config 热配置表单（configEpoch 竞态、保存中重复点击）**：R27-01 已确立「PUT 成功后→refetch→再 setConfigEpoch」顺序（Admin.tsx:531-540）；`if (saving) return` + disabled 双幂等守卫（:518-520）；`!loaded` 时按钮 disabled + 函数内早退（:517）；refetch 失败不自增代际（表单保留用户输入）。configEpoch effect 依赖 `[loaded, configEpoch]`，react-query structural sharing 对同数据引用不变不重跑。无竞态可触发。✓
- **Login 激活码输入（粘贴/IME 组合、激活中 Esc）**：Input onChange 直接 setActivationCode（Login.tsx:264）；粘贴/IME 组合最终复用同一 onChange，激活码为手输字符串（XK-XXXX 等），后端按码值等值校验 + 票据绑定账号双重闸；组合输入产生的中文/乱序不会构成合法码（服务端拒，无状态危害）。激活中 Esc 显式 `!activating` 不响应（:232-238），「取消」按钮 activating disabled。无边界可触发 bug。✓
- **App useTickingCountdown 引用与切 Tab interval 清理**：Dashboard/Select 各自独立挂 useTickingCountdown，hook 内 `[]` effect + cleanup 正确（useTickingCountdown.ts:9-12）；切 Tab 卸载即 clearInterval、重挂载新 interval；Dashboard 依赖 electives 缓存回弹首帧即有目标。无 interval 泄漏、无跨路由残留。✓
- **api/client.ts 错误类型窄化（ApiError code 在 catch）**：`api()` 统一抛 `ApiError{code,msg,data}`（非 ApiError 的 fetch 网络异常原样冒泡，AbortError 映射为 code=-2）。catch 侧全仓库 `catch(e:any)`（Login.tsx:50/83、Select.tsx:97/125、Admin.tsx:271/338/358/545），`e.code===1001` 判据对 ApiError 恒正确、对原生 Error `e.code` undefined 自然落 else 分支取 `e.message`，`e.data?.ticket` 用 `?.` 防空。Dashboard `isSessionError` 用 `instanceof ApiError`（Dashboard.tsx:42-44）已窄化。无 type-unsafe 崩溃路径。✓

---

## 结论

- **MAJOR 0 / MINOR 1（新）/ OBSERVE 2（延续）×若干**。无功能级错误。
- 最需主控留意 3 条（按真实影响排序）：
  1. **M-1 Toast 定位机制**——R47 O-1 的「已论证无碍」建立在错误事实前提上（viewport 并无 bottom-4 right-4 类，且 toast 全部 portal 进 viewport、wrapper 是空壳），真实定位依赖「无 inset fixed 元素」的浏览器 static-position 行为，未实测、未言明。当前视觉正常属碰巧稳定，建议实测确认后两行固化（viewport 补 inset 类 + 删空壳 wrapper）。
  2. **O-1 五件零消费残件（Dialog/Sheet/Table/ui + react-select/react-switch 依赖）**——跨轮积累（Login.tsx 自认迁移候选从未发生），键盘可达性（手写模态无焦点陷阱）与依赖清洁度待统一裁决。
  3. **N-2/N-3 + O-4（双按钮 / 401 保护窗口 / Admin 后台轮询无失败态降频）**——延续 MINOR/OBSERVE，纯视觉与保守方向。
- tsc `--noEmit` 全绿（exit 0）；F46-F1/F2、B45-N1、F43 四件套、F42-M1 全部复证闭合；本轮核心产出为**确证「上轮榜首 Toast 断言」依赖错误事实并给出真实机制铁证（升级 MINOR）**，另核实六项新视角全部无果。