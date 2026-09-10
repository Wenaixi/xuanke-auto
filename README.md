# 至道选课自动化（Go + React）

将 Python 版选课脚本重构为 Go 后端 + React 前端的单二进制服务：账密登录（验证码自动识别）、三课程选择、定时抢课、重启状态恢复。

## 架构

- 后端：Go 标准库 net/http + modernc.org/sqlite（纯 Go，免 CGO），零框架依赖
- 前端：React 18 + Vite + TypeScript + Radix Primitives（黑白高级、无圆角）
- 单二进制：前端构建产物通过 go:embed 嵌入，一个端口服务 API 与页面
- 抢课：Scheduler 每 300ms 轮询课程数据，窗口开启瞬间并发提交三个目标课程

## 构建

```bash
cd web && npm install && npm run build   # 前端产物输出到 backend/web/dist
cd backend && go build -o xuanke.exe .    # 编译单二进制（含前端）
```

## 运行

```bash
./xuanke.exe
# 打开 http://localhost:8080
```

### 复用已有登录会话（无需重新登录）

```bash
XUANKE_TOKEN=<idToken> XUANKE_COOKIE="menu_sidebar_scroll_top=..; access_limit_cookie=..; zd_edu_cookie=.." ./xuanke.exe
```

不设置环境变量时，服务从 SQLite 恢复上次登录的账密/token（自动重登需在前端登录页手动操作）。

## 功能

- 账密登录：完整逆向登录链路（GET /login -> 验证码 -> RSA 加密 -> doLogin），验证码走硅基流动 Vision 识别
- 三课程选择：体育 / 校本1 / 校本2 各选 1 门，展示课程名、老师、地点、课节、报名时间、人数与可报名状态
- 定时抢课：窗口开启（2026-09-13 09:00:00）后并发报名，成功后持久化不重复提交
- 状态面板：窗口倒计时、三课程状态（等待/提交中/已报名/失败）、报名日志
- 重启恢复：账密/token/目标/已成功课程全部持久化，重启自动续跑
- 安全：登录限流（每 IP 每分钟 5 次）、panic recover、请求体限制、安全响应头、API 不回传密码

## 目录结构

```
backend/
  main.go                 # 入口：组装 + 重启恢复 + 前端嵌入
  web/embed.go            # go:embed 前端产物
  internal/config/        # 配置（端口/数据库/开放时间/视觉配置）
  internal/db/            # SQLite 打开与建表
  internal/zhidao/        # 至道平台 API 客户端（登录/查询/报名/详情/验证码）
  internal/scheduler/     # 抢课调度器（300ms 轮询 + 并发提交）
  internal/store/         # SQLite 持久化（账密/token/目标/日志/状态）
  internal/api/           # REST API + 安全中间件
  cmd/probe/              # 接口探测工具（验证至道平台连通性）
web/
  src/routes/             # 登录页 / 状态面板 / 课程选择
  src/components/         # Button / Input（Radix Slot 包装）
  src/api/client.ts       # 统一请求封装（未登录事件广播）
```

## 配置（环境变量）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| XUANKE_PORT | 8080 | HTTP 端口 |
| XUANKE_DB | data/xuanke.db | SQLite 路径 |
| SF_API_KEY | 空 | 硅基流动 API Key（验证码识别必需） |
| XUANKE_TOKEN | 空 | 复用已有 idToken（跳过登录） |
| XUANKE_COOKIE | 空 | 复用已有 Cookie（k=v; k2=v2） |

## 开发

```bash
cd backend && go run .        # 后端 :8080
cd web && npm run dev          # 前端 :5173（/api 代理到 8080）
```

## 测试

```bash
cd backend && go test ./... && go test -race ./internal/scheduler/
```
