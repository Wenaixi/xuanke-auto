# R109 后端只读审查报告

审查对象：xuanke-auto HEAD `47fe87e`（R108 收尾轮）。backend/ 自 B101-01（20c5882）起连续七轮零产品代码改动（`git log 20c5882..HEAD -- backend/` 精确统计 COUNT=0），最近一次后端代码改动为 `ca08c46d`（B88-01 ProbeForAccount 回写段身份复核）。工作树洁净。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件。

审查方式：Read / Grep / Bash 只读命令（go build / go vet / 定向 go test -race 实证 + 逐点走读 + git blame/log 实证）。核心走读范围：身份防线矩阵第二十四轮（全家福写点 grep 实证 + 偶发写点专项）、O105-01 抖动基线第二十四轮夹具走读、观察项与知识位盯守（O105-02/B105-01/O106-01/B101-01~B106-01）、既往观察项延续（O92-02/M87-01/O90-01 + 契约 20/17 强扫）、新契约角度 a（HTTP 路由与鉴权全量核对）。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE（延续观察 + 新发现）

### B109-01（OBSERVE，轻量证据补强）：`RemoveFull` 仍为全仓零调用死方法，且 CLI 层未消费

**位置**：`internal/scheduler/scheduler.go:2037-2049`。

**事实确认（grep 实证）**：全仓（含 `_test.go` 与 `cmd/`）`.RemoveFull` / `RemoveFull(` 命中仅定义处一处，调用方为零——B17-02 观察自第 17 轮起已维持 90+ 轮。手动退选解封（RemoveDone）与快照解封（releaseFullIfFreedLocked :1781）两条真实路径均不经过它。**无新证据、无提级依据**，仅按历轮口径确认死方法零漂移延续（文档分叉无行为危害，删除需动契约面，按"精准修改"观察不修）。

### B109-02（OBSERVE，延续）：O106-01 票据内存态残余面与 O105-02 撞名单判据

- **O106-01**：sweepExpired 双 map 清扫（store.go:73-87）+ ConsumeTicket 过期即删（:128-131）+ handleActivate 先 ConsumeTicket 再 ConsumeActivationCode（handler.go:199→203）三重回收在案；残余仅"重启丢票 + 5 分钟清扫间隔间歇留存"，量级与概率双低，维持低风险观察。
- **O105-02**：handleAdminDeleteAccount 删除保护（handler.go:992 `acct == "" || acct != req.Account || d.IsAdminAccountName(acct)`）仍为单判据，维持观察无新依据。

## 必查项逐条结论

### 1. 身份防线矩阵（第二十四轮）——通过

**新代码写点核查前置**：backend/ 连续七轮零产品代码改动（`git log 20c5882..HEAD -- backend/` COUNT=0，git log 实证），因此"新代码引入的写点"实质为空集——本轮聚焦既有写点全家福与偶发写点的再次核位。

**sameClientFor 调用点**：`:204`（定义）/`:850`（ProbeForAccount 回写段）/`:1489`（失效分支）/`:1521`（成功分支）/`:1551`（风控退避）/`:1571`（窗口关闭）/`:1600`（实时复核回锁统一复核）/`:1635`（确证满员分支）——与 R105~R108 清单**零漂移**。git blame 确认回写段防线引入于 `ca08c46d`（2026-09-23 05:42），B88-01 提交。

**写点全家福 grep 实证**（本轮回合重跑，与 R107 V2 清单逐一对位）：

| 写点 | 行号 | 防线 |
|---|---|---|
| `openTimeDetected[acct]=` | :856（锁内 + sameClientFor :850 后） | 探测回写段身份复核 |
| `openTimeDetected["*"]=` | :1108（全局载体，无身份概念） | 锁内写：1106-1110 |
| `acctData[acct]=` + `acctDataAt[acct]=` | :863/:864（锁内 + 身份复核后） | 同上 |
| `tokenValid[acct]=true/false` | :1241（决策侧，ClientFor 复核 :1208-1210 后）/ :1262（写回侧，复核 :1254-1258 后） | 双闭合 |
| `reloginAt[acct]=` / `reloginFail[acct]++` / `relogging[acct]=true` | :1231/:1232-1233/:1236（决策侧） | 同上；PurgeAccount :506-510 全清 |
| `inflight[acct][id]=true` | :1471（spawnChain）/ :1905（TryAcquireSubmit） | 同步 delete :1490/:1501/:1513/:1910（sync.Once）/:1950/:2002/:510(Purge) |
| `done[acct][id]=true` | :615（RestoreDone）/ :1529（spawnChain 成功 + sameClientFor :1521 后）/ :1934（MarkDone + ClientFor :1927） | 全在位 |
| `refused[acct][id]=true` | :634（RestoreRefused）/ :2011（RemoveDone + ClientFor :1995）；MarkDone :1938 delete + DeleteRefusedClass 库行 :1945 | 全在位 |
| `full[acct][class]=true` | :1767（markFullLocked，仅 :1575/:1639 两条 sameClientFor 后调用续） | 单入口 |
| `rateLimited[acct][class]=` | :1714（仅 :1555 风控分支 sameClientFor 后） | 单入口 |
| `state.Courses[idx].Status/Result` | setStateLocked 统一入口 :1846-1851 + 内联写 :1473/1803/1961/2014/2045 | 全部处于持锁 + 身份防线后 |

**偶发写点专项（第二十四轮重查）**：① `MarkTokenValid`（:1306-1319）纯 delete 清失效标记，handler 401 拦截保证不达已删账号，reloginMu→s.mu 锁序对齐；② `TryAcquireSubmit`（:1896-1914）api 手动独占入口、无网络往返后写回；③ `releaseFullIfFreedLocked`（:1781-1809）由 spawnChain 持锁调用、写"该账号自己 full 解封"语义；④ `submitAll`（:1342-1380）链顶 ClientFor 过滤 + spawnChain 链顶二次判（:1388）+ 取 client 后再判（:1409）双保险。**无新裸露写点、无新增偶发写点**。

**手动四路 accountExists**：:245（课程读）/ :295（手动报名）/ :373（手动退选）/ :537（状态读）在位；handleSetTargets 内联凭据校验（:461-477）同源。target 写为普通会话时无透传路径（allowAccountOverride=false）。

**第七分支统一复核**：实时复核回锁后统一 sameClientFor（:1600）先于三路（失效/确证满员/未现满员），B43-01 决策侧复核（maybeRelogin 入口 :1208-1210）+ B43-02 第六分支 + B41-01 全分支覆盖均无退化。回归钉：probe_identity_test（M88-01 三钉）+ scheduler 身份防线族 + window 守卫族定向 race 全绿（5.32s / 3.69s）。

### 2. O105-01 抖动基线（第二十四轮）——通过

socketPreheat（client_test.go:25-30）+ TestMain 包级预热（:39-40）+ readyProbe 宽栅栏（:88-110，200ms×10 + 2s 超时）布局与 R107/R108 描述逐字符一致，零改动。api 包夹具同款（handler_test.go 预创建套接字 + readyProbe）。本轮 `go build ./...` 全绿（BUILD_EXIT=0）、`go vet` 五包全干净（VET_EXIT=0）、scheduler/api/zhidao/session/db 五包定向 -race 全绿（总耗时 ~21s，零 flake）。**未发现新时序脆弱点**，口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（八轮 30 跑 4 单 FAIL ~13%），全量 race 按约定由主控跑。

### 3. 观察项与知识位盯守——通过

- **O105-02 删除保护撞名**（handler.go:992）：见 B109-02，维持观察。
- **B105-01 展示层**：handleAdminStats 的 `open_time_set`（handler.go:964，识别槽非零即 true，含过期值）与学生端 open_time_known 依 now 过期判定（scheduler.go:714-716）不对称维持，走读无新依据提级。注意 open_time_str（:898-902）对过期值**照常输出日期**——与决策锚 1「识别过期只影响展示层、识别值绝不截断」一致，管理员看"上次识别的开放时间"语义成立。
- **O106-01 票据**：见 B109-02。
- **B101-01（sanitizeError）**：定义 :581-593 无漂移（剥 *url.Error → Op+底层、Unwrap 下沉）；doRequest :450 唯一应用点；唯一 token URL 通道 = doRequest :422 `?idToken=`。脱敏族定向 race 全绿。
- **B102-01 / B103-01 / B104-01 / B105-02 / B105-03 / B106-01**：http 直调点 URL 静态、实时复核活化条件、手动路径无在飞窗口、TokenValidFor 半态、probeSem cap 4、三引擎 withConcurrency（captcha.go:34-107，Mutex+Cond 动态限流器）——各就其位无退化。
- **gateWait/gateTryAcquire 家族（B42-01 知识位）**：全局 doLogin 频率闸门唯一共享 gateUsed 计数；Relogin（:166）走 gateWait 阻塞排队，LoginByPassword（:243-246）走 gateTryAcquire 非阻塞准入——两条路径严格共享每分钟 2 次预算。GatePump（:68-77）由 main.go:157 每 30s 驱动窗口翻页广播。测试专用 ResetGateForTest 仅 api 夹具使用，正式代码零调用（grep 实证）。

### 4. 既往观察项延续——通过

- **契约 20 全仓强扫**：grep `第 ?[0-9]{1,2} ?轮`（生产代码）零命中；`(R|O|B|M|F)[0-9]{2}-[0-9]{2}` 命中仅合法锚点（scheduler.go:843 M88-01、client.go:574-577 B101-01、main.go:189/tray_windows.go:53/tray_quit_test.go M86-01）。**零违规轮次标签**。注意本轮扫描含 backend/main.go 与托盘文件，仍干净。
- **契约 17 零吞错**：全仓 `_ =` 落库点扫描（grep `_ = ` + `s.store.` / `d.Store.` 交叉核对）——MarkDone（:1945/:1973/:1976）/RemoveDone（:2020/:2026/:2029）/spawnChain（:1504/:1532/:1535/:1558/:1614/:1662/:1770）/maybeRelogin 写回（:1269）/SetTargetsForAccount（:476/:484）全部 `if err != nil { log.Printf }`；`_ =` 仅命中非落库面（Prewarm goroutine :307、ProbeForAccount 探测结果 :1075、io.Copy 读盘、config os.Setenv/WriteFile、MarkDone/RemoveDone 返回值——后者是"状态写回，错误已内记日志"的设计返回，非吞错）。api 层 AppendLog 各点同型。**零吞错落库点**。
- **O92-02 logintest**：cmd/logintest/main.go:73 初始化识别并发 1，引擎判定源分叉维持观察无新证据。
- **M87-01 窗口**：windowClosedLocked 三判据单源（:918-939）在位，open 单快照复用（:924）在位。
- **O90-01 CRLF**：go vet 五包全干净（VET_EXIT=0）。

### 5. 新契约角度 a（HTTP 路由与鉴权全量核对）——通过

**全端点-中间件对应核对**（router.go:97-199 逐条 + handler 实现走读）：

| 端点 | 鉴权 | JSON 门 | 核对结论 |
|---|---|---|---|
| GET /api/health | 免认证 | 无（只读） | 仅探活，无副作用 |
| POST /api/login | 免认证（限流 429 + 双条件管理员判定） | 内联 CSRF 门 :101 | 限流桶独立（loginLim） |
| POST /api/activate | 免认证（限流 429 + 票据校验） | 内联 CSRF 门 :115 | 票据单次防重放在位 |
| GET/POST/DELETE /api/admin/codes | requireAdminSession（Bearer/X-Auth-Token 双通道） | POST 才 requireJSONBody（GET/DELETE 放行空 body，标准 REST 语义） | 无缺口 |
| GET/PUT /api/admin/config | requireAdminSession | PUT 才 requireJSONBody | 无缺口 |
| GET /api/admin/stats | requireAdminSession | 无 | 无缺口 |
| GET /api/admin/accounts / DELETE /api/admin/accounts | requireAdminSession | DELETE 放行空 body | 无缺口 |
| GET /api/admin/logs | requireAdminSession | 无 | 无缺口 |
| GET /api/electives | requireAuth | 无（只读） | 管理员穿透走 accountExists |
| POST /api/electives/select / exit | requireAuth | requireJSONBody | 手动两路 accountExists 在位 |
| PUT /api/targets | requireAuth | requireJSONBody | 内联凭据校验 |
| GET /api/accounts / state / logs | requireAuth | 无 | 状态读 accountExists 在位 |
| POST /api/logout | requireAuth | 无（空 body） | 服务端吊销 |
| `/api/` 兜底 | — | — | 显式 404（writeJSONStatus :198） |
| `/api`（无斜杠）| — | — | SpaHandler :28 拒 404（双保险） |

**核对要点**：
- 鉴权四通道全覆盖：requireAuth 401（handler.go:1097）+ requireAdminSession 403（:606）+ CSRF 403（jsonContentType 拒绝，router.go:72）+ 限流 429（登录/激活两处）——B40-01 家族核对延续在位。
- `writeJSONStatus` 使用面扫描：仅基础设施错误路径（401/403/429/404/500 + 配置落库失败 500），业务路径全走 writeJSON（body code 契约，HTTP 恒 200）——B39-02 不破坏前端契约。
- `allowAccountOverride` 判据 = `Sessions.IsAdminToken`（:1067），普通会话 ?account= 无效；管理员会话透传全部过 accountExists 凭据表判据。
- 登录管理员双条件（B43-04）：`:121` `req.Account == adminName && ConstantTimeCompare(...)==1` 在位，撞名学生放行教务登录；错误分支 loginTimingFlat 时延拉平（:124/:137）在位。
- 限流桶惰性 GC（:1143-1149，>1024 桶 + 1 分钟间隔）防 OOM；clientIP 可信反代开关（:1174-1187，XUANKE_TRUSTED_PROXY=on 且回环才信 XFF 最右非空）默认关闭。
- **无遗漏端点、无多余暴露**：全部副作用端点均有 requireJSONBody 或内联 CSRF 门，全部只读端点均有 requireAuth/requireAdminSession；无任何端点裸暴露写操作。

## 验证表

| 验证 | 结果 |
|---|---|
| `git log 20c5882..HEAD -- backend/` | COUNT=0（backend 连续七轮零产品改动） |
| `git blame scheduler.go:850` | `ca08c46d`（B88-01 提交）sameClientFor 回写防线在位 |
| `go build ./...` | BUILD_EXIT=0 |
| `go vet` 五包 | VET_EXIT=0（O90-01 CRLF 干净） |
| scheduler 定向 race（身份防线族+窗口守卫+时钟兜底） | 全绿（5.32s + 3.69s） |
| api 定向 race（手动四路+删除保护+激活族+CSRF） | 全绿（2.97s + 4.69s） |
| zhidao 脱敏族 / session / db 定向 race | 全绿（1.91s / 1.92s / 4.29s） |
| 写点全家福 grep | 12 类写点与 R107 V2 清单逐一对位、无新写点 |
| sameClientFor 调用点 | 7 处零漂移 |
| 契约 20 生产代码强扫 | 零违规，仅 M88-01/B101-01/M86-01 合法锚点 |
| 契约 17 零吞错 | 全仓 `_ =` 落库点扫描零违规 |
| 新角度 a（路由鉴权全量） | 19 端点逐一对应，无遗漏/无多余暴露 |

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第二十四轮闭合**：backend 零改动前提下全家福 12 类写点 + 偶发写点专项 + sameClientFor 7 处零漂移，无新裸露。 | 通过（走读 + grep + 定向 race） |
| **O105-01 抖动基线第二十四轮**：夹具逐字符零改动，go build/vet 全净，五包定向 race 全绿零 flake。 | 通过 |
| **B101-01~B106-01 知识位**：脱敏链、http 直调、实时复核活化、手动无在飞窗口、TokenValidFor、probeSem、三引擎限流——全部在位无退化；gateWait/gateTryAcquire 家族核对通过。 | 通过 |
| **O105-02 / B105-01 / O106-01**：维持历轮观察，无新依据提级。 | 维持 |
| **O92-02 / M87-01 / O90-01 + 契约 20/17**：维持历轮结论；契约强扫零违规。 | 通过 |
| **新契约角度 a（HTTP 路由与鉴权全量）**：19 端点逐一核对，鉴权/CSRF/限流/404 全覆盖，无遗漏端点无多余暴露。 | 通过 |

## 结论

1. **身份防线矩阵第二十四轮闭合**：backend/ 连续七轮零产品代码改动前提下，写点全家福（12 类）与偶发写点专项全部与 R107 V2 清单逐一对位，sameClientFor 7 调用点零漂移，无新裸露写点、无新增偶发写点。
2. **O105-01 抖动基线第二十四轮**：夹具零改动，go build/vet 全净，五包定向 -race 全绿零 flake，无新时序脆弱点，口径维持。
3. **观察项与知识位**：O105-02/B105-01/O106-01 维持观察无新依据；B101-01~B106-01 + B42-01 闸门家族全部在位无退化。
4. **既往观察项延续**：O92-02/M87-01/O90-01 维持；契约 20 全仓强扫零违规（仅合法锚点）；契约 17 零吞错复核通过。
5. **新契约角度 a（HTTP 路由与鉴权全量）**：19 端点逐一核对——鉴权（requireAuth/requireAdminSession 双中间件四通道）、CSRF（JSON 门全覆盖）、限流（登录/激活独立桶 429）、404（/api 兜底双保险）全部正确，无遗漏端点、无多余暴露。
6. **新发现仅 B109-01（OBSERVE，RemoveFull 死方法延续）+ B109-02（O106-01/O105-02 延续）**——均无提级依据；无 MINOR 以上新缺陷。

工作树后端文件洁净。
