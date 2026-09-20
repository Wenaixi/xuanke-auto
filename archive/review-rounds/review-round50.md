# review-round50 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（CI 重跑缓存语义漏洞）/ MINOR 1（HTTP-500 家族 body code 开裂）/ OBSERVE 1 新，前端 MAJOR 0 / MINOR 0 / OBSERVE 2。修复 2 处（后端 body code 对齐 + CI -count 统一）；前端零修复（第七收敛轮）。

## 审查发现（写入 archive/review-rounds/round50-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / MINOR 1 / OBSERVE 1 新 + 延续 27）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-50-01 | MAJOR（CI 基础设施升级） | F49-M1 重跑防御缓存语义漏洞：重跑链两命令缓存键不同（首条无 -count=1 恒命中 (cached)），二次 CI 跑起重跑被羽化吸收不到 flake；且 -race -count=1 4 轮仍有 2 flake 本体未根除 | ✅ 修复（F50-M2 两命令统一 -count=1，a27e4a2） |
| MINOR-50-01 | MINOR | writeJSONStatus HTTP-500 家族 body code 开裂——F48-O3 落库失败 body=500 vs B43-05 stats 目标数失败 body=1，同写真实 HTTP 500 但 body 编码两套 | ✅ 修复（F50-M1 对齐 500，d7ac801） |
| OBSERVE-50-01 | OBSERVE | flake 形态伪装业务断言（撞名学生测试把 connectex 翻译成"管理口令错误"）——佐证登录链路首请求重试根治方向 | ⚠️ 延续观察 |
| 新视角六项 | — | acctTargets×done/full/refused 交叉 / courses 发布归属 / Token 旧值窗口 / LoadDone 恢复序 / SelectClass 重试幂等 / AdminNameValue 空值——全确认无问题 | ⚠️ 留档观察 |
| F48-O3 + 上轮实修 | — | config 家族整风闭合（B43-05 body code 残余见 MINOR-50-01）；F46-O1 clamp + B45/B43/B44 全复核无回归 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O-1 | OBSERVE | 五件零消费残件跨轮第 5 次复证——下轮建议终裁（删除 or 迁移二选一） | ⚠️ 延续（R51 终裁，倾向删除） |
| O-2 | OBSERVE | N-2/N-3 + O-2~O-6 延续（双按钮/401 保护窗口/焦点陷阱/Admin 轮询/整帧重渲） | ⚠️ 延续 |
| F48-M1 复核 | — | Toast 定位闭合并扩展核证（多 toast 堆叠顺序正确、每 toast 独立 duration 计时删除） | ✅ 闭合 |
| 新视角六项 | — | 过滤排序空态 / 日志低频加载 / ConfigTab 错误路径 / 激活跳转时序 / 401 代理态 / Toast 堆叠——全无问题 | ⚠️ 观察 |

## 修复（主控直修——改动小、CI/可观测性层）
| 编号 | commit | 内容 |
|---|---|---|
| F50-M1 | `d7ac801` | handler.go:908 stats 目标数失败 body code 1→500（HTTP-500 家族统一） |
| F50-M2 | `a27e4a2` | ci.yml 重跑链两命令统一 `-count=1`（修复缓存键不一致） |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `go test -count=1 -run "TestAdminStats" ./internal/api/` → 绿（3.3s）
- 审查代理实证：4 轮全量 `-race -count=1 -p 1` 3 绿 1 flake（api 2 测、单跑全绿）；前端 tsc+build 绿

## 观察项延续（下轮复核）
后端：孤儿登录 / 双槽分叉 / reloginResults 满丢弃 / classFullRealtime 重复 / tick 无 recover / task_log 无清理 / 429 头 / B43-05 500 测试空档（body code 已对齐）/ XUANKE_PORT 校验 / 优雅退出 / flake 伪装业务断言；前端：**O-1 五件残件终裁（R51）** / N-2~N-3 / O-2~O-6 / Dashboard key 不对称