# R128 后端只读审查报告

- 审查对象：backend/（Go 后端）
- 审查日期：2026-09-24
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R127 报告（commit 08b43da）。R127（08b43da）之后 backend/ 零产品改动（git diff --stat 08b43da -- backend/ 空输出；六个核心文件 scheduler/api/accounts/db/store/session git status 零修改）。
- 时间盒：35 分钟。背景记忆锚：R128 = 身份防线矩阵第四十三轮闭合。

## 一、逐项核验结果

### 1. 身份防线矩阵第四十三轮闭合 —— 零漂移

**git 基线**：R127（08b43da）后 backend/ 零产品改动。身份防线矩阵本体连续第六轮纯观察（R127 起零产品改动链延续）。

**sameClientFor 定义逐行**
- :199-210 注释 + func 定义（"仅判账号名存在挡不住同名重建""需持 s.mu"），:204 定义、:209 核心比较、:212-224 clientIdentity（reflect 指针身份，nil/非指针返回 0）。与 R127 逐行一致，零漂移。

**7 个调用点精确行号（grep 实测，与 top 锚点一一对应 ZERO 漂移）**
| 锚点 | 实测行号 | 语义 |
|------|---------|------|
| :850 | :850 | ProbeForAccount 回写段（M88-01 防线；复核失败整体放弃回写，绝不动 acctData/openTimeDetected） |
| :1489 | :1489 | spawnChain 失效分支（先复核再 maybeRelogin :1499，语义顺序完整保留） |
| :1521 | :1521 | spawnChain 成功分支（done :1529 + setStateLocked + AppendLog :1532 + SaveSuccess :1535 全在复核通过后同一持锁段） |
| :1551 | :1551 | spawnChain 风控退避分支（markRateLimitedLocked :1555） |
| :1571 | :1571 | spawnChain 窗口关闭分支（markFullLocked :1575） |
| :1600 | :1600 | spawnChain 实时复核回锁后统一复核（六分支统一点，B43-02 第六分支） |
| :1635 | :1635 | spawnChain 确证满员分支（doneHas 守卫 :1627 至 markFullLocked :1639） |

**写点抽查 5 类（本轮换类：acctData / openTimeDetected / refused / reloginAt / tokenValid——前两轮已查 done/full/inflight/rateLimited/state.Courses，无组合重复）全部持锁 + 复核后写入**
- acctData[acct] 写点族：ProbeForAccount 回写 :863/:864（锁内 + sameClientFor :850 复核通过后同一持锁段）；ProbeNow :965/:966（持锁）；probe() 主帧 :1115/:1116（持锁）。无裸写。
- openTimeDetected[acct] 写点 :856（锁内 + sameClientFor 复核后）+ PurgeAccount 删除 :506（持锁）；openTimeDetected["*"] 全局槽 :1108（持锁）；读侧 openTimeForLocked :428-431 持锁读。识别槽删除点仅 PurgeAccount（"关闭不等于时间消失"契约保留槽），零裸写。
- refused[acct] 写点族：SetTargetsForAccount :458（锁内 delete 整账号）；RestoreRefused :633-634（持锁注入）；RemoveDone :2011（持锁 + 入口 ClientFor 复核 :1995）；MarkDone :1939（持锁 + 入口复核 :1927）；MarkDone 内 delete 单课 :1938-1939（持锁）。refused 写函数无 *Locked 后缀但全部在持锁函数体内（持锁语义等价），零裸写。
- reloginAt/tokenValid 写在 maybeRelogin 决策段 :1231/:1241（reloginMu 到 s.mu 双锁内、:1208 ClientFor 复核之后）+ 写回侧成功分支 :1261/:1262（锁内 + :1254 ClientFor 复核后 + :1265 重取注册表 token）。零裸写。
- 偶发写点复走（R127 清单）：MarkTokenValid :1306-1318（reloginMu 到 s.mu 双锁同步清除）；TryAcquireSubmit :1896-1914（sync.Once 幂等释放族）；releaseFullIfFreedLocked :1781-1811（快照缺/空保持 full 不解封守卫）；submitAll 链顶 ClientFor 过滤 :1405/:1409。行为均符合既有契约。

**类级结构证据（R127 新角度延续）**
- full 唯一写函数 markFullLocked :1760（*Locked 后缀 = 持锁专用）+ rateLimited 唯一写函数 markRateLimitedLocked :1710（同）；*Locked 后缀写函数族全数清点：写操作族（markFullLocked / markRateLimitedLocked / releaseFullIfFreedLocked / setStateLocked）+ 读/计算族（nowAlignedLocked / openTimeForLocked / enrichTargetPubMetaLocked / rebuildCoursesForAccountLocked / rebuildCoursesLocked / tokenValidForLocked / windowClosedLocked / isRateLimitedLocked / statusIndexLocked）。**写点全部落 *Locked 函数或持锁函数体内 = "写点全部持锁"的结构性保证成立**。
- state.Courses 写点全量清点：PurgeAccount 滤行重写 :512-518 / rebuildCoursesForAccountLocked 重建 :572-578 + :592-602（RestoreTargets :531 调用，持锁）/ MarkDone 追加 :1964-1970（持锁 + 入口复核）/ RemoveDone 就地改 :2014-2015（持锁 + 入口复核）/ spawnChain 内层层经 setStateLocked :1846（内部 idx 小于 0 守卫）/ releaseFullIfFreedLocked 解封 :1803-1804（持锁）。无一处裸写。

**手动五路 accountExists（handler.go）全数在位**
- :255-258 课程读；:305-308 手动报名；:397-400 手动退选；:497-512 handleSetTargets 内联 LoadCredentials 循环比对（与 accountExists 同判据源，第 5 路）；:573-576 状态读。判据定义 :1109-1120（LoadCredentials 逐账号比对 + 异常兜底）。四路 accountExists(q) 调用 + 第 5 路内联实现 = 五路全覆盖。

**maybeRelogin 双侧**
- 入口决策侧 :1208（ClientFor 存在性复核，reloginMu 到 s.mu 双锁内、任何 map 写入前——B43-01）；写回侧 :1254（B21-03 存在性复核，整个成功分支含落库静默放弃，只清 relogging :1247）。决策段 :1231 reloginAt / :1233 reloginFail 累加 / :1236 relogging / :1241 tokenValid 全在复核后同一持锁段。spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin :1499；实时复核失效分支 :1608-1610 同理。

### 2. OBSERVE-117-01 知识位第十一轮 —— 确认在位

:1244-1284 逐行复读：
- :1246-1247 失败/成功统一清 relogging（失败也清，才能再试）。
- :1254 ClientFor(acct) 复核 => 已删则整个成功分支静默放弃（不写 tokenValid/reloginAt/库行，只清 relogging）。
- :1259-1262 成功分支 delete(reloginFail) / reloginAt 刷新 / tokenValid=false 全在复核后同一持锁段。
- :1265-1274 **重取当前注册表 client 的 Token() 落库**（if client, ok := s.clients.ClientFor(acct); ok 后 client.Token() 非空才 UpdateIDToken :1269）——绝不使用发起重登时旧身份的 token；空串跳过；落库失败 log（:1270）。
- 结构性保证链持续成立：写回侧取当前注册表 token = 本知识位核心，零漂移第十一轮。

### 3. B110-01 审计链第十八轮 —— 零漂移

- 手动失败六处 AppendLog（全 if err != nil 结构）：select :362（token 失效）/ :374（read 类）/ :381（其余失败）；exit :443 / :452 / :459。
- 成功路径审计行：select 成功至 MarkDone :1976 AppendLog（成功）+ :1973 SaveSuccess；exit 成功至 RemoveDone :2020 DeleteSuccess + :2026 SaveRefused + :2029 AppendLog（成功）。set_targets :558 / logout :616 / delete_account :1055 均同款零吞错。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员 markFullLocked :1770 —— 全数 if err != nil 结构。
- 零吞错穷举：正则精确扫描 `_ = .*(AppendLog|Save*|Delete*|UpdateIDToken)` 与双下划线形式 -> 全仓产品+测试零命中。
- handler.go :387 与 :467 对 MarkDone/RemoveDone 的 `_ =` 调用：两者内部零吞错（内部实现逐个 if err != nil 记日志），非吞错点，判定成立。
- task_log 无 DELETE 消费者（store.go 仅 INSERT :206 + 20000 行窗口查询 :231），删账号六表事务不含日志——审计保留纯设计决策，未变化。

### 4. O105-01 抖动基线 —— 夹具在位 + 五包 race 实测绿

- 夹具零漂移：socketPreheat（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97 + TestMain :39-42 包级预热）+ readyProbe（zhidao/client_test.go:88、accounts/manager_test.go:26、api/handler_test.go:185 全数在位）。accounts 包继续无 socketPreheat（注释宣称 8 轮全绿实证，包序保护 + readyProbe 兜底，维持观察）。
- 本机 where gcc 默认 PATH 无 gcc；ls /d/mingw64/bin/gcc.exe 存在 => 借该工具链 PATH 前置（首步即做，R125-R127 教训沉淀内置）：
  - 定向 race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ => zhidao 2.661s ok / accounts 1.532s ok
  - 支撑定向防线 race（借同工具链）：CGO_ENABLED=1 go test -race -count=1 -run 'TestScheduler|TestWindow|TestAdminStats|TestMigrate|TestPurge|TestRelogin|TestDeleted|TestSubmit|TestProbeIdentity|TestRefused|TestOpenRetain' ./internal/scheduler/ ./internal/db/ => scheduler 14.112s ok / db 2.224s ok
  - 全量 api race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ => 16.836s ok（含登录闸门断言族、状态码族、XFF 透传族）
  - 全量非 race：go test -count=1 ./... => 全包 ok（db/runtime/scheduler/secure/session/store/zhidao；web 无测试文件）

### 5. 新契约角度（本轮自选未覆盖纵深）

**A. 认证全链路登录闸门族纵深（R127 角度换向续走）**
- gateWait :49-64（阻塞版，Relogin 用）+ gateTryAcquire :223-235（非阻塞版，LoginByPassword :244 首行）共享同一 gateMu/gateUsed/gateWindow 计数与 gateLoginPerMin=2；GatePump :66-77（main 后台协程每 30s 广播）与 gateTryAcquire 各自复位窗口——gateTryAcquire 的窗口复位是准入时顺带（time.Since(gateWindow) 大于等于 minute 才重置），GatePump 是定时推进（窗口到点即复位 + 广播），两者对"窗口已过"判读同源（同一 gateMu 互斥），语义自洽：手动登录旧窗口已满时顺带开新窗、自动重登排队由 GatePump 唤醒，绝无双窗口竞态。
- ResetGateForTest :82 测试专用（api 夹具 authenticateDirect 批量注册用），正式代码零调用。
- LoginByPassword 失败清理语义 :253-274：wasShell 快照（Token() 为空串）-> 失败仅摘除本次新建的空壳，绝不误删持有效 token 的工作客户端（黄金期手滑输错密码即全程失联的修复防线成立）。

**B. 基础设施状态码家族核对（B40-01/B39-02 契约延展）**
- writeJSONStatus 定义 :95；调用点全量 11 处：requireAuth 401 :1133 / requireAdminSession 403 :642 / requireJSONBody CSRF-403（router 72,102,116）/ 登录限流 429（router 107）/ 激活限流 429（router 121）/ 配置落库失败 500 :855 / stats 目标数失败 500 :960 / panic recover 500 :1239 / 未知 /api 404（router 198）。**四大家族（鉴权/CSRF/限流/panic）+ 404 全数真实 HTTP 状态码**，与前端 client.ts 只读 body code 契约零冲突（状态码与 body code 同值双通道对齐）。
- httptest 断言族覆盖（panic 后 500 等），状态码族成家族核对无缺口。

**C. 数据库增量迁移规范确认（2026-09-18 定立契约）**
- migrateAddPublishMeta :43-57 在 refuseLegacy :32 **之前**调用（Open 流程 :28 先迁移后拒绝），逐列 columnExists :45 判存在、缺才 ALTER ADD COLUMN 纯增量安全；publish_name/begin_date 两列补全。
- refuseLegacy 缺列清单 :80 只含 targets.priority/allow_swap/task_log.account（**已剔除已迁移的 publish_name/begin_date**，与"缺列清单对应剔除已迁移列"契约对齐）+ account 表 / 空账号目标 / 缺 settings 表 三类数据形状拒绝。
- 迁移 TDD 守护：TestMigrateAddsPublishMetaColumns（db_test.go:34-64，旧库放真实数据行 -> Open 成功 -> 两列补齐 + 旧行保留）；TestOpenAndSchema :9-28 新列存在性双查；旧 shape 拒绝族（TestRefuseOldSchemaMissingColumns/TestRefuseLegacyDB/TestRefuseEmptyAccountTargets）全在位。

**D. 快照 TTL 40s 与轮询关系 + 连接池预热 maybePrewarm 走查**
- snapshotTTL=40s :106（大于探测间隔，保证 /electives 有数据可读）；读侧 ElectivesSnapshotFor :780/:795、ElectivesSnapshot :881、CheckClassSelectable :1871 同源判定；ElectivesSnapshotFor 回退链（"任何账号专属帧过期 => (nil,false) 触发真刷新，绝不回退全局帧"契约-8 锚）在 :786-799 逐行成立，目标账号判据用 len(acctTargets[acct]) > 0 :777（空 slice 不算有目标，防清空目标后专属帧恒过期实时探）。
- maybePrewarm :293-310 走查（R127 观察项候选继续维持）：三行守卫（open 零值 / 开窗前 2 分钟窗口 / 15s 节流）+ AnyClient -> Prewarmer 断言 -> goroutine Prewarm（zhidao.Client.Prewarm :96-109 静默 GET /login + io.Discard）。窗口期外零调用（now.After(open) 返回），行为简单无复杂时序分支，靠 go vet + 全绿编译保障。
- XFF 可信反代（XUANKE_TRUSTED_PROXY 契约）:1210-1223：仅 loopback + on 才信 X-Forwarded-For 最右非空值，默认关闭绝不盲信公网伪造；httptest 表驱动族覆盖 on/off/边界全态，零漂移。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| git 基线 | git diff --stat 08b43da -- backend/ | 空（零产品改动） |
| 核心文件 status | git status --short -- backend/internal/{scheduler,api,accounts,db,store,session}/ | 全部空 |
| 编译 | go build ./... | 通过（BUILD_EXIT=0） |
| 静态检查 | go vet ./... | 通过（VET_EXIT=0） |
| 定向 race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ | zhidao 2.661s ok / accounts 1.532s ok |
| 定向防线 race | CGO_ENABLED=1 go test -race -count=1 -run 定向模式 ./internal/scheduler/ ./internal/db/ | scheduler 14.112s ok / db 2.224s ok |
| 全量 api race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ | 16.836s ok |
| 全量非 race 测试 | go test -count=1 ./... | 全包 ok |
| 契约20 轮次标签扫描 | grep 产品 + 测试 Go 源码 轮次标签全家族 | 零命中（唯 P-500 为 HTTP 状态码非轮次标签） |
| 零吞错穷举 | grep `_ = .*` 及双下划线 落库函数族 | 零命中 |

**gcc 状态说明**：where gcc 默认 PATH 无 gcc；/d/mingw64/bin/gcc.exe 存在 => 借 mingw64 工具链跑 CGO=1 race 全套。若未来机器缺该工具链，CGO_ENABLED=0（无 race）仍可编译 + 全绿非 race 全包验证（R125 已跑过），race 缺口属环境差异非缺陷。

## 三、分级发现

**无新增发现（零 CRITICAL/MAJOR/MINOR）。**

三条 OBSERVE 观察项（与 R127 一致，均非本轮引入、确定性边界已知）：
1. scheduler.go:1068 probeSem cap=4 常驻——大于 50 账号部署时 per-account 探测并发排队可能拉长单账号探测等待。ponytail 注释已标注，维持观察。
2. handler.go:553-557 目标保存双写库路径（handler 层直落 + scheduler 接口二次落库 enrich 发布元数据覆盖），崩溃一致性已确认成立，属防御性冗余，维持观察。
3. scheduler.go:293-310 maybePrewarm 无独立单元测试（与 probeIntervalFor/submitIntervalFor 有测试形成反差）——本轮走查确认行为极简（三行守卫 + 15s 节流 + AnyClient goroutine），靠 go vet/编译 + 全绿保障。候选注释"预热语义应由集成级观测验证，新增进度契约需同步补测试"，维持观察。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线 | R127（08b43da）后 backend/ 零产品改动 |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635 全对位） |
| 写点抽查 5 类（本轮换类） | acctData/openTimeDetected/refused/reloginAt/tokenValid 全持锁 + 复核后写入；无裸写 |
| 类级结构证据（R127 新角度延续） | *Locked 后缀写函数族全数清点成立：写点全部落 *Locked 或持锁函数体 |
| 手动五路 accountExists + maybeRelogin 双侧 | 全数在位（:255/:305/:397/:497-512/:573 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 11 轮确认在位（写回侧当前注册表 ClientFor 后重取 Token() 落库，绝不串旧身份） |
| B110-01 审计链 | 第 18 轮零漂移（手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 五包 race 全绿（zhidao 2.7s/accounts 1.5s/scheduler 14.1s/db 2.2s/api 16.8s，借 /d/mingw64 gcc；缺 gcc 属环境差异） |
| 新契约角度 | 登录闸门族纵深（GatePump/gateTryAcquire 窗口复位同源无竞态）+ 状态码家族核对（鉴权/CSRF/限流/panic/404 全真实码）+ 数据库迁移规范确认 + 快照 TTL/轮询关系 + maybePrewarm 走查，未发现新盲区 |
| 回归锚测试 | TestWindowOpenSubmitsWithoutProbeReset/TestSubmitSuspendedWhenOpenTimeCleared/TestAdminStatsWindowOpenedUsesScheduler 全在位全绿 |
| 契约20 轮次标签 | 产品 + 测试文件双零命中 |

## 五、收尾总结

第四十三轮身份防线矩阵闭合：零产品改动链延续（第六轮纯观察）。7 个 sameClientFor 调用点、本轮换类 5 类写点（acctData/openTimeDetected/refused/reloginAt/tokenValid）、*Locked 后缀写函数族类级结构证据、手动五路 accountExists、maybeRelogin 双侧全数在位且语义自洽。OBSERVE-117-01 第十一轮、B110-01 第十八轮、O105-01 均零漂移与实测绿（借 mingw64 gcc 完成五包 race + 全量非 race）。新角度（登录闸门族纵深/状态码家族核对/数据库迁移规范/快照 TTL 与轮询关系/maybePrewarm 走查）未发现盲区，maybePrewarm 测试覆盖差列为维持观察第三项。进度 129/256。