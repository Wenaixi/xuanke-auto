# round141 后端审查发现（身份防线矩阵第五十六轮）

> 审查模式：绝对只读。工作树基线 `60ea9d6`（R140 归档，身份防线矩阵第五十五轮闭合）。产物仅 `archive/review-rounds/` 下两份 findings（本文件 + r141-frontend 由并行前端代理产出），其余零修改。
> 实测环境：Windows 11 / go1.26.1 / mingw64 gcc + WinLibs gcc 双编译器（race 需 cgo，双路径均验证）。

## 结论前置

| 级别 | 数量 | 说明 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| LOW | 1 | 观察项延续：`handleAdminDeleteAccount` 中 `PurgeAccount` 失败无状态回滚（半删态自愈语义已文档化，非缺陷，见「维持观察项」） |

**建议：APPROVE（通过）。** 身份防线矩阵第五十六轮零漂移，OBSERVE-117-01 知识位第二十四轮在位，B110-01 审计链第三十一轮零吞错，O105-01 抖动基线证据充足，LOW-132/133 时间基残扫零混用。无任何阻塞项。

## 验证表

| 验证项 | 实测时间 | 结果 | 数据 |
|--------|----------|------|------|
| 基线确认 | 2026-09-24 | ✅ | `git rev-parse HEAD` = `60ea9d65db903bc0158cb3d7f4c36fb61203b898`，工作区仅 `?? ../archive/review-rounds/round141-frontend-findings.md` 一个未跟踪文件（并行前端代理产物），`git status --short --branch` 无其他漂移 |
| `go build ./...` | 2026-09-24 | ✅ | 全包通过（BUILD_EXIT=0） |
| `go vet ./...` | 2026-09-24 | ✅ | 全包通过（VET_EXIT=0） |
| 定向 `-race` 四包 | 2026-09-24 | ✅ | `./internal/zhidao ok`（cached 后 fresh）· `./internal/accounts ok 1.301s` · `./internal/scheduler ok 15.006s` · `./internal/api ok 13.404s`，均为 `-count=1` 无缓存实跑 |
| `-race` 扩展六包 | 2026-09-24 | ✅ | `db 2.410s / store 14.573s / session 1.816s / runtime 1.645s / secure 1.679s / config 1.723s` 全部绿 |
| 身份防线族十测精确复跑 | 2026-09-24 | ✅ | `-run "TestDeletedAccountRebuiltSameNameChain"` 七测全 PASS + `TestMaybeReloginDeletedAccountSkipsMaps` 等六测全 PASS，逐条 RUN/PASS 实证（见下节详细清单） |
| 回归锚 | 2026-09-24 | ✅ | `TestWindowOpenSubmitsWithoutProbeReset`（scheduler_test.go:1277）PASS 1.06s + `TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go:1011）PASS 0.42s，均在 race 下逐条实测 |
| 轮次标签扫描 | 2026-09-24 | ✅ | 产品代码（internal/ 与 cmd/ 全仓）`第 N 轮/R..轮/round N` 零命中；唯一命中是测试文件中的"第 3 轮"连续探测轮数语义叙述（合规） |
| 时间基残扫 | 2026-09-24 | ✅ | 见 LOW-132/133 回首核 |
| TDD 双测试在位 | 2026-09-24 | ✅ | `TestSnapshotTTLUsesAlignedClock`（scheduler_test.go:1516）+ `TestClockSyncFailureBackoffUsesAlignedClock`（:1543）均在位，race 精确跑绿 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第五十六轮闭合 — ✅ 在位

**sameClientFor 定义调度器内部零漂移。** `scheduler.go:204-210`，内部 `ClientFor` 取当前注册表客户端，`clientIdentity`（scheduler.go:215-224）用 `reflect.ValueOf(c).Pointer()` 做指针身份比对，nil / 非指针 / 账号不存在均返回 false。与 R135-R140 归档描述逐字一致。

**7 个调用点逐一追到"写什么状态/落什么库行"的终局**（每个调用点都在网络往返回锁后、写内存态与落库之前）：

| 行号 | 位置 | 非同一身份时放弃的终局写入 |
|------|------|------------------------------|
| `:850` | `ProbeForAccount` 写回侧 | `openTimeDetected[acct]` 识别槽 + `acctData[acct]`/`acctDataAt[acct]` 专属快照（防过期快照/错年级帧写入重建账号） |
| `:1489` | spawnChain 失效分支 | `delete(inflight)` + `maybeRelogin` 触发 + `setStateLocked(failed)` + `AppendLog`（防 Erroneous 旧链把失效态与重登触发写进新身份） |
| `:1521` | spawnChain 成功分支 | `done[acct][classID]=true` + `setStateLocked(success)` + `AppendLog(success)` + `SaveSuccess` 库行（防"重启假成功"） |
| `:1551` | spawnChain 风控退避分支 | `markRateLimitedLocked`（30s 退避）+ `setStateLocked(failed)` + `AppendLog`（防假"退避中"吞黄金期） |
| `:1571` | spawnChain 窗口关闭分支 | `markFullLocked` 永久满员退避 + `AppendLog` |
| `:1600` | spawnChain 实时复核 Unauthorized 分支 | `maybeRelogin` + `setStateLocked(failed)` + `AppendLog`（第六分支，B43-02 闭合） |
| `:1635` | spawnChain 实时复核确证满员分支 | `markFullLocked`（前置 `doneHas` 让位防覆盖手动胜利状态不变） |

所有分支共享同一失败语义：静默放弃整链/回写，不写任何内存态与库行，且失败发生在 `inflight` 已统一清位之后、无残留占位。

**maybeRelogin 双侧复核**：决策侧 `:1208` 锁内 `ClientFor` 存在性复核（B43-01，入口写 map 前置）；写回侧 `:1254` 重登完成时二次 `ClientFor` 存在性复核（已删账号静默放弃整个成功分支）；`:1265-1273` 锁内再取当前注册表 `client.Token()` 重取新 token 落库 `UpdateIDToken`——同名重建场景旧 goroutine 在此复核处即被丢弃，绝不把旧身份 token 写进新身份凭据行。

**手动五路 accountExists**：`handler.go:255`（handleElectives 课程读）、`:305`（handleElectiveSelect 手动报名）、`:397`（handleElectiveExit 手动退选）、`:497-512`（handleSetTargets 目标写，凭据表内联遍历）、`:573`（handleState 状态读）。判据同源 `accountExists`（:1106-1120，LoadCredentials 逐账号比对，"确实登录过"的更强真理源），五路全覆盖，查无此账号整体拒绝、绝不用全局帧/空状态假装成功。

**写点换类全持锁/唯一性射证**：

| 写点类 | 写入位置 | 锁/宿主 |
|--------|----------|---------|
| `lastSubmit` | `:1346` `submitAll` 内 | 持 `s.mu`，写入基 `nowAlignedLocked()`；宿主唯一性：`submitAll()` 唯一生产调用点 `tick:1035` |
| `lastSyncStart` | `:356`（发起路径）、`:405`（无客户端兜底复位） | 均持 `s.mu` |
| `syncing` | `:355` true / `:369` goroutine 回调 false / `:404` 无客户端 false | 三处均持 `s.mu`（`:369` goroutine 内 `s.mu.Lock(); defer Unlock`） |
| `lastProbe` | `:679`（Start 重登回传主循环补探）、`:964`（ProbeNow）、`:1089`（probe 失败落地）、`:1114`（probe 成功落地） | 四处均持 `s.mu`；ProbeForAccount 刻意不写（:868-872 注释：只归 probe()/ProbeNow，防管理员穿透探测旁路全校节流） |
| `state.EmptyProbeRuns` | `:1156` ++ / `:1158` = 0（probe 入账） | 持 `s.mu` |
| `warnedNoTargets`（无锁写点） | `:1371-1372`，且仅在 `if !s.warnedNoTargets` 内自写 | **宿主唯一性成立**：唯一调用宿主 `submitAll()`，`submitAll()` 唯一生产调用点 `tick:1035`（tick 主循环单 goroutine 串行），写入紧随 `len(chains)==0` 判断、单写单读，不并发，无锁安全 |

***Locked 写函数族双向射证**：13 个 `*Locked` 定义（nowAlignedLocked/openTimeForLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/rebuildCoursesLocked/tokenValidForLocked/windowClosedLocked/isRateLimitedLocked/markRateLimitedLocked/markFullLocked/releaseFullIfFreedLocked/statusIndexLocked/setStateLocked），全部要求持锁调用；外部写函数（SetTargetsForAccount/RestoreTargets/PurgeAccount/MarkDone/RemoveDone/RemoveFull/TryAcquireSubmit/StateForAccount）首行取锁，与 Locked 内部"需持 s.mu"契约双向吻合，无"外部函数直接写 state/map 不带锁"的裸露路径。

### 2. OBSERVE-117-01 知识位第二十四轮 — ✅ 在位

写回侧顺序实测：`s.clients.ClientFor(acct)` 复核（:1254）→ `err==nil && relogged` 成功分支 → `delete(reloginFail)`/`reloginAt=Now`/`tokenValid=false`（:1260-1262）→ 锁内二次 `ClientFor(acct)` 重取 `client.Token()`（:1265-1267）→ `UpdateIDToken` 落库（:1269）。同名重建场景下旧 goroutine 在一次复核即被丢弃，二次重取拿到的是"复核通过时"的注册表客户端，Token 为当前身份新 token，绝不串旧身份。写回侧防线上还保留 `tokenValid` 关键语义：`tokenValid[acct]=true` 只在发起决策段置位（:1241），成功分支才清（:1262），与 `MarkTokenValid` 锁序（reloginMu→s.mu，:1307-1319）串行化，杜绝"手动登录清标记与在途重登置位"的并发半态。

### 3. B110-01 审计链第三十一轮 — ✅ 在位

**手动 6 失败位**（handler.go）：`:362` 报名 token 失效 / `:374` 报名 read 类（请求已发出结果未知，最需留痕）/ `:381` 报名其余业务失败 / `:443` 退选 token 失效 / `:452` 退选 read 类 / `:459` 退选其余业务失败——六位全部 `AppendLog` 且 `if err != nil { log.Printf }` 零吞错。**成功审计行**：手动报名成功走 `MarkDone` 内部 `AppendLog(select, msg, true)`（:1976）+ `SaveSuccess`(:1973)，手动退选成功走 `RemoveDone` 内部 `DeleteSuccess`/`SaveRefused`/`AppendLog(exit, true)`（:2020/2026/2029）；`_ = d.Sched.MarkDone(...)`（:387）与 `_ = d.Sched.RemoveDone(...)`（:467）的返回值恒 nil（内部已处理全部落库错误），不吞任何真实错误。**自动链失败族**：spawnChain 失效/风控/窗口关闭/实时复核失效/确证满员/普通失败六分支全部 `AppendLog`（scheduler.go:1504/1558/1614/1662/1770/1532/1535），全部 `if err != nil` 记账。

**零吞错穷举**：`grep "_ = \|_, _ ="` 产品代码（排除测试）命中仅 9 处——`handler.go:387/467`（MarkDone/RemoveDone 恒 nil 返回值）、`config.go:96/141/160`（os.Setenv/os.MkdirAll/os.WriteFile 配置落盘最佳努力）、`scheduler.go:307`（pw.Prewarm 异步预热）、`:1075`（probe 账号子 goroutine 的 ProbeForAccount 信号量内并行）、`client.go:107/124`（io.Copy(io.Discard) 刻意丢弃响应体）。**全部落库调用点（AppendLog/SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefused/UpdateIDToken/SetTargetsForAccount/SaveCredential/SaveAccountName）逐一 grep 复核：零 `_ =` 形态，全部 `if err != nil { log.Printf }`**。零吞错穷举通过。

**token 脱敏延续**：`client.go:581 sanitizeError`（剥 url.Error 完整 URL 的 idToken 段，`sanitizerErr` Unwrap 下沉保判型）→ `isConnErrRetryable`/`IsReadErr` errors.As/Is 穿透不变；`scheduler.go:1283` `maskedToken(newTok)` 只显前 8 位；`manager.go:319 tokenShort` 同样只显前 8 位。日志全扫描确认无任何完整 token 形态泄漏。

### 4. O105-01 抖动基线 — ✅ 证据充分

`socketPreheat` 夹具在位：`zhidao/client_test.go:25`（预创建-关闭 127.0.0.1 回环套接字排空冷启动窗口）；`readyProbe` 夹具在位：`accounts/manager_test.go:26`（含"accounts 未配 socketPreheat 双保险、8 轮全绿实证、残余则第一候选即补"的注释）、`api/handler_test.go:179`、`captcha_test.go`（"socketPreheat 只预占单端口，并发首请求形态需 readyProbe 把 accept 就绪前首请求吃掉"与 api/zhidao 同款）。四个关键包 `-race -count=1` 无缓存实跑全部绿（见验证表），身份防线族十测归入定向复跑。回归锚两枚常年内置在每轮 race 中，均绿。额外使用 WinLibs gcc 复跑一处 race（`TestMaybeReloginDeletedAccountSkipsMaps`）同样绿，双编译器路径均验证通过。

### 5. LOW-132/133 回首核 — ✅ 通过

`git show` 白线：`975dc2b`（LOW-132-01）把 `lastSyncFailAt`/`syncFailedWindow` 写入改 `nowAlignedLocked`，判读侧 `:342` 同基准；`d75f38c`（LOW-133-01）把快照 TTL 判读侧四处 `time.Since` 改 `nowAlignedLocked().Sub`，写入侧 `:835/:864/:1116/:966` 已对齐钟。两次提交均带 TDD 测试（`TestClockSyncFailureBackoffUsesAlignedClock` / `TestSnapshotTTLUsesAlignedClock` 先红后绿），本轮已精确 race 复跑全绿。

全量时间基残扫（`time.Since|time.Now()`，排除测试与 `time.Time{}` 字面量）现残余共三族，**写读同基自洽为合规**：
- **scheduler relogin 族**：`time.Since` 仅 `:1220/:1227`（reloginAt 读）与 `time.Now` `:1231/:1261`（reloginAt 写）——写读同基自洽，属 LOW-134 留下的"残余仅 reloginAt 一处"合规态。
- **accounts gateWindow 族**：`manager.go:53/71/74/226/227`——gateWait/gateTryAcquire 的 gateWindow 写（time.Now）与读（time.Since）同基自洽；登录闸门语义本就基于本地钟（全账号 doLogin 预算，与平台时钟无关），不参与开窗点判定。
- **独立自洽孤岛**（不涉开窗点/快照 TTL 判定，各为独立闭环）：api `loginLimiter` tokenBucket（handler.go:1176）；session 会话/票据 TTL 过期（store.go:74/106/128/156/168/183/198）；zhidao 非登录动态参数生成（captcha `v=`、`uniqueDeviceID` 均为单次生成值）。

残留零混用，LOW-132/133 回首核通过。

## 新契约角度纵深（自选 ×2）

### 角度 A：登录闸门族 B42-01 双侧收口全链路

选取理由：B42-01（round42 沉淀）把全局 doLogin 频率闸门从"只收口自动重登"扩展到"学生手动登录/管理员换绑全入口收口"，本轮从「门控结构 → 双侧触发点枚举 → 测试夹具」全链路实测，验证出口 IP 锁号防线在每一侧都真正生效。

**门控结构**：`gateMu sync.Mutex` 与 `gateWindow/gateUsed` 共享计数 + `gateCond *sync.Cond`。`gateWait()`（manager.go:49-64）阻塞语义：预算满则 `gateCond.Wait()` 挂起等待下个窗口广播；`gateTryAcquire()`（:223-235）非阻塞语义：`time.Since(m.gateWindow) >= time.Minute` 翻页重置，quota 充足消耗并返回 true、已满立即返回 false，与 gateWait 共享同一 `gateMu`/`gateUsed`，排队重登与手动登录严格共享全账号每分钟 doLogin 预算（= 2 次，gateLoginPerMin 常量，:161）。

**双侧收口验证**：
1. 自动重登侧：`Relogin(acct)`（:166-175）先取客户端再 `gateWait()` 阻塞排队——多账号并发失效重登时实际触达平台 doLogin 被收敛到 2 次/分钟，绝不刷爆出口 IP。
2. 手动登录侧：`LoginByPassword`（:243-246）入口 `gateTryAcquire()` 非阻塞准入——quota 满立即返回"登录尝试过于频繁，请稍后再试"，**绝不用阻塞 gateWait**（排队会挂起用户登录响应数分钟）；管理员换绑同走此收口（注释 :241-242 明确"统一收敛更安全"）。
3. `GatePump`（:68-77）main 后台 goroutine 每 30s 调用翻页广播，唤醒排队重登重新竞争预算；`ResetGateForTest`（:82-87）测试专用清空窗口，api 夹具批量注册借它防误拦（handler_test.go:364-368）。

**测试证据**：manager_test.go 有专门的闸门预算消耗测试（gateTryAcquire 与 gateWait 共享计数、旧窗口重置、预算耗尽即拒），本轮已随 accounts 包 `-race` 全量实跑覆盖。

结论：B42-01 双侧收口全链路（结构 → 触发 → 测试）闭环，自动重登阻塞排队 + 手动登录非阻塞拒绝的双语义边界清晰，无任何入口绕过闸门直发 doLogin。

### 角度 B：多账号并发调度一致性（acctData 独立帧 + 写锁纪律）

选取理由：多账号年级隔离是本系统最高价值的正确性资产（年级串线会让高二学生看到高三课程），本轮从「acctData 写入点清点 → 各读取方快照隔离 → 全局帧回退链」三态逐环实测，验证"账号 A 绝不携带 B 的会话"契约完整。

**写入点全清点**（scheduler.go）：
- `:863-864` `ProbeForAccount` 内 `acctData[acct]=data` + `acctDataAt[acct]=now`——**唯一的专属帧写入点**，且已持 `s.mu`、前置 `sameClientFor` 身份复核（B41-01 族），网络往返后回锁复核才落写。刻意不写全局 `lastProbe/lastData`（:866-872 注释）。
- `:1115-1116` `probe()` 内 `lastData= data` + `lastDataAt= now`——**唯一的全局帧写入点**，同样持 `s.mu`。
- `:505` PurgeAccount `delete(s.acctData, acct)` 等全量清理，删号后绝不留专属帧残留。

**读取方快照隔离验证**：
- `ElectivesSnapshotFor`（:761-805）三态：目标账号（`len(acctTargets)>0`）专属帧新鲜即返回、过期返回 (nil,false) 触发真刷新、**绝不回退全局帧**（:784 显式 return nil,false）；非目标账号有专属帧新鲜返回、过期同样 (nil,false)；从未有过专属帧的纯浏览才回退全局帧（:801-804）。全局帧恰好被 order[0] 账号刷新的场景不会让浏览者读到错年级数据（:790-794 注释实证）。
- `CheckClassSelectable`（:1863-1892）手动报名复核**只用该账号专属帧**，过期即放行交给平台，不用全局 lastData 兜底（跨年级帧无参考价值）。
- `classFullInSnapshot`（:1721-1744）/`releaseFullIfFreedLocked`（:1781-1811）满员判定**专属帧优先、全局帧兜底**——满员是"该门课在全校唯一"的事实，全局帧兜底安全（课程 id 全局唯一，跨年级无错帧风险）。

**结论**：acctData 唯一写入点持锁 + 身份复核前置，读取方专属帧优先、过期触发刷新绝不串年级，多账号并发调度一致性三态闭环。grade 隔离契约在 scheduler 层无裸露写点。

## 维持观察项

1. **LOW 观察（延续）**：`handleAdminDeleteAccount` 中 `PurgeAccount` 无错误返回、若 `Store.DeleteAccount` 失败则进入"注册表已摘 / 库行未清"的半删态——已文档化为"重启 Restore 重建客户端自愈"的刻意决策，且远优于"假删除成功"（客户端在飞写回）。维持观察，不列为缺陷。
2. **obs 观察（延续）**：`classFullRealtime` 底层 `IsClassFull` 因平台未下发 maxCount 恒判不满，实时复核实为防御性路径——真满员主路径为快照 `max_count`（实证字段）。契约已在 CLAUDE.md 落盘，审核轮次持续确认不在本轮引入新的误判形态。
3. **`warnedNoTargets` 无锁写点的宿主唯一性**依赖 `submitAll()` 的生产唯一调用点 `tick:1035`——该依赖被 `scheduler_test.go` 的 `s.submitAll()` 直接调用（测试态多 goroutine 不触发，仅串行无锁场景下唯一性成立）。若未来新增 `submitAll` 并发调用宿主，需先补锁。维持观察。
4. **观察**：api 登录/激活限流桶 `loginLimiter` 使用本地钟自洽闭环（写读同基），但 token 桶容量/速率常量（`loginBurst`/`loginRate`/`bucketTTL`）为编译期常量，无运维调参通路——如果学校 NAT 下误杀需要运行时放宽，属未来可选增强，非本轮缺陷。

## 结尾建议

**APPROVE（通过）。**

身份防线矩阵第五十六轮：sameClientFor 定义与 7 调用点零漂移、每条追到写状态/落库终局，maybeRelogin 决策侧/写回侧双侧复核 + 二次重取 Token 落库在位，手动五路 accountExists 判据同源，五大写点类全持锁、无锁写点 warnedNoTargets 宿主唯一性射证成立，*Locked 族双向射证闭环。OBSERVE-117-01 知识位第二十四轮在位。B110-01 审计链第三十一轮手动 6 失败位 + 成功行 + 自动链失败族全部落地，零吞错穷举通过。O105-01 抖动基线 socketPreheat/readyProbe 夹具在位，定向 race 四包 + 扩展六包实证全绿。LOW-132/133 回首核白线核对 + 时间基残扫零混用。新契约纵深（登录闸门族 B42-01 双侧收口、多账号并发调度一致性）两向均无断点。

无 CRITICAL/HIGH/MEDIUM，1 条 LOW 观察项为延续性观察，无阻塞——轮次归档可闭合。
