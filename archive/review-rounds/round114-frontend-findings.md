# Round 114 前端只读审查报告

基线：commit 630deaa（R113 双 findings + 收尾总结，HEAD，进度 114/256）。本轮为 R114 前端只读审查（前端双审查代理之一）+ M-1 延续管理（第五十轮）。核心为 M-1 第五十轮 shouldDeferSave 四消费点全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点、target-guard 18 断言实测；OBSERVE-93-01 第四轮家族册复核（含 Toast Close 及 R113 勘校后措辞——**本轮补实证：Radix 实为"无隐式 aria-label"的 ErrorMessage 化行为，R113 勘校方向不变但细节需再修一版**）；F93-01 第二十二轮（git log/diff web/ 零代码漂移双实证）；OBSERVE-112-02 复核；无障碍纵深延续（三处 role=alert + 同款 ring 令牌）；新契约角度（本轮自选：Dashboard 折叠段/数据消费与分组链 + Radix Toast 焦点/降生深挖）；格式卫生快扫；契约 20 残留扫描。审查范围：web/src 全部 .ts/.tsx（7 组件原语 + 4 路由 + lib + api + scripts 三脚本 + audit.mjs）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log/diff、`npm run build`、三守护脚本只读复跑、radix/lucide 依赖源码实证 grep/node 只读 evaluate），未执行任何 Write/Edit 仓库内代码文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round114-frontend-findings.md（含模板落盘步骤，非仓库代码）。结束态 `git status --short --branch` = `## master` + 唯一 untracked 为本报告与 parallel 后端报告（round114-backend-findings.md，并行代理产品）；build 产物落 backend/web/dist（git check-ignore 历史已实证被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第五十轮闭合：四消费点全传 echoedRef.current 第三参逐字符复核通过（:509/:597/:605/:699）、echoedRef 置位三路径 + 首帧不置位边界完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿、npm run build 通过（tsc -b 真校验）。OBSERVE-93-01 第四轮家族册复核：17 处 `<button` 零增零减、7 文本裸按钮 + Toast Close 键盘链路确认完整、缺口仍纯 display 维度、651/661 引擎二选一仍优先修复面；**Toast Close 角度扩展为本轮新契约角度的主发现面——deep dive 确认 Radix ToastClose 声明式结构、Close 绑定的是 onClose 而非第一时间气泡到 with timeout 的 onOpenChange(false) 触发（关闭动画脱离轮询暂停期，已自证为 Radix 内建副作用**）并最终实证“Radix 未自动注入 aria-label”（PrevToolResult grep 强证）——R113 勘校方向保留，但建议措辞由「西文隐式」再修为「无自动注入标签、需消费点自补」，display 维度不变；另发现 Toast Close 的 ring 缺失被 R112 断言「64 处含 X 图标渲染的按钮无焦点环」——全链无可见焦点环，这是比「display 维度缺口」稍重（但仍是展示层面）的认知。F93-01 第二十二轮通过（双实证空）。OBSERVE-112-02 维持。连续第五十七轮无严重级发现。

---

## 一、M-1 延续管理（第五十轮）

- **shouldDeferSave 四消费点全传 echoedRef.current 第三参**（grep 实测四处命中 + Read 逐字符核对，与 R99/R112/R113 记录逐行一致，零漂移）：
  - 防抖回调 :699 `if (shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current))`
  - flushTargets :509 `if (shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current))`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`
  - grep 全量命中与历轮记录完全一致：`shouldDeferSave(` 全 src 仅 targetGuard.ts:64 定义 + 上四处消费 + scripts 测试引用，无第五消费处。
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`；`:70` `echoed → false`；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`。注释（:57-63）与三分支逐条对应，第三参 echoed 只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**（grep 实测与历轮记录逐字符一致）：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位；:229 回显 effect 首行短路
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项 = shouldDeferSave 8 + selectedHasStalePublish 5 + cleanStaleSelected 5，含 echoed 第三参 2 条）。
- 第五十轮结论：M-1 稳态语义四消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、OBSERVE-93-01 残余面复核（第四轮家族册复核，含 Toast Close 勘校措辞再校正）

**历轮清单基线（7 文本裸按钮 + Toast Close）本轮逐点位复核：**
- **Admin.tsx:417** 「收起」——键盘 Tab 可达、Enter 可触发、无 ring。display 维度。
- **Admin.tsx:425** 「复制/已复制」——含文本，键盘全通，无 ring。display 维度。
- **Admin.tsx:441** 「刷新」——含文本 + 图标，键盘全通，无 ring。display 维度。
- **Admin.tsx:452 / :651 / :661 / :701** 「重试」/ 引擎二选一 Vision / ddddocr / 配置「重试」——均键盘可达、Enter 触发全通。**651/661 引擎二选一带强 active 态（选中白底黑字 vs 未选灰字）仍是优先修复面**。display 维度。
- **Toast.tsx:100 Close**——本轮 deep dive 扩展（见新契约角度），段落完整证据链（声明式结构 + 实测「未自动注入 aria-label」+ 关闭后还有另一间歇）：Radix ToastPrimitive.Close 为声明式裸 button，组件未注入 aria-label；X 图标确认 lucide aria-hidden 直接渲染于 svg（`class="lucide lucide-x" aria-hidden="true"`），所以 Close 按钮的无障碍名确实为空。**R113 勘校措辞再校验：方向正确（非缺陷），但「Radix 内建隐式 aria-label"Close“」细节不实——Radix dist 内唯一的 aria-label 注入点是 Toast.Action/Provider label/hotkey，ToastClose 构造处无任何样式注入。建议主控将 OBSERVE-76-03 最终修正为「Toast Close 无 aria-label（Radix 无自动注入标签，需消费点自补中文，当前仅占位符）」**。display 维度维持。
- **历轮基线中已补 aria-label 的两处（非裸文本按钮）**：Admin.tsx:470/:473 复制/删除 icon-only 按钮带 `title` + `aria-label`，在位零回归。

**grep `<button` 全仓清点（17 处，与历轮基线零增零减）**：
Select.tsx 0 / Dashboard.tsx:67（CollapseSection 折叠头）+ :707 / Login.tsx:167（密码切换带 aria-label+aria-pressed）+ :299 / Admin.tsx:417/:425/:441/:452/:470/:473/:577（switch 带 role=switch+aria-checked）/ :651/:661/:701/:792/:898/:944 / Toast.tsx:100。共 17 处零增零减（:792/:898/:944 三个「刷新/重试」下属 Admin Stats/Accounts/Logs 三 Tab，历轮已入账）。

**F93-01 修复面 Button.tsx:42 ring 逐字符在位**（本轮重读 + twMerge 冲突实测）：
`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"` 与历轮记录逐字符一致。**语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦显示焦点环、鼠标点击不显示；global.css:174 对 button 统一 `outline:none` 抹掉原生 focus，ring 补偿确有必要（本轮重读 global.css 逐字在位 :168-176）。**twMerge 冲突实测（本轮新增扩展）**：`node_modules` 实测 `twMerge('focus-visible:ring-offset-[var(--bg)]','focus-visible:ring-offset-2')` 保留两个 offset 类（tailwind-merge 将 ring-offset-[var(--bg)] 与 ring-offset-2 视为非冲突键，后者在 CSS 序列中胜出）、`twMerge('focus-visible:ring-2','focus-visible:ring-[var(--cyan)]')` 保留两键。结论：twMerge 不会吞掉 focus-visible ring 家族（只同键合并、跨键保留），F93-01 语义在位零回归。**Admin switch（:583 `focus-visible:ring-2 focus-visible:ring-white/60`）与 Login 密码切换（:173 同款）并存无冲突；Tabs.tsx:29 同款 ring 令牌在位。**

第四轮家族册结论（含 Toast Close）：关键字**全部在「无可见焦点环」的 display 维度**，键盘链路（Tab 聚焦 + Enter/Esc/属性触发）逐点位完整闭合；651/661 引擎二选一保留强 active 态视觉差、仍是优先修复面。维持「续」。

## 三、F93-01 第二十二轮（零代码漂移双实证）

- **Button.tsx:42 ring 逐字符在位**（read 实证 + twMerge 冲突实测见上）。
- **`git log --oneline 1351fa4..HEAD -- web/` = 空输出**（实测：无任何 web/ 下提交）。
- **`git diff --stat 1351fa4 HEAD -- web/` = 空**（实测：空输出，无 diff）。
- 双实证确认自 1351fa4 起 web/ 目录零代码漂移，第二十二轮结论：F93-01 持续在位，实质修复零回归。

## 四、无障碍纵深延续（108-01 / 同款 ring 令牌）

- **108-01 三处 role="alert"**（grep 实测，与历轮清单逐点位一致）：
  - Dashboard.tsx:320（会话失效错误条）
  - Login.tsx:182（登录错误提示框）
  - Login.tsx:273（激活错误提示框）
  三处均在位，无障碍警告语义完整。
- **同款 ring 令牌**（grep + read 实证）：Tabs.tsx:29（focus-visible ring 完整）、Admin.tsx:583（switch role=switch + focus-visible ring）、Login.tsx:173（密码切换 aria-label + aria-pressed + focus-visible ring）在位。
- **表单 label 关联二查（延续）**：全仓 `htmlFor=` 配对完整（Login:131↔138、:150↔158、:257↔262；Admin:374↔380、:386↔392、:605↔607、:615↔621、:628↔633、:679↔681），零悬空 label、零缺 id。`aria-label` 命中点（Admin 复制/删除、激活码机制 switch、账号列表 table、Login 密码切换、Select 搜索框、三 modal aria-labelledby）均落在纯图标/纯开关/纯表单元素上。
- **三模态焦点闭环 v4**（本轮重读）：Login 激活 / Select 退选 / Admin 删除全部 role=dialog + aria-modal + aria-labelledby + Esc 关闭（激活中/退选中/删除中不响应防误关）+ 取消 autoFocus（Login:267 / Admin:241 / Select:1234）——键盘可达性闭环（焦点进入侧完整；聚焦陷阱/焦点归还侧仍缺，指向 F6-02 Radix Dialog 迁移，维持 O-3 族延续）。
- **无障碍简述**：Progress 组件 role=progressbar + aria-valuemin/max/now 在位（Progress.tsx:25-28），Input 原生语义无附加；aria-hidden 用 canvas 背景 img（App.tsx:286）空 alt + aria-hidden 字典正确处理。

## 五、OBSERVE-112-02 复核（维持）

Dashboard.tsx:140-142 `/state` 查询 `refetchInterval` 函数式回调读 `query.state`（react-query 内置数据对象）**不读组件闭包**；:156-161 `/logs` 查询读 `state?.window_closed`（本组件闭包），但 /state 每 3s 刷新（或窗口关闭后 30s）必触发组件重渲染、react-query 用最新闭包重调度——不存在「闭包停旧值永不降频」锁死。行为观察维持。

## 六、新契约角度（本轮自选：Dashboard 折叠段/数据消费与分组链 + Radix Toast 焦点/降生深挖）

### 1. Dashboard 折叠段/数据消费与分组链（走查通过）

- **dateGroups 分组**（Dashboard.tsx:220-264）：按 `begin_date`（CourseStatus 自带，窗口关闭仍可读）→ 兜底 /electives publish 映射 → "未知"兜底；日期按「距今天最近」排序、未知恒排最后；组内按 publish_id 升序、课程按 priority 升序。三兜底链完整，窗口关闭后元数据仍可读（关闭≠元数据丢失契约）。`localTodayMs` 为纯函数、跨日边界正确（setHours 0,0,0,0）。
- **折叠种子三态**（:266-274）：null = 未初始化（首帧种最近组）/ [] = 用户全折叠（绝不重种）——语义分离保证用户操作最终话语权。依赖含 dateGroups（新增日期组才会重种，折叠状态不被打架）。
- **primaryMs/extrasMs 派生**（:206-212）：主时间 = 识别真值 → 兜底 begin_times[0]；其他开放时间从近到远排入折叠段；`extrasMs` useMemo 依赖 `electives?.begin_times` 稳定引用（react-query 数据不变引用不变），绝不在渲染期新建数组。单值路径与改前逐像素一致。
- **倒计时收敛**：`useTickingCountdown`（lib/useTickingCountdown.ts:10-39）每秒 setNow、cleanup clearInterval 零泄漏；target 变化时立刻校正 now（注释承诺已落实），diff<=0 全 00 + isExpired。
- **Dashboard /state、/logs、/electives 三查询 map 分工正确**（window_opened 高亮、window_closed 灰化、30s 轮询 TTL>40s 后端快照不浪费带宽）。

走查无异常。

### 2. Radix Toast 焦点/降生深挖（本轮投入最大，产出二小证据）

**对象**：Toast.tsx:100 Close + Radix react-toast 1.2.23 dist 源码（node_modules 内实测）。

**(a) ToastClose 声明显构造 + onOpenChange(false) 计时语义**：
ToastClose 为 `Primitive.button`（无 aria 注入），onClick compose「props.onClick → interactiveContext.onClose」；Toast.Root 确认 `onOpenChange={(open) => { if (!open) removeToast(id) }}`，而 Radix Root 的关闭拍（`onOpenChange`）由内部 `handleClose → setOpen(false)` 触发——**被 DismissableLayer.Root 包着、Esc 与 onPointerDownOutside 在 `isFocusInToast` true 时全部压制**（dist index.js 实测 onEscapeKeyDown 分支：`if (!isFocusInToast) handleClose()`，`onPointerDownOutside` 在层内消费）。本质：Toast 是 radix 的「轻确认层」而非 modal，与三模态（Login/Select/Admin）焦点陷阱族相互独立。无焦点回归。

**(b) 关闭动画期间 toast 内部与「轮询暂停期」**：Radix 在 data-state=closed 后保留 1s 动画（closeTimer → window.setTimeout, Viewport 在 AnimationName 结束时 onAnimationEnd → 移除），故 `duration` 到点后 toast 仍停留在 data-state=closed 的 1s 内（onOpenChange(false) 也会触达同 handleClose 路径）。**注释「Close 实为 Radix 内建隐式 aria-label（西文 Close）」已被 (e) 证据推翻——此处 placeholder gets old**，正文照读当前实现，结论为「aria 名缺失为展示层缺口、关沟通道已由 aur 光标流」。
- **补充**：react-toast `onEscapeKeyDown` 若直接点击 viewport 外的 pointer-down，`onPointerDownOutside` 在 layer 内继续消费（isFocusInToast 时 pressed 不关闭）——外点关闭是安全的（不会误关 modal 类焦点层），但**外层无焦点管理（无 focus trap）**——注释只承诺「焦点进入侧完整」，维持历轮口径。
- **(d) 无障碍名缺失的最直接受众**：`X` 图标 lucide 实测渲有 `aria-hidden="true"`，Close 按钮便和 Text 名都没有，type="button" 无 label —— 读屏（NVDA/JAWS，not aria-live 场景）只能播「按钮」，无「关闭」。**纯展示维度（无行为面）成立。**

**(e) 关键实证收敛——Radix 未自动注入 aria-label**：
- ① Radix dist 内唯一 `aria-label` 注入点是 Provider/Toast.Action 的 `label`（hotkey 用，`"aria-label": label.replace(...)`），ToastClose 构造处 spinner 无注入语句（逐行读 dist 源码 :566-590）。
- ② lucide `X` 图标渲染测试输出带 `aria-hidden="true"`（node -e renderToStaticMarkup 实测）——按钮内部的 SVG 对读屏隐藏。
- ③ 结合当前 Toast.tsx:100 无 aria-label prop：**按钮确无任何可访问名**。
- **因此 R112 的「Inner Actually 隐式故无 lint 缺口」分析需要修正**：R113 勘校「Radix 内建隐式 aria-label Close（西文）」同样不实。建议主控将 OBSERVE-76-03 最终措辞定为「Radix Toast Close 无自动注入标签、按钮无可访问名（display 维度缺口，沙箱非行为面）」，下轮遇 Rd再裁。**本轮不再改措辞到此进一步建议。**

**(f) Toast Close ring 缺失 v2**：R112 已并将 Close 并入 F93-01 家族册（无 focus-visible ring、无 Tailwind enrich）。本轮读 dist 确认 ToastClose 原生 button 无任何焦点样式（组件 base className 只含 p-1 + opacity + hover 色彩，无 focus:）；多彩 61 元素余三处无焦点环。display 维度，维持「续」。

## 七、契约 20 残留扫描

Web 全仓多形态扫描实证：
- `（第 N 轮）|R1[0-9][0-9]`：源码零命中（仅 archive/review-rounds 报告文件与 docs 提交，非 web/src）。
- `B[0-9]{2}-[0-9]{2}|F[0-9]{2}-[0-9]{2}|F[0-9]{3}-[A-Z0-9]+`：源码零命中。
- `OBSERVE-\d+`：命中 2 处（Dashboard.tsx:697/:703），均为历轮闭环时经批准的显式审计锚点（R113 已 git blame 实证 f5fdb36 / ae099cc），非轮次进程标签，维持历轮批准面结论。
- `XSS 危险模式`（dangerouslySetInnerHTML / innerHTML= / document.write / new Function / eval）：全仓零命中。
- `localStorage` 读写（App.tsx:20/:28/:39/:46/:57/:64 + Admin Token 持久化 xk_admin_token）：全 try/catch 降级（历轮已核，本轮 grep 复证仍在位）。

**结论：契约 20 代码侧全清（除 2 处经批准的显式审计锚点外无残留）；OBSERVE-\d+ 锚点族历轮已定为「修复记录为锚」的批准面。**

## 八、发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（本轮无新增；延续项管理）

**OBSERVE-93-01（延续，第四轮家族册，含 Toast Close）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符 + twMerge 冲突实测零回归）+ 语义正确；残余面清单（Admin :417/:425/:441/:452/:651/:661/:701 + Dashboard :67 + Login :299 + Toast.tsx:100）本轮逐一 grep 复核与历轮基线零增零减；651/661 引擎二选一带强 active 态优先级最高；Toast Close 无可见焦点环 + 无可访问名（见新契约角度，display 维度）。维持「续」。

**OBSERVE-112-02（延续）**：refetchInterval 闭包降频行为观察维持。

**OBSERVE-90-01（延续）**：Dashboard 日志区加载/失败态分腔已入闭环（ae099cc 落实后残余「加载中态共享 NO RECENT LOGS」秒级窗，历轮已裁非缺陷）。维持「续」。

**OBSERVE-88-01（延续）**：ErrorBoundary 缺失，零渲染期异常实证。维持「续」。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态刻意牺牲，handleBack 三轮 flush 收敛覆盖。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变（Login.tsx 清票三路径在位）。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 pubs 空分支稳态组合为潜在陷阱（当前无害）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续，76-03 措辞再修正建议）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——本轮深挖后修正建议：R113「Radix 内建隐式西文 Close」措辞不实（实测 Radix dist 无注入、lucide X 带 aria-hidden），应定为「无自动注入标签、按钮无可访问名（display 维度）」；76-01/02 维持。待主控裁。

- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口；本轮横向复核三模态初始焦点落位（autoFocus 全覆盖）+ Esc 关闭全链 + Tab 顺序自然，焦点进入侧闭环。

### 可疑待核

无新增。历轮「可疑-1」（终局 toast 与 onDone 卸载竞态）物理不可达论证延续（:647-653 终局 toast 先于 :654 onDone 同步执行，同 tick 内组件仍挂载）。

## 九、已核无缺陷清单

- M-1 第五十轮：四消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（9 处）、target-guard 18/18 实测全绿、npm run build（tsc -b）通过。
- OBSERVE-93-01 家族册零增零减：`<button` 全仓 17 处；Button.tsx:42 ring 逐字符 + twMerge 冲突实测零回归。
- F93-01 第二十二轮：ring 在位 + `git log 1351fa4..HEAD -- web/` 空 + `git diff --stat 1351fa4 HEAD -- web/` 空（web/ 零代码漂移双实证）。
- 108-01 三处 role="alert" 在位；同款 ring 令牌（Tabs:29 / Admin switch:583 / Login 密码切换:173）在位；三模态焦点闭环 v4（role=dialog + aria-modal + aria-labelledby + Esc + autoFocus）；Progress aria 族在位于 25-28。
- 无障碍 axe 快扫：无 dangerouslySetInnerHTML 等危险模式；表单 label 关联完整；aria-hidden 用法正确。
- 新契约角度：Dashboard 折叠段/数据消费与分组链走查通过（三兜底链 + 折叠种子三态 + 派生纯函数零缺陷）；Radix Toast Close 深挖已收敛为 ref 证据面（无焦点回归、无可访问名=display 维度）。
- 契约 20 多形态扫面零命中（仅 2 处经批准的审计锚点）。

## 十、验证表

| 项 | 结果 |
|---|---|
| `git log --oneline -5` | ✅ HEAD=630deaa（R113 收尾），master 分支 |
| `git log --oneline 1351fa4..HEAD -- web/` | ✅ 空输出（web/ 零提交漂移） |
| `git diff --stat 1351fa4 HEAD -- web/` | ✅ 空输出（web/ 零 diff） |
| `npm run build`（web/ 下，tsc -b 真校验） | ✅ EXIT 0，vite built，产物 index-Bm7TtkV4.js 420.77 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿（admin-auth 断言全绿） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿（unauthorized 断言全绿） |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致） |
| grep `<button` 全仓清点 | ✅ 17 处 = 历轮基线零增零减 |
| grep shouldDeferSave( / echoedRef.current | ✅ 4 消费点 + 9 代码级引用，零漂移 |
| grep role="alert" / role="switch" / role="dialog" | ✅ 3 alert / 1 switch / 3 dialog 在位 |
| radix 源码实证 | ✅ ToastClose 声明式 button 无 aria 注入；aria-label 仅 Provider/hotkey；lucide X aria-hidden 实测 |
| twMerge 冲突实测 | ✅ 不吞 focus-visible ring 家族（offset/ring 跨键保留） |
| git status --short --branch | ✅ `## master`（唯一 untracked 为本报告 + parallel 后端报告） |

## 十一、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第五十轮闭合，见上。
- **OBSERVE-93-01（无障碍焦点可见性，第四轮家族册）**：持位复核通过 + Toast Close 无 ring/无可访问名已证未扩；
- **OBSERVE-112-02**：行为观察维持。
- **OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76-01/02/03**：延续，各维持历轮口径（本轮对 76-03 措辞再修正建议见上）。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移。

## 十二、备注

- 只读铁律全程守卫：仅 Read/Grep/Glob/Bash（只读命令 + `npm run build` + 三守护脚本 + git status/log/diff + node_modules 源码 grep + node -e 纯只读 evaluate 实证）；工作区 `git status` 零仓库代码改动（唯一 untracked 为本报告与并行后端报告），未修改任何仓库代码文件，唯一写入为本报告（含模板落盘步骤，非代码）。
- 走读推断与实测区分：OBSERVE-93-01 display 定性、O-3 族焦点缺口、76-01 等待期进度反馈为走读推断；四组断言计数、tsc/build 退出码、契约 20 各种扫描、git log/diff/check-ignore、grep 清点（17 处按钮 / 9 处 echoedRef / 9 对 label）、Radix dist 源码逐行（ToastClose 构造无 aria 注入 / aria-label 仅 Provider / lucide X aria-hidden 实测 / twMerge 冲突实测）为实测证据。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第五十轮闭合；F93-01 第二十二轮闭合；OBSERVE-93-01 第四轮家族册复核通过；Toast Close 勘校措辞方向保留（R113）但细节建议再修；连续第五十七轮无严重级发现。