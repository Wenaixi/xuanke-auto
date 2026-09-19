# round41 前端审查原始发现

> 审查基线：master（round40 总结落盘后，git status 仅 4 个社区文件未跟踪）
> 范围：`web/src/` 全部 `.ts/.tsx`（App.tsx / api/client.ts / routes/{Select,Admin,Dashboard,Login}.tsx / lib/{targetGuard,useTickingCountdown,utils}.ts / types.ts / main.tsx / components/ui/*），绝对只读。
> 校验：`cd web && npx tsc -p tsconfig.app.json --noEmit` exit 0（注意：根 tsconfig 是 references 空壳，`tsconfig.app.json` 才真校验 src）。
> 后端对照：`backend/internal/api/handler.go` / `scheduler/scheduler.go` / `router.go`（手动报名/退选后端不读 isOk、?account= 凭据表校验、/state courses 按账号过滤、`Time.Time` 走 RFC3339 序列化、AccountsWithTargets 排序确定）。
> 上轮观察项核对结论：N-3（resetRetry 清零指数退避）与 O-1（useTickingCountdown 整页重渲染）**确认仍成立**，分别以 OBSERVE 结尾各占一条；O-2 多 Tab 互踢本期无新证据。round39/40 已修锚点（M-2 文案、M-3 状态/electives 双降频 30s、M-1 stale toast、F40-M1 清理）属实，但 F40-M1 的"随发布重建清理"存在**实现范围与注释承诺分叉**（见 M-1）。

---

## MAJOR（数据丢失 / 决策锚承诺分叉）

### M-1. F40-M1「随发布重建清理」只覆盖首次回显——已回显账号发布集合重建后 selected 残留旧 publish_id 无任何自动清理路径，黄金期保存链持续被 stale 守卫锁死

- **文件路径:行号**：`web/src/routes/Select.tsx:279-286`（cleanStaleSelected 唯一调用点）与 `:220-289`（回显 effect 整体）；对照 `lib/targetGuard.ts:24-40`
- **严重级**：MAJOR
- **一句话问题**：`cleanStaleSelected(prev, currentIds)` 挂在整个回显 effect 内部、且 effect 首行 `if (echoedRef.current) return` 短路——**凡是已完成过一次回显的账号（即正常使用的主流形态），echoedRef 恒 true，之后发布集合整体重建（开窗瞬间平台清空又恢复、publish_id 全变，CLAUDE.md 实证）时 stale 清理永不执行**。seal 地只剩 flush/防抖消费时刻的 `selectedHasStalePublish` 守卫在命中时置脏 + toast「请刷新页面重新选择」——但界面旧 Tab 已消失、用户**无法用 UI 清除残留 key**，黄金期每次改目标都被拦（toast 有提示但无自愈），唯一解锁路径是整页刷新。
- **触发场景推演**（自洽时序）：
  1. 开窗前学生进大厅，勾选 A@p1（旧发布，publish_id=1）→ 防抖 PUT 落库；首次回显 effect 合并完成后 `echoedRef.current = true`。
  2. 开窗瞬间平台发布集合整体重建（/electives 轮询返回新发布 p9，旧 tab 消失）→ `publishesRef.current=[p9]`；`selected` 仍残留非空键 `{p1:[A]}`。回显 effect 因 `echoedRef=true` 在第一行短路，F40-M1 的清理代码路径根本不执行。
  3. 学生在黄金期点选新目标 B@p9 → 防抖 400ms 回调在消费时刻 `selectedHasStalePublish({p1:[A], p9:[B]}, [p9]) === true` → `dirtyRef=true` + toast 拦下；flush 同款拦下。B 永不落库。
  4. 用户改了一次又一次，每次都 toast，但 p1 残留 key 界面无入口清除（旧 tab 不存在），唯一出路是「刷新页面」——而刷新会重挂组件、echoedRef 复位，回显过滤幽灵课程后才恢复；黄金期这恰恰是最不该打断用户的操作。
- **与决策锚的关系**：F40-M1 注释与 CLAUDE.md 决策锚声称「随发布重建清理即可解锁……随建随清（首选出路）」——**实现只在『首个回显』时机清理**，对已回显账号完全不成立，实现与契约分叉。
- **建议修法**（一行）：把 stale 清理从回显 effect 抽成一个独立的 `useEffect`，仅依赖 `[publishes, data]`（不依赖 echoedRef），命中 `selectedHasStalePublish` 即 `setSelected(s => cleanStaleSelected(s, currentIds))` + 一次保存驱动；flush/防抖守卫命中 stale 时也可直接就地 `cleanStaleSelected` 再放行，而非只置脏。

### M-2. `flushTargets` 缺「回显未完成」守卫——handleBack 在 /state 首帧 5s 超时后走 flush 会拿未合并的 selected 整包 PUT，静默覆盖后端旧目标

- **文件路径:行号**：`web/src/routes/Select.tsx:445-514`（flushTargets 守卫清单）对照 `:534-547`（handleBack 的 5s 回显等待超时后无条件进 flush loop）；防抖回调的回显守卫在 `:618-625`（与 flush 不对称）
- **严重级**：MAJOR
- **一句话问题**：防抖回调开头有「`!echoedRef.current && (stateDataRef undefined || courses 非空) → 置脏 return`」的回显未完成守卫，**flushTargets 却没有同款守卫**。handleBack 的 5s 等待只保证"等待期间回显合并完成"——/state 首帧持续失败超时后，flush 用"只含用户新点击"的 selected 成功通过全部守卫（发布在场、无 stale、非空集）直接 PUT，把后端已保存旧目标整包替换成用户本意"新增"的那几门。
- **触发场景推演**（自洽时序）：
  1. 学生进选课大厅。/electives 成功渲染 publishes=[p1]（旧目标 A@p1 曾由上一会话保存于后端）。
  2. /state 首次请求失败或极慢（校园网/后端重载抖动），5s 内未到达 → `stateData === undefined`，回显 effect 停在 `if (stateData === undefined) return`（echoe 未发生）。
  3. 学生手快点选 B@p1（同发布新增）→ `rev=1`；点「返回」→ handleBack 条件 `rev>0 && !echoedRef && stateData.courses 未定义` 成立 → 等 5s 超时。
  4. flush loop 第一轮：`flushTargets` 读 `latestSelected={p1:[B]}`、`revRef=1`、`publishesRef=[p1]` → 发布缺席守卫放行（p1 在场）、stale 放行、空集放行、`targetsUseCurrentPublishes` 放行 → `saveNow` → **PUT {p1:[B]}**。
  5. 后端 [p1:A] 被整体覆盖为 [p1:B]——用户"加一门"变"替换全部"，A 静默丢失，调度器立刻改按 [B] 行动（开窗瞬间最不该发生的目标漂移）。
  6. 修复后正确行为：flush 命中回显未完成即置脏等待，绝不 PUT。
- **建议修法**（一行）：flushTargets 顶部补与防抖回调同款守卫——`if (!echoedRef.current && (stateDataRef.current === undefined || (stateDataRef.current.courses?.length ?? 0) > 0)) { dirtyRef.current = true; return }`；handleBack 超时后直接放行返回并 toast「未完成回显，目标未保存」。

---

## MINOR（展示 / 健壮性）

### N-1. 名额未公布（max_count=0）时进度条按 `value=selected_count / max=1` 渲染为满条，与"名额未公布"文案并存误导

- **文件路径:行号**：`web/src/routes/Select.tsx:1014-1018`（`<Progress value={c.selected_count} max={c.max_count || 1} indicatorColor={progressColor} />`）对照 `:927` `unannounced` 判定
- **严重级**：MINOR
- **一句话问题**：`max_count=0`（名额未公布）时 `max={0 || 1} = 1`，`selected_count`（数十到数百）除以 1 的直接满条（Progress 组件内部 clamp 100%），同卡片文案明确写"名额未公布"但视觉进度条被撑满红色/琥珀色——展示两套事实。
- **触发场景推演**：任何 max_count 为本期未公布的课程（F30-01 实证的新课程常见形态）渲染即现，无特殊时序。
- **建议修法**（一行）：`max_count<=0` 时 `<Progress max={1} value={0} />` 或干脆不渲染进度条（连同 `aria-valuenow=large` 的意义缺失一并消除）。

### N-2. 网关/代理返回非 JSON 错误体时 401 不广播 UNAUTHORIZED_EVENT，会话失效不触发自动摘除

- **文件路径:行号**：`web/src/api/client.ts:45-51`
- **严重级**：MINOR
- **一句话问题**：`r.json()` 先于 `j.code === 401` 判断——反向代理/网关返回 HTML/文本 401（`content-type: text/html`）时 `r.json()` 抛错走 `ApiError(-2, "服务器响应异常…")`，不派发 `xk:unauthorized` 事件，App.onUnauthorized 不被调用，失效会话的账号永久残留直到下一次成功 JSON 401 或被别的路径剔除。
- **触发场景推演**：后端 12h TTL 过期/手动吊销后，经 Nginx/网关卡在 JS 前的部署下，首个请求被网关以 HTML 401 拦截 → 前端把错误当普通网络错误 toast，会话残留；用户"退出再登录"时旧令牌仍占槽位。
- **建议修法**（一行）：`r.status === 401` 时在 `r.json()` 之前先广播事件（或先 `r.text()` 判定含 401 状态再尝试 JSON）。

---

## OBSERVE（上轮确认成立的观察项 / 无行为差异的结构观察）

### O-1. round40 N-3 仍在：回显合并/发布清理导致的 selected 变化会 resetRetry 清零失败重发退避计时

- **文件路径:行号**：`web/src/routes/Select.tsx:592-595`（防抖 effect 入口 `resetRetry()`）
- **严重级**：OBSERVE（已观察，本轮核实仍在）
- **一句话问题**：`resetRetry` 由「选中/回显合并/清理」任一 selected 变化触发；若此刻恰好有 `scheduleRetry` 正在倒计时（失败重发 2/4/8/16s），一次非用户改动的回显合并就把指数退避清零，重发从 2s 重新开始；极端下连续轮询合并可把重发无限推迟到成功为止。注释已承认该风险（"成功或用户新改动清零"），本轮仅记录未加重。
- **建议修法**：若想把退避语义钉死，可改为仅在「用户动作（pick/清空）」调用 resetRetry，回显派生变化不碰。

### O-2. round41 确认：useTickingCountdown 每秒 `setNow` 驱动 Select/Dashboard 双页整帧重渲染

- **文件路径:行号**：`web/src/lib/useTickingCountdown.ts:10-11`
- **严重级**：OBSERVE（上轮 O-1 延续，Dashboard.tsx:182 注释已承认）
- **一句话问题**：hook 在 `[]` 依赖 interval 内每 1000ms setState → 消费组件子树整帧重渲（Select 的数百张课程卡片网格 / Dashboard 的日期分组矩阵每 1s 全量重建 JSX），与注释「只重渲染倒计时一处」的承诺分叉。500 课程 Scale 下每秒一次大子树 reconcile 在低频老爷机上肉眼可感。行为无错、纯性能，标观察。
- **建议修法**：倒计时切为独立 `<Ticker>` 子组件（React.memo + 秒级 setState 只重建四个数字的 span），或改用 CSS-duration 锚定的纯文本滚动。

### O-3. 结构观察：Dashboard 的 `dateGroups` useMemo 依赖 `[courses, electives?.publishes]`，`courses` 每次 /state 30s 才变一次，重组成本可忽略；`logs` 轮询降频依赖闭包 `state`（Dashboard.tsx:146-147），react-query 每轮重取最新渲染闭包，行为正确。均为"看起来可疑、实测无错"的合规项，记录共证。

### O-4. 上轮 O-2（多 Tab 会话互踢）本期无新证据：同学号多 Tab 各自 Bearer，服务端 `Sessions` 无互踢实现（handler 层未见 RevokeByAccount 类调用、仅逐 session Delete），故不构成前端侧缺陷，记录结案。

---

## 已核对无问题的重点区域

- `Select.tsx` 目标自动保存链（F7-01 仅 rev 驱动 / F13-C1 rev>0 才 PUT / F15-F17 假清空守卫 / F19-01 幽灵过滤 / F31/F35 真合并）：消费时刻 read-ref（selectedRef/revRef/stateDataRef/publishesRef）与防抖 400ms 依赖数组 `[rev, selected, sessionToken, toast, hasPublishes, echoDone]` 全部正确；`hasPublishes` 布尔信号不重置轮询窗口；echoDone 驱动"空 courses 置位后保存链重试"。
- `handleBack` 的等待循环（回显 5s + flush 3 轮 + pendingSaving 21s 等待）：除 M-2 的回显守卫缺失外，`savingRef/dirtyRef/retryState` 串行化与 `flushTargets` 的补发闭环（飞行 PUT 完成 after finally auto-补发）、`unmountedRef` 卸载清理（F13-C2/F20-01 挂载复位）行为正确。
- `targetGuard.ts` 两纯函数（selectedHasStalePublish 空数组键不判过期、cleanStaleSelected 无变化返回原引用防重渲染）逻辑正确、TDD 语义闭合。
- 手动报名/退选在飞幂等 `ReadonlySet<number>`（M29-01）按课程独立跟踪，finally 函数式删除不踩并发；后端 `TryAcquireSubmit` 双守卫闭环。
- 与后端契约对齐：报名/退选 body `{class_id, course_name}` 与 `TargetsRequest{targets}` 结构完全匹配 handler.go 的 JSON 反序列化字段；`?account=` 透传的凭据表校验（B26-02/B27-01/B27-02 与 handleSetTargets 的 B15-M4 同源）在完整链路（读/写/手动操作/状态）四处对称实现；`OpenTime time.Time` RFC3339 序列化与前端 `new Date(openTimeStr)` 解析兼容。
- App.tsx 会话编排：`onUnauthorized` 以 `detail.session` 反查归属、快照式三连落盘（F8-01）、代理态/管理态双复位（F15-07/F25-01/M28-01/M29-02）、`onBackToStudent` 无学生账号完整登出语义（F21-05）；账号迁移 effect `accounts useMemo` 稳定引用。选择渲染树 `key={targetAccount}`/`key={current}` 挂载隔离（F36-01）+ 组件内 accountKey 兜底守卫双保险。
- 321 / 519 行注释中声明的 TDZ 规避（refetchInterval 回调不读组件顶部 stateData、回显 effect 不自引用后声明 publishes）与 TS 校验（es2023 + verbatimModuleSyntax + noUnusedLocals）一并核实通过。
- 可访问性：三个手写模态（登录激活码 / 退选确认 / 删除确认）均带 role=dialog/aria-modal/aria-labelledby/Esc 关闭（激活进行中/删除进行中禁用 Esc 防误关）；CollapseSection `useId`（round40 F40-N2）+ `aria-expanded/aria-controls`；Label 的 htmlFor/autoFocus/aria-pressed/role=switch 全部就位。
- Dashboard 日期分组本地零点语义（parseDateKey/localTodayMs）与 "未知" 兜底排末尾；isFullFallback 仅匹配「failed + 已满员」文案，不含 pending 误判。