# round56 前端只读审查发现报告

> 审查基线：master @ `363d8e4`（R55 收官，工作树预期仅根目录 5 个未跟踪社区文档 + archive/review-rounds/ 内 R55 相关文件）。审查期间绝对只读（未创建/修改/删除任何文件，唯一新建文件为本报告）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **成功**（tsc -b + vite 全绿，1948 modules / 417.54 kB js / 41.07 kB css）；TDD 断言 `npx jiti scripts/target-guard-check.ts` **16 项全绿**、`admin-auth-check.ts` **6 项全绿**、`unauthorized-check.ts` **5 项全绿**；`git status --short` 与基线完全一致（仅 5 个未跟踪社区文档）；`git diff HEAD -- web/src/` 零输出；`git log 93e07ef..363d8e4 -- web/src/` 零提交——**R55 至 R56 之间 web/src 无任何改动，本轮为纯读复核，继承性回归的判据是逐字符比对而非 diff**。
> 范围 `web/src/` 全部 `.ts/.tsx`（18 文件）；后端契约对照 `backend/internal/api/handler.go`（handleElectives 235-277 / handleElectiveSelect 285-360 / handleElectiveExit 362-424 / handleSetTargets 435-517 / handleState 519-539 / requireAuth 1074-1093 / allowAccountOverride 1052-1056 / accountExists 1058-1072 / handleAdminStats 878-952）、`backend/internal/scheduler/scheduler.go`（StateForAccount 704-730 / SchedulerState 44-59 / open_time 序列化 49）。
> Radix 运行时实证：`web/node_modules/@radix-ui/react-toast/dist/index.js`——ToastViewport pointerEvents 由 `hasToasts ? void 0 : "none"` 控制（228），与 R55 引用版本逐字节一致；`@radix-ui/react-tabs/dist/index.js:221` TabsContent `present && children` 懒渲染实证延续。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 9（2 条新观察 + 7 条延续）。** 与 R55 相同，web/src 连续两轮零修复后依旧零缺陷：六防保存链（F43/F42/F40/F39/F36/F48-M1）全部落位且语义正确，本轮新视角重点（A. Toast viewport 滚动边界 / C. NaN 防御 / D. Dashboard key 不对称）经实证均为"当前受控源下零触发、收益极低"的 OBSERVE，维持不动。

**与 R55 相比唯一新增 OBSERVE**：N1. Dashboard `relativeCountdown` 折叠行时间摘要以 `Date.now()` 渲染期为基准（最坏 1 秒陈旧），与主矩阵逐秒一致性的承诺存在毫秒级偏差——纯展示层、零影响，不动作。

**最致命 3 条（按影响排序）**：本轮实际无致命缺陷；前三优先关注方向见文末结论节。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无 MINOR。）

---

## OBSERVE（观察项，未加重）

### O-1.【延续，R55 O-1 升级复核】Toast viewport 滚动交互残余——滚轮需悬停 toast 上、滚动条不可拖、无自动滚动到底部

**一句话问题**：M-C 修复（`max-h-[80vh] overflow-y-auto`，Toast.tsx:105）达成"堆叠超限可滚动查看旧 toast"的目标，Radix 官方 pointer-events 模式下滚动交互残余四类（滚轮悬停 toast 才滚动 / 间隙穿透滚到页面 / 滚动条不可拖 / 无自动滚动到底部），是否值得升级处理。

**本轮升级复核结论（对照任务书重点 A）**：
1. **机制实证（node_modules 逐字节复核）**：`react-toast/dist/index.js:228` `style: { pointerEvents: hasToasts ? void 0 : "none" }`——viewport pointer-events 由 `toastCount>0` 实时驱动；`hasToasts &&` 条件渲染 FocusProxy（230/243）。toast 存在时 viewport 恢复 pointer-events（void 0 = 继承），不存在时 none。**与 R55 引用的实现逐字节一致，无版本漂移**。
2. **是否升级判断**：F48-M1 的设计意图即"仅 toast 可交互"（viewport pointer-events-none + Root pointer-events-auto，Toast.tsx:78/105）；**当前残余交互全部是这套设计的自然产物，任何"修复"（如 viewport 改 pointer-events-auto + Root 改 none）都会破坏 F48-M1 语义**（背景页面不可点 → 用户必须先关 toast 才能操作）。备选方案（限 4 条挤最旧）牺牲"查看旧 toast 内容"（dirtyRef 保留 + 失败文案可读性），更差。
3. **触发概率量化**：M-2 去重（同 title 合并只更新 description，Toast.tsx:40-55）+ 全仓 15 个 toast 调用点全部字符串 title → 同刻并存 >4 条概率极低；3.5s 自动消失。滚动需求实际触发频率趋近零。
4. **替代机制"自动滚到底部"**：Radix 官方 viewport 无 scrollTo 管理（源码无滚动位置逻辑），需外部 effect 监听 toasts 变化——为"大概率永远不触发"的场景加 effect + ref + scrollTo 三件套，ponytail 权衡否决。
5. **结论**：维持 OBSERVE，不升级 MINOR、不动作。若用户反馈"滚不到旧 toast"再评估限条数方案。

### O-2.【延续，R55 O-2 复核】useTickingCountdown 非法日期串 NaN 防御——是否值得一行 `!Number.isFinite`

**一句话问题**：`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`——非法日期串 → NaN → `NaN<=0` false → `pad(NaN)` → **"NaN" 四位 + isExpired=false**。一行 `!Number.isFinite(diff) || diff <= 0` 即可防御，是否值得。

**本轮升级复核**：
1. **当前两个调用点的输入源全受控且合法**：Dashboard.tsx:190-195 与 Select.tsx:749-754 的 target 来源只有两个——`openTimeStr`（后端 `StateForAccount` 输出，Go `time.Time` JSON 序列化）、`new Date(number).toISOString()`（`begin_times` 是 `number[]` 毫秒时间戳，`new Date(ms)` 恒合法）。R55 复核确认调度器层无自定义 MarshalJSON、无格式化字符串路径——**结构上不存在非法串入口**。
2. **未来来源扩展的触发条件**：若未来把 target 换成"后端下发的可读字符串"（如 `2006-01-02 15:04:05` 直接下发）——但本项目 open_time 就是 `time.Time` 走 Go 标准 JSON 序列化（RFC3339 形如 `2026-09-13T09:00:00+08:00`），`new Date(RFC3339)` 合法；即使换格式，`new Date("2006-01-02 15:04:05")` 在主流浏览器（Safari 例外）也能解析。**Safari 严格模式会把非 ISO 串判 Invalid Date——这是唯一的理论触发点**。
3. **防御成本**：一行 `!Number.isFinite(diff) || diff <= 0`；NaN 分支归入"全 00 + isExpired=true"语义——注意 isExpired=true 会改变横幅分支（Select.tsx:826 "本地已到开窗点"），NaN 时显示该文案是误导还是保守，语义上可讨论。
4. **结论**：当前全源受控 + 零触发 + 修复一行会引入 isExpired 语义争议 → 维持 OBSERVE 不动（与 R55 同裁决）。**若未来开放时间来源引入不可信字符串源，先补防御再上线**（任务书预判的触发条件成立，但当前不存在）。

### O-3.【延续】手写模态无焦点陷阱 + F6-02 锚点注释
三处手写 modal（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）均有 role=dialog/aria-modal + Esc 关闭 + autoFocus，但均无完整焦点陷阱（Tab 可逃出弹层）；grep `Radix Dialog|F6-02` 仅命中 Login.tsx:222 一处锚点注释。维持 OBSERVE。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询
React bailout 使叶子不重渲、实际 DOM 写入仅倒计时文本节点；Radix TabsContent 懒渲染（`present && children`，react-tabs/dist/index.js:221 实证）非激活 Tab 查询无观察者不轮询——管理员停留任一 Tab 时轮询 = 该 Tab 单查询。维持 OBSERVE。

### O-5.【延续，R55 O-5 复核】Dashboard key 不对称 + expandedDates 跨账号残留——是否值得一行 key 修复

**一句话问题**：App.tsx:332 Dashboard 挂载点无 `key={account}`（Select 两处 App.tsx:294/340 有 `key={account}`）——账号切换原地重渲，`expandedDates`（Dashboard.tsx:268-273）残留旧账号折叠态。

**本轮升级复核**：
1. **残留影响范围**：`expandedDates` 是 `string[] | null`，仅存"哪个日期组展开"。账号切换后 dateGroups 按新账号 courses 重建——旧账号展开的日期 key（如 "2026-09-13"）若新账号也有同日期组，会沿用展开态；无同日期则种子 effect 重种首个。**影响=折叠状态跨账号复用，纯展示层，零数据/零保存链影响**。
2. **会不会自愈**：`expandedDates` 状态属于 Dashboard 实例，账号切换（原地重渲非重建）不重置；但 Dashboard 与 Select 路由互斥，用户切号通常先回 Dashboard（挂载不变）→ 切换账号。真正受影响的窗口 = "A 账号折叠全部 → 切 B 账号 → B 首组也收起"。概率低、无数据风险。
3. **修复成本**：`key={current}` 一行——但会导致账号切换时 Dashboard 整体重建（react-query 缓存 key 已含 account，重建仅重渲无网络重复）。**一行收益 = 折叠态干净；成本 = 无。**
4. **结论**：维持 OBSERVE。若做切号视觉一致性整治，一行 `key={account}` 即可；当前纯展示层不值得为它动 App 渲染树。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体
grep 复证：`CardFooter` 仅 Card.tsx:46/51 自身定义，全仓零消费；Button 12 变体（Button.tsx:7）实际 `variant=` 传值仅 primary/outline/ghost/dark/destructive 子集、Badge 8 变体（Badge.tsx:5）实际仅 primary/outline 子集。维持 OBSERVE（组件本体被消费，清剿收益低）。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页生命周期
无 storage 事件监听；多标签页共享 localStorage 的 B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持 OBSERVE。

### O-8.【延续】401 保护窗口闭包 current + 双形态按钮
App.tsx:188-235 onUnauthorized 闭包捕捉 current、effect 依赖 [adminName, inAdmin, adminToken] 重建刷新闭包，保守方向确认；Select.tsx:1090-1156 官网 btn_type / 冲刺双轨按钮设计意图确认。维持观察。

### N-1.【本轮新增】Dashboard relativeCountdown 折叠行以渲染期 Date.now() 为基准（毫秒级陈旧）

**一句话问题**：Dashboard.tsx:197-198 `const nowMs = Date.now()` 在渲染期计算，折叠行时间摘要（relativeCountdown，94-108）以该渲染拍为基准；下一拍由 useTickingCountdown 的 1s interval 驱动。行内摘要的"分钟/小时"更新在渲染拍之间最多陈旧 1 秒（`totalMin = Math.floor(diff/60000)` 等取整运算天然吞掉不足一分钟的余量）——**对显示精度零影响**（分钟级单位本身就不是秒级敏感）。

**证据链**：
1. Dashboard.tsx:92 注释承诺"折叠行渲染期直接算 Date.now() 即新鲜（最坏 1 秒陈旧），绝不为此再建额外定时器"——承诺的实现正确。
2. 取整语义：`totalMin = Math.floor(diff/60000)`、`totalHour`、`d` 全向下取整，陈旧 1 秒只影响"xx分59秒→xx分00秒"的进位边界显示，分钟粒度下不可感知。
3. 无第三方消费者（`nowMs` 仅传 relativeCountdown）。

**结论**：设计正确、实现正确，纯观察记录——不动作（ponytail：为分钟级摘要建定时器是负收益）。

---

## 重点核对结论（任务书逐条裁决）

### A. R55 O-1（Toast viewport 滚动残余）复核 → 维持 OBSERVE（见 O-1）

- 机制实证：node_modules 逐字节一致（`hasToasts ? void 0 : "none"`），无版本漂移。
- **任何"修复"都破坏 F48-M1 设计意图**（pointer-events-none viewport + auto Root 是"仅 toast 可交互"的实现方式），备选方案更差。
- 触发概率趋近零（M-2 去重 + 3.5s 自消 + 同刻 >4 条极低）。
- **结论：不升级、不动作，维持观察**。

### B. Select 保存链六防全路径复证（任务书重点 B）——逐字符零回归

web/src 在 `93e07ef..363d8e4` 之间零提交（`git log` 实证），六防代码自 R54 修后未再被触碰——**"复证"的判据升级为逐字符通读而非 diff**。全部落位且语义正确：

| 防护族 | 通读复证结论 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 双参三消费点（防抖 676 / flushTargets 499 / handleBack 589）判据同源；`stateData===undefined || (courses 非空 && hasSelected)`——首帧带旧目标但用户全清空（hasSelected=false）→ 放行 PUT []。16 项断言全绿 |
| **F42-M1 判据与数据源解耦** | 纯数据判据不依赖 echoedRef（防抖 effect 依赖含 stateData，Select.tsx:737——/state 数据到达触发重跑自愈）；flushTargets 与 handleBack 三处消费时刻读 `stateDataRef.current` 快照。零死锁路径 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（回显 287 / 独立清理 effect 316-327 / 防抖 696 / flush 518）全部存在；独立清理 effect 依赖含 selected（327）覆盖"回显合并带出 stale"时序；toast 判 `!unmountedRef.current` 不轰炸卸载后（520/698） |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）——场景 C/D 断言覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:294/340 双 keyed + Select accountKey 守卫（195-202）声明于 echoedRef/rev 之后 TDZ 安全；`if (accountKey !== account)` 渲染期同步复位 selected/rev/echoedRef/echoDone |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow。**viewport 不在空壳 wrapper 上**（R54 修正确认） |
| **Toast 去重语义** | 去重 key = `typeof title === "string"`（全仓 15 调用点全字符串）；合并保留原 id → React key 稳定；M-B duration 保留首值（`{...prev[idx], description, variant}` 不再携带 duration）→ Radix duration effect 不重启计时器 |

### C. useTickingCountdown NaN 防御（任务书重点 C）→ 维持 OBSERVE（见 O-2）

### D. Dashboard key={account} 不对称（任务书重点 D）→ 维持 OBSERVE（见 O-5）

### E. 上轮观察项延续复核（全表）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| O-1（Toast viewport 滚动） | OBSERVE | 机制实证逐字节一致，修复会破坏 F48-M1，触发概率趋零 | **延续（升级复核后仍 OBSERVE）** |
| O-2（NaN 防御） | OBSERVE | 两调用点输入源全受控合法；Safari 非 ISO 串是唯一理论点但本项目 open_time 走 RFC3339；修复一行引 isExpired 语义争议 | **延续** |
| O-3（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱，F6-02 锚点仅 Login.tsx:222 | 延续 |
| O-4（每秒重渲 + Admin 单查询） | OBSERVE | bailout + TabsContent 懒渲染实证延续 | 延续 |
| O-5（Dashboard key 不对称） | OBSERVE | 影响=折叠态跨账号复用纯展示层；修复一行零成本 | **延续** |
| O-6（ui 模板残宽） | OBSERVE | CardFooter 零消费 + 死变体 grep 复证 | 延续 |
| O-7（多标签页） | OBSERVE | 无 storage 同步，方向安全 | 延续 |
| O-8（401 闭包 + 双形态按钮） | OBSERVE | 保守方向确认 | 延续 |

### F. 全包逐行通读找新问题（任务书重点 F）

**目标保存链 / 回显合并 / 手动报名协同 / Admin 五 Tab / 401 处理 / Toast 去重逐区通读，唯一新观察 = N-1（relativeCountdown 渲染期基准），其余零缺陷。**

补充验证（消费点对后端契约的依赖全部成立）：
1. **`?account=` 穿透全路径凭据表校验**：handleElectives（handler.go:243-252）/ handleElectiveSelect（293-301）/ handleElectiveExit（366-374）/ handleSetTargets（437-476）/ handleState（526-536）五路全部先 `allowAccountOverride`（1052-1056，仅管理员会话令牌可穿透）再 `accountExists`/LoadCredentials（1058-1072）——**幽灵账号整体拒绝，前端"账号不存在"分支（401 事件不会误广播、Select 正常渲染空列表）**。
2. **PUT /targets 幂等语义**：`req.Targets == nil → []`（484-486）+ 数量/范围校验（490-507）+ 落库（508-512）+ 调度器同步（512）——前端 flushTargets/防抖消费时刻守卫与后端校验完全兼容；`priority > 999` 拒绝边界与前端 priority 序号（数组下标）永不越界。
3. **手动报名/退选在飞互斥**：后端 `TryAcquireSubmit` 拒并发（318-323/391-396），前端 `actionLoading` Set<number> 按课程独立跟踪（Select.tsx:52/92/119）——**双端幂等**。
4. **401 单广播**：client.ts:64-69（HTTP 401 前置广播）+ 75-95（body 401 仅在 `r.status !== 401` 时补广播）——三种形态各单次，App.onUnauthorized 幂等。
5. **`open_time` 序列化契约**：Go `time.Time`（scheduler.go:49 `json:"open_time"`）→ JSON 标准 RFC3339（`2026-09-13T09:00:00+08:00`）→ 前端 `new Date(openTimeStr)` 解析合法（Select.tsx:749 / Dashboard.tsx:191 / 205-206）——**后端与前端契约一致，O-2 NaN 的理论触发点不存在**。

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 9（N-1 本轮新增 + O-1~O-8 延续）**。tsc exit 0、build 成功（1948 modules / 417.54 kB js / 41.07 kB css）、TDD 断言 16+6+5 全绿、web/src 与 HEAD 逐字节一致（零提交基线）。
- **最致命 3 条（按影响排序）**：
  1. （本轮无真实缺陷）首要关注 = **O-1：Toast viewport 滚动交互残余**——经实证为 Radix 官方 pointer-events 模式 + F48-M1 设计意图的自然产物，任何修复都会破坏"仅 toast 可交互"语义；触发概率趋零。无动作，仅记录边界。
  2. **O-2：useTickingCountdown NaN 防御**——当前输入源全受控合法（Go RFC3339 / 毫秒数），Safari 非 ISO 串是唯一理论点；若未来来源扩展为不可信字符串先补 `!Number.isFinite` 再上线。
  3. **O-5：Dashboard 挂载点无 key={account}**——expandedDates 跨账号残留纯展示层；若做切号视觉一致性整治，一行加 key 即可。
- **已核对无缺陷的高风险区域**：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读零回归；`?account=` 穿透五路凭据表校验（B15/B26/B27 族）与前端契约对齐；PUT /targets 幂等与后端校验兼容；手动报名/退选双端幂等；401 三形态单广播；open_time RFC3339 序列化契约前后端一致（NaN 理论触发点不存在）。
- **建议优先修复方向**：本轮零修复项，全部 OBSERVE 无需动作。连续三轮（R54 五修后）前端防线稳定，下轮可转向极低频残余（O-5 一行 key / O-2 防御一行）作为体验整治候选，或维持纯观察。
