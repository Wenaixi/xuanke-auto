# round140 后端审查发现（身份防线矩阵第五十五轮）

> 审查模式：绝对只读。工作树基线 `16bb702`（R139 归档）。产物仅 `archive/review-rounds/` 下两份 findings（本文件 + r140-frontend 由并行前端代理产出），其余零修改。
> 实测环境：Windows 11 / go1.26.8 / mingw64 gcc（race 需 cgo）。

## 结论前置

| 级别 | 数量 | 说明 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| LOW | 1 | 观察项延续：`handleAdminDeleteAccount` 中 `d.Sched.PurgeAccount(acct)` 失败时无状态回滚（半删态自愈语义已文档化，非缺陷，见「维持观察项」） |

**建议：APPROVE（通过）。** 身份防线矩阵第五十五轮零漂移，OBSERVE-117-01 知识位第二十三轮在位，B110-01 审计链第三十轮零吞错，O105-01 抖动基线证据充足，LOW-132/133 时间基残扫零混用。无任何阻塞项。

## 验证表

| 验证项 | 实测时间 | 结果 | 数据 |
|--------|----------|------|------|
| 基线确认 | 2026-09-24 | ✅ | `git rev-parse HEAD` = `16bb702faf9686a1b2de767e2f9dd192260e7b0f`，工作区仅 `?? archive/review-rounds/round140-frontend-findings.md` 一个未跟踪文件（并行前端代理产物），`git status --short --branch` 无其他漂移 |
| `go build ./...` | 2026-09-24 | ✅ | 全包通过 |
| `go vet ./...` | 2026-09-24 | ✅ | 全包通过 |
| 定向 `-race` 四包 | 2026-09-24 | ✅ | `./internal/zhidao ok`（cached 后 fresh）· `./internal/accounts ok 1.271s` · `./internal/scheduler ok 15.081s` · `./internal/api ok 15.343s`，均为 `-count=1` 无缓存实跑 |
| `-race` 扩展六包 | 2026-09-24 | ✅ | `db 5.336s / secure 4.590s / session 4.758s / runtime 2.138s / config 4.619s / store 23.900s` 全部绿 |
| 身份防线族十测复跑 | 2026-09-24 | ✅ | `-run "TestWindowOpenSubmitsWithoutProbeReset\|TestDeletedAccount\|TestRealtimeRecheck\|TestUnauthorizedBranch\|TestMaybeRelogin\|TestProbeDeletedThenRebuilt"` scheduler 3.219s 绿；调度器全量 3.917s 绿（含 30+ 防线族） |
| 回归锚 | 2026-09-24 | ✅ | `TestWindowOpenSubmitsWithoutProbeReset`（scheduler_test.go:1277）与 `TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go:1011）均在位，每轮 race 必然覆盖 |
| 轮次标签扫描 | 2026-09-24 | ✅ | 产品代码 `第 N 轮/R..轮/round N` 零命中；唯一命中 `session/store.go:117` 是注释引用 `docs/review-round13.md` 的文档路径（合规）；测试文件仅 "第 3 轮" 状叙述（指连续探测轮数语义） |
| 时间基残扫 | 2026-09-24 | ✅ | 见 LOW-132/133 回首核 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第五十五轮闭合 — ✅ 在位

**sameClientFor 定义调度器内部无漂移。** `scheduler.go:204-210`，内部 `ClientFor` 取当前注册表客户端，`clientIdentity`（scheduler.go:215-224）用 `reflect.ValueOf(c).Pointer()` 做指针身份比对，nil / 非指针 / 账号不存在均返回 false。与 R135-R139 归档描述逐字一致。

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

**手动五路 accountExists**：`handler.go:255`（handleElectives 课程读）、`:305`（handleElectiveSelect 手动报名）、`:397`（handleElectiveExit 手动退选）、`:497-512`（handleSetTargets 目标写，凭据表内联遍历）、`:573`（handleState 状态读）。判据同源 `accountExists`（:1106-1120，LoadCredentials 逐账号比对，"确实登录过"的更强真理源），五路全覆盖，查无此账号整体拒绝、绝不用全局帧/空状态假装成功。`session/store.go:161 Account(token)` 校验过期与吊销，`handleAdminDeleteAccount` 的 `RevokeAccount` 让被删账号既有令牌立即失效。

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

### 2. OBSERVE-117-01 知识位第二十三轮 — ✅ 在位

写回侧顺序实测：`s.clients.ClientFor(acct)` 复核（:1254）→ `err==nil && relogged` 成功分支 → `delete(reloginFail)`/`reloginAt=Now`/`tokenValid=false`（:1260-1262）→ 锁内二次 `ClientFor(acct)` 重取 `client.Token()`（:1265-1267）→ `UpdateIDToken` 落库（:1269）。同名重建场景下旧 goroutine 在一次复核即被丢弃，二次重取拿到的是"复核通过时"的注册表客户端，Token 为当前身份新 token，绝不串旧身份。写回侧防线上还保留 `tokenValid` 关键语义：`tokenValid[acct]=true` 只在发起决策段置位（:1241），成功分支才清（:1262），与 `MarkTokenValid` 锁序（reloginMu→s.mu，:1307-1319）串行化，杜绝"手动登录清标记与在途重登置位"的并发半态。

### 3. B110-01 审计链第三十轮 — ✅ 在位

**手动 6 失败位**（handler.go）：`:362` 报名 token 失效 / `:374` 报名 read 类（请求已发出结果未知，最需留痕）/ `:381` 报名其余业务失败 / `:443` 退选 token 失效 / `:452` 退选 read 类 / `:459` 退选其余业务失败——六位全部 `AppendLog` 且 `if err != nil { log.Printf }` 零吞错。**成功审计行**：手动报名成功走 `MarkDone` 内部 `AppendLog(select, msg, true)`（:1976）+ `SaveSuccess`(:1973)，手动退选成功走 `RemoveDone` 内部 `DeleteSuccess`/`SaveRefused`/`AppendLog(exit, true)`（:2020/2026/2029）；`_ = d.Sched.MarkDone(...)`（:387）与 `_ = d.Sched.RemoveDone(...)`（:467）的返回值恒 nil（内部已处理全部落库错误），不吞任何真实错误。**自动链失败族**：spawnChain 失效/风控/窗口关闭/实时复核失效/确证满员/普通失败六分支全部 `AppendLog`（scheduler.go:1504/1558/1614/1662/1770/1532/1535），全部 `if err != nil` 记账。

**零吞错穷举**：`grep "_ = \|_, _ ="` 全仓（排除测试与二进制 onnxruntime.dll）命中仅 8 处——`config.go` 的 `os.Setenv/os.MkdirAll/os.WriteFile`（配置落盘最佳努力，模板生成失败有后续日志兜底）、`scheduler.go:307` `pw.Prewarm()`（异步预热）、`:1075` probe 账号子 goroutine 的 `ProbeForAccount`（信号量内并行，每账号子链自身日志）、`client.go:107/124` `io.Copy(io.Discard)`（刻意丢弃响应体）、browser/tray 启动调用。**全部 store 落库调用点 29 处逐一 grep 复核：零 `_ =` 形态，全部 `if err != nil { log.Printf }`**。零吞错穷举通过。

**token 脱敏延续**：`client.go:581 sanitizeError`（剥 url.Error 完整 URL 的 idToken 段，`sanitizerErr` Unwrap 下沉保判型）→ `isConnErrRetryable`/`IsReadErr` errors.As/Is 穿透不变；`scheduler.go:1283` `maskedToken(newTok)` 只显前 8 位；`sanitize_test.go` 双测钉住"输出不含 token/idToken= + 判型保留"。账户恢复侧 `manager.go:315` `tokenShort(cd.IDToken)` 同样只显前 8 位。

### 4. O105-01 抖动基线 — ✅ 证据充分

`socketPreheat` 夹具在位：`zhidao/client_test.go:25`（预创建-关闭 127.0.0.1 回环套接字排空冷启动窗口）；`readyProbe` 夹具在位：`accounts/manager_test.go:26`（含"accounts 未配 socketPreheat 双保险、8 轮全绿实证、残余则第一候选即补"的注释）、`api/handler_test.go:179`、`captcha_test.go`（"socketPreheat 只预占单端口，并发首请求形态需 readyProbe 把 accept 就绪前首请求吃掉"与 api/zhidao 同款）。四个关键包 `-race -count=1` 无缓存实跑全部绿（见验证表），身份防线族十测归入定向复跑。回归锚两枚常年内置在每轮 race 中，均绿。

### 5. LOW-132/133 回首核 — ✅ 通过

`git show` 白线：`975dc2b`（LOW-132-01）把 `lastSyncFailAt`/`syncFailedWindow` 写入改 `nowAlignedLocked`，判读侧 `:342` 同基准；`d75f38c`（LOW-133-01）把快照 TTL 判读侧四处 `time.Since` 改 `nowAlignedLocked().Sub`，写入侧 `:835/:864/:1116/:966` 已对齐钟。两次提交均带 TDD 测试（`TestClockSyncFailureBackoffUsesAlignedClock` / `TestSnapshotTTLUsesAlignedClock` 先红后绿）。

全量时间基残扫（`time.Since|time.Now()`，排除测试与 `time.Time{}` 字面量）现残余共三族，**写读同基自洽为合规**：
- **scheduler relogin 族**：`time.Since` 仅 `:1220/:1227`（reloginAt 读）与 `time.Now` `:1231/:1261`（reloginAt 写）——写读同基自洽，属 LOW-134 留下的"残余仅 reloginAt 一处"合规态。
- **accounts gateWindow 族**：`manager.go:53/71/74/226/227`——gateWait/gateTryAcquire 的 gateWindow 写（time.Now）与读（time.Since）同基自洽；登录闸门语义本就基于本地钟（全账号 doLogin 预算，与平台时钟无关），不参与开窗点判定。
- **独立自洽孤岛**（不涉开窗点/快照 TTL 判定，各为独立闭环）：api `loginLimiter` tokenBucket（handler.go:1176）；session 会话/票据 TTL 过期（store.go:74/106/128/156/168/183/198）；zhidao 非登录动态参数生成（captcha `v=`、`uniqueDeviceID` 均为单次生成值）。

残留零混用，LOW-132/133 回首核通过。

## 新契约角度纵深（自选 ×2）

### 角度 A：凭据 AES-256-GCM 加密链 + 会话级账号绑定

选取理由：登录/会话/凭据是本系统鉴权的边界，C-1 数据安全防线在历轮多被抽查，本轮从「密文形态→主密钥管理→库行载体→读回解密→会话绑定」全链路逐环实测。

**加密链闭环**：
1. 密钥源：`main.go:49-53` `secure.LoadOrCreateKey`——`XUANKE_MASTER_KEY`（64 位 hex）优先，否则生成 32 字节随机密钥落 `data/.master_key`；`secure/crypto.go:16-31` 对非 64 位 hex 环境变量、非 32 字节损坏密钥文件一律拒绝（`errors.New` 拒绝初始化），绝不带病工作。
2. 注入：`encrypt/decrypt` 闭包注入 `Deps`（main.go:54-55 + api.Register 传参），`secureEncrypt`（handler.go:60-71）对未注入 Encrypt 返回"拒绝未加密存储"。
3. 落库：登录成功 `manager.go:279` `m.st.SaveCredential(acct, enc, token)`——密码密文 + 当前 token 写入 credentials 表 `password_enc` 列（store.go:29-35 upsert）；`vision_key` 存 settings 表同样走 `enc:` 前缀。
4. 读回：`main.go:88-98` 对 settings 的 vision_key **严格要求 `enc:` 前缀**，未加密旧明文"彻底拒绝加载"；`manager.go:295-317` Restore 时 `decrypt(cd.PasswordEnc)` 解密失败（主密钥变更后旧密文不可解）记日志降级"无保存账密"，绝不把密文当明文注入。
5. 兜底：`db.go:59-98 refuseLegacy` 对 account 表/空账号目标/缺 priority/allow_swap/account 列/缺 settings 表的旧库形状拒绝启动；`migrateAddPublishMeta`（db.go:43-57）只做"纯新增列"自动迁移旧库放行，分界判据与决策契约「数据库增量迁移规范」逐字对齐，且 `refuseLegacy` 缺列清单已剔除已迁移列——exe 双击不再因缺新列 log.Fatal。

**会话绑定闭环**：`requireAuth`（handler.go:1123-1139）把 `sessionAccount(r)` 注入 context；五路写路径（目标/选课/退选/状态/日志）全部 `acct := sessionAccount(r)` 起步；管理员透传 `?account=` 统一经 `allowAccountOverride`（= `IsAdminToken` 会话，handler.go:1102-1104）放行，撞名学生普通会话 token 的 Admin=false 永不匹配（F52-M1 管理令牌判据绑定"后端签发带 adminName"实测在位）；`Sessions.RevokeAccount` 删除账号即吊销全部既有令牌（不等 12h TTL）。`Store.DeleteAccount`（store.go:388-415）事务内清 credentials/targets/success/refused/activations 六表，与 `Accounts.Remove`→`PurgeAccount`→`RevokeAccount` 组成 memory-first 四序。

结论：加密链从密钥熵源到旧库兼容全环闭环，会话账号绑定与管理员穿透判据同源，零断点。

### 角度 B：writeJSONStatus 基础设施状态码家族复盘

选取理由：round40 立下"同类基础设施错误路径改真实状态码时必须逐家族核对"的规则，本轮把该规则当持续审计点做全族清点，验证 HTTP 状态码语义与前端 body.code 契约双轨无冲突。

**writeJSONStatus**（handler.go:95-99）与 writeJSON 同款响应体 + 真实 HTTP 状态码；`writeJSON` 本体保持"业务失败仍 200"（handler.go:84-89 注释：前端契约只读 body code）。

**家族全清点**（全部 httptest 可断言真实状态码）：
| 家族 | 位置 | 状态码 |
|------|------|--------|
| panic / recoverMiddleware | handler.go:1239 | 500（TCP/代理可见） |
| requireAuth 会话失效 | handler.go:1133 | 401 |
| requireAdminSession | handler.go:642 | 403 |
| requireJSONBody CSRF 门（三处副作用路由包装 + 登录/激活已内联） | router.go:72/102/116 | 403（B40-01 补的最后一块缺口实测在位） |
| 登录限流 | router.go:107 | 429 |
| 激活限流 | router.go:121 | 429 |
| admin stats 目标数失败 | handler.go:855/960 | 500 |
| SPA 兜底未知接口 | router.go:195-198 | 404（Go 1.22 `HandleFunc("GET /...")` 未注册路径，先设头再 writeJSONStatus） |

五大家族（鉴权 401 / 管理与 CSRF 403 / 限流 429 / panic 500 / 未知路由 404）逐项核对在位，规则遵守"先 httptest 断言真实状态码再实现"的落地方式延续到 round40 后的每一个新增路径。前端 client.ts 从不读 HTTP status，body.code 契约不受任何 428→500 语义变化影响。

## 维持观察项

1. **LOW 观察（延续）**：`handleAdminDeleteAccount` 中 `PurgeAccount` 无错误返回、若 `Store.DeleteAccount` 失败则进入"注册表已摘 / 库行未清"的半删态——已文档化为"重启 Restore 重建客户端自愈"的刻意决策，且远优于"假删除成功"（客户端在飞写回）。维持观察，不列为缺陷。
2. **obs 观察（延续）**：`classFullRealtime` 底层 `IsClassFull` 因平台未下发 maxCount 恒判不满，实时复核实为防御性路径——真满员主路径为快照 `max_count`（实证字段）。契约已在 CLAUDE.md 落盘，审核轮次持续确认不在本轮引入新的误判形态。
3. **`warnedNoTargets` 无锁写点的宿主唯一性**依赖 `submitAll()` 的生产唯一调用点 `tick:1035`——该依赖被 `scheduler_test.go` 的 `s.submitAll()` 直接调用（测试态多 goroutine 不触发，仅串行无锁场景下唯一性成立）。若未来新增 `submitAll` 并发调用宿主，需先补锁。维持观察。
4. **观察**：api 登录/激活限流桶 `loginLimiter` 使用本地钟自洽闭环（写读同基），但 token 桶容量/速率常量（`loginBurst`/`loginRate`/`bucketTTL`）为编译期常量，无运维调参通路——如果学校 NAT 下误杀需要运行时放宽，属未来可选增强，非本轮缺陷。

## 结尾建议

**APPROVE（通过）。**

身份防线矩阵第五十五轮：sameClientFor 定义与 7 调用点零漂移、每条追到写状态/落库终局，maybeRelogin 决策侧/写回侧双侧复核 + 二次重取 Token 落库在位，手动五路 accountExists 判据同源，五大写点类全持锁、无锁写点 warnedNoTargets 宿主唯一性射证成立，*Locked 族双向射证闭环。OBSERVE-117-01 知识位第二十三轮在位。B110-01 审计链第三十轮手动 6 失败位 + 成功行 + 自动链失败族全部落地，零吞错穷举通过。O105-01 抖动基线 socketPreheat/readyProbe 夹具在位，定向 race 四包 + 扩展六包实证全绿。LOW-132/133 回首核白线核对 + 时间基残扫零混用。新契约纵深（凭据加密链/会话绑定、writeJSONStatus 家族）两向均无断点。

无 CRITICAL/HIGH/MEDIUM，1 条 LOW 观察项为延续性观察，无阻塞——轮次归档可闭合。