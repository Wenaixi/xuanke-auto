# R120 前端只读审查报告

## 头部

- **审查对象 HEAD**：`e970b61b52c8b458ec34310fd29ac8a86013b7b1`（docs(review): R119 双 findings + 收尾总结，进度 120/256）
- **审查范围**：`web/`（React 18 + Vite + TS + Radix UI + Tailwind）
- **审查方式**：绝对只读；Read/Grep 逐点实证 + `npm run build`（tsc -b 真校验）+ 三个纯函数断言脚本 + audit.mjs；未修改任何仓库文件（仅写本报告）
- **审查时间**：R120 轮，聚焦清单 10 项逐条走查

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**

连续五十七轮零严重级状态延续。本轮聚焦清单 10 项全部「通过/维持」，无新问题引入。

## 必查项逐条结论

### 1. M-1 第五十六轮闭合复核 —— 通过

- **shouldDeferSave 四消费点逐字符一致**：`:509`（flushTargets）、`:597`（handleBack 首轮）、`:605`（5s 等待循环）、`:699`（防抖回调），四处均为 `shouldDeferSave(stateDataRef.current, <selected布尔>, echoedRef.current)` 三参形态逐字符一致。grep 确认全仓 `shouldDeferSave(` 恰 4 消费点 + 1 定义点（targetGuard.ts:64），零增零减。
- **shouldDeferSave 纯函数逻辑**（targetGuard.ts:64-72）：`stateData===undefined` 无条件推迟 → `echoed` 已回显放行 → 否则 `courses 非空 && 有选中` 才推迟。三判据顺序与契约 35/57 一致（清空语义与数据缺席分判：`hasSelected=false` 放行，首帧未到仍推迟）。
- **echoedRef 置位三路径**：`:200`（账号切换复位）、`:240`（/state 首帧 courses 空 → 置位 + setEchoDone）、`:297`（回显合并完成 → 置位）。首帧不置位边界：`:229`（`if (echoedRef.current) return` 已回显短路）、`:234`（`stateData === undefined` 直接 return，绝不提前置位）、`:247`（pubs 空 return）、`:319`（重建清理 effect 首行 `!echoedRef.current` 短路）。运行时读点 grep 清点 12 处（含注释），与上一轮零漂移。
- **target-guard-check.ts 18 断言全绿**：`node --import jiti/register scripts/target-guard-check.ts`，尾行「target-guard 断言全绿」，覆盖 shouldDeferSave 8 断言（首帧未到/清空分发/回显稳态/首帧+已回显仍推迟）+ selectedHasStalePublish 6 + cleanStaleSelected 4。

**结论：M-1 第五十六轮闭合通过，四消费点三参形态与置位三路径逐字符在位，18 断言全绿。**

### 2. OBSERVE-93-01 残余面复核（第十轮家族册） —— 维持

- **7 个带文本裸按钮 + Toast Close 逐点位确认**：
  - SourceTab `:417`「收起」/ `:425`「复制」（onCopy）/ `:441`「刷新」（codesQuery.refetch）/ `:452`「重试」（refetch）——均为内联 `onClick` 原生 `type="button"`，键盘可聚焦，无 icon-only 缺 label 问题。
  - ConfigTab 引擎二选一 `:651`（硅基流动 Vision）/ `:661`（本地 ddddocr）——**强 active 态在位**（`engine===... ? "border-white bg-white text-black font-medium" : ...`），仍是优先修复面（无 `aria-pressed`，active 态仅视觉表达）。
  - ConfigTab `:701`「重试」（configQuery.refetch）。
  - Toast.tsx:100 Close —— Radix `ToastPrimitive.Close`，自带 `focus:outline-none` + 裸标签无 focus ring（缺口仍纯 display 维度，Radix 基层按钮键盘可达）。
- **grep `<button` 全仓清点：17 处，分布仅 3 文件**（Admin.tsx / Dashboard.tsx / Login.tsx），残余面清单零增零减。
- **OBSERVE-76-03 二勘定案措辞复核**：本仓为纯展示侧「Text 裸按钮（键盘可聚焦、读屏有文本标签）」与「icon-only 无 aria-label」两族；全仓 icon-only 按钮均有 aria-label（Admin:470 复制 `aria-label="复制"`、417/425/441/452/701 文本按钮天然可读；Login:170-173 密码可见性切换按钮 `aria-label` + `aria-pressed` 双语义化）——二勘定案「缺口纯 display 维度、部分点位有强 active 态值得优先迁移」维持成立。
- **F93-01 修复面 Button.tsx:42 ring**：`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"` 逐字符在位零回归；同款 ring 令牌扩散确认（Tabs.tsx:29 / Admin switch :583 / Login 密码切换 :173 个别用 white/60，设计归属是「核心交互 vs 弱交互」区分，已有轮次裁决）。

**结论：OBSERVE-93-01 残余面清单零增零减，引擎二选一 651/661 仍为优先修复面（维持观察），按钮家族 ring 底座在位。**

### 3. F93-01 第二十八轮 —— 通过

- `git log --oneline 1351fa4..HEAD -- web/`：**空输出**。
- `git diff --stat 1351fa4 HEAD -- web/`：**空**。
- Button.tsx:42 focus-visible ring 逐字符在位（见第 2 项证据）。

**结论：web/ 自 1351fa4 起零代码漂移双实证成立，F93-01 修复面连续第二十八轮无回归。**

### 4. OBSERVE-115-01 弹窗族复核 —— 通过

三处裸 div 弹窗逐处核对：

| 弹窗 | role=dialog | aria-modal | aria-labelledby | Esc 关闭 | autoFocus | 焦点陷阱声明 |
|------|------------|------------|-----------------|---------|-----------|--------------|
| Select 退选 `:1201-1250` | `:1204` | `:1205` (true) | `exit-modal-title` | `:1207-1211`（退选中禁用） | 取消按钮 `:1234` | `:1199-1200` 注释 |
| Login 激活 `:223-240+` | `:226` | `:227` (true) | `activate-dialog-title` | `:232-238`（激活中禁用） | 输入框 `autoFocus`(267) | `:220-222` 注释 |
| Admin 删除 `:210-218+` | `:213` | `:214` (true) | `delete-acct-modal-title` | `:216-218`（删除中禁用） | 取消按钮 `:241` | `:208-209` 注释 |

- 三处最小语义门全部在位；无焦点陷阱均为注释声明刻意取舍（Login 注释明确「完整焦点陷阱迁移到 Radix Dialog 属后续候选」）。
- Select 退选弹窗仍为优先修复面（唯一带 Esc 三重语义门 + `aria-labelledby` 最完整，与 Login/Admin 同构，已非缺陷仅是未迁移 Radix）——维持观察。

**结论：OBSERVE-115-01 弹窗族零漂移，最小语义门与 Esc 关闭三处齐整。**

### 5. OBSERVE-116-01 跟踪项第四轮复核 —— 维持（不修）

- useTickingCountdown（lib）自愿内注释如实口径：`每秒 setNow 实际触发宿主路由组件整树重渲染（React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件（潜在优化，非当前承诺——已按实现如实口径）》`——潜在优化注释在位。
- 无卡顿证据：Dashboard/Select 轮询驱动的是 3s/2s 查询级刷新，每秒 setNow 由 React 差分承担；两条路由组件树规模可控（Dashboard 卡片数 = 目标数，Select 课≤82 门）。
- 活化条件未触发：无新增性能指标/用户感知卡顿报告，维持不修。
- 「勿动轮询链路契约」确认未被动：`/state` Dashboard 3s/30s（升降频 window_closed 与 error 态）、Select 2s/30s；`/electives` Select 2s/10s/30s（inRange/window_opened 升频、window_closed/error 降频，B41/F52 钉死）——全站轮询节奏与上轮逐字符一致。

**结论：OBSERVE-116-01 维持，潜在优化注释在位，轮询链路契约零漂移。**

### 6. OBSERVE-117-02 timer 类型卫生观察 —— 维持

- Select.tsx:393：`const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })`——timer 字段类型化到位，`resetRetry`（`:409-413`）与 scheduleRetry（`:420-429`）配对清零/判空中均检查 `retryState.current.timer` 非空才 clearTimeout；卸载 cleanup（`:404`）同步清退避 timer。
- `npm run build`（tsc -b）通过——纯类型卫生非缺陷确认。

**结论：OBSERVE-117-02 维持，纯类型卫生观察成立。**

### 7. OBSERVE-112-02 复核 —— 通过

- Dashboard.tsx:140 `/state` refetchInterval 函数式回调：分支 `query.state.error || query.state.status === "error" ? 30000 : query.state.data?.window_closed ? 30000 : 3000`——`query.state` 参数由 react-query 每次重调注入（非闭包捕获），无陈旧风险。
- Dashboard.tsx:148 `/logs` 回调闭包读 `state?.window_closed`——注释（`:151-153`）「/state 每 3s 刷新数据一变组件重渲染，react-query 用最新闭包重调度」正是对本观察的既有承接：/state 3s 刷新必触发组件重渲染 → logs 闭包每秒调度一拍必然是新鲜闭包 → 闭包不陈旧成立。
- Select.tsx:148 同款函数式 refetchInterval 于 `/state` 2s/30s 已验证（第 5 项轮询链路）。

**结论：OBSERVE-112-02 通过，query.state 非闭包 + 重渲染驱动闭包新鲜双链条实证在位。**

### 8. 无障碍纵深延续 —— 通过

- **108-01 三处 role="alert"**：Dashboard:320（会话失效提示条，isSessionError 仅业务码 401 才显）+ Login:182（登录错误）+ Login:273（激活错误）——grep 全仓恰 3 处，逐字在位，零增零减。
- **同款 ring 令牌族**：Button.tsx:42（cyan ring）+ Tabs.tsx:29（cyan ring 对齐）+ Admin:583（white/60 switch）+ Login:173（white/60 密码切换）——四个使用点逐字核对在位。
- Dashboard CollapseSection（`:50-100` 一带）为可选新角度收获：`aria-expanded`/`aria-controls` + `useId` 已落位（注释明确「此前硬编码 collapse-body 悬空 + 多实例重复 id」的修复），手写折叠段零依赖实现的有障碍基线完整。

### 9. 新契约角度（本轮自选）—— 数据展示一致性对照

选「window_closed 三态 / max_count 判据簇」做纵深走查（候选清单内）：

- **window_closed 三态同源**：
  - Dashboard:346 `{state?.window_closed ? "窗口已关闭" : state?.window_opened ? "窗口已开放" : "待命中"}`；
  - Dashboard:751 `window_closed ? "窗口已关闭" : window_opened ? "窗口开放中" : "系统待命中"`；
  - Admin:740 `s.window_closed ? "已关闭" : s.window_opened ? "已开放" : "待命中"`（后端 /admin/stats 补发，契约 B39-05）；
  - Select:829 `window_closed ? "已关闭" : window_opened ? "已开放" : "待命中"`。
  四处文案措辞微异但三态逻辑一致（同一 windowClosedLocked 单源），判据序 window_closed ≥ window_opened ≥ 待命中，与 B39-05/契约 2 对齐，无情态错位。
- **max_count 判据簇同源**（契约 15「max_count>0 && selected_count>=max_count」）：
  - Select:36 `if (!c.max_count) return 0`（进度率 0 保护）；
  - Select:967 `!onlyAvailable || c.max_count === 0 || c.selected_count < c.max_count`（筛选）；
  - Select:977 `c.max_count > 0 ? c.max_count - c.selected_count : 0`（排序）；
  - Select:1006 `const isFull = c.max_count > 0 && c.selected_count >= c.max_count`（徽章/进度色主判据）；
  - Select:1009 `remaining = c.max_count > 0 ? max(0, max_count-selected_count) : 0`；
  - Select:1103 `max={unannounced ? 1 : c.max_count}`（Progress 分子分母防除零）。
  六处 `max_count>0` 防护齐整，`0=名额未公布`语义四处同源，未发现自相矛盾。Dashboard 侧 isFullFallback（status failed + result 含「已满员」）与 Select 的课程级 isFull 判据分属快照/实时两条路径（契约 14「真满员主路径为快照 classFullInSnapshot」），口径对齐。

**结论：数据展示一致性对照无异常发现，window_closed 三态 + max_count 判据簇四处/六处全部同源无漂移。**

### 10. 格式卫生快扫 —— 通过

- 全仓 grep 模板字符串/相邻 JSX 无半程态残留；逐段 Read 覆盖 Select/Admin/Dashboard/Login 主要渲染区，缩进级差一致（4 空格 + JSX 属性换行规整）；`(next[c.publish_id] ??= []).push(...)` 行首分号逃逸手法（Select:275）合法且带空行前置，无违和。
- 无 TODO/FIXME/XXX/HACK 残留（grep 产物为空）。

## 契约 20 检查（轮次前缀标签）

grep `（第 N 轮）` / 第 N 轮 / 轮次形态：**全仓零命中**。全部注释为「为什么/契约/陷阱」本身口径（本轮通读 Select.tsx 全文注释 + Dashboard 关键注释 + targetGuard.ts 注释确认）。

## 验证表

| 命令 | 结果 |
|------|------|
| `npm run build`（tsc -b + vite） | 通过，1948 modules，产物 index-1KHlpqcc.css / index-Bm7TtkV4.js |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全部通过，视觉表面协调一致 |
| `git log --oneline 1351fa4..HEAD -- web/` | 空输出（零漂移实证一） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证二） |
| grep `shouldDeferSave(` / `echoedRef.current` / `<button` / `role="alert"` / `focus-visible:ring` / 轮次标签 / window_closed / max_count | 逐点比对确认，零增零减 |

## 已核无缺陷清单

- M-1 五十六轮：shouldDeferSave 四消费点三参形态逐字符在位 + echoedRef 置位三路径/首帧边界完整 + 18 断言全绿
- OBSERVE-93-01：7+1 裸按钮键盘链路完整、缺口纯 display、引擎二选一强 active 态在位（维持优先修复面）
- F93-01：Button ring 逐字符在位 + web/ 零代码漂移双实证
- OBSERVE-115-01：三弹窗最小语义门 + Esc 关闭 + autoFocus 逐处在位
- OBSERVE-116-01：潜在优化注释在位、无卡顿证据、轮询契约零漂移
- OBSERVE-117-02：retryState timer 类型卫生非缺陷、构建通过
- OBSERVE-112-02：query.state 非闭包 + 重渲染闭包新鲜双链条在位
- 108-01 role="alert" 三处 + ring 令牌族四使用点逐字在位
- 契约 20：轮次前缀标签全仓零命中
- 格式卫生：零半程态、零 TODO 残留

## 结论

R120 前端只读审查完成，聚焦清单 10 项全部「通过/维持」。关键复核结论：

- **M-1 第五十六轮闭合通过**——shouldDeferSave 四消费点三参形态逐字符一致，echoedRef 置位三路径（账号复位/空首帧/合并完成）与首帧不置位边界（undefined return / pubs 空 return / 已回显短路）完整，18 断言全绿，第六十二轮零漂移延续。
- **OBSERVE-93-01 残余面维持**——裸按钮清单零增零减（恰好 17 处、3 文件），缺口仍纯 display 维度；引擎二选一 651/661 带强 active 态仍是优先修复面；Button.tsx:42 ring 与同款令牌族逐字在位。
- **OBSERVE-116-01 跟踪项维持**——每秒 setNow 潜在优化注释如实口径在位、无卡顿证据、活化条件未触发，「勿动轮询链路契约」（/state /logs 3s/30s、Select /electives 2s/10s/30s）未被动，第四轮维持不修。
- **OBSERVE-117-02 维持**——timer 类型卫生纯观察，构建通过非缺陷。

连续五十七轮零严重级状态延续，web/ 自 1351fa4 起零代码漂移（R120 双实证），稳定期审查纪律未见松动。