package scheduler

import (
	"errors"
	"io"
	"net"
	"testing"

	"xuanke-auto/backend/internal/zhidao"
)

// TestClassifyPlatformError 平台错误分类纯函数（表驱动）。
// 手动/自动报名决策树合一后共用同一分类：token 失效 / read 中断 / 窗口关闭 /
// 风控退避 / 普通业务错误 五族归一。文案匹配为逐字迁移的子串集合，
// zhidao.ErrUnauthorized 与 IsReadErr 判定收口。
func TestClassifyPlatformError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want platformErrorKind
	}{
		{"未登录 token 失效", zhidao.ErrUnauthorized, errAuth},
		{"read 中断（RST 形态）", &net.OpError{Op: "read", Err: errors.New("connection reset")}, errRead},
		{"read EOF（FIN 形态）", io.EOF, errRead},
		{"窗口关闭文案", errors.New("不在选修报名时间范围内，无法选课！"), errWindowClosed},
		{"未开启文案", errors.New("选课未开启"), errWindowClosed},
		{"报名时间已结束", errors.New("报名时间已结束"), errWindowClosed},
		{"风控频繁文案", errors.New("操作过于频繁，请稍后重试"), errRateLimit},
		{"429 文案", errors.New("HTTP 429 Too Many Requests"), errRateLimit},
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
