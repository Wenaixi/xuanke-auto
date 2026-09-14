# 第 19 轮全模块安全审查与修复记录

> 覆盖：后端全部源码（main.go/api/scheduler/zhidao/accounts/session/runtime/store/db/config/secure/cmd/web-embed）+ 前端全部源码（App/client/types/Login/Dashboard/Select/Admin/components）+ 构建 CI。
> 审查方式：两个 opus 权威子代理并行只读审查（后端 + 前端独立通道，均附"宁缺毋滥 + 文件行号 + 触发场景 + 已知观察项去重"模板），发现全部经主通道逐一读码推演定案；修复独立 commit，全部 TDD。
> 本轮结论：**后端 3 项（B19-01 幽灵窗口兜底挂起 / B19-02 删账号全量清理 + 目标重设契约收敛 / B19-03 实时复核命中 token 失效未触发自动重登）**；**前端 3 项（F19-01 回显幽灵 publish_id 目标锁死 / F19-02 登录双 Enter 重复提交 / F19-03 激活码复制整链失效无兜底）**。

---

## 一、后端发现与修复状态（第 19 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **B19-01** | MAJOR | **幽灵窗口 2s 高频盯守烧平台**：`WindowClosed` 为 false（快照非空）时，`probeIntervalFor` 对"开放时间已过 + 从未开过窗"的空快照形态仍落入临门 2s 分支——平台窗口从未开启或已关闭且从未被探测确认（幽灵窗口），探测永续 2s 轰炸 findElectivesData（"访问过于频繁"熔断形态）。**修复**：`WindowClosed()` 新增兜底判定——`syncFailStreak>=3`（时钟同步连续失败 = 平台不可达信号）且 `openTimeNow()` 非零时视为幽灵窗口返回 true（tick 提交守卫挂起 + `probeIntervalFor` 降回 30s 常态）；新增 `syncFailedWindow` 字段写而不读（仅留档，判据读取 syncFailStreak），时钟成功即清。黄金期不受影响：开窗瞬间探测立刻脱离 2s 分支 |
| **B19-02** | MAJOR | **删账号后状态残留 + 重设目标误清持久历史**：`handleAdminDeleteAccount` 只调 `SetTargetsForAccount(acct, nil)`，删了 targets 却残留 acctTargets/done/full/rateLimited/inflight/refused/acctData/acctDataAt/tokenValid/reloginAt/reloginFail/relogging 内存态 + 状态.Courses 行（前端状态脏 + 重启后 refused 幽灵复活）；而 `SetTargetsForAccount` 原内部 `delete(s.done, acct)` 设计"重设目标清 stale done"，**与 RestoreDone 跨目标持久历史契约冲突**（重启恢复时目标被重设、已成功课程重新提交——`TestRestoreDoneSkipsResubmit` 实证回归红）。**修复**：新增 `PurgeAccount(acct)` 全量清理（含状态.Courses 过滤），删除路径改调它；`SetTargetsForAccount` 只清 refused，**绝不**清 done/full/rateLimited/inflight（done=跨目标持久历史、full/rateLimited=真实满员/风控退避、inflight=防并发双包），TDD：`TestSetTargetsPurgesStaleState` 断言四者全保留 + refused 清空；`TestPurgeAccount` 断言全清理 |
| **B19-03** | MAJOR | **实时人数复核命中 token 失效未触发自动重登**：`classFullRealtime`（IsClassFull → StudentCounts → findElectivesStudentCount，**学生数接口同样鉴权** code=-1）命中 `ErrUnauthorized` 时 cErr 被丢弃——错误被当普通失败处理、置 failed 后下个 tick 又重打已失效的报名接口（必然再失败），token 失效的恢复被延迟到探测/手动路径才发现（失效数秒内黄金期空转）。**修复**：与 SelectClass 分支（B8-M7）对称——`cErr != nil && errors.Is(cErr, zhidao.ErrUnauthorized)` 时 `maybeRelogin(acct)` + 置 failed"教务令牌失效，自动重登中" + 落日志。TDD：`TestRealtimeRecheckUnauthorizedTriggersRelogin` 先红后绿（夹具 `IsClassFull` 早退分支漏解锁曾致 Relogin 永久卡死，补显式解锁） |

## 二、前端发现与修复状态（第 19 轮）

| 编号 | 严重度 | 问题 | 决策/修复 |
|---|---|---|---|
| **F19-01** | MAJOR | **回显 effect 构建幽灵 publish_id 目标，目标被锁死改不了存不上**：`/state courses` 按旧 publish_id 下发（发布集合整体重建后/旧学期残留），回显 effect 照单全收构建 `initial`——带幽灵 publish_id 的条目进 selected 后，flushTargets/防抖保存的 `targetsUseCurrentPublishes` 校验必失败 → 一路置脏跳过（安全方向：绝不假清空）→ **目标锁死在读不出的旧条目上，用户永远改不了也存不上**。**修复**：回显即刻过滤，只用当前 publishes 集合内的 publish_id 构建 initial（与消费时刻校验同一判据），幽灵条目根本不进 selected。**注意**：effect 声明于 `const publishes` 之前（TDZ），用依赖中的 `data` 自行推导——重演 F18-01 构建中断陷阱，`tsc -b` 立即报 TS2448/TS2454 |
| **F19-02** | MINOR | **登录表单双 Enter/双击重复提交**：按钮 `disabled` 依赖 React 状态渲染落地有延迟，连按两次可在 disabled 生效前发出两个重复登录请求——教务多份并发登录互相挤掉会话（旧 token 失效），且验证码识别并发放大平台限流压力。**修复**：`submit` 入口先查 `loading` 在飞标记短路幂等 |
| **F19-03** | MINOR | **激活码复制整链失效无兜底**：`navigator.clipboard` 在非安全上下文（http:// 明文内网部署 / iframe 嵌入 / 权限被拒）整体不可用，抛错落入 catch 只提示"未授权剪贴板"——管理员复制激活码整链失效。**修复**：降级链——clipboard API 不可用 → 手动 textarea + `document.execCommand("copy")` 兜底（旧兼容同步路径）→ 仍失败才提示并把完整激活码展示给管理员抄录 |

## 三、修复细节（本轮后端 2 个 commit + 前端 3 个 commit，独立 commit）

- **B19-01 + B19-02**（commit 7ad2e2e）：幽灵窗口兜底挂起 + 删账号全量清理（PurgeAccount）
- **B19-03**（commit d740413）：实时复核命中 token 失效 → maybeRelogin（与 SelectClass 分支对称）
- **F19-01**（commit 0344468）：回显 effect 幽灵 publish_id 过滤（TDZ 陷阱：用 data 推导）
- **F19-02**（commit 7e420c8）：登录 submit 入口 loading 短路幂等
- **F19-03**（commit 980cf8e）：激活码复制 execCommand 兜底 + 失败展示完整码

## 四、回归证据（提交时点）

- `cd web && npm run build`（`tsc -b` + `vite build`）— 通过（408.06 kB / gzip 122.23 kB），F19-01 修复过程实证一次 TS2448/TS2454 红灯（TDZ）→ 用 data 推导后绿灯
- `go test -race -count=1 ./...` — 全绿（api / config / db / runtime / scheduler / secure / session / store / zhidao），`go vet ./...` clean
- B19-03 TDD：`TestRealtimeRecheckUnauthorizedTriggersRelogin` 先红（3s 超时，夹具 IsClassFull 早退漏解锁死锁）→ 补显式解锁 + 恢复分支后绿灯（0.07s）

---

## 提交索引（本轮后端 2 个 commit + 前端 3 个 commit）

```
7ad2e2e fix(backend): 第19轮B19-01+B19-02 幽灵窗口兜底挂起 + 删账号全量清理
d740413 fix(backend): 第19轮B19-03 实时人数复核命中token失效未触发自动重登
0344468 fix(web): 第19轮F19-01 选课大厅回显effect构建幽灵publish_id目标
7e420c8 fix(web): 第19轮F19-02 登录表单双Enter/双击重复提交
980cf8e fix(web): 第19轮F19-03 激活码复制整链失效无兜底
```

## 第 19 轮观察项（未修复，留档）

- **HTTP 探测单飞**（B14-I2/B16-M2 延续）：`ProbeForAccount`/`ProbeNow` 未接入 probing/lastProbe 节流——触发条件已精确化（30s 周期至多多 1 次），维持观察
- **Decrypt 死字段**（B16-I1 延续）：加密工具中未使用的解密函数字段，删除待定
- **RemoveFull 死方法**：手动退选解封后已无调用方，删除待定
- **probe per-account 错误静默**：`ProbeForAccount` 返回值被丢弃（`_, _ =`），频率受 30s 节流约束，维持观察

## 下轮待办

- 第 20 轮：发射两枚 opus 只读 agent（后端 + 前端），沿用同一模板（读全部模块、宁缺毋滥、已知观察项去重）
