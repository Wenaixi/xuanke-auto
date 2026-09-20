# round58 前端只读审查发现报告

> 审查基线：master @ `1434b5d`（R57 收官，R57 前端零修复轮；R55→R56→R57 之间 `web/src/` 连续三轮零提交，本轮 `git log 808a4e6..HEAD -- web/src/` 再实证零输出 + `git diff HEAD -- web/src/` 零输出，即 R55→R58 间 web/src **连续四轮零提交**）。审查期间绝对只读（唯一新建文件为本报告）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **成功**（tsc -b + vite 全绿，1948 modules / 417.54 kB js / 41.07 kB css，产物落 backend/web/dist，`git status --short` 与基线一致——仅根目录 5 个未跟踪社区文档，无 web/src 改动）；TDD 断言 `npx jiti scripts/target-guard-check.ts` **16 项全绿**、`admin-auth-check.ts` **6 项全绿**、`unauthorized-check.ts` **5 项全绿**。
> 范围 `web/src/` 全部 `.ts/.tsx`（19 文件，含全部 ui 组件与 lib 纯函数）；后端契约对照 `backend/internal/api/handler.go`（handleElectives 233-277 / handleElectiveSelect 285-360 / handleState 519-539 / handleLogs 553-561 / handleLogout 563-575 / requireAdminSession 584-598）、`backend/internal/scheduler/scheduler.go`（submitIntervalFor 289-295 / sprintDuration 72 / tick 967-1032 / submitAll 1337+ / probeIntervalFor 75-108 / openTimeFor 419-441）。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 10（N-1 延续 + O-1~O-8 延续 + 1 条本轮新增极轻观察 N-2）。** 与 R57 相同，web/src 连续四轮零提交后依旧零缺陷：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读零回归，任务书新视角 A-E 全部经实证裁决——A（N-1 冲刺文案）连续两轮观察后**裁决维持**（附整治路径与不整治理由，见 N-1），B（保存链六防）零回归，C（O-2 NaN / O-5 Dashboard key）连续四轮观察后**裁决维持**，D（全包逐行通读）唯一新观察 = N-2（开窗后 pick() 立即性边界，纯文案/时机描述级），E（上轮观察项全表）逐条延续。

**最致命 3 条（按影响排序）**：本轮实际无致命缺陷；按关注价值排序见文末结论节。

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无 MINOR。）

---

## OBSERVE（观察项，未加重）

### N-1.【延续，连续两轮】「开始冲刺/后台冲刺」按钮文案与后端冲刺实际逻辑的描述性出入——**裁决维持不动作**（附极简整治方向）

**一句话问题**：Select.tsx:1126-1134 `t.in_date_range || stateData?.window_opened` 分支下按钮文案恒为「设为后台冲刺目标 / 已设为后台冲刺#{优先级}」——但后端 `submitIntervalFor`（scheduler.go:290-295）定义的冲刺只在**开窗点后 10 秒黄金期**（`now.After(open) && now.Before(open.Add(sprintDuration))`，sprintDuration=10s 实证 scheduler.go:72），黄金期后目标提交降为 **1s 常态**（submitIntervalNormal）。即开窗后第 11 秒起用户看到的"冲刺"文案描述的后端行为已不存在（提交仍在进行，只是降频）。

**本轮复证（任务书重点 A 前半）**：
1. Select.tsx:1126 分支判定 → 1127-1135 开窗后收敛为 ghost 小按钮，文案 1134 `已设为后台冲刺${priorityName(selIdx)}`。
2. scheduler.go:70-72 实证 `submitIntervalSprint=250ms / submitIntervalNormal=1s / sprintDuration=10s`；290-295 `submitIntervalFor` 黄金期判定；tick 1027-1029 `submitInterval := s.submitIntervalFor(now, open)` 按黄金期选频。
3. 黄金期后 1s 常态提交照常进行（scheduler.go:1027-1031 实证）——"后台预选目标"语义真实存在，文案未到"错误"程度。

**任务书重点 A 后半（pick() 行为与文案语义一致性）——本轮补实证**：Select.tsx:346-372 `pick()` 只做 `setSelected + setRev + toast`，**不触发任何立即报名请求**；400ms 防抖后 PUT /targets（saveNow 430-434），后端 tick 下一拍按 `submitIntervalFor` 提交目标。即开窗后点击"设为后台冲刺目标"的行为 = 把课程加入预选目标等待后台提交，与文案的"后台自动提交"核心语义一致（唯一出入在"冲刺"一词描述的高频时段）。

**裁决（连续两轮观察后的明确结论）：维持 OBSERVE 不动作。** 理由：
1. **前端无法精确感知黄金期**——黄金期判定基于调度器对齐钟 + 10s 窗口（scheduler.go:291），前端只知 `in_date_range`/`window_opened` 两个粗粒度信号，**做三态文案（预选/冲刺/后台预选）是伪精确**：前端无法区分"开窗后 0-10s"与"开窗后 >10s"，三态方案下"冲刺"态判定本身就是猜测，为纯文案收益引入不诚实判定，负收益。
2. 黄金期 10s 内用户点击场景下文案确实准确（250ms 冲刺真实存在）；10s 后用户的主要操作通道是**官网 btn_type=2「报名」primary 大按钮**（Select.tsx:1110-1122，仍在按钮区首位），冲刺 ghost 小按钮只是次要入口。
3. **若未来仍想消除出入，极简方向 = 统一改「后台目标」**（`设为后台目标` / `已设为后台目标`），删掉"冲刺"二字即可：1s 常态与 250ms 冲刺均为"后台自动提交目标"，语义全时段成立，零行为风险、不引入窗口态细分。此方案成本一行文案、收益为消除描述出入；当前零行为影响，**不做**（ponytail：纯文案改动不在黄金期价值区间）。

### N-2.【本轮新增，极轻】开窗后点击冲刺小按钮 → 目标提交存在至多一拍调度延迟（时机描述级，非缺陷）

**一句话问题**：开窗后点击「设为后台冲刺目标」仅写入目标 + 防抖保存，后端下一 tick（黄金期 250ms / 常态 1s）才真正提交——用户可能误以为点击即报名。

**证据链**：
1. Select.tsx:346-372 `pick()` 无任何网络请求，仅 `setSelected` + `setRev`（371）。
2. 防抖 400ms 后 PUT /targets（saveNow 430-434）。
3. 后端 tick（scheduler.go:1027-1031）下一拍 `submitIntervalFor` 提交 → 黄金期 ≤250ms、常态 ≤1s 延迟。

**触发条件**：开窗后用户点冲刺小按钮而非官网「报名」主按钮（btn_type=2 primary 大按钮，Select.tsx:1110-1122——该按钮直发 `handleSelectClass` 立即报名）。

**影响**：纯时机感知级——最多 1 秒延迟，与 N-1 同源（冲刺按钮语义 = 后台预选入口），不构成行为缺陷；官网「报名」按钮才是立即报名通道，二者职责分离清晰（按钮区同卡片上下排列，用户可区分）。

**裁决**：OBSERVE 记录不动作；若做 N-1 文案整治（统一"后台目标"）时顺带在按钮 title/文案补一句"后台提交，至多 1 秒内生效"即可闭合。零行为影响。

### O-1.【延续】Toast viewport 滚动交互残余——滚轮需悬停 toast 上、滚动条不可拖、无自动滚动到底部

**本轮复核**：Toast.tsx:105 viewport `max-h-[80vh] overflow-y-auto pointer-events-none` + Root `pointer-events-auto`（F48-M1 设计），`@radix-ui/react-toast` viewport pointer-events 由 `hasToasts ? void 0 : "none"` 驱动（R56 实证）。任何"修复"都破坏 F48-M1"仅 toast 可交互"语义；M-2 去重 + 3.5s 自消下同刻 >4 条概率趋零。**维持 OBSERVE 不动作**。

### O-2.【延续，连续四轮】useTickingCountdown 非法日期串 NaN 防御——是否值得一行 `!Number.isFinite`

**本轮复证（任务书重点 C 前半）**：`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`——非法串 → NaN → "NaN" 四位 + isExpired=false。两调用点输入源全受控——`openTimeStr` 走后端 `time.Time` JSON RFC3339（scheduler.go:46 实证 `OpenTime time.Time json:"open_time"` 无自定义 MarshalJSON）、兜底 `new Date(number).toISOString()` 恒合法；Select.tsx:749-754 与 Dashboard.tsx:190-195 双调用点逐一复核。Safari 严格模式非 ISO 串是唯一理论点，本项目不存在该输入形态。修复一行会引入 isExpired 语义争议（Select.tsx:826 "本地已到开窗点"横幅）。**裁决维持 OBSERVE 不动作**（若未来开放时间来源扩展为不可信字符串，先补 `!Number.isFinite` 再上线）。

### O-3.【延续】手写模态无焦点陷阱 + F6-02 锚点注释
三处手写 modal（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）均有 role=dialog/aria-modal + Esc 关闭 + autoFocus，但均无完整焦点陷阱（Tab 可逃出弹层）。维持 OBSERVE。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询
React bailout 使叶子不重渲、实际 DOM 写入仅倒计时文本节点；Radix TabsContent 懒渲染（`present && children`）非激活 Tab 无观察者不轮询。维持 OBSERVE。

### O-5.【延续，连续四轮】Dashboard key 不对称 + expandedDates 跨账号残留——是否值得一行 key 修复

**本轮复证（任务书重点 C 后半）**：App.tsx:332 Dashboard 挂载点无 `key={account}`（Select 两处 App.tsx:294/340 有）——账号切换原地重渲，`expandedDates`（Dashboard.tsx:268-273）与 `extrasOpen`（212）残留旧账号折叠态。后端侧 `StateForAccount`（scheduler.go 按 `c.Account == acct` 过滤）与 `handleElectives` 的 `allowAccountOverride + accountExists` 凭据表校验（handler.go:243-253）双证数据不串线，跨账号残留最多是"折叠状态"而非"数据串线"。**裁决维持 OBSERVE**（纯展示层、触发窗口 = "A 折叠全部 → 切 B → B 首组也收起"；若做切号视觉一致性整治，一行 `key={current}` 即可，成本零收益 = 折叠态干净）。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体
grep 复证：`CardFooter` 仅 Card.tsx:46/51 自身定义，全仓零消费；Button 12 变体（Button.tsx:7）实际 `variant=` 传值仅 primary/outline/ghost/dark/destructive 子集、Badge 8 变体（Badge.tsx:5）实际仅 primary/outline 子集。维持 OBSERVE。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页生命周期
无 storage 事件监听；多标签页共享 localStorage 的 B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持 OBSERVE。

### O-8.【延续】401 保护窗口闭包 current + 双形态按钮
App.tsx:187-235 onUnauthorized 闭包捕捉 current、effect 依赖 [adminName, inAdmin, adminToken] 重建刷新闭包；`lostRaw = detail?.session || detail?.account || current`——session 恒为 401 真实主体（业务请求恒带 Bearer），current 兜底永不触发，陈旧闭包无实际影响。Select.tsx:1090-1156 官网 btn_type / 冲刺双轨按钮设计意图确认。维持观察。

---

## 重点核对结论（任务书逐条裁决）

### A. N-1 文案整治裁决 → **维持 OBSERVE**（连续两轮观察，理由见 N-1 详述）

核心三连：① 前端无黄金期精确感知 → 三态文案是伪精确（负收益）；② 黄金期后用户主通道是官网「报名」primary 大按钮，冲刺小按钮是次要入口；③ 若治，极简方向 = 统一「后台目标」删"冲刺"二字（一行文案全时段语义成立），当前零行为影响不做。pick() 行为（346-372）只设目标、无立即报名请求，与"后台自动提交"核心语义一致，唯一出入在"冲刺"一词描述的高频时段——非行为缺陷。

### B. Select 保存链六防全路径复证（任务书重点 B）——逐字符零回归

web/src 在 `808a4e6..1434b5d` 之间零提交（本轮 `git log` 实证），六防代码自 R54 修后连续三轮未被触碰——本轮为**逐字符通读 + 关键路径推演**。全部落位且语义正确：

| 防护族 | 通读复证结论 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 双参三消费点（防抖 Select.tsx:676 / flushTargets 499 / handleBack 589）判据同源；`stateData===undefined || (courses 非空 && hasSelected)`——首帧带旧目标但用户全清空（hasSelected=false）→ 放行 PUT []。16 项断言全绿 |
| **F42-M1 判据与数据源解耦** | 纯数据判据不依赖 echoedRef（防抖 effect 依赖含 stateData，Select.tsx:737）；flushTargets 与 handleBack 三处消费时刻读 `stateDataRef.current` 快照（499/589/676）。零死锁路径——推演"PUT [] 在飞 + 用户立刻点新课"时序：置脏 → savingRef 串行化 finally 补发最新快照（455-458）→ 收敛，无丢失 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 消费点全在（回显 287 / 独立清理 effect 316-327 / 防抖 696 / flush 518）；独立清理 effect 依赖含 selected（327）；toast 判 `!unmountedRef.current` 不轰炸卸载后（520/698）；`cleanStaleSelected` 无变更返回原引用防不必要重渲染（targetGuard.ts:42） |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19：`arr !== undefined && arr.length > 0`）——场景 C/D 断言覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:294/340 双 keyed + Select accountKey 守卫（195-202）声明于 echoedRef/rev 之后 TDZ 安全；`if (accountKey !== account)` 渲染期同步复位 selected/rev/echoedRef/echoDone。TDZ 复查：防抖 effect（645-737）引用 selectedCount（739 声明）**仅在异步 timer 回调内**，effect 同步体只定义 build/timer，回调 400ms 后执行时声明已完成——无 TDZ |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow。viewport 不在空壳 wrapper 上 |
| **Toast 去重语义** | 去重 key = `typeof title === "string"`；全仓 15 调用点 title 全字符串（grep 复证 Select 8 + Admin 7）；合并保留原 id → React key 稳定；M-B duration 保留首值。Select.tsx:289 与 321 两处「发布已更新」同 title 同 description 去重合并无害 |

### C. O-2（NaN 防御）与 O-5（Dashboard key 不对称）→ 连续四轮观察后裁决维持（见 O-2 / O-5）

- **O-2**：两调用点输入源全受控合法（Go RFC3339 / 毫秒数）；修复一行引 isExpired 语义争议（Select.tsx:826 横幅依赖 isExpired 语义）。**维持**。
- **O-5**：影响 = 折叠态（expandedDates + extrasOpen）跨账号复用，纯展示层零数据影响（后端双证不串线）；一行 `key={account}` 零成本收益 = 折叠态干净。**维持**（切号视觉一致性整治时一并加）。

### D. 全包逐行通读找新问题（任务书重点 D）

**目标保存链 / 回显合并 / 手动报名协同 / Admin 五 Tab / 401 处理 / Toast 去重 / 多账号年级隔离前端侧 / 悬浮栏 / 倒计时逐区通读，唯一新观察 = N-2（开窗后 pick() 提交时机，时机描述级），其余零缺陷。**

补充验证（消费点对后端契约的依赖全部成立）：
1. **`?account=` 穿透全路径凭据表校验**：handleElectives（handler.go:243-253）/ handleElectiveSelect（293-301）/ handleState（526-536）三路 + handleSetTargets/handleElectiveExit（历轮实证）全部先 `allowAccountOverride`（管理员会话令牌）再 `accountExists`——幽灵账号整体拒绝，前端 `?account=encodeURIComponent(account)` 的穿透 URL 与后端校验兼容。
2. **手动报名/退选双端幂等**：后端 `TryAcquireSubmit` 拒并发 → `CheckClassSelectable` 快照复核 → `SelectClass` 平台真实报名 → `MarkDone`；前端 `actionLoading` Set<number> 按课程独立跟踪（Select.tsx:52/92-93/106-110/119-120/131-135）——报名/退选各自函数式删除只清自己的 id，绝不抹掉其他课程在飞标记。双端幂等成立。
3. **401 单广播三形态**：client.ts:64-69（HTTP 401 前置广播，防网关非 JSON 体）+ 75-95（body 401 仅在 `r.status !== 401` 时补广播）——HTTP401+body401 / 网关非 JSON / 旧式 HTTP200+body401 三种形态各单次；App.onUnauthorized 幂等（185-235）。
4. **Admin 五 Tab**：CodesTab removing Set 按码独立跟踪（311/345-350/361-365）+ generating 守卫（327-328）；ConfigTab F18-02 `!loaded` 拒绝保存（517）+ R27-01 refetch 成功才自增 epoch（537-540）+ apiKey 刻意不回填（脱敏语义，503-511 只回填非密钥字段 + 保存时 `apiKey.trim()` 才传 530）；StatsTab 三态窗口（719）；AccountsTab targets/success 空数组兜底（817-818）；LogsTab limit=200（879）。零缺陷。
5. **多账号年级隔离前端侧**：Dashboard dateGroups 输入源 = `state.courses`（StateForAccount 按 acct 过滤，scheduler.go 实证）+ `/electives` 专属快照（ElectivesSnapshotFor 回退链决策锚 8）——前端无跨账号数据串线；Select 挂载点 `key={account}` + accountKey 守卫双保险；Admin 各查询 queryKey 含 account（317/494/703/790/878）。
6. **手动退选 refused 语义对齐**：Dashboard 状态机（586-616 已确认选课/已满员·退避备选/报名异常/提交中/待命）与后端 status 字段（pending/in_range/submitted/success/failed）全枚举对应，`isFullFallback` 用 `result.includes("已满员")` 匹配调度器 markFullLocked 文案（36-38）。零遗漏。
7. **倒计时条件顺序正确**：Select.tsx:809-835「未识别到开放时间」（821）先于 `cd.isExpired`（826）——target=null（识别缺席 + begin_times 空）时走"未识别"而非误显"本地已到开窗点"；Dashboard 文案行（399-419）同款顺序（openTimeStr → begin_times[0] → 未识别 → 同步中）正确。

### E. 上轮观察项延续复核（全表，任务书重点 E）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| N-1（冲刺文案） | OBSERVE（R57 新增） | 双端复证：文案恒"后台冲刺"，后端冲刺仅 10s 黄金期；pick() 只设目标无立即报名；前端无黄金期感知 → 三态伪精确，统一"后台目标"一行可治但零收益 | **延续**（见 N-1 详述） |
| O-1（Toast viewport 滚动） | OBSERVE | 机制实证逐字节一致，修复破坏 F48-M1，触发概率趋零 | **延续** |
| O-2（NaN 防御） | OBSERVE | 输入源全受控合法；修复一行引 isExpired 语义争议 | **延续** |
| O-3（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱 | 延续 |
| O-4（每秒重渲 + Admin 单查询） | OBSERVE | bailout + TabsContent 懒渲染实证延续 | 延续 |
| O-5（Dashboard key 不对称） | OBSERVE | 影响=折叠态跨账号复用纯展示层（后端双证不串数据）；一行 key 零成本 | **延续** |
| O-6（ui 模板残宽） | OBSERVE | CardFooter 零消费 + 死变体 grep 复证 | 延续 |
| O-7（多标签页） | OBSERVE | 无 storage 同步，方向安全 | 延续 |
| O-8（401 闭包 + 双形态按钮） | OBSERVE | 保守方向确认；current 兜底永不触发（session 恒有值） | 延续 |

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 10（N-1 延续 + N-2 本轮新增 + O-1~O-8 延续）**。tsc exit 0、build 成功（1948 modules / 417.54 kB js / 41.07 kB css）、TDD 断言 16+6+5 全绿、web/src 与 HEAD 逐字节一致（R55→R58 连续四轮零提交基线，本轮 `git log 808a4e6..HEAD -- web/src/` 实证零输出）。
- **最致命 3 条（按影响排序）**：
  1. （本轮无真实缺陷）首要关注 = **N-1：「后台冲刺」按钮文案与后端冲刺实际逻辑（仅开窗后 10 秒黄金期 250ms）的描述性出入**——纯文案、零行为影响，窗口开放中后台提交完全正常（1s 常态）；连续两轮观察后裁决维持，若未来整治极简方向 = 统一「后台目标」删"冲刺"二字（三态文案是前端无法感知黄金期的伪精确，明确否决）。
  2. **O-5：Dashboard 挂载点无 key={account}**——expandedDates/extrasOpen 跨账号残留纯展示层；若做切号视觉一致性整治，一行加 key 即可。
  3. **O-2：useTickingCountdown NaN 防御**——当前输入源全受控合法（Go RFC3339 / 毫秒数）；若未来开放时间来源扩展为不可信字符串先补 `!Number.isFinite` 再上线。
- **已核对无缺陷的高风险区域**：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读 + 关键时序推演零回归（含"PUT [] 在飞 + 用户点新课"串行化收敛推演）；`?account=` 穿透三路凭据表校验与前端契约对齐；手动报名/退选双端幂等（Set<number> 独立跟踪）；401 三形态单广播；open_time RFC3339 序列化契约前后端一致（NaN 理论触发点不存在）；多账号年级隔离前端侧零串线；倒计时条件顺序（未识别先于 isExpired）正确。
- **建议优先修复方向**：本轮零修复项，全部 OBSERVE 无需动作。连续四轮（R54 五修后）前端防线稳定，下轮可转向极低频残余（O-5 一行 key / N-1 文案删"冲刺" / O-2 防御一行）作为体验整治候选，或维持纯观察。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | 成功（1948 modules / 417.54 kB js / 41.07 kB css，dist 落 backend/web/dist） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | 16/16 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 工作树一致性 | `git status --short` | 仅根目录 5 个未跟踪社区文档，与基线一致 |
| web/src 零改动 | `git diff HEAD -- web/src/` | 零输出 |
| 跨轮次零提交 | `git log 808a4e6..HEAD -- web/src/` | 零输出 |
