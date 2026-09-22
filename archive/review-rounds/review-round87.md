# review-round87 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 1（走读推断）/ OBSERVE 2**；前端 **零 CRITICAL/MAJOR/MINOR/零新 OBSERVE**（连续第三十三轮零严重级）。核心：**B86-01 托盘退出修复闭环复核成立**（标准库源码实证），M87-01 尽力优雅语义注释落地。

## 审查发现（写入 archive/review-rounds/round87-{backend,frontend}-findings.md）

### 后端（MINOR 1 走读推断 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M87-01 | MINOR（走读推断） | 托盘退出 5s Shutdown 超时与在飞 spawnChain（SelectClass 最长 15s）的窗口临界竞态——超时前请求恰好返回时在飞链写 failed 分叉真实结果；超时后进程强杀则最后提交落库丢失（重启重抢兜底） | ✅ **记录 + 注释兜底**：不实现 sched.Stop 前置（引新竞态收益趋零），main.go 注释明确「尽力优雅语义——进程退出强杀在飞 goroutine，最后时刻提交以重启后重试为准」 |
| O87-01 | OBSERVE | B86-01 闭环成立——net/http 源码实证 Shutdown 只断 listener + 关 idle conn、不强杀在飞，5s 超时前在飞请求不被强杀；ListenAndServe 在 listener 关闭时刻即返回 ErrServerClosed（不依赖 5s 是否等完） | ✅ 记录（防后续误报 CRITICAL） |
| O87-02 | OBSERVE | systray.Quit 空回调 + nativeLoop 退出时序确认——B86-01 把 systray.Quit 放 srvShutdown 后避免「图标先没、服务活着」窗口，顺序正确 | ✅ 记录 |
| O86-01（延续） | OBSERVE | api 抖动零复现——race 全绿（api 22.15s）connectex 零样本 | ⚠️ 维持基线 |
| O86-02（延续） | OBSERVE | spawnChain 快照满员分支三重论证复证通过 | ✅ 记录 |

### 前端（OBSERVE 0 新 / 1 归档 / 1 提级评估维持）
- **M-1 第二十三轮闭合** + 六防保存链零回归 + 新视角五组全收敛（target 校正 effect 无陈显窗口 / 定时器无 visibilitychange 堆积 / Toast 并发合并 / 表单慢网络 loading / React key 稳定性）。
- **OBSERVE-85-01 归档**：F86-01 注释口径修正复核通过（git show 0b7a0ba 仅动 3 文件 13 增 7 删、三处注释逐字符如实口径、旧口径全仓零残留）——观察项移出延续清单。
- **OBSERVE-86-01 提级评估维持**：Admin 表单 label 无 htmlFor 纯可访问性弱项（隐式 label 包裹点击可聚焦、主流读屏隐式关联可用）、零功能影响，低于升级阈值。
- OBSERVE-85-02/84-01/83-01/77-02/76-01/76-02/76-03 延续；77-01 归档复证（R84/R85/R86 三连证）。

## 修复（主控核实后）
1. **M87-01（后端 MINOR 走读推断）注释落地**（94ce86a）：main.go 托盘退出注释补「尽力优雅语义」——进程退出强杀在飞 goroutine（含 spawnChain 网络往返），最后时刻提交结果以重启后重试为准（RestoreDone 只恢复已落库成功）；srv.Shutdown 只断 listener+idle conn 不强杀在飞，5s 超时后由 main 返回强制终止。**不实现 sched.Stop 前置**（决策：引新竞态收益趋零，见下「核实裁决」）。

## 核实记录（M87-01 深度核实）
- 走读 main.go:191-197（srvShutdown 5s 超时 + setExitActions）+ scheduler.go:693-695（Stop 即 cancel，只停 tick goroutine）+ scheduler.go:1436-1465（spawnChain 锁内分支序列）——确认窗口真实存在（黄金期冲刺 + 在飞报名 + 托盘退出同帧），但影响面限单课状态、触发概率低、重启恢复兜底。
- **决策：维持现状 + 注释明确「尽力优雅」，不实现 sched.Stop 前置**——net/http Shutdown 本身不强杀在飞、5s 超时后进程整体强杀已把最后时刻在飞链吸收；前置 Stop 反而新增「停止调度」与「在飞链继续写」的平行约束，需要 ctx 感知 + inflight 清位等待，收益（理论消除）远小于复杂性与新竞态引入面。托盘「退出程序」语义本意即终止，不承诺最后 5s 在飞链的落库原子性。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 11 包全绿**（api 22.15s / store 31.65s / scheduler 15.43s，exit 0）
- `go build ./...` / `go vet ./...` / `gofmt -l .` 零输出 + GOOS=windows CGO=1 交叉编译通过
- 前端 `tsc -b` / `tsc -b --force` EXIT 0 + `npm run build` 全量成功 + target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 + audit.mjs 77 项

## 观察项延续（下轮复核）
后端：M87-01 窗口留档（注释兜底，触发即重启重试）/ O87-01/02 记录 / O86-01 抖动基线；前端：M-1 延续管理（第二十四轮）/ OBSERVE-86-01 htmlFor（低优先级候选）/ 85-02 / 84-01 / 83-01 + 77-02 + 76-01/02/03 全表续；OBSERVE-85-01 归档退出。

## 教训
1. **「走读推断 MINOR」的处置分「要不要动代码」与「要不要留文档」**：M87-01 窗口真实存在但优先级低——采纳「注释明确尽力优雅语义」而拒绝 sched.Stop 前置（收益趋零 + 引新竞态 + 平行约束），注释兜底让未来轮次不必重复推断。
2. **标准库源码实证是跨出「第三方库黑盒」的可靠手段**：审查代理对照 net/http server.go 源码（trackListener/listenerGroup.Wait/closeIdleConns 幂等只关 idle）核实 Shutdown 不强杀在飞，O87-01 结论可防后续误报 CRITICAL——「库行为」类走读一律查源码不猜。
3. **连续 33 轮前端零严重级 = 观察项管理纪律累积**：前提复核归档（77-01 三连证）、零成本修复落地（85-01 F86-01）、提级评估维持（86-01）——观察项不堆积、不遗忘、定期归档。
4. **api 抖动连续两轮零复现（R86/R87 race 全绿 connectex 零样本）**：「零残余」目标持续被实测支持，抖动观察进入稳定基线——R80-R87 归因链（夹具共享全局态 + Windows 回环冷启动）未被任何新样本推翻。