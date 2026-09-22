# review-round88 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → **TDD 修复** → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 1（走读推断）/ OBSERVE 2**；前端 **零 CRITICAL/MAJOR/MINOR**（连续第三十四轮零严重级）+ 1 条新 OBSERVE。**核心：M88-01 实质修复（B88-01，ProbeForAccount 回写段身份复核）**——B43 身份防线族最后一个裸露写点闭合。

## 审查发现（写入 archive/review-rounds/round88-{backend,frontend}-findings.md）

### 后端（MINOR 1 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M88-01 | MINOR（走读推断） | ProbeForAccount 探测期间「删号 + 同名重建 + 在飞探测同帧」，旧链 FindElectives 网络往返返回后仍把快照写进重建账号的 acctData/acctDataAt、识别槽 openTimeDetected 亦可能被旧批次覆盖——B43 族最后一个未用指针身份复核的写点 | ✅ **采纳修复（B88-01）**：回写段持锁先 `sameClientFor(acct, client)` 复核（与 spawnChain 六分支/重登写回侧同族），身份已变/已删整体放弃；TDD 三钉先红后绿 |
| O88-01 | OBSERVE | sharedTransport 显式 TLSClientConfig 恒 nil——走读核算持默认语义（系统证书池/defaultVerifyTLS）、与契约 43 正交、实测全绿 | ⚠️ 记录（显式性边缘项，无需补） |
| O88-02 | OBSERVE | srvShutdown 5s vs 进程退出在「反向慢连」形态极端尾部——Shutdown 返回后 main 返回路径上 tick 可能再跑一拍并 spawn 新链被进程强杀吞掉（理论边界，M87-01 同族） | ⚠️ 记录（未来退出流程重构参考） |
| O86-01（延续） | OBSERVE | api 抖动零复现——race 全绿（api 23.4s/store 39.8s）connectex 零样本 | ⚠️ 维持基线 |
| M87-01（延续） | — | 注释兜底充分性核验——main.go:191-192 含两分支语义（在飞若超时前返回则写状态、否则进程强杀），未来轮次可据此理解边界 | ✅ 复核通过 |

### 前端（OBSERVE 1 / 1 落地建议 / M-1 第二十四轮闭合）
- **M-1 第二十四轮闭合** + 六防保存链零回归 + 新视角五组全收敛（refetchInterval 升降频 / 焦点管理 / ErrorBoundary / 文案一致性 / 快速切号竞态）。
- **OBSERVE-88-01（新）**：ErrorBoundary 全仓零命中——渲染期异常理论白屏无反馈；正常路径防御充分 + 88 轮零实证，评低优先级防御候选（~20 行修复）。⚠️ 维持观察。
- **OBSERVE-86-01 落地**（F88-01）：Admin 五处 label 补 htmlFor + Input 补 id（Login 同款，5 label + 5 id，纯可访问性增强零行为变更）。
- OBSERVE-85-02/84-01/83-01/77-02/76-01/76-02/76-03 延续。

## 核实记录（M88-01 深度核实）
- 重读 scheduler.go:816-869（ProbeForAccount 全流程）：入口已取 client 指针、FindElectives 网络段后回写 acctData/acctDataAt/识别槽——确认「入口 ClientFor 存在性」与「回写段身份复核」之间的网络窗口真实存在（删号+同名重建发生在网络往返内的左链/右链争夺）。
- sameClientFor（:208-214）+ clientIdentity（:219-228）复用 spawnChain 同款实现，回写段持锁调用不新增锁序风险（s.mu 已在持锁段）。
- **TDD 三钉**（probe_identity_test.go）：
  1. `TestProbeDeletedThenRebuiltSameNameDropsSnapshot`：旧身份阻塞式 FindElectives 期间删号+同名重建 → 回写丢弃 → 修复前 FAIL（旧链写回重建账号）→ 修复后 PASS
  2. `TestProbeForAccountDropsWriteWhenRemoved`：探测发起后已删 → 回写整体放弃 → 修复前 FAIL → 修复后 PASS
  3. `TestProbeChainSameClientIdentity`：身份不变正常路径照常落新帧（对偶防误伤）→ 恒 PASS

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 全 11 包全绿（后台实证）+ scheduler 全量 race 21.7s 绿（含新三钉）
- `go build ./...` / `go vet ./...` / `gofmt -l .` 零输出 + GOOS=windows CGO1 交叉编译通过
- 前端 `tsc -b` EXIT 0 + `npm run build` 全量成功（419.87 kB JS）+ target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 提交链
- `ca08c46` fix(backend): B88-01 ProbeForAccount 回写段身份复核 + F88-01 Admin htmlFor（4 文件 243+ 11-）

## 观察项延续（下轮复核）
后端：O88-01 TLS 显式性 / O88-02 退出极端尾部 / O86-01 抖动基线 / M87-01 窗口留档；前端：M-1 延续管理（第二十五轮）/ OBSERVE-88-01 ErrorBoundary（低优先级候选）/ 85-02 / 84-01 / 83-01 + 77-02 + 76-01/02/03 全表续；OBSERVE-86-01 落地归档。

## 教训
1. **「入口有复核 ≠ 回写有复核」——网络往返后写点必须逐一清点**：M88-01 的 ProbeForAccount 入口已有 ClientFor 存在性检查，但 FindElectives 15s 网络段后回写没有任何身份复核——B43 族（spawnChain 六分支/重登写回侧/maybeRelogin 决策侧）已闭合多年，ProbeForAccount 是漏网最后一个。**教训：每次补身份防线都要 grep 全部「网络往返后持锁写状态」点，入口存在性检查不是回写侧防线**。
2. **走读推断 MINOR 也值得 TDD 实证**：审查代理标注「走读推断」，但三钉先红后绿完整复现（旧链写回重建账号 FAIL 真实触发）——不是「红不了的问题」，是「可测试的身份竞态」。之前 B43-03 的教训是「红不了=不是问题」，M88-01 相反：红得了=必须修。**走读推断 → 先写测试看红不红 → 红就修、不红就记录语义兜底**。
3. **测试夹具的阻塞钩子族要成族补齐**：fakeClient 已有 selectBlock/fullBlock 两个网络阻塞钩子，FindElectives（探测路径）缺——补 findBlock 后「在飞探测竞态」族测试才可写。**成族类夹具缺失是测试覆盖盲区的信号**。
4. **零成本修复族保持纪律**：OBSERVE-86-01（htmlFor）连续两轮评估维持后，本轮前端代理建议落地、主控采纳——5 label + 5 id 零行为变更，可访问性弱项不堆积。**观察项在「低优先级候选」与「落地」之间要有决断，不无限期挂账**。