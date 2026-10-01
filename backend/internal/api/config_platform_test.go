package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xuanke-auto/backend/internal/runtime"
	"xuanke-auto/backend/internal/sites"
)

// platformDeps 构造只关心"选课平台"配置的 Deps（真实 store/runtime + 可注入 RebindPlatform）。
func platformDeps(t *testing.T, rebind func(platformID, baseURL string) error) (*Deps, *testDeps) {
	t.Helper()
	td := newTestDeps(t)
	td.rt.Update(func(c *runtime.Config) {
		c.PlatformID = sites.DefaultID
		c.PlatformBaseURL = ""
	})
	return &Deps{
		Runtime: td.rt, Store: td.store, Encrypt: td.enc,
		RebindPlatform: rebind,
	}, td
}

// TestAdminConfigPlatformSwitchRebind 切平台走「先真实生效、后落库」：stub 收到档案 ID 与
// 覆盖地址，成功后运行时配置与 settings 同步更新（重启后仍是同一档案）。
func TestAdminConfigPlatformSwitchRebind(t *testing.T) {
	var calls int
	var gotID, gotBase string
	d, td := platformDeps(t, func(id, base string) error {
		calls++
		gotID, gotBase = id, base
		return nil
	})

	body := "{\"platform_id\":\"" + sites.DefaultID + "\",\"platform_base_url\":\"https://mirror.example.com\"}"
	code, resp := putConfig(t, d, body)
	if code != http.StatusOK || resp["code"].(float64) != 0 {
		t.Fatalf("合法平台配置应成功: code=%d resp=%v", code, resp)
	}
	if calls != 1 || gotID != sites.DefaultID || gotBase != "https://mirror.example.com" {
		t.Fatalf("必须以新档案与地址切换一次，实际 calls=%d id=%q base=%q", calls, gotID, gotBase)
	}
	cfg := td.rt.Get()
	if cfg.PlatformID != sites.DefaultID || cfg.PlatformBaseURL != "https://mirror.example.com" {
		t.Fatalf("运行时配置未更新: %+v", cfg)
	}
	kv, err := td.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if kv["platform_id"] != sites.DefaultID || kv["platform_base_url"] != "https://mirror.example.com" {
		t.Fatalf("平台配置必须落库（否则重启回退）: %v", kv)
	}
}

// TestAdminConfigPlatformRejectsEmptyID 空档案一律拒绝，且不触发切换、不改生效值、不落库。
func TestAdminConfigPlatformRejectsEmptyID(t *testing.T) {
	calls := 0
	d, td := platformDeps(t, func(id, base string) error { calls++; return nil })

	if _, resp := putConfig(t, d, "{\"platform_id\":\"\"}"); resp["code"].(float64) == 0 {
		t.Fatal("空档案必须被拒")
	}
	if calls != 0 {
		t.Fatalf("空档案不得触发切换，实际 %d 次", calls)
	}
	if cfg := td.rt.Get(); cfg.PlatformID != sites.DefaultID {
		t.Fatalf("被拒时不得改动生效配置: %+v", cfg)
	}
	if kv, _ := td.store.LoadSettings(); kv["platform_id"] != "" {
		t.Fatalf("被拒时不得落库任何平台值（空值会清掉重启恢复的档案）: %v", kv)
	}
}

// TestAdminConfigPlatformRebindFailureKeepsOld 切换失败（未知档案/非法地址/读凭据失败）
// 一律 HTTP 400 + 生效值与库均不变，绝不出现"档案换了、客户端没换"的半生效状态。
func TestAdminConfigPlatformRebindFailureKeepsOld(t *testing.T) {
	d, td := platformDeps(t, func(id, base string) error {
		return errors.New("未知选课平台")
	})
	code, resp := putConfig(t, d, "{\"platform_id\":\"nope\"}")
	if code != http.StatusBadRequest || resp["code"].(float64) == 0 {
		t.Fatalf("切换失败必须写真实 400：code=%d resp=%v", code, resp)
	}
	if cfg := td.rt.Get(); cfg.PlatformID != sites.DefaultID {
		t.Fatalf("切换失败不得改动生效配置: %+v", cfg)
	}
	if kv, _ := td.store.LoadSettings(); kv["platform_id"] == "nope" {
		t.Fatal("切换失败不得落库")
	}
}

// TestAdminConfigPlatformRejectedWithoutRebind 未注入 RebindPlatform（测试直构等形态）
// 时明确拒绝，绝不"只改配置不换客户端"造成重启前后不一致。
func TestAdminConfigPlatformRejectedWithoutRebind(t *testing.T) {
	d, td := platformDeps(t, nil)
	if _, resp := putConfig(t, d, "{\"platform_id\":\""+sites.DefaultID+"\"}"); resp["code"].(float64) == 0 {
		t.Fatal("无 RebindPlatform 时必须明确拒绝")
	}
	if cfg := td.rt.Get(); cfg.PlatformID != sites.DefaultID {
		t.Fatalf("被拒时不得改动配置: %+v", cfg)
	}
}

// TestAdminConfigViewPlatformFields GET 回显必须带齐"选哪份档案 + 档案元数据 + 全部可选档案"：
// 前端下拉零硬编码，靠这一份列表渲染（新增档案零前端改动）。
func TestAdminConfigViewPlatformFields(t *testing.T) {
	d := &Deps{}
	v := d.configView(runtime.Config{PlatformID: sites.DefaultID, PlatformBaseURL: "https://mirror.example.com"})
	if v.PlatformID != sites.DefaultID {
		t.Fatalf("platform_id 回显错误: %q", v.PlatformID)
	}
	if v.PlatformName == "" || v.PlatformDefaultBaseURL == "" {
		t.Fatalf("档案元数据不得空展示: %+v", v)
	}
	if v.PlatformBaseURL != "https://mirror.example.com" {
		t.Fatalf("站点地址覆盖回显错误: %q", v.PlatformBaseURL)
	}
	if len(v.Platforms) == 0 {
		t.Fatal("必须下发可选档案列表")
	}
	// 未知 ID 也不空展示：名称回退为 ID 本身（绝不显示空白让管理员猜）。
	bad := d.configView(runtime.Config{PlatformID: "no-such"})
	if bad.PlatformName != "no-such" {
		t.Fatalf("未知档案名称应回退为 ID，实际 %q", bad.PlatformName)
	}
}

// TestAdminStatsIncludesPlatformName 运行状态页必须能显示"跑的是哪一套接口"：
// 缺该字段时管理员切过平台也无从确认（多档案并存时的唯一核对点）。
func TestAdminStatsIncludesPlatformName(t *testing.T) {
	td := newTestDeps(t)
	d := &Deps{Store: td.store, Sched: td.sched, Runtime: td.rt, Accounts: td.accts}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	w := httptest.NewRecorder()
	d.handleAdminStats(w, req)
	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON: %v (%s)", err, w.Body.String())
	}
	if resp.Code != 0 {
		t.Fatalf("stats 应成功: %+v", resp)
	}
	if got, _ := resp.Data["platform_id"].(string); got != sites.DefaultID {
		t.Fatalf("stats.platform_id 应为当前档案，实际 %q", got)
	}
	if got, _ := resp.Data["platform_name"].(string); got == "" {
		t.Fatal("stats.platform_name 不得为空")
	}
}

// TestAdminConfigPlatformRebindSuccessThenSaveFails 落库失败但切换已成功的半生效形态：
// 必须 HTTP 500 + body code 500 + 生效配置已换（rebind 先行）+ 库保持旧值（重启回退），
// 且明确文案提示"重启后将回退"（契约 17：落库失败绝不静默吞错，管理员要能看到风险）。
func TestAdminConfigPlatformRebindSuccessThenSaveFails(t *testing.T) {
	d, td := platformDeps(t, func(id, base string) error {
		return nil // 切换本身成功
	})
	saveSettingsErrForTest = errors.New("settings 落库失败")
	t.Cleanup(func() { saveSettingsErrForTest = nil })

	body := `{"platform_id":"` + sites.DefaultID + `","platform_base_url":"https://mirror.example.com"}`
	code, resp := putConfig(t, d, body)
	if code != http.StatusInternalServerError {
		t.Fatalf("落库失败必须如实 500，实际 code=%d resp=%v", code, resp)
	}
	if resp["code"].(float64) != 500 {
		t.Fatalf("body code 应为 500: %v", resp)
	}
	if msg, _ := resp["msg"].(string); !strings.Contains(msg, "落库失败") || !strings.Contains(msg, "重启后将回退") {
		t.Fatalf("必须明确告知落库失败与重启回退风险: %q", msg)
	}
	// 生效配置已换（先真实生效、后落库——即使落库失败也不回滚生效值）
	cfg := td.rt.Get()
	if cfg.PlatformID != sites.DefaultID || cfg.PlatformBaseURL != "https://mirror.example.com" {
		t.Fatalf("切换成功后生效配置应已更新: %+v", cfg)
	}
	// 库保持旧值（重启回退旧档案），且绝不能把新值写入 settings
	kv, err := td.store.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if kv["platform_base_url"] == "https://mirror.example.com" {
		t.Fatalf("落库失败不得把新值写进 settings: %v", kv)
	}
}
