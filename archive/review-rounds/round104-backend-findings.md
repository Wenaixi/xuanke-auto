# R104 后端只读审查报告

审查对象：xuanke-auto HEAD `998ef94`（R103 收官收尾轮：纯文档提交，仅 archive/review-rounds 三个文件，后端零代码改动——`git show 998ef94 --name-only` 实证 backend/ 未触碰，工作树洁净）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round104-backend-findings.md）。

审查方式：Read / Grep / Bash 只读命令（定向 go test -race / go vet / 逐点走读）。核心走读范围：身份防线矩阵第十九轮（spawnChain 七分支 + maybeRelogin 双侧 + MarkDone/RemoveDone/SubmitAll + PurgeAccount + api 手动路径四路）、O103-01 抖动基线第十九轮夹具走读、B101-01/B102-01/B103-01 持续盯守（六测试在位 + 全 http 直调点枚举）、既往观察项延续（O103-02/O92-02/M87-01/O90-01 + 契约 20 强扫 + 契约 17 零吞错）。新契约角度本轮聚焦：**数据库并发写与事务边界复查（单写者锁 / Restore 各表顺序 / 迁移流程）+ 手动操作状态迁移穷举**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## MINOR（知识位固化，非缺陷）

### B104-01：手动报名/退选路径的"同一身份"防线缺位是**理论缺口而非历史遗留裸写点**——api 手动路径不存在可并发插入删号+同名重建的窗口，B20-01 的存在性复核已封死该竞态（走读评估）

**位置**：`handler.go:286-433`（handleElectiveSelect/handleElectiveExit）+ `scheduler.go:1896-1914/1922-1982/1989-2035`（TryAcquireSubmit/MarkDone/RemoveDone）。

**争议澄清——历史审查 R39/R41 的"手动路径同款竞态"指控本轮评估为从未成立**：
- R39（M39-01）与 R41 将自动链 spawnChain 的"存在性≠同一性"problem statement 表述为"**同族还有**……手动 MarkDone/RemoveDone"，提示手动路径同受删号+同名重建寄生。R41 裁定自动链六分支补 sameClientFor（B41-01）、固定 MarkDone/RemoveDone 在矩阵中作为"存在性"档（round89/90/91 矩阵表第 13 项，历轮注明"手动报名网络段由 api 层持有"）。**但历轮未明说为什么手动路径不需要指针身份**——本轮补上这个知识位。
- **触发场景推演（对照 M39-01 的 t0-t5 六步）**：同名重建"重新登录"必须经 `POST /api/login`→handleLogin→`issueSession`。该链路**全程运行于一个 http handler 的单一 goroutine、绝不释放任何 MUTEX、不产生任何在飞网络+解锁的间隙**；而手动报名（SelectClass 最长 15s）同样在单 handler 内同步执行。两条操作各自的函数内不存在"发起请求→释放锁→等网络→回锁再判断"的结构，因此**不存在 M39-01 推演所需的交错点**（自动链的寄生窗口恰恰来自 spawnChain goroutine 解 s.mu 后的网络段）。B20-01 在 MarkDone 内的存在性复核本就是对"删除恰好在本请求完成后发生"的兜底，指针身份在此无额外增益（同名重建要么发生在请求前——存在性复核已拒绝；要么发生在请求后——新身份独立且成功已真实发生在旧身份平台会话上）。
- **结论**：手动路径在 B20-01 定案语义下已封死；"存在性≠同一性"的泛化指控不适用于无在飞窗口的同步 API 路径。**知识位固化**：若未来引进"异步手动报名"（handler 启动 goroutine 后回执）——即在飞网络段跨 goroutine——MarkDone/RemoveDone 必须同族升级为 sameClientFor 指针身份比对，matrix 第 13 项跟随升级。

## OBSERVE（延续观察）

### O104-01（O103-01/O102-01/……抖动基线第十九轮）：夹具走读无新脆弱点，定向跑全绿零 flake（走读 + 实测）

socketPreheat（client_test.go:25 预创建-关闭回环套接字）+ readyProbe 宽栅栏（zhidao 200ms×10+2s 超时 / api 10×200ms / accounts 同款）布局与 R103 逐字符一致，本轮零改动。**未见新时序脆弱点**。定向跑（zhidao 脱敏六测+判型+识别 / scheduler 身份防线族+窗口守卫+提交 / api 管理+会话+撞名+穿透+手动复核 / session / db / store / accounts）全绿零 flake。基线口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（八轮 30 跑 4 单包单 FAIL ~13%）。主控跑全量 race 吸收结论。

### O104-02（O103-02 删除保护撞名延续复核）：handler.go:992 单判据与 B43-04 双条件不对称——维持观察，无新依据提级（走读）

handleAdminDeleteAccount（handler.go:992）`if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只在账号名等于 `AdminNameValue()` 时拦截。撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下防护永远只有"测账号名==管理员名"的外壳；B43-04 双条件已让撞名学生正常登录，但**该撞名学生可从管理后台被另一个管理员账号删除**（作为学生账号删除是合法操作）。历轮定"低优先级不修"三条理由本轮复核仍成立（防空删管理员自己的硬护栏语义 / requireAdminSession 前置 / 撞名正常登录语义已封）。**维持观察**。

### O104-03（O92-02 logintest 引擎判定源分叉）：维持关闭（走读）。

### O104-04（M87-01 窗口延续复核）：维持 MINOR + 注释兜底（走读）。

### O104-05（O90-01 CRLF 延续复核）：维持（实测——go vet 八包全干净 VET_EXIT=0）。

### O104-06（契约 17 零吞错延续扫查）：通过（走读）。全仓 `_ =` 落库点 grep 实证——MarkDone/RemoveDone/spawnChain 六分支全部 `if err != nil { log.Printf }`；本轮补扫 db.go Open 流程（schema 创建/迁移/refuseLegacy 全部 error return 上抛）与 store 全部 22 个方法 return error（调用方逐处 log）零吞错。

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第十九轮闭合**：`sameClientFor`（scheduler.go:199-232 **主控 grep 实证 7 处调用点**：:850 ProbeForAccount 回写段 / :1489 失效 / :1521 成功 / :1551 风控 / :1571 窗口关闭 / :1600 实时复核入口统一复位 / :1635 确证满员——全部在历史清单内，无新裸露写点）。第七分支实时复核 ErrUnauthorized（:1608：cErr 命中后也许 within 统一 sameClientFor 之后）`.maybeRelogin` 非裸调，:1621 确证满员分支在 :1635 身份复核后才 markFullLocked、:1627 doneHas 胜利状态让位对称。maybeRelogin 决策侧存在性复核 :1208（先 reloginMu 后 s.mu 锁序一致）+ 写回侧 :1254（先清 relogging 再剔删除竞态、成功分支内存写 l UpdateIDToken 落库同门）。ProbeForAccount 回写段 :850 持锁先 sameClientFor + 识别槽双条件 :855-858（空快照绝不删槽）。MarkDone :1927 / RemoveDone :1995 / SubmitAll :1355 / spawnChain 链顶 :1388 + 取 client 双判 :1405/:1409 / TryAcquireSubmit :1896-1914 inflight 互斥。api 手动报名/退选 :293-301/:371-379 accountExists 凭据表前置 + ErrUnauthorized → MaybeRelogin :349/:417。PurgeAccount :495-519 全量清。**无新裸露写点**。round41 测试族 refresh（scheduler_test.go:3031-3556 七测 + TestDeletedAccountManualInFlightDropsState）实测全绿。 | 通过（走读 + 定向 race 实测） |
| **B101-01 修复持续盯守（第十二轮避坑）**：sanitizeError（client.go:581-593 `Op + " " + ue.Err.Error()` + sanitizerErr Unwrap 下沉）doRequest 接入点 :450 单一收口。**六测试全绿实测**（TestSanitizeErrorStripsTokenFromDialError/PreservesJudgment/NilSafe/OriginalErrorPreserved/NeverEmpty + TestDoRequestSanitizesDialError 端到端）。判型函数 isConnErrRetryable（:496）/IsReadErr 经脱敏后判定不破坏。B102-01 透传扫描：裸 http.Do 五点（Prewarm:102/SyncServerTime:119/fetchLoginPage:300/fetchCaptchaImage:334/submitLogin:370）URL 均静态无 idToken，doRequest 仍唯一携 token 通道且 :450 闭环；captcha.go vision 请求走 APIKey 头通道 URL 静态。**无回归**。 | 通过（实测 3.4s 全绿） |
| **B103-01 知识位活化条件复核**：实时复核确证满员分支在真实平台不可达（CountEntry.MaxCount 恒 0，现代 `json` 静默忽略——平台 select.js 轮询回调只读 id/selectedCount/auditedCount 三键、两 HAR 无同期样本）；`classFullRealtime`（:1751）实际恒 (false,nil)/(false,err)，:1621 确证满员分支恒不进入。活化条件 =「平台向 countList 下发 maxCount」，届时 :1600/:1635 two sameClientFor + :1627 doneHas 胜利状态让位已在位直接受益。**维持知识位，无退化** | 通过（走读） |
| **新契约角度 a：手动操作完整状态迁移穷举**：handleElectiveSelect/Exit 全路径枚举——参数校验 → accountExists 前置 → TryAcquireSubmit 占位 → CheckClassSelectable 快照复核（窗口未开/满员拒绝 :327-330）→ ClientFor :333（幽灵账号已被前置拒绝）→ SelectClass/ExitClass 网络段 → 四分类错误归并（ErrUnauthorized→MaybeRelogin / IsReadErr→"可能已处理"文案 / isRateLimit 风控→不落库 / 其它→原文）→ 成功 MarkDone:363/RemoveDone:431。状态迁移闭环：MarkDone 清 refused（内存+库行, :1938-1948）+full+rateLimited+置 success / RemoveDone 删 done+置 refused+置 pending。手动退选后 spawnChain 该课永久跳过（:1447 refusedHas）；手动重选成功后自动引擎恢复接管。窗口已关平台"无效课程ID"→ markFull 不轰炸（:1570 isWindowClosedError）。**全链路无缺口**。 | 通过（走读 + api 定向测试） |
| **新角度 a：数据库并发写与事务边界复查**：SQLite `SetMaxOpenConns(1)`（db.go:23）天然单写者串行，WAL + busy_timeout 5000。事务族（SetTargetsForAccount 先删后插 / CreateActivationCodes 批量原子 / ConsumeActivationCode 压单 UPDATE 原子防超卖 + 事务内已激活查重 :313-319 / SaveSettings 全量替换 / DeleteAccount 六表单事务）全部 Begin→defer Rollback→Commit 规范、失败整体回滚。迁移流程（db.go Open）顺序 = schemaSQL 建表 → migrateAddPublishMeta 增量 ALTER（缺列才补）→ refuseLegacy 缺列清单已剔除已迁移列（:80 三列不含 publish_name/begin_date）。Restore 顺序（main.go:117-141）= RestoreDone → RestoreTargets 循环 → LoadRefused+RestoreRefused，与契约 6 一致；RestoreRefused 注释"必须先于 SetTargetsForAccount 循环"与 main.go 实际顺序匹配（后者已改好）。**无缺口**。 | 通过（走读 + 定向 race） |
| **契约 20 强扫**：全仓生产代码 grep `\b(B\|M\|O\|R\|F)[0-9]{2}-[0-9]{2}\b` 命中 3 处（scheduler.go:843 M88-01；quit_shared.go:7 / tray_windows.go:53/:65 M86-01）——协议锚点符合规范；`第 N 轮` 字面零命中生产代码。**无违规需上报**。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test ./internal/zhidao/ -run 'TestSanitizeErrorStrips\|TestSanitizeErrorPreserves\|TestSanitizeErrorNil\|TestSanitizeErrorOriginal\|TestSanitizeErrorNeverEmpty\|TestDoRequestSanitizes\|TestIsReadErr\|TestRecognizeCaptcha' -count=1 -race` | 全绿（脱敏六测试 + 判型 + 识别，3.4s） |
| `go test ./internal/scheduler/ -run 'TestDeletedAccountRebuiltSameName\|TestDeletedAccountManualInFlight\|TestProbeForAccount\|TestProbeChainSameClient\|TestWindowClosed\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestSubmit\|TestPurgeAccount\|TestOpenTimeRetained\|TestRestoreDoneSkipsResubmit' -count=1 -race` | 全绿（身份防线族 + 窗口守卫 + 提交，4.96s） |
| `go test ./internal/session/ ./internal/db/ ./internal/accounts/ -count=1 -race` | 全绿（session×2 + db×2 + accounts×2=6 包文件） |
| `go test ./internal/store/ -count=1 -race` | 全绿（24.7s 含激活并发） |
| `go test ./internal/api/ -count=1 -race -run 'TestAdminDeleteProtectsRenamedAdmin\|TestRenamedAdminSessionBindsConfigName\|TestAccountOverride\|TestElectiveSelectRejectsWindowClosed\|TestElectiveSelectRejectsFullClass\|TestAdminDeleteAccountMemoryFirst\|TestAdminDeleteRejectsUnnormalizedAccount\|TestAdminDeleteCodeNoBodyOK\|TestAdminDeleteAccountNoBodyOK' -v` | 全绿（9 测逐条 PASS——管理后台 + 撞名 + 穿透 + 手动复核 + 删除保护） |
| `go vet ./internal/{scheduler,zhidao,api,session,accounts,store,db}/` | 通过（VET_EXIT=0） |
| `git show 998ef94 --name-only` / `git status --short` | R103 纯文档轮（backend/ 零改动）；工作树干净 |

## 结论

1. **身份防线矩阵第十九轮闭合**：主控 grep 实证 7 调用点全部在历史清单内，无新裸露写点；第七分支、MarkDone/RemoveDone、api 手动路径四路、PurgeAccount 全部复核通过。
2. **O103-01 抖动基线第十九轮**：夹具走读无新脆弱点，定向跑多批全绿零 flake，口径维持（~13% 低频残余由包序 + 冷启动窗口主导）。
3. **B101-01/B102-01/B103-01 持续盯守**：六测试实测全绿无回归；doRequest 仍唯一携 token URL 通道且闭环；B103-01 活化条件（平台下发 maxCount）仍在位无退化。
4. **既往观察项延续**：O103-02 删除保护撞名维持观察；O92-02/M87-01/O90-01/契约 17/契约 20 均维持历轮结论（零新依据）。
5. **新契约角度**：手动操作状态迁移穷举闭环无缺口；DB 并发写与事务边界（单写者锁/Restore 顺序/迁移流程）全正确。
6. **新发现仅 B104-01 一条「知识位」级**：揭示了历史审查的一个误读——手动路径"存在性≠同一性"的泛化指控从未适用（api 手动路径无在飞窗口、删号+同名重建无法插入）；B20-01 存在性复核已封死。未来若引入异步手动报名（跨 goroutine 网络段），MarkDone/RemoveDone 才需同族升级 sameClientFor 指针身份。

工作树后端文件洁净。