package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 业务码契约测试：前端按 code 分流而非匹配中文文案（文案一改就静默失效，
// 且"重试永远不成功"的失败态会被管理员当成故障反复点）。
//
// 覆盖三处产出：激活码机制关闭（激活入口与管理入口共用一个常量）、
// 激活票据无效。既有测试不断言这些码，故在此逐条钉死。

func postJSON(t *testing.T, d *testDeps, path, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	d.api.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s 期望 HTTP 200（业务码走 body），实际 %d", path, rec.Code)
	}
	var j map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &j); err != nil {
		t.Fatal(err)
	}
	return j
}

func bodyCode(t *testing.T, j map[string]any) int {
	t.Helper()
	code, ok := j["code"].(float64)
	if !ok {
		t.Fatalf("响应缺 code 字段: %+v", j)
	}
	return int(code)
}

// TestActivateDisabledReturnsSpecificCode 机制关闭时 /api/activate 必须回
// codeActivationDisabled 而非通用失败码 1——前端据此隐藏重试出口（重试恒失败）。
func TestActivateDisabledReturnsSpecificCode(t *testing.T) {
	d := newTestDepsMode(t, false) // 激活码机制关闭
	j := postJSON(t, d, "/api/activate", `{"account":"acct1","code":"XK-1","ticket":"t"}`)
	if got := bodyCode(t, j); got != codeActivationDisabled {
		t.Fatalf("机制关闭应回 code=%d，实际 %d", codeActivationDisabled, got)
	}
}

// TestAdminCodesDisabledReturnsSpecificCode 管理端激活码接口在机制关闭时同样
// 回专属码（GET/POST/DELETE 共用这一个前置分支）。
func TestAdminCodesDisabledReturnsSpecificCode(t *testing.T) {
	d := newTestDepsMode(t, false)
	adminTok := adminTokenFor(t, d)
	_, j := doJSONAdmin(t, d.api, "GET", "/api/admin/codes", "", adminTok)
	if got := bodyCode(t, j); got != codeActivationDisabled {
		t.Fatalf("机制关闭应回 code=%d，实际 %d", codeActivationDisabled, got)
	}
}

// TestActivateInvalidTicketReturnsSpecificCode 无效票据必须回 codeTicketInvalid，
// 前端据此引导「重新登录」而非「检查激活码」——两者下一步操作完全不同。
func TestActivateInvalidTicketReturnsSpecificCode(t *testing.T) {
	d := newTestDepsMode(t, true) // 机制开启
	j := postJSON(t, d, "/api/activate", `{"account":"acct1","code":"XK-1","ticket":"not-a-real-ticket"}`)
	if got := bodyCode(t, j); got != codeTicketInvalid {
		t.Fatalf("票据无效应回 code=%d，实际 %d", codeTicketInvalid, got)
	}
}

// TestBusinessCodesDistinctFromGenericFailure 专属业务码不得复用通用失败码 1：
// 前端据 code 决定「重试有没有意义」，与 1 混用即退回靠文案猜的老路。
func TestBusinessCodesDistinctFromGenericFailure(t *testing.T) {
	codes := map[string]int{
		"codeNotActivated":       codeNotActivated,
		"codeActivationDisabled": codeActivationDisabled,
		"codeTicketInvalid":      codeTicketInvalid,
		"codeSessionInvalid":     codeSessionInvalid,
	}
	seen := map[int]string{}
	for name, c := range codes {
		if c == 1 {
			t.Fatalf("%s 不得复用通用失败码 1", name)
		}
		if c == 0 {
			t.Fatalf("%s 不得占用成功码 0", name)
		}
		if prev, dup := seen[c]; dup {
			t.Fatalf("业务码 %d 被 %s 与 %s 复用", c, prev, name)
		}
		seen[c] = name
	}
}
