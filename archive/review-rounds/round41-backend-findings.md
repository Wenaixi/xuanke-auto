# round41 后端审查原始发现

> 审查基线：master @ eb750d8（round40 全部修复已落盘；`cd backend && go build ./...` 与 `go vet ./...` 双通过实证）
> 范围：backend/ 下全部 Go 源码（main.go、cmd/、internal/ 各包），绝对只读模式，未修改/创建/删除任何文件。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-30（含 DB 迁移规范 / 部署规范）+ legacy/website-source 逆向契约 + round39/40 修复报告。
> 方法：全包（scheduler 1982 行 / api handler 1184 行 / zhidao client 707 行 / accounts / session / store / db / secure / config / runtime / main / cmd / web-embed 等）逐行通读 + 锁序/竞态逐状态序列推演 + 上轮观察项（M40-01、m40-02、o40-01~04、m39-02、o39-01~05）逐一核实。编译验证 exit=0。

---

## 一、新增发现（本轮首次报告）

### CRITICAL-41-01（新增）scheduler.go:1516 & 1530 —— 风控/窗口关闭两条分支的 map 写入经链表 `s.inflight[acct]` 的 map 指向同账号同名重建后的新 map，且**未做 B39-01 指针身份复核**，删号后同名重建的陈旧链拿新身份继续落库

- **文件:行号**：backend/internal/scheduler/scheduler.go:1516（`markRateLimitedLocked`）、1530（`markFullLocked`）；与之对称的**已复核**分支：1490（成功）、1465（失效）、1551（实时复核三路——其中 1582 的 `markFullLocked` 同样无身份复核，同一条）。
- **严重级**：CRITICAL
- **问题一句话**：B39-01 的"同一身份"防线只覆盖了 spawnChain 的成功分支（1490）与失效分支（1465），而 `isRateLimitError`（1516）与 `isWindowClosedError`（1530）两条分支在 `err != nil` 的归并路径上只做了 `delete(s.inflight[acct], classID)`（1482）就继续操作——账号被删后同名重建时，陈旧链命中风控/窗口关闭错误，`markFullLocked`/`markRateLimitedLocked` 会**无辅助判据**地把 full/rateLimited 状态写进重建身份。
- **触发场景推演**（与 TestDeletedAccountRebuiltSameNameChainDropsSuccess 完全同构，只是把"成功"换成"风控/已满员"错误）：
  1. t0：tick → submitAll 为账号 A 生成提交链，链捕获旧客户端 O（chainClient），`SelectClass` 进入 15s 网络往返。
  2. t1（网络在飞）：管理员删除 A → `Accounts.Remove(A)` + `PurgeAccount(A)`（清空 done/full/rateLimited/inflight 全族 map）→ `Store.DeleteAccount(A)`。
  3. t2：A 重新登录 → `ensure(A)` 注册新客户端 N；新会话下发、凭据落库。
  4. t3：旧链 `O.SelectClass` 返回错误，平台文案触发 `isRateLimitError`（真实形态：平台"访问过于频繁，请稍后重试"）或 `isWindowClosedError`。
  5. t4：旧链回锁，删了 inflight 后进入 1516/1530。`markFullLocked(acct, t)`（1530）→ `s.full["A"]` **对重建账号**记 true（PurgeAccount 已清，此写全新）→ `setStateLocked(s.statusIndexLocked(A, classID), "failed", "该课程已满员，退避至下一备选")` —— `statusIndexLocked` 此时在重建账号的 courses 里查不到（PurgeAccount 已清状态行）→ idx=-1，`setStateLocked` 静默跳过，但 **`s.full[A]` 的 map 写入已发生**。
  6. 结果：重建后的账号 A **从第一秒起对课程 x 记入 full（未满员却永久退避）**，黄金期自动抢课对该备选全程静默跳过；若 t5 时重建账号恰好也把 x 设为目标，`spawnChain` 的 `fullHas(acct, x)`（1427）会让自动链永久跳过——**陈旧链用新身份证实"已满员"**，与 B39-01 根治的"寄生死代码"（陈旧链写 success 假成功）同一竞态，只是反向（陈旧链写 full 假退避）。手动报名成功可解封（MarkDone 清 full），但若该课本就热门、用户没手动点，黄金期全程错失。
- **建议修法（一行）**：1516/1530 两分支在写 full/rateLimited 前补 `s.sameClientFor(acct, chainClient)` 复核（false → 静默放弃整链），与成功/失效分支同族；`markFullLocked`（1582 实时复核满员分支）同理。
- **注意**：B39-01 的修复测试 `TestDeletedAccountRebuiltSameNameChainDropsSuccess` 只覆盖"成功"分支 → 本缺陷是**已修复同类防御的不可达分支残余**。被删账号重启恢复由 B21-03/B18-M2 覆盖，但"同上同名重建"在 R39 已确认是最深竞态（有独立测试固化），此处两分支未被测试覆盖。

### MAJOR-41-02（新增）scheduler.go:993-998 —— tick 提交守卫零值 `Render 前 state.WindowOpened=true` 已确证开窗时仍挂起（B11-A1 过宽），而 `probe()` 的 `opened` 判定（1102-1108）恰恰是"发布级 InDateRange"，两处对"开窗"的定义在产品语义上分叉

- **文件:行号**：backend/internal/scheduler/scheduler.go:993-998（`if open.IsZero() { return }`）；1102-1108（probe 里 `opened := data.Publishes[i].InDateRange` 迭代）。
- **严重级**：MAJOR
- **问题一句话**：`tick()` 提交守卫在 `open.IsZero()` 时无条件 return，**先于** `!opened && !now.After(open)` 与 `WindowClosed()` 两条判据——即使 `s.state.WindowOpened==true`（probe 已由发布级 `inDateRange` 实证"平台窗口已开"），只要识别槽 `openTimeDetected` 一直为空（平台批次未下发非空 `beginTimes`），自动链仍被永久挂起，黄金期 250ms 冲刺 0 次。而 probe 的"开窗"判定用的是**发布级 inDateRange**（windowOpened 由 `p.InDateRange` 置 true），tick 的"该不该提交"却只看**开窗时刻 token**，两个开窗判据的产品语义分叉——窗口确证开启（发布级）却拿不到"开启时刻"（顶层级）时全盘停摆。
- **触发场景推演**：新一批选课发布，平台顶层响应 `beginTimes` 缺省/为空（F7-01 记录的过渡态/接口裁剪），但 `selectElectivesData` 非空且 `inDateRange=true` → probe 把 `WindowOpened` 置 true（1102-1108 走通）→ 识别槽 `openTimeDetected` 恒空（probe 只在 1091 `len(data.BeginTimes)>0` 才写槽）→ 每 tick（300ms）`open.IsZero()` 在 993 直接 return → **黄金期提交 0 次**，`opened` 与 `WindowClosed` 判据（都在其后）永不执行，直到窗口关闭阶段才可能由降频修正。这是 o39-01 已观察但 R39/R40 均维持"B11-A1 刻意语义"的同一缺口——但本轮读到 `WindowOpened` 的置位证据确实独立于识别槽存在（1102-1108 纯发布级），且 `StateForAccount` 也据此对前端报 `window_opened=true` 而 `open_time_known=false`，前端展示"窗口已开"而引擎从未提交。
- **建议修法**：零值守卫改为「`open.IsZero() && !s.state.WindowOpened`」二者同时成立才挂起（窗口确证开启后不再依赖开窗时刻）；或把"提交放行"定义统一为 `opened || now.After(open)`。同为一行，同时消除 `WindowOpened=true + open 零值` 的语义矛盾。
- **与已写入决策锚的关系**：B11-A1 的本意是"open 零值防 `!now.After(open)` 恒放行轰炸平台"。但该防轰炸目标是**未开窗**场景——`WindowOpened=true` 时轰炸目标（未开窗期）已消失，守卫过宽误伤黄金期。既有测试 `TestSubmitSuspendedWhenOpenTimeCleared`（1333）断言的是"清空开窗点后 tick 不调 SelectClass"，其夹具 `time.Now().Add(-time.Hour)` 且 WindowOpened 默认 false；补 `&& !s.state.WindowOpened` 后该测试仍绿（其窗口从未开过），不会破坏既有语义。

---

## 二、已观察到、经核实维持观察/低风险（与上轮结论一致，简述）

- **M40-01**（scheduler.go:1195/1202/1206/1236）：maybeRelogin 退避/节流仍用本地钟 `time.Since`——**读写同基内部自洽**，与 m39-02 同族（读侧本地钟 vs 写侧对齐钟），时钟对齐误差 ~640ms 对 30s 节流无实质危害；维持观察，不升级。
- **m40-02**（scheduler.go:1416）：spawnChain 锁内多次取 `now` 仅统计口径亚毫秒噪音，读写对称正确（B16-M1 已落），维持观察。
- **o40-01 ~ o40-04**：未激活不发会话无残留 / reloginResults channel 满丢弃为刻意权衡 / columnExists 拼接全编译期常量 / interval≤0 生产恒 300ms——均核实维持观察。
- **m39-02**：`ElectivesSnapshotFor`（778/793/799）、`ElectivesSnapshot`（870）、`CheckClassSelectable`（1803）5 处快照 TTL 读侧仍 `time.Since` 本地钟；与写侧对齐钟偏差 ≤640ms vs 40s TTL，低危害维持观察。
- **o39-02**（submitAll 快照与在飞链竞态）：用户移除目标后一直在飞链仍按旧快照提交——`SetTargetsForAccount` 与 in-flight 链无联动（无撤销语义）；黄金期窗口下用户撤销目标的概率与窗口都极窄，行为可辩护（平台最终把关 + done 按契约不清），维持观察。
- **o39-03**（vision key 空串无法清空）：PUT /api/admin/config 对空格 key 保留旧值无清除语义，维持观察。
- **o39-04**（NAT 出口登录限流合并）：`XUANKE_TRUSTED_PROXY=off` 时全校共享 5 次/分钟单桶，文档化权衡，维持观察。
- **o39-05**（tick 主循环无 recover）：调度器 goroutine 无 panic 恢复，仅"未来回归"防御性缺口，维持观察。

---

## 三、已核对无问题的重点区域（本轮逐项复核）

- **零吞错落库点**：grep 实证全部 `s.store.*`/`d.Store.*` 落库调用均 `if err != nil { log.Printf }`，无 `_ =` 吞错（含 scheduler/SetTargetsForAccount/DeleteRefused、MarkDone、RemoveDone、重登 UpdateIDToken、api 全日志点）。
- **B39-01 成功/失效分支已闭合**：`sameClientFor`（reflect 指针身份）在成功分支 1490 与失效分支 1465 生效，测试 `TestDeletedAccountRebuiltSameNameChainDropsSuccess` 在库；实时复核回锁三路（1551）的"账号存在性"复核逐路正确。
- **会话/票据**：32 字节 crypto/rand（熵源故障 panic，拒绝可预测令牌）、12h TTL 惰性失效 + 5min 清扫、ticket 绑定账号 + 单次防重放（ConsumeTicket 在码校验前销毁为刻意设计 F13-m1）、RevokeAccount 覆盖删号吊销。
- **激活码原子性**：`used_uses < total_uses` 单条 UPDATE 原子防超卖、事务内查重防已激活双扣、批量生成单事务、64bit 熵。
- **凭据 AES-256-GCM**：nonce 前置 hex、`.master_key`/env 32 字节校验（F17-04）、enc: 前缀、旧明文拒绝加载。
- **锁纪律**：`s.mu` 覆盖全部状态写入；`reloginMu → s.mu` 锁序全链一致（maybeRelogin/TokenValidFor/MarkTokenValid 三处同序，无反向路径，无重入）。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow；probing 置位-检查锁内原子；probeSem cap 4（F17-01）封顶 per-account 并发。
- **WindowClosed 三判据单源** `windowClosedLocked()`：主判据（+10s 裕量）/时钟失败≥3（带"开放时间已过"）/幽灵窗口 EmptyProbeRuns≥3（+10s 裕量），StateForAccount 与 WindowClosed() 共用，open 单快照复用。
- **删账号 memory-first 四步序**：Remove → PurgeAccount → DeleteAccount → RevokeAccount 时序正确；`?account=` 四路凭据表校验（目标写/课程读/手动报名退选/状态读）全覆盖。
- **计时器/goroutine**：sweepLoop 优雅退出（Close 等待 done）；Start 的 ticker Stop；maybePrewarm/maybeSyncClock 异步 goroutine 均无泄漏路径（probeSem defer 释放）。
- **SQL 全参数化**（modernc 占位符绑定）；数据库迁移 migrateAddPublishMeta 增量 ALTER + refuseLegacy 缺列清单与迁移列对应正确（refuseLegacy 在迁移之后调用，缺列判定不含已迁移列）。
- **路由/中间件**：/api 前缀（含精确 /api）显式 404 不落 SPA；CSRF 缓解（jsonContentType、DELETE 放行空 body 语义正确）；XFF 仅在回环+`XUANKE_TRUSTED_PROXY=on` 时信任；`requireJSONBody` 已写真实 HTTP 403（B40-01）；panic 500/401/429 全部真实状态码（B39-02/B40-01 家族闭合）。
- **登录平**：管理员口令恒定时间比对 + 300ms 固定延迟拉平（loginTimingFlat）；`XUANKE_TRUSTED_PROXY` XFF 解析取最右非空值且限回环；`maskKey`/`maskedToken`/`tokenShort` 脱敏正确。
- **编译/静态检查**：`cd backend && go build ./...` 与 `go vet ./...` 双通过（本次实证 exit=0）。