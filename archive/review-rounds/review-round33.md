# 第 33 轮全模块审查记录（2026-09-16）

> 审查范围：backend 全部模块（scheduler/handler/router/manager/client/store/session/runtime/
> config/secure/db/main/embed/全部测试）+ web 全部模块（App/Login/Dashboard/Select/Admin/
> client/types/useTickingCountdown/main/全部 UI 原语）。并行子代理产出：后端全模块只读审查
> （code-reviewer opus）、前端全模块只读审查（code-reviewer opus）。主 gate 逐条现场核实
> （读源码 + 推演真实触发路径）后决策：后端 1 项确认修复 + 1 项顺手收敛，前端 1 项确认修复
> + 1 项观察项顺手修复。

## 后端（B33 系列，2 项确认修复 + 2 项维持观察）

### B33-01（MINOR）错误落库吞错——success/refused 行落库失败被静默忽略，重启恢复契约可被破坏
**缺陷**（review33-backend）：scheduler.go 三处落库调用点对返回错误一律 `_ =` 吞掉：
- spawnChain 成功分支（约 1325-1326）：`s.store.AppendLog(...)` 丢错误、`_ = s.store.SaveSuccess(...)`
- MarkDone（约 1706-1707）：`_ = s.store.SaveSuccess(...)`、`_ = s.store.AppendLog(...)`
- RemoveDone（约 1749-1754）：`_ = s.store.DeleteSuccess(...)`、`_ = s.store.SaveRefused(...)`、`_ = s.store.AppendLog(...)`
**触发路径**：SQLite 单写者 SetMaxOpenConns(1) 串行执行，正常几乎不失败；磁盘满 / IO 错误 /
数据库文件锁异常时写事务失败返回 err——此刻内存态已写入（done/refused 集合、状态 Courses），
唯独库行没落上。RemoveDone 的 refused 行丢失：重启后 LoadRefused 读不到该课 → 恢复路径把用户
已手动退选的课当新目标重新抢回（B9-02 契约被静默破坏）；SaveSuccess 行丢失：重启后 RestoreDone
无该课记录 → 已报名成功的课被重新提交（平台返回"选课处理中，请勿重复操作！"，无害但刷屏）。
**修复**：三处落库全部捕获错误并 `log.Printf` 记录（含退选落库失败时"重启后自动引擎可能抢回"的
警示语），成功路径行为零改动。**TDD**：新增 `TestStoreFailuresLogged` —— 引入 `failStore`（
`fail` 开关置真后 SaveSuccess/SaveRefused 返回错误），MarkDone + RemoveDone 各触发一次失败落库，
断言标准日志出现"落库失败"字样；红灯（日志只含成功/退选正常行）→ 绿灯。

### B33-02（可疑 1，顺手收敛）probeIntervalFor 对 openTimeNow() 三重读取——热改开放时间极小窗口下反复切换探测间隔
**缺陷**（review33-backend 可疑 1）：函数内 73/76/82 行三次 `s.openTimeNow()` 每次重新读
runtime.Store 快照，管理员 hot-reload 开放时间的亚毫秒窗口内两次读取可能不一致：一次落 30s
远间隔、下一次落 2s 近探测。无实质危害，但与 B20-03 同族的时间基准漂移。**修复**：函数入口
`open := s.openTimeNow()` 取一次快照复用。行为零差异，仅消除亚毫秒漂移窗口。

### 维持观察
- **可疑 2（tick 提交守卫锁外 open vs WindowClosed 锁内再读 TOCTOU）**：属实但被 300ms tick
  循环自然吸收，且修法需同步动 tick 守卫取值位置（与 B33-02 不同函数），属防御性重构——
  **维持观察**，若未来热改开放时间 + 提交判定需要精确到亚毫秒再一并收敛。
- 观察项 1-8 全表延续（死代码族 / lastSyncFailAt 本地钟 / settings 并发无上限 / HTTP 探测单飞 /
  task_log 无容量上限 / PUT config 空变更 / spawnChain 单次调用 + reloginResults 缓冲 8 /
  WindowClosed 闩锁翻转）。

## 前端（Select-33 系列，1 项确认修复 + 1 项观察项顺手修复）

### Select-33-01（MAJOR）返回按钮的 flush 可在回显合并之前执行——后端旧目标被整包静默覆盖
**缺陷**（review33-frontend）：进页后 `/electives`（课程列表）与 `/state`（已保存目标）并发拉取。
`/state` 首帧晚到或失败 retry 时，回显 effect 因 `stateData?.courses` 为 undefined 直接 return、
`echoedRef` 保持 false，后端旧目标从未进入 `selected`。此时用户点选课程 C → `rev=1` → 点击
"返回控制台" → handleBack 第一轮 `flushTargets()` **不等回显**，直接由 `[publishesRef × selected]`
联查构建 `targets=[C]` → 三处守卫全过（publishes 非空、targets 非空、publish_id 属当前发布集）
→ PUT `{"targets":[C]}` 整包覆盖，本意"添加一门"变"替换全部"，且保存成功静默无提示。
M30-03/31-01 已修复"数据晚到用户先点 → 回显被 rev>0 跳过"的正向合并路径，但**返回路径在
stateData 到达前 flush 不触发任何合并逻辑**——这是修复链的最后一环缺口。
**修复**：handleBack 首轮 flush 前增加"回显等待"——`!echoedRef.current && (stateData 未到 ||
courses 非空)` 时最多等 5s（10ms 轮询 echoedRef），再等一帧（渲染提交落地 selectedRef 同步），
期间回显 effect 把旧目标补进 selected，flush 自然全量提交；`stateData` 已到且 courses 为空
（确证后端无旧目标）立即放行；5s 兜底：/state 持续失败时合并永不发生，等无可等继续——flush
内假清空守卫仍拦截发布缺席的覆盖（安全方向）。**TDD 形态**：前端无单测框架，以 `npm run build`
（tsc -b 严格类型检查）+ 触发路径手工推演作为验证。提交 `22fa4d2`。

### Select-33-02（观察项 3，顺手修复）max_count=0 容量文案形态误导
**缺陷**（review33-frontend 观察项 3）：`fillRate` 在 `max_count=0` 时显示
"`3 / 0 人 (0%)`"——与"名额未公布"徽章并存视觉矛盾（M30-04/32-02 只覆盖了徽章/筛选/isFull，
未覆盖该行数字）。**修复**：`unannounced` 时改显 `"N 人已报 · 名额未公布"`。提交 `00bd41c`。

### 维持观察
- 观察项 1（防抖/flush 守卫拦下脏改动无恢复信号——发布集非空→非空重建时 hasPublishes 布尔
  不变不驱动重试，悬空改动待到下次用户改动才落库；安全方向不假清空，与 22-05 幽灵条目同源）
- 观察项 2（handleBack 循环期间发布恒空的脏块三轮后 onDone 丢弃且无提示——安全方向刻意行为）
- 观察项 4（Admin 五 Tab 子组件 state 跨挂载重置，F12-M2 注释已声明）
- 观察项 5（Select 组件无 key——当前 targetAccount 必经 null 中间态不 A→B 复用；未来若加
  直切入口需补 key）

## 已核无缺陷清单（子代理详核，20 项后端 + 9 项前端）

后端：锁序 s.mu→m.mu/reloginMu→s.mu 全路径无死锁；Courses 列表原地过滤底层数组复用安全；
删除账号防线三路齐全 + memory-first；windowClosedLocked 三判据单源（B29-02 共用）；spawnChain
四重防线（B30-01 双 ClientFor/B23-03 tokenValidForLocked/B23-01 doneHas/B18-M1 isWindowClosedError）；
maybeRelogin 决策段锁序 + B21-03 复核 + 30s 退避 + syncing 调用侧置位 + 无客户端复位；probe
主判据 + EmptyProbeRuns 双 10s 裕量 + B21-01 streak 达 3 只复位 offset；tick 提交守卫顺序与
黄金期/探测间隔契约；zhidao 客户端 idToken+cookie 双通道 / code=-1 回 ErrUnauthorized / code=1
不 throw（isOk 双判）/ form 编码 / Login 收敛 / 成功写账密 / SetVision 不挥动引擎 / B29-01 模板
透传；handler accountExists 四路透传矩阵 + handleSetTargets 字段域校验 + loginLimiter XFF 仅回环；
router requireJSONBody 只包 POST / DELETE 空 body 放行 / /api/ 404 + embed /api 前缀 404；
secure .master_key 32 字节同校 + AES-GCM nonce 前置 + enc: 拒绝旧明文；session 票据单次防重放 +
12h TTL；store 事务族原子性；manager B24-01 wasShell 判别 + B29-01 SetRecognizer 写模板 +
gateWait 有界；main 启动序 FormatOpenTime 空串零值 + Restore 契约序 + http.Server 超时；db
SetMaxOpenConns(1) + schema v4；config/runtime 环境变量+data/.env + 前置校验闭包外；captcha
normalizeCaptchaText 3-5 位 + captchaLimiter 热收敛；cmd 工具链无缺陷。

前端：跨账号代理残留不成立（返回必经 setTargetAccount(null) 卸载）；401 多 tab 误杀管理员
不成立（lostAccount 特判分支 return）；Dashboard/Select /state 同 key 缓存冲突数据等价无缺陷；
防抖 400ms 窗口内新改动 + 旧 PUT 在飞无乱序丢失（savingRef+lastJson 串行化）；回显合并全清空
守卫成立（31-04）；StrictMode 双挂载 unmountedRef 成立（F20-01）；flushTargets 双发 PUT 成立
（lastJson 去重 + savingRef 补发）；手动报名/退选在飞 Set 跟踪成立（M29-01）；ConfigTab 保存后
refetch 回填收敛（R27-01/S28-02）。

## 契约一致性核对（对照 legacy/ 真实源码基线 + CLAUDE.md 逆向契约）

- class_room_name 与原合同源；课程级 title 承载禁用原因；删除账号页从不写 body。
- beginTimes 顶层字段、currentYearTermList 10 字段、countList {id, selectedCount, auditedCount}
  （maxCount 未实证）均有 HAR 实证支撑。
- code=1 不 throw 双判、form 编码非 JSON、idToken URL 通道独立可鉴权——与逆向基线一致。
- isWindowClosedError 已知缺口（真实关闭文案"无效的课程ID"不在匹配集合）已被 B18-M1 tick
  提交守卫 WindowClosed() 前置挡住，不再构成轰炸路径。
- 全部一致，无不符项。

## 回归

- 后端：`go build ./... && go vet ./...` 全绿；`go test -race -count=1 ./...` 全量通过
  （含 B33-01 新增断言，api/scheduler 等 9 包 ok）。
- 前端：`npm run build`（tsc -b + vite）通过（Select-33-01 + 33-02 落地后）。

## 观察项（本轮追加/延续）

- 观察 33-01：tick 提交守卫 open 读取 TOCTOU（被 300ms tick 吸收，热改窗口亚毫秒级，维持观察）。
- 观察 33-02：防抖/flush 守卫拦下脏改动无恢复信号（发布集非空→非空重建，安全方向）。
- 观察 33-03：handleBack 循环期间发布恒空脏块三轮后丢弃无提示（安全方向刻意）。
- 历轮观察项全表延续（task_log 无容量上限、死代码族 emptyRunsFor/syncFailedWindow/RemoveFull/
  Decrypt、HTTP 探测单飞、probe per-account 错误静默、幽灵课程条目无 UI 提示、sortTightest 对
  max_count=0 排序、tsconfig 缺 strict、reloginBackoff 死分支、RestoreRefused 注释过时、settings
  captcha_concurrency 无上限、PUT config 空变更、adminName 撞名、reloginResults 缓冲 8、CountEntry
  maxCount 未实证、Go YearTerm 未消费 gradeName/gradeId 等）。
