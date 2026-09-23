# R108 前端只读审查 Findings

基线：commit 838a5b4（R107 收尾，进度 108/256）。**本轮为 OBSERVE-107-01 修复回首轮 + M-1 第四十四轮 + F93-01 第十六轮**，必查四项 + 新契约角度 a（无障碍纵深三查——键盘可达/读屏语义/焦点管理/aria 动态态全量清点）。审查范围：web/src 全部 .ts/.tsx，交叉核对 ae099cc 提交 diff、git log、组件库 .tsx、build/守护脚本实测。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR；新增 OBSERVE 一条（OBSERVE-108-01：页面内嵌错误提示区全库零 aria-live/role=alert，读屏用户无法感知状态变更）；OBSERVE-107-01 确认修复并转「闭合」态；OBSERVE-106-01 维持（建议转闭合待主控合计）。** 必查 1：Dashboard.tsx:148 refetchLogs 解构 + :707-712 重试按钮逐字符复核通过、ae099cc 后 web/ 零新提交、build 产物哈希核对（css 41.72 kB 与历轮逐字节一致）；必查 2：M-1 第四十四轮闭合（四处消费点 + 置位三路径 + 首帧边界 + 读点清点 + 六防保存链零回潮全部通过）；必查 3：F93-01 第十六轮复核通过（Button.tsx:42 ring 在位 + 残余面清单一致 + 本轮新增成员 Dashboard:707 重试按钮落带文本可读面）；必查 4：OBSERVE-107-01/106-01 回首、88-01 等延续、契约 20 全仓扫描零命中；新角度 a（无障碍纵深三查）暴露 OBSERVE-108-01，键盘可达全达标。

---

## 二、新发现

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| OBSERVE-108-01 | OBSERVE | web/src 全部页面内嵌错误条（Dashboard :319/:699、Select :908、Login :181/:272、Admin Codes :449 / Config :698 / Stats :789 / Accounts :895 / Logs :941） | **页面内嵌错误/失败提示区全库零 aria-live、零 role="alert"、零 role="status"**（grep 全仓命中数为 0）——顺手清点 aria-busy 亦零命中。读屏用户无法感知「加载失败」「会话失效」「需要重试」等动态状态变更：错误条出现时无任何 ARIA 播报，只能靠 Tab 遍历碰巧聚焦到重试按钮才得知失败；对照 Radix Toast（ToastProvider，自带 aria-live 播报）与刷新/计数等视觉动效，页面内嵌状态区无声是真实不对称。量级：读屏用户可用性降级（状态感知缺失），无数据/安全危害。修复面 display 级：错误条容器挂 `role="alert"`（隐式 assertive live region）或统一引入 live region 包裹。按历轮 OBSERVE 口径立条跟踪。 |

> 备注（不立条，清点留痕）：本轮修复的 Dashboard 日志重试按钮（:708-711）样式 `text-neutral-400 hover:text-white transition-colors` 与 Admin 家族重试按钮 :452/:792/:898/:944 逐字节一致；Config :701 重试为 `underline hover:text-white` 历史样式（略异，量级低于立条线）。

---

## 三、必查项逐条结论

### 1. OBSERVE-107-01 修复回首轮
- **refetchLogs 解构逐字符在位**：Dashboard.tsx:148 `const { data: logs, isError: logsErr, refetch: refetchLogs } = useQuery({`——与 ae099cc diff 首改行（原 `:145` 处）逐字符一致。
- **onClick 调用逐字符在位**：Dashboard.tsx:707-712——`<button onClick={() => refetchLogs()} className="text-neutral-400 hover:text-white transition-colors">重试</button>`，与 ae099cc diff 新增块逐字符一致；失败态注释块（:702-706 OBSERVE-107-01 锚点注释）在位合法保留。
- **ae099cc 后 web/ 零新代码提交**：`git log --oneline ae099cc..HEAD -- web/` 空集（exit 0）；ae099cc 即 web/ 当前最后一个代码提交（其父链 838a5b4 为 docs 收尾）。
- **build 哈希**：npm run build 产出 `index-C_ZSK3dI.js 420.73 kB / index-1KHlpqcc.css 41.72 kB`——css 41.72 kB 与 R105/R106/R107 记录逐字节一致；JS 名每次构建重摇（R106=CXO4oHGZ、R107=DnlP8miw、本轮=C_ZSK3dI）属 Vite 正常哈希抖动，源码 diff 已实证零改动，产物差异不构成回归证据。
- **结论：OBSERVE-107-01 → 转「闭合」。**

### 2. 保存链 M-1 稳态语义（第四十四轮）
- **四处消费点全传第三参 echoedRef.current 逐字符复核通过**：
  - :509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` —— flushTargets
  - :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` —— handleBack 判定
  - :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` —— handleBack 循环
  - :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` —— 防抖回调
  - 全仓 grep `shouldDeferSave(` 命中仅此四处消费点 + targetGuard.ts:64 定义 + 数处注释，**无第五消费处确证**。
- **shouldDeferSave 纯函数定义（targetGuard.ts:64-72）逐字符复核**：`stateData === undefined → true`；`echoed → false`；`(stateData.courses?.length ?? 0) > 0 && hasSelected`——与 R107/R106 记录完全一致。守卫从「真合并/等待/消费时刻」「已回显稳态放行编辑」「首帧未到无条件推迟」三段注释语义完整。
- **echoedRef 置位三路径**：:162 声明 `useRef(false)`；:200 账号复位 effect 置 false；:240 courses 空分支置 true + setEchoDone(true)；:297 合并完成置 true + setEchoDone(true)。**首帧边界**：:234 `if (stateData === undefined) return`、:247 `if (pubs.length === 0) return` 前置短路不置位；:229 回显 effect 首行盾、:319 清理 effect 盾在位；:256 全清空守卫 `rev > 0 && !anyHas → return prev` 不置位。
- **读点全量清点**：运行时读点 6 处 = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699，与历轮完全一致。
- **六防保存链零回潮**：防抖回调五判据（shouldDeferSave :699 → 发布缺席 :709 → selectedHasStalePublish :717 → 联查空 :732 → targetsUseCurrentPublishes :737）与 flush 五判据（:509/:515/:526/:553/:558）同款消费时刻读最新 ref 逐字符在位；dirtyRef 置 true 三处（saveNow catch :455 / flush 在飞 :563 / 防抖在飞 :742）全在；saveNow finally 补发链（:457-466）在位；unmountedRef 守卫 + 卸载后不 fire/不 toast 语义对齐；防抖 effect 依赖含 hasPublishes/echoDone/stateData 三自愈信号（:755）在位。
- **TDD 实证**：target-guard-check 断言全绿（含 F43 全清空放行 + 稳态 echoed=true 放行编辑 + 首帧未到即使 echoed=true 仍推迟的边界断言 output 已复核）。
- **结论：第四十四轮延续闭合。**

### 3. F93-01 无障碍焦点（第十六轮复核）
- **Button.tsx:42 ring 逐字符在位**：`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]`，置于 base class（cn() 首参），与 R93 提交原文逐字符一致；Tabs.tsx:29 ring 同款 token 在位（R104 修复后无回归）；Admin :583 switch `role="switch" aria-checked` + `focus-visible:ring-2 focus-visible:ring-white/60` 在位；Login :173 密码切换 ring 在位。
- **残余面清单与历轮一致 + 本轮新增成员**：裸 `<button>` 无 ring 面全量 grep 复核——Dashboard CollapseSection :67、**Dashboard 新增重试按钮 :707（本轮修复新增成员，带文本「重试」）**、Admin :417/:425/:441/:452/:651/:661/:701/:792/:898/:944、Login 激活取消 :299、Toast Close :100，全部在位。**OBSERVE-106-01 的 7 个带文本裸按钮（:417/:425/:441/:452/:651/:661/:701）确认在位更新**；R104 补录 3 个 refetch 按钮（:701/:792/:898）在位；:792/:898/:944/:107 为同族 refetch 按钮成员。同族自带 ring 面排除（Admin :577 switch / Login :173）。新增成员 :707 与 OBSERVE-106-01 同属「带文本读屏可读无 aria-label 缺口」分界线内，不升级缺口面。
- **结论：第十六轮复核通过（F93-01 本体零回归，残余面加一成员但口径不变）；OBSERVE-106-01 维持「续」，建议转「闭合」待主控裁决。**

### 4. 既往观察项延续
- **OBSERVE-107-01（上一轮新增，本轮回首）→ 转「闭合」**：见必查 1。
- **OBSERVE-106-01（上一轮新增，本轮回首）**：7 个带文本裸按钮仍在位、分界线确认；无新证据、无回归，建议转「闭合」待主控合计。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode，无 error boundary fallback）。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：85-02 倒计时 hook 注释如实口径在位（web/src/lib/useTickingCountdown.ts，整秒 setNow 真实语义）；84-01 激活票据三路径清票复核在位（Login :87-97 过期/已用分支清票、:92-96 错误分支清票、:232-238/:301-306 取消清票）；83-01 Select :247 `pubs.length === 0` 分支仍在；77-02 窗口关闭无目标管理入口；76-03 Toast Close :100 无 aria-label（Radix 受控，历轮口径维持）。全部维持「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮三模态（Login 激活 :223 / Admin 删除 :210 / Select 退选 :1201）role=aria-modal/aria-labelledby/Esc/autoFocus 复核在位。
- **契约 20 全仓扫描零命中**：轮次标签族（`第 .* 轮|（第.*轮）|round\s*[0-9]` 多形态 grep）src/ + scripts/ 零命中；行号引用族零命中；dangerouslySetInnerHTML / innerHTML= / new Function / eval 零命中；localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级；导航能力（window.open / location.* / history.*）零命中；tabIndex 全仓零命中——全仓零命中。

### 5. 新契约角度 a——无障碍纵深三查（键盘可达/读屏语义/焦点管理/aria 动态态全量清点）
- **键盘可达面（第一查）全达标**：focus-visible ring 覆盖与 F93-01 必查 3 全量一致（Button/Tabs/Admin switch/Login 密码切换四处持 ring）；tabIndex 全仓零命中（无破坏原生 Tab 序的自定义 tabIndex）；全部可交互元素为原生 button/Input/label 语义；三模态 Esc 关闭全部在位；autoFocus 三处（Admin:241 删除取消 / Login:267 激活码 / Select:1234 退选取消）均落模态内合理。**结论：键盘操控零缺口。**
- **读屏语义面（第二查）部分达标**：ARIA 属性族清点——role=dialog/aria-modal/aria-labelledby（三模态）✅、aria-expanded/aria-controls（CollapseSection :67-80，useId 唯一 id）✅、role=switch/aria-checked（Admin :577）✅、aria-pressed（Login :172 密码切换）✅、aria-label（select/search 输入 :870、Admin 图标按钮 :470/:473、Admin 表格 :833、Admin switch :581）✅、role=progressbar/aria-valuenow（Progress :23-28）✅。**缺口 = 页面内嵌错误/失败/会话失效提示区全库零 aria-live/role=alert/role=status，aria-busy 亦零** → 立 OBSERVE-108-01（见新发现表）。
- **焦点管理面（第三查）**：三手写 modal 无焦点陷阱（Tab 可穿出背景）、无关闭后焦点恢复（关闭后焦点回 body）——历轮 O-3 族「聚焦陷阱/焦点恢复」观察已在册，指向 F6-02 Radix Dialog 迁移单一出口，本轮无新增、维持。
- **aria 动态态（第四查）**：aria-busy 零命中（加载态无忙碌语义）、aria-live 零命中（动态状态无播报）、Radix Toast 自带 aria-live 为唯一播报通道——与 OBSERVE-108-01 同族，合并立条。
- **新角度结论**：键盘可达零缺口；读屏语义总体完备但暴露一个系统性 OBSERVE（页面内嵌错误条无 live region 播报）；焦点管理保持历轮观察；aria 动态态并入同一条立条。

---

## 四、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第四十四轮闭合，见必查 2。下轮继续常规核对。
- **OBSERVE-107-01（Dashboard 日志失败态补重试）**：本轮确认修复 → 转「闭合」，移出观察列表。
- **OBSERVE-106-01（Admin 7 个带文本裸按钮无焦点环）**：维持「续」，建议转「闭合」待主控合计。
- **OBSERVE-108-01（本轮新增）**：页面内嵌错误条无 aria-live/role=alert，OBSERVE，修复 display 级（容器挂 role="alert"）。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十六轮持位复核通过（Button.tsx:42 ring 在位零回归）；残余面清单 + 新增成员 Dashboard:707。维持「续」。
- **OBSERVE-88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `git log --oneline ae099cc..HEAD -- web/` | ✅ 空集（0 提交，ae099cc 即 web/ 最后一个代码提交） |
| Dashboard.tsx:148 refetchLogs 解构 / :707-712 重试按钮 | ✅ 逐字符复核 + 与 ae099cc diff 逐字符一致 |
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 700ms，产物 index-C_ZSK3dI.js 420.73 kB / index-1KHlpqcc.css 41.72 kB（css 与历轮记录逐字节一致） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 断言全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 断言全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 断言全绿 |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力/tabIndex 全形态） | ✅ 零命中 |
| 无障碍纵深扫描（aria-live/role=alert/role=status/aria-busy/tabIndex/focus-visible/autoFocus/裸 button 全量） | ✅ 见必查 3 与必查 5（OBSERVE-108-01 命中） |
| `git status --short --branch` | ✅ `## master`；未跟踪仅本报告与 backend 兄弟文件（dist 已 gitignore） |

## 六、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + npm run build + 四守护脚本 + git log/show/status/diff），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：M-1 逐字符、四守护脚本断言、tsc/build 退出码、契约 20 扫描各类、git log/extract/rev-parse/diff、OBSERVE-107-01 修复逐字符、build 产物哈希均为实测证据；OBSERVE-108-01（读屏用户对页面内嵌错误条无 live 播报的可用性量级）为走读推断 + 全仓 grep 实证（aria-live 命中数 0）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新增 OBSERVE 一条（108-01 无障碍 live region 缺口）；OBSERVE-107-01 转闭合；OBSERVE-106-01 维持续（建议转闭合待主控合计）；M-1 第四十四轮闭合；F93-01 第十六轮通过；连续第五十四轮无严重级发现。