# review-round46 总结（2026-09-20）

## 概述
R44/R45 双收敛后仍保持零功能级错误的第三轮收敛：两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（测试基础设施层，CI 假红源头）/ MINOR 0 / OBSERVE 5，前端 MAJOR 0 / MINOR 0 / OBSERVE 3。修复 4 处（前端 Dashboard 2 合 1 commit + 后端 scheduler clamp + CI -p2）。

## 审查发现（写入 archive/review-rounds/round46-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / MINOR 0 / OBSERVE 5）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-46-01 | MAJOR（测试基础设施） | go test ./... 并行 10 包偶发 httptest connectex 连接 flake——7 轮实证（5 个不同测试失败、单跑 3-5 轮全绿、无业务断言失败、无 -race 报告）坐实 Windows 宿主并行测试的端口/连接队列争抢，非业务缺陷，CI 假红源头 | ✅ 修复（CI -p 2 限并行包数，4daaff5） |
| OBSERVE-46-01 | OBSERVE | scheduler.interval 非正数无 clamp——NewTicker(0) 实证 panic，生产恒 300ms 不可达，与 tick 无 recover 同族防御缺口 | ✅ 修复（New 内 clamp + TestScheduleIntervalClamped，591827b） |
| OBSERVE-46-02 | OBSERVE | zhidao 不消费 Retry-After/429 头——平台实证仅 body 文案、30s 固定退避刻意保守 | ⚠️ 延续观察 |
| OBSERVE-46-03 | OBSERVE | task_log 无容量上限与清理——长期部署 DB 膨胀，历轮延续 | ⚠️ 延续观察 |
| OBSERVE-46-04 | OBSERVE | handleSetTargets 不校验 class_id 重复与跨年级 publish_id——管理员可信+平台最终把关的有意边界 | ⚠️ 延续观察 |
| OBSERVE-46-05 | OBSERVE | B43-05 的 500 分支测试空档——failingTargetsStore 已定义未接上（Register 收具体类型，接口化超范围） | ⚠️ 延续观察 |
| B45-N2/N3 + B45-N1 后端侧 + B43-01/02/04/05 + B44-01 | — | 全部复核正确闭合无回归；identity 三族防线全路径闭合 | ✅ |
| 上轮观察项 15 条 | — | OBSERVE-45-02 已被 B45-N3 闭合；其余 14 条（孤儿登录/双槽分叉/reloginResults 满丢弃/classFullRealtime 重复/tick 无 recover/access_limit_cookie 占位/死字段/时延等）全部延续 | ⚠️ 延续 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O-1（延续 14 轮） | OBSERVE | Dashboard 状态卡+手机悬浮栏只消费 window_opened 两态、窗口关闭后恒显"待命中"误导（数据层/Select 横幅/Admin 三态都已消费，唯独控制台主视图漏） | ✅ 修复（三态 window_closed 优先，2efcec1） |
| O-2（新） | OBSERVE | Dashboard /state 与 /logs 失败态不降频（与 Select 侧 F40-M3/F42-M3 不对称），网络波动期默认落地页 3s 轰炸 | ✅ 修复（error 首行短路 30s，同 2efcec1） |
| B45-N1 复核 | — | client.ts gate 三形态各单次广播、无遗漏形态——R45 MINOR 关闭 | ✅ |
| F43 四件套 + 新视角六角落 | — | 全绿无回归（Dashboard 时区/actionLoading 清理/Login 竞态/多账号 401 收敛/Radix 受控/定时器泄漏） | ✅ |

## 修复（主控直修，非修复代理——改动小或测试基础设施层）
| 编号 | commit | 内容 |
|---|---|---|
| F46-M1 | `4daaff5` | ci.yml 后端测试 `go test -p 2`——限并行包数压制 httptest 连接 flake |
| F46-O1 | `591827b` | scheduler.New interval<=0 clamp 300ms + TestScheduleIntervalClamped（跑绿） |
| F46-F1/F46-F2 | `2efcec1` | Dashboard 状态卡/悬浮栏三态 + 双查询失败态降频 |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `go test -run TestScheduleIntervalClamped ./internal/scheduler/` → 绿
- `cd web && npx tsc -p tsconfig.app.json --noEmit && npm run build` → 全绿
- 审查代理实证 go test -race 主体全绿（api/zhidao 偶发 flake 已归因 CI 修复）

## 观察项延续（下轮复核）
后端：孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / classFullRealtime 重复 / tick 无 recover / task_log 无清理 / 429 头不消费 / handleSetTargets 边界 / B43-05 测试空档 / access_limit_cookie 占位；前端：N-2~N-4（双按钮/动效/401 保护窗口）/ O-1~O-5（resetRetry/整帧重渲/焦点陷阱/保护窗口/Admin 懒加载）