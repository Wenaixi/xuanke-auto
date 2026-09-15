# 第 29 轮全模块审查记录（2026-09-15）

> 审查范围：backend（api/scheduler/accounts/zhidao/store/session/runtime/config/secure/db/main）
> + web 全部模块。两个只读子代理并行产出发现，主 gate 逐条核实（读源码 + 推演
> 真实触发路径）。确认后端 2 项（1 MAJOR + 1 MINOR）+ 前端 2 项 MINOR
> （均核实为真并 TDD 修复）。

## 后端（2 项，确认修复）

### B29-01（MAJOR）SetRecognizer 模板引擎不更新——新 ensure 客户端识别引擎恒 nil 硬故障
**缺陷**（review29-backend）：`accounts.Manager.SetRecognizer` 只遍历**当前已有**客户端注入
识别器，从不更新 `m.vision.recognizer`；`SetVision` 更新 `m.vision` 时 `cfg.recognizer`
恒为 nil。因此 `m.vision.recognizer` 从启动起恒 nil，任何**之后新建**的客户端在
`zhidao.New(m.baseURL, m.vision)` 里只能走 `APIKey != ""` 兜底 Vision——ddddocr 引擎对
新账号一律不生效。完整触发路径：按文档配置 `XUANKE_CAPTCHA_ENGINE=ddddocr`（SF_API_KEY
留空=ddddocr 典型用途），启动 initCaptchaAtStartup 注入现有客户端成功；第一个**新学生
账号** `/api/login` → `ensure` 走 New → 模板 recognizer nil 且 APIKey 空 → 识别报
"未配置验证码识别引擎"×3 登录失败——**系统从第一个新账号起无法登录任何新账号**
（Restore 既有账号被注入过引擎，掩盖问题在重启前不被发现）。
**修复**：SetRecognizer 同步写模板（新增 `VisionConfig.WithRecognizer/Recognizer` 导出
访问器跨包操作未导出字段，不破坏 client.go 包内直接访问）；SetVision 赋值前保留模板
当前引擎（与 `Client.SetVision`"绝不挥动引擎切换"语义对齐）；补 `Client.CurrentRecognizer`
测试读取器。**TDD**：`TestNewClientAfterSetRecognizerGetsEngine` 红灯（新账号登录报
"未配置验证码识别引擎"，识别 3 次全败）→ 绿灯 + "SetVision 不清模板"对偶守卫。
**验证**：go build+vet 绿，accounts/zhidao/api 三包测试通过。commit `ed64aec`。

### B29-02（MINOR）StateForAccount 镜像 WindowClosed 兜底判据——window_closed 字段与挂起状态分叉
**缺陷**（review29-backend）：`state.WindowClosed` 字段只在 probe() 主判据写入；
`WindowClosed()` 方法另两条兜底判据（B19-01 时钟连续失败≥3 / B20-02 幽灵窗口
EmptyProbeRuns≥3）返回 true 时不回写字段。`StateForAccount` 直接浅拷贝 `s.state` →
`/api/state` 下发 `window_closed=false` → 前端横幅仍显示倒计时/"已开放"、日志与课程轮询
维持 10s/2s 高频（F9-07 降频失效）——展示与实际挂起状态分叉。
**修复**：抽 `windowClosedLocked()` 三条判据单源，StateForAccount 与 WindowClosed() 共用，
杜绝两套真相。**TDD**：`TestStateForAccountMirrorsWindowClosed` 红灯（幽灵窗口兜底
EmptyProbeRuns=3 时字段仍 false）→ 绿灯（三判据全镜像 + 非关闭不误报）。
**验证**：go build+vet 绿，scheduler/api 全量测试通过。commit `fc0ef5f`。

## 前端（2 项，确认修复）

### M29-01（MINOR）手动报名/退选在飞操作改 Set 按课程独立跟踪——单值被并发不同课程互相覆盖
**缺陷**（review29-frontend）：Select.tsx 的 `actionLoading` 是单值 `useState<number | null>`
（47 行），报名与退选共用，F26-03 幂等守卫 `if (actionLoading === c.id) return` 只在
"同一课程双击"场景有效。真实触发链：课程 A 报名在飞 → 用户点课程 B，
`setActionLoading(B)` 覆盖 A 的标记 → A 的 finally `setActionLoading(null)` 把 B 的在飞态
一并抹掉（单值无法区分归属）→ B 按钮恢复可点 → 用户再点 B 发第三发 → 后端
TryAcquireSubmit 拒绝 → finally invalidateQueries 照跑 → **假失败 toast 在同一课程维度
之外复发**（正是 F26-03 想消除的缺陷换一种课程组合重演）。
**修复**：`actionLoading` 改 `ReadonlySet<number>`；守卫 `has(c.id)`；置位 `new Set(prev).add(c.id)`
（函数式）；finally `new Set(prev).delete(c.id)` 只清自己的 id（报名/退选共享同一 Set，
互不覆盖）；按钮 disabled/文案/Esc 守卫/F21-03 确认按钮 disabled 全部同步 `has()`。
**验证**：`npm run build`（tsc -b + vite）全绿。commit `b79991a`。

### M29-02（MINOR）登出/401 吊销未重置 page 视图态——重登后跳过 Dashboard 直接掉进选课大厅
**缺陷**（主 gate 复核自 review29-frontend 说服链）：App 组件永挂载（Login 只是条件渲染
分支），`page` 不随 sessions 清空而重置。主动登出路径：学生在选课大厅（page="select"）
点登出 → 重新登录 → page 残留 "select" → 渲染分支跳过 Dashboard 直接掉进选课大厅
（与 F15-07 targetAccount 同族，D4"回登录页"契约被凿穿）。401 被动吊销路径同源：
唯一账号在选课大厅被吊销 → account-reselect effect `setCurrent("")` 回登录页 → 重新登录
后 page 仍未复位。F15-07 只清了 targetAccount，page 是同类视图态遗漏。
**修复**：两处复位——logout() 分支 `setPage("dashboard")`；account-reselect effect
`accounts.length === 0` 分支同样 `setPage("dashboard")`（吊销不经 logout，两路对称）。
**验证**：`npm run build` 全绿。commit `1a014d3` + 补丁 `2087ebf`。

## 可疑待核裁决（review29-backend）
无达到需主会话推演门槛的项目——删除竞态三链闭环、时钟对齐、窗口状态机、透传校验矩阵
均已核闭环。

## 可疑待核裁决（review29-frontend）
- **① logout 不重置 page → 确认为真实缺陷，M29-02 修复**（上文）。
- **② tsconfig.app.json 缺 `strict` → 维持观察**（S27-02 延续）：系统性类型安全隐患，
  长期维持观察；开 strict 是全量工作（当前代码必有解构可选链/隐式 any 等报错），
  且已有 tsc -b 兜底类型基础错误，非本轮可顺手落地。
- **③ 守卫拦截吞改动与"下次进入恢复"注释语义落差 → 定不修**：flushTargets/防抖回调
  置 `dirtyRef=true` 的脏块不持久化，卸载即丢（handleBack 循环 3 轮后 onDone）。
  推演：守卫拦下的是"发布缺席/错位 = 假清空"（F15/F16/F17 链刻意安全方向）——
  发布缺席属平台过渡态（开窗瞬间清空，下一轮防抖/下次改动正常落库）；publishes 恒空
  （窗口关闭）时目标本无实际提交意义，"绝不强行假清空"优先于"尽量保存"（F21-01
  注释明示）。无真实缺陷。

## 观察项（本轮追加/延续）
- tsconfig 缺 strict（S27-02/S29-② 延续）。
- 历轮观察项全表延续：后端——reloginBackoff backoffMax=10min 死分支、RestoreRefused
  注释过时（行为正确）、lastSyncFailAt 本地钟 vs 对齐钟（B20-03 同族）、settings 恢复
  captcha_concurrency 无上限校验、probe per-account 近超时窄窗排队、PUT config 空变更
  先落库（23-11）、HTTP 探测单飞（22-01）、Decrypt 死字段/RemoveFull 死方法/emptyRunsFor
  死代码/syncFailedWindow 写而不读；前端——handleBack 极端失败路径刻意兜底、Dashboard
  日志 key 缺 account 维度（令牌唯一性+服务端过滤双覆盖）、handleBack 等待期用户继续
  操作边缘安全方向、手动报名假失败 toast 存在性已并入 M29-01。

## review29-backend 详核报告补全（主 gate 复核，并入本档）
- **已核无缺陷 15 项**（详核清单）：① 删除账号三链竞态闭环（B26-01 memory-first 顺序下
  PurgeAccount 持 s.mu 天然排在任何持锁落库临界区后、DeleteAccount 恒在其后，库行写入
  必然先于 DeleteAccount 被事务清掉，无幽灵复活）；② spawnChain 提交链（done/full/
  rateLimited/inflight/refused 判定顺序、B23-03 tokenValidForLocked 短路语义、
  B23-01 满员分支 doneHas 让位、chainMu 链去重、inflight 各分支清理 sync.Once 幂等）；
  ③ 时钟对齐链（lastSyncTime 只成功推进、lastSyncFailAt 30s 退避、syncing 单飞、
  F12-B1 无客户端复位、B21-01 streak 真实可达）；④ 窗口状态机（prevOpened 先捕获再覆写、
  B26-03 双裕量对称、B20-02 入账/归零/自愈、B11-A1 零值挂起、tick 守卫五顺序）；
  ⑤ 探测节流（lastProbe 只归 probe()/ProbeNow、probing 持锁原子、probeSem cap 4）；
  ⑥ ?account= 透传校验矩阵四路（B15-M4/B26-02/B27-01/B27-02 全 accountExists 同源）；
  ⑦ 会话与票据（12h TTL + 5 分钟清扫、票据单次防重放、requireAuth/requireAdminSession
  令牌提取一致、RevokeAccount 立即吊销）；⑧ 激活码（单事务 UPDATE 原子条件不超卖、
  已激活先扣次再查回滚、批量单事务、crypto/rand 失败 panic、16 位 hex 不可枚举 + 限流）；
  ⑨ SQL 注入面（全部参数化占位符，columnExists 拼接均来自编译期常量数组）；
  ⑩ AES-GCM 加密链路（secureEncrypt 未注入报错、随机 nonce、.master_key 32 字节同校、
  加载严格校验 enc: 前缀）；⑪ CSRF 与限流（requireJSONBody 强制 application/json、
  login/activate 独立桶、XFF 只在可信反代+回环启用）；⑫ 登录失败清理
  （wasShell 在 Login 前快照，B24-01 契约保真）；⑬ 路由（/api/ 通配 404 精确注册、
  SpaHandler path.Clean 防穿越 + fs.Sub 内不越界）；⑭ 时间基准（三处探测时间戳
  nowAligned() 统一、markRateLimitedLocked 与读侧同源、relogin 族本地钟内部自洽）；
  ⑮ open_time 契约（FormatOpenTime 空串零值、B21-04 前置校验整体拒绝、reparse 零值挂起）。
- **观察项新增 4 条（其余为历轮延续，全表见四节）**：
  - writeJSON 不写 HTTP 状态码：业务错误恒 200 + body code（B7-C4 约定），自洽设计。
  - m.vision.recognizer 恒 nil 伴生设计：SF_API_KEY 非空时 New 的 APIKey 兜底保 Vision，
    无行为影响（主触发路径只在 ddddocr 部署成立，B29-01 已修）。
  - ddddocr 识别结果不做 normalizeCaptchaText 净化（仅 VisionRecognizer 挂净化）：
    字符集输出通常干净稳定，识别错误由 Login 提交被拒分支刷新验证码重试兜底，低风险维持观察。
  - LocalDdddOcrRecognizer 子进程 stderr 回传可能含本机 Python 路径：仅本地引擎报错
    路径出现，泄露面极小，维持观察。
  - 学生账号名与 adminName 撞名：学号恰等于 XUANKE_ADMIN_NAME 时教务登录被管理口令
    分支抢先（密码不符报"管理口令错误"），管理员改名可解，边缘场景维持观察。
  - gateWait/gateCond 最坏等待 60s：挂起的是重登 goroutine，不阻塞调度器主循环，有界可接受。

## 已核无缺陷（后端，主 gate 复核 + review29-backend 详核）
- 删除账号三链竞态闭环：B26-01 memory-first 顺序（Accounts.Remove → PurgeAccount →
  Store.DeleteAccount → Sessions.RevokeAccount）下，PurgeAccount 持 s.mu 天然排在任何
  持 s.mu 落库临界区之后、DeleteAccount 恒在 PurgeAccount 之后——SaveSuccess/
  SaveRefused/UpdateIDToken 的库行写入必然先于 DeleteAccount，已删账号写回行总是被
  事务清掉；内存侧由 PurgeAccount 兜底，无幽灵复活路径。
- spawnChain 提交链：done/full/rateLimited/inflight/refused 判定顺序正确；B23-03
  tokenValidForLocked 短路语义确认；B23-01 满员分支 doneHas 让位与"未现满员"分支对称。
- 时钟对齐/窗口状态机/透传校验矩阵（B27-01/02 accountExists 四路）全部闭环。

## 已核无缺陷（前端，主 gate 复核）
- 目标保存串行化链：savingRef/dirtyRef/rev 驱动 + flushTargets/handleBack 补发收敛
  （F21-01/F26-01 时序读码复验，pendingSaving 判据含在飞 PUT 与退避 timer 双闸）。
- 假清空守卫链：F15-01→F16-01→F17-02 消费时刻判据三处守卫、publishes 缺席/联查空集/
  publish_id 漂移三路分类置脏，无绕过路径。
- 回归回显 effect：F19-01 幽灵 publish_id 过滤 + rev>0 短路 + prev 有内容即返回。
- 401 吊销链：快照式落盘、session 反查归属、F25-01 管理员自我吊销清代理态齐备。

## 回归
- backend：`go build ./... && go vet ./... && go test -race ./...` 全量通过。
- frontend：`npm run build`（tsc -b + vite build）通过（三个前端提交均验证）。