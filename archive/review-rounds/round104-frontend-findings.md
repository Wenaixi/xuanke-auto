# R104 前端只读审查 Findings

基线：commit 998ef94（R103 双 findings + 收尾总结，HEAD，进度 104/256；web/ 自 f08937e 后连续零代码提交的纯观察轮）。本轮为 R104 前端只读审查 + M-1 延续管理（第四十轮）+ F93-01 第十二轮复核。新契约角度选定 **a) 无障碍纵深（键盘导航/焦点环/ARIA 完整边界）**——走查发现两个历轮残余面清单之外的同族新面，维护 OBSERVE 记录。审查范围：web/src 全部 .ts/.tsx（8 组件原语 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs），交叉核对 App 状态机 / client.ts 401 契约 / global.css。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

---

## 一、概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR；新增 OBSERVE 两条（无障碍纵深新查面，均非历轮残余面清单成员，属清单之外的首查暴露）。** M-1 延续管理第四十轮闭合：四处消费点（Select.tsx:509 / :597 / :605 / :699）全传 echoedRef.current 第三参逐字符复核通过、置位三路径 + 首帧不置位边界完整、读点全量清点（运行时读点 6 处 = 回显短路 1 + 清理守卫 1 + 消费 4，无第五消费处）、target-guard 18/18 实测全绿。F93-01 第十二轮复核通过：Button.tsx:42 ring 行逐字符在位、git log 实测 f08937e 后 web/ 目录零代码提交（`git log f08937e..HEAD -- web/` 空集）、残余面清单与历轮一致（同时清点出历轮清单未收录的 3 个 Admin refetch 裸按钮，见下）。新契约角度无障碍纵深入口清点：焦点环族、aria 动态态族、模态语义族、表格语义族——其中两个新面按历轮「UI 增强类立 OBSERVE」口径记观察；其余（Input/Button/Tabs 部分/label/autoComplete/移动端 touch）均达标。连续第五十轮无严重级发现。

## 二、新发现（OBSERVE 级，均非历轮残余面清单成员——清单之外首查暴露）

| 编号 | 级别 | 位置 | 内容 |
|------|------|------|------|
| OBSERVE-104-01 | OBSERVE | `web/src/components/ui/Tabs.tsx:29`（TabsTrigger base class） | **Radix TabsTrigger 渲染 `<button>` 且被全站 `button { outline: none }` 统一抹掉原生 focus（global.css:174），base class 仅 `focus-visible:outline-none`（主动去环）无 ring 补偿——键盘 Tab 聚焦 Tab 标签页时无任何可见焦点标识**（当前激活 Tab 有 data-state=active 背景区分，但非激活 Tab 导航焦点不可见）。F93-01 只收敛了 Button 组件，TabsTrigger 与裸按钮同族同源却从未列入历轮残余面清单。触发面：Select.tsx:935-951（发布 Tab 列表）、Admin.tsx 五 Tab 主控。WCAG 2.4.7 Focus Visible 违例，纯键盘可用性，无数据/安全/功能危害——按历轮无障碍增强口径立 OBSERVE，建议与 F6-02 Radix Dialog 迁移一并收敛（TabsTrigger base class 加 `focus-visible:ring-2 focus-visible:ring-[var(--cyan)]` 一行即可，与 Button :42 同款 token）。 |
| OBSERVE-104-02 | OBSERVE | `web/src/routes/Admin.tsx:833-893`（AccountsTab 表格） | **账号管理表格缺表格语义——`<table>` 无 `aria-label`（表格自陈述名缺失）、`<th>` 表头无 `scope="col"`（列头与数据格关联断裂），读屏用户无法获知表格主题与表头-数据列对应关系**。历轮无障碍横查聚焦按钮焦点环与 aria 动态态，从未走查表格语义族。无数据/安全影响（纯读屏可用性降级），按同口径立 OBSERVE。修复建议一行族（`<table aria-label="账号列表">` + 四 `<th scope="col">`），可随 F6-02 迁移动作批量收敛，或降级为备注不立条——本轮立条保持表格语义作为独立审查维度可被跟踪。 |
| （备注，不立条） | — | `Admin.tsx:792/:898/:944` | 历轮残留面清单（R93/R94/R100-103）「Admin 复制/刷新/删除/收起/引擎切换/重试」只收录到 :701（config 重试），**stats 重试 :792 / accounts 重试 :898 / logs 重试 :944 三个 refetch 裸按钮从未进入清单**——同族（裸按钮无焦点环）非新缺陷，仅清点遗漏，随 OBSERVE-93-01 残余面合并跟踪，下轮补录清单。 |

## 三、必查项逐条结论

### 1. 保存链 M-1 稳态语义（第四十轮）
- **四处消费点全传第三参 echoedRef.current，逐字符复核通过**：
  - flushTargets 判定 Select.tsx:509 `shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current)` ✅
  - handleBack 判定 :597 `revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)` ✅
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)` ✅
  - 防抖回调 :699 `shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current)` ✅
  - 第二参 hasSelected 语义各点正确对齐（flush 与防抖用「>0」布尔、handleBack 用 hasSelectedNow() 纯函数，F43 清空分判契约落实）。
- **echoedRef 置位三路径**：:200（account 变化复位 false + setEchoDone(false)）、:240（courses 空分支置 true + setEchoDone(true)）、:297（合并完成置 true + setEchoDone(true)）；**首帧边界**：:234 `if (stateData === undefined) return` 前置 return 不置位、:247 `pubs.length === 0` return 不置位（OBSERVE-83-01 立足点仍在）。:229 回显 effect 首行短路 `if (echoedRef.current) return`、:319 清理 effect 守卫在位。
- **读点全量清点**：运行时读点 6 处 = 回显短路 :229 + 清理守卫 :319 + 四消费点 :509/:597/:605/:699；代码级引用 9 处（声明 :162 + 置位 3 + 读 6，另 :755 依赖用 echoDone 非 echoedRef）。**考据无第五消费处**。
- **六防保存链零回潮**：防抖回调五判据（shouldDeferSave→发布缺席→selectedHasStalePublish→联查空→targetsUseCurrentPublishes，:699/:709/:717/:732/:737）与 flush 五判据（:509/:515/:526/:553/:558）同款消费时刻读最新 ref；setSelected 六代码调用点（:198/:252/:290/:322/:353/:363）无第三来源；dirtyRef 置 true 仅三处（:455 saveNow catch / :563 flush 在飞 / :742 防抖在飞）；unmountedRef 读写点与「卸载后不 fire/不 toast」契约对齐（:390-408 挂载复位+卸载置位、:431/:442/:447/:459 守卫）。saveNow 的 finally 补发链（:457-466：dirtyRef && attempt===0 才紧循环，失败重试走退避 timer）在位。
- **结论：第四十轮延续闭合。** target-guard 18/18 实测全绿（含「已回显稳态放行编辑不闷死」「全清空放行 PUT []」两个关键 TDD 断言）。

### 2. F93-01 无障碍焦点（第十二轮复核）
- **Button.tsx:42 ring 逐字符在位**：`"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与 R93 提交原文一致；ring 颜色 `--cyan` = #ffffff（global.css:32 实证）纯白高对比，offset 2px + 深黑底，键盘聚焦清晰可见、鼠标点击不触发（focus-visible 语义）。全站 `<Button>` 经 base class 一次收敛（variantStyles/sizeStyles 30+ 形态共用）。
- **git log 实证**：`git log f08937e..HEAD -- web/` 空集（0 提交），web/ 自 f08937e 后连续零代码修改（npm run build 产物哈希与 R103 逐字节一致，见验证表）。
- **残余面清单与历轮一致**：Dashboard CollapseSection :67、Admin :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100——逐行 grep 复核均仍为裸 `<button>` 无 ring。同族并发面：Admin :577 role=switch 与 Login :173 密码切换自带 ring（在位）、Input :14-15 focus:ring 自带（在位）。**本轮新增清点发现**：3 个 refetch 裸按钮（:792/:898/:944）从未进历轮清单（见新发现表备注），随残余面合并跟踪。
- **结论：第十二轮复核通过，无回归。**

### 3. 既往观察项延续
- **OBSERVE-90-01（延续）**：Dashboard.tsx:697-718 日志区仍只有 `logs && logs.length > 0 ? … : NO RECENT LOGS` 二态，无 isLoading/isError 分支；对照 Admin LogsTab :928-950 完整四态仍不对称。里程碑第五轮评估（同信道次位表现，修复 8 行无行为收益）维持不落地，维持「续」。
- **OBSERVE-88-01（延续）**：ErrorBoundary 全仓零命中复证（main.tsx 裸 createRoot + StrictMode，无 componentDidCatch/getDerivedStateFromError 引用）。渲染期异常源近零（所有 data 经 api() 校验 + 类型契约），维持「续」不落地。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76-01/02/03（延续）**：81-02 黄金期 400ms 防抖竞态（handleBack 三轮 flush 收敛 :611-642 + 等一帧复查 revRef :622-624 覆盖无新触发面）；84-01 激活票据清票三路径（:87-90/:91-97/:232-238 + :299-311）；83-01 Select :247 `pubs.length === 0` 分支仍在（courses 非空 + publishes 空 + echoedRef 未置位稳态，当前行为无害）；77-02 窗口关闭无目标管理入口；76-03 Toast Close 无 aria-label（本轮重读：Radix Toast.Close 为受控组件，无自定义 aria-label，历轮口径维持）。全部维持「续」。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮三模态横复核（Select 退选 :1201-1248 / Admin 删账号 :210-244 / Login 激活 :223-316）role=dialog + aria-modal + aria-labelledby + Esc keydown 全链在位，autoFocus 落位（Select 取消 :1234 / Admin 取消 :241 / Login 激活码输入框 :267），Tab 顺序自然。
- **契约 20 全仓扫描零命中**：轮次前缀标签族（`第 N 轮|R1?0?x|B/F/O[0-9X]{2}-|M-1|F\d{2}A?[A-Z]?\d` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中（global.css:86 为 CSS 注释合法文本，非标签）。

### 4. 新契约角度 a——无障碍纵深（键盘导航 / 焦点环 / ARIA 完整边界）
- **焦点环族**：Button 全站收敛（上一节）；Input :14-15 focus:ring 在位；Tabs 系仅 `focus-visible:outline-none` 无 ring——**新面立 OBSERVE-104-01**。Admin :583 / Login :173 常规裸按钮自带 ring 排除。CollapseSection :67 有 aria-expanded/aria-controls（useId 唯一）但无环（已是残余面清单成员）。
- **aria 动态态族**：Progress aria-valuenow（unannounced 显式传 0 = 注释「未公布无进度」契约落实，R101 核验过，无变异）；CollapseSection aria-expanded + aria-controls（useId :64 保证 DOM id 唯一）；Admin :577 role=switch + aria-checked + aria-label="激活码机制"；Login :171-172 密码切换 aria-label + aria-pressed；Toast 通知由 Radix Toast 内部 aria-live 承载（node_modules @radix-ui/react-toast index.mjs:376 `aria-live: polite/assertive` 实证）——通知可读屏读出。**该族全达标**。
- **模态语义族**：三模态全链（上节）达标；背景 Tab 可穿出 → O-3 族聚焦陷阱延续。**达标以外延续面已记录**。
- **表格语义族**：Admin AccountsTab 表（:833-893）——**新面立 OBSERVE-104-02**（无 aria-label / th 无 scope）。Select 课程卡与 Dashboard 目标卡均为语义化 div/button，无表格族缺口；Badge/Progress/Card 均无表格角色。
- **Label 关联族**：Login 账号/密码 label htmlFor 双向（:131/:150）、激活码 label htmlFor（:257）；Admin 全部 Input label 均有 htmlFor（count :374/:386、config :605/:615/:679）——全达标。**autoComplete 族**：Login username/current-password 在位（:142/:163）。**aria-hidden 族**：装饰性图标均 lucide（默认 aria-hidden 由 aria 工具确定性提供），背景画布 aria-hidden :284 在位。
- **新角度结论**：无障碍纵深暴露两个历轮清单外同族新面（TabsTrigger 焦点环、Admin 表格语义），均按 OBSERVE 口径立条跟踪，不上升严重度（纯可用性、修复均一行级、无行为面）。

## 四、历轮观察延续
- **M-1（echoedRef 第三参稳态语义）**：第四十轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十二轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零代码提交）；残余面清单与历轮一致，**本轮补录 3 个遗漏 refetch 裸按钮（:792/:898/:944）** 随集合跟踪。维持「续」。
- **OBSERVE-104-01 / 104-02（本轮新增）**：TabsTrigger 焦点环缺失 / Admin 表格语义缺失——无障碍纵深首查暴露，均 OBSERVE，指向 F6-02 Radix Dialog 迁移时的批量收敛（连同一行级修复建议）。
- **OBSERVE-90-01 / 88-01 / 85-02 / 84-01 / 83-01 / 77-02 / 76 族（延续）**：全部维持历轮结论，无新依据。
- **O-3 族（聚焦陷阱/滚动穿透/焦点恢复）**：指向 F6-02 Radix Dialog 迁移单一出口，维持观察。

## 五、验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 711ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist（哈希与 R103 逐字节一致，web/ 零代码改动实证） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（含「已回显稳态放行」「全清空放行」两关键断言） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git log f08937e..HEAD -- web/` | ✅ 空集（0 提交） |
| `git status --short --branch` | ✅ `## master`（HEAD=998ef94）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 六、备注
- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + npm run build + 四守护脚本 + git log/status），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：M-1 逐字符、四守护脚本断言计数、audit 全绿、tsc/build 退出码、契约 20 扫描各类、git log/rev-parse、F93-01 ring 行逐字符、Radix toast aria-live（node_modules index.mjs:376 源码实证）均为实测证据；OBSERVE-104-01（TabsTrigger 渲染 button 继承 outline:none 的推断链 = global.css:174 reset 实测 + Tabs.tsx:29 逐字符 + Radix tabs Trigger 渲染 button 的标准事实）、OBSERVE-104-02 影响面量级、3 个 refetch 裸按钮清单补录为走读推断。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；新增 OBSERVE 两条（无障碍纵深首查暴露，均历轮清单外）；M-1 第四十轮闭合；F93-01 第十二轮通过；连续第五十轮无严重级发现。