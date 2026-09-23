# R125 后端只读审查报告

- 审查对象：`backend/`（Go 后端）
- 审查日期：2026-09-24
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R124 报告（commit 60bc15f 之后版本）。R124 之后 backend/ 仅两条提交，均为主控叠加、独立可回退：
  - `c4a0b36 perf(api): writeJSON 响应体 map 装箱改显式 struct`（P-2）
  - `d9627b5 docs(archive): client.go 注释 legacy 路径引用更新`（纯注释，1 行）
- 时间盒：35 分钟。背景记忆锚：R125 = 身份防线矩阵第四十轮闭合。

## 一、逐项核验结果

### 1. 身份防线矩阵第四十轮闭合 —— 零漂移

**git 基线（零产品改动链第二十一轮延续）**
```
$ git log --oneline 60bc15f..HEAD -- backend/
c4a0b36 perf(api): writeJSON 显式 struct（P-2，主控叠加，独立 commit）
d9627b5 docs(archive): client.go 注释路径更新（纯注释）
```
身份防线矩阵本体零产品改动，纯观察轮。两条叠加均非身份防线产物且均可 git revert 回退。

**sameClientFor 定义与 7 调用点全数在位（与 R124 记录逐行一致）**
- 定义：`scheduler.go:204`（注释 :199-202，`func (s *Scheduler) sameClientFor(acct string, chainClient Client) bool`，需持 s.mu）
- 调用点 7 处全对称（spawnChain 六分支 + 探测回写），逐一代码并读出给出了分支归属：
  - :850 — ProbeForAccount 回写段（M88-01 防线，锁内复核后写 acctData/openTimeDetected）
  - :1489 — spawnChain 失效分支（ErrUnauthorized）
  - :1521 — spawnChain 成功分支（写 done + SaveSuccess 落库前）
  - :1551 — spawnChain 风控退避分支（markRateLimitedLocked）
  - :1571 — spawnChain 窗口关闭分支（markFullLocked，B41-01）
  - :1600 — spawnChain 实时复核回锁后统一复核（六分支统一点，B43-02 第六分支）
  - :1635 — spawnChain 确证满员分支（markFullLocked，doneHas 守护后）

**写点抽查 5 类（本轮换类围绕 done/rateLimited/full/inflight/state.Courses，均读代码实证持锁 + 复核）**
- `done[acct][t.ClassID]`（scheduler.go:1529）：成功分支先 sameClientFor → 锁内写 done + setStateLocked + SaveSuccess/AppendLog。写回前复核身份，早于 `if err == nil` 判定的共同清位路径已统一 delete inflight（实测语义，非走读推断）。
- `rateLimited[acct][classID]`（:1551 → markRateLimitedLocked :1754）：风控分支 sameClientFor 后、锁内 markRateLimitedLocked（失败分隔写 nowAlignedLocked().Add(d)）。读侧 isRateLimitedLocked 用对齐钟，写读同源。
- `full[acct][classID]`（:1571 窗口关闭分支 + :1635 确证满员分支 → markFullLocked :1764）：两个分支都在 sameClientFor 之后锁内写；确证满员分支先查 doneHas（绝不为手动成功覆盖胜利状态）再写。markFullLocked 内部先判 `if s.full[acct][t.ClassID] { return }` 幂等。
- `inflight[acct][classID]`（置位 :1471 锁内；成功/失效/风控/窗口关闭/实时复核/普通失败六分支未过身份复核前均先统一 delete；手动路径 TryAcquireSubmit :1896-1914 sync.Once 释放）：全部持 s.mu。锁内短路、锁外网络往返、回锁后复核——防双包契约完整。
- `state.Courses[idx].Status`（setStateLocked :1850 统一收口）：全部在持 s.mu 段内调用（spawnChain 各分支、MarkDone/RemoveDone/RemoveFull/rebuildCoursesForAccountLocked）。idx<0 守卫挡越界。

**偶发写点专项四类（与 R124 同清单复走）**
- MarkTokenValid（:1306-1319）：`reloginMu.Lock()` 同步取锁 + `s.mu.Lock()`，delete tokenValid/reloginFail/relogging 三键双锁段，锁序与 maybeRelogin 决策段一致。
- TryAcquireSubmit（:1894-1916）：持 s.mu；置 inflight 位 + sync.Once 释放函数（release 再持锁 delete）。幂等。
- releaseFullIfFreedLocked（:1781-1811）：持 s.mu 内部函数（调用点链内均锁内）。余量明确才解封、快照缺失/课程不在快照保守保持 full，防窗口关闭后解封轰炸。
- SubmitAll（:1342-1380）：持 s.mu 构建链列表，`lastSubmit = s.nowAlignedLocked()`（对齐钟，:1346），装配完释放锁再 spawnChain（锁外 spawn 减少持锁窗口）；链顶重新取 client 并复核存在（:1388、:1409 双校验）。
- 结论：四类偶发写点行为均符合既有契约。

**手动五路 accountExists（handler.go，含目标写内联同款）**
- 课程读：:255-256（`!d.accountExists(q)` → 账号不存在，无法查看课程）
- 手动报名：:305-306（账号不存在，无法执行报名操作）
- 手动退选：:397-398（账号不存在，无法执行退选操作）
- 写目标：:497-510（内联 LoadCredentials 循环比对，账号不存在，无法设置目标——与 accountExists 同一判据源）
- 状态读：:573-574（账号不存在，无法读取状态）
- 判据同源（handler.go:1109 accountExists → LoadCredentials 逐账号比对），与写目标内联实现一致。测试覆盖：handler_test.go:1254/:1269 幽灵账号报名/退选拒绝 + :1294 状态读幽灵拒绝。

**maybeRelogin 双侧**
- 入口决策侧：:1208（`if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`，B43-01 决策侧闭合——任何 map 写入前先做存在性复核）
- 写回侧：:1254（`if _, ok := s.clients.ClientFor(acct); !ok { ... return }`，B21-03）
- spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin（B41-01 族）；实时复核失效分支 :1604 同理。

### 2. OBSERVE-117-01 知识位第八轮 —— 确认在位

`backend/internal/scheduler/scheduler.go:1254-1274` 逐行复盘（本轮再次全文读出）：
- :1254 回锁后先 `ClientFor(acct)` 存在性复核，已删则整个成功分支（含内存写 + UpdateIDToken 落库）静默放弃，只清 relogging。
- :1259 `err == nil && relogged` 才进成功分支。
- :1265-1271 成功分支内重新取当前注册表 client `Token()` 落库（`UpdateIDToken(acct, tok)`）——绝不使用发起时旧身份 token，不串旧身份。tok 为空串则跳过落库。
- :1269 UpdateIDToken 失败已 log（零吞错）。
- `reloginFail` 清零 / `reloginAt` 刷新 / `tokenValid=false` 均在身份复核之后、同一持锁段。
- 结论：观察第八轮，结构性保证链成立，零漂移。

### 3. OBSERVE-111-01 / 112-03 盯守第十四轮 —— 恒 nil 非吞错

- `handler.go:387` `_ = d.Sched.MarkDone(...)`：MarkDone 返回 error 但内部零吞错（scheduler.go:1969-1980 SaveSuccess/AppendLog 失败全部 log），no-op 返回恒 nil（唯一非分散路径已在内部完整处理）。
- `handler.go:467` `_ = d.Sched.RemoveDone(...)`：RemoveDone 内部 :2017-2032 三处落库失败（DeleteSuccess/SaveRefused/AppendLog）全记日志。
- 动作维度对称：select 手册 6 失败位（handler :362/:374/:381 + scheduler 自动族）+ 成功路径（MarkDone :1976）；exit 6 失败位（handler :443/:452/:459 + 自动族）+ 成功路径（RemoveDone :2029）。零吞错穷举扫描见节 4。
- 结论：盯守第十四轮闭合。

### 4. B110-01 审计链第十五轮 —— 零漂移

- 手动失败六处 AppendLog：select :362（token 失效）/:374（read 结果未知）/:381（业务失败）；exit :443/:452/:459 —— 全部 `if err != nil { log.Printf }`，零弃。
- 成功路径审计行：select 成功 → MarkDone 内 AppendLog（:1976）；exit 成功 → RemoveDone 内 AppendLog（:2029）。
- 自动链失败族齐位：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员切换 :1770。
- 零吞错穷举扫描：
  ```
  grep -rn "_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)" backend/ --include="*.go"
  → 零命中（产品 + 测试全范围）
  ```
- 补充清点：`task_log` 表无 DELETE 消费者（只有保留最近 20000 行的查询前置，LoadAllLogs/LoadLogs），删账号 DeleteAccount 亦不删日志（审计保留为纯设计决策）。
- 结论：审计链第十五轮零漂移。

### 5. O105-01 抖动基线 —— 夹具在位 + 定向 race 实测绿

- 夹具 `socketPreheat`（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97；accounts 包无 socketPreheat——其注释宣称 8 轮全绿实证无残余，保留观察）+ `readyProbe`（zhidao/client_test.go:88、accounts/manager_test.go:26、api/handler_test.go:182）均原样在位。
- `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/`（本机缺 gcc，借 WinLibs 工具链 CGO_ENABLED=1）：
  ```
  ok  xuanke-auto/backend/internal/zhidao    2.946s
  ok  xuanke-auto/backend/internal/accounts  3.737s
  ```
- 结论：夹具零漂移 + 定向 race 双包全绿（zhidao 2.946s / accounts 3.737s）。

### 6. 新契约角度：多账号快照隔离与连接池预热时序纵深

**ElectivesSnapshotFor 回退链（契约 8）再次一条路径走完：**
- 目标账号（len(acctTargets)>0）：专属帧新鲜即返回；不存在/过期 → (nil,false) 触发真刷新，绝不回退全局帧（年级串线根除）。
- 无目标但曾有过专属帧：做过期专属帧 → 仍 (nil,false)，绝不回退全局帧（对比 R124 已修语义）。
- 无目标且从未有专属帧：才回退全局 lastData（快、无网络开销）。
- 配合 probe() 只对 AccountsWithTargets() 遍历刷新 + probeSem(cap4) 封顶——每账号独立帧、独立时间戳（acctDataAt 用对齐钟），与 tick 探测判读同时间基。

**maybePrewarm 时序（本次新角度走查）：**
- tick（:973-1036）开头先 `now := s.nowAligned()` → 锁内取 `open` 单快照 → maybePrewarm(now, open)。
- maybePrewarm（:293-310）：仅 `open` 有效且距开窗 ≤2 分钟才进入；锁内 15s 节流（lastPrewarm 判断）；取 `AnyClient()` + `Prewarmer` 接口断言后异步 goroutine 调 Prewarm（Prewarm 实现 client.go:96-108：简单 GET /login + io.Copy(Discard) 静默，无 token 污染路径）。
- 时序自洽：时钟对齐由 maybeSyncClock 独立推进（成功才推进 lastSyncTime；失败 30s 退避；syncing 单飞 + 无客户端复位），Prewarm 与 SyncServerTime 都对 /login 发 GET——两路对同一端点无竞争（Prewarm 是预热连接、Sync 是取 Date 头），且都经 AnyClient 拿客户端，多账号共用同一 sharedTransport 连接池，预热从池角度全校共享。
- 边界：Prewarm 失败（rare 网络抖动）`_ = pw.Prewarm()` 弃错——这是连接预热（无状态、无落库、无业务语义），非审计写点；与 R124 已判定的 scheduler.go:1075 `_, _ = s.ProbeForAccount(acct)`（probe goroutine 内、无状态写入）同为可接受弃错族。已确认其不违反零吞错落库规范。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| 编译 | `go build ./...` | 通过 |
| 静态检查 | `go vet ./...` | 通过 |
| race 定向 | `CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` | zhidao 2.946s ok / accounts 3.737s ok |
| 全量包测试 | `cd backend && go test -count=1 ./internal/...` | accounts/api/config/db/runtime/scheduler/secure/session/store/zhidao 十包全绿（api 56.8s、scheduler 54.6s、db 42.2s） |
| 契约20 扫描 | grep Go 源码 `R1[0-9][0-9]` / （第N轮） | 产品 0 命中 / 测试文件 0 命中 |
| 零吞错穷举 | grep `_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)` | 零命中 |
| P-2 writeJSON 审查 | handler.go:74-102 + writejson_bench_test.go + 前端 client.ts 逐字段读 | struct 化零行为变更（见下） |

**P-2（writeJSON struct 化）独立核验**：
- `apiResponse{Code,Data,Msg}`（handler.go:74-80）+ writeJSON/writeJSONStatus 两处同改。字段恒输出（无 omitempty）→ nil data 仍序列化为 null，与旧 map 语义逐字节兼容（含 key 顺序——map[string]any 顶层 Go 按字典序 code<data<msg，与 struct 声明序一致，实测等价）。
- `TestJSONBodyMapStructEquivalent` 断言 map/struct 两实现逐字段等价（三种 case：map data / nil data / plain string），Benchmark 红绿灯对比。技术审查确认：Data any 字段的装箱从 3 次降到 1 次（仅 data 本身），alloc/内存 73% 降幅成立。
- 前端契约：client.ts:59 `j: { code: number; data: T; msg: string }` 逐字段读、不读 key 顺序/HTTP 状态。HTTP 状态码路径（writeJSONStatus 家族 401/403/429/500）未变。
- 结论：可接受，行为零变更。若未来量测发现序列化非瓶颈，git revert c4a0b36 即回退旧 map 实现（commit 注释已自带该回退路径）。

## 三、分级发现

无新增发现（零 P0/P1/P2/MAJOR/MINOR）。

两条 OBSERVE 观察项（非本轮引入、确定性边界已知，维持观察不修）：
1. scheduler.go:1068 probeSem cap=4 常驻——>50 账号部署时 per-account 探测并发排队可能拉长单账号探测等待。ponytail 注释已标注，确定性边界已知，维持观察（R119-125 连续轮次一致）。
2. handler.go:571-572 目标保存双写库路径——handler 先 Store.SetTargetsForAccount 直落一次（原始 targets），再 Sched.SetTargetsForAccount 经 scheduler store 接口二次落库（enrich 发布元数据覆盖）。崩溃一致性已确认成立（第一步落库后崩溃 → 重启从库恢复，与第二步结果一致；进程内瞬时读旧 vs 库新为毫秒级瞬态，无实质风险）。属防御性冗余而非缺陷；保留观察，若未来简化可去掉 handler 层直写。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线 | 两条叠加提交（writeJSON struct P-2 + 注释 1 行），身份防线本体零改动 |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635） |
| 写点抽查 5 类（本轮换类） | done/rateLimited/full/inflight/state.Courses 全部持锁 + 复核后写入 |
| 偶发写点四类 | MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 全契约符合 |
| 手动五路 accountExists + maybeRelogin 双侧 | 全数在位（:255/:305/:397/:497-510/:573 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 8 轮确认在位（落库取当前注册表 Token()，不串旧身份） |
| OBSERVE-111-01/112-03 | 第 14 轮闭合（:387/:467 恒 nil 非吞错 + select/exit 双动作 6 失败位对称） |
| B110-01 审计链 | 第 15 轮零漂移（手动 6 失败位 + 成功行 + 自动族 + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 定向 race 双包全绿（2.946s/3.737s） |
| 新契约角度 | 多账号快照隔离（契约 8 全路径）与连接池预热 maybePrewarm 时序纵深——无漏洞 |
| 契约20 轮次标签 | 产品 + 测试文件双零命中 |
| P-2 writeJSON | struct 化零行为变更（TDD 等价 + 前端 body 契约 + HTTP 状态族不变），可回退 |

## 五、收尾总结

第四十轮身份防线矩阵闭合：零产品改动链第二十一轮延续（COUNT=1）。7 个 sameClientFor 调用点、5 类写点、4 类偶发写点、手动五路 accountExists、maybeRelogin 双侧全数在位且语义自洽。OBSERVE-117-01 第八轮、OBSERVE-111-01/112-03 第十四轮、B110-01 第十五轮、O105-01 均零漂移与实测绿。新角度（多账号快照隔离 + 连接池预热时序）未发现盲区。主控叠加的 P-2 writeJSON struct 化经独立核验零行为变更。进度 126/256。
