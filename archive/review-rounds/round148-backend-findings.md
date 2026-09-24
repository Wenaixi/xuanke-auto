# R148 后端只读审查 —— 身份防线矩阵第六十三轮

审查基线与工作树
- 基线提交：237edb8（R147 归档，身份防线矩阵第六十二轮闭合）
- 审查时间：2026-09-24
- 审查范围：backend/internal/{scheduler,api,accounts,zhidao,secure,session,store} 全部相关源码/测试 + backend/main.go
- 模式：绝对只读，零代码改动（唯一写文件即本报告）

## 结论前置

| 级别 | 数量 | 汇总 |
|------|------|------|
| CRITICAL | 0 | 无 |
| HIGH | 0 | 无 |
| MEDIUM | 0 | 无 |
| LOW | 3 | 维持观察项（warnedNoTargets 无锁写 / 两处 `_ =` 忽略返回 / accounts 测试无 socketPreheat 双保险），均延续历史论证覆盖，无实际危害 |

**最终裁决：APPROVE（通过归档）**

## 验证表（全部实测）

| 验证项 | 命令 | 结果 | 实测耗时 |
|--------|------|------|----------|
| 编译 | `go build ./...` | 双绿（exit 0） | 实测 |
| 静态检查 | `go vet ./...` | 双绿（exit 0，无输出） | 实测 |
| 竞态 zhidao | `export PATH=/d/mingw64/bin:$PATH` + `go test -race ./internal/zhidao/ ./internal/accounts/` | ok 2.330s / 1.724s | 实测 |
| 竞态 scheduler+api | 同上 `./internal/scheduler/ ./internal/api/` | ok 15.460s / 16.641s | 实测 |
| 身份防线族十测（-race 定向，-v） | `go test -race -run 'TestDeletedAccountRebuiltSameNameChain\|TestMaybeReloginDeletedAccountSkipsMaps\|TestWindowOpenSubmitsWithoutProbeReset\|TestScheduleIntervalClamped'` ./internal/scheduler/ | 10 测全 PASS（成功/失效/风控/窗口关闭/确证满员/实时复核失效/决策侧四 map + 回归锚 + clamp） | 实测 3.234s |
| 回归锚 scheduler | `go test -race -run TestWindowOpenSubmitsWithoutProbeReset` ./internal/scheduler/ | PASS（1.05s） | 实测 |
| 回归锚 api | `go test -run TestAdminStatsWindowOpenedUsesScheduler` ./internal/api/ | PASS（0.39s） | 实测 |
| 脱敏判型族 | `go test -race -run 'TestSanitizeError\|TestDoRequestSanitizesDialError'` ./internal/zhidao/ | ok（1.417s） | 实测 |
| 轮次标签扫描（产品） | `grep -rn "第.*轮\|R[0-9][0-9]轮\|round [0-9]"` 全后端非测试 | 零命中 | 实测 |
| 轮次标签扫描（测试） | 同正则 `*_test.go` | 5 处"第 3 轮/第 %d 轮"属时钟同步轮询/快照轮数叙述，非轮次前缀标签，合规 | 实测 |
| 工作区漂移 | `git status --short` + `git status --porcelain` | 仅 r148-frontend-findings.md untracked（前端代理并发产物），后端零漂移 | 实测 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十三轮闭合 —— ✅ 在位，零漂移

**sameClientFor 定义与 7 调用点逐一核实（scheduler.go，行号与 R147 完全一致——基线至 HEAD 无代码改动）**

- 定义 :204-210：`ClientFor(acct)` 存在性 + `clientIdentity(current) == clientIdentity(chainClient)` 反射指针比对（:215-224，`reflect.ValueOf(c).Pointer()`，nil 返回 0）。注释明确"需持 s.mu"。
- grep 全仓库 `sameClientFor` 命中：定义 1 + 调用 7 处（:850/:1489/:1521/:1551/:1571/:1600/:1635），无第八处（:846 为注释引用非调用）。测试侧两处引用均为同名重建族测试注释叙述。

| 调用点 | 分支 | 写什么状态/落什么库行 | 终局核证 |
|--------|------|----------------------|----------|
| :850 | ProbeForAccount 回写段（M88-01 探测身份防线） | `openTimeDetected[acct]`（非空 beginTimes 才写 :855-858）+ `acctData[acct]` + `acctDataAt[acct]`（nowAligned）。复核失败整体放弃回写返回 `(data,nil)` | 删号/同名重建期间在飞探测绝不污染重建账号识别槽与快照 |
| :1489 | spawnChain 失效分支（ErrUnauthorized） | 复核失败：清 inflight + 静默 return。重登决策（:1499 maybeRelogin）必须落在身份闸之后（:1494-1498 注释实证） | 旧链命中新身份失效绝不触发重登、绝不为新身份写失败计数 |
| :1521 | spawnChain 成功分支 | `done[acct][classID]=true` + `setStateLocked("success")` + AppendLog(:1532) + SaveSuccess(:1535)，全 `if err != nil { log.Printf }` | 假 success 行/重启假成功被拦 |
| :1551 | 风控退避 isRateLimitError | `markRateLimitedLocked(30s)` + `setStateLocked` + AppendLog(:1558) | 假退避吞黄金期被拦 |
| :1571 | 窗口关闭 isWindowClosedError | `markFullLocked` | 假"已满员"永久退避被拦 |
| :1600 | 实时复核回锁入口（B43-02 第六分支统一身份闸） | 三路判据（cErr ErrUnauthorized→maybeRelogin / 确证满员→markFullLocked / 未现满员 failed）之前 | 实时复核命中新身份失效不写新身份 |
| :1635 | 实时复核确证满员分支 | `markFullLocked`；前置 `doneHas`（:1627 手动成功让位，绝不覆盖胜利状态） | 陈旧链反向写 full 被拦 |

**maybeRelogin 双侧（OBSERVE-117-01 知识位第三十一轮）确认到位**

- 决策侧 :1208：`ClientFor(acct)` 存在性复核，已删账号不发起重登、不写任何 relogin 族 map（TestMaybeReloginDeletedAccountSkipsMaps 固化 tokenValid/reloginFail/reloginAt/relogging 四 map 零 key 契约，实测 PASS 0.00s）。
- 写回侧 :1254：重登完成先 `ClientFor(acct)` 复核，已删则整个成功分支（含内存写 + UpdateIDToken 落库）静默放弃、只清 relogging。
- :1265-1273 二次 `ClientFor(acct)` 重取**当前注册表** client → `client.Token()` 非空才 `store.UpdateIDToken` 落库。同名重建场景：前置 :1254 在同一把锁同一次持有内先拦（注册表指针已换即放弃整个成功分支），二次重取属于"重登成功者自身身份"的 token——旧链新身份组合不可能发生。知识位在位，三十一轮无漂移。

**手动五路 accountExists（handler.go）**

- :255 课程读（handleElectives）
- :305 手动报名（handleElectiveSelect）
- :397 手动退选（handleElectiveExit）
- :497-512 目标写（handleSetTargets）——内联 `LoadCredentials` 逐账号比对，注释 :494-497 明示"仅 accounts 表会误伤 authenticateDirect 直连的已登录账号；判据与 accountExists :1109-1120 同源——凭据表真理源"
- :573 状态读（handleState）

五路全到位，目标写内联形态与 accountExists（:1109-1120 LoadCredentials 逐账号比对）语义等价。

**写点换类 5 类 + warnedNoTargets 宿主唯一性（逐写点锁核证）**

- `lastSubmit`：submitAll :1346 锁内 `nowAlignedLocked()` 写入；读侧 tick :1029-1030 锁内。读写同基（对齐钟）自洽。
- `lastSyncStart`：:356 maybeSyncClock 锁内写入；成功分支 :393 锁内回读。写读同持 s.mu。
- `syncing`：:355 锁内置位 / :369（goroutine 完成回调）/:404（无客户端复位路径）锁内复位——复位路径全持锁，TestClockSyncNoClientResetsSyncing 固化。
- `lastProbe`：:679 Start 主循环锁内回零 / :964 ProbeNow 锁内 / :1089 probe 失败锁内 / :1114 probe 成功锁内。四写点全部持 s.mu。
- `state.EmptyProbeRuns`：:1156/:1158 probe 锁内入账/归零。
- `warnedNoTargets`（:1371-1372）——唯一无锁写点。宿主唯一性射证：submitAll 仅由 tick（:1035）单 goroutine 调用（Start :666-684 主循环 select），全仓库 grep 无第二处读/写；TestSubmitAllWarnsOnceOnNoTargets 固化语义。实际无并发竞争。延续观察。

**`*Locked` 写函数族 13 个 + 外部写函数首行取锁双向射证**

- `*Locked` 函数全量 grep 命中 13 个：nowAlignedLocked(:273，纯读)/openTimeForLocked(:427，读)/enrichTargetPubMetaLocked(:539，读)/rebuildCoursesForAccountLocked(:570，写)/rebuildCoursesLocked(:647，写)/tokenValidForLocked(:739，读)/windowClosedLocked(:918，读)/isRateLimitedLocked(:1691，读含过期删 key)/markRateLimitedLocked(:1710，写)/markFullLocked(:1760，写)/releaseFullIfFreedLocked(:1781，写)/statusIndexLocked(:1835，读)/setStateLocked(:1846，写)。写类 5 个全部只在持 s.mu 上下文被调用（SetTargetsForAccount/RestoreTargets 锁内 enrich；probe 锁内 windowClosedLocked/isRateLimitedLocked/markFullLocked/setStateLocked；MarkDone/RemoveDone 锁内 markFull 族；TryAcquireSubmit 锁内 inflight 位）。
- 外部写函数首行取锁：SetTargetsForAccount(:455)/PurgeAccount(:496)/RestoreTargets(:526)/RestoreDone(:608)/RestoreRefused(:627)/TryAcquireSubmit(:1897)/MarkTokenValid(:1312-1315 reloginMu→s.mu 双锁，与 maybeRelogin 锁序一致)/MarkDone(:1923)/RemoveDone(:1990)/RemoveFull(:2039)。MarkTokenValid 双锁序与 maybeRelogin/TokenValidFor 对齐。
- 双向射证闭环：无锁外调用 *Locked 写函数路径；无外部写函数漏首行取锁。

### 2. OBSERVE-117-01 知识位 —— ✅ 在位（第三十一轮，见上 maybeRelogin 双侧明细）

### 3. B110-01 审计链第三十八轮 —— ✅ 零漂移

- **手动 6 失败位 AppendLog 逐一在位**：:362（报名 token 失效）、:374（报名 read 类）、:381（报名其余业务失败）、:443（退选 token 失效）、:452（退选 read 类）、:459（退选其余业务失败）。每处 `if err != nil { log.Printf }` 不吞错。成功审计行：手动报名/退选成功经 MarkDone(:1976)/RemoveDone(:2029) 内部 AppendLog；登录 :237 / 管理员登录 :136 / 登出 :616 / 设目标 :558 / 删账号 :1055 / 配置更新 :864 全在位；scheduler 自动链全分支（:1504/:1532/:1558/:1614/:1662/:1770）AppendLog 落库失败 log.Printf 零吞错。
- **零吞错穷举**：产品代码 `_ =`/`_, _ =` 命中仅 handler.go :387 `_ = d.Sched.MarkDone(...)` 与 :467 `_ = d.Sched.RemoveDone(...)`——核证两函数恒返回 nil（内部落库失败已 log，账号已删静默返回 nil），忽略返回值无害。其余 `_ =` 全在测试/无害路径（scheduler.go :307 prewarm goroutine、:1075 probe goroutine、client.go :107/:124 io.Copy drain、accounts trust manager gate 等）。scheduler.go 的 `_, _ = s.ProbeForAccount(acct)`（:1075）为 probe goroutine 单飞语义，返回值不求（探测结果由帧写回），合规。
- **网络层 token 脱敏延续**：zhidao/client.go :450 doRequest 连接失败统一 `sanitizeError`（:581-593，剥 *url.Error 完整 URL、Unwrap 保留判型链）；scheduler.go :1283 重登成功日志 `maskedToken(newTok)`（:1334-1339，长度 ≤8 输出 `***`）。sanitize_test.go 判型穿透族（dial/write/read-rst/fin/shortread/timeout/business/nil 八形态）定向 race 实测全 PASS（1.417s）；TestDoRequestSanitizesDialError 端到端——真实连接层失败剥 URL 不含 token/idToken= 段且保留 dial/connectex 描述与判型。

### 4. O105-01 抖动基线 —— ✅ 在位且实测绿

- socketPreheat 夹具：zhidao/client_test.go :25-30（预创建-关闭 127.0.0.1 套接字）+ :40 包级调用 + captcha_test/sanitize_test 逐调用；api handler_test.go :67-71 套接字预创建。
- readyProbe 夹具：zhidao/client_test.go :88-113（200ms×10 + 2s 显式超时宽栅栏）+ :78/:164 逐测试调用；api/handler_test.go :179-203 同款 + :139 就绪探测；accounts/manager_test.go :26-40 readyProbe（无 socketPreheat 双保险，注释明确"8 轮全绿实证无残余，若未来再出冷启动 flake 第一候选即补"——延续观察）。
- 定向 `-race` 四包实测全绿：zhidao 2.330s / accounts 1.724s / scheduler 15.460s / api 16.641s。双回归锚实测 PASS。
- 身份防线族十测（-race -v）全 PASS，含同名校验族六测试（成功/失效/风控/窗口关闭/确证满员/实时复核失效）+ 决策侧四 map + 回归锚 + interval clamp。

### 5. LOW-132/133 回首核 —— ✅ 通过

`time.Since`/`time.Now()` 全量扫描（产品代码）分类裁决：
- scheduler.go :269/:274 = `nowAligned`/`nowAlignedLocked` 内部实现（对齐钟基准本身），合规。
- scheduler.go reloginAt 族：:1220/:1227 读 `time.Since(t)`（reloginAt 存值），:1231/:1261 写 `time.Now()`——同一本地基写读自洽。
- accounts/manager.go gateWindow 族：:53/:74/:227 写 `time.Now()` + :71/:226 读 `time.Since(gateWindow)`——同一本地基写读自洽（B42-01 闸门族内部）。
- handler.go :1176 loginLimiter tokenBucket 纯本地自洽。
- session/store.go :74/:106/:156/:168/:183/:198 会话/票据 TTL 纯本地自洽（与调度器无交互）。
- zhidao/client.go :113-137 SyncServerTime RTT 计时/clockOffset 计算：`time.Since(start)` 差值与 `time.Now()` 绝对值的时钟对齐算法自身基准，写完即被调度器 `nowAligned` 同基消费，合规。
- :328/:355 captcha v 时间戳 / uniqueId 设备指纹 = 平台契约要求的"当前毫秒/36 进制时间戳"，非节流判读，合规。
- 时钟族写入侧（lastSyncFailAt/syncFailedWindow）已收敛到对齐钟（:372/:377 nowAlignedLocked，读侧 :342 传对齐 now）。

**裁决：残余时间基（reloginAt/gateWindow/limiter/session TTL/SyncServerTime）均为写读同基自洽或平台契约需求，合规。**

### 6. 新契约角度纵深（自选 ×2，选因说明）

**（a）探测定时族三件套（lastProbe 节流闸门 / probing 单飞 / probeSem 信号量）**

为什么选：`probe()` 是调度器唯一面向全校的探测容器，60+ 轮被反复加固，但"三件套各自管什么、谁写谁读、什么条件放弃"的边界最容易被后续改动混淆——每轮都声称"在位"，本轮从三者互不越界的角度实测核证。

实测核证（scheduler.go）：
- **lastProbe = 全校节流闸门**：只有 probe()（:1114 成功 / :1089 失败计节流）与 ProbeNow（:964）写入；tick 读 `now.Sub(last) >= probeIntervalForOpen(now, open)`（:987）判定是否需探测。ProbeForAccount（账号级，:873 注释）与 Start 主循环（:679 重登成功回零触发补探测）各司其职——账号级探测绝不影响全校节流，避免被管理员手动点课旁路吞掉临门 2s 盯守。
- **probing = probe() 主体单飞守卫**：:1050-1054 置位（持 s.mu，"置位-检查"原子），成功 :1113 / 失败 :1088 / 无账号 :1081 三路复位全持锁。命中单飞直接放弃本次探测——最长推迟一个 tick（300ms），临门/黄金期无实质损失。但**它只保护 probe() 主体，不保护 per-account goroutine**（注释 :1061-1071 明示）。
- **probeSem = per-account 并发上限**：cap 4 结构化信号量（:1073-1074 `s.probeSem <- struct{}{}` / defer 释放），把"峰值从 N 降到 4"（:1066 注释），跨批（2s 周期短于一批耗时）由同一信号量约束绝不叠加；`ponytail: cap=4 常驻，若平台放宽熔断或账号数 >50 再调`（:1068）——刻意简化标注了上限与升级路径。
- 三件套边界：lastProbe 管"多久探测一次"（节流维度）、probing 管"同一瞬间几个 probe 主体"（单飞维度）、probeSem 管"一个 probe 批次里几个账号并发"（并发维度）——三者互不越界，覆盖了探测的全部共振面。N 账号部署下峰值并发 = 1（probe 主体） + 4（probeSem 封顶） = 5，与"访问过于频繁 1 分钟熔断"实证契约对齐。

**（b）登录闸门族 B42-01 双侧收口 + 凭据 AES-256-GCM 加密链**

为什么选：登录是出口 IP 唯一触达平台的路径，闸门双侧（自动重登 gateWait 阻塞 / 手动登录 gateTryAcquire 非阻塞）共享计数的正确性决定"锁号防线"是否被旁路；凭据加密链决定密文落库是否真无明文残留。两条都是安全结论不可走读的路径。

实测核证：
- **双侧收口**：`gateWait`（manager.go :49-64，阻塞排队，被调度 Relogin 用）与 `gateTryAcquire`（:223-235，非阻塞准入，LoginByPassword :244 用）共享同一 `gateMu` 与 `gateUsed` 计数（:40-43），全账号每分钟 doLogin 预算严格 ≤ gateLoginPerMin=2（:161）。doLogin 全入口穷举：产品代码 `c.Login(` 仅 manager.go :254（LoginByPassword 内部）一处直接路径，Relogin（:166-175）经 gateWait，LoginByPassword 经 gateTryAcquire——无第三入口旁路。`GatePump`（:68-77）由 main.go :153-159 后台 goroutine 每 30s 推进窗口翻页，唤醒排队重登。测试夹具批量注册撞闸门用 `ResetGateForTest`（:82-87，测试专用，正式代码不调用——grep 确认仅 test 引用）。
- **AES-256-GCM 加密链**：secure/crypto.go `LoadOrCreateKey`（:15-45：XUANKE_MASTER_KEY 64 位 hex 或 DB 旁 .master_key，两者都校验 32 字节，损坏/截断显式拒绝启动）、`Encrypt`（:48-63：随机 nonce 前置，GCM Seal，hex 输出）、`Decrypt`（:66-87：hex 解码 + nonce 前缀 + GCM Open，密文长度非法显式报错）。凭据落库：LoginByPassword 成功分支 `SaveCredential(acct, enc, token)`（manager.go :277-281，`enc` = encrypt(password) 后密文）；vision_key 同样 `d.secureEncrypt` 加 `enc:` 前缀（handler.go :60-71，未注入 Encrypt 时拒绝——严禁明文入库），settings 表永不出现明文（:832 注释）。main.go 恢复侧：vision_key 无 `enc:` 前缀一律拒绝加载（:88-97，彻底拒绝旧版未加密明文）。配 `maskKey` 回显脱敏（只显 ****+后 4 位）与 token 落库前 8 位脱敏日志双层防护。
- handler_test.go :167-173 注入真实 AES-256-GCM 加密（rand 32 字节主密钥），TestVisionKeyEncryptedAtRest（:752-790）固化"settings 表只存 enc: 前缀密文 + 恢复规则复刻 main"契约——加密链与恢复链 TDD 双闭环。

## 维持观察项（LOW）

1. **scheduler.go:1371-1372 `warnedNoTargets` 无锁写**——宿主唯一性已证（submitAll 仅由 tick 主循环单 goroutine 调用），全仓库 grep 无第二处读/写，TestSubmitAllWarnsOnceOnNoTargets 固化语义；延续观察，未来若引入并发调用 submitAll 的路径须先收口。
2. **handler.go :387/:467 两处 `_ =`** 忽略 MarkDone/RemoveDone 返回 error——实测两函数恒返回 nil（内部落库失败已 log，账号已删静默返回 nil），语义无害；延续观察。
3. **accounts/manager_test.go 无 socketPreheat 双保险**（只有 readyProbe）——注释明确实证无残余；延续观察，若未来该包再出冷启动 flake 第一候选即补 socketPreheat。

## 结尾建议

**APPROVE**。身份防线矩阵第六十三轮闭合：sameClientFor 定义 + 7 调用点零漂移且每条追到终局写点（状态/map/库行）；maybeRelogin 双侧 + 二次 ClientFor 重取当前 token 落库（OBSERVE-117-01 知识位第三十一轮）在位；手动五路 accountExists（含目标写内联判据）同源；写点换类 5 类 + warnedNoTargets 唯一无锁点全收口；`*Locked` 写函数族 13 个 + 外部写函数首行取锁双向射证闭环。B110-01 审计链第三十八轮零吞错穷举零命中 + 脱敏延续零漂移。O105-01 四包定向 -race 全绿 + 身份防线族十测全 PASS + 双回归锚实测通过。LOW-132/133 回首核通过（时间基残余全为写读同基自洽或平台契约需求，时钟族写入侧已全收敛对齐钟）。新契约纵深（探测定时族三件套互不越界、登录闸门族双侧共享计数 + 凭据 AES-256-GCM 加密链/恢复链双闭环）均实证在位。工作树除本报告与前端并发产出外零改动。