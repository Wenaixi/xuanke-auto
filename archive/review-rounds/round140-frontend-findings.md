# R140 前端只读审查 findings

> 轮次标识：M-1 第七十六轮。审查日期：2026-09-24。工作树基线：`16bb702`（R139 归档）。审查模式：绝对只读（唯一写入为本报告）。
> 审查范围：`web/`（React 19 + Vite + TS 前端）。

## 结论前置

| 分级 | 数量 | 概要 |
|------|------|------|
| CRITICAL | 0 | 无 |
| HIGH | 0 | 无 |
| MEDIUM | 0 | 无 |
| MINOR | 0 | 无 |
| OBSERVE | 3 | 维持观察项（Select 倒计时候选、Admin refetch 出口、perf 守卫弱断言） |

**总评：建议 APPROVE。** 全部聚焦清单逐项实测通过，`web/` 自 `16bb702` 基线双空实证成立，前端零改动，无任何新增缺陷。TDD 实测超预期：`target-guard-check.ts` 断言数从既往记录目视确认为 18（脚本内含 shouldDeferSave 8 条 + selectedHasStalePublish 5 条 + cleanStaleSelected 5 条）。

## 验证表

| 验证项 | 命令/依据 | 结果 | 实测数据 |
|--------|----------|------|----------|
| 工作区状态 | `git status --short --branch` | 通过 | `## master`，无任何改动 |
| 基线确认 | `git rev-parse HEAD` | 通过 | `16bb702faf9686a1b2de767e2f9dd192260e7b0f` |
| F93-01 双空实证 1 | `git log --oneline 16bb702..HEAD -- web/` | 双空成立 | 输出空（exit 0） |
| F93-01 双空实证 2 | `git diff --stat 16bb702..HEAD -- web/` | 双空成立 | 输出空 |
| build | `npm run build`（web 目录，tsc -b + vite） | 通过 | `✓ built in 1.02s`，`1948 modules transformed`，产物 `index.html` / `index-1KHlpqcc.css` / `index-27qti0_B.js`（420.90 kB / gzip 125.53 kB） |
| target-guard-check | `npx tsx web/scripts/target-guard-check.ts` | 全绿 | **18 断言全过**（退出码 0） |
| perf-countdown-guard | `npx tsx web/scripts/perf-countdown-guard.ts` | 全绿 | 3 断言全过 |
| admin-auth-check | `npx tsx web/scripts/admin-auth-check.ts` | 全绿 | 6 断言全过 |
| unauthorized-check | `npx tsx web/scripts/unauthorized-check.ts` | 全绿 | 5 断言全过 |
| XSS 面 | Grep `dangerouslySetInnerHTML` 于 `web/src` | 零命中 | 0 处 |

## 聚焦清单逐项裁决

### 1. M-1 第七十六轮（保存链守卫）—— ✅ 在位

- **shouldDeferSave 定义**——`web/src/lib/targetGuard.ts:64-71` 与 R139 完全一致：判据本体 `stateData === undefined → true`（:69）、`echoed → false`（:70）、`courses 非空 && hasSelected → true`（:71）；第三参 `echoed` 的稳态放行语义（:57-63 注释）与实现逐字符无漂移。函数体 55 行含 8 条注释，全部面述语义与代码一致。
- **恰 4 消费点逐字符一致**——`grep -c "shouldDeferSave(" web/src/routes/Select.tsx` = **4**（`grep` 整词含导入行为 5，以函数调用形态为准）。四个消费点逐一核对，三参数形态 `(stateDataRef.current, 选中数, echoedRef.current)` 统一：
  - `Select.tsx:509` flushTargets 首诊（传入 `latestSelectedCount > 0`）
  - `Select.tsx:597` handleBack 首诊（传入 `hasSelectedNow()` 闭包函数，:579-580 消费时刻读 selectedRef 算）
  - `Select.tsx:605` handleBack 5s 等待循环（50ms 轮询 + 同 hasSelectedNow）
  - `Select.tsx:699` 防抖回调（传入渲染期 `selectedCount > 0`）
- **echoedRef 三置位、无第四处写 true**——`grep "echoedRef.current ="` = 3：`:200`（账号复位守卫 `= false`，复位语义非写 true）、`:240`（courses 空分支 `= true`，确证后端无旧目标）、`:297`（合并完成 `= true`）。:229（回显首闸）、:319（清理 effect 首闸）等只读位均不写值。三置位与 R139 一致。
- **首帧四边界**——:229（echoedRef 短路）、:234（stateData undefined 返回，绝不提前置位）、:247（pubs 空返回）、:319（清理 effect 未回显短路）。四边界与 R139 逐行一致。
- **cleanStaleSelected 原引用返回**——`targetGuard.ts:26-43`，`changed` 为假时返回 `selected` 原引用（:42），无变更不引重渲染；只删"非空且不在集合"的 key（空数组键=清空语义保留，:36-39）。
- **F43-M1 hasSelected 清空分判**——三参判据含 hasSelected 第二参；两条清空路径脚本实测：`deferAssert("首帧未到 + 全清空 → 推迟", ..., true)` 与 `deferAssert("courses 非空 + 全清空 → 放行", ..., false)`，`canSelect For 全绿`。守卫对"首帧未到 + 全清空"仍保守推迟（宁等首帧，防回显 effect 空分支同时置位前覆盖）；"courses 非空 + 全清空"放行 PUT []（清空意图确凿）。用户意图与数据缺席分判零漂移。
- **flush/handleBack/防抖三闸双闸等回显**——三处消费点公式一致：flush（:509）+ handleBack（:597 首诊 / :605 等待循环）+ 防抖（:699），全部消费时刻读 ref 镜像（`:174` selectedRef / `:176` revRef / `:181` stateDataRef 同步置位在上方）。handleBack 5s 等待（:598-610）带 50ms 轮询 + 一帧落地 + 三轮收敛（:611-642）；防抖 effect 依赖含 `echoDone` 与 `stateData`（:755）——/state 数据到达驱动重跑自愈，唯一解锁不求整页刷新。

**TDD 实测**：`npx tsx web/scripts/target-guard-check.ts` **18 断言全绿**（退出码 0），逐条输出含 shouldDeferSave 稳态放行与清空分判全部通过。`npm run build` 通过（tsc -b + vite，1.02s）。

### 2. OBSERVE-93-01（button 面）—— ✅ 在位

实测 `grep -c "<button"` 全仓 **17 处，分布于 3 文件**：

| 文件 | 行号 |
|------|------|
| `web/src/routes/Admin.tsx`（13） | 417、425、441、452、470、473、577、651、661、701、792、898、944 |
| `web/src/routes/Dashboard.tsx`（2） | 67（CollapseSection 语义折叠）、709（/logs 失败重试） |
| `web/src/routes/Login.tsx`（2） | 167（密码可见切换 aria-pressed）、299（激活弹窗取消） |

零增零减，与 R139 一致。**651/661 候选继续维持**（识别引擎切换二选一按钮，语义为 display 级单选态，无自动升级面，见"维持观察项"）。

### 3. F93-01（双空实证）—— ✅ 在位

两项命令均实测输出空、exit 0：
- `git log --oneline 16bb702..HEAD -- web/` → **无任何提交**
- `git diff --stat 16bb702..HEAD -- web/` → **无任何 diff**

`web/` 自 R139 基线零改动，前端全库维持 16bb702 快照。

### 4. OBSERVE-116-01（注释口径 + 五路轮询契约）—— ✅ 在位

- **useTickingCountdown.ts:3-8 与 Dashboard.tsx 三处同源**——hook 顶部注释（自 tick + memo 叶子收敛 + 过期语义）与 Select.tsx:204-207 同款注释、Dashboard.tsx:206-209（删除整页每秒 setTick）、:394-398（memo 叶子注释）四处语境统一，无漂移。
- **五路轮询契约逐键零漂移**——error 与 window_closed 降 30000 端到端成立：

| 路由 | 查询 | error 降频 | window_closed 降频 |
|------|------|-----------|--------------------|
| Select | `/electives`（:58） | 30000（:76） | 30000（:80） |
| Select | `/state`（:148） | 30000（:153） | 30000（:154） |
| Dashboard | `/state`（:169） | 30000 | 30000 |
| Dashboard | `/logs`（:185） | 30000 | 30000（读 `state?.window_closed` 闭包） |
| Dashboard | `/electives`（:203） | 恒 30000（静态） | 恒 30000 |

  Select `/electives` 升频判定读 `query.state.data.publishes` 与复用消息常驻缓存 `query.state.data?.window_closed` 双信号（:76-81），TDZ 防护注释（:72-74）在位；`/state` 每 2s 刷新触发组件重渲染，react-query 用最新闭包重调度轮询间隔（:70-71 澄清注释）。

### 5. OBSERVE-115-01（弹窗族）—— ✅ 在位

三处最小语义门行号与 R139 一致，逐行核对：
- **Select.tsx:1204-1211**——退选二次确认 Modal：`role="dialog" + aria-modal="true" + aria-labelledby="exit-modal-title"`，Esc 关闭带 `actionLoading.has(exitModalClass.id)` 在飞防误关（:1208），取消按钮 `autoFocus`（:1234）。
- **Login.tsx:226-238**——激活码 Modal：`role="dialog" + aria-modal="true" + aria-labelledby="activate-dialog-title"`，Esc 关闭带 `activating` 在飞防误关（:233），激活码输入 `autoFocus`。
- **Admin.tsx:213-218**——删除账号 Modal：`role="dialog" + aria-modal="true" + aria-labelledby="delete-acct-modal-title"`，Esc 关闭带 `deleting` 在飞防误关（:217），取消按钮 `autoFocus`。

三处语义门（role/aria-modal/aria-labelledby + Esc + in-flight 防误关）齐全且对称，R139 后无漂移。

### 6. R125 候选复核（Select:850 内联 cd.*）—— ✅ 维持成立，不实现

- **Select.tsx:844/850 内联 cd.\*（倒计时渲染）**——实测成立：`:844` `cd.isExpired`，`:850` `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒`。选课大厅为**整屏一体化**倒计时（文案 + 数字区块一体，无 Dashboard 四格独立矩阵），拆 memo 叶子收益低（Select.tsx:204-207 注释原语："DOM 差分成本可忽略"）。**维持记录不实现**。
- **缓解因子无回归**——perf-countdown-guard 3 断言全绿（`useTickingCountdown` 仍 setInterval 每秒 setNow 自 tick、Dashboard 存在 memo 化叶子、路由组件顶层无裸 cd.* 消费 = 0 处）。Dashboard 渲染期 cd.* 仅在 `openTimeStr ?? electives?.begin_times?.[0]` 输入（:222-227）与 MemoCountdownMatrix props 传递，整树 770 行 DOM 不因每秒 tick 全量重建。
- **守卫盲区契约零漂移**——Select.tsx:821-826 注释契约在位：window_closed 优先 → window_opened 高亮 → 未识别 → 本地已到点待开，`isExpired` 只是本地"倒计时走到 0"不说明窗口开放/关闭，与 R139 记录完全一致。

### 7. 新契约角度（时间盒自选二）

**① 管理后台会话态流转五路径成对全清（管理令牌标记 `xk_admin_token` 家族）**

选此方向的原因：F52-M1 引入的"登录响应带 adminName 才标记管理令牌 + `isCurrentAdminSession` 判据"是管理态恢复的唯一事实源——若任一退出路径漏清标记，撞名学生/已删管理员/吊销会残留管理态导致五 Tab 连环 403。实测逐路径对账（`web/src/App.tsx` + `lib/adminAuth.ts`）：

- **主动登出 `logout()`**（:112-132）——`setAdminToken("") + saveAdminToken("")`（:121-122），注释明确"残留不影响判定——sessions[admin] 已删，isCurrentAdminSession 恒 false"，标记仍清（成对）。
- **账号迁移 effect**（:147）——`!isCurrentAdminSession(..., adminToken)` 判据命中即 `setInAdmin(false)`。此路径**不显式清标记**，但语义自洽：当前 active 账号已非管理员，判据因"会话 token ≠ 标记"或"sessions[admin] 不存在"恒 false，标记残留不造成复活。R139 注释（:146-147）已说明"判据同渲染"，属判据驱动自动退而非漏清。
- **删除账号 `onDeleted()`**（:160-178）——`acct === adminName` 时 `setInAdmin(false) + setAdminToken("") + saveAdminToken("")`（:172-176），F39-M1 的"删除列表含管理员自身必须退管理态"在位。
- **401 吊销 `onUnauthorized()`**（:188-230）——`lostAccount === adminName` 时同款三连清（:210-215），且先退代理态（:204-206）再退管理态（防卡死在已吊销学生代理页）。
- **返回学生端 `onBackToStudent`**（App.tsx:306-329，Admin prop）——`setInAdmin(false) + setAdminToken("") + saveAdminToken("")`（:309-313，注释明言"不清则 isCurrentAdminSession 仍命中，Admin 继续显示"）；无其他学生账号时完整登出语义（清 sessions + current，防 account-reselect effect 弹回）。

**实测结论**：五路径中四条显式清标记、一条（账号迁移）由判据驱动自动退且残留无害，成对关系完整。F52-M1 的撞名学生普通会话 token 与标记永不匹配（adminToken 由"带 adminName 的签发"唯一注入），无循环死锁路径。admin-auth-check 6 断言全绿（token 一致恢复 / 撞名学生 / 吊销 / 无标记 / 普通学生 / 自定义管理员名）。

**② Dashboard 折叠 `extrasOpen` 键盘路径（aria-expanded / aria-controls / useId）**

选此方向的原因：CollapseSection 是手动折叠段的可访问性契约（无 Radix Collapsible，手写最小实现），aria 关联的正确性依赖 `useId` 唯一性与面板条件渲染的先后顺序。实测（`web/src/routes/Dashboard.tsx:50-88`）：

- **button 携带 `aria-expanded={open}` + `aria-controls={id}`**（:69-70），`useId()`（:64）保证多实例（extras 折叠段 + 每个日期分组）id 唯一，杜绝硬编码 `"collapse-body"` 重复 id 违反 DOM 唯一性——注释记录此前的缺陷与修复语义（:61-63）。
- **面板 `id={id}` 只在 open 时渲染**（:81-84 `open &&`）——关闭态面板不在 DOM（无隐藏内容残留读屏命中），`aria-expanded=false` 与"无面板"语义一致。
- **键盘路径**：button 原生可聚焦，Space/Enter 触发 onClick（:71），零额外 JS；extrasOpen 状态（:244）经 `open={extrasOpen}`（:431）传入，切换即重渲染，无键盘事件缺口。
- **关联核对**：`Logo` 与正文 escape 区零冲突（useId 别处无同 id 硬编码）；折叠段正文在 expand 前不可 Tab 进入（条件渲染），读屏按 aria-controls 引导后展开，契约闭合。

**交叉发现（零漂移）**：`key={account}` 挂载点两处（App.tsx:294 `key={targetAccount}` 代理三、:340 `key={current}` 学生 Select）与 Select.tsx:195-202 accountKey 复位守卫双保险在位——401 吊销切号整体重建语义成立；倒计时 begin_times 兜底（F39-N1）在 Select.tsx:767-772（`openTimeStr ?? begin_times[0]`）与 Dashboard.tsx:222-227 同构，主矩阵全 00 与折叠列表有未来自相矛盾的收口语义无漂移。

## 维持观察项

1. **Select.tsx:204-207 注释口径残留**——自 R124 起延续：注释仍写"如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）"，而 Dashboard 侧已拆 MemoCountdownMatrix 并同步注释口径（其页内标注已从"潜在优化"转"已拆"）。Select 侧为整屏集成式倒计时（收益低、:810-853 一体化），口径残留属上下文差异而非错误，维持记录不实现。
2. **OBSERVE-93-01 refetch 出口候选维持**——Admin 651/661 等 refetch 按钮无自动升级语义；students 端 Dashboard:709 /logs 失败重试为 display 级出口（注释 OBSERVE-107-01 语义），与辅助决策契约 30"打开 admin stats 补发 window_closed"（状态展示层）无耦合，维持观察。
3. **perf-countdown-guard 弱断言属性**：3 断言基于正则结构匹配（自 tick 契约 / memo 叶子存在 / 无裸 cd.* 裸消费），非真实渲染剖面。memo 失效但正则仍通过是理论候选，但 memo 是性能界限的分水岭，结构性事实守卫足以拦截回归，与 R139 判定一致，继续维持。

## 工作区一致性核验

git 报告 `## master` 无任何未跟踪/修改文件；`npm run build`（tsc -b + vite）在 `web/` 内完成，产物落 `backend/web/dist/`（git 忽略，已 `check-ignore` 确认）与基线同轨；四个守卫脚本均在 `web/` 内以 `npx tsx` 运行；`git status --short` 在构建前后均输出空。除本报告外零漂移。

## 结论

**建议 APPROVE。** M-1 第七十六轮保存链守卫（shouldDeferSave 恰 4 消费点逐字符一致 + echoedRef 三置位无第四处 + 首帧四边界 + hasSelected 清空分判）实测成立；F93-01 双空实证（第四十八轮）、OBSERVE-93/116/115 全数在位；R125 候选（Select:850 内联倒计时）维持记录不实现；两个新契约角度（管理令牌五路径成对全清 / Dashboard 折叠键盘路径）均零偏离；四守卫脚本（target-guard 18 / perf-countdown 3 / admin-auth 6 / unauthorized 5）全绿；build + XSS 面零命中。前端自 `16bb702` 基线零改动，无需要修复项。