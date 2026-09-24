# R154 后端只读审查 Findings —— 身份防线矩阵第六十九轮闭合

> 审查基线：`a72df70`（R153 归档，身份防线矩阵第六十八轮闭合）。模式：绝对只读，唯一写文件为本报告。
> 时间：2026-09-24。实测工具：`go build` / `go vet` / `go test -race -count=1`（MinGW gcc，`export PATH=/d/mingw64/bin:$PATH`）+ 走读追写终局。

## 结论前置（分级）

**CRITICAL 0 / HIGH 0 / MEDIUM 0 / LOW 0（无新增缺陷）**

身份防线矩阵第六十九轮闭合成立、零漂移。sameClientFor 定义（scheduler.go:204）与 7 调用点（:850/:1489/:1521/:1551/:1571/:1600/:1635）逐一定位并追写终局，六分支 + 实时复核 ErrUnauthorized 分支全族闭合；maybeRelogin 双侧（决策侧 :1208 / 写回侧 :1254 / 二次 ClientFor 重取 Token 落库 :1265-1273）完整；手动五路 accountExists（:255/:305/:397/:573/:497-512）判据同源；写点换类 5 类 + 无锁写点 warnedNoTargets 唯一性射证；*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证成立。OBSERVE-117-01 知识位第三十七轮、B110-01 审计链第四十四轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（凭据 AES-256-GCM 加密链终局 / 重启恢复 RestoreTargets 语义族）无漂移。

---

## 验证表（实测时间与数据）

| 验证项 | 结果 | 实测证据/数据 |
|--------|------|---------------|
| `go build ./...` | ✅ | EXIT=0，无输出 |
| `go vet ./...` | ✅ | EXIT=0，无输出 |
| 定向 race 四包（强制非缓存） | ✅ 全绿 | `-race -count=1 -v`：zhidao 2.013s / accounts 1.398s / scheduler 15.041s / api 13.476s |
| 追加 race 六包（纵深加固） | ✅ 全绿 | session 1.470s / store 13.869s / secure 1.645s / db 2.301s / runtime 1.613s / config 1.693s |
| 身份防线族（scheduler 定向，全量 run 内含） | ✅ 全绿 | TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}（各 0.01s）+ TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin（0.51s）+ TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight（0.01s）+ TestMaybeReloginDeletedAccountSkipsMaps（0.00s）+ TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime（0.15s）+ TestDeletedAccountReloginSuccessDropsState / TestDeletedAccountInFlightDropsSuccess / TestRealtimeRecheckDeletedAccountDropsLog / TestUnauthorizedBranchDeletedAccountSkipsState 等 |
| 回归锚双测 | ✅ 全绿 | TestWindowOpenSubmitsWithoutProbeReset（scheduler 1.02s PASS）+ TestAdminStatsWindowOpenedUsesScheduler（api 0.42s PASS） |
| 既有守卫契约 | ✅ 绿 | TestSubmitSuspendedWhenOpenTimeCleared（scheduler 1.26s PASS）|
| 轮次标签扫描 | ✅ 零命中 | 产品代码仅 `internal/session/store.go:117` 引用历史文档路径 `docs/review-round13.md`（文档路径，合规）；测试文件 3 处"第 3 轮"行为叙述（scheduler_test.go:1659/:1662/:2518）合规 |
| 零吞错穷举 | ✅ 零命中 | 四处 `_ =`/`_, _ =`（scheduler :307/:1075、handler :387/:467）逐一确认非落库（见第 3 节） |
| 工作区零漂移 | ✅ | `git diff a72df70 -- backend/` 全空；未跟踪仅 `?? archive/review-rounds/round154-frontend-findings.md`（并行前端代理产物） |
| 时间基 LOW-132/133 回首 | ✅ | 见专项第 5 节 |

---

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十九轮闭合 —— ✅ 在位（sameClientFor 7 调用点终局逐一追写）

**sameClientFor 定义与坐标核对（相对基线零漂移）**

- 定义 scheduler.go:204-210：锁内 `ClientFor(acct)` 存在 + nil 判 + `clientIdentity(current) == clientIdentity(chainClient)`；clientIdentity :215-224 用 `reflect.ValueOf(x).Pointer()` 取接口动态值底层指针，nil→0、非 Ptr 动态类型→0。接口依赖 :118-135：Client 含 `Token() string`（重登后读取新 token 落库）。与契约逐字符一致。
- 实测 7 调用点：**:850（ProbeForAccount 回写段，M88-01）、:1489（失效分支）、:1521（成功分支）、:1551（风控退避分支）、:1571（窗口关闭分支）、:1600（实时复核三路入口）、:1635（实时复核确证满员分支）**——与 R153 坐标逐字符一致，零漂移。

**各调用点"写什么状态/落什么库行"终局逐一追写：**

| 调用点 | 位置 | 身份复核失败时放弃的写入 |
|--------|------|--------------------------|
| 探测回写 | :850 | 放弃 `acctData[acct]`/`acctDataAt[acct]`/`openTimeDetected[acct]` 识别槽写入（:855-864 整块短路；纯内存帧不落库；防新旧年级帧串写重建身份）。已删/身份变 → 返回 data+日志不写帧 |
| 失效分支 | :1489 | 先清 inflight（:1490 公共清位）→ 放弃 maybeRelogin 触发 + failed 状态 + AppendLog 失效日志（:1494-1508 整块短路）。身份前置保护 Manager 失败计数（reloginFail）不被新身份污染 |
| 成功分支 | :1521 | 放弃 `done[acct][id]=true` + setStateLocked success + AppendLog(success) + SaveSuccess 库行（:1526-1540 整块短路，重启 RestoreDone 无假成功） |
| 风控退避 | :1551 | 放弃 `markRateLimitedLocked`（30s 退避）+ failed 状态 + AppendLog（:1555-1561 整块短路，黄金期不被假退避静默跳过） |
| 窗口关闭 | :1571 | 放弃 `markFullLocked`（永久满员退避）+ 状态 + AppendLog（:1575 短路） |
| 实时复核入口 | :1600 | 拦截其下三路（ErrUnauthorized→maybeRelogin/失败写库 :1608-1619；确证满员→markFullLocked :1639；普通失败→failed+AppendLog :1660-1665）整块写面 |
| 实时复核确证满员 | :1635 | 在 doneHas 让位（:1627 绝不覆盖手动胜利状态）之后加码身份复核，放弃 markFullLocked（:1639 短路） |

六分支对称结论：成功/失效/风控/窗口关闭/确证满员/实时复核 ErrUnauthorized（:1608）全族闭合叠合，加上 submitAll 遍历判 ClientFor（:1355）、spawnChain 入口判 ClientFor（:1388）、goroutine 取 client 二次判（:1405-1409）、inflight 双门口（:1464/:1471）、doneHas/refusedHas/fullHas 门（:1436/:1447/:1452）——整链无裸露写点。TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + RealtimeUnauthorizedDropsRelogin + SuccessDropsInflight 七测全绿（各分支独立红绿，waitChainExit 契约规避 inflight nil-map 假绿）。

**maybeRelogin 双侧确认（:1198-1299）：**

- 决策侧 :1208：入口 `reloginMu.Lock()` + `s.mu.Lock()`（:1199/:1201）后先 `ClientFor(acct)` 存在性复核，!ok 即整体 return——绝不写 tokenValid/reloginFail/reloginAt/relogging 任何 map。TestMaybeReloginDeletedAccountSkipsMaps 四 map 全空实证。
- 写回侧 :1254：goroutine 内成功分支前先 ClientFor 复核 → :1259 `err==nil && relogged` 成功分支 → :1265 **二次 ClientFor 重取当前注册表 client** → `client.Token()` :1266 取新 token → :1269 UpdateIDToken 落库（:1268 nil store 判）。同名重建场景：第一次跟第二次复核之间注册表被顶替时，第二次重取即拿到新身份客户端，落库写入方永远是"当前最新注册的客户端身份"（OBSERVE-117-01）。失败侧 :1292-1297 只清 relogging、reloginFail 保留递增计数（指数退避表不振荡）。
- 重登族 map 写点全量盘点：tokenValid/reloginFail/reloginAt/relogging 写入仅发生在 maybeRelogin 决策段（:1231/:1236/:1241）与成功分支（:1260/:1261/:1262）、清理在 PurgeAccount（:507-510）与 MarkTokenValid（:1316-1318）与失败清 relogging（:1247）与退避窗口过 delete reloginAt（:1226）——全部持 reloginMu+s.mu（MarkTokenValid 同序对齐 :1312-1315），零锁外写。

**手动五路 accountExists 确认（handler.go）：**

- 课程读 :255、手动报名 :305、手动退选 :397、状态读 :573、目标写 :497-512（内联 LoadCredentials 循环判 found，注释明示"仅用 accounts 表会误伤 authenticateDirect 直连"的判据同源变体）——五路全部凭据表判据。accountExists 定义 :1109-1120（LoadCredentials 失败返回 false），穿透门 :1103 仅管理员会话（Sessions.IsAdminToken，session/store.go:191-203）。五路对应测试全绿：TestAdminElectivesUnknownAccountRejects / TestAdminElectiveSelectUnknownAccountRejects / TestAdminStateUnknownAccountRejects / TestSetTargetsUnknownAccountDoesNotFabricate。

**写点换类 5 类 + 无锁写点唯一性：**

- lastSubmit（提交节流）：submitAll :1346 锁内 `s.nowAlignedLocked()` 写，tick :1029 锁内读。
- lastSyncStart（时钟同步发起时刻）：:356 锁内置位、:393 锁内读（成功推进 lastSyncTime）、:405 锁内复位。
- syncing（时钟单飞标记）：:355 锁内置位、:369 goroutine 锁内回写复位、:404 无客户端路径锁内复位。
- lastProbe（全校探测节流闸门）：probe :1089（失败计入）/ :1114（成功计入）、ProbeNow :964（管理员强制刷新）、Start 主循环 :679（重登成功回传统一处清零补探测，注释明确"避免 goroutine 并发写 s.lastProbe 竞态"）——全部锁内写，tick :976 锁内读。ProbeForAccount 刻意不写（:868-872 注释：只归 probe/ProbeNow 管理，防止管理员穿透探测旁路全校节流）。
- state.EmptyProbeRuns（幽灵窗口量变）：probe :1156（+1）/ :1158（归零）锁内，windowClosedLocked :935 锁内读。
- 无锁写点 warnedNoTargets :1371-1372：唯一宿主 = submitAll（判定在 :1368 `len(chains)==0` 之后、:1375 return 之前）。调用方唯一（tick :1035），单写者单读者同一链式 if 连续执行，初始 zero-value false，无目标时必走到。**注意：此写点确实位于 s.mu.Unlock()（:1367）之后无锁执行**——但宿主唯一性成立（全文件仅此一处读写，grep 实证），是该字段语义下的合规无锁单写者，非漂移。首例并发写 Race 报告出现前维持观察（延续 R153 观察项 7）。

***Locked 写函数族 13 个 + 外部写函数首行取锁双向射证：**

- 定义 13 个：nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846。逐一追调用上下文全在锁段内（spawnChain 锁段 / tick 锁段 / StateForAccount / WindowClosed / probe / purge / restore / TryAcquireSubmit / MarkDone / RemoveDone / RemoveFull），无锁外裸调。
- 外部写函数首行取锁核查：SetTargetsForAccount :455、PurgeAccount :496、RestoreTargets :526、RestoreDone :608、RestoreRefused :627、ProbeForAccount 回写段 :849（锁 + sameClientFor 前置复核锁内）、ProbeNow :963、ElectivesSnapshotFor :762、TryAcquireSubmit :1897、MarkDone :1923、RemoveDone :1990、RemoveFull :2039——全部首行取锁，双向射证成立。

### 2. OBSERVE-117-01 知识位第三十七轮 —— ✅ 在位

见第 1 节 maybeRelogin 写回侧。关键证据链：:1246 goroutine 内 `s.mu.Lock()` → :1247 清 relogging → :1254 `ClientFor(acct)` 存在性复核（已删则整体放弃，只清标记）→ :1259 成功分支 → :1265 二次 `ClientFor(acct)` 重取当前注册表 client → :1266 `client.Token()` 取新 token → :1269 UpdateIDToken 落库。同名重建场景：第一次跟第二次复核之间注册表被顶替时，第二次重取即拿到新身份客户端，落库写入方永远是"当前最新注册的客户端身份"——与 R153 逐字符一致，零漂移。TestReloginSuccessWithNilStoreNoPanic、TestDeletedAccountReloginSuccessDropsState、TestDeletedAccountRebuiltSameNameChainDropsRelogin 全绿。

### 3. B110-01 审计链第四十四轮 —— ✅ 零漂移

- 手动 6 失败位 AppendLog：handler.go :362（报名失效）、:374（报名 read）、:381（报名业务失败）、:443（退选失效）、:452（退选 read）、:459（退选业务失败）——全部 `if ... != nil { log.Printf }` 零吞错。
- 成功行：报名成功走 MarkDone 内 AppendLog（scheduler.go:1976）、退选成功走 RemoveDone 内 AppendLog（:2029）、目标保存 :558、登录 :237、管理员 login :136、删除账号 :1055、登出 :616——全部带错误处理。
- 自动链失败族：spawnChain 六分支 AppendLog 全部 `if err := ...; err != nil { log.Printf }`（:1504/:1532/:1558/:1614/:1662/:1770）+ markFullLocked（:1770）+ MarkDone 内 SaveSuccess/AppendLog（:1973/:1976）+ RemoveDone 内 DeleteSuccess/SaveRefused/AppendLog（:2020/:2026/:2029）+ SetTargetsForAccount 内 SetTargetsForAccount/DeleteRefused（:476/:484）+ 重登成功 UpdateIDToken（:1269）。
- 零吞错穷举：`_ =`/`_, _ =`/直接赋值忽略形态全量扫描——scheduler.go `_ =`（:307 Prewarm 异步 goroutine 丢返回值）、`_, _ =`（:1075 ProbeForAccount 探测 goroutine 丢返回值），handler.go `_ =`（:387 MarkDone、:467 RemoveDone）——四处全部**非落库**：:307 预热结果不上抛、:1075 探测错误已在 ProbeForAccount 内部处理、:387/:467 手动成功路径 error 恒 nil（业务错误全部前置 return）。所有 `store.Save*/Delete*/AppendLog/UpdateIDToken/SetTargetsForAccount` 落库点 grep 穷举零"忽略错误"形态，全部 `if err != nil { log.Printf }`。
- 网络层 token 脱敏延续抽查：zhidao/client.go doRequest 统一 `sanitizeError`（:450 剥 url.Error URL 文本、:566-593 Unwrap 下沉保留 errors.As/Is 判型、`ue.Op==""` 防御），scheduler.go:1283 重登成功日志走 maskedToken（:1334，>8 位仅前 8 位、≤8 位 "***"）。TestMaskedTokenBoundary / TestSanitizeError 系列绿。交叉验证：sanitizerErr.Unwrap 下沉后 isConnErrRetryable/IsReadErr 判型穿透不变（isreaderr_test / sanitize_test 覆盖 dial/write/read-rst/fin/shortread/timeout/business 七形态）。

### 4. O105-01 抖动基线 —— ✅ 实测绿

- socketPreheat/readyProbe 夹具在位：zhidao/client_test.go:25/:88、captcha_test.go:17/:29/:52/:79、sanitize_test.go:97、api/handler_test.go:179/:185、accounts/manager_test.go:26（readyProbe，注释自认无 socketPreheat 双保险、8 轮全绿实证无残余，延续观察项 5）。
- 定向 race 实测（`/d/mingw64/bin` gcc 路径下）：zhidao/accounts/scheduler/api 四包 `-race -count=1` 全绿（2.013/1.398/15.041/13.476s），追加 session/store/secure/db/runtime/config 六包全绿（纵深加固）。
- 定向身份防线族（含在 scheduler 全量 run 内）+ z回归锚 TestWindowOpenSubmitsWithoutProbeReset（1.02s）+ TestAdminStatsWindowOpenedUsesScheduler（0.42s）+ TestSubmitSuspendedWhenOpenTimeCleared（1.26s）全部绿。

### 5. LOW-132 / LOW-133 回首核 —— ✅ 通过

- `time.Since()/time.Now()` 产品代码全量扫：残余仅两族——① `reloginAt` 写（:1231/:1261 本地钟）读（:1220/:1227 `time.Since(t)` 本地钟）**同基自洽**（30s 重登节流语义不依赖服务端对齐钟，同一重登流程内部时间差）；② accounts/manager.go `gateWindow` 三处（gateWait :53/:54/:61、gateTryAcquire :226/:227、GatePump :71/:74）本地钟写读**同基自洽**（分钟窗口闸门语义为本地时间差）。其余非节流语义独立点（zhidao SyncServerTime RTT 测量 :113/:135/:137、captcha ?v 随机种子 :328、uniqueDeviceID 时间戳 :355、session 会话/票据 TTL store.go、api loginLimiter handler.go:1176）均为各自子系统内自洽，与调度器节流时间基无关。探测族时间戳（lastProbe/lastSubmit/lastSyncStart/lastSyncFailAt/lastSyncTime/lastPrewarm/EmptyProbeRuns 入账侧/rateLimited 截止）全部对齐钟写、对齐钟读——与 R134-R153 各轮裁决一致，零新增时间基孤岛。
- `git diff a72df70 -- backend/` 全空，白线零漂移。

### 6. 新契约角度纵深（自选 ×2，时间盒内完成）

**角度 A：凭据 AES-256-GCM 加密链终局（选择理由：credentials 表 password_enc + id_token 是库内最敏感数据，加密链（secure → store → accounts.Restore/LoginByPassword）与身份防线（重启恢复不复活假凭据）相交于"凭据落库方必须取当前真相"，且与重登成功落新 token 的 OBSERVE-117-01 位点共享同一数据面）**

- `secure` 包完整链：LoadOrCreateKey（secure/crypto.go:15-45，env 优先 + 64 位 hex 校验 / .master_key 文件 32 字节校验，损坏/截断/空文件一律显式报错拒绝带伤启动——此前不校验文件、新部署 WriteFile 随机覆盖损坏文件会永久损坏数据库）；Encrypt :48-63（AES-256-GCM + 随机 nonce 前置 + hex 输出）；Decrypt :66-87（hex 解码 + nonce 分离 + GCM Open + 密文长度校验）。TestEncryptDecryptRoundTrip / TestEncryptUniqueNonce / TestDecryptTamper / TestLoadOrCreateKey / TestLoadOrCreateKeyRejectsTruncatedFile 全绿。
- 存储面：SaveCredential upsert（store.go:29-35，password_enc + id_token 同锁更新，`ON CONFLICT(account) DO UPDATE`）；UpdateIDToken 只刷 token（:56-59）——重登成功落库与登录落库两条路径共用 credentials 表，无双表分叉。
- 解密面：main.go:61-65 Restore 时 decrypt（解密失败留痕"自动重登将无保存账密"，accounts/manager.go:300-307）；vision_key 严格 `enc:` 前缀（main.go:88-98 拒绝未加密明文 + 解密失败忽略留痕）。比 R151 纵深进一步追加验证：password_enc 与 id_token 字段互相独立落库、重登只刷 id_token 不触碰 password_enc（UpdateIDToken 单列 UPDATE 实证），解密失败在主密钥不可变语义下只影响该账号自动重登、不阻断内存登录（LoginByPassword 加密失败留痕 TestLoginByPasswordEncryptFailLogs 绿）。
- 结论：加密链五层（密钥加载 → 加密 → 存储 → 读取 → 解密）逐点有测试锚定，与 R151 逐字符一致，零漂移。

**角度 B：重启恢复 RestoreTargets 语义族（选择理由：恢复顺序契约（RestoreDone → 逐账号 RestoreTargets → LoadRefused+RestoreRefused）是"用户手动退选意图不被重启抵消"的根，且 RestoreTargets vs SetTargetsForAccount 的唯一区别（不清 refused）在内存 + 库行双层的一致性直接决定恢复正确性）**

- 顺序契约：main.go:117-141 确认 RestoreDone（:120）→ 逐账号 LoadTargetsForAccount + RestoreTargets（:122-133）→ LoadRefused + RestoreRefused（:137-140）。注释明示"RestoreTargets（不清 refused）——此前用 SetTargetsForAccount 会 delete refused + 删库行，重启后手动退选记录全丢、自动引擎重新抢回"。
- 双函数差异确认：RestoreTargets :526-533（不清 refused、enrichTargetPubMetaLocked 兜底补全 publish 元数据）；SetTargetsForAccount :454-489（清 refused 内存 + DeleteRefused 库行，且定案不清 done/full/rateLimited/inflight）。
- 迁移面交叉验证：Targets 表 publish_name/begin_date 纯增量迁移（db.go:43-57 migrateAddPublishMeta 逐列 columnExists 缺才 ALTER）、refuseLegacy 缺列清单与迁移列对应剔除（:80 清单剩 priority/allow_swap/account 三列，publish_name/begin_date 不在此列）、TestMigrateAddsPublishMetaColumns 绿、TestRefuseLegacyDB 绿。
- 结论：恢复顺序族与数据库迁移族构成"重启后用户意图完整保留"的双层证据，实测绿。

---

## 维持观察项（延续既有记录 + 本轮）

1. `IsClassFull` 实时复核受 CountEntry.MaxCount 未实证下发限制——恒 false 兜底路径，真满员主判据为快照 max_count，属既有契约非缺陷。
2. `RemoveFull`（:2038）目前仅有定义无产品调用方（外部手动/snapshot 更新语义保留），预留接口，未来消费前保持观察。
3. `syncFailedWindow` 写而不读（:182/:377），判据用 syncFailStreak——留档字段语义自洽。
4. 验证码识别引擎 ddddocr 本地推理在 CGO=0 交叉编译形态依赖 native_ocr_stub 回退，属既有双轨契约。
5. accounts 包测试夹具无 socketPreheat 双保险（manager_test.go:24 注释自认），8 轮全绿实证无残余——若未来再出冷启动 flake 第一候选即补 socketPreheat。
6. 管理员 `DELETE /api/admin/accounts` 与 `DELETE /api/admin/codes`（router.go:153/:137）为兼容标准 REST 客户端刻意去掉 requireJSONBody 门（注释明示）——副产品：管理员 token 泄露场景下跨站表单无法伪造这些 DELETE（无 body 的表单 POST 不匹配 DELETE 方法），CSRF 风险面仍闭合。维持观察。
7. **warnedNoTargets 无锁写点（:1371-1372）**：位于 s.mu.Unlock() 之后、submitAll 的 `len(chains)==0` 分支内，宿主唯一（全文件仅此一处读写 grep 实证）、单写者单读者连续执行，语义自洽非漂移。首例并发写 Race 报告出现前维持观察。

---

## 结尾建议

**APPROVE**

身份防线矩阵第六十九轮闭合成立，sameClientFor 7 调用点终局逐一追写零漂移（六分支 + 实时复核 ErrUnauthorized 全族对称），maybeRelogin 双侧完整（决策侧 :1208 存在性复核 + 写回侧 :1254 复核 + :1265 二次 ClientFor 重取当前注册表 Token 落库）、手动五路 accountExists（:255/:305/:397/:573/:497-512）、写点换类 5 类 + warnedNoTargets 唯一性、*Locked 写函数族 13 个双向射证全部实测确认；OBSERVE-117-01 知识位第三十七轮、B110-01 审计链第四十四轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（凭据 AES-256-GCM 加密链终局 / 重启恢复 RestoreTargets 语义族）无漂移。CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。