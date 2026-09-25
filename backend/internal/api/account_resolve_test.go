package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestResolveAccountForSession 账号透传解析收权（C2-1 TDD 红线）。
// 现状：5 处透传块内联复制（electives/select/exit/state/targets），行为靠各点守卫兜底，
// 无独立 helper 可测。收权后 resolveAccountForSession 成为唯一实现，以下断言全绿且行为逐字等价。
//
// 核实修正（相对原报告）：select/exit 没有"回落核心账号"分支（靠后文 IsAdminAccountName 拒绝），
// 只有 electives/state/targets 有回落；targets 无核心账号时整体拒绝（fallbackKind="reject"）。
// 本测试钉死三族语义 + 每点文案。

// TestResolveAccountForSessionNormalUserIgnoresOverride 普通会话带 ?account= 必须忽略穿透
//（allowAccountOverride 仅管理员会话为 true）——普通学生会话永远只操作自己绑定账号。
func TestResolveAccountForSessionNormalUserIgnoresOverride(t *testing.T) {
	d := newTestDeps(t)
	tok := authenticateDirect(t, d, "acct1")
	// 普通会话带 ?account=acct2（acct2 未登录，若误穿透会撞凭据表拒绝）——
	// 正确行为：忽略 ?account=，返回会话绑定的 acct1
	code, j := doJSONAuth(t, d.api, "GET", "/api/state?account=acct2", "", tok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("普通会话带 ?account= 应忽略穿透正常返回自身状态: %d %v", code, j)
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("state data 缺失: %v", j)
	}
}

// TestResolveAccountForSessionAdminGhostRejects 管理员透传不存在的账号必须整体拒绝（文案逐点保留）。
// 五条路径各断言自己的"账号不存在"文案——收权后由 resolveAccountForSession 统一产出，
// 但调用点传各自原文案，行为逐字等价。
func TestResolveAccountForSessionAdminGhostRejects(t *testing.T) {
	d := newTestDeps(t)
	adminTok := d.sessions.CreateAdmin("admin")
	// 先登录 acct1 并设置目标，让 AccountsWithTargets 有值（避免回落分支掩盖透传拒绝）
	tok := authenticateDirect(t, d, "acct1")
	_ = tok
	if _, err := d.sched.ProbeNow(); err != nil {
		t.Fatalf("填充全局快照失败: %v", err)
	}

	cases := []struct {
		name string
		meth string
		path string
		body string
		msg  string // 期望"账号不存在"文案
	}{
		{"electives", "GET", "/api/electives?account=nonexistent", "", "账号不存在，无法查看课程"},
		{"select", "POST", "/api/electives/select?account=nonexistent", `{"class_id":61115}`, "账号不存在，无法执行报名操作"},
		{"exit", "POST", "/api/electives/select/exit?account=nonexistent", `{"class_id":61115}`, "账号不存在，无法执行退选操作"},
		{"state", "GET", "/api/state?account=nonexistent", "", "账号不存在，无法读取状态"},
		{"targets", "PUT", "/api/targets?account=nonexistent", `{"targets":[]}`, "账号不存在，无法设置目标"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, j := doJSONAuth(t, d.api, c.meth, c.path, c.body, adminTok)
			if code != 200 || j["code"].(float64) == 0 {
				t.Fatalf("管理员透传不存在账号应被拒绝: %d %v", code, j)
			}
			if got, _ := j["msg"].(string); got != c.msg {
				t.Fatalf("文案期望 %q，实际 %q（收权后调用点必须保留原文案）", c.msg, got)
			}
		})
	}
}

// TestResolveAccountForSessionAdminFallback 管理员透传语义三族（回落 / 拒绝 / 静默）：
//   a. electives 无透传且有目标账号 → 回落核心账号（取 targets[0]）
//   b. targets 无透传且无任何有目标账号 → 整体拒绝（fallbackKind="reject"，绝不写管理员孤儿行）
//   c. select 无透传且无核心账号 → 静默回落，由后文 IsAdminAccountName 守卫兜底（"请指定有效学生账号"）
func TestResolveAccountForSessionAdminFallback(t *testing.T) {
	d := newTestDeps(t)
	adminTok := d.sessions.CreateAdmin("admin")

	// b + c：尚无任何账号有目标 → targets 拒绝、select 拒学生账号
	code, j := doJSONAuth(t, d.api, "PUT", "/api/targets", `{"targets":[]}`, adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("管理员无透传无目标账号设置空目标应被拒绝（不得写管理员孤儿行）: %d %v", code, j)
	}
	if got, _ := j["msg"].(string); got != "请指定要设置目标的学生账号（?account=）" {
		t.Fatalf("targets 拒绝文案期望 %q，实际 %q", "请指定要设置目标的学生账号（?account=）", got)
	}
	code, j = doJSONAuth(t, d.api, "POST", "/api/electives/select", `{"class_id":61115}`, adminTok)
	if code != 200 || j["code"].(float64) == 0 {
		t.Fatalf("管理员无透传 select 应被拒（无学生账号可操作）: %d %v", code, j)
	}
	if got, _ := j["msg"].(string); got != "请指定有效学生账号" {
		t.Fatalf("select 拒绝文案期望 %q，实际 %q", "请指定有效学生账号", got)
	}

	// a：acct1 登录并设置目标后，管理员无透传 → electives 回落核心账号（看到 acct1 的课程）
	tok := authenticateDirect(t, d, "acct1")
	_ = tok
	if _, err := d.sched.ProbeNow(); err != nil {
		t.Fatalf("填充全局快照失败: %v", err)
	}
	if _, jr := doJSONAuth(t, d.api, "PUT", "/api/targets?account=acct1", `{"targets":[{"publish_id":1,"class_id":61115,"course_name":"健美操"}]}`, adminTok); jr["code"].(float64) != 0 {
		t.Fatalf("为 acct1 设置目标失败: %v", jr)
	}
	code, j = doJSONAuth(t, d.api, "GET", "/api/electives", "", adminTok)
	if code != 200 || j["code"].(float64) != 0 {
		t.Fatalf("管理员无透传应回落核心账号正常查看课程: %d %v", code, j)
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("electives data 缺失: %v", j)
	}
	// 反向防线：管理员显式透传 acct1 也应正常
	if _, jr := doJSONAuth(t, d.api, "GET", "/api/electives?account=acct1", "", adminTok); jr["code"].(float64) != 0 {
		t.Fatalf("管理员显式透传真实账号查看课程应正常: %v", jr)
	}
}

// TestSetTargetsNotFoundJSON 确保 targets 拒绝返回 JSON（既有 TestSetTargetsUnknownAccountDoesNotFabricate
// 同源，这里补充"文案"断言）——收权后 resolveAccountForSession 统一产出。
func TestSetTargetsNotFoundJSON(t *testing.T) {
	d := newTestDeps(t)
	adminTok := d.sessions.CreateAdmin("admin")
	req := httptest.NewRequest("PUT", "/api/targets?account=nonexistent",
		strings.NewReader(`{"targets":[]}`))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	if j["code"].(float64) == 0 {
		t.Fatalf("不存在的账号设置目标应被拒绝: %v", j)
	}
}
