# 至道选课自动化

一个选课助手：Go 后端 + React 前端，前端产物嵌入二进制，装好就是一个 exe。

功能包括账密登录（验证码自动识别）、多账号多备选预选、按平台开放时间定时抢课、重启后状态恢复。

## 一、快速开始

要用现成的，去 [Releases](https://github.com/Wenaixi/xuanke-auto/releases) 下载对应平台的压缩包解压：

- `xuanke-windows-amd64.zip`（Windows x64，内置离线验证码识别）
- `xuanke-windows-arm64.zip`（Windows on ARM，如骁龙 X 笔记本）
- `xuanke-linux-amd64.tar.gz`（Linux x64，内置离线验证码识别）
- `xuanke-linux-arm64.tar.gz`（Linux ARM）
- `xuanke-darwin-arm64.tar.gz`（macOS Apple Silicon）
- `xuanke-darwin-amd64.tar.gz`（macOS Intel）
- `xuanke-android-arm64.apk`（Android ARM64，Go 引擎内嵌 APK，WebView 直连本机引擎，侧载安装）

解压后双击 `xuanke.exe`（或 `./xuanke`），浏览器打开 http://localhost:3091。首次运行会生成 `data/.env`，里面是管理口令。登录后进选课大厅配置目标，调度器负责到点抢报。

程序启动后常驻系统托盘，右键菜单有三个入口：打开浏览器、关于（显示版本与数据库路径）、退出。关闭浏览器标签不会停止程序，退出要从托盘菜单走。

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

- **学生**：填教务平台账号密码，进入选课控制台。选课大厅里配置预选目标，开放时间一到调度器自动抢报。
- **管理员**：登录页账号填 `admin`，密码填 `data/.env` 里的 `XUANKE_ADMIN_TOKEN`，进入管理后台（激活码 / 系统配置 / 运行状态 / 账号管理 / 日志总览）。
- **激活码机制**：默认关闭，本地直接用。设 `XUANKE_ACTIVATION=on` 启用后，学生账号首次登录要输入管理员分发的激活码（`XK-XXXX-XXXX-XXXX-XXXX`）。

## 二、配置（data/.env）

首次运行自动生成模板到可执行文件同目录 `data/.env`。真实环境变量优先，文件兜底。模板见仓库根 `.env.example`：

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

开放时间不在配置里：由平台 `beginTimes` 自动识别，不可配置。

## 三、开发

```bash
# 前后端分离开发（热重载）
cd backend && go run .     # 后端 :3091
cd web && npm run dev      # 前端 :5173（/api 代理到 3091）
```

验证由 CI 全托管：push 到 master/main 或开 PR 会触发 `.github/workflows/ci.yml`，自动跑后端全量测试 + 前端构建 + 全平台编译（Windows x64/arm64、Linux、macOS）。日常开发不必在本地装 Go/C 编译器，也不必手动 `go build`/`go test`，以 CI 结果为准。

确实要本地构建或联调时：

1. 先跑 `bash backend/scripts/fetch-onnxruntime.sh windows amd64` 下载内嵌识别引擎库（已 gitignore，可重复执行）
2. 再 `cd backend && CGO_ENABLED=1 go build ./...`（Windows 需要 MinGW gcc）

## 四、发布

打 `v*` 格式的 tag 会自动触发 `.github/workflows/release.yml` 出包并建 Release：

```bash
git tag v0.2.16 && git push origin v0.2.16
```

产物覆盖 Windows x64 与 ARM64（各含 GUI 与控制台两个 exe）、Linux x64/ARM64、macOS 双架构、Android ARM64 APK，均附 `.env.example` 与 `checksums.txt`。

除 macOS Intel 外，各平台都内置离线 ddddocr 识别引擎（CGO=1 内嵌模型与 ONNX Runtime），识别过程不依赖外网。macOS Intel 用 CGO=0 构建，不含该引擎。识别引擎库（onnxruntime dll/so/dylib）不入 git，CI 构建时经 `fetch-onnxruntime.sh` 下载并缓存，版本锁定微软官方 v1.25.0（与 go.sum 对齐）。

Android 包是自制薄壳：Go 引擎以 c-shared 编入 `libxuanke.so`，Java 壳用 WebView 加载 127.0.0.1 上的本机引擎，侧载分发。

## 五、测试与质量门

所有验证由 `.github/workflows/ci.yml` 完成（push/PR 触发），本地不必装 Go/C 工具链：

- **后端**：`go test -race ./...` 全包。改 tick 守卫或探测时序前，先跑 `TestWindowOpenSubmitsWithoutProbeReset` 与 `TestAdminStatsWindowOpenedUsesScheduler` 对照新旧行为。
- **前端**：`npm run build`（tsc -b + Vite）、`npm run guard`（六个防回归守卫）、`npm test`（vitest 纯函数测试）。项目根的 `tsc --noEmit` 是 references 空壳，不会报错。
- **跨平台**：Windows x64/ARM64（CGO=1 内嵌 ddddocr）编译验证、Linux/macOS 交叉编译验证、Android APK（NDK c-shared 交叉 + Gradle 打包）验证。

## 六、安全与数据

- 敏感配置只进 `data/.env`（已被忽略），代码里没有硬编码密钥，日志只打 token/密码前 8 位。
- 凭据用 AES-256-GCM 加密落库。`data/` 整个目录（db + .env + .master_key）随部署一起备份迁移。
- 提交前对照 `SECURITY.md` 的红线清单与 `CONTRIBUTING.md` 的校验要求。涉及平台契约的改动，要说明与真实站点行为的对应关系。
