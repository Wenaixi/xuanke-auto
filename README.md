# 至道选课自动化

Go 后端 + React 前端的单二进制选课服务：账密登录（验证码自动识别）、多账号多备选预选、定时抢课、重启状态恢复。前端产物嵌入二进制，双击单个 exe 即可开箱即用。

## 一、快速开始

```bash
# 1. 构建前端（产物输出到 backend/web/dist）
cd web && npm install && npm run build

# 2. 编译单二进制（Windows 原生内嵌 ddddocr 识别引擎需 CGO=1）
cd backend && CGO_ENABLED=1 go build -o xuanke.exe .

# 3. 运行（自动读取同目录 data/.env；首次运行自动生成）
./xuanke.exe
# 打开 http://localhost:3091
```

**登录方式：**

- **学生**：填教务平台账号密码，登录成功进入选课控制台（选课大厅配置预选目标、调度器窗口开启自动抢报）
- **管理员**：登录页账号填 `admin`、密码填 `data/.env` 中的 `XUANKE_ADMIN_TOKEN`，进入管理后台（激活码 / 系统配置 / 运行状态 / 账号管理 / 日志总览）
- **激活码机制**：默认开启，学生账号首次登录需输入管理员分发的激活码（`XK-XXXX-XXXX-XXXX-XXXX`）；`XUANKE_ACTIVATION=off` 可完全关闭

## 二、配置（data/.env）

首次运行自动生成模板到可执行文件同目录 `data/.env`，真实环境变量优先、文件兜底：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| XUANKE_ADMIN_TOKEN | 随机生成 | 管理口令（必填；缺失拒绝启动） |
| XUANKE_ADMIN_NAME | admin | 管理员登录账号名 |
| SF_API_KEY | 空 | 硅基流动 Vision 密钥（登录验证码识别；ddddocr 引擎免密钥） |
| XUANKE_CAPTCHA_ENGINE | vision | 识别引擎（vision=云识别；ddddocr=本地离线） |
| XUANKE_ACTIVATION | on | 激活码机制开关（off=完全关闭） |
| XUANKE_OPEN_TIME | 2026-09-13 09:00:00 | 选课开放时间 |
| XUANKE_PORT | 3091 | HTTP 端口 |
| XUANKE_DB | data/xuanke.db | SQLite 路径 |
| XUANKE_MASTER_KEY | 自动生成 | 数据加密主密钥（生成于 data/.master_key，需与 db 一起备份） |

## 三、开发

```bash
cd backend && go run .     # 后端 :3091
cd web && npm run dev      # 前端 :5173（/api 代理到 3091）
```

```bash
cd backend && go test -race ./...   # 后端全量测试（含竞态检测）
cd web && npm run build             # 前端类型检查 + 构建
```

详细设计与实现见根目录 `CLAUDE.md`。
