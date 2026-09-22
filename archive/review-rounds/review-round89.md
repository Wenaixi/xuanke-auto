# review-round89 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 1**；前端 **零 CRITICAL/MAJOR/MINOR/零新 OBSERVE**（连续第三十五轮零严重级）。核心：**B88-01 修复行为面复核闭环 + 身份防线全员清点 16 项矩阵闭合**——R88 教训落地，无新缺口。

## 审查发现（写入 archive/review-rounds/round89-{backend,frontend}-findings.md）

### 后端（OBSERVE 1）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O89-01 | OBSERVE | probe_identity_test.go 缺尾换行（git 提交内容即含 `\ No newline`，随 B88-01 进入仓库），纯格式噪音 | ✅ **采纳修复**（9d37fc0）：补末尾换行 gofmt 合规 |
| B88-01 复核 | — | ProbeForAccount 回写段身份复核行为面正确——已删/同名重建整体放弃写 acctData/acctDataAt/识别槽（三不写）；「身份通过 && beginTimes 非空」双条件才覆盖识别槽（空 beginTimes 不破坏「关闭≠时间消失」）；锁序无变化；probe() per-account goroutine 复用同一路径全一致；三钉实测绿（1.434s） | ✅ 闭环 |
| 身份防线全员清点 | — | **16 项矩阵**逐点 grep 闭合：spawnChain 六分支 + 实时复核 ErrUnauthorized 第七分支 + maybeRelogin 决策/写回双侧 + ProbeForAccount（本轮修复第 10 点）+ 手动 MarkDone/RemoveDone + submitAll 链顶；ProbeNow/probe() 全局帧无身份维度风险（正确性在全局语义侧成立） | ✅ 零裸露写点 |
| O86-01（延续） | OBSERVE | api 抖动**第四轮零复现**——race 全绿（api 29.0s/store 28.8s）connectex 零样本 | ⚠️ 维持基线 |
| O88-01/02 | OBSERVE | sharedTransport TLS 显式性 / Shutdown 反向慢连——复核通过无新触发面 | ⚠️ 记录 |
| M87-01 | — | 窗口再评估维持 MINOR + 注释兜底 | ✅ 维持 |

### 前端（零新 OBSERVE / 1 归档 / M-1 第二十五轮闭合）
- **M-1 第二十五轮闭合** + 六防保存链零回归 + 新视角五组全收敛（react-query 缓存无泄漏 / 表单校验 Infinity 钳制 / Admin 五 Tab 状态同步 / **时间显示实测关键：空格分隔 open_time V8 按本地解析正确（getTime=1789261200000 与 CLAUDE.md 时间戳一致，历史「解析失败」顾虑不成立）** / 请求取消三件套完备）。
- **OBSERVE-86-01 归档**：F88-01 htmlFor 复核通过（ca08c46 六对 label+id 逐字符配对、全仓零未配对、id 唯一、与 Login 同款、git diff 零意外改动）。
- **OBSERVE-88-01（ErrorBoundary）维持观察不落地**：预防性复杂度 vs 88 轮零渲染期异常实证，后端契约破坏性变更时才值得 ~20 行兜底。
- OBSERVE-85-02/84-01/83-01/77-02/76-01/76-02/76-03 延续。

## 修复（主控核实后）
1. **O89-01（后端格式项）**：probe_identity_test.go 补末尾换行（9d37fc0），纯格式 gofmt 合规，无专门回归需求。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 12 包全绿 exit 0**（api 29.0s / store 28.8s / scheduler 16.5s，审查代理实测 + 主控复核）
- `go build ./...` / `go vet ./...` 零输出 + Windows CGO1 / Linux CGO0 双平台交叉编译通过
- 前端 `tsc -b` EXIT 0 + `npm run build` 全量成功 + target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 提交链
- `9d37fc0` style(backend): O89-01 补末尾换行

## 观察项延续（下轮复核）
后端：身份防线矩阵作为持续审查清单（每轮补防线都按 16 项矩阵清点）；O86-01 抖动基线（连续四轮零复现）；O88-01/02；M87-01 窗口留档。前端：M-1 延续管理（第二十六轮）/ OBSERVE-88-01 ErrorBoundary（低优先级候补）/ 85-02 / 84-01 / 83-01 + 77-02 + 76-01/02/03 全表续。

## 教训
1. **「补身份防线」的正确姿势 = 全员清点矩阵**：R88 修复 B88-01 后，R89 立即按 16 项矩阵逐点 grep 复核，确认 ProbeForAccount 是最后裸露写点、ProbeNow/probe() 全局帧无身份维度——**修复带出「清点矩阵」作为附录，下轮直接复核**，比每轮重新扫更高效。
2. **全局帧与 per-account 帧的身份风险分叉**：ProbeNow/probe() 写 lastData（全校全局帧）不适用删号+同名重建的年级串线风险（无需 sameClientFor），per-account 帧（acctData/openTimeDetected）才需要身份复核——**身份防线只加在有账号维度的写点**，全局帧加了是过度防御。
3. **「识别槽仅在身份通过 && beginTimes 非空双条件覆盖」的语义保持**：B88-01 修复把识别槽覆盖并入身份复核段，但保留「空 beginTimes 不覆盖、保留旧识别值」的「关闭≠时间消失」契约——**修复不得改变既有语义契约**（复核只加在写前、不改写条件）。
4. **格式噪音单独提交**：O89-01 缺尾换行随 B88-01 进入仓库，因涉及测试文件按「纯格式项」单独提交（9d37fc0）不混入下次修复——格式修复独立 commit 便于审计。