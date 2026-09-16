# 第 23 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读 opus 子代理并行产出发现，主 gate 逐条核实（读源码 + 读产品代码
> 推演真实触发路径），确认后端 3 项 MINOR（均核实为真并修复），前端 0 项新增真实缺陷。
> 修复走 TDD（先红灯后绿灯）后独立中文 commit。

## 后端（3 项，全部确认修复）

### B23-01（MINOR）实时人数复核"确证满员"分支不覆盖手动报名成功——同族竞态对称缺口
**缺陷**：spawnChain 非满员失败 → 实时复核（锁外，最长 15s）→ 锁内 `cErr==nil && full`
满员分支（scheduler.go:1331-1334）调 `markFullLocked` **无 doneHas 复核**——锁外复核窗口内
手动路径 `TryAcquireSubmit` 可抢到已释放的 inflight 位并 `MarkDone` 置 done+success，复核
返回真满后 `markFullLocked` 把 success 覆盖成 "failed/已满员" 并追加一条假"已满员"日志；
紧邻的"未现满员"分支（1338 有 `doneHas` 复核"绝不覆盖胜利状态"）有复核，两分支不对称。
`done==true` 保证不二次报名（1190 守卫），但 UI 上真实报成功的课恒显示"已满员·退避下一
备选"、日志多条假"已满员"，直到重设目标重建状态才恢复。
**触发场景**：自动链对课程 X SelectClass 网络抖动失败 → 复核窗口内用户手动报名成功 →
复核恰好返回真满 → success 被覆盖成 failed + 假日志。
**修复**：满员分支同样先查 `doneHas`，已置 done 即让位（与"未现满员"分支对称），绝不覆盖
胜利状态。**TDD**：`TestRealtimeFullRecheckKeepsManualSuccess` 修复前红（状态 failed 覆写
success，"该课程已满员，退避至下一备选"）→ 修复后绿；对偶守卫
`TestRealtimeFullRecheckWithNoManualDoneMarksFull` 无手动介入仍照常记 full。
**commit**：`9c0ecc6`

### B23-03（MINOR）token 已知失效且重登退避期时 spawnChain 每 tick 仍真实打平台
**缺陷**：token 失效 → 首链命中 ErrUnauthorized 触发 maybeRelogin → 重登失败后 `relogging`
被清（失败也清，才能再试）、`tokenValid[acct]` 保持失效、退避表 30s/60s… 递增。此后开窗
期每个 tick 的 submitAll 为该账号每门目标重新 spawnChain：链顶只有 `relogging` 短路，对
已知失效 token 没有任何跳过 → 每门课每 tick（平日 1s、黄金期 250ms）真实发起一次
SelectClass（必然 code=-1）并每题 AppendLog"教务令牌失效，自动重登中"（1244-1247）写库，
失效期间烧平台请求额度 + SQLite 日志表一个下午堆积数万条。`tokenValid` 置位后链上无短路
是"Token 失效走 maybeRelogin"这条已核验设计的实际缺口。
**修复**：链顶补 `tokenValidForLocked(acct)` 短路（tokenValid=true || relogging=true 即
"已知失效"，重登成功/手动登录后清 false 才恢复提交）；状态保持原样（前端 /state 只读
token_valid 显示"已失效·自动恢复中"）。**TDD**：`TestSpawnChainSkipsWhenTokenInvalid`
修复前红（调用 1 次）→ 修复后绿（0 次）。**commit**：`9c0ecc6`

### B23-02（MINOR）登录失败残留空 token 客户端抢占核心账号位
**缺陷**：`LoginByPassword` 先 `m.ensure(acct)` 直接写 clients+order 再 `c.Login`，失败
返回时空 token（无账密）客户端留在注册表首位——该账号成为 `AnyClient()`/
`AnyClientWithAccount()`（取 order[0]）与 B22-01 全局帧回退的载体：调度器 probe() 主体
900 行 FindElectives 用空 token 恒返回 ErrUnauthorized → 906-910 触发 maybeRelogin →
`ReloginIfNeeded` 报"未登录且无保存账密"、`reloginFail` 递增、`lastData` 永不刷新，未配置
目标的全校浏览视图持续报错直到该账号成功登录（MarkTokenValid 才清退避表）。
**触发场景**：部署者"首次登录即手滑输错密码"，无需恶意。
**修复**：登录失败即把残留空壳从注册表摘除（cleanup clients+order），下个 tick 不再作为
AnyClient 探测载体。**commit**：`45afce3`

## 前端（0 项）
本轮前端通道通读 Select 目标保存链路 / App 会话 401 / Login 激活 / Admin 五 Tab /
api client / UI 原语 / useTickingCountdown，并核对 Radix 受控语义 / react-query v5 轮询
组合 / 路由资源清理 / 类型层。三项疑似缺陷（F23-01/F23-02 activeTab 回退、F23-03 窗口
信号）主 gate 核实为**误报**：`tabs` 由 useMemo 从 publishes 推导，发布集合重建后必重算，
`t.open` 每轮随新数据刷新，主区按钮判断（Select.tsx:758）用实时字段
`t.in_date_range || stateData?.window_opened` 双信号合流，误判窗口 ≤1 个 /state 轮询周期
（2s）——结论：**无新增真实缺陷**。观察项：O-1 进页空 publishes 回显跳过（与 F19-01 契约
一致）、O-2 按钮显示条件窗口合流（≤2s 短暂误判，can_select/btn_type 平台同源下发）、
O-3 Admin 登出卸载无实质影响、O-4 Dialog/Sheet/Table 未实例化死代码（TSC 无报错）、
O-5 @radix-ui/react-select/react-switch 零引用死依赖、O-6 激活码字符串作 key 安全。

## 主 gate 核实结论
- **B23-01/B23-02/B23-03 均确认为真实缺陷**（读源码 + 推演真实触发路径，非猜测），且三者
  相互放大：B23-02 的空壳账号放大 B23-03 的失效链轰炸；B23-03 又使 B23-01 的复核窗口更
  常见。全部走 TDD 修复。
- 观察项沿用（24-01~24-04：WindowClosed 闩锁逐轮翻转、spawnChain 捕获后删除的单次调用、
  reloginResults 缓冲 8 非阻塞丢弃、PUT /api/admin/config 空变更先落库）——24-01 闩锁
  翻转有界自愈（EmptyProbeRuns≥3 后稳定挂起）、24-02 单次有界无写回、24-03 量级有界、
  24-04 幂等 UI 不发包。维持观察不修。
- 已核无缺陷：maybeRelogin/MarkTokenValid/TokenValidFor 锁序统一 reloginMu→s.mu；
  tick 提交守卫顺序与 B18-M1/B21-02 判据一致；submitAll 锁序无环；B19-03 对称触发重登
  覆盖完整；日志脱敏全链路无明文密码/完整 token。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全绿。
