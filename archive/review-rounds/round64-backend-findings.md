# R64 后端只读审查发现报告

> 审查基线：master @ `1861093`（R63 收官）。R63 后端一修（commit `76c734c`）：zhidao 包新增导出 `SharedTransport()` 访问器（复用 sharedTransport 连接池）+ `cmd/probe/main.go` 改 `&http.Client{Timeout: 15*time.Second, Transport: zhidao.SharedTransport()}`（原 `http.DefaultClient` Timeout=0 无兜底 + 默认 2 连接池）。前端一修（commit `7e4ef3a`）：`shouldDeferSave` 增第三参数 echoed（回显已完成维度），三消费点传 `echoedRef.current`。
>
> 范围：backend/ 全部 Go 源码。方法：全包逐行通读（2026-09-21 实测 2004 行/文件的调度器曾有临时文件改动，当日基线文件数为 15,227 行含测试）+ 协议族交叉核对（R52 连接活性自愈 / R40-43 身份防线 / R62-63 注释契约 / B39-02 状态码家族 / F 系列前端契约）+ 全量 `-race -count=1 -p 1 -timeout 900s ./...` 连跑 **24 轮**统计。
> 铁律遵守：绝对只读，无任何仓库内文件修改（含测试注释），未新建任何临时验证程序于仓库内（全部验证均是只读命令与已有测试）。

---

## 结论先行

- **CRITICAL 0 / MAJOR 0 / MINOR 0 / OBSERVE 3**。**R63 后端一修完全正确**，生产逻辑本轮零 CRITICAL、零 MAJOR、**零 MINOR**——R63 修复（MINOR-63-01）经连接池并发语义 + 契约族比对实证完全正确；R63 前端一修（echoed 第三参数）与后端 /state 契约确认语义一致，后端侧无需任何处理。唯一新视角发现均为低影响 OBSERVE。
- **最致命 3 条（按影响排序）**：
  1. **flake 趋势创历史新高：R64 全量 **22/24 全绿**（2 轮 FAIL，全部 Windows 回环冷启动残余，隔离复跑 + 单独连跑恒绿）**——趋势 R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → R63 18/21 → **R64 22/24**。连续全绿段（RUN5-RUN11 连 7 绿 + RUN13-RUN24 连 12 绿，最长连续 12 轮）证明 readyProbe/socketPreheat 自 R63 起对已知残余面已大幅收敛。2 轮 FAIL 均为 api/zhidao/accounts 包 mock 服务器冷启动残余（connectex / FLUSH+Hijack 直断 / 高并发识别直断），均无确定性缺陷证据。
  2. **R12 全量轮 accounts 包双 FAIL（connectex，冷启动形态新宿主包）**——`TestLoginByPasswordAllowedWhenGateBudgetAvailable` / `TestNewClientAfterSetRecognizerGetsEngine` 在 accounts 包冷启动下 mock /login 首请求 connectex（前台 `go test ./internal/accounts/` 隔离 3 连跑含 27.85s 一轮仍全绿 + 单测 5 连跑全绿证明是残余）。accounts 测试夹具是唯一"每测试新建 mock server 且无 readyProbe/socketPreheat"的包（与 api/zhidao 夹具对比），是残余面下一个收敛点。
  3. **R4 全量轮 zhidao `TestCaptchaConcurrency` FAIL（FLUSH+Hijack 直断 + 高并发识别窗口合流）**——mock 在热 keep-alive 池下对 10 并发识别中的某连接直断（`wsarecv: An existing connection was forcibly closed`），httpDo 自愈仅重试 dial/write 错误（read 不重试，防双报语义正确），该 read 错误穿透导致 1 路识别失败。隔离 10 连跑 + 单独 3 连跑全绿。属 R52 已知残余面（高并发 mock 直断形态首次命中），非缺陷。

---

## 分级发现

### OBSERVE-64-01：`TestProbeIntervalFor` 注释"临门 5s"与常量 `probeIntervalNear = 2s` 表述错位（测试注释历史残留）

- **位置**：`internal/scheduler/scheduler_test.go:994`（`// TestProbeIntervalFor 分阶段探测间隔：平日 30s、临门与已到点 5s 收紧。`）、1004（`// 临门：距开放 4 分钟 → 5 秒`）、1007（`t.Fatalf("临门应 5s...")`）、1009（`// 已到点：开放后 1 分钟 → 5 秒盯守`）。
- **一句话问题**：测试注释与断言文案中的"5s"是早期版本（探测间隔曾为 5s）的历史残留；当前常量 `probeIntervalNear = 2 * time.Second`（scheduler.go:64）且断言比较的正是 `probeIntervalNear` 值（恒过）。断言逻辑正确（比较的是常量，2s 时也通过），注释误导"临门 5s"。
- **证据链**：`scheduler.go:64` `probeIntervalNear = 2 * time.Second // 临门收紧至 2 秒`；`scheduler_test.go:1005-1007` 用 `got != probeIntervalNear` 判断并 `Fatalf("临门应 5s...")`——文案与实际常量的唯一错位点。
- **触发条件**：维护者按注释推断"临门 5s"，与 probeIntervalNear=2s 的真实契约互相矛盾。
- **影响**：极低（纯注释/文案残留，断言恒真）。仅凭 CLAUDE.md 决策锚 20（"代码注释严禁轮次前缀标签，轮次决策历史统一落手册"）的注释卫生精神应清理文本，非破坏性。
- **修复方向**：将四处"5 秒"改为"2 秒"（与常量一致）。一行级修订。

### OBSERVE-64-02：`accounts` 包测试夹具是唯一无"冷启动就绪前移"的包（R12 轮 connectex 双 FAIL 的夹具层面归因）

- **位置**：`internal/accounts/manager_test.go:32/106/198`（三处裸 `httptest.NewServer`）。
- **一句话问题**：api 包 `newTestDepsModeName` 已实现"套接字预创建 + readyProbe 就绪前移"（handler_test.go:67-71 + 139-141），zhidao 包已有 `socketPreheat + TestMain + readyProbe`（client_test.go:25-42/50-79），唯独 accounts 包三处 `httptest.NewServer` 使用后无任何冷启动前移——R12 全量轮该包两测试在 mock accept 就绪前收到首请求 connectex（64 秒测试总耗时被 connectex 后 client.go 重试吃掉，最终在 2s 内仍失败）。
- **证据链**：R12 `FAIL: TestLoginByPasswordAllowedWhenGateBudgetAvailable (2.02s)` `初始化登录会话失败: Get "http://127.0.0.1:50495/login": dial tcp ...connectex` + 同轮 `TestNewClientAfterSetRecognizerGetsEngine (2.01s)` 同形态；`go test ./internal/accounts/` 隔离 3 连跑全绿（含 27.85s 慢轮——同是冷启动残余被 fetchLoginPage 首请求重试摊薄）;单测 5 连跑全绿。与 R63 的 api 包 ReadErr/UnauthorizedRelogin 同族残余（冷启动首请求形态），唯一区别是本包无人前移窗口。
- **触发条件**：全量 `-p 1` 串行下，前序包（api/scheduler 的高耗时 mock + TIME_WAIT 海量）结束后 accounts 包首个 mock server 的首请求落在 accept 未就绪窗口。
- **影响**：极低（测试 flake ~4.2% 全量轮命中率，固定发生在 accounts 包，产品零影响）。夹具层面对齐 api/zhidao 的 readyProbe 即根治，但修复动作属测试夹具改动，非产品缺陷。
- **修复方向**：accounts 包夹具构造后加一条 `readyProbe`（与 api/zhidao 同款 200ms×5 轮询）或复用其模式。

### OBSERVE-64-03：`TestOpenOnReadonlyPath`（db 包）用 Windows 保留设备路径 `C:\nul\nul\test.db` 模拟"只读路径打开失败"，在非 Windows 平台不成立

- **位置**：`internal/db/settings_test.go:12`（`Open(\`C:\nul\nul\test.db\`)`）。
- **一句话问题**：`\nul\` 是 Windows 保留设备路径；在 Linux/macOS（CI 有 CGO=0 交叉编译）下该字符串是普通目录路径，`MkdirAll` 成功、Open 行为与断言意图不符（Linux 上会静默建目录、SQLite 打开成功返回 nil，测试 flake 或误判取决于断言方向）。当前断言是"必须返回错误"，Linux 上 `Open` 大概率成功 → 测试红。
- **证据链**：`db.go:16-18` `os.MkdirAll(filepath.Dir("C:\nul\nul\test.db"))`；Windows 保留设备名 `NUL` 在任意 Windows 路径段均被内核拒绝。CLAUDE.md 部署规范确认 Windows 为发布主平台（单 exe/CGO=1），Linux/macOS 为 CGO=0 交叉编译目标（ci.yml 全量测试在 Linux worker 跑）。
- **触发条件**：在非 Windows 上执行 `go test ./internal/db/`。
- **影响**：低（当前仅 Windows 开发/CI，未触发；跨平台 CI 若加 windows-only 约束前会红）。测试意图（"只读路径应报错"）正确，夹具平台假设未标注。
- **修复方向**：加 `runtime.GOOS` 判断或 `t.Skip` 非 Windows（或用 `t.TempDir()` 构造只读权限目录替代系统保留路径）；不影响 Windows 现状。

---

## 新视角逐项实证裁决

### A. R63 后端一修是否完全正确

**① SharedTransport() 导出访问器——并发安全 + 契约族**

- **http.Transport 并发语义**：`net/http.Transport` 官方文档明确"Transport 是并发安全对象，多个 goroutine 可同时调用其方法"；RoundTripper 契约要求实现必须安全并发复用。`sharedTransport` 全局唯一实例在多账号 `zhidao.Client` 之间本就共享（client.go:87 全部客户端绑定同一指针），Export 出去给 `cmd/probe` 复用只是把"包内共享"扩大为"包间共享"，连接池（MaxIdleConnsPerHost=64）按 host 归一，不引入任何新的并发冲突。
- **客户端级状态隔离**：`http.Client`（Timeout 等）不写入 Transport；连接池内的 "idle keep-alive conn" 与 "Cookie" 无任何耦合——`cmd/probe` 手动构造的 `http.Client{Timeout: 15s, Transport: sharedTransport}` 复用连接池但请求头（idToken param + Cookie header）完全自带，不会与其他客户端共享认证状态。
- **与全仓契约族一致性**：全仓到真实平台的生产路径（`doRequest` 走 `c.http` = sharedTransport + 15s Timeout；`fetchLoginPage`/`fetchCaptchaImage`/`submitLogin` 走会话内 `http.Client{Timeout: 15s}`）全部"共享连接池或同等超时兜底"。probe 修复后语义对齐：Timeout=15s（与 doRequest/fetchLoginPage 完全一致）+ sharedTransport（与 doRequest 完全一致）。**唯一缺失的自愈层 `httpDo` 对 probe 不生效**——probe 是 cli 调试工具（token 需手动注入真实值，R61/62 已定位"非可用工具"），一次请求失败直接退出是合理的工具语义，绝无必要引入重试（重试 POST 双报风险在此路径适用性无关）。**裁决：A①完全正确，无并发风险，契约族一致**。
- **交叉复核**：`http.DefaultClient` 在修复后已从全仓消失（grep 仅剩注释文本——probe/main.go:29 注释与 handle_test readyProbe 注释），所有真实 HTTP 构造点均有 Timeout ✓。

**② R63 收尾"全量回归全绿"结论是否正确——本轮 24 轮独立复验**

- R63 报告称"21 轮 18 全绿"（3 轮 FAIL 全为 api 包 mock 冷启动，隔离复跑全绿）。本轮 24 轮独立复验：**22 全绿 / 2 轮 FAIL**（R4 zhidao 单包 TestCaptchaConcurrency / R12 accounts 包双测试），排除 R63 未覆盖的 zhidao/accounts 两个新地区残余后，**api 包该两测试本轮 0 命中**（ReadErr/UnauthorizedRelogin 连续 24 轮全绿，自 R63 修后未再触发）。R63"api 包残余收窄"结论成立且进一步收敛。**裁决：A②R63 结论正确**。

**③ 前端 R63 M-1（echoed 第三参数）与后端 /state 契约兼容性**

- `/state` 契约：`StateForAccount` 按账号全量下发 courses（handler.go:547 → scheduler.go:704-729 逐账号过滤 `st.Courses`），`SetTargetsForAccount`→`rebuildCoursesForAccountLocked` 保证"只要该账号还有目标，courses 就非空"。因此"courses 永驻非空"在已回显账号是常态（除非用户把目标全清空）。
- **兼容性裁决**：前端 `echoed=true`（回显 effect 已把后端旧目标合并进 selected，且只合并一次）→ 放行整包 PUT，与后端"整包 PUT = 该账号全量目标替换"语义完全一致（`handleSetTargets` 先 `SetTargetsForAccount` 全删后插，store 事务内 delete+insert）。放行后的 PUT 内容 = 合并后的完整 selected = 后端将落库的目标，无覆盖丢失。首帧未到（stateData===undefined）无条件下推 → 与后端"SaveTargets 未知旧数据时先探明再全量写"安全方向一致。**裁决：A③后端侧确认兼容，前端修复正确无需后端联动**。

### B. 全量 flake 复测（唯一判定源）

- **24 轮统计**（`-race -count=1 -p 1 -timeout 900s ./...` 连续连跑）：
  - RUN1-RUN3 全绿 → **RUN4 FAIL**（zhidao TestCaptchaConcurrency 37.04s）→ RUN5-RUN11 连 7 绿 → **RUN12 FAIL**（accounts 双测试）→ RUN13-RUN24 连 12 绿。
  - **合计 24 轮：22 全绿 / 2 轮单包 FAIL**（zhidao 1 + accounts 1）。
- **FAIL 明细与隔离复跑**：
  - **R4 zhidao `TestCaptchaConcurrency`（37.04s）**：`识别 0 应成功: Post .../chat/completions: read tcp ...: wsarecv: ... forcibly closed`——10 并发 Vision 识别中 mock 热 keep-alive 连接被服务端直断（httptest 每个请求独立 goroutine，关闭 idle 连接是服务端合法行为），httpDo 自愈仅重试 dial/write（read 不重试双报语义正确）→ 该路 read 错误穿透 → 断言失败。**隔离 10 连跑 + count=3 单独跑全绿**。
  - **R12 accounts 包两测试（2.02s + 2.01s）**：`initialize login session failed: dial tcp ...connectex`（mock /login 首请求冷启动）——accounts 包是唯一无 readyProbe 前移的包（见 OBSERVE-64-02）。**acounts 包隔离 3 连跑全绿（含 27.85s 慢轮）+ 单测 5 连跑全绿**。
- **趋势**：R57 3/11 → R58 2/10 → R59 2/11 → R60 2/12 → R61 1/16 → R62 17/21 → R63 18/21 → **R64 22/24**。**结论：残余未根除但持续被压低，且 FAIL 地区在漂移（R63 的 api 包残余已消失 24 轮零命中；新命中 zhidao 高并发直断 + accounts 无就绪前移两个夹具面）。CI `||` 重跑仍是正确姿势。**

### C. gofmt -l . 全量复检

- **零输出**（backend/ 全部含测试）。R63 改动后无格式化回潮。

### D. 全包逐行通读找新问题

- **未发现 CRITICAL / MAJOR / MINOR**。OBSERVE-64-01/02/03 为仅有的三处新观察（注释残留 / 测试夹具缺口 / 跨平台测试假设）。
- **重点复核通过的区域**：
  - 连接活性自愈族：httpDo dial/write 重试 + read 不重试 + IsReadErr 四分支 + ErrUnexpectedEOF 穿透断言（isreaderr_test 全形态）+ fetchLoginPage 4xx/5xx 独立重试注释——R62/63 修复全部保留且无回潮。
  - 身份防线六分支 sameClientFor（成功/失效/风控/窗口关闭/实时复核满员/实时复核失效）+ maybeRelogin 决策侧 ClientFor 复核 + 写回侧复核——R63 相关测试（R39-43 族 12+ 测试）逐条通读全绿。
  - 窗口状态三判据单源 windowClosedLocked（含 10s 裕量、EmptyProbeRuns 量变、syncFailStreak 复位语义）+ 识别槽三硬契约 + openTimeForLocked 绝不截断过期值。
  - SQLite 单写者 + busy_timeout 5s + WAL + foreign_keys + migrateAddPublishMeta 先于 refuseLegacy + 空库日志窗口语义（window_empty_test）+ 零吞错落库点（全仓扫描 `_ =` 落库/AppendLog 均已记日志）。
  - AES-256-GCM 密钥族（随机 nonce / 篡改拒绝 / master_key 32 字节校验）+ 凭据 enc: 前缀拒绝明文 + vision_key 加密落库。
  - 登录限流双桶（login/activate 独立 + GC）+ B42-01 gateTryAcquire 非阻塞准入 + gateWait 共享计数 + 撞名学生双条件（B43-04）+ 管理口令时延拉平（n4）。
  - 状态码家族：panic 500 / 会话 401 / 管理 403 / 限流 429 / 未知 /api 404 + writeJSONStatus 先头后码（R59 实证）——HTTP 层与 body.code 双通道对齐。
  - 前端契约面：/state 按账号全量下发 courses、open_time_known 过期降级、window_closed 三态同源、begin_times 恒下发、识别槽保留不删——与 R63 前端 echoed 修复兼容（A③）。

---

## 上轮观察项延续表

| 上轮编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| MINOR-63-01（cmd/probe DefaultClient 无超时无自愈） | 已修（76c734c：SharedTransport 导出 + probe 15s） | **闭合**——http.Transport 官方并发安全、共享连接池无认证状态耦合、与 doRequest/fetchLoginPage 同 Timeout；`http.DefaultClient` 全仓消失；httpDo 自愈对 cli 工具不适用性合理 | **闭合** |
| OBSERVE-63-01（ReadErr 测试冷启动 FAIL） | 延续 | **消失**——24 轮 0 命中（R63 修后 api 包 ReadErr/UnauthorizedRelogin 全程未触发） | **闭合（残余消失）** |
| OBSERVE-63-02（UnauthorizedRelogin mock 缺 /chat/completions 分支） | 延续 | **消失**——同 R63 修后 24 轮 0 命中 | **闭合（残余消失）** |
| OBSERVE-63-03（api 包最脆弱） | 延续 | **漂移**——api 包本期不再是最脆弱包（新命中 zhidao 高并发直断 + accounts 冷启动）；api 包 24 轮全绿 | **延续（地区漂移 → OBSERVE-64-02）** |
| OBSERVE-63-04（cmd/probe 非可用工具） | 延续 | 未变（token 手动注入真实值、无认证、运行必失败）——本轮 MINOR-63-01 已修后工具面无遗漏 | **延续** |
| OBSERVE-63-05（AdminStats 半真测试） | 延续 | 未变（`failingTargetsStore` 类型已定义但未使用；是否接口化属超范围） | **延续** |
| OBSERVE-62-06/07/61-03/04/07 | 延续 | 未变 | **延续** |

---

## 已核对无缺陷的高风险区域

- **R63 后端一修全链路**：SharedTransport 并发安全（http.Transport 官方契约）+ 连接池无认证耦合 + Timeout 与 doRequest/fetchLoginPage 全仓一致 + DefaultClient 全仓零残留。
- **R62 后端两修全链路**：IsReadErr 四分支与标准库转移/传输层逐段一致（body.readLocked ErrUnexpectedEOF / transportReadFromServerError 仅响应头阶段 / 超时双文案）——R63 后未回潮、isreaderr_test 全形态断言在位。
- **前端 R63 M-1（echoed 第三参数）与 /state 契约**：服务端"整包 PUT = 全量替换"与前端"已回显后 selected 完整"语义自洽，无覆盖丢失。
- **身份防线族 / 窗口判据 / 识别槽契约 / 登录链路 / 数据库迁移 / 加密密钥族 / 会话票据 / 限流与状态码家牌 / XFF 可信反代**：全部沿上轮零缺陷结论复核通过。

---

## 验证实证表

| 项 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `gofmt -l .` | **零输出** |
| 后端全量 `-race -count=1 -p 1 -timeout 900s ./...` | **24 轮：22 全绿 / 2 轮单包 FAIL**（R4 zhidao 37.04s / R12 accounts 双测试）；全部有 FAIL 轮日志含明确的"残留形态"（用力直断 wsarecv + connectex） |
| 隔离复跑（FAIL 后） | TestCaptchaConcurrency：count=3 单独跑 + 10 连跑（前轮已验）+ 双包合跑全绿；accounts 包：隔离 3 连跑全绿（含 27.85s 慢轮）+ 双单测 5 连跑全绿 |
| 前端验证 | `npm run build`（tsc -b + vite）全绿（R63 前端修复产物 1948 模块 1.14s）；`node --import jiti scripts/target-guard-check.ts` 因 Node24 ESM 扩展名解析不兼容无法独立运行（**说明**：该脚本使用 `--import jiti` 未加扩展名的 `import "../src/lib/targetGuard"`，Node 24 strict resolve 拒绝对无扩展名导入的映射——TDD 断言在既有 `tsc -b` 构建通过的前提下未单跑，属运行环境差异而非脚本缺陷；R63 前端一修已由前端代理 build 全绿背书） |
| 测试函数总数 | **210**（api 55 / scheduler 87 / store 13 / zhidao 23 / accounts 6 / db 6 / session 11 / config 2 / runtime 2 / secure 5）+ zhidao TestMain 1 |
| 审查期间工作树 | 与基线一致（仅并行前端代理 round64-frontend-findings.md 一个未跟踪文件） |
| 临时验证程序 | 未创建（全部验证走只读命令与既有测试） |

---

## 教训

1. **flake 地区的漂移是"收敛进度"的真实信号**：R63 修掉 api 包两测试后，api 包 24 轮零命中（残余从 api 包消失），但新命中出现在两个此前从未 FAIL 的包——zhidao 高并发识别 mock 直断（R52 已知形态的新宿主）与 accounts 无就绪前移。**收敛工作既要修掉已知宿主，也要预判"残余会流向哪个最薄弱的夹具"**——accounts 是唯一裸 `httptest.NewServer` 无 readyProbe 的包，正是下一个收敛点。
2. **注释"数值表述"与常量一致性是读代码者的误导温床**：`probeIntervalNear=2s` 而测试注释/断言文案写"5s"（历史探测间隔残留），断言按常量比较恒绿、误导仅存在于文本层。**凡注释写明具体阈值，审查时必须对照常量值核对**（本例警示教育）。
3. **测试夹具的平台假设要显式化**：db 包 `TestOpenOnReadonlyPath` 用 Windows 保留设备路径 `C:\nul\` 模拟只读失败，在 Linux CI 下会因路径是普通目录而改变语义。**测试用平台专属设施时要么标注 GOOS 约束、要么用跨平台等价构造**（当前 Windows 主平台未触发，跨平台 CI 扩展前应补齐）。