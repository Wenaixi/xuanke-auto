# review-round101 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 1（B101-01）/ OBSERVE 4**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / 零新增 OBSERVE**（连续第四十七轮零严重级）。**后端终结连续九轮回合零修复记录，落地 1 条实质修复（token 脱敏契约缺口）**；前端连续第九轮纯观察。核心产出：**B101-01 网络层错误剥 URL 脱敏 + 身份防线矩阵第十六轮闭合 + 抖动基线十六轮零复现**。

## 审查发现（写入 archive/review-rounds/round101-{backend,frontend}-findings.md）

### 后端（MINOR 1 + OBSERVE 4）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| B101-01 | MINOR | doRequest 网络层失败时标准库 `*url.Error` 文本回放完整请求 URL（含 `?idToken=` 完整 token），经错误包装链进入进程日志（scheduler.go:1098/:1294）、库内 task_log + /api/state + /api/logs 回显（:1654-1655）、手动报名/退选前端回显（handler.go:360/:426）——破坏「token 只显前 8 位」脱敏契约的全路径完整性 | ✅ **修复**（TDD 先红后绿，见下） |
| B101-02 | OBSERVE | 日志脱敏完整性全路径扫描——除 B101-01 外全部写点闭环（maskedToken/tokenShort/Vision 原文脱敏/maskKey 逐点归档） | ⚠️ B101-01 修复后闭环达成 |
| B101-03 | OBSERVE | 契约 20 强弱扫描——X-XX 锚点 5 处（M86-01/M88-01）全部映射决策手册契约条目、内容为「为什么/契约」合规 | ⚠️ 维持（历轮裁定延续） |
| O101-01 | OBSERVE | 抖动基线第十六轮——定向 race（三包+七包+单包复跑+全 10 包）**本轮零复现**；socketPreheat/readyProbe 双保险使用正确、无新脆弱点；低频口径维持（R94-R101 八轮 30 跑 4 单 FAIL ~13%） | ⚠️ 维持（CI `-p 1` + 失败重跑吸收） |
| O101-02 | OBSERVE | 删除保护撞名延续（handler.go:992 单判据 vs B43-04 双条件不对称）——三条历史理由本轮逐一复核仍成立 | ⚠️ 维持观察 |
| B101-04/05/06 | — | 新契约角度全绿：多账号并发提交全局对称性（sessionAccount 绑定 + 管理员穿透凭证表判据 + 无 URL 猜账号旁路）/ SQLite 锁内无网络慢操作 + 登录期 URL 无 idToken 不泄露 / SyncServerTime 对齐时钟无遗漏分支 | ✅ 闭合 |

### 前端（零新增 OBSERVE / M-1 第三十七轮闭合）
- **M-1 第三十七轮闭合**（shouldDeferSave 三消费点 + handleBack while 四处全传 `echoedRef.current` 第三参逐字符一致、置位三路径 + 首帧边界完整、读点代码级引用 9 处无第五消费处）+ 六防保存链零回潮 + target-guard 18/18 实测全绿。
- **F93-01 第九轮复核**：Button.tsx:42 ring 逐字符在位、`git log f08937e..HEAD -- web/` 实测空集（web/ 零代码提交）、build 产物哈希与 R100 逐字节一致。
- **新契约角度**：aria 动态态四类（expanded/checked/pressed/valuenow）随 state 零滞后 + Progress 未公布场景 valuenow=0 注释承诺已落实；性能边界（每秒 setNow 整树重渲染面历轮一致、列表 key 全量零不稳、useMemo 五处全有稳定引用目的）；tabular-nums 9 处零缺漏。O-3 族补充「模态卸载焦点回落 body、长列表场景数十次 Tab 重导航」影响面量级论证（用户体验级，维持观察不升级）。

## 核实记录（关键）
- **B101-01 深度核实**：主控逐行读 doRequest（client.go:414-464）确认 URL 拼 `?idToken=` 完整 token；httpDo（:474-488）只对 dial/write 自愈一次、重试仍失败原样上抛；三消费链原文透传。根因选点：zhidao 层上抛前统一剥 URL（一处拦截覆盖全部下游）vs 消费侧四处各剥——取前者（更小 diff + 判型不受影响）。
- **TDD 先红后绿**：新写 sanitize_test.go 六测试（剥完整 token/判型保留 errors.As·Is 穿透/nil 安全/业务错误原样透传不误伤/输出非空/**端到端真实 connectex 向必拒端口发带 token 请求**）→ 编译期红（缺 sanitizeError 符号）→ client.go 加 sanitizerErr（Unwrap 下沉）+ sanitizeError 实现 + doRequest 接入 → 全绿。zhidao 包全量测试 8.84s 零回归。
- **回归**：10 包 `-race -count=1 -p 1` 全绿（zhidao 20.7s/scheduler 21.2s/api 18.1s/store 23.5s 等）+ `go build`/`go vet` 零输出。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 ./internal/{zhidao,scheduler,api,store,db,session,accounts,config,runtime,secure}` → **10 包全绿**
- `go build ./...` / `go vet ./...` 零输出
- 前端 tsc EXIT 0 + build 产物哈希与 R100 逐字节一致 + target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项（审查代理实测）

## 观察项延续（下轮复核）
后端：身份防线矩阵（第十七轮）/ O101-01 抖动基线 / O101-02 删除保护撞名 / B101-03 契约 20 / M87-01 窗口；前端：M-1 延续管理（第三十八轮）/ OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。**B101-01 修复成为新契约**：网络层错误统一剥 URL 脱敏；新消费点或错误链扩展不得再让 url.Error 原文落日志。

## 教训
1. **契约类缺口要追全路径而不是单点**：B101-01 第一眼是「一个网络错误日志」——顺着错误包装链追下去才发现它贯穿日志/库表/前端三端。**「token 只显前 8 位」这类横切契约，审查必须问「这条路径每一环是否都遵守」，而不是「是否有任一环遵守」**。
2. **判型与展示分离的脱敏模式可复用**：sanitizerErr 剥文本 + Unwrap 下沉——展示脱敏却不破坏 errors.As/Is 判型链。这是「文本改写」类脱敏的通用做法（错误判型依赖底层错误，剥 URL 只伤展示不伤语义）。
3. **多轮审查 + 稳定期后仍可能挖出真实缺陷**：连续多轮零严重级 ≠ 无缺陷——B101-01 是连续多轮纯观察后由「日志脱敏全路径」新审查角度挖出的实质修复，证明新契约角度走查的价值在稳定期依然成立。