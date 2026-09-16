# 黑白极简艺术风格与零圆角 shadcn/ui 深度重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan ta***REMOVED***-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有至道选课系统的前端界面全面重构为具有瑞士国际主义设计风格、黑白单色极简艺术感、绝对零圆角（Zero-Radius）、且兼具极客控制台严谨性与现代画廊优雅感的工业级前端界面。

**Architecture:** 基于 React 19 + TypeScript + Vite 技术栈，采用 shadcn/ui 的组件设计哲学与 Radix UI 无障碍原语，在 `src/components/ui/` 目录下构建纯粹零圆角的原子组件体系，彻底重塑登录页、监控仪表盘和选课大厅三大页面。

**Tech Stack:** React 19, TypeScript, Vite, @radix-ui 原语系列, clsx, tailwind-merge, lucide-react, @tanstack/react-query

**Spec:** `docs/superpowers/specs/2026-09-11-monochrome-minimalist-ui-redesign.md`

## Global Constraints

- 全局所有组件与容器严禁使用圆角，严格采用绝对零圆角（`border-radius: 0 !important` / `rounded-none`）。
- 严格遵循黑白单色调色板（纯黑画布 `#09090b`、表面背景 `#121215` / `#18181b`、发丝边框 `1px solid #27272a`、纯白文字 `#fafafa`）。
- 全局时间戳、秒级倒计时、容量数字比例统一启用等宽数字（`font-variant-numeric: tabular-nums`）。
- 交互悬停统一采用 150ms 的黑白反色瞬时平滑过渡。
- 每个任务完成后立即运行编译检查，确保零类型错误，并立刻执行 git commit。

---

### Task 1: 基础设施与工具函数搭建

**Files:**
- Modify: `web/package.json`
- Create: `web/src/lib/utils.ts`

**Interfaces:**
- Produces: `cn(...inputs: ClassValue[]): string` 用于合并类名

- [ ] **Step 1: 安装必要依赖包**

在 `web` 目录下安装 `clsx`, `tailwind-merge`, `lucide-react`。
命令：`npm install clsx tailwind-merge lucide-react`

- [ ] **Step 2: 创建 `src/lib/utils.ts` 工具函数**

编写标准的 shadcn/ui `cn` 函数，封装 `clsx` 与 `twMerge`。

```typescript
import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

- [ ] **Step 3: 运行构建检查确保工具函数可用**

运行：`npm run build`
预期结果：构建通过，无类型错误。

- [ ] **Step 4: 提交代码**

```bash
git add web/package.json web/package-lock.json web/src/lib/utils.ts
git commit -m "feat(ui): install dependencies and add cn utility function"
```

---

### Task 2: 全局单色设计系统与 CSS 变量升级

**Files:**
- Modify: `web/src/styles/global.css`

**Interfaces:**
- Produces: CSS 变量 `--bg`, `--surface`, `--surface-soft`, `--border`, `--border-strong`, `--fg`, `--fg-muted`, `--fg-dim` 及零圆角全局重置规则

- [ ] **Step 1: 重写 `web/src/styles/global.css`**

配置符合黑白极简艺术风的色阶变量、强制零圆角、等宽数字全局类名、极细发丝边框样式和自定义暗黑滚动条。

```css
:root {
  --bg: #09090b;
  --surface: #121215;
  --surface-soft: #18181b;
  --border: #27272a;
  --border-strong: #52525b;
  --border-invert: #fafafa;
  --fg: #fafafa;
  --fg-muted: #a1a1aa;
  --fg-dim: #71717a;
}

* {
  border-radius: 0 !important;
  box-sizing: border-box;
}

html, body, #root {
  height: 100%;
  margin: 0;
  background: var(--bg);
  color: var(--fg);
}

body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.tabular-nums {
  font-variant-numeric: tabular-nums;
}

/* 极简发丝线边框 */
.hairline-border {
  border: 1px solid var(--border);
}

/* 自定义极客滚动条 */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-track {
  background: var(--bg);
}
::-webkit-scrollbar-thumb {
  background: var(--border);
}
::-webkit-scrollbar-thumb:hover {
  background: var(--border-strong);
}
```

- [ ] **Step 2: 运行构建验证**

运行：`npm run build`
预期结果：构建通过。

- [ ] **Step 3: 提交代码**

```bash
git add web/src/styles/global.css
git commit -m "style(ui): configure monochrome palette and zero-radius global styles"
```

---

### Task 3: 零圆角基础原子组件库（Button, Input, Badge, Card, Progress）

**Files:**
- Create/Modify: `web/src/components/ui/Button.tsx`
- Create/Modify: `web/src/components/ui/Input.tsx`
- Create: `web/src/components/ui/Badge.tsx`
- Create: `web/src/components/ui/Card.tsx`
- Create: `web/src/components/ui/Progress.tsx`

**Interfaces:**
- Produces: 
  - `Button`: 支持 `variant` ("default" | "outline" | "ghost" | "invert") 与 `size` ("sm" | "default" | "lg" | "icon")
  - `Input`: 纯粹发丝边框矩形输入框
  - `Badge`: 极细边框状态胶囊/标签 ("default" | "outline" | "secondary")
  - `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent`, `CardFooter`
  - `Progress`: 矩形无圆角几何进度条

- [ ] **Step 1: 重构 `web/src/components/ui/Button.tsx`**

实现多变体、绝对零圆角、带有 150ms 悬停黑白反色动画的 Button。

- [ ] **Step 2: 重构 `web/src/components/ui/Input.tsx`**

实现聚焦时发丝边框纯白高亮、无圆角的 Input。

- [ ] **Step 3: 创建 `web/src/components/ui/Badge.tsx`**

实现几何线框与反色标签组件。

- [ ] **Step 4: 创建 `web/src/components/ui/Card.tsx`**

实现统一 1px 细线边框、纯黑背景的精密卡片体系。

- [ ] **Step 5: 创建 `web/src/components/ui/Progress.tsx`**

实现外框 1px 细线、内部白色实心填充的零圆角进度条。

- [ ] **Step 6: 编译并验证所有基础组件**

运行：`npm run build`
预期结果：无 TypeScript 错误。

- [ ] **Step 7: 提交代码**

```bash
git add web/src/components/ui/
git commit -m "feat(ui): implement zero-radius core atomic components"
```

---

### Task 4: 复合交互组件库（Dialog, Table, Tabs）

**Files:**
- Create: `web/src/components/ui/Dialog.tsx`
- Create: `web/src/components/ui/Table.tsx`
- Create: `web/src/components/ui/Tabs.tsx`

**Interfaces:**
- Produces:
  - `Dialog`, `DialogTrigger`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`, `DialogFooter`
  - `Table`, `TableHeader`, `TableBody`, `TableHead`, `TableRow`, `TableCell`
  - `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent`

- [ ] **Step 1: 基于 @radix-ui/react-dialog 创建 `Dialog.tsx`**

实现暗黑半透明发丝边框遮罩与居中画廊模态框，带有右上角纯直角关闭图标。

- [ ] **Step 2: 创建 `Table.tsx`**

实现网格化数据表格，表头微缩大写字符，行悬停浅色过渡，数字右对齐。

- [ ] **Step 3: 基于 @radix-ui/react-tabs 创建 `Tabs.tsx`**

实现纯矩形分段控制器，选中项黑白反色高亮。

- [ ] **Step 4: 编译检查**

运行：`npm run build`
预期结果：编译通过。

- [ ] **Step 5: 提交代码**

```bash
git add web/src/components/ui/Dialog.tsx web/src/components/ui/Table.tsx web/src/components/ui/Tabs.tsx
git commit -m "feat(ui): implement zero-radius dialog, table, and tabs components"
```

---

### Task 5: 登录页面（Login.tsx）画廊海报级重构

**Files:**
- Modify: `web/src/routes/Login.tsx`

**Interfaces:**
- Consumes: `Button`, `Input`, `Card`, `cn`
- Produces: 居中极简艺术登录界面，保持原有 RSA 加密与图形验证码识别逻辑 100% 完整可用

- [ ] **Step 1: 重构 `web/src/routes/Login.tsx` 视觉布局**

采用瑞士画廊海报构图，极细十字对齐基准线，等宽字符标题 `ZHIDAO // ELECTIVES`，精致的验证码图片展示区与刷新按钮，高对比度反色登录按钮。

- [ ] **Step 2: 编译测试**

运行：`npm run build`
预期结果：无语法和类型错误。

- [ ] **Step 3: 提交代码**

```bash
git add web/src/routes/Login.tsx
git commit -m "feat(ui): redesign login route with monochrome gallery aesthetics"
```

---

### Task 6: 监控中心仪表盘（Dashboard.tsx）重构

**Files:**
- Modify: `web/src/routes/Dashboard.tsx`

**Interfaces:**
- Consumes: `Button`, `Card`, `Badge`, `Progress`, `cn`
- Produces: 极客监控仪表盘，包含大字号等宽数字倒计时脉搏器、Bento Grid 目标课程监控矩阵与复古控制台日志流

- [ ] **Step 1: 重塑顶部导航与引擎状态横幅**

设计极简极细顶栏，包含系统标题、实时状态小绿/白点、选课与登出操作按钮。

- [ ] **Step 2: 构建倒计时大字看板（Countdown Bento Card）**

将倒计时格式化为独立的 天/时/分/秒 网格单元，采用特大等宽字体排版。

- [ ] **Step 3: 构建预选目标监控卡片矩阵（Target Courses Bento Grid）**

展示 3 门预选课程，带有细线外框、发布编号标签、课程名称、状态指示徽章以及详细执行反馈。未配置时呈现优雅的空状态。

- [ ] **Step 4: 构建极客控制台风格的实时日志流（Console Terminal）**

纯黑背景，等宽发丝边框，左侧时间戳对齐，右侧高亮执行日志，支持滚动条平滑查看。

- [ ] **Step 5: 编译测试**

运行：`npm run build`
预期结果：编译通过。

- [ ] **Step 6: 提交代码**

```bash
git add web/src/routes/Dashboard.tsx
git commit -m "feat(ui): redesign dashboard with bento grid and tabular countdown"
```

---

### Task 7: 选课大厅（Select.tsx）矩阵卡片与详情弹窗重构

**Files:**
- Modify: `web/src/routes/Select.tsx`

**Interfaces:**
- Consumes: `Button`, `Input`, `Card`, `Badge`, `Progress`, `Dialog`, `Tabs`, `cn`
- Produces: 矩阵式选课大厅，包含实时搜索过滤栏、分类标签切换、课程卡片网格与画册级详情弹窗

- [ ] **Step 1: 重塑选课大厅顶栏与过滤工具栏**

返回控制台按钮、实时搜索输入框、仅看有剩余名额复选开关、分类选项卡。

- [ ] **Step 2: 重构课程卡片网格（Course Cards Grid）**

展示课程名、班级代号、教师列表、上课人数与计划人数比例，嵌入式零圆角容量进度条，一键设置为目标或报名按钮。

- [ ] **Step 3: 集成课程详情模态弹窗（ClassDetail Dialog）**

点击课程卡片“查看详情”后弹出，清晰呈现上课教室、排课节次、学期学分、授课方式等逆向抓包获得的完整信息。

- [ ] **Step 4: 编译测试**

运行：`npm run build`
预期结果：编译通过。

- [ ] **Step 5: 提交代码**

```bash
git add web/src/routes/Select.tsx
git commit -m "feat(ui): redesign select course hall with matrix cards and details dialog"
```

---

### Task 8: 整体构建打包与全链路交付验证

**Files:**
- None (全面校验与构建生成)

- [ ] **Step 1: 运行全量 TypeScript 与 Vite 构建**

运行：`npm run build`（在 `web` 目录）
预期结果：tsc 类型检查 0 错误，vite 打包生成 `dist/`。

- [ ] **Step 2: 验证后端 Go 单元测试与嵌入构建**

运行：`go test ./...`（在 `backend` 目录）
预期结果：全部测试通过。

- [ ] **Step 3: 更新根目录 CLAUDE.md 记录完成状态**

记录本次重构全部落地并验证完毕。

- [ ] **Step 4: 最终提交**

```bash
git add .
git commit -m "feat(ui): complete monochrome minimalist zero-radius ui overhaul"
```
