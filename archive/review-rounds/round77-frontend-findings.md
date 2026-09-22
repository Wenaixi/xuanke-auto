# Round 77 前端只读审查报告

基线：commit 7b939d6（R76 双 findings + 收尾总结，R76 收官 HEAD）。本轮为 **R77 前端全模块只读审查**，审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts）+ web/scripts 三组 TDD 断言脚本。**前端 R76 后零修改**（git log 确认 web/src 最近提交为 9f5c217，R76/R77 两轮仅改 archive/ 与 backend tray 图标；`git diff HEAD --stat -- web/src` 空）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + tsc -b + 三组断言脚本只读复跑），工作区洁净零改动。

## 概述

**前端 R76 后零修改，全链重走读确认 M-1 延续第十三轮闭合、六防保存链零回潮、登录/激活链 + 撞名学生管理态 + 人性化细节全部与承诺一致；构建与三组断言全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5）。本轮零 CRITICAL、零 MAJOR、2 条 MINOR + 2 条新 OBSERVE + 1 条可疑待核（理论竞态实测不可达）。连续第二十三轮无严重级发现。本轮重点抓"功能障碍/逻辑错误/细节不够人性化"：发现 2 条真实 MINOR（管理端失败态吞并、保存链退避停手后离开无终局提示），均为"极端失败场景下 UX 误导/改动无感知"，非数据永久丢失。**

---

## M-1 延续管理（第十三轮）——前端零修改确认

- `git log --oneline -8 -- web/src`：最近前端提交为 9f5c217（R73 收尾）；R76/77（40a8bf2/7b939d6）仅 backend tray + archive/ 三份 docs，前端零触及。`git diff HEAD --stat -- web/src` 空。
- 故本轮 M-1 复核 = 与 R76 基线逐字符比对 + 全链重走读。

### M-1 第十三轮核对（逐字符比对 + 行为走读）

- **shouldDeferSave 三消费点全部传 `echoedRef.current` 第三参**（三参数签名 `shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)`）：
  - 防抖回调 Select.tsx:684 —— `selectedCount > 0`。
  - flushTargets :500 —— `latestSelectedCount > 0`。
  - handleBack 判定 :593 + while :601 —— 两处均消费时刻 `hasSelectedNow()`（:575-576 读 selectedRef 计算）。
  - 与 R76 基线逐字符一致，零回潮。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - courses 空分支 :238-239（置 true + echoDone 同步）。
  - 合并完成分支 :295-296（置 true + echoDone 同步 + :287-294 回显内 stale 清理 toast 兜底）。
  - account reset :200（置 false + setSelected({}) + rev 清零 + echoDone false）。
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位。
  - 独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable 延续。
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；data/rev/selected 随轮询重建引用但首行短路零开销。
- **TDD 复跑**：target-guard 18/18 全绿（含 M-1 稳态两条：courses 非空+有选中+已回显→放行 / 首帧未到+已回显→仍推迟）。
- **行号引用残留扫描**（`见 \d+ 行`/`第 \d+ 行`/`innerHTML`/`dangerouslySetInnerHTML`/`eval(` 多模式 grep）：web/src 全仓零命中。

### OBSERVE-66-03 setSelected 调用点清点（复跑）

| 行号 | 形式 | 归属 |
|---|---|---|
| :198 | setSelected({}) | account reset |
| :250 | setSelected(prev=>) 函数式 | 回显合并 |
| :288 | setSelected(prev=>cleanStale) 函数式 | 回显内清理 |
| :320 | setSelected(prev=>cleanStale) 函数式 | 独立清理 effect |
| :351/:361 | setSelected(对象式快照) | 用户 pick |

**与前轮完全一致、无新增、无第三来源。**

---

## 六防保存链（F43/F42/F40/F39/F36/F48-M1）零回归复核

| 防线 | 位置 | 复核结果 |
|---|---|---|
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true`/`echoed→false`/`courses 非空 && hasSelected`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :680-687 防抖回调 stateDataRef + effect 依赖 :745 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]` | 判据只读 stateDataRef（非 echoedRef）；/state 到达触发 effect 重跑自愈；stateData/echoDone/hasPublishes 三路解锁闭合 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用（:42）；effect 依赖含 selected，合并后重跑清理；脚本场景 F-J 全绿 |
| F39-M1 消费时刻双闸 | 防抖 :695-698 / :704-714 / :720-723 / :726-729 + flushTargets :507-509 / :519-529 / :547-549 / :553-556 | 四判据逐条重读：发布缺席+已有选中→置脏 / stale 残留→置脏+toast / 联查空集+已有选中→置脏 / 发布 id 漂移→置脏——全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并（已触碰发布保留用户现状含空数组、未触碰补旧目标）、`:254 rev > 0 && !anyHas` 不合并；:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :720 / flush :547 全清空放行 + 回显 :254 全清空不合并 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT []；清空语义绝不复活 |
| key={account} | App.tsx:294（targetAccount 分支）/ :340（current 分支） | 双挂载点 key 绑定账号；Select 内兜底守卫 :195-202 延续，TDZ 注释（:193-194）正确 |
| Toast 定位 | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` + 布局类在 viewport 上 | 无回归；同 title 去重合并（idRef 自增 + findIndex 更新 description 保留 duration）逻辑自洽 |

### 防抖 effect 细节复核（重点风险位重走读）

- **resetRetry 放 effect 顶部**：:650-653 `if (rev === 0) return; resetRetry()` —— 新改动立即清旧退避 timer。
- **setTimeout async 回调闭包**：回调读 `selected`（:657/:704）为 effect 创建时快照，`publishesRef`/`stateDataRef` 消费时刻最新；selected 快照正确性由"effect 依赖含 selected"（:745）兜底。
- **saveNow 去重/串行化**：:426-428 `json === lastJson.current` 跳过重复；:434 卸载后成功不落 lastJson；:437-448 catch 置 dirty + scheduleRetry（attempt≥5 停）；:449-457 finally `savingRef=false` + `dirtyRef && attempt===0` 补发最新快照；四门卸载保护（:423/:434/:439/:451）拦截卸载后一切交互，无孤儿请求。
- **handleBack 保存链静止等待**：:607-638 三轮循环；`:618 if (!dirtyRef.current && !savingRef.current)` 等一帧复查 revRef 一致才 break；`:629-636 pendingSaving()`（savingRef || retryState.timer）覆盖退避 timer 排队；21s 兜底绝不无限挂起。**本轮新关注：`if (!dirtyRef.current && !savingRef.current)` 里 dirtyRef 为"守卫拦下的保留脏"与"保存失败置脏"共用语义，onDone 前不区分二者——见 MINOR-77-02。**

---

## 登录/激活链 + 撞名学生管理态 + 人性化细节核（复跑）

### 登录/激活链

| 项 | 位置 | 核证结论 |
|---|---|---|
| 登录幂等守卫 | Login.tsx:36 `if (loading) return` | 连按两次 Enter/快速双击只发一请求 |
| 票据贯通 | :54 `setPendingTicket((e.data?.ticket as string) || "")` + :78 激活 body 带 `ticket` | 1001 分支保存 data.ticket（ApiError.data 透传 client.ts:30-33）随激活回传；取消清票；"过期/已用"与"激活码错误"两分支同款清票、文案差异化引导——完整 |
| 激活幂等守卫 | :68 `if (activating) return` | 与 submit 对称 |
| 401 单广播 | client.ts:64 HTTP 状态码前置广播 + :85 body 层 `r.status !== 401` 兜底 | HTTP 401 单次、HTTP 200+body 401 单次、双形态互斥不重复；HTTP 非 JSON 体先广播再抛 -2 |
| 20s 超时兜底 | client.ts:56 `setTimeout(() => ctrl.abort(), 20000)` + :103 AbortError→"请求超时，请重试" | 与注释"20 秒"一致；signal 显式接入（:58 `rest.signal ?? ctrl.signal`）调用方优先 |
| 登出吊销 | App.tsx:112-132 | apiLogout 作废服务端令牌 + 快照式三连 + 清 adminToken/targetAccount/page 全路径 |
| onDeleted | App.tsx:160-178 | 快照式三连 + 清 targetAccount/current + 删管理员自身退管理态+清标记（:172-177） |
| onUnauthorized | App.tsx:187-235 | 按令牌反查归属（:193-197）、管理代理态不误杀（:217 提前 return）、删管理自身退管理态+清标记（:210-215）、快照式落盘（:225-230）——全链路完整 |

### 撞名学生管理态

- `isCurrentAdminSession` 纯函数：`adminToken !== "" && sessions[adminName] === adminToken`；渲染判据 :299 与账号迁移 effect :147 同源；admin-auth 6/6 全绿。**第十三轮零回潮**。
- 管理令牌持久化 `xk_admin_token` 全路径清除点：logout :121-122 / onDeleted :175-176 / onUnauthorized :213-214 / onBackToStudent（App :312-313）四处全覆盖，无泄漏路径。

### 人性化细节核

| 项 | 位置 | 核证结论 |
|---|---|---|
| 倒计时 begin_times 兜底 | Select :757-762 / Dashboard :190-195 | 识别缺席（open_time_known=false）用 `begin_times[0]` 兜底，识别槽建立后以识别真值为准 |
| 倒计时横幅五态 | Select :817-843 | `!stateData`→同步中 / window_closed→已关闭 / window_opened→已开放 / 双缺席→未识别到开放时间 / isExpired→本地已到点等待 / 正常倒数；`|| cd.isExpired` 已清除 |
| 倒计时自校正 | useTickingCountdown :16-18 | target 变化立即回正，消除 1s 陈旧偏差 |
| btn_type 三向 | Select :1106 `===1` 退选 / :1119 `===2` 报名 / 其他不渲染 | 与官网逆向契约一致；can_select 双守卫 disabled+title |
| max_count=0 名额未公布 | :957 筛选不过滤 / :996 isFull / :999-1000 unannounced / :1082-1084 文案 / :1091-1095 Progress value=0 max=1 | 四处同源 `max_count>0` 判据；不误显"已满额/余0席/满条" |
| 空态 | Select :906-912 无可选批次 / Dashboard :535-545 无预选课程 | 均有明确文案 + 行动引导；顶栏徽章分母 `publishes.length > 0 ? '/'+len : ''` 杜绝 "/0" |
| aria 无障碍 | 退选弹窗 role=dialog+Esc+autoFocus（Select :1191-1224）/ 删除弹窗同款（Admin :210-243）/ 激活弹窗+Esc（Login :223-238）/ switch role+aria-checked（Admin :568-572）/ progressbar（Progress :23-28）/ CollapseSection useId（Dashboard :64-67） | 全站完备 |
| 在飞幂等全站点 | 登录/激活/报名/退选/删账号/删激活码/生成码/保存配置 | Set 独立跟踪无互踩，disabled 渲染延迟前入口短路齐全 |

---

## 发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR（2 条）

**MINOR-77-01 — Admin 管理端五 Tab 首次加载失败态被吞并成业务空态/永"加载中"，管理员无法区分"真无数据"与"请求失败"**

- 位置：Admin.tsx CodesTab :445-476（`isLoading ? 加载中 : data && data.length>0 ? 列表 : 暂无激活码`）、AccountsTab :801-868（失败→"暂无账号"）、LogsTab :892-906（失败→"暂无日志"）、StatsTab :742-744（失败→"加载中..."永转）、ConfigTab :680-684（失败→表单空白 + "配置加载中"提示）。
- 触发场景推演：管理端打开（或切回）任意 Tab，恰逢网络抖动 / 后端重启 / 反向代理超时 → react-query `retry:1`（App.tsx:13）一次重试也失败后 → `data` 为 undefined。CodesTab/AccountsTab/LogsTab 均走"空数据"分支渲染业务空态：激活码列表显示"暂无激活码，生成后即可分发"（误导管理员以为已生成的码全没了）、账号表显示"暂无账号"（误导 wait 删除或误删保护）、日志显示"暂无日志"；StatsTab 永"加载中..."；ConfigTab 表单空白（保存按钮已 disabled，安全但无解释）。Codes 有手动"刷新"按钮兜底，其余四 Tab 无任何重试入口，只能切 Tab/刷新页面碰运气。学生端 Select.tsx:898-902 有 `isError` 错误条，管理端全缺失——失败态与空态混为一谈。
- 修复建议（供后续修复轮参考）：各 Tab 用 `isError` 分支渲染明确错误文案 + "重试"按钮（`refetch()`），空态分支加 `!isError` 前提；或统一抽一个 `QueryGuard` 组件。属人性化/可操作性缺陷，无数据风险。

**MINOR-77-02 — 目标保存退避 5 次停手后离开页面，改动静默丢失且无终局提示（返回 Dashboard 后才暴露）**

- 位置：Select.tsx:412-421（scheduleRetry `attempt >= 5 停止`）+ :437-448（catch toast + scheduleRetry）+ handleBack :607-638（收敛循环 `if (!dirtyRef.current && !savingRef.current)` 用 dirty 判"静止"）+ :639 `onDone()`。
- 触发场景推演：网络持续异常（后端重启/代理断连）期间用户改了目标 → 防抖 PUT 失败 → `scheduleRetry` 指数退避 2/4/8/16/16s（约 46s）连续 5 次全失败 → `attempt>=5` 停手、不再自动重发（设计如此，"等下一次用户改动接管"）。此时点"返回控制台"：flushTargets 再发一次 PUT（仍失败 → scheduleRetry attempt 已满仍停），`dirtyRef.current=true` 使保存链"非静止"，handleBack 等待一轮后超时继续 → `onDone()` 卸载。**用户返回 Dashboard 时界面无任何"目标保存失败、改动未落库"的终局提示**（失败期间的 toast 是同 title 合并的红条，已过期；handleBack 全程无文案）。下次进入 Select 回显后端的旧目标，用户以为已保存的改动静默消失。
- 复核结论：改动是"临时丢失"（内存目标随卸载丢弃，后端仍是旧目标；网络恢复后用户重新选一次即可）。非不可恢复数据永久丢失，属"尽力而为后的静默无感知"。安全方向正确（绝不假清空），但人性化闭环缺一环。
- 修复建议（供后续修复轮参考）：handleBack 在收敛循环结束后、`onDone()` 前检查 `dirtyRef.current`（且非"守卫假清空脏块"则排除，可用最终一次 flush 的 put 结果 ref 区分），为 true 时弹一次明确 toast"目标保存失败，改动未落库"（重复文案合并机制防轰炸）。dirty 双语义（守卫拦下的假清空脏 vs 保存失败脏）需拆分或加区分标记。

### OBSERVE（2 条新增 + 延续项）

**OBSERVE-77-01 — Dashboard 与 Select 同 queryKey 但 URL 不同，跨路由切换首帧命中旧 URL 缓存数据**

- 位置：Dashboard.tsx:133 `/state`（无 account 参数）vs Select.tsx:147 `/state?account=`，但两处 queryKey 均为 `["state", account, sessionToken]`。
- 触发场景推演：普通学生在 Dashboard（queryKey 缓存 `/state` 响应）→ 点"选课大厅"→ Select 同 key 不同 URL（`/state?account=`）首次渲染**读取 Dashboard 缓存的第一帧数据**（staleTime 0 立即 refetch 覆盖，第二次渲染即正确）。普通学生场景后端 handleState 忽略非管理员 account 参数（allowAccountOverride=false，acct=sessionAccount），两 URL 语义等价 → 零功能影响。
- 复核结论：仅"缓存身份（queryKey）与请求 URL 解耦"的脆弱设计观察：未来若 /state 按参数返回不同数据（管理员自定义筛选等）会被缓存串味。管理员代理路径（App targetAccount 分支）不经过 Dashboard，实际无触发路径。**建议裁决：续**（低于升级阈值）。

**OBSERVE-77-02 — 窗口关闭期间 Select 无目标管理入口，目标在关闭期间不可变更（设计行为）**

- 位置：Select.tsx:906-912 空态卡（publishes 恒空时无任何课程卡）+ :316-327 独立清理 effect 首行 `if (!echoedRef.current || publishes.length === 0) return` + flushTargets :507 发布缺席守卫。
- 触发场景推演：选课窗口关闭（publishes 恒空）后用户回到 Select：空态卡"当前无可选课程批次（选课窗口未开放或已关闭）"无操作按钮，Dashboard 仍有旧目标卡片展示（契约 3：关闭≠时间消失）。用户无法在关闭期间删除/调整目标——目标保持冻结直至窗口重开。
- 复核结论：与契约 3（关闭≠时间消失、目标持久化）一致，cleanStale 在 publishes 空时不清理亦为"防假清空"的刻意守卫，无数据风险；"关闭期间目标不可管理"是设计边界而非缺陷。**建议裁决：续**（纯观察）。

**OBSERVE-76-01（延续）**：handleBack 保存静默等待期（最多 3×21s=63s）零进度反馈，维持「续」。

**OBSERVE-76-02（延续）**：Admin CodesTab `uses` 输入无前端上限（后端 1000 兜底），维持「续」。

**OBSERVE-76-03（延续）**：Toast 关闭按钮（Toast.tsx:100-102）无 aria-label/title，读屏朗读默认 "Close"，维持「续」。

**OBSERVE-75-02（延续）**：Dashboard extrasMs 随 /state 轮询重建引用，维持「续」。

**OBSERVE-75-04（延续）**：Select 空态卡与 window_closed 信号弱相关，维持「续」。

**OBSERVE-71-01 / 71-02 / 70-03 / 66-03（延续）**：防抖 effect selectedCount 闭包快照由 selected 依赖兜底 / Dashboard F10-06 注释并存 / Select 搜索框 aria-label 与 placeholder 同串 / setSelected 五调用点无新增——均维持「续」。

### 可疑待核

**可疑-1 — 回显 effect 置位 echoedRef 早于合并 setState 渲染提交：毫秒窗口内用户点击合并前快照 → 防抖 PUT 覆盖后端旧目标（理论竞态）**

- 位置：Select.tsx:295-296（回显 effect 同步体内置 `echoedRef.current = true` + `setEchoDone(true)`，而 :250 的 `setSelected(合并)` 是异步调度，渲染提交在本 effect 之后）与防抖消费时刻 :684 `shouldDeferSave(..., echoedRef.current)`（echo 为 true 即放行）。
- 触发场景推演：理论上若用户在"回显 effect 同步执行"到"合并 setState 提交的下一次渲染"之间的亚毫秒窗口内 pick 课程，pick 用**合并前** selected（空/旧快照）构建目标；400ms 防抖后 echoedRef 早已为 true → 放行 → build 只用用户新点课程 → 整包 PUT 覆盖后端旧目标。
- 复核结论：物理不可达——useEffect 在 paint 之后运行，effect 同步体内置位与 setSelected 调度在**同一帧内同步完成**，用户下一次事件（click）必然发生在合并渲染提交之后（浏览器事件循环分帧），不存在用户可观察的中间渲染；且 400ms 防抖窗口远大于该间隙。仅作理论梳理留存，**建议裁决：无需修复，作为稳定性契约注释候选**（后续若真 async 化 effect 则必须同步置位与 setSelected 的原子性）。

---

## 已核无缺陷清单

- 前端 R76 后零修改确认（git log 谱系核证），M-1 第十三轮三消费点逐字符比对零回潮、echoedRef 置位三路径 + 首帧不置位边界完整。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + F15/F16/F17 消费时刻双闸）：逐条判据与注释逐一对应，自愈链（stateData/echoDone/hasPublishes 三路解锁）完整闭合，handleBack 收敛循环无挂起路径。
- 撞名学生管理态判定：admin-auth 6/6 全绿，刷新恢复判据正确，令牌标记四清除点全覆盖。
- 票据贯通 / 401 单广播（HTTP 401 前置 + body 401 兜底互斥） / 20s 超时兜底 / 登出吊销 / onDeleted / onUnauthorized：全链路完整，快照式三连一致无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布四处同源 / 倒计时兜底 / 空态 / aria 全站完备。
- 轮询降频全站统一（window_closed/failure 30s、黄金期 2s、其余 10s、Dashboard 3s/30s），refetchInterval TDZ 规避正确（Select :77 经 queryClient.getQueryData 读 /state 缓存）。
- 后端交叉核证族：handleLogin 管理员双条件 + 撞名放行教务登录 / 1001 票据签发；handleElectives/ElectiveSelect/Exit/State 凭据表 accountExists 四路；spawnChain 链顶 plus 取 client 双复核 + 六分支（成功/失效/风控/窗口关闭/确证满员/实时复核）sameClientFor 指针身份比对全谱系存在；maybeRelogin 写回侧 ClientFor 复核 + 重登失败退避指数（backoffMin=30s 封顶 10m）与前端 reloginAt 语义一致；WindowClosed 三判据单源 windowClosedLocked 与 StateForAccount 同真相；markFullLocked 每课一次 + full 保留，releaseFullIfFreedLocked 空快照不解封（窗口关闭防轰炸）；SetTargetsForAccount 清 refused 不清 done/full/inflight 契约对齐前端"重设目标"语义。
- XSS/敏感数据/硬编码开放时间：零命中（无 innerHTML/dangerouslySetInnerHTML/eval；token 只存 localStorage + Bearer 头发送；开放时间唯一事实源注释清晰）。CSS 无危险 URL 注入。
- localStorage 读写全 try/catch 降级（App.tsx load/save 八处含 adminName/adminToken），隐私模式静默降级不崩。

## 构建验证

| 项 | 结果 |
|---|---|
| `npx tsc -b`（web/ 下） | ✅ EXIT 0（类型全通过，无 TDZ 误触） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| 全仓残留扫描（`见 \d+ 行`/`innerHTML`/`eval(`/`dangerouslySetInnerHTML`） | ✅ 零命中 |
| `git status --short --branch` | ✅ 工作区洁净（master，零改动） |

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第十三轮闭合，见上。
- **OBSERVE-76-01/02/03**：handleBack 等待期无进度反馈 / Admin uses 无上限 / Toast Close 无 aria-label——本轮复跑无升级证据，维持「续」。
- **OBSERVE-75-02 / 75-04 / 71-01 / 71-02 / 70-03 / 66-03**：维持既往。
- **可疑-1**（回显置位与合并渲染提交的理论竞态）：物理不可达，留存为稳定性契约注释候选。

## 备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + tsc -b + 三组 TDD 脚本只读复跑）；工作区 `git status` 洁净，未修改任何仓库文件（唯一写入为本报告文件，属 team-lead 明确指定的输出路径）。
- 复核与既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态、target-guard 判据、删账号 memory-first、指针身份比对族、401 单广播双形态互斥）。
- 本轮定级口径：零 CRITICAL/MAJOR；2 条 MINOR（管理端失败态吞并、保存链停手后离开无终局提示）+ 2 条新 OBSERVE + 1 条理论可疑（不可达）；连续第二十三轮无严重级发现。