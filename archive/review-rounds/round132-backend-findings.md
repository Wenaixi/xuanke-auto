# R132 后端审查报告（绝对只读，身份防线矩阵第四十七轮）

- 日期：2026-09-24
- 基线：R131（073f7fd）已归档；`backend/` 自 B110-01 修复后**零产品改动**（HEAD=073f7fd，`git diff 073f7fd --stat -- backend/ internal/` 全空）
- 模式：绝对只读（除本报告外零修改；唯一写文件为 `archive/review-rounds/round132-backend-findings.md`）
- 聚焦清单按记忆锚点逐项核验（时间盒 35 分钟，实际 ~22 分钟完成）

## 验证表

| 项目 | 结果 | 证据 |
|------|------|------|
| go build ./... | 绿（exit 0） | 全部包编译通过 |
| go vet ./... | 绿（exit 0） | 无 vet 告警 |
| 定向 race（zhidao+accounts） | 绿 | 借 /d/mingw64 gcc，`CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/`：2.026s / 1.371s ok |
| 定向 race（scheduler） | 绿 | 同工具链：15.262s ok |
| 定向 race（api+store+session） | 绿 | 同工具链：14.131s / 21.159s / 2.124s ok |
| 全量回归（无 race） | 绿 | `go test -count=1 -p 1 ./internal/... ./...`：13 包全 ok（scheduler 21.371s / api 12.301s / store 2.335s） |
| gcc 探测 | `/d/mingw64/bin/gcc.exe` 存在 | `where gcc` 不在 PATH，按清单 ls 探测命中 |
| 契约 20 轮次标签扫描 | 零命中（产品代码） | 全仓扫（含 `第 *N* 轮`/`R..轮`/`round *N*`）仅两处历史文档引用：session/store.go:117（"docs/review-round13.md 与 CLAUDE.md 落盘"）与 tray_windows.go:99（"R79"）——均为对已归档决策文档的引用、非决策历史标签残留（语义即"为什么/契约"本身，合规）；测试文件 scheduler_test.go:1583/1586/2442 三处为测试断言叙述（"第 3 轮后…"非轮次前缀），合规延续 |
| 零吞错穷举 | 零命中 | `_ = .*(AppendLog|Save|Delete|UpdateIDToken...)` 双形式全仓（测试外）零命中；scheduler/handler 两文件全部 29 处 store 写点均 `if err := ...; err != nil { log }` |

## 1. 身份防线矩阵第四十七轮闭合（无新增，延续纯观察）

- **sameClientFor 定义**：scheduler.go:204，`clientIdentity` 于 :215（`reflect.ValueOf(c).Pointer()` 指针身份，nil 返回 0 恒非同一），注释 199-203 行含"同名重建/反射指针身份"完整语义。零漂移。
- **7 调用点逐一确认**（与 R131 记录逐一对齐）：
  - :850 —— ProbeForAccount 回写前，`!sameClientFor` 放弃写 acctData/openTimeDetected
  - :1489 —— ErrUnauthorized 分支，身份不符 delete inflight + 静默弃链
  - :1521 —— 成功分支写 done/state/SaveSuccess 前
  - :1551 —— 风控退避分支 markRateLimitedLocked 前
  - :1571 —— 窗口关闭分支 markFullLocked 前
  - :1600 —— 实时复核回锁后三路前（含存在性判定）
  - :1635 —— 实时复核确证满员 markFullLocked 前
  - 六分支 + 探测回写 = 7，全数为「网络往返后持锁写入」前最后一道身份闸，零漂移。
- **写点换类抽查 5 类**（三族交替取未重复者）：
  - done：:1529 `s.done[acct][t.ClassID] = true`（成功分支，持锁）
  - full：:1767（markFullLocked 内，持锁）
  - inflight：:1471 置位 + :1490/:1501/:1513 清位（全持锁）；TryAcquireSubmit :1905 置位 :1910 清位（持锁 + sync.Once 防双施放）
  - rateLimited：:1714（markRateLimitedLocked，持锁，写入侧用对齐钟 nowAlignedLocked 与判读侧同源）
  - state.Courses：**全写点持锁**——setStateLocked :1850-51、MarkDone :1961-62、RemoveDone :2014-15、RemoveFull :2045-47、releaseFullIfFreedLocked :1803-04、rebuildCoursesForAccountLocked :592/578、PurgeAccount :518、probe :1473。StateForAccount 读侧 :718-723 只拷贝（`st.Courses = nil` 重建，不动共享值）。无裸写。
- **类级结构证据延续**：*Locked 后缀写函数族全数在列（openTimeForLocked/nowAlignedLocked/windowClosedLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/rebuildCoursesLocked/tokenValidForLocked/markFullLocked/releaseFullIfFreedLocked/markRateLimitedLocked/statusIndexLocked/setStateLocked 等，均为持锁私有实现）；外部可调写函数（SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkTokenValid/TryAcquireSubmit/MarkDone/RemoveDone/RemoveFull/ProbeForAccount/ProbeNow）全部函数体首行取锁（defer 或显式 Lock/Unlock 配对）。
- **无锁写点宿主 goroutine 唯一性射证延续**：reloginResults 通道唯一消费者为 Start() 单 goroutine select 循环（:675-681，持 s.mu 写 lastProbe 复位）；唯一无锁写点是 goroutine 开头段（:1246-1247 delete(relogging)，在锁内）。全域锁覆盖，零漂移。
- **手动五路 accountExists**：:255（课程读）/ :305（手动报名）/ :397（手动退选）/ :497-510（目标写，内联 LoadCredentials 逐账号比对）/ :573（状态读）——五路全数在位；:1109-1120 判据同源（凭据表）。`handleSetTargets` 无透传且无核心账号时整体拒绝（:518-520），管理员名不落孤儿行。零漂移。
- **maybeRelogin 双侧**：决策侧 :1208 `ClientFor(acct)` 存在性复核（锁内、任何 map 写入前，注释覆盖"删号与在飞探测 ErrUnauthorized 同帧"污染场景）；写回侧 :1254 ClientFor 复核 → :1265 重取当前注册表 client.Token() 落库。与 R131 记录一致零漂移。

## 2. OBSERVE-117-01 知识位第十五轮 —— 确认在位

写回侧先 ClientFor 复核（:1254，已删整块放弃、只清 relogging）→ 重取当前注册表 `client.Token()` 落库（:1265-1272，UpdateIDToken 失败仅 log 不吞）。重登成功分支只写"当前注册表身份"的新 token，绝不串旧身份；失败/未重登保持 tokenValid=true 展示"已失效"（:1286-1297）。在位零漂移。决策侧锁序 reloginMu→s.mu 与 TokenValidFor/MarkTokenValid 对齐（:1307-1318），杜绝手动登录与在途重登并发半态读。

## 3. B110-01 审计链第二十二轮 —— 零漂移

- 手动失败六处 AppendLog 全数在位：handler.go :362（报名失效）/ :374（报名 read）/ :381（报名其余）/ :443（退选失效）/ :452（退选 read）/ :459（退选其余）。
- 成功路径审计行：:387（MarkDone 内部 :1976 AppendLog success）、:467（RemoveDone 内部 :2029 exit success）、:558（set_targets）。handler 层其余 11 处库写（登录/注销/配置/删账号/激活码）全部 `if err := ...; err != nil { log }`。
- 自动链失败族：scheduler.go :1504（失效）/ :1532/:1535（成功 AppendLog+SaveSuccess）/ :1558（风控）/ :1770 markFullLocked（满员）/ :1614（实时复核失效）/ :1662（read/其余失败）全数带错误日志。MarkDone :1945/:1973/:1976、RemoveDone :2020/:2026/:2029、SetTargetsForAccount :476/:484、UpdateIDToken :1269 全部持显式 err 分支。
- 零吞错穷举：`_ = .*(AppendLog|Save|Delete|UpdateIDToken|SetTargets|DeleteRefused|PurgeAccount|...)` 双形式全仓（测试外）零命中；scheduler.go 与 handler.go 两文件 `_ =` 仅三处——:307 `go func() { _ = pw.Prewarm() }()`（预热网络请求，错误静默由下一 15s 周期自然重试，非库写）、:1075 `_, _ = s.ProbeForAccount(acct)`（探测结果由调用方已处理，非库写）、handler :387/:467 MarkDone/RemoveDone（调度器内存态方法返回恒 nil，非库写吞错）。合规零漂移。

## 4. O105-01 抖动基线 —— 夹具在位 + 定向 race 全绿

- socketPreheat 定义于 zhidao/client_test.go:25，逐测试调用保留（:40/:52）；readyProbe 三包在位：zhidao（client_test.go:78/:86/:164）、accounts（manager_test.go:26/:87/:164）、api（handler_test.go:139/:185）。captcha_test.go:17/:29/:52/:79 亦调用。
- 定向 race 全绿（借 /d/mingw64 gcc，`export PATH=/d/mingw64/bin:$PATH` 后 CGO_ENABLED=1）：zhidao+accounts 2.026s/1.371s、scheduler 15.262s、api+store+session 14.131s/21.159s/2.124s。gcc 状态：`where gcc` 不在 PATH，`/d/mingw64/bin/gcc.exe` 命中，全轮实测绿。
- 回归陷阱测试在位：TestWindowOpenSubmitsWithoutProbeReset（scheduler_test.go:1277）与 TestAdminStatsWindowOpenedUsesScheduler（api/handler_test.go:1011）在改动 tick 守卫/探测时序前的双锚点存在。

## 5. 新契约角度（登录闸门族 + 时间基一致性纵深，本自选未覆盖角度）

R130/131 已走激活码生命周期/并发调度一致性，本轮选「登录闸门族全入口收口 + 时间基一致性」纵深：
- **登录闸门族全入口收口确认**：Manager.gateWait（阻塞，2 次/分钟，只收口 Relogin 自动重登，manager.go:49-62）与 gateTryAcquire（非阻塞，手动登录 LoginByPassword :244 前置、管理员换绑同走收口，:223-235）共享 gateMu/gateUsed 同一计数，全账号每分钟 doLogin 预算严格共享——B42-01 契约在位且无绕行旁路。ResetGateForTest 测试专用在位（:82-87）。
- **时钟族时间基一致性**：lastSyncStart（:356，对齐钟）、lastSyncTime（:393，对齐钟）、clockOffset（:390-391，对齐钟语义）、syncing 首行取锁、lastSyncFailAt 失败退避（:372）——核对见【新增发现】。
- **恢复顺序契约**（main.go:120-140）：RestoreDone → 逐账号 RestoreTargets（不清 refused）→ LoadRefused+RestoreRefused，顺序与 CLAUDE.md 契约 6 完全一致；SetTargetsForAccount 只清 refused 的"绝不清理"语义在 :454-465 定案注释在位。

## 发现汇总（分级）

- CRITICAL：0
- HIGH：0
- MEDIUM：0
- **LOW：1（新增，契约打磨级，非缺陷）**
- 维持观察（非新增）：maybePrewarm 无独立单测（R127 首提，行为经 tick 集成路径覆盖）、probeSem cap=4 常驻（R131）、`go func() { _ = pw.Prewarm() }()` 错误静默（预热失败由下个 15s 周期自然重试，与库写入零吞错规范不同域）。

### 新增发现

[Scheduler.go:372] lastSyncFailAt 用本地钟写入、判读侧用对齐钟——时间基混用残留
Confidence: HIGH
Issue: `maybeSyncClock` 失败回调 `s.lastSyncFailAt = time.Now()`（本地钟），而退避判读侧 :342 `now.Sub(s.lastSyncFailAt) < 30s` 中的 now 是**对齐钟**（tick() :974 `now := s.nowAligned()` 传入）。同一函数内 lastSyncStart/lastSyncTime 均用对齐钟（:356/:393），只有 lastSyncFailAt 一处落本地钟——与已消灭的"写入本地/读对齐"混用模式（探测时间戳 / 退避写入侧修复）同族，是本函数内最后一座混用孤岛。实测偏差 ~640ms 对 30s 退避无实质影响（±0.6s 误差不可感），风险等级 LOW。
Fix: 失败回调改 `s.lastSyncFailAt = s.nowAlignedLocked()`（锁内取对齐钟）即可与同族时间基准完全一致；无行为预期变化，属契约打磨。

## 结论

身份防线矩阵第四十七轮闭合：7 个 sameClientFor 调用点逐一对齐零漂移，本轮换类 5 类写点（done/full/inflight/rateLimited/state.Courses）全持锁；*Locked 写函数族类级结构证据延续；手动五路 accountExists、maybeRelogin 双侧在位。OBSERVE-117-01 第十五轮、B110-01 第二十二轮、O105-01 均零漂移与实测绿。零产品改动链延续（第十轮纯观察）。新增 1 项 LOW 契约打磨发现（lastSyncFailAt 时间基混用），无 CRITICAL/HIGH/MEDIUM。进度 133/256。
