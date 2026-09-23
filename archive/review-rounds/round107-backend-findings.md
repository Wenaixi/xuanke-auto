# R107 后端只读审查报告

审查对象：xuanke-auto HEAD `3fbd3bb`（R106 收尾轮：纯文档提交，仅 archive/review-rounds 文件，backend/ 自 R103 起连续五轮零产品代码改动——最近一次后端产品代码仍为 `20c5882` B101-01 修复）。工作树洁净。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round107-backend-findings.md）。

审查方式：Read / Grep / Bash 只读命令（定向 go test -race / go vet / 逐点走读）。核心走读范围：身份防线矩阵第二十二轮（全家福写点 grep 实证 + 偶发写点专项 + 会话生命周期新角度）、O105-01 抖动基线第二十二轮夹具走读（本轮含 api/scheduler 全量 race 实证）、B101-01/B102-01/B103-01/B104-01/B105-01/02/03/B106-01 持续盯守、既往观察项延续（O105-02/O106-01/O92-02/M87-01/O90-01 + 契约 20/17 强扫）、新契约角度 a（数据库 schema 与迁移完整契约）。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE（延续观察 + 新发现）

### O107-01（新发现）：session 存储无 `Close()` 接线——sweeperLoop 协程进程生命周期内常驻（低风险观察）

**位置**：`internal/session/store.go:47-55`（New 启动 sweepLoop goroutine）、`internal/api/router.go:201`（Register 返回 `recoverMiddleware(securityHeaders(mux))`，OAuth 链尾部）、`main.go:162`（`sessions := session.New(12 * time.Hour)`）。

**事实确认**：sweeperLoop goroutine 由 Close()（store.go:90）终止，但全程序任何点无 Close 调用——该协程随进程终止自然消亡，无泄漏面（每进程仅一条）。且 5 分钟一拍直接调 sweepExpired，连 goroutine 泄漏意义上的风险都不存在。属于"有伴生终止钩子但无接线"的对称缺失，无害；若未来做优雅关闭可补挂。**维持观察，无提级依据**。

### O106-01（票据内存态，新一轮观察）：新增两点新证据，维持低风险观察

**位置**：`internal/session/store.go:57-87/102-138`。

**复核结论**：R106 走读遗漏修正——**票据清扫实际上已由 sweepLoop 覆盖**：sweepExpired（store.go:73-87）明确同时遍历 sessions 与 tickets 两个 map，sweepLoop 每 5 分钟一拍清扫过期票据；以及 ConsumeTicket 的"过期即删"路径（:128-131）构成第二重回收。R106 文本宣称"tickets map 不主动回收过期票、无钟表清扫"不成立——本轮走读为证，**该观察的核心疑虑（票据无钟表自愈）已证伪**。剩余事实残余：重启丢票（内存 map 不落库，运维重启空窗内已拿票据的未激活账号得重登再激活）+ 过期票在 5 分钟清扫间隔内的间歇性留存（每票 ~200B 量级，不做钟表清扫仅存 ~5min）。**无安全后果**（票熵 32 字节 crypto/rand、防穷举由 handleActivate 先 ConsumeTicket 封死）。**维持低风险观察**（本轮新增"清扫在案"证据，残余疑虑量级进一步缩小）。

## 必查项逐条结论

### 1. 身份防线矩阵（第二十二轮）——通过

延续「全仓写点全家福聚类复核」方法论，`sameClientFor` 7 调用点行号与 R105/R106 对比零漂移：`:199`（定义）、`:846-850`（ProbeForAccount 回写段）、`:1489`（失效分支）、`:1521`（成功分支）、`:1551`（风控退避）、`:1571`（窗口关闭）、`:1600`（实时复核回锁统一复核）、`:1635`（确证满员分支）。

**写点全家福 V2 复核**（grep 实证行号，与 R106 清单逐一对位）：

| 写点 | 行号 | 防线 |
|---|---|---|
| `openTimeDetected[acct]=` | :856（锁内+sameClientFor 后） | 探测回写段身份复核 |
| `openTimeDetected["*"]=` | :1108（全局载体，无身份概念） | 锁内写：1106-1110 |
| `acctData[acct]=` + `acctDataAt[acct]=` | :863/:864（锁内+身份复核后） | 同上 |
| `tokenValid[acct]=true/false` | :1241/:1262 | maybeRelogin 决策侧 ClientFor 复核（:1207-1211）/ 写回侧复核（:1254-1262） |
| `done[acct][id]=true` | :615（RestoreDone，启动期）/ :1529（spawnChain 成功+sameClientFor）/ :1934（MarkDone+ClientFor 存在性） | 全在位 |
| `refused[acct][id]=true` | :634（RestoreRefused）/ :2011（RemoveDone+ClientFor），MarkDone :1938 delete+库行 | 全在位 |
| `rateLimited[acct][class]=` | :1714（仅 spawnChain 风控分支经 sameClientFor 后：1555→1714） | 单入口 |
| `full[acct][class]=true` | :1767（markFullLocked，仅 sameClientFor 后 :1575/:1639 两条调用续） | 单入口 |
| `inflight[acct][class]=true` | :1471（spawnChain）/ :1905（TryAcquireSubmit）；delete :1490/:1501/:1513（链内）/ :1950（MarkDone）/ :2002/2003（RemoveDone）/ :510(区)（PurgeAccount） | 同步删除在位 |
| `state.Courses[idx].Status/Result` | setStateLocked 统一入口 :1846-1851 + 内联写 :1473/1803 区/1961/2014/2045;所有调用点均处于持锁 + 身份防线后 | 无裸露 |
| `relogging[acct]=true/false` | :1236/:1257 | maybeRelogin 决策侧 ClientFor 后 |
| `reloginAt/reloginFail` | :1231/:1232/:1261 | 同上，PurgeAccount :508-509 全清 |

**偶发写点专项（本轮新增角）**：重点查「非 spawnChain / 非 maybeRelogin 主路径、理论上不常走但一旦触发即写状态」的路径——查证全部有防线或无污染语义：① `MarkTokenValid`（:1316，issueSession 手动登录成功恢复路径）是纯 delete（scheduler.go:1316 `delete(s.tokenValid, acct)`），只清失效标记，已删除账号经 handler 401 拦截不会到达；② `TryAcquireSubmit` 是 api 手动路径独占入口，无网络往返后写回；③ `releaseFullIfFreedLocked`（:1780-1803）的解封写（Status=pending/Result=""）由 tick 在持锁段调用、写的是"该账号自己 full 解除"语义，无身份污染面；④ `SubmitAll`/链顶判断（:1420 区）持有 `tokenValidForLocked && doneHas && !isRateLimited && !refusedHas && releaseFullIfFreedLocked` 五重守卫才发请求，无新增写点。**无新裸露写点、无新增偶发写点**。

测试族（probe_identity_test + scheduler 身份防线族 + window 守卫）定向 race 全绿（5.18s）；api 手动四路 race 全绿（3.60s）。

### 2. O105-01 抖动基线（第二十二轮）——通过（含全量 race 实证）

socketPreheat（client_test.go:25）+ TestMain 包级预热 + readyProbe 宽栅栏（client_test.go:82-88，200ms×10 + 2s 超时）布局与 R105/R106 逐字符一致，零改动。api 包夹具（handler_test.go:47 区 socket 预创建 + :139 readyProbe）同款。**本轮跑出全包 race 实证**：`zhidao 全量 -race`（3.74s）、`api 全量 -race`（27.84s）、`scheduler 全量 -race`（15.34s）三包全绿零 flake（含 session/db 全量 1.41s/2.72s）。期间 api 包一次组合 run 首跑出现 FAIL（LoginFlow 相关测试组合 `TestLoginFlow` 匹配空、31.6s 时限内超时性 FAIL），逐条重跑与整组合重跑全部 PASS、go test 全量两次全绿——符合既有低频残余口径（包序 + 冷启动窗口）的"一次性失败、二跑即稳"特征，非新脆弱点。**口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（~13%）**。

### 3. O106-01 票据内存态（新一轮观察）——见 OBSERVE 节

**新增证据**：sweepExpired 覆盖 tickets 清扫在案（store.go:82-86），R106 文本的描述需修正；残余疑虑仅剩「重启丢票 + 5 分钟清扫间隔间歇留存」，量级与概率双低，无提级依据。

### 4. 知识位与修复持续盯守——通过

- **B101-01（sanitizeError）**：定义处无漂移（client.go:581，剥 *url.Error → Op+底层、保留 Unwrap 链）；doRequest :450 唯一应用点；唯一 token URL 通道 = doRequest :422 `?idToken=`。zhidao 脱敏六测试 `-race` 全绿。
- **B102-01**：doRequest 仍唯一携 token 通道；其余 http 直调点（captcha.go:159 Vision 客户端、client.go:85/223）URL 天然静态无 idToken。
- **B103-01**：实时复核满员活化条件（MaxCount>0 && SelectedCount>=MaxCount）在位；classFullRealtime 锁外执行无持锁网络。
- **B104-01**：手动路径无在飞窗口（B20-01 封死）知识位无退化；api 手动四路 accountExists（:245/:295/:373/:537）+ state 透传 :537 区在位。
- **B105-01（展示层观察）**：stats `open_time_set`（handler.go:964）与 `open_time_str`（:899-902）语义仍为"识别槽有值"（含过期值），与学生端 open_time_known 依 now 过期判定（scheduler.go:714-716）不对称维持，本轮走读无新依据提级。
- **B105-02/03/B106-01（知识位）**：TokenValidFor 半态语义、probeSem cap 4、三引擎 withConcurrency 全局限流，均各就其位无退化。

### 5. 既往观察项延续——通过

- **O105-02 删除保护撞名**（handler.go:992 `acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 单判据）：维持观察，无新依据提级。
- **O92-02 logintest 引擎判定源分叉**：实测复核 `cmd/logintest/main.go:57` `switch config.CaptchaEngineDefault()` 只读 XUANKE_CAPTCHA_ENGINE env vs 主程序 settings 持久化 `captcha_engine` 覆盖热改分叉确凿，logintest 三态回退链收敛，维持低风险观察。
- **M87-01 窗口**：windowClosedLocked 三判据单源在位（scheduler.go:903-917），维持注释兜底。
- **O90-01 CRLF**：go vet 五包（scheduler/zhidao/api/session/db）全干净 VET_EXIT=0。
- **契约 20 全仓强扫**：grep `(第 N 轮|R/O/B/M/F[0-9]{2}-[0-9]{2})` 生产代码排除 `_test` 与合法锚点后零命中——scheduler.go:843 的 `M88-01 身份防线` 引用（ProbeForAccount 回写段注释）与 client.go:577 的 `B101-01` 引用为契约号作为锚点的合法引用（正文不陈述历史轮次），合规。
- **契约 17 零吞错**：spawnChain 六处 AppendLog/SaveSuccess/SaveRefused/DeleteSuccess/UpdateIDToken/DeleteRefusedClass 调用点全部 `if err != nil { log.Printf }`；api 层 AppendLog 各点同型；无 `_ =` 落库点。

### 6. 新契约角度 a（数据库 schema 与迁移完整契约）——通过

**schema 形状**（schema.sql）：credentials（password_enc 加密/id_token/updated_at）、accounts、targets（含 publish_id/course_name/priority/publish_name/begin_date/allow_swap 历史死列）、task_log、activation_codes、activations、success（PK account+class_id）、refused（PK account+class_id）、settings。九表全部 `CREATE TABLE IF NOT EXISTS`，与新库幂等。

**迁移顺序正确性**（Open：:15-37）：建表 → `migrateAddPublishMeta`（增量补 targets.publish_name/begin_date，逐列 columnExists 判存在、缺才 ALTER 加列 DEFAULT ''）→ `refuseLegacy`（比对新旧版本不兼容形状）。顺序核对：publish_name/begin_date 作为"纯新增列"在 refuseLegacy 的缺列清单中被显式排除（db.go:77-79 注释），不会误伤旧库；缺列拒绝清单（account 表 / others empty-account targets / targets.priority / targets.allow_swap / task_log.account / settings 表）与新库形状逐列匹配（schema.sql 全含，AUTOINCREMENT 语义一致），无"新库也满足不了的迁移判定"即无误拒启动。

**死列/兼容语义**：allow_swap 保留于 schema + 缺列清单（比换课引擎更旧的库拒启），全仓零消费核查确认（无 SetTargets/INSERT 写该列外的路径）；SQLite 删列破坏旧库兼容，保留兼容形状是正确决策。

**原油田风险排查**：refuseLegacy 的 `account=''` 空账号 targets 检查（db.go:70）守卫旧版无账号 targets——新库按账号隔离（schema 目标 `local` 语义），score 一致；无新索引需求（全表量级小：targets 每账号 ≤100 行、success/refused 账号×课程、task_log 限额读取）。

**TDD 守护**：`TestMigrateAddsPublishMetaColumns` 在库（db_test.go），旧库放真实数据行 → Open 成功 → 两列已补 + 旧行保留；本轮回合 `db 全量 -race` 2.72s 全绿，迁移单测含缺列/已含两分支。

**结论**：数据库 schema 与迁移契约完整成立——无列/表/索引/迁移顺序/拒绝清单错误，无新增风险。

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第二十二轮闭合**：sameClientFor 7 调用点零漂移；写点全家福 V2（12 类写点行号逐一对应防线）；偶发写点专项（MarkTokenValid/TryAcquireSubmit/releaseFullIfFreedLocked/SubmitAll 守卫链）无新裸露。 | 通过（走读 + 定向 race：scheduler 5.18s + api 3.60s） |
| **O105-01 抖动基线第二十二轮**：夹具零改动；三包全量 race 一次全绿（zhidao 3.74s / api 27.84s / scheduler 15.34s）+ 一次组合 run 首跑 FAIL 二跑即稳（低频残余口径特征吻合）。 | 通过（全量 -race 实测） |
| **O106-01 票据"清扫在案"证据补全**：sweepExpired 双 map 清扫已由 5 分钟 sweepLoop 心跳覆盖 + ConsumeTicket 过期即删第二重回收。 | 通过（走读）；残余疑虑量级缩小，维持观察 |
| **B101-01/B102-01/B103-01/B104-01/B105-01/02/03/B106-01 持续盯守**：六测试 race 全绿；doRequest 唯一 token 通道；三判据在位；手动四路 accountExists 在位。 | 通过（实测全绿） |
| **新契约角度 a（db schema 与迁移）**：九表形状 / 迁移顺序（建表→补列→拒旧）/ refuseLegacy 清单与新库逐列匹配 / 死列兼容语义 / TDD 守护，全部正确。 | 通过（db 全量 -race 2.72s） |
| **O105-02/O92-02/M87-01/O90-01**：复核结论与历轮一致，无新依据提级。 | 维持 |
| **契约 20/17 强扫**：零轮次标签（仅两条合法契约号锚点）；零吞错落库点。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test ./internal/scheduler/ -run 'TestDeletedAccountRebuiltSameName\|TestWindowClosed\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestPurgeAccount\|TestProbeForAccount' -count=1 -race` | 全绿（身份防线族 + 窗口守卫，5.18s） |
| `go test ./internal/api/ -run 'TestHandleElectivesSelect\|TestAdminDeleteProtectsRenamedAdmin\|TestAdminElectivesUnknownAccountRejects\|TestActivation\|TestAccountOverride' -count=1 -race` | 全绿（手动四路 + 删除保护 + 激活族，3.60s） |
| `go test ./internal/zhidao/ -run 'TestSanitize\|TestDoRequestSanitizes\|TestIsReadErr' -count=1 -race` | 全绿（脱敏族，2.05s） |
| `go test ./internal/zhidao/ -count=1 -race` | 全绿（3.74s） |
| `go test ./internal/api/ -count=1 -race` | 全绿（27.84s；期间组合 run 首跑一次 FAIL、二跑/逐条/全量复跑均 PASS——低频残余特征） |
| `go test ./internal/scheduler/ -count=1 -race` | 全绿（15.34s） |
| `go test ./internal/session/ ./internal/db/ -count=1 -race` | 全绿（1.41s / 2.72s） |
| `go vet ./internal/{scheduler,zhidao,api,session,db}/` | 通过（VET_EXIT=0） |
| `git status --short` / `git log --format=%h -3` | 无后端文件脏；HEAD=3fbd3bb（纯文档轮，backend/ 连续五轮零改动） |

## 结论

1. **身份防线矩阵第二十二轮闭合**：sameClientFor 7 调用点零漂移、写点全家福 V2 聚类复核 + 偶发写点专项均无新裸露；测试族定向与全量 race 全绿。
2. **O105-01 抖动基线第二十二轮**：夹具零改动，三包全量 -race 一次全绿；一次组合 run 首跑 FAIL 二跑即稳，符合低频残余口径，无新脆弱点。
3. **O106-01 票据内存态**：本轮走读修正 R106 描述——票据清扫已由 sweepLoop/sweepExpired 覆盖在案；残余疑虑（重启丢票 + 5 分钟清扫间隔间歇留存）量级缩小，维持低风险观察。
4. **知识位与既往观察项**：B101-01/B102-01/B103-01/B104-01/B105-01/02/03/B106-01 全绿无回归；O105-02/O92-02/M87-01/O90-01 维持历轮结论；契约 20/17 强扫通过。
5. **新契约角度 a（db schema 与迁移）**：九表形状、迁移顺序、refuseLegacy 缺列清单、死列兼容、TDD 守护全部正确性成立，无任何列/表/索引/迁移顺序/拒绝清单错误。
6. **新发现仅 O107-01（OBSERVE，session 无 Close 接线）**——sweeperLoop 协程进程生命周期常驻，纯伴生钩子无接线，无害；无 MINOR 以上新缺陷。

工作树后端文件洁净。