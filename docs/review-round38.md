# 第 38 轮（最终轮）全模块只读审查

## 范围与方法

本轮为 38 轮审查循环的收官轮，对 `backend/` 全部 Go 模块与 `web/` 全部前端源码做全量只读审查。验证手段：`go build ./...`、`go vet ./...`、`go test -race -count=1 ./...` 全绿（9 包），前端 `npm run build`（tsc -b + vite）通过。所有发现均对照根 CLAUDE.md 的历轮定案与决策锚评估，不重复已修复/已定案事项。

## 前端（F38 系列，MAJOR 一处）

### F38-01 MAJOR——回显 effect 首帧未到即置位 echoedRef，代理态下旧目标覆盖删除

**位置**：`web/src/routes/Select.tsx` 回显 effect（207-260 行区域）

**缺陷**：回显 effect 在 `stateData` 首帧尚未到达（`courses` 为 undefined）时即置位 `echoedRef`/`setEchoDone`。首帧未到 ≠ 无旧目标——`/state` 与 `/electives` 并发拉取，`/state` 首帧晚到或失败 retry 期间 `stateData` 是 undefined，旧目标尚未回显进 `selected`；此时用户（管理员代理态）点击课程改动 `rev`，防抖保存/返回 flush 用未合并的 `selected` 整包 PUT，把后端旧目标静默覆盖删掉（"添加一门"变"替换全部"）。

**触发路径**：管理员代理学生查看选课大厅 → 页面同时拉 `/electives`（课程列表，先到）与 `/state`（旧目标，晚到）→ 用户在 `/state` 到达前先点选课程 → echoedRef 已置位 → 回显被永久跳过 → flush/防抖只含新点课程 → 旧目标被覆盖删除。

**修复**：effect 顶部补 `if (stateData === undefined) return`——首帧未到继续等待绝不置位 echoedRef；`const courses = stateData.courses ?? []`；仅当 `courses.length === 0`（确证后端无旧目标）才置位 `echoedRef`/`setEchoDone`。其后真合并逻辑（按 publish_id、31-01/31-04/35-01、幽灵 publish_id 过滤）保持不变。

**决策锚**：F38-01——首帧未到（`stateData === undefined`）绝不置位 echoedRef，回显等待持续到首帧到达或确证 courses 为空。

**验证**：`npm run build`（tsc -b + vite）通过。commit `106a2a4`。

### 可疑项裁决（前端，全部维持现状）

- **可疑-1（handleBack 等待循环无视觉反馈）**：行为正确（等待是瞬时的），纯 UI 增强，维持观察 38-01。
- **可疑-2（electives 持续失败期间改动延迟落库）**：守卫拦下的是假清空/漂移脏块，改动等发布恢复由 M30-01 驱动重试，安全方向，维持观察 38-02。
- **可疑-3（回显 effect 依赖不含 account）**：App 挂载点 key={account}（F36-01）+ Select 内部 accountKey 复位守卫双重防线已覆盖，维持现状。
- **可疑-4（echoDone 打断退避 timer）**：数据不丢（retryState 保留），仅重试节奏变化，行为正确维持现状。

## 后端（M-38 系列，MINOR 两处）

### M-38-01 MINOR——实时人数复核结果块回锁后无账号复核，删号竞态最后一块裸露写点

**位置**：`backend/internal/scheduler/scheduler.go` spawnChain 实时复核结果块（1395-1441 行）

**缺陷**：实时人数复核（`classFullRealtime` → `IsClassFull`，网络往返最长 15s）在锁外执行后回锁，三路分支（1401 ErrUnauthorized → maybeRelogin + setState + AppendLog；1414 真满员 → markFullLocked 重建 full[acct]；1434 通用失败 → setStateLocked + AppendLog）**全部没有删号复核**。锁外网络段内管理员删除账号时，回锁照写会给已删账号落幽灵失败审计日志、重建 full/relogin 族幽灵 map 条目。`setStateLocked` 的 idx<0 守卫（1621-1623）只挡状态数组越界，挡不住落库与 map 写。

**与既有防线对比**：链顶 B30-01（双 ClientFor）、ErrUnauthorized 分支 B37-03、成功分支 B18-M2、手动路径 B20-01、重登成功分支 B21-03——实时复核结果块是删号竞态防线的**最后一块裸露写点**。

**修复**：1396 行回锁后统一补 `if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`——已删账号静默放弃整块（inflight 已在 1333 行清掉，无残留）。

**TDD 陷阱实证（测试形态修正）**：`TestRealtimeRecheckDeletedAccountDropsLog` 初版用"等 inflight 清理"作为等待形态——但 inflight 在 SelectClass 返回后（进实时复核之前）就已清理，等待它会让断言与 goroutine 落日志**并发执行**，测试在旧实现上假绿（实测 0.01s 通过，5.982s 全包）。修正：等 `chains` 活跃标记消失（= goroutine 的 defer 已执行、结果块全部落地）再断言。修正后旧实现真红（实际 1 行）→ 修复绿（0 行）。

**TDD**：`TestRealtimeRecheckDeletedAccountDropsLog`（新增）——selectErr[61115]=connection reset 走实时复核路径 → fullBlock 阻塞 IsClassFull 模拟复核在途 → 期间 removed["acct1"]=true 删号 → 放行 → 断言 logCount==0。修复前红（1 行）→ 修复后绿。

**决策锚**：M-38-01——实时复核结果块回锁后必须先复核 ClientFor 再进三路分支，已删静默 return；删号竞态写回防线族（B18-M2/B20-01/B21-03/B23-01/B30-01/B37-03/M-38-01）至此全闭合。

### M-38-02 MINOR——启动日志读 openTime 死字段而非 openTimeNow 运行时读取器

**位置**：`backend/internal/scheduler/scheduler.go` Start() 571 行

**缺陷**：启动日志 `s.openTime.Format(...)` 读的是 New 初始化时的 openTime 字段——管理员运行期热改开放时间（SetOpenTimeFn）后，启动日志仍显示初始值，与 tick/probe 实际生效值分叉，运维查"窗口时间设没设对"被误导。

**先实证再改**：grep 确认 `s.openTime` 字段全部读点（143/146/191/197 初始化、378-381 openTimeNow 兜底、571 日志三处）——改后 openTimeNow 是运行时读取器（openTimeFn 优先，nil 兜底 openTime），日志反映热改后生效值，无死字段风险。

**修复**：571 行 `s.openTime.Format` → `s.openTimeNow().Format`。

**决策锚**：M-38-02——启动日志与全部运行时判定统一走 openTimeNow 单读取器，热改后日志与生效值一致。

## 第 38 轮观察项维持清单（跨轮延续，全部维持现状）

- **38-01**：handleBack 等待循环无视觉反馈（UI 增强非缺陷）
- **38-02**：electives 持续失败期间改动延迟落库（M30-01 驱动重试，安全方向）
- **38-03**：HTTP 侧探测未接入单飞守卫（F13-M1/B14-I2 定案延续）
- **38-04**：Decrypt 死字段（B16-I1 延续）
- **38-05**：RemoveFull 死方法（B17-02 延续）
- **38-06**：probe per-account 错误静默（频率受 30s 节流约束）
- **38-07**：幽灵课程条目无 UI 提示（回显已过滤，安全方向不假清空）
- **38-08**：emptyRunsFor 死代码 / syncFailedWindow 写而不读
- **38-09**：PUT /api/admin/config 空变更先落库后报"无可应用配置项"（幂等 UI 不发包）
- **38-10**：task_log 无容量上限（纯网络失败期累积）
- **38-11**：App.tsx onUnauthorized 代理态守卫张力（观察 37-01 延续，维持"绝不误杀有效令牌"优先）
- **38-12**：未公布名额课程在剩余排序中恒排最前（观察 37-02 延续，82 门实际数据触发概率低）

## 已核无缺陷的高风险区域清单（本轮逐项复核）

- **windowClosedLocked 三判据单源**（B29-02/B32-01/B35-02）：主判据/判据2（syncFailStreak≥3 带"开放时间已过"）/判据3（EmptyProbeRuns≥3）open 单快照统一，EmptyProbeRuns 入账 10s 裕量（B21-02）与主判据裕量（B26-03）对称。
- **spawnChain 删号竞态防线族全闭合**：链顶 B30-01 → 取 client B30-01 → 成功分支 B18-M2 → 失效分支 B37-03 → 实时复核结果块 M-38-01（本轮补）。手动路径 B20-01、重登成功 B21-03、重登失败分支 B21-03 齐。
- **实时复核三路分支内部**：B19-03 失效对称重登、B23-01 满员分支 doneHas 复核、通用失败 doneHas 复核——本轮 M-38-01 补前置账号复核后，三路分支写点全部受"账号仍存在"守卫保护。
- **删账号 memory-first 四步序**（Remove → PurgeAccount → DeleteAccount → RevokeAccount）+ 透传矩阵（B15-M4/B26-02/B27-01/B27-02）。
- **全部落库点零吞错**：B33-01 家族（B34-03/B35-01/B36-01）全覆盖，grep 实证无 `_ =` 吞 SaveSuccess/SaveRefused/DeleteSuccess/AppendLog/UpdateIDToken/DeleteRefusedClass。
- **回显合并链（前端）**：F38-01 首帧守卫 + echoedRef 一次合并 + 31-01 真合并 + 31-04 全清空 + 35-01 同发布补进 + F19-01 幽灵过滤 + 34-01 stateDataRef 双闸。
- **handleBack**：33-01 回显等待 + 32-01 flushedRev 收敛 + M30-02 + F26-01 pendingSaving + 3 轮上限。
- **探测防轰炸**：30s 节流 + probing 单飞 + probeSem cap4 + 窗口关闭判据三判据 + open 零值守卫。
- **时钟校准**：syncing 调用侧置位、30s 失败退避、streak 只在成功清零、无客户端复位。

## 结论

第 38 轮（最终轮）未发现 CRITICAL 级缺陷。前端 MAJOR 一处（**F38-01 回显首帧守卫**）、后端 MINOR 两处（**M-38-01 实时复核结果块删号复核**、**M-38-02 启动日志读取器统一**）已落地修复并独立提交。全部可疑项裁决完毕（前端 4 项维持现状、后端可疑项经复核无真实触发路径）。删号竞态写回防线族至此全闭合，回显合并链首帧守卫补齐——38 轮全模块审查循环收官。

回归：`go build ./...` + `go vet ./...` + `go test -race -count=1 ./...`（9 包）全绿 + `npm run build`（tsc -b + vite）通过。
