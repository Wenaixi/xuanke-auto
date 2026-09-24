# round142 后端只读审查 findings（身份防线矩阵第五十七轮复核）

- 审查基点：`2ca7e0c`（R141 归档，身份防线矩阵第五十六轮闭合）
- 模式：绝对只读（唯一写文件 = 本报告）
- 实测窗口：本机 Windows 11 + mingw64 gcc（`/d/mingw64/bin`），`go test -race` 定向六包
- 结论前置：**无 CRITICAL / 无 HIGH / 无 MEDIUM / 无 LOW 级缺陷，建议 APPROVE**

## 验证表（实测时间与数据）

| 项目 | 命令 | 实测结果 |
|------|------|---------|
| 构建 | `go build ./...` | exit 0 |
| 静态检查 | `go vet ./...` | exit 0 |
| race zhidao 包 | `go test -race -count=1 ./internal/zhidao/` | ok，2.929s |
| race accounts 包 | `go test -race -count=1 ./internal/accounts/` | ok，3.005s |
| race api 包 | `go test -race -count=1 ./internal/api/` | ok，15.981s |
| race scheduler 全包 | `go test -race -count=1 ./internal/scheduler/` | ok，15.319s |
| race db + store 包 | `go test -race -count=1 ./internal/db/ ./internal/store/` | ok，2.480s / 20.666s |
| 六包非 race 全量回归 | `go test -count=1 ./internal/{scheduler,api,zhidao,accounts,store,db}/` | 全 ok（14.165s / 9.978s / 1.689s / 0.873s / 2.480s / 1.427s） |
| 回归锚三测（race） | `-run "TestAdminStatsWindowOpenedUsesScheduler\|TestWindowOpenSubmitsWithoutProbeReset\|TestProbeDeletedThenRebuiltSameNameDropsSnapshot\|TestMigrateAddsPublishMetaColumns"` | api ok 2.007s / scheduler ok 2.507s / db ok 1.588s |
| 身份防线族（race） | `-run "TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}\|TestMaybeReloginDeletedAccountSkipsMaps\|TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin"` | scheduler ok，2.495s |
| 契约轮次标签扫描 | grep `第 .*轮\|round\|R1[0-9][0-9]\|R[0-9][0-9]-` 于 internal/ 产品代码 | 零命中（仅 schedule 名 `Round` 无关、session 包历史文档引用） |
| 工作区漂移 | `git status --porcelain` | 仅 `archive/review-rounds/round142-frontend-findings.md`（前端代理并行产出，非本代理写入）；后端零修改 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第五十七轮闭合 — ✅ 在位（零漂移）

**sameClientFor 定义与 7 处调用点逐一核对**（scheduler.go）：

- 定义 :204-210：`ClientFor(acct)` 存在性 + `clientIdentity` 反射指针身份比对，nil/不存在均非同一身份。
- :850 `ProbeForAccount` 回写段：网络往返后持锁复核身份，非同一身份整体放弃 acctData/acctDataAt/openTimeDetected 回写（M88-01，probe_identity_test.go 有已删/同名重建双回归测试）。
- :1489 失效分支（ErrUnauthorized）：先身份复核再 maybeRelogin / 清 inflight / setStateLocked / AppendLog。
- :1521 成功分支：身份复核后才写 done + SaveSuccess + AppendLog（库行终局 success 行）。
- :1551 风控退避分支：身份复核后才 markRateLimitedLocked + 状态 + AppendLog。
- :1571 窗口关闭分支：身份复核后才 markFullLocked。
- :1600 实时复核回锁后的三路入口统一复核（在 doneHas 快照判满之前执行，且 :1635 确证满员分支二次复核）。
- :1635 确证满员分支：身份复核后才 markFullLocked。

每条调用点"写什么"终局追踪完毕：非同一身份一律静默 return，不写内存态、不落库行、不记日志；:1499 失效分支调 maybeRelogin 前已确认同一身份（重登决策正确归账），:1608 实时复核 ErrUnauthorized 分支对称。

**maybeRelogin 双侧**（:1198-1299）：
- 决策侧 :1208 `ClientFor` 存在性复核，已删账号不发起重登、不写任何 map（含 tokenValid/reloginFail/reloginAt/relogging 四组写点全部在其后方）；锁序 reloginMu→s.mu 对齐。
- 写回侧 :1254 二次 `ClientFor` 存在性复核（成功分支整体放弃内存写与库写）；:1265-1273 落库前再取 `ClientFor` 的当前 client.Token()（OBSERVE-117-01 第二十五轮，见下），新 token 以脱敏 maskedToken 日志记录。

**手动五路 accountExists**（handler.go）：
- :255 课程读；:305 手动报名；:397 手动退选；:497-512 目标写（内联循环）；:573 状态读。判据同源（`accountExists :1109` 用 LoadCredentials 凭据表），整体拒绝"账号不存在"，无全局帧假成功路径。

**写点换类 5 类**：
- lastSubmit：仅 :1346 `submitAll` 开头（s.mu 内 nowAlignedLocked 写入），调用点唯一（:1035 tick），唯一宿主射证。
- lastSyncStart / syncing：:355-356（maybeSyncClock 决策段，s.mu 内）+ :369/:404-405（goroutine 回写，s.mu 内），全部持锁。
- lastProbe：:679（Start 主循环 reloginResults 通道，s.mu 内）、:964（ProbeNow）、:1089/:1114（probe，s.mu 内），全部持锁。
- EmptyProbeRuns：:1156-/1158（probe，s.mu 内且判据"已过开窗点 10s 裕量"）。
- 无锁写点 warnedNoTargets：:1371-1372 由 `s.tick():1035 → s.submitAll():1368-1374` 唯一路径触发，宿主唯一性射证成立（对 goroutine 串行语义无并发写，读侧仅 log 判断，无竞态窗口）。

**\*Locked 写函数族 13 个 + 外部写函数首行取锁双向射证**：
- Locked 族清单：`nowAlignedLocked/openTimeForLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/rebuildCoursesLocked/tokenValidForLocked/windowClosedLocked/isRateLimitedLocked/markRateLimitedLocked/markFullLocked/releaseFullIfFreedLocked/statusIndexLocked/setStateLocked`（13 个）。逐一确认全部在 s.mu 持有下被调用（内部不再取锁、无嵌套死锁）。
- 外部写函数 `MarkDone/RemoveDone/RemoveFull/TryAcquireSubmit/SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkTokenValid/TokenValidFor` 首行或声明处均 `s.mu.Lock()/defer Unlock()`（MarkTokenValid/TokenValidFor 额外先取 reloginMu），取锁于闭函数首行，双向射证无缺口。

### 2. OBSERVE-117-01 知识位第二十五轮 — ✅ 在位

写回侧 :1254 先 ClientFor 存在性复核 → :1265 再次 `ClientFor` 重取当前注册表 client → `client.Token()` 落库（:1266-1274，UpdateIDToken）。同名重建场景：决策侧/写回侧/落库前 Token 重取三处全部基于"当前注册表"而非发起时客户端快照，绝不串旧身份。对应测试 `TestDeletedAccountReloginSuccessDropsState`（:933）在位且 race 全绿。

### 3. B110-01 审计链第三十二轮 — ✅ 在位（零漂移）

- 手动 6 失败位 AppendLog：:362（报名失效）、:374（报名 read）、:381（报名普通失败）、:443（退选失效）、:452（退选 read）、:459（退选普通失败）——全部 `if err := ... ; err != nil { log }` 绝不 `_ =`。
- 成功审计行：:388 MarkDone 内 AppendLog success；:558 set_targets；:1055 delete_account；:616 logout；:136/:237 login——全链路。
- 自动链失败族：spawnChain 六分支每分支失败均 AppendLog（:1504/:1532/:1558/:1614/:1662 + markFullLocked 内 :1770）。
- 零吞错穷举：全包 `_ =`/`_, _ =`/直接赋值忽略形态仅 6 处，逐一为合规豁免——
  scheduler.go:307 `_ = pw.Prewarm()`（goroutine 内预热广播，失败无消费方）；scheduler.go:1075 `_, _ = s.ProbeForAccount(acct)`（probe 每账号探测结果仅作快照刷新，错误在下游 probe 主体统一处理）；handler.go:387/467 `_ = MarkDone/RemoveDone`（内部已记录错误日志，返回 nil 恒）；client.go:107/124 `_, _ = io.Copy(io.Discard, resp.Body)`（响应体排空，读失败无审计价值）。
- 网络层 token 脱敏延续抽查：zhidao/client.go :563-593 sanitizeError/sanitizerErr（改?完整 URL、Unwrap 下沉判型穿透）、scheduler.go :1283 maskedToken（新 token 只显前 8 位）。doRequest :450 统一走 sanitizeError。

### 4. O105-01 抖动基线 — ✅ 在位（实测绿）

- socketPreheat（zhidao/client_test.go:25）+ TestMain 包级预热（:40）；api 包 handler_test.go:53-67 套接字预创建 + readyProbe（:179-185）；accounts 包 readyProbe（manager_test.go:26，三处调用）；sanitize_test/captcha_test 均有 socketPreheat——夹具全部在位。
- 定向 race 实测：zhidao / accounts / api / scheduler 四包全绿（见验证表），远超"至少四包"要求（额外 db + store 也绿）。
- 回归锚 `TestWindowOpenSubmitsWithoutProbeReset`（:1277）+ `TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go:1011，断言 stats.window_closed 与 `d.Sched.WindowClosed()` 同源）均绿。
- gcc 环境：`/d/mingw64/bin/gcc.exe` 在位，`go env CGO_ENABLED` 默认 0，race 构建经显式 PATH 注入可用；Windows 原生内嵌 ddddocr 的 CGO=1 构建不受本审查影响（测试路径全 CGO=0）。

### 5. LOW-132/133 回首核 — ✅ 通过

- `git show` 白线核对：本轮（2ca7e0c 相对 R140 基线）后端增量仅 R141 文档变更，无代码文件修改；工作区零漂移确认。
- time.Since/time.Now() 全量扫（剔除注释与测试文件）残余写点穷举：
  - scheduler：:269/:274 `nowAligned()`（time.Now()+offset，对齐钟基自洽）；:1220/:1227/:1231/:1261 `reloginAt` 读写同基 relogin 节流自洽；write 侧 :1231/:1261 与 read 侧 :1220/:1227 均为本地钟，写读同基。
  - accounts：manager.go :53/:71/:74/:226/:227 全部为 `gateWindow` 节奏（gateWait/gateTryAcquire/GatePump 三处写读同基）。
  - zhidao/client.go :113/:135/:137 SyncServerTime 中点估算（刻意独立时间基）；:328/:355 captcha/uniqueId（与调度无关的请求元数据）。
  - handler.go :1176 loginLimiter（独立桶语义）。
  - 残余仅 reloginAt 与 gateWindow 两族，均为"写读同基自洽"，无新混用孤岛；rateLimited 族已收敛至 nowAlignedLocked（:1714 写入 / isRateLimitedLocked 读侧）。

### 6. 新契约角度纵深（自选 ×2）

**角度 A：凭据 AES-256-GCM 加密链（完整链路走查）**
- 主密钥：main.go:50 `secure.LoadOrCreateKey(cfg.DBPath)`——XUANKE_MASTER_KEY 环境变量（64 hex 校验 32 字节）或 DB 旁 `.master_key`（32 字节严格校验，损坏拒绝启动）；config.go loadDotEnv :89-97 语义"仅回填空值 # 不截断 + 真实环境变量恒优先"，主密钥回填无竞态。
- 加密器：secure/crypto.go Encrypt（随机 nonce 前置 + hex）/Decrypt，AES-256-GCM；main.go:54-55 注入闭包。
- 密码落库：accounts/manager.go LoginByPassword :277-288 `encrypt(password)` 成功加密落库，失败留痕"凭据未落库"；Restore :295-309 解密失败留痕（主密钥不可变检测）。
- vision_key 同强度加密：handler.go :62-71 `secureEncrypt`（enc: 前缀，未注入 Encrypt 拒绝存储）；:833 落库；main.go :87-98 读回——`enc:` 前缀强制，旧明文拒绝加载。
- 决策锚 17 零吞错延伸抽查：LoginByPassword 加密失败 / Restore 解密失败均 log 留痕，无 `_ =`。
- 裁决：加密链全链路闭合，无明文落盘点，主密钥损坏/未注入路径均有显式拒绝或留痕。

**角度 B：重启恢复 RestoreTargets 语义 + 删账号 memory-first 四序**
- 恢复序（main.go :117-141）：RestoreDone → 逐账号 RestoreTargets → LoadRefused + RestoreRefused，与 scheduler.go :521-533 注释契约逐字一致。RestoreTargets 不清 refused（内存+库），RestoreRefused 在目标恢复后注入不被覆盖。restore 语义测试位：TestRestoreDoneSkipsResubmit（:567）、refused 族。
- 删账号四序（handler.go :1040-1054）：`Accounts.Remove` → `Sched.PurgeAccount` → `Store.DeleteAccount` → `Sessions.RevokeAccount`——客户端先消失，在飞链复核立即失败静默放弃；PurgeAccount 全量清（含 openTimeDetected/tokenValid/relogin 族/acctData，scheduler.go :495-518）。半删态（库删失败）由重启 Restore 自愈，绝不假成功。
- PurgeAccount 与 SetTargetsForAccount 边界：重设目标不清 done/full/rateLimited/inflight（:459-465 定案注释）与删账号全量清理严格分离，测试 TestPurgeAccount/TestSetTargetsPurgesStaleState 在位。
- 迁移规范复核：db.go Open 序 = 建表 → migrateAddPublishMeta → refuseLegacy；:77-80 清单已对应剔除已迁移列（publish_name/begin_date），TestMigrateAddsPublishMetaColumns 在旧库预置真实行验证迁移后数据保留。

## 维持观察项（无阻断）

1. `AccountsWithTargets()` 排序输出（:755 sort.Strings）是 api 层"核心账号"选择依赖，若未来账号名含不可比较序的 Unicode 需复核稳定性——当前全数字学号场景无影响。
2. session 包票据 TTL（5 分钟）与激活码消耗之间的重放窗口由 ConsumeTicket 单次语义覆盖，契约已落 docs；无新暴露面。
3. Prewarm goroutine 失败静默（`_ = pw.Prewarm()`）是刻意取舍（预热失败不阻塞任何路径），若未来要审计预热成功率可加通道留痕——非本轮缺陷。

## 建议

**APPROVE**。聚焦清单六项全部 ✅ 在位且实测绿；身份防线矩阵五十七轮闭合（sameClientFor 定义+8 调用点、maybeRelogin 双侧+二次 Token 重取、手动五路 accountExists、写点换类持锁/唯一性、Locked 族双向射证）；OBSERVE-117-01 / B110-01 / O105-01 / LOW-132/133 全部维持零漂移，且每项均有对应测试钉守（waitChainExit 等待契约杜绝假绿）。构建双绿、四包以上定向 race 全绿、回归锚钉守绿、工作区后端零漂移。无 CRITICAL/HIGH/MEDIUM/LOW 级发现。