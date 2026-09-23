# R108 后端只读审查报告

审查对象：xuanke-auto HEAD `838a5b4`（R107 收尾轮）。backend/ 自 B101-01（20c5882）起连续六轮零产品代码改动（`git log 20c5882..HEAD -- backend/` 为空），工作树洁净。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件。

审查方式：Read / Grep / Bash 只读命令（go vet / go build 定向实证 + 逐点走读）。核心走读范围：身份防线矩阵第二十三轮（全家福写点 grep 实证 + 偶发写点专项）、O105-01 抖动基线第二十三轮夹具走读、O107-01 误判修正（本轮核心）、O106-01 延续、既往观察项延续（O105-02/O92-02/M87-01/O90-01 + 契约 20/17 强扫）、新契约角度 a（调度器 tick 时序亚毫秒竞态复查）。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## 新发现表

| 编号 | 级别 | 结论 |
|---|---|---|
| B108-01 | OBSERVE 修正（知识位） | **O107-01 误判纠正**：R107 断言"session 无 Close() 接线、全程序任何点无 Close 调用"**不成立**——`main.go:163` `defer sessions.Close()` 自第 3 轮（5cb04b0，2026-09-14）起就在位（git blame 实证），Close 实现（store.go:90-98）带 `sweeperClose.Do` 防重入 + `<-sweeperDone` 等待协程退出，完整收口。**O107-01 转入闭合**（核心疑虑"无接线"被证伪），残余面为零：sweeperLoop 由 main 退出路径正常终止。教训同 R106 对 O106-01：审查文本宣称必须经 grep/blame 实证再下结论。 |
| B108-02 | OBSERVE（延续） | O106-01 票据内存态：sweepExpired 双 map 清扫在案（store.go:73-87）+ ConsumeTicket 过期即删（:128-131）双重回收；残余仅"重启丢票 + 5 分钟清扫间隔间歇留存"，量级与概率双低，维持低风险观察无提级依据。 |
| B108-03 | OBSERVE（延续） | O105-02 删除保护撞名（handler.go:254/992 区单判据）、O92-02 logintest 引擎判定源分叉、M87-01 窗口注释兜底——均维持观察，无新依据提级。 |

## 必查项逐条结论

### 1. 身份防线矩阵（第二十三轮）——通过

`sameClientFor` 7 调用点与 R105/R106/R107 对比**零漂移**：`:204`（定义）/`:850`（ProbeForAccount 回写段）/`:1489`（失效分支）/`:1521`（成功分支）/`:1551`（风控退避）/`:1571`（窗口关闭）/`:1600`（实时复核回锁统一复核）/`:1635`（确证满员分支）。

**写点全家福 grep 实证**（本轮回合重跑，与 R107 清单逐一对位）：

| 写点 | 行号 | 防线 |
|---|---|---|
| `openTimeDetected[acct]=` | :856（锁内 + sameClientFor :850 后） | 探测回写段身份复核 |
| `acctData[acct]=` + `acctDataAt[acct]=` | :863/:864（锁内 + 身份复核后） | 同上 |
| `reloginAt[acct]=` / `relogging[acct]=` / `tokenValid[acct]=` | :1231/:1236/:1241（决策侧）/ :1261/:1262（写回侧） | maybeRelogin 决策侧 ClientFor 复核（:1209-1211 在位）+ 写回侧复核（:1254-1258，已删账号整分支静默放弃） |
| `inflight[acct][id]=true` | :1471（spawnChain）/ :1905 区（TryAcquireSubmit） | 同步 delete 在位（:1490/:1501/:1513/:1950/:2002-2003/:510 区） |
| `done[acct][id]=true` | :1529（spawnChain 成功 + sameClientFor :1521 后）/ :1934（MarkDone + ClientFor 存在性 :1927） | 全在位 |
| `refused[acct][id]=true` | :2011（RemoveDone + ClientFor :1995）；MarkDone :1938 delete + :1945 库行同步清 | 全在位 |
| `full[acct][id]=true` | :1767（markFullLocked，仅 sameClientFor 后 :1575/:1639 两条调用续） | 单入口 |
| `rateLimited[acct][class]=` | :1714（仅 spawnChain 风控分支经 sameClientFor :1555 后） | 单入口 |

**偶发写点专项（第二十三轮重查）**：① `MarkTokenValid`（:1306-1316）纯 delete 清失效标记，handler 401 拦截保证不达已删账号；② `TryAcquireSubmit`（:1896）api 手动独占入口、无网络往返后写回；③ `releaseFullIfFreedLocked`（:1781）由 tick 持锁调用、写自己账号解封语义；④ `submitAll`（:1342-1380）链顶 ClientFor 过滤 + spawnChain 链顶二次判（:1388）+ 取 client 后再判（:1409）双保险。**无新裸露写点、无新增偶发写点**。

**手动四路 accountExists**：:245（课程读）/ :295（手动报名）/ :373（手动退选）/ :537（目标写/状态读）在位。

### 2. O105-01 抖动基线（第二十三轮）——通过

socketPreheat（client_test.go:25-30）+ TestMain 包级预热（:39-40）+ readyProbe 宽栅栏（:88，200ms×10 + 2s 超时）布局与 R107 描述逐字符一致，零改动。api 包夹具同款（handler_test.go:139 / :185，预创建套接字 + readyProbe 5 次全败上抛）。本轮 `go build ./...` 与 `go vet` 五包（scheduler/api/session/zhidao/db）全干净（VET_EXIT=0），未跑全量 race（按主控约定由主控跑）。**未发现新时序脆弱点，口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（八轮 30 跑 4 单 FAIL ~13%）**。

### 3. 观察项与知识位盯守——通过

- **O107-01**：见 B108-01——**误判修正 + 闭合**（Close 接线在位）。
- **O106-01**：见 B108-02——维持低风险观察。
- **B101-01（sanitizeError）**：doRequest 唯一 token 通道（:422 `?idToken=`）、脱敏应用点零漂移；B102-01（http 直调点 URL 静态）在位。
- **B103-01 / B104-01 / B105-01 / B105-02 / B105-03 / B106-01**：实时复核活化条件、手动路径无在飞窗口、展示层 open_time_set 语义、TokenValidFor 半态、probeSem cap 4、三引擎 withConcurrency——各就其位无退化。

### 4. 既往观察项延续——通过

- **契约 20 全仓强扫**：`grep (第 N 轮|R/O/B/M/F[0-9]{2}-[0-9]{2})` 生产代码排除 `_test` 后仅命中合法锚点：scheduler.go:843（M88-01 身份防线引用）、client.go:577 区（B101-01 引用）、quit_shared.go:7 与 tray_windows.go:53（M86-01 缺陷形态引用）。**零违规轮次标签**。注意：本轮扫描含 `backend/main.go`（R71 已剥离）与 `backend/*.go` 托盘文件，仍干净。
- **契约 17 零吞错**：MarkDone（:1945/:1973/:1976）/ RemoveDone（:2020/:2026/:2029）/ spawnChain（:1504/:1532/:1535/:1558 等）全部 `if err != nil { log.Printf }`，无 `_ =` 落库点。
- **O92-02 / M87-01 / O90-01**：均维持观察无新证据（M87-01 的 windowClosedLocked 三判据单源 :918 在位）。

### 5. 新契约角度 a（调度器 tick 时序亚毫秒竞态复查）——通过

逐点走读 `tick()`（scheduler.go:973-1036）判定顺序：

1. **`now := s.nowAligned()`（:974）+ `open := s.openTimeForLocked("")`（:977）**：tick 开头取单次快照，`probeIntervalForOpen(now, open)`（:987）与 `submitIntervalFor(now, open)`（:1031）复用同一 open——注释明确"热改亚毫秒窗口内立即探测判定与节流间隔若各自取 open，可能读到新旧两个不同值"，与契约 2「单快照复用」完全一致。
2. **探测先于提交判定**（:992-994 然后 :996-998 读 `opened`）：opened 是"本次 tick 探测后"的最新值，顺序正确无竞态。
3. **提交守卫四层**：`open.IsZero() && !opened` 挂起（:1011）→ `!opened && !now.After(open)` 挂起（:1014）→ `WindowClosed()` 挂起（:1024）→ 提交节流闸门（:1032）。B41-02 语义（WindowOpened 让位零值守卫）与 B11-A1 防轰炸（未开窗挂起）在位。
4. **开窗到点首次 tick 立即探测**（:989 `now.After(open) && last.Before(open.Add(-time.Second))`）：对齐时钟判定，无亚毫秒翻转面。

唯一理论差异点：`tick` 的 open（:977）与 `WindowClosed()` 内部 windowClosedLocked 各自取的 open 在热改毫秒窗内可能不同值——但两者语义均为"开放时间为固定值"，毫秒级新值不会造成放行/挂起翻转实质风险，且 windowClosedLocked 内部已按契约 2 单快照复用。**无新竞态，tick 时序符合契约 1/2/32**。

## 验证表

| 验证 | 结果 |
|---|---|
| `git log 20c5882..HEAD -- backend/` | 空（backend 连续六轮零产品改动） |
| `git blame main.go:163` | `5cb04b0`（第 3 轮）`defer sessions.Close()` 在位 |
| `go build ./...` | BUILD_EXIT=0 |
| `go vet` 五包 | VET_EXIT=0（O90-01 CRLF 干净） |
| 写点全家福 grep | 8 类写点与 R107 清单逐一对位、无新写点 |
| sameClientFor 调用点 | 7 处零漂移 |
| 契约 20 生产代码强扫 | 零违规，仅 M88-01/B101-01/M86-01 合法锚点 |

## 已核无缺陷清单（走读实证）

- requireAuth / requireAdminSession 双中间件会话鉴权：`Authorization: Bearer` + `X-Auth-Token` 双通道，401/403 均写真实 HTTP 状态码（handler.go:1097/:606），B40-01 家族核对延续在位。
- handleActivate 先 ConsumeTicket 再 ConsumeActivationCode（handler.go:199→203），票据单次防重放契约在位。
- issueSession 手动登录成功调 MarkTokenValid（handler.go:226）——tokenValid 唯一自动清零路径外的合法恢复路径。
- Register 全部端点鉴权对应：健康/登录/激活免认证；激活码管理四端点 requireAdminSession；业务五端点 + logout requireAuth；`/api/` 前缀兜底 404 不落 SPA（router.go:194-199）。**无遗漏端点**。
- maybeRelogin 决策侧复核在 maybeRelogin 入口、任何 map 写入前（scheduler.go:1209-1211）；写回侧复核先清 relogging 再判已删（:1247-1258）——B43-01 双闭合无退化。
