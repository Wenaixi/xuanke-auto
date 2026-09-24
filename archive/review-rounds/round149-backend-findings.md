# R149 后端只读审查发现（身份防线矩阵闭合第六十四轮）

> 审查基线：commit b907334（docs(review): R148 收尾——身份防线矩阵第六十三轮闭合）。
> 模式：除本 findings 报告外零文件写入，工作区零漂移。
> 实测时间：2026-09-24；环境：Windows 11 / go1.26.8 / gcc(mingw64 /d/mingw64/bin)。

## 结论前置

- **CRITICAL：无**
- **HIGH：无**
- **MEDIUM：无**
- **LOW（观察项，不阻塞）：**
  1. `accounts` 包三处测试夹具（manager_test.go:26 readyProbe）只有就绪探测、无 `socketPreheat` 双保险——zhidao/api 两包已有套接字预创建，accounts 现存 8 轮全绿实证无残余，但包序变化仍会暴露冷启动窗口（夹具注释自述"未来若再出 flake 第一候选即补 socketPreheat"）。维持观察。
  2. `ProbeForAccount` 写 `openTimeDetected[acct]` 识别槽不回写全局 `lastProbe`（注释自述 lastProbe 只归 probe()/ProbeNow 的全局维度），语义为防管理员穿透探测旁路全校节流闸门——刻意设计，维持观察。
- **建议：APPROVE**

## 验证表（全部实测）

| 项目 | 结果 | 数据 |
|---|---|---|
| 工作树基线 | ✅ | git log 顶部 = b907334（R148 归档） |
| 工作区漂移 | ✅ | `git status --short` 空；`git diff HEAD --stat` 空 |
| `go build ./...` | ✅ | 退出码 0 |
| `go vet ./...` | ✅ | 退出码 0 |
| 定向 `-race` 四包 | ✅ | `go test -race -count=1 ./internal/zhidao ./internal/accounts ./internal/scheduler ./internal/api`——zhidao 16.164s / accounts 18.781s / scheduler 15.380s / api 25.195s，全部 `ok`，RACE_EXIT=0（含 6 个删号同名重建身份防线测试 + 探测定时四 map 测试 + 实时复核失效测试） |
| 回归锚双绿 | ✅ | `TestWindowOpenSubmitsWithoutProbeReset`（scheduler_test.go:1277）与 `TestAdminStatsWindowOpenedUsesScheduler`（handler_test.go:1011）在 race 全包运行中通过 |
| 契约轮次标签扫描 | ✅ | `grep 第N轮 / R..轮 / round N` 产品代码零命中（scheduler.go:235 的 `ctx, cancel` 为误命中排除；session/store.go 引用 review-round13.md 为文档引用合规） |
| 时间基残留扫描 | ✅ | 见 LOW-132/133 回首核 |
| `_ =` / `_, _ =` / 落库忽略形态穷举 | ✅ | 见 B110-01 |

## 聚焦清单逐项裁决

### 1. 身份防线矩阵第六十四轮闭合 —— ✅ 在位

**sameClientFor 定义（scheduler.go:204）与 7 调用点零漂移。**

- 定义 `sameClientFor(acct, chainClient)`：`ClientFor` 取注册表现客户端 → `clientIdentity` 反射指针比对（`reflect.ValueOf(c).Pointer()`）。nil 与非存在恒 false，注释"调用点：spawnChain 成功/失效分支写状态与落库前"。
- 7 调用点逐一追到"写什么"的终局：
  - **:850（ProbeForAccount 回写段）**：不通过 → 放弃写 `openTimeDetected[acct]` + `acctData/acctDataAt` 三 map。终局：旧探测链不污染重建账号识别槽与年级快照。
  - **:1489（失效分支）**：不通过 → `delete(inflight)` 后静默 return，**且不触发 maybeRelogin**（注释明确：旧链命中 ErrUnauthorized 但身份已变时不得重登，防幽灵 reloginFail 计数污染重建身份首登退避）。终局：不写 tokenValid/reloginFail/reloginAt/relogging，不落 AppendLog。
  - **:1521（成功分支）**：不通过 → 放弃写 `done[acct]` + `setStateLocked(success)` + `SaveSuccess` 库行 + AppendLog。终局：重建重启后无假成功行。
  - **:1551（风控退避）**：不通过 → 放弃 `markRateLimitedLocked` + failed 状态 + AppendLog。终局：重建身份无假退避（黄金期不被静默跳过）。
  - **:1571（窗口关闭按满员）**：不通过 → 放弃 `markFullLocked`。终局：重建身份无假满员永久退避。
  - **:1600（实时复核三路共用入口）**：回锁后先指针身份复核，再进 ErrUnauthorized/确证满员/普通失败三路。终局：实时复核网络段（最长 15s）内删号重建后，旧链不写 failed 状态、不触发 maybeRelogin、不写 full。
  - **:1635（确证满员分支内层复核）**：不通过 → 放弃 `markFullLocked`。终局：与 doneHas 守卫（绝不覆盖手动胜利状态）叠成双层防线。
- **maybeRelogin 双侧闭合**：
  - 决策侧（:1208）：入口锁内 `ClientFor` 存在性复核，已删账号不发起、不写任何 relogin 族 map。实测锚 `TestMaybeReloginDeletedAccountSkipsMaps`（scheduler_test.go:3477）验证四 map（tokenValid/reloginFail/reloginAt/relogging）均无残留。
  - 写回侧（:1254）：goroutine 完成后先 `ClientFor` 存在性复核（已删则只清 relogging 静默放弃）。
  - **二次重取（:1265-1273）OBSERVE-117-01**：成功分支 `if client, ok := s.clients.ClientFor(acct); ok { if tok := client.Token(); ... }` 重取**当前注册表客户端**的 Token() 落库 `UpdateIDToken`——绝不用 Relogin 调用的返回值或旧指针的 token。落库失败记日志（:1270）。
- **手动五路 accountExists（handler.go）**：:255 课程读 / :305 手动报名 / :397 手动退选 / :497-512 目标写（此路内联 LoadCredentials 循环，与 accountExists 同源）；:573 状态读——五路全覆盖，判据同源（凭据表 = "确实登录过"更强真理源）。
- **写点换类 5 类全持锁/唯一性射证**：
  - `lastSubmit`：submitAll:1346 锁内置 `nowAlignedLocked()` 写入；读侧 tick:1029 锁内读 + submitIntervalFor 判读。写读同对齐钟。
  - `lastSyncStart`/`syncing`：maybeSyncClock:355-356 锁内写；完成回调 :369/:404-405 锁内复位。
  - `lastProbe`：probe() :1089/:1114 锁内写；ProbeNow:964 锁内；Start 主循环 reloginResults 回传 :678-679 锁内。唯一外围写点在 Start 的 select 分支（注释自述"避免 goroutine 并发写 s.lastProbe 竞态"）。
  - `state.EmptyProbeRuns`：probe() :1156-1158 锁内（入账 +10s 裕量）。
  - **无锁写点 `warnedNoTargets`（:1371-1372）宿主唯一性射证**：字段声明 scheduler.go:176，唯一读写发生在 submitAll 的 `len(chains)==0` 顶级分支内（该路径无 goroutine 并发），单写单读标记，合规。
- **`*Locked` 写函数族 13 个 + 外部写函数首行取锁双向射证**：13 个 Locked 函数（nowAlignedLocked/openTimeForLocked/enrichTargetPubMetaLocked/rebuildCoursesForAccountLocked/rebuildCoursesLocked/tokenValidForLocked/windowClosedLocked/isRateLimitedLocked/markRateLimitedLocked/markFullLocked/releaseFullIfFreedLocked/statusIndexLocked/setStateLocked）全部带 `需持 s.mu` 契约注释；外部写函数（SetTargetsForAccount/PurgeAccount/RestoreTargets/RestoreDone/RestoreRefused/MarkDone/RemoveDone/RemoveFull/submitAll/ProbeNow/ProbeForAccount/StateForAccount/MarkTokenValid/RemoveFull）首行均为 `s.mu.Lock()`/`s.mu.Lock(); defer s.mu.Unlock()`——双向无例外。

### 2. OBSERVE-117-01 知识位第三十二轮 —— ✅ 在位

maybeRelogin 写回侧定位已确认完整合同：先 `ClientFor` 存在性复核（:1254）→ 成功分支再二次 `ClientFor`（:1265）重取当前注册表客户端指针 → `client.Token()`（zhidao/client.go:195 锁内读当前 token）→ `UpdateIDToken` 落库（store.go:56）。同名重建场景（旧链触发重登、重登耗时期间删号重建）下重取的是新身份 token——重建身份自身所属，绝不串旧身份。写序（先复核存在性、后重取 Token）杜绝"复核通过后被删除"的毫秒窗口（复核与重取同持 s.mu，删除的 PurgeAccount 同锁互斥）。

### 3. B110-01 审计链第三十九轮 —— ✅ 在位

- 手动 6 失败位 AppendLog 全覆盖：handler.go:362（报名失效）/ :374（报名 read）/ :381（报名业务失败）/ :443（退选失效）/ :452（退选 read）/ :459（退选业务失败）——全部 `if err := AppendLog(...); err != nil { log.Printf }` 记日志，零吞错。
- 成功审计行：报名成功（MarkDone:scheduler.go:1976）+ 退选成功（RemoveDone:2029）+ 手动登录/管理员登录/登出/目标保存/删账号等早既在。
- 自动链失败族：spawnChain 六分支各带 AppendLog（:1504/:1532/:1558/:1614/:1662/:1770 markFull）+ 探测定时（:823/:1094/:956 ProbeNow）触发 maybeRelogin 由重登日志链路覆盖。
- **零吞错穷举**：`_ =`/`_, _ =`/直接赋值忽略形态全量扫 `internal/zhidao|accounts|scheduler|api|store|db|session|secure|runtime|config` 产品代码，命中仅 6 处且全部为允许形态：io.Copy(io.Discard) 丢弃响应体 ×2（zhidao:107/:124）、`_ = pw.Prewarm()`（预热失败静默，延迟无关）、`_, _ = s.ProbeForAccount(acct)`（probe 探测段错误本来就走日志路径）、`_ = d.Sched.MarkDone/RemoveDone`（handler 手动成功路径，其内部已处理落库错误并记日志）。**全部落库点（SaveSuccess/SaveRefused/DeleteSuccess/DeleteRefused/UpdateIDToken/SetTargetsForAccount）零 `_ =` 命中**。
- 网络层 token 脱敏延续抽查：sanitizeError（zhidao/client.go:581）剥 `*url.Error` 完整请求 URL 文本（`?idToken=` 即认证通道），Unwrap 下沉 `sanitizerErr`（:566-572）保 isConnErrRetryable/IsReadErr 判型穿透（sanitize_test.go 断言"脱敏后判定同原始"）；scheduler.go:1283 重登成功日志走 `maskedToken(newTok)`（:1334，>8 位只显前 8 位，≤8 位输出 `***`）。

### 4. O105-01 抖动基线 —— ✅ 在位

- `socketPreheat` 夹具：zhidao/client_test.go:25（net.Listen 127.0.0.1:0 后 Close），包装级 TestMain 预加热（:39）+ 逐测试调用（:40/:52）+ sanitize_test:97。
- `readyProbe` 夹具：zhidao/client_test.go:88（200ms×10 + 2s 超时，总窗口 ~2s）；api/handler_test.go:185 同款；accounts/manager_test.go:26 同款。
- **定向 race 实测**（借 /d/mingw64/gcc）：四包全绿，见验证表。身份防线族十测全部随包通过（6 个删号同名重建 + TestMaybeReloginDeletedAccountSkipsMaps + TestDeletedAccountRebuiltSameNameChainRealtimeUnauthorizedDropsRelogin + TestDeletedAccountRebuiltSameNameChainSuccessDropsInflight + TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime）。
- 回归锚 `TestWindowOpenSubmitsWithoutProbeReset` + `TestAdminStatsWindowOpenedUsesScheduler` 绿（含在 race 全包运行）。

### 5. LOW-132/133 回首核 —— ✅ 通过

- `git show b907334` 白线核对：R148 归档仅写作 review-round148 三文档，产品代码零改动。
- 时间基全量扫 `time.Since/time.Now()` 产品代码 + 排除注释后，残余仅为两处"写读同基自洽"：
  - `reloginAt`（scheduler.go:1220/:1227/:1231/:1261）写 `time.Now()`、读 `time.Since(t)`——同本地钟自洽（relogin 防抖与时钟对齐无关，平台侧无对齐语义）。
  - `gateWindow`（manager.go:53/:71/:74/:226/:227）写读同本地钟——自洽。
  - 其余调度器时间判定（`now.Sub(lastProbe)`/`now.AlignedLocked`）全部对齐钟；SyncServerTime（client.go:113/:135/:137）本地钟测 RTT、减法得 offset，属测量基准本身不可对齐，合规。
- 断言"残余仅 reloginAt 与 gateWindow 两处写读同基自洽"成立。

## 新契约角度纵深（自选 ×2）

选择理由：这两处是本轮聚焦清单以外承压最重的两条链路——窗口三判据是"防轰炸契约"的总闸，快照回退链是"年级串线"的历史重灾区，各自都有多轮沉淀痕迹，适合纵深复核。

### 角度一：窗口状态三判据单源 open 快照（windowClosedLocked / probe 入账 / tick 守卫）

- `windowClosedLocked`（scheduler.go:918-939）三条判据共用**单次 `open := s.openTimeForLocked("")` 快照**（:924）：主判据 `s.state.WindowClosed`（probe 写入）/ 时钟连续失败 ≥3 且 `!open.IsZero() && now.After(open)` / 从未开窗 `EmptyProbeRuns >= 3` 同快照。注释明确"一次读取避免识别值被新批次覆盖的不一致窗口"。
- probe() 入账侧（:1145-1159）：判定侧补"现在 After(open+10s)"、空快照入账侧同样 +10s 裕量且 `now.After(open.Add(10*time.Second))`，两处 10s 裕量各自取 `open` 但都在同一探测内（探测期间开放时间识别只被本探测覆盖，跨探测一致性由识别槽全局单值 + 同锁保护）。
- `StateForAccount`/`WindowClosed()` 共用同一 `windowClosedLocked` 实现（:703/:910），杜绝"展示层与挂起判定两套真相"分叉。
- 关联回归锚：`TestProbeIntervalZeroOpenTime`（scheduler_test.go）四态（零值 30s / 过期+关闭 30s / 过期+快照非空 2s / 未来临门 2s）在 race 全绿中通过；`TestSubmitSuspendedWhenOpenTimeCleared`（夹具 WindowOpened=true 默认）保持绿，确认 B41-02"零值守卫让位于 WindowOpened"未破坏既有挂起语义。
- **裁决：单源 open 快照约定在判据集、入账侧、守卫侧三处一致实施，无漂移。**

### 角度二：年级隔离快照回退链三态（ElectivesSnapshotFor）

- 三态契约（scheduler.go:761-805）复查：
  1. **有目标账号**（`len(acctTargets[acct]) > 0`）：专属帧存在且新鲜（`nowAlignedLocked().Sub(snappedAt) <= TTL`）→ 返回专属帧；专属帧缺失或过期 → **返回 (nil,false) 触发真刷新，绝不回退全局帧**（注释 :764-771：全局帧可能被 order[0] 账号刷新为新鲜错年级帧，回退即年级串线 + 浏览者永不触发本账号刷新）。
  2. **无目标但曾有专属帧**（:786-799）：同规则——专属帧新鲜返回，过期返回 (nil,false) 绝不回退全局帧（:794 明确"过期专属帧 → 必须返回 false"）。
  3. **从未有过专属帧的纯浏览**（:801-804）：允许回退全局帧（快、无网络），且全局帧也过期才返回 (nil,false)。
- 三态边界与 `AccountsWithTargets()`（`len>0`）判定一致；`probe()` 只对不同目标账号刷新专属帧（:1069）。
- 关联：`ProbeForAccount` 回写段（:850 sameClientFor）保证重建身份不会被旧链过期专属帧污染；测试锚 `TestElectiveSelectRejectsWindowClosed`（快照渲染正确性）含在 api race 全绿中。
- **裁决：三态边界与判据（len>0 而非 key 存在性）实现与契约文档一致，无回退链漂移。**

## 维持观察项

1. accounts 包 readyProbe 无 socketPreheat 双保险（见结论前置 LOW-1）。
2. `ProbeForAccount` 刻意不回写全局 lastProbe（见结论前置 LOW-2）。
3. 实时复核 `classFullRealtime`（:1751）保留为"平台未来下发 maxCount 时自动生效"防御路径，当前真满员主路径为快照判满（实证 maxCount 恒 0）——存在维持多轮， unflagged。
4. `syncFailedWindow`/`syncFailStreak` 写而不读留档字段（注释自述），对称保留供未来时间差调参——维持观察。

## 结尾

聚焦清单六项全部 ✅ 在位，定向 race 四包全绿，身份防线矩阵六十四轮闭合成立，OBSERVE-117-01 知识位第三十二轮在位，B110-01 审计链第三十九轮零漂移，O105-01 实测绿，LOW-132/133 回首核通过。

**建议：APPROVE**