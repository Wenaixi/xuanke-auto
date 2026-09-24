# R151 后端只读审查 Findings —— 身份防线矩阵第六十六轮闭合

> 审查基线：`6045db1`（R150 归档）。模式：绝对只读，唯一写文件为本报告。
> 时间：2026-09-24。实测工具：`go build` / `go vet` / `go test -race -count=1`（MinGW gcc，`export PATH=/d/mingw64/bin:$PATH && export CC=gcc`）+ 走读追写。

## 结论前置（分级）

**CRITICAL 0 / HIGH 0 / MEDIUM 0 / LOW 0（无新增缺陷）**

本轮为纯观察轮，身份防线矩阵第六十六轮闭合成立。全部聚焦项与验证项经实测归因 ✅ 在位，未发现任何漂移、遗漏或新契约缺口。

---

## 验证表（实测时间与数据）

| 验证项 | 结果 | 实测证据/数据 |
|--------|------|---------------|
| `go build ./...` | ✅ | EXIT=0，无输出 |
| `go vet ./...` | ✅ | EXIT=0，无输出 |
| 定向 race 四包（强制非缓存） | ✅ 全绿 | `-race -count=1`：zhidao 2.037s / accounts 1.302s / scheduler 15.007s / api 13.528s；另补充 secure 1.321s + session 1.410s + store 11.611s 三包也全绿 |
| 身份防线族十测（race） | ✅ 全绿 | TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + SameNameChainRealtimeUnauthorizedDropsRelogin + SameNameChainSuccessDropsInflight + TestMaybeReloginDeletedAccountSkipsMaps + TestProbeDeletedThenRebuiltSameNameDropsSnapshot + TestProbeForAccountDropsWriteWhenRemoved，13 项全 PASS |
| 回归锚双测试 | ✅ 双绿 | TestWindowOpenSubmitsWithoutProbeReset（race 1.03s）已由测试夹具证伪、TestAdminStatsWindowOpenedUsesScheduler（-race 0.42s）均 PASS |
| 轮次标签扫描 | ✅ 零命中 | `grep "第 N 轮\|R..轮\|round N\|RoundN"` 产品代码零命中；测试文件中仅两处叙述性"第 3 轮"（scheduler_test.go:1659/:1662/:2518 描述同步失败循环语义，属行为叙述非轮次前缀标签）与 RoundTrip 测试命名，合规 |
| 工作区零漂移 | ✅ | `git status --porcelain` 仅 `?? archive/review-rounds/round151-frontend-findings.md`（并发代理产物，非本次修改），产品代码零 diff |
| 全量 go test ./...（无 race） | ✅ | 11 包全 ok |
| 时间基 LOW-132/133 回首 | ✅ | 见专项第 5 节 |
| 契约注释轮次标签 | ✅ | 产品代码注释零"第 N 轮/R..轮"前缀；历史决策归手册，代码只讲"为什么/契约/陷阱"（抽样 scheduler.go 各处注释符合） |

---

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十六轮闭合 —— ✅ 在位（7 调用点终局逐一追写）

**sameClientFor 定义与 7 调用点坐标核对（相对基线零漂移）**

- 定义 `clientIdentity` reflect 指针身份：scheduler.go:199-224，`reflect.ValueOf(c).Pointer()`，nil→0，接口值解码判 Ptr 类型，与契约一致。
- 实测 7 调用点行号：**:850（ProbeForAccount 回写段）、:1489（失效分支）、:1521（成功分支）、:1551（风控退避分支）、:1571（窗口关闭分支）、:1600（实时复核三路入口）、:1635（实时复核确证满员分支）**。

**各调用点"写什么状态/落什么库行"终局逐一追写：**

| 调用点 | 位置 | 身份复核失败时放弃的写入 |
|--------|------|--------------------------|
| 探测回写 | :850 | 放弃 `acctData[acct]`/`acctDataAt[acct]`/`openTimeDetected[acct]` 识别槽写入（不落库，纯内存帧；防年级串线/过期数据一帧可见） |
| 失效分支 | :1489 | 放弃 `inflight` 残留（:1490 delete）+ 跳过 maybeRelogin 触发 + 不写 failed 状态 + 不 AppendLog 失效日志（:1502/:1504 两处被整体短路；重登是异步副作用，identity 复核前置保护 manager 失败计数不被新身份污染） |
| 成功分支 | :1521 | 放弃 `done[acct][id]=true` + setStateLocked success + AppendLog(success) + SaveSuccess 库行（:1526-1540 整块短路，重启 RestoreDone 无假成功） |
| 风控退避 | :1551 | 放弃 `markRateLimitedLocked`（30s 退避）+ failed 状态 + AppendLog（:1555-1561 整块短路，黄金期不被假退避静默跳过） |
| 窗口关闭 | :1571 | 放弃 `markFullLocked`（永久满员退避）+ 状态 + AppendLog（:1575 短路） |
| 实时复核入口 | :1600 | 拦截其下三路（ErrUnauthorized→maybeRelogin/失败写库；确证满员→markFullLocked；普通失败→failed+AppendLog）整块写面 |
| 实时复核确证满员 | :1635 | 在 doneHas 让位（:1627 绝不覆盖胜利状态）之后加码身份复核，放弃 markFullLocked |

六分支对称结论：成功/失效/风控/窗口关闭/确证满员/实时复核 ErrUnauthorized（:1608），加上栈顶已删检查（:1388）与链顶取 client 后二次判（:1409），七道防线成族闭合，无裸露写点。

**maybeRelogin 双侧确认：**

- 决策侧 :1208：入口锁内先 `ClientFor(acct)` 存在性复核，已删账号整体 return（不写 tokenValid/reloginFail/reloginAt/relogging 任何 map），与 B43-01 契约一致。
- 写回侧 :1254：goroutine 内先 ClientFor 复核→:1259 `err==nil && relogged` 成功分支→**二次 ClientFor 重取当前注册表 client（:1265）`client.Token()` 落库（:1269 UpdateIDToken）**——OBSERVE-117-01 知识位第三十四轮在位，同名重建场景旧 goroutine 复得的 Token 会属于当前身份（事实上二者一致），但**落库写入方是二级复核后的最新客户端指针**，结构上彻底排除"写的是旧身份 token"。失败侧 :1292 之前只清 relogging，reloginFail 保留递增后的计数（指数退避表不振荡）。
- 手动路径 `MaybeRelogin` 导出别名（:1323）与 `MarkTokenValid`（:1312，含 reloginMu→s.mu 锁序对齐）对称闭环。

**手动五路 accountExists 确认：**

- 课程读 :255、手动报名 :305、手动退选 :397、状态读 :573、目标写 :497-512（LoadCredentials 循环判 found）——五路全部凭据表判据，accountExists 定义 :1109-1120。允许 override 门 :1103 仅管理员会话。
- 实测 5 路测试全部 PASS：TestAdminElectivesUnknownAccountRejects / TestAdminElectiveSelectUnknownAccountRejects / TestAdminStateUnknownAccountRejects / TestSetTargetsUnknownAccountDoesNotFabricate / TestAdminDeleteAccountMemoryFirst（--race 全绿）。

**写点换类 5 类 + 无锁写点唯一性：**

- 5 类换查看全部持锁：lastSubmit（submitAll :1346 锁内 `nowAlignedLocked`，读侧 tick :1029 锁内）、lastSyncStart（maybeSyncClock :356 锁内）、syncing（:355 置位 / :369 回写 / :404 复位，均锁内）、lastProbe（probe:1089/:1114、ProbeNow:964，均锁内；tick 读 :976 锁内）、state.EmptyProbeRuns（probe :1156 锁内）。
- 无锁写点 warnedNoTargets：唯一宿主 submitAll :1371-1372，判定在 :1368 `if len(chains)==0` 之后、:1375 return 之前——链式 if 内部连续执行，host 唯一、单写者、初始 zero-value false 在 SetTargets 后有目标再全删时必然走到，语义自洽（每 tick 只打一次警告）。

***Locked 写函数族 13 个 + 外部写函数首行取锁双向射证：**

- 定义 13 个：nowAlignedLocked :273 / openTimeForLocked :427 / enrichTargetPubMetaLocked :539 / rebuildCoursesForAccountLocked :570 / rebuildCoursesLocked :647 / tokenValidForLocked :739 / windowClosedLocked :918 / isRateLimitedLocked :1691 / markRateLimitedLocked :1710 / markFullLocked :1760 / releaseFullIfFreedLocked :1781 / statusIndexLocked :1835 / setStateLocked :1846。实测每个只被 Locked 上下文中调用（追到 spawnChain 锁段 / tick 锁段 / StateForAccount 锁段 / WindowClosed 锁段等）。
- 外部写函数首行取锁核查：SetTargetsForAccount :455 首行 `s.mu.Lock()`、PurgeAccount :496 同、RestoreTargets :526 同、RestoreDone :608 同、RestoreRefused :627 同、ProbeForAccount 回写段 :849 锁（写前 sameClientFor）、ProbeNow :963 锁、ElectivesSnapshotFor :762 锁、MarkDone :1923 / RemoveDone :1990 / RemoveFull :2039 / TryAcquireSubmit :1897 全部首行取锁。双向射证成立。

### 2. OBSERVE-117-01 知识位第三十四轮 —— ✅ 在位

见第 1 节 maybeRelogin 写回侧（:1254→:1265-1273）。二次 ClientFor 重取当前注册表 `client.Token()` 落库，同名重建场景绝不串旧身份（源码 + TestReloginSuccessWithNilStoreNoPanic 等链上测试绿）。

### 3. B110-01 审计链第四十一轮 —— ✅ 零漂移

- 手动 6 失败位 AppendLog 逐一确认：handler.go :362（报名失效）、:374（报名 read）、:381（报名业务失败）、:443（退选失效）、:452（退选 read）、:459（退选业务失败）——全部 `if ... != nil { log.Printf }` 零吞错。
- 成功行：报名成功走 MarkDone 内 AppendLog（:1976），退选成功 RemoveDone 内 AppendLog（:2029 区域），目标保存 :558，登录 :237，管理员 login :136、删除 :1055。全部带错误处理。
- 自动链失败族：scheduler spawnChain 六分支 AppendLog 全部 `if err := ...; err != nil { log.Printf }`（:1504/:1532/:1558/:1614/:1662/:1770）。
- 零吞错穷举：`_ =`/`_, _ =`/直接赋值忽略形态扫描——scheduler.go 仅两处 `_ =`（:307 Prewarm goroutine 内错误丢弃、:1075 ProbeForAccount 探测丢返回值），均**非落库**且设计语义正确（预热/探测并发 goroutine 结果不需上抛）；handler.go 两处 `_ =`（:387 MarkDone、:467 RemoveDone）为手动成功路径，error 恒定 nil（函数内绝不返回 error 语义分支见源码），非落库忽略；落库点 grep 穷举零命中（无 `_ = store.X()` 形态）。
- 网络层 token 脱敏延续：zhidao/client.go :444-450 doRequest 统一 `sanitizeError`（:581-593 剥 url.Error URL、保留 Unwrap 判型），maskedToken :1334（>8 位仅前 8 位）；调度器 :1283 重登成功日志走 maskedToken；TestMaskedTokenBoundary / TestSanitizeErrorStripsTokenFromDialError / TestPreservesJudgment / TestOriginalErrorPreserved / TestNeverEmptyError 全在且绿。
- 连接活性自愈族：httpDo :478-492（dial/write 重试一次，read 不重试防双报），判型 isConnErrRetryable :496 / IsReadErr :523，captcha.go :162 也走 httpDo——两复用路径统一收口。TestIsReadErrCoversAllForms 绿。

### 4. O105-01 抖动基线 —— ✅ 实测绿

- socketPreheat/readyProbe 夹具在位：zhidao/client_test.go:25/:88、captcha_test.go:17/:29、sanitize_test.go:97、api/handler_test.go:179/:185、accounts/manager_test.go:26 均有，尚覆盖 scheduled probes。
- 定向 race 实测：用 `/d/mingw64/bin/gcc`（已有）`export PATH=/d/mingw64/bin:$PATH && export CC=gcc` 跑 zhidao/accounts/scheduler/api 四包全绿（含多轮）。
- 回归锚双绿已列入验证表。

### 5. LOW-132 / LOW-133 回首核 —— ✅ 通过

- `time.Since()/time.Now()` 产品代码全量扫：scheduler.go 仅 7 处 `time.Now()/time.Since`——:269/:274（nowAligned 定义本尊）、:1220/:1227/:1231/:1261（reloginAt 写读）+ :1707 注释。判定：残余仅 **reloginAt 写读同基自洽**（本地钟写入+本地钟判期，重登节流 30s 语义不依赖对齐钟）+ **gateWindow**（accounts/manager.go：gateWait/gateTryAcquire/GatePump 三处均本地钟写读，窗口语义一致）——两处均合规，与 R134-R150 各轮裁决一致，零新增时间基孤岛。
- git show 白线核对（相对基线无新增混用）：无漂移。

### 6. 新契约角度纵深（自选 ×2，时间盒内完成）

**角度 A：凭据 AES-256-GCM 加密链终局走查（选择理由：凭据是防御纵深核心，含 OBSERVE-117 落点的 token 与密码两路）**

- 加密器：secure/crypto.go AES-256-GCM + 随机 nonce 前置 + hex；LoadOrCreateKey 拒绝非 32 字节 env/损坏 .master_key（拒绝带伤启动）。main.go:50-55 注入。
- 密码入库：accounts.Manager.LoginByPassword :277-288 `encrypt(password)` → SaveCredential 落库 `password_enc`；加密失败明确留痕（:286，零吞错）。store.go:29 SaveCredential UPSERT。
- token 入库：项目经理 Login 成功 `c.SetCredentials`（client.go:270）→ LoginByPassword 内 SaveCredential(idToken) 同库行；调度器重登成功 UpdateIDToken（store.go:56）走客户段二次 ClientFor 最新 token——两条写面同源（客户段 Token()）。
- vision_key 入库：handler.go:833 secureEncrypt（`enc:` 前缀）→ settings 落库；main.go:88-97 拒不兼容明文（拒绝旧版未加密）。
- 解密侧唯一入口：accounts.Manager.Restore :295-317（decrypt PasswordEnc）+ main.go:90（vision_key）。服务启动即拒绝未加密。
- 全链无明文滞留面：磁盘仅 `enc:` 前缀密文；内存密码只在客户端 account/password 与本次登录事务内存在。测试：TestEncryptDecryptRoundTrip / TestEncryptUniqueNonce / TestDecryptTamper / TestLoadOrCreateKeyRejectsTruncatedFile / TestAdminConfigVisionKeyEncryptedAtRest / TestAdminConfigRefuseUnencryptedVisionKey / TestManualElectiveFailureAppendsLog 全套绿（含 race）。结论：凭据加密链闭合无通洞。

**角度 B：基础设施 HTTP 状态码家族复盘 + 识别引擎热切换模板链（选择理由：B39-02/B40-01 家族契约需成族核对，且决策 18 模板链是热切换正确性核心）**

- 状态码家族四类逐一盘点（router/handler）：panic 500（:1239 recoverMiddleware→writeJSONStatus）、会话 401（:1133）、管理 403（:642）、CSRF-403（router requireJSONBody :72/:102/:116 →403）、限流 429（router :107/:121）、stats 目标数失败 500（:960）、config 落库失败 500（:855）。测试断言 **HTTP 状态码本体**（TestRecoverMiddlewareHidesPanicDetail 断言 rec.Code==500、TestRequireJSONBodyRejectsFormContentType 断言 403、TestLoginRateLimit 断言 429、TestAuthRequired 断言 401）——家族成族核对通过，无漏网 writeJSON 恒 200 的基础设施失败路径。
- 识别引擎热切换模板链：accounts.Manager.SetVision :191-199 `cfg.WithRecognizer(m.vision.Recognizer())` 保留当前引擎 → SetRecognizer :209 同步模板 + 逐客户端注入；zhidao.Client.SetVision :167（ddddocr 部署时保留本地引擎）→ SetRecognizer :181 写 recognizer。applyCaptchaRecognizerFor（router.go:33）在 handleAdminConfig 热更新 + 启动 init 双侧调用。决策 18"引擎切换绝不挥动"实测成立（TestNewClientAfterSetRecognizerGetsEngine / TestSetVisionKeepsLocalRecognizer / TestSetVisionRebuildsWhenCurrentIsVisionOrNil 绿）。

---

## 维持观察项（无新增，延续既有记录）

1. `IsClassFull` 实时复核仍受 CountEntry.MaxCount 未实证下发限制——恒 false 兜底路径，真满员主判据为快照 max_count，属既有契约非缺陷。
2. `RemoveFull`（:2038）目前仅有定义无产品调用方（外部手动/snapshot 更新语义保留），属预留接口，非死代码但未来消费前保持观察。
3. `syncFailedWindow` 写而不读（:182/:377），判据用 syncFailStreak——留档字段语义自洽。
4. 验证码识别引擎 ddddocr 本地推理在 CGO=0 交叉编译形态依赖 native_ocr_stub 回退，属既有双轨契约。
5. gateWait 阻塞型与 gateTryAcquire 非阻塞型共享 gateUsed 计数——退避窗口内手动登录立即被拒但排队重登不受影响，语义经测试固化（TestLoginByPasswordRejectsWhenGateBudgetExhausted 等绿）。

---

## 结尾建议

**APPROVE**

身份防线矩阵第六十六轮闭合成立，OBSERVE-117-01 知识位第三十四轮、B110-01 审计链第四十一轮、O105-01 抖动基线、LOW-132/133 回首核四项独立位点全部实测确认；新契约角度两方向（凭据加密链终局 / 状态码家族+热切换模板链）无漂移。CRITICAL/HIGH/MEDIUM/LOW 全零，无修复需求，可归档。