# review-round42 总结（2026-09-20）

## 概述
按主控协议走完整循环：两个 opus 只读审查代理（后端/前端）并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 派两个 sonnet 修复代理串行 TDD 修复 → 收尾全量回归。发现后端 3 条 + 前端 9 条，确认修复后端 2 条 + 前端 3 条，1 条 MAJOR 降级观察，其余观察。本轮特色：后端发现"学生手动登录绕行 doLogin 频率闸门"（gateWait 只收口自动重登、LoginByPassword 旁路裸露），前端发现"防抖保存链死锁"（守卫置脏后 setSelected 同引用被 React bailout 永不重入）。

## 审查发现（写入 archive/review-rounds/round42-{backend,frontend}-findings.md）

### 后端 3 条（MAJOR 2 / MINOR 1）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| MAJOR-42-01 | MAJOR | 学生手动登录 LoginByPassword 完全绕行全局 doLogin 频率闸门（gateWait/gateLoginPerMin=2 只收口 Manager.Relogin 自动重登）——多账号失效排队重登时手动登录并发触达平台，出口 IP 可被刷进锁号窗口 | ✅ 修复（B42-01 非阻塞准入） |
| MAJOR-42-02 | MAJOR | Relogin/classFullRealtime 锁外持旧客户端指针与 Remove 无锁序化，删除后孤儿登录仍打平台 | ⚠️ 降级观察（Go 内存安全 + B21-03 挡写回侧，孤儿登录语义影响有限） |
| MINOR-42-01 | MINOR | spawnChain 失效分支先 maybeRelogin 后身份复核，幽灵账号残留 reloginFail 计数污染重建账号 | ✅ 修复（B42-02） |

### 前端 9 条（MAJOR 3 / MINOR 4 / OBSERVE 2）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M-1 | MAJOR | 防抖保存链死锁——守卫命中置脏后 setSelected 返回同引用被 React bailout，订阅永不重入，保存链永不恢复（黄金期改目标永不落库、无 toast、唯一解锁刷新） | ✅ 修复（F42-M1） |
| M-2 | MAJOR | /electives 轮询间隔依赖 /state window_closed，/state 失败期间失败态未降频 | ⚠️ 观察（"恒 2s"表述夸大，核心已被 F42-M3 修复） |
| M-3 | MAJOR | /electives 升频（window_opened ? 2000）被 st===undefined 吞掉，/state 失败即黄金期升频失效 | ✅ 修复（F42-M3） |
| N-1 | MINOR | F41-M1 独立清理 effect 依赖漏 selected，发布重建与 /state 首帧交错时清理落在回显合并之前 | ✅ 修复（F42-N1） |
| N-2 | MINOR | handleBack 全清空守卫误判，等待超时后合法清空永不落库 | ⚠️ 观察（影响极窄） |
| N-3 | MINOR | 报名按钮渲染绑 in_date_range||window_opened，窗口未开时 btn_type=2 也隐藏 | ⚠️ 观察（官网契约对齐争议） |
| N-4 | MINOR | 激活失败票据残留，重登携带旧票据死循环 | ⚠️ 观察（低概率） |
| O-1/2 | OBSERVE | resetRetry 清零退避 / useTickingCountdown 整页重渲染 | ⚠️ 观察（延续） |

## 修复（TDD 严格模式，独立 commit，未 push）

### 后端 2 条（修复代理 a7e9cc02b8880794c）
| 缺陷 | commit | 测试形态 | 红→绿 |
|---|---|---|---|
| B42-01 MAJOR | `e1394d0` | TestLoginByPasswordRejectsWhenGateBudgetExhausted + TestLoginByPasswordAllowedWhenGateBudgetAvailable | quota 满仍放行红→拒绝 0 doLogin 绿 |
| B42-02 MINOR | `a8f1856` | TestDeletedAccountRebuiltSameNameChainDropsRelogin（同名重建夹具） | relogCalls==1 红→0 绿 |

### 前端 3 条（修复代理 a59ba40c07d3e9728）
| 缺陷 | commit | 测试形态 |
|---|---|---|
| F42-M1 MAJOR | `3d6f357` | shouldDeferSave 纯函数 + target-guard-check.ts 3 断言红（is not a function）→绿 |
| F42-M3 MAJOR | `9e2cbab` | interval 回调 error→30000 降频，逻辑走查 + tsc |
| F42-N1 MINOR | `9e2cbab`（同批） | 依赖补 selected + cleanStaleSelected 幂等性论证 |

## 新增决策锚（已沉淀进根 CLAUDE.md）
33. **doLogin 全入口必须收口全局频率闸门**（B42-01）：gateWait 只收口 Manager.Relogin（自动重登），学生手动登录 LoginByPassword 是绕行旁路——多账号失效排队重登时手动登录并发触达平台刷爆出口 IP。修：新增 `gateTryAcquire()` **非阻塞准入**（与 gateWait 同一 gateMu 共享计数，先 Lock 再检查/消耗），LoginByPassword 的 c.Login 前调用，quota 满返回"登录尝试过于频繁"——**绝不用阻塞 gateWait**（会挂起用户登录响应数分钟）；管理员换绑同走收口；api 夹具批量注册撞闸门用 `ResetGateForTest()`（测试专用，正式代码不调用）。
34. **保存链守卫判据必须与数据源解耦**（F42-M1）：防抖守卫原判据 `!echoedRef.current && (stateDataRef undefined || courses>0)` 依赖 echoedRef——/state 持续失败期间命中置脏后 selected 无变化（setSelected 同引用被 React bailout）→ 订阅永不重入 → 保存链死锁至刷新。修：判据改纯数据（`stateData===undefined || courses 非空` → 推迟，抽 shouldDeferSave 纯函数），防抖 effect 依赖补 stateData（数据到达触发重跑自愈）——守卫只拦"回显确未完成"区间，已回显正常路径不受影响。

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0（后台验证）
- `cd backend && go test -race -count=1 ./...` → 9 包全绿（后台验证）
- `cd web && npx tsc -b && npm run build` → 前端代理验证全绿

## 观察项延续（下轮复核）
后端：MAJOR-42-02 孤儿登录（B21-03 挡写回侧维持）/ M40-01 本地钟混用 / m40-02 锁内多取 now / o40-01~04 / round39-41 延续全部；前端：M-2 降频覆盖 / N-2 全清空边界 / N-3 报名按钮 / N-4 票据残留 / O-1 resetRetry / O-2 整页重渲染
