# R126 后端只读审查报告

- 审查对象：`backend/`（Go 后端）
- 审查日期：2026-09-24
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R125 报告（commit 51ce8b0 之后版本）。R125（51ce8b0）之后 backend/ 零产品改动（`git diff 51ce8b0 -- backend/` 空、`git log -1 -- internal/scheduler/scheduler.go` 仍为 R93 纯注释提交 fcf9bfc）。
- 时间盒：35 分钟。背景记忆锚：R126 = 身份防线矩阵第四十一轮闭合。

## 一、逐项核验结果

### 1. 身份防线矩阵第四十一轮闭合 —— 零漂移

**git 基线**：`git diff --stat 51ce8b0 -- backend/` 空输出；五个核心文件（scheduler/api/accounts/db/session）`git status` 零修改。身份防线矩阵本体零产品改动，纯观察轮。

**sameClientFor 定义 + 7 调用点全数在位（与 R125 逐行一致性实测）**
- 定义：`scheduler.go:204-210`（注释 :199-203，`func (s *Scheduler) sameClientFor(acct string, chainClient Client) bool`，需持 s.mu）；`clientIdentity` :215-224（reflect 指针身份，nil/非指针返回 0）。
- 调用点 grep 命中 7 处代码 + 1 处注释引：
  - :850 — ProbeForAccount 回写段（M88-01 防线，锁内复核后写 openTimeDetected :856 / acctData :863 / acctDataAt :864；身份不符则整体放弃回写）
  - :1489 — spawnChain 失效分支（ErrUnauthorized；身份不符 delete inflight :1490 后静默放弃，重登 :1499 落在复核之后）
  - :1521 — spawnChain 成功分支（写 done :1529 + setStateLocked + AppendLog :1532 + SaveSuccess :1535）
  - :1551 — spawnChain 风控退避分支（markRateLimitedLocked :1555）
  - :1571 — spawnChain 窗口关闭分支（markFullLocked :1575）
  - :1600 — spawnChain 实时复核回锁后统一复核（六分支统一点，B43-02 第六分支；漏了此点则陈旧链命中新身份 ErrUnauthorized 会把 maybeRelogin/failed 写进重建身份）
  - :1635 — spawnChain 确证满员分支（doneHas 守卫 :1627 → markFullLocked :1639）

**写点抽查 5 类（本轮按锚点建议换类：acctDataAt / openTimeDetected / refused / reloginAt / tokenValid）全部持锁 + 复核后写入**
- `acctDataAt[acct]`（scheduler.go:864）：ProbeForAccount 回写段，sameClientFor 复核（:850）通过后同一持锁段写入，对齐钟（:835 取）。首核对全部五类。
- `openTimeDetected[acct]`（:856）：同上，与 acctDataAt 同一复核段。全校槽 `openTimeDetected["*"]`（:1108）由 probe() 写——语义为"全校共享识别槽"，无账号身份绑定，无需 sameClientFor（与全局帧 lastData 同族）。
- `refused[acct]`（:634 RestoreRefused 持锁；:1938-1939 MarkDone 手动重报清 key + DeleteRefusedClass 落库持锁；:2011 RemoveDone 写值持锁）：全部 s.mu 内写入。
- `reloginAt[acct]`（grep 全仓仅 2 处写入：:1231 决策段、:1261 成功分支）：决策段 :1231 在入口 ClientFor 存在性复核（:1208）与 reloginFail 退避判断之后；成功分支 :1261 在写回侧 ClientFor 复核（:1254）之后。无第四处隐藏写点。
- `tokenValid[acct]`（:1241 决策段置位 / :1262 成功分支清位 / MarkTokenValid :1316 清位 / PurgeAccount delete :507）：全部持 reloginMu→s.mu 双锁段。

**偶发写点专项（与 R125 同清单复走）：** MarkTokenValid（:1306-1319 同步取 reloginMu→s.mu，三键删除）；TryAcquireSubmit（:1896-1914 sync.Once 幂等释放）；releaseFullIfFreedLocked（:1781-1811 快照缺/空保持 full 不解封）；SubmitAll（:1342-1380 对齐钟 lastSubmit + 链顶 ClientFor 过滤 :1355）。行为均符合既有契约。

**手动五路 accountExists（handler.go）**
- 课程读 :255-256；手动报名 :305-306；手动退选 :397-398；写目标 :497-510（内联 LoadCredentials 循环比对，与 accountExists 同判据源）；状态读 :573-574。判据同源（handler.go:1109-1120 LoadCredentials 逐账号比对）。grep `accountExists` 命中 4 调用点 + :1109 定义；写目标内联实现了同语义。

**maybeRelogin 双侧**
- 入口决策侧 :1208（`if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }`，B43-01）；写回侧 :1254（B21-03）。spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin :1499；实时复核失效分支 :1610 同理。

### 2. OBSERVE-117-01 知识位第九轮 —— 确认在位

`scheduler.go:1254-1284` 逐行复读：
- :1254 回锁后先 `ClientFor(acct)` 复核 → 已删则整个成功分支静默放弃（relogging :1247 在复核前已清）。
- :1265-1274 重取当前注册表 client 的 `Token()` 落库（UpdateIDToken :1269）——绝不使用发起重登时旧身份 token；空串跳过；失败已 log。
- :1261 reloginAt 刷新 / :1262 tokenValid=false 均在复核后同一持锁段。
- 结构性保证链持续成立：写回侧取当前注册表 token = 本知识位核心，零漂移。

### 3. B110-01 审计链第十六轮 —— 零漂移

- 手动失败六处 AppendLog（全 `if err != nil { log.Printf }`）：select :362 / :374 / :381；exit :443 / :452 / :459。
- 成功路径审计行：select → MarkDone 内 AppendLog :1976（成功）；exit → RemoveDone 内 AppendLog :2029（成功）。set_targets :558 / logout :616 / delete_account :1055 均同款零吞错。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员切换 :1770 —— 全数 `if err != nil` 结构。
- 零吞错穷举：`grep -rn "_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)" internal/ --include="*.go"` → 零命中（产品 + 测试全范围）。
- `handler.go:387` `_ = d.Sched.MarkDone(...)` 与 `:467` `_ = d.Sched.RemoveDone(...)`：两者内部零吞错（R125 已判恒 nil 非吞错），本次复走确认，无新的吞错落库点。
- task_log 无 DELETE 消费者（store.go 仅 INSERT :206 + 前置 20000 行窗口查询 :231/:422），删账号 DeleteAccount（:388-414 六表事务）不删日志——审计保留为纯设计决策。

### 4. O105-01 抖动基线 —— 夹具在位 + 定向 race 实测绿

- 夹具零漂移：`socketPreheat`（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97）+ `readyProbe`（zhidao/client_test.go:88、accounts/manager_test.go:26、api/handler_test.go:139）全数在位。accounts 包无 socketPreheat（注释宣称 8 轮全绿实证，保留观察）。
- 本机 `where gcc` 默认 PATH 无 gcc（`go version` go1.26.8 windows/amd64）；探测到 `/d/mingw64/bin/gcc.exe`，借该工具链 PATH 前置跑：
  ```
  CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/
  ok  xuanke-auto/backend/internal/zhidao    1.759s
  ok  xuanke-auto/backend/internal/accounts  1.755s
  ```
  定向 race 双包全绿。若本机缺失 gcc 属环境差异非缺陷（本次找到 WinLibs 工具链，无缝覆盖）。
- 支撑测试：`TestMigrate|TestWindow|TestAdminStats|TestDeletedAccountRebuiltSameNameChain|TestSubmitSuspended` 定向跑 db+scheduler 全绿（0.562s/4.691s）。B42-02/B43-02 标注测试存在（scheduler_test.go:3183/3493）。

### 5. 新契约角度：会话吊销 RevokeAccount 全链路纵深（自选，此前未覆盖）

`handleAdminDeleteAccount`（handler.go:1015-1059）memory-first 顺序复走：
1. `Accounts.Remove`（:1040）→ 客户端注册表先摘，在飞链落库前复核立即失败静默放弃（B39-01 空窗根因消除）。
2. `Sched.PurgeAccount`（:1045）→ 调度器 13 项 map 全清（scheduler.go:498-518，含 openTimeDetected 识别槽/relogin 族）+ state.Courses 账号行清除。
3. `Store.DeleteAccount`（:1046）→ 六表事务清库（credentials/accounts/targets/success/refused/activations，:388-414）；日志保留。
4. `Sessions.RevokeAccount`（:1054）→ session/store.go:207-215 锁内遍历删除该账号全部会话，被删账号既有浏览器令牌立即失效，不等 12h TTL——会话层与调度器层双重收敛。

走查结论：四步各自持锁、顺序自洽，被删账号的五个身份防线复核点（spawnChain 六分支 / ProbeForAccount / 手动五路）全部从"删号"得到一次性收敛。补查 `ProbeNow`（:944-968）与 `maybeSyncClock` 的 AnyClient 均写**全校共享帧**（lastData/clockOffset）、无账号身份绑定语义，无需 sameClientFor——与全局帧设计自洽，非身份防线缺口。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| git 基线 | `git diff 51ce8b0 -- backend/` | 空（零产品改动） |
| 编译 | `go build ./...` | 通过 |
| 静态检查 | `go vet ./...` | 通过 |
| 定向 race | `PATH=/d/mingw64/bin:$PATH CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` | zhidao 1.759s ok / accounts 1.755s ok |
| 定向迁移/窗口/防线测试 | `CGO_ENABLED=0 go test -count=1 -run 'TestMigrate|TestWindow|TestAdminStats|TestDeletedAccountRebuiltSameNameChain|TestSubmitSuspended' ./internal/db/ ./internal/scheduler/` | db 0.562s ok / scheduler 4.691s ok |
| 全量包测试 | `go test -count=1 ./internal/scheduler/ ./internal/session/ ./internal/db/ ./internal/accounts/ ./internal/api/` | 五包全绿（16.3s/2.3s/3.1s/2.5s/14.7s） |
| 契约20 扫描 | grep Go 源码 `R1[0-9][0-9]` / （第N轮） | 产品 + 测试双零命中 |
| 轮次标签族 | grep `P-[0-9]|F[0-9]{2}-|B[0-9]{2}-|OBSERVE-[0-9]` | 产品注释零命中（测试断言文案 B29/B42/B43 为契约锚定，非标签残留） |
| 零吞错穷举 | grep `_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)` | 零命中 |
| P-2 writeJSON 实测 | `go test -bench 'BenchmarkJSONWrite' -benchmem ./internal/api/` | map 721B/15alloc vs struct 192B/6alloc（详情见下） |

**P-2（writeJSON struct 化）独立核验**：
- 实测基准：`BenchmarkJSONWriteMap 2088ns/op 721B/15alloc` vs `BenchmarkJSONWriteStruct 1084ns/op 192B/6alloc`——R125 记录的 alloc 73% 降幅、内存 720→192B 复现（struct 版更快 ~48%）。
- API 响应体 struct（handler.go:78-82）字段恒输出无 omitempty，nil data 序列化 null，与旧 map 逐字节兼容（TestJSONBodyMapStructEquivalent 实测绿）。
- 前端契约（client.ts `j: {code,data,msg}` 逐字段读）不受 key 顺序影响；writeJSONStatus 家族 HTTP 状态码路径未变。可回退（commit c4a0b36 自带回退注释）。

## 三、分级发现

**无新增发现（零 CRITICAL/MAJOR/MINOR）。**

两条 OBSERVE 观察项维持（与 R125 一致，均非本轮引入、确定性边界已知）：
1. scheduler.go:1068 probeSem cap=4 常驻——>50 账号部署时 per-account 探测并发排队可能拉长单账号探测等待。ponytail 注释已标注（"若平台放宽熔断或账号数 >50 再调"），维持观察。
2. handler.go:553-557 目标保存双写库路径（handler 层直落 + scheduler 接口二次落库 enrich 发布元数据覆盖），崩溃一致性已确认成立，属防御性冗余而非缺陷，维持观察。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线 | R125（51ce8b0）后 backend/ 零产品改动 |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635） |
| 写点抽查 5 类（本轮换类） | acctDataAt/openTimeDetected/refused/reloginAt/tokenValid 全持锁 + 复核后写入，reloginAt 全仓仅 2 写点 |
| 偶发写点四类 | MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 全契约符合 |
| 手动五路 accountExists + maybeRelogin 双侧 | 全数在位（:255/:305/:397/:497-510/:573 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 9 轮确认在位（写回侧当前注册表 ClientFor→重取 Token() 落库，不串旧身份） |
| B110-01 审计链 | 第 16 轮零漂移（手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 定向 race 双包全绿（1.759s/1.755s，借 /d/mingw64 gcc；若缺 gcc 属环境差异） |
| 新契约角度 | 会话吊销 RevokeAccount 全链路 memory-first 四步收敛——无漏洞；ProbeNow/时钟 AnyClient 写全校帧无需身份复核设计自洽 |
| 契约20 轮次标签 | 产品 + 测试文件双零命中 |
| P-2 writeJSON | struct 化零行为变更（基准复现 alloc 6→15、等价断言绿、前端 body 契约 + HTTP 状态族不变），可回退 |

## 五、收尾总结

第四十一轮身份防线矩阵闭合：零产品改动链第二十二轮延续（COUNT=1）。7 个 sameClientFor 调用点、本轮换类 5 类写点（acctDataAt/openTimeDetected/refused/reloginAt/tokenValid）、4 类偶发写点、手动五路 accountExists、maybeRelogin 双侧全数在位且语义自洽。OBSERVE-117-01 第九轮、B110-01 第十六轮、O105-01 均零漂移与实测绿。新角度（会话吊销全链路 + ProbeNow/时钟全校帧身份语义边界）未发现盲区。主控叠加的 P-2 writeJSON struct 化经独立基准复现确认零行为变更且性能不降反升。进度 127/256。
