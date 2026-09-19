# round40 前端审查原始发现

> 审查基线：master @ fa6c003（round39 总结落盘后）
> 范围：`web/src/` 全部 `.ts/.tsx`（App.tsx / api/client.ts / routes/Select.tsx / Login.tsx / Dashboard.tsx / Admin.tsx / lib/targetGuard.ts / lib/useTickingCountdown.ts / lib/utils.ts / types.ts / main.tsx / components/ui/*），绝对只读。
> 校验：`cd web && npx tsc --noEmit` exit 0；前端契约对照项目根 CLAUDE.md《工程决策手册》（26 条决策锚）+ backend 侧 handler.go / scheduler.go 核验（手动报名/退选后端无 isOk 字段双判、window_closed 与 /state 同源、账号穿透凭据表校验等）。

本轮为第 40 轮。round39 已修并落决策锚的项：C-1（selectedHasStalePublish 双闸）、M-1（删管理员退管理态）、N-1（Dashboard 主倒计时 begin_times 兜底）、N-3（StatsTab 窗口三态）、N-4（文案）、N-5（accounts useMemo）。其中 N-1 存在**成半修复**提出新问题（见 M-2），C-1 引入**静默死锁**新问题（见 M-1）。round39 的 M-2 未在修复清单内，重证仍成立（见 M-3）。

---

## MAJOR（明确错误行为）

### M-1. round39 C-1 守卫命中路径全程静默 + 旧发布残留 key 不可达 = 保存链静默死锁（独有的"看起来保存成功实则永存不上"）

- **文件路径:行号**：`web/src/routes/Select.tsx:448-451`（flushTargets 守卫）、`web/src/routes/Select.tsx:612-615`（防抖守卫）、对照 `pick()` 298-315（先弹"已设为首选"成功 toast）
- **严重级**：MAJOR
- **一句话问题**：`selectedHasStalePublish` 命中即 `dirtyRef.current = true; return`——**无任何 toast/UI 提示**；且触发它的旧 publish_id 键在发布重建后对应的 Tab 已消失、用户**无法通过界面清除**，守卫将永久拦截后续所有保存（每次防抖/flush 都静默跳过），唯一恢复途径是整页刷新（重载后回显 F19-01 过滤掉幽灵课程、selected 重建为空）。
- **触发场景推演**（自洽时序）：
  1. 窗口开放前，用户在 Select 内于旧发布 P1 下勾选了课程 A：`selected = {P1:[A]}`，防抖 PUT 已成功落后端。
  2. 开窗瞬间平台发布集合整体重建（F18-03 实证"publish_id 全变"，round39 C-1 场景同款）→ `/electives` 轮询拉到新集合 [P9,...] → `publishesRef` 更新；但 `selected` 里的 `P1:[A]` 不随任何 effect 清理（回显 effect 只过滤 courses 合并路径、不清已触碰 key）。
  3. 用户在黄金期点选新课 X@P9 → `pick()` 先弹 **"已设为首选" 成功 toast**（UI 上 X 高亮）→ `rev++` → 400ms 防抖回调在消费时刻命中 `selectedHasStalePublish({P1:[A], P9:[X]}, [P9...])` → 置脏**无声返回**。
  4. 之后用户每次改动都重复"UI 高亮 + toast + 永不落库"，Dashboard 上永远看不到 X，调度器按旧目标行动；P1 的残留条目在界面上**无任何入口可清除**（旧 Tab 已消失），`hasPublishes` 触发重跑也无法自愈（守卫判据恒命中）。
  5. 结果：C-1 守卫成功防住了"整包覆盖删除"（数据不丢），但把用户锁死在一个**滚不出的静默失败态**——恰在开窗黄金期（最需要改目标的时刻）失效。
- **与已落决策锚的关系**：round39 C-1 的"置脏跳过"是刻意安全方向（保数据 > 可保存），但其**静默无反馈 + 无解锁路径**属性是本轮新指出的缺口——守卫只防了一头，没有为用户提供另一头的出路。
- **建议修法**（一行）：两个守卫命中处各加一句 `toast({title:"发布已更新", description:"旧批次目标已失效，请刷新页面继续操作", variant:"warning"})`；或更彻底：在发布集合重建时把 `selected` 中"非空且不在当前发布集合"的 key 随重建清理（连同一次触发重存），用户无需手动刷新。

### M-2. Dashboard 主倒计时 begin_times 兜底只改了一半——矩阵在倒计具体开窗时刻，同屏"预计开放时间"仍显示"未识别到开放时间"

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:158-163`（`cd = useTickingCountdown(openTimeStr ?? begin_times[0]...)`）与 `Dashboard.tsx:367-373`（文案行 `openTimeStr ? ... : state ? "未识别到开放时间" : ...`）
- **严重级**：MAJOR（信息自相矛盾的展示缺陷，紧接 open_time_known=false 的启动/探测期必现）
- **一句话问题**：round39 N-1 修复只把倒计时输入换成 begin_times 兜底，文案行未同步——`open_time_known=false` 但 `begin_times[0]` 非空时，四格大矩阵明确倒数"距开放还有 X 天 Y 时"，同一屏文字却宣告"未识别到开放时间"，两行展示两套事实，学生无法判断该信哪个（这正是 N-1 描述的同款矛盾，只是换了个出现位置）。
- **触发场景推演**：学生进入 Dashboard 后 `/state` 首帧返回 `open_time_known=false`（调度器识别槽尚未建立），而 `/electives` 已下发 `begin_times=[未来开窗点]` → 矩阵按 begin_times 正常倒数，文案行显示"未识别到开放时间"。识别槽建立后 `openTimeStr` 出现、文案切回正常（矛盾窗口 = 首帧 ~ 识别完成之间，通常数十秒）。
- **建议修法**（一行）：文案行同样吃兜底——`openTimeStr ?? (begin_times[0] ? 格式化(begin_times[0]) : state ? "未识别到开放时间" : ...)`，与矩阵同源。

### M-3. [重证] round39 MAJOR M-2 未被修复：`/state` 查询失败/无数据时 refetchInterval 恒 2s 高频重试

- **文件路径:行号**：`web/src/routes/Select.tsx:142`（`refetchInterval: (query) => (query.state.data?.window_closed ? 30000 : 2000)`）
- **严重级**：MAJOR（round39 已报，修复清单中无 M-2 条目，此处重证仍成立）
- **一句话问题**：react-query 失败时 data 为最后一次成功值或 undefined；窗口未关闭（data 非关态）或从未成功（data undefined）时该 interval 全取 **2000ms**，网络挂断/后端重启期间 `/state` + `/electives` 双查询叠加固定 2s 轰炸（与项目"窗口关闭降频/失败分级退避"的防轰炸理念相悖，公网部署下日志/代理层被刷屏）。
- **建议修法**（一行）：`(query) => (query.state.error || query.state.status === "error" ? 30000 : query.state.data?.window_closed ? 30000 : 2000)`。
- **说明**：round39-frontend-fix-report 审计确认其修复清单只有 F39-C1/M1/N1/N3/N4/N5，M-2 不在其中。

---

## MINOR（边界瑕疵）

### N-1. Dashboard 日期分组"今天"基准用 UTC 日期，分组键却按本地日期——UTC+8 凌晨 00:00–07:59 之间"距今天最近"排序整体错一档

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:207`（`const todayMs = parseDateKey(new Date().toISOString().slice(0, 10))`）
- **严重级**：MINOR
- **一句话问题**：`parseDateKey` 注释明确修了"裸 Date 串按 UTC 解析"的坑、用 `k + "T00:00:00"` 锁本地零点（第 105-107 行），但"今天"基准仍用 `toISOString()`（恒 UTC）。UTC+8 时区每日 00:00–07:59（UTC 前一日的 16:00–23:59）`toISOString().slice(0,10)` 是**昨天**的日期 → `todayMs` = 昨天的本地零点 → 昨天上午的课程组距离=0 被排在最前，今天的课程组反被排到后面；跨日课程排序全错一档。
- **触发场景推演**：2026-09-20 02:00（UTC+8）打开 Dashboard，本地今天是 9-20，`toISOString()` = "2026-09-19" → todayMs = 9-19 本地零点。9-19 开课的发布组距离 0（排在首位），9-20 开课的发布组距离 1 天（排第二）——"距离今天最近在前"的契约对当天凌晨查看的用户完全颠倒。
- **建议修法**（一行）：`const todayMs = new Date(); todayMs.setHours(0,0,0,0); todayMs.getTime()` 或 `parseDateKey(new Date().toLocaleDateString("en-CA"))`。

### N-2. CollapseSection 的 `aria-controls="collapse-body"` 悬空——目标元素不存在，且补上也必与多实例冲突

- **文件路径:行号**：`web/src/routes/Dashboard.tsx:64-77`（`aria-controls="collapse-body"` 见 66 行；正文 `<div className="px-3 pb-3 ...">` 无任何 id）
- **严重级**：MINOR
- **一句话问题**：注释声称"button 带 aria-expanded/aria-controls，读屏可感知展开状态"，但 `aria-controls` 指向的 `id="collapse-body"` 在本组件内从未落位——该属性 is dangling（对读屏无效）；且 CollapseSection 在 Dashboard 被多处同时实例化（extras 折叠段 + 每个日期组），即使补上 `id` 也会产生全页重复 id 的 DOM 冲突。
- **触发场景推演**：NVDA/读屏用户聚焦任一折叠段 `<button>`，`aria-controls` 引用不存在的 region-id，展开/折叠后读屏无法跟随内容区（本应"读屏可感知"的承诺落空）。
- **建议修法**（一行）：把 `id` 提升为 prop 并在调用处传唯一值，或组件内 `const id = useId()` 后 `aria-controls={id}` + 正文 `id={id}`。

### N-3. [重证] round39 MINOR N-2 未被修复：防抖 effect 无条件 `resetRetry()` 仍会清零指数退避

- **文件路径:行号**：`web/src/routes/Select.tsx:566`（effect 体开头 `resetRetry()`，依赖含 `[rev, selected, sessionToken, toast, hasPublishes, echoDone]`）
- **严重级**：MINOR（已观察，仍成立）
- **一句话问题**：回显合并（setSelected）、echoDone 置位、hasPublishes 翻转都会重跑 effect → 每次重跑都清掉排队中的退避 timer 并归零 attempt，网络抖动期间"指数退避永远长不大"（重发提前而非丢失）。round39 观察结论维持不变，属已知语义分叉（与 n14"指数退避"承诺相悖），危害有限。

---

## OBSERVE（存疑待核 / 低概率边界）

### O-1. useTickingCountdown 每秒触发 Select 整页重渲染（注释"只重渲染 hook 消费处"与 React 机制不符）

- **文件路径:行号**：`web/src/lib/useTickingCountdown.ts:9-12`（`setInterval(() => setNow(Date.now()), 1000)`）+ `web/src/routes/Select.tsx:655`（顶层消费）
- **严重级**：OBSERVE
- **一句话问题**：hook 顶层 setNow 每秒使**整个 Select 组件树**重渲染（React 没有"只重渲染一个 hook 的消费处"的能力——useState 在组件作用域），含全部 Tabs + 82+ 课程卡片网格；在开窗黄金期（页面负载峰值、250ms 冲刺语境）每秒全量重渲染大网格属于可感知的 CPU 浪费。Dashboard 同理但树更小。非正确性缺陷，属结构与注释承诺分叉。
- **建议修法**：把倒计时块抽成独立的 `<Countdown target=... />` 子组件（tick 只重建它），或 Select 里大网格套 `React.memo`。

### O-2. round39 O-1/O-2/O-3 项维持不变（已观察，仅重证）

- `web/src/App.tsx:52-96` 多 Tab 共享 localStorage、任一 Tab 登出即删全体会话（O-1）。
- `web/src/api/client.ts:69-75` 调用方 signal abort 与超时共用"请求超时，请重试"文案（O-2，当前无调用方 signal 场景，实际不可触发）。
- `web/src/routes/Select.tsx:182-188` 渲染期写 `echoedRef.current`（O-3，StrictMode 下双执行幂等，已有 key={account} + F36-01 双保险）。

---

## 已核对无问题的重点区域（本轮逐条验证）

- **目标自动保存链主体**：防抖仅由 rev 驱动、`lastJson` 去重、`savingRef/dirtyRef` 串行补发、`unmountedRef` 卸载停手（F13-C2/F20-01）与挂载复位、`handleBack` 三轮 flush + 21s 有界等待 + 假清空脏块刻意不等——逐条核对无新缺口。
- **round39 C-1 修复落点**：`selectedHasStalePublish` 在 flushTargets（448）与防抖回调（612）两处消费时刻**构建之前**调用，空数组键（用户清空）不判过期——判据与 TDD 断言脚本 `web/scripts/target-guard-check.ts` 语义一致。
- **回显合并族**：echoedRef 只合并一次、全清空绝不复活、首帧未到绝不置位、幽灵 publish_id 过滤（F19-01）、状态镜像 ref 消费时刻读取——全链路无缺口（唯一新缺口是 M-1 的守卫静默锁定）。
- **401 链**：client.ts 事件带 session 真实主体、App 按令牌反查、管理员代理态与被吊销管理员自身两分支均退出代理+管理态（F25-01 对称）、快照式三连落盘。round39 M-1 修补后 onDeleted 与 onUnauthorized 完全对称。
- **窗口状态横幅**：Select 顶部 `window_closed 优先 → window_opened → 未识别 → isExpired → 倒计时` 分支顺序正确（F15-03）。
- **手动报名/退选在飞幂等**：`ReadonlySet<number>` 按课程独立跟踪 + 入口短路；后端 `TryAcquireSubmit` 单课锁二次拦截（handler.go:315-320）；后端退出/报名侧 `Code!=0 || !IsOk` 双判契约与前端 `res.msg` 透传一致。
- **激活链**：票据随 1001 透传、取消清票据、弹窗 Esc + 激活中不响应、幂等短路一致。
- **倒计时 hook**：target 变化即时校正 now、过期全 00、interval 只在 `[]` effect 挂载（StrictMode 双挂载安全）。
- **localStorage 读写容错**：App.tsx 全部四函数 try/catch 降级内存态。
- **无 XSS 注入点**：全仓库零 `dangerouslySetInnerHTML`/`eval`/`innerHTML`/`document.write`，课程名/教师名均经 React 文本节点渲染。
- **类型正确性**：`npx tsc --noEmit`（web 目录）exit 0；Select.tsx 的 refetchInterval TDZ 规避（读 react-query 缓存而非组件 stateData）正确；Admin stats 窗口三态读 `window_closed?: boolean` 容错正确。

---

## 结论

- **MAJOR 3 / MINOR 3 / OBSERVE 2**，共 8 条。
- M-1 是最高优先级新发现：round39 C-1 守卫（防数据丢失）引入了**无反馈 + 无解锁路径**的保存链静默死锁，恰在开窗黄金期触发概率最高；一行 toast / 或随重建清理 stale key 即可闭环。
- M-2 是 round39 N-1"修复只改一半"的残留矛盾（矩阵倒数 vs 文案"未识别"）；M-3 与 N-3 是 round39 明确未修项的复证。
- N-1（UTC 今天基准）与 N-2（悬空 aria-controls）为全新小问题，均一行可修。