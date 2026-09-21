# Round 72 前端只读审查报告

基线：commit 175ed72（R71 收官 HEAD，即"R71 补剥离前端残留 N 系列标签"）。本轮为 **R72 前端全模块只读审查**，核证对象为 R71 注释剥离后全前端源码（web/src 全部 .ts/.tsx + web/scripts 断言脚本），重点复核注释剥离完整性（契约 20）、M-1 延续（第八轮）、六防保存链、登录/激活链、人性化细节。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令，未修改任何仓库文件）。

## 概述

**R71 注释剥离逐行核证通过（无一处剥离破坏技术内容/语义，仅 Admin.tsx 2 处 N 系列标签 + TDD 脚本 3 处注释标签残留，均为剥离遗漏非语义破坏）；M-1 第八轮闭合、OBSERVE-66-03 setSelected 五调用点无新增；构建验证全绿（target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）。本轮零 CRITICAL、零 MAJOR、零 MINOR，仅 5 条新 OBSERVE（2 条契约 20 剥离遗漏 + 3 条注释/维护性观察，零行为影响）+ 3 条"可疑待核"（均归因设计留白、给出证据与裁决建议后闭合）。连续第十八轮零严重级发现。**

---

## R71 注释剥离完整性核（契约 20）

### 核证方法

对 R71 两个前端剥离提交做**逐行 diff 核读** + 全仓残留扫描：

- `4a9dea4`（R71 主剥离：App 28 处 / client 24 处 / Toast 10 处 / adminAuth 2 处 / targetGuard 6 处 / useTickingCountdown 4 处 / Admin 42 处 / Dashboard 22 处 / Login 24 处 / Select 全量）+ `175ed72`（补剥离残留 16 处，6 文件）。
- 逐 diff 行核对：剥离内容是否仅为 `N\d+` / `M\d+` / `F\d+` / `B\d+` / `R\d+` / `MAJOR-x` / `n\d+` 等**轮次标签前缀**，或标签前缀在句子中剥离后语义是否完整。

### 逐文件抽样核证（选取改动最密集处）

| 文件 | 抽样剥离 | 语义完整性判定 |
|---|---|---|
| Select.tsx | `F42-M3：` / `F9-05：` / `F26-03：` / `M29-01：` / `F5-05：` / `MAJOR-G：` 前缀剥离 | 均只去标签，"轮询判定来源：in_date_range 与 window_opened 双信号合并"等主句完整，技术内容零损伤 |
| Admin.tsx | `M28-01：` / `F12-M2：` / `F19-03：` / `F20-02：` / `F30-01：` / `n11：` 前缀剥离；`（见 review-round13 F13-M2）` → `（见工程记忆库契约文档）` 指位改写 | 指位改写仍可追溯，技术内容完整 |
| App.tsx | `N4：` / `M-7：` 前缀剥离 | 完整 |
| Dashboard.tsx | `N1：` / `N2：` / `N10：` 前缀剥离 | 完整 |
| client.ts | `M-7` 剥离 | 完整 |
| types.ts | `N3：` / `N5：` 剥离 | 完整 |
| Toast.tsx / Login.tsx | `F7-03` 等剥离 | 完整 |

### 残留扫描（`grep -rnE "N[0-9]+[：:]|M[0-9]+[：:]|F[0-9]+[：:]|B[0-9]+[：:]|n[0-9]+[：:]|第[0-9]+轮|MAJOR-|MINOR-|OBSERVE-|R[0-9]+[-_]|N\d+-\d+|F\d+-\d+|B\d+-\d+"`）

**web/src 残留 2 处**（见新 OBSERVE-72-01 / OBSERVE-72-02），**web/scripts 残留 3 处**（TDD 脚本注释，见 OBSERVE-72-03）。三者均为**剥离遗漏**而非语义破坏——残留标签后续一并在注释中交代了"为什么/契约"语义，技术内容零损伤，判定「契约 20 遗留收尾项」，非缺陷。

### 结论

**R71 剥离未破坏任何技术内容/语义。** 逐行 diff 核证全部命中"仅标签前缀被摘除"，主句、因果链、判据说明完整保留；剥离中唯一一次"指位改写"（Admin.tsx review-round13 引用 → 工程记忆库契约文档）方向正确、可追溯。契约 20 落地基本完成，残留 5 处标签纯属遗漏，建议随下次前端卫生提交顺带收尾。

---

## M-1 延续管理（第八轮）+ OBSERVE-66-03 setSelected 清点

### M-1 第八轮核对依据

- **三消费点逐字符传 `echoedRef.current`**（全部为 `shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)` 三参数签名）：
  - 防抖回调 Select.tsx:684 —— `selectedCount > 0`，:681-683 注释 R63 M-1 语义完整（"已回显完成的稳态…放行普通编辑；未回显仍置脏等回显合并自愈"）。
  - flushTargets :500 —— `latestSelectedCount > 0`，:498-499 注释完整。
  - handleBack 判定 :593 + while :601 —— 两处均 `hasSelectedNow()`，:590-592/:599-600 注释完整。
- **三消费点消费前读最新 ref**：stateDataRef.current（:684/:500/:593/:601）、selectedRef.current（:483）、revRef.current（:484/:593）——与 R63 M-1 基线一致。
- **echoedRef 置位三路径 + 首帧不置位边界**：courses 空 :238-239 置位（echoDone 同步置 true 双信号）、合并完成 :295-296 置位、account reset :200 置 false；首帧未到 :232 `if (stateData === undefined) return` 前置 return **绝不置位**；独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable 注释——与 R71 基线逐字符一致，零回潮。
- **TDD 脚本复跑**：target-guard 18/18 全绿（含 R63 M-1 稳态两条断言：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。注意 TDD 脚本为 **2 参数** `shouldDeferSave(state, hasSelected)` 调用，而源实现第三参数 `echoed` 有默认 undefined 语义——2 参数调用等价于 `echoed=false`，与"首帧未到/未回显推迟"断言目标一致，不构成脚本-实现分叉（见下条 TDD 口径 OBSERVE-72-04）。

### OBSERVE-66-03 setSelected 调用点清点

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | setSelected({}) | account reset |
| :250 | setSelected(prev=>) 函数式 | 回显合并 |
| :288 | setSelected(prev=>cleanStale) 函数式 | 回显内清理（echoedRef 短路内） |
| :320 | setSelected(prev=>cleanStale) 函数式 | 独立清理 effect(:316-327) |
| :351/:361 | setSelected(对象式快照) 函数式更新 | 用户 pick |

**无新增、无第三来源**（其余 setSelected 命中均为注释文本）；亚帧双来源模型无扩散，下游五道防线兜底不变。

---

## 六防保存链零回归复核

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F15/F16/F17 假清空守卫链 | flushTargets :507-509 / :547-549、防抖 :695-698 / :720-722、targetsUseCurrentPublishes :553-556 / :726-729 | 消费时刻 `publishesRef.current.length===0 && selectedCount>0` 置脏、`targets.length===0 && latestSelectedCount>0` 置脏、`!targetsUseCurrentPublishes` 置脏，全部判据与守卫注释逐字符一致；`every` 空集恒真语义注释到位 |
| F42/F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点 | 纯数据判据 + hasSelected 第二参 + echoed 第三参，脚本 18/18 全绿 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用——脚本场景 F-J 全绿 |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 合并按 publish_id 真合并、`rev > 0 && !anyHas` 不合并、:287-294 回显内 stale 清理 + toast 兜底 |
| key={account} 挂载点 | App.tsx :294（targetAccount 分支）/ :340（current 分支） | 两挂载点均 key 绑定账号，账号切换即整体重建 |
| Toast 定位 | Toast.tsx :105 viewport 固定 bottom-right | 定位类在 viewport 上（R52 修复），无回归 |

### 复核发现（非缺陷，注释与实现分叉）

**发现 1：Select.tsx:569 清空守卫注释误引已删渲染期常量 `F15/F16/F17 链`——注释说"publishes 恒空"守卫是"F15/F16/F17 链的刻意安全方向"，但守卫的真实判据是 `publishesRef.current.length===0 && latestSelectedCount>0`（:507），而非渲染期常量；且 :471-473 注释已明确"渲染期常量已无引用，删除"。同一守卫机制两处注释指位错位，属 R71 剥离后清理链的注释残留。**

- 位置：Select.tsx:569。
- 触发场景：纯注释级，不触发任何运行路径。
- 建议裁决：**MINOR 级注释卫生项**——:569 的 "F15/F16/F17 链" 指位应与 :471-473 同口径改为"防抖/flush 消费时刻双闸"或直接指位 `flushTargets :507 守卫`；纯注释修正，无行为影响。判定为 OBSERVE-72-05（不单独立 MAJOR/MINOR，因零行为影响；若项目坚持"注释必须与实现同源"，应随下次前端卫生提交修正）。

---

## 登录/激活链 + 撞名学生管理态核

| 项 | 位置 | 复核结果 |
|---|---|---|
| 撞名学生管理态判定 | App.tsx:299 `inAdmin \|\| isCurrentAdminSession(sessions, adminName, adminToken)` + adminAuth.ts:6-12 | 判据绑会话 token 标记（仅登录响应带 adminName 写入 xk_admin_token），撞名学生普通会话 token 恒不匹配；TDD 脚本 6/6 全绿（含场景 B/F） |
| 票据贯通 | Login.tsx:51-55（1001 分支存 pendingTicket）+ :76-79（激活请求回传 ticket）+ client.ts:24-34（ApiError.data 透传） | 票据全链路贯通，注释声称的 F11-A1 语义与实现逐字符一致 |
| 401 单广播 | client.ts:64-95 | HTTP 401 在 r.json() 前广播 + body 401 且非 HTTP 401 时才再广播（r.status!==401 守卫），三形态各单次广播，B45-N1 注释语义与实现一致 |
| 登出吊销 | App.tsx:112-132 + client.ts:143-149 | /api/logout 先作废服务端令牌再本地快照式删除，与注释一致 |

---

## 人性化细节核

| 项 | 位置 | 复核结果 |
|---|---|---|
| 倒计时兜底 F39-N1 | Dashboard.tsx:190-195 / Select.tsx:757-762（openTimeStr ?? begin_times[0]） | 识别缺席时吃 begin_times[0]，避免"主矩阵 00 + 列表有值"自相矛盾；识别真值优先兜底只在缺席生效 |
| btn_type 三向 | Select.tsx:1106-1131（1=退选红 / 2=报名白 / 其他不渲染） | 官网逆向契约逐字符对齐；can_select 双守卫（disabled + title） |
| max_count=0 名额未公布 | Select.tsx:996-1000（isFull 判据 `max_count>0 && selected>=max`）、:1000 unannounced、:1082-1095 容量文案/Progress 空条 | 与后端 IsClassFull 同源，0=未公布非满员，筛选/徽章/进度/文案四处同源，注释完整 |
| 空态提示 | Select.tsx:906-912（无可选批次空态）、:1172-1176（搜索无结果）；Dashboard.tsx:535-545（未添加预选课程空态） | 均有明确文案 + 行动引导，无裸空白主体 |
| aria/可访问性 | Login 激活弹窗 role=dialog+Esc（:223-238）、Select 退选弹窗（:1191-1201）、Admin 删除弹窗（:210-218）、CollapseSection aria-expanded/controls+useId（Dashboard:64-88）、激活开关 role=switch+aria-checked（Admin:568-583）、搜索框 aria-label（Select:860）、密码可见切换 aria-label/pressed（Login:171-173） | 全站无障碍基础完备，注释声称的语义与实现一致 |

---

## 新发现

### 无 CRITICAL / 无 MAJOR / 无 MINOR（连续第十八轮）

全前端源码地毯式排查，重点覆盖 R72 清单之外的新证据面：

- **App.tsx 双 Select 挂载点 / 路由分支顺序**：targetAccount 分支在 inAdmin 分支前（:292-299），渲染路由与 handleBack/handleDone 回调对称；logout/onDeleted/onUnauthorized 三路 targetAccount 清空齐全（:126/:167/:204-206），无残留代理态死锁路径。
- **Select.tsx 轮询区间**：electives 回调 :76-81 失败态 30s、window_closed 30s、inRange/window_opened 2s、其余 10s——"开窗瞬间平台空 publishes 不把慢轮询带进黄金期"语义完整；:77 用 `queryClient.getQueryData(["state",account,sessionToken])` 读缓存避 TDZ，注释与实现一致。
- **App.tsx:147 账号迁移 effect 兜底管理态**：`!isCurrentAdminSession(loadSessions(), adminName, adminToken) → setInAdmin(false)`，撞名学生/吊销/登出后自动退出管理态；依赖含 adminToken，标记清除即触发。
- **代码注释内行号引用完整性**：Select.tsx:490 引"防抖 effect 内 618 行"——现防抖守卫位于 :684，**行号陈旧 94 行**（R71 剥离净删行所致，剥离本身未改逻辑）。属注释漂移，不影响行为，归入 OBSERVE-72-06。
- **文案/后端契约核对**：前端依赖的"已满员"（后端 scheduler.go:1764 `该课程已满员，退避至下一备选`）、"激活票据无效或已过期"（handler.go:200）、"选课成功"（refused_test.go:143）均在后端存在，零悬空引用。
- **countList/字段契约**：前端消费字段（publish_id/class_id/course_name/priority）与后端 TargetsRequest 一致；无平台私有字段泄漏进组件（begin_times 仅经 /electives 代理响应消费）。

### 新 OBSERVE（纯注释/遗漏项，零行为影响）

**OBSERVE-72-02 — Admin.tsx:207/:752 注释残留 N 系列标签（R71 补剥离遗漏）**

- 位置：web/src/routes/Admin.tsx:207 `{/* N3：删除账号二次确认 Dialog（替代 window.confirm，符合黑白极简设计） */}`、:752 `{/* N5：教务令牌有效性可视化——管理员一眼看到各账号 token 是否失效/恢复中 */}`。
- 一句话：R71 两个剥离提交（4a9dea4 / 175ed72）均未覆盖这两处 JSX 块注释，N3/N5 标签残留。
- 影响面：零行为影响，契约 20 收尾遗漏。
- 建议裁决：**删**——随下次前端卫生提交剥离 `N3：` / `N5：` 前缀即可（契约 20 要求轮次历史只存 CLAUDE.md，代码注释只留"为什么/契约/陷阱"）。

**OBSERVE-72-01 — 撤回修正记录**

- 本轮残留扫描阶段将 Select.tsx 的若干「平台」「官网」等业务词误记疑似标签，逐行复查后确认 Select.tsx 无残留（真实残留仅 Admin.tsx 两处与 scripts 三处，见 OBSERVE-72-02/03）。本条自撤回，不计入发现。

**OBSERVE-72-03 — TDD 脚本注释残留轮次标签（R71 只剥离 src/，scripts/ 未覆盖）**

- 位置：web/scripts/target-guard-check.ts:33 `F42-M1：`、:93 `F40-M1：`；web/scripts/unauthorized-check.ts:1 `F41-N2`。
- 一句话：R71 剥离范围仅 web/src，web/scripts 断言脚本注释残留 3 处标签；脚本注释本就在契约 20 覆盖范围（R71 提交 7e4ef3a 曾改过 scripts 注释口径）。
- 影响面：零行为影响（脚本断言 18/18 全绿）。
- 建议裁决：**删**——随下次前端卫生提交统一剥离。

**OBSERVE-72-04 — TDD 断言脚本与源实现第三参数默认值语义分叉**

- 位置：web/scripts/target-guard-check.ts 全部 `shouldDeferSave(...)` 断言为 2 参数调用（:43-44/:47-48/:60/:64/:72/:77），源实现 targetGuard.ts:64-71 为 3 参数签名（第三参 `echoed` 未设默认值）。
- 一句话：2 参数调用第三参为 `undefined`，`if (echoed) return false` 判 false 分支成立 → 等价于 `echoed=false`（未回显语义）；脚本断言与"首帧未到/未回显推迟"目标一致，但**类型层无第三参默认值，若未来第三参改为"未传=已回显"语义，脚本会静默全红而无人察觉**。
- 影响面：零当前行为缺陷（断言全绿即证明当前语义正确）；属可维护性观察——脚本未断言"已回显"稳态下的真实 3 参数调用（R63 M-1 的核心场景）事实上已由 :72/:77 两条 3 参数断言覆盖。
- 建议裁决：**续**——当前断言语义正确，建议后续给 `shouldDeferSave` 第三参补 `echoed = false` 默认值（语义显式化），或脚本统一 3 参数调用；可选项非缺陷。

**OBSERVE-72-05 — Select.tsx:569 清空守卫注释误引已删渲染期常量**

- 位置：web/src/routes/Select.tsx:569 `（publishes 恒空）不在此列——那是 F15/F16/F17 链的刻意安全方向，等无可等，绝不强行假清空`。
- 一句话：注释把"发布缺席 + 已有选中 = 假清空守卫"归因给 F15/F16/F17 链，但该守卫真实判据在 :507（flushTargets 消费时刻 `publishesRef.current.length===0 && latestSelectedCount>0`）；且 :471-473 注释已明确"渲染期常量已无引用，删除"——同机制两处注释指位错位，属 R71 剥离后清理链的注释残留（剥离历史标签时未同步对齐指位词）。
- 影响面：零行为影响。
- 建议裁决：**删/改**——:569 的 "F15/F16/F17 链" 指位应与 :471-473 同口径（指位"防抖/flush 消费时刻双闸"或直接指位 :507 守卫），随下次前端卫生提交修正。

**OBSERVE-72-06 — Select.tsx:490 注释内行号引用陈旧（净删行漂移 94 行）**

- 位置：web/src/routes/Select.tsx:490 `// 回显未完成守卫：与防抖回调同款判据（见防抖 effect 内 618 行）`。
- 一句话：现防抖守卫位于 :684，注释引用的 "618 行" 陈旧 94 行——R71 剥离净删注释行（非逻辑改动）导致行号漂移。
- 影响面：零行为影响；防抖 effect 内确有同名守卫（:684），指位仍可人工定位。
- 建议裁决：**续/改**——行号引用类注释天然随增删行漂移，建议改为语义指位（"见防抖回调内的回显未完成守卫"）去除行号，或随下次卫生提交更正为 :684。

---

## 可疑待核清单（证据不足 / 设计留白，未定级）

**可疑-1 — Select.tsx 清空守卫条件与"用户无 UI 全清空入口"的张力**

- 位置：flushTargets :489-490 / 防抖 :651 的 `latestRev===0 return` 等判据体系假设"用户可显式清空全部目标"，但当前 UI 无"一键清空"按钮——清空只能逐个取消（pick 移除）。
- 触发场景推演：用户逐课取消至 selected 空对象 → 防抖触发 `build()` 产出 [] → `next.length===0 && selectedCount>0` 守卫中 `selectedCount` 是**回调闭包捕获的渲染期旧值**（防抖 400ms 后执行），若在最后一次 pick 后 400ms 内无新渲染提交，`selectedCount` 可能仍 >0 → 本次保存被误判"假清空"置脏跳过 → 下次防抖才落库。影响：仅多等一次防抖（400ms），安全方向。
- 复核结论：`selectedCount` 由 `selected` 派生且 effect 依赖含 `selected`（:745），最后一次 pick 必然触发重渲染使 effect 重建、新版闭包 selectedCount 已归零 → **误判窗口不存在**。清空语义本身由 :254 `rev>0 && !anyHas` 回显不合并 + :547 放行路径完整保障。判定：**设计留白非缺陷，闭合**。

**可疑-2 — handleBack 三轮 flush 循环在"保存链持续失败"下最长等待时间**

- 位置：handleBack :607-638，三轮循环每轮最多等 21s（:631 deadline），最坏 ~63s 才 onDone。
- 触发场景推演：后端持续不可达时点返回，目标保存重试退避 2/4/8/16/16s，三轮循环各等满 21s → 返回按钮最多挂 63s。
- 复核结论：api 20s 超时兜底 + 失败退避 5 次停手（:414 `attempt >= 5`），用户预期"保存尽力而为"，63s 上限属契约内（注释"绝不无限挂起"成立）。移动端返回路径（底部操作栏? 无——仅 Dashboard 有，Select 返回按钮始终可见）无专项超时提示。判定：**设计留白，维持 O- 级观察，不升级**。

**可疑-3 — Dashboard 折叠段 aria-controls 指向 id 在"open=false"时无目标节点**

- 位置：Dashboard CollapseSection :82 `<div id={id}>` 只在 `open` 时渲染；`open=false` 时按钮的 aria-controls 指向不存在的节点。
- 触发场景推演：读屏用户折叠状态下触发 aria-controls 引用 → 目标节点缺失。
- 复核结论：WAI-ARIA 允许 aria-controls 指向"可能不存在的未来节点"（折叠内容展开前无 DOM 属正常模式），且 Dashboard 折叠段无独立交互陷阱。判定：**非缺陷，闭合**。

---

## 已核无缺陷清单

- 六防保存链（假清空守卫 / shouldDeferSave / cleanStaleSelected / 回显合并 / key={account} / Toast 定位）：逐字符零回归，TDD 18/18 全绿。
- M-1 三消费点 + echoedRef 置位三路径 + 首帧不置位边界：第八轮闭合。
- OBSERVE-66-03 setSelected 五调用点：无新增、无第三来源。
- 撞名学生管理态判定（adminToken 标记）：admin-auth 6/6 全绿，刷新恢复判据正确。
- 票据贯通（F11-A1）：Login 1001 分支存票 + 激活回传 + ApiError.data 透传，全链路完整。
- 401 单广播（F41-N2 + B45-N1）：HTTP 401 前置广播 + body 401 双 401 去重，三形态各单次。
- 登出吊销 / onDeleted / onUnauthorized 三路会话清理：快照式三连一致，无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布 / 倒计时兜底 / 空态 / aria：全部与注释承诺一致。
- 轮询降频（window_closed 30s / 失败 30s / 开窗 2s）：全站统一，TDZ 规避正确。
- Dashboard 折叠种子 / 日期分组 / 时间摘要：无重种、无时区偏移、无 stale。
- Admin 配置保存（refetch→epoch 回填）/ 激活码 Set 在飞跟踪 / 删除在飞幂等：无新缺陷。
- 无障碍（激活/退选/删除弹窗 Esc、role/aria、switch 语义）：全站完备。

## 构建验证

| 项 | 结果 |
|---|---|
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| TDD 脚本依赖（jiti） | ✅ node_modules 完整（注意：须在 web/ 目录下运行，仓库根运行会 ERR_MODULE_NOT_FOUND） |
| R71 剥离语义核 | ✅ 逐行 diff 通过，技术内容零损伤 |

> 本轮只读审查未运行 `npm run build` 与后端测试（R71 已全绿且本轮零代码改动；并行 backend 审查代理独立验证后端）。三组 TDD 断言脚本全部复跑 exit 0（target-guard 18/18、admin-auth 6/6、unauthorized 5/5，均须在 web/ 目录下运行）。

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第八轮闭合，见上。
- **OBSERVE-66-03**（亚帧 setSelected 五调用点）：无新增，维持「续」。
- **N-1~N-3**（三态冲刺文案 / pick 一拍调度延迟 / 格式卫生）：R68/R69 已把 N-3 落位，N-1/N-2 维持，本轮无新证据升级。
- **O-1~O-12**（Toast viewport / 倒计时 NaN / 手写 modal 焦点 / 管理多标签 / Dashboard key 不对称 / ui 模板残宽 / Button dark bg-black / 401 闭包 current / 状态机四态 / M-1 边角 / Dashboard nowMs 渲染期 Date.now + extras 1s 陈旧窗口）：逐条复查无升级证据，维持。
- **OBSERVE-70-01/02/03**（R70 三观察）：01/02 已随 R70 归一闭合；03（Select aria-label 与 placeholder 同串）维持「续」。
- **oxlint 12 条既有**：全为既有集合（R71 报告已列清单），本轮零新增，维持。

## 备注

- 只读铁律全程：未修改任何仓库文件（`git status` 工作树 web/ 与 scripts/ 洁净）。
- 复核与 CLAUDE.md 既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态载入、target-guard 判据完全对齐源实现）。
- 本轮报告故意收窄定级口径：R71 剥离逐行核证通过 + 全前端地毯排查，零 CRITICAL/MAJOR/MINOR；2 条新 OBSERVE（Admin N3/N5 残留、TDD 脚本标签残留）+ 3 条复核发现（TDD 第三参语义、:569 注释指位、:490 行号陈旧）均已给出裁决建议，全部为注释/维护性级别，无行为影响。
