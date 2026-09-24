# R154 前端只读审查报告（M-1）

> 审查模式：绝对只读（唯一写文件即本报告）。工作树基线 a72df70（R153 归档）。审查时间 2026-09-24。

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
| 工作树基线 | `git rev-parse HEAD` | 通过 | HEAD = a72df700d3be…，== a72df70 基线，分支 master 干净 |
| F93-01 双空实证 | `git log --oneline a72df70..HEAD -- web/` | 通过 | 输出为空（exit 0），基线后 web/ 零提交 |
| F93-01 双空实证 | `git diff --stat a72df70..HEAD -- web/` | 通过 | 输出为空（exit 0），基线后 web/ 零文件改动 |
| M-1 守卫脚本 1 | `node --import jiti/register scripts/target-guard-check.ts` | 全绿 | 18/18 断言 ✓（8 条 shouldDeferSave + 5 条 stale + 5 条 clean），exit 0 |
| M-1 守卫脚本 2 | `node --import jiti/register scripts/perf-countdown-guard.ts` | 全绿 | 3/3 断言 ✓（自 tick / memo 叶子 / 无裸 cd.*），exit 0 |
| M-1 守卫脚本 3 | `node --import jiti/register scripts/admin-auth-check.ts` | 全绿 | 6/6 断言 ✓，exit 0 |
| M-1 守卫脚本 4 | `node --import jiti/register scripts/unauthorized-check.ts` | 全绿 | 5/5 断言 ✓，exit 0 |
| 视觉审计 | `node scripts/audit.mjs` | 全绿 | A/B/C 三组全 ✓（含 6 文件 × 5 项黑洞清零），exit 0 |
| 构建 | `npm run build` | 通过 | tsc -b + vite build，1948 modules，built in 513ms，exit 0 |
| XSS 面 | `grep -rn "dangerouslySetInnerHTML" web/src/` | 零命中 | exit 1（无匹配） |
| 工作区漂移 | `git status --short --branch` | 零漂移 | 仅 `## master` 无改动；`git diff --stat a72df70 HEAD` 全仓库为空 |

## 聚焦清单逐项裁决

### 1. M-1 第九十轮闭合（保存链守卫）— ✅ 在位

- **shouldDeferSave 定义**：`web/src/lib/targetGuard.ts:64-71`。判据本体 :69-71 实测为：
  - `if (stateData === undefined) return true`（首帧未到无条件推迟）
  - `if (echoed) return false`（已回显稳态放行）
  - `return (stateData.courses?.length ?? 0) > 0 && hasSelected`（courses 非空且有选中才推迟）
  与聚焦清单逐字符一致。
- **恰 4 消费点**：`grep -o "shouldDeferSave(" src/routes/Select.tsx | wc -l` = **4**。逐处核验：
  - `Select.tsx:509`（flushTargets 消费时刻，传 `latestSelectedCount > 0` + `echoedRef.current`）
  - `Select.tsx:597`（handleBack 首轮守卫，传 `hasSelectedNow()`）
  - `Select.tsx:605`（handleBack 5s 等待循环 while 条件，同 hasSelectedNow）
  - `Select.tsx:699`（防抖回调 400ms 消费时刻，传 `selectedCount > 0`）
  四处均为完整三参调用（stateData/ref、hasSelected、echoedRef.current），无第四处裸调用。
- **echoedRef 三置位无第四处写 true**：`:200`（accountKey 守卫复位 false）、`:240`（courses 空首帧置 true）、`:297`（回显合并完成置 true）。grep `echoedRef.current = ` 仅 3 行，无第四处。
- **首帧四边界**：`:229`（`if (echoedRef.current) return` 只合并一次）、`:234`（`if (stateData === undefined) return` 首帧未到绝不提前置位）、`:247`（`if (pubs.length === 0) return` 发布缺席不置位）、`:319`（发布重建清理 effect 首行 `if (!echoedRef.current || publishes.length === 0) return` 未回显不清理）——四边界与契约一致。
- **F40-M1 cleanStaleSelected**：targetGuard.ts:26-43，实测空数组 key 保留、无变更返回原对象引用（:42 `return changed ? next : selected`），消费点 Select.tsx:290/:322。
- **F43-M1 hasSelected 清空分判**：shouldDeferSave 第二参数 hasSelected，脚本 8 条断言中「首帧未到 + 全清空 → 推迟」「courses 非空 + 全清空 → 放行」两条清空分判实测通过。
- **flush/handleBack/防抖三闸双闸等回显**：flush 守卫（:509）+ handleBack 双闸（:597 首轮 + :605 等待循环）全部消费 `echoedRef.current` 第三参，防抖 effect 依赖数组含 `echoDone`（:755）与 `stateData`——回显完成/数据到达双路驱动重跑自愈。

### 2. OBSERVE-93-01 第四十四轮 — ✅ 在位（零增零减）

- `<button` 全仓实测：Admin.tsx **13** / Dashboard.tsx **2** / Login.tsx **2** = **17 处 3 文件**，与 R153 口径一致，零增零减。
- 651/661 候选维持：Select.tsx:650-662 实测为注释区（「退出前 flush 已由"无用户改动即跳过"收敛」+ hasPublishes 声明），仍为名义候选，无漂移。

### 3. F93-01 第六十二轮 — ✅ 在位（双空实证）

- `git log --oneline a72df70..HEAD -- web/` 输出为空；`git diff --stat a72df70..HEAD -- web/` 输出为空。双命令 exit 0。基线后 web/ 无任何提交与文件改动，候选（Select 倒计时渲染内联 cd.*）维持不实现。

### 4. OBSERVE-116-01 第三十八轮 — ✅ 在位（注释口径统一 + 五路轮询零漂移）

- **注释口径**：`useTickingCountdown.ts:3-8` 实测「内部自 tick（每秒 setNow）… 每秒 setNow 实际触发宿主路由组件重渲染（React 语义：useState 归属宿主即重渲染宿主）… 消费方须把每秒变化的 cd.* 收敛到 memo 叶子组件（Dashboard 已拆 CountdownMatrix 并 memo）」；Dashboard.tsx:206-209 同口径「删除整页每秒 setTick——倒计时收敛 useTickingCountdown 自 tick（Dashboard/Select 同实现）… cd.* 已收敛到 MemoCountdownMatrix 叶子」；Dashboard.tsx:90-93 同源「宿主路由组件每秒 setNow 引发的重渲染到这里被 React bail out… 已拆」。三处口径一致（含"潜在优化 → 落地实现"的演进事实），无矛盾残留。
- **五路轮询契约逐键零漂移**：
  - `/state`（Dashboard:171-172）：error→30000，window_closed→30000，否则 3000
  - `/logs`（Dashboard:186-188）：error→30000，state.window_closed→30000，否则 3000
  - `/electives`（Dashboard:203）：恒 30000
  - Select `/state`+`/electives` 双查询（Select:153-154）：error→30000，window_closed→30000，否则 2000
  - error 与 window_closed 全部降 30000，无一漂移为其他值。

### 5. OBSERVE-115-01 弹窗族 — ✅ 在位（三处最小语义门行号一致）

- Select:1204（`role="dialog" aria-modal="true" aria-labelledby="exit-modal-title"` + Escape 守卫）
- Login:226（`role="dialog" aria-modal="true" aria-labelledby="activate-dialog-title"` + Esc 关闭）
- Admin:213（`role="dialog" aria-modal="true" aria-labelledby="delete-acct-modal-title"` + Escape 守卫）
- 三处行号与 R153 一致，零漂移。

### 6. R125 候选复核 — ✅ 维持（不实现）

- **Select.tsx:850 内联 cd.\***：实测 `{cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒` 在路由组件 JSX 内直接渲染（:850），倒计时秒级变化确会触发 Select 整树重渲染——候选事实仍成立。
- **缓解因子无回归**：
  - 2s 轮询吸收：Select `/state`+`/electives` 成功态 2s 轮询，每秒 setNow 独立于轮询，但重渲染 diff 成本可忽略（Select.tsx:204-207 注释「DOM 差分成本可忽略」）。
  - memo 叶子：Dashboard 侧已拆 MemoCountdownMatrix 落地；Select 侧单行文本内联，整树无昂贵列表重建，DOM diff 为纯文本节点比对。
  - 守卫盲区契约零漂移：perf-countdown-guard 3 断言全绿（Dashboard 侧 memo 叶子在位、路由组件顶层无裸 cd.* 消费），未扩及 Select 侧，维持名义候选记录。

### 7. 新契约角度（自选 ×2，时间盒内完成）

**角度一：401 吊销切号整体重建（key={account}）— 在位，纵深验证通过**

选此方向的理由：契约 13（决策手册）声明 Select 挂载点必须 `key={account}` 以在 401 切号/代理切换/重登时整体重建实例（selected/echoedRef/rev 全复位），是「防串线」族的核心实现点，且本轮正好在核 echoedRef 置位族，属同源纵深。

实测证据：
- `App.tsx:293-295`：`<Select key={targetAccount} account={targetAccount}>`，key 绑定 targetAccount 对象——账号切换即整体卸载重建，selected/echoedRef/rev 状态全随实例销毁。
- 代理态与管理员态共享 targetAccount 语义：`App.tsx:124`（登出清除代理）、:130（page 复位）、:202（管理员 401 被动吊销同步退出代理态）注释族完整。
- onUnauthorized 被动吊销（:147-160 区间）实测含三层：反查归属（先账号名直查再令牌反查，非本机会话跳过）→ 代理态同步退出（`prev === lostAccount || lostAccount === adminName ? null : prev`）→ 管理态同步退出（lostAccount === adminName 时 setInAdmin(false) + 清管理令牌标记）。
- Select 内部 accountKey 兜底守卫（:195-202）：account 变化时 setSelected({}) / setRev(0) / echoedRef=false / setEchoDone(false)，与 App key 双保险，TDZ 注释（:194 声明于状态区之后）已在位。
- 结论：`key={account}` 与内部兜底双路齐备，无漂移。401 被动吊销（不经过 logout）与主动登出六清除点族（onUnauthorized/logout/onDeleted 三路径全含 setInAdmin(false) + setAdminToken("") + saveAdminToken("")）成对对称，撞名学生不可达管理态（isCurrentAdminSession 纯函数 token 比对）。

**角度二：管理令牌五路径成对全清（isCurrentAdminSession 纯函数）— 在位，纵深验证通过**

选此方向的理由：契约 41（F52-M1）管理态判定绑定「带 adminName 的签发」，登出/吊销/删除/切号/回学生端全路径清标记是对称性的试金石，且 admin-auth-check 6 断言在聚焦清单内，属随手可闭环的纵深。

实测证据：
- `web/src/lib/adminAuth.ts`：`isCurrentAdminSession(sessions, adminName, adminToken)` = `adminToken !== "" && sessions[adminName] === adminToken`。纯函数，admin-auth-check 6 断言全绿（含撞名学生 token ≠ 管理 token → 学生端）。
- 五路径成对全清实测：
  1. logout（:118-122）：setInAdmin(false) + setAdminToken("") + saveAdminToken("")
  2. onUnauthorized 吊销管理员（:157-161）：同三连
  3. onDeleted 删管理员自身（:169-173）：同三连
  4. 账号迁移 effect（:147）：`!isCurrentAdminSession(...) → setInAdmin(false)`——唯一依据为 token 比对，无 adminName 字符串相等判定残留
  5. `setAdminToken` 登记侧（:98-99）：仅登录响应带 adminName 才 setInAdmin(true) + saveAdminToken(token)，撞名学生响应无 adminName 不进入
- 结论：判定纯函数化 + 全路径清标记 + 持久化（xk_admin_token）降级（try/catch）三件套在位，无「账号名=管理员名」冒充判据残留。401 吊销不经过 logout 的被动路径与主动登出完全对称。

## 维持观察项

1. **Select.tsx:204-207 注释口径残留**（R124 起延续）：注释「如需真正做到『只重渲染倒计时一处』需拆独立 memo 叶子组件（潜在优化，非当前承诺）」——Dashboard 已拆 memo 叶子落地而 Select 未拆，注释与 Dashboard:90-93「已拆」口径不对称。名义候选维持，无实施动作。
2. **Admin 轮询带宽**：Admin 后台轮询无独立降频记录（本轮未重核 Admin 轮询间隔，未列入聚焦清单，保持历史观察）。
3. **perf 守卫弱断言**：perf-countdown-guard 对 Select 侧仅结构性断言（Dashboard 侧），Select 内联 cd.* 不红灯，弱断言性质不变。
4. **Button.tsx 裸 button 候选名义**：`Button.tsx` 中 `const Comp = asChild ? Slot : "button"`（Button.tsx:34）——Button 组件内部裸 button 不在 OBSERVE-93-01 的 17 处统计内（该统计只数路由文件），候选名义维持。

## 结尾

**建议：APPROVE。** 聚焦清单 7 项（M-1 第九十轮闭合 / OBSERVE-93-01 第四十四轮 / F93-01 第六十二轮 / OBSERVE-116-01 第三十八轮 / OBSERVE-115-01 / R125 候选 / 新契约角度 ×2）全部 ✅ 在位零漂移；四守卫脚本 18+3+6+5 断言与视觉审计全绿；构建通过；XSS 面零命中；工作区零漂移。无 CRITICAL/HIGH/MEDIUM/MINOR 级发现。
