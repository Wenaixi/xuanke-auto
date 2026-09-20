# review-round53 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（flake 残余收尾）/ MINOR 2 + OBSERVE 12+，前端 MAJOR 0 / MINOR 3 / OBSERVE 6（含 **O-1 重大认知修正**）。

**修复 8 处**（前端 3 + 后端 5，两个 commit）：`323d241`（前端三修）+ `22a61f5`（后端五修）。

## 审查发现（写入 archive/review-rounds/round53-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / MINOR 2 / OBSERVE 12+）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-53-01 | MAJOR（flaky 收尾，降级观察通过） | R52 三通道收敛后 flake 降至 1/3~1/2（zhidao 隔离归零、api 8.3%），残余 = 每测试独立 mock server accept 就绪前首请求连接拒绝 | ✅ 修复（readyProbe 夹具就绪探测收尾，10 轮全绿） |
| MINOR-53-01 | MINOR | httpDo 注释声称 WaitForState 预检但实现是失败后 cloneReq 重发——文档漂移 | ✅ 修复（注释对齐实现） |
| MINOR-53-02 | MINOR | cmd 三件套参数校验薄（probe token 未转义 / bench -n≤0 除零 / logintest -limit 越界 panic） | ✅ 修复（一行防御 ×3） |
| OBSERVE-53-01 | OBSERVE | task_log 无清理量化（黄金期 ≈5670 行/窗口、100 万行时全表扫描 50-100ms）3 档方案 | ⚠️ 延续观察（建议索引/上限清理） |
| 新视角六项 | — | httpDo×spawnChain 双报防线 / tick 持锁窗口 / 预热交互 / task_log 量化 / resp.Body 全量清点 / cmd 健壮性——全无缺陷或量化留档 | ✅ |
| F52-M1 后端侧契约 | — | 撞名学生签发普通会话响应无 adminName、管理员必带——端到端自洽无混淆通道 | ✅ 闭合 |

### 前端（MAJOR 0 / MINOR 3 / OBSERVE 6）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-1 | MINOR | Select 倒计时未同步 F39-N1 begin_times 兜底——识别缺席窗口内两页展示矛盾 | ✅ 修复（一行对齐） |
| M-2 | MINOR | 目标保存失败重试链红 toast 轰炸（46s 最多 6 个堆叠）无去重 | ✅ 修复（ToastProvider 按 title 去重合并 + duration 可配） |
| M-3 | MINOR | 管理端三处成功 toast 未用 success variant + duration 全站 3.5s 不可配 | ✅ 修复 |
| O-1【修正】 | OBSERVE【核心认知】 | R52 O-5"Admin 四查询同跑"被 Radix TabsContent 懒渲染实证推翻——实际单激活查询轮询，资源账降约 75% | ✅ 修正记录（无需修复） |
| O-2~O-8 | OBSERVE | 渲染收敛可接受 / Dashboard key 不对称 / 焦点陷阱 / N-2/N-3 / ui 模板残宽 / 多标签页边界 / 注释残留收窄 | ⚠️ 延续 |
| F52-M1 复证 | — | 撞名学生管理态修复 8 条全路径复核零缺陷 | ✅ |

## 修复（主控核实后直修）
| commit | 内容 |
|---|---|
| `323d241` | 前端 F53-M1/M2/M3：Select 倒计时兜底 + Toast 去重/duration + 管理端 success variant（3 文件 29+9 行） |
| `22a61f5` | 后端 F53-M1~M5：api readyProbe 夹具收尾 flake + httpDo 注释对齐 + cmd 三件套参数防御（5 文件 61+3 行） |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `go test -race -count=1 -p 1 ./...` → **全包全绿**（10 包全 ok）
- api 隔离 readyProbe 后 10 轮连跑全绿（修复前 12 轮 1 FAIL）
- 前端 tsc + tsc -b + npm run build 全绿（417.43 kB js / 41.04 kB css）

## 观察项延续（下轮复核）
后端：flake 残余（R53 收尾后 R54 复核是否彻底归零）/ task_log 无清理（3 档方案待落地）/ tick 无 recover + 优雅退出 / 孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / failingTargetsStore 无消费点 / 429 头 / XUANKE_PORT / access_limit_cookie 占位 / 本地钟混用；前端：N-2~N-3 / O-2~O-8 / Dashboard key 不对称 / ui 模板残宽 / useTickingCountdown 每秒重渲 / 多标签页一致性。