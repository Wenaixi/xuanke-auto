# round52 后端审查原始发现

> 审查基线：master @ `7a8a177`（R51 收官落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档 CODE_OF_CONDUCT/CONTRIBUTING/LICENSE/SECURITY/THIRD_PARTY_NOTICES，无工作区改动；审查期间工作区零改动，绝对只读，未修改/创建/删除任何文件——唯一新文件是本报告）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 + legacy/website-source 逆向契约 + round39~51 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 上轮观察项逐一核实 + 本轮新视角扫查（Start 幂等性、handleLogout 会话吊销联动、Restore×LoginByPassword 并发、LoadCredentials 单账号失败语义、zhidao HTTP 非 200 响应路径、initCaptchaAtStartup 幂等）。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0（本轮独立重跑确认）。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 连跑 3 轮（第 1/2 轮全绿 EXIT 0，第 3 轮 FAIL 1 包）+ api 包单独连跑 5 轮（第 4 轮 FAIL 3 测、其余全绿）——失败详情实证如下，详见 MAJOR-52-01。

---

## CRITICAL

（无本轮新增 CRITICAL。F51-O1 前端删依赖对后端零影响；F50-M1/M2 双修复核闭合；本轮新视角六项均确认无数据污染/崩溃通道。）

---

## MAJOR

### MAJOR-52-01：httptest mock 连接 flake 本体继续存活——本轮实证失败断言面扩至 3 个新测试，仍全部同根 connectex（R49/R50/R51 同族延续，非新增缺陷，无升级亦无降级）

- **位置**：`backend/internal/api/handler_test.go`（newTestDepsModeName 内 httptest.NewServer mock 平台）+ `.github/workflows/ci.yml:59`（`||` 重跑链）。本轮失败测试：TestAdminConfigHotReload / TestElectiveSelectRejectsWindowClosed / TestElectiveSelectRejectsFullClass。
- **一句话问题**：F50-M2 已正确统一两条命令 `-count=1`（重跑侧真实执行），但 flake 本体未根除——本轮 api 包单独连跑 5 轮中第 4 轮 FAIL 3 测，失败详情全部实证为 `Get "http://127.0.0.1:<port>/login": dial tcp ... connectex: A connection attempt failed...`（mock 平台 HTTP server 瞬时连接拒绝，Windows 127.0.0.1 回环端口 churn）。全量第 3 轮 FAIL 1 包同为 api（tail 截断未见详情，与单跑同根）。CI `||` 重跑 + 单跑复验均全绿吸收。
- **新增样本（观察面扩大）**：此前两轮样本集中在 flake 伪装"未激活 1001"（TestLoginUnactivatedNeedsCode）与"登录失败文案"断言；本轮 TestAdminConfigHotReload 在"关闭激活码后登录应直接签发会话"（handler_test.go:612）断言处挂掉、TestElectiveSelectRejectsWindowClosed（:979）/TestElectiveSelectRejectsFullClass（:1006）在"初始化登录会话失败"断言处挂掉——同根 connectex 经 zhidao.Login→LoginByPassword err 翻译成业务文案，落在完全不同语义的断言行上。佐证 MAJOR-51-01 的根治建议（登录链路测试夹具首请求失败重试一次 / mock server 就绪探测）同时消掉全部伪装形态。
- **实证**：① 全量 3 轮：第 1、2 轮全绿，第 3 轮 FAIL（api 包）；② api 单跑 5 轮：第 4 轮 FAIL 3 测（3 个不同断言行，详情逐条打印 connectex）、其余 4 轮全绿；③ 失败测试单跑均绿。**无 race 报告**。
- **裁决**：MAJOR（CI 基础设施级，延续 R49/R50/R51 同族）。主控若判定 CI 重跑已足够吸收可降级观察；本轮不强推固定根治，但样本数已累计超过 6 个不同断言行，建议在登录链路测试夹具层做首请求失败重试以彻底熄灭。

---

## 本轮重点核对（上轮新契约，防回归）—— 4 条全部闭合

### 1. F51-O1（前端删 Dialog/Sheet/Table 组件 + 3 依赖）对后端零影响 — 确认闭合
- commit c2ed5b5 diff 复核：仅动 `web/package.json`、`web/package-lock.json`、`web/src/components/ui/{Dialog,Sheet,Table}.tsx` 五个前端文件，backend/go.mod、backend/*.go 零触碰。前后端依赖完全隔离：backend 仅 require `go-ddddocr` + `modernc.org/sqlite`（go.mod 与 `git show` 均实证），Go embed 用的是 `//go:embed dist/*`（web/dist 静态产物，与 web/node_modules 无关）。**无任何回归面**。
### 2. F50-M1（stats body code 500 对齐）+ F50-M2（CI 重跑 -count 统一）— 复核正确闭合
- F50-M1：handler.go:912 目标数失败分支 `writeJSONStatus(w, 500, 500, ...)`，与 F48-O3 落库失败分支（:810）同族对齐；前端只读 body code 契约不变（client.ts 只特判 HTTP 401，R51 已核）。F50-M2：ci.yml 两条命令统一 `-count=1`，重跑侧真实执行。两条均语法/断言正确无回归。
- **残余观察延续**：`failingTargetsStore`（handler_test.go:829）仍无消费点（TestAdminStatsTargetsLoadFailureReturns500 只测正常路径 200 + body code 0），500 分支无真红绿，仅实现内复查背书——无法在 store 方法注入失败的容器下实现（Register 接收 *store.Store 非接口，测试注释明确"替代注入方案超范围"）。延续观察。
### 3. F48-O3 / F46-O1 / B45 / B43 / B44 上轮实修 — 全部复核正确闭合无回归
- scheduler.go:237-239 interval clamp + TestScheduleIntervalClamped 真红绿；钉子集（TestWindowOpenSubmitsWithoutProbeReset / TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime / TestAdminStatsWindowOpenedUsesScheduler / TestScheduleIntervalClamped）复跑全绿。
- B43-01 入口复核（maybeRelogin ClientFor 存在性）+ sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员两分路）；身份族测试 6 个在库绿（TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+RealtimeUnauthorized）。
- B45-N3 / B44-01 accounts 包回归绿；B42-01 gateTryAcquire 收口 + ResetGateForTest 夹具（api 测试多账号批量注册前置）无干扰。

---

## MINOR

（无本轮新增 MINOR。R50 MINOR-50-01 的 writeJSONStatus 家族 body code 开裂已被 F50-M1 修复闭合，本轮家族状态码逐点复核无新开裂。）

---

## OBSERVE

### OBSERVE-52-01：flake 伪装业务断言的新样本面（R51 OBSERVE-51-01 同根延续，样本累至 6+ 断言行）
- 本轮 api 单跑第 4 轮 FAIL 3 测详情实证：TestAdminConfigHotReload（assert 612 行）、TestElectiveSelectRejectsWindowClosed（:979）、TestElectiveSelectRejectsFullClass（:1006）分别在三个**完全不同语义**的断言行收到 `登录失败: 初始化登录会话失败: ... connectex` 业务文案——mock 连接失败经 zhidao.Login→LoginByPassword err 翻译，落到"关闭激活码后应签发会话"/"窗口关闭应被拒"等断言上误红。佐证根治建议（登录链路首请求失败重试一次）同时消掉所有伪装形态。

### 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-51-01（httptest flake 本体） | MAJOR | 全量 3 轮 1 FAIL + api 单跑 5 轮 1 FAIL（3 测 connectex 同根、3 个新断言行实证）——F50-M2 闭合缓存漏洞后 flake 本体继续存活。 | **延续 MAJOR（即本轮 MAJOR-52-01）** |
| OBSERVE-51-01（flake 伪装业务断言） | 观察 | 新样本 3 个（TestAdminConfigHotReload 等断言行 612/979/1006），形态与 R50/R51 完全同根。 | 延续观察（并入 MAJOR-52-01） |
| OBSERVE-49-01（main 优雅退出） | 观察 | 全仓 `signal.Notify` 仍 0 处；sched.Stop/sessions.Close 依赖 defer；SIGTERM 时飞行 spawnChain success 可能未落库（SQLite WAL crash-safe 兜底）。 | 延续观察 |
| OBSERVE-49-02（chains map 无泄漏） | 观察 | 生命周期严格配对（spawnChain 入口置位 + defer 清）+ cap=账号×发布有界；waitChainExit 契约（用 chains 活跃标记而非 inflight）6 处测试沿用。 | 延续观察 |
| OBSERVE-49-03（rateLimited×done/full） | 观察 | MarkDone 清 rateLimited+full；SetTargetsForAccount 刻意保留（B19-02）；读侧同锁。 | 延续观察 |
| OBSERVE-49-04（gate 窗口翻转） | 观察 | 共享 gateMu 串行、窗口重置锁内；每窗口 doLogin≤2；gateTryAcquire 非阻塞优先。ResetGateForTest 测试专用不干扰计数。 | 延续观察 |
| OBSERVE-49-05（task_log 无清理） | 观察 | 全仓 `DELETE FROM task_log` 仍 0 处；AppendLog 8+ 写点无条件 INSERT。 | 延续观察 |
| OBSERVE-49-06（大响应解析） | 观察 | 2× 响应体峰值内存 + 零值缺省容错 + 空快照提前返回（parseElectives code:0 空 publishes 直返空快照绝不落兜底）。 | 延续观察 |
| OBSERVE-48-01（submitAll 300ms 拍） | 观察 | spinChain 链内串行不受节流；黄金期 250ms 间隔 + tick 300ms 拍。 | 延续观察 |
| OBSERVE-48-02（reloginResults cap 8） | 观察 | 非阻塞 select default；满丢弃仅并发重登爆炸 >8。 | 延续观察 |
| OBSERVE-47-02（B43-05 500 分支无真断言） | 观察 | failingTargetsStore 仍无消费点；500 分支走实现内复查 + 实现注释背书。 | 延续观察 |
| OBSERVE-47-03（XUANKE_PORT 无校验） | 观察 | `:abc` → ListenAndServe log.Fatalf 拒绝启动；失败形态清晰。 | 延续观察 |
| OBSERVE-47-04（孤儿登录） | 观察 | Relogin 锁内取指针锁外 Login；删号+在途重登并发平台侧一次孤儿登录，频率极低无污染。 | 延续观察 |
| OBSERVE-46-02（Retry-After 不消费） | 观察 | 全仓 `Retry-After` 0 处；isRateLimitError 文案匹配固定 30s；平台无 429 头样本。 | 延续观察 |
| OBSERVE-46-04（目标不校验重复/跨年级） | 观察 | byPub 分组 + 快照复核 + 平台把关。 | 延续观察 |
| OBSERVE-45-01（access_limit_cookie 占位） | 观察 | probe/main.go:24 与 submitLogin（client.go:371）同款占位宽度；token 权威通道 URL 参数。 | 延续观察 |
| OBSERVE-43-01（probe 双槽分叉） | 观察 | 主体 `["*"]` + per-account `[acct]`；优先级 [acct]→[*]；全校单值不实际分叉。 | 延续观察 |
| OBSERVE-43-03（classFullRealtime 网络段重复） | 观察 | IsClassFull 恒 false（maxCount 未下发）；快照判满主路径。 | 延续观察 |
| OBSERVE-43-04（tick 无 recover） | 观察 | tick/probe/spawnChain 均无 recover；interval clamp 封堵最可达 panic。 | 延续观察 |
| MINOR-43-02（syncFailedWindow 死字段） | 观察 | 写（scheduler.go:382/399）不读、测试也无消费点；注释载明留档语义。 | 延续观察 |
| MINOR-43-03（登录失败无固定延迟） | 观察 | 学生走网络往返天然延迟；管理员错误口令分支 loginTimingFlat(300ms) 保留。 | 延续观察 |
| MINOR-43-04（reloginBackoff 注释） | 观察 | n=1 返 30s、n=2 返 60s；封顶 5 防溢出（30s<<5=960s>10m）。 | 延续观察 |
| M40-01 / m40-02 / o40-01~04 / m39-02 / o39-02~05 | 观察 | 全部延续（未激活不发会话 / reloginResults 满丢弃 / columnExists 全常量 / interval clamp / SubmitAllowed 钉子 / submitAll 快照竞态 / vision key 空串不清空 / NAT 合并限流 / tick 无 recover / task_log 无清理）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **scheduler Start 幂等性 / Stop 后 Start 语义**：`start bool` 守卫防重复 Start（scheduler.go:664-670 双 Start 第二次直接 return，绝不双 tick）；但 Stop（cancel）后 `start` 不复位——Start 是对单个调度器实例的"一次性启动"，Stop 是终止信号，二者按 main 调用序列（New→Restore*→Start→defer/dead 退出）使用，无"反悔重开"需求。TestScheduleIntervalClamped 用 t.Cleanup(s.Stop) 实证 Start→Stop 无 panic 无泄漏。**无问题**（重复 Start 不双跑、Stop 幂等由 context.CancelFunc 语义保证）。唯一可留意点：Stop 后 Start 不会重启（start 恒 true），属"行为未定义但无调用方"，不构成缺陷。
- **handleLogout 与会话吊销联动**：handleLogout（handler.go:567）先取 token → Sessions.Delete → 记日志。删除即服务端令牌失效，后续该令牌所有请求 401（TestLogoutRevokesToken 实证）——前端 client.ts 收到 401 走 onUnauthorized 登出/切号，与本轮无关（前端侧）。管理端删账号经 RevokeAccount 全量吊销。**无问题**。
- **accounts Restore 与 LoginByPassword 并发（同一账号 restore 中又被手动登录）**：Restore（manager.go:295）逐账号 ensure→SetCredentials→SetCookies，无整块大锁——与 LoginByPassword 的 ensure（锁内）+ Login（锁外）并发时，同账号两个客户端可能是**不同实例**（Restore ensure 建一个、LoginByPassword ensure 拿同一个——ensure 是 map key 幂等，两个调用途同一实例），SetCredentials 最后一次写赢 token，会话不交叉污染。B43-01/B21-03 的 ClientFor 复核保护写回侧。**无数据污染通道**。
- **store LoadCredentials 解码失败单账号语义**：manager.Restore（manager.go:296-317）对单账号解密失败 try-catch 内联（日志 + pwd 保持空）继续处理下一账号——不整体中断恢复；密码解密失败账号降级为"无保存账密"（自动重登不可用但 token 生效可正常抢课），与 B44-01 加密失败留痕对称。main.go:59 分叉 `err != nil` 只记日志不 FATAL（DB 打开失败才 FATAL）。**单账号失败不拖垮整体，语义合理**。
- **zhidao HTTP 状态码非 200 响应（500/404 时 body 解析路径）**：doRequest（client.go:409-428）对任何状态码都读 Body → json.Unmarshal→再判 code——平台返回 HTML 500 时 Unmarshal 失败返回"响应解析失败"，错误化正确、不 panic、不静默；SelectClass/YearTerms 等对 err 正常上抛进入调度失败路径。唯一留意：**非 JSON 的 500 不会进入 ErrUnauthorized 判定（无所谓，平台不鉴权的 500 本就不该触发重登）**。**无崩溃通道**。
- **main initCaptchaAtStartup 幂等（多次调用引擎覆盖）**：router.go:22-30 用包级 `captchaSemInit bool` 守卫仅首次生效；生产仅 Register 一处调用；测试多 handler 重建时后续调用直接 return 不覆盖既有引擎。**无引擎覆盖问题**。
- **web/.omc + embed（是否被 //go:embed 误嵌）**：`//go:embed all:dist` 只嵌 web/dist（`dist/` 单目录），web/.omc 在 dist 外不被嵌入；SpaHandler 的 fs.Stat 只在 dist 内查文件。**无问题**。
- **契约 20（代码注释无轮次标签）**：产品代码 `第 N 轮` 0 残留；MAJOR/MINOR 大写引用为历史决策文档锚点（MAJOR-B/C/D/F 等）属正常"为什么"记录非轮次标签。测试注释仍有历史轮次记录（refused_test.go:71 等），判据允许保留（R51 已裁决）。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查**：`go build ./...`、`go vet ./...` 双通道本轮独立重跑 exit=0。
- **测试**：全量 `-race -count=1 -p 1` 3 轮（第 1、2 轮全绿；第 3 轮 FAIL 1 包 api）+ api 单跑 5 轮（第 4 轮 FAIL 3 测 connectex 同根、其余全绿）；无 race 报告。F50 钉子集锁定在库。
- **身份复核族**：spawnChain 六分支 sameClientFor + maybeRelogin 决策侧（B43-01）/写回侧（B21-03）闭合。
- **WindowClosed 三判据单源**：windowClosedLocked（scheduler.go:914-935）+ StateForAccount 镜像 + handleAdminStats window_closed 同源（B29-02）。
- **doLogin 闸门全收口**：gateWait（重登排队）/gateTryAcquire（学生手动登录 + 管理员换绑，B42-01）共享 gateMu/gateUsed 计数；both ≤2/min。
- **鉴权与数据安全**：writeJSONStatus 家族真实状态码（401/403/429/500/panic 500）；SpaHandler /api 前缀（含精确 /api）双处 404；XFF 仅回环+开关（XUANKE_TRUSTED_PROXY）；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 32 字节双重校验。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量拼接；migrateAddPublishMeta 增量幂等（publish_name/begin_date）+ refuseLegacy 缺列清单对应剔除（priority/allow_swap/account 拒绝）。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 / empty priorityId（jQuery 丢弃语义） / captcha Limiter Cond 动态热收敛 / 重试收敛 ≤3 / code=-1 统一 ErrUnauthorized（doRequest:425）/ cookie+idToken 双通道与逆向契约一致。
- **session 层**：12h TTL + 5min 清扫 + sweeperClose sync.Once + ConsumeTicket 锁内四查；sweepLoop 防泄漏（Close 等待 sweeperDone）。
- **前端契约**：client.ts 只读 body code、HTTP 仅特判 401——F50-M1/M2 的 HTTP 状态码增强无回归。
- **CI**：ci.yml 两条 `go test -p 1 -count=1 -v`（F50-M2 已统一 -count）+ Windows job `$env:CGO_ENABLED='1'` 前缀（F49-M1 陷阱修复，CGO=1 内嵌 ddddocr 构建验证）。

---

## 结论

- **MAJOR 1（httptest flake 本体延续，MAJOR-51-01 无升级无降级）/ OBSERVE 1 新样本面（flake 伪装业务断言累至 6+ 断言行）+ 延续 27+ 项；F51-O1、F50-M1/M2、F48-O3、F46-O1、B45/B43/B44 上轮新契约全部复核闭合无回归**。
- 最重 3 条（按影响排序）：
  1. **MAJOR-52-01（httptest mock 连接 flake 本体仍在）**——全量 3 轮 1 FAIL + api 单跑 5 轮 1 FAIL（3 测 connectex 同根、断言行 612/979/1006 三个新样本），失败测试单跑全绿；CI `||` 重跑 + 单跑复验能吸收但产生低频假红噪音。根治仍建议"登录链路测试夹具首请求失败重试一次或 mock server 就绪探测"（样本已达 6+ 断言行，建议本轮或下轮实施）。
  2. **OBSERVE-52-01（flake 伪装业务断言新样本面）**——TestAdminConfigHotReload 等三个测试的断言收到 connectex 翻译后的"登录失败"业务文案而误红，佐证根治建议同时消掉全部伪装形态。
  3. **观察延续最高优先残余**：task_log 无清理（超 50 轮未动，黄金期日志风暴）、tick 无 recover + 优雅退出缺口（SIGTERM 在飞 success 可能未落库，SQLite WAL crash-safe 兜底）。
- 上轮全部修复项（F51-O1 前端删依赖 / F50-M1 stats 500 / F50-M2 CI -count/ F48-O3 家族整风 / F46-O1 clamp / B45/B43/B44）复核无回归。上轮观察项 27+ 条全部复核：除 MAJOR-51-01 延续为本轮 MAJOR-52-01（flake 本体）+ OBSERVE-51-01 累入 OBSERVE-52-01 外，其余全部延续无升级、无降级。
- 残余风险集中在：CI flake 假红噪音（52-01）、task_log 长期无清理（49-05/47-01 观察延续）、孤儿登录平台侧副作用（42-02）、tick 无 recover（43-04）+ 优雅退出（49-01）。