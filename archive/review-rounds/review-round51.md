# review-round51 总结（2026-09-20）

## 概述
两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮后端 MAJOR 1（flake 本体延续）/ OBSERVE 1 新 + 延续 27，前端 MAJOR 0 / MINOR 0 / OBSERVE 2（含 **O-1 五件残件终裁**）。修复 1 处（前端终裁执行删 3 组件+3 依赖）；后端无修复（MAJOR-51-01 裁决延续观察）。

## 审查发现（写入 archive/review-rounds/round51-{backend,frontend}-findings.md）

### 后端（MAJOR 1 / OBSERVE 1 新 + 延续 27）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-51-01 | MAJOR（CI 基础设施延续） | httptest mock 连接 flake 本体未根除——F50-M2 已根治缓存羽化漏洞（闭合），但 -p 1 下 6 轮仍 2 度误红（connectex 同根、单跑全绿） | ⚠️ 延续观察（CI `\|\|` 重跑可吸收低频假红；根治侵入测试渗透，Ponytail 维持现状） |
| OBSERVE-51-01 | OBSERVE | flake 伪装业务断言新样本（handler_test.go:357 TestLoginUnactivatedNeedsCode 收到 connectex 业务文案）——同根佐证根治方向 | ⚠️ 延续观察 |
| 新视角六项 | — | acctData×窗口关闭 / handleState 关闭后 Courses 构造 / reloginFail 封顶 / targets priority 事务 / 空 body 首探 / SaveSettings 六键完整——全确认无问题 | ⚠️ 留档观察 |
| F50-M1/M2 复核 | — | stats body code 500 对齐 + CI -count 统一均闭合无回归；MINOR-50-01 已关闭 | ✅ |

### 前端（MAJOR 0 / MINOR 0 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| O-1（终裁） | OBSERVE（终裁执行项） | 五件零消费残件跨轮第 6 次复证——独立风险面评估支持删除（无隐藏消费点/peer deps 不裂/产物本就不含），精确清单补 `@radix-ui/react-dialog` 第三行 | ✅ 终裁执行（c2ed5b5） |
| O-2 | OBSERVE | N-2/N-3/O-3~O-6 全部维持原判 | ⚠️ 延续 |
| F50-M1 前端无感知 | — | client.ts 只读 body code、StatsTab 不消费失败码——body code 1→500 零影响 | ✅ |
| 新视角五项 | — | 空态不误触假清空守卫 / dateGroups 重建错组保守 / Login 双失效自愈 / accounts useMemo 稳定 / Admin 轮询延续——全无果不改逻辑 | ✅ |

## 修复（主控直修 + 终裁执行）
| 编号 | commit | 内容 |
|---|---|---|
| F51-O1 | `c2ed5b5` | 删除三件零消费组件（Dialog/Sheet/Table，+753 行净删）+ 三依赖移除（react-dialog/select/switch，react-dialog 为终裁补强项）；npm install 同步 lock；tsc+build 双绿（产物 CSS 略降） |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0（审查代理实证）
- 6 轮 `-race -count=1 -p 1`：3 轮全绿 + 2 度 api flake（connectex 同根、单跑四轮全绿）
- `cd web && npm install && npx tsc -p tsconfig.app.json --noEmit && npm run build` → 全绿（终裁后验证）

## 观察项延续（下轮复核）
后端：flake 本体 / task_log 无清理 / 孤儿登录 / tick 无 recover + 优雅退出 / 429 头 / B43-05 测试空档 / XUANKE_PORT / 双槽分叉 / reloginResults 满丢弃 / classFullRealtime 重复 / submitAll 300ms 拍 / chains map / etc；前端：N-2~N-3 / O-2~O-5 / Dashboard key 不对称 / O-1 已闭合