# round62 前端只读审查发现报告

> 审查基线：master @ `5f43d5f`（R61 收官，web/src 零修复轮）。本轮开局与收尾 `git status --short` 双确认工作树干净；`git diff HEAD -- web/src/` **零改动（0 行）**、`git log 5f43d5f..HEAD -- web/src/` **零输出**——R55→R62 连续八轮 web/src 未被触碰。审查期间绝对只读（唯一新建文件为本报告；临时断言脚本全部落系统 `%TEMP%` 且用后即删）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **exit 0**（1948 modules / 417.54 kB js / 41.07 kB css）；TDD 断言 target-guard-check / admin-auth-check / unauthorized-check 全绿（16+6+5）；`node scripts/audit.mjs` 视觉护栏 **1 处违例**（Button dark variant `bg-neutral-900` 裸匹配误报，见 O-9，基线既有非本轮引入）。
> 范围 `web/src/` 全部 19 个 `.ts/.tsx`；后端契约对照 `backend/internal/{api/handler.go, scheduler/scheduler.go, session/store.go}`。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 13（N-1 延续第 6 轮 + N-2 延续第 5 轮 + N-3 延续第 4 轮 + O-1~O-9 延续 + O-10 新增）.** web/src 连续八轮零提交后依旧零缺陷：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读 + 关键时序推演零回归（含本轮三个新推演：手动报名后 /state 轮询携带旧 courses 的推迟语义、5s 等待超时的收敛行为、全清空 PUT 在飞 + 立刻点新课的 lastJson 去重闭环）；任务书新视角 A-E 全部经实证裁决（A 维持 + 本轮对 N-1 落位点与 audit.mjs 误报做了字形级复核；B 零回归 + 新增"courses 非空 + 有选中"推迟语义的正反向双重实证；C 维持；D 全包逐行通读零新增缺陷，新增 O-10 一个纯展示观察）；E 逐条延续。

**最致命 3 条（按影响排序）**：本轮无真实缺陷。关注序：① N-1（冲刺文案）+② O-5（Dashboard key）+③ O-9（Button dark variant `bg-black` 视觉护栏相邻 + audit.mjs 对 `hover:bg-neutral-900` 的误报）。

---

## CRITICAL（数据丢失）

（本轮无 CRITICAL。）

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无 MINOR。）

---

## OBSERVE

### N-1.【延续，连续六轮】「冲刺」文案与后端仅 10 秒黄金期冲刺的实际行为出入——维持，本轮落位点字形级复核

**证据链**：scheduler.go:70-72 `submitIntervalSprint=250ms / submitIntervalNormal=1s / sprintDuration=10s`；submitIntervalFor（290-294）黄金期判定 `now.After(open) && now.Before(open.Add(sprintDuration))`。前端全部「冲刺」文案点位（字形级枚举）：
1. Select.tsx:1123 注释行「本项目特冲刺/预选目标按钮」；
2. Select.tsx:1134 `已设为后台冲刺${priorityName(selIdx)}` / `设为后台冲刺目标`；
3. Dashboard.tsx:648 `isInRange ? "冲刺提交中"`（isInRange=status in {in_range, submitted}，覆盖黄金期与常态提交段）。

**本轮裁决**：前端无黄金期精确感知（三态伪精确，明确否决做）。Dashboard「冲刺提交中」与后端 `submitted` 状态映射语义一致（引擎在黄金期确实以 250ms 冲刺提交、常态 1s 仍提交），文案成立面较宽但非虚假；若做体验整治，极简方向 = 统一「后台目标」删“冲刺”，**两行落位** Select.tsx:1134 按钮文案 + Dashboard.tsx:648 状态行，注释 1123 顺带清理。连续六轮维持确认：从「未看见」升级为「已看见、故意不做」的完整记录。

### N-2.【延续，第五轮】开窗后 pick → 至多一拍调度延迟（250ms 黄金期 / 1s 常态）。维持。

### N-3.【延续，第四轮】三处格式卫生（英文注释残留 + 两处排版），本轮字形级复证：
1. Select.tsx:1123 注释尾「only affects itself」英文残留；
2. Select.tsx:379 `const lastJson = useRef("")` 缩进 4 空格（邻接 380-382 皆 2 空格）；
3. Dashboard.tsx:501 `</div>                <div>` 同行折叠。

本轮复证：`git show HEAD:web/src/...` 三处与工作树逐字节一致（基线既有非本轮引入），`git log -- web/src/` 最近前端改动 47faff4（R54），三处瑕疵 R54 前已存在。样式/注释卫生级，零运行影响。若做体验整治，三项各一行、零风险。

### O-1.【延续】Toast viewport 滚动交互残余。Toast.tsx:105 `overflow-y-auto pointer-events-none` + Root `pointer-events-auto`（F48-M1）——修复破坏「仅 toast 可交互」；M-2 去重 + 3.5s 自消下同刻>4 条概率趋零。维持。

### O-2.【延续，连续八轮】useTickingCountdown NaN 防御（`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`）——输入源全受控合法（`open_time` 走 Go `time.Time` JSON RFC3339，无自定义 MarshalJSON；兜底 `new Date(number).toISOString()` 恒合法）。本轮再核 open_time 序列化链路（scheduler.SchedulerState.OpenTime `time.Time` 标准 JSON、handler 侧 `open.Format("2006-01-02 15:04:05")` 经 `time.Parse` 回读）依旧无不可信串入口。修复一行引 isExpired 语义争议。**维持**（未来开放时间来源若扩展为不可信字符串，先补 `!Number.isFinite` 再上线）。

### O-3.【延续】三处手写 modal 无完整焦点陷阱（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）。均已有 role=dialog/aria-modal + Esc + autoFocus。维持。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询——bailout + TabsContent 懒渲染实证延续。维持。

### O-5.【延续，连续八轮】Dashboard 挂载点无 `key={account}`（App.tsx:331，Select 两处 293/339 有）→ `expandedDates`（268-273）/`extrasOpen`（212）跨账号残留。后端 `StateForAccount`（704-730 按 `c.Account == acct` 过滤 Courses）双证数据不串线，影响纯展示层。本轮再核：`expandedDates` 折叠种子 null 语义只在首帧种下、`extrasOpen` 布尔对账号不敏感，跨账号视觉残留实际影响面极小。**维持**；切号视觉一致性整治时一行 `key={account}` 即可。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button 12 变体/实际子集、Badge 8 变体/实际子集。本轮 grep 复核依旧（`variant=` 全量枚举：Button 实际消费 primary/outline/ghost/dark/secondary(0)/success(0)/warning(0)/invert(0)；Badge 实际消费 primary/outline/destructive 三枚）。维持。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页——无 storage 监听，B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持。

### O-8.【延续】401 保护窗口闭包 current：`lostRaw = detail?.session || detail?.account || current`，session 恒为 401 真实主体、current 兜底永不触发；effect 依赖 [adminName, inAdmin, adminToken] 每改重建闭包，陈旧闭包无实际影响。本轮再核 `onUnauthorized` 内 `isCurrentAdminSession(loadSessions(), adminName, adminToken)` 每次从 localStorage 快照重读 sessions、adminName/adminToken 来自闭包（管理标记改由 setAdminToken 触发 effect 重建闭包）——唯一潜在陈旧源是 `current`（渲染闭包兜底），而 session 恒优先、兜底永不触发，维持。

### O-9.【延续】Button dark variant `bg-black` 实色与「实心不透明黑洞清零」视觉护栏相邻（audit.mjs C 组）。

**证据链**：Button.tsx:21 `dark: "bg-black text-white border border-white/25..."`；audit.mjs C 组检查 `min-h-screen bg-black` 与 `bg-black/25|30|70|75`、`bg-neutral-95x`，**不检查裸 `bg-black`**。本轮字形级复核：**audit.mjs 的 1 处违例是误报**——C 组对 `bg-neutral-900` 的裸匹配命中了 Button.tsx:21 的 `hover:bg-neutral-900`（dark variant hover 微移色，同一行同时含 `bg-black` 语义与 hover 过渡色），非黑洞表面。Dashboard.tsx:295/735 两处消费 `variant="dark"`（选课大厅按钮与移动端悬浮栏），黑底白字在纯黑画布上视觉成立。

**触发条件**：无运行触发；纯静态样式分类。
**影响**：零（dark variant 就是设计里"黑底白字"语义，画布底色同为 `#000000`）。
**修复方向**：若视觉护栏升级为「裸 `bg-black` 也拦」，dark variant 可改用 `bg-black/40` 半透明或 `bg-[#09090b]` 面板色；当前 dashboard 黑底白字按钮是主设计语言一部分，**倾向维持并放宽 audit.mjs 对该行的豁免**。附注：`hover:bg-neutral-900` 裸匹配误报 R60 前已存在、非本轮引入——本轮如实记录不改（本报告唯一新建文件原则）。

### O-10.【新增】Dashboard 状态机四态文案的「提交中」与后端 `submitted`/`in_range` 双状态映射——纯展示观察。

**证据链**：Dashboard.tsx:589 `isInRange = c.status === "in_range" || c.status === "submitted"`；609-616 两态统一渲染「提交中」，648 行 `isInRange ? "冲刺提交中"`。后端 scheduler 状态机 `pending / in_range / submitted / success / failed`（五态）——`in_range` 为「窗口开放、排队待提交」、`submitted` 为「已发出报名请求」；前端把两态合并为「提交中」展示，语义成立（均非终态且引擎正在推进），但「in_range（未提交）→ 显示冲刺提交中」在严格语义上有轻微提前。与 N-1 同源（冲刺文案），零行为影响。**维持**；若随 N-1 一并整治，统一改「后台提交中」。

**附加实证（本轮新增五个时序/引用链复核，含 D 项）**：

1. **pick() fallback 判定源**：Select.tsx:346 `pick(t.publish_id, c)` 回调参数来自渲染期 map 的 `t`（当前发布与课程列表），不含任何旧闭包引用——fallback 判定源恒为最新渲染数据。且 `pick` 是事件处理器（每次渲染新定义），`selected` 为渲染闭包当前值，与 `arr = [...(selected[publishId] ?? [])]` 的"先快照后 set"语义一致（两次点击间必有渲染提交，F10-02 注释论证成立）。
2. **Dashboard dateGroups 纯 useMemo**：Dashboard.tsx:219-263 依赖 `[courses, electives?.publishes]`，两输入均为 react-query 缓存引用（数据不变引用不变），`pubById`/`byDate`/`groups` 全在 useMemo 内部新建、无外部突变——分组无渲染外副作用。`key={g.key}`/`key={pub.publish_id}`/`key={c.class_id}` 均唯一。
3. **App onUnauthorized 闭包**：见 O-8 实证——`adminName/inAdmin/adminToken` 每改重建 effect 闭包，`onUnauthorized` 每次从 localStorage 快照重读 sessions 反查 lostAccount，无陈旧状态污染。
4. **共享 react-query 缓存两路由**：Dashboard.tsx:171-175 与 Select.tsx:56-57 同 `queryKey: ["electives", account, sessionToken]`——Dashboard 轮询恒 30s、Select 轮询按窗口信号升/降频，同一缓存被两个轮询调度器管理（react-query 合并为最近 refetchInterval）。切页不重复请求、开窗后 Select 侧升频即时接管——设计意图成立。唯一理论竞态：Dashboard 30s 轮询与 Select 2s 轮询同屏交错挂载（实际路由互斥，Admin 代理态例外见 B 表），无实际影响。
5. **saveNow lastJson 去重链**：Select.tsx:429 `if (json === lastJson.current) return`——lastJson 只记录"上一次成功保存的 json"；全清空 PUT [] 成功后 lastJson="[]"，用户立刻点新课产生新 json（含新课程）≠ "[]" → 正常再发。若旧 PUT 在飞（savingRef=true）→ 走 dirty 分支，飞行 PUT 完成 finally 补发最新快照。**无乱序覆盖路径**（本轮新推演，见 B 表场景四）。

---

## 重点核对结论（任务书逐条裁决）

### A. N-1 / N-2 / N-3 / O-1~O-9 延续复核——维持，落位点字形级复核

三连确认：① 前端无黄金期精确感知 → 三态文案伪精确（负收益，明确否决）；② 黄金期后用户主通道是官网「报名」primary 大按钮（Select.tsx:1110-1122），冲刺 ghost 小按钮是次要入口；③ 若治 = 两行：Select.tsx:1134 + Dashboard.tsx:648 统一「后台目标」删“冲刺”，注释 1123 顺带清理。N-2/N-3 同源维持。O-1~O-9 逐项复核机制实证相同（见上表）。

### B. Select 保存链六防全路径复证（逐字符零回归 + 三个新时序推演）

| 防护族 | 本轮结构与语义复证 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 三消费点同源（Select.tsx:499/589/676）；断言 6 项覆盖全清空放行；`select-none` 页根与输入交互互不干扰 |
| **F42-M1 判据解耦** | 纯数据判据不依赖 echoedRef；防抖 effect 依赖含 stateData（737）自愈；sets 直连 publish 过滤与消费时刻双闸判据同源 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（287/316-327/518/696）全在；独立清理 effect（316-327）依赖含 selected；toast 判 `!unmountedRef.current` 不轰炸卸载后 |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）；断言场景 C/D 覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:293/339 双 keyed + accountKey 守卫（195-202）TDZ 安全 |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow |

关键时序推演（均无丢失路径，本轮补三则新推演）：

- **【本轮新增】「手动报名成功后 /state 轮询携带旧 courses」**：actionLoading invalidate state → /state 下一拍返回旧 courses（后端尚未更新）→ 防抖 effect 随 stateData 变重跑 → 守卫 shouldDeferSave(courses 非空, 有选中)=true → 推迟（安全方向：此刻 selected 已含后端目标，整包 PUT 与后端一致，推迟抑制无害）。解锁：/state 2s 轮询 courses 空 → effect 重跑放行；或 handleBack 5s 等待收敛。**不丢失、不覆盖**。
- **【本轮新增】「handleBack 5s 等待超时的收敛行为」**：courses 恒非空（/state 持续失败）→ 5s 超时 while 退出 → flush 内 shouldDeferSave 仍 true → 置脏返回 → 三循环后 onDone。内存 target 未落库，但后端已是最新（手动路径），下次进页回显合并恢复——**无数据丢失，安全方向**。
- **【本轮新增】「全清空 PUT 在飞 + 立刻点新课」**：lastJson="[]" 与新 json 不同 → 正常再发；savingRef=true 时走 dirty 分支、飞行完成 finally 补发最新快照。**绝无乱序覆盖**（lastJson 去重 + savingRef 串行化双保障）。
- **「PUT [] 在飞 + 用户立刻点新课」** → recovery（lastJson 不更新 → 防抖再 PUT）→ 收敛；
- **「全清空 PUT 在飞 + 回显旧 courses 到达」** → merged=false 且 hasTouched=false → 返回 prev 放行 → 无复活；
- **「发布重建 + /state 首帧交错」** → 合并带出 stale → 独立清理 effect 随 selected 重跑清掉 → 防抖落库；
- **「type=number 输入非法字符串（concurrency）」** → `Math.min(16, Math.max(1, NaN || 1))` → 1 兜底；

**核心推演正面确认（本轮重点）**：「/state 首帧未到即点新课 → /state 到达 courses=[旧目标] → 回显合并（已触碰发布补进旧目标）→ 合并后 selected 完整（新课 + 旧目标）→ stale 检查放行」。合并后防抖守卫 shouldDeferSave(courses 非空, 有选中)=true 推迟命中——这是**安全方向**（此刻 selected 已含全部后端目标，PUT 整包=后端旧目标+新课，即使放行也无覆盖丢失；推迟仅是延后落库），且解锁路径三路齐全（/state 轮询 courses 空自愈 / handleBack 5s 等待 / 用户后续改动驱动）。**零回归确认**。

**反向推演（假清空误放行检查）**：「courses 非空 + 用户有选中」时守卫恒推迟——若后端 courses 尚未同步（旧目标残留），selected 已合并完整时推迟无害；若 selected 不含后端目标（合并未发生）推迟防覆盖。**假清空（selectedCount>0 却产出空集）路径由 F17-01 独立守卫兜住**。全链路无「courses 非空 + 有选中」却放行整包 PUT 覆盖删除后端的路径——**F42 语义零回归**。

**TDZ**：防抖 effect（645-737）同步体不引用后声明物；`const lastJson` 缩进瑕疵不改语义。

### C. O-2 / O-5 —— 维持（第 8 轮，见 O-2/O-5）

### D. 全包逐行通读找新问题

**结论：零新增缺陷，新增 O-10 一个纯展示观察。** 本轮除 R61 复核项外新增：

1. **`?account=` 五路凭据表校验**：handleElectives/handleElectiveSelect/handleElectiveExit/handleSetTargets/handleState（handler.go 235-249 / 286-301 / / 437-476 / 530-546）全部先 allowAccountOverride（1067-1069，`Sessions.IsAdminToken`）再 accountExists（1074-1085，LoadCredentials 逐账号）。admin 名兜底 IsAdminAccountName 与 `sessionAccount`（requireAuth 注入会话绑定账号）对齐——**管理员自身会话带 `?account=` 学生名时，acct 被 q 覆盖为学生名，读/写全路径凭据表已挡，绝无管理员名穿透**。
2. **手动报名/退选双端幂等**：后端 TryAcquireSubmit（1892-1903，inflight 单课程锁）拒并发 + CheckClassSelectable（1859-1888）快照复核 + SelectClass → MarkDone（1918-1978）；前端 actionLoading Set<number> 独立跟踪、函数式删除只清自己 id（Select.tsx:52/92-93/106-110/119-120/131-135）。
3. **401 三形态单广播**：client.ts:64-69（HTTP 401 前置）+ 75-95（body 401 仅 `r.status !== 401` 补）；App.onUnauthorized 幂等（187-235）——本轮再核 `onUnauthorized` 的 `lostAccount === adminName` 分支（210-215）在**双删保护**（setInAdmin(false)+setAdminToken("")）后仍先走 `setTargetAccount` 清理（204-206）再删除会话（225-230），顺序安全。
4. **Admin 五 Tab**：CodesTab removing Set 按码独立/ConfigTab `!loaded` 拒存 + refetch 成功才自增 epoch/StatsTab `window_closed` 三态 + `token_valid` 部分失效/AccountsTab targets 空数组兜底/LogsTab limit=200。本轮新增核对：**CodesTab 生成数量 max=100 与后端 handleAdminCodes `req.Count < 1 || > 100` 拒绝边界一致；Uses max=1000 与后端 `req.Uses > 1000` 拒绝一致**（前端无 max 硬上限、由后端 1000 兜底）。零缺陷。
5. **多账号年级隔离前端侧**：StateForAccount 704-730 按账号过滤 Courses；ElectivesSnapshotFor（766-810）「专属帧过期 → 返回 (nil,false) 绝不回退全局帧」契约再核（B28-01 语义在位）；Select keyed 双保险；Admin 各 queryKey 含 account（n11 语义在位）。
6. **手动退选 refused 语义对齐**：Dashboard isFullFallback `result.includes("已满员")`（36-38）匹配 markFullLocked 文案「该课程已满员，退避至下一备选」（scheduler.go:1764）；状态机 586-616 对后端 status 枚举全覆盖。
7. **倒计时条件顺序**：Select.tsx:809-835「未识别」分支（821）先于 `cd.isExpired`（826）；Dashboard 文案行同款顺序正确（407-413）。
8. **刷新/回显/卸载生命周期**：unmountedRef StrictMode 复位（398-407）+ F20-01；echoedRef 永不重放（226-297 首行短路）；selectedRef/revRef/stateDataRef 每渲染同步（173-181）。
9. **Session/激活链**：Login 1001 分支保存 ticket（54）+ 激活回传（78）+ F12-M3 票据过期引导（87-97）；session.CreateTicket/ConsumeTicket（store.go:102-138）票据绑定账号 + 单次防重放 + 5 分钟 TTL。本轮新增核对：**Login 1001 响应 `e.data?.ticket` 与后端 handleActivate 的 ConsumeTicket（handler.go:197-199 票据在激活码校验失败时已销毁）语义闭环**。零缺陷。
10. **【本轮新增】CodeTab generate 与后端边界对齐**（见上 4）。

### E. 上轮观察项延续复核（全表）

| 编号 | 本轮核实 | 裁决 |
|---|---|---|
| N-1 | scheduler.go:70-72/290-294 冲刺实测 + Select.tsx:1134/Dashboard.tsx:648 两落位点字形级复核 | **延续**（第 6 轮） |
| N-2 | pick→PUT/targets→tick 提交链路 | **延续**（第 5 轮） |
| O-1~O-8 | 逐项复核机制实证相同（见上），无新增变化 | **延续** |
| O-9 | Button dark `bg-black` + audit.mjs 对 `hover:bg-neutral-900` 的裸匹配误报 | 维持（见上） |
| N-3 | 三处格式瑕疵基线既有性复证（git show HEAD 逐字节一致） | **延续**（第 4 轮） |
| O-10 | 新增：Dashboard 状态机「提交中」合并 in_range/submitted 两态 | 维持（纯展示） |

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 13（N-1 第 6 轮 + N-2 第 5 轮 + N-3 第 4 轮 + O-1~O-9 + O-10 新增）**。tsc exit 0、build exit 0（1948 modules / 417.54 kB / 41.07 kB css）、三个 TDD 断言 16+6+5 全绿、audit.mjs 1 处违例（Button.tsx `hover:bg-neutral-900` 裸匹配误报，基线既有，见 O-9）、web/src 与 HEAD 逐字节一致（R55→R62 连续八轮零提交基线实证）。
- **最致命 3 条**：
  1. **N-1：「后台冲刺」文案 vs 后端仅 10 秒黄金期 250ms 冲刺**——纯文案、零行为影响；连续六轮维持。若整治 = 两行（Select.tsx:1134 + Dashboard.tsx:648）统一「后台目标」。
  2. **O-5：Dashboard 无 key={account}**——expandedDates/extrasOpen 跨账号残留纯展示层；一行加 key 即可。
  3. **O-9：Button dark variant `bg-black` 与「实心黑洞清零」护栏相邻**——纯样式分类、零运行影响；当前倾向维持并考虑 audit.mjs 豁免该行。
- **已核对无缺陷的高风险区**：六防保存链逐字符零回归（含三个新推演：手动报名后 /state 旧 courses 推迟语义、handleBack 5s 等待收敛、全清空 PUT 在飞 + 立刻点新课 lastJson 去重闭环）；核心推演「/state 首帧未到即点新课 → 回显合并 → 推迟命中（安全方向）+ 三路解锁」正面确认；反向推演「courses 非空 + 有选中」无整包覆盖路径；后面板 5 Tab 零缺陷（含 CodesTab 生成数量/Uses 边界与后端一致）；`?account=` 五路凭据表校验；401 三形态单广播；倒计时三态；多账号隔离。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | exit 0（1948 modules / 417.54 kB js / 41.07 kB css） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | 16/16 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 视觉护栏 | `node scripts/audit.mjs` | 1 处违例（Button dark `hover:bg-neutral-900` 裸匹配误报，基线既有，见 O-9） |
| 保存链时序模拟 | `%TEMP%` 临时脚本（5 个场景断言） | 场景一/二/三/四/五 全部按预期收敛（推迟=安全方向，解锁路径三路齐全） |
| 工作树一致性 | `git status --short` | 报告写入与构建后均零输出 |
| web/src 零改动 | `git diff HEAD -- web/src/` | 0 行 |
| 跨轮次零提交 | `git log 5f43d5f..HEAD -- web/src/` | 零输出 |
| 临时文件清理 | `%TEMP%` 下断言脚本 rm | 已清理（仓库内零残留） |
| 报告落盘核验 | `Write round62-frontend-findings.md` | 文件已写、正文完整（含验证实证表） |
