# R131 后端审查报告（绝对只读，身份防线矩阵第四十六轮）

- 日期：2026-09-24
- 基线：R130（7e5597c）已归档；`backend/` 自 B110-01 修复后**零产品改动**（git status 全空，仅前端报告未归档为新文件）
- 模式：绝对只读（除本报告外零修改；唯一写文件为 `archive/review-rounds/round131-backend-findings.md`）
- 聚焦清单按记忆锚点逐项核验（时间盒 35 分钟，实际 ~28 分钟完成）

## 验证表

| 项目 | 结果 | 证据 |
|------|------|------|
| go build ./... | 绿（exit 0） | 全部包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race（zhidao+accounts） | 绿 | 借 /d/mingw64 gcc，`CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/`：zhidao 2.572s ok / accounts 1.352s ok |
| 定向 race（scheduler） | 绿 | 同工具链：14.998s ok |
| 定向 race（api+store） | 绿 | 同工具链：api 15.125s / store 12.639s ok |
| 全量回归（无 race） | 绿 | `go test -count=1 ./...`：11 包全 ok（scheduler 27.0s / api 17.8s / store 16.2s） |
| gcc 探测 | `/d/mingw64/bin/gcc.exe` 存在 | `where gcc` 不在 PATH，按清单 ls 探测命中 |
| 契约 20 轮次标签扫描 | 零命中（产品代码） | 全仓 `第 *N* 轮` 扫描仅 scheduler_test.go:1583/1586/2442 三处——全部是测试注释描述"该轮回退/空快照轮数"（非轮次前缀标签，语义为测试断言说明，非决策历史，合规） |
| 零吞错穷举 | 零命中 | `_ = .*(AppendLog|Save|Delete|UpdateIDToken...)` 双形式全仓零命中；60 处库写全部 `if err := ...; err != nil { log }`（61 处计数含注释/别名行） |

## 1. 身份防线矩阵第四十六轮闭合（无新增，延续纯观察）

- **sameClientFor 定义**：scheduler.go:204（注释 199-203 行，含"同名重建/反射指针身份"完整语义）。`clientIdentity` 于 :215，`reflect.ValueOf(c).Pointer()` 判指针身份，nil 返回 0 恒非同一。
- **7 调用点逐一确认**（全为 spawnChain/ProbeForAccount 写回段，锁内）：
  - :850（ProbeForAccount 回写前，`!sameClientFor(acct, client)` 放弃写快照/识别槽）
  - :1489（ErrUnauthorized 分支，身份不符 delete inflight + 静默弃链）
  - :1521（成功分支写 done/state/SaveSuccess 前）
  - :1551（风控退避分支 markRateLimitedLocked 前）
  - :1571（窗口关闭分支 markFullLocked 前）
  - :1600（实时复核回锁后三路前）
  - :1635（实时复核确证满员 markFullLocked 前）
  - 六分支 + 探测回写 = 7，与 R130 记录逐一对齐零漂移。
- **写点换类抽查 5 类**（三族交替取未重复者）：
  - done：:1529 `s.done[acct][t.ClassID] = true`（成功分支，持锁）
  - full：:1767（markFullLocked 内，持锁）
  - inflight：:1471 置位 + :1513/:1501/:1490 清位（全持锁）；TryAcquireSubmit :1905 置位 :1910 清位（持锁）
  - rateLimited：:1714（markRateLimitedLocked，持锁）
  - state.Courses：**全部 6 写点持锁**——:1473/:1850-51（setStateLocked）、:1803-04（releaseFullIfFreedLocked）、:1961-62/:2014/:2046-47（MarkDone/RemoveDone/RemoveFull）、:592/:1964（append）、:518/:578（整列重建）。StateForAccount 读侧 :719-723 只拷贝（`st.Courses = nil` 重建，不动 s.state.Courses）。无裸写。
- **类级结构证据延续**：*Locked 后缀写函数族 14 个全数清点（:273/:427/:539/:570/:647/:739/:918/:1691/:1710/:1760/:1781/:1835/:1846 + 注释，均为持锁私有实现）；无 *Locked 后缀的外部可调写函数（MarkDone/RemoveDone/RemoveFull/SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkTokenValid/TryAcquireSubmit/ProbeForAccount/ProbeNow）全部函数体首行取锁（defer 或显式 Lock/Unlock 配对）。
- **无锁写点宿主 goroutine 唯一性射证（R130 升级项复核）**：reloginResults 通道只在 Start() 单 goroutine select 循环消费（:675-681），唯一无锁写点是 goroutine 开头（:1246-1247 `delete(relogging); ClientFor` 在锁内）→ 全域锁覆盖。
- **手动五路 accountExists**：handler.go :255（课程读）/ :305（手动报名）/ :397（手动退选）/ :497-510（目标写，内联 LoadCredentials 逐账号比对）/ :573（状态读）——五路全数在位；:1109-1120 定义判据同源（凭据表）。`handleSetTargets` 缺透传无核心账号时整体拒绝（:518-520），管理员名不落孤儿行。
- **maybeRelogin 双侧**：决策侧 :1208 `ClientFor(acct)` 存在性复核（锁内、任何 map 写入前）；写回侧 :1254 ClientFor 复核 → :1265-1272 重取当前注册表 client.Token() 落库，绝不串旧身份。与 R130 记录一致。

## 2. OBSERVE-117-01 知识位第十四轮 —— 确认在位

写回侧先 ClientFor 复核 → 重取当前注册表 `client.Token()` 落库（:1265-1274），绝不串旧身份。重登成功分支只落"当前注册表身份"的新 token（UpdateIDToken 在 :1269，失败仅 log 不吞）；失败/未重登保持 tokenValid=true 展示"已失效"（:1286-1297 语义不变）。在位零漂移。

## 3. B110-01 审计链第二十一轮 —— 零漂移

- 手动失败六处 AppendLog 全数在位：handler.go :362（报名失效）/ :374（报名 read）/ :381（报名其余失败）/ :443（退选失效）/ :452（退选 read）/ :459（退选其余失败）。
- 成功路径审计行：:387（MarkDone 内部 :1976 AppendLog success）、:467（RemoveDone 内部 :2029 exit success）、:558（set_targets）。
- 自动链失败族：scheduler.go :1504（失效）/ :1532（成功）/ :1558（风控）/ :1570 markFullLocked 内 :1770（满员）/ :1614（实时复核失效）/ :1662（read/其余失败）全数带 `if err := ...; err != nil { log }`。
- 零吞错穷举：`_ = .*(AppendLog|Save|Delete|UpdateIDToken|...)` 双形式全仓（测试外）零命中；`= d.Store.` / `= s.store.` 直接赋值忽略亦零命中。
- 唯一 `_ =` 使用点 :387/:467 为 `_ = d.Sched.MarkDone(...)` / `_ = d.Sched.RemoveDone(...)`——调度器内存态方法（内部已自持错误处理，返回恒 nil），非库写吞错，合规（R130 同点复核一致）。

## 4. O105-01 抖动基线 —— 夹具在位 + 定向 race 全绿

- socketPreheat 定义于 client_test.go:25，逐测试调用保留（:40/:52、captcha_test.go:17/:52）。
- readyProbe 三包在位：zhidao（client_test.go:78/:86）、accounts（manager_test.go:26/:87/:164/:247，三处夹具就绪探测）、api（handler_test.go:139/:185）。captcha_test.go:29/:79 亦调用。
- 定向 race 四包全绿（借 /d/mingw64 gcc，`export PATH=/d/mingw64/bin:$PATH` 后 CGO_ENABLED=1）：zhidao+accounts 2.572s/1.352s、scheduler 14.998s、api+store 15.125s/12.639s。gcc 状态：`where gcc` 不在 PATH，`/d/mingw64/bin/gcc.exe` 命中，全轮实测绿。

## 5. 新契约角度（多账号并发调度一致性纵深）

多账号并发调度一致性（R124 曾走，本轮升级纵深为"注册表遍历 + 单飞信号量 + 身份防线三族交织"）：
- submitAll 持锁遍历 acctTargets 快照生成链（:1345-1366），ClientFor 过滤后 spawnChain 在 goroutine 内二次 ClientFor（:1405-1411）——毫秒窗口删号防线仍在。
- probe() 单飞 probing 置位（:1049-1055）+ probeSem(cap 4) 信号量封顶 per-account 并发（:1073-1075，`ponytail: cap=4 常驻，若平台放宽熔断或账号数 >50 再调`）——峰值 N→4，跨批不叠加。
- 身份防线三族在 spawnChain 六分支 + ProbeForAccount 回写 + maybeRelogin 双侧共 8 处闭环，本角度未发现盲区。
- **新发现（维持观察）**：probeSem cap=4 与 probeIntervalNear 2s 周期交互——N>4 账号时一批在途 >2s 会自然错峰（信号量排队），N≤4 无叠加；行为自洽无缺陷，列维持观察。
- **验证链其余纵深**：连接池预热 maybePrewarm 只在开窗前 2 分钟 15s 节流（:293-310）与 maybeSyncClock 60s 成功闸门/30s 失败退避（:331-343）独立互不干扰——预热写 lastPrewarm 持锁、时钟写 clockOffset/syncing 持锁，无共享字段交叉写。登录重试收敛（识别≤3 提交≤2 网络/配置错误立即返回）实证在位（client.go:218-279；fetchLoginPage :293 网络层自愈一次不消耗验证码）。httpDo 网络层自愈（client.go:478-492，仅 dial/write 重试一次，read 不重试防双报）与 captcha.go recognizeViaVision（:160）复用同源——契约 40 号"连接活性自愈族"在位。

## 发现汇总（分级）

- CRITICAL：0
- HIGH：0
- MEDIUM：0
- LOW：0
- 维持观察（非新增）：maybePrewarm 无独立单测（R127 首提，行为经 tick 集成路径覆盖）、probeSem cap 常驻（如上）。**新增发现：无，明确为零。**

## 结论

身份防线矩阵第四十六轮闭合：7 个 sameClientFor 调用点逐一对齐零漂移，本轮换类 5 类写点（done/full/inflight/rateLimited/state.Courses）全持锁或宿主 goroutine 唯一性射证；*Locked 写函数族类级结构证据延续；手动五路 accountExists、maybeRelogin 双侧在位。OBSERVE-117-01 第十四轮、B110-01 第二十一轮、O105-01 均零漂移与实测绿。零产品改动链延续（第九轮纯观察）。进度 132/256。
