package zhidao

// sanitizeError 脱敏契约测试：网络层错误（*url.Error / *net.OpError）经
// http.Client.Do 返回时文本回放完整请求 URL——doRequest 把完整 idToken 拼进
// "?idToken=" URL 参数通道，连接层失败后该错误原文若进入日志/库表/前端回显，
// 会话凭证整体泄露。本测试钉住 sanitizeError 必须：
//   1. 输出不含完整的请求 URL（idToken 段整体剥除）
//   2. 保留判型所需语义（net.OpError 可穿透、dial/write/read/超时形态全部命中）
// 与 IsReadErr/isConnErrRetryable 复用同一 mock 形态（rst/dial/write/fin/shortread/timeout）。
import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
)

// TestSanitizeErrorStripsTokenFromDialError：dial 失败（黄金期 connectex 最常见形态）
// 的 *url.Error 文本含完整 URL，sanitizeError 输出必须剥掉 token。
func TestSanitizeErrorStripsTokenFromDialError(t *testing.T) {
	tok := "12345678901234567890"
	base := &url.Error{
		Op:  "Post",
		URL: "http://127.0.0.1/app/electives?idToken=" + tok, // 与真实 doRequest 一致：URL 的 idToken 参数即完整 token
		Err: fmt.Errorf("dial tcp 127.0.0.1:80: connectex: %s", "No connection could be made because the target machine actively refused it"),
	}
	out := sanitizeError(base).Error()
	if strings.Contains(out, tok) {
		t.Fatalf("sanitizeError 输出仍含完整 token %q：%q", tok, out)
	}
	if strings.Contains(out, "idToken=") {
		t.Fatalf("sanitizeError 输出仍含 idToken= 参数：%q", out)
	}
	if !strings.Contains(out, "dial tcp") {
		t.Fatalf("sanitizeError 丢失底层错误语义（应保留 dial tcp 可读描述）：%q", out)
	}
}

// TestSanitizeErrorPreservesJudgment：sanitizeError 输出保留判型所需语义——
// IsReadErr / isConnErrRetryable 判定不受剥 URL 影响。先对原始错误断言判定
// （防测试自身形态失效），再对脱敏后错误断言同判定。
func TestSanitizeErrorPreservesJudgment(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantIs  func(error) bool
		wantHit bool
	}{
		{"dial", &url.Error{Op: "Post", URL: "http://x?idToken=1", Err: &net.OpError{Op: "dial", Err: errors.New("connectex")}}, isConnErrRetryable, true},
		{"write", &url.Error{Op: "Post", URL: "http://x?idToken=1", Err: &net.OpError{Op: "write", Err: errors.New("write failed")}}, isConnErrRetryable, true},
		{"read-rst", &url.Error{Op: "Post", URL: "http://x?idToken=1", Err: &net.OpError{Op: "read", Err: errors.New("connection reset")}}, IsReadErr, true},
		{"fin", &url.Error{Op: "Post", URL: "http://x?idToken=1", Err: io.EOF}, IsReadErr, true},
		{"shortread", &url.Error{Op: "Post", URL: "http://x?idToken=1", Err: io.ErrUnexpectedEOF}, IsReadErr, true},
		{"timeout", errors.New("Post \"https://x\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"), IsReadErr, true}, // 纯文本形态（非 url.Error）应原样保留
		{"business", errors.New("验证码错误"), isConnErrRetryable, false},
		{"nil", nil, isConnErrRetryable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.wantIs(tc.err); got != tc.wantHit {
				t.Fatalf("原始判定 want=%v got=%v（基准形态失效？测试自身有误）", tc.wantHit, got)
			}
			san := sanitizeError(tc.err)
			if got := tc.wantIs(san); got != tc.wantHit {
				t.Fatalf("sanitizeError 后判定 want=%v got=%v（剥 URL 破坏判型）", tc.wantHit, got)
			}
			if !tc.wantHit {
				return // 不命中形态无需再查 URL
			}
		})
	}
}

// TestSanitizeErrorNilSafe：nil 原样返回，永远不 nil deref。
func TestSanitizeErrorNilSafe(t *testing.T) {
	if sanitizeError(nil) != nil {
		t.Fatalf("sanitizeError(nil) 应返回 nil")
	}
}

// TestSanitizeErrorOriginalErrorPreserved：非网络层错误（业务错误/纯文本错误）
// 原样透传，绝不改写（错误文案契约不被误伤）。
func TestSanitizeErrorOriginalErrorPreserved(t *testing.T) {
	e := errors.New("选课处理中，请勿重复操作") // 平台下发业务文案，不含敏感
	if out := sanitizeError(e); out != e {
		t.Fatalf("业务错误不应被改写：got %q want %q", out, e)
	}
}

// TestDoRequestSanitizesDialError：端到端——doRequest 向不可达端口发带完整
// idToken 的请求（真实 connectex），返回错误必须已剥 URL（不含 token 与
// idToken= 段），同时保留底层 dial 语义（IsReadErr 判定仍正确）。
func TestDoRequestSanitizesDialError(t *testing.T) {
	socketPreheat() // 排空冷启动窗口（与全包夹具同款）
	// 向必拒端口发 findElectivesData（URL 拼完整 token）——真实连接层失败
	tok := "12345678901234567890"
	c := New("http://127.0.0.1:1", VisionConfig{})
	c.SetCredentials("acct", "pwd", tok)
	body, err := c.doRequest(http.MethodPost, "/electives/select/findElectivesData", nil, "")
	if err == nil {
		t.Fatalf("向不可达端口发请求应返回错误，实际 nil（body=%s）", body)
	}
	msg := err.Error()
	if strings.Contains(msg, tok) {
		t.Fatalf("doRequest 连接层错误仍含完整 token %q：%q", tok, msg)
	}
	if strings.Contains(msg, "idToken=") {
		t.Fatalf("doRequest 连接层错误仍含 idToken= 参数：%q", msg)
	}
	if !strings.Contains(msg, "dial") || (strings.Contains(msg, "connectex") && runtime.GOOS != "windows") {
		t.Fatalf("doRequest 连接层错误丢失 dial/connectex 可读描述：%q", msg)
	}
	if IsReadErr(err) {
		t.Fatalf("dial 连接层错误不应命中 IsReadErr（判型被脱敏破坏）")
	}
	if !isConnErrRetryable(err) {
		t.Fatalf("dial 连接层错误应命中 isConnErrRetryable（判型被脱敏破坏）")
	}
}

// TestSanitizeErrorNeverEmptyError：剥 URL 后输出绝不空串（消费侧 .Error() 仍可读）。
func TestSanitizeErrorNeverEmptyError(t *testing.T) {
	base := &url.Error{Op: "Post", URL: "http://host/path?idToken=abcdef", Err: errors.New("dial tcp: refused")}
	out := sanitizeError(base)
	if out == nil {
		t.Fatalf("sanitizeError 不应返回 nil")
	}
	if out.Error() == "" {
		t.Fatalf("sanitizeError 输出为空串（消费侧读不到错误）")
	}
}