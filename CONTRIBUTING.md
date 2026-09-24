# 贡献指南

欢迎为至道选课自动化（xuanke-auto）贡献代码。项目不大，期望每一份改动都让仓库更干净、更好维护。

## 前置阅读

- 仓库内文档：`README.md`（上手指南）、`SECURITY.md`（敏感信息红线）、`CODE_OF_CONDUCT.md`（行为准则）。

## 工作流程

1. **Fork 本仓库**，从 `master` 开分支（`feature/xxx` 或 `fix/xxx`）。
2. **小步提交**：一个小模块或一个小修复一次 commit，提交信息用中文，写清"改了什么、为什么"。
3. **CI 验证（推荐）**：push 到 master/main 或开 PR，GitHub Actions 自动跑后端全量测试 + 前端构建 + 全平台交叉编译（Windows x64/arm64、Linux、macOS）。本地无需安装 Go/C 工具链。
   - 仅当你要本地调试时才手动构建：先 `bash backend/scripts/fetch-onnxruntime.sh windows amd64` 下载识别引擎库，再 `cd backend && CGO_ENABLED=1 go build ./...`。
   - 后端回归注意：改 tick 守卫或探测时序前，先跑 `go test -run 'TestWindowOpenSubmitsWithoutProbeReset|TestAdminStatsWindowOpenedUsesScheduler'`。
   - 前端回归以 `npm run build` 为准（项目根 `tsc --noEmit` 是 references 空壳，不报错）。
4. **提交规范**：代码注释只写"为什么 / 契约 / 陷阱"，不要出现"第 N 轮""B18-XX"式轮次前缀（决策历史由维护者本地归档，不入仓库）。

## 写测试

- 新功能或缺陷修复一律走 **TDD**：先写失败测试（红灯）→ 最小实现（绿灯）→ 重构。
- 测试断言真实行为，不"手写实现当断言"（否则恒假绿，测不出契约）。
- 后端测试放 `backend/internal/<pkg>/` 对应包；前端暂无测试框架，用 `npm run build` 类型检查 + `npm run guard` 守卫脚本兜底。

## 编码规范

- **语言**：注释与提交信息用简体中文；代码标识符用英文。文档不用 emoji。
- **简洁优先**：不过度设计，不为一次性代码建抽象；能用标准库就不用第三方库；`go test -race` 的规范先于性能。
- **精准修改**：只碰必须碰的；不顺手"改进"无关代码；发现死代码在 PR 里指出，而非顺手删。
- **安全**：不把口令/token/密钥提交进仓库（见 SECURITY.md）；日志只打印敏感值前 8 位；新增配置项先确认是否含敏感值（含则必须走 data/.env + 脱敏回显）。

## 提交 PR

- 描述清"改了什么、为什么、怎么验证"。
- 涉及平台契约（zhidao.fj.cn 选课接口）的改动，请在 PR 里说明与真实站点行为的对应关系（契约基线由维护者本地归档，不入仓库）。
- 保持 PR 小且聚焦，一个 PR 一件事。

## 行为准则

参与本项目即表示你同意 [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)。