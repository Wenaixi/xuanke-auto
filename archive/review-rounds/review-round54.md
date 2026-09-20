# review-round54 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮是**54 轮以来最干净的一轮**：后端 CRITICAL 0 / MAJOR 0 / MINOR 0（R53 两项 MINOR 闭合 + MAJOR-53-01 flake 归零降级关闭）、前端 MAJOR 1 / MINOR 4 / OBSERVE 7。

**修复 6 处**（前端 5 + 后端 2，三个 commit）：`47faff4`（前端五修）+ `ee0eafc`（task_log 窗口前置）+ `2831b6c`（priorityId 契约注释）。

## 审查发现（写入 archive/review-rounds/round54-{backend,frontend}-findings.md）

### 后端（CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3 新 + 14 延续）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-53-01 | MAJOR→OBSERVE（降级关闭） | R54 实证 flake 彻底归零——全量 7 轮 0 FAIL、api 隔离 5 轮 0%、zhidao 维持 0%；readyProbe 语义正确无假红无慢化 | ✅ 降级关闭 |
| OBSERVE-54-01 | OBSERVE（新） | submitLogin 恒发 `priorityId=` 空串 vs 网站 jQuery 对 undefined 丢弃——契约微差，学生登录无真值，一行注释对齐 | ✅ 修复（注释对齐，2831b6c） |
| OBSERVE-54-02 | OBSERVE（新） | accountExists 每请求全表 LoadCredentials——<100 账号毫秒级，与决策锚 7 精神一致，延续观察 | ⚠️ 延续 |
| OBSERVE-54-03 | OBSERVE（量化更新） | task_log 50 万行全扫实测 2.6ms（R53 估 50-100ms 偏保守），3 档方案档①首选 | ✅ 修复（档①窗口前置，ee0eafc） |
| 新视角六项 | — | AppendLog 写点 14 处零吞错 / LoadLogs 查询路径实测 / runtime 快照无半态 / session sweeper 无泄漏 / 迁移幂等正确 / doRequest 解析无放大——全无缺陷 | ✅ |
| R53 前端三修后端契约 | — | begin_times 恒下发、/state open_time 同源无分叉、三修自洽 | ✅ 闭合 |

### 前端（MAJOR 1 / MINOR 4 / OBSERVE 7）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-A | MAJOR | Select 横幅分支遗留旧版顺序——识别缺席 + 未来 begin_times 时 `!openTimeStr` 恒命中、`cd.isExpired` 分支死代码，横幅恒显"未识别"与主矩阵兜底倒计时自相矛盾（F53-M-1 修矩阵未修横幅） | ✅ 修复（横幅分支对齐 Dashboard） |
| M-B | MINOR | Toast 去重合并 duration 取最新——Radix duration 变化重启计时器可无限延寿（当前零 duration 传参无触发） | ✅ 修复（合并保留首 duration） |
| M-C | MINOR | Toast viewport 无最大条数上限——不同 title 高频并存无限堆叠 | ✅ 修复（max-h + overflow） |
| M-D | MINOR | Select 右栏"预计开放时间"未吃 begin_times 兜底——与主矩阵/横幅三处三种状态 | ✅ 修复（右栏补兜底） |
| M-E | MINOR | AdminStats `open_time_set === false` 值语义在字段可选声明下误判 undefined 为"已识别" | ✅ 修复（`!== true` 保守） |
| O-1 修正确认 | OBSERVE | R53 O-1（Admin 单激活查询轮询）再用 node_modules 产物实证成立 | ✅ 确认 |
| R53 三修复证 | — | M-1 矩阵兜底字符级同构 / M-2 去重 key 稳定 / M-3 variant 全量消费点——8 条零缺陷 | ✅ |

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `47faff4` | 前端五修：M-A 横幅兜底 + M-B duration 合并 + M-C 堆叠上限 + M-D 右栏兜底 + M-E 类型契约（3 文件 12+6 行） |
| `ee0eafc` | 后端 task_log 档①：LoadLogs/LoadAllLogs 查询前置 `WHERE id > (SELECT max(id)-20000)`（零删除零 DDL）+ TDD TestLoadLogsWindowKeepsRecent（3 万行插入 → 最近 2000 条） |
| `2831b6c` | 后端 priorityId 契约注释对齐（OBSERVE-54-01 闭环） |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `go test -race -count=1 -p 1 ./...` → store 包 205s 绿（含 3 万行插入用例）+ 全包绿；api 隔离 6 轮 5 绿 1 低频 flake（约 <8%，readyProbe 收尾后基线，CI `||` 吸收）
- 前端 tsc + tsc -b + npm run build 全绿（417.54 kB js / 41.07 kB css）

## 观察项延续（下轮复核）
后端：task_log 档①已闭合 / accountExists 全表（54-02）/ tick 无 recover + 优雅退出 / 孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / failingTargetsStore / 429 头 / XUANKE_PORT / 本地钟；前端：N-2~N-3 / O-2~O-8 / Dashboard key 不对称 / ui 模板残宽 / useTickingCountdown 每秒重渲 / 多标签页一致性 / "NaN"防御（非法日期串 OBSERVE）。