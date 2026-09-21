# review-round71 总结（2026-09-22）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮前端连续**十七轮零 MAJOR 零 MINOR**（M-1 第八轮低成本核对闭合）；后端 **CRITICAL 0 / MAJOR 0 / MINOR 1 / OBSERVE 4**——**生产逻辑连续九轮零 MINOR**。另按用户需求落地**系统托盘**（Windows/Linux 桌面右键菜单：打开浏览器/关于·数据库路径/退出）+ **激活码默认关闭 + 识别引擎默认 ddddocr** 双默认值。

**修复/新增**：config 双默认值（TDD）+ 托盘四文件 + main 接线 + 注释/README 同步 + MINOR-71-01 测试隔离 + OBSERVE-71-01/02 注释文档修复。

## 审查发现（写入 archive/review-rounds/round71-{backend,frontend}-findings.md）

### 后端（MINOR 1 / OBSERVE 4）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-71-01 | MINOR | R71 新增 config 测试裸调 Load() 会读真实 data/.env（旧默认 on/vision 时测试环境相关红）——默认值双源无一致性约束 | ✅ 修复（TDD 断言钉死 + 默认值改 off/ddddocr） |
| OBSERVE-71-01 | OBSERVE | main.go:71 + handler.go:935-941 注释仍写「默认 vision」与 ddddocr 实现脱节 | ✅ 修复（注释对齐 + 空串兜底对齐 ddddocr） |
| OBSERVE-71-02 | OBSERVE | README 仍列激活码默认 on + XUANKE_OPEN_TIME 死配置行 | ✅ 修复（README 同步 + 删死配置行） |
| OBSERVE-71-03 | OBSERVE | 托盘四文件未跟踪、main 接线半程（并行推进中的工作区） | ✅ 收尾（commit db6b08a） |
| OBSERVE-71-04 | OBSERVE | 本机 data/.env 在 config 测试运行中被被动改写（on→off）——MINOR-71-01 副作用 | ✅ 记录（gitignore 豁免不污染仓库） |
| R70 复核 | — | 探活成族正确（TestRecognizeCaptcha 断言限定 /chat/completions、TestLoginNetworkError 500 mock 就绪判定正确）；zhidao 连续 4 轮 -p 1 全绿，探活宿主清零成立 | ✅ 闭合 |
| 契约抽查 | — | 窗口关闭三判据 / 删号 memory-first / sameClientFor 六分支 / IsReadErr 全路径 / doLogin 闸门 / 开放时间唯一事实源——6 条全成立 | ✅ |
| flake | — | -p 1 串行 4 轮 3 绿 1 红（红为 MINOR-71-01 环境塑形非 connectex）；-p 2 并行 2 轮 3 例 connectex（api 包，隔离复跑全绿）——串行归零、并行残余仍在 | ⚠️ 记录 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 2）
连续十七轮零 MAJOR 零 MINOR。**M-1 第八轮低成本核对闭合** + **OBSERVE-66-03 setSelected 五调用点无新增无第三来源**。R70 注释归一核证通过（:529 指位 :482-483 准确、全仓「3 门」仅存说明注释）。OBSERVE-71-01（Select 防抖回调 selectedCount 未入 effect 依赖的既有 oxlint 项）**续**；OBSERVE-71-02（Dashboard 三注释协作旧案）**续**；OBSERVE-70-03（搜索框 aria-label 同串 DRY）**续**。构建全绿（target-guard 18/18 / audit exit 0 / oxlint 12 条全既有）。

## 用户需求落地（R71 专项）
| 需求 | 实现 | 验证 |
|---|---|---|
| 系统托盘（右键打开浏览器/退出） | systray 常驻托盘，Windows/Linux 桌面；菜单 打开浏览器/关于/退出 | 三平台构建通过 |
| 关于显示数据库路径 | Win32 MessageBox / zenity，显示数据库绝对路径+监听地址+选课大厅 | 构建通过 |
| 默认激活码不激活 | XUANKE_ACTIVATION 未设置 → off | TestActivationCodesDefaultOff 断言 |
| 托盘 UI 符合项目风格 | 深色系统主题自动暗色 + 程序内生成纯黑底白点 ICO | 构建通过 |
| Linux/Docker | Linux 桌面 CGO=1 带托盘；Docker/服务器 CGO=0 无托盘+自动开浏览器 | linux CGO=0 构建通过 |
| 只维护 win/linux/docker | darwin 走 tray_other 占位（不在维护清单） | darwin CGO=0 构建通过 |
| 默认识别引擎 ddddocr | CaptchaEngineDefault 未设置 → ddddocr | TestCaptchaEngineDefaultDdddocr 断言 |
| 注释字母数字标签清理 | 清理子代理逐文件剥离 F/B/R/M 等历史轮次标签（契约 20） | 进行中（见下） |

## 修复（主控核实后 TDD 直修）
| commit | 内容 |
|---|---|
| `18a76aa` | config 双默认值（激活码 off + 引擎 ddddocr）+ TDD 断言 |
| `db6b08a` | 托盘四文件 + main 接线 + 注释/README 同步（331 行新增） |
| 待清理代理 | 注释历史轮次标签剥离（契约 20，两批 commit） |

## 收尾全量回归
- `go build ./... && go vet ./internal/... ./cmd/...` → 全绿；`gofmt -l .` → 零输出
- 全量 `go test -race -count=1 -p 1 -timeout 900s ./...` → 第一轮全绿（store 69.5s）+ 第二轮待跑
- 前端 `npm run build` 全绿（注释清理不影响）

## 观察项延续（下轮复核）
后端：flake 串行归零/并行残余（api 包 connectex，CI -p 1 收口）/ OBSERVE-63-04 probe 工具 / OBSERVE-63-05 stats 半真 / OBSERVE-66-01 登记防御 / OBSERVE-70-03 store 批插极值；前端：M-1 延续管理 / OBSERVE-66-03 维持 / N-1~N-3 / O-1~O-12 / OBSERVE-71-01/02 续。

## 教训
1. **「默认值」是跨层契约，注释/文档/测试必须同源**：R71 改 config 两个默认值后，main.go 注释、handler.go 空串兜底、README 表格、.env 模板全部要与实现一致——「默认值双源」是 MINOR-71-01 的根因（测试读真实 .env 与模板各持一份默认值）。改默认值必须同步：实现 + 兜底 + 注释 + 模板 + README + 测试。
2. **托盘是多平台工程，build tag 隔离是唯一正确姿势**：systray 在 CGO=0 下无法编译（undefined nativeLoop），Linux 桌面（CGO=1）与 Docker（CGO=0）必须用 `linux && cgo` vs `linux && !cgo` 双文件隔离；darwin 不在维护清单走占位。任何「全平台都要」的需求都要先问「平台形态」——桌面 vs 无头容器是两回事。
3. **用户需求与审查流程并行推进时的协作纪律**：R71 用户在审查期间提了托盘+默认值+注释清理三个需求，与 R71 审查发现（MINOR-71-01 等）交错——主控按「审查发现修审查的、用户需求落用户的」并行推进，最后统一收尾提交，避免子代理与主控在同一文件上打架（R71 后端代理曾 stash 还原 go.mod 属轻微冲突，已透明记录）。