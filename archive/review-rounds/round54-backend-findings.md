# round54 后端只读审查发现报告

> 审查基线：master @ `7e8900e`（R53 收官，commit 22a61f5 已含 R53 后端五修）。工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 R53 相关文件；审查期间零改动、绝对只读，唯一新文件是本报告。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go）。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 1-41 + legacy/website-source 逆向契约 + round39~53 各轮报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 本轮新视角六项逐项实证 + 上轮观察项逐条裁决。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0（本轮独立重跑确认）。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 连跑 **7 轮 0 FAIL**（R53 为 6 轮 1 FAIL / 16.7%）——flake 归零，详见 MAJOR-53-01 裁决与 flake 频率对比表。
> 本轮额外实证：SQLite task_log 查询计划 EXPLAIN（50 万行内存库基准）+ db 迁移测试 4 项 + login_page.html 登录表单字段契约核对。

---

## CRITICAL

（无本轮新增 CRITICAL。R53 全部修复复核闭合；本轮新视角六项均确认无数据污染/崩溃/双报通道；7 轮 race 全量连跑零 FAIL 且无 race 报告。）

---

## MAJOR

（本轮无新增 MAJOR。MAJOR-53-01 经本轮 7 轮连跑实证归零，裁决为降级关闭，见下。）

---

## 本轮重点三项核实

### 1. R53 flake 收尾复核：readyProbe + cmd 三件套 —— 语义正确、频率归零，建议 MAJOR-53-01 降级关闭

**readyProbe 语义正确性（逐环核验）**：
- 实现（handler_test.go:182-204）：`http.NewRequest(GET baseURL+"/ready")` → `http.DefaultClient.Do` → 连接层错误重试一次仍失败原样上抛。`/ready` 路径未在 mock 平台 HandlerFunc 的 switch 中注册，落入 default 分支返回 `{"code":1,"msg":"unknown /ready"}` 的合法 JSON 200——**健康探测以"HTTP 成功 + 任意业务 JSON"完成，绝不以业务码为准，语义正确**。
- 慢化评估：每测试 `newTestDepsModeName` 多 1 次本地 HTTP 往返（loopback，~0.1ms 级），api 包 40+ 测试新增总开销 <10ms；api 包全量实测 19-23s（R53 为 87s）——readyProbe 未引入可感知慢化（时长差异来自负载波动，方向为更快而非更慢）。
- 假红评估：readyProbe 失败即 `t.Fatalf`——若引入假红，7 轮全量会显现；本轮 0 FAIL 实证无假红。重复 `resp.Body` Close（`defer` + 显式 Close）仅一次执行，无双重关闭。
- **cmd 三件套（commit 22a61f5）原契约未被破坏**：bench `-n<=0` 回退 200（不除零、不产生负时长）；logintest `-limit` 越界收敛到真实账号数（不 slice 越界）；probe `url.QueryEscape(token)`（token 纯数字但拒绝特殊字符裸插 URL）。三处均为防御性加码，原有"参数有效"路径行为不变。

**flake 频率实证对比**：

| 维度 | R53 | R54 | 变化 |
|---|---|---|---|
| 全量 `-race -p 1` 连跑 | 6 轮 1 FAIL（16.7%） | **7 轮 0 FAIL（0%）** | 归零 |
| api 包隔离 | 12 轮 1 FAIL（8.3%） | 5 轮 0 FAIL（0%） | 归零 |
| zhidao 包隔离 | 20 轮 0% | 7 轮全量内含 0 FAIL | 维持 0% |
| 失败断言行 | captcha_test:28,87 / handler_test:508 | 无 | 归零 |
| race 报告 | 无 | 无 | — |

**裁决**：MAJOR-53-01 **降级关闭**（转入 OBSERVE 级：F52 三通道 + R53 readyProbe 四重收尾已把测试夹具 flake 压到 0/7 轮；残余风险仅"极端低概率的宿主环境冷启动"，由 readyProbe 连接错重试 + socketPreheat 双保险兜底）。后续若 CI 复现，按 OBSERVE 级处理即可，不再升级。

### 2. task_log 清理 3 档方案评估（OBSERVE-53-01 延续）—— 本轮实证量化 + 具体落地建议

**查询计划实证（50 万行内存 SQLite，modernc.org/sqlite）**：
- `EXPLAIN QUERY PLAN SELECT ... FROM task_log ORDER BY id DESC LIMIT 2000` → **`SCAN task_log`（全表扫描）**；取 2000 行实测 **2.6ms**。
- `SELECT ... FROM task_log WHERE account='acct1' ORDER BY id DESC LIMIT 500`（无索引）→ 同样全扫，取 500 行实测 **1.0ms**。
- 建 `(account, id)` 索引后 → `SEARCH USING INDEX idx_task_log_account (account=?)`，取 500 行实测 **1.2ms**。
- 结论：**50 万行量级在现代 SQLite（含内存缓存）下全表扫描仅毫秒级**——R53 估算"100 万行 50-100ms"偏保守，实测量级更低（磁盘 WAL 冷缓存下略高，仍 <50ms）。task_log 要涨到 100 万行需按黄金期 5670 行/窗口估算约 **176 个窗口（数十年）**。索引/扫描优化在当前数据量级实为**防御性投资**，只有当 task_log 长期不清理堆积时才产生实质收益。

**3 档落地建议（供主控决策，本轮不执行）**：
- **档①（索引/查询前置）**：`LoadLogs`/`LoadAllLogs` 查询前置 `WHERE id > (SELECT max(id) FROM task_log) - 20000`（保留最近 2 万条的窗口扫描，自增主键使 `max(id)` 走 O(1) 索引），一行 SQL、零 DDL、零数据删除；或建 `(account, id)` 复合索引（LoadLogs 按账号取近期日志受益，LoadAllLogs 无受益）。**推荐档①**（零契约风险、审计不截断）。
- **档②（上限清理）**：启动或窗口关闭后 `DELETE FROM task_log WHERE id <= (SELECT max(id) FROM task_log) - 20000`。收益：表恒 <2 万行；风险：审计线索截断（本项目日志仅展示用途，无审计法规要求）。ponytail 判：**若主控接受截断，档②更省心**（档①的索引/窗口扫描在 2 万行下恒毫秒级）。
- **档③（归档导出）**：超需求（选课工具无审计法规要求），不推荐。
- **涉及文件**：`store.go`（LoadLogs/LoadAllLogs SQL 或新增清理函数）+ `main.go`（启动调用）或 `scheduler`（窗口关闭回调）。
- **TDD 形态**：档①——插入 3 万行 + 断言 `LoadAllLogs(2000)` 返回最近 2000 条且 `EXPLAIN QUERY PLAN` 不含 `SCAN task_log`（或含 `SEARCH`/`max` 子查询）；档②——`PruneTaskLog(maxRows)` 插入 3 万行 → 调用 → 断言 `SELECT count(*)` ≤ 2 万 + 最近行保留。

**结论**：任务书要求"选出哪档 + 改哪几个文件 + TDD 形态"——**建议档①（LoadAllLogs/LoadLogs 查询前置 max(id)-20000 窗口）为首选**（零删除、一行 SQL、契约不变），若主控接受审计截断则档②并列（上限清理）。本轮仅评估，不执行。

### 3. R53 前端三修后端侧契约复核 —— 全部自洽，begin_times 恒下发

- **begin_times 下发自洽**：`handleElectives` 直传 `data`（`ElectivesData{BeginTimes, Publishes}`，client.go:576-579）——`begin_times` 是平台顶层 `beginTimes` 毫秒数组的透传（HAR 实证全校共享单值；平台未下发时为 nil → JSON 编码为 `null`，数组空时为 `[]`）。**后端恒下发该字段**（空/缺时前端 `data?.begin_times?.[0] ?? null` 安全短路），前端 F39-N1/53-M-1 兜底的数据通道可靠。
- **/state open_time 联动**：`StateForAccount` 依识别槽有效时刻判定 `open_time_known`（识别值已过去 → false），识别槽保留不删。前端 Select 主倒计时 `openTimeStr ?? begin_times[0]`——识别缺席时兜底平台开窗点；识别建立后以识别真值为准。两通道同源（都来自平台 beginTimes 探测），无分叉窗口。**契约自洽，无缺陷**。
- **前端三修落地核验**：M-1 Select 兜底（Select.tsx:749-754 已 `openTimeStr ?? data.begin_times[0]`）；M-2 Toast 去重（Toast.tsx:40-52 同 title 合并更新 description 不新增）；M-3 success variant + duration 可配（Toast.tsx:73 `duration ?? 3500` + Admin 成功 toast 补 variant）。**三修均已落地且与后端契约无冲突**。
- **残余观察**：`/electives` 空快照（窗口关闭）返回空结构体（`begin_times` 为 nil）→ 前端兜底失效、主倒计时回 null——此时 `window_closed=true` 横幅显"已关闭"，展示自洽，无缺陷。

---

## MINOR

（本轮无新增 MINOR。R53 两项 MINOR 已由 commit 22a61f5 实修闭合：MINOR-53-01 httpDo 注释已对齐实现（client.go:453-460），MINOR-53-02 cmd 三件套参数防御已落地。闭合复核证据：commit 22a61f5 包含 5 文件改动，其中 client.go 注释改写 + cmd 三文件边界防御，与 R53 报告修复方向逐条一致。）

---

## OBSERVE

### OBSERVE-54-01（本轮新增）：submitLogin 恒显式发 `priorityId=` 空串 vs 网站 jQuery 对 undefined 丢弃该键

- **位置**：`backend/internal/zhidao/client.go:350`（`form.Set("priorityId", "")`）。
- **实证**：`legacy/login_page.html` 登录表单体 `var para = { captcha, identification, uniqueId, priorityId: localStorage["priorityId"] }`——`localStorage["priorityId"]` 在学生首次登录（未进过 `/home/menus`，layout.js 才写入 `localStorage.priorityId=(t.user||{}).id`)时返回 undefined → **jQuery `postReq` 表单编码对 undefined 值静默丢弃该键**。Go 端恒 `form.Set("priorityId","")` 发出 `priorityId=` 空值。
- **影响**：平台侧解析"空串 priorityId" 与 "缺键"在绝大多数服务端等价（空值不触发切换用户逻辑）；学生登录本就无 priorityId 真实值。属**契约微差**（`undefined→丢弃` vs `空串显式发送`），无实质功能影响。
- **修复方向（可选）**：提交登录时不写该键（`priorityId` 仅管理员切换用户场景有值，学生登录恒空，两者等价），或保留空串并在注释载明与 jQuery 的语义差异。**推荐后者**（一行注释，零行为变化）。
- **TDD**：无（纯契约注释对齐；行为等价无断言差异）。

### OBSERVE-54-02（本轮新增）：accountExists 每请求全表 LoadCredentials

- **位置**：`backend/internal/api/handler.go:1061-1072`（accountExists）+ 调用点 handleElectives:245 / handleElectiveSelect:295 / handleElectiveExit:367 / handleState:528 / handleSetTargets:452（内联 LoadCredentials 循环）。
- **影响**：每次管理员 `?account=` 穿透请求全表查凭据（`SELECT account, password_enc, id_token FROM credentials`）。数据量 <100 账号（部署规模），全表毫秒级，**无需索引/缓存**。与决策锚 7 的"权限路径判据同源、不允许全局静默"精神一致（宁可全表也不准绕过）。**延续观察，不修复**。

### OBSERVE-54-03：task_log 清理量化更新（档①首选）—— 见重点核实 2，建议随主控决策落地。

### 上轮观察项延续（逐条裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| **MAJOR-53-01（flake 残余）** | MAJOR | 7 轮全量 0 FAIL、api 5 轮 0 FAIL、无新断言行；readyProbe 无假红/无慢化 | **降级关闭**（OBSERVE 级残留风险） |
| MINOR-53-01（httpDo 注释） | MINOR | commit 22a61f5 已把注释对齐实现 | **闭合** |
| MINOR-53-02（cmd 三件套） | MINOR | commit 22a61f5 三处防御已落地、原契约不破坏 | **闭合** |
| OBSERVE-53-01（task_log 量化） | 观察 | **实证：50 万行全扫仅 2.6ms，百万行需数十年**；档①（查询前置 max(id)-20000）+ 档②（上限清理）并列，档③超需求 | 延续（建议档①/②） |
| OBSERVE-53-02（tick 无 recover + 无优雅退出） | 观察 | signal.Notify 仍 0 处；SQLite WAL crash-safe 兜底 | 延续 |
| OBSERVE-53-03（孤儿登录） | 观察 | B43-01 决策侧 + B21-03 写回侧双闭合，残余窗口极低 | 延续 |
| OBSERVE-53-04（双槽分叉） | 观察 | [acct]→["*"] 优先级，全校单值不实际分叉 | 延续 |
| OBSERVE-53-05（reloginResults cap 8） | 观察 | 非阻塞丢弃，下个 tick 探测兜底 | 延续 |
| OBSERVE-53-06（failingTargetsStore 无消费点） | 观察 | 仍无消费点，Register 接收 *store.Store 非接口无法注入 | 延续 |
| OBSERVE-53-07（429 头不消费） | 观察 | 全仓 Retry-After 0 处；固定 30s 文案匹配 | 延续 |
| OBSERVE-53-08（XUANKE_PORT 无校验） | 观察 | `:abc` log.Fatalf 拒绝启动，形态清晰 | 延续 |
| OBSERVE-53-10（flake 伪装面） | 观察 | **归零**（并入 MAJOR-53-01 关闭） | 闭合 |
| OBSERVE-53-11（access_limit_cookie 占位） | 观察 | 三处占位值不统一但不影响鉴权；**本轮新增 OBSERVE-54-01（priorityId 空串契约微差）同族** | 延续 |
| OBSERVE-53-12 / M40（本地钟混用） | 观察 | 残余均与开窗判点无关 | 延续 |
| OBSERVE-52 及更早全部观察项 | 观察 | 无变化 | 全部延续 |

---

## 本轮新视角六项实证结论

### A. AppendLog 写点全量清点 + 日志行大小
- **写点清单**：scheduler.go 8 处（1500 失效 / 1528 成功 / 1554 风控 / 1610 实时复核失效 / 1647 失败 / 1755 满员 / 1961 手动成功 / 2014 手动退选）+ handler.go 6 处（126 管理员登录 / 227 登录 / 513 目标保存 / 571 注销 / 819 配置更新 / 1007 删账号）= **14 处生产写点**，全部 `if err != nil { log.Printf }` 零吞错（决策锚 17 符合）。黄金期高发写点为 scheduler 的 select 族（1500/1528/1554/1610/1647/1755）。
- **行大小**：result 文案中文为主——典型 60-160 字节（如"账号 acct1: 触发平台风控退避 30s: 报名失败: 选课处理中请勿重复操作！"约 90 字节），含 `err.Error()` 时可到 200+ 字节。黄金期 9 链 × 250ms 冲刺换算，**单窗口 ≈5670 行 ≈ 0.9-1.1MB**（与 R53 估算一致）。为 task_log 清理方案提供精确输入。

### B. store.LoadAllLogs 查询路径 —— 实测见重点核实 2（SCAN + 索引收益量化 + limit 边界正确）
- `limit` 边界：LoadLogs `limit<=0 || >500 → 100`；LoadAllLogs `limit<=0 || >2000 → 500`——0/负数均安全回退，无负 LIMIT 崩溃。
- 与 `ORDER BY id DESC` 对自增主键的利用：**无索引下全表扫描但毫秒级**（50 万行 2.6ms），百万行量级才需索引/窗口前置（见重点核实 2）。

### C. runtime.Store Update/Get 快照语义 —— 无缺陷
- `Get()` 返回 `Config` 值拷贝（全值类型：bool/string/int），调用方握有不随后续 Update 变化的稳定快照，**无半态**。
- `Update()` 闭包在同一写锁临界区执行读-改-写，原子；handler.go 热改路径 `rt.Update(func(c *Config){...})` 内只改字段不触网络/SQLite（落库与 dispatch 都在 Update 之后），**无持锁慢路径**。

### D. session.Store sweeperLoop 生命周期 —— 无缺陷
- `New(ttl>0)` 才建 `sweeperStop/sweeperDone` 并启协程；`ttl<=0`（测试零 TTL 场景）不启动。
- `Close()` 由 `sweeperClose sync.Once` 幂等；`sweeperStop==nil` 直接 return（不 close nil channel）；正常路径 `close(stop)` → `<-done` 等待协程退出，无在途 sweep 泄漏。
- ticket 5min TTL：`CreateTicket` 锁内写；`ConsumeTicket` 锁内四查（存在/used/过期/账号匹配）后 delete——单次使用 + 防重放 + 过期淘汰全部成立；sweepExpired 每 5min 扫描 tickets 清过期。**无泄漏/竞态**。

### E. db.go 迁移 + refuseLegacy 清单 —— 增量幂等正确（R54 无新列）
- `migrateAddPublishMeta` 逐列 `columnExists` 判存在 → 缺才 `ALTER TABLE ADD COLUMN`，幂等（重跑全跳过）；`refuseLegacy` 缺列清单（targets.priority / targets.allow_swap / task_log.account）与新迁移列（publish_name/begin_date）**对应剔除正确**——新增列绝不触发旧库拒绝启动（数据保留契约）。
- 实测 4 项迁移测试全绿：TestMigrateAddsPublishMetaColumns / TestRefuseOldSchemaMissingColumns / TestRefuseLegacyDB / TestRefuseEmptyAccountTargets。
- **R54 无新增列**，迁移规范遵守度评估无需扩面。

### F. zhidao doRequest 响应体解析 —— 无缺陷、无优化必要
- findElectivesData 响应（82 门 × 57 字段 ≈ 40-80KB）读入 `[]byte` + 两次 Unmarshal（doRequest code 提取 + parseElectives 全量）——**峰值内存 ≈ 2-3× 响应体，毫秒级解析**，无放大缺陷。
- `extractMsg` 二次解析仅在 code=-1 错误路径触发（`{msg}` 极小结构），开销可忽略。
- code=-1 包装链：`ErrUnauthorized` + `extractMsg(data)`（`%w` 包裹）——上层 `errors.Is(err, ErrUnauthorized)` 全链匹配成立（spawnChain/probe/handleElectiveSelect 六处实测断言生效）。**无优化必要**。

---

## 已核对无缺陷的高风险区域

- **身份防线族**：sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员两分路）+ maybeRelogin 决策侧（B43-01）/写回侧（B21-03）+ waitChainExit 等待契约。测试 6+ 个（TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}+RealtimeUnauthorized）7 轮全绿。
- **WindowClosed 三判据单源**：windowClosedLocked 主判据 + 时钟失败 + 幽灵窗口三条，StateForAccount/WindowClosed/admin stats 共用。钉子测试 TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler / TestSubmitSuspendedWhenOpenTimeCleared 全绿。
- **鉴权与数据安全**：管理员双条件（B43-04）+ 撞名学生响应无 adminName（F52-M1）；writeJSONStatus 基础设施状态码家族（panic 500/会话 401/管理 403/限流 429/CSRF 403）；SpaHandler /api 双处 404；XFF 仅回环+开关；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 32 字节双重校验；登录/激活独立限流桶。
- **SQL**：全参数化绑定；columnExists/migrateAddPublishMeta 全常量；refuseLegacy 缺列清单对应剔除；**R54 新视角 B（LoadAllLogs 查询路径）实测**。
- **登录链路**：RSA-PKCS1v1.5 / empty priorityId（本轮实证 jQuery 丢弃 vs Go 空串，见 OBSERVE-54-01）/ captcha Limiter Cond 动态热收敛 / 重试收敛 ≤3 / 验证码一次性绝不带同码重试。
- **连接池与时钟**：sharedTransport MaxIdleConnsPerHost=64 + 120s 空闲；SyncServerTime 中点近似 + 成功才推进 lastSyncTime + 失败 30s 退避 + streak≥3 复位。
- **R53 前端三修后端契约**（重点核实 3 全部自洽）。
- **新视角 A/C/D/E/F 全部无缺陷**（见上）。

---

## 结论

- **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3 新增（54-01 priorityId 契约微差、54-02 accountExists 全表、54-03 task_log 量化更新）+ 延续 14 项**。R53 两个 MINOR 与 MAJOR-53-01 全部由 commit 22a61f5 实修闭合；R53 前端三修后端契约无冲突。
- **flake 频率结论**：**R53 全量 16.7% → R54 全量 7 轮 0%（0 FAIL）**、api 隔离 5 轮 0%（R53 8.3%）、zhidao 维持 0%——readyProbe + socketPreheat + 三通道四重收尾实证归零，MAJOR-53-01 **建议降级关闭**。
- **task_log 落地建议**：档①（LoadLogs/LoadAllLogs 查询前置 `WHERE id > (SELECT max(id)-20000)`，零删除零 DDL）**首选**；档②（启动/窗口关闭后 DELETE 保留 2 万条）按主控是否接受审计截断并列；档③归档超需求。涉及 store.go + main.go/scheduler，TDD 为插入 3 万行 → 断言负载与行数上限。
- **最致命 3 条（按影响排序）**：
  1. **OBSERVE-54-03（task_log 无清理）**——虽实证 50 万行全扫仅 2.6ms，但表无清理即无限增长，累积到百万行后 stats/admin 日志页退化；建议随档①一行 SQL 落地，防患于数十年后不划算的代价。
  2. **OBSERVE-54-01（priorityId 空串 vs jQuery 丢弃）**——契约微差，学生登录恒无真值，影响极低；一行注释对齐即可。
  3. **OBSERVE-53-02（tick 无 recover + 无优雅退出）**——SIGTERM 在飞 success 未落库时重启 RestoreDone 不恢复、该课被重新提交（平台幂等兜底可接受），延续观察。
- **建议优先修复方向**：① MAJOR-53-01 关闭（无需代码，报告决议即可）；② task_log 档①一行 SQL + 迁移测试（若主控拍板）；③ OBSERVE-54-01 提交登录注释对齐（可选）。

---

## 附：本轮实证数据表

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| 全量 race 7 轮 | 0 FAIL（每轮 api 19-23s / scheduler 14-15s / 其余包 1-4s） |
| db 迁移测试 | 4/4 全绿 |
| SQLite 50 万行 EXPLAIN | LoadAllLogs `SCAN task_log` 2.6ms；LoadLogs 无索引 1.0ms；带 (account,id) 索引 1.2ms |
| login_page.html 实证 | `priorityId: localStorage["priorityId"]`（undefined → jQuery 丢弃键） |