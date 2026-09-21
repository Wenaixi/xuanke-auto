# round65 前端只读审查发现报告

> 审查基线：master @ `527152d`（R64 收官）。开局 `git status --short --branch` = `## master` 干净；收尾复检仍干净。web/src 连续十一轮零提交（R55→R64，仅 R63 M-1 修复 `7e4ef3a`，本轮复查 `git diff 1861093..527152d --stat -- web/src/` 为空确认 R65 基线无 web/src 改动）。本轮按任务书新视角 A-E 五条主线执行；审查期间绝对只读——唯一新建文件为本报告，`%TEMP%` 未落任何临时脚本。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 12（延续集）**

**R64 M-1 修复（R63 合入的 echoed 维度）判定：仍然闭合，且本轮从端到端稳态编辑真实落库路径拿到新的后端侧契约证据。** 核心延续观察两项：① O-11 M-1 修复边角语义（两条收敛窗口变宽/假阳性命中路径）经 R63→R65 三轮核对维持，本轮新补一条微观假阳性命中路径（新增改备选后 2s 轮询 `/state` 已返回重建后的自身 courses、`rev>0` 回显 effect 短路不修正）——所有命中均为保守方向（PUT 整包与后端一致，无覆盖删除）；② 新稳态「echoed=true 放行」带来的「已回显账号任意时刻新增/新增的防抖保存全部走通」——本轮走通了「已回显账号新增 → 防抖 PUT → 后端 SetTargetsForAccount 持锁 + rebuildCoursesForAccountLocked 重建」完整链路。**无数据丢失/覆盖路径，无新缺陷**。

**最致命 3 条（按影响排序，均为 OBSERVE 级，无功能缺陷）**：
1. **O-11（延续）**：M-1 修复的两条边角语义偏移。① 首帧未到 + 有选中 → 多轮 50ms 轮询收敛窗口在原「5s 兜底」下实际最坏 20s（api 超时上限）×N 轮；② 新稳态假阳性命中路径在本轮新补一条「新增改备选后 2s 轮询 + rev>0 回显短路」——全部保守方向，无覆盖、无静默撤销。**维持**。
2. **N-1（延续第 8 轮）**：「后台冲刺」文案 vs 后端仅 10 秒黄金期 250ms 冲刺（`submitIntervalFor`）——纯文案，若治 = 两行统一「后台目标」删“冲刺”。**维持**。
3. **N-3（延续第 6 轮）**：三处格式卫生——`client.ts:86-94` 的 body-401 分支内嵌 `let account` + 重复正则再匹配（与顶部 `extractAccountFromPath` 同文案、未复用之，导出层 `:94` 未加 `else` 缩进），`Select.tsx:379` 的 `const lastJson` 前导 4 空格缩进。**本轮字形级复证逐字节一致，维持**。

---

## 新视角 A 逐项实证裁决

### A-① 上轮观察项延续复核

| 编号 | R64 状态 | R65 复核 | 裁决 |
|---|---|---|---|
| **M-1** | 已闭合 | 修复本体（targetGuard.ts:64-71 + 三消费点）+ 回显 effect 置位点（238/295/200）语义、时序、断言覆盖全部维持；**新增端到端证据**：稳态编辑真实落库到 /state 回显自身改动的后端链路逐段核对（handleTargets → SetTargetsForAccount 持锁 → rebuildCoursesForAccountLocked，scheduler.go:459-495/575-609）确认无覆盖、无残留 | **已修，闭合（第 2 轮确认）** |
| **N-1** | 延续第 8 轮 | 落位点字面复核（Select.tsx:1144/1162 两处「后台冲刺/后台冲刺目标」+ Dashboard.tsx:648「冲刺提交中」+ Select.tsx:1133 英文注释 `only affects itself`）——逐字节一致 | 延续（第 9 轮） |
| **N-2** | 延续第 7 轮 | 开窗后 pick → 至多一拍调度延迟（250ms 黄金期 / 1s 常态）。维持 | 延续（第 8 轮） |
| **N-3** | 延续第 6 轮 | 三处格式卫生字形级复证一致（详见结论第 3 条） | 延续（第 7 轮） |
| **O-1** | Toast viewport 滚动残余 | Toast.tsx:105 实测维持；另见本轮 Toast 去重/3.5s 自消 + 退避重试 6 次上限下的残余观察 | 延续 |
| **O-2** | 倒计时 NaN 防御 | useTickingCountdown.ts:20 `target ? ... : 0` + 输入源全受控复证 | 延续 |
| **O-3** | 三处手写 modal 焦点陷阱 | Login/Select/Admin 三处 role=dialog + aria-modal + Esc + autoFocus 复证 | 延续 |
| **O-4** | 每秒整页重渲 | useTickingCountdown 单 hook 自 tick + 消费组件重渲（Dashboard fallback 注释），bailout 依赖论证 | 延续 |
| **O-5** | Dashboard 无 key={account} | App.tsx:331-344 复证；后端 StateForAccount 按账号过滤 + 数据不串线 | 延续 |
| **O-6** | ui 模板残宽 | CardFooter 零消费 + Button 12 变体/实际子集 | 延续 |
| **O-7** | 管理态多标签页 | xk_admin_token/name 无 storage 监听，方向安全 | 延续 |
| **O-8** | 401 闭包 current | client.ts:64-69 前置广播 + App 反查；session 恒为真实主体 | 延续 |
| **O-9** | Button dark + audit 误报 | audit.mjs `bg-neutral-95x` 裸行匹配将 Button dark variant 行误报（函数体非 className）；实测 `node scripts/audit.mjs` → exit 1（1 处违例确认）。**维持豁免** | 维持 |
| **O-10** | Dashboard 状态机四态 | in_range 预留/无写入点复证 | 维持 |
| **O-11** | M-1 修复边角语义 | ① 收敛窗口（安全方向，评分见 A-②）；② 新稳态假阳性命中——**本轮新补一条命中路径**（见 A-④） | 维持（更新） |

### A-② M-1 修复端到端稳态编辑真实落库证据（R65 新增）

上一轮 O-11 的「编辑 PUT → /state 回显自身改动」路径停留在机制论证，本轮补齐后端侧契约证据：

1. **PUT /api/targets**：`handleTargets`（handler.go:486-525）校验（条数上限 100 / class_id>0 / publish_id>0 / priority≤999）后依次 `Store.SetTargetsForAccount`（落库）→ `Sched.SetTargetsForAccount`（scheduler.go:459-495，`s.mu` 持锁）→ `rebuildCoursesForAccountLocked(acct, targets)`（575-609：删旧状态行重建，命中 done 置 `success`、命中 refused 置 `pending`+「已手动退选（自动引擎不再接管，可重新设为目标恢复）」，**发布元数据 PublishName/BeginDate 随目标透传**）。
   → **前端该 PUT 的唯一合法副作用 = /state 轮询返回与 selected 完全一致的 courses。** 确认 O-11「保守命中」的根因（前端整包 PUT 与后端是同构重建）在实现层成立。
2. **GET /api/state**：`handleState` → `Sched.StateForAccount`（704-729，同样持锁）只回发该账号 courses；**`OpenTimeKnown` 判定 `st.OpenTime.After(nowAlignedLocked())`**——若 Put 发生在窗口关闭之后（open_time 恒为过去时刻 → false），则 stateData.open_time_known=false、`openTimeStr=null`，`/state` 数据到达时防抖 effect 已重跑、404 兜底 / `begin_times[0]` 兜底同步维持。无首帧自锁杀。

### A-③ R64 关注的两个旧场景复核（M-1 修复涉及）

- **「编辑 PUT 后 /state 回显自身改动」**（R64 O-11-② 场景 1）：PUT 完成后 courses 重建为自身 targets；`selected` 与 courses 重构一致，回显合并无新增 entry；echoedRef 已 true 短路；**任何时刻防抖/手动 flush 的整包 PUT 与后端一致，无覆盖删除。** 维持。
- **「手动报名后旧 courses」**（R64 O-11-② 场景 2）：handleSelectClass 成功只 invalidate `/electives`+`/state`，后端手动 `MarkDone` 已按目标重建 courses；`/state` 轮询返回的 courses 与 selected 依然一致。维持。

### A-④ 新稳态假阳性命中路径（R65 新增微观记录）

R64 O-11-② 列了 4 条路径（编辑 PUT 回显重构 / 手动报名旧 courses / 全清空 PUT [] 在飞 / /state 持续失败）。**本轮新补一条**：

**新增改备选（2s 轮询 /state 已返回重建后的自身 courses、rev>0 回显 effect 短路不修正）**：
- putback：pick() 设备选后 setSelected 含 [首选,备选]；400ms 防抖 → PUT → 后端 rebuild → 2s 轮询 /state 返回 courses=[首选,备选]；
- 回显 effect 依赖含 rev，rev 已 >0 → effect 重跑，`if (echoedRef.current) return` 立即短路（不合并 courses → 不把「后端自身镜像」重复合并进 selected）；
- targetGuard 返回 false（无 release 残留 + echo=true）→ 不置脏。
- 结论：**这条路径在 M-1 修复前（echoed 一直 false + 守卫 `courses 非空 && hasSelected`）会被判「未回显」推迟一次；M-1 后 true 放行——行为朝「立即落库」收敛，无折叠、无覆盖。** 加入 O-11 第②条路径清单。

### A-⑤ target-guard-check 两条 M-1 断言复证

`target-guard-check.ts:70-79` 两条断言（`courses 非空 + 有选中 + 已回显 → 放行`、`首帧未到 + 已回显标志 → 仍推迟`）本轮实测全绿（18/18）。字面复核：断言参数传 `shouldDeferSave(nonEmptyState, true, true)` / `shouldDeferSave(undefined, true, true)`——分别覆盖稳态放行与 undefined 优先级不破，真实覆盖 M-1 语义。

---

## 新视角 B：六防保存链逐字符零回归（F43/F42/F40/F39/F36/F48-M1 + M-1 维度）

| 防护族 | 本轮结构复证 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard hasSelected 第二参数全绿；echoed=true 下 hasSelected 被短路但清空语义由「全清空 + 首帧确证」双闸兜住（`stateData===undefined` 无条件推迟 + 回显 effect `rev>0 && !anyHas` 不合并） |
| **F42-M1 判据解耦（含 M-1 echoed 维度）** | 纯函数判据 + 三消费点（flushTargets:501 / handleBack:594 + while:602 / 防抖:686）全部传 `stateDataRef.current + selectedCount/hasSelectedNow() > 0 + echoedRef.current`；echoed 只放行「已回显稳态」；/state 持续失败期间 echoed 恒 false、判据退化为原实现同款、stateData 依赖 effect 重跑自愈——**本轮在 O-11-② 场景 3（全清空 PUT 在飞）与 A-④（新增改备选）两条新稳态路径上重推，均不被守卫误拦** |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish 四消费点（回显 287 / 独立清理 effect 316-327 / flush 520 / 防抖 706）+ cleanStaleSelected 清理 effect（依赖含 selected）+ toast 判 `!unmountedRef.current` 全在 |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期；断言场景 C/D 全绿 |
| **F36-01 key={account} + 账号复位守卫** | App.tsx:293/339 双 keyed + Select.tsx:195-202（acountKey + setSelected/setRev/echoedRef = false/setEchoDone(false)，TDZ 安全） |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50`（F48-M1 已把定位类并入 viewport） |

**唯一未演进点（维持）**：`client.ts:84-95` body-401 分支内嵌 `let account = ""` + 重复正则（`/electives` 业务 URL 携带的 `&` 也用 `[?&]` 匹配）——与 `extractAccountFromPath` 同款逻辑未复用，非缺陷（该函数专门服务广播路径，两处负责不同时序，见 C-3 闭包链路）。

---

## 新视角 C：五个实证链延续

1. **pick() fallback 判定源恒最新渲染**：Select.tsx:346 `pick(t.publish_id, c)` 回调参数来自渲染期 map 的 `t`（可选链、course_name 均来自已提交的 ClassItem）；selected 为渲染闭包当前值。维持。
2. **Dashboard dateGroups 纯 useMemo**：Dashboard.tsx:219-263 依赖 `[courses, electives?.publishes]`；`pubById/byDate/groups` 全在 useMemo 内部新建；key 唯一（dateKey→publish_id→class_id）。维持。
3. **App onUnauthorized 闭包**：依赖 `[adminName, inAdmin, adminToken]`；listener 每次重建 + `onUnauthorized` 每次从 localStorage 快照重读 sessions 反查。维持。
4. **共享 react-query 缓存两路由**：Dashboard:172 与 Select:56 同 key `["electives", account, sessionToken]` + 同 URL → 切页零重复请求；/state 同源（131/146）；Select 内 refetchInterval 通过 `queryClient.getQueryData(["state", account, sessionToken])` 读缓存、与 stateData 同源零时序依赖。**本轮加验：React 19 并发下 useQuery queryKey 从未变，ObservedQueries 去重成立。** 维持。
5. **saveNow lastJson 去重链**：putTargets 成功后 lastJson=json，重复 PUT（回显合并/轮询重跑）短路；在飞走 dirty 补发。维持。

---

## 新视角 D：全包逐行通读找新问题

**结论：零新增缺陷（MAJOR/MINOR 皆无），新增 1 条 OBSERVE 细节（O-12：Dashboard extrasMs useMemo 依赖链中的 `primaryMs` 派生值 + `Date.now()` 渲染期读取），其余文件复核无新增，沿用 N/O 延续集。** 逐文件要点：

1. **App.tsx**：login/logout/onUnauthorized/onDeleted/onBackToStudent 五条会话链全核对；`xk_admin_token` 由 4 处写入（login:98/99、logout:121/122、onUnauthorized:214、onBackToStudent:312）+ 2 处清除（onDeleted:175）。**onDeleted 里删除的是管理员自己时 `acct === adminName` 单键判定**（等 adminName 恰为 `""` 时与「删除自身」同语义，无冲突）；onBackToStudent 在 `others.length > 0` 时调用 `setCurrent(others[0])` 前不清 adminToken → 已清。零缺陷。
2. **Select.tsx**：M-1 修复载体，六防链正文已复证；`saveNow` 无抛错路径（catch 全兜）；`handleBack` 3 轮 flush 收敛 + `while` 兜底；`useTickingCountdown` 兜底 `begin_times[0]` → `toISOString()`（R64 已确认往返无损）。零新增缺陷。
3. **api/client.ts**：`api()` 三形态 401 单广播（HTTP-401 → body-401 → `r.status !== 401` guard）、AbortError 统一映射、non-2xx `-2` 文案。**唯一新观察**：body-401 分支内嵌重复正则（见 B 表），格式卫生但非缺陷。
4. **Login.tsx**：submit/activate 幂等守卫 + 1001 票据透传 + F12-M3 票据过期引导。零缺陷。
5. **Dashboard.tsx**：F10-06 去硬编码；logs limit=100 与后端 LoadLogs 对齐；日期分组 `begin_date` 切片 `(c.begin_date ?? "").slice(0, 10)`；`extrasMs` 依赖链复证（D 节新观察）。零新增缺陷。
6. **Admin.tsx**：五 Tab 全量；CodesTab removing Set 独立；ConfigTab `!loaded` 拒存 + refetch 才自增 epoch；StatsTab `window_closed` 三态 + `token_valid` 部分失效；LogsTab limit=200；删除 Dialog 幂等 + autoFocus。零缺陷。
7. **ui 组件 + global.css**：Button/Input/Card/Badge/Progress/Tabs/Toast 逐字节复证；`Progress` `value/max` 非负防御（`Math.min(Math.max(...,0),100)`）；Toast `variant === "warning"` 走 amber 配色 + `AlertCircle`，Select.tsx 的「发布已更新」toast 用 warning variant 有对应样式。零缺陷。
8. **lib/useTickingCountdown.ts**：`diff` 全受控；F10-07 目标变化即校正 now。零缺陷。

### D-新 OBSERVE：Dashboard `extrasMs` 派生值在渲染期读取 `Date.now()`（`nowMs`）驱动折叠行摘要

- 位置：`Dashboard.tsx:94-108`（relativeCountdown）+ 198/448（`const nowMs = Date.now()` 渲染期读取 + extras 折叠行调用）。
- 机制：`useTickingCountdown` 每秒 `setNow` 触发依赖它的组件（`cd` 消费区块）重渲染，`nowMs` 在每次重渲染时重新取 `Date.now()`（最坏 1s 陈旧，属「整页只重渲染倒计时一处」注释承诺范围内）；`relativeCountdown` 对 past 常量返回「已开放」。
- **触发条件**：任何使 `cd` 之外组件重渲染的事件（/state 2s 轮询、/electives 30s、toast、extrasOpen 折叠切换）都会让 `nowMs` 重新计算——瞬时成本 = 每次重渲染一次 `Date.now()` 调用（微秒级），无累积。**折叠行行内摘要的 1s 陈旧窗口由 1s tick 驱动刷新，无实时性缺口（40s 内最多 1s 陈旧）。**
- 影响：无（纯展示层、微秒级成本）。观察意义仅在「渲染期读取时间」这一模式与 `useTickingCountdown` 秒级自 tick 的边界——若未来改为 10s tick，折叠行摘要将最长 10s 陈旧。维持方向安全。

---

## 新视角 E：验证实证

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit`（后台） | exit 0 |
| 前端生产构建 | `cd web && npm run build`（后台） | exit 0（1948 modules / 417.58 kB js / 41.07 kB css） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts`（后台） | 18/18 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts`（后台） | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts`（后台） | 5/5 全绿 |
| 视觉护栏 audit | `node scripts/audit.mjs`（前台重跑） | exit 1，1 处违例 = Button.tsx:21 dark variant 行 `bg-neutral-95x` 裸行匹配误报（函数体非 className，R64 基线既有，O-9 豁免） |
| **后端契约核对** | `grep/sed handler.go 486-525/530-548 + scheduler.go 459-495/575-609/704-729` | 确认 targets 落库 → 持锁重建 courses（含元数据透传）→ /state 只回本账号 courses，O-11 根因复证 |
| **react-query 键清洁度** | `grep -rn "useQuery(\\|queryKey" web/src` | 全站 8 个查询键全部唯一且同路由同键，v8 观察者去重成立 |
| 调试残留/TODO | `grep -rn "console.\\|TODO\\|FIXME" web/src` | 零命中（"XK-XXXX-XXXX-XXXX" 是 placeholder，非 TODO） |
| 工作树一致性 | 开局 `## master` / 收尾 `git status --short --branch` | 干净；`git diff HEAD --stat -- web/src/` = 0 行 |
| 临时文件 | 全部验证为只读命令，`%TEMP%` 随后清理 | 仓库内仅本报告 |

---

## 上轮观察项延续表

| 编号 | 上轮裁决 | 本轮复核 | 裁决 |
|---|---|---|---|
| M-1 | 已修闭合 | 修复本体 + 端到端 NEB 验证（A-②）+ 断言 18/18 | **已修，第 2 轮闭合确认** |
| N-1 | 延续 第8 轮 | 落位点字形复核 | 延续（第 9 轮） |
| N-2 | 延续 第7 轮 | 维持 | 延续（第 8 轮） |
| N-3 | 延续 第6 轮 | 字形级逐字节一致 | 延续（第 7 轮） |
| O-1~O-8 | 逐项延续 | 机制实证相同 | 延续 |
| O-9 | 维持豁免 | audit exit 1 实测复核 | 维持（豁免） |
| O-10 | 维持 | 并入 N-1 同源 | 维持 |
| O-11 | 新增 | R65 复核（收敛窗口评分 + 假阳性命中路径清单补一条 A-④） | **维持（更新）** |
| **O-12** | — | **本轮新增**（D 节）：Dashboard `nowMs` 渲染期日期读取 + extrasMs 折叠行 1s 陈旧窗口 | 新增 |

---

## 已核对无缺陷的高风险区域清单

- **R63 M-1 修复本体**：targetGuard.ts:64-71 echoed 参数 + 三消费点 + 两张置位路径（238/295/200）；端到端 PUT → rebuild → /state 链路同构无覆盖（A-②）。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1）：B、C 表逐条复证，M-1 叠加性增强零回归。
- 后端 /state//targets 契约（handleTargets 校验 + 持锁重建 + 账号过滤回发）。
- 401 三形态单广播 + onUnauthorized 幂等 + 会话快照式三连（A-①-O-8 复核）。
- Admin 五 Tab 全量（在飞幂等/配置拒存/删除 Dialog/日志 limit）。
- 倒计时三态顺序、回显/防抖/刷新/卸载生命周期、手动报名/退选双端幂等、Dashboard dateGroups 纯 useMemo、共享 react-query 缓存、saveNow lastJson 去重链。

## 教训

1. **连续多轮零提交的稳态审查重点 = 「上一轮修复闭合的证据升级」而非「找全新缺陷」**：R63 M-1 修复在本轮从「机制论证」升级为「端到端后端契约证据」——同样的修复，上一轮的闭合主要靠纯函数断言 + 时序推演，本轮补齐了「PUT 后 courses 重建由 rebuildCoursesForAccountLocked 保证与 selected 同构」的实现层证据。闭合核验应随轮次逐层渗透（断言层 → 时序层 → 实现层），证明层次升级本身就是审查进展。
2. **「新增提防抖保存」路径也要核 `rev>0` 短路与 echoed 的组合**：A-④ 新补的假阳性命中路径说明，稳态放行（echoed=true）后每个新改动路径都会过一遍回显 effect（rev 变更触发重跑），但 `echoedRef.current` 首行短路让「后端镜像不重复合入 selected」。若未来有人把「回显 effect 依赖」里的 rev 移除，会把后端镜像写出意外条目。审查稳态解锁时，要连「解锁后所有被重新激活的路径」一起验证。
3. **后端契约证据比前端代码更值钱**：O-11-② 的全部「保守命中」路径都依赖「PUT 整包与后端一致」这一前提——本轮直接读 handler.go + scheduler.go 确认 `SetTargetsForAccount` 持锁重建 courses 后的数据与前端 selected 同构，而非再次从机制上论证。前端审查团队应保持「后端契约比对」作为常备弹药。
4. **归档报告的证据链引用要让后续轮次能够逐段复现**：O-11 从 R64 起每轮引用「scheduler.go:459-495/575-609」即每轮可精准回读根因，把「假阳性路径清单」做成可追加清单（R65 在本轮补了 1 条）比在正文里重构一次更省审查成本。