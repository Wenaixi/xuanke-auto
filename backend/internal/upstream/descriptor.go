package upstream

import (
	"fmt"
	"net/url"
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
}

// Decoders 各接口响应的解码钩子：字段名与响应形态由站点决定，引擎不猜任何键名。
type Decoders struct {
	Terms     func(body []byte) ([]YearTerm, error)
	Electives func(body []byte) (*ElectivesData, error)
	Counts    func(body []byte) ([]CountEntry, error)
	// OpResult 报名/退选响应 → (平台消息, 平台是否判定成功, 解析错误)。
	// 成败判据（code==0 && isOk 这类双布尔约定）刻意留在站点侧：不同站点约定不同，
	// 引擎只消费"成功/失败"这一位事实。
	OpResult func(body []byte) (msg string, ok bool, err error)
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

	CodeOK           int // 业务成功码
	CodeUnauthorized int // 未登录码（引擎据此上抛 ErrUnauthorized）

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
	if d.TokenParam == "" || d.TokenCookie == "" {
		return fmt.Errorf("站点档案 %s 缺少鉴权载体名（TokenParam/TokenCookie）", d.ID)
	}
	if d.Login.UserAgent == "" {
		return fmt.Errorf("站点档案 %s 缺少 UserAgent", d.ID)
	}
	if d.Login.EncryptIdentification == nil || d.Login.DeviceID == nil {
		return fmt.Errorf("站点档案 %s 缺少登录钩子", d.ID)
	}
	if d.Decode.Terms == nil || d.Decode.Electives == nil || d.Decode.Counts == nil ||
		d.Decode.OpResult == nil || d.Decode.Login == nil || d.Decode.Msg == nil {
		return fmt.Errorf("站点档案 %s 缺少响应解码钩子", d.ID)
	}
	for name, f := range map[string]string{
		"Year": d.Form.Year, "Term": d.Form.Term, "IDs": d.Form.IDs, "ClassID": d.Form.ClassID,
	} {
		if f == "" {
			return fmt.Errorf("站点档案 %s 缺少表单字段名 %s", d.ID, name)
		}
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
