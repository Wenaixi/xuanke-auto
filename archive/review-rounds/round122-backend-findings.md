# R122 后端只读审查报告

审查对象：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto\backend`（Go 后端）
审查方式：只读（零仓库文件修改；本轮唯一写盘为本报告文件）
审查时间：2026-09-23

---

## 一、分级发现

### 无阻塞级问题

无 P0/P1 级问题。身份防线矩阵、审计链、可观测性契约全部在位。

---

## 二、必查项逐项结论

### 1. 身份防线矩阵第三十七轮闭合（COUNT=1，零产品改动链第十七轮延续）

- **commit 核验**：`git log --oneline 20c5882..HEAD -- backend/` 仅 `57bf401` 一条——COUNT=1，零产品改动链第十七轮延续成立。
- **57bf401 内容核验**：B110-01 修复（手动报名/退选失败分支补库内 AppendLog），改动仅 `handler.go`（+26 行）与 `handler_test.go`（+133 行），非产品逻辑改动，符合零产品改动链语义。
- **sameClientFor 零漂移**：定义 `scheduler.go:204`（持 s.mu），7 调用点 `:850（probe 回写段）/:1489（失效分支）/:1521（成功分支）/:1551（风控分支）/:1571（窗口关闭分支）/:1600（实时复核回锁后统一入口）/:1635（确证满员分支）`grep 实证全部在位，六分支 + probe 单点共 7 处，零漂移。
- **写点全家福抽查 5 类**（本轮换类：openTimeDetected/acctData/refused/reloginAt/reloginFail）：
  - `openTimeDetected` 写入点 `:856`/`:1108` 均在持 s.mu 锁内，且 probe 回写段 `:849` 锁内先 `sameClientFor` 指针身份复核（M88-01 防线实证）；写入用对齐钟 `nowAlignedLocked`。
  - `acctData[acct]` 写入 `:863` 同处锁内，受同一身份防线覆盖。
  - `refused` 写入点 `:634`（Load/Restore 段）与 `:2011`（RemoveDone）均持 s.mu；`RemoveDone` 内先 `ClientFor` 存在性复核（:1995）再写。
  - `reloginAt`/`reloginFail`：maybeRelogin 决策段持 `reloginMu + s.mu` 双锁，entry 先 `ClientFor` 存在性复核（:1208）；成功分支写回前再次 `ClientFor` 复核（:1254，B21-03 写回侧）。
- **偶发写点专项四类**：
  - `MarkTokenValid`（:1306）：持 `reloginMu→s.mu` 双锁，与 TokenValidFor/maybeRelogin 对齐锁序，杜绝手动登录与在途重登并发半态。仅 destroy issueSession 恢复路径调用（手动登录成功），无身份复核需求（新身份自身写入）。
  - `TryAcquireSubmit`（:1894）：持 s.mu，inflight 位在飞互斥，release 用 sync.Once 防双清。
  - `releaseFullIfFreedLocked`（:1776，需持锁）调用点 `:1451` 位于 spawnChain 持锁段；快照明确有余量才解封，空快照/课程不在快照中一律保持 full——窗口关闭后不重打报名接口防线在位。
  - `submitAll`（:1342）：链生成段持锁，逐账号 `ClientFor` 存在性复核（:1355），已删账号不入链。
- **手动四路 accountExists**：`handler.go :245（handleElectives 课程读）/:295（handleElectiveSelect）/:387（handleElectiveExit）/:563（handleState 状态读）`四路全覆盖，凭据表校验契约在位。
- **maybeRelogin 双侧**：决策侧 `:1208`（入口锁内先 ClientFor 存在性复核，B43-01）+ 写回侧 `:1254-1274`（成功分支先复核再写内存与落库 UpdateIDToken，B21-03）。双侧闭合。

### 2. OBSERVE-117-01 知识位第五轮确认在位

`scheduler.go:1254-1274` 逐行实证：重登 goroutine 成功分支首行 `:1254` 先 `ClientFor(acct)` 复核，随后 `:1265` 再次 `ClientFor` 取**当前注册表** client 的 `Token()`（:1266）落库 `UpdateIDToken`——落库取的是复核后当前注册表身份的新 token，绝非 goroutine 发起时快照的旧身份 token；写回侧结构保证链持续成立。第五轮零漂移。

### 3. OBSERVE-111-01 / 112-03 盯守第十一轮维持

- `handler.go:377` `_ = d.Sched.MarkDone(...)` 与 `:457` `_ = d.Sched.RemoveDone(...)`：读 `MarkDone`（:1922）/`RemoveDone`（:1989）本体确认返回 `error` 但**函数体内所有落库点全部 `if err := ...; err != nil { log.Printf }` 记录**（SaveSuccess:1973 / AppendLog:1976 / DeleteSuccess:2020 / SaveRefused:2026 / AppendLog exit:2029），错误永不静默——`_ =` 接收的恒 nil（函数体仅返回 nil），非吞错。手动成功路径的审计行也在此两函数内（MarkDone:1976 AppendLog success / RemoveDone:2029 AppendLog success）。
- 双文案动作维度：select 路径 `:361-368`（read 文案）+ `:370-374`（业务失败文案）；exit 路径 `:440-446` + `:448-452`，两动作对称（本轮实证：exit 分支也带 6 行失败落库）。select/exit 双维度无口。

### 4. B110-01 审计链第十二轮维持 + 本次修复归入零漂移

- 手动失败六处 AppendLog：select `:352 / :364 / :371`，exit `:433 / :442 / :449`，全部 `if err != nil { log.Printf }` 无吞错；动作维度 select/exit + 失效/read/业务 3 分支付 6 分支全覆盖。
- 成功路径审计行：手动成功经 MarkDone/RemoveDone 内部 AppendLog success（:1976/:2029）；登录 set_targets/logout/config/delete_account 各成功路径 AppendLog 在位（:126/:227/:548/:606/:854/:1045）。
- **零吞错穷举扫描**：`grep "_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)"` 全仓**零命中**；非测试代码 `_ = ` 模式仅剩 10 处（handler 2 处恒 nil 上述 + config os.Setenv/os.MkdirAll/os.WriteFile 3 处基础设施 + Prewarm io 2 处 + ProbeForAccount 1 处 + io.Copy Discard 2 处），无一为业务落库吞错。
- B110-01 修复后自动链（scheduler）与手动链（handler）失败审计完全对称：自动链失败 7 处 AppendLog（:1504/:1532/:1558/:1614/:1662/:1770 + tick 段），手动链失败 6 处，双侧同语义。**本轮 B110-01 修复使审计链覆盖缺口闭合，观察项转正为契约，第十二轮保持并强化。**

### 5. O105-01 抖动基线零漂移

- `socketPreheat`/`readyProbe` 夹具：zhidao `http_test.go` socket 预创建 + api `handler_test.go` readyProbe（:139/:179，5 次重试）+ accounts `manager_test.go` readyProbe（:26 定义，:87/:164/:247 三处调用）全在位零漂移；manager_test 明确注释"zhidao/api 有 socket 预创建，accounts 8 轮全绿实证无残余，未来若 flake 第一候选补 socketPreheat"。
- **定向 race 实测**：`go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` → `ok` 3.221s + `ok` 1.898s，**通过**。

### 6. 新契约角度：限流与退避的可观测持久化（本轮走查结论）

**可观测通道已完整闭环**：三处退避全部以 `task_log` 失败审计行对外暴露，持久化后管理员可经 `/api/logs` 核对谁在退避、何时触发、为何退避：

| 退避机制 | 入账 | 时间窗 | 可观测痕迹 |
|---|---|---|---|
| rateLimited 30s 风控退避 | `markRateLimitedLocked`（:1710，`nowAlignedLocked().Add(30s)`） | 入账即 AppendLog 失败行 `:1558`；退避期 spawnChain `isRateLimitedLocked` 跳过 | `task_log` 行「账号 X: 触发平台风控退避 30s: ...」+ `/state.courses.status=failed` + Result 文案「触发平台风控退避 30 秒」 |
| reloginAt 30s 重登节流 | maybeRelogin `:1231` | 期间退避窗口未过则跳过；成功/失败都刷新 | 触发时 `:1235` 日志「触发自动重登」+ 失效 AppendLog（手动 `:352/:433`、自动 `:1504/:1614`）+ `/state.token_valid=false` |
| probe 30s 节流闸门 | `probeIntervalFar`（:63）+ probeAt 记录 | 全校 30s 节流 + probing 单飞 + probeSem(4) | 无 task_log 行（纯测量节流非业务失败，无审计必要），但探测结果经 openTimeDetected/acctData 反映到 `/state.open_time` 与 `/electives`；加入探测量日志非必需 |

**判定**：rateLimited/reloginAt 两类业务性退避均已落 task_log + state 双通道可观测，管理员在 /api/logs 能看到完整退避时间线（入账时刻 + 文案 + 后续恢复成功的 success 行）；probe 节流属内部测量防护无需日志。**无缺口**。另实证一个细节：`probeAt` 节流非独立 map——探测频率由 tick 内 `probeIntervalFor` 输出间隔与 probing 单飞施加，无需持久化可观测。

---

## 三、契约 20 轮次标签扫描

- grep 轮次前缀标签模式（"（第 N 轮）/roundN/R12x（第 N 轮）"）于 Go 源码与测试（排除二进制）：**零命中**。
- 新提交 57bf401 注释中「B110-01」属契约编号引用（"为什么/契约"语义），非轮次循环标签，符合契约 20。
- session/store.go:117 引用「docs/review-round13.md」为文档文件名引用，非代码内轮次前缀标签，合规。

---

## 四、验证表

| 验证项 | 命令/方式 | 结果 |
|---|---|---|
| go build | `go build ./...` | PASS（exit 0） |
| go vet | `go vet ./internal/scheduler/ ./internal/api/ ./internal/accounts/` | PASS（exit 0） |
| 定向 race | `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` | PASS（zhidao 3.221s + accounts 1.898s） |
| 手工/自动失败审计对称（B110-01 TDD） | 读测试 TestManualElectiveFailureAppendsLog / TestManualElectiveReadErrAppendsLog | 断言齐全（select+exit 双动作、业务失败+read 双形态、校验 !IsOK 失败行），修复后合同成立 |

注：本轮 `go build`/`vet`/race 均实际执行通过；未跑全仓 `go test ./...`（时间盒内定向核验），定向两包 + build/vet 已覆盖清单要求。

---

## 五、总结

R122 后端闭环三连：身份防线矩阵第三十七轮闭合（COUNT=1，零产品改动链第十七轮延续）；B110-01 修复将手动失败审计从"零留痕"转为"六分支全落库"，自动/手动审计链完全对称，第十一轮观察项转正；OBSERVE-117-01 知识位第五轮、OBSERVE-111-01/112-03 第十一轮、O105-01 抖动基线均为零漂移维持。新契约角度走查确认退避可观测性闭环。零阻塞问题。