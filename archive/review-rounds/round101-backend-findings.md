# R101 后端只读审查报告

审查对象：xuanke-auto HEAD `16d1aec`（R100 里程碑轮已收官，本轮为 R101 双 opus 审查首轮）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round101-backend-findings.md）。并行前端代理产出 round101-frontend-findings.md。

审查方式：Read / Grep / Glob / Bash 只读命令（定向 go test -race / go build / go vet / gofmt / 实测 url.Error 行为 + 逐点走读）。核心走读范围：backend/internal/{scheduler(全量),api(全量),zhidao(全量),accounts,store,db,session,config,runtime,secure} + cmd 三工具 + tray 双文件 + quit_shared.go + main.go。新契约角度本轮聚焦：**多账号并发提交的全局对称性（sessionAccount 强制会话绑定下账号路由无旁路）+ 日志脱敏完整性（token/password 只显前 8 位契约的全路径覆盖，含错误包装链）**，附带评估 SQLite 单写者锁内网络慢操作边界与 SyncServerTime 对齐时钟遗漏分支。

## CRITICAL

无。

## MAJOR

无。

## MINOR

### B101-01：doRequest 网络层错误文本携带完整 idToken（`?idToken=<全量 token>`），经错误包装链进入磁盘日志/库内 task_log/前端回显——token 脱敏契约路径缺口（实测 + 走读）

**证据**：`doRequest`（client.go:422）拼 `c.baseURL + path + "?idToken=" + url.QueryEscape(tok)` 构造请求 URL；连接层失败（dial/write connectex / 超时）时标准库 `http.Client.Do` 返回的 `*url.Error` **文本包含完整请求 URL**（本轮实测：`Post "http://127.0.0.1:1/electives/select/findElectivesData?idToken=12345678901234567890": dial tcp ... connectex: ...`，url.Error 的 Error() 直接回放 request.URL.String()）。`httpDo`（client.go:474-488）只对 retryable 连接错误自愈重试一次，重试仍失败时把 `*url.Error` **原样上抛**（client.go:485）。该错误随后进入三条消费链：

1. **进程日志**：scheduler.go:1098 `log.Printf("查询课程失败: %v", err)`（probe 首路径）/ :1294 `自动重登失败: %v`（Relogin 失败包装）/ :1556 风控分支 `+err.Error()`（风控文案来自平台不泄露，此处 err 多为业务错误不携 URL，但连接层超时形态同入口）。
2. **task_log 落库 + 前端报名日志回显**：spawnChain 非归类错误分支 scheduler.go:1654-1655 `failMsg := err.Error()` / `logMsg := "账号 " + acct + ": " + err.Error()` → `setStateLocked`（/api/state 回显）+ `AppendLog`（/api/logs 与 /api/admin/logs 回显）。
3. **手动报名/退选响应**：handler.go:360 / :426 `writeJSON(w, 1, nil, err.Error())` → 前端弹窗/toast 即时回显完整 token。

**与既有脱敏契约的关系**：契约族已覆盖 maskedToken（scheduler.go:1334 前 8 位）、tokenShort（manager.go:319 `***` / 前 8 位）、Vision 识别原文脱敏（captcha.go:206）、vision_key 回显脱敏（maskKey）、错误文案用户化（read 类/ErrUnauthorized 分支均已替换为不含 token 的文案）。**唯独"非 ErrUnauthorized 非 read 的连接层网络错误"分支把 err.Error() 原文放行**——URL 参数通道（`idToken=`）恰好是 token 的权威载体，故泄露的是完整会话凭证本身。

**前置条件评估**：dial/write 失败（平台连不上 / 学校网络抖动 / Windows 回环 keep-alive 活性）才触发，黄金期瞬间最常见；错误在不同消费点频繁度不同（日志 100% 落、库内 appendLog 每题一次、前端手工回显要求"手动报名恰好撞网络失败"）。泄露面受众 = 本机/服务器磁盘日志阅读者（单机部署即本人 / 运维管理员）、`/api/logs` 会话账号本人、管理后台邮件审计者。属自用工具，"管理员即高权限位"敌情模型下实际风险有限——但**破坏了"token 脱敏全路径"契约的完整性**，与 Vision 原文脱敏的先例（"原文回传客户端是信息外泄面"）不对称。

**核实建议**：修复方向可选（a）zhidao 层错误上抛前剥 URL（`shouldSanitizeDoErr`：url.Error 时改 `err.Op + " " + err.Err` 描述，保留 ErrUnauthorized/IsReadErr 判型所需语义）；（b）scheduler/handler 消费侧统一 `sanitizeErr(err)` 剥 `?idToken=xxx` 段。最小改动为（b）侧一层包装拦截四类进入点。真实平台偶发 connectex 的战术价值远低于此静默改动价值，但**契约完整性角度值得处理**。

## OBSERVE

### B101-02：日志脱敏完整性全路径扫描——除 B101-01 外全部写点闭环（实测）

逐一归档全部 `log.Printf`/`fmt.Errorf`/回显点（scheduler.go 35 处、zhidao/captcha.go 1 处、api handler 21 处、accounts/manager.go 5 处、main.go 7 处、cmd 三工具）：`账号 %s` 类只含账号名；token 输出仅 maskedToken/tokenShort；密码只输出"加密/解密失败 %v"（err 为密钥错误不含明文）；vision_key 仅 maskKey 回显；session randToken（熵源故障 panic 不落日志）；cmd/logintest 输出 `token[:8]`。**唯一缺口即 B101-01 的 doRequest 错误链**。契约局域网闭环达成。

### B101-03：contract-20 强弱扫描——X-XX 注释锚点裁定合规（走读）

主控特别提示留意"不匹配「第 N 轮」字面形态的行首/行尾编号锚点"。全仓 `--include=*.go`（排除 _test.go）grep `\b(B|M|O|R|F)[0-9]{2}-[0-9]{2}\b` 命中仅 5 处：scheduler.go:843 `M88-01 身份防线`、quit_shared.go:7 / tray_windows.go:53 / :65 `M86-01`（回归钉）。历轮 R93/R94 已裁定："注释含协议锚点（B42-01/M86-01 等以契约号标注，非轮次历史）符合规范"。本轮强扫一致：这些锚点全部映射 CLAUDE.md 决策手册契约条目（可追溯性），内容均表述"为什么/契约/陷阱"而非"第 N 轮"决策历史，**不违规，维持合规**。（对照组：session/store.go:117 的 `docs/review-round13.md` 文档路径引用，历轮亦裁定非决策标签。）无发现需要上报的违规。

### O101-01（O100-01/O99-01/O98-01/O97-01/O86-01 抖动基线第十六轮）：定向 race 本轮零复现，低帧口径维持（实测）

**证据**：本轮代理权限内做定向三包+七包 race（`go test -race -count=1 -p 1`）：zhidao 3.275s / scheduler 15.187s / api 23.392s 全绿；store 40.831s / db / session / accounts / config / runtime / secure 全绿；zhidao 单包复跑 2.422s 全绿；全包非 race 10 包全绿。**本轮零复现**。走读评价：TestRecognizeCaptcha 仍是包内唯一"无探活 mock 宿主"但 readyProbe 双保险已在（captcha_test.go:29）；socketPreheat（client_test.go:25-30 包级 TestMain + 逐测试）与 readyProbe（client_test.go:88-113 宽栅栏 10×200ms）使用正确；api 夹具 net.Listen 预创建 + readyProbe（handler_test.go:67-71/139-141）生效。**未见新测试时序脆弱点**。基线口径维持：低频残余由"宿主环境冷启动窗口 + 包序"主导，CI `-p 1` + 失败重跑吸收，非产品缺陷（R94-R101 八轮 30 跑中 4 次单包单 FAIL，~13%）。

### O101-02（O100-02/O99-02/O98-02/O80-01 删除保护撞名延续复核）：单判据与 B43-04 双条件不对称——历轮维持观察，无新依据提级（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只在账号名等于 `AdminNameValue()` 时拦截删除；IsAdminAccountName（handler.go:56-58）单字段比对。撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下该学生被删除保护永久覆盖。历轮归"低优先级不修"的三条理由本轮逐一复核仍成立：① requireAdminSession 前置（router.go:153-159）；② B43-04 双条件签发让撞名学生正常登录；③ 删除保护是"防空删管理员自己"的硬护栏语义。**维持观察**。

### B101-04：新契约角度一——多账号并发提交全局对称性全绿（走读）

逐 handler 复核账号路由：`sessionAccount(r)` 从 requireAuth 注入的 sessionCtxKey 取值（handler.go:1049-1054）；`allowAccountOverride` 仅 `Sessions.IsAdminToken` 为真才放行 `?account=` 穿透（handler.go:1066-1068）；五个 `?account=` 消费点（handleElectives:243 / handleElectiveSelect:293 / handleElectiveExit:371 / handleSetTargets:446 / handleState:535）全部 `accountExists`（凭据表逐账号比对）确证后路由至该账号专属 client；普通会话绝不向非绑定账号发起任何请求。`GenerateClient` 无旁路（accounts.Manager.ClientFor 单表查找）；scheduler 侧 `sessionAccount` 不存在——所有提交链按 `acctTargets` map 键 + ClientFor 绑定客户端，无凭 URL 参数猜账号的路径。handleLogs（handler.go:564）直接用 sessionAccount 过滤日志，管理员会话享 admin 全量。**账号路由无旁路、穿透仅管理员、凭证表判据同源，全局对称**。

### B101-05：新契约角度二（附）——SQLite 单写者锁内无网络慢操作（走读） + doLogin 返回错误亦含 idToken（附注）

SQLite 侧：`SetMaxOpenConns(1)`（db.go:23）串行化写；持锁写只发生在 spawnChain 成功/失效/满员分支的 `AppendLog/SaveSuccess/SaveRefused/DeleteRefusedClass`（微秒级 Exec），明示不跨"锁外复核"网络段（scheduler.go:1583-1587 注释）。**锁内无网络慢操作，正确性边界成立**。

附带发现（同 B101-01 族）：**登录提交错误也可能携 idToken**——`submitLogin`（client.go:361-370）用独立 sess（无 token 注入 URL，登录期 URL 无 idToken，**不泄露**）；但 `doRequest` 全家族（findElectivesData / selectElectivesClass / exitElectivesClass / findElectivesStudentCount / /electives/select）都携带 idToken，其 dial/write 错误均经 B101-01 链路。**登录期无该面，仅已登录会话的课程/报名/人数接口受影响**；`fetchLoginPage`/captcha 用独立 sess 无 idToken 亦无此面。

### B101-06：新契约角度二（附）——SyncServerTime 对齐时钟无遗漏分支（走读）

`nowAligned()`/`nowAlignedLocked()` 唯一实现 = `time.Now().Add(clockOffset)`（scheduler.go:265-275）。全部时间语义消费点逐点核对：tick 提交守卫（:974/:1011-1015）、probeIntervalForOpen（:88-102，now 由 tick 对齐钟传入）、探测/提交/退避/识别四类写入（:835/:962/:1059/:1346/:1441/:1714 全走 nowAligned 族）、rateLimited 读写（:1707-1715 注释载明基准统一）、syncFailedWindow/reloginAt 三处裸 `time.Now()`（:372/:377/:1231/:1261）——全部只做"事件时刻留档/节流间隔计时"（如失败时刻、退避发起时刻），判读侧用 `time.Since` 做差值抵消 offset（同一本地钟），**不参与开窗点判定，非时间基混用**。WindowClosed 两兜底判据（:925/:935）用 nowAlignedLocked 对齐钟。**无未用对齐时钟的遗漏分支**。

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵（第十六轮）**：`sameClientFor`（scheduler.go:204-210，判 nil 恒 false + reflect 指针身份）+ 全量调用点逐点核对——spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口统一复位（:1600）/确证满员（:1635）六分支，**第七分支实时复核 ErrUnauthorized（:1608）的 maybeRelogin 落入 :1600 统一 sameClientFor 之后才触发——七分支对称闭合**；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeForAccount 回写段（:850）+ 识别槽覆盖仅身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；ProbeNow（:963-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 三路（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）；PurgeAccount（:495-519）全量清含识别槽。**无新裸露写点**。round41 测试族全部用 waitChainExit（scheduler_test.go:3109）。 | 通过（走读 + 定向 race） |
| **B88-01 修复持续复核（第十六轮）**：ProbeForAccount 持锁先 sameClientFor（:850），失败整体放弃并 return data（:851-854）；识别槽覆盖双条件（:855-858）；acctData nil 守卫在身份复核后（:859-862）。probe_identity_test.go 三钉走读语义正确；定向 race 全绿。 | 通过 |
| **O98-02 删除保护撞名**：handler.go:992 单判据 + B43-04 双条件不对称，历轮维持观察（详见 O101-02）。 | 维持观察 |
| **O92-02 logintest 引擎判定源分叉**：cmd/logintest/main.go:57 走 `CaptchaEngineDefault()` 环境变量，主程序走 settings 持久化值覆盖——三态回退链完整，诊断工具语义固有分叉，历三档案关闭维持。 | 维持关闭 |
| **M87-01 窗口**：5s Shutdown + 在飞链尽力优雅语义仍由 main.go:191-200 注释全覆盖；quit_shared.go 双 nil 防御 + setExitActions 两半段 + tray_quit_test.go 三钉。 | 维持 MINOR + 注释兜底 |
| **O90-01 CRLF**：git HEAD scheduler.go CRLF 0 / LF 2049（实测），工作区 CRLF 为 checkout 转换噪音，永久无动作。 | 维持 |
| **契约 17 零吞错**：spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库、maybeRelogin UpdateIDToken、handleAdminConfig 落库、handleAdminDeleteAccount、login 加密失败、main 恢复各表——全部 `if err != nil { log.Printf }` 零吞错。 | 通过 |
| **多账号全局资源边界（B101-04 复核）**：sharedTransport 128/64 + HTTP/2；probeSem cap 4；captcha 1-20 值域校验；gate 2/min 双入口共享 gateMu；reloginResults buffered 8。全部有界。 | 通过 |
| **契约 1/2/6/7/9-13/14/15/16/21-25/27-31/34-41 历轮复核资产**：本轮未扰动的契约条目维持历轮结论不变（HEAD 仅归档文件新增，零生产代码改动）。 | 维持 |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/scheduler/ ./internal/api/` | 全绿（3.275s / 15.187s / 23.392s） |
| `go test -race -count=1 -p 1 ./internal/store/ ./internal/db/ ./internal/session/ ./internal/accounts/ ./internal/config/ ./internal/runtime/ ./internal/secure/` | 全绿（store 40.831s 等，含 scheduler 身份的依赖面板） |
| `go test -race -count=1 -p 1 ./internal/zhidao/`（单包复跑） | 全绿（2.422s） |
| `go test -count=1 -p 1 ./internal/{db,config,runtime,secure,session,accounts,store,zhidao,scheduler,api}/`（全 10 包非 race） | 全绿 |
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0） |
| 实测 url.Error 文本 | 确证 `Post "http://127.0.0.1:1/electives/select/findElectivesData?idToken=...": ...` 含完整 token（B101-01 锚） |
| `git show HEAD:backend/internal/scheduler/scheduler.go` CRLF 计数 | CRLF 0 / LF 2049（LF 合规） |
| `git status --short --branch` | `## master`，后端仓库文件零改动（前端代理归档文件除外） |

## 结论

1. **身份防线矩阵第十六轮闭合**：spawnChain 七分支（含第七分支实时复核 ErrUnauthorized）sameClientFor 全覆盖、maybeRelogin 双侧、ProbeForAccount 回写段双条件——无裸露写点，定向 race 全绿。
2. **O101-01 抖动基线第十六轮**：定向 race 本轮零复现，低频残余口径维持（R94-R101 八轮 30 跑 4 次单 FAIL ~13%），readyProbe/socketPreheat 双保险使用正确，无新脆弱点。
3. **B101-01（MINOR，本轮仅此一条非观察发现）**：doRequest 网络层错误把完整 `?idToken=` URL 原样带入日志/库表/前端回显，破坏 token 脱敏全路径契约——建议消费侧或 zhidao 上抛侧剥 URL 的轻量修复。
4. **新契约角度**：多账号并发提交全局对称性全绿（sessionAccount 绑定 + 管理员穿透凭证表判据 + 无 URL 猜账号旁路）；日志脱敏扫描唯一缺口即 B101-01；SQLite 锁内无网络慢操作；对齐时钟无遗漏分支。
5. **契约 20 强弱扫描**：X-XX 锚点 5 处全部映射契约号（M86-01/M88-01），内容表述"为什么/契约"，历轮裁定维持合规，不报违规。

工作树后端文件洁净。本轮发现仅 B101-01 一条 MINOR 具修复价值的真实缺口，其余为延续观察记录。