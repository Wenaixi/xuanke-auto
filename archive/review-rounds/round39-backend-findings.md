# round39 后端审查原始发现

> 审查基线：master @ 63a7dfc（go build ./... 与 go vet ./... 双重通过）
> 范围：backend/ 下全部 Go 源码（main.go、cmd/、internal/ 各包），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》约 20 条决策锚 + 部署/迁移规范 + legacy/website-source 逆向契约。

---

## MAJOR（明确的功能失效或错误行为，1 条）

### M39-01 scheduler.go:1459-1481（写成功分支的 ClientFor 复核是"存在性"而非"同一性"）
- **文件:行号**：backend/internal/scheduler/scheduler.go:1459-1481（同族还有 1434-1438 失效分支、1208-1212 重登分支）
- **严重级**：MAJOR
- **问题陈述**：删账号防线 `if _, ok := s.clients.ClientFor(acct); !ok` 只校验"账号名当前是否在注册表"，不校验客户端是否仍是发起提交时的同一身份——账号被删后同名重建（同学生换绑/管理员误删后立刻加回）会命中，陈旧的在飞链把成功状态与 success 行写回给**重建身份**。
- **触发场景推演**：
  1. t0：tick → submitAll 为账号 A 生成提交链 C，C 取到旧客户端 OA，进入 `OA.SelectClass(617xx)` 网络往返（最长 15s）。
  2. t1（C 网络在飞期间）：管理员删除 A —— `Accounts.Remove(A)` 摘旧客户端、`PurgeAccount(A)` 清空 A 全部内存态、`Store.DeleteAccount(A)` 清库行。
  3. t2（同一在飞窗口内）：A 重新登录 —— `ensure(A)` 在注册表新建客户端 NA，issueSession、SaveCredential 落库。
  4. t3：`OA.SelectClass` 返回成功（旧 token 在删除前已鉴权通过）。
  5. t4：C 回锁，`ClientFor(A)` 命中 NA（存在性通过）→ `s.done[A][classID]=true`、setStateLocked("success")、`SaveSuccess(A,classID)`（落库 success 行）+ AppendLog。
  6. 结果：重建身份 A 从未主动选择该课程，却带有一条"已报名成功"success 行与 done 标记；`SetTargetsForAccount` 契约绝不清 done（决策锚 6），重启 RestoreDone 也恢复——重建账号被陈旧链"寄生"。
- **建议修法**：在链 goroutine 启动时捕获 `*zhidao.Client` 指针并在返回后与 `ClientFor` 结果做指针身份比对（旧指针≠注册表现指针 → 静默放弃），而非只比对账号名存在性。

---

## MINOR（边界瑕疵 / 契约不自洽，4 条）

### m39-01 handler.go:74-77 —— writeJSON 永不写 HTTP 状态码，panic 恢复的"500"实为 HTTP 200
- **文件:行号**：backend/internal/api/handler.go:74-77（writeJSON）；1144-1155（recoverMiddleware）；router.go:67-75（requireJSONBody）、97-121（登录限流 429）、handler.go:1044-1047（401）、581-584（403）
- **严重级**：MINOR
- **问题陈述**：`writeJSON` 只设 Content-Type、从不 `WriteHeader(code)`，除 `/api/` 显式 WriteHeader(404) 外所有响应都是 HTTP 200 + body `code` 字段；`recoverMiddleware` 注释声称"panic 返回 500 不崩溃"，实际返回 HTTP 200 + body code=500。监控/反代/安全扫描无法在 HTTP 层识别鉴权失败、限流与内部错误，panic 被伪装成 200 成功响应。
- **触发场景推演**：服务端任一 handler panic → `recoverMiddleware` 捕获并 `writeJSON(w,500,...)` → 客户端与监控收到 HTTP 200 + `{"code":500}`；存活探针只查状态码时误判"服务健康"，管理员后台看到的是业务错误而非服务器故障。除 `/api/` 404 外，401/403/429 同理（已被现有注释确认的"业务代码恒 200 约定"只对 404 开了例外）。
- **建议修法**：在 `writeJSON` 增加 `status int` 或显式 `w.WriteHeader`（至少 recoverMiddleware 与 requireAuth/requireJSONBody/loginLim 等基础设施路径写真实 4xx/5xx）。

### m39-02 scheduler.go:750,765,771-772,842,1772 —— 快照 TTL 读侧仍用本地钟 `time.Since` 对照对齐钟写入的时间戳，违反 B20-03"时间基准统一"契约
- **文件:行号**：backend/internal/scheduler/scheduler.go:750（ElectivesSnapshotFor）、765（同函数第二处）、771-772（回退全局帧 TTL）、842-843（ElectivesSnapshot）、1772（CheckClassSelectable）
- **严重级**：MINOR
- **问题陈述**：`ProbeForAccount`/`probe`/`ProbeNow` 写入 `acctDataAt`/`lastDataAt` 用的是 `s.nowAligned()`（对齐钟），但这 5 处读侧用的是 `time.Since(...)`（本地钟），与 B20-03 注释声称的"探测时间戳写入侧改用对齐钟，与读取同一时间基"（只改了写入侧）并不一致——读侧仍混用本地钟。
- **触发场景推演**：clockOffset = server-local ≠ 0（实测 ~640ms）。写入 `acctDataAt = 对齐钟(=本地+offset)`；读侧 `time.Since(acctDataAt) = 本地now - (本地写时刻+offset)`，TTL 判定被整体偏移 ±offset。offset>0（服务器快）时帧显得更"新鲜"，TTL 被拉长约 offset；offset<0 时帧更早过期、ElectivesSnapshotFor 对"过期专属帧"多走一次 ProbeForAccount 真刷新。当前量级（≤0.7s vs 40s TTL）无实质危害，但契约明确声称单基准，且未来时钟漂移代理（offset 不受 30s 失败退避约束时可累积）会放大。
- **建议修法**：5 处读侧统一改传 `now := s.nowAligned()` 入参（或读到 offset 后用 `time.Now().Add(offset)` 计算），与 tick/probeIntervalFor 同源。

### m39-03 scheduler.go:1222 —— 重登成功分支 `s.store.UpdateIDToken` 缺少 `if s.store != nil` 守卫，是全仓库唯一裸写的落库点
- **文件:行号**：backend/internal/scheduler/scheduler.go:1222
- **严重级**：MINOR
- **问题陈述**：scheduler 内其余全部落库点（spawnChain/MarkDone/RemoveDone/SetTargetsForAccount 等共 10+ 处）都包 `if s.store != nil`，唯有重登成功分支的 `s.store.UpdateIDToken(acct, tok)` 未判 nil——与 B33-01"零吞错落库点"规范同族的防御不一致，store 为 nil 的测试/复用场景会直接 nil 指针 panic。
- **触发场景推演**：任何一个 `scheduler.New(clients, nil /*store*/, ...)` 的测试/嵌入场景，账号调用 maybeRelogin 且重登成功 → goroutine 解锁后 `s.store.UpdateIDToken` → nil interface 方法调用 panic；调度器 goroutine 无 recover（见 o39-05），整个轮询引擎静默死亡。
- **建议修法**：补 `if s.store != nil { ... }` 外衣（与 1469-1478/1873-1880 相同模式）。

### m39-04 scheduler.go:902-907 —— `emptyRunsFor` 死代码（无调用方），判据实际直读 `s.state.EmptyProbeRuns`
- **文件:行号**：backend/internal/scheduler/scheduler.go:904-907
- **严重级**：MINOR
- **问题陈述**：`emptyRunsFor(open)` 的 `_ = open` 形参 + 只返回 `s.state.EmptyProbeRuns`，全仓库无任何调用点（windowClosedLocked 直接读 state 字段）；幽灵窗口判据的真实读数与"由 probe() 入账推进"的是同一个字段。残留函数误导维护者以为判据另有入口，且形参 open 完全不消费属于假签名。
- **建议修法**：删除 `emptyRunsFor`（判据签名已收敛到 `windowClosedLocked()` 单源，字段读直接内联）。

---

## OBSERVE（存疑待核 / 低概率边界，5 条）

### o39-01 scheduler.go:975-977 —— `open.IsZero()` 守卫在 `WindowOpened=true` 时也挂起提交，窗口已由探测确证开启却仍停摆
- **文件:行号**：backend/internal/scheduler/scheduler.go:975-977
- **严重级**：OBSERVE（其语义已按 B11-A1 刻意实现并落 CLAUDE.md，此处论证残余缺口）
- **问题陈述**：tick 提交守卫第一判据 `if open.IsZero() { return }` 优先于所有后续判定。即使 `state.WindowOpened == true`（probe 已由 `p.InDateRange` 确证窗口开启），只要识别槽（`openTimeDetected`）一直为空——平台响应未带非空 `beginTimes`（接口裁剪/新批次预热期）——自动链仍被永久挂起，黄金期 250ms 冲刺完全不执行。
- **触发场景推演**：某次发布批次平台顶层 `beginTimes` 缺省/为空 → `probp/probeForAccount` 不写识别槽 → open 恒零；同时 publishes 非空且 `InDateRange=true` → `WindowOpened=true`。tick 第 975 行直接 return，`opened` 与 `WindowClosed` 判据永不执行（二者都在其后），提交 0 次。contrast：`WindowOpened` 是"平台已实证窗口开"的更强证据，却没有任何路径绕开零值守卫。
- **建议修法**：把零值守卫后移到 `if !opened && !now.After(open)` 之后，或增加 `s.state.WindowOpened` 作为放行例外（窗口确证开启后提交不再依赖开窗时刻）。

### o39-02 scheduler.go:1294-1331 + 1351 —— submitAll 目标快照与在飞链竞态：同 tick 内移除/替换的目标仍会被在飞链提交
- **文件:行号**：backend/internal/scheduler/scheduler.go:1294-1331（快照构建）、1351（goroutine 启动）
- **严重级**：OBSERVE
- **问题陈述**：submitAll 在持 s.mu 时把 `s.acctTargets` 拷贝成 chain 快照，spawnChain goroutine 随后异步执行；锁释放后、链对目标迭代期间（网络往返可到 15s），用户 PUT /api/targets 移除某课或整体清空——在飞链仍按旧快照发起 SelectClass，用户"撤销目标"意图在该链生命周期内不被尊重。
- **触发场景推演**：黄金期提交中，用户在前端把某课程从目标移除 → handleSetTargets（SetTargetsForAccount 重建 acctTargets/state）立即生效；但同 tick 已 spawn 的链仍持有含该课的 ts 快照，下一发 SelectClass 照常提交并 SaveSuccess——用户已明确撤销的课程被自动报名成功，且 done 按契约不清（决策锚 6），重启后仍显示"已报名成功"。
- **建议修法**：链内每课程提交前（inflight 之前）复核 `s.acctTargets[acct]` 中仍存在该 `(publish_id, class_id)`（重读而非快照）。

### o39-03 handler.go:751-756 —— 管理员无法清空已配置的 Vision API Key
- **文件:行号**：backend/internal/api/handler.go:751-756
- **严重级**：OBSERVE
- **问题陈述**：PUT /api/admin/config 对 `vision_api_key` 空串/全空白值静默忽略（保留旧值），没有任何清除语义——一旦配置了错误的 key（或想切纯 ddddocr 并把 key 从 settings 里彻底抹掉），UI 无法置空，settings 表永久残留密文旧 key。
- **触发场景推演**：管理员误填 key 后想恢复留空（"未配置"回显态），留空提交 → 后端忽略 → 回显仍是旧掩码；安全审计需求"敏感值清除即清除"不可达。
- **建议修法**：给 PUT 增加显式清除字段（如 `"vision_api_key_reset": true`）或把空串语义改为"清空"（前提：引擎已切 ddddocr 或允许无 key 运行）。

### o39-04 handler.go:1086-1142 —— 登录/激活限流按 clientIP 分桶，学校 NAT 单出口下全校合并进 5 次/分钟一个桶
- **文件:行号**：backend/internal/api/handler.go:1086-1114（tokenBucket allow）、1116-1142（clientIP）
- **严重级**：OBSERVE（本身是文档化的安全权衡，此处提示部署风险）
- **问题陈述**：`clientIP` 在 `XUANKE_TRUSTED_PROXY=off`（默认）时取 RemoteAddr 主机段；学校/宿舍 NAT 下所有学生、乃至管理员在同一出口 IP 后，login/activate 各 5 次/分钟的令牌桶被全校共享——正常上学时段几个学生同时登录即互相挤爆，管理员从同一出口也会被 429 锁住。
- **触发场景推演**：某个家庭宽带/学校无线出口下有 5 名学生同分钟内完成首次登录 → 第 6 个（含管理员）收到"登录尝试过于频繁"；激活码尝试同理被挤爆。代码注释已承认"全校共用一个配额"但只在可配 XFF 反代时才缓解。
- **建议修法**：部署文档要求前置可信反代并开 `XUANKE_TRUSTED_PROXY=on`；或在不信任 XFF 的前提下把登录桶容量/速率放宽到按"账号名+IP"复合键。

### o39-05 scheduler.go:636-654 —— 唯一调度主循环 goroutine 无 recover，tick 内未来改动引入 panic 即静默停摆
- **文件:行号**：backend/internal/scheduler/scheduler.go:636-654（Start 的 goroutine）
- **严重级**：OBSERVE
- **问题陈述**：tick 主循环没有任何 panic 恢复；HTTP 侧有 recoverMiddleware，但调度器、探测、重登、提交链（tick/probe/spawnChain goroutine）都在中间件覆盖范围之外——一旦 tick 内部出现空指针/map 越界（如未来 `s.openTimeDetected` 未初始化等回归），goroutine 直接退出，无日志、无存活告警，抢课静默永久停止。
- **触发场景推演**：任何使 tick 内某行 panic 的回归（nil map 写入、nil 接口方法调用等）→ 主循环 goroutine 退出；`HasProbed`/`WindowOpened` 停在旧值或零值，管理员前端看到"未探测"但无任何错误提示，直到选课窗口结束才发现全程未抢课（当前无 panic 路径，属防御性缺口）。
- **建议修法**：tick goroutine 内 `defer func(){ if rec:=recover(); rec!=nil { log.Printf(...); } }()` 后继续循环（panic 只丢本 tick），或至少在退出前打一条醒目错误日志。

---

## 已核对无问题的重点区域（梗读核过）

- **调度器锁纪律**：`s.mu` 覆盖全部状态写入（acctTargets/inflight/done/full/refused/rateLimited/acctData/tokenValid/openTimeDetected），无锁外写状态；`reloginMu → s.mu` 锁序全链一致（maybeRelogin/TokenValidFor/MarkTokenValid 三处同序，无反向路径），无重入。
- **WindowClosed 三判据单源** `windowClosedLocked()`：主判据（曾开窗+空快照+已过 open+10s，+10s 裕量在判定侧）/ 时钟连续失败≥3（带"开放时间已过"）/ 幽灵窗口 EmptyProbeRuns≥3（+10s 裕量）——`WindowClosed()` 与 `StateForAccount` 共用，无两套真相；单次 open 快照复用。
- **开放时间识别槽三硬契约**：直返识别值不截断（0c193a2 根治黄金期挂起）、空快照不删槽、写入全部持 s.mu（ProbeForAccount 与 probe 两处均已带锁，无数据竞争）。
- **tick 提交守卫链**：open 零值→挂起；`!opened && !now.After(open)`→未到点；`WindowClosed()`→挂起；lastSubmit+submitIntervalFor（黄金期 250ms）——各判据顺序与语义正确（除 o39-01 的零值例外缺口）。
- **探测节流三件套**：`lastProbe` 只归 probe()/ProbeNow 写、失败同样入账防 300ms 轰炸；`probing` 置位-检查在锁内原子；`probeSem`(cap 4) 封顶 per-account 并发；B6-04 契约（ProbeForAccount 不写 lastProbe）已落实。
- **删账号 memory-first + 落库前锁内复核 ClientFor**：自动链成功 / 失效分支 / 手动 MarkDone/RemoveDone / 重登成功 / 实时复核回锁后三路 / spawnChain 链顶+goroutine 内双判据全部到位（唯一不足即 M39-01 的"存在性≠同一性"）。
- **重登与闸门**：reloginFail 只在成功清零（B21-01 语义）、失败保留计数、`gateWait`/`GatePump` 每分钟 ≤2 次 doLogin，30s 基础节流 + 指数退避封顶 10min，逻辑正确。
- **零吞错落库点**：全部 SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefusedClass/AppendLog/SetTargetsForAccount/UpdateIDToken 调用均 `if err != nil { log.Printf }`（唯一例外即 m39-03 的 nil-store 守卫缺失，与吞错无关）。
- **凭据 AES-256-GCM**：nonce 前置 hex、`.master_key`/env 32 字节校验、enc: 前缀、旧明文拒绝加载，正确。
- **会话与票据**：32 字节 crypto/rand（熵源故障拒绝签发）、12h TTL 惰性失效+后台清扫（Close 防泄漏）、ticket 绑定账号+单次防重放（ConsumeTicket 在码校验前销毁是刻意设计、已文档化）。
- **激活码原子性**：`used_uses < total_uses` 单条 UPDATE 原子防超卖、已激活不扣次（事务内查重）、CreateActivationCodes 单事务、密钥 64bit 熵。
- **SQL 全面参数化**（modernc 占位符绑定）；数据库迁移（migrateAddPublishMeta 增量 ALTER + refuseLegacy 缺列清单已不含已迁移列）正确。
- **路由/中间件**：/api 前缀（含精确 /api）显式 404 不落 SPA；csrf 缓解（jsonContentType 拒表单、DELETE 已放行空 body）；XFF 仅在回环+`XUANKE_TRUSTED_PROXY=on` 时信任；`go build ./...` + `go vet ./...` 双通过。
- **登录平**：管理员口令等时比对+固定 300ms 延迟抹平时延差；登录/激活限流桶独立。