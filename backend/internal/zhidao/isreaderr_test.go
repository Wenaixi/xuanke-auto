package zhidao

// IsReadErr 形态矩阵契约测试：IsReadErr 必须覆盖"请求已发出、
// 平台可能已处理"的全部三种形态——RST（net.OpError read）/ FIN（io.EOF）/ 超时
// （Client.Timeout exceeded awaiting headers）。任一形态 miss 会让 scheduler/api
// 对"平台可能已抢到课"的错误显示"报名失败"误导文案（黄金期重复报名被拒时 failed 残留）。
import (
	"errors"
	"io"
	"net"
	"testing"
)

func TestIsReadErrCoversAllForms(t *testing.T) {
	// RST 形态：*net.OpError.Op=="read"（真实 http.Client 返回经 url.Error 包装）
	rst := &net.OpError{Op: "read", Err: errors.New("wsarecv: connection reset")}
	if !IsReadErr(rst) {
		t.Fatalf("RST 形态应命中 IsReadErr，实际 false")
	}
	// url.Error 包装链穿透（http.Client.Do 的真实返回形态）
	wrapped := &net.OpError{Op: "read", Err: errors.New("connection reset")}
	if !IsReadErr(&urlError{err: wrapped}) {
		t.Fatalf("url.Error 包装的 RST 应命中（errors.As 穿透），实际 false")
	}
	// FIN 形态：纯 io.EOF（服务端读完 body 后正常 Close，真实平台"已处理未响应"典型形态）
	if !IsReadErr(io.EOF) {
		t.Fatalf("FIN(io.EOF) 形态应命中 IsReadErr，实际 false")
	}
	if !IsReadErr(&urlError{err: io.EOF}) {
		t.Fatalf("url.Error 包装的 FIN 应命中（errors.Is 穿透），实际 false")
	}
	// 短读形态：正文 Content-Length 未传完就断连（标准库 body.readLocked 对
	// LimitedReader 短读包装为 ErrUnexpectedEOF；Do 层再包 url.Error，errors.Is 穿透）
	if !IsReadErr(io.ErrUnexpectedEOF) {
		t.Fatalf("短读(ErrUnexpectedEOF) 形态应命中 IsReadErr，实际 false")
	}
	if !IsReadErr(&urlError{err: io.ErrUnexpectedEOF}) {
		t.Fatalf("url.Error 包装的短读应命中（errors.Is 穿透），实际 false")
	}
	// 超时形态：awaiting headers（请求体已到达、等待响应头超时）
	timeout := errors.New("Post \"https://x\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
	if !IsReadErr(timeout) {
		t.Fatalf("超时形态应命中 IsReadErr，实际 false")
	}
	// 超时形态二：reading body（响应头已到达、正文传输超时——比 awaiting headers
	// 更强地"平台已响应"，标准库 client.go:994 wrap 文案）
	bodyTimeout := errors.New("context deadline exceeded (Client.Timeout or context cancellation while reading body)")
	if !IsReadErr(bodyTimeout) {
		t.Fatalf("reading body 超时形态应命中 IsReadErr，实际 false")
	}
	// 对照：dial/write 错误不命中（与 isConnErrRetryable 互斥）
	if IsReadErr(&net.OpError{Op: "dial", Err: errors.New("connectex")}) {
		t.Fatalf("dial 错误不应命中 IsReadErr")
	}
	if IsReadErr(&net.OpError{Op: "write", Err: errors.New("write failed")}) {
		t.Fatalf("write 错误不应命中 IsReadErr")
	}
	// 业务错误不命中
	if IsReadErr(errors.New("验证码错误")) {
		t.Fatalf("业务错误不应命中 IsReadErr")
	}
	// nil 不命中
	if IsReadErr(nil) {
		t.Fatalf("nil 不应命中 IsReadErr")
	}
}

// urlError 最小 url.Error 模拟（真实 *url.Error 的 Err 字段，errors.As/Is 穿透路径一致）。
type urlError struct{ err error }

func (e *urlError) Error() string { return "Post \"http://x\": " + e.err.Error() }
func (e *urlError) Unwrap() error { return e.err }
