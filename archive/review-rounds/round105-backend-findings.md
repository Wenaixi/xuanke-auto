# R105 后端只读审查报告

审查对象：xuanke-auto HEAD `f8dc077`（R104 收官收尾轮：纯文档提交，仅 archive/review-rounds 三个文件，backend/ 零代码改动——R103 起连续三轮后端零改动，最近一次后端产品代码为 `20c5882` B101-01 修复）。工作树洁净。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round105-backend-findings.md）。

审查方式：Read / Grep / Bash 只读命令（定向 go test -race / go vet / 逐点走读）。核心走读范围：身份防线矩阵第二十轮（全家福写点 grep 实证 + spawnChain 六分支 + 第七分支 + maybeRelogin 双侧 + PurgeAccount + api 手动路径 + 探测三入口）、O104-01 抖动基线第二十轮夹具走读、B101-01/B102-01/B103-01/B104-01 持续盯守（六测试 race 实测 + 全 http 直调点枚举 + 通道脱敏复核）、既往观察项延续（O104-02/O92-02/M87-01/O90-01 + 契约 20/17 强扫）、窗口判据与状态展示新契约角度（beginTimes 识别槽 / windowClosedLocked 三判据 / /api/state 与 /api/admin/stats 前后端字段消费一致性 + 多账号并发全局资源边界）。

## CRITICAL

无。

## MAJOR

无。

## MINOR

### B105-01：/api/admin/stats 的 `open_time_set` 只表达"识别槽有值"，与 expired 识别值在管理后台长显"可点的已过去时间"——默认级低优先（走读 + 走读，OBSERVE-104-02 保持）

**位置**：`handler.go:898-902/:964`（`open := d.Sched.RecognizedOpenTime()` → `openTimeStr` 零值空串 + `open_time_set: !open.IsZero()`）+ `scheduler.go:440-445`（RecognizedOpenTime 返回"识别值本身，不做过期截断"）+ `Admin.tsx:735`（`value: s.open_time_set !== true ? "未识别" : s.open_time`）。

**事实确认**：后端识别值绝不截断是正确契约（决策锚 1/17：tick 提交守卫/TODO 判据都直接消费 open 过去值，这里截断会让开窗点后 `open` 恒零、提交循环被第一守卫永久挂起）。`open_time_set` 语义 = "识别槽存在"（含过期值），与 stats 注释 `开放时间=识别态唯一事实源` 一致。前端 Admin 三态横条另用 `window_closed/window_opened` 三态判断，**不**依赖 open_time_set；Dashboard 学生端用 `open_time_known`（StateForAccount 内 **依 now 判定** `!st.OpenTime.IsZero() && st.OpenTime.After(nowAligned)`，过期即 false）→ 学生端不会把过期旧值当当前开放时间。

**遗留不对称点**：管理后台 `open_time_set` 恒 true（识别过 + 过期），`识别开放时间` 行会**永久显示已过去的开窗时刻**（历史批次残留），若运维不看日历时间会把过期值当"当前开放时间"。学生端 `open_time_known` 的"依 now 判定过期"语义未同步到 stats 字段。影响：展示误导，无调度/安全后果；识别槽保留是"关闭≠时间消失"契约要的。**五代无新依据**（O104-02 维持观察）。

### B105-02：admin stats `token_valid` 汇总未标"半态"，调度器半态是刻意的（TokenValidFor 语义即含 relogging）；api 手动路径无在飞竞态——全部维持 R104 定案（OBSERVE，知识位性）

**位置**：`handler.go:941-946`（`tokValid` 每账号 `d.Sched.TokenValidFor(a)`）+ `scheduler.go:738-741`（`tokenValidForLocked = !s.tokenValid[acct] && !s.relogging[acct]`，重登进行中/正在退避都算失效，前端显示"已失效·自动恢复中"）。

**确认**：TokenValidFor 把 `relogging` 半态并入"失效"，前端 /state `token_valid=false` → "已失效·自动恢复中"横幅，这个文案已经把半态传达给用户；admin stats 的 token_valid 直接复用同一方法、同一语义（无半态位字段），与"管理员一眼看 token 状态"目标一致。**非缺陷**，维持知识位：若未来要展示"半态"需为 stats 增加 relogging 专用字段，当前无。

### B105-03：R104-B104-01 定案后，手动报名由"异步躺平"变为"同步退避"形态的事实已闭合；probeSem 常驻 cap=4 有 ponytail 注释明示——全部维持 R104 结论（知识位性）

**位置**：`manager.go:218-236`（gateTryAcquire 非阻塞）、`handler.go:132`（LoginByPassword 收口）、`scheduler.go:1068-1074`（probeSem cap 4 + ponytail 注释）、`scheduler.go:1276-1281/678-682`（reloginResults cap 8 非阻塞 select+default）。

## OBSERVE（延续观察）

### O105-01（O104-01 抖动基线第二十轮）：夹具走读无新脆弱点，定向跑全绿零 flake（走读 + 实测）

socketPreheat（client_test.go:25 + captcha_test.go:17）+ readyProbe 宽栅栏（api handler_test.go:179/zhidao captcha_test.go:29）布局与 R104 逐字符一致，本轮零改动。**未发现新时序脆弱点**。定向跑（zhidao 脱敏 race、scheduler 身份防线族+窗口守卫 race、api 手动五测+删除/穿透 race、session/db race、go vet 三包）全绿零 flake。基线口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（~13%）。主控跑全量 race 吸收结论。

### O105-02（O104-02 删除保护撞名延续复核）：handler.go:992 单判据与 B43-04 双条件不对称——维持观察，无新依据提级

handleAdminDeleteAccount（:992）`acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只在账号名等于 `AdminNameValue()` 时拦截；撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下防护只有名字外壳。历轮三条低优先级理由（防空删管理员自己的硬护栏语义 / requireAdminSession 前置 / 撞名正常登录语义已封）有测试锚定（TestAdminDeleteProtectsRenamedAdmin PASS）+ 实测无新依据，**维持观察**。

### O105-03（O92-02 logintest 引擎判定源分叉）：维持关闭（走读）。

### O105-04（M87-01 窗口延续复核）：维持 MINOR + 注释兜底（走读）。

### O105-05（O90-01 CRLF 延续复核）：维持（实测 go vet 三包全干净 VET_EXIT=0）。

### O105-06（契约 17 零吞错延续扫查）：通过（走读）。新增 `handleAdminStats` 目标数失败族（:905-925）不静默计 0、明确 500 报错——无 `_ =` 落库点。

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第二十轮闭合**：全仓**写点全家福 grep 实证**——`s.openTimeDetected[acct]=：856 / ["*"]=：1108 / s.acctData[acct]=：863 / s.tokenValid[acct]=true:1241 / false:1262 / s.done[acct][id]=true:1529/MarkDone:1934/RestoreDone:615 / s.refused[acct][id]=true:634/RemoveDone:2011/RestoreRefused:630 / s.rateLimited[acct][class]=：1714 / s.full[acct][class]=true:1767 / s.inflight[acct][class]=true:1471/TryAcquireSubmit:1905 / s.state.Courses[status/resulg]：1450区-1860区（setStateLocked 统一）+ MarkDone:1961/RemoveDone:2014 / PurgeAccount 全量清 :495-517`。逐一对应身份防线：probe 回写段 :850（sameClientFor + 识别槽双条件 :855-858 空快照不删槽）✓ / maybeRelogin 决策侧 :1208（ClientFor 复核）写回侧 :1254（先清 relogging 再剔删除竞态、成功分支内存写+UpdateIDToken 落库同门）✓ / spawnChain 六分支 sameClientFor :1489/:1521/:1551/:1571/:1600/:1635 + 第七分支 ErrUnauthorized 在 :1600 统一复核后、:1627 doneHas 胜利状态让位、:1645 doneHas 对称 ✓ / MarkDone :1927（存在性）+RemoveDone :1995（存在性）✓ / SubmitAll :1355 + 链顶 :1388 + 取 client 双判 :1405/:1409 ✓ / api 手动路径四路 accountExists 前置 :245/:295/:373/:537 凭据表 + B104-01 知识位（手动路径无在飞窗口，B20-01 已封死）✓。**偶发探测入口**：ProbeForAccount 失效 :822 → maybeRelogin 决策侧复核兜底、ProbeNow :954 与 probe :1091 同理。**无新裸露写点**。round41 测试族 + probe_identity_test 观测在绿。 | 通过（走读 + 定向 race 实测 4.56s） |
| **B101-01/B102-01/B103-01/B104-01 持续盯守**：zhidao 六测试 `-race` 实测全绿（TestSanitize 五测 + TestIsReadErrCoversAllForms）。doRequest 仍唯一携 token URL 通道（:450 经 sanitizeError 闭环）；`grep access_token\|idToken` 在 api/handler 零命中——透传路径零 idToken 拼接；B102-01 五点 http 直调 URL 天然静态（无 idToken）。B103-01 活化条件（平台下发 maxCount → IsClassFull 判据 `ce.MaxCount > 0 && ce.SelectedCount >= ce.MaxCount`）在位、classFullRealtime 仍锁外 :1589/:1751 无持锁网络；B104-01 知识位无退化。**维持无回归**。 | 通过（实测全绿） |
| **新角度 b：窗口状态展示契约全链路**（/api/state + /api/admin/stats 各字段消费端一致性）：**判定正确性全链路成立**——beginTimes 识别：探测成功才写槽、空快照绝不清槽（`s.openTimeDetected` 在 acctData 写前条件守卫,注释"关闭≠时间消失"）；state 向下广播 `open_time_known` 依 now **判定过期**（scheduler.go:410-438/714-716），Student 端 Dashboard.tsx:184-190 消费 `open_time_known && open_time`，识别过期=null→全 00 过期态，与折叠列表 begin_times 兜底并存 #F39-N1 在位；admin stats `window_opened/window_closed` 同源 `d.Sched.WindowOpened()/WindowClosed()`，Admin.tsx:735-740 三态（待命中/已开放/已关闭）依赖 window_closed 补发（B39-05 在位）；windowClosedLocked 三判据单源 :903-917（主判据/时钟连续失败≥3+已过/EmptyProbeRuns≥3+已过）供 /state 与 /api/admin/stats 同源，其内主判据 `s.state.WindowClosed` 仍由单判定写入，**无分叉**。**再见不对称**：B105-01。 | 通过（走读 + 定向实测） |
| **新角度 a：多账号并发全局资源边界**：**全部为锁保护且偏安全**——`probeSem`（cap 4 信号量）`<-s.probeSem` 阻塞天然排队绝不死锁、极端 N 账号并发时等待者排在信号量上（非 tryAcquire）；`reloginResults` cap 8 + `select+default` 非阻塞（scheduler.go:1278-1281）已防僵尸通道溢出；`gateWait/gateTryAcquire` 同一 gateMu 串行化、gateLoginPerMin=2（manager.go:161）+ 收口整链路；共享连接池自愈族已验证；`lastSubmit`（scheduler.go:1346）锁内写、submitAll 在读 recent 无并发写。**开窗点单值与窗口判定一致性**：probe 用单 open 快照（tick 开头 :977/:987 取一次 + windowClosedLocked 内单快照 :909），提交决策 `open.IsZero() && !opened` 挂起（scheduler.go:1011）、WindowOpened 与识别槽双通道自洽（B43-01 语义统一）；tick 开头 `nowAligned` 单快照复用三处判据 :977/:987/:989 无亚毫秒热改窗口。**崩溃安全**：probeSem 操作非临界无死锁；band经多 buffer 有界——无单向依赖环可证无死锁（Walk：probeSem→ProbeForAccount→可能 maybeRelogin→可能有 Login goroutine→可能写 reloginResults：与主循环 select reloginResults 互不持锁阻塞）。 | 通过（走读 + 定向 race） |
| **契约 20 强扫**：全仓生产代码 grep `\b(B\|M\|O\|R\|F)[0-9]{2}-[0-9]{2}\b` 零命中（本轮加了 `head -0` 排除后仍零输出）；`第 N 轮` 字面量零命中。**无违规需上报**。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test ./internal/zhidao/ -run 'TestSanitize\|TestDoRequestSanitizes\|TestIsReadErr' -count=1 -race -v` | 全绿（脱敏五测 + 判型实测 PASS，2.15s） |
| `go test ./internal/scheduler/ -run 'TestDeletedAccountRebuiltSameName\|TestDeletedAccountManualInFlight\|TestProbeForAccount\|TestWindowClosed\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestPurgeAccount' -count=1 -race` | 全绿（身份防线族 + 窗口守卫 + 提交，4.56s） |
| `go test ./internal/scheduler/ -run 'TestWindowClosed' -count=1 -race -v` | 全绿（5 测逐条 PASS） |
| `go test ./internal/api/ -run 'TestAdminDeleteProtectsRenamedAdmin\|TestAccountOverride\|TestElectiveSelectRejectsWindowClosed\|TestElectiveSelectRejectsFullClass' -count=1 -race -v` | 全绿（删除保护 + 穿透 + 手动复核） |
| `go test ./internal/api/ -run 'TestHandleElectivesSelectAndExit\|TestHandleElectivesSelectUnauthorizedRelogin\|TestHandleElectivesSelectReadErrMessage\|TestElectivesSnapshot\|TestAdminElectivesUnknownAccountRejects' -count=1 -race -v` | 全绿（手动报名/退选全路径 5 测） |
| `go test ./internal/session/ ./internal/db/ -count=1 -race` | 全绿（会话过期/吊销 + db 迁移） |
| `go vet ./internal/{scheduler,zhidao,api}/` | 通过（VET_EXIT=0） |
| `git status --short` / `git log --format=%h -3` | 工作树干净；HEAD=f8dc077（R104 纯文档轮，backend/ 零改动） |

## 结论

1. **身份防线矩阵第二十轮闭合**：全仓写点**全家福 grep 实证**——spawnChain 六分支 + 第七分支（:1600 统一 `sameClientFor`、:1627 doneHas 让位、:1645 对称）全部在位；探测三入口（Probe/probe/ProbeNow）失效时 maybeRelogin 决策侧复核兜底；MarkDone/RemoveDone/SubmitAll/PurgeAccount/api手动四路全部复核通过。**无新裸露写点**。
2. **O104-01 抖动基线第二十轮**：夹具走读无新脆弱点，定向跑多批多包全绿零 flake，口径维持（~13% 低频残余由包序 + 冷启动窗口主导）。
3. **B101-01/B102-01/B103-01/B104-01 持续盯守**：六测试 race 实测全绿无回归；doRequest 仍唯一携 token URL 通道且闭环（api 层零 idToken 直拼）；B103-01 活化条件在位；B104-01 知识位无退化。
4. **既往观察项延续**：O104-02 删除保护撞名维持观察；O92-02/M87-01/O90-01/契约 17/契约 20 均维持历轮结论（零新依据）。
5. **新契约角度**：窗口状态展示加状态判定全链路正确（beginTimes 识别/state 过期语义/Dashboard 消费/admin stats 三态同源/三判据单源/胜利状态让位族），仅发现一处展示层遗留不对称（B105-01：admin `open_time_set` 不区分识别过期）+ 一条知识位固化（B105-02）。
6. **新发现仅 B105-01（MINOR，展示层）+ B105-02/B105-03 两条知识位**：管理后台 stats 的 `open_time_set` 语义是"识别槽存在"而非"识别当前有效"——学生端 `open_time_known` 有依 now 过期判定，stats 缺同语义，窗口关闭后管理后台"识别开放时间"行永久显示已过的历史开窗时刻（展示误导，无调度/安全后果）。

工作树后端文件洁净。