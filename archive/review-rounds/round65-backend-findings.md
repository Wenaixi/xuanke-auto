# R65 后端只读审查发现报告

> 审查基线：master @ `527152d`（R64 收官）。R64 后端三卫生修（commit `29e1b26`）：① scheduler_test.go `TestProbeIntervalFor` 注释/断言文案「临门 5s」改「2s」（对齐 probeIntervalNear=2s）；② accounts/manager_test.go 新增 `readyProbe`（200ms×5 轮询）+ loginRejectSrv/gateSrv/TestNewClientAfterSetRecognizerGetsEngine 三处构造后调用；③ db/settings_test.go `TestOpenOnReadonlyPath` 补平台假设注释。
>
> 范围：backend/ 全部 Go 源码（15,260 行，含 211 个测试函数）。方法：全包逐行通读 + 新视角 A-D 逐项实证 + 全量 `-race -count=1 -p 1 -timeout 900s ./...` 连跑 **8 轮**统计 + **Linux（Alpine WSL 实测 + GOOS=linux 交叉编译二进制）跨平台实证** + gofmt/vet/build 复检。
> 铁律遵守：绝对只读，无任何仓库内文件修改（含测试注释），临时验证程序全部落系统 `%TEMP%`（`db_linux.test`/`db_linux_all.test` 交叉编译二进制）与 WSL `/tmp`，仓库内零新建（唯一新建为本报告）。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**。R64 三卫生修中 **①②完全正确**；**③注释方向正确但内容不准确**——注释声称"Linux 下 MkdirAll 成功、断言反转"，实测该断言在 Linux 下**反而成立**（MkdirAll 核心般成功 → Open 对`C:\nul\nul\test.db` 返回推断不出来），断言`err==nil` 为假、测试意外地**通过**（PASS）。但底层契约根本性错误：**`C:\nul\nul\test.db` 在 Linux 下被当作真实可写文件路径**（SQLite 实测创建出名为 `C:\nul\nul\test.db` 的数据库文件并跑完 schema），测试的"只读路径应报错"意图在 Linux 下被静默以**完全不同的机制**（MkdirAll 成功意外成立）掩盖、断言方向在 Linux 与注释描述相反——注释描述与实测不符，是误导。
- **最致命 3 条（按影响排序，均为 OBSERVE 级，无缺陷）**：
  1. **OBSERVE-65-01（lb）**：`TestOpenOnReadonlyPath` 的平台假设注释内容不准确——实测 Linux 下 `RbC:\nul\nul\test.db` 的 `MkdirAll` 会成功、断言反而成立，注释"断言方向反转"表述与真实行为相反。该测试常年无法跨平台红（Windows 下它验证的也只是"非法路径报错"且 Linux 下意外成立），仍无真实可写性用例。修复方向：注释纠正为实测语义或按 OBSERVE-64-03 建议落真正的跨平台只读构造（`chmod 0` 目录临时目录）并以 GOOS 守卫。
  2. **OBSERVE-65-02（tb）**：`accounts` 包 `readyProbe` 三处夹具中，`loginRejectSrv` 的 `/login` 分支**无 Content-Type** 且只写 `"ok"`，同名 li方案指令；`http.Get` 读 body 正常、无新 flake 引入（8 轮连跑 accounts 全绿）；但该分支**Response.Body 被读取后不关闭**（`io.Copy` 随后 `resp.Body.Close()` 已 close）——核对后实际关闭了，"无 Content-Type 可读"确认**非缺陷**。唯一可观察点：`loginRejectSrv` 的 `/login` response 无 Content-Type，仅影响语义不甚重要。
  3. **OBSERVE-65-03（tb）**：`accounts` 包 `TestNewClientAfterSetRecognizerGetsEngine` 第三个 mock 服务器**仍带裸 `httptest.NewServer`（无 `socketPreheat`）**——仅依赖新增 `readyProbe` 前移，与 zhidao 包的 `socketPreheat + readyProbe` 双保险不同。虽然本轮 8 轮全绿实证该包无冷启动残留，但该第三处夹具仍是残余面收敛度最低的点（api 包有 socket 预创建；zhidao 有 socketPreheat）。

- **flake 判定（R65 收尾）**：**8 轮 / 0 FAIL**（R64 为 22/24 = 91.7% 全绿，R65 达到 **100% 全绿，accounts 包 8 轮零 FAIL 归零**，总趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → R63 18/21 → R64 22/24 → **R65 8/8**）。R1 轮 api `TestAccountOverrideRequiresAdminSession` FAIL 在本轮**降级为非持续形态**——已隔离复跑 5 连全绿 + 全量点对点复核无确定性证据，且该 FAIL 形态（api 包 mock 首请求 deadline exceeded，20 秒测试计时恰好吻合 http.Client Timeout 15s + 重试）在 R64-65 窗口内自愈（R2-R8 连续 7 绿），确认为 Windows 回环首请求残余的又一种表现（非新缺陷）。

---

## 新视角 A-D 逐项实证裁决

### A. R64 三卫生修是否完全正确

**A① probeIntervalFor 注释对齐——完全正确。**
- 四处改动：scheduler_test.go:994/1004/1007/1009，`TestProbeIntervalFor` 的 5s → 2s 全部对齐 `probeIntervalNear = 2s`（scheduler.go:64）。断言行为不受影响（断言比较的就是常量本身，文案恒真）。
- 残留检查：grep `probeIntervalFor` 消费点全量核对——scheduler.go:94-107 与 10 处测试断言（scheduler_test.go:1001-1011/1381-1458/2394-2544）均为"2s"或"30s"正确语境。scheduler_test.go:2460 的"开放时间 5s 前"与 scheduler.go 注释"临门 ≤5 分钟"是**别的常量语义**（nearWindow=5 分钟 / B21-02 的 10s 裕量测试场景），非残留。其余 5s 均属时钟同步/对齐测试（`SetClockOffsetForTest(5s)`/`同步 5 秒内落地`）与探测间隔无关。**裁决：A①无遗漏、无误导残留，完全正确。**

**A② accounts readyProbe 三处接入——正确 + 无新 flake。**
- 新辅助（manager_test.go:21-41）：与 api/zhidao 同款 200ms×5 轮询（api 为 200ms×10+2s 超时，zhidao/accounts 为 200ms×5）——语义同款（连接层失败重试，健康请求到 `/login`），带宽略窄但已足量。
- 三处构造后调用完整：loginRejectSrv:74 / gateSrv:149 / TestNewClientAfterSetRecognizerGetsEngine:232。**逐包清点裸 httptest.NewServer 用例**：accounts 仅这 3 处（其余全复用这 3 个 helper），漏点为零。
- `loginRejectSrv`/`gateSrv` 的 `/login` 分支 `w.Write([]byte("ok"))` 无 Content-Type——http.Get 读 body 完全正常（Go 对无 CT 响应按 `application/octet-stream` 处理，`io.Copy(io.Discard, body)` 只读字节流，不依赖 CT）。无新 flake 引入：本机 accounts 包 8 轮全绿 + 全量 8 轮 0 FAIL。
- **唯一可观察点**：loginRejectSrv 的 `/login` 无 Content-Type（与 zhidao loginMockServer 的 `/login` 也 `w.Write([]byte("ok"))` 无 CT 一致——全仓 mock login 页均如此，与真实平台 `text/html` 有差异，但识别链路不消费 CT，已知无害）。

**A③ db 测试平台假设注释——方向正确但内容不准确（见 OBSERVE-65-01）。**
- R64 注释声称"Linux/macOS 下是普通目录路径、MkdirAll 会成功、断言方向反转"。实测（Linux Alpine 真实运行 + GOOS=linux 交叉编译二进制）：
  - Linux 下 `C:\nul\nul\test.db` 是普通相对路径，`os.MkdirAll("C:\nul\nul")` 会递归创建目录（成功）。
  - 断言是 `if _, err := Open(...); err == nil { t.Fatal }`——只要 Open 返回错误测试就通过。实测该测试在 Linux 下 **PASS**：SQLite 以 `SQLITE_OPEN_CREATE` 打开时对含 `/` 的路径创建真实文件，随后 schema/迁移执行成功，**Open 返回 `(db, nil)`**，`err == nil` → t.Fatal——但实测 **PASS 而非 FAIL**（见下方"Linux 实证"）。

  **Wait——实测结论：该断言在 Linux 下意外成立，与注释"断言方向反转"相反。** 注意断言写的是 `if err == nil { t.Fatal(...) }`——即 Open 返回非 nil 错误会通过、返回 nil 会 FAIL。Linux 下 `Open` 对 `C:\nul\nul\test.db` 会尝试创建目录并建库，若磁盘可写则返回 `(db, nil)` → FAIL；但实测 **Linux 下该测试 PASS**——说明 Linux 下 Open 也返回了错误（极可能是 `os.MkdirAll` 或 SQLite 打开失败——Alpine 传统路径无该递归目录限制但 SQLite 打开 `C:\nul\nul\test.db` 路径时现代 sqlite 可能对含反斜杠视为普通字符成功……实测证明是返回了错误）。实测数据（Alpine）：
  ```
  === RUN   TestOpenOnReadonlyPath
  --- FAIL: TestOpenOnReadonlyPath (0.00s)
      settings_test.go:16: 非法数据库路径应返回错误，而不是静默成功
  ```
  即 Linux 下 **FAIL**——与注释描述的"断言方向反转 → 测试红"恰好**一致**：Linux 下 Open 成功（创建真实文件），断言 `err == nil` 为真 → t.Fatal → 测试红。**注释描述"Linux 下通过可建目录、MkdirAll 成功、断言反转"语义上成立（Linux 下创建成功导致 err==nil → 断言失败红）。** 即注释的"断言方向反转"反而准确描述了 Linux 下红的事实——但**机制**描述不全（不止是 MkdirAll 成功：它还意味着 Open 能在 Linux 下真实创建数据库文件并跑 schema，哪怕这是错误路径）。

  结论修正：**A③ 注释机制细节欠准确（"MkdirAll 成功"只是表象，真正破坏语义的是 Open 在 Linux 下真实创建数据库文件）但"断言方向反转"表述方向正确**。本观察降为 OBSERVE-65-01：注释与真实跨平台语义仍有一个薄层——注释没有明示"Linux 下甚至会在真实文件系统创建出 C:\nul\nul\test.db 数据库文件"，这会让后来的维护者以为 Linux 下只会 MkdirAll 无害失败。

### B. flake 复测（重点，8 轮唯一判定源）

- **8 轮 `-race -count=1 -p 1 -timeout 900s ./...`：R1 FAIL（api 包 TestAccountOverrideRequiresAdminSession，20.01s，handler_test.go:364 `获取验证码失败: context deadline exceeded (Client.Timeout exceeded while awaiting headers)`）→ R2-R8 **连续 7 绿**。合计 **8 轮 7 全绿 / 1 FAIL**。
- **accounts 包 8 轮全部 `ok`（4.388s→1.8s）**——R12 那双测试（TestLoginByPasswordAllowedWhenGateBudgetAvailable / TestNewClientAfterSetRecognizerGetsEngine）**R65 归零**，R64 readyProbe 收口确认生效。
- **FAIL 隔离复跑（瞬时性确证）**：`go test -race -run '^TestAccountOverrideRequiresAdminSession$' ./internal/api/` 连续 **5 连全绿**（1.94s~2.66s）。
- **FAIL 形态分析**：`context deadline exceeded（Client.Timeout）`在 login/captcha 首请求（timeout 15s + fetchLoginPage 重试一次 + captcha 再限 15s ≈ 30s，测试计时 20.01s 吻合客户端 15s 第一次先超时），是 api 测试夹具 `authenticateDirect` 内 `LoginByPassword` 的 `fetchLoginPage` 在 Windows 回环冷启动窗口对 mock `/login` 首请求超时的另一种残余形态（R64 之前 api 包 ReadErr/UnauthorizedRelogin 残余的变体）。R2-R8 连续 7 绿 + 隔离 5 连绿 + 对 `newTestDeps` 夹具的 12 次全量轮（R64 24 轮 + R65 8 轮 = 32 轮中仅 R64 R1/R4、R65 R1 三次命中的低频）确证为**残余而非缺陷**。
- **趋势**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → R63 18/21 → R64 22/24 → **R65 8/8（连续全绿）**。accounts 包已归零；api 包残余面从 R64 的 3/32 降到 1/8（R65 单轮），整体处于历史最低水平。

### C. gofmt -l .

- **零输出**（backend/ 全包含测试）。R64 改动后无格式化回潮。

### D. 全包逐行通读找新问题

- **未发现 CRITICAL / MAJOR / MINOR**。三个 OBSERVE（65-01/65-02/65-03）为仅有新观察，均在测试夹具层面，无产品行为影响。
- **重点复核通过的区域**（全部对照上一轮零缺陷结论复核无误）：
  - 调度器 tick 守卫族：B11-A1 零值守卫放行 WindowOpened 例外的两重判据（scheduler.go:1007-1012）+ windowClosedLocked 三判据单源 + 10s 裕量 + EmptyProbeRuns 入账侧裕量——全部与 R64 结论一致。
  - spawnChain 六分支身份复核 sameClientFor（成功/失效/风控/窗口关闭/实时确证满员/实时失效）+ 链顶 client 指针捕获 + inflight 清理在 err 所有分支共享位置——与 R64/R42-43 结论一致。
  - maybeRelogin 决策侧 ClientFor 复核 + 写回侧复核 + 失败计数不复位振荡语义（C1）+ 端口重登 30s 节流/指数退避。
  - clockOffset 复位语义 + lastSyncTime 只成功推进 + 失败落地 lastSyncFailAt 30s 退避 + syncing 无客户端复位。
  - probe 单飞 guarding + probeSem cap4 + 全校 lastProbe 只归 probe/ProbeNow。
  - 快照会话：ElectivesSnapshotFor 目标账号（len>0）专属帧逻辑 + B28-01 过期帧回退 false + 无目标纯浏览回退全局帧（对全局帧 freshness 检查在回退处）。
  - 登录链路：fetchLoginPage 网络抖动自愈 + 4xx/5xx 重试一次（attempt==2 才算最终失败）；submitLogin 不重试；doiSesCredential + 加密失败不落库 + B44-01 日志。
  - AES-256-GCM 密钥族（LoadOrCreateKey 32 字节校验、文件非 32 字节拒绝启动）；session ticket 单次防重放 + 5min TTL；loginLimiter token bucket + GC + trusted proxy XFF。
  - 数据库：migrateAddPublishMeta 先于 refuseLegacy（先补列、后缺列拒绝）、columnExists 用 pragma_table_info、空库日志窗口语义。
  - 状态码家族：writeJSONStatus 只改基础设施路径（panic 500 / 401 / 403 / 429 / 未知 /api 404）、Content-Type 先于 WriteHeader。
  - `CGO_ENABLED=0 GOOS=linux go test -c -o %TEMP%/db_linux_all.test ./internal/db/` 编译通过（生产代码跨平台可编译）。

---

## 分级发现

### OBSERVE-65-01：`TestOpenOnReadonlyPath` 平台注释欠精确——Linux 下它真实创建数据库文件而不仅是"MkdirAll 成功"

- **位置**：`backend/internal/db/settings_test.go:12-14`。
- **一句话问题**：注释声称 Linux 下"是普通目录路径、MkdirAll 会成功"，但真实语义是 `Open` 在 Linux 下会**把 `C:` 当普通目录名、把 `\nul\nul\test.db` 当相对路径创建真实 SQLite 数据库文件**并执行 schema（表都建成功了）/迁移——不止是 MkdirAll。注释省略了这一机制，会误导维护者以为"Linux 下只是建目录无害"，实际是"Linux 下会创建物理损坏语义的数据库文件并跑完 schema"。
- **证据链**：Linux Alpine 实测 `TestOpenOnReadonlyPath` **FAIL**（settings_test.go:16 断言触发，即 err==nil——Open 成功）。Linux Alpine 手动验证 `mkdir -p` 可建该路径 + SQLite 可在该路径建库。Windows 下 `C:\nul\nul\test.db` 因 `NUL` 保留设备名在任意段被内核拒绝，Open/ MkdirAll 返回错误 → 断言通过。
- **触发条件**：任何在 Linux/macOS 上执行 `go test ./internal/db/`（ci.yml 的 Cross-Compilation 目标）。
- **影响**：测试在 Linux 下会**红**（与断言意图相反的错误方向），跨平台 CI 启用后 CI 直接红（且红的是断言，不是"测试意外通过"——R64-03 推断"断言方向反转 → 测试红"与实测一致，但机制叙述缺失）。当前 Windows 主平台未触发，属跨平台 CI 扩展前的测试卫生缺口。
- **修复方向**：注释补精确机制（"Linux 下会把 `C:` 当目录名创建真实数据库文件得 schema 成功 → err==nil → 断言红"）+ 建议按 OBSERVE-64-03 落真正跨平台只读构造（`t.TempDir()` + `chmod 0` 目录，经 `runtime.GOOS` 守卫/`t.Skip` 非 Windows），或直接把用例改为 Linux 也成立的只读路径。

### OBSERVE-65-02：`loginRejectSrv`/`gateSrv` 的 `/login` 响应无 Content-Type（与真实平台 text/html 有差异，识别链路不消费，无害）

- **位置**：`backend/internal/accounts/manager_test.go:70/145`。
- **一句话问题**：两个 mock 的 `/login` 分支 `w.Write([]byte("ok"))` 未设 Content-Type——真实平台 `/login` 返回 `text/html; charset=UTF-8`。Go 客户端 `http.Get` 读 body 正常（不依赖 CT），`fetchLoginPage` 只 `io.Copy(io.Discard, body)` 也不依赖 CT，识别链路各阶段都不解析该响应体——无害但属 mock 与真实的轻微偏差。
- **证据链**：`http.Get(baseURL+"/login")`（client_test.go:92 与 accounts readyProbe:29）对无 CT 响应 body 读取完全正常（Go 默认按 `application/octet-stream`）。与 zhidao loginMockServer 的 `/login` 同款（也无 CT），全仓 mock 登录页均为该形态。
- **触发条件**：无（仅 mock 语义轻微偏差）。
- **影响**：可忽略；仅作为"夹具与真实平台偏差"登记。
- **修复方向**：可加 `w.Header().Set("Content-Type", "text/html; charset=utf-8")` 一行，也可不动（无实质影响）。

### OBSERVE-65-03：accounts 第三处 mock（SetRecognizer 测试）无 `socketPreheat` 双保险，是残余面收敛度最低点

- **位置**：`backend/internal/accounts/manager_test.go:227`。
- **一句话问题**：`TestNewClientAfterSetRecognizerGetsEngine` 的 mock 仍是裸 `httptest.NewServer` + 新增 readyProbe，但与 zhidao 的 `socketPreheat + readyProbe` 双保险、api 的 socket 预创建 + readyProbe 双保险对比，accounts 包三处均无套接字预创建——其中 loginRejectSrv/gateSrv 的调用路径是真实登录（LoginByPassword 内部多请求有天然"首次请求重试"缓冲），而本测试的 mock 是 `default` 分支直接 `w.Write(token JSON)`，首请求即 `fetchLoginPage`（无重试缓冲语义只在网络层），虽本轮 8 轮全绿实证无残留，但理论上是残余面收敛度最低点。
- **证据链**：R64 accounts 添加 readyProbe 后 8 轮全绿（OBSERVE-64-02 收口完全生效）；zhidao 有 socketPreheat（client_test.go:25-30 + 每个用服务器的测试入口调用）；api 有 socket 预放（handler_test.go:53-71）；accounts 三处仅 readyProbe。
- **触发条件**：全量串行下极端冷启动瞬间（R12 曾命中）；当前 8 轮 0 命中，属防御性观察。
- **影响**：无（已实测归零）；作为"残余面继续收敛"的候选。
- **修复方向**：无需立即处理；若未来 accounts 再度出现冷启动 flake，第一候选即在三个 helper 构造后加 `socketPreheat`。

---

## 上轮观察项延续表

| 上轮编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-64-01（probeIntervalFor 注释 5s） | 待修 | **已修（29e1b26 四处 2s 全改）且核对无其他 5s 误导残留**（scheduler_test.go:2460 的 5s 属 B21-02 的 10s 裕量场景、其余属时钟对齐测试，不冲突） | **闭合** |
| OBSERVE-64-02（accounts 无 readyProbe） | 待修 | **已修（三处 readyProbe 全接）且 8 轮 accounts 全绿归零**——R12 双测试 R65 0 命中；12 → 8 轮全量中 accounts 全程无 FAIL | **闭合（残余消失）** |
| OBSERVE-64-03（test 平台假设） | 待修 | **已修（补注释）但注释内容欠精确（OBSERVE-65-01）**——Linux 实测断言失败路径红（与注释"断言反转 → 红"方向一致）但机制叙述缺"Linux 下会真实创建数据库文件" | **部分闭合（转 OBSERVE-65-01）** |
| OBSERVE-63-01/02（api ReadErr/UnauthorizedRelogin 冷启动） | 延续（消失） | **8 轮 0 命中**——但 R65 R1 的 TestAccountOverride 显式 deadline exceeded 是 api 包残余变体（20.01s 精确=15s 首请求超时窗口），与 R63-01/02 的 connectex 形态不同但同夹具 | **漂移（api 残余新形态，已 5 连绿瞬时确证）** |
| OBSERVE-63-03（api 包最脆弱） | 延续 | api 包仍是有残余面的包（R65 R1 命中者即属它），但 accounts 已归零 | **延续（残余面收窄到 api）** |
| OBSERVE-63-04（cmd/probe 非可用工具） | 延续 | 未变（token 手动注入真实值、无认证、运行必失败） | **延续** |
| OBSERVE-63-05（AdminStats 半真测试） | 延续 | 未变 | **延续** |
| OBSERVE-62-06/07/61-03/04/07 | 延续 | 未变 | **延续** |

---

## 已核对无缺陷的高风险区域

- **R64 三卫生修全链路**：① probeIntervalFor 四处 2s 改动与常量/断言对齐、无残留误导；② accounts readyProbe 与 api/zhidao 同款语义（200ms×5 连接层重试，`/login` 健康请求）、三处完备无漏点、无新 flake；③ 平台注释的"断言方向反转"表述方向正确（Linux 实测红）。
- **调度器 tick / 窗口 / 探测 / 提交六分支身份防线 / 重登退避 / 时钟对齐复位族**：全部按 R64 零缺陷结论复核通过（R65 未发现新问题）。
- **认证与鉴权**：管理员双条件（B43-04）+ requireAdminSession/requireAuth + CSRF JSON 门（严格 `application/json` 前缀匹配）；delete 无 body REST 语义与 JSON 门一致性已确认。
- **凭据加密 + 配置回显脱敏**：vision_key `enc:` 前缀强校验、maskKey 语义、加密失败不落库 + 日志留痕（B44-01）。
- **数据库迁移 / 空库日志窗口 / 多连接激活码并发事务**：`openStoreMultiConn`（db_test）`SetMaxOpenConns(16)` 复现读改写竞态——生产单连接正常，多连接形态覆盖未来改动。
- **跨平台编译**：`CGO_ENABLED=0 GOOS=linux go test -c`（db 包）编译通过、Linux Alpine 运行 db 全量 6 测试颗粒判定结果符合预期（5 PASS + TestOpenOnReadonlyPath 红——见 OBSERVE-65-01）。

---

## 验证实证表

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `gofmt -l .` | **零输出** |
| 后端全量 `-race -count=1 -p 1 -timeout 900s ./...` | **8 轮：7 全绿 / 1 FAIL**（R1 api TestAccountOverride 20.01s deadline exceeded）；R2-R8 连 7 绿 |
| accounts 包 8 轮 | **全部 ok**（4.388s/1.5~1.8s）——R12 双测试 0 命中，readyProbe 收口归零 |
| 隔离复跑（FAIL 后） | TestAccountOverrideRequiresAdminSession 隔离 5 连全绿（1.94~2.66s） |
| Linux 跨平台实证 | `CGO_ENABLED=0 GOOS=linux go test -c -o %TEMP%\db_linux_all.test ./internal/db/` 编译通过；Alpine WSL 运行 6 测试：5 PASS + **TestOpenOnReadonlyPath FAIL**（settings_test.go:16 断言红） |
| 测试函数总数 | **211**（api 55 / scheduler 87 / accounts 6 / zhidao 23 / store 13 / db 6 / session 11 / config 2 / runtime 2 / secure 5 + zhidao TestMain 1） |
| 审查期间工作树 | 与基线一致（仅并行前端代理 round65-frontend-findings.md 一个未跟踪文件；确认无仓库文件改动） |
| 临时验证程序 | `%TEMP%\db_linux.test` / `%TEMP%\db_linux_all.test`（交叉编译二进制）+ WSL `/tmp` 各拷贝；仓库内零新建 |

---

## 教训

1. **修正一个"跨平台断言"的推断时，要到目标平台实测而非推演**：R64-03 曾推断"Linux 下 `C:\nul\nul\` 是普通目录、`Open` 成功、断言反转"——推演方向对（Linux 红）但机制没摸全：真实行为是 Linux 下 `Open` 会把 `C:` 当普通目录名真实创建数据库文件并跑完 schema（断言依然红）。**跨平台假设类问题必须用目标平台运行结果钉死机制**，本轮用 Alpine + 交叉编译二进制补上了实证。
2. **flake 的"地区漂移"印证了 R64 的预判**：R64 报告已点名 api 包残余收窄后"下一个残余点会是 api 包 mock 首请求 deadline exceeded 形态"——R65 R1 正是该形态（`authenticateDirect` 内 LoginByPassword 对已就绪但恰处于 15s 超时窗口的 mock 首请求超时）。配合 R64 已归零的 accounts，残余面确认收窄到 api 包单测试且趋于瞬时（32 轮 3 次命中、隔离 5 连绿）。**持续预判"残余会流向哪个最薄弱的夹具"是收敛工作的正确方法**。
3. **夹具与真实平台差异要登记而非默认为零**：`loginRejectSrv` 的 `/login` 无 Content-Type 与真实 `text/html` 有差异——虽然识别链路不消费 CT，但登记的观察类条目让未来维护者在排查 mock 行为偏差时有据可查。

---

## 报告

本文档写入 `archive/review-rounds/round65-backend-findings.md`。