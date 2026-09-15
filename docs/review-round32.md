# 第 32 轮全模块审查记录（2026-09-16）

> 审查范围：backend 全部模块（scheduler/handler/router/manager/client/store/session/runtime/
> config/secure/db/main/embed/全部测试）+ web 全部模块（App/Login/Dashboard/Select/Admin/
> client/types/useTickingCountdown/main/全部 UI 原语）。并行子代理产出：后端全模块只读审查、
> 前端全模块只读审查、真实网站逆向契约三份（请求基础设施/选课页面/HAR 真实样本比对）。
> 主 gate 逐条核实（读源码 + 推演真实触发路径）后决策：前端 3 项确认修复，后端 1 项确认修复，
> 逆向契约差异全部洗入 CLAUDE.md「真实网站源码对照基线」。另按主人指令把"持续参考真实网站
> 源码保证逆向正确性"写入 CLAUDE.md 并派子代理核对。

## 后端（1 项，确认修复）

### B32-01（MINOR）windowClosedLocked 判据2 缺"开放时间已过"——未来开窗点 + 平台故障恢复后黄金期提交被挂起
**缺陷**（review32-backend）：判据2 实现为 `syncFailStreak>=3 && !openTimeNow().IsZero()`，
只检查"开放时间非零"、没有 `now.After(open)` 比较——紧邻注释明写场景是"从未开过窗的空快照
+ **开放时间已过**"，注释与实现不符。触发链：open_time 指向未来（典型部署）→ 开窗前平台连续
不可达（教务维护/网络故障），maybeSyncClock 每 ~30s 失败一次，≥3 次（约 90s）后 syncFailStreak=3
→ windowClosedLocked 判据2 返回 true（open 非零即触发）→ tick 提交守卫挂起提交 + probeIntervalFor
恒命中降回 30s → 平台恰在开窗点恢复：syncFailStreak 只在同步成功时归零，下次同步要等 30s 退避
过期——开窗后黄金期（10s 内 250ms 冲刺）**0 提交、0 近模式探测**直到下一次同步成功（≤30s）
才自愈。判据3（EmptyProbeRuns）入账侧已有 B21-02 的 `now.After(open+10s)` 自保护，判据2 的
计数源 syncFailStreak 不依赖 probe/open、独立累计，是唯一无自保护的窗口。
**修复**：判据2 补 `&& nowAlignedLocked().After(openTimeNow())`（"开放时间已过"才视同关闭），
与 B19-01 注释原意一致。**TDD**：`TestStateForAccountMirrorsWindowClosed` 追加场景 2b
（未来开窗点 + syncFailStreak=3 不得视同关闭 + 不得镜像进状态字段）红灯（1522 行
"时钟失败 3 次但开放时间在未来"）→ 绿灯；窗口关闭/幽灵窗口测试族全量回归绿。

## 前端（3 项，确认修复）

### Select-32-01（MAJOR）handleBack 等待循环闭包陈旧——等待期间新改动被旧快照覆盖/丢失
**缺陷**（review32-frontend）：handleBack 是渲染闭包捕获的 async 函数，等待循环（最长 21s/轮 ×
3 轮）期间 setSelected/setRev 不会更新闭包里的 selected/rev 快照。触发链：selected={P:[A]}、rev=1，
点"返回控制台" → flushTargets 发 PUT [A]（savingRef=true）进等待循环 → 用户在等待窗口内反悔点选
B（selected={P:[A,B]}、rev=2）→ PUT [A] 完成退出循环 → 第二轮 flushTargets 仍用旧闭包
selected={P:[A]} → 若 lastJson 已等于 [A]（首轮 PUT 成功）直接跳过、随后 `!dirty && !saving`
break → onDone 卸载、防抖 timer（B 的保存）被 cleanup 清除 → **B 静默丢失**；若防抖 timer 恰好先
触发保存了 [A,B]，第二轮 flush 的 json=[A] !== lastJson=[A,B] 会真实 PUT 旧快照把已落库的 B
覆盖删除。F21-01/M30-02 只覆盖"在飞 PUT/退避 timer"，未覆盖"等待期间的新用户改动"。
**修复**：加 `selectedRef/revRef` 镜像（每次渲染同步 current），flushTargets 消费时刻一律读 ref
（含 rev===0 判据与 selectedCount）；handleBack break 前先 flush、记录本轮消费的 revRef 快照、
等一帧（setTimeout 0）后复查 revRef 未再变化才 break，变化则 continue 多等一轮收敛（≤3 轮）。
提交 `51a5a3e`。

### Select-32-02（MINOR）max_count=0（名额未公布）处理残留两处缺口——M30-04 同族未修完
**缺陷**（review32-frontend）：M30-04 只修了 isFull 判据，同文件另两处仍把 max_count=0 当"已满"：
「仅看有余量」筛选 `c.selected_count < c.max_count` 在 max_count=0 时 `0<0` 恒 false，把名额未公布
的新课程整体滤掉（用户以为课程不存在）；`remaining = Math.max(0, c.max_count - c.selected_count)`
在 max_count=0 时得 0，徽章显示"余 0 席"琥珀警示 + 进度色落 amber——用户以为该课程只剩 0 席/接近
满员而放弃。与已修好的 isFull 判据同屏自相矛盾。
**修复**：筛选判据补 `c.max_count === 0 ||`；remaining 仅 `c.max_count > 0` 时计算，为 0 时徽章改显
"名额未公布"、进度色回 emerald（与 fillRate 的 `!max_count → 0` 语义对齐）。提交 `51a5a3e`。

### Select-32-03 / Admin-32-03（MINOR）手写弹窗无初始焦点——打开后立即按 Esc 不生效
**缺陷**（review32-frontend）：退选确认 Modal 与删除确认 Dialog 都是裸 div 手写弹窗（role=dialog +
onKeyDown Escape），但打开后焦点仍停留在后台触发按钮上——keydown 事件 target 是后台按钮，
冒泡路径不含弹窗 div，Esc 根本不触发 onKeyDown。F21-03/F20-04 声称"补 Esc 关闭"，实际只在用户
先点击弹窗内部（或 Tab 若干次）后才生效。Login 激活弹窗因 autoFocus 在激活码输入框而可用，同族
不对称。
**修复**：两个弹窗的「取消」按钮加 `autoFocus`，初始焦点落进弹窗内，打开即按 Esc 生效。提交
`51a5a3e`。

## 逆向契约核对（三份子代理报告，差异全部洗入 CLAUDE.md「真实网站源码对照基线」）

### 报告一：选课页面 API 契约（select.js / electivesDetail.js 源码证据）
- 学期列表真实 10 字段（补 gradeName/gradeId/termName/nextSchoolYear/enrollmentYear/
  yearTermGradeText）——gradeName/gradeId 是年级隔离的直接数据源，Go YearTerm 未消费
- findElectivesData 请求体是 **form 编码非 JSON**（jQuery 对象默认序列化），空 body 平台返回
  当前激活学期；响应顶层无 msg 字段
- 课程级 `publish_id` 与 `group_no` **服务端直发，不需推导**；teacher_name_list 逗号分隔
- btn_type/can_select/title 源码证据：btn_type 1=退选（btn-danger）/2=报名（btn-primary）其他值
  不渲染；can_select false 加 disabled + 点击事件 `i.can_select && postReq` 双守卫；title 非空
  转 lay-tips 悬浮
- 实时人数轮询：`ids=<逗号分隔字符串>`、10s、只更新 selected_count/audited_count 两格；b 数组
  只在 inDateRange=true 的发布收集课程 id——窗口未开/已关前端**根本不发轮询**
- 窗口开启自动刷新：1500ms 检查 beginTimes，开启时刻前后 1.5s 内自动 f() 整页重拉
- classDetail 仍被官方页面 class_name 链接调用（UI 入口），57 字段完整可查

### 报告二：请求基础设施契约（common.js / main.js / layout.js 源码证据）
- **token 平台真相**：权威来源是 `window.idToken` 全局变量（/home/menus 响应 token 填充），
  cookie 从不承载 token——"idToken=zd_edu_cookie"是本项目存储约定非平台机制
- correctUrl 拼接判定式：`url 已含 "=" → & 拼接，否则 ?`；token 经 encodeURIComponent
- popReq 完整契约：`(url,data,success,fail,showSuccessToast=false,showFailToast=true,
  showLoading=false,async=true)` 默认成功静默/失败弹 layer.msg；data 对象 form 编码；
  `__op_tip_msg/__op_tip_seconds` 确认弹窗机制
- checkResp 完整状态码机：code=0 正常（带 guestToken 则 setGuestToken）、-1 跳 /login、-6 域名
  迁移、-10 短信验证码风控、-11 强制改密、-12 reload、-14/-15 跳 errors[0].name；**code=1 业务
  失败不 throw、成败由 isOk/isFail 决定**（Go SelectClass 双判已对齐）
- cache:false 只对 GET 生效，POST 不改写 URL（本项目全 POST 恒定 ?idToken=）
- 登录第四字段 priorityId（/home/menus 的 user.id），undefined 时 jQuery 丢弃
- getUniqueDeviceId 与 login.py 复刻逐字段一致（UA|Win32|height|width|时间戳36进制）
- 验证码 URL 首载无参数、刷新才加 ?v=

### 报告三：HAR 真实样本比对（173 条请求 + 课程 HAR + server.log + task_log）
- doLogin 四字段（captcha/identification/uniqueId/priorityId）与失败/成功响应形态全实证
- **Cookie 必要性弱化**：13 个鉴权请求 0 Cookie 头全部成功——URL 参数通道独力可鉴权
- 未开放≠关闭：未开放时 publishes 82 门完整 + title 禁用提示；关闭时 code:0 + 空 publishes
  （server.log "探测成功：0 个发布"实证）——B18-M1"先开过窗才判定关闭"前提被实证支持
- 真实错误文案：未开放 title = "不在选修报名时间范围内，无法选课！"、重复提交 = "选课处理中，
  请勿重复操作！"（task_log 实证）、报名成功 "选课成功！"
- token 位数脱敏不可考（server.log 实证前 8 位纯数字），历史"19 位"降级为参考
- classDetail 契约（body id= + value 嵌套 57 字段）实证，UI 改版前 select.js 仍含 onclick

## 已核无缺陷清单（review32-backend 详核，18 项）
1. windowClosedLocked 三判据单源（B29-02）——StateForAccount 与 WindowClosed 共用同实现
   （判据2 缺口已由 B32-01 单列修复）
2. probe() 主判据 `prevOpened && !opened && len==0 && now.After(open+10s)`（B18-M1+B26-03）；
   EmptyProbeRuns 入账 `now.After(open+10s)`（B21-02）；prevOpened 先捕获再覆写顺序正确
3. maybeSyncClock：streak 失败不清零、≥3 复位 offset（B21-01）；lastSyncFailAt 30s 退避
   （B9-03）；无客户端复位 syncing（F12-B1）；成功才推进 lastSyncTime
4. maybeRelogin：reloginMu→s.mu 锁序；成功分支落库前复核 ClientFor（B21-03）；
   reloginResults 非阻塞发送
5. MarkTokenValid 对齐锁序清 tokenValid/reloginFail/relogging（F12-B3）
6. spawnChain：链顶+goroutine 双重 ClientFor（B30-01）；tokenValidForLocked 短路（B23-03）；
   满员分支 doneHas 复核（B23-01）；成功前复核（B18-M2）；ErrUnauthorized 双路对称重登
7. submitAll：ClientFor 过滤 + 按 pID 分组排序，链并发正确
8. probe 单飞 probing（F12-B2）+ probeSem cap 4（F17-01）+ lastProbe 只归 probe/ProbeNow（B6-04）
9. ElectivesSnapshotFor：目标账号缺失/过期绝不回退全局帧（B22-01）；无目标过期专属帧返回
   (nil,false)（B28-01）
10. CheckClassSelectable 过期快照按无快照放行（B18-m1）
11. MarkDone/RemoveDone 落库前 ClientFor 复核（B20-01）
12. handleAdminDeleteAccount memory-first 顺序（B26-01）+ trim 逐字节校验（B15-M5）
13. 管理员 ?account= 四路透传 accountExists 校验（B27-01/02，handleSetTargets/handleElectives/
    handleElectiveSelect/handleElectiveExit/handleState 全落地）
14. router.go：/api/ 404 catch-all、POST/PUT 强制 JSON、DELETE 放行空 body（B31-01）、
    requireAdminSession→requireJSONBody 包装顺序正确
15. web/embed.go /api 前缀整体 404（B30-02），GET /api 不再回退 index.html
16. FormatOpenTime 空串零值（B25-01）
17. 第 32 轮前端改动核实：32-02/32-03 落地无副作用
18. go build/vet 全绿 + 全量 -race 通过（api/scheduler 等 9 包）

## 可疑待核裁决
- **review32-frontend 可疑 1（api() HTTP 401 非 JSON body 不广播）**：后端 requireAdminSession
  对无效 Bearer 返回 JSON body（code:401），纯文本/空 body 形态不存在——依赖后端响应形态，
  纯前端无法确证。**定不修，维持观察**。
- **review32-frontend 可疑 2（学生账号 401 + 管理员代理特判组合）**：inAdmin 时 lostAccount 非
  adminName 的 401 直接 return 不剔除学生会话——该事件只在本浏览器以学生 token 发请求时才发生
  （学生已登出即不再发），触发概率极低。**维持观察**。
- **review32-frontend 可疑 3（学生账号名与 adminName 冲突）**：依赖后端是否禁止同名注册，后端
  学生账号无此约束（admin 是独立管理通道）。**定不修，保留决策**。
- **review32-frontend 可疑 4（Dashboard/Select 共享 ["state",account,sessionToken] key 但 URL
  不同）**：学生场景后端两 URL 返回一致故无数据错；若未来分叉即串数据。**维持观察**。
- **review32-backend 观察 1（handleBack 三轮回合收敛上限）**：第 3 轮 continue 后循环结束 onDone，
  改动恰好落在第 3 轮等帧之后 ≤1 帧（约 16ms）窗口内会丢失——防无限挂起的刻意收敛，前两轮已
  各 flush 一次真实 PUT，触发窗口极窄，安全方向可接受。**维持观察**。

## 回归
- backend：`go build ./... && go vet ./...` 全绿；`go test -race -count=1 ./...` 全量通过
  （含 B32-01 新增断言与窗口关闭测试族）。
- frontend：`npm run build`（tsc -b + vite）通过。

## 观察项（本轮追加/延续）
- 观察 32-01：CountEntry.MaxCount 字段（countList 是否含 maxCount）未实证，需开窗期实测确认。
- 观察 32-02：Go YearTerm 未消费 gradeName/gradeId（年级隔离直接数据源）——当前按账号 per-account
  探测已隔离年级，学期列表字段仅备选路径用，低优先。
- 历轮观察项全表延续（task_log 无容量上限、emptyRunsFor/syncFailedWindow/RemoveFull 死代码、
  sortTightest 对 max_count=0 排序、handleBack 极端失败路径、tsconfig 缺 strict、reloginBackoff
  死分支、RestoreRefused 注释过时、settings captcha_concurrency 无上限、probe per-account 近超时
  排队、PUT config 空变更、HTTP 探测单飞、adminName 撞名、reloginResults 缓冲 8 丢弃等）。
