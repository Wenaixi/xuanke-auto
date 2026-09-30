package upstream

// IsReadErr 形态矩阵契约测试：IsReadErr 必须覆盖"请求已发出、
// 平台可能已处理"的全部三种形态——RST（net.OpError read）/ FIN（io.EOF）/ 超时
// （Client.Timeout exceeded awaiting headers）。任一形态 miss 会让 scheduler/api
// 对"平台可能已抢到课"的错误显示"报名失败"误导文案（黄金期重复报名被拒时 failed 残留）。
import (
	"context"
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
	// 超时形态：走 net.Error.Timeout() 判定，**不匹配错误文案**（标准库两种 wrap
	// 文案随版本变化，靠 strings.Contains 匹配既脆又违反「错误分流只认结构化事实」）。
	// context.DeadlineExceeded 实现 net.Error，是超时的标准代表。
	if !IsReadErr(context.DeadlineExceeded) {
		t.Fatalf("超时形态（context.DeadlineExceeded）应命中 IsReadErr，实际 false")
	}
	// 超时穿透包装链：url.Error 包装后仍应命中（errors.As 穿透）。
	if !IsReadErr(&urlError{err: context.DeadlineExceeded}) {
		t.Fatalf("url.Error 包装的超时应命中（errors.As 穿透），实际 false")
	}
	// 反向防线：非超时的普通错误不得被误判为 read 中断。
	if IsReadErr(errors.New("Client.Timeout exceeded while awaiting headers")) {
		t.Fatal("纯文本超时文案不实现 net.Error，不得被判为 read 中断（正是去掉文案匹配的原因）")
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
