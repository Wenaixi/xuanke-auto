package upstream

import (
	"encoding/json"
	"errors"
	"time"
)

// testDescriptor 引擎测试夹具的站点档案：路径与键名沿用真实站点的形状（各 mock
// 服务器本来就按该形状应答），**解码钩子刻意只实现信封层**——真实站点的字段名解码
// 归 internal/sites/zhidao 并由其测试钉住；引擎侧只关心"调用钩子并消费结果"。
// 这样引擎测试不会因为某个站点的字段改名而变红，站点契约测试也不会漏。
func testDescriptor(baseURL string) SiteDescriptor {
	return SiteDescriptor{
		ID:             "test-site",
		Name:           "测试站点",
		Note:           "单测夹具",
		DefaultBaseURL: baseURL,

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

		CodeUnauthorized: -1,

		// 夹具模拟"有图形验证码"的站点：登录测试的 mock 会校验表单里的captcha 键。
		// 不声明 Enabled 时登录链路会跳过验证码段，mock 随即返回"参数缺失"。
		HasWindowSignal: true,
		Captcha:         CaptchaSpec{Enabled: true, Charset: "A-Za-z0-9", MinLen: 3, MaxLen: 5},

		Form: FormFields{
			Year:           "schoolYear",
			Term:           "schoolTerm",
			IDs:            "ids",
			ClassID:        "classId",
			Captcha:        "captcha",
			Identification: "identification",
			UniqueID:       "uniqueId",
			PriorityID:     "priorityId",
		},
		Login: LoginHooks{
			UserAgent: "xuanke-test-ua",
			EncryptIdentification: func(account, _ string) (string, error) {
				return "enc-" + account, nil
			},
			DeviceID: func(ua string, _ time.Time) string { return "dev-" + ua },
		},
		Decode: Decoders{
			// 学期/课程/人数解码归站点包；引擎测试不消费它们（这里的 mock 也不发这些响应）。
			Terms:     func([]byte) ([]YearTerm, error) { return nil, errors.New("夹具未实现学期解码") },
			Electives: func([]byte) (*ElectivesData, error) { return &ElectivesData{}, nil },
			Counts:    func([]byte) ([]CountEntry, error) { return nil, nil },
			OpResult:  decodeResultEnvelope,
			Login:     decodeTokenEnvelope,
			Msg:       decodeMsgEnvelope,
		},
	}
}

// newTestClient 引擎测试统一的客户端构造（生效地址 = 夹具默认地址）。
func newTestClient(baseURL string, vision VisionConfig) *Client {
	return New(testDescriptor(baseURL), baseURL, vision)
}

// decodeResultEnvelope 通用信封解码：code/isOk/msg（真实站点判据是 code==0 && isOk，
// 由站点包钉住；夹具沿用同一形状以保证引擎测试覆盖"业务失败"分支）。
func decodeResultEnvelope(body []byte) (string, bool, error) {
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

func decodeTokenEnvelope(body []byte) (string, error) {
	var j struct {
		IsOk  bool   `json:"isOk"`
		Token string `json:"token"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return "", err
	}
	if !j.IsOk || j.Token == "" {
		return "", errors.New("登录被拒绝: " + j.Msg)
	}
	return j.Token, nil
}

func decodeMsgEnvelope(body []byte) string {
	var j struct {
		Msg string `json:"msg"`
	}
	json.Unmarshal(body, &j)
	return j.Msg
}
