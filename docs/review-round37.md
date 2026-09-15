# 第 37 轮 全模块只读审查

## 范围与方法

本轮对 `backend/` 全部 Go 模块做了通读审查：scheduler.go / handler.go / client.go / main.go / runtime / store / session / accounts / api/router / zhidao 全家 / secure / db / config 及全部测试文件。

验证手段：`go build ./...`、`go vet ./...`、`go test ./...` 全绿，`go test -race`（scheduler + api）全绿。所有发现均对照根 CLAUDE.md 的历轮定案与决策锚评估，不重复已修复/已定案事项。

## 修复（MINOR 两处 + 顺手收敛两处）

### B37-01 MINOR——MarkDone 手动重报成功只清内存 refused，库内 refused 行残留

**位置**：`backend/internal/scheduler/scheduler.go` MarkDone（1711-1726 区域）

**缺陷**：MarkDone 仅 `delete(s.refused[acct], classID)` 清内存标记；store 层只有整账号 `DeleteRefused(acct)`，无单课删除方法，故库内 refused 行残留。

**触发路径**：① 用户手动报名 C 成功（success 落库）→ ② 用户手动退选 C（RemoveDone：DeleteSuccess 删 success 行 + SaveRefused 落库 refused 行 + 内存 refused 置位）→ ③ 用户手动重报 C 成功（MarkDone：done 置位 + 内存 refused 删除，但库内 refused 行残留）→ ④ 重启 → main.go 恢复序 RestoreDone（C 进 done）→ RestoreTargets → LoadRefused（读到 C 残留行）→ RestoreRefused（C 进内存 refused）→ `rebuildCoursesForAccountLocked` 按 B10-03 契约"refused 优先于 done"置状态为 `"已手动退选（自动引擎不再接管）"`。

**后果**：C 课程实际已报名成功，但状态显示"已手动退选"；spawnChain 对 C 命中 `refusedHas` 永久跳过。前端误导 + 窗口重开后该课永不自动接管。与 B5-07 的"手动报名成功解除 refused"只完成内存侧不对称。

**修复**：store 层新增单课 `DeleteRefusedClass(acct, classID)`（`DELETE FROM refused WHERE account=? AND class_id=?`）；MarkDone 成功分支在内存 delete 处同步落库，失败记日志（B33-01 家族零吞错）。内存侧 delete 保留（用户意图）不补回。

**TDD**：
- store 层 `TestDeleteRefusedClassOnlyRemovesOneClass`：单课删除只影响目标课程、保留该账号其他退选课与其他账号记录、幂等。
- scheduler 层 `TestManualReselectClearsRefusedRow`（persistentStore 忠实复刻 SQLite 语义）：RemoveDone 落库行 → MarkDone 后库行清空 + done 置位 + 内存 refused 解除。

### B37-02 MINOR——ElectivesSnapshotFor 目标判据用 map key 存在性而非 len>0

**位置**：`backend/internal/scheduler/scheduler.go` ElectivesSnapshotFor（645 行）

**缺陷**：`if _, hasTargets := s.acctTargets[acct]; acct != "" && hasTargets` 用 map key 存在判断。handler 层清空目标时 `req.Targets = []Target{}`（非 nil 空 slice），`SetTargetsForAccount` 留下 `acctTargets[acct] = []`（key 存在）。

**触发路径**：用户设过目标→清空目标（合法操作，TestSetTargetsEmptyAllowed 固化）→ 该账号 `hasTargets=true` → 走"有目标账号"专属帧分支。但 `AccountsWithTargets()` 只返回 `len>0`，probe() 不再 per-account 刷新该账号 → acctData 40s 过期后**每次** `/api/electives` 触发 `ProbeForAccount` 网络探测（而从未设目标的账号回退全局帧零网络）。

**后果**：清空目标的账号浏览选课大厅频繁打平台 findElectivesData（前端 10s 轮询 × 每次过期即打），且与"无目标账号走全局帧"的行为契约不一致。数据正确（安全方向），无年级串线风险，但网络行为与注释描述相悖。

**修复**：判据改 `len(s.acctTargets[acct]) > 0`。

**TDD**：`TestManualSnapshotFallbackOnlyWhenOwnFresh` 追加断言 4——清空目标后（SetTargetsForAccount 留空 slice）账号回退全局帧快路径（ok=true），不得返回 (nil,false) 触发探测。旧实现红灯 → 修复绿。

### B37-03 可疑-1 收敛——spawnChain ErrUnauthorized 分支补 ClientFor 复核

**位置**：`backend/internal/scheduler/scheduler.go` 1304-1316

**观察**：B30-01 只覆盖链顶与取 client 两处。ErrUnauthorized 分支 `s.maybeRelogin(acct)`（锁外 goroutine 执行登录，最坏 2 分钟）后，分支仍执行 `setStateLocked(...)` + `AppendLog("教务令牌失效，自动重登中")`。若在 SelectClass 网络往返（最长 15s）期间管理员删除账号，分支会给幽灵账号写 failed 状态行 + 一行 DB 日志。B21-03 只在重登 goroutine 成功分支做 ClientFor 复核，挡不住这一条。

**修复**：分支内 setState+AppendLog 前持锁复核 `ClientFor(acct)` 仍存在，已删只清 inflight 静默放弃（与成功分支 B18-M2 同款防线）。

**TDD**：`TestUnauthorizedBranchDeletedAccountSkipsState`——SelectClass 阻塞钩子卡住网络往返 → 期间删号 → 放行 → 断言状态不被覆写为 failed、日志 0 行。旧实现红灯 → 修复绿。

### B37-04 可疑-2 收敛——handleAdminStats 零值 open 输出 "0001-01-01 00:00:00"

**位置**：`backend/internal/api/handler.go` handleAdminStats（933 行）

**观察**：管理员清空 open_time（F7-02 合法操作）后，stats 的 `open_time` 字段为 year-1 字符串；`open_time_set:false` 同步下发。配置回显 handleConfig 对零值已输出空串，stats 字符串若为 year-1 会与布尔字段自相矛盾。

**修复**：零值输出空串，与 config 回显对齐。

**TDD**：`TestAdminStatsOpenTimeZeroShowsEmptyString`——通过管理接口清空 open_time → stats `open_time==""` 且 `open_time_set==false`。旧实现红灯（"0001-01-01 00:00:00"）→ 修复绿。

### 可疑-3 顺手修正——maskKey 首行注释与实现矛盾

**位置**：`backend/internal/api/handler.go` 678-689

**观察**：注释第一行"空值恒显 ****"，实现 `if v == "" { return "" }` 返回空串；第二段注释与实现一致。行为正确，首行注释自相矛盾。

**修复**：首行注释改为"设置了就有掩码，没设置才为空"（保留历轮 A4 决策语义）。

## 已核无缺陷的高风险区域清单

- **windowClosedLocked 三判据单源**（764-786）：主判据（至少开过窗 + 空快照 + `now.After(open+10s)`）、判据2（syncFailStreak≥3 带"开放时间已过" B32-01）、判据3（EmptyProbeRuns≥3 且 open 非零）——open 单快照（B35-02）、判据2/3 的 `!open.IsZero()` 保护齐全；StateForAccount 与 WindowClosed() 共用（B29-02）。EmptyProbeRuns 入账侧 10s 裕量（B21-02）与主判据裕量（B26-03）对称。
- **spawnChain 防轰炸与竞态防线**：链顶双 ClientFor 复核（B30-01）、tokenValidForLocked 短路（B23-03）、relogging 短路、done 让位（成功分支 + 实时复核满员分支 B23-01）、锁外实时复核（C-4）与 F13-m2 锁内 DB 写窗口、isRateLimitedLocked/markRateLimitedLocked 对齐钟读写（B16-M1）。本轮补 ErrUnauthorized 分支复核（B37-03）后，删号竞态三条写回路径 + 失效分支全部闭合。
- **删账号 memory-first 四步序**（handler.go 987-1001）：Remove → PurgeAccount → DeleteAccount → RevokeAccount，B18-M2/B20-01/B21-03 的锁内 ClientFor 复核在手动/自动/重登三路齐全。
- **全部落库点零吞错**：grep 确认 scheduler.go + handler.go 无 `_ =` 吞掉 SaveSuccess/SaveRefused/DeleteRefused/DeleteSuccess/AppendLog/UpdateIDToken，B33-01 家族（B34-03/B35-01/B36-01）全覆盖。本轮 MarkDone 新增 DeleteRefusedClass 落库也走日志规范。
- **时钟校准**：syncing 调用侧置位（B8-M1）、30s 失败退避（B9-03）、lastSyncTime 只在成功推进（B7-M1）、streak 只在成功清零且 ≥3 复位 offset（B21-01）、无客户端复位 syncing（F12-B1）。
- **探测防轰炸**：30s 节流 + probing 单飞（F12-B2）+ probeSem cap4（F17-01）；tick 首探豁免对零值 open 恒 false 不误触发。
- **管理员 ?account= 透传**：handleSetTargets（B15-M4）/handleElectives（B26-02）/select+exit（B27-01）/handleState（B27-02）四路 accountExists 凭据表校验齐全；handleSetTargets 无透传无目标账号整体拒绝（B20-04）。
- **重登链路**：reloginMu→s.mu 锁序三路一致（maybeRelogin/MarkTokenValid/TokenValidFor）、退避封顶（maxReloginFail=5）、B21-03 重登成功分支 ClientFor 复核、B24-01 空壳摘除 vs 有效客户端保留。
- **登录/激活安全**：识别≤3+提交≤2 收敛、网络错误立即返回、票据单次防重放（F13-m1）、激活码原子消耗事务（B5-08 已激活不扣次）、CSRF JSON 门、登录/激活独立限流桶、管理员口令恒定时间比对+时延拉平。
- **测试质量抽查**：TestStoreFailuresLogged、TestSetTargetsDeleteRefusedFailureLogged（B36-01）、TestGhostWindowClockFailuresSuspend 已改真实 maybeSyncClock 路径（B22-02 去除手动注入假绿）、TestRealtimeFullRecheckKeepsManualSuccess 等新契约测试断言与实现语义一致；`go test -race` 全绿证明锁序无死锁/竞态。
- **已知观察项按任务清单未再报告**（HTTP 探测单飞、Decrypt 死字段、RemoveFull 死方法、emptyRunsFor 死代码、syncFailedWindow 写而不读、TestClockSyncFailureResetsOffset 注释过时、task_log 无容量上限、TestRefusedPersistedAcrossRestart 用 fakeStore 模拟恢复序等）。

## 结论

第 37 轮未发现 CRITICAL/MAJOR 级新缺陷。两个 MINOR 已落地修复（**MarkDone 同步清库内 refused 单课行**、**ElectivesSnapshotFor 目标判据改 len>0**），两个可疑项已顺手收敛（**ErrUnauthorized 分支补 ClientFor 复核**、**stats 零值 open 输出空串**），maskKey 矛盾注释已修正。其余高风险区域经通读 + 编译 + race 验证均无真实触发路径的缺陷。

回归：`go build ./...` + `go vet ./...` + `go test ./...` + `go test -race ./...` 全绿。

---

## 前端部分（M37 系列，主 gate 现场核实 + 前端子代理 review37-frontend 报告）

### M37-01（MINOR）「按剩余排序」排序键与标签语义不符

**位置**：`web/src/routes/Select.tsx` 排序块（原 807-811 行）。

**缺陷**：排序按钮文案"剩余名额正序"（注释"名额越少越靠前，抢手课程一眼可见"），排序键却是 `a.selected_count - b.selected_count`（已报名数升序）——两课 `max_count` 不同时"已报少"≠"剩余少"（A=1/5 余4 vs B=10/100 余90，当前把 B 排前而 B 剩余更多），排序结果与 UI 意图及同屏徽章矛盾。

**修复**：排序键改真实剩余名额 `remaining(c) = c.max_count > 0 ? c.max_count - c.selected_count : 0`；`max_count=0`（名额未公布）映射为 0（最紧张），与 M30-04/32-02 同源语义。commit `5175a0c`。

**验证**：`npm run build`（tsc -b + vite）通过。

### M37-02（MINOR）handleBack 回显等待忙轮询无调度让步

**位置**：`web/src/routes/Select.tsx` 487-489 行。

**缺陷**：`while (!echoedRef.current && Date.now() < deadline) await setTimeout(r, 10)`——5s 兜底窗口内约 500 个 10ms 定时器忙轮询，与回显合并 effect 同宏任务循环争抢时间片（33-01 前置守卫兜住数据不丢，属性能/调度瑕疵）。

**修复**：轮询间隔 10ms→50ms（React 一帧约 16ms，50ms 足够感知合并完成且不抢渲染调度）。commit `96bab74`。

**验证**：`npm run build` 通过。

### 可疑项裁决

- **可疑-3（App.tsx:168-170 onUnauthorized 守卫阻止代理目标失效会话清理）**：主 gate 现场核实后**否决采纳修复方案，维持现状记观察**。理由：① 前端 401 归属反查（`detail.session`）可靠，但改窄守卫 `lostAccount === current` 会在代理态下网络抖动/后端 12h TTL 恰好到期时**误删学生有效会话**，重开 F8-01"绝不误杀"契约；② 代理态停留期间管理员自身会话过期的正确处理已由 `lostAccount === adminName` 分支专门覆盖（清 targetAccount + 退出管理态），守卫保护的正是"代理态下有效令牌不被误杀"；③ 三条失效来源中唯一卡死路径已被现有分支覆盖。观察 37-01。
- **可疑-2（useTickingCountdown 时区依赖浏览器本地时区）**：与后端调度器（只信对齐钟）解耦、属展示层非决策层；本项目部署形态服务器与浏览器同机构同时区，无实际触发。判误报（与观察 14-02 同族）。
- **可疑-4（发布级/课程级 can_select 字段同名）**：类型隔离（`t` 为 `Publish & {...}`、`c` 为 `ClassItem`），JSX 内无同名混淆点，:270 仅字符串拼接无逻辑依赖、:944 只读课程级。判误报，可读性瑕疵而已。

### 前端已核无缺陷高风险区域（review37-frontend 逐项核验 + 主 gate 抽查）

目标自动保存链（防抖+savingRef/lastJson/targetRef/dirtyRef 串行化+假清空三闸+hasPublishes 驱动发布恢复重试+stateDataRef 双闸）、回显合并（echoedRef 一次/31-01 真合并/31-04 全清空/35-01 同发布补进/幽灵过滤/TDZ 规避）、handleBack（33-01 回显等待/32-01 flushedRev 收敛/3 轮上限）、在飞幂等全链（M29-01 Set/31-03 removing Set/deleting/generating/Login submit/activate/ConfigTab saving）、视图态清理（logout page/account-reselect 空账号/401 session 归属/F25-01/App-34-02/M28-01/adminName 持久化）、btn_type 三向契约、max_count=0 全语义五处同源、XSS/存储安全（dangerouslySetInnerHTML/innerHTML/eval/new Function 零命中、localStorage 仅 4 处均 try/catch 降级）、Tabs 受控兜底、手写弹窗 Esc+autoFocus、api 20s 超时。

### 观察项（本轮追加）

- 观察 37-01：App.tsx onUnauthorized 代理态守卫"不误杀有效令牌"vs"残留失效代理目标槽位"的张力——保留前者优先（数据完整/会话安全优先），后端会话吊销由 12h TTL 自清，维持观察。
- 观察 37-02：M37-01 修复后"名额未公布"课程在剩余排序中恒排最前——与 32-02"未公布≠最紧张"语义张力，82 门实际数据中未公布课程极罕见，触发概率低，维持观察。
