# Round 76 前端只读审查报告

基线：commit 7b1f441（R75 双 findings + 收尾总结，R75 收官 HEAD）。本轮为 **R76 前端全模块只读审查**，审查范围：web/src 全部 .ts/.tsx（main.tsx、App.tsx、api/client.ts、lib/{targetGuard,adminAuth,useTickingCountdown,utils}.ts、routes/{Login,Select,Dashboard,Admin}.tsx、components/ui/* 全部容器、types.ts、styles/global.css）+ web/scripts 三组 TDD 断言脚本 + audit.mjs 视觉护栏。**前端 R75 后零修改**（7b1f441 仅改 archive/ 三份 docs；git log 确认 web/src 自 9f5c217 后无任何提交）。仓库：E:\newCC\aaa-dsh-go\aaa-old\xuanke-auto。只读铁律全程遵守（仅 Read / Grep / Glob / Bash 只读命令 + 3 组 TDD 脚本只读复跑 + tsc -b 只读验证 + audit.mjs 只读扫描），`git status` 工作区洁净零改动。

## 概述

**R75 后前端零改动，全链重走读确认 M-1 第十二轮闭合、六防保存链逐条无回潮、登录/激活链 + 撞名学生管理态 + 人性化细节全部与承诺一致；构建与三组断言全绿（tsc -b EXIT 0 / target-guard 18/18 / admin-auth 6/6 / unauthorized 5/5 / audit.mjs 全部通过）。本轮零 CRITICAL、零 MAJOR、零 MINOR，3 条新 OBSERVE（均提示/观察级零行为）+ 既往 OBSERVE 延续 + 无可疑待核。连续第二十二轮零严重级发现。本轮重点抓"功能障碍/逻辑错误/人性化细节"，未发现任何数据丢失/静默覆盖/安全漏洞/核心功能不可用。**

---

## M-1 延续管理（第十二轮）

### 前端 R75 后零修改确认

- `git log --oneline -8 -- web/`：最近前端改动为 9f5c217（R73 收尾注释）、382b7a6（R72 收尾）、175ed72/4a9dea4（R71 剥离标签）；7b1f441（R75）`git show --stat` 仅 archive/ 三份 md，前端零触及。
- 故本轮 M-1 延续管理以"与 R74/R75 基线逐字符比对 + 全链重走读"为正确姿势。

### M-1 第十二轮核对（逐字符比对 + 行为走读）

- **三消费点全部传 `echoedRef.current` 第三参**（三参数签名 `shouldDeferSave(stateDataRef.current, <hasSelected>, echoedRef.current)`）：
  - 防抖回调 Select.tsx:684 —— `selectedCount > 0`；:681-683 注释"已回显完成的稳态…不闷死 / 未回显仍置脏"完整保留。
  - flushTargets :500 —— `latestSelectedCount > 0`；:498-499 注释完整。
  - handleBack 判定 :593 + while :601 —— 两处均 `hasSelectedNow()`（:575-576 消费时刻读 selectedRef 计算）；:590-592/:599-600 注释完整。
- **echoedRef 置位三路径 + 首帧不置位边界**：
  - courses 空分支 :238-239（置 true + echoDone 同步）——"确证后端无旧目标"语义，全程无旧目标账号的改动经 echoDone 解锁落库。
  - 合并完成分支 :295-296（置 true + echoDone 同步 + 合并内 stale 清理 :287-294 toast 兜底）。
  - account reset :200（置 false + setSelected({}) + rev 清零 + echoDone false）——App 双挂载点 key={account} 已整体重建实例，此处兜底重置防旧账号污染。
  - 首帧未到 :232 `if (stateData === undefined) return` 前置 return 绝不置位；独立清理 effect :316-327 首行 `if (!echoedRef.current || publishes.length === 0) return` 守卫 + eslint-disable——完成回显前的清理绝不干扰回显合并时序。
- 回显 effect 依赖 :297 `[stateData, data, rev, selected, toast]`；data/rev/selected 随轮询重建引用但首行短路零开销。
- **TOAST 冒泡进 updater 排查**：:287-294 的 stale 清理 toast 位于 effect 体内（非 updater），:320 独立清理 effect 同款——updater 纯函数要求（:341-345 注释）无违反；pick() 已改为"事件处理器内先算 next 快照再 setSelected(next)"，toast 在 setState 外，无副作用混入。
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
| F43-M1 shouldDeferSave | targetGuard.ts:64-71 + 三消费点传第三参 | 纯数据判据三分支（`undefined→true` / `echoed→false` / `courses 非空 && hasSelected`）与注释逐一对应；脚本 18/18 全绿 |
| F42-M1 判据与数据源解耦 | :680-687 防抖回调 stateDataRef + effect 依赖 :745 `[rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData]` | 判据只读 stateDataRef（非 echoedRef），/state 到达触发 effect 重跑自愈；stateData/echoDone/hasPublishes 三路解锁闭合 |
| F40-M1 cleanStaleSelected | targetGuard.ts:26-43 + 独立清理 effect :316-327 | 只删"非空且不在集合"key、空 key 保留、无变更返回原引用（:42 `return changed ? next : selected`）；脚本场景 F-J 全绿；effect 依赖含 selected（:305-311 注释覆盖"合并后掉队清残留"时序） |
| F39-M1 消费时刻双闸 | 防抖 :695-698 / :704-714 / :720-723 / :726-729 + flushTargets :507-509 / :519-529 / :547-549 / :553-556 | 四判据逐条重读：发布缺席+已有选中→置脏 / stale 残留→置脏+toast / 联查空集+已有选中→置脏 / 发布 id 漂移→置脏——全部消费时刻读最新 publishesRef/selectedRef/stateDataRef |
| F36 回显真合并 + rev>0 全清空不合并 | :250-279 | 按 publish_id 真合并（已触碰发布保留用户现状含空数组、未触碰补旧目标）、`:254 rev > 0 && !anyHas` 不合并；:277 `!hasTouched && !merged` 保持现状不返新引用 |
| F48-M1 清空语义 | shouldDeferSave 第二参 + 防抖 :720 / flush :547 全清空放行 + 回显 :254 全清空不合并 | 首帧携带旧目标但用户全清空 → hasSelected=false → 放行 PUT []；清空语义绝不复活——第七轮维持闭合 |
| key={account} | App.tsx:294（targetAccount 分支）/ :340（current 分支） | 双挂载点 key 绑定账号；Select 内兜底守卫 :195-202（account 变化复位 selected/rev/echoedRef/echoDone）延续，TDZ 注释（:193-194）正确 |
| Toast 定位 | Toast.tsx:105 viewport `fixed bottom-4 right-4 z-50` + 布局类在 viewport 上（:70-104 注释 wrapper 空壳删净） | 无回归；同 title 去重合并（idRef 自增 + findIndex 更新 description 保留 duration）逻辑自洽 |

### 防抖 effect 细节复核（重点风险位重走读）

- **resetRetry 放 effect 顶部**：:650-653 `if (rev === 0) return; resetRetry()` —— 用户新改动立即清旧退避 timer，杜绝"旧退避 2s 后把新改动旧快照刷掉"的不一致路径；下一轮保存由防抖路径正常驱动。
- **setTimeout async 回调闭包**：回调读 `selected`（:657/:704）为 effect 创建时快照，`publishesRef`/`stateDataRef`/`selectedCount` 消费时刻语义——selected 快照正确性由"effect 依赖含 selected"（:745）兜底：任一 pick 均重建 effect 冻结新闭包；selectedCount 渲染期常量在守卫里做"偏保守不会放过真实假清空"判据（OBSERVE-71-01 同源，安全方向）。
- **saveNow 去重/串行化**：:426-428 `json === lastJson.current` 跳过重复；:434 卸载后成功不落 lastJson；:437-448 catch 置 dirty + scheduleRetry（attempt≥5 停）；:449-457 finally `savingRef=false` + `dirtyRef && attempt===0` 补发最新快照——四门卸载保护（:423/:434/:439/:451）拦截卸载后一切交互，无孤儿请求。
- **handleBack 保存链静止等待**：:607-638 三轮循环；`:618 if (!dirtyRef.current && !savingRef.current)` 等一帧复查 revRef 一致才 break；`:629-636 pendingSaving()`（savingRef || retryState.timer）覆盖退避 timer 排队在飞；21s 兜底绝不无限挂起；持续失败 attempt≥5 后保存链停（timer null + saving false）→ 循环自然收敛 onDone——"已尽最大努力"语义而非静默假清空。

---

## 登录/激活链 + 撞名学生管理态 + 人性化细节核（复跑）

### 登录/激活链（Login.tsx + client.ts + App.tsx）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 登录幂等守卫 | Login.tsx:36 `if (loading) return` | 连按两次 Enter/快速双击只发一请求 |
| 票据贯通 | :54 `setPendingTicket((e.data?.ticket as string) || "")` + :78 激活请求 body 带 `ticket` | 1001 分支保存 data.ticket（ApiError.data 透传 client.ts:30-33）随激活回传；取消激活清票；"过期/已用"与"激活码错误"两分支同款清票、文案差异化引导——完整 |
| 激活幂等守卫 | :68 `if (activating) return` | 与 submit 对称 |
| 401 单广播 | client.ts:64 HTTP 状态码前置广播 + :85 body 层 `r.status !== 401` 兜底 | HTTP 401 单次、HTTP 200+body 401 单次、双形态互斥不重复；HTTP 非 JSON 体先广播再抛 -2 |
| 20s 超时兜底 | client.ts:56 `setTimeout(() => ctrl.abort(), 20000)` + :103 AbortError→"请求超时，请重试" | 与注释"20 秒"一致；signal 显式接入（:58 `rest.signal ?? ctrl.signal`）调用方优先 |
| 登出吊销 | App.tsx:112-132 | apiLogout 作废服务端令牌 + 快照式三连 + 清 adminToken/targetAccount/page 全路径 |
| onDeleted | App.tsx:160-178 | 快照式三连 + 清 targetAccount/current + 删管理员自身退管理态+清标记（:172-177） |
| onUnauthorized | App.tsx:187-235 | 按令牌反查归属（:193-197 lostRaw=session 优先再 account）、管理代理态不误杀（:217 `isCurrentAdminSession && inAdmin && lostAccount !== adminName` 提前 return）、删管理自身退管理态+清标记（:210-215）、快照式落盘（:225-230）——全链路完整 |

### 撞名学生管理态（adminAuth.ts + App.tsx 渲染判据）

- `isCurrentAdminSession` 纯函数：`adminToken !== "" && sessions[adminName] === adminToken`；渲染判据 :299 `inAdmin || isCurrentAdminSession(sessions, adminName, adminToken)` 与账号迁移 effect :147 同源；admin-auth 6/6 全绿。**第十二轮零回潮**。
- 管理令牌持久化 `xk_admin_token` 全路径清除点：logout :121-122 / onDeleted :175-176 / onUnauthorized :213-214 / onBackToStudent（App :312-313）四处全覆盖，无泄漏路径。
- 后端交叉核证：B43-04 管理员双条件（`Account == adminName && ConstantTimeCompare`）已由 handler 登录分支负责，前端绑定"响应带 adminName"标记令牌——两侧契约一致。

### 人性化细节核（复跑 + 本轮新增关注）

| 项 | 位置 | 核证结论 |
|---|---|---|
| 倒计时 begin_times 兜底 | Select :757-762 / Dashboard :190-195 | 识别缺席（open_time_known=false）用 `begin_times[0]` 兜底，识别槽建立后以识别真值为准——CLAUDE.md 契约 26 承诺一致 |
| 倒计时横幅五态 | Select :817-843 | `!stateData`→同步中 / window_closed→已关闭 / window_opened→已开放 / 双缺席→未识别到开放时间 / isExpired→本地已到点等待 / 正常倒数；`|| cd.isExpired` 已清除，无"关闭后恒显已开放"矛盾 |
| 倒计时自校正 | useTickingCountdown :16-18 `setNow(Date.now())` | target 变化（首次同步/开窗瞬间）立即回正，消除 1s 陈旧偏差 |
| btn_type 三向 | Select :1106 `===1` 退选 / :1119 `===2` 报名 / 其他不渲染 | 与官网逆向契约一致；can_select 双守卫 disabled+title；开窗瞬间窗口信号缺失时按钮置灰由 can_select 决定不锁死手动通道（:1102-1104 注释） |
| max_count=0 名额未公布 | :957 筛选不过滤 / :996 isFull `max_count>0 &&` / :999-1000 unannounced / :1082-1084 文案 / :1091-1095 Progress value=0 max=1 | 四处同源 `max_count>0` 判据；未公布课程不误显"已满额/余0席/满条"，进度条空条如实（aria-valuenow=0 语义正确） |
| 空态 | Select :906-912 无可选批次 / Dashboard :535-545 无预选课程 | 均有明确文案 + 行动引导；顶栏徽章分母 `publishes.length > 0 ? '/'+len : ''`（:786-787）杜绝 "/0" |
| aria 无障碍 | 退选弹窗 role=dialog+Esc+autoFocus（Select :1191-1224）/ 删除弹窗同款（Admin :210-243）/ 激活弹窗 + Esc（Login :223-238）/ switch role+aria-checked（Admin :568-572）/ progressbar（Progress :23-28）/ CollapseSection useId（Dashboard :64-67）/ 搜索 aria-label | 全站完备；useId 消除重复 id |
| 在飞幂等全站点 | 登录/激活/报名/退选/删账号/删激活码/生成码/保存配置 | Set 独立跟踪无互踩（Select :52-53 / Admin :311），disabled 渲染延迟前入口短路齐全（Login :36/:68 / Admin :327/:349/:519） |
| 手动报名/退选后端契约 | client.ts:112-138 selectElective/exitElective ↔ handler.go:285-433 | body `{class_id, course_name}` 与 `{class_id}` JSON 解码一致（ElectiveActionRequest）;凭据表校验 :293-301 查无此账号拒绝;TryAcquireSubmit 在飞互斥;ErrUnauthorized 触发 MaybeRelogin 自动重登;read err 区分文案——契约全链一致 |
| 速率限制等无 UI 阻塞 | Admin copy execCommand 兜底（:82-109） | 剪贴板非安全上下文降级完整，仍失败展示激活码人工抄录——无死路径 |

---

## 发现清单

### CRITICAL

无。

### MAJOR

无。

### MINOR

无。

### OBSERVE（3 条新增 + 延续项）

**OBSERVE-76-01 — handleBack 保存静默等待期零进度反馈（延续 R73 可疑-1 / OBSERVE-75-01 复跑）**

- 位置：Select.tsx:571-640（handleBack 全链 + :607-638 收敛循环）。
- 触发场景推演：点"返回控制台"时若在飞 PUT 未完成（savingRef true）或退避重试 timer 排队中（pendingSaving），按钮挂起等待最多 3×21s=63s，期间界面无任何"正在保存目标..."文案或 loading 指示——用户可能误以为按钮卡死而重复点击（重复点击再次触发 handleBack 进入新一轮等待），或直接刷新页面丢失内存改动。
- 复核结论：完整业务链（先 flush → 等回显 → 等保存链静止 → 收敛）正确，63s 上限非挂起且绝不假清空；本轮复跑确认内存残留零风险（卸载前 finally 自动补发 + 等待收敛）。**建议裁决：续**（人性化增强候选，非缺陷——可加轻量"正在保存目标..."文案）。

**OBSERVE-76-02 — Admin CodesTab uses 输入无前端上限（延续 OBSERVE-75-03 复跑）**

- 位置：Admin.tsx:385-392 `setUses(Math.max(1, Number(e.target.value) || 1))`（仅下限钳制）。
- 触发场景推演：管理员输入 "5000" 可提交（前端放行），后端 handleAdminCodes `req.Uses > 1000` 拒绝报"每个激活码使用次数上限为 1000"，前端弹失败 toast——功能正确但 UX 有一步返工。
- 复核结论：`count` 输入已限 1-100（:380 `Math.min(100, ...)`）而 `uses` 未对称限流；后端值域校验完备、数据安全零风险。**建议裁决：续**（可选对称加 `Math.max(1, Math.min(1000, ...))`，纯 UX）。

**OBSERVE-76-03 — Toast 关闭按钮无 aria-label（新增观察项）**

- 位置：Toast.tsx:100-102 `<ToastPrimitive.Close>` 无 aria-label/title。
- 触发场景推演：读到 toast 文案后键盘/读屏用户 Tab 到关闭按钮时，屏幕阅读器朗读的默认名称为 "Close"（Radix 关闭按钮无隐式标签），非中文"关闭"提示，与全站中文化 aria 风格略不一致。
- 复核结论：功能性零影响（点击/Enter 均正常关闭），aria 细节瑕疵。**建议裁决：续**（可选补 `aria-label="关闭"`）。

**OBSERVE-75-01（延续）**：handleBack 等待期无进度反馈——本轮并入 OBSERVE-76-01，维持「续」。

**OBSERVE-75-02（延续）**：Dashboard extrasMs 随 /state 轮询重建引用（useMemo 依赖语义正确、实际重算频率≈0），维持「续」。

**OBSERVE-75-04（延续）**：Select 空态卡与 window_closed 信号弱相关，维持「续」。

**OBSERVE-71-01（延续）**：防抖 effect 缺失 selectedCount 依赖——回调读 selectedCount 为闭包快照、由 selected 依赖兜底，安全方向，维持「续」。

**OBSERVE-71-02（延续）**：Dashboard F10-06 动态目标集合相关注释并存，维持「续」。

**OBSERVE-70-03（延续）**：Select 搜索框 aria-label 与 placeholder 同串，维持「续」。

**OBSERVE-66-03（延续）**：setSelected 五调用点无新增，维持「续」。

### 可疑待核

无。（前轮可疑-1"目标保存 PUT 的 course_name 不携带发布元数据"已由后端 enrichTargetPubMetaLocked 双段补全 + restoreTargets 启动兜底闭合，本轮复跑确认无新疑点。）

---

## 已核无缺陷清单

- 前端 R75 后零修改确认（git log 谱系核证），M-1 第十二轮三消费点逐字符比对零回潮、echoedRef 置位三路径 + 首帧不置位边界完整。
- 六防保存链（F43/F42/F40/F39/F36/F48-M1 + F15/F16/F17 消费时刻双闸）：逐条判据与注释逐一对应，渲染期常量清退完成（targetsUseCurrentPublishes 仅在消费时刻调用），自愈链（stateData/echoDone/hasPublishes 三路解锁）完整闭合，handleBack 收敛循环（维度覆盖退避 timer 在飞 + 21s 兜底）无挂起路径。
- 撞名学生管理态判定：admin-auth 6/6 全绿，刷新恢复判据正确，令牌标记四清除点全覆盖。
- 票据贯通 / 401 单广播（HTTP 401 前置与 body 401 兜底互斥） / 20s 超时兜底 / 登出吊销 / onDeleted / onUnauthorized：全链路完整，快照式三连一致无覆盖丢失。
- btn_type 三向 / max_count=0 名额未公布四处同源 / 倒计时兜底 / 空态 / aria 全站完备（含弹窗 Esc + autoFocus + switch/progressbar 语义）。
- 轮询降频全站统一（window_closed/failure 30s、黄金期 2s、其余 10s、Dashboard 3s/30s），refetchInterval TDZ 规避正确（Select :77 经 queryClient.getQueryData 读 /state 缓存，绝不直读组件顶部 stateData）。
- 后端交叉核证族（新增本轮）：handleElectiveSelect/Exit 凭据表校验 + 手动路径 ErrUnauthorized→MaybeRelogin + read err 文案区分 / markFullLocked 每课一次 + full 不重复写日志 / releaseFullIfFreedLocked 空快照不解封（窗口关闭防轰炸）。handleSetTargets 数量/范围/发布 id 校验与前端 TargetsRequest JSON 解码交叉核证一致。
- XSS/敏感数据/硬编码开放时间：零命中（无 innerHTML/dangerouslySetInnerHTML/eval；token 只存 localStorage + Bearer 头发送；开放时间唯一事实源注释清晰）。CSS 无危险 URL 注入（background 无 url() 动态拼装）。
- localStorage 读写全 try/catch 降级（App.tsx load/save 六处 + adminName/adminToken 两处），隐私模式静默降级不崩。
- 视觉护栏 audit.mjs 全部通过：画布基础类齐全（180%/160%/--bg-shift/遮罩/真实 img）、弹窗遮罩统一 .glass-overlay、无 bg-black/25|30|70|75 与整页 bg-black 违例（Select:1013 bg-black/15 选中态与 :1193/Admin:212 bg-black/80 弹窗遮罩为审查豁免项——半透明玻璃形态，非 audit 拦截清单）。

## 构建验证

| 项 | 结果 |
|---|---|
| `npx tsc -b`（web/ 下） | ✅ EXIT 0（类型全通过，无 TS2448/TS2454 等 TDZ 误触） |
| `node --import jiti/register scripts/target-guard-check.ts` | ✅ 18/18 全绿 |
| `node --import jiti/register scripts/admin-auth-check.ts` | ✅ 6/6 全绿 |
| `node --import jiti/register scripts/unauthorized-check.ts` | ✅ 5/5 全绿 |
| `node scripts/audit.mjs` | ✅ 全部通过（视觉护栏） |
| 全仓残留扫描（`见 \d+ 行`/`innerHTML`/`eval(`/`dangerouslySetInnerHTML`） | ✅ 零命中 |

## 历轮观察延续

- **M-1**（echoedRef 第三参数稳态语义）：第十二轮闭合，见上。
- **OBSERVE-66-03**（亚帧 setSelected 五调用点）：无新增，维持「续」。
- **OBSERVE-70-03 / 71-01 / 71-02 / 73-01~04 / 74-01~04 / 75-02 / 75-04**：本轮复跑无升级证据，维持既往。
- **OBSERVE-75-01**（handleBack 等待期无进度反馈）并入本轮 OBSERVE-76-01；**OBSERVE-75-03**（uses 无前端上限）并入 OBSERVE-76-02。

## 备注

- 只读铁律全程：仅 Read/Grep/Glob/Bash（只读命令 + 3 组 TDD 脚本只读复跑 + tsc -b 只读验证 + audit.mjs 只读扫描）；工作区 `git status` 洁净，未修改任何仓库文件（唯一写入为本报告文件，属 team-lead 明确指定的输出路径）。
- 复核与既有决策锚零冲突（开放时间事实源、F10-06 动态集合、M-1 稳态、target-guard 判据、契约 20 剥离完成度、删账号 memory-first、指针身份比对族、401 单广播双形态互斥）。
- 本轮定级口径：零 CRITICAL/MAJOR/MINOR；3 条新 OBSERVE（均提示级零行为）+ 5 条既往 OBSERVE 延续；无可疑待核；连续第二十二轮无严重级发现。