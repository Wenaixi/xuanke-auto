# 贡献指南

欢迎为至道选课自动化（xuanke-auto）贡献代码。项目不大，期望每一份改动都让仓库更干净、更好维护。

## 前置阅读

- `README.md`：上手指南与架构概览
- `SECURITY.md`：敏感信息红线（改配置或日志前必读）
- `CODE_OF_CONDUCT.md`：行为准则
- `.github/workflows/ci.yml`（质量门）与 `release.yml`（发版）

## 工作流程

1. **Fork 本仓库**，从 `master` 开分支（`feature/xxx` 或 `fix/xxx`）。
2. **小步提交**：一个小模块或一个小修复一次 commit。提交信息用中文，写清改了什么、为什么。
3. **验证**：push 到分支或开 PR，`ci.yml` 会自动跑后端全量测试 + 前端构建 + 全平台编译（Windows x64/arm64、Linux、macOS），全绿才算过。
   - 日常开发不用本地装 Go/C 工具链。偶尔要在本地调试，先 `bash backend/scripts/fetch-onnxruntime.sh windows amd64`，再 `cd backend && CGO_ENABLED=1 go build ./...`。
   - 改 tick 守卫或探测时序前，先跑 `go test -run 'TestWindowOpenSubmitsWithoutProbeReset|TestAdminStatsWindowOpenedUsesScheduler'` 对照新旧行为。
   - 前端回归以 `npm run build` 为准。项目根的 `tsc --noEmit` 是 references 空壳，不会报错。
4. **提交规范**：代码注释只写为什么、契约、陷阱，不要带「第 N 轮」「B18-XX」这类轮次前缀。决策历史由维护者本地归档，不进仓库。

## 写测试

- 新功能或缺陷修复走 TDD：先写失败测试（红灯），再最小实现（绿灯），然后重构。
- 测试要断言真实行为，不能把实现抄一遍当断言，否则恒假绿，测不出契约。
- **后端**：测试放在 `backend/internal/<pkg>/` 对应包里，`go test -race ./...` 全量跑。
- **前端**：目前只有两套测试手段。一是纯函数测试（vitest，`npm test`，测试文件为 `src/**/*.test.ts`），二是源码形状守卫（`npm run guard`，六个脚本断言字体/性能/目标保存/管理态/401/懒加载这些契约没有被改回去）。这里没有 DOM 环境，也没有 `.tsx` 组件测试，所以组件层的行为要靠守卫脚本或人工验证来兜。
- 测试要能跨平台跑（CI 覆盖 Linux/Windows/macOS）：不依赖 Windows 保留路径，不硬编码平台特有的错误文案（如 `connectex`），不假设 CI 环境装有 Python。

## 编码规范

- **语言**：注释与提交信息用简体中文，代码标识符用英文，文档不用 emoji。
- **简洁优先**：不做过度设计，不为一次性代码建抽象，能用标准库就不用第三方库，通过 `go test -race` 比性能优化优先。
- **精准修改**：只碰必须碰的地方，不顺手改进无关代码。发现死代码在 PR 里指出，不要顺手删掉。
- **安全**：不把口令/token/密钥提交进仓库（见 `SECURITY.md`），日志只打印敏感值前 8 位。新增配置项先判断是否含敏感值，含则必须走 `data/.env` 并做脱敏回显。

## 提交 PR

- 描述写清改了什么、为什么、怎么验证的。
- 涉及平台契约（zhidao.fj.cn 选课接口）的改动，请在 PR 里说明与真实站点行为的对应关系。契约基线由维护者本地归档，不进仓库，外部无法对照。
- 保持 PR 小而聚焦，一个 PR 做一件事。

## 发版

1. **先更新 `CHANGELOG.md`**：在顶部加新版本小节，写清用户可见的变化（修复 / 新增 / 变更），只写用户能感知的行为，不写内部实现术语。
2. 合并到 `master` 后打标签（`v0.2.x`）并推送，标签会触发 `release.yml` 出全平台产物并建 Release。
3. **Release 说明用 `CHANGELOG.md` 里该版本的正文替换自动生成的内容**（`gh release edit vX.Y.Z --notes-file <文件>`，或直接在网页上粘贴）。`generate_release_notes` 只会给出一行 commit 对比链接，对用户没有信息量——尤其涉及「需要卸载重装」这类必须提前告知的变化时。
4. 安卓包签名密钥固定在 `build/android/xuanke.jks`，CI 会断言 APK 证书指纹与预期一致。**换密钥会让所有老用户无法覆盖安装**，改动前先确认。

## 行为准则

参与本项目即表示你同意 [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)。
