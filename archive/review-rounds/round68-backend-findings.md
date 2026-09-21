# R68 后端只读审查发现报告

> 审查基线：master @ `96e0f50`（R67 收官 docs 提交）。本轮重点为 R67 两处测试夹具修复（commit `33722d3`）逐行复核 + zhidao 栅栏配平后的 flake 残余面独立复跑 + 历轮决策手册闭合抽查 + 全包新缺陷地毯式搜索 + 跨轮修复闭合抽查。
> 范围：backend/ 全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_{unix,windows}.go），含全部测试文件。绝对只读，临时验证程序放 %TEMP% 跑完即删。
> 方法：R67 两处修复 diff 逐行核证 + 全量 `-race -count=1 -p 1 -timeout 900s ./...` **3 轮** + zhidao/accounts/api 隔离复跑 2 轮 + build/vet/gofmt 复检 + 决策手册契约抽查（7 条）+ 全包逐行通读 + %TEMP% 独立程序验证 doRequest URL 拼接。

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3（均为测试夹具/注释/文档层面）**。产品逻辑零 MINOR 连续第六轮（R63 起）。
- **R67 OBSERVE-67-01 修复成立：zhidao 包 readyProbe 已对齐 api 宽栅栏（200ms×10 + 显式 2s 超时），3 轮全量 race 全部 PASS，zhidao 包残余面归零**。曾 FAIL 的 `TestLoginLogsFailureSummary` 单独复跑 PASS（0.03s）。残余面未流动——本轮 10 包 3 轮共 30 次包运行零 FAIL，残余面暂歇（非根除，见 flake 统计节）。
- **R67 OBSERVE-67-02 修复成立：`TestLocalDdddOcrAvailable` 已补非零验证力断言**，本机实测 PASS（python 3.12 + ddddocr 可导入 → available=true → 断言通过）。同时核证该修复**不会在 CI（Linux ubuntu-latest）上误红**——CI 容器不装 python，`exec.LookPath` 失败走 Skip；装 python 无 ddddocr 的部署机上 available=false 且断言红属正确语义（该函数在那种环境本应返回 false）。
- **残余面继 OBSERVE-67-01 收敛后最低收敛点 = accounts 包 readyProbe（200ms×5、http.Get 无显式超时）**——本轮 accounts 6+ 次全绿，无实证样本，登记为 OBSERVE-68-01（与 zhidao 修复前同构的最窄栅栏，建议下轮顺手配平，非必须）。
- 决策手册契约抽查 7 条**全部成立**。跨轮修复闭合抽查（httpDo 仅 dial/write 重试 / IsReadErr 四形态消费点 / cmd/probe SharedTransport / db 平台注释 / recoverMiddleware 防御）**全部闭合**。

---

## R67 夹具修复复核（commit 33722d3 逐行核证）

### 1. zhidao readyProbe 宽栅栏（client_test.go:82-113）—— 正确

核实结果：**正确**。逐项核对：

- **探测循环逻辑**：`for i := 0; i <= probeRetries; i++` 共 11 次尝试（初始 1 次 + 10 次重试），`NewRequest` 每次循环重建（无请求体复用污染），成功分支 `io.Copy(io.Discard, resp.Body); resp.Body.Close()` 读空并关闭（body 不泄漏），失败分支 `lastErr = err` 覆盖（最后一条错误报给 Fatal），`if i < probeRetries` 才 sleep——第 11 次失败后不再 sleep 直接 Fatal。逻辑与 api 包 readyProbe（handler_test.go:184-215）逐字段对齐。
- **总窗口**：200ms × 10 + 2s 显式超时 = ~2s 轮询窗口 + 单请求 2s 上限，确实覆盖 store 包 59s 高耗时（本轮实测 store 28~53s）末尾的 TIME_WAIT 排空窗口。注释中"总窗口 ~2s"表述准确。
- **/login 路径 mock 返回非 200 也视为成功**：mock handler（:55-75）对 /login 恒写 200 "ok"，与 readyProbe 无关；readyProbe 本身**读 body 不管状态码**（`err == nil` 即成功返回）——与原实现（http.Get + err==nil）语义完全一致，未改变探测语义。该语义对就绪探测正确：探测只关心"accept 是否就绪"，HTTP 状态码无关。
- **新隐患**：无。显式 `&http.Client{Timeout: 2s}` 比原 `http.Get`（DefaultClient 无超时）更安全——mock 极端挂起时不再无限阻塞。io.Copy 错误未检查属就绪探测可接受（只是排空 body）。
- **注释准确性**：:84-87 注释准确引用 R67 OBSERVE-67-01 样本。唯一残留——**:47 `loginMockServer` 上方注释仍写"与 api 包 readyProbe 同款轮询：200ms×5，总窗口 ~1s"**（已过时，现为 200ms×10+2s），见 OBSERVE-68-02。

### 2. local_ocr_test.go 验证力断言 —— 正确

核实结果：**正确**。逐项核对：

- **新断言逻辑**：先测 `LocalDdddOcrAvailable("")`，`exec.LookPath("python")` 失败即 Skip；python 存在时 `!available` 即 `t.Error`。本开发机 python 3.12.8 + ddddocr 可导入 → available=true → 断言不触发 → PASS（实测 `go test -run TestLocalDdddOcrAvailable ./internal/zhidao/` PASS）。验证力：available=false 且 python 存在时测试必红——`LocalDdddOcrAvailable` 在"装了 python 无 ddddocr"环境返回 false 是正确的回退语义，该断言红即是正确语义被正确表达。
- **R67 任务书担忧"CI 若装了 python 但无 ddddocr → 新断言红"**：**不存在该问题**。CI 用 `ubuntu-latest`（ci.yml 第 8-13 步），GitHub 托管 Ubuntu 容器不预装 python（且 `actions/setup-python` 未在 CI 中使用），`exec.LookPath("python")` 失败 → Skip，断言不触发。若未来 CI 装 python，则该断言红在"无 ddddocr 的部署环境"语义下是**正确**的——函数在那种环境本就应返回 false（本地 ddddocr 引擎不可用，回退 Vision），断言红钉死正确行为。无需修改。
- **断言消息含"（开发机已装 ddddocr）或 false（无该包时的回退语义）"的"或"表述**：语义上略松散（available 在 python 存在时只有 true/false 二值，true 时通过、false 时 t.Error 总会红），但断言方向正确（任一二值都有验证力），非缺陷。
- **潜在微瑕（登记不修）**：断言消息把"false（无该包时的回退语义）"写成似乎"允许"的表述，实际 `!available` 无条件 `t.Error`——开发机上无 ddddocr 会红。但本开发机有 ddddocr，且该红语义正确，不影响。

---

## flake 统计（R68 全量 3 轮 + 隔离复跑 2 轮）

| 轮次 | 结果 | 各包耗时 |
|---|---|---|
| 全量 R1 | **ok** | accounts 1.5s / api 20.8s / config 1.6s / db 2.9s / runtime 1.6s / scheduler 14.6s / secure 1.6s / session 1.6s / store 52.9s / zhidao 2.5s |
| 全量 R2 | **ok** | accounts 2.0s / api 18.1s / store 39.5s / zhidao 4.2s / 全绿 |
| 全量 R3 | **ok** | accounts 1.8s / api 25.5s / store 28.3s / zhidao 3.6s / 全绿 |
| 隔离复跑 zhidao+accounts+api R1 | **ok** | zhidao 2.7s / accounts 1.8s / api 19.1s |
| 隔离复跑 zhidao+accounts+api R2 | **ok** | zhidao 2.5s / accounts 1.4s / api 46.1s |
| 单独复跑 TestLoginLogsFailureSummary | **ok** | 0.03s（R67 曾 FAIL 的测试） |
| 单独复跑 TestLocalDdddOcrAvailable | **ok** | 2.04s |
| store TestLoadLogsWindowKeepsRecent 单独 | **ok** | 33.37s（窗口批插 30050 行） |

- **结论：zhidao 栅栏配平后残余面归零**。R67 R1 的 `TestLoginLogsFailureSummary` readyProbe 5 次全败样本本轮 3 轮全量 + 单独复跑零再现。**残余面未流动到下一宿主**——accounts（下一最窄栅栏 200ms×5）本轮 6+ 次运行全绿，api（包序在 store 前）全绿。
- **残余面现状**：残余面是"包序最末 mock 包 + 最窄 readyProbe 栅栏"的函数。本轮 store 高耗时（28~53s，波动大）结束后接 zhidao（宽栅栏 2s）全绿。accounts 包仍在 store **之前**（包名字母序 accounts < api < ... < store < zhidao），故最恶劣时刻（store 之后）只轮到 zhidao——**accounts 的窄栅栏目前被包序天然保护**。若未来包序变化（新增 z 开头包）或 zhidao 移到 store 前，accounts 窄栅栏暴露。这是 OBSERVE-68-01 登记理由。
- **残余面历史**：R56 12 轮 zhidao FAIL → R64 单命中 → R67 单命中 → **R68 3 轮全量 + 2 轮隔离全绿**。"残余面根除"幻觉仍须警惕——3 轮全绿是样本收敛，非概率为 0（R67 也是 5 轮 1 命中）。CI `||` 重跑 + 宽栅栏是永久防线。
- build/vet/gofmt：`go build ./...` / `go vet ./...` / `gofmt -l .` 全部零输出。

---

## 契约抽查（7 条，全部成立）

1. **契约 2 窗口关闭三判据单源**（scheduler.go:914-935 windowClosedLocked）：主判据 state.WindowClosed（probe:1142 带 +10s 裕量 + prevOpened 前提）/ syncFailStreak≥3 且开放时间非零且已过 / EmptyProbeRuns≥3 且从未开窗且开放时间非零且已过——三条判据用单次 open 快照（:920 `s.openTimeForLocked("")` 取一次复用）。StateForAccount（:708）与 WindowClosed()（:903）共用同一实现。**成立**。
2. **契约 4 删账号 memory-first 四步**（handler.go:1005-1019）：Accounts.Remove → Sched.PurgeAccount → Store.DeleteAccount → Sessions.RevokeAccount。注释（:997-1004）明确说明 Remove 前置让在飞链锁内复核立即失败。**成立**。
3. **契约 21/31/37 sameClientFor 六分支**（scheduler.go:209-228 + spawnChain:1478-1664）：成功（:1517）/ 失效（:1485）/ 风控（:1547）/ 窗口关闭（:1567）/ 实时确证满员（:1631）/ 实时失效（:1596）六条 err 归并路径全部在写状态/落库前 `sameClientFor` 指针身份比对。`clientIdentity` 用 reflect.ValueOf(c).Pointer() 取指针、nil/非指针返回 0。**成立**。
4. **契约 33 B42-01 doLogin 全入口收口**：gateTryAcquire（manager.go:223-235）非阻塞准入 + LoginByPassword 前置（:244）+ 与 gateWait 共享 gateMu/gateUsed + ResetGateForTest 测试专用（api 夹具 authenticateDirect 三处调用，handler_test.go:363-367）。**成立**。
5. **契约 5/17 实时复核满员 doneHas 优先 + 落库失败必记日志**：实时复核满员分支先 doneHas（scheduler.go:1623）绝不覆盖手动胜利状态；全仓落库点零 `_ =` 吞错（scheduler/store/api 全部 `if err != nil { log.Printf }`；api 的 `_ = d.Sched.MarkDone/RemoveDone` 是函数内已记日志的幂等状态同步，非吞错）。**成立**。
6. **契约 40 IsReadErr 四形态消费点全路径**：定义 client.go:519-548（RST read / FIN io.EOF / 短读 ErrUnexpectedEOF / 超时两文案），消费点三处——scheduler.go:1652（自动链失败文案区分）、handler.go:356/423（手动报名/退选文案区分）。**全路径闭合**。
7. **契约 3 关闭≠时间消失 + 契约 1 识别槽保留**：probe 空快照不删 openTimeDetected（:1102-1106 只在下发非空 beginTimes 时覆盖）；StateForAccount 过期识别值只影响 open_time_known 展示（:719），绝不截断识别槽。**成立**。

---

## 新发现

### OBSERVE-68-01：accounts 包 readyProbe 栅栏仍窄（200ms×5 无显式超时），是 OBSERVE-67-01 收敛后残余面最低收敛点

- 位置：`backend/internal/accounts/manager_test.go:23-43`（三处调用 :79/:156/:239）。
- 一句话问题：zhidao 包已配平 api 宽栅栏，但 accounts 包 readyProbe 仍是 `http.Get` + `200ms×5`（总窗口 ~1s、无显式超时）——与 zhidao 修复前逐字同构的最窄栅栏。
- 影响面：测试夹具层面（产品零影响）。本轮 accounts 包 6+ 次运行全绿（全量 3 轮 + 隔离 2 轮 + 单独 1 次），**无实证样本**。当前被包序天然保护（字母序 accounts < store，store 高耗时之后接的是 zhidao 而非 accounts）；若包序变化或未来残余面流动，accounts 窄栅栏将暴露。
- 机制说明：与 R67 的机制完全同构——Windows 回环 TIME_WAIT 冷启动窗口在 store 包高耗时（28~53s）末尾最恶劣，1s 轮询窗口不足以覆盖重链表排空。宽栅栏（200ms×10+2s）已被 api/zhidao 双实证为收敛点。
- 建议：下轮顺手把 accounts 包 readyProbe 对齐 api/zhidao（200ms×10 + 显式 2s 超时 + NewRequest 每次重建），一次性配平三包栅栏，杜绝包间栅栏宽度差异成为残余面流动通道。低优、可续。

### OBSERVE-68-02：zhidao `loginMockServer` 注释残留过时栅栏参数（200ms×5 → 已改 200ms×10）

- 位置：`backend/internal/zhidao/client_test.go:47`。
- 一句话问题：`loginMockServer` 上方注释仍写"与 api 包 readyProbe 同款轮询：200ms×5，总窗口 ~1s"，而 readyProbe 已在 commit 33722d3 改为 200ms×10+2s 并删除了该函数内部的旧注释——:47 是新注释的遗漏残留，维护者误读夹具参数时会与真实窗口（~2s）产生偏差。
- 影响面：纯注释准确性（零功能影响）。R67 修复只更新了 readyProbe 函数自身的注释（:82-87），未同步 :47 调用点注释。
- 机制说明：commit 33722d3 diff 中 :47 注释未在修改范围内，属改动遗漏。
- 建议：把 :47 注释改为"与 api 包 readyProbe 同款轮询：200ms×10 + 显式 2s 超时，总窗口 ~2s"。一行注释，下轮顺手。

### OBSERVE-68-03：store 测试注释"实测 30050 行 ~225s"已过时（实测 33s）

- 位置：`backend/internal/store/store_test.go:32`。
- 一句话问题：注释写"逐条 INSERT 让每次提交都触发 WAL 同步落盘（实测 30050 行 ~225s，击穿 go test 默认 10m 超时）；批事务实测 ~0.5s"——本机实测批事务版本 TestLoadLogsWindowKeepsRecent 单独跑 **33.37s**（含 -race），非注释所述 ~0.5s；全量轮 store 包 28~53s。
- 影响面：纯注释数值过时（零功能影响）。注释的"逐条 INSERT 225s 击穿超时"是历史参照，批事务 ~0.5s 的断言与实测（33s）不符——可能写于开发机非 -race 或旧 SQLite 版本，或批事务单次 Commit 的 WAL fsync 在高并发插入下比预估慢。
- 机制说明：33s 差异不影响任何语义（窗口裁剪逻辑已由断言验证），仅"~0.5s"的量化参照过时。
- 建议：改为"批事务实测 ~30s（-race 下），远低于逐条提交的 ~225s"或删除量化值。一行注释，可续。

---

## 历轮观察延续

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-67-01（zhidao readyProbe 窄栅栏） | 续（建议对齐 api 宽栅栏） | 已修（33722d3 配平 200ms×10+2s），3 轮全量 + 单独复跑全绿，残余面归零 | **闭合** |
| OBSERVE-67-02（TestLocalDdddOcrAvailable 恒绿死） | 续（低优补断言） | 已修（33722d3 补非零验证力断言），本机实测 PASS，CI 无 python 走 Skip 无误红 | **闭合** |
| OBSERVE-66-01（recoverMiddleware 双层响应防御） | 登记防御 | 全仓仍无该类现实路径（handler 全单次 writeJSON/writeJSONStatus，SpaHandler 无 panic 后写路径） | **延续（登记防御）** |
| OBSERVE-66-02（SetVision 末端 else 兜底） | 登记闭合 | TestSetVisionKeepsLocalRecognizer（client_test.go:241-257）+ TestSetVisionRebuildsWhenCurrentIsVisionOrNil（:268-281）双护栏在位 | **闭合（延续登记）** |
| OBSERVE-65-01（db 平台注释） | 已修 | settings_test.go:12-17 注释与 Linux 建库机制逐段一致，跨平台红白语义明确 | **闭合** |
| OBSERVE-64-01（probeIntervalFor 注释 5s→2s） | 已修 | 全仓无"临门 5s"残留（nearWindow=5 分钟为独立常量语义） | **闭合** |
| OBSERVE-63-01（api ReadErr/UnauthorizedRelogin 冷启动） | 已闭合 | 本轮 api 包 6+ 次全绿零命中 | **延续（消失）** |
| OBSERVE-63-03（api 包最脆弱） | 已更新（zhidao 接棒） | 残余面自 R67 后归零，当前 30 次包运行零 FAIL——"最脆弱包"标签继续不固定 | **更新（暂歇）** |
| OBSERVE-63-04（probe 非可用工具） | 延续 | 未变（token 需手动注入 + SharedTransport 已对齐契约） | **延续** |
| OBSERVE-63-05（AdminStats 半真测试） | 延续 | failingTargetsStore（handler_test.go:898-905）定义未用（Register 收 *store.Store 具体类型，注释 :883-888 明确说明不改造接口）；TestAdminStatsTargetsLoadFailureReturns500 已改为最小契约回归（正常路径 200） | **延续（已自我注释化）** |
| OBSERVE-62-06/07、OBSERVE-61-03/04/06/07/08 | 延续 | 均未变 | **延续** |

---

## 已核对无缺陷的高风险区域

- 调度器 tick 守卫族（零值守卫放行 WindowOpened 例外 B41-02 / windowClosedLocked 三判据单源 / 250ms 冲刺 / 提交节流对齐钟 / spawnChain 六分支身份防线 / relogin 决策与写回复核 + 指数退避封顶 10m / probeSem cap4 与 probing 单飞 / submitAll 链顶 ClientFor 过滤 + spawnChain 双重判定）。
- 认证鉴权（requireAdminSession/requireAuth + CSRF requireJSONBody 真实 403 + 管理员双条件 B43-04 + 登录/激活双限流桶 + clientIP 可信反代 XFF + 会话 12h TTL + 删号 RevokeAccount + /api 前缀 404 + SPA 兜底 /api 拒 404）。
- 凭据加密（AES-256-GCM + 32 字节主密钥 XUANKE_MASTER_KEY 或 .master_key 严格校验 + vision_key enc: 前缀 + maskKey 脱敏 + secureEncrypt 无加密器拒绝 + 激活票据单次防重放）。
- 数据库（migrateAddPublishMeta 先于 refuseLegacy + 缺列清单与已迁移列对齐 + 激活码单事务原子扣减 + 空库日志窗口语义 + 日志档①窗口 id>max-20000）。
- 登录链路（RSA PKCS1 + uniqueDeviceID + priorityId 空串 + fetchLoginPage 抖动自愈 + httpDo 仅 dial/write 重试 + IsReadErr 四形态与 isConnErrRetryable 互斥 + Vision 识别 3~5 位净化 + ddddocr 三引擎回退链）。
- 前端配置回显链（Admin.tsx:530 留空 = 不改 key、:529 脱敏回显、:513-550 save 在飞幂等 + refetch 真值回填 F7-02）。
- cmd 三工具（probe SharedTransport + logintest -limit 边界 + 40s 间隔 + bench -token 注入 + -n 边界）。

---

## 教训

1. **"栅栏配平"要一次性扫清全包**：R67 只配平了 zhidao（当时的残余宿主），accounts 的 readyProbe 仍是同构窄栅栏（200ms×5）——本轮虽被包序天然保护（store 高耗时之后接 zhidao 而非 accounts），但配平动作是"最低收敛点修补"而非"包间栅栏宽度差异根除"。下次任何包序变化/新增包都可能让残余面流动到 accounts。一次性把三个 mock 包（api/zhidao/accounts）全部对齐 200ms×10+2s 才是终态。
2. **注释更新要成族核对调用点**：R67 修改 readyProbe 函数时更新了函数自身注释，但漏了调用点 loginMockServer 上方的同款注释（:47 仍写 200ms×5）——同一概念的注释分布在两处时，改动只同步一处是常见遗漏。grep 注释关键词（如"readyProbe""200ms"）核对全部出现点是复查手段。
3. **量化注释要标注测量条件**：store 测试"批事务实测 ~0.5s"与 -race 实测 33s 差 60 倍——量化值若不标注是否含 -race/平台，会随时间漂移成误导。注释里保留数量级语义（"远低于逐条提交"）即可，精确值最好带上测量上下文。

## 发现摘要（给主控）

- **OBSERVE-68-01**：accounts 包 readyProbe 栅栏仍窄（200ms×5 无显式超时），是 R67 收敛后残余面最低收敛点；本轮 accounts 全绿（无实证样本），被包序天然保护。裁决：续（低优，下轮顺手对齐 200ms×10+2s 一次性配平三包）。
- **OBSERVE-68-02**：zhidao `loginMockServer` 注释（client_test.go:47）残留"200ms×5"过时参数（实际已 200ms×10）。裁决：续（一行注释，下轮顺手）。
- **OBSERVE-68-03**：store 测试注释（store_test.go:32）"批事务实测 ~0.5s"过时（-race 实测 33.37s）。裁决：续（量化值标注测量条件或删值）。
- **R67 两条 OBSERVE 修复均复核成立**：readyProbe 宽栅栏 + local_ocr 断言，3 轮全量 + 隔离复跑全绿，zhidao 残余面归零。
- 无 CRITICAL / MAJOR / MINOR。
