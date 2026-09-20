# round43 后端审查原始发现

审查范围：`backend/` 全部 Go 源码（scheduler / api / accounts / session / store / db / secure / config / runtime / zhidao / main / web）。
判据：项目根 CLAUDE.md 工程决策手册（决策锚 B01-B34 + 契约速记）、平台逆向锚（legacy/website-source + HAR）。
本轮对 round42 观察项逐一核对：MAJOR-42-02（B21-03 发起侧裸奔）、M40-01、m40-02、o40-01~o40-04、m39-02、o39-02~o39-05。

---

## CRITICAL

（无本轮新增 CRITICAL。round42 的 B42-02 已收口重登顺序，identity 复核族全分支闭合，未发现可确证的账号级数据污染新通道。）

---

## MAJOR

### MAJOR-43-01（B43-02，主控核实落盘）：`maybeRelogin` 入口无账号存在性复核 —— 探测定时三处直调可污染同名重建账号
- 位置：`backend/internal/scheduler/scheduler.go:1187-1221`（maybeRelogin 入口）；调用点 `:821`（ProbeForAccount）/ `:944`（ProbeNow）/ `:1083`（probe）对 ErrUnauthorized 无条件直调
- 一句话问题：B42-02 只收口了 spawnChain 失效分支（先 `sameClientFor` 再 `maybeRelogin`），但探测定时三处对 ErrUnauthorized 无条件调 `maybeRelogin`；而 maybeRelogin 入口（1188-1191 两把锁内）只有 `relogging/reloginFail/reloginAt` 三组 map 检查，**没有 `ClientFor` 存在性复核**——删号与在飞探测返回 ErrUnauthorized 同帧（窗口约 15s）时，`tokenValid[acct]=true`（1216 行置位）/ `reloginFail[acct]++`（1211 行）/ `reloginAt[acct]=now` 被重新写入已删账号的 map key（PurgeAccount 已清）。
- 触发场景推演：管理员删账号 A（PurgeAccount 全清 A 的 map）→ 已在飞持有 A 客户端实例的探测（ProbeForAccount 814 行已取 client）/全局探测（probe 1067 行 AnyClient 取到 A）在网络往返期间完成删除 → `FindElectives` 命中 ErrUnauthorized → 820/821 行 `maybeRelogin(A)` → 入口无存在性复核直接写 4 组 map（tokenValid=true / reloginFail=1 / reloginAt / relogging=true）。随后同名重建（换绑/误删加回 `LoginByPassword` ensure 新客户端）：新客户端 token 有效，但调度器 map 残留 `tokenValid[A]=true` + `reloginFail=1` → 前端 /state 显"已失效·自动恢复中" + spawnChain 整链挂起提交 + 该账号首次真实失效时无辜 30s 退避——正是 B42-02 注释（1473-1476 行）声称要防的"同名重建污染"，但 spawnChain 路径之外仍可触发。B21-03 只护了重登 goroutine 的**写回侧**，**决策侧裸奔**。
- 建议修法（与 B42-02 同族，收敛单点）：maybeRelogin 入口 `s.mu.Lock()` 后、任何 reloginAt/reloginFail/relogging/tokenValid 写入前，`if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`（账号已删不发起重登、不写任何 map）。

### MAJOR-43-02：`spawnChain` 实时复核 ErrUnauthorized 分支无指针身份复核 —— B39-01/B41-01 同族防线漏掉一个写点
- 位置：`backend/internal/scheduler/scheduler.go:1583-1595`
- 一句话问题：实时复核命中 token 失效的分支（`cErr != nil && errors.Is(cErr, ErrUnauthorized)`）只做了账号名存在性复核（1575 行 `ClientFor(acct)`），**缺 `sameClientFor` 指针身份比对**，是身份防线最后一块裸露写点。
- 触发场景推演：账号 A 在飞 SelectClass 期间被管理员删除并同名重建（换绑/误删加回，注册表指针已换）。本链走到 1568 行锁外 `classFullRealtime` → 命中新身份的 ErrUnauthorized → 回锁后 1575 行存在性复核通过（新身份存在）→ **无指针比对直接放行** → `maybeRelogin(acct)` 对新身份触发自动重登 + `setStateLocked("failed","教务令牌失效，自动重登中")` + 落一条假失效日志。同名重建账号的首次自动重登被迫早于其真实 token 失效（无辜消耗登录预算），且状态行被旧链污染。与同函数内成功分支（1500 行）/风控分支（1530 行）/窗口关闭分支（1550 行）/确证满员分支（1610 行）四处的 `sameClientFor` 不对称——round41 修了满员分支却漏了紧邻的失效分支。
- 建议修法：1568 行回锁后、进失效分支前，把 1575 行换成 `if !s.sameClientFor(acct, chainClient) { s.mu.Unlock(); return }`（存在性已内含于 sameClientFor）。

### MAJOR-43-03：`spawnChain` 成功分支身份复核失败时 inflight 位漏删，旧链残留阻塞同名重建账号手动报名
- 位置：`backend/internal/scheduler/scheduler.go:1491-1504`
- 一句话问题：成功分支身份复核失败（1500 行 `!sameClientFor`）时直接 `return`，**`s.inflight[acct][classID]` 位未清**（1468 行失效分支有 `delete(s.inflight...)`，1500 行成功分支没有——不对称）。
- 触发场景推演：账号 A 在飞 SelectClass（inflight 位已占）→ 管理员删除 A 后**立即同名重建**（`LoginByPassword` ensure 新客户端；PurgeAccount 清 inflight 发生在重建前、重建后旧链才返回）→ 旧链 SelectClass 失败/成功返回但身份已变 → 1500 行 identity 复核失败 → return（无清位）→ 同名重建账号该课程 inflight 位恒 true → `TryAcquireSubmit` 永久 false（手动报名恒"该课程正在提交中"）+ 自动链 `inflightHas` 恒跳过该课。清位入口只剩手动 RemoveDone/重启，黄金期内该课彻底失联。
- 建议修法：1500 行分支 return 前补 `delete(s.inflight[acct], t.ClassID)`（与 1468 行失效分支对称）。

### MAJOR-43-04：`handleLogin` 学生账号名与配置管理员名相同时不可登录或会话身份错乱
- 位置：`backend/internal/api/handler.go:117-136`
- 一句话问题：管理员名校验只挡"用户名 == adminName"；**学生账号恰好也叫 adminName 时**，`POST /api/login` 走 117 行管理员分支——口令必错 → 该学生永远无法登录（DoS）。
- 触发场景推演：配置 `XUANKE_ADMIN_NAME=admin`，教务平台恰好有学生账号名为 `admin`（真实学校教务系统学生学号不含 admin，但教师/管理员账号或平台允许自定义账号名时可能撞名）。反向：管理员改名后（`XUANKE_ADMIN_NAME=teacher`），学生账号撞新名同样被吞；若学生账号不撞名，`IsAdminAccountName(acct)` 对 `?account=`/手动操作/删除保护仍按配置名判定，无错位。属配置撞名缺陷，非主动攻击通道。
- 建议修法：管理员分支改为 `if req.Account == adminName && subtle.ConstantTimeCompare(...)` 双条件，教务登录分支对撞名账号显式拒绝提示改名。

### MAJOR-43-05：`handleAdminStats` 的 `targetsCount` 循环吞错静默计 0
- 位置：`backend/internal/api/handler.go:885-891`
- 一句话问题：`LoadTargetsForAccount(a)` 失败时静默 `continue`（`if err == nil { targetsCount += len(ts) }`），DB 故障时 stats 显示 `targets_count=0` 误导管理员"无人设目标"，违反"零吞错"精神（非落库点，但为状态展示）。
- 触发场景推演：SQLite 磁盘故障/读锁等待超时，`LoadTargetsForAccount` 对某个账号失败 → targets_count 缺算；管理员看到 0 目标误判部署异常。
- 建议修法：失败时 `log.Printf` 并累计 error，最终返回明确报错（对齐 handleAdminStats 其他数据源"任一失败即 500"的风格）。

---

## MINOR

### MINOR-43-01：`LoginByPassword` 凭据加密失败静默跳过落库
- 位置：`backend/internal/accounts/manager.go:277-283`
- 一句话问题：`encrypt(password)` 返回 err 时**整个凭据不落库**（内存登录成功、重启后账号丢失），且无日志。
- 触发场景推演：主密钥损坏/加密器异常时，学生登录成功、会话可用，但密码未持久化——重启后 Restore 无凭据，自动重登永久"无保存账密"（只能手动重登）。
- 建议修法：`else { log.Printf(...) }`。

### MINOR-43-02：`syncFailedWindow` 与 `lastSyncStart` 字段只写不读（死字段）
- 位置：`backend/internal/scheduler/scheduler.go:375 / 391 / 185 / 354`
- 一句话问题：`syncFailedWindow`（B19-01 留档）与 `lastSyncStart`（B7-M1 推进）只被写入、从未被读取——维护者可读性误导，非功能性缺陷。
- 建议修法：删除或补注释"仅留档"。

### MINOR-43-03：`handleLogin` 管理口令错误分支与正确分支均 Sleep 300ms，但学生账号教务登录失败路径无固定延迟
- 位置：`backend/internal/api/handler.go:123-129 / 137-139`
- 一句话问题：n4 时延拉平只覆盖"管理员名"场景；**学生账号名 + 错误口令**走教务登录（网络往返天然延迟）与"学生账号名 + 正确口令"（成功路径也走教务登录）时延接近，未确证明显分叉——维持观察。

### MINOR-43-04：`reloginBackoff` 的 `backoffMin << (n-1)` 在 n=1 时即 30s，注释"首次失败即基础间隔"与调用点 `reloginFail++` 前置语义存在一处微偏（n 为累计失败次数）
- 位置：`backend/internal/scheduler/scheduler.go:1168-1180`
- 一句话问题：`reloginBackoff(n)` 用**本次发起时已递增到**的失败次数（1211 行先 ++ 再判断退避），n=1 返回 30s、n=2 返回 60s——语义正确但注释"连续失败 1 次=基础 30s"与实际（第 2 次失败才 60s）易误读，非功能缺陷。

---

## OBSERVE

### OBSERVE-43-01：`probe()` 全局帧客户端（order[0]）与 per-account 帧客户端不一致时，`openTimeDetected["*"]` 与 `[acct]` 双槽可能分叉（承接 m39-02 快照 TTL 读侧本地钟观察）
- 位置：`backend/internal/scheduler/scheduler.go:1067 / 1095-1100 / 1144`
- 说明：`windowClosedLocked` 与 `probeIntervalForOpen`/tick 一律读 `openTimeForLocked("")`（即 `["*"]` 槽）。当 order[0] 账号无目标而其他账号有目标时，`probe()` 主体用 order[0] 客户端写 `["*"]`，per-account goroutine 用目标账号客户端写 `[acct]`——两者 beginTimes 相同（全校共享契约）时无分叉；仅当平台对某账号不下发 beginTimes 时 `["*"]` 与 `[acct]` 可能不同。未确证真实分叉，维持观察。
- 建议：probe() 主体的 beginTimes 识别写入可统一用 order[0] 帧，per-account 只写专属槽，避免双槽语义竞走。

### OBSERVE-43-02：`reloginResults` cap 8 满丢弃补探测信号（round42 o40-02 延续）
- 位置：`backend/internal/scheduler/scheduler.go:257 / 1257-1260 / 673-678`

### OBSERVE-43-03：`classFullRealtime` 每失败 tick 网络往返（IsClassFull 恒 false 时锁外网络段重复）
- 位置：`backend/internal/scheduler/scheduler.go:1568 / 1715-1721`（防御性路径，注释已自述）

### OBSERVE-43-04：`tick` 提交分支无 recover（o39-05 延续），spawnChain goroutine 内 `s.clients` 为 nil 时 `submitAll` 已判空、链内 `ClientFor` 有判，未确证 panic 路径
- 位置：`backend/internal/scheduler/scheduler.go:964-1024`

### OBSERVE-43-05：M-38-01 实时复核回锁后存在性复核使用 `ClientFor(acct)`，与 `sameClientFor` 双判并存（存在性复核先于身份复核，属冗余防御层）
- 位置：`backend/internal/scheduler/scheduler.go:1575 / 1610`

---

## 已核对无问题的重点区域

- **B42-01 doLogin 全入口闸门**：`LoginByPassword` 已收口 `gateTryAcquire`（manager.go:244），与排队重登共享 gateUsed 计数，无旁路。`ReloginIfNeeded`/`Manager.Relogin` 走 gateWait 阻塞排队，语义对齐。
- **B42-02 重登顺序**：spawnChain 失效分支先 identity 复核、后 maybeRelogin（scheduler.go:1468-1478），身份已变不触发重登，Manager.Relogin 对已删账号不再被旧链调用——已修复，无新问题。
- **B39-01/B41-01 identity 复核族**：成功（1500）/失效（1468）/风控（1530）/窗口关闭（1550）/确证满员（1610）五分支已闭合；`sameClientFor` 内含存在性判定，`clientIdentity` 反射指针比对正确（覆盖测试 fake 指针）。
- **WindowClosed 三判据单源**：`windowClosedLocked`（scheduler.go:907-928）与 `StateForAccount`/`WindowClosed()` 共用，判据 2/3 带"开放时间非零"守卫，open 单快照复用，无分叉。
- **openTime 识别槽三硬契约**：识别槽保留（空快照不删）、展示层过期判定独立（StateForAccount 712 行）、写入持锁（1095-1100/841-846）——全部符合。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1000 行）语义正确，`WindowOpened=true + 识别槽空` 提交不再挂起。
- **零吞错落库点**：全部 `SaveSuccess/DeleteSuccess/UpdateIDToken/SaveRefused/DeleteRefused/DeleteRefusedClass/AppendLog` 调用均 `if err != nil { log.Printf }`；仅 `handleAdminStats` 的 targetsCount 循环属非落库读点（见 MAJOR-43-05）。
- **删账号 memory-first + PurgeAccount**：`handleAdminDeleteAccount`（handler.go:969-983）顺序正确；`PurgeAccount` 全量清理含 openTimeDetected/acctData/tokenValid/relogin 族。
- **tick 时间基**：探测/提交节流读写两侧均对齐钟（B20-03），`markRateLimitedLocked` 写入对齐钟（1678 行），`lastProbe` 只归 probe/ProbeNow 写入。
- **激活码/ticket**：`ConsumeTicket` 单次销毁绑定账号；`ConsumeActivationCode` 单条 UPDATE 原子扣次；已激活不重复扣次（B5-08）。
- **凭据加密**：AES-256-GCM + 随机 nonce 前置；`.master_key` 32 字节校验（F17-04）；`vision_key` 强制 `enc:` 前缀，拒绝旧明文。
- **?account= 凭据表校验**：handleElectives / handleElectiveSelect / handleElectiveExit / handleState / handleSetTargets 五路均 `accountExists` 或 LoadCredentials 校验，查无此账号整体拒绝。
- **writeJSONStatus 基础设施路径**：panic 500 / 会话 401 / 管理 403 / 限流 429 均写真实 HTTP 状态码。
- **XFF 信任反代**：仅 `XUANKE_TRUSTED_PROXY=on` 且回环地址才信最右非空值，默认关闭。
- **SQL 注入**：全部参数化查询；`columnExists` 拼接值为全常量（o40-03 确认）。
- **SpaHandler /api 兜底**：`/api` 精确与 `/api/` 前缀均 404，不回退 index.html。
- **会话清扫**：session.Store Close 双 channel 收口，无 goroutine 泄漏；sweeperStop 非 nil 判定正确。

---

## 结论

- 未发现新增 CRITICAL；MAJOR 4 条（maybeRelogin 决策侧污染 1 + 实时复核失效分支缺 identity 复核 1 + inflight 位漏删 1 + 管理员撞名 1）+ OBSERVE 中复述的 stats 吞错（MAJOR-43-05 归类观察，正文列 4 条 MAJOR）。其中 **MAJOR-43-01（maybeRelogin 入口缺存在性复核）** 是身份防线最后一块裸露写点，与 B42-02 收口的 spawnChain 路径直接不对称，且可通过常规探测路径触发（非极端时序），建议优先修复。
- round42 观察项核实：MAJOR-42-02 已修复（B42-02）；m40-02/o40-02（reloginResults 满丢弃）确认仍在，维持观察；其余观察项未升级。
