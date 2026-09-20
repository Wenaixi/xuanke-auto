# round50 后端审查原始发现

> 审查基线：master @ `7e89cfd`（R49 收敛落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动；审查期间工作区零改动，绝对只读，未修改/创建/删除任何文件——唯一新文件是本报告）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 + legacy/website-source 逆向契约 + round39~49 各轮发现与修复报告逐条复核。
> 方法：全包逐行通读 + 竞态/锁序逐状态序列推演 + 上轮观察项逐一核实 + 本轮新视角扫查（acctTargets×done/full/refused 交叉、courses 发布归属、Token() 旧值窗口、LoadDone 恢复序、SelectClass 重试幂等、AdminNameValue 空值）。
> 编译与静态检查：`go build ./...`、`go vet ./...` 双通道 exit=0。
> 测试实证：全量 `go test -race -count=1 -p 1 ./...` 连跑 4 轮——第 1/2/4 轮全绿 EXIT 0；**第 3 轮 FAIL（api 包 2 测：TestLoginAdminNameCollisionStudentCredential 报业务文案形态失败、TestRequireJSONBodyRejectsFormContentType 报 `connectex: A connection attempt failed`）**，两测试单跑/子集复跑全绿（`go test -race -count=1 -run TestLoginAdminNameCollisionStudentCredential` 1.7s 绿 / `-run TestRequireJSONBodyRejectsFormContentType` 绿）。详见 MAJOR-50-01。

---

## CRITICAL

（无本轮新增 CRITICAL。identity 复核全分支闭合；删号 memory-first 四步序无新裸写点；本轮新视角六项均确认无数据污染通道。）

---

## MAJOR

### MAJOR-50-01：F49-M1 CI 重跑防御自身含缓存语义漏洞——第一个 `go test`（无 `-count=1`）重度复用缓存，`||` 重跑捕获不到 flake（F50 延续/升级）

- **位置**：`.github/workflows/ci.yml:59`（`go test -p 1 -v ./... || go test -p 1 -v -count=1 ./...`）
- **一句话问题**：重跑链的两个命令**缓存键不同**（第一条不带 `-count=1`，第二条带）——`go test -race` 的缓存命中依赖完全相同的标志集合，`-count=1` 是缓存破坏键之一。CI 首跑无缓存时两条命令都真实跑（重跑机制按设计工作），但**任何"第二次及以后"的 CI 跑 / 本地手工复跑**第一条恒命中缓存（输出 `ok  (cached)`），flaky 测试根本不重跑；若首条命令真实执行为红，整个 `run:` 步骤的进程 exit code 来自第二条命令（`||` 语义），第二条又是独立完整跑、不依赖首条结果——**重跑与首跑是两条等价且不互为备份的完整测试**，flake 落在首条、第二条全绿时任务仍被标红，重跑机制实际吸收不到该次失败（它吸收的只是"落在第二条的 flake"）。实测（本机）：`go test ./internal/db/ && go test ./internal/db/` 第二条输出 `ok (cached)`——首条失败、重跑被缓存羽化的场景确实可达，重跑侧不可能为红吸收。
- **实证**：① 本轮 4 轮全量 `-race -count=1 -p 1`（= CI `-count=1` 支）仍有 1 轮 2 测 FAIL（见顶），说明 `-p 1` 下 flake 依旧（与 R49 MAJOR-49-01 结论一致，佛 `-count` 无关）；② 本地演示第二条 `(cached)`（见结论节底部）。**CI 一旦跑过一次并缓存，重跑防御即由"吸收一次 flake"退化为"重跑一次缓存命中的同名命令"，标记假红/假绿语义都变得不确定**。
- **裁决**：MAJOR（CI 基础设施级）升级确认并向 F49-M1 修复本身开火——建议两条命令都统一 `-count=1` 前缀（首条也强制不缓存），或改用单条 `go test -p 1 -count=1 ./... || continue` 语义，或引入 `t.Parallel` 面修复 flake 本体（登录链路测试首次 `client.Do` 加重试，Windows 回环连接瞬时拒绝即绕过）。**业务代码零改动需求**（本轮业务断言全部绿，两 flake 均为 httptest mock 连接建立失败的下游形态）。

---

## 本轮重点核对（上轮新契约，防回归）—— 3 条全部闭合（其中 1 条带家族残余见 MINOR-50-01）

### 1. F49-M1 CI 重跑语法正确但带缓存语义漏洞 —— 见 MAJOR-50-01
- YAML `||` 链、注释与命令语法均合法；重跑目标"吸收瞬时 flake"在**首次 CI 跑场景**成立，但 `-count=1` 与缺省混用使其对"二次及以后"跑完全失效（详上）。

### 2. F48-O3 config 家族整风 — 正确闭合，但 B43-05 的 908 行 500 分支 body code=1 与家族新约定（body code=500）不一致（见 MINOR-50-01）
- 落库失败分支 `writeJSONStatus(w, 500, 500, ...)`（handler.go:810）+ `TestAdminConfigSaveFailStillDispatch`（handler_test.go:553-586）断言 `code != 500`/`j.code==500` 真红绿——上轮 47-02 的"500 分支无真断言"在此路径已闭合。
- 成功路径仍写 JSON（HTTP 200）+ 值域校验拒绝 + 无变更 PUT 三个行为均不变；B43-05 的 `failingTargetsStore` 仍无消费点（500 分支仍走实现复查背书，无真红绿钉）——观察项延续。

### 3. B45/B43/B44 + F46-O1 上轮实修 — 全部复核正确闭合无回归
- F46-O1 interval clamp（scheduler.go:237-239）+ TestScheduleIntervalClamped 真红绿；本轮钉子集（WindowOpenSubmitsWithoutProbeReset / SubmitSuspendedWhenOpenTimeCleared / SubmitAllowedWhenWindowOpenedWithZeroOpenTime / TestAdminStatsWindowOpenedUsesScheduler）锁定复跑全绿。
- B43-01 入口复核 + B39-01/B41-01/B43-02 六分支 sameClientFor 全闭合；身份族测试（TestDeletedAccountRebuiltSameNameChainDrops{Success,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull} + RealtimeUnauthorizedDropsRelogin + DropsInflight + maybeReloginSkipMaps）都在库绿。
- B45-N3 加密失败回归钉、B44-01 解密失败日志对称保留——accounts 包回归绿。

---

## MINOR

### MINOR-50-01：writeJSONStatus HTTP-500 家族 body code 开裂——F48-O3 落库失败分支 body=500，B43-05 stats 目标数失败分支 body=1（新视角，家族一致性残余）

- **位置**：`backend/internal/api/handler.go:810`（`writeJSONStatus(w, 500, 500, ...)`）vs `:908`（`writeJSONStatus(w, 500, 1, ...)`）
- **说明**：两处同写真实 HTTP 500（基础设施错误路径），但 body `code` 一个传 `500`（F48-O3 新约定"家族整风取与 HTTP 一致的语义码"）、一个传 `1`（B43-05 沿用旧 writeJSON 的"1=业务失败"约定）。前端 `client.ts` 只按 `j.code!==0` 抛错、`r.status` 仅特判 401，故**用户无感知**；但反代/脚本按 body code 归类时 `code=1` 会被当业务失败统计，同一"服务端持久化层错误"出现两种 body 编码——F48-O3 的"writeJSONStatus 家族统一"目标未覆盖 908 行。建议 908 行 body code 改 500（语义对齐）+ 补断言（failingTargetsStore 引入消费点）或明确注释该偏放电来自 B43-05 的旧约定。
- **裁决**：MINOR（非数据正确性，家族一致性/可观测性残余）。如修复代理认为 908 行 code=1 是刻意兼容（前端 1001/ticket 之外的业务错误都 code=1），可在注释载明后降级观察。

---

## OBSERVE

### OBSERVE-50-01：flake 形态可伪装业务断言失败——"管理口令错误"掩盖 mock 连接失败根因（本轮 flake 归因实证）
- **位置**：`handler_test.go:1453`（TestLoginAdminNameCollisionStudentCredential 失败断言装 `msg:管理口令错误`）
- **说明**：第 3 轮该测试断言收到的响应为 `{code:1, msg:"管理口令错误"}`——handleLogin 的管理员名分支把 **LoginByPassword 的任何 err**（含"初始化登录会话失败: Get http://…: connectex"）都翻译成"管理口令错误"（B43-04 语义），使底层 mock server 连接失败在失败断言上表现为业务文案。**与本轮同时出现的 TestRequireJSONBodyRejectsFormContentType 的 connectex 同根**——flaky 面从"断言显示连接错误"到"断言显示业务文案"都有，CI 重跑/人工误判真实回归的成本更高。观察级，佐证 MAJOR-50-01 修复建议（登录链路首请求重试）可同时消两种形态。
### OBSERVE-50-02 及其余延续（见下表）。

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MAJOR-49-01 httptest flake F49-M1 重跑已建 | MAJOR | **升级确认 + 指向修复本身**：`-race -count=1` 下 4 轮仍有 1 轮 2 测 FAIL（同为 mock 连接失败）、单跑全绿——flake 未根除；且重跑链 `-count` 混用使"二次及以后"重跑被缓存羽化（见 MAJOR-50-01）。 | **升级为 MAJOR-50-01** |
| OBSERVE-49-01 main 优雅退出 | 观察 | 全仓 grep 仍无 signal.Notify；sched.Stop/sessions.Close 依赖 defer。SIGTERM 时飞行 spawnChain 的 success 可能未落库（重报幂等），SQLite WAL crash-safe。 | 延续观察 |
| OBSERVE-49-02 chains map 无泄漏 | 观察 | 生命周期严格配对 + cap=账号×发布有界；身份族测试在库绿。 | 延续观察 |
| OBSERVE-49-03 rateLimited×done/full | 观察 | MarkDone 清 rateLimited+full（1941-1943）；SetTargetsForAccount 刻意保留（B19-02）；读侧同锁。 | 延续观察 |
| OBSERVE-49-04 gate 窗口翻转 | 观察 | 共享 gateMu 串行、窗口重置锁内临界区；每窗口 doLogin≤2。gateTryAcquire 非阻塞优先语义刻意。 | 延续观察 |
| OBSERVE-49-05 task_log 无清理 | 观察 | 全仓 grep 仍无 DELETE FROM task_log；AppendLog 8+ 写点无条件 INSERT；黄金期日志风暴量化保留。 | 延续观察 |
| OBSERVE-49-06 大响应解析 | 观察 | 2× 响应体峰值内存 + 零值缺省容错 + 空快照提前返回 + 解析失败兜底。 | 延续观察 |
| OBSERVE-48-01 submitAll 300ms 拍 | 观察 | spinChain 链内串行不受节流；黄金期 250ms 间隔 + tick 300ms 拍，首提交 ≤300ms。 | 延续观察 |
| OBSERVE-48-02 reloginResults cap 8 | 观察 | 非阻塞发送 select default；满丢弃仅在并发重登爆炸 >8。 | 延续观察 |
| OBSERVE-47-01 task_log 无清理 | 观察 | 同 49-05 延续。 | 延续观察 |
| OBSERVE-47-02 B43-05 500 分支无真断言 | 观察 | `failingTargetsStore` 仍无消费点；TestAdminStatsTargetsLoadFailureReturns500 仍只测正常路径。本轮发现其 body code=1 与 F48-O3 家族不齐（MINOR-50-01）。 | 延续观察 |
| OBSERVE-47-03 XUANKE_PORT 无校验 | 观察 | `:abc` → ListenAndServe `unknown port` log.Fatalf 拒绝启动；失败形态清晰。 | 延续观察 |
| OBSERVE-47-04 孤儿登录 | 观察 | Relogin 锁内取指针锁外 Login；删号+在途重登并发时平台侧一次孤儿登录，频率极低无污染。 | 延续观察 |
| OBSERVE-46-02 Retry-After 不消费 | 观察 | isRateLimitError 文案匹配 30s 固定退避；平台无 429 头样本。 | 延续观察 |
| OBSERVE-46-04 目标不校验重复/跨年级 | 观察 | byPub 分组 + 快照复核 + 平台把关；有意边界。 | 延续观察 |
| OBSERVE-45-01 access_limit_cookie 占位 | 观察 | probe/main.go:24 与 submitLogin 同款；token 权威通道 URL 参数。 | 延续观察 |
| OBSERVE-43-01 probe 双槽分叉 | 观察 | 主体 `["*"]` + per-account `[acct]`；优先级 [acct]→[*]；全校单值不实际分叉。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 网络段重复 | 观察 | IsClassFull 恒 false（maxCount 未下发）；快照判满主路径。 | 延续观察 |
| OBSERVE-43-04 tick 无 recover | 观察 | tick/probe/spawnChain 均无 recover；interval 已 clamp 封堵最可达 panic。 | 延续观察 |
| MINOR-43-02 syncFailedWindow 死字段 | 观察 | 写（382/399）不读，注释载明"留档语义"。 | 延续观察 |
| MINOR-43-03 登录失败无固定延迟 | 观察 | 学生走网络往返天然延迟；管理员错误口令分支 Sleep 保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释 | 观察 | n=1 返 30s、n=2 返 60s；注释一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录 | 观察 | 见 47-04。 | 延续观察 |
| M40-01 / m40-02 / o40-01~04 / m39-02 / o39-02~05 | 观察 | 全部延续（未激活不发会话 / reloginResults 满丢弃 / columnExists 全常量 / interval clamp / 快照 TTL 读侧本地钟 / submitAll 快照竞态 / vision key 空串无法清空 / NAT 合并限流 / tick 无 recover / task_log 无清理）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **acctTargets 内存与 done/full/refused 交叉**：SetTargetsForAccount 只清 refused（B19-02），done 残留跨目标持久历史正确（TestRestoreDoneSkipsResubmit 钉）；full 残留不影响展示（重建按目标生成 Courses 时命中 done 置"重启恢复"，full 只在 spawnChain 判跳，且下个快照 releaseFullIfFreed 自愈——仅当快照明确有余量才解封）。**无问题**。
- **courses 发布归属（publish_id 漂移）**：/state.courses 的 publish_id 来自目标持久化（store.targets.publish_id），前端按 publish_id 分组不依赖 /electives 空发布；enrichTargetPubMetaLocked 只补面（publish_name/begin_date），不移动分组归属。**无问题**。
- **accounts.Token() 强登后旧值窗口**：Login() 成功分支 SetCredentials 锁内同步写 c.token；Relogin goroutine 重登成功读 client.Token() 与进程内其他登录互斥于 client.mu——不存在"读到强登后旧值"窗口（写后立即读同锁串行化）。**无问题**。
- **LoadDone/LoadSuccess 恢复序**：main.go RestoreDone 先于逐账号 RestoreTargets（B10-01），task_log 由运行期 AppendLog 写、不参与恢复；refused 先于 SetTargetsForAccount 的注释（RestoreRefused 先注入）与 main.go 实际顺序一致（139 行 LoadRefused 在 120 行 RestoreTargets 循环之后、但 SetTargetsForAccount 不调，RestoreTargets 不清 refused——顺序契约成立）。**无问题**。
- **SelectClass 重试幂等**：平台兵行 code=1"选课处理中，请勿重复操作" → err → 非满员失败路径 → classFullRealtime（IsClassFull 恒 false）→ 置 failed + AppendLog → 下 tick 重打；平台对同班重复报名幂等（已报不可重报），最终一次会拿到成功确认并落 done——不会重复报名，仅多打几次（黄金期已由 full 退避/风控退避收敛）。**无问题**。
- **AdminNameValue 空值**：Deps.AdminNameValue 默认 "admin"；XUANKE_ADMIN_NAME 留空时管理员登录账号=admin；B43-04 撞名学生走教务分支正确；main.go adminNameOrDefault 日志对称。**无问题**。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查**：`go build ./...`、`go vet ./...` 双通道 exit=0。
- **测试**：4 轮 `-race -count=1 -p 1 ./...`，3 轮全绿 EXIT 0，第 3 轮 api 2 flake（TestLoginAdminNameCollisionStudentCredential 业务文案形态 + TestRequireJSONBodyRejectsFormContentType `connectex`，同根 = mock 连接建立失败；单跑全绿）。失败全部与 R49 MAJOR-49-01 同源——业务断言不涉及，无 race 报告。
- **identity 复核族**：spawnChain 成功/失效/风控/窗口关闭/实时复核（cErr 分支 + 满员分支）六分支 sameClientFor；maybeRelogin 决策侧 + 写回侧 + MarkDone/RemoveDone 写回侧闭合。TestScheduleIntervalClamped 等钉子集锁定在库。
- **WindowClosed 三判据单源** windowClosedLocked + StateForAccount + handleAdminStats 同源；开窗点 10s 裕量单快照复用。
- **doLogin 闸门全收口**：gateTryAcquire/gateWait 共享 gateMu 串行、无旁路；api 测试夹具 ResetGateForTest 每注册前重置不干扰计数。
- **鉴权与数据安全**：writeJSONStatus 四家族真实状态码（401/403/429/500）+ CSRF-403；SpaHandler /api 前缀 404；XFF 仅回环 + 开关；凭据 AES-256-GCM enc: 前缀 + 旧明文拒绝加载；master_key 32 字节校验。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量拼接；migrateAddPublishMeta 增量幂等 + refuseLegacy 缺列清单对应剔除。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 / uniqueDeviceID 复刻 / captcha 信号量（Cond 动态热收敛）/ 重试收敛 ≤3 / code=-1 统一 ErrUnauthorized / cookie+idToken 双通道与逆向契约一致。
- **session 层**：12h TTL + 5min 清扫 + sweeperClose sync.Once + ConsumeTicket 锁内四查。
- **前端契约**：client.ts 只读 body code、HTTP 仅特判 401——F48-O3/B43-05 的 HTTP 状态码为纯增强无回归。

---

## 结论

- **MAJOR 1（CI 重跑缓存语义漏洞 + flake 未根除，升级确认）/ MINOR 1（writeJSONStatus HTTP-500 家族 body code 开裂）/ OBSERVE 1 新（flake 伪装业务断言）+ 延续 27 项**。
- 最重 3 条（按影响排序）：
  1. **MAJOR-50-01（F49-M1 重跑防御缓存语义漏洞）**——`go test -p 1 -v ./... || go test -p 1 -v -count=1 ./...` 两条命令缓存键不同：首条（无 `-count=1`）恒命中缓存，重跑侧被相当于缓存羽化，二次 CI 跑起重跑机制实际失效；且本轮 4 轮 `-race -count=1` 仍有 2 flake（同根 mock 连接失败），flake 本体未根除。建议首条也加 `-count=1`（或改用单条 + continue 语义），并评估登录链路测试首请求重试作为根治。不改业务代码。
  2. **MINOR-50-01（writeJSONStatus HTTP-500 家族 body code 开裂）**——F48-O3 落库失败分支 body code=500，B43-05 stats 目标数失败 908 行 body code=1，同写真实 HTTP 500 但 body 编码两套；前端无感知（只读 body code!=0），运维/脚本按 body code 归类会误判。建议 908 行对齐 500 或注释載明刻意。
  3. **OBSERVE-50-01（flake 伪装业务断言）**——TestLoginAdminNameCollisionStudentCredential 把 mock 连接失败（"初始化登录会话失败: …connectex"）经 B43-04 管理员分支翻译成"管理口令错误"业务文案，使 flaky 失败观感为业务回归；佐证"登录链路首请求重试"修复同时消两种形态。
- 上轮观察项 27 条全部复核无升级，唯 MAJOR-49-01 升级为本轮 MAJOR-50-01。F48-O3 正确闭合（B43-05 的 body code 家族残余见 MINOR）。
- 残余风险集中在：CI 重跑机制准确性（50-01）、writeJSONStatus 家族 body code 一致性（50-01 MINOR）、task_log 长期无清理（49-05/47-01 观察延续）、孤儿登录平台侧副作用（42-02）、tick 无 recover（43-04）+ 优雅退出（49-01）。