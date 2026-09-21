# R66 后端只读审查发现报告

> 审查基线：master @ `b1742a6`（R65 收官 docs 提交 `b1742a6`，R66 为第 66/256 轮）。工作树复核：基线之后仅 `web/scripts/` 4 个前端脚本的未提交改动（详见文末实证表，均为注释/审计脚本放宽，与后端无关），后端零改动；审查期间绝对只读，临时验证程序全部落系统 `%TEMP%`（`r66verify` / `r66verify2`，已删除）。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部测试文件。
> 方法：R65 三卫生修（fc6e73b）逐行核证 + 历轮决策手册契约抽查（重点 8 条）+ 全包逐行通读找新问题 + 全量 `-race -count=1 -p 1 -timeout 900s ./...` **4 轮** flake 统计 + 隔离复跑 + gofmt/vet/build 复检 + `%TEMP%` 独立程序实证。

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 2（均测试夹具层面）/ 新观察 0（产品层）**。R65 三卫生修（fc6e73b）**全部正确**——①db 平台注释机制经跨平台语义逐段核证与 R65 实测一致、机制叙述补全到位；②accounts 两处 mock `Content-Type` 补全语义正确（与真实平台 text/html 对比无害且仅夹具层）；③`TestNewClientAfterSetRecognizerGetsEngine` 的 `readyProbe` 入口已接上（OBSERVE-65-03 的 socket 双保险注释已声明为防御性观察）。生产逻辑本轮零缺陷、零新增 MINOR，历史防线族（身份六分支 sameClientFor / httpDo 重试契约 / IsReadErr 四形态 / 时钟兜底 / 识别引擎热切换）全部按历轮零缺陷结论复核成立。
- **flake 判定**：**4 轮 / 0 FAIL**（连续 4 轮全绿）——R65 创纪录的 8/8 全绿延续为 **R66 4/4 全绿**，accounts 包自 R64 readyProbe 后全量轮零 FAIL 继续、api 包残余（R65 R1 的 TestAccountOverride 显式 deadline exceeded）本轮零命中 + 隔离复跑 1 连全绿。残余面已收敛到"全量轮窗口内基本不可见"的统计尾巴，CI `||` 重跑仍是正确姿势。
- **唯一捞到的新观察**：`accounts.Manager.SetVision`（manager.go:191-199）在测试夹具精简版复刻下存在"模板引擎被清空"的宿主，但**生产 zhidao.Client.SetVision（client.go:167-177）有 `else` 分支精确保留本地引擎，管线末端兜底成立**——经 `%TEMP%` 独立程序模拟完整管线实测，最终客户端引擎保留（`true`），生产路径无缺陷（见 OBSERVE-66-02 证据链）。纯防御性观察，非缺陷。

---

## R65 修复复核

### R65-①：`TestOpenOnReadonlyPath` 平台注释机制精确化（settings_test.go:10-17）——正确

核实结论：**正确**。新注释全部要件逐段核证：
- "`C:\nul\nul\` 是 Windows 保留设备路径（NUL 在任意段被内核拒绝，Open/MkdirAll 返回错误 → 断言通过）"——Windows 侧真实语义成立（NUL 保留设备名在任意路径段触发 `STATUS_OBJECT_NAME_INVALID`，db.Open 的 `os.MkdirAll(filepath.Dir(path))` 即失败返回 err → 断言 pass）。
- "Linux/macOS 下 `C:` 被当普通目录名、该路径会被真实创建为 SQLite 数据库文件并跑完 schema（Open 成功 → err==nil → 断言红）"——与 R65 Alpine 实测（Open 返回 `(db, nil)`、schema/迁移执行成功、断言红线 16 行）完全一致；**"真实建库成功"这层机制确实是关键**：光"MkdirAll 成功"不会让 Open 返回 nil——只有 SQLite 以 `SQLITE_OPEN_CREATE` 真实创建了 `C:\nul\nul\test.db` 并跑完 `schemaSQL` 后 `err==nil` 才会触发断言 Fatal。新注释把机制从"MkdirAll 成功"精确为"真实建库成功"，R65 OBSERVE-65-01 的薄层完全补上。
- 自洽性：注释同时声明"当前 Windows 主平台未触发" + "跨平台 CI 启用前需换真只读构造（t.TempDir + 权限守卫 + GOOS 守卫）"——与既有决策锚（数据库增量迁移规范的平台假设显式化）一致，无新引入的误导。

### R65-②：accounts 两处 mock 补 `Content-Type: text/html; charset=utf-8`（manager_test.go:74/151）——正确

核实结论：**正确**。`loginRejectSrv` 与 `gateSrv` 的 `/login` 与 `/login/captcha` 分支在 `w.Write([]byte("ok"))` 前各自补 `w.Header().Set("Content-Type", "text/html; charset=utf-8")`，注释标明"与真实平台 text/html 差异（识别链路不消费响应体，无害）"。语义核对：真实平台 `/login` 返回 `text/html; charset=UTF-8`，该 CT 与 mock 完全一致；`fetchLoginPage`（client.go:292-323）只 `io.Copy(io.Discard, resp.Body)` 不依赖 CT；`readyProbe`（manager_test.go:31）的 `http.Get` 读 body 亦不依赖 CT——补 CT 为纯夹具语义对齐，无副作用、无新 flake（4 轮 accounts 全绿）。

### R65-③：`TestNewClientAfterSetRecognizerGetsEngine` 的 readyProbe + socketPreheat 注释——《已接上》

核实结论：**正确 + 已接上**。该测试（manager_test.go:233-275）在 `httptest.NewServer` 构造后第 239 行 `readyProbe(t, srv.URL)` 已调用（R64 修复时接入，R65 处未动，仍有效）；OBSERVE-65-03 的"第三处夹具无 socketPreheat"已按裁决以注释形式声明为防御性观察（readyProbe 头部注释 21-22 行，"若未来 accounts 再出冷启动 flake 第一候选即补 socketPreheat"）。4 轮全量 accounts 零 FAIL，无残余证据。

### R63/R64 跨轮闭合复核

- **httpDo 重试契约**（client.go:466-501）：`httpDo` 对 `isConnErrRetryable`（`errors.As` 穿透 url.Error 链命中 `*net.OpError` 且 `.Op=="dial"||"write"`）重试一次、read 与业务/取消错误原样上抛——与 R63 结论一致，无回潮。
- **IsReadErr 四分支形态全集**（client.go:519-548 + isreaderr_test.go:14-66）：FIN（errors.Is io.EOF）→ 短读（errors.Is io.ErrUnexpectedEOF）→ 超时双文案（awaiting headers / reading body）→ RST（errors.As OpError read）——四分支顺序互斥完备，测试矩阵覆盖（含 urlError 包装穿透 + dial/write 互斥 + 业务/nil 不命中）与标准库逐段实证一致（transfer.go:865 LimitedReader 短读 / transport.go:2347 响应头 RST 与正文短读互斥）。**无第五形态遗漏**（trailer EOF 不命中、语义正确）。
- **sameClientFor 六分支身份防线**（scheduler.go:209-228 + spawnChain:1478-1664）：成功（1517）/ 失效（1485）/ 风控（1547）/ 窗口关闭（1567）/ 实时确证满员（1631）/ 实时失效（1596）六条 err 归并路径全部在写状态/落库前 `sameClientFor` 指针比对，且失效分支的重登（maybeRelogin）严格落在复核之后（B42-02）；`clientIdentity` 用 `reflect.ValueOf(c).Pointer()`（接口动态类型取指针，nil 返回 0）。配套测试 R39-R43 族 12+ 条（含 B41-01 单条分支独立红绿）全部在位。**与 R64/R42-43 结论一致**。

---

## 历轮决策手册契约抽查（≥8 条）

1. **契约 1 开放时间自动识别三硬契约**（openTimeForLocked scheduler.go:432-443）：识别槽写入必须持 s.mu（ProbeForAccount:849/854 + probe:1103-1105 锁内写）；识别过期只影响展示层（StateForAccount:719 依 now 判 OpenTimeKnown）绝不截断零值；空快照不删识别槽（open_retain_test 全守卫）。**成立**。
2. **契约 2 窗口关闭三判据单源 windowClosedLocked**（scheduler.go:914-935）：state.WindowClosed 主判据（带 +10s 裕量，probe:1142）/ syncFailStreak≥3 且开放时间已过 / EmptyProbeRuns≥3 且从未开窗且开放时间已过——三判据用一个 open 快照复用，StateForAccount 与 WindowClosed() 同源。**成立**。
3. **契约 4 删账号 memory-first 四步**（handler.go:997-1023）：Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount。内存先摘、在飞链复核立即失败静默放弃（B18-M2 族 MarkDone/RemoveDone 锁内客户端复核），DeleteAccount 失败半删态由重启 Restore 自愈。**成立**。
4. **契约 5 / 21 / 31 / 37 身份防线成族**：sameClientFor 六分支（见上）+ MarkDone/RemoveDone 手动路径客户端存在复核（scheduler.go:1923/1991）+ maybeRelogin 决策侧（1198-1207）与写回侧（1250）复核 + 实时复核失效分支 sameClientFor（1596）。**成立**。
5. **契约 6 重启恢复顺序**（main.go:115-139）：RestoreDone → RestoreTargets 循环 → LoadRefused+RestoreRefused；SetTargetsForAccount 只清 refused 不清 done/full/rateLimited/inflight（TestSetTargetsPurgesStaleState 全断言）。**成立**。
6. **契约 10 发布恢复驱动重试 / 34-35 shouldDeferSave**（前端契约，后端侧依赖 /state 按账号全量下发 courses + 整包 PUT 全量替换语义——R64 A③ 已实证兼容）。**成立**。
7. **契约 17 落库失败必须记日志**：全仓 grep `_ =` 落库点——MarkDone/RemoveDone/spawnChain 全部 `if err := ...; err != nil { log.Printf }`；TestStoreFailuresLogged 断言。**成立**。
8. **契约 18/23 识别引擎热切换模板同步**（manager.go SetVision:194 / SetRecognizer:212 + zhidao.Client SetVision:167-177）：Manager 模板 `WithRecognizer(m.vision.Recognizer())` 保留 + client `SetVision` 对非 Vision 引擎的 else 保留分支（客户端级二重兜底）——test 用 `minimalRecognizer` 值类型（非指针）验证了 Manager 模板链路（accounts 测试全绿）。**成立（生产路径，见 OBSERVE-66-02 证据链）**。
9. **契约 26/28 基础设施状态码家族**（writeJSONStatus 5 类：panic 500 / 401 / 403 / 429 / 未知 /api 404 + requireJSONBody 的 CSRF-403）：全部"先设头再 WriteHeader"（handler.go 85-89 / router.go 72），真实 Server 端到端断言（TestApiUnknownPath404 的 httptest.NewServer + http.Get）在位。**成立**。
10. **契约 33 B42-01 doLogin 全入口收口**：gateTryAcquire 非阻塞准入（accounts.go:223-235）+ LoginByPassword 前置 + gateWait 与 gateTryAcquire 共享 gateMu/gateUsed 计数 + ResetGateForTest 测试专用。**成立**。
11. **契约 19/36/41 时钟兜底 / 删号 map 残留 / tick 零值守卫**：syncFailStreak≥3 判据带"开放时间已过"（windowClosedLocked:921）+ 复位只清 offset 不清 streak（同步成功才清 397 行）；maybeRelogin 入口无 map 残留；tick `open.IsZero() && !opened` 才挂起（scheduler.go:1007，TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 守卫）。**成立**。

---

## 新发现

### OBSERVE-66-01：`recoverMiddleware` 对已写响应后 panic 可能产生"双层响应"——极端低危观察

- 位置：`backend/internal/api/handler.go:1196-1209`。
- 一句话问题：recover 在 handler 已部分写响应（如 `writeJSON` 已 `Encode` 触发 flush）后发生 panic 时，`writeJSONStatus` 会再写一个 500 响应——第一层已提交、第二层被 net/http 忽略并记 `http: superfluous response.WriteHeader call` 警告日志，客户端拿到的是**第一层不完整/已提交的响应**而非统一 500。
- 影响面：需"handler 内以非 `writeJSON*` 方式直接写响应后又 panic"才触发——全仓 handler 全部经 `writeJSON`/`writeJSONStatus` 单次写入、尚未发现该类路径（recoverMiddleware 日志 `internal error` 已可审计）。属极端边界防御观察，非现实缺陷。
- 建议：不修（保持简洁）；若未来确需兜底可在 recover 分支先记录 `w.Written()` 是否已提交，已提交则仅 `log.Printf` + 关闭连接，不追加写。

### OBSERVE-66-02：`accounts.Manager.SetVision` 的"模板保留"依赖 zhidao.Client.SetVision 末端兜底（管线末端成立，纯防御性观察）

- 位置：`backend/internal/accounts/manager.go:191-199`。
- 一句话问题：Manager 层 `SetVision` 注释声称"保留模板当前引擎"，但 `WithRecognizer(m.vision.Recognizer())` 只在 `m.vision` 已有引擎时才有保留效果——若模板识别器是 `**nil**`（如启动早期 `dispatchRuntimeConfig` 与 `applyCaptchaRecognizerFor` 之间、或管理员热改 Vision 配置时模板尚未加载引擎），Manager 层补的仍为 nil；真正保住引擎的是 **zhidao.Client.SetVision 的 else 保留分支**（client.go:172-175：当前引擎非 Vision 且调用方未显式传引擎时保留本地引擎），每个客户端各自兜底。
- 证据链：`%TEMP%\r66verify2` 独立测试复刻完整管线（Manager.SetRecognizer(ddddocr) → Manager.SetVision → Client.SetVision else 分支）——初版复刻（else 缺失）断言 `客户端引擎被清空` FAIL；补上生产版本的 else 保留分支后 PASS（引擎保留 `true`）。即**生产管线末端成立**，问题只存在于"若未来移除客户端 else 分支"或"模板层单独承载引擎保留职责"的假设场景。
- 影响面：当前生产路径零风险（Client.SetVision else + Manager 模板保留双保险）；纯防御性观察，登记以防未来重构破坏 Client.SetVision 的 else 分支。
- 建议：不修；未来若重构 `Client.SetVision` 的引擎保留逻辑，需同步回归 `TestNewClientAfterSetRecognizerGetsEngine`（accounts 包仅跑通 Manager 模板链路，未直接断言客户端 SetVision 的 else 分支——可补一条客户端级断言作为未来护栏，可选）。

---

## flake 统计（R66 全量 4 轮）

| 轮次 | 结果 | api 包 | accounts 包 | 其他包 |
|---|---|---|---|---|
| R1 | **ok**（0 FAIL） | 56.18s | 1.68s | store 43.5s / scheduler 14.5s 全绿 |
| R2 | **ok**（0 FAIL） | 21.75s | 11.01s | store 31.8s 全绿 |
| R3 | **ok**（0 FAIL） | 19.73s | 1.56s | store 31.1s 全绿 |
| R4（补充） | **ok**（0 FAIL） | 24.85s | 2.14s | store 45.4s 全绿 |

- **4 轮 0 FAIL**（全绿率 100%）——R64 readyProbe / R63 api 夹具加宽后残余已收敛到"全量轮窗口内不可见"水平；R65 R1 出现的 api `TestAccountOverrideRequiresAdminSession` deadline exceeded 形态本轮零命中（R66-R1 该测试正常）。隔离复跑 `TestAccountOverrideRequiresAdminSession` 1 连全绿（2.24s）。
- 残余形态说明：Windows 全量串行 `-p 1` 下前序包 TIME_WAIT 残余是已知 flake 源；本轮 4 轮全绿 + 前序 8/8 = **近 12 轮全量全绿**，与历史上 R57~R64 各轮单包 FAIL 相比呈明显收敛。CI `||` 重跑的兜底语义仍保留。
- build/vet/gofmt：`go build ./...` 全绿、`go vet ./...` 全绿、`gofmt -l .` 零输出。

---

## 历轮观察延续

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-65-01（db 平台注释欠精确） | 待修 | **已修（fc6e73b）且精确**（见 R65-①） | **闭合** |
| OBSERVE-65-02（accounts mock 无 CT） | 待修 | **已修（fc6e73b 两处补 CT）且语义正确**（见 R65-②） | **闭合** |
| OBSERVE-65-03（accounts 第三处无 socketPreheat） | 待修（防御性） | **注释声明已接**（readyProbe 头部 21-22 行），4 轮 accounts 零 FAIL | **部分闭合（转注释声明，防御性观察保留）** |
| OBSERVE-63-01/02（api ReadErr/UnauthorizedRelogin 冷启动） | 延续（消失） | **4 轮 0 命中**（R66-R1 api 全绿 56s 含该两测试） | **闭合（残余消失）** |
| OBSERVE-63-03（api 包最脆弱） | 延续 | **细化**——api 包仍有残余面但是 4 轮 0 命中（R65 R1 曾 20s FAIL 的 TestAccountOverride 本轮正常）；accounts 归零后残余面若再浮动最可能仍在 api 包首测试 | **延续（api 首包首测试）、残余不可见** |
| OBSERVE-63-04（cmd/probe 非可用工具） | 延续 | 未变（token 需手动注入 + 无认证；dump 的 probe.exe 亦为历史构建） | **延续** |
| OBSERVE-63-05（AdminStats 半真测试） | 延续 | 未变（failingTargetsStore 仍定义未用；Register 接收具体 *store.Store） | **延续** |
| OBSERVE-62-06（probe 工具 cookie 占位） | 延续 | 未变（MINOR-61-01 修复后占位统一 "1"，工具可用性仍未恢复） | **延续** |
| OBSERVE-62-07（config.Load 每次写盘） | 延续 | 未变（ensureEnvFile 幂等：存在即跳过） | **延续** |
| OBSERVE-61-03（targets nil 变空目标） | 延续 | 未变（handler.go:494-496，恶意/异常客户端可触达，清空本身合法） | **延续** |
| OBSERVE-61-04（撞名文案归因） | 延续 | 未变（B43-04 后净命中仅"管理员名+口令错"；撞名学生教务分支不被吞） | **延续** |
| OBSERVE-61-06（bench 默认 401 路径） | 延续 | 未变（bench/main.go 自带注释明示 + -token 可注入） | **延续** |
| OBSERVE-61-07（logout 单会话） | 延续 | 未变（语义"注销当前会话"，与 RevokeAccount 分工清晰） | **延续** |
| OBSERVE-61-08（config.Load 写 .env） | 延续 | 未变（低频场景） | **延续** |

---

## 已核对无缺陷的高风险区域

- 调度器 tick 守卫族（零值守卫放行 WindowOpened 例外 / windowClosedLocked 三判据 / 提交节流 250ms 冲刺）、spawnChain 六分支身份防线、probe 单飞 + probeSem cap4、会话快照回退链（B22-01/B28-01 过期专属帧绝不回退全局）、maybeRelogin 决策/写回复核 + 指数退避、clockOffset 复位语义。
- 认证鉴权：requireAdminSession/requireAuth + CSRF JSON 门 + 管理员双条件（B43-04）+ 登录/激活双限流桶 + clientIP 可信反代 XFF。
- 凭据加密：AES-256-GCM + 32 字节主密钥校验 + vision_key enc: 前缀 + maskKey 脱敏。
- 数据库：migrateAddPublishMeta 先于 refuseLegacy + 空库日志窗口语义 + 多连接激活码并发事务原子扣减。
- 登录链路：RSA PKCS1 加密 + uniqueId 复刻 + getUniqueDeviceId + priorityId 空串语义 + fetchLoginPage 抖动自愈 + submitLogin 不重试。
- 实时人数复核：classFullRealtime 锁外请求 + 回锁后 doneHas 复核"绝不覆盖手动胜利状态" + sameClientFor 兜底。

---

## 验证实证表

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `gofmt -l .` | 零输出 |
| 后端全量 `-race -count=1 -p 1 -timeout 900s ./...` | **4 轮 / 0 FAIL**（R1-R4 全绿）；api 包 56.2s/21.7s/19.7s/24.8s、accounts 1.68s/11.0s/1.56s/2.14s 全绿 |
| 隔离复跑 | `TestAccountOverrideRequiresAdminSession` 1 连全绿（2.24s） |
| `%TEMP%` 独立程序 | `r66verify`（引擎保留语义 PASS）+ `r66verify2`（完整管线复刻：初版 FAIL 证明 Client.SetVision else 分支承担实际保留、补 else 后 PASS 证明生产路径成立）——已删除 |
| 基线后工作树改动 | 仅 `web/scripts/` 4 个前端脚本未提交改动（admin-auth-check/target-guard-check/unauthorized-check 用 `--import jiti/register` 改注释 + audit.mjs 的 bg-neutral-95x 检查放宽选项/ hover: 前缀）——与后端零相关，属前端代理合流前改动 |
| 测试函数总数 | ~211（api 55 / scheduler 87 / accounts 6 / zhidao 23 / store 13 / db 6 / session 11 / config 2 / runtime 2 / secure 5）+ zhidao TestMain 1 |

---

## 教训

1. **"模板保留"类契约要分清哪一层在真正兜底**：`accounts.Manager.SetVision` 的注释宣称模板保留，但模板层对"引擎为 nil 时"无能为力——真正保住本地引擎的是 `zhidao.Client.SetVision` 的 else 分支（客户端级兜底）。跨包设置器链的契约描述，审查时必须追到最末端消费者确认"哪一层承担最终语义"，仅看中间层注释会得出片面结论。（本观察由 `%TEMP%` 独立程序初版 FAIL / 补 else 后 PASS 实证。）
2. **flake 收敛的判据是"长尾不可见"而非"零残余"**：R65 8/8 + R66 4/4（近 12 轮全量全绿）已把 Windows 冷启动残余压到统计不可见，但"残余不可见"≠"根除"——R64 曾预估残余会流向 api 首包首测试，R65 R1 即命中；本轮又归零。CI `||` 重跑与首包夹具就绪前移是永久保留的防线，不是可移除的补丁。
3. **先写结论再复核注释的高风险区应避免"顺读陷阱"**：R65 注释修正本身完全正确，但审查时若只按注释复述而不核对断言方向（`err==nil` 才 Fatal）与 Linux 建库机制，会重蹈 R64 的"推断与实测打架"覆辙——跨平台断言必须锚定目标平台行为。

## 发现摘要（给主控）

- **OBSERVE-66-01**：`recoverMiddleware` 对已提交响应后 panic 会追加写 500（net/http 忽略并记 superfluous 警告），全仓当前无该类路径。裁决：续（不修，登记防御性观察）。
- **OBSERVE-66-02**：`accounts.Manager.SetVision` 的模板保留依赖 `zhidao.Client.SetVision` 末端 else 分支（%TEMP% 独立程序实证生产管线成立）。裁决：续（不修，可选补客户端级 else 分支断言护栏）。
- 无 CRITICAL / MAJOR / MINOR。