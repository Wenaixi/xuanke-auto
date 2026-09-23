# R102 后端只读审查报告

审查对象：xuanke-auto HEAD `75b7811`（R101 收官：B101-01 token 脱敏修复落地，身份防线矩阵第十六轮闭合，进度 102/256）。本轮回合为只读审查——全程零仓库文件修改，唯一写入为本报告文件（archive/review-rounds/round102-backend-findings.md）。主控双 opus 并行，后端侧由本代理执行（替代上一代理的未尽事宜）。

审查方式：Read / Grep / Bash 只读命令（定向 go test + go vet + 逐点走读）。核心走读范围：B101-01 修复回首轮（zhidao/client.go sanitizeError/sanitizerErr + doRequest 接入 + sanitize_test.go 六测试 + 全部 http 直调点枚举）、身份防线矩阵第十七轮（spawnChain 七分支 + maybeRelogin 双侧 + ProbeForAccount 回写段 + MarkDone/RemoveDone/SubmitAll/Restore + PurgeAccount 全量清）、抖动基线第十七轮夹具走读、既往观察项延续（O101-02/O92-02/M87-01/O90-01 + 契约 20 强扫）。新契约角度本轮聚焦：**B101-01 修复后的遗漏透传路径扫描（全 zhidao 包 http 直调点逐一枚举）+ keep-alive 连接池缺口 s 的账号边界评估**。

## CRITICAL

无。

## MAJOR

无。

## MINOR

### B102-01：登录链路辅助请求的裸 `sess.Do` 错误文本携带完整请求 URL——但登录期 URL 无 idToken，属"有 URL 无敏感"形态，与 B101-01 同族语义需盯守（走读评估）

**位置**：`fetchLoginPage`（client.go:300 `sess.Do`）/ `fetchCaptchaImage`（client.go:334）/ `submitLogin`（client.go:370）/ `Prewarm`（client.go:102）/ `SyncServerTime`（client.go:119）五处均为**裸 `c.http.Do`/`sess.Do` 直调，不经 sanitizeError**。

**逐点脱敏面分析**：
| 直调点 | 是否带 token | 连接层失败时 URL 是否回落 |
|---|---|---|
| Prewarm（GET /login） | 无（URL 恒 `/login`） | 是——但 URL 无可泄露段 |
| SyncServerTime（GET /login） | 无 | 是——同上 |
| fetchLoginPage（GET /login） | 无（登录初始化期 token 尚未存在，sess 为独立 Client 无 token 注入） | 是——同上 |
| fetchCaptchaImage（GET captcha） | 无 | 是——同上 |
| submitLogin（POST doLogin） | 无（form body 携 identification，URL 不含） | 是——同上 |
| **doRequest 全家族五接口** | **是（?idToken=）** | **已由 :450 sanitizeError 拦截** |

**结论**：B101-01 修复已闭合**全部携 token 的 URL 通道**（doRequest 是唯一把 token 拼进 URL 的路径）；五处登录链路辅助请求 URL 均无 token，网络层错误进日志时**不泄露会话凭证**。登录期 form body（identification 为 RSA 密文）不会因连接层错误回显——`url.Error` 文本只含 URL 不含 body，submitLogin 业务拒绝文案（`登录被拒绝: %s` 取平台 msg、`提交登录请求失败: %w` 包 conn err 均无 token）。**同族缺口不存在**，维持观察即可。

附带一个**契约瑕疵而非缺陷**：登录链路错误经 `log.Printf`（client.go:247/:261/:274/:1294）落盘的 err 文本在 B101-01 修复后仍然可读性完好（`dial tcp ...` 语义留存，sanitize_test 六测试已钉）。若未来平台把 token 移入请求头/body 通道（现契约锚定 `idToken` URL 参数，HAR 实证），需在提交方重新审计本扫描结论——本条存为知识位不报缺陷。

## OBSERVE（延续观察）

### O102-01（O101-01/O100-01/O99-01/O98-01/O97-01/O86-01 抖动基线第十七轮）：定向跑本轮无 flake，口径维持（实测）

走读 socketPreheat/readyProbe 双保险使用正确（client_test.go:39-42 包级 TestMain + 逐测试 + 逐 server；readyProbe 宽栅栏 10×200ms + 2s 显式超时，client_test.go:88-113；captcha_test.go 无探活 mock 宿主由 readyProbe 双保险覆盖注释完整）。本轮定向跑（zhidao 脱敏+判型+识别 + scheduler 身份防线族 + 窗口守卫）全绿无 flake，**未见新时序脆弱点**。基线口径维持：低频残余由「包序 + Windows 回环冷启动窗口」主导（八轮 30 跑 4 单包单 FAIL ~13%），CI `-p 1` + 失败重跑吸收。主控将跑全量 race 吸收结论。

### O102-02（O101-02/O100-02/O99-02/O98-02/O80-01 删除保护撞名延续复核）：handler.go:992 单判据与 B43-04 双条件不对称——历轮维持观察，无新依据提级（走读）

**证据**：handler.go:992 `if acct == "" || acct != req.Account || d.IsAdminAccountName(acct)` 删除保护仍只在账号名等于 `AdminNameValue()` 时拦截。撞名场景（`XUANKE_ADMIN_NAME` 配成某学生学号）下该学生被删除保护永久覆盖。历轮归"低优先级不修"的三条理由本轮逐一复核仍成立（requireAdminSession 前置 / B43-04 双条件签发让撞名学生正常登录 / 删除保护是防空删管理员自己的硬护栏语义）。**维持观察**。

### O102-03（O92-02 logintest 引擎判定源分叉延续复核）：维持关闭（走读）

cmd/logintest 走 `CaptchaEngineDefault()` 环境变量、主程序走 settings 持久化值覆盖——三态回退链完整，诊断工具语义固有分叉。R92 归档关闭、历轮维持，无新依据。**维持关闭**。

### O102-04（M87-01 窗口延续复核）：维持 MINOR + 注释兜底（走读）

5s Shutdown 超时与在飞 spawnChain 尽力优雅语义仍由 main.go:191-200 注释完整覆盖；quit_shared.go 双 nil 防御 + setExitActions 两半段 + tray_quit_test.go 三钉。**历轮结论仍准确，无新依据**。

### O102-05（O90-01 CRLF 延续复核）：维持（实测）

`go vet` 对 scheduler.go/zhidao 全干净（VET_EXIT=0）。CRLF 纯工作区转换噪音，仓库内容 LF 合规，历轮裁定永久无动作。**维持**。

### O102-06（契约 17 零吞错延续扫查）：通过（走读）

spawnChain 六分支 AppendLog/SaveSuccess、MarkDone/RemoveDone 双路、markFullLocked、SetTargetsForAccount 落库、maybeRelogin UpdateIDToken（:1264-1280）、handleAdminDeleteAccount、main 恢复各表——全部 `if err != nil { log.Printf }` 零吞错。本轮未扰动，维持历轮结论。

## 已核无缺陷清单（走读 + 定向实测）

| 项 | 结论 |
|---|---|
| **身份防线矩阵第十七轮闭合**：`sameClientFor`（scheduler.go:204-210 判 nil 恒 false + reflect 指针身份）+ 全量调用点**主控已 grep 实证 7 处**（:850 ProbeForAccount 回写段 / :1489 失效 / :1521 成功 / :1551 风控 / :1571 窗口关闭 / :1600 实时复核入口统一复位 / :1635 确证满员）。本轮补复核：第七分支实时复核 ErrUnauthorized（:1608）的 maybeRelogin 落入 :1600 统一 sameClientFor 之后才触发、非裸调（与 R101 归档语义一致）。maybeRelogin 决策侧存在性复核 :1208 + 写回侧 :1254（重登完成先清 relogging 再复核、成功分支内存写+落库同门）；ProbeForAccount 回写段 :850 持锁先 sameClientFor + 识别槽双条件覆盖 `len(data.BeginTimes)>0`（:855-858，空快照绝不删槽——关闭≠时间消失契约）；MarkDone :1927 / RemoveDone :1995 ClientFor 存在性、SubmitAll :1355 过滤已删账号、spawnChain 链顶 :1388 + 取 client 两处 :1405/:1409；RestoreDone/RestoreTargets/RestoreRefused 无网络不需身份；PurgeAccount :495-519 全量清含识别槽。**无新裸露写点**。round41 测试族 refresh 走读（scheduler_test.go:3023-3507 TestDeletedAccountRebuiltSameNameChainDrops{Succeed,Relogin,RateLimitBackoff,WindowClosedFull,RealtimeRecheckFull}）全部复走同款夹具、第七条独立红绿。 | 通过（走读 + 定向跑） |
| **B101-01 修复回首轮**：`sanitizeError`（client.go:581-593）`errors.As(err,&ue)` 命中 *url.Error → `Op + " " + ue.Err.Error()` 输出；非 url.Error 原样透传（:586-587，业务错误文案契约不误伤，TestSanitizeErrorOriginalErrorPreserved 钉）。`sanitizerErr`（:566-572）`Error()` 改写 + `Unwrap()=底层 err`——`isConnErrRetryable`/`IsReadErr` 的 errors.As/Is 对 net.OpError/io.EOF/超时文案穿透不受影响（TestSanitizeErrorPreservesJudgment 八子项全绿，含纯文本 timeout 形态原样保留）。doRequest 接入点 :450 单一收口。剥 URL 后可读性保留（`dial tcp ...`/`connectex` 语义留存，TestSanitizeErrorStripsTokenFromDialError + TestDoRequestSanitizesDialError 端到端实测绿）。**errors.As/Is 判型未破坏、业务错误透传原样**。六测试 + isreaderr_test 全绿实测。 | 通过（实测 2.985s 全绿） |
| **B102-01 遗漏透传路径扫描（本轮新角度）**：逐一枚举 zhidao 包全部 http 直调点（详上表）——**doRequest 是唯一把 token 拼进 URL 的通道且已闭环**；五处登录/预热/时钟辅助请求 URL 均无 token，连接层错误文本进日志无凭证泄露；submitLogin 业务拒绝文案不携 URL/body（identification 为 RSA 密文不因 url.Error 回显）。**同族缺口不存在**。 | 通过（走读 + 逐点枚举） |
| **keep-alive 连接池缺口 s 的账号边界评估（本轮新角度附）**：sharedTransport 全局共享（client.go:22-34，MaxIdleConns 128/PerHost 64/IdleConn 120s/HTTP2）。账号边界语义：**TCP/TLS 连接本身无账号态**，账号隔离由逐请求的 URL 参数（idToken）+ Cookie 头（zd_edu_cookie）承载——共用连接池不同账号先后复用同一物理连接的保持态**绝不串账号**；请求头发送由 Client.Do 每次组装，Cookie/令牌取自该 Client 自身 mu 保护的 map（doRequest :415-421 锁内快照），无跨账号复用缓冲区的共存面。与 B101-04 归档（R101）的全局资源边界结论一致，**连接池共享正确、无账号边界旁路**。 | 通过（走读） |
| **契约 20 强弱扫描**：全仓生产代码（排除 _test.go 与 docs/archive）grep `\b(B|M|O|R|F)[0-9]{2}-[0-9]{2}\b` 命中 5 处：scheduler.go:843 M88-01、main.go:189/quit_shared.go:7/tray_windows.go:53/:65 M86-01。（分别扫描 R 前缀轮次锚点：无 fallthrough。）历轮裁定"注释含协议锚点（契约号标注，非轮次决策历史）符合规范"，本轮强扫一致；mapper 全部映射 CLAUDE.md 决策手册契约条目，内容均表述"为什么/契约/陷阱"。测试注释中"历史轮次/第 3 轮"类字面为测试场景描述（非生产注释），docs/review-rounds 文件名不违规。**无违规需上报**。 | 通过（走读） |

## 构建验证表

| 命令 | 结果 |
|---|---|
| `go test ./internal/zhidao/ -run 'TestSanitize\|TestDoRequestSanitizes\|TestIsReadErr\|TestRecognizeCaptcha' -count=1` | 全绿（2.985s，脱敏六测试 + 判型五形态 + 识别） |
| `go test ./internal/scheduler/ -run 'TestDeletedAccountRebuiltSameName\|TestProbeForAccount\|TestProbeChainSameClient\|TestWindowOpenSubmitsWithoutProbeReset\|TestSubmitSuspendedWhenOpenTimeCleared' -count=1` | 全绿（9.333s，身份防线六测试 + 窗口守卫双基线） |
| `go vet ./internal/zhidao/ ./internal/scheduler/` | 通过（VET_EXIT=0） |
| `git status --short --branch` | 干净（工作树后端零改动） |

## 结论

1. **身份防线矩阵第十七轮闭合**：主控 grep 实证 7 调用点 + 本轮补复核第七分支 maybeRelogin 落在 :1600 统一复核之后、MarkDone/RemoveDone/SubmitAll/Restore 各写点封口——无新裸露写点。
2. **B101-01 修复回首轮通过**：sanitizeError 剥 URL 保留判型与可读性、business 错误原样透传，六测试实测全绿；本轮新角度逐点扫描确认 **doRequest 是唯一携 token URL 通道且已闭环，五处辅助请求无敏感 URL，同族缺口不存在**。
3. **抖动基线第十七轮**：定向跑零复现，双保险夹具走读无新脆弱点，口径维持（~13% 低频残余由包序+冷启动窗口主导）。
4. **既往观察项延续**：O101-02 删除保护撞名维持观察；O92-02 / M87-01 / O90-01 / 契约 17 / 契约 20 均维持历轮结论（零新依据）。
5. **新发现仅 B102-01 一条 OBSERVE 级（若归类）**——但经逐点枚举实为"无敏感 URL、无需处理"的钉死契约，实际本轮无具修复价值的真实缺口；B101-01 修复质量整体优秀。

工作树后端文件洁净。