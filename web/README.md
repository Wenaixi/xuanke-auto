# 至道选课自动化 · 前端（web/）

React 19 + Vite + TypeScript + Radix Primitives + Tailwind CSS 构建的选课大厅前端，纯黑白极简风格。

## 开发

```bash
npm install
npm run dev          # 本地开发（Vite 代理 /api 到后端 :3091）
```

## 质检与构建

```bash
npm run build        # tsc -b + Vite 生产构建（产物输出到 backend/web/dist）——质量门
npm run test         # vitest 纯函数测试
npm run guard        # 六个源码形状守卫（性能 / 目标保存 / 管理态 / 401 / 懒加载 / 字体）
npm run lint         # oxlint 快速 lint
npx tsc --noEmit     # 注意：web/tsconfig.json 是 references 空壳，此命令不会真校验
```

前端回归以 `npm run build`（`tsc -b`）为准。构建产物通过 Go 原生 `//go:embed` 嵌入后端单二进制，最终用户不需要安装 Node.js。CI（`ci.yml`）每次 push/PR 都会跑 `build` + `guard` + `test`。

## 目录

- `src/routes/`：路由页面（Login / Select 选课大厅 / Dashboard / Admin 管理后台，路由级懒加载）
- `src/components/`：共享组件（含性能敏感的 `CountdownLeaf` 倒计时叶子）
- `src/lib/`：业务逻辑（`adminAuth` 管理态、`targetGuard` 目标保存守卫、`useTickingCountdown` 倒计时）
- `src/styles/global.css`：设计令牌唯一入口（字体、字号阶梯、面板透明档、灰阶）
- `scripts/`：防回归守卫脚本（`npm run guard` 执行，CI 也跑）
- `public/bg.jpg`：水墨背景图
- `public/favicon.ico`：浏览器标签图标（16/32/48 多尺寸）
- `public/logo.png`：页面左上角标题旁的 logo（128px）

## 依赖

- `@tanstack/react-query`：API 轮询与缓存（课程/状态/日志/管理后台）
- `@radix-ui/*`：Dialog / Tabs / Toast 等无头原语
- `lucide-react`：线性图标
- `@fontsource-variable/*`：自托管可变字体（Geist / Geist Mono / Noto Sans SC）
- `tailwindcss`：原子化样式

## 规范

- 涉及与后端交互的数据契约（如 /state、/electives、/admin/* 响应字段），改动时两端同步，以根 `README.md` 与后端 `backend/internal/api/handler.go` 为准。
- UI 设计语言：纯黑白 + 冷灰文字 + 1px 发丝边框，圆角控制在 4/6/8px，数字用等宽（`tabular-nums`）。主操作白底黑字，次操作透明底加冷灰边框，状态不单靠颜色区分。不要引入第二套配色或卡片体系，不用 emoji 和大圆角阴影。
- 字体与字号不要手写：统一用 `src/styles/global.css` 里的 `--font-*` 与 `--text-*` 令牌，`font-guard.ts` 会断言组件层没有手写 `font-family`、也没有绕过令牌的任意值字号。
