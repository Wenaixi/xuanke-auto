# 0004 选课平台档案化，站点差异不进引擎

## Status

已接受。

## Context

上游选课站点的接口形态会变：域名可能迁，路径可能改，字段名可能重命名，登录表单与加密方式也可能换。改动前这些事实（8 个接口路径、登录表单键名、`idToken` 与 `zd_edu_cookie` 两个鉴权载体名、`code/isOk` 双布尔判据、以及 5 组响应字段族）以字面量形式散落在 `backend/internal/zhidao/client.go` 与 `login.go` 里，站点根地址则硬编码在 `config.go`。任何一处变化都要动引擎代码并发版，而"每年接口可能不一样"是已知现实。

与此同时，真正需要长期稳定的东西——会话隔离、token 失效自动重登、探测节流、开窗识别、黄金期冲刺、失败退避、满员判定——与站点是哪一家、接口叫什么名字毫无关系。

## Decision

把「站点长什么样」与「流程怎么跑」切开，接缝只有一个数据结构：`upstream.SiteDescriptor`。

- **引擎**（`backend/internal/upstream`）只负责流程与传输：共享连接池、自愈重试、错误脱敏、token+Cookie 双通道会话、`ErrUnauthorized`、中立数据模型（`Class/Publish/ElectivesData/YearTerm/CountEntry`）、验证码识别引擎、登录链路骨架。它不认识任何具体站点的路径或字段名。
- **平台档案**（`backend/internal/sites/<名字>/`）只提供站点事实：站点默认地址、9 个路径、登录表单键名、鉴权载体名、状态码语义、4 个响应解码函数与 2 个登录钩子（账密加密、设备指纹）。每个平台一个 Go 适配器包。
- **注册表**（`backend/internal/sites`）是唯一的档案清单：`List()` / `Resolve(id)` / `DefaultID`。前端与管理后台只消费 `List()`，因此新增档案零前端改动。
- **管理员可选、可覆盖**：「系统配置 → 选课平台」从注册表选档案，并可覆盖站点地址（换域名/镜像免发版）。切换走"先验证后生效"：档案不存在、地址非法、读凭据失败一律整体拒绝并保持原状。
- **跨平台会话绝不外发**：`credentials.platform_id` 记录 token 属于哪个档案（旧库由纯加列迁移补齐）。`Restore` 只注入与当前档案同源的 token；`SetProfile` 重建客户端时一律置空 token 并落库打上新标记。切换后各账号用已保存账密自动重登。
- **档案只增不删**：settings 里持久化的档案标识必须始终可解析，否则升级/降级后会静默把请求打到另一个站点。

## Consequences

- 新增一年的接口形态 = 新增一个适配器包 + 注册表一行；引擎、调度器、API 与前端一行不改（新增档案连前端都不用动）。代价是多了一层间接：读代码时要先看档案才知道某个请求打到哪里。
- 平台专有事实集中在一个包里，结构上可验证：`web/scripts/platform-guard.ts` 断言前端源码零站点域名/接口方法名/鉴权载体名，前端只消费中立数据模型。
- 目标课程、成功记录、退选记录**不因切换平台清空**（换平台是同一套选课语义的接口换代）；只有会话与站点地址属于平台。
- 退役档案不能删除，注册表会随时间积累（可标注"仅回溯"）。这是刻意的：静默换平台是最难发现的线上故障。
- `config.Config` 里保留了 `DefaultPlatformID` 字面量（而不是 import 注册表），因为上游的 `native_ocr`（CGO 构建）反依赖 `config`，直接 import 会在 CGO=1 构建里成环；一致性由 `internal/sites` 的 `TestDefaultIDMatchesConfig` 锁定。
- **核对的源文件**：`backend/internal/upstream/descriptor.go`（`SiteDescriptor` 字段、`Validate`、`ValidateBaseURL`、`NormalizeBaseURL`）；`backend/internal/upstream/client.go`（`doRequest` 用 `TokenParam/TokenCookie/RefererPath`；`YearTerms/FindElectives/StudentCounts/classOp` 用档案路径与表单键名）；`backend/internal/upstream/login.go`（`LoginEngine.desc`、`EncryptIdentification`/`DeviceID` 钩子、`Decode.Login`）；`backend/internal/sites/registry.go`（`List/Resolve/IDs/DefaultID` 与"只增不删"注释）；`backend/internal/sites/zhidao/zhidao.go`（路径、键名、状态码、解码与登录钩子的唯一出口）；`backend/internal/accounts/manager.go`（`New(desc, baseURL, ...)`、`Restore` 的跨平台丢弃、`SetProfile` 三步语义）；`backend/internal/api/handler.go`（`AdminConfigView.Platform*`、PUT 的"先验证后生效"、`RebindPlatform`）；`backend/server.go`（`newPlatformRebinder`、`effectiveBaseURL`、启动期 `sites.Resolve` 的 fatal 分支）；`backend/internal/config/config.go`（`PlatformID`/`PlatformBaseURL` 与 `DefaultPlatformID`）；`backend/internal/runtime/config.go`（`platform_id` 只接受可解析值、`platform_base_url` 空串为有效语义）；`backend/internal/db/db.go`（`migrateAddCredentialPlatform`）；测试锚点 `backend/internal/sites/registry_test.go`、`backend/internal/sites/zhidao/zhidao_test.go`、`backend/internal/accounts/profile_test.go`、`backend/internal/api/config_platform_test.go`、`backend/platform_rebind_test.go`、`web/scripts/platform-guard.ts`。

### 后续解耦（2026-09-30，延伸本 ADR 而非推翻）

初次档案化只覆盖「地址/路径/键名/解码/算法」，遗漏三类必须下沉的事实。逐行核实后
确认的缺口与对应补齐：

1. **语义判据**：`btn_type`（站点按钮编码，1=退选/2=报名）与 `in_date_range`（窗口是否
   开放）曾同时存在于引擎数据结构、调度器开窗判定与前端按钮渲染——等于把「这个站点
   怎么表示按钮」写进了 UI。补齐为中立 `action`（`enroll`/`withdraw`/`none`）与**三态**
   `selectable`（`true`/`false`/`nil`，`nil` = 站点不下发该信号；退化模式依赖此三态来
   区分「站点说未开」与「平台无此信号」），站点编码由适配器映射。
2. **规格常量**：验证码「3~5 位 + 纯英数字」写死在引擎 seam、`access_limit_cookie`
   写死在 accounts 包、验证码 URL 硬拼 `?v=`——全部下沉为档案字段（`CaptchaSpec` /
   `SessionCookies` / `CaptchaCacheBustParam`）。零值语义统一为「不校验/不拼/不需要」，
   保证旧档案不声明也能工作。
3. **错误判据**：`scheduler/errors.go` 用中文文案（频繁/429/稍后重试/关闭/未开启/
   报名时间/已结束）判断风控与窗口关闭，与本项目「错误分流只认结构化事实」的既有
   契约冲突，且平台原文含「已满员」的非满员失败会被误判。补齐为
   `SiteError{Kind, Msg, Code}` + 档案 `OpErrorClassifier`；**中文匹配只允许存在于
   站点适配器包内**，那是它唯一合法的位置。

新增档案 `testsite`（刻意在每个维度都与知到不同）作为**机械验收**：它能在零引擎改动
下接入并通过全测试，即证明接缝真的解耦。任何人往引擎里加新的站点假设，它会红。

**明确不做的事**（成本收益裁定，且会牺牲现有能力）：`class_id` 保持整数（字符串 ID
平台由适配器转稳定可逆整数，映射约束写进档案注释）；三层数据模型固定（无学期概念的
平台在适配器造虚拟学期）；无开窗信号的平台走退化模式（探测到数据即视为开窗，黄金期
冲刺精度下降但不拒绝加载）。

#### 落地结果（2026-09-30 全部完成并实测）

七个任务按依赖顺序落地，15 个后端包 + 前端 46 个测试 + 七套守卫全绿。落地过程中
额外发现并修掉三处**实测才暴露**的问题（都不是设计问题，是实现细节陷阱）：

- **验证码长度必须按字符数判定，不能用 `len()`**：`Charset` 可声明含中日韩等多字节
  字符集（`\p{Han}`），`len("1中2中")` = 7 而非 4，会把合法识别结果误拒。
- **档案字符集的合法性无法用通用启发式判定**：实测三种判据全部失败——「能否编译」
  （`[invalid` 恰好编译成合法嵌套字符类）、「是否含字母数字」（`[invalid]` 恰含
  i/n/v/a/l/d）、「剔除比例过半」（合法的纯数字类 `0-9` 剔掉全部 26 个字母，恒超半数）。
  故 `validCaptchaCharset` 只挡**编译不过**的硬错误，语义正确性由档案作者负责；
  真出问题的后果轻微（多保留噪声字符会被长度门禁拦下），不会静默丢字符。
- **`Scheduler.hasWindowSignal` 的零值陷阱**：`New()` 若按零值 `false` 初始化，
  全部探测会走退化模式（有数据即开窗），「窗口未开」判定彻底失效——表现为窗口
  未开时课程状态直接 `success`。故 `New()` 刻意默认 `true`，由装配面按档案纠正。

平台切换时的接线是**唯一不能漏的一步**：`newPlatformRebinder` 的 `onProfile` 回调
把新档案的 `HasWindowSignal` 同步给调度器（`TestPlatformRebinderNotifiesProfile` 守护）。
漏掉它会让切到无信号平台后开窗永远判不出来，调度器全线停摆。

守卫同步加强：`platform-guard.ts` 的站点事实黑名单补入 `btn_type` 与 `in_date_range`
（当初恰是漏了这两项，才让按钮编码泄漏进 UI 而守卫全绿），并新增四条**正向**断言
（`action` 中立枚举、`selectable` 三态、无 `btn_type` 声明、消费点读中立名）——
只有黑名单会在「字段被删掉」时误判为通过，正向断言补上这一缺口。

#### 传输层补齐（2026-10-01）

上两轮把「站点返回什么数据」彻底档案化了，但深度审查（逐行核实 + 实测，非推测）
确认「怎么发请求、怎么读信封」这一层仍写死在引擎里。四种信封的实测结果：

| 站点信封 | 改动前引擎行为 |
|---|---|
| `{"code":"UNAUTHORIZED"}` | 崩：`cannot unmarshal string into int` |
| 纯文本 `UNAUTHORIZED` | 崩：`invalid character 'U'` |
| `{"status":401,"data":null}` | **`err=nil` 静默通过** |
| `{"data":{...}}`（无信封） | 通过（侥幸） |

第三行最隐蔽：token 失效被判成成功，自动重登永不触发，调度器静默停摆且无日志线索。

补齐四个档案字段（全部**零值兼容**，既有档案不写任何一项则行为逐字不变）：

- `Envelope func(body) (code int, unauthorized bool, err error)`：业务状态判据下沉。
  `unauthorized` 直载「是否未登录」这一位事实，档案不必把字符串码映射回 int 空间；
  `err` 表示「响应体不是本形态」，引擎据此跳过信封检查交下层解码器。
- `AuthMode`（四态）+ `HeaderTokenName`：token 通道可关闭。零值 `AuthDual`。
- `Headers map[string]string`：附加头可增删，**空串值 = 显式删除**（这是关掉
  jQuery 指纹的唯一入口）。空 map = 引擎默认头集。
- `FormEncoding`（`form`/`json`）：登录体编码可切换，两条路径共用「空键名绝不提交」契约。

同时删除死字段 `CodeOK`——全仓零消费点，纯维护陷阱。

**方法论留档**：本轮所有新行为都用负向验证确认过「测试真能抓回归」，且抓到的
报错正是审查阶段实测的缺口本身（拆掉 `Envelope` 钩子后报 `err=<nil>`；
把 `AuthHeader` 误加进查询分支后报出 `=TOK123`）。这比「测试通过」有说服力得多。

`testsite` 同步升级为传输层真覆盖：字符串业务码、纯文本错误页、Header-only 鉴权、
无 XHR 指纹头四个维度各有一条真实 HTTP 往返断言。它上轮遗留的包注释
「鉴权只经 Header 语义」本是**虚假声明**（实际填着 URL 参数 + Cookie，而引擎
当时根本不支持 Header 鉴权）——虚假覆盖比没有覆盖更糟，已改成真话。

顺带修一处实测隐患：`submitLogin` 曾无条件写 `cookies[TokenCookie]`，纯 Header
平台该字段为空，会产出「空键名 Cookie」（形如 `=TOK`）。
