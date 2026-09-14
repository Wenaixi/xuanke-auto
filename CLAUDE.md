# xuanke-auto 项目记忆库

## 项目目标
自动化选课：知道教育平台 https://www.zhidao.fj.cn/admin.html#/electives/select
使用 Python + requests 实现课程查询和定时抢报。登录、选课、课程详情接口均已逆向完成并实测通过。

## 认证机制
- **无 Bearer Token**，使用 `idToken` URL 参数方案
- `idToken` = `zd_edu_cookie` cookie 的值
- 每个 API 请求 URL 后附加 `?idToken=<token>`
- Cookie 可复用；过期后运行 `python login.py` 自动重新登录，成功后自动写回 config.py
- token 是 19 位纯数字（如 ***REMOVED***），**无 Expires 会话级 cookie**，有效期由服务端决定
- 失效表现：接口统一返回 `{"code":-1,"msg":"您未登录,请刷新页面重新登录"}`

## 登录链路（login.py，逆向自 /login 页面内联 JS）
1. `GET /login` 初始化会话，种下 `access_limit_cookie`
2. `GET /login/captcha?v={random}` 获取验证码图片，会话绑定 `_jfinal_captcha`
3. `identification` = RSA 公钥加密（PKCS1 v1.5，1024 位）的 `{"userName":账号,"password":密码}` JSON，base64 输出
   - 公钥（DER base64）内嵌在 /login 页面：`MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCWuhgriWbHIbPCQyHmablwQSyItcLyKlQU/0ydXkvU4KJtEExNmuXS0xdoVLBRGxNO5f2u2MkNGzJrFhSpVL68Qc0knhWofzs+BdtpSF4nMi7BteOvOKi0OkvhhCBcHL71Vk8UXOsaKDZkZ3lCBVQHpSA4+s6pi9xIeF93jz6pGwIDAQAB`
4. `uniqueId` = base64(UA|Win32|屏幕高|屏幕宽|毫秒时间戳转36进制)（复刻 getUniqueDeviceId）
5. `POST /login/doLogin`（form 编码），成功返回 `token`，即新 idToken

## 验证码识别（captcha.py）
- 使用**硅基流动（SiliconFlow）Vision 模型**识别，配置在本地 `config.py`（SF_BASE_URL / SF_API_KEY / SF_MODEL）
- 调用 OpenAI 兼容 `/chat/completions` 多模态接口，实测一次成功率接近 100%
- 识别失败会自动刷新验证码重试（login.py 内置）

## 核心接口（选课，全部 POST + idToken）

| 接口 | 请求体 | 说明 |
|------|--------|------|
| `POST /electives/select?idToken=..` | 无 | 学期列表 currentYearTermList（含 selected 标记） |
| `POST /electives/select/findElectivesData?idToken=..` | json: `{"schoolYear":2026,"schoolTerm":1}` | 课程数据（不传 body 也会返回当前学期） |
| `POST /electives/select/findElectivesStudentCount?idToken=..` | form: `ids=1,2,3`（逗号分隔） | 实时已报/已确认人数（前端每 10 秒轮询） |
| `POST /electives/select/selectElectivesClass?idToken=..` | form: `classId=<课程id>` | 报名（参数名已用 classId=-1 无风险验证：返回"选修班不存在"） |
| `POST /electives/select/exitElectivesClass?idToken=..` | form: `classId=<课程id>` | 退选 |

响应统一为 `{"code":0,"isOk":true,...}`；code=-1 未登录、code=1 业务错误（如"选修班不存在"）。

> 注：`POST /electives/classDetail` 详情接口已逆向（上课地点/授课老师/课节等，详见旧版记录），因选课大厅已取消"详情弹窗"，后端与前端不再调用，代码已全部移除；如未来需要可据 HAR（课程www.zhidao.fj.cn.har）恢复。

## 课程数据关键字段（findElectivesData → electivesClassList 每项）
- `id`：课程 id（报名用 classId，详情用 id），范围 61115-61283
- `course_name`：课程名；`class_name`：选修班名称；`teacher_name_list`：任课教师
- `can_select`：当前是否可报名；`btn_type`：1=退选按钮，2=报名按钮；`btn_text`：按钮文字（报名/退选）
- `selected_count`/`max_count`/`plan_count`：已报/可报/计划人数
- `publish_id`：所属发布；`plan_id`：教学计划 id（报名不需要）
- 外层 selectElectivesData 每项含 `publishName`（如"高二年体育"）、`canSelect`（最多可选几门）、`hasSelected`（已选几门）、`inDateRange`（窗口是否开放）、`groupCount`（组数）、`beginDate`/`endDate`

## 已知数据
- 选课开放时间：**2026-09-13 09:00:00**（时间戳 1789261200000），窗口 09:00-10:30
- 三个发布：体育（3 门，36 人/门）、校本1（40 门，29 人）、校本2（39 门，29 人）
- 例：健美操=61115、排球=61125、篮球=61135；校本1 篮球=61205；健身瑜伽=61276；近代物理选讲=61245

## 文件结构
```
xuanke-auto/
├── config.py      # token、cookie、账号密码、验证码 Vision 配置（login.py 自动更新 token）
├── captcha.py     # 验证码识别（硅基流动 Vision，配置读 config.py）
├── login.py       # 完整登录（RSA+Vision 验证码识别），成功后写回 config.py
├── xuanke.py      # 主脚本，query / detail / monitor 三种模式
├── 课程www.zhidao.fj.cn.har  # 课程详情弹窗抓包（classDetail）
└── CLAUDE.md      # 本文件
```

## 使用方式
```bash
python login.py            # 重新登录并更新 token
python xuanke.py query     # 查询当前课程信息
python xuanke.py detail 61245 61115   # 查看课程详情（上课地点/授课老师）
python xuanke.py monitor   # 监控模式（窗口开后自动提交）
```

在 `xuanke.py` 顶部修改：
- `AUTO_SUBMIT = True` 开启自动提交
- `TARGETS` 支持课程 id（如 [61135]）或课程名子串（如 ["篮球"]，会命中所有同名班次）

## 依赖
`requests`、`cryptography`（验证码识别走硅基流动 Vision API，需 config.py 中配置 SF_API_KEY）

## 注意事项
- HAR 中并不存在 `/electives/select/apply`，真实报名接口是 `selectElectivesClass`（历史文档误记为 apply）；`/electives/apply` 是教师端"选修课申报"页面入口，与学生报名无关
- `findElectivesStudentCount` 窗口未开放时 countList 为空属正常（ids 为空时报错 code=1）
- 登录成功后 `access_limit_cookie` 会更新为新会话的值，但 API 请求主要靠 idToken，access_limit_cookie 不敏感
- 待办：xuanke.py 遇到 code=-1（token 过期）时尚未自动重新登录，只在 monitor 打印提示
## Go + React 现代版（backend/ + web/）

**状态：** 独立模块化 React 前端 + Go 嵌入式单二进制交付（开箱即用，免环境依赖）。选课窗口 2026-09-13 09:00:00。已落地：连接池预热 / 服务端时钟对齐 / 黄金期 250ms 冲刺 / 失败分级退避 / ddddocr 本地识别（引擎热切换）。

### 高性能抢课架构（P0 三刀 + 智能调度）
- **共享高性能连接池 + 预热（压制 1.9s TLS 握手）**：`zhidao.sharedTransport`——`MaxIdleConnsPerHost: 64`、`IdleConnTimeout: 120s`、`ForceAttemptHTTP2: true`；调度器开窗前 2 分钟起 `maybePrewarm` 每 15s 静默 GET /login 保持 TCP/TLS 热态，首波提交零握手等待
- **服务端时钟毫秒级对齐（tick 全程用校准时间）**：`SyncServerTime` 读 HTTP `Date` 响应头 + RTT/2 中点近似得 `clockOffset`，调度器 `nowAligned()` 统一取校准时刻判定开窗点与冲刺期，根本性消除本地时钟误差（实测校准偏差 ~640ms）；约 60 秒闸门内不同步一次（`time.Minute`）。**第 7 轮 B7-M1 + 第 8 轮 B8-M1 + 第 9 轮 B9-03 根治**：`lastSyncTime` 只被成功采样推进 + **`maybeSyncClock` 同步段先查 `syncing` 再置位 + `lastSyncStart` 记录发起时刻**——B7 的 `syncing` 置位原在异步 goroutine 内部（调用侧检查恒 false = 死代码，每次 300ms tick 都并发起 SyncServerTime goroutine），B8 把防重入移到调用侧（第二个调用直接放弃）；**B9 加失败退避：失败落地记 `lastSyncFailAt`，30s 窗口内不再发起（此前失败后 `lastSyncTime` 恒零、快路径被绕过，每 tick 都裸打同步轰炸）；成功即清 `lastSyncFailAt`（瞬断不拖延后续校准）**；失败绝不吃闸门也不复位，SyncServerTime 挂起 >300ms 不再跳过一个 60s 校准窗口，开窗前抖动不会导致黄金期全程无校准。**第 12 轮 F12-B1 补齐（无账号悬空根除）**：复位 syncing 的唯一路径原是异步 goroutine 内部——空库部署/账号全删/客户端非 TimeSyncer 时 `syncing` 置 true 后无人复位，后续每个 tick 在 `if s.syncing` 处直接返回，时钟校准从启动起永久休眠、clockOffset 恒 0 且无任何错误日志。修复：先确认有可同步客户端再置位，无客户端时复位 syncing（并补 `clients==nil` 空指针防御）；`TestClockSyncNoClientResetsSyncing` 固化。
- **开窗前 10 秒黄金期 250ms 高频冲刺**：`submitIntervalFor` 依据对齐后时刻在开窗后 10s 内压到 250ms 间隔持续 submitAll，10s 后回落 1s 常态；探测仍受 30s 节流但提交完全不受限
- **失败分级智能退避（风控 30s / 网络快重试）**：`isRateLimitError` 匹配"频繁/429/稍后重试"文案→`markRateLimitedLocked` 该课程退避 30s；纯网络失败终止本链下 tick 快重试（250ms 黄金期）；Token 失效走 maybeRelogin 异步自动重登
- **验证码识别引擎二选一 + 并发限流（默认 1）**：`CaptchaRecognizer` 接口抽象——`VisionRecognizer`（硅基流动 Vision 云）/ `LocalDdddOcrRecognizer`（子进程调本机 Python ddddocr，免 API 密钥）。全局 Mutex+Cond 动态限流器（`captchaLimiter`：Acquire/Release/SetLimit 热收敛，管理员改并发即时生效，支持 3 秒内 20 次热调不死锁）；管理员「系统配置-识别引擎与并发」二选一切换并校验环境缺失自动回退 Vision；`captcha_engine`/`captcha_concurrency` 持久化 settings 重启恢复
- **并发重登会话隔离加固（数据层 2 个 CRITICAL 连根拔除）**：
  1. **重登只信客户端内部账密**：`zhidao.ReloginIfNeeded` 与调用方传入的账号名完全解耦——客户端本身唯一绑定账号（`ensure` 分配、`Restore`/`SetCredentials` 注入），重登一律用客户端内部 `account/password` 登录自己，并发为多账号调用时绝不用空壳账密把别人的 token 写进别人的客户端（数据层 CRITICAL 1）；
  2. **token_valid 读写串行化**：`scheduler.TokenValidFor` 与 `maybeRelogin` 统一用 `reloginMu` 串行化"决策是否重登/查询有效性"两段，消除并发重登时 `tokenValid`/`relogging` 读到的半态竞态（数据层 CRITICAL 2）；
  3. **运行时登录也写账密（第 6 轮 CRITICAL 3 根治）**：`zhidao.Login` 成功分支同步 `SetCredentials(account,password,token)`——此前运行时账密登录（/api/login、`LoginByPassword`）从不注入内部账密，客户端只有 `Restore` 恢复路径有账密，导致 token 一失效 `ReloginIfNeeded` 永远报"未登录且无保存账密"、黄金期失效即全程停摆；修复后每个客户端始终只用自己的账密重登自己，绝不交叉污染。
- **限流 IP 透传可信反代开关（第 6 轮 B6-05）**：登录/激活按 IP 分桶限流（5 次/分钟）只认 `RemoteAddr`；反代部署时学校 NAT 全校共桶互锁。`XUANKE_TRUSTED_PROXY=on` 且 RemoteAddr 为回环（真实本机反代）才信任 `X-Forwarded-For` 最右非空值——默认关闭，绝不盲信公网伪造 XFF（可刷爆他人限流桶/绕过自身限流）。
- **窗口状态裸指针根治（调度器 MAJOR 3）**：`WindowOpened` 由返回 `*bool` 裸指针改为布尔值快照——调用方拿地址读不再被并发探测改写的内存（指针悬空竞态已根除），新增 `HasProbed` 供 admin/stats 区分"从未探测"与"探测结果为关"，`boolPtr` 残留已清除。

### 架构设计（模块化开发 + 单二进制嵌入交付）
- **开发态（前后端分离极速热重载）**：
  - 前端：React 18 + Vite + TypeScript + Radix UI 原语 + Tailwind CSS，独立在 `web/` 开发，享受秒级 HMR；
  - 后端：Go 标准库 `net/http`（Go 1.22+ 原生路由）+ `modernc.org/sqlite`（纯 Go 免 CGO），独立在 `backend/` 监听 `:3091` 提供 REST API 与按账号隔离的日志查询流（前端 2s/10s 轮询 `/api/logs`，**非 SSE**——B8-M5 已修正旧文档"SSE 实时抢课日志流"的说法，SSE 需长连接+断线重连，相对轮询无收益且轮询已被 window_closed 降频覆盖）。
- **发布态（前端嵌入二进制，便携单文件交付）**：
  - 前端 `npm run build` 生成的纯静态产物（`web/dist`）通过 Go 原生 `//go:embed dist/*` 嵌入进 `backend/xuanke.exe`；
  - 最终用户无需安装 Node.js 或前端环境，双击单个 `xuanke.exe` 即可在单一端口同时提供后端抢课引擎与前端网页，开箱即用！
- **防逆向交付（garble 混淆）**：本地构建可用 `garble -literals -tiny build -ldflags="-s -w -H windowsgui"` 混淆 Windows 发布 exe（`-literals` 加密字符串字面量、`-tiny` 剥源码路径、`-ldflags` 剥符号表；实测 12.6MB → 24.7MB，strings 扫描零命中）。**注意**：CGO=1 与 garble 不兼容（garble 需 CGO=0），而 Windows 原生内嵌 ddddocr 必须 CGO=1——故自动发布流水线 release.yml 不再产出 garble 混淆版（Windows 交付即 CGO=1 原样构建；Linux/macOS 走 CGO=0 交叉编译，如需混淆可在本地手动执行）。garble 用 `go install mvdan.cc/garble@latest`（注意 v0.17.0 需 go ≥1.26.2，自动切 go1.26.8 工具链）
- **单二进制内嵌原生 ddddocr（彻底摆脱 Python 运行时依赖）**：ONNX 模型（common_old.onnx 13MB）+ 字符集（charsets_old.json 56KB）+ ONNX Runtime（onnxruntime.dll 16MB）经 `//go:embed` 编译进单个 exe；运行时懒加载把资源释出到 `%TEMP%\xuanke_ddddocr_assets`（dumpIfDiff 对比大小，无变化不重写），调用 `github.com/yangbin1322/go-ddddocr` 的 `Classification` 直接在进程内推理，单次识别 5~10ms。**构建双轨（build-tag）**：`native_ocr.go`（`//go:build windows && cgo`）走内嵌实现，`native_ocr_stub.go`（`!windows || !cgo`）返回 false/nil 自动回退本地 Python 桥接或 Vision——Linux/macOS 与 CGO=0 交叉编译不受影响。**引擎优先级**：router 对 `XUANKE_CAPTCHA_ENGINE=ddddocr` 先试 `NativeDdddOcrAvailable()` → 回退本地 Python → 再回退 Vision。**注意**：CGO=1 与 garble 混淆不兼容（garble 需 CGO=0），Windows 发布版必须用 CGO=1 原样构建，Linux/macOS 才走 garble。
- **选课窗口关闭后平台行为（实测）**：窗口结束后 `findElectivesData` 返回 `code:0` 但 `publishes`/`electivesData` 全空（并非 token 失效 code=-1）；`parseElectives` 对此直接返回空快照，`FindElectives` 不再因空快照落后陷入学期列表兜底重试（兜底拿不到更多课程，纯浪费时间）；`selectElectivesClass` 对已关闭窗口返回 `code:1` 报名错误，调度器新增 `isWindowClosedError`（匹配"关闭/未开启/报名时间/已结束"）按满员记入 `full` 集合，窗口关闭后不再每个 tick 反复轰炸报名接口。
- **全校探测节流闸门（第 6 轮 B6-04）**：`scheduler.lastProbe` 全校 30s 探测节流闸门**只归 spring 正规探测 `probe()`/`ProbeNow` 写入**；管理员穿透探测 `ProbeForAccount` 概不旁路——开窗前管理员点一次课程页若吞掉闸门，全校探测即被高频轰炸触发平台熔断（课程拉空）。**第 12 轮 F12-B2 + 第 14 轮 B14-I2 定案**：`probe()` 内有 `probing` 单飞守卫（持锁置位，HTTP 并发探测与 tick 探测不再 N+1 轰上游，命中放弃本次最长推迟一个 300ms tick）；HTTP 侧 `ProbeForAccount`/`ProbeNow` **未接入 probing/lastProbe 节流**——精确触发窗口已推演：正常时序 30s 探测间隔 < 快照 40s TTL 几乎从不过期，最坏 15s 超时挂起 + 快照过期拉宽到 ~25s 且恰逢前端刷新才直打一次（频率"每 30s 周期至多多 1 次"），不足以触发平台熔断；接入单飞会破坏"管理员刷新强制拿最新数据"语义 + 临门 2s 盯守期误命中卡顿。**维持观察不修，触发条件已精确化，未来平台更敏感再落地**。
- **第 7 轮 B7-C4**：未知 `/api/xxx` 由 `Register` 的 `mux.HandleFunc("/api/", ...)` 显式 404（JSON body + HTTP 404）——绝不再落入 `mux.Handle("/", SpaHandler)` 的 SPA 兜底返回 200 HTML。**第 12 轮 F12-B2 探测单飞守卫**：`probing` 在途标记——probe() 最长耗时 FindElectives 网络往返（15s 超时），HTTP 侧快照过期并发的 ProbeForAccount/ProbeNow 会与 tick 探测同时打上游，N 账号开窗全期形成 N+1 并发 findElectivesData；命中 `probing` 直接放弃本次（最长推迟一个 300ms tick，临门/黄金期无实质损失），任何返回路径统一复位。**第 17 轮 F17-01 补齐（probe() 内部 per-account 并发也封顶）**：probe() 内每目标账号 goroutine 调 ProbeForAccount **不受 probing 单飞保护**（守卫只护全局主体），临门/开窗期 probeIntervalNear **2s 周期**（B15 修正前误记 30s）下 N 账号变每 2s N+1 并发 findElectivesData 直打上游（"访问过于频繁 1 分钟熔断"实证契约冲突）——新增 `probeSem` 结构化信号量（cap 4）封顶 per-account 并发，峰值 N→4，跨批受同一信号量约束绝不叠加，全局 FindElectives 主体不受影响（per-account 全部入场后才执行）。
- **前端目标自动保存只由用户改动驱动（第 7 轮 F7-01 CRITICAL）**：Select 页 `rev` 计数只归 pick()/用户操作自增（依赖不含 publishes）；轮询拉回的 `publishes` 经 `publishesRef` 读取——窗口开启瞬间平台短暂清空 publishes 时**绝不**触发 `{"targets":[]}` 抹除服务端/调度器目标（黄金期空转 + 窗口关闭后目标永久丢失已根治）。发布列表收缩 ≠ 用户意图清空目标。**假清空守卫链（F15-01/F16-01/F17-02）**：目标集由 `[publishes × selected]` 联查构建，任一为空即 targets=[]。F15-01 判据渲染期 `publishesMissing`（400ms 后读旧闭包）；F16-01 `targetsUseCurrentPublishes` 消费时刻全数校验 publish_id 必属当前 publishesRef（防抖+flush 双闸）；**F17-02 补漏**——every 校验对**空 targets 恒真**，消费时刻另加"selectedCount>0 却构建出空集 = 假清空"守卫（selectedCount 只随用户改动所在渲染更新，只会偏保守绝不放过真实假清空）。改 Select.tsx 保存逻辑前先看此处。
- **日志系统与窗口关闭防御（2026-09-13 全链路补齐）**：
  - **探测日志**：`probe()` 每次成功输出"探测成功：%d 个发布，窗口状态 %v（已关闭 %v）"；`ProbeForAccount` 空快照输出"探测返回空课程快照（选课窗口关闭或学期无发布）"——"课程为什么为空"从日志一眼可查。
  - **窗口关闭状态暴露**：`SchedulerState.WindowClosed`（json `window_closed`）——快照为空且从未开过窗=已关闭，随 `/state` 下发；`handleState` 已透传。
  - **窗口关闭后探测降频**：`probeIntervalFor` 对"开放时间已过 + WindowClosed"降回 30s，窗口结束后 2 秒高频盯守浪费请求且刷屏日志；管理员热改开放时间到未来（新一轮）时临门 2s 盯守不受影响。
  - **自动重登日志**：触发输出"触发自动重登（原因：教务 token 失效，连续失败 N 次）"、成功输出"自动重登恢复（新 token 前8位...）"脱敏、失败输出原因、退避窗口过再试也有日志。
  - **登录全链路日志**：`zhidao.Login` 每次识别成功输出"第N次验证码识别成功（引擎 %T，识别 N 位字符）"、识别失败/提交被拒输出第 N 次与原因、全部失败输出"登录失败（共 3 次识别尝试）"收尾——引擎切换、重试次数、失败原因全可见。
  - **不打印敏感信息**：所有日志不含明文密码与完整 token（token 只显示前 8 位）。

### UI 设计系统规范（纯黑白极简艺术 + 瑞士国际排版规范）
- **设计哲学**：彻底清除任何喧宾夺主、聒噪浮夸的技术宣传口号（如“毫秒级并发”、“智能Vision识别”等广告横幅），全面转向**纯黑白极简艺术风格（Monochrome Fine Art）**，致敬瑞士国际平面排版与现代高奢画廊策展美学。
- **色彩与材质**：
  - 画布底色：绝对纯粹的极夜深黑 `#000000` 与沉浸式深暗 `#09090b`，彻底驱逐一切彩色荧光与渐变光晕；
  - 边框与线条：1 像素精致冷灰发丝线（`rgba(255, 255, 255, 0.12)` / `border-neutral-900`），利落沉稳；
  - 文字排版：冷冽纯白 `#ffffff` 作为最高对比度焦点，中性冷灰 `#a1a1aa` / `#71717a` 承担次要与辅助信息；
  - 单色反差原则：主操作按钮采用经典的纯白反转底配纯黑文字（`bg-white text-black`），状态指示均采用无色彩的单色徽章，彻底移除彩色 emoji 表情包。
- **排版几何规范**：
  - 倒计时与监控指标采用等宽数字（`tabular-nums`），犹如机械雕刻般严整稳固；
  - 极简几何圆角（4px - 8px），摒弃臃肿膨胀的大圆角与花哨阴影，保留建筑般的硬朗质感。

### 关键决策与系统化调试排错记录
- **窗口开启后的官网同款手动报名/退选（2026-09-13 落地，与自动抢课并存）**：选课窗口**开启前**保持自动预选逻辑；**开启后**用户可在选课大厅按官网交互手动报名/退选（btn_type=1 显示红底退选按钮、btn_type=2 显示白底报名按钮、can_select 控制可点性），操作后前端 2 秒动态轮询同步全量状态。后端新增 `POST /api/electives/select`（报名）与 `POST /api/electives/select/exit`（退选），请求体 `{classId, courseName}`，管理员会话可带 `?account=` 穿透到任意账号、普通会话强制绑定额定账号。两接口与调度器通过**排他互斥**协同：`TryAcquireSubmit` 抢在飞锁（防手动与自动并发重复发包），成功回调 `MarkDone`（写 success 表 + 清 full/rateLimited + 置 success）、退选回调 `RemoveDone`（删 success + 置 pending）、满员回调 `RemoveFull`。平台业务错误文案原样下发前端提示。**契约**：窗口已关闭时平台返回"无效的课程ID"，调度器按满员记入 full 集不再轰炸。
- **`btn_type` 状态机落地规范（官网逆向契约）**：btn_type 与 can_select 正交——btn_type 决定"点击后的动作"（1=退选、2=报名），can_select 决定"是否可点"（false 时按钮禁用并展示 title 承载的禁用原因）。`hasSelected`/`canSelect` 为发布级计数（已选/上限），`selected_count`/`max_count` 为课程级名额。前端 2 秒动态轮询仅在窗口开放（in_date_range）时启用，关闭后回落到 10 秒。
- **调度器对外新增 4 方法（手动选课协同，供 api 层调用）**：`TryAcquireSubmit(acct, classID)` 返回一次性 release（inflight 在飞互斥，冲突返回 false 提示"该课程正在提交中"）；`MarkDone(acct, classID, courseName, msg)` 写 done + 清 full/rateLimited + 置 success + SaveSuccess + AppendLog；`RemoveDone(acct, classID)` 删 done + 置 pending + AppendLog("exit")；`RemoveFull(acct, classID)` 清除满员标记（手动退选后重选先解封）。spawnChain 提交前增加 inflight 去重——手动提交进行中的课程自动链跳过，杜绝双发包。**第 8 轮 B8-M2 + 第 9 轮 B9-02 + 第 10 轮 B10-01/B10-03 加固**：`RemoveDone` 必须先 `DeleteSuccess(acct, classID)` 删 success 表行再 `SaveRefused(acct, classID)` 落库 refused 表再 AppendLog——B8 防重启时 `RestoreDone` 把已退选课恢复成"已报名成功"（手动退选后重启假成功）；B9 防重启后 refused 内存丢失、恢复路径把自动引擎重新抢回用户退选的课。**重启恢复顺序契约（main.go，B10-01 修正）**：恢复目标**必须用专用 `RestoreTargets`（与 `SetTargetsForAccount` 唯一区别是不清 refused 内存+库行）**，顺序为 `RestoreDone(success)` → 逐账号 `LoadTargetsForAccount`+`RestoreTargets` → `LoadRefused`+`RestoreRefused`（RestoreTargets 不清库行，LoadRefused 仍能读到全部退选）——此前误用 `SetTargetsForAccount` 做恢复，其内部 `DeleteRefused` 删库行 → LoadRefused 读到空 map，重启后手动退选记录全丢、自动引擎重新抢回（B9-02 被恢复顺序抵消）。`SetTargetsForAccount` 只在**用户主动重设目标**时调用（其内部同步 `DeleteRefused` 清库=重新接管）。**refused 优先于 done（B10-03）**：重设目标时先判 refused 再判 done——课程曾被报成功后手动退选，done 的"重启恢复：已报名成功"文案会掩盖退选意图，refused 优先强制 pending"已手动退选（自动引擎不再接管）"。`DeleteAccount` 事务同步清 refused 行。
- **管理员后台（admin 账号 + 管理口令，会话级鉴权）**：`record admin` + `XUANKE_ADMIN_TOKEN` 比对（ConstantTimeCompare）→ `session.CreateAdmin` → 后续全部管理接口走 `requireAdminSession`（Bearer 会话）鉴权。入口：登录页账号填 `admin`、密码填管理口令（绕过教务登录，免平台限流）。管理接口全部挂 `/api/admin/*`：`config` GET/PUT 热改 + SaveSettings 落库、`stats` 运行状态、`codes` 激活码生成/列表/删除、`accounts` 账号管理（禁止删 admin）、`logs` 全量日志。普通用户会话访问一律 403。前端 admin 登录后进独立管理页（routes/Admin.tsx，五 Tab：激活码/配置/状态/账号/日志）
- **运行时热配置中心（runtime.Store，全部热重载免重启）**：管理员改动立即进 `runtime.Store`（RWMutex + Get 快照拷贝 + Update 闭包）。生效链路——激活码开关（`activationEnabled()` 三处登录/激活/生成读取）、Vision url/key/model（`Accounts.SetVision` 推全部客户端）、开放时间（`sched.SetOpenTimeFn` 调度器逐 tick 读取）；PUT 同步 `SaveSettings` 全量落库（settings k/v 表），重启后 LoadSettings 覆盖环境变量恢复。敏感值回显脱敏（`maskKey` 只显 `****`+后4位）
- **教务 token 失效自动重登（取代早期"禁止自动重登"）**：doRequest 对 code=-1 返回 `ErrUnauthorized` 本身不重登；调度器探测命中该错误时按账号标记失效并异步自动重登（防重入 + 30 秒节流，Vision 持续失败不轰炸登录接口），成功后新 token 落库（UpdateIDToken）+ 立即补一次探测。**第 8 轮 B8-M7 + 第 9 轮 B9-01 + 第 10 轮 B10-02 完整化**：手动**报名**与**退选**路径命中 `ErrUnauthorized` 同样触发自动重登（调度器导出幂等别名 `MaybeRelogin` 供 api 层调用）——但**只调 `MaybeRelogin`，绝不先调 `MarkTokenValid`**（后者只该用于"手动登录成功"的 issueSession 恢复路径；在手动操作分支会 `delete(reloginFail)` 击穿指数退避，Vision 持续故障时退避恒从 30s 重来、平台锁号防线失效）。B9-01 只修了报名分支，B10-02 补上退选分支漏删的 `MarkTokenValid`（B9-01 半成品根治）。网络类失败绝不重登。前端 `/state` 只读 `token_valid` 显示有效性（有效 / 已失效·自动恢复中），不显示次数与时间。**第 12 轮 F12-B3 锁纪律对齐**：`tokenValid` 失效标记在决策段置位（与发起同持 `reloginMu→s.mu` 两把锁，语义"发起重登即 token 已知失效"），`MarkTokenValid` 对齐锁序补取 `reloginMu`——此前置位在重登 goroutine 开头（只持 s.mu），手动登录成功清标记后会被在途重登覆写回 true，/state 短暂闪动；现在两条路径串行化杜绝半态读。
- **登录验证码重试收敛（防空炸平台限流）**：`zhidao.Login` 三层上限——识别最多 3 次（识别失败/识别结果为空/提交被拒均刷新验证码重试）、提交最多 2 次（提交被拒多为验证码过期）、初始化会话/取验证码/网络/配置错误一律立即返回。杜绝旧版"识别 10 次"引发的"登录失败次数过多，请 30 分钟后重试"平台熔断
- **会话复用与完整 Cookie 注入**：登录成功（Login）后自动提取服务端下发的所有会话 Cookie（尤其是 `access_limit_cookie` 与 `zd_edu_cookie`），若未下发则注入默认保护 Cookie。API 请求严格遵循 idToken + Cookie 双通道机制，避免服务端报 code=1 鉴权缺失
- **课程探测 30 秒节流（根因修复"选课大厅突然啥都没了"）**：调度器对 `findElectivesData` 的成功探测加 30 秒最小间隔（`probeInterval` 常量），探测成功或失败均记录时间戳，网络故障时不会 300ms 疯狂重试；窗口未开启时也绝不高频轮询，从根因消除平台"访问过于频繁"1 分钟熔断导致的课程列表拉空。轮询 ticker 仍为 300ms（负责窗口开启后的**立即**探测与提交），但探测动作本身被 30 秒节流闸门挡下
- **按账号各自 Token 专属查询与年级隔离快照（彻底废弃单一全局快照，根因根除年级串线）**：至道教务平台按学籍年级动态下发课程发布（高二下发高二体育+校本1+校本2共82门课，高三仅下发高三体育1门课）。旧架构使用单一全局快照加盲目探测，导致首个高三账号的快照覆盖全校产生严重串线。现已全面重构为：调度器按账号隔离维护 `acctData` 与 `acctDataAt` 映射表，提供 `ElectivesSnapshotFor(acct)` 与 `ProbeForAccount(acct)` 专属方法；后台定时探测与开窗冲刺并发为每一个配置了目标的账号独立刷新年级快照；满员检测 `classFullInSnapshot` 优先对齐本账号年级名额。
- **全链路多账号独立维护（前端至后端彻底打通）**：管理后台「账号管理」为每个学生账号提供专属「选课大厅」入口，点击即可无缝进入该学生名下的独立选课大厅；`/api/electives?account=xxx`、`/api/targets?account=xxx`、`/api/state?account=xxx` 全面支持目标账号透传，管理员未传参时自动对齐首个有预选目标的核心账号（绝不再盲目抓取首个高三测试账号）；前端选课大厅顶部明确显示当前维护账号并隔离缓存与自动保存。
- **多账号物理隔离 + 会话级账号绑定**：每个账号独立 `zhidao.Client`（账号 A 绝不携带账号 B 的会话），认证后服务端签发随机 Bearer 会话令牌（12h TTL），所有租户接口从会话读取账号（`sessionAccount(r)`）——忽略客户端传入的账号参数，`/state`、`/targets`、`/electives/detail`、`/electives` 均按会话账号隔离。
- **部署访问口令 gate + 移除硬编码密钥**：`XUANKE_ADMIN_TOKEN` 必须来自环境变量或 `data/.env`（缺失直接 `log.Fatal` 拒绝启动），激活码管理接口用 `crypto/subtle.ConstantTimeCompare` 恒定时间比对；`SF_API_KEY` 也走同源注入。登录限流每分钟 5 次（token bucket，每 IP）
- **激活码鉴权（取代登录口令 gate）+ 可开关**：登录不再校验部署口令；教务登录成功 → `IsActivated` 未激活返回 `code=1001`（前端据此弹出激活码输入模态框）→ `POST /api/activate {account, code}` 事务内扣减激活码次数 + 记录 `activations` → 签发会话。激活一次永久免激活。激活码由 `X-Admin-Token` 管理接口生成（`XK-XXXX-XXXX-XXXX-XXXX`，4 组共 16 位十六进制，POST count 1-100 × uses≥1 / GET 列表 / DELETE）。开关：`XUANKE_ACTIVATION=off` 完全禁用（登录直接签发会话、activate/admin 接口拒绝）。**票据契约（第 11 轮 F11-A1 贯通）**：1001 响应体带 `data.ticket`（绑定本次登录账号的短期单次票据），前端**必须随激活请求回传**（`{account, code, ticket}`），后端 `handleActivate` 对 `req.Ticket==""` 直接拒绝——此前前端丢弃票据，激活码机制整链不可用。实现通道：`ApiError` 携带响应体 `data` 字段透传票据（抛错处不丢数据）。**票据单次防重放（第 13 轮 m1 决策落盘）**：`handleActivate` 先 `ConsumeTicket` 再校验激活码——**错误激活码会把票据销毁，用户必须重新登录拿新票据**。这是刻意安全设计：宁可输错激活码重登一次，也不让同票据在 5 分钟 TTL 内反复探测不同激活码（防穷举/防占用）。前端 F12-M3 已引导"票据过期/已用请取消后重新登录即可进入"。**未修复项：激活码本身可被重放探测**（同一码对多账号）；受码不可枚举（16 位 hex）+ 速率限制保护，接受为残留风险。
- **多备选课程 + 满员人数对比退避**：每个发布可配置多门备选目标，`targets.priority` 持久化排序；调度器每（账号×发布）一条 goroutine 链（`spawnChain`）按优先级依次尝试，快照满员跳过 → inflight 去重 → `SelectClass` 失败后 `IsClassFull` 实时复核人数（`selected_count >= max_count`），真满才 `markFullLocked` 切下一备选，网络类失败终止本链下一 tick 重试
- **日志按账号隔离**：`task_log.account` 列 + `AppendLog(acct, ...)` + `LoadLogs(acct, limit)` WHERE 过滤，`/api/logs` 只返回当前会话账号自己的日志，账号间不可互通查看
- **凭据与敏感配置 AES-GCM 严格加密入库（彻底移除旧明文兼容）**：密码与 settings 表的 vision_key 均经 AES-256-GCM 加密后存入（带有 enc: 密文前缀，XUANKE_MASTER_KEY 环境变量或 data/.master_key 提供主密钥）。secureEncrypt 未注入加密器直接报错拒绝，杜绝明文入库；服务启动加载 settings 时严格校验 enc: 前缀并解密还原，若读取到未加密旧明文直接打印警告并拒绝加载，坚决执行拒绝旧版畸形数据的策略
- **不兼容旧数据库（拒绝启动）**：`db.Open` 检测到旧 `account` 表或 `targets.account=''` 空账号行即报错拒绝启动，提示删除 `data/xuanke.db` 重建
- **超高性能架构**：窗口开后首个 tick 立即探测（豁免 30s 节流）、每账号每课程独立 goroutine 并发提交、`/api/electives` 直读内存课程快照（snapshotTTL 40s > probeInterval 30s，页面浏览零上游请求）
- **学期列表容错与自动平滑回退（Fallback）**：`FindElectives` 在尝试获取可选学期列表时，若因特定时段或接口异常导致学期列表返回错误（如 code=1），自动回退并直接请求默认激活学期数据（`findElectivesData` 传空体），杜绝选课大厅因非核心接口报错而白屏或崩溃
- **预选目标课程支持随时清空（0 门合法）**：`handleSetTargets` 解除“至少需要 1 门”的死锁限制，允许用户重置清空全部目标；同时本地数据库严禁注入测试课程，保证新账号登录时绝对干净空白
- **UI 设计系统规范（纯黑白极简艺术 + 瑞士国际排版规范）**：彻底清除任何喧宾夺主的技术宣传口号（如“毫秒级并发”、“每个发布批次锁定1门心仪目标·秒级抢报”、“目标阵容”等吵闹词汇），全站统一为纯黑白极简高级艺术设计（纯黑 `#000000` 底色、发丝灰边框、纯白高对比文字与单色等宽数据）
- **水墨画布背景（全站含登录/管理页）**：`web/public/bg.jpg`（来源用户桌面 boqi.jpg）经 Vite 打包进 `backend/web/dist` 再 `//go:embed` 进单二进制。实现：App.tsx 根部渲染 `<div className="canvas-bg">`（fixed 钉视口 + `overflow:hidden`），内含**真实 `<img className="canvas-bg-img">`**（图片本体，便于 `getBoundingClientRect` 量测真实渲染高）与 **`.canvas-bg-mask`**（独立深黑渐变遮罩，不随图片位移，保证白字任意亮度可读）。**滚动表现**（用户明确要求）：全端图片本体 fixed 容器内不随文档滚动；移动端 180% 宽贴顶，桌面端（`@media (hover:hover) and (pointer:fine)`）160% 宽；桌面端图片本体 `transform: translateY(var(--bg-shift,0px))` 由 App.tsx useEffect 按 `scrollY` 写入——**位移封顶量 = 图片真实渲染高度 − 视口高**（实测量测，杜绝凭 1.6 系数估算），边滚边露出图片下方、露到底边即冻结，再滚整张图不动；移动端不消费该变量恒 0 贴顶。**堆叠层级关键**：背景 `z-index:0`，路由内容包在 `.app-content`（`position:relative; z-index:1`）里——不可用 `z-index:-1`（在含 fixed/Portal 堆叠上下文的页面上会被内容层的实色背景盖住，已用 test-a/test-b 隔离实验验证）。`#root` 必须 `background-color:transparent`，html/body 保留 `--bg` 纯黑底色兜底
- **磨砂玻璃组件（.glass / .glass-strong 工具类）**：`global.css` 定义——`rgba(255,255,255,0.06)` 半透明白底 + `backdrop-filter: blur(16px) saturate(1.2)` 毛玻璃 + 1px 发丝边框；`glass-strong`（`rgba(18,18,22,0.72)` + blur(20px)）用于选中态卡片与底部固定栏等需强可读性面板。选课大厅/控制中心的卡片、搜索栏、标签页、底部栏全部换用
- **选课大厅人性化便利**：顶栏新增选课开放倒计时条（`/state.open_time` 秒级跳动，窗口开放显示"窗口已开放"脉动点）；搜索栏新增"按剩余排序"（名额少靠前）+ "仅看有余量"双筛按钮；课程网格桌面 3 列 → `lg:3 xl:4` 列更密集，卡片内边距 `p-3.5` 紧凑化
- **管理员配置语义（第 7 轮 F7-02）**：`PUT /api/admin/config` 的 `open_time=""` = **显式清空开放时间**（内存/落库/回显三处对齐，调度器解除窗口机制）——此前空串被 format 判错静默忽略导致"清空保存"假成功；前端保存成功后 configEpoch 自增触发回填，表单不再停留在与生效配置分叉的旧值。**第 11 轮 B11-A1 补齐（提交挂起守卫）**：open_time 清空后 `runtime.reparse` 把 `OpenTimeParsed` 置零值 `time.Time`，tick 的 `!opened && !now.After(open)` 对零值 open 恒 false → 提交循环**永续放行**，对"已满员/已成功"目标每 1s 仍刷平台报名接口（绕过 C-3 窗口关闭防轰炸）。修复：tick 提交段在 `open.IsZero()` 时直接 return——窗口机制被显式解除 = 挂起提交，绝不放行。TDD：`TestSubmitSuspendedWhenOpenTimeCleared`（红灯"调用 1 次"→绿灯"0 次"）。
- **第 8 轮前端健壮性收尾（F8 系列）**：F8-01 401 踢除账号的 **localStorage 落盘必用快照模式**（外层取 `sessions` 快照 → 判断 → `saveSessions(next)` → `setSessions(next)`），**绝不用 updater 外闭包标志位**（React 并发下 updater 运行前 flag 已被求值 = 恒失真，踢除动作不落盘 → 刷新后失效账号从 localStorage 复活反复 401 轰炸）；F8-04 Dashboard 倒计时收敛 `useTickingCountdown` 自 tick 组件（每秒只重建倒计时一处，消灭整页 setTick 重建）；F8-03 管理员删除账号后立即 `invalidateQueries(["admin-accounts"])`（删除即时生效不靠 10s 轮询兜底）。**第 9 轮 F9-07**：`useTickingCountdown` 收敛到 `web/src/lib/` 共用（Dashboard/Select 同一实现，倒计时只重渲染消费处）；**轮询降频闭包澄清（F9-05 误报）**：refetchInterval 读组件闭包 state/stateData 是新鲜的——`/state` 查询数据更新触发组件重渲染，react-query 用最新闭包重调度轮询间隔，**绝不存在"闭包停旧值永不降频"**；logs 降频读 `state?.window_closed`、electives 降频读 `stateData?.window_closed` 均正确（此前误改的 `query.state.data?.window_closed` 是错的——ElectivesData 无该字段）。**第 10 轮 F10 系列**：F10-02 `pick()` 的 `toast` **移出 `setSelected` updater**（updater 必须纯函数——StrictMode 双调、并发渲染丢弃重放会让弹窗重复弹出；改为事件处理器内先算 next 快照再 setSelected，两次独立点击间有渲染提交，与函数式更新等价）；F10-05 401 事件 detail **同时携带 `{account, session}`，App 归属判定一律以 `detail.session` 为准**（`?account=` 穿透目标只在 session 缺失时兜底）——管理员代理看学生大厅时 URL 挂的是学生名，若优先取它则管理员自身会话过期后反查落空、卡死在代理页反复 401；F10-06 去掉 Dashboard"3 门"旧约束残留（后端上限 100 门且每发布可配多条备选）；F10-07 `useTickingCountdown` 的 target 变化时补 `setNow(Date.now())`（注释"回到 now"落实到实现）；F10-04 client.ts 超时注释"2 秒"改正为"20 秒"并说明保持权衡（`api()` 是全站共用通道，/electives 大列表在开窗黄金期响应偏慢，收紧到 2 秒会掐断刷新；目标保存有指数退避重发兜底）。**第 12 轮 F12-M 系列**：F12-M1 选课大厅退出前 flush 挂起的目标保存（返回按钮先 `flushTargets` 再 onDone——400ms 防抖 timer 随卸载被清，最后一次点选到返回间隔 <400ms 时整批目标永不 PUT；复用 lastJson 去重 + savingRef/dirtyRef 串行化，绝不与飞行中 PUT 乱序）；F12-M2 Admin 五 Tab 受控化（非受控 defaultValue 只在首次挂载生效，进出学生大厅后 Tab 恒回落"激活码"，提升 value+onValueChange 跨挂载保留）；F12-M3 激活失败分场景引导（票据 5 分钟单次，过期/已用后账号其实已激活，捕获"激活票据无效或已过期"提示取消后重新登录即可进入并清 pendingTicket）；F12-M4 `--fg-muted` 对齐次要冷灰 `#a1a1aa`（此前误标纯白 #ffffff 与 --fg 同色，Card 描述/TabsList/Table 表头/Toast 描述/outline 按钮全站次要信息失去视觉层级差）、`--fg-dim` 补灰 `#71717a`。**第 13 轮 F13 系列**：F13-C1 flushTargets 整包抹目标根治（rev===0 不 PUT——进页数据未就绪/窗口关闭 publishes 恒空时返回即抹空后端目标，F7-01 契约不可侵犯；清空目标仍 rev>0 合法落库）、F13-C2 卸载后孤儿重试链根除（unmountedRef 守卫 saveNow/scheduleRetry，卸载后绝不再发起/重试/轰炸 toast）、F13-M2 Admin Tab 受控化注释对齐（仅 Tab 间保留不跨挂载，需跨挂载提升 App 层/localStorage）、F13-M3 票据过期清空激活码输入防晦涩报错（无票据时点激活引导重新登录）。**第 14 轮 F14 系列**：F14-03 选课大厅空态兜底（publishes 恒空→tabs.length===0 时渲染"当前无可选课程批次（窗口未开放或已关闭）"提示卡 + 徽章分母 length>0 才显示不再 n/0）。
- **第 14 轮后端（B14 系列）**：B14-M1 TestWindowOpenSubmitsWithoutProbeReset 契约修正（原 openTime 未来 1 小时 + tick 守卫 702 行 `!opened && !now.After(open)` 恒 return 致首段断言恒红 3 秒超时；改过去 1 秒后守卫放行提交路径而探测仍被 lastProbe 节流挡住，真实覆盖"提交不依赖探测节流"）；B14-M2 TestAdminStatsWindowOpenedUsesScheduler 后半段补"与 WindowOpened() 同源"断言（原 `_ = st` 丢结果 + 注释与 mock 数据 inDateRange=true 矛盾，stats 契约从未被验证；断言随执行顺序漂移——ProbeNow 走全局主循环、本次 tick 已把 WindowOpened 刷回 false，故断言"stats === WindowOpened()"对齐实际探测状态）；B14-N1 管理端日志账号硬编码擦除（AppendLog("admin") → d.AdminNameValue()，管理员改名后日志不再错位）；B14-I1 删除测 map 语义的无效测试段（手写 reloginFail=5 再 delete 断言 0 恒绿，F13-i1 同链条收敛）。**测试假象双实证**：B14-M1 恒红、B14-M2 恒绿——修改 tick 守卫/探测时序前务必先跑 `go test -run 'TestWindowOpenSubmitsWithoutProbeReset|TestAdminStatsWindowOpenedUsesScheduler'` 确认它们真在测"提交与探测解耦"与"stats 与探测状态同源"。**第 15 轮（B15 系列）**：B15-M2 open_time 零值探测降频（probeIntervalFor 入口 IsZero 返 30s——零值 open 对 `now.After(open.Add(-nearWindow))` 恒 true 落临门 2s 且快照非空时 WindowClosed 恒 false，探测永久 2s 轰炸 findElectivesData，提交已被 B11-A1 挂起须同步降频；TDD TestProbeIntervalZeroOpenTime 红灯 got 2s→绿灯）；B15-M4 管理员透传目标孤儿行（handleSetTargets 对 ?account= 任意串无条件 SetTargetsForAccount → store.targets 无主行 + 重启幽灵复活 + 调度器永不执行；透传前以 LoadCredentials 凭据表校验账号真实存在，不存在整体拒绝；TDD TestSetTargetsUnknownAccountDoesNotFabricate，判据历经 ListAccounts 误伤 authenticateDirect 已登录账号改凭据表）；B15-M5 管理员删除空格账号假成功（handleAdminDeleteAccount 只 TrimSpace 判空，前端空格失手 trim 后删真账号还显"已删除"；修 trim 后必须与原值逐字节一致否则整体拒绝；TDD TestAdminDeleteRejectsUnnormalizedAccount）。**前端第 15 轮（F15 系列）**：F15-01 flushTargets 与防抖双重"发布缺席+已有选中=假清空"守卫（rev>0 但 publishes 空时 PUT [] 会永久抹除已落库目标，与 F7-01 同源，防抖与 flush 两条腿都补）；F15-02 saveNow 卸载后失败也绝不 toast（F13-C2 对齐）；F15-03 窗口横幅状态机修正（window_closed 优先→window_opened→isExpired→倒计时，此前含 isExpired 使窗口关闭后恒显示"已开放"与 F14-03 空态卡矛盾）；F15-07 登出清除代理目标账号 setTargetAccount(null)。**第 16 轮（B16/F16 系列）**：F16-01 目标保存"发布 id 漂移"假清空（publishesMissing 是渲染期常量，防抖回调 400ms 后读旧闭包值；开窗瞬间平台清空恢复致发布集合整体重建 id 全变，build() 拿最新 publishesRef 联查 selected[旧 publish_id] → 全 undefined → PUT 空 targets 抹掉后端目标；抽 targetsUseCurrentPublishes 消费时刻全数校验 publish_id 必属当前 publishesRef，防抖+flush 双闸，任一漂移置脏跳过）；B16-M1 退避截止基准与读侧统一对齐钟（markRateLimitedLocked 写入改 nowAlignedLocked，此前本地钟写入 vs 对齐钟判期两套时间基边界同一语义）；B15-M1 inflight 所有权竞态深查**不成立**（所有 inflight 访问持 s.mu，网络调用在锁外，sync.Once 幂等释放）；B15-M3 观察时间基准混用收敛为 B16-M1 已修；HTTP 侧探测单飞维持观察（B14-I2/B16-M2 延续）。**第 17 轮（B17/F17 系列）**：**F17-01 MAJOR** per-account 探测 N+1 并发封顶（probe() 内每目标账号 goroutine 调 ProbeForAccount **不受 F12-B2 probing 单飞保护**——守卫只护 probe() 主体全局 FindElectives；临门/开窗期 probeIntervalNear **2s 周期**（B15 修正前误记 30s）下 N 账号变每 2s N+1 并发 findElectivesData，与"访问过于频繁 1 分钟熔断"实证契约冲突；修：新增 probeSem 结构化信号量 cap 4 封顶 per-account 并发，峰值 N→4，跨批受同一信号量约束绝不叠加，全局主体不受影响）；**F17-04** .master_key 损坏文件显式报错（LoadOrCreateKey 只校验环境变量长度、预生成密钥文件原样返回任意字节——AES 初始化失败 store 无对账致凭据静默不可读，新部署会用随机 32 字节覆盖损坏文件使库永久不可解密；文件路径同校 32 字节拒绝带伤启动，TDD 红灯→绿灯）；**F17-02** 目标保存"空集假清空"最后拼图（F16-01 的 every 校验对空 targets 恒真恒过，且 publishesMissing 是渲染期常量 400ms 后读旧闭包——窗口关闭 publishes 恒空时用户清空再添加目标→build 产出 []→PUT [] 抹掉后端目标；修：防抖回调与 flushTargets 两处消费时刻补"selectedCount>0 却构建出空集=假清空"守卫，仅极限场景 selectedCount 滞后误伤、安全方向绝不放过真实假清空）。R17-03 壁纸 CSS/JS transform 分叉**误报澄清**（App.tsx 每次滚动直接写 inline transform，优先级恒高于 CSS 的 var(--bg-shift) 兜底默认——滚动机制实际正常）。观察项：17-02 RemoveFull 死方法、17-03 WindowClosed 误标降频（行为可接受）、17-05 Decrypt 死字段（B16-I1 延续）、17-06 probe per-account 错误静默（频率受 30s 节流约束）。**第 18 轮（B18/F18 系列）**：**F18-01 CRITICAL** Select.tsx 死代码常量致前端构建中断（F17-02 把假清空守卫判据改为消费时刻实时判据后，渲染期常量 publishesMissing 遗留成死代码，TDZ 引用声明于其后的 publishesRef/selectedCount 触发 tsc -b 5 错、npm run build 退出码 2——CI 与 Tag 发布流水线必然红；删除即绿。**警示**：项目根 tsc --noEmit 是 references 空壳不报错，只有 npm run build（tsc -b）真校验，前端回归一律以 npm run build 为准）；**F18-02 MAJOR** 配置加载前保存覆盖真实配置（ConfigTab 在 configQuery 返回前保存按钮可点，初始空值 baseUrl=""/model=""/openTime=""/activationOn=true 整体覆盖生效配置，open_time 清空即 B11-A1 调度器挂起提交+激活码误开；按钮 disabled={saving||!loaded} + save() 入口守卫 + 加载中提示）；**F18-03 MINOR** 选课大厅 Tabs 非受控 defaultValue 发布重建后悬空（改受控 value 跟随 tabs[0] 兜底）；**B18-M1 MAJOR** 窗口关闭防轰炸漏洞（isWindowClosedError 只匹配"关闭/未开启/报名时间/已结束"，真实平台关闭文案"无效的课程ID"不命中→不记 full→实时复核路径 countList 空报"课程无人数数据"→未成功目标每 1s（黄金期 250ms）永续轰炸报名接口，现有测试注入"已结束"文案掩盖真实形态；修① tick 提交守卫加 `if s.WindowClosed() { return }`（与 B11-A1 零值守卫并列）② WindowClosed 判定升级"至少开过窗"前提——prevOpened 先捕获再覆写（顺序 bug 实证：先覆写后捕获读到恒为本轮值，三个窗口关闭契约测试全红），未开过窗即空快照（学期无发布/平台异常）不算关闭防黄金期误挂起）；**B18-M2 MAJOR** 删除账号与在飞 spawnChain 网络往返竞态写回 success 行（DeleteAccount 清 credentials/accounts/targets/success 表 + Accounts.Remove 与在飞链 SelectClass 最长 15s 竞态，成功分支 SaveSuccess 把已删账号行写回、重启后重新登录被 RestoreDone 恢复成"已报名成功"假状态；修复写 done/落库前锁内复核 `s.clients.ClientFor(acct)` 仍存在，已删放弃落库——与重登完成路径 891 行同款防线，TDD TestDeletedAccountInFlightDropsSuccess 红灯 1 行→绿灯 0 行）；**B18-m1 MINOR** CheckClassSelectable 快照过期无 TTL 校验（手动报名复核只判 data==nil，40s 旧快照仍按旧数据拦截真实操作；以 acctDataAt 时间戳判过期，过期一律按"无快照"放行由平台把关，与 ElectivesSnapshotFor 过期回退语义对齐；TDD 红灯"该课程已满员或不可选"→绿灯）。**窗口关闭状态契约（B18-M1 升级）**：WindowClosed = 至少开过窗 + 空快照 + 开放时间已过；tick 提交守卫顺序 open 零值 → 未开未到点 → WindowClosed → lastSubmit 节流；**测试预置关闭状态必须用空快照形态（fc.data.Publishes=nil），仅置位会被首 tick probe 覆写**。**第 19 轮（B19/F19 系列）**：**B19-01 MAJOR** 幽灵窗口 2s 高频盯守烧平台（WindowClosed 为 false 时 probeIntervalFor 对"开放时间已过+从未开过窗"空快照形态仍落临门 2s——平台窗口从未开启/已关闭且从未被探测确认，探测永续 2s 轰炸 findElectivesData；修：WindowClosed() 兜底判定 `syncFailStreak>=3`（时钟连续失败=平台不可达信号）且 openTimeNow 非零即视幽灵窗口返回 true，提交挂起+探测降回 30s；新增 syncFailedWindow 字段**写而不读**仅留档，判据读 syncFailStreak，时钟成功即清；黄金期不受影响）；**B19-02 MAJOR** 删账号状态残留 + 重设目标误清持久历史（handleAdminDeleteAccount 只 SetTargetsForAccount(nil) 残留全部内存态+状态.Courses 行，重启后 refused 幽灵复活；而 SetTargetsForAccount 内部 delete(done) 与 **RestoreDone 跨目标持久历史契约冲突**（TestRestoreDoneSkipsResubmit 实证回归红）；修：新增 PurgeAccount(acct) 全量清理（含状态.Courses 过滤）删除路径改调它；**SetTargetsForAccount 只清 refused，绝不清 done/full/rateLimited/inflight**——done=跨目标持久历史、full/rateLimited=真实满员/风控退避、inflight=防并发双包）；**B19-03 MAJOR** 实时人数复核命中 token 失效未触发自动重登（classFullRealtime→IsClassFull→StudentCounts→findElectivesStudentCount **学生数接口同样鉴权** code=-1，cErr 此前被丢弃当普通失败处理、下个 tick 又重打已失效报名接口，恢复延迟到探测/手动路径；修：与 SelectClass 分支 B8-M7 对称——cErr==ErrUnauthorized 时 maybeRelogin+置 failed+落日志）；**F19-01 MAJOR** 选课大厅回显 effect 构建幽灵 publish_id 目标被锁死（/state courses 按旧 publish_id 下发，回显照单全收构建 initial——带幽灵 publish_id 的条目进 selected 后 flushTargets/防抖保存的 targetsUseCurrentPublishes 校验必失败→一路置脏跳过（安全方向不假清空）→目标锁死在读不出的旧条目上永远改不了存不上；修：回显即刻过滤只用当前 publishes 集合内 publish_id，幽灵条目不进 selected；**注意 effect 声明于 const publishes 之前（TDZ），用依赖中 data 自行推导——重演 F18-01 构建中断陷阱，tsc -b 立即报 TS2448/TS2454**）；**F19-02 MINOR** 登录表单双 Enter/双击重复提交（按钮 disabled 依赖 React 渲染落地有延迟，连按两次可在 disabled 生效前发两个重复登录请求——并发登录互相挤掉会话+验证码识别并发放大限流；修：submit 入口先查 loading 在飞标记短路幂等）；**F19-03 MINOR** 激活码复制整链失效无兜底（navigator.clipboard 非安全上下文整体不可用，抛错只提示未授权；修：降级链 clipboard 不可用→textarea+document.execCommand("copy") 兜底→仍失败提示并把完整激活码展示抄录）。观察项：19-03 HTTP 探测单飞（B14-I2 延续）、19-04 Decrypt 死字段、19-05 RemoveFull 死方法、19-06 probe per-account 错误静默（频率受 30s 节流约束）。**第 20 轮（B20/F20 系列）**：**B20-01 MAJOR** 删除账号与在飞手动报名/退选竞态写回库行（B18-M2 只修自动链 spawnChain，手动路径同款竞态整链开放——DeleteAccount 清凭据/库行 + Accounts.Remove 与在飞手动 SelectClass/ExitClass（最长 15s）竞态，handler 直接 MarkDone/RemoveDone 无复核 → SaveSuccess 写回已删账号 success 行（重启 RestoreDone 假成功）/SaveRefused 写回 refused 行（重启 RestoreRefused 令自动引擎永久跳过该课）；修：MarkDone/RemoveDone 持锁段复核 `s.clients.ClientFor(acct)` 仍存在，已删静默放弃落库，与 B18-M2 同款防线手动两路对称补齐，TDD TestDeletedAccountManualInFlightDropsState 红灯 2 行→绿灯 1 行+refused 0 行）；**B20-02 MINOR** 幽灵窗口防轰炸闭环缺口（B19-01 兜底只覆盖时钟失败路径 syncFailStreak>=3，从未开过窗+空快照+时钟接口正常+开放时间已过时 WindowClosed 恒 false——tick 提交段 1s 周期 SelectClass（"无效的课程ID"不命中 isWindowClosedError）+probeIntervalFor 恒临门 2s 高频 findElectivesData；修：新增 EmptyProbeRuns 探测量变（空快照+未开窗+开放时间已过每轮+1 否则归零，json 省略），WindowClosed() 加第三条判据（从未开窗&&EmptyProbeRuns>=3 视同关闭），提交挂起+探测降回 30s；开窗前正常空快照首探不计数、开窗/非空快照即时归零自愈；TDD TestGhostWindowEmptyProbesSuspend 红→绿+反向断言）；**B20-03 MINOR** 探测时间戳时间基准混用（probe()/ProbeForAccount/ProbeNow 写入 lastProbe/lastDataAt/acctDataAt 用本地 time.Now()，tick 判读 now.Sub/快照过期 time.Since/提交节流全走对齐钟——两套时间基契约不自洽（B16-M1 同族）；修：三处探测时间戳写入侧统一改 s.nowAligned()/nowAlignedLocked，判读判据零改动）；**B20-04 MINOR** 管理员不带 account 设置目标落入孤儿行（handleSetTargets 对 allowAccountOverride 为真且未传 ?account= 时 acct 停在管理员名，SetTargetsForAccount(admin,ts) 写进无主行（重启幽灵复活）+污染 AccountsWithTargets 首账号选择；修：与 handleElectives/handleState 对齐——无透传取 targetAccts[0] 兜底，无任何有目标账号则整体拒绝，TDD TestAdminSetTargetsWithoutAccountRejects 红→绿 + TestStudentSetTargetsWithoutAccountOK 反向防线）；**F20-01 MAJOR** StrictMode 重挂载后目标自动保存整链静默失效（unmountedRef 只有 cleanup-effect 置 true 无 remount 复位——开发态 <StrictMode>（main.tsx）mount→unmount→remount 两遍后恒真，saveNow/scheduleRetry/补发全部短路，用户改选目标永不 PUT 且无任何报错（开发调试期间改动静默丢失只能靠刷新回显发现）；修：挂载 effect 复位 unmountedRef.current=false + cleanup 置 true 双向，重挂载后保存链路恢复，真正卸载仍停手（F13-C2 契约不变））；**F20-02/F20-03/F20-04 MINOR** 复制兜底 textarea 补 readOnly 加固（可编辑区干扰选中致 execCommand 假失败）+ 激活码弹窗补 Esc 关闭（F7-03 注释承诺 Esc 回归键盘可达性但实现从未落地，裸 div 无 keydown）+ 管理员删除确认弹窗补 Esc 关闭对称闭环。观察项：20-01 HTTP 探测单飞（B14-I2 延续）、20-02 Decrypt 死字段、20-03 RemoveFull 死方法、20-04 probe per-account 错误静默、20-05 幽灵课程条目无 UI 提示（回显已过滤，安全方向不假清空，维持观察）。**第 21 轮（B21/F21 系列）**：**B21-01 MAJOR** syncFailStreak 死代码根治（maybeSyncClock 失败分支在**同一临界区**内先 `++` 再 `if>=3 清零`，外部读取方 `WindowClosed()` 持同锁永远读不到 3——streak 值域恒 {0,1,2}，B19-01 的"时钟失败≥3 视同幽灵窗口"判据实为不可达死代码（对应测试手动注入 3 恒假绿）；修：达到 3 只复位 `clockOffset=0` 回退本地时钟，streak 持续累计至同步成功才清零自愈，瞬断 1 次只记 1 次绝不误触发，`WindowClosed()` 判据首次真实生效）；**B21-02 MINOR** EmptyProbeRuns 入场加 10s 裕量（B20-02 入账判据 `now.After(open)` 对"开窗点后平台预清空 publishes"（F7-01 记录的真实现象）立刻计数，过渡态持续 ≥6 秒即在**真实窗口已开**时累计到 3 → WindowClosed 第三条判据误触发挂起黄金期提交+探测降频；修：入账侧 `now.After(open+10s)` 才 `EmptyProbeRuns++`，开窗点后 10s 内空快照探测不计轮数，黄金期结束仍连续空才确证幽灵窗口，开窗/非空快照仍即时归零；**裕量必须在入账侧**——`WindowClosed()` 不读时间只读 count）；**B21-03 MINOR** 重登成功分支补账号已删复核（B18-M2 自动链/B20-01 手动路径都有"落库前锁内复核 `ClientFor`"防线，唯独**重登成功分支**缺同款——DeleteAccount（清凭据表+Accounts.Remove）与在途 Login（Vision 最坏 2 分钟）竞态，成功返回后无条件写回 `tokenValid`/`reloginAt` + `UpdateIDToken` 落库把已删账号新 token 写回 credentials 表，重启 Restore 重建客户端凭据幽灵复活；修：写入前持锁复核 ClientFor 仍存在，已删整段（内存写+落库）静默放弃只清 `relogging`；TDD TestDeletedAccountReloginSuccessDropsState 阻塞重登→PurgeAccount→放行→断言无 key 残留）；**B21-04 MINOR** 非法 open_time 混改半假成功（旧实现 `else if FormatOpenTime==nil` 校验失败**静默忽略**该项——混改 PUT 返回"配置已更新并生效"但 open_time 未变落库旧值，与 F7-02 空串假成功同族；若把 `writeJSON+return` 写进 `Runtime.Update` 闭包内，return 只退闭包不退出 handler = 双写响应+前置字段部分应用；修：格式校验前置到闭包之外（与非法引擎/越界并发同策略），非空非法值整体拒绝绝不落库；TDD TestAdminConfigHotReload 混改 PUT 断言 `vision_model` 保持 new-model 未被应用）；**F21-01 MAJOR** 返回按钮直接卸载吞掉最后一批改动（`flushTargets(); onDone()` 同步卸载——publishes 为空时 flush 置脏 return、savingRef 为 true 时置脏 return 飞行 PUT 完成后 finally 因 unmountedRef 已真跳过补发，两种路径都让用户**真实改动静默丢失**（F15/F16/F17 守卫拦假清空是安全方向但用户改动也被误伤吞掉）；修：返回按钮改异步 `handleBack`——flush 后若在飞 PUT 结束会经 finally 自动补发（补发链在卸载前自接），循环等 dirty 清空才 onDone，守卫拦下的假清空脏块刻意放行绝不强清，api 20s 超时兜底绝不无限挂起）；**F21-02 MINOR** activeTab 失效值（`value={activeTab ?? tabs[0]}` 的 `??` 只在 null 时回退——发布集合整体重建（开窗瞬间清空/publish_id 全变）后 activeTab 是 **stale-non-null**，Tabs value 指向不存在 Trigger 主区悬空（F18-03 只修了 null 悬空）；修：activeTab 必须存在于当前 tabs 才生效否则回退 `tabs[0]`）；**F21-03 MINOR** 退选弹窗补 Esc 关闭（有 `role=dialog/aria-modal` 却无 onKeyDown Escape——F7-03 承诺"Esc 回归键盘可达性"在 Select 退选弹窗从未落地；修：Esc 与取消同逻辑，退选中 actionLoading 不响应防误关，F20-03/04 对称闭环）；**F21-04 MINOR** 激活入口补幂等守卫（activate() 无 `if (activating) return`，与 submit() 的 F19-02 不对称——disabled 渲染落地延迟连按两次发重复激活请求后到响应处理 onLogin 把会话挤成旧值；修：入口先查 activating 短路）；**F21-05 MINOR** 管理员"学生端"无账号假登出（onBackToStudent 在 others.length===0 时仅 `setCurrent("")`，account-reselect effect 立即 `setCurrent(accounts[0])`（仍是 admin）→ 渲染条件 `inAdmin || current === adminName` 恒真 Admin 继续显示——D4 注释声称的"回登录页"从未生效；修：无学生账号=完整登出语义清空 sessions+current+targetAccount，渲染落 Login 页且 effect 不再弹回；有学生账号直接切到它）。观察项：21-01 HTTP 探测单飞（B14-I2 延续）、21-02 Decrypt 死字段（B16-I1 延续）、21-03 RemoveFull 死方法（B17-02 延续）、21-04 probe per-account 错误静默（频率受 30s 节流约束）、21-05 幽灵课程条目无 UI 提示（回显已过滤安全方向不假清空）、21-06 `emptyRunsFor` 辅助方法死代码（`WindowClosed()` 直接读 `state.EmptyProbeRuns`）。**第 22 轮（B22 系列）**：**B22-01 MAJOR 目标账号快照无条件回退全局帧——混合年级部署年级串线全开（读取侧最后一块拼图）**（ElectivesSnapshotFor 对"该账号专属快照缺失"无条件回退全局 lastData，且只要全局帧新鲜就返回 ok=true——handleElectives 拿到的数据直接渲染给用户、**根本不会触发该账号的 ProbeForAccount**；而 probe() 的 per-account 探测只遍历 AccountsWithTargets() 有目标课程的账号。触发场景：混合年级部署文档实证高二下发 82 门、高三仅 1 门——部署者先登录"高三测试账号" → m.order[0] 恒为高三 → lastData 恒为高三帧；高二学生登录打开选课大厅无专属帧 → 回退高三帧（仅 1 门体育课）→ 永不发 per-account 探测 → 大厅**永久**只显示高三那 1 门课，82 门全部不可见；修：目标账号（已配置目标）专属帧缺失/过期即返回 `(nil, false)` 让 handleElectives 走 ProbeForAccount 真取该账号年级帧；**无目标账号**（仅浏览/手动报名）保持回退全局帧（快、无网络开销，且 CheckClassSelectable 本就只读本账号专属帧不用全局帧兜底——B18-m1 已注明）；TDD TestManualSnapshotFallbackOnlyWhenOwnFresh 断言 1 目标账号专属帧缺失不得回退（红）→ 断言 2 专属帧写入后返回该账号帧（绿）；写入侧 B5-10（ProbeForAccount 不写全局帧）/B6-04（不写 lastProbe）已齐，读取侧回退路径是年级串线最后一块拼图本轮已堵）；**B22-02 MINOR 时钟幽灵窗口测试恒假绿形态收敛**（B21-01 把 syncFailStreak 判据变成真实可达后，对应测试 TestGhostWindowClockFailuresSuspend 仍是手动进样 `syncFailStreak=3`——手动注入永不经过 maybeSyncClock 失败分支，测试全绿也证明不了判据真实触发路径；修：改真实 maybeSyncClock 连续失败 3 次——每轮清 lastSyncFailAt 突破 30s 退避 + 轮询等 syncing 落地模拟三次独立失败的时间流逝；场景 B 同步成功 streak 归零→幽灵窗口解除自愈；场景 C 同形态 probeIntervalFor 必须 30s）；**内部消费方 classFullInSnapshot/releaseFullIfFreedLocked 的全局帧回退仅用于满员判定（课程不存在即返回 false/保持 full），不构成年级串线维持现状**；前端本轮无新增真实缺陷（高风险区域 12 项逐一核过）。观察项：22-01 HTTP 探测单飞（B14-I2 延续）、22-02 Decrypt 死字段（B16-I1 延续）、22-03 RemoveFull 死方法（B17-02 延续）、22-04 probe per-account 错误静默（频率受 30s 节流约束）、22-05 幽灵课程条目无 UI 提示（回显已过滤安全方向不假清空）、22-06 emptyRunsFor 死代码、22-07 syncFailedWindow 写而不读。
- 数据库 data/xuanke.db（纯 Go SQLite），重启恢复加密凭据/token/目标/已成功课程/已退选课程；`.master_key` 为主密钥文件（需与 db 一起备份）；schema v4：targets.priority / task_log.account / activation_codes / activations / **refused（B9-02 手动退选持久化）** 表，缺列拒绝启动（`targets.allow_swap` 列为历史遗留，读路径已不含换课，保留列不读不写）

### 部署（公网）
配置统一放 **`backend/data/.env`**（随 data/ 一起备份迁移；真实环境变量优先，文件兜底）：
```bash
# data/.env 示例（首次启动自动生成带注释模板）
XUANKE_ADMIN_TOKEN=你的口令      # 必填：管理口令（激活码管理用，缺失拒绝启动）
SF_API_KEY=sk-...                # 可选：教务登录验证码识别密钥
XUANKE_ACTIVATION=on             # 激活码机制开关：on=启用；off=完全关闭（登录直接进入系统）
# XUANKE_MASTER_KEY=<64位hex>    # 可选：数据加密主密钥，不填自动生成 data/.master_key
# XUANKE_PORT=3091 / XUANKE_DB=data/xuanke.db   # 可选
```
```bash
xuanke.exe                       # 无任何环境变量直接启动，自动读取同目录 data/.env
```
- 登录只需教务账密；未激活账号返回 code=1001 由前端弹激活码模态框；激活码从管理员后台「配置/激活码」面板生成/分发（admin 账号 + 管理口令登录）；激活一次永久免激活
- **管理员入口**：登录页账号填 `admin`、密码填管理口令（= `XUANKE_ADMIN_TOKEN` 值），登录后进入独立管理员界面：激活码管理、运行时配置（激活码开关 / Vision url-key-model / 开放时间，热重载免重启）、运行状态、账号管理、日志总览

### Go 接口速查（backend/internal/zhidao）
- Login(account, password) (token, err)：完整登录链路含 Vision 验证码，重试收敛（识别≤3 次 + 提交≤2 次，网络/配置错误立即返回）
- FindElectives() (*ElectivesData, error)：学期列表 -> 课程数据（含 BeginTimes/Publishes/Classes）
- SelectClass(id) / StudentCounts(ids) / IsClassFull(id)（ClassDetail 已移除；ExitClass 退选接口保留在 zhidao 层供未来人工退课/管理用，调度器不再调用）
- SetCredentials / SetCookies / Token / SetVision(热更新识别配置) / ReloginIfNeeded

### 测试
cd backend && go test ./...（含 scheduler -race）；cd web && npm run build（tsc 类型检查）

### CI/CD 自动化工作流与 Release 发布规范（.github/workflows/）
项目建立了完备的 GitHub Actions 持续集成与自动化发布流水线，分为 Push 质检流与 Tag 自动化多架构发布流：

1. **分支自动化质量检测（.github/workflows/ci.yml）**：
   - 触发时机：代码推送（push）至 `master` 或 `main` 分支，以及针对该分支的 Pull Request；自动忽略纯文档（.md）改动；支持并发任务自动取消旧运行（concurrency）。
   - 前端质检：基于 Node.js 22 + npm 缓存，执行 `npm ci` 与 `npm run build`（tsc 类型检查与 Vite 打包），验证前端编译完整性并将静态资产产物自动落盘至 `backend/web/dist`。
   - 后端质检：基于 Go 稳定版 + go.sum 依赖缓存，执行全量单元测试（`go test -v ./...`），并进行单二进制可执行文件打包编译验证（`CGO_ENABLED=0 go build`），确保 `//go:embed` 静态资产正确内嵌。
   - **PowerShell 前缀赋值陷阱（第 6 轮 F6-01 实证）**：Windows job（build/交叉编译）里 `CGO_ENABLED=1 go build` 是 **CommandNotFoundException（exit 127，非终止）**——go build 漏执行或 CGO=0 静默降级，CI 假绿 + Windows 版实际没内嵌 ddddocr。一律用 `$env:CGO_ENABLED='1'` 前缀（ci.yml + release.yml 两个 Windows build step 已修正）。

2. **Tag 自动化多架构打包与发布（.github/workflows/release.yml）**：
   - 触发时机：推送版本标签 `git push origin v*`（例如 `v1.0.0`）；支持 `workflow_dispatch` 手动触发。
   - 前置构建：先由 Node.js 构建前端最新生产级静态资产。
   - 多架构并行交叉编译：
     - Windows x64 GUI 模式（`xuanke-windows-amd64.exe`）：CGO=1 内嵌原生 ddddocr，注入 `-ldflags="-s -w -H windowsgui"`，消除控制台黑框；
     - Windows x64 控制台模式（`xuanke-windows-amd64-console.exe`）：CGO=1 内嵌原生 ddddocr，注入 `-ldflags="-s -w"`，保留终端日志输出，便于运维排错；
     - Linux x64 服务端部署版（`xuanke-linux-amd64`）：CGO=0 纯 Go 交叉编译，兼容主流 Linux 服务器系统；
     - macOS 双架构（`xuanke-darwin-arm64` / `xuanke-darwin-amd64`）：CGO=0 交叉编译，支持 Apple Silicon M系列与 Intel 芯片。
   - 交付物打包规范：
     - 自动为各平台注入脱敏无害的生产配置模板 `.env.example`，避免敏感凭据外泄同时降低用户配置门槛；
     - Windows 打包为 `.zip`，Linux/macOS 打包为 `.tar.gz`；
     - 自动计算所有发布资产的 SHA256 哈希清单写入 `checksums.txt`，防篡改校验。
   - 发布托管：通过 `softprops/action-gh-release@v2` 自动创建 GitHub Release，自动根据 commit 提交历史生成 Release Notes，并自动上传全部构件资产。

