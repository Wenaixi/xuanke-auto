# R114 后端只读审查报告（身份防线矩阵第二十九轮闭合）

## 头部

- 审查对象 HEAD：`630deaa`（docs(review): R113 双 findings + 收尾总结，进度 114/256）
- 审查方式：只读走查 + git 实证 + 定向 `go test -race` 验证（全程零仓库文件修改，仅本报告落盘）
- 聚焦范围：backend/（scheduler.go 身份防线矩阵 + api handler.go 手动路径审计链 + zhidao/api 测试夹具 O105-01 + cmd 三工具契约）
- 审查时间：2026-09-23 约 25 分钟

## 分级发现

### CRITICAL / MAJOR / MINOR

无。稳定期第九轮连续零产品改动，本轮无任何够格立条缺陷。

### OBSERVE（延续）

无新增 OBSERVE。前序延续观察项（OBSERVE-111-01、OBSERVE-112-03）本轮确认零漂移，见下。

## 必查项逐条结论

### 1. 身份防线矩阵第二十九轮闭合 —— 通过

**COUNT 实证**：`git log 20c5882..HEAD -- backend/` 输出唯一一条 `57bf401 fix(api): B110-01 手动报名/退选失败分支补库内审计日志`。`git show --stat 57bf401` 确认其改动仅 `backend/internal/api/handler.go`(+26) 与 `backend/internal/api/handler_test.go`(+133)，即 B110-01 审计修复（非产品行为改动，R110 起约定不计入产品改动链）。**R110-R113 零产品改动链延续，COUNT=1，第二十九轮闭合。**

- **sameClientFor 7 调用点零漂移**：定义 `:204`；调用点 `:850`（ProbeForAccount 回写段）/`:1489`（失效）/`:1521`（成功）/`:1551`（风控）/`:1571`（窗口关闭）/`:1600`（实时复核统一）/`:1635`（确证满员）。与 R113 记录的行号完全一致，逐点 grep 实证。
- **写点全家福 12 类防线对应**（grep + 走读逐类核验）：
  - `openTimeDetected[acct]/[*]`：唯一写点 `:856`（ProbeForAccount 锁内回写段，`sameClientFor` :850 前置守卫）与 `:1108`（probe 全校帧，probe 入口取 client 后同类防线区域）→ 已守卫
  - `acctData[acct]`+`acctDataAt[acct]`：`:863/:864`，同锁内 `sameClientFor` :850 守卫 → 已守卫
  - `tokenValid`：写点 `:1241`（maybeRelogin 决策侧，`:1208` ClientFor 复核之后）、`:1262`（重登成功分支，`:1254` 写回侧复核之后）、`MarkTokenValid :1316`（手动登录成功路径，锁序 reloginMu→s.mu）→ 已守卫
  - `reloginAt/reloginFail/relogging`：决策侧 `:1231/:1233/:1236` 与写回侧 `:1261` 均在同族复核之后 → 已守卫
  - `inflight`：`:1468-1471`（spawnChain 申请）、`:1490/:1501/:1513`（清位，全在身份复核后或公共清位路径）→ 已守卫
  - `done`：`:1529`（成功分支，`:1521` sameClientFor 之后）、`:611/:615`（RestoreDone，启动期注入无竞态窗口）→ 已守卫
  - `refused`：`:634`（RestoreRefused 启动期）、`:2011`（RemoveDone，`:1995` ClientFor 复核之后）→ 已守卫
  - `full`：`:1575/:1639`（窗口关闭/确证满员分支，`sameClientFor` 前置）、`:1767`（markFullLocked 内部）→ 已守卫
  - `rateLimited`：`:1555`（风控分支，`:1551` sameClientFor 之后）、`:1714`（markRateLimitedLocked）→ 已守卫
  - `Courses` 状态（setStateLocked）：`:1474/:1502/:1530/:1556/:1612/:1660` 全数位于各分支身份复核之后或链顶已过滤 → 已守卫
- **偶发写点专项**：`MarkTokenValid :1306`（锁序 reloginMu→s.mu，delete tokenValid/reloginFail/relogging）；`TryAcquireSubmit :1896`（inflight 互斥 + 手动标记，释放路径 delete）；`releaseFullIfFreedLocked :1781`（`:1451` spawnChain 调用，快照不满解 full 回 pending）；`submitAll :1342`（小写，任务单写 SubmitAll 指此——`:1355` ClientFor 过滤后 spawnChain，写 lastSubmit 用对齐钟，读侧判据同一时间基）→ 全部在对应防线内
- **手动四路 accountExists**：`:245`（handleElectives）/`:295`（handleElectiveSelect）/`:387`（handleElectiveExit）/`:563`（handleState），定义 `:1099`，与 R113 记录一致 → 零漂移
- **maybeRelogin 双侧**：决策侧 `:1208` `if _, ok := s.clients.ClientFor(acct); !ok`（锁内、任何 map 写入前）；写回侧 `:1254` 同款复核（成功后不落 tokenValid/UpdateIDToken）。R43-01 双闭合要求满足 → 零漂移

### 2. OBSERVE-111-01/112-03 盯守（第三轮）—— 通过

- `handler.go:377 _ = d.Sched.MarkDone(...)` / `:457 _ = d.Sched.RemoveDone(...)` 恒 nil 非吞错：返回值虽在 handler 丢弃，但两实现内全部落库失败均 `log.Printf`（MarkDone `:1974/:1977`，RemoveDone `:2021/:2027/:2030`），零 `_ =` 吞错落库点，契约 17 维持。
- 手动失败分支与自动链同文案 + 动作维度可区分：手动 select 路径 `:352/:364/:371` 用 `"select"`、exit 路径 `:433/:442/:449` 用 `"exit"`；自动链统一 `"select"`（scheduler.go `:1504/:1558/:1614/:1662`）。文案同语义、动作维度可区分 → 零漂移。

### 3. B110-01 审计链持续盯守（第四轮）—— 通过

- 手动失败三分支 AppendLog 六处全部在位：`grep -c 'req.ClassID, "select"'` = 3、`grep -c 'req.ClassID, "exit"'` = 3。
- 六处全部 `if err := ...; err != nil { log.Printf }` 模式，零 `_ =` 吞错。
- 成功路径审计行未回归：MarkDone 内 `:1976` `AppendLog(..., "select", msg, true)`、RemoveDone 内 `:2029` `AppendLog(..., "exit", "手动退选成功...", true)` 在位。57bf401 修复经 R111 回首轮 + R112/R113 第二轮/第三轮零漂移，本轮为第四轮 → 维持。

### 4. B110-02 观察复核 —— 通过

- reloginAt 本地钟写（`:1231` 决策侧 / `:1261` 写回侧 `time.Now()`）与读（`:1219/:1227` `time.Since(t)`）同一本地钟基，自洽。该字段语义是"节流/退避的挂钟计时基准"，不参与窗口判点（那由对齐钟 `nowAlignedLocked` 负责），两套时间基职责分离无混用 → 维持。

### 5. O105-01 抖动基线 —— 通过（含一次低频残余记录）

- `socketPreheat`（client_test.go:25-30 预创建-关闭 127.0.0.1 套接字；TestMain :39-42 包级调用 + 逐测试调用双保险）与 `readyProbe`（handler_test.go:185，200ms×10 轮询 + 显式 2s 超时）逐字符零漂移，与 R113 基线一致。
- 定向 `go test -race -count=1 ./internal/api ./internal/zhidao`：zhidao 全绿；api 首轮 1 FAIL——`TestHandleElectivesSelectUnauthorizedRelogin`（16.98s）命中 `dial tcp 127.0.0.1:62892 connectex`（Windows 回环冷启动 TIME_WAIT 残余）。**重跑该单测绿（1.561s）、重跑 api 全包绿（22.746s）**——符合既有基线"跑失败但重跑绿记低频残余不立条"。未跑 scheduler -race（全量由主控在归档后跑）。

### 6. 新契约角度（本轮自选：cmd 三工具契约逐点核对）—— 通过

- **cmd/bench**：串行延迟测试，`-token` 注入 Bearer 说明 401 拒绝路径 vs 真实业务路径区分；`-n ≤0` 回退默认防除零。契约无偏离。
- **cmd/probe**：token 从环境变量 `XUANKE_PROBE_TOKEN` 读取（替代硬编码，安全审计契约）；`15s 超时 + zhidao.SharedTransport()` 与生产 HTTP 契约族对齐；`url.QueryEscape(token)` 拼 `idToken`；输出只打 body 前 600 字节不泄露 token（契约 17 满足）。
- **cmd/logintest**：`--limit/--wait` 参数 + `-limit` 收敛到真实账号数防 slice 越界；默认并发 1 尊重平台登录限流；识别引擎跟随运行时配置（ddddocr→本地→Vision 回退链）；token 只显前 8 位 `token[:min(8, len(token))]`。契约无偏离。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 20c5882..HEAD -- backend/` | 唯一 `57bf401`（B110-01 审计修复） |
| `git show --stat 57bf401` | 仅 handler.go(+26)+handler_test.go(+133) |
| `grep sameClientFor/clientIdentity scheduler.go` | 定义 :204、7 调用点 :850/:1489/:1521/:1551/:1571/:1600/:1635 全在位 |
| `grep accountExists handler.go` | :245/:295/:387/:563 四路 + 定义 :1099 |
| `grep AppendLog handler.go` | 手动失败 select 3 + exit 3 = 6 处，全部 `if err != nil` 记日志 |
| `grep -c 'req.ClassID, "select"' / '"exit"' handler.go` | 3 / 3 |
| `go build ./...` | 通过（exit 0） |
| `go vet ./...` | 通过（exit 0） |
| `go test -race -count=1 ./internal/api ./internal/zhidao` | zhidao ok；api 首轮 1 FAIL（TestHandleElectivesSelectUnauthorizedRelogin connectex 低频残余） |
| 重跑 `-run TestHandleElectivesSelectUnauthorizedRelogin` | ok（1.561s） |
| 重跑 `go test -race -count=1 ./internal/api` | ok（22.746s） |

## 已核无缺陷清单

1. 身份防线矩阵第二十九轮：7 调用点 + 12 类写点 + 4 偶发写点 + 手动四路 + maybeRelogin 双侧，行号零漂移。
2. OBSERVE-111-01（MarkDone/RemoveDone 返回值丢弃非吞错）第三轮零漂移。
3. OBSERVE-112-03（手动/自动同文案双日志动作维度可区分）第三轮零漂移。
4. B110-01 手动失败审计链（六处 AppendLog + 零吞错 + 成功路径审计行）第四轮零漂移。
5. B110-02 reloginAt 本地钟读写同基自洽。
6. O105-01 socketPreheat/readyProbe 夹具零漂移，api 全包 race 重跑绿（低频残余不立条）。
7. cmd 三工具契约与 CLAUDE.md 文档对齐（token 只显前 8 位、参数边界防御、生产 HTTP 契约族对齐）。

## 结论

身份防线矩阵第二十九轮闭合：R110 起零产品改动链延续至 COUNT=1（唯一 `57bf401` 为 B110-01 审计修复，非产品行为改动）。sameClientFor 7 调用点与写点全家福 12 类防线行号逐点实证零漂移。OBSERVE-111-01/112-03 盯守第三轮确认零漂移，B110-01 审计链第四轮确认零漂移。本轮无 CRITICAL/MAJOR/MINOR 缺陷，无新增 OBSERVE。唯一注意点：api 包首轮 race 又命中一次 connectex 低频残余（重跑绿），属 O105-01 已知基线内，不立条。
