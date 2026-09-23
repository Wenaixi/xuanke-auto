# R105 前端只读审查 Findings

基线：commit f8dc077（R104 双 findings + 收尾总结，HEAD，进度 105/256）。**本轮为 web/ 自 f08937e 后连续零漂移记录终结后的首次回归确认——037410a（OBSERVE-104-01/104-02 修复）是 web/ 首个代码提交**。本轮做 OBSERVE-104 修复回首轮 + M-1 稳态语义第四十一轮 + F93-01 第十三轮 + 新契约角度 b（无障碍纵深二查）。审查范围：web/src 全部 .ts/.tsx，交叉核对 037410a 提交 diff、global.css 还原。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR；新增 OBSERVE 一条（OBSERVE-105-01，无障碍纵深二查暴露的历轮清单外新面）；OBSERVE-104-01/104-02 确认修复并转「闭合」态。** M-1 稳态语义第四十一轮闭合：四处消费点全传 echoedRef.current 第三参逐字符复核通过、置位三路径 + 首帧不置位边界完整、读点全量清点（运行时读点 6 处，无第五消费处）、六防保存链零回潮。F93-01 第十三轮复核通过：Button.tsx:42 ring 保留 + 残余面清单与历轮一致（含 R104 补录的 3 个 refetch 裸按钮）。037410a 首次 web/ 代码提交内容与描述完全一致（Tabs ring 一行 + Admin 表格语义），git log 实证其后 web/ 零提交。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| OBSERVE-105-01 | OBSERVE | `web/src/routes/Admin.tsx:470/:473`（激活码列表行复制/删除图标按钮） | **图标按钮仅有 `title` 属性、无 `aria-label`**——内部只有图标（Copy / Trash2 lucide 默认 aria-hidden），可访问名称仅靠 `title` 文本。title 是鼠标悬停触发的原生提示，并非标准可访问名称来源（读屏用户默认不读 title、键盘用户无法悬停获取）。历轮残余面清单收录此二按钮为「裸按钮无焦点环」同族成员，但始终未立「无 aria-label」的读屏语义缺陷条——无障碍纵深的读屏语义族历轮已核查 aria 动态态/表格/模态/标签关联，独漏这处图标按钮族。修复为一行级（各补 `aria-label="复制"/"删除"`），与 Admin 表格 aria-label 族同源。触发面：激活码列表复制/删除，纯读屏可用性降级，无数据/安全/功能危害——按历轮无障碍增强口径立 OBSERVE。 |

> 备注（不立条，清点遗留）：历轮残余面清单中的裸按钮（无焦点环）同族面仍维持——Dashboard CollapseSection :67、Admin :417/:425/:441/:452/:470/:473/:651/:661/:701/:792/:898/:944、Login 激活取消 :299、Toast Close :100，均仍为裸 button 无 ring，与历轮一致。其中 :470/:473 属「图标按钮无 aria-label」新缺口（立 OBSERVE-105-01），其余为已知「无焦点环」族维持观察。

## 三、必查项逐条结论

### 1. OBSERVE-104-01/104-02 修复回首轮（web/ 零漂移终结后的首次回归确认）
- **Tabs.tsx:29**：ring 类逐字符复核**在位**——`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，与 Button.tsx:42 同款 token 逐字符一致；置于 base class（cn() 首参），**全站 Tab 一次收敛**（Select 发布 Tab :935-951 + Admin 五 Tab 主控同用 TabsTrigger）。语义正确——focus-visible 只在键盘 Tab 触发、鼠标点击不显示（WCAG 2.4.7 Focus Visible 对齐）。`git show 037410a -- web/src/components/ui/Tabs.tsx` 实证提交恰好是这一行，与 R104 建议一行级修复完全一致。
- **Admin.tsx:833-840**：`<table aria-label="账号列表">` + 四 `<th scope="col">`（账号/目标课程/已选成功/操作）**逐字符复核在位**，与 037410a diff 一致（提交 10 行改动仅 Tabs 行 + 表格语义族）。scope 与列对应关系正确。
- **037410a 之后 web/ 零提交**：`git log 037410a..HEAD -- web/` **空集（exit 0）实证**——首个 web/ 代码提交后无新代码，回归面聚焦在本轮复核的两处。

### 2. 保存链 M-1 稳态语义（第四十一轮）
- **四处消费点全传第三参 echoedRef.current 逐字符复核通过**：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets 判定
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` —— handleBack 循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
  - 第二参 hasSelected 语义各点对齐（flush/防抖用「>0」布尔、handleBack 用 hasSelectedNow() 纯函数，F43 清空分判契约落实）。`shouldDeferSave` 纯函数定义（targetGuard.ts:64-72）逐字符复核：`stateData===undefined → true`；`echoed → false`；`(courses?.length ?? 0) > 0 && hasSelected`。
- **echoedRef 置位三路径**：:200（account 变化复位 false + setEchoDone(false)）、:240（courses 空分支置 true + setEchoDone(true)）、:297（合并完成置 true + setEchoDone(true)）；**首帧边界**：:234 `if (stateData === undefined) return` 前置 return 不置位、:247 `pubs.length === 0` return 不置位（OBSERVE-83-01 立足点仍在）。:229 回显 effect 首行短路、:319 清理 effect 守卫在位。
- **读点全量清点**：运行时读点 6 处 = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699；代码级引用 9 处（声明 :162 + 置位 3 + 读 6，另 :755 依赖用 echoDone 非 echoedRef）。**考据无第五消费处**。
- **六防保存链零回潮**：防抖回调五判据（shouldDeferSave→发布缺席→selectedHasStalePublish→联查空→targetsUseCurrentPublishes，:699/:709/:717/:732/:737）与 flush 五判据（:509/:515/:526/:553/:558）同款消费时刻读最新 ref；dirtyRef 置 true 仅三处（:455 saveNow catch / :563 flush 在飞 / :742 防抖在飞）；saveNow 的 finally 补发链（:457-466：dirtyRef && attempt===0 才紧循环，失败重试走退避 timer）在位。unmountedRef 读写点与「卸载后不 fire/不 toast」契约对齐。
- **结论：第四十一轮延续闭合。** target-guard-check 脚本实测全绿（含「已回显稳态放行编辑」「全清空放行 PUT []」两个关键 TDD 断言）。

### 3. F93-01 无障碍焦点（第十三轮复核）
- **Button.tsx:42 ring 逐字符在位**：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，与 R93 提交原文一致；ring 色 `--cyan` = #ffffff（global.css:32 实证）纯白高对比，offset 2px + 深黑底，键盘聚焦清晰可见、鼠标点击不触发（focus-visible 语义）。全站 `<Button>` 经 base class 一次收敛。
- **残余面清单与历轮一致**：Dashboard CollapseSection :67、Admin :417/:425/:441/:452/:470/:473/:651/:661/:701/:792/:898/:944、Login 激活取消 :299、Toast Close :100——逐行 grep 复核均仍为裸 `<button>` 无 ring。同族自带 ring 面排除（Admin :577 role=switch 与 Login :173 密码切换）。**R104 补录的 3 个 refetch 裸按钮（:792/:898/:944）确认已入清单**。
- **结论：第十三轮复核通过，无回归。**

### 4. 既往观察项延续
- **OBSERVE-104-01 / 104-02（上一轮新立，本轮回首）→ 转「闭合」**：修复随首个 web/ 代码提交 037410a 落地，本轮逐字符复核在位（见必查 1），R104 建议的一行级修复与提交实现完全一致。从「续」转「闭合」。
- **OBSERVE-90-01（延续）**：Dashboard.tsx:697-718 日志区仍只有 `logs && logs.length > 0 ? … : NO RECENT LOGS` 二态，对照 Admin LogsTab 完整四态仍不对称。维持「续」不落地。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode，无 componentDidCatch/getDerivedStateFromError 引用）。维持「续」。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：81-02 黄金期 400ms 防抖竞态（handleBack 三轮 flush 收敛 :611-642 + 等一帧复查 revRef :622-624）；84-01 激活票据清票三路径（:87-90/:91-97/:232-238 + :299-311）；83-01 Select :247 `pubs.length === 0` 分支仍在；77-02 窗口关闭无目标管理入口；76-03 Toast Close 无 aria-label（Radix 受控组件，历轮口径维持）。全部维持「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮三模态横复核（Select 退选 :1201-1248 / Admin 删账号 :210-244 / Login 激活 :223-316）role=dialog + aria-modal + aria-labelledby + Esc keydown 全链在位，autoFocus 落位（Select 取消 :1234 / Admin 取消 :241 / Login 激活码输入框 :267），Tab 顺序自然。
- **契约 20 全仓扫描零命中**：轮次前缀标签族（`第 N 轮|R1?0?x|B/F/O[0-9X]{2}-|M-1|F\d{2}A?[A-Z]?\d` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。

### 5. 新契约角度 b——无障碍纵深二查（键盘可达性 / 读屏语义 / 焦点管理完整面）
- **键盘可达性族**：Button 全站收敛（上一节）；Input :14-15 focus:ring 在位；TabsTrigger（本轮已修）ring 在位；Admin :583 开关 / Login :173 密码切换自带 ring；所有可聚焦功能元素均可 Tab 到达，无 `tabIndex` 滥用（全仓 tabIndex 零命中）。**该族达标**。
- **读屏语义族**：role=switch + aria-checked（Admin :577）、aria-label + aria-pressed（Login 密码切换）、aria-expanded + aria-controls（Dashboard CollapseSection，useId 唯一）、表格 aria-label + scope（本轮已修）、模态 role=dialog + aria-modal + aria-labelledby（三处）、Progress aria-valuenow（unannounced 显式传 0 = 「未公布无进度」契约落实）。**本轮新增清点**：Admin 激活码列表复制 :470 / 删除 :473 图标按钮只有 title 无 aria-label——读屏可访问名称缺失，立 OBSERVE-105-01（历轮读屏语义族核查的盲区）。
- **焦点管理族**：退选模态 autoFocus 落「取消」按钮（:1234，Esc 打开即生效）；无自动移焦陷阱；背景 Tab 可穿出 → O-3 族聚焦陷阱延续。**达标以外延续面已记录**。
- **新角度结论**：无障碍纵深二查暴露一个历轮清单外同族新面（图标按钮无 aria-label），按 OBSERVE 口径立条跟踪，不上升严重度（纯可用性、修复一行级、无行为面）。

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十一轮闭合，见上。下轮继续常规核对。
- **OBSERVE-104-01 / 104-02（无障碍：TabsTrigger 焦点环 + Admin 表格语义）**：本轮确认修复 → 转「闭合」，移出观察列表。
- **OBSERVE-105-01（本轮新增）**：Admin :470/:473 图标按钮 title 唯一、无 aria-label——无障碍纵深读屏语义族首查暴露，OBSERVE，修复一行级。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十三轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单与历轮一致（含 R104 补录 3 个 refetch 裸按钮）。维持「续」。
- **OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log 037410a..HEAD -- web/` | ✅ 空集（0 提交，web/ 首个代码提交后无新改动） |
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 1.02s，产物 index-CZO4oHGZ.js 420.22 kB / index-1KHlpqcc.css 41.72 kB（与 dist 一致，无漂移） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ target-guard 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ admin-auth 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ unauthorized 断言全绿 |
| `git show 037410a -- web/src/components/ui/Tabs.tsx` | ✅ 提交行逐字符与现文件一致（ring 族一行） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + npm run build + 三守护脚本 + git log/show/status），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：M-1 逐字符、三守护脚本断言、tsc/build 退出码、契约 20 扫描各类、git log/show/rev-parse、OBSERVE-104 修复逐字符、037410a 提交 diff 实证均为实测证据；OBSERVE-105-01（title 非标准可访问名称、lucide 默认 aria-hidden 的读屏影响面量级）为走读推断。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新增 OBSERVE 一条（图标按钮 aria-label 缺口，读屏语义族首查暴露）；OBSERVE-104-01/104-02 转闭合；M-1 第四十一轮闭合；F93-01 第十三轮通过；连续第五十一轮无严重级发现。
