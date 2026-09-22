# R92 后端只读审查报告

审查对象：xuanke-auto HEAD `62d8da7`（R91 收官，进度 92/256）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round92-backend-findings.md）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race 测试 / go build / go vet / gofmt / 定向走读 + git log 溯源）。核心走读范围：backend/main.go、internal/{api,accounts,db,runtime,scheduler,session,store,secure,zhidao} 全量、cmd/{bench,probe,logintest} 三工具、quit_shared.go、web/embed.go、db/schema.sql + db.go 迁移族。新契约角度本轮聚焦：**数据库迁移与 schema 演进一致性**（migrateAddPublishMeta / refuseLegacy / 新列添加规范 / 历史遗留死列）。

> 注：并行前端审查代理在其会话中修改了 `web/src/routes/Select.tsx` 并产出 round92-frontend-findings.md（本报告写作时 git status 可见），属前端代理产出，非本代理改动；本报告的工作树洁净声明限定在后端仓库文件范围。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O92-01：targets.allow_swap 为全仓零消费点的历史遗留死列，且 refukeLegacy 清单冗余（走读 + git log 溯源实测）

**证据**：`grep -rn allow_swap` 全仓仅命中三处——`internal/db/schema.sql:29`（列定义）、`internal/db/db.go:79`（refuseLegacy 缺列拒绝启动清单）、`internal/db/db_test.go:40`（迁移测试造旧表）。**scheduler.go 全 2053 行无 allow_swap/Swap/maybeSwapLocked 任何消费**；store.go 的 `INSERT INTO targets`（:103）与 `SELECT ... FROM targets`（:113）均不含该列；web/ 前端 ts/tsx 零引用。git log 溯源：该列由 `6ac5978 feat(scheduler): 实现骑驴找马自动换课与失败回抢防护引擎`（2026-09-13）引入，当时 store.go 的 INSERT/SELECT 含 allow_swap，scheduler 有 maybeSwapLocked 换课引擎（164 行增量）；该换课引擎在后继演进中整体移除（scheduler.go 当前无任何残留），store 读写随之去掉该列，**但 schema.sql 列定义与 db.go 缺列清单未同步清理**。

**触发场景推演**：无运行时触发——列在 schema 中定义、所有兼容库都有该列，无实际拒绝启动行为变化（v2 库 priority/allow_swap/task_log.account 三列同批引入，refuseLegacy 拦截由 priority 已兜住，allow_swap 在清单中冗余，不改变任何拒绝集合）。**危害面是维护误导**：下个维护者读 schema.sql 看到 allow_swap 列、读 db.go 看到它被列为"兼容性必需列"，会误以为系统存在"换课开关"语义并尝试消费——实际零消费（与 R40-B40-02 定案"死配置字段必须连同文档清除"同族，但此处是 schema 列非配置字段，删列属破坏性 DDL 不轻动）。

**修复建议**：不删列（SQLite 删列破坏旧库兼容性，且列存在无害），在 `schema.sql:29` 与 `db.go:79` 清单两处补注释「allow_swap 为历史换课引擎遗留列，当前版本无消费点，保留仅兼容旧库；勿据此推断系统存在换课开关」。属注释级澄清，无行为修改。**走读 + git 历史实测**，非行为缺陷。

### O92-02：cmd/logintest 识别引擎选择读环境变量而非运行时配置中心，与主程序热改可短期分叉（走读推断）

**证据**：`cmd/logintest/main.go:57` `switch config.CaptchaEngineDefault()`——该函数（config.go:106）只读 `XUANKE_CAPTCHA_ENGINE` 环境变量 + 默认 ddddocr；而主程序 main.go:68-111 先建 `runtime.New(...CaptchaEngine: config.CaptchaEngineDefault())` 再以 settings 表持久化值**覆盖**（captcha_engine 键，main.go:102-104），且管理员可经 PUT /api/admin/config 热改（handler.go:785-788 落入 settings）。主程序"运行时配置中心优先于环境变量"与 logintest"仅环境变量 + 编译默认"两套判定源——管理员在后台把引擎热改为 vision 后，logintest 仍按 env 默认 ddddocr 执行识别。

**触发场景推演**：管理员配置识别引擎为 vision（settings 持久化）→ 运维用 logintest 批量验证登录 → logintest 走 ddddocr 分支，若本机无 Python/ddddocr 则回退 Vision（logintest:62-68 三态回退链仍会落到 Vision），实际多数场景仍可用；仅在"ddddocr 本地可用但管理员刻意切 vision（如本地引擎不稳定）"时 logintest 行为与主程序分叉。**低风险**：logintest 是运维诊断工具，不参与生产抢课，且三态回退链提供兜底。

**修复建议（可选）**：logintest 改用 `st.LoadSettings()` 读 captcha_engine 键（与主程序同源），缺失时回退 env。属工具一致性增强，非缺陷。**走读推断**，未实测（需登录场景复现）。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无新可疑项 | 本轮走读未发现需主控深度核实的存疑点；O92-01/02 均为已闭环的观察项，无需主控介入。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（R91 矩阵逐点复核）**：spawnChain 链顶双 ClientFor（:1392 + :1409-1415 二次判 + :1421 指针捕获）、六分支 sameClientFor（失效 :1493 / 成功 :1525 / 风控 :1555 / 窗口关闭 :1575 / 实时复核 ErrUnauthorized :1604 / 实时复核满员 :1639），maybeRelogin 决策侧（:1212 ClientFor 存在性）+ 写回侧（:1258），ProbeForAccount 回写段（:854 sameClientFor + :859-862 识别槽双条件），ProbeNow 全局帧（:967-971）/ probe() 全局帧（:1110-1120 无身份维度，全局语义正确），MarkDone（:1931）/ RemoveDone（:1999）/ SubmitAll（:1359）ClientFor 存在性，Restore 路径（RestoreDone:611 / RestoreTargets:529 / RestoreRefused:630 无网络不需身份）。`clientIdentity` 用 reflect.ValueOf(c).Pointer()（:219-228）；`sameClientFor` 判 nil 恒 false（:208-214）。**无新裸露写点**。 | 通过 |
| **B88-01 修复持续复核**：ProbeForAccount 回写段持 s.mu 内先 sameClientFor（:854），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:859-862）；`s.acctData` nil 守卫在身份复核后、写入前（:863-866）。probe_identity_test.go 三钉（TestProbeDeletedThenRebuiltSameNameDropsSnapshot / TestProbeForAccountDropsWriteWhenRemoved / TestProbeChainSameClientIdentity）本轮定向 + 全量 race 回归全绿（见构建验证表）。 | 通过（实测 + 走读） |
| **O91-01 runtime.Config Get 拷贝语义复核**：config.go:35-39 Get RLock 返回值拷贝、Update（:42-48）写锁内原子应用。读点集中在 api 层（activationEnabled/handleAdminConfig/handleAdminStats/applyCaptchaRecognizerFor），写点唯一=handleAdminConfig PUT。读快照与热改无脏读，记录仍准确。 | 通过（走读） |
| **O91-02 SaveSettings 单事务全替换复核**：store.go:351-366 单事务 DELETE+循环 INSERT、defer Rollback 原子回滚；LoadSettings（:369-384）全读由 main 恢复（main.go:77-111）。当前 settings 族 5 键全替换无缺失。记录仍准确，无新触发面。 | 通过（走读） |
| **O86-01 api 抖动基线第七轮**：后台全量 `go test -race -count=1 -p 1 -timeout 900s ./...` **14 包 exit 0**（api 27.080s / store 20.091s / scheduler 15.387s / zhidao 2.294s / accounts 1.887s 实测）；**connectex 零样本**（R86-R92 连续七轮零复现，grep 日志零匹配）。 | 通过（实测） |
| **M87-01 窗口再评估**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）。历轮维持 MINOR + 注释兜底，仍准确。 | 通过 |
| **数据库迁移与 schema 演进**（本轮新角度）：Open 流程「建表 → migrateAddPublishMeta → refuseLegacy」（db.go:15-37）顺序正确——先迁移补列再拒绝剩余缺列，publish_name/begin_date 明确不在 refuseLegacy 清单（:76-78 注释），TestMigrateAddsPublishMetaColumns / TestRefuseOldSchemaMissingColumns / TestRefuseLegacyDB / TestRefuseEmptyAccountTargets 四测试实测绿（db 包 2.512s 全绿）。**唯一发现 = O92-01 allow_swap 死列**（非行为缺陷）。 | 通过（实测 + 走读） |
| **识别引擎双轨边界**（本轮复核）：native_ocr.go（windows+cgo build tag）`//go:embed` 三资产 + dumpIfDiff 大小比对幂等释出（:95-103）+ initMu.Once 懒加载 + 官方 OCR 模式（ModelDir）归一化契约（:73-89 注释）；native_ocr_stub.go（!windows‖!cgo）返回 false/nil 回退。applyCaptchaRecognizerFor 三态链（ddddocr 原生→本地→Vision）与 SetRecognizer 模板同步（accounts/manager.go:191-216）。 | 通过（走读） |
| **多账号登录/重登并发边界**（本轮复核）：gateWait 阻塞（Relogin）与 gateTryAcquire 非阻塞（LoginByPassword:244）共享 gateMu/gateUsed 同一分钟预算（B42-01）；GatePump 每 30s 广播推进；ResetGateForTest 仅测试。maybeRelogin goroutine 生命周期由 reloginMu 决策串行化 + relogging 单飞 + reloginResults chan cap 8 非阻塞回传（:1282-1285），进程退出强杀符合尽力优雅语义。 | 通过（走读） |
| **HTTP 安全头与响应族**（本轮复核）：securityHeaders 三头（nosniff/DENY/no-referrer，:1213-1215）最外层覆盖全路由；writeJSONStatus 家族先设头再 WriteHeader（:85-89）与 requireJSONBody CSRF-403、登录/激活限流 429、panic 500、管理 403、会话 401、未知 /api/ 404 成族对齐（router.go:69-199）；SPA 兜底 `/api` 前缀双门闭环（web/embed.go:28-31）。 | 通过（走读） |
| **cmd 三工具契约一致性**（本轮新角度）：bench 的 -token/-n 边界防御（main.go:22-25）、probe 用 XUANKE_PROBE_TOKEN env + SharedTransport() 对齐生产连接池（main.go:18-32）、logintest 从 DB 解密凭据 + 三态识别引擎回退链 + -limit 边界收敛（main.go:76-79）。**唯一发现 = O92-02 logintest 引擎判定源与主程序分叉**（低风险观察项）。 | 通过（实测 + 走读） |
| **配置持久化对称性**（本轮复核）：settings 写入=PUT /api/admin/config 单事务全替换、读取=main 启动 LoadSettings 全读恢复（main.go:77-111）；vision_key enc: 前缀 + 明文拒绝加载（main.go:87-97）与 secureEncrypt（handler.go:62-71）对称；DeleteAccount 六表清行 + PurgeAccount 内存全清（memory-first 顺序，handler.go:1004-1010）配成"内存+库"双清。 | 通过（走读） |

### 身份防线全员清点矩阵（R92 复核——R91 矩阵延续，行号未漂移）

| # | 写点 | 网络往返 | 复核类型 | 结论 |
|---|---|---|---|---|
| 1 | spawnChain 链顶（:1392 + :1409-1415） | 取 client 前 | ClientFor 存在性（两次）+ 指针捕获 :1421 | 通过 |
| 2 | spawnChain 失效分支（:1493-1497） | SelectClass → ErrUnauthorized | sameClientFor | 通过 |
| 3 | spawnChain 成功分支（:1525-1529） | SelectClass 成功 | sameClientFor | 通过 |
| 4 | spawnChain 风控退避分支（:1555-1558） | SelectClass 命中 rateLimit | sameClientFor | 通过 |
| 5 | spawnChain 窗口关闭分支（:1575-1578） | SelectClass 命中 closed | sameClientFor | 通过 |
| 6 | spawnChain 实时复核满员分支（:1639-1642） | classFullRealtime 网络段后回锁 | sameClientFor | 通过 |
| 7 | spawnChain 实时复核 ErrUnauthorized 分支（:1604-1624） | classFullRealtime 网络段后回锁 | sameClientFor | 通过 |
| 8 | maybeRelogin 决策侧（:1212） | 发起前 | ClientFor 存在性 | 通过 |
| 9 | maybeRelogin 写回侧（:1258-1262） | Login 返回后 | ClientFor 存在性 | 通过 |
| 10 | ProbeForAccount 回写段（:854，B88-01） | FindElectives 返回后 | sameClientFor | 通过（七轮实测绿） |
| 11 | ProbeNow 全局帧（:967-971） | FindElectives 返回后 | 无身份维度（全局帧） | 通过 |
| 12 | probe() 全局帧（:1110-1120） | FindElectives 返回后 | 同 ProbeNow | 通过 |
| 13 | MarkDone（:1931-1934） | 手动报名网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 14 | RemoveDone（:1999-2002） | 手动退选网络段由 api 层持有 | ClientFor 存在性 | 通过 |
| 15 | SubmitAll（:1359-1361） | 无网络（内存链构建） | ClientFor 存在性（下发链前过滤） | 通过 |
| 16 | Restore 路径（RestoreDone:611 / RestoreTargets:529 / RestoreRefused:630） | 无网络 | 不需（账号都在注册表） | 通过 |

## 契约抽查表（抽查 7 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:431-442），空快照不删槽、非空 beginTimes 才覆盖（:859-862 / :1110-1114）；识别过期只影响展示层（StateForAccount:718 After 判定），tick 提交守卫 `open.IsZero() && !opened` 让位于 WindowOpened（:1015-1017）。 | 通过 |
| **2/3（窗口关闭判据单源 + 关闭≠时间消失）**：windowClosedLocked 三判据单源（:922-943）+ 主判据 10s 裕量（:1150）+ EmptyProbeRuns 入账同 10s 裕量（:1159）；StateForAccount:707 与 WindowClosed() 共用同一实现。 | 通过 |
| **21（防寄生 = 指针身份比对）**：sameClientFor + clientIdentity（reflect.ValueOf(c).Pointer()）与矩阵 1-16 全闭合。 | 通过（回归实测绿） |
| **B42-01（doLogin 全入口统一频率闸门）**：gateTryAcquire 非阻塞（LoginByPassword:244）+ gateWait 阻塞（Relogin）共享 gateMu/gateUsed 同一窗口计数；ResetGateForTest 仅测试。 | 通过 |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（spawnChain 六分支 / MarkDone / RemoveDone / markFullLocked / SetTargetsForAccount 落库与 DeleteRefused / maybeRelogin UpdateIDToken / handler 登录/配置/删除 / accounts SaveCredential 失败）。 | 通过 |
| **M86-01（托盘退出闭环）**：quit_shared.go 注入点双 nil 防御（:29-35）；srvShutdown → ListenAndServe 返 ErrServerClosed → main 自然退出（main.go:201-205）。 | 通过 |
| **B40-02（死配置字段连同文档清除）**：`Config.OpenTime` 已彻底移除（config.go:55-57 注释明确「配置层不再注入」，.env 模板无该键）——**但 schema 层暴露 O92-01 同类隐患**（allow_swap 死列未标注），已记录待注释澄清。 | 通过（部分观察） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `cd backend && go test -race -count=1 -p 1 -timeout 900s ./...`（后台） | **全绿**，14 包 exit 0（api 27.080s / store 20.091s / scheduler 15.387s / zhidao 2.294s / accounts 1.887s / db 2.512s）；connectex 零样本 |
| 定向回归（probe_identity 三钉 + 迁移族 + 窗口状态 + 管理员 stats） | `scheduler 2.470s / db 1.075s / api 1.107s` 全绿 |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过（零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音；git 仓库版本 LF 合规零检出——O90-01 延续，无需动作） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`M web/src/routes/Select.tsx`、`?? round92-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵延续**：逐点复核全部网络往返后写状态/落库点，无新裸露写点；B88-01 修复回写段七轮 race 回归全绿；probe_identity_test.go 三钉实测通过。
2. **O86-01 api 抖动基线第七轮零复现**（race 全绿、connectex 零样本）；O91-01/02 记录复核仍准确；M87-01 维持 MINOR + 注释兜底。
3. **新角度扫查**：数据库迁移与 schema 演进全流程（Open 顺序 / 迁移 TDD / refuseLegacy 清单）一致，唯一发现 **O92-01 targets.allow_swap 全仓零消费死列**（git 溯源 6ac5978 换课引擎已移除但 schema/db.go 未同步清理，注释级澄清建议）；cmd 三工具契约一致，唯一发现 **O92-02 logintest 引擎判定源与主程序热改分叉**（低风险观察）。识别引擎双轨、多账号登录/重登并发边界、HTTP 安全头成族、配置持久化对称性全部闭环。
4. **新增 OBSERVE 两条**（O92-01/02），无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求。

工作树后端文件洁净。本轮为纯观察轮（与 R90/R91 同型），无代码修改建议提交。
