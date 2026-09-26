# 前端 UI 细节重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变背景机制、后端契约和选课业务状态机的前提下，重构登录、学生看板、选课大厅和管理员后台的信息层级、黑白透明视觉、响应式编排和状态过渡。

**Architecture:** 保留现有 React 路由和 Radix UI 基础组件，以 `global.css` 与 `components/ui` 作为统一视觉入口，再按页面分别调整信息层级。状态过渡优先使用现有 CSS/Tailwind 类与 `prefers-reduced-motion`；不把 beUI、Rare UI、Transitions.dev 或 liquid-glass 变成必需运行时依赖。液态玻璃若要验证，只能在主线页面稳定后作为小面积、可回退的独立实验。

**Tech Stack:** React 19、TypeScript、Vite 8、Tailwind CSS 4、Radix UI Tabs、lucide-react、Vitest Node 环境、现有公共 UI 组件。

**Spec:** `archive/superpowers/specs/2026-09-26-ui-detail-refinement-design.md`

## Global Constraints

- 不能修改 `.canvas-bg`、`.canvas-bg-img`、`.canvas-bg-mask`、`.app-content`、背景图片、背景滚动位移或遮罩机制。
- 不能修改后端接口、轮询频率、调度、自动重登、账号隔离、目标保存、报名/退选状态机。
- `window_opened`、`window_closed`、`btn_type`、`can_select` 和平台 `title` 仍是业务状态唯一来源。
- 学生看板移除心跳周期、频控策略、教务令牌技术状态和原始调度日志；保留可行动的失败原因和重新登录入口。
- 管理后台保留激活码、配置、运行状态、账号、日志五个分区及诊断能力。
- 背景透明层必须在 Android WebView 保持可读；不得将 `backdrop-filter` 作为基础可读性前提。
- 动效只表达状态变化；必须支持 `prefers-reduced-motion`；不增加持续漂浮、鼠标追踪、眩光、数字翻牌或全页背景动画。
- 优先复用 `web/src/components/ui/`；只有纯逻辑边界才新增 Vitest，现有测试环境不引入 jsdom 或组件测试框架。
- 过程文档落在 `archive/superpowers/`；每个可独立验证的小模块完成后单独提交；不推送远程。

---

### Task 1: 建立公共视觉与动效基线

**Files:**
- Modify: `web/src/styles/global.css`
- Modify: `web/src/components/ui/Button.tsx`
- Modify: `web/src/components/ui/Card.tsx`
- Modify: `web/src/components/ui/Tabs.tsx`
- Modify: `web/src/components/ui/Badge.tsx`
- Test: `web/src/lib/courseView.test.ts`（仅在公共状态样式纯函数被新增时不扩展；本任务以构建和浏览器检查为主）

**Interfaces:**
- Consumes: 现有 CSS 变量、`.glass`/`.glass-strong`、公共组件 props、Radix Tabs data attributes。
- Produces: 四类页面共用的黑白透明面板、按钮、标签、焦点环、禁用态、减少动效规则；不改变现有组件公开 props。

- [ ] **Step 1: 盘点公共组件的现有样式入口**

  以 `ButtonProps.variant/size`、`Card` 家族、`TabsList/TabsTrigger/TabsContent` 和 `BadgeProps.variant` 为唯一公共视觉入口，记录需要保持的变体名称和尺寸名称。不得新增第二套同义组件。

- [ ] **Step 2: 调整全局状态过渡与减少动效规则**

  在 `global.css` 中将全局交互过渡限制为颜色、边框、透明度和轻微按压反馈；新增明确的 `@media (prefers-reduced-motion: reduce)` 规则，关闭非必要 transform/动画，同时保留焦点、颜色和状态变化。

- [ ] **Step 3: 调整公共按钮、卡片、标签和 Tabs 的层级**

  保持白底主操作、透明次操作、冷灰辅助信息和白色可见焦点环。Tabs 继续使用 Radix 的键盘行为和 ARIA 语义；只调整布局、边框、活动态和移动端换行，不改 `value/onValueChange` 逻辑。

- [ ] **Step 4: 检查背景机制没有被触碰**

  对 `global.css` 做差异检查，确认 `.canvas-bg`、`.canvas-bg-img`、`.canvas-bg-mask`、`.app-content` 的定位、z-index、尺寸和 transform 规则未被改写；确认 `.glass` 不重新启用 `backdrop-filter`。

- [ ] **Step 5: 验证并提交**

  Run: `cd web && npm run build`

  Expected: TypeScript 与 Vite 构建通过，输出仍写入 `backend/web/dist`。

  Run: `cd web && npm run guard`

  Expected: 现有性能、目标保存、管理员认证、未授权和懒加载守卫全部通过。

  Commit: `git add web/src/styles/global.css web/src/components/ui && git commit -m "refactor(web): unify monochrome interaction foundation"`

---

### Task 2: 重构登录页信息层级和反馈

**Files:**
- Modify: `web/src/routes/Login.tsx`
- Reuse: `web/src/components/ui/Button.tsx`, `web/src/components/ui/Card.tsx`, `web/src/components/ui/Input.tsx`

**Interfaces:**
- Consumes: Login 现有 `submit`、`loading`、`error`、`pendingAccount`、激活码模态框和密码显隐状态。
- Produces: 只改变登录页可见信息、布局和过渡，不改变登录/激活回调及认证错误行为。

- [ ] **Step 1: 删除内部机制页脚**

  删除“账号隔离”“实时调度”两项展示；保留账号/密码标签、认证说明、错误框、提交按钮和激活入口。不得删除任何状态变量、请求或激活码逻辑。

- [ ] **Step 2: 优化移动端登录布局**

  保持单列表单和足够的触摸尺寸；将标题、字段、错误、主按钮按提交路径排列。错误文本必须在窄屏内换行，不得被固定高度或透明面板裁切。

- [ ] **Step 3: 增加克制的提交与错误状态过渡**

  只使用公共 Button 状态、透明度/颜色过渡和现有图标；不新增持续加载动画。激活码模态框继续保持 `role=dialog`、`aria-modal`、标题关联和 Escape 行为。

- [ ] **Step 4: 验证登录展示边界**

  Run: `cd web && npm run build`

  Expected: 登录组件通过 TypeScript/Vite 编译，激活模态框没有 JSX 或可访问性错误。

  Smoke: 启动开发服务，检查桌面和窄屏登录页；验证空账号、错误认证、加载中、激活码模态框、密码显示/隐藏和 Escape 关闭。

- [ ] **Step 5: 提交**

  Commit: `git add web/src/routes/Login.tsx && git commit -m "refactor(web): simplify login surface"`

---

### Task 3: 重构学生看板的开放前后焦点

**Files:**
- Modify: `web/src/routes/Dashboard.tsx`
- Modify: `web/src/lib/courseView.ts`（仅在抽取看板所需纯展示决策时）
- Modify: `web/src/lib/courseView.test.ts`（与新增纯函数同步）

**Interfaces:**
- Consumes: `state.window_opened`、`state.window_closed`、开放时间解析、目标课程状态、`isSessionError`、现有 Query 数据和 `onGoSelect/onLogout` 回调。
- Produces: 开放前突出倒计时，开放后突出报名进度；学生界面不再渲染内部调度说明和原始日志；保留用户可行动状态。

- [ ] **Step 1: 明确看板展示状态的纯逻辑边界**

  如需抽取，新增只依赖输入数据的展示决策函数，例如返回 `"before-open" | "opened" | "closed" | "syncing"` 的状态，不读取 React 状态、不发请求、不改变后端信号。优先复用现有 `courseView.ts`，避免在 Dashboard 和 Select 各写一套开放时间解析。

- [ ] **Step 2: 为纯逻辑写边界测试**

  覆盖同步中、待命、窗口开放、窗口关闭，以及开放前 `begin_times` 兜底仍能显示时间的情况。测试不能把本地倒计时过期当作平台已开放；只有 `window_opened` 才能进入开放展示。

- [ ] **Step 3: 调整开放前布局**

  保留明显的开放时间/倒计时主区域；将目标课程移到下一层；删除“心跳轮询周期”“频控保护策略”“教务令牌”“后台调度心跳与通信机制”等学生不可操作的技术说明。状态文案使用用户能理解的词汇。

- [ ] **Step 4: 调整开放后布局**

  当 `state.window_opened` 为真时，倒计时区域切换为简洁窗口状态，目标课程和报名结果成为主要区域。保持稳定 key 和现有课程数据结构，避免开放前后切换时丢失折叠状态或目标展示状态。

- [ ] **Step 5: 收敛日志展示**

  移除学生看板的原始调度日志流及其轮询展示；保留课程卡中的成功、提交中、满员、失败结果和网络/凭据失效的行动入口。不要删除 API 类型或管理员日志页的数据消费。

- [ ] **Step 6: 验证并提交**

  Run: `cd web && npm test -- --run src/lib/courseView.test.ts`

  Expected: 看板相关纯函数测试通过。

  Run: `cd web && npm run build && npm run guard`

  Expected: 构建和全部前端守卫通过。

  Smoke: 使用可控状态数据分别检查开放前、已开放、已关闭、同步失败和凭据失效；桌面与窄屏确认首屏焦点、课程结果、重新登录按钮和背景滚动均正常。

  Commit: `git add web/src/routes/Dashboard.tsx web/src/lib/courseView.ts web/src/lib/courseView.test.ts && git commit -m "refactor(web): focus dashboard on window and enrollment state"`

---

### Task 4: 重构选课大厅的比较密度与移动端操作顺序

**Files:**
- Modify: `web/src/routes/Select.tsx`
- Reuse: `web/src/components/ui/Tabs.tsx`, `web/src/components/ui/Button.tsx`, `web/src/components/ui/Badge.tsx`, `web/src/components/ui/Progress.tsx`
- Test: `web/src/lib/targetGuard.test.ts`（仅当视觉调整涉及目标保存纯函数边界时；不改现有保存契约则不新增测试）

**Interfaces:**
- Consumes: 现有 `selected`、`actionLoading`、`btn_type`、`can_select`、`class_full`、搜索/排序/过滤和 `useTargetSave`。
- Produces: 桌面均衡紧凑课程比较、手机单列操作顺序；报名/退选、目标保存、平台禁用规则保持不变。

- [ ] **Step 1: 重新编排顶部信息**

  保留窗口状态、批次摘要、搜索和筛选；移动端按“窗口状态 → 批次切换 → 搜索/筛选 → 课程”顺序排列。继续使用现有 Radix Tabs，不改变活动批次和键盘导航。

- [ ] **Step 2: 收敛课程卡信息层级**

  课程名、优先级/选择状态、教师/地点、容量和操作按钮作为主信息；课程 ID、内部字段和重复提示降为辅助信息。保持 `class_full`、余量未公布、已选优先级和平台提示的语义准确。

- [ ] **Step 3: 调整按钮反馈但不改变业务判定**

  报名/退选按钮只根据现有 `btn_type` 渲染，根据现有 `can_select` 禁用，并继续使用平台 `title` 作为辅助提示。`actionLoading` 按课程 ID 独立展示提交中；成功/失败只在请求结果确认后呈现。

- [ ] **Step 4: 调整目标保存反馈与返回路径**

  保持 `useTargetSave` 的防抖、flush、回显、stale publish 守卫和 `handleBack` 行为；只调整 toast/状态视觉，不在 JSX 层新增第二套保存逻辑。返回按钮仍等待保存链稳定后执行回调。

- [ ] **Step 5: 验证边界**

  Run: `cd web && npm run build && npm run guard && npm test`

  Expected: 构建、守卫和纯函数测试全部通过。

  Smoke: 检查桌面多列、窄屏单列、批次切换、搜索、排序、仅看有余量、报名、退选、在飞禁用、满员、名额未公布、保存后返回；确认背景和请求时序未改变。

- [ ] **Step 6: 提交**

  Commit: `git add web/src/routes/Select.tsx && git commit -m "refactor(web): streamline course selection surface"`

---

### Task 5: 统一管理后台工作台视觉

**Files:**
- Modify: `web/src/routes/Admin.tsx`
- Reuse: `web/src/components/ui/Tabs.tsx`, `web/src/components/ui/Card.tsx`, `web/src/components/ui/Button.tsx`, `web/src/components/ui/Badge.tsx`

**Interfaces:**
- Consumes: 现有五个 Tab、配置保存、激活码复制/删除、账号代理/删除、统计、日志和删除确认 Dialog。
- Produces: 黑白透明运营工作台；保留高信息密度和全部管理能力，不改变管理 API、轮询门控或会话判定。

- [ ] **Step 1: 保持五 Tab 和管理出口**

  保留激活码、系统配置、运行状态、账号管理、日志总览及返回学生端/退出入口。不得把管理状态仅放进学生页或删除现有诊断字段。

- [ ] **Step 2: 统一 Tab、卡片、表单和状态层级**

  使用公共组件现有接口调整间距、边框、标题层级、输入态、错误态和危险操作态；保留管理页相对学生页更高的信息密度。重复的技术宣传性副标题可删，但配置含义和操作后果必须清楚。

- [ ] **Step 3: 优化移动端工作台**

  让 Tab 在窄屏可横向滚动或清晰换行，表单与操作按钮保持足够触摸区域，统计项和日志行不发生横向遮挡。危险操作确认 Dialog 保持可关闭、可读和键盘可达。

- [ ] **Step 4: 验证管理行为**

  Run: `cd web && npm run build && npm run guard`

  Expected: 构建和管理员认证守卫通过。

  Smoke: 依次检查五个 Tab 的加载/错误/空态、配置保存、激活码复制与删除、账号代理与删除确认、日志滚动和返回学生端；确认管理员会话状态不受视觉重构影响。

- [ ] **Step 5: 提交**

  Commit: `git add web/src/routes/Admin.tsx && git commit -m "refactor(web): refine admin operations workspace"`

---

### Task 6: 评估局部 liquid-glass 与完成全量验收

**Files:**
- Modify: `web/src/styles/global.css`（仅在实验通过且需要静态类时）
- Modify: `web/src/routes/Login.tsx`（仅在实验通过且需要挂载类时）
- Modify: `web/src/routes/Dashboard.tsx`（仅在实验通过且需要挂载类时）
- Modify: `web/src/routes/Select.tsx`（仅在实验通过且需要挂载类时）
- Modify: `web/src/routes/Admin.tsx`（仅在实验通过且需要挂载类时）
- Modify: `archive/superpowers/specs/2026-09-26-ui-detail-refinement-design.md`（仅记录实际采用/否决结果）
- Modify: `CLAUDE.md`（补充本次已落地的长期工程约束和实际采用结果）

**Interfaces:**
- Consumes: Task 1-5 的稳定页面、`samasante/liquid-glass` 的浏览器限制、现有透明背景和 Android WebView 约束。
- Produces: 采用一个小面积玻璃增强，或明确否决并保留现有静态透明方案；最终验证证据和项目记忆更新。

- [ ] **Step 1: 先做无依赖浏览器验收**

  启动 `web` 开发服务，检查桌面与窄屏登录、看板开放前/后、选课大厅、管理五 Tab。记录关键状态截图或观察结果，不先安装 liquid-glass。

- [ ] **Step 2: 评估玻璃实验是否必要**

  只选择一个小面积内容尺寸目标，例如单个状态徽标或操作反馈层；禁止用于整页、宽导航、倒计时主面板和课程卡网格。若静态 `.glass` 已满足透明效果，则直接否决新增依赖。

- [ ] **Step 3: 验证浏览器与移动端降级**

  检查 Chromium、Safari/Firefox（若环境可用）和 Android WebView 的可读性、黑块、滚动和交互帧率。任何环境出现黑块、文字对比下降或明显卡顿，都移除实验代码，保留现有静态透明层。

- [ ] **Step 4: 完成全量验证**

  Run: `cd web && npm test`

  Expected: 所有 Vitest 纯函数测试通过。

  Run: `cd web && npm run guard`

  Expected: 五个现有前端守卫全部通过。

  Run: `cd web && npm run build`

  Expected: 完整前端构建通过，产物可被后端嵌入目录消费。

  Smoke: 使用桌面和窄屏真实走完登录、激活、看板窗口状态、选课筛选/报名/退选/保存返回、管理员五 Tab；核对背景机制和关键状态语义。

- [ ] **Step 5: 更新长期项目记忆**

  在 `CLAUDE.md` 追加已验证的 UI 约束：学生页隐藏内部机制说明、看板开放前后焦点、管理页保留诊断能力、背景滤镜的 Android WebView 限制、液态玻璃最终是否采用。只记录实际落地事实，不记录未执行的候选方案。

- [ ] **Step 6: 提交最终验收模块**

  若 Task 6 实际修改了规格和项目记忆，只提交实际修改的文件：

  `git add archive/superpowers/specs/2026-09-26-ui-detail-refinement-design.md CLAUDE.md && git commit -m "docs: record verified UI refinement constraints"`

  若玻璃实验被否决且规格/项目记忆没有新增事实，则不创建空提交；Task 1-5 的页面提交已经构成完整代码变更记录。

  Do not add `xuanke-android-arm64.apk`; it is an existing unrelated untracked artifact.
