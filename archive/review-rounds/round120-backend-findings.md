# R120 后端只读审查报告（身份防线矩阵第三十五轮）

## 头部

- **审查对象**：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`（Go 后端 `backend/`）
- **审查 HEAD**：`e970b61`（R119 收尾总结，master，`git status --short --branch` 仅 round120-frontend-findings.md 未跟踪、无产品改动）
- **审查方式**：绝对只读——零仓库文件修改。`git log` 实证提交历史、`grep`/`Read` 逐点核位、后台定向 `go test -race` 实证行为、`go build`/`go vet` 实证编译。唯一写入 = 本报告文件。
- **聚焦范围**：聚焦清单 7 项全量执行（身份防线矩阵第三十五轮闭合 / OBSERVE-111-01+112-03 盯守第九轮 / B110-01 审计链第十轮 / OBSERVE-117-01 知识位第三轮 / B110-02 观察复核 / O105-01 抖动基线 / 新契约角度 = 登录链路与限流闸门纵深走查：config→runtime→识别引擎（native_ocr/local_ocr/stub）→secure→doLogin 频率闸门→会话/激活票据→基础设施状态码家族）。

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**
**OBSERVE：延续 4 项（111-01/112-03 维持、117-01 知识位确认在位、B110-02 维持、O105-01 维持）**

本轮 7 项必查全部通过/维持/在位，无证据够格立任何缺陷条——稳定期审查价值确认为「无新证据」。（注：`round120-frontend-findings.md` 为并行前端代理产物，非本报告。）

## 必查项逐条结论

### 1. 身份防线矩阵第三十五轮闭合 —— 通过（零产品改动链延续第十五轮，COUNT=1）

**COUNT 实证**：`git log --oneline 20c5882..HEAD -- backend/` 仅 1 条 `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志），与 R110-R119 连续十四轮同一条。**零产品改动链延续第十五轮，COUNT=1**。

**sameClientFor 7 调用点逐点实证（行号与任务清单逐字符零漂移）**：

| 调用点 | 行号 | 身份防线角色 | 实证 |
|--------|------|--------------|------|
| 定义 | `scheduler.go:204` | 注释 :199-203 接口值反射指针身份比对 + nil/已删非同一 | Read 实证 `current==nil→false`（:206）+ `clientIdentity` 比较（:209） |
| ProbeForAccount 回写段 | :850 | M88-01：网络往返后锁内复核，非同一身份整体放弃快照/识别槽写回 | Read 实证 `if !s.sameClientFor(acct, client) { Unlock; log; return data, nil }`（:851-854） |
| 失效分支 | :1489 | 复核不通过先 delete(inflight) 再 return；maybeRelogin 调用落在复核之后（:1499） | Read 实证 |
| 成功分支 | :1521 | 写 done/state/SaveSuccess/AppendLog 前复核，非同一身份静默放弃写成功落库 | Read 实证（:1526-1540） |
| 风控分支 | :1551 | markRateLimitedLocked 前复核，绝不落退避/状态/日志到重建身份 | Read 实证 |
| 窗口关闭分支 | :1571 | markFullLocked 前复核，防假满员永久退避写进重建身份 | Read 实证 |
| 实时复核统一 | :1600 | classFullRealtime 网络段回锁后先复核再进三路分支 | Read 实证（注释 :1596-1599 明确与五分支对称） |
| 确证满员分支 | :1635 | doneHas 胜利状态让位后、markFullLocked 前复核 | Read 实证 |

**写点全家福 12 类逐一对应防线**（主控裁定维持全量核位后逐点实证，本轮与前轮行号体系一致）：

- `openTimeDetected[acct]`：写 :856（ProbeForAccount 锁内 sameClientFor 复核后，读实证 :850→:855-858）、全校槽写 :1108（probe()，持 s.mu）；清 :506 PurgeAccount。无裸写。
- `acctData[acct]`/`acctDataAt[acct]`：写 :863/:864（锁内复核后）；清 :504/:505。无裸写。
- `tokenValid[acct]`：写 :1241 置位（maybeRelogin 决策侧 :1208 ClientFor 存在性复核之后）、:1262 清零（重登写回侧 :1254-1258 复核之后）；清 :511 + MarkTokenValid :1316。双闭合。
- `reloginAt[acct]`：写 :1231（决策侧 time.Now()）/ :1261（写回侧刷新）；清 :512 + :1226 退避窗口已过。见必查项 5。
- `reloginFail[acct]`：写 :1233（决策侧 ++，锁内复核后）；清 :1259（成功 delete）+ :513 PurgeAccount + MarkTokenValid :1317。无裸写。
- `relogging[acct]`：写 :1236 置位（决策侧）；清 :1247（goroutine 开头恒清）+ :514 PurgeAccount + MarkTokenValid :1318。无裸写。
- `inflight[acct]`：写 :1469-1471（spawnChain 锁内置位）/ :1896-1905（TryAcquireSubmit）；清 :1490（失效分支复核前）/ :1501/:1513（锁内）+ MarkDone :1950-1951 + RemoveDone :2002-2003 + PurgeAccount :502。无裸写。
- `done[acct]`：写 :1526-1529（成功分支 sameClientFor 复核后）/ MarkDone :1931-1934（ClientFor 复核 :1927 后）；清 RemoveDone :1999-2000 + PurgeAccount :499。无裸写。
- `refused[acct]`：写 :630-634（RestoreRefused）/ RemoveDone :2008-2011（ClientFor 复核 :1995 后）；清 MarkDone :1938-1940（同步 DeleteRefusedClass 清库内行 :1945）+ PurgeAccount :503。无裸写。
- `full[acct]`：写 markFullLocked :1760-1767（封面同一身份防线：:1555/:1575/:1639 均有 sameClientFor，:1458 在 spawnChain 持锁内）；清 releaseFullIfFreedLocked :1800 + MarkDone :1953-1954 + RemoveDone :2005-2006 + PurgeAccount :500。无裸写。
- `rateLimited[acct]`：写 markRateLimitedLocked :1710-1714（对齐钟写，仅在 :1551 风控分支同身份复核后）；清 MarkDone :1956-1957 + PurgeAccount :501。无裸写。
- `state.Courses` 状态行：写点全部持锁且在身份/存在性复核之后——spawnChain :1474 submitted / :1502 failed / :1530 success / :1556 failed / :1660 failed / 满员 :1768；MarkDone :1961-1970 success（复核后）；RemoveDone :2013-2015 pending（复核后）；ReleaseFull :1803-1804；清 :518 目标重建。无裸写。

**偶发写点专项**：
- `MarkTokenValid`（:1306-1319）：锁序 reloginMu→s.mu 对齐 maybeRelogin 决策段；只清 tokenValid/reloginFail/relogging 三个恢复标记、不写任何业务身份态；唯一合法入口 = issueSession 手动登录成功（handler.go:226）。spawnChain/手动报名退选路径只调 MaybeRelogin 绝不先调 MarkTokenValid（注释「delete reloginFail 击穿指数退避」防线在位，:345-347/:427-429）。无裸写。
- `TryAcquireSubmit`（:1896-1914）：持 s.mu，inflight 位占位 + `sync.Once` 幂等释放；`(nil,false)` 分支 handler 侧 `defer release()` 未注册不 panic（走读 handler.go:318-323/:410-415 实证）。无裸写。
- `releaseFullIfFreedLocked`（:1781-1811）：持锁；fullHas 守卫；只有快照明确余量（MaxCount>0 且 Selected<Max）才解封，空快照/课程不在快照恒保持 full（窗口关闭防轰炸）。无裸写。
- `SubmitAll`（:1342-1380）：持锁构建链，首行 per-account `ClientFor(acct)` 存在性过滤（:1355，已删账号彻底跳过）；`lastSubmit` 用 `nowAlignedLocked()`（:1346，与 tick 判读对齐钟同基）；冷启动无目标一次性警告（:1371-1374）。尾部统一走 spawnChain 身份防线。无裸写。

**手动四路 accountExists**（handler.go，全部实证在位，判据同源 `accountExists` :1096-1109 LoadCredentials 逐账号比对）：
- :245 课程读（handleElectives）
- :295 手动报名（handleElectiveSelect）
- :387 手动退选（handleElectiveExit）
- :563 状态读（handleState）
查无账号整体拒绝（"账号不存在，无法..."明确文案），四路全覆盖。

**maybeRelogin 双侧**：
- 决策侧 :1208 锁内 `ClientFor(acct)` 存在性复核（不通过即 unlock return，不写任何 map）。探测定时三处直调入口实证：ProbeForAccount :823 / ProbeNow :956 / probe :1094 + spawnChain 失效分支 :1499（前置 :1489 sameClientFor）/ 实时复核失效 :1610（前置 :1600 sameClientFor）+ handler 手动两处 MaybeRelogin :349/:431——全部入口收口到顶层锁内复核。
- 写回侧 :1254-1258（goroutine 开头先复核客户端仍存在，已删则整个成功分支含 `UpdateIDToken` 落库静默放弃、只清 relogging）+ :1263-1274（成功分支再取当前注册表 client 及其 `.Token()` 才落库）。

### 2. OBSERVE-111-01/112-03 盯守（第九轮）—— 维持（零漂移）

- `handler.go:377` `_ = d.Sched.MarkDone(...)` / `:457` `_ = d.Sched.RemoveDone(...)` 返回值丢弃**仍非吞错**：Read 两函数全函数实证（MarkDone :1922-1982 / RemoveDone :1989-2035），内部全部落库点 `if err != nil { log.Printf }` 包裹（SaveSuccess / DeleteSuccess / SaveRefused / AppendLog），返回 nil 恒成立；`:100` 处 `_ = s.RemoveDone("acct1",1)` 仅出现在测试 `TestStoreFailuresLogged`（scheduler_test.go:100，断言落库失败必记日志），非产品代码吞错点。
- 手动失效分支与自动链**同文案**：`"账号 <acct>: 教务令牌失效，自动重登中"` 手动（handler.go:352 select / :433 exit）与自动（scheduler.go:1504/:1614）逐字符一致；日志动作维度 `select`/`exit` 可区分（handler.go 全部 exit 动作 :433/:442/:449 + scheduler.go:2029 手动退选成功）。**零漂移。**

### 3. B110-01 审计链持续盯守（第十轮）—— 维持（零漂移）

- 手动失败三分支 AppendLog **六处在位，零漂移**：
  - 报名（select）：:352 失效 / :364 read 类 / :371 其余业务失败
  - 退选（exit）：:433 失效 / :442 read 类 / :449 其余业务失败
  六处全部 `if err != nil` 包裹记日志。
- 成功路径审计行未回归：MarkDone :1976（`AppendLog(acct, classID, "select", msg, true)`）+ SaveSuccess :1973；RemoveDone :2020 DeleteSuccess + :2026 SaveRefused + :2029 `AppendLog(...,"exit","手动退选成功...",true)`。Read 全函数实证。
- 自动链失败族审计行在位：失效 :1504 / 风控 :1558 / 实时复核失效 :1614 / 普通失败（含 read 类）:1662 / 满员 :1770。
- **零吞错点穷举扫描**：grep `_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)` 全仓（含测试）**零命中**；契约 17（落库失败记日志绝不静默吞错）全仓持续成立。

### 4. OBSERVE-117-01 知识位盯守（第三轮）—— 确认在位

- **关键性质核验**：maybeRelogin 写回侧（scheduler.go:1263-1274）成功分支先清内存标记（:1259-1262），随后才重新取当前注册表客户端（:1265 `client, ok := s.clients.ClientFor(acct)`）并读其 `.Token()`（:1266），非空才 `UpdateIDToken(acct, tok)` 落库（:1269）。Read 实证逐行在位。
- 支撑链持续成立：`ReloginIfNeeded`（zhidao/client.go:602-613）成功路径调 `c.Login(acct, pwd)`；`Login` 成功后在 `c.mu` 内更新 `c.token` 且同步 `c.cookies["zd_edu_cookie"]`（client.go:392-394）——重登完成后同一实例 `.Token()` 已为新值。`UpdateIDToken`（store.go:56-59）按 account 主键 UPDATE，落库值恒为重登后新 token，**不串旧身份/旧 token**。
- 知识位关键性质持续在位；若未来改动破坏「落库取当前注册表 token」需警觉（延续观察）。

### 5. B110-02 观察复核 —— 维持

`reloginAt` 写（:1231 决策侧 `time.Now()` / :1261 成功分支刷新 `time.Now()`）与读（:1219 退避窗口 `time.Since(t)` / :1227 30s 节流 `time.Since(t)`）**同基本地钟**：Read 完整函数实证，两写两读均在本地钟 `time.Now()` 基下；本地钟仅用于「退出到下一次尝试的间隔」相对比较，不参与开窗点判定/时间基对齐（后者归 `nowAligned`）。无混用，维持观察。

### 6. O105-01 抖动基线 —— 通过（定向 race 全绿）

- `socketPreheat()`（zhidao/client_test.go:25-30）：`net.Listen("tcp","127.0.0.1:0")` 立即 Close，逐字符实证；调用点 :40（TestMain）/ :52（loginMockServer）+ captcha_test.go:17/:52/:76 + sanitize_test.go:97 全在位。
- `readyProbe` 双版本：api 版（handler_test.go:185-216，10 次×200ms 轮询 + 显式 2s 超时总窗口 ~2s，调用 :139）；zhidao 版（client_test.go:88-，同宽栅栏，调用 :78/:164）；accounts 版（manager_test.go:26-，三处调用，注释明确「若未来 accounts 再出冷启动 flake 第一候选即补 socketPreheat」）。逐字符零漂移。
- **定向 `go test -race -count=1` 实测全绿**（详见验证表）：zhidao 2.505s / api 40.533s / scheduler 15.867s / accounts 2.016s / db 2.545s / store 27.246s——**零 connectex 抖动残余、零超时、零 race**。跑失败重跑绿的低频残余本轮未出现。

### 7. 新契约角度（本轮自选）：登录链路与限流闸门纵深走查 —— 通过（无新缺陷）

**登录链路全链（config→Login 链路）**：
- `config.Load`（config.go:51-70）：`XUANKE_OPEN_TIME` 读取已整体移除（注释 :55-57「配置层不再注入，避免识别槽为空时把过期日期当开窗点」——B40-02 死配置清除契约在位）；`loadDotEnv`「仅回填空值 + 不按 # 截断值」语义（:74-99，注释明确口令/密钥中合法 # 绝不截断破坏）。
- `ensureEnvFile`（:131-161）：首启自动生成随机管理口令（`randomAdminToken` :13-19 熵源故障拒绝启动）；模板不含 XUANKE_OPEN_TIME 行（:142-159 实证）。
- 登录链路（zhidao/client.go）：`fetchLoginPage` 网络瞬时抖动自愈一次（:292-323，纯 GET /login 不消耗验证码限额，F52-M2 在位）；`fetchCaptchaImage` 会话绑定（:326-346）；`submitLogin` 表单 `captcha/identification/uniqueId/priorityId` 四字段（:351-410，priorityId 空串契约注释载明「平台解析空串与缺键等价」）；RSA PKCS1 v1.5 加密 identification（rsa.go:17-36）；成功 `SetCredentials` 写账密供自动重登（client.go:270）。
- 识别引擎三态：`native_ocr.go`（windows+cgo，`//go:embed` ONNX+charsets+dll，懒加载释出 `%TEMP%\xuanke_ddddocr_assets`，`dumpIfDiff` 大小比对 :94-103，`Ocr:true+ModelDir` 官方 OCR 模式注释 :70-79）；`local_ocr.go`（Python 子进程桥接，30s 超时 + `show_ad=False`）；`native_ocr_stub.go`（非 windows/非 cgo 回退 false）。`applyCaptchaRecognizerFor`（router.go:33-55）ddddocr→内置原生→本地 Python→Vision 四级回退，`SetRecognizer` 同步模板（契约 18 在位，accounts/manager.go:209-216）。

**限流闸门族（doLogin 全局频率闸门）**：
- `gateWait`（manager.go:49-64 阻塞排队）+ `gateTryAcquire`（:223-235 非阻塞）+ `GatePump`（:68-77 每分钟翻页广播）+ `LoginByPassword`（:243-290 手动登录收口 gateTryAcquire，quota 满返回「登录尝试过于频繁」）+ `ResetGateForTest`（:82-87 仅测试）。`Relogin`（:166-175）内部明确调 `gateWait`——R40「接口间接触发指控必须追到真实实现」知识位持续在位。
- 登录/激活 HTTP 层限流桶（handler.go:1135-1191，`loginRate=5/60` + burst 5 + 桶 TTL 10min 惰性清理防 OOM）；`clientIP` 可信反代开关（:1193-1219，默认不信 XFF、回环才信最右非空值）。

**基础设施状态码家族核对（B40-01 家族）**：panic→500（recoverMiddleware :1221-1233，httptest 断言 `TestRecoverMiddlewareHidesPanicDetail` :590-617）；会话 401（requireAuth :1123，`TestAuthRequired` :312-328）；管理 403（requireAdminSession :623-637，`TestLoginAdminNameCollisionStudentCredential` :505-525）；限流 429（router.go:105-123，`TestLoginRateLimit` :1400-1428 断言 HTTP 429）；CSRF-403（requireJSONBody router.go:69-77 + `TestRequireJSONBodyRejectsFormContentType` :1593-1616 + `TestLoginRejectsFormContentType` :1562-1585）；SPA /api 前缀 404（router.go:194-199 + web/embed.go:22-31 双兜底，`TestApiUnknownPath404` :1665-1707 含真实 Server 端到端 CT 断言）。**家族齐全，零缺口。**

**会话/激活票据**：`CreateTicket`/`ConsumeTicket`（session/store.go:100-138，单次防重放 + 绑定账号 + 5min TTL）；`RevokeAccount`（:207-215 删号即吊销全部会话）；`randToken` 熵源故障拒绝签发（:224-231）。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline -8` / `git rev-parse HEAD` | HEAD=`e970b61`（R119 收尾），最近 8 条均 docs(review) 收尾 |
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 `57bf401` 一条（B110-01 审计修复），COUNT=1（零产品改动链第十五轮） |
| `git status --short --branch` | 仅 `?? ../archive/review-rounds/round120-frontend-findings.md`（并行前端代理产物，非本仓库产品改动） |
| `go build ./...` | BUILD_OK |
| `go vet ./...` | VET_OK（零输出） |
| `go test -race -count=1 ./internal/zhidao ./internal/api` | ok（zhidao 2.505s / api 40.533s），exit 0 |
| `go test -race -count=1 ./internal/scheduler ./internal/accounts ./internal/db ./internal/store` | ok（scheduler 15.867s / accounts 2.016s / db 2.545s / store 27.246s），exit 0 |
| Grep 证据族（sameClientFor 7 点 / 写点 12 类 / maybeRelogin 6 入口 / accountExists 4 路 / 手动 AppendLog 6 处 / exit 动作维度 / 零吞错穷举扫描 / 契约20 轮次前缀扫描） | 全命中，行号与任务清单零漂移；零吞错扫描全空 |

## 已核无缺陷清单

- 手动路径 TryAcquireSubmit `(nil,false)` 时 handler 侧 `defer release()` 未注册、无 nil 调用 panic；成功路径 deferred release 幂等（sync.Once）。
- `releaseFullIfFreedLocked` 对空快照/课程不在快照恒不解封，窗口关闭防轰炸残留闭环（注释与实现一致）。
- PurgeAccount 成败齐全性（scheduler.go:495-519）：12 类写点对应 map 键 + state.Courses 该账号行 + openTimeDetected 识别槽一并清除。
- maybeRelogin 决策侧入口先 ClientFor 存在性复核（:1208），写回侧 :1254 复核，双侧闭合；Writeback UpdateIDToken 取当前注册表 token（OBSERVE-117-01 关键性质在位）。
- doRequest code=-1 → ErrUnauthorized（统一鉴权归口）；httpDo 只重试 dial/write、read 错误不重试（幂等防线）；sanitizeError 剥 URL（B101-01）在位。
- 探测节流三件套（probing 单飞 + 30s 节流闸门 + probeSem cap 4）在位（读 :1049-1077 区）。
- 契约 17（token/密码只显前 8 位）：maskedToken（:1334）+ tokenShort（manager.go:319-324，len≤8 输出 "***"）+ :1283 `new token %s...` 脱敏在位。
- 契约 20（注释无轮次前缀标签）：全仓 `（第 N 轮）` 扫描仅命中测试注释「第 3 轮」字样（scheduler_test.go:1583-1586/:2442，为测试轮次描述的领域语言、非轮次决策前缀标签），产品代码零命中。
- `_ = s.RemoveDone("acct1",1)` 仅测试 TestStoreFailuresLogged（断言落库失败记日志的逆向用例），非产品吞错点。

## 结论

1. **身份防线矩阵第三十五轮闭合**：`20c5882..HEAD` 后端仅 57bf401（B110-01 审计修复），**零产品改动链延续第十五轮，COUNT=1**。sameClientFor 7 调用点行号零漂移；写点全家福 12 类逐一对应防线（含偶发写点专项 MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 四类零裸写复证）；手动四路 accountExists + maybeRelogin 双侧（决策侧 :1208 + 写回侧 :1254）全在位；六 err 归并分支身份复核成族齐备。**矩阵保持完全闭合，无新证据够格立条。**
2. **OBSERVE-111-01/112-03 盯守（第九轮）**：维持。handler.go:377/:457 `_ = MarkDone/RemoveDone` 恒 nil 非吞错（函数体落库点全日志化，:100 的 `_ =` 仅测试内）；失效文案与自动链同文案、select/exit 动作维度可区分。**零漂移。**
3. **B110-01 审计链（第十轮）**：手动失败三分支 AppendLog 六处在位，成功路径审计行（MarkDone/RemoveDone）未回归，自动链失败族审计行齐位，零吞错穷举扫描全空。**零漂移。**
4. **OBSERVE-117-01 知识位（第三轮）**：确认在位。重登写回侧取当前注册表 token 落库、不串旧身份，支撑链（ReloginIfNeeded→Login 更新实例 token→UpdateIDToken 按主键写新值）持续成立。
5. **B110-02**：reloginAt 本地钟读写同基自洽，维持观察。
6. **O105-01 抖动基线**：socketPreheat/readyProbe 逐字符零漂移；定向 -race 六包全量首轮即绿（zhidao/api/scheduler/accounts/db/store），零抖动残余、零超时。
7. **新契约角度（登录链路与限流闸门纵深）**：config 死配置清除、识别引擎三态回退、doLogin 全局频率闸门族、基础设施状态码家族（500/401/403/429/CSRF-403/SPA-404）核对齐全，无新缺陷。

**审视结论：无 CRITICAL/MAJOR/MINOR，新增 OBSERVE = 零。稳定期价值 = 确认无新证据够格立条，全部延续项维持零漂移。**
