# review-round86 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → TDD 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 1 / MINOR 0 / OBSERVE 2**；前端 **MAJOR 0 / MINOR 0 / OBSERVE 1 新 + 1 评估**。后端 M86-01 为**实质缺陷级发现**（托盘「退出」活挂后台，R85 结论被推翻），前端 OBSERVE-85-01 注释口径分叉落地修复。连续第三十二轮前端零严重级。

## 审查发现（写入 archive/review-rounds/round86-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M86-01 | **MAJOR** | 托盘「退出」不会退出进程——systray.Run 在 goroutine 里、main 早已越过托盘等待阻塞在 ListenAndServe；只 systray.Quit() 让非 main goroutine 结束，main 永不返回，HTTP 服务活挂为无托盘无头驻留进程。R85-02「托盘退出即整体退出」结论不成立 | ✅ **采纳修复（B86-01）**：新增 quit_shared.go 统一退出出口 + main 注入 srv.Shutdown + 托盘注入 systray.Quit，mQuit 点击 → quitApplication() 先关服务再退图标，ListenAndServe 返回 ErrServerClosed 解锁 main 自然退出；tray_quit_test.go 三回归钉 TDD（先红后绿） |
| O86-01 | OBSERVE | api 首跑抖动本轮完全未复现（race 全绿含 api 36.2s），O85-01 的「readyProbe 后补 50-100ms 静默窗口」评估为不必要（夹具层无法已知首操作请求、必要性不足），CI 维持 `-p 1` + 失败重跑 | ⚠️ 维持现状（不实施静默窗口） |
| O86-02 | OBSERVE | spawnChain 快照满员分支无 sameClientFor 三重论证再复证——锁内即刻写/删除竞态仅微秒/PurgeAccount 清 full/同名重建不继承，无新污染通道 | ✅ 记录（复证通过） |

### 前端（OBSERVE 1 新 + 1 评估）
- **M-1 第二十二轮闭合** + 六防保存链零回归 + 新视角五组全收敛（react-query 查询键/失败路径 state 复位/卸载后 setState/无障碍基线/tabular-nums）。
- **OBSERVE-86-01**（新）：Admin CodesTab/ConfigTab 表单 label 无 htmlFor 程序化关联，纯可访问性弱项、零功能影响——⚠️ 维持观察。
- **OBSERVE-85-01 评估**：useTickingCountdown 注释口径分叉维持 OBSERVE 不升级——✅ 采纳零成本修复（F86-01）：三处注释改如实口径（每秒 setNow 触发宿主整树重渲染、React 语义不可绕过），拆 memo 叶子组件标为潜在优化非当前承诺。零行为变更。
- OBSERVE-85-02/84-01/83-01/77-02/76-01/76-02/76-03 延续；77-01 归档复证。

## 修复（主控核实后 TDD 修复）
1. **B86-01（后端 MAJOR）**：托盘「退出」先优雅关服务再退图标。`quit_shared.go`（无 build tag 共享注入点：shutdownServer/systrayQuit + setExitActions 判 nil 不覆盖 + quitApplication 顺序保证）+ main 注入 srv.Shutdown（5s 超时）+ tray_windows/tray_linux 注入 systray.Quit + mQuit 分支改调 quitApplication；main.go ListenAndServe 返回 ErrServerClosed 时优雅退出返回。**TDD**：tray_quit_test.go 三测试先红（未实现符号编译失败）→ 绿（Shutdown 后 Serve 返回 ErrServerClosed / 先 shutdown 后 systray 顺序 / 无托盘平台判 nil 安全跳过）。
2. **F86-01（前端 OBSERVE-85-01 落地）**：useTickingCountdown.ts / Select.tsx / Dashboard.tsx 三处注释如实口径。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → **全 11 包全绿**（api 17.5s 零抖动样本、store 36.9s、scheduler 15.2s，exit 0）
- `go build ./...` / `go vet ./...` / `gofmt -l .` 零输出 + 四平台交叉编译全绿（WIN CGO1/CGO0 + LINUX CGO0 + DARWIN）
- 前端 `npx tsc -b --pretty false` EXIT 0 + `npm run build` 全量成功（1948 modules / 419.54 kB JS / 41.11 kB CSS）+ 三组断言全绿（target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）+ audit.mjs 77 项

## 观察项延续（下轮复核）
后端：O86-01 api 抖动基线（CI `-p 1` 收口）延续；O86-02 快照满员分支边界记录；托盘退出加固后真机双击验证候选（B86-01 已 TDD 钉死，真机托盘点击 e2e 属手动验证项）。前端：M-1 延续管理（第二十三轮）/ OBSERVE-86-01 htmlFor / 85-02 / 84-01 / 83-01 + 77-02 + 76-01/02/03 全表续。

## 教训
1. **第三方库 goroutine 模型是审查盲区——R85-02 结论被 R86 实测推翻**：R85 走读断言「systray.Run 返回 → main 全部执行完 → 进程退出」，漏掉关键 Go 语义——main 早在 runTray 返回后就已越过托盘等待、parked 在 ListenAndServe；进程退出只在 main.main 返回或 os.Exit 时发生，非 main goroutine 全部结束不终止进程。**教训：涉及「退出/生命周期」的走读必须画出 main 协程与库 goroutine 的执行时序，进程退出判据只认 main.main 返回或 os.Exit**。R85 的 O85-02 观察（依赖库隐式消息顺序）方向对但严重度判低了——真正问题不是库演进风险，而是当前版本就已不退出。
2. **「点退出=退图标留服务」的产品语义缺陷值得 MAJOR**：无头驻留进程用户只能靠任务管理器强杀（图标已消失无法再操作），且后台服务继续监听端口——属功能缺陷非数据/安全问题，定级 MAJOR 恰当。
3. **退出链路修复要跨 build tag 全平台一致**：tray_windows/tray_linux 的 mQuit 分支都要改调 quitApplication（共享注入点保证顺序），占位平台（linux cgo0/darwin）判 nil 跳过。四平台交叉编译验证覆盖全部形态。
4. **「只重渲染一处」注释口径分叉落地成本极低**：OBSERVE-85-01 提级评估后选零成本修复（三处注释改如实口径），拆 memo 叶子组件留待性能触发——观察项到修复要评估「零成本修复 vs 大改」，前者优先。
5. **api 抖动从「需根除」到「维持现状」的裁决更新**：O86-01 本轮 race 全绿零样本，审查评估「readyProbe 后补静默窗口」不必要——连续多轮收敛 + 夹具层无法已知首操作请求，收益远小于复杂度，CI `-p 1` + 失败重跑已足够。抖动类观察项应定期用实测重新裁决「是否值得根除」。
