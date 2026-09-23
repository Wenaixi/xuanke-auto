# review-round97 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 4**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / 零新增 OBSERVE**（连续第四十三轮零严重级）。**双端零代码修改需求**——连续第五轮纯观察轮，核心产出：**O97-01 抖动残余与 R94 同族定级 + 身份防线矩阵第十二轮闭合**。

## 审查发现（写入 archive/review-rounds/round97-{backend,frontend}-findings.md）

### 后端（OBSERVE 4）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O97-01 | OBSERVE | 抖动基线第十二轮——三轮全量 race 二绿一「zhidao 单包单次 FAIL」（34.4s 异常耗时、无 panic 无 DATA RACE）；定向复现独立全量 1 次全绿 + count=3 全绿 + -json 全绿 + TestLogin 六项全绿 | ⚠️ 维持（与 R94 同族归因：Windows 回环冷启动窗口低频残余，CI `-p 1` + 失败重跑吸收口径不变，非产品缺陷） |
| O97-02 | OBSERVE | gofmt CRLF 噪音延续（scheduler.go 工作区 CRLF vs git LF 合规） | ⚠️ 记录 |
| O97-03 | OBSERVE | 契约 20 注释回归仅一处文档路径引用非违规（归档文件名可追溯性） | ⚠️ 记录 |
| O97-04 | OBSERVE | 删除保护撞名维持观察 | ⚠️ 维持 |
| 身份防线矩阵延续 | — | 16 项矩阵第十二轮逐点 grep（sameClientFor 全调用点：spawnChain 六分支 + 实时复核 + maybeRelogin 双侧 + 探测回写 + MarkDone/RemoveDone/SubmitAll + Restore）无新裸露写点 | ✅ 闭合 |
| B88-01 持续复核 | — | 回写段双条件识别槽覆盖 + probe_identity_test.go 三钉 + round41 七分支族定向全绿 | ✅ 闭环 |

### 前端（零新增 OBSERVE / M-1 第三十三轮闭合）
- **M-1 第三十三轮闭合**（四消费点逐字符复核、置位三路径 + 首帧边界完整、读点 9 处无第五消费处）+ 六防零回归 + **F93-01 第五轮复核**（Button.tsx:42 ring 在位、git log 实测 R93 后 web/ 零提交零漂移）。
- 新视角四项全收敛：状态更新一致性 / 表单键盘可达性 / 视觉反馈层次 / 深链浏览器行为。
- OBSERVE-90-01/88-01/85-02/84-01/83-01/77-02/76 族/O-3 族延续。

## 核实记录（关键）
- **O97-01**：zhidao 单包单次 FAIL 34.4s 异常耗时（正常 2-3s）——无 panic 无 DATA RACE 排除数据竞争；定向复现彻底（独立全量/count=3/-json/TestLogin 六项全绿）证明非确定性缺陷；与 R94 connectex 低频残余同族（Windows 回环冷启动窗口）。**裁决：维持 CI 重跑吸收口径，不推荐夹具改动**。
- **手动操作状态迁移穷举**（后端代理新角度）：TryAcquireSubmit→CheckClassSelectable→SelectClass/ExitClass→MarkDone/RemoveDone 全分支闭环（ErrUnauthorized 走重登绝不清退避 / read 类「可能已处理」文案 / doneHas 绝不覆盖胜利状态 / MarkDone 解除 refused 恢复自动接管）——主控认可走读证据。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 三轮二绿一 zhidao 单包残余（O97-01 记录范围）+ 定向复现全绿
- `go build ./...` / `go vet ./...` 零输出
- 前端 `tsc -b` EXIT 0 + `npm run build` 全量成功（1948 modules）+ target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 观察项延续（下轮复核）
后端：O97-01 抖动基线（zhidao 残余面与 R94 同族）/ 身份防线矩阵（第十三轮）/ O97-02/03/04 / M87-01 窗口；前端：M-1 延续管理（第三十四轮）/ OBSERVE-90-01 日志区三态 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **zhidao 单包异常耗时 FAIL 的定位方法**：34.4s 异常耗时 + 无 panic 无 DATA RACE → 定向复现四连（独立全量/count=3/-json/TestLogin 六项）全绿 → 归因冷启动窗口——**异常耗时的单包 FAIL 优先排查宿主环境而非数据竞争**（race 检测零告警 + 定向复现全绿是排除产品缺陷的完整证据链）。
2. **身份防线矩阵十二轮零新裸露写点**：清单化复核已连续十二轮把防线缺口类发现面系统性收窄——矩阵价值随轮次累积验证。
3. **前端 43 轮零严重级的「web/ 零提交」实证**：F93-01 第五轮复核用 git log 实测「R93 后 web/ 零提交」——连续多轮前端零改动的状态下，复核聚焦「零漂移」比「找新缺陷」更有效（状态稳定期观察项管理纪律）。