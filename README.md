# 至道选课自动化

Go 后端 + React 前端的单二进制选课服务：账密登录（验证码自动识别）、多账号多备选预选、定时抢课、重启状态恢复。前端产物嵌入二进制，双击单个 exe 即可开箱即用。

## 一、快速开始

想直接用，去 [Releases](https://github.com/Wenaixi/xuanke-auto/releases) 下载对应平台压缩包解压即可：
- `xuanke-windows-amd64.zip`（Windows x64，内置离线验证码识别，免装 Python）
- `xuanke-windows-arm64.zip`（Windows on ARM，如骁龙 X 笔记本）
- `xuanke-linux-amd64.tar.gz`（Linux 服务器，识别走云端视觉或本机 Python）
- `xuanke-darwin-*.tar.gz`（macOS）

解压后双击 `xuanke.exe`（或 `./xuanke`），浏览器打开 http://localhost:3091 即可使用。首次运行自动生成 `data/.env`（配管理口令），账号登录点选"选课大厅"即可配置目标并交给调度器定时抢报。

从源码构建（一般不需要，CI 已全托管）：

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
- **激活码机制**：默认关闭，本地直接用；`XUANKE_ACTIVATION=on` 启用后，学生账号首次登录需输入管理员分发的激活码（`XK-XXXX-XXXX-XXXX-XXXX`）

## 二、配置（data/.env）

首次运行自动生成模板到可执行文件同目录 `data/.env`，真实环境变量优先、文件兜底。模板见仓库根 `.env.example`：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| XUANKE_ADMIN_TOKEN | 随机生成 | 管理口令（必填；缺失拒绝启动） |
| XUANKE_ADMIN_NAME | admin | 管理员登录账号名 |
| XUANKE_CAPTCHA_ENGINE | ddddocr | 识别引擎（ddddocr=本地离线免密钥；vision=云识别需填 SF_API_KEY） |
| SF_API_KEY | 空 | OpenAI 兼容视觉 API 密钥（仅 vision 引擎需要） |
| XUANKE_ACTIVATION | off | 激活码机制开关（on=启用激活码；默认 off 登录直接进入系统） |
| XUANKE_PORT | 3091 | HTTP 端口 |
| XUANKE_DB | data/xuanke.db | SQLite 路径 |
| XUANKE_MASTER_KEY | 自动生成 | 数据加密主密钥（生成于 data/.master_key，需与 db 一起备份） |
| XUANKE_TRUSTED_PROXY | off | 可信反代下信 X-Forwarded-For 做 IP 限流（默认关，防伪造） |

> 开放时间不在此列：由平台 `beginTimes` 自动识别，不可配置（配置层不注入默认值）。

## 三、开发

```bash
# 前后端分离开发（热重载）
cd backend && go run .     # 后端 :3091
cd web && npm run dev      # 前端 :5173（/api 代理到 3091）
```

> **验证已在 CI 全托管**：push 到 master/main 或 PR 触发 `.github/workflows/ci.yml` 自动跑后端全量测试 + 前端构建 + 全平台编译（Windows x64/arm64、Linux、macOS）。本地**无需安装 Go/C 编译器**，也不要手动 `go build`/`go test`，以 CI 结果为准。
>
> 仅当你确实要本地构建/联调时：
> 1. 先跑 `bash backend/scripts/fetch-onnxruntime.sh windows amd64` 下载内嵌识别引擎库（已 gitignore；幂等）
> 2. 再 `cd backend && CGO_ENABLED=1 go build ./...`（Windows 需 MinGW gcc；否则直接依赖 CI）

## 四、发布

打 tag（`v*`）自动触发 `.github/workflows/release.yml` 出包并建 Release：

```bash
git tag v0.1.0 && git push origin v0.1.0
```

产物覆盖 **Windows x64 / Windows ARM64**（各含 GUI 与控制台两个 exe）、**Linux x64**、**macOS 双架构**，均附 `.env.example` 与 `checksums.txt`。Windows 原生内嵌 ddddocr 需 CGO=1；Linux/macOS 走 CGO=0 纯 Go。识别引擎库（onnxruntime）不入 git，CI 构建时下载 + 缓存（微软官方 v1.25.0，与 go.sum 对齐）。

## 五、测试与质量门

所有验证由 GitHub Actions 完成（`ci.yml`，push/PR 触发），本地无需装 Go/C 工具链：

- **后端**：`go test -race ./...` 全包。改 tick 守卫 / 探测时序前先跑 `TestWindowOpenSubmitsWithoutProbeReset` 与 `TestAdminStatsWindowOpenedUsesScheduler`。
- **前端**：`npm run build`（tsc -b + Vite）+ `npm run guard`（五个防回归守卫）。项目根 `tsc --noEmit` 是 references 空壳，不报错。
- **跨平台**：Windows x64/ARM64（CGO=1 内嵌 ddddocr）编译验证 + Linux/macOS 交叉编译验证。

## 六、安全与数据

- 敏感配置只进 `data/.env`（已被忽略），代码内无硬编码密钥；日志只打 token/密码前 8 位。
- 凭据 AES-256-GCM 加密落库；`data/` 整目录（db + .env + .master_key）随部署一起备份迁移。
- 提交前对照 `SECURITY.md` 红线清单与 `CONTRIBUTING.md` 校验要求（涉及平台契约的改动须说明与真实站点行为的对应关系）。
