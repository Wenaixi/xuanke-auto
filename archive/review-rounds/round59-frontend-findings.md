# round59 前端只读审查发现报告

> 审查基线：master @ `899e8c3`（R58 收官，R58 前端零修复轮；R55→R58 间 `web/src/` 连续四轮零提交 + R58→R59 继续零提交，实证 `git log 899e8c3^..HEAD -- web/src/` 零输出 + `git diff HEAD -- web/src/` 零输出（0 行））。审查期间绝对只读（唯一新建文件为本报告，`git status --short` 写报告前已确认工作树完全干净——含 archive/ 下 121 个已跟踪文件，与基线一致）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **成功**（tsc -b + vite 全绿，1948 modules / 417.54 kB js / 41.07 kB css，产物落 backend/web/dist 已忽略，构建后 `git status --short` 仍零输出）；TDD 断言 `npx jiti scripts/target-guard-check.ts` **16 项全绿**、`admin-auth-check.ts` **6 项全绿**、`unauthorized-check.ts` **5 项全绿**。
> 范围 `web/src/` 全部 `.ts/.tsx`（19 文件）；后端契约对照 `backend/internal/api/handler.go`（handleLogin B43-04 121 / handleElectives 235-277 / handleElectiveSelect 285-360 / handleSetTargets 435-517 / handleState 519-539 / handleAdminStats 865-957 / requireAuth 1077-1095 / allowAccountOverride+accountExists 1055-1075）、`backend/internal/scheduler/scheduler.go`（submitIntervalSprint=250ms/Normal=1s/sprintDuration=10s 69-73 / submitIntervalFor 289-295 / tick 提交 1024-1031 / StateForAccount 704-730 / openTimeForLocked 432-443 / windowClosedLocked 909-935 / probe 识别槽写入 1097-1123 / markFullLocked 1744-1756 / MarkDone 1907-1967 / SetTargetsForAccount 459-494 / rebuildCoursesForAccountLocked 573-609）、`backend/internal/zhidao/client.go`（Class 552-569 / Publish 572-582 / ElectivesData 584-588）。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 10（N-1 延续第 3 轮 + N-2 延续第 2 轮 + 1 条本轮新增极轻观察 N-3 + O-1~O-8 延续）。** 与 R57/R58 相同，web/src 连续五轮零提交后依旧零缺陷：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读零回归，任务书新视角 A-E 全部经实证裁决——A（N-1 冲刺文案 + N-2 调度延迟）连续观察后**裁决维持**（附极简整治方向），B（六防全路径复证）零回归，C（O-2 NaN / O-5 Dashboard key）连续五轮观察后**裁决维持**，D（全包逐行通读）唯一新观察 = N-3（渲染层英文注释残留 + 两处既有格式瑕疵复核），E（上轮观察项全表）逐条延续。

**最致命 3 条（按影响排序）**：本轮实际无致命缺陷；按关注价值排序见文末结论节。

---

## CRITICAL（数据丢失）

（本轮无 CRITICAL。）

---

## MAJOR（明确错误行为 / 合法操作被静默撤销）

（本轮无 MAJOR。）

---

## MINOR（展示 / 边界一致性 / 协议冗余）

（本轮无 MINOR。）

---

## OBSERVE（观察项，未加重）

### N-1.【延续，连续三轮】「后台冲刺」按钮文案与后端冲刺实际逻辑的描述性出入——**裁决维持不动作**（附极简整治方向）

**一句话问题**：Select.tsx:1134（开窗分支）按钮文案恒为「已设为后台冲刺#{优先级}」，但后端 `submitIntervalFor`（scheduler.go:290-295）定义的冲刺只在**开窗点后 10 秒黄金期**（`now.After(open) && now.Before(open.Add(sprintDuration))`，sprintDuration=10s 实证 scheduler.go:72），黄金期后目标提交降为 **1s 常态**（submitIntervalNormal）。即开窗后第 11 秒起"冲刺"文案描述的后端行为已不存在（提交仍在进行，只是降频）。

**本轮复证（任务书重点 A 前半）**：
1. Select.tsx:1126 `t.in_date_range || stateData?.window_opened` 分支 → 1127-1135 开窗后收敛为 ghost 小按钮，文案 1134 `已设为后台冲刺${priorityName(selIdx)}`。
2. scheduler.go:69-73 实证 `submitIntervalSprint=250ms / submitIntervalNormal=1s / sprintDuration=10s`；290-295 `submitIntervalFor` 黄金期判定；tick 1027 `submitInterval := s.submitIntervalFor(now, open)` 按黄金期选频。
3. 黄金期后 1s 常态提交照常进行（scheduler.go:1027-1031 实证）——"后台预选目标"语义真实存在，文案未到"错误"程度。

**pick() 行为一致性复证（任务书重点 A 后半）**：Select.tsx:346-372 `pick()` 只做 `setSelected + setRev + toast`，**不触发任何立即报名请求**；400ms 防抖后 PUT /targets（saveNow 430-434），后端 tick 下一拍按 `submitIntervalFor` 提交目标。即开窗后点击"设为后台冲刺目标"的行为 = 把课程加入预选目标等待后台提交，与文案的"后台自动提交"核心语义一致（唯一出入在"冲刺"一词描述的高频时段）。

**裁决（连续三轮观察）**：维持 OBSERVE 不动作。理由：① 前端无黄金期精确感知（黄金期判定基于调度器对齐钟 + 10s 窗口，前端只有 in_date_range/window_opened 粗信号），三态文案是伪精确，纯文案收益引入不诚实判定，负收益；② 黄金期 10s 内用户点击场景下文案确实准确（250ms 冲刺真实存在）；③ 若未来整治，极简方向 = 统一「后台目标」删"冲刺"二字（一行文案全时段语义成立，零行为风险）。ponytail：纯文案改动不在黄金期价值区间，不做。

### N-2.【延续，第二轮】开窗后点击冲刺小按钮 → 目标提交存在至多一拍调度延迟（时机描述级，非缺陷）

**一句话问题**：开窗后点击「设为后台冲刺目标」仅写入目标 + 防抖保存，后端下一 tick（黄金期 250ms / 常态 1s）才真正提交——用户可能误以为点击即报名。

**证据链**：pick()（Select.tsx:346-372）无网络请求 → 防抖 400ms 后 PUT /targets → 后端 tick（scheduler.go:1026-1031）下一拍 `submitIntervalFor` 提交。

**裁决**：OBSERVE 记录不动作；若做 N-1 文案整治时顺带在按钮 title 补"后台提交，至多 1 秒内生效"即可闭合。零行为影响。

### N-3.【本轮新增，极轻】Select.tsx:1123 渲染层英文注释残留 + 两处既有格式瑕疵复核（格式卫生级）

**一句话问题**：Select.tsx:1123 冲刺按钮区块注释「only affects itself」为英文残留（与周边中文注释混排），项目规范（全局 CLAUDE.md：「文档和注释不要使用 emoji，不要 ai 味」，历轮 gofmt/注释清扫口径）要求注释中文化——极小格式瑕疵。

**证据链**：
1. Select.tsx:1123：`{/* 本项目特冲刺/预选目标按钮：... only affects itself */}`——「only affects itself」是英文半句残留，无法与前句中文构成完整语义（疑似历史迭代残句，意为"按钮形态只影响自身不干扰官网按钮"）。
2. 连带复核（既有，非本轮引入）：Select.tsx:379 `const lastJson = useRef("")` 缩进为 4 空格（同块注释后首个语句，邻接声明 380-382 皆 2 空格），属历史格式瑕疵；Dashboard.tsx:501 `</div>                <div className="py-2.5...">` 同一行两元素闭合/开启（缺换行）。

**触发条件**：纯阅读/维护场景，零运行影响。

**影响**：格式卫生级，无功能/展示影响。

**修复方向**（若做体验整治）：改「该按钮形态只影响自身」即可（一行）；两处格式瑕疵顺手对齐（Select.tsx:379 缩进 4→2、Dashboard.tsx:501 换行）。零风险。

**裁决**：OBSERVE 记录。ponytail：格式瑕疵收益率近零，仅在 N-1 文案整治顺带时清理。

### O-1.【延续】Toast viewport 滚动交互残余——滚轮需悬停 toast 上、滚动条不可拖、无自动滚动到底部

**本轮复核**：Toast.tsx:105 viewport `max-h-[80vh] overflow-y-auto pointer-events-none` + Root `pointer-events-auto`（F48-M1 设计）；Radix `hasToasts ? void 0 : "none"` 驱动实证延续。任何"修复"都破坏 F48-M1"仅 toast 可交互"语义；M-2 去重 + 3.5s 自消下同刻 >4 条概率趋零。**维持 OBSERVE 不动作**。

### O-2.【延续，连续五轮】useTickingCountdown 非法日期串 NaN 防御——是否值得一行 `!Number.isFinite`

**本轮复证（任务书重点 C 前半）**：`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`——非法串 → NaN → "NaN" 四位 + isExpired=false。两调用点（Dashboard.tsx:190-195 / Select.tsx:749-754）输入源全受控——`openTimeStr` 走后端 `OpenTime time.Time json:"open_time"`（scheduler.go:49 实证，无自定义 MarshalJSON）RFC3339、兜底 `new Date(number).toISOString()` 恒合法；本次再核 open_time 序列化链路（Go 标准 time.Time JSON → RFC3339 → `new Date` 合法）无任何可注入不可信串的路径。Safari 严格模式非 ISO 串是唯一理论点，本项目不存在该输入形态。修复一行会引入 isExpired 语义争议（Select.tsx:826 "本地已到开窗点"横幅）。**裁决维持 OBSERVE 不动作**（若未来开放时间来源扩展为不可信字符串，先补 `!Number.isFinite` 再上线）。

### O-3.【延续】手写模态无焦点陷阱 + F6-02 锚点注释
三处手写 modal（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）均有 role=dialog/aria-modal + Esc 关闭 + autoFocus，但均无完整焦点陷阱（Tab 可逃出弹层）。grep `Radix Dialog|F6-02` 仅命中 Login.tsx:222 一处锚点注释。维持 OBSERVE。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询
React bailout 使叶子不重渲、实际 DOM 写入仅倒计时文本节点；Radix TabsContent 懒渲染（`present && children`）非激活 Tab 无观察者不轮询——管理员停留任一 Tab 时轮询 = 该 Tab 单查询（codes 5s / stats 5s / accounts 10s / logs 5s 之一；ConfigTab 无轮询）。维持 OBSERVE。

### O-5.【延续，连续五轮】Dashboard key 不对称 + expandedDates 跨账号残留——是否值得一行 key 修复

**本轮复证（任务书重点 C 后半）**：App.tsx:332 Dashboard 挂载点无 `key={account}`（Select 两处 App.tsx:294/340 有）——账号切换原地重渲，`expandedDates`（Dashboard.tsx:268-273）与 `extrasOpen`（212）残留旧账号折叠态。后端侧 `StateForAccount`（scheduler.go:724-727 按 `c.Account == acct` 过滤 Courses）与 `handleElectives` 的 `allowAccountOverride + accountExists` 凭据表校验（handler.go:243-252）双证数据不串线，跨账号残留最多是"折叠状态"而非"数据串线"。**裁决维持 OBSERVE**（纯展示层、触发窗口窄；若做切号视觉一致性整治，一行 `key={account}` 即可）。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button/Badge 死变体
grep 复证：`CardFooter` 仅 Card.tsx:46/51 自身定义，全仓零消费；Button 12 变体（Button.tsx:7）实际 `variant=` 传值仅 primary/outline/ghost/dark/destructive 子集、Badge 8 变体（Badge.tsx:5）实际仅 primary/outline 子集。维持 OBSERVE。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页生命周期
无 storage 事件监听；多标签页共享 localStorage 的 B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持 OBSERVE。

### O-8.【延续】401 保护窗口闭包 current + 双形态按钮
App.tsx:187-235 onUnauthorized 闭包捕捉 current、effect 依赖 [adminName, inAdmin, adminToken] 重建刷新闭包；`lostRaw = detail?.session || detail?.account || current`——session 恒为 401 真实主体（业务请求恒带 Bearer），current 兜底永不触发，陈旧闭包无实际影响。Select.tsx:1090-1156 官网 btn_type / 冲刺双轨按钮设计意图确认。维持观察。

---

## 重点核对结论（任务书逐条裁决）

### A. N-1（冲刺文案）/ N-2（调度延迟）→ 连续观察后裁决**维持**（见 N-1 / N-2 详述）

核心三连：① 前端无黄金期精确感知 → 三态文案是伪精确（负收益）；② 黄金期后用户主通道是官网「报名」primary 大按钮（Select.tsx:1110-1122），冲刺 ghost 小按钮是次要入口；③ 若治，极简方向 = 统一「后台目标」删"冲刺"二字（一行文案全时段语义成立），当前零行为影响不做。N-2 与 N-1 同源（冲刺按钮语义 = 后台预选入口），"至多一拍调度延迟"是时机感知级描述，非行为缺陷。

### B. Select 保存链六防全路径复证（任务书重点 B）——逐字符零回归

web/src 自 R54 修复后（含 R56 新视角复核）连续五轮未被触碰，本轮**逐字符通读 + 关键时序推演**。全部落位且语义正确：

| 防护族 | 通读复证结论 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 双参三消费点（防抖 Select.tsx:676 / flushTargets 499 / handleBack 589）判据同源；`stateData===undefined || (courses 非空 && hasSelected)`——首帧带旧目标但用户全清空（hasSelected=false）→ 放行 PUT []。16 项断言全绿 |
| **F42-M1 判据与数据源解耦** | 纯数据判据不依赖 echoedRef（防抖 effect 依赖含 stateData，Select.tsx:737—/state 到达触发重跑自愈）；flushTargets 与 handleBack 消费时刻读 `stateDataRef.current` 快照（499/589/676）。零死锁路径：推演"PUT [] 在飞 + 用户立刻点新课"→ 置脏 → savingRef 串行化 finally 补发（455-458）→ 收敛，无丢失 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（回显 287 / 独立清理 effect 316-327 / 防抖 696 / flush 518）全部在；独立清理 effect 依赖含 selected（327）覆盖"回显合并带出 stale"时序；toast 判 `!unmountedRef.current` 不轰炸卸载后（520/698）；cleanStaleSelected 无变更返回原引用（targetGuard.ts:42） |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19：`arr !== undefined && arr.length > 0`）——场景 C/D 断言覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:294/340 双 keyed + Select accountKey 守卫（195-202）声明于 echoedRef/rev 之后 TDZ 安全；TDZ 复查：防抖 effect（645-737）引用 selectedCount（739 声明）**仅在异步 timer 回调内**，effect 同步体只定义 build/timer，回调 400ms 后执行时声明已完成——无 TDZ |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow。viewport 不在空壳 wrapper 上 |
| **Toast 去重语义** | 去重 key = `typeof title === "string"`；全仓 grep 复核 17 个 toast 调用点（Select 10 + Admin 7）title 全字符串（本轮 grep 实证）；合并保留原 id → React key 稳定；M-B duration 保留首值 |

### C. O-2（NaN 防御）与 O-5（Dashboard key 不对称）→ 连续五轮观察后裁决**维持**（见 O-2 / O-5）

- **O-2**：open_time RFC3339 链路（Go `time.Time` JSON）与 begin_times 毫秒数双输入源全受控，结构性无非法串入口；修复一行引 isExpired 语义争议。**维持**。
- **O-5**：影响 = 折叠态跨账号复用纯展示层（后端双证不串线）；一行 `key={current}` 零成本收益 = 折叠态干净。**维持**（切号视觉一致性整治时一并加）。

### D. 全包逐行通读找新问题（任务书重点 D）

**目标保存链 / 回显合并 / 手动报名协同 / Admin 五 Tab / 401 处理 / Toast 去重 / 多账号年级隔离前端侧 / 悬浮栏 / 倒计时逐区通读，唯一新观察 = N-3（英文注释残留 + 两处既有格式瑕疵复核），其余零缺陷。**

补充验证（消费点对后端契约的依赖全部成立）：
1. **`?account=` 穿透全路径凭据表校验**：handleElectives（handler.go:243-252）/ handleElectiveSelect（293-301）/ handleElectiveExit（366-374）/ handleSetTargets（437-476）/ handleState（526-536）五路全部先 `allowAccountOverride`（1057-1059，仅管理员会话令牌可穿透）再 `accountExists`（1064-1075）——幽灵账号整体拒绝，前端 `?account=encodeURIComponent(account)` 穿透 URL 与后端校验兼容。
2. **手动报名/退选双端幂等**：后端 `TryAcquireSubmit` 拒并发（handler.go:318-323/391-396）→ `CheckClassSelectable` 快照复核 → `SelectClass`/`ExitClass` 平台真实调用 → `MarkDone`/`RemoveDone`；前端 `actionLoading` Set<number> 按课程独立跟踪（Select.tsx:52/92-93/106-110/119-120/131-135）——报名/退选各自函数式删除只清自己的 id，绝不抹掉其他课程在飞标记。双端幂等成立。
3. **401 单广播三形态**：client.ts:64-69（HTTP 401 前置广播，防网关非 JSON 体）+ 75-95（body 401 仅在 `r.status !== 401` 时补广播）——HTTP401+body401 / 网关非 JSON / 旧式 HTTP200+body401 三种形态各单次；App.onUnauthorized 幂等（188-231）。
4. **Admin 五 Tab**：CodesTab removing Set 按码独立跟踪（311/345-350/361-365）+ generating 守卫（327-328）；ConfigTab F18-02 `!loaded` 拒绝保存（513-519）+ R27-01 refetch 成功才自增 epoch（537-540）+ apiKey 刻意不回填（脱敏语义，503-511 + 保存时 `apiKey.trim()` 才传 530）；StatsTab 三态窗口（719 `s.window_closed ? 已关闭 : s.window_opened ? 已开放 : 待命中`）+ open_time_set `!== true` 保守未识别（714）；AccountsTab targets/success 空数组兜底（817-818）；LogsTab limit=200（879）。零缺陷。
5. **多账号年级隔离前端侧**：Dashboard dateGroups 输入源 = `state.courses`（StateForAccount 按 acct 过滤，scheduler.go:724-727）+ `/electives` 专属快照（ElectivesSnapshotFor 回退链决策锚 8）——前端无跨账号数据串线；Select 挂载点 `key={account}` + accountKey 守卫双保险；Admin 各查询 queryKey 含 account（317/494/703/790/878）。
6. **手动退选 refused 语义对齐**：Dashboard 状态机（586-616）与后端 status 字段（pending/in_range/submitted/success/failed，scheduler.go:38）全枚举对应；`isFullFallback` 用 `result.includes("已满员")` 匹配 markFullLocked 文案「该课程已满员，退避至下一备选」（scheduler.go:1753）——`includes("已满员")` ⊆ 该文案，匹配成立。零遗漏。
7. **倒计时条件顺序正确**：Select.tsx:809-835「未识别到开放时间」（821）先于 `cd.isExpired`（826）——target=null（识别缺席 + begin_times 空）时走"未识别"而非误显"本地已到开窗点"；Dashboard 文案行（399-419）同款顺序正确。右栏（Select.tsx:837-843）三态兜底（识别值 → begin_times[0] → 未知）与 R54 M-D 修复一致。
8. **刷新/回显/卸载生命周期交错**：unmountedRef StrictMode 挂载复位（398-407）+ 卸载置位清退避 timer；echoedRef 曾置位永不重放（159-162）；selectedRef/revRef/stateDataRef 镜像 ref 各消费时刻读取（480-518）——后三 ref 每渲染同步（173/175/180），闭包捕获旧值问题经 ref 通道消除。零缺陷。

### E. 上轮观察项延续复核（全表，任务书重点 E）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| N-1（冲刺文案） | OBSERVE（R57 新增，R58 延续） | 双端复证：文案恒"后台冲刺"，后端冲刺仅 10s 黄金期；pick() 只设目标无立即报名；前端无黄金期感知 → 三态伪精确，统一"后台目标"一行可治但零收益 | **延续**（第 3 轮） |
| N-2（调度延迟） | OBSERVE（R58 新增） | 开窗后 pick → 至多一拍（250ms/1s）提交延迟，时机描述级 | **延续**（第 2 轮） |
| O-1（Toast viewport 滚动） | OBSERVE | 机制实证逐字节一致，修复破坏 F48-M1，触发概率趋零 | **延续** |
| O-2（NaN 防御） | OBSERVE | 输入源全受控合法（RFC3339/毫秒数）；修复一行引 isExpired 语义争议 | **延续** |
| O-3（手写模态无焦点陷阱） | OBSERVE | 三处 modal 均无焦点陷阱，F6-02 锚点仅 Login.tsx:222 | 延续 |
| O-4（每秒重渲 + Admin 单查询） | OBSERVE | bailout + TabsContent 懒渲染实证延续 | 延续 |
| O-5（Dashboard key 不对称） | OBSERVE | 影响=折叠态跨账号复用纯展示层（后端双证不串数据）；一行 key 零成本 | **延续** |
| O-6（ui 模板残宽） | OBSERVE | CardFooter 零消费 + 死变体 grep 复证 | 延续 |
| O-7（多标签页） | OBSERVE | 无 storage 同步，方向安全 | 延续 |
| O-8（401 闭包 + 双形态按钮） | OBSERVE | 保守方向确认；current 兜底永不触发 | 延续 |
| N-3（本轮新增） | — | English 注释残留 + 两处既有格式瑕疵复核 | 新增（OBSERVE） |

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 10（N-1 延续第 3 轮 + N-2 延续第 2 轮 + N-3 本轮新增 + O-1~O-8 延续）**。tsc exit 0、build 成功（1948 modules / 417.54 kB js / 41.07 kB css）、TDD 断言 16+6+5 全绿、web/src 与 HEAD 逐字节一致（R55→R59 连续五轮零提交基线，本轮 `git log 899e8c3^..HEAD -- web/src/` 实证零输出）。
- **最致命 3 条（按影响排序）**：
  1. （本轮无真实缺陷）首要关注 = **N-1：「后台冲刺」按钮文案与后端冲刺实际逻辑（仅开窗后 10 秒黄金期 250ms）的描述性出入**——纯文案、零行为影响，窗口开放中后台提交完全正常（1s 常态）；连续三轮观察后裁决维持，若未来整治极简方向 = 统一「后台目标」删"冲刺"二字（三态文案是前端无法感知黄金期的伪精确，明确否决）。
  2. **O-5：Dashboard 挂载点无 key={account}**——expandedDates/extrasOpen 跨账号残留纯展示层；若做切号视觉一致性整治，一行加 key 即可。
  3. **O-2：useTickingCountdown NaN 防御**——当前输入源全受控合法（Go RFC3339 / 毫秒数）；若未来开放时间来源扩展为不可信字符串先补 `!Number.isFinite` 再上线。
- **已核对无缺陷的高风险区域**：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读 + 关键时序推演零回归（含"PUT [] 在飞 + 用户点新课"串行化收敛推演、TDZ 复查）；`?account=` 穿透五路凭据表校验与前端契约对齐；手动报名/退选双端幂等（Set<number> 独立跟踪）；401 三形态单广播；open_time RFC3339 序列化契约前后端一致（NaN 理论触发点不存在）；多账号年级隔离前端侧零串线；倒计时三处兜底（矩阵/横幅/右栏）同源一致；refused 后端文案（"该课程已满员，退避至下一备选"）与前端 `includes("已满员")` 匹配成立。
- **建议优先修复方向**：本轮零修复项，全部 OBSERVE 无需动作。连续五轮（R54 五修后）前端防线稳定，下轮可转向极低频残余（O-5 一行 key / N-1 文案删"冲刺" / O-2 防御一行 / N-3 注释卫生）作为体验整治候选，或维持纯观察。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | 成功（1948 modules / 417.54 kB js / 41.07 kB css，dist 落 backend/web/dist 已忽略） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | 16/16 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 工作树一致性 | `git status --short` | 零输出（含 archive/ 下 121 个已跟踪文件，与基线一致） |
| web/src 零改动 | `git diff HEAD -- web/src/` | 0 行 |
| 跨轮次零提交 | `git log 899e8c3^..HEAD -- web/src/` | 零输出 |