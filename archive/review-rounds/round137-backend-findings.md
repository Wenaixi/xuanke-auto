# R137 后端审查报告（绝对只读，身份防线矩阵第五十二轮）

- 日期：2026-09-24
- 基线：R136 归档（0cf543a）；工作树 `git status --short --branch` 显示 `## master` + 唯一未跟踪文件 `?? archive/review-rounds/round137-frontend-findings.md`（前端并行代理产物）。backend/ 产品代码自 R136 后零改动（`git log -- backend/internal/scheduler/scheduler.go backend/internal/api/handler.go` 最近提交 = d75f38c/975dc2b，均属时钟时间基域，身份防线零改动）。
- 模式：绝对只读（唯一允许写文件为本报告；全仓库其余零修改）
- 聚焦清单六项全实测 + 2 个自选新纵深（时间盒 35 分钟内完成，取手动 4 方法协同族 + 激活票据/删号四序）
- 结论前置：**身份防线矩阵第五十二轮闭合，全部验证项实测绿，新增发现 0，无 CRITICAL/HIGH/MEDIUM/LOW，裁定 APPROVE**

## 验证表（全部实测）

| 项目 | 结果 | 证据（实测时间/数据） |
|------|------|------|
| go build ./... | 绿（exit 0） | 全包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race 四包 | 全绿 | 借 `/d/mingw64/bin/gcc.exe`（实测存在）注入 PATH：zhidao 2.405s / accounts 1.680s / scheduler 15.202s / api 21.351s 全 ok 零竞态 |
| 身份防线族 race | 全绿 | `-race -run 'TestDeletedAccountRebuiltSameNameChainDrops(Success\|Relogin\|RateLimitBackoff\|WindowClosedFull\|RealtimeRecheckFull)\|TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin\|TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight\|TestRealtimeFullRecheckKeepsManualSuccess\|TestRealtimeFullRecheckWithNoManualDoneMarksFull\|TestSubmitSuspendedWhenOpenTimeCleared\|TestRealtimeRecheckUnauthorizedTriggersRelogin\|TestMaybeReloginDeletedAccountSkipsMaps'` scheduler 包 3.542s 全 PASS（waitChainExit 等待契约在位，scheduler_test.go:3185，注释明言绝不用 inflight 等待防假绿） |
| 回归锚 | 全绿 | TestWindowOpenSubmitsWithoutProbeReset + TestAdminStatsWindowOpenedUsesScheduler（scheduler_test.go:1277 / handler_test.go:1011）定向 race 2.417s 全 PASS |
| 时钟时间基族 race | 全绿 | TestSnapshotTTLUsesAlignedClock（:1516）+ TestClockSyncFailureBackoffUsesAlignedClock（:1543）+ TestClockSyncNoRetryWithinBackoff + TestObservesClockBackoff + TestReloginLogs + TestReloginFailureLogs 2.140s 全 PASS |
| 登录闸门族 race | 全绿 | TestLoginByPasswordRejectsWhenGateBudgetExhausted + TestLoginByPasswordAllowedWhenGateBudgetAvailable（accounts 包）1.337s 全 PASS |
| 手动审计族 race | 全绿 | TestManualElectiveFailureAppendsLog + TestManualElectiveReadErrAppendsLog（handler_test.go:1800/:1866，就地覆盖 select+exit 双路）+ TestHandleElectivesSelectUnauthorizedRelogin（:1965）api 包 2.800s 全 PASS |
| 契约轮次标签扫描 | 产品代码零命中 | 全仓扫 `第N轮/R..轮/round N/Round` 产品代码零命中；唯一命中为 session/store.go:117 注释引用历史文档名 `docs/review-round13.md`（历史引用，合规） |
| 零吞错穷举 | 零库写忽略 | `_ =` 仅 2 处防御性忽略（handler.go:387 MarkDone / :467 RemoveDone，非库写）；`_, _ =` 仅 3 处非库写忽略（scheduler.go:1075 ProbeForAccount、client.go:107/:124 io.Copy）；测试外零落库点用 `_ =` |
| 工作区零漂移 | 成立 | 除本报告外唯一未跟踪文件为前端代理产物 round137-frontend-findings.md |

## 1. 身份防线矩阵第五十二轮闭合

### 1.1 sameClientFor 定义与 7 调用点逐一确认零漂移

- 定义：scheduler.go:204 `sameClientFor(acct, chainClient)` → :205-209 `ClientFor(acct)` 存在性内含 + 指针身份比对（`!ok || current == nil` 即返回 false）；:215 `clientIdentity` 用 `reflect.ValueOf(c).Pointer()` 取具体指针类型底层指针值，:220-223 nil/非指针保护返回 0 恒非同一。grep 全仓 `clientIdentity(` 仅 :209/:215 出现，无旁路比对。零漂移。
- 7 调用点逐一追到"写什么状态 / 落什么库行"终局：
  - **:850**（ProbeForAccount 回写段，M88-01 单点）——网络 FindElectives 最长 15s 后锁内复核；不放行即放弃写 `acctData[acct]` / `acctDataAt[acct]`（:863-864）与 `openTimeDetected[acct]`（:856），绝不覆盖重建账号识别槽/年级串线。注释 :843-848 覆盖同名重建顶替语义。
  - **:1489**（spawnChain 失效分支）——身份不符锁内 `delete(s.inflight[acct], t.ClassID)`（:1490）后静默弃链；**:1499 `maybeRelogin` 落在复核之后**（注释 :1494-1497 明言"重登必须落在身份复核之后"）——旧链绝不消费新身份登录预算。
  - **:1521**（成功分支）——通过才 `done[acct][t.ClassID]=true`（:1529）+ `setStateLocked(success)`（:1530）+ `SaveSuccess` 落库（:1535）+ "报名成功" AppendLog（:1532）。注释 :1515-1520 覆盖重启 RestoreDone 假成功语义。
  - **:1551**（风控退避分支）——通过才 `markRateLimitedLocked`（:1555 写 30s 退避截止）+ setStateLocked(failed) + AppendLog（:1558）。注释 :1547-1550 覆盖假退避让黄金期被静默跳过语义。
  - **:1571**（窗口关闭分支）——通过才 `markFullLocked`（:1575 写 full 集合 + AppendLog :1770）。注释 :1565-1569 覆盖"无效的课程ID"按满员记 full 语义。
  - **:1600**（实时复核回锁后三路总闸）——存在性内含于 sameClientFor（注释 :1596-1599 六路对称声明）。不放行锁内直接 return，绝不让三路（失效重登 :1608-1619 / 确证满员 :1627-1641 / 普通失败 :1645-1666）写重建身份。
  - **:1635**（确证满员分支内的二重复核）——回锁后 :1600 总闸之外，:1635 在"确证满员"落 markFullLocked（:1639）前再复核一次（注释 :1631-1634 覆盖"入口存在性复核挡不住新身份存在但指针不同"）；:1627 `doneHas` 让位于手动 MarkDone 的胜利状态。第六分支 ErrUnauthorized（:1608）与 :1635 同族对称。

### 1.2 maybeRelogin 双侧 + 二次 ClientFor 终局

- **决策侧 :1208**——`reloginMu.Lock()` 后 `s.mu.Lock()`，锁内、任何 map 写入前 `ClientFor(acct)` 存在性复核（注释 :1202-1207 明言"探测定时三处对 ErrUnauthorized 直调本入口……账号已删不发起重登、不写任何 map"）；已删静默 return。随后才 read/write `relogging` / `reloginFail` / `reloginAt` / `tokenValid`（:1212-1241）。
- **写回侧 :1254**——goroutine 开头锁内 `ClientFor(acct)` 复核，已删整个成功分支（内存写 + 落库）静默放弃只清 relogging。
- **二次 ClientFor 终局 :1265-1273**——success 分支先清内存标记，再 `if client, ok := s.clients.ClientFor(acct); ok` → `if tok := client.Token(); tok != ""` → `UpdateIDToken(acct, tok)` 落库。落库 token 恒取自"当前注册表该账号客户端"，绝非 goroutine 发起时捕获的旧值（OBSERVE-117-01 关键性质，见 §2）。

### 1.3 手动五路 accountExists（判据同源，handler.go）

| 路径 | 行号 | 语义 |
|------|------|------|
| handleElectives（课程读） | :255 | 凭据表查无 → "账号不存在，无法查看课程" |
| handleElectiveSelect（手动报名） | :305 | 查无 → "账号不存在，无法执行报名操作" |
| handleElectiveExit（手动退选） | :397 | 查无 → "账号不存在，无法执行退选操作" |
| handleSetTargets（目标写） | :497-512 | 显式 LoadCredentials 循环比对（注释明言 authenticateDirect 直连建立会话的账号须被放行） |
| handleState（状态读） | :573 | 查无 → "账号不存在，无法读取状态" |

判据同源：`accountExists`（:1109-1120）= `LoadCredentials` 逐账号比对（凭据表= "确实登录过"的更强真理源，:1106-1108 注释）。`allowAccountOverride`（:1102-1104）仅管理员会话可穿透。五路全覆盖，无幽灵账号假装成功路径。

### 1.4 写点换类 5 类全持锁 + *Locked 族双向射证

- **lastSubmit**：写点在 submitAll 首行 :1346（`s.lastSubmit = s.nowAlignedLocked()`），持 s.mu；tick 判读 :1029 持锁取。
- **lastSyncStart**：写点 maybeSyncClock :356 持锁；读 :393（sync goroutine 持锁推进 lastSyncTime）。
- **syncing**：置位 :355（持锁）、复位 :369（sync goroutine 持锁）/ :404（无客户端 fallback 持锁）。
- **lastProbe**：写点四路全部持锁——ProbeNow :964 / probe 失败 :1089 / probe 成功 :1114 / Start 主循环重登回传 :679。注释 :868-871 明言"lastProbe 只归 probe() 与 ProbeNow 管理"。
- **state.EmptyProbeRuns**：写点 probe :1156/:1158 持锁（入账侧 +10s 裕量，:1151-1154 注释）。
- **\*Locked 写函数族**（持锁被调方）：nowAlignedLocked（:273 自实现不取锁，调用方持 s.mu）/ openTimeForLocked（:427）/ enrichTargetPubMetaLocked（:539）/ rebuildCoursesForAccountLocked（:570）/ rebuildCoursesLocked（:647）/ ProcessStart…（无）/ tokenValidForLocked（:739 只读）/ isRateLimitedLocked（:1691 读写）/ markRateLimitedLocked（:1710）/ releaseFullIfFreedLocked（:1781）/ statusIndexLocked（:1835）/ setStateLocked（:1846）/ markFullLocked（:1760）/ windowClosedLocked（:918）——全部只被锁内调用，名字即锁契约。
- **外部写函数首行取锁**双向射证：SetTargetsForAccount :455 / PurgeAccount :496 / RestoreTargets :526 / RestoreDone :608 / RestoreRefused :627 / MarkDone :1923 / RemoveDone :1990 / RemoveFull :2039 全部首行 `s.mu.Lock()`。全仓 grep 无"外部写点漏锁"形态。
- **无锁写点宿主唯一性射证**：`warnedNoTargets`（:1371-1372，submitAll 内写）唯一调用链 = tick（:1035 调 submitAll）→ Start goroutine for-select 主循环（:673-675 tick 唯一调用点），天然串行；`start` 字段 :664 持 s.mu 写。`chains` map 由独立 chainMu 保护（:1392-1403 三处）。无第二宿主。

## 2. OBSERVE-117-01 知识位第二十轮确认在位

- 写回侧先 :1254 `ClientFor(acct)` 复核（已删整体放弃，含内存写与落库）→ success 分支 :1265-1266 **另取一次当前注册表 client → client.Token()** → :1269 `UpdateIDToken(acct, tok)` 落库。落库 token 恒为重登完成后注册表内该账号客户端的实值，绝非 goroutine 发起时捕获的旧身份/旧 token。
- 结构性保证链完整：ReloginIfNeeded（client.go:602）→ Login（client.go:217）→ submitLogin 成功在 c.mu 内写 `c.token` 与 zd_edu_cookie（client.go:392-394）→ Token()（:195-199 锁内读当前值）→ UpdateIDToken 按 account 主键写新值（store.go:56-59）。
- 同名重建场景：删号后新 *Client 注册，旧重登 goroutine 返回时 :1254 正常通过（新身份存在），:1265 重取的正是**新身份**的当前 token——不串旧身份（B21-03 定版语义连续第二十轮零漂移）。
- 活化条件（未来改动破坏"落库取当前注册表 token"关键性质）未触发。

## 3. B110-01 审计链第二十七轮零漂移

- 手动 6 失败位 AppendLog 逐一就位：**handleElectiveSelect** :362（ErrUnauthorized "教务令牌失效，自动重登中"）/ :374（read 类"结果未知"最需留痕）/ :381（业务失败原文）；**handleElectiveExit** :443 / :452 / :459 三路对称。全部 `if err := ...; err != nil { log.Printf }` 零吞错。
- 成功审计行：手动 select → MarkDone 内 AppendLog（:1976）；手动 exit → RemoveDone 内 AppendLog（:2029）；login 日志 issueSession :237（含管理员 :136）；set_targets :558。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 确证满员 markFullLocked :1770 / 普通失败 :1662，全部落库失败记日志。
- 零吞错穷举（测试外）：`_ =` 仅 2 处非库写防御性忽略（MarkDone/RemoveDone 返回值，库写在其内部已处理）；`_, _ =` 仅 3 处（ProbeForAccount / io.Copy 两处）。零库写忽略。
- 网络层 token 脱敏延续：doRequest :422 拼 `?idToken=` → :447-450 网络错误走 `sanitizeError`（:581-593 剥 *url.Error 完整 URL 文本、Unwrap 下沉底层错误保判型穿透）→ 消费侧 maskedToken（scheduler.go:1283，len>8 只显前 8 位）。B101-01 延续无回归。

## 4. O105-01 抖动基线实测绿

- 夹具在位：zhidao/client_test.go:25 socketPreheat（:41-45 TestMain 包级预预热）+ :88 readyProbe（200ms×10+2s 宽栅栏）；captcha_test.go:17/:52 同款；api/handler_test.go:185 readyProbe；accounts/manager_test.go:26 readyProbe（三处夹具，无 socketPreheat 双保险的既有缺口已注释锁定）。scheduler 包无 mock 服务器夹具（纯 fake），不需就绪探测。
- 定向 race 四包实测全绿（时间见验证表），含身份防线族十测、回归锚两测、时钟时间基族、登录闸门族、手动审计族——全部一次通过，无 flake。

## 5. LOW-132/133 回首核

- **git show 白线核对**：975dc2b 仅 scheduler.go 2 行实质改动（:372 lastSyncFailAt / :377 syncFailedWindow 改 `nowAlignedLocked()`）+ 测试 49 行；d75f38c 仅 4 处判读侧 `time.Since` 改 `nowAlignedLocked().Sub`（ElectivesSnapshotFor ×2 :780/:795 / ElectivesSnapshot :881 / CheckClassSelectable :1871）+ 测试 27 行。均只动时间基域，身份防线零改动。
- **时间基全量扫零残留**（产品代码）：
  - scheduler.go:1220/:1227 `time.Since(reloginAt)` → 写 :1231/:1261 同 `time.Now()` 本地钟，读写同基自洽（relogin 退避是"距上次重登 30s"相对度量，与对齐钟无关），**合规**。
  - accounts/manager.go:71/:226 `time.Since(gateWindow)` → 写 :53/:74/:227 同本地钟，读写同基自洽（登录闸门"每分钟 2 次"为相对度量），**合规**。
  - client.go:135/:137 `time.Since(start)` → 本就是 RTT/clockOffset 计算需要本地钟，**非调度判读**。
  - session/store.go 与 handler.go:1176 的 time.Now 为会话/票据/限流桶过期判定（秒级精度，非调度敏感路径），**合规**。
- 全量扫零命中测试文件众多（New 的 openTime 参数、deadline 轮询），均为测试侧对本地钟的合法依赖，无产品代码混用孤岛。

## 6. 新契约角度纵深（自选 ×2）

### 6.1 手动报名/退选 4 方法协同族（TryAcquireSubmit / MarkDone / RemoveDone / RemoveFull）

选此方向的原因：手动与自动两类引擎的交汇点是删号竞态与胜利状态语义的最后博弈面，且契约文档 14-16 是高频引用区。

- **TryAcquireSubmit**（:1896）：锁内 inflight 位在飞互斥（false 即"该课程正在提交中"），`sync.Once` 包装释放函数（:1907-1912），与 spawnChain :1464 inflightHas 双闸免并发双发包。
- **MarkDone**（:1922）：首行取锁 + ClientFor 复核（删号放弃落库）→ done 置位 → **同步删库内 refused 单课行**（:1945 DeleteRefusedClass，注释：手动重报成功后残留行重启会被恢复成"已手动退选"）→ 清 inflight/full/rateLimited（:1950-1958）→ success 置位 → SaveSuccess：1973 + AppendLog：1976。手动成功即解除假退避（:1956-1958），与"手动重选表达接管意图"语义闭环。
- **RemoveDone**（:1989）：ClientFor 复核 → 删 done/inflight/full → refused 置位 → **DeleteSuccess 删 success 行**（:2020，注释：不然重启 RestoreDone 恢复假成功）/ SaveRefused（:2026，注释：不然重启自动引擎抢回退选课）/ AppendLog（:2029）。
- **RemoveFull**（:2038）：全锁清 full + failed→pending（:2045-2047）。
- 协同实证：handleElectiveSelect :328 acquire → :337 备选复核（CheckClassSelectable 全锁 :1863）→ :343 ClientFor → :387 MarkDone；handleElectiveExit :420 acquire → :428 ClientFor → :467 RemoveDone。六失败位 AppendLog 已在 §3 验证。
- **结论：四方法协同覆盖闭环，无竞态缺口，与回合 14 契约逐字对齐。**

### 6.2 激活票据 ConsumeTicket 先于校验 + 删号 memory-first 四序

选此方向的原因：两个"时序即安全"的经典契约，时间盒内可快速核到底。

- **ConsumeTicket 先于校验**：handleActivate :209 `ConsumeTicket(req.Ticket, acct)` 在 :213 `ConsumeActivationCode` 之前。ConsumeTicket（session/store.go:118-138）锁内校验存在/未用/未过期/账号绑定一致后 `t.used = true + delete` 一次性作废；注释 :114-117 明言"票据在激活码校验失败时已被销毁（单次防重放刻意决策），宁可输错激活码重登一次，绝不让同一票据反复探测不同激活码"。ConsumeActivationCode（store.go:292-324）单条 UPDATE 原子扣减（`used_uses < total_uses` 防超卖，:298）+ 已激活账号不双扣回滚（:313-318）。防穷举与防超卖双闭环。
- **删号 memory-first 四序**：handleAdminDeleteAccount 顺序 = :1040 `Accounts.Remove`（内存注册表先摘，让落库前锁内复核 ClientFor 的防线从一开始就生效）→ :1045 `Sched.PurgeAccount`（调度器全量清，注释明言"删账号路径必须走 PurgeAccount 而非 SetTargetsForAccount(nil)"）→ :1046 `Store.DeleteAccount`（库行，失败留半删态自愈注释 :1047-1048）→ :1054 `Sessions.RevokeAccount`（会话吊销，吊浏览令牌不等 12h TTL）。与决策契约 4 `Remove → PurgeAccount → DeleteAccount → RevokeAccount` 逐字对齐，注释 :1032-1039 直述空窗根因与防御顺序。
- **结论：双契约在位，零漂移。**

## 7. 维持观察项

- 无新增。既有观察项本轮零漂移。

## 8. 结论

**裁定：APPROVE**

身份防线矩阵第五十二轮闭合（sameClientFor 7 调用点逐一追到写状态/落库终局零漂移；maybeRelogin 双侧 + 二次 ClientFor 终局射证；手动五路 accountExists 判据同源；写点换类 5 类全持锁；*Locked 族与外部写函数双向射证；无锁写点宿主唯一性证明）。OBSERVE-117-01 知识位第二十轮在位。B110-01 审计链第二十七轮零漂移（手动 6 失败位 + 零吞错穷举 + token 脱敏延续）。O105-01 抖动基线定向 race 四包 + 身份防线族 + 回归锚全绿。LOW-132/133 回首白线核对通过 + 时间基残留全部读写同基自洽。两个自选纵深（手动 4 方法协同族 / 激活票据与删号四序）契约在位。契约轮次标签零命中。工作区除本报告与前端并行产物外零漂移。全轮无 CRITICAL/HIGH/MEDIUM/LOW 发现。