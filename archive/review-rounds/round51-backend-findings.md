# round51 后端审查原始发现

> 审查基线：master @ `f49d348`（R50 收敛落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档 CODE_OF_CONDUCT/CONTRIBUTING/LICENSE/SECURITY/THIRD_PARTY_NOTICES，无工作区改动；审查期间工作区零改动，绝对只读，未修改/创建/删除任何文件——唯一新文件是本报告）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 + legacy/website-source 逆向契约 + round39~50 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 上轮观察项逐一核实 + 本轮新视角扫查（acctData×WindowClosed、handleState 关闭后 Courses 构造、reloginFail 恢复、targets priority 事务、空 body 首探、SaveSettings 全量替换）。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 连跑 3 轮（第 1/3/6 轮全绿 EXIT 0）+ api/scheduler 组合与 api 三八五轮单跑（15.798~30.8s）——其中 2 次 FULL：
>   ① 组合跑（api+scheduler）：api 包 FAIL，失败 1 测 TestLogoutRevokesToken（无详情尾巴，判定 mock 连接 flake）；
>   ② 三连跑 api 第 1 轮 FAIL 3 测：TestAccountOverrideRequiresAdminSession / TestLoginUnactivatedNeedsCode / TestLogoutRevokesToken，**根因同一**——`handler_test.go:357` 断言收到 `登录失败: 初始化登录会话失败: Get "http://127.0.0.1:50620/login": connectex: A connection attempt failed`，即 httptest mock 服务器瞬时连接建立失败；其中 TestLoginUnactivatedNeedsCode 恰以"未激活应 1001"为断言的测试、也恰好是 R50 OBSERVE-50-01"flake 伪装业务断言"的同根样本。失败测试单跑四轮全绿。详见 MAJOR-51-01。

---

## CRITICAL

（无本轮新增 CRITICAL。F50-M1/M2 双修闭合；identity 六分支 + maybeRelogin 决策/写回侧闭合；本轮新视角六项均确认无数据污染通道。）

---

## MAJOR

### MAJOR-51-01：httptest mock 连接 flake 本体仍未根除——`-p 1` + `-count=1` 双命令重跑能吸收但每月多次误红依然存在（F50-M2 延续/确认，非新增缺陷）

- **位置**：`.github/workflows/ci.yml:59`（`go test -p 1 -count=1 -v ./... || go test -p 1 -count=1 -v ./...`）+ `backend/internal/api/handler_test.go`（doJSON/authenticateDirect 引用的 mock 平台服务器）
- **一句话问题**：F50-M2 已正确修复缓存键不一致（两条命令统一 `-count=1`，重跑侧真实执行——语法/assertion 均正确无回归），**但 flake 本体仍存活**：本轮 6 轮 test 中 2 度出现 api 包 FAIL，失败详情实证根因恒为 `connectex`（mock 平台 HTTP server 瞬时连接拒绝，Windows 127.0.0.1 回环 TIME_WAIT churn）。CI 的 `||` 重跑机制按设计吸收该 flake（首条失败→二条真实重跑→全绿标 PASS），故 R50 断言"重跑链二次跑被缓存羽化失效"已被 F50-M2 根除——遗留的是**低频假红噪音**（本轮 -p 1 下仍约 1/6 轮次误红），非阻塞性缺陷。
- **实证**：① 6 轮全量 `-race -count=1 -p 1 ./...`：3 轮全绿、2 轮单包 FAIL（api）、1 轮组合跑 FAIL；② 三连跑 api 第 1 轮 FAIL 3 测详情均为 `Get "http://127.0.0.1:50620/login": connectex`，且其中 TestLoginUnactivatedNeedsCode 的失败断言形态恰是"未激活应返回 1001（教务处 code=1 login 失败）"——即 mock 连接失败被 handleLogin 的 LoginByPassword err 翻译成"登录失败: 初始化登录会话失败: …connectex"业务文案（与 R50 OBSERVE-50-01 完全同根）；③ 失败测试单跑四轮全绿。**无 race 报告**。
- **裁决**：MAJOR（CI 基础设施级，延续 R49/R50 同族，无升级、无降级）——建议根治路径仍未变（F50-01 建议的登录链路测试夹具首请求失败重试一次，或 mock 服务器启动就绪探测），如主控判断 CI 重跑已足够吸收则可降级观察；本轮不强推。

---

## 本轮重点核对（上轮新契约，防回归）—— 4 条全部闭合

### 1. F50-M1（stats body code 500 对齐）+ F50-M2（CI 重跑 -count 统一）— 双修语法/断言正确无回归
- diff 复核：F50-M1 将 `handleAdminStats` 目标数失败分支 body code 由 `1` 改为 `500`（writeJSONStatus(w, 500, 500)），与 F48-O3 落库失败分支（handler.go:810）家族对齐，注释载明动机；前端契约只读 body code!=0，行为不变。F50-M2 将 CI 重跑两条命令统一 `-count=1`，解决了 R50 MAJOR-50-01 的缓存键不一致漏洞（首条无 count 恒命中缓存 → 重跑被 (cached) 羽化）。两条均正确闭合。唯一残余：B43-05 的 `failingTargetsStore` 测试件仍无消费点（TestAdminStatsTargetsLoadFailureReturns500 只测正常路径 200）+ 908 行 500 分支无真红绿——**观察项延续**（同 R50 MINOR-50-01 已指出"实现注释背书"）。
### 2. F48-O3 config 家族整风 — 正确闭合
- handler.go:810 落库失败分支（body=500）+ handler_test.go:550-586 TestAdminConfigSaveFailStillDispatch 双断言（HTTP 500 + body code 500）真红绿闭合；成功路径仍写 JSON（HTTP 200）+ 值域校验拒绝 + 无变更 PUT 三行为不变。
### 3. F46-O1 clamp + B45/B43/B44 上轮实修 — 全部复核正确闭合无回归
- scheduler.go:237-239 interval clamp + TestScheduleIntervalClamped 真红绿；钉子集（TestWindowOpenSubmitsWithoutProbeReset / TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime / TestAdminStatsWindowOpenedUsesScheduler / TestScheduleIntervalClamped）复跑全绿。
- B43-01 入口复核 + sameClientFor 六分支（成功/失效/风控/窗口关闭/实时复核 cErr+满员两分路）+ maybeRelogin 决策侧/写回侧闭合；身份族测试在库绿。
- B45-N3 加密失败回归钉、B44-01 解密失败日志对称保留——accounts 包回归绿。

---

## MINOR

（无本轮新增 MINOR。R50 MINOR-50-01 的 writeJSONStatus 家族 body code 开裂已被 F50-M1 修复闭合。）

---

## OBSERVE

### OBSERVE-51-01：R50 的 TestLoginUnactivatedNeedsCode 成为"flake 伪装业务断言"的新样本（OBSERVE-50-01 同根延续）
- 本轮三连跑第 1 轮 FAIL 详情实证：`handler_test.go:357` 断言收到 `map[code:1 msg:登录失败: 初始化登录会话失败: Get "http://127.0.0.1:50620/login": connectex...]`——mock 连接失败经 zhidao.Login→LoginByPassword err 翻译成"登录失败"业务文案，测试以"未激活应返回 1001"断言而失败。观察到具体断言行号（357），佐证 R50 的 flake 伪装面 "从断言显示连接错误到断言显示业务文案都有"。根治依赖登录链路首请求重试（同 MAJOR-51-01）。

### 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-50-01（CI 重跑缓存语义漏洞） | MAJOR | **F50-M2 已修复闭合**（两条命令统一 -count=1 验证通过）；遗留 flake 本体见 MAJOR-51-01。 | **已修复，升级为本轮 MAJOR-51-01（flaky 残余）** |
| MINOR-50-01（writeJSONStatus family body code 开裂） | MINOR | **F50-M1 已修复闭合**（908 行 body code 1→500）；唯一残余 = failingTargetsStore 无消费点 + 500 分支无真红绿（仅实现内复查背书）。 | **已修复，残余观察延续** |
| OBSERVE-50-01（flake 伪装业务断言） | 观察 | 新样本实证（TestLoginUnactivatedNeedsCode 断言 357 行收到 connectex 业务文案）。 | 延续观察 |
| MAJOR-49-01 httptest flake | MAJOR | -p 1 + -count=1 下 6 轮 2 度 FAIL（同根 connectex）、单跑全绿 —— flake 本体未根除。 | 延续观察（并入 MAJOR-51-01） |
| OBSERVE-49-01 main 优雅退出 | 观察 | 全仓 grep 仍无 signal.Notify；sched.Stop/sessions.Close 依赖 defer；SIGTERM 时飞行 spawnChain success 可能未落库（SQLite WAL crash-safe）。 | 延续观察 |
| OBSERVE-49-02 chains map 无泄漏 | 观察 | 生命周期严格配对 + cap=账号×发布有界。 | 延续观察 |
| OBSERVE-49-03 rateLimited×done/full | 观察 | MarkDone 清 rateLimited+full；SetTargetsForAccount 刻意保留（B19-02）；读侧同锁。 | 延续观察 |
| OBSERVE-49-04 gate 窗口翻转 | 观察 | 共享 gateMu 串行、窗口重置锁内；每窗口 doLogin≤2；gateTryAcquire 非阻塞优先。ResetGateForTest 在 api 夹具多账号注册前调用、不干扰计数。 | 延续观察 |
| OBSERVE-49-05 task_log 无清理 | 观察 | 全仓 grep 仍无 DELETE FROM task_log；AppendLog 8+ 写点无条件 INSERT。 | 延续观察 |
| OBSERVE-49-06 大响应解析 | 观察 | 2× 响应体峰值内存 + 零值缺省容错 + 空快照提前返回。 | 延续观察 |
| OBSERVE-48-01 submitAll 300ms 拍 | 观察 | spinChain 链内串行不受节流；黄金期 250ms 间隔 + tick 300ms 拍。 | 延续观察 |
| OBSERVE-48-02 reloginResults cap 8 | 观察 | 非阻塞 select default；满丢弃仅并发重登爆炸 >8。 | 延续观察 |
| OBSERVE-47-01 task_log 无清理 | 观察 | 同 49-05。 | 延续观察 |
| OBSERVE-47-02 B43-05 500 分支无真断言 | 观察 | failingTargetsStore 仍无消费点；500 分支走实现内复查。F50-M1 已把 body code 对齐 500（不再有 MINOR 持有）。 | 延续观察 |
| OBSERVE-47-03 XUANKE_PORT 无校验 | 观察 | `:abc` → ListenAndServe log.Fatalf 拒绝启动；失败形态清晰。 | 延续观察 |
| OBSERVE-47-04 孤儿登录 | 观察 | Relogin 锁内取指针锁外 Login；删号+在途重登并发平台侧一次孤儿登录，频率极低无污染。 | 延续观察 |
| OBSERVE-46-02 Retry-After 不消费 | 观察 | isRateLimitError 文案匹配固定 30s；平台无 429 头样本。 | 延续观察 |
| OBSERVE-46-04 目标不校验重复/跨年级 | 观察 | byPub 分组 + 快照复核 + 平台把关。 | 延续观察 |
| OBSERVE-45-01 access_limit_cookie 占位 | 观察 | probe/main.go:24 与 submitLogin 同款占位宽度 26 字符；token 权威通道 URL 参数。 | 延续观察 |
| OBSERVE-43-01 probe 双槽分叉 | 观察 | 主体 `["*"]` + per-account `[acct]`；优先级 [acct]→[*]；全校单值不实际分叉。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 网络段重复 | 观察 | IsClassFull 恒 false（maxCount 未下发）；快照判满主路径。 | 延续观察 |
| OBSERVE-43-04 tick 无 recover | 观察 | tick/probe/spawnChain 均无 recover；interval clamp 封堵最可达 panic。 | 延续观察 |
| MINOR-43-02 syncFailedWindow 死字段 | 观察 | 写（382/399）不读，注释载明留档语义。 | 延续观察 |
| MINOR-43-03 登录失败无固定延迟 | 观察 | 学生走网络往返天然延迟；管理员错误口令分支 Sleep 保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释 | 观察 | n=1 返 30s、n=2 返 60s；封顶 5 防溢出（30s<<5=960s>10m）。 | 延续观察 |
| M40-01 / m40-02 / o40-01~04 / m39-02 / o39-02~05 | 观察 | 全部延续（未激活不发会话 / reloginResults 满丢弃 / columnExists 全常量 / interval clamp / SubmitAllowed 钉子 / submitAll 快照竞态 / vision key 空串不清空 / NAT 合并限流 / tick 无 recover / task_log 无清理）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **acctData 快照 × 窗口关闭态（是否被继续读取/空发布透传）**：WindowClosed 后 acctData[acct] 仍驻留（PurgeAccount 才删），ElectivesSnapshotFor/classFullInSnapshot/releaseFullIfFreedLocked/CheckClassSelectable 继续按锁读它——空发布快照下 classFullInSnapshot 查不到课程返回 false（不误判满员）、releaseFullIfFreedLocked 遇空 publishes 保持 full 不解封（C-3 正确）、CheckClassSelectable 放行交平台把关（与"关闭后空快照"契约一致）。**无问题**。
- **handleState 窗口关闭后的 Courses 构造**：Courses 全部来自 rebuildCoursesForAccountLocked 按 targets 重建，依赖持久化 publish_id/publish_name/begin_date，与 /electives 空发布零依赖（关闭≠时间消失契约）；StateForAccount 只过滤不重建。**无问题**。
- **reloginFail 的恢复与封顶**：maybeRelogin 决策侧 `if < maxReloginFail {++}`（1228）封顶 5；成功分支 delete 清零（1256）+ MarkTokenValid 手动登录 delete 清零（1313）；失败保留递增次数（C1 契约）——封顶语义正确、无溢出；reloginBackoff(maxReloginFail)=30s<<4=480s、溢出踩 backoffMax 10min。**无问题**。
- **targets 表 priority 全量替换事务性**：store.SetTargetsForAccount 先 DELETE 后循环 INSERT 包在单事务（Begin/Commit/Rollback）+ sort.SliceStable 按 priority；api handleSetTargets 先 Store 落库成功才调 Sched.SetTargetsForAccount（内存→库顺序，异常不半生效）。**无问题**。
- **findElectivesData 空 body 首探 vs 显式 schoolYear**：FindElectives 空 POST 首探 → parse 失败才回退 YearTerms+form 兜底；parseElectives 对 code:0 空 publishes 直接返回空快照"绝不落兜底"（577-583 注释明示），与逆向契约（空 body 平台自动返回当前激活学期）一致。**无问题**。
- **SaveSettings 全量写后 restart 字段完整性**：handleAdminConfig PUT 六个键全部覆盖写入（activation_enabled/vision_base_url/vision_key/vision_model/captcha_engine/captcha_concurrency），SaveSettings 单事务全量替换；restart 恢复按 LoadSettings 六键逐一 parse（main.go 78-108），缺键回落 runtime.New 初始值，.env 模板与实际键名一致。**无问题**。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查**：`go build ./...`、`go vet ./...` 双通道 exit=0。
- **测试**：6 轮 `-race -count=1`（-p 1 全量 ×3 + 组合 ×1 + api ×2），3 轮全量全绿 + 2 次 api 单包 FAIL（connectex 同根）+ 1 次组合 FAIL；失败测试单跑四轮全绿；无 race 报告。F50 钉子集锁定在库。
- **身份复核族**：spawnChain 六分支 sameClientFor + maybeRelogin 决策/写回侧闭合。
- **WindowClosed 三判据单源**：windowClosedLocked + StateForAccount 镜像 + handleAdminStats window_closed 同源（B29-02）。
- **doLogin 闸门全收口**：gateTryAcquire/gateWait 共享 gateMu；api 夹具 ResetGateForTest 多账号注册前置不干扰计数。
- **鉴权与数据安全**：writeJSONStatus 家族真实状态码（401/403/429/500）；SpaHandler /api 前缀 404（含精确 /api）；XFF 仅回环+开关；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝 + master_key 32 字节双重校验。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量拼接；migrateAddPublishMeta 增量幂等 + refuseLegacy 缺列清单对应剔除（priority/allow_swap/account 拒绝、publish_name/begin_date 迁移）。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 / priorityId 丢弃语义 / captcha Limiter Cond 动态热收敛 / 重试收敛 ≤3 / code=-1 统一 ErrUnauthorized / cookie+idToken 双通道与逆向契约一致。
- **session 层**：12h TTL + 5min 清扫 + sweeperClose sync.Once + ConsumeTicket 锁内四查。
- **契约 20（代码注释无轮次前缀标签）**：产品代码 0 残留；测试文件仍有 ~17 处"第 N 轮/第 N 轮 MAJOR"历史注释（如 refused_test.go:71 等），属测试注释历史记录，非产品代码标签，未强制清除。
- **前端契约**：client.ts 只读 body code、HTTP 仅特判 401——F50-M1/M2 的 HTTP 状态码增强无回归。

---

## 结论

- **MAJOR 1（httptest flake 本体未根除，F50-M2 闭合后续延）/ OBSERVE 1 新样本（flake 伪装业务断言）+ 延续 27+ 项；R50 两项修复（F50-M1/M2）全部闭合无回归**。
- 最重 3 条（按影响排序）：
  1. **MAJOR-51-01（httptest mock 连接 flake 本体仍在）**——F50-M2 已根治缓存羽化漏洞（两条命令统一 `-count=1` 验证通过），但 -p 1 下 6 轮测试仍 2 度误红（api 包 connectex 同根），失败测试单跑四轮全绿；CI `||` 重跑能吸收但产生低频假红噪音。根治仍建议"登录链路测试夹具首请求失败重试一次或 mock server 就绪探测"。
  2. **OBSERVE-51-01（flake 伪装业务断言新样本）**——TestLoginUnactivatedNeedsCode 断言行 357 实测收到 `登录失败: 初始化登录会话失败: …connectex` 业务文案（mock 连接失败经 LoginByPassword err 翻译），与 R50 OBSERVE-50-01 同根，佐证根治建议同时消两种形态。
  3. **观察延续最高优先残余**：task_log 无清理（50 轮未动，黄金期日志风暴）、tick 无 recover + 优雅退出缺口（SIGTERM 在飞 success 可能未落库）。
- R50 全部修复项（F50-M1 stats body code 500 对齐 + F50-M2 CI -count 统一）复核无回归，MINOR-50-01 随 F50-M1 关闭。上轮观察项 27+ 条全部复核：MAJOR-50-01 已修复（升级为 flaky 残余），MINOR-50-01 已修复（failingTargetsStore 无消费点残余观察延续），其余延续无升级。
- 残余风险集中在：CI flake 假红噪音（51-01）、task_log 长期无清理（49-05/47-01 观察延续）、孤儿登录平台侧副作用（42-02）、tick 无 recover（43-04）+ 优雅退出（49-01）。
