# R156 前端只读审查发现（M-1 第九十二轮闭合）

> 模式：绝对只读（唯一写文件即本报告）。工作树基线 = 522354a（R155 里程碑归档，M-1 第九十一轮闭合）。审查时间 2026-09-24，全项实测。

## 结论前置

- **CRITICAL**：无
- **HIGH**：无
- **MEDIUM**：无
- **MINOR**：无
- **OBSERVE**：维持观察项 4 条（Select.tsx:204-207 注释口径残留、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选名义）——全部延续此前多轮裁决，均不新增实施动作。

**建议：APPROVE**。聚焦清单 7 项全部逐项实测核验在位零漂移，四守卫脚本（18+3+6+5 断言）与视觉审计 audit.mjs 全绿，`npm run build`（tsc -b + vite）通过，XSS 面零命中，工作区零漂移。无任何需修复项。

## 验证表（实测时间 2026-09-24）

| 验证项 | 命令 | 结果 | 实测数据 |
|--------|------|------|----------|
| 工作树基线 | `git status --short --branch` + `git rev-parse HEAD` | 通过 | 分支 master 干净，HEAD = 522354a（R155 里程碑归档），无工作区改动 |
| F93-01 双空实证 | `git log --oneline 522354a..HEAD -- web/` | 通过 | 输出为空（exit 0），基线后 web/ 零提交 |
| F93-01 双空实证 | `git diff --stat 522354a..HEAD -- web/` | 通过 | 输出为空（exit 0），基线后 web/ 零文件改动 |
| M-1 守卫脚本 1 | `node --import jiti/register scripts/target-guard-check.ts` | 全绿 | 18/18 断言 ✓（8 条 shouldDeferSave + 5 条 stale + 5 条 clean），"target-guard 断言全绿" |
| M-1 守卫脚本 2 | `node --import jiti/register scripts/perf-countdown-guard.ts` | 全绿 | 3/3 断言 ✓（自 tick / memo 叶子 / 路由组件顶层无裸 cd.* 消费），"perf-countdown-guard 断言全绿" |
| M-1 守卫脚本 3 | `node --import jiti/register scripts/admin-auth-check.ts` | 全绿 | 6/6 断言 ✓，"admin-auth 断言全绿" |
| M-1 守卫脚本 4 | `node --import jiti/register scripts/unauthorized-check.ts` | 全绿 | 5/5 断言 ✓，"unauthorized 断言全绿" |
| 视觉审计 | `node scripts/audit.mjs` | 全绿 | A 组 5 项画布背景 + B 组 2 项工具类 + C 组 6 文件 × 5 项黑洞清零全 ✓，"全部通过，视觉表面协调一致"，exit 0 |
| 构建 | `cd web && npm run build` | 通过 | tsc -b + vite build，1948 modules transformed，built in 438ms，exit 0 |
| XSS 面 | `grep -rn "dangerouslySetInnerHTML" web/src/` | 零命中 | 18 个文件各 0（exit 1 无匹配）；`innerHTML`/`eval(`/`document.write` 同查零命中 |
| 工作区漂移 | `git status --porcelain` | 零漂移 | 构建后 0 行改动（web/dist 落 backend/web/dist 且被 gitignore） |

> 注：R155 用 `npx tsx` 跑守卫脚本，本轮实测 `npx tsx` 不在依赖中（package.json devDependencies 无 tsx），改用 node 内置 `--import jiti/register`（jiti 已随 node_modules 存在），输出一致全绿。node v24.4.1。

## 聚焦清单逐项裁决

### 1. M-1 第九十二轮闭合（保存链守卫）— ✅ 在位

- **shouldDeferSave 定义**：`web/src/lib/targetGuard.ts:64-71`。判据本体 :69-71 实测逐字符为：
  - `if (stateData === undefined) return true`（首帧未到无条件推迟）
  - `if (echoed) return false`（已回显稳态放行）
  - `return (stateData.courses?.length ?? 0) > 0 && hasSelected`（courses 非空且有选中才推迟）
  与聚焦清单逐字符一致。
- **恰 4 消费点**：`grep -c "shouldDeferSave(" src/routes/Select.tsx` = **4**（全 src 仅此 4 处）。逐处核验完整三参：
  - :509（flushTargets）`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
  - :597（handleBack 首闸）`revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
  - :605（handleBack 5s 等待 while）`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline`
  - :699（防抖 400ms 回调）`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`
  四消费点均完整传三参（stateData/hasSelected/echoedRef.current），逐字符一致，无第 5 处新增。
- **echoedRef 三置位无第四处写 true**：`grep "echoedRef.current = "` 全文件仅 3 处——:200（账号复位守卫置 false）、:240（回显 courses 空分支置 true）、:297（回显合并完成置 true）。:319 为读（清理 effect 守卫）。
- **首帧四边界全在位**：:229（`if (echoedRef.current) return`）、:234（`if (stateData === undefined) return`，注释：绝不提前置位回显完成）、:247（`if (pubs.length === 0) return`）、:319（清理 effect 守卫 `if (!echoedRef.current || publishes.length === 0) return`）——四边界坐标与聚焦清单逐字符一致。
- **F40-M1 cleanStaleSelected**：`targetGuard.ts:26-43`，原引用返回路径 :42 `return changed ? next : selected`——无变更返回原对象引用（脚本场景 J 实测断言绿）；空 key 保留语义 :36-38（只清"非空且不在集合"）；消费点 :290/:322 两处随发布重建清理 + toast 解锁路径（发布已更新提示）。
- **F43-M1 hasSelected 清空分判**：shouldDeferSave 第二参数 hasSelected + target-guard 脚本场景 M（"courses 非空 + 全清空 → 放行"实测绿）——显式清空与数据缺席分判在位。
- **flush/handleBack/防抖三闸双闸等回显**：:509（flush）、:597/:605（handleBack 首闸 + 5s 等待 while）、:699（防抖回调）三闸全走 shouldDeferSave 纯数据判据，:605 while 循环 5s 兜底；handleBack 双闸（:597 首闸 + :605 等待轮询）等回显完成。依赖补 stateData（防抖 effect 依赖尾部 `stateData` 实测在位）——/state 到达触发 effect 重跑自愈，无 bailout 死锁。
- **守卫脚本**：`target-guard-check.ts` 18/18 断言全绿（实测时间戳见上表）。

### 2. OBSERVE-93-01 第四十六轮 — ✅ 在位（零增零减）

- `grep -rn "<button"` 全 src（除 Button.tsx 内部）实测 **17 处 3 文件**，与 R155 逐行一致：
  - Admin.tsx **13 处**：:417（生成收起）/ :425（本次生成复制）/ :441（全部激活码刷新）/ :452（空态刷新）/ :470（复制 aria-label）/ :473（删除 aria-label）/ :577（role=switch 激活码开关）/ :651（识别引擎 Vision）/ :661（识别引擎 ddddocr）/ :701（配置刷新）/ :792（状态刷新）/ :898（账号刷新）/ :944（日志刷新）
  - Dashboard.tsx **2 处**：:67（CollapseSection 折叠头，aria-expanded/aria-controls/useId）/ :709（日志失败重试）
  - Login.tsx **2 处**：:167（密码可见性切换 aria-pressed）/ :299（激活取消）
- **651/661 候选维持**：Admin.tsx:651/:661 实测为识别引擎切换双按钮（硅基流动 Vision / 本地 ddddocr，带选中态白底黑字样式）。候选事实（裸 button 语义可再强化）维持，无漂移。
- 与 R155 记录逐行核对：17 处 3 文件零增零减，行号全数一致。

### 3. F93-01 第六十四轮 — ✅ 在位（双空实证）

- `git log --oneline 522354a..HEAD -- web/`：输出为空（exit 0）——基线后 web/ 零提交。
- `git diff --stat 522354a..HEAD -- web/`：输出为空（exit 0）——基线后 web/ 零文件改动。
- 双空实证成立，web/ 自 R155 里程碑以来零漂移。git show 522354a 确认该提交仅含 archive/review-rounds/review-round155.md + 双 findings（307 insertions），不触 web/。

### 4. OBSERVE-116-01 第四十轮 — ✅ 在位（注释口径统一）

- **useTickingCountdown.ts:3-8 注释口径**：顶部注释"消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo），宿主因自身 props/state 无变化而快速 bail out，重渲染不会扩散到整树"——与 Dashboard.tsx 三处（:90-93 CountdownMatrix memo 叶子注释"已拆"口径 / :206-211 整页 tick 收敛注释 / :394-398 四格矩阵 memo 注释）同源同口径，零分叉。
- **五路轮询契约逐键零漂移**：
  - Select /electives（:58）：失败/error→30000，window_closed→30000，inRange||window_opened→2000，否则 10000
  - Select /state（:148）：失败/error→30000，window_closed→30000，否则 2000
  - Dashboard /state（:169）：失败/error→30000，window_closed→30000，否则 3000
  - Dashboard /logs（:185）：失败/error→30000，window_closed（读组件闭包 state）→30000，否则 3000
  - Dashboard /electives（:200）：恒 30000
  - **error 与 window_closed 双信号降 30000 逐键零漂移**——五路全部"失败态 30s + 窗口关闭 30s"契约一致，无一路遗漏或漂移。
  - Admin 侧（:319/:726/:820/:915，config 无轮询）属历史观察项不在此契约族。

### 5. OBSERVE-115-01 弹窗族 — ✅ 在位（行号与 R155 一致）

- Select:1204（`role="dialog" aria-modal="true" aria-labelledby="exit-modal-title"` + Escape 守卫，actionLoading 在飞不响应）
- Login:226（`role="dialog" aria-modal="true" aria-labelledby="activate-dialog-title"` + Esc 关闭，activating 中不响应）
- Admin:213（`role="dialog" aria-modal="true" aria-labelledby="delete-acct-modal-title"` + Escape 守卫，deleting 中不响应）
- 三处最小语义门行号与 R155 逐字符一致，零漂移。

### 6. R125 候选复核 — ✅ 维持（不实现）

- **Select.tsx:850 内联 cd.\***：实测 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒` 在路由组件 JSX 内直接渲染（:850），倒计时秒级变化确会触发 Select 整树重渲染——候选事实仍成立。
- **缓解因子无回归**：Select /electives 轮询 2s（inRange/window_opened 升频）+ /state 2s，倒计时 tick 与轮询同拍；cd.* 消费仅 3 处（:844 isExpired 判定 / :850 数字渲染），无新增扩散；使用侧为纯文本 span 无复杂子树，重渲染差分成本低。
- **守卫盲区契约零漂移**：:823-845 状态条注释（`|| cd.isExpired` 移除后 window_closed 优先判"已关闭"、window_opened 才显"已开放"）实测在位，无回归。
- 维持记录不实现——与 R155 裁决一致，Select 侧未拆 memo 叶子（拆分收益被 2s 轮询同拍吸收，属潜在优化非当前承诺）。

### 7. 新契约角度（自选 ×2，时间盒 35 分钟内完成）

**角度一：401 吊销切号整体重建（key={account}）+ 账号复位守卫 — 在位，纵深验证通过**

选此方向的理由：契约 13（决策手册）声明 Select 挂载点必须 `key={account}` + 账号复位守卫（401 被动吊销自动切剩余账号时旧账号 selected 防抖 PUT 整包覆盖新账号目标的风险），是本轮聚焦清单外"跨账号状态隔离"的关键契约点，且 R154 曾抽查、本轮可完整闭环。

实测证据：
- `App.tsx:294`（代理分支 `key={targetAccount}`）+ `App.tsx:340`（学生分支 `key={current}`）双挂载点 key 在位，账号切换即整体重建实例。
- `Select.tsx:190-202`：账号复位守卫——`const [accountKey, setAccountKey] = useState(account)`（:195）+ `if (accountKey !== account)`（:196）同步复位 `setSelected({})` / `setRev(0)` / `echoedRef.current = false` / `setEchoDone(false)`（:197-201），守卫声明于 echoedRef/rev 之后（TDZ 防御，注释 :194）。
- 401 吊销链路：client.ts :64-68 在 r.json() 前广播（网关非 JSON 体兜底）+ :81-88 body 401 分支 HTTP 401 已广播则跳过重复广播（三形态各单次）；App.tsx :187-235 onUnauthorized 按令牌反查归属账号 → 剔除会话 → 管理员自身被吊销同步退管理态/清管理令牌。current 切换 → key 变 → Select 实例重建，旧账号 selected/echoedRef/rev 全复位。
- 结论：双挂载点 key + 账号复位守卫四件套（selected/rev/echoedRef/echoDone）+ 401 剔除链全在位，跨账号目标污染路径闭合。

**角度二：管理令牌五路径成对全清（isCurrentAdminSession 纯函数）— 在位，纵深验证通过**

选此方向的理由：契约 41（F52-M1）声明管理态判定必须绑定"带 adminName 的签发"而非账号名字符串相等，管理令牌标记需 logout/onDeleted/onUnauthorized/onBackToStudent 全路径清除——"五路径成对全清"是防 403 循环死锁的完整闭环，本轮实测清点。

实测证据：
- 写入唯一源：App.tsx:98-99（login 内 `setAdminToken(token)` + `saveAdminToken(token)`，仅"登录响应带 adminName"触发，注释 :95-97）。
- 清除点全量清点 `grep "setAdminToken(" / "saveAdminToken("`：**成对 5 处** = 写入 1 对（:98-99）+ 清除 4 对（logout :121-122 / onDeleted :175-176 / onUnauthorized :213-214 / onBackToStudent :312-313），全部 `setAdminToken("")` 与 `saveAdminToken("")` 成对出现，无单侧遗漏。
- 纯函数判定：`adminAuth.ts:6-12 isCurrentAdminSession(sessions, adminName, adminToken)` = `adminToken !== "" && sessions[adminName] === adminToken`——撞名学生普通会话 token 永远匹配不上标记，刷新不误进管理页。App 三处消费：:147（账号迁移 effect 兜底）/ :217（代理页 401 不误杀管理员）/ :299（渲染判据，与 inAdmin 并）。admin-auth-check 6/6 断言全绿。
- 结论：写入唯一源 + 四清除点全路径成对 + 纯函数判定三要件闭环，无管理令牌残留导致的 403 死锁路径。

## 维持观察项

1. **Select.tsx:204-207 注释口径残留**（R124 起延续）：注释「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——Dashboard 已拆 memo 叶子落地而 Select 未拆，注释与 Dashboard:90-93「已拆」口径不对称（本轮 :207 实测仍为「DOM 差分成本可忽略」口径，未升级为「已拆」）。名义候选维持，无实施动作。
2. **Admin 轮询带宽**：Admin 后台五 Tab 轮询（:319 codes 5000 / :726 stats 5000 / :820 accounts 10000 / :915 logs 5000，config 无轮询）无独立失败降频记录，本轮未重核其与全站失败降频契约的关系，保持历史观察。
3. **perf 守卫弱断言**：perf-countdown-guard 对 Select 侧仅结构性断言（Dashboard 侧 memo 叶子在位、路由组件顶层无裸 cd.* 消费——但 Select 侧顶层正是唯一的 cd.* 消费点，断言不红灯），弱断言性质不变。
4. **Button.tsx 裸 button 候选名义**：`Button.tsx` 中 `const Comp = asChild ? Slot : "button"`（Button.tsx:33）——Button 组件内部裸 button 不在 OBSERVE-93-01 的 17 处统计内（该统计只数路由文件），候选名义维持。

## 结尾

聚焦清单 7 项（M-1 第九十二轮闭合 / OBSERVE-93-01 第四十六轮 / F93-01 第六十四轮 / OBSERVE-116-01 第四十轮 / OBSERVE-115-01 / R125 候选 / 新契约角度 ×2）全部 ✅ 在位零漂移；四守卫脚本 18+3+6+5 断言与视觉审计 audit.mjs 全绿；`npm run build` 通过；XSS 面零命中；工作区零漂移。无 CRITICAL/HIGH/MEDIUM/MINOR 级发现。

**建议：APPROVE。**
