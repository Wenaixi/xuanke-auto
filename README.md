# 至道选课自动化

知道教育平台（<https://www.zhidao.fj.cn>）的自动化选课助手。Go 后端把 React 前端经 `//go:embed` 编译进同一个可执行文件，启动后在本机 `3091` 端口提供完整界面：不需要单独部署前端、不需要外部数据库，Windows / Linux 版还内置离线验证码识别，运行期不依赖除教务平台以外的外部服务。

它能做这些事：

- 教务账号密码登录（验证码自动识别，识别引擎可换云端视觉模型）
- 多账号并行：每个账号独立教务会话，目标与运行状态按账号隔离
- 在选课大厅预选备选课程，调度器在平台开放时刻自动提交
- 手动报名与退选（二次确认 + 在飞幂等，不会重复发包）
- 管理员后台：激活码分发、运行配置热改、账号管理、调度日志

相关文档：[CONTRIBUTING.md](./CONTRIBUTING.md)（提交与代码要求）、[SECURITY.md](./SECURITY.md)（安全红线）、[CHANGELOG.md](./CHANGELOG.md)（版本变更）、[THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md)（第三方依赖、模型与字体许可）、[CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)、[LICENSE](./LICENSE)（MIT）。

## 一、平台要求

| 平台 | 发布产物 | 内置离线识别 | 说明 |
| --- | --- | --- | --- |
| Windows x64 / ARM64 | `xuanke-<tag>-windows-*.zip` | 是 | 含 GUI（`-H windowsgui`）与控制台两个 exe；常驻系统托盘 |
| Linux x64 / ARM64 | `xuanke-<tag>-linux-*.tar.gz` | 是 | 以 `-tags notray` 构建，无托盘；进程随启动它的终端存活 |
| macOS Apple Silicon / Intel | `xuanke-<tag>-darwin-*.tar.gz` | 否 | CGO=0 纯 Go 构建；识别需切 `vision` 引擎并填 `SF_API_KEY` |
| Android 8.0+（arm64-v8a） | `xuanke-android-arm64.apk` | 是 | 侧载安装；Go 引擎以 c-shared 形式内置，WebView 加载本机 `127.0.0.1:3091` |

Linux 与 macOS 产自同一条发布流水线（`release.yml` 的 `build-unix` job），但构建方式不同：**Linux 是 CGO=1 并内嵌 ddddocr，macOS 双架构都是 CGO=0、不含该引擎**（macOS 交叉编译 ONNX Runtime 不稳定，识别走云端兜底）。

预编译产物不需要任何工具链。只有从源码构建才需要：

- **Go**：`backend/go.mod` 声明 `go 1.26.8`；CI 用 `stable`。编 Android 引擎需 **Go 1.26.x**——Go 1.27 的 c-shared 有已知回归，会静默产出 UNDEF 符号、装机 dlopen 才炸（`release.yml` 里已钉版本）。
- **Node.js**：CI 用 22（`ci.yml`），前端 `web/package.json` 要求 Vite 8 / TypeScript 6 / React 19。
- **C 工具链**：Windows 下 CGO=1 需要 MinGW-w64 gcc；Linux / Android 的交叉编译在 CI 里分别用 zig cc 与 NDK clang（NDK `26.3.11579264`）。

## 二、快速开始

### 方式 A：用发布产物

从 [Releases](https://github.com/Wenaixi/xuanke-auto/releases) 下载对应平台的压缩包，`<tag>` 即版本标签（当前最新为 `v0.3.0`）：

- `xuanke-<tag>-windows-amd64.zip` / `xuanke-<tag>-windows-amd64-console.zip`（Windows x64，GUI 版 / 控制台版）
- `xuanke-<tag>-windows-arm64.zip` / `xuanke-<tag>-windows-arm64-console.zip`（Windows on ARM，如骁龙 X 笔记本）
- `xuanke-<tag>-linux-amd64.tar.gz` / `xuanke-<tag>-linux-arm64.tar.gz`
- `xuanke-<tag>-darwin-arm64.tar.gz` / `xuanke-<tag>-darwin-amd64.tar.gz`（Apple Silicon / Intel）
- `xuanke-android-arm64.apk`（APK 文件名不带 tag）
- `checksums.txt`（以上产物的 SHA-256）

每个压缩包（APK 除外）内含可执行文件与一份 `.env.example` 模板。

Windows：解压后双击 `xuanke.exe`，或在 PowerShell 里：

```powershell
.\xuanke.exe
```

Linux / macOS：

```bash
tar -xzf xuanke-<tag>-linux-amd64.tar.gz
chmod +x xuanke
./xuanke
```

Android：把 `xuanke-android-arm64.apk` 传到手机侧载安装。首次启动会请求通知权限——前台服务靠通知常驻，用于在后台继续抢课；返回键只是把 App 收进后台，引擎不受影响。

首次运行会在可执行文件同目录生成 `data/.env`（随机管理口令 + 默认配置），随后：

- 服务监听 `http://127.0.0.1:3091`（改 `XUANKE_LISTEN_HOST` 可开局域网/公网）
- 默认浏览器会自动打开该地址；Windows 版常驻系统托盘，右键菜单为「打开浏览器 / 关于（版本与数据库路径）/ 退出」
- 关掉浏览器标签不会停止服务；退出要走托盘菜单（Windows）或结束进程（Linux / macOS 无托盘）

### 方式 B：从源码构建

```bash
git clone https://github.com/Wenaixi/xuanke-auto.git
cd xuanke-auto

# 1. 构建前端：vite.config.ts 已把产物直接输出到 backend/web/dist
cd web && npm ci && npm run build

# 2. 下载内嵌识别引擎用的 ONNX Runtime 动态库（不入 git，已存在则跳过）
cd ../backend && bash scripts/fetch-onnxruntime.sh windows amd64

# 3. 编译单二进制
CGO_ENABLED=1 go build -ldflags="-s -w -H windowsgui" -o xuanke.exe .
./xuanke.exe
```

PowerShell 下环境变量必须写成 `$env:CGO_ENABLED = '1'`：直接写 `CGO_ENABLED=1` 会被 PowerShell 当成命令名报 CommandNotFoundException（CI 里就踩过这个坑，会导致构建静默漏执行），CGO=1 还需要 MinGW-w64 gcc：

```powershell
cd web; npm ci; npm run build
cd ..\backend
$env:CGO_ENABLED = '1'
go build -ldflags="-s -w -H windowsgui" -o xuanke.exe .
.\xuanke.exe
```

不需要内置识别引擎时（例如 macOS，或只想跑通链路）用纯 Go 构建，此时第 2 步可以跳过，运行期改用 `vision` 引擎：

```bash
cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o xuanke .
```

`backend/web/dist` 是通过 `//go:embed all:dist` 在**编译期**打进二进制的。改了前端代码必须重新 `npm run build` 再 `go build`，否则新 exe 里仍是旧资源；开发期请走下面的 Vite 开发服务器。

## 三、配置（data/.env）

优先级是 **真实环境变量 > `data/.env` 文件**。文件不存在时首次运行自动生成（随机管理员口令 + 默认项）；默认位置是可执行文件同目录的 `data/.env`，源码 `go run` 时是当前工作目录下的 `data/`。

| 键 | 默认值 | 作用 |
| --- | --- | --- |
| `XUANKE_ADMIN_TOKEN` | 首次运行随机生成（24 位十六进制） | **必填**，管理员登录口令；为空时拒绝启动 |
| `XUANKE_ADMIN_NAME` | `admin` | 管理员登录账号名，登录页账号填它才走管理入口 |
| `XUANKE_ACTIVATION` | 关（仅 `on` 开启） | 激活码机制开关；开启后学生首次登录返回 `code=1001`，需管理员分发的激活码 |
| `XUANKE_CAPTCHA_ENGINE` | `ddddocr` | 识别引擎：`ddddocr`（本地离线免密钥）/ `vision`（OpenAI 兼容视觉 API） |
| `SF_API_KEY` | 空 | 视觉识别密钥；`vision` 引擎必需，留空则该引擎不可用 |
| `SF_BASE_URL` | `https://api.siliconflow.cn/v1` | 视觉识别服务地址，可改任意 OpenAI 兼容服务 |
| `SF_MODEL` | `Qwen/Qwen3-VL-30B-A3B-Instruct` | 视觉识别模型名 |
| `XUANKE_PORT` | `3091` | HTTP 端口 |
| `XUANKE_LISTEN_HOST` | `127.0.0.1` | 监听地址：仅本机 / 内网 IP 开局域网 / 域名走穿透 / `0.0.0.0` 所有网卡 |
| `XUANKE_DB` | `data/xuanke.db` | SQLite 路径（相对可执行文件目录） |
| `XUANKE_MASTER_KEY` | 自动生成到 `data/.master_key` | 64 位十六进制（32 字节）数据加密主密钥 |
| `XUANKE_TRUSTED_PROXY` | 关（仅 `on` 开启） | 可信反代后采信 `X-Forwarded-For` 做 IP 限流；仅在 RemoteAddr 为回环时才生效 |

两点约定：

- **开放时间不在配置里**。唯一事实源是平台 `beginTimes` 的自动识别，没有对应环境变量，也没有可以手工填的开关。
- **管理后台改过的配置会落库并覆盖 env 初值**。`activation_enabled`、`vision_base_url`、`vision_key`、`vision_model`、`captcha_engine`、`captcha_fallback`、`captcha_concurrency`、`listen_host`、`listen_port` 这九个键由 `runtime` 包的一张配置表统一维护，启动顺序是「env 初值 → 落库值覆盖 → 按最终值监听」。同名项在后台改过之后，光改 `.env` 不会立刻生效。

## 四、使用流程

1. **登录**。打开 `http://127.0.0.1:3091`，填教务学号/账号与密码（验证码由后端自动识别后提交）。启用激活码机制时，未激活账号会弹出激活码输入框，激活一次后永久免激活。多账号可分别登录，会话与目标互不干扰。管理员用 `XUANKE_ADMIN_NAME`（默认 `admin`）+ `XUANKE_ADMIN_TOKEN` 登录，直接进管理后台。
2. **选目标**。进「选择课程」（选课大厅）：按发布/学期浏览课程，勾选备选课程并保存为预选目标（每账号最多 100 门，多选时按优先级排序）；同一页也能对单门课手动报名或退选。
3. **等待抢课**。回到「选课助手」看窗口状态与倒计时。调度器自己识别平台开放时间，到点按目标顺序提交；课程满员、窗口关闭或平台风控都会记入结果并停止对该课程的无效重复提交；教务会话失效会自动重登，无需人工干预。
4. **看结果**。「选课助手」页的「预选目标课程」按开放时间分组显示每门课的状态与报名进度，「调度日志」是实时事件流（同样能在管理后台的「日志总览」里翻）。

「选课助手」页由四块组成：选课时间窗口（没开放时是倒计时与预计开放时间，开放后换成报名进度）、账号状态、预选目标课程、调度日志。

> 注意区分「未开放」和「已关闭」：开放前平台照样返回完整课程列表（只是按钮禁用），关闭后才会返回空列表。

## 五、管理员后台

登录页账号填 `XUANKE_ADMIN_NAME`（默认 `admin`）、密码填 `XUANKE_ADMIN_TOKEN`，即进入管理后台。管理接口全部挂在 `/api/admin/*` 下，逐个走管理员会话校验。五个标签页：

| 标签 | 内容 |
| --- | --- |
| 激活码 | 生成（可指定每码可用次数）、查看、删除；机制关闭时给出明确提示而不是伪装成加载失败 |
| 系统配置 | 激活码开关、识别引擎与识别并发、视觉 API 地址/密钥/模型、监听地址与端口、引擎兜底开关；保存立即生效并落库 |
| 运行状态 | 窗口状态、激活码机制、账号与课程计数、实际生效的识别引擎等诊断字段 |
| 账号管理 | 已登录账号列表、跳转到某个账号的选课大厅、删除账号（连同其会话与调度状态） |
| 日志总览 | 全量调度/审计日志（默认取最近 500 条，可用 `?limit=` 调整） |

Android 内置形态有两处差异：端口锁定 `3091`（Java 壳按该端口加载页面，后台不允许改）且后台不展示账密。

## 六、开发

前后端分开跑，各自热重载：

```bash
# 终端 1：后端（监听 3091，数据落当前工作目录的 data/）
cd backend && go run .

# 终端 2：前端（Vite 开发服务器；vite.config.ts 把 /api 代理到 localhost:3091）
cd web && npm run dev
```

开发服务器端口没有在 `web/vite.config.ts` 里固定，以终端打印的地址为准。前端栈为 React 19 + TypeScript + Vite + Tailwind CSS 4 + Radix Primitives + TanStack Query；后端只用标准库 `net/http`（Go 1.22+ 原生路由）与 `modernc.org/sqlite`（纯 Go，免 CGO），只有内嵌 ddddocr 识别引擎这一处才需要 CGO。

`backend/cmd/` 下有三个排障用小工具，用 `go run ./cmd/<name>` 运行：`bench`、`probe`、`logintest`。

平台接口契约的对照基线（站点前端源码与抓包记录）属于本地专有资料，不随仓库分发——本仓库不包含这些文件，改动平台相关代码时以现有实现与测试为准。

## 七、测试与质量门

后端：

```bash
cd backend
go test ./...          # 全量
go test -race ./...    # 推荐口径（CONTRIBUTING.md 同款）
```

CI 里实际执行的是 `go test -p 1 -count=1 -v ./...`，失败会自动重跑一次以吸收回环连接抖动。改窗口判定或 tick 时序前，先跑 `TestWindowOpenSubmitsWithoutProbeReset` 与 `TestAdminStatsWindowOpenedUsesScheduler` 对照新旧行为。

前端：

```bash
cd web
npm run build   # tsc -b + vite build（唯一的真实类型检查入口）
npm run guard   # 六套源码形状守卫：倒计时 / 目标保存 / 管理态 / 401 / 懒加载 / 字体
npm test        # vitest 纯函数测试（node 环境）
npm run lint    # oxlint
```

项目根的 `tsc --noEmit` 是 references 空壳、不会报错，只有 `tsc -b` 真校验类型。

`.github/workflows/ci.yml` 在 push 到 `master`/`main` 或开 PR 时触发（纯 `.md` 改动被 `paths-ignore` 跳过），三个 job：

| job | Runner | 做什么 |
| --- | --- | --- |
| `ci-pipeline` | ubuntu-latest | 前端 `npm ci && npm run build && npm run guard && npm test`；后端全量测试；`CGO_ENABLED=0` 编单二进制（验证 embed）；`GOOS=linux` / `GOOS=darwin` 交叉编译 |
| `ci-windows` | windows-latest | 下载 onnxruntime dll，用 `CGO_ENABLED=1` 编 Windows GUI exe（验证内嵌 ddddocr 可编译） |
| `ci-windows-arm64` | windows-11-arm | 装 llvm-mingw，用 `CGO_ENABLED=1` 编 Windows ARM64 exe |

## 八、发布

推 `v*` 格式的 tag（或手动触发 `workflow_dispatch`）即启动 `.github/workflows/release.yml`：四个构建 job 并行，最后一个 job 汇总产物、生成 `checksums.txt` 并创建 GitHub Release。

```bash
git tag v0.2.28        # 版本号与 Release 标签一致；当前最新为 v0.3.0
git push origin v0.2.28
```

| job | 产物 |
| --- | --- |
| `build-windows` | GUI（`-H windowsgui`）+ 控制台两个 exe → `xuanke-<tag>-windows-amd64{,-console}.zip` |
| `build-windows-arm64` | 同上两份，ARM64 |
| `build-unix` | Linux amd64 / arm64（CGO=1，`-tags notray`）与 macOS arm64 / amd64（CGO=0）四个 tar.gz |
| `build-android` | `xuanke-android-arm64.apk`：NDK 编 c-shared，Gradle 8.7 打包 |

几个关键事实：

- **版本注入有校验**。APK 的 `versionName` / `versionCode` 由 tag 改写 `build/android/app/build.gradle`，构建后用 `aapt dump badging` 与 `apksigner verify --print-certs` 双重断言；版本号或签名指纹对不上就直接失败，不留到装机才发现。
- **ONNX Runtime 固定微软官方 v1.25.0**（与 `go.sum` 的 `onnxruntime_go v1.25.0` 对齐），不随仓库分发，构建时由 `backend/scripts/fetch-onnxruntime.sh` 下载并缓存；脚本内含 ABI 目录定位与架构自检，取错架构会在构建期报错。
- **归档里的 `.env.example` 是生成出来的**：Windows 模板默认 `ddddocr`，Linux / macOS 模板默认 `vision` 并提示填 `SF_API_KEY`。

## 九、安全与数据

- **凭据加密**：教务密码与视觉 API 密钥都以 `enc:` 前缀经 AES-256-GCM 入库。主密钥来自 `XUANKE_MASTER_KEY`，未设置时自动生成到 `data/.master_key`。读到旧版明文、主密钥损坏或未注入加密器时一律拒绝启动，不降级放行。
- **日志脱敏**：教务 token 只输出前 8 位，密码从不写日志；管理后台回显视觉密钥时只给掩码与末 4 位。
- **接口防护**：登录与激活各自按 IP 限流（令牌桶，5 次/分、突发 5，超限返回 HTTP 429「登录尝试过于频繁，请稍后再试」）；副作用请求强制 `Content-Type: application/json`（拒绝跨站表单 POST，返回 HTTP 403）；未注册的 `/api/*` 一律 404，不会回退成首页 HTML。
- **默认只听本机**：`XUANKE_LISTEN_HOST` 默认 `127.0.0.1`。要暴露到局域网或公网请自行加反代与 TLS；只有在可信反代之后显式设 `XUANKE_TRUSTED_PROXY=on`、且请求确实来自回环时，才会采信 `X-Forwarded-For`。
- **数据目录**：`data/` 里的 SQLite 库、`.env`、`.master_key` 必须一起备份迁移；换掉主密钥会让已加密的凭据不可读。
- **已知风险（Android）**：APK 内置固定管理账密并固定监听 `127.0.0.1:3091`，同设备上的其它应用也能连上该端口用内置口令登录管理页，因此 APK 只适合自用或小范围侧载分发。

## 十、界面字体

界面字体随包分发，均为 **SIL OFL 1.1**，来源与版权见 [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md)：

| 用途 | 字体 | 说明 |
| --- | --- | --- |
| 正文与全部界面文字 | Geist Variable | 可变无衬线，与 Geist Mono 同源；按 `unicode-range` 分片按需加载 |
| 左上角标题 | jf open 粉圓 的派生子集 | 12,556 字节，只含三个繁体标题（選課助手 / 系統管理 / 選擇課程）的用字与 ASCII；粉圓是繁体优先字体，改标题文案必须重打子集 |
| 等宽数字（倒计时、课程 ID、统计表格） | Geist Mono | Geist 数字带 `tnum`，用它会让倒计时严格对齐、表格列不跳动 |

三套字体合计给产物增加约 **0.16 MiB**（Geist + Geist Mono 共 146,868 字节，粉圓子集 12,556 字节）。粉圓子集的许可证全文随 `web/public/fonts/jf-openhuninn-OFL.txt` 一起进 dist（进而进 exe / APK）——OFL 要求随包保留许可证，不要删。

**Geist 不含汉字**，中文由系统中文字体按 `PingFang SC`（macOS / iOS）→ `Microsoft YaHei UI`（Windows）→ `Noto Sans CJK SC`（Linux）顺位兜底。因此有两个已知代价：各平台中文观感略有差异；Windows 的微软雅黑只有 Light / Regular / Bold 三档，`font-medium`(500) 与 `font-semibold`(600) 会静默落为 400 / 700，字重层次比自托管可变字体弱。

字体只允许在 `web/src/styles/global.css` 的 `@theme` 里定义：`--font-sans` / `--font-mono` / `--font-title` 三个令牌，组件层一律用 `font-sans` / `font-mono` / `font-title` 工具类；`npm run guard` 里的 `web/scripts/font-guard.ts` 会拦下组件层手写的 `font-family`。

## 十一、目录结构

```
backend/                    Go 后端（module xuanke-auto/backend）
  main.go                   桌面入口：runServer + 托盘
  server.go                 服务装配：DB / 加密 / 调度器 / 会话 / 路由 / SPA
  platform_android.go       Android JNI 入口（//go:build android）
  jni_android.c             JNI_OnLoad 与 logcat 日志桥
  tray_windows.go           Windows 托盘；tray_linux*.go / tray_other.go 为其它形态
  browser_windows.go        用默认浏览器打开页面
  internal/api/             HTTP 路由与处理器（/api/*、/api/admin/*）
  internal/scheduler/       开窗识别、探测与提交调度
  internal/upstream/          平台客户端：登录、课程、报名退选、识别引擎
  internal/accounts/        多账号注册表与凭据生命周期
  internal/{config,runtime,secure,session,store,db}/
  web/                      embed 目标（//go:embed all:dist）
  scripts/fetch-onnxruntime.sh
  cmd/{bench,probe,logintest}/   排障小工具
web/                        React 前端（Vite 构建，产物落 backend/web/dist）
  src/routes/               Login / Dashboard / Select / Admin
  src/components/           含 ui/ 基础组件与 Footer
  src/lib/                  纯函数与守卫逻辑（含 vitest 测试）
  src/styles/global.css     设计令牌（颜色、排版、字体）
  scripts/*-guard.ts        npm run guard 的六套源码形状守卫
  public/                   图标、粉圓子集与其 OFL 许可证
build/android/              Android 薄壳（Gradle + Java + 资源）
.github/workflows/          ci.yml / release.yml
docs/agents/                Agent 工作流配置（domain / issue-tracker / triage-labels）
```

## 十二、常见问题

**启动即退出，日志说「服务启动失败（监听 …）」**：端口被占用（常见于已经开着一个实例）。改 `XUANKE_PORT`，或进管理后台「系统配置」改监听地址与端口——后台改是热重载，不用重启进程（Android 形态端口固定 `3091`）。

**启动即退出，日志说「未设置管理口令」**：`data/.env` 里的 `XUANKE_ADMIN_TOKEN` 为空，或该行被删掉了。填上口令（或设同名环境变量）后重启。注意只有 `data/.env` 整体不存在时，首次运行才会自动生成随机口令；文件在但口令空着就是拒绝启动。

**提示识别不可用 / 教务登录一直失败**：默认识别引擎是 `ddddocr`，本机没有可用引擎时不会静默转云端计费。去管理后台「系统配置」切到 `vision` 并填 `SF_API_KEY`，或者打开引擎兜底开关。macOS 产物本身不含内置引擎，首次使用就要配 `vision`。

**提示「登录尝试过于频繁，请稍后再试」**：登录与激活限流按出口 IP 记账（5 次/分）。同一出口下多人使用或反复重试都会撞上限流，等一分钟或换出口即可。

**前端改了但页面没变**：`backend/web/dist` 是编译期 embed 进二进制的。按 `npm run build` → `go build` → 重启的顺序做，只重启不重编一定是旧资源；开发期直接开 Vite 开发服务器最省事。

**Android 安装报 `INSTALL_FAILED_UPDATE_INCOMPATIBLE`**：APK 签名是固定的，历史上曾因每次发布现场生成新密钥导致新旧包签名不一致。已经装过旧签名包的话，只能先卸载（会清掉 App 数据）再装一次；此后的升级不会再遇到。

**Android 上没反应 / 通知反复弹**：先确认给了通知权限（Android 13+ 的前台服务保活要用），再用 `adb logcat -s XuanKe` 看引擎日志——c-shared 形态下 Go 的 stderr 不会进 logcat，必须走这条日志桥。

**抢课结束后，日志里的状态怎么读**：`已满员` 表示快照人数已满，或窗口关闭后平台拒绝报名（这种也算满员，避免每轮都白刷接口）；含「报名时间 / 未开启 / 已结束」字样的失败是平台还没开放；风控类错误会退避 30 秒再试；教务会话失效会自动重登，不需要手工干预。

**换机器怎么迁移**：把整个 `data/` 目录（SQLite 库 + `.env` + `.master_key`）一起拷过去。三者缺一，轻则凭据读不出来，重则服务直接拒绝启动。
