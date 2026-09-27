# 贡献指南

欢迎为至道选课自动化（xuanke-auto）贡献代码。项目不大，期望每一份改动都让仓库更干净、更好维护。

## 前置阅读

- [`README.md`](./README.md)：上手指南与架构概览
- [`SECURITY.md`](./SECURITY.md)：敏感信息红线与威胁模型（改配置、日志、网络代码前必读）
- [`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md)：行为准则
- [`.github/workflows/ci.yml`](./.github/workflows/ci.yml)（质量门）与 [`.github/workflows/release.yml`](./.github/workflows/release.yml)（发版）

## 环境准备

### 需要本地安装的

| 工具 | 要求 | 用途 |
| --- | --- | --- |
| Node.js | 22（CI 固定 `node-version: 22`；更高版本一般也能跑） | `web/` 的构建、守卫、纯函数测试 |
| npm | 随 Node | 一律用 `npm ci`，按 `web/package-lock.json` 锁定依赖版本 |
| Go | 与 `backend/go.mod` 声明的版本兼容 | 后端测试与构建 |
| Git | 近期版本 | 分支、提交、tag |

### 不需要本地安装的（CI 全托管）

- **完全不想装工具链也能贡献**：CI 会跑前端构建 + 守卫 + 测试、后端全量测试，以及各平台的编译检查。
- Linux 托盘依赖（`libayatana-appindicator3-dev`、`libgtk-3-dev`、`libglib2.0-dev`）、各平台 onnxruntime 动态库、Android SDK/NDK/JDK/Gradle 8.7、Windows ARM64 的 llvm-mingw，全部由 CI 现装现下。
- 只有本地调试 CGO=1 发布形态时才需要 C 编译器（gcc / llvm-mingw），跑测试不需要。

### 必须先做的一步：构建前端

`web/dist` 经 `//go:embed all:dist` 进二进制（见 [`backend/web/embed.go`](./backend/web/embed.go)），而 `backend/web/dist/` 被 `.gitignore` 忽略。**新克隆的仓库在 `npm run build` 之前，后端连编译都过不去**（`pattern all:dist: no matching files found`），CI 也是先建前端再跑 Go：

```bash
cd web && npm ci && npm run build   # 产物直接落 backend/web/dist
```

想复现发布形态（CGO=1 内嵌 ddddocr）再补：

```bash
# 各平台 onnxruntime 动态库不入库（16MB+），按 <goos> <goarch> 现下
bash backend/scripts/fetch-onnxruntime.sh windows amd64   # 另有 linux amd64 / linux arm64 / darwin arm64 / android arm64

cd backend && CGO_ENABLED=1 go build -ldflags="-s -w" -o xuanke-dev.exe .
```

`go env CGO_ENABLED` 在本机没有 C 编译器时是 `0`，此时 `native_ocr_win_*.go`（`//go:build windows && amd64 && cgo`）不参与编译，跑的是纯 Go 路径；`CGO_ENABLED=1` 但缺工具链会直接报 `cgo: C compiler "gcc" not found`。这只影响验证发布形态，不影响跑测试。

## 分支与提交规范

- 从 `master` 开功能分支，名字体现改动范围。仓库里实际用过的：`ui/redesign-v2`、`ui/liquid-glass-gooey-redesign`、`feature-android-apk`（PR #2 合并）、`feature-full-platform-native-and-apk`（PR #1 合并）。
- 提交信息格式 `类型(范围): 描述`，类型小写英文，描述用简体中文：

```
fix(web): 账号管理表格操作按钮改小并禁止折行
feat(config): 监听地址可配置并可后台热重载
docs: 更新日志与发版纪律
```

  用过的类型前缀：`feat` / `fix` / `docs` / `chore` / `build` / `ci` / `refactor`；范围如 `web` / `ui` / `config` / `android` / `typography` / `brand` / `changelog`。
- 一个小模块或一个小修复一次 commit，不写 emoji。
- **代码注释只写「为什么 / 契约 / 陷阱」**，不带「第 N 轮」「B18-XX」这类轮次前缀；决策历史由维护者本地归档，不进仓库。

## 本地质量门

改完先本地跑一遍，顺序与 CI 一致：

```bash
# 前端：构建（tsc -b + vite）→ 源码形状守卫 → 纯函数测试
cd web
npm ci
npm run build     # 产物落 backend/web/dist
npm run guard     # 六套形状守卫：性能倒计时 / 目标保存 / 管理态 / 401 / 懒加载 / 字体
npm test          # vitest（当前 2 个文件 38 个用例）
```

```bash
# 后端：全量测试
cd backend
go test ./...
```

- **`-race` 不是默认门**：CI 跑的是 `go test -p 1 -count=1 -v ./...`（`-p 1` 串行跑包，失败自动重跑一次吸收回环 flake），**不带 `-race`**。改调度器并发状态机、探测时序、账号生命周期时本地自补：

  ```bash
  cd backend && go test -race ./internal/scheduler/
  ```

  `-race` 需要 C 工具链；没有时是 `cgo: C compiler "gcc" not found`（build failed）而不是测试失败——把它放到有工具链的机器上跑。
- **回归陷阱**：改 tick 守卫或探测时序前，先跑 `go test -run 'TestWindowOpenSubmitsWithoutProbeReset|TestAdminStatsWindowOpenedUsesScheduler'` 对照新旧行为。
- 前端回归以 `npm run build` 为准。项目根的 `tsc --noEmit` 是 references 空壳，不会报错；真校验在 `tsc -b` 里。
- **embed 是编译期固化**：验证顺序必须 `npm run build` → `go build` → 启动服务，否则服务端发的是旧资源 hash，浏览器看不到新 UI。

## 写测试

- 新功能或缺陷修复走 TDD：先写失败测试（红灯），再最小实现（绿灯），然后重构。
- 测试要断言真实行为，不能把实现抄一遍当断言，否则恒假绿，测不出契约。
- **后端**：测试放在 `backend/internal/<pkg>/` 对应包里。
- **前端**：目前只有两套测试手段。一是纯函数测试（vitest，`npm test`，文件为 `src/**/*.test.ts`），二是源码形状守卫（`npm run guard`，六个 jiti 脚本断言字体/性能/目标保存/管理态/401/懒加载这些契约没有被改回去）。这里没有 DOM 环境，也没有 `.tsx` 组件测试，组件层行为要靠守卫脚本或人工验证来兜。
- 测试要能跨平台跑（CI 覆盖 Linux/Windows/macOS）：不依赖 Windows 保留路径，不硬编码平台特有的错误文案（如 `connectex`），不假设 CI 环境装有 Python。

## 编码规范

- **语言**：注释与提交信息用简体中文，代码标识符用英文，文档不用 emoji。
- **简洁优先**：不做过度设计，不为一次性代码建抽象，能用标准库就不用第三方库；能通过 `go test -race` 就优先于性能优化。
- **精准修改**：只碰必须碰的地方，不顺手改进无关代码。发现死代码在 PR 里指出，不要顺手删掉。
- **安全**：不把口令/token/密钥提交进仓库（见 `SECURITY.md`）。日志里 token 只打印前 8 位（`maskedToken` / `tokenShort`），**密码一律不进日志**；网络层错误上抛前必须剥掉带 `?idToken=` 的 URL（`sanitizeError`）。新增配置项先判断是否含敏感值，含则必须走 `data/.env` 并做脱敏回显。

## 提交 PR

- 描述写清改了什么、为什么、怎么验证的（贴出实际跑过的命令与结果，不要只写「已测试」）。
- 保持 PR 小而聚焦，一个 PR 做一件事。
- **CI 的触发条件是 push 到 `master`/`main` 或对这两个分支开 PR**，并忽略纯文档改动（`paths-ignore: '**.md'`）；功能分支的裸 push 不会触发 CI，所以要把改动推上来跑门禁，请直接开 PR。CI 的 job 分布：

  | job | 运行环境 | 做什么 |
  | --- | --- | --- |
  | `ci-pipeline` | ubuntu-latest | 前端 `npm ci && npm run build && npm run guard && npm test`；后端 `go test -p 1 -count=1 -v ./...`（失败自动重跑一次）；`CGO_ENABLED=0` 单二进制构建；Linux/macOS 交叉编译检查 |
  | `ci-windows` | windows-latest | 前端 build；下载 win-amd64 onnxruntime；`CGO_ENABLED=1` 内嵌 ddddocr 的 GUI 单二进制构建 |
  | `ci-windows-arm64` | windows-11-arm | 装 llvm-mingw；下载 win-arm64 onnxruntime；`CGO_ENABLED=1` 构建 |

  注意 CI **不**构建 Android APK（在 `release.yml` 里），也**不**在 Windows/macOS 上跑测试——那三个 job 只验证「能不能编出来」。
- 涉及平台契约（zhidao.fj.cn 选课接口）的改动，请在 PR 里说明依据：改了哪个接口、请求编码、字段名或错误文案，依据来自真实站点行为还是抓包样本，有哪些实测结果。契约基线（HAR 与站点 JS）由维护者本地归档、整树被 `.gitignore` 忽略，**他人克隆后无从对照**；给不出依据的契约改动由维护者复核后才合并。

## 发版

1. **先更新 `CHANGELOG.md`**：在顶部加新版本小节（`## v0.2.28 - YYYY-MM-DD`，段名用 `### 修复` / `### 变更` / `### 新增`），写清用户可见的变化，只写用户能感知的行为，不写内部实现术语。
2. **合并到 `master`**（走 PR，CI 全绿）。
3. **打轻量 tag**（仓库历史 tag 除 `v0.2.16` 外全部是轻量 tag，不要加 `-a`/`-m`）：

   ```bash
   git switch master && git pull
   git tag v0.2.28
   git push origin v0.2.28
   ```

   推送 tag 触发 `release.yml`（tag 规则 `v*`）。`workflow_dispatch` 也能手动触发，但**无 tag 时产物名会退化成分支名、APK 版本变成 `0.0.0+<短 sha>`、versionCode 固定为 1**，只适合排障，正式发版一律用 tag。
4. **等四个构建 job + 汇总 job 跑完**（`gh run watch`）：`build-windows`（x64 GUI + 控制台，CGO=1 内嵌 ddddocr）、`build-windows-arm64`（同款，`windows-11-arm` runner + llvm-mingw）、`build-unix`（Linux amd64/arm64 CGO=1 内嵌 ddddocr；macOS arm64/amd64 CGO=0 不含内置引擎）、`build-android`（arm64-v8a APK：`libxuanke.so` 经 NDK 交叉编译 + 导出符号自检 + 版本注入校验 + 签名指纹断言），最后由 `release` job 汇总。产物清单：

   | 产物 | 形态 |
   | --- | --- |
   | `xuanke-<tag>-windows-amd64.zip` / `-windows-amd64-console.zip` | 内含 `xuanke.exe`（GUI 版带 `-H windowsgui`）与 `.env.example` |
   | `xuanke-<tag>-windows-arm64.zip` / `-windows-arm64-console.zip` | 同上，ARM64 |
   | `xuanke-<tag>-linux-amd64.tar.gz` / `-linux-arm64.tar.gz` | 内含 `xuanke`（`-tags notray`，服务器版无托盘）与 `.env.example` |
   | `xuanke-<tag>-darwin-arm64.tar.gz` / `-darwin-amd64.tar.gz` | 同上，无内置识别引擎 |
   | `xuanke-android-arm64.apk` | 侧载包，文件名**不含版本号**（版本在 APK 内：`v0.2.9` → versionName `0.2.9`、versionCode `209`） |
   | `checksums.txt` | `sha256sum` 覆盖上述 `*.zip` / `*.tar.gz` / `*.apk` |

5. **用 `CHANGELOG.md` 里该版本的正文替换 Release 自动说明**：把 `## v0.2.28` 到下一个 `##` 之前的内容另存为临时文件（如 `release-notes.md`，别提交），然后：

   ```bash
   gh release edit v0.2.28 --notes-file release-notes.md
   gh release view v0.2.28     # 复核
   ```

   `release.yml` 里 `generate_release_notes: true`，自动说明基于 commit 列表与对比链接，对用户没有信息量——尤其涉及「需要卸载重装」这类必须提前告知的变化时。
6. **Android 升级提示**：签名密钥固定在 [`build/android/xuanke.jks`](./build/android/xuanke.jks)，`release.yml` 会断言 APK 证书指纹与预期一致。**换密钥会让所有老用户无法覆盖安装**（只能卸载重装、清数据），改动前先确认；v0.2.15 → v0.2.16 已经踩过一次。

## 禁止事项

| 不要做 | 原因 |
| --- | --- |
| 手改 `backend/web/dist/` 或 `web/dist/` 里的文件 | 它们是 `npm run build` 的产物、被 `.gitignore` 忽略，且 embed 是编译期固化；手改既不入库，也会被下次构建覆盖 |
| 提交 `data/`、`*.db`、`.env`、`*.pem`、`*.key`、`.master_key` | 真实账号、会话与主密钥只允许留在本地 `data/`（`SECURITY.md` 红线） |
| 提交 onnxruntime 动态库、`libxuanke.so`、`dist-release/` 产物 | 16MB×N 的体积产物，由 `fetch-onnxruntime.sh` 与 CI 现下现编 |
| 替换 `build/android/xuanke.jks` 或改签名配置 | 会让已装 APK 无法覆盖安装，CI 指纹断言会红 |
| 把本地专有基线当公共文档引用 | `archive/legacy/` 的 HAR 与站点 JS、`AGENTS.md` 记忆库整树被 `.gitignore` 忽略，在他人克隆里不存在，引用等于给出死链 |
| 在源码、注释、文档、issue 里贴真实账号/学号/token/抓包原文 | 同上，属于敏感信息红线 |

## 行为准则

参与本项目即表示你同意 [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)。
