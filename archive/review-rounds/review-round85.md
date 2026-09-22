# review-round85 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**；前端 **MAJOR 0 / MINOR 0 / OBSERVE 2**。前端连续**三十一轮**零严重级；后端连续多轮零 MINOR。**双端零代码修改需求**——核心产出是三条边界观察（api 抖动归因延续、托盘退出链路库演进风险、快照满员分支结构合理性）与前端两条低优先级观察。

## 审查发现（写入 archive/review-rounds/round85-{backend,frontend}-findings.md）

### 后端（OBSERVE 3，无 MINOR/MAJOR/CRITICAL）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O85-01 | OBSERVE | api 抖动归因维持——全仓 race 首跑复现 TestAdminElectiveSelectUnknownAccountRejects/TestLoginActivateSeparateBuckets connectex 冷启动，多轮连跑收敛零 FAIL、无 race 双绿、定向单跑恒绿，产品逻辑无涉 | ⚠️ 维持观察（CI `-p 1` + 失败重跑已吸收；可选优化 readyProbe 后补静默窗口） |
| O85-02 | OBSERVE | 托盘退出链路完整（systray v1.2.2 源码核查 WM_CLOSE→WM_DESTROY→PostQuitMessage→nativeLoop case 0→Run 返回→main 自然退出）但依赖库隐式消息顺序——main 无显式 Shutdown、无 signal.Notify 兜底，库更新可能致托盘退出变活挂 | ⚠️ 维持观察（可选加固：mQuit 分支先 srv.Close() 或 main 挂 signal.NotifyContext） |
| O85-03 | OBSERVE | spawnChain 快照满员分支无 sameClientFor——经论证安全：删除竞态窗口仅锁内微秒、PurgeAccount 清 full、同名重建不继承，结构合理防后续误报 | ⚠️ 记录（边界论证，防误报） |
| 复核全表 | — | R84 归档完整性（git show 3 文档零代码）/ build tag 10 文件矩阵 + 硬锚 4264/22 双源同值 / 契约 20 零标签 / 新角度 8 项抽核全过（登录限流桶独立/票据消费并发原子防超卖/LoadRefused 空库/SubmitAll 分组/Stop 幂等/退避封顶） | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 2）
**M-1 第二十一轮闭合** + 六防保存链零回归 + 新视角五项全收敛（横幅五态 hierarchy / Dashboard 折叠释放 / 撞名管理态 / CodesTab useMemo 落空 / Toast 去重 duration 保留）。
- **OBSERVE-85-01**（useTickingCountdown 注释声称"只重渲染倒计时一处"与实现不符——hook 在路由组件顶层调用，每秒 setNow 重渲染整棵组件树；纯注释口径分叉 + 非本轮引入预存在行为）⚠️ 维持观察（低优先级候选：修正三处注释或抽 memo 叶子组件）。
- **OBSERVE-85-02**（黄金期末尾平台清空与末次点选同落 400ms 防抖窗口的改动静默丢弃——极窄子秒级竞态，属"绝不假清空"安全方向刻意牺牲）⚠️ 维持观察。
- OBSERVE-77-01 归档确认复证；OBSERVE-84-01/83-01/76 系列延续。

## 修复（主控核实后直修）
无（双端零代码修改需求——三条后端 OBSERVE 均为边界论证/延续观察，前端两条低优先级候选维持）。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 全 11 包除 api 首跑偶发 connectex 外全绿（审查代理多轮实证，复跑收敛）
- `go build` / `go vet` / `gofmt -l` 零输出 + 四组合交叉编译全绿
- 前端 `npm run build` 绿 + 三组断言全绿（审查代理实证）

## 观察项延续（下轮复核）
后端：O85-01 api 抖动根除级处置候选（CI 零残余）/ O85-02 systray 退出链路主动加固选项 / 可疑 linux-CGO1 宿主限制；前端：M-1 延续管理（第二十二轮）/ OBSERVE-85-01/02 + 84-01 + 83-01 + 76/78 系列 + 66-03 全表续 / 可疑-1 物理不可达维持。

## 教训
1. **"经论证安全"的边界也要成文防误报**：O85-03 快照满员分支无 sameClientFor 初看像六分支缺口，但结构性差异（锁内即刻写 vs 网络后慢写）+ 删除竞态窗口微秒级 + PurgeAccount 清 full 三重论证证明安全——审查报告主动记录边界论证，后续轮次不会重复误报。
2. **第三方库隐式行为是演进风险点**：托盘退出依赖 systray WM_CLOSE→WM_DESTROY→PostQuitMessage 隐式消息顺序，main 无显式 Shutdown/signal.Notify 兜底——现状正确但库更新可能静默变活挂，OBSERVE 留档 + 可选加固候选（srv.Close 或 signal.NotifyContext）比立即改动更符合 Ponytail。
3. **注释声称与实现的分叉是持续审查主题**：OBSERVE-85-01（useTickingCountdown"局部渲染"注释 vs 顶层 hook 全树重渲染）与历轮注释口径修复同族——"注释先行、实现未跟上"的表述在低性能代价时降级观察，但维护误导成本真实。
4. **api 抖动已从"找根因"进入"持续收敛基线"阶段**：连续 6 轮（R80-R85）归因一致（夹具共享全局态 + Windows 回环冷启动）、CI `-p 1` + 重跑零回归——抖动不再是每轮审查焦点，转为观察延续项，轮次重心让给新契约角度扫查。