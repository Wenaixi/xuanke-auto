package scheduler

import (
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"xuanke-auto/backend/internal/upstream"
)

// TestClassifyPlatformError 平台错误分类纯函数（表驱动）。
// 手动/自动报名决策树共用同一分类：token 失效 / read 中断 / 窗口关闭 / 风控退避 /
// 满员 / 普通业务错误。**判据只认结构化事实**（ErrUnauthorized / IsReadErr /
// SiteError.Kind），绝不匹配平台文案。
func TestClassifyPlatformError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want platformErrorKind
	}{
		{"未登录 token 失效", upstream.ErrUnauthorized, errAuth},
		{"read 中断（RST 形态）", &net.OpError{Op: "read", Err: errors.New("connection reset")}, errRead},
		{"read EOF（FIN 形态）", io.EOF, errRead},
		// 结构化分类：三条 SiteError 分支
		{"档案分类为风控", &upstream.SiteError{Kind: upstream.OpErrorRateLimited, Msg: "任意文案", Code: 1}, errRateLimit},
		{"档案分类为窗口关闭", &upstream.SiteError{Kind: upstream.OpErrorWindowClosed, Msg: "任意文案", Code: 1}, errWindowClosed},
		{"档案分类为满员", &upstream.SiteError{Kind: upstream.OpErrorClassFull, Msg: "任意文案", Code: 1}, errFull},
		{"档案未分类", &upstream.SiteError{Kind: upstream.OpErrorUnknown, Msg: "课程不存在", Code: 1}, errOther},
		// 关键防线：含「已满员」字样的普通错误绝不能被误判成满员——
		// 这正是中文文案匹配方案的根本失败模式（旧实现会判 errOther，此处锁死）。
		{"含已满员字样但未分类", errors.New("该课程已满员，无法退选"), errOther},
		{"含已结束字样但未分类", errors.New("学期已结束，请下学期再试"), errOther},
		{"普通业务错误", errors.New("课程不存在"), errOther},
		{"nil 错误", nil, errOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyPlatformError(c.err); got != c.want {
				t.Fatalf("classifyPlatformError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// TestClassifyPlatformErrorUnwrapsSiteError SiteError 被包装后仍能穿透分类
// （errors.As 穿透 fmt.Errorf 的 %w 包装链）——热路径的错误都带上下文包装。
func TestClassifyPlatformErrorUnwrapsSiteError(t *testing.T) {
	inner := &upstream.SiteError{Kind: upstream.OpErrorRateLimited, Msg: "操作过于频繁", Code: 1}
	wrapped := fmt.Errorf("报名失败: %w", inner)
	if got := classifyPlatformError(wrapped); got != errRateLimit {
		t.Fatalf("包装后的 SiteError 应仍判风控，实际 %v", got)
	}
}
