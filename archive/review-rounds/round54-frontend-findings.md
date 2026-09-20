# round54 前端只读审查发现报告

> 审查基线：master @ `7e8900e`（R53 收官，工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 R53 相关文件）。审查期间绝对只读（未创建/修改/删除任何文件，唯一新建文件为本报告）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **成功**（tsc -b + vite 全绿，1948 modules，产物 417.43 kB js / 41.04 kB css，构建后 `git status --porcelain -- web/` **零输出**）；TDD 断言脚本 `npx jiti scripts/target-guard-check.ts` **16 项全绿**、`admin-auth-check.ts` **6 项全绿**、`unauthorized-check.ts` **全绿**。
> 范围 `web/src/` 全部 `.ts/.tsx`（18 文件）；后端契约对照 `backend/internal/api/handler.go`（handleElectives 235-277 / handleState 519-539 / handleAdminStats 865-954）、`backend/internal/scheduler/scheduler.go`（StateForAccount 704-730 / openTimeForLocked 432-439 / ElectivesSnapshotFor 766-799 / probe 探测量入账 1097-1107）、`backend/internal/zhidao/client.go`（parseElectives 621-662）、`backend/internal/store/store.go`（LogEntry 212-219）。
> Radix 运行时实证：`web/node_modules/@radix-ui/react-tabs/dist/index.js:193-224`（TabsContent → react-presence）、`web/node_modules/@radix-ui/react-presence/dist/index.js:55-62`（`present=false` 返回 null → 非激活 Tab 子树**从 React 树卸载**）、`web/node_modules/@radix-ui/react-toast/dist/index.js:362-404`（duration 生命周期 / PAUSE-RESUME 机制 / 重挂载即 reset）。

## 本轮结论先行

**MAJOR 1 / MINOR 4 / OBSERVE 7（其中 1 条为 O-1 延续 + 6 条新视角/延续）。** R53 前端三修（M-1/M-2/M-3）**全路径复核通过（8 条核验零缺陷）**；R53 改动未引入任何保存链回归（F43/F42/F40/F39/F36/F48-M1 六件套逐字符复证零回归）。

唯一 MAJOR 为 **M-A（Select 横幅在识别缺席 + 未来 begin_times 场景的"本地已到开窗点"文案永久假活）**——F15-03 对 Dashboard 横幅的分支顺序（window_closed → window_opened → openTimeStr → isExpired）在 Select 侧被**旧版遗留顺序**（window_closed → window_opened → !openTimeStr → isExpired）取代，导致 Select 横幅从不走 `cd.isExpired` 分支；而 F53-M-1 的 begin_times 兜底**没有**给横幅分支加补丁，识别缺席 + 未来开窗点场景下 Select 横幅永远显示"未识别到开放时间（平台尚未下发或识别已过期）"，与同屏主矩阵吃 begin_times 兜底显示的"距开放还有 X 分 X 秒"**自相矛盾**（"未识别"文案 + 确切倒计时并存）。M-2 去重还有一个 MINOR 级的 **duration 覆盖合并**（M-B）。

最致命 3 条（影响排序）见文末结论节。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

### M-A.【跨页横幅矛盾】Select 横幅分支顺序遗留旧版——识别缺席 + 未来 begin_times 时永远显"未识别到开放时间"而非倒计时

**一句话问题**：Dashboard 横幅分支是 `window_closed → window_opened → openTimeStr → cd.isExpired`（cd.isExpired 分支在 openTimeStr 有值时触发），而 **Select 横幅分支是 `!openTimeStr`（没有 isExpired 分支）**——R53 修复了 Select 主矩阵的兜底但**没有**同步横幅分支。识别缺席 + 未来 begin_times 场景下，Select 主矩阵已通过兜底显示确切倒计时，但横幅永远走 `!openTimeStr` 分支显"未识别到开放时间"，与同屏倒计时自相矛盾。

**证据链**：
1. Select.tsx:821-825：`!openTimeStr ? <span>未识别到开放时间（平台尚未下发或识别已过期）</span>`。**这个分支在 openTimeStr=null（识别缺席）时恒命中**。
2. Select.tsx:826-827：`cd.isExpired ? "本地已到开窗点..."` ——该分支在 `!openTimeStr` 分支之后，**当且仅当 `openTimeStr` 有值**时才会到达（因为 `!openTimeStr` 已提前 return）。识别缺席时（openTimeStr=null）这个分支**永远执行不到**。
3. F53-M-1（Select.tsx:749-754）：`useTickingCountdown(openTimeStr ?? (data?.begin_times?.[0] != null ? new Date(data.begin_times[0]).toISOString() : null))`——主矩阵在识别缺席时吃 begin_times 兜底，cd.isExpired=false、显"距开放还有 X 分 X 秒"。
4. Dashboard.tsx:335-346 横幅：`window_closed → window_opened → "待命中"` 三段，无 openTimeStr/isExpired 分支（Dashboard 的 isExpired 场景已被 window_opened 接管）。Dashboard 的矩阵（Dashboard.tsx:190-195）与文案行（407-414）**双双**吃兜底，自洽。
5. 触发：识别槽未建立 / 识别值已过期（`open_time_known=false`），但平台已下发 `begin_times[0]`（未来开窗点）。这是开窗前 10 分钟的最关键盯守窗口——用户切到 Select 页想确认距离开窗还有多久，主矩阵显示"距开放还有 5 分 12 秒"（F53-M-1 兜底生效），横幅却说"未识别到开放时间（平台尚未下发或识别已过期）"。
6. 影响：纯展示层矛盾（跨页+同屏双重），**无数据破坏、无保存链影响**。定 MAJOR 因为这是 R53 修复（M-1）自身引入的**不完整修复**——R53 M-1 修了矩阵的兜底却没修横幅的分支顺序，且横幅 `cd.isExpired` 分支在修复前就是死代码（Select.tsx:826 的分支**从来不可达**）。F53-M-1 的修复意图是"识别缺席时给用户确切倒计时"，但横幅仍按旧逻辑报"未识别"，语义自相矛盾，用户会认为"主矩阵是编造的"而错过盯守窗口。从影响面看：跨页矛盾（M-1 的原始动机）其实**只解决了一半**。

**修复方向**（摇梯最小档）：Select 横幅分支改与 Dashboard 一致——把 `!openTimeStr` 分支改为在**兜底也缺席**时才显"未识别"：`!cd 有值（即 openTimeStr 为 null 且 begin_times[0] 也为 null）` 才走"未识别"分支；`cd.isExpired` 分支保留在 openTimeStr 无值但 begin_times 兜底有值时触发。具体一行：把横幅分支从 `!openTimeStr ?` 改为 `!openTimeStr && !(data?.begin_times?.[0] != null) ?`（与矩阵兜底条件同源）。F15-03 对 Dashboard 的 isExpired 语义（"本地已到开窗点，等待平台窗口开放"）完全适用于 Select 同场景。

**TDD 形态**：纯函数断言脚本（`web/scripts/` 惯例）：抽 `resolveBannerOpen(openTimeStr, beginTimes)` 返回 `"unknown" | "open" | "closed" | "pending" | "countdown" | "expired"`，断言"识别缺席 + 未来 begin_times → countdown（而非 unknown）"、"识别缺席 + begin_times 缺席 → unknown"、"识别有效 → countdown"、"window_opened → open 优先"、"window_closed → closed 优先"。tsc -b + 构建验证即可。

---

## MINOR（展示 / 边界一致性 / 协议冗余）

### M-B.【体验/一致性】Toast 去重合并的 duration 取最新——同 title 长文案 toast 被新短 duration 覆盖缩短显示

**一句话问题**：M-2 去重合并时 `duration: msg.duration ?? prev[idx].duration`——新调用的 duration 会**覆盖**已展示 toast 的 duration，而 Radix Toast 的 duration 是 mount 期一次性（`startTimer` 在 mount effect 中读 duration），合并后**原 toast 的关闭计时器已按原 duration 在跑**，被覆盖的 duration 对已挂载 toast 无任何效果，却会造成**误读**——去重后长文案 toast 看似配置了新短 duration 但实际仍按旧值计时，行为与配置分叉。

**证据链**：
1. Toast.tsx:47：`merged[idx] = { ...prev[idx], description: msg.description, variant: msg.variant ?? prev[idx].variant, duration: msg.duration ?? prev[idx].duration }`——duration 在合并时取最新。
2. Radix 实现实证（`web/node_modules/@radix-ui/react-toast/dist/index.js`）：`startTimer(duration)` 在 mount effect（402-404 行）读 duration 并 `setTimeout(handleClose, duration)`；后续 duration 变化触发 effect 重跑（依赖数组含 duration，404 行）**先 clearTimeout 再 startTimer(新duration)**——**duration 变化确实重启计时器**。但去重合并发生在 setToasts 更新（React state），Toast.Root 的 duration prop 变化会触发 effect 重启——**这反而意味着合并后的 duration 生效了**。
3. **但注意关键细节**：去重合并时 **key（title）稳定、id 不变**（`merged[idx]` 保留原 id）→ React key 不变 → Toast.Root **不被卸载重挂**，duration prop 变化走 effect 重跑重启计时器（新 duration 生效）。所以 M-B 的"覆盖无效"在 Radix 实现下**不成立**。
4. **真正的问题**：`duration ?? prev` 的覆盖语义在**同一 toast 生命期内的多次合并**下会反复重启计时器。场景：目标保存失败 46s 退避链，每次失败都 `toast({title:"目标保存失败", duration: 3000})`（设想），第一次 3500ms 合并后续 3000ms——Radix 重启计时器后 toast 在 3s 消失；但合并本身每次 setToasts 都触发重渲染、重启计时器，**只要合并频率 > duration，toast 可以无限期存活**（50ms 一次合并 + 3s duration → 永不消失）。当前代码没有任何调用点传 duration（grep 全仓 `duration:` 在 toast 调用点零命中），故**当前无实际触发**，纯潜在语义洞。
5. 影响：当前无触发（全仓零 duration 传参）。但 M-2 的"合并取最新 duration"语义在未来有调用点传入 duration 时会引入"合并重启计时器 = 延寿"的潜在行为，与注释"取最新"的预期（缩短）相反。定 MINOR（协议冗余/未来语义洞）。

**修复方向**：合并分支保留原 duration（`duration: prev[idx].duration`）——同 title toast 是"更新文案"语义，duration 应保持首次挂载的配置（用户预期"看到这条消息这么久后消失"）；新 duration 只对真正的新 toast 生效。与 `variant: msg.variant ?? prev[idx].variant`（更新视觉）的"取最新"意图区分。

**TDD 形态**：`web/scripts/toast-dedup-check.ts`（R53 M-2 的修复脚本若已建）：补场景"合并时不改 duration"断言（第二次合并后 duration 仍是首次值）。

### M-C.【展示边界】Toast viewport 无最大条数上限——不同 title 高频并存时无限堆叠

**一句话问题**：M-2 只按同 title 去重，不同 title 的 toast 照常追加；viewport（Toast.tsx:102）无 max 条数逻辑，失败重试链 + 选课操作 + 轮询提示等不同 title 高频并发时 viewport 可无限堆叠。

**证据链**：
1. Toast.tsx:50-51：`return [...prev, { ...msg, id: ... }]`——不同 title 恒追加，无上限。
2. Toast.tsx:102：viewport `max-w-[380px] flex-col gap-2`，无 max 条数裁剪/滚动；叠满 380px 高度后新 toast 挤压旧 toast（Radix viewport 是 flex column，无 overflow）。
3. 全仓 toast 调用点 16 处（grep 实测），title 去重合并后：报名失败/退选失败/目标保存失败/激活失败/删除失败/生成失败/保存失败 = 7 个不同 destructive title，叠加"已设为首选/已取消目标/发布已更新"等不同 default/warning title，黄金期（开窗瞬间 2s 高频轮询 + 用户高频点选）可短时间堆叠 5-8 条不同 title。
4. 影响：体验级（红条互相遮挡、误以为操作失败），无数据破坏。定 MINOR。

**修复方向**：viewport 内加 `max-h` + overflow-y-auto，或 ToastProvider 内限 max 条数（如 4 条，新来挤掉最旧）。摇梯最小档：viewport className 补 `max-h-[80vh] overflow-y-auto` 即可防无限堆叠（不新增状态逻辑）。

**TDD 形态**：纯展示级，tsc -b + 构建验证即可（无需额外脚本）。

### M-D.【跨页一致性】Select 右栏"预计开放时间"文案未吃 begin_times 兜底

**一句话问题**：Dashboard 文案行（Dashboard.tsx:407-414）已按 R53 之前就吃 `begin_times[0]` 兜底（F39-N1 的配套），而 Select 右栏（Select.tsx:838-841）仍是 `stateData?.open_time_known && openTimeStr ? new Date(openTimeStr)... : "未知"`——识别缺席 + 未来 begin_times 场景下，Select 主矩阵（F53-M-1 已兜底）显示确切倒计时，右栏却显"未知"。

**证据链**：
1. Select.tsx:838-841：`{stateData?.open_time_known && openTimeStr ? new Date(openTimeStr).toLocaleString(...) : "未知"}`——无 begin_times 兜底。
2. Dashboard.tsx:407-414：`{openTimeStr ? new Date(openTimeStr)... : electives?.begin_times?.[0] != null ? new Date(electives.begin_times[0])... : state ? "未识别到开放时间" : "正在同步..."}`——已吃兜底（F39-N1 配套）。
3. 触发：与 M-A 同场景（识别缺席 + 未来 begin_times），Select 页主矩阵显示倒计时、横幅显"未识别"（M-A）、右栏显"未知"——三处三种状态。
4. 影响：与 M-A 同族展示矛盾，无数据破坏。定 MINOR（M-A 的关联缺口，M-A 修复时一并对齐）。

**修复方向**：Select 右栏与 Dashboard 同构：`new Date(openTimeStr ?? begin_times[0] 格式化)`，兜底条件与矩阵（F53-M-1）同源；都不在才"未知"。

**TDD 形态**：与 M-A 共用 `resolveBannerOpen`/新增 `resolveExpectedTimeLabel(openTimeStr, beginTimes)` 纯函数断言（tsc -b 即可）。

### M-E.【类型契约】types.ts 未反映 N3/N5 注释承诺——`open_time_set` 与 `token_valid` 在 AdminStats 中可选但 StatsTab 消费缺兜底

**一句话问题**：types.ts 的 AdminStats `open_time_set?: boolean` / `token_valid?: Record<string, boolean>` 是可选字段（注释"N3/N5 缺省视作..."），但 StatsTab 消费 `s.captcha_engine === "ddddocr"` 与 `s.open_time_set === false` 时**假定字段一定存在**——后端 handler.go 实际总是下发这两个字段（writeJSON 固定键），故当前无实际触发；但若未来后端移除该键（可选标记已声明"可能缺失"），前端 `s.open_time_set === false` 会把 undefined 误判为"已识别"、`Object.values(s.token_valid ?? {})` 正确兜底——不一致。

**证据链**：
1. types.ts:119-128：`captcha_engine?: string` / `captcha_concurrency?: number` / `open_time_set?: boolean` / `token_valid?: Record<string, boolean>` 全部可选。
2. Admin.tsx:713：`value: s.open_time_set === false ? "未识别" : s.open_time`——`undefined === false` 为 false → 显 `s.open_time`（空串或识别值）。若后端不返回该键，undefined 被当"已识别"展示空串。
3. Admin.tsx:725：`value: s.captcha_engine === "ddddocr" ? ... : "硅基流动 Vision"`——`undefined === "ddddocr"` false → 显 Vision。后端 handler.go:949 恒下发该键（空串兜底 vision），故当前无实际触发。
4. 后端实证：handler.go:949-952 恒下发 `captcha_engine`/`captcha_concurrency`/`token_valid`/`open_time_set` 四个键（writeJSON map 固定键）。
5. 影响：无当前实际触发（后端恒下发）；类型标记与消费假定不完全一致，属防御性类型契约弱化。定 MINOR（类型契约/防御深度）。

**修复方向**：AdminStats 中 `open_time_set` 改必选（后端恒下发，types.ts:126 注释"缺省视作全部有效"的兜底描述与后端固定键矛盾）或 StatsTab 消费补 `s.open_time_set !== true` 判定（undefined 保守视为"未识别"）。摇梯最小档：`open_time_set !== true`。

**TDD 形态**：tsc -b 类型门（改必选后 StatsTab 消费无需变）。

---

## OBSERVE（观察项，未加重）

### O-1.【修正/延续】Admin 五 Tab 单激活查询轮询——R53 O-1 实证复核通过

**复核结论**：R53 O-1 已修正 round52 O-5"四查询同跑"为前提错误。本轮用 node_modules 产物逐行再实证：
- `@radix-ui/react-tabs/dist/index.js:205`：`<Presence present={forceMount || isSelected} children={({present}) => <div hidden={!present} children={present && children} />}>`。
- `@radix-ui/react-presence/dist/index.js:55-62`：`const forceMount = typeof children === "function"` → TabsContent 是 render-prop（children 为函数）→ `forceMount=true` → 恒 `cloneElement(child, {ref})` **不卸载**，但 `hidden={!present}` + `children: present && children` → 非激活 Tab 的**内容子树不渲染**（`present && children` 为 false，子组件不 mount）。
- **关键再核**：TabsContent 自身是函数组件、**恒挂载**（Presence cloneElement 保留组件实例），但 `present && children` 让子 Tab 组件的 **useQuery 观察者从未 mount**（React 不渲染不存在的组件）→ 查询无观察者 → 不轮询。
- ConfigTab 无 refetchInterval（Admin.tsx:493-496）确认；activeTab 受控（Admin.tsx:56）切换即旧卸载/新挂载。
- **裁决**：R53 O-1 修正成立、本轮再实证通过。管理员停留任一 Tab 时轮询 = 该 Tab 单查询。

### O-2.【延续】useTickingCountdown 每秒整页重渲——量化收敛可接受

**复核结论**：同 R53 O-2 结论。本轮补 React 19 语义再核：`setNow(Date.now())` 每秒触发消费组件重渲，bailout 机制使叶子引用不变不写 DOM。维持 OBSERVE。

### O-3.【延续】Dashboard expandedDates 跨账号残留 + 挂载 key 不对称

**复核结论**：App.tsx:331-337 Dashboard 挂载点仍无 `key={account}`（Select 两处 App.tsx:293/339 有）——账号切换原地重渲，`expandedDates` 残留旧账号折叠态。维持 OBSERVE。

### O-4.【延续】手写模态无焦点陷阱（F6-02 Radix Dialog 锚点）

**复核结论**：三处手写 modal（Login 激活 / Select 退选 / Admin 删除）仍无焦点陷阱；F6-02 锚点注释 Login.tsx:222 仍在。维持 OBSERVE。

### O-5.【延续】Dashboard/Select 倒计时兜底依赖稳定性——react-query 引用稳定 + openTimeStr 主源优先，无无谓 setNow

**复核结论**：
1. Select.tsx:749-754：`data?.begin_times?.[0]` 读 react-query 缓存（`queryKey: ["electives", ...]`）。react-query 数据不变引用不变（`query.state.data` 引用只在 refetch 后新对象才变）→ `begin_times[0]` 值只在 refetch 后可能变。
2. /electives 轮询：Select 的 `refetchInterval` 在窗口开放中为 2s、关闭后 30s。每次 refetch 成功后若 `begin_times[0]` 值不变 → `new Date(begin_times[0]).toISOString()` 结果**字符串值不变** → `useTickingCountdown` 的 target 不变 → target effect（useTickingCountdown.ts:16-18）不重跑、`setNow` 不触发。**无无谓 setNow**。
3. 同值字符串：`toISOString()` 对相同毫秒值恒产生相同字符串 → 引用无关、按值比较稳定。
4. `openTimeStr`（识别真值）优先：识别建立瞬间（open_time_known 从 false→true）`openTimeStr` 从 null 变为识别值 → target 变 → effect 触发一次 `setNow`（F10-07 意图：target 变化立即校正 now）——这是**必要**的校正，不是无谓。
5. **结论**：无无谓 setNow，依赖稳定。维持 OBSERVE（非缺陷）。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体

**复核结论**：grep 复证——`CardFooter`（Card.tsx:46-51）全仓零消费；Button `invert/success/warning/secondary/icon`、Badge `default/success/warning/destructive/secondary/active` 死变体仍在。维持 OBSERVE。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页生命周期

**复核结论**：无 storage 事件同步；多标签页共享 localStorage 的 B 标签登出清标记 → A 刷新回学生端。方向安全。维持 OBSERVE。

### O-8.【延续】F51-O1 注释残留缩至一处

**复核结论**：grep `Radix Dialog|F6-02` 仅命中 Login.tsx:222 一处。维持观察。

---

## 本轮新视角六项——逐条裁决

- **A. Toast 去重后的 duration 生命周期** → 见 M-B/M-C：Radix duration 变化会重启计时器（实证 402-404 行），故"覆盖 duration"实际生效但语义洞是"合并重启计时器 = 潜在延寿"（当前无调用点传 duration，纯未来洞）；viewport 无 max 上限（M-C）。
- **B. Select 倒计时兜底依赖稳定性** → 见 O-5：值比较稳定、无无谓 setNow、识别建立瞬间的必要校正正确。**无缺陷**。
- **C. Dashboard/Select 双页倒计时一致性深核** → 两页主矩阵同构（O-5 确认），但**横幅**（M-A）与**右栏**（M-D）不同构——识别缺席 + 未来 begin_times 场景下三处展示互相矛盾。**M-A 是本轮核心发现**。
- **D. Admin 五 Tab 懒渲染复核** → 见 O-1：node_modules 产物实证 `present && children` 条件渲染 → 非激活 Tab 子组件不 mount → 查询无观察者不轮询；ConfigTab 无 refetchInterval；activeTab 受控切换资源清理正确。**R53 O-1 修正成立**。
- **E. types.ts 契约完整性** → SchedulerState/ElectivesData/CourseStatus/AdminStats 与后端逐字段对齐（backend/store LogEntry 212-219 / scheduler CourseStatus 32-54 / zhidao Class/Publish/ElectivesData 542-579 / handler AdminStats 935-953 全字段名与类型核对一致）。唯一不一致：AdminStats 可选字段消费缺兜底（M-E）。**其余字段对齐无缺陷**。
- **F. useTickingCountdown 边界** → 四分支（非法日期串 / undefined/null / 过去时刻 / 未来极远时刻）：
  - **非法日期串**：`new Date("2026-09-20 09:00")` 部分浏览器返回 Invalid Date → `.getTime()` 返回 `NaN` → `diff = NaN` → `NaN <= 0` 为 false → 走正常分支 `Math.floor(NaN/1000)` = NaN → `pad(NaN)` 返回 `"NaN"`（`NaN.toString().padStart(2,"0")`）。**isExpired=false 但四位全是 "NaN"**——展示层显示 "NaN天 NaN时..."。**当前无调用点会传入非法串**（openTimeStr 来自后端 open_time 格式化为 "2006-01-02 15:04:05"、begin_times 走 `new Date(number).toISOString()`），但若未来开放时间字符串来自其他源则触发。
  - **undefined/null**：target 类型是 `string | null`，undefined 不通过类型门；`null` → diff=0 → 全 00 + isExpired=true（正确，F9-07 注释明确"null 直接视为过期"）。
  - **过去时刻**：`diff <= 0` → 全 00 + isExpired=true（正确）。
  - **未来极远时刻**：`new Date("9999-12-31").getTime()` 正常 → days 巨大数字 → `pad` 前导零正常（无溢出）。
  - **NaN 语义修正建议**：`diff <= 0` 分支应改为 `!Number.isFinite(diff) || diff <= 0`——NaN 视作过期（安全方向：绝不显示 "NaN" 而非编造）。定 OBSERVE（当前无触发，防御性加固）。

---

## 重点核对结论（任务书逐条裁决）

### 1. R53 前端三修全路径复核（8 条核验）

| 核验点 | 证据链 | 裁决 |
|---|---|---|
| ①　M-1 Select 倒计时兜底与 Dashboard N1 同构 | Select.tsx:749-754 `useTickingCountdown(openTimeStr ?? (data?.begin_times?.[0] != null ? new Date(...).toISOString() : null))` 与 Dashboard.tsx:190-195 逐字符同构；`data` 为 /electives 查询结果、`data?.begin_times?.[0] != null` 空态安全（数组空 → undefined → null 兜底）；识别有效时 `openTimeStr ??` 短路以识别真值为准；与横幅"本地已到开窗点"分支（Select.tsx:826）互不干扰（`openTimeStr=null` 时横幅走 `!openTimeStr` 分支，见 M-A）。**矩阵本体正确** | 矩阵正确 / 横幅遗留（M-A） |
| ②　M-2 Toast 去重 `typeof title === "string"` | Toast.tsx:40 `const key = typeof msg.title === "string" ? msg.title : null`——ReactNode title（pick 的"已设为首选"等全部是字符串；`title` 字段在 ToastMessage 类型是 `React.ReactNode`，但全仓 16 个调用点全部传字符串字面量）→ 全部参与去重；去重查找 `prev.findIndex((t) => t.title === key)` 按字符串值比较正确 | 正确 |
| ③　M-2 合并 description/variant/duration 取最新 + idRef 自增 | Toast.tsx:41 `idRef.current += 1` 恒自增（去重分支也自增）→ 新 id 不被去重跳过；去重分支更新 `{...prev[idx], description: msg.description, variant: msg.variant ?? prev[idx].variant, duration: msg.duration ?? prev[idx].duration}` 保留原 id → **key 稳定**（同 title 更新原 toast 不重建）；新增分支 `{...msg, id: t-${idRef.current}}` | 正确 |
| ④　M-2 三个既有 toast 调用点无回归 | Select.tsx:96/123（报名/退选成功 success）、Dashboard 无 toast 调用点（grep 全仓，Dashboard.tsx 不消费 useToast）、Login.tsx 激活/登录错误 toast 正常追加（不同 title）——全部走去重逻辑，无 title 冲突 | 正确 |
| ⑤　M-3 Admin success variant 全量消费点核对 | Admin.tsx:270（已删除）/337（激活码已生成）/541（配置已保存）三处补 success；Admin 其他 toast：105/272/339/359/546 失败 destructive、Admin 无其他 success 调用点；ToastVariant 类型 `"default" | "success" | "warning" | "destructive"` 消费点 = Toast.tsx:77-80 四个 variant className 分支 + 84-87 四个图标分支——success 分支（CheckCircle2 + emerald 边框）R53 前已存在 | 正确 |
| ⑥　R53 改动未引入保存链回归 | 见下节回归复证表 | 零回归 |
| ⑦　构建后 web 零 git 变更 | `git status --porcelain -- web/` 构建前后均零输出 | 正确 |
| ⑧　TDD 断言全绿 | target-guard-check 16 项 / admin-auth-check 6 项 / unauthorized-check 全绿 | 正确 |

### 2. F53 后防保存链回归复证（R53 改动是否引入新回归）

| 防护族 | 复证结论 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | shouldDeferSave 第二参数 hasSelected（targetGuard.ts:57-61）三消费点同步传入：防抖回调 676 行 `selectedCount > 0`、flushTargets 499 行 `latestSelectedCount > 0`、handleBack 589 行 `hasSelectedNow()`；`courses 非空 && hasSelected` 才推迟，全清空放行 PUT []。R53 未触碰 targetGuard.ts 与三消费点。**零回归** |
| **F42-M1 判据与数据源解耦** | shouldDeferSave 纯数据判据（stateData undefined \|\| courses 非空 && hasSelected）、不依赖 echoedRef；防抖 effect 依赖含 stateData（Select.tsx:737）；flushTargets 用 stateDataRef.current。R53 改动仅在倒计时区块（749-754），未触碰。**零回归** |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 消费点（回显 effect 287 + 独立清理 effect 316-327 + 防抖回调 696 + flushTargets 518）R53 未触碰。**零回归** |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19），16 项断言全绿。**零回归** |
| **F36-01 key={account} 双保险** | App.tsx:293/339 双 keyed + Select 内部 accountKey 守卫（195-202）声明于 echoedRef/rev 之后 TDZ 安全。R53 未触碰。**零回归** |
| **F48-M1 Toast 定位** | Toast.tsx:102 viewport `fixed bottom-4 right-4 z-50` 定位类在 viewport 上。R53 仅改 viewport 上方 Toast.Root 区块，未动定位。**零回归** |

---

## 上轮观察项延续复核表

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| N-2（官网 btn_type + 冲刺按钮双形态） | MINOR 观察延续 | Select.tsx:1090-1149 双轨设计意图确认 | 延续 |
| N-3（App 401 保护窗口闭包 current） | MINOR 观察延续 | App.tsx:187-235 闭包 + deps 重建确认，方向安全 | 延续 |
| O-2（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱，F6-02 锚点未动 | 延续 |
| O-3（401 保护窗口数据面） | OBSERVE | targetAccount 清代理态 + lostAccount 反查正确 | 延续 |
| O-1（Admin 五 Tab 单激活查询轮询，R53 修正） | OBSERVE 修正成立 | node_modules 产物实证 `present && children` → 非激活子组件不 mount | **确认成立** |
| O-2（useTickingCountdown 每秒重渲） | OBSERVE | 量化 bailout 可接受 + React 19 语义再核 | 延续 |
| Dashboard key 不对称 | OBSERVE | 无 key 挂载 + expandedDates 残留 | 延续 |
| ui 模板残宽 | OBSERVE | grep 复证 | 延续 |
| F51-O1 注释残留 | OBSERVE | 仅 Login.tsx:222 一处 | 延续 |

---

## 结论

- **MAJOR 1（M-A Select 横幅识别缺席 + 未来 begin_times 自相矛盾）/ MINOR 4（M-B duration 合并语义洞 / M-C viewport 无上限 / M-D Select 右栏未吃兜底 / M-E AdminStats 可选字段消费兜底缺）/ OBSERVE 7（O-1 修正确认 + O-2~O-8 延续）**。tsc exit 0、build 成功、构建后 web 零 git 变更、TDD 断言全绿。
- **最致命 3 条（按影响排序）**：
  1. **M-A Select 横幅"未识别到开放时间"在识别缺席 + 未来 begin_times 场景下恒显，与主矩阵兜底倒计时自相矛盾**（F53-M-1 修矩阵未修横幅，`cd.isExpired` 分支在 Select 侧从不触发——横幅分支顺序遗留旧版，跨页+同屏双重展示矛盾，恰是用户盯守开窗的关键窗口）。修复方向：横幅分支改与 Dashboard 一致（兜底也缺席才显"未识别"）。
  2. **M-C Toast viewport 无最大条数上限**（不同 title 高频并存时无限堆叠，黄金期 2s 轮询 + 高频点选时概率最高）。
  3. **M-E AdminStats `open_time_set === false` 消费在字段可选声明下会把 undefined 误判"已识别"**（当前后端恒下发故无实际触发，类型契约与消费假定不一致的防御缺口）。
- **已核对无缺陷的高风险区域**：R53 前端三修 8 条全路径（含 Toast 去重 key 稳定性、duration 合并语义、M-3 variant 全量消费点）；F43/F42/F40/F39/F36/F48-M1 六防保存链零回归；useTickingCountdown 四边界（null/过去/未来/非法串——唯一"NaN"展示为纯防御性 OBSERVE）；types.ts 与后端全字段对齐；Admin 五 Tab 懒渲染实证（O-1 确认）。
- **建议优先修复方向**：M-A 横幅分支对齐（一行，与 Dashboard 同构，联动修复 M-D 右栏）→ M-C viewport 上限（一行）→ M-E 类型契约/消费兜底（`open_time_set !== true`）。M-B/M-A 均摇梯最小档一行内完成。
