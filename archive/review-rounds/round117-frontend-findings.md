# R117 前端只读审查报告

- 审查对象 HEAD：`dffd608 docs(review): R116 双 findings + 收尾总结……进度 117/256`
- 审查方式：只读（Grep/Read 逐点实证 + 四个脚本/构建实证），零仓库文件修改
- 聚焦范围：M-1 第五十三轮闭合复核 / OBSERVE-93-01 第七轮家族册复核 / F93-01 第二十五轮 / OBSERVE-115-01 弹窗族 / OBSERVE-116-01 双层重渲染跟踪项首复核 / OBSERVE-112-02 refetchInterval 闭包 / 无障碍纵深延续 / 本轮自选角度（数据展示一致性 + Dashboard 分组链）/ 格式卫生快扫

---

## 一、分级发现

### CRITICAL / MAJOR / MINOR
无。

### OBSERVE（延续）
- **OBSERVE-93-01 残余面（第七轮家族册复核）**：维持。7 个带文本裸按钮（Admin :417/:425/:441/:452/:651/:661/:701）+ Toast.tsx:100 Close 逐点位确认——键盘链路完整（全部原生 button 可 Tab/Enter/Space），缺口仍纯 display 维度（无语义/无 aria 标注），非行为缺陷。651/661 引擎二选一带强 active 态（选中白底黑字）仍是优先修复面（按钮无 aria-pressed，读屏无法感知当前引擎）。
- **OBSERVE-76-03 二勘定案措辞**：维持。Toast.tsx:100 `ToastPrimitive.Close` 为 Radix 内建隐式 aria-label（"Close"），属"有标签但无中文可访问名"，非"无自动注入标签"——二勘定案措辞无误，与代码事实相符。
- **OBSERVE-115-01 弹窗族**：维持。三处裸 div 弹窗（Select 退选 / Login 激活 / Admin 删除）最小语义门逐处在位（role=dialog + aria-modal + aria-labelledby + Esc 关闭 + 取消 autoFocus）；无焦点陷阱为注释声明刻意取舍（"完整焦点陷阱迁移到 Radix Dialog 属后续候选"），背景表单可 Tab 穿出属已知留白；Select 退选弹窗仍优先修复面（actionLoading 在飞守卫双按钮同源）。
- **OBSERVE-116-01 双层重渲染**：首次复核维持。useTickingCountdown 每秒 setNow + /state 3s 轮询（Select 2s）双层重渲染，已注释为潜在优化（"DOM 差分成本可忽略；需拆 memo 叶子组件（潜在优化，非当前承诺）"），无卡顿证据、活化条件未触发，不修。「勿动轮询链路契约」未被动：Dashboard /state 与 /logs 3s/30s、Select /state 2s、/electives 30s 升降频与 B41/F52 契约完全一致（见必查项 5）。
- **OBSERVE-112-02 refetchInterval 闭包**：维持。Dashboard.tsx:140 `/state` 与 :156 `/logs` 均用函数式 `refetchInterval: (query) => query.state...`——/state 用 `query.state.data` 判 window_closed 非闭包；/logs 的 `refetchInterval` 依赖闭包 `state` 但 /state 3s 刷新必然重渲染本组件 → 闭包不陈旧（注释 :151-155 逐字对齐实现）。/electives 恒 30s 不依赖任何闭包。

### OBSERVE（本轮新增）
- **OBSERVE-117-01（观察）**：`retryState.current.timer`（Select.tsx:393）用 `ReturnType<typeof setTimeout>` 泛型声明；`setTimeout(() => {...}, delay)` 返回 NodeJS.Timeout 时（tsconfig 含 node 类型或 DOM+Node 混合 lib 时）赋值给该联合类型存在类型张力，当前构建通过说明类型兼容；属类型卫生观察，非缺陷（B116 弹窗族/审计链零漂移延续）。

---

## 二、必查项逐条结论

### 1. M-1 第五十三轮闭合复核 —— 通过
- `shouldDeferSave(` 消费点 grep 恰 **4 处**：Select.tsx:509/:597/:605/:699，全部传第三参 `echoedRef.current` 逐字符一致；另有定义点 `targetGuard.ts:64` 与测试脚本引用（target-guard-check.ts）。
- `echoedRef.current` 运行时读点清点：置位三路径 :240（courses 空置位）/ :297（回显合并完成置位）/ :200（账号切换复位 false，属"置位"族）；边界 :229（首行短路）/ :234（stateData undefined 不置位）/ :247（pubs 空不置位）/ :319（独立清理 effect 首行守卫）；消费 :509/:597/:605/:699。首帧不置位边界完整。
- 判据家族法则（契约 F43-M1）：shouldDeferSave 纯数据判据 + echoed 第三参区分稳态/暂态，与契约一致。
- 验证：`node --import jiti/register scripts/target-guard-check.ts` → 18 断言全绿。

### 2. OBSERVE-93-01 残余面复核（第七轮家族册）—— 维持
- grep `<button` 全仓清点共 18 处（Login 2 / Dashboard 2 / Admin 14），带文本裸按钮残余面清单零增零减（Admin :417/:425/:441/:452/:651/:661/:701 + Toast.tsx:100 = 8 点位，与家族册一致）。
- 651/661 引擎二选一：纯 display 缺 aria-pressed，强 active 态（白底黑字）无读屏反馈——仍是优先修复面。
- OBSERVE-76-03 二勘措辞复核确认（见分级发现）。
- F93-01 修复面 Button.tsx:42 ring 逐字符在位零回归（见必查项 3）。

### 3. F93-01 第二十五轮 —— 通过
- Button.tsx:42 ring 逐字符在位：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，注释说明 global.css 对 button 统一 outline:none、此处补 focus-visible ring 作补偿，语义准确。
- web/ 零代码漂移双实证：
  - `git log --oneline 1351fa4..HEAD -- web/` → **空输出**
  - `git diff --stat 1351fa4 HEAD -- web/` → **空**
- 分支 master，工作树干净。

### 4. OBSERVE-115-01 弹窗族复核 —— 维持
- Select 退选弹窗（:1201-1250）：role=dialog + aria-modal + aria-labelledby="exit-modal-title" + Esc 关闭（`!actionLoading.has(id)` 防误关）+ 取消 autoFocus（R16 契约"手写弹窗「取消」autoFocus"在位）；"确认退选"按钮 onClick 直发，无焦点陷阱为注释声明刻意取舍。
- Login 激活弹窗（:223-316）：role=dialog + aria-modal + aria-labelledby="activate-dialog-title" + Esc 关闭（`!activating` 防误关）+ 激活码 Input autoFocus；取消按钮 :299 与 Esc 同逻辑（清 pendingAccount/pendingTicket/activateError）——F16 契约"与取消按钮同逻辑"一致。
- Admin 删除弹窗（:210-280）：role=dialog + aria-modal + aria-labelledby="delete-acct-modal-title" + Esc（`!deleting`）+ 取消 autoFocus；删除后 onDeleted 处理（见 App.tsx:160 管理自身删号退管理态 F39-M1 契约在位）。
- 零漂移确认。

### 5. OBSERVE-116-01 双层重渲染跟踪项 —— 维持（不修）
- useTickingCountdown.ts:10-15 每秒 setNow + Dashboard/Select 整树重渲染；注释已如实口径（"useState 归属宿主即重渲染宿主，DOM 差分成本可忽略；『只重渲染倒计时一处』需拆 memo 叶子组件（潜在优化，非当前承诺）"），Dashboard.tsx:177-181 同款注释。无卡顿证据、活化条件未触发。
- 「勿动轮询链路契约」未被动（逐点核对）：
  - Dashboard /state :140-145：失败/error→30s；data.window_closed→30s；否则 3s
  - Dashboard /logs :156-161：失败/error→30s；闭包 state.window_closed→30s；否则 3s
  - Select /state :148-155：失败/error→30s；data.window_closed→30s；否则 2s
  - Select /electives :58-82：失败/error→30s；window_closed→30s；inRange||window_opened→2s；否则 10s（B41/F52 契约逐字核对）
  - Dashboard /electives :174：恒 30s
  - 与 B41/F52 升降频契约一致，未被动。

### 6. OBSERVE-112-02 复核 —— 通过
- Dashboard.tsx:140 `/state` 函数式 refetchInterval 用 `query.state`（react-query 官方非闭包通道）判 window_closed；:156 `/logs` 依赖闭包 `state` 但注释 :151-155 明确"logs 降频读组件闭包 state（/state 查询数据）即新鲜——/state 每 3s 刷新数据一变组件重渲染，react-query 用最新闭包重调度"，闭包不陈旧，实现与注释一致。
- Select.tsx:148 同用 `query.state.data?.window_closed`；Select.tsx:58 `/electives` 用 `query.state.error` + `queryClient.getQueryData` 读跨查询状态（TDZ 注释 :72-74 说明不直读顶部 stateData），非闭包。
- 验证通过。

### 7. 无障碍纵深延续 —— 通过
- 三处 role="alert" 在位：Dashboard :320 / Login :182 / Login :273。
- 同款 ring 令牌（focus-visible ring + 无自定义 active 态）在位：Tabs :29 / Admin switch :583（focus-visible:ring-white/60）/ Login 密码切换 :173（focus-visible:ring-white/60）。
- Admin switch :577-591：role=switch + aria-checked + aria-label，键盘可聚焦可开关。
- Dashboard CollapseSection :67-72：aria-expanded + aria-controls + useId（:64）唯一 id。
- 无新发现缺口。

### 8. 新契约角度（本轮自选：数据展示一致性 + Dashboard 分组链纵深）—— 通过
- **window_closed 三态一致性**：Dashboard 顶部徽章 :343-346（window_closed→已关闭 / window_opened→已开放 / 否则待命中）、运行指标 :751（移动端悬浮栏同源三态）、preconnect :343 与 badge 配色（closed 用 outline + text-neutral-500 灰化）三处同源同文案；types.ts:115-117 注释"未下发时 undefined 走待命中，绝不假报关闭"与实现一致（`state?.window_closed` 可选链）。
- **max_count 判据簇一致性**：Select.tsx:1006 isFull（`max_count > 0 && selected_count >= max_count`）、:1009 remaining（max_count=0 恒 0）、:1010 unannounced（`max_count <= 0`）、:1093-1094 文案（名额未公布 vs n/m）、:1102-1103 Progress value/max（unannounced 空条）、:967 筛选（max_count===0 放行）、:977 排序（max_count=0→0 最紧张）——与 CLAUDE.md 决策契约 15（`max_count>0 && selected_count>=max_count`，0=名额未公布，四处同源）全核对一致。fillRate :36-37 `if (!c.max_count) return 0` 同源。后端 IsClassFull 同判据（契约）对齐。
- **Dashboard 分组链**：dateGroups useMemo :220-264——分组键优先 CourseStatus 自带 begin_date（关闭≠元数据丢失契约），兜底 /electives pubById 映射，双缺落"未知"；日期排序（|日期-今日零点| 升序、"未知"恒末）；发布名自带优先、兜底映射、再兜底 `发布 #id`；组内按 priority 升序。折叠种子 null/[] 语义分离（:266-274，绝不重种）。相对倒计时 relativeCountdown 由主矩阵每秒 tick 驱动（:90-91/:198-199 注释与实现一致）。
- **登录/激活表单错误态恢复**：Login 主表单错误 role=alert :182、激活弹窗错误 :273、激活中按钮 disabled 双向（主表 + 弹窗）、取消清错误 :299-306、Esc 同逻辑。无错误态残留。

### 9. 格式卫生快扫 —— 通过
- 相邻 JSX 同行/缩进级差半程态：grep 抽查 Dashboard :333-335/:342-347/:363-369/:394-396/:400-415、Select :1000-1017/:1027-1029、Admin :653-657 无复发；无杂散空行半缩进、无 TSX 同一行开标签紧贴文本。

---

## 三、验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline -1` | `dffd608`（R116 收尾，进度 117/256） |
| `git log --oneline 1351fa4..HEAD -- web/` | 空输出（web/ 零代码漂移实证一） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（实证二） |
| `git status --short --branch` | `## master`（干净） |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 77 断言全绿（含 main.tsx 无 bg-black/70、无整页 bg-black 遮挡画布等） |
| `npm run build`（tsc -b 真校验 + Vite） | ✓ built in ~500ms；产物 index-Bm7TtkV4.js 420.77 kB |

## 四、已核无缺陷清单
- M-1 四消费点第三参逐字符一致、echoedRef 置位/边界全路径、18 断言全绿
- F93-01 ring 逐字符在位 + web/ 零漂移双实证
- OBSERVE-93-01 残余面清单零增零减（18 处 `<button` 全仓清点）
- OBSERVE-76-03 措辞与代码事实相符
- OBSERVE-115-01 三弹窗最小语义门逐处在位、零漂移
- OBSERVE-116-01 注释如实口径、轮询链路契约（B41/F52）逐点未被动
- OBSERVE-112-02 函数式 refetchInterval 非闭包/闭包不陈旧
- 108-01 role=alert 三处 + ring 令牌三处 + switch 语义化在位
- window_closed 三态 / max_count 判据簇 / Dashboard 分组链一致性
- 契约 20 轮次前缀标签：`grep 第 N 轮/（第/R\d+/round\d+` → 零命中

## 五、结论
- **M-1 第五十三轮**：闭合。shouldDeferSave 四消费点 + echoedRef 置位三路径/首帧不置位边界逐字符复核通过，18 断言全绿，零漂移。
- **OBSERVE-93-01 残余面**：维持第七轮结论——缺口纯 display 维度，键盘链路完整；651/661 引擎二选一（缺 aria-pressed）仍优先修复面。
- **OBSERVE-116-01 跟踪项**：首次复核维持"不修"——已注释为潜在优化、无卡顿证据、活化条件未触发；轮询链路契约未被动。
- 本轮分级发现：零 CRITICAL/MAJOR/MINOR；新增 OBSERVE-117-01（retryState timer 类型卫生观察，非缺陷）。
