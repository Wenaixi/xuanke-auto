package sites

import (
	"strings"
	"testing"

	"xuanke-auto/backend/internal/config"
	"xuanke-auto/backend/internal/upstream"
)

// TestDefaultIDMatchesConfig 默认档案 ID 单源核对：config 为避免 import cycle
// （upstream 的 CGO 文件反依赖 config，而注册表经适配器依赖 upstream）用了字面量，
// 一致性只能靠本测试锁——改一处漏一处必红。
func TestDefaultIDMatchesConfig(t *testing.T) {
	if DefaultID != config.DefaultPlatformID {
		t.Fatalf("默认平台 ID 分叉：sites.DefaultID=%q，config.DefaultPlatformID=%q",
			DefaultID, config.DefaultPlatformID)
	}
	if _, err := Resolve(DefaultID); err != nil {
		t.Fatalf("默认档案不可解析: %v", err)
	}
}

// TestBuiltinsValidateAndUnique 每条内置档案必须过自检（路径/键名/钩子齐全）且 ID 自洽：
// 缺路径的档案会让运行时在第一次请求时才炸，现场只是"未知的解析错误"。
func TestBuiltinsValidateAndUnique(t *testing.T) {
	ids := IDs()
	if len(ids) == 0 {
		t.Fatal("注册表为空")
	}
	for _, id := range ids {
		d, err := Resolve(id)
		if err != nil {
			t.Fatalf("Resolve(%q) 失败: %v", id, err)
		}
		if d.ID != id {
			t.Fatalf("档案键与 ID 不一致：键 %q，ID %q", id, d.ID)
		}
		if err := d.Validate(); err != nil {
			t.Fatalf("档案 %s 未通过自检: %v", id, err)
		}
	}
	if len(List()) != len(ids) {
		t.Fatalf("List 与 IDs 数量不一致：%d vs %d", len(List()), len(ids))
	}
}

// TestValidateRejectsIncompleteDescriptor 负向守护：一份合法内置档案的**副本**，
// 逐个破坏其关键字段后，Validate 必须报错并点名该字段。
//
// 意义：既有两条 Validate 测试都是正向的（内置档案能通过），属恒真型断言——把
// pathFields 或解码钩子的判据改坏成"永远通过"，它们照样绿。缺解码钩子的档案在
// 运行时是 nil 函数调用（硬崩），缺路径则拼出无前导斜杠的 URL 让平台 404，
// 两者都只在第一次请求时炸，故自检必须有真实的拦截力。
func TestValidateRejectsIncompleteDescriptor(t *testing.T) {
	base, err := Resolve(DefaultID)
	if err != nil {
		t.Fatalf("默认档案不可解析: %v", err)
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("前置：默认档案应通过自检: %v", err)
	}
	cases := []struct {
		name    string
		break_  func(*upstream.SiteDescriptor)
		wantSub string
	}{
		{"缺报名路径", func(d *upstream.SiteDescriptor) { d.SelectPath = "" }, "SelectPath"},
		{"路径无前导斜杠", func(d *upstream.SiteDescriptor) { d.SelectPath = "electives/select" }, "SelectPath"},
		{"缺鉴权载体名", func(d *upstream.SiteDescriptor) { d.TokenCookie = "" }, "TokenCookie"},
		{"缺登录加密钩子", func(d *upstream.SiteDescriptor) { d.Login.EncryptIdentification = nil }, "登录钩子"},
		{"缺设备指纹钩子", func(d *upstream.SiteDescriptor) { d.Login.DeviceID = nil }, "登录钩子"},
		{"缺报名结果解码", func(d *upstream.SiteDescriptor) { d.Decode.OpResult = nil }, "解码钩子"},
		{"缺课程数据解码", func(d *upstream.SiteDescriptor) { d.Decode.Electives = nil }, "解码钩子"},
		{"缺表单键名", func(d *upstream.SiteDescriptor) { d.Form.ClassID = "" }, "表单字段名"},
		{"缺默认站点地址", func(d *upstream.SiteDescriptor) { d.DefaultBaseURL = "" }, "默认站点地址"},
		{"缺名称", func(d *upstream.SiteDescriptor) { d.Name = "" }, "缺少名称"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := base // 值拷贝：破坏副本不影响注册表里的原档案
			tc.break_(&broken)
			err := broken.Validate()
			if err == nil {
				t.Fatalf("%s 时 Validate 必须报错（自检形同虚设）", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("错误文案应点名 %q，实际: %v", tc.wantSub, err)
			}
		})
	}
}

// TestResolveValidatesBuiltins 注册表解析出的档案必须已过自检——Resolve 是装配面
// 的唯一入口（启动解析与后台热切换都走它），故自检钉在这里最贴近调用面。
func TestResolveValidatesBuiltins(t *testing.T) {
	for _, id := range IDs() {
		d, err := Resolve(id)
		if err != nil {
			t.Fatalf("Resolve(%q) 失败: %v", id, err)
		}
		if err := d.Validate(); err != nil {
			t.Fatalf("档案 %s 未通过自检: %v", id, err)
		}
	}
}

// TestHistoricalIDsRetained 契约：**内置档案只增不删**——settings 表里持久化的
// platform_id 必须始终可解析，否则升级/降级后会静默回退到别的档案（把请求打到另一个
// 站点），而这是最难发现的线上故障。退役档案只能标记，不能从表里删。
func TestHistoricalIDsRetained(t *testing.T) {
	for _, id := range []string{"zhidao-2026"} {
		if _, err := Resolve(id); err != nil {
			t.Fatalf("历史档案 %s 必须保留（存量部署的 settings.platform_id 依赖它）: %v", id, err)
		}
	}
}

// TestResolveUnknownListsOptions 未知识别必须给出可执行的提示（列出可选档案）。
func TestResolveUnknownListsOptions(t *testing.T) {
	_, err := Resolve("no-such-platform")
	if err == nil {
		t.Fatal("未知档案必须报错，绝不静默回退默认档案")
	}
	if !strings.Contains(err.Error(), DefaultID) {
		t.Fatalf("错误提示应列出可选档案，实际: %v", err)
	}
}

// TestListSortedStable 前端下拉按稳定顺序渲染（两次调用顺序一致）。
func TestListSortedStable(t *testing.T) {
	a, b := List(), List()
	if len(a) != len(b) {
		t.Fatalf("List 长度不稳定: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("List 顺序不稳定: %v vs %v", a, b)
		}
		if a[i].Name == "" || a[i].DefaultBaseURL == "" {
			t.Fatalf("档案 %s 元数据不完整: %+v", a[i].ID, a[i])
		}
	}
}


// ResolveValidated 装配单点：Resolve + Validate 合并（启动/工具统一走它）。
func TestResolveValidated(t *testing.T) {
	// 未知 ID 必须报错
	if _, err := ResolveValidated("no-such"); err == nil {
		t.Fatal("未知档案必须报错")
	}
	// 全部内置档案通过
	for _, id := range IDs() {
		d, err := ResolveValidated(id)
		if err != nil {
			t.Fatalf("ResolveValidated(%q) 失败: %v", id, err)
		}
		if d.ID != id {
			t.Fatalf("ID 不一致: %q vs %q", d.ID, id)
		}
	}
}

// EffectiveBaseURL 生效地址单源：覆盖优先，非法覆盖回退默认。
func TestEffectiveBaseURL(t *testing.T) {
	desc, err := ResolveValidated(DefaultID)
	if err != nil {
		t.Fatal(err)
	}
	def := upstream.NormalizeBaseURL(desc.DefaultBaseURL)
	// 空覆盖 → 默认
	if got := EffectiveBaseURL(desc, ""); got != def {
		t.Fatalf("空覆盖应回退默认地址，got %q want %q", got, def)
	}
	// 合法覆盖 → 覆盖
	if got := EffectiveBaseURL(desc, "https://mirror.example.com/"); got != "https://mirror.example.com" {
		t.Fatalf("合法覆盖应生效，got %q", got)
	}
	// 非法覆盖（手改 DB 的垃圾值）→ 回退默认，绝不裸拼接
	if got := EffectiveBaseURL(desc, "not a url"); got != def {
		t.Fatalf("非法覆盖应回退默认地址，got %q want %q", got, def)
	}
}
