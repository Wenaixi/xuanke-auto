// Package testsite 第二站点适配器（解耦的机械验收物）。
//
// 本包的存在意义不是"多一个平台"，而是**机械证明**「加平台不需要动引擎」：
// 它刻意在每个维度上都与知道教育平台不同——
//   - 无开窗信号（HasWindowSignal=false，走引擎退化模式）；
//   - 按钮编码是字符串 "ENROLL"/"WITHDRAW" 而非知到的 1/2；
//   - 验证码 4 位纯数字（知到是 3~5 位字母数字）；
//   - 无防缓存参数（知到拼 ?v=）；
//   - 无会话 Cookie（知到需 access_limit_cookie=1）；
//   - 登录表单无设备指纹、无优先身份（知到两者都有）；
//   - 错误分类用英文码（知到用中文文案）；
//   - 鉴权只经 Header 语义（知到走 URL 参数 + Cookie 双通道）；
//   - 无学期概念、无「发布」概念（靠虚拟层补齐）；
//   - 成败判据是单布尔 status=="OK"（知到是 code==0 && isOk 双布尔）。
//
// 任何人往引擎/调度器里加了新的知到假设，这个包的形状会让它红。
// 它不对应任何真实站点，部署不应选择本档案。
package testsite

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"xuanke-auto/backend/internal/upstream"
)

// ID 档案标识（落库 settings.platform_id，一经发布不可改）。
const ID = "testsite"

// defaultBaseURL 站点根地址默认值（管理员可在后台覆盖）。
const defaultBaseURL = "https://testsite.example.com"

// userAgent 站点要求的统一 UA。
const userAgent = "Mozilla/5.0 (compatible; TestSiteBot/1.0)"

// errRejected 登录被拒（站点不返回可读文案时用它兜底）。
var errRejected = errors.New("登录被拒绝")

// Descriptor 返回本档案（注册表与测试共用同一出口，绝不各自手抄一份）。
func Descriptor() upstream.SiteDescriptor {
	return upstream.SiteDescriptor{
		ID:             ID,
		Name:           "测试站点（解耦验收）",
		Note:           "形态刻意不同于知到：无开窗信号/字符串按钮码/4位纯数字验证码/英文错误码/虚拟学期",
		DefaultBaseURL: defaultBaseURL,

		LoginPath:   "/auth/login",
		CaptchaPath: "/auth/captcha",
		DoLoginPath: "/auth/submit",

		TermsPath:     "/term/list",
		ElectivesPath: "/course/list",
		CountsPath:    "/course/count",
		SelectPath:    "/course/enroll",
		ExitPath:      "/course/withdraw",
		RefererPath:   "/portal",

		// 鉴权载体名与知到完全不同（知到是 idToken + zd_edu_cookie）。
		TokenParam:  "tsToken",
		TokenCookie: "ts_session",

		CodeOK:           0,
		CodeUnauthorized: -9,

		// 不下发任何开窗信号 → 引擎走退化模式（探测到数据即视为开窗）。
		HasWindowSignal: false,
		// 英文码分类（知到是中文文案匹配）——判据确实随站点走。
		OpErrorClassifier: classifyOpError,
		Captcha: upstream.CaptchaSpec{
			Enabled: true,
			Charset: "0-9",
			MinLen:  4,
			MaxLen:  4,
		},
		CaptchaCacheBustParam: "",        // 不拼防缓存参数
		SessionCookies:       map[string]string{}, // 无会话 Cookie 需求

		Form: upstream.FormFields{
			Year:           "year",
			Term:           "term",
			IDs:            "ids",
			ClassID:        "courseId",
			Captcha:        "captchaCode",
			Identification: "account", // 明文提交账密
			UniqueID:       "",        // 无设备指纹
			PriorityID:     "",        // 无优先身份
		},
		Login: upstream.LoginHooks{
			UserAgent: userAgent,
			// 明文账密（知到走 RSA 加密）——刻意对照，证明加密算法是站点事实。
			EncryptIdentification: func(account, password string) (string, error) {
				return account + ":" + password, nil
			},
			DeviceID: func(ua string, now time.Time) string { return "" },
		},
		Decode: upstream.Decoders{
			Terms:     decodeTerms,
			Electives: decodeElectives,
			Counts:    decodeCounts,
			OpResult:  decodeOpResult,
			Login:     decodeLogin,
			Msg:       decodeMsg,
		},
	}
}

// decodeTerms 站点无学期概念，返回唯一虚拟学期（Selected=true）供引擎的兜底
// 重试路径使用（引擎的 YearTerm 结构体与兜底逻辑零改动）。
func decodeTerms(body []byte) ([]upstream.YearTerm, error) {
	return []upstream.YearTerm{{SchoolYear: 0, SchoolTerm: 0, Selected: true}}, nil
}

// decodeElectives 解码课程列表并造一个虚拟发布（站点无「发布」概念）。
// Selectable 恒为 nil——testsite 不下发开窗信号，引擎据此走退化模式。
func decodeElectives(body []byte) (*upstream.ElectivesData, error) {
	var raw struct {
		Code int `json:"code"`
		Data struct {
			Classes []struct {
				ID       int    `json:"id"`
				Group    string `json:"group"`
				Name     string `json:"name"`
				Teacher  string `json:"teacher"`
				Room     string `json:"room"`
				Capacity int    `json:"capacity"`
				Enrolled int    `json:"enrolled"`
				Action   string `json:"action"` // 站点直接下发字符串语义
				Enabled  bool   `json:"enabled"`
			} `json:"classes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := &upstream.ElectivesData{}
	if len(raw.Data.Classes) == 0 {
		// 空课程 = 空快照（窗口关闭形态），不是错误。
		return out, nil
	}
	pub := upstream.Publish{
		PublishID:   1,
		PublishName: "全部课程",
		BeginDate:   "",
		Selectable:  nil, // 不下发开窗信号
		TotalCount:  len(raw.Data.Classes),
	}
	for _, c := range raw.Data.Classes {
		pub.Classes = append(pub.Classes, upstream.Class{
			ID:              c.ID,
			PublishID:       1,
			CourseName:      c.Name,
			ClassName:       c.Group,
			TeacherNameList: c.Teacher,
			ClassroomName:   c.Room,
			SelectedCount:   c.Enrolled,
			MaxCount:        c.Capacity,
			// 满员派生在解析端算一次（引擎与前端一律读 ClassFull）。
			ClassFull:  c.Capacity > 0 && c.Enrolled >= c.Capacity,
			CanSelect:  c.Enabled,
			Action:     mapAction(c.Action),
			ActionText: "",
		})
	}
	out.Publishes = append(out.Publishes, pub)
	return out, nil
}

// mapAction 把站点的字符串按钮语义映射为中立操作类型。未知值 → none（不渲染按钮）。
func mapAction(a string) string {
	switch strings.ToLower(strings.TrimSpace(a)) {
	case "enroll":
		return "enroll"
	case "withdraw":
		return "withdraw"
	}
	return "none"
}

// decodeCounts 实时人数（字段名与形态也随站点：知到是 countList/selectedCount）。
func decodeCounts(body []byte) ([]upstream.CountEntry, error) {
	var raw struct {
		Code int `json:"code"`
		Data []struct {
			ID       int `json:"id"`
			Enrolled int `json:"enrolled"`
			Capacity int `json:"capacity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]upstream.CountEntry, 0, len(raw.Data))
	for _, d := range raw.Data {
		out = append(out, upstream.CountEntry{ID: d.ID, SelectedCount: d.Enrolled, MaxCount: d.Capacity})
	}
	return out, nil
}

// decodeOpResult 成败判据 = code==0 && status=="OK"（单布尔约定，
// 与知到的 code==0 && isOk 双布尔刻意不同）。
func decodeOpResult(body []byte) (string, bool, error) {
	var j struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", false, err
	}
	ok := j.Code == 0 && j.Status == "OK"
	return j.Status, ok, nil
}

// decodeLogin 登录响应取 token；code 非 0 或 token 空一律判被拒。
func decodeLogin(body []byte) (string, error) {
	var j struct {
		Code  int    `json:"code"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", err
	}
	if j.Code != 0 || j.Token == "" {
		return "", errRejected
	}
	return j.Token, nil
}

// decodeMsg 提取可读文案（英文码）；解析失败返回空串。
func decodeMsg(body []byte) string {
	var j struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(body, &j) == nil {
		return j.Status
	}
	return ""
}

// classifyOpError 英文码分类（知到用中文文案）——判据确实随站点走，
// 引擎与调度器只消费 upstream.OpErrorKind 枚举位。
func classifyOpError(msg string, code int) upstream.OpErrorKind {
	switch msg {
	case "RATE_LIMITED":
		return upstream.OpErrorRateLimited
	case "WINDOW_CLOSED":
		return upstream.OpErrorWindowClosed
	case "CLASS_FULL":
		return upstream.OpErrorClassFull
	}
	return upstream.OpErrorUnknown
}
