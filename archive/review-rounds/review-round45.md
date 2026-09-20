# review-round45 总结（2026-09-20）

## 概述
R44 后第二个收敛轮：两个 opus 只读审查代理并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 修复 → 回归。本轮双线零 MAJOR：后端 0 CRITICAL/0 MAJOR/0 MINOR + 2 观察级（均顺手收敛），前端 0 MAJOR/4 MINOR（1 新 + 3 延续）。修复 3 处（前端 1 + 后端 2），全为协议清洁与小健壮性。

## 审查发现（写入 archive/review-rounds/round45-{backend,frontend}-findings.md）

### 后端（0 CRITICAL / 0 MAJOR / 0 MINOR / 2 OBSERVE 新增）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| OBSERVE-45-01 | OBSERVE | access_limit_cookie 占位值 `***REMOVED***` 与真实下发值 1 不一致——token 权威通道是 URL 参数、cookie 仅冗余，无功能路径依赖 | ⚠️ 延续观察（同 Restore "1" 占位，客户端 SetCookies 合并语义） |
| OBSERVE-45-02 | OBSERVE | B44-01 加密失败分支无单测（manager_test 五测试均未注入失败 encrypt）——补回归钉 | ✅ 修复（B45-N3 测试） |
| 延伸观察 | OBSERVE | Restore 解密失败静默置空密码（manager.go:299-302）与 B44-01 留痕不对称 | ✅ 修复（B45-N2 补 log） |
| B44-01 + B43-01/02/04/05 复核 | — | else 分支不影响正常落库 / maybeRelogin 入口复核锁内先于全部 map 写入 / sameClientFor 闭环 / 撞名双条件 / stats firstErr 500 | ✅ 全部正确闭合无回归 |
| 新视角扫查（store/session/secure/runtime/zhidao/browser） | — | 单写者无读锁等待 + 事务 commit 错误如实返回 / 票据 TTL + ConsumeTicket 锁内四查无竞态 / maskKey len≤4 返 "****" 无 panic / Update 闭包 + Get 快照拷贝无半态 / ErrUnauthorized 仅 code=-1 / browser 构建标签互斥无死代码 | ✅ 全无新增问题 |

### 前端（MAJOR 0 / MINOR 4 / OBSERVE 5）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| N-1（新） | MINOR | client.ts 双重 401 广播——F41-N2 HTTP 层前置广播 + B39-02 writeJSONStatus 双 401 组合后同一响应连发两次 UNAUTHORIZED_EVENT（App 幂等无害，批量吊销事件风暴翻倍） | ✅ 修复（B45-N1 一行） |
| N-2/N-3/N-4（延续） | MINOR | 双按钮并存设计意图 / 动效语言不统一纯视觉 / 401 管理员保护窗口保守方向 | ⚠️ 延续观察 |
| O-1~O-4 | OBSERVE | resetRetry 退避语义 / 整帧重渲 / 焦点陷阱 / 保护窗口 | ⚠️ 延续 |
| O-5（新） | OBSERVE | Admin 五 Tab 无条件挂载，非激活 Tab 后台轮询常跑（5s×3+10s×1）——纯性能冗余 | ⚠️ 延续观察 |
| F43 四件套 + round42-44 | — | 全绿无继承性回归；TDD 16/16；tsc/build exit 0 | ✅ |

## 修复（主控直修，非修复代理——改动小、无需双代理）
| 编号 | commit | 内容 |
|---|---|---|
| B45-N1 | `51c8c64` | client.ts body 层 `if (j.code === 401)` 加 `if (r.status !== 401)` 前置——网关 HTML 401 / 旧式 HTTP 200+body 401 / writeJSONStatus 双 401 三种形态各单次广播 |
| B45-N2 | `cf121e0` | Restore 解密失败补 log.Printf（与 LoginByPassword 加密失败日志 B44-01 对称，零吞错） |
| B45-N3 | `8322f48` | TestLoginByPasswordEncryptFailLogs——假 encrypt 返回 error → 登录仍成功返回 token、fakeStore.saved 保持 false（凭据不落库） |

## 收尾全量回归
- `cd backend && go build ./... && go vet ./... && go test ./internal/{accounts,api,scheduler}/` → exit 0（三包绿）
- `cd web && npx tsc -p tsconfig.app.json --noEmit` → exit 0
- 审查代理实证 `go test -race -count=1 ./...` 10 包全绿

## 观察项延续（下轮复核）
后端：孤儿登录（MAJOR-42-02）/ 双槽分叉 / reloginResults cap8 满丢弃 / classFullRealtime 网络段重复 / tick 无 recover / access_limit_cookie 占位值 / 死字段 / 学生失败路径时延；前端：N-2~N-4 延续 / O-1~O-5（含 O-5 Admin Tab 懒加载）