# R67 后端只读审查发现报告

> 审查基线：master @ `868d7a6`（R66 收官 docs 提交）。R66 后端零改动，仅归档 docs（`6b91d12`/`868d7a6`）+ 前端卫生修（`9b8854f`）；基线之后工作树有 `506ab3c`（前端格式卫生二修，与后端无关）+ 前端代理新增 `round67-frontend-findings.md`（未提交，后端零相关）。审查期间绝对只读，临时验证全部为只读 go test 复跑（无 %TEMP% 独立程序需求——R66 两条 OBSERVE 的结论已由既有测试护栏与源码实证闭合，无需再建独立复刻）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部测试文件。
> 方法：R66 两条 OBSERVE 逐条复核（源码逐行 + 既有测试护栏）+ 历轮决策手册契约抽查（8 条）+ 全包逐行通读找新问题 + 全量 `-race -count=1 -p 1 -timeout 900s ./...` **5 轮** flake 统计 + mock-heavy 5 包串行 **4 轮** 补充 + 隔离复跑 + gofmt/vet/build 复检。

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 2（均测试夹具/测试覆盖层面）/ 产品逻辑本轮零 MINOR 连续第五轮**。R66 两条 OBSERVE **复核成立**：OBSERVE-66-01（recoverMiddleware 对已提交响应后 panic 追加写 500 被 net/http 忽略）全仓当前确无该类现实路径；OBSERVE-66-02（Manager.SetVision 模板保留依赖 Client.SetVision 末端 else 分支）生产管线末端成立且既有护栏 `TestSetVisionKeepsLocalRecognizer`（client_test.go:231）确已在位——两条登记防御均无需动作，R66 结论核实无误。
- **唯一值得关注的动态 = flake 残余面首次自 R56 后回流 zhidao 包**：R1 全量轮 `TestLoginLogsFailureSummary` FAIL（7.04s，`mock 服务器就绪探测失败: http://127.0.0.1:50448`）= zhidao 包 `loginMockServer` 内 `readyProbe`（200ms×5）在 store 包 59s 高耗时（30050 行插入）结束后 mock accept 未就绪窗口内 **5 次全败** connectex——隔离 4 连全绿 + 5 连全绿 + 该测试单独 4 连全绿，确证冷启动残余非确定性缺陷。zhidao 包的 readyProbe 栅栏（200ms×5，无显式 2s 超时）比 api 包（200ms×10+2s 超时）窄，是残余面最低收敛点。
- 公约抽查 8 条（窗口关闭三判据单源 / 删号 memory-first 四步 / sameClientFor 六分支 / IsReadErr 四形态 / 重启恢复顺序 / doLogin 闸门 / 时钟兜底 / 引擎热切换模板）**全部成立**。跨轮修复闭合（httpDo 仅 dial/write 重试、cmd/probe SharedTransport、accounts readyProbe 三处、db 平台注释）**全部闭合**。

---

## R66 结论复核

### OBSERVE-66-01：recoverMiddleware 对已提交响应后 panic 追加写 500 —— 成立

核实结果：**成立（全仓无该类现实路径）**。逐项核对：
- `recoverMiddleware`（handler.go:1196-1209）在 `next.ServeHTTP(w, r)` panic 后无条件 `writeJSONStatus(w, 500, ...)`——若 panic 前响应已部分提交（writeJSON 已 `Encode` 触发 flush），第二次 `WriteHeader(500)` 被 net/http 忽略并记 `superfluous response.WriteHeader` 警告，客户端拿到的是第一层已提交响应。
- **全仓 handler 排除该类路径**（grep `w.Write`/`WriteHeader`/`WriteString`）：生产 handler 全部经 `writeJSON`/`writeJSONStatus` **单次**编码写，无一在 panic 前另写响应体。SpaHandler（embed.go）`fs.Stat` 确认存在后交给 `FileServer`（其内部不 panic），文件不存在路径 `w.Write(indexBytes)` 后无任何可能 panic 的代码（ReadFile 错误分支已先 404 return）。测试 mock 的 panic（TestRecoverMiddlewareHidesPanicDetail）是**零写入后 panic**，recover 正常 500。**核对无新的"写响应后 panic"现实路径**，OBSERVE-66-01 维持登记状态（防御观察，"已提交则仅记日志不追加写"的改进建议仍可选）。

### OBSERVE-66-02：accounts.Manager.SetVision 模板保留依赖 zhidao.Client.SetVision 末端 else 分支 —— 成立且护栏在位

核实结果：**成立（生产路径末端兜底 + 既有护栏已存在）**。逐项核对：
- `accounts.Manager.SetVision`（manager.go:191-199）`cfg.WithRecognizer(m.vision.Recognizer())` 保留模板当前引擎——引擎为 nil 时补的仍为 nil，模板层不承载最终语义。真正兜底的是 `zhidao.Client.SetVision`（client.go:167-177）：当前引擎非 Vision 且调用方未显式传引擎时 `else` 分支保留本地引擎。
- **既有护栏确认在位**：`TestSetVisionKeepsLocalRecognizer`（client_test.go:229-247）断言本地 ddddocr 引擎在 SetVision 传入零值 cfg 后仍保留（`cur != nil` + 类型断言 `localRecognizer`）——R66 主控核实的"护栏已存在"结论与源码一致。对偶测试 `TestSetVisionRebuildsWhenCurrentIsVisionOrNil`（:258-271）覆盖 nil/Vision 时按新配置重建。R66 建议的"可选补客户端级 else 分支断言护栏"已满足，无新缺口。

---

## 契约抽查（8 条，全部成立）

1. **契约 2 窗口关闭三判据单源**（scheduler.go:914-935 windowClosedLocked）：主判据 state.WindowClosed（probe:1142 带 +10s 裕量）/ syncFailStreak≥3 且开放时间已过 / EmptyProbeRuns≥3 且从未开窗且开放时间已过（:931）——三条判据用单次 open 快照（:920），StateForAccount（:708）与 WindowClosed()（:903）共用同一实现。**成立**（配套 TestStateForAccountMirrorsWindowClosed / TestGhostWindowEmptyProbesSuspend / TestGhostWindowClockFailuresSuspend 在位）。
2. **契约 4 删账号 memory-first 四步**（handler.go:1005-1019）：Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount——Remove 前置使在飞链锁内复核立即失败静默放弃。**成立**（TestDeletedAccountInFlightDropsSuccess / TestDeletedAccountManualInFlightDropsState 在位）。
3. **契约 21/31/37 sameClientFor 六分支**（scheduler.go:209-228 + spawnChain:1478-1664）：成功（1517）/ 失效（1485）/ 风控（1547）/ 窗口关闭（1567）/ 实时确证满员（1631）/ 实时失效（1596）六条 err 归并路径全部在写状态/落库前 `sameClientFor` 指针身份比对；`clientIdentity` 反射指针（nil 返回 0）。配套测试 12+ 条（TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull,RealtimeUnauthorizedDropsRelogin,SuccessDropsInflight}）全部在位。**成立**。
4. **契约 6 重启恢复顺序**（main.go:115-139）：RestoreDone → 循环 RestoreTargets → LoadRefused+RestoreRefused；RestoreTargets（scheduler.go:530-538）不清 refused，与 SetTargetsForAccount（:459-494 只清 refused 不清 done/full/rateLimited/inflight）分工清晰。**成立**（TestRefusedRestartOrderRealDB 用真实 SQLite 语义持久化复刻顺序契约）。
5. **契约 17 落库失败必须记日志**：全仓 grep 零 `_ =` 落库点——MarkDone/RemoveDone/spawnChain 全部 `if err != nil { log.Printf }`（含 reloginAt 写回 / DeleteRefusedClass / SaveSuccess / AppendLog 各处）。TestStoreFailuresLogged / TestSetTargetsDeleteRefusedFailureLogged 断言。**成立**。
6. **契约 33 B42-01 doLogin 全入口收口**：gateTryAcquire（manager.go:223-235）非阻塞准入 + LoginByPassword 前置（:244）+ 与 gateWait 共享 gateMu/gateUsed 计数 + ResetGateForTest 测试专用。api 夹具 authenticateDirect 批量注册处 3 处 ResetGateForTest（handler_test.go:363-367）对齐。**成立**（TestLoginByPasswordRejectsWhenGateBudgetExhausted 断言 doLogin 0 次）。
7. **契约 19/36 时钟兜底 / 删号 map 残留**：windowClosedLocked 判据 2 带"开放时间已过"（:921）+ `now.After(open)`；复位只清 clockOffset（:390）不清 streak（成功才归零 :396）；maybeRelogin 入口 ClientFor 存在性复核（:1204）后不写 map + goroutine 写回侧复核（:1250）。**成立**（TestGhostWindowClockFailuresSuspend 用真实 maybeSyncClock 累计 streak 到 3，非手动注入假绿）。
8. **契约 18/23 识别引擎热切换模板同步**：Manager.SetRecognizer 写模板（:212）+ SetVision 保留引擎（:194）+ Client.SetVision else 分支（client.go:172-175）三层；TestNewClientAfterSetRecognizerGetsEngine（manager_test.go:233-275）+ 双护栏（见 R66-②）在位。**成立**。

---

## 新发现

### OBSERVE-67-01：zhidao 包 `readyProbe` 栅栏窄于 api 包（200ms×5 无显式超时），flake 残余面自 R56 后首次回流 zhidao 包

- 位置：`backend/internal/zhidao/client_test.go:85-103`（readyProbe）+ `loginMockServer`（:50-80）。
- 一句话问题：R1 全量轮 `TestLoginLogsFailureSummary` FAIL（7.04s）——`mock 服务器就绪探测失败: http://127.0.0.1:50448`，即 `loginMockServer` 内 `readyProbe(t, srv.URL)` 的 **200ms×5 轮询全败**（connectex），mock server accept 未就绪窗口未被排空。全量 `-p 1` 下 zhidao 是 store 包（59s 高耗时、30050 行插入产生海量 TIME_WAIT）之后的最末 mock 包，store 结束时的冷启动窗口压过了 zhidao 的 5 次重试栅栏。
- 影响面：测试 flake（全量 5 轮 1 命中，隔离 4 连全绿 + 单独 4 连全绿 + 5 连全绿证明非确定性缺陷）；产品零影响。残余面历史：R56 全量 zhidao 12 轮 FAIL 根因（socketPreheat+TestMain 修复）→ R64 R4 TestCaptchaConcurrency 单命中 → **R67 R1 TestLoginLogsFailureSummary 再命中**。zhidao 包 readyProbe 是残余面最低收敛点（api 包 200ms×10+2s 显式超时，zhidao/accounts 均 200ms×5）。
- 机制说明：http.Get 首请求 connectex → 轮询 200ms 间隔 5 次全败（总窗口 ~1s）→ Fatal。冷启动窗口在 store 包 59s 高耗时末尾最恶劣，1s 轮询窗口不足以覆盖重链表的 TIME_WAIT 排空。
- 建议：夹具层面将 zhidao 包 `readyProbe` 对齐 api 包（200ms×10 + 显式 2s 超时的 http.Client），或在 `loginMockServer` 构造后额外调用一次 `socketPreheat()`（与 api 包"套接字预创建 + 就绪前移"双保险对齐）。遵 CI `||` 重跑兜底语义。非产品缺陷，可续排期。

### OBSERVE-67-02：`TestLocalDdddOcrAvailable`（zhidao 包）恒绿死测试——三种形态均无失败断言

- 位置：`backend/internal/zhidao/local_ocr_test.go:10-18`。
- 一句话问题：`available := LocalDdddOcrAvailable("")` 后——①available=true → 直接返回通过；②available=false 且 `exec.LookPath("python")` 失败 → skip；③available=false 且 python 存在（装了 python 但无 ddddocr 的部署机）→ 走到函数结尾无任何断言，照常通过。三种形态测试恒 PASS，对 `LocalDdddOcrAvailable` 的实现（exec.Command("python","-c","import ddddocr").Run()==nil）零验证力。
- 影响面：纯测试覆盖缺口（无功能影响）。该函数是 `applyCaptchaRecognizerFor` 的 ddddocr 分支探测入口（router.go:40）——在装了 Python 但没装 ddddocr 的部署机上应回退 Vision，当前无测试钉死该分支。
- 机制说明：第 ③ 态是"该函数应返回 false 而测试却视为通过"的盲区；函数本身实现正确（exec 探测），问题只在测试恒绿。
- 建议：可将断言改为 `if available && python exists { t.Error("装了 python 无 ddddocr 时 LocalDdddOcrAvailable 应返回 false") }` 或在 available 时真正断言 true（此时该环境确有 ddddocr）——使测试具备任一方向的验证力。可续（低优先）。

---

## flake 统计（R67 全量 5 轮 + mock-heavy 5 包 4 轮补充）

| 轮次 | 结果 | 命中 |
|---|---|---|
| 全量 R1 | **FAIL**（zhidao 11.69s） | `TestLoginLogsFailureSummary`：`mock 服务器就绪探测失败: http://127.0.0.1:50448`（readyProbe 5 次全败 connectex） |
| 全量 R2 | ok | zhidao 2.83s / store 76.6s / api 23.4s 全绿 |
| 全量 R3 | ok | zhidao 3.32s / api 37.1s / store 53.5s 全绿 |
| 全量 R4 | ok | zhidao 3.09s / api 23.6s / store 33.3s 全绿 |
| 全量 R5 | ok | zhidao 3.14s / api 25.5s / store 43.0s 全绿 |
| 5 包串行 R1 | ok | — |
| 5 包串行 R2 | ok | — |
| 5 包串行 R3 | ok | — |
| 5 包串行 R4 | ok | — |

- **5 轮全量 1 FAIL（zhidao 包首现）**，隔离复跑 `TestLoginLogsFailureSummary` **4 连全绿**（1.78s~2.3s）+ 两日志测试 5 连全绿 + 四测试复跑全绿——确证冷启动残余。zhidao 包自 R56 后残余面归零，R64 曾单命中（TestCaptchaConcurrency），本轮再命中（TestLoginLogsFailureSummary）。
- **总体趋势**：R56 12 轮 zhidao FAIL → R64 22/24 → R65 8/8 → R66 4/4 全绿 → R67 5 轮 1 命中。残余面再次证伪"根除"幻觉——本轮流动宿主 = store 包之后最末 mock 包 zhidao（readyProbe 栅栏 200ms×5 最窄），与 R64 预估的 api 首包首测试动态不同。**CI `||` 重跑 + 栅栏加宽是永久防线，不可移除**。
- build/vet/gofmt：`go build ./...` / `go vet ./...` / `gofmt -l .` 全部零输出。

---

## 历轮观察延续

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-66-01（recover 双层响应防御） | 登记防御 | 全仓仍无该类路径（handler 全单次 writeJSON/writeJSONStatus、SpaHandler 无 panic 后写路径），新增检查无新的现实路径 | **延续（登记防御）** |
| OBSERVE-66-02（SetVision 末端 else 兜底） | 登记闭合（护栏已存在） | TestSetVisionKeepsLocalRecognizer 在位 + R66 主控核实无新缺口 | **闭合（延续登记）** |
| OBSERVE-65-01（db 平台注释精准化） | 已修（fc6e73b） | settings_test.go:12-17 注释与 Linux 建库机制逐段一致，跨平台红白语义明确 | **闭合** |
| OBSERVE-65-02/03（accounts READYPROBE） | 已修/转注释声明 | 3 处 readyProbe 在位（:80/157/239），accounts 包 5 轮全绿零 FAIL | **闭合** |
| OBSERVE-64-01（probeIntervalFor 注释 5s→2s） | 已修 | 全仓无"临门 5s"残留（nearWindow=5 分钟为独立常量语义） | **闭合** |
| OBSERVE-63-01（api ReadErr/UnauthorizedRelogin 冷启动） | 已闭合 | 本轮 api 包 5 轮全绿零命中 | **延续（消失）** |
| OBSERVE-63-03（api 包最脆弱） | 已细化 | 残余面本轮流动到 zhidao 包（见 OBSERVE-67-01）——"最脆弱包"标签随 R56 后收敛已不固定 | **更新（zhidao 包接棒）** |
| OBSERVE-64-02（accounts 唯一无冷启动前移） | 已修（readyProbe 三处） | accounts 包 5 轮全量全绿 | **闭合** |
| OBSERVE-63-04（probe 非可用工具） | 延续 | 未变（token 需手动注入 + SharedTransport 已对齐契约 R63 闭合） | **延续** |
| OBSERVE-63-05（AdminStats 半真测试） | 延续 | 未变（failingTargetsStore 定义未用 + Register 具体类型） | **延续** |
| OBSERVE-62-06/07、OBSERVE-61-03/04/06/07/08 | 延续 | 均未变 | **延续** |

---

## 已核对无缺陷的高风险区域

- 调度器 tick 守卫族（零值守卫放行 WindowOpened 例外 B41-02 / windowClosedLocked 三判据 / 250ms 冲刺 / 提交节流对齐钟 / spawnChain 六分支身份防线 / relogin 决策与写回复核 + 指数退避封顶 10m / probeSem cap4 与 probing 单飞）。
- 认证鉴权（requireAdminSession/requireAuth + CSRF requireJSONBody + 管理员双条件 B43-04 + 登录/激活双限流桶 + clientIP 可信反代 XFF + 会话 12h TTL + 删号 RevokeAccount）。
- 凭据加密（AES-256-GCM + 32 字节主密钥 XUANKE_MASTER_KEY 或 .master_key + vision_key enc: 前缀 + maskKey 脱敏 + secureEncrypt 无加密器拒绝）。
- 数据库（migrateAddPublishMeta 先于 refuseLegacy + 缺列清单与已迁移列对齐 + 激活码单事务原子扣减 + 空库日志窗口语义 window_empty_test）。
- 登录链路（RSA PKCS1 + uniqueDeviceID + priorityId 空串 + fetchLoginPage 抖动自愈 + httpDo 仅 dial/write 重试 + IsReadErr 四形态与 isConnErrRetryable 互斥 + Vision 识别 3~5 位净化）。
- cmd 三工具（probe SharedTransport / logintest -limit 边界 + 40s 间隔 / bench -token 注入 + -n 边界）。

---

## 教训

1. **"残余面消失"的标签不能一劳永逸**：zhidao 包自 R56 socketPreheat+TestMain 修复后全量 0 FAIL 长达 10+ 轮，本轮 R1 又现——残余面是"包序最后一位最末 mock 包 + 最窄 readyProbe 栅栏"的函数，store 包 59s 高耗时（30050 行插入）在每轮全量末尾制造最恶劣 TIME_WAIT 窗口。修复方向不是再给它打补丁式特殊处理，而是把 zhidao/accounts 的 readyProbe 统一对齐 api 包 200ms×10+2s 超时的宽栅栏（夹具一次性配平，杜绝包间栅栏宽度差异）。
2. **"恒绿死测试"要在覆盖审计中专门排雷**：`TestLocalDdddOcrAvailable` 三态全过、零断言——这类"因环境检测跳过而恒通过"的测试是最隐蔽的无验证力代码，人工走读全包时极易顺读放行。判断标准：测试末端是否有可达的失败断言（skip 分支之外）。
3. **R66 结论复核的取证要"源码 + 既有护栏"双锚**：OBSERVE-66-02 的"末端兜底成立"仅凭源码走读会漏掉"护栏早已存在"的事实——对照既有测试（TestSetVisionKeepsLocalRecognizer）确认后再下"闭合"裁决，避免报告开空头支票。

## 发现摘要（给主控）

- **OBSERVE-67-01**：zhidao 包 readyProbe 栅栏窄（200ms×5）+ flake 残余面自 R56 后首次回流（全量 R1 `TestLoginLogsFailureSummary` readyProbe 5 次全败 connectex；隔离 4+5 连全绿确证非缺陷）。裁决：续（测试夹具收敛项，建议对齐 api 包 200ms×10+2s 宽栅栏）。
- **OBSERVE-67-02**：`TestLocalDdddOcrAvailable` 三态恒绿死测试（available=false+python 存在时走到结尾无断言）。裁决：续（低优，补任一方向断言使具验证力）。
- 无 CRITICAL / MAJOR / MINOR。R66 两条 OBSERVE 均复核成立、无需动作。