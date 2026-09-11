# 深度 shadcn/ui 现代温润人性化双端自适应 UI 重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan ta***REMOVED***-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于正统 shadcn/ui 架构与 Radix 原语，将至道选课前端从生硬冰冷的极客死黑直角界面，全面重构为现代高颜值、深空温润微圆角（6px-16px）、全流程人性化有温度、且完美适配手机与电脑双端的 Web 应用，并确保前端构建产物无缝嵌入 Go 二进制实现单文件便携交付。

**Architecture:** 前端独立采用 React 18 + Vite + TypeScript + Radix UI + Tailwind CSS 架构，沉淀正统 shadcn/ui 组件体系与移动端抽屉（Sheet）和轻量吐司（Toast）；三大核心路由（Login/Dashboard/Select）全流程消除技术黑话，注入自然语言状态与生机语义色（翡翠绿/琥珀金/珊瑚红/天青蓝）；生产构建物通过 Go 原生 `go:embed` 封装进 `backend/xuanke.exe`。

**Tech Stack:** React 19 / Vite 8 / TypeScript 6 / Tailwind CSS / Radix UI Primitives (@radix-ui/react-dialog, @radix-ui/react-toast, @radix-ui/react-tabs, @radix-ui/react-slot) / Lucide React / Go 1.22+ net/http + modernc.org/sqlite

**Spec:** `docs/superpowers/specs/2026-09-11-refined-rounded-ui-redesign.md`

## Global Constraints
- 彻底移除 `global.css` 中暴力全局规则 `* { border-radius: 0 !important; }`。
- 严禁出现未经转化的冰冷技术黑话（如 `code=1`、`null`、`undefined` 或单调死黑终端界面）。
- 圆角阶梯：`sm (6px)` 用于徽标 Badge、`md (8px)` 用于按钮 Button/输入框 Input/进度条、`lg (12px)` 用于卡片 Card/表格外框、`xl (16px)` 用于弹窗 Dialog/抽屉 Sheet。
- 双端响应式：移动端按钮/交互元素高度严格保证 ≥ 44px（触控友好），手机端课程详情采用底部滑出抽屉（Sheet/Drawer），绝不产生水平横向滚动条。
- 每次完成一个组件或一个核心页面的重构后，必须执行 `npm run build` 验证 TypeScript 编译与构建无误，并立即触发本地 Git 提交（本能 commit，禁止未经允许推送远程）。

---

### Task 1: 全局样式与现代温润设计令牌（Global Styles & Design Tokens）

**Files:**
- Modify: `web/src/styles/global.css`

**Interfaces:**
- Produces: 现代深空色阶变量群（`--bg: #0b0f17`、`--surface: #131b26`、`--surface-muted: #1c2636`、`--border: rgba(255,255,255,0.08)`、`--border-hover: rgba(255,255,255,0.18)`）、语义点缀色（`--emerald: #10b981`、`--amber: #f59e0b`、`--rose: #f43f5e`、`--cyan: #0ea5e9`）、微圆角令牌（`--radius-sm: 6px`、`--radius-md: 8px`、`--radius-lg: 12px`、`--radius-xl: 16px`）。

- [ ] **Step 1: 重写 global.css 清除零圆角硬编码并注入现代设计令牌**

编辑 `web/src/styles/global.css`，彻底移除 `* { border-radius: 0 !important; }`，引入深空色盘与温润交互动效。

- [ ] **Step 2: 验证样式文件编译无语法错误**

运行命令：`cd web && npm run build`
预期输出：`tsc` 与 `vite build` 成功通过。

- [ ] **Step 3: 提交 Git 变更**

```bash
git add web/src/styles/global.css
git commit -m "style: overhaul global design tokens with refined radius and ambient dark palette"
```

---

### Task 2: 深度重塑正统 shadcn/ui 组件库（UI Components Revamp）

**Files:**
- Modify: `web/src/components/ui/Button.tsx`
- Modify: `web/src/components/ui/Input.tsx`
- Modify: `web/src/components/ui/Badge.tsx`
- Modify: `web/src/components/ui/Card.tsx`
- Modify: `web/src/components/ui/Progress.tsx`
- Modify: `web/src/components/ui/Dialog.tsx`
- Modify: `web/src/components/ui/Tabs.tsx`
- Modify: `web/src/components/ui/Table.tsx`
- Create: `web/src/components/ui/Toast.tsx`
- Create: `web/src/components/ui/Sheet.tsx`

**Interfaces:**
- Button: `variant: "default" | "primary" | "success" | "warning" | "destructive" | "outline" | "secondary" | "ghost"`, `size: "default" | "sm" | "lg" | "icon"`, 自带 8px 微圆角与 44px 移动端尺寸自适应。
- Badge: `variant: "default" | "success" | "warning" | "destructive" | "primary" | "outline"`, 6px 微圆角。
- Card: `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`, 12px 微圆角与微光阴影。
- Progress: 8px 胶囊圆角，支持翡翠绿/天青蓝/琥珀金指示条。
- Dialog: 16px 圆角，带微光描边与漫反射景深。
- Sheet: 手机端底部上滑抽屉（Bottom Drawer），支持从底部平滑滑出展示课程详情。
- Toast: 弹出式轻量级操作反馈吐司通知。

- [ ] **Step 1: 重构 Button.tsx 与 Input.tsx**
实现温润微圆角、触控友好高度、平滑反色微光动画与清晰聚焦环。

- [ ] **Step 2: 重构 Badge.tsx 与 Progress.tsx**
赋予状态徽章生动的语义色彩（翡翠绿、琥珀金、珊瑚红、天青蓝）与 6px 微圆角；Progress 采用饱满胶囊滑槽。

- [ ] **Step 3: 重构 Card.tsx, Tabs.tsx 与 Table.tsx**
Card 采用 12px 微圆角与微光质感；Tabs 支持平滑药丸滑动效果；Table 容器采用 12px 圆角并柔化表头。

- [ ] **Step 4: 重构 Dialog.tsx 并创建移动端专属 Sheet.tsx (底部抽屉)**
Dialog 拥有 16px 圆角与浮空阴影；Sheet 基于 Radix Dialog 实现手机端底部上滑抽屉。

- [ ] **Step 5: 创建基于 Radix Toast 的友好通知组件 Toast.tsx**
封装 `ToastProvider`, `Toast`, `ToastTitle`, `ToastDescription`, `useToast`，提供人情味即时反馈。

- [ ] **Step 6: 执行 TypeScript 编译与构建验证**

运行命令：`cd web && npm run build`
预期输出：零错误构建成功。

- [ ] **Step 7: 提交 Git 变更**

```bash
git add web/src/components/ui/
git commit -m "feat(ui): implement deep shadcn/ui components with refined radius, sheet, and toast"
```

---

### Task 3: 登录界面全流程人性化重塑（Login.tsx Revamp）

**Files:**
- Modify: `web/src/routes/Login.tsx`

**Interfaces:**
- 人性化登录体验：欢迎引导语、会话安全状态微标（`🟢 会话就绪 · RSA-1024 密钥安全保护中`）、密码明暗小眼睛切换、分步骤自然语言状态（连接中 -> 智能识别验证码 -> 登录就绪）、手机端居中沉浸自适应。

- [ ] **Step 1: 重构 Login.tsx 界面与人机交互**
用温润的 16px 浮层卡片替代冷硬直角框，加入 Lucide 图标（`ShieldCheck`, `Eye`, `EyeOff`, `Sparkles`, `Lock`, `User`），加入友好提示与动态状态展示。

- [ ] **Step 2: 验证登录界面构建无类型错误**

运行命令：`cd web && npm run build`
预期输出：编译无错误。

- [ ] **Step 3: 提交 Git 变更**

```bash
git add web/src/routes/Login.tsx
git commit -m "feat(ui): redesign login flow with humanized feedback, rsa pill, and soft rounded card"
```

---

### Task 4: 仪表盘巨幕倒计时与目标监控人性化重塑（Dashboard.tsx Revamp）

**Files:**
- Modify: `web/src/routes/Dashboard.tsx`

**Interfaces:**
- 巨幕自然语言倒计时看板：大号等宽脉搏数字（天/时/分/秒） + 人性化时间解读（“距离 2026-09-13 09:00:00 开抢 还有 X 天 X 小时”）。
- 核心抢课备战小贴士：温润微光卡片提示抢课准备策略。
- 3 门重点看护目标课程卡片：翡翠绿/琥珀金进度条，容量实时紧迫度提示。
- 人性化活动动态流：用通俗易懂的自然语言动态替换生硬的 UNIX 极客黑白代码日志。
- 手机端专属底部悬浮操作栏（Mobile Dock）：单手大拇指轻松启停抢课引擎。

- [ ] **Step 1: 重构 Dashboard.tsx 布局与数据渲染**
实现 Bento Grid 响应式排版（PC 多列大屏，手机自适应单列），集成巨幕倒计时卡片、目标课程卡片、自然语言动态面板与移动端悬浮底栏。

- [ ] **Step 2: 验证仪表盘构建无误**

运行命令：`cd web && npm run build`
预期输出：零错误构建。

- [ ] **Step 3: 提交 Git 变更**

```bash
git add web/src/routes/Dashboard.tsx
git commit -m "feat(ui): redesign dashboard with natural countdown, target cards, humanized feed, and mobile dock"
```

---

### Task 5: 选课大厅画册级体验与双端自适应抽屉详情（Select.tsx Revamp）

**Files:**
- Modify: `web/src/routes/Select.tsx`

**Interfaces:**
- 直观人性化状态标签：`🔥 仅剩 X 席`、`🌿 名额充裕`、`🔒 已满额`、`✨ 监控队列中`。
- 多维度模糊搜索与快捷过滤：支持搜索课程名/教师/地点，支持一键筛选“有余量”和“已选/目标”。
- 手机端自适应双模态详情：大屏使用 16px 微圆角模态 Dialog，手机端自动变身底部上滑抽屉 Sheet。
- 一键报名/退选交互：触控大按钮，成功时触发 Toast 贴心提示。

- [ ] **Step 1: 重构 Select.tsx 页面结构与交互逻辑**
引入微圆角课程卡片、生动徽标、多维度筛选工具栏、响应式双端详情展示（PC Dialog / 移动端 Sheet）及 Toast 状态通知。

- [ ] **Step 2: 验证选课大厅构建成功**

运行命令：`cd web && npm run build`
预期输出：零错误构建。

- [ ] **Step 3: 提交 Git 变更**

```bash
git add web/src/routes/Select.tsx
git commit -m "feat(ui): redesign select hall with humanized capacity badges, responsive sheet, and toast feedback"
```

---

### Task 6: 导航系统与移动端自适应整合（Layout & App.tsx Revamp）

**Files:**
- Modify: `web/src/App.tsx`

**Interfaces:**
- 顶部温润毛玻璃导航栏：带有柔和微光边框，显示系统当前会话状态、当前页面切换卡。
- 移动端防误触触控区自适应，确保在手机横竖屏与 PC 宽屏下均自如伸展。

- [ ] **Step 1: 优化 App.tsx 全局布局与导航交互**
配置全局 ToastProvider，完善顶部导航栏与页面容器的响应式内边距（移动端预留底部悬浮栏空间，杜绝遮挡）。

- [ ] **Step 2: 执行前端全量构建**

运行命令：`cd web && npm run build`
预期输出：`dist/` 静态产物顺利生成。

- [ ] **Step 3: 提交 Git 变更**

```bash
git add web/src/App.tsx
git commit -m "feat(ui): integrate global toast provider and responsive navigation layout"
```

---

### Task 7: Go 后端静态产物单二进制嵌入与路由验证（Backend Embedding Verification）

**Files:**
- Verify: `backend/main.go`
- Verify: `backend/internal/web/web.go` (若有) 或静态托管逻辑

**Interfaces:**
- `go:embed` 将 `web/dist` 打包进单一二进制可执行文件 `xuanke.exe`。
- 访问 `http://localhost:8080/` 时正确回退至前端单页应用（SPA fallback）。

- [ ] **Step 1: 检查 Go 后端静态资源嵌入代码与路由分发**
确保 Go 1.22+ 路由能够无缝提供 API（`/api/*`, `/events`）并对前端静态资源提供完整的 SPA 支持。

- [ ] **Step 2: 执行 Go 单元测试与单二进制编译构建**

运行命令：`cd backend && go test ./... && go build -o xuanke.exe .`
预期输出：测试全部 PASS，成功生成便携单二进制文件 `xuanke.exe`。

- [ ] **Step 3: 提交 Git 变更**

```bash
git add backend/
git commit -m "chore(build): verify frontend embedding in single executable binary"
```

---

### Task 8: 全链路端到端构建与双端视觉验收（End-to-End Verification）

**Files:**
- Review all changed files.

- [ ] **Step 1: 运行前端严格类型检查与完整打包**
运行命令：`cd web && npm run build`
确保零 Warning/零 Error，产物大小精炼。

- [ ] **Step 2: 运行后端竞态测试与编译**
运行命令：`cd backend && go test -race ./... && go build -o xuanke.exe .`
确保抢课调度器等核心逻辑在并发竞态检查下 100% 稳健通过。

- [ ] **Step 3: 最终成果归档与 Git 记录**
整理工作树状态，确认全部改动均已本地 Commit，准备向主人汇报完工！
