# 至道选课自动化 · 前端（web/）

React 19 + Vite + TypeScript + Radix Primitives + Tailwind CSS 构建的选课大厅前端，纯黑白极简艺术风格设计。

## 开发

```bash
npm install
npm run dev          # 本地开发（Vite 代理 /api 到后端 :3091）
```

## 质检与构建

```bash
npx tsc --noEmit     # TypeScript 类型检查
npm run build        # tsc 类型检查 + Vite 生产构建（产物输出到 backend/web/dist）
npm run guard        # 五个防回归守卫断言（性能/目标保存/管理态/401/懒加载契约）
npm run lint         # oxlint 快速 lint
```

构建产物通过 Go 原生 `//go:embed` 嵌入后端单二进制，最终用户无需安装 Node.js。

## 目录

- `src/routes/`：路由页面（Login / Select 选课大厅 / Admin 管理后台，懒加载）
- `src/components/`：共享组件（含性能敏感的 `CountdownLeaf` 倒计时叶子）
- `scripts/`：防回归守卫脚本（`npm run guard` 执行，CI 也跑）
- `public/bg.jpg`：水墨背景图

## 依赖

- `@tanstack/react-query`：API 轮询与缓存（课程/状态/日志/管理后台）
- `@radix-ui/*`：Dialog / Tabs / Toast 等无头原语
- `lucide-react`：线性图标
- `tailwindcss`：原子化样式

## 规范

- 前端回归以 `npm run build` 为准（项目根 `tsc --noEmit` 是 references 空壳，不报错，必须 `tsc -b`）。
- 涉及与后端交互的数据契约（如 /state、/electives、/admin/* 响应字段），改动时两端同步，参考后端 `handler.go` 与 `archive/review-rounds/` 中的决策契约。