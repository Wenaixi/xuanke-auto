# round42 后端审查原始发现

> 审查基线：master @ 6298812（round41 全部修复已落盘；`cd backend && go build ./...` 与 `go vet ./...` 双通过实证 exit=0，只读无副作用）
> 范围：backend/ 下全部 Go 源码（main.go、cmd/probe、cmd/logintest、cmd/bench、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚（round41 已扩至约 32 条）+ legacy/website-source 逆向契约 + round39/40/41 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 锁序/竞态逐状态序列推演 + 对上轮观察项（M40-01、m40-02、o40-01~04、m39-02、o39-01~05）逐一核实 + 编译/静态检查实证。

---

## 一、新增发现（本轮首次报告）

### MAJOR-42-01（新增）handler.go:215-227 —— 管理员 ?account= 穿透 + 手动登录成功 = 击穿全局重登频率闸门，且与在途自动重登形成同账号双登录并发

- **文件:行号**：backend/internal/api/handler.go:215-227（`issueSession` → `Accounts.LoginByPassword`）；对照 accounts/manager.go:156-165（`Manager.Relogin` 的 gateWait 收口）与 scheduler.go:1187-1210（maybeRelogin 的发起决策）
- **严重级**：MAJOR
- **问题一句话**：全局重登频率闸门（`gateLoginPerMin=2`/分钟，安全审计 CRITICAL 2a 设计的防平台锁号防线）只在 `Manager.Relogin` 收口；而 `handleLogin`（学生登录）→ `Accounts.LoginByPassword` 是**完全绕行闸门**的第三条 doLogin 入口——注释明言"管理员入口不受全局重登闸门约束"，但**管理员只是账号名的子集**：任何学生账号经 `POST /api/login` 都是这条旁路。
- **触发场景推演**：N 账号部署 + 多账号同时 token 失效 → 调度器每账号 maybeRelogin 各自 goroutine 发起 `Relogin`（gateWait 排队中，预算 2/分钟）；与此同时某学生刷新页面重新登录（/api/login）→ `LoginByPassword` 直发 doLogin，与排队中的自动重登并发触达平台；平台"登录失败次数过多，请 30 分钟后重试"按 IP 计数时，N+1 并发齐射（含旁路）把出口 IP 刷进锁号窗口。此路径在本轮核实为新缺口——round40 M40-02 只修了"maybeRelogin 走 gate"的判定，`LoginByPassword` 的旁路从未纳入收敛面。
- **建议修法**：`LoginByPassword` 发起 doLogin 前同样经 `gateWait()`（对未激活登录的预检、激活后的正式登录同理，二者都会触达平台 doLogin）。

### MAJOR-42-02（新增）accounts/manager.go:156-165 + scheduler.go:1562 —— 锁外实时复核 `classFullRealtime` 直取 `ClientFor`，与删号后的 `Accounts.Remove` 无锁序化，陈旧客户端指针可在注册表删除后被继续使用

- **文件:行号**：backend/internal/accounts/manager.go:156-165（`Relogin`：`m.mu.Lock()` 取 `c` → 释放 → 锁外 `c.ReloginIfNeeded()`）；scheduler.go:1562（`classFullRealtime` 同构：`ClientFor` 取指针后锁外 `IsClassFull`）
- **严重级**：MAJOR
- **问题一句话**：`Manager.Relogin` 与 `Remove` 之间无锁序化——`Relogin` 锁内取出客户端指针、释放 `m.mu` 后锁外执行 `c.Login`（最坏 Vision 2 分钟），期间管理员 `Remove` 已把该客户端从注册表删除；删除后的客户端对象仍被在途 goroutine 持有并继续使用（Go 内存安全，但语义上是"已删账号继续在打平台 doLogin/报名接口"），与 B21-03 在同一窗口里、却只覆盖了"重登成功写回"一侧——**发起侧**仍裸露。
- **触发场景推演**：t0 账号 A token 失效 → maybeRelogin 发起 → `Manager.Relogin` 锁内取 A 客户端 → 释放锁进入 `Login`（网络+Vision 全程可达 2 分钟）；t1 管理员删 A → `Accounts.Remove`（注册表移除）+ PurgeAccount + DeleteAccount；t2 在途 `Login` 仍在执行（对象引用存活）→ 完成 doLogin 后 `SetCredentials` 写入旧客户端（无人消费的孤儿对象）+ 返回成功 → maybeRelogin goroutine 回锁 `ClientFor(A)` 复核不存在 → 静默放弃写回（B21-03 已挡）——但**平台侧已发生一次 A 的完整登录**：若 A 重新注册（重新登录），其 token 已在 t2 被在途登录刷新为最新，而新客户端拿的是旧 token → 直到下个失效检测周期前，新身份用旧 token 打接口（平台侧该 token 已被在途登录作废）→ 短暂失效抖动。同构窗口也存在于 `classFullRealtime`（scheduler.go:1562 锁外 `IsClassFull` 用旧指针）。
- **建议修法**：`Relogin`/`classFullRealtime` 锁外使用客户端前，捕获指针后补一次身份复核（与 `sameClientFor` 同族），或锁内拷贝出所需方法所需的最小数据（token/cookie 快照）。

### MINOR-42-01（新增）scheduler.go:1461-1462 —— 失效分支先 `maybeRelogin` 再回锁复核，`maybeRelogin` 在锁外发起时可能恰逢账号已删，触发"幽灵重登"

- **文件:行号**：backend/internal/scheduler/scheduler.go:1461-1462（`if errors.Is(err, zhidao.ErrUnauthorized) { s.maybeRelogin(acct) ... }`）
- **严重级**：MINOR
- **问题一句话**：spawnChain 失效分支对 `ErrUnauthorized` 无条件调 `maybeRelogin(acct)`——此调用在锁外且不校验客户端身份，账号在 SelectClass 往返期间被删后，本分支仍会为"已删账号"发起一次完整的自动重登（`Manager.Relogin` → `gateWait` → `Login`），平台侧白白消耗一次 doLogin 预算（gateLoginPerMin=2 的稀缺配额被幽灵账号吃一份）。
- **触发场景推演**：t0 账号 A 提交链 SelectClass 在飞；t1 管理员删 A（Remove+PurgeAccount）；t2 旧链返回 ErrUnauthorized → 本分支 `maybeRelogin(A)` 进入 `Manager.Relogin` → `ClientFor(A)` 已不存在 → 返回"账号 A 未注册"错误并递增 reloginFail 计数——`reloginFail[A]` 写入已删账号的 map（PurgeAccount 之后的全新写入，重建账号后残留失败计数）；同时平台 doLogin 预算未被消耗但流程已走完（`ReloginIfNeeded` 之前 `ClientFor` 已挡）。残留的 `reloginFail[A]` 会让重建账号的首次重登退避 30s（需 30s 才能清掉）。
- **建议修法**：失效分支在 `maybeRelogin` 前先 `sameClientFor(acct, chainClient)` 复核（与成功/风控分支同族，一行）。

---

## 二、已观察到、经核实维持观察/低风险（与上轮结论一致，简述）

- **M40-01**（scheduler.go:1199/1206/1210）：maybeRelogin 退避/节流仍用本地钟 `time.Since` 写读同基（`time.Now()` 写、`time.Since` 读），内部自洽；与 m39-02 同族，偏差 ≤640ms 对 30s 节流无实质危害，维持观察。
- **m40-02**（scheduler.go:1420）：spawnChain 每门课锁内取 `now`，亚毫秒统计口径噪音，读写对称正确，维持观察。
- **o40-01 ~ o40-04**：未激活不发会话无残留 / reloginResults 满丢弃为刻意权衡 / columnExists 拼接全编译期常量 / interval≤0 生产恒 300ms——均核实维持观察。
- **m39-02**：`ElectivesSnapshotFor`（778/793/799）、`ElectivesSnapshot`（870）、`CheckClassSelectable`（1829）5 处快照 TTL 读侧仍 `time.Since` 本地钟；写侧对齐钟偏差 ≤640ms vs 40s TTL，低危害维持观察。
- **o39-02**（submitAll 快照与在飞链竞态）：用户移除目标后一直在飞链仍按旧快照提交，无撤销语义，维持观察。
- **o39-03**（vision key 空串无法清空）：PUT /api/admin/config 对空格 key 保留旧值，维持观察。
- **o39-04**（NAT 出口登录限流合并）：`XUANKE_TRUSTED_PROXY=off` 时全校共享单桶，文档化权衡，维持观察。
- **o39-05**（tick 主循环无 recover）：调度器 goroutine 无 panic 恢复，维持观察（round41 未引入新 panic 点）。

---

## 三、已核对无问题的重点区域（本轮逐项复核）

- **B39-01 + B41-01 指针身份复核全分支闭合**：`sameClientFor`（reflect 指针身份）在成功（1494）/失效（1469）/风控退避（1524）/窗口关闭（1544）/实时复核满员（1604）全部生效；grep 实证 `markRateLimitedLocked`/`markFullLocked` 的五个调用点全部前置身份复核；round41 三个 TDD 测试在库。
- **B41-02 零值守卫已修复**：tick 零值守卫现为 `if open.IsZero() && !opened { return }`（1000），WindowOpened=true 时放行提交；`TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime` 在库；`TestSubmitSuspendedWhenOpenTimeCleared`（WindowOpened=false 夹具）仍绿。
- **B40-01 已修复**：`requireJSONBody` 拒绝分支现写真实 HTTP 403（router.go:72），与登录/激活 CSRF 门同款。
- **B40-02 已修复**：config.go 已无 OpenTime 字段与硬编码 2026 日期；`openTimeForLocked` 回退遗留字段生产恒零值。
- **零吞错落库点**：grep 实证全仓库无 `_ = s.store.*` / `_ = d.Store.*`；`MarkDone`/`RemoveDone` 的 `_ = d.Sched.*`（handler.go:355/419）为调度器方法返回值（方法内部日志，非落库吞错），不属于契约违反。
- **删账号 memory-first 四步序**：Remove → PurgeAccount → DeleteAccount → RevokeAccount 时序正确；`?account=` 四路凭据表校验（目标写/课程读/手动报名退选/状态读）全覆盖。
- **WindowClosed 三判据单源** `windowClosedLocked()`：主判据（+10s 裕量）/时钟失败≥3（带"开放时间已过"）/幽灵窗口 EmptyProbeRuns≥3（+10s 裕量）——StateForAccount 与 WindowClosed() 共用，open 单快照复用；`syncFailStreak` 失败在锁内 ++ 不回零、成功清零（B21-01 正确）。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow（B6-04）；probing 单飞锁内置位-检查；probeSem cap 4（F17-01）；tick 首探豁免（978）对零值 open 恒 false 不误触发。
- **会话/票据**：32 字节 crypto/rand、12h TTL 惰性失效 + 5min 清扫、ticket 绑定账号 + 单次防重放、RevokeAccount 覆盖删号吊销；`ConsumeActivationCode` 单条 UPDATE 原子防超卖 + 事务内查重防已激活双扣。
- **凭据 AES-256-GCM**：nonce 前置 hex、`.master_key`/env 32 字节校验（F17-04）、enc: 前缀、旧明文拒绝加载。
- **SQL 全面参数化**（modernc 占位符绑定）；`columnExists` 拼接为编译期常量；migrateAddPublishMeta 增量 ALTER + refuseLegacy 缺列清单与迁移列对应正确（refuseLegacy 在迁移之后调用）。
- **路由/中间件**：/api 前缀（含精确 /api）显式 404 不落 SPA；panic 500/会话 401/管理 403/限流 429 全部真实状态码（B39-02/B40-01 家族闭合）；XFF 仅回环+`XUANKE_TRUSTED_PROXY=on` 时信任最右非空值。
- **登录平**：管理员口令恒定时间比对 + 300ms 固定延迟拉平；`maskKey`/`maskedToken`/`tokenShort` 脱敏正确。
- **编译/静态检查**：`cd backend && go build ./...` 与 `go vet ./...` 双通过（本次实证 exit=0）。
