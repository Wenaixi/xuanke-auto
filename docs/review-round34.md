# 第 34 轮全模块审查记录（2026-09-16）

> 审查范围：backend 全部模块 + web 全部模块。并行子代理产出：后端全模块只读审查
> （review34-backend，已完成）、前端全模块只读审查（review34-frontend，已完成，报告经
> SendMessage 补全）。主 gate 逐条现场核实（读源码 + 推演真实触发路径）后决策：
> 后端 3 条 MINOR 确认修复；前端 2 项确认修复 + 1 项可疑裁决修复 + 1 项场景 B 自愈链
> 补强；注释轮次前缀标签全仓库清理（web 21 处 + backend 38 处）。

## 前端（Select-34 / App-34 系列）

### Select-34-01（MAJOR）防抖自动保存路径无"回显未完成"守卫——/state 首帧晚到时防抖 PUT 覆盖后端旧目标
**缺陷**（review34-frontend MAJOR 1）：handleBack 返回路径已有 33-01 的 5s 回显等待，但
**核心停留用法**——进页后用户在选课大厅停留、防抖自动保存（400ms）路径完全对称缺口：
防抖 effect 的消费时刻三处守卫（publishes 缺席 + 已有选中 / 联查产物为空 + 已有选中 /
publish_id 漂移）均无 `echoedRef` 检查。触发路径：进页 → `/electives` 课程列表先到 →
`/state`（已保存目标）首帧晚到或失败 retry → 用户在首帧到达前点选课程 C → rev=1 →
防抖 effect 依赖 [rev,...] 重跑挂 400ms timer（首帧到达前后都会触发）→ 回调消费时刻读
`publishesRef`（非空）+ `selected`（只含 [C]）→ 三守卫全过 → PUT `{"targets":[C]}` 整包覆盖
后端 [A,B] → 首帧回显到达时后端已经是 [C]，旧目标 [A,B] 静默丢失且无提示（"添加一门"变
"替换全部"复发）。M30-03/31-01 只修了"回显被 rev>0 跳过"的正向合并路径；33-01 只修了返回
按钮 flush 路径；**防抖路径是修复链真正的对称缺口**。
**修复**：
- 新增 `stateDataRef` 镜像（防抖 effect 依赖不含 stateData——轮询刷新不得重置 400ms 窗口，
  回调闭包捕获的 stateData 恒为 effect 创建时的旧值；必须读 ref 判"首帧是否已到/有无旧目标"）。
- 防抖回调消费时刻首加守卫：`!echoedRef.current && (stateDataRef.current === undefined ||
  (stateDataRef.current.courses?.length ?? 0) > 0)` → 置脏 return（等回显合并）。
  合并触发 selected 变化 → effect 重跑 → 新 timer 携带完整目标落库（自愈）；
  courses 为空 = 确证后端无旧目标 → 直接放行。
**验证**：`npm run build`（tsc -b + vite）通过。提交 `85d6921`（与下述两项同批）。

### Select-34-01 补充（场景 B 自愈链）无旧目标账号的防抖改动不被守卫永久拦下
**缺陷**（主 gate 现场深查发现）：echo effect 对"courses 空 = 确证后端无旧目标"分支只置
`echoedRef`（不写 selected）——防抖 effect 依赖不含 echoedRef（ref 变化不触发 effect），
置脏的改动在场景 B（全程无旧目标的账号）下无任何信号驱动重试，被 34-01 守卫永久拦下。
**修复**：echoedRef + 新增 `echoDone` state，空 courses 分支一并置位；防抖 effect 依赖加
`echoDone` 驱动重跑（echo 只完成一次，不重置 400ms 窗口）。**验证**：`npm run build` 通过。
提交 `2d1b8b2`。

### App-34-02（MINOR）onUnauthorized 未同步退出管理态——管理员会话吊销后管理态残留
**缺陷**（review34-frontend MINOR 2）：onUnauthorized 清除被吊销账号的本地会话时，只在
`setTargetAccount(prev => prev === lostAccount || lostAccount === adminName ? null : prev)`
退出代理态；`lostAccount === adminName`（管理员自身会话被吊销）时不 `setInAdmin(false)`。
而 account-reselect effect 会把 current 切到剩余学生账号（sessions[admin] 已删）→ 渲染命中
`inAdmin || current === adminName` 分支 → Admin 用学生令牌拉管理接口（403 假象）+ 后续
401 事件按管理员逻辑误判（连锁误删学生会话）。
**修复**：`if (lostAccount === adminName) setInAdmin(false)`——与 setTargetAccount 同块。
**验证**：`npm run build` 通过。

### Select-34-03（可疑 3，裁决修复）btn_type 异常值（非 1/2）被渲染成报名按钮——官网契约漂移
**缺陷**（review34-frontend 可疑 3）：原三元链 `btn_type === 1 ? 退选 : 报名` 的 else 兜底把
一切非 1 值（0/3/undefined）渲染成"报名"按钮。对照真实网站源码
（legacy/website-source/...select.js formatter）：`1==btn_type → bt_signOut（退选）、
2==btn_type → bt_signIn（报名）、其他值不渲染任何操作按钮`。异常 btn_type 值若携带
can_select=true，点击会真实触发报名请求（平台行为未定义）。
**修复**：改三向 `1 → 退选 / 2 → 报名 / else → null`（只保留"设为后台冲刺目标"按钮）。
**验证**：`npm run build` 通过。

### 注释轮次前缀标签清理（主人指令"注释代码绝不出现 X-XX（第 N 轮）"彻底落地）
第 31 轮已剥离 140 处，本轮复核仍残留 59 处（web 21 + backend 38，"（第 N 轮）"括号
形态与句首"第 N 轮："形态）——批量正则剥离（`（第\s*\d+\s*轮）`/`\(第\s*\d+\s*轮\)`/
`第\s*\d+\s*轮[：:]`），纯注释零行为影响。**验证**：`go build/vet` + `go test -race ./...`
全绿 + `npm run build` 通过。提交 `35703c0`（web）+ 下述后端 commit（含后端）。

## 后端（B34 系列，3 项确认修复）

### B34-01（MINOR）windowClosedLocked 判据2 仍在亚毫秒窗口内两次独立读取 openTimeNow()
**缺陷**（review34-backend MINOR 1）：B33-02 只收敛了 probeIntervalFor；windowClosedLocked
判据2 内 `!s.openTimeNow().IsZero() && ...After(s.openTimeNow())` 两次独立读取——管理员
PUT open_time="" 清空瞬间（F7-02 合法操作），第一次读非零过 IsZero、第二次读零值使
`After(零值)` 恒 true → "开放时间已过"条件被误判满足，三条件（syncFailStreak≥3 + 时钟失败
+ 开放时间已过）视同关闭。实际影响被 B11-A1 零值守卫与 probeIntervalFor 零值降频吸收
（偏安全方向：挂起而非轰炸），streak≥3 罕见。**修复**：`open := s.openTimeNow()` 取一次复用，
与 B33-02 同策略（注释落盘）。

### B34-02（MINOR）tick 外层 open 快照与 probeIntervalFor 内部快照不一致
**缺陷**（review34-backend MINOR 2）：tick 830 行取的 open 用于 839 行"窗口到点立即探测"
判定；837 行探测间隔判定 probeIntervalFor 内部又自取一次——热改亚毫秒窗口内两值分叉，
立即探测判定与节流间隔基于不同 open。附带同源：probe() 内 979/988 两处 10s 裕量基准
（WindowClosed 主判据 vs EmptyProbeRuns 入账）各调 `openTimeNow().Add(10s)`,同一探测内
可能分叉。**修复**：抽 `probeIntervalForOpen(now, open)` 纯函数，tick 复用已取 open 传入
（原 `probeIntervalFor` 保留为包装）；probe() 内 `open := s.openTimeNow()` 取一次复用两处
裕量判定。

### B34-03（MINOR）spawnChain 失败路径 5 处 AppendLog 落库错误未记录（B33-01 只覆盖成功/退选路径）
**缺陷**（review34-backend MINOR 3）：B33-01 补了成功/退选路径（SaveSuccess/SaveRefused/
DeleteSuccess 业务状态行），但失败路径 5 处 AppendLog（token 失效 / 风控退避 / 实时复核
失效 / 报名失败 / 满员）直接调用不检查返回——日志丢失只影响审计完整性，不破坏重启恢复
契约。**修复**：5 处全部 `if err := ...; err != nil { log.Printf("...落库失败: %v", err) }`，
与 B33-01 日志规范对齐。

## 已核无缺陷清单（review34-backend 10 项 + review34-frontend 复核）

后端：B18-M1 prevOpened 捕获顺序正确（先捕获再覆写，三契约测试 + -time.Hour 与 10s 裕量
对称）；成功后退选再重启状态分叉不成立（RemoveDone 必调 DeleteSuccess → RestoreDone 不含
该课 → rebuild doneHas=false → status=pending → RestoreRefused 落退选文案，契约闭环）；
B23-03 token 失效短路位置正确（链顶 relogging 后其余守卫前，tokenValid 置位决策段持
reloginMu→s.mu，MarkTokenValid 同锁序无半态读）；锁序 s.mu→accounts.mu 单向无反向 +
runtime.mu(RLock) 在 s.mu 内而 Runtime.Update 写锁路径不取 s.mu 无死锁环；删账号竞态防线
memory-first 四路 ClientFor 复核 + spawnChain 链顶/goroutine 内双复核齐全；窗口状态机三
判据（B32-01 判据2 带"开放时间已过" + B26-03/B21-02 双 10s 裕量 + B20-02 入账侧裕量）与
测试对齐；死代码/写而不读（emptyRunsFor/RemoveFull/Decrypt/syncFailedWindow）均在观察项；
登录清理 B24-01 wasShell 在 Login 前取值两测试对齐；平台契约（findElectivesData 空 POST
合法、electivesClassList 字段逐字段吻合真实 select.js、countList 只消费
id/selectedCount/auditedCount）；路由与安全（GET /api 无尾斜杠 SpaHandler 404 已落地、
DELETE 空 body 放行、登录/激活独立限流桶、票据单次防重放）。

前端：防抖 400ms 窗口新改动 + 旧 PUT 在飞无乱序（savingRef+lastJson 串行化）；回显合并全
清空守卫（31-04）；StrictMode 双挂载 unmountedRef（F20-01）；flushTargets 双发 PUT 去重；
手动报名/退选在飞 Set 跟踪（M29-01）；ConfigTab refetch 回填收敛；btn_type 三向契约已按
真实 select.js 对齐。

## 契约一致性核对（对照 legacy/ 真实源码基线 + CLAUDE.md 逆向契约）

- findElectivesData 请求体 form 编码非 JSON ✓（HAR 空 POST + select.js popReq jQuery 表单编码）
- 轮询 `ids=b.join(",")` 逗号字符串 ✓（select.js 原样）；b 数组只在 inDateRange=true 收集 ✓；
  10s 轮询 ✓；1500ms 开窗检查 ✓
- btn_type 1=退选/2=报名、`i.can_select&&postReq(...)` 双守卫 ✓（真实 select.js）
- 窗口关闭形态 code:0 空 publishes 与 findElectivesData 空响应结构吻合 ✓
- 无新发现契约不符；parseElectives 的 `raw.Code == 0 && len==0 → 空快照` 判定与 HAR 结构一致。

## 回归

- 后端：`go build ./... && go vet ./...` 全绿；`go test -race -count=1 ./...` 全量通过
  （9 包 ok，含 B34 三项修复后）。
- 前端：`npm run build`（tsc -b + vite）通过（含 34-01 + 场景 B 自愈链 + 34-02 + 34-03 +
  注释剥离后）。

## 观察项（本轮追加/延续）

- 观察 34-01：IsClassFull 依赖 countList.maxCount 未实证（client.go:694）——平台实际不含该
  字段则 MaxCount 恒 0 → 实时复核永不确证满员 → 开窗中满员反复重试到窗口关闭（幸有
  B18-M1 WindowClosed 守卫兜底）；建议下次实测确认。
- 观察 34-02：handleAdminStats 零值 open 输出 "0001-01-01 00:00:00"（open_time_set=false
  前端可区分，无消费缺陷）。
- 观察 34-03：native ddddocr dumpIfDiff 写 os.TempDir() 固定路径（多实例同跑同目录，单实例
  交付无碍）。
- 观察 34-04：PUT /api/admin/config 空变更先落库后报"没有可应用的有效配置项"（23-11 延续）。
- 观察 34-05：reloginAt/lastSyncFailAt 本地钟 vs 对齐钟（偏差 ~640ms/30s 无实质影响）。
- 历轮观察项全表延续（task_log 无容量上限、死代码族、HTTP 探测单飞、probe per-account 错误
  静默、幽灵课程条目无 UI 提示、sortTightest 对 max_count=0 排序、tsconfig 缺 strict、
  reloginBackoff 死分支、RestoreRefused 注释过时、settings captcha_concurrency 无上限、
  CountEntry maxCount 未实证、Go YearTerm 未消费 gradeName/gradeId 等）。
