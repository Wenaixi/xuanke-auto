package zhidao

import (
	"testing"
	"time"

	"xuanke-auto/backend/internal/upstream"
)

// TestDescriptorValidate 档案自检：缺路径/缺键名/缺钩子会在第一次请求时才炸，
// 且现场是"未知解析错误"——必须在装配面拦下。注册表测试同口径。
func TestDescriptorValidate(t *testing.T) {
	if err := Descriptor().Validate(); err != nil {
		t.Fatalf("内置档案未通过自检: %v", err)
	}
}

// TestDescriptorMatchesLegacyContract 站点契约锚：路径、鉴权载体名、表单键名、
// 状态码逐项钉住。权威锚是本地专有的 archive/legacy（真实站点 JS + HAR 抓包，
// 整树被 .gitignore 忽略、他人克隆环境不可得）——本测试即该基线的提炼与固化，
// 防止改档案时静默漂移。
func TestDescriptorMatchesLegacyContract(t *testing.T) {
	d := Descriptor()

	if d.ID != "zhidao-2026" {
		t.Fatalf("档案 ID 变更会破坏存量部署的 settings.platform_id 反查: %q", d.ID)
	}
	if d.DefaultBaseURL != "https://www.zhidao.fj.cn" {
		t.Fatalf("默认站点地址错误: %q", d.DefaultBaseURL)
	}

	for _, c := range []struct {
		name, got, want string
	}{
		{"LoginPath", d.LoginPath, "/login"},
		{"CaptchaPath", d.CaptchaPath, "/login/captcha"},
		{"DoLoginPath", d.DoLoginPath, "/login/doLogin"},
		{"TermsPath", d.TermsPath, "/electives/select"},
		{"ElectivesPath", d.ElectivesPath, "/electives/select/findElectivesData"},
		{"CountsPath", d.CountsPath, "/electives/select/findElectivesStudentCount"},
		{"SelectPath", d.SelectPath, "/electives/select/selectElectivesClass"},
		{"ExitPath", d.ExitPath, "/electives/select/exitElectivesClass"},
		{"RefererPath", d.RefererPath, "/admin.html"},
		{"TokenParam", d.TokenParam, "idToken"},
		{"TokenCookie", d.TokenCookie, "zd_edu_cookie"},
		{"Form.Year", d.Form.Year, "schoolYear"},
		{"Form.Term", d.Form.Term, "schoolTerm"},
		{"Form.IDs", d.Form.IDs, "ids"},
		{"Form.ClassID", d.Form.ClassID, "classId"},
		{"Form.Captcha", d.Form.Captcha, "captcha"},
		{"Form.Identification", d.Form.Identification, "identification"},
		{"Form.UniqueID", d.Form.UniqueID, "uniqueId"},
		{"Form.PriorityID", d.Form.PriorityID, "priorityId"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if d.CodeOK != 0 || d.CodeUnauthorized != -1 {
		t.Fatalf("状态码语义错误: ok=%d unauthorized=%d", d.CodeOK, d.CodeUnauthorized)
	}
	// 登录钩子必须接真算法（UA 与真实浏览器一致、账密走 RSA、设备指纹复刻前端 JS）。
	if d.Login.UserAgent == "" {
		t.Fatal("缺少 UA")
	}
	if d.Login.EncryptIdentification == nil || d.Login.DeviceID == nil {
		t.Fatal("缺少登录钩子")
	}
}

// TestDecodeElectives HAR 真实响应片段（字段与真实一致）→ 中立数据模型。
func TestDecodeElectives(t *testing.T) {
	raw := `{"code":0,"beginTimes":[1789261200000],"selectElectivesData":[
	  {"publishId":3225,"publishName":"高二年体育","beginDate":"2026-09-13 09:00:00","inDateRange":false,
	   "canSelect":1,"hasSelected":0,"groupCount":1,"totalCount":3,
	   "electivesClassList":[{"id":61115,"publish_id":3225,"course_name":"健美操","class_name":"健美操1、2班",
	     "teacher_name_list":"陈跃强","class_room_name":"操场","lessons_date":null,"selected_count":0,
	     "max_count":36,"plan_count":36,"can_select":false,"btn_type":2,"btn_text":"报名"}]}
	]}`
	data, err := decodeElectives([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 1 {
		t.Fatalf("期望 1 个发布，实际 %d", len(data.Publishes))
	}
	if data.BeginTimes[0] != 1789261200000 {
		t.Fatalf("beginTimes 未解析: %+v", data.BeginTimes)
	}
	p := data.Publishes[0]
	if p.PublishID != 3225 || p.PublishName != "高二年体育" {
		t.Fatalf("发布解析错误: %+v", p)
	}
	if p.Selectable != nil && *p.Selectable {
		t.Fatal("窗口应未开放")
	}
	if len(p.Classes) != 1 || p.Classes[0].ID != 61115 || p.Classes[0].CourseName != "健美操" {
		t.Fatalf("课程解析错误: %+v", p.Classes)
	}
}

// TestDecodeElectivesDerivesClassFull 满员派生字段 class_full 单一记忆点：
// 名额未公布（max_count=0）、未满、满员三种形态解析后必须 false/false/true——
// 前端与后端共 10 处手写判据收敛为解析端单点下发。
func TestDecodeElectivesDerivesClassFull(t *testing.T) {
	body := []byte(`{"code":0,"selectElectivesData":[{"publishId":1,"electivesClassList":[
		{"id":1,"max_count":0,"selected_count":5},
		{"id":2,"max_count":10,"selected_count":3},
		{"id":3,"max_count":10,"selected_count":10}
	]}]}`)
	ed, err := decodeElectives(body)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(ed.Publishes) != 1 || len(ed.Publishes[0].Classes) != 3 {
		t.Fatalf("发布/课程数量不符: %+v", ed)
	}
	got := map[int]bool{}
	for _, c := range ed.Publishes[0].Classes {
		got[c.ID] = c.ClassFull
	}
	if got[1] || got[2] || !got[3] {
		t.Fatalf("ClassFull 派生错误（1 未公布=false, 2 未满=false, 3 满员=true）: %+v", got)
	}
}

// TestDecodeElectivesEmptyPublishes 窗口关闭形态：成功码 + 空发布 → 空快照（非错误）。
func TestDecodeElectivesEmptyPublishes(t *testing.T) {
	ed, err := decodeElectives([]byte(`{"code":0,"selectElectivesData":[]}`))
	if err != nil {
		t.Fatalf("空发布不应报错: %v", err)
	}
	if len(ed.Publishes) != 0 || len(ed.BeginTimes) != 0 {
		t.Fatalf("期望空快照，实际 %+v", ed)
	}
}

// TestDecodeOpResult 报名/退选成败判据（双布尔）：code==0 && isOk 才算成功——
// 「code=0 但 isOk=false」必须判失败（真实站点前端看 isOk/isFail 布尔）。
func TestDecodeOpResult(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"双真成功", `{"code":0,"isOk":true,"msg":"选课成功！"}`, true},
		{"code=0 但 isOk=false", `{"code":0,"isOk":false,"msg":"选课处理中，请勿重复操作！"}`, false},
		{"code=1 业务失败", `{"code":1,"isOk":false,"msg":"不在选修报名时间范围内，无法选课！"}`, false},
	}
	for _, c := range cases {
		msg, ok, err := decodeOpResult([]byte(c.body))
		if err != nil {
			t.Fatalf("%s: 解析失败 %v", c.name, err)
		}
		if ok != c.want {
			t.Errorf("%s: ok=%v, want %v", c.name, ok, c.want)
		}
		if msg == "" {
			t.Errorf("%s: 消息不应为空（管理员据此排障）", c.name)
		}
	}
}

// TestDecodeLogin 登录响应：isOk 为假或 token 为空一律判被拒。
func TestDecodeLogin(t *testing.T) {
	tok, err := decodeLogin([]byte(`{"code":0,"isOk":true,"token":"tok-1","msg":"登录成功"}`))
	if err != nil || tok != "tok-1" {
		t.Fatalf("成功响应应返回 token，got %q %v", tok, err)
	}
	if _, err := decodeLogin([]byte(`{"code":1,"isOk":false,"msg":"验证码错误"}`)); err == nil {
		t.Fatal("isOk=false 必须判被拒")
	}
	if _, err := decodeLogin([]byte(`{"code":0,"isOk":true,"token":""}`)); err == nil {
		t.Fatal("token 为空必须判被拒")
	}
}

// TestDecodeTerms 学期列表：code!=0 报错，成功返回列表。
func TestDecodeTerms(t *testing.T) {
	terms, err := decodeTerms([]byte(`{"code":0,"currentYearTermList":[{"schoolYear":2026,"schoolTerm":1,"selected":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 1 || terms[0].SchoolYear != 2026 || !terms[0].Selected {
		t.Fatalf("学期解析错误: %+v", terms)
	}
	if _, err := decodeTerms([]byte(`{"code":1,"msg":"学期查询失败"}`)); err == nil {
		t.Fatal("code!=0 必须报错")
	}
}

// TestDecodeMsg 错误文案提取（token 失效时附在错误里供排障）。
func TestDecodeMsg(t *testing.T) {
	if got := decodeMsg([]byte(`{"code":-1,"msg":"您未登录,请刷新页面重新登录"}`)); got != "您未登录,请刷新页面重新登录" {
		t.Fatalf("文案提取错误: %q", got)
	}
}

// TestEncryptIdentification RSA 1024 PKCS1 v1.5 密文 base64 后至少 170+ 字符。
func TestEncryptIdentification(t *testing.T) {
	s, err := encryptIdentification("***REMOVED***", "***REMOVED***")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) < 100 {
		t.Fatalf("cipher too short: %d", len(s))
	}
}

// TestUniqueDeviceID 设备指纹非空（复刻前端 getUniqueDeviceId）。
func TestUniqueDeviceID(t *testing.T) {
	tt, err := time.ParseInLocation("2006-01-02T15:04:05-07:00", "2026-09-13T09:00:00+08:00", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if got := uniqueDeviceID("test-ua", tt); got == "" {
		t.Fatal("empty device id")
	}
}

// TestDescriptorCaptchaSpecAndSignals 档案新字段必须如实映射知到的真实形态。
// 这些值曾写死在引擎与 accounts 包（验证码 3~5 位、access_limit_cookie、?v=），
// 现在归档案所有，测试锁死映射关系防止回退。
func TestDescriptorCaptchaSpecAndSignals(t *testing.T) {
	d := Descriptor()
	if !d.HasWindowSignal {
		t.Fatal("知到下发 inDateRange 与 beginTimes，HasWindowSignal 必须为真")
	}
	if !d.Captcha.Enabled {
		t.Fatal("知到有图形验证码，Captcha.Enabled 必须为真")
	}
	if d.Captcha.Charset != "A-Za-z0-9" || d.Captcha.MinLen != 3 || d.Captcha.MaxLen != 5 {
		t.Fatalf("知到验证码为 3~5 位纯英数字，实际 %+v", d.Captcha)
	}
	if d.CaptchaCacheBustParam != "v" {
		t.Fatalf("知到验证码URL 拼 ?v=，实际 %q", d.CaptchaCacheBustParam)
	}
	if d.SessionCookies["access_limit_cookie"] != "1" {
		t.Fatalf("access_limit_cookie 占位丢失: %+v", d.SessionCookies)
	}
	if d.OpErrorClassifier == nil {
		t.Fatal("知到必须提供 OpErrorClassifier")
	}
}

// TestClassifyOpError 站点错误分类（表驱动）——这是全仓唯一匹配知到文案的位置。
// 关键用例：含「已满员」的非满员失败文案不得被判为满员之外的动作，
// 「选课处理中」是并发重复提交而非风控（归错会让 30s 退避套到每次重复点击）。
func TestClassifyOpError(t *testing.T) {
	cl := Descriptor().OpErrorClassifier
	cases := []struct {
		name string
		msg  string
		want upstream.OpErrorKind
	}{
		{"风控频繁", "操作过于频繁，请稍后重试", upstream.OpErrorRateLimited},
		{"429 字样", "HTTP 429 Too Many Requests", upstream.OpErrorRateLimited},
		{"操作太快", "操作太快了", upstream.OpErrorRateLimited},
		{"窗口未开放", "不在选修报名时间范围内，无法选课！", upstream.OpErrorWindowClosed},
		{"选课未开启", "选课未开启", upstream.OpErrorWindowClosed},
		{"报名时间已结束", "报名时间已结束", upstream.OpErrorWindowClosed},
		{"窗口关闭", "当前窗口已关闭", upstream.OpErrorWindowClosed},
		{"满员", "该课程已满员", upstream.OpErrorClassFull},
		{"名额已满", "名额已满，请选择其他课程", upstream.OpErrorClassFull},
		{"重复提交非风控", "选课处理中，请勿重复操作！", upstream.OpErrorUnknown},
		{"课程不存在", "课程不存在", upstream.OpErrorUnknown},
		{"空文案", "", upstream.OpErrorUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cl(c.msg, 1); got != c.want {
				t.Fatalf("classifyOpError(%q) = %v, want %v", c.msg, got, c.want)
			}
		})
	}
}


// TestDecodeElectivesMapsBtnTypeToNeutralAction站点按钮编码必须映射为中立操作类型。
// 引擎与前端只认enroll/withdraw/none，绝不认btn_type 的 1/2 编码——
// 「站点怎么表示按钮」属站点事实，泄漏进 UI 就等于把某年的接口形态焊死。
func TestDecodeElectivesMapsBtnTypeToNeutralAction(t *testing.T) {
	body := []byte(`{"code":0,"selectElectivesData":[{"publishId":1,"electivesClassList":[
		{"id":1,"btn_type":1,"can_select":true,"max_count":0,"selected_count":0},
		{"id":2,"btn_type":2,"can_select":true,"max_count":0,"selected_count":0},
		{"id":3,"btn_type":9,"can_select":true,"max_count":0,"selected_count":0}
	]}]}`)
	data, err := decodeElectives(body)
	if err != nil {
		t.Fatal(err)
	}
	cs := data.Publishes[0].Classes
	if len(cs) != 3 {
		t.Fatalf("应解出 3 门课，实际 %d", len(cs))
	}
	if cs[0].Action != "withdraw" {
		t.Fatalf("btn_type=1 应映射 withdraw，实际 %q", cs[0].Action)
	}
	if cs[1].Action != "enroll" {
		t.Fatalf("btn_type=2 应映射 enroll，实际 %q", cs[1].Action)
	}
	if cs[2].Action != "none" {
		t.Fatalf("btn_type=9（其他值，官网不渲染按钮）应映射 none，实际 %q", cs[2].Action)
	}
}

// TestDecodeElectivesSelectableTriState 发布级开窗信号必须映射为三态 selectable。
// nil = 站点不下发该信号（退化模式据此判定），与"站点说未开"是不同事实。
func TestDecodeElectivesSelectableTriState(t *testing.T) {
	body := []byte(`{"code":0,"selectElectivesData":[
		{"publishId":1,"inDateRange":true,"electivesClassList":[]},
		{"publishId":2,"inDateRange":false,"electivesClassList":[]}
	]}`)
	data, err := decodeElectives(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 2 {
		t.Fatalf("应解出 2 个发布，实际 %d", len(data.Publishes))
	}
	p1 := data.Publishes[0]
	if p1.Selectable == nil || !*p1.Selectable {
		t.Fatalf("inDateRange=true 应映射 selectable=true，实际 %v", p1.Selectable)
	}
	p2 := data.Publishes[1]
	if p2.Selectable == nil || *p2.Selectable {
		t.Fatalf("inDateRange=false 应映射 selectable=false，实际 %v", p2.Selectable)
	}
}
