# R106 后端只读审查报告

审查对象：xuanke-auto HEAD `f5fdb36`（OBSERVE-106-02 前端日志区修复，web/src/routes/Dashboard.tsx；backend/ 自 R103 起连续四轮零产品代码改动——最近一次后端产品代码仍为 `20c5882` B101-01）。工作树下无后端文件脏（仅本报告文件 + 并发前端报告文件为新增）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件。

审查方式：Read / Grep / Bash 只读命令（定向 go test -race / go vet / 逐点走读）。核心走读范围：身份防线矩阵第二十一轮（全家福写点 grep 实证 + spawnChain 第七分支 + maybeRelogin 双侧 + PurgeAccount + api 手动四路 + 写点聚类复核）、O105-01 抖动基线第二十一轮夹具走读、B101-01/B102-01/B103-01/B104-01/B105-01 持续盯守、既往观察项延续（O105-02/O92-02/M87-01/O90-01 + 契约 20 强扫）、新契约角度 c（激活码生命周期闭环走读）+ 登录全链路状态机二查（对照 R96 归档）。

## CRITICAL

无。

## MAJOR

无。

## MINOR

无。

## OBSERVE（延续观察 + 新发现）

### O106-01（新发现）：激活票据依赖内存态、30 分钟到期只对扫表生效——重启丢票据、到期无钟表自愈（低风险观察）

**位置**：`internal/session/store.go:102-108/:118-138`（CreateTicket/ConsumeTicket 均只碰内存 `s.tickets` map）+ `165`（sessions 同 map，其有效期由 sweeper 扫描生效）`;`internal/api/handler.go:153-155`（登录未激活即 CreateTicket）。

**事实确认**：票据不落库、进程重启即丢（运维重启空窗内已拿票据的未激活账号得重登录再激活——发生概率窗口极小、非灾难，故标 OBSERVE 不标 MINOR）。票据的"过期自动失效"无钟表自愈：5 分钟 TTL 只有"消费时判 `time.Now().After(t.expires)` 拒用"，不存在 session sweeper 同款定期清扫；tickets map 不主动回收过期票，只有同 token 再被消费时才删。幌子：N 账号部署下每次登录失败重登都 CreateTicket，短时大量未激活登录可让 tickets map 累积过期票（每票 ~200+ 字节，属可控量级）。无安全后果：票熵 32 字节 crypto/rand、防穷举由 handleActivate 先 ConsumeTicket（用掉即作废）封死。

### O106-02（新角度：激活码生命周期闭环）：handleActivate 的票据占用先于激活码单事务校验，且已激活防双扣在事务内但"已激活账号持真码"会消耗一次合法码次数——闭环无缺陷但有三处可精化的交接语义

**位置**：`internal/api/handler.go:181-215`（handleActivate：先 ConsumeTicket 作废票 → 再 ConsumeActivationCode）+ `internal/store/store.go:288-324`（ConsumeActivationCode 单事务：UPDATE 扣次 → 查已激活 → 已激活回滚不扣；INSERT activations → Commit）。

**事实确认**：
1. **票据先占用**（handler.go:199-202）在激活码校验前——主动作废票再让用户重登重拿（防重放穷举的刻意决策，契约 13 已落盘），即使激活码无效也绝不让同一票反复探测不同码。见 R105 未变动。
2. **已激活持真码会扣次**：ConsumeActivationCode 单事务 UPDATE 扣次（used_uses+1）成功后才查"该账号是否已激活"，已激活分支 `return false, nil` 让事务回滚（不使用这枚已扣次数）——但回滚前那一条扣次 UPDATE 并未触发数据变更（回滚撤销），**实际不扣**。出入链完全对称：已激活 → 回滚（票已作废、码未扣、号未复激活）→ 前端"该账号已激活"。**无半生效窗口**。
3. **激活一次永久免激活**：activation 行不随激活码删除/账号删除外任何动作消除；DeleteAccount 六表清理含 activations 行（store.go:410）——删除账号再登录即重新未激活，闭环收敛。

**无缺陷**，三处交接语义（对码方先执票、已激活回滚不扣、票据无钟表清扫）均注释在案、各有权衡记录。闭环成立。

### B106-01（知识位固化）：DDDocr 两本地引擎也走全局并发限流——`captcha_concurrency` 热改对全引擎生效，管控"平台熔断"的并发收敛面完整

**位置**：`captcha.go:102-107`（withConcurrency 全局限流）+ `local_ocr.go:32-35`（LocalDdddOcrRecognizer.Recognize = withConcurrency）+ `native_ocr.go:106-109`（NativeDdddOcrRecognizer.Recognize = withConcurrency）+ `captcha.go:196-201`（VisionRecognizer.Recognize = withConcurrency）。

**确认**：三引擎全部套 withConcurrency——`captcha_concurrency`（默认 1，热改上限 20）对 ddddocr 本机推理同样收口。并发热改闭环（applyCaptchaRecognizerFor → SetCaptchaConcurrency → globalLimiter.SetLimit+Broadcast）全链路挂钩。

## 必查项逐条结论

### 1. 身份防线矩阵（第二十一轮）——通过

R105「全仓写点全家福」方法论延续，`sameClientFor` 7 调用点逐一定位未漂移：
- `:850`（ProbeForAccount 回写段：空快照不删识别槽 + 身份复核后写 acctData/acctDataAt/openTimeDetected）
- `:1489`（ErrUnauthorized 失效分支）/ `:1521`（成功分支）/ `:1551`（风控退避分支）/ `:1571`(窗口关闭分支）/ `:1600`（实时复核回锁后统一复核）/ `:1635`（确证满员分支）——spawnChain 六分支 + 第七分支全覆盖。

主要写入路径聚类复核（行号相对 R105 无漂移）：
- `s.openTimeDetected[acct]` 仅 :856（ProbeForAccount 锁内 + sameClientFor 后）；`s.openTimeDetected["*"]` 仅 probe() :1108（锁内，全局载体，无身份概念）；PurgeAccount :506 全量删。
- `s.acctData[acct]` 仅 :863（锁内 + 身份复核后）。
- `s.tokenValid[acct]=true` :1241（maybeRelogin 决策侧，先 ClientFor 复核）`;false` :1262（重登成功 goroutine 写回侧，先 ClientFor 复核）;MarkTokenValid :1316（delete）。
- `s.done[acct][id]=true` :615（RestoreDone）/ :1529（spawnChain 成功分支 + sameClientFor）/ :1934（MarkDone + ClientFor 存在性）。RestoreDone 无身份概念（启动期单线程）。
- `s.refused[acct][id]` :634（RestoreRefused）/ :2011（RemoveDone + ClientFor）；MarkDone :1938（delete + DeleteRefusedClass 库行同步）。
- `s.rateLimited[acct][class]` :1714（markRateLimitedLocked——仅 spawnChain 风控分支经 sameClientFor 后调用 :1555，手动/其他路径不入此点）。
- `s.full[acct][class]` :1767（markFullLocked——spawnChain 窗口关闭/确证满员分支经 sameClientFor 后 :1575/:1639）。
- `s.inflight[acct][class]=true` :1471（spawnChain）+ :1905（TryAcquireSubmit）；delete 点 :1490/:1501/:1513（spawnChain）/ :1950（MarkDone）/ :2011 区（RemoveDone）/ :510（PurgeAccount）。
- `s.state.Courses` 写点：setStateLocked（:1846-1851 统一入口）+ MarkDone :1961/:1964（append）+ RemoveDone :2014-2015 + PurgeAccount :512-518 过滤 + :1474/:1804 状态覆写；判据外层全数在持锁 + 身份防线后。
- `s.acctTargets`：SetTargetsForAccount / RestoreTargets / PurgeAccount 三处。

**无新裸露写点**。偶发探测入口（ProbeForAccount :822→maybeRelogin 决策侧复核、ProbeNow :954、probe :1091）继续由决策侧/doneHas 胜利状态让位覆盖。测试族（probe_identity_test + open_detect_test + scheduler 身份防线族）定向 race 实测全绿。

### 2. O105-01 抖动基线（第二十一轮）——通过

socketPreheat（client_test.go:25）+ readyProbe 宽栅栏（client_test.go:88，与 captcha_test.go:29/api handler_test.go:179 同款 200ms×10 + 2s 超时）布局与 R105 逐字符一致，本轮零改动。未发现新时序脆弱点（夹具按测试包分别预热、mock 包就绪窗口前移）。定向多包 race 多批实测零 flake。基线口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（~13%）。主控全量 race 吸收结论。

### 3. 知识位与修复持续盯守——通过

- **B101-01（sanitizeError）**：定义 client.go:581（剥 *url.Error → Op+底层，保留 Unwrap 链）；doRequest :450 唯一应用点。zhidao 六测试 `-race` 全绿。**唯一 token URL 通道 = doRequest :422** `?idToken=`；api 层零 idToken 直拼。sanitizeError 覆盖网络层；业务层（顶层 ExtractMsg 返回平台明文文案）不含 token，天然闭合。
- **B102-01**：doRequest 仍是唯一携 token 通道（:422），其余 http 直调点自然静态 URL。
- **B103-01**：实时复核满员分支活化条件（classFullRealtime :1785 + IsClassFull `ce.MaxCount > 0 && ce.SelectedCount >= ce.MaxCount`）在位；cv 复核网络段 :1589 锁外执行无持锁网络。平台实证不可达知识位无退化。
- **B104-01**：手动路径无在飞窗口（B20-01 已封）知识位无退化；api 手动四路 accountExists（:245/:295/:373/:537）在位。
- **B105-01（展示层维护观察）**：stats `open_time_set`（handler.go:964）与 `open_time_str`（:899-902）语义仍为"识别槽有值"（含过期值），与学生端 open_time_known 的依 now 过期判定（scheduler.go:714）不对称维持——本轮走读无新依据提级，维持观察（识别值绝不截断契约正确，展示层遗留不影响调度）。

### 4. 既往观察项延续——通过

- **O105-02 删除保护撞名**（handler.go:992 单判据）：维持观察，无新依据提级。
- **O92-02 logintest 引擎判定源分叉**：实测复核 `cmd/logintest/main.go:57` `switch config.CaptchaEngineDefault()`（只读 XUANKE_CAPTCHA_ENGINE env + 默认 ddddocr），主程序 main.go:102-104 settings 持久化 `captcha_engine` 覆盖热改——**分叉确凿**；但 logintest 三态回退链（NativeDdddOcrAvailable → LocalDdddOcrAvailable → Vision）多数场景仍收敛使用，维持低风险观察（运维诊断工具不参与抢课）。
- **M87-01 窗口**：维持注释兜底。
- **O90-01 CRLF**：go vet 三包全干净 VET_EXIT=0。
- **契约 20 强扫**：全仓生产代码 grep `(第 N 轮|R\d+-\d+|O\d+-\d+|B\d+-\d+|M\d+-\d+|F\d+-\d+)` 仅命中两个**契约引用注释**（scheduler.go:843 引用 M88-01 决策契约、client.go:577 引用 B101-01 修复）——均属"契约号作为锚点引用"而非轮次标签污染，正文不陈述历史轮次结论，合规。
- **契约 17 零吞错**：handleAdminStats 目标数失败族（handler.go:903-926）仍明确 500，无 `_ =` 落库点。

### 5. 新契约角度（激活码生命周期闭环 + 登录状态机二查）——见 O106-01/02 + 下列分项

**激活码生命周期闭环**（生成 → 分发 → 激活 → 扣次 → 审计 → 清理）：
- 生成：newActivationCode（handler.go:690）crypto/rand 16 位 hex（64bit 熵），panic-on-failure；POST /api/admin/codes 批量原子落库（:649-661 任何失败不半批滞留）。req.Count 1-100 / req.Uses 1-1000 值域门。
- 分发/审计：ListActivationCodes 全量读（含 used_uses）；GET/POST/DELETE 全走 requireAdminSession + CSRF 门。
- 激活：handleActivate（票据先占；激活码单事务扣次+记激活原子；已激活回滚不扣）——防双扣/防超卖由 `used_uses < total_uses` 单条 UPDATE 原子保证（store.go:298）。
- 清理：DeleteActivationCode 单行删；DeleteAccount 事务清 activations 行（:410）。激活码无过期字段（生成即生效），无自动清理逻辑——激活码长生命周期（无 TTL 字段）是有意设计（管理端手动删除）。

**登录全链路状态机二查（对照 R96 归档）**：
- submitLogin body 四字段（captcha/identification/uniqueId/priorityId）与 R96 一致；priorityId 空串语义注释在案。
- Login 主循环 maxCaptchaAttempts=3：识别失败/提交被拒 → continue 刷新验证码重识别（页面会话 Cookie 每 attempt 独立 jar）；网络/配置错误立即返回（绝不无谓重试）。
- 登录成功 SetCredentials 写入客户端内部账户/密码（client.go:270）——ReloginIfNeeded 自愈基础，无回归。
- gateTryAcquire 收口 LoginByPassword（manager.go:244）+ gateWait 收口 ReloginIfNeeded（:173）全链路收敛 2 次/分钟平台登录。ReloginIfNeeded 回退链（Login 失败即返回不循环）正确。

**无漂移**。

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第二十一轮闭合**：7 个 sameClientFor 调用点全在位；10 类写点逐一对应防线；无新裸露写点。 | 通过（走读 + 定向 race 实测 5.16s） |
| **激活码生命周期闭环**：生成/分发/激活/扣次/审计/清理全链路原子性成立（单事务扣次+记激活、已激活回滚不扣、票据先占用防重放）。 | 通过（走读 + TestActivate* race 实测） |
| **ddddocr 本地引擎同样走全局并发限流**（native_ocr.go:108/local_ocr.go:34 均套 withConcurrency），并发热改对全引擎生效。 | 通过（走读） |
| **B101-01/B102-01/B103-01/B104-01/B105-01 持续盯守**：六测试 race 全绿；doRequest 唯一 token 通道；三判据在位；防删除族在位。 | 通过（实测全绿） |
| **O92-02 logintest 引擎分叉**：实测复核确认，维持观察。 | 维持 |
| **契约 20/17 强扫**：仅两条契约号引用注释（合法锚点），零轮次标签；零吞错落库点。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test ./internal/scheduler/ -run 'TestDeletedAccountRebuiltSameName\|TestWindowClosed\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared\|TestPurgeAccount\|TestProbeForAccount' -count=1 -race` | 全绿（身份防线族 + 窗口守卫 + 提交，5.16s） |
| `go test ./internal/api/ -run 'TestHandleElectivesSelect\|TestHandleElectivesSelectUnauthorizedRelogin\|TestAdminDeleteProtectsRenamedAdmin\|TestElectiveSelectRejects\|TestAdminElectivesUnknownAccountRejects\|TestActivation' -count=1 -race` | 全绿（手动四路 + 激活族，3.22s） |
| `go test ./internal/api/ -run 'TestActivateRequiresTicketAndBinding\|TestActivateBadCode' -count=1 -race -v` | 全绿（激活票据测试逐条 PASS，1.95s） |
| `go test ./internal/zhidao/ -run 'TestSanitize\|TestDoRequestSanitizes\|TestIsReadErr' -count=1 -race` | 全绿（脱敏五测 + 判型，2.08s） |
| `go test ./internal/session/ ./internal/db/ -count=1 -race` | 全绿（1.78s/2.54s） |
| `go vet ./internal/{scheduler,zhidao,api}/` | 通过（VET_EXIT=0） |
| `git status --short` / `git log --format=%h -3` | 无后端文件脏；HEAD=f5fdb36（前端改动，backend/ 连续四轮零改动） |

## 结论

1. **身份防线矩阵第二十一轮闭合**：sameClientFor 7 调用点无漂移、写点全家福聚类复核无新裸露写点、测试族定向 race 全绿。
2. **O105-01 抖动基线第二十一轮**：夹具走读无新脆弱点，定向跑多包全绿零 flake，口径维持（~13%）。
3. **知识位持续盯守**：B101-01/B102-01/B103-01/B104-01 全绿无回归；B105-01 展示层观察维持。
4. **既往观察项**：O105-02 删除保护撞名维持观察；O92-02 logintest 引擎判定源分叉经实测复核确凿、维持低风险观察；M87-01/O90-01 维持；契约 20 强扫仅两条合法锚点引用。
5. **新契约角度（激活码生命周期闭环 + 登录状态机二查）**：闭环原子性成立，两处交接语义（票据先占、已激活回滚不扣）注释在案；登录链路无漂移。唯一新观察 O106-01（票据内存态无钟表清扫 + 重启丢票）低风险，以及 B106-01 知识位（本地 ddddocr 引擎同样走全局并发限流）。
6. **新发现仅 O106-01（OBSERVE）+ B106-01（知识位）**：无 MINOR 以上新缺陷。激活票据过期只靠消费时判、map 不主动回收——量级可控（每票 ~200+B），安全无影响。

工作树后端文件洁净。