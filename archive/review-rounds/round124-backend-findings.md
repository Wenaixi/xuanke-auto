# R124 后端只读审查报告

- 审查对象：`backend/`（Go 后端）
- 审查日期：2026-09-23
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R123 报告（commit e970b61 之后版本），`20c5882..HEAD -- backend/` 应有且仅有 `57bf401`。

## 一、逐项核验结果

### 1. 身份防线矩阵第三十九轮闭合 —— 零漂移

**git 基线（COUNT=1，零产品改动链第十九轮延续）**
```
$ git log --oneline 20c5882..HEAD -- backend/
57bf401 fix(api): B110-01 手动报名/退选失败分支补库内审计日志
```
仅一条，且为 B110-01 审计链补齐 commit（非身份防线新改动）。身份防线矩阵本轮零产品改动，纯观察轮。

**sameClientFor 定义与调用点零漂移**
- 定义：`backend/internal/scheduler/scheduler.go:204`（注释在 :199，`func sameClientFor(acct, chainClient Client) bool`，需持 s.mu）。
- 7 个调用点全数在位：:850（ProbeForAccount 回写）、:1489（失效分支）、:1521（成功分支）、:1551（风控退避）、:1571（窗口关闭）、:1600（实时复核入口六分支统一点）、:1635（确证满员）。spawnChain 六分支 + 探测回写各分支对称闭合，与手册契约 31/37 一致。

**写点全家福 12 类抽查（本轮换类 5 类，均读代码实证持锁 + 复核）**
- `acctDataAt[acct]`（scheduler.go:864）：probe 网络往返后回锁，先 `sameClientFor` 复核才写（M88-01 防线），并在 `s.mu` 持有下写入。
- `openTimeDetected[acct]`（:856）：同一回锁段内、身份复核后写入，锁内持有。
- `refused[acct]`（:634 RestoreRefused / :631-635 循环注入；:2011 RemoveDone 清理后置位）：全部 `s.mu.Lock()` 段内。RestoreRefused 是重启恢复序（不涉及在飞竞态，无网络往返）。
- `reloginAt[acct]`（:1231 发起重登置位、:1261 成功刷新）：maybeRelogin 内 `reloginMu` + `s.mu` 双锁段，且入口 :1208 先做 `ClientFor` 存在性复核（决策侧关闭 B43-01）；成功回写侧 :1254 再做存在性复核（写回侧，B21-03）。
- `tokenValid[acct]`（:1241 置 true、:1262 置 false、:1316 MarkTokenValid 删除）：全部双锁段内；MarkTokenValid 锁序对齐 reloginMu→s.mu（:1312-1315）。
- 结论：5 类写点全部持锁 + 存在性/身份复核后写入，零漂移。

**偶发写点专项四类**
- `MarkTokenValid`（:1306-1319）：`reloginMu.Lock()` 同步拿锁（非 defer 顺序），`s.mu.Lock()` 后 delete tokenValid/reloginFail/relogging —— 全双锁段，与 maybeRelogin 决策段锁序一致。
- `TryAcquireSubmit`（:1896-1914）：持 s.mu；置 inflight 位 + `sync.Once` 释放函数（release 再持锁 delete）。无竞态，幂等。
- `releaseFullIfFreedLocked`（:1781-1811）：持 s.mu 内部函数（调用点 :1451 均在锁内）。余量明确才解封、课程不在快照保守保持 full，防误解封。
- `SubmitAll`（:1342-1380）：持 s.mu 构建链列表，`lastSubmit = s.nowAlignedLocked()`（对齐钟，:1346），装配完释放锁再 spawnChain —— 锁外 spawn（减少持锁窗口），链顶重新取 client 并复核存在（:1388、:1409 双校验）。
- 结论：四类偶发写点行为均符合既有契约。

**手动四路 accountExists + maybeRelogin 双侧**
- handler.go:245（课程读 ?account= 校验）、:295（手动报名）、:387（手动退选）、:563（状态读）——四路全数在位且报错明确（"账号不存在，无法…"）。
- maybeRelogin 入口决策侧 :1208 + 回写侧 :1254 `ClientFor` 复核均齐，并有指针身份升级（spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin，B43-01 + B41-01 族）。

### 2. OBSERVE-117-01 知识位第七轮 —— 确认在位

`backend/internal/scheduler/scheduler.go:1254-1274` 逐行复盘：
- :1254 回锁后先 `ClientFor(acct)` 存在性复核，已删则放弃整个成功分支（含内存写与新 token 落库），只清 relogging。
- :1259 `err==nil && relogged` 才进成功分支。
- :1265 `s.clients.ClientFor(acct).Token()` 在**成功分支内重新取当前注册表 token** 落库（`UpdateIDToken`）——绝不使用发起时旧身份 token，不串旧身份。
- :1269 `UpdateIDToken(acct, tok)` 失败有日志（非零吞错）。
- `reloginFail` 清零 / `reloginAt` 刷新 / `tokenValid=false` 均在身份复核之后、同一持锁段。
- 结论：观察第七轮，结构性保证链成立，零漂移。

### 3. OBSERVE-111-01 / 112-03 盯守第十三轮 —— 恒 nil 非吞错

- `handler.go:377` `_ = d.Sched.MarkDone(...)`：MarkDone 返回 error 但内部已 log 全部落库失败（scheduler.go:1946/1952 等全部零吞错），no-op 返回恒 nil（其唯一非分散路径已在内部完整处理）。非吞错。
- `handler.go:457` `_ = d.Sched.RemoveDone(...)`：同上，RemoveDone 内部 :2021/:2027/:2030 三处落库失败全记日志。
- **动作维度对称**：select ：352（失效）/ :364（read）/ :371（业务失败）三处失败 AppendLog + 成功路径 :522（handleElectiveSelect 内 AppendLog / MarkDone 内 AppendLog）。exit ：433/:442/:449 三处失败 AppendLog + 成功 AddLog（RemoveDone 内 :2029）。select/exit 双动作 6 失败位全齐，B110-01 审计链完整。
- 结论：盯守第十三轮闭合。

### 4. B110-01 审计链第十四轮 —— 零漂移

- 手动失败六处 AppendLog：select :352/:364/:371，exit :433/:442/:449 —— 全部 `if err != nil { log.Printf }`，零吞错。
- 成功路径审计行：select 成功 → MarkDone 内 AppendLog（:1976）；exit 成功 → RemoveDone 内 AppendLog（:2029）。
- 自动链失败族齐位：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员切换 :1770。
- 零吞错穷举扫描：`grep -E "_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)"` 全仓零命中。专项复核 `_ =` fallback 写点：scheduler.go:1075 `_, _ = s.ProbeForAccount(acct)`（probe goroutine 内，无状态写入、无落库，可接受）；handler.go 无其他。
- 结论：审计链第十四轮零漂移。

### 5. O105-01 抖动基线 —— 零漂移 + 实测绿

- 夹具 `socketPreheat`（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97）+ `readyProbe`（zhidao/api/accounts 三包同款轮询式宽栅栏）均原样在位。
- `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/`
  ```
  ok  xuanke-auto/backend/internal/zhidao   3.614s
  ok  xuanke-auto/backend/internal/accounts 1.550s
  ```
- 结论：夹具零漂移。

### 6. 新契约角度：多账号并发调度一致性边界

tick 主循环（:973-1036）逐段走查多账号协调：

- **探测路径**：tick → lastProbe 全局节流闸门（probe 单飞）→ probe() 对 `AccountsWithTargets()` 每账号独立 goroutine + `probeSem`(cap 4) 封顶 per-account 探测并发（:1069-1077）。每账号独立 `acctData/acctDataAt/openTimeDetected/tokenValid` 识别槽。全局 FindElectives 主体在 per-account 全部入场后执行，其返回值只写 `lockData/lockDataAt`（全局帧 nth 兜底，:1064-1116）——写全局帧 + lastProbe 均在 s.mu 内。
- **提交路径**：submitAll 在 s.mu 内按账号收集链（先按 PublishID 分组、Priority 稳定排序），一次 `lastSubmit = nowAlignedLocked()` —— **lastSubmit 是全校共享提交节流闸门**，公平性由"每 tick 每账号每发布各 spawn 一条链"天然保证（tick 每 300ms 轮询，黄金期 250ms 时所有账号等量获得时间片），不存在某账号霸占提交带宽。
- **探测节流公平性扫描**：lastProbe 全局闸门 → 所有账号共享；probeSem cap=4 结构化信号量 → N 账号并发探测时排队公平（chan 先进先出）；多账号提交无全局锁外等待（spawnChain 锁内只做短路判断，网络往返在锁外）。唯一共享节流 = lastProbe（30s 全校闸门），低频账号不会因高频账号探测被挡——探测是全校一次、per-account 并行，开窗黄金期不受 30s 节流限制（probe 提交解耦契约已文档化）。
- **一致性边界结论**：多账号并发调度核心（探测：per-account 独立刷新 + cap4 信号量；提交：每 tick 每账号每发布一条链 + 全局 lastSubmit 公平时间片）无一致性漏洞。观察项：probeSem 的 cap=4 是全局的（含所有账号），若账号 >50 需上调（scheduler.go:1068 ponytail 注释已标注）——确定性边界已知且非本轮触发。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| 编译 | `go build ./...` | 通过 |
| 静态检查 | `go vet ./...` | 通过 |
| race 定向 | `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` | zhidao 3.614s ok / accounts 1.550s ok |
| 契约20 扫描（轮次前缀标签） | grep Go 源码 `(R...)/（第N轮）` | 零命中 |

## 三、分级发现

**无新增发现（零 P0/P1/P2）。**

唯一观察项（非本轮引入、确定性边界已知）：
- scheduler.go:1068 `probeSem` cap=4 全局封顶 per-account 探测并发，ponytail 注释已标注"若平台放宽熔断或账号数 >50 再调"——>50 账号部署时探测并发排队可能拉长单个账号探测等待，属可调参数的确定性边界，非缺陷。保持观察。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线（COUNT=1） | 通过，仅 57bf401 一条（B110-01 审计链补齐） |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635） |
| 写点抽查 5 类（换类） | acctDataAt/openTimeDetected/refused/reloginAt/tokenValid 全部持锁 + 复核 |
| 偶发写点四类 | MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 全契约符合 |
| 手动四路 accountExists + maybeRelogin 双侧 | 全数在位（:245/:295/:387/:563 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 7 轮确认在位（落库取当前注册表 token 不串旧身份） |
| OBSERVE-111-01/112-03 | 第 13 轮闭合（:377/:457 恒 nil 非吞错 + select/exit 双动作 6 失败位对称） |
| B110-01 审计链 | 第 14 轮零漂移（手动六失败位 + 成功行 + 自动族 + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 定向 race 双包全绿 |
| 多账号并发一致性 | 无漏洞（探测 per-account + cap4 信号量、提交每 tick 每账号一条链 + 全局 lastSubmit 公平时间片） |