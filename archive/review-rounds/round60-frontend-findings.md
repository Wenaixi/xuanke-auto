# round60 前端只读审查发现报告

> 审查基线：master @ `eaff50a`（R59 收官，web/src 零修复轮）。本轮开局 `git status --short` 确认工作树干净（清洗 dist/ 构建产物与报告写入确认前后共三次）；`git log eaff50a^..HEAD -- web/src/` **零输出**、`git diff HEAD -- web/src/` **零改动（0 行）**——R55→R60 连续六轮 web/src 未被触碰。审查期间绝对只读（唯一新建文件为本报告）。
> 校验实况：`cd web && npx tsc -p tsconfig.app.json --noEmit` **exit 0**；`npm run build` **exit 0**（tsc -b + vite 全绿，1948 modules / 417.54 kB js / 41.07 kB css，dist 落 backend/web/dist 已忽略，构建后 status 仍零输出）；TDD 断言 `npx jiti scripts/target-guard-check.ts` / `admin-auth-check.ts` / `unauthorized-check.ts` 全绿（16+6+5）。
> 范围 `web/src/` 全部 19 个 `.ts/.tsx`；后端契约对照 `backend/internal/{api/handler, scheduler/scheduler}.go`。

## 本轮结论先行

**MAJOR 0 / MINOR 0 / OBSERVE 11（N-1 延续第 4 轮 + N-2 延续第 3 轮 + N-3 延续第 2 轮 + O-1~O-8 延续）。** web/src 连续六轮零提交后依旧零缺陷：六防保存链（F43/F42/F40/F39/F36/F48-M1）逐字符通读 + 关键时序推演零回归；任务书新视角 A-E 全部经实证裁决（A 维持 + 本轮给出「统一后台目标」一行具体落点；B 零回归 + 复查 379 行缩进、501 行折叠、1123 英文注释三处格式瑕疵仍原样在位——确认连续轮次"格式卫生零动作"是从未加重而非从未看见；C 维持；D 全包逐行通读**零新增缺陷**——本轮将 R55「全仓 toast title 全字符串」的 grep 口径扩宽为 `title:` 裸匹配实证复核，含 Select.tsx:363 三目 title 两分支均普通字符串字面量、全部 17 个调用点 title 求值恒为字符串，去重 key 无任何碰撞面，**不成立任何新观察**，比 R59 记录的"17 个调用点 title 全字符串"多一次字形级实证；E 逐条延续）。

**最致命 3 条（按影响排序）**：本轮无真实缺陷。关注序：① N-1（冲刺文案）+② O-5（Dashboard key）+③ N-3（注释卫生）。

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

## OBSERVE

### N-1.【延续，连续四轮】「冲刺」文案与后端仅 10 秒黄金期冲刺的实际行为出入——维持，本轮补一行落位点

**证据链**：scheduler.go:70-72 `submitIntervalSprint=250ms / submitIntervalNormal=1s / sprintDuration=10s`；submitIntervalFor（290-294）黄金期判定 `now.After(open) && now.Before(open.Add(sprintDuration))`。前端 Select.tsx:1126-1135 开窗按钮文案 `已设为后台冲刺${priorityName(selIdx)}`。

**本轮裁决**：前端无黄金期精确感知（三态伪精确，明确否决做）；若未来体验整治，极简方向 = 统一「后台目标」删“冲刺”，**一行落位** Select.tsx:1134 `已设为后台冲刺${...}` → `后台目标${...}`；同时 pick 区的 toast（352-369）「已设为首选/已设为备选目标」已是中性词——无需改。N-2（调度延迟）同源，维持；按钮 title 顺带补「后台提交，至多 1 秒内生效」可闭合。

### N-2.【延续，第三轮】开窗后 pick → 至多一拍调度延迟（250ms 黄金期 / 1s 常态）。维持。

### N-3.【延续，第二轮】三处格式卫生（英文注释残留 + 两处瑕疵），本轮复核仍在位：

1. Select.tsx:1123 注释尾「only affects itself」英文残留；
2. Select.tsx:379 `const lastJson = useRef("")` 缩进 4 空格（邻接 380-382 皆 2 空格）；
3. Dashboard.tsx:501 `</div>                <div>` 同行折叠。

本轮完整复核确认第 2、3 项是 R59 首次记录时的**既有**瑕疵（非本轮引入）；并确认连续轮次「维持不动作」是从未加重而非从未看见。样式/注释卫生级，零运行影响。若做体验整治，三项各一行、零风险。

### O-1.【延续】Toast viewport 滚动交互残余。Toast.tsx:105 `overflow-y-auto pointer-events-none` + Root `pointer-events-auto`（F48-M1）——修复破坏「仅 toast 可交互」；M-2 去重 + 3.5s 自消下同刻>4 条概率趋零。维持。

### O-2.【延续，连续六轮】useTickingCountdown NaN 防御（`useTickingCountdown.ts:20` `diff = target ? new Date(target).getTime() - now : 0`）——输入源全受控合法（`open_time` 走 Go `time.Time` JSON RFC3339，无自定义 MarshalJSON；兜底 `new Date(number).toISOString()` 恒合法）。本轮再核 open_time 序列化链路依旧无不可信串入口。修复一行引 isExpired 语义争议。**维持**（未来开放时间来源若扩展为不可信字符串，先补 `!Number.isFinite` 再上线）。

### O-3.【延续】三处手写 modal 无完整焦点陷阱（Login.tsx:224-316 / Select.tsx:1182-1231 / Admin.tsx:210-284）。均已有 role=dialog/aria-modal + Esc + autoFocus。维持。

### O-4.【延续】useTickingCountdown 每秒整页重渲 + Admin 单查询轮询——bailout + TabsContent 懒渲染实证延续。维持。

### O-5.【延续，连续六轮】Dashboard 挂载点无 `key={account}`（App.tsx:332，Select 两处 294/340 有）→ `expandedDates`（268-273）/`extrasOpen`（212）跨账号残留。后端 `StateForAccount`（704-730 按 `c.Account == acct` 过滤 Courses）双证数据不串线，影响纯展示层。**维持**；切号视觉一致性整治时一行 `key={account}` 即可。

### O-6.【延续】ui 模板残宽：CardFooter 零消费 + Button 12 变体/实际子集、Badge 8 变体/实际子集。本轮 grep 复核依旧。维持。

### O-7.【延续】xk_admin_token/xk_admin_name 多标签页——无 storage 监听，B 标签登出清标记 → A 刷新回学生端。方向安全（永不误进管理页）。维持。

### O-8.【延续】401 保护窗口闭包 current：`lostRaw = detail?.session || detail?.account || current`，session 恒为 401 真实主体、current 兜底永不触发；effect 依赖 [adminName, inAdmin, adminToken] 每改重建闭包，陈旧闭包无实际影响。维持。

**附加实证（本轮）**：对 Toast.tsx:40 去重 key（`typeof msg.title === "string"`）的**全部 17 个 toast 调用点**（Select 10 + Admin 7）做 `title:` 裸匹配字形级枚举——全部为普通字符串/三目普通字符串（Select.tsx:363 三目两分支均为 `"已设为首选"/"已设为备选目标"` 字面量），无模板字面量、无变量、无 ReactNode；key 恒为字符串，M-2 去重按字符串合并语义成立，无任何碰撞面。维持。

---

## 重点核对结论（任务书逐条裁决）

### A. N-1 / N-2 连续观察（第 4 / 3 轮）——维持，落位点明确

三连：① 前端无黄金期精确感知 → 三态文案伪精确（负收益，明确否决）；② 黄金期后用户主通道是官网「报名」primary 大按钮（Select.tsx:1110-1122），冲刺 ghost 小按钮是次要入口；③ 若治 = 一行「后台目标」删“冲刺”（Select.tsx:1134）。N-2 同源，「至多一拍延迟」是时机感知级描述非缺陷。

### B. Select 保存链六防全路径复证（逐字符零回归）

| 防护族 | 本轮结构与语义复证 |
|---|---|
| **F43 全清空 ≠ 数据缺席** | targetGuard.ts:57-61 `shouldDeferSave(stateData, hasSelected)` 三消费点同源（Select.tsx:676/499/589）；断言 6 项覆盖全清空放行；`select-none` 页根与输入交互互不干扰 |
| **F42-M1 判据解耦** | 纯数据判据不依赖 echoedRef；防抖 effect 依赖含 stateData（737）自愈；sets 直连 publish 过滤与消费时刻双闸判据同源 |
| **F40-M1 targetGuard 三角** | selectedHasStalePublish/cleanStaleSelected 四消费点（287/316-327/696/518）全在；独立清理 effect 依赖含 selected；toast 判 `!unmountedRef.current` 不轰炸卸载后 |
| **F39-C1 selectedHasStalePublish** | 空 key 绝不判过期（targetGuard.ts:9-19）；断言场景 C/D 覆盖 |
| **F36-01 key={account} 双保险** | App.tsx:294/340 双 keyed + accountKey 守卫（195-202）TDZ 安全 |
| **F48-M1 Toast 定位** | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` 定位类未动；M-C 仅追加 max-h/overflow |

关键时序推演（均无丢失路径）：**「PUT [] 在飞 + 用户立刻点新课」**→ recovery（lastJson 不更新 → 防抖再 PUT）→ 收敛；**「全清空 PUT 在飞 + 回显旧 courses 到达」**→ merged=false 且 hasTouched=false → 返回 prev 放行 → 无复活；**「发布重建 + /state 首帧交错」**→ 合并带出 stale → 清理 effect 随 selected 重跑清掉 → 防抖落库；**「type=number 输入非法字符串（concurrency）」**→ `Math.min(16, Math.max(1, NaN || 1))` → 1 兜底，e.g. `2e` 态时值=1 落库下游 `Math.max(1, concurrency||1)` 再兜。**TDZ**：防抖 effect（645-737）同步体不引用后声明物；`const lastJson` 缩进瑕疵不改语义。

### C. O-2 / O-5 —— 维持（第 3/6 轮，见 O-2/O-5）

### D. 全包逐行通读找新问题

**结论：零新增缺陷。** 本轮对 R55 已核的「toast title 全字符串」做字形级复核（17 个调用点 `title:` 裸匹配 + 三目两分支实证），确认无任何碰撞面——不成立新观察。补充实证：

1. **`?account=` 五路凭据表校验**：handleElectives/handleElectiveSelect/handleElectiveExit/handleSetTargets/handleState（handler.go 243-252 / 293-301 / 366-374 / 437-476 / 526-536）全部先 allowAccountOverride（1057-1059）再 accountExists（1064-1075，LoadCredentials 逐账号）。**处理 Allow：普通会话如遇 `?account=`（管理员语义透传）前后端契约一致**。
2. **手动报名/退选双端幂等**：后端 TryAcquireSubmit 拒并发 + 快照复核 + SelectClass → MarkDone；前端 actionLoading Set<number> 独立跟踪、函数式删除只清自己 id（Select.tsx:52/92-93/106-110/119-120/131-135）。
3. **401 三形态单广播**：client.ts:64-69（HTTP 401 前置）+ 75-95（body 401 仅 `r.status !== 401` 补）；App.onUnauthorized 幂等。
4. **Admin 五 Tab**：CodesTab removing Set 按码独立/ConfigTab `!loaded` 拒存 + refetch 成功才自增 epoch/StatsTab `window_closed` 三态 + `token_valid` 部分失效/AccountsTab targets 空数组兜底/LogsTab limit=200。零缺陷。
5. **多账号年级隔离前端侧**：StateForAccount 704-730 按账号过滤 Courses；ElectivesSnapshotFor 回退链；Select keyed 双保险；Admin 各 queryKey 含 account。
6. **手动退选 refused 语义对齐**：Dashboard isFullFallback `result.includes("已满员")` 匹配 markFullLocked 文案；状态机 586-616 对后端 status 枚举全覆盖。
7. **倒计时条件顺序**：Select.tsx:809-835「未识别」分支（821）先于 `cd.isExpired`（826）；Dashboard 文案行同款顺序正确。
8. **刷新/回显/卸载生命周期**：unmountedRef StrictMode 复位 + F20-01；echoedRef 永不重放；selectedRef/revRef/stateDataRef 每渲染同步。

### E. 上轮观察项延续复核（全表）

| 编号 | 本轮核实 | 裁决 |
|---|---|---|
| N-1 | scheduler.go:70-72/290-294 冲刺实测 + Select.tsx:1134 落位点 | **延续**（第 4 轮） |
| N-2 | pick→PUT/targets→tick 提交链路 | **延续**（第 3 轮） |
| O-1~O-8 | 逐项复核机制实证相同（见上），无新增变化 | **延续** |
| N-3 | 三处格式瑕疵再核在位 | **延续**（第 2 轮） |

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 11（N-1 第 4 轮 + N-2 第 3 轮 + N-3 第 2 轮 + O-1~O-8）**。tsc exit 0、build exit 0（1948 modules / 417.54 kB / 41.07 kB css）、三个 TDD 断言 16+6+5 全绿、web/src 与 HEAD 逐字节一致（R55→R60 连续六轮零提交基线实证）。
- **最致命 3 条**：
  1. **N-1：「后台冲刺」文案 vs 后端仅 10 秒黄金期 250ms 冲刺**——纯文案、零行为影响；连续四轮维持。若整治 = 一行 Select.tsx:1134 统一「后台目标」。
  2. **O-5：Dashboard 无 key={account}**——expandedDates/extrasOpen 跨账号残留纯展示层；一行加 key 即可。
  3. **N-3：格式卫生（English 注释 + 两处排版）**——均零行为影响；能回的最矮扳手 = 1123 注释一行 + 379/501 排版对齐。
- **已核对无缺陷的高风险区**：六防保存链逐字符零回归（含「PUT [] 在飞」「全清空 PUT 在飞」「发布重建交错」三个时序推演）；后面板 5 Tab 零缺陷；`?account=` 五路凭据表校验；401 三形态单广播；倒计时三态；多账号隔离。

## 验证实证表

| 验证项 | 命令 | 结果 |
|---|---|---|
| TypeScript 类型检查 | `cd web && npx tsc -p tsconfig.app.json --noEmit` | exit 0 |
| 前端生产构建 | `cd web && npm run build` | exit 0（1948 modules / 417.54 kB js / 41.07 kB css） |
| targetGuard 断言 | `npx jiti scripts/target-guard-check.ts` | 16/16 全绿 |
| adminAuth 断言 | `npx jiti scripts/admin-auth-check.ts` | 6/6 全绿 |
| unauthorized 断言 | `npx jiti scripts/unauthorized-check.ts` | 5/5 全绿 |
| 工作树一致性 | `git status --short` | 报告写入与构建后均零输出 |
| web/src 零改动 | `git diff HEAD -- web/src/` | 0 行 |
| 跨轮次零提交 | `git log eaff50a^..HEAD -- web/src/` | 零输出 |
| 报告落盘核验 | `Read round60-frontend-findings.md` | 文件已写、正文完整（含验证实证表） |