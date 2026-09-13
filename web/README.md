# 至道选课自动化 · 前端（web/）

React 18 + Vite + TypeScript + Radix Primitives + Tailwind CSS 构建的选课大厅前端，纯黑白极简艺术风格设计。

## 开发

```bash
npm install
npm run dev          # 本地开发（Vite 代理 /api 到后端 :3091）
```

## 质检与构建

```bash
npx tsc --noEmit     # TypeScript 类型检查
npm run build        # tsc 类型检查 + Vite 生产构建（产物输出到 backend/web/dist）
```

构建产物通过 Go 原生 `//go:embed` 嵌入后端单二进制，最终用户无需安装 Node.js。

## 依赖

- `@tanstack/react-query`：API 轮询与缓存（课程/状态/日志/管理后台）
- `@radix-ui/*`：Dialog / Tabs / Toast 等无头原语
- `lucide-react`：线性图标
- `tailwindcss`：原子化样式