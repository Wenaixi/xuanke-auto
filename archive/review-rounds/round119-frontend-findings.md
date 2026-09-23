# R119 前端只读审查报告

- 审查对象：`web/`（React 18 + Vite + TS + Radix UI + Tailwind）
- HEAD：`71b2a95`（docs(review) R118 收尾；web/ 自 `1351fa4` 起零代码漂移，其后 12 个 commit 全为 docs，`git merge-base 1351fa4 HEAD = 1351fa4` 证实）
- 审查方式：全程只读；grep 逐点实证 + `npm run build`（tsc -b 真校验）+ 断言脚本 + git 锚点双实证
- 聚焦范围：聚焦清单 10 项 + 新契约角度（Dashboard 数据消费与分组链纵深）

---

## 分级发现

**CRITICAL：无**
**MAJOR：无**
**MINOR：无**

## OBSERVE（延续 + 新）

- **OBSERVE-93-01 残余面（第九轮家族册复核，维持不修）**：Admin 7 个带文本裸按钮（:417 收起 / :425 复制 / :441 刷新 / :452 重试 / :651+:661 引擎二选一 / :701 重试）全部带自名文本、原生 button + onClick 键盘 Tab 可达；Toast.tsx:100 Close 纯图标按钮缺口仍纯 display 维度。grep `<button` 全仓清点：Admin 15 / Login 3 / Dashboard 3（含 :67 Collapsible 头按钮，额外核到 aria-expanded + aria-controls + useId 唯一 id）——残余面清单零增零减。**651/661 引擎二选一带强 active 态（border-white bg-white text-black）仍是优先修复面**。OBSERVE-76-03 二勘措辞实证成立：Radix Toast.Close 不自动注入 aria-label，Close 无可访问名缺口属纯读屏维度。
- **OBSERVE-115-01 弹窗族（维持）**：三处裸 div 弹窗最小语义门逐处在位——Select 退选（:1203 role=dialog/aria-modal/aria-labelledby=exit-modal-title + onKeyDown Esc 归 all 课程），Login 激活（:224 同族 + activating 下不响应），Admin 删除（:211 同族）；autoFocus 三处全覆盖（Select :1234 / Login :267 / Admin :241）。无焦点陷阱为注释声明刻意取舍（登录、Admin 均有"完整焦点陷阱迁移到 Radix Dialog 属后续候选"注释）。**Select 退选弹窗仍优先修复面**（完整迁移候选）。零漂移。
- **OBSERVE-116-01 跟踪项第三轮（维持不修）**：useTickingCountdown 每秒 setNow + 轮询双层重渲染。注释已如实口径（"每秒 setNow 触发宿主路由组件整树重渲染…DOM 差分成本可忽略；「只重渲染倒计时一处」需拆 memo 叶子组件，潜在优化，非当前承诺"），与 Select.tsx:204 同款注释、Dashboard.tsx:177 同款——三处口径完全一致，无卡顿证据，活化条件未触发。**轮询链路契约未被动**：/state 3s（Dashboard :140-145 函数式 refetchInterval）、/logs 3s/30s（:156-161）、Select /electives 2s/10s/30s（:69-80）、/electives 恒 30s（Dashboard :174）——B41/F52 钉死的间隔原样在位，无任何改动。
- **OBSERVE-117-02 timer 类型卫生（维持，纯类型卫生非缺陷）**：Select.tsx:393 `retryState.current.timer = null as ReturnType<typeof setTimeout> | null`，tick 形态正确，npm run build 全绿实测通过，无运行期影响。
- **OBSERVE-112-02（维持）**：Dashboard :140 refetchInterval 回调从 `query.state.data` 读取（显式不引用闭包 state，避开循环初始化推断）；/logs 侧 :156-159 读闭包 `state?.window_closed` 降频，注释明确 /state 每 3s 刷新触发重渲染、react-query 用最新闭包重调度，闭包不陈旧——判据成立。Select :80 用 `queryClient.getQueryData` 读 /state 缓存，兼防御 refetchInterval 回调 TDZ（注释明示）。

## 必查项逐条结论（10 项全覆盖）

### 1. M-1 第五十五轮闭合复核 —— 通过
- `shouldDeferSave(` 恰 4 消费点（grep 实证，定义在 targetGuard.ts:64 不计）：Select.tsx:509（flushTargets 首闸）、:597（handleBack 首闸）、:605（handleBack 等待循环）、:699（防抖回调）。四消费点第三参 `echoedRef.current` 逐字符一致（读点 :509/:597/:605/:699 全部 `shouldDeferSave(stateDataRef.current, <第二参>, echoedRef.current)`）。
- echoedRef 置位三路径完整：:200（账号切换复位 false）、:240（/state 首帧 courses 空 → 确证无旧目标，置 true + setEchoDone）、:297（回显合并完成 → 置 true）。
- 首帧不置位边界全在位：:229（echoed 已 true 短路 return）、:234（stateData===undefined return，不置位）、:247（pubs.length===0 return，不置位）、:319（发布清理独立 effect 读 echoedRef 不置位）。TDZ 注释（:244-246、:316-317）与实现一致。
- `echoedRef.current` 运行时读点清点：写 3（200/240/297）+ 读 6（229/319/509/597/605/699），与预期一致，无漏网读点。
- 边界语义复核：F43-M1「全清空与数据缺席分判」在位（shouldDeferSave 三参数签名 = stateData/hasSelected/echoed，targetGuard.ts:64-71 注释完整）；F42-M1 纯数据判据在位（echoed 只在"courses 非空 + 有选中"暂态推迟，稳态放行普通编辑）。
- **`node --import jiti/register scripts/target-guard-check.ts` 18 断言全绿**（含"首帧未到 + 已回显标志 → 仍推迟"边界）。

### 2. OBSERVE-93-01 残余面复核 —— 维持（结论见上）
### 3. F93-01 第二十七轮 —— 通过
- Button.tsx:42 ring 逐字符在位：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，前导注释（global.css 基础重置 outline:none 补偿）在位于 40 行。
- git 双实证：`git log --oneline 1351fa4..HEAD -- web/` 空输出；`git diff --stat 1351fa4 HEAD -- web/` 空——web/ 自锚点起零产品代码漂移（merge-base 确认 1351fa4 是 HEAD 祖先）。
### 4. OBSERVE-115-01 弹窗族复核 —— 通过（结论见上，零漂移）
### 5. OBSERVE-116-01 第三轮复核 —— 通过（结论见上，维持不修 + 轮询契约未被动）
### 6. OBSERVE-117-02 timer 类型卫生 —— 通过（构建绿，纯类型卫生）
### 7. OBSERVE-112-02 复核 —— 通过（query.state 非闭包判据成立）
### 8. 无障碍纵深 —— 通过
- role="alert" 三处全在位：Dashboard :320 / Login :182 / Login :273。
- 同款 ring 令牌三处：Tabs.tsx:29（TabsTrigger）/ Admin.tsx:583（switch）/ Login.tsx:173（密码切换），逐字符 `focus-visible:ring-2 focus-visible:ring-[var(--cyan)]|ring-white/60` 在位。
- 换新角度顺手核到：Dashboard Collapsible header :67 带 aria-expanded + aria-controls={useId}（此前硬编码 collapse-body 的重复 id 问题已根治，注释 :61-63 在位）。
### 9. 新契约角度（Dashboard 数据消费与分组链）—— 通过，无缺陷
- dateGroups（:220-264）依赖 `[courses, electives?.publishes]`：分组键三源 ``(c.begin_date ?? "").slice(0,10) → pubById.get(c.publish_id)?.begin_date → "未知"``，与后端知识位（调度器随目标持久化 publish_name/begin_date，窗口关闭 /state 仍自带）完全闭合——关闭后目标可读性不降级，Dashboard 展示依赖零 /electives 强依赖。
- 日期键同基准：parseDateKey 显式拼 "T00:00:00" 锁本地零点（:113-114，防 UTC 解析 8 小时偏移）+ localTodayMs setHours(0,0,0,0)（:120-123）——同基准无偏移，UTC+8 凌晨 00:00-07:59 不漂移。
- 排序 |日期-今天零点| 升序（距今天最近在前）、"未知"恒排最后（窗口关闭兜底组）；组内 priority 升序、发布 publish_id 升序。
- 折叠种子 null/[] 语义分离（:266-274）：null=未初始化首帧种下最近组、[]=用户主动全折叠绝不重种——种子失效（expandedDates 变更后 effect 守卫 return）判定正确。
- primaryMs/extrasMs（:206-212）：begin_times 稳定引用 + fallback 在 useMemo 内部，依赖数组用 `electives?.begin_times` 引用不抖动。
- cd 倒计时 openTimeStr ?? begin_times[0] 兜底（:191-196）：与 Select :767-772 完全同构，F39-N1「主倒计时吃 begin_times 兜底」双页一致。
- **结论：Dashboard 数据消费与分组链无正确性缺陷。与 window_closed 三态展示（:343-346 / :751）同源后端三判据单源契约一致。**
### 10. 格式卫生快扫 —— 通过
- grep 无 `</span><span`、`</div><div`、兄弟元素同行粘连；缩进级差无半程态。生成区 JSX 全闭合规范。

## 契约 20 检查
- `grep -rnE "第 ?[0-9]{1,3} ?轮|（R ?1[0-9]{2}）|R1[0-9]{2}沉淀|R1[0-9]{2}[ -]"`（routes/components/lib/App/main）——**零残留**。宽泛版 `第[一二三四五六七八九十百0-9]+（第|轮` 亦零命中。轮次历史全部收敛于项目 CLAUDE.md 工程决策手册。

---

## 验证表

| 命令 | 结果 |
|------|------|
| `git log --oneline 1351fa4..HEAD -- web/` | 空（web/ 零漂移实证一） |
| `git diff --stat 1351fa4 HEAD -- web/` | 空（零漂移实证二） |
| `git merge-base 1351fa4 HEAD` | = 1351fa4（锚点确为 HEAD 祖先，12 个 commit 全 docs） |
| `npm run build`（web/，tsc -b + vite） | exit 0，411ms，dist 生成 |
| `node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿 |
| `node scripts/audit.mjs` | 18 项全过（视觉表面协调一致） |
| grep `shouldDeferSave(` | 恰 4 消费点 + 1 定义 |
| grep `echoedRef.current` | 写 3 + 读 6，与预期一致 |
| grep `<button` 全仓 | Admin 15 / Login 3 / Dashboard 3 + Toast Close |
| grep role="alert" / focus-visible:ring | 3 处 alert + 3 处同款 ring 令牌在位 |
| grep 契约 20 轮次前缀 | 零残留 |

## 已核无缺陷清单
- M-1 shouldDeferSave 四消费点第三参逐字符一致；echoedRef 三置位 + 四不置位边界完整
- F93-01 Button.tsx:42 ring + 零漂移双实证
- OBSERVE-93-01 残余面七点位 + Toast Close 键盘链路完整
- OBSERVE-115-01 三弹窗最小语义门 + Esc 关闭 + autoFocus 全覆盖
- OBSERVE-116-01 轮询链路契约（3s/30s/2s/10s）未被任何改动触碰
- Dashboard 分组链（begin_date 三源 / 本地零点 parse / 排序 / 折叠种子）无正确性缺陷
- admin-auth / unauthorized / audit 断言全部绿色

## 结论
- **M-1 第五十五轮：闭合状态确认**。四消费点统一传第三参 echoedRef.current，判据家族（纯数据 + echoed 稳态放行）语义自洽；target-guard 18 断言全绿 + tsc -b 构建通过双重实证。第五十五轮延续零产品改动的纪律闭环。
- **OBSERVE-93-01 残余面：维持**，零增零减；优先修复面仍为 Admin 651/661 引擎二选一（强 active 态裸按钮）。
- **OBSERVE-116-01 跟踪项：第三轮维持不修**；轮询链路契约未被动（本项与此前各轮同判据）。
- **OBSERVE-117-02：维持**，纯类型卫生。
- 无 CRITICAL / MAJOR / MINOR，无契约 20 残留，无格式卫生复发。
- 本轮新契约角度（Dashboard 数据消费与分组链）纵深走查无缺陷，与后端知识位（publish 元数据随目标持久化）完全闭合。