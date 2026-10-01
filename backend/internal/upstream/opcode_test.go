package upstream

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 本文件钉住「业务码从档案解码器贯通到错误分类链」这一契约。
//
// 回归动机：classOp 曾把 OpResult 解出的业务码丢弃，两处硬传 0——
// OpErrorClassifier(msg, 0) 与 SiteError{Code: 0}。分类器签名明明承诺了
// code 参数，引擎却永远给 0，使「按数字码分类错误的平台」接不了（只能被迫
// 退回文案匹配，而平台文案会随版本变化、且原文含「已满员」的非满员失败
// 会被误判）。两个内置分类器都只用 msg 参数，故缺陷此前不可见。
//
// 变异验证：把 classOp 里的 OpErrorClassifier(msg, code) 改回 (msg, 0)
// 后，本文件的第一条用例必须红（报 Code == 0）；这证明它不是恒真断言。

// TestOpErrorClassifierReceivesRealBusinessCode 是「贯通」的定义性断言：
// 分类器拿到的必须是响应里的真实业务码，据此能分出不同种类；SiteError.Code
// 同样携带该码。摘掉 classOp 的 code 实参会立刻红。
func TestOpErrorClassifierReceivesRealBusinessCode(t *testing.T) {
	socketPreheat()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"code": 42, "isOk": false, "msg": "业务被拒"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	var gotCode int
	c.desc.OpErrorClassifier = func(msg string, code int) OpErrorKind {
		gotCode = code
		if code == 42 {
			return OpErrorRateLimited
		}
		return OpErrorUnknown
	}
	c.SetCredentials("acct", "pwd", "tok")

	_, err := c.SelectClass(61115)
	if err == nil {
		t.Fatal("业务失败必须报错")
	}
	var se *SiteError
	if !errors.As(err, &se) {
		t.Fatalf("失败必须是 *SiteError（调度器靠它分流），实际 %T", err)
	}
	if gotCode != 42 {
		t.Errorf("分类器收到的业务码=%d，want 42（引擎曾恒传 0，分类器签名形同虚设）", gotCode)
	}
	if se.Code != 42 {
		t.Errorf("SiteError.Code=%d，want 42（业务码必须随错误上抛供排障）", se.Code)
	}
	if se.Kind != OpErrorRateLimited {
		t.Errorf("SiteError.Kind=%v，want 限流（分类器按数字码分出的种类必须生效）", se.Kind)
	}
}

// TestOpErrorClassifierCodeAndMessageAreIndependent 证明「码透传」与「按码分类」
// 是两件独立的事：分类器不认识的码归 OpErrorUnknown，但业务码仍须完整上抛。
// 若分类被静默丢弃、退化成恒 0，本用例的 Code 断言即红。
func TestOpErrorClassifierCodeAndMessageAreIndependent(t *testing.T) {
	socketPreheat()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"code": 7, "isOk": false, "msg": "不认识的状态"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.desc.OpErrorClassifier = func(msg string, code int) OpErrorKind {
		if code == 42 {
			return OpErrorRateLimited
		}
		return OpErrorUnknown
	}
	c.SetCredentials("acct", "pwd", "tok")

	_, err := c.ExitClass(61115)
	if err == nil {
		t.Fatal("业务失败必须报错")
	}
	var se *SiteError
	if !errors.As(err, &se) {
		t.Fatalf("失败必须是 *SiteError，实际 %T", err)
	}
	if se.Kind != OpErrorUnknown {
		t.Errorf("分类器不认识的码应归未分类，got %v", se.Kind)
	}
	if se.Code != 7 {
		t.Errorf("SiteError.Code=%d，want 7（未识别的码也必须原样透传，不得因分类失败而归零）", se.Code)
	}
}

// TestOpErrorClassifierNilFallsBackToUnknown 档案未提供分类器时归未分类且**不崩**
// （nil 函数调用是硬崩，纯属零值兼容的基本要求），业务码仍上抛。
func TestOpErrorClassifierNilFallsBackToUnknown(t *testing.T) {
	socketPreheat()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"code": 3, "isOk": false, "msg": "课程不存在"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, VisionConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	c.desc.OpErrorClassifier = nil
	c.SetCredentials("acct", "pwd", "tok")

	_, err := c.SelectClass(61115)
	var se *SiteError
	if !errors.As(err, &se) {
		t.Fatalf("失败必须是 *SiteError，实际 %v", err)
	}
	if se.Kind != OpErrorUnknown {
		t.Errorf("无分类器应归未分类，got %v", se.Kind)
	}
	if se.Code != 3 {
		t.Errorf("SiteError.Code=%d，want 3", se.Code)
	}
}