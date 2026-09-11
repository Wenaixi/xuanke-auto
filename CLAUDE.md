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
| `POST /electives/classDetail?idToken=..` | form: `id=<课程id>` | 课程详情弹窗（上课地点/授课老师/课节等） |
| `POST /electives/select/selectElectivesClass?idToken=..` | form: `classId=<课程id>` | 报名（参数名已用 classId=-1 无风险验证：返回"选修班不存在"） |
| `POST /electives/select/exitElectivesClass?idToken=..` | form: `classId=<课程id>` | 退选 |

响应统一为 `{"code":0,"isOk":true,...}`；code=-1 未登录、code=1 业务错误（如"选修班不存在"）。

## classDetail 详情接口（弹窗逆向，新 HAR 来源：课程www.zhidao.fj.cn.har）
- 请求：`POST /electives/classDetail?idToken=..`，form 体 `id=61245`
- 响应 value 关键字段：
  - `course_name` 课程名、`class_name` 选修班名称、`teacher_name` 授课老师
  - `classroom_name` **上课地点**、`lessons_date` **上课课节**、`school_year_term` 学年学期
  - `course_type_name` 课程类型（选修I/必修）、`method_name` 任课方式、`evaluate_type_name` 评价方式
  - `audited_count` / `plan_count` 已确认/计划人数、`class_status_str` 状态（未开始）
  - `shareUrl` 分享链接（`/electives/detail/elecClass/<hex>`）
  - `electivesClassId` 选修班 id（= 请求的 id）
- 注意：`class_name` 可能带"1、2班"后缀（体育课合并班），`lessons_date` 体育课为 null（课节在别的字段）

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

**状态：** 独立模块化 React 前端 + Go 嵌入式单二进制交付（开箱即用，免环境依赖）。选课窗口 2026-09-13 09:00:00。

### 架构设计（模块化开发 + 单二进制嵌入交付）
- **开发态（前后端分离极速热重载）**：
  - 前端：React 18 + Vite + TypeScript + Radix UI 原语 + Tailwind CSS，独立在 `web/` 开发，享受秒级 HMR；
  - 后端：Go 标准库 `net/http`（Go 1.22+ 原生路由）+ `modernc.org/sqlite`（纯 Go 免 CGO），独立在 `backend/` 监听 `:8080` 提供 REST API 与 SSE 实时抢课日志流。
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

### 关键决策
- **禁止自动重登**：doRequest 对 code=-1 直接返回 ErrUnauthorized，不触发重登（平台"访问过于频繁"限流 1 分钟，频繁登录会触发）。重登能力保留为显式 ReloginIfNeeded（最多一次）
- **会话复用**：环境变量 XUANKE_TOKEN + XUANKE_COOKIE 注入已有会话（无需重新登录）；注入的 token 通过 SaveTokenOnly 持久化，重启自动恢复
- **Cookie 携带**：API 请求需带 access_limit_cookie + zd_edu_cookie（idToken 参数 + Cookie 双通道，仅 idToken 会"未登录"，带 Cookie 才有效）
- **平台限流**：课程详情/查询高频会被限流 1 分钟（msg="访问过于频繁"），调度器轮询 300ms 已考虑此风险
- **UI 设计系统规范**：采用瑞士国际主义与黑白极简艺术风格，严格绝对零圆角（Zero-Radius / rounded-none），基于 shadcn/ui 组件哲学与 Radix UI 原语。色盘仅使用 #09090b 纯黑底、#121215 表面底、#27272a 发丝边框与 #fafafa 冷冽纯白文本，通过几何高对比度与等宽数字排版展现顶级工业高级感
- 数据库 data/xuanke.db（纯 Go SQLite），重启恢复账密/token/目标/已成功课程

### Go 接口速查（backend/internal/zhidao）
- Login(account, password) (token, err)：完整登录链路含 Vision 验证码，10 次重试
- FindElectives() (*ElectivesData, error)：学期列表 -> 课程数据（含 BeginTimes/Publishes/Classes）
- ClassDetail(id) / SelectClass(id) / StudentCounts(ids)
- SetCredentials / SetCookies / Token / ReloginIfNeeded

### 测试
cd backend && go test ./...（含 scheduler -race）；cd web && npm run build（tsc 类型检查）
