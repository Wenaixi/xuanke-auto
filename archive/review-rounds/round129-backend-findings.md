# R129 后端只读审查报告

- 审查对象：backend/（Go 后端）
- 审查日期：2026-09-24
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R128 报告（commit 53a163f）。R128（53a163f）之后 backend/ 零产品改动（git diff --stat 53a163f -- backend/ 空输出；git status --short -- backend/ 零修改）。
- 时间盒：35 分钟。背景记忆锚：R129 = 身份防线矩阵第四十四轮闭合。

## 一、逐项核验结果

### 1. 身份防线矩阵第四十四轮闭合 —— 零漂移

**git 基线**：R128（53a163f）后 backend/ 零产品改动。身份防线矩阵本体连续第七轮纯观察（R127 起零产品改动链延续）。

**sameClientFor 定义逐行（:199-224）**
- :199-203 注释（"仅判账号名存在挡不住同名重建""需持 s.mu""调用点：spawnChain 成功/失效分支写状态与落库前"）；:204 定义（`current, ok := s.clients.ClientFor(acct)`，`!ok || current == nil` 即 false）；:209 核心比较 `clientIdentity(current) == clientIdentity(chainClient)`；:212-224 clientIdentity（nil 返回 0、非指针返回 0、reflect.ValueOf(c).Pointer() 取底层指针）。与 R128 逐行一致，零漂移。

**7 个调用点精确行号（grep 实测，与 top 锚点一一对应 ZERO 漂移）**
| 锚点 | 实测行号 | 语义 |
|------|---------|------|
| :850 | :850 | ProbeForAccount 回写段（M88-01 防线；复核失败整体放弃回写，绝不动 acctData/openTimeDetected，:852 日志点破） |
| :1489 | :1489 | spawnChain 失效分支（先复核再 maybeRelogin :1499，:1484-1493 注释完整） |
| :1521 | :1521 | spawnChain 成功分支（done :1529 + setStateLocked + AppendLog :1532 + SaveSuccess :1535 全在复核通过后同一持锁段） |
| :1551 | :1551 | spawnChain 风控退避分支（markRateLimitedLocked :1555） |
| :1571 | :1571 | spawnChain 窗口关闭分支（markFullLocked :1575） |
| :1600 | :1600 | spawnChain 实时复核回锁后统一复核（六分支统一点，B43-02 第六分支） |
| :1635 | :1635 | spawnChain 确证满员分支（doneHas 守卫 :1627 至 markFullLocked :1639） |

**写点抽查 5 类（本轮换类：lastProbe / lastData 与 lastDataAt / probing / syncing 与 lastSyncTime / syncFailStreak——与前几轮已查 done/full/inflight/rateLimited/state.Courses + acctData/openTimeDetected/refused/reloginAt/tokenValid 无组合重复）全部持锁**
- lastProbe 写点族：ProbeNow :964（持锁）；probe() 主帧 :1089（失败同计）/ :1114（持锁）；reloginResults 主循环 :679（持锁）。**唯一不含锁的 lastProbe 写点在主循环 select 内**（:675-681，位于 Start goroutine 单线程循环，天然串行无并发竞争，注释"避免 goroutine 并发写 s.lastProbe 竞态"完整成立）。读侧 tick :976 持锁取快照。零裸写。
- lastData/lastDataAt 写点族：ProbeNow :965/:966（持锁）；probe() :1115/:1116（持锁）。读侧 ElectivesSnapshot :881、ElectivesSnapshotFor 回退链 :801、releaseFullIfFreedLocked :1787 全部持锁或锁内。零裸写。
- probing 写点族：probe() :1050（持锁置位）/ :1081/:1088/:1113（持锁复位，所有退出路径全覆盖：无账号/失败/成功三出口均复位）。零裸写。
- syncing 写点族：maybeSyncClock :355（持锁置位）/ :369（goroutine 回调持锁复位）/ :404（无客户端兜底持锁复位——三出口全闭合）。lastSyncTime 只成功推进 :393 + :331 持锁判读；syncFailStreak 只在持锁段 ++ :371 / 清零 :391。零裸写。
- MarkTokenValid :1306-1318（reloginMu 到 s.mu 双锁同步清除）+ TryAcquireSubmit :1896-1914（sync.Once 幂等释放族）复走，行为符合既有契约。

**类级结构证据（R127 新角度延续）**
- full 唯一写函数 markFullLocked :1760 + rateLimited 唯一写函数 markRateLimitedLocked :1710（均 *Locked 后缀 = 持锁专用）；*Locked 后缀写函数族全数清点：写操作族（markFullLocked / markRateLimitedLocked / releaseFullIfFreedLocked / setStateLocked）+ 读/计算族（nowAlignedLocked / openTimeForLocked / enrichTargetPubMetaLocked / rebuildCoursesForAccountLocked / rebuildCoursesLocked / tokenValidForLocked / windowClosedLocked / isRateLimitedLocked / statusIndexLocked）。**"写点全部落 *Locked 或持锁函数体"的结构性保证持续成立。**
- state.Courses 写点全量清点（第六轮复走，零漂移）：PurgeAccount 滤行重写 :512-518 / rebuildCoursesForAccountLocked 重建 :570-602（RestoreTargets :531 调用，持锁）/ MarkDone 追加 :1964-1970（持锁 + 入口复核）/ RemoveDone 就地改 :2014-2015（持锁 + 入口复核）/ spawnChain 内 setStateLocked :1846（内部 idx<0 守卫）/ releaseFullIfFreedLocked 解封 :1803-1804（持锁）。无一处裸写。

**手动五路 accountExists（handler.go）全数在位**
- :255-258 课程读；:305-308 手动报名；:397-400 手动退选；:497-512 handleSetTargets 内联 LoadCredentials 循环比对（与 accountExists 同判据源，第 5 路）；:573-576 状态读。判据定义 :1109-1120（LoadCredentials 逐账号比对 + 异常兜底 false）。四路 accountExists(q) 调用 + 第 5 路内联实现 = 五路全覆盖，零漂移。

**maybeRelogin 双侧**
- 入口决策侧 :1208（ClientFor 存在性复核，reloginMu 到 s.mu 双锁内、任何 map 写入前——B43-01）；写回侧 :1254（B21-03 存在性复核，整个成功分支含落库静默放弃，只清 relogging :1247）。决策段 :1231 reloginAt / :1233 reloginFail 累加 / :1236 relogging / :1241 tokenValid 全在复核后同一持锁段。spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin :1499；实时复核失效分支 :1608-1610 同理。

### 2. OBSERVE-117-01 知识位第十二轮 —— 确认在位

:1244-1284 逐行复读：
- :1246-1247 失败/成功统一清 relogging（失败也清，才能再试）。
- :1254 ClientFor(acct) 复核 => 已删则整个成功分支静默放弃（不写 tokenValid/reloginAt/库行，只清 relogging）。
- :1259-1262 成功分支 delete(reloginFail) / reloginAt 刷新 / tokenValid=false 全在复核后同一持锁段。
- :1265-1274 **重取当前注册表 client 的 Token() 落库**（if client, ok := s.clients.ClientFor(acct); ok 后 client.Token() 非空才 UpdateIDToken :1269）——绝不使用发起重登时旧身份的 token；空串跳过；落库失败 log（:1270）。
- 结构性保证链持续成立：写回侧取当前注册表 token = 本知识位核心，零漂移第十二轮。

### 3. B110-01 审计链第十九轮 —— 零漂移

- 手动失败六处 AppendLog（全 if err != nil 结构）：select :362（token 失效）/ :374（read 类）/ :381（其余失败）；exit :443 / :452 / :459。
- 成功路径审计行：select 成功至 MarkDone :1976 AppendLog（成功）+ :1973 SaveSuccess；exit 成功至 RemoveDone :2020 DeleteSuccess + :2026 SaveRefused + :2029 AppendLog（成功）。set_targets :558 / logout :616 / delete_account :1055 / config :864 / 登录成功 :237 均同款零吞错。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员 markFullLocked :1770 —— 全数 if err != nil 结构。
- 零吞错穷举：正则精确扫描 `_ = .*(AppendLog|Save*|Delete*|UpdateIDToken)` 与双下划线形式 -> 全仓产品+测试零命中（EXIT=1 空输出）。
- handler.go :387 与 :467 对 MarkDone/RemoveDone 的 `_ =` 调用：两者内部零吞错（内部实现逐个 if err != nil 记日志），非吞错点，判定成立。

### 4. O105-01 抖动基线 —— 夹具在位 + 四包 race 实测绿

- 夹具零漂移：socketPreheat（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97 + TestMain :39-42 包级预热）+ readyProbe（zhidao/client_test.go:88、accounts/manager_test.go:26、api/handler_test.go:179 全数在位）。accounts 包继续无 socketPreheat（注释宣称 8 轮全绿实证，包序保护 + readyProbe 兜底，维持观察）。
- 本机 where gcc 默认 PATH 无 gcc；ls /d/mingw64/bin/gcc.exe 存在 => 借该工具链 PATH 前置（首步即做，R125-R128 教训沉淀内置）：
  - 定向 race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ => zhidao 2.624s ok / accounts 1.321s ok
  - 支撑定向防线 race（借同工具链）：CGO_ENABLED=1 go test -race -count=1 -run 'TestScheduler|TestWindow|TestAdminStats|TestMigrate|TestPurge|TestRelogin|TestDeleted|TestSubmit|TestProbeIdentity|TestRefused|TestOpenRetain' ./internal/scheduler/ ./internal/db/ => scheduler 14.238s ok / db 2.100s ok
  - 全量 api race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ => 17.276s ok（含登录闸门断言族、状态码族、XFF 透传族、AdminStats 族）
  - 全量非 race：go test -count=1 ./... => 全包 ok（backend/accounts/api/config/db/runtime/scheduler/secure/session/store/zhidao；cmd 与 web 无测试文件）

### 5. 新契约角度（本轮自选未覆盖纵深）

**A. tick 主循环状态机纵深（tick/probe/windowClosedLocked 三判据单源）**
- tick :973-1039 状态机：探测闸门（lastProbe 单快照 + probeIntervalForOpen :987，热改亚毫秒窗口内单次 open 复用）→ 开窗瞬间强制探测（:989-991 `now.After(open) && last.Before(open-1s)` 放行）→ 提交触发三判据（:1011 零值守卫 `open.IsZero() && !opened` 挂起；:1014 `!opened && !now.After(open)` 未到点挂起；:1024 WindowClosed 挂起）→ 提交节流（:1032 submitIntervalFor 对齐钟）。**B41-02 零值守卫让位于 WindowOpened** 的语义在 :1007-1013 注释 + 实现逐行成立（WindowOpened=true 时零值守卫绝不挂起，黄金期 250ms 冲刺不受识别槽缺席影响）。
- windowClosedLocked :918-939 三判据单源：主判据 state.WindowClosed（+10s 裕量在 probe 主判据 :1146 入账）/ 时钟连续失败 ≥3 且开放时间已过（:925）/ 幽灵窗口 EmptyProbeRuns≥3（:935，open 单快照复用）。StateForAccount :703 与 WindowClosed() :907 共用同一实现，绝无双真相分叉。
- reloginResults 主循环收口 :675-681（成功回传后 lastProbe 置零补探测，主循环单线程无竞态），通道非阻塞发送 :1278-1281（select default 弃信号不持锁阻塞）——重登补探测与探测节流自洽。

**B. 快照 TTL 40s + 轮询关系（契约 8 回退链复走）**
- snapshotTTL=40s :106（大于探测间隔，保证 /electives 有数据可读）；ElectivesSnapshotFor :761-810 回退链逐行成立：**任何账号专属帧"存在但过期"→ 返回 (nil,false) 触发真刷新，绝不回退全局 lastData**（:786-799）；目标账号判据 len(acctTargets[acct])>0 :777（空 slice 不算有目标）；无目标纯浏览且从未有专属帧才允许回退全局帧。CheckClassSelectable :1863-1892 同源判定（快照过期放行交平台）。
- 轮询关系：findElectivesStudentCount 前端 10s 轮询（前端契约）与调度器探测节流（30s 常态/2s 临门）两条独立链路不互相污染；lastProbe 只归 probe()/ProbeNow（:868-872 注释"管理员穿透探测不写 lastProbe"逐行成立）。

**C. 恢复链 RestoreDone→RestoreTargets→RestoreRefused 全序（main.go）**
- main.go :117-141：RestoreDone(success) :120 → 逐账号 LoadTargetsForAccount + RestoreTargets（不清 refused，:129-132）→ LoadRefused + RestoreRefused :137-140（**必须最后注入**，:135-136 注释"RestoreTargets 不清库行，此处 LoadRefused 仍能读到全部退选；RestoreRefused 注入后不被任何后续步骤覆盖"）。与 scheduler.go :621-625 的 RestoreRefused 顺序修正注释自洽。
- RestoreTargets :525-533 不清 refused（与 SetTargetsForAccount :454 唯一区别），SetTargetsForAccount 只在用户主动重设目标时调用且只清 refused、绝不清 done/full/rateLimited/inflight（R128 已核，本轮复走 :454-493）。refused 优先于 done（spawnChain :1447-1450 拒绝守卫在 done 判定 :1436 之后——手动退选优先于任何自动恢复）。

**D. SubmitAll 锁外 spawn 与 lastSubmit 对齐钟 + 删账号 memory-first 全序**
- submitAll :1342-1380：锁内收集链 + 统一 nowAlignedLocked 写 lastSubmit（:1346，锁内一次性写，锁外 spawnChain :1377-1379）；锁外 spawn 受 chainMu/chains 去重守卫（:1392-1398）+ spawnChain 链顶 ClientFor 复核（:1388 + goroutine 内二次取 client :1405-1411）双闸。账号被删（ClientFor 不存在）链级跳过 :1355。零锁外裸写。
- 删账号 memory-first 全序（契约 4）：handler.go :1040 `d.Accounts.Remove(acct)` → :1045 `d.Sched.PurgeAccount(acct)` → :1046 `d.Store.DeleteAccount(acct)` → :1054 `d.Sessions.RevokeAccount(acct)`（R126 已走 RevokeAccount 全链路，本轮确认顺序未变）→ AppendLog delete_account :1055 零吞错。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| git 基线 | git diff --stat 53a163f -- backend/ | 空（零产品改动） |
| 核心文件 status | git status --short -- backend/ | 全部空 |
| 编译 | go build ./... | 通过（BUILD_EXIT=0） |
| 静态检查 | go vet ./... | 通过（VET_EXIT=0） |
| 定向 race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ | zhidao 2.624s ok / accounts 1.321s ok |
| 定向防线 race | CGO_ENABLED=1 go test -race -count=1 -run 定向模式 ./internal/scheduler/ ./internal/db/ | scheduler 14.238s ok / db 2.100s ok |
| 全量 api race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ | 17.276s ok |
| 全量非 race 测试 | go test -count=1 ./... | 全包 ok |
| 契约20 轮次标签扫描 | grep 产品 + 测试 Go 源码 轮次标签全家族 | 零命中（唯 session/store.go:117 为文档路径引用非轮次标签） |
| 零吞错穷举 | grep `_ = .*` 及双下划线 落库函数族 | 零命中 |

**gcc 状态说明**：where gcc 默认 PATH 无 gcc；/d/mingw64/bin/gcc.exe 存在 => 借 mingw64 工具链跑 CGO=1 race 全套。若未来机器缺该工具链，CGO_ENABLED=0（无 race）仍可编译 + 全绿非 race 全包验证（R125 已跑过），race 缺口属环境差异非缺陷。

## 三、分级发现

**无新增发现（零 CRITICAL/MAJOR/MINOR）。**

三条 OBSERVE 观察项（与 R128 一致，均非本轮引入、确定性边界已知）：
1. scheduler.go:1068 probeSem cap=4 常驻——大于 50 账号部署时 per-account 探测并发排队可能拉长单账号探测等待。ponytail 注释已标注，维持观察。
2. handler.go:553-557 目标保存双写库路径（handler 层直落 + scheduler 接口二次落库 enrich 发布元数据覆盖），崩溃一致性已确认成立，属防御性冗余，维持观察。
3. scheduler.go:293-310 maybePrewarm 无独立单元测试（与 probeIntervalFor/submitIntervalFor 有测试形成反差）——本轮走查确认行为极简（三行守卫 + 15s 节流 + AnyClient goroutine），靠 go vet/编译 + 全绿保障。候选注释"预热语义应由集成级观测验证，新增进度契约需同步补测试"，维持观察。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线 | R128（53a163f）后 backend/ 零产品改动 |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635 全对位） |
| 写点抽查 5 类（本轮换类） | lastProbe/lastData 与 lastDataAt/probing/syncing 与 lastSyncTime/syncFailStreak 全持锁；reloginResults 主循环收口单线程天然串行；无裸写 |
| 类级结构证据（R127 新角度延续） | *Locked 后缀写函数族全数清点成立：写点全部落 *Locked 或持锁函数体 |
| 手动五路 accountExists + maybeRelogin 双侧 | 全数在位（:255/:305/:397/:497-512/:573 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 12 轮确认在位（写回侧当前注册表 ClientFor 后重取 Token() 落库，绝不串旧身份） |
| B110-01 审计链 | 第 19 轮零漂移（手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account/config/登录 + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 四包 race 全绿（zhidao 2.6s/accounts 1.3s/scheduler 14.2s/db 2.1s/api 17.3s，借 /d/mingw64 gcc；缺 gcc 属环境差异） |
| 新契约角度 | tick 主循环状态机纵深 + 快照 TTL/轮询关系 + 恢复链全序 + SubmitAll 锁外 spawn/lastSubmit 对齐钟 + 删账号 memory-first 全序，未发现新盲区 |
| 回归锚测试 | TestWindowOpenSubmitsWithoutProbeReset/TestSubmitSuspendedWhenOpenTimeCleared/TestAdminStatsWindowOpenedUsesScheduler 全在位全绿 |
| 契约20 轮次标签 | 产品 + 测试文件双零命中 |

## 五、收尾总结

第四十四轮身份防线矩阵闭合：零产品改动链延续（第七轮纯观察）。7 个 sameClientFor 调用点、本轮换类 5 类写点（lastProbe/lastData 与 lastDataAt/probing/syncing 与 lastSyncTime/syncFailStreak）、*Locked 后缀写函数族类级结构证据、手动五路 accountExists、maybeRelogin 双侧全数在位且语义自洽。OBSERVE-117-01 第十二轮、B110-01 第十九轮、O105-01 均零漂移与实测绿（借 mingw64 gcc 完成四包 race + 全量非 race）。新角度（tick 主循环状态机纵深/快照 TTL 与轮询关系/恢复链全序/SubmitAll 锁外 spawn 与 lastSubmit 对齐钟/删账号 memory-first 全序）未发现盲区，maybePrewarm 测试覆盖差列为维持观察第三项。进度 130/256。
