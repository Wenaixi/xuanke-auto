# round42 前端审查原始发现

> 审查基线：master（round41 fix-report 落地后，`6298812`）
> 范围：`web/src/` 全部 `.ts/.tsx`（App.tsx / api/client.ts / routes/{Select,Admin,Dashboard,Login}.tsx / lib/{targetGuard,useTickingCountdown,utils}.ts / types.ts / main.tsx / components/ui/*），绝对只读，仅 Write 本报告。
> 校验：`cd web && npx tsc -p tsconfig.app.json --noEmit` exit 0（根 tsconfig 是 references 空壳，`tsconfig.app.json` 才真校验 src）。
> 后端对照：`backend/internal/{api/handler.go,api/router.go,scheduler/scheduler.go,zhidao/client.go}`（GET /api/electives 只有快照不触发探测、CourseStatus/state 契约、schedule 提交链、手动报名/退选 body）。
> 上轮核对结论：F41-M1（独立清理 effect）/F41-M2（flush 回显守卫）/F41-N1（进度条）/F41-N2（非 JSON 401 前置广播）**全部属实且已修复**；N-3（resetRetry）与 O-1（每秒整页重渲染）确认仍在，以 OBSERVE 各留一条。

---

## MAJOR（明确错误行为扰动主线）

### M-1. 防抖 effect 订阅缺「填」角色——守卫命中置脏后若 `setSelected` 返回相同引用（原子幂等）被 React bailout，防抖订阅永不重入，保存链永不恢复

- **文件路径:行号**：`web/src/routes/Select.tsx:634-666`（防抖 effect 首行 `if (rev === 0) return`）+ `:718-722`（依赖数组 `[rev, selected, ..., hasPublishes, echoDone]`）+ `:460-462`（flushTargets 同款守卫）
- **严重级**：MAJOR
- **一句话问题**：防抖 effect **只在 `selected` 变化时重跑**；`(rev>0 && !echoed && (stateData undefined || courses>0))` 守卫命中置脏后，非空 courses 场景只由「foreign 轮询课程字段 → 用户再次点击 → setSelected」才能「填回订阅并触发新 timer」——同发布重复点击撤销/重选到原子状态，`setSelected` 返回**相同引用被 React bailout**，订阅永不重入，f1 未执行，保存链永不恢复。
- **触发场景推演**：
  1. `/state` 首帧失败（campus 抖动/后端重载），`stateData===undefined`；学生已 echo 一次（回显完成后首次改动保存成功，`echoedRef.current=true`）。
  2. 学生点选课 B@p1 → 400ms timer → fl 守卫 `!echoedRef.current` **误判为"回显未完成"** → `dirtyRef=true; return`（fl `revRef>0 && !echoed` 同款）。后一次用户改动（加入 C@p1）在 f2 可过（`rev>0` 且 pollutedPublished 过滤后 selected 全空），f3 `targets=[] && selectedCount>0` 命中 → 同样置脏。保存链不再恢复。
  3. 因 2s /electives 轮询 `refetchInterval`（同一 closure，见 M-2）依赖 `/state` 的 `data.window_closed`，而 `/state` 仍是 undefined（publish 未注定），轮询用同 interval 每 2s 拉一次 `/electives`；轮询的 `data` 不变（`publishes` 引用不触发）、`setSelected` 被 React bailout → 防抖 effect 永不重入，hook 超时无订阅。后一次用户对窗口字段的 `setSelected` 也返回**相同引用**（对象恒等）→ bailout，永不恢复。
  4. 结果：**黄金期用户改动永不落库**，无 toast、无日志；唯一的解锁路径是整页刷新（重挂组件让 f1 直接运行）。这就是保存链死锁。
- **与决策锚的关系**：round41 N-3 注释承认"成功或用户改动清零"，而此处守卫判据是 `!echoedRef.current && (stateDataRef undefined || courses>0)`——`echoedRef.current=true` 时守卫本就**不该命中**，但跑旧 closure 的 fl 逻辑必须读 ref 判该字面量。把 `otherRef` 标为 `true` 无缓解——闭包在 effect 建立后从未更新。
- **建议修法**（一行）：守卫的 `!echoedRef.current` 分支改用**消费时刻**判"回显是否真的没完成"——`if (stateDataRef.current===undefined || (stateDataRef.current.courses?.length ?? 0)>0)`，不回 `echoedRef`；或给防抖 effect 加「/state 新增首帧」依赖（每次 data 变化重置订阅），让任何新 data 触发重跑。

### M-2. `/electives` 轮询间隔依赖 `/state` 的 `window_closed`——/state 查询失败（data undefined）期间 never 返回 30000，`/electives` 固定 2s 高速轰炸

- **文件路径:行号**：`web/src/routes/Select.tsx:142-149`（`refetchInterval` 回调）+ `:139-142`（`/state` 查询）
- **严重级**：MAJOR（网络分层）
- **一句话问题**：`/state` 查询失败后 `data` 为 undefined，`st?.window_closed` undefined → `inRange || st?.window_opened ? 2s : 10s` 走 2s；`/electives` 查询自己的轮询间隔被 `/state` 的这个 `data.window_closed` 入闸——`/state` 失败期间 /electives 恒 2s 高速轰炸（与 `F40-M3` 同族，注释已承认"降频失败分支"但 `/electives` 一侧没同步）。
- **触发场景推演**：`backend restart / 网络抖动` → `/state` 失败（data undefined）→ `/electives` 轮询每 2s 一次无限轰炸日志与代理层；恢复后 30s 兜底路径永不走。
- **建议修法**（一行）：`/electives` 的 refetchInterval 并入 `query.state.error || ...` 判失败即 30000，或读 `/state` 的 `st?.window_closed` 时优先查 `queryClient` 的 `["state", account, sessionToken]` 缓存是否含 error。

### M-3. `/electives` 轮询升频（`window_opened ? 2000`）被 `st===undefined` 吞掉——`/state` 失败即黄金期升频失效

- **文件路径:行号**：`web/src/routes/Select.tsx:60-75`（`refetchInterval` 回调）
- **严重级**：MAJOR（`/electives` 轮询契约）
- **一句话问题**：`st` 取 `["state", account, sessionToken]` 缓存；react-query 缓存有 data 但**伴 error 同存在**（读取仍需 st），失败后又需要 `st.window_closed` 判 30s → 失败时 `st===undefined` → 升频判定（`window_opened ? 2000`）从未评估；engine 静默退化到"每 10s 一轮"，开窗黄金期本该 250ms 冲刺由前端驱动却被吞掉。
- **触发场景推演**：`/state` 偶发失败（校园网丢包）→ 前端每 10s 才拉一次 /electives；开窗瞬间（publishes 空）该 10s 轮询拉到新 publish 前窗口状态模糊，/electives 顶着 10s 慢轮询进黄金期（F5-05 注释描述本轮紧邻场景）。
- **建议修法**（一行）：读 `/state` 缓存失败时**不丢**现有升频信号——`st?.window_opened ?? last value store`（保留失败前最近成功的 `window_opened`），或并入 `query.state.data?.in_date_range` 置信度。

---

## MINOR（展示 / 边界健壮性）

### N-1. F41-M1 独立清理 effect 依赖 `[publishes, echoedRef, toast]`——漏掉 `selected`/`stateData`：发布重建与 `/state` 首帧到达交错时 selected 清理可能落在**回显合并之前**，导致合并后 stale 残留 / 数据无提示

- **文件路径:行号**：`web/src/routes/Select.tsx:307-318`（`selectedHasStalePublish` + `cleanStaleSelected` 调用）+ `:308`（`if (!echoedRef.current || publishes.length === 0) return`）
- **严重级**：MINOR
- **一句话问题**：独立清理 effect 不依赖 `selected`——发布重建瞬间（publishes 引用变）effect 重跑，`selected` 此刻是**重建前的旧值**；`echoedRef.current===true` 时通用；但若发布时间恰在 `/state` 首帧到达前（reconstruction 早于回显），清理只跑一次，之后回显 effect 把「旧 publish_id」course 经 `currentIds` 过滤掉新 selected 也不携带旧残留——`selected` 不依赖、不会被 effect 重排，正确性实际依赖"选中时刻选中集完整"。**cleanStaleSelected 按当前 `publishes` 清掉旧 id 时，若 selected 里恰好还挂着「下一个发布重建要保留」的课程——没有防护**。
- **触发场景推演**：开窗瞬间平台清空又恢复（publishes 先空后满）；`selected` 含旧 p1:[A]；发布重建（全新 p9）与 `/state` 首帧（courses 含旧 p1 课程）交错 → effect A 先按 p9 清掉 p1:[A] + toast「旧批次目标已失效」→ 回显 effect 后到，`ordered` course 带 p1（`currentIds` 已不包含 p1）→ 被 `currentIds.has()` 过滤 → 合并不到 selected → selected 只剩用户新点课程 → **dirty 置脏** → flush 守卫 `revRef>0 && !echoed` 分支因 `echoedRef=true` 放行 → `saveNow` 已持有旧 courses → 覆盖后端。
- **建议修法**（一行）：把 `selected` 加进独立清理 effect 的依赖（`[publishes, selected, echoedRef, toast]`），命中 stale 即重清理；或清理前先判「`/state` 首帧已合并完成」再清理。

### N-2. `handleBack` 的 5s 首帧等待窗口里用户改为全清空 → flush 守卫误判「回显未完成」置脏 → 等待超时后 onDone，`rev>0` 但 never 落库

- **文件路径:行号**：`web/src/routes/Select.tsx:576-589`（handleBack 首帧等待）+ `:472-503`（flushTargets 守卫）
- **严重级**：MINOR
- **一句话问题**：handleBack 首帧等待 `(revRef.current > 0 && !echoedRef.current && stateData===undefined)` 覆盖「首帧未到」，但 `/state` 首帧的 **courses 为空**（用户确认后端无旧目标）时 `echoedRef` 已在回显 effect 空 courses 分支**置位**（`echoedRef=true`），条件不再成立；等待结束时 flush 进 `if (latestRev>0 && !echoedRef.current && stateDataRef.current===undefined)` → `dirtyRef=true; return` → rev 保留；下一次 handleBack 重进，`echoedRef` 已 true → flush 过所有守卫 → `saveNow` → PUT。**若用户等待期间把目标改为全清空（rev>0 且 selected 空）**，flush 的「假清空 + 发布缺席」守卫拦截（`latestSelectedCount>0` 才拦），selected 全空直接放行 → PUT []（合法清空）正确；但「首帧 courses 空」的 `echoedRef=true` 已让 flush **跳过「发布缺席 + 已有选中」守卫**（`stateDataRef.current.courses.length===0` 分支不拦截）→ 用户清空时 platform 恰关窗（publishes 空）→ `targets=[]`、selected 全空 → **合法清空却永不落库**（dirty 卡在 `revRef>0` 条件）。
- **触发场景推演**：进大厅 → 点开窗后选 A@p1 → 开窗瞬间平台关窗（publishes 清空）→ handleBack → 等待超时 → flush「发布缺席+已有选中」守卫清空（`latestSelectedCount` 是旧 render 的快照）→ 通过 → PUT [] → **后端目标被抹**（本应放行 false）。
- **建议修法**（一行）：flush 的「发布缺席」守卫条件补 `!!echoed` 提前置脏并等待下一轮。

### N-3. 手动「报名」在窗口开启前（t.in_date_range=false 且 window_opened=false）被隐藏——报名按钮分支只在 `in_date_range || window_opened` 渲染（操作按钮区），`btn_type===2` 但窗口未开时「报名」按钮不出现，用户无法主动触发

- **文件路径:行号**：`web/src/routes/Select.tsx:1070`（`t.in_date_range || stateData?.window_opened ? ... : 标准自动预选设置`）
- **严重级**：MINOR（官网契约对齐，非缺陷——若平台 `btn_type===2` 且窗口未开，前端隐藏报名按钮，用户零手段强制报名）
- **一句话问题**：官网真实 select.js 渲染「报名」按钮（`btn_type===2`）由平台 `can_select`/`btn_type` 决定，**与窗口开关无关**；本项目前端把「报名按钮」绑在 `in_date_range || window_opened` 上——窗口未开时该按钮不渲染，仅剩「设为预选目标」（不直接报名）。若平台有窗口未开但仍开放报名的特殊批次，用户无法手动触发。
- **建议修法**（一行）：报名/退选按钮渲染条件改以 `c.btn_type` 为唯一判据（btn_type 1/2 即渲染，disabled 交给 `can_select`），`in_date_range` 只决定「预选目标」vs「手动报名」的文案切换。

### N-4. `Login.tsx` 激活成功后 `pendingTicket` 不清空 → `/activate` 后再次登录（1001 未激活账号）票据依然携带旧值，服务端 ConsumeTicket 可能报「票据已用」

- **文件路径:行号**：`web/src/routes/Login.tsx:80-82`（激活成功 `setPendingAccount("") / setPendingTicket("")`）
- **严重级**：MINOR（票据 5 分钟单次，正常用户激活一次即可；若激活失败重登再激活，票票据旧值过期概率高）
- **一句话问题**：激活成功分支已清空；失败分支保留 `pendingTicket`（F12-M3 只清「过期/已用」分支）。用户激活失败（网络抖动）后**不做取消**→ 再次「激活」→ 重登 1001 → `setPendingTicket((e.data?.ticket ...)||"")` 覆盖（新票据）——行为正确；但若用户「激活失败 → 重新登录」（票据服务端已 Consume 半次）→ 新 1001 携带**旧票据**回传 → ConsumeTicket 报「激活票据无效或已过期」→ 死循环。低概率但可触达。
- **建议修法**（一行）：激活失败非「过期」分支也补 `setPendingTicket("")`，下一轮 1001 强制用新票据。

---

## OBSERVE（观察项）

### O-1. round40 N-3 / round41 O-1 延续：回显合并/发布清理导致的 `selected` 变化 resetRetry 清零指数退避

- **文件路径:行号**：`web/src/routes/Select.tsx:636-638`（防抖 effect 入口 `resetRetry`）
- **严重级**：OBSERVE（两轮已观察，未加重）
- **一句话问题**：回显合并（非用户动作）产生的 `selected` 变化同样走进防抖 effect → `resetRetry()` 清掉排队中的退避 timer + 归零 attempt；网络抖动下连续轮询合并可让重发从不长大。行为无错、语义与 n14「指数退避」承诺分叉。建议仅在用户动作（pick/清空）调用 resetRetry。

### O-2. round40 O-1 延续：useTickingCountdown 每秒 setNow 驱动 Select/Dashboard 整帧重渲染

- **文件路径:行号**：`web/src/lib/useTickingCountdown.ts:8-12` + 两端消费（Select.tsx:734 / Dashboard.tsx:134）
- **严重级**：OBSERVE（已观察）
- **一句话问题**：top-level `setInterval(setNow, 1000)` 每秒整帧子树重渲染（Select 数百张课程卡片 / Dashboard 日期分组全量重建），与注释「只重渲染倒计时一处」承诺分叉；500 课程 Scale 下可感知。行为无错、纯性能，建议抽独立 `<Ticker>` 子组件或 React.memo。

---

## 已核对无问题的重点区域

- `client.ts`：`extractAccountFromPath` + 非 JSON 401 前置广播（F41-N2）双路径验证正确；`api()` 20s 超时 signal、F7-09 abort 文案、-2 兜底均对齐。
- `App.tsx`：401 归属判定、快照式三连落盘（F8-01）、代理态/管理态双复位（F15-07/F25-01/M28-01/M29-02）、onBackToStudent 完整登出（F21-05）、Select key={account}（F36-01）全链路核对无缺口。
- `targetGuard.ts`：selectedHasStalePublish / cleanStaleSelected 两纯函数、空数组键不判过期、无变化返回原引用——TDD 语义闭合。
- `Select.tsx` 自动保存链：F7-01 仅 rev 驱动 / F13-C1 rev>0 才 PUT / F15-F17 假清空守卫 / F19-01 幽灵过滤 / F31/F35 真合并、消费时刻 read-ref、`hasPublishes` 布尔信号不重置 400ms 窗口、`echoDone` 驱动空 courses 置位后重试、unmountedRef 卸载停手（F13-C2/F20-01）、handleBack 三轮 flush + 21s 有界等待 + 补发闭环——逐条正确。
- 手动报名/退选在飞幂等 `ReadonlySet<number>`、后端 4 方法与身份复核链（sameClientFor/B39-01 + flush 守卫）闭环。
- 可访问性：三个手写模态（role=dialog/aria-modal/aria-labelledby/Esc + 进行中防误关）、CollapseSection useId + aria-expanded/controls、role=switch/aria-pressed/autoFocus——全部就位。
- Dashboard：日期分组本地零点（parseDateKey/localTodayMs，round40 N-1 已修）、「未知」兜底、isFullFallback、priorityName——核对无新增问题。
- `<Progress>` unannounced 空条（F41-N1）与 `aria-valuenow` 语义、窗口状态横幅分支顺序（F15-03）——正确。

---

## 结论

- **MAJOR 3 / MINOR 4 / OBSERVE 2**，共 9 条。
- M-1 为最高优先级新发现：防抖订阅缺「填」角色 + `setSelected` 原子幂等导致**保存链死不恢复**；M-2/M-3 是 `/electives` 轮询收益被 `/state` 单点拖累的两个网络契约缺口（与 F40-M3 同族但未覆盖）。
