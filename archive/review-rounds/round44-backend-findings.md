# round44 后端审查原始发现

> 审查基线：master @ b275270（round43 全部修复已落盘；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式，未修改/创建/删除任何文件。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚（R43 已扩至 39 条）+ legacy/website-source 逆向契约 + round39~43 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 锁序/竞态逐状态序列推演 + 对上轮观察项逐一核实。`cd backend && go build ./...` 与 `go vet ./...` 双通过实证 exit=0；`go test -race -count=1 ./...` 10 包全绿。

---

## CRITICAL

（无本轮新增 CRITICAL。B43-01/02 决策侧+写回侧身份双闭合后，删号-同名重建竞态在调度器路径已全部封死；未发现可确证的账号级数据污染新通道。）

---

## MAJOR

（无本轮新增 MAJOR。上轮 5 条 MAJOR 已修复并经本轮逐项复核无回归，见下节。）

---

## 本轮重点核对（上轮新契约，防回归）—— 4 条全部正确闭合

### 1. B43-01 maybeRelogin 入口复核 —— 已正确闭合
- **位置**：`backend/internal/scheduler/scheduler.go:1190-1200`
- **核实**：入口 `s.mu.Lock()` 后、任何 map 写入前补 `if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`（1197-1200），位于 reloginAt/reloginFail/relogging/tokenValid 全部写入之前；锁内判定、锁内返回，与 reloginMu→s.mu 锁序一致。探测定时三处直调路径 ProbeForAccount（821）/ ProbeNow（944）/ probe（1083）全部落在本入口统一复核之下，无旁路；判定后不写任何 map，符合"决策侧复核"语义。测试 `TestMaybeReloginDeletedAccountSkipsMaps`（scheduler_test.go:3387）在库。

### 2. B43-02 实时复核失效分支 —— 已正确闭合
- **位置**：`backend/internal/scheduler/scheduler.go:1589-1592`（回锁后 `!s.sameClientFor(acct, chainClient)` 前置）与 1597-1608（失效分支）。
- **核实**：`sameClientFor` 内含存在性判定（210-215：ClientFor 不存在即 false）；`chainClient` 捕获于链顶 1406 行（取 client 成功时才赋值，goroutine 内已然非 nil），锁外复核网络段（1578 `classFullRealtime`）前后始终有效。同名重建/删号时序下陈旧链统一在 1589 行静默放弃，不触达 1597 失效分支的 `maybeRelogin` 与 failed 状态写。新竞态核查：锁外段内 `chainClient` 指针不会被回收（Go 指针值安全），回锁后 `sameClientFor` 用 `clientIdentity`（reflect 指针比对）复取当前注册表，正确区分"对象仍被持有但已被注册表摘除"的删号场景。测试 `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（scheduler_test.go:3421）在库（500ms 轮询断言 relogCalls==0）。

### 3. B43-04 管理员撞名双条件 —— 已正确闭合
- **位置**：`backend/internal/api/handler.go:121`（`req.Account == adminName && subtle.ConstantTimeCompare(...)==1`）。
- **核实**：管理员签发路径只认双条件；撞名学生走 132 行教务登录成功签发普通会话（`issueSession` → `Sessions.Create`，Admin:false）。`requireAdminSession` 判 `Sessions.IsAdmin(tok)`（会话级 Admin 标记），撞名学生普通会话绝不进管理员分支——无残余错位判据。`IsAdminAccountName`（56-58）对 `?account=`/删除保护的判据仍按配置名，语义与 B43-04 一致：管理员（双条件签发的 Admin 会话）受删除保护；撞名学生因账号名恰为配置名也被一并保护（不可删除/不可穿透），这是"管理员名占用"的本意，无需修正。测试 `TestLoginAdminNameCollisionStudentCredential`（handler_test.go:1442）覆盖未激活→1001 颁票据→激活→普通会话签发→非法定管理员权限→反向管理口令错误五态。

### 4. B43-05 handleAdminStats 500 路径 —— 已正确闭合
- **位置**：`backend/internal/api/handler.go:888-907`
- **核实**：targetsCount 循环对任一 `LoadTargetsForAccount` 失败记日志 + 累计 firstErr；循环后 `if targetErr != nil { writeJSONStatus(w, 500, 1, nil, "统计目标数失败: ..."); return }`——HTTP 500 + body code=1 双通道，与 handleAdminStats 其他数据源（ListAccounts/LoadSuccess/LoadAllLogs）"任一失败即报错"风格一致。统计正确性：仅成功路径累加 len(ts)，失败不计入，无部分成功误导。测试 `TestAdminStatsTargetsLoadFailureReturns500`（handler_test.go:810）在库。`writeJSONStatus` 只覆盖基础设施/统计错误路径，业务 writeJSON 调用点未扰动。

---

## MINOR

### MINOR-44-01：`LoginByPassword` 凭据加密失败分支无日志（MINOR-43-01 复核确认，建议补）
- **位置**：`backend/internal/accounts/manager.go:277-283`
- **一句话问题**：`if enc, err := encrypt(password); err == nil { ... }` 失败时静默跳过落库、无日志——内存登录成功但凭据不落库，重启后 Restore 无凭据，自动重登永久"无保存账密"。触发现实性低（encrypt 失败需加密器异常/主密钥损坏，而这些场景启动即拒），但与决策锚 17"零吞错"及 `SaveCredential` 内部 `log.Printf` 的对称性有差距。
- **建议修法**：`else { log.Printf("[accounts] 账号 %s 密码加密失败，凭据未落库: %v", acct, err) }`，一行。

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-43-01 probe() 全局帧（order[0]）与 per-account 帧客户端不一致时 openTimeDetected["*"] 与 [acct] 双槽可能分叉 | 延续 | probe() 主体用 AnyClient（order[0]）写 `["*"]`（1097），per-account goroutine 经 ProbeForAccount 写 `[acct]`（843）；windowClosedLocked/tick/probeIntervalFor 读 `openTimeForLocked("")`（先 [acct] 后 ["*"] 回退）。全校共享单值契约（HAR 实证）下不实际分叉；仅当平台对某账号不下发 beginTimes 时潜在不同。 | 延续观察 |
| OBSERVE-43-02 reloginResults cap 8 满丢弃补探测信号 | 延续 | 通道 cap 8（257），重登成功非阻塞发送（1268 select default），tick 主循环消费（673-678）重置 lastProbe→下 tick 补探测。满丢弃只在并发重登爆炸（>8）时发生，属刻意权衡。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 每失败 tick 锁外网络段重复 | 延续 | IsClassFull 恒 false（maxCount 平台未下发），非满员失败每 tick 重打 findElectivesStudentCount（1578）。注释已自述"防御性路径"，快照判满主路径 classFullInSnapshot（1446）在实时复核前已生效，频率受提交间隔约束。 | 延续观察 |
| OBSERVE-43-04 tick 提交分支无 recover | 延续 | tick（962-1025）→ probe/submitAll→spawnChain goroutine 均无 recover。s.clients 判空在 maybeSyncClock/submitAll/spawnChain 处处有，未确证 panic 路径。属"未来回归"防御缺口。 | 延续观察 |
| OBSERVE-43-05 实时复核回锁后 ClientFor+sameClientFor 双判冗余 | 已消解 | 1589 行现只 `sameClientFor`（内含存在性），独立 `ClientFor` 前置已被 B43-02 移除。 | 已消解 |
| MINOR-43-01 LoginByPassword 凭据加密失败静默跳过落库 | 复核成立 | manager.go:277-283 无日志（见 MINOR-44-01）。 | 维持观察（建议补 log） |
| MINOR-43-02 syncFailedWindow/lastSyncStart 死字段 | 延续 | syncFailedWindow（375/392）写而不读但注释自述留档语义；lastSyncStart（354 写、391 读）实际有读点。 | 延续观察 |
| MINOR-43-03 学生账号教务登录失败路径无固定延迟 | 延续 | 学生账号走教务网络往返天然延迟，与成功路径时延接近未确证分叉；管理员名错误口令分支固定 Sleep 300ms（handler.go:137）保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释微偏 | 延续 | reloginBackoff(n) 调用侧先 reloginFail++（1221-1222）再传入，n=1 返 30s、n=2 返 60s；注释"连续失败 1 次=基础 30s，之后翻倍"与实际语义一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录（发起侧裸奔） | 延续 | Manager.Relogin（manager.go:167-175）锁内取指针、锁外 ReloginIfNeeded；删号+在途重登并发时平台侧发生一次孤儿登录（token 刷新到孤儿对象），新身份短暂用旧 token。同构窗口存在于 classFullRealtime（1578 锁外用旧指针）。频率极低、无数据污染（新 token 落库被 B21-03 复核拦截）。 | 延续观察 |
| M40-01 本地钟混用（退避/节流 time.Since） | 延续 | maybeRelogin 退避/节流（1209/1216/1220）本地钟读写同基内部自洽，偏差 ≤640ms 对 30s 节流无实质危害。 | 延续观察 |
| m40-02 锁内多取 now | 延续 | spawnChain 1430 行锁内取一次 nowAlignedLocked，亚毫秒统计口径噪音，读写对称。 | 延续观察 |
| o40-01~04 / m39-02 / o39-02~05 | 延续 | 逐一复核：未激活不发会话无残留 / reloginResults 满丢弃刻意权衡 / columnExists 拼接全编译期常量 / interval≤0 生产恒 300ms / 快照 TTL 读侧本地钟（≤640ms vs 40s 低危害） / submitAll 快照与在飞链竞态（无撤销语义，平台最终把关） / vision key 空串无法清空 / NAT 出口登录限流合并 / tick 无 recover（同 OBSERVE-43-04）。 | 全部延续观察 |

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查/测试**：`go build ./...`、`go vet ./...`、`go test -race -count=1 ./...` 三通道 exit=0，10 包全绿（含 scheduler -race）。
- **identity 复核族全闭合**：spawnChain 六分支（成功/失效/风控/窗口关闭/实时复核满员/实时复核失效）全部 sameClientFor 前置；maybeRelogin 入口（B43-01）决策侧复核；MarkDone/RemoveDone/重登 goroutine 写回侧（B18-M2/B20-01/B21-03）——决策+写回+失效归并三族闭合。B43-03 已实测归因（1502 行公共清位已存在）并固化契约测试。
- **WindowClosed 三判据单源** `windowClosedLocked()`（907-928）：主判据（+10s 裕量）/时钟失败≥3（带"开放时间已过"）/幽灵窗口 EmptyProbeRuns≥3（+10s 裕量），StateForAccount 与 WindowClosed()/handleAdminStats(window_closed) 共用同源，open 单快照复用。
- **openTime 识别槽三硬契约**：识别槽保留（空快照不删，ProbeForAccount 841-846 / probe 1095-1100 只在非空 beginTimes 时写）；展示层过期判定独立（StateForAccount 712-714）；写入持锁。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1000）语义正确，WindowOpened=true + 识别槽空时提交不放挂；既有测试 TestSubmitSuspendedWhenOpenTimeCleared 仍绿。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow；probing 单飞锁内置位-检查（1038-1044）；probeSem cap 4（1058-1066）；tick 首探豁免（978）与 submitIntervalFor 250ms 冲刺（1020）。
- **多账号年级隔离**：probe() 每账号独立 goroutine + ProbeForAccount 专属帧；ElectivesSnapshotFor 目标账号过期专属帧 → (nil,false) 触发真刷新，绝无年级串线回退。
- **凭据加密**：AES-256-GCM + 随机 nonce 前置；`.master_key`/env 32 字节校验（F17-04）；enc: 前缀；旧明文拒绝加载（main.go:93-95）。LoginByPassword 登录成功后 SetCredentials 写客户端内部账密（B6-01），重启 Restore 重建。
- **doLogin 闸门全收口**：gateTryAcquire + gateWait 共享 gateMu/gateUsed 计数，LoginByPassword 与 Relogin 无旁路（B42-01）；GatePump 每分钟推进。
- **鉴权与数据安全**：会话 12h TTL 惰性失效 + 5min 清扫；requireAdminSession/requireAuth 写真实 HTTP 状态码（403/401）；SpaHandler /api 精确+前缀 404（embed.go:28-31）；XFF 仅回环+XUANKE_TRUSTED_PROXY=on 信任最右非空。
- **SQL**：全参数化绑定（modernc 占位符）；columnExists 拼接全常量；migrateAddPublishMeta 增量迁移 + refuseLegacy 缺列清单对应；db.Open SetMaxOpenConns(1) 串行化。
- **handleAdminStats**：open_time 展示用 RecognizedOpenTime（零值输出空串，open_time_set 判未识别）；window_opened/window_closed 与调度器同源；account_count/success_count/log_count 全读库无吞错。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 1024 位公钥硬编码；uniqueDeviceID UA|Win32|881|1410|时间戳36进制 base64；captcha 识别并发信号量 + 3 次重试收敛；ErrUnauthorized（code=-1）统一由 doRequest 抛出；cookie/idToken 双通道与 HAR 实证一致。
- **cmd 工具**：probe 读 XUANKE_PROBE_TOKEN（无硬编码 token）；logintest 跟随识别引擎 + wait 间隔符合平台限流；bench 串行压测 401/真实路径。均只读、无副作用。

---

## 结论

- 未发现新增 CRITICAL/MAJOR。上轮 5 条 MAJOR（B43-01~05）经本轮逐项复核全部正确闭合、无回归；身份防线（决策侧+写回侧+失效归并）三族闭合后，删号-同名重建竞态在调度器路径已全部封死。
- 上轮观察项 14 条全部复核：OBSERVE-43-05（双判冗余）已被 B43-02 修复消解；其余 13 条（含 MAJOR-42-02 孤儿登录、M40-01、m40-02、o40-01~04、m39-02、o39-02~05）全部延续观察，无升级。
- 新增 MINOR 级建议 1 条（MINOR-44-01：LoginByPassword 加密失败补日志，与 MINOR-43-01 同源），建议修复代理顺手补一行。
- 残余风险集中在两条延续观察：孤儿登录的平台侧副作用（MAJOR-42-02，频率极低、无数据污染）与 tick 无 recover（防御性缺口，无当前可触发路径）。