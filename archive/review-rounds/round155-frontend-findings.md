# R155 前端只读审查报告（M-1）

> 审查模式：绝对只读（唯一写文件即本报告）。工作树基线 f1d4b37（R154 归档）。审查时间 2026-09-24。

## 结论前置

- **CRITICAL**：无
- **HIGH**：无
- **MEDIUM**：无
- **MINOR**：无
- **OBSERVE**：维持观察项 4 条（Select.tsx:204-207 注释口径残留、Admin 轮询带宽、perf 守卫弱断言、Button.tsx 裸 button 候选名义）——全部延续此前多轮裁决，均不新增实施动作。

**建议：APPROVE**。聚焦清单 7 项全部逐项实测核验在位，四守卫脚本 + 视觉审计全绿，`npm run build` 通过，XSS 面零命中，工作区零漂移。无任何需修复项。

## 验证表（实测时间 2026-09-24）

| 验证项 | 命令 | 结果 | 实测数据 |
|--------|------|------|----------|
| 工作树基线 | `git status --short --branch` + `git log --oneline -5` | 通过 | 分支 master 干净，HEAD = f1d4b37（R154 归档提交），无工作区改动 |
| F93-01 双空实证 | `git log --oneline f1d4b37..HEAD -- web/` | 通过 | 输出为空（exit 0），基线后 web/ 零提交 |
| F93-01 双空实证 | `git diff --stat f1d4b37..HEAD -- web/` | 通过 | 输出为空（exit 0），基线后 web/ 零文件改动 |
| M-1 守卫脚本 1 | `npx tsx scripts/target-guard-check.ts` | 全绿 | 18/18 断言 ✓（8 条 shouldDeferSave + 5 条 stale + 5 条 clean），"target-guard 断言全绿" |
| M-1 守卫脚本 2 | `npx tsx scripts/perf-countdown-guard.ts` | 全绿 | 3/3 断言 ✓（自 tick / memo 叶子 / 路由组件顶层无裸 cd.* 消费），"perf-countdown-guard 断言全绿" |
| M-1 守卫脚本 3 | `npx tsx scripts/admin-auth-check.ts` | 全绿 | 6/6 断言 ✓，"admin-auth 断言全绿" |
| M-1 守卫脚本 4 | `npx tsx scripts/unauthorized-check.ts` | 全绿 | 5/5 断言 ✓，"unauthorized 断言全绿" |
| 视觉审计 | `node scripts/audit.mjs` | 全绿 | 6 文件 × 5 项黑洞清零 + 对齐断言全 ✓，"全部通过，视觉表面协调一致" |
| 构建 | `cd web && npm run build` | 通过 | tsc -b + vite build，1948 modules transformed，built in 347ms，exit 0 |
| XSS 面 | `grep -rn "dangerouslySetInnerHTML" web/src/` | 零命中 | exit 1（无匹配）；`innerHTML`/`eval(` 同查零命中 |
| 工作区漂移 | `git status --short` | 零漂移 | 构建后 0 行改动（web/dist 落 backend/web/dist 且被忽略） |

## 聚焦清单逐项裁决

### 1. M-1 第九十一轮闭合（保存链守卫）— ✅ 在位

- **shouldDeferSave 定义**：`web/src/lib/targetGuard.ts:64-71`。判据本体 :69-71 实测为：
  - `if (stateData === undefined) return true`（首帧未到无条件推迟）
  - `if (echoed) return false`（已回显稳态放行）
  - `return (stateData.courses?.length ?? 0) > 0 && hasSelected`（courses 非空且有选中才推迟）
  与聚焦清单逐字符一致。
- **恰 4 消费点**：`grep -c "shouldDeferSave(" src/routes/Select.tsx` = **4**（全 src 搜索除 targetGuard.ts 定义外零额外消费点）。逐处核验：
  - `Select.tsx:509`（flushTargets 消费时刻，传 `latestSelectedCount > 0` + `echoedRef.current`）
  - `Select.tsx:597`（handleBack 首轮守卫，传 `hasSelectedNow()`）
  - `Select.tsx:605`（handleBack 5s 等待循环 while 条件，同 hasSelectedNow）
  - `Select.tsx:699`（防抖回调 400ms 消费时刻，传 `selectedCount > 0`）
  四处均为完整三参调用（stateData 经 stateDataRef、hasSelected、echoedRef.current），无第四处裸调用。
- **echoedRef 三置位无第四处写 true**：`:200`（accountKey 守卫复位 false）、`:240`（courses 空首帧置 true）、`:297`（回显合并完成置 true）。`grep "echoedRef.current = "` 仅 3 行，无第四处写 true。`setEchoDone` 与 echoedRef 同步（:201/:241/:298），无分叉。
- **首帧四边界**：`:229`（`if (echoedRef.current) return` 只合并一次）、`:234`（`if (stateData === undefined) return` 首帧未到绝不提前置位）、`:247`（`if (pubs.length === 0) return` 发布缺席不置位）、`:319`（发布重建清理 effect 首行 `if (!echoedRef.current || publishes.length === 0) return` 未回显不清理）——四边界与契约一致。
- **F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，实测空数组 key 保留（:36 `arr.length > 0` 判据）、无变更返回原对象引用（:42 `return changed ? next : selected`），消费点 Select.tsx:290/:322（回显 effect 内 + 独立发布重建 effect，各带 toast 解锁提示）。TDD 场景 F-J（含"无变更 → 返回原引用"恒等断言）全绿。
- **F43-M1 hasSelected 清空分判**：shouldDeferSave 第二参数 hasSelected，脚本 8 条 shouldDeferSave 断言中「首帧未到 + 全清空 → 推迟」「courses 非空 + 全清空 → 放行（PUT []）」两条清空分判实测通过；handleBack 侧 hasSelectedNow 在 :597/:605 两处同参消费。
- **flush/handleBack/防抖三闸双闸等回显**：flush 守卫（:509）+ handleBack 双闸（:597 首轮 + :605 5s 等待循环，50ms 轮询）全部消费 `echoedRef.current` 第三参；防抖 effect 依赖数组含 `echoDone`（:755）与 `stateData`（:752-755 注释：回显完成/数据到达双路驱动重跑自愈）——三闸消费点语义统一为"已回显稳态放行、未回显推迟"。

### 2. OBSERVE-93-01 第四十五轮 — ✅ 在位（零增零减）

- `<button` 全仓实测：Admin.tsx **13** / Dashboard.tsx **2** / Login.tsx **2** = **17 处 3 文件**，与 R154 口径一致，零增零减。
- 651/661 候选维持：Admin.tsx:651/:661 实测为识别引擎切换双按钮（硅基流动 Vision / 本地 ddddocr，带选中态样式），Dashboard.tsx:709 为重试按钮。候选事实（裸 button 语义可再强化）维持，无漂移。

### 3. F93-01 第六十三轮 — ✅ 在位（双空实证）

- `git log --oneline f1d4b37..HEAD -- web/` 输出为空（exit 0）；`git diff --stat f1d4b37..HEAD -- web/` 输出为空（exit 0）。基线后 web/ 无任何提交与文件改动，Select 倒计时渲染内联 cd.*（:850）候选维持不实现。

### 4. OBSERVE-116-01 第三十九轮 — ✅ 在位（注释口径统一 + 五路轮询零漂移）

- **注释口径**：`useTickingCountdown.ts:3-8` 实测「内部自 tick（每秒 setNow）… 每秒 setNow 实际触发宿主路由组件重渲染（React 语义：useState 归属宿主即重渲染宿主）… 消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」；Dashboard.tsx:90-93「宿主路由组件每秒 setNow 引发的重渲染到这里被 React bail out… 只有这 4 个数字文本节点重渲染（注释口径同步：不再声称"只重渲染倒计时一处"需拆分，已拆）」；Dashboard.tsx:206-209「删除整页每秒 setTick——倒计时收敛 useTickingCountdown 自 tick（Dashboard/Select 同实现）… cd.* 已收敛到 MemoCountdownMatrix 叶子」；Select.tsx:204-207「本地每秒刷新倒计时：收敛到 lib/useTickingCountdown 自 tick 组件… DOM 差分成本可忽略」。四处口径同源（含"潜在优化 → 落地实现"演进事实），无矛盾残留。
- **五路轮询契约逐键零漂移**（error 与 window_closed 全部降 30000）：
  - `/state`（Dashboard:171-174）：error→30000，window_closed→30000，否则 3000
  - `/logs`（Dashboard:186-188）：error→30000，state.window_closed→30000，否则 3000
  - `/electives`（Dashboard:203）：恒 30000
  - Select `/state`（Select:153-154）：error→30000，window_closed→30000，否则 2000
  - Select `/electives`（Select:76/:80）：error→30000，window_closed→30000，成功态按 inRange/window_opened 升 2000 否则 10000
  - 无一漂移为其他值。

### 5. OBSERVE-115-01 弹窗族 — ✅ 在位（三处最小语义门行号一致）

- Select:1204（`role="dialog" aria-modal="true" aria-labelledby="exit-modal-title"` + Escape 守卫，actionLoading 在飞不响应）
- Login:226（`role="dialog" aria-modal="true" aria-labelledby="activate-dialog-title"` + Esc 关闭，activating 中不响应）
- Admin:213（`role="dialog" aria-modal="true" aria-labelledby="delete-acct-modal-title"` + Escape 守卫，deleting 中不响应）
- 三处行号与 R154 一致，零漂移。

### 6. R125 候选复核 — ✅ 维持（不实现）

- **Select.tsx:850 内联 cd.\***：实测 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒` 在路由组件 JSX 内直接渲染（:850），倒计时秒级变化确会触发 Select 整树重渲染——候选事实仍成立。
- **缓解因子无回归**：
  - 2s 轮询吸收：Select `/state`+`/electives` 成功态 2s 轮询与每秒 setNow 相互独立，DOM diff 为纯文本节点比对（Select.tsx:204-207 注释「DOM 差分成本可忽略」）。
  - memo 叶子：Dashboard 侧已拆 MemoCountdownMatrix（:117 `const MemoCountdownMatrix = memo(CountdownMatrix)`）落地；Select 侧单行文本内联无列表重建。
  - 守卫盲区契约零漂移：perf-countdown-guard 3 断言全绿（Dashboard 侧 memo 叶子在位、路由组件顶层无裸 cd.* 消费），未扩及 Select 侧，维持名义候选记录。

### 7. 新契约角度（自选 ×2，时间盒 35 分钟内完成）

**角度一：手动操作在飞守卫（actionLoading ReadonlySet 族）— 在位，纵深验证通过**

选此方向的理由：契约 16（决策手册）声明前端手动操作在飞幂等须用 `ReadonlySet<number>` 按课程独立跟踪（单值会被异课程互踩），是本轮聚焦清单外少有的"状态机完整性"契约点，且 Select.tsx 恰好是本轮核心审查文件，顺手可闭环。

实测证据：
- `Select.tsx:49-52`：注释「单值 actionLoading 被并发不同课程操作互相覆盖（A 在飞时点 B 会覆盖 A 的标记…）」+ `const [actionLoading, setActionLoading] = useState<ReadonlySet<number>>(new Set())`——Set 按课程 id 独立跟踪。
- 入口双守卫：`:92`（handleSelectClass 报名 `if (actionLoading.has(c.id)) return`）+ `:119`（handleConfirmExit 退选同款），双击/连点第二发短路不产生并发请求。
- 函数式置位/清除：置位 `setActionLoading((prev) => new Set(prev).add(c.id))`，finally 内函数式清除「只删自己的在飞标记」——不覆盖异课程在飞态。
- 渲染守卫全家：`:1120/:1133`（报名/退选按钮 disabled）+ `:1126/:1139`（"报名中..."/"退选中..."文案）+ `:1208/:1232/:1242/:1245`（退选弹窗 Esc/确认/取消 disabled + "退选中..."），全链路统一按课程在飞。
- 结论：Set 独立跟踪 + 入口双守卫 + 函数式置位/清除 + 渲染 disabled 四件套在位，无单值互踩残留，异课程并发不产生假失败 toast。

**角度二：Dashboard 折叠键盘路径（CollapseSection aria-expanded/aria-controls/useId）— 在位，纵深验证通过**

选此方向的理由：项目未装 Radix Collapsible，手写折叠段是"零依赖最小实现"与无障碍基线的交汇点；注释明确「button 带 aria-expanded/aria-controls，键盘可聚焦、读屏可感知展开状态」，且此前轮次只抽查过行号、未验证 useId 多实例唯一性，本轮可完整闭环。

实测证据：
- `Dashboard.tsx:50-79`：CollapseSection 组件——button 带 `type="button" aria-expanded={open} aria-controls={id} onClick={onToggle}`，正文 div 带 `id={id}`。useId 生成（:64）保证单实例唯一、零冲突（注释：此前硬编码 "collapse-body" 在多实例化时违反 DOM 唯一性）。
- 两处实例化零重复 id：extras 折叠段（:430，`open={extrasOpen}` 受控，`onToggle` 切 extrasOpen）+ 日期分组（:553，`open={expanded}` 受控，`onToggle` 按 g.key 进出 expandedDates 数组）——各实例独立 useId，不共享 id。
- 键盘可达性闭环：button 原生可聚焦（无 disabled），展开/收起状态经 aria-expanded 报读屏；正文经 aria-controls 关联。
- 结论：折叠族无障碍基线（aria-expanded + aria-controls + useId 唯一性 + 受控开关）四件套在位，无硬编码 id 残留，多实例零冲突。

## 维持观察项

1. **Select.tsx:204-207 注释口径残留**（R124 起延续）：注释「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——Dashboard 已拆 memo 叶子落地而 Select 未拆，注释与 Dashboard:90-93「已拆」口径不对称（本轮 :206 实测仍为「DOM 差分成本可忽略」口径，未升级为「已拆」）。名义候选维持，无实施动作。
2. **Admin 轮询带宽**：Admin 后台五 Tab 轮询（refetchInterval 5000/5000/10000/5000 + stats 5000）无独立降频记录，本轮未重核其与失败降频契约的关系，保持历史观察。
3. **perf 守卫弱断言**：perf-countdown-guard 对 Select 侧仅结构性断言（Dashboard 侧 memo 叶子在位、路由组件顶层无裸 cd.* 消费——但 Select 侧顶层正是唯一的 cd.* 消费点，断言不红灯），弱断言性质不变。
4. **Button.tsx 裸 button 候选名义**：`Button.tsx` 中 `const Comp = asChild ? Slot : "button"`（Button.tsx:33）——Button 组件内部裸 button 不在 OBSERVE-93-01 的 17 处统计内（该统计只数路由文件），候选名义维持。

## 结尾

**建议：APPROVE。** 聚焦清单 7 项（M-1 第九十一轮闭合 / OBSERVE-93-01 第四十五轮 / F93-01 第六十三轮 / OBSERVE-116-01 第三十九轮 / OBSERVE-115-01 / R125 候选 / 新契约角度 ×2）全部 ✅ 在位零漂移；四守卫脚本 18+3+6+5 断言与视觉审计全绿；构建通过；XSS 面零命中；工作区零漂移。无 CRITICAL/HIGH/MEDIUM/MINOR 级发现。
