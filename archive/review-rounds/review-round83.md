# review-round83 总结（2026-09-23）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实（失败测试 90+ 次复现、时序日志定位）+ 决策 → 修复 → 回归。本轮后端 **CRITICAL 0 / MAJOR 0 / MINOR 2 / OBSERVE 2**；前端 **MAJOR 0 / MINOR 0 / OBSERVE 1**。前端连续**二十九轮**零严重级；后端核心产出是 M83-01 的真实抖动测试修复（一个夹具时序缺陷被 90+ 次压测钉死后重构测试消除）。

## 审查发现（写入 archive/review-rounds/round83-{backend,frontend}-findings.md）

### 后端（MINOR 2 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M83-01 | MINOR | **TestWindowOpenSubmitsWithoutProbeReset 真实可复现偶发失败（90 余次 4 FAIL ~4-8%）**——测试 open 取过去时刻致 tick 提交守卫 `!opened && !now.After(open)` 恒放行，窗口未开时也提交，pending 断言被 connection reset 击败；-2m 版本二次探测仍抢先于 setAllOpened | ✅ **实修**（open 改未来 5 秒 + setAllOpened 后手动 resetProbe，10+ 连跑 + race 同族 3 轮全绿） |
| M83-02 | MINOR | api 全量 race 首跑一次 FAIL（60.4s）后续 10+ 轮高压全绿——R82 夹具共享全局态归因维持 | ⚠️ 维持观察 |
| O83-01 | OBSERVE | session 包 10 处 `New(time.Hour)` 缺 Close（范围小、无正确性影响，与 R82 api 夹具同型） | ⚠️ 维持观察不批量改（Ponytail：收益 < 改动成本） |
| O83-02 | OBSERVE | handler_test.go:152 注释保留 "O82-01" 编号引用（契约 20 许可边界） | ⚠️ 许可维持 |
| 复核全表 | — | R82 Cleanup 修复回归通过（一行 diff / LIFO 注册序安全 / Close 幂等）/ tray 平台分工无变化 / build tag 八文件互斥四组合编译全绿 / 12 项新角度契约抽核全过 / 契约 20 零标签 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 1）
**M-1 第十九轮闭合** + **保存链新视角五项全收敛**（防抖闭包代际正确 / lastJson 跨会话隔离 + 还原自愈闭环 / handleBack 21s 兜底超时路径不漏报——api 20s abort 保证失败落地先于兜底 / echo effect 轮询重跑短路零成本 / hasPublishes 发布重建按设计收敛）。R80/R81/R82 修复持续复核无回潮 + 契约 20 零命中（package-lock 内 R 双数字为 base64 校验和）、XSS/localStorage/audit 全绿。
- **OBSERVE-83-01**（"courses 非空 + publishes 空 + echoedRef 未置位"稳态组合为前瞻性陷阱——当前窗口关闭后无编辑入口、rev 恒 0 完全无害，仅未来开放"关闭后目标管理"入口时才需守卫解锁）⚠️ 按裁决维持观察，与 OBSERVE-77-02 关联留档。

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `4460dc3` | 后端：TestWindowOpenSubmitsWithoutProbeReset 夹具时序重构（M83-01）——open 改未来 5s + setAllOpened 后 resetProbe，消除 4-8% 偶发 |

**修复过程实证**：初版按审查建议改 -2m 仍复发（实测 6 次 1 FAIL）→ 系统化 Debug 捕获失败轮次二次探测日志 → 追查 submitAll/spawnChain/probe 链路确认根因是"提交守卫对过去 open 恒放行"而非探测分支 → 重构测试（未来 open + opened=false 挂起提交 + resetProbe 驱动探测）→ 10 连跑 + race 同族 3 轮全绿。审查"改 -2m"建议被实测证伪后修正为更根本的时序重构。

## 收尾全量回归
- 后端 `go test -race -count=1 -p 1 -timeout 900s ./...` → 待跑（审查代理多轮实证：除 M83-01 抖动外 11 包全绿）
- `go build` / `go vet` / `gofmt -l` 零输出
- 修复后测试 10 连跑 + race 5 测试族 3 轮 + 4 连跑全绿（主控实证）

## 观察项延续（下轮复核）
后端：M83-02 api 抖动（首跑偶发、高压全绿，四抖动测试族证据链延）+ O83-01 session 包 Close 观察 / O83-02 注释边界；前端：M-1 延续管理（第二十轮）/ OBSERVE-83-01 + 76/77/78 系列 + 66-03 全表续 / 可疑-1 物理不可达维持（本轮新视角推演再确认）。

## 教训
1. **审查的测试修复建议也要实测证伪再采纳**：M83-01 审查建议"改 -2m"——我改后仍复发（6 次 1 FAIL），系统化 Debug 才揪出真根因（提交守卫对过去 open 恒放行，比二次探测更根本）。审查给方向、实测定方案，两者缺一不可。
2. **抖动测试的根因要拆两次**：第一层表象（二次探测抢先）可能掩盖更深的判据问题（过去 open 使提交守卫失守）——R83 日志捕获+链路走查区分了"探测分支"与"提交守卫"两条路径，最终重构测试（未来 open + resetProbe）一次消除两个触发面。
3. **"修复成本 < 建议改动面"时 Ponytail 生效**：O83-01 建议改 10 个测试文件，但范围小（协程 ≤10）、生产中不能跑到、与已收口的 api 夹具同型——维持观察零代码更符合最小必要原则。
4. **前端六防保存链的"换方向"扫查模式值得延续**：5 项推演（闭包快照/lastJson 往返/21s 兜底/effect 开销/发布重建）全收敛无新缺陷，但每项推演都以"假设不成立→实测代码"收尾（如 21s vs 20s 超时窗口），是连续 29 轮零严重级的重要支撑。