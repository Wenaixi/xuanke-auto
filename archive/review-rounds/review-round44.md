# review-round44 总结（2026-09-20）

## 概述
R43 全修后第一个"收敛轮"：两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮发现后端 MINOR 1（凭据加密失败补日志）、前端 MAJOR 0 / MINOR 3 / OBSERVE 4——**双线零功能级错误，R39-R44 连续修复的收敛信号**。

## 审查发现（写入 archive/review-rounds/round44-{backend,frontend}-findings.md）

### 后端（0 CRITICAL / 0 MAJOR / 1 MINOR）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MINOR-44-01 | MINOR | LoginByPassword 凭据加密失败静默跳过落库无日志——内存登录成功但重启 Restore 无账密、自动重登永久"无保存账密"，与决策锚 17 零吞错及 SaveCredential 内部 log.Printf 对称性有差距 | ✅ 修复（B44-01，一行 else log.Printf） |
| B43-01~05 复核 | — | maybeRelogin 入口复核锁内先于全部 map 写入、探测定时三处统一收口；实时复核失效分支 sameClientFor 闭环、chainClient 锁外段始终有效；撞名双条件后 requireAdminSession 判会话级 Admin 无残余错位；stats firstErr 500 达标 | ✅ 全部正确闭合无回归 |
| 观察项 14 条 | — | OBSERVE-43-05（双判冗余）被 B43-02 消解；其余 13 条（孤儿登录/双槽分叉/reloginResults 满丢弃/tick 无 recover/快照 TTL 本地钟等）全部延续 | ⚠️ 延续观察 |

### 前端（MAJOR 0 / MINOR 3 / OBSERVE 4）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| N-1 | MINOR | 开窗过渡期动效语言不统一（置灰按钮无 transition、双按钮切换无动画）——纯视觉一致性，行为零风险 | ⚠️ 延续观察（无需修） |
| N-2 | MINOR | 官网 btn_type 按钮 + 冲刺/预选按钮同卡双形态并存——双轨设计意图、两条持久化互不冲突、无数据分叉；仅视觉权重可选微调 | ⚠️ 延续观察（设计意图） |
| N-3 | MINOR | App 401 管理员保护窗口保守方向——端到端追查确认 lostAccount 判据经 loadSessions 反查，无数据风险 | ⚠️ 延续观察 |
| O-1~O-4 | OBSERVE | resetRetry 退避语义 / 整帧重渲 / 焦点陷阱 / 保护窗口 | ⚠️ 延续 |
| F43 四件套复核 | — | F43-M1 全清空放行主路径三处放行+rev===0 双层跳过 / F43-N1 btn_type 判据 / F43-N3 动效产物实证（@keyframes enter/exit 齐全）/ F43-N2 票据四清票点对称 | ✅ 全部落地正确无继承性回归 |

## 修复（主控直修，非修复代理——单行改动）
**B44-01**（MINOR，commit `4efa120`）：manager.go 凭据加密失败 `else { log.Printf("[accounts] 账号 %s 密码加密失败，凭据未落库（自动重登将无保存账密）: %v", acct, err) }`。理由：改动极小、无需测试基建、单行补日志不引入新行为，主控直接实现符合 TDD 轻量原则（契约锚 17 零吞错对称性修复）。build+vet 通过 + accounts/api/scheduler 三包 race 回归。

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `cd backend && go test -race -count=1 ./...` → 审查代理实证 10 包全绿 + 主控三包复验

## 观察项延续（下轮复核）
后端：孤儿登录平台侧副作用（MAJOR-42-02）/ 双槽分叉 / reloginResults cap8 满丢弃 / classFullRealtime 网络段重复 / tick 无 recover / 快照 TTL 本地钟 / 死字段 / 学生失败路径时延；前端：O-1 resetRetry / O-2 整帧重渲 / O-3 焦点陷阱 / O-4+N-3 保护窗口 / N-1 动效语言 / N-2 双按钮视觉权重