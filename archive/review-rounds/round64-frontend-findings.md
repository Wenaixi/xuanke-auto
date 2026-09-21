# round64 前端只读审查发现报告

> 审查基线：master @ `1861093`（R63 收官，R63 前端一修 commit `7e4ef3a` 已合入）。开局 `git status --short --branch` = `## master` 干净。本轮聚焦 **R63 M-1 修复完全性核验**（任务书新视角 A）+ 六防保存链零回归（B）+ 五个实证链延续（C）+ 全包逐行通读（D）+ 验证实证（E）。审查期间绝对只读——唯一新建文件为本报告；全部验证（tsc/build/断言/audit）为只读命令，`%TEMP%` 未落任何临时脚本。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 14（延续集）**

**R63 M-1 修复判定：完全正确，修复闭合。** 核心新增 OBSERVE 一条（O-11：R63 M-1 修复在「`/state` 首帧未到 + 首帧携带旧目标」暂态下的两轮收敛窗口变宽 + 新稳态新增假阳性命中路径），**无 MAJOR/MINOR，无数据丢失/覆盖路径**。R63 M-1 修复本身零缺陷；三消费点（防抖/flush/handleBack+while）传入 `echoedRef.current` 的时序与回显 effect（226-297）置位点覆盖全部回显完成路径；`echoed=true` 放行与 `undefined` 无条件推迟的正确组合验证成立；无任何误放行（回显未完成却覆盖后端旧目标）的路径。TDD 断言补的两条确实覆盖修复语义（`courses 非空 + 有选中 + 已回显 → 放行`、`首帧未到 + 已回显标志 → 仍推迟`），第 2 条尤其验证了「echoed 误 true 也无损」的保守边界。

**最致命 3 条（本轮按影响排序，均为 OBSERVE 级观察，无缺陷）**：
1. **O-11（本轮新增观察）**：M-1 修复引入的两条边角语义偏移——①「首帧未到 + 有选中」由「等待期间 ≤1s（在飞 PUT 补发快）或 5s 兜底（handleBack）」变为「echoed 在回显 effect 到达前恒 false → 多轮 50ms 轮询窗口（单轮 PUT 往返 ≥ 20ms API 超时上限，实际 1s 内收敛）」——窗口变宽仅 2~5s，安全方向，无数据丢失/覆盖；② 新稳态「echoed=true 放行普通编辑」引入的「courses 非空恒真 → 编辑 PUT 后回显重构注入自身改动 + 手动报名后 /state 轮询旧 courses」等假阳性命中路径——保守方向（PUT 整包与后端一致，无覆盖删除）。见 D-2 全时序推演。
2. **N-1（延续第 7 轮）**：「后台冲刺」文案 vs 后端仅 10 秒黄金期 250ms 冲刺——纯文案、零行为影响。维持。
3. **O-9（延续）**：Button dark variant `bg-black` 与「实心黑洞清零」护栏相邻；audit.mjs 对 `hover:bg-neutral-900` 裸匹配误报（本轮实测 exit 1，1 处违例确认）。维持豁免。

---

## MINOR

（本轮无 MINOR。）

---

## OBSERVE

### O-11.【本轮新增】M-1 修复的两条边角语义偏移（安全方向，无缺陷）

**位置**：`web/src/lib/targetGuard.ts:64-71` + `web/src/routes/Select.tsx:226-297`（回显 effect）三消费点。

**① 首帧未到 + 有选中：多轮 50ms 收敛窗口变宽（原 5s 兜底 → 新稳态 5s 内逐轮收敛）**
- 原实现：`shouldDeferSave(undefined, true)` 无条件 true → 防抖置脏；stateData 依赖 effect 重跑 → 重跑后同判据打回；直到「courses 空」或 handleBack 5s。F42-M1 自愈语义 = 一条「/state 数据到达」事件链。
- M-1 后：`echoed` 在回显 effect 到达前恒 false → 判据退化为原 `(courses 非空 && hasSelected)` 同款 → 首帧未到（undefined）无条件 true 仍推迟；**当首帧到达且 courses 非空** → 回显 effect 合并 → echoed 置 true → 防抖 effect（依赖含 stateData）重跑 → 新 timer → 守卫放行 → 整包 PUT（含旧目标 + 用户改动）→ 落库。
- 收敛窗口 = 回显 effect 置位后的一轮防抖（400ms）→ 一轮 PUT 往返（≤20s，实际校园网 1s 内）→ 约 1~3 秒。原 F42-M1 语义只覆盖「首帧到达触发一次 effect 重跑」，现为「首帧到达 + echoed 翻转后再次重跑」——两轮收敛，窗口变宽 2~5s，纯安全方向。

**② 新稳态 echoed=true 引入的假阳性命中路径（保守方向）**
- 稳态放行后「courses 非空恒真」注入以下命中路径，全部为「PUT 整包与后端一致」的保守 PUT，无覆盖删除：
  - 编辑 PUT → 后端 rebuildCoursesForAccountLocked 按 targets 重建 courses → 下一拍 /state 轮询返回自身改动 → 无副作用（已落库）。
  - 手动报名/退选成功 → actionLoading invalidate state → /state 轮询返回旧 courses（发布级 hasSelected 未变）→ 无新改动不触发防抖。
  - 全清空 PUT [] 在飞 → 首帧 courses 携带旧目标到达 → 回显 effect `rev>0 && !anyHas` 返回 prev 不合并 → echoed 置 true → 防抖重跑 → shouldDeferSave(stateData, false, true) → `echoed=true` 放行 → PUT [] 落库，清空语义完整保存。
  - `/state` 持续失败 → echoed 恒 false（回显 effect 只在 stateData 到达时置位）→ 仍推迟 + handleBack 5s 兜底（F42-M1 自愈语义不受破坏）。
- 结论：echoed=true 放行只发生在「回显 effect 已执行且合并完成」后——selected 已含全部后端目标（回显合并语义保证），整包 PUT 与后端一致，无覆盖删除路径。

**裁决**：OBSERVE（无行为缺陷，窗口变宽为安全方向，假阳性命中为保守方向）。若未来追求精确，可让防抖回调消费时刻改用 `echoedRef.current || shouldDeferSave(...)` 短路，但当前实现更保守（防「echoed 误 true 而回显未发生」的边界）。

### N-1.【延续，连续七轮】「冲刺」文案 vs 后端仅 10 秒黄金期 250ms 冲刺——维持

**落位点**：Select.tsx:1134 `已设为后台冲刺`/`设为后台冲刺目标` + Dashboard.tsx:648 `isInRange ? "冲刺提交中"` + Select.tsx:1123 注释 `only affects itself`。前端无黄金期精确感知（三态伪精确，明确否决做）。若治 = 两行统一「后台目标」删“冲刺”。

### N-2.【延续，第六轮】开窗后 pick → 至多一拍调度延迟（250ms 黄金期 / 1s 常态）。维持。

### N-3.【延续，第五轮】三处格式卫生（英文注释残留 Select.tsx:1123 / `const lastJson` 4 空格缩进 Select.tsx:379 / Dashboard.tsx:501 同行折叠）。本轮字形级复证与工作树逐字节一致。维持。

### O-1.【延续】Toast viewport 滚动交互残余（Toast.tsx:105）。M-2 去重 + 3.5s 自消下概率趋零。维持。

### O-2.【延续，连续九轮】useTickingCountdown NaN 防御（`diff = target ? ... : 0`）——输入源全受控合法。维持。

### O-3.【延续】三处手写 modal 无完整焦点陷阱（Login/Select/Admin）——均已有 role=dialog/aria-modal + Esc + autoFocus。维持。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询——bailout + TabsContent 懒渲染。维持。

### O-5.【延续，连续九轮】Dashboard 挂载点无 `key={account}`（App.tsx:331）→ `expandedDates`/`extrasOpen` 跨账号残留。后端 StateForAccount 按账号过滤双证数据不串线，影响纯展示层。维持。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button 12 变体/实际子集、Badge 8 变体/实际子集。维持。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页——无 storage 监听。方向安全。维持。

### O-8.【延续】401 保护窗口闭包 current：`lostRaw = detail?.session || detail?.account || current`，session 恒为 401 真实主体、current 兜底永不触发。维持。

### O-9.【延续】Button dark variant `bg-black` 实色 + audit.mjs 对 `hover:bg-neutral-900` 裸匹配误报（本轮实测 exit 1 确认，1 处违例 = Button.tsx:21）。维持豁免。

### O-10.【并入 N-1 同源】Dashboard 状态机四态文案的「提交中」与后端 `submitted`/`in_range` 双状态映射——纯展示观察（in_range 为预留态无写入点）。维持。

---

## 新视角 A 逐项实证裁决（R63 M-1 修复完全性核验——本轮核心）

### A-① shouldDeferSave echoed 维度语义完备性

`web/src/lib/targetGuard.ts:64-71` 现实现：

```ts
export function shouldDeferSave(stateData, hasSelected, echoed): boolean {
  if (stateData === undefined) return true   // 首帧未到：无条件推迟（回显未发生，PUT 会覆盖旧目标）
  if (echoed) return false                    // 已回显：selected 完整，整包 PUT 与后端一致，放行
  return (stateData.courses?.length ?? 0) > 0 && hasSelected  // 未回显：仅"courses 非空 && 有选中"推迟
}
```

- **undefined 无条件推迟与 echoed=true 放行的组合正确**：两个分支互斥覆盖「回显完成」与「回显未完成」全部分区；「undefined + echoed=true」落入首行（首帧未到回显 effect 必然未置位 echoed，但函数级防御仍先判 undefined 再判 echoed——防御性，实测断言「首帧未到 + 已回显标志 → 仍推迟」覆盖）。
- **echoedRef 置位点覆盖全部回显完成路径**：回显 effect（Select.tsx:226-297）两条置位路径——courses 空分支（234-241，确证后端无旧目标）与非空合并分支（250-296，合并完成后 295-296 置位）。**但注意**：非空合并分支在 `pubs.length === 0`（245 行）时提前 return **不置位**——即「/state 首帧到达、courses 非空、但 /electives 尚无 publishes（开窗瞬间平台清空）」时 echoed 恒 false → 防抖继续推迟 + handleBack 5s 兜底，安全方向（此刻 selected 只含用户改动，整包 PUT 会覆盖后端旧目标——推迟正确）。publishes 恢复后 299 行 tabs 重建，回显 effect 依赖 `data`（data 更新触发重跑）→ 补合并 + 置位。**置位路径完整，无遗漏回显完成场景**。
- **三消费点时序正确**：防抖（686）/flushTargets（501）/handleBack 判定（594）+ while（602）全部传 `echoedRef.current`；handleBack 的 while 每 50ms 重读 ref（回显 effect 置位后 50ms 内感知）→ 等待立即结束。无「echoed 在回显 effect 置位前被消费为旧值」的时序漏洞（消费全部在 effect 之后或 ref 实时读取）。

### A-② 五条关键时序推演

- **a) 首帧未到即点新课 → echoed=false → 仍推迟**：正确。`stateData===undefined` → 首行 true；回显 effect 不置位（228-231 明确「首帧未到绝不提前置位」）→ echoed 恒 false → 判据退化为原实现同款。与 F42-M1 语义一致，无误放行。
- **b) 首帧到达 courses 非空 → 回显合并 → echoed=true → 用户改备选 → 防抖放行**：正确。合并后 selected 含全部后端目标 + 用户新改动 → 整包 PUT 与后端一致 → 落库后后端 rebuildCoursesForAccountLocked 按 targets 重建 → 下一拍 /state 轮询返回自身改动（无覆盖）。**本轮实证：原 M-1 缺陷（已回显稳态编辑永不落库）在合并语义下不存在**——selected 完整无缺，整包 PUT 无覆盖。
- **c) 首帧到达 courses 空 → echoed=true（空分支 238）→ 用户全清空后改 → 放行**：正确。空 courses = 确证后端无旧目标 → echoed=true → 防抖放行 PUT [] 或新目标（清空语义）。注意：此处「courses 空 + 已回显」下 hasSelected 仍参与第二参数但 echoed=true 短路放行——与 F43 全清空语义一致。
- **d) /state 持续失败 → echoed 恒 false → 仍推迟 + 5s 兜底**：正确。回显 effect 只在 stateData 到达时置位 echoed（232 行 `stateData === undefined` return）→ 失败期间 echoed 恒 false → `stateData===undefined` 无条件推迟 → handleBack 5s 兜底继续（flush 内 F15/F16/F17 假清空守卫仍拦截）→ F42-M1 自愈语义（/state 数据到达触发防抖 effect 重跑）不受破坏。**本轮实证：M-1 修复未破坏 F42-M1 的核心契约**。
- **e) 全清空 PUT 在飞 + 首帧携带旧目标 → rev>0 && !anyHas → 不合并 → echoed 置 true → 防抖重跑放行 PUT []**：正确。回显 effect 254 行 `if (rev > 0 && !anyHas) return prev` 不合并；295-296 仍置位 echoed → 防抖重跑 → shouldDeferSave(stateData, false, true) → echoed=true 放行 → PUT [] 落库，清空语义完整保存绝不复活。

### A-③ 反向推演（假清空/覆盖检查）——无任何误放行路径

穷举「echoed 误 true + 回显未完成」的可行组合：
- **echoed 只能由回显 effect 置位**（Select.tsx:238/295 两处 + accountKey 复位 200）——非回显 effect 无任何路径写 echoedRef。置位时 selected 必已包含完整后端目标（courses 空分支确证无旧目标 / 非空分支合并完成）→ echoed=true 时「回显未完成」不可能成立。
- **唯一边界**：「pubs.length === 0 时 courses 非空」分支不置位（245 行 return）→ echoed 恒 false → 推迟（安全方向），即使 5s 兜底放行，flush 内 F15/F16/F17 守卫仍拦截发布缺席的覆盖。
- 结论：**无任何路径在回显未完成时误放行整包 PUT 覆盖后端旧目标**。

### A-④ target-guard-check 补的两条断言是否覆盖修复语义

| 断言 | 覆盖语义 | 裁决 |
|---|---|---|
| `courses 非空 + 有选中 + 已回显 → 放行`（target-guard-check.ts:70-74） | M-1 核心修复：稳态放行（selected 完整） | **真实覆盖**——正是 M-1 缺陷的判据三元组 |
| `首帧未到 + 已回显标志 → 仍推迟`（75-79） | undefined 无条件推迟优先于 echoed | **真实覆盖且关键**——验证「echoed 参数不破坏 undefined 优先级」，守住 F42-M1 首帧保护 |

第 2 条尤其重要：它排除了「echoed 参数被调用方错误传入 true 时破坏首帧保护」的回归——M-1 修复即使被未来调用方误传 echoed=true，`stateData===undefined` 首行仍拦截。**两条断言确实覆盖修复语义且留有余量**。

---

## 新视角 B：六防保存链逐字符零回归（F43/F42/F40/F39/F36/F48-M1 + M-1 修复）

| 防护族 | 本轮结构与语义复证（M-1 修复后） |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:64-71 第二参数 hasSelected 保留；断言 8 项覆盖（target-guard-check.ts:42-79）；echoed=true 下 hasSelected 被短路但清空语义由「全清空 + 首帧确证」双闸兜住 |
| **F42-M1 判据解耦** | 纯数据判据不依赖 echoedRef 的初衷在「首帧未到」路径保留（stateData===undefined 无条件 true）；echoed 参数仅作为「已回显稳态」的附加放行信号；防抖 effect 依赖仍含 stateData（Select.tsx:747）自愈。**M-1 修复未破坏 F42-M1 核心：/state 持续失败期间 echoed 恒 false、判据退化为原实现同款、自愈链完整** |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（287/316-327/518/696）全在；独立清理 effect（316-327）依赖含 selected；toast 判 `!unmountedRef.current` 不轰炸卸载后。M-1 修复未触碰 |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）；断言场景 C/D 覆盖。M-1 修复未触碰 |
| **F36-01 key={account} 双保险** | App.tsx:293/339 双 keyed + accountKey 守卫（Select.tsx:195-202，echoedRef 复位 200 在守卫内）TDZ 安全。M-1 修复未触碰 |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动。M-1 修复未触碰 |

**关键时序推演（M-1 修复后补 2 则新稳态推演，见 O-11）**：
- 编辑 PUT → 后端 rebuildCourses 注入自身改动 → /state 轮询返回 → 无副作用（已落库）。
- 手动报名/退选 → actionLoading invalidate → /state 旧 courses → 无新改动不触发防抖。
- 全清空在飞 → 首帧旧 courses → rev>0 && !anyHas 不合并 → echoed 置 true → 防抖放行 PUT []。
- /state 持续失败 → echoed 恒 false → 推迟 + 5s 兜底（F42-M1 自愈链完整）。
- saveNow lastJson 去重链（Select.tsx:429）：全清空 PUT [] 成功后 lastJson="[]"，新改动 json ≠ "[]" 正常再发；在飞走 dirty 补发。无乱序覆盖。

**核心结论**：六防链在「首帧/全清空/在飞/发布重建/新稳态编辑」五类场景下零回归——M-1 修复是叠加性增强（echoed 仅放行已回显稳态），未移除任何既有防线。

---

## 新视角 C：五个实证链延续（R62/R63 报告附加实证）——逐一复核维持

1. **pick() fallback 判定源恒最新渲染**：Select.tsx:346 `pick(t.publish_id, c)` 回调参数来自渲染期 map 的 `t`；`pick` 是事件处理器（每次渲染新定义），`selected` 为渲染闭包当前值，与「先快照后 set」一致。维持。
2. **Dashboard dateGroups 纯 useMemo**：Dashboard.tsx:219-263 依赖 `[courses, electives?.publishes]` 均 react-query 缓存引用；`pubById`/`byDate`/`groups` 全在 useMemo 内部新建、无外部突变；key 均唯一。维持。
3. **App onUnauthorized 闭包**：`adminName/inAdmin/adminToken` 每改重建 effect 闭包（依赖数组 235）；`onUnauthorized` 每次从 localStorage 快照重读 sessions 反查 lostAccount。维持。
4. **共享 react-query 缓存两路由**：Dashboard.tsx:171-175 与 Select.tsx:56-57 同 `queryKey: ["electives", account, sessionToken]`——两路由互斥挂载但共享缓存，切页零重复请求；Select 升频即时接管。维持。
5. **saveNow lastJson 去重链**：见 B 表推演。维持。

---

## 新视角 D：全包逐行通读找新问题

**结论：零新增缺陷（MAJOR/MINOR 皆无），新增 O-11 一条 OBSERVE（见上）。** 逐文件通读要点：

1. **App.tsx**：sessionToken/current/accounts 派生无异常；`if (accountKey !== account)` 渲染期 setState（Select.tsx:196-202）在 React 19 并发下是合法 bailout 模式（条件 setState + 返回，未触发额外渲染循环——accountKey 与 account 仅在切换瞬间不同一次）；onDeleted 的 `if (acct === adminName) setInAdmin(false)`（F39-M1）与 logout/onUnauthorized 对称。零缺陷。
2. **Select.tsx**：本文件为 M-1 修复载体，逐字符复核三消费点（501/594/602/686）+ 回显 effect 置位点（238/295/200）。见 A-①/②/③。零缺陷。
3. **api/client.ts**：HTTP 401 前置广播（F41-N2）+ body 401 仅 `r.status !== 401` 补（N-1 R45 单广播）；`extractAccountFromPath` 对含字面 `account=` 的非 query 段（如伪路径 `/account=xxx/state`）不误匹配（断言 D 覆盖）。零缺陷。
4. **Login.tsx**：1001 分支保存 ticket + 激活回传 + F12-M3 票据过期引导。零缺陷。
5. **Dashboard.tsx**：F10-06 去硬编码「/ 3 门」；logs limit=100 与后端 LoadLogs 上限对齐；日期分组 useMemo 纯化；collapsed 折叠种子 null/[] 语义分离。零缺陷。
6. **Admin.tsx**：五 Tab 全量核对——CodesTab removing Set 按码独立 / ConfigTab `!loaded` 拒存 + refetch 成功才自增 epoch / StatsTab `window_closed` 三态 + `token_valid` 部分失效 / AccountsTab targets/success 空数组兜底 / LogsTab limit=200。零缺陷。
7. **ui 组件 + global.css**：Button/Input/Card/Badge/Progress/Tabs/Toast 全部无异常；Toast 去重 + 自增 id + viewport 定位（F48-M1）。零缺陷。
8. **lib/useTickingCountdown**：`new Date(target).getTime()` 输入源受控合法（O-2）。**本轮追加核对**：`Select.tsx:759-764` 兜底 `new Date(data.begin_times[0]).toISOString()`——`begin_times` 为毫秒时间戳（后端 `BeginTimes []int64` JSON），`toISOString()` 输出 RFC3339，useTickingCountdown 再 `new Date()` 解析回毫秒——往返无损，时区安全（toISOString 恒 UTC、Date 解析恒 UTC）。零缺陷。

---

## 上轮观察项延续表（M-1 修复闭合复核 + N/O 逐条）

| 编号 | R63 报告 | R64 复核 | 裁决 |
|---|---|---|---|
| **M-1** | MINOR（shouldDeferSave 稳态推迟无解锁） | **修复闭合**：echoed 第三参数 + 三消费点 + 2 断言全落地；语义完备性/时序/反向推演/断言覆盖四项实证全部通过（A-①~④） | **已修，闭合** |
| N-1 | 冲刺文案 7 轮延续 | 落位点字形级复核（Select:1134/Dashboard:648/注释1123） | 延续（第 8 轮） |
| N-2 | pick 一拍延迟 | 维持 | 延续（第 7 轮） |
| N-3 | 三处格式卫生 | 与工作树逐字节一致 | 延续（第 6 轮） |
| O-1~O-8 | 逐项 | 机制实证相同 | 延续 |
| O-9 | Button dark bg-black + audit 误报 | 本轮实测 audit exit 1（1 处违例确认） | 维持（豁免） |
| O-10 | Dashboard 状态机文案 | 维持，并入 N-1 同源 | 维持 |
| **O-11** | — | **本轮新增**：M-1 修复两轮收敛窗口变宽 + 新稳态假阳性命中路径（安全方向） | 新增 |

---

## 已核对无缺陷的高风险区域清单

- **R63 M-1 修复本体**：targetGuard.ts:64-71 echoed 参数 + 三消费点（Select.tsx:501/594/602/686）+ 回显 effect 置位点（238/295/200）——语义完备、时序正确、无误放行。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1）：M-1 修复后零回归（B 表逐条复证）。
- 后端 `/state`/`/targets` 契约：StateForAccount 按账号过滤全量下发 courses（courses 永驻根因）；rebuildCoursesForAccountLocked 按 targets 重建。
- 401 三形态单广播 + onUnauthorized 幂等 + 会话快照式三连。
- Admin 五 Tab 全量（在飞幂等/配置拒存/删除 Dialog/日志 limit）。
- 倒计时三态顺序、回显/防抖/刷新/卸载生命周期、手动报名/退选双端幂等、Dashboard dateGroups 纯 useMemo、共享 react-query 缓存两路由、saveNow lastJson 去重链。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | exit 0（1948 modules / 417.58 kB js / 41.07 kB css） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | **18/18 全绿**（R63 补 2 条后新基线） |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 视觉护栏 | `node scripts/audit.mjs` | exit 1，1 处违例（Button dark `hover:bg-neutral-900` 裸匹配误报，基线既有，见 O-9） |
| 后端契约核对 | `grep scheduler.go` StateForAccount/rebuildCoursesForAccountLocked | courses 永驻契约确认（M-1 根因复证） |
| 工作树一致性 | `git status --short --branch` | 开局 `## master` / 收尾零输出（build 产物落 ignored web/dist） |
| web/src 零改动 | `git diff HEAD -- web/src/` | 0 行 |
| 临时文件 | `%TEMP%` 与仓库 | 未新建任何临时脚本（全部验证为只读命令）；仓库内仅本报告 |

---

## 教训

1. **「回声参数」的正确组合不是 or 而是「优先级保护」**：shouldDeferSave 的 `stateData===undefined` 首行在 echoed 参数之前——这个顺序不是巧合而是关键（即使调用方误传 echoed=true，首帧保护不破）。断言第 2 条（首帧未到 + 已回显 → 仍推迟）把这个顺序钉死成契约。审查「新增参数」时，除了验「参数在消费点传对」，还要验「参数与既有判据的优先级关系」。
2. **修复闭合的核验要覆盖「修复前缺陷的完整触发链」**：M-1 修复的核验不只验「新稳态放行」，还要反向验「旧暂态（首帧未到/全清空/在飞）不受影响」——本轮对五条时序逐条重推（A-②），确认 echoed 参数是叠加性增强而非替代既有防线。
3. **新稳态会带来新的假阳性命中路径**（O-11）：echoed=true 放行后，「courses 非空恒真」从「阻碍保存」变为「常态伴随」——编辑 PUT 后回显注入自身改动、手动报名后 /state 旧 courses 等路径全部「保守命中」而非「覆盖破坏」。审查稳态解锁时必须同时问「放行后哪些新路径会命中守卫」。
