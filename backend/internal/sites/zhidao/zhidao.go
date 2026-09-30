// Package zhidao 知道教育平台（2026 版）站点适配器。
//
// 本包只承载"这个站点长什么样"：接口路径、登录表单键名、鉴权载体名、状态码语义、
// 响应解码与登录算法。**流程**（会话、探测、提交、退避、黄金期、多账号隔离）全在
// upstream 引擎，调度器与前端一行都不认识本站点。新增一个年份/站点 = 新增一个同形
// 适配器包 + 在 internal/sites 注册表加一行。
//
// 站点契约的唯一权威锚是本地专有基线 archive/legacy/（真实站点 JS + HAR 抓包），
// 该基线整树被 .gitignore 忽略、他人克隆环境里不可得——本包即该基线的提炼与固化。
package zhidao

import (
	"encoding/json"
	"fmt"
	"strings"

	"xuanke-auto/backend/internal/upstream"
)

// ID 档案标识（落库 settings.platform_id，一经发布不可改：存量部署按它反查档案）。
const ID = "zhidao-2026"

// defaultBaseURL 站点根地址默认值（管理员可在后台覆盖，换域名/镜像免发版）。
const defaultBaseURL = "https://www.zhidao.fj.cn"

// userAgent 站点要求的统一 UA（登录页与已登录接口共用同一份，与真实浏览器一致）。
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"

// Descriptor 返回本平台的站点档案（注册表与测试共用同一出口，绝不各自手抄一份）。
func Descriptor() upstream.SiteDescriptor {
	return upstream.SiteDescriptor{
		ID:             ID,
		Name:           "知道教育平台（2026 版）",
		Note:           "2026 年春季学期实测接口；站点地址可在下方覆盖（换域名/镜像免发版）",
		DefaultBaseURL: defaultBaseURL,

		LoginPath:   "/login",
		CaptchaPath: "/login/captcha",
		DoLoginPath: "/login/doLogin",

		TermsPath:     "/electives/select",
		ElectivesPath: "/electives/select/findElectivesData",
		CountsPath:    "/electives/select/findElectivesStudentCount",
		SelectPath:    "/electives/select/selectElectivesClass",
		ExitPath:      "/electives/select/exitElectivesClass",
		RefererPath:   "/admin.html",

		TokenParam:  "idToken",
		TokenCookie: "zd_edu_cookie",

		CodeOK:           0,
		CodeUnauthorized: -1,

		// 下发inDateRange 与 beginTimes 两种开窗信号，引擎走精确判定。
		HasWindowSignal: true,
		// 站点报文的分类判据。这是全仓**唯一**允许匹配知到文案的位置——
		// 引擎与调度器只消费 upstream.OpErrorKind 枚举位，零站点感知。
		OpErrorClassifier: classifyOpError,
		// 真实站点验证码为 3~5 位纯英数字（HAR 抓包确认）。
		Captcha: upstream.CaptchaSpec{
			Enabled: true,
			Charset: "A-Za-z0-9",
			MinLen:  3,
			MaxLen:  5,
		},
		// 站点要求该Cookie 标记访问频次；真实值由登录动态下发，此处仅占位。
		SessionCookies:       map[string]string{"access_limit_cookie": "1"},
		CaptchaCacheBustParam: "v",

		Form: upstream.FormFields{
			Year:           "schoolYear",
			Term:           "schoolTerm",
			IDs:            "ids",
			ClassID:        "classId",
			Captcha:        "captcha",
			Identification: "identification",
			UniqueID:       "uniqueId",
			PriorityID:     "priorityId",
		},
		Login: upstream.LoginHooks{
			UserAgent:             userAgent,
			EncryptIdentification: encryptIdentification,
			DeviceID:              uniqueDeviceID,
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

// decodeTerms 学期列表：currentYearTermList + code/msg。
func decodeTerms(body []byte) ([]upstream.YearTerm, error) {
	var j struct {
		Code                int                 `json:"code"`
		Msg                 string              `json:"msg"`
		CurrentYearTermList []upstream.YearTerm `json:"currentYearTermList"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, err
	}
	if j.Code != 0 {
		return nil, fmt.Errorf("学期列表错误 code=%d: %s", j.Code, j.Msg)
	}
	return j.CurrentYearTermList, nil
}

// decodeElectives 课程数据：selectElectivesData 下的发布与课程列表。
func decodeElectives(body []byte) (*upstream.ElectivesData, error) {
	var raw struct {
		Code                int     `json:"code"`
		Msg                 string  `json:"msg"`
		BeginTimes          []int64 `json:"beginTimes"`
		SelectElectivesData []struct {
			PublishID   int              `json:"publishId"`
			PublishName string           `json:"publishName"`
			BeginDate   string           `json:"beginDate"`
			InDateRange bool             `json:"inDateRange"`
			CanSelect   int              `json:"canSelect"`
			HasSelected int              `json:"hasSelected"`
			GroupCount  int              `json:"groupCount"`
			TotalCount  int              `json:"totalCount"`
			Classes     []upstream.Class `json:"electivesClassList"`
		} `json:"selectElectivesData"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	// 站点在选课窗口关闭后对课程接口返回成功码但空发布。
	// 这不是 token 失效也非错误——按业务"无课可报"处理，返回空快照即可，
	// 避免上层引擎因快照为空而陷入学期列表兜底重试。
	if raw.Code == 0 && len(raw.SelectElectivesData) == 0 {
		return &upstream.ElectivesData{}, nil
	}
	out := &upstream.ElectivesData{BeginTimes: raw.BeginTimes}
	for _, p := range raw.SelectElectivesData {
		// class_full 单一记忆点：派生判据只在解析端算一次，
		// 调度器与前端一律读本字段，不再各自手写 MaxCount>0 && SelectedCount>=MaxCount。
		for i := range p.Classes {
			p.Classes[i].ClassFull = p.Classes[i].MaxCount > 0 && p.Classes[i].SelectedCount >= p.Classes[i].MaxCount
		}
		out.Publishes = append(out.Publishes, upstream.Publish{
			PublishID:   p.PublishID,
			PublishName: p.PublishName,
			BeginDate:   p.BeginDate,
			InDateRange: p.InDateRange,
			CanSelect:   p.CanSelect,
			HasSelected: p.HasSelected,
			GroupCount:  p.GroupCount,
			TotalCount:  p.TotalCount,
			Classes:     p.Classes,
		})
	}
	return out, nil
}

// decodeCounts 实时人数：countList（站点不下发 maxCount，故 MaxCount 恒 0，仅防御性保留）。
func decodeCounts(body []byte) ([]upstream.CountEntry, error) {
	var j struct {
		Code      int                   `json:"code"`
		CountList []upstream.CountEntry `json:"countList"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, err
	}
	if j.Code != 0 {
		return nil, fmt.Errorf("人数查询错误: %s", decodeMsg(body))
	}
	return j.CountList, nil
}

// decodeOpResult 报名/退选响应 → (消息, 是否成功)。
// 本站点成败判据 = code==0 && isOk（双布尔；code=1 的业务失败不在 throw 名单，
// 真实前端看 isOk/isFail 布尔）。判据刻意留在站点侧：引擎只消费"成功/失败"这一位事实。
func decodeOpResult(body []byte) (string, bool, error) {
	var j struct {
		Code int    `json:"code"`
		IsOk bool   `json:"isOk"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", false, err
	}
	return j.Msg, j.Code == 0 && j.IsOk, nil
}

// decodeLogin 登录响应 → token（isOk 为假或 token 为空一律判登录被拒）。
func decodeLogin(body []byte) (string, error) {
	var j struct {
		Code  int    `json:"code"`
		IsOk  bool   `json:"isOk"`
		Token string `json:"token"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", fmt.Errorf("登录响应解析失败: %w", err)
	}
	if !j.IsOk || j.Token == "" {
		return "", fmt.Errorf("登录被拒绝: %s", j.Msg)
	}
	return j.Token, nil
}

// decodeMsg 从错误响应体提取可读文案（token 失效时附在错误里供排障）。
func decodeMsg(body []byte) string {
	var j struct {
		Msg string `json:"msg"`
	}
	json.Unmarshal(body, &j)
	return j.Msg
}

// classifyOpError 把知到的报名/退选失败响应归一为结构化分类。
// 判据依据真实前端逆向结论（记忆库第 4 节）：知到的 code=1 是**通用**业务失败
//（语义"重试可能有用"），code=0 但 isOk=false 同为失败；具体原因只体现在文案里。
// 故判据必须读文案——而**读文案这件事只允许发生在站点适配器包内**，引擎与调度器
// 拿到的是 upstream.OpErrorKind 枚举位，对站点零感知。
//
// 「选课处理中，请勿重复操作！」刻意归 Unknown 而非 RateLimited：它是并发重复提交
//（code=0 + isOk=false），不是风控限流；归错会让 30s 退避套到每次重复点击上。
// 满员文案归ClassFull 供调度器直接记 full（真实前端无maxCount 判据，满员靠文案）。
func classifyOpError(msg string, code int) upstream.OpErrorKind {
	switch {
	case strings.Contains(msg, "频繁") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "稍后重试") || strings.Contains(msg, "太快"):
		return upstream.OpErrorRateLimited
	case strings.Contains(msg, "关闭") || strings.Contains(msg, "未开启") ||
		strings.Contains(msg, "报名时间") || strings.Contains(msg, "已结束") ||
		strings.Contains(msg, "不在选修"):
		return upstream.OpErrorWindowClosed
	case strings.Contains(msg, "已满") || strings.Contains(msg, "名额已满"):
		return upstream.OpErrorClassFull
	}
	return upstream.OpErrorUnknown
}
