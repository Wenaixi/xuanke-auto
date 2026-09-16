# 黑白极简艺术风格与零圆角 shadcn/ui 深度重构设计规范

## 1. 概述与设计理念

本文档定义了至道选课自动化系统前端界面的深度重构规范。目标是将现有简陋、粗糙的界面全面重构为具有瑞士国际主义设计风格、黑白单色极简艺术感、绝对零圆角（Zero-Radius）、且兼具极客控制台严谨性与现代画廊优雅感的工业级前端界面。

重构基于 React 19 + TypeScript + Vite 技术栈，采用 shadcn/ui 的组件设计哲学与 Radix UI 无障碍原语，配合纯粹的单色系设计系统，实现形式追随功能、线条冷峻利落的高级视觉体验。

## 2. 核心视觉系统规范

### 2.1 纯粹单色调色板（Monochrome Palette）
完全摒弃杂乱的高饱和色彩，通过黑白灰的明度阶梯与反转对比构建视觉层级：

- 背景层：
  - 主画布背景（Canvas）：#09090b（纯黑底色）
  - 卡片/表面背景（Surface）：#121215（微高明度黑底，营造空间层次）
  - 弱化背景（Muted / Soft）：#18181b（用于表头、次级卡片底色）
- 边框与线条层：
  - 默认细线（Border）：#27272a（冷灰发丝线，1px 绝对利落）
  - 聚焦/强化边框（Border Strong）：#52525b
  - 高亮边框（Border Invert）：#fafafa（白线，用于焦点卡片）
- 文字与前景层：
  - 核心主标题/主高亮（Foreground）：#fafafa（纯净雪白，高对比度）
  - 次要说明文本（Muted Foreground）：#a1a1aa（优雅浅灰）
  - 弱化辅助信息（Dim）：#71717a（暗灰，用于标签、版权、次要时间戳）
- 状态指示（单色几何形态区分）：
  - 成功/开放（Active / Success）：#fafafa 实心白块 + 闪烁微光，或白底反转黑字
  - 警告/等待（Pending / Warning）：#a1a1aa 线框空心块
  - 危险/异常（Error / Danger）：#ffffff 纯白外线描边伴随斜向几何纹理提示

### 2.2 绝对零圆角原则（Zero-Radius Mandate）
- 全局所有元素的 border-radius 严格设置为 0px。
- 严禁出现任何圆弧、药丸型胶囊按钮或椭圆。
- 所有徽章（Badge）、按钮（Button）、卡片（Card）、输入框（Input）、弹窗（Dialog）、进度条（Progress）均为纯粹的直角四边形。

### 2.3 排版与字阶系统
- 英文字体优先使用 Inter, system-ui, -apple-system，搭配等宽字体 JetBrains Mono / SFMono-Regular。
- 中文字体统一采用 PingFang SC, Microsoft YaHei，清晰锐利。
- 数据排版全面启用等宽数字（font-variant-numeric: tabular-nums），确保秒级倒计时、人数统计与时间戳上下绝对对齐。
- 标题排版采用大写宽字距（tracking-widest / tracking-[0.25em]），呈现现代杂志封面般的构图张力。

### 2.4 微交互规范
- 按钮触感：悬停时触发 150ms 的瞬间反转动画（白底变黑字，或黑底变白字），带有发丝级边框变化。
- 列表与表格行：鼠标滑过时应用细腻的 #18181b 背景色渐变过渡。

## 3. 组件库架构（shadcn/ui 风格体系）

在 `web/src/components/ui/` 目录下建立模块化、高复用、零圆角的原子组件：

1. `utils.ts`：
   - 提供 `cn(...inputs)` 工具函数（集成 clsx 和 tailwind-merge），用于条件式类名合并。
2. `Button.tsx`：
   - 包含变体：default（纯白边框黑底，hover白底黑字反色）、outline（灰边框透明底）、secondary（弱化表面底色）、ghost（透明底悬停微显）、destructive（白边黑底加反色强调）。
   - 尺寸：sm, default, lg, icon。
3. `Card.tsx`：
   - 包含 Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter。
   - 统一 1px #27272a 边框，无圆角，带有均匀的几何内边距。
4. `Badge.tsx`：
   - 用于课程状态、窗口状态、发布标签。
   - 提供 default（实心白底黑字）、outline（细边灰字）、secondary（微黑灰底）三种变体。
5. `Input.tsx`：
   - 优雅的 1px 发丝边框输入框，focus 时边框瞬间转变为 #fafafa，无圆角。
6. `Progress.tsx`：
   - 纯粹几何直角进度条，外框 1px 细线，内部填充为纯白色等宽条块。
7. `Table.tsx`：
   - 包含 Table, TableHeader, TableBody, TableHead, TableRow, TableCell。
   - 严格的表格网格线，表头字母大写浅灰微缩，数字等宽对齐。
8. `Dialog.tsx`：
   - 基于 @radix-ui/react-dialog 封装的极简线框遮罩与模态弹窗，用于展示课程详情（授课教师、教室、学分模式等）。
9. `Tabs.tsx`：
   - 基于 @radix-ui/react-tabs 封装的矩形分段控制器，选中项具有醒目的反色高亮。

## 4. 页面重构架构蓝图

### 4.1 登录页（Login.tsx）
- 构图：居中极简画廊构图，带有极细的水平与垂直构图辅助线。
- 头部：大字距冷冽标题 `ZHIDAO // ELECTIVE AUTOMATION`，副标题呈现严谨技术参数说明。
- 表单：账密输入框带有等宽标签，验证码识别状态以极简状态指示器呈现。
- 交互：登录按钮具有强烈的反色视觉反馈，错误提示以极简线框警示条呈现。

### 4.2 控制中心仪表盘（Dashboard.tsx）
- 顶栏：
  - 极简艺术 Logo 与版本标示。
  - 右侧提供紧凑的全局操作按钮（进入选课大厅、断开连接/注销）。
- 状态与时间总览（Hero Banner）：
  - 左侧：醒目震撼的大字号等宽数字倒计时（天/时/分/秒独立网格分割），具有跳动的时钟脉搏。
  - 右侧：调度器并发引擎状态（未开放 / 正在秒级轮询 / 窗口已开抢课中）。
- 核心目标监控区（Target Courses Grid）：
  - 采用 Bento Grid 排版，展示用户预选的 3 门目标课程（体育、校本1、校本2）。
  - 每个卡片清晰标示发布编号、课程代码、名称、报名当前状态（等待窗口/已报名成功/失败）。
  - 若未配置目标，提供充满设计感的空状态占位与引导按钮。
- 事件流终端（Live Event Logs）：
  - 采用复古极客控制台风格，深黑底色，等宽字体滚动呈现毫秒级执行日志，支持清晰的时间戳标定。

### 4.3 选课大厅（Select.tsx）
- 检索与筛选栏：
  - 顶部搜索框（支持实时按课程名称模糊过滤）。
  - 分类切换标签（全部、体育课、校本选修一、校本选修二）。
  - 状态快捷过滤（全部课程 / 仅看有剩余名额）。
- 课程卡片瀑布流 / 矩阵：
  - 每门课程卡片呈现完整关键信息：课程名称、班次编号、教师名、当前人数/计划人数比例。
  - 嵌入式直角细线容量进度条。
  - 底部操作栏：查看详细信息（呼出详情弹窗）、设定为目标课程（或一键直接报名/退选）。
- 详情弹窗（ClassDetail Dialog）：
  - 呈现逆向抓包所获取的上课地点、上课节次、考核方式、学分属性、课程描述等画册级细节排版。

## 5. 依赖与工程计划

1. 引入必要辅助库：
   - 安装 `clsx` 与 `tailwind-merge`（用于组件类名构建）。
   - 安装 `lucide-react`（用于统一的高级细线矢量图标）。
2. 构建 UI 组件库（Button, Input, Card, Badge, Table, Progress, Dialog, Tabs）。
3. 改造全局 CSS（`global.css` 完善颜色变量、等宽数字支持、发丝细线与滚动条规范）。
4. 依次重构三大页面并逐一通过 TypeScript 编译与渲染测试。
5. 最终验证与本地构建打包验证。
