# R123 后端只读审查报告

审查对象：`backend/`（Go 后端），范围 `20c5882..HEAD`。
性质：纯只读走查，零仓库文件改动。

## 一、聚焦清单逐项结论

### 1. 身份防线矩阵第三十八轮闭合（零产品改动链第十八轮延续）

- **git 基线核验**：`git log --oneline 20c5882..HEAD -- backend/` 仅一条 `57bf401 fix(api): B110-01 手动报名/退选失败分支补库内审计日志`（handler.go +26 行、handler_test.go +133 行）——COUNT=1，与前十七轮同构延续成立。
- **sameClientFor 定义与调用点**：定义 scheduler.go:204，7 调用点 :850/:1489/:1521/:1551/:1571/:1600/:1635 全部在位，与 R122 基线零漂移。调用点覆盖 spawnChain 成功/失效/风控退避/窗口关闭/实时复核命中失效/确证满员六分支 + 链顶快照回退保护，与 CLAUDE.md 契约 31/37 全分支清点对齐。
- **写点全家福抽查**：按「换类」要求新抽查 done/rateLimited/full/inflight/state.Courses 五类：
  - **done/rateLimited/full/state.Courses**：spawnChain 成功分支（:1526-1541）与确证满员分支（:1639+）均先 `sameClientFor`（内含 ClientFor 存在性）再持 `s.mu` 写 map + `setStateLocked` + 落库，identity 复核通过才写。
  - **MarkDone/RemoveDone**（手动报名/退选，:1922/:1989）：均锁内先 `ClientFor(acct)` 存在性复核，已删账号静默放弃全部内存写与落库。
  - **releaseFullIfFreedLocked**（:1781）：锁内调用方持有 `s.mu`，仅写 full map + state.Courses，无网络往返故无身份竞态敞口，签名已注「需持锁」。
  - **TryAcquireSubmit**（:1896）：锁内 inflight 位占用/释放，纯内存互斥，不涉及身份写回。
- **偶发写点专项四类**：MarkTokenValid（:1306 锁序 reloginMu→s.mu，清 tokenValid/reloginFail/relogging，仅清不写）；TryAcquireSubmit 见上；releaseFullIfFreedLocked 见上；SubmitAll 主体 :1341，链内全部身份复核分支实证在读代码段。四类均无裸写点。
- **手动四路 accountExists**：handler.go :245（课程读）/ :295（手动报名）/ :387（手动退选）/ :563（状态读），四路全覆盖且各自返回明确「账号不存在」错误文案。
- **maybeRelogin 双侧**：决策侧 :1208 锁内 `ClientFor(acct)` 存在性复核（已删账号不发起重登、不写任何 map）；写回侧 :1254 重登完成时再次复核，已删账号静默放弃 tokenValid/reloginAt 内存写 + UpdateIDToken 落库。双侧闭合成立。

### 2. OBSERVE-117-01 知识位第六轮（scheduler.go:1254-1274）

逐行实证：重登完成 goroutine 回锁 → 先 `ClientFor(acct)` 存在性复核（:1254-1258）→ 通过后才 delete reloginFail / 置 tokenValid / **重新取 `s.clients.ClientFor(acct)` 再读当前注册表 token**（:1265-1266）落库——token 来源是「写入时刻的注册表当前身份」，已删账号在复核处整体退出，绝不串旧身份。第六轮确认在位，结构保证链持续成立。

### 3. OBSERVE-111-01/112-03 盯守第十二轮

- handler.go:377 `_ = d.Sched.MarkDone(...)` 与 :457 `_ = d.Sched.RemoveDone(...)`：两者返回值是 `error`，成功路径仅用于触发同步状态写，错误在函数内部记日志（MarkDone/RemoveDone 全部分支 log.Printf 记录落库失败，:1946/:1974/:2021/:2027 等），非吞错。
- 双文案动作维度：select 路（:352/:364/:371）与 exit 路（:433/:442/:449）六处文案均带动作维度「报名/退选」，可辨性保持。第十二轮维持零漂移。

### 4. B110-01 审计链第十三轮（转正契约后）

- 手动失败六处 AppendLog 全部在位：select :352/:364/:371、exit :433/:442/:449（本轮唯一新增代码，经 git diff 逐行核验，错误落库均 `log.Printf` 记录不吞错）。
- 成功路径审计行在位：手动 MarkDone（scheduler.go:1976）、RemoveDone（:2029）、自动链成功（:1532）。
- 自动链失败族齐位：失效 :1504、风控 :1558、实时复核失效 :1614、非满员失败 :1662、满员 :1770。
- 零吞错穷举扫描：`grep '_ = .*(AppendLog|SaveSuccess|SaveRefused|DeleteSuccess|DeleteRefusedClass|UpdateIDToken|DeleteRefused)'` 全仓**零命中**。仅存的 `_ =` 是 MarkDone/RemoveDone 两个不返回落库错误的封装调用（错误已内记日志），符合契约 17 语义。
- 新增 TDD 测试：`TestManualElectiveFailureAppendsLog`（业务失败双路留痕）+ `TestManualElectiveReadErrAppendsLog`（read 类结果未知最需留痕），修复前红修复后绿，语义与自动链对称。

### 5. O105-01 抖动基线

- socketPreheat（zhidao/client_test.go:25 + captcha_test.go:17/:52 + sanitize_test.go:97 调用）+ readyProbe（zhidao/client_test.go:88、captcha_test.go:29/:79、api/handler_test.go:185、accounts/manager_test.go:26）夹具零漂移，账户测试注释标明「8 轮全绿实证无残余」。
- `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/`：**ok**（zhidao 3.513s / accounts 1.426s，exit 0）。

### 6. 新契约角度：调度状态机异常自愈会话

- **rateLimited 退避**：入账 markRateLimitedLocked（:1710，对齐钟写入截止时刻，map 懒建）→ 等待 isRateLimitedLocked（:1691 读侧超时即 delete 自愈；tick 守卫 :1442 跳过该课）→ MarkDone/RemoveDone（:1956/:2040 区）与 SetTargets（:501 PurgeAccount 时整账号清）显式清理。生命周期完整、状态单一来源 `rateLimited[acct][classID]` 截止时刻，无旁路写入。
- **syncFailStreak**：入账 maybeSyncClock 失败分支 ++（:371，持锁）→ ≥3 复位 clockOffset=0（:384-386，**绝不在此清零 streak**，防旧实现值域 {0,1,2} 判据不可达）→ 自愈：同步成功统一清零（:391）；判定侧 windowClosedLocked 判据 2 带「开放时间已过」附加条件（:925），且复位只清 offset、streak 保留至同步成功。单一来源成立。
- **EmptyProbeRuns**：入账 probe 空快照 +1（:1155，**含已过开窗点 +10s 裕量**、非空快照即归零 :1158）→ 判定侧判据 3（:935，open 非零 + 从未开窗 + runs≥3）→ 自愈：平台真开窗下发非空快照后归零 + WindowOpened 置位推翻判据 3。单次 open 快照复用（:924）防亚毫秒热改分叉。
- 三机制均有「入账→等待→自愈/复位」完整生命周期且状态单一来源，与 CLAUDE.md 契约 2/18 一致。

## 二、分级发现

- **严重**：无。
- **中等**：无。
- **轻微/观察项**：无新发现。

## 三、必查项结论

| 项 | 结论 |
| --- | --- |
| 身份防线第三十八轮 | 闭合，sameClientFor 七调用点零漂移 |
| OBSERVE-117-01 | 第六轮确认在位（落库取当前注册表 token 支撑链成立） |
| OBSERVE-111-01/112-03 | 第十二轮维持，`_ =` 恒 nil 非吞错，双文案动作维度保留 |
| B110-01 审计链 | 第十三轮零漂移，新增手动失败六处 AppendLog + 两条 TDD 测试实证 |
| O105-01 抖动基线 | socketPreheat/readyProbe 夹具零漂移 + 定向 race 全绿 |
| 异常自愈会话 | 三机制生命周期完整、状态单一来源 |
| 契约 20 | Go 源码与测试零轮次前缀标签残留（grep `第\s*\d+\s*轮` 仅命中普通用语与历史注释） |

## 四、验证表

| 命令 | 结果 |
| --- | --- |
| `git log --oneline 20c5882..HEAD -- backend/` | 仅 `57bf401` 一条（COUNT=1） |
| `go build ./...` | 通过（exit 0） |
| `go vet ./...` | 通过（exit 0） |
| `go test -race -count=1 -p 1 ./internal/zhidao/ ./internal/accounts/` | ok（3.513s / 1.426s） |
| 零吞错穷举 grep | 全仓零命中 |

## 五、代码动点

- 本轮唯一 backend 变更：`57bf401`（handler.go +26 行 / handler_test.go +133 行，B110-01 手动失败审计日志补齐）。已逐行核验，不触碰任何身份防线代码。
