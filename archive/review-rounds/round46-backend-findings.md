# round46 后端审查原始发现

> 审查基线：master @ `338efb8`（R45 收敛落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式，未修改/创建/删除任何文件。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚（R45 后仍 42 条）+ legacy/website-source 逆向契约 + round39~45 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 对上轮观察项逐一核实 + 本轮新视角扫查。`go build ./...`、`go vet ./...` 双通道 exit=0；`go test -race -count=1 ./...` 主体全绿（api/zhidao 偶发 httptest 连接 flake，见 MAJOR 节）。

---

## CRITICAL

（无本轮新增 CRITICAL。身份防线三族（决策侧 B43-01 / 写回侧 B21-03 / 失效归并 B41-01）在调度器全路径仍闭合，未发现可确证的账号级数据污染新通道。）

---

## MAJOR

### MAJOR-46-01：`go test ./...` 并行执行时 api/zhidao 包偶发 httptest 连接 flake（测试基础设施层，非业务逻辑）

- **位置**：`backend/internal/api/handler_test.go`（TestAdminDeleteAccountMemoryFirst / TestStudentSetTargetsWithoutAccountOK / TestAdminConfigHotReload / TestAdminStatsAccountsLogs / TestLoginActivateSeparateBuckets）、`backend/internal/zhidao/*_test.go`（登录链路测试）
- **一句话问题**：全量 `go test -race -count=1 ./...` 10 包并行跑时，api 与 zhidao 两包偶发失败——失败形态统一为 httptest mock server 的 `dial tcp 127.0.0.1:<port>: connectex: A connection attempt failed`（连接建立失败/超时，TestAdminStatsAccountsLogs 一次 15.83s 超时）或断言窗口内未完成。**每个失败测试单独复跑 / 连跑 3~5 轮均全绿**（实测记录：首轮全量 2 个 api 测试失败→单跑绿；api 连跑 5 轮 1 次失败（TestAdminConfigHotReload）→单跑绿；zhidao 并发跑失败 1 次→连跑 3 轮绿；api 连跑 6 轮 5 绿 1 红（红者为 TestAdminStatsAccountsLogs 与 TestLoginActivateSeparateBuckets 中随机一个）→各自单跑 3 轮全绿）。失败测试的共性：**全部走真实登录链路（httptest mock + Vision 识别 + doLogin 网络往返）**——该路径的测试对连接建立延迟最敏感。
- **归因推演**：Windows 宿主 + `go test ./...` 并行起 10 个测试进程，每个进程又各自建多个 httptest server；机器在并行编译/测试峰值时端口与连接队列争抢，偶发 `httptest.NewServer` 已就绪但 `c.http.Do` 连接建立被拒/超时；失败测试共性走真实登录网络往返链路（对连接延迟最敏感）。失败断言全部指向"mock server 请求发起失败"，无任何业务断言失败（无 code/状态断言错），无 -race 报告。属测试环境资源竞争 flake，非代码缺陷。
- **为什么给 MAJOR 而非观察**：这是 CI（.github/workflows/ci.yml `go test -v ./...`）的**假红源头**——R45 起的双收敛轮全量测试都报"全绿"，本轮首跑即踩，若在 CI 上触发会让合并被无谓阻断。且测试对网络 mock 的容错（重试 / 连接预热）为零，属基础设施层的真实薄弱点。建议：CI 可加 `-p 2`（限制并行包数）或对登录链路测试加首请求重试；不做业务代码改动。
- **裁决**：MAJOR（测试基础设施层），业务代码零改动需求。

---

## 本轮重点核对（上轮新契约，防回归）—— 全部正确闭合

### 1. B45-N2/N3（Restore 解密日志 + 加密失败回归钉）—— 已正确闭合
- **位置**：`backend/internal/accounts/manager.go:295-317`（Restore）+ `:282-287`（LoginByPassword 加密失败 else 分支）；测试 `TestLoginByPasswordEncryptFailLogs`（manager_test.go:240-269）
- **核实**：Restore 解密失败 else 分支（302-307）只补日志、不影响正常恢复路径（299-301 解密成功原样 SetCredentials）；与 LoginByPassword 加密失败日志（282-287）对称。**新测试真红假绿核验**：注入假 encrypt 恒返 error → 登录成功返回 token、`fakeStore.saved==false`、doLogin 恰好 1 次——测试直接断言"加密失败不阻断登录 + 凭据不落库"，非仅查日志字符串，判定真实。单跑 `go test -race -run TestLoginByPasswordEncryptFailLogs ./internal/accounts/` 绿（日志实测输出"密码加密失败，凭据未落库"）。B44-01 遗留的"加密失败分支无单测"空档（OBSERVE-45-02）已被本轮新测试闭合。

### 2. B45-N1（前端 401 双广播收敛，后端侧契约确认）—— HTTP+body 双通道语义自洽
- **位置**：`backend/internal/api/handler.go:1076-1080`（requireAuth）、router.go 各基础设施路径
- **核实**：requireAuth 写 `writeJSONStatus(w, 401, 401, nil, ...)`——HTTP 401 + body code=401 双通道，前端 `client.ts` 以 HTTP 层广播 + body 层广播两段消费（B45-N1 已收敛重复广播，前端修复不在本包）。后端侧 writeJSONStatus 家族契约（B39-02）未变：仅 panic 500 / 会话 401 / 管理 403 / 限流 429 / requireJSONBody CSRF 403 五族走真实状态码，其余 100+ 业务 writeJSON 调用点仍 HTTP 恒 200。body code 与 HTTP 状态码恒一致（403/401/429/500），无"HTTP 200 + body 401"或"HTTP 401 + body 0"的错位形态。测试 `TestAuthRequired`（handler_test.go:242-258）断言 HTTP 401 + body 401 双通道——自洽。

### 3. B43-01/02/04/05 + B44-01 五项实修 —— 全部复核正确闭合、无新竞态
- **B43-01**：maybeRelogin 入口（scheduler.go:1197-1200）`ClientFor` 复核在锁内、位于全部 relogin 族 map 写入之前；探测定时三处（ProbeForAccount 821 / ProbeNow 944 / probe 1083）全落统一入口。测试 `TestMaybeReloginDeletedAccountSkipsMaps`（3387）绿。
- **B43-02**：实时复核失效分支（1589-1592）`sameClientFor` 前置，内含存在性判定；`TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin`（3421）绿。
- **B43-04**：管理员撞名双条件（handler.go:121）`req.Account==adminName && ConstantTimeCompare==1`；撞名学生走教务登录（132）。`TestLoginAdminNameCollisionStudentCredential`（1442）绿。
- **B43-05**：handleAdminStats 500 路径（888-907）targetsCount 失败累计 firstErr → 循环后 writeJSONStatus(500)；`TestAdminStatsTargetsLoadFailureReturns500`（810）绿。**注**：该测试当前不注入失败（注释自述"实现内复查"，仅测正常路径 200），失败分支 500 无真断言——B43-05 的 500 语义靠代码复查背书，非测试红绿钉。观察级留档。
- **B44-01**：加密失败补日志（manager.go:282-287），见重点核对 1。

---

## MINOR

（无本轮新增 MINOR。R45 无遗留 MINOR；本轮扫到的三处偏项均为观察级或测试基础设施层，见下。）

---

## OBSERVE

### OBSERVE-46-01：scheduler.interval 非正数无兜底——`time.NewTicker(非正)` 直接 panic（新视角实证）
- **位置**：`backend/internal/scheduler/scheduler.go:665`（`ticker := time.NewTicker(s.interval)`）
- **说明**：本轮实证 `time.NewTicker(0)` 抛 panic（`non-positive interval for NewTicker`）。生产 main.go:113 恒传 `300*time.Millisecond`，测试全部传 >0——**无任何生产路径可达**，且 interval 无 clamp/校验。属与"tick 无 recover"（OBSERVE-43-04）同族的"未来回归防御缺口"：若未来有人传 0/负值（时间操控/配置化），Start() 协程 panic 直接崩掉整个调度器且无 recover 兜底。建议：`New` 内 `if interval <= 0 { interval = 300 * time.Millisecond }` 一行防御。
- **裁决**：观察级（无当前可触发路径）。

### OBSERVE-46-02：zhidao 客户端不消费 `Retry-After`/429 响应头，风控退避固定 30s
- **位置**：`backend/internal/zhidao/client.go:379-429`（doRequest 只解 body code）、`scheduler.go:1534-1552`（isRateLimitError 命中后 markRateLimitedLocked 固定 30s）
- **说明**：平台风控若返回标准 HTTP 429 + `Retry-After` 头，doRequest 不读该头（只解析 body JSON），调度器侧统一 30s 退避不随服务端指示调整。当前平台契约实证为 body 文案（"操作过于频繁，请稍后重试"）且 HAR 无 429 头样本——isRateLimitError 文案匹配已覆盖，30s 固定退避是刻意保守值。若平台未来改为标准 429+Retry-After，30s 固定值可能偏短（退避未满即重试）或偏长（浪费黄金期）。属防御性未来契约缺口。
- **裁决**：观察级。

### OBSERVE-46-03：task_log 无容量上限与清理（历轮观察延续）
- **位置**：`backend/internal/store/store.go:200-209`（AppendLog 无条件 INSERT）、schema.sql:33-42（task_log 无唯一键/无 TTL）
- **说明**：LoadLogs/LoadAllLogs 有读限（100/1000/500）但无任何清理路径。黄金期失败重试期（非满员失败每 tick AppendLog 一行）长运行数月后 task_log 行数可到数十万级，`LoadAllLogs(1000)` 走 `ORDER BY id DESC` 无索引扫描（id 主键倒序仍全表排序）。公网长期部署下 DB 文件持续膨胀。历轮观察项（o39-05 同族），本轮复核无新增恶化，仍无清理路径。
- **裁决**：延续观察。建议未来 `AppendLog` 加周期 `DELETE FROM task_log WHERE id < (max_id - N)` 或按天保留。

### OBSERVE-46-04：handleSetTargets 不校验 class_id 重复与"publish_id 属于该账号年级"
- **位置**：`backend/internal/api/handler.go:490-507`（校验：条数≤100 / class_id>0 / publish_id>0 / priority 0-999，无重复与跨年级校验）
- **说明**：目标由前端从该账号的课程数据联查构建（前端 F15 契约），后端不校验"同一 class_id 重复"（两条同课目标按 priority 排序后 submitAll 先去重 byPub 内同 classID——spawnChain 第一门成功即终止链，重复目标无实际危害）与"publish_id 是否属于该账号年级"（管理员透传跨年级 publish_id 会写入目标，提交时 CheckClassSelectable 用该账号快照复核窗口/满员，快照查无此课则放行给平台把关——平台最终把关正确，无年级串线数据污染）。两条均为"管理员是可信主体 + 平台最终把关"的有意边界，非缺陷。
- **裁决**：观察级。

### OBSERVE-46-05：B43-05 的 500 分支无真断言（测试空档延续）
- **位置**：`backend/internal/api/handler_test.go:810-826`（TestAdminStatsTargetsLoadFailureReturns500 只测正常路径 200，注释自述"失败路径走实现内复查"）
- **说明**：注释给出的原因是"Register 接收 *store.Store 非接口，无法注入替身"——但文件 828-835 行已有 `failingTargetsStore` 包装类型（包裹 Store 覆写 LoadTargetsForAccount 恒失败），说明注入路径其实可行、只是未接到该测试上（failingTargetsStore 在测试文件里定义了却无消费点）。B43-05 的 500 语义当前靠实现复查背书，无红绿钉。
- **裁决**：观察级（建议修复代理把 failingTargetsStore 接上该测试——顺手一行）。

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-45-01 access_limit_cookie 占位值 `***REMOVED***` 与真实下发值 `1` 不一致 | 观察级 | submitLogin（client.go:370-371）与 cmd/probe/main.go:24 同款占位；Restore 路径用 `"1"`（manager.go:313）。token 权威通道是 URL 参数（逆向契约），占位值无功能影响。 | 延续观察 |
| OBSERVE-45-02 加密失败分支无单测 | 已被 B45-N3 修复 | `TestLoginByPasswordEncryptFailLogs`（manager_test.go:240-269）现为真实红绿钉（非仅查日志），见重点核对 1。 | 已闭合 |
| OBSERVE-43-01 probe() 全局帧与 per-account 帧双槽分叉 | 延续 | probe() 主体写 `["*"]`（1097）、per-account goroutine 写 `[acct]`（843）；openTimeForLocked 先 [acct] 后 ["*"] 回退。全校共享单值契约（HAR 实证）下不实际分叉。 | 延续观察 |
| OBSERVE-43-02 reloginResults cap 8 满丢弃 | 延续 | 通道 cap 8（257），非阻塞发送 select default（1268），tick 主循环消费（673-678）。满丢弃仅在并发重登爆炸时，刻意权衡。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 每失败 tick 锁外网络段重复 | 延续 | IsClassFull 恒 false（maxCount 平台未下发），注释自述防御性路径，快照判满主路径先生效。 | 延续观察 |
| OBSERVE-43-04 tick 提交分支无 recover | 延续 | tick（962-1025）→ probe/submitAll/spawnChain goroutine 均无 recover；本轮新增 OBSERVE-46-01（interval 非正 panic）与之同族。未确证 panic 路径。 | 延续观察（与 46-01 合并建议） |
| MINOR-43-02 syncFailedWindow/lastSyncStart 死字段 | 延续 | syncFailedWindow（375/392）写而不读但注释留档语义；lastSyncStart 有读点。 | 延续观察 |
| MINOR-43-03 学生账号教务登录失败路径无固定延迟 | 延续 | 学生账号走教务网络往返天然延迟；管理员名错误口令分支固定 Sleep 300ms（handler.go:137）保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释微偏 | 延续 | 调用侧先 reloginFail++ 再传入，n=1 返 30s、n=2 返 60s；注释一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录（发起侧裸奔） | 延续 | Manager.Relogin（manager.go:167-175）锁内取指针、锁外 ReloginIfNeeded；删号+在途重登并发时平台侧一次孤儿登录。同构窗口 classFullRealtime（1578 锁外用旧指针）。频率极低、无数据污染。 | 延续观察 |
| M40-01 本地钟混用（退避/节流 time.Since） | 延续 | maybeRelogin 退避/节流（1209/1216/1220）本地钟同基内部自洽，偏差 ≤640ms 对 30s 节流无实质危害。 | 延续观察 |
| m40-02 锁内多取 now | 延续 | spawnChain 1430 行锁内取一次 nowAlignedLocked，亚毫秒统计口径噪音。 | 延续观察 |
| o40-01~04 / m39-02 / o39-02~05 | 延续 | 逐一复核：未激活不发会话 / reloginResults 满丢弃 / columnExists 全编译期常量 / interval≤0 生产恒 300ms（本轮实证 NewTicker 非正 panic，见 46-01，建议补 clamp）/ 快照 TTL 读侧本地钟 / submitAll 快照与在飞链竞态 / vision key 空串无法清空 / NAT 出口登录限流合并 / tick 无 recover / task_log 无清理（本轮新证 46-03）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **tick 主循环（962-1025）**：interval≤0 无 clamp 兜底（见 46-01）；`submitIntervalFor` 黄金期压缩边界正确——`open.IsZero()` 不进冲刺分支（零值恒返 1s）、`now.Before(open)` 未开窗返 1s、黄金期 10s 内返 250ms、过后返 1s，负值不可能（编译期常量）；`!lastSubmit.IsZero() && now.Sub(lastSubmit) < submitInterval` 闸门正确。
- **多账号 order 一致性**：order 由 accounts.Manager 维护（ensure 追加 / Remove 删除，manager.go:125-135 遍历删除）；调度器 acctTargets/done/full/inflight/refused 全部按账号 map 隔离，与 order 无耦合——submitAll 遍历时用 `ClientFor` 过滤已删账号（1344），spawnChain 按 `acct+publishID` 键防重。重新登录同名账号 ensure 返回既有客户端不重复 append order；删号后重建 ensure 重新 append。**order 变化不破坏 acctTargets/inflight/done 一致性**。
- **handleSetTargets 校验**：条数≤100 / class_id>0 / publish_id>0 / priority 0-999 全覆盖（490-507），重复与跨年级不校验属有意边界（见 46-04）。
- **AppendLog 容量**：无上限与清理（见 46-03）。
- **secure token 随机源与恒定时比对**：randToken（session/store.go:224-231）crypto/rand 32 字节；激活码（handler.go:681-688）crypto/rand 8 字节 64bit 熵；管理口令（config.go:13-19）crypto/rand 12 字节；三处 rand 失败均 panic 拒签发。`subtle.ConstantTimeCompare` 调用点全仓库仅 handler.go:121 一处（管理员口令比对）——比对调用点全覆盖无遗漏。
- **zhidao RetryAfter**：不消费 429 头（见 46-02）。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查/测试**：`go build ./...`、`go vet ./...` 双通道 exit=0；`go test -race -count=1 ./...` 主体全绿（10 包中 api/zhidao 偶发 httptest 连接 flake 见 MAJOR-46-01，失败测试单跑全部绿）；B45/B43/B44 相关测试子集独立复跑绿。
- **identity 复核族全闭合**：spawnChain 六分支（成功 1510 / 失效 1478 / 风控 1540 / 窗口关闭 1560 / 实时复核满员 1624 / 实时复核失效 1589）全部 sameClientFor 前置；maybeRelogin 入口（B43-01）决策侧复核；MarkDone/RemoveDone/重登 goroutine 写回侧（B18-M2/B20-01/B21-03）——决策+写回+失效归并三族闭合，本轮未发现新裸露写点。
- **WindowClosed 三判据单源** `windowClosedLocked()`（907-928）：主判据（+10s 裕量）/时钟失败 ≥3（带"开放时间已过"）/幽灵窗口 EmptyProbeRuns≥3（+10s 裕量），StateForAccount 与 WindowClosed()/handleAdminStats(window_closed) 共用同源；开窗点 10s 裕量入账与判定同源单快照复用。
- **openTime 识别槽三硬契约**：识别槽保留（空快照不删）；展示层过期判定独立；写入持锁。TestOpenTimeRetainedAfterWindowClosed / TestAccountOpenTimeDetection 绿。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1000）语义正确；TestSubmitSuspendedWhenOpenTimeCleared（1330）/TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime（3359）双绿。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow；probing 单飞锁内置位-检查（1038-1044）；probeSem cap 4（1058-1066）；tick 首探豁免（978）与 submitIntervalFor 250ms 冲刺（1020）。
- **多账号年级隔离**：probe() 每账号独立 goroutine + ProbeForAccount 专属帧；ElectivesSnapshotFor 目标账号过期专属帧 → (nil,false) 触发真刷新，绝无年级串线回退。
- **doLogin 闸门全收口**：gateTryAcquire（manager.go:223-235，非阻塞准入）与 gateWait（阻塞排队）共享 gateMu/gateUsed 计数；LoginByPassword 与 Relogin 无旁路；GatePump 每分钟推进。
- **鉴权与数据安全**：会话 12h TTL 惰性失效 + 5min 清扫；requireAdminSession/requireAuth 写真实 HTTP 状态码（403/401）；SpaHandler /api 前缀 404（embed.go:28-31）；XFF 仅回环+XUANKE_TRUSTED_PROXY=on 信任最右非空。
- **SQL**：全参数化绑定；columnExists 拼接全常量；migrateAddPublishMeta 增量迁移幂等 + refuseLegacy 缺列清单对应；db.Open SetMaxOpenConns(1) 串行化 + busy_timeout(5000)。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 1024 位公钥硬编码；uniqueDeviceID 复刻逐字段一致；captcha 识别并发信号量 + 3 次重试收敛；ErrUnauthorized（code=-1）统一由 doRequest 抛出；cookie/idToken 双通道与逆向契约一致；`submitLogin` 的 `access_limit_cookie` 占位值（见 46-01 延续 OBSERVE-45-01）。
- **凭据加密**：AES-256-GCM + 随机 nonce 前置；`.master_key`/env 32 字节校验；enc: 前缀；旧明文拒绝加载（main.go:87-95）。
- **cmd 工具**：probe 读 XUANKE_PROBE_TOKEN（无硬编码 token）；logintest 跟随识别引擎 + wait 间隔符合平台限流；bench 串行压测 401/真实路径。均只读、无副作用。

---

## 结论

- **MAJOR 1（测试基础设施层）/ MINOR 0 / OBSERVE 5（3 新 + 2 延续）**，共 6 条。
- 最重 3 条（按影响排序）：
  1. **MAJOR-46-01（httptest 连接 flake）**——`go test ./...` 并行跑时 api/zhidao 包偶发 `connectex` 连接失败，失败测试单跑全绿、无业务断言失败。CI 假红源头，建议 CI 限 `-p` 或登录测试首请求重试，不改业务代码。
  2. **OBSERVE-46-01（interval 非正 panic）**——`time.NewTicker(0)` 实证 panic，生产恒 300ms 不可达，与 tick 无 recover 同族防御缺口，一行 clamp 可堵。
  3. **OBSERVE-46-03（task_log 无清理）**——长期部署 DB 膨胀，历轮观察延续。
- 上轮观察项 15 条全部复核：OBSERVE-45-02（加密失败单测）已被 B45-N3 闭合；其余 14 条（含 MAJOR-42-02 孤儿登录、OBSERVE-43-01~04、MINOR-43-02~04、M40-01、m40-02、o40-01~04、m39-02、o39-02~05）全部延续观察，无升级。
- B45-N2/N3 + B45-N1（后端侧）+ B43-01/02/04/05 + B44-01 全部正确闭合无回归；identity 三族防线在调度器路径仍全闭合。
- 残余风险集中在三条延续观察：孤儿登录平台侧副作用（MAJOR-42-02）、tick 无 recover + interval 非正 panic（OBSERVE-43-04 + 46-01）、task_log 无清理（46-03）。
