# round43 前端审查原始发现

> 审查基线：master（round42 fix-report 落地后，`3622a8f`）
> 范围：`web/src/` 全部 `.ts/.tsx`（App.tsx / api/client.ts / routes/{Select,Admin,Dashboard,Login}.tsx / lib/{targetGuard,useTickingCountdown,utils}.ts / types.ts / main.tsx / components/ui/*），绝对只读。
> 校验：`cd web && npx tsc -p tsconfig.app.json --noEmit` exit 0（根 tsconfig 是 references 空壳，`tsconfig.app.json` 才真校验 src）。
> 上轮（round42）三项修复核对结论：
> - F42-M1（shouldDeferSave 纯数据判据 + 防抖 effect 依赖补 stateData）——**已正确落地**。守卫三处（防抖回调 / flushTargets / handleBack 5s 等待）判据同源、均读 `stateDataRef.current` 消费时刻快照；`stateData` 进依赖数组后 /state 数据到达触发 effect 重跑挂新 timer，round42 M-1 描述的"守卫命中置脏后 selected 无变化被 React bailout 导致订阅永不重入"的死锁路径已闭合（补 `echoDone` 依赖亦在：courses 空场景单靠 selected 不重跑，回显置位驱动一次）。
> - F42-M3（/electives 轮询失败态降频 30s + 失败态不吞 window_opened 升频）——**已正确落地**。`query.state.error || status==="error"` 首行短路降频；`window_opened` 升频信号从 `/state` 缓存 `getQueryData` 读取，不再依赖闭包内 stateData 或 TDZ。
> - F42-N1（独立清理 effect 依赖补 selected）——**已正确落地**，依赖数组 `[publishes, selected, echoedRef, toast]` 与 cleanStaleSelected 幂等返回原引用共同保证不引出多余重渲染。
> 上轮观察项核对结论（沿用任务书注记，可简短带过）：N-2（handleBack 全清空误判）**确认仍成立且未被 F42 覆盖，本轮升级为 MAJOR 复证**（见 M-1）；N-3（报名按钮窗口门控）确认仍成立，MINOR 复证；N-4（激活失败票据残留）确认仍成立，MINOR 复证；O-1/O-2（resetRetry、每秒整页重渲）确认仍在，OBSERVE 复证。

---

## MAJOR（明确错误行为 / 合法用户操作被静默撤销）

### M-1. [round42 N-2 复证升级] 用户"清空全部目标"在 `/state` courses 非空期间**永不落库**——shouldDeferSave 纯数据判据把"清空意图"误判为"回显未完成"，返回后旧目标复活

- **文件路径:行号**：`web/src/routes/Select.tsx:671-674`（防抖回调 `if (shouldDeferSave(stateDataRef.current)) { dirtyRef=true; return }`）+ `:499-502`（flushTargets 同款）+ `:584-595`（handleBack 5s 等待同判据）+ `lib/targetGuard.ts:53-57`（shouldDeferSave 定义）
- **严重级**：MAJOR
- **一句话问题**：`shouldDeferSave` = `stateData===undefined || courses 非空`，只表达"后端仍有旧目标需先回显合并"，但**全清空（selectedCount===0, rev>0）时根本无需合并——PUT [] 正是用户意图**。当前判据对"全清空"与"数据缺席/慢首帧"一视同仁，防抖与 flush 双闸全被命中置脏，清空永不 PUT，且无自愈信号可解锁（courses 恒非空时 echoDone/stateData 依赖只能让 effect 重跑、重跑后同判据继续打回）。
- **触发场景推演**：
  1. 学生首次进入选课大厅。`/electives` 快照秒回（发布+课程立即可点），`/state` 首帧因后端重建较慢 3~5s 后才到达。
  2. 学生先点选 A、B 两门目标（rev=2），犹豫后**全部取消**（rev=4，`selected={}`）。此窗口内 `/state` 首帧到达，返回 `courses=[A,B]`（旧目标仍在后端——防抖 400ms 期间 `/state` 未到，`shouldDeferSave` true，从未 PUT 成功过一次）。
  3. 学生点「返回控制台」：`handleBack` 满足 `revRef>0 && shouldDeferSave` → 进入 5s 等待；等待期间 `shouldDeferSave` 恒 true（courses 非空且轮询每次都带着 [A,B]）→ 超时。回显 effect 在 `rev>0 && !anyHas` 分支提前 return prev（不合并），置 echoDone=true——但 `shouldDeferSave` 不读 echoDone，防抖 effect 重跑后回调仍被 `courses 非空` 打回置脏。
  4. flush 三轮全被 `shouldDeferSave` 打回置脏 → `onDone()` 卸载。**后端 targets 仍为 [A,B]**。
  5. 学生再次进入选课大厅：全新挂载 → 回显 effect 合并 `courses=[A,B]` 回 selected → **清空被静默撤销，旧目标复活**。
- **与 F42-M1 的关系**：F42-M1 修的是"首帧未到/慢首帧时新改动永不落库被 deadlock"，判据升级为纯数据后**恰好同步放大了全清空场景的覆盖缺口**——round42 N-2 建议修法（flush 发布缺席守卫补 `!!echoed`）未落地，F42 也未给"清空意图"开逃生门。core 语义：`shouldDeferSave` 担忧的"只含用户新改动覆盖后端旧目标"只有当 selected **非空**时才危险；selected 全空时 PUT [] 表达的是显式清空。
- **建议修法**（一行）：`flushTargets` 与防抖回调在 `shouldDeferSave` 守卫**之前**加 `if (latestSelectedCount === 0) { /* 落库前不拦截——清空意图放行 */ }`，或把判据改为 `shouldDeferSave(stateData) && selectedCount > 0`（share 到 targetGuard 纯函数 + TDD 断言补"全空放行"分叉）。

---

## MINOR（展示 / 边界健壮性）

### N-1. [round42 N-3 复证] 手动「报名/退选」按钮被 `in_date_range || window_opened` 双重窗口门控隐藏——平台已下发 `btn_type=1/2` 时窗口信号缺失即无操作按钮，手动抢课接口被锁死

- **文件路径:行号**：`web/src/routes/Select.tsx:1080`（`{t.in_date_range || stateData?.window_opened ? (… btn_type 1/2 …) : (标准预选按钮)}`）
- **严重级**：MINOR（官网契约对齐分歧，非数据错误）、round42 报告后未修
- **一句话问题**：官网真实 select.js 渲染「报名/退选」按钮以平台 `btn_type`（1/2）为准，与窗口开关无关；本项目把整个"操作按钮区"绑在 `in_date_range || window_opened` 上。平台下发 `btn_type=2` 但窗口信号未达（调度器识别槽未建立 / 学校网络下 `/state` 慢 / `in_date_range` 刹那为 false）时，学生看不到「报名」按钮，无法手动抢课。
- **触发场景推演**：开窗瞬间平台 publishes 短暂清空又恢复（F18-03 已处理的形态）：清空期间 `data.publishes=[]` → 无 btn 渲染（空态卡）；恢复后 `publishes` 里 `in_date_range` 若恰在翻转窗口内为 false 而 `/state` 的 `window_opened` 又未识别 → 完整课程列表重新渲染成「设为预选目标」按钮群，**无报名按钮**；学生只能设预选目标等后台冲刺，手动报名通道隐形。
- **建议修法**（一行）：按钮区渲染条件改为以 `c.btn_type` 为唯一判据（`btn_type===1||btn_type===2` 即渲染），`disabled={!c.can_select}` 已双守卫；`in_date_range||window_opened` 只决定展示文案（预选 vs 手动）。

### N-2. [round42 N-4 复证] `Login.tsx` 激活失败时 `pendingTicket` 不清空——失败后旧票据滞留，二次激活/重登 1001 可能携带已消费票据造成「票据已用」死循环

- **文件路径:行号**：`web/src/routes/Login.tsx:91-93`（catch 非「过期/已用」分支只 `setActivateError`，不清票）
- **严重级**：MINOR（票据 5 分钟单次，低概率可触达）、round42 报告后未修
- **一句话问题**：激活失败（网络抖动/激活码错）分支只弹错误、`pendingTicket` 保留旧值；用户「失败 → 重新登录」时新 1001 响应 `setPendingTicket((e.data?.ticket)||"")` 会覆盖新票——正确；但「失败 → 用户点激活（用旧票）→ 服务端已 Consume 半次」时服务端报「激活票据无效或已过期」，引导文案（"已开通，重新登录即可"）也许与实际票态不符。
- **建议修法**（一行）：catch 非「过期」分支末尾补 `setPendingTicket("")`，强制下一轮 1001 用新票据。

### N-3.【新发现】Tailwind 4 项目未启用 tw-animate 插件，全部 `animate-in/zoom-in/slide-in/fade-in` 动效类在构建产物中**不存在**——模态/Toast/Dialog 声明的进场与退场动画静默失效

- **文件路径:行号**：`web/src/components/ui/Dialog.tsx:18,36`、`Sheet.tsx:18,41-44`、`Toast.tsx:58`、`Select.tsx:1165 / Admin.tsx:212 / Login.tsx:221`（`animate-in fade-in duration-150`）、`web/src/styles/global.css:1`（仅 `@import "tailwindcss"`）、`web/package.json`（无 tailwindcss-animate / tw-animate-css 依赖）
- **严重级**：MINOR（纯视觉，行为无错）
- **一句话问题（验证证据）**：`animate-in`/`zoom-in-95`/`slide-in-from-bottom-full`/`fade-in-0` 均为 tailwindcss-animate 插件工具类，Tailwind 4 原生不含；`grep -c "animate-in" backend/web/dist/assets/index-*.css` = **0**——构建产物里根本没有这些类，镜像了版本之后所有声明动效的 UI 都以无动画方式出现/消失。
- **触发场景推演**：任一路径打开退选确认模态，期望的 `animate-in fade-in duration-150` 渐入、Toast 的 `slide-in-from-bottom-full`、Dialog 的 `zoom-in-95` 均不出现（立即呈现/立即消失）；对功能零影响，但设计系统"极简几何动效"承诺未兑现，且一旦未来依赖它做视觉时序（如 backdrop 渐隐）会平台差异不可控。
- **建议修法**（一行）：global.css 加 `@plugin "tailwindcss-animate";`（或安装 `tw-animate-css` 后 `@import "tw-animate-css";`），构建后复验 dist CSS 含 animate-in。

---

## OBSERVE（观察项，未加重）

### O-1. [round40 N-3 / round41 O-1 / round42 O-1 延续] 防抖 effect 入口 `resetRetry()` 仍会把「回显合并 / 发布清理 / echoDone 置位」等**非用户动作**的 selected 变化清零指数退避

- **文件路径:行号**：`web/src/routes/Select.tsx:643`
- **一句话问题**：F42-M1 新增 `stateData` 依赖后，effect 还随 `/state` 内容真变（window_opened 翻转、courses 首次到达、识别槽建立等）重跑并 `resetRetry()`——网络抖动中"重发退避长不大"的语义分叉延续三轮回。行为无错（保证重发不无限推迟），仅与 n14「指数退避」承诺分叉。建议仅在用户动作（pick/清空）调用 resetRetry。

### O-2. [round40 O-1 / round41 O-2 / round42 O-2 延续] `useTickingCountdown` 每秒 `setNow` 驱动消费页整帧重渲染

- **文件路径:行号**：`web/src/lib/useTickingCountdown.ts:8-12` + Select.tsx:744 / Dashboard.tsx:175
- **一句话问题**：Select 数百课程卡片、Dashboard 日期分组每 1s 全量重建 JSX，与注释「只重渲染倒计时一处」承诺分叉；纯性能，行为无错。建议抽独立 `<Ticker>` 子组件 + React.memo。

### O-3. [延续] 三处手写模态（Select 退选 / Admin 删除 / Login 激活）无焦点陷阱——背景可 Tab 穿出，读屏在 aria-modal 下可退到背景

- **文件路径:行号**：`Select.tsx:1163-1212 / Admin.tsx:210-284 / Login.tsx:219-312`
- **一句话问题**：F7-03 补了 role/aria/Esc、F20/F21 补了进行中防误关，但 backdrop 外内容仍可聚焦（Tab 循环仅靠浏览器默认），F6-02 注释已承认"焦点陷阱迁移 Radix Dialog 属后续候选"。无新证据，OBSERVE。

### O-4.【新观察】App.tsx 401 处理器「管理员代理态 return」分支依赖闭包 `current`，与前置代理复位分支语义叠加后存在保守保护窗口

- **文件路径:行号**：`web/src/App.tsx:174-176`
- **一句话问题**：`if (current === adminName && inAdmin && lostAccount !== adminName) return` 在**管理员于管理页正常操作**时，收到**学生账号**的 401 事件会整体跳过剔除（含落盘与 setState）——此时学生页面并不在场（无轮询请求），该事件大概率来自陈旧响应；跳过可避免误杀事件竞态，但也意味着学生会话若在被代理期间失效，会一直残留到学生端下次真正请求时才被剔除。保守方向、注释有意图声明，本轮无触发实证，记录共证。

---

## 已核对无问题的重点区域

- **F42-M1 全链**：`shouldDeferSave` 三处消费点（防抖回调 / flushTargets / handleBack 等待）判据同源且均读 `stateDataRef.current` 消费时刻快照；防抖 effect 依赖补 `stateData` 后死锁路径闭合；`echoDone` 依赖覆盖 courses 空场景自愈；TDD 断言脚本 `web/scripts/target-guard-check.ts` 四判据与 targetGuard.ts 实现一致。
- **F42-M3**：`/electives` refetchInterval 失败态 30s 降频 + `window_opened` 从 `queryClient.getQueryData` 取（TDZ 注释正确，回调同步执行期不直读组件 const）。
- **自动保存链其余**：F7-01 仅 rev 驱动、F13-C1 rev===0 跳过、F13-C2/F20-01 unmountedRef 双向、F15/F16/F17 假清空守卫、`lastJson` 去重只在成功后更新、saveNow finally 补发链、handleBack 三轮 flush + pendingSaving(timer) 有界等待——全部正确。
- **回显合并**：`echoedRef` 一次性 + `rev>0&&!anyHas` 全清空不合并 + 空 courses 置 echoDone + `key={account}` 挂载复位（App 两处挂载点均带 key）——正确。*唯一缺口即 M-1（清空永不落库），非合并方向。*
- **手动报名/退选**：`ReadonlySet<number>` 在飞幂等、unmountedRef 后 toast 守卫、成功失败都 invalidate 双查询——正确。
- **client.ts**：F41-N2 非 JSON 401 前置广播 + `extractAccountFromPath` 纯函数 + F7-09 abort 文案 + 20s 超时——正确。
- **App.tsx**：401 归属判定（session 优先、账号名直查再令牌反查）、快照式三连落盘、M28-01 onDeleted、M29-02 page 复位、onBackToStudent 完整登出、[F25-01/F15-07] 代理复位——正确（O-4 仅为保守方向观察）。
- **Dashboard**：本地零点日期分组（parseDateKey/localTodayMs）、isFullFallback、「未知」兜底、begin_times 兜底与文案同源、logs 闭包降频——正确。
- **可访问性基础**：role=dialog/aria-modal/aria-labelledby/Esc、进行中防误关、autoFocus 取消钮、CollapseSection useId+aria-expanded/controls、role=switch、aria-pressed——就位（focus trap 见 O-3）。
- **类型**：`npx tsc -p tsconfig.app.json --noEmit` exit 0；TS2448/TS2454 TDZ 历史点（refetchInterval 不直读 stateData、独立清理 effect 声明于 publishes 之后、ka=account 守卫声明于 echoRef 之后）——当前全合规。

---

## 结论

- **MAJOR 1 / MINOR 3 / OBSERVE 4**，共 8 条。
- M-1 为唯一立得住的高危项：round42 N-2「清空永不落库」在 F42 纯数据判据下确认存活且无自愈信号，直接导致"用户清空目标 → 返回 → 再进入被旧目标复活"的静默撤销；修法一行（`selectedCount===0` 时跳过 shouldDeferSave）。
- N-3 为自定义双验的新发现（产物验尸 grep -c=0），低成本可修。
- round42 的 F42 三件套（M1/M3/N1）全部核对属实，未发现继承性回归。