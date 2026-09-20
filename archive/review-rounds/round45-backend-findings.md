# round45 后端审查原始发现

> 审查基线：master @ `c498171`（R44 收敛轮落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式，未修改/创建/删除任何文件。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚（R44 后仍 39 条）+ legacy/website-source 逆向契约 + round39~44 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 对上轮观察项逐一核实。`go build ./...`、`go vet ./...` 双通过 exit=0；`go test -race -count=1 ./...` 10 包全绿；新增/改名关键路径相关测试子集独立复跑绿。

---

## CRITICAL

（无本轮新增 CRITICAL。身份防线三族（决策侧 B43-01 / 写回侧 B21-03 / 失效归并 B41-01）在调度器全路径已闭合，未发现可确证的账号级数据污染新通道。）

---

## MAJOR

（无本轮新增 MAJOR。上轮 B43-01/02/04/05 四项实修 + B44-01 日志补强经本轮逐项复核全部正确闭合、无回归，见下节。）

---

## 本轮重点核对（上轮新契约，防回归）—— 5 条全部正确闭合

### 1. B44-01 凭据加密失败日志 —— 已正确闭合
- **位置**：`backend/internal/accounts/manager.go:282-287`（`else { log.Printf("[accounts] 账号 %s 密码加密失败，凭据未落库（自动重登将无保存账密）: %v", acct, err) }`）
- **核实**：else 分支只补日志、不额外执行任何落库/回滚副作用；正常路径（`encrypt` 成功 → `SaveCredential`）完全不受影响（277-281 行原样保留），`err == nil` 分支无行为改动。加密失败仅发生在运行期加密器异常（主密钥损坏启动即拒），日志是唯一审计线索——补日志后与决策锚 17 零吞错对称。唯一微瑕：无对应单测断言该日志（manager_test.go 五个测试均未覆盖 encrypt 失败分支），建议未来补一行假 encrypt 注入测试。

### 2. B43-01 maybeRelogin 入口复核 —— 已正确闭合
- **位置**：`backend/internal/scheduler/scheduler.go:1197-1200`（入口 `s.mu.Lock()` 后、任何 reloginAt/reloginFail/relogging/tokenValid 写入前 `if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`）
- **核实**：判定在锁内、返回也在锁内，无中途写 map；位于 reloginFail++（1221）/reloginAt（1220）/tokenValid（1230）/relogging（1225）全部写入之前。探测定时三处直调路径 ProbeForAccount（821）/ProbeNow（944）/probe（1083）以及手动路径 MaybeRelogin（1312）全部落在统一入口复核下，无旁路。测试 `TestMaybeReloginDeletedAccountSkipsMaps`（scheduler_test.go:3387）在库并复跑绿。

### 3. B43-02 实时复核失效分支 sameClientFor —— 已正确闭合
- **位置**：`backend/internal/scheduler/scheduler.go:1589-1592`（锁外 classFullRealtime 网络段后回锁、进三路分支前统一 `if !s.sameClientFor(acct, chainClient)`）
- **核实**：`sameClientFor` 内含存在性判定（210-215：ClientFor 不存在即 false），无需独立 ClientFor 前置；chainClient 捕获于链顶 1406 行且非 nil。同名重建/删号时序下陈旧链统一在 1589 行静默放弃，不触达失效分支（1597）的 maybeRelogin 与 failed 状态写、不触达满员分支（1610）的 markFullLocked。锁外网络段指针值不回收（Go 安全），回锁后 reflect 指针比对正确。测试 `TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（scheduler_test.go:3421）在库并复跑绿。

### 4. B43-04 管理员撞名双条件 —— 已正确闭合
- **位置**：`backend/internal/api/handler.go:121`（`req.Account == adminName && subtle.ConstantTimeCompare(...)==1`）
- **核实**：管理员签发路径只认双条件；撞名学生走 132 行教务登录成功签发普通会话（issueSession → Sessions.Create，Admin:false）。`requireAdminSession` 判 `Sessions.IsAdmin(tok)`（会话级 Admin 标记），撞名学生普通会话绝不进管理员分支——无残余错位判据。`IsAdminAccountName`（56-58）对 ?account=/删除保护的判据仍按配置名，语义与 B43-04 一致。测试 `TestLoginAdminNameCollisionStudentCredential`（handler_test.go:1442）在库并复跑绿。

### 5. B43-05 handleAdminStats 500 路径 —— 已正确闭合
- **位置**：`backend/internal/api/handler.go:888-907`
- **核实**：targetsCount 循环对任一 `LoadTargetsForAccount` 失败记日志 + 累计 firstErr；循环后 `if targetErr != nil { writeJSONStatus(w, 500, 1, nil, ...) }`——HTTP 500 + body code=1 双通道，仅成功路径累加 len(ts)。测试 `TestAdminStatsTargetsLoadFailureReturns500`（handler_test.go:810）在库并复跑绿。`writeJSONStatus` 只覆盖基础设施/统计错误路径，业务 writeJSON 调用点未扰动。

---

## MINOR

（无本轮新增 MINOR。R44 唯一 MINOR（MINOR-44-01 LoginByPassword 加密失败补日志）已被 B44-01 修复闭合；本轮扫到的两处微偏均为观察级，见下节。）

---

## OBSERVE

### OBSERVE-45-01：`access_limit_cookie` 占位值 `***REMOVED***` 与真实下发值 `1` 不一致（新视角，无功能影响）
- **位置**：`backend/internal/zhidao/client.go:370-371`（`if _, ok := c.cookies["access_limit_cookie"]; !ok { c.cookies["access_limit_cookie"] = "***REMOVED***" }`）、`backend/cmd/probe/main.go:24`（同款占位）
- **说明**：经 Python 二进制直读确认该字面量就是 `***REMOVED***`（非脱敏渲染）。平台登录限流 cookie 真实值为 `1`（login.py/har 实证）；占位值仅影响该 cookie 的取值本身。token 通道由 idToken URL 参数承载（权威通道），平台接口鉴权按 zd_edu_cookie + URL 参数双通道，占位值不阻断——但若平台侧对 access_limit_cookie 值做硬比对，可能触发 cookie 重置（无功能影响，Restore 路径每次启动仍用 `"1"` 占位）。属"防御性占位 + 注释缺"微偏，建议未来统一占位值或补一行注释说明。
- **裁决**：观察级。理由：token 权威通道是 URL 参数（逆向契约），cookie 仅为冗余通道；无任何当前可确证的功能路径依赖该值。

### OBSERVE-45-02：`LoginByPassword` 加密失败分支无单测（B44-01 补强的测试空档）
- **位置**：`backend/internal/accounts/manager_test.go`（五个测试均未注入失败 encrypt）
- **说明**：B44-01 的 else 分支日志是本次修复核心，但无测试断言该分支被触发时正常路径仍落库、失败路径仍有日志。行为已验证正确（人工推演 + 全量测试绿），但属"修复无回归测试"的空档。建议修复代理补一行假 encrypt 注入测试。

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MINOR-44-01 LoginByPassword 加密失败补日志 | 已被 B44-01 修复 | manager.go:282-287 else 分支正确留痕、不影响正常落库路径（见重点核对 1）。 | 已闭合 |
| OBSERVE-43-01 probe() 全局帧与 per-account 帧双槽分叉 | 延续 | probe() 主体用 AnyClient（order[0]）写 `["*"]`（1097），per-account goroutine 经 ProbeForAccount 写 `[acct]`（843）；windowClosedLocked/tick/probeIntervalFor 读 `openTimeForLocked("")`（先 [acct] 后 ["*"] 回退）。全校共享单值契约（HAR 实证）下不实际分叉。 | 延续观察 |
| OBSERVE-43-02 reloginResults cap 8 满丢弃补探测信号 | 延续 | 通道 cap 8（257），重登成功非阻塞发送（1268 select default），tick 主循环消费（673-678）。满丢弃只在并发重登爆炸时发生，刻意权衡。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 每失败 tick 锁外网络段重复 | 延续 | IsClassFull 恒 false（maxCount 平台未下发），非满员失败每 tick 重打 findElectivesStudentCount（1578）。注释已自述防御性路径，快照判满主路径先生效，频率受提交间隔约束。 | 延续观察 |
| OBSERVE-43-04 tick 提交分支无 recover | 延续 | tick（962-1025）→ probe/submitAll→spawnChain goroutine 均无 recover；s.clients 判空在 maybeSyncClock/submitAll/spawnChain 处处有，未确证 panic 路径。属未来回归防御缺口。 | 延续观察 |
| MINOR-43-02 syncFailedWindow/lastSyncStart 死字段 | 延续 | syncFailedWindow（375/392）写而不读但注释留档语义；lastSyncStart（354 写、391 读）实际有读点。 | 延续观察 |
| MINOR-43-03 学生账号教务登录失败路径无固定延迟 | 延续 | 学生账号走教务网络往返天然延迟；管理员名错误口令分支固定 Sleep 300ms（handler.go:137）保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释微偏 | 延续 | reloginBackoff(n) 调用侧先 reloginFail++（1221-1222）再传入，n=1 返 30s、n=2 返 60s；注释与实际一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录（发起侧裸奔） | 延续 | Manager.Relogin（manager.go:167-175）锁内取指针、锁外 ReloginIfNeeded；删号+在途重登并发时平台侧发生一次孤儿登录，新身份短暂用旧 token。同构窗口存在于 classFullRealtime（1578 锁外用旧指针）。频率极低、无数据污染。 | 延续观察 |
| M40-01 本地钟混用（退避/节流 time.Since） | 延续 | maybeRelogin 退避/节流（1209/1216/1220）本地钟读写同基内部自洽，偏差 ≤640ms 对 30s 节流无实质危害。 | 延续观察 |
| m40-02 锁内多取 now | 延续 | spawnChain 1430 行锁内取一次 nowAlignedLocked，亚毫秒统计口径噪音。 | 延续观察 |
| o40-01~04 / m39-02 / o39-02~05 | 延续 | 逐一复核：未激活不发会话无残留 / reloginResults 满丢弃刻意权衡 / columnExists 拼接全编译期常量 / interval≤0 生产恒 300ms / 快照 TTL 读侧本地钟（≤640ms vs 40s 低危害） / submitAll 快照与在飞链竞态（无撤销语义，平台最终把关） / vision key 空串无法清空 / NAT 出口登录限流合并 / tick 无 recover（同 OBSERVE-43-04）。 | 全部延续观察 |

---

## 本轮新视角扫查（store/db/session/secure/runtime/zhidao/browser）

- **store/db 层**：`SetMaxOpenConns(1)` 单写者串行化下无并发读锁等待；WAL + busy_timeout(5000) 覆盖写锁等待。所有事务均 `defer tx.Rollback()`（回滚安全）且错误路径直接 return；`CreateActivationCodes`/`SaveSettings`/`DeleteAccount`/`SetTargetsForAccount`/`ConsumeActivationCode` 五处事务的 commit 错误均如实返回，无吞错。`migrateAddPublishMeta` 幂等性实证：逐列 `columnExists` 判定，已存在直接 continue；`TestMigrateAddsPublishMetaColumns`（db_test.go:34）覆盖缺列自动补齐 + 旧数据保留 + 重复 Open 不报错。`refuseLegacy` 缺列清单已对应剔除已迁移列（targets.priority/allow_swap/task_log.account 不含 publish_name/begin_date）——无冲突。**结论：无新增问题。**
- **session 层**：票据 TTL 5 分钟惰性过期（ConsumeTicket 内判 + sweepLoop 每 5 分钟清扫）；ConsumeTicket 在锁内完成存在性/未用/过期/账号匹配四查后置 used+delete，两并发 activate 同一票据必一个成功一个报"已使用"——无竞态（F13-m1 契约确认）。Create 后立即 Account 读取正常。sweeperClose sync.Once + sweeperStop/sweeperDone 收口，无 goroutine 泄漏。**结论：无新增问题。**
- **secure 层**：`maskKey` 对空串返空、len≤4 返 `"****"`，`v[len(v)-4:]` 仅在 len>4 分支执行——无切片越界 panic。Encrypt 随机 nonce 前置 + GCM 认证；Decrypt 校验密文长度非法分支。LoadOrCreateKey 对 env 与文件两侧均 32 字节校验。**结论：无新增问题。**
- **runtime 层**：Update 在写锁内应用闭包，Get 在读锁内返回完整拷贝——并发 PUT/GET 下 Get 要么拿到旧值要么拿到新值，无半态。handleAdminConfig PUT 先 Update（内存生效）再 saveSettings（落库）最后 dispatchRuntimeConfig（下游热下发），三条路径（含落库失败）均不静默。**结论：无新增问题。**
- **zhidao 层**：ErrUnauthorized 判定仅 `j.Code == -1`（doRequest:425）——平台契约中 code=-1 是唯一未登录码（checkResp 状态码机），code=1 业务失败不误判（SelectClass/ExitClass 走 `code!=0 || !isOk` 独立判定）。URL 拼接 `baseURL+path+"?idToken="+url.QueryEscape(tok)` 恒定 `?` 前缀（全部 POST、无 query 混叠）；idToken 双通道（URL 参数 + zd_edu_cookie）与逆向契约一致。**结论：无新增问题。**
- **browser 文件**：`browser_windows.go`（windows build tag）与 `browser_unix.go`（!windows）互斥，各自实现 openBrowser，无死代码；garble/构建标签路径（native_ocr.go windows+cgo / native_ocr_stub.go !windows||!cgo）互斥完整，无残留。cmd/probe、cmd/logintest、cmd/bench 均只读、无副作用。**结论：无新增问题。**
- **Restore 解密静默弱路径**：`manager.go:299-302` 对 decrypt 失败（主密钥变更后凭据不可解密）静默置空密码继续恢复——自动重登将"无保存账密"，与 B44-01 的"加密失败必须留痕"不对称。但主密钥变更本身是拒绝启动场景（LoadOrCreateKey 校验），运行期不可达；且无日志会误导运维排查自动重登失败原因。**观察级微偏**，建议补 log（与 B44-01 对称）。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查/测试**：`go build ./...`、`go vet ./...` 双通道 exit=0；`go test -race -count=1 ./...` 10 包全绿（含 scheduler -race）；B43/B44 相关测试子集独立复跑绿。
- **identity 复核族全闭合**：spawnChain 六分支（成功 1510/失效 1478/风控 1540/窗口关闭 1560/实时复核满员 1624/实时复核失效 1589）全部 sameClientFor 前置；maybeRelogin 入口（B43-01）决策侧复核；MarkDone/RemoveDone/重登 goroutine 写回侧（B18-M2/B20-01/B21-03）——决策+写回+失效归并三族闭合，无裸露写点。
- **WindowClosed 三判据单源** `windowClosedLocked()`（907-928）：主判据（+10s 裕量）/时钟失败 ≥3（带"开放时间已过"）/幽灵窗口 EmptyProbeRuns≥3（+10s 裕量），StateForAccount 与 WindowClosed()/handleAdminStats(window_closed) 共用同源。
- **openTime 识别槽三硬契约**：识别槽保留（空快照不删，ProbeForAccount 841-846 / probe 1095-1100 只在非空 beginTimes 时写）；展示层过期判定独立（StateForAccount 712-714）；写入持锁。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1000）语义正确，WindowOpened=true + 识别槽空时提交不放挂；既有测试 TestSubmitSuspendedWhenOpenTimeCleared 仍绿。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow；probing 单飞锁内置位-检查（1038-1044）；probeSem cap 4（1058-1066）；tick 首探豁免（978）与 submitIntervalFor 250ms 冲刺（1020）。
- **多账号年级隔离**：probe() 每账号独立 goroutine + ProbeForAccount 专属帧；ElectivesSnapshotFor 目标账号过期专属帧 → (nil,false) 触发真刷新，绝无年级串线回退。
- **doLogin 闸门全收口**：gateTryAcquire（manager.go:223-235，非阻塞准入）与 gateWait（阻塞排队）共享 gateMu/gateUsed 计数，LoginByPassword 与 Relogin 无旁路（B42-01）；GatePump 每分钟推进。
- **鉴权与数据安全**：会话 12h TTL 惰性失效 + 5min 清扫；requireAdminSession/requireAuth 写真实 HTTP 状态码（403/401）；SpaHandler /api 精确+前缀 404（embed.go:28-31）；XFF 仅回环+XUANKE_TRUSTED_PROXY=on 信任最右非空。
- **SQL**：全参数化绑定；columnExists 拼接全常量；migrateAddPublishMeta 增量迁移幂等 + refuseLegacy 缺列清单对应；db.Open SetMaxOpenConns(1) 串行化 + busy_timeout 覆盖写锁等待。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 1024 位公钥硬编码；uniqueDeviceID UA|Win32|881|1410|时间戳36进制 base64；captcha 识别并发信号量 + 3 次重试收敛；ErrUnauthorized（code=-1）统一由 doRequest 抛出。
- **凭据加密**：AES-256-GCM + 随机 nonce 前置；`.master_key`/env 32 字节校验；enc: 前缀；旧明文拒绝加载（main.go:87-95）。LoginByPassword 登录成功后 SetCredentials 写客户端内部账密（B6-01），重启 Restore 重建。
- **cmd 工具**：probe 读 XUANKE_PROBE_TOKEN（无硬编码 token）；logintest 跟随识别引擎 + wait 间隔符合平台限流；bench 串行压测 401/真实路径。均只读、无副作用。

---

## 结论

- 未发现新增 CRITICAL/MAJOR。上轮 4 条 MAJOR（B43-01/02/04/05）+ 1 条 MINOR（B44-01 日志补强）经本轮逐项复核全部正确闭合、无回归；身份防线（决策侧+写回侧+失效归并）三族闭合后，删号-同名重建竞态在调度器路径已全部封死。
- 上轮观察项 14 条全部复核：MINOR-44-01 已闭合；其余 13 条（含 OBSERVE-43-01~04、MINOR-43-02~04、MAJOR-42-02 孤儿登录、M40-01、m40-02、o40-01~04、m39-02、o39-02~05）全部延续观察，无升级。
- 新增观察级 2 条：OBSERVE-45-01（access_limit_cookie 占位值 `***REMOVED***` 与真实下发值 `1` 不一致，无功能影响）、OBSERVE-45-02（B44-01 加密失败分支无单测）。均不阻塞，可留给后续轮次顺手处理。
- 残余风险集中在两条延续观察：孤儿登录的平台侧副作用（MAJOR-42-02，频率极低、无数据污染）与 tick 无 recover（防御性缺口，无当前可触发路径）。
