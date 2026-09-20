# review-round49 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（测试基础设施 flake 升级确认）/ OBSERVE 6、前端 MAJOR 0 / MINOR 0 / OBSERVE 3。修复 1 处（CI 重跑防御）；前端零修复（第六收敛轮；F48-M1 闭合）。

## 审查发现（写入 archive/review-rounds/round49-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / MINOR 0 / OBSERVE 6）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-49-01 | MAJOR（测试基础设施升级） | F48-M1 `-p 1` 未根除 httptest flake——4 轮全量 3 FAIL 全为 connectex 超时、失败单跑全绿；`-p` 档位只能缓解包间并发、根除不了"单包内部 test 级连接 churn"（Windows 回环 TIME_WAIT） | ✅ 修复（CI 失败自动重跑一次，25d779c） |
| OBSERVE-49-01 | OBSERVE | main 优雅退出无信号处理——SIGTERM 时 sched.Stop/sessions.Close 不执行；SQLite WAL crash-safe + 会话内存态无数据损坏，仅飞行链 success 可能未落库（重报幂等） | ⚠️ 留档观察（公网生产化时补 signal） |
| OBSERVE-49-02/03/04/06 | OBSERVE | chains map 无泄漏无溢出 / rateLimited 与 done 交叉正确（MarkDone 清 rateLimited）/ 闸门窗口翻转无计数竞态 / 大响应解析零值容错——确认无问题 | ⚠️ 留档观察 |
| OBSERVE-49-05 | OBSERVE | task_log 黄金期日志风暴量化（≈33 行/发布·10s）写侧无限增长无清理 | ⚠️ 留档观察 |
| F48-O3 复核 | — | config 家族整风闭合（TestAdminConfigSaveFailStillDispatch 断言 HTTP 500） | ✅ |
| 上轮观察 18 条 | — | 48-03 已闭合（F48-O3）/ 46-01 已闭合（F46-O1）/ 48-01 升级为 49-01 / 其余 14 延续 | ⚠️ 延续 |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O-1 | OBSERVE | 五件零消费残件（Dialog/Sheet/Table + react-select/react-switch）——跨轮积累仍未裁决 | ⚠️ 延续（下轮给终裁：迁移 or 删除） |
| O-2 | OBSERVE | N-2/N-3 + O-3~O-5（双按钮/401 保护窗口/焦点陷阱/Admin 轮询）延续 | ⚠️ 延续 |
| O-3 | OBSERVE | useTickingCountdown 整页重渲 + oxlint warning 集群非功能 | ⚠️ 延续 |
| F48-M1 复核 | — | Toast 定位闭合并逐文件核证（viewport className 落 ol、Swipe 交互不受 pointer-events-none 影响） | ✅ 闭合 |
| 新视角 | — | Dashboard key={account} 不对称（折叠状态跨账号残留纯 UX 灰尘）——宁缺毋滥不报 | ⚠️ 观察 |

## 修复（主控直修——改动小、测试基础设施层）
| 编号 | commit | 内容 |
|---|---|---|
| F49-M1 | `25d779c` | ci.yml go test 失败自动重跑一次（`|| go test -p 1 -v -count=1 ./...`）吸收瞬时 flake——不改业务代码、不牺牲确定性 |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0（审查代理实证）
- Flake 定位：失败测试单跑/子集复跑全绿（api 15.8s / zhidao 3.4s / 钉子集 5 测 / 身份族 7 测）
- `cd web && npx tsc -p tsconfig.app.json --noEmit` exit 0 + build 成功

## 观察项延续（下轮复核）
后端：孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / classFullRealtime 重复 / tick 无 recover / task_log 无清理 / 427 头 / B43-05 测试空档 / XUANKE_PORT 校验 / 优雅退出 / submitAll 300ms 拍；前端：O-1 残件终裁 / N-2~N-3 / O-2~O-5 / Dashboard key 不对称