# 0001 前端产物 embed 进单二进制

## Status

已接受。

## Context

本工具要在学生自己的 Windows 机器上双击即用，另有 Android APK 形态。用户机不能假设装有 Node 或 Go，也不能要求另起一个静态资源服务、再配反向代理和跨域。而前端（React + Vite）的产物理所当然是一堆静态文件。

## Decision

前端构建产物由 `//go:embed all:dist` 打进后端二进制，同一个进程既服务静态文件也服务 `/api`。发布流水线固定「先 `npm run build`、再 `go build`」。

## Consequences

- **embed 是编译期固化**：前端改完不重新 `npm run build` + `go build`，服务端发出的仍是旧资源 hash，浏览器看不到新 UI——验证界面必须按「构建前端 → 编译后端 → 启动」的顺序走，否则会把「代码没生效」误判成缺陷。代价是改前端要重新编译整个二进制，二进制体积也随前端产物增长。
- `web/dist/` 与 `backend/web/dist/` 都被忽略，他人克隆后不构建就没有可嵌入的 dist，`go build` 会因 embed 找不到目录而失败——这正好把「忘了构建前端」暴露在编译期而不是运行期。
- 换来的是零外部依赖的交付：没有静态目录、没有反代、没有跨域与端口协商。
- 开发态不受影响：Vite dev server 独立 HMR，经代理转发 `/api`，不必每次重编后端。
- **核对的源文件**：`backend/web/embed.go`（`:11` 的 embed 指令；`:22-51` 的 SPA 回退，其中 `:28-31` 把 `/api` 前缀一律 404，免得未知 API 路径被回退成 200 的 `index.html`）；`.github/workflows/ci.yml`（`:48` 前端构建、`:76` 后端构建）；`.github/workflows/release.yml`（`:34`/`:65` 同序）；`.gitignore`（`:67-68` 忽略两处 dist）。
