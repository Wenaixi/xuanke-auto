# R153 前端审查报告（M-1）

基线：7b319be（R152 归档，第八十九轮闭合） | 审查时间：2026-09-24 | 审查代理：R153 前端（只读）
范围：web/ 前端（React 19 + Vite + TS），聚焦清单逐项实测，约定零代码修改（唯一写文件为本报告）。
审查日期上下文：R152 归档为 7b319be（HEAD 即基线，web/ 自基线零提交零差异——第 3 项双空实证与「工作区零漂移」相互印证）。

## 结论前置

无 CRITICAL / HIGH / MEDIUM / MINOR 级别发现。聚焦清单 7 项全部在位（✅），无任何漂移。四条维持观察项依旧维持（Select.tsx:204-207 注释口径残留自 R124 起延续、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选维持候选）。两条新契约纵深（保存链 400ms 防抖窗口与 flush 双闸竞态 / 管理令牌六清除点成对全清 + 收尾二路核对）均无发现。建议 **APPROVE**。

| 级别 | 数量 | 说明 |
|------|------|------|
| CRITICAL | 0 | — |
| HIGH | 0 | — |
| MEDIUM | 0 | — |
| MINOR | 0 | — |
| OBSERVE | 4 | 维持观察（Select 注释口径残留 / Admin refetch 出口 / perf 守卫弱断言 / Button 裸 button 候选），均无回归 |

## 验证表（实测数据）

| 验证项 | 命令 | 结果 | 实测时间 |
|--------|------|------|----------|
| 构建 | `cd web && npm run build`（tsc -b + vite build） | 通过（exit 0），vite v8.3.0，1948 modules，产物 dist/assets/index-27qti0_B.js 420.90 kB / index-1KHlpqcc.css 41.72 kB（与 R152 至 R146 连续七轮同哈希同尺寸——R144 后 web/ 零变更连续佐证再得一轮，产物已嵌入 backend/web/dist） | 2026-09-24 |
| target 守卫 | `cd web && node --import jiti/register scripts/target-guard-check.ts` | 18 断言全绿，末行"target-guard 断言全绿"，exit 0（无 tsx，node_modules 含 jiti，脚本头注释 :8 明确该用法为等价路径） | 2026-09-24 |
| perf 守卫 | `cd web && node --import jiti/register scripts/perf-countdown-guard.ts` | 3 断言全绿，exit 0 | 2026-09-24 |
| admin-auth 守卫 | `cd web && node --import jiti/register scripts/admin-auth-check.ts` | 6 断言全绿，exit 0 | 2026-09-24 |
| unauthorized 守卫 | `cd web && node --import jiti/register scripts/unauthorized-check.ts` | 5 断言全绿，exit 0 | 2026-09-24 |
| 视觉审计 | `cd web && node scripts/audit.mjs` | 全部断言全绿（exit 0），"全部通过，视觉表面协调一致"（74 项） | 2026-09-24 |
| XSS 面 | `grep -rn dangerouslySetInnerHTML web/src/` | 实测 0 行（grep 命中均为 archive/ 审查报告复述，源码零命中） | 2026-09-24 |
| 工作区 | `git status --porcelain --untracked-files=all` | 仅未跟踪 `archive/review-rounds/round153-frontend-findings.md`（本报告）；web/ 相关零改动，跟踪区零漂移 | 2026-09-24 |
| git 双空 | `git rev-parse 7b319be` = HEAD（7b319bebd747cc5415947107726971de61e074ad）+ `git log --oneline 7b319be..HEAD -- web/`（0 行）+ `git diff --stat 7b319be..HEAD -- web/`（0 行） | 双空实证（基线即 HEAD，安全带与三次幂核验） | 2026-09-24 |
| 倒计数字节级 | `git diff 7b319be HEAD -- web/ \| wc -l` | 0 行（字节级空，进一步确证无 index 差异） | 2026-09-24 |
| 守卫调用点 | `grep -c "shouldDeferSave(stateDataRef" web/src/routes/Select.tsx` | 4（:509/:597/:605/:699），与基线 `git show 7b319be:...Select.tsx` 同值确证 | 2026-09-24 |
| 原生 button | `grep -c "<button"` 逐文件 | 17 处 3 文件（Admin 13 / Dashboard 2 / Login 2），Select 0 / Button.tsx 0，零增零减 | 2026-09-24 |

## 聚焦清单逐项裁决

### 1. M-1 保存链守卫闭合（shouldDeferSave） ✅ 在位

**shouldDeferSave 定义**（web/src/lib/targetGuard.ts:64-71）全文读取逐字符核验，判据本体三行与契约逐行一致（:69 `if (stateData === undefined) return true`；:70 `if (echoed) return false`；:71 `return (stateData.courses?.length ?? 0) > 0 && hasSelected`）。

**恰 4 消费点** 逐字符一致（第一参数均为最新 stateDataRef、第二参数均为消费时刻算出的 hasSelected、第三参数均为 echoedRef.current）：
- Select.tsx:509（flushTargets）：`shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)`
- Select.tsx:597（handleBack 入场门，前置 `revRef.current > 0`）：`shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)`
- Select.tsx:605（handleBack 5s 轮询等待 while）：同一行形态
- Select.tsx:699（防抖 400ms 回调）：`shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)`

**echoedRef 三置位无第四处写 true**：赋值形态全仓 grep 确认仅 :200（`= false` 账号切换复位严格置 false 不触第四处）、:240、:297（两处 `= true`）。全 13 处 `echoedRef.current` 引用中其余均为读（:229 回显 effect 短路 / :319 清理 effect 短路 / :507/:594/:603/:696 注释 + 守卫第三参数）。echoDone 置位亦恰 3（:201/:241/:298，与 echoedRef 一一对应）。

**首帧四边界**：:229（`if (echoedRef.current) return`）、:234（`if (stateData === undefined) return` 绝不提前置位回显完成）、:247（`if (pubs.length === 0) return`，effect 声明于 const publishes 之前、TDZ 由 data 自推导）、:319（清理 effect 未回显短路）。逐行核对在位。

**F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，:42 `return changed ? next : selected` 原引用返回契约实测确认（守卫脚本第 18 条断言绿）。随发布重建清理三消费点（回显 effect 内 :290-295 + 独立清理 effect :318-329 + 防抖/flush 消费时刻 :526/:717）在位。

**F43-M1 hasSelected 清空分判**：守卫脚本 8 条 defer 断言全绿——「courses 非空 + 有选中 + 未回显 → 推迟」vs「courses 非空 + 全清空 → 放行」成对确证；第三参数 echoed 两向「已回显稳态 → 放行」「首帧未到 + 已回显 → 仍推迟」成对确证。

**flush/handleBack/防抖三闸双闸等回显**：flush（:509）与防抖回调（:699）同一 shouldDeferSave 入口；handleBack 入场门（:597）+ 5s 等待 while（:605）+ 合并后一帧落地（:609）+ 三轮收敛循环（:611-642）齐备。防抖回调消费时刻双闸（:732 联查空 / :737 发布 id 漂移）与 flush 侧守卫族五层（:515 发布缺席 / :526 stale / :553 联查空 / :558 漂移）全核对。dirtyRef 写点实测仍恰 3（:455 真实失败 / :563/:742 飞行中补发），守卫命中路径绝不置 dirtyRef。防抖 effect 依赖数组（:755 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]`）与 R152 一致。

### 2. OBSERVE-93-01 第四十三轮 ✅ 维持

`grep -c "<button"` 逐文件实测 = 17 处零增零减（Admin.tsx 13 / Dashboard.tsx 2 / Login.tsx 2 / Select.tsx 0 / Button.tsx 0）。651/661 候选行号吻合（:651 硅基流动 Vision / :661 本地 ddddocr 引擎二选，选中态白底黑字）。:577 开关带 `role="switch" + aria-checked + aria-label="激活码机制"`。Dashboard CollapseSection（:67-70 + :64 useId）折叠段与 Login 密码可见性切换（:171-172 aria-label/aria-pressed）语义完整。维持候选名义，无新增裸渲面。

### 3. F93-01 第六十一轮双空实证 ✅ 实证

`git log --oneline 7b319be..HEAD -- web/` 与 `git diff --stat 7b319be..HEAD -- web/` 输出全空；`git rev-parse 7b319be` 与 HEAD 相等、`git diff 7b319be HEAD -- web/` 逐字节 0 行，三重确认基线即归档点、web/ 至本报告前零提交零差异。Select 倒计时候选维持不实现（R125 复核见第 6 项）。

### 4. OBSERVE-116-01 第三十七轮 ✅ 在位

注释口径统一确认：useTickingCountdown.ts:3-8（自 tick 宿主重渲染语义 + memo 叶子承诺）、Dashboard.tsx:206-212（整页 setTick 已删注释）、:229（折叠行实时标签由主矩阵 tick 驱动）、Select.tsx:204-207（倒计时注释）四处同源。自 tick（:13 `setInterval` 1000ms）+ target 变化即校正 now（:19-21）语义未改。

五路轮询契约逐键零漂移（error 与 window_closed 降 30000 全键一致）：
- Select /electives（:58-82）：error→30000（:76）/ window_closed→30000（:80）/ inRange||window_opened→2000（:81）/ 常态→10000（:81）
- Select /state（:148-155）：error→30000（:153）/ window_closed→30000（:154）/ 常态→2000（:154）
- Dashboard /state（:169-174）：error→30000（:170-171）/ window_closed→30000（:172-173）/ 常态→3000（:174）
- Dashboard /logs（:185-190）：error→30000（:186-187）/ window_closed→30000（:188-189）/ 常态→3000（:190）
- Dashboard /electives（:200-204）：恒 30000（快照 TTL 40s > 30s 注释 :196-199 确认）
全部 refetchInterval 源码原文读取，闭包陈旧性语篇（Select :63-81 与 Dashboard :180-183）与 R152 记录一致。

### 5. OBSERVE-115-01 弹窗族 ✅ 与 R152 一致

三处最小语义门行号实测与 R152 一致：Select.tsx:1204（退选确认 `role="dialog"`）、Login.tsx:226（激活码输入）、Admin.tsx:213（删除账号确认）。每处均带 aria-modal="true"（Select:1205 / Login:227 / Admin:214）与 aria-labelledby（exit-modal-title / activate-dialog-title / delete-acct-modal-title）。Esc 关闭三处齐备且带防误关守卫（Select:1208 `!actionLoading.has(...)` / Login:233 `!activating` / Admin:217 `!deleting`）。autoFocus 三处在位（Select:1234 / Login:267 / Admin:241）。

### 6. R125 候选复核 ✅ 维持成立

Select.tsx:850 顶栏横幅条件渲染内 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`（:844-853 单行文本插值，非 memo 叶子）维持站在「路由顶层非叶子」。缓解因子实测无回归：① Dashboard 主矩阵 memo 化（:117 `MemoCountdownMatrix = memo(CountdownMatrix)`，Progress/Tabs 等组件 props 稳定引用核过），② Select 该处单行文本插值 DOM 差分成本可忽略，③ useTickingCountdown 自 tick 语义未改。守卫盲区契约零漂移：perf 守卫第 3 条断言（perf-countdown-guard.ts:34-37）只量 Dashboard.tsx 裸 `cd.*` 行消费、过滤 memo 系列标签，不量条件渲染内 Select:850 文本插值——实测绿（当前 0 处），盲区维持记录不实现。

### 7. 新契约角度（自选 ×2，纵深） ✅ 无发现

**纵深 A：目标保存防抖 400ms 窗口与 flush/handleBack 双闸竞态**

- 防抖出入口（Select.tsx:684-747）：`setTimeout(..., 400)` + cleanup clearTimeout（:747）。重入/覆盖时序核验：新改动在飞行 PUT 期间到达时置 dirtyRef（:741-743），飞行完成 finally :462-465 自接补发——「旧 PUT 后到覆盖新数据」形态被「飞行中补发最新快照」双向拆解，乱序竞态闭合。防抖 callback 内消费时刻全部从 publishesRef（:663-664 `publishesRef.current = publishes` 每次渲染同步）读取，杜绝渲染闭包旧值。hasPublishes 布尔（:662）+ echoDone（:748-751 注释）+ stateData（:752-754 注释）三个 effect 依赖驱动重跑路径核过，无「守卫命中置脏后无解锁信号」死锁残余。
- flush 与 handleBack 竞态：flushTargets 只消费 ref（:491-492），handleBack 等待期间新改动经 revRef 感知（:621-624 flushedRev 比对）、pendingSaving 三信号（:633）+ 21s 超时（:635-639）+ 三轮收敛循环（:611-642）。同 title 去重 toast（Toast.tsx:40-55 自增 id + 同文合并）在失败重试链与双闸并发下不堆叠。无发现。

**纵深 B：管理令牌六清除点成对全清 + 收尾二路核对（isCurrentAdminSession 纯函数 + 账号迁移 effect 对称）**

- isCurrentAdminSession（web/src/lib/adminAuth.ts:6-12）六守卫断言实测全绿（管理一致→恢复 / 撞名学生→学生端 / 吊销→学生端 / 无标记→学生端 / 普通学生→学生端 / 自定义名→恢复）。
- 管理令牌成对清除 `setAdminToken("") + saveAdminToken("")` 全路径清点：logout 登出（App.tsx:121-122）、onDeleted 删管理员自己（:175-176）、onUnauthorized 吊销管理员会话（:213-214）、onBackToStudent 退出管理态（:312-313）——成对四清点全在位（与 R152 记录一致）。写管理令牌侧仅 login 响应带 adminName（:98-99）。inAdmin 与 adminToken 六清除点（:107 置 true / :118/:172/:210/:309 置 false）两侧枚举无遗漏。账号迁移 effect（:135-148）：无账号重置 page、else-if 切 accounts[0]、管理判定失效即 setInAdmin(false) 三支齐备；自定义 adminName 兜底（:294 `key={targetAccount}`、:340 `key={current}`）使 App 两处 Select 挂载点整体重建、账号切换零残留。
- 收尾二路核对：401 设备侧（client.ts:76-95，HTTP 401 前置广播防网关非 JSON 体、旧式 HTTP 200+body 401 仍覆盖，三形态各单次广播不双发）；视觉侧 Progress（Progress.tsx:27 `aria-valuemax={max}` 真实名义与 `aria-valuenow={value}` 同步 Select 名额未公布语义，无细分漂移）+ Tabs 组件（受控 value 兜底、Radix roving focus 未破坏）。无发现。

## 维持观察项（OBSERVE，无回归）

1. **Select.tsx:204-207 注释口径残留**（自 R124 起延续）：注释仍写「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——但 Dashboard 侧已落地拆分（CountdownMatrix memo），Select 顶栏倒计时为单行文本插值、DOM 差分成本可忽略（:850）。注释属「历史承诺未同步更新」而非行为缺陷，与既有架构兼容，维持观察。
2. **Admin refetch 出口（轮询带宽）**：五 Tab 查询轮询（codes 5000 :319 / config 5000 :726 / stats 10000 :820 / accounts 5000 :915 / logs 5000）维持，全部手动 refetch 出口 + invalidateQueries 到位。子 Tab 切换不销毁查询组件实例、轮询后台常驻，观察级带宽成本，无回归。
3. **perf 守卫弱断言**：第 3 条断言（perf-countdown-guard.ts:34-37）只量 Dashboard.tsx 裸 `cd.*` 行消费、完全不量条件渲染内的 cd.* 文本插值（如 Select:850），本轮实测绿再次坐实盲区存在但行为无回归，维持记录。
4. **Button.tsx 裸 button 包装候选维持候选**：Button.tsx 本体 `grep -c "<button"` = 0（React.forwardRef + Slot 抽象输出 button 元素，:31-51 全文读取确认），OBSERVE-93-01 17 处 `<button` 全在消费侧（Admin/Dashboard/Login），语义均完整，非风险项，维持候选名义。

## 建议

**APPROVE**。聚焦清单 7 项全部在位、四守卫脚本全绿（18+3+6+5=32 断言）+ 视觉审计全绿、构建通过（vite v8.3.0，产物与 R152 至 R146 连续七轮同尺寸同哈希——R144 后 web/ 零变更再得一轮证据）、XSS 面源码零命中、web/ 工作区零漂移、F93-01 双空实证（7b319be 基线即 HEAD，web/ 零提交零差异 + 字节级 0 行三重确认）。两条新契约纵深（防抖 400ms 窗口与 flush/handleBack 双闸竞态 / 管理令牌六清除点成对全清 + 收尾二路核对）均无发现。维持观察项不构成合并阻塞，无需本轮修复。