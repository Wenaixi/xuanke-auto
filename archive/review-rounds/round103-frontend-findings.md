# Round 103 前端只读审查报告

基线：commit a98d5e3（R102 收尾总结 + 后端 findings，HEAD，进度 103/256；连续八轮双端零代码修改的纯观察轮）。本轮为 R103 前端只读审查 + M-1 延续管理（第三十九轮）。核心为 M-1 第三十九轮 shouldDeferSave 三消费点 + handleBack while（web/src/routes/Select.tsx 防抖回调 :699 / flushTargets :509 / handleBack 判定 :597 + while :605）全传 echoedRef.current 第三参逐字符复核、echoedRef 置位三路径 + 首帧不置位边界、读点全量清点（考据无第五消费处）；六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归；F93-01 Button focus-visible 持续复核（第十一轮）；OBSERVE-90-01 / OBSERVE-88-01 / OBSERVE-85-02 / OBSERVE-84-01 / OBSERVE-83-01 / OBSERVE-77-02 / OBSERVE-76-01/02/03 / O-3 族延续管理；新契约角度走读（本轮选定 b) 状态恢复与竞态——账号切换/登出/401 三路下组件状态与请求的取消/忽略）；契约 20 扫描（轮次标签族 / 行号引用族 / XSS 危险模式 / localStorage try/catch 降级 / 导航能力全形态）。审查范围：web/src 全部 .ts/.tsx（7 组件原语 + 4 路由 + lib + api + scripts 四脚本 + audit.mjs）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。

## 只读铁律声明

全程仅使用 Read / Grep / Glob / Bash 只读命令（git status/rev-parse/log、`npx tsc -b --pretty false`、`npm run build`、四守护脚本只读复跑、各类 grep 扫描），未执行任何 Write/Edit 仓库内文件、未执行任何 git 变更命令。唯一写入为本报告文件 archive/review-rounds/round103-frontend-findings.md。结束态 `git status --short --branch` = `## master`（无 untracked）；build 产物落 backend/web/dist（git check-ignore 确认被忽略）。零仓库代码改动。

## 概述

**本轮零 CRITICAL、零 MAJOR、零 MINOR + 延续观察管理（无新增 OBSERVE，宁缺毋滥）。** M-1 延续管理第三十九轮闭合：三消费点（:509/:597/:605/:699）全传 echoedRef.current 第三参逐字符复核通过、echoedRef 置位三路径（:200 复位 / :240 空分支置位 / :297 合并完成置位）+ 首帧不置位边界（:234 stateData undefined / :247 publishes 空）完整、读点全量清点（代码级引用 9 处 = 置位 3 + 守卫 2 + 消费 4，考据无第五消费处）、target-guard 18/18 实测全绿。F93-01 持续复核通过（第十一轮）：Button.tsx:42 ring 行逐字符在位、git log 实测自 f08937e 后 web/ 目录零代码提交（`git log f08937e..HEAD -- web/` 空集）。新契约角度 b（状态恢复与竞态）逐路走查无新增漏洞，唯一疑点「手动操作卸载后 invalidate+toast」核定为可接受语义（详见第四节，不立条）。契约 20 扫描零命中。连续第四十九轮无严重级发现。

---

## 一、M-1 延续管理（第三十九轮）

- **shouldDeferSave 三消费点全传 echoedRef.current 第三参**（grep 实测四处 + Read 逐字符核对，与 R101/R102 记录逐行一致，零漂移）：
  - 防抖回调 :699 `if (shouldDeferSave(stateDataRef.current, selectedCount > 0, echoedRef.current))`
  - flushTargets :509 `if (shouldDeferSave(stateDataRef.current, latestSelectedCount > 0, echoedRef.current))`
  - handleBack 判定 :597 `if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current))`
  - handleBack while :605 `while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline)`（与 :597 同参同判据）
  - grep 全量命中与历轮完全一致：`shouldDeferSave(` 全 src 仅 targetGuard.ts:64 定义 + 上四处消费 + scripts 测试引用，**无第五消费处**。
- **targetGuard.ts:64-72 三参三分支**逐条重读：`:69` `stateData === undefined → true`（首帧未到无条件推迟）；`:70` `echoed → false`（稳态放行）；`:71` `(courses?.length ?? 0) > 0 && hasSelected → true`（回显未完成推迟）。注释（:45-63）与三分支逐条对应；第三参 echoed 只稳态放行、绝不驱动守卫判据——F42-M1「判据与数据源解耦」语义延续零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**（grep 实测与历轮记录逐字符一致）：
  - 首帧确证无旧目标（courses 空）:240-241 置 true + setEchoDone(true)
  - 合并完成 :297-298 置 true + setEchoDone(true)
  - 账号复位 :200 置 false（声明于 echoedRef/rev/setRev/setEchoDone 之后 :194，TDZ 不触发；F36-01 兜底守卫在位）
  - 首帧未到 :234 `if (stateData === undefined) return` 前置 return；:247 `pubs.length === 0` 等发布同样不置位（OBSERVE-83-01 立足点仍在）
- **读点全量清点**（grep 实测代码级引用共 9 处）：置位 3（:200/:240/:297）+ 守卫 2（回显 effect 首行短路 :229 / 独立清理 effect 守卫 :319）+ 消费点 4（:509/:597/:605/:699）。考据：无第五消费处——echoedRef 全量 grep 命中 24 行（含注释）与历轮一致，代码级 9 处零漂移。
- **target-guard 断言 18/18** 实测全绿（脚本输出逐行 ✓ 18 项 = shouldDeferSave 8 + selectedHasStalePublish 5 + cleanStaleSelected 5，含 echoed 第三参 2 条）。
- 第三十九轮结论：M-1 稳态语义消费点与 targetGuard 纯函数实现逐字符一致，延续闭合。

## 二、六防保存链（F43/F42/F40/F39-M1/F36/F48-M1）零回归

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-72 + 消费点传第三参 | 三参三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :699 stateDataRef + 防抖 effect 依赖 :755（含 stateData） | 判据只读 stateDataRef；/state 到达触发 effect 重跑自愈；echoed 第三参仅稳态放行 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立 effect :318-329 | 只删「非空且不在集合」key、空 key 保留、无变更返回原引用（:42）；依赖 [publishes, selected, echoedRef, toast] 覆盖时序巧合 |
| F39-M1 消费时刻双闸 | 防抖 :699-745 + flush :509-566 | 五判据逐条重读，全部消费时刻读最新 publishesRef/selectedRef/stateDataRef；守卫命中纯 return 不置 dirtyRef |
| F36 回显真合并 | :252-281 | 按 publish_id 真合并、:256 rev>0 且无任何条目不合并、:279 !hasTouched && !merged 保持现状不返新引用、:251 currentIds 过滤幽灵 publish_id |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :732 / flush :553 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT [] |
| key={account} | App.tsx:293-298 / :339-344 双挂载点 + Select 兜底守卫 :195-202 | 无回潮（:195-202 声明于 echoedRef/rev 之后，TDZ 不触发） |

- **setSelected 调用点清点**（grep 实测）：:198（account reset）/ :252（回显合并函数式）/ :290（回显内 cleanStale 函数式）/ :322（独立清理 effect 函数式）/ :353/:363（pick 对象式快照）——六处代码调用，无第三来源。
- **dirtyRef 置位语义清点**（grep 实测）：置 true 仅三处 :455（saveNow catch 真实失败）/ :563（flush 飞行中标记补发）/ :742（防抖飞行中标记补发）；:463 为补发前置 false。全部为真实失败/飞行中补发语义；守卫分支纯 return 零置位。零回潮。
- **unmountedRef 全读写点**（grep 实测）：挂载复位 :401 / cleanup 置 true :403 / saveNow 三守卫 :431/:442/:447 / finally 补发守卫 :459 / flush toast 守卫 :527 / 防抖 toast 守卫 :718——全部与「卸载后不 fire/不 toast」契约对齐。**注意：handleSelectClass/handleConfirmExit（手动报名/退选）的 invalidateQueries + toast 不在 unmountedRef 守卫清单内，本属历轮缺省面——经新一轮核定为可接受语义（见第四节）。

## 三、F93-01 Button focus-visible 持续复核（第十一轮）

- **ring 行在位**（实测）：Button.tsx:42 base class 含 `"focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)]"`，与历轮记录逐字符一致。git log 实测 f08937e（F93 提交）→ HEAD 无任何 `web/` 代码提交（`git log --oneline -- web/` 最新代码提交即 f08937e，之后仅 docs 提交；`git log f08937e..HEAD -- web/` **实测空集**），该行未被改动。
- **语义正确性**：`focus-visible:` 变体由 `:focus-visible` 伪类驱动——键盘 Tab 聚焦触发（焦点环可见）、鼠标点击不触发。global.css「基础交互重置」对 button 统一 `outline: none`，ring 补偿确有必要。
- **全站一致性**：全站 `variant="` 命中 30+ 处经 base class 一次收敛；Admin 开关（Admin:583 `focus-visible:ring-2 focus-visible:ring-white/60`）、Login 密码切换（Login:173 同款）、Input 自带 focus ring（Input.tsx:14 `focus:ring-1 focus:ring-[var(--cyan)]`）并存无冲突。
- **残余面确认**（本轮逐一 grep 复核行号对应裸 button 与历轮清单逐项一致，非新发现）：Dashboard CollapseSection 折叠头（:67）、Admin 复制/刷新/删除/收起/引擎切换/重试（:417/:425/:441/:452/:470/:473/:651/:661/:701）、Login 激活取消（:299）、Toast Close（:100）。OBSERVE-93-01 延续。
- F93-01 第十一轮复核通过：实质修复持续在位，无回归，残余面维持观察。

## 四、新契约角度（本轮选定 b）状态恢复与竞态（账号切换/登出/401 三路下组件状态与请求的取消/忽略）

### 请求生命周期与卸载竞态走查（Select 路由为主，App 状态机为辅）

**1. 卸载后网络请求的取消**：`api()`（client.ts:38-109）支持调用方 signal（:51-53 `rest.signal ?? ctrl.signal` 显式接入），但全前端的 react-query 查询（/state、/electives、/logs、/admin/*）**统一不传 signal**——卸载后 react-query 缓存层接管请求结果（数据进缓存、组件不 setState），无泄漏/无 setState-after-unmount 警告（React 18 已静默容忍）。手动操作（selectElective/exitElective/saveNow PUT）也不用 signal，飞行中卸载后 then/catch 仍执行。**历轮已将此剧集为「可接受——结果仍有效 + 后端幂等防线兜底」，本轮无新证据升级**。

**2. 账号切换（key={account} 重建）**：App.tsx:293-298/:339-344 双挂载点 key={account}，账号切换 = Select 整体卸载重建——selected/echoedRef/rev/actionLoading/exitModalClass 全复位，旧账号状态绝不污染新账号。**残余面**：卸载前在飞的 selectElective/exitElective 请求返回后 toast（见第 4 点）。

**3. 登出/401 剔除（会话吊销）**：App.onUnauthorized（:187-235）快照式删会话 + setCurrent 切换——若被吊销账号恰好是当前 Select 所在账号，组件同步卸载；查询失活、在飞请求结果进缓存。**残余面**：管理员代理查看学生时自身会话失效（:217-219 `isCurrentAdminSession && inAdmin && lostAccount !== adminName → return`）保管理态不误杀——历轮已钉死。**本轮核对 targetAccount 代理分支**：:204-206 `setTargetAccount(prev => prev===lostAccount || lostAccount===adminName ? null : prev)` 同时覆盖「代理学生被吊销」与「管理员自身被吊销」两路，代理 Select 卸载零残留。无新发现。

**4. 手动操作卸载后 toast（本轮唯一疑点，核定为可接受）**：handleSelectClass（:86-112）与 handleConfirmExit（:115-137）的 catch/finally 内 `toast(...)` + `invalidateQueries` 均【无】unmountedRef 守卫——与保存链「卸载后不 fire/不 toast」契约（:390-408 unmountedRef 注释 + saveNow/flush/防抖 toast 守卫 :527/:718）不对称。触发路径：用户在 Select 点报名 → 立即点返回（handleBack 在 63s 等待窗口内等待保存链静止，但手动报名不在其内）→ onDone 卸载 → 报名请求仍在飞 → 返回后 finally 内 toast「报名成功/失败」轰到已回到 Dashboard 的管理员/其他账号。**裁定不立条**：① toast 是「已完成请求」的结果反馈（非新请求、非退避轰炸循环，最多一次）；② 手动操作结果对用户仍有告知价值（操作确实完成/失败，语义与保存链「卸载后不 fire 无事发生」不同）；③ 多账号场景影响轻微（切走可忽略）。与屡轮对「保存链卸载后绝不 toast」的严格契约同源于「轰炸循环防骚扰」动机，手动操作无循环形态、无双发放大，落 OBSERVE 亦无实际依据——宁缺毋滥，仅记备注。

**5. Dashboard/Login/Admin 竞态复核**：Dashboard 无手动写操作（只读查询 + onGoSelect/onLogout 回调）；Login submit/activate 有 loading/activating 在飞幂等（:36/:68），卸载竞态不复存在（Login 只在新登录后卸载，pendingTicket 即清）；Admin 有 removing（删账号 in-flight Set）守卫。零新发现。

**结论**：状态恢复与竞态三路（账号切换/登出/401）全部由 key={account} + App 快照式状态机兜底，无新增漏洞；手动操作卸载后 toast 认定为可接受语义，仅备注。

## 五、发现清单

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### OBSERVE（本轮无新增；延续项维持）

**OBSERVE-93-01（延续，第十一轮）**：Button 组件 focus-visible ring 持续在位（Button.tsx:42 逐字符复核 + git log 实证 web/ 零代码提交）；残余面清单（Dashboard CollapseSection :67、Admin :417/:425/:441/:452/:470/:473/:651/:661/:701、Login 激活取消 :299、Toast Close :100）逐一 grep 复核无漂移。保持「续」。

**OBSERVE-90-01（延续，里程碑第五轮评估）**：Dashboard.tsx 日志区（:697-718）缺「加载中 / 拉取失败」分支——/logs 首次加载中、持续失败时 logs=undefined 均显示 "NO RECENT LOGS"。本轮重读位置无变化，与 Admin LogsTab 四态对照仍存在不对称。**里程碑第五轮评估**：显著矛盾（/logs===/state 同信道）影响面 = 仅日志区文案误导（次位表现），连续五轮无用户报告/故障样本，修复引入 8 行 + 无行为收益，**维持观察不落地**。

**OBSERVE-88-01（延续，里程碑第七轮评估）**：ErrorBoundary 缺失——本轮全仓零命中复证（componentDidCatch/getDerivedStateFromError/ErrorBoundary 零命中，main.tsx 裸 createRoot + StrictMode）。渲染期异常源为近零 + 引入复杂度，**维持观察不落地**。

**OBSERVE-85-02（延续）**：黄金期末尾 400ms 防抖竞态改动静默丢弃——安全方向刻意牺牲。本轮重推 handleBack 三轮 flush 收敛（:611-642）+ 等待窗口内 pick 新改动由「等一帧复查 revRef :622-624」覆盖，无新触发面。维持「续」。

**OBSERVE-84-01（延续）**：激活失败后票据空请求文案突变——本轮重读 Login.tsx 清票三路径（票据无效 :87-90 / 通用失败 :91-97 / 取消 Esc :232-238 + 按钮 :299-311）与历轮记录一致。维持「续」。

**OBSERVE-83-01（延续）**：Select :247 `pubs.length === 0` 分支仍在，courses 非空 + publishes 空 + echoedRef 未置位稳态组合为潜在陷阱（当前行为无害）。维持「续」。

**OBSERVE-77-02（延续）**：窗口关闭期间无目标管理入口。维持「续」。

**OBSERVE-76-01/02/03（延续）**：handleBack 等待期（最大 63s+5s）零进度反馈 / Admin uses 无前端上限（后端 1-1000 兜底）/ Toast Close 无 aria-label——均维持「续」。

**O-3 族（聚焦陷阱/滚动穿透/焦点恢复三缺）**：延续，指向 F6-02 Radix Dialog 迁移单一出口。本轮横向复核三模态（Select 退选 / Admin 删账号 / Login 激活）autoFocus 落位 + Esc 关闭全链 + Tab 顺序自然——维持观察。

### 备注（非立条）

- **手动操作卸载后 invalidate + toast（本轮走查新发现面，核定为可接受）**：handleSelectClass/handleConfirmExit 的 toast/invalidateQueries 无 unmountedRef 守卫，与保存链「卸载后不 fire/不 toast」契约不对称；裁定为可接受语义（结果反馈非循环轰炸），若未来多账号高频手动操作成为痛感再落地。

## 六、已核无缺陷清单

- M-1 延续管理（第三十九轮）：三消费点全传 echoedRef.current 第三参逐字符完整、置位三路径 + 首帧不置位边界、读点全量清点（代码级引用 9 处，无第五消费处）、target-guard 18/18 实测全绿。
- 六防保存链：各判据与注释逐条对应，防抖五判据 + flush 五判据消费时刻读最新 ref，setSelected 六代码调用点无第三来源，dirtyRef 置 true 仅三处，unmountedRef 读写点与「卸载后不 fire/不 toast」契约对齐。
- F93-01：ring 行持续在位（第十一轮复核，git log 实证 f08937e 后 web/ 零代码提交）、语义正确、全站收敛无回归、残余面清单与历轮一致。
- 新契约角度 b（状态恢复与竞态）：账号切换（key={account} 双挂载点 + 兜底守卫）/ 登出（五态复位）/ 401 剔除（快照式 + targetAccount 双路清除 + 管理态保留）三路全走查无新漏洞；Login/Admin 在飞幂等双守卫在位；手动操作卸载后 toast 核定为可接受语义。
- 契约 20 扫描：轮次前缀标签族（`第 N 轮|R9X|B/F/O[0-9X]{2}|M-1|F\d{2}A?|B\d{2}-` 全形态）/ 行号引用族（`\b\w+\.tsx?:\d+\b`）/ XSS 危险模式（dangerouslySetInnerHTML / innerHTML= / new Function / eval 零命中）/ localStorage 六处读写（App.tsx:20/:28/:39/:46/:57/:64）全 try/catch 降级 / 导航能力（window.open / location.* / history.* 零命中）——全仓零命中。（global.css:89「180% 宽」为 CSS 注释里的合法宽度值，非轮次标签。）
- 其余复跑无回潮：手动报名/退选 Set 在飞幂等 + 双 invalidate（Select :86-137）、登录/激活链幂等 + 票据贯通 + 401 三形态单广播（client.ts:64-95）、btn_type 三向、max_count=0 四处同源、useTickingCountdown target 变化校正 now、Dashboard 日期分组本地零点（parseDateKey/localTodayMs）、Toast 同 title 去重合并（标题 ReactNode 退化同文比较）、Admin 复制 clipboard 降级链（readOnly 加固在位）、Tabs 受控化（Admin :56 / Select :929）、Progress max=1 空条兜底、App 状态机九路径复位链路全闭环。

## 七、构建验证表

| 项 | 结果 |
|---|---|
| `npx tsc -b --pretty false`（web/ 下） | ✅ EXIT 0 |
| `npm run build`（web/ 下） | ✅ built in 586ms，产物 index-DoRmkvE8.js 420.02 kB / index-1KHlpqcc.css 41.72 kB 落 backend/web/dist（哈希与历轮逐字节一致，web/ 零代码改动实证） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18（脚本输出逐行 ✓ 18 项） |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6（脚本输出逐行 ✓ 6 项） |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5（脚本输出逐行 ✓ 5 项） |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉表面协调一致） |
| 契约 20 残留扫描（轮次标签族/行号族/XSS/localStorage try/catch/导航能力全形态） | ✅ 零命中 |
| `git status --short --branch` | ✅ `## master`（HEAD=a98d5e3）；除本报告外零改动（build 产物 git check-ignore 确认被忽略） |

## 八、历轮观察延续

- **M-1（echoedRef 第三参稳态语义）**：第三十九轮闭合，见上。下轮继续常规核对。
- **OBSERVE-93-01（无障碍焦点可见性）**：第十一轮持位复核通过（Button.tsx:42 ring 在位零回归，git log 实证 web/ 零代码提交）；残余面清单逐字无漂移。维持「续」。
- **OBSERVE-90-01（延续）**：Dashboard 日志区缺加载/失败态区分——位置无变化（:697-718），里程碑第五轮评估维持不落地。
- **OBSERVE-88-01（延续）**：ErrorBoundary 缺失——里程碑第七轮评估维持观察不落地。
- **OBSERVE-85-02 / 84-01 / 83-01 / 77-02 / 76-01/02/03（延续）**：全部维持历轮结论。
- **O-3 族**：聚焦陷阱/滚动穿透/焦点恢复——指向 F6-02 Radix Dialog 迁移单一出口，维持观察。
- **本轮新增备注**：手动操作（handleSelectClass/handleConfirmExit）卸载后 invalidate + toast 无 unmountedRef 守卫——核定为可接受语义，不立条，列入下轮顺带复核面。

## 九、备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + `npx tsc -b --pretty false` + `npm run build` + 四守护脚本 + git status/rev-parse/log）；工作区 `git status` 零改动（HEAD=a98d5e3，master，`## master`），未修改任何仓库代码文件，唯一写入为本报告。
- 走读推断与实测区分：M-1 逐字符、四守护脚本断言计数（18/18、6/6、5/5）、audit 全绿、tsc/build 退出码、契约 20 扫描各类、git rev-parse/log（含 `git log f08937e..HEAD -- web/` 空集实证）、F93-01 ring 行逐字符、手动操作卸载后行为均为实测/走读混合证据；「手动 toast 可接受语义」的裁定与「卸载后 react-query 请求结果进缓存不 setState」为走读推断。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；无新增 OBSERVE（宁缺毋滥）；M-1 第三十九轮闭合；连续第四十九轮无严重级发现。