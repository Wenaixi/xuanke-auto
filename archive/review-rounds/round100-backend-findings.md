# R100 后端只读审查报告

审查对象：xuanke-auto HEAD `9af1de0`（R99 收官，进度 100/256；R100 里程碑轮为纯观察轮延续——HEAD 与 R99 相同，无新代码提交）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round100-backend-findings.md）。并行前端代理产出 round100-frontend-findings.md（见 git status `??`，非本代理改动）。

审查方式：Read / Grep / Glob / Bash 只读命令（后台全量 race **四轮** / go build / go vet / gofmt / 逐点走读）。核心走读范围：backend/main.go、internal/{scheduler(全量),api(handler+router 全量),zhidao(client+captcha+rsa+device_id+local_ocr+native_ocr 双文件+stub),accounts,store,session,db+schema,config,runtime,secure}、quit_shared.go、tray_windows.go/tray_linux.go/tray_linux_cgo0.go/tray_other.go/browser 双文件、native_ocr 资产、cmd 三工具（bench/probe/logintest）。新契约角度本轮聚焦：**主程序启动顺序完整一致性（config.Load→db.Open→secure→accounts→runtime→scheduler→tray→http 的依赖与失败路径）+ 平台契约逆向对照（legacy/website-source+HAR 与当前实现差异）+ 多账号全局资源边界（连接池/限流/节流/缓冲在 N 账号下按需扩展性）+ 日志系统可运维性（task_log 语义/零吞错/脱敏）+ 安全口纵深（凭据加密→会话→管理鉴权→CSRF→激活码→限流多层有效性）**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE

### O100-01（O86-01/O97-01/O98-01/O99-01 api 抖动基线第十五轮）：四轮全量 race 三绿一"TestRecognizeCaptcha 单次 connectex"——残余宿主轮换回 zhidao 首测试，低频波动口径实证（实测）

**证据**：四轮全量 race（`go test -race -count=1 -p 1 -timeout 900s ./...`）：**R1 全 11 包全绿**（api 37.441s / store 73.073s / scheduler 16.043s / zhidao 3.892s）；**R2 仅 zhidao 包单次 FAIL**（17.560s，远超正常 2.3-4.7s 的异常耗时）——失败为 `TestRecognizeCaptcha` 的 `readyProbe` 探活首请求 connectex（`captcha_test.go:29`，10 次轮询全败），后续定向复现（第三轮 zhidao 4.678s 全绿）零复现。**R3 全 11 包全绿**（api 39.255s / store 67.176s / zhidao 4.678s）；**R4 全 11 包全绿**（api 24.175s / store 56.989s / zhidao 3.960s）。

**失败归因**：TestRecognizeCaptcha 是 zhidao 包中**唯一保留"无探活 mock"形态的首测试**（captcha_test.go:27-28 注明"包内仅剩的无探活 mock 首请求宿主"）——虽已加 readyProbe 双保险，但全量轮前序包（R2 前序 store 73s 高耗时）TIME_WAIT 冷启动窗口残留偶发穿透 readyProbe 的 ~2s 轮询窗口。**与 R94/R97/R98 低频残余同族**（R94/R97 均出现过 zhidao 包单次 connectex，残余宿主轮换于 api/zhidao 之间），非产品逻辑缺陷，无 panic / 无 DATA RACE。

**基线结论维持**：低频波动口径不变——"宿主环境冷启动窗口 + 包序主导，CI `-p 1` + 失败重跑吸收，非产品缺陷"。R99 四轮全绿后的本轮单例回升不改变该口径（历轮样本：R94-R100 七轮 26 跑中 4 次单包单 FAIL，~15%）。**不推荐为此改代码**（readyProbe 已全覆盖、即时修复收益趋零、产品代码零关联）。

### O100-02（O99-02/O98-02/O97-04/O80-01 删除保护撞名延续复核）：handleAdminDeleteAccount 单判据与 B43-04 双条件不对称——历轮维持观察，本轮无新依据提级（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只看账号名等于 `AdminNameValue()`；IsAdminAccountName（handler.go:56-58）单字段比对。撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下该学生被删除保护永久覆盖。但：① 该路径 requireAdminSession 管理会话鉴权前置（router.go:153-159）——即使撞名学生账号在注册表，也必须有管理员会话 token 才能到达删除路径；② B43-04 已为登录路径双条件签发，撞名学生可正常教务登录；③ 删除保护是"防空删除管理员自己"的硬护栏，与登录准入语义不同。历轮归"低优先级不修"，本轮走读一致维持观察。

### O100-03（O90-01 scheduler.go gofmt CRLF 噪音延续复核）：工作区 CRLF 转换噪音，git 仓库内容 LF 合规（实测）

**证据**：`gofmt -l .` 仍只检出 `internal/scheduler/scheduler.go`；`git show HEAD:backend/internal/scheduler/scheduler.go` 实测 CRLF 0 / LF 2049，差异纯为 checkout 时 `core.autocrlf=true` 的 LF→CRLF 转换噪音。非源码缺陷，无需提交动作。历轮 O90-01 维持。

### O100-04：长期挂账观察项清算评估（里程碑轮视角，走读；仅评估不落地）

**候选清零的挂账项**：

| 挂账项 | 实际影响 | 修复成本 | 裁决建议 |
|---|---|---|---|
| **O92-02 logintest 引擎判定源分叉**（cmd/logintest/main.go:57 走 `CaptchaEngineDefault()` 环境变量 + 编译默认，主程序以 settings 持久化值覆盖） | 纯诊断工具不参与抢课；仅"管理员刻意切 vision 而本机有 ddddocr"时登录测试引擎与生产分叉，但三态回退链完整（ddddocr→原生→本地→Vision） | 读取 settings 表 + 解密 vision_key，约 20 行 | **维持不修（已三档案关闭）**——诊断工具与生产热改分叉是"工具不加载运行时配置中心"的固有语义，非缺陷；本心理保持最小。
| **O98-02 删除保护撞名**（同上 O100-02） | 需"管理员名与学号撞名 + 管理会话被破坏"两条件叠加才触达；硬护栏语义优先 | 改为指针身份比对需穿透 store 层级 | **维持观察**——历 15 轮零新触发面，清栏需管理员撞名断言，成本>收益。
| **O90-01 scheduler.go CRLF** | 零运行时影响，纯工作区噪音 | 重装 core.autocrlf=false 或单文件 LF 化（会污染 git blob） | **永久维持**——git HEAD 内容 LF 合规，无提交动作。
| **前端 OBSERVE-88-01 ErrorBoundary 缺失** | 历轮（round88-98）零渲染期异常实证 + 全站列表 `?? []` 兜底 + isError 分支；防御充分 | ~20 行 | 归前端代理审视；本端无投票。

**结论**：R85-R99 近 15 轮观察项全部为"低频测试抖动 / 工作区噪音 / 低优先级可选增强"三类，**无一达到值得在 100 轮节点清栏修复的实际影响**。裁决：O92-02 维持关闭态（三档案）、O98-02 维持观察、O90-01 永久无动作。里程碑轮不制造修复。

### O100-05：契约 20 注释轮次标签全仓扫描零违规（实测）

**证据**：grep `（第 \d+ 轮）|round\d\d|R\d\d-\d\d` 全部 backend/*.go（排除 _test.go）**零命中**。历轮固化回归维持（前几轮 session/store.go 的 docs/review-round13.md 文档引用也已剥离）。

## 可疑待核（需主控深度核实）

| 项 | 说明 |
|---|---|
| 无 | 四轮全量 race（三绿 + 一轮单包单次低频残余已归因）+ 逐点走读闭环，无新增可疑项。 |

## 已核无缺陷清单（走读 + 实测）

| 项 | 结论 |
|---|---|
| **身份防线 16 项矩阵延续（第十五轮）**：逐点 grep `sameClientFor`（scheduler.go:204，判 nil 恒 false + reflect 指针身份）+ 全量调用点——ProbeForAccount 回写段（:850）、spawnChain 失效（:1489）/成功（:1521）/风控（:1551）/窗口关闭（:1571）/实时复核入口（:1600）/确证满员（:1635）六分支再加实时复核满员（spawnChain 内 4 分支与 B41-01 六分支闭合并计）；maybeRelogin 决策侧（:1208 ClientFor 存在性）+ 写回侧（:1254）；ProbeNow（:964-967）/probe()（:1106-1116）全局帧无身份维度语义正确；MarkDone（:1927）/RemoveDone（:1995）/SubmitAll（:1355）ClientFor 存在性；Restore 路径（RestoreDone:607/RestoreTargets:525/RestoreRefused:626 无网络不需身份）。round41 七分支测试族全部使用 waitChainExit（scheduler_test.go:3109，绝不用 inflight 等待——PurgeAccount 删 nil map 恒 false 假绿陷阱载明）。grep 全量复核无新裸露写点。 | 通过（走读 + 测试族走读核对） |
| **B88-01 修复持续复核（第十五轮）**：ProbeForAccount 回写段持锁先 sameClientFor（:850），失败整体放弃写 acctData/acctDataAt/识别槽并 return data；识别槽覆盖只在身份通过 + `len(data.BeginTimes)>0` 双条件（:855-858）；acctData nil 守卫在身份复核后写入前（:859-862）。probe_identity_test.go 三钉（:16 同名重建/:116 已删/:176 正常对偶）走读语义 + 四轮全量 race 回归（三绿一低频残余与它们无关）。 | 通过（实测） |
| **主程序启动顺序完整一致性（本轮新角度）**：config.Load（缺失 AdminToken log.Fatal 拒启、缺 SF_API_KEY 仅警告）→ db.Open（建表→migrateAddPublishMeta 增量迁移→refuseLegacy 拒旧）→ secure.LoadOrCreateKey（env 优先，.master_key 32 字节校验，损坏拒启）→ accounts.New + Restore（SetCredentials + SetCookies 合并占位，解密失败留痕）→ runtime.New + LoadSettings 恢复（vision_key 仅 enc: 前缀解密、未加密明文彻底拒绝）→ scheduler.New + RestoreDone/RestoreTargets/RestoreRefused（顺序契约 6）→ sched.Start → runTray → GatePump 后台协程 → session.New + api.Register + SpaHandler → http.Server 显式超时四件套。**依赖有序、失败路径每步显式（log.Fatal / log.Printf + 继续），无初始化竞态**。 | 通过（走读） |
| **平台契约逆向对照（本轮新角度）**：legacy/website-source 25 个 JS + legacy HAR 173 条已核实契约在 zhidao 层全落地——YearTerms 10 字段仅解 3（go 端按需）；findElectivesData 表单编码 vs 空 body 首探 + 学期列表兜底重试（client.go:648-683 双路径）；parseElectives 顶层 code:0 空 publishes 直接返回空快照绝不误入兜底（:709-711）；班级字段 publish_id/group_no 服务端直接下发（Class 结构含 publish_id）三键；btn_type/can_select/title 双守卫渲染满员语义（Class 结构含 CanSelect/BtnType/Title 三字段，scheduler IsClassFull 判 MaxCount>0 && SelectedCount>=MaxCount 同源验证）；轮询 10s ids= 逗号串 + `b` 数组只在 inDateRange=true 下收集（IsClassFull → StudentCounts → form ids 逗号 join 完全对齐）；correctUrl 拼 `=` 判定、token encodeURIComponent（doRequest url.QueryEscape 等价）。**契约零漂移**。 | 通过（走读） |
| **多账号全局资源边界（本轮新角度）**：sharedTransport MaxIdleConns 128/MaxIdleConnsPerHost 64（N 账号多发布并发提交零阻塞）；probeSem cap 4 封顶 per-account 探测并发（并发峰值 N→4，ponytail 注释载明升级路径）；captcha 全局信号量并发 1（默认）动态热调（SetCaptchaConcurrency 1-20 值域校验）；gateLoginPerMin=2 全局 doLogin 预算（gateWait 阻塞 / gateTryAcquire 非阻塞双入口共享 gateMu）；reloginResults buffered 8 非阻塞 select 弃信号不持锁。**N 账号下全局资源全部有界、无 unbounded 增长面**。 | 通过（走读） |
| **日志系统可运维性（本轮新角度）**：task_log 自增主键 max(id) 窗口查询（永不清理但 O(1) 索引窗口 2 万条）；LoadAllLogs 上限 500/2000 边界；scheduler spawnChain 六分支 AppendLog 全部 `if err != nil { log.Printf }` 零吞错；token 脱敏 maskedToken 前 8 位（scheduler.go:1334）与 login 日志 engine 类型不泄密；management 日志 action 四类（login/logout/select/set_targets/config/delete_account）语义完整；启动/退出日志（sched.Start 已启动 / ListenAndServe 监听地址 / 托盘退出）齐全。**可运维性闭环**。 | 通过（走读） |
| **安全口纵深（本轮新角度）**：凭据 AES-256-GCM（secure crypto 双方法 + enc: 前缀 + 主密钥 32 字节校验）→ 会话内存注册表（crypto/rand 32 字节令牌 panic 拒签；12h TTL；sweepLoop 5 分钟清扫）→ 管理鉴权 requireAdminSession Bearer 会话 403 + B43-04 双条件签发 → CSRF（router 全副作用 POST/PUT requireJSONBody 403 + DELETE 空 body 放行语义）→ 激活码（单事务扣次原子 + activation 防重 + 票据单次 5 分钟 TTL 绑定账号）→ 限流（登录/激活独立桶 5/min + tokenBucket 惰性 GC + XUANKE_TRUSTED_PROXY 回环才信 XFF）。**七层纵深逐层有效，无信任边界缺口**。 | 通过（走读） |
| **M87-01 窗口再评估（第十五轮）**：5s Shutdown 超时与在飞 spawnChain 语义仍由 main.go:191-200 注释完整覆盖（尽力优雅：强杀在飞 goroutine，最后时刻提交结果以重启后重试为准；RestoreDone 只恢复已落库 success）；quit_shared.go 双 nil 防御（:30-36）+ setExitActions 两半段注入（main:200 关服务 / tray_windows.go:54 与 tray_linux.go:52 各注入退图标）+ tray_quit_test.go 三测试钉死顺序与 nil 安全。**历轮维持 MINOR + 注释兜底，仍准确**。 | 通过 |
| **零吞错复核（契约 17）**：本轮扫查 spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone、markFullLocked、SetTargetsForAccount 落库与 DeleteRefused、maybeRelogin UpdateIDToken、handleAdminConfig 落库（:814 log + :819 500）、handleAdminDeleteAccount AppendLog、login 分支加密失败/落库失败、accounts LoginByPassword 加密失败（:286）、main 恢复各表失败——全部 `if err != nil { log.Printf }` 零吞错。 | 通过 |
| **O100-01 抖动基线第十五轮**：四轮全量（三绿 + zhidao 单包单次低频残余归因，TestRecognizeCaptcha readyProbe connectex 17.56s 异常耗时，与 R94/R97/R98 同族，宿主轮换回 zhidao 首测试）；R3/R4 全绿。基线结论：低频残余由宿主冷启动窗口 + 包序主导，非产品缺陷，CI `-p 1` + 失败重跑吸收口径不变。 | 通过（实测，O100-01） |

## 契约抽查表（抽查 8 条，逐条验）

| 契约 | 结果 |
|---|---|
| **1（开放时间唯一事实源 + 识别槽不截断零值）**：openTimeForLocked 恒返回识别值本身（:427-438），空快照不删槽、非空 beginTimes 才覆盖（ProbeForAccount:855-858 每账号 / probe:1106-1111 全校）；识别过期只影响展示层（StateForAccount:714 After 判定 → OpenTimeKnown）；tick 提交守卫 `open.IsZero() && !opened` 让位 WindowOpened（:1011-1013）；配置层零注入（config.go:55-57 无 XUANKE_OPEN_TIME 读取、.env 模板无该行）。 | 通过（实测） |
| **2（WindowClosed 三判据单源 windowClosedLocked）**：WindowClosed()（:907）与 StateForAccount（:703 调 windowClosedLocked）共用同一实现；判据 1 主判据（曾开窗 + 空快照 + 已过开放时间 +10s 裕量，probe:1146）；判据 2 时钟失败 ≥3 + 开放时间已过（:925）；判据 3 幽灵窗口 EmptyProbeRuns ≥3 + 开放时间已过（:935）。open 单快照对三条判据统一（:924）。测试预置关闭状态走"空快照形态"（refused_test.go 注释载明）。 | 通过（走读） |
| **6（重启恢复顺序契约）**：main.go RestoreDone（:120）→ 逐账号 Load+RestoreTargets（:132）→ LoadRefused+RestoreRefused（:140-141）；RestoreTargets 仅不清 refused（scheduler.go:525）；SetTargetsForAccount 只在 handleSetTargets 主动调用且只清 refused 绝不清 done/full/rateLimited/inflight（:454 注释族）；TestRestoreDoneSkipsResubmit 固化契约。 | 通过（走读） |
| **14（手动报名/退选 4 方法协同）**：TryAcquireSubmit 在飞互斥（:1896）+ MarkDone 清 done/inflight/full/rateLimited + 库内 refused 行（:1938-1948）+ RemoveDone 落 refused + DeleteSuccess 行（:2018-2028）+ RemoveFull；窗口已关按满员记 full；spawnChain 提交前 inflight 去重（:1464）。 | 通过（走读） |
| **17（落库失败零吞错）**：全仓 `if err != nil { log.Printf }` 零吞错（本轮复核主要写点全绿）。 | 通过 |
| **B42-01（doLogin 全局频率闸门）**：gateTryAcquire 非阻塞（manager.go:223-235）+ gateWait 阻塞（:49-64）共享 gateMu/gateUsed 同一窗口计数；LoginByPassword pre-Login 准入（:244）；ResetGateForTest 仅测试（:82）。双入口无绕行。 | 通过（走读） |
| **B43-04（撞名双条件）**：handler.go:121 管理员名 + `subtle.ConstantTimeCompare` 双条件；不匹配的撞名学生走教务登录正常签发；教务也失败且账号是管理员名才报"管理口令错误"+ loginTimingFlat 300ms 时延拉平（:121-142）。 | 通过（走读） |
| **7（?account= 透传全路径凭据表校验）**：handleElectives:245 / handleElectiveSelect:295 / handleElectiveExit:372 / handleSetTargets:453 / handleState:537 五处 accountExists / 凭据表逐账号比对全覆盖，查无账号整体拒绝，绝不用全局帧假装成功；仅管理员会话可穿透（allowAccountOverride:1066）。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test -race -count=1 -p 1 -timeout 900s ./...`（R1） | **全 11 包全绿**（api 37.441s / store 73.073s / scheduler 16.043s / zhidao 3.892s） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（R2） | 11 包中 10 包全绿，**zhidao 包单次 FAIL**（17.560s，TestRecognizeCaptcha readyProbe connectex，O100-01 低频残余归因，无 panic 无 DATA RACE） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（R3) | **全 11 包全绿**（api 39.255s / store 67.176s / zhidao 4.678s） |
| `go test -race -count=1 -p 1 -timeout 900s ./...`（R4） | **全 11 包全绿**（api 24.175s / store 56.989s / zhidao 3.960s） |
| `go build ./...` | 通过（BUILD_EXIT=0） |
| `go vet ./...` | 通过（VET_EXIT=0，两次零输出） |
| `gofmt -l .`（backend） | 仅 `internal/scheduler/scheduler.go`（CRLF 工作区转换噪音，O90-01 延续；git 仓库版本 LF 合规——O100-03 实测 CRLF 0/LF 2049） |
| `grep` 契约 20 轮次标签全仓扫描 | backend 全部 .go 零命中（O100-05） |
| `git status --short --branch` | `## master` + 并行前端代理产出（`?? archive/review-rounds/round100-frontend-findings.md`，非本代理改动）；后端仓库文件零改动 |

## 结论

1. **身份防线 16 项矩阵第十五轮延续**：逐点 grep 全部「网络往返后持锁写状态/落库」点，无新裸露写点；B88-01 修复回写段复核通过；probe_identity_test.go 三钉走读语义正确；round41 测试族 waitChainExit 等待契约载明。**身份防线矩阵已达十五轮连续闭合**。
2. **O86-01 抖动基线第十五轮**：四轮全量（三绿 + R2 zhidao 单包单次低频残余归因）。R2 具体失败 TestRecognizeCaptcha readyProbe connectex（17.56s 异常耗时）与 R94/R97 的 zhidao 同族、残余宿主轮换回 zhidao 首测试。**基线维持低频波动口径：CI `-p 1` + 失败重跑吸收，非产品缺陷**（R94-R100 七轮 26 跑中 4 次单包单 FAIL，~15%）。
3. **新角度扫查全绿**：主程序启动顺序完整一致（config→db 迁移→secure→accounts→runtime→scheduler→tray→http 每步失败路径显式）；平台契约逆向对照零漂移（HAR/website-source 已核实契约全落地）；多账号全局资源全部有界（连接池 128/64、probeSem cap 4、captcha 1-20、gate 2/min、reloginResults buffered 8）；日志系统可运维性闭环（task_log O(1) 窗口 + 零吞错 + token 脱敏前 8 位）；安全口七层纵深逐层有效。
4. **里程碑轮清算评估**：R85-R99 近 15 轮观察项全部为"低频测试抖动 / 工作区噪音 / 低优先级可选增强"三类，无一达清栏修复实际影响——O92-02 维持关闭（三档案）、O98-02 维持观察、O90-01 永久无动作。
5. **新增 OBSERVE 两条**（O100-01 抖动基线第十五轮低频残余记录、O100-05 契约 20 零违规回归）、延续复核三条（O100-02 删除保护撞名 / O100-03 CRLF 噪音 / O100-04 挂账清算评估），**无 CRITICAL / MAJOR / MINOR，本轮无代码修改需求**。

工作树后端文件洁净。本轮为纯观察轮（与 R89-R99 同型），无代码修改建议提交。