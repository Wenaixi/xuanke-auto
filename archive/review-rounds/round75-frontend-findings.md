# Round 75 前端只读审查报告

基线：commit 79cd469（R74 双 findings + 收尾总结，R74 收官 HEAD）。本轮为 **R75 前端全模块只读审查**，核证对象为 R74 收尾后的全前端源码（web/src 全部 .ts/.tsx + web/scripts 断言脚本 + components/ui 全部）。**前端 R74 本轮零修改**（79cd469 仅改 archive/ 三份 docs，bb619a1 仅改 backend scheduler 注释与测试注释；git log 确认 web/src 自 9f5c217 后无任何提交）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + 3 组 TDD 断言脚本只读复跑 + npm run build 只读验证），`git status` 除已存在的 `round75-backend-findings.md`（r75-backend-reviewer 产物）外零改动，dist/ 为 .gitignore 忽略目录。

## 概述

**R74 变更后前端零回归：M-1 延续第十一轮闭合、六防保存链全链逐条核证无回潮、登录/激活链 + 撞名学生管理态 + 人性化细节全部与承诺一致；构建实测全绿（build exit 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）。本轮零 CRITICAL、零 MAJOR、零新增 MINOR，4 条新 OBSERVE（均提示/观察级零行为）+ 4 条既往 OBSERVE 延续 + 无实质可疑待核。连续第二十一轮零严重级发现，前端全地毯式排查未发现任何数据丢失/静默覆盖/安全漏洞/核心功能不可用。**

---

## M-1 延续管理（第十一轮）+ R74 变更回归检查

### 前端 R74 本轮零修改确认

- `git log --all --oneline -- web/src`：最近前端改动为 9f5c217（R73 收尾注释）、382b7a6（R72 收尾）、175ed72/4a9dea4（R71 剥离标签）——R74 两提交（bb619a1 / 79cd469）均零前端文件。
- `git show bb619a1 --stat`：仅 `backend/internal/scheduler/scheduler.go` + `scheduler_test.go` 两文件 8 行，前端零触及。
- **故本轮 M-1 延续管理的正确姿势 = 与 R74 基线逐字符比对 + 全链重走读，而非"找 R74 改了什么"**。以下为比对结果。

### M-1 第十一轮核对（与 R74/R73 基线逐字符比对）

- **三消费点全部传 `echoedRef.current` 第三参**（三参数签名 `shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)`）：
  - 防抖回调 Select.tsx:684 —— `selectedCount > 0`，:681-683 注释"已回显完成的稳态…不闷死 / 未回显仍置脏"完整。
  - flushTargets :500 —— `latestSelectedCount > 0`，:498-499 注释完整。
  - handleBack 判定 :593 + while :601 —— 两处均 `hasSelectedNow()`，:590-592/:599-600 注释完整。
- **echoedRef 置位三路径 + 首帧不置位边界**：courses 空分支 :238-239（置 true + echoDone 同步）、合并完成分支 :295-296、account reset :200（置 false）；首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable。**与 R74 逐字符一致，零回潮**。
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；data/rev/selected 随轮询重建引用但首行短路零开销。
- **TDD 复跑**：target-guard 18/18 全绿（含 M-1 稳态两条：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。
- **行号引用残留扫描**（`见 \d+ 行`/`第 \d+ 行`/`innerHTML`/`dangerouslySetInnerHTML`/`eval(` 多模式 grep）：全仓零命中——R72/R73 行号族修复无新残留。

### OBSERVE-66-03 setSelected 调用点清点（复跑）

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | setSelected({}) | account reset |
| :250 | setSelected(prev=>) 函数式 | 回显合并 |
| :288 | setSelected(prev=>cleanStale) 函数式 | 回显内清理 |
| :320 | setSelected(prev=>cleanStale) 函数式 | 独立清理 effect |
| :351/:361 | setSelected(对象式快照) | 用户 pick |

**与前轮完全一致、无新增、无第三来源**。

---

## 六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归复核

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据 + hasSelected 第二参 + echoed 第三参；`if (stateData===undefined) return true` / `if (echoed) return false` / `courses 非空 && hasSelected` 三分支与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :680-687（stateDataRef）+ effect 依赖 :745`[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]` | 判据只读 stateDataRef（非 echoedRef）→ /state 到达触发 effect 重跑自愈；依赖含 stateData/echoDone/hasPublishes 三路解锁，自愈链完整闭合 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用；脚本场景 F-J 全绿；effect 依赖含 selected（:310-311 注释：合并后重跑补清理） |
| F39-M1 消费时刻双闸 | 防抖 :695-698 / :704-714 / :720-723 / :726-729 + flushTargets :507-509 / :519-529 / :547-549 / :553-556 | 双闸四判据逐条重读：发布缺席+已有选中→置脏 / stale 残留→置脏+toast / 联查空已假清空→置脏 / 发布 id 漂移→置脏——全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并、`rev > 0 && !anyHas` 不合并、:287-294 回显内 stale 清理 + toast 兜底、:297 依赖完整性 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :720 / flush :547 全清空放行 + 回显 :254 全清空不合并 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT []；清空语义绝不复活 —— 已核六轮维持闭合 |
| key={account} | App.tsx:294（targetAccount 分支）/ :340（current 分支） | 双挂载点均 key 绑定账号；Select 内兜底守卫 :195-202（account 变化复位会话态）延续 |
| Toast 定位 | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` + 布局类在 viewport 上 | 无回归；同 title 去重合并（idRef 自增 + findIndex 更新 description 保留 duration）逻辑自洽 |

### 防抖 effect 细节复核（重点风险位）

- **resetRetry 放 effect 顶部**：:650-653 `if (rev === 0) return; resetRetry()` ——新改动立即清旧退避 timer，无不一致路径。
- **防抖回调闭包**：回调读 `selected`（:657/:704）为 effect 创建时快照，`publishesRef`/`stateDataRef` 消费时刻最新——selected 快照正确性依赖"effect 依赖含 selected"（:745），任一 pick 均重建 effect 冻结新闭包，无陈旧分叉；回调读 `selectedCount`（:684/:697/:722）闭包快照由 selected 依赖兜底（OBSERVE-71-01 同源）。
- **saveNow 去重/串行化**：:426-428 `json === lastJson.current` 跳过重复；:434 卸载后成功不落 lastJson；:450-457 finally `dirtyRef && attempt===0` 补发最新快照。两处 `if (unmountedRef.current) return`（:423/:434/:439/:451）拦截卸载后一切交互，无孤儿请求。
- **handleBack 保存链静止等待**：:607-638 三轮循环，`:618 if (!dirtyRef.current && !savingRef.current)` 等一帧复查 revRef 一致才 break；`:629-636 pendingSaving()` 覆盖退避 timer 排队（2/4/8/16/16s）在飞；21s 兜底绝不无限挂起（OBSERVE-75-01 见下，新增观察项）。

---

## 登录/激活链 + 撞名学生管理态 + 人性化细节核（复跑）

### 登录/激活链（Login.tsx + client.ts + App.tsx）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 登录幂等守卫 | Login.tsx:36 `if (loading) return` | 连按两次 Enter/快速双击只发一请求 |
| 票据贯通 | :54 `setPendingTicket((e.data?.ticket as string) || "")` + :78 激活请求 body 带 `ticket` | 1001 分支保存 data.ticket 随激活回传；取消激活清票；"过期/已用"与"激活码错误"两分支同款清票、文案引导差异化——完整 |
| 激活幂等守卫 | :68 `if (activating) return` | 与 submit 对称 |
| 401 单广播 | client.ts:64 HTTP 状态码前置广播 + :85 body 层 `r.status !== 401` 兜底 | HTTP 401 单次、HTTP 200+body 401 单次、双形态互斥不重复 |
| 20s 超时兜底 | client.ts:56 `setTimeout(() => ctrl.abort(), 20000)` + :103 AbortError→"请求超时，请重试" | 与注释"20 秒"一致；signal 显式接入调用方优先 |
| 登出吊销 | App.tsx:112-132 | apiLogout 作废服务端令牌 + 快照式三连 + 清 adminToken/targetAccount/page 全路径 |
| onDeleted | App.tsx:160-178 | 快照式三连 + 清 targetAccount/current + 删管理员自身退管理态+清标记 |
| onUnauthorized | App.tsx:187-235 | 按令牌反查归属、管理代理态不误杀（:217 `isCurrentAdminSession && inAdmin && lostAccount !== adminName` 提前 return）、删管理自身退管理态+清标记、快照式落盘——全链路完整 |

### 撞名学生管理态（adminAuth.ts + App.tsx 渲染判据）

- `isCurrentAdminSession` 纯函数：`adminToken !== "" && sessions[adminName] === adminToken`；渲染判据 :299 `inAdmin || isCurrentAdminSession(sessions, adminName, adminToken)` 与账号迁移 effect :147 同源；admin-auth-check 6/6 全绿。**第十一轮零回潮**。
- 管理令牌持久化 `xk_admin_token` 全路径清除点（logout :121-122 / onDeleted :175-176 / onUnauthorized :213-214 / onBackToStudent :312-313）四处全覆盖，无泄漏路径。

### 人性化细节核（复跑 + 本轮新增关注）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 倒计时 begin_times 兜底 | Select :757-762 / Dashboard :190-195 | 识别缺席（open_time_known=false）用 `begin_times[0]` 兜底，识别槽建立后以识别真值为准——CLAUDE.md 契约 26 承诺一致 |
| 倒计时横幅文案 | Select :817-843（五态分支） | `!stateData`→同步中 / window_closed→已关闭 / window_opened→已开放 / 双缺席→未识别到开放时间 / isExpired→本地已到点等待 / 正常倒数；`|| cd.isExpired` 已清除，无"关闭后恒显已开放"矛盾 |
| 倒计时自校正 | useTickingCountdown :16-18 `setNow(Date.now())` | target 变化（首次同步/开窗瞬间）立即回正，消除 1s 陈旧偏差 |
| btn_type 三向 | Select :1106/:1119 `===1` 退选 / `===2` 报名 / 其他不渲染 | 与官网逆向契约一致；can_select 双守卫 disabled+title；窗口未开照样可点报名（title 提示"不在选修报名时间范围内"）不锁死手动通道 |
| max_count=0 名额未公布 | :957 筛选不过滤 / :996 isFull `max_count>0 &&` / :999-1000 unannounced / :1082-1084 文案 / :1091-1095 Progress value=0 max=1 | 四处同源 `max_count>0` 判据；未公布课程不误显"已满额/余0席/满条"，进度条空条如实 |
| 空态 | Select :906-912 无可选批次 / Dashboard :535-545 无预选课程 | 均有明确文案 + 行动引导；顶栏徽章分母 `publishes.length > 0 ? '/'+len : ''`（:786-787）杜绝 "/0" |
| aria 无障碍 | 退选弹窗 role=dialog+aria-labelledby+Esc（:1191-1201）/ 删除弹窗同款（Admin :210-218）/ 激活弹窗 + Esc（Login :223-238）/ switch role+aria-checked（Admin :568-572）/ progressbar（Progress :23-28）/ CollapseSection useId（Dashboard :64-67）/ 搜索 aria-label | 全站完备；useId 消除重复 id |
| 在飞幂等全站点 | 登录/激活/报名/退选/删账号/删激活码/生成码/保存配置 | Set 独立跟踪无互踩，disabled 渲染延迟前入口短路齐全 |

---

## 发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。（延续项 OBSERVE-73-01 Admin useState 措辞已由 R73 修复归零；以下均为 OBSERVE 级。）

### OBSERVE（4 条新增 + 延续项）

**OBSERVE-75-01 — handleBack 保存静默等待期零进度反馈（人性化建议项）**

- 位置：Select.tsx:571-640（handleBack 全链 + :607-638 三轮收敛循环）。
- 触发场景推演：用户点"返回控制台"时若在飞 PUT 未完成（savingRef true）或退避重试 timer 排队中（pendingSaving），按钮挂起等待最多 3×21s=63s，期间界面无任何"正在保存目标..."文案或 loading 指示——用户可能误以为按钮卡死而重复点击（重复点击再次触发 handleBack 进入新一轮等待），或直接刷新页面丢失内存改动。
- 复核结论：完整业务链（先 flush → 等回显 → 等保存链静止 → 收敛）正确，63s 上限非挂起（api 20s 超时飞快失败）且绝不假清空；属 R73 可疑-1"可选增强（非缺陷）：等待期间可加轻量'正在保存目标...'文案降低焦虑"的延续——本轮复跑确认该增强仍未实现。**建议裁决：续**（人性化增强候选，非缺陷）。

**OBSERVE-75-02 — Dashboard extrasMs 随 /state 轮询重建引用**

- 位置：Dashboard.tsx:208-211 useMemo + :426-454 折叠段。
- 触发场景推演：`primaryMs` 依赖 `openTimeStr`（来自 /state 每 3s 刷新），openTimeStr 变化 → `extrasMs` useMemo 重算返回新数组引用（即便元素相同）→ CollapseSection 每次 /state 刷新重渲染。开销可忽略（折叠段内最多几条时间文本），无功能影响。
- 复核结论：useMemo 依赖 `[electives?.begin_times, primaryMs]` 引用/标量语义正确；openTimeStr 通常稳定（识别后同一值），实际重算频率≈0。**建议裁决：续**（纯观察）。

**OBSERVE-75-03 — Admin CodesTab uses 输入无前端上限（后端 1000 兜底）**

- 位置：Admin.tsx:385-392 `setUses(Math.max(1, Number(e.target.value) || 1))`（仅下限钳制）。
- 触发场景推演：管理员输入 "5000" 可提交（前端放行），后端 handleAdminCodes :645 `req.Uses > 1000` 拒绝报"每个激活码使用次数上限为 1000"，前端弹失败 toast——功能正确但 UX 有一步返工。
- 复核结论：`count` 输入已限 1-100（:380 `Math.min(100, ...)`）而 `uses` 未对称限流；后端值域校验完备、数据安全零风险。**建议裁决：续**（可选对称加 `Math.max(1, Math.min(1000, ...))`，纯 UX 优化）。

**OBSERVE-75-04 — Select 空态卡与 window_closed 信号弱相关（延续 R73-03）**

- 位置：Select.tsx:906-912 空态文案"当前无可选课程批次（选课窗口未开放或已关闭）"不读取 `stateData.window_closed`（:819 横幅读之）。
- 触发场景推演：窗口已关但 /state 首帧未到（stateData undefined）时横幅显"正在同步选课开放时间..."、空态与横幅并存秒级；/state 到达即横幅转"选课窗口已关闭"，空态文案的"或已关闭"二选一句式已涵盖状态，自愈。
- 复核结论：与 R73 判据一致，无功能缺陷。**建议裁决：续**（低于升级阈值）。

**OBSERVE-71-01（延续）**：防抖 effect 缺失 selectedCount 依赖——回调读 selectedCount 实为闭包快照、由 selected 依赖兜底，维持「续」。

**OBSERVE-71-02（延续）**：Dashboard F10-06 动态目标集合相关注释并存，维持「续」。

**OBSERVE-70-03（延续）**：Select 搜索框 aria-label 与 placeholder 同串，维持「续」。

**OBSERVE-66-03（延续）**：setSelected 五调用点无新增，维持「续」。

### 可疑待核

**可疑-1 — Select :429 目标保存 PUT 的 course_name 不携带发布元数据**

- 位置：Select.tsx:530-541 targets 构建（无 publish_name/begin_date 字段）。
- 触发场景推演：前端 PUT /api/targets 的 body 只含 `{publish_id, class_id, course_name, priority}`，后端 handleSetTargets → Sched.SetTargetsForAccount 内部 `enrichTargetPubMetaLocked`（scheduler.go:543）从专属帧/全校帧补齐 publish_name/begin_date——补齐有数据源时成功；帧缺失（HTTP 直存时专属帧常过期）则落库目标缺元数据，但 `restoreTargets`（:535）启动恢复时再次兜底全校帧 lastData 补全，`/state.courses` 分组近乎总可读。
- 复核结论：后端双段补全 + 前端消费时刻 `targetsUseCurrentPublishes` 放行语义自洽；HTTP 直存缺帧场景由启动恢复兜底（"窗口重开后重存目标自愈"注释），非数据丢失。**闭合**（与后端 enrichTargetPubMetaLocked 交叉核证）。

---

## 已核无缺陷清单

- 前端 R74 零修改确认（git log 谱系核证），M-1 第十一轮三消费点逐字符比对零回潮。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + F15/F16/F17 消费时刻双闸）：逐条判据与注释逐一对应，渲染期常量清退完成，自愈链（stateData/echoDone/hasPublishes 三路解锁）完整闭合。
- 撞名学生管理态判定：admin-auth 6/6 全绿，刷新恢复判据正确，令牌标记全路径清（四清除点）。
- 票据贯通 / 401 单广播 / 20s 超时兜底 / 登出吊销 / onDeleted / onUnauthorized：全链路完整，快照式三连一致无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布四处同源 / 倒计时兜底 / 空态 / aria 全站完备。
- 轮询降频全站统一（window_closed/failure 30s、黄金期 2s、其余 10s、Dashboard 3s/30s），TDZ 规避正确（refetchInterval 经 queryClient.getQueryData 读 /state 缓存）。
- 后端交叉核证族：handleElectiveSelect 凭据表校验 + CheckClassSelectable 窗口/满员复核 + TryAcquireSubmit 在飞互斥 / SelectClass doRequest code=-1→ErrUnauthorized / MarkDone 客户端存在性复核 + 删 refused 行 + success 落库 / RemoveDone 落 refused 行 + 清 success（重启恢复契约复核成立）；submitAll 删号跳过 + spawnChain 链顶/取 client 双复核 + 六分支指针身份比对（sameClientFor）全谱系存在；maybeRelogin 决策侧存在性复核;WindowClosed 三判据单源 windowClosedLocked + StateForAccount 同源;handleSetTargets 数量/范围/发布 id 校验 + 凭据表透传校验；handleAdminDeleteAccount memory-first 四段防线 + TrimSpace 拒绝。
- 前端 PUT 请求体与后端 TargetsRequest 解码交叉核证（class_id/publish_id/priority 边界 + maxTargetsPerAccount=100）一致。
- XSS/敏感数据/硬编码开放时间：零命中（无 innerHTML/dangerouslySetInnerHTML/eval；token 只存 localStorage + Bearer 头发送；开放时间唯一事实源注释清晰）。
- localStorage 读写全 try/catch 降级（App.tsx load/save 六处），隐私模式静默降级不崩。

## 构建验证

| 项 | 结果 |
|---|---|
| `npm run build`（web/ 下） | ✅ exit 0（tsc -b + vite，1948 modules，dist/ 与 .gitignore 忽略，git status 洁净） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| 全仓残留扫描（`见 \d+ 行`/`innerHTML`/`eval(`/`dangerouslySetInnerHTML`） | ✅ 零命中 |

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第十一轮闭合，见上。
- **OBSERVE-66-03**（亚帧 setSelected 五调用点）：无新增，维持「续」。
- **OBSERVE-70-03 / 71-01 / 71-02 / 73-01~04 / 74-01~04**：本轮复跑无升级证据，维持既往。
- **OBSERVE-75-01**（handleBack 等待期无进度反馈）：R73 可疑-1 可选增强延续，本轮记录为独立观察项。

## 备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + 3 组 TDD 脚本只读复跑 + npm run build 只读验证）；未修改任何仓库文件（唯一报告文件为本文件，属于 team-lead 明确指定的输出路径）。
- 复核与既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态、target-guard 判据、契约 20 剥离完成度、删账号 memory-first、指针身份比对族）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；4 条新 OBSERVE（均提示级零行为）+ 4 条 fÃ¼r既往 OBSERVE 延续 + 1 条可疑待核（交叉核证后闭合）；连续第二十一轮无严重级发现。