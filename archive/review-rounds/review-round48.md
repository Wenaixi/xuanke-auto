# review-round48 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（测试基础设施 flake 残余）/ OBSERVE 5、前端 MAJOR 0 / MINOR 1 / OBSERVE 2。修复 3 处（前端 Toast 定位根因 + 后端家族整风 + CI -p1）。本轮特色：**审查链自我纠错**——R47 把 Toast 定位机制记错（"viewport 自带 bottom-4 right-4"），R48 逐核 Radix 源码推翻并给铁证。

## 审查发现（写入 archive/review-rounds/round48-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / MINOR 0 / OBSERVE 5）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-48-01 | MAJOR（测试基础设施） | httptest 连接 flake 在 -p 2 下未根除（全量 4 轮 2 FAIL、唯一断言 context deadline、无业务失败）——CI 假红源头延续 | ✅ 修复（-p 1 单包串行，0b116a7） |
| OBSERVE-48-01 | OBSERVE | submitAll 黄金期 300ms 整数拍（≥250ms 节流）——架构既定语义恒定无相对劣势 | ⚠️ 留档观察 |
| OBSERVE-48-02 | OBSERVE | reloginResults 锁序确认无交叉（生产不持锁非阻塞、消费 select 内） | ⚠️ 留档观察 |
| OBSERVE-48-03 | OBSERVE | writeJSON 家族唯一偏差点 handkleAdminConfig 落库失败 HTTP 200+body 500 与 B43-05 不统一 | ✅ 修复（writeJSONStatus 500，e3efe83）+ 测试断言更新 |
| OBSERVE-48-04/05 | OBSERVE | ensure 并发无重复 order / SaveSettings 全量原子——确认无问题 | ⚠️ 留档观察 |
| 上轮观察 14 条 | — | OBSERVE-46-01 interval clamp 已被 F46-O1 闭合；其余 13 条延续 | ⚠️ 延续 |

### 前端（MAJOR 0 / MINOR 1 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-1 | MINOR | Toast 定位机制：R47 O-1 事实错记被推翻——viewport 无 inset 类、toast 全 portal 进 viewport、wrapper 空壳，真实定位依赖浏览器 static-position 行为 | ✅ 修复（eaa762f，布局并入 viewport + 删空壳） |
| O-1 | OBSERVE | 五件零消费残件（Dialog/Sheet/Table + react-select/react-switch 依赖） | ⚠️ 延续观察 |
| F46-F1/F2+B45-N1+F43 四件套+F42-M1 | — | 全部复证闭合无回归 | ✅ |

## 修复
| 编号 | commit | 内容 |
|---|---|---|
| F48-M1（前端） | `eaa762f` | Toast 定位类并入 Radix Viewport（bottom-4 right-4 z-50 布局）+ 删空壳 wrapper——根因修复（TDD 形态：tsc+build 绿；逻辑走查 static-position 不再依赖） |
| F48-O3（后端） | `e3efe83` | handleAdminConfig 落库失败 writeJSON 500 → writeJSONStatus 500（家族整风）；TestAdminConfigSaveFailStillDispatch 断言更新 HTTP 200→500 |
| F48-M1（CI） | `0b116a7` | ci.yml go test -p 2 → -p 1 单包串行（根除 flake 残余，确定性优先） |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `go test -count=1 -run "TestAdminConfig" ./internal/api/` → 绿（3.3s）
- `cd web && npx tsc -p tsconfig.app.json --noEmit && npm run build` → 全绿
- 审查代理实证：build/vet 双 pass、F46 钉子集 4 测全 PASS、race 连跑 flake 归因测试基础设施

## 观察项延续（下轮复核）
后端：孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / classFullRealtime 重复 / tick 无 recover / task_log 无清理 / 429 头 / access_limit_cookie 占位 / B43-05 测试空档 / XUANKE_PORT 校验 / submitAll 300ms 拍之留档；前端：N-2~N-3 / O-1 残件 / O-2~O-5 / Toast 已固化的后续 Regsee