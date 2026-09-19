# xuanke-auto 项目记忆库

## 项目目标
自动化选课：知道教育平台 https://www.zhidao.fj.cn/admin.html#/electives/select
使用 Python + requests 实现课程查询和定时抢报。登录、选课、课程详情接口均已逆向完成并实测通过。

## 认证机制
- **无 Bearer Token**，使用 `idToken` URL 参数方案
- **平台真相**：`idToken` = 前端 `window.idToken` 全局变量（`/home/menus` 响应 token 填充），HTTP 请求 URL 后附加 `?idToken=<token>`；cookie 从不承载 token
- **本项目存储约定**：token 落 `zd_edu_cookie` cookie（Python 侧自有约定，供 requests 会话携带）
- Cookie 可复用；过期后运行 `python login.py` 自动重新登录，成功后自动写回 config.py
- token 是纯数字，**无 Expires 会话级 cookie**，有效期由服务端决定
- 失效表现：接口统一返回 `{"code":-1,"msg":"您未登录,请刷新页面重新登录"}`（checkResp 弹 msg 并跳 /login）

## 登录链路（login.py，逆向自 /login 页面内联 JS）
1. `GET /login` 初始化会话，种下 `access_limit_cookie`
2. `GET /login/captcha?v={random}` 获取验证码图片，会话绑定 `_jfinal_captcha`
3. `identification` = RSA 公钥加密（PKCS1 v1.5，1024 位）的 `{"userName":账号,"password":密码}` JSON，base64 输出（公钥 DER base64 内嵌在 /login 页面）
4. `uniqueId` = base64(UA|Win32|屏幕高|屏幕宽|毫秒时间戳转36进制)（复刻 getUniqueDeviceId）
5. `POST /login/doLogin`（form 编码），成功返回 `token`，即新 idToken

## 验证码识别（captcha.py）
- 使用**硅基流动（SiliconFlow）Vision 模型**识别，配置在本地 `config.py`（SF_BASE_URL / SF_API_KEY / SF_MODEL）
- 调用 OpenAI 兼容 `/chat/completions` 多模态接口，实测一次成功率接近 100%；识别失败自动刷新验证码重试（login.py 内置）

## 核心接口（选课，全部 POST + idToken）

| 接口 | 请求体 | 说明 |
|------|--------|------|
| `POST /electives/select?idToken=..` | 无 | 学期列表 currentYearTermList（含 selected 标记） |
| `POST /electives/select/findElectivesData?idToken=..` | 空 body 或 form: `schoolYear=2026&schoolTerm=1` | 课程数据（表单编码，**不是 JSON**；空 body 平台自动返回当前激活学期） |
| `POST /electives/select/findElectivesStudentCount?idToken=..` | form: `ids=1,2,3` | 实时已报/已确认人数（前端每 10 秒轮询） |
| `POST /electives/select/selectElectivesClass?idToken=..` | form: `classId=<课程id>` | 报名 |
| `POST /electives/select/exitElectivesClass?idToken=..` | form: `classId=<课程id>` | 退选 |

响应统一为 `{"code":0,"isOk":true,...}`；code=-1 未登录、code=1 业务错误（如"选修班不存在"）。

> 注：`classDetail` 详情接口已逆向（form: `id=<课程id>`，`value` 嵌套 57 字段），但选课大厅已取消详情弹窗、代码已移除；如未来需要可据 HAR（课程www.zhidao.fj.cn.har）恢复。

## 课程数据关键字段（findElectivesData → electivesClassList 每项）
- `id`：课程 id（报名用 classId）；`course_name`/`class_name`/`teacher_name_list`：名称与任课教师
- `can_select`：当前是否可报名；`btn_type`：1=退选按钮，2=报名按钮；`title`：按钮禁用原因
- `selected_count`/`max_count`/`plan_count`：已报/可报/计划人数；`publish_id`/`plan_id`：所属发布/教学计划
- 外层 selectElectivesData 每项含 `publishName`/`canSelect`/`hasSelected`/`inDateRange`（窗口是否开放）/`groupCount`/`beginDate`/`endDate`

### 真实网站源码对照基线（逆向锚，任何平台契约改动先对照这里，绝不凭记忆臆断）
真实前端源码落盘 `legacy/website-source/`（25 个 JS）+ 真实抓包 `legacy/www.zhidao.fj.cn.har`（173 条 entry）。已核实契约：
- **学期列表**：`currentYearTermList` 真实 10 字段（含 `gradeName`/`gradeId` 年级隔离数据源，Go `YearTerm` 目前只解 3 字段未消费）
- **课程数据请求体**：jQuery 对象 → `application/x-www-form-urlencoded` 表单编码，不是 JSON；空 body 是合法首探路径；响应顶层 `{code, beginTimes(millis), selectElectivesData}` 无 msg
- **课程级字段**：服务端直接下发 `publish_id` 与 `group_no`，都不需要推导；`teacher_name_list` 逗号分隔
- **btn_type/can_select/title 源码证据**：btn_type 1=退选（btn-danger）/2=报名（btn-primary），其他值不渲染按钮；`can_select` false 加 `disabled` 且点击事件 `i.can_select && postReq(...)` 双守卫；title 转 lay-tips 悬浮
- **实时人数轮询**：`ids=<逗号分隔字符串>`（前端预 join）；10s 轮询；只更新 `selected_count`←`selectedCount` 与 `audited_count`←`auditedCount`；**`b` 数组只在 `inDateRange=true` 的发布下收集课程 id**——窗口未开/已关时 `b` 空 → 前端不发轮询
- **错误文案（HAR+日志实证）**：未开放不可选 title=`不在选修报名时间范围内，无法选课！`；重复提交=`选课处理中，请勿重复操作！`（code=1 并发重复，task_log 实证）；成功=`选课成功！`；满员与"无效的课程ID"无 HAR 样本（review 实证背书）
- **窗口开启自动刷新（官网契约）**：1500ms 检查一次 `beginTimes`，开窗前后 1.5s 内自动整页重拉——这是"开窗瞬间自动探测"的官方等效实现
- **报名/退选 body**：单字段 `classId=<数字id>` 表单编码；退选先 `layer.confirm`，报名直发；成败均整页重拉
- **countList 字段**：`{id, selectedCount, auditedCount}`；**`maxCount` 判定不存在**——真实 select.js 轮询回调只读 id/selectedCount/auditedCount 三键；满员可点性全由服务端 `can_select[+btn_type+title]` 双守卫判定，前端从不做数字对比。Go 端 `CountEntry.MaxCount` 保留字段但平台未下发恒 0——实时复核 `IsClassFull` 恒 false，真满员主路径为快照 `classFullInSnapshot`（`max_count` 实证字段）
- **token 真实机制**：权威来源 `window.idToken`（由 `/home/menus` 响应 `token` 填充，sessionStorage 冗余）；cookie 从不承载 token；Go 端 idToken + Cookie 双通道属防御性冗余，契约主体锚定 URL 参数
- **correctUrl 拼接**：url 已含 `=`（带 query）→ 用 `&` 拼接；否则用 `?`；token 经 encodeURIComponent
- **popReq 完整契约**：默认成功静默、失败弹 layer.msg；`data` 对象 jQuery 默认表单编码；`__op_tip_msg/__op_tip_seconds` 是平台下发确认弹窗提示字段
- **checkResp 状态码机**：code=0 正常；-1 弹 msg+跳 /login；-6 域名迁移；-10 短信风控弹窗；-11 强制改密；-12 弹 msg+reload；-14/-15 跳 errors[0].name。**关键边界：code=1（业务失败）不在 checkResp throw 名单——成败看响应 `isOk/isFail` 布尔**（Go `SelectClass` 用 `Code != 0 || !IsOk` 双判已对齐）
- **cache:false 澄清**：jQuery 3.x 只对 GET 追加 `_=` 时间戳，POST 不改写 URL——本项目全部 POST，URL 恒定不抖动
- **登录第四字段 priorityId**：`localStorage["priorityId"]` = `/home/menus` 的 `user.id`；undefined 时 jQuery 丢弃该键
- **getUniqueDeviceId**：与 login.py 复刻逐字段一致（UA|Win32|screen.height|screen.width|时间戳36进制 `|` join 后 btoa）

## 已知数据
- 选课开放时间：**2026-09-13 09:00:00**（时间戳 1789261200000），窗口 09:00-10:30
- 三个发布：体育（3 门，36 人/门）、校本1（40 门，29 人）、校本2（39 门，29 人）
- 例：健美操=61115、排球=61125、篮球=61135；校本1 篮球=61205；健身瑜伽=61276；近代物理选讲=61245

## 文件结构
```
xuanke-auto/
├── config.py      # token、cookie、账号密码、验证码 Vision 配置（login.py 自动更新 token）
├── captcha.py     # 验证码识别（硅基流动 Vision）
├── login.py       # 完整登录（RSA+Vision），成功后写回 config.py
├── xuanke.py      # 主脚本，query / detail / monitor 三种模式
├── legacy/
│   ├── www.zhidao.fj.cn.har          # 选课全链路真实抓包（173 条 entry）
│   ├── 课程www.zhidao.fj.cn.har      # 课程详情弹窗抓包（classDetail）
│   └── website-source/               # 从 HAR 提取的真实站点前端源码（25 个 JS）
└── CLAUDE.md      # 本文件
```

## 使用方式
```bash
python login.py            # 重新登录并更新 token
python xuanke.py query     # 查询当前课程信息
python xuanke.py detail 61245 61115   # 查看课程详情
python xuanke.py monitor   # 监控模式（窗口开后自动提交）
```
`xuanke.py` 顶部可改：`AUTO_SUBMIT`（自动提交开关）、`TARGETS`（课程 id 或课程名子串，如 ["篮球"] 命中所有同名班次）。

## 依赖
`requests`、`cryptography`（验证码识别走硅基流动 Vision API，需 config.py 配 SF_API_KEY）

## 注意事项
- 真实报名接口是 `selectElectivesClass`（历史文档误记为 apply）；`/electives/apply` 是教师端申报入口，与学生报名无关
- `findElectivesStudentCount` 窗口未开放时前端根本不发请求（b 数组只在 inDateRange=true 下收集课程 id）
- 待办：xuanke.py 遇到 code=-1 时尚未自动重登，只在 monitor 打印提示

## Go + React 现代版（backend/ + web/）

**状态：** 独立模块化 React 前端 + Go 嵌入式单二进制交付（开箱即用）。双引擎后端（CGO=1 内嵌 ddddocr / CGO=0 纯 Go 交叉编译）。已落地：连接池预热 / 服务端时钟对齐 / 黄金期 250ms 冲刺 / 失败分级退避 / 平台 beginTimes 自动识别开放时间。

### 架构设计（模块化开发 + 单二进制嵌入交付）
- **开发态（前后端分离极速热重载）**：前端 React 18 + Vite + TS + Radix UI + Tailwind 独立在 `web/` 秒级 HMR；后端 Go 标准库 `net/http`（Go 1.22+ 原生路由）+ `modernc.org/sqlite`（纯 Go 免 CGO）独立在 `backend/` 监听 `:3091`。
- **发布态（前端嵌入二进制）**：`web/dist` 经 Go 原生 `//go:embed dist/*` 嵌入 `backend/xuanke.exe`——最终用户双击单个 exe 即同时提供后端抢课引擎与前端网页。
- **防逆向与引擎二选一冲突**：garble（`-literals -tiny`）需 CGO=0，而 Windows 原生内嵌 ddddocr 必须 CGO=1——发布流水线 Windows 走 CGO=1 原样构建，Linux/macOS 走 CGO=0 交叉编译（可本地手动 garble）。
- **单二进制内嵌原生 ddddocr**：ONNX 模型 + 字符集 + onnxruntime.dll 经 `//go:embed` 编进单 exe，运行时懒加载释出 `%TEMP%\xuanke_ddddocr_assets`（dumpIfDiff 对比大小）；`classification` 进程内推理 5~10ms。**build-tag 双轨**（`native_ocr.go` windows+cgo / `_stub.go` 回退 Python 桥接或 Vision）。
- **窗口关闭形态（open vs closed 分界）**：**未开放≠关闭**——开窗前 publishes 完整 82 门 + title 禁用提示；关闭后 `code:0` 空 publishes。调度器"先开过窗才判定关闭"；`selectElectivesClass` 对已关窗口返回 `code:1`，按满员记 full 不轰炸。
- **SPA 兜底**：SpaHandler 首行 `/api` 前缀整体 404（未注册 /api/xxx 一律 404，不落 index.html）。
- **日志系统约定**：探测成功/空快照、窗口关闭、自动重登、登录全链路均输出可读日志；**token/密码只显前 8 位**（点击看契约 17）。

### 高性能抢课架构（P0 三刀 + 智能调度）
- **共享连接池 + 预热（压制 1.9s TLS 握手）**：`sharedTransport`——`MaxIdleConnsPerHost:64`、`IdleConnTimeout:120s`、`ForceAttemptHTTP2:true`；开窗前 2 分钟 `maybePrewarm` 每 15s 静默 GET /login 保持 TCP/TLS 热态。
- **服务端时钟毫秒级对齐**：`SyncServerTime` 读 HTTP `Date` 头 + RTT/2 得 `clockOffset`，`nowAligned()` 统一取校准时刻判定开窗点与冲刺期（实测偏差 ~640ms）。契约：成功才推进 `lastSyncTime`；失败落地 `lastSyncFailAt` 退避 30s、成功即清；无可用同步客户端时复位 `syncing` 防永久休眠。
- **开窗前 10 秒黄金期 250ms 高频冲刺**：`submitIntervalFor` 压缩 submitAll 间隔，10s 后回落 1s 常态；提交不受 30s 节流限制。
- **失败分级智能退避**：风控文案→`markRateLimitedLocked` 30s；纯网络失败→下 tick 快重试；token 失效→maybeRelogin 异步自动重登。
- **限流 IP 透传可信反代开关**：`XUANKE_TRUSTED_PROXY=on` 且 RemoteAddr 回环才信 `X-Forwarded-For` 最右非空值——默认关闭，绝不盲信公网伪造 XFF。
- **窗口状态裸指针根治**：`WindowOpened` 返回布尔值快照（非 `*bool` 裸指针，杜绝调用方拿地址读被并发改写）。
- **探测节流三件套**：全校 30s 节流闸门（`probe()`/`ProbeNow` 写入）+ `probing` 单飞守卫（探测量并发时放弃本次）+ `probeSem`(cap 4) 信号量封顶 per-account 并发（峰值 N→4）。HTTP 侧 `ProbeForAccount` 刻意不套单飞（管理员刷新要强制最新 + 频率已受节流约束），维持观察。
- **多账号年级隔离快照**：调度器按账号维护 `acctData`/`acctDataAt`，`ElectivesSnapshotFor`/`ProbeForAccount`;每个账号独立 `zhidao.Client`（账号 A 绝不携带 B 的会话）+ 会话级账号绑定（`sessionAccount(r)` 忽略客户端传参）。**年级串线根除**：probe() 只探测有目标的账号，读取侧见决策契约 7。

### 鉴权与数据安全
- **管理员后台**：登录页账号填 `admin` + 管理口令进入独立管理页（五 Tab：激活码/配置/状态/账号/日志）；管理接口全部挂 `/api/admin/*` 走 `requireAdminSession`（Bearer 会话）鉴权；`XUANKE_ADMIN_NAME` 自定义管理员名。
- **运行时热配置中心**：管理员改动立即进 `runtime.Store`（RWMutex + 快照拷贝 + Update 闭包），PUT 同步 `SaveSettings` 全量落库重启恢复；敏感值回显脱敏（`maskKey` 只显 `****`+后4位）。**开放时间不在其列**（自动识别）。
- **激活码鉴权 + 可开关**：教务登录成功 → `IsActivated` 未激活返回 `code=1001`（前端弹激活码模态框）→ `POST /api/activate` 事务内扣减次数 + 记录 activations → 签发会话。激活一次永久免激活；`XUANKE_ACTIVATION=off` 完全禁用。
- **凭据 AES-256-GCM 严格加密入库**：密码与 vision_key 均 `enc:` 密文前缀；`XUANKE_MASTER_KEY` 环境变量或 `data/.master_key`；未注入加密器/读到未加密旧明文/损坏主密钥一律拒绝启动。
- **门票/票据**：1001 响应体 `data.ticket` 前端必须随激活请求回传；`handleActivate` 先 ConsumeTicket 再校验激活码（防穷举，详见决策契约 14）。

### UI 设计系统规范（纯黑白极简艺术 + 瑞士国际排版）
- 彻底清除技术宣传口号（"毫秒级并发""智能 Vision 识别"等广告横幅），全站**纯黑 `#000000` / 深暗 `#09090b` 底 + 1px 发丝冷灰边框 + 纯白高对比文字 + 单色徽章**，移除彩色 emoji。
- 主按钮纯白反转底黑字（`bg-white text-black`）；次要信息冷灰 `#a1a1aa` / 更弱 `#71717a`。
- 倒计时/监控指标等宽数字（`tabular-nums`）；极简几何圆角（4-8px）摒弃大圆角与阴影。
- 水墨画布背景 `web/public/bg.jpg`：fixed 根部容器 + 真实 img（getBoundingClientRect 量测渲染高，位移封顶=图片高-视口高，桌面 160% 宽、移动 180% 贴顶）+ 独立深黑渐变 mask；`z-index:0` + `.app-content`(`z-index:1`) 堆叠；`#root` 必须透明。磨砂玻璃工具类 `.glass/.glass-strong`（rgba 白底 + backdrop-filter blur）。

### 工程决策手册（核心契约）

**窗口状态与开放时间**
1. **开放时间唯一事实源 = 平台 beginTimes 自动识别（不可配置）**：识别槽 `openTimeDetected[acct]→["*"]`。三硬契约：① 识别槽存在必须返回识别值本身，绝不截断零值——开窗瞬间起该值恒为过去，截断会让 tick 提交守卫把黄金期 250ms 永久挂起；挂起判据只归"从未识别"（槽空）。② 识别过期只影响展示层（`open_time_known=false` 前端显"未识别到开放时间"），槽保留不删（关闭≠时间消失）。③ 识别槽写入必须持 s.mu。探测节奏：未到期→临门 2s 盯守；已到点+已关→30s 兜底；已到点+快照非空→继续临门 2s（平台随时开窗绝不误降频）。
2. **WindowClosed 判据单源 `windowClosedLocked()`**：`WindowClosed()` 与 `StateForAccount` 必须共用三条判据——主判据（曾开窗 + 空快照 + 开放时间已过，带 **+10s 裕量**）/ 时钟连续失败 ≥3（须另带"开放时间已过"，防未来开窗点误挂）、/ 幽灵窗口 EmptyProbeRuns ≥3（入账侧同样 +10s 裕量、`now.After(open+10s)`，裕量必须在入账侧）。判据内取**单次 open 快照**复用（防热改亚毫秒窗口内读取不一致）。测试预置关闭状态必须用"空快照形态"（仅置位会被首 tick probe 覆写）。
3. **关闭≠时间消失契约**：目标发布元数据（`publish_name`/`begin_date`）随 targets 持久化，`/state.courses` 自带日期/发布名，Dashboard 分组零依赖 /electives；`/api/electives` 窗口关闭后空发布不映射，靠 state 自带。空快照绝不 delete 识别槽。旧库缺这两列走自动迁移（见「数据库增量迁移规范」）。

**删账号与并发竞态防线**
4. **删账号必须 memory-first**：`Accounts.Remove` → `Sched.PurgeAccount` → `Store.DeleteAccount` → `Sessions.RevokeAccount`——客户端先消失，在飞链的复核立即失败静默放弃落库，杜绝毫秒级空窗写回。
5. **"落库前锁内复核 ClientFor" 防线族**：凡在网络往返后持锁写状态/落库的分支，都必须先复核账号仍存在——自动链成功 / 手动 MarkDone/RemoveDone / 重登成功 / 实时人数复核回锁后三路 / spawnChain 链顶与取 client 后两处。已删账号静默放弃（不写库行/日志/map）。实时复核满员分支必须先查 doneHas（绝不覆盖手动报名成功的胜利状态）。
6. **重启恢复顺序契约**：恢复目标必须用 `RestoreTargets`（与 `SetTargetsForAccount` 唯一区别是不清 refused 内存+库行），顺序 `RestoreDone(success)` → 逐账号 Load+RestoreTargets → `LoadRefused`+`RestoreRefused`；`SetTargetsForAccount` 只在用户主动重设目标时调用且**只清 refused，绝不清 done/full/rateLimited/inflight**（done=跨目标持久历史、full=真实满员、inflight=防双包；清库失败必须记日志）。refused 优先于 done。
7. **?account= 透传全路径凭据表校验（accountExists，判据同源）**：写目标 / 课程读 / 手动报名退选 / 状态读四路全覆盖，查无此账号 → 明确"账号不存在"整体拒绝，绝不用全局帧/空状态假装成功。
8. **ElectivesSnapshotFor 回退链**：**任何账号专属帧"存在但过期"→ 返回 (nil,false) 触发真刷新，绝不回退全局 lastData**（全局帧恰被 order[0] 账号刷新为新鲜时回退 = 年级串线 + 浏览者永不触发本账号刷新）；目标账号判据用 `len(acctTargets)>0`（空 slice 不算有目标）；无目标纯浏览且从未有专属帧才允许回退全局帧。

**前端目标自动保存（假清空/回显/返回三族）**
9. **假清空守卫链**：目标集由 `[publishes × selected]` 联查构建、消费时刻（防抖回调 + flush 双闸）实时校验 `publish_id` 必属当前 publishesRef 且 空集必伴 selectedCount>0——任一异常置脏跳过，绝不以 `[]` 整包抹除后端目标。
10. **"发布恢复"必须驱动重试**：守卫拦下的改动不能等死——防抖 effect 依赖加稳定布尔 `hasPublishes`（发布空→非空重跑新 timer 正常保存；非空期间布尔不变绝不重置 400ms 窗口）。
11. **回显合并按 publish_id 真合并**：已触碰发布保留现状（含主动清空的空数组，清空语义绝不复活），未触碰发布按 priority 补进旧目标；`rev>0 && 全空`（用户已全清空）绝不合并；**`stateData===undefined`（首帧未到）绝不置位 echoedRef**。
12. **返回/防抖双闸等回显完成**：handleBack 首轮 flush 与防抖 400ms 保存都必须在消费时刻等 echoedRef（`stateData 未到 || courses 非空` 时最多 5s 轮询）；**消费时刻一律读 selectedRef/revRef 镜像**（async 闭包捕获悖论）；flush 自己发起的 PUT 也要等 savingRef 静止；守卫拦下的假清空脏块放行，真实改动静默丢失绝不允许。
13. **Select 挂载点必须 `key={account}` + 账号复位守卫**：账号切换（401 吊销切号/代理切换/重登）即整体重建实例，绝不复用旧账号 selected/echoedRef/rev；守卫必须声明于 echoedRef/rev 之后（TDZ 构建陷阱：tsc -b 报 TS2448/TS2454）。

**手动报名/退选协同**
14. **4 方法协同**：`TryAcquireSubmit`（inflight 在飞互斥，冲突提示"该课程正在提交中"）→ 成功 `MarkDone`（写 success + 清 full/rateLimited + **同步删库内 refused 单课行**——手动重报成功后残留行重启会被恢复成"已手动退选"）/ `RemoveDone`（删 success + 置 pending + 落 refused）/ `RemoveFull`。窗口已关时平台"无效的课程ID"按满员记 full 不再轰炸。spawnChain 提交前 inflight 去重杜绝双发包。
15. **btn_type 状态机（官网逆向契约）**：1=退选、2=报名、**其他值不渲染操作按钮**；can_select 决定"是否可点"；`hasSelected`/`canSelect` 为发布级计数，`selected_count`/`max_count` 为课程级名额。前端 isFull 与后端 IsClassFull 同源：`max_count>0 && selected_count>=max_count`（0=名额未公布，筛选/徽章/进度色四处同源）。
16. **前端手动操作在飞幂等**：actionLoading 用 `ReadonlySet<number>` 按课程独立跟踪（单值会被异课程互踩）；所有副作用按钮都加在飞守卫；手写弹窗「取消」autoFocus（Esc 打开即生效）。

**健壮性兜底**
17. **落库失败必须记日志绝不静默吞错**：全部 `SaveSuccess/SaveRefused/DeleteSuccess/AppendLog` 调用 `if err != nil { log.Printf }`，绝不允许 `_ =`——落库失败恰好破坏重启恢复契约（refused 行丢失=退选被静默撤销、success 行丢失=已成功课被重抢）与审计线索。全仓库零吞错落库点。
18. **识别引擎热切换必须同步模板**：`SetRecognizer` 写进 `VisionConfig` 模板（WithRecognizer），`SetVision` 赋值前保留当前引擎——否则新 ensure 客户端引擎恒 nil，ddddocr 部署下新账号登录全败。
19. **时钟兜底/幽灵窗口挂起语义**：`syncFailStreak≥3` 判据必须带"开放时间已过"；复位只清 clockOffset、streak 累计至同步成功才清零自愈。
20. **代码注释严禁轮次前缀标签（"X-XX（第 N 轮）"）**——轮次决策历史统一落本手册，代码注释只写"为什么/契约/陷阱"本身，历史残留发现即剥离。

### 数据库增量迁移规范（2026-09-18 定立）
- **旧库缺"纯新增列"自动迁移，缺"既有语义改变"的列/表拒绝启动**——分界判据：新版本只向既有表追加读写范围（如 targets.publish_name/begin_date 随目标持久化的元数据），`ALTER TABLE ADD COLUMN` 是纯增量安全操作绝不破坏数据，旧库必须能用；新旧对同一列/表语义不兼容才拒绝启动引导重建。
- 迁移实现放 `db.go` 的 `migrateAddPublishMeta`（Open 流程在 refuseLegacy **之前**调用）：逐列 `columnExists` 判存在，缺才 ALTER；refuseLegacy 缺列清单必须对应剔除已迁移列（缺列拒绝启动曾让用户双击打不开——exe 启动即 log.Fatal）。
- TDD 守护：`TestMigrateAddsPublishMetaColumns`（旧库放真实数据行 → Open 成功 → 两列已补 + 旧行保留）。
- 未来新加列一律先写迁移函数 + 迁移测试，再考虑是否加入 refuseLegacy 清单。

### 部署（公网）
配置统一放 **`backend/data/.env`**（随 data/ 一起备份迁移；真实环境变量优先，文件兜底）：
```bash
# data/.env 示例（首次启动自动生成带注释模板）
XUANKE_ADMIN_TOKEN=你的口令      # 必填：管理口令（激活码管理用，缺失拒绝启动）
SF_API_KEY=sk-...                # 可选：教务登录验证码识别密钥
XUANKE_ACTIVATION=on             # 激活码开关：on=启用；off=完全关闭
# XUANKE_MASTER_KEY=<64位hex>    # 可选：数据加密主密钥，不填自动生成 data/.master_key
# XUANKE_PORT=3091 / XUANKE_DB=data/xuanke.db   # 可选
```
```bash
xuanke.exe                       # 无任何环境变量直接启动，自动读取同目录 data/.env
```
- 登录只需教务账密；未激活返回 code=1001 弹激活码模态框；激活码从管理员后台生成/分发（admin + 管理口令）；激活一次永久免激活。
- **管理员入口**：登录页账号填 `admin`、密码填 `XUANKE_ADMIN_TOKEN` 值 → 独立管理界面（激活码管理 / 运行时配置 / 运行状态 / 账号管理 / 日志总览，热重载免重启）。

### Go 接口速查（backend/internal/zhidao）
- Login(account, password)：完整登录链路含验证码，重试收敛（识别≤3 + 提交≤2，网络/配置错误立即返回）
- FindElectives() (*ElectivesData, error)：学期列表 → 课程数据（含 BeginTimes/Publishes/Classes）
- SelectClass(id) / StudentCounts(ids) / IsClassFull(id)（ClassDetail 已移除；ExitClass 保留在 zhidao 层）
- SetCredentials / SetCookies / Token / SetVision(热更新识别配置) / ReloginIfNeeded

### 测试
`cd backend && go test ./...`（含 scheduler -race）；`cd web && npm run build`。**后端回归陷阱**：改 tick 守卫/探测时序前先跑 `TestWindowOpenSubmitsWithoutProbeReset|TestAdminStatsWindowOpenedUsesScheduler` 确认新旧行为（恒红/恒绿双实证）。**前端回归必须以 npm run build 为准**（项目根 tsc --noEmit 是 references 空壳不报错，只有 tsc -b 真校验）。

### CI/CD 自动化工作流与 Release 发布规范（.github/workflows/）
1. **分支自动化质量检测（ci.yml）**：push 至 master/main 或 PR 触发；忽略纯文档改动；前端 `npm ci && npm run build`（tsc + Vite），产物落 `backend/web/dist`；后端全量 `go test -v ./...` + `CGO_ENABLED=0 go build` 验证 //go:embed。**陷阱**：Windows job 里 `CGO_ENABLED=1 go build` 是 CommandNotFoundException（exit 127 非终止）——go build 漏执行假绿，必须 `$env:CGO_ENABLED='1'` 前缀。
2. **Tag 自动化发布（release.yml）**：`git push origin v*` 或 workflow_dispatch；多架构并行交叉编译——Windows x64 GUI（`-H windowsgui`，CGO=1 内嵌 ddddocr）/ Windows x64 控制台（保留日志）/ Linux x64（CGO=0）/ macOS 双架构（CGO=0）。各平台注入脱敏 `.env.example` 模板；打 zip/tar.gz；SHA256 清单 `checksums.txt`；softprops/action-gh-release@v2 自动建 Release + Notes。