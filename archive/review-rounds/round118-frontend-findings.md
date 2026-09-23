# R118 前端只读审查报告

## 头部

- 审查对象：`web/`（React 18 + Vite + TS + Radix UI + Tailwind），HEAD = `7abb197`（R117 收官）
- 审查方式：Read/Grep 逐点实证 + 运行全部前端验证脚本 + `npm run build`（tsc -b 真校验）+ git 零漂移双实证
- 聚焦范围：M-1 第五十四轮闭合复核、OBSERVE-93-01 残余面第八轮家族册、F93-01 第二十六轮、OBSERVE-115-01 弹窗族、OBSERVE-116-01 跟踪项第二轮、OBSERVE-117-02 类型卫生、OBSERVE-112-02 闭包复核、无障碍纵深、登录/激活表单错误态纵深、格式卫生快扫、契约 20 残留扫描

## 分级发现

**CRITICAL 0 / MAJOR 0 / MINOR 0**——连续第五十五轮零严重级（稳定期纪律产物）。

## OBSERVE（延续，全部维持不修）

| 编号 | 结论 | 证据 |
|------|------|------|
| OBSERVE-93-01 | 维持。7 个带文本裸按钮在册零增零减，键盘链路完整（原生 button 天然 Tab 可达），缺口纯 display 维度（global.css:173 `outline:none` 统一抹掉原生 focus，裸按钮无 ring 补偿 → 焦点环失显，不阻塞键盘操作）。651/661 引擎二选一带强 active 态仍为优先修复面 | Admin.tsx:417/:425/:441/:452/:651/:661/:701；icon 按钮 470/473 已带 title+aria-label；works 全部 refetch 文本按钮 | 
| OBSERVE-76-03 | 二勘定案措辞复核确认：Toast.tsx:100 `ToastPrimitive.Close` 无自动注入标签、无可访问名（仅 `focus:outline-none`），display 维度，维持纯记录 | 逐字符核对 Toast.tsx:100 |
| OBSERVE-115-01 | 零漂移维持。三处裸 div 弹窗（Select 退选 / Login 激活 / Admin 删除）最小语义门全在位：`role="dialog" + aria-modal="true" + aria-labelledby` + `onKeyDown Esc`（激活/退选中防误关）+ autoFocus 取消。无焦点陷阱为每处注释声明刻意取舍（"完整焦点陷阱迁移到 Radix Dialog 属后续候选"）。Select 退选弹窗（最常触发）仍优先修复面 | Select.tsx:1201-1250 / Login.tsx:223-316 / Admin.tsx:210-250 |
| OBSERVE-116-01 | 第二轮复核维持不修。useTickingCountdown（lib/useTickingCountdown.ts:4-7）与 Select.tsx:204-207 / Dashboard.tsx:177-181 均已如实注释"每秒 setNow 触发宿主整树重渲染（潜在优化，非当前承诺）"。无卡顿证据，活化条件未触发。「勿动轮询链路契约」（/state 2s / 窗口关降 30s / 失败降 30s）未被碰（零漂移实证兜底） | `git diff 1351fa4 HEAD -- web/` 空 |
| OBSERVE-117-02 | 复核确认纯类型卫生非缺陷：retryState.current.timer（Select.tsx:393）`ReturnType<typeof setTimeout>` 声明，`clearTimeout` 消费类型匹配，tsc -b 通过 | Select.tsx:393 + npm run build 绿 |
| OBSERVE-112-02 | 复核确认无陈旧闭包：Dashboard.tsx:140-146 /state 轮询用 `query.state`（非闭包，函数式参数自 react-query 实时注入），失败态降 30s；:156-162 /logs 轮询读组件闭包 `state?.window_closed`，注释已澄明 "3s 刷新触发重渲染 → react-query 用最新闭包重调度，闭包永不陈旧"——函数式 refetchInterval 每次渲染重求值，无闭包停旧值 | Dashboard.tsx:131-162 |

**新增 OBSERVE：无。** 新契约角度纵深走查（登录/激活表单全链路错误态与恢复）未发现缺口。

## 必查项逐条结论（10/10 通过）

1. **M-1 第五十四轮闭合复核**：**通过**。`shouldDeferSave(` 恰 4 处消费点（:509 flush / :597 handleBack 条件 / :605 handleBack while / :699 防抖回调），grep 全仓命中除定义与脚本断言外零冗余。四消费点第三参逐字符核对全为 `echoedRef.current`，第二参各自形态正确（flush=latestSelectedCount>0、handleBack=hasSelectedNow()、防抖=selectedCount>0，均为消费时刻 ref 镜像或渲染期最近值）。condition 守卫第二参 hasSelectedNow() 首次核对确认。echoedRef 置位三路径在册：:200（账号切换复位 false）/ :240（courses 空首帧置 true）/ :297（非空合并后置 true）；首帧不置位边界全在位：:229（已回显 return）/ :234（stateData undefined return）/ :247（pubs 空 return）/ :319（清理 effect 未回显 return）。`echoedRef.current` 运行时读点：:200（写）、:229、:240（写）、:297（写）、:319、:509、:597、:605、:699——写 3 点读 6 点，与三路径/四消费点断言完全吻合。**target-guard-check.ts 18 断言全绿**（8 shouldDeferSave + 5 selectedHasStalePublish + 5 cleanStaleSelected，含"首帧未到+已回显→仍推迟"边界）。
2. **OBSERVE-93-01 残余面第八轮**：**通过**。grep `<button` 全仓清点（Admin 13 / Login 2 / Dashboard 2 / Select 0 处裸 button）——残余面清单零增零减，见上表证据。
3. **F93-01 第二十六轮**：**通过**。Button.tsx:42 `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]` 逐字符在位（注释 39-41 行声明的"全站 Button 一次收敛（无障碍基线）"契约维持）；`git log --oneline 1351fa4..HEAD -- web/` 空 + `git diff --stat 1351fa4 HEAD -- web/` 空——web/ 自 R108 修复后零代码漂移双实证（本轮为第七轮双实证）；build 产物 index-1KHlpqcc.css 41.72 kB / index-Bm7TtkV4.js 420.77 kB 与 R109-R111 逐字节一致。
4. **OBSERVE-115-01 弹窗族**：**通过**。见上表。
5. **OBSERVE-116-01 第二轮**：**通过**。见上表。
6. **OBSERVE-117-02**：**通过**。见上表。
7. **OBSERVE-112-02**：**通过**。见上表。
8. **无障碍纵深**：**通过**。三处 `role="alert"`（Dashboard.tsx:320 / Login.tsx:182 / Login.tsx:273）在位，与 1351fa4 修复逐字符一致；同款 ring 令牌三处核对完成：Tabs.tsx:29（`ring-[var(--cyan)]` 系列 + offset-[var(--bg)]）、Admin switch :583（`focus-visible:ring-2 focus-visible:ring-white/60` + button role=switch aria-checked 语义化注释 :575-576）、Login 密码切换 :173（同款 `ring-2 ring-white/60` + aria-label/aria-pressed :171-172）。共 8 处关键交互点（Dashboard 折叠 :67 aria-expanded/aria-controls=useId；Select 手写弹窗 autoFocus；Admin/Login 弹窗 Esc）全部在位。
9. **新契约角度（本轮自选：登录/激活表单全链路错误态与恢复）**：**通过，无新缺陷**。逐点核对 Login.tsx 全链路：submit 入口 `if (loading) return` 幂等短路（:36）；输入校验（:37-40）；1001 分支捕获并留存票据 `pendingTicket`（:51-55）；activate 幂等短路（:68）；票据已消费引导分支（/激活票据无效或已过期/ → 清票 + 明确文案"取消后重新登录即可进入"，:87-90）；激活错误独立态 `activateError` 绝不污染主表单 error（:27-29 声明 + :56-58 分离赋值）；取消激活三连清（pendingAccount/pendingTicket/activateError，:302-305）+ Esc 同逻辑（:233-237）；loading/activating 双 disabled 兜底。表单错误态与恢复链路完整无缺口。
10. **格式卫生快扫**：**通过**。audit.mjs 视觉表面检查 40 断言全绿（各文件 bg-black 透明度族 + 无整页 bg-black 不遮挡画布）；相邻 JSX 同行/缩进级差半程态未现复发。契约 20 扫描：`grep -rniE "第[0-9]+轮|round[0-9]{2}"` 全 src/ 零命中，注释无轮次前缀标签残留（Dashboard.tsx:697/:703 注释含 "OBSERVE-106-02/OBSERVE-107-01" 为决策契约标识非轮次前缀，属合规规范引用）。

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 1351fa4..HEAD -- web/` | 空输出（web/ 零提交漂移） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（web/ 零字节漂移） |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 40 断言全绿 |
| `npm run build`（tsc -b + vite build） | exit 0，index-1KHlpqcc.css 41.72 kB / index-Bm7TtkV4.js 420.77 kB 与 R109-R111 逐字节一致 |
| `grep -c "<button"` 全仓 | Admin 13 / Login 2 / Dashboard 2 / Select 0，残余面清单无漂移 |
| 契约 20 轮次前缀扫描 | src/ 零命中 |

## 已核无缺陷清单

- M-1 shouldDeferSave 契约族（四消费点第三参 + echoedRef 置位三路径 + 首帧边界）：零漂移
- useTickingCountdown 双层重渲染注释口径：已如实（"潜在优化，非当前承诺"）
- api/client.ts 401 前置广播 + body 双形态单次广播去重（:64-96）：在位
- App.tsx 401 反查归属 / 管理员代理存活守卫 / onBackToStudent 无学生账号完整登出语义：在位
- Date 分组链（parseDateKey 本地零点、localTodayMs、日期距今天排序、未知组恒排最后）：在位
- Admin 配置表单 `!loaded` 保存锁（防初始空值覆盖生效配置）：在位
- Button.tsx asChild/Slot 转发 + ring 基线：在位
- 弹窗 Esc 在飞防误关（actionLoading/deleting/activating 守卫）：三处齐备

## 结论

R118 前端零严重级延续（连续第五十五轮）。**M-1 第五十四轮闭合**：shouldDeferSave 四消费点第三参 `echoedRef.current` 逐字符全传、echoedRef 置位三路径与首帧不置位四边界完整、18 断言全绿。**OBSERVE-93-01 残余面第八轮**：7 个带文本裸按钮清单零增零减、缺口确认仍纯 display（焦点环失显不阻塞键盘链路）、651/661 强 active 态仍为优先修复面。**OBSERVE-116-01 第二轮**：双层重渲染已如实注释、轮询链路零漂移、活化条件未触发，维持不修。**OBSERVE-117-02**：timer 类型卫生确认非缺陷。新契约角度（登录/激活表单错误态与恢复）纵深走查零缺口。本轮零代码改动，web/ 自 1351fa4 起零漂移第七轮双实证，build 产物逐字节稳定。