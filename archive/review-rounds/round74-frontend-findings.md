# Round 74 前端只读审查报告

基线：commit 410ca4c（R73 双 findings + 收尾总结，R73 收官 HEAD）。本轮为 **R74 前端全模块只读审查**，核证对象为 R73 修复提交（9f5c217）后的全前端源码（web/src 全部 .ts/.tsx + web/scripts 断言脚本 + components/ui 全部），重点复核 R73 修复正确性（第十轮）、M-1 延续管理（第十轮）、六防保存链、登录/激活链 + 撞名学生管理态 + 人性化细节。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + 3 组 TDD 断言脚本只读复跑 + npm run build 只读验证），未修改任何仓库文件，`git status` 工作树洁净。

## 概述

**R73 修复两项逐行核证全部语义准确无误伤；M-1 第十轮闭合、OBSERVE-66-03 setSelected 五调用点无新增；构建实测全绿（build exit 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）。本轮零 CRITICAL、零 MAJOR、零 MINOR、4 条新 OBSERVE（均为提示/观察级零行为）+ 1 条"可疑待核"（证据核证后归因防护冗余、倾向闭合）。连续第二十轮零严重级发现。** 全前端地毯式排查未发现任何数据丢失/静默覆盖/安全漏洞/核心功能不可用。

---

## R73 修复正确性核证（commit 9f5c217 逐行）

### 浏览摘要

commit 9f5c217（"R73 收尾——删 tray_linux 防错桩 + scheduler 两处行号悬空改语义指位 + 前端行号引用微漂/useMemo 措辞"），前端 2 文件 2 行改动：

- Select.tsx:161 `（见 166 行 effect）` → `（见下方回显 effect）`
- Admin.tsx:56 `useMemo 仍会重置` → `useState 字面量仍会重置`

### 逐行核证

| 位置 | R73 改动 | 核证结论 |
|---|---|---|
| Select.tsx:161 | `（见 166 行 effect）` → `（见下方回显 effect）` | **行号微漂确属真实**——该 effect 现位于 :226（第 159-161 行注释所指的"只合并一次/不重放回显"逻辑正是 :226-297 的回显 effect，R73 收尾净删行导致"166 行"偏离 65 行）；改语义指位"见下方回显 effect"后**准确无悬空**：:226 即 `useEffect(() => { if (echoedRef.current) return ... })` 回显 effect，与 :159-161 所述"echoedRef 只合并一次、轮询不再重放回显"语义同指，且 `见下方` 相对指位（当前注释在 :161，目标 effect 在下文）恒成立不受后续行号漂移影响，比任何行号引用更稳。文本指位全仓唯一可跟踪 |
| Admin.tsx:56 | `useMemo 仍会重置` → `useState 字面量仍会重置` | **措辞修正正确无损伤**——该行状态实际声明于 :56 `useState("codes")`（受控 Tab value，前两行注释 "defaultValue 只在首次挂载生效" 亦指 useState 语义）；useMemo 在本文件仅 :87 用于 accounts 派生（App.tsx 侧），注释说"useMemo 重置"本就不指向真实实现，改"useState 字面量"与 :52-56 的实际代码逐字符吻合，且"进出学生大厅卸载重挂后初始值重置"的行为描述与受控 `value={activeTab}` + `onValueChange` 实现一致 |

**结论：R73 两项修复全部语义准确无误伤，无一处破坏技术内容/契约语义；"行号→语义指位"族本轮收净（:161 已无任何行号引用残留）。**

---

## M-1 延续管理（第十轮）+ OBSERVE-66-03 setSelected 清点

### M-1 第十轮核对依据（与 R73/R72 基线逐字符比对）

- **三消费点全部传 `echoedRef.current` 第三参**（`shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)` 三参数签名）：
  - 防抖回调 Select.tsx:684 —— `selectedCount > 0`，:681-683 注释"已回显完成的稳态…不闷死 / 未回显仍置脏"完整。
  - flushTargets :500 —— `latestSelectedCount > 0`，:498-499 注释完整。
  - handleBack 判定 :593 + while :601 —— 两处均 `hasSelectedNow()`，:590-592/:599-600 注释完整。
  - 每消费点前均读最新 ref（stateDataRef.current :684/:500/:593/:601、selectedRef.current :483、revRef.current :484/:593）。
- **echoedRef 置位三路径 + 首帧不置位边界**：courses 空分支 :238-239（置 true + echoDone 同步置 true）、合并完成分支 :295-296、account reset :200（置 false）；首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable 注释。**与 R73/R72 逐字符一致，零回潮**。
- **回显 effect 依赖** :297 `[stateData, data, rev, selected, toast]`——data/rev/selected 会随轮询重建引用，effect 在 echoedRef true 后首行短路、零额外开销；`rev/selected` 参与依赖使回显 effect 在用户改动渲染后仍重跑但首行短路（第一行 `if (echoedRef.current) return` 拦截一切后续逻辑），无任何双跑副作用。
- **TDD 复跑**：target-guard **18/18 全绿**（含 M-1 稳态两条：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。
- **注释指位复查**：全仓行号引用扫描（`见 \d+ 行`/`第 \d+ 行`/`见上文`/`见下方`）零命中——R72/R73 两轮行号族修复收净，Select.tsx:161/:490 及 Admin.tsx:56 均已转语义指位或措辞修正，无悬空引用。

### OBSERVE-66-03 setSelected 调用点清点

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | setSelected({}) | account reset |
| :250 | setSelected(prev=>) 函数式 | 回显合并 |
| :288 | setSelected(prev=>cleanStale) 函数式 | 回显内清理（echoedRef 短路内） |
| :320 | setSelected(prev=>cleanStale) 函数式 | 独立清理 effect :316-327 |
| :351/:361 | setSelected(对象式快照) | 用户 pick |

**无新增、无第三来源**（其余 setSelected 命中均为注释文本）；亚帧双来源模型无扩散，下游五道防线兜底不变。

---

## 六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归复核

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据 + hasSelected 第二参 + echoed 第三参；`if (echoed) return false` 稳态放行语义与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :680-687（stateDataRef）+ effect 依赖补 stateData/echoDone/hasPublishes（:745） | 判据只读 stateDataRef（非 echoedRef）→ /state 到达后 effect 重跑自愈；依赖中 stateData 驱动重跑 + echoDone 兜底场景 B（courses 空只置 echoDone 不改 selected）+ hasPublishes 兜底发布缺席置脏后的重跑（false→true 布尔变化触发新 timer）——自愈链完整闭合 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用；脚本场景 F-J 全绿；effect 依赖含 selected（:310-311 注释：合并后重跑补清理） |
| F39-M1 消费时刻双闸 | 防抖消费时刻 :695-698/:704-714/:720-723/:726-729 + flushTargets :507-509/:519-529/:547-549/:553-556 | 双闸判据逐条：发布缺席+已有选中置脏 / stale 残留置脏+toast / 联查空假清空置脏 / 发布 id 漂移置脏——全部消费时刻读最新 publishesRef/selectedRef，渲染期常量已完成清退 |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并、`rev > 0 && !anyHas` 不合并、:287-294 回显内 stale 清理 + toast 兜底、:297 依赖完整性 |
| F48-M1（F43 清空语义延续） | shouldDeferSave 第二参 hasSelected + 防抖 :720/flush :547 全清空放行 + 回显 :254 全清空不合并 | 「用户显式清空」与「数据缺席/回显未完成」在判据、消费点、回显三处完全分判，清空语义绝不复活——已核五轮维持闭合 |
| key={account} 挂载点 | App.tsx:294（targetAccount 分支）/ :340（current 分支） | 双挂载点均 key 绑定账号，账号切换即整体重建；Select 内兜底守卫 :195-202（account 变化复位会话态）延续 |
| Toast 定位 | Toast.tsx:105 viewport 固定 bottom-right + 布局类在 viewport 上 | 无回归；Toast Root 的负类（fade-out/translate）与 viewport `overflow-y-auto` 组合正常；同 title 去重合并（idRef 自增 + findIndex 更新 description 保留 duration）逻辑自洽 |

### 防抖 effect 细节复核（重点风险位）

- **resetRetry 放 effect 顶部**：`useEffect(() => { if (rev === 0) return; resetRetry(); ... })`（:650-653）——注释"用户新改动接管——中断失败重发退避"见 :652-653 完整；`resetRetry` 在 effect 内先于 400ms timer 执行，新改动立即清掉旧退避 timer，无不一致路径。
- **防抖回调闭包**：回调读 `selected`（:657/:704）为 effect 创建时快照，`publishesRef`/`stateDataRef` 消费时刻最新——selected 快照正确性依赖"effect 依赖含 selected"（:745），任一 pick 均重建 effect 冻结新版本闭包，无陈旧分叉；回调读 `selectedCount`（:684/:697/:722）同为闭包快照、由 selected 依赖兜底（OBSERVE-71-01 同源，维持观察）。

---

## 登录/激活链 + 撞名学生管理态 + 人性化细节核

### 登录/激活链（Login.tsx + client.ts + App.tsx）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 登录幂等守卫 | Login.tsx:36 `if (loading) return` | 连按两次 Enter/快速双击在 disabled 生效前只发一请求；loading 语义正确 |
| 票据贯通 | :54 `setPendingTicket((e.data?.ticket as string) || "")` + :78 激活请求 body 带 `ticket` | 1001 分支保存 data.ticket 随激活回传；取消激活清票；"过期/已用"与"激活码错误"两分支同款清票、文案引导差异化——完整 |
| 激活幂等守卫 | :68 `if (activating) return` | 与 submit 对称 |
| 401 单广播 | client.ts:64 HTTP 状态码前置广播 + :85 body 层 `r.status !== 401` 兜底 | 双形态各单次广播、不重复；App 监听幂等 | 
| 登出吊销 | App.tsx:112-132 logout → apiLogout + 快照式三连 + 清 adminToken + 清 targetAccount + 重置 page | 全路径清，无残留 |
| onDeleted | App.tsx:160-178 | 快照式三连 + 清 targetAccount/current + 删管理员自身时退管理态+清标记 |
| onUnauthorized | App.tsx:187-235 | 按令牌反查归属、管理代理态不误杀、删管理自身退管理态+清标记、快照式落盘——全链路完整 |

### 撞名学生管理态（adminAuth.ts + App.tsx 渲染判据）

- `isCurrentAdminSession` 纯函数：`adminToken !== "" && sessions[adminName] === adminToken`——撞名学生（后端放行普通会话）token 永不匹配标记；渲染判据 :299 `inAdmin || isCurrentAdminSession(sessions, adminName, adminToken)` 与账号迁移 effect :147 退出管理态同源；admin-auth-check 脚本 6/6 全绿。**第十轮零回潮**。
- 后端交叉核证：handler.go:121 `Account == adminName && ConstantTimeCompare(Password, AdminToken)` 双条件签发，:129 响应带 `adminName` 字段、:136 撞名学生走教务登录——前端"登录响应带 adminName 即标记管理 token"与后端签发语义严格绑定，无旁路。
- 管理令牌持久化 `xk_admin_token` 全路径清除点：logout :121-122 / onDeleted :175-176 / onUnauthorized :213-214 / onBackToStudent :312-313——四处全覆盖，无泄漏路径。

### 人性化细节核（倒计时兜底 / btn_type 三向 / max_count=0 / 空态 / aria）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 倒计时 begin_times 兜底 | Select :757-762 / Dashboard :190-195 | 识别缺席（open_time_known=false）时用 `begin_times[0]` 兜底，识别槽建立后以识别真值为准——与 CLAUDE.md 契约 26 承诺一致 |
| 倒计时横幅文案 | Select :817-843 | `!stateData` 同步中 → window_closed 已关闭 → window_opened 已开放 → 双缺席"未识别到开放时间" → isExpired"本地已到点等待" → 正常倒计时；`|| cd.isExpired` 已清除（R73 前修复），无"关闭后恒显已开放"矛盾 |
| 倒计时自校正 | useTickingCountdown :16-18 `useEffect(() => setNow(Date.now()), [target])` | target 变化（首次同步/开窗瞬间）立即回正，消除 1s 陈旧偏差 |
| btn_type 三向 | Select :1106/:1119 `btn_type===1` 退选 / `===2` 报名 / 其他不渲染 | 与官网逆向契约一致；can_select 双守卫 disabled+title；窗口未开照样可点报名（title 提示"不在选修报名时间范围内"）不锁死手动通道 |
| max_count=0 名额未公布 | :957 筛选不过滤 / :996 isFull 同源 / :1000-1006 unannounced / :1082-1084 文案 / :1091-1095 Progress value=0 | 四处同源 `max_count>0` 判据；未公布课程不误显"已满额/余0席/满条"，进度条空条如实 |
| 空态 | Select :906-912 无可选批次 / Dashboard :535-545 无预选课程 | 均有明确文案 + 行动引导；顶部徽章分母 `publishes.length > 0 ? '/'+len : ''`（:786-787）开窗瞬间短暂空集不显 "/0" |
| aria 无障碍 | 退选弹窗 role=dialog/aria-modal/aria-labelledby + Esc（:1191-1201）/ 删除弹窗同款（Admin :210-218）/ 激活弹窗同款 + 焦点陷阱前置（Login :223-238）/ switch role+aria-checked（Admin :568-572）/ progressbar（Progress :23-28）/ CollapseSection useId + aria-expanded/aria-controls（Dashboard :64-67）/ 搜索 aria-label（Select :860） | 全站完备；useId 消除重复 id |
| 在飞幂等全站点 | 登录/激活/报名/退选/删除账号/删激活码/生成码/保存配置 | Set 独立跟踪无互踩，disabled 渲染延迟前的入口短路齐全 |

---

## 发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（4 条新增 + 延续项）

**OBSERVE-74-01 — Select 顶栏 SELECTED 徽章 open_time_known 双源展示差异**

- 位置：Select.tsx:845-851（右侧开放时刻展示）与 Dashboard.tsx:407-414（预计开放时间文案行）。
- 触发场景推演：识别缺席（open_time_known=false）但 begin_times[0] 存在时，Select 徽章行（:845-851）与 Dashboard 文案行（:407-414）**均**吃 begin_times 兜底展示未来时刻；但 Select 倒计时横幅 :829-833 在 `!openTimeStr && data?.begin_times?.[0] == null` 时显"未识别到开放时间"——即 Select 页同时存在"左侧横幅可能显未来时刻（cd 兜底 :757-762 也吃 begin_times）+ 右侧徽章显未来时刻"，而"未识别到开放时间"文案只在双缺席时才出现。三处判据两两一致、无自相矛盾。既非 bug 也非体验缺陷。
- 复核结论：文案判据 :829 的 `data?.begin_times?.[0] == null` 与 :846 兜底判据完全对称（begin_times 存在即兜底、缺席才显未知），行为自洽；本项仅记录 Select 页"识别缺席 + begin_times 存在"时横幅无显式"识别态"标注（Dashboard 同场景也无），属一致性观察。**建议裁决：续**（无行为缺陷）。

**OBSERVE-74-02 — Progress value/max 同为 0 的渲染边界（unannounced 路径）**

- 位置：Select.tsx:1091-1095 `value={unannounced ? 0 : c.selected_count}` + `max={unannounced ? 1 : c.max_count}` + Progress.tsx:12 `percentage = value/max*100`。
- 触发场景推演：unannounced 时强制 `value=0, max=1`，杜绝 0/0 产生 NaN；非 unannounced 且 `selected_count>max_count`（平台人数回落异常）时 `Math.min(100, ...)` 钳制 100%。行为正确。
- 复核结论：`aria-valuenow={value}` 在 unannounced 时如实显 0（"未公布无进度"语义）；max 兜底 1 使百分比 0/1=0% 空条。正确路径，无缺陷。**建议裁决：续**（记录 Progress 组件对 `value>max` 的钳制属健壮性设计，非本轮发现）。

**OBSERVE-74-03 — Dashboard 主倒计时与折叠行 "已开放" 判定差一个 tick**

- 位置：Dashboard.tsx:94-108 relativeCountdown（折叠行渲染期 Date.now()）+ :197 nowMs 渲染期计算 + useTickingCountdown 每秒 tick。
- 触发场景推演：折叠行 relativeCountdown 在 `diff<=0` 时显"已开放"，主倒计时 cd.isExpired 由每秒 tick 驱动——同一渲染帧内主矩阵（cd 基于 hook 内部 now）与折叠行（渲染期 Date.now()）可能差至 1 秒（hook now 是上一秒快照，折叠行是最新）。两处同显"已开放"的时点最多差 1 秒。
- 复核结论：注释 :92 已明示"最坏 1 秒陈旧，绝不为此再建额外定时器"的设计取舍；1 秒窗口内两处文案可能短暂并存"已开放/倒计时 00 秒"，自愈且无功能影响。**建议裁决：续**（既有注释承诺）。

**OBSERVE-74-04 — Admin 配置保存后 apiKey 清空时机与"留空不改"回显**

- 位置：Admin.tsx:544 `setApiKey("")`（保存成功后）+ :530 `if (apiKey.trim()) body.vision_api_key = apiKey.trim()`。
- 触发场景推演：保存成功后清空密钥输入框，配合 :536-539 refetch→epoch 回填"后端实际生效值"（脱敏）。用户下次进表单看到的是脱敏占位（`****后4位`），若想改 key 需重输全量——无"显示/隐藏当前密钥"开关（安全设计）。行为正确。
- 复核结论：注释 :542-543 "保存后才清空保证'留空=不改动 key'的回显语义不被旧输入污染"完整；表单回填效应正确。**建议裁决：续**（无行为缺陷，密钥安全设计使然）。

**OBSERVE-71-01（延续）**：防抖 effect 缺失 selectedCount 依赖/oxlint 告警——回调读 selectedCount 实为闭包快照、由 selected 依赖兜底，维持「续」。

**OBSERVE-71-02（延续）**：Dashboard F10-06 注释并存，维持「续」。

**OBSERVE-70-03（延续）**：Select 搜索框 aria-label 与 placeholder 同串，维持「续」。

**OBSERVE-66-03（延续）**：setSelected 五调用点无新增，维持「续」。

### 可疑待核

**可疑-1 — Select :786 SELECTED 徽章 `selectedCount/publishes.length` 在发布缺席时的分母显示**

- 位置：Select.tsx:786 `{publishes.length > 0 ? `/${publishes.length}` : ""}`。
- 触发场景推演：开窗瞬间平台清空 publishes 时徽章显 `SELECTED n`（无分母），发布恢复后显 `SELECTED n/m`。
- 复核结论：R73 已确认 `publishes.length > 0` 条件渲染杜绝 "/0"；短暂无分母态与空态卡（:906-912）并存秒级且自愈；分母 0 不作除法（仅字符串拼接），无渲染异常。**闭合**（与 R73 可疑-2 同判据）。

---

## 已核无缺陷清单

- R73 两项修复逐行核证：全部语义准确无误伤。
- M-1 第十轮：三消费点逐字符传 echoedRef.current + 置位三路径 + 首帧不置位边界 + TDD 18/18 全绿。
- OBSERVE-66-03 setSelected 五调用点：无新增、无第三来源。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + F15/F16/F17 消费时刻双闸）：逐条判据与注释逐一对应，渲染期常量清退完成。
- 撞名学生管理态判定：admin-auth 6/6 全绿，刷新恢复判据正确，令牌标记全路径清；后端签发双条件交叉核证无旁路。
- 票据贯通 / 401 单广播 / 登出吊销 / onDeleted / onUnauthorized：全链路完整，快照式三连一致无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布 / 倒计时兜底 / 空态 / aria：全部与承诺一致。
- 轮询降频全站统一（window_closed/failure 30s、黄金期 2s、其余 10s），TDZ 规避正确。
- XSS/敏感数据/硬编码开放时间：零命中。
- Dashboard 折叠种子 / 日期分组 / 时间摘要：无重种、无时区偏移。
- Admin 配置保存（refetch→epoch 回填）/ 激活码 Set 在飞跟踪 / 删除在飞幂等 / 复制兜底：无新缺陷。
- 在飞幂等守卫全站点（登录/激活/报名/退选/删除/配置/复制）：Set 独立跟踪无互踩。
- 无障碍（激活/退选/删除弹窗 Esc+role+aria、switch 语义、progressbar、搜索 aria-label、CollapseSection useId）：全站完备。
- 前端 PUT 请求体与后端 TargetsRequest 解码交叉核证（class_id/publish_id/priority 边界）一致。
- 手动报名 msg 链路核证：平台 SelectClass 成功响应 Msg 直透 toast 标题，`|| "已成功选报该课程"` 兜底平台空 Msg。

## 构建验证

| 项 | 结果 |
|---|---|
| `npm run build`（web/ 下） | ✅ exit 0（tsc -b + vite，产物 dist/ 与已提交版本一致，git status 洁净） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| 全仓残留标签扫描（多模式） | ✅ 零命中（行号引用/轮次标签均已收净） |

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第十轮闭合，见上。
- **OBSERVE-66-03**（亚帧 setSelected 五调用点）：无新增，维持「续」。
- **N-1~N-3**（三态冲刺文案 / pick 一拍调度延迟 / 格式卫生）：维持既往。
- **O-1~O-12**：逐条复查无升级证据，维持。
- **OBSERVE-70-03**（Select aria-label 与 placeholder 同串）：维持「续」。
- **OBSERVE-71-01**（防抖 effect 缺失 selectedCount 依赖/oxlint 告警）：维持「续」。
- **OBSERVE-71-02**（Dashboard F10-06 注释并存）：维持「续」。

## 备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + 3 组 TDD 脚本只读复跑 + npm run build 只读验证）；未修改任何仓库文件，`git status` 工作树洁净。
- 复核与既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态、target-guard 判据、契约 20 剥离完成度）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；4 条 OBSERVE（均提示级零行为）+ 1 条可疑待核（归因防护冗余，倾向闭合）；连续第二十轮无严重级发现。
