# R117 后端只读审查报告（身份防线矩阵第三十二轮）

## 头部

- 审查对象：`E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto`（Go 后端 `backend/`）
- 审查 HEAD：`dffd608`（R116 收尾），工作区 clean（`git status --short` 空）
- 审查方式：只读。`git log 20c5882..HEAD -- backend/` 变更面核位、`grep` 逐点实证、`Read` 走读关键函数、定向 `go test -race -count=1 ./internal/api ./internal/zhidao`（分轮）。全程零仓库文件改动
- 聚焦范围：聚焦清单 7 项全量执行（身份防线矩阵第三十二轮 / OBSERVE 盯守 / B110-01 审计链 / B110-02 / O105 抖动基线 / task_log 保留 / 新契约角度 = scheduler tick 主循环状态机纵深）

## 分级发现

- **CRITICAL**：无
- **MAJOR**：无
- **MINOR**：无
- **OBSERVE**：延续 2 项 + 新 1 项（`OBSERVE-117-01`，知识位，无产品改动）

## 必查项逐条结论

### 1. 身份防线矩阵第三十二轮闭合 —— 通过（零产品改动链延续第十二轮，COUNT=1）

**变更面实证**：`git log --oneline 20c5882..HEAD -- backend/` 仅 1 条：

```
57bf401 fix(api): B110-01 手动报名/退选失败分支补库内审计日志
```

为 B110-01 审计修复（非产品改动），与前十一轮同一条。**零产品改动链延续第十二轮，COUNT=1**。

**sameClientFor 7 调用点行号零漂移**（grep `internal/scheduler/scheduler.go`）：

| 行号 | 位置 | 身份防线角色 | 实证 |
|------|------|--------------|------|
| :204 | 定义（含 `clientIdentity` reflect 指针） | — | 读实证：`current == nil → false`、`clientIdentity` 比较 |
| :850 | ProbeForAccount 回写段 | M88-01：网络往返后锁内复核，非同一身份放弃快照/识别槽写回 | 读实证：`if !s.sameClientFor(acct, client) { Unlock; log; return data, nil }` |
| :1489 | spawnChain 失效分支（ErrUnauthorized） | 六分支家族：同名重建旧链不写失效态/不触发重登 | 读实证：复核不通过先 `delete(s.inflight)` 再 return；重登调用落在复核之后 |
| :1521 | spawnChain 成功分支 | 六分支家族：不写 done/状态/库行到重建身份 | 读实证 |
| :1551 | 风控退避分支 | 六分支家族：不对重建身份落 rateLimited | 读实证 |
| :1571 | 窗口关闭分支 | 六分支家族：不对重建身份 markFullLocked | 读实证 |
| :1600 | 实时复核三路统一入口 | B43-02 第六分支：回锁后先同身份复核再分支 | 读实证：注释明确"成功/失效/风控/窗口关闭/确证满员五分支对称" |
| :1635 | 确证满员分支 | B39-01 族：doneHas 胜利状态让位后同身份复核 | 读实证 |

**写点全家福 12 类防线对应**（主控裁定维持全量核位后逐点实证）：

| 写点类 | 写点位置 | 防线 |
|--------|----------|------|
| `openTimeDetected[acct]`（识别槽） | :856（账号回写，锁内 `sameClientFor` 之后）；:1108（全校 `"*"`，锁内）；:507 PurgeAccount 删 | M88-01 身份复核 / 锁证 |
| `acctData[acct]` / `acctDataAt[acct]` | :863/:864（锁内身份复核后）；PurgeAccount 删 | M88-01 |
| `tokenValid` | :1241 maybeRelogin 决策侧置位（双锁）；:1262 重登成功清零（写回侧 ClientFor 复核后）；:1309 MarkTokenValid 删；:507 PurgeAccount 删 | B43-01 决策侧存在性复核 / B21-03 写回侧复核 |
| `reloginAt` | :1231（决策侧 `time.Now()` 写）；:1261（写回侧刷新）；读 :1219/:1227 | B110-02 同基（见必查项 4） |
| `reloginFail` | :1259 成功清零 / MarkTokenValid 删 / PurgeAccount 删 | B43-01 |
| `relogging` | :1245 置位 / :1255 goroutine 完成清 / MarkTokenValid 删 / PurgeAccount 删 | — |
| `inflight[acct]` | TryAcquireSubmit :1896 置位；release（once 幂等）删；spawnChain 各分支统一清位；PurgeAccount 删 | B20-01 在飞互斥 |
| `done[acct]` | :1528 spawnChain 成功 / MarkDone；:507 PurgeAccount 删 | `sameClientFor` / MarkDone 存在性复核 |
| `refused[acct]` | RemoveDone / MarkDone 删 + `DeleteRefusedClass` 同步清库内行 / PurgeAccount 删 | 手动落库复核 |
| `full[acct]` | `markFullLocked`（封面 `sameClientFor`）/ RemoveFull / `releaseFullIfFreedLocked` 解封（保守：空快照不解封）/ MarkDone 删 / PurgeAccount 删 | B39-01 族 |
| `rateLimited[acct]` | `markRateLimitedLocked` 对齐钟写（封面 `sameClientFor`）/ MarkDone 删 / PurgeAccount 删 | B39-01 族 |
| Courses 状态（`state.Courses[].Status/Result`） | `setStateLocked`（`statusIndexLocked` idx<0 守卫只挡越界、不挡 map 写） | 前置岗位均在锁内 `sameClientFor` / 存在性复核之后 |

**偶发写点专项**：
- `MarkTokenValid`（:1306）：`reloginMu → s.mu` 双取锁对齐 maybeRelogin 锁序，决策段内完成清除与重登发起/查询有效串行，三分支合法入口（issueSession 手动登录成功）。无裸写。
- `TryAcquireSubmit`（:1896）：`s.mu` 内判在飞+置位，release 内嵌 `sync.Once` 幂等；失败 `(nil,false)` 时 handler 侧 `defer release()` 未注册不 panic（走读 handler.go:321-344 实证）。
- `releaseFullIfFreedLocked`（:1781）：持锁；空快照/课程不在快照恒保持 full 不解封（防窗口关闭防轰炸残留），明确有余量才解封。测试锚点 :1878/:2264/:2292。
- `SubmitAll`（:1342）：`lastSubmit` 用 `nowAlignedLocked()`（与 tick 判读同基，契约注释 B 明确"本地时钟写 lastSubmit 会与对齐时钟判定基准混用"）；对每个账号先 `ClientFor` 存在性过滤再建链。

**手动四路 accountExists**：handler.go :245（课程读）/ :295（手动报名）/ :387（手动退选）/ :563（状态读），四路判据同源 `accountExists`（:1096 LoadCredentials 逐账号），查无账号整体拒绝并明确文案。全部实证在位。

**maybeRelogin 双侧**：决策侧 :1208 锁内 `ClientFor(acct)` 存在性复核（不通过即 unlock return，不写任何 map）；写回侧 :1254 goroutine 完成回锁后先 `ClientFor` 复核，已删则整个成功分支（内存写 + `UpdateIDToken` 落库）静默放弃只清 relogging。双侧闭合。

### 2. OBSERVE-111-01/112-03 盯守（第六轮）—— 维持（零漂移）

- handler.go:377 `_ = d.Sched.MarkDone` / :457 `_ = d.Sched.RemoveDone` 恒 nil 非吞错：读 `MarkDone`/`RemoveDone` 全函数实证，全部落库点 `if err != nil { log.Printf }` 包裹（SaveSuccess/DeleteSuccess/SaveRefused/AppendLog 均有日志），返回值 nil 仅出现在"账号已删"场景（已提前 Log"放弃落库"）。丢返回值不吞错、不破坏审计链。
- 手动失效分支与自动链同文案双日志：:352 `"教务令牌失效，自动重登中"`（select）/ :433 同文案（exit），与 scheduler 自动链失效分支文案一致；日志动作维度 `select`/`exit` 可区分。零漂移。

### 3. B110-01 审计链持续盯守（第七轮）—— 维持（零漂移）

handler.go 手动失败三分支 AppendLog 六处实证在位：
- 报选（select）：:352 失效 / :364 read 类（"报名请求已发出但响应读取失败"）/ :371 其余业务失败
- 退选（exit）：:433 失效 / :442 read 类 / :449 其余业务失败

全部 `if err != nil` 包裹记日志。成功路径未回归：`MarkDone` 内 `AppendLog("select", msg, true)` + `SaveSuccess`；`RemoveDone` 内 `AppendLog("exit", "手动退选成功...", true)` + `DeleteSuccess` + `SaveRefused`。全局落库点抽查：非测试代码 32 处 `if err := s.store./d.store./Store.AppendLog` 形态全覆盖，无 `_ =` 吞错落库点（含 B110-01 修复线全部调用点）。

### 4. B110-02 reloginAt 本地钟写读同基 —— 通过

- 写：`:1231  s.reloginAt[acct] = time.Now()`（决策侧）、`:1261  s.reloginAt[acct] = time.Now()`（写回侧刷新）
- 读：`:1219/1227  time.Since(t)`（决策侧退避/节流判定）

`reloginAt` 是纯本地节流/退避计时（不跨时钟语义、不参与开窗点判定），写读均 `time.Now()` 基，同基自洽。B110-02 观察持续成立。

### 5. O105-01 抖动基线 —— 通过（可定向跑测）

- `socketPreheat` 定义 client_test.go:19-27（预创建-关闭一个 127.0.0.1 回环套接字）逐字符核对在位；captcha_test :17/:52/:76 调用 + 信号量并发评议保留。
- api 包 `readyProbe` handler_test.go:179-（200ms×10 + 显式 2s 超时总窗口 ~2s）逐字符核对在位；manager_test.go:17-36 readyProbe 无 socketPreheat 双保险注释（"8 轮全绿实证无残余；若未来再出冷启动 flake 第一候选即补"）在位。
- 跑测（分轮，详见验证表）：zhidao round1 ok；api 第 3、4 轮 `ok 37.576s` / 无 FAIL。第 1 轮 TSAN OOM（error code 1455，api 首字节 fail）、第 2 轮 FAIL 47.073s 无具体失败测试名（机器压力形态）。按约定"跑失败但重跑绿记低频残余不立条"——重跑绿，不立条。

### 6. task_log 审计保留复核 —— 通过（设计在位，纯记录观察）

store.go DeleteAccount（:388-414）事务六表：credentials / accounts / targets / success / refused / activations，注释明确"报名日志保留（审计用途），仅重新登录即可重建凭据与客户端"。task_log 刻意不删。`LoadAllLogs`（:421-）保留读取。设计在位。未来若有人把 task_log 加入删除范围需警觉（延续观察）。

### 7. 新契约角度：scheduler tick 主循环状态机纵深 —— 通过（无新缺陷）

每轮换角落深度走查，本轮选 tick 主循环状态机（决策契约 1/2/3/32/40/41 落地面）：
- **open 零值补触发符号分析**：tick :992 补触发 `if !probe && now.After(open) && last.Before(open.Add(-time.Second))`——open 为零值时 `last.Before(open.Add(-1s))` 因 last 为真实时间恒 false，补触发恒不误触发（未识别开窗点不会因此 300ms 高频探测）；且 `probe := last.IsZero() || now.Sub(last) >= probeIntervalForOpen(...)`，`probeIntervalForOpen` 零值 open 返回 `probeIntervalFar=30s`（:63/:98 注释"未识别/识别过期=远间隔"，熔断形态降频）。逻辑自洽。
- **提交守卫三判据**（:1002-1033）：① `open.IsZero() && !opened` 挂起（B41-02 例外：WindowOpened=true 时释放，黄金期 250ms 不受零值误伤）；② `!opened && !now.After(open)` 未到点挂起；③ `WindowClosed()` 单源三判据（:918 `windowClosedLocked`：state 位 / syncFailStreak≥3 且 open 非零且已过 / 幽灵窗口 EmptyProbeRuns≥3 且从未开窗且已过——三判据共享单次 open 快照）。与契约 2/32 对齐。
- **EmptyProbeRuns 入账 10s 裕量对称**（:1156-数学）：`!opened && len(Publishes)==0 && now.After(open.Add(10s))` 才 +1，与判定侧 `WindowClosed` 主判据 `now.After(open.Add(10s))` 入账/判定双裕量对称；过渡态不误挂。
- **syncFailStreak 自愈语义**（:371-391）：失败先 `++` 后再判 `>=3`（杜绝旧实现"临界区 ++ 再清零导致值域恒 {0,1,2}"的死代码），同步成功统一清零；`clockOffset` 复位只发生在失败≥3 时。契约 19 成立。
- **探测节流三件套**（:1045-1089）：probing 单飞（持锁置位保原子）+ 30s 节流闸门（`lastProbe`，失败同样计入防网络风暴）+ probeSem(cap 4) per-account 并发封顶。与 CLAUDE.md 高性能架构一致。
- 无新缺陷发现；与契约文档零漂移。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline -6` | 最近 6 条均为 docs(review) 收尾，HEAD=dffd608 |
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 57bf401 一条（B110-01） |
| `git status --short` | 空（clean） |
| `go build ./...` | 首跑一次 cmd/bench VirtualAlloc OOM（errno 1455 机器压力），重跑 exit=0 |
| `go test -race -count=1 ./internal/zhidao` | ok 6.150s（与 api 同轮首跑时） |
| `go test -race -count=1 ./internal/api` ×4 | 第 1 轮 TSAN OOM(1455) FAIL；第 2 轮 FAIL 47s（无失败名，疑压力）；第 3 轮 `ok 37.576s`；第 4 轮 无 FAIL exit 0（组合 grep 实证）。重跑绿 → 记低频残余不立条 |
| grep/Read 逐点 | sameClientFor 8 行号 / 写点 12 类 / AppendLog 六处 / DeleteAccount 六表 / 偶发写点 / tick 状态机，全部实证 |

## 已核无缺陷清单

- 手动路径 TryAcquireSubmit 失败返回 `(nil,false)` 时 `defer release()` 未注册、无 nil 调用 panic；成功路径 deferred release 幂等（once），与 spawnChain 统一清位不冲突
- `releaseFullIfFreedLocked` 对空快照/查无课程恒不解封，窗口关闭防轰炸残留闭环
- `openTimeForLocked` 绝不做"识别过期截断"（决策契约 1），挂起判据只归"从未识别"；识别槽写入全持 `s.mu`
- 时钟对齐失败段：先 `++` 再判 `>=3`，成功统一清零自愈；`syncing` 无客户端路径复位（空库部署不永久休眠）
- `submitAll` 对已删账号先 `ClientFor` 过滤再建链，冷启动无目标一次性警告日志（`warnedNoTargets`）
- 窗口关闭判定不依赖脆弱错误文案，以探测状态单源推导（tick :1033 与 `windowClosedLocked`）
- 全校识别槽 `openTimeDetected["*"]` 写入持 `s.mu`（:1104-1107），与 tick 持锁读并发安全

## 结论

1. **身份防线矩阵第三十二轮闭合**：`20c5882..HEAD` 后端仅 57bf401（B110-01 审计修复），零产品改动链延续至第十二轮，COUNT=1。sameClientFor 7 调用点行号零漂移，写点全家福 12 类逐一对应防线（含偶发写点专项 MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll），手动四路 accountExists + maybeRelogin 双侧全部在位。**无新证据够格立条**，矩阵闭合。
2. **OBSERVE-111-01/112-03 盯守（第六轮）**：维持。`_ = MarkDone/RemoveDone` 恒 nil 非吞错，手动失效分支与自动链同文案且 `select`/`exit` 动作可区分，零漂移。
3. **B110-01 审计链（第七轮）**：维持，零漂移。手动失败三分支 AppendLog 六处在位，成功路径审计行未回归。
4. **新观察 OBSERVE-117-01（知识位）**：maybeRelogin 写回侧仅"存在性复核"而非"指针身份复核"（B21-03 安全审查定版即如此）。实证残余面分析：删号后同名重建场景下，旧重登 goroutine 成功写回会清掉新身份的 `reloginFail`/`tokenValid` 标记（`ClientFor` ok），但 `UpdateIDToken` 落库时重新取当前注册表客户端并读其 `.Token()`（非旧 goroutine 捕获 token，:1269-1274），**不会把旧 token 写进新身份**；残余面仅为新身份展示层瞬态（失败计数被清/有效标记被置），下轮探测或重登自愈。设计合理无需改动，列为观察以防未来改动破坏"落库取当前注册表 token"这一关键性质。

报告落盘完成。R117 后端报告已落盘 + 主要发现摘要：零 CRITICAL/MAJOR/MINOR；身份防线矩阵第三十二轮闭合（零产品改动链第十二轮 COUNT=1，7 调用点零漂移、写点全家福 12 类实证）；OBSERVE-111-01/112-03 盯守第六轮与 B110-01 审计链第七轮均零漂移；O105 定向 race 测试 api 中途两次环境压力 FAIL 后重跑绿（记低频残余）；新知识位 OBSERVE-117-01（重登写回侧存在性复核残余面：UpdateIDToken 取当前注册表 token 不串旧身份）；tick 主循环状态机纵深复查无新缺陷。