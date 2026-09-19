# round40 后端审查原始发现

> 审查基线：master @ fa6c003（round39 全部修复已落盘；`go build ./...` 与 `go vet ./...` 双通过实证）
> 范围：backend/ 下全部 Go 源码（main.go、cmd/、internal/ 各包），绝对只读模式，未修改任何文件。
> 判据：项目根 CLAUDE.md《工程决策手册》约 26 条决策锚 + 部署/迁移规范 + legacy/website-source 逆向契约 + round39 修复报告。
> 方法：全文件通读 + 锁序/竞态逐路径推演 + 编译验证；对上轮观察项（m39-02/o39-01…o39-05）逐一核实。

---

## MAJOR（明确的功能失效或错误行为，1 条）

### M40-01 scheduler.go:1195/1202 —— maybeRelogin 退避/节流判定混用本地钟 `time.Since`，全仓库最后一处"时间基准分裂"

- **文件:行号**：backend/internal/scheduler/scheduler.go:1195（失败退避 `time.Since(t) < wait`）、1202（30s 基础节流 `time.Since(t) < reloginInterval`）
- **严重级**：MAJOR
- **问题陈述**：`maybeRelogin` 的两处节流判定用本地钟 `time.Since`，而写入侧 `s.reloginAt[acct] = time.Now()`（1206 行）同样是本地钟——这对内部一致，但调度器的提交守卫（spawnChain `isRateLimitedLocked`）与 tick 判定全部用对齐钟（B16-M1/MAJOR-F 已消灭同类混用）。`reloginAt` 与 `rateLimited` 是同一族"防轰炸时间戳"，两者若一个本地钟一个对齐钟，黄金期失效-重登-再提交的路径上退避窗口被整体平移 offset（实测 ~640ms 量级，多数场景无害）。真正的问题在下一条的合并推演：**它与 M40-02 叠加后把整条黄金期失效恢复链路变成"本地钟写、对齐钟读"的完整跨钟判定，方向与 B20-03/MAJOR-F 明确宣示的单一时间基契约相悖**。
- **触发场景推演**：clockOffset 为负（服务器慢于本地，极端部署/时钟漂移代理下可达秒级）→ 重登成功后 `s.reloginAt` 记录本地时刻；spawnChain 的 `tokenValidForLocked` 读 `relogging`（已清）放行，`isRateLimitedLocked` 用对齐钟判退避——两类时间戳基准不同，黄金期失效-重登-抢课的时序判定整体漂移。
- **建议修法**：`reloginAt[acct] = s.nowAligned()`，判定统一 `s.nowAligned().Sub(t)`（与 B16-M1/MAJOR-F 同源）。

### M40-02 scheduler.go:1220 —— 自动重登不走全局 doLogin 频率闸门，`gateWait/gateLoginPerMin` 形同虚设

- **文件:行号**：backend/internal/scheduler/scheduler.go:1220（`s.clients.Relogin(acct)`）
- **严重级**：MAJOR
- **问题陈述**：accounts.Manager 的全局重登闸门（`gateWait` + `gateLoginPerMin=2`）只在 `Manager.Relogin`（`ReloginIfNeeded` → `Client.Login`）路径生效；而**整个调度器唯一的自动重登调用点就是 `maybeRelogin` → `s.clients.Relogin(acct)`**（全仓库 grep 实证，handler 层失效路径也只调 `MaybeRelogin`，最终汇聚到这里）。也就是说：平台 doLogin 的全校收敛防线只防了"从未经过调度器的直接登录"，而真正高频的重登流量是"N 账号同时失效时 maybeRelogin 各自并发 goroutine 发起 Login"——**恰是该闸门注释声明要防的场景**，全部绕行。
- **触发场景推演**：多账号部署 + 平台 token 集中过期（统一批次/同一失效时刻）→ 调度器 per-account 失效检测各自触发 maybeRelogin → 每个账号一个 goroutine 直接 `ReloginIfNeeded` → `Login`（完整登录链路：GET /login + GET /login/captcha + Vision 识别 + POST /login/doLogin）→ 同时打平台 doLogin，N 并发（此前 B9-03 只退避了"单账号连续失败"的重试节奏，挡不住 N 账号首波齐射）。平台"登录失败次数过多，请 30 分钟后重试"按 IP 计数时，全校（一个出口 IP）一次性锁号。
- **建议修法**：`maybeRelogin` 发起前也走 `AccountClients` 暴露的 gate 语义（或把 `Relogin` 接口改为经 `Manager.gateWait()` 排队），保证全部 doLogin 流量收敛到 `gateLoginPerMin`。

---

## MINOR（边界瑕疵 / 契约不自洽，3 条）

### m40-01 router.go:70-72 —— requireJSONBody 的 CSRF-403 仍用 writeJSON 恒 200，round39 改漏一处

- **文件:行号**：backend/internal/api/router.go:70-72（`requireJSONBody`）；同族 handler.go:104-106/305-307/378-380/476-478/739-740/948-950（业务解析失败路径）
- **严重级**：MINOR
- **问题陈述**：B39-02 把 4 类基础设施错误（panic 500 / 会话 401 / 管理 403 / 限流 429）改成了真实 HTTP 状态码，但 `requireJSONBody`（副作用的 CSRF 第一道门）仍然 `writeJSON(w, 403, ...)` → HTTP 200。CSRF 拒绝与登录接口的 CSRF 拒绝（router.go:100 已写 403）语义相同、路径相邻，HTTP 层状态却分裂——监控/反代对"被 CSRF 拒的副作用请求"无法在 HTTP 层识别。
- **触发场景推演**：安全扫描/反代按状态码识别 CSRF 拦截面：登录接口 403、PUT /api/targets 等副作用接口 200——错误码语义不一致，且与 m39-01（本轮确认修复后）声称的"基础设施路径全覆盖"陈述不符。
- **建议修法**：`requireJSONBody` 改 `writeJSONStatus(w, http.StatusForbidden, 403, nil, "仅接受 JSON 提交")`（与登录/激活两处 CSRF 门同款）。

### m40-02 scheduler.go:1411/1416 —— 同一课程成功/退避判定后，链内下一门课程才开始计算 now，但 `now` 在每门课锁内取

- **文件:行号**：backend/internal/scheduler/scheduler.go:1416（`now := s.nowAlignedLocked()` 在锁内）
- **严重级**：MINOR
- **问题陈述**：spawnChain 每门课在锁内取 `now`（对齐钟）判 `isRateLimitedLocked` 与 `markRateLimitedLocked` 写入——读写基准已统一（B16-M1 正确），此处观察的是**统计口径**：同一链内多门备选课程，`now` 每门课各自取一次，黄金期 250ms 节奏下多门课之间的退避截止基准不一致（亚毫秒级，无实质危害）。属于已确认正确的读写对称前提下的边界噪音，供调参时注意。
- **触发场景推演**：无真实错误序列；仅记录"锁内多次取钟"的统计差异。
- **建议修法**：链开头取一次 `now` 复用到全部课程（与 tick/open 单快照同策略）。

### m40-03 scheduler.go:975-977 + config.go:58 —— `OpenTime` 配置字段已完全死掉，但 config 仍默认注入 2026 年日期

- **文件:行号**：backend/internal/config/config.go:58（`OpenTime: envOr("XUANKE_OPEN_TIME", "2026-09-13 09:00:00")`）
- **严重级**：MINOR
- **问题陈述**：`cfg.OpenTime` 在 main.go 从未消费（scheduler.New 传 `time.Time{}`，注释明言"开放时间不做任何配置注入"），openTime 回退字段只在 `openTimeForLocked` 无识别槽时兜底（生产恒零）。config 层仍保留一个**硬编码 2026-09-13 09:00:00 的默认值**与 `XUANKE_OPEN_TIME` 环境变量读取——若未来某路径误用 `cfg.OpenTime`（或运维按旧文档配了它），会产生"识别槽为空时把 2026-09-13 当开窗点"的误导。死配置 + 过期日期同时存在，是明确的维护陷阱。
- **触发场景推演**：运维参考旧部署文档设置 `XUANKE_OPEN_TIME` → 值被解析进 cfg.OpenTime 但无人消费 → 识别槽未填充时展示层/判定层不会用它（契约正确），但文档与配置残骸误导排查。
- **建议修法**：删除 `Config.OpenTime` 字段与 `envOr` 默认值（或至少移除硬编码日期，改为注释说明"已由平台 beginTimes 自动识别取代"）。

---

## OBSERVE（存疑待核 / 低概率边界，4 条）

### o40-01 handler.go:216-227 —— `issueSession` 未激活前签发的是"未激活会话"占位，但 `/api/activate` 成功路径直接签发新会话，旧会话不吊销

- **文件:行号**：backend/internal/api/handler.go:215-228（issueSession）、150-152（未激活分支只发 ticket）
- **严重级**：OBSERVE
- **问题陈述**：未激活账号登录 → `handleLogin` 走 `CreateTicket` 分支，**不**调用 issueSession（不签发会话）；`handleActivate` 成功后才签发会话。会话签发路径在"未激活"与"已激活"之间没有互相吊销——但未激活分支本就不发会话，因此无残留令牌可复用。核实后该观察无真实触发路径（未激活账号始终没有会话，激活后新会话唯一），维持现状。
- **建议修法**：无（确认无缺陷）。

### o40-02 scheduler.go:679-680 + 1253-1256 —— `reloginResults` 补探测信号依赖 tick 主循环 select 消费，channel 满时非阻塞丢弃

- **文件:行号**：backend/internal/scheduler/scheduler.go:1253-1256（select default 丢弃）、679（消费）
- **严重级**：OBSERVE
- **问题陈述**：重登成功回传的"补一次探测"信号在 channel 满（cap 8）时静默丢弃——注释明言"宁可弃掉"。窗口开启瞬间多账号同时重登成功（8 个以上），后续账号的补探测被丢，其专属年级帧/识别槽要等到下个 tick（≤300ms）正常探测才刷新——黄金期丢失一次即时探测，属已文档化的刻意权衡。
- **建议修法**：无（与注释语义一致，维持观察）。

### o40-03 db.go:100-107 —— `columnExists` 拼接表/列名进 SQL（非参数化）

- **文件:行号**：backend/internal/db/db.go:101
- **严重级**：OBSERVE
- **问题陈述**：`SELECT 1 FROM pragma_table_info('%s') WHERE name='%s'` 用 `fmt.Sprintf` 拼接表名/列名——但两者均为编译期常量（调用点全部字面量），无用户输入可达路径，SQL 注入不成立。属于风格观察（若未来引入动态表名需改为白名单）。
- **建议修法**：无（当前无注入面，维持现状）。

### o40-04 scheduler.go:666-684 —— tick 主循环 select 分支在 `reloginResults` 上无限等待，`s.interval` 为 0 时 ticker 永不触发

- **文件:行号**：backend/internal/scheduler/scheduler.go:665-681
- **严重级**：OBSERVE
- **问题陈述**：`Start` 用 `time.NewTicker(s.interval)`；`New` 未对 interval≤0 做防御（生产 main.go 恒传 300ms，测试传值可控）。若调用方传 0/负值，ticker 立即触发一次后**永久静默**（select 只剩 ctx/reloginResults 分支），抢课引擎零探测零提交且无日志。属构造时契约（文档未声明），低概率。
- **建议修法**：`New` 内 `if interval <= 0 { interval = time.Second }`（一行防御）。

---

## 已核对无问题的重点区域（本轮逐项复核，梗读核过）

- **B39-01 指针身份比对已修复**：`sameClientFor`/`clientIdentity`（reflect 指针身份）在 spawnChain 成功分支（1490）与失效分支（1465）全部生效；B39-01 修复测试 `TestDeletedAccountRebuiltSameNameChainDropsSuccess` 已在库；重登成功分支经核实无跨身份寄生面（B21-03 存在性 + 自读新 token，fix-report 论证成立）。
- **B39-02 writeJSONStatus 已修复**：panic 500 / 会话 401 / 管理 403 / 登录与激活限流 429 全部真实状态码（handler.go + router.go git diff 实证）；`requireJSONBody` 的 403 是 B39-02 明确改动范围外的漏网（m40-01）。
- **B39-03 重登 nil store 守卫已修复**：scheduler.go:1243-1247 已包 `if s.store != nil`。
- **B39-04 emptyRunsFor 死代码已删除**：grep 实证全仓库无该符号。
- **B39-05 admin stats 补发 window_closed**：handler.go:919 已就位，与 WindowClosed() 同源。
- **m39-02 快照 TTL 本地钟混用（5 处）**：本轮再次核对 `ElectivesSnapshotFor`（778/793/799）、`ElectivesSnapshot`（870）、`CheckClassSelectable`（1803）仍用 `time.Since`——与 round39 定案"契约声明单基准、量级无害维持观察"一致，未升级。
- **o39-01 open.IsZero() 挂起提交**：tick 守卫链仍零值优先 return（996-998），识别槽空 + WindowOpened=true 的缺口未动——round39 定案"语义已按 B11-A1 刻意实现"维持观察。
- **o39-02 submitAll 快照竞态 / o39-03 vision key 空串清空 / o39-04 NAT 出口限流 / o39-05 tick 无 recover**：均维持观察（已文档化权衡，无新证据升级）。
- **WindowClosed 三判据单源** `windowClosedLocked()`：主判据（+10s 裕量）/时钟失败≥3（带"开放时间已过"）/幽灵窗口 EmptyProbeRuns≥3（+10s 裕量）——StateForAccount 与 WindowClosed() 共用，open 单快照（1130 行与 913 行各取一次，符合 B33-02/B35-02）。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow（B6-04）；probing 单飞锁内置位-检查（1034-1040）；probeSem cap 4（1058-1059）。tick 首探豁免（978）对零值 open 恒 false 不误触发。
- **删账号 memory-first 四步序 + 透传矩阵**：Remove → PurgeAccount → DeleteAccount → RevokeAccount 时序正确；四路 `accountExists`（目标写/课程读/手动报名退选/状态读）全覆盖。
- **零吞错落库点**：grep 实证全部 `s.store.*`/`d.Store.*` 调用均 `if err != nil { log.Printf }`，无 `_ =` 吞 SaveSuccess/SaveRefused/DeleteRefusedClass/DeleteSuccess/AppendLog/UpdateIDToken/SetTargetsForAccount。
- **凭据 AES-256-GCM**：nonce 前置 hex、`.master_key`/env 32 字节校验（F17-04）、enc: 前缀、旧明文拒绝加载——全部正确。
- **会话与票据**：32 字节 crypto/rand、12h TTL 惰性失效 + 5min 后台清扫、ticket 绑定账号 + 单次防重放（ConsumeTicket 在码校验前销毁为刻意设计，F13-m1 落盘）。
- **激活码原子性**：`used_uses < total_uses` 单条 UPDATE 原子、已激活不扣次（事务内查重 B5-08）、CreateActivationCodes 单事务、64bit 熵。
- **SQL 全面参数化**（modernc 占位符绑定）；数据库迁移 migrateAddPublishMeta 增量 ALTER + refuseLegacy 缺列清单与迁移列对应正确。
- **登录平**：管理员口令恒定时间比对 + 300ms 时延拉平、登录/激活独立限流桶、token 只显前 8 位（maskedToken 对 ≤8 位输出 `***`）。
- **编译/静态检查**：`cd backend && go build ./...` 与 `go vet ./...` 双通过（本次实证 exit=0）。
