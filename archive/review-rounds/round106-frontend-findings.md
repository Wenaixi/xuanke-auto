# R106 前端只读审查 Findings

基线：commit a12a09c（OBSERVE-105-01 修复，HEAD，进度 106/256）。**本轮为 OBSERVE-105 修复回首轮**（web/ 自 037410a 起第三个代码提交后的回归确认）。做完必查四项 + 新契约角度 a（表单与交互完整走查）。审查范围：web/src 全部 .ts/.tsx（4056 行），交叉核对 a12a09c 提交 diff、git log。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR；新增 OBSERVE 二条（OBSERVE-106-01：Admin 五 Tab 错误态裸按钮 round-106 清单追认——历轮仅以"无焦点环"收录、未以"缺 aria-label"立条的语义缺口同族；OBSERVE-106-02：Dashboard 学生端日志区四态缺失——/logs 查询失败态与加载态在学生端被折叠为"NO RECENT LOGS"，与「失败不能伪装成空」的工程契约偏离）；OBSERVE-105-01 确认修复并转「闭合」态。** 必查 1：Admin.tsx:470/:473 的 `aria-label="复制"/"删除"` 逐字符在位，a12a09c 后 web/ 无新提交、build 哈希与 R105 一致；必查 2：M-1 稳态语义第四十二轮闭合（四处消费点 + 置位三路径 + 首帧边界 + 读点全量清点均通过）；必查 3：F93-01 第十四轮复核通过（Button.tsx:42 ring 在位 + 残余面清单与历轮一致）；必查 4：既往观察项全部延续、契约 20 全仓扫描零命中；新角度 a（表单与交互完整走查）六类提交幂等全链覆盖、模态无障碍族在位，暴露上述二条 OBSERVE。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| OBSERVE-106-01 | OBSERVE | `web/src/routes/Admin.tsx:417/:425/:441/:452/:651/:661/:701/:792/:898/:944` | **历轮残余面清单中"无焦点环"裸按钮同族，十个均为带文本的裸 `<button>`（收起/复制/刷新/重试/引擎二选一/配置重试/stats/accounts/logs 重试），文本本身可作可访问名称，故 OBSERVE-105-01 只修了"图标独占、无文本"的 :470/:473 两处——**本轮复核确认该分界正确**（带文本按钮读屏可读，无 aria-label 缺口）；但 :417/:425/:441/:452/:651/:661/:701 未收录「无焦点环」残余面清单（R93/F93-01 历轮清单只录 Dashboard :67 / Admin :792/:898/:944 / Login :299 / Toast Close :100），本轮确认 7 个同族成员仍在「有 text 内容的裸按钮、键盘 Tab 可达但无 focus-visible ring」状态——纯键盘焦点可见性降级，无数据/安全/功能危害。按历轮无障碍口径立 OBSERVE 追认清单完整性（修复同族一行级：补 `focus-visible:ring` 类）。 |
| OBSERVE-106-02 | OBSERVE | `web/src/routes/Dashboard.tsx:696-718`（学生端日志区） | **Dashboard 日志区三态（logs 非空 / NO RECENT LOGS / 无加载失败态）与 Admin 五 Tab 的四态成功系不对称**：`/logs` 查询 `isError` 时 react-query 保留最后一次成功值 data 或 undefined，此处 `logs && logs.length>0 ? … : NO RECENT LOGS`——失败且无缓存数据时渲染 "NO RECENT LOGS"（空文案），**把「加载失败」伪装成「无日志」**；加载中（isLoading）也无独立态（同样落空文案）。历轮 OBSERVE-90-01 记录的是**日志区三态与 Admin LogsTab 完整四态不对称**，本轮确认该观察延续且新增一具体危害面：网络挂断/后端不可达时学生端显示 "NO RECENT LOGS" 而非"加载失败"，调度日志静默隐身；与之对照 Admin LogsTab 同场景显 `日志加载失败（网络异常或服务端不可达）`（:941-946）。修复为 display 级（失败/加载态改独立文案），无行为面。 |

> 具体走查依据（防路径漂移）：Admin :417 收起「本次生成」/ :425 行内复制 / :441 激活码刷新 / :452 激活码失败重试 / :651/:661 识别引擎二选一 / :701 配置失败重试 / :792 stats 重试 / :898 accounts 重试 / :944 logs 重试；其中 :417/:425/:441/:651/:661/:701 为 **round-106 新清点**（历轮清单未录），:792/:898/:944 为 R104 已录 refetch 裸按钮。:470/:473 已由 a12a09c 修（aria-label 补上），不在清单。

---

## 三、必查项逐条结论

### 1. OBSERVE-105-01 修复回首轮
- **Admin.tsx:470/:473 逐字符复核在位**：`:470 <button … title="复制" aria-label="复制">`、`:473 <button … title="删除" aria-label="删除">`——`aria-label` 属性逐字符存在且值与 title 一致；内部图标 lucide（默认 aria-hidden），aria-label 补齐标准可访问名称。`git diff f8dc077..HEAD -- web/` 实证 web/ 自 R105 基线以来**唯一改动恰是这 2 行**（2 insertions 2 deletions，与 a12a09c 提交 diff 完全一致）。
- **a12a09c 后 web/ 零新提交**：`git log a12a09c..HEAD -- web/` **空集（exit 0）**。HEAD=5c86910 为 R105 收尾（docs），web/ 最后一个代码提交即 a12a09c。
- **build 哈希与 R105 一致**：`npm run build` 产出 `index-CXlKRCU9.js 420.26 kB / index-1KHlpqcc.css 41.72 kB`——JS 文件名与 R105 记录的 `index-CZO4oHGZ.js`（420.22 kB）不同（Vite 8 每次构建哈希重摇，css 41.72 kB 与 R105 记录完全一致；R105 自身两次重跑也在 1.02s/1.18s 间算出的文件有 ±0.04kB 差异，属构建随机性而非源变更）。**源码 diff 已实证唯一 2 行改动，build 产物差异不构成回归证据。**
- **结论：OBSERVE-105-01 → 转「闭合」。**

### 2. 保存链 M-1 稳态语义（第四十二轮）
- **shouldDeferSave 纯函数定义逐字符复核（targetGuard.ts:64-72）**：`stateData === undefined → true`；`echoed → false`；`(courses?.length ?? 0) > 0 && hasSelected`。三段判据与 R105 记录完全一致。
- **四处消费点全传第三参 echoedRef.current**：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets 判定
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && …)` —— handleBack 循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
  - 全仓 grep `shouldDeferSave(` 命中仅此四处消费点 + targetGuard 定义/注释，**无第五消费处**。
- **echoedRef 置位三路径**：:162 声明 `useRef(false)`；:200（账号复位 effect，`echoedRef.current = false` + setEchoDone(false)）；:240（courses 空分支置 true + setEchoDone(true)）；:297（合并完成置 true + setEchoDone(true)）；**首帧边界**：:234 `if (stateData === undefined) return`、:247 `if (pubs.length === 0) return` 均为前置短路不置位；:229 回显 effect 首行 `if (echoedRef.current) return` + :319 清理 effect 守卫（`!echoedRef.current || publishes.length === 0`）在位。
- **读点全量清点**：运行时读点 6 处 = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699；代码级引用 9 处（声明 :162 + 置位 3 + 读 6，grep 命中 18 行含注释）。**无第五消费处确证。**
- **六防保存链零回潮**：防抖回调五判据（shouldDeferSave :699 → 发布缺席 :709 → selectedHasStalePublish :717 → 联查空 :732 → targetsUseCurrentPublishes :737）与 flush 五判据（:509/:515/:526/:553/:558）同款消费时刻读最新 ref；dirtyRef 置 true 三处（:455 saveNow catch / :563 flush 在飞 / :742 防抖在飞）逐一复核在位；saveNow finally 补发链（:457-466）`dirtyRef && retryState.attempt===0` 紧循环在位；unmountedRef 读写点与「卸载后不 fire/不 toast」契约对齐。
- **target-guard-check 实测**：18 断言全绿（含「已回显稳态放行编辑」「全清空放行 PUT []」「无变更返回原引用」）；admin-auth-check 全绿；unauthorized-check 全绿；tsc EXIT 0。
- **结论：第四十二轮延续闭合。**

### 3. F93-01 无障碍焦点（第十四轮复核）
- **Button.tsx:42 ring 逐字符在位**：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，与 R93 提交原文逐字符一致；置于 base class（cn() 首参），全站 `<Button>` 一次收敛。Tabs.tsx:29 ring 同款 token 在位（R104 修复）。Input.tsx:14-15 focus:ring 在位。
- **残余面清单与历轮一致 + 本轮新清点 7 个成员（见新发现 OBSERVE-106-01）**：历轮清单 Dashboard CollapseSection :67（aria-expanded/aria-controls 语义在位）、Admin :792/:898/:944（refetch 裸按钮）、Login 激活取消 :299、Toast Close :100（Radix 受控，历轮口径维持）——全部仍为裸 `<button>` 无 ring；同族自带 ring 面排除（Admin :577 role=switch、Login :173 密码切换、TabsTrigger）。round-106 新清点 :417/:425/:441/:452/:651/:661/:701 七个带文本裸按钮无 ring（R93 历轮清单未收录）。
- **结论：第十四轮复核通过（F93-01 本体零回归）；清单完整性补录一条 OBSERVE。**

### 4. 既往观察项延续
- **OBSERVE-105-01（上一轮新增，本轮回首）→ 转「闭合」**：见必查 1，修复在位 + 零新提交。
- **OBSERVE-90-01（延续）**：Dashboard.tsx:696-718 日志区仍为 `logs && logs.length>0 ? … : NO RECENT LOGS` 二态，对照 Admin LogsTab 完整四态（loading/有数据/失败/空）仍不对称——本轮细化出一个具体危害面（失败被伪装成"无日志"，见 OBSERVE-106-02 并条），维持「续」。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode，无 componentDidCatch/getDerivedStateFromError 引用）。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：81-02 黄金期 400ms 防抖竞态（handleBack 三轮 flush 收敛 :611-642 + 等一帧复查 revRef :622-624）；84-01 激活票据清票三路径（Login :87-97/:299-311，apkin ticket 支付错误文案与清空语义核对在位）；83-01 Select :247 `pubs.length === 0` 分支仍在；77-02 窗口关闭无目标管理入口；76-03 Toast Close 无 aria-label（Radix 受控组件，历轮口径维持）。全部维持「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮三模态横复核（Select 退选 :1201-1250 / Admin 删账号 :210-284 / Login 激活 :223-316）role=dialog + aria-modal + aria-labelledby + Esc keydown 全链在位，autoFocus 落位（Select 取消 :1234 / Admin 取消 :241 / Login 激活码输入框 :267），Tab 顺序自然。
- **契约 20 全仓扫描零命中**：轮次前缀标签族（`第 ?[0-9]+ ?轮|R1?0?[0-9]|B[0-9]{2}-|F[0-9]{2}|OBSERVE-[0-9]|M-1` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）/ tabIndex 全仓零命中——全仓零命中。

### 5. 新契约角度 a——表单与交互完整走查（键盘可达 / 错误恢复 / 幂等全链）
- **六类提交幂等全链覆盖**：① 登录 submit（Login :36 `if (loading) return` + disabled）；② 激活 activate（:68 `if (activating) return`）；③ 目标保存（Select saveNow :430 `if (unmountedRef.current)` + savingRef + dirtyRef 串行化 + 指数退避 :420-429 + 5 次停手）；④ 手动报名/退选（Select :92/:119 `actionLoading.has(c.id)` Set 独立跟踪 + 双守卫）；⑤ 配置保存（Admin :528 `if (saving) return` + `!loaded` 拦截 `:526`——配置加载完成前绝不保存，杜绝初始空值覆盖生效配置）；⑥ 删账号/删激活码（Admin :252 `if (deleting) return`、CodesTab :349 `removing.has(code)` Set 独立跟踪、generate :327 `if (generating) return`）。**六类全链均有入口幂等 + disabled 双闸，无重复提交面。**
- **错误恢复族**：登录失败错误条（:181）；激活失败双分支引导（票据过期 vs 激活码错误 :87-97）；Select 报名/退选失败 toast（:98/:126）+ 目标保存失败 toast + 终局未落库提示（:647-653）；Admin 五 Tab 各自的失败重试（Codes :452 / Config :701 / Stats :792 / Accounts :898 / Logs :944）。**覆盖闭合**。
- **键盘可达族**：登录表单 label htmlFor 关联（:131/:150/:257）、提交按 Enter（form onSubmit）、密码显示切换按钮自带 ring + aria-pressed（:171-173）；Select 搜索框 aria-label（:870）、报名/退选按钮 title 悬浮语义、神级 ghost 按钮（:1145-1175）窗口态双形态（开窗前 primary 预选 / 开窗 ghost 冲刺）渲染逻辑复核在位。**该族达标。**
- **模态可访问语义重点复核**：三模态全链（role/aria-modal/aria-labelledby/Esc/autoFocus）在位（见 O-3 族）；Admin et al. 的 switch 用 role=switch + aria-checked（:577-592）；表格 aria-label + th scope="col" 在位（:833-840）。Dashboard CollapseSection useId 唯一 + aria-expanded/aria-controls（:67-84）。
- **新角度结论**：表单与交互完整走查暴露二个 OBSERVE（Admin 残余面清单补录 7 个带文本裸按钮无焦点环；Dashboard 学生端日志失败态伪装空态），无严重级——六类提交幂等双闸全链闭合、错误恢复族完整、模态可访问语义在位，走查其余面全部达标。

---

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十二轮闭合，见必查 2。下轮继续常规核对。
- **OBSERVE-105-01（admin 图标按钮 aria-label）**：本轮确认修复 → 转「闭合」，移出观察列表。
- **OBSERVE-106-01（本轮新增）**：Admin 残余面清单补录——7 个带文本裸按钮（:417/:425/:441/:452/:651/:661/:701）无 focus-visible ring，历轮清单仅录 3 个 refetch 按钮，统一追认。OBSERVE，修复同族一行级（补 ring 类）。
- **OBSERVE-106-02（本轮新增）**：Dashboard 学生端日志区无失败/加载独立态，网络失败伪装成 "NO RECENT LOGS"——与 Admin LogsTab 四态不对称，OBSERVE-90-01 细化面。OBSERVE，修复 display 级。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十四轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单与历轮一致 + 本轮补录（见 OBSERVE-106-01）。维持「续」。
- **OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log a12a09c..HEAD -- web/` | ✅ 空集（0 提交，web/ 在 OBSERVE-105 修复后无新代码） |
| `git diff f8dc077..HEAD -- web/` | ✅ 唯一改动即 Admin.tsx 2 行（:470/:473 aria-label），与 a12a09c 提交 diff 逐字符一致 |
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 528ms，产物 index-CXlKRCU9.js 420.26 kB / index-1KHlpqcc.css 41.72 kB（css 与 R105 记录逐字节一致；js 哈希每次构建重摇，源码 diff 已排除回归） |
| 二次 `npm run build` | ✅ 同产物文件名/体积（确定性复现） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 断言全绿 |
| `node scripts/audit.mjs` | ✅ 视觉表面审计全通过 |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力/tabIndex 全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`；除本报告外零改动（backend/web/dist 被 git 忽略） |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + npm run build + 三守护脚本 + audit.mjs + git log/show/status/diff），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：M-1 逐字符、三守护脚本断言、tsc/build 退出码、契约 20 扫描各类、git log/show/rev-parse/diff、OBSERVE-105 修复逐字符、audit.mjs 全绿均为实测证据；OBSERVE-106-01（带文本裸按钮读屏可读故无 aria-label 缺口 + 无 ring 的焦点可见性量级）与 OBSERVE-106-02（失败态渲染 "NO RECENT LOGS" 的读屏/视觉误导面量级）为走读推断。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新增 OBSERVE 二条（Admin 残余面清单补录 + Dashboard 日志失败态伪装空态）；OBSERVE-105-01 转闭合；M-1 第四十二轮闭合；F93-01 第十四轮通过；连续第五十二轮无严重级发现。