# round57 前端只读审查发现报告

> 审查基线：master @ `808a4e6`（R56 收官，R56 前端零修复轮；R55→R56→R57 之间 `web/src/` 连续两轮零提交，实证 `git log 363d8e4..808a4e6 -- web/src/` 零输出 + `git diff HEAD -- web/src/` 零输出）。审查期间绝对只读（唯一新建文件为本报告）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **成功**（tsc -b + vite 全绿，1948 modules / 417.54 kB js / 41.07 kB css，产物落 backend/web/dist，`git check-ignore` 确认 dist 已忽略，工作树无 diff）；TDD 断言 `npx jiti scripts/target-guard-check.ts` **16 项全绿**、`admin-auth-check.ts` **6 项全绿**、`unauthorized-check.ts` **5 项全绿**；`git status --short` 与基线一致（仅根目录 5 个未跟踪社区文档）。
> 范围 `web/src/` 全部 `.ts/.tsx`（18 文件）；后端契约对照 `backend/internal/api/handler.go`（handleLogin 102-159 / handleActivate 181-214 / handleElectives 235-277 / handleElectiveSelect 285-360 / handleElectiveExit 362-424 / handleSetTargets 435-517 / handleState 519-539 / handleAdminStats 865-957 / requireAuth/allowAccountOverride 1052-1072）、`backend/internal/scheduler/scheduler.go`（StateForAccount 704-730 / windowClosedLocked 909-935 / probe 1039-1160 / tick 969-1032 / openTimeFor 421-443 / MarkDone 1907-1967 / CheckClassSelectable 1848-1877 / submitIntervalFor 290-295）、`backend/internal/accounts/manager.go`（gateTryAcquire 223-235 / LoginByPassword 243+）、`backend/internal/store/store.go`（ConsumeActivationCode 292-324 / SaveCredential 29+）。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 10（1 条本轮新增 N-1 + 9 条延续）。** 与 R56 相同，web/src 连续三轮零修复后依旧零缺陷：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读零回归，任务书新视角 A-E 全部经实证裁决为 OBSERVE 且维持不动。新增一条纯记录级观察 N-1（「开始冲刺」按钮文案与后端冲刺实际逻辑的描述性出入，纯文案、零行为影响）。

**最致命 3 条（按影响排序）**：本轮实际无致命缺陷；按关注价值排序见文末结论节。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无 MINOR。）

---

## OBSERVE（观察项，未加重）

### O-1.【延续】Toast viewport 滚动交互残余——滚轮需悬停 toast 上、滚动条不可拖、无自动滚动到底部

**一句话问题**：M-C 修复（`max-h-[80vh] overflow-y-auto`，Toast.tsx:105）达成"堆叠超限可滚动查看旧 toast"的目标，Radix 官方 pointer-events 模式下滚动交互残余四类。

**本轮复核**：R56 已实证 `@radix-ui/react-toast/dist/index.js` viewport pointer-events 由 `hasToasts ? void 0 : "none"` 驱动（与 F48-M1 设计意图一致）；任何"修复"都破坏 F48-M1"仅 toast 可交互"语义，备选方案更差；M-2 去重 + 3.5s 自消下同刻 >4 条概率趋零。**维持 OBSERVE 不动作**。

### O-2.【延续】useTickingCountdown 非法日期串 NaN 防御——是否值得一行 `!Number.isFinite`

**一句话问题**：`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`——非法日期串 → NaN → "NaN" 四位 + isExpired=false。

**本轮复核**：两调用点（Dashboard.tsx:190-195 / Select.tsx:749-754）输入源全受控——`openTimeStr` 走后端 `time.Time` JSON RFC3339（scheduler.go:46 实证 `OpenTime time.Time json:"open_time"` 无自定义 MarshalJSON）、兜底 `new Date(number).toISOString()` 恒合法；Safari 严格模式对非 ISO 串判 Invalid Date 是唯一理论点，本项目不存在该输入形态。修复一行会引入 isExpired 语义争议（Select.tsx:826 "本地已到开窗点"横幅）。**维持 OBSERVE 不动作**。

### O-3.【延续】手写模态无焦点陷阱 + F6-02 锚点注释
三处手写 modal（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）均有 role=dialog/aria-modal + Esc 关闭 + autoFocus，但均无完整焦点陷阱（Tab 可逃出弹层）。维持 OBSERVE。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询
React bailout 使叶子不重渲、实际 DOM 写入仅倒计时文本节点；Radix TabsContent 懒渲染（`present && children`）非激活 Tab 无观察者不轮询。维持 OBSERVE。

### O-5.【延续】Dashboard key 不对称 + expandedDates 跨账号残留——是否值得一行 key 修复

**一句话问题**：App.tsx:332 Dashboard 挂载点无 `key={account}`（Select 两处 App.tsx:294/340 有 `key={account}`）——账号切换原地重渲，`expandedDates`（Dashboard.tsx:268-273）残留旧账号折叠态。

**本轮升级复核（任务书重点 C）**：
1. **影响范围实测**：`expandedDates` 是 `string[] | null`，仅存"哪个日期组展开"。账号切换后 dateGroups 按新账号 courses 重建——旧账号展开的日期 key（如 "2026-09-13"）若新账号也有同日期组会沿用展开态；无同日期则种子 effect 重种首个。**纯展示层，零数据/零保存链影响**。
2. **后端多账号年级隔离侧确认**：`StateForAccount`（scheduler.go:704-730）按 `c.Account == acct` 过滤 Courses（724-727 实证），账号 A 的目标绝不出现在账号 B 的 /state——前端 Dashboard 分组（dateGroups，Dashboard.tsx:219-263）输入源天然按账号隔离，跨账号残留最多是"折叠状态"而非"数据串线"。**与决策锚 7（?account= 全路径凭据表校验）不冲突**。
3. **成本**：`key={current}` 一行；收益 = 折叠态干净。触发窗口 = "A 折叠全部 → 切 B → B 首组也收起"，概率低。
4. **结论**：连续三轮 OBSERVE。若做切号视觉一致性整治，一行 `key={account}` 即可；当前纯展示层不值得为它动 App 渲染树。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体
grep 复证：`CardFooter` 仅 Card.tsx:46/51 自身定义，全仓零消费；Button 12 变体（Button.tsx:7）实际 `variant=` 传值仅 primary/outline/ghost/dark/destructive 子集、Badge 8 变体（Badge.tsx:5）实际仅 primary/outline 子集。维持 OBSERVE。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页生命周期
无 storage 事件监听；多标签页共享 localStorage 的 B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持 OBSERVE。

### O-8.【延续】401 保护窗口闭包 current + 双形态按钮
App.tsx:188-235 onUnauthorized 闭包捕捉 current、effect 依赖 [adminName, inAdmin, adminToken] 重建刷新闭包，保守方向确认；Select.tsx:1090-1156 官网 btn_type / 冲刺双轨按钮设计意图确认。维持观察。

### N-1.【本轮新增】「开始冲刺」按钮文案与后端冲刺实际逻辑的描述性出入

**一句话问题**：Select.tsx 冲刺/预选按钮逻辑分支 `t.in_date_range || stateData?.window_opened`（1126）——**开窗后（窗口开放中）文案仍显示「设为后台冲刺目标」**，但后端 `submitIntervalFor`（scheduler.go:290-295）定义的冲刺只发生在 **开窗点后 10 秒黄金期**（`now.After(open) && now.Before(open.Add(sprintDuration))`，sprintDuration=10s，scheduler.go 291 实证），且 `probeIntervalNear=2s` 下的目标提交在黄金期后**继续按 1s 常态提交**（submitIntervalNormal）。即开窗后第 11 秒起，"冲刺"文案描述的后端行为已不存在——窗口开放中后台仍在提交（只是降为 1s 常态），文案未到"错误"程度，属**描述性出入**。

**证据链**：
1. Select.tsx:1126 `t.in_date_range || stateData?.window_opened ?` → 开窗后按钮形态收敛为 ghost 小按钮（1127-1135），文案 `已设为后台冲刺${priorityName(selIdx)}`。
2. scheduler.go:290-295 `submitIntervalFor`：`now.After(open) && now.Before(open.Add(sprintDuration))` 才返回 `submitIntervalSprint`（250ms 实证 sprintDuration 定义见 scheduler.go 上部，注释"开窗前 10 秒黄金期"）；否则 `submitIntervalNormal`（1s）。
3. scheduler.go:1027 `submitInterval := s.submitIntervalFor(now, open)`——黄金期外 1s 常态提交照常进行。

**触发条件**：窗口开放中（in_date_range=true）且距开窗点超过 10 秒，用户看到「已设为后台冲刺#N」徽标/按钮文案。

**影响**：纯文案（description-level）。行为层面，开窗后目标提交完全正常（1s 常态即"后台预选目标"语义），不会因文案误导产生数据风险；用户在窗口开放中手动点冲刺按钮也只是把课程加入预选目标（pick() 行为，Select.tsx:346-372），不触发任何额外后端请求。

**修复方向**（若治）：文案区分三态——窗口未开（「设为预选目标」）/ 黄金期内（「设为冲刺目标」）/ 黄金期后开窗中（「设为后台预选」），或统一为「设为后台目标」。当前零行为影响，**维持 OBSERVE 不动作**。

---

## 重点核对结论（任务书逐条裁决）

### A. N-1（relativeCountdown 渲染期基准）→ 维持 OBSERVE（纯观察，不值得处理）

- 机制复核：Dashboard.tsx:197-198 `const nowMs = Date.now()` 渲染期计算，折叠行摘要以该渲染拍为基准；下一拍由 useTickingCountdown 的 1s interval 驱动。**最坏 1 秒陈旧**。
- 取整语义：`relativeCountdown`（94-108）`totalMin/totalHour/d` 全向下取整，陈旧 1 秒只影响"xx分59秒→xx分00秒"进位边界显示，分钟粒度不可感知。
- 连带一致性检查（任务书要求）：主矩阵 `cd`（useTickingCountdown，每秒 tick）与折叠行 `nowMs`（渲染期）——同一渲染拍内主矩阵的 `cd.days/hours/minutes/seconds` 与折叠行摘要共享同一 now 基准（tick 触发重渲后两者同时刷新），**承诺未打破**：两处显示同一时刻的快照，差异只有"折叠行分钟取整 vs 主矩阵秒级"，这是设计意图（分钟级摘要不需要秒级精度）。
- **结论**：纯观察记录，不动作（ponytail：为分钟级摘要建定时器是负收益）。

### B. Select 保存链六防全路径复证（任务书重点 B）——逐字符零回归

web/src 在 `363d8e4..808a4e6` 之间零提交（`git log` 实证），六防代码自 R54 修后连续三轮未被触碰——**"复证"判据升级为逐字符通读**。全部落位且语义正确：

| 防护族 | 通读复证结论 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 双参三消费点（防抖 Select.tsx:676 / flushTargets 499 / handleBack 589）判据同源；`stateData===undefined || (courses 非空 && hasSelected)`——首帧带旧目标但用户全清空（hasSelected=false）→ 放行 PUT []。16 项断言全绿 |
| **F42-M1 判据与数据源解耦** | 纯数据判据不依赖 echoedRef（防抖 effect 依赖含 stateData，Select.tsx:737——/state 数据到达触发重跑自愈）；flushTargets 与 handleBack 三处消费时刻读 `stateDataRef.current` 快照（499/589/676）。零死锁路径 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（回显 287 / 独立清理 effect 316-327 / 防抖 696 / flush 518）全部存在；独立清理 effect 依赖含 selected（327）覆盖"回显合并带出 stale"时序；toast 判 `!unmountedRef.current` 不轰炸卸载后（520/698） |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）——场景 C/D 断言覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:294/340 双 keyed + Select accountKey 守卫（195-202）声明于 echoedRef/rev 之后 TDZ 安全；`if (accountKey !== account)` 渲染期同步复位 selected/rev/echoedRef/echoDone |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow。viewport 不在空壳 wrapper 上（R54 修正确认） |
| **Toast 去重语义** | 去重 key = `typeof title === "string"`（全仓 15 调用点全字符串）；合并保留原 id → React key 稳定；M-B duration 保留首值 |

### C. O-2（useTickingCountdown NaN 防御）与 O-5（Dashboard key 不对称）→ 连续三轮观察后维持（见 O-2 / O-5）

- **O-2**：两调用点输入源全受控合法（Go RFC3339 / 毫秒数），Safari 非 ISO 串是唯一理论点但本项目 open_time 走 `time.Time` 标准序列化（scheduler.go:46 实证无自定义 MarshalJSON）；修复一行引 isExpired 语义争议。**维持**。
- **O-5**：一行 `key={account}` 零成本收益 = 折叠态干净；当前纯展示层、触发窗口窄。**维持**（若做切号视觉一致性整治时一并加）。

### D. 全包逐行通读找新问题（任务书重点 D）

**目标保存链 / 回显合并 / 手动报名协同 / Admin 五 Tab / 401 处理 / Toast 去重 / 多账号年级隔离前端侧逐区通读，唯一新观察 = N-1（「开始冲刺」文案描述性出入），其余零缺陷。**

补充验证（消费点对后端契约的依赖全部成立）：
1. **`?account=` 穿透全路径凭据表校验**：handleElectives（handler.go:243-252）/ handleElectiveSelect（293-301）/ handleElectiveExit（366-374）/ handleSetTargets（444-476）/ handleState（526-536）五路全部先 `allowAccountOverride`（1057-1059，仅管理员会话令牌可穿透）再 `accountExists`/LoadCredentials（1064-1072）——幽灵账号整体拒绝。
2. **PUT /targets 幂等语义**：`req.Targets == nil → []`（484-486）+ 数量/范围校验（490-507）+ 落库（508-512）+ 调度器同步（512）——前端 flushTargets/防抖消费时刻守卫与后端校验完全兼容；`priority > 999` 拒绝边界与前端 priority 序号（数组下标）永不越界。
3. **手动报名/退选在飞互斥**：后端 `TryAcquireSubmit` 拒并发（318-323/391-396，scheduler.go:1881-1899）→ `CheckClassSelectable` 快照复核（327，scheduler.go:1848-1877）→ `SelectClass` 平台真实报名 → `MarkDone`（358）→ 前端 `actionLoading` Set<number> 按课程独立跟踪（Select.tsx:52/92/119）——双端幂等。
4. **401 单广播**：client.ts:64-69（HTTP 401 前置广播）+ 75-95（body 401 仅在 `r.status !== 401` 时补广播）——三种形态各单次，App.onUnauthorized 幂等。
5. **`open_time` 序列化契约**：`time.Time`（scheduler.go:46 `json:"open_time"`）→ RFC3339 → 前端 `new Date(openTimeStr)` 解析合法（Select.tsx:749 / Dashboard.tsx:191 / 205-206）——NaN 理论触发点不存在。
6. **多账号年级隔离前端侧**：Dashboard dateGroups 输入源 = `state.courses`（StateForAccount 按 acct 过滤，scheduler.go:724-727）+ `/electives` 专属快照（ElectivesSnapshotFor 回退链决策锚 8）——前端无跨账号数据串线；Select 挂载点 `key={account}`（App.tsx:294/340）+ accountKey 守卫双保险。
7. **手动退选 refused 语义对齐**：后端 `RemoveDone` 置 refused（决策锚 14，A2 绝不静默抢回）→ Dashboard 状态行 `已手动退选（自动引擎不再接管，可重新设为目标恢复）`（scheduler.go:595 实证）→ 前端文案 `已确认选课/已满员·退避备选/报名异常/提交中/待命` 状态机（Dashboard.tsx:586-616）与后端 status 字段（pending/in_range/submitted/success/failed）全枚举对应，无遗漏。

### E. 上轮观察项延续复核（全表）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| O-1（Toast viewport 滚动） | OBSERVE | 机制实证逐字节一致，修复会破坏 F48-M1，触发概率趋零 | **延续** |
| O-2（NaN 防御） | OBSERVE | 输入源全受控合法（RFC3339/毫秒数）；Safari 非 ISO 串是唯一理论点；修复一行引 isExpired 语义争议 | **延续** |
| O-3（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱，F6-02 锚点仅 Login.tsx:222 | 延续 |
| O-4（每秒重渲 + Admin 单查询） | OBSERVE | bailout + TabsContent 懒渲染实证延续 | 延续 |
| O-5（Dashboard key 不对称） | OBSERVE | 影响=折叠态跨账号复用纯展示层（年级隔离后端侧实证不串数据）；修复一行零成本 | **延续** |
| O-6（ui 模板残宽） | OBSERVE | CardFooter 零消费 + 死变体 grep 复证 | 延续 |
| O-7（多标签页） | OBSERVE | 无 storage 同步，方向安全 | 延续 |
| O-8（401 闭包 + 双形态按钮） | OBSERVE | 保守方向确认 | 延续 |
| N-1（relativeCountdown 渲染期基准） | OBSERVE | 纯观察，分钟粒度零影响 | 延续 |

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 10（N-1 本轮新增 + O-1~O-8 + 上轮 N-1 延续）**。tsc exit 0、build 成功（1948 modules / 417.54 kB js / 41.07 kB css）、TDD 断言 16+6+5 全绿、web/src 与 HEAD 逐字节一致（连续两轮零提交基线）。
- **最致命 3 条（按影响排序）**：
  1. （本轮无真实缺陷）首要关注 = **N-1：「开始冲刺」按钮文案与后端冲刺实际逻辑（仅开窗后 10 秒黄金期 250ms）的描述性出入**——纯文案、零行为影响，窗口开放中后台提交完全正常（1s 常态）；若做文案整治建议区分「设为预选目标 / 设为冲刺目标 / 设为后台预选」三态。
  2. **O-5：Dashboard 挂载点无 key={account}**——expandedDates 跨账号残留纯展示层；若做切号视觉一致性整治，一行加 key 即可。
  3. **O-2：useTickingCountdown NaN 防御**——当前输入源全受控合法（Go RFC3339 / 毫秒数）；若未来开放时间来源扩展为不可信字符串先补 `!Number.isFinite` 再上线。
- **已核对无缺陷的高风险区域**：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读零回归；`?account=` 穿透五路凭据表校验与前端契约对齐；PUT /targets 幂等与后端校验兼容；手动报名/退选双端幂等；401 三形态单广播；open_time RFC3339 序列化契约前后端一致（NaN 理论触发点不存在）；多账号年级隔离前端侧零串线。
- **建议优先修复方向**：本轮零修复项，全部 OBSERVE 无需动作。连续三轮（R54 五修后）前端防线稳定，下轮可转向极低频残余（O-5 一行 key / O-2 防御一行 / N-1 文案三态）作为体验整治候选，或维持纯观察。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | 成功（1948 modules / 417.54 kB js / 41.07 kB css，dist 落 backend/web/dist 已忽略） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | 16/16 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 工作树一致性 | `git status --short` | 仅根目录 5 个未跟踪社区文档，与基线一致 |
| web/src 零改动 | `git diff HEAD -- web/src/` | 零输出 |
| 跨轮次零提交 | `git log 363d8e4..808a4e6 -- web/src/` | 零输出 |
| dist 忽略确认 | `git check-ignore backend/web/dist/index.html` | 命中（已忽略） |
