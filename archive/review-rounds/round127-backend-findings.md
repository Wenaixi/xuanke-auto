# R127 后端只读审查报告

- 审查对象：backend/（Go 后端）
- 审查日期：2026-09-24
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R126 报告（commit b42a539）。R126（b42a539）之后 backend/ 零产品改动（git diff --stat b42a539 -- backend/ 空输出）。
- 时间盒：35 分钟。背景记忆锚：R127 = 身份防线矩阵第四十二轮闭合。

## 一、逐项核验结果

### 1. 身份防线矩阵第四十二轮闭合 —— 零漂移

**git 基线**：git diff b42a539 -- backend/ 空输出；六个核心文件（scheduler/api/accounts/db/store/session）git status 零修改。身份防线矩阵本体零产品改动，纯观察轮。

**sameClientFor 定义 + 7 调用点全数在位（与 R126 逐行一致性实测）**
- 定义：scheduler.go:204-210（注释 :199-203，func (s *Scheduler) sameClientFor(acct string, chainClient Client) bool，需持 s.mu）；clientIdentity :215-224（reflect 指针身份，nil/非指针返回 0）。
- 调用点 grep 命中 7 处代码（无任何调用点漂移或新增）：
  - :850 — ProbeForAccount 回写段（M88-01 防线，锁内复核后写 openTimeDetected :856 / acctData :863 / acctDataAt :864；身份不符则整体放弃回写）
  - :1489 — spawnChain 失效分支（ErrUnauthorized；身份不符 delete inflight :1490 后静默放弃，重登 :1499 落在复核之后，先复核再 maybeRelogin 语义完整）
  - :1521 — spawnChain 成功分支（写 done :1529 + setStateLocked + AppendLog :1532 + SaveSuccess :1535）
  - :1551 — spawnChain 风控退避分支（markRateLimitedLocked :1555）
  - :1571 — spawnChain 窗口关闭分支（markFullLocked :1575）
  - :1600 — spawnChain 实时复核回锁后统一复核（六分支统一点，B43-02 第六分支）
  - :1635 — spawnChain 确证满员分支（doneHas 守卫 :1627 至 markFullLocked :1639）

**写点抽查 5 类（本轮换类：done / full / inflight / rateLimited / state.Courses）全部持锁 + 复核后写入**
- done[acct][id] 三写点：RestoreDone :611-615（持锁注入）、spawnChain 成功 :1526-1529（sameClientFor :1521 复核通过后同一持锁段）、MarkDone :1931-1934（入口 ClientFor 存在性复核 :1927 后持锁）。全部在 s.mu 内。
- full[acct][classID] 单写点族：markFullLocked :1760-1767（唯一写入函数，全 *Locked 命名后缀 = 持锁专用，调用方全已持锁）；markRateLimitedLocked :1710-1714 同理（rateLimited 唯一写点）。类级约束成立：分类 map 的写函数全部 *Locked 后缀，恰是"写点全部持锁"的结构性证据；触发点分属 spawnChain 风控/窗口关闭/确证满员分支，全部在 sameClientFor :1551/:1571/:1635 复核之后。
- inflight[acct][classID] 两写点：spawnChain :1468-1471（锁内置位，置位后立即放锁再 SelectClass）、TryAcquireSubmit :1899-1905（sync.Once 幂等释放族）。失效分支清位 :1490/:1501、成功统一清位 :1513、无残留。
- state.Courses 五写点：PurgeAccount :519 滤行重写（持锁）、:578/592 重建重写（持锁）、MarkDone :1964 追加（持锁 + 入口复核）、StateForAccount :721（读侧拷贝，不含写）。spawnChain 内全部经 setStateLocked :1846（内部上锁，构造 idx<0 越界守卫）。新增观察确认：setStateLocked 是 courses 状态行的唯一就地写入闸门。
- 偶发写点专项（R126 清单复走）：MarkTokenValid :1307-1321（同步取 reloginMu 到 s.mu 三键删除）；TryAcquireSubmit :1896-1914（sync.Once 幂等释放）；releaseFullIfFreedLocked :1781-1811（快照缺/空保持 full 不解封，防窗口关闭后解封轰炸）；submitAll :1342-1380（对齐钟 lastSubmit，链顶 ClientFor 过滤 :1355）。行为均符合既有契约。

**手动五路 accountExists（handler.go）全数在位**
- :255-256 课程读；:305-306 手动报名；:397-398 手动退选；:497-510 写目标（内联 LoadCredentials 循环比对，与 accountExists 同判据源）；:573-574 状态读。判据定义 :1109-1120（LoadCredentials 逐账号比对 + 含异常兜底）。

**maybeRelogin 双侧**
- 入口决策侧 :1208（if _, ok := s.clients.ClientFor(acct); !ok { s.mu.Unlock(); return }，B43-01 存在性复核在 reloginMu 到 s.mu 双锁内、任何 map 写入前）；写回侧 :1254（B21-03）。决策段 :1231 reloginAt / :1233 reloginFail++ / :1236 relogging / :1241 tokenValid 全部在复核后同一持锁段。spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin :1499；实时复核失效分支 :1608-1610 同理。

### 2. OBSERVE-117-01 知识位第十轮 —— 确认在位

scheduler.go:1254-1284 逐行复读：
- :1254 回锁后先 ClientFor(acct) 复核 → 已删则整个成功分支静默放弃（只清 relogging :1247，不写 tokenValid/reloginAt/库行）。
- :1265-1274 复核通过后重取当前注册表 client 的 Token()（if client, ok := s.clients.ClientFor(acct); ok { if tok := client.Token(); ... }）落库（UpdateIDToken :1269）——绝不使用发起重登时旧身份的 token；空串跳过；失败已 log（:1270）。
- :1261 reloginAt 刷新 / :1262 tokenValid=false 仍在复核后同一持锁段。
- 结构性保证链持续成立：写回侧取当前注册表 token = 本知识位核心，零漂移第十轮。

### 3. B110-01 审计链第十七轮 —— 零漂移

- 手动失败六处 AppendLog（全 if err != nil { log.Printf }）：select :362 / :374 / :381；exit :443 / :452 / :459。
- 成功路径审计行：select 至 MarkDone 内 AppendLog :1976（成功）；exit 至 RemoveDone 内 AppendLog :2029（成功）。MarkDone :1980 落库 SaveSuccess、RemoveDone :2023/:2028 双落库 DeleteSuccess/SaveRefused。set_targets :558 / logout :616 / delete_account 均同款零吞错。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员切换 :1770 —— 全数 if err != nil 结构。
- 零吞错穷举：grep -rn "_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)" internal --include=*.go（全仓产品+测试）→ 零命中。
- handler.go:387 _ = d.Sched.MarkDone(...) 与 :467 _ = d.Sched.RemoveDone(...)：两者内部零吞错（R126 已判恒 nil 非吞错），本次复走确认。
- task_log 无 DELETE 消费者（store.go 仅 INSERT :206 + 前置 20000 行窗口查询 :231/:422），删账号 DeleteAccount（:388-414 六表事务）不删日志——审计保留纯设计决策。

### 4. O105-01 抖动基线 —— 夹具在位 + 定向 race 实测绿

- 夹具零漂移：socketPreheat（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97）+ readyProbe（zhidao/client_test.go:88、accounts/manager_test.go:26、api/handler_test.go:139）全数在位。accounts 包无 socketPreheat（注释宣称 8 轮全绿实证，包序保护 + readyProbe 兜底，维持观察）。
- 本机 where gcc 默认 PATH 无 gcc（go version go1.26.8 windows/amd64）；探测到 /d/mingw64/bin/gcc.exe，借该工具链 PATH 前置（首步即做，R125/R126 教训沉淀内置）：
  export PATH=/d/mingw64/bin:$PATH 且 CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/
  ok  xuanke-auto/backend/internal/zhidao    3.324s
  ok  xuanke-auto/backend/internal/accounts  2.236s
  定向 race 双包全绿。
- 支撑定向 race（借同工具链）：CGO_ENABLED=1 go test -race -count=1 -run 'TestScheduler|TestWindow|TestAdminStats|TestMigrate|TestPurge|TestRelogin|TestDeleted|TestSubmit' ./internal/scheduler/ ./internal/db/ 至 scheduler 14.649s ok / db 2.375s ok。
- 全量 api race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ 至 20.267s ok（含 B42/B43 登录闸门断言族）。

### 5. 新契约角度：认证全链路 gateWait/gateTryAcquire 闸门族（自选纵深）

accounts/manager.go 全局 doLogin 频率闸门（安全审计核心：防出口 IP 锁号）逐路实证：

**闸门计数唯一源**：gateUsed 全仓仅两处写入——gateWait :60（阻塞版，自动重登用）+ gateTryAcquire :231（非阻塞版，手动登录用），后者在 LoginByPassword :244 首行调用；两者共享同一 gateMu 锁与 gateLoginPerMin=2 计数，排队重登与手动登录严格共享全账号每分钟 doLogin 预算（互斥 + 同一节奏）。伪满分支计数器永不为 0 越界（time.Since(m.gateWindow) 大于等于 time.Minute 才重置窗口起点）。

**自动侧（Relogin）**：scheduler 走接口到真实实现 Manager.Relogin :166-176 先取客户端再 m.gateWait() 再 ReloginIfNeeded（幂等判定在 zhidao/client.go:602-614，内部用客户端自身 account/password，绝不被调用方参数影响）——B42-01 闸门完全生效。

**手动侧（LoginByPassword）**：:244 !m.gateTryAcquire() 立即拒绝"登录尝试过于频繁，请稍后再试"，绝不阻塞用户登录响应（非阻塞语义正确）。管理员入口（handleLogin :131 双条件后才走教务；撞名学生/普通学生手动登录）全部经此收口。超预算时不注册空壳客户端——测试断言"被拒后 ClientFor('acct1') 必须不存在"。

**测试覆盖实证（TDD 红绿锚）**：
- TestLoginByPasswordRejectsWhenGateBudgetExhausted（manager_test.go:174-196）：gateUsed 置满后拒绝且 doLogin 0 次（断言 calls==0）。
- TestLoginByPasswordAllowedWhenGateBudgetAvailable（:198-239）：跨窗口重置后放行，消耗 1 次预算且恰好触达 1 次 doLogin（断言 calls==1 且 gateUsed==1）。
- TestLoginAdminWrongPasswordTimingFlat（handler_test.go:1485-1505）：管理员名 + 非管理口令走教务登录（撞名学生走教务成功分支，绝不回"管理口令错误"）。
- TestLoginAdminNameCollisionStudentCredential：撞名学生教务登录成功签发普通会话，绝不被管理员分支吞掉。
- api 测试夹具 authenticateDirect 批量注册借 ResetGateForTest（:364-368）保证不被闸门误拦；测试专用，正式代码零调用。

**安全审计结论**：认证全链路闸门族成立——自动/手动两路共享单一预算、阻塞/非阻塞语义各自正确、管理员换绑同收口、测试双断言钉死计数共享。未发现新缺口。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| git 基线 | git diff b42a539 -- backend/ | 空（零产品改动） |
| 编译 | go build ./... | 通过（BUILD_EXIT=0） |
| 静态检查 | go vet ./... | 通过（VET_EXIT=0） |
| 定向 race | PATH=/d/mingw64/bin:$PATH 且 CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ | zhidao 3.324s ok / accounts 2.236s ok |
| 定向防线 race | CGO_ENABLED=1 go test -race -count=1 -run 定向模式 ./internal/scheduler/ ./internal/db/ | scheduler 14.649s ok / db 2.375s ok |
| 全量 api race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ | 20.267s ok |
| 契约20 轮次标签扫描 | grep Go 源码 第[0-9N一-九]*轮 / R[0-9]{2,}-[0-9] | 0 命中 |
| 零吞错穷举 | grep _ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused) | 零命中 |

**gcc 状态说明**：where gcc 默认 PATH 无 gcc；ls /d/mingw64/bin/gcc.exe 存在 → 借 mingw64 工具链（export PATH=/d/mingw64/bin:$PATH）跑 CGO=1 race。若未来机器缺该工具链，CGO_ENABLED=0（无 race）仍可编译 + 全绿非 race 全包验证（R125 已跑过），race 缺口属环境差异非缺陷。

## 三、分级发现

**无新增发现（零 CRITICAL/MAJOR/MINOR）。**

三条 OBSERVE 观察项（与 R126 一致，均非本轮引入、确定性边界已知）：
1. scheduler.go:1068 probeSem cap=4 常驻——大于 50 账号部署时 per-account 探测并发排队可能拉长单账号探测等待。ponytail 注释已标注（"若平台放宽熔断或账号数大于 50 再调"），维持观察。
2. handler.go:553-557 目标保存双写库路径（handler 层直落 + scheduler 接口二次落库 enrich 发布元数据覆盖），崩溃一致性已确认成立，属防御性冗余，维持观察。
3. scheduler.go:293-305 maybePrewarm 无独立单元测试（与 probeIntervalFor/submitIntervalFor 有测试形成反差）——本函数靠 go vet/编译 + 行为简单（三行守卫 + 15s 节流 + AnyClient goroutine 预热）保障，可观测性窗口期 2 分钟短且耗时 15s 的预热语义与 tick 主干分享同一测试基建。候选注释"预热语义应由集成级观测验证，新增进度契约需同步补测试"，维持观察。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线 | R126（b42a539）后 backend/ 零产品改动 |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635） |
| 写点抽查 5 类（本轮换类） | done/full/inflight/rateLimited/state.Courses 全持锁 + 复核后写入；*Locked 后缀 = 写点持锁结构性证据 |
| 偶发写点四类 | MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 全契约符合 |
| 手动五路 accountExists + maybeRelogin 双侧 | 全数在位（:255/:305/:397/:497-510/:573 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 10 轮确认在位（写回侧当前注册表 ClientFor 后重取 Token() 落库，绝不串旧身份） |
| B110-01 审计链 | 第 17 轮零漂移（手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 定向 race 全绿（zhidao 3.3s/accounts 2.2s/scheduler 14.6s/db 2.4s/api 20.3s，借 /d/mingw64 gcc；缺 gcc 属环境差异） |
| 新契约角度 | 认证全链路 gateWait/gateTryAcquire 闸门族——自动/手动共享单一预算、非阻塞语义正确、测试双断言钉死计数，无新缺口；maybePrewarm 无独立测试列候选观察 |
| 契约20 轮次标签 | 产品 + 测试文件双零命中 |
| P-2 writeJSON | 非本轮基准；R126 已实测 struct 化零行为变更（alloc 73% 降、HTTP 状态族不变），维持 |

## 五、收尾总结

第四十二轮身份防线矩阵闭合：零产品改动链延续（第五轮纯观察）。7 个 sameClientFor 调用点、本轮换类 5 类写点（done/full/inflight/rateLimited/state.Courses）、4 类偶发写点、手动五路 accountExists、maybeRelogin 双侧全数在位且语义自洽——新增类级证据：分类 map 写函数全部 *Locked 后缀（结构性保证"写点全部持锁"）。OBSERVE-117-01 第十轮、B110-01 第十七轮、O105-01 均零漂移与实测绿（借 mingw64 gcc 完成五包 race）。新角度（认证全链路闸门族）未发现盲区，maybePrewarm 测试覆盖差列为维持观察。进度 128/256。
