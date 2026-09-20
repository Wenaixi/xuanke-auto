# round47 后端审查原始发现

> 审查基线：master @ `f1ea495`（R46 收敛落盘后；`cd backend && git status --short` 仅 5 个未跟踪根级文档，无工作区改动）。
> 范围：backend/ 下全部 Go 源码（main.go、cmd/{probe,logintest,bench}、internal/{api,accounts,config,db,runtime,scheduler,secure,session,store,zhidao}、web/embed.go、browser_*.go），绝对只读模式。
> 判据：项目根 CLAUDE.md《工程决策手册》决策锚 + legacy/website-source 逆向契约 + round39~46 各轮发现与修复报告逐条复核。
> 方法：全包通读 + 竞态/锁序推演 + 上轮观察项逐一核实 + 本轮新视角扫查。`go build ./...`、`go vet ./...` 双通道 exit=0；`go test -race -count=1 ./...`（10 包全量首跑）全绿；`go test -p 2 -race -count=1 ./...`（CI 等效）二次全绿；F46 回归钉子集（scheduler 组合含 TestScheduleIntervalClamped + api 组合含 TestAdminStatsWindowOpenedUsesScheduler）连跑 3 轮全绿。

---

## CRITICAL

（无本轮新增 CRITICAL。identity 复核三族（决策侧 B43-01 / 写回侧 B21-03 / 失效归并 B41-01）在调度器全路径仍闭合；删号内存优先四步序（ResetGateForTest 注释确认测试专用）无新裸露写点。）

---

## MAJOR

（无本轮新增 MAJOR。F46-O1 interval clamp / F46-M1 CI -p 2 均正确闭合，见重点核对节。首跑 scheduler 组合 5 正则连跑出现过一次 FAIL，但随后同组合连跑 3 轮 + 全量 scheduler 单跑 + -p 2 全量均全绿，无业务断言失败成分，判定为测试环境连接 flake 残余（与 MAJOR-46-01 同源、-p 2 下载仍偶发但显著缓解），非代码缺陷，不升级。）

---

## 本轮重点核对（上轮新契约，防回归）—— 全部正确闭合

### 1. F46-O1 interval clamp —— 正确闭合，无假绿
- **位置**：`backend/internal/scheduler/scheduler.go:237-239`（New 内 `if interval <= 0 { interval = 300ms }`）
- **核实**：clamp 在 `&Scheduler{ interval: interval }` 赋值**之前**执行——正 interval 路径完全不受影响；非正值在赋值前已被替换，Start() 内 `time.NewTicker(s.interval)` 永不拿到非正。测试 `TestScheduleIntervalClamped`（scheduler_test.go:3552-3568）真绿不假绿：`New(accts, &fakeStore{}, time.Time{}, 0)` → 断言 `s.interval==300ms` 且 `s.Start()` 不 panic + `t.Cleanup(s.Stop)` 能正常收摊——若 clamp 缺失，NewTicker(0) 直接 panic 该测试必红（红绿翻转真实可证）。生产 main.go:113 传 300ms 不变。

### 2. F46-M1 CI -p 2 —— 语法正确且只需 ci.yml
- **位置**：`.github/workflows/ci.yml:56`（`go test -p 2 -v ./...`）
- **核实**：YAML 注释与命令分隔正确、行长合法、`-p 2` 两个 workflow 是否需要——**release.yml 的 go build 各步骤均无 `go test`**（release.yml 只做多架构交叉编译，无单元测试步骤），所以只有 ci.yml 需要降并行。"两个 workflow 版本是否都需要"的疑问裁决为：**只需 ci.yml**，release.yml 无 go test 无需改动，修改范围正确。

### 3. B45-N2/N3 + B43-01/02/04/05 + B44-01 —— 全部复核正确闭合无回归
- B45-N3 加密失败回归钉：`TestLoginByPasswordEncryptFailLogs`（manager_test.go:240-269）仍真红绿；`Restore` 解密失败日志（manager.go:295-317）对称保留。
- B43-01 maybeRelogin 入口存在性复核（scheduler.go:1204）仍在锁内、位于全部 relogin 族 map 写入前。
- B43-02 实时复核失效分支 `sameClientFor`（1596）前置；B41-01 err 归并三路（风控 1547 / 窗口关闭 1567 / 实时复核满员 1631）成族闭合。
- B43-04 管理员撞名双条件（handler.go:121）+ 撞名学生走教务登录（132）；B43-05 500 语义（888-907）+ failingTargetsStore 无消费（见 OBSERVE-47-02）。

---

## MINOR

（无本轮新增 MINOR。）

---

## OBSERVE

### OBSERVE-47-01：task_log 无行数上限清理（历轮延续，本轮修正一处说辞）
- **位置**：`backend/internal/store/store.go:206-207`（AppendLog 无条件 INSERT）、schema.sql:34-42（task_log 无唯一键/无 TTL）、handleAdminLogs（handler.go:1008-1012 仅钳 limit 下界）
- **核实**：**修正上一轮"ORDER BY id DESC 无索引扫描"的说法不准确**——id 为 INTEGER PRIMARY KEY（rowid 别名），SQLite `ORDER BY id DESC LIMIT N` 直接走主键倒序索引、只扫 N 行，读侧 O(N) 无性能问题；**真正的问题只剩"写无限增长"**：全仓 grep 无任何 `DELETE FROM task_log` 清理路径，黄金期失败重试期（非满员失败每 tick s.store.AppendLog 一行，scheduler.go 共 7 处无保护失败写点）长运行数月后 DB 文件持续膨胀（SQLite WAL 模式下膨胀更明显，且每行 INSERT 都要走 WAL 落盘）。LoadAllLogs 的 limit 上钳 2000（store.go:415）只保护读，不保护容量。历轮观察 o39-05/46-03 同源。
- **裁决**：延续观察。建议 AppendLog 侧 `DELETE FROM task_log WHERE id < (max_id - N)` 周期性收敛（可挂 GatePump 同款后台协程）。

### OBSERVE-47-02：B43-05 的 500 分支仍无真断言（failingTargetsStore 定义无消费点延续）
- **位置**：`backend/internal/api/handler_test.go:810-826`（TestAdminStatsTargetsLoadFailureReturns500 只测正常路径）+ `:828-835`（failingTargetsStore 定义后无任何消费点，grep 全包仅出现于该类定义处）
- **说明**：与 round46 观察 46-05 相同——注释自述"Register 接收 *store.Store 非接口无法注入"，但测试文件内已存在包装类型 `failingTargetsStore`（embed *store.Store 覆写 LoadTargetsForAccount 恒失败），说明注入路径其实可行（newTestDeps 已解耦 Store 引用，只需把 sched 换成持 failingTargetsStore 的实例），只是没接上。B43-05 的 500 语义当前靠实现复查背书。修复代理可顺手把失败分支接成真断言（failingTargetsStore 是现成素材，一行接线）。
- **裁决**：观察级延续。

### OBSERVE-47-03：XUANKE_PORT 非数字/超范围无校验（新视角，失败形态清晰非静默）
- **位置**：`backend/internal/config/config.go:58`（`Port: envOr("XUANKE_PORT", "3091")`）、main.go:160-175（`:addr` 直传 http.Server）
- **说明**：XUANKE_PORT=abc 时 `ListenAndServe` 对 `:abc` 报 `listen tcp: lookup tcp/abc: unknown port` 并以 log.Fatalf 拒绝启动——失败形态明确（启动即报错，绝不带错端口误服务），但报错未提示"端口配置非法"而是裸 listen 错误，运维排障需回读监听层错误。值域（如 1-65535）与数字格式均无校验。属配置错误拒启动的可接受边界，与 XUANKE_DB 路径非法（db.Open 失败 log.Fatalf）同类。
- **裁决**：观察级。

### OBSERVE-47-04：孤儿登录（MAJOR-42-02）延续
- 复核 `Manager.Relogin`（manager.go:166-175）：锁内取指针、锁外 gateWait + ReloginIfNeeded；删号+在途重登并发时平台侧一次孤儿 Login。spawnChain 的 `classFullRealtime`（1578 锁外用旧指针）同构窗口在 `sameClientFor` 归并路径复核后已被封住写回，仅剩平台侧一次无主 doLogin（频率极低、无数据污染）。gateWait 的 Cond.Wait 释放锁等待 Broadcast、GatePump 每 30s 推进窗口，无死锁无消费缺失。
- **裁决**：延续观察。

---

## 对上轮观察项逐一核实（成立升级 / 不成立降级 / 延续裁决）

| 编号 | 上轮结论 | 本轮核实 | 裁决 |
|---|---|---|---|
| OBSERVE-45-01 access_limit_cookie 占位 | 观察 | probe/main.go:24 与 submitLogin（client.go:370-371）同款 `***REMOVED***`；Restore（manager.go:313）用 `"1"`；token 权威通道是 URL 参数。 | 延续观察 |
| OBSERVE-43-01 probe 双槽分叉 | 延续 | 主体写 `["*"]`（1104）、per-account 写 `[acct]`（850）；openTimeForLocked 先 [acct] 后 ["*"] 回退；全校单值契约不实际分叉。 | 延续观察 |
| OBSERVE-43-02 reloginResults cap8 满丢弃 | 延续 | 通道 cap 8（257），非阻塞发送 select default（1274-1277），仅并发重登全成功时弃补探测信号。 | 延续观察 |
| OBSERVE-43-03 classFullRealtime 网络段重复 | 延续 | IsClassFull 恒 false（maxCount 平台未下发，client.go:696-704 注释实证），快照判满类fullInSnapshot 主路径先生效。 | 延续观察 |
| OBSERVE-43-04 tick 无 recover | 延续 | tick/probe/submitAll/spawnChain goroutine 均无 recover；interval clamp 已堵最可达 panic 路径（F46-O1），残余概率极低。 | 延续观察 |
| MINOR-43-02 syncFailedWindow 死字段 | 延续 | 写（382/399）不读，注释载明"留档语义"，判据用 syncFailStreak。 | 延续观察 |
| MINOR-43-03 登录失败无固定延迟 | 延续 | 学生走教务网络往返天然延迟；管理员错误口令分支 Sleep(loginTimingFlat) 保留。 | 延续观察 |
| MINOR-43-04 reloginBackoff 注释 | 延续 | 调用侧先 reloginFail++ 再传，n=1 返 30s、n=2 返 60s；注释一致。 | 延续观察 |
| MAJOR-42-02 孤儿登录 | 延续 | 见 OBSERVE-47-04。 | 延续观察 |
| M40-01 本地钟混用 | 延续 | maybeRelogin 退避（1209/1216/1220）同基内部自洽；≤640ms 对 30s 节流无实质危害。 | 延续观察 |
| m40-02 锁内多取 now | 延续 | submitAll 1437 锁内取一次 nowAlignedLocked。 | 延续观察 |
| o40-01~04 / m39-02 / o39-02~05 | 延续 | 未激活不发会话 / reloginResults 满丢弃 / columnExists 全常量 / interval≤0 已 clamp（F46-O1 闭合）/ 快照 TTL 读侧本地钟 / submitAll 快照竞态 / vision key 空串无法清空 / NAT 合并登录限流 / tick 无 recover / task_log 无清理（47-01）。 | 全部延续观察 |

---

## 本轮新视角扫查结论

- **acctData/acctDataAt 快照内存：无只增不减**——ProbeForAccount 每次探测整体覆写 `acctData[acct]`（859，旧 ElectivesData 指针被替换即 GC 释放）；PurgeAccount 全量 delete（509-510）；窗口级 old 帧引用只有 `ElectivesSnapshotFor` 读返回，不再持有。每账号常驻 1 帧（82 门课约百 KB 级），长运行内存有界。
- **handleState/handleElectives 序列化**：数据量级小（课程百门级），每请求新建 encoder 无缓冲复用需求；StateForAccount 的 st.Courses 从共享 state.Courses 拷贝（723-728）量级小；无早退 leak。
- **accounts.order 内存碎片**：Remove（manager.go:129-134）原地 append 删除 O(n) memmove，账号数量上限百级无实际碎片；AnyClient 空 order 返回 (nil,false)（141-144）调用方（probe/maybeSyncClock/ProbeNow 均判 ok 才用）边界正确。
- **store.DeleteAccount 级联**：delete credentials/accounts/targets/success/refused/activations 六表全清（store.go:392-408）；**仅 task_log 保留该账号行**——注释载明"日志保留审计用途"，属刻意设计非孤儿行。级联完整。
- **db WAL/busy_timeout**：SetMaxOpenConns(1)（db.go:23）串行化单写者，busy_timeout(5000) 实际极少触发（无并发连接竞争）；journal_mode(WAL) 开启磁盘吞吐优。无问题。
- **config env 解析**：XUANKE_PORT 非数字失败形态清晰（见 47-03）；XUANKE_DB 路径非法 db.Open log.Fatalf；XUANKE_MASTER_KEY 长度严格校验（secure/crypto.go:16-21）；SF_KEY 未配置仅警告不拒启动（main.go:30-32）。整体"配置错误要么拒启动要么可诊断"，无静默误行为。

---

## 已核对无问题的重点区域（本轮逐项复核）

- **编译/静态检查/测试**：`go build ./...`、`go vet ./...` 双通道 exit=0；`go test -race -count=1 ./...`全绿 + `go test -p 2 -race -count=1 ./...`（CI 等效）二次全绿；F46 钉子集（TestScheduleIntervalClamped / TestWindowOpenSubmitsWithoutProbeReset / TestAdminStatsWindowOpenedUsesScheduler / TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime + api 组合）连跑 3 轮全绿。
- **identity 复核族全闭合**：spawnChain 六分支（成功 1517 / 失效 1485 / 风控 1547 / 窗口关闭 1567 / 实时复核满员 1631 / 实时复核失效 1596）全部 sameClientFor 前置；maybeRelogin 入口决策侧（1204）+ 重登 goroutine 写回侧（1250）闭合；MarkDone/RemoveDone 写回侧（1912/1980）复核。
- **WindowClosed 三判据单源** windowClosedLocked()（914-935）+ StateForAccount 共用（708）+ handleAdminStats window_closed（935）同源；测试 TestAdminStatsWindowOpenedUsesScheduler 断言 window_closed 字段存在且与调度器同源。
- **openTime 识别槽三硬契约**：识别槽保留（空快照不删）；展示层过期判定独立（719 open_time_known）；写入持锁（850/1104）。**识别槽内存有界**：PerAccount 删除清槽（511）+ PurgeAccount 全清只给现存账号用。
- **B41-02 零值守卫**：`open.IsZero() && !opened`（1007）语义正确；TestSubmitSuspendedWhenOpenTimeCleared / TestSubmitAllowedWhenWindowOpenedWithZeroOpenTime 双绿。
- **探测节流三件套**：lastProbe 只归 probe()/ProbeNow；probing 单飞锁内置位-检查（1045-1051）；probeSem cap 4（1069-1071）；tick 首探豁免（985）。
- **多账号年级隔离**：probe() 每账号独立 goroutine + ProbeForAccount 专属帧；ElectivesSnapshotFor 目标账号过期专属帧 → (nil,false) 真刷新绝不回退全局帧（782-810）。
- **doLogin 闸门全收口**：gateTryAcquire（manager.go:223-235 非阻塞）+ gateWait（阻塞）共享 gateMu/gateUsed；LoginByPassword 与 Relogin 无旁路；GatePump 每 30s 推进广播。
- **鉴权与数据安全**：requireAuth 401 / requireAdminSession 403 / recoverMiddleware 500 / 登录激活限流 429 一族真实状态码；SpaHandler /api 前缀 404（embed.go:28-31）；XFF 仅回环 + XUANKE_TRUSTED_PROXY=on 信任最右非空。
- **SQL**：全参数化绑定；columnExists/Migrate 全常量拼接；migrateAddPublishMeta 增量幂等 + refuseLegacy 缺列清单对应剔除；SQLite 单连接串行 + busy_timeout(5000) + WAL。
- **登录链路（zhidao）**：RSA-PKCS1v1.5 1024 位公共钥硬编码；uniqueDeviceID 复刻逐字段一致；captcha 全局信号量动态热收敛；识别失败刷新验证码重试收敛（≤3）；doLogin 闸门在 manager 层收口。`Login` 内 `sess` 独立 http.Client（不共享 token 请求连接池），每次 attempt 独立 jar 会话——无共享连接池污染。
- **cmd 工具**：probe 读 XUANKE_PROBE_TOKEN（无硬编码 token）；logintest 跟随识别引擎 + wait 默认 40s 符合平台限流；bench -token 可选注入（无 token 标注测 401 拒绝路径）。均只读、无副作用、不写库。

---

## 结论

- **MAJOR 0 / MINOR 0 / OBSERVE 4（2 新 + 2 延续）**，共 4 条。
- 最重 3 条（按影响排序）：
  1. **OBSERVE-47-01（task_log 无行数上限清理）**——黄金期失败重试每 tick 写一行，长运行 DB 持续膨胀且无清理路径；本轮修正上轮"读侧无索引"说辞后问题收敛为纯写侧容量。历轮观察延续。
  2. **OBSERVE-47-02（B43-05 500 分支无真断言）**——failingTargetsStore 现成却无消费点，接线一行即可补齐红绿钉。
  3. **OBSERVE-47-03（XUANKE_PORT 无校验）**——失败形态清晰（启动拒），非静默误行为，观察留档。
- 上轮观察项 14 条全部复核：均延续观察，无升级。F46-O1/F46-M1 两条上轮新契约正确闭合（F46-O1 clamp 赋值前生效且测试真红绿；F46-M1 只需 ci.yml、release.yml 无 go test 无需改）。
- **首次 scheduler 组合 5 正则连跑出现一次单包 FAIL，随后 ·同组合 3 轮 + 全量 scheduler 单跑 + -p 2 全量二次全绿、丢包与信号量行为无业务断言失败——判定为 MAJOR-46-01 同源连接 flake 在 -p 2 下载偶发残余，非代码缺陷，不升级。**
- 残余风险集中在三条延续观察：孤儿登录平台侧副作用（MAJOR-42-02）、tick 无 recover（43-04，interval 路径已由 F46-O1 clamp 封堵）、task_log 无清理（47-01）。