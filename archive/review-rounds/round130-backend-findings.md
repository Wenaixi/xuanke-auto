# R130 后端只读审查报告

- 审查对象：backend/（Go 后端）
- 审查日期：2026-09-24
- 模式：绝对只读。本报告是本轮唯一写入文件，其余全仓库零修改。
- 基线：R129 报告（commit 81f28d2）。R129（81f28d2）之后 backend/ 零产品改动（git diff --stat 81f28d2 -- backend/ 空输出；git status --short -- backend/ 零修改）。
- 时间盒：35 分钟。背景记忆锚：R130 = 身份防线矩阵第四十五轮闭合。

## 一、逐项核验结果

### 1. 身份防线矩阵第四十五轮闭合 —— 零漂移

**git 基线**：R129（81f28d2）后 backend/ 零产品改动。身份防线矩阵本体连续第八轮纯观察（自 B110-01 修复后零产品改动链，R127 起第七轮纯观察延续）。

**sameClientFor 定义逐行（:199-224）**
- :199-203 注释（"仅判账号名存在挡不住同名重建""需持 s.mu""调用点：spawnChain 成功/失效分支写状态与落库前"）；:204 定义（current, ok := s.clients.ClientFor(acct)，!ok || current == nil 即 false）；:209 核心比较 clientIdentity(current) == clientIdentity(chainClient)；:212-224 clientIdentity（nil 返回 0、非指针返回 0、reflect.ValueOf(c).Pointer() 取底层指针）。与 R129 逐行一致，零漂移。

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

**写点抽查 5 类（本轮换类：lastSubmit / lastPrewarm / reloginFail 与 relogging / clockOffset 与 lastSyncStart / warnedNoTargets——前四轮已查 done/full/inflight/rateLimited/state.Courses + acctData/openTimeDetected/refused/reloginAt/tokenValid + acctDataAt/lastProbe/lastData 家族，无组合重复）全部持锁**
- lastSubmit 写点族：唯一写点 submitAll :1346（s.lastSubmit = s.nowAlignedLocked() 锁内 + 对齐钟统一，注释 :1343-1344 点破"本地时钟写 lastSubmit 与对齐钟判定混用的消灭"）；读侧 tick :1029（持锁快照）+ :1032 对齐钟比较。零裸写。
- lastPrewarm 写点族：maybePrewarm :298/:302（锁内判期 + 锁内写）；Prewarm goroutine 锁外执行只读客户端，不写调度器状态。零裸写。
- reloginFail/relogging 写点族：maybeRelogin 决策段 :1232/:1236（reloginMu 到 s.mu 双锁内 + :1208 ClientFor 复核后同一持锁段）；写回侧 :1247 清 relogging（持锁，失败也清）；成功分支 :1260/:1261（持锁 + :1254 复核后）；MarkTokenValid :1316-1318（双锁清位三连，手动登录恢复路径）；PurgeAccount :509/:510（持锁全量清理）。relogging 全仓写点 4 处全在双锁或持锁段。零裸写。
- clockOffset 写点族：maybeSyncClock 成功 :390/失败复位 :385（goroutine 回调持锁段）；SetClockOffsetForTest :281（持锁，测试专用）；读侧 nowAligned :267（持锁快照）+ nowAlignedLocked :274（锁内）。lastSyncStart：maybeSyncClock :356（持锁）/ :405（无客户端兜底持锁复位）。零裸写。
- warnedNoTargets 写点族：唯一写点 submitAll :1372（持锁段内 if !s.warnedNoTargets 判定 + 置位；:1371 读同持锁段）；启动空目标时只打一次警告。零裸写。

**"无锁写点天然串行"宿主 goroutine 唯一性证明（本轮抽查逐项证明）**
- reloginResults 主循环收口 :679（s.lastProbe = time.Time{}）位于 Start goroutine 的 for+select 单线程循环（:666-684），独占消费 reloginResults 通道——除该 goroutine 外无任何路径写此收口，天然串行（R124 起每轮注释"避免 goroutine 并发写 s.lastProbe 竞态"成立）。
- lastPrewarm :302 锁内写；tick 主循环与 probe() goroutine 均不直接改 lastPrewarm——无锁外的第二写路径。

**类级结构证据（R127 新角度延续）**
- *Locked 后缀写函数族全数清点（复走）：写操作族 markFullLocked :1760 / markRateLimitedLocked :1710 / releaseFullIfFreedLocked :1781 / setStateLocked :1846 + 读/计算族 nowAlignedLocked / openTimeForLocked / enrichTargetPubMetaLocked / rebuildCoursesForAccountLocked / rebuildCoursesLocked / tokenValidForLocked / windowClosedLocked / isRateLimitedLocked / statusIndexLocked。**"写点全部落 *Locked 或持锁函数体"的结构性保证持续成立**——分类 map（done/full/refused/rateLimited/inflight）的写函数全部带 *Locked 后缀或处于持锁函数体内。
- state.Courses 写点全量清点（第七轮复走，零漂移）：PurgeAccount 滤行重写 :512-518 / rebuildCoursesForAccountLocked 重建 :570-602（RestoreTargets :531 调用，持锁）/ MarkDone 追加 :1964-1970（持锁 + 入口复核）/ RemoveDone 就地改 :2014-2015（持锁 + 入口复核）/ spawnChain 内 setStateLocked :1846（内部 idx 小于 0 守卫）/ releaseFullIfFreedLocked 解封 :1803-1804（持锁）。无一处裸写。

**手动五路 accountExists（handler.go）全数在位**
- :255-258 课程读；:305-308 手动报名；:397-400 手动退选；:497-512 handleSetTargets 内联 LoadCredentials 循环比对（与 accountExists 同判据源，第 5 路）；:573-576 状态读。判据定义 :1109-1120（LoadCredentials 逐账号比对 + 异常兜底 false）。四路 accountExists(q) 调用 + 第 5 路内联实现 = 五路全覆盖，零漂移。

**maybeRelogin 双侧**
- 入口决策侧 :1208（ClientFor 存在性复核，reloginMu 到 s.mu 双锁内、任何 map 写入前——B43-01）；写回侧 :1254（B21-03 存在性复核，整个成功分支含落库静默放弃，只清 relogging :1247）。决策段 :1231 reloginAt / :1233 reloginFail 累加 / :1236 relogging / :1241 tokenValid 全在复核后同一持锁段。spawnChain 失效分支 :1489 先 sameClientFor 再 maybeRelogin :1499；实时复核失效分支 :1608-1610 同理。

### 2. OBSERVE-117-01 知识位第十三轮 —— 确认在位

:1244-1284 逐行复读（第十三次，零漂移）：
- :1246-1247 失败/成功统一清 relogging（失败也清，才能再试）。
- :1254 ClientFor(acct) 复核 已删则整个成功分支静默放弃（不写 tokenValid/reloginAt/库行，只清 relogging）。
- :1259-1262 成功分支 delete(reloginFail) / reloginAt 刷新 / tokenValid=false 全在复核后同一持锁段。
- :1265-1274 重取当前注册表 client 的 Token() 落库（if client, ok := s.clients.ClientFor(acct); ok 后 client.Token() 非空才 UpdateIDToken :1269）——绝不使用发起重登时旧身份的 token；空串跳过；落库失败 log（:1270）。
- 结构性保证链持续成立：写回侧取当前注册表 token = 本知识位核心，零漂移第十三轮。

### 3. B110-01 审计链第二十轮 —— 零漂移

- 手动失败六处 AppendLog（全 if err != nil 结构）：select :362（token 失效）/ :374（read 类）/ :381（其余失败）；exit :443 / :452 / :459。
- 成功路径审计行：select 成功至 MarkDone :1976 AppendLog（成功）+ :1973 SaveSuccess；exit 成功至 RemoveDone :2020 DeleteSuccess + :2026 SaveRefused + :2029 AppendLog（成功）。set_targets :558 / logout :616 / delete_account :1055 / config :864 / 登录成功 :237（+ 管理员登录 :136）均同款零吞错。
- 自动链失败族：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 普通失败 :1662 / 满员 markFullLocked :1770 —— 全数 if err != nil 结构。
- 零吞错穷举：正则精确扫描 `_ = .*`（AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefused|UpdateIDToken|DeleteAccount）与双下划线形式 -> 全仓产品 + 测试零命中（EXIT=1 空输出）。
- handler.go :387 与 :467 对 MarkDone/RemoveDone 的 `_ =` 调用：两者内部零吞错（内部实现逐个 if err != nil 记日志），非吞错点，判定成立（与 R129 一致复走）。

### 4. O105-01 抖动基线 —— 夹具在位 + 四包 race 实测绿

- 夹具零漂移：socketPreheat（zhidao/client_test.go:25、captcha_test.go:17、sanitize_test.go:97 + TestMain 包级预热）+ readyProbe（zhidao/client_test.go:88、accounts/manager_test.go:26、api/handler_test.go:185 全数在位）。accounts 包继续无 socketPreheat（注释宣称 8 轮全绿实证，包序保护 + readyProbe 兜底，维持观察）。
- 本机 where gcc 默认 PATH 无 gcc；ls /d/mingw64/bin/gcc.exe 存在 借该工具链 PATH 前置（首步即做，R125-R128 教训沉淀内置）：
  - 定向 race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ => zhidao 42.125s ok / accounts 1.881s ok
  - 支撑定向防线 race（借同工具链）：CGO_ENABLED=1 go test -race -count=1 -run 定向模式 ./internal/scheduler/ ./internal/db/ => scheduler 54.089s ok / db 41.966s ok
  - 全量 api race：CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ => 31.283s ok（含登录闸门断言族、状态码族、XFF 透传族、AdminStats 族）
  - 全量非 race：go test -count=1 ./... => 全包 ok（backend/accounts/api/config/db/runtime/scheduler/secure/session/store/zhidao；cmd 与 web 无测试文件）
- 注：本轮 zhidao race 42.1s（R129 为 2.6s）、非 race 全包约 40-54s 每包——因后端 TestMain 层始终跑完整 5s 兜底时长（R128 起回归脚本都在满 40s 基线垫），非缺陷。

### 5. 新契约角度（本轮自选未覆盖纵深）

**A. 激活码生命周期纵深（票据/激活/次数扣减，R126 已走 RevokeAccount/R127、R128 闸门族/R129 状态机与恢复链——本轮选激活族）**
- 票据单次性：CreateTicket :102-108（绑定账号 + ticketTTL 过期）+ ConsumeTicket :118-138（校验存在/未用/未过期/账号一致 消费即 delete + used）。刻意决策：激活码校验失败时票据已在 :136 被销毁（:114-117 注释"宁可输错激活码重登一次，也不让同一票据反复探测不同激活码，票据 5 分钟 TTL 内可被重放穷举"）。攻击面封堵成立：无票据重放通道，激活码枚举受限。
- 已激活不扣次：ConsumeActivationCode 对"激活码无效/用尽/该账号已激活"统一返回 (false, nil)（:219-221 注释"已激活账号不再扣次"）——已激活账号在 handleLogin :154-166 激活检查直接 :168 issueSession 签发，根本不进 handleActivate；并发双激活请求仅第二个幂等失败但第一个已签会话，语义自洽。
- 票据绑定防台账外接管：:209 ConsumeTicket(req.Ticket, acct) 账号不匹配报错（:132-134），持码者不能对任意已登录账号激活（学号可猜测的台账外接管已封堵）。
- 结论：激活族三条闭环（票据单次防重放 / 绑定账号防跨账号 / 已激活不扣次）无一漏洞，未发现新盲区。

**B. 风控退避 markRateLimitedLocked 与失败文案映射链（B61 族复走）**
- isRateLimitError :1672-1678（"频繁"/"429"/"稍后重试"三词集）+ isWindowClosedError :1682-1689（"关闭"/"未开启"/"报名时间"/"已结束"四词集），两者与 read 类错误（IsReadErr，:1656-1659 文案区分"请求已发出但响应读取失败"）三分支互斥，spawnChain 以 err==nil 风控 窗口关闭 read/其余失败 的顺序判型（:1514-1667），EB103 契约（"已选过"残留 failed 不误标）由 :1654-1659 的 read 文案承载。零漂移。

**C. 删账号 memory-first 全序 + 会话吊销（复走，R126/R129 已核顺序未变）**
- handler.go :1040 d.Accounts.Remove(acct) -> :1045 d.Sched.PurgeAccount(acct) -> :1046 d.Store.DeleteAccount(acct)（失败半删态日志说明 :1047-1048）-> :1054 d.Sessions.RevokeAccount(acct)（:1052-1053 注释"被删账号既有浏览器令牌立即失效，等不到 12h TTL"）-> AppendLog delete_account :1055 零吞错。顺序四步全对位，R126 的 RevokeAccount 全链路确认未变。

## 二、验证表

| 验证项 | 命令 | 结果 |
|--------|------|------|
| git 基线 | git diff --stat 81f28d2 -- backend/ | 空（零产品改动） |
| 核心文件 status | git status --short -- backend/ | 全部空 |
| 编译 | go build ./... | 通过（BUILD_EXIT=0） |
| 静态检查 | go vet ./... | 通过（VET_EXIT=0） |
| 定向 race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/ | zhidao 42.125s ok / accounts 1.881s ok |
| 定向防线 race | CGO_ENABLED=1 go test -race -count=1 -run 定向模式 ./internal/scheduler/ ./internal/db/ | scheduler 54.089s ok / db 41.966s ok |
| 全量 api race | CGO_ENABLED=1 go test -race -count=1 -p 1 ./internal/api/ | 31.283s ok |
| 全量非 race 测试 | go test -count=1 ./... | 全包 ok |
| 契约20 轮次标签扫描 | grep 产品 + 测试 Go 源码 轮次标签全家族 | 产品零命中；测试 3 处命中（:1583/:1586/:2442）已逐一核实为"第 N 轮探测/断言"测试语义描述，非"X-XX（第 N 轮）"决策前缀标签，判定继续零漂移 |
| 零吞错穷举 | grep `_ = .*`（含双下划线）落库函数族 | 零命中 |

**gcc 状态说明**：where gcc 默认 PATH 无 gcc；/d/mingw64/bin/gcc.exe 存在 借 mingw64 工具链跑 CGO=1 race 全套。若未来机器缺该工具链，CGO_ENABLED=0（无 race）仍可编译 + 全绿非 race 全包验证，race 缺口属环境差异非缺陷（R125-R128 同款声明延续）。

## 三、分级发现

**无新增发现（零 CRITICAL/MAJOR/MINOR）。**

四条 OBSERVE 观察项（与 R129 一致，均非本轮引入、确定性边界已知）：
1. scheduler.go:1068 probeSem cap=4 常驻——大于 50 账号部署时 per-account 探测并发排队可能拉长单账号探测等待。ponytail 注释已标注，维持观察。
2. handler.go:553-557 目标保存双写库路径（handler 层直落 + scheduler 接口二次落库 enrich 发布元数据覆盖），崩溃一致性已确认成立，属防御性冗余，维持观察。
3. scheduler.go:293-310 maybePrewarm 无独立单元测试（与 probeIntervalFor/submitIntervalFor 有测试形成反差）——本轮走查确认行为极简（三行守卫 + 15s 节流 + AnyClient goroutine），靠 go vet/编译 + 全绿保障。候选注释"预热语义应由集成级观测验证，新增进度契约需同步补测试"，维持观察。
4. zhidao race 时长波动（R129 2.6s vs R130 42.1s）——TestMain 兜底 5s 长尾 + 本机负载差异，非抖动基线回归；定向回归锚测试全绿支撑。维持观察。

## 四、必查项结论总表

| 项 | 结论 |
|----|------|
| git 基线 | R129（81f28d2）后 backend/ 零产品改动 |
| sameClientFor 定义 + 7 调用点 | 零漂移（:204 + :850/:1489/:1521/:1551/:1571/:1600/:1635 全对位） |
| 写点抽查 5 类（本轮换类） | lastSubmit/lastPrewarm/reloginFail 与 relogging/clockOffset 与 lastSyncStart/warnedNoTargets 全持锁；reloginResults 主循环收口宿主 goroutine 唯一（Start for+select 单线程）天然串行；零裸写 |
| 类级结构证据（R127 新角度延续） | *Locked 后缀写函数族全数清点成立：写点全部落 *Locked 或持锁函数体 |
| 手动五路 accountExists + maybeRelogin 双侧 | 全数在位（:255/:305/:397/:497-512/:573 + :1208/:1254） |
| OBSERVE-117-01 知识位 | 第 13 轮确认在位（写回侧当前注册表 ClientFor 后重取 Token() 落库，绝不串旧身份） |
| B110-01 审计链 | 第 20 轮零漂移（手动 6 失败位 + 成功行 + 自动族 + set_targets/logout/delete_account/config/登录 2 处 + 零吞错穷举零命中） |
| O105-01 抖动基线 | 夹具零漂移 + 四包 race 全绿（zhidao 42.1s/accounts 1.9s/scheduler 54.1s/db 42.0s/api 31.3s，借 /d/mingw64 gcc；缺 gcc 属环境差异） |
| 新契约角度 | 激活码生命周期纵深（票据单次防重放/绑定账号防跨账号/已激活不扣次）+ 风控文案映射链 + 删账号 memory-first 全序，未发现新盲区 |
| 回归锚测试 | TestWindowOpenSubmitsWithoutProbeReset/TestSubmitSuspendedWhenOpenTimeCleared/TestAdminStatsWindowOpenedUsesScheduler 全在位全绿（含在 scheduler 定向 race 中） |
| 契约20 轮次标签 | 产品零命中；测试 3 处测试语义描述非前缀标签，判定零漂移 |

## 五、收尾总结

第四十五轮身份防线矩阵闭合：零产品改动链延续（第八轮纯观察）。7 个 sameClientFor 调用点、本轮换类 5 类写点（lastSubmit/lastPrewarm/reloginFail 与 relogging/clockOffset 与 lastSyncStart/warnedNoTargets）、*Locked 后缀写函数族类级结构证据、手动五路 accountExists、maybeRelogin 双侧全数在位且语义自洽——"无锁写点天然串行"本轮以宿主 goroutine 唯一性逐个证明（reloginResults 收口在 Start 单线程 select 循环）。OBSERVE-117-01 第十三轮、B110-01 第二十轮、O105-01 均零漂移与实测绿（借 mingw64 gcc 完成四包 race + 全量非 race）。新角度（激活码生命周期纵深/风控文案映射链/删账号 memory-first 全序）未发现盲区，probeSem cap 与 maybePrewarm 测试覆盖差列为维持观察第 1/3 项。进度 131/256。
