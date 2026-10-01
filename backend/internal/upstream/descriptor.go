package upstream

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// FormFields 平台表单的字段名。同一套流程（学期列表 / 课程数据 / 实时人数 / 报名退选 /
// 登录）在各站点用的键名不同，故键名是档案数据而非引擎字面量。
type FormFields struct {
	Year           string // 学期列表兜底请求：学年
	Term           string // 学期列表兜底请求：学期
	IDs            string // 实时人数：逗号分隔的课程 id 串
	ClassID        string // 报名/退选：单个课程 id
	Captcha        string // 登录：验证码
	Identification string // 登录：加密后的账密
	UniqueID       string // 登录：设备指纹
	PriorityID     string // 登录：优先身份（学生端无真值，保持空串）
}

// LoginHooks 登录链路的站点专有部分：统一 UA 与两处算法，引擎只按契约调用。
type LoginHooks struct {
	// UserAgent 站点要求的一致 UA（登录页与已登录接口共用同一份）。
	UserAgent string
	// EncryptIdentification 提交登录前的账密加密。知道教育平台走登录页内嵌 RSA
	// 公钥的 PKCS1 v1.5；别的站点可能是明文或 AES——故是钩子而非引擎里的固定实现。
	EncryptIdentification func(account, password string) (string, error)
	// DeviceID 设备指纹（复刻站点前端 JS 的 getUniqueDeviceId 生成规则）。
	DeviceID func(ua string, now time.Time) string
	// Custom 自定义登录接管钩子（可选，nil = 走引擎默认 4 步流）。
	// 适用场景：统一身份认证 CAS/OAuth2 多步 302 重定向、滑动验证码、或者非标准两步认证等。
	// 当提供此钩子时，LoginEngine.Login 优先全权委托给此钩子，在会话完成后返回 token，
	// 会话过程中产生的 Cookie 会被引擎自动捕获并持久化。
	// 这使得接入任何非标登录平台时，引擎代码零改动！
	Custom func(sess *http.Client, baseURL, account, password string) (string, error)
}

// AuthMode 会话 token 的下发通道。四态而非布尔：不同平台要的通道不同。
// 为什么必须下沉：引擎曾无条件把 token 拼进查询参数（`?TokenParam=TOK`），
// 纯 Header 鉴权的现代 API 接不了；参数名为空时更会拼出 `?=TOK` 这种畸形 URL，
// 平台多半直接拒，而现场报的是「参数缺失」这类难定位的错误。
type AuthMode int

const (
	// AuthDual 查询参数 + Cookie 双通道。**零值**，保留给需要冗余的站点
	//（知到即此类：平台同时比对 URL token 与会话 Cookie）。
	AuthDual AuthMode = iota
	// AuthQuery 仅查询参数。
	AuthQuery
	// AuthCookie 仅 Cookie。
	AuthCookie
	// AuthHeader 仅 HTTP 头。档案须在 HeaderTokenName 声明头名
	//（如 "Authorization"），引擎写入 `Bearer <token>`。
	AuthHeader
)

// CaptchaSpec 平台图形验证码的规格。**零值语义统一为「不校验」**，使未声明的档案
// 仍能工作：新平台不声明即不做字符集过滤与长度门禁，不会因规格写死而被静默丢弃识别结果。
//
// 为什么必须下沉：曾把「3~5 位 + 纯英数字」写死在引擎 seam，那是知到的图片规格；
// 换个平台（4 位纯数字、含中文、6 位混合）会让识别结果被引擎静默丢弃，登录三次重试全废。
type CaptchaSpec struct {
	// Enabled 站点是否有图形验证码。false = 登录链路跳过取图与识别。
	Enabled bool
	// Charset 允许的字符集（正则字符类内容，如 "A-Za-z0-9"、"0-9"）。
	// 空串 = 不做字符集过滤，只去首尾空白（适配字符集不定的平台）。
	// 非法表达式在档案 Validate（装配面）被拒，绝不拖到运行时。
	Charset string
	// MinLen 识别长度下限。0 = 不校验下限。
	MinLen int
	// MaxLen 识别长度上限。0 = 不校验上限。
	MaxLen int
}

// Normalize 按本规格净化识别结果：按 Charset 过滤字符（空 = 只去首尾空白）。
// 非法 Charset 兜底不过滤（Validate 已在装配面拦下，运行时不 panic）。
func (s CaptchaSpec) Normalize(raw string) string {
	return normalizeByCharset(raw, s.Charset)
}

// Acceptable 识别长度是否落在声明区间内。MinLen/MaxLen 为 0 表示该侧不校验。
func (s CaptchaSpec) Acceptable(n int) bool {
	if s.MinLen > 0 && n < s.MinLen {
		return false
	}
	if s.MaxLen > 0 && n > s.MaxLen {
		return false
	}
	return true
}

// normalizeByCharset 按正则字符类内容过滤掉不在集合内的字符。
// charset 为空 = 只去首尾空白（适配字符集不定的平台）。
//
// 字符集的**合法性由 Validate 在装配面校验**（见 validCaptchaCharset），本函数
// 只负责执行。注意不能用"能否编译"当合法性判据——像 "[invalid" 这种残缺写法，
// `[^`+charset+`]` 恰好仍是合法正则（嵌套字符类），会把 b/i/n/… 当非法字符静默剔掉。
// 运行时只做"编译失败则不过滤"这一层防御，绝不静默丢字符。
func normalizeByCharset(raw, charset string) string {
	raw = strings.TrimSpace(raw)
	if charset == "" {
		return raw
	}
	re, err := regexp.Compile("[^" + charset + "]")
	if err != nil {
		return raw
	}
	return re.ReplaceAllString(raw, "")
}

// validCaptchaCharset 校验档案声明的字符集能否作为正则字符类使用。
//
// **只校验"能否编译"这一条硬错误**，并说清为什么不做语义校验（勿再尝试加启发式）：
// 语义残缺的写法（如 "[invalid"）在 Go 正则里会编译成嵌套字符类 `[[invalid]`，
// 该类**确实匹配字母 a**（嵌套类里就有 a）。已实测三种通用启发式全部失败：
//   ① 能否编译——残缺写法能编译；
//   ② 是否含字母数字——`[invalid]` 恰含 i/n/v/a/l/d，纯数字类 "0-9" 又确实不含字母；
//   ③ 剔除比例过半——合法的 "0-9" 会剔掉全部 26 个字母，恒超半数。
// 故职责边界是：挡掉**编译不过**的明显错误；语义正确性由档案作者负责（他看得懂
// 自己写什么）。真出问题的后果轻微——多保留噪声字符会被长度门禁拦下，
// 不会静默丢字符（那才是严重后果）。
func validCaptchaCharset(charset string) error {
	if strings.TrimSpace(charset) == "" {
		return nil // 空 = 不做字符集过滤
	}
	if _, err := regexp.Compile("[" + charset + "]"); err != nil {
		return fmt.Errorf("字符集 %q 不是合法的正则字符类: %w", charset, err)
	}
	return nil
}

// Decoders 各接口响应的解码钩子：字段名与响应形态由站点决定，引擎不猜任何键名。
type Decoders struct {
	Terms     func(body []byte) ([]YearTerm, error)
	Electives func(body []byte) (*ElectivesData, error)
	Counts    func(body []byte) ([]CountEntry, error)
	// OpResult 报名/退选响应 → (平台消息, 平台业务码, 平台是否判定成功, 解析错误)。
	// 成败判据（code==0 && isOk 这类双布尔约定）刻意留在站点侧：不同站点约定不同，
	// 引擎只消费"成功/失败"与业务码这两位事实，不解析任何键名。
	// code 是平台业务码，透传给 OpErrorClassifier 与 SiteError.Code——曾恒传 0，
	// 让"按数字码分类"的平台接不了（分类器签名承诺了引擎给不出的事实）。业务码是
	// 字符串、无法映射进 int 空间的平台一律传 0，改由文案分类。
	OpResult func(body []byte) (msg string, code int, ok bool, err error)
	Login    func(body []byte) (token string, err error)
	// Msg 从错误响应体里提取可读文案（token 失效时附在错误里供排障）。
	Msg func(body []byte) string
}

// SiteDescriptor 一个上游选课站点的接口形态——引擎与站点之间的唯一 seam：
// 站点包只提供"地址 + 路径 + 键名 + 解码钩子"，流程（会话、重试、探测、提交、退避、
// 黄金期、多账号隔离）全部归引擎。新增一个年份/站点 = 新增一个站点包 + 注册表一行，
// 引擎与调度器零改动。
type SiteDescriptor struct {
	ID             string // 稳定标识（落 settings.platform_id）
	Name           string // 管理员端展示名
	Note           string // 管理员端补充说明（接口版本/实测范围）
	DefaultBaseURL string // 站点根地址默认值（管理员可在后台覆盖）

	LoginPath   string // 登录页（会话初始化 / 时钟对齐 / 预热共用的轻量 GET）
	CaptchaPath string // 验证码图片路径（引擎拼 ?v=<毫秒>）
	DoLoginPath string // 登录提交路径

	TermsPath     string // 学期列表
	ElectivesPath string // 课程数据
	CountsPath    string // 实时人数
	SelectPath    string // 报名
	ExitPath      string // 退选
	RefererPath   string // 已登录接口的 Referer 页面路径

	TokenParam  string // 会话 token 的查询参数名（如 idToken）
	TokenCookie string // 会话 token 的 Cookie 名（双通道冗余）
	// AuthMode 会话 token 的下发通道。四态而非布尔：不同平台要的通道不同，
	// 强制双通道会让纯 Header 平台接不了，也会让不需要 token 参数的平台
	// 收到形如 `?=TOK` 的畸形 URL。零值 AuthDual = 既有行为，既有档案零改动。
	AuthMode AuthMode
	// HeaderTokenName 仅 AuthMode == AuthHeader 时必填（引擎写 `Bearer <token>`）；
	// 其余形态忽略该字段。
	HeaderTokenName string
	// Headers 该站点要求/期望的附加请求头，引擎逐条覆盖写入已登录接口与登录提交。
	// **空 map = 引擎默认头集**（见 applyHeaders 的 X-Requested-With/Accept），
	// 保留既有实测行为；声明了 map 才逐条接管。值为空串 = 显式删除该头，
	// 供纯 REST 平台去掉 jQuery 时代的后台指纹（如 text/javascript 与 q=0.01）。
	Headers map[string]string
	// FormEncoding 登录表单的请求体编码。""（零值）/"form" =
	// application/x-www-form-urlencoded（jQuery 默认，绝大多数平台）；
	// "json" = application/json，体为档案 Form 各键值组成的 JSON 对象
	// （空键名仍不提交，与 form 路径同契约）。
	FormEncoding string

	CodeUnauthorized int // 未登录码（**仅在 Envelope 为 nil 时生效**，见 Envelope）
	// Envelope 从响应体提取业务状态：返回 (业务码, 是否未登录, 解析错误)。
	// 信封形态随站点而异——整数码 `{"code":0}`、字符串码 `{"status":"OK"}`、
	// 嵌套状态甚至纯文本错误页，引擎不自解任何固定键名。
	// nil = 回落引擎默认（解 `{"code":int}` 并与 CodeUnauthorized 比较），
	// 保留给沿用该信封的档案。**字符串码/纯文本错误页的站点必须显式提供**：
	// 缺钩子时前者会因类型不匹配直接崩、后者同样崩，而「状态键名不同的
	// 未登录响应」更糟——会被当成成功静默放行，令 token 失效后的自动重登永不触发。
	// err != nil 表示「该响应体不是本形态」（如平台返回纯文本页），
	// 引擎据此跳过信封检查，把判定交给各接口的解码钩子。
	Envelope func(body []byte) (code int, unauthorized bool, err error)
	// UnauthorizedStatuses 声明「哪些 HTTP 状态码代表未登录」。**零值 = 不声明 =
	// 行为与本字段引入前逐字不变**（沿用只靠响应体表达未登录的站点，如知到）；
	// 纯 REST 平台按标准声明 401/403 即可接入。
	//
	// 为什么必须下沉：实测平台只回 HTTP 状态码时，引擎走 doRequest 只读响应体、
	// 全程不看 resp.StatusCode，四种形态里两种**静默放行**（401/403 带业务码时
	// err=nil），会话已死引擎却以为还活着，报名全败而日志一片正常；另两种崩在
	// 「响应解析失败」被误判成网络抖动 → 无限重试且**永不触发自动重登**。
	// Envelope 钩子覆盖不了这种形态：它只看 body，body 为空时钩子无从判别。
	//
	// 判据时机在**读响应体之前**——否则 401+空体会先崩在 JSON 解析上，
	// 正是上述静默失效的根因。5xx 不应声明：那是服务端故障，重试即可，
	// 判成会话失效会引发无意义的重登风暴。
	UnauthorizedStatuses []int

	// HasWindowSignal 站点是否下发开窗信号（时间戳或布尔）。
	// false = **退化模式**：探测到非空课程数据即视为开窗。绝不因平台缺此信号而拒绝加载，
	// 代价是该平台失去黄金期 250ms 冲刺精度（退化为 1s 轮询）。
	HasWindowSignal bool
	// OpErrorClassifier 平台错误分类钩子：把站点响应归一为 OpErrorKind。
	// nil = 全部归 OpErrorUnknown（普通业务错误，调用方原文透传）。
	// 判据（码/文案匹配）**只允许存在于站点适配器包内**——这是引擎与调度器零站点感知的保证。
	OpErrorClassifier func(msg string, code int) OpErrorKind
	// Captcha 图形验证码规格（零值 = 不校验）。
	Captcha CaptchaSpec
	// SessionCookies 登录后需补齐的会话 Cookie 占位值（键=Cookie名，值=占位内容）。
	// 用于平台要求某些固定 Cookie 而它只在特定响应里下发的场景（如知到的
	// 某站的频次标记 Cookie）。空 map = 不需要。真实会话值由登录动态更新，此处仅占位。
	SessionCookies map[string]string
	// CaptchaCacheBustParam 验证码 URL 的防缓存参数名（引擎拼 ?<name>=<毫秒>）。空串 = 不拼。
	CaptchaCacheBustParam string

	Form   FormFields
	Login  LoginHooks
	Decode Decoders
}

// pathFields 返回全部必须"非空且以 / 开头"的路径字段（校验用，注释与断言同源）。
func (d SiteDescriptor) pathFields() map[string]string {
	return map[string]string{
		"LoginPath":     d.LoginPath,
		"CaptchaPath":   d.CaptchaPath,
		"DoLoginPath":   d.DoLoginPath,
		"TermsPath":     d.TermsPath,
		"ElectivesPath": d.ElectivesPath,
		"CountsPath":    d.CountsPath,
		"SelectPath":    d.SelectPath,
		"ExitPath":      d.ExitPath,
		"RefererPath":   d.RefererPath,
	}
}

// Validate 站点档案自检：注册表测试、启动期解析（resolveStartupPlatform）与后台
// 热切换（newPlatformRebinder）三处都会消费它——一份缺路径或缺解码钩子的档案会让
// 运行时在第一次请求时才炸（缺钩子是 nil 函数调用硬崩，缺路径拼出无前导斜杠 URL
// 让平台 404），且现场是"未知的解析错误"，故必须在装配面拦下。
func (d SiteDescriptor) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("站点档案缺少 ID")
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("站点档案 %s 缺少名称", d.ID)
	}
	if err := ValidateBaseURL(d.DefaultBaseURL); err != nil {
		return fmt.Errorf("站点档案 %s 的默认地址非法: %w", d.ID, err)
	}
	if strings.TrimSpace(d.DefaultBaseURL) == "" {
		return fmt.Errorf("站点档案 %s 缺少默认站点地址", d.ID)
	}
	for name, p := range d.pathFields() {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("站点档案 %s 的 %s=%q 必须以 / 开头", d.ID, name, p)
		}
	}
	// 鉴权载体名判据随通道而变：Header 通道的平台本就没有查询参数名，
	// 强制它非空等于把纯 REST 平台挡在门外；反之走查询通道却漏给参数名，
	// 引擎会拼出 `?=TOK` 的畸形 URL，平台直接拒而现场报"参数缺失"。
	if d.AuthMode == AuthHeader {
		if strings.TrimSpace(d.HeaderTokenName) == "" {
			return fmt.Errorf("站点档案 %s 声明 Header 鉴权但未给出 HeaderTokenName", d.ID)
		}
	} else if (d.AuthMode == AuthDual || d.AuthMode == AuthQuery) && strings.TrimSpace(d.TokenParam) == "" {
		return fmt.Errorf("站点档案 %s 缺少鉴权载体名 TokenParam", d.ID)
	}
	// Cookie 通道没开的平台（仅查询/仅头）本就没有会话 Cookie 名，强制它非空
	// 等于把这类平台挡在门外。
	if (d.AuthMode == AuthDual || d.AuthMode == AuthCookie) && strings.TrimSpace(d.TokenCookie) == "" {
		return fmt.Errorf("站点档案 %s 缺少鉴权载体名 TokenCookie", d.ID)
	}
	// 声明的未登录状态码必须是合法 HTTP 错误码。挡 2xx/3xx：把它们当未登录会让
	// 正常响应被当成会话失效 → 无限重登且掩盖真实错误。判据放装配面是因为错值
	// 在运行时只表现为「莫名其妙一直重登」，现场极难定位。
	for _, s := range d.UnauthorizedStatuses {
		if s < 400 || s > 599 {
			return fmt.Errorf("站点档案 %s 的 UnauthorizedStatuses 含非错误状态码 %d（必须落在 400~599）", d.ID, s)
		}
	}
	if d.Login.UserAgent == "" {
		return fmt.Errorf("站点档案 %s 缺少 UserAgent", d.ID)
	}
	if d.Login.Custom == nil {
		if d.Login.EncryptIdentification == nil || d.Login.DeviceID == nil {
			return fmt.Errorf("站点档案 %s 缺少登录钩子", d.ID)
		}
	}
	if d.Decode.Terms == nil || d.Decode.Electives == nil || d.Decode.Counts == nil ||
		d.Decode.OpResult == nil || d.Decode.Login == nil || d.Decode.Msg == nil {
		return fmt.Errorf("站点档案 %s 缺少响应解码钩子", d.ID)
	}
	// 四个基础表单键无条件必填；若未走自定义登录，Identification 也必须非空
	requiredForms := map[string]string{
		"Year": d.Form.Year, "Term": d.Form.Term, "IDs": d.Form.IDs, "ClassID": d.Form.ClassID,
	}
	if d.Login.Custom == nil {
		requiredForms["Identification"] = d.Form.Identification
	}
	for name, f := range requiredForms {
		if f == "" {
			return fmt.Errorf("站点档案 %s 缺少表单字段名 %s", d.ID, name)
		}
	}
	// 声明了验证码却没给表单键名：登录链路会把识别结果拼到空键名上（写出`=value`），
	// 平台必拒且现场是"参数缺失"这类难定位的错误——必须在装配面拦下。
	if d.Captcha.Enabled && strings.TrimSpace(d.Form.Captcha) == "" {
		return fmt.Errorf("站点档案 %s 声明有验证码但未给出表单字段名 Form.Captcha", d.ID)
	}
	// 声明了验证码却没给长度下限：识别结果长度无从校验，登录会带着噪声提交。
	if d.Captcha.Enabled && d.Captcha.MinLen <= 0 {
		return fmt.Errorf("站点档案 %s 声明有验证码但未给出 MinLen", d.ID)
	}
	if d.Captcha.MaxLen > 0 && d.Captcha.MaxLen < d.Captcha.MinLen {
		return fmt.Errorf("站点档案 %s 的验证码 MaxLen(%d) 小于 MinLen(%d)", d.ID, d.Captcha.MaxLen, d.Captcha.MinLen)
	}
	// 字符集合法性：残缺写法（如 "[invalid"）恰好能编译成合法正则却会静默剔掉
	// 所有字母数字，必须在装配面拦下，绝不拖到运行时丢字符。
	if err := validCaptchaCharset(d.Captcha.Charset); err != nil {
		return fmt.Errorf("站点档案 %s 的验证码字符集非法: %w", d.ID, err)
	}
	// 声明了设备指纹却没给键名：同理会把指纹拼到空键名上。
	if strings.TrimSpace(d.Form.UniqueID) == "" && d.Login.DeviceID != nil && d.Login.DeviceID("ua", time.Now()) != "" {
		return fmt.Errorf("站点档案 %s 的 DeviceID 钩子会产出非空值但未给出表单字段名 Form.UniqueID", d.ID)
	}
	// 登录体编码只认两种形态：写错值不会崩，只会让平台收到无法解析的请求体，
	// 现场报的是"参数错误"这类难定位的问题，故在装配面拦下。
	if d.FormEncoding != "" && d.FormEncoding != "form" && d.FormEncoding != "json" {
		return fmt.Errorf("站点档案 %s 的 FormEncoding=%q 非法（只允许 form/json）", d.ID, d.FormEncoding)
	}
	return nil
}

// NormalizeBaseURL 规整站点根地址：去掉首尾空白与尾部斜杠。
// 引擎以 baseURL + path 拼接，用户（或档案）若以 / 结尾会拼出 "//electives/..."，
// 部分网关会 404——故拼接前统一规整，调用方无需各自处理。
func NormalizeBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// ValidateBaseURL 校验站点地址：空串合法（语义 = 用档案默认地址）；非空必须是
// http/https 且带主机名。后台保存与启动期解析共用同一判据，避免两处规则分叉。
func ValidateBaseURL(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("站点地址无法解析: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("站点地址必须以 http:// 或 https:// 开头")
	}
	if u.Host == "" {
		return fmt.Errorf("站点地址缺少主机名")
	}
	return nil
}
