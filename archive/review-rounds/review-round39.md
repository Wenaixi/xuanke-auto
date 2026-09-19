# review-round39 总结（2026-09-20）

## 概述
本轮按主控指令走完整协议：两个 opus 只读审查代理（后端/前端）并行审查全部模块 → 各自写入独立 findings md → 主控逐条深度核实 → 决策 → 派两个 sonnet 修复代理串行 TDD 修复 → 收尾全量回归。发现后端 10 条 + 前端 12 条，确认修复后端 5 条 + 前端 6 条，其余维持观察。

## 审查发现（写入 archive/review-rounds/round39-{backend,frontend}-findings.md）

### 后端 10 条（MAJOR 1 / MINOR 4 / OBSERVE 5）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| M39-01 | MAJOR | 删号防线 ClientFor 只查"账号名存在性"不查"客户端身份同一性"，删后同名重建被陈旧在飞链寄生 | ✅ 修复 |
| m39-01 | MINOR | writeJSON 永不写 HTTP 状态码，panic 恢复"500"实为 HTTP 200 | ✅ 修复 |
| m39-02 | MINOR | 快照 TTL 读侧 time.Since 混用本地钟（B20-03 只改写入侧） | ⚠️ 观察（偏差 ≤0.7s/40s TTL 无实质危害） |
| m39-03 | MINOR | 重登成功 UpdateIDToken 无 nil-store 守卫（全仓库唯一裸写） | ✅ 修复 |
| m39-04 | MINOR | emptyRunsFor 死代码（`_ = open` 假签名，无调用点） | ✅ 修复 |
| o39-01 | OBSERVE | open.IsZero() 守卫在 WindowOpened=true 时仍挂起提交 | ⚠️ 观察（B11-A1 刻意实现，识别槽空=平台异常） |
| o39-02 | OBSERVE | 在飞链快照旧目标竞态 | ⚠️ 观察（链生命周期短，done 语义有争议） |
| o39-03 | OBSERVE | 空串清空 vision_api_key 被静默忽略 | ⚠️ 观察（历轮裁决"清空无合法用途"） |
| o39-04 | OBSERVE | NAT 出口登录限流全校共享 | ⚠️ 观察（文档化权衡 + XFF 反代缓解） |
| o39-05 | OBSERVE | tick 主循环无 recover | ⚠️ 观察（防御性缺口，当前无 panic 路径） |

### 前端 12 条（CRITICAL 1 / MAJOR 2 / MINOR 6 / OBSERVE 3）
| 编号 | 级别 | 一句话 | 核实裁决 |
|---|---|---|---|
| C-1 | CRITICAL | 防抖/flush 守卫只校验产出集合，selected 旧 publish_id 被静默丢弃、产出非空放行 → 整包 PUT 覆盖删已保存目标 | ✅ 修复（F39-C1） |
| M-1 | MAJOR | 管理员删除自己后未退管理态，学生令牌渲染 Admin 连环 401 误删学生会话 | ✅ 修复（F39-M1） |
| M-2 | MAJOR | /state 失败时 refetchInterval 恒 2s 高频重试 | ⚠️ 观察（retry:1 + 20s 超时兜底，长时间断网才成立） |
| N-1 | MINOR | 主倒计时不吃 begin_times 兜底，注释承诺未实现 | ✅ 修复（F39-N1） |
| N-2 | MINOR | 回显合并重跑 effect 清空指数退避状态 | ⚠️ 观察（影响有限，拆分 effect 有新 bug 风险） |
| N-3 | MINOR | Admin StatsTab 无窗口关闭信号 | ✅ 修复（F39-N3 + 后端 B39-05 补字段） |
| N-4 | MINOR | "单次熔断冷却"文案与后端分级退避分叉 | ✅ 修复（F39-N4） |
| N-5 | MINOR | accounts 每次渲染新数组致 effect 每渲染重跑 | ✅ 修复（F39-N5） |
| N-6 | MINOR | priorityName 双文件副本 | ⚠️ 观察（纯函数稳定，低风险重复） |
| O-1/2/3 | OBSERVE | 多 Tab 会话互踢 / AbortError 文案 / 渲染期 ref 写 | ⚠️ 观察（O-3 有 F36-01 双保险） |

## 修复（TDD 严格模式，独立 commit，未 push）

### 后端 5 条（修复代理 a2a031dd029a4d844）
| 缺陷 | commit | 测试形态 | 红→绿 |
|---|---|---|---|
| B39-05 | `8489d07` | TestAdminStatsWindowOpenedUsesScheduler 追加 window_closed 断言 | 字段缺失红→绿 |
| B39-04 | `5211761` | 纯删除（grep 实证无调用点） | 编译过 |
| B39-03 | `f32bf8c`+`874cec3` | TestReloginSuccessWithNilStoreNoPanic | nil panic 红→race 绿 |
| B39-02 | `db04008` | TestRecoverMiddlewareHidesPanicDetail 追加 HTTP 500 断言 + 401/403/429 全覆盖 | panic 实际 200 红→绿 |
| B39-01 | `5376bb6` | TestDeletedAccountRebuiltSameNameChainDropsSuccess（新夹具 perAccount 映射） | 1 行 success 写回红→绿 |

### 前端 6 条（修复代理 ae871117461d61825）
| 缺陷 | commit | 测试形态 |
|---|---|---|
| F39-C1 | `11c865b` | 纯函数断言脚本 web/scripts/target-guard-check.ts（5 场景先红后绿） |
| F39-M1 | `9c50626` | 类型校验 + 渲染分支走查 |
| F39-N1 | `1fdb83c` | 类型校验 + 逻辑走查 |
| F39-N3 | `013b51d` | 类型校验（AdminStats 补 window_closed?） |
| F39-N4 | `3749fdf` | 纯文案 |
| F39-N5 | `c7627a4` | 类型校验 + build |

## 新增决策锚（已沉淀进根 CLAUDE.md）
1. **删号后同名重建防寄生 = 客户端指针身份比对**（B39-01）：spawnChain 成功/失效分支复核从"账号名存在性"升级为"发起提交 client 指针与注册表现指针同一性"（sameClientFor + clientIdentity 反射）；已删账号天然 false（B18-M2 保留）；重登分支无需（新 token 落库属新身份自身，B21-03 覆盖）
2. **writeJSONStatus 只改基础设施路径**（B39-02）：新增 writeJSONStatus 只改 recoverMiddleware 500 / requireAuth 401 / requireAdminSession 403 / 登录激活限流 429 + CSRF 403；writeJSON 本体与 100+ 业务调用点不动——前端契约只读 body code，HTTP 状态码为纯增强
3. **admin stats 补发 window_closed**（B39-05）：与学生端 /state 同源（三判据单源），前端 Admin StatsTab 三态闭环
4. **目标保存守卫必须拦"selected 残留旧 publish_id"部分覆盖**（F39-C1）：防抖/flush 消费时刻前置校验"非空 selected key ⊆ 当前 publishes"（selectedHasStalePublish 纯函数）——F15/F16/F17 链堵住"空集/漂移 id"形态，堵不住"旧 key 被静默丢弃、产出非空全合法"的部分覆盖
5. **删除管理员自己必须退管理态**（F39-M1）：onDeleted 补 setInAdmin(false) 与 logout/onUnauthorized 对称
6. **主倒计时吃 begin_times 兜底**（F39-N1）：识别缺席时 useTickingCountdown(openTimeStr ?? begin_times 格式化)，注释承诺落实

## 收尾全量回归
- `cd backend && go build ./... && go vet ./...` → exit 0
- `cd backend && go test -race -count=1 ./...` → 9 包全绿（accounts/api/config/db/runtime/scheduler/secure/session/store/zhidao）
- `cd web && npx tsc -b && npm run build` → exit 0

## 观察项延续（下轮复核）
后端：m39-02 读侧本地钟 / o39-01 零值守卫 WindowOpened 例外 / o39-02 在飞链快照 / o39-03 key 清空 / o39-04 NAT 限流 / o39-05 tick 无 recover；前端：M-2 /state 失败 2s 轮询 / N-2 退避清零 / N-6 priorityName / O-1~3
