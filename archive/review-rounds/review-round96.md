# review-round96 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**；前端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / 零新增 OBSERVE**（连续第四十二轮零严重级）。**双端零代码修改需求**——连续第四轮纯观察轮，核心产出：**connectex 连续零复现基线记录 + 身份防线矩阵第十一轮闭合 + 登录链路完整状态机新角度**。

## 审查发现（写入 archive/review-rounds/round96-{backend,frontend}-findings.md）

### 后端（OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O96-01 | OBSERVE | connectex 连续零复现基线记录——三轮全量 race 全 11 包全绿、api 包零 connectex（O95-01 基线连续第二轮零复现，R94 两轮 1-2 例低频残余已归档为宿主环境冷启动外溢） | ⚠️ 记录（CI `-p 1` + 失败重跑吸收口径维持） |
| O96-03 | OBSERVE | gofmt CRLF 噪音实证无误报（scheduler.go 工作区 CRLF vs git 版本 LF 合规零检出） | ⚠️ 记录（历轮 R90-01 同因延续） |
| O96-06 | OBSERVE | 契约 20 注释轮次标签回归检查——仅一处 docs/review-round13.md 文件路径引用非违规 | ⚠️ 记录（docs 归档文件名含轮次编号属叙述性引用、非代码注释违规） |
| 身份防线矩阵延续 | — | 16 项矩阵第十一轮逐点 grep（sameClientFor 六分支 + maybeRelogin 双侧 + ProbeForAccount 回写段 + MarkDone/RemoveDone/SubmitAll + Restore）无新裸露写点 | ✅ 闭合 |
| B88-01 持续复核 | — | round41 七分支测试族 + probe_identity_test.go 三钉定向 race 全绿 | ✅ 闭环 |
| O96-02/04/05 | OBSERVE | 资产体积 29.7MB 无新增 / 删除保护撞名 / logintest 分叉——均维持历轮观察 | ⚠️ 维持 |

### 前端（零新增 OBSERVE / M-1 第三十二轮闭合）
- **M-1 第三十二轮闭合**（四消费点逐字符一致、置位三路径 + 首帧边界完整、读点 9 处无第五消费处）+ 六防零回归 + **F93-01 第四轮复核**（Button.tsx:42 ring 在位、语义正确、全站收敛无回归）。
- 新视角五项全收敛：数据绑定与表单状态（受控输入全覆盖 + 提交前双层 clamp）/ 路由级状态持久化（会话/账号切换/登出三路视图复位收敛）/ 视觉一致性（token 色彩体系统一 audit 背书）/ aria 动态态（expanded/checked/pressed/valuenow 全随 state 同步）/ 性能边界（列表 key 全唯一稳定、memo YAGNI 成立）。
- **无障碍残留面备注新增 Progress aria-valuetext 项**并入 OBSERVE-93-01 线维持观察不落地。
- OBSERVE-90-01/88-01/85-02/84-01/83-01/77-02/76 族/O-3 族延续。

## 核实记录（关键）
- **O96-01**：三轮全量 race 全绿实测（审查代理）+ 历轮归档链（R94 低频残余 → R95/R96 连续零复现）——「宿主环境冷启动外溢」的定级被连续两轮实证支持，CI 口径维持。
- **O96-06**：契约 20 轮次标签回归——命中「review-round13.md」属 docs 归档文件名（叙述性、非代码注释），不违反契约 20（契约管生产代码注释）。无需动作。
- **登录链路状态机新角度**（后端代理）：管理员双条件签发 + loginTimingFlat 时延拉平 / LoginByPassword gateTryAcquire 非阻塞 + wasShell 失败清理 / 手动激活票据单次防重放 / MarkTokenValid 与 maybeRelogin 锁序 / ReloginIfNeeded 客户端自绑定——全部迁移闭环正确，主控认可走读证据。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 三轮全 11 包全绿（审查代理实测）
- `go build ./...` / `go vet ./...` 零输出 + 定向 race 回归全绿
- 前端 `tsc -b` EXIT 0 + `npm run build` 全量成功（1948 modules）+ target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 观察项延续（下轮复核）
后端：O96-01 抖动基线 / 身份防线矩阵（第十二轮）/ O96-02 体量 / O96-04 撞名 / O96-05 logintest / M87-01 窗口；前端：M-1 延续管理（第三十三轮）/ OBSERVE-90-01 日志区三态 / 88-01 / 85-02 / 84-01 / 83-01 + 77-02 + 76 族 + O-3 族全表续。

## 教训
1. **connectex 连续两轮零复现确认「宿主环境冷启动外溢」定级正确**：R94 低频残余 → R95/R96 连续零复现——「多轮实测 + 形态归因」的定级方法论再次验证：单轮样本既不能证明缺陷也不能证明消失，连续多轮才够。CI `-p 1` + 失败重跑口径持续吸收。
2. **契约 20 的「轮次标签零残留」要区分代码注释与文档引用**：O96-06 命中的 review-round13.md 属归档文件名（叙述性历史记录），非生产代码注释违规——**契约 20 管的是代码注释的「为什么/契约/陷阱」纯度，归档文件名带轮次编号是必要的可追溯性**，不重复立条。
3. **身份防线矩阵十轮零新裸露写点**：连续十一轮清单化复核，防线缺口类缺陷的发现面已被系统性收窄——矩阵作为持续防御结构将继续每轮复核直至 256 轮。