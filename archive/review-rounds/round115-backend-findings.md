# R115 后端只读审查报告（身份防线矩阵第三十轮里程碑）

## 审查对象与方法

- 审查对象 HEAD：`4c32a81`（docs(review): R114 双 findings……进度 115/256）
- 审查范围：`backend/`（scheduler / api / zhidao / accounts / store / db / session / config）
- 审查方式：`git log 20c5882..HEAD -- backend/` 实证改动清单；`grep`/`Read` 逐点核位（sameClientFor 调用点/写点全家福/maybeRelogin 双侧/手动四路）；定向 `go test -race` 红绿实证；`go build`/`go vet ./...` 静态检查。全程零仓库文件改动。
- 结论前置：**无 CRITICAL / MAJOR / MINOR，无新立 OBSERVE。** 身份防线矩阵第三十轮闭合，零产品改动链延续第十轮（COUNT=1），OBSERVE-111-01/112-03 盯守第四轮零漂移，B110-01 审计链第五轮零漂移。

## 分级发现

- **CRITICAL/MAJOR/MINOR：无。** 稳定期无新证据够格立条，遵循宁缺毋滥。
- **OBSERVE（延续，非新立）：**
  - `OBSERVE-111-01` 延续（第四轮盯守）：按主控要求持续盯守 handler.go `_ = d.Sched.MarkDone/RemoveDone` 返回值丢弃。第四轮确认：恒 nil 非吞错——MarkDone/RemoveDone 内部的落库点（SaveSuccess/DeleteSuccess/SaveRefused/AppendLog/DeleteRefusedClass）全部 `if err != nil { log.Printf }` 零吞错；返回值丢弃仅代表"调度器内存态同步失败不阻断前端响应"，与落库日志化不冲突。维持盯守，零漂移。
  - `OBSERVE-112-03` 延续（第四轮盯守）：手动失效分支与自动链同文案双日志动作维度可区分——手动 `select`（handler.go:352）/ `exit`（handler.go:433）分别落 `select`/`exit` action，自动链恒 `select`。动作维度与文案维度双可辨。维持盯守，零漂移。

## 必查项逐条结论

### 1. 身份防线矩阵第三十轮闭合（里程碑）——通过

**改动 COUNT 实证**：`git log --oneline 20c5882..HEAD -- backend/` 精确返回 1 条 = `57bf401`（B110-01 手动报名/退选失败分支补库内审计日志）。B101-01 起零产品改动链延续第十轮成立（57bf401 为审计可观测性修复，非产品逻辑改动，`git show 57bf401 --stat` 实证仅 handler.go +26 行 / handler_test.go +133 行，test 先红后绿）。

**sameClientFor 7 调用点逐点核位（零漂移）**：
| 调用点 | 实际行号 | 语义 |
|---|---|---|
| 定义 | :204 | 指针身份比对（clientIdentity 反射 Pointer） |
| ProbeForAccount 回写段 | :850 | 探测网络往返后写 acctData/openTimeDetected 前复核 |
| 失效分支 | :1489 | ErrUnauthorized 写状态前复核 |
| 成功分支 | :1521 | 写 done/状态/库行前复核 |
| 风控退避 | :1551 | markRateLimited 前复核 |
| 窗口关闭 | :1571 | isWindowClosedError → markFullLocked 前复核 |
| 实时复核统一 | :1600 | classFullRealtime 网络段回锁后三路分支前复核 |
| 确证满员 | :1635 | 实时复核确证满员 markFullLocked 前复核 |

与主控约定行号完全吻合，零漂移。`clientIdentity` 定义 :215 与 sameClientFor 同族。

**写点全家福 12 类逐一对应防线（均持锁 + 身份/存在性防线在旁）**：
- `openTimeDetected`：写点 :856（ProbeForAccount，:850 sameClientFor 前置）/ :1108（probe 全局 `"*"`，AnyClient 探测载体天然全局）；删 :506（PurgeAccount）
- `acctData/acctDataAt`：写 :863/:864（:850 前置）；删 :504/:505
- `tokenValid`：写 :1241（maybeRelogin 决策侧，:1208 ClientFor 入口复核前置）/ :1262（重登成功写回侧，:1254 复核前置）；清 MarkTokenValid :1316；删 :507
- `reloginAt/reloginFail/relogging`：写 :1231/:1233/:1236（决策侧）/ :1247/:1260/:1261（写回侧）；清 MarkTokenValid :1317/:1318；删 :508/:509/:510
- `inflight`：写 :1471 + 稳定清位 :1490/:1501/:1513（网络往返后统一清位，含身份复核失败分支）；TryAcquireSubmit :1905（持 lock）；MarkDone/RemoveDone 同步清 :1950/:2002；删 :502
- `done`：写 :1529（:1521 复核前置）/ MarkDone :1934（:1927 存在性复核）/ RestoreDone :607；删 RemoveDone :2000 / PurgeAccount :499
- `refused`：写 MarkDone :1938 解 / RemoveDone :2011 置；SetTargetsForAccount :458 清；PurgeAccount :503
- `full`：写 markFullLocked :1767（:1551/:1571/:1635 复核前置）；解 releaseFullIfFreedLocked :1800 / MarkDone :1953 / RemoveDone :2005 / RemoveFull :2042；删 :500
- `rateLimited`：写 markRateLimitedLocked :1714（:1551 前置）；删 isRateLimitedLocked :1703 过期自删 / MarkDone :1956 / PurgeAccount :501
- `Courses` 状态：setStateLocked/rebuildCoursesLocked/rebuildCoursesForAccountLocked 全部持 s.mu 映射，无裸露写点

**偶发写点专项**：
- `MarkTokenValid`（:1306-1319）：与 maybeRelogin 对齐 reloginMu→mu 锁序；只 delete 三个标记 key，无网络往返后写身份语义，仅 issueSession 成功路径调用；账号已删时 delete 空 key 无副作用——存在性复核不是该写点的防线需求，维护判断正确。
- `TryAcquireSubmit`（:1896-1914）：持锁置/清 inflight，安全。
- `releaseFullIfFreedLocked`（:1781-1811）：纯快照驱动解封，空快照/未知保守不解，安全。
- `submitAll`（:1342-1380）：收集链时 ClientFor 过滤 + spawnChain 链顶二次复核（:1388）双防线。

**手动四路 accountExists**：:245（handleElectives）/ :295（handleElectiveSelect）/ :387（handleElectiveExit）/ :563（handleState）——与主控描述完全吻合。

**maybeRelogin 双侧**：决策侧 :1208（锁内 ClientFor 存在性复核，任何 map 写入前）；写回侧 :1254（重登完成后复核）+ :1265 成功分支再次 ClientFor 取 token（防御冗余，非缺陷）。

**里程碑方法论复盘（盲区评估）**：矩阵方法论历轮演进「逐点查调用点 → 写点全家福 grep → 偶发写点专项 → 核位+实证不漂移」本轮全部执行。经复核确认：判据侧（`sameClientFor` 七分支 + `clientIdentity`）、写侧（12 类全家福）、读侧（ElectivesSnapshotFor/StateForAccount/TokenValidFor）三角形闭合，未发现新盲区。本轮新增的一个已闭合审视角度：`reloginResults` 通道非阻塞发送（:1279 default 丢弃）与 `syncing` 复位（:404）均为有注释的刻意设计决策，非裸露写点。矩阵已连续三十轮同态，建议下轮起将「写点全家福 grep」从全量降为抽样核查以控制审查成本，但 sameClientFor 七调用点核位保留全量（核心防线）。

### 2. OBSERVE-111-01/112-03 盯守（第四轮）——维持，零漂移

handler.go:377 `_ = d.Sched.MarkDone(...)` / :457 `_ = d.Sched.RemoveDone(...)` 原样在位。恒 nil 语义核验：调度器侧 MarkDone 内 SaveSuccess/AppendLog/DeleteRefusedClass 全日志化（:1944/:1973/:1976），RemoveDone 内 DeleteSuccess/SaveRefused/AppendLog 全日志化（:2020/:2026/:2029）；返回值丢弃只丢"内存态同步失败"，尾部 log.Printf 兜底。手动失效分支双日志 :352（select）/ :433（exit）动作维度可区分。

### 3. B110-01 审计链持续盯守（第五轮）——维持，零漂移

手动失败三分支六处 AppendLog 全在位：select :352（ErrUnauthorized）/ :364（IsReadErr）/ :371（业务失败）；exit :433（ErrUnauthorized）/ :442（IsReadErr）/ :449（业务失败）。六处 err 全部 `if err != nil { log.Printf }` 零吞错。成功路径审计行未回归：MarkDone 内 select/true（:1976）+ RemoveDone 内 exit/true（:2029）。专属 TDD 测试 TestManualElectiveFailureAppendsLog / TestManualElectiveReadErrAppendsLog 定向 race 跑绿。

### 4. B110-02 观察复核——通过（本地钟同基自洽）

`reloginAt` 写（:1231 `time.Now()` 决策侧 / :1261 写回侧）与读（:1219/:1227 `time.Since(t)`）均以本地钟为基准，无"写对齐钟/读本地钟"混基。该字段语义是"退避窗口计时"，本地钟自洽正确（对齐钟只用于开窗点判定/探测节流，与 relogin 退避无耦合）。

### 5. O105-01 抖动基线——零漂移

handler_test.go:53-71 socketPreheat（`net.Listen("tcp","127.0.0.1:0")` + 立即 Close）逐字符在位；:139 `readyProbe(zhi.URL)` 调用 + :185-216 定义（`probeRetries=10` × `probeDelay=200ms`、独立 2s 超时 client）逐字符一致。定向 race 实测：scheduler 3.559s / api 2.995s / zhidao 3.697s / accounts 2.174s / store 33.634s / db 2.076s 全绿，待命夹具未再现连接层抖动。

### 6. 新契约角度（本轮自选：错误文案端到端可辨性 + 基础设施状态码家族核对）——通过

对四族 AppendLog 的 action/文案维度全量对照：自动链族（select：失效 :1504 / 成功 :1532 / 风控 :1558 / 实时复核失效 :1614 / 失败 :1662 / 满员 :1770）、手动族（select :352/:364/:371 + exit :433/:442/:449 + 成功 :1976/:2029）、会话族（login :126/:227 + logout :606）、管理族（set_targets :548 / config :854 / delete_account :1045）。动作维度 6 类可区分（login/logout/set_targets/config/delete_account/select/exit），select vs exit 双维度可辨。手动/自动同 action=select 时靠 result 文案（含账号与平台 msg）区分，审计可追踪，不构成新发现。基础设施状态码家族（router.go）逐一核对：panic 500（:1230）/ 401 requireAuth（:1123）/ 403 requireAdminSession（:632）+ requireJSONBody（:72）/ 429 登录限流（:107）+激活限流（:121）/ 404 /api/ 兜底（:198）全家族 writeJSONStatus 齐备，B40-01 要求（逐家族核对）达成。

## 验证表

| 命令 | 结果 |
|---|---|
| `git log --oneline 20c5882..HEAD -- backend/` | COUNT=1（57bf401） |
| `git show 57bf401 --stat` | handler.go +26 / handler_test.go +133，TDD 声明 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（全包） |
| `go test -race -count=1 ./internal/scheduler ./internal/api`（定向：身份防线 6 测试 + 审计日志 2 测试 + 窗口语义） | ok（scheduler 3.559s / api 2.995s） |
| `go test -race -count=1 ./internal/zhidao ./internal/accounts ./internal/store` | ok（3.697s / 2.174s / 33.634s） |
| `go test -race -count=1 ./internal/db` | ok（2.076s，迁移契约测试绿） |

## 已核无缺陷清单

- scheduler tick 状态机：零值守卫让位 WindowOpened（B41-02，:1011）+ WindowClosed 挂起提交（:1024）+ 黄金期 submitIntervalFor（:284-290）
- 开放时间识别槽三硬契约（:709 过期不截断 / :838 关闭≠删除 / :842 持锁写）
- windowClosedLocked 三判据单源（:918-939，open 单快照复用 +10s 裕量）
- doRequest 契约（:414-468）：Cookie 双通道 / ErrUnauthorized code=-1 判定 / sanitizeError（:450，B101-01 统一剥 URL）
- httpDo 活性自愈（:478-492）只重试 dial/write，read 不重试（防双报）；IsReadErr 形态矩阵（:523-552）与 isreaderr_test.go 对照
- store 迁移契约（db.go:43-57 增量迁移 / :60-98 refuseLegacy 缺列清单剔除已迁移列）与 db_test.go 三个迁移测试全绿
- PurgeAccount 全量清理 12 族（:495-519）与 handleAdminDeleteAccount memory-first 四步序（:1030-1044）
- 查阅不存在账户整体拒绝四路 + 状态读共用 windowClosedLocked 单源（StateForAccount :703）
- login 双条件管理员判定（handler.go:121 ConstantTimeCompare）与平权时延（:124/:137）
- requireAdminSession Bearer 会话鉴权 + X-Auth-Token 兼容（:623-637）

## 结论

1. **身份防线矩阵第三十轮闭合（里程碑）**：COUNT=1（57bf401），零产品改动链延续第十轮；sameClientFor 七调用点行号零漂移；写点全家福 12 类逐一对应防线；偶发写点专项四类核完；手动四路 + maybeRelogin 双侧在契。矩阵方法论三十轮演进复盘完成，未发现新盲区，建议下轮起抽样化全家福 grep（sameClientFor 七点保留全量）。
2. **OBSERVE-111-01/112-03 盯守第四轮**：维持，零漂移（恒 nil 非吞错已核 / 动作维度可区分已核）。
3. **B110-01 审计链第五轮**：维持，零漂移（六处失败 AppendLog + 成功审计行 + TDD 全绿）。
4. **B110-02 / O105-01**：通过（本地钟自洽 / 夹具逐字符零漂移 + 全包 race 绿）。
5. 无 CRITICAL / MAJOR / MINOR，无新立 OBSERVE——本轮价值为「确认无新证据够格立条」的负断言取证与第三十轮里程碑收口。