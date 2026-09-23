# review-round104 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归 → 归档。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + B104-01 知识位 + OBSERVE 6 延续**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 + 新增 OBSERVE 2（OBSERVE-104-01/104-02）**（连续第五十轮零严重级）。**后端零新增代码修改；前端落地 2 条实质修复（无障碍基础项 OBSERVE-104-01/104-02，web/ 自 f08937e 后首次代码提交 037410a）**。核心产出：**身份防线矩阵第十九轮闭合 + B104-01 知识位（历史审查误读纠偏）+ 前端无障碍纵深首查暴露两缺陷并修复 + 前端 M-1 第四十轮闭合**。

## 审查发现（写入 archive/review-rounds/round104-{backend,frontend}-findings.md）

### 后端（MINOR 0 + B104-01 知识位 + OBSERVE 6 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| B104-01 | 知识位 | 手动报名/退选路径「同一身份」防线缺位是**理论缺口而非历史遗留裸写点**——api 手动路径是同步单 goroutine、无「解锁→在飞网络→回锁」交错窗口，删号+同名重建无法插入；B20-01 存在性复核已封死 | ✅ 钉死契约（活化条件：未来若引入异步手动报名「跨 goroutine 网络段」，MarkDone/RemoveDone 才需升级 sameClientFor 指针身份比对） |
| O104-01 | OBSERVE | 抖动基线第十九轮——夹具走读无新脆弱点、定向跑多批全绿零 flake、~13% 低频口径维持 | ⚠️ 维持 |
| O104-02 | OBSERVE | 删除保护撞名延续（handler.go:992 单判据 vs B43-04 双条件不对称）——三条历史理由复核仍成立 | ⚠️ 维持观察 |
| O104-03/04/05/06 | OBSERVE | logintest 维持关闭 / M87-01 维持 MINOR / CRLF 维持 / 契约 17 零吞错通过 | ⚠️ 均维持历轮结论 |
| 身份防线矩阵延续 | — | **第十九轮闭合**（sameClientFor 七调用点 :850/:1489/:1521/:1551/:1571/:1600/:1635 全在历史清单内、第七分支落统一复核、MarkDone/RemoveDone/SubmitAll/api 手动路径四路封口完整——无新裸露写点） | ✅ 闭合 |
| 手动迁移/DB 事务 | — | 手动操作状态迁移穷举闭环无缺口；DB 并发写与事务边界复查全正确（单写者锁天然串行、事务族规范、迁移/Restore 顺序与契约一致） | ✅ 通过 |

### 前端（零严重级 + 新增 OBSERVE 2 + 修复 2）
- **OBSERVE-104-01（新）**：TabsTrigger 渲染 `<button>` 被 global.css:174 统一 `outline:none` 抹掉焦点环，base class 仅 `focus-visible:outline-none` 无 ring 补偿——键盘 Tab 聚焦 Tab 页无可见焦点（Select 发布 Tab + Admin 五 Tab 触发面），WCAG 2.4.7 违例。F93-01 只收敛 Button 组件，TabsTrigger 同族同源从未入列。**主控核实实锤 → TDD 修复**：Tabs.tsx:29 base class 补 `focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`（与 Button :42 同款 token，全站 Tab 一次收敛）。
- **OBSERVE-104-02（新）**：Admin 账号管理表格缺表格语义——`<table>` 无 aria-label、`<th>` 无 scope="col"，读屏表格语义断裂。**主控核实实锤 → TDD 修复**：补 `<table aria-label="账号列表">` + 四 `<th scope="col">`。
- **备注不立条**：Admin :792/:898/:944 三个 refetch 裸按钮历轮清单遗漏补录，随 OBSERVE-93-01 残余面合并跟踪。
- **M-1 第四十轮闭合**（四消费点 :509/:597/:605/:699 全传 echoedRef.current 逐字符一致、置位三路径 + 首帧边界完整、读点代码级引用 9 处无第五消费处）+ 六防零回潮 + target-guard 18/18 实测全绿。
- **F93-01 第十二轮复核**：Button.tsx:42 ring 逐字符在位、web/ 零漂移实证（本轮后终结——037410a 为 web/ 首次代码提交）。
- **新契约角度（无障碍纵深）**：焦点环族（Input/Button/Admin 开关/Login 密码切换全达标）、aria 动态态族（Progress/CollapseSection/switch/pressed/Radix toast aria-live 源码实证）、模态语义族（三模态全链在位）、Label 关联族/autoComplete 族全达标——唯 TabsTrigger 焦点环与 Admin 表格语义两缺陷（已修）。

## 核实记录（关键）
- **OBSERVE-104-01 实锤**：主控读 Tabs.tsx:29 逐字符确认 `focus-visible:outline-none` 无 ring 补偿 + global.css:174 对 button 统一 outline:none——键盘 Tab 聚焦 Tab 页焦点不可见是真实缺陷（非推断）。修复与 Button :42 同款 token，一处收敛全站 Tab。
- **OBSERVE-104-02 实锤**：主控读 Admin.tsx:833-840 确认 table 无 aria-label、四 th 无 scope——读屏语义断裂真实存在。一行族修复（aria-label + 四 scope）。
- **B104-01 知识位（历史误读纠偏）**：R39/R41 曾把 spawnChain 的「存在性≠同一性」泛化到手动 MarkDone/RemoveDone——本轮走读证实该指控从未成立（手动路径同步单 goroutine 无在飞窗口），B20-01 已封死。活化条件固化。
- **TDD 验证链**：tsc EXIT 0 + build 成功 + target-guard 18/18 + admin-auth 6/6 + unauthorized 5/5 + audit 全绿。

## 收尾全量回归
- 后端定向 `go test -race`：zhidao 脱敏族 3.4s 全绿 + scheduler 身份防线族 4.96s 全绿 + session/db/accounts + store 24.7s + api 管理族 9 测全绿 + `go vet` 八包零输出（审查代理实测）
- 前端 tsc EXIT 0 + build 成功（修复后首次产出新哈希）+ 三组断言 18/18 + 6/6 + 5/5 + audit 全绿（主控实测）

## 观察项延续（下轮复核）
后端：身份防线矩阵（第二十轮）/ O104-01 抖动基线 / O104-02 删除保护撞名 / B104-01 知识位盯守 / M87-01 窗口；前端：M-1 延续管理（第四十一轮）/ OBSERVE-104-01/104-02 修复回首轮（web/ 零漂移记录终结后首次代码提交的回归确认）/ OBSERVE-93-01 残余面（补录 3 个 refetch 按钮）/ 90-01 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **「同族同源」盲区是审查的反复教训**：F93-01 修了 Button 组件焦点环，TabsTrigger 渲染的同样是 button、同样被 global.css 抹掉原生 focus——却直到第十二轮无障碍纵深首查才暴露。**修复一个组件族时，要追「同族还有谁」（同一 base class 继承者 / 同一全局 reset 受影响者），而不是只修点**。
2. **知识位让历史误读现形**：B104-01 揭示 R39/R41 的「存在性≠同一性」泛化指控从未适用于手动路径——**持续的知识位固化（含活化条件）能让早年审查的误判被系统识别并纠偏**，这是矩阵复核价值的又一证明。
3. **无障碍基础项是低成本高收益的「人性化细节」**：TabsTrigger 焦点环一行 + 表格语义一行族——修复成本极低、收益是键盘用户与读屏用户的完整可访问性。审查「功能细节不够人性化」时无障碍应是一等公民。