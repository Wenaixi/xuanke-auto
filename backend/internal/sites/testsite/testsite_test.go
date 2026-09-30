package testsite

import (
	"testing"

	"xuanke-auto/backend/internal/upstream"
)

// TestDescriptorValidates 档案必须自检通过——注册表的负向测试已证明自检有拦截力，
// 正向的"内置档案能过"在这里只是一道防回归的薄断言。
func TestDescriptorValidates(t *testing.T) {
	if err := Descriptor().Validate(); err != nil {
		t.Fatalf("testsite 档案必须自检通过: %v", err)
	}
}

// TestDescriptorShapes testsite 刻意在每个维度都与知到不同——这些断言是「解耦」的
// 声明本身：若哪天有人往引擎里加了知到假设，本档案的形状会让它红。
func TestDescriptorShapes(t *testing.T) {
	d := Descriptor()
	if d.HasWindowSignal {
		t.Fatal("testsite 不下发开窗信号，HasWindowSignal 必须为假（走退化模式）")
	}
	if !d.Captcha.Enabled || d.Captcha.MinLen != 4 || d.Captcha.MaxLen != 4 {
		t.Fatalf("验证码应为 4 位规格（知到是 3~5 位），实际 %+v", d.Captcha)
	}
	if d.Captcha.Charset != "0-9" {
		t.Fatalf("验证码应为纯数字（知到是字母数字），实际 %q", d.Captcha.Charset)
	}
	if d.CaptchaCacheBustParam != "" {
		t.Fatalf("testsite 不拼防缓存参数（知到拼 ?v=），实际 %q", d.CaptchaCacheBustParam)
	}
	if len(d.SessionCookies) != 0 {
		t.Fatalf("testsite 无会话 Cookie 需求（知到需 access_limit_cookie），实际 %+v", d.SessionCookies)
	}
	if d.Form.Captcha == "" {
		t.Fatal("testsite 登录表单**有**验证码键（其验证码规格是 4 位纯数字）")
	}
	if d.Form.UniqueID != "" {
		t.Fatal("testsite 无设备指纹（知到有 uniqueId）")
	}
	if d.Form.PriorityID != "" {
		t.Fatal("testsite 无优先身份（知到有 priorityId）")
	}
}

// TestOpErrorClassifierTestsite 错误分类用**英文码**（知到用中文文案）——
// 证明分类判据确实随站点走，引擎与调度器只消费 OpErrorKind 枚举位。
func TestOpErrorClassifierTestsite(t *testing.T) {
	cl := Descriptor().OpErrorClassifier
	cases := []struct {
		msg  string
		want upstream.OpErrorKind
	}{
		{"RATE_LIMITED", upstream.OpErrorRateLimited},
		{"WINDOW_CLOSED", upstream.OpErrorWindowClosed},
		{"CLASS_FULL", upstream.OpErrorClassFull},
		{"COURSE_NOT_FOUND", upstream.OpErrorUnknown},
		{"随便什么别的文案", upstream.OpErrorUnknown},
		{"", upstream.OpErrorUnknown},
		// 反向防线：知到的中文文案在 testsite 的分类器下**不得**命中任何分类
		// （若命中，说明分类器实现里写死了知到文案而非按本站点约定）。
		{"操作过于频繁，请稍后重试", upstream.OpErrorUnknown},
		{"不在选修报名时间范围内，无法选课！", upstream.OpErrorUnknown},
	}
	for _, c := range cases {
		if got := cl(c.msg, 1); got != c.want {
			t.Fatalf("classify(%q)=%v, want %v", c.msg, got, c.want)
		}
	}
}

// TestDecodeTermsVirtualSingleTerm 站点无学期概念：必须造出唯一且 selected 的
// 虚拟学期，供引擎的学期兜底重试路径使用（引擎数据结构零改动）。
func TestDecodeTermsVirtualSingleTerm(t *testing.T) {
	for _, body := range []string{`{"code":0}`, `{"code":0,"terms":[]}`, `{}`} {
		terms, err := decodeTerms([]byte(body))
		if err != nil {
			t.Fatalf("body=%s: %v", body, err)
		}
		if len(terms) != 1 || !terms[0].Selected {
			t.Fatalf("body=%s: 应造出唯一且 selected 的虚拟学期，实际 %+v", body, terms)
		}
	}
}

// TestDecodeOpResultStatusOK 成败判据 = status=="OK" 单一字符串
// （知到是 code==0 && isOk 双布尔，且站点状态是整数而非字符串）。
func TestDecodeOpResultStatusOK(t *testing.T) {
	if _, ok, err := decodeOpResult([]byte(`{"status":"OK"}`)); err != nil || !ok {
		t.Fatalf("status=OK 应判成功，ok=%v err=%v", ok, err)
	}
	for _, bad := range []string{
		`{"status":"FULL"}`,
		`{"status":"WINDOW_CLOSED"}`,
		`{"status":""}`, // 无状态键 = 未确认成功，绝不放过
	} {
		if _, ok, err := decodeOpResult([]byte(bad)); err != nil || ok {
			t.Fatalf("%s 应判失败，ok=%v err=%v", bad, ok, err)
		}
	}
}

// TestDecodeElectivesNoWindowSignal 站点无「发布」与「开窗信号」两个概念：
// 必须造出唯一虚拟发布，且 Selectable 恒为 nil（引擎据此走退化模式）。
func TestDecodeElectivesNoWindowSignal(t *testing.T) {
	body := []byte(`{"code":0,"data":{"classes":[
		{"id":9001,"group":"体育","name":"篮球","teacher":"李老师","room":"体育馆",
		 "capacity":30,"enrolled":5,"action":"ENROLL","enabled":true},
		{"id":9002,"group":"体育","name":"足球","teacher":"王老师","room":"操场",
		 "capacity":30,"enrolled":30,"action":"ENROLL","enabled":true},
		{"id":9003,"group":"艺术","name":"书法","teacher":"赵老师","room":"教室",
		 "capacity":20,"enrolled":3,"action":"WITHDRAW","enabled":true}
	]}}`)
	data, err := decodeElectives([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 1 {
		t.Fatalf("应造出唯一虚拟发布，实际 %d 个", len(data.Publishes))
	}
	pub := data.Publishes[0]
	if pub.Selectable != nil {
		t.Fatalf("testsite 不下发开窗信号，Selectable 必须为 nil，实际 %v", *pub.Selectable)
	}
	if len(pub.Classes) != 3 {
		t.Fatalf("应有 3 门课，实际 %d", len(pub.Classes))
	}
	if pub.TotalCount != 3 {
		t.Fatalf("TotalCount 应为 3，实际 %d", pub.TotalCount)
	}
	// 字符串按钮码映射为中立语义
	if pub.Classes[0].Action != "enroll" {
		t.Fatalf("ENROLL 应映射 enroll，实际 %q", pub.Classes[0].Action)
	}
	if pub.Classes[2].Action != "withdraw" {
		t.Fatalf("WITHDRAW 应映射 withdraw，实际 %q", pub.Classes[2].Action)
	}
	// 满员派生必须由适配器算好（引擎不猜）
	if !pub.Classes[1].ClassFull {
		t.Fatal("30/30 必须派生 ClassFull=true")
	}
	if pub.Classes[0].ClassFull {
		t.Fatal("5/30 必须派生 ClassFull=false")
	}
	// 未知按钮码映射为 none（不渲染操作按钮）
	if pub.Classes[0].ID != 9001 {
		t.Fatalf("课程 ID 应原样保留，实际 %d", pub.Classes[0].ID)
	}
}

// TestDecodeElectivesEmpty 空课程列表返回空快照（窗口关闭形态），不是错误。
func TestDecodeElectivesEmpty(t *testing.T) {
	data, err := decodeElectives([]byte(`{"code":0,"data":{"classes":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Publishes) != 0 {
		t.Fatalf("空课程应返回空快照，实际 %d 个发布", len(data.Publishes))
	}
}

// TestDecodeCounts 实时人数：字段名与形态也随站点（知到是 countList/selectedCount）。
func TestDecodeCounts(t *testing.T) {
	counts, err := decodeCounts([]byte(`{"code":0,"data":[
		{"id":9001,"enrolled":7,"capacity":30},
		{"id":9002,"enrolled":30,"capacity":30}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 {
		t.Fatalf("应解出 2 条，实际 %d", len(counts))
	}
	if counts[0].ID != 9001 || counts[0].SelectedCount != 7 {
		t.Fatalf("第 1 条映射错误: %+v", counts[0])
	}
	if counts[1].MaxCount != 30 {
		t.Fatalf("第 2 条 MaxCount 应为 30，实际 %d", counts[1].MaxCount)
	}
}

// TestDecodeLogin 登录响应取 token；状态非 OK 或 token 空一律判被拒。
func TestDecodeLogin(t *testing.T) {
	tok, err := decodeLogin([]byte(`{"status":"OK","token":"ts-token-1"}`))
	if err != nil || tok != "ts-token-1" {
		t.Fatalf("成功响应应返回 token，got %q %v", tok, err)
	}
	for _, bad := range []string{
		`{"status":"REJECTED","token":"x"}`,
		`{"status":"OK","token":""}`,
		`not-json`,
	} {
		if _, err := decodeLogin([]byte(bad)); err == nil {
			t.Fatalf("%s 应判登录被拒", bad)
		}
	}
}

// TestDecodeMsg 提取可读文案（英文码），解析失败返回空串而非 panic。
func TestDecodeMsg(t *testing.T) {
	if got := decodeMsg([]byte(`{"status":"RATE_LIMITED"}`)); got != "RATE_LIMITED" {
		t.Fatalf("应提取 status，实际 %q", got)
	}
	for _, bad := range []string{``, `not-json`, `{}`} {
		if got := decodeMsg([]byte(bad)); got != "" {
			t.Fatalf("%q 应返回空串，实际 %q", bad, got)
		}
	}
}
