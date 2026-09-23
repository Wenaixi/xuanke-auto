# R112 前端只读审查报告（round112-frontend-findings.md）

## 头部信息

- **审查对象 HEAD**：`88fcee0`（R111 收尾提交，工作树干净，git status 无未提交改动）
- **审查范围**：`web/` 全前端（React 18 + Vite + TS + Radix UI + Tailwind），四个路由组件 Select.tsx（1254 行）/ Dashboard.tsx（770 行）/ Admin.tsx（954 行）/ Login.tsx（319 行）+ lib/（targetGuard.ts、useTickingCountdown.ts）+ components/ui/
- **审查方式**：逐字符 Read + Grep 实证 + `npm run build`（tsc -b 真校验）后台实跑 + 三个纯函数断言脚本实跑
- **聚焦范围**：M-1 第四十八轮复核、OBSERVE-93-01 残余面复核（第二轮家族册）、F93-01 第二十轮、无障碍纵深延续（108-01/ring 令牌）、新契约角度（登录/激活表单全链路错误态与恢复）、格式卫生快扫

## 分级发现

### CRITICAL：无

### MAJOR：无

### MINOR：无

本轮零 CRITICAL/MAJOR/MINOR——连续五十多轮稳定期纪律的延续，必查项全部通过。

### OBSERVE（延续）

**OBSERVE-93-01（维持，缺口纯 display 维度）**：7 个带文本裸按钮（Admin.tsx :417/:425/:441/:452/:651/:661/:701）逐点位复核确认键盘链路完整——全部是原生 `<button>` 可聚焦可 Enter/Space 激活、文本可读；缺口仍在 display 维度（global.css:174 `outline:none` + 无 focus-visible ring，Tab 聚焦无可见焦点环）。651/661 引擎二选一带强 active 态（`border-white bg-white text-black`）仍是优先修复面。grep `<button` 全仓清点 16 处：Login 2 + Dashboard 2 + Admin 12（7 裸 + 2 图标 aria-label + 1 role=switch + 2 失败态重试），历轮基线 17 处口径含两处图标裸按钮已补 aria-label（:470/:473，a12a09c 修复）后净 15 处裸按钮——本轮 7 带文本裸按钮清单与历轮报告完全一致，零增零减。F93-01 修复面（Button.tsx:42 ring）逐字符在位（`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`）。

**OBSERVE-111-01（前轮观察，本轮确认无新证据）**：MarkDone/RemoveDone 返回值丢弃防回归点属后端面，本轮前端零相关改动，维持待观察。

### OBSERVE（本轮新增）

**OBSERVE-112-01：Toast 区域脱离 aria-live 播报通道**。`src/components/ui/Toast.tsx` 用 Radix `ToastPrimitive.Provider/Root` 渲染全站 toast（目标保存失败/报名成功/发布更新等），Radix Toast 原生带 `aria-live` 播报语义（其 Role=status）。但对比 108-01 修复面——页面内嵌错误条补 `role="alert"` 的目的就是"读屏用户可感知动态错误状态"，而 toast 通道路径下 Radix 默认 viewport 虽已带播报，检查发现组件内所有 `ToastPrimitive.Close` 均只有裸 `focus:outline-none`，无 focus-visible ring 补偿（global.css:174 对 button 统一 outline:none）。这与 F93-01 全站收敛的 ring 令牌不一致，属纯 display 维度（键盘用户 Tab 到关闭钮无可见焦点环）。优先级低于 651/661 引擎二选一（那些带 active 态有颜色反馈，close 无任何视觉焦点提示）。

**OBSERVE-112-02：React-Query 轮询间隔回调的隐式数据源（继续观察）**：Dashboard.tsx:140 `refetchInterval: (query) => query.state.data?.window_closed ? 30000 : 3000` 与 :156 logs 间隔读取组件闭包 `state`（/state 查询数据）来降频——注释已明确"组件重渲染时 react-query 用最新闭包重调度，闭包不陈旧"，与 Select.tsx:77 `queryClient.getQueryData` 直接读缓存的写法不同但行为等价（双查询同 key 共享 react-query 缓存）。读通全链确认无陈旧闭包陷阱（/state 每 3s 刷新必触发重渲染），纯观察不构成缺陷。

## 必查项逐条结论

### 1. M-1 第四十八轮闭合复核 —— 通过

- **shouldDeferSave 四消费点逐字符一致**：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` ✓
  - :597 `shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` ✓
  - :605（while 循环）`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` ✓
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` ✓
  - 四处均传第三参 `echoedRef.current`，参数位序逐字符一致。
- **grep 确认 `shouldDeferSave(` 恰 4 消费点**（Select.tsx 509/597/605/699）+ 定义 1 处（targetGuard.ts:64）+ 断言脚本 7 处，无第五消费处。
- **`echoedRef.current` 运行时读点恰 6 处**：:200（账号切换复位写 false）/ :229（回显 effect 首行短路读）/ :240（courses 空置 true）/ :297（回显完成置 true）/ :319（发布重建独立 effect 首行短路读）/ :509、:597、:605、:699（消费点）。:303/:507/:594/:603/:696 为注释引用。置位三路径 :200/:240/:297 完整；首帧不置位边界 :234（stateData===undefined 提前 return）/ :247（pubs 空 return）/ :229（echoedRef 已真 return）在位。:319 是独立 effect 的读点（属运行读），grep 总命中含 :319，符合历轮口径。
- **`node --import jiti/register scripts/target-guard-check.ts` 实跑：18 断言全绿**（含第三参数四边界：courses 非空+有选中+未回显→推迟 / 已回显→放行 / 全清空→放行 / 首帧未到+已回显→仍推迟）。
- **契约 20 复核**：Select.tsx 逐段走读，历史轮次编号锚点已全部剥离（R92 清理后无复发），grep `第 *N* 轮` 零命中。

### 2. OBSERVE-93-01 残余面复核（第二轮家族册复核）—— 通过/维持

- 7 个带文本裸按钮逐点位确认：键盘链路完整（原生 button 可聚焦可激活文本可读），缺口纯 display 维度（global.css:174 outline:none + 无 focus-visible ring）。
- :651/:661 引擎二选一带强 active 态（选中侧 `border-white bg-white text-black`），仍是优先修复面。
- grep `<button` 全仓 16 处（Login 167/299、Dashboard 67/707、Admin 417/425/441/452/470/473/577/651/661/701/792/898/944），历轮基线 17 处含两图标按钮补 aria-label（a12a09c）后净面一致，零增零减。
- F93-01 修复面（Button.tsx:42 ring 令牌）逐字符在位零回归。

### 3. F93-01 第二十轮 —— 通过

- Button.tsx:42 ring 逐字符在位。
- `git log --oneline 1351fa4..HEAD -- web/` 空输出 + `git diff --stat 1351fa4 HEAD -- web/` 空——1351fa4 之后无任何 web 代码提交，前端零代码漂移确证（后续提交 57bf401/616ab9e/88fcee0 均为 backend/docs 面）。

### 4. 无障碍纵深延续 —— 通过

- **108-01 三处 role="alert" 全在位**：Dashboard.tsx:320 / Login.tsx:182 / Login.tsx:273，逐字符一致。
- **同款 ring 令牌在位**：Tabs.tsx:29、Admin.tsx:583（role=switch）、Login.tsx:173（密码切换），与 Button.tsx:42 同款 `focus-visible:ring-2` 令牌。
- **可选换新角度——表单 label 关联**：Login 账号（htmlFor=login-account）/密码（htmlFor=login-password）/激活码（htmlFor=activation-code）全关联；Admin 生成数量/次数/接口地址/密钥/模型/并发上限六处 `htmlFor` 与 Input id 一一对应（识别引擎无 input 是二选一按钮组，纯 display 缺口已归 OBSERVE-93-01）；Select 搜索框 `aria-label`（:870）；Dashboard CollapseSection `aria-expanded`+`aria-controls`（useId 唯一）+ Progress `role=progressbar`。Tab 顺序由 Radix Tabs 托管 role=tab 原生键盘导航。

### 5. 新契约角度（本轮自选：登录/激活表单全链路错误态与恢复）—— 通过

登录/激活双入口纵深走查：
- **幂等守卫**：submit() 入口 `if (loading) return`（:36）与 activate() 入口 `if (activating) return`（:68）双幂等，杜绝双击并发重复登录/激活。
- **错误态与恢复**：错误与激活错误独立 state（activateError 不污染主表单 error）；票据过期/已用分支（:87-90）给明确引导文案"已开通，重新登录即可直进"；非过期失败同样清 pendingTicket（:95），防滞留旧票无限重试拿"激活票据无效"误导文案；取消激活清 pendingAccount/pendingTicket/activateError（:302-305）。
- **101-01/1001 流程**：e.code===1001 存 pendingAccount + pendingTicket（:51-55），激活成功 onLogin 落 token；后端票据 5 分钟单次契约在前端有完整兜底文案。
- **键盘可达**：激活模态框 role=dialog/aria-modal/aria-labelledby + Esc 关闭（:232-238）+ autoFocus 激活码输入 + 取消按钮；主表单禁用 loading 期间按钮 disabled。
- 结论：登录/激活全链路错误态与恢复闭环，无缺陷。

### 6. 格式卫生快扫 —— 通过

- grep `>\s*</` 空标签同行、`/>\s*<` JSX 相邻同行零命中。
- 超长行存在但为既有多属性 JSX 行（Admin.tsx 表格行 aria-label/th 等 166-241 字符、Select.tsx 卡片 div 等），历轮基线内，无新增半程态（首标签行内换行后属性未缩进对齐）。
- audit.mjs 视觉表面协调检查全绿。

## 验证表（实跑命令 + 结果）

| 命令 | 结果 |
|------|------|
| `npm run build`（web/，tsc -b + vite build） | exit 0，1948 modules，产物落 backend/web/dist |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 全部通过（视觉表面协调） |
| `git log --oneline 1351fa4..HEAD -- web/` | 空（前端零代码漂移确证） |
| grep `<button` / `role="alert"` / `focus-visible:ring` / `shouldDeferSave(` / `echoedRef.current` / `第 N 轮` | 逐项核对，见必查项 |

## 已核无缺陷清单

- 目标保存补发链状态机（saveNow/scheduleRetry/flushTargets/handleBack 三循环收敛 + unmountedRef 复位 + StrictMode 双挂载防护）——Round43 以来维护无回归。
- 回显三 effect（合并/echoedRef 置位/发布重建清理）依赖与守卫判据一致。
- useTickingCountdown 单定时器收敛（Dashboard/Select 双消费，卸载 clearInterval）。
- react-query 轮询降频（/state 与 /logs 失败态 30s / window_closed 30s / 开窗 3s·2s 升频）与 Select 侧 TDZ 防御（refetchInterval 回调读缓存不读组件 const）。
- 满员判定 IsClassFull 同源契约前端四处落位（筛选/排序/徽章/进度条全走 `max_count>0 && selected_count>=max_count`，0=未公布不误判满员）。
- App.tsx 两处 Select 挂载点 `key={targetAccount}` / `key={current}` + Select 账号切换复位守卫（accountKey 兜底）在位。
- 契约 20：代码注释零轮次编号锚点（仅 Dashboard :697/:703 引用 OBSERVE-106-02/107-01 观察编号，属"引用决策来源"非"轮次前缀标签"，历轮口径认可）。

## 结论

M-1 第四十八轮闭合复核**通过**——shouldDeferSave 四消费点逐字符一致（全传 echoedRef.current 第三参）、置位/首帧不置位边界完整、18 断言全绿。OBSERVE-93-01 残余面复核**维持**（缺口仍纯 display 维度、键盘链路完整、7 裸按钮清单零增零减、F93-01 修复面在位）。本轮零 CRITICAL/MAJOR/MINOR，新增 2 条 OBSERVE（Toast 关闭钮无 ring 属 93-01 家族残余、react-query 闭包轮询行为观察）。前端代码 1351fa4 后零漂移，全站处于稳定期收敛态。
