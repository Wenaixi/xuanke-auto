# round45 前端审查原始发现

> 审查基线：master @ `c498171`（round44 总结落盘，前端零修复、观察延续）。工作树干净（仅根目录 5 个未跟踪社区文档）。
> 校验：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`cd web && npm run build` **exit 0**（tsc -b && vite build 绿，产物 `backend/web/dist/assets/index-BvE-Yqfx.css`，已 gitignore 未污染）；`node --import jiti/register scripts/target-guard-check.ts` **16/16 断言全绿**；`npx oxlint src/` exit 0（仅既有 warning）。范围 `web/src/` 全部 `.ts/.tsx`，绝对只读。
> 后端契约对照：`backend/internal/{api/handler.go, store/store.go, scheduler/scheduler.go, zhidao/client.go}`（ClassItem/Publish/State/AdminStats JSON 字段、writeJSONStatus 家族、LoadAllLogs 上限、handleState 账号校验）。
> 上轮（round44）MINOR 3 / OBSERVE 4 核对结论：
> - **N-1（开窗动效语言不统一）**——复证仍成立（纯视觉，功能零风险），但已多轮观察，本轮无可触发的新证据，维持 MINOR 并继续延续。
> - **N-2（官网按钮 + 冲刺按钮双形态并存）**——复证仍成立（设计意图，无数据分叉），维持 MINOR 延续。
> - **N-3（App 401 管理员代理保护窗口依赖闭包 current）**——复证仍成立（保守方向），维持 MINOR 延续。
> - **O-1/O-2/O-3/O-4**——逐条复证仍在：O-1 resetRetry 退避清零（Select.tsx:648）、O-2 每秒整帧重渲（useTickingCountdown.ts:8-12）、O-3 手写模态无焦点陷阱（三处）、O-4 401 保护窗口数据面（与 N-3 互补视角）。均维持 OBSERVE。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

### N-1.【新发现｜本轮任务书视角一】`api/client.ts` 会话失效 401 在 writeJSONStatus 时代**同一响应双重广播** UNAUTHORIZED_EVENT——重复触发经核验幂等无害，属协议冗余路径

- **文件路径:行号**：`web/src/api/client.ts:64-69`（`r.status === 401` 前置广播）+ `:75-88`（`j.code === 401` 二次广播）；后端 `backend/internal/api/handler.go:1078`（`requireAuth` → `writeJSONStatus(w, http.StatusUnauthorized, 401, nil, "会话无效或已过期，请重新登录")`）
- **严重级**：MINOR（协议冗余 + 重复事件风暴放大；无状态/数据危害）
- **一句话问题**：F41-N2 为了处理网关 HTML/文本 401（JSON 解析会抛错）把失效广播提前到 `r.json()` 之前；B39-02 又把 requireAuth 从「HTTP 200 + body 401」改为「HTTP 401 + body 401」——两个机制组合后，**一次真实会话失效请求会连续触发两次事件**（先 HTTP 层、JSON 解析成功后 body 层再发一次）。
- **触发场景推演**：会话 TTL 过期 / 管理员吊销后，前端任一带 session 的请求（/state /electives /targets /admin/*）经 requireAuth 返回 `writeJSONStatus(401, 401)` → client 第一段 `r.status === 401` 广播 → App.onUnauthorized 剔除账号 → `r.json()` 成功 → `j.code === 401` 再次广播 → onUnauthorized 第二次执行，`loadSessions()` 已无该账号 → `if (!lostAccount) return` 提前退出。净效果：**事件处理幂等，第二次早退，无账号误删、无重复弹登出**。多账号并发失效时每个请求各自归属（session 优先反查），不会跨账号误杀。
- **为什么给 MINOR 而非直接关闭**：功能面已核销为无害（App.onUnauthorized 快照式三连 + `snap[lostAccount]===undefined` early return 双幂等）；但该路径是「B39-02 落定真实 HTTP 401 后首次出现的组合触发」，比旧形态（HTTP 恒 200 仅 body 广播）多了一倍事件。极端场景（批量账号吊销）下 2×事件风暴叠加，且未来若有人在 onUnauthorized 里挂非幂等副作用会立刻踩中。低成本可收敛。
- **触发归属复核**：网关 HTML 401 → 仅 HTTP 层广播（正确，F41-N2 主场景）；旧式业务 body 401 + HTTP 200 → 仅 body 层广播。只有 writeJSONStatus 双 401（requireAuth）走双重。recoverMiddleware 500 / requireAdminSession 403 / 限流 429 的 body code 均 ≠401，不触发第二段。
- **建议修法**（一行）：HTTP 层广播后 if 里立 `return`（跳过后续 body 层判断）不成立——网关 HTML 401 时 `j.code` 不可达，需保留 body 层。更稳的改法是两段共用一个 `if (r.status === 401 || j.code === 401)` 判定内先取 j（容错）再广播一次；或驱动侧 `UNAUTHORIZED_EVENT` 广播前用 `session` 去重（`lastUnauthorized=useRef`）。点到为止，功能面已无害，非本轮必需。

### N-2./N-3./N-4.【round44 N-1/N-2/N-3 延续复证】

- **文件路径:行号**：`Select.tsx:1083-1150`（双按钮并存 + 动效语言不一致处）、`App.tsx:174-176`（401 保护分支）
- **严重级**：MINOR（延续，均无功能风险）
- **复证结论**：N-1（动效语言不统一）——`disabled:opacity-40` 静态置灰与手工模态 `animate-in fade-in` 渐入并存，官方 select.js 逆向契约里禁用按钮本就是静态无动效，纯视觉一致性取舍；N-2（官网按钮与冲刺按钮同卡并存）——双轨设计意图，持久化路径（selectElective 立即报名 vs selected 目标集合）互不写冲突；N-3（401 保护窗口）——`return` 前 `lostAccount !== adminName` 已排除管理员本体，`lostAccount` 经 `loadSessions()` 反查，保守方向成立。三条均维持上一轮裁决，无升级。

---

## OBSERVE（观察项，未加重）

### O-1.【round40 N-3 / round41-44 O-1 延续】防抖 effect 入口 `resetRetry()` 清零指数退避（Select.tsx:648）

- 非用户动作（/state 数据到达、echoDone 置位、发布重建）也会清零退避，网络抖动中"重发退避长不大"。行为无错，仅与 n14「指数退避」承诺分叉。延续。

### O-2.【延续】`useTickingCountdown` 每秒 `setNow` 驱动消费页整帧重渲染（lib/useTickingCountdown.ts:8-12 + Select.tsx:749 / Dashboard.tsx:175）

- Select 数百卡片、Dashboard 日期分组每 1s 全量重建 JSX，注释「只重渲染倒计时一处」承诺分叉。Dashboard 注释已自认依赖整页重渲驱动相对时间戳。纯性能。延续。

### O-3.【延续】三处手写模态无焦点陷阱（Select 退选 / Admin 删除 / Login 激活）

- backdrop 外内容仍可 Tab 穿出；F7-03 的 role/aria/Esc、F32 的 autoFocus 取消钮均已就位，仅差完整 focus trap（Radix Dialog 迁移为 F6-02 后续候选）。延续。

### O-4.【延续】App.tsx 401 管理员代理态保护窗口

- 学生会话在被代理期间失效时残留到学生端下次真请求才剔除；端到端确认无数据风险（学生页不在场时无请求）。保守方向。延续。

### O-5.【新观察】Admin 五 Tab 全部无条件挂载，三个轮询查询在任意 Tab 停留期间后台常跑

- **文件路径:行号**：`Admin.tsx:161-205`（五 `TabsContent` 无条件渲染子组件）+ `:316-320`（codes refetchInterval 5000）+ `:702-706`（stats 5000）+ `:876-880`（logs 5000）+ `:788-792`（accounts 10000）
- **一句话问题**：Radix TabsContent 默认全部挂载（非懒加载），管理员停留在「系统配置」Tab 时，codes/stats/logs/accounts 四个查询的轮询仍按各自间隔后台刷新——每 5s 三个请求、10s 一个请求恒在跑。功能无错（react-query 缓存复用、切换 Tab 即时展示最新数据），但「配置 Tab 作为纯编辑界面」的内存/带宽冗余在公网部署下可感知。
- **建议**（若未来优化）：给 TabsContent 加 `forceMount={false}` 由 Radix 卸载非激活内容（需确认子组件查询在卸载后停轮询），或对 config Tab 场景把其他四查询的 refetchInterval 静态置 0/`false`。零功能风险，观察即可。

---

## 已核对无问题的重点区域（铁证）

1. **F43 四件套全绿**：
   - F43-M1 全清空放行：`shouldDeferSave(stateData, hasSelected)` 三消费点（防抖回调 676 / flushTargets 499 / handleBack 589+595）判据同源；TDD 16/16 全绿（含「courses 非空+全清空→放行」分叉）。rev===0 纯浏览仍双层跳过。
   - F43-N1 btn_type 判据：`btn_type===1` 退选 / `===2` 报名无条件渲染，`disabled={actionLoading.has(c.id) || !c.can_select}` 双守卫，title 回退文案与平台实证 CSR 一致；`===0`/其他值不渲染官网按钮——与官网逆向契约对齐，与后端 `zhidao.Class.BtnType`（client.go:493 int）契约一致。
   - F43-N2 票据生命周期：Login.tsx 五个清票点（成功 80-81 / 过期 88 / 其他失败 95 / Esc 233-235 / 取消 303-305）对称，后端 ConsumeTicket 失败即毁票（handler.go:199）对齐。
   - F43-N3 动效插件：`global.css:2 @plugin "tailwindcss-animate"`；本轮重建产物实测 `.animate-in{--tw-enter-*;animation-name:enter;animation-duration:.15s}` + `@keyframes enter/exit` 均在 dist CSS 中（grep 1 处 .animate-in + keyframes 命中），确认构建链路自洽。
2. **round42/round44 无继承性回归**：防抖 effect 依赖组（`[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`）、独立清理 effect 依赖 `[publishes, selected, echoedRef, toast]`、`/electives` 失败态 30s 降频 + `getQueryData` 读 window_opened 升频、`/state` refetchInterval 失败 30s、Dashboard begin_times 兜底——逐条复核正确。
3. **本轮新视角核对**：
   - **AbortController 清理**：`finally { clearTimeout(timer) }` 就位；调用方 signal 优先时兜底 timer 20s 后 abort 对已结束 fetch 无副作用（无泄漏，仅微小多余回调）。
   - **401 归属与多账号**：`detail.session` 优先反查、令牌反查落空即跳过（App.tsx:154-159），多账号并发失效互不误杀（N-1 双重广播幂等核销见上）。
   - **激活码批量生成/删除**：`generating` 与 `ReadonlySet<string>` 双在飞守卫、表格 key 用 `c.code` 唯一、删除成功 `codesQuery.refetch()` 即时刷新。
   - **Admin 五 Tab 刷新依赖**：每个查询自带 refetchInterval + 操作后 refetch/invalidate；切换 Tab 无数据残留（缓存复用）。仅 O-5 的"配置 Tab 期间后台轮询常跑"属性能冗余。
   - **搜索/筛选/排序组合边界**：`onlyAvailable + sortTightest` 同开时先 filter 后对副本 sort，互不覆盖；`matchAvailable` 判 `max_count===0 || selected_count < max_count`（F32-02：名额未公布不被当已满滤掉）；搜索无结果 → 网格内 `filteredClasses.length===0` col-span-full 空态卡，「没有符合当前搜索或筛选条件的选修课程」文案与官网未满语义一致。`activeTab` 受控 value `activeTab && tabs.some(...) ? activeTab : String(tabs[0].publish_id)` 在发布重建后回落首 Tab 不悬空；搜索只筛单 Tab 内 classes，不改变 tabs 集合 → 无联动竞态。
   - **Dashboard 日志/分组**：日志 `max-h-64 overflow-y-auto` + 后端 limit 100/200（store.go:223-226 / 414-416），文案 RECENT 100 / 最近 N 条对齐；`expandedDates` null/[] 语义分离种子只种一次，用户折叠不复活；`dateGroups` useMemo 依赖 `[courses, electives?.publishes]` 引用稳定。
   - **StrictMode**：`setInterval`/事件监听全部走 `[]` effect + cleanup；unmountedRef 双向复位（F20-01）；生产构建无双调用。
   - **类型与后端字段漂移**：ClassItem/Publish/ElectivesData/SchedulerState/AdminStats 逐一对照 Go struct json tag（client.go:481-515 / scheduler.go:45-59 / handler.go:706-713）——全字段名类型匹配，custom 键（stats 的 `open_time_set`/`token_valid`）与 handler.go:928-946 map 构造一致；`open_time` 学生端 RFC3339 time.Time / 管理端 `Format("2006-01-02 15:04:05")` 空格字符串，前端分别按 `new Date()` 解析与纯文本展示，均兼容。
4. **手动报名/退选在飞幂等**：`ReadonlySet<number>` 按课程独立跟踪、finally 函数式删除、成功失败都 invalidate 双查询；后端 TryAcquireSubmit 单课锁二道防线。正确。
5. **可访问性基础设施**：role=dialog/aria-modal/aria-labelledby/Esc（进行中防误关）、autoFocus 取消钮、CollapseSection useId+aria-expanded/controls、role=switch/aria-pressed、label htmlFor——就位。

---

## 结论

- **MAJOR 0 / MINOR 4（1 新 + 3 延续）/ OBSERVE 5（4 延续 + 1 新）**，共 9 条。
- 最重 3 条（按功能影响排序，均低风险）：
  1. **N-1（双重广播）**——本轮唯一新发现；B39-02 writeJSONStatus 与 F41-N2 前置广播组合后同一 401 响应触发两次 UNAUTHORIZED_EVENT。经 App.onUnauthorized 幂等（快照三连 + early return）核销为功能无害，但属新契约引入的冗余路径，多账号批量吊销时事件风暴翻倍。一行可收敛。
  2. **N-3（401 管理员保护窗口）**——保守方向确认，端到端无数据风险，延续观察。
  3. **N-2/N-1（双按钮 + 动效语言）**——设计意图/纯视觉，延续。
- F43 四件套全部复核绿，TDD 16/16，tsc/build 全绿；round42-44 无继承性回归。本轮不存在功能级错误，最值得动手的是 N-1 的一行收敛（协议清洁）+ O-5 的懒加载（性能），均为零风险 polish。