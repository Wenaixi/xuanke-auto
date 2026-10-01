package upstream

import (
	"strings"
	"testing"
	"time"
)

// 档案自检的负向测试：每条判据都必须有"破坏后确实报错且点名原因"的守护。
// 正向测试（内置档案能通过自检）是恒真型断言——把判据改坏成永远通过照样绿，
// 故必须有配套的负向用例。
func TestValidateRejectsIncompleteDescriptor(t *testing.T) {
	cases := []struct {
		name    string
		breakFn func(d *SiteDescriptor)
		wantMsg string
	}{
		{
			name:    "声明有验证码但未给表单键名",
			breakFn: func(d *SiteDescriptor) { d.Form.Captcha = "" },
			wantMsg: "Form.Captcha",
		},
		{
			name:    "声明有验证码但未给长度下限",
			breakFn: func(d *SiteDescriptor) { d.Captcha.MinLen = 0 },
			wantMsg: "MinLen",
		},
		{
			name:    "验证码上限小于下限",
			breakFn: func(d *SiteDescriptor) { d.Captcha.MaxLen = 1 },
			wantMsg: "MaxLen",
		},
		{
			name:    "设备指纹产出非空但未给表单键名",
			breakFn: func(d *SiteDescriptor) { d.Form.UniqueID = "" },
			wantMsg: "Form.UniqueID",
		},
		{
			name:    "未登录状态码声明了 2xx（正常响应会被当会话失效）",
			breakFn: func(d *SiteDescriptor) { d.UnauthorizedStatuses = []int{200} },
			wantMsg: "UnauthorizedStatuses",
		},
		{
			name:    "未登录状态码声明了 3xx（重定向会被当会话失效）",
			breakFn: func(d *SiteDescriptor) { d.UnauthorizedStatuses = []int{302} },
			wantMsg: "UnauthorizedStatuses",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := testDescriptor("http://x.test")
			c.breakFn(&d)
			err := d.Validate()
			if err == nil {
				t.Fatalf("破坏 %s 后 Validate 应报错", c.name)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Fatalf("报错应点名 %s，实际: %v", c.wantMsg, err)
			}
		})
	}
}

// TestValidateAcceptsProfileWithoutCaptcha 无验证码平台必须能通过自检——
// 「登录链路不被验证码绑死」是解耦的一部分，装配面不能反过来要求所有平台都有验证码。
func TestValidateAcceptsProfileWithoutCaptcha(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.Captcha = CaptchaSpec{Enabled: false}
	d.Form.Captcha = ""
	if err := d.Validate(); err != nil {
		t.Fatalf("无验证码平台应通过自检: %v", err)
	}
}

// TestValidateAcceptsProfileWithoutDeviceID 无设备指纹平台（钩子恒返回空）必须能通过自检。
func TestValidateAcceptsProfileWithoutDeviceID(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.Form.UniqueID = ""
	d.Login.DeviceID = func(ua string, now time.Time) string { return "" }
	if err := d.Validate(); err != nil {
		t.Fatalf("无设备指纹平台应通过自检: %v", err)
	}
}

// TestCaptchaSpecNormalizeAndAcceptable 规格的两个纯函数：净化与长度门禁。
// 零值语义 = 不校验（适配未声明规格的平台），非零值严格按声明执行。
func TestCaptchaSpecNormalizeAndAcceptable(t *testing.T) {
	cases := []struct {
		name   string
		spec   CaptchaSpec
		raw    string
		wantN  string
		wantOK bool
	}{
		{"知到规格剔汉字", CaptchaSpec{Charset: "A-Za-z0-9"}, " ab掀c1 ", "abc1", true},
		{"纯数字规格剔字母", CaptchaSpec{Charset: "0-9"}, "1a2b3c4d", "1234", true},
		{"零值规格只去空白", CaptchaSpec{}, "  a-b  ", "a-b", true},
		{"零值规格长度恒可接受", CaptchaSpec{}, "任意长度", "任意长度", true},
		{"下限门禁拒绝过短", CaptchaSpec{MinLen: 4}, "123", "123", false},
		{"上限门禁拒绝过长", CaptchaSpec{MaxLen: 4}, "12345", "12345", false},
		{"区间内通过", CaptchaSpec{MinLen: 3, MaxLen: 5}, "abcd", "abcd", true},
		{"纯符号类按声明过滤", CaptchaSpec{Charset: "0-9!@#"}, "1a!2@", "1!2@", true},
		{"中日韩字符集按声明保留", CaptchaSpec{Charset: "0-9\\p{Han}"}, "1中a2", "1中2", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.spec.Normalize(c.raw)
			if got != c.wantN {
				t.Fatalf("Normalize(%q) = %q, want %q", c.raw, got, c.wantN)
			}
			if ok := c.spec.Acceptable(len([]rune(got))); ok != c.wantOK {
				t.Fatalf("Acceptable(%d 位 %q) = %v, want %v", len([]rune(got)), got, ok, c.wantOK)
			}
		})
	}
}

// TestValidCaptchaCharset 字符集合法性必须能识别「碰巧能编译但语义残缺」的写法。
// "[invalid" 是典型：拼成 `[`+charset+`]` 与 `[^`+charset+`]` 都是合法正则（嵌套
// 字符类），却会把所有字母数字判为非法字符——只在装配面校验才拦得住。
func TestValidCaptchaCharset(t *testing.T) {
	cases := []struct {
		name    string
		charset string
		wantErr bool
	}{
		{"空串合法（不过滤）", "", false},
		{"字母数字", "A-Za-z0-9", false},
		{"纯数字", "0-9", false},
		{"含符号", "0-9!@#", false},
		{"中日韩", "0-9\\p{Han}", false},
		{"字母区间（合法）", "a-z", false},
		// 嵌套字符类在Go 正则里合法且能匹配字母——**通用启发式无法识别语义残缺**
		// （实测见validCaptchaCharset 注释）。故 validCaptchaCharset 只挡编译错误。
		{"嵌套字符类（合法，语义由作者负责）", "[invalid", false},
		// 实证：Go 正则里"["、"a-"、"a{2,"、"(a" 都能编译（嵌套/字面量语义），
		// 真正编译不过的只有反斜杠类残缺——测试只锁这一类真实可达的硬错误。
		{"真编译不过（尾部反斜杠）", "a\\", true},
		{"真编译不过（未闭合属性名）", "\\p{", true},
}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validCaptchaCharset(c.charset)
			if c.wantErr && err == nil {
				t.Fatalf("字符集 %q 应被判非法", c.charset)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("字符集 %q 应判合法: %v", c.charset, err)
			}
		})
	}
}

// TestValidateRejectsBadCaptchaCharset 残缺字符集必须在档案自检（装配面）被拒，
// 而不是拖到登录时静默丢字符。
func TestValidateRejectsBadCaptchaCharset(t *testing.T) {
	d := testDescriptor("http://x.test")
	d.Captcha.Charset = "\\p{" // 编译不过的残缺写法
	err := d.Validate()
	if err == nil {
		t.Fatal("编译不过的字符集必须在 Validate 被拒")
	}
	if !strings.Contains(err.Error(), "字符集") {
		t.Fatalf("报错应点名字符集，实际: %v", err)
	}
}
