# R156 后端只读审查 findings —— 身份防线矩阵第七十一轮

- 审查模式：绝对只读（唯一写文件为本报告）
- 工作树基线：`522354a`（上一里程碑归档）
- 实测时间：2026-09-24 16:20–16:35（本机时区 +08:00）
- Go 版本：go1.26.8 windows/amd64

## 结论前置（分级）

| 级别 | 数量 | 摘要 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| LOW | 0 | — |

**结论：APPROVE。** 身份防线矩阵第七十一轮闭合成立、零漂移。sameClientFor 定义与 7 调用点逐一追到"写什么状态/落什么库行"终局——六分支 + 探测回写全族闭合，网络往返后持锁写入前均为最后一道身份闸；maybeRelogin 决策侧/写回侧双侧完整；手动五路 accountExists 判据同源；写点换类 5 类全持锁 + 无锁写点 warnedNoTargets 宿主唯一性射证；*Locked 写函数族与外部写函数"首行取锁"双向射证。OBSERVE-117-01、B110-01 审计链、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（窗口状态三判据单源 open 快照复用 / 基础设施状态码家族整风核对）无漂移。`go build` / `go vet` 双绿，`-race` 全包绿。

## 验证表（实测记录）

| 验证项 | 命令 | 结果 |
|--------|------|------|
| 编译 | `go build ./...` | PASS（exit 0） |
| 静态检查 | `go vet ./...` | PASS（exit 0） |
| race·身份防线族十测 + 回归锚 | `go test -race -run 'TestDeletedAccountRebuiltSameNameChainDrops(Success\|Relogin\|RateLimitBackoff\|WindowClosedFull\|RealtimeRecheckFull)\|...RealtimeUnauthorizedDropsRelogin\|...SuccessDropsInflight\|TestProbeDeletedThenRebuiltSameNameDropsSnapshot\|TestProbeForAccountDropsWriteWhenRemoved\|TestProbeChainSameClientIdentity\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared' ./internal/scheduler/` | 12/12 PASS（4.192s） |
| race·scheduler 全量 | `go test -race -count=1 ./internal/scheduler/` | PASS（15.086s） |
| race·zhidao 全量（防缓存） | `go test -race -count=1 ./internal/zhidao/` | PASS（1.906s） |
| race·accounts 全量（防缓存） | `go test -race -count=1 ./internal/accounts/` | PASS（1.306s） |
| race·api 全量（防缓存） | `go test -race -count=1 ./internal/api/` | PASS（11.154s） |
| race·其余包（secure/db/store/config/session/runtime） | `go test -race -count=1 ./internal/secure/ ./internal/db/ ./internal/store/ ./internal/config/ ./internal/session/ ./internal/runtime/` | 6/6 PASS |
| race·全仓收尾 | `go test -race -count=1 ./...` | 全包 PASS |
| 回归锚·窗口开启提交 | `TestWindowOpenSubmitsWithoutProbeReset`（含于上面族测） | PASS |
| 回归锚·admin stats 同源 | `TestAdminStatsWindowOpenedUsesScheduler` 全量 + 独立复跑（1.910s） | PASS |
| 定向·api 身份/账号族 | 14 测（含 TestAdminDeleteAccountMemoryFirst / TestAccountOverrideRequiresAdminSession / TestLoginAdminNameCollisionStudentCredential 等） | 14/14 PASS |
| 定向·脱敏/判型族 | `TestSanitizeError*` 6 测 + `TestIsReadErrCoversAllForms` 1 测 + `TestDoRequestSanitizesDialError` | 8/8 PASS |
| 产品代码轮次标签扫描 | grep "第 [0-9一二三…] 轮\|R[0-9]+ 轮\|round [0-9]" main.go + internal/ 非测试文件 | 零命中 |
| 工作区漂移 | `git diff 522354a -- backend/` | 0 行（测试期间无任何产物变更，除本报告与前端并行报告两个 untracked 文件） |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第七十一轮闭合 —— ✅ 在位

**sameClientFor 定义（scheduler.go:204）+ clientIdentity（:215）**：逐字符确认。`reflect.ValueOf(c).Pointer()` 对接口动态类型为指针时返回底层指针值；nil 与非指针统一归 0（同一身份恒等、跨身份恒不等）；注释明确"仅判账号名存在挡不住同名重建"。7 调用点零漂移，逐一追终局：

| 行号 | 分支 | 身份复核失败时的终局 |
|------|------|----------------------|
| :850 | ProbeForAccount 探测回写 | 放弃写 openTimeDetected / acctData / acctDataAt——绝不覆盖重建账号识别槽与年级帧 |
| :1489 | spawnChain ErrUnauthorized | delete inflight 后 return——**绝不再触发 maybeRelogin**（:1494-1498 注释钉死"重登必须落在身份复核之后"），不写状态、不落日志 |
| :1521 | 报名成功 | return——不写 done / setStateLocked(success) / AppendLog / SaveSuccess，杜绝重建身份假成功（重启 RestoreDone 恢复假状态） |
| :1551 | 风控退避 isRateLimitError | return——不写 rateLimited / setStateLocked / AppendLog，王牌"假退避让黄金期被静默跳过"污染点闭合 |
| :1571 | 窗口关闭 isWindowClosedError | return——不写 markFullLocked，杜绝"已满员"永久退避写进重建身份 |
| :1600 | 实时复核结果块（三路归并入口） | return——整块（ErrUnauthorized→maybeRelogin/setState/AppendLog、确证满员→markFullLocked、未满员→setState/AppendLog）整体放弃 |
| :1635 | 实时复核确证满员子分支 | return——不写 markFullLocked（:1600 已拦全员，此处为满员单路二次拦） |

身份防线族十测全部独立红绿锚定：六条 `TestDeletedAccountRebuiltSameNameChainDrops{S/R/RateLimitBackoff/WindowClosedFull/RealtimeRecheckFull}` + `RealtimeUnauthorizedDropsRelogin`（实时复核分支同族补测）+ `SuccessDropsInflight`（成功分支 inflight 位无残留）+ 探测两条（`TestProbeDeletedThenRebuiltSameNameDropsSnapshot` / `TestProbeForAccountDropsWriteWhenRemoved` / `TestProbeChainSameClientIdentity`）。测试用 `waitChainExit`（scheduler_test.go:3185）等待 chains 活跃标记消失而绝不用 inflight 等待（PurgeAccount 后读 nil map 恒 false 假绿陷阱已在测试注释钉死）。

**maybeRelogin 双侧**：决策侧 :1208 锁内 `ClientFor(acct)` 存在性复核（探测定时三处 ProbeForAccount:823/ProbeNow:956/probe:1094 对 ErrUnauthorized 直调入口，删号与在飞探测同帧时先复核再写任何 map）；写回侧 :1254 成功分支先复核客户端仍存在才进成功块；:1265-1273 **二次 ClientFor 重取当前注册表 client.Token() 落库**（UpdateIDToken）——同名重建场景旧链即使过了 :1254（仅判存在）也拿不到新身份 token 写库，OBSERVE-117-01 语义完全成立。

**手动五路 accountExists**：handler.go :255（handleElectives）/ :305（handleElectiveSelect）/ :397（handleElectiveExit）/ :497-512（handleSetTargets 内联 LoadCredentials 逐账号比对，不走共享函数因需区分错误路径）/ :573（handleState）——五路判据同源（凭据表 = "确实登录过"的更强真理源），路由 :1109-1120 定义 `accountExists`，选课/退选/状态三路复用，目标写路径因返回错误文案不同用了相等逻辑的内联实现（:497-512 循环等价于 accountExists）。

**写点换类 5 类 + 无锁写点宿主唯一性**：
- `lastSubmit`（:1346 submitAll 锁内写）/ `lastSyncStart`（:356/:405 持锁）/ `syncing`（:355/:369/:404 持锁）/ `lastProbe`（:679 重登回补锁内 / :964 ProbeNow 锁内 / :1089/:1114 probe 锁内）/ `state.EmptyProbeRuns`（:1156/:1158 probe 锁内）——全部持 s.mu 写。
- `warnedNoTargets`：唯一写点 :1371-1372（位于 `s.mu.Unlock()` 之后、submitAll 的 `len(chains)==0` 分支内），grep 全文件仅此一处读写；宿主"链空分支"在 unlock 后与任何并发写者无共享，单写者单读者连续执行，语义自洽非漂移（维持观察项延续）。

**\*Locked 写函数族双向射证**：函数族 13 个——标记型 `markRateLimitedLocked`(:1710) / `setStateLocked`(:1846) / `markFullLocked`(:1760) / `releaseFullIfFreedLocked`(:1781) / 判据型 `isRateLimitedLocked`(:1691,含超期删除写) / `classFullInSnapshot`(:1721) / `openTimeForLocked`(:427) / `windowClosedLocked`(:918) / `tokenValidForLocked`(:739) / `statusIndexLocked`(:1835) / 重建型 `rebuildCoursesForAccountLocked`(:570) / `rebuildCoursesLocked`(:647) / 补全型 `enrichTargetPubMetaLocked`(:539)。逐一核对全部调用点均在持 s.mu 段内（tick/probe/spawnChain/StateForAccount 等锁内调用，行级追证无裸调）。外部写函数首行取锁射证：SetTargetsForAccount(:455) / PurgeAccount(:496) / RestoreTargets(:526) / RestoreDone(:608) / RestoreRefused(:627) / Start(:659) / MarkTokenValid(:1314,reloginMu→mu 双锁) / ProbeForAccount(:849 回写段锁) / ProbeNow(:963) / submitAll(:1345) / TryAcquireSubmit(:1897) / MarkDone(:1923) / RemoveDone(:1990) / RemoveFull(:2039)——首行 `s.mu.Lock()` 或缺省 deferred Unlock 形态，全量射证。

### 2. OBSERVE-117-01 —— ✅ 在位

写回侧先 ClientFor 复核（:1254）→ 二次 ClientFor 重取当前注册表 `client.Token()`（:1265）→ 非空才 `UpdateIDToken` 落库（:1269）——同名重建场景旧链已由 :1254 存在性复核拦死，:1265 再取到的 token 恒属当前注册表身份，绝不串旧身份 token 落库。Restore 侧（manager.go:295-317）恢复时 `SetCredentials(acct, pwd, cd.IDToken)` 用库内落库 token，与落库侧首尾一致。

### 3. B110-01 审计链 —— ✅ 在位

- **手动 6 失败位 AppendLog**：select 三路 :362（token 失效）/ :374（read 类预期未知）/ :381（其余业务失败）；exit 三路 :443 / :452 / :459——六路全部 `if aErr := ...; aErr != nil { log.Printf }` 留痕，零 `_ =`。
- **成功审计行**：手动报名成功 MarkDone 内 AppendLog(:1976, isOK=true) + 手动退选成功 RemoveDone 内 AppendLog(:2029, isOK=true)。
- **自动链失败族**：调度器 8 处 AppendLog（:1504 失效 / :1532 成功 / :1558 风控 / :1614 实时复核失效 / :1662 失败 / :1770 满员 / :1976 手动成功 / :2029 手动退选），全部失败分支 isOK=false 审计留痕。
- **零吞错穷举**：全仓 grep `_ =` / `_, _ =` / 直接赋值忽略形态仅 6 处且全部语义自洽——Prewarm goroutine（fire-and-forget 连接预热，报错无消费方）、ProbeForAccount per-account 探测 goroutine（`:1075 _, _ =` 探测失败静默由下一 tick 自愈）、MarkDone/RemoveDone `_ =`（二者方法内已完整记日志、返回 nil，外层忽略无损失）、zhidao client.go :107/:124 `io.Copy(io.Discard, resp.Body)` 两个 drain 收尾。**落库失败全部 `if err != nil { log.Printf }` 显式留痕，无 `_ =` 落库点**。
- **网络层 token 脱敏延续抽查**：doRequest :450 连接层错误统一 `sanitizeError(err)`（:581-593，剥 URL 文案、Unwrap 下沉底层判型）；scheduler :1283 重登成功日志 `maskedToken(newTok)`（前 8 位）；manager.go Restore 日志 `tokenShort`（前 8 位）。脱敏契约测试 6 测 + doRequest 端到端 1 测全绿（本轮定向复跑确认）。

### 4. O105-01 抖动基线 —— ✅ 在位

- socketPreheat（client_test.go:25，预创建-关闭 127.0.0.1 回环套接字排空冷启动窗口）与 readyProbe（client_test.go:82 / captcha_test.go:29 / handler_test.go:185 宽栅栏轮询）均在位；api handler_test.go:139 夹具入口明确 `readyProbe(zhi.URL)`；accounts 包 8 轮全绿无残余（manager_test.go:24 注释双方位记录）。
- 定向 race 实测（`export PATH=/d/mingw64/bin:$PATH`）：scheduler + zhidao + accounts + api 四包 + 其余六包 + `./...` 全绿（见验证表）。
- 回归锚 TestWindowOpenSubmitsWithoutProbeReset（探测量变后黄金期外首次 tick 立即提交不被节流吞）+ TestAdminStatsWindowOpenedUsesScheduler（admin stats 与学生端 /state 同源 windowClosedLocked）双绿复跑确认。

### 5. LOW-132 / LOW-133 回首核 —— ✅ 通过

- `time.Since()/time.Now()` 产品代码全量扫：残余仅两族——① `reloginAt` 写（:1231/:1261 本地钟）读（:1220/:1227 `time.Since(t)` 本地钟）**同基自洽**（30s 重登节流语义为同流程内部时间差，不依赖对齐钟）；② accounts/manager.go `gateWindow`（gateWait :53/:54、gateTryAcquire :226/:227、GatePump :71/:74）本地钟写读**同基自洽**（分钟窗口闸门语义为本地时间差）。其余非节流语义独立点（zhidao SyncServerTime RTT 测量 :113/:135/:137、captcha ?v 随机种子 :328、uniqueDeviceID 时间戳 :355、session 会话/票据 TTL、api loginLimiter handler.go:1176）均为各自子系统内自洽。探测族时间戳全部对齐钟写、对齐钟读，零新增时间基孤岛。
- `git diff f1d4b37..522354a -- backend/` 0 行 + `git diff 522354a -- backend/` 0 行，白线零漂移。

## 新契约角度纵深（自选 ×2，时间盒内完成）

**角度 A：窗口状态三判据单源 open 快照复用契约（选择理由：前三轮纵深覆盖探测定时族提交链路，但"每个判据函数取单次 open 快照复用于全部相关子判据"这一横向契约——热改亚毫秒窗口内两处独立取 open 读到新旧两个值的竞态根除——在三个入口间是否一致，本轮首次系统性比对）**

- **windowClosedLocked（:918-939）**：三条判据单源实现（state.WindowClosed 主判据 / syncFailStreak≥3 + 开放时间已过 / EmptyProbeRuns≥3 + 从未开窗 + 开放时间已过）。**:924 单次取 `open := s.openTimeForLocked("")`，判据 2（:925）与判据 3（:935）共用同一快照**——热改识别槽亚毫秒窗口内两次独立取 open 会读到新旧两个批次值（一次放行/一次判关）。StateForAccount(:703) 与 WindowClosed()(:910) 共用同一实现，杜绝两套真相分叉。
- **probe()（:1145）**：探测内 `open := s.openTimeForLocked("")` 单快照，同时喂给主判据 `state.WindowClosed`（:1146 `now.After(open.Add(10s))`）与 EmptyProbeRuns 入账（:1155 同 10s 裕量）——同一探测内两处 10s 裕量若各自取 open 会基于两个不同值（注释 :1143-1144 明示同款）。
- **tick()（:977）**：`open := s.openTimeForLocked("")` 单快照，同时喂给探测间隔判定（:987 `probeIntervalForOpen(now, open)`）、提交守卫零值（:1011/:1014）与提交间隔（:1031 `submitIntervalFor(now, open)`）——与 probeIntervalFor 便利包装（:82，仅供测试锚定的 13 处测试）刻意区分：生产 tick 绝不二次取 open。
- **三个入口统一契约**：每个持锁判定函数各自在入口取一次 open 快照、复用给函数内全部相关子判据；跨函数不要求同一快照（各函数锁内再取，语义一致）。空快照绝不删除识别槽（probe :1106-1110 只在非空 beginTimes 时覆盖），"关闭≠时间消失"契约在判据层与展示层 double 成立。

**角度 B：基础设施状态码家族整风核对（选择理由：B39-02/B40-01 曾改 5 类基础设施路径为真实 HTTP 状态码，本次对该家族做全量清单核对——鉴权 401 / 管理 403 / CSRF-403 / 限流 429 / panic 500 是否成家族、有无漏网路径、测试是否用真实状态码断言而非只看 body）**

- **家族清单全量核对**：recoverMiddleware 500（:1239）/ requireAuth 401（:1133）/ requireAdminSession 403（:642）/ 登录 429（router.go:107）/ 激活 429（router.go:121）/ JSON Content-Type CSRF 403 三处（router.go:72/:102/:116）/ 未知路径 404（router.go:198）/ 配置落库失败 500（:855）/ 统计目标数失败 500（:960）——**成家族对齐**。
- **writeJSON 本体与 100+ 业务调用点零改动**：业务路径仍恒 200，前端契约只读 body code（client.ts 从不读 HTTP status），HTTP 状态码为纯增强。
- **测试族用真实状态码断言**：TestLoginRateLimit 断言"body code=429 **且 HTTP 真实 429**"（handler_test.go:1415，注释明示修复前恒 200）；TestApiUnknownPath404 断言 404；TestRecoverMiddlewareHidesPanicDetail 断言 panic 后 HTTP 500；TestRequireJSONBodyRejectsFormContentType 断言 CSRF 403；TestAdminStatsTargetsLoadFailureReturns500 / TestAdminConfigSaveFailStillDispatch 断言 500 族。全部 `httptest.ResponseRecorder.Code` 断言真实状态码（B40-01 家族核对教训已落测试注释）。
- **非?account= 边角确认**：管理员 `DELETE /api/admin/accounts` 与 `/codes`（router.go:158/:137 requireJSONBody 是 POST 专用, 挂载路径核对无 body 门）为兼容标准 REST 客户端刻意去掉 requireJSONBody 门——副产品管理 token 泄露场景下跨站表单无法伪造这些 DELETE（无 body 表单 POST 不匹配 DELETE 方法），CSRF 风险面仍闭合（维持观察延续）。

## 维持观察项（延续既有记录 + 本轮）

1. `IsClassFull` 实时复核受 CountEntry.MaxCount 未实证下发限制——恒 false 兜底路径，真满员主判据为快照 max_count（classFullInSnapshot），属既有契约非缺陷。
2. `RemoveFull`（:2038）目前仅定义无产品调用方（预留接口），未来消费前保持观察。
3. `syncFailedWindow`（:182/:377/:394）写而不读，判据用 syncFailStreak——留档字段语义自洽，注释明示。
4. 验证码识别引擎 ddddocr 本地推理在 CGO=0 交叉编译形态依赖 native_ocr_stub 回退，属既有双轨契约。
5. accounts 包测试夹具无 socketPreheat 双保险（manager_test.go:24 注释自认），多轮全绿实证无残余——若未来再出冷启动 flake 第一候选即补 socketPreheat。
6. 管理员 `DELETE /api/admin/accounts` 与 `DELETE /api/admin/codes` 刻意去掉 requireJSONBody 门——副产物 CSRF 风险面仍闭合。维持观察。
7. **warnedNoTargets 无锁写点（:1371-1372）**：宿主唯一、单写者单读者连续执行，语义自洽非漂移。首例并发写 Race 报告出现前维持观察。
8. `probeSem` cap=4 常驻（`ponytail:` 注释标记），平台熔断收紧或账号数突破 50 时需重估——当前无触发信号。

## 结尾建议

**APPROVE**

身份防线矩阵第七十一轮闭合成立：sameClientFor 7 调用点终局逐一追写零漂移（六分支 + 探测回写全族对称，实时复核结果块为最后一块裸露写点已闭合），maybeRelogin 双侧完整（决策侧 :1208 存在性复核 + 写回侧 :1254 复核 + :1265-1273 二次 ClientFor 重取当前注册表 Token 落库）、手动五路 accountExists（:255/:305/:397/:497-512/:573）、写点换类 5 类 + warnedNoTargets 唯一性、*Locked 写函数族 13 个双向射证全部实测确认；OBSERVE-117-01 知识位、B110-01 审计链、O105-01 抖动基线、LOW-132/133 回首核四项独立位点均确认在位。新契约角度两方向（窗口状态三判据单源 open 快照复用 / 基础设施状态码家族整风核对）无漂移。CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。