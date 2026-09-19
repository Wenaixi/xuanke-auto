# round39 前端审查原始发现

审查范围：`web/src/` 全部 `.ts/.tsx`（App.tsx / api/client.ts / routes/Select.tsx / Login.tsx / Dashboard.tsx / Admin.tsx / lib/useTickingCountdown.ts / lib/utils.ts / types.ts / main.tsx / components/ui/*）。同时对照 `backend/internal/api/handler.go`、`internal/scheduler/scheduler.go`、`internal/session/store.go`、`internal/store/store.go` 核验前后端契约（`?account=` 穿透、`window_closed` 下发、激活票据、目标保存等）。`npx tsc -b` 与 `npx tsc --noEmit -p tsconfig.app.json` 均零错误。

本轮为第 39 轮（前 38 轮已确认修复并落 CLAUDE.md 决策锚的项不重复列出，只报"已修但仍有残留缺口"或"全新发现"）。

---

## CRITICAL（数据丢失）

### C-1. 防抖/flush 目标保存链只校验"产出集合"，不校验 selected 的旧 publish_id key——发布集合重建后"添加一门"变"整包替换"，用户勾选过的旧发布目标被静默覆盖删除

- **文件路径:行号**：`web/src/routes/Select.tsx:558-619`（防抖 build + 消费守卫）、`web/src/routes/Select.tsx:443-475`（flushTargets 同源）
- **严重级**：CRITICAL
- **一句话问题**：`build()` 只遍历 `publishesRef.current`（最新发布集合）与 `selected` 联查，产出 targets 后三道守卫（发布缺席 / 联查空集 / `targetsUseCurrentPublishes`）全部只校验"产出集合本身"；`selected` 里键为**已消失的旧 publish_id** 的课程被静默丢弃，而产出非空时校验全部放行 → 整包 PUT 把后端旧目标替换成"只有新发布课程"的目标。
- **触发场景推演**（自洽时序）：
  1. 用户在窗口开放前勾选目标：`selected = { P1: [篮球], P2: [排球] }`，防抖 400ms 已保存到后端（`/state.courses` 有这两门）。
  2. 平台发布集合整体重建（开窗瞬间平台清空又恢复、publish_id 全变——F18-03 注释实证；或页面 reload 后 /electives 数据源变化）。此时 `data` 更新 → `publishesRef.current` 变成新集合，但 **selected 的旧 key P1/P2 不会被任何 effect 清理**：回显 effect 只按新 `currentIds` 过滤 courses 合并路径，不清理用户已触碰的 key（F19-01 只防"幽灵 id 进 selected"，不清已存在 key）。
  3. 重建后 400ms 内用户在新发布 P9 下又点选了一门新课：`pick` → `setSelected({...selected, P9: [新课]})` → `rev++` → 防抖 effect 重跑挂 400ms timer。
  4. timer 触发：`build()` 遍历 `publishesRef.current`（新集合），`selected[新发布]` 大多为 undefined，只产出 `[P9: 新课]`。`selectedCount=3 > 0` → F17-01 空集守卫不拦；`targetsUseCurrentPublishes([P9课])` → P9 在新集合内 → **放行**。
  5. `saveNow` → `PUT {"targets":[P9新课]}` → 后端目标从 `[P1篮球, P2排球]` 被整包替换为 `[P9新课]`。用户明确勾选过的篮球/排球目标永久丢失，后台自动抢课不再抢它们。
  - 本质：这是"添加一门变替换全部"（F7 系列注释反复描述要防的场景）的**发布重建变体**。前轮守卫只堵住了"产出空集"（F17-01）和"产出含漂移 id"（F16-01）两种形态，堵不住"selected 旧 key 被静默丢弃、产出恰好非空且全数合法"的部分覆盖形态。
- **建议修法**（一行）：在 build/flushTargets 消费时刻增加前置校验——`Object.keys(selected).some(k => selected[k].length>0 && !currentPublishIds.has(Number(k)))` 即置脏跳过（与回显 effect 的 currentIds 过滤同判据，覆盖"selected 残留旧发布 key"场景）；或更简单：`build()` 前先断言 `selected` 的全部非空 key ⊆ `publishesRef.current` 集合。

---

## MAJOR（明确错误行为）

### M-1. 管理员删除"自己"（管理员账号）后未退出管理态，学生令牌继续渲染 Admin 面板 → 连环 401 误删

- **文件路径:行号**：`web/src/App.tsx:124-133`（onDeleted）
- **严重级**：MAJOR
- **一句话问题**：`onDeleted` 里 `if (current === acct) setCurrent("")` 只重置当前账号，未像 logout/onUnauthorized 那样处理"被删账号恰是管理员"时同步 `setInAdmin(false)`；账号管理列表（`/api/admin/accounts`）含管理员自身，删除自己后 `inAdmin` 残留 true。
- **触发场景推演**：
  1. 管理员 `admin` 在 Admin → 账号管理 → 误删自己（列表第一行即 admin，删除弹窗可确认）。
  2. 后端 `DeleteAccount` 成功 → 前端 `onDeleted("admin")`：`current === "admin"` → `setCurrent("")`；`inAdmin` 仍是 true。
  3. 账号迁移 effect（App.tsx:99-112）：`accounts` 还剩学生 → `setCurrent(students[0])`。
  4. 渲染条件 `targetAccount ? … : inAdmin || current === adminName ?` → `inAdmin=true` → **Admin 用学生令牌 `sessionToken` 渲染** → 五个 Tab 的管理接口全部返回 401 → client.ts 广播 `UNAUTHORIZED_EVENT` → `onUnauthorized` 反查到学生账号 → 剔除学生会话 → 管理员被拖回登录页，且学生的会话被误删（管理员自己误删自己的错误操作放大成了删一个学生会话）。
  - 对比：logout()（App.tsx:86）与 onUnauthorized（App.tsx:166）都显式 `setInAdmin(false)`，唯独 onDeleted 漏了这一支。
- **建议修法**（一行）：onDeleted 里补 `if (acct === adminName) setInAdmin(false)`（与 onUnauthorized 第 166 行对称）。

### M-2. `/state` 查询失败时 refetchInterval 回调读不到 `window_closed`，恒 2s 高频重试打接口（网络挂断时全站轮询轰炸）

- **文件路径:行号**：`web/src/routes/Select.tsx:141`（`refetchInterval: (query) => (query.state.data?.window_closed ? 30000 : 2000)`）
- **严重级**：MAJOR
- **一句话问题**：react-query 在查询失败时 `query.state.data` 为 undefined，函数式 refetchInterval 无条件回落 2s；`/state` 每 2s 失败重试一次，且与 `/electives`（其 interval 也读 `/state` 缓存）双查询叠加，网络故障/后端停机期间形成固定 2s 高频轰炸。
- **触发场景推演**：
  1. 学生停在选课大厅，后端进程重启或网络断开（公网部署常见）。
  2. `/state` 与 `/electives` 两查询各自失败 → 数据清空 → 两路 refetchInterval 都取 2s 分支。
  3. 此后每 2 秒两个请求持续打已挂掉的后端，直到用户手动返回/刷新；日志/代理层被无效请求刷屏，与项目"窗口关闭降频""失败分级退避"的防轰炸理念相悖。
  - Dashboard 的 `/state`（Dashboard.tsx:119）与 `/electives`（142 行恒 30s）无此问题（前者 data 失败同样回落 3s，但 3s 尚可；Select 是 2s）。
- **建议修法**（一行）：interval 回调失败态（`query.state.error` / `query.state.status === "error"`）返回 30s 兜底间隔，成功才按 window_closed/开窗升频。

---

## MINOR（边界瑕疵）

### N-1. Dashboard 主倒计时矩阵不吃 begin_times 兜底，识别未完成瞬间与"预计开放时间"行分叉显示矛盾（注释承诺未实现）

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:153-171`
- **严重级**：MINOR
- **一句话问题**：第 162-167 行注释"openTimeStr 未识别时用 begin_times[0] 兜底当下主时间，避免…悬空"，但 `cd = useTickingCountdown(openTimeStr)`（第 155 行）只吃 `openTimeStr`（未识别=null → 全 00 + 过期态），`primaryMs` 的 begin_times 兜底只用于从"其他开放时间"折叠列表剔除主时间——注释承诺的兜底实际未落到倒计时矩阵上。
- **触发场景推演**：
  1. 学生进入 Dashboard 时调度器识别槽尚未建立（首帧 `/state` 的 open_time_known=false 或 open_time 已过期），但平台 `begin_times` 已下发未来开窗点。
  2. 主矩阵显示 `00 天 00:00:00` 过期态，同一屏幕"预计开放时间"行显示 begin_times[0] 格式化出的未来时刻 → 学生看到"已到点/已过期"却同时又看到一个未来开窗点，误判窗口已开/已过。
- **建议修法**（一行）：`useTickingCountdown(openTimeStr ?? (electives?.begin_times?.[0] ? new Date(electives?.begin_times?.[0]).toISOString() : null))` 或把兜底时间转字符串喂给 cd。

### N-2. 回显合并触发的防抖 effect 重跑会清空指数退避状态

- **文件路径:行号**：`web/src/routes/Select.tsx:557`（effect 体开头 `resetRetry()`）
- **严重级**：MINOR
- **一句话问题**：`resetRetry()` 在 effect 每次重跑时无条件执行，而 effect 依赖含 `selected`/`echoDone`；回显合并（setSelected）与全清空 echoDone 置位都会重跑 effect → 正在排队的退避重发 timer 被清、attempt 归零，指数退避从 2s/4s/8s 退化为"立即重试"。
- **触发场景推演**：
  1. 用户点选 → 防抖 400ms → `saveNow` 失败（网络抖动）→ `scheduleRetry` 挂 2s timer（attempt=1）。
  2. 同一瞬间 `/state` 首帧到达、回显 effect 合并旧目标 → `setSelected` → 防抖 effect 因 selected 变化重跑 → `resetRetry()` 清 2s timer + attempt=0 → 挂新 400ms timer → 提前重发。
  3. 平台/网络持续抖动时退避永远长不大（每次回显/刷新都归零），与 n14 注释"指数退避"承诺分叉。影响有限（重发提前而非丢失），但违背设计语义。
- **建议修法**（一行）：`resetRetry()` 只在 `rev` 真正因用户改动变化时调用（拆分依赖：用户改动 effect 与回显合并 effect 分离，或 resetRetry 移到 pick() 内）。

### N-3. 管理后台"运行状态"Tab 无窗口关闭信号，窗口关闭后恒显示"待命中"误导管理员

- **文件路径:行号**：`web/src/routes/Admin.tsx:713-714`（StatsTab rows："窗口状态"只用 `window_opened`）
- **严重级**：MINOR
- **一句话问题**：后端 `/api/admin/stats` 未下发 window_closed（对照 handler.go:890-910），前端"窗口状态"只有"已开放/待命中"两态；窗口已关闭（学生端横幅已显示"选课窗口已关闭"）后，管理页仍显示"待命中"。
- **触发场景推演**：选课窗口关闭后管理员打开运行状态 Tab，看到"窗口状态: 待命中"，与后台日志里持续的空快照/关闭信号冲突，管理员误以为窗口尚未开始而反复刷新。
- **建议修法**（一行）：stats 响应补 `window_closed` 字段并在 StatsTab 增加"已关闭"展示（与 SchedulerState 同源）。

### N-4. 硬编码文案"频控保护策略 · 单次熔断冷却"与后端失败分级退避实现分叉（过时文案残留）

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:452-455`
- **严重级**：MINOR
- **一句话问题**：后端已是"失败分级智能退避（风控 30s / 网络快重试 / 退避重发）"（CLAUDE.md 决策锚），前端仍展示"单次熔断冷却"，与真实策略不符的静态文案。
- **建议修法**（一行）：删除或改为"分级退避 · 自动恢复"。

### N-5. App.tsx 账号迁移 effect 依赖不稳定（每次渲染新数组）

- **文件路径:行号**：`web/src/App.tsx:99-112`（依赖 `[accounts, current, adminName]`，`accounts = Object.keys(sessions)` 每次渲染新数组）
- **严重级**：MINOR
- **一句话问题**：`accounts` 每次渲染重建 → effect 每次渲染都重跑；当前无功能影响（体内均为条件 setState），但属于依赖不稳定反模式，未来在 effect 体内加副作用（如打日志/发请求）即变 bug。
- **建议修法**（一行）：`const accounts = useMemo(() => Object.keys(sessions), [sessions])`。

### N-6. `priorityName` 在 Select.tsx 与 Dashboard.tsx 双份副本

- **文件路径:行号**：`web/src/routes/Select.tsx:40-42`、`web/src/routes/Dashboard.tsx:31-33`
- **严重级**：MINOR
- **一句话问题**：两处逐字重复的纯函数（p===0?"首选":"备选 p"），单文件靠 `priorityName` 导名，跨文件无共享；若未来优先级语义变化只改一处即分叉。
- **建议修法**（一行）：抽到 `lib/` 共享导出（不引依赖，5 行函数）。

---

## OBSERVE（存疑待核）

### O-1. 多标签页共享 localStorage 会话，任一 Tab 登出即删全体会话

- **文件路径:行号**：`web/src/App.tsx:52-96`（loadSessions/saveSessions + logout）
- **严重级**：OBSERVE
- **一句话问题**：会话映射存 `localStorage`（无 storage 事件监听），同浏览器两 Tab 共享；Tab A 登出/被 401 剔除 → 清 localStorage → Tab B 的会话同步消失（下次请求 401）。项目未声明支持多 Tab，但这是隐蔽的"会话互踢"行为。
- **触发场景推演**：管理员开两个 Tab 分别以管理员和学生登录，任一侧登出另一侧会话即失效。
- **建议修法**：明确单 Tab 使用边界（README 声明），或监听 `storage` 事件同步 `sessions` 状态。

### O-2. 调用方 signal 主动 abort 被映射为"请求超时"文案

- **文件路径:行号**：`web/src/api/client.ts:69-75`
- **严重级**：OBSERVE
- **一句话问题**：`AbortError` 统一映射"请求超时，请重试"；当前 api() 无调用方传 signal 的主动取消场景（全部走 20s 兜底超时），故实际不会出现，但语义上"用户/组件主动取消"与"超时"共用一个文案，未来若接入调用方 signal 会误导。
- **建议修法**：区分 `e.name==="AbortError" && rest.signal?.aborted` 时映射"请求已取消"。

### O-3. Select.tsx:182-188 渲染期写入 ref（echoedRef.current=false）属于渲染期副作用模式

- **文件路径:行号**：`web/src/routes/Select.tsx:182-188`
- **严重级**：OBSERVE
- **一句话问题**：React 官方允许渲染期"调整 state"（条件 setState 幂等），但同时在渲染期写 ref（`echoedRef.current = false`）严格说不是纯函数路径（StrictMode 双渲染会执行两次，当前幂等无害）；已有 `key={account}`（App.tsx:245/288）+ F36-01 双保险，此守卫是纯兜底。仅记录模式，不建议改动（改动反而破坏 TDZ 顺序设计）。

---

## 已核对无问题的重点区域（本轮逐条验证，不再重复报）

- **目标自动保存链主体**：防抖 400ms 只由 `rev`/用户改动驱动（F7-01）；`saveNow` 串行化（MAJOR-H）+ `lastJson` 去重 + `savingRef/dirtyRef` 补发；卸载后 `unmountedRef` 停手（F13-C2）与挂载复位（F20-01）均正确；`handleBack` 三轮 flush 收敛 + 21s 等待有界 + 假清空脏块刻意不等待（契约）均自洽。
- **假清空守卫链 F15/F16/F17**：开窗瞬间 publishes 清空、窗口关闭、id 漂移三种形态的"产出空集/含漂移 id"均被消费时刻守卫拦截（缺陷仅在 C-1 的"部分覆盖"形态，已报）。
- **回显合并（M30-03/31-01/33-01/34-01/35-01）**：echoedRef 只合并一次、全清空不复活、首帧未到绝不置位、状态镜像 ref 消费时刻读取——逐条核对无缺口。
- **跨账号复用**：App 两处 Select 挂载点均 `key={account}` / `key={targetAccount}`（App.tsx:245/288），+ F36-01 渲染期复位兜底，账号切换实例重建闭环。
- **401 链**：client.ts 事件带 session（真实主体）+ App 按令牌反查、删除后快照式落盘、代理态退出（F25-01）、管理员自身会话退出管理态——与后端 `requireAuth`/`allowAccountOverride`（仅 Admin:true 会话穿透）核对一致。
- **手动报名/退选在飞幂等**：`ReadonlySet<number>` 按课程独立跟踪 + 入口短路（F26-03/M29-01）；后端 `TryAcquireSubmit` 单课锁二次拦截，双保险成立。
- **激活链**：票据随 1001 透传（F11-A1）、激活幂等短路（F21-04）、票据过期引导重登——与后端 `ConsumeTicket` 单次销毁契约核对一致。
- **全弹窗 Esc**：Select 退选弹窗 / Admin 删除弹窗 / Login 激活弹窗 三处 `onKeyDown` Esc 均有 `!在飞` 防误关守卫，`autoFocus` 取消按钮一致。
- **localStorage 读写容错**：App.tsx loadSessions/saveSessions/loadAdminName/saveAdminName 全 try/catch 降级内存态（N4）。
- **登录 RSA/验证码链路**（Login.tsx）与后端 `handleLogin`/`issueSession`/`MarkTokenValid` 契约一致；管理员登录时延拉平、恒定时间口令比对为后端侧，前端无泄露面。
- **倒计时 useTickingCountdown**：target 变化即时校正 now、过期全 00、`setInterval` 只在 `[]` effect 挂载（StrictMode 双挂载安全）。
- **无 `dangerouslySetInnerHTML`/`eval`/`window.open`/`document.write`**，全仓库零直接 XSS 注入点（课程名/教师名等均经 React 文本节点渲染）。
- **Dashboard 日期分组**：parseDateKey 显式锁本地零点、未知日期兜底组排最后、关闭≠元数据丢失（CourseStatus 自带 publish_name/begin_date）——与后端 enrichTargetPubMetaLocked 落库契约核对一致。

---

## 结论

- **CRITICAL 1 / MAJOR 2 / MINOR 6 / OBSERVE 3**，共 12 条。
- C-1 是唯一会造成数据丢失（目标被整包覆盖）的问题，建议优先修复（一行守卫）。
- 其余均不影响数据完整性，属展示/语义/健壮性边界。
