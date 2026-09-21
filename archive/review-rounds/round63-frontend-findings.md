# round63 前端只读审查发现报告

> 审查基线：master @ `8251452`（R62 收官，web/src 连续八轮零提交）。本轮开局与收尾 `git status --short` 双确认工作树干净；`git diff HEAD -- web/src/` **零改动（0 行）**、`git log 5f43d5f..HEAD -- web/src/` **零输出**——R55→R63 连续九轮 web/src 未被触碰。审查期间绝对只读（唯一新建文件为本报告；临时验证全部为只读命令，不新建任何仓库内文件，%TEMP% 未落任何临时脚本）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **exit 0**（1948 modules / 417.54 kB js / 41.07 kB css）；TDD 断言 target-guard-check / admin-auth-check / unauthorized-check 全绿（16+6+5）；`node scripts/audit.mjs` **1 处违例（exit 1）**——Button dark variant 同一行 `bg-black`+`hover:bg-neutral-900` 的裸匹配误报，基线既有非本轮引入（见 O-9）。
> 范围 `web/src/` 全部 19 个 `.ts/.tsx`；后端契约对照 `backend/internal/{api/handler.go, scheduler/scheduler.go}`。

## 本轮结论先行

**MAJOR 0 / MINOR 1（新增 M-1）/ OBSERVE 13（N-1 延续第 7 轮 + N-2 延续第 6 轮 + N-3 延续第 5 轮 + O-1~O-10 延续，其中 O-10 并入 N-1 同源）。**

web/src 连续九轮零提交后依旧**零 MAJOR/CRITICAL**。任务书新视角 A-E 全部经实证裁决（A 维持，N-1 落位点字形级复核；B 六防保存链逐字符通读 + 五则新时序推演，其中核心推演「已有目标账号再次编辑」首次完整论证；C 五个实证链逐一复核维持；D 全包逐行通读新增 M-1 一个 MINOR + 新增 O-10（Dashboard 状态机文案）并入 N-1；E 逐条延续）。

**最致命 3 条（按影响排序）**：
1. **M-1（本轮新增 MINOR）**：`shouldDeferSave(courses 非空 && hasSelected)` 在「已回显账号再次编辑」稳态下恒推迟 + 无解锁信号——`/state` 轮询始终携带后端旧目标（仅当引擎把状态推进到不再返回 courses 才停），编辑目标永久不落库。连续九轮全部审查报告将 F42-M1 论证局限在"首帧回显完成前"的暂态，**均未覆盖"回显完成后、课程仍在 courses 内"的稳态**——本轮补齐。触发后无数据丢失（返回后下次进页回显合并恢复 = 改动无声撤销），故判 MINOR；但有明确人为触发路径（进页改动一个备选 → 等 /state 轮询到达 → 再改 → 永不保存），属"合法操作被静默撤销"边缘。
2. **N-1（延续第 7 轮）**：「后台冲刺」文案 vs 后端仅 10 秒黄金期 250ms 冲刺——纯文案、零行为影响；连续七轮维持。若整治 = 两行（Select.tsx:1134 + Dashboard.tsx:648）统一「后台目标」。
3. **O-9（延续）**：Button dark variant `bg-black` 与「实心黑洞清零」护栏相邻；audit.mjs 对同一行 `hover:bg-neutral-900` 的裸匹配误报（exit 1）。维持豁免。

---

## CRITICAL（数据丢失）

（本轮无 CRITICAL。）

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

### M-1.【本轮新增】`shouldDeferSave` 在「已回显账号再次编辑」稳态下恒推迟 + 无解锁——旧目标永驻 courses 期间编辑永不落库（回显完成后改动被静默撤销）

**文件路径:行号**：`web/src/lib/targetGuard.ts:57-61`（判据本体）+ `web/src/routes/Select.tsx:676`（防抖回调消费点）、`:499`（flushTargets 消费点）、`:589`（handleBack 消费点）。

**一句话问题**：`shouldDeferSave(stateData, hasSelected)` = `stateData===undefined || (courses 非空 && hasSelected)` 的语义被设计为「回显未完成守卫」，但 F42-M1/F43-M1 修复后判据只认**数据形态**、不认「回显是否已完成」——**已回显完成（echoedRef=true）且 courses 永驻非空的账号，编辑后恒命中推迟且无解锁信号**：防抖回调置脏返回后 selected 无变化被 React bailout、effect 不重入；`/state` 轮询每次仍带着同一批后端旧目标 → `courses 非空` 恒真 → 依赖 stateData 的重跑也过不了守卫；echoDone 只置一次不再触发。保存链对"后续所有编辑"永久静默拦截。

**证据链**：
1. 判据本体 targetGuard.ts:61 `return stateData === undefined || ((stateData.courses?.length ?? 0) > 0 && hasSelected)`——无 echoedRef/时序维度，只按数据形态判。
2. 后端契约：`PUT /targets`（handler.go:518-522）→ `SetTargetsForAccount`（scheduler.go:459-494）→ `rebuildCoursesForAccountLocked`（575-609）**按 targets 重建 courses**；`StateForAccount`（704-730）按账号过滤 `s.state.Courses` 全量下发。**结论：只要该账号还有 ≥1 个目标课程，/state 轮询的 courses 就永远非空**。
3. F43-M1 论证（round43 findings M-1，28116f3 修复）：当时缺陷是「**全清空**（selectedCount===0）被 courses 非空打回、永不落库、返回后旧目标复活」，修复方案是给 shouldDeferSave 加 `hasSelected` 第二参数——**只解锁了 hasSelected=false（全清空）路径，hasSelected=true 的普通编辑路径在 courses 非空下继续被推迟**，当时与之后多轮均未追问该路径的解锁信号。
4. F42-M1 论证（round42 findings M-1，防抖订阅死锁）：缺陷场景是「/state 首帧失败 + `!echoedRef` 误判」；修复方案是「判据改纯数据 + effect 依赖补 stateData 自愈」——**stateData 驱动的自愈只在"首帧到达/内容首次变化"时有效**：stateData 从 undefined 变为非空 / 轮询首次拉回 courses 会触发一次 effect 重跑，但重跑后同判据继续打回；courses 一旦稳定为同一批旧目标，后续轮询 stateData 引用虽变、效果与首帧相同——**没有"courses 从非空变空"的事件，就没有任何解锁**。
5. 九轮报告全部将 F42-M1 论证局限在「首帧回显完成前/首帧失败」的**暂态**（round42/43 缺陷描述、round56-62 复核表均围绕"首帧未到即点新课""全清空在飞""手动报名后 /state 携带旧 courses"），**从未覆盖「回显已完成后、课程仍在 courses 内」的稳态**——本轮补齐（正是"已看见"与"从未看见"的边界）。

**触发条件**（人为可复现，有明确路径）：
1. 账号已有 ≥1 个后端目标（任意课程在 courses 内），进 Select。
2. `/state` 首帧到达并回显合并完成（echoedRef=true，courses 非空——正常稳态）。
3. 用户改动一个目标（如把某发布备选从 A 换成 B）→ 400ms 防抖 → 守卫命中置脏（`courses 非空 && hasSelected` 恒真）。
4. 此后用户**再做任何改动**：最后一次改动同样被置脏。防抖订阅随 selected 变化重跑、依赖 stateData 的变化重跑，全部同判据打回。**内存目标与后端分叉，永不落库**。
5. 返回：handleBack 5s 等待（shouldDeferSave 恒真）→ flush 置脏 → 三轮循环收敛 → onDone。改动未保存。
6. 下次进页：回显 effect 合并**后端旧目标**（echoedRef 已置位不回放；但 selected 是新建实例，回显合并把后端旧目标补进）→ 用户上次的编辑**无声消失**（被旧目标覆盖，除非用户再手动重改）。

**影响**：
- **无数据丢失**（后端始终是旧的完整目标，改动只是"未落库"）；返回后回显合并把旧目标展示回来 = 编辑被**静默撤销**（用户看不到任何 toast/提示）。
- 触发窗口为「整个窗口开放期 + 关闭后 courses 非空期」——正是用户最常编辑目标的时段；但核心主路径（窗口开放期点官网"报名"按钮）不经过保存链，受影响的是预选目标编辑。
- 触发依赖 `courses 非空`：真清空（hasSelected=false）不触发（F43 已解锁）；`/state` 首帧失败不触发（不落库但后端未变，下次进页一致）。

**修复方向**（若整治，两个候选，任选其一即可）：
1. **判据加入"回显已完成"语义**：`shouldDeferSave` 第三参数 `echoed`（或消费点改判 `echoedRef.current`）——回显已完成（echoedRef=true）即放行普通编辑（合并已发生、selected 完整）。改动最小：三消费点加 `&& !echoedRef.current` 或判据加参数。**风险**：须重新审视「courses 非空 + 已回显 + 用户改动」整包 PUT 的覆盖正确性——selected 已含合并后的全部后端目标（回显合并语义保证），整包 PUT 与后端一致，放行安全；这正是 round62 推演「合并后 selected 已含全部后端目标，即使放行也无覆盖丢失」的结论延伸。
2. **后端 courses 语义区分「待回显」与「已落库目标」**：/state 下发目标时带「是否与前端已保存版本一致」的标记（如 targets_rev），守卫只在 rev 不一致时推迟。改动大，需前后端协同。
3. **最小干预**：维持现状 + 在 flushTargets/防抖置脏时**已有 toast 提示**（F40-M1 守卫已有「发布已更新」toast；本守卫命中时补一条「正在同步，稍后自动保存」toast 让用户知道改动未落库）——缓解"静默"但不解死锁，且新增 toast 可能轰炸。

**保守裁决**：风险真实存在但触发需"编辑不落库后继续编辑"的特定节奏，且无数据丢失；判 MINOR。若未来轮次整治，推荐方案 1（判据加回显已完成维度的最小改动），并同步补 target-guard-check 断言（`shouldDeferSave(stateData非空, true, true)=false`）先红后绿。

---

## OBSERVE

### N-1.【延续，连续七轮】「冲刺」文案与后端仅 10 秒黄金期冲刺的实际行为出入——维持，本轮落位点字形级复核

**证据链**：scheduler.go:70-72 `submitIntervalSprint=250ms / submitIntervalNormal=1s / sprintDuration=10s`；submitIntervalFor（291-294）黄金期判定 `now.After(open) && now.Before(open.Add(sprintDuration))`。前端全部「冲刺」文案点位（字形级枚举）：
1. Select.tsx:1123 注释行「本项目特冲刺/预选目标按钮」；
2. Select.tsx:1134 `已设为后台冲刺${priorityName(selIdx)}` / `设为后台冲刺目标`；
3. Dashboard.tsx:648 `isInRange ? "冲刺提交中"`（isInRange=status in {in_range, submitted}，覆盖黄金期与常态提交段）。

**本轮裁决**：前端无黄金期精确感知（三态伪精确，明确否决做）。Dashboard「冲刺提交中」与后端 `submitted` 状态映射语义一致。连续七轮维持确认：从「未看见」升级为「已看见、故意不做」的完整记录。若做体验整治，极简方向 = 统一「后台目标」删“冲刺”，**两行落位** Select.tsx:1134 按钮文案 + Dashboard.tsx:648 状态行，注释 1123 顺带清理。

### N-2.【延续，第六轮】开窗后 pick → 至多一拍调度延迟（250ms 黄金期 / 1s 常态）。维持。

### N-3.【延续，第五轮】三处格式卫生（英文注释残留 + 两处排版），本轮字形级复证：
1. Select.tsx:1123 注释尾「only affects itself」英文残留；
2. Select.tsx:379 `const lastJson = useRef("")` 缩进 4 空格（邻接 380-382 皆 2 空格）；
3. Dashboard.tsx:501 `</div>                <div>` 同行折叠。

本轮复证：三处与工作树逐字节一致（基线既有非本轮引入），样式/注释卫生级，零运行影响。若做体验整治，三项各一行、零风险。

### O-1.【延续】Toast viewport 滚动交互残余。Toast.tsx:105 `overflow-y-auto pointer-events-none` + Root `pointer-events-auto`（F48-M1）——修复破坏「仅 toast 可交互」；M-2 去重 + 3.5s 自消下同刻>4 条概率趋零。维持。

### O-2.【延续，连续九轮】useTickingCountdown NaN 防御（`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`）——输入源全受控合法（`open_time` 走 Go `time.Time` JSON RFC3339；兜底 `new Date(number).toISOString()` 恒合法）。本轮再核 open_time 序列化链路（scheduler.SchedulerState.OpenTime `time.Time` 标准 JSON、handler 侧 open 解析）依旧无不可信串入口。维持（未来开放时间来源若扩展为不可信字符串，先补 `!Number.isFinite` 再上线）。

### O-3.【延续】三处手写 modal 无完整焦点陷阱（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）。均已有 role=dialog/aria-modal + Esc + autoFocus。维持。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询——bailout + TabsContent 懒渲染实证延续。维持。

### O-5.【延续，连续九轮】Dashboard 挂载点无 `key={account}`（App.tsx:331，Select 两处 293/339 有）→ `expandedDates`（268-273）/`extrasOpen`（212）跨账号残留。后端 StateForAccount（704-730 按账号过滤 Courses）双证数据不串线，影响纯展示层。维持；切号视觉一致性整治时一行 `key={account}` 即可。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button 12 变体/实际子集、Badge 8 变体/实际子集。本轮 grep 复核依旧。维持。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页——无 storage 监听，B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持。

### O-8.【延续】401 保护窗口闭包 current：`lostRaw = detail?.session || detail?.account || current`，session 恒为 401 真实主体、current 兜底永不触发。本轮再核 onUnauthorized 内 `isCurrentAdminSession(loadSessions(), adminName, adminToken)` 每次从 localStorage 快照重读 sessions，维持。

### O-9.【延续】Button dark variant `bg-black` 实色与「实心不透明黑洞清零」视觉护栏相邻（audit.mjs C 组）。

**证据链**：Button.tsx:21 `dark: "bg-black text-white border border-white/25..."`；audit.mjs C 组检查 `min-h-screen bg-black` 与 `bg-black/25|30|70|75`、`bg-neutral-95x`，**不检查裸 `bg-black`**。本轮实测复核：`node scripts/audit.mjs` **exit 1，1 处违例** = Button.tsx:21 的 `hover:bg-neutral-900` 裸匹配命中（同一行同时含 `bg-black` 语义与 hover 过渡色），**误报确认**。Dashboard.tsx:295/735 两处消费 `variant="dark"`（选课大厅按钮与移动端悬浮栏），黑底白字在纯黑画布上视觉成立。

**影响**：零（dark variant 是设计里"黑底白字"语义，画布底色同为 `#000000`）。
**修复方向**：若视觉护栏升级为「裸 `bg-black` 也拦」，dark variant 可改用 `bg-black/40` 半透明或 `bg-[#09090b]` 面板色；当前**倾向维持并放宽 audit.mjs 对该行的豁免**（audit.mjs 属审查脚本，不在 web/src 范围内，本轮绝不修改——本报告唯一新建文件原则）。

### O-10.【并入 N-1 同源】Dashboard 状态机四态文案的「提交中」与后端 `submitted`/`in_range` 双状态映射——纯展示观察。

**证据链**：Dashboard.tsx:589 `isInRange = c.status === "in_range" || c.status === "submitted"`；609-616 两态统一渲染「提交中」，648 行 `isInRange ? "冲刺提交中"`。后端 scheduler 状态机 `pending / in_range / submitted / success / failed`（五态）——`in_range` 为「窗口开放、排队待提交」（本次核对：仅注释声明，scheduler.go 全源码无 in_range 写入点，实际状态流转为 pending→submitted→success/failed，in_range 属预留态）、`submitted` 为「已发出报名请求」（scheduler.go:1470 唯一写入点）。前端把两态合并为「提交中」展示，语义成立。与 N-1 同源（冲刺文案），零行为影响。维持；若随 N-1 一并整治，统一改「后台提交中」。

---

## 重点核对结论（任务书逐条裁决）

### A. N-1 / N-2 / N-3 / O-1~O-10 延续复核——维持，落位点字形级复核

三连确认：① 前端无黄金期精确感知 → 三态文案伪精确（负收益，明确否决）；② 黄金期后用户主通道是官网「报名」primary 大按钮（Select.tsx:1110-1122），冲刺 ghost 小按钮是次要入口；③ 若治 = 两行：Select.tsx:1134 + Dashboard.tsx:648 统一「后台目标」删“冲刺”，注释 1123 顺带清理。O-9 本轮实测 audit.mjs exit 1（误报确认，维持豁免）。N-2/N-3 同源维持。O-1~O-8、O-10 逐项复核机制实证相同（见上表）。

### B. 六防保存链逐字符零回归（F43/F42/F40/F39/F36/F48-M1）——含本轮核心新推演

| 防护族 | 本轮结构与语义复证 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 三消费点同源（Select.tsx:499/589/676）；断言 6 项覆盖全清空放行（scripts/target-guard-check.ts:42-67，本轮实测 16/16 全绿） |
| **F42-M1 判据解耦** | 纯数据判据不依赖 echoedRef；防抖 effect 依赖含 stateData（Select.tsx:737）自愈；三处消费时刻读 stateDataRef.current 快照（499/589/676） |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（287/316-327/518/696）全在；独立清理 effect（316-327）依赖含 selected；toast 判 `!unmountedRef.current` 不轰炸卸载后 |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）；断言场景 C/D 覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:293/339 双 keyed + accountKey 守卫（Select.tsx:195-202）TDZ 安全 |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow |

关键时序推演（本轮五则，均无数据丢失路径）：

- **【本轮核心新推演】「已有目标账号再次编辑的稳态推迟」→ 见 M-1**：`/state` 轮询永驻携带后端旧目标（后端契约：StateForAccount 704-730 按账号过滤全量下发；SetTargetsForAccount→rebuildCoursesForAccountLocked 575-609 按 targets 重建 courses，只要还有目标 courses 就非空）→ `shouldDeferSave(courses 非空 && hasSelected)` 恒推迟 → 防抖回调置脏返回后 selected 无变化被 React bailout → effect 不重入；stateData 依赖的重跑同判据打回 → **无解锁信号**。F42-M1（round42）与 F43-M1（round43）的论证均只覆盖「首帧回显完成前」暂态，本轮首次补齐稳态论证。
- **【本轮复核】「/state 首帧未到即点新课 → 回显合并 → 推迟命中」**：合并后 selected 完整（新课 + 旧目标）→ 守卫推迟 = 安全方向（此刻 PUT 整包与后端一致，即使放行也无覆盖丢失）；解锁路径 = /state 轮询 courses 空 / handleBack 5s / 后续改动。维持。
- **【本轮复核】「全清空 PUT 在飞 + 首帧携带旧目标（courses 非空）到达」**：回显 effect `rev>0 && !anyHas` 分支 return prev 不合并；echoedRef/echoDone 置位；防抖重跑 `shouldDeferSave(stateData, hasSelected=false)` → false 放行 → PUT [] 落库 → 清空语义完整保存，绝不复活。双闸闭环确认。
- **【本轮复核】「手动报名后 /state 轮询携带旧 courses 的推迟语义」**：actionLoading invalidate state → /state 下一拍返回旧 courses → 防抖 effect 随 stateData 变重跑 → 守卫命中推迟（安全方向：selected 已含后端目标，推迟抑制无害）。解锁：/state 2s 轮询 courses 空 → effect 重跑放行；或 handleBack 5s 等待收敛。不丢失、不覆盖。
- **【本轮复核】「saveNow lastJson 去重链」**：Select.tsx:429 `if (json === lastJson.current) return`——lastJson 只记录"上一次成功保存的 json"；全清空 PUT [] 成功后 lastJson="[]"，用户立刻点新课产生新 json（含新课程）≠ "[]" → 正常再发。若旧 PUT 在飞（savingRef=true）→ 走 dirty 分支，飞行 PUT 完成 finally 补发最新快照。无乱序覆盖路径。

**核心推演正面确认**：六防链在「首帧/全清空/在飞/发布重建」四类场景下零回归——除 M-1 的「已回显稳态编辑」外无推迟死锁路径；M-1 无数据丢失但有静默撤销，判 MINOR 并给出最小修复方向。

**TDZ**：防抖 effect（645-737）同步体不引用后声明物；`const lastJson` 缩进瑕疵不改语义。

### C. 五个实证链延续（R62 报告附加实证）——逐一复核维持

1. **pick() fallback 判定源恒最新渲染**：Select.tsx:346 `pick(t.publish_id, c)` 回调参数来自渲染期 map 的 `t`；`pick` 是事件处理器（每次渲染新定义），`selected` 为渲染闭包当前值，与 `arr = [...(selected[publishId] ?? [])]` 的"先快照后 set"语义一致。维持。
2. **Dashboard dateGroups 纯 useMemo**：Dashboard.tsx:219-263 依赖 `[courses, electives?.publishes]`，两输入均为 react-query 缓存引用（数据不变引用不变）；`pubById`/`byDate`/`groups` 全在 useMemo 内部新建、无外部突变；`key={g.key}`/`key={pub.publish_id}`/`key={c.class_id}` 均唯一。维持。
3. **App onUnauthorized 闭包**：`adminName/inAdmin/adminToken` 每改重建 effect 闭包（依赖数组 235）；`onUnauthorized` 每次从 localStorage 快照重读 sessions 反查 lostAccount，无陈旧状态污染。维持。
4. **共享 react-query 缓存两路由**：Dashboard.tsx:171-175 与 Select.tsx:56-57 同 `queryKey: ["electives", account, sessionToken]`——Dashboard 轮询恒 30s、Select 轮询按窗口信号升/降频，同一缓存被两个轮询调度器管理（react-query 合并为最近 refetchInterval）。切页不重复请求、开窗后 Select 侧升频即时接管。维持。
5. **saveNow lastJson 去重链**：见 B 表推演。维持。

### D. 全包逐行通读找新问题

**结论：新增 M-1 一个 MINOR + O-10 并入 N-1。** 除 M-1 外零新增缺陷。逐文件通读要点：

1. **后端状态机核对（本轮重点新发现）**：`in_range` 状态在 scheduler.go 全源码**无写入点**（仅 scheduler.go:38 注释声明为五态之一）——实际流转为 `pending`（重建/恢复）→ `submitted`（spawnChain 1470）→ `success`/`failed`；Dashboard.tsx:589 的 `isInRange = status==="in_range" || status==="submitted"` 中 in_range 属预留态（恒 false），submitted 为真实覆盖。前端合并展示「提交中」语义成立、零行为影响（O-10）。
2. **后端 `/state` 契约与 M-1 关联**：StateForAccount（704-730）`st.Courses = nil` 后按账号过滤全量下发 `s.state.Courses`——**只要该账号还有目标课程，courses 就永非空**；这就是 M-1「courses 非空」恒真的后端根因。
3. **手动报名/退选双端幂等**：actionLoading Set<number> 独立跟踪、函数式删除只清自己 id（Select.tsx:52/92-93/106-110/119-120/131-135）；后端 TryAcquireSubmit（1892-1903）inflight 单课程锁。零缺陷。
4. **401 三形态单广播**：client.ts:64-69（HTTP 401 前置）+ 75-95（body 401 仅 `r.status !== 401` 补）；App.onUnauthorized 幂等（187-235）。零缺陷。
5. **Admin 五 Tab**：CodesTab removing Set 按码独立/ConfigTab `!loaded` 拒存 + refetch 成功才自增 epoch/StatsTab `window_closed` 三态 + `token_valid` 部分失效/AccountsTab targets 空数组兜底/LogsTab limit=200。本轮新增核对：CodesTab 生成数量 max=100 与后端 handleAdminCodes `req.Count < 1 || > 100` 拒绝边界一致；Uses 无前端 max 硬上限、由后端 1000 兜底。零缺陷。
6. **倒计时条件顺序**：Select.tsx:809-835「未识别」分支（821）先于 `cd.isExpired`（826）；Dashboard 文案行同款顺序正确（407-413）。
7. **刷新/回显/卸载生命周期**：unmountedRef StrictMode 复位（398-407）+ F20-01；echoedRef 永不重放（226-297 首行短路）；selectedRef/revRef/stateDataRef 每渲染同步（173-181）。零缺陷。
8. **Session/激活链**：Login 1001 分支保存 ticket（54）+ 激活回传（78）+ F12-M3 票据过期引导（87-97）；ticket 过期/已用与激活码错误两分支均清 pendingTicket（87-97），服务端 ConsumeTicket 先销毁票据的语义闭环。零缺陷。
9. **Dashboard 日志/状态卡片**：F10-06 去掉硬编码「/ 3 门」；logs limit=100 与后端 LoadLogs 上限一致（691 行 RECENT 100 文案）。零缺陷。

### E. 上轮观察项延续复核（全表）

| 编号 | 本轮核实 | 裁决 |
|---|---|---|
| N-1 | scheduler.go:70-72/291-294 冲刺实测 + Select.tsx:1134/Dashboard.tsx:648 两落位点字形级复核 | **延续**（第 7 轮） |
| N-2 | pick→PUT/targets→tick 提交链路 | **延续**（第 6 轮） |
| N-3 | 三处格式瑕疵基线既有性复证 | **延续**（第 5 轮） |
| O-1~O-8 | 逐项复核机制实证相同，无新增变化 | **延续** |
| O-9 | Button dark `bg-black` + audit.mjs 对 `hover:bg-neutral-900` 裸匹配误报（本轮实测 exit 1 确认） | 维持（豁免） |
| O-10 | Dashboard 状态机「提交中」合并 in_range/submitted 两态（本轮补 in_range 无写入点实证） | 维持，并入 N-1 同源 |

---

## 已核对无缺陷的高风险区域清单

- 六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读：除 M-1 的「已回显稳态编辑推迟」外零缺陷；F43 全清空、F40 发布重建清理、F36 keyed 挂载、F48-M1 Toast 定位全部在位。
- 后端 `/state`/`/targets`/`/electives` 契约对照：状态机五态枚举、StateForAccount 按账号过滤、SetTargetsForAccount→rebuildCourses 链路、beginTimes 识别槽语义。
- 401 三形态单广播 + onUnauthorized 幂等 + 会话快照式三连（login/logout/onDeleted）。
- Admin 五 Tab 全量：激活码生成/删除在飞幂等、配置保存 !loaded 拒存、账号删除 Dialog 二次确认、日志 limit 对齐。
- 倒计时三态顺序、回显/防抖/刷新/卸载生命周期、手动报名/退选双端幂等、pick 渲染闭包、Dashboard dateGroups 纯 useMemo。
- 共享 react-query 缓存两路由、saveNow lastJson 去重链、Toast M-2 去重 + 自增 id。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | exit 0（1948 modules / 417.54 kB js / 41.07 kB css） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | 16/16 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 视觉护栏 | `node scripts/audit.mjs` | exit 1，1 处违例（Button dark `hover:bg-neutral-900` 裸匹配误报，基线既有，见 O-9） |
| 后端契约核对 | `grep scheduler.go` 状态机/submitInterval/StateForAccount/SetTargetsForAccount | in_range 无写入点；courses 永驻契约确认（M-1 根因） |
| 工作树一致性 | `git status --short` | 报告写入与构建后均零输出 |
| web/src 零改动 | `git diff HEAD -- web/src/` | 0 行 |
| 跨轮次零提交 | `git log 5f43d5f..HEAD -- web/src/` | 零输出 |
| 临时文件 | `%TEMP%` 与仓库 | 未新建任何临时脚本（全部验证为只读命令）；仓库内仅本报告 |
| 报告落盘核验 | `Write round63-frontend-findings.md` | 文件已写、正文完整（含验证实证表） |

---

## 教训

1. **「安全方向」论证必须区分暂态与稳态**：F42-M1/F43-M1 的「courses 非空 → 推迟」判据在「首帧回显完成前」是安全方向，但在「回显完成后、课程仍在 courses 内」的稳态下会变成永久推迟且无解锁信号。连续九轮审查全部聚焦暂态推演，无人追问稳态——**审查走读的盲区不在代码本身，而在"把同一判据的两个语义阶段当成一个"**。
2. **纯数据守卫的自愈信号必须"状态可达"**：F42 的 stateData 依赖自愈依赖「courses 从非空变空」或「从 undefined 变非空」的事件；当 courses 永驻非空时，stateData 引用变化虽然触发 effect 重跑，但同判据打回 = 自愈信号不可达。**设计守卫时除了问"什么信号触发重跑"，还要问"重跑后判据能否翻转"**。
3. **后端契约是前端守卫语义的上界**：M-1 的根因一半在后端——`StateForAccount` 恒下发 courses（有目标就非空）。前端守卫想表达「回显未完成」却只能用「courses 非空」代理，契约上就无法区分「待回显」与「已落库」。未来若整治，优先让守卫获得"回显已完成"维度（前端 echoedRef 即可，无需后端改动）。
