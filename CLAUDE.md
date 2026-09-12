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

**状态：** 独立模块化 React 前端 + Go 嵌入式单二进制交付（开箱即用，免环境依赖）。选课窗口 2026-09-13 09:00:00。已落地：连接池预热 / 服务端时钟对齐 / 黄金期 250ms 冲刺 / 骑驴找马换课 / 失败分级退避 / ddddocr 本地识别（引擎热切换）。

### 高性能抢课架构（P0 三刀 + 智能调度）
- **共享高性能连接池 + 预热（压制 1.9s TLS 握手）**：`zhidao.sharedTransport`——`MaxIdleConnsPerHost: 64`、`IdleConnTimeout: 120s`、`ForceAttemptHTTP2: true`；调度器开窗前 2 分钟起 `maybePrewarm` 每 15s 静默 GET /login 保持 TCP/TLS 热态，首波提交零握手等待
- **服务端时钟毫秒级对齐（tick 全程用校准时间）**：`SyncServerTime` 读 HTTP `Date` 响应头 + RTT/2 中点近似得 `clockOffset`，调度器 `nowAligned()` 统一取校准时刻判定开窗点与冲刺期，根本性消除本地时钟误差（实测校准偏差 ~640ms）；5 秒内不同步一次
- **开窗前 10 秒黄金期 250ms 高频冲刺**：`submitIntervalFor` 依据对齐后时刻在开窗后 10s 内压到 250ms 间隔持续 submitAll，10s 后回落 1s 常态；探测仍受 30s 节流但提交完全不受限
- **失败分级智能退避（风控 30s / 网络快重试）**：`isRateLimitError` 匹配"频繁/429/稍后重试"文案→`markRateLimitedLocked` 该课程退避 30s；纯网络失败终止本链下 tick 快重试（250ms 黄金期）；Token 失效走 maybeRelogin 异步自动重登
- **骑驴找马自动换课（每目标可配，默认关）**：目标 `AllowSwap=true` 且已持有同发布保底课时，`maybeSwapLocked` 在快照显示更高优先级心仪课有空位时执行 退保底（ExitClass）→ 抢心仪（SelectClass）；心仪抢报失败**立即回抢保底课**绝不裸奔；换课成功转移 done + SaveSuccess/RemoveSuccess（重启不重复报名）。db.AddColumn 原地迁移 `targets.allow_swap` 支持 v4 老库无缝升级（非拒绝重建）
- **验证码识别引擎二选一 + 并发限流（默认 1）**：`CaptchaRecognizer` 接口抽象——`VisionRecognizer`（硅基流动 Vision 云）/ `LocalDdddOcrRecognizer`（子进程调本机 Python ddddocr，免 API 密钥）。全局信号量 `captchaSemaphore` 串行化识别（默认并发 1，管理员可热收敛）；Admin「系统配置-识别引擎与并发」二选一切换并校验环境缺失自动回退 Vision；`captcha_engine`/`captcha_concurrency` 持久化 settings 重启恢复

### 架构设计（模块化开发 + 单二进制嵌入交付）
- **开发态（前后端分离极速热重载）**：
  - 前端：React 18 + Vite + TypeScript + Radix UI 原语 + Tailwind CSS，独立在 `web/` 开发，享受秒级 HMR；
  - 后端：Go 标准库 `net/http`（Go 1.22+ 原生路由）+ `modernc.org/sqlite`（纯 Go 免 CGO），独立在 `backend/` 监听 `:3091` 提供 REST API 与 SSE 实时抢课日志流。
- **发布态（前端嵌入二进制，便携单文件交付）**：
  - 前端 `npm run build` 生成的纯静态产物（`web/dist`）通过 Go 原生 `//go:embed dist/*` 嵌入进 `backend/xuanke.exe`；
  - 最终用户无需安装 Node.js 或前端环境，双击单个 `xuanke.exe` 即可在单一端口同时提供后端抢课引擎与前端网页，开箱即用！

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
- **管理员后台（admin 账号 + 管理口令，会话级鉴权）**：`record admin` + `XUANKE_ADMIN_TOKEN` 比对（ConstantTimeCompare）→ `session.CreateAdmin` → 后续全部管理接口走 `requireAdminSession`（Bearer 会话）鉴权。入口：登录页账号填 `admin`、密码填管理口令（绕过教务登录，免平台限流）。管理接口全部挂 `/api/admin/*`：`config` GET/PUT 热改 + SaveSettings 落库、`stats` 运行状态、`codes` 激活码生成/列表/删除、`accounts` 账号管理（禁止删 admin）、`logs` 全量日志。普通用户会话访问一律 403。前端 admin 登录后进独立管理页（routes/Admin.tsx，五 Tab：激活码/配置/状态/账号/日志）
- **运行时热配置中心（runtime.Store，全部热重载免重启）**：管理员改动立即进 `runtime.Store`（RWMutex + Get 快照拷贝 + Update 闭包）。生效链路——激活码开关（`activationEnabled()` 三处登录/激活/生成读取）、Vision url/key/model（`Accounts.SetVision` 推全部客户端）、开放时间（`sched.SetOpenTimeFn` 调度器逐 tick 读取）；PUT 同步 `SaveSettings` 全量落库（settings k/v 表），重启后 LoadSettings 覆盖环境变量恢复。敏感值回显脱敏（`maskKey` 只显 `****`+后4位）
- **教务 token 失效自动重登（取代早期"禁止自动重登"）**：doRequest 对 code=-1 返回 `ErrUnauthorized` 本身不重登；调度器探测命中该错误时按账号标记失效并异步自动重登（防重入 + 30 秒节流，Vision 持续失败不轰炸登录接口），成功后新 token 落库（UpdateIDToken）+ 立即补一次探测。网络类失败绝不重登。前端 `/state` 只读 `token_valid` 显示有效性（有效 / 已失效·自动恢复中），不显示次数与时间
- **登录验证码重试收敛（防空炸平台限流）**：`zhidao.Login` 三层上限——识别最多 3 次（识别失败/识别结果为空/提交被拒均刷新验证码重试）、提交最多 2 次（提交被拒多为验证码过期）、初始化会话/取验证码/网络/配置错误一律立即返回。杜绝旧版"识别 10 次"引发的"登录失败次数过多，请 30 分钟后重试"平台熔断
- **会话复用与完整 Cookie 注入**：登录成功（Login）后自动提取服务端下发的所有会话 Cookie（尤其是 `access_limit_cookie` 与 `zd_edu_cookie`），若未下发则注入默认保护 Cookie。API 请求严格遵循 idToken + Cookie 双通道机制，避免服务端报 code=1 鉴权缺失
- **课程探测 30 秒节流（根因修复"选课大厅突然啥都没了"）**：调度器对 `findElectivesData` 的成功探测加 30 秒最小间隔（`probeInterval` 常量），探测成功或失败均记录时间戳，网络故障时不会 300ms 疯狂重试；窗口未开启时也绝不高频轮询，从根因消除平台"访问过于频繁"1 分钟熔断导致的课程列表拉空。轮询 ticker 仍为 300ms（负责窗口开启后的**立即**探测与提交），但探测动作本身被 30 秒节流闸门挡下
- **多账号物理隔离 + 会话级账号绑定**：每个账号独立 `zhidao.Client`（账号 A 绝不携带账号 B 的会话），认证后服务端签发随机 Bearer 会话令牌（12h TTL），所有租户接口从会话读取账号（`sessionAccount(r)`）——忽略客户端传入的账号参数，`/state`、`/targets`、`/electives/detail` 均按会话账号隔离。/electives 全校共享（同一平台同一学期数据），但读取快照不需要账号身份
- **部署访问口令 gate + 移除硬编码密钥**：`XUANKE_ADMIN_TOKEN` 必须来自环境变量或 `data/.env`（缺失直接 `log.Fatal` 拒绝启动），激活码管理接口用 `crypto/subtle.ConstantTimeCompare` 恒定时间比对；`SF_API_KEY` 也走同源注入。登录限流每分钟 5 次（token bucket，每 IP）
- **激活码鉴权（取代登录口令 gate）+ 可开关**：登录不再校验部署口令；教务登录成功 → `IsActivated` 未激活返回 `code=1001`（前端据此弹出激活码输入模态框）→ `POST /api/activate {account, code}` 事务内扣减激活码次数 + 记录 `activations` → 签发会话。激活一次永久免激活。激活码由 `X-Admin-Token` 管理接口生成（`XK-XXXX-XXXX-XXXX`，POST count 1-100 × uses≥1 / GET 列表 / DELETE）。开关：`XUANKE_ACTIVATION=off` 完全禁用（登录直接签发会话、activate/admin 接口拒绝）
- **多备选课程 + 满员人数对比退避**：每个发布可配置多门备选目标，`targets.priority` 持久化排序；调度器每（账号×发布）一条 goroutine 链（`spawnChain`）按优先级依次尝试，快照满员跳过 → inflight 去重 → `SelectClass` 失败后 `IsClassFull` 实时复核人数（`selected_count >= max_count`），真满才 `markFullLocked` 切下一备选，网络类失败终止本链下一 tick 重试
- **日志按账号隔离**：`task_log.account` 列 + `AppendLog(acct, ...)` + `LoadLogs(acct, limit)` WHERE 过滤，`/api/logs` 只返回当前会话账号自己的日志，账号间不可互通查看
- **凭据与敏感配置 AES-GCM 严格加密入库（彻底移除旧明文兼容）**：密码与 settings 表的 vision_key 均经 AES-256-GCM 加密后存入（带有 enc: 密文前缀，XUANKE_MASTER_KEY 环境变量或 data/.master_key 提供主密钥）。secureEncrypt 未注入加密器直接报错拒绝，杜绝明文入库；服务启动加载 settings 时严格校验 enc: 前缀并解密还原，若读取到未加密旧明文直接打印警告并拒绝加载，坚决执行拒绝旧版畸形数据的策略
- **不兼容旧数据库（拒绝启动）**：`db.Open` 检测到旧 `account` 表或 `targets.account=''` 空账号行即报错拒绝启动，提示删除 `data/xuanke.db` 重建
- **超高性能架构**：窗口开后首个 tick 立即探测（豁免 30s 节流）、每账号每课程独立 goroutine 并发提交、`/api/electives` 直读内存课程快照（snapshotTTL 40s > probeInterval 30s，页面浏览零上游请求）
- **学期列表容错与自动平滑回退（Fallback）**：`FindElectives` 在尝试获取可选学期列表时，若因特定时段或接口异常导致学期列表返回错误（如 code=1），自动回退并直接请求默认激活学期数据（`findElectivesData` 传空体），杜绝选课大厅因非核心接口报错而白屏或崩溃
- **预选目标课程支持随时清空（0 门合法）**：`handleSetTargets` 解除“至少需要 1 门”的死锁限制，允许用户重置清空全部目标；同时本地数据库严禁注入测试课程，保证新账号登录时绝对干净空白
- **UI 设计系统规范（纯黑白极简艺术 + 瑞士国际排版规范）**：彻底清除任何喧宾夺主的技术宣传口号（如“毫秒级并发”、“每个发布批次锁定1门心仪目标·秒级抢报”、“目标阵容”等吵闹词汇），全站统一为纯黑白极简高级艺术设计（纯黑 `#000000` 底色、发丝灰边框、纯白高对比文字与单色等宽数据）
- **水墨画布背景（全站含登录/管理页）**：`web/public/bg.jpg`（来源用户桌面 boqi.jpg）经 Vite 打包进 `backend/web/dist` 再 `//go:embed` 进单二进制。实现：App.tsx 根部渲染 `<div className="canvas-bg" aria-hidden />`，`.canvas-bg` 为 fixed 全屏 + `linear-gradient(rgba(0,0,0,0.72) 45%→rgba(0,0,0,0.38) 中部)` 深黑遮罩保证任意亮度下白字清晰。**堆叠层级关键**：背景 `z-index:0`，路由内容包在 `.app-content`（`position:relative; z-index:1`）里——不可用 `z-index:-1`（在含 fixed/Portal 堆叠上下文的页面上会被内容层的实色背景盖住，已用 test-a/test-b 隔离实验验证）。`#root` 必须 `background-color:transparent`，html/body 保留 `--bg` 纯黑底色兜底
- **磨砂玻璃组件（.glass / .glass-strong 工具类）**：`global.css` 定义——`rgba(255,255,255,0.06)` 半透明白底 + `backdrop-filter: blur(16px) saturate(1.2)` 毛玻璃 + 1px 发丝边框；`glass-strong`（`rgba(18,18,22,0.72)` + blur(20px)）用于选中态卡片与底部固定栏等需强可读性面板。选课大厅/控制中心的卡片、搜索栏、标签页、底部栏全部换用
- **选课大厅人性化便利**：顶栏新增选课开放倒计时条（`/state.open_time` 秒级跳动，窗口开放显示"窗口已开放"脉动点）；搜索栏新增"按剩余排序"（名额少靠前）+ "仅看有余量"双筛按钮；课程网格桌面 3 列 → `lg:3 xl:4` 列更密集，卡片内边距 `p-3.5` 紧凑化
- 数据库 data/xuanke.db（纯 Go SQLite），重启恢复加密凭据/token/目标/已成功课程；`.master_key` 为主密钥文件（需与 db 一起备份）；schema v4：targets.priority / targets.allow_swap / task_log.account / activation_codes / activations 表，缺列拒绝启动

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
- SelectClass(id) / ExitClass(id) / StudentCounts(ids) / IsClassFull(id)（ClassDetail 已移除）
- SetCredentials / SetCookies / Token / SetVision(热更新识别配置) / ReloginIfNeeded

### 测试
cd backend && go test ./...（含 scheduler -race）；cd web && npm run build（tsc 类型检查）
