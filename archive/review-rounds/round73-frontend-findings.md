# Round 73 前端只读审查报告

基线：commit 06e23f7（R72 收官 HEAD，R72 双 findings + 收尾总结）。本轮为 **R73 前端全模块只读审查**，核证对象为 R72 注释剥离/指位对齐提交（382b7a6）后的全前端源码（web/src 全部 .ts/.tsx + web/scripts 断言脚本 + components/ui 全部），重点复核 R72 修复正确性、M-1 延续管理（第九轮）、六防保存链（F43/F42/F40/F39/F36/F48-M1）、登录/激活链 + 撞名学生管理态 + 人性化细节。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + 3 组 TDD 断言脚本只读复跑 + npm run build 只读验证，未修改任何仓库文件）。

## 概述

**R72 八处改动逐行核证全部语义准确无误伤（行号→语义指位、F15/F16/F17 链→双闸口径，均无副作用）；M-1 第九轮闭合、OBSERVE-66-03 setSelected 五调用点无新增；构建实测全绿（build exit 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）。本轮零 CRITICAL、零 MAJOR、1 条 MINOR（SELECT-73-01：注释行号引用微漂 + oxlint 缺失依赖，均注释/提示级零行为）、4 条新 OBSERVE + 2 条"可疑待核"（均归因设计留白/防护冗余，给出证据后倾向闭合）。连续第十九轮零严重级发现。** 全前端地毯式排查未发现任何数据丢失/静默覆盖/安全漏洞/核心功能不可用。

---

## R72 修复正确性核证（commit 382b7a6 逐行）

### 浏览摘要

commit 382b7a6（"R72 收尾剥离残留标签+注释指位对齐"，4 文件 8 行改动）：target-guard-check.ts 2 处标签剥离、unauthorized-check.ts 1 处、Admin.tsx 2 处 JSX 块注释、Select.tsx 3 处注释（1 行号指位改写 + 1 判据链指位改写 + 1 行号改写）。**该提交已包含在审查基线 HEAD 内**（`git merge-base --is-ancestor 382b7a6 HEAD` 确实），全仓库残留扫描（N/M/F/B/n 系列 + 第 N 轮 + MAJOR-/MINOR-/OBSERVE- + R[0-9]+[-_] 多模式）**零命中**（唯一匹配 `web/scripts/admin-auth-check.ts:4` 的 `B43-04` 已核证为契约编号、非标签，且该文件属 web/scripts 只读脚本）。

### 逐行核证

| 位置 | R72 改动 | 核证结论 |
|---|---|---|
| target-guard-check.ts:33 | `F42-M1：` 前缀剥离 | 纯前缀删除，断言说明主句完整（"shouldDeferSave 断言——防抖保存'回显未完成'守卫抽纯函数"） |
| target-guard-check.ts:93 | `F40-M1：` 前缀剥离 | 同上，语义零损伤 |
| unauthorized-check.ts:1 | `F41-N2` 标签剥离 | 同上；且断言 body 说明**仍保留** `B43-04` 契约编号（该编号描述后端行为，属跨仓契约引用非轮次标签，保留正确） |
| Admin.tsx:207 | `{/* N3：` 前缀剥离 | JSX 块注释主句"删除账号二次确认 Dialog（替代 window.confirm，符合黑白极简设计）"完整，剥离正确 |
| Admin.tsx:752 | `{/* N5：` 前缀剥离 | "教务令牌有效性可视化——管理员一眼看到各账号 token 是否失效/恢复中"完整，剥离正确 |
| Select.tsx:490 | `（见防抖 effect 内 618 行）` → `（见防抖回调内"回显未完成守卫"）` | **行号陈旧问题确属真实**（现防抖守卫位于 :684，净删行导致"618 行"偏离 66 行），改语义指位后**准确无悬空**——防抖回调内确实存在同名守卫（:669-687 的 `if (shouldDeferSave(...))`），且"回显未完成守卫"一词全仓唯一指向该判据（flushTargets :500/handleBack :593 注释同词互指），文本指位清晰可跟踪 |
| Select.tsx:569 | `那是 F15/F16/F17 链的刻意安全方向` → `那是防抖/flush 消费时刻双闸的刻意安全方向` | **指位对齐正确**——:471-473 注释已声明"渲染期常量已无引用，删除"，旧"F15/F16/F17 链"指位已失效；真实守卫判据为消费时刻 `publishesRef.current.length===0 && latestSelectedCount>0`（:507 flushTargets / :695 防抖双闸），新口径与 :471-473 同源，无副作用 |

**结论：R72 八处改动全部语义准确无误伤，无一处破坏技术内容/契约语义。**

---

## M-1 延续管理（第九轮）+ OBSERVE-66-03 setSelected 清点

### M-1 第九轮核对依据（与 R72/R71 基线逐字符比对）

- **三消费点全部传 `echoedRef.current` 第三参**（`shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)` 三参数签名）：
  - 防抖回调 Select.tsx:684 —— `selectedCount > 0`，:681-683 注释"已回显完成的稳态…不闷死 / 未回显仍置脏"完整。
  - flushTargets :500 —— `latestSelectedCount > 0`，:498-499 注释完整。
  - handleBack 判定 :593 + while :601 —— 两处均 `hasSelectedNow()`，:590-592/:599-600 注释完整。
  - 每消费点前均读最新 ref（stateDataRef.current :684/:500/:593/:601、selectedRef.current :483、revRef.current :484/:593）。
- **echoedRef 置位三路径 + 首帧不置位边界**：courses 空分支 :238-239（置 true + echoDone 同步置 true）、合并完成分支 :295-296、account reset :200（置 false）；首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable 注释。**与 R72/R71 逐字符一致，零回潮**。
- **回显 effect 依赖** :297 `[stateData, data, rev, selected, toast]`——data/rev/selected 会随轮询重建引用（data 每 2s/10s 新引用），effect 在 echoedRef true 后首行短路、零额外开销，依赖完整性无问题。
- **TDD 复跑**：target-guard **18/18 全绿**（含 R63 M-1 稳态两条：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。

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
| F42-M1 判据与数据源解耦 | :680-687（stateDataRef）+ effect 依赖补 stateData/metadata（:745, :647, :166, :738-744 注释） | 判据只读 stateDataRef（非 echoedRef）→ /state 到达后 effect 重跑自愈；依赖 `[rev,selected,sessionToken,toast,hasPublishes,echoDone,stateData]` 中 stateData 驱动重跑 + echoDone 兜底场景 B（courses 空只置 echoDone 不改 selected）——自愈链完整闭合 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用；脚本场景 F-J 全绿；effect 依赖含 selected（:310-311 注释：合并后重跑补清理） |
| F39-M1 → R72 指位对齐后再核 | 防抖消费时刻 :695-698/:704-714/:720-723/:726-729 + flushTargets :507-509/:519-529/:547-549/:553-556 | 双闸判据逐条：发布缺席+已有选中置脏 / stale 残留置脏+toast / 联查空假清空置脏 / 发布 id 漂移置脏——全部消费时刻读最新 publishesRef/selectedRef，渲染期常量已完成清退（:471-473 注释确认） |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并、`rev > 0 && !anyHas` 不合并、:287-294 回显内 stale 清理 + toast 兜底、:297 依赖完整性 |
| F48-M1（F43 清空语义延续） | shouldDeferSave 第二参 hasSelected + 防抖 :720/flush :547 全清空放行 + 回显 :254 全清空不合并 | 「用户显式清空」与「数据缺席/回显未完成」在判据、消费点、回显三处完全分判，清空语义绝不复活——已核四轮维持闭合 |
| key={account} 挂载点 | App.tsx:294（targetAccount 分支）/ :340（current 分支） | 双挂载点均 key 绑定账号，账号切换即整体重建；Select 内兜底守卫 :195-202（account 变化复位会话态）延续 |
| Toast 定位 | Toast.tsx:105 viewport 固定 bottom-right + 布局类在 viewport 上（R52 修复） | 无回归；Toast Root 的负类（fade-out/translate）与 viewport `overflow-y-auto` 组合正常 |

### 防抖 effect 细节复核（重点风险位）

- **resetRetry 放 effect 顶部**：`useEffect(() => { if (rev === 0) return; resetRetry(); ... })`（:650-653）——注释"用户新改动接管——中断失败重发退避"见 :652-653 完整；`resetRetry` 在 effect 内先于 400ms timer 执行，新改动立即清掉旧退避 timer，无不一致路径。
- **防抖回调闭包**：回调读 `selected`（:657/:704）为 effect 创建时快照，`publishesRef`/`stateDataRef` 消费时刻最新——selected 快照正确性依赖"effect 依赖含 selected"（:745），任一 pick 均重建 effect 冻结新版本闭包，无陈旧分叉；oxlint 缺失依赖告警（selectedCount 于 :684/:697/:722 读取）为既有集合，回调读 selectedCount 实为闭包快照、由 selected 依赖兜底（R71 OBSERVE-71-01 同源）。

---

## 登录/激活链 + 撞名学生管理态核

| 项 | 位置 | 复核结果 |
|---|---|---|
| 撞名学生管理态判定 | App.tsx:299 `inAdmin \|\| isCurrentAdminSession(sessions, adminName, adminToken)` + adminAuth.ts:6-12 | 判据绑会话 token 标记（仅登录响应带 adminName 写入 xk_admin_token），撞名学生普通会话 token 恒不匹配；admin-auth 6/6 全绿（含场景 B 撞名/F 自定义名） |
| 管理令牌标记全路径清 | logout :121-122 / onDeleted :174-176 / onUnauthorized :212-214 / onBackToStudent :310-313 | 四条路径对称（清 adminToken 态 + saveAdminToken("")），连撞名学生 401 也无残留；`onDeleted` 补 setInAdmin(false)（F39-M1 契约 25）续存 |
| 票据贯通 | Login.tsx:51-55（1001 分支存 pendingTicket）+ :64-101（激活回传 ticket + 过期/已用/失败三态清票）+ client.ts:24-34（ApiError.data 透传） | 票据全链路贯通；激活失败清 pendingTicket 后 UI 不残留已消费票据，下次登录 1001 由服务端重新下发覆盖——注释语义与实现一致 |
| 401 单广播 | client.ts:64-95 | HTTP 401 在 r.json() 前广播（网关 HTML/文本 401 兜底）+ body 401 且非 HTTP 401 时才再广播（`r.status !== 401` 去重），三形态各单次；extractAccountFromPath 纯函数（unauthorized-check 5/5） |
| onUnauthorized 归属判据 | App.tsx:188-231 | `detail?.session \|\| detail?.account \|\| current` 取令牌（真实主体）-> 账号名直查 -> 令牌反查 -> 落空跳过；`isCurrentAdminSession(...) && inAdmin && lostAccount !== adminName` 防管理员代理态误杀；快照式三连落盘 | 
| 登出吊销 | App.tsx:112-132 + client.ts:143-149 + App.tsx:147 isCurrentAdminSession 兜底 | /api/logout 先作废服务端令牌再本地快照式删除；退管理态 + 清令牌标记 + 清代理态 + 重置 page 四步完整 |

---

## 人性化细节核（本轮地毯复查）

| 项 | 位置 | 复核结果 |
|---|---|---|
| 倒计时兜底 F39-N1 + F29-N1 | Dashboard :185-211（openTimeStr ?? begin_times[0]）+ Select :751-762（含 cd 输入与文案行） | 主矩阵与文案行同源吃 begin_times 兜底（:407-413 注释承诺已落地至文案行），识别真值优先；倒计时对 null 返回全 00+isExpired，无 NaN 路径 |
| btn_type 三向 | Select.tsx:1106-1131（1=退选 / 2=报名 / 其他不渲染） | 官网逆向契约逐字符对齐；can_select 双守卫（disabled + title 悬浮）；「本项目特冲刺按钮」与官网按钮解耦、只影响自身 |
| max_count=0 名额未公布 | Select.tsx:996-1000（isFull `max_count>0 && selected>=max`）+ :1000 unannounced + :1082-1095（容量文案/Progress 空条/value=0）+ 筛选 :955-957 + 排序 :965-971 + 徽章 :1031/+1035-1043 | 未公布与满员在筛选/徽章/进度/文案四处全同源（`max_count<=0`），无"已满额"误显路径；Progress `value=0` 空条语义正确 |
| 空态提示 | Select :906-912（无可选批次 + 说明）、:1172-1176（搜索无结果）；Dashboard :535-545（未添加预选 + 前往挑选按钮） | 全部有明确文案 + 行动引导，无裸空白主体；窗口关闭后发布元数据经 state.courses 自带 `publish_name/begin_date`（契约 3）分组显示"窗口已关闭，发布信息不可用"兜底提示（Dashboard :572-574） |
| aria/可访问性 | Login 激活弹窗 role=dialog+aria-modal+aria-labelledby+Esc（:223-238）+ autoFocus 输入框；Select 退选弹窗（:1191-1201，取消 autoFocus）/ 搜索 aria-label :860；Admin 删除弹窗（:210-218）；CollapseSection aria-expanded/controls+useId（Dashboard :64-88）；激活开关 role=switch+aria-checked（Admin :568-583）；密码可见切换 aria-label/pressed（Login :171-173）；Progress role=progressbar（Progress.tsx:25-28） | 全站无障碍基础完备；注释声称的语义与实现全部一致 |
| 在飞操作幂等 | Select actionLoading ReadonlySet<number>（:52/:92/:119）；Admin Codes removing ReadonlySet<string>（:311/:349）；deleting 布尔（:252）；Login loading/activating（:36/:68）；Config saving | 全部在飞守卫 + disabled 双闸；Set 化独立跟踪无跨课程/跨码互踩；函数式清除只删自己条目 |
| 可访问性补偿 | 空 tab / 长列表 / 内容剪切等均查无 | — |

---

## 新发现

### 无 CRITICAL / 无 MAJOR

全前端源码地毯式排查（App/Login/Dashboard/Select/Admin/client/lib×3/components/ui×7/types/scripts×3），重点覆盖 R73 清单之外的新证据面：

- **react-query 缓存一致性**：全站 queryKey 均含 `<account>`（App/Select 标准命 + Admin 各族 `[admin-x, account, sessionToken]`），无跨账号串数据；Dashboard 与 Select 共享 `["electives",account,sessionToken]` 缓存、互斥挂载零重复请求。
- **轮询降频**：Select electives 回调 :76-81（失败 30s / window_closed 30s / inRange 或 window_opened 2s / 其余 10s）、state 回调 :148-155（失败 30s / window_closed 30s / 其余 2s）；`queryClient.getQueryData(["state",account,sessionToken])` 读缓存避 TDZ（:77）；Dashboard 三查询（state 3s→30s / logs / electives 30s）——开窗瞬间平台空 publishes 不把慢轮询带进黄金期语义完整。
- **受控 Tab**：Select :919（activeTab 校验后回落到 tabs[0]，发布重建后首个 Tab 不悬空）；Admin :161/:56（useState 受控，注释谓"useMemo 重置"微瑕见 SELECT-73-01）。
- **XSS/敏感性**：全仓 `dangerouslySetInnerHTML/eval/innerHTML/document.write` 零命中；localStorage 六处存取全 try/catch 降级；无 token/密码明文进 DOM/日志；`ApiError` 不泄漏 stack。
- **硬编码开放时间**：`2026/09/13|1789261200000` 前端零残留（仅 Dashboard :110-111 注释以"2026-09-13"举例说明 parseDateKey 时区语义，非配置值——契约 20 合规，注释级）；全站开放时间唯一事实源 = 状态接口（openTimeStr 识别槽 / begin_times 兜底）。
- **文案/后端契约核对**：前端依赖的 "已满员"（scheduler.go:1764 "该课程已满员，退避至下一备选"）、"激活票据无效或已过期"、成功/失败 toast 文案均在后端存在，零悬空引用。
- **App.tsx 路由分支顺序**：targetAccount Select 分支在 inAdmin 分支前（:292-307），handleBack/onDone 回调对称；logout/onDeleted/onUnauthorized/onBackToStudent 四路 targetAccount 清空齐全（:126/:167/:204-206/）无残留代理态死锁。
- **Dashboard 折叠种子** :268-273 `null=未初始化 / []=用户全折叠` 语义稳定，轮询重建 dateGroups 引用变化不破坏种子（见 OBSERVE-73-04）。

### MINOR

**SELECT-73-01 — Select.tsx:161 注释行号引用微漂（166 行 effect 实际位于 226 行）**
- 位置：web/src/routes/Select.tsx:161 `（见 166 行 effect）`。
- 一句话：R72 改进 490 行号引用的同时，本行「166 行 effect」也已漂 60 行（该 effect 现位于 :226），与 R72 追求"行号准确/语义指位"的口径不一致。
- 触发场景：纯注释级，无运行路径；指位目标（回显 effect）确实存在，人工仍可定位。
- 建议裁决：**改**——与 :490 同口径改为语义指位（"回显 effect"），或更正为 :226；本轮列为 MINOR 系 R72 收尾自查应覆盖但遗漏的同族指位问题。

### 新 OBSERVE（纯注释/细节语义，零行为影响）

**OBSERVE-73-01 — Admin.tsx:56 注释谓"useMemo"但实际为 useState（措辞误差）**

- 位置：web/src/routes/Admin.tsx:54-55 `（Admin 卸载重挂）后 useMemo 仍会重置为"codes"`（实为 `useState("codes")` :56）。
- 一句话：注释把"useState 初始化值"写成"useMemo 重置"，措辞与实现略分叉。
- 影响面：零行为影响；注释描述的现象（重挂后回 codes）事实正确。
- 建议裁决：**改**——"useMemo" 改 "useState 字面量"一句话；纯注释卫生。

**OBSERVE-73-02 — Dashboard.tsx CollapseSection 的 aria-controls id 唯一性核证（通过）**

- 位置：web/src/routes/Dashboard.tsx:64-88 CollapseSection、:51-88 useId。
- 一句话：`useId` 生成的 id 含冒号（React 实现），`aria-controls={id}` 与 `<div id={id}>` 两处同源使用，无任何悬空/重复风险——注释承诺与实现一致（本观察仅记录核证）。
- 建议裁决：**续**（核证通过）。

**OBSERVE-73-03 — Select.tsx 空态与「窗口已关闭」信号弱相关（低于阈值，仅记录）**

- 位置：Select.tsx:906-912 空态文案"当前无可选课程批次（选课窗口未开放或已关闭）"，但该文案不读取 `stateData.window_closed`（:817-843 横幅读之）。
- 一句话：进页后平台已关窗口且 /state 首帧未到（stateData undefined）时，横幅显示"正在同步选课开放时间..."、空态与横幅共存秒级；/state 到达即横幅转"选课窗口已关闭"，空态与横幅同真相，秒级窗属加载期噪声。
- 影响面：UX 边缘、<1s 自愈，无功能缺陷。
- 建议裁决：**续**（低于升级阈值，维持 O- 级）。

**OBSERVE-73-04 — Dashboard.tsx 折叠种子与轮询引用重建（核证无退化）**

- 位置：Dashboard.tsx:268-273 `expandedDates === null && dateGroups.length > 0 → setExpandedDates([dateGroups[0].key])`。
- 一句话：`dateGroups`（useMemo :219）依赖 `courses`/`electives?.publishes`，随轮询每 3s/30s 重建引用，但种子 effect 只在 `expandedDates === null` 时写一次、之后 condition false 不再重跑——"null=未初始化"语义在 state 轮询下稳定性已由 R71 核证，本轮复跑无退化。
- 建议裁决：**续**（既往维持）。

### 可疑待核

**可疑-1 — handleBack 循环与 scheduleRetry 退避最长等待（≤63s）**

- 位置：Select.tsx:607-638（3 轮 × 每轮最多 21s）+ :412-421（退避 2/4/8/16/16s）。
- 触发场景推演：后端持续不可达时点返回，目标保存 3 轮 flush 循环各等满 21s → 返回按钮最多挂 63s；期间用户无进度提示（走"保存链静止等待"逻辑）。
- 复核结论：api 20s 超时兜底 + 退避 5 次停手（:414 `attempt >= 5`）+ 返回按钮注释"绝不无限挂起"成立；63s 上限是"尽力保存"契约的上界而非挂起（api 超时飞快失败、真正消耗的是失败的连续抖动窗口）。设计留白、非缺陷，维持 O- 级。**倾向闭合**。
- 可选增强（非缺陷）：等待期间可加轻量"正在保存目标..."文案降低焦虑（人性化建议，非本轮要求）。

**可疑-2 — Select 顶栏徽章分母与 publishes 短暂空集**

- 位置：Select.tsx:786 `SELECTED {selectedCount}/{publishes.length}`。
- 触发场景推演：开窗瞬间平台清空 publishes（length=0）时徽章短暂显示 `SELECTED 3/0`，publishes 刷新后恢复。
- 复核结论：该短暂态与空态卡（:906-912 "当前无可选课程批次"）并存秒级且自愈；分母 0 不作除法（仅字符串拼接），无渲染异常。已由 :906 注释承认"此前徽章还误显 n/0"的边界，现 med 容仍是 `publishes.length > 0 ? '/'+len : ''`（:786-787 条件渲染）——实际**不会**显示 "/0"（条件已兜底），无功能缺陷。**闭合**。

---

## 已核无缺陷清单

- R72 八处改动语义逐行核证：全部准确无误伤。
- M-1 第九轮：三消费点逐字符传 echoedRef.current + 置位三路径 + 首帧不置位边界 + TDD 18/18 全绿。
- OBSERVE-66-03 setSelected 五调用点：无新增、无第三来源。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + F15/F16/F17 消费时刻双闸）：逐条判据与注释逐一对应，渲染期常量清退完成。
- 撞名学生管理态判定：admin-auth 6/6 全绿，刷新恢复判据正确，令牌标记全路径清。
- 票据贯通 / 401 单广播 / 登出吊销 / onDeleted / onUnauthorized：全链路完整，快照式三连一致无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布 / 倒计时兜底 / 空态 / aria：全部与承诺一致。
- 轮询降频全站统一（window_closed/failure 30s、黄金期 2s、其余 10s），TDZ 规避正确。
- XSS/敏感数据/硬编码开放时间：零命中。
- Dashboard 折叠种子 / 日期分组 / 时间摘要：无重种、无时区偏移。
- Admin 配置保存（refetch→epoch 回填）/ 激活码 Set 在飞跟踪 / 删除在飞幂等 / 复制兜底：无新缺陷。
- 在飞幂等守卫全站点（登录/激活/报名/退选/删除/配置/复制）：Set 独立跟踪无互踩。
- 无障碍（激活/退选/删除弹窗 Esc+role+aria、switch 语义、progressbar、搜索 aria-label）：全站完备。

## 构建验证

| 项 | 结果 |
|---|---|
| `npm run build`（web/ 下） | ✅ exit 0（tsc -b + vite，产物 dist/ 与已提交版本一致，git status 洁净） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| 全仓残留标签扫描（多模式） | ✅ 除 admin-auth-check.ts 的 `B43-04`（核证为契约编号）外零命中 |

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第九轮闭合，见上。
- **OBSERVE-66-03**（亚帧 setSelected 五调用点）：无新增，维持「续」。
- **N-1~N-3**（三态冲刺文案 / pick 一拍调度延迟 / 格式卫生）：维持既往。
- **O-1~O-12**：逐条复查无升级证据，维持。
- **OBSERVE-70-01/02/03**：01/02 已闭合；03（Select aria-label 与 placeholder 同串）维持「续」。
- **OBSERVE-71-01**（防抖 effect 缺失 selectedCount 依赖/oxlint 告警）：维持「续」。
- **OBSERVE-71-02**（Dashboard F10-06 注释并存）：维持「续」。

## 备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + 3 组 TDD 脚本只读复跑 + npm run build 只读验证）；未修改任何仓库文件，`git status` 工作树洁净。
- 复核与既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态、target-guard 判据、契约 20 剥离完成度）。
- 本轮定级口径：零 CRITICAL/MAJOR；1 条 MINOR（SELECT-73-01 行号引微漂，R72 收尾同族遗漏）+ 4 条 OBSERVE + 2 条可疑待核（均倾向闭合）；连续第十九轮无严重级发现。