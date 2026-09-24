# R153 后端只读审查 Findings —— 身份防线矩阵第六十八轮闭合

> 审查基线：`7b319be`（R152 归档，身份防线矩阵第六十七轮闭合）。模式：绝对只读，唯一写文件为本报告。
> 时间：2026-09-24。实测工具：`go build` / `go vet` / `go test -race -count=1`（MinGW gcc，`export PATH=/d/mingw64/bin:$PATH`）+ 走读追写终局。

## 结论前置（分级）

**CRITICAL 0 / HIGH 0 / MEDIUM 0 / LOW 0（无新增缺陷）**

身份防线矩阵第六十八轮闭合成立、零漂移。sameClientFor 定义（scheduler.go:204）与 7 调用点逐一追写终局确认无裸露写点；maybeRelogin 双侧（决策侧 :1208 / 写回侧 :1254 / 二次 ClientFor 重取 Token 落库 :1265-1273）完整；手动五路 accountExists 全覆盖；写点换类 5 类 + 无锁写点 warnedNoTargets 唯一性射证；*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证成立。OBSERVE-117-01 知识位第三十六轮、B110-01 审计链第四十三轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（连接活性自愈族 httpDo 判型 / 登录闸门族 B42-01 双侧收口）无漂移。

---

## 验证表（实测时间与数据）

| 验证项 | 结果 | 实测证据/数据 |
|--------|------|---------------|
| `go build ./...` | ✅ | EXIT=0，无输出 |
| `go vet ./...` | ✅ | EXIT=0，无输出 |
| 定向 race 四包（强制非缓存） | ✅ 全绿 | `-race -count=1`：zhidao 2.552s / accounts 1.737s / scheduler 15.378s / api 16.957s |
| 身份防线族 + 回归锚（race 定向） | ✅ 全绿 | 15 测定向 PASS：TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin + TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight + TestMaybeReloginDeletedAccountSkipsMaps + TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime + TestProbeNowUnauthorizedTriggersRelogin + TestDeletedAccountInFlightDropsSuccess + TestRealtimeRecheckDeletedAccountDropsLog + TestUnauthorizedBranchDeletedAccountSkipsState + TestSchedulerManualSyncAndSubmitMutex + TestWindowOpenSubmitsWithoutProbeReset + TestAdminStatsWindowOpenedUsesScheduler（scheduler 2.157s / api 0.720s） |
| api 状态码家族 + 凭据表五路（race 定向） | ✅ 全绿 | 9 测 PASS：TestRequireJSONBodyRejectsFormContentType / TestRecoverMiddlewareHidesPanicDetail / TestAdminStatsWindowOpenedUsesScheduler / TestAdminElectivesUnknownAccountRejects / TestAdminElectiveSelectUnknownAccountRejects / TestAdminStateUnknownAccountRejects / TestSetTargetsUnknownAccountDoesNotFabricate / TestManualElectiveFailureAppendsLog / TestLoginRateLimit（api 2.572s） |
| 既有守卫契约 | ✅ 绿 | TestSubmitSuspendedWhenOpenTimeCleared（B41 零值守卫回归）scheduler 1.521s PASS |
| 轮次标签扫描 | ✅ 零漂移 | 产品代码仅 `internal/session/store.go:117` 引用 `docs/review-round13.md`（历史文档路径，合规）；测试文件 4 处"第 3 轮"（scheduler_test.go:1659/:1662/:2518 描述同步失败循环与幽灵窗口空快照轮数，行为叙述合规），零轮次前缀标签 |
| 工作区零漂移 | ✅ | 工作树仅 `?? archive/review-rounds/round153-frontend-findings.md` 一个未跟踪文件（前端报告的产物），`git diff 7b319be -- backend/` 全空，产品代码零修改 |
| 时间基 LOW-132/133 回首 | ✅ | 见专项第 5 节 |

---

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十八轮闭合 —— ✅ 在位（7 调用点终局逐一追写）

**sameClientFor 定义与坐标核对（相对基线零漂移）**

- 定义 scheduler.go:204-210：锁内 `ClientFor(acct)` 存在 + nil 判 + `clientIdentity(current) == clientIdentity(chainClient)`；clientIdentity :215-224 用 `reflect.ValueOf(x).Pointer()` 取接口动态值底层指针，nil→0、非 Ptr 动态类型→0。与契约一致。
- 实测 7 调用点：**:850（ProbeForAccount 回写段）、:1489（失效分支）、:1521（成功分支）、:1551（风控退避分支）、:1571（窗口关闭分支）、:1600（实时复核三路入口）、:1635（实时复核确证满员分支）**——与 R152 坐标逐字符一致，零漂移。

**各调用点"写什么状态/落什么库行"终局逐一追写：**

| 调用点 | 位置 | 身份复核失败时放弃的写入 |
|--------|------|--------------------------|
| 探测回写 | :850 | 放弃 `acctData[acct]`/`acctDataAt[acct]`/`openTimeDetected[acct]` 识别槽写入（:855-864 整块短路；纯内存帧不落库；防新旧年级帧串写重建身份）。已删/身份变 → 返回 data+日志不写帧 |
| 失效分支 | :1489 | 先清 inflight（:1490 保留公共清位）→ 放弃 maybeRelogin 触发 + failed 状态 + AppendLog 失效日志（:1494-1508 整块短路）。身份前置保护 Manager 失败计数（reloginFail）不被新身份污染 |
| 成功分支 | :1521 | 放弃 `done[acct][id]=true` + setStateLocked success + AppendLog(success) + SaveSuccess 库行（:1526-1540 整块短路，重启 RestoreDone 无假成功） |
| 风控退避 | :1551 | 放弃 `markRateLimitedLocked`（30s 退避）+ failed 状态 + AppendLog（:1555-1561 整块短路，黄金期不被假退避静默跳过） |
| 窗口关闭 | :1571 | 放弃 `markFullLocked`（永久满员退避）+ 状态 + AppendLog（:1575 短路） |
| 实时复核入口 | :1600 | 拦截其下三路（ErrUnauthorized→maybeRelogin/失败写库 :1608-1619；确证满员→markFullLocked :1639；普通失败→failed+AppendLog :1660-1665）整块写面 |
| 实时复核确证满员 | :1635 | 在 doneHas 让位（:1627 绝不覆盖手动胜利状态）之后加码身份复核，放弃 markFullLocked（:1639 短路） |

六分支对称结论：成功/失效/风控/窗口关闭/确证满员/实时复核 ErrUnauthorized（:1608）全族闭合叠合，加上 submitAll 遍历判 ClientFor（:1355）、spawnChain 入口判 ClientFor（:1388）、goroutine 取 client 二次判（:1405-1409）、inflight 双门口（:1464/:1471）、doneHas/refusedHas/fullHas 门（:1436/:1447/:1452）——整链无裸露写点。

**maybeRelogin 双侧确认（:1198-1299）：**

- 决策侧 :1208：入口 `reloginMu.Lock()` + `s.mu.Lock()`（:1199/:1201）后先 `ClientFor(acct)` 存在性复核，!ok 即整体 return——绝不写 tokenValid/reloginFail/reloginAt/relogging 任何 map。TestMaybeReloginDeletedAccountSkipsMaps 四 map 全空实证（决策侧 + 写回侧全绿）。
- 写回侧 :1254：goroutine 内成功分支前先 ClientFor 复核 → :1259 `err==nil && relogged` 成功分支 → :1265 **二次 ClientFor 重取当前注册表 client** → `client.Token()` :1266 取新 token → :1269 UpdateIDToken 落库（:1268 nil store 判）。OBSERVE-117-01 知识位在位：落库写入方是二级复核后的最新客户端指针，同名重建场景新身份 token 落库必然取自新身份，结构上排除"写旧身份 token"。失败侧 :1292-1297 只清 relogging、reloginFail 保留递增计数（指数退避表不振荡）。
- 重登族 map 写点全量盘点：tokenValid/reloginFail/reloginAt/relogging 写入仅发生在 maybeRelogin 决策段（:1231/:1236/:1241）与成功分支（:1260/:1261/:1262）、清理在 PurgeAccount（:507-510）与 MarkTokenValid（:1316-1318）与失败清 relogging（:1247）与退避窗口过 delete reloginAt（:1226）——全部持 reloginMu+s.mu（MarkTokenValid 同序对齐），零锁外写。

**手动五路 accountExists 确认（handler.go）：**

- 课程读 :255、手动报名 :305、手动退选 :397、状态读 :573、目标写 :497-512（内联 LoadCredentials 循环判 found，注释明示"仅用 accounts 表会误伤 authenticateDirect 直连"的判据同源变体）——五路全部凭据表判据。accountExists 定义 :1109-1120（LoadCredentials 失败返回 false），允许 override 门 :1103 仅管理员会话（Sessions.IsAdminToken）。5 路测试全绿（见验证表）。目标写 :497 与 :555 覆盖面一致，属同一契约两处实现。

**写点换类 5 类 + 无锁写点唯一性：**

- lastSubmit（提交节流）：submitAll :1346 锁内 `s.nowAlignedLocked()` 写，tick :1029 锁内读。时间基对齐钟双端。
- lastSyncStart（时钟同步发起时刻）：:356 锁内置位、:393 锁内读（成功推进 lastSyncTime）、:405 锁内复位。全部持锁。
- syncing（时钟单飞标记）：:355 锁内置位、:369 goroutine 锁内回写复位、:404 无客户端路径锁内复位。全部持锁。
- lastProbe（全校探测节流闸门）：probe :1089（失败计入）/ :1114（成功计入）、ProbeNow :964（管理员强制刷新）、Start 主循环 :679（重登成功回传统一处清零补探测，注释明确"避免 goroutine 并发写 s.lastProbe 竞态"）——全部锁内写，tick :976 锁内读。ProbeForAccount 刻意不写（:868 注释：只归 probe/ProbeNow 管理，防止管理员穿透探测旁路全校节流）。
- state.EmptyProbeRuns（幽灵窗口量变）：probe :1156（+1）/ :1158（归零）锁内，windowClosedLocked :935 锁内读。判据侧 10s 裕量与入账侧 10s 裕量同一 open 快照（:1145）。
- 无锁写点 warnedNoTargets :1371-1372：唯一宿主 = submitAll（判定在 :1368 `len(chains)==0` 之后、:1375 return 之前）。调用方唯一（tick :1035），单写者单读者同一链式 if 连续执行，初始 zero-value false，无目标时必走到。**注意：此写点确实位于 s.mu.Unlock()（:1367）之后无锁执行**——但宿主唯一性成立（全文件仅此一处读写，grep 实证），是该字段语义下的合规无锁单写者，非漂移。消除需提锁申请一个锁内整包 flag 判断段，成本大于收益，维持观察（见观察项 7）。

***Locked 写函数族 13 个 + 外部写函数首行取锁双向射证：**

- 定义 13 个：nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846。逐一追调用上下文全在锁段内（spawnChain 锁段 / tick 锁段 / StateForAccount / WindowClosed / probe / purge / restore / TryAcquireSubmit / MarkDone / RemoveDone / RemoveFull），无锁外裸调。
- 外部写函数首行取锁核查：SetTargetsForAccount :455、PurgeAccount :496、RestoreTargets :526、RestoreDone :608、RestoreRefused :627、ProbeForAccount 回写段 :849（锁 + sameClientFor 前置复核锁内）、ProbeNow :963、ElectivesSnapshotFor :762、TryAcquireSubmit :1897、MarkDone :1923、RemoveDone :1990、RemoveFull :2039——全部首行取锁，双向射证成立。

### 2. OBSERVE-117-01 知识位第三十六轮 —— ✅ 在位

见第 1 节 maybeRelogin 写回侧。关键证据链：:1246 goroutine 内 `s.mu.Lock()` → :1247 清 relogging → :1254 `ClientFor(acct)` 存在性复核（已删则整体放弃，只清标记）→ :1259 成功分支 → :1265 二次 `ClientFor(acct)` 重取当前注册表 client → :1266 `client.Token()` 取新 token → :1269 UpdateIDToken 落库。同名重建场景：第一次跟第二次复核之间注册表被顶替时，第二次重取即拿到新身份客户端，落库写入方永远是"当前最新注册的客户端身份"——结构与 R152 逐字符一致，零漂移。TestReloginSuccessWithNilStoreNoPanic、TestDeletedAccountReloginSuccessDropsState、TestDeletedAccountRebuiltSameNameChainDropsRelogin 全绿。

### 3. B110-01 审计链第四十三轮 —— ✅ 零漂移

- 手动 6 失败位 AppendLog：handler.go :362（报名失效）、:374（报名 read）、:381（报名业务失败）、:443（退选失效）、:452（退选 read）、:459（退选业务失败）——全部 `if ... != nil { log.Printf }` 零吞错。
- 成功行：报名成功走 MarkDone 内 AppendLog（scheduler.go:1976）、退选成功走 RemoveDone 内 AppendLog（:2029）、目标保存 :558、登录 :237、管理员 login :136、删除账号 :1055、配置热改 :864、登出 :616——全部带错误处理。
- 自动链失败族：spawnChain 六分支 AppendLog 全部 `if err := ...; err != nil { log.Printf }`（:1504/:1532/:1558/:1614/:1662/:1770）+ markFullLocked（:1770）+ MarkDone 内 SaveSuccess/AppendLog（:1973/:1976）+ RemoveDone 内 DeleteSuccess/SaveRefused/AppendLog（:2020/:2026/:2029）+ SetTargetsForAccount 内 SetTargetsForAccount/DeleteRefused（:476/:484）+ 重登成功 UpdateIDToken（:1269）。
- 零吞错穷举：`_ =`/`_, _ =`/直接赋值忽略形态全量扫描——scheduler.go `_ =`（:307 Prewarm 异步 goroutine 丢返回值）、`_, _ =`（:1075 ProbeForAccount 探测 goroutine 丢返回值），handler.go `_ =`（:387 MarkDone、:467 RemoveDone）——四处全部**非落库**：:307 预热结果不上抛、:1075 探测错误已在 ProbeForAccount 内部处理、:387/:467 手动成功路径 error 恒 nil（业务错误全部前置 return）。所有 `store.Save*/Delete*/AppendLog/UpdateIDToken/SetTargetsForAccount` 落库点 grep 穷举零"忽略错误"形态，全部 `if err != nil { log.Printf }`。
- 网络层 token 脱敏延续抽查：zhidao/client.go:450 doRequest 统一 `sanitizeError`（:566-593 剥 url.Error URL 文本、Unwrap 下沉保留 errors.As/Is 判型、`ue.Op==""` 防御），scheduler.go:1283 重登成功日志走 maskedToken（:1334，>8 位仅前 8 位、≤8 位 "***"）。TestMaskedTokenBoundary / TestSanitizeError 系列绿。

### 4. O105-01 抖动基线 —— ✅ 实测绿

- socketPreheat/readyProbe 夹具在位：zhidao/client_test.go:25/:88、captcha_test.go:17/:29/:52/:79、sanitize_test.go:97、api/handler_test.go:67（套接字预创建）/179/:185、accounts/manager_test.go:26（readyProbe，注释自认无 socketPreheat 双保险、8 轮全绿实证无残余，延续 R152 观察项）。
- 定向 race 实测（`/d/mingw64/bin` gcc 路径下）：zhidao/accounts/scheduler/api 四包 `-race -count=1` 全绿（2.552/1.737/15.378/16.957s）。
- 回归锚 TestWindowOpenSubmitsWithoutProbeReset + TestAdminStatsWindowOpenedUsesScheduler 定向 PASS（见验证表）。

### 5. LOW-132 / LOW-133 回首核 —— ✅ 通过

- `time.Since()/time.Now()` 产品代码全量扫：scheduler.go 残余仅两族——① `reloginAt` 写（:1231/:1261 本地钟）读（:1220/:1227 `time.Since(t)` 本地钟）**同基自洽**（30s 重登节流语义不依赖服务端对齐钟，同一重登流程内部时间差）；② accounts/manager.go `gateWindow` 三处（gateWait :53/:54/:61、gateTryAcquire :226/:227、GatePump :71/:74）本地钟写读**同基自洽**（分钟窗口闸门语义为本地时间差）。其余探测族时间戳（lastProbe/lastSubmit/lastSyncStart/lastSyncFailAt/lastSyncTime/lastPrewarm/EmptyProbeRuns 入账侧/rateLimited 截止）全部对齐钟写、对齐钟读——与 R134-R152 各轮裁决一致，零新增时间基孤岛。
- git show 白线核对：`git diff 7b319be -- backend/` 全空，无逐字节漂移。

### 6. 新契约角度纵深（自选 ×2，时间盒内完成）

**角度 A：连接活性自愈族 httpDo 判型（选择理由：重试安全性是防轰炸契约的最后闸门——dial/write 重试安全、read 重试对 POST 会双报凿穿幂等，且与"可能已处理"文案链、token 脱敏链共享同一错误语义分型）**

- `httpDo` :478-492：首次 `c.Do` 后仅 `isConnErrRetryable` 命中才 `cloneReq` 重发一次；read/业务/取消错误原样上抛。`cloneReq` :554-561 深拷贝（GetBody 重放保 ContentLength 同步）。
- `isConnErrRetryable` :496-505：仅 `*net.OpError` 的 `dial`/`write` 两 Op 归可重试（请求未到达服务端，重发安全）；read 恒 false（互斥注释 :522）。
- `IsReadErr` :523-545：RST（Op=="read" 且 errors.As 穿透 url.Error）、FIN（errors.Is io.EOF）、短读（errors.Is io.ErrUnexpectedEOF）、超时（两文案 Contains）四种形态独立程序实证，与服务端"已完整消费 body 可能已处理"语义绑定。
- 收口面：doRequest :444 经 httpDo 唯一路径 + captcha 识别复用同家族（captcha.go）；scheduler.go :1656 zhidao.IsReadErr 区分"可能已处理"文案、handler.go :371/:450 手动路径同款。TestIsReadErr / TestReadErrRetryable 系列绿。本轮新增交叉验证：isConnErrRetryable 与 IsReadErr 组成完整互斥分型（dial/write / read+EOF+短读+超时），无第三种裸 `*url.Error` 形态流出——错误链到调度器身份复核/风控归并（isRateLimitError/isWindowClosedError）前已被此分型消化，与 sameClientFor 六分支的 err 归并路径完整闭合。

**角度 B：登录闸门族 B42-01 双侧收口（选择理由：doLogin 频率闸门是平台锁号防线的咽喉，阻塞/非阻塞双语义共享 gateUsed 计数的正确性直接影响多账号失效期的 IP 安全）**

- `gateWait` :49-64：`gateCond.Wait()` 阻塞等下一个分钟广播窗口，多账号重登排队保证全部完成；`GatePump` :68-77（main.go:153-159 每 30s 后台协程调用）窗口翻页 Broadcast。
- `gateTryAcquire` :223-235：非阻塞准入（同一 gateMu/gateUsed 共享计数、`time.Since(gateWindow)>=time.Minute` 翻页），quota 满立即返回 false。
- 双侧收口验收：`Manager.Relogin` :173 阻塞 gateWait（自动重登排队）；`LoginByPassword` :244 非阻塞 gateTryAcquire（手动登录全用 quota 满即拒、绝不挂起用户响应），`wasShell` 判别（:253）保"登录失败只摘空壳"。api 测试夹具 `ResetGateForTest`（:82-87）隔离闸门计数。
- 执行路径核对：manualRegister/issueSession 触碰 LoginByPassword 全收口 + scheduler.MaybeRelogin→Manager.Relogin 全排队。B42 契约（"绝不阻塞用户登录响应 + 排队重登共享 quota"）在锁与 cond 层面双闭环，无绕行旁路。四包 race 下无死锁。

---

## 维持观察项（延续既有记录 + 本轮新增）

1. `IsClassFull` 实时复核受 CountEntry.MaxCount 未实证下发限制——恒 false 兜底路径，真满员主判据为快照 max_count，属既有契约非缺陷。
2. `RemoveFull`（:2038）目前仅有定义无产品调用方（外部手动/snapshot 更新语义保留），预留接口，未来消费前保持观察。
3. `syncFailedWindow` 写而不读（:182/:377），判据用 syncFailStreak——留档字段语义自洽。
4. 验证码识别引擎 ddddocr 本地推理在 CGO=0 交叉编译形态依赖 native_ocr_stub 回退，属既有双轨契约。
5. accounts 包测试夹具无 socketPreheat 双保险（manager_test.go:24 注释自认），8 轮全绿实证无残余——若未来再出冷启动 flake 第一候选即补 socketPreheat。
6. 管理员 `DELETE /api/admin/accounts` 与 `DELETE /api/admin/codes`（router.go:137/:153）为兼容标准 REST 客户端刻意去掉 requireJSONBody 门（注释明示）——副产品：管理员 token 泄露场景下跨站表单无法伪造这些 DELETE（无 body 的表单 POST 不匹配 DELETE 方法），CSRF 风险面仍闭合。维持观察。
7. **warnedNoTargets 无锁写点（:1371-1372）**：位于 s.mu.Unlock() 之后、submitAll 的 `len(chains)==0` 分支内，宿主唯一（全文件仅此一处读写 grep 实证）、单写者单读者连续执行，语义自洽非漂移。消除它需在锁内整包核算"是否全账号无目标"，成本大于当前收益——首例并发写 Race 报告出现前维持观察。

---

## 结尾建议

**APPROVE**

身份防线矩阵第六十八轮闭合成立，sameClientFor 7 调用点终局逐一追写零漂移，maybeRelogin 双侧完整（决策侧 :1208 存在性复核 + 写回侧 :1254 复核 + :1265 二次 ClientFor 重取当前注册表 Token 落库）、手动五路 accountExists（:255/:305/:397/:497-512/:573）、写点换类 5 类 + warnedNoTargets 唯一性、*Locked 写函数族 13 个双向射证全部实测确认；OBSERVE-117-01 知识位第三十六轮、B110-01 审计链第四十三轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（连接活性自愈族 httpDo 判定 / 登录闸门族 B42-01 双侧收口）无漂移。CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。