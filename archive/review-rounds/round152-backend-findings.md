# R152 后端只读审查 Findings —— 身份防线矩阵第六十七轮闭合

> 审查基线：`6768bc4`（R151 归档，身份防线矩阵第六十六轮闭合）。模式：绝对只读，唯一写文件为本报告。
> 时间：2026-09-24。实测工具：`go build` / `go vet` / `go test -race -count=1`（MinGW gcc，`export PATH=/d/mingw64/bin:$PATH && export CC=gcc`）+ 走读追写。

## 结论前置（分级）

**CRITICAL 0 / HIGH 0 / MEDIUM 0 / LOW 0（无新增缺陷）**

本轮为纯观察轮，身份防线矩阵第六十七轮闭合成立。sameClientFor 定义与 7 调用点零漂移、maybeRelogin 双侧完整、手动五路 accountExists 全覆盖、写点换类 5 类 + 无锁写点唯一性、*Locked 写函数族双向射证全部实测在位；OBSERVE-117-01 知识位第三十五轮、B110-01 审计链第四十二轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认。新契约角度两方向无漂移。

---

## 验证表（实测时间与数据）

| 验证项 | 结果 | 实测证据/数据 |
|--------|------|---------------|
| `go build ./...` | ✅ | EXIT=0，无输出 |
| `go vet ./...` | ✅ | EXIT=0，无输出 |
| 定向 race 四包（强制非缓存） | ✅ 全绿 | `-race -count=1`：zhidao 1.984s / accounts 1.422s / scheduler 15.011s / api 14.188s |
| 身份防线族（race） | ✅ 全绿 | 定向：TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin + TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight + TestMaybeReloginDeletedAccountSkipsMaps + TestProbeDeletedThenRebuiltSameNameDropsSnapshot + TestProbeForAccountDropsWriteWhenRemoved，全部 PASS（scheduler 2.470s / accounts 1.368s，`[no tests to run]` 属测试名不匹配的正常定向跳过） |
| 回归锚双测试 | ✅ 双绿 | TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler 已包含在 scheduler 全量 race 中 PASS |
| 凭据表五路 + 状态码家族（race 定向） | ✅ 全绿 | TestAdminElectivesUnknownAccountRejects / TestAdminElectiveSelectUnknownAccountRejects / TestAdminStateUnknownAccountRejects / TestSetTargetsUnknownAccountDoesNotFabricate / TestAdminDeleteAccountMemoryFirst / TestManualElectiveFailureAppendsLog / TestMaskedTokenBoundary / TestSanitizeError / TestIsReadErr / TestRequireJSONBody / TestRecoverMiddleware / TestLoginRateLimit / TestAuthRequired（api 2.409s / zhidao 1.480s 全 PASS） |
| 全量 go test ./...（无 race） | ✅ | 11 包全 ok（含 cmd/bench、cmd/logintest、cmd/probe、web 无测试文件） |
| 轮次标签扫描 | ✅ 零漂移 | 产品代码 `grep "第 N 轮\|R..轮\|round N\|RoundN"` 零命中；测试文件仅 4 处叙述性"第 3 轮"（scheduler_test.go:1652/:1659/:1662/:1674 描述同步失败循环语义、:2518 描述幽灵窗口空快照轮数）与 RoundTrip/roundtrip 测试命名，属行为叙述非轮次前缀标签，合规 |
| 工作区零漂移 | ✅ | 基线 `6768bc4` 与工作树 `git diff 6768bc4 --stat` 全空，`git status --porcelain` 仅 `?? archive/review-rounds/round152-backend-findings.md` 一个未跟踪文件（即本报告），产品代码零修改 |
| 时间基 LOW-132/133 回首 | ✅ | 见专项第 5 节 |

---

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十七轮闭合 —— ✅ 在位（7 调用点终局逐一追写）

**sameClientFor 定义与 7 调用点坐标核对（相对基线零漂移）**

- 定义 scheduler.go:204-210：锁内 `ClientFor(acct)` 存在 + nil 判 + `clientIdentity(current) == clientIdentity(chainClient)`；clientIdentity :215-224 用 `reflect.ValueOf(x).Pointer()` 对接口动态值取底层指针，nil→0、非 Ptr 动态类型→0，与契约一致。
- 实测 7 调用点行号：**:850（ProbeForAccount 回写段）、:1489（失效分支）、:1521（成功分支）、:1551（风控退避分支）、:1571（窗口关闭分支）、:1600（实时复核三路入口）、:1635（实时复核确证满员分支）**。

**各调用点"写什么状态/落什么库行"终局逐一追写：**

| 调用点 | 位置 | 身份复核失败时放弃的写入 |
|--------|------|--------------------------|
| 探测回写 | :850 | 放弃 `acctData[acct]`/`acctDataAt[acct]`/`openTimeDetected[acct]` 识别槽写入（:855-864 整块短路；纯内存帧不落库；防新旧年级帧串写重建身份） |
| 失效分支 | :1489 | 放弃 `delete(inflight)` 前的状态写 + maybeRelogin 触发 + failed 状态 + AppendLog 失效日志（:1490 delete 保留为公共清位；:1494-1508 整块短路；重登副作用前的身份前置保护 Manager 失败计数不被新身份污染） |
| 成功分支 | :1521 | 放弃 `done[acct][id]=true` + setStateLocked success + AppendLog(success) + SaveSuccess 库行（:1526-1540 整块短路，重启 RestoreDone 无假成功） |
| 风控退避 | :1551 | 放弃 `markRateLimitedLocked`（30s 退避）+ failed 状态 + AppendLog（:1555-1561 整块短路，黄金期不被假退避静默跳过） |
| 窗口关闭 | :1571 | 放弃 `markFullLocked`（永久满员退避）+ 状态 + AppendLog（:1575 短路） |
| 实时复核入口 | :1600 | 拦截其下三路（ErrUnauthorized→maybeRelogin/失败写库；确证满员→markFullLocked；普通失败→failed+AppendLog）整块写面 |
| 实时复核确证满员 | :1635 | 在 doneHas 让位（:1627 绝不覆盖胜利状态）之后加码身份复核，放弃 markFullLocked（:1639 短路） |

六分支对称结论：成功/失效/风控/窗口关闭/确证满员/实时复核 ErrUnauthorized（:1608）全族闭合，加上栈顶已删检查（:1388）与链顶取 client 后二次判（:1409），八道防线成族，无裸露写点。

**maybeRelogin 双侧确认（:1198-1299）：**

- 决策侧 :1208：入口 `reloginMu.Lock()` + `s.mu.Lock()` 后先 `ClientFor(acct)` 存在性复核，!ok 即整体 return——绝不写 tokenValid/reloginFail/reloginAt/relogging 任何 map（TestMaybeReloginDeletedAccountSkipsMaps 回写侧 + 决策侧四 map 全空实证），与 B43-01 契约一致。
- 写回侧 :1254：goroutine 内先 ClientFor 复核→:1259 `err==nil && relogged` 成功分支→**:1265 二次 ClientFor 重取当前注册表 client `client.Token()` 落库（:1269 UpdateIDToken）**——OBSERVE-117-01 知识位第三十五轮在位，落库写入方是二级复核后的最新客户端指针，结构上彻底排除"写的是旧身份 token"。失败侧 :1292-1297 只清 relogging、reloginFail 保留递增后的计数（指数退避表不振荡，语义自洽）。
- 手动路径 `MaybeRelogin` 导出别名（:1323）与 `MarkTokenValid`（:1306-1319，含 reloginMu→s.mu 锁序对齐，与 TokenValidFor/maybeRelogin 同序）对称闭环。
- 双重写点结构确认：同一 acct 下 ClientFor 重取在 :1254（存在性）→ :1265（取 Token）两次独立调用，删号竞态两边同挡。

**手动五路 accountExists 确认（handler.go）：**

- 课程读 :255、手动报名 :305、手动退选 :397、状态读 :573、目标写 :497-512（LoadCredentials 循环判 found）——五路全部凭据表判据，accountExists 定义 :1109-1120，允许 override 门 :1103 仅管理员会话。实测 5 路测试全绿（见验证表）。
- 目标写 :497 走内联 LoadCredentials 而非 accountExists 函数（"仅用 accounts 表会误伤 authenticateDirect 直连建立会话的已登录账号"注释），判据同源（凭据表）且覆盖面与 :555 一致，属同一契约的两处实现。

**写点换类 5 类 + 无锁写点唯一性：**

- lastSubmit（submitAll :1346 锁内 `s.nowAlignedLocked()`，读侧 tick :1029 锁内）、lastSyncStart（maybeSyncClock :356 锁内置位 / :393 锁内读 / :405 锁内复位）、syncing（:355 锁内置位 / :369 锁内回写 / :404 锁内复位）、lastProbe（probe :1089/:1114、ProbeNow :964 均锁内；tick 读 :976 锁内；tick 主循环 :679 重登成功后锁内复位清零）、state.EmptyProbeRuns（probe :1156/:1158 锁内）——全部持锁写、读侧持锁，零裸露。
- 无锁写点 warnedNoTargets：唯一宿主 submitAll :1371-1372（`if !s.warnedNoTargets { s.warnedNoTargets = true; log... }`），判定在 :1368 `if len(chains) == 0` 之后、:1375 return 之前——链式 if 内部连续执行，host 唯一、单写者、初始 zero-value false 在 SetTargets 后有目标再全删时必然走到。唯一写点、唯一读点（自身判定），语义自洽。

***Locked 写函数族 13 个 + 外部写函数首行取锁双向射证：**

- 定义 13 个：nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846。实测每个只被 Locked 上下文调用（追到 spawnChain 锁段 / tick 锁段 / StateForAccount 锁段 / WindowClosed 锁段 / purge/restore 锁段等），无锁外裸调。
- 外部写函数首行取锁核查：SetTargetsForAccount :455、PurgeAccount :496、RestoreTargets :526、RestoreDone :608、RestoreRefused :627、ProbeForAccount 回写段 :849（锁 + sameClientFor 前置）、ProbeNow :963、ElectivesSnapshotFor :762、MarkDone :1923、RemoveDone :1990、RemoveFull :2039、TryAcquireSubmit :1897 全部首行取锁。双向射证成立。

### 2. OBSERVE-117-01 知识位第三十五轮 —— ✅ 在位

见第 1 节 maybeRelogin 写回侧（:1254→:1265-1273）。二次 ClientFor 重取当前注册表 `client.Token()` 落库，同名重建场景绝不串旧身份。已删账号两重防线（写回侧首查 + 二级取 Token 前重查）。TestReloginSuccessWithNilStoreNoPanic 等链上测试全绿。

### 3. B110-01 审计链第四十二轮 —— ✅ 零漂移

- 手动 6 失败位 AppendLog：handler.go :362（报名失效）、:374（报名 read）、:381（报名业务失败）、:443（退选失效）、:452（退选 read）、:459（退选业务失败）——全部 `if ... != nil { log.Printf }` 零吞错。
- 成功行：报名走 MarkDone 内 AppendLog（scheduler.go:1976）、退选走 RemoveDone 内 AppendLog（:2029）、目标保存 :558、登录 :237、管理员 login :136、删除 :1055。全部带错误处理。
- 自动链失败族：spawnChain 六分支 AppendLog 全部 `if err := ...; err != nil { log.Printf }`（:1504/:1532/:1558/:1614/:1662/:1770）。
- 零吞错穷举：`_ =`/`_, _ =`/直接赋值忽略形态全量扫描——scheduler.go `_ =`（:307 Prewarm goroutine）、`_, _ =`（:1075 ProbeForAccount 探测 goroutine），handler.go `_ =`（:387 MarkDone、:467 RemoveDone）——四处均**非落库**：:307 预热并发 goroutine 结果不需上抛、:1075 探测并发 goroutine 丢返回值（错误已由 ProbeForAccount 内部处理/日志）、:387/:467 手动成功路径 error 恒 nil（函数内绝不返回错误语义分支）。全部 store.sessionX 落库点 grep 穷举（SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefused/AppendLog/UpdateIDToken/SetTargetsForAccount）零"忽略错误"形态，全部 `if err != nil { log.Printf }`。
- 网络层 token 脱敏延续抽查：zhidao/client.go:450 doRequest 统一 `sanitizeError`（:581-593 剥 url.Error URL 文本、保留 Unwrap 判型下沉），scheduler.go:1283 重登成功日志走 maskedToken（:1334，>8 位仅前 8 位）；TestMaskedTokenBoundary / TestSanitizeError 系列全绿。
- 连接活性自愈族：httpDo :478-492（dial/write 重试一次，read 不重试防双报），isConnErrRetryable :496（仅 dial/write Op），IsReadErr :523（io.EOF/io.ErrUnexpectedEOF/超时文案/net.OpError read），两复用路径统一收口。TestIsReadErr 系列全绿。

### 4. O105-01 抖动基线 —— ✅ 实测绿

- socketPreheat/readyProbe 夹具在位：zhidao/client_test.go:25/:88、captcha_test.go:17/:29/:52/:79、sanitize_test.go:97、api/handler_test.go:139/:185、accounts/manager_test.go:26（manager 注释明确"三处夹具未有 socketPreheat 双保险，8 轮全绿实证无残余"）。尚覆盖 scheduled probes。
- 定向 race 实测（`/d/mingw64/bin/gcc`）：zhidao/accounts/scheduler/api 四包全绿（含多轮与定向身份族）。
- 回归锚 TestWindowOpenSubmitsWithoutProbeReset + TestAdminStatsWindowOpenedUsesScheduler 已包含在 scheduler 全量 race 中绿。

### 5. LOW-132 / LOW-133 回首核 —— ✅ 通过

- `time.Since()/time.Now()` 产品代码全量扫：scheduler.go 仅 7 处——:269/:274（nowAligned 定义本尊）、:1220/:1227/:1231/:1261（reloginAt 写读）+ :1707 注释。判定：残余仅 `reloginAt` 写（:1231/:1261 本地钟）读（:1220/:1227 time.Since 本地钟）同基自洽（30s 重登节流语义不依赖对齐钟）+ `gateWindow`（accounts/manager.go：gateWait :53/:54/:61、gateTryAcquire :226/:227、GatePump :71/:74 三处本地钟写读同基）——两处均合规，与 R134-R151 各轮裁决一致，零新增时间基孤岛。
- git show 白线核对（相对基线无新增混用）：`git diff 6768bc4` 全空，无漂移。

### 6. 新契约角度纵深（自选 ×2，时间盒内完成）

**角度 A：窗口状态三判据单源 open 快照 + 幽灵窗口空快照入账（选择理由：windowClosedLocked 是调度器最核心判定，三判据共享同一 open 快照语义是 R142-R151 连续多轮的纵深主轴）**

- `windowClosedLocked` :918-939：三判据单源——① 主判据 `s.state.WindowClosed`（probe :1146 写入：`prevOpened && !opened && len(Publishes)==0 && now.After(open.Add(10s))`）；② 时钟连续失败 ≥3 且开放时间非零且已过（:925，`syncFailStreak`+`open`+`nowAlignedLocked`）；③ 幽灵窗口 `!WindowOpened && EmptyProbeRuns>=3` 且开放时间非零且已过（:935）。
- **单快照复用**：judgement ② 与 ③ 用 `open := s.openTimeForLocked("")` 取一次快照（:924）——识别槽是唯一事实源，一次读取避免"识别值在两次读取间被新批次覆盖"的不一致窗口；probe 入账段（:1145/:1154 裕量 + 主判据 :1146 + EmptyProbeRuns 入账 :1155）同样取一次 open 复用。热改亚毫秒窗口内不存在两护栏读到新旧两个值的分叉。
- **状态机验证**：probe :1124-1158——先捕获 prevOpened 再覆写（:1126/:1127，避免覆写后读取拿恒新值），主判据 `prevOpened && !opened && 空 && 已过 10s`（开过再关 = 实质关闭）；EmptyProbeRuns 入账 `!opened && 空 && 已过 10s`（:1155），否则归零（:1158）；绝不触碰 state.WindowClosed（注释显式）。测试 TestWindowClosedState / TestWindowOpenSubmitsWithoutProbeReset / TestGhostWindow 系列绿。判据取单快照、入账取单快照、ProbeForAccount 识别槽写入 :856 持 s.mu，无锁外写。

**角度 B：年级隔离快照回退链三态 + 删账号 memory-first 四序（选择理由：快照回退链是"浏览不触发刷新/触发真刷新/回退全局"三态判据，删账号四序是 identity 防线的构造前提，两者交叉验证防线有效性）**

- `ElectivesSnapshotFor` :761-805 三态判据：① 有目标账号（`len(acctTargets[acct])>0`）：专属帧新鲜（≤snapshotTTL）→ 返回 (data,true)；专属帧缺失/过期 → 返回 (nil,false) 触发 ProbeForAccount 真刷新，**绝不回退全局 lastData**（防年级串线；判据用 len>0 含空 slice 语义：清空目标的账号落 `[]` 键存在但按 len=0 走纯浏览态）；② 无目标账号但曾有专属帧：新鲜即返回、过期同样 (nil,false) 触发刷新（"回退并不比刷新快 + 数据是错年级"注释明确）；③ 从未有过专属帧：才允许回退全局 lastData（:801），过期也 (nil,false)。
- 与 CheckClassSelectable :1863-1892 交叉验证：手动报名复核只读该账号专属帧、过期一律放行（绝不跨账号回退），同快照回退契约。TestElectivesSnapshotFallback 防御性基线全绿。
- 删账号 memory-first 四序：handler.go:1040-1054 `Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount`。Remove 前置让在飞链的落库前复核（MarkDone/RemoveDone 的 ClientFor 检查、spawnChain 成功分支 sameClientFor）立即失败静默放弃，毫秒级空窗从根因消除；PurgeAccount（scheduler.go:495-519）删 11 类 map + state.Courses 行；DeleteAccount 失败仅半删态（注册表已摘，重启 Restore 自愈），Sessions.RevokeAccount 兜底吊销既有令牌。与身份防线族互证：四序时序恰是 sameClientFor 六分支测试构造的竞态窗口。

---

## 维持观察项（无新增，延续既有记录）

1. `IsClassFull` 实时复核仍受 CountEntry.MaxCount 未实证下发限制——恒 false 兜底路径，真满员主判据为快照 max_count，属既有契约非缺陷。
2. `RemoveFull`（:2038）目前仅有定义无产品调用方（外部手动/snapshot 更新语义保留），属预留接口，非死代码但未来消费前保持观察。
3. `syncFailedWindow` 写而不读（:182/:377），判据用 syncFailStreak——留档字段语义自洽。
4. 验证码识别引擎 ddddocr 本地推理在 CGO=0 交叉编译形态依赖 native_ocr_stub 回退，属既有双轨契约。
5. gateWait 阻塞型与 gateTryAcquire 非阻塞型共享 gateUsed 计数——退避窗口内手动登录立即被拒但排队重登不受影响，语义经测试固化。
6. accounts 包测试夹具无 socketPreheat 双保险（manager_test.go:24 注释自认），8 轮全绿实证无残余——若未来再出冷启动 flake 第一候选即补 socketPreheat。

---

## 结尾建议

**APPROVE**

身份防线矩阵第六十七轮闭合成立，sameClientFor 7 调用点终局逐一追写零漂移，maybeRelogin 双侧 + 二次 ClientFor 重取 Token 落库、手动五路 accountExists、写点换类 5 类 + warnedNoTargets 唯一性、*Locked 写函数族 13 个双向射证全部实测确认；OBSERVE-117-01 知识位第三十五轮、B110-01 审计链第四十二轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（窗口三判据单源 open 快照 / 年级隔离快照回退链三态 + 删账号四序互证）无漂移。CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。