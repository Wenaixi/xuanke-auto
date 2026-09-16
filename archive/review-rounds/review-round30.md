# 第 30 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main/web/embed）
> + web 全部模块。两个只读子代理并行产出发现，主 gate 逐条核实（读源码 + 推演真实触发路径）。
> 确认后端 2 项 MINOR + 前端 5 项（1 MAJOR + 4 MINOR），全部核实为真并修复。
> 说明：review30-backend 审查任务曾因网络 ECONNRESET 中断一次，主 gate 发消息恢复后续产出完整报告。

## 后端（2 项，确认修复）

### B30-01（MINOR）spawnChain 链顶无账号存在复核——删账号后为幽灵账号建 inflight + 逐课写日志
**缺陷**（review30-backend）：`submitAll` 已过滤 `ClientFor(acct)` 不存在的账号，但账号可在
`ClientFor` 取 client 之后被管理员删除（`Accounts.Remove` memory-first，B26-01）时，本链仍继续
为每门目标课锁内建 `inflight[acct][classID]=true` → 报"账号会话未建立，等待重新登录" →
`setStateLocked("failed")` + `AppendLog` 逐课落库（DB 单写者串行，删账号后日志表继续堆积）；
最后一课删除 inflight 条目但 `inflight[acct]` 空 map 残留。PurgeAccount 虽清 inflight/state，
但本链在 Purge 之后运行重写了这些内存态；`!ok` 失败分支此前无 ClientFor 复核（只有
`err==nil` 成功分支 1300 行有）。
**触发条件**：管理员删除账号的毫秒窗口内，该账号 spawnChain 恰好启动（黄金期 250ms tick 下极易命中）。
**修复**：spawnChain 链顶前置 `ClientFor(acct)` 判据（已删账号静默放弃整链）+ goroutine 内取
client 后再判一次（覆盖入口判据到取 client 之间的毫秒窗口，绝不给 nil client 调 SelectClass）。
删除原 `!ok` 失败分支（已由链顶判据覆盖，不再可能到达）。提交 `2cbf9b3`。

### B30-02（MINOR）GET /api 无尾斜杠时落入 SPA 兜底返回 index.html
**缺陷**（review30-backend）：router.go 注册 `mux.HandleFunc("/api/",...)` 显式 404 兜底，但
`/api`（无尾斜杠）不匹配该前缀 pattern，落到 main.go 的 `mux.Handle("/", SpaHandler)`——
SpaHandler 的 `path.Clean("/api")=="/api"` 不命中 `/`/`.` 分支，`fs.Stat(sub,"api")` 失败 → 回退
index.html（HTTP 200 text/html）。这是 B7-C4"未注册 /api/xxx 一律 404"的极小残余：`GET /api`
返回 200 HTML，安全扫描会误判"任意路径可 200"。
**修复**：SpaHandler 首行加 `if p == "/api" || strings.HasPrefix(p, "/api/") { http.NotFound(w, r); return }`——
所有 /api 前缀（含精确 /api）统一 404，与 router.go 的 /api/ 显式 404 同案。提交 `2cbf9b3`。

## 前端（5 项，确认修复）

### M30-01（MAJOR）开窗瞬间目标被守卫拦下后"发布恢复"不触发自动重试——改动永不落库
**缺陷**（review30-frontend）：防抖 effect 依赖 `[rev, selected, sessionToken, toast]`。开窗瞬间
平台清空 publishes 时消费时刻守卫置 `dirtyRef=true` return；publishes 恢复后组件重渲染但
rev/selected 引用/token/toast 全未变 → effect 不重跑 → 无新 timer → 改动永不落库。注释承诺
"等发布恢复再落库"但恢复不驱动 effect——用户设完目标盯着倒计时等开窗（本系统核心用法），
目标永远存不进后端、自动引擎按旧目标抢课，核心功能静默失效。
**修复**：effect 依赖加稳定布尔 `const hasPublishes = (data?.publishes?.length ?? 0) > 0`——发布
从空→非空时 effect 重跑 → 新 timer → 消费时刻守卫通过 → 正常保存；publishes 非空期间轮询
刷新布尔值不变、effect 不重跑，绝不把 400ms 防抖窗口无限重置。提交 `31e2a30`。

### M30-02（MINOR）handleBack 首次 flush 发起的 PUT 失败后组件已卸载——改动静默丢弃
**缺陷**（review30-frontend）：handleBack 先 `flushTargets()`——flush 在 savingRef=false 时走
`void saveNow()` 直接发 PUT（不发脏），handleBack 读 `!dirtyRef.current` break → onDone 同步
卸载；PUT 失败后 catch 走 `unmountedRef.current return`（F13-C2）→ 静默丢弃。F21-01/F26-01
只等"flush 前已有在飞 PUT / 退避 timer"，首次 flush 自己发起的 PUT 恰是唯一没等的路径。
**修复**：break 条件补 `&& !savingRef.current`——flush 发起的 PUT 在飞时进入下方 pendingSaving
等待循环，失败后 catch 走 scheduleRetry（组件未卸载，等待循环在 onDone 前），退避重试到成功
或 21s 收敛。提交 `31e2a30`。

### M30-03（MINOR）回显被 `rev>0` 无条件跳过——首次点击先于回显时旧目标被覆盖删除
**缺陷**（review30-frontend）：回显 effect `if (rev > 0) return`。进页后课程列表（/electives 内存
快照）先渲染，/state 首次加载慢于 electives（或首帧失败 retry 拉长到秒级）时，用户在
stateData 到达前先点选课程 → rev=1 → stateData 到达后回显被跳过，后端旧目标永远不进
selected → 防抖 PUT 只含新点课程 → 后端旧目标被静默覆盖删除（本意"添加一门"变"替换全部"）。
**修复**：跳过条件从 `rev>0` 改为 `echoedRef` 只合并一次 + setSelected updater 内"selected 已有
内容即返回"（保用户已操作课程不动、未涉及旧目标补进）。第 4 轮"清空后轮询旧 courses 再次
回填撤销清空"竞态语义由 echoedRef 延续。提交 `31e2a30`。

### M30-04（MINOR）max_count=0 时误显"已满额"——与后端 IsClassFull 判据不同源
**缺陷**（review30-frontend 可疑待核①，主 gate 核实）：前端 `isFull = c.selected_count >= c.max_count`，
max_count=0（未公布名额的新课程）时 `0>=0` 恒真 → 误显示"已满额"徽章 + 进度条染红。后端
`IsClassFull` 已用 `ce.MaxCount > 0 && ce.SelectedCount >= ce.MaxCount` 判满员（后端不误判），
前端判据与后端不同源。
**修复**：`isFull = c.max_count > 0 && c.selected_count >= c.max_count`（与后端同款判据，
0 表示名额未公布而非满员）。`fillRate` 已用 `!c.max_count` 判 0 不需改。提交 `31e2a30`。

### M30-05（MINOR）Admin generate/remove/删除账号三处缺在飞幂等守卫（F19-02 同族漏网）
**缺陷**（review30-frontend）：`generate` 无 `if (generating) return`，按钮 disabled 依赖渲染
落地，双击发出两个 POST /admin/codes → 激活码重复生成 count 个（后端落库两次，前端
setGenerated 被第二发覆盖）；`remove` 无守卫双击删除同一码第二发后端报"不存在"假失败 toast；
删除账号确认 onClick 无 `if (deleting) return`，双击第二发 DELETE 报"账号不存在"假失败。
**修复**：generate 入口加 `if (generating) return`；remove 加 `if (removing) return`（新增
removing state，finally 复位）；删除确认 onClick 首行加 `if (deleting) return`。提交 `31e2a30`。

## 可疑待核裁决（review30-backend）
- **scheduler.go:1034 `time.Since(t)` 本地钟 vs 对齐钟**：maybeRelogin 退避判读与 reloginAt 写入
  两端同为本地钟，内部自洽；与 tick 对齐钟判定无交互（退避只在此链内判读）。偏差相互抵消，
  无实际错误，仅契约文档层面不统一。**定不修，维持观察**。
- **scheduler.go:322 `lastSyncFailAt = time.Now()` 本地钟**：写读两端时间基不一致（偏差 ~640ms/
  30s 退避），已在历轮观察项（MINOR3 第 28 轮裁决维持），写侧持锁改 nowAlignedLocked 会重入
  死锁。**定不修，维持观察**。
- **emptyRunsFor 死代码**：全仓唯一引用只有定义自身，WindowClosed 直接读 state.EmptyProbeRuns，
  不可达但无害（观察项 21-06/22-06/23-06 延续）。**维持观察**。

## 可疑待核裁决（review30-frontend）
- **① max_count=0 误满员**：后端确认 `IsClassFull` 用 `MaxCount>0 && ...`（后端不误判），前端
  判据不同源 → **定修**，并入 M30-04。
- **② App.tsx:53 `setInAdmin(account===adminName)` 旧闭包**：login 只在管理员登录时传
  adminName 且渲染兜底 `inAdmin || current===adminName` 掩盖误判；后端登录响应 adminName
  恒等于当前账号名。**定不修，维持观察**。
- **③ onDeleted 用 loadSessions 非 state**：仅 localStorage 不可写降级场景（N4 隐私模式），
  M28-01 已注明。**维持观察**。

## review30-backend 已核无缺陷清单（详核）
- **?account= 透传校验链**：五处消费点（handleElectives/handleElectiveSelect/handleElectiveExit/
  handleSetTargets/handleState）全部经 allowAccountOverride + accountExists（凭据表逐账号比对）
  校验；handleAccounts 是唯一不带校验的（直接返回注册表列表，属既定管理员功能）。无第五路缺口。
- **WindowClosed 判据单源**：windowClosedLocked() 三判据单源，WindowClosed() 与 StateForAccount
  共用；probe 写入点与 EmptyProbeRuns 入账 10s 裕量对称（B26-03/B21-02）。
- **时钟基准**：探测时间戳（probe/ProbeForAccount/ProbeNow）、lastSubmit、markRateLimitedLocked
  写入侧全部对齐钟；lastSyncFailAt 本地钟属历轮观察项。
- **登录/激活/删除链路**：空壳清理判据（wasShell）、重登成功 ClientFor 复核（B21-03）、
  memory-first 删除顺序（B26-01）、MarkDone/RemoveDone 复核（B20-01）、spawnChain 成功分支
  复核（B18-M2）、令牌失效链顶短路（B23-03）、实时复核 doneHas 让位（B23-01）全在岗。
- **会话/票据**：ConsumeTicket 先校验后消费（F13-m1 防重放）、RevokeAccount 删除全部会话、
  session token 随机 32 字节。
- **store 落库**：SaveSettings 全量替换事务、DeleteAccount 6 表事务、ConsumeActivationCode
  原子扣次 + 已激活不双扣、main.go 恢复顺序（RestoreDone → RestoreTargets → RestoreRefused）。
- **识别引擎注入**：B29-01 模板透传 + SetVision 保留引擎 + Client.SetVision 保留本地引擎，测试齐备。
- **限流**：loginLimiter/activateLim 独立桶、XFF 可信反代 gate、gateWait 每分钟 2 次 doLogin。

## review30-frontend 已核无缺陷清单（详核）
- **401 吊销链**：快照式落盘（F8-01）、detail.session 归属（F10-05）、管理员代理特判与
  F25-01 targetAccount 清理、管理员自身被吊销后 inAdmin 残留由 account-reselect effect 兜底。
- **目标保存串行化**：saveNow json===lastJson 提前 return 经 finally 补发分支但第二轮
  dirtyRef 已清无死循环（逐层推演确认）。
- **StrictMode 双调**：pick 事件处理器不双调；防抖 effect 依赖 toast 为 useCallback 稳定函数，
  弹出不重置 timer。
- **handleBack 三轮收敛**：守卫拦下假清空脏块三轮后刻意放行（F21-01），等待循环 21s deadline
  无无限挂起。
- **App 渲染条件顺序**：targetAccount 代理优先、current===adminName 兜底、page 复位（M29-02）
  双路径覆盖主动登出与被动吊销。
- **构建**：全文件 import 均被使用（noUnusedLocals 下无残留）、无 TDZ/死代码（F18-01 模式全查），
  npm run build（tsc -b）通过。

## 回归
- backend：`go build ./... && go vet ./...` 全绿；`go test -race ./...` 全量通过。
- frontend：`npm run build`（tsc -b + vite）通过（M30-01~05 同一提交验证）。

## 观察项（本轮追加/延续）
- 本轮无新增观察项；历轮观察项全表延续（tsconfig 缺 strict、reloginBackoff 死分支、
  RestoreRefused 注释过时、lastSyncFailAt 本地钟、settings captcha_concurrency 无上限、
  probe per-account 近超时排队、PUT config 空变更、HTTP 探测单飞、Decrypt/RemoveFull/
  emptyRunsFor/syncFailedWindow 死代码、handleBack 极端失败路径、Dashboard 日志 key 缺
  account 维度、adminName 撞名、ddddocr 识别无净化等）。
